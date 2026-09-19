package workflows

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// ========== Journey 11: Disbursement Processing ==========
// This workflow implements a two-phase commit pattern for safe disbursements:
// Phase 1: Reserve funds in TigerBeetle (pending transfer)
// Phase 2a: Execute external payment via Mojaloop FSPIOP
// Phase 2b: Commit or rollback TigerBeetle based on Mojaloop result

type DisbursementInput struct {
	DisbursementID    string   `json:"disbursementID"`
	ProgramID         string   `json:"programID"`
	BeneficiaryIDs    []string `json:"beneficiaryIDs"`
	Amount            float64  `json:"amount"`
	ScheduledDate     string   `json:"scheduledDate"`
	DisbursementType  string   `json:"disbursementType"` // regular, emergency, supplemental
	InitiatedBy       string   `json:"initiatedBy"`
	Currency          string   `json:"currency"`         // NGN, TZS, etc.
	UseMojaloop       bool     `json:"useMojaloop"`      // Whether to use Mojaloop for external payment
	FeeAmount         float64  `json:"feeAmount"`        // Optional fee per disbursement
	FeeAccountID      string   `json:"feeAccountID"`     // Fee collection account
}

type DisbursementResult struct {
	DisbursementID       string                 `json:"disbursementID"`
	Status               string                 `json:"status"`
	SuccessCount         int                    `json:"successCount"`
	FailureCount         int                    `json:"failureCount"`
	RolledBackCount      int                    `json:"rolledBackCount"`
	TotalAmount          float64                `json:"totalAmount"`
	TotalFees            float64                `json:"totalFees"`
	TransactionIDs       []string               `json:"transactionIDs"`
	MojaloopTransferIDs  []string               `json:"mojaloopTransferIDs"`
	ReconciliationID     string                 `json:"reconciliationID"`
	SettlementCycleID    string                 `json:"settlementCycleId"`
	FailedTransfers      []FailedTransfer       `json:"failedTransfers"`
	Message              string                 `json:"message"`
}

type FailedTransfer struct {
	BeneficiaryID       string  `json:"beneficiaryID"`
	Amount              float64 `json:"amount"`
	Reason              string  `json:"reason"`
	PendingTransferID   string  `json:"pendingTransferId,omitempty"`
	RolledBack          bool    `json:"rolledBack"`
}

// PendingDisbursementInfo tracks a pending disbursement for two-phase commit
type PendingDisbursementInfo struct {
	BeneficiaryID       string  `json:"beneficiaryID"`
	PendingTransferID   string  `json:"pendingTransferId"`
	Amount              float64 `json:"amount"`
	FeeAmount           float64 `json:"feeAmount"`
	Reserved            bool    `json:"reserved"`
}

func DisbursementProcessingWorkflow(ctx workflow.Context, input DisbursementInput) (*DisbursementResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting two-phase commit disbursement workflow", "disbursementID", input.DisbursementID)

	activityOptions := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    5 * time.Minute,
			MaximumAttempts:    5,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, activityOptions)

	var result DisbursementResult
	result.DisbursementID = input.DisbursementID
	result.TotalAmount = input.Amount * float64(len(input.BeneficiaryIDs))
	result.TotalFees = input.FeeAmount * float64(len(input.BeneficiaryIDs))

	// Step 1: Validate program budget availability (including fees)
	totalRequired := result.TotalAmount + result.TotalFees
	var budgetAvailable bool
	err := workflow.ExecuteActivity(ctx, "ValidateProgramBudgetActivity", input.ProgramID, totalRequired).Get(ctx, &budgetAvailable)
	if err != nil || !budgetAvailable {
		result.Status = "failed"
		result.Message = "Insufficient program budget"
		return &result, fmt.Errorf("budget validation failed: %w", err)
	}

	// Step 2: Create disbursement batch record
	var batchID string
	err = workflow.ExecuteActivity(ctx, "CreateDisbursementBatchActivity", input.DisbursementID, input.ProgramID, len(input.BeneficiaryIDs), totalRequired).Get(ctx, &batchID)
	if err != nil {
		return &result, err
	}

	// Step 3: Validate all beneficiaries are eligible
	var eligibilityResults map[string]bool
	err = workflow.ExecuteActivity(ctx, "ValidateBeneficiaryEligibilityBatchActivity", input.BeneficiaryIDs, input.ProgramID).Get(ctx, &eligibilityResults)
	if err != nil {
		return &result, err
	}

	// Step 4: Create settlement cycle for Mojaloop tracking (if using Mojaloop)
	var settlementCycleID string
	if input.UseMojaloop {
		err = workflow.ExecuteActivity(ctx, "CreateSettlementCycleActivity", input.Currency).Get(ctx, &settlementCycleID)
		if err != nil {
			logger.Warn("Failed to create settlement cycle", "error", err)
		}
		result.SettlementCycleID = settlementCycleID
	}

	// Step 5: TWO-PHASE COMMIT - Process disbursements with reserve/commit/rollback pattern
	var transactionIDs []string
	var mojaloopTransferIDs []string
	var failedTransfers []FailedTransfer
	var pendingDisbursements []PendingDisbursementInfo

	for _, beneficiaryID := range input.BeneficiaryIDs {
		if !eligibilityResults[beneficiaryID] {
			failedTransfers = append(failedTransfers, FailedTransfer{
				BeneficiaryID: beneficiaryID,
				Amount:        input.Amount,
				Reason:        "Beneficiary not eligible",
				RolledBack:    false,
			})
			result.FailureCount++
			continue
		}

		// PHASE 1: Reserve funds in TigerBeetle (pending transfer)
		var pendingTransferID string
		err = workflow.ExecuteActivity(ctx, "ReserveFundsForDisbursementActivity", input.ProgramID, beneficiaryID, input.Amount, input.FeeAmount, input.FeeAccountID).Get(ctx, &pendingTransferID)
		if err != nil {
			failedTransfers = append(failedTransfers, FailedTransfer{
				BeneficiaryID: beneficiaryID,
				Amount:        input.Amount,
				Reason:        fmt.Sprintf("Failed to reserve funds: %v", err),
				RolledBack:    false,
			})
			result.FailureCount++
			continue
		}

		pendingDisbursements = append(pendingDisbursements, PendingDisbursementInfo{
			BeneficiaryID:     beneficiaryID,
			PendingTransferID: pendingTransferID,
			Amount:            input.Amount,
			FeeAmount:         input.FeeAmount,
			Reserved:          true,
		})
	}

	// PHASE 2: Execute external payments and commit/rollback based on results
	for _, pending := range pendingDisbursements {
		var externalPaymentSuccess bool
		var mojaloopTransferID string

		if input.UseMojaloop {
			// PHASE 2a: Execute Mojaloop FSPIOP transfer (Quote -> Transfer -> Fulfilment)
			err = workflow.ExecuteActivity(ctx, "ExecuteMojaloopTransferActivity", pending.BeneficiaryID, pending.Amount, input.Currency, settlementCycleID).Get(ctx, &mojaloopTransferID)
			externalPaymentSuccess = err == nil
			if externalPaymentSuccess {
				mojaloopTransferIDs = append(mojaloopTransferIDs, mojaloopTransferID)
			}
		} else {
			// Direct disbursement without Mojaloop (internal transfer only)
			externalPaymentSuccess = true
		}

		if externalPaymentSuccess {
			// PHASE 2b-SUCCESS: Commit the pending transfer in TigerBeetle
			var commitTransferID string
			err = workflow.ExecuteActivity(ctx, "CommitDisbursementActivity", pending.PendingTransferID, pending.Amount).Get(ctx, &commitTransferID)
			if err != nil {
				// Commit failed - this is a critical error, funds are stuck in pending
				logger.Error("CRITICAL: Failed to commit disbursement after successful external payment",
					"beneficiaryID", pending.BeneficiaryID,
					"pendingTransferID", pending.PendingTransferID,
					"error", err)
				failedTransfers = append(failedTransfers, FailedTransfer{
					BeneficiaryID:     pending.BeneficiaryID,
					Amount:            pending.Amount,
					Reason:            fmt.Sprintf("Commit failed after external payment: %v", err),
					PendingTransferID: pending.PendingTransferID,
					RolledBack:        false,
				})
				result.FailureCount++
			} else {
				transactionIDs = append(transactionIDs, commitTransferID)
				result.SuccessCount++
			}
		} else {
			// PHASE 2b-FAILURE: Rollback the pending transfer in TigerBeetle
			err = workflow.ExecuteActivity(ctx, "RollbackDisbursementActivity", pending.PendingTransferID).Get(ctx, nil)
			rolledBack := err == nil
			if !rolledBack {
				logger.Error("CRITICAL: Failed to rollback disbursement after failed external payment",
					"beneficiaryID", pending.BeneficiaryID,
					"pendingTransferID", pending.PendingTransferID,
					"error", err)
			}
			failedTransfers = append(failedTransfers, FailedTransfer{
				BeneficiaryID:     pending.BeneficiaryID,
				Amount:            pending.Amount,
				Reason:            "External payment failed via Mojaloop",
				PendingTransferID: pending.PendingTransferID,
				RolledBack:        rolledBack,
			})
			result.FailureCount++
			if rolledBack {
				result.RolledBackCount++
			}
		}
	}

	result.TransactionIDs = transactionIDs
	result.MojaloopTransferIDs = mojaloopTransferIDs
	result.FailedTransfers = failedTransfers

	// Step 6: Close settlement cycle (if using Mojaloop)
	if input.UseMojaloop && settlementCycleID != "" {
		err = workflow.ExecuteActivity(ctx, "CloseSettlementCycleActivity", settlementCycleID).Get(ctx, nil)
		if err != nil {
			logger.Warn("Failed to close settlement cycle", "error", err)
		}
	}

	// Step 7: Update disbursement batch status
	err = workflow.ExecuteActivity(ctx, "UpdateDisbursementBatchStatusActivity", batchID, result.SuccessCount, result.FailureCount).Get(ctx, nil)
	if err != nil {
		logger.Warn("Failed to update batch status", "error", err)
	}

	// Step 8: Perform reconciliation
	var reconciliationID string
	err = workflow.ExecuteActivity(ctx, "ReconcileDisbursementActivity", batchID, transactionIDs).Get(ctx, &reconciliationID)
	if err != nil {
		logger.Warn("Reconciliation failed", "error", err)
	}
	result.ReconciliationID = reconciliationID

	// Step 9: Update program budget (only for successful disbursements)
	successfulAmount := input.Amount * float64(result.SuccessCount)
	successfulFees := input.FeeAmount * float64(result.SuccessCount)
	err = workflow.ExecuteActivity(ctx, "UpdateProgramBudgetActivity", input.ProgramID, -(successfulAmount + successfulFees)).Get(ctx, nil)
	if err != nil {
		logger.Error("Failed to update program budget", "error", err)
	}

	// Step 10: Publish disbursement events to Kafka
	err = workflow.ExecuteActivity(ctx, "PublishKafkaEventActivity", "disbursement.completed", input.DisbursementID, result).Get(ctx, nil)
	if err != nil {
		logger.Warn("Failed to publish Kafka event", "error", err)
	}

	// Step 11: Send notifications to beneficiaries
	for _, txID := range transactionIDs {
		err = workflow.ExecuteActivity(ctx, "SendDisbursementNotificationActivity", txID, input.Amount).Get(ctx, nil)
		if err != nil {
			logger.Warn("Failed to send notification", "transactionID", txID, "error", err)
		}
	}

	// Step 12: Create audit log
	err = workflow.ExecuteActivity(ctx, "CreateAuditLogActivity", "disbursement.processed", input.DisbursementID, input.InitiatedBy, result).Get(ctx, nil)
	if err != nil {
		logger.Warn("Failed to create audit log", "error", err)
	}

	// Step 13: Trigger Lakehouse analytics ingestion
	err = workflow.ExecuteActivity(ctx, "IngestToLakehouseActivity", "disbursement", result).Get(ctx, nil)
	if err != nil {
		logger.Warn("Failed to ingest to Lakehouse", "error", err)
	}

	// Determine final status
	if result.FailureCount == 0 {
		result.Status = "completed"
		result.Message = fmt.Sprintf("All %d disbursements processed successfully (two-phase commit)", result.SuccessCount)
	} else if result.SuccessCount == 0 {
		result.Status = "failed"
		result.Message = fmt.Sprintf("All %d disbursements failed (%d rolled back)", result.FailureCount, result.RolledBackCount)
	} else {
		result.Status = "partial"
		result.Message = fmt.Sprintf("%d succeeded, %d failed (%d rolled back)", result.SuccessCount, result.FailureCount, result.RolledBackCount)
	}

	logger.Info("Two-phase commit disbursement completed",
		"status", result.Status,
		"successCount", result.SuccessCount,
		"failureCount", result.FailureCount,
		"rolledBackCount", result.RolledBackCount)
	return &result, nil
}

// ========== Journey 11 Supporting Workflows ==========

// RetryFailedDisbursementsWorkflow retries failed disbursements
type RetryDisbursementInput struct {
	OriginalDisbursementID string           `json:"originalDisbursementID"`
	FailedTransfers        []FailedTransfer `json:"failedTransfers"`
	ProgramID              string           `json:"programID"`
	Amount                 float64          `json:"amount"`
}

func RetryFailedDisbursementsWorkflow(ctx workflow.Context, input RetryDisbursementInput) (*DisbursementResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting retry failed disbursements workflow", "originalDisbursementID", input.OriginalDisbursementID)

	activityOptions := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    5 * time.Minute,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, activityOptions)

	var result DisbursementResult
	result.DisbursementID = input.OriginalDisbursementID + "-retry"

	var transactionIDs []string
	var stillFailed []FailedTransfer

	for _, failedTransfer := range input.FailedTransfers {
		// Retry TigerBeetle transfer
		var transferID string
		err := workflow.ExecuteActivity(ctx, "ExecuteTigerBeetleTransferActivity", input.ProgramID, failedTransfer.BeneficiaryID, failedTransfer.Amount).Get(ctx, &transferID)
		
		if err != nil {
			stillFailed = append(stillFailed, FailedTransfer{
				BeneficiaryID: failedTransfer.BeneficiaryID,
				Amount:        failedTransfer.Amount,
				Reason:        fmt.Sprintf("Retry failed: %v", err),
			})
			result.FailureCount++
		} else {
			transactionIDs = append(transactionIDs, transferID)
			result.SuccessCount++
		}
	}

	result.TransactionIDs = transactionIDs
	result.FailedTransfers = stillFailed

	if result.FailureCount == 0 {
		result.Status = "completed"
		result.Message = "All retries successful"
	} else {
		result.Status = "partial"
		result.Message = fmt.Sprintf("%d succeeded, %d still failed", result.SuccessCount, result.FailureCount)
	}

	// Publish retry result
	_ = workflow.ExecuteActivity(ctx, "PublishKafkaEventActivity", "disbursement.retry.completed", result.DisbursementID, result).Get(ctx, nil)

	return &result, nil
}

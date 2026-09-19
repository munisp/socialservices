package workflows

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// ========== Payments Reconciliation Workflow ==========

// ReconciliationInput contains the input for the reconciliation workflow
type ReconciliationInput struct {
	ReconciliationID string    `json:"reconciliationId"`
	StartDate        time.Time `json:"startDate"`
	EndDate          time.Time `json:"endDate"`
	ProgramID        string    `json:"programId,omitempty"` // Optional: filter by program
	InitiatedBy      string    `json:"initiatedBy"`
	ReconcileType    string    `json:"reconcileType"` // daily, weekly, monthly, manual
}

// ReconciliationResult contains the result of the reconciliation workflow
type ReconciliationResult struct {
	ReconciliationID     string                `json:"reconciliationId"`
	Status               string                `json:"status"` // completed, failed, partial
	TotalTransactions    int                   `json:"totalTransactions"`
	MatchedTransactions  int                   `json:"matchedTransactions"`
	MismatchedCount      int                   `json:"mismatchedCount"`
	MissingInLedger      int                   `json:"missingInLedger"`
	MissingInProvider    int                   `json:"missingInProvider"`
	AmountDiscrepancy    float64               `json:"amountDiscrepancy"`
	Discrepancies        []TransactionMismatch `json:"discrepancies"`
	ResolvedCount        int                   `json:"resolvedCount"`
	PendingResolution    int                   `json:"pendingResolution"`
	CompletedAt          time.Time             `json:"completedAt"`
	Message              string                `json:"message"`
}

// TransactionMismatch represents a discrepancy between ledger and provider
type TransactionMismatch struct {
	TransactionID     string    `json:"transactionId"`
	BeneficiaryID     string    `json:"beneficiaryId"`
	LedgerAmount      float64   `json:"ledgerAmount"`
	ProviderAmount    float64   `json:"providerAmount"`
	LedgerStatus      string    `json:"ledgerStatus"`
	ProviderStatus    string    `json:"providerStatus"`
	DiscrepancyType   string    `json:"discrepancyType"` // amount_mismatch, status_mismatch, missing_ledger, missing_provider
	DetectedAt        time.Time `json:"detectedAt"`
	ResolutionStatus  string    `json:"resolutionStatus"` // pending, auto_resolved, manual_required, resolved
	ResolutionAction  string    `json:"resolutionAction,omitempty"`
	ResolvedAt        *time.Time `json:"resolvedAt,omitempty"`
}

// PaymentsReconciliationWorkflow performs end-to-end reconciliation between
// the internal ledger (TigerBeetle) and external payment providers
func PaymentsReconciliationWorkflow(ctx workflow.Context, input ReconciliationInput) (*ReconciliationResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting payments reconciliation workflow",
		"reconciliationId", input.ReconciliationID,
		"startDate", input.StartDate,
		"endDate", input.EndDate,
	)

	activityOptions := workflow.ActivityOptions{
		StartToCloseTimeout: 60 * time.Minute,
		HeartbeatTimeout:    5 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second * 5,
			BackoffCoefficient: 2.0,
			MaximumInterval:    5 * time.Minute,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, activityOptions)

	result := &ReconciliationResult{
		ReconciliationID: input.ReconciliationID,
	}

	// Step 1: Fetch transactions from internal ledger (TigerBeetle)
	var ledgerTransactions []LedgerTransaction
	err := workflow.ExecuteActivity(ctx, "FetchLedgerTransactionsActivity",
		input.StartDate, input.EndDate, input.ProgramID).Get(ctx, &ledgerTransactions)
	if err != nil {
		result.Status = "failed"
		result.Message = fmt.Sprintf("Failed to fetch ledger transactions: %v", err)
		return result, err
	}
	logger.Info("Fetched ledger transactions", "count", len(ledgerTransactions))

	// Step 2: Fetch transactions from payment provider(s)
	var providerTransactions []ProviderTransaction
	err = workflow.ExecuteActivity(ctx, "FetchProviderTransactionsActivity",
		input.StartDate, input.EndDate, input.ProgramID).Get(ctx, &providerTransactions)
	if err != nil {
		result.Status = "failed"
		result.Message = fmt.Sprintf("Failed to fetch provider transactions: %v", err)
		return result, err
	}
	logger.Info("Fetched provider transactions", "count", len(providerTransactions))

	// Step 3: Perform matching and identify discrepancies
	var matchResult MatchingResult
	err = workflow.ExecuteActivity(ctx, "MatchTransactionsActivity",
		ledgerTransactions, providerTransactions).Get(ctx, &matchResult)
	if err != nil {
		result.Status = "failed"
		result.Message = fmt.Sprintf("Failed to match transactions: %v", err)
		return result, err
	}

	result.TotalTransactions = len(ledgerTransactions)
	result.MatchedTransactions = matchResult.MatchedCount
	result.MismatchedCount = len(matchResult.Mismatches)
	result.MissingInLedger = matchResult.MissingInLedger
	result.MissingInProvider = matchResult.MissingInProvider
	result.AmountDiscrepancy = matchResult.TotalAmountDiscrepancy
	result.Discrepancies = matchResult.Mismatches

	logger.Info("Matching completed",
		"matched", result.MatchedTransactions,
		"mismatched", result.MismatchedCount,
		"missingInLedger", result.MissingInLedger,
		"missingInProvider", result.MissingInProvider,
	)

	// Step 4: Attempt auto-resolution for known discrepancy patterns
	if len(matchResult.Mismatches) > 0 {
		var autoResolved []string
		err = workflow.ExecuteActivity(ctx, "AutoResolveDiscrepanciesActivity",
			matchResult.Mismatches).Get(ctx, &autoResolved)
		if err != nil {
			logger.Warn("Auto-resolution failed", "error", err)
		} else {
			result.ResolvedCount = len(autoResolved)
			// Update discrepancy statuses
			for i := range result.Discrepancies {
				for _, resolvedID := range autoResolved {
					if result.Discrepancies[i].TransactionID == resolvedID {
						result.Discrepancies[i].ResolutionStatus = "auto_resolved"
						now := workflow.Now(ctx)
						result.Discrepancies[i].ResolvedAt = &now
					}
				}
			}
		}
	}

	// Step 5: Create reconciliation report
	err = workflow.ExecuteActivity(ctx, "CreateReconciliationReportActivity",
		input.ReconciliationID, result).Get(ctx, nil)
	if err != nil {
		logger.Warn("Failed to create reconciliation report", "error", err)
	}

	// Step 6: Update ledger invariants check
	var invariantsValid bool
	err = workflow.ExecuteActivity(ctx, "CheckLedgerInvariantsActivity",
		input.ProgramID).Get(ctx, &invariantsValid)
	if err != nil || !invariantsValid {
		logger.Error("Ledger invariants check failed", "error", err)
		// Create alert for manual investigation
		_ = workflow.ExecuteActivity(ctx, "CreateReconciliationAlertActivity",
			input.ReconciliationID, "INVARIANTS_FAILED", "Ledger invariants check failed").Get(ctx, nil)
	}

	// Step 7: Publish reconciliation event
	err = workflow.ExecuteActivity(ctx, "PublishKafkaEventActivity",
		"reconciliation.completed", input.ReconciliationID, result).Get(ctx, nil)
	if err != nil {
		logger.Warn("Failed to publish reconciliation event", "error", err)
	}

	// Step 8: Create audit log
	err = workflow.ExecuteActivity(ctx, "CreateAuditLogActivity",
		"reconciliation.completed", input.ReconciliationID, input.InitiatedBy, result).Get(ctx, nil)
	if err != nil {
		logger.Warn("Failed to create audit log", "error", err)
	}

	// Step 9: Send notifications for unresolved discrepancies
	result.PendingResolution = result.MismatchedCount - result.ResolvedCount
	if result.PendingResolution > 0 {
		err = workflow.ExecuteActivity(ctx, "SendReconciliationAlertActivity",
			input.ReconciliationID, result.PendingResolution, result.AmountDiscrepancy).Get(ctx, nil)
		if err != nil {
			logger.Warn("Failed to send reconciliation alert", "error", err)
		}
	}

	// Determine final status
	result.CompletedAt = workflow.Now(ctx)
	if result.MismatchedCount == 0 {
		result.Status = "completed"
		result.Message = fmt.Sprintf("All %d transactions reconciled successfully", result.TotalTransactions)
	} else if result.PendingResolution == 0 {
		result.Status = "completed"
		result.Message = fmt.Sprintf("Reconciliation completed. %d discrepancies auto-resolved", result.ResolvedCount)
	} else {
		result.Status = "partial"
		result.Message = fmt.Sprintf("Reconciliation completed with %d pending discrepancies requiring manual review", result.PendingResolution)
	}

	logger.Info("Reconciliation workflow completed", "status", result.Status)
	return result, nil
}

// LedgerTransaction represents a transaction from the internal ledger
type LedgerTransaction struct {
	TransactionID string    `json:"transactionId"`
	BeneficiaryID string    `json:"beneficiaryId"`
	ProgramID     string    `json:"programId"`
	Amount        float64   `json:"amount"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"createdAt"`
	Reference     string    `json:"reference"`
}

// ProviderTransaction represents a transaction from the payment provider
type ProviderTransaction struct {
	ProviderTxID  string    `json:"providerTxId"`
	Reference     string    `json:"reference"` // Maps to our transaction ID
	BeneficiaryID string    `json:"beneficiaryId"`
	Amount        float64   `json:"amount"`
	Status        string    `json:"status"`
	ProcessedAt   time.Time `json:"processedAt"`
	ProviderName  string    `json:"providerName"`
}

// MatchingResult contains the result of transaction matching
type MatchingResult struct {
	MatchedCount           int                   `json:"matchedCount"`
	MissingInLedger        int                   `json:"missingInLedger"`
	MissingInProvider      int                   `json:"missingInProvider"`
	TotalAmountDiscrepancy float64               `json:"totalAmountDiscrepancy"`
	Mismatches             []TransactionMismatch `json:"mismatches"`
}

// ========== Dispute/Chargeback Workflow ==========

// DisputeInput contains the input for the dispute workflow
type DisputeInput struct {
	DisputeID       string    `json:"disputeId"`
	TransactionID   string    `json:"transactionId"`
	BeneficiaryID   string    `json:"beneficiaryId"`
	DisputeType     string    `json:"disputeType"` // chargeback, reversal, correction
	DisputeReason   string    `json:"disputeReason"`
	DisputedAmount  float64   `json:"disputedAmount"`
	InitiatedBy     string    `json:"initiatedBy"`
	EvidenceURLs    []string  `json:"evidenceUrls,omitempty"`
	CreatedAt       time.Time `json:"createdAt"`
}

// DisputeResult contains the result of the dispute workflow
type DisputeResult struct {
	DisputeID       string     `json:"disputeId"`
	Status          string     `json:"status"` // resolved, rejected, escalated
	Resolution      string     `json:"resolution"`
	ResolvedAmount  float64    `json:"resolvedAmount"`
	ResolvedAt      *time.Time `json:"resolvedAt,omitempty"`
	Message         string     `json:"message"`
}

// DisputeResolutionWorkflow handles payment disputes and chargebacks
func DisputeResolutionWorkflow(ctx workflow.Context, input DisputeInput) (*DisputeResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting dispute resolution workflow", "disputeId", input.DisputeID)

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

	result := &DisputeResult{
		DisputeID: input.DisputeID,
	}

	// Step 1: Validate the original transaction
	var txValid bool
	err := workflow.ExecuteActivity(ctx, "ValidateTransactionActivity",
		input.TransactionID).Get(ctx, &txValid)
	if err != nil || !txValid {
		result.Status = "rejected"
		result.Message = "Original transaction not found or invalid"
		return result, nil
	}

	// Step 2: Check dispute eligibility (time limits, amount limits, etc.)
	var eligible bool
	err = workflow.ExecuteActivity(ctx, "CheckDisputeEligibilityActivity",
		input.TransactionID, input.DisputeType, input.DisputedAmount).Get(ctx, &eligible)
	if err != nil || !eligible {
		result.Status = "rejected"
		result.Message = "Dispute not eligible based on policy rules"
		return result, nil
	}

	// Step 3: Create dispute record
	err = workflow.ExecuteActivity(ctx, "CreateDisputeRecordActivity",
		input).Get(ctx, nil)
	if err != nil {
		return result, err
	}

	// Step 4: Freeze the disputed amount if applicable
	if input.DisputeType == "chargeback" {
		err = workflow.ExecuteActivity(ctx, "FreezeDisputedAmountActivity",
			input.BeneficiaryID, input.DisputedAmount).Get(ctx, nil)
		if err != nil {
			logger.Warn("Failed to freeze disputed amount", "error", err)
		}
	}

	// Step 5: Wait for investigation (with timeout)
	investigationTimeout := 7 * 24 * time.Hour // 7 days
	investigationCtx, cancel := workflow.WithCancel(ctx)
	defer cancel()

	var investigationResult string
	investigationFuture := workflow.ExecuteActivity(investigationCtx, "InvestigateDisputeActivity",
		input.DisputeID, input.EvidenceURLs)

	// Wait for investigation or timeout
	selector := workflow.NewSelector(ctx)
	selector.AddFuture(investigationFuture, func(f workflow.Future) {
		err = f.Get(ctx, &investigationResult)
	})

	timerFuture := workflow.NewTimer(ctx, investigationTimeout)
	selector.AddFuture(timerFuture, func(f workflow.Future) {
		investigationResult = "timeout"
	})

	selector.Select(ctx)

	// Step 6: Process resolution based on investigation
	switch investigationResult {
	case "approved":
		// Process refund/reversal
		err = workflow.ExecuteActivity(ctx, "ProcessDisputeRefundActivity",
			input.TransactionID, input.BeneficiaryID, input.DisputedAmount).Get(ctx, nil)
		if err != nil {
			result.Status = "escalated"
			result.Message = "Refund processing failed, escalated for manual review"
		} else {
			result.Status = "resolved"
			result.Resolution = "refund_processed"
			result.ResolvedAmount = input.DisputedAmount
			now := workflow.Now(ctx)
			result.ResolvedAt = &now
			result.Message = "Dispute resolved in favor of beneficiary"
		}
	case "rejected":
		result.Status = "rejected"
		result.Resolution = "dispute_denied"
		result.Message = "Dispute rejected based on investigation findings"
	case "timeout":
		result.Status = "escalated"
		result.Resolution = "investigation_timeout"
		result.Message = "Investigation timed out, escalated for manual review"
	default:
		result.Status = "escalated"
		result.Message = "Unknown investigation result, escalated for manual review"
	}

	// Step 7: Unfreeze amount if dispute was rejected
	if result.Status == "rejected" && input.DisputeType == "chargeback" {
		_ = workflow.ExecuteActivity(ctx, "UnfreezeDisputedAmountActivity",
			input.BeneficiaryID, input.DisputedAmount).Get(ctx, nil)
	}

	// Step 8: Update dispute record
	err = workflow.ExecuteActivity(ctx, "UpdateDisputeRecordActivity",
		input.DisputeID, result).Get(ctx, nil)
	if err != nil {
		logger.Warn("Failed to update dispute record", "error", err)
	}

	// Step 9: Send notification
	err = workflow.ExecuteActivity(ctx, "SendDisputeNotificationActivity",
		input.BeneficiaryID, input.DisputeID, result.Status).Get(ctx, nil)
	if err != nil {
		logger.Warn("Failed to send dispute notification", "error", err)
	}

	// Step 10: Create audit log
	err = workflow.ExecuteActivity(ctx, "CreateAuditLogActivity",
		"dispute.resolved", input.DisputeID, input.InitiatedBy, result).Get(ctx, nil)
	if err != nil {
		logger.Warn("Failed to create audit log", "error", err)
	}

	logger.Info("Dispute resolution workflow completed", "status", result.Status)
	return result, nil
}

// ========== Scheduled Reconciliation Workflow ==========

// ScheduledReconciliationWorkflow runs reconciliation on a schedule
func ScheduledReconciliationWorkflow(ctx workflow.Context, reconcileType string) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting scheduled reconciliation", "type", reconcileType)

	for {
		// Calculate date range based on reconcile type
		now := workflow.Now(ctx)
		var startDate, endDate time.Time
		var waitDuration time.Duration

		switch reconcileType {
		case "daily":
			startDate = now.AddDate(0, 0, -1).Truncate(24 * time.Hour)
			endDate = now.Truncate(24 * time.Hour)
			waitDuration = 24 * time.Hour
		case "weekly":
			startDate = now.AddDate(0, 0, -7).Truncate(24 * time.Hour)
			endDate = now.Truncate(24 * time.Hour)
			waitDuration = 7 * 24 * time.Hour
		case "monthly":
			startDate = now.AddDate(0, -1, 0).Truncate(24 * time.Hour)
			endDate = now.Truncate(24 * time.Hour)
			waitDuration = 30 * 24 * time.Hour
		default:
			return fmt.Errorf("unknown reconcile type: %s", reconcileType)
		}

		// Create reconciliation input
		input := ReconciliationInput{
			ReconciliationID: fmt.Sprintf("scheduled-%s-%s", reconcileType, now.Format("20060102")),
			StartDate:        startDate,
			EndDate:          endDate,
			InitiatedBy:      "system",
			ReconcileType:    reconcileType,
		}

		// Execute reconciliation as child workflow
		childCtx := workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
			WorkflowID: input.ReconciliationID,
		})

		var result ReconciliationResult
		err := workflow.ExecuteChildWorkflow(childCtx, PaymentsReconciliationWorkflow, input).Get(ctx, &result)
		if err != nil {
			logger.Error("Scheduled reconciliation failed", "error", err)
		} else {
			logger.Info("Scheduled reconciliation completed", "status", result.Status)
		}

		// Wait for next scheduled run
		_ = workflow.Sleep(ctx, waitDuration)
	}
}

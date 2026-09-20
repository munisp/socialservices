package journeys

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// Payment Journey Definitions (Journeys 6-12)
// These journeys map to existing implemented components:
// - DisbursementProcessingWorkflow, PaymentsReconciliationWorkflow, DisputeResolutionWorkflow
// - TigerBeetle middleware, Mojaloop FSPIOP client
// - UI: Disbursements page
// - BFF: disbursements router

// Journey 6: Disbursement Scheduling
// UI Entry: Admin Disbursements page
// BFF: trpc.disbursements.schedule
// Workflow: DisbursementProcessingWorkflow
type DisbursementScheduleInput struct {
	JourneyContext   *JourneyContext       `json:"journeyContext"`
	ProgramID        string                `json:"programId"`
	DisbursementName string                `json:"disbursementName"`
	Amount           float64               `json:"amount"`
	Currency         string                `json:"currency"`
	ScheduledDate    string                `json:"scheduledDate"`
	BeneficiaryIDs   []string              `json:"beneficiaryIds,omitempty"`
	Criteria         *DisbursementCriteria `json:"criteria,omitempty"`
}

type DisbursementCriteria struct {
	Status      string   `json:"status,omitempty"`
	Regions     []string `json:"regions,omitempty"`
	Districts   []string `json:"districts,omitempty"`
	PMTScoreMin float64  `json:"pmtScoreMin,omitempty"`
	PMTScoreMax float64  `json:"pmtScoreMax,omitempty"`
}

type DisbursementScheduleResult struct {
	JourneyRunID       string    `json:"journeyRunId"`
	DisbursementID     string    `json:"disbursementId"`
	ProgramID          string    `json:"programId"`
	TotalBeneficiaries int       `json:"totalBeneficiaries"`
	TotalAmount        float64   `json:"totalAmount"`
	ScheduledDate      time.Time `json:"scheduledDate"`
	Status             string    `json:"status"`
}

// DisbursementScheduleJourney orchestrates scheduling a disbursement
func DisbursementScheduleJourney(ctx workflow.Context, input DisbursementScheduleInput) (*DisbursementScheduleResult, error) {
	jc := input.JourneyContext

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    time.Minute,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Step 1: Emit journey started event
	workflow.ExecuteActivity(ctx, EmitJourneyEventActivity, jc, "journey.started", map[string]interface{}{
		"journeyType": "disbursement_schedule",
		"programId":   input.ProgramID,
	})

	// Step 2: Check authorization
	var authorized bool
	err := workflow.ExecuteActivity(ctx, CheckAuthorizationActivity, jc, "disbursement:create").Get(ctx, &authorized)
	if err != nil || !authorized {
		return nil, fmt.Errorf("authorization failed")
	}

	// Step 3: Validate program budget
	var budgetValid bool
	err = workflow.ExecuteActivity(ctx, ValidateProgramBudgetActivity, input.ProgramID, input.Amount).Get(ctx, &budgetValid)
	if err != nil || !budgetValid {
		return nil, fmt.Errorf("insufficient program budget")
	}

	// Step 4: Get eligible beneficiaries
	var beneficiaryIDs []string
	if len(input.BeneficiaryIDs) > 0 {
		beneficiaryIDs = input.BeneficiaryIDs
	} else if input.Criteria != nil {
		err = workflow.ExecuteActivity(ctx, GetEligibleBeneficiariesActivity, input.ProgramID, input.Criteria).Get(ctx, &beneficiaryIDs)
		if err != nil {
			return nil, fmt.Errorf("failed to get eligible beneficiaries: %v", err)
		}
	} else {
		return nil, fmt.Errorf("no beneficiaries specified")
	}

	if len(beneficiaryIDs) == 0 {
		return nil, fmt.Errorf("no eligible beneficiaries found")
	}

	totalAmount := input.Amount * float64(len(beneficiaryIDs))

	// Step 5: Create disbursement record
	var disbursementID string
	err = workflow.ExecuteActivity(ctx, CreateDisbursementRecordActivity, map[string]interface{}{
		"programId":            input.ProgramID,
		"name":                 input.DisbursementName,
		"amountPerBeneficiary": input.Amount,
		"currency":             input.Currency,
		"totalAmount":          totalAmount,
		"beneficiaryCount":     len(beneficiaryIDs),
		"scheduledDate":        input.ScheduledDate,
		"status":               "scheduled",
		"createdBy":            jc.ActorID,
		"tenantId":             jc.TenantID,
	}).Get(ctx, &disbursementID)
	if err != nil {
		return nil, fmt.Errorf("failed to create disbursement: %v", err)
	}

	// Step 6: Create disbursement items for each beneficiary
	for _, beneficiaryID := range beneficiaryIDs {
		workflow.ExecuteActivity(ctx, CreateDisbursementItemActivity, disbursementID, beneficiaryID, input.Amount, input.Currency)
	}

	// Step 7: Reserve budget in TigerBeetle (pending transfer)
	var reservationID string
	err = workflow.ExecuteActivity(ctx, CreatePendingTransferActivity, map[string]interface{}{
		"disbursementId": disbursementID,
		"programId":      input.ProgramID,
		"amount":         totalAmount,
		"currency":       input.Currency,
	}).Get(ctx, &reservationID)
	if err != nil {
		// Non-fatal: reservation can be done at execution time
		workflow.GetLogger(ctx).Warn("Failed to reserve budget", "error", err)
	}

	// Step 8: Publish Kafka event
	workflow.ExecuteActivity(ctx, PublishKafkaEventActivity, "disbursement.scheduled", map[string]interface{}{
		"disbursementId":   disbursementID,
		"programId":        input.ProgramID,
		"beneficiaryCount": len(beneficiaryIDs),
		"totalAmount":      totalAmount,
		"scheduledDate":    input.ScheduledDate,
		"correlationId":    jc.CorrelationID,
	})

	// Step 9: Create audit log
	workflow.ExecuteActivity(ctx, CreateAuditLogActivity, map[string]interface{}{
		"action":        "disbursement_scheduled",
		"entityType":    "disbursement",
		"entityId":      disbursementID,
		"performedBy":   jc.ActorID,
		"tenantId":      jc.TenantID,
		"correlationId": jc.CorrelationID,
		"details": map[string]interface{}{
			"programId":        input.ProgramID,
			"beneficiaryCount": len(beneficiaryIDs),
			"totalAmount":      totalAmount,
		},
	})

	// Step 10: Write to lakehouse
	workflow.ExecuteActivity(ctx, WriteLakehouseFactActivity, "disbursement_facts", map[string]interface{}{
		"disbursementId":   disbursementID,
		"programId":        input.ProgramID,
		"scheduledDate":    input.ScheduledDate,
		"beneficiaryCount": len(beneficiaryIDs),
		"totalAmount":      totalAmount,
		"status":           "scheduled",
		"tenantId":         jc.TenantID,
	})

	// Step 11: Emit journey completed event
	workflow.ExecuteActivity(ctx, EmitJourneyEventActivity, jc, "journey.completed", map[string]interface{}{
		"disbursementId": disbursementID,
	})

	scheduledDate, _ := time.Parse("2006-01-02", input.ScheduledDate)

	return &DisbursementScheduleResult{
		JourneyRunID:       jc.JourneyRunID,
		DisbursementID:     disbursementID,
		ProgramID:          input.ProgramID,
		TotalBeneficiaries: len(beneficiaryIDs),
		TotalAmount:        totalAmount,
		ScheduledDate:      scheduledDate,
		Status:             "scheduled",
	}, nil
}

// Journey 7: Disbursement Execution
// UI Entry: Admin Disbursements page (Execute button)
// BFF: trpc.disbursements.execute
// Workflow: DisbursementProcessingWorkflow with TigerBeetle + Mojaloop
type DisbursementExecuteInput struct {
	JourneyContext *JourneyContext `json:"journeyContext"`
	DisbursementID string          `json:"disbursementId"`
	ApprovalID     string          `json:"approvalId,omitempty"`
}

type DisbursementExecuteResult struct {
	JourneyRunID   string                   `json:"journeyRunId"`
	DisbursementID string                   `json:"disbursementId"`
	Status         string                   `json:"status"`
	SuccessCount   int                      `json:"successCount"`
	FailedCount    int                      `json:"failedCount"`
	TotalAmount    float64                  `json:"totalAmount"`
	ExecutedAt     time.Time                `json:"executedAt"`
	FailedItems    []FailedDisbursementItem `json:"failedItems,omitempty"`
}

type FailedDisbursementItem struct {
	BeneficiaryID string `json:"beneficiaryId"`
	Reason        string `json:"reason"`
}

// DisbursementExecuteJourney orchestrates executing a disbursement with two-phase commit
func DisbursementExecuteJourney(ctx workflow.Context, input DisbursementExecuteInput) (*DisbursementExecuteResult, error) {
	jc := input.JourneyContext

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    time.Minute,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Step 1: Emit journey started event
	workflow.ExecuteActivity(ctx, EmitJourneyEventActivity, jc, "journey.started", map[string]interface{}{
		"journeyType":    "disbursement_execute",
		"disbursementId": input.DisbursementID,
	})

	// Step 2: Check authorization
	var authorized bool
	err := workflow.ExecuteActivity(ctx, CheckAuthorizationActivity, jc, "disbursement:execute").Get(ctx, &authorized)
	if err != nil || !authorized {
		return nil, fmt.Errorf("authorization failed")
	}

	// Step 3: Get disbursement details
	var disbursement map[string]interface{}
	err = workflow.ExecuteActivity(ctx, GetDisbursementDetailsActivity, input.DisbursementID).Get(ctx, &disbursement)
	if err != nil {
		return nil, fmt.Errorf("disbursement not found: %v", err)
	}

	status := disbursement["status"].(string)
	if status != "scheduled" && status != "approved" {
		return nil, fmt.Errorf("disbursement cannot be executed: status is %s", status)
	}

	// Step 4: Update status to processing
	workflow.ExecuteActivity(ctx, UpdateDisbursementStatusActivity, input.DisbursementID, "processing")

	// Step 5: Get disbursement items
	var items []map[string]interface{}
	err = workflow.ExecuteActivity(ctx, GetDisbursementItemsActivity, input.DisbursementID).Get(ctx, &items)
	if err != nil {
		return nil, fmt.Errorf("failed to get disbursement items: %v", err)
	}

	// Step 6: Process each item with two-phase commit
	successCount := 0
	failedCount := 0
	var failedItems []FailedDisbursementItem
	totalAmount := 0.0

	for _, item := range items {
		beneficiaryID := item["beneficiaryId"].(string)
		amount := item["amount"].(float64)

		// Phase 1: Create pending transfer in TigerBeetle
		var pendingTransferID string
		err = workflow.ExecuteActivity(ctx, CreatePendingTransferActivity, map[string]interface{}{
			"disbursementId": input.DisbursementID,
			"beneficiaryId":  beneficiaryID,
			"amount":         amount,
		}).Get(ctx, &pendingTransferID)
		if err != nil {
			failedItems = append(failedItems, FailedDisbursementItem{
				BeneficiaryID: beneficiaryID,
				Reason:        fmt.Sprintf("pending transfer failed: %v", err),
			})
			failedCount++
			continue
		}

		// Phase 2: Execute external payment via Mojaloop
		var paymentResult map[string]interface{}
		err = workflow.ExecuteActivity(ctx, ExecuteMojaloopTransferActivity, map[string]interface{}{
			"beneficiaryId":     beneficiaryID,
			"amount":            amount,
			"pendingTransferId": pendingTransferID,
			"correlationId":     jc.CorrelationID,
		}).Get(ctx, &paymentResult)

		if err != nil {
			// Void the pending transfer
			workflow.ExecuteActivity(ctx, VoidPendingTransferActivity, pendingTransferID)
			failedItems = append(failedItems, FailedDisbursementItem{
				BeneficiaryID: beneficiaryID,
				Reason:        fmt.Sprintf("payment failed: %v", err),
			})
			failedCount++
			continue
		}

		// Phase 3: Commit the pending transfer
		err = workflow.ExecuteActivity(ctx, CommitPendingTransferActivity, pendingTransferID).Get(ctx, nil)
		if err != nil {
			workflow.GetLogger(ctx).Error("Failed to commit transfer", "error", err)
		}

		// Update item status
		workflow.ExecuteActivity(ctx, UpdateDisbursementItemStatusActivity, item["id"], "completed", paymentResult["transactionId"])

		successCount++
		totalAmount += amount
	}

	// Step 7: Update disbursement status
	finalStatus := "completed"
	if failedCount > 0 && successCount == 0 {
		finalStatus = "failed"
	} else if failedCount > 0 {
		finalStatus = "partial"
	}

	workflow.ExecuteActivity(ctx, UpdateDisbursementStatusActivity, input.DisbursementID, finalStatus)

	// Step 8: Publish Kafka event
	workflow.ExecuteActivity(ctx, PublishKafkaEventActivity, "disbursement.executed", map[string]interface{}{
		"disbursementId": input.DisbursementID,
		"status":         finalStatus,
		"successCount":   successCount,
		"failedCount":    failedCount,
		"totalAmount":    totalAmount,
		"correlationId":  jc.CorrelationID,
	})

	// Step 9: Create audit log
	workflow.ExecuteActivity(ctx, CreateAuditLogActivity, map[string]interface{}{
		"action":        "disbursement_executed",
		"entityType":    "disbursement",
		"entityId":      input.DisbursementID,
		"performedBy":   jc.ActorID,
		"tenantId":      jc.TenantID,
		"correlationId": jc.CorrelationID,
		"details": map[string]interface{}{
			"status":       finalStatus,
			"successCount": successCount,
			"failedCount":  failedCount,
			"totalAmount":  totalAmount,
		},
	})

	// Step 10: Write to lakehouse
	workflow.ExecuteActivity(ctx, WriteLakehouseFactActivity, "disbursement_execution_facts", map[string]interface{}{
		"disbursementId": input.DisbursementID,
		"executionDate":  time.Now().Format("2006-01-02"),
		"status":         finalStatus,
		"successCount":   successCount,
		"failedCount":    failedCount,
		"totalAmount":    totalAmount,
		"tenantId":       jc.TenantID,
	})

	// Step 11: Emit journey completed event
	workflow.ExecuteActivity(ctx, EmitJourneyEventActivity, jc, "journey.completed", map[string]interface{}{
		"disbursementId": input.DisbursementID,
		"status":         finalStatus,
	})

	return &DisbursementExecuteResult{
		JourneyRunID:   jc.JourneyRunID,
		DisbursementID: input.DisbursementID,
		Status:         finalStatus,
		SuccessCount:   successCount,
		FailedCount:    failedCount,
		TotalAmount:    totalAmount,
		ExecutedAt:     time.Now(),
		FailedItems:    failedItems,
	}, nil
}

// Journey 8: Retry Failed Disbursements
// UI Entry: Admin Disbursements page (Retry button)
// BFF: trpc.disbursements.retry
// Workflow: RetryFailedDisbursementsWorkflow
type RetryDisbursementInput struct {
	JourneyContext *JourneyContext `json:"journeyContext"`
	DisbursementID string          `json:"disbursementId"`
	ItemIDs        []string        `json:"itemIds,omitempty"` // If empty, retry all failed
}

type RetryDisbursementResult struct {
	JourneyRunID   string    `json:"journeyRunId"`
	DisbursementID string    `json:"disbursementId"`
	RetriedCount   int       `json:"retriedCount"`
	SuccessCount   int       `json:"successCount"`
	FailedCount    int       `json:"failedCount"`
	RetriedAt      time.Time `json:"retriedAt"`
}

// RetryDisbursementJourney orchestrates retrying failed disbursement items
func RetryDisbursementJourney(ctx workflow.Context, input RetryDisbursementInput) (*RetryDisbursementResult, error) {
	jc := input.JourneyContext

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    time.Minute,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Step 1: Emit journey started event
	workflow.ExecuteActivity(ctx, EmitJourneyEventActivity, jc, "journey.started", map[string]interface{}{
		"journeyType":    "retry_disbursement",
		"disbursementId": input.DisbursementID,
	})

	// Step 2: Check authorization
	var authorized bool
	err := workflow.ExecuteActivity(ctx, CheckAuthorizationActivity, jc, "disbursement:retry").Get(ctx, &authorized)
	if err != nil || !authorized {
		return nil, fmt.Errorf("authorization failed")
	}

	// Step 3: Get failed items
	var failedItems []map[string]interface{}
	if len(input.ItemIDs) > 0 {
		err = workflow.ExecuteActivity(ctx, GetDisbursementItemsByIDsActivity, input.ItemIDs).Get(ctx, &failedItems)
	} else {
		err = workflow.ExecuteActivity(ctx, GetFailedDisbursementItemsActivity, input.DisbursementID).Get(ctx, &failedItems)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get items: %v", err)
	}

	if len(failedItems) == 0 {
		return nil, fmt.Errorf("no failed items to retry")
	}

	// Step 4: Retry each failed item
	successCount := 0
	failedCount := 0

	for _, item := range failedItems {
		beneficiaryID := item["beneficiaryId"].(string)
		amount := item["amount"].(float64)
		itemID := item["id"].(string)

		// Update item status to retrying
		workflow.ExecuteActivity(ctx, UpdateDisbursementItemStatusActivity, itemID, "retrying", nil)

		// Create pending transfer
		var pendingTransferID string
		err = workflow.ExecuteActivity(ctx, CreatePendingTransferActivity, map[string]interface{}{
			"disbursementId": input.DisbursementID,
			"beneficiaryId":  beneficiaryID,
			"amount":         amount,
		}).Get(ctx, &pendingTransferID)
		if err != nil {
			workflow.ExecuteActivity(ctx, UpdateDisbursementItemStatusActivity, itemID, "failed", nil)
			failedCount++
			continue
		}

		// Execute payment
		var paymentResult map[string]interface{}
		err = workflow.ExecuteActivity(ctx, ExecuteMojaloopTransferActivity, map[string]interface{}{
			"beneficiaryId":     beneficiaryID,
			"amount":            amount,
			"pendingTransferId": pendingTransferID,
			"correlationId":     jc.CorrelationID,
		}).Get(ctx, &paymentResult)

		if err != nil {
			workflow.ExecuteActivity(ctx, VoidPendingTransferActivity, pendingTransferID)
			workflow.ExecuteActivity(ctx, UpdateDisbursementItemStatusActivity, itemID, "failed", nil)
			failedCount++
			continue
		}

		// Commit transfer
		workflow.ExecuteActivity(ctx, CommitPendingTransferActivity, pendingTransferID)
		workflow.ExecuteActivity(ctx, UpdateDisbursementItemStatusActivity, itemID, "completed", paymentResult["transactionId"])
		successCount++
	}

	// Step 5: Update disbursement status if all items now completed
	var remainingFailed int
	workflow.ExecuteActivity(ctx, CountFailedDisbursementItemsActivity, input.DisbursementID).Get(ctx, &remainingFailed)
	if remainingFailed == 0 {
		workflow.ExecuteActivity(ctx, UpdateDisbursementStatusActivity, input.DisbursementID, "completed")
	}

	// Step 6: Publish Kafka event
	workflow.ExecuteActivity(ctx, PublishKafkaEventActivity, "disbursement.retried", map[string]interface{}{
		"disbursementId": input.DisbursementID,
		"retriedCount":   len(failedItems),
		"successCount":   successCount,
		"failedCount":    failedCount,
		"correlationId":  jc.CorrelationID,
	})

	// Step 7: Create audit log
	workflow.ExecuteActivity(ctx, CreateAuditLogActivity, map[string]interface{}{
		"action":        "disbursement_retried",
		"entityType":    "disbursement",
		"entityId":      input.DisbursementID,
		"performedBy":   jc.ActorID,
		"tenantId":      jc.TenantID,
		"correlationId": jc.CorrelationID,
		"details": map[string]interface{}{
			"retriedCount": len(failedItems),
			"successCount": successCount,
			"failedCount":  failedCount,
		},
	})

	// Step 8: Emit journey completed event
	workflow.ExecuteActivity(ctx, EmitJourneyEventActivity, jc, "journey.completed", map[string]interface{}{
		"disbursementId": input.DisbursementID,
		"successCount":   successCount,
	})

	return &RetryDisbursementResult{
		JourneyRunID:   jc.JourneyRunID,
		DisbursementID: input.DisbursementID,
		RetriedCount:   len(failedItems),
		SuccessCount:   successCount,
		FailedCount:    failedCount,
		RetriedAt:      time.Now(),
	}, nil
}

// Journey 9: Payments Reconciliation
// UI Entry: Admin Reports page
// BFF: trpc.disbursements.reconcile
// Workflow: PaymentsReconciliationWorkflow
type ReconciliationInput struct {
	JourneyContext *JourneyContext `json:"journeyContext"`
	StartDate      string          `json:"startDate"`
	EndDate        string          `json:"endDate"`
	ProgramID      string          `json:"programId,omitempty"`
}

type ReconciliationResult struct {
	JourneyRunID      string    `json:"journeyRunId"`
	ReconciliationID  string    `json:"reconciliationId"`
	TotalTransactions int       `json:"totalTransactions"`
	MatchedCount      int       `json:"matchedCount"`
	MismatchedCount   int       `json:"mismatchedCount"`
	UnreconciledCount int       `json:"unreconciledCount"`
	TotalAmount       float64   `json:"totalAmount"`
	ReconciledAt      time.Time `json:"reconciledAt"`
}

// ReconciliationJourney orchestrates payment reconciliation
func ReconciliationJourney(ctx workflow.Context, input ReconciliationInput) (*ReconciliationResult, error) {
	jc := input.JourneyContext

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 60 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    time.Minute,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Step 1: Emit journey started event
	workflow.ExecuteActivity(ctx, EmitJourneyEventActivity, jc, "journey.started", map[string]interface{}{
		"journeyType": "reconciliation",
		"startDate":   input.StartDate,
		"endDate":     input.EndDate,
	})

	// Step 2: Check authorization
	var authorized bool
	err := workflow.ExecuteActivity(ctx, CheckAuthorizationActivity, jc, "reconciliation:run").Get(ctx, &authorized)
	if err != nil || !authorized {
		return nil, fmt.Errorf("authorization failed")
	}

	// Step 3: Create reconciliation record
	var reconciliationID string
	err = workflow.ExecuteActivity(ctx, CreateReconciliationRecordActivity, map[string]interface{}{
		"startDate": input.StartDate,
		"endDate":   input.EndDate,
		"programId": input.ProgramID,
		"status":    "processing",
		"createdBy": jc.ActorID,
		"tenantId":  jc.TenantID,
	}).Get(ctx, &reconciliationID)
	if err != nil {
		return nil, fmt.Errorf("failed to create reconciliation: %v", err)
	}

	// Step 4: Get internal transactions from TigerBeetle
	var internalTxns []map[string]interface{}
	err = workflow.ExecuteActivity(ctx, GetTigerBeetleTransactionsActivity, input.StartDate, input.EndDate, input.ProgramID).Get(ctx, &internalTxns)
	if err != nil {
		return nil, fmt.Errorf("failed to get internal transactions: %v", err)
	}

	// Step 5: Get external transactions from Mojaloop/bank
	var externalTxns []map[string]interface{}
	err = workflow.ExecuteActivity(ctx, GetExternalTransactionsActivity, input.StartDate, input.EndDate).Get(ctx, &externalTxns)
	if err != nil {
		return nil, fmt.Errorf("failed to get external transactions: %v", err)
	}

	// Step 6: Match transactions
	var matchResult map[string]interface{}
	err = workflow.ExecuteActivity(ctx, MatchTransactionsActivity, internalTxns, externalTxns).Get(ctx, &matchResult)
	if err != nil {
		return nil, fmt.Errorf("failed to match transactions: %v", err)
	}

	matchedCount := int(matchResult["matchedCount"].(float64))
	mismatchedCount := int(matchResult["mismatchedCount"].(float64))
	unreconciledCount := int(matchResult["unreconciledCount"].(float64))
	totalAmount := matchResult["totalAmount"].(float64)

	// Step 7: Store reconciliation results
	workflow.ExecuteActivity(ctx, StoreReconciliationResultsActivity, reconciliationID, matchResult)

	// Step 8: Update reconciliation status
	status := "completed"
	if mismatchedCount > 0 || unreconciledCount > 0 {
		status = "requires_review"
	}
	workflow.ExecuteActivity(ctx, UpdateReconciliationStatusActivity, reconciliationID, status)

	// Step 9: Publish Kafka event
	workflow.ExecuteActivity(ctx, PublishKafkaEventActivity, "reconciliation.completed", map[string]interface{}{
		"reconciliationId":  reconciliationID,
		"matchedCount":      matchedCount,
		"mismatchedCount":   mismatchedCount,
		"unreconciledCount": unreconciledCount,
		"correlationId":     jc.CorrelationID,
	})

	// Step 10: Create audit log
	workflow.ExecuteActivity(ctx, CreateAuditLogActivity, map[string]interface{}{
		"action":        "reconciliation_completed",
		"entityType":    "reconciliation",
		"entityId":      reconciliationID,
		"performedBy":   jc.ActorID,
		"tenantId":      jc.TenantID,
		"correlationId": jc.CorrelationID,
		"details": map[string]interface{}{
			"startDate":         input.StartDate,
			"endDate":           input.EndDate,
			"matchedCount":      matchedCount,
			"mismatchedCount":   mismatchedCount,
			"unreconciledCount": unreconciledCount,
		},
	})

	// Step 11: Write to lakehouse
	workflow.ExecuteActivity(ctx, WriteLakehouseFactActivity, "reconciliation_facts", map[string]interface{}{
		"reconciliationId":   reconciliationID,
		"reconciliationDate": time.Now().Format("2006-01-02"),
		"startDate":          input.StartDate,
		"endDate":            input.EndDate,
		"matchedCount":       matchedCount,
		"mismatchedCount":    mismatchedCount,
		"unreconciledCount":  unreconciledCount,
		"totalAmount":        totalAmount,
		"tenantId":           jc.TenantID,
	})

	// Step 12: Emit journey completed event
	workflow.ExecuteActivity(ctx, EmitJourneyEventActivity, jc, "journey.completed", map[string]interface{}{
		"reconciliationId": reconciliationID,
		"status":           status,
	})

	return &ReconciliationResult{
		JourneyRunID:      jc.JourneyRunID,
		ReconciliationID:  reconciliationID,
		TotalTransactions: len(internalTxns),
		MatchedCount:      matchedCount,
		MismatchedCount:   mismatchedCount,
		UnreconciledCount: unreconciledCount,
		TotalAmount:       totalAmount,
		ReconciledAt:      time.Now(),
	}, nil
}

// Journey 10: Dispute Resolution
// UI Entry: Admin Reconciliation detail page
// BFF: trpc.disbursements.resolveDispute
// Workflow: DisputeResolutionWorkflow
type DisputeResolutionInput struct {
	JourneyContext   *JourneyContext `json:"journeyContext"`
	DisputeID        string          `json:"disputeId"`
	Resolution       string          `json:"resolution"` // "accept_internal", "accept_external", "manual_adjustment"
	AdjustmentAmount float64         `json:"adjustmentAmount,omitempty"`
	Notes            string          `json:"notes"`
}

type DisputeResolutionResult struct {
	JourneyRunID string    `json:"journeyRunId"`
	DisputeID    string    `json:"disputeId"`
	Resolution   string    `json:"resolution"`
	ResolvedAt   time.Time `json:"resolvedAt"`
}

// DisputeResolutionJourney orchestrates resolving a payment dispute
func DisputeResolutionJourney(ctx workflow.Context, input DisputeResolutionInput) (*DisputeResolutionResult, error) {
	jc := input.JourneyContext

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    time.Minute,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Step 1: Emit journey started event
	workflow.ExecuteActivity(ctx, EmitJourneyEventActivity, jc, "journey.started", map[string]interface{}{
		"journeyType": "dispute_resolution",
		"disputeId":   input.DisputeID,
	})

	// Step 2: Check authorization
	var authorized bool
	err := workflow.ExecuteActivity(ctx, CheckAuthorizationActivity, jc, "dispute:resolve").Get(ctx, &authorized)
	if err != nil || !authorized {
		return nil, fmt.Errorf("authorization failed")
	}

	// Step 3: Get dispute details
	var dispute map[string]interface{}
	err = workflow.ExecuteActivity(ctx, GetDisputeDetailsActivity, input.DisputeID).Get(ctx, &dispute)
	if err != nil {
		return nil, fmt.Errorf("dispute not found: %v", err)
	}

	// Step 4: Apply resolution
	switch input.Resolution {
	case "accept_internal":
		// No adjustment needed, internal record is correct
		workflow.ExecuteActivity(ctx, MarkExternalRecordAsReconciledActivity, dispute["externalTransactionId"])
	case "accept_external":
		// Adjust internal record to match external
		workflow.ExecuteActivity(ctx, AdjustTigerBeetleBalanceActivity, dispute["accountId"], dispute["externalAmount"])
	case "manual_adjustment":
		// Create adjustment transaction
		workflow.ExecuteActivity(ctx, CreateAdjustmentTransactionActivity, map[string]interface{}{
			"disputeId": input.DisputeID,
			"accountId": dispute["accountId"],
			"amount":    input.AdjustmentAmount,
			"reason":    input.Notes,
		})
	}

	// Step 5: Update dispute status
	workflow.ExecuteActivity(ctx, UpdateDisputeStatusActivity, input.DisputeID, "resolved", input.Resolution, input.Notes)

	// Step 6: Publish Kafka event
	workflow.ExecuteActivity(ctx, PublishKafkaEventActivity, "dispute.resolved", map[string]interface{}{
		"disputeId":     input.DisputeID,
		"resolution":    input.Resolution,
		"correlationId": jc.CorrelationID,
	})

	// Step 7: Create audit log
	workflow.ExecuteActivity(ctx, CreateAuditLogActivity, map[string]interface{}{
		"action":        "dispute_resolved",
		"entityType":    "dispute",
		"entityId":      input.DisputeID,
		"performedBy":   jc.ActorID,
		"tenantId":      jc.TenantID,
		"correlationId": jc.CorrelationID,
		"details": map[string]interface{}{
			"resolution":       input.Resolution,
			"adjustmentAmount": input.AdjustmentAmount,
			"notes":            input.Notes,
		},
	})

	// Step 8: Emit journey completed event
	workflow.ExecuteActivity(ctx, EmitJourneyEventActivity, jc, "journey.completed", map[string]interface{}{
		"disputeId":  input.DisputeID,
		"resolution": input.Resolution,
	})

	return &DisputeResolutionResult{
		JourneyRunID: jc.JourneyRunID,
		DisputeID:    input.DisputeID,
		Resolution:   input.Resolution,
		ResolvedAt:   time.Now(),
	}, nil
}

// RegisterPaymentJourneys registers all payment journey definitions
func RegisterPaymentJourneys(registry *JourneyRegistry) {
	registry.Register(&JourneyDefinition{
		Key:                 "disbursement_schedule",
		Name:                "Disbursement Scheduling",
		Description:         "Schedule a disbursement for a program cohort",
		Category:            "payments",
		WorkflowType:        "DisbursementScheduleJourney",
		RequiredPermissions: []string{"disbursement:create"},
		UIEntryPoints:       []string{"Admin:DisbursementsPage"},
		BFFEndpoints:        []string{"trpc.disbursements.schedule"},
		MiddlewareHooks:     []string{"kafka", "redis", "permify", "tigerbeetle", "lakehouse"},
	})

	registry.Register(&JourneyDefinition{
		Key:                 "disbursement_execute",
		Name:                "Disbursement Execution",
		Description:         "Execute a scheduled disbursement with two-phase commit",
		Category:            "payments",
		WorkflowType:        "DisbursementExecuteJourney",
		RequiredPermissions: []string{"disbursement:execute"},
		UIEntryPoints:       []string{"Admin:DisbursementsPage"},
		BFFEndpoints:        []string{"trpc.disbursements.execute"},
		MiddlewareHooks:     []string{"kafka", "redis", "permify", "tigerbeetle", "mojaloop", "lakehouse"},
	})

	registry.Register(&JourneyDefinition{
		Key:                 "retry_disbursement",
		Name:                "Retry Failed Disbursements",
		Description:         "Retry failed disbursement items",
		Category:            "payments",
		WorkflowType:        "RetryDisbursementJourney",
		RequiredPermissions: []string{"disbursement:retry"},
		UIEntryPoints:       []string{"Admin:DisbursementsPage"},
		BFFEndpoints:        []string{"trpc.disbursements.retry"},
		MiddlewareHooks:     []string{"kafka", "redis", "permify", "tigerbeetle", "mojaloop"},
	})

	registry.Register(&JourneyDefinition{
		Key:                 "reconciliation",
		Name:                "Payments Reconciliation",
		Description:         "Reconcile internal ledger with external payment records",
		Category:            "payments",
		WorkflowType:        "ReconciliationJourney",
		RequiredPermissions: []string{"reconciliation:run"},
		UIEntryPoints:       []string{"Admin:ReportsPage"},
		BFFEndpoints:        []string{"trpc.disbursements.reconcile"},
		MiddlewareHooks:     []string{"kafka", "redis", "permify", "tigerbeetle", "mojaloop", "lakehouse"},
	})

	registry.Register(&JourneyDefinition{
		Key:                 "dispute_resolution",
		Name:                "Dispute Resolution",
		Description:         "Resolve payment disputes from reconciliation",
		Category:            "payments",
		WorkflowType:        "DisputeResolutionJourney",
		RequiredPermissions: []string{"dispute:resolve"},
		UIEntryPoints:       []string{"Admin:ReconciliationDetailPage"},
		BFFEndpoints:        []string{"trpc.disbursements.resolveDispute"},
		MiddlewareHooks:     []string{"kafka", "redis", "permify", "tigerbeetle"},
	})
}

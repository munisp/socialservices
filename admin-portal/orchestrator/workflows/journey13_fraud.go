package workflows

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// ========== Journey 13: Fraud Investigation ==========

type FraudInvestigationInput struct {
	InvestigationID string   `json:"investigationID"`
	AlertID         string   `json:"alertID"`
	BeneficiaryID   string   `json:"beneficiaryID"`
	FraudType       string   `json:"fraudType"` // duplicate_identity, fake_documents, transaction_fraud, collusion
	Severity        string   `json:"severity"`  // low, medium, high, critical
	Evidence        []string `json:"evidence"`
	InitiatedBy     string   `json:"initiatedBy"`
}

type FraudInvestigationResult struct {
	InvestigationID string                 `json:"investigationID"`
	CaseNumber      string                 `json:"caseNumber"`
	Status          string                 `json:"status"`
	RiskScore       float64                `json:"riskScore"`
	Findings        []Finding              `json:"findings"`
	Recommendation  string                 `json:"recommendation"`
	ActionTaken     string                 `json:"actionTaken"`
	RecoveryAmount  float64                `json:"recoveryAmount"`
	ClosureDate     time.Time              `json:"closureDate,omitempty"`
	Message         string                 `json:"message"`
}

type Finding struct {
	Type        string    `json:"type"`
	Description string    `json:"description"`
	Evidence    []string  `json:"evidence"`
	Timestamp   time.Time `json:"timestamp"`
}

func FraudInvestigationWorkflow(ctx workflow.Context, input FraudInvestigationInput) (*FraudInvestigationResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting fraud investigation workflow", "investigationID", input.InvestigationID)

	activityOptions := workflow.ActivityOptions{
		StartToCloseTimeout: 60 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    5 * time.Minute,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, activityOptions)

	var result FraudInvestigationResult
	result.InvestigationID = input.InvestigationID

	// Step 1: Generate case number
	var caseNumber string
	err := workflow.ExecuteActivity(ctx, "GenerateFraudCaseNumberActivity", input.FraudType).Get(ctx, &caseNumber)
	if err != nil {
		return &result, err
	}
	result.CaseNumber = caseNumber

	// Step 2: Create investigation record
	err = workflow.ExecuteActivity(ctx, "CreateInvestigationRecordActivity", input.InvestigationID, caseNumber, input.BeneficiaryID, input.FraudType, input.Severity).Get(ctx, nil)
	if err != nil {
		return &result, err
	}

	// Step 3: Suspend beneficiary account immediately if severity is high/critical
	if input.Severity == "high" || input.Severity == "critical" {
		err = workflow.ExecuteActivity(ctx, "SuspendBeneficiaryAccountActivity", input.BeneficiaryID, "Fraud investigation in progress").Get(ctx, nil)
		if err != nil {
			logger.Warn("Failed to suspend account", "error", err)
		}
		result.ActionTaken = "Account suspended"
	}

	// Step 4: Collect evidence from multiple sources
	var evidenceData map[string]interface{}
	err = workflow.ExecuteActivity(ctx, "CollectEvidenceActivity", input.BeneficiaryID, input.AlertID, input.Evidence).Get(ctx, &evidenceData)
	if err != nil {
		logger.Warn("Failed to collect evidence", "error", err)
	}

	// Step 5: Run ML fraud detection analysis
	var mlAnalysisResult map[string]interface{}
	err = workflow.ExecuteActivity(ctx, "RunMLFraudAnalysisActivity", input.BeneficiaryID, evidenceData).Get(ctx, &mlAnalysisResult)
	if err != nil {
		logger.Warn("ML analysis failed", "error", err)
	} else {
		if score, ok := mlAnalysisResult["risk_score"].(float64); ok {
			result.RiskScore = score
		}
	}

	// Step 6: Cross-check with other beneficiaries for patterns
	var relatedCases []string
	err = workflow.ExecuteActivity(ctx, "FindRelatedFraudCasesActivity", input.BeneficiaryID, input.FraudType).Get(ctx, &relatedCases)
	if err != nil {
		logger.Warn("Failed to find related cases", "error", err)
	}

	// Step 7: Verify documents with external databases
	var documentVerification map[string]bool
	err = workflow.ExecuteActivity(ctx, "VerifyDocumentsExternalActivity", input.BeneficiaryID).Get(ctx, &documentVerification)
	if err != nil {
		logger.Warn("Document verification failed", "error", err)
	}

	// Step 8: Analyze transaction patterns
	var transactionAnalysis map[string]interface{}
	err = workflow.ExecuteActivity(ctx, "AnalyzeTransactionPatternsActivity", input.BeneficiaryID).Get(ctx, &transactionAnalysis)
	if err != nil {
		logger.Warn("Transaction analysis failed", "error", err)
	}

	// Step 9: Compile findings
	var findings []Finding

	// Add document verification findings
	if documentVerification != nil {
		for docType, isValid := range documentVerification {
			if !isValid {
				findings = append(findings, Finding{
					Type:        "document_fraud",
					Description: fmt.Sprintf("Invalid %s detected", docType),
					Evidence:    input.Evidence,
					Timestamp:   time.Now(),
				})
			}
		}
	}

	// Add transaction pattern findings
	if transactionAnalysis != nil {
		if suspicious, ok := transactionAnalysis["suspicious_patterns"].(bool); ok && suspicious {
			findings = append(findings, Finding{
				Type:        "transaction_fraud",
				Description: "Suspicious transaction patterns detected",
				Evidence:    []string{"transaction_analysis_report"},
				Timestamp:   time.Now(),
			})
		}
	}

	result.Findings = findings

	// Step 10: Calculate recovery amount if fraud confirmed
	if len(findings) > 0 && result.RiskScore > 0.7 {
		var recoveryAmount float64
		err = workflow.ExecuteActivity(ctx, "CalculateFraudRecoveryAmountActivity", input.BeneficiaryID).Get(ctx, &recoveryAmount)
		if err != nil {
			logger.Warn("Failed to calculate recovery amount", "error", err)
		}
		result.RecoveryAmount = recoveryAmount
	}

	// Step 11: Generate recommendation
	if result.RiskScore >= 0.8 {
		result.Recommendation = "Terminate beneficiary and initiate legal proceedings"
	} else if result.RiskScore >= 0.6 {
		result.Recommendation = "Suspend beneficiary pending further investigation"
	} else if result.RiskScore >= 0.4 {
		result.Recommendation = "Issue warning and monitor closely"
	} else {
		result.Recommendation = "Close case - insufficient evidence"
	}

	// Step 12: Create approval request for final action
	var approvalID string
	err = workflow.ExecuteActivity(ctx, "CreateApprovalRequestActivity", input.InvestigationID, "fraud_investigation_closure", input.InitiatedBy).Get(ctx, &approvalID)
	if err != nil {
		logger.Warn("Failed to create approval request", "error", err)
	}

	// Step 13: Wait for approval (with timeout)
	approvalSelector := workflow.NewSelector(ctx)
	var approved bool
	
	approvalChannel := workflow.GetSignalChannel(ctx, "investigation_approved")
	approvalSelector.AddReceive(approvalChannel, func(c workflow.ReceiveChannel, more bool) {
		c.Receive(ctx, &approved)
	})

	// Timeout after 7 days
	timeoutTimer := workflow.NewTimer(ctx, 7*24*time.Hour)
	approvalSelector.AddFuture(timeoutTimer, func(f workflow.Future) {
		approved = false
	})

	approvalSelector.Select(ctx)

	// Step 14: Execute final action based on approval
	if approved {
		err = workflow.ExecuteActivity(ctx, "ExecuteFraudActionActivity", input.BeneficiaryID, result.Recommendation).Get(ctx, nil)
		if err != nil {
			logger.Error("Failed to execute fraud action", "error", err)
		}
		result.ActionTaken = result.Recommendation
		result.Status = "closed"
	} else {
		result.Status = "pending_approval"
		result.ActionTaken = "Awaiting approval"
	}

	// Step 15: Publish event to Kafka
	err = workflow.ExecuteActivity(ctx, "PublishKafkaEventActivity", "fraud.investigation.completed", input.InvestigationID, result).Get(ctx, nil)
	if err != nil {
		logger.Warn("Failed to publish Kafka event", "error", err)
	}

	// Step 16: Create audit log
	err = workflow.ExecuteActivity(ctx, "CreateAuditLogActivity", "fraud.investigation.completed", input.InvestigationID, input.InitiatedBy, result).Get(ctx, nil)
	if err != nil {
		logger.Warn("Failed to create audit log", "error", err)
	}

	// Step 17: Update fraud statistics
	err = workflow.ExecuteActivity(ctx, "UpdateFraudStatisticsActivity", input.FraudType, result.RiskScore, len(findings)).Get(ctx, nil)
	if err != nil {
		logger.Warn("Failed to update statistics", "error", err)
	}

	result.ClosureDate = time.Now()
	result.Message = fmt.Sprintf("Investigation %s. Risk score: %.2f, Findings: %d", result.Status, result.RiskScore, len(findings))

	logger.Info("Fraud investigation completed", "caseNumber", caseNumber, "riskScore", result.RiskScore, "status", result.Status)
	return &result, nil
}

// ========== Journey 13: Fraud Recovery Workflow ==========

type FraudRecoveryInput struct {
	InvestigationID string  `json:"investigationID"`
	BeneficiaryID   string  `json:"beneficiaryID"`
	RecoveryAmount  float64 `json:"recoveryAmount"`
	RecoveryMethod  string  `json:"recoveryMethod"` // deduction, legal, write_off
	InitiatedBy     string  `json:"initiatedBy"`
}

type FraudRecoveryResult struct {
	RecoveryID      string  `json:"recoveryID"`
	AmountRecovered float64 `json:"amountRecovered"`
	Status          string  `json:"status"`
	Message         string  `json:"message"`
}

func FraudRecoveryWorkflow(ctx workflow.Context, input FraudRecoveryInput) (*FraudRecoveryResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting fraud recovery workflow", "investigationID", input.InvestigationID)

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

	var result FraudRecoveryResult
	result.RecoveryID = fmt.Sprintf("REC-%s", input.InvestigationID)

	// Execute recovery based on method
	switch input.RecoveryMethod {
	case "deduction":
		// Deduct from future disbursements
		var deducted float64
		err := workflow.ExecuteActivity(ctx, "DeductFromFutureDisbursementsActivity", input.BeneficiaryID, input.RecoveryAmount).Get(ctx, &deducted)
		if err != nil {
			return &result, err
		}
		result.AmountRecovered = deducted
		result.Status = "recovered"

	case "legal":
		// Initiate legal proceedings
		err := workflow.ExecuteActivity(ctx, "InitiateLegalProceedingsActivity", input.BeneficiaryID, input.RecoveryAmount).Get(ctx, nil)
		if err != nil {
			return &result, err
		}
		result.Status = "legal_proceedings"
		result.AmountRecovered = 0

	case "write_off":
		// Write off the amount
		err := workflow.ExecuteActivity(ctx, "WriteOffFraudAmountActivity", input.BeneficiaryID, input.RecoveryAmount).Get(ctx, nil)
		if err != nil {
			return &result, err
		}
		result.Status = "written_off"
		result.AmountRecovered = 0
	}

	// Update investigation record
	err := workflow.ExecuteActivity(ctx, "UpdateInvestigationRecoveryActivity", input.InvestigationID, result.AmountRecovered, result.Status).Get(ctx, nil)
	if err != nil {
		logger.Warn("Failed to update investigation record", "error", err)
	}

	// Publish event
	err = workflow.ExecuteActivity(ctx, "PublishKafkaEventActivity", "fraud.recovery.completed", result.RecoveryID, result).Get(ctx, nil)
	if err != nil {
		logger.Warn("Failed to publish Kafka event", "error", err)
	}

	result.Message = fmt.Sprintf("Recovery %s. Amount: %.2f", result.Status, result.AmountRecovered)

	return &result, nil
}

package workflows

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// ========== Journey 12: Grievance Submission & Resolution ==========

type GrievanceInput struct {
	GrievanceID   string   `json:"grievanceID"`
	BeneficiaryID string   `json:"beneficiaryID"`
	Category      string   `json:"category"` // payment, service, eligibility, fraud, other
	Description   string   `json:"description"`
	Priority      string   `json:"priority"` // low, medium, high, critical
	Attachments   []string `json:"attachments"`
	SubmittedBy   string   `json:"submittedBy"`
}

type GrievanceResult struct {
	GrievanceID     string    `json:"grievanceID"`
	CaseNumber      string    `json:"caseNumber"`
	Status          string    `json:"status"`
	AssignedTo      string    `json:"assignedTo"`
	ResolutionDate  time.Time `json:"resolutionDate,omitempty"`
	Resolution      string    `json:"resolution,omitempty"`
	EscalationLevel int       `json:"escalationLevel"`
	Message         string    `json:"message"`
}

func GrievanceSubmissionWorkflow(ctx workflow.Context, input GrievanceInput) (*GrievanceResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting grievance submission workflow", "grievanceID", input.GrievanceID)

	activityOptions := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    time.Minute,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, activityOptions)

	var result GrievanceResult
	result.GrievanceID = input.GrievanceID

	// Step 1: Validate beneficiary exists
	var beneficiaryExists bool
	err := workflow.ExecuteActivity(ctx, "ValidateBeneficiaryExistsActivity", input.BeneficiaryID).Get(ctx, &beneficiaryExists)
	if err != nil || !beneficiaryExists {
		result.Status = "rejected"
		result.Message = "Beneficiary not found"
		return &result, fmt.Errorf("beneficiary validation failed")
	}

	// Step 2: Generate case number
	var caseNumber string
	err = workflow.ExecuteActivity(ctx, "GenerateCaseNumberActivity", input.Category).Get(ctx, &caseNumber)
	if err != nil {
		return &result, err
	}
	result.CaseNumber = caseNumber

	// Step 3: Create grievance record
	err = workflow.ExecuteActivity(ctx, "CreateGrievanceRecordActivity", input.GrievanceID, caseNumber, input.BeneficiaryID, input.Category, input.Description, input.Priority).Get(ctx, nil)
	if err != nil {
		return &result, err
	}

	// Step 4: Store attachments
	if len(input.Attachments) > 0 {
		err = workflow.ExecuteActivity(ctx, "StoreGrievanceAttachmentsActivity", input.GrievanceID, input.Attachments).Get(ctx, nil)
		if err != nil {
			logger.Warn("Failed to store attachments", "error", err)
		}
	}

	// Step 5: Run ML classification for auto-routing
	var suggestedCategory string
	err = workflow.ExecuteActivity(ctx, "ClassifyGrievanceActivity", input.Description).Get(ctx, &suggestedCategory)
	if err == nil && suggestedCategory != input.Category {
		logger.Info("ML suggested different category", "original", input.Category, "suggested", suggestedCategory)
	}

	// Step 6: Assign to case worker based on category and workload
	var assignedTo string
	err = workflow.ExecuteActivity(ctx, "AssignGrievanceActivity", input.GrievanceID, input.Category, input.Priority).Get(ctx, &assignedTo)
	if err != nil {
		return &result, err
	}
	result.AssignedTo = assignedTo

	// Step 7: Create approval workflow if high priority
	if input.Priority == "high" || input.Priority == "critical" {
		var approvalID string
		err = workflow.ExecuteActivity(ctx, "CreateApprovalRequestActivity", input.GrievanceID, "grievance_review", assignedTo).Get(ctx, &approvalID)
		if err != nil {
			logger.Warn("Failed to create approval request", "error", err)
		}
	}

	// Step 8: Send notification to beneficiary
	err = workflow.ExecuteActivity(ctx, "SendNotificationActivity", input.BeneficiaryID, fmt.Sprintf("Your grievance has been received. Case number: %s", caseNumber)).Get(ctx, nil)
	if err != nil {
		logger.Warn("Failed to send notification", "error", err)
	}

	// Step 9: Send notification to assigned case worker
	err = workflow.ExecuteActivity(ctx, "SendCaseWorkerNotificationActivity", assignedTo, input.GrievanceID, caseNumber, input.Priority).Get(ctx, nil)
	if err != nil {
		logger.Warn("Failed to send case worker notification", "error", err)
	}

	// Step 10: Publish event to Kafka
	err = workflow.ExecuteActivity(ctx, "PublishKafkaEventActivity", "grievance.submitted", input.GrievanceID, result).Get(ctx, nil)
	if err != nil {
		logger.Warn("Failed to publish Kafka event", "error", err)
	}

	// Step 11: Create audit log
	err = workflow.ExecuteActivity(ctx, "CreateAuditLogActivity", "grievance.submitted", input.GrievanceID, input.SubmittedBy, result).Get(ctx, nil)
	if err != nil {
		logger.Warn("Failed to create audit log", "error", err)
	}

	// Step 12: Set up SLA timer based on priority
	var slaHours int
	switch input.Priority {
	case "critical":
		slaHours = 24
	case "high":
		slaHours = 72
	case "medium":
		slaHours = 168 // 7 days
	default:
		slaHours = 336 // 14 days
	}

	// Start timer for SLA monitoring (will trigger escalation if not resolved)
	_ = workflow.NewTimer(ctx, time.Duration(slaHours)*time.Hour)

	result.Status = "open"
	result.EscalationLevel = 0
	result.Message = fmt.Sprintf("Grievance submitted successfully. Case number: %s, assigned to: %s", caseNumber, assignedTo)

	logger.Info("Grievance submission completed", "caseNumber", caseNumber, "assignedTo", assignedTo)
	return &result, nil
}

// ========== Journey 12: Grievance Resolution Workflow ==========

type GrievanceResolutionInput struct {
	GrievanceID    string `json:"grievanceID"`
	Resolution     string `json:"resolution"`
	ResolutionType string `json:"resolutionType"` // resolved, escalated, rejected, withdrawn
	ResolvedBy     string `json:"resolvedBy"`
	Notes          string `json:"notes"`
}

func GrievanceResolutionWorkflow(ctx workflow.Context, input GrievanceResolutionInput) (*GrievanceResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting grievance resolution workflow", "grievanceID", input.GrievanceID)

	activityOptions := workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    time.Minute,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, activityOptions)

	var result GrievanceResult
	result.GrievanceID = input.GrievanceID

	// Step 1: Get grievance details
	var grievanceDetails map[string]interface{}
	err := workflow.ExecuteActivity(ctx, "GetGrievanceDetailsActivity", input.GrievanceID).Get(ctx, &grievanceDetails)
	if err != nil {
		return &result, err
	}

	beneficiaryID := grievanceDetails["beneficiary_id"].(string)
	result.CaseNumber = grievanceDetails["case_number"].(string)

	// Step 2: Update grievance status and resolution
	err = workflow.ExecuteActivity(ctx, "UpdateGrievanceResolutionActivity", input.GrievanceID, input.Resolution, input.ResolutionType, input.ResolvedBy, input.Notes).Get(ctx, nil)
	if err != nil {
		return &result, err
	}

	result.Status = input.ResolutionType
	result.Resolution = input.Resolution
	result.ResolutionDate = time.Now()

	// Step 3: If escalated, reassign to higher level
	if input.ResolutionType == "escalated" {
		var newAssignee string
		err = workflow.ExecuteActivity(ctx, "EscalateGrievanceActivity", input.GrievanceID).Get(ctx, &newAssignee)
		if err != nil {
			logger.Warn("Failed to escalate grievance", "error", err)
		} else {
			result.AssignedTo = newAssignee
			result.EscalationLevel++
		}
	}

	// Step 4: Send notification to beneficiary
	err = workflow.ExecuteActivity(ctx, "SendNotificationActivity", beneficiaryID, fmt.Sprintf("Your grievance (Case: %s) has been %s. Resolution: %s", result.CaseNumber, input.ResolutionType, input.Resolution)).Get(ctx, nil)
	if err != nil {
		logger.Warn("Failed to send notification", "error", err)
	}

	// Step 5: Publish event to Kafka
	err = workflow.ExecuteActivity(ctx, "PublishKafkaEventActivity", "grievance.resolved", input.GrievanceID, result).Get(ctx, nil)
	if err != nil {
		logger.Warn("Failed to publish Kafka event", "error", err)
	}

	// Step 6: Create audit log
	err = workflow.ExecuteActivity(ctx, "CreateAuditLogActivity", "grievance.resolved", input.GrievanceID, input.ResolvedBy, result).Get(ctx, nil)
	if err != nil {
		logger.Warn("Failed to create audit log", "error", err)
	}

	// Step 7: Update case worker performance metrics
	err = workflow.ExecuteActivity(ctx, "UpdateCaseWorkerMetricsActivity", input.ResolvedBy, input.ResolutionType).Get(ctx, nil)
	if err != nil {
		logger.Warn("Failed to update metrics", "error", err)
	}

	result.Message = fmt.Sprintf("Grievance %s successfully", input.ResolutionType)

	logger.Info("Grievance resolution completed", "grievanceID", input.GrievanceID, "resolutionType", input.ResolutionType)
	return &result, nil
}

// ========== Journey 12: Grievance Escalation Workflow ==========

type GrievanceEscalationInput struct {
	GrievanceID string `json:"grievanceID"`
	Reason      string `json:"reason"`
	EscalatedBy string `json:"escalatedBy"`
}

func GrievanceEscalationWorkflow(ctx workflow.Context, input GrievanceEscalationInput) (*GrievanceResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting grievance escalation workflow", "grievanceID", input.GrievanceID)

	activityOptions := workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    time.Minute,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, activityOptions)

	var result GrievanceResult
	result.GrievanceID = input.GrievanceID

	// Get current escalation level
	var currentLevel int
	err := workflow.ExecuteActivity(ctx, "GetGrievanceEscalationLevelActivity", input.GrievanceID).Get(ctx, &currentLevel)
	if err != nil {
		return &result, err
	}

	// Escalate to next level
	newLevel := currentLevel + 1
	result.EscalationLevel = newLevel

	// Assign to supervisor/manager based on level
	var newAssignee string
	err = workflow.ExecuteActivity(ctx, "AssignToEscalationLevelActivity", input.GrievanceID, newLevel).Get(ctx, &newAssignee)
	if err != nil {
		return &result, err
	}
	result.AssignedTo = newAssignee

	// Update grievance record
	err = workflow.ExecuteActivity(ctx, "UpdateGrievanceEscalationActivity", input.GrievanceID, newLevel, newAssignee, input.Reason).Get(ctx, nil)
	if err != nil {
		return &result, err
	}

	// Notify new assignee
	err = workflow.ExecuteActivity(ctx, "SendEscalationNotificationActivity", newAssignee, input.GrievanceID, newLevel, input.Reason).Get(ctx, nil)
	if err != nil {
		logger.Warn("Failed to send escalation notification", "error", err)
	}

	// Publish event
	err = workflow.ExecuteActivity(ctx, "PublishKafkaEventActivity", "grievance.escalated", input.GrievanceID, result).Get(ctx, nil)
	if err != nil {
		logger.Warn("Failed to publish Kafka event", "error", err)
	}

	result.Status = "escalated"
	result.Message = fmt.Sprintf("Grievance escalated to level %d, assigned to %s", newLevel, newAssignee)

	return &result, nil
}

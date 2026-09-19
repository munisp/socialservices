package workflows

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// ProgramEnrollmentInput represents the input for program enrollment
type ProgramEnrollmentInput struct {
	BeneficiaryID string `json:"beneficiaryID"`
	ProgramID     string `json:"programID"`
	EnrolledBy    string `json:"enrolledBy"`
}

// ProgramEnrollmentResult represents the workflow result
type ProgramEnrollmentResult struct {
	BeneficiaryID  string `json:"beneficiaryID"`
	ProgramID      string `json:"programID"`
	EnrollmentID   string `json:"enrollmentID"`
	Status         string `json:"status"` // enrolled, waitlisted, rejected
	StartDate      string `json:"startDate"`
	Message        string `json:"message"`
}

// ProgramEnrollmentWorkflow orchestrates program enrollment
func ProgramEnrollmentWorkflow(ctx workflow.Context, input ProgramEnrollmentInput) (*ProgramEnrollmentResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting program enrollment workflow", "beneficiaryID", input.BeneficiaryID, "programID", input.ProgramID)

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

	var result ProgramEnrollmentResult
	result.BeneficiaryID = input.BeneficiaryID
	result.ProgramID = input.ProgramID

	// Step 1: Validate program exists and is active
	var programValid bool
	err := workflow.ExecuteActivity(ctx, "ValidateProgramActivity", input.ProgramID).Get(ctx, &programValid)
	if err != nil || !programValid {
		result.Status = "rejected"
		result.Message = "Program not found or inactive"
		return &result, err
	}

	// Step 2: Check beneficiary eligibility for program
	var eligibilityResult map[string]interface{}
	err = workflow.ExecuteActivity(ctx, "CheckProgramEligibilityActivity", input.BeneficiaryID, input.ProgramID).Get(ctx, &eligibilityResult)
	if err != nil {
		logger.Error("Eligibility check failed", "error", err)
		result.Status = "rejected"
		result.Message = "Failed to check eligibility"
		return &result, err
	}

	if !eligibilityResult["eligible"].(bool) {
		result.Status = "rejected"
		result.Message = eligibilityResult["reason"].(string)
		
		// Publish rejection event
		_ = workflow.ExecuteActivity(ctx, "PublishKafkaEventActivity", "program.enrollment.rejected", input.BeneficiaryID, result).Get(ctx, nil)
		return &result, nil
	}

	// Step 3: Check if already enrolled in this program
	var alreadyEnrolled bool
	err = workflow.ExecuteActivity(ctx, "CheckExistingEnrollmentActivity", input.BeneficiaryID, input.ProgramID).Get(ctx, &alreadyEnrolled)
	if err != nil {
		logger.Warn("Failed to check existing enrollment", "error", err)
	} else if alreadyEnrolled {
		result.Status = "rejected"
		result.Message = "Already enrolled in this program"
		return &result, nil
	}

	// Step 4: Check program capacity
	var capacityAvailable bool
	err = workflow.ExecuteActivity(ctx, "CheckProgramCapacityActivity", input.ProgramID).Get(ctx, &capacityAvailable)
	if err != nil {
		logger.Warn("Failed to check program capacity", "error", err)
		capacityAvailable = true // Assume capacity available on error
	}

	if !capacityAvailable {
		// Add to waitlist
		var waitlistPosition int
		err = workflow.ExecuteActivity(ctx, "AddToWaitlistActivity", input.BeneficiaryID, input.ProgramID).Get(ctx, &waitlistPosition)
		if err != nil {
			logger.Error("Failed to add to waitlist", "error", err)
			result.Status = "rejected"
			result.Message = "Program full and waitlist unavailable"
			return &result, err
		}
		
		result.Status = "waitlisted"
		result.Message = "Added to waitlist at position " + string(rune(waitlistPosition))
		
		// Publish waitlist event
		_ = workflow.ExecuteActivity(ctx, "PublishKafkaEventActivity", "program.waitlisted", input.BeneficiaryID, result).Get(ctx, nil)
		_ = workflow.ExecuteActivity(ctx, "SendNotificationActivity", input.BeneficiaryID, result.Message).Get(ctx, nil)
		
		return &result, nil
	}

	// Step 5: Create enrollment record
	var enrollmentID string
	err = workflow.ExecuteActivity(ctx, "CreateEnrollmentRecordActivity", input.BeneficiaryID, input.ProgramID, input.EnrolledBy).Get(ctx, &enrollmentID)
	if err != nil {
		logger.Error("Failed to create enrollment record", "error", err)
		result.Status = "rejected"
		result.Message = "Failed to create enrollment"
		return &result, err
	}
	result.EnrollmentID = enrollmentID
	result.Status = "enrolled"

	// Step 6: Set start date
	var startDate string
	err = workflow.ExecuteActivity(ctx, "DetermineProgramStartDateActivity", input.ProgramID).Get(ctx, &startDate)
	if err != nil {
		logger.Warn("Failed to determine start date", "error", err)
		startDate = time.Now().Format("2006-01-02")
	}
	result.StartDate = startDate

	// Step 7: Initialize beneficiary benefits in TigerBeetle
	err = workflow.ExecuteActivity(ctx, "InitializeBenefitAccountActivity", input.BeneficiaryID, input.ProgramID, enrollmentID).Get(ctx, nil)
	if err != nil {
		logger.Warn("Failed to initialize benefit account", "error", err)
	}

	// Step 8: Publish enrollment event to Kafka
	err = workflow.ExecuteActivity(ctx, "PublishKafkaEventActivity", "program.enrolled", input.BeneficiaryID, map[string]interface{}{
		"beneficiaryID": input.BeneficiaryID,
		"programID":     input.ProgramID,
		"enrollmentID":  enrollmentID,
		"startDate":     startDate,
		"enrolledBy":    input.EnrolledBy,
	}).Get(ctx, nil)
	if err != nil {
		logger.Warn("Failed to publish Kafka event", "error", err)
	}

	// Step 9: Send notification
	result.Message = "Successfully enrolled in program. Start date: " + startDate
	err = workflow.ExecuteActivity(ctx, "SendNotificationActivity", input.BeneficiaryID, result.Message).Get(ctx, nil)
	if err != nil {
		logger.Warn("Failed to send notification", "error", err)
	}

	// Step 10: Create audit log
	err = workflow.ExecuteActivity(ctx, "CreateAuditLogActivity", "program.enrolled", input.BeneficiaryID, input.EnrolledBy, result).Get(ctx, nil)
	if err != nil {
		logger.Warn("Failed to create audit log", "error", err)
	}

	logger.Info("Program enrollment completed", "beneficiaryID", input.BeneficiaryID, "enrollmentID", enrollmentID)
	return &result, nil
}

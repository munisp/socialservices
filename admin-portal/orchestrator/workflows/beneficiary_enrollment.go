package workflows

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// EnrollBeneficiaryInput represents the input for beneficiary enrollment
type EnrollBeneficiaryInput struct {
	FirstName      string                 `json:"firstName"`
	LastName       string                 `json:"lastName"`
	DateOfBirth    string                 `json:"dateOfBirth"`
	NationalID     string                 `json:"nationalID"`
	PhoneNumber    string                 `json:"phoneNumber"`
	Address        string                 `json:"address"`
	BiometricData  map[string]interface{} `json:"biometricData"`
	Documents      []DocumentUpload       `json:"documents"`
	ProgramID      string                 `json:"programID"`
	EnrolledBy     string                 `json:"enrolledBy"`
	OrganizationID string                 `json:"organizationID"`
}

type DocumentUpload struct {
	Type     string `json:"type"`
	FileURL  string `json:"fileURL"`
	FileName string `json:"fileName"`
}

// EnrollBeneficiaryResult represents the workflow result
type EnrollBeneficiaryResult struct {
	BeneficiaryID     string `json:"beneficiaryID"`
	Status            string `json:"status"`
	KYCStatus         string `json:"kycStatus"`
	TigerBeetleAcctID string `json:"tigerBeetleAcctID"`
	Message           string `json:"message"`
}

// EnrollBeneficiaryWorkflow orchestrates the complete beneficiary enrollment process
func EnrollBeneficiaryWorkflow(ctx workflow.Context, input EnrollBeneficiaryInput) (*EnrollBeneficiaryResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting beneficiary enrollment workflow", "nationalID", input.NationalID)

	// Configure activity options
	activityOptions := workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    time.Minute,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, activityOptions)

	var result EnrollBeneficiaryResult

	// Step 1: Validate National ID
	var nationalIDValid bool
	err := workflow.ExecuteActivity(ctx, "ValidateNationalIDActivity", input.NationalID).Get(ctx, &nationalIDValid)
	if err != nil {
		logger.Error("National ID validation failed", "error", err)
		return nil, err
	}
	if !nationalIDValid {
		result.Status = "rejected"
		result.Message = "Invalid national ID"
		return &result, nil
	}

	// Step 2: Check for duplicates (using Redis cache + DB)
	var isDuplicate bool
	err = workflow.ExecuteActivity(ctx, "CheckDuplicateBeneficiaryActivity", input.NationalID, input.PhoneNumber).Get(ctx, &isDuplicate)
	if err != nil {
		logger.Error("Duplicate check failed", "error", err)
		return nil, err
	}
	if isDuplicate {
		result.Status = "rejected"
		result.Message = "Beneficiary already exists"
		return &result, nil
	}

	// Step 3: Validate documents using Python ML service
	var documentsValid bool
	err = workflow.ExecuteActivity(ctx, "ValidateDocumentsActivity", input.Documents).Get(ctx, &documentsValid)
	if err != nil {
		logger.Error("Document validation failed", "error", err)
		return nil, err
	}
	if !documentsValid {
		result.Status = "pending_review"
		result.Message = "Documents require manual review"
		result.KYCStatus = "pending"
		return &result, nil
	}

	// Step 4: Create beneficiary record in database
	var beneficiaryID string
	err = workflow.ExecuteActivity(ctx, "CreateBeneficiaryRecordActivity", input).Get(ctx, &beneficiaryID)
	if err != nil {
		logger.Error("Failed to create beneficiary record", "error", err)
		return nil, err
	}
	result.BeneficiaryID = beneficiaryID

	// Step 5: Create TigerBeetle financial account
	var tigerBeetleAcctID string
	err = workflow.ExecuteActivity(ctx, "CreateTigerBeetleAccountActivity", beneficiaryID, input.ProgramID).Get(ctx, &tigerBeetleAcctID)
	if err != nil {
		logger.Error("Failed to create TigerBeetle account", "error", err)
		// Continue workflow - account can be created later
		tigerBeetleAcctID = "pending"
	}
	result.TigerBeetleAcctID = tigerBeetleAcctID

	// Step 6: Publish Kafka event
	err = workflow.ExecuteActivity(ctx, "PublishKafkaEventActivity", "beneficiary.enrolled", beneficiaryID, map[string]interface{}{
		"beneficiaryID":     beneficiaryID,
		"nationalID":        input.NationalID,
		"programID":         input.ProgramID,
		"enrolledBy":        input.EnrolledBy,
		"organizationID":    input.OrganizationID,
		"tigerBeetleAcctID": tigerBeetleAcctID,
	}).Get(ctx, nil)
	if err != nil {
		logger.Warn("Failed to publish Kafka event", "error", err)
		// Non-critical - continue
	}

	// Step 7: Cache beneficiary data in Redis (via Dapr)
	err = workflow.ExecuteActivity(ctx, "CacheBeneficiaryDataActivity", beneficiaryID, input).Get(ctx, nil)
	if err != nil {
		logger.Warn("Failed to cache beneficiary data", "error", err)
		// Non-critical - continue
	}

	// Step 8: Send SMS notification
	err = workflow.ExecuteActivity(ctx, "SendSMSNotificationActivity", input.PhoneNumber, "Welcome! Your enrollment is being processed.").Get(ctx, nil)
	if err != nil {
		logger.Warn("Failed to send SMS", "error", err)
		// Non-critical - continue
	}

	// Step 9: Create audit log entry
	err = workflow.ExecuteActivity(ctx, "CreateAuditLogActivity", "beneficiary.enrolled", beneficiaryID, input.EnrolledBy, map[string]interface{}{
		"action":        "enroll_beneficiary",
		"beneficiaryID": beneficiaryID,
		"programID":     input.ProgramID,
	}).Get(ctx, nil)
	if err != nil {
		logger.Warn("Failed to create audit log", "error", err)
	}

	result.Status = "enrolled"
	result.KYCStatus = "approved"
	result.Message = "Beneficiary enrolled successfully"

	logger.Info("Beneficiary enrollment completed", "beneficiaryID", beneficiaryID)
	return &result, nil
}

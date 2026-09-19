package workflows

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// ========== Journey 6: Beneficiary Suspension ==========

type SuspensionInput struct {
	BeneficiaryID string `json:"beneficiaryID"`
	Reason        string `json:"reason"`
	SuspendedBy   string `json:"suspendedBy"`
	Duration      string `json:"duration"` // temporary, permanent
}

type SuspensionResult struct {
	BeneficiaryID  string `json:"beneficiaryID"`
	Status         string `json:"status"`
	SuspensionID   string `json:"suspensionID"`
	EffectiveDate  string `json:"effectiveDate"`
	Message        string `json:"message"`
}

func BeneficiarySuspensionWorkflow(ctx workflow.Context, input SuspensionInput) (*SuspensionResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting suspension workflow", "beneficiaryID", input.BeneficiaryID)

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

	var result SuspensionResult
	result.BeneficiaryID = input.BeneficiaryID

	// Validate beneficiary
	var exists bool
	_ = workflow.ExecuteActivity(ctx, "ValidateBeneficiaryExistsActivity", input.BeneficiaryID).Get(ctx, &exists)
	
	// Create suspension record
	var suspensionID string
	_ = workflow.ExecuteActivity(ctx, "CreateSuspensionRecordActivity", input.BeneficiaryID, input.Reason, input.Duration).Get(ctx, &suspensionID)
	result.SuspensionID = suspensionID

	// Freeze all active benefits in TigerBeetle
	_ = workflow.ExecuteActivity(ctx, "FreezeBenefitAccountsActivity", input.BeneficiaryID).Get(ctx, nil)

	// Block card transactions
	_ = workflow.ExecuteActivity(ctx, "BlockCardTransactionsActivity", input.BeneficiaryID).Get(ctx, nil)

	// Update beneficiary status
	_ = workflow.ExecuteActivity(ctx, "UpdateBeneficiaryStatusActivity", input.BeneficiaryID, "suspended").Get(ctx, nil)

	result.Status = "suspended"
	result.EffectiveDate = time.Now().Format("2006-01-02")
	result.Message = "Beneficiary suspended successfully"

	// Publish event
	_ = workflow.ExecuteActivity(ctx, "PublishKafkaEventActivity", "beneficiary.suspended", input.BeneficiaryID, result).Get(ctx, nil)

	// Send notification
	_ = workflow.ExecuteActivity(ctx, "SendNotificationActivity", input.BeneficiaryID, "Your account has been suspended. Reason: "+input.Reason).Get(ctx, nil)

	// Audit log
	_ = workflow.ExecuteActivity(ctx, "CreateAuditLogActivity", "beneficiary.suspended", input.BeneficiaryID, input.SuspendedBy, result).Get(ctx, nil)

	return &result, nil
}

// ========== Journey 7: Beneficiary Reactivation ==========

type ReactivationInput struct {
	BeneficiaryID string `json:"beneficiaryID"`
	Reason        string `json:"reason"`
	ReactivatedBy string `json:"reactivatedBy"`
}

type ReactivationResult struct {
	BeneficiaryID string `json:"beneficiaryID"`
	Status        string `json:"status"`
	Message       string `json:"message"`
}

func BeneficiaryReactivationWorkflow(ctx workflow.Context, input ReactivationInput) (*ReactivationResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting reactivation workflow", "beneficiaryID", input.BeneficiaryID)

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

	var result ReactivationResult
	result.BeneficiaryID = input.BeneficiaryID

	// Check if beneficiary is suspended
	var isSuspended bool
	_ = workflow.ExecuteActivity(ctx, "CheckSuspensionStatusActivity", input.BeneficiaryID).Get(ctx, &isSuspended)
	
	if !isSuspended {
		result.Status = "failed"
		result.Message = "Beneficiary is not suspended"
		return &result, nil
	}

	// Close suspension record
	_ = workflow.ExecuteActivity(ctx, "CloseSuspensionRecordActivity", input.BeneficiaryID, input.Reason).Get(ctx, nil)

	// Unfreeze benefit accounts
	_ = workflow.ExecuteActivity(ctx, "UnfreezeBenefitAccountsActivity", input.BeneficiaryID).Get(ctx, nil)

	// Unblock card transactions
	_ = workflow.ExecuteActivity(ctx, "UnblockCardTransactionsActivity", input.BeneficiaryID).Get(ctx, nil)

	// Update status to active
	_ = workflow.ExecuteActivity(ctx, "UpdateBeneficiaryStatusActivity", input.BeneficiaryID, "active").Get(ctx, nil)

	result.Status = "active"
	result.Message = "Beneficiary reactivated successfully"

	// Publish event
	_ = workflow.ExecuteActivity(ctx, "PublishKafkaEventActivity", "beneficiary.reactivated", input.BeneficiaryID, result).Get(ctx, nil)

	// Send notification
	_ = workflow.ExecuteActivity(ctx, "SendNotificationActivity", input.BeneficiaryID, "Your account has been reactivated").Get(ctx, nil)

	// Audit log
	_ = workflow.ExecuteActivity(ctx, "CreateAuditLogActivity", "beneficiary.reactivated", input.BeneficiaryID, input.ReactivatedBy, result).Get(ctx, nil)

	return &result, nil
}

// ========== Journey 8: Household Registration ==========

type HouseholdRegistrationInput struct {
	HeadOfHouseholdID string   `json:"headOfHouseholdID"`
	MemberIDs         []string `json:"memberIDs"`
	RegisteredBy      string   `json:"registeredBy"`
}

type HouseholdRegistrationResult struct {
	HouseholdID       string   `json:"householdID"`
	HeadOfHouseholdID string   `json:"headOfHouseholdID"`
	MemberCount       int      `json:"memberCount"`
	Status            string   `json:"status"`
	Message           string   `json:"message"`
}

func HouseholdRegistrationWorkflow(ctx workflow.Context, input HouseholdRegistrationInput) (*HouseholdRegistrationResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting household registration workflow", "headOfHouseholdID", input.HeadOfHouseholdID)

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

	var result HouseholdRegistrationResult
	result.HeadOfHouseholdID = input.HeadOfHouseholdID
	result.MemberCount = len(input.MemberIDs) + 1

	// Validate head of household
	var headExists bool
	_ = workflow.ExecuteActivity(ctx, "ValidateBeneficiaryExistsActivity", input.HeadOfHouseholdID).Get(ctx, &headExists)

	// Validate all members
	for _, memberID := range input.MemberIDs {
		var memberExists bool
		_ = workflow.ExecuteActivity(ctx, "ValidateBeneficiaryExistsActivity", memberID).Get(ctx, &memberExists)
	}

	// Create household record
	var householdID string
	_ = workflow.ExecuteActivity(ctx, "CreateHouseholdRecordActivity", input.HeadOfHouseholdID, input.MemberIDs).Get(ctx, &householdID)
	result.HouseholdID = householdID

	// Link members to household
	for _, memberID := range input.MemberIDs {
		_ = workflow.ExecuteActivity(ctx, "LinkMemberToHouseholdActivity", memberID, householdID).Get(ctx, nil)
	}

	// Calculate household benefits
	_ = workflow.ExecuteActivity(ctx, "CalculateHouseholdBenefitsActivity", householdID).Get(ctx, nil)

	result.Status = "registered"
	result.Message = "Household registered successfully"

	// Publish event
	_ = workflow.ExecuteActivity(ctx, "PublishKafkaEventActivity", "household.registered", householdID, result).Get(ctx, nil)

	// Audit log
	_ = workflow.ExecuteActivity(ctx, "CreateAuditLogActivity", "household.registered", householdID, input.RegisteredBy, result).Get(ctx, nil)

	return &result, nil
}

// ========== Journey 9: Beneficiary Exit/Graduation ==========

type ExitInput struct {
	BeneficiaryID string `json:"beneficiaryID"`
	ExitReason    string `json:"exitReason"` // graduated, relocated, deceased, ineligible
	ExitDate      string `json:"exitDate"`
	ProcessedBy   string `json:"processedBy"`
}

type ExitResult struct {
	BeneficiaryID   string  `json:"beneficiaryID"`
	Status          string  `json:"status"`
	FinalBalance    float64 `json:"finalBalance"`
	SettlementID    string  `json:"settlementID"`
	Message         string  `json:"message"`
}

func BeneficiaryExitWorkflow(ctx workflow.Context, input ExitInput) (*ExitResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting beneficiary exit workflow", "beneficiaryID", input.BeneficiaryID)

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

	var result ExitResult
	result.BeneficiaryID = input.BeneficiaryID

	// Get final balance from TigerBeetle
	var finalBalance float64
	_ = workflow.ExecuteActivity(ctx, "GetBeneficiaryBalanceActivity", input.BeneficiaryID).Get(ctx, &finalBalance)
	result.FinalBalance = finalBalance

	// Process final settlement if balance > 0
	if finalBalance > 0 {
		var settlementID string
		_ = workflow.ExecuteActivity(ctx, "ProcessFinalSettlementActivity", input.BeneficiaryID, finalBalance).Get(ctx, &settlementID)
		result.SettlementID = settlementID
	}

	// Deactivate all enrollments
	_ = workflow.ExecuteActivity(ctx, "DeactivateAllEnrollmentsActivity", input.BeneficiaryID).Get(ctx, nil)

	// Close TigerBeetle accounts
	_ = workflow.ExecuteActivity(ctx, "CloseBenefitAccountsActivity", input.BeneficiaryID).Get(ctx, nil)

	// Deactivate card
	_ = workflow.ExecuteActivity(ctx, "DeactivateCardActivity", input.BeneficiaryID).Get(ctx, nil)

	// Update beneficiary status
	_ = workflow.ExecuteActivity(ctx, "UpdateBeneficiaryStatusActivity", input.BeneficiaryID, "exited").Get(ctx, nil)

	// Create exit record
	_ = workflow.ExecuteActivity(ctx, "CreateExitRecordActivity", input.BeneficiaryID, input.ExitReason, input.ExitDate).Get(ctx, nil)

	result.Status = "exited"
	result.Message = "Beneficiary exit processed successfully"

	// Publish event
	_ = workflow.ExecuteActivity(ctx, "PublishKafkaEventActivity", "beneficiary.exited", input.BeneficiaryID, result).Get(ctx, nil)

	// Send notification
	_ = workflow.ExecuteActivity(ctx, "SendNotificationActivity", input.BeneficiaryID, "Your enrollment has ended. Final balance: "+string(rune(int(finalBalance)))).Get(ctx, nil)

	// Audit log
	_ = workflow.ExecuteActivity(ctx, "CreateAuditLogActivity", "beneficiary.exited", input.BeneficiaryID, input.ProcessedBy, result).Get(ctx, nil)

	return &result, nil
}

// ========== Journey 10: Death Registration ==========

type DeathRegistrationInput struct {
	BeneficiaryID    string `json:"beneficiaryID"`
	DateOfDeath      string `json:"dateOfDeath"`
	DeathCertificate string `json:"deathCertificate"` // File URL
	NextOfKinID      string `json:"nextOfKinID"`
	RegisteredBy     string `json:"registeredBy"`
}

type DeathRegistrationResult struct {
	BeneficiaryID string  `json:"beneficiaryID"`
	Status        string  `json:"status"`
	FinalBalance  float64 `json:"finalBalance"`
	TransferID    string  `json:"transferID"`
	Message       string  `json:"message"`
}

func DeathRegistrationWorkflow(ctx workflow.Context, input DeathRegistrationInput) (*DeathRegistrationResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting death registration workflow", "beneficiaryID", input.BeneficiaryID)

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

	var result DeathRegistrationResult
	result.BeneficiaryID = input.BeneficiaryID

	// Validate death certificate
	var certificateValid bool
	_ = workflow.ExecuteActivity(ctx, "ValidateDeathCertificateActivity", input.DeathCertificate).Get(ctx, &certificateValid)

	// Get final balance
	var finalBalance float64
	_ = workflow.ExecuteActivity(ctx, "GetBeneficiaryBalanceActivity", input.BeneficiaryID).Get(ctx, &finalBalance)
	result.FinalBalance = finalBalance

	// Transfer balance to next of kin (if provided and balance > 0)
	if input.NextOfKinID != "" && finalBalance > 0 {
		var transferID string
		_ = workflow.ExecuteActivity(ctx, "TransferBalanceToNextOfKinActivity", input.BeneficiaryID, input.NextOfKinID, finalBalance).Get(ctx, &transferID)
		result.TransferID = transferID
	}

	// Close all accounts and enrollments
	_ = workflow.ExecuteActivity(ctx, "DeactivateAllEnrollmentsActivity", input.BeneficiaryID).Get(ctx, nil)
	_ = workflow.ExecuteActivity(ctx, "CloseBenefitAccountsActivity", input.BeneficiaryID).Get(ctx, nil)
	_ = workflow.ExecuteActivity(ctx, "DeactivateCardActivity", input.BeneficiaryID).Get(ctx, nil)

	// Update beneficiary status
	_ = workflow.ExecuteActivity(ctx, "UpdateBeneficiaryStatusActivity", input.BeneficiaryID, "deceased").Get(ctx, nil)

	// Create death record
	_ = workflow.ExecuteActivity(ctx, "CreateDeathRecordActivity", input.BeneficiaryID, input.DateOfDeath, input.DeathCertificate).Get(ctx, nil)

	result.Status = "deceased"
	result.Message = "Death registration processed successfully"

	// Publish event
	_ = workflow.ExecuteActivity(ctx, "PublishKafkaEventActivity", "beneficiary.deceased", input.BeneficiaryID, result).Get(ctx, nil)

	// Audit log
	_ = workflow.ExecuteActivity(ctx, "CreateAuditLogActivity", "beneficiary.deceased", input.BeneficiaryID, input.RegisteredBy, result).Get(ctx, nil)

	return &result, nil
}

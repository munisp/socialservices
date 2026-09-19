package journeys

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/workflow"
)

// Lifecycle Journey Definitions (Journeys 14-18)
// These journeys map to existing implemented components:
// - ProfileUpdateWorkflow, BeneficiarySuspensionWorkflow, BeneficiaryReactivationWorkflow
// - BeneficiaryExitWorkflow, DeathRegistrationWorkflow
// - UI: Beneficiary detail page
// - BFF: beneficiaries router

// Journey 14: Profile Update
// UI Entry: Admin Beneficiary detail page, Mobile profile screen
// BFF: trpc.beneficiaries.update
// Workflow: ProfileUpdateWorkflow
type ProfileUpdateInput struct {
	JourneyContext *JourneyContext        `json:"journeyContext"`
	BeneficiaryID  string                 `json:"beneficiaryId"`
	Updates        map[string]interface{} `json:"updates"`
	Reason         string                 `json:"reason,omitempty"`
}

type ProfileUpdateResult struct {
	JourneyRunID   string                 `json:"journeyRunId"`
	BeneficiaryID  string                 `json:"beneficiaryId"`
	UpdatedFields  []string               `json:"updatedFields"`
	PreviousValues map[string]interface{} `json:"previousValues"`
	UpdatedAt      time.Time              `json:"updatedAt"`
}

// ProfileUpdateJourney orchestrates beneficiary profile updates
func ProfileUpdateJourney(ctx workflow.Context, input ProfileUpdateInput) (*ProfileUpdateResult, error) {
	jc := input.JourneyContext
	
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
		RetryPolicy: &workflow.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    time.Minute,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)
	
	// Step 1: Emit journey started event
	workflow.ExecuteActivity(ctx, EmitJourneyEventActivity, jc, "journey.started", map[string]interface{}{
		"journeyType":   "profile_update",
		"beneficiaryId": input.BeneficiaryID,
	})
	
	// Step 2: Check authorization
	var authorized bool
	err := workflow.ExecuteActivity(ctx, CheckAuthorizationActivity, jc, "beneficiary:update").Get(ctx, &authorized)
	if err != nil || !authorized {
		return nil, fmt.Errorf("authorization failed")
	}
	
	// Step 3: Get current profile for audit trail
	var currentProfile map[string]interface{}
	err = workflow.ExecuteActivity(ctx, GetBeneficiaryProfileActivity, input.BeneficiaryID).Get(ctx, &currentProfile)
	if err != nil {
		return nil, fmt.Errorf("beneficiary not found: %v", err)
	}
	
	// Step 4: Validate updates
	var validationResult map[string]interface{}
	err = workflow.ExecuteActivity(ctx, ValidateProfileUpdatesActivity, input.Updates).Get(ctx, &validationResult)
	if err != nil {
		return nil, fmt.Errorf("validation failed: %v", err)
	}
	
	// Step 5: Store previous values for audit
	previousValues := make(map[string]interface{})
	updatedFields := make([]string, 0)
	for field := range input.Updates {
		if currentVal, ok := currentProfile[field]; ok {
			previousValues[field] = currentVal
		}
		updatedFields = append(updatedFields, field)
	}
	
	// Step 6: Apply updates
	err = workflow.ExecuteActivity(ctx, UpdateBeneficiaryProfileActivity, input.BeneficiaryID, input.Updates).Get(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to update profile: %v", err)
	}
	
	// Step 7: Update cache
	workflow.ExecuteActivity(ctx, CacheBeneficiaryDataActivity, input.BeneficiaryID, input.Updates)
	
	// Step 8: Send notification
	if phone, ok := currentProfile["phone"].(string); ok {
		workflow.ExecuteActivity(ctx, SendSMSNotificationActivity, phone, "Your profile has been updated. If you did not make this change, please contact support.")
	}
	
	// Step 9: Publish Kafka event
	workflow.ExecuteActivity(ctx, PublishKafkaEventActivity, "beneficiary.profile_updated", map[string]interface{}{
		"beneficiaryId": input.BeneficiaryID,
		"updatedFields": updatedFields,
		"correlationId": jc.CorrelationID,
	})
	
	// Step 10: Create audit log
	workflow.ExecuteActivity(ctx, CreateAuditLogActivity, map[string]interface{}{
		"action":        "profile_updated",
		"entityType":    "beneficiary",
		"entityId":      input.BeneficiaryID,
		"performedBy":   jc.ActorID,
		"tenantId":      jc.TenantID,
		"correlationId": jc.CorrelationID,
		"details": map[string]interface{}{
			"updatedFields":  updatedFields,
			"previousValues": previousValues,
			"newValues":      input.Updates,
			"reason":         input.Reason,
		},
	})
	
	// Step 11: Emit journey completed event
	workflow.ExecuteActivity(ctx, EmitJourneyEventActivity, jc, "journey.completed", map[string]interface{}{
		"beneficiaryId": input.BeneficiaryID,
		"updatedFields": updatedFields,
	})
	
	return &ProfileUpdateResult{
		JourneyRunID:   jc.JourneyRunID,
		BeneficiaryID:  input.BeneficiaryID,
		UpdatedFields:  updatedFields,
		PreviousValues: previousValues,
		UpdatedAt:      time.Now(),
	}, nil
}

// Journey 15: Beneficiary Suspension
// UI Entry: Admin Beneficiary detail page
// BFF: trpc.beneficiaries.suspend
// Workflow: BeneficiarySuspensionWorkflow
type BeneficiarySuspensionInput struct {
	JourneyContext *JourneyContext `json:"journeyContext"`
	BeneficiaryID  string          `json:"beneficiaryId"`
	Reason         string          `json:"reason"`
	SuspensionType string          `json:"suspensionType"` // "temporary", "permanent"
	Duration       int             `json:"duration,omitempty"` // days, for temporary
	Notes          string          `json:"notes,omitempty"`
}

type BeneficiarySuspensionResult struct {
	JourneyRunID   string    `json:"journeyRunId"`
	BeneficiaryID  string    `json:"beneficiaryId"`
	SuspensionType string    `json:"suspensionType"`
	SuspendedAt    time.Time `json:"suspendedAt"`
	ReactivationDate *time.Time `json:"reactivationDate,omitempty"`
}

// BeneficiarySuspensionJourney orchestrates beneficiary suspension
func BeneficiarySuspensionJourney(ctx workflow.Context, input BeneficiarySuspensionInput) (*BeneficiarySuspensionResult, error) {
	jc := input.JourneyContext
	
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
		RetryPolicy: &workflow.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    time.Minute,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)
	
	// Step 1: Emit journey started event
	workflow.ExecuteActivity(ctx, EmitJourneyEventActivity, jc, "journey.started", map[string]interface{}{
		"journeyType":   "beneficiary_suspension",
		"beneficiaryId": input.BeneficiaryID,
	})
	
	// Step 2: Check authorization
	var authorized bool
	err := workflow.ExecuteActivity(ctx, CheckAuthorizationActivity, jc, "beneficiary:suspend").Get(ctx, &authorized)
	if err != nil || !authorized {
		return nil, fmt.Errorf("authorization failed")
	}
	
	// Step 3: Verify beneficiary is active
	var status string
	err = workflow.ExecuteActivity(ctx, GetBeneficiaryStatusActivity, input.BeneficiaryID).Get(ctx, &status)
	if err != nil {
		return nil, fmt.Errorf("beneficiary not found: %v", err)
	}
	if status == "suspended" {
		return nil, fmt.Errorf("beneficiary already suspended")
	}
	
	// Step 4: Calculate reactivation date for temporary suspension
	var reactivationDate *time.Time
	if input.SuspensionType == "temporary" && input.Duration > 0 {
		rd := time.Now().AddDate(0, 0, input.Duration)
		reactivationDate = &rd
	}
	
	// Step 5: Suspend beneficiary
	err = workflow.ExecuteActivity(ctx, SuspendBeneficiaryActivity, input.BeneficiaryID, map[string]interface{}{
		"reason":           input.Reason,
		"suspensionType":   input.SuspensionType,
		"reactivationDate": reactivationDate,
		"notes":            input.Notes,
		"suspendedBy":      jc.ActorID,
	}).Get(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to suspend beneficiary: %v", err)
	}
	
	// Step 6: Block TigerBeetle account
	workflow.ExecuteActivity(ctx, BlockTigerBeetleAccountActivity, input.BeneficiaryID, input.Reason)
	
	// Step 7: Cancel pending disbursements
	workflow.ExecuteActivity(ctx, CancelPendingDisbursementsActivity, input.BeneficiaryID, input.Reason)
	
	// Step 8: Update cache
	workflow.ExecuteActivity(ctx, CacheBeneficiaryDataActivity, input.BeneficiaryID, map[string]interface{}{
		"status": "suspended",
	})
	
	// Step 9: Send notification
	workflow.ExecuteActivity(ctx, SendSuspensionNotificationActivity, input.BeneficiaryID, input.Reason, reactivationDate)
	
	// Step 10: Publish Kafka event
	workflow.ExecuteActivity(ctx, PublishKafkaEventActivity, "beneficiary.suspended", map[string]interface{}{
		"beneficiaryId":    input.BeneficiaryID,
		"reason":           input.Reason,
		"suspensionType":   input.SuspensionType,
		"reactivationDate": reactivationDate,
		"correlationId":    jc.CorrelationID,
	})
	
	// Step 11: Create audit log
	workflow.ExecuteActivity(ctx, CreateAuditLogActivity, map[string]interface{}{
		"action":        "beneficiary_suspended",
		"entityType":    "beneficiary",
		"entityId":      input.BeneficiaryID,
		"performedBy":   jc.ActorID,
		"tenantId":      jc.TenantID,
		"correlationId": jc.CorrelationID,
		"details": map[string]interface{}{
			"reason":           input.Reason,
			"suspensionType":   input.SuspensionType,
			"reactivationDate": reactivationDate,
			"notes":            input.Notes,
		},
	})
	
	// Step 12: Write to lakehouse
	workflow.ExecuteActivity(ctx, WriteLakehouseFactActivity, "suspension_facts", map[string]interface{}{
		"beneficiaryId":  input.BeneficiaryID,
		"suspensionDate": time.Now().Format("2006-01-02"),
		"reason":         input.Reason,
		"suspensionType": input.SuspensionType,
		"tenantId":       jc.TenantID,
	})
	
	// Step 13: Emit journey completed event
	workflow.ExecuteActivity(ctx, EmitJourneyEventActivity, jc, "journey.completed", map[string]interface{}{
		"beneficiaryId": input.BeneficiaryID,
	})
	
	return &BeneficiarySuspensionResult{
		JourneyRunID:     jc.JourneyRunID,
		BeneficiaryID:    input.BeneficiaryID,
		SuspensionType:   input.SuspensionType,
		SuspendedAt:      time.Now(),
		ReactivationDate: reactivationDate,
	}, nil
}

// Journey 16: Beneficiary Reactivation
// UI Entry: Admin Beneficiary detail page
// BFF: trpc.beneficiaries.reactivate
// Workflow: BeneficiaryReactivationWorkflow
type BeneficiaryReactivationInput struct {
	JourneyContext *JourneyContext `json:"journeyContext"`
	BeneficiaryID  string          `json:"beneficiaryId"`
	Reason         string          `json:"reason"`
	Notes          string          `json:"notes,omitempty"`
}

type BeneficiaryReactivationResult struct {
	JourneyRunID  string    `json:"journeyRunId"`
	BeneficiaryID string    `json:"beneficiaryId"`
	ReactivatedAt time.Time `json:"reactivatedAt"`
}

// BeneficiaryReactivationJourney orchestrates beneficiary reactivation
func BeneficiaryReactivationJourney(ctx workflow.Context, input BeneficiaryReactivationInput) (*BeneficiaryReactivationResult, error) {
	jc := input.JourneyContext
	
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
		RetryPolicy: &workflow.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    time.Minute,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)
	
	// Step 1: Emit journey started event
	workflow.ExecuteActivity(ctx, EmitJourneyEventActivity, jc, "journey.started", map[string]interface{}{
		"journeyType":   "beneficiary_reactivation",
		"beneficiaryId": input.BeneficiaryID,
	})
	
	// Step 2: Check authorization
	var authorized bool
	err := workflow.ExecuteActivity(ctx, CheckAuthorizationActivity, jc, "beneficiary:reactivate").Get(ctx, &authorized)
	if err != nil || !authorized {
		return nil, fmt.Errorf("authorization failed")
	}
	
	// Step 3: Verify beneficiary is suspended
	var status string
	err = workflow.ExecuteActivity(ctx, GetBeneficiaryStatusActivity, input.BeneficiaryID).Get(ctx, &status)
	if err != nil {
		return nil, fmt.Errorf("beneficiary not found: %v", err)
	}
	if status != "suspended" {
		return nil, fmt.Errorf("beneficiary is not suspended: status is %s", status)
	}
	
	// Step 4: Reactivate beneficiary
	err = workflow.ExecuteActivity(ctx, ReactivateBeneficiaryActivity, input.BeneficiaryID, map[string]interface{}{
		"reason":        input.Reason,
		"notes":         input.Notes,
		"reactivatedBy": jc.ActorID,
	}).Get(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to reactivate beneficiary: %v", err)
	}
	
	// Step 5: Unblock TigerBeetle account
	workflow.ExecuteActivity(ctx, UnblockTigerBeetleAccountActivity, input.BeneficiaryID)
	
	// Step 6: Update cache
	workflow.ExecuteActivity(ctx, CacheBeneficiaryDataActivity, input.BeneficiaryID, map[string]interface{}{
		"status": "active",
	})
	
	// Step 7: Send notification
	workflow.ExecuteActivity(ctx, SendReactivationNotificationActivity, input.BeneficiaryID)
	
	// Step 8: Publish Kafka event
	workflow.ExecuteActivity(ctx, PublishKafkaEventActivity, "beneficiary.reactivated", map[string]interface{}{
		"beneficiaryId": input.BeneficiaryID,
		"reason":        input.Reason,
		"correlationId": jc.CorrelationID,
	})
	
	// Step 9: Create audit log
	workflow.ExecuteActivity(ctx, CreateAuditLogActivity, map[string]interface{}{
		"action":        "beneficiary_reactivated",
		"entityType":    "beneficiary",
		"entityId":      input.BeneficiaryID,
		"performedBy":   jc.ActorID,
		"tenantId":      jc.TenantID,
		"correlationId": jc.CorrelationID,
		"details": map[string]interface{}{
			"reason": input.Reason,
			"notes":  input.Notes,
		},
	})
	
	// Step 10: Write to lakehouse
	workflow.ExecuteActivity(ctx, WriteLakehouseFactActivity, "reactivation_facts", map[string]interface{}{
		"beneficiaryId":    input.BeneficiaryID,
		"reactivationDate": time.Now().Format("2006-01-02"),
		"reason":           input.Reason,
		"tenantId":         jc.TenantID,
	})
	
	// Step 11: Emit journey completed event
	workflow.ExecuteActivity(ctx, EmitJourneyEventActivity, jc, "journey.completed", map[string]interface{}{
		"beneficiaryId": input.BeneficiaryID,
	})
	
	return &BeneficiaryReactivationResult{
		JourneyRunID:  jc.JourneyRunID,
		BeneficiaryID: input.BeneficiaryID,
		ReactivatedAt: time.Now(),
	}, nil
}

// Journey 17: Beneficiary Exit/Graduation
// UI Entry: Admin Beneficiary detail page
// BFF: trpc.beneficiaries.exit
// Workflow: BeneficiaryExitWorkflow
type BeneficiaryExitInput struct {
	JourneyContext *JourneyContext `json:"journeyContext"`
	BeneficiaryID  string          `json:"beneficiaryId"`
	ExitType       string          `json:"exitType"` // "graduation", "voluntary", "ineligible", "relocated"
	Reason         string          `json:"reason"`
	EffectiveDate  string          `json:"effectiveDate"`
	Notes          string          `json:"notes,omitempty"`
}

type BeneficiaryExitResult struct {
	JourneyRunID   string    `json:"journeyRunId"`
	BeneficiaryID  string    `json:"beneficiaryId"`
	ExitType       string    `json:"exitType"`
	EffectiveDate  time.Time `json:"effectiveDate"`
	FinalPayment   float64   `json:"finalPayment,omitempty"`
	ExitedAt       time.Time `json:"exitedAt"`
}

// BeneficiaryExitJourney orchestrates beneficiary exit/graduation
func BeneficiaryExitJourney(ctx workflow.Context, input BeneficiaryExitInput) (*BeneficiaryExitResult, error) {
	jc := input.JourneyContext
	
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Minute,
		RetryPolicy: &workflow.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    time.Minute,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)
	
	// Step 1: Emit journey started event
	workflow.ExecuteActivity(ctx, EmitJourneyEventActivity, jc, "journey.started", map[string]interface{}{
		"journeyType":   "beneficiary_exit",
		"beneficiaryId": input.BeneficiaryID,
		"exitType":      input.ExitType,
	})
	
	// Step 2: Check authorization
	var authorized bool
	err := workflow.ExecuteActivity(ctx, CheckAuthorizationActivity, jc, "beneficiary:exit").Get(ctx, &authorized)
	if err != nil || !authorized {
		return nil, fmt.Errorf("authorization failed")
	}
	
	// Step 3: Verify beneficiary exists and is active
	var status string
	err = workflow.ExecuteActivity(ctx, GetBeneficiaryStatusActivity, input.BeneficiaryID).Get(ctx, &status)
	if err != nil {
		return nil, fmt.Errorf("beneficiary not found: %v", err)
	}
	if status == "exited" || status == "graduated" {
		return nil, fmt.Errorf("beneficiary already exited")
	}
	
	effectiveDate, _ := time.Parse("2006-01-02", input.EffectiveDate)
	
	// Step 4: Calculate and process final payment if applicable
	var finalPayment float64
	if input.ExitType == "graduation" {
		err = workflow.ExecuteActivity(ctx, CalculateFinalPaymentActivity, input.BeneficiaryID).Get(ctx, &finalPayment)
		if err == nil && finalPayment > 0 {
			workflow.ExecuteActivity(ctx, ProcessFinalPaymentActivity, input.BeneficiaryID, finalPayment)
		}
	}
	
	// Step 5: Cancel future disbursements
	workflow.ExecuteActivity(ctx, CancelFutureDisbursementsActivity, input.BeneficiaryID, effectiveDate)
	
	// Step 6: Update beneficiary status
	exitStatus := "exited"
	if input.ExitType == "graduation" {
		exitStatus = "graduated"
	}
	
	err = workflow.ExecuteActivity(ctx, UpdateBeneficiaryExitActivity, input.BeneficiaryID, map[string]interface{}{
		"status":        exitStatus,
		"exitType":      input.ExitType,
		"exitReason":    input.Reason,
		"effectiveDate": effectiveDate,
		"notes":         input.Notes,
		"exitedBy":      jc.ActorID,
	}).Get(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to update exit status: %v", err)
	}
	
	// Step 7: Close TigerBeetle account
	workflow.ExecuteActivity(ctx, CloseTigerBeetleAccountActivity, input.BeneficiaryID, input.Reason)
	
	// Step 8: Remove from program enrollments
	workflow.ExecuteActivity(ctx, EndProgramEnrollmentsActivity, input.BeneficiaryID, effectiveDate, input.ExitType)
	
	// Step 9: Update cache
	workflow.ExecuteActivity(ctx, CacheBeneficiaryDataActivity, input.BeneficiaryID, map[string]interface{}{
		"status": exitStatus,
	})
	
	// Step 10: Send notification
	workflow.ExecuteActivity(ctx, SendExitNotificationActivity, input.BeneficiaryID, input.ExitType, effectiveDate)
	
	// Step 11: Publish Kafka event
	workflow.ExecuteActivity(ctx, PublishKafkaEventActivity, "beneficiary.exited", map[string]interface{}{
		"beneficiaryId": input.BeneficiaryID,
		"exitType":      input.ExitType,
		"effectiveDate": input.EffectiveDate,
		"finalPayment":  finalPayment,
		"correlationId": jc.CorrelationID,
	})
	
	// Step 12: Create audit log
	workflow.ExecuteActivity(ctx, CreateAuditLogActivity, map[string]interface{}{
		"action":        "beneficiary_exited",
		"entityType":    "beneficiary",
		"entityId":      input.BeneficiaryID,
		"performedBy":   jc.ActorID,
		"tenantId":      jc.TenantID,
		"correlationId": jc.CorrelationID,
		"details": map[string]interface{}{
			"exitType":      input.ExitType,
			"reason":        input.Reason,
			"effectiveDate": input.EffectiveDate,
			"finalPayment":  finalPayment,
		},
	})
	
	// Step 13: Write to lakehouse
	workflow.ExecuteActivity(ctx, WriteLakehouseFactActivity, "exit_facts", map[string]interface{}{
		"beneficiaryId": input.BeneficiaryID,
		"exitDate":      time.Now().Format("2006-01-02"),
		"exitType":      input.ExitType,
		"reason":        input.Reason,
		"finalPayment":  finalPayment,
		"tenantId":      jc.TenantID,
	})
	
	// Step 14: Emit journey completed event
	workflow.ExecuteActivity(ctx, EmitJourneyEventActivity, jc, "journey.completed", map[string]interface{}{
		"beneficiaryId": input.BeneficiaryID,
		"exitType":      input.ExitType,
	})
	
	return &BeneficiaryExitResult{
		JourneyRunID:  jc.JourneyRunID,
		BeneficiaryID: input.BeneficiaryID,
		ExitType:      input.ExitType,
		EffectiveDate: effectiveDate,
		FinalPayment:  finalPayment,
		ExitedAt:      time.Now(),
	}, nil
}

// Journey 18: Death Registration
// UI Entry: Admin Beneficiary detail page
// BFF: trpc.beneficiaries.registerDeath
// Workflow: DeathRegistrationWorkflow
type DeathRegistrationInput struct {
	JourneyContext    *JourneyContext `json:"journeyContext"`
	BeneficiaryID     string          `json:"beneficiaryId"`
	DateOfDeath       string          `json:"dateOfDeath"`
	CauseOfDeath      string          `json:"causeOfDeath,omitempty"`
	DeathCertificateID string         `json:"deathCertificateId,omitempty"`
	ReportedBy        string          `json:"reportedBy"`
	Notes             string          `json:"notes,omitempty"`
}

type DeathRegistrationResult struct {
	JourneyRunID   string    `json:"journeyRunId"`
	BeneficiaryID  string    `json:"beneficiaryId"`
	DateOfDeath    time.Time `json:"dateOfDeath"`
	RegisteredAt   time.Time `json:"registeredAt"`
	SurvivorBenefits bool    `json:"survivorBenefits"`
}

// DeathRegistrationJourney orchestrates death registration
func DeathRegistrationJourney(ctx workflow.Context, input DeathRegistrationInput) (*DeathRegistrationResult, error) {
	jc := input.JourneyContext
	
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Minute,
		RetryPolicy: &workflow.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    time.Minute,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)
	
	// Step 1: Emit journey started event
	workflow.ExecuteActivity(ctx, EmitJourneyEventActivity, jc, "journey.started", map[string]interface{}{
		"journeyType":   "death_registration",
		"beneficiaryId": input.BeneficiaryID,
	})
	
	// Step 2: Check authorization
	var authorized bool
	err := workflow.ExecuteActivity(ctx, CheckAuthorizationActivity, jc, "beneficiary:register_death").Get(ctx, &authorized)
	if err != nil || !authorized {
		return nil, fmt.Errorf("authorization failed")
	}
	
	// Step 3: Verify beneficiary exists
	var beneficiary map[string]interface{}
	err = workflow.ExecuteActivity(ctx, GetBeneficiaryProfileActivity, input.BeneficiaryID).Get(ctx, &beneficiary)
	if err != nil {
		return nil, fmt.Errorf("beneficiary not found: %v", err)
	}
	
	if beneficiary["status"] == "deceased" {
		return nil, fmt.Errorf("death already registered")
	}
	
	dateOfDeath, _ := time.Parse("2006-01-02", input.DateOfDeath)
	
	// Step 4: Immediately stop all payments
	workflow.ExecuteActivity(ctx, CancelAllDisbursementsActivity, input.BeneficiaryID, "death_registered")
	
	// Step 5: Block TigerBeetle account
	workflow.ExecuteActivity(ctx, BlockTigerBeetleAccountActivity, input.BeneficiaryID, "deceased")
	
	// Step 6: Update beneficiary status
	err = workflow.ExecuteActivity(ctx, UpdateBeneficiaryDeathActivity, input.BeneficiaryID, map[string]interface{}{
		"status":             "deceased",
		"dateOfDeath":        dateOfDeath,
		"causeOfDeath":       input.CauseOfDeath,
		"deathCertificateId": input.DeathCertificateID,
		"reportedBy":         input.ReportedBy,
		"notes":              input.Notes,
		"registeredBy":       jc.ActorID,
	}).Get(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to register death: %v", err)
	}
	
	// Step 7: Check for survivor benefits (if head of household)
	var survivorBenefits bool
	if beneficiary["isHeadOfHousehold"] == true {
		householdID := beneficiary["householdId"].(string)
		err = workflow.ExecuteActivity(ctx, ProcessSurvivorBenefitsActivity, householdID, input.BeneficiaryID).Get(ctx, &survivorBenefits)
		if err != nil {
			workflow.GetLogger(ctx).Warn("Failed to process survivor benefits", "error", err)
		}
	}
	
	// Step 8: End program enrollments
	workflow.ExecuteActivity(ctx, EndProgramEnrollmentsActivity, input.BeneficiaryID, dateOfDeath, "deceased")
	
	// Step 9: Update cache
	workflow.ExecuteActivity(ctx, CacheBeneficiaryDataActivity, input.BeneficiaryID, map[string]interface{}{
		"status": "deceased",
	})
	
	// Step 10: Publish Kafka event
	workflow.ExecuteActivity(ctx, PublishKafkaEventActivity, "beneficiary.death_registered", map[string]interface{}{
		"beneficiaryId":    input.BeneficiaryID,
		"dateOfDeath":      input.DateOfDeath,
		"survivorBenefits": survivorBenefits,
		"correlationId":    jc.CorrelationID,
	})
	
	// Step 11: Create audit log
	workflow.ExecuteActivity(ctx, CreateAuditLogActivity, map[string]interface{}{
		"action":        "death_registered",
		"entityType":    "beneficiary",
		"entityId":      input.BeneficiaryID,
		"performedBy":   jc.ActorID,
		"tenantId":      jc.TenantID,
		"correlationId": jc.CorrelationID,
		"details": map[string]interface{}{
			"dateOfDeath":        input.DateOfDeath,
			"deathCertificateId": input.DeathCertificateID,
			"reportedBy":         input.ReportedBy,
			"survivorBenefits":   survivorBenefits,
		},
	})
	
	// Step 12: Write to lakehouse
	workflow.ExecuteActivity(ctx, WriteLakehouseFactActivity, "death_facts", map[string]interface{}{
		"beneficiaryId":    input.BeneficiaryID,
		"dateOfDeath":      input.DateOfDeath,
		"registrationDate": time.Now().Format("2006-01-02"),
		"survivorBenefits": survivorBenefits,
		"tenantId":         jc.TenantID,
	})
	
	// Step 13: Emit journey completed event
	workflow.ExecuteActivity(ctx, EmitJourneyEventActivity, jc, "journey.completed", map[string]interface{}{
		"beneficiaryId":    input.BeneficiaryID,
		"survivorBenefits": survivorBenefits,
	})
	
	return &DeathRegistrationResult{
		JourneyRunID:     jc.JourneyRunID,
		BeneficiaryID:    input.BeneficiaryID,
		DateOfDeath:      dateOfDeath,
		RegisteredAt:     time.Now(),
		SurvivorBenefits: survivorBenefits,
	}, nil
}

// RegisterLifecycleJourneys registers all lifecycle journey definitions
func RegisterLifecycleJourneys(registry *JourneyRegistry) {
	registry.Register(&JourneyDefinition{
		Key:          "profile_update",
		Name:         "Profile Update",
		Description:  "Update beneficiary profile with audit trail",
		Category:     "lifecycle",
		WorkflowType: "ProfileUpdateJourney",
		RequiredPermissions: []string{"beneficiary:update"},
		UIEntryPoints:       []string{"Admin:BeneficiaryDetailPage", "Mobile:ProfileScreen"},
		BFFEndpoints:        []string{"trpc.beneficiaries.update"},
		MiddlewareHooks:     []string{"kafka", "redis", "permify"},
	})
	
	registry.Register(&JourneyDefinition{
		Key:          "beneficiary_suspension",
		Name:         "Beneficiary Suspension",
		Description:  "Suspend beneficiary with payment blocking",
		Category:     "lifecycle",
		WorkflowType: "BeneficiarySuspensionJourney",
		RequiredPermissions: []string{"beneficiary:suspend"},
		UIEntryPoints:       []string{"Admin:BeneficiaryDetailPage"},
		BFFEndpoints:        []string{"trpc.beneficiaries.suspend"},
		MiddlewareHooks:     []string{"kafka", "redis", "permify", "tigerbeetle", "lakehouse"},
	})
	
	registry.Register(&JourneyDefinition{
		Key:          "beneficiary_reactivation",
		Name:         "Beneficiary Reactivation",
		Description:  "Reactivate suspended beneficiary",
		Category:     "lifecycle",
		WorkflowType: "BeneficiaryReactivationJourney",
		RequiredPermissions: []string{"beneficiary:reactivate"},
		UIEntryPoints:       []string{"Admin:BeneficiaryDetailPage"},
		BFFEndpoints:        []string{"trpc.beneficiaries.reactivate"},
		MiddlewareHooks:     []string{"kafka", "redis", "permify", "tigerbeetle", "lakehouse"},
	})
	
	registry.Register(&JourneyDefinition{
		Key:          "beneficiary_exit",
		Name:         "Beneficiary Exit/Graduation",
		Description:  "Process beneficiary exit or graduation with final payment",
		Category:     "lifecycle",
		WorkflowType: "BeneficiaryExitJourney",
		RequiredPermissions: []string{"beneficiary:exit"},
		UIEntryPoints:       []string{"Admin:BeneficiaryDetailPage"},
		BFFEndpoints:        []string{"trpc.beneficiaries.exit"},
		MiddlewareHooks:     []string{"kafka", "redis", "permify", "tigerbeetle", "lakehouse"},
	})
	
	registry.Register(&JourneyDefinition{
		Key:          "death_registration",
		Name:         "Death Registration",
		Description:  "Register beneficiary death and process survivor benefits",
		Category:     "lifecycle",
		WorkflowType: "DeathRegistrationJourney",
		RequiredPermissions: []string{"beneficiary:register_death"},
		UIEntryPoints:       []string{"Admin:BeneficiaryDetailPage"},
		BFFEndpoints:        []string{"trpc.beneficiaries.registerDeath"},
		MiddlewareHooks:     []string{"kafka", "redis", "permify", "tigerbeetle", "lakehouse"},
	})
}

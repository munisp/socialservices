package workflows

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// ========== Approval Delegation Workflow ==========

// DelegationInput contains the input for creating an approval delegation
type DelegationInput struct {
	DelegationID    string    `json:"delegationId"`
	DelegatorID     string    `json:"delegatorId"`     // User delegating their approval authority
	DelegateID      string    `json:"delegateId"`      // User receiving the delegation
	ApprovalTypes   []string  `json:"approvalTypes"`   // Types of approvals being delegated
	StartTime       time.Time `json:"startTime"`
	EndTime         time.Time `json:"endTime"`
	Reason          string    `json:"reason"`
	MaxAmount       float64   `json:"maxAmount,omitempty"` // Optional: max amount delegate can approve
	RequireNotify   bool      `json:"requireNotify"`       // Notify delegator of actions taken
	CreatedAt       time.Time `json:"createdAt"`
}

// DelegationResult contains the result of the delegation workflow
type DelegationResult struct {
	DelegationID    string     `json:"delegationId"`
	Status          string     `json:"status"` // active, expired, revoked, completed
	ActionsApproved int        `json:"actionsApproved"`
	ActionsRejected int        `json:"actionsRejected"`
	TotalAmount     float64    `json:"totalAmount"`
	ExpiredAt       *time.Time `json:"expiredAt,omitempty"`
	RevokedAt       *time.Time `json:"revokedAt,omitempty"`
	RevokedBy       string     `json:"revokedBy,omitempty"`
	Message         string     `json:"message"`
}

// ApprovalDelegationWorkflow manages time-bounded approval delegation
func ApprovalDelegationWorkflow(ctx workflow.Context, input DelegationInput) (*DelegationResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting approval delegation workflow",
		"delegationId", input.DelegationID,
		"delegator", input.DelegatorID,
		"delegate", input.DelegateID,
	)

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

	result := &DelegationResult{
		DelegationID: input.DelegationID,
		Status:       "active",
	}

	// Step 1: Validate delegation request
	var validationResult ValidationResult
	err := workflow.ExecuteActivity(ctx, "ValidateDelegationActivity",
		input.DelegatorID, input.DelegateID, input.ApprovalTypes).Get(ctx, &validationResult)
	if err != nil || !validationResult.Valid {
		result.Status = "rejected"
		result.Message = fmt.Sprintf("Delegation validation failed: %s", validationResult.Reason)
		return result, nil
	}

	// Step 2: Check for conflicting delegations
	var hasConflict bool
	err = workflow.ExecuteActivity(ctx, "CheckDelegationConflictsActivity",
		input.DelegatorID, input.DelegateID, input.StartTime, input.EndTime).Get(ctx, &hasConflict)
	if err != nil || hasConflict {
		result.Status = "rejected"
		result.Message = "Conflicting delegation already exists"
		return result, nil
	}

	// Step 3: Create delegation record
	err = workflow.ExecuteActivity(ctx, "CreateDelegationRecordActivity", input).Get(ctx, nil)
	if err != nil {
		return result, err
	}

	// Step 4: Grant temporary permissions to delegate
	err = workflow.ExecuteActivity(ctx, "GrantDelegatedPermissionsActivity",
		input.DelegateID, input.ApprovalTypes, input.MaxAmount).Get(ctx, nil)
	if err != nil {
		// Rollback delegation record
		_ = workflow.ExecuteActivity(ctx, "RevokeDelegationRecordActivity", input.DelegationID).Get(ctx, nil)
		return result, err
	}

	// Step 5: Send notification to delegate
	err = workflow.ExecuteActivity(ctx, "SendDelegationNotificationActivity",
		input.DelegateID, input.DelegatorID, input.ApprovalTypes, input.EndTime).Get(ctx, nil)
	if err != nil {
		logger.Warn("Failed to send delegation notification", "error", err)
	}

	// Step 6: Create audit log
	err = workflow.ExecuteActivity(ctx, "CreateAuditLogActivity",
		"delegation.created", input.DelegationID, input.DelegatorID, input).Get(ctx, nil)
	if err != nil {
		logger.Warn("Failed to create audit log", "error", err)
	}

	// Step 7: Wait for delegation period to end or revocation signal
	delegationDuration := input.EndTime.Sub(input.StartTime)
	
	// Set up signal channel for revocation
	revocationChan := workflow.GetSignalChannel(ctx, "revoke-delegation")
	
	// Set up timer for expiration
	timerFuture := workflow.NewTimer(ctx, delegationDuration)
	
	selector := workflow.NewSelector(ctx)
	
	var revoked bool
	var revokedBy string
	
	selector.AddReceive(revocationChan, func(c workflow.ReceiveChannel, more bool) {
		var revocationSignal RevocationSignal
		c.Receive(ctx, &revocationSignal)
		revoked = true
		revokedBy = revocationSignal.RevokedBy
	})
	
	selector.AddFuture(timerFuture, func(f workflow.Future) {
		// Timer expired - delegation period ended
	})
	
	selector.Select(ctx)

	// Step 8: Revoke permissions
	err = workflow.ExecuteActivity(ctx, "RevokeDelegatedPermissionsActivity",
		input.DelegateID, input.ApprovalTypes).Get(ctx, nil)
	if err != nil {
		logger.Error("Failed to revoke delegated permissions", "error", err)
	}

	// Step 9: Get delegation statistics
	var stats DelegationStats
	err = workflow.ExecuteActivity(ctx, "GetDelegationStatsActivity",
		input.DelegationID).Get(ctx, &stats)
	if err != nil {
		logger.Warn("Failed to get delegation stats", "error", err)
	} else {
		result.ActionsApproved = stats.ActionsApproved
		result.ActionsRejected = stats.ActionsRejected
		result.TotalAmount = stats.TotalAmount
	}

	// Step 10: Update delegation record
	now := workflow.Now(ctx)
	if revoked {
		result.Status = "revoked"
		result.RevokedAt = &now
		result.RevokedBy = revokedBy
		result.Message = fmt.Sprintf("Delegation revoked by %s", revokedBy)
	} else {
		result.Status = "expired"
		result.ExpiredAt = &now
		result.Message = "Delegation period ended"
	}

	err = workflow.ExecuteActivity(ctx, "UpdateDelegationRecordActivity",
		input.DelegationID, result).Get(ctx, nil)
	if err != nil {
		logger.Warn("Failed to update delegation record", "error", err)
	}

	// Step 11: Send completion notification
	err = workflow.ExecuteActivity(ctx, "SendDelegationCompletionNotificationActivity",
		input.DelegatorID, input.DelegateID, result).Get(ctx, nil)
	if err != nil {
		logger.Warn("Failed to send completion notification", "error", err)
	}

	// Step 12: Create final audit log
	err = workflow.ExecuteActivity(ctx, "CreateAuditLogActivity",
		"delegation.completed", input.DelegationID, input.DelegatorID, result).Get(ctx, nil)
	if err != nil {
		logger.Warn("Failed to create final audit log", "error", err)
	}

	logger.Info("Approval delegation workflow completed", "status", result.Status)
	return result, nil
}

// ValidationResult contains the result of delegation validation
type ValidationResult struct {
	Valid  bool   `json:"valid"`
	Reason string `json:"reason,omitempty"`
}

// RevocationSignal is sent to revoke a delegation
type RevocationSignal struct {
	RevokedBy string `json:"revokedBy"`
	Reason    string `json:"reason"`
}

// DelegationStats contains statistics about actions taken during delegation
type DelegationStats struct {
	ActionsApproved int     `json:"actionsApproved"`
	ActionsRejected int     `json:"actionsRejected"`
	TotalAmount     float64 `json:"totalAmount"`
}

// ========== Break-Glass Access Workflow ==========

// BreakGlassInput contains the input for break-glass access request
type BreakGlassInput struct {
	RequestID       string   `json:"requestId"`
	RequestorID     string   `json:"requestorId"`
	AccessLevel     string   `json:"accessLevel"`     // elevated, admin, superadmin
	Resources       []string `json:"resources"`       // Resources being accessed
	Justification   string   `json:"justification"`
	Duration        int      `json:"duration"`        // Duration in minutes
	IncidentID      string   `json:"incidentId,omitempty"` // Optional: linked incident
	CreatedAt       time.Time `json:"createdAt"`
}

// BreakGlassResult contains the result of break-glass access
type BreakGlassResult struct {
	RequestID       string     `json:"requestId"`
	Status          string     `json:"status"` // granted, denied, expired, revoked
	GrantedAt       *time.Time `json:"grantedAt,omitempty"`
	ExpiredAt       *time.Time `json:"expiredAt,omitempty"`
	ActionsPerformed []string  `json:"actionsPerformed"`
	Message         string     `json:"message"`
}

// BreakGlassAccessWorkflow handles emergency elevated access with full audit trail
func BreakGlassAccessWorkflow(ctx workflow.Context, input BreakGlassInput) (*BreakGlassResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting break-glass access workflow",
		"requestId", input.RequestID,
		"requestor", input.RequestorID,
		"accessLevel", input.AccessLevel,
	)

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

	result := &BreakGlassResult{
		RequestID: input.RequestID,
	}

	// Step 1: Validate break-glass request
	var eligible bool
	err := workflow.ExecuteActivity(ctx, "ValidateBreakGlassRequestActivity",
		input.RequestorID, input.AccessLevel, input.Justification).Get(ctx, &eligible)
	if err != nil || !eligible {
		result.Status = "denied"
		result.Message = "Break-glass request validation failed"
		
		// Log denied attempt
		_ = workflow.ExecuteActivity(ctx, "CreateAuditLogActivity",
			"breakglass.denied", input.RequestID, input.RequestorID, input).Get(ctx, nil)
		
		// Send alert for denied break-glass attempt
		_ = workflow.ExecuteActivity(ctx, "SendBreakGlassAlertActivity",
			input.RequestorID, input.RequestID, "denied", input.Justification).Get(ctx, nil)
		
		return result, nil
	}

	// Step 2: Create break-glass record
	err = workflow.ExecuteActivity(ctx, "CreateBreakGlassRecordActivity", input).Get(ctx, nil)
	if err != nil {
		return result, err
	}

	// Step 3: Grant elevated access
	err = workflow.ExecuteActivity(ctx, "GrantElevatedAccessActivity",
		input.RequestorID, input.AccessLevel, input.Resources).Get(ctx, nil)
	if err != nil {
		result.Status = "denied"
		result.Message = "Failed to grant elevated access"
		return result, err
	}

	now := workflow.Now(ctx)
	result.Status = "granted"
	result.GrantedAt = &now

	// Step 4: Send immediate alert to security team
	err = workflow.ExecuteActivity(ctx, "SendBreakGlassAlertActivity",
		input.RequestorID, input.RequestID, "granted", input.Justification).Get(ctx, nil)
	if err != nil {
		logger.Warn("Failed to send break-glass alert", "error", err)
	}

	// Step 5: Create audit log for access grant
	err = workflow.ExecuteActivity(ctx, "CreateAuditLogActivity",
		"breakglass.granted", input.RequestID, input.RequestorID, input).Get(ctx, nil)
	if err != nil {
		logger.Warn("Failed to create audit log", "error", err)
	}

	// Step 6: Start monitoring session
	err = workflow.ExecuteActivity(ctx, "StartAccessMonitoringActivity",
		input.RequestorID, input.RequestID).Get(ctx, nil)
	if err != nil {
		logger.Warn("Failed to start access monitoring", "error", err)
	}

	// Step 7: Wait for duration or revocation
	accessDuration := time.Duration(input.Duration) * time.Minute
	
	revocationChan := workflow.GetSignalChannel(ctx, "revoke-breakglass")
	timerFuture := workflow.NewTimer(ctx, accessDuration)
	
	selector := workflow.NewSelector(ctx)
	
	var revoked bool
	
	selector.AddReceive(revocationChan, func(c workflow.ReceiveChannel, more bool) {
		var signal struct {
			RevokedBy string `json:"revokedBy"`
			Reason    string `json:"reason"`
		}
		c.Receive(ctx, &signal)
		revoked = true
	})
	
	selector.AddFuture(timerFuture, func(f workflow.Future) {
		// Timer expired
	})
	
	selector.Select(ctx)

	// Step 8: Revoke elevated access
	err = workflow.ExecuteActivity(ctx, "RevokeElevatedAccessActivity",
		input.RequestorID, input.AccessLevel, input.Resources).Get(ctx, nil)
	if err != nil {
		logger.Error("Failed to revoke elevated access", "error", err)
		// Critical: send emergency alert
		_ = workflow.ExecuteActivity(ctx, "SendEmergencyAlertActivity",
			"BREAK_GLASS_REVOCATION_FAILED", input.RequestID, input.RequestorID).Get(ctx, nil)
	}

	// Step 9: Stop monitoring and get actions performed
	var actions []string
	err = workflow.ExecuteActivity(ctx, "StopAccessMonitoringActivity",
		input.RequestorID, input.RequestID).Get(ctx, &actions)
	if err != nil {
		logger.Warn("Failed to stop access monitoring", "error", err)
	}
	result.ActionsPerformed = actions

	// Step 10: Update result
	expiredAt := workflow.Now(ctx)
	result.ExpiredAt = &expiredAt
	if revoked {
		result.Status = "revoked"
		result.Message = "Break-glass access revoked"
	} else {
		result.Status = "expired"
		result.Message = fmt.Sprintf("Break-glass access expired after %d minutes", input.Duration)
	}

	// Step 11: Update break-glass record
	err = workflow.ExecuteActivity(ctx, "UpdateBreakGlassRecordActivity",
		input.RequestID, result).Get(ctx, nil)
	if err != nil {
		logger.Warn("Failed to update break-glass record", "error", err)
	}

	// Step 12: Create final audit log with all actions
	err = workflow.ExecuteActivity(ctx, "CreateAuditLogActivity",
		"breakglass.completed", input.RequestID, input.RequestorID, result).Get(ctx, nil)
	if err != nil {
		logger.Warn("Failed to create final audit log", "error", err)
	}

	// Step 13: Send completion notification
	err = workflow.ExecuteActivity(ctx, "SendBreakGlassCompletionNotificationActivity",
		input.RequestorID, input.RequestID, result).Get(ctx, nil)
	if err != nil {
		logger.Warn("Failed to send completion notification", "error", err)
	}

	logger.Info("Break-glass access workflow completed",
		"status", result.Status,
		"actionsPerformed", len(result.ActionsPerformed),
	)
	return result, nil
}

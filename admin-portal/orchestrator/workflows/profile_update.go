package workflows

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// ProfileUpdateInput represents the input for profile update
type ProfileUpdateInput struct {
	BeneficiaryID string                 `json:"beneficiaryID"`
	Updates       map[string]interface{} `json:"updates"`
	UpdatedBy     string                 `json:"updatedBy"`
	Reason        string                 `json:"reason"`
}

// ProfileUpdateResult represents the workflow result
type ProfileUpdateResult struct {
	BeneficiaryID    string                 `json:"beneficiaryID"`
	Status           string                 `json:"status"` // approved, pending_approval, rejected
	UpdatedFields    []string               `json:"updatedFields"`
	RequiresApproval bool                   `json:"requiresApproval"`
	ApprovalID       string                 `json:"approvalID"`
	Message          string                 `json:"message"`
}

// ProfileUpdateWorkflow orchestrates beneficiary profile updates
func ProfileUpdateWorkflow(ctx workflow.Context, input ProfileUpdateInput) (*ProfileUpdateResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting profile update workflow", "beneficiaryID", input.BeneficiaryID)

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

	var result ProfileUpdateResult
	result.BeneficiaryID = input.BeneficiaryID
	result.UpdatedFields = []string{}

	// Step 1: Validate beneficiary exists
	var beneficiaryExists bool
	err := workflow.ExecuteActivity(ctx, "ValidateBeneficiaryExistsActivity", input.BeneficiaryID).Get(ctx, &beneficiaryExists)
	if err != nil || !beneficiaryExists {
		result.Status = "rejected"
		result.Message = "Beneficiary not found"
		return &result, err
	}

	// Step 2: Validate update permissions
	var hasPermission bool
	err = workflow.ExecuteActivity(ctx, "CheckUpdatePermissionActivity", input.UpdatedBy, input.BeneficiaryID).Get(ctx, &hasPermission)
	if err != nil || !hasPermission {
		result.Status = "rejected"
		result.Message = "Insufficient permissions to update profile"
		return &result, err
	}

	// Step 3: Determine which fields require approval
	var sensitiveFields []string
	var normalFields []string
	
	err = workflow.ExecuteActivity(ctx, "ClassifyUpdateFieldsActivity", input.Updates).Get(ctx, &map[string]interface{}{
		"sensitive": &sensitiveFields,
		"normal":    &normalFields,
	})
	if err != nil {
		logger.Warn("Failed to classify fields", "error", err)
		// Treat all as sensitive if classification fails
		for field := range input.Updates {
			sensitiveFields = append(sensitiveFields, field)
		}
	}

	// Step 4: Get current beneficiary data for audit trail
	var currentData map[string]interface{}
	err = workflow.ExecuteActivity(ctx, "GetBeneficiaryDataActivity", input.BeneficiaryID).Get(ctx, &currentData)
	if err != nil {
		logger.Warn("Failed to get current data", "error", err)
	}

	// Step 5: Apply normal field updates immediately
	if len(normalFields) > 0 {
		var updatedNormalFields []string
		err = workflow.ExecuteActivity(ctx, "ApplyFieldUpdatesActivity", input.BeneficiaryID, normalFields, input.Updates).Get(ctx, &updatedNormalFields)
		if err != nil {
			logger.Error("Failed to apply normal field updates", "error", err)
		} else {
			result.UpdatedFields = append(result.UpdatedFields, updatedNormalFields...)
		}
	}

	// Step 6: Handle sensitive fields - create approval request
	if len(sensitiveFields) > 0 {
		result.RequiresApproval = true
		
		var approvalID string
		err = workflow.ExecuteActivity(ctx, "CreateApprovalRequestActivity", input.BeneficiaryID, sensitiveFields, input.Updates, input.UpdatedBy, input.Reason).Get(ctx, &approvalID)
		if err != nil {
			logger.Error("Failed to create approval request", "error", err)
			result.Status = "rejected"
			result.Message = "Failed to create approval request for sensitive fields"
			return &result, err
		}
		result.ApprovalID = approvalID

		// Wait for approval (with timeout)
		approvalCtx, cancel := workflow.WithCancel(ctx)
		defer cancel()
		
		approvalOptions := workflow.ActivityOptions{
			StartToCloseTimeout: 7 * 24 * time.Hour, // 7 days for approval
			HeartbeatTimeout:    time.Hour,
		}
		approvalCtx = workflow.WithActivityOptions(approvalCtx, approvalOptions)

		var approvalResult map[string]interface{}
		err = workflow.ExecuteActivity(approvalCtx, "WaitForApprovalActivity", approvalID).Get(approvalCtx, &approvalResult)
		
		if err != nil {
			logger.Warn("Approval timeout or error", "error", err)
			result.Status = "pending_approval"
			result.Message = "Awaiting approval for sensitive field updates. Approval ID: " + approvalID
		} else if approvalResult["approved"].(bool) {
			// Apply sensitive field updates
			var updatedSensitiveFields []string
			err = workflow.ExecuteActivity(ctx, "ApplyFieldUpdatesActivity", input.BeneficiaryID, sensitiveFields, input.Updates).Get(ctx, &updatedSensitiveFields)
			if err != nil {
				logger.Error("Failed to apply sensitive field updates", "error", err)
				result.Status = "rejected"
				result.Message = "Approval granted but update failed"
				return &result, err
			}
			result.UpdatedFields = append(result.UpdatedFields, updatedSensitiveFields...)
			result.Status = "approved"
			result.Message = "Profile updated successfully after approval"
		} else {
			result.Status = "rejected"
			result.Message = "Sensitive field updates rejected by approver"
			
			// Publish rejection event
			_ = workflow.ExecuteActivity(ctx, "PublishKafkaEventActivity", "profile.update.rejected", input.BeneficiaryID, result).Get(ctx, nil)
			return &result, nil
		}
	} else {
		result.Status = "approved"
		result.Message = "Profile updated successfully"
	}

	// Step 7: Publish update event to Kafka
	err = workflow.ExecuteActivity(ctx, "PublishKafkaEventActivity", "profile.updated", input.BeneficiaryID, map[string]interface{}{
		"beneficiaryID":  input.BeneficiaryID,
		"updatedFields":  result.UpdatedFields,
		"updatedBy":      input.UpdatedBy,
		"reason":         input.Reason,
		"requiresApproval": result.RequiresApproval,
		"approvalID":     result.ApprovalID,
	}).Get(ctx, nil)
	if err != nil {
		logger.Warn("Failed to publish Kafka event", "error", err)
	}

	// Step 8: Create audit log with before/after data
	err = workflow.ExecuteActivity(ctx, "CreateDetailedAuditLogActivity", "profile.updated", input.BeneficiaryID, input.UpdatedBy, currentData, input.Updates, result).Get(ctx, nil)
	if err != nil {
		logger.Warn("Failed to create audit log", "error", err)
	}

	// Step 9: Invalidate cached beneficiary data
	err = workflow.ExecuteActivity(ctx, "InvalidateBeneficiaryCacheActivity", input.BeneficiaryID).Get(ctx, nil)
	if err != nil {
		logger.Warn("Failed to invalidate cache", "error", err)
	}

	// Step 10: Send notification
	err = workflow.ExecuteActivity(ctx, "SendNotificationActivity", input.BeneficiaryID, result.Message).Get(ctx, nil)
	if err != nil {
		logger.Warn("Failed to send notification", "error", err)
	}

	logger.Info("Profile update completed", "beneficiaryID", input.BeneficiaryID, "status", result.Status)
	return &result, nil
}

package journeys

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// Admin Journey Definitions (Journeys 19-22)
// These journeys map to existing implemented components:
// - ApprovalDelegationWorkflow, BreakGlassAccessWorkflow, BulkOperationWorkflow
// - UI: Approvals page, Admin settings
// - BFF: approvals router, workflow router

// Journey 19: Approval Delegation
// UI Entry: Admin Approvals page
// BFF: trpc.approvals.delegate
// Workflow: ApprovalDelegationWorkflow
type ApprovalDelegationInput struct {
	JourneyContext *JourneyContext `json:"journeyContext"`
	DelegatorID    string          `json:"delegatorId"`
	DelegateID     string          `json:"delegateId"`
	ApprovalTypes  []string        `json:"approvalTypes"` // "disbursement", "enrollment", "grievance", etc.
	StartDate      string          `json:"startDate"`
	EndDate        string          `json:"endDate"`
	Reason         string          `json:"reason"`
}

type ApprovalDelegationResult struct {
	JourneyRunID string    `json:"journeyRunId"`
	DelegationID string    `json:"delegationId"`
	DelegatorID  string    `json:"delegatorId"`
	DelegateID   string    `json:"delegateId"`
	StartDate    time.Time `json:"startDate"`
	EndDate      time.Time `json:"endDate"`
	CreatedAt    time.Time `json:"createdAt"`
}

// ApprovalDelegationJourney orchestrates approval delegation
func ApprovalDelegationJourney(ctx workflow.Context, input ApprovalDelegationInput) (*ApprovalDelegationResult, error) {
	jc := input.JourneyContext

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
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
		"journeyType": "approval_delegation",
		"delegatorId": input.DelegatorID,
		"delegateId":  input.DelegateID,
	})

	// Step 2: Check authorization
	var authorized bool
	err := workflow.ExecuteActivity(ctx, CheckAuthorizationActivity, jc, "approval:delegate").Get(ctx, &authorized)
	if err != nil || !authorized {
		return nil, fmt.Errorf("authorization failed")
	}

	// Step 3: Verify delegator can delegate (must be the actor or admin)
	if jc.ActorID != input.DelegatorID {
		var isAdmin bool
		workflow.ExecuteActivity(ctx, CheckAdminRoleActivity, jc.ActorID).Get(ctx, &isAdmin)
		if !isAdmin {
			return nil, fmt.Errorf("cannot delegate for another user")
		}
	}

	// Step 4: Verify delegate exists and is active
	var delegateActive bool
	err = workflow.ExecuteActivity(ctx, CheckUserActiveActivity, input.DelegateID).Get(ctx, &delegateActive)
	if err != nil || !delegateActive {
		return nil, fmt.Errorf("delegate user not found or inactive")
	}

	// Step 5: Check for conflicting delegations
	var hasConflict bool
	err = workflow.ExecuteActivity(ctx, CheckDelegationConflictActivity, input.DelegatorID, input.StartDate, input.EndDate).Get(ctx, &hasConflict)
	if hasConflict {
		return nil, fmt.Errorf("conflicting delegation exists for this period")
	}

	startDate, _ := time.Parse("2006-01-02", input.StartDate)
	endDate, _ := time.Parse("2006-01-02", input.EndDate)

	// Step 6: Create delegation record
	var delegationID string
	err = workflow.ExecuteActivity(ctx, CreateDelegationRecordActivity, map[string]interface{}{
		"delegatorId":   input.DelegatorID,
		"delegateId":    input.DelegateID,
		"approvalTypes": input.ApprovalTypes,
		"startDate":     startDate,
		"endDate":       endDate,
		"reason":        input.Reason,
		"status":        "active",
		"createdBy":     jc.ActorID,
		"tenantId":      jc.TenantID,
	}).Get(ctx, &delegationID)
	if err != nil {
		return nil, fmt.Errorf("failed to create delegation: %v", err)
	}

	// Step 7: Update Permify with delegation permissions
	workflow.ExecuteActivity(ctx, GrantDelegationPermissionsActivity, input.DelegateID, input.DelegatorID, input.ApprovalTypes)

	// Step 8: Notify delegate
	workflow.ExecuteActivity(ctx, SendDelegationNotificationActivity, input.DelegateID, input.DelegatorID, input.ApprovalTypes, startDate, endDate)

	// Step 9: Publish Kafka event
	workflow.ExecuteActivity(ctx, PublishKafkaEventActivity, "approval.delegation_created", map[string]interface{}{
		"delegationId":  delegationID,
		"delegatorId":   input.DelegatorID,
		"delegateId":    input.DelegateID,
		"approvalTypes": input.ApprovalTypes,
		"correlationId": jc.CorrelationID,
	})

	// Step 10: Create audit log
	workflow.ExecuteActivity(ctx, CreateAuditLogActivity, map[string]interface{}{
		"action":        "delegation_created",
		"entityType":    "delegation",
		"entityId":      delegationID,
		"performedBy":   jc.ActorID,
		"tenantId":      jc.TenantID,
		"correlationId": jc.CorrelationID,
		"details": map[string]interface{}{
			"delegatorId":   input.DelegatorID,
			"delegateId":    input.DelegateID,
			"approvalTypes": input.ApprovalTypes,
			"startDate":     input.StartDate,
			"endDate":       input.EndDate,
			"reason":        input.Reason,
		},
	})

	// Step 11: Emit journey completed event
	workflow.ExecuteActivity(ctx, EmitJourneyEventActivity, jc, "journey.completed", map[string]interface{}{
		"delegationId": delegationID,
	})

	return &ApprovalDelegationResult{
		JourneyRunID: jc.JourneyRunID,
		DelegationID: delegationID,
		DelegatorID:  input.DelegatorID,
		DelegateID:   input.DelegateID,
		StartDate:    startDate,
		EndDate:      endDate,
		CreatedAt:    time.Now(),
	}, nil
}

// Journey 20: Break Glass Access
// UI Entry: Admin emergency access page
// BFF: trpc.approvals.breakGlass
// Workflow: BreakGlassAccessWorkflow
type BreakGlassAccessInput struct {
	JourneyContext *JourneyContext `json:"journeyContext"`
	ResourceType   string          `json:"resourceType"` // "beneficiary", "disbursement", "program", etc.
	ResourceID     string          `json:"resourceId"`
	Justification  string          `json:"justification"`
	Duration       int             `json:"duration"` // minutes
}

type BreakGlassAccessResult struct {
	JourneyRunID string    `json:"journeyRunId"`
	AccessID     string    `json:"accessId"`
	ResourceType string    `json:"resourceType"`
	ResourceID   string    `json:"resourceId"`
	ExpiresAt    time.Time `json:"expiresAt"`
	GrantedAt    time.Time `json:"grantedAt"`
}

// BreakGlassAccessJourney orchestrates emergency break-glass access
func BreakGlassAccessJourney(ctx workflow.Context, input BreakGlassAccessInput) (*BreakGlassAccessResult, error) {
	jc := input.JourneyContext

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    time.Minute,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Step 1: Emit journey started event (high priority alert)
	workflow.ExecuteActivity(ctx, EmitJourneyEventActivity, jc, "journey.started", map[string]interface{}{
		"journeyType":  "break_glass_access",
		"resourceType": input.ResourceType,
		"resourceId":   input.ResourceID,
		"priority":     "critical",
	})

	// Step 2: Check authorization (must have break-glass permission)
	var authorized bool
	err := workflow.ExecuteActivity(ctx, CheckAuthorizationActivity, jc, "system:break_glass").Get(ctx, &authorized)
	if err != nil || !authorized {
		return nil, fmt.Errorf("break-glass access denied")
	}

	// Step 3: Validate justification (must be non-empty and meaningful)
	if len(input.Justification) < 20 {
		return nil, fmt.Errorf("justification must be at least 20 characters")
	}

	// Step 4: Limit duration (max 60 minutes)
	duration := input.Duration
	if duration > 60 {
		duration = 60
	}
	if duration < 5 {
		duration = 5
	}

	expiresAt := time.Now().Add(time.Duration(duration) * time.Minute)

	// Step 5: Create break-glass access record
	var accessID string
	err = workflow.ExecuteActivity(ctx, CreateBreakGlassRecordActivity, map[string]interface{}{
		"actorId":       jc.ActorID,
		"resourceType":  input.ResourceType,
		"resourceId":    input.ResourceID,
		"justification": input.Justification,
		"duration":      duration,
		"expiresAt":     expiresAt,
		"ipAddress":     jc.IPAddress,
		"userAgent":     jc.UserAgent,
		"tenantId":      jc.TenantID,
	}).Get(ctx, &accessID)
	if err != nil {
		return nil, fmt.Errorf("failed to create break-glass record: %v", err)
	}

	// Step 6: Grant temporary Permify permissions
	workflow.ExecuteActivity(ctx, GrantTemporaryPermissionsActivity, jc.ActorID, input.ResourceType, input.ResourceID, expiresAt)

	// Step 7: Send immediate alerts to security team
	workflow.ExecuteActivity(ctx, SendBreakGlassAlertActivity, map[string]interface{}{
		"accessId":      accessID,
		"actorId":       jc.ActorID,
		"resourceType":  input.ResourceType,
		"resourceId":    input.ResourceID,
		"justification": input.Justification,
		"expiresAt":     expiresAt,
	})

	// Step 8: Publish Kafka event (high priority)
	workflow.ExecuteActivity(ctx, PublishKafkaEventActivity, "security.break_glass_access", map[string]interface{}{
		"accessId":      accessID,
		"actorId":       jc.ActorID,
		"resourceType":  input.ResourceType,
		"resourceId":    input.ResourceID,
		"justification": input.Justification,
		"expiresAt":     expiresAt,
		"correlationId": jc.CorrelationID,
		"priority":      "critical",
	})

	// Step 9: Create audit log (detailed)
	workflow.ExecuteActivity(ctx, CreateAuditLogActivity, map[string]interface{}{
		"action":        "break_glass_access",
		"entityType":    input.ResourceType,
		"entityId":      input.ResourceID,
		"performedBy":   jc.ActorID,
		"tenantId":      jc.TenantID,
		"correlationId": jc.CorrelationID,
		"severity":      "critical",
		"details": map[string]interface{}{
			"accessId":      accessID,
			"justification": input.Justification,
			"duration":      duration,
			"expiresAt":     expiresAt,
			"ipAddress":     jc.IPAddress,
			"userAgent":     jc.UserAgent,
		},
	})

	// Step 10: Write to lakehouse (security events)
	workflow.ExecuteActivity(ctx, WriteLakehouseFactActivity, "security_events", map[string]interface{}{
		"eventType":    "break_glass_access",
		"accessId":     accessID,
		"actorId":      jc.ActorID,
		"resourceType": input.ResourceType,
		"resourceId":   input.ResourceID,
		"eventDate":    time.Now().Format("2006-01-02"),
		"tenantId":     jc.TenantID,
	})

	// Step 11: Schedule access revocation
	workflow.ExecuteActivity(ctx, ScheduleAccessRevocationActivity, accessID, expiresAt)

	// Step 12: Emit journey completed event
	workflow.ExecuteActivity(ctx, EmitJourneyEventActivity, jc, "journey.completed", map[string]interface{}{
		"accessId":  accessID,
		"expiresAt": expiresAt,
	})

	return &BreakGlassAccessResult{
		JourneyRunID: jc.JourneyRunID,
		AccessID:     accessID,
		ResourceType: input.ResourceType,
		ResourceID:   input.ResourceID,
		ExpiresAt:    expiresAt,
		GrantedAt:    time.Now(),
	}, nil
}

// Journey 21: Bulk Operations
// UI Entry: Admin Bulk operations page
// BFF: trpc.workflow.bulkOperation
// Workflow: BulkOperationWorkflow
type BulkOperationInput struct {
	JourneyContext *JourneyContext        `json:"journeyContext"`
	OperationType  string                 `json:"operationType"` // "suspend", "reactivate", "enroll", "disenroll", "update"
	EntityType     string                 `json:"entityType"`    // "beneficiary", "household"
	EntityIDs      []string               `json:"entityIds"`
	Parameters     map[string]interface{} `json:"parameters,omitempty"`
	Reason         string                 `json:"reason"`
}

type BulkOperationResult struct {
	JourneyRunID   string    `json:"journeyRunId"`
	OperationID    string    `json:"operationId"`
	OperationType  string    `json:"operationType"`
	TotalCount     int       `json:"totalCount"`
	SuccessCount   int       `json:"successCount"`
	FailedCount    int       `json:"failedCount"`
	FailedEntities []string  `json:"failedEntities,omitempty"`
	CompletedAt    time.Time `json:"completedAt"`
}

// BulkOperationJourney orchestrates bulk operations on entities
func BulkOperationJourney(ctx workflow.Context, input BulkOperationInput) (*BulkOperationResult, error) {
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
		"journeyType":   "bulk_operation",
		"operationType": input.OperationType,
		"entityType":    input.EntityType,
		"entityCount":   len(input.EntityIDs),
	})

	// Step 2: Check authorization
	permission := fmt.Sprintf("%s:bulk_%s", input.EntityType, input.OperationType)
	var authorized bool
	err := workflow.ExecuteActivity(ctx, CheckAuthorizationActivity, jc, permission).Get(ctx, &authorized)
	if err != nil || !authorized {
		return nil, fmt.Errorf("authorization failed for %s", permission)
	}

	// Step 3: Validate entity count (max 10000)
	if len(input.EntityIDs) > 10000 {
		return nil, fmt.Errorf("bulk operation limited to 10000 entities")
	}

	// Step 4: Create operation record
	var operationID string
	err = workflow.ExecuteActivity(ctx, CreateBulkOperationRecordActivity, map[string]interface{}{
		"operationType": input.OperationType,
		"entityType":    input.EntityType,
		"entityCount":   len(input.EntityIDs),
		"parameters":    input.Parameters,
		"reason":        input.Reason,
		"status":        "processing",
		"createdBy":     jc.ActorID,
		"tenantId":      jc.TenantID,
	}).Get(ctx, &operationID)
	if err != nil {
		return nil, fmt.Errorf("failed to create operation record: %v", err)
	}

	// Step 5: Process entities in batches
	batchSize := 100
	successCount := 0
	failedCount := 0
	var failedEntities []string

	for i := 0; i < len(input.EntityIDs); i += batchSize {
		end := i + batchSize
		if end > len(input.EntityIDs) {
			end = len(input.EntityIDs)
		}
		batch := input.EntityIDs[i:end]

		// Process batch
		var batchResult map[string]interface{}
		err = workflow.ExecuteActivity(ctx, ProcessBulkBatchActivity, map[string]interface{}{
			"operationType": input.OperationType,
			"entityType":    input.EntityType,
			"entityIds":     batch,
			"parameters":    input.Parameters,
			"reason":        input.Reason,
			"actorId":       jc.ActorID,
		}).Get(ctx, &batchResult)

		if err != nil {
			// Mark all in batch as failed
			failedCount += len(batch)
			for _, id := range batch {
				failedEntities = append(failedEntities, id)
			}
		} else {
			batchSuccess := int(batchResult["successCount"].(float64))
			batchFailed := int(batchResult["failedCount"].(float64))
			successCount += batchSuccess
			failedCount += batchFailed

			if failed, ok := batchResult["failedEntities"].([]string); ok {
				failedEntities = append(failedEntities, failed...)
			}
		}

		// Update progress
		progress := float64(i+len(batch)) / float64(len(input.EntityIDs)) * 100
		workflow.ExecuteActivity(ctx, UpdateBulkOperationProgressActivity, operationID, progress)
	}

	// Step 6: Update operation status
	status := "completed"
	if failedCount > 0 && successCount == 0 {
		status = "failed"
	} else if failedCount > 0 {
		status = "partial"
	}

	workflow.ExecuteActivity(ctx, UpdateBulkOperationStatusActivity, operationID, map[string]interface{}{
		"status":         status,
		"successCount":   successCount,
		"failedCount":    failedCount,
		"failedEntities": failedEntities,
	})

	// Step 7: Publish Kafka event
	workflow.ExecuteActivity(ctx, PublishKafkaEventActivity, "bulk_operation.completed", map[string]interface{}{
		"operationId":   operationID,
		"operationType": input.OperationType,
		"entityType":    input.EntityType,
		"totalCount":    len(input.EntityIDs),
		"successCount":  successCount,
		"failedCount":   failedCount,
		"correlationId": jc.CorrelationID,
	})

	// Step 8: Create audit log
	workflow.ExecuteActivity(ctx, CreateAuditLogActivity, map[string]interface{}{
		"action":        "bulk_operation",
		"entityType":    "bulk_operation",
		"entityId":      operationID,
		"performedBy":   jc.ActorID,
		"tenantId":      jc.TenantID,
		"correlationId": jc.CorrelationID,
		"details": map[string]interface{}{
			"operationType": input.OperationType,
			"entityType":    input.EntityType,
			"totalCount":    len(input.EntityIDs),
			"successCount":  successCount,
			"failedCount":   failedCount,
			"reason":        input.Reason,
		},
	})

	// Step 9: Write to lakehouse
	workflow.ExecuteActivity(ctx, WriteLakehouseFactActivity, "bulk_operation_facts", map[string]interface{}{
		"operationId":   operationID,
		"operationType": input.OperationType,
		"entityType":    input.EntityType,
		"operationDate": time.Now().Format("2006-01-02"),
		"totalCount":    len(input.EntityIDs),
		"successCount":  successCount,
		"failedCount":   failedCount,
		"tenantId":      jc.TenantID,
	})

	// Step 10: Emit journey completed event
	workflow.ExecuteActivity(ctx, EmitJourneyEventActivity, jc, "journey.completed", map[string]interface{}{
		"operationId":  operationID,
		"successCount": successCount,
		"failedCount":  failedCount,
	})

	return &BulkOperationResult{
		JourneyRunID:   jc.JourneyRunID,
		OperationID:    operationID,
		OperationType:  input.OperationType,
		TotalCount:     len(input.EntityIDs),
		SuccessCount:   successCount,
		FailedCount:    failedCount,
		FailedEntities: failedEntities,
		CompletedAt:    time.Now(),
	}, nil
}

// Journey 22: Offline Sync
// UI Entry: Mobile app (automatic), Admin sync status page
// BFF: trpc.worldClass.offlineSync
// Workflow: OfflineSyncWorkflow
type OfflineSyncInput struct {
	JourneyContext     *JourneyContext `json:"journeyContext"`
	DeviceID           string          `json:"deviceId"`
	LastSyncTime       time.Time       `json:"lastSyncTime"`
	PendingChanges     []PendingChange `json:"pendingChanges"`
	ConflictResolution string          `json:"conflictResolution"` // "server_wins", "client_wins", "merge"
}

type PendingChange struct {
	EntityType string                 `json:"entityType"`
	EntityID   string                 `json:"entityId"`
	Operation  string                 `json:"operation"` // "create", "update", "delete"
	Data       map[string]interface{} `json:"data"`
	Timestamp  time.Time              `json:"timestamp"`
}

type OfflineSyncResult struct {
	JourneyRunID      string    `json:"journeyRunId"`
	SyncID            string    `json:"syncId"`
	DeviceID          string    `json:"deviceId"`
	UploadedCount     int       `json:"uploadedCount"`
	DownloadedCount   int       `json:"downloadedCount"`
	ConflictsResolved int       `json:"conflictsResolved"`
	SyncedAt          time.Time `json:"syncedAt"`
}

// OfflineSyncJourney orchestrates offline data synchronization
func OfflineSyncJourney(ctx workflow.Context, input OfflineSyncInput) (*OfflineSyncResult, error) {
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
		"journeyType":    "offline_sync",
		"deviceId":       input.DeviceID,
		"pendingChanges": len(input.PendingChanges),
	})

	// Step 2: Check authorization
	var authorized bool
	err := workflow.ExecuteActivity(ctx, CheckAuthorizationActivity, jc, "sync:execute").Get(ctx, &authorized)
	if err != nil || !authorized {
		return nil, fmt.Errorf("authorization failed")
	}

	// Step 3: Verify device registration
	var deviceValid bool
	err = workflow.ExecuteActivity(ctx, VerifyDeviceRegistrationActivity, input.DeviceID, jc.ActorID).Get(ctx, &deviceValid)
	if err != nil || !deviceValid {
		return nil, fmt.Errorf("device not registered")
	}

	// Step 4: Create sync record
	var syncID string
	err = workflow.ExecuteActivity(ctx, CreateSyncRecordActivity, map[string]interface{}{
		"deviceId":       input.DeviceID,
		"actorId":        jc.ActorID,
		"lastSyncTime":   input.LastSyncTime,
		"pendingChanges": len(input.PendingChanges),
		"status":         "processing",
		"tenantId":       jc.TenantID,
	}).Get(ctx, &syncID)
	if err != nil {
		return nil, fmt.Errorf("failed to create sync record: %v", err)
	}

	// Step 5: Process pending changes (upload)
	uploadedCount := 0
	conflictsResolved := 0

	for _, change := range input.PendingChanges {
		// Check for conflicts
		var hasConflict bool
		var serverVersion map[string]interface{}
		err = workflow.ExecuteActivity(ctx, CheckSyncConflictActivity, change.EntityType, change.EntityID, change.Timestamp).Get(ctx, &hasConflict)

		if hasConflict {
			// Resolve conflict based on strategy
			switch input.ConflictResolution {
			case "client_wins":
				// Apply client change
				err = workflow.ExecuteActivity(ctx, ApplySyncChangeActivity, change).Get(ctx, nil)
			case "server_wins":
				// Skip client change, server version will be downloaded
				continue
			case "merge":
				// Merge changes
				workflow.ExecuteActivity(ctx, GetServerVersionActivity, change.EntityType, change.EntityID).Get(ctx, &serverVersion)
				err = workflow.ExecuteActivity(ctx, MergeSyncChangesActivity, change, serverVersion).Get(ctx, nil)
			}
			conflictsResolved++
		} else {
			// No conflict, apply change
			err = workflow.ExecuteActivity(ctx, ApplySyncChangeActivity, change).Get(ctx, nil)
		}

		if err == nil {
			uploadedCount++
		}
	}

	// Step 6: Get server changes since last sync (download)
	var serverChanges []map[string]interface{}
	err = workflow.ExecuteActivity(ctx, GetServerChangesSinceActivity, input.LastSyncTime, jc.ActorID, jc.TenantID).Get(ctx, &serverChanges)
	if err != nil {
		return nil, fmt.Errorf("failed to get server changes: %v", err)
	}

	downloadedCount := len(serverChanges)

	// Step 7: Update sync record
	workflow.ExecuteActivity(ctx, UpdateSyncRecordActivity, syncID, map[string]interface{}{
		"status":            "completed",
		"uploadedCount":     uploadedCount,
		"downloadedCount":   downloadedCount,
		"conflictsResolved": conflictsResolved,
		"completedAt":       time.Now(),
	})

	// Step 8: Update device last sync time
	workflow.ExecuteActivity(ctx, UpdateDeviceLastSyncActivity, input.DeviceID, time.Now())

	// Step 9: Publish Kafka event
	workflow.ExecuteActivity(ctx, PublishKafkaEventActivity, "sync.completed", map[string]interface{}{
		"syncId":            syncID,
		"deviceId":          input.DeviceID,
		"uploadedCount":     uploadedCount,
		"downloadedCount":   downloadedCount,
		"conflictsResolved": conflictsResolved,
		"correlationId":     jc.CorrelationID,
	})

	// Step 10: Create audit log
	workflow.ExecuteActivity(ctx, CreateAuditLogActivity, map[string]interface{}{
		"action":        "offline_sync",
		"entityType":    "sync",
		"entityId":      syncID,
		"performedBy":   jc.ActorID,
		"tenantId":      jc.TenantID,
		"correlationId": jc.CorrelationID,
		"details": map[string]interface{}{
			"deviceId":          input.DeviceID,
			"uploadedCount":     uploadedCount,
			"downloadedCount":   downloadedCount,
			"conflictsResolved": conflictsResolved,
		},
	})

	// Step 11: Emit journey completed event
	workflow.ExecuteActivity(ctx, EmitJourneyEventActivity, jc, "journey.completed", map[string]interface{}{
		"syncId":          syncID,
		"uploadedCount":   uploadedCount,
		"downloadedCount": downloadedCount,
	})

	return &OfflineSyncResult{
		JourneyRunID:      jc.JourneyRunID,
		SyncID:            syncID,
		DeviceID:          input.DeviceID,
		UploadedCount:     uploadedCount,
		DownloadedCount:   downloadedCount,
		ConflictsResolved: conflictsResolved,
		SyncedAt:          time.Now(),
	}, nil
}

// RegisterAdminJourneys registers all admin journey definitions
func RegisterAdminJourneys(registry *JourneyRegistry) {
	registry.Register(&JourneyDefinition{
		Key:                 "approval_delegation",
		Name:                "Approval Delegation",
		Description:         "Delegate approval authority to another user",
		Category:            "admin",
		WorkflowType:        "ApprovalDelegationJourney",
		RequiredPermissions: []string{"approval:delegate"},
		UIEntryPoints:       []string{"Admin:ApprovalsPage"},
		BFFEndpoints:        []string{"trpc.approvals.delegate"},
		MiddlewareHooks:     []string{"kafka", "redis", "permify"},
	})

	registry.Register(&JourneyDefinition{
		Key:                 "break_glass_access",
		Name:                "Break Glass Access",
		Description:         "Emergency access to restricted resources",
		Category:            "admin",
		WorkflowType:        "BreakGlassAccessJourney",
		RequiredPermissions: []string{"system:break_glass"},
		UIEntryPoints:       []string{"Admin:EmergencyAccessPage"},
		BFFEndpoints:        []string{"trpc.approvals.breakGlass"},
		MiddlewareHooks:     []string{"kafka", "redis", "permify", "lakehouse"},
	})

	registry.Register(&JourneyDefinition{
		Key:                 "bulk_operation",
		Name:                "Bulk Operations",
		Description:         "Perform bulk operations on entities",
		Category:            "admin",
		WorkflowType:        "BulkOperationJourney",
		RequiredPermissions: []string{"bulk:execute"},
		UIEntryPoints:       []string{"Admin:BulkOperationsPage"},
		BFFEndpoints:        []string{"trpc.workflow.bulkOperation"},
		MiddlewareHooks:     []string{"kafka", "redis", "permify", "lakehouse"},
	})

	registry.Register(&JourneyDefinition{
		Key:                 "offline_sync",
		Name:                "Offline Sync",
		Description:         "Synchronize offline mobile data with server",
		Category:            "admin",
		WorkflowType:        "OfflineSyncJourney",
		RequiredPermissions: []string{"sync:execute"},
		UIEntryPoints:       []string{"Mobile:SyncScreen", "Admin:SyncStatusPage"},
		BFFEndpoints:        []string{"trpc.worldClass.offlineSync.sync"},
		MiddlewareHooks:     []string{"kafka", "redis", "permify", "dapr"},
	})
}

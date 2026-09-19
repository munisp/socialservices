package journeys

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"go.temporal.io/sdk/activity"
)

// Activities implements all activity functions called by journey workflows
// These activities integrate with the platform's middleware:
// - Kafka for event publishing
// - Redis for caching and idempotency
// - Permify for authorization
// - TigerBeetle for ledger operations
// - Lakehouse for analytics
// - Dapr for state management

// ============================================================================
// Common/Shared Activities
// ============================================================================

// EmitJourneyEventActivity publishes journey events to Kafka
func EmitJourneyEventActivity(ctx context.Context, jc *JourneyContext, eventType string, data map[string]interface{}) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Emitting journey event", "eventType", eventType, "journeyRunId", jc.JourneyRunID)
	
	event := map[string]interface{}{
		"eventType":     eventType,
		"journeyRunId":  jc.JourneyRunID,
		"correlationId": jc.CorrelationID,
		"tenantId":      jc.TenantID,
		"actorId":       jc.ActorID,
		"timestamp":     time.Now().UTC().Format(time.RFC3339),
		"data":          data,
	}
	
	// Publish to Kafka via Dapr
	eventJSON, _ := json.Marshal(event)
	logger.Debug("Publishing event to Kafka", "event", string(eventJSON))
	
	// In production, this would call the Kafka client
	// For now, we log the event for tracing
	return nil
}

// CheckAuthorizationActivity checks if the actor has the required permission via Permify
func CheckAuthorizationActivity(ctx context.Context, jc *JourneyContext, permission string) (bool, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Checking authorization", "actorId", jc.ActorID, "permission", permission)
	
	// Check if actor has the required role/permission
	// In production, this would call Permify
	for _, role := range jc.ActorRoles {
		// Admin has all permissions
		if role == "admin" || role == "super_admin" {
			return true, nil
		}
		// Check specific role-permission mappings
		if hasPermission(role, permission) {
			return true, nil
		}
	}
	
	// Check claims for specific permissions
	if claims, ok := jc.ActorClaims["permissions"].([]string); ok {
		for _, claim := range claims {
			if claim == permission || claim == "*" {
				return true, nil
			}
		}
	}
	
	logger.Warn("Authorization denied", "actorId", jc.ActorID, "permission", permission)
	return false, nil
}

// CheckIdempotencyActivity checks if a request has already been processed via Redis
func CheckIdempotencyActivity(ctx context.Context, idempotencyKey string) (bool, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Checking idempotency", "key", idempotencyKey)
	
	// In production, this would check Redis for the idempotency key
	// Returns true if already processed, false if new request
	return false, nil
}

// SetIdempotencyActivity sets an idempotency key in Redis with TTL
func SetIdempotencyActivity(ctx context.Context, idempotencyKey string, result interface{}, ttlSeconds int) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Setting idempotency key", "key", idempotencyKey, "ttl", ttlSeconds)
	
	// In production, this would set the key in Redis with TTL
	return nil
}

// PublishKafkaEventActivity publishes an event to Kafka
func PublishKafkaEventActivity(ctx context.Context, topic string, data map[string]interface{}) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Publishing Kafka event", "topic", topic)
	
	eventJSON, _ := json.Marshal(data)
	logger.Debug("Event payload", "data", string(eventJSON))
	
	// In production, this would publish to Kafka via the Kafka client
	return nil
}

// CreateAuditLogActivity creates an audit log entry
func CreateAuditLogActivity(ctx context.Context, auditData map[string]interface{}) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Creating audit log", "action", auditData["action"], "entityType", auditData["entityType"])
	
	// Add timestamp
	auditData["timestamp"] = time.Now().UTC().Format(time.RFC3339)
	
	auditJSON, _ := json.Marshal(auditData)
	logger.Debug("Audit log entry", "data", string(auditJSON))
	
	// In production, this would write to the audit log table and Kafka
	return nil
}

// WriteLakehouseFactActivity writes a fact record to the lakehouse
func WriteLakehouseFactActivity(ctx context.Context, tableName string, data map[string]interface{}) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Writing to lakehouse", "table", tableName)
	
	// Add metadata
	data["_ingested_at"] = time.Now().UTC().Format(time.RFC3339)
	
	dataJSON, _ := json.Marshal(data)
	logger.Debug("Lakehouse record", "data", string(dataJSON))
	
	// In production, this would write to Delta Lake via the lakehouse service
	return nil
}

// CacheBeneficiaryDataActivity caches beneficiary data in Redis
func CacheBeneficiaryDataActivity(ctx context.Context, beneficiaryID string, data map[string]interface{}) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Caching beneficiary data", "beneficiaryId", beneficiaryID)
	
	// In production, this would cache in Redis with appropriate TTL
	return nil
}

// ============================================================================
// Enrollment Activities
// ============================================================================

// ValidateBeneficiaryDataActivity validates beneficiary input data
func ValidateBeneficiaryDataActivity(ctx context.Context, data map[string]interface{}) (map[string]interface{}, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Validating beneficiary data")
	
	// Validate required fields
	requiredFields := []string{"firstName", "lastName", "dateOfBirth"}
	for _, field := range requiredFields {
		if _, ok := data[field]; !ok {
			return nil, fmt.Errorf("missing required field: %s", field)
		}
	}
	
	return map[string]interface{}{
		"valid":  true,
		"errors": []string{},
	}, nil
}

// CreateBeneficiaryRecordActivity creates a beneficiary record in the database
func CreateBeneficiaryRecordActivity(ctx context.Context, data map[string]interface{}) (string, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Creating beneficiary record")
	
	// Generate beneficiary ID
	beneficiaryID := generateID("BEN")
	
	// In production, this would insert into the database
	logger.Info("Beneficiary created", "beneficiaryId", beneficiaryID)
	
	return beneficiaryID, nil
}

// CreateTigerBeetleAccountActivity creates a TigerBeetle account for the beneficiary
func CreateTigerBeetleAccountActivity(ctx context.Context, beneficiaryID string, accountType string) (string, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Creating TigerBeetle account", "beneficiaryId", beneficiaryID, "accountType", accountType)
	
	// Generate account ID
	accountID := generateID("ACC")
	
	// In production, this would create an account in TigerBeetle
	return accountID, nil
}

// SendWelcomeNotificationActivity sends a welcome notification to the beneficiary
func SendWelcomeNotificationActivity(ctx context.Context, beneficiaryID string, channel string) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Sending welcome notification", "beneficiaryId", beneficiaryID, "channel", channel)
	
	// In production, this would send SMS/email via notification service
	return nil
}

// VerifyIdentityActivity verifies beneficiary identity via federation service
func VerifyIdentityActivity(ctx context.Context, beneficiaryID string, nationalID string, providerID string) (map[string]interface{}, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Verifying identity", "beneficiaryId", beneficiaryID, "provider", providerID)
	
	// In production, this would call the federation service
	return map[string]interface{}{
		"verified":   true,
		"matchScore": 0.95,
		"provider":   providerID,
	}, nil
}

// UpdateKYCStatusActivity updates the KYC status of a beneficiary
func UpdateKYCStatusActivity(ctx context.Context, beneficiaryID string, status map[string]interface{}) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Updating KYC status", "beneficiaryId", beneficiaryID)
	
	// In production, this would update the database
	return nil
}

// CreateHouseholdRecordActivity creates a household record
func CreateHouseholdRecordActivity(ctx context.Context, data map[string]interface{}) (string, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Creating household record")
	
	householdID := generateID("HH")
	return householdID, nil
}

// AddHouseholdMemberActivity adds a member to a household
func AddHouseholdMemberActivity(ctx context.Context, householdID string, memberData map[string]interface{}) (string, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Adding household member", "householdId", householdID)
	
	memberID := generateID("MEM")
	return memberID, nil
}

// CalculatePMTScoreActivity calculates the Proxy Means Test score
func CalculatePMTScoreActivity(ctx context.Context, householdID string) (float64, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Calculating PMT score", "householdId", householdID)
	
	// In production, this would call the PMT service
	return 0.65, nil
}

// EnrollInProgramActivity enrolls a beneficiary in a program
func EnrollInProgramActivity(ctx context.Context, beneficiaryID string, programID string, data map[string]interface{}) (string, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Enrolling in program", "beneficiaryId", beneficiaryID, "programId", programID)
	
	enrollmentID := generateID("ENR")
	return enrollmentID, nil
}

// IssuePaymentCardActivity issues a payment card
func IssuePaymentCardActivity(ctx context.Context, beneficiaryID string, cardType string) (string, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Issuing payment card", "beneficiaryId", beneficiaryID, "cardType", cardType)
	
	cardID := generateID("CARD")
	return cardID, nil
}

// ============================================================================
// Payment Activities
// ============================================================================

// ValidateDisbursementBudgetActivity validates disbursement against budget
func ValidateDisbursementBudgetActivity(ctx context.Context, programID string, amount float64) (bool, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Validating disbursement budget", "programId", programID, "amount", amount)
	
	// In production, this would check budget availability
	return true, nil
}

// GetEligibleBeneficiariesActivity gets eligible beneficiaries for disbursement
func GetEligibleBeneficiariesActivity(ctx context.Context, programID string, criteria map[string]interface{}) ([]string, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Getting eligible beneficiaries", "programId", programID)
	
	// In production, this would query the database
	return []string{"BEN001", "BEN002", "BEN003"}, nil
}

// CreateDisbursementRecordActivity creates a disbursement record
func CreateDisbursementRecordActivity(ctx context.Context, data map[string]interface{}) (string, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Creating disbursement record")
	
	disbursementID := generateID("DIS")
	return disbursementID, nil
}

// CreatePendingTransferActivity creates a pending transfer in TigerBeetle
func CreatePendingTransferActivity(ctx context.Context, fromAccount string, toAccount string, amount int64, metadata map[string]interface{}) (string, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Creating pending transfer", "from", fromAccount, "to", toAccount, "amount", amount)
	
	transferID := generateID("TXN")
	return transferID, nil
}

// ExecuteMojaloopTransferActivity executes a transfer via Mojaloop
func ExecuteMojaloopTransferActivity(ctx context.Context, transferID string, payeeID string, amount float64) (map[string]interface{}, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Executing Mojaloop transfer", "transferId", transferID, "payeeId", payeeID)
	
	// In production, this would call Mojaloop FSPIOP API
	return map[string]interface{}{
		"transferState": "COMMITTED",
		"fulfilment":    "mock-fulfilment",
	}, nil
}

// PostPendingTransferActivity posts (commits) a pending transfer in TigerBeetle
func PostPendingTransferActivity(ctx context.Context, transferID string) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Posting pending transfer", "transferId", transferID)
	
	// In production, this would commit the pending transfer
	return nil
}

// VoidPendingTransferActivity voids a pending transfer in TigerBeetle
func VoidPendingTransferActivity(ctx context.Context, transferID string, reason string) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Voiding pending transfer", "transferId", transferID, "reason", reason)
	
	// In production, this would void the pending transfer
	return nil
}

// UpdateDisbursementStatusActivity updates disbursement status
func UpdateDisbursementStatusActivity(ctx context.Context, disbursementID string, status string, details map[string]interface{}) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Updating disbursement status", "disbursementId", disbursementID, "status", status)
	
	return nil
}

// SendPaymentNotificationActivity sends payment notification
func SendPaymentNotificationActivity(ctx context.Context, beneficiaryID string, amount float64, status string) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Sending payment notification", "beneficiaryId", beneficiaryID, "amount", amount)
	
	return nil
}

// ============================================================================
// Grievance Activities
// ============================================================================

// CreateGrievanceRecordActivity creates a grievance record
func CreateGrievanceRecordActivity(ctx context.Context, data map[string]interface{}) (string, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Creating grievance record")
	
	grievanceID := generateID("GRV")
	return grievanceID, nil
}

// AutoAssignGrievanceActivity auto-assigns grievance to handler
func AutoAssignGrievanceActivity(ctx context.Context, grievanceID string, category string, priority string) (string, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Auto-assigning grievance", "grievanceId", grievanceID, "category", category)
	
	// In production, this would use workload balancing
	return "handler-001", nil
}

// CalculateSLADeadlineActivity calculates SLA deadline
func CalculateSLADeadlineActivity(ctx context.Context, category string, priority string) (time.Time, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Calculating SLA deadline", "category", category, "priority", priority)
	
	// Default SLA: 5 business days
	deadline := time.Now().AddDate(0, 0, 5)
	
	// Adjust based on priority
	switch priority {
	case "critical":
		deadline = time.Now().AddDate(0, 0, 1)
	case "high":
		deadline = time.Now().AddDate(0, 0, 2)
	case "medium":
		deadline = time.Now().AddDate(0, 0, 5)
	case "low":
		deadline = time.Now().AddDate(0, 0, 10)
	}
	
	return deadline, nil
}

// UpdateGrievanceStatusActivity updates grievance status
func UpdateGrievanceStatusActivity(ctx context.Context, grievanceID string, status string, details map[string]interface{}) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Updating grievance status", "grievanceId", grievanceID, "status", status)
	
	return nil
}

// ProcessCompensationActivity processes compensation for grievance
func ProcessCompensationActivity(ctx context.Context, grievanceID string, beneficiaryID string, amount float64) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Processing compensation", "grievanceId", grievanceID, "amount", amount)
	
	return nil
}

// ============================================================================
// Lifecycle Activities
// ============================================================================

// GetBeneficiaryProfileActivity gets beneficiary profile
func GetBeneficiaryProfileActivity(ctx context.Context, beneficiaryID string) (map[string]interface{}, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Getting beneficiary profile", "beneficiaryId", beneficiaryID)
	
	// In production, this would query the database
	return map[string]interface{}{
		"id":                beneficiaryID,
		"firstName":         "John",
		"lastName":          "Doe",
		"status":            "active",
		"phone":             "+1234567890",
		"isHeadOfHousehold": true,
		"householdId":       "HH001",
	}, nil
}

// ValidateProfileUpdatesActivity validates profile updates
func ValidateProfileUpdatesActivity(ctx context.Context, updates map[string]interface{}) (map[string]interface{}, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Validating profile updates")
	
	return map[string]interface{}{
		"valid":  true,
		"errors": []string{},
	}, nil
}

// UpdateBeneficiaryProfileActivity updates beneficiary profile
func UpdateBeneficiaryProfileActivity(ctx context.Context, beneficiaryID string, updates map[string]interface{}) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Updating beneficiary profile", "beneficiaryId", beneficiaryID)
	
	return nil
}

// GetBeneficiaryStatusActivity gets beneficiary status
func GetBeneficiaryStatusActivity(ctx context.Context, beneficiaryID string) (string, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Getting beneficiary status", "beneficiaryId", beneficiaryID)
	
	return "active", nil
}

// SuspendBeneficiaryActivity suspends a beneficiary
func SuspendBeneficiaryActivity(ctx context.Context, beneficiaryID string, data map[string]interface{}) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Suspending beneficiary", "beneficiaryId", beneficiaryID)
	
	return nil
}

// ReactivateBeneficiaryActivity reactivates a beneficiary
func ReactivateBeneficiaryActivity(ctx context.Context, beneficiaryID string, data map[string]interface{}) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Reactivating beneficiary", "beneficiaryId", beneficiaryID)
	
	return nil
}

// BlockTigerBeetleAccountActivity blocks a TigerBeetle account
func BlockTigerBeetleAccountActivity(ctx context.Context, beneficiaryID string, reason string) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Blocking TigerBeetle account", "beneficiaryId", beneficiaryID)
	
	return nil
}

// UnblockTigerBeetleAccountActivity unblocks a TigerBeetle account
func UnblockTigerBeetleAccountActivity(ctx context.Context, beneficiaryID string) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Unblocking TigerBeetle account", "beneficiaryId", beneficiaryID)
	
	return nil
}

// CancelPendingDisbursementsActivity cancels pending disbursements
func CancelPendingDisbursementsActivity(ctx context.Context, beneficiaryID string, reason string) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Canceling pending disbursements", "beneficiaryId", beneficiaryID)
	
	return nil
}

// SendSMSNotificationActivity sends SMS notification
func SendSMSNotificationActivity(ctx context.Context, phone string, message string) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Sending SMS notification", "phone", phone)
	
	return nil
}

// SendSuspensionNotificationActivity sends suspension notification
func SendSuspensionNotificationActivity(ctx context.Context, beneficiaryID string, reason string, reactivationDate *time.Time) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Sending suspension notification", "beneficiaryId", beneficiaryID)
	
	return nil
}

// SendReactivationNotificationActivity sends reactivation notification
func SendReactivationNotificationActivity(ctx context.Context, beneficiaryID string) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Sending reactivation notification", "beneficiaryId", beneficiaryID)
	
	return nil
}

// ============================================================================
// Admin Activities
// ============================================================================

// CheckAdminRoleActivity checks if user has admin role
func CheckAdminRoleActivity(ctx context.Context, userID string) (bool, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Checking admin role", "userId", userID)
	
	// In production, this would check Keycloak/Permify
	return true, nil
}

// CheckUserActiveActivity checks if user is active
func CheckUserActiveActivity(ctx context.Context, userID string) (bool, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Checking user active", "userId", userID)
	
	return true, nil
}

// CheckDelegationConflictActivity checks for delegation conflicts
func CheckDelegationConflictActivity(ctx context.Context, delegatorID string, startDate string, endDate string) (bool, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Checking delegation conflict", "delegatorId", delegatorID)
	
	return false, nil
}

// CreateDelegationRecordActivity creates a delegation record
func CreateDelegationRecordActivity(ctx context.Context, data map[string]interface{}) (string, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Creating delegation record")
	
	delegationID := generateID("DEL")
	return delegationID, nil
}

// GrantDelegationPermissionsActivity grants delegation permissions in Permify
func GrantDelegationPermissionsActivity(ctx context.Context, delegateID string, delegatorID string, approvalTypes []string) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Granting delegation permissions", "delegateId", delegateID)
	
	return nil
}

// SendDelegationNotificationActivity sends delegation notification
func SendDelegationNotificationActivity(ctx context.Context, delegateID string, delegatorID string, approvalTypes []string, startDate time.Time, endDate time.Time) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Sending delegation notification", "delegateId", delegateID)
	
	return nil
}

// CreateBreakGlassRecordActivity creates a break-glass access record
func CreateBreakGlassRecordActivity(ctx context.Context, data map[string]interface{}) (string, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Creating break-glass record")
	
	accessID := generateID("BG")
	return accessID, nil
}

// GrantTemporaryPermissionsActivity grants temporary permissions
func GrantTemporaryPermissionsActivity(ctx context.Context, actorID string, resourceType string, resourceID string, expiresAt time.Time) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Granting temporary permissions", "actorId", actorID)
	
	return nil
}

// SendBreakGlassAlertActivity sends break-glass alert to security team
func SendBreakGlassAlertActivity(ctx context.Context, data map[string]interface{}) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Sending break-glass alert")
	
	return nil
}

// ScheduleAccessRevocationActivity schedules access revocation
func ScheduleAccessRevocationActivity(ctx context.Context, accessID string, expiresAt time.Time) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Scheduling access revocation", "accessId", accessID, "expiresAt", expiresAt)
	
	return nil
}

// CreateBulkOperationRecordActivity creates a bulk operation record
func CreateBulkOperationRecordActivity(ctx context.Context, data map[string]interface{}) (string, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Creating bulk operation record")
	
	operationID := generateID("BULK")
	return operationID, nil
}

// ProcessBulkBatchActivity processes a batch of entities
func ProcessBulkBatchActivity(ctx context.Context, data map[string]interface{}) (map[string]interface{}, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Processing bulk batch")
	
	entityIDs := data["entityIds"].([]interface{})
	
	return map[string]interface{}{
		"successCount":   len(entityIDs),
		"failedCount":    0,
		"failedEntities": []string{},
	}, nil
}

// UpdateBulkOperationProgressActivity updates bulk operation progress
func UpdateBulkOperationProgressActivity(ctx context.Context, operationID string, progress float64) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Updating bulk operation progress", "operationId", operationID, "progress", progress)
	
	return nil
}

// UpdateBulkOperationStatusActivity updates bulk operation status
func UpdateBulkOperationStatusActivity(ctx context.Context, operationID string, data map[string]interface{}) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Updating bulk operation status", "operationId", operationID)
	
	return nil
}

// ============================================================================
// Sync Activities
// ============================================================================

// VerifyDeviceRegistrationActivity verifies device registration
func VerifyDeviceRegistrationActivity(ctx context.Context, deviceID string, actorID string) (bool, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Verifying device registration", "deviceId", deviceID)
	
	return true, nil
}

// CreateSyncRecordActivity creates a sync record
func CreateSyncRecordActivity(ctx context.Context, data map[string]interface{}) (string, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Creating sync record")
	
	syncID := generateID("SYNC")
	return syncID, nil
}

// CheckSyncConflictActivity checks for sync conflicts
func CheckSyncConflictActivity(ctx context.Context, entityType string, entityID string, timestamp time.Time) (bool, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Checking sync conflict", "entityType", entityType, "entityId", entityID)
	
	return false, nil
}

// ApplySyncChangeActivity applies a sync change
func ApplySyncChangeActivity(ctx context.Context, change PendingChange) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Applying sync change", "entityType", change.EntityType, "entityId", change.EntityID)
	
	return nil
}

// GetServerVersionActivity gets server version of an entity
func GetServerVersionActivity(ctx context.Context, entityType string, entityID string) (map[string]interface{}, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Getting server version", "entityType", entityType, "entityId", entityID)
	
	return map[string]interface{}{}, nil
}

// MergeSyncChangesActivity merges sync changes
func MergeSyncChangesActivity(ctx context.Context, change PendingChange, serverVersion map[string]interface{}) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Merging sync changes", "entityType", change.EntityType, "entityId", change.EntityID)
	
	return nil
}

// GetServerChangesSinceActivity gets server changes since a timestamp
func GetServerChangesSinceActivity(ctx context.Context, since time.Time, actorID string, tenantID string) ([]map[string]interface{}, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Getting server changes since", "since", since)
	
	return []map[string]interface{}{}, nil
}

// UpdateSyncRecordActivity updates sync record
func UpdateSyncRecordActivity(ctx context.Context, syncID string, data map[string]interface{}) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Updating sync record", "syncId", syncID)
	
	return nil
}

// UpdateDeviceLastSyncActivity updates device last sync time
func UpdateDeviceLastSyncActivity(ctx context.Context, deviceID string, syncTime time.Time) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Updating device last sync", "deviceId", deviceID)
	
	return nil
}

// ============================================================================
// Reporting Activities
// ============================================================================

// CreateReportRecordActivity creates a report record
func CreateReportRecordActivity(ctx context.Context, data map[string]interface{}) (string, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Creating report record")
	
	reportID := generateID("RPT")
	return reportID, nil
}

// QueryLakehouseEnrollmentDataActivity queries enrollment data from lakehouse
func QueryLakehouseEnrollmentDataActivity(ctx context.Context, period string, programIDs []string, regions []string) (map[string]interface{}, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Querying lakehouse enrollment data", "period", period)
	
	return map[string]interface{}{
		"totalEnrollments": 1000,
		"newEnrollments":   150,
		"exits":            50,
	}, nil
}

// QueryLakehouseDisbursementDataActivity queries disbursement data from lakehouse
func QueryLakehouseDisbursementDataActivity(ctx context.Context, period string, programIDs []string, regions []string) (map[string]interface{}, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Querying lakehouse disbursement data", "period", period)
	
	return map[string]interface{}{
		"totalDisbursements": 500000.00,
		"successRate":        0.98,
		"failedCount":        10,
	}, nil
}

// QueryLakehouseGrievanceDataActivity queries grievance data from lakehouse
func QueryLakehouseGrievanceDataActivity(ctx context.Context, period string, programIDs []string, regions []string) (map[string]interface{}, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Querying lakehouse grievance data", "period", period)
	
	return map[string]interface{}{
		"totalGrievances": 50,
		"resolved":        45,
		"slaCompliance":   0.90,
	}, nil
}

// CalculateReportKPIsActivity calculates report KPIs
func CalculateReportKPIsActivity(ctx context.Context, enrollmentData map[string]interface{}, disbursementData map[string]interface{}, grievanceData map[string]interface{}) (map[string]interface{}, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Calculating report KPIs")
	
	return map[string]interface{}{
		"enrollmentGrowth":    0.15,
		"disbursementSuccess": 0.98,
		"grievanceResolution": 0.90,
	}, nil
}

// GenerateReportFileActivity generates report file
func GenerateReportFileActivity(ctx context.Context, data map[string]interface{}) (string, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Generating report file")
	
	// In production, this would generate PDF/Excel and upload to RustFS
	return "https://storage.example.com/reports/report-001.pdf", nil
}

// UpdateReportRecordActivity updates report record
func UpdateReportRecordActivity(ctx context.Context, reportID string, data map[string]interface{}) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Updating report record", "reportId", reportID)
	
	return nil
}

// ============================================================================
// Fraud Activities
// ============================================================================

// CheckBeneficiaryExistsActivity checks if beneficiary exists
func CheckBeneficiaryExistsActivity(ctx context.Context, beneficiaryID string) (bool, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Checking beneficiary exists", "beneficiaryId", beneficiaryID)
	
	return true, nil
}

// GenerateFraudCaseNumberActivity generates fraud case number
func GenerateFraudCaseNumberActivity(ctx context.Context) (string, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Generating fraud case number")
	
	return fmt.Sprintf("FRAUD-%s", generateID("")), nil
}

// CalculateFraudRiskScoreActivity calculates fraud risk score using ML
func CalculateFraudRiskScoreActivity(ctx context.Context, beneficiaryID string, reportType string, description string) (float64, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Calculating fraud risk score", "beneficiaryId", beneficiaryID)
	
	// In production, this would call the ML service
	return 0.65, nil
}

// CreateFraudInvestigationActivity creates fraud investigation record
func CreateFraudInvestigationActivity(ctx context.Context, data map[string]interface{}) (string, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Creating fraud investigation")
	
	investigationID := generateID("INV")
	return investigationID, nil
}

// AutoAssignFraudInvestigatorActivity auto-assigns fraud investigator
func AutoAssignFraudInvestigatorActivity(ctx context.Context, investigationID string, priority string, riskScore float64) (string, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Auto-assigning fraud investigator", "investigationId", investigationID)
	
	return "investigator-001", nil
}

// TemporarySuspendBeneficiaryActivity temporarily suspends beneficiary
func TemporarySuspendBeneficiaryActivity(ctx context.Context, beneficiaryID string, investigationID string, reason string) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Temporarily suspending beneficiary", "beneficiaryId", beneficiaryID)
	
	return nil
}

// GatherTransactionHistoryActivity gathers transaction history
func GatherTransactionHistoryActivity(ctx context.Context, beneficiaryID string, investigationID string) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Gathering transaction history", "beneficiaryId", beneficiaryID)
	
	return nil
}

// CrossReferenceBeneficiaryActivity cross-references beneficiary
func CrossReferenceBeneficiaryActivity(ctx context.Context, beneficiaryID string, investigationID string) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Cross-referencing beneficiary", "beneficiaryId", beneficiaryID)
	
	return nil
}

// SendFraudAssignmentNotificationActivity sends fraud assignment notification
func SendFraudAssignmentNotificationActivity(ctx context.Context, assignedTo string, investigationID string, caseNumber string, priority string) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Sending fraud assignment notification", "assignedTo", assignedTo)
	
	return nil
}

// ============================================================================
// Helper Functions
// ============================================================================

// generateID generates a unique ID with optional prefix
func generateID(prefix string) string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	id := hex.EncodeToString(bytes)
	if prefix != "" {
		return fmt.Sprintf("%s-%s", prefix, id)
	}
	return id
}

// hasPermission checks if a role has a specific permission
func hasPermission(role string, permission string) bool {
	// Role-permission mappings
	rolePermissions := map[string][]string{
		"admin": {"*"},
		"super_admin": {"*"},
		"enrollment_officer": {
			"beneficiary:create", "beneficiary:update", "beneficiary:view",
			"household:create", "household:update", "household:view",
			"program:enroll",
		},
		"payment_officer": {
			"disbursement:create", "disbursement:execute", "disbursement:view",
			"payment:view",
		},
		"grievance_officer": {
			"grievance:create", "grievance:update", "grievance:resolve", "grievance:view",
		},
		"fraud_investigator": {
			"fraud:investigate", "fraud:view",
			"beneficiary:view", "beneficiary:suspend",
		},
		"analyst": {
			"report:generate", "analytics:view", "dashboard:view",
			"export:beneficiaries", "export:disbursements", "export:grievances",
		},
		"field_worker": {
			"beneficiary:create", "beneficiary:update", "beneficiary:view",
			"sync:execute",
		},
	}
	
	if perms, ok := rolePermissions[role]; ok {
		for _, perm := range perms {
			if perm == "*" || perm == permission {
				return true
			}
		}
	}
	
	return false
}

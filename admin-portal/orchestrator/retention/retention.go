package retention

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// RetentionPolicy defines data retention rules for different data types
type RetentionPolicy struct {
	DataType       string        `json:"dataType"`
	HotRetention   time.Duration `json:"hotRetention"`   // Keep in primary storage
	WarmRetention  time.Duration `json:"warmRetention"`  // Move to warm storage
	ColdRetention  time.Duration `json:"coldRetention"`  // Move to cold/archive storage
	DeleteAfter    time.Duration `json:"deleteAfter"`    // Permanently delete
	ComplianceHold bool          `json:"complianceHold"` // Legal/compliance hold
	PIIHandling    string        `json:"piiHandling"`    // anonymize, pseudonymize, delete
	AuditRequired  bool          `json:"auditRequired"`  // Require audit log for operations
}

// DefaultRetentionPolicies returns the default retention policies for the platform
func DefaultRetentionPolicies() map[string]RetentionPolicy {
	return map[string]RetentionPolicy{
		"audit_logs": {
			DataType:       "audit_logs",
			HotRetention:   90 * 24 * time.Hour,       // 90 days
			WarmRetention:  365 * 24 * time.Hour,      // 1 year
			ColdRetention:  7 * 365 * 24 * time.Hour,  // 7 years
			DeleteAfter:    10 * 365 * 24 * time.Hour, // 10 years
			ComplianceHold: true,
			PIIHandling:    "pseudonymize",
			AuditRequired:  true,
		},
		"transactions": {
			DataType:       "transactions",
			HotRetention:   30 * 24 * time.Hour,       // 30 days
			WarmRetention:  365 * 24 * time.Hour,      // 1 year
			ColdRetention:  7 * 365 * 24 * time.Hour,  // 7 years
			DeleteAfter:    10 * 365 * 24 * time.Hour, // 10 years
			ComplianceHold: true,
			PIIHandling:    "anonymize",
			AuditRequired:  true,
		},
		"beneficiary_data": {
			DataType:       "beneficiary_data",
			HotRetention:   365 * 24 * time.Hour,      // 1 year
			WarmRetention:  3 * 365 * 24 * time.Hour,  // 3 years
			ColdRetention:  7 * 365 * 24 * time.Hour,  // 7 years
			DeleteAfter:    10 * 365 * 24 * time.Hour, // 10 years
			ComplianceHold: false,
			PIIHandling:    "anonymize",
			AuditRequired:  true,
		},
		"session_data": {
			DataType:       "session_data",
			HotRetention:   7 * 24 * time.Hour,   // 7 days
			WarmRetention:  30 * 24 * time.Hour,  // 30 days
			ColdRetention:  90 * 24 * time.Hour,  // 90 days
			DeleteAfter:    180 * 24 * time.Hour, // 180 days
			ComplianceHold: false,
			PIIHandling:    "delete",
			AuditRequired:  false,
		},
		"notifications": {
			DataType:       "notifications",
			HotRetention:   30 * 24 * time.Hour,      // 30 days
			WarmRetention:  90 * 24 * time.Hour,      // 90 days
			ColdRetention:  365 * 24 * time.Hour,     // 1 year
			DeleteAfter:    2 * 365 * 24 * time.Hour, // 2 years
			ComplianceHold: false,
			PIIHandling:    "delete",
			AuditRequired:  false,
		},
		"workflow_history": {
			DataType:       "workflow_history",
			HotRetention:   30 * 24 * time.Hour,      // 30 days
			WarmRetention:  180 * 24 * time.Hour,     // 180 days
			ColdRetention:  365 * 24 * time.Hour,     // 1 year
			DeleteAfter:    3 * 365 * 24 * time.Hour, // 3 years
			ComplianceHold: false,
			PIIHandling:    "anonymize",
			AuditRequired:  true,
		},
		"metrics": {
			DataType:       "metrics",
			HotRetention:   7 * 24 * time.Hour,       // 7 days (full resolution)
			WarmRetention:  30 * 24 * time.Hour,      // 30 days (downsampled)
			ColdRetention:  365 * 24 * time.Hour,     // 1 year (aggregated)
			DeleteAfter:    2 * 365 * 24 * time.Hour, // 2 years
			ComplianceHold: false,
			PIIHandling:    "delete",
			AuditRequired:  false,
		},
		"logs": {
			DataType:       "logs",
			HotRetention:   7 * 24 * time.Hour,   // 7 days
			WarmRetention:  30 * 24 * time.Hour,  // 30 days
			ColdRetention:  90 * 24 * time.Hour,  // 90 days
			DeleteAfter:    365 * 24 * time.Hour, // 1 year
			ComplianceHold: false,
			PIIHandling:    "delete",
			AuditRequired:  false,
		},
	}
}

// RetentionInput is the input for the retention workflow
type RetentionInput struct {
	DataType    string    `json:"dataType"`
	DryRun      bool      `json:"dryRun"`
	InitiatedBy string    `json:"initiatedBy"`
	CreatedAt   time.Time `json:"createdAt"`
}

// RetentionResult is the result of the retention workflow
type RetentionResult struct {
	DataType           string    `json:"dataType"`
	RecordsProcessed   int64     `json:"recordsProcessed"`
	RecordsMovedToWarm int64     `json:"recordsMovedToWarm"`
	RecordsMovedToCold int64     `json:"recordsMovedToCold"`
	RecordsDeleted     int64     `json:"recordsDeleted"`
	RecordsAnonymized  int64     `json:"recordsAnonymized"`
	BytesFreed         int64     `json:"bytesFreed"`
	Errors             []string  `json:"errors,omitempty"`
	CompletedAt        time.Time `json:"completedAt"`
	DryRun             bool      `json:"dryRun"`
}

// StorageTier represents a storage tier
type StorageTier string

const (
	StorageTierHot  StorageTier = "hot"
	StorageTierWarm StorageTier = "warm"
	StorageTierCold StorageTier = "cold"
)

// DataRetentionWorkflow implements the data retention workflow
func DataRetentionWorkflow(ctx workflow.Context, input RetentionInput) (*RetentionResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting data retention workflow", "dataType", input.DataType, "dryRun", input.DryRun)

	// Activity options with retry
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Minute,
		HeartbeatTimeout:    5 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    time.Minute,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	result := &RetentionResult{
		DataType: input.DataType,
		DryRun:   input.DryRun,
	}

	// Step 1: Get retention policy
	var policy RetentionPolicy
	err := workflow.ExecuteActivity(ctx, GetRetentionPolicyActivity, input.DataType).Get(ctx, &policy)
	if err != nil {
		return nil, fmt.Errorf("failed to get retention policy: %w", err)
	}

	// Step 2: Check for compliance holds
	if policy.ComplianceHold {
		var hasHold bool
		err = workflow.ExecuteActivity(ctx, "CheckComplianceHoldActivity", input.DataType).Get(ctx, &hasHold)
		if err != nil {
			logger.Warn("Failed to check compliance hold, proceeding with caution", "error", err)
		}
		if hasHold {
			logger.Info("Data type has compliance hold, skipping deletion", "dataType", input.DataType)
			result.Errors = append(result.Errors, "Compliance hold active - deletion skipped")
		}
	}

	// Step 3: Move data to warm storage
	var warmResult TierMoveResult
	err = workflow.ExecuteActivity(ctx, "MoveToWarmStorageActivity", MoveDataInput{
		DataType:  input.DataType,
		OlderThan: policy.HotRetention,
		DryRun:    input.DryRun,
	}).Get(ctx, &warmResult)
	if err != nil {
		logger.Error("Failed to move data to warm storage", "error", err)
		result.Errors = append(result.Errors, fmt.Sprintf("Warm storage move failed: %v", err))
	} else {
		result.RecordsMovedToWarm = warmResult.RecordsMoved
		result.BytesFreed += warmResult.BytesMoved
	}

	// Step 4: Move data to cold storage
	var coldResult TierMoveResult
	err = workflow.ExecuteActivity(ctx, "MoveToColdStorageActivity", MoveDataInput{
		DataType:  input.DataType,
		OlderThan: policy.WarmRetention,
		DryRun:    input.DryRun,
	}).Get(ctx, &coldResult)
	if err != nil {
		logger.Error("Failed to move data to cold storage", "error", err)
		result.Errors = append(result.Errors, fmt.Sprintf("Cold storage move failed: %v", err))
	} else {
		result.RecordsMovedToCold = coldResult.RecordsMoved
		result.BytesFreed += coldResult.BytesMoved
	}

	// Step 5: Handle PII based on policy
	if policy.PIIHandling == "anonymize" || policy.PIIHandling == "pseudonymize" {
		var anonResult AnonymizeResult
		err = workflow.ExecuteActivity(ctx, "AnonymizeDataActivity", AnonymizeInput{
			DataType:  input.DataType,
			OlderThan: policy.ColdRetention,
			Method:    policy.PIIHandling,
			DryRun:    input.DryRun,
		}).Get(ctx, &anonResult)
		if err != nil {
			logger.Error("Failed to anonymize data", "error", err)
			result.Errors = append(result.Errors, fmt.Sprintf("Anonymization failed: %v", err))
		} else {
			result.RecordsAnonymized = anonResult.RecordsAnonymized
		}
	}

	// Step 6: Delete expired data (if no compliance hold)
	if !policy.ComplianceHold {
		var deleteResult DeleteResult
		err = workflow.ExecuteActivity(ctx, "DeleteExpiredDataActivity", DeleteInput{
			DataType:  input.DataType,
			OlderThan: policy.DeleteAfter,
			DryRun:    input.DryRun,
		}).Get(ctx, &deleteResult)
		if err != nil {
			logger.Error("Failed to delete expired data", "error", err)
			result.Errors = append(result.Errors, fmt.Sprintf("Deletion failed: %v", err))
		} else {
			result.RecordsDeleted = deleteResult.RecordsDeleted
			result.BytesFreed += deleteResult.BytesFreed
		}
	}

	// Step 7: Create audit log
	if policy.AuditRequired && !input.DryRun {
		err = workflow.ExecuteActivity(ctx, "CreateRetentionAuditLogActivity", AuditLogInput{
			DataType:    input.DataType,
			InitiatedBy: input.InitiatedBy,
			Result:      result,
		}).Get(ctx, nil)
		if err != nil {
			logger.Error("Failed to create audit log", "error", err)
			result.Errors = append(result.Errors, fmt.Sprintf("Audit log failed: %v", err))
		}
	}

	result.RecordsProcessed = result.RecordsMovedToWarm + result.RecordsMovedToCold + result.RecordsDeleted + result.RecordsAnonymized
	result.CompletedAt = workflow.Now(ctx)

	logger.Info("Data retention workflow completed",
		"dataType", input.DataType,
		"recordsProcessed", result.RecordsProcessed,
		"bytesFreed", result.BytesFreed)

	return result, nil
}

// ScheduledRetentionWorkflow runs retention for all data types on a schedule
func ScheduledRetentionWorkflow(ctx workflow.Context) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting scheduled retention workflow")

	policies := DefaultRetentionPolicies()

	// Run retention for each data type in parallel
	var futures []workflow.Future
	for dataType := range policies {
		input := RetentionInput{
			DataType:    dataType,
			DryRun:      false,
			InitiatedBy: "scheduled",
			CreatedAt:   workflow.Now(ctx),
		}

		future := workflow.ExecuteChildWorkflow(ctx, DataRetentionWorkflow, input)
		futures = append(futures, future)
	}

	// Wait for all to complete
	var errors []string
	for i, future := range futures {
		var result RetentionResult
		if err := future.Get(ctx, &result); err != nil {
			errors = append(errors, fmt.Sprintf("Retention failed for data type %d: %v", i, err))
		}
	}

	if len(errors) > 0 {
		logger.Error("Some retention workflows failed", "errors", errors)
	}

	logger.Info("Scheduled retention workflow completed")
	return nil
}

// Activity input/output types

type MoveDataInput struct {
	DataType  string        `json:"dataType"`
	OlderThan time.Duration `json:"olderThan"`
	DryRun    bool          `json:"dryRun"`
}

type TierMoveResult struct {
	RecordsMoved int64 `json:"recordsMoved"`
	BytesMoved   int64 `json:"bytesMoved"`
}

type AnonymizeInput struct {
	DataType  string        `json:"dataType"`
	OlderThan time.Duration `json:"olderThan"`
	Method    string        `json:"method"`
	DryRun    bool          `json:"dryRun"`
}

type AnonymizeResult struct {
	RecordsAnonymized int64 `json:"recordsAnonymized"`
}

type DeleteInput struct {
	DataType  string        `json:"dataType"`
	OlderThan time.Duration `json:"olderThan"`
	DryRun    bool          `json:"dryRun"`
}

type DeleteResult struct {
	RecordsDeleted int64 `json:"recordsDeleted"`
	BytesFreed     int64 `json:"bytesFreed"`
}

type AuditLogInput struct {
	DataType    string           `json:"dataType"`
	InitiatedBy string           `json:"initiatedBy"`
	Result      *RetentionResult `json:"result"`
}

// RetentionActivities implements the retention activities
type RetentionActivities struct {
	db          *sql.DB
	warmStorage StorageClient
	coldStorage StorageClient
	auditLogger AuditLogger
}

// StorageClient interface for storage operations
type StorageClient interface {
	Store(ctx context.Context, key string, data []byte) error
	Delete(ctx context.Context, key string) error
	List(ctx context.Context, prefix string) ([]string, error)
}

// AuditLogger interface for audit logging
type AuditLogger interface {
	Log(ctx context.Context, action string, details map[string]interface{}) error
}

// NewRetentionActivities creates a new RetentionActivities instance
func NewRetentionActivities(db *sql.DB, warmStorage, coldStorage StorageClient, auditLogger AuditLogger) *RetentionActivities {
	return &RetentionActivities{
		db:          db,
		warmStorage: warmStorage,
		coldStorage: coldStorage,
		auditLogger: auditLogger,
	}
}

// GetRetentionPolicyActivity retrieves the retention policy for a data type
func GetRetentionPolicyActivity(ctx context.Context, dataType string) (*RetentionPolicy, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Getting retention policy", "dataType", dataType)

	policies := DefaultRetentionPolicies()
	policy, ok := policies[dataType]
	if !ok {
		return nil, fmt.Errorf("unknown data type: %s", dataType)
	}

	return &policy, nil
}

// CheckComplianceHoldActivity checks if a data type has a compliance hold
func (a *RetentionActivities) CheckComplianceHoldActivity(ctx context.Context, dataType string) (bool, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Checking compliance hold", "dataType", dataType)

	// Check for active compliance holds in the database
	var count int
	err := a.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM compliance_holds 
		WHERE data_type = ? AND status = 'active' AND (expires_at IS NULL OR expires_at > NOW())
	`, dataType).Scan(&count)
	if err != nil {
		// If table doesn't exist, assume no hold
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}

	return count > 0, nil
}

// MoveToWarmStorageActivity moves data to warm storage
func (a *RetentionActivities) MoveToWarmStorageActivity(ctx context.Context, input MoveDataInput) (*TierMoveResult, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Moving data to warm storage", "dataType", input.DataType, "olderThan", input.OlderThan)

	result := &TierMoveResult{}

	if input.DryRun {
		// Count records that would be moved
		var count int64
		err := a.db.QueryRowContext(ctx, fmt.Sprintf(`
			SELECT COUNT(*) FROM %s 
			WHERE created_at < ? AND storage_tier = 'hot'
		`, input.DataType), time.Now().Add(-input.OlderThan)).Scan(&count)
		if err != nil && err != sql.ErrNoRows {
			return nil, err
		}
		result.RecordsMoved = count
		return result, nil
	}

	// Get records to move
	cutoff := time.Now().Add(-input.OlderThan)
	rows, err := a.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT id, data FROM %s 
		WHERE created_at < ? AND storage_tier = 'hot'
		LIMIT 10000
	`, input.DataType), cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		activity.RecordHeartbeat(ctx, result.RecordsMoved)

		var id string
		var data []byte
		if err := rows.Scan(&id, &data); err != nil {
			continue
		}

		// Store in warm storage
		key := fmt.Sprintf("%s/%s", input.DataType, id)
		if err := a.warmStorage.Store(ctx, key, data); err != nil {
			log.Printf("Failed to store %s in warm storage: %v", key, err)
			continue
		}

		// Update storage tier in database
		_, err := a.db.ExecContext(ctx, fmt.Sprintf(`
			UPDATE %s SET storage_tier = 'warm', data = NULL WHERE id = ?
		`, input.DataType), id)
		if err != nil {
			log.Printf("Failed to update storage tier for %s: %v", id, err)
			continue
		}

		result.RecordsMoved++
		result.BytesMoved += int64(len(data))
	}

	return result, nil
}

// MoveToColdStorageActivity moves data to cold storage
func (a *RetentionActivities) MoveToColdStorageActivity(ctx context.Context, input MoveDataInput) (*TierMoveResult, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Moving data to cold storage", "dataType", input.DataType, "olderThan", input.OlderThan)

	result := &TierMoveResult{}

	if input.DryRun {
		// Count records that would be moved
		var count int64
		err := a.db.QueryRowContext(ctx, fmt.Sprintf(`
			SELECT COUNT(*) FROM %s 
			WHERE created_at < ? AND storage_tier = 'warm'
		`, input.DataType), time.Now().Add(-input.OlderThan)).Scan(&count)
		if err != nil && err != sql.ErrNoRows {
			return nil, err
		}
		result.RecordsMoved = count
		return result, nil
	}

	// Get records from warm storage to move to cold
	cutoff := time.Now().Add(-input.OlderThan)
	rows, err := a.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT id FROM %s 
		WHERE created_at < ? AND storage_tier = 'warm'
		LIMIT 10000
	`, input.DataType), cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		activity.RecordHeartbeat(ctx, result.RecordsMoved)

		var id string
		if err := rows.Scan(&id); err != nil {
			continue
		}

		// Get data from warm storage
		key := fmt.Sprintf("%s/%s", input.DataType, id)
		warmKeys, err := a.warmStorage.List(ctx, key)
		if err != nil || len(warmKeys) == 0 {
			continue
		}

		// Move to cold storage (compress and archive)
		if err := a.coldStorage.Store(ctx, key, nil); err != nil {
			log.Printf("Failed to store %s in cold storage: %v", key, err)
			continue
		}

		// Delete from warm storage
		if err := a.warmStorage.Delete(ctx, key); err != nil {
			log.Printf("Failed to delete %s from warm storage: %v", key, err)
		}

		// Update storage tier in database
		_, err = a.db.ExecContext(ctx, fmt.Sprintf(`
			UPDATE %s SET storage_tier = 'cold' WHERE id = ?
		`, input.DataType), id)
		if err != nil {
			log.Printf("Failed to update storage tier for %s: %v", id, err)
			continue
		}

		result.RecordsMoved++
	}

	return result, nil
}

// AnonymizeDataActivity anonymizes PII in data
func (a *RetentionActivities) AnonymizeDataActivity(ctx context.Context, input AnonymizeInput) (*AnonymizeResult, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Anonymizing data", "dataType", input.DataType, "method", input.Method)

	result := &AnonymizeResult{}

	// Define PII fields for each data type
	piiFields := map[string][]string{
		"beneficiary_data": {"first_name", "last_name", "email", "phone", "address", "national_id"},
		"transactions":     {"beneficiary_name", "account_number"},
		"audit_logs":       {"user_email", "ip_address"},
		"workflow_history": {"initiated_by_email"},
	}

	fields, ok := piiFields[input.DataType]
	if !ok {
		return result, nil
	}

	cutoff := time.Now().Add(-input.OlderThan)

	if input.DryRun {
		var count int64
		err := a.db.QueryRowContext(ctx, fmt.Sprintf(`
			SELECT COUNT(*) FROM %s 
			WHERE created_at < ? AND anonymized = false
		`, input.DataType), cutoff).Scan(&count)
		if err != nil && err != sql.ErrNoRows {
			return nil, err
		}
		result.RecordsAnonymized = count
		return result, nil
	}

	// Build update query based on method
	var updates []string
	for _, field := range fields {
		if input.Method == "anonymize" {
			updates = append(updates, fmt.Sprintf("%s = '[REDACTED]'", field))
		} else if input.Method == "pseudonymize" {
			updates = append(updates, fmt.Sprintf("%s = SHA2(%s, 256)", field, field))
		}
	}

	if len(updates) == 0 {
		return result, nil
	}

	query := fmt.Sprintf(`
		UPDATE %s SET %s, anonymized = true 
		WHERE created_at < ? AND anonymized = false
		LIMIT 10000
	`, input.DataType, joinStrings(updates, ", "))

	res, err := a.db.ExecContext(ctx, query, cutoff)
	if err != nil {
		return nil, err
	}

	affected, _ := res.RowsAffected()
	result.RecordsAnonymized = affected

	return result, nil
}

// DeleteExpiredDataActivity deletes expired data
func (a *RetentionActivities) DeleteExpiredDataActivity(ctx context.Context, input DeleteInput) (*DeleteResult, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Deleting expired data", "dataType", input.DataType, "olderThan", input.OlderThan)

	result := &DeleteResult{}
	cutoff := time.Now().Add(-input.OlderThan)

	if input.DryRun {
		var count int64
		err := a.db.QueryRowContext(ctx, fmt.Sprintf(`
			SELECT COUNT(*) FROM %s WHERE created_at < ?
		`, input.DataType), cutoff).Scan(&count)
		if err != nil && err != sql.ErrNoRows {
			return nil, err
		}
		result.RecordsDeleted = count
		return result, nil
	}

	// Delete from cold storage first
	keys, err := a.coldStorage.List(ctx, input.DataType+"/")
	if err == nil {
		for _, key := range keys {
			if err := a.coldStorage.Delete(ctx, key); err != nil {
				log.Printf("Failed to delete %s from cold storage: %v", key, err)
			}
		}
	}

	// Delete from database
	res, err := a.db.ExecContext(ctx, fmt.Sprintf(`
		DELETE FROM %s WHERE created_at < ? LIMIT 10000
	`, input.DataType), cutoff)
	if err != nil {
		return nil, err
	}

	affected, _ := res.RowsAffected()
	result.RecordsDeleted = affected

	return result, nil
}

// CreateRetentionAuditLogActivity creates an audit log entry for retention operations
func (a *RetentionActivities) CreateRetentionAuditLogActivity(ctx context.Context, input AuditLogInput) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Creating retention audit log", "dataType", input.DataType)

	details := map[string]interface{}{
		"dataType":           input.DataType,
		"initiatedBy":        input.InitiatedBy,
		"recordsProcessed":   input.Result.RecordsProcessed,
		"recordsMovedToWarm": input.Result.RecordsMovedToWarm,
		"recordsMovedToCold": input.Result.RecordsMovedToCold,
		"recordsDeleted":     input.Result.RecordsDeleted,
		"recordsAnonymized":  input.Result.RecordsAnonymized,
		"bytesFreed":         input.Result.BytesFreed,
		"completedAt":        input.Result.CompletedAt,
	}

	if len(input.Result.Errors) > 0 {
		details["errors"] = input.Result.Errors
	}

	return a.auditLogger.Log(ctx, "data_retention", details)
}

// Helper function to join strings
func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}

// StorageClassConfig defines storage class configurations for Kubernetes
type StorageClassConfig struct {
	Name              string            `json:"name"`
	Provisioner       string            `json:"provisioner"`
	ReclaimPolicy     string            `json:"reclaimPolicy"`
	VolumeBindingMode string            `json:"volumeBindingMode"`
	Parameters        map[string]string `json:"parameters"`
}

// GetStorageClassConfigs returns storage class configurations for tiered storage
func GetStorageClassConfigs() []StorageClassConfig {
	return []StorageClassConfig{
		{
			Name:              "hot-storage",
			Provisioner:       "kubernetes.io/gce-pd",
			ReclaimPolicy:     "Retain",
			VolumeBindingMode: "WaitForFirstConsumer",
			Parameters: map[string]string{
				"type":             "pd-ssd",
				"replication-type": "regional-pd",
			},
		},
		{
			Name:              "warm-storage",
			Provisioner:       "kubernetes.io/gce-pd",
			ReclaimPolicy:     "Retain",
			VolumeBindingMode: "WaitForFirstConsumer",
			Parameters: map[string]string{
				"type": "pd-balanced",
			},
		},
		{
			Name:              "cold-storage",
			Provisioner:       "kubernetes.io/gce-pd",
			ReclaimPolicy:     "Retain",
			VolumeBindingMode: "WaitForFirstConsumer",
			Parameters: map[string]string{
				"type": "pd-standard",
			},
		},
	}
}

// GenerateStorageClassYAML generates Kubernetes StorageClass YAML
func GenerateStorageClassYAML() string {
	configs := GetStorageClassConfigs()
	var yaml string

	for _, config := range configs {
		params, _ := json.Marshal(config.Parameters)
		yaml += fmt.Sprintf(`---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: %s
  labels:
    app.kubernetes.io/part-of: social-protection
provisioner: %s
reclaimPolicy: %s
volumeBindingMode: %s
parameters: %s
`, config.Name, config.Provisioner, config.ReclaimPolicy, config.VolumeBindingMode, string(params))
	}

	return yaml
}

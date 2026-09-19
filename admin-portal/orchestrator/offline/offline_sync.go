package offline

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// SyncOperation represents a type of sync operation
type SyncOperation string

const (
	SyncOpCreate SyncOperation = "create"
	SyncOpUpdate SyncOperation = "update"
	SyncOpDelete SyncOperation = "delete"
)

// SyncStatus represents the status of a sync record
type SyncStatus string

const (
	SyncStatusPending   SyncStatus = "pending"
	SyncStatusSynced    SyncStatus = "synced"
	SyncStatusConflict  SyncStatus = "conflict"
	SyncStatusFailed    SyncStatus = "failed"
)

// ConflictResolution represents how to resolve conflicts
type ConflictResolution string

const (
	ConflictServerWins ConflictResolution = "server_wins"
	ConflictClientWins ConflictResolution = "client_wins"
	ConflictMerge      ConflictResolution = "merge"
	ConflictManual     ConflictResolution = "manual"
)

// OfflineRecord represents a record that can be synced offline
type OfflineRecord struct {
	ID            string          `json:"id"`
	EntityType    string          `json:"entityType"`
	EntityID      string          `json:"entityId"`
	Operation     SyncOperation   `json:"operation"`
	Data          json.RawMessage `json:"data"`
	Checksum      string          `json:"checksum"`
	ClientVersion int64           `json:"clientVersion"`
	ServerVersion int64           `json:"serverVersion"`
	Status        SyncStatus      `json:"status"`
	DeviceID      string          `json:"deviceId"`
	UserID        int64           `json:"userId"`
	CreatedAt     time.Time       `json:"createdAt"`
	SyncedAt      *time.Time      `json:"syncedAt,omitempty"`
	ConflictData  json.RawMessage `json:"conflictData,omitempty"`
}

// SyncBatch represents a batch of records to sync
type SyncBatch struct {
	ID        string          `json:"id"`
	DeviceID  string          `json:"deviceId"`
	UserID    int64           `json:"userId"`
	Records   []OfflineRecord `json:"records"`
	Timestamp time.Time       `json:"timestamp"`
}

// SyncResult represents the result of a sync operation
type SyncResult struct {
	BatchID       string           `json:"batchId"`
	TotalRecords  int              `json:"totalRecords"`
	SyncedCount   int              `json:"syncedCount"`
	ConflictCount int              `json:"conflictCount"`
	FailedCount   int              `json:"failedCount"`
	Conflicts     []SyncConflict   `json:"conflicts,omitempty"`
	ServerChanges []OfflineRecord  `json:"serverChanges,omitempty"`
	Timestamp     time.Time        `json:"timestamp"`
}

// SyncConflict represents a sync conflict
type SyncConflict struct {
	RecordID     string          `json:"recordId"`
	EntityType   string          `json:"entityType"`
	EntityID     string          `json:"entityId"`
	ClientData   json.RawMessage `json:"clientData"`
	ServerData   json.RawMessage `json:"serverData"`
	Resolution   ConflictResolution `json:"resolution"`
	ResolvedData json.RawMessage `json:"resolvedData,omitempty"`
}

// DeviceRegistration represents a registered offline device
type DeviceRegistration struct {
	ID              string     `json:"id"`
	DeviceID        string     `json:"deviceId"`
	UserID          int64      `json:"userId"`
	DeviceName      string     `json:"deviceName"`
	DeviceType      string     `json:"deviceType"` // android, ios, web
	PushToken       string     `json:"pushToken,omitempty"`
	LastSyncAt      *time.Time `json:"lastSyncAt,omitempty"`
	LastSeenAt      time.Time  `json:"lastSeenAt"`
	OfflineCapacity int64      `json:"offlineCapacity"` // bytes
	Status          string     `json:"status"` // active, inactive, revoked
	CreatedAt       time.Time  `json:"createdAt"`
}

// OfflineSyncService manages offline synchronization
type OfflineSyncService struct {
	db                 *sql.DB
	conflictResolution ConflictResolution
	mu                 sync.RWMutex
	entityHandlers     map[string]EntitySyncHandler
}

// EntitySyncHandler handles sync operations for a specific entity type
type EntitySyncHandler interface {
	GetServerVersion(ctx context.Context, entityID string) (int64, error)
	GetServerData(ctx context.Context, entityID string) (json.RawMessage, error)
	ApplyCreate(ctx context.Context, entityID string, data json.RawMessage, userID int64) error
	ApplyUpdate(ctx context.Context, entityID string, data json.RawMessage, version int64, userID int64) error
	ApplyDelete(ctx context.Context, entityID string, userID int64) error
	MergeData(ctx context.Context, clientData, serverData json.RawMessage) (json.RawMessage, error)
	GetChangesSince(ctx context.Context, since time.Time, limit int) ([]OfflineRecord, error)
}

// NewOfflineSyncService creates a new OfflineSyncService
func NewOfflineSyncService(db *sql.DB, defaultResolution ConflictResolution) *OfflineSyncService {
	return &OfflineSyncService{
		db:                 db,
		conflictResolution: defaultResolution,
		entityHandlers:     make(map[string]EntitySyncHandler),
	}
}

// RegisterEntityHandler registers a handler for an entity type
func (s *OfflineSyncService) RegisterEntityHandler(entityType string, handler EntitySyncHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entityHandlers[entityType] = handler
}

// GetEntityHandler returns the handler for an entity type
func (s *OfflineSyncService) GetEntityHandler(entityType string) EntitySyncHandler {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.entityHandlers[entityType]
}

// RegisterDevice registers a new offline device
func (s *OfflineSyncService) RegisterDevice(ctx context.Context, reg *DeviceRegistration) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO offline_devices (id, device_id, user_id, device_name, device_type, push_token, offline_capacity, status, last_seen_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, 'active', NOW(), NOW())
		ON DUPLICATE KEY UPDATE 
			device_name = VALUES(device_name),
			push_token = VALUES(push_token),
			offline_capacity = VALUES(offline_capacity),
			last_seen_at = NOW(),
			status = 'active'
	`, reg.ID, reg.DeviceID, reg.UserID, reg.DeviceName, reg.DeviceType, reg.PushToken, reg.OfflineCapacity)
	return err
}

// UpdateDeviceLastSeen updates the last seen timestamp for a device
func (s *OfflineSyncService) UpdateDeviceLastSeen(ctx context.Context, deviceID string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE offline_devices SET last_seen_at = NOW() WHERE device_id = ?
	`, deviceID)
	return err
}

// ProcessSyncBatch processes a batch of offline records
func (s *OfflineSyncService) ProcessSyncBatch(ctx context.Context, batch *SyncBatch) (*SyncResult, error) {
	result := &SyncResult{
		BatchID:      batch.ID,
		TotalRecords: len(batch.Records),
		Timestamp:    time.Now(),
	}

	for _, record := range batch.Records {
		handler := s.GetEntityHandler(record.EntityType)
		if handler == nil {
			result.FailedCount++
			continue
		}

		// Check for conflicts
		serverVersion, err := handler.GetServerVersion(ctx, record.EntityID)
		if err != nil && err != sql.ErrNoRows {
			result.FailedCount++
			continue
		}

		// Detect conflict
		if serverVersion > record.ServerVersion && record.Operation != SyncOpCreate {
			conflict, err := s.handleConflict(ctx, handler, &record, serverVersion)
			if err != nil {
				result.FailedCount++
				continue
			}
			if conflict != nil {
				result.Conflicts = append(result.Conflicts, *conflict)
				result.ConflictCount++
				continue
			}
		}

		// Apply the operation
		err = s.applyOperation(ctx, handler, &record)
		if err != nil {
			result.FailedCount++
			continue
		}

		result.SyncedCount++

		// Record the sync
		s.recordSync(ctx, &record)
	}

	// Get server changes since last sync
	serverChanges, err := s.getServerChangesSince(ctx, batch.DeviceID, batch.Timestamp)
	if err == nil {
		result.ServerChanges = serverChanges
	}

	// Update device last sync time
	s.updateDeviceLastSync(ctx, batch.DeviceID)

	return result, nil
}

// handleConflict handles a sync conflict
func (s *OfflineSyncService) handleConflict(ctx context.Context, handler EntitySyncHandler, record *OfflineRecord, serverVersion int64) (*SyncConflict, error) {
	serverData, err := handler.GetServerData(ctx, record.EntityID)
	if err != nil {
		return nil, err
	}

	conflict := &SyncConflict{
		RecordID:   record.ID,
		EntityType: record.EntityType,
		EntityID:   record.EntityID,
		ClientData: record.Data,
		ServerData: serverData,
		Resolution: s.conflictResolution,
	}

	switch s.conflictResolution {
	case ConflictServerWins:
		// Server data wins, no client changes applied
		conflict.ResolvedData = serverData
		return nil, nil // No conflict to report, server wins silently

	case ConflictClientWins:
		// Client data wins, apply client changes
		conflict.ResolvedData = record.Data
		return nil, nil // No conflict to report, will apply client changes

	case ConflictMerge:
		// Attempt to merge
		mergedData, err := handler.MergeData(ctx, record.Data, serverData)
		if err != nil {
			conflict.Resolution = ConflictManual
			return conflict, nil
		}
		conflict.ResolvedData = mergedData
		record.Data = mergedData
		return nil, nil

	case ConflictManual:
		// Return conflict for manual resolution
		return conflict, nil

	default:
		return conflict, nil
	}
}

// applyOperation applies a sync operation
func (s *OfflineSyncService) applyOperation(ctx context.Context, handler EntitySyncHandler, record *OfflineRecord) error {
	switch record.Operation {
	case SyncOpCreate:
		return handler.ApplyCreate(ctx, record.EntityID, record.Data, record.UserID)
	case SyncOpUpdate:
		return handler.ApplyUpdate(ctx, record.EntityID, record.Data, record.ClientVersion, record.UserID)
	case SyncOpDelete:
		return handler.ApplyDelete(ctx, record.EntityID, record.UserID)
	default:
		return fmt.Errorf("unknown operation: %s", record.Operation)
	}
}

// recordSync records a successful sync
func (s *OfflineSyncService) recordSync(ctx context.Context, record *OfflineRecord) {
	now := time.Now()
	s.db.ExecContext(ctx, `
		INSERT INTO offline_sync_log (record_id, entity_type, entity_id, operation, device_id, user_id, synced_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, record.ID, record.EntityType, record.EntityID, record.Operation, record.DeviceID, record.UserID, now)
}

// getServerChangesSince gets server changes since a timestamp
func (s *OfflineSyncService) getServerChangesSince(ctx context.Context, deviceID string, since time.Time) ([]OfflineRecord, error) {
	var changes []OfflineRecord

	// Get last sync time for this device
	var lastSync time.Time
	err := s.db.QueryRowContext(ctx, `
		SELECT COALESCE(last_sync_at, '1970-01-01') FROM offline_devices WHERE device_id = ?
	`, deviceID).Scan(&lastSync)
	if err != nil {
		lastSync = since.Add(-24 * time.Hour) // Default to 24 hours ago
	}

	// Query each registered entity type for changes
	s.mu.RLock()
	handlers := make(map[string]EntitySyncHandler)
	for k, v := range s.entityHandlers {
		handlers[k] = v
	}
	s.mu.RUnlock()

	for entityType, handler := range handlers {
		entityChanges, err := handler.GetChangesSince(ctx, lastSync, 1000)
		if err != nil {
			continue
		}
		for i := range entityChanges {
			entityChanges[i].EntityType = entityType
		}
		changes = append(changes, entityChanges...)
	}

	return changes, nil
}

// updateDeviceLastSync updates the last sync time for a device
func (s *OfflineSyncService) updateDeviceLastSync(ctx context.Context, deviceID string) {
	s.db.ExecContext(ctx, `
		UPDATE offline_devices SET last_sync_at = NOW() WHERE device_id = ?
	`, deviceID)
}

// GetPendingChangesForDevice gets pending server changes for a device
func (s *OfflineSyncService) GetPendingChangesForDevice(ctx context.Context, deviceID string, limit int) ([]OfflineRecord, error) {
	var lastSync time.Time
	err := s.db.QueryRowContext(ctx, `
		SELECT COALESCE(last_sync_at, '1970-01-01') FROM offline_devices WHERE device_id = ?
	`, deviceID).Scan(&lastSync)
	if err != nil {
		return nil, err
	}

	return s.getServerChangesSince(ctx, deviceID, lastSync)
}

// ResolveConflict manually resolves a conflict
func (s *OfflineSyncService) ResolveConflict(ctx context.Context, conflict *SyncConflict, resolution ConflictResolution, resolvedData json.RawMessage) error {
	handler := s.GetEntityHandler(conflict.EntityType)
	if handler == nil {
		return fmt.Errorf("no handler for entity type: %s", conflict.EntityType)
	}

	var dataToApply json.RawMessage
	switch resolution {
	case ConflictServerWins:
		dataToApply = conflict.ServerData
	case ConflictClientWins:
		dataToApply = conflict.ClientData
	case ConflictMerge:
		if resolvedData == nil {
			return fmt.Errorf("resolved data required for merge resolution")
		}
		dataToApply = resolvedData
	default:
		return fmt.Errorf("invalid resolution: %s", resolution)
	}

	// Apply the resolved data
	return handler.ApplyUpdate(ctx, conflict.EntityID, dataToApply, 0, 0)
}

// GenerateChecksum generates a checksum for data
func GenerateChecksum(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// Temporal Workflows

// OfflineSyncWorkflowInput is the input for the sync workflow
type OfflineSyncWorkflowInput struct {
	BatchID   string          `json:"batchId"`
	DeviceID  string          `json:"deviceId"`
	UserID    int64           `json:"userId"`
	Records   []OfflineRecord `json:"records"`
	Timestamp time.Time       `json:"timestamp"`
}

// OfflineSyncWorkflow orchestrates offline synchronization
func OfflineSyncWorkflow(ctx workflow.Context, input OfflineSyncWorkflowInput) (*SyncResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting offline sync workflow", "batchId", input.BatchID, "deviceId", input.DeviceID)

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Minute,
		HeartbeatTimeout:    2 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    time.Minute,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Process the sync batch
	batch := &SyncBatch{
		ID:        input.BatchID,
		DeviceID:  input.DeviceID,
		UserID:    input.UserID,
		Records:   input.Records,
		Timestamp: input.Timestamp,
	}

	var result *SyncResult
	err := workflow.ExecuteActivity(ctx, ProcessSyncBatchActivity, batch).Get(ctx, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to process sync batch: %w", err)
	}

	// If there are conflicts requiring manual resolution, wait for signals
	if result.ConflictCount > 0 {
		for _, conflict := range result.Conflicts {
			if conflict.Resolution == ConflictManual {
				// Wait for conflict resolution signal (with timeout)
				var resolution struct {
					Resolution   ConflictResolution `json:"resolution"`
					ResolvedData json.RawMessage    `json:"resolvedData"`
				}

				signalChan := workflow.GetSignalChannel(ctx, "resolve_conflict_"+conflict.RecordID)
				selector := workflow.NewSelector(ctx)

				selector.AddReceive(signalChan, func(c workflow.ReceiveChannel, more bool) {
					c.Receive(ctx, &resolution)
				})

				// Timeout after 24 hours
				timerFuture := workflow.NewTimer(ctx, 24*time.Hour)
				selector.AddFuture(timerFuture, func(f workflow.Future) {
					// Default to server wins on timeout
					resolution.Resolution = ConflictServerWins
				})

				selector.Select(ctx)

				// Apply resolution
				err := workflow.ExecuteActivity(ctx, ResolveConflictActivity, conflict, resolution.Resolution, resolution.ResolvedData).Get(ctx, nil)
				if err != nil {
					logger.Warn("Failed to resolve conflict", "recordId", conflict.RecordID, "error", err)
				}
			}
		}
	}

	// Send push notification if there are server changes
	if len(result.ServerChanges) > 0 {
		workflow.ExecuteActivity(ctx, NotifyDeviceActivity, input.DeviceID, len(result.ServerChanges)).Get(ctx, nil)
	}

	logger.Info("Offline sync completed", "batchId", input.BatchID, "synced", result.SyncedCount, "conflicts", result.ConflictCount)
	return result, nil
}

// BulkSyncWorkflow handles bulk synchronization for multiple devices
func BulkSyncWorkflow(ctx workflow.Context, deviceIDs []string) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting bulk sync workflow", "deviceCount", len(deviceIDs))

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    5 * time.Minute,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Process devices in parallel with limited concurrency
	const maxConcurrency = 10
	sem := make(chan struct{}, maxConcurrency)

	for _, deviceID := range deviceIDs {
		sem <- struct{}{}

		workflow.Go(ctx, func(ctx workflow.Context) {
			defer func() { <-sem }()

			var changes []OfflineRecord
			err := workflow.ExecuteActivity(ctx, GetPendingChangesActivity, deviceID, 1000).Get(ctx, &changes)
			if err != nil {
				logger.Warn("Failed to get pending changes", "deviceId", deviceID, "error", err)
				return
			}

			if len(changes) > 0 {
				workflow.ExecuteActivity(ctx, NotifyDeviceActivity, deviceID, len(changes)).Get(ctx, nil)
			}
		})
	}

	return nil
}

// Activity implementations

func ProcessSyncBatchActivity(ctx context.Context, batch *SyncBatch) (*SyncResult, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Processing sync batch", "batchId", batch.ID)

	service := getOfflineSyncServiceFromContext(ctx)
	if service == nil {
		return nil, fmt.Errorf("offline sync service not available")
	}

	return service.ProcessSyncBatch(ctx, batch)
}

func ResolveConflictActivity(ctx context.Context, conflict SyncConflict, resolution ConflictResolution, resolvedData json.RawMessage) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Resolving conflict", "recordId", conflict.RecordID)

	service := getOfflineSyncServiceFromContext(ctx)
	if service == nil {
		return fmt.Errorf("offline sync service not available")
	}

	return service.ResolveConflict(ctx, &conflict, resolution, resolvedData)
}

func GetPendingChangesActivity(ctx context.Context, deviceID string, limit int) ([]OfflineRecord, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Getting pending changes", "deviceId", deviceID)

	service := getOfflineSyncServiceFromContext(ctx)
	if service == nil {
		return nil, fmt.Errorf("offline sync service not available")
	}

	return service.GetPendingChangesForDevice(ctx, deviceID, limit)
}

func NotifyDeviceActivity(ctx context.Context, deviceID string, changeCount int) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Notifying device", "deviceId", deviceID, "changeCount", changeCount)

	// In production, send push notification via FCM/APNS
	// For now, log the notification
	logger.Info("Push notification sent", "deviceId", deviceID, "message", fmt.Sprintf("%d changes available for sync", changeCount))
	return nil
}

// Context key for offline sync service
type offlineSyncServiceKey struct{}

func WithOfflineSyncService(ctx context.Context, service *OfflineSyncService) context.Context {
	return context.WithValue(ctx, offlineSyncServiceKey{}, service)
}

func getOfflineSyncServiceFromContext(ctx context.Context) *OfflineSyncService {
	service, _ := ctx.Value(offlineSyncServiceKey{}).(*OfflineSyncService)
	return service
}

// Database migration for offline sync tables
func GetOfflineSyncMigration() string {
	return `
-- Offline devices table
CREATE TABLE IF NOT EXISTS offline_devices (
    id VARCHAR(64) PRIMARY KEY,
    device_id VARCHAR(128) NOT NULL UNIQUE,
    user_id BIGINT NOT NULL,
    device_name VARCHAR(255) NOT NULL,
    device_type ENUM('android', 'ios', 'web') NOT NULL,
    push_token VARCHAR(512),
    offline_capacity BIGINT DEFAULT 0,
    status ENUM('active', 'inactive', 'revoked') NOT NULL DEFAULT 'active',
    last_sync_at TIMESTAMP NULL,
    last_seen_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_user_id (user_id),
    INDEX idx_device_id (device_id),
    INDEX idx_status (status),
    INDEX idx_last_sync (last_sync_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Offline sync log table
CREATE TABLE IF NOT EXISTS offline_sync_log (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    record_id VARCHAR(64) NOT NULL,
    entity_type VARCHAR(64) NOT NULL,
    entity_id VARCHAR(128) NOT NULL,
    operation ENUM('create', 'update', 'delete') NOT NULL,
    device_id VARCHAR(128) NOT NULL,
    user_id BIGINT NOT NULL,
    synced_at TIMESTAMP NOT NULL,
    INDEX idx_device_id (device_id),
    INDEX idx_entity (entity_type, entity_id),
    INDEX idx_synced_at (synced_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Offline conflicts table
CREATE TABLE IF NOT EXISTS offline_conflicts (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    record_id VARCHAR(64) NOT NULL,
    entity_type VARCHAR(64) NOT NULL,
    entity_id VARCHAR(128) NOT NULL,
    client_data JSON NOT NULL,
    server_data JSON NOT NULL,
    resolution ENUM('server_wins', 'client_wins', 'merge', 'manual', 'pending') NOT NULL DEFAULT 'pending',
    resolved_data JSON,
    resolved_by BIGINT,
    device_id VARCHAR(128) NOT NULL,
    user_id BIGINT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    resolved_at TIMESTAMP NULL,
    INDEX idx_status (resolution),
    INDEX idx_entity (entity_type, entity_id),
    INDEX idx_device (device_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
`
}

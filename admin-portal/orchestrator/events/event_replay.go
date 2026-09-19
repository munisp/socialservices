package events

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// Event represents a domain event
type Event struct {
	ID            int64           `json:"id" db:"id"`
	EventType     string          `json:"eventType" db:"event_type"`
	AggregateType string          `json:"aggregateType" db:"aggregate_type"`
	AggregateID   string          `json:"aggregateId" db:"aggregate_id"`
	Payload       json.RawMessage `json:"payload" db:"payload"`
	Metadata      json.RawMessage `json:"metadata,omitempty" db:"metadata"`
	Version       int64           `json:"version" db:"version"`
	CreatedAt     time.Time       `json:"createdAt" db:"created_at"`
	ProcessedAt   *time.Time      `json:"processedAt,omitempty" db:"processed_at"`
}

// ReplayJob represents an event replay job
type ReplayJob struct {
	ID             int64      `json:"id" db:"id"`
	Name           string     `json:"name" db:"name"`
	AggregateType  *string    `json:"aggregateType,omitempty" db:"aggregate_type"`
	AggregateID    *string    `json:"aggregateId,omitempty" db:"aggregate_id"`
	FromTimestamp  *time.Time `json:"fromTimestamp,omitempty" db:"from_timestamp"`
	ToTimestamp    *time.Time `json:"toTimestamp,omitempty" db:"to_timestamp"`
	Status         string     `json:"status" db:"status"` // pending, running, completed, failed, cancelled
	TotalEvents    int        `json:"totalEvents" db:"total_events"`
	ProcessedCount int        `json:"processedCount" db:"processed_count"`
	FailedCount    int        `json:"failedCount" db:"failed_count"`
	ErrorMessage   *string    `json:"errorMessage,omitempty" db:"error_message"`
	StartedAt      *time.Time `json:"startedAt,omitempty" db:"started_at"`
	CompletedAt    *time.Time `json:"completedAt,omitempty" db:"completed_at"`
	CreatedBy      int64      `json:"createdBy" db:"created_by"`
	CreatedAt      time.Time  `json:"createdAt" db:"created_at"`
}

// EventHandler is a function that processes an event
type EventHandler func(ctx context.Context, event *Event) error

// EventReplayService manages event replay operations
type EventReplayService struct {
	db       *sql.DB
	handlers map[string][]EventHandler
	mu       sync.RWMutex
}

// NewEventReplayService creates a new EventReplayService
func NewEventReplayService(db *sql.DB) *EventReplayService {
	return &EventReplayService{
		db:       db,
		handlers: make(map[string][]EventHandler),
	}
}

// RegisterHandler registers an event handler for a specific event type
func (s *EventReplayService) RegisterHandler(eventType string, handler EventHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handlers[eventType] = append(s.handlers[eventType], handler)
}

// GetHandlers returns handlers for an event type
func (s *EventReplayService) GetHandlers(eventType string) []EventHandler {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.handlers[eventType]
}

// StoreEvent stores a new event
func (s *EventReplayService) StoreEvent(ctx context.Context, eventType, aggregateType, aggregateID string, payload interface{}, metadata map[string]interface{}) (*Event, error) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	var metadataBytes []byte
	if metadata != nil {
		metadataBytes, err = json.Marshal(metadata)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal metadata: %w", err)
		}
	}

	// Get next version for this aggregate
	var version int64
	err = s.db.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(version), 0) + 1 FROM events 
		WHERE aggregate_type = ? AND aggregate_id = ?
	`, aggregateType, aggregateID).Scan(&version)
	if err != nil {
		return nil, fmt.Errorf("failed to get version: %w", err)
	}

	result, err := s.db.ExecContext(ctx, `
		INSERT INTO events (event_type, aggregate_type, aggregate_id, payload, metadata, version, created_at)
		VALUES (?, ?, ?, ?, ?, ?, NOW())
	`, eventType, aggregateType, aggregateID, payloadBytes, metadataBytes, version)
	if err != nil {
		return nil, fmt.Errorf("failed to insert event: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get insert id: %w", err)
	}

	return &Event{
		ID:            id,
		EventType:     eventType,
		AggregateType: aggregateType,
		AggregateID:   aggregateID,
		Payload:       payloadBytes,
		Metadata:      metadataBytes,
		Version:       version,
		CreatedAt:     time.Now(),
	}, nil
}

// GetEvents retrieves events with optional filters
func (s *EventReplayService) GetEvents(ctx context.Context, aggregateType, aggregateID *string, fromTime, toTime *time.Time, limit int) ([]Event, error) {
	query := "SELECT id, event_type, aggregate_type, aggregate_id, payload, metadata, version, created_at, processed_at FROM events WHERE 1=1"
	args := []interface{}{}

	if aggregateType != nil {
		query += " AND aggregate_type = ?"
		args = append(args, *aggregateType)
	}
	if aggregateID != nil {
		query += " AND aggregate_id = ?"
		args = append(args, *aggregateID)
	}
	if fromTime != nil {
		query += " AND created_at >= ?"
		args = append(args, *fromTime)
	}
	if toTime != nil {
		query += " AND created_at <= ?"
		args = append(args, *toTime)
	}

	query += " ORDER BY created_at ASC"
	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query events: %w", err)
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		var event Event
		if err := rows.Scan(
			&event.ID, &event.EventType, &event.AggregateType, &event.AggregateID,
			&event.Payload, &event.Metadata, &event.Version, &event.CreatedAt, &event.ProcessedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan event: %w", err)
		}
		events = append(events, event)
	}

	return events, nil
}

// GetEventCount returns the count of events matching filters
func (s *EventReplayService) GetEventCount(ctx context.Context, aggregateType, aggregateID *string, fromTime, toTime *time.Time) (int, error) {
	query := "SELECT COUNT(*) FROM events WHERE 1=1"
	args := []interface{}{}

	if aggregateType != nil {
		query += " AND aggregate_type = ?"
		args = append(args, *aggregateType)
	}
	if aggregateID != nil {
		query += " AND aggregate_id = ?"
		args = append(args, *aggregateID)
	}
	if fromTime != nil {
		query += " AND created_at >= ?"
		args = append(args, *fromTime)
	}
	if toTime != nil {
		query += " AND created_at <= ?"
		args = append(args, *toTime)
	}

	var count int
	err := s.db.QueryRowContext(ctx, query, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count events: %w", err)
	}

	return count, nil
}

// CreateReplayJob creates a new replay job
func (s *EventReplayService) CreateReplayJob(ctx context.Context, name string, aggregateType, aggregateID *string, fromTime, toTime *time.Time, userID int64) (*ReplayJob, error) {
	// Count events to replay
	totalEvents, err := s.GetEventCount(ctx, aggregateType, aggregateID, fromTime, toTime)
	if err != nil {
		return nil, fmt.Errorf("failed to count events: %w", err)
	}

	result, err := s.db.ExecContext(ctx, `
		INSERT INTO replay_jobs (name, aggregate_type, aggregate_id, from_timestamp, to_timestamp, status, total_events, processed_count, failed_count, created_by, created_at)
		VALUES (?, ?, ?, ?, ?, 'pending', ?, 0, 0, ?, NOW())
	`, name, aggregateType, aggregateID, fromTime, toTime, totalEvents, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to insert replay job: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get insert id: %w", err)
	}

	return &ReplayJob{
		ID:            id,
		Name:          name,
		AggregateType: aggregateType,
		AggregateID:   aggregateID,
		FromTimestamp: fromTime,
		ToTimestamp:   toTime,
		Status:        "pending",
		TotalEvents:   totalEvents,
		CreatedBy:     userID,
		CreatedAt:     time.Now(),
	}, nil
}

// GetReplayJob retrieves a replay job by ID
func (s *EventReplayService) GetReplayJob(ctx context.Context, jobID int64) (*ReplayJob, error) {
	job := &ReplayJob{}
	err := s.db.QueryRowContext(ctx, `
		SELECT id, name, aggregate_type, aggregate_id, from_timestamp, to_timestamp, 
		       status, total_events, processed_count, failed_count, error_message,
		       started_at, completed_at, created_by, created_at
		FROM replay_jobs WHERE id = ?
	`, jobID).Scan(
		&job.ID, &job.Name, &job.AggregateType, &job.AggregateID, &job.FromTimestamp, &job.ToTimestamp,
		&job.Status, &job.TotalEvents, &job.ProcessedCount, &job.FailedCount, &job.ErrorMessage,
		&job.StartedAt, &job.CompletedAt, &job.CreatedBy, &job.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get replay job: %w", err)
	}
	return job, nil
}

// UpdateReplayJobStatus updates a replay job's status
func (s *EventReplayService) UpdateReplayJobStatus(ctx context.Context, jobID int64, status string, processedCount, failedCount int, errorMsg *string) error {
	var completedAt *time.Time
	if status == "completed" || status == "failed" || status == "cancelled" {
		now := time.Now()
		completedAt = &now
	}

	_, err := s.db.ExecContext(ctx, `
		UPDATE replay_jobs 
		SET status = ?, processed_count = ?, failed_count = ?, error_message = ?, completed_at = ?
		WHERE id = ?
	`, status, processedCount, failedCount, errorMsg, completedAt, jobID)
	return err
}

// StartReplayJob marks a job as started
func (s *EventReplayService) StartReplayJob(ctx context.Context, jobID int64) error {
	now := time.Now()
	_, err := s.db.ExecContext(ctx, `
		UPDATE replay_jobs SET status = 'running', started_at = ? WHERE id = ?
	`, now, jobID)
	return err
}

// ProcessEvent processes a single event through registered handlers
func (s *EventReplayService) ProcessEvent(ctx context.Context, event *Event) error {
	handlers := s.GetHandlers(event.EventType)
	if len(handlers) == 0 {
		// No handlers registered, mark as processed
		now := time.Now()
		_, err := s.db.ExecContext(ctx, `
			UPDATE events SET processed_at = ? WHERE id = ?
		`, now, event.ID)
		return err
	}

	for _, handler := range handlers {
		if err := handler(ctx, event); err != nil {
			return fmt.Errorf("handler failed for event %d: %w", event.ID, err)
		}
	}

	// Mark as processed
	now := time.Now()
	_, err := s.db.ExecContext(ctx, `
		UPDATE events SET processed_at = ? WHERE id = ?
	`, now, event.ID)
	return err
}

// Temporal Workflows

// EventReplayWorkflowInput is the input for the replay workflow
type EventReplayWorkflowInput struct {
	JobID         int64   `json:"jobId"`
	AggregateType *string `json:"aggregateType,omitempty"`
	AggregateID   *string `json:"aggregateId,omitempty"`
	FromTimestamp *string `json:"fromTimestamp,omitempty"` // RFC3339 format
	ToTimestamp   *string `json:"toTimestamp,omitempty"`   // RFC3339 format
	BatchSize     int     `json:"batchSize"`
	InitiatedBy   int64   `json:"initiatedBy"`
}

// EventReplayWorkflowResult is the result of the replay workflow
type EventReplayWorkflowResult struct {
	JobID          int64     `json:"jobId"`
	Status         string    `json:"status"`
	TotalEvents    int       `json:"totalEvents"`
	ProcessedCount int       `json:"processedCount"`
	FailedCount    int       `json:"failedCount"`
	Error          string    `json:"error,omitempty"`
	CompletedAt    time.Time `json:"completedAt"`
}

// EventReplayWorkflow orchestrates event replay
func EventReplayWorkflow(ctx workflow.Context, input EventReplayWorkflowInput) (*EventReplayWorkflowResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting event replay workflow", "jobId", input.JobID)

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

	result := &EventReplayWorkflowResult{
		JobID: input.JobID,
	}

	// Parse timestamps
	var fromTime, toTime *time.Time
	if input.FromTimestamp != nil {
		t, err := time.Parse(time.RFC3339, *input.FromTimestamp)
		if err == nil {
			fromTime = &t
		}
	}
	if input.ToTimestamp != nil {
		t, err := time.Parse(time.RFC3339, *input.ToTimestamp)
		if err == nil {
			toTime = &t
		}
	}

	// Start the job
	err := workflow.ExecuteActivity(ctx, StartReplayJobActivity, input.JobID).Get(ctx, nil)
	if err != nil {
		result.Error = fmt.Sprintf("failed to start job: %v", err)
		result.Status = "failed"
		return result, nil
	}

	// Get total event count
	var totalEvents int
	err = workflow.ExecuteActivity(ctx, GetEventCountActivity, input.AggregateType, input.AggregateID, fromTime, toTime).Get(ctx, &totalEvents)
	if err != nil {
		result.Error = fmt.Sprintf("failed to count events: %v", err)
		result.Status = "failed"
		workflow.ExecuteActivity(ctx, UpdateReplayJobStatusActivity, input.JobID, "failed", 0, 0, &result.Error).Get(ctx, nil)
		return result, nil
	}
	result.TotalEvents = totalEvents

	if totalEvents == 0 {
		result.Status = "completed"
		result.CompletedAt = workflow.Now(ctx)
		workflow.ExecuteActivity(ctx, UpdateReplayJobStatusActivity, input.JobID, "completed", 0, 0, nil).Get(ctx, nil)
		return result, nil
	}

	// Process events in batches
	batchSize := input.BatchSize
	if batchSize <= 0 {
		batchSize = 100
	}

	processedCount := 0
	failedCount := 0
	offset := 0

	for offset < totalEvents {
		// Check for cancellation
		if ctx.Err() != nil {
			result.Status = "cancelled"
			result.ProcessedCount = processedCount
			result.FailedCount = failedCount
			errMsg := "workflow cancelled"
			workflow.ExecuteActivity(ctx, UpdateReplayJobStatusActivity, input.JobID, "cancelled", processedCount, failedCount, &errMsg).Get(ctx, nil)
			return result, nil
		}

		// Fetch batch of events
		var events []Event
		err = workflow.ExecuteActivity(ctx, GetEventsBatchActivity, input.AggregateType, input.AggregateID, fromTime, toTime, offset, batchSize).Get(ctx, &events)
		if err != nil {
			logger.Warn("Failed to fetch events batch", "offset", offset, "error", err)
			break
		}

		if len(events) == 0 {
			break
		}

		// Process each event
		for _, event := range events {
			var processErr error
			err = workflow.ExecuteActivity(ctx, ProcessEventActivity, event).Get(ctx, &processErr)
			if err != nil || processErr != nil {
				failedCount++
				logger.Warn("Failed to process event", "eventId", event.ID, "error", err)
			} else {
				processedCount++
			}
		}

		// Update progress
		workflow.ExecuteActivity(ctx, UpdateReplayJobStatusActivity, input.JobID, "running", processedCount, failedCount, nil).Get(ctx, nil)

		offset += len(events)
	}

	// Complete the job
	result.ProcessedCount = processedCount
	result.FailedCount = failedCount
	result.CompletedAt = workflow.Now(ctx)

	if failedCount > 0 {
		result.Status = "completed_with_errors"
		errMsg := fmt.Sprintf("%d events failed to process", failedCount)
		result.Error = errMsg
		workflow.ExecuteActivity(ctx, UpdateReplayJobStatusActivity, input.JobID, "completed", processedCount, failedCount, &errMsg).Get(ctx, nil)
	} else {
		result.Status = "completed"
		workflow.ExecuteActivity(ctx, UpdateReplayJobStatusActivity, input.JobID, "completed", processedCount, failedCount, nil).Get(ctx, nil)
	}

	logger.Info("Event replay completed", "jobId", input.JobID, "processed", processedCount, "failed", failedCount)
	return result, nil
}

// ScheduledReplayWorkflow runs periodic event replay for consistency checks
func ScheduledReplayWorkflow(ctx workflow.Context, aggregateType string) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting scheduled replay workflow", "aggregateType", aggregateType)

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 1 * time.Hour,
		HeartbeatTimeout:    10 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    5 * time.Minute,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Get events from last 24 hours
	now := workflow.Now(ctx)
	fromTime := now.Add(-24 * time.Hour)

	var events []Event
	err := workflow.ExecuteActivity(ctx, GetEventsBatchActivity, &aggregateType, nil, &fromTime, &now, 0, 10000).Get(ctx, &events)
	if err != nil {
		return fmt.Errorf("failed to fetch events: %w", err)
	}

	logger.Info("Processing events for consistency check", "count", len(events))

	for _, event := range events {
		err = workflow.ExecuteActivity(ctx, ProcessEventActivity, event).Get(ctx, nil)
		if err != nil {
			logger.Warn("Failed to process event in scheduled replay", "eventId", event.ID, "error", err)
		}
	}

	return nil
}

// Activity implementations

type EventActivities struct {
	service *EventReplayService
}

func NewEventActivities(service *EventReplayService) *EventActivities {
	return &EventActivities{service: service}
}

func StartReplayJobActivity(ctx context.Context, jobID int64) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Starting replay job", "jobId", jobID)

	service := getEventServiceFromContext(ctx)
	if service == nil {
		return fmt.Errorf("event service not available")
	}

	return service.StartReplayJob(ctx, jobID)
}

func GetEventCountActivity(ctx context.Context, aggregateType, aggregateID *string, fromTime, toTime *time.Time) (int, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Getting event count")

	service := getEventServiceFromContext(ctx)
	if service == nil {
		return 0, fmt.Errorf("event service not available")
	}

	return service.GetEventCount(ctx, aggregateType, aggregateID, fromTime, toTime)
}

func GetEventsBatchActivity(ctx context.Context, aggregateType, aggregateID *string, fromTime, toTime *time.Time, offset, limit int) ([]Event, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Getting events batch", "offset", offset, "limit", limit)

	service := getEventServiceFromContext(ctx)
	if service == nil {
		return nil, fmt.Errorf("event service not available")
	}

	// Build query with offset
	query := "SELECT id, event_type, aggregate_type, aggregate_id, payload, metadata, version, created_at, processed_at FROM events WHERE 1=1"
	args := []interface{}{}

	if aggregateType != nil {
		query += " AND aggregate_type = ?"
		args = append(args, *aggregateType)
	}
	if aggregateID != nil {
		query += " AND aggregate_id = ?"
		args = append(args, *aggregateID)
	}
	if fromTime != nil {
		query += " AND created_at >= ?"
		args = append(args, *fromTime)
	}
	if toTime != nil {
		query += " AND created_at <= ?"
		args = append(args, *toTime)
	}

	query += fmt.Sprintf(" ORDER BY created_at ASC LIMIT %d OFFSET %d", limit, offset)

	rows, err := service.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query events: %w", err)
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		var event Event
		if err := rows.Scan(
			&event.ID, &event.EventType, &event.AggregateType, &event.AggregateID,
			&event.Payload, &event.Metadata, &event.Version, &event.CreatedAt, &event.ProcessedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan event: %w", err)
		}
		events = append(events, event)
	}

	return events, nil
}

func ProcessEventActivity(ctx context.Context, event Event) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Processing event", "eventId", event.ID, "eventType", event.EventType)

	service := getEventServiceFromContext(ctx)
	if service == nil {
		return fmt.Errorf("event service not available")
	}

	return service.ProcessEvent(ctx, &event)
}

func UpdateReplayJobStatusActivity(ctx context.Context, jobID int64, status string, processedCount, failedCount int, errorMsg *string) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Updating replay job status", "jobId", jobID, "status", status)

	service := getEventServiceFromContext(ctx)
	if service == nil {
		return fmt.Errorf("event service not available")
	}

	return service.UpdateReplayJobStatus(ctx, jobID, status, processedCount, failedCount, errorMsg)
}

// Context key for event service
type eventServiceKey struct{}

func WithEventService(ctx context.Context, service *EventReplayService) context.Context {
	return context.WithValue(ctx, eventServiceKey{}, service)
}

func getEventServiceFromContext(ctx context.Context) *EventReplayService {
	service, _ := ctx.Value(eventServiceKey{}).(*EventReplayService)
	return service
}

// Database migration for event tables
func GetEventMigration() string {
	return `
-- Events table for event sourcing
CREATE TABLE IF NOT EXISTS events (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    event_type VARCHAR(128) NOT NULL,
    aggregate_type VARCHAR(64) NOT NULL,
    aggregate_id VARCHAR(128) NOT NULL,
    payload JSON NOT NULL,
    metadata JSON,
    version BIGINT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    processed_at TIMESTAMP NULL,
    INDEX idx_aggregate (aggregate_type, aggregate_id),
    INDEX idx_event_type (event_type),
    INDEX idx_created_at (created_at),
    INDEX idx_version (aggregate_type, aggregate_id, version),
    UNIQUE KEY uk_aggregate_version (aggregate_type, aggregate_id, version)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Replay jobs table
CREATE TABLE IF NOT EXISTS replay_jobs (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    aggregate_type VARCHAR(64),
    aggregate_id VARCHAR(128),
    from_timestamp TIMESTAMP NULL,
    to_timestamp TIMESTAMP NULL,
    status ENUM('pending', 'running', 'completed', 'failed', 'cancelled') NOT NULL DEFAULT 'pending',
    total_events INT NOT NULL DEFAULT 0,
    processed_count INT NOT NULL DEFAULT 0,
    failed_count INT NOT NULL DEFAULT 0,
    error_message TEXT,
    started_at TIMESTAMP NULL,
    completed_at TIMESTAMP NULL,
    created_by BIGINT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_status (status),
    INDEX idx_created_at (created_at),
    INDEX idx_created_by (created_by)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
`
}

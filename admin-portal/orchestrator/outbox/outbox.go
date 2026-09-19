package outbox

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// OutboxEntry represents a message in the outbox table
type OutboxEntry struct {
	ID            string          `json:"id" db:"id"`
	AggregateType string          `json:"aggregateType" db:"aggregate_type"`
	AggregateID   string          `json:"aggregateId" db:"aggregate_id"`
	EventType     string          `json:"eventType" db:"event_type"`
	Payload       json.RawMessage `json:"payload" db:"payload"`
	CreatedAt     time.Time       `json:"createdAt" db:"created_at"`
	ProcessedAt   *time.Time      `json:"processedAt,omitempty" db:"processed_at"`
	RetryCount    int             `json:"retryCount" db:"retry_count"`
	LastError     *string         `json:"lastError,omitempty" db:"last_error"`
}

// OutboxPublisher handles writing events to the outbox table
type OutboxPublisher struct {
	db *sql.DB
}

// NewOutboxPublisher creates a new outbox publisher
func NewOutboxPublisher(db *sql.DB) *OutboxPublisher {
	return &OutboxPublisher{db: db}
}

// Publish writes an event to the outbox table within the same transaction
func (p *OutboxPublisher) Publish(ctx context.Context, tx *sql.Tx, aggregateType, aggregateID, eventType string, payload interface{}) error {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	entry := OutboxEntry{
		ID:            uuid.New().String(),
		AggregateType: aggregateType,
		AggregateID:   aggregateID,
		EventType:     eventType,
		Payload:       payloadBytes,
		CreatedAt:     time.Now().UTC(),
		RetryCount:    0,
	}

	query := `
		INSERT INTO outbox (id, aggregate_type, aggregate_id, event_type, payload, created_at, retry_count)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	_, err = tx.ExecContext(ctx, query,
		entry.ID,
		entry.AggregateType,
		entry.AggregateID,
		entry.EventType,
		entry.Payload,
		entry.CreatedAt,
		entry.RetryCount,
	)
	if err != nil {
		return fmt.Errorf("failed to insert outbox entry: %w", err)
	}

	return nil
}

// PublishWithoutTx writes an event to the outbox table without an existing transaction
func (p *OutboxPublisher) PublishWithoutTx(ctx context.Context, aggregateType, aggregateID, eventType string, payload interface{}) error {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	if err := p.Publish(ctx, tx, aggregateType, aggregateID, eventType, payload); err != nil {
		return err
	}

	return tx.Commit()
}

// OutboxProcessor handles reading and processing events from the outbox table
type OutboxProcessor struct {
	db            *sql.DB
	publisher     EventPublisher
	batchSize     int
	maxRetries    int
	retryInterval time.Duration
}

// EventPublisher interface for publishing events to external systems (Kafka, etc.)
type EventPublisher interface {
	Publish(ctx context.Context, topic string, key string, payload []byte) error
}

// NewOutboxProcessor creates a new outbox processor
func NewOutboxProcessor(db *sql.DB, publisher EventPublisher, batchSize, maxRetries int, retryInterval time.Duration) *OutboxProcessor {
	return &OutboxProcessor{
		db:            db,
		publisher:     publisher,
		batchSize:     batchSize,
		maxRetries:    maxRetries,
		retryInterval: retryInterval,
	}
}

// ProcessPendingEntries processes unprocessed outbox entries
func (p *OutboxProcessor) ProcessPendingEntries(ctx context.Context) (int, error) {
	query := `
		SELECT id, aggregate_type, aggregate_id, event_type, payload, created_at, retry_count, last_error
		FROM outbox
		WHERE processed_at IS NULL AND retry_count < ?
		ORDER BY created_at ASC
		LIMIT ?
		FOR UPDATE SKIP LOCKED
	`

	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	rows, err := tx.QueryContext(ctx, query, p.maxRetries, p.batchSize)
	if err != nil {
		return 0, fmt.Errorf("failed to query outbox entries: %w", err)
	}
	defer rows.Close()

	var entries []OutboxEntry
	for rows.Next() {
		var entry OutboxEntry
		if err := rows.Scan(
			&entry.ID,
			&entry.AggregateType,
			&entry.AggregateID,
			&entry.EventType,
			&entry.Payload,
			&entry.CreatedAt,
			&entry.RetryCount,
			&entry.LastError,
		); err != nil {
			return 0, fmt.Errorf("failed to scan outbox entry: %w", err)
		}
		entries = append(entries, entry)
	}

	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("error iterating outbox entries: %w", err)
	}

	processedCount := 0
	for _, entry := range entries {
		topic := fmt.Sprintf("%s.%s", entry.AggregateType, entry.EventType)
		key := entry.AggregateID

		if err := p.publisher.Publish(ctx, topic, key, entry.Payload); err != nil {
			// Update retry count and last error
			errStr := err.Error()
			updateQuery := `
				UPDATE outbox
				SET retry_count = retry_count + 1, last_error = ?
				WHERE id = ?
			`
			if _, err := tx.ExecContext(ctx, updateQuery, errStr, entry.ID); err != nil {
				return processedCount, fmt.Errorf("failed to update retry count: %w", err)
			}
			continue
		}

		// Mark as processed
		now := time.Now().UTC()
		updateQuery := `
			UPDATE outbox
			SET processed_at = ?
			WHERE id = ?
		`
		if _, err := tx.ExecContext(ctx, updateQuery, now, entry.ID); err != nil {
			return processedCount, fmt.Errorf("failed to mark entry as processed: %w", err)
		}
		processedCount++
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return processedCount, nil
}

// CleanupProcessedEntries removes old processed entries
func (p *OutboxProcessor) CleanupProcessedEntries(ctx context.Context, olderThan time.Duration) (int64, error) {
	cutoff := time.Now().UTC().Add(-olderThan)
	query := `
		DELETE FROM outbox
		WHERE processed_at IS NOT NULL AND processed_at < ?
	`

	result, err := p.db.ExecContext(ctx, query, cutoff)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup processed entries: %w", err)
	}

	return result.RowsAffected()
}

// GetDeadLetterEntries returns entries that have exceeded max retries
func (p *OutboxProcessor) GetDeadLetterEntries(ctx context.Context, limit int) ([]OutboxEntry, error) {
	query := `
		SELECT id, aggregate_type, aggregate_id, event_type, payload, created_at, retry_count, last_error
		FROM outbox
		WHERE processed_at IS NULL AND retry_count >= ?
		ORDER BY created_at ASC
		LIMIT ?
	`

	rows, err := p.db.QueryContext(ctx, query, p.maxRetries, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query dead letter entries: %w", err)
	}
	defer rows.Close()

	var entries []OutboxEntry
	for rows.Next() {
		var entry OutboxEntry
		if err := rows.Scan(
			&entry.ID,
			&entry.AggregateType,
			&entry.AggregateID,
			&entry.EventType,
			&entry.Payload,
			&entry.CreatedAt,
			&entry.RetryCount,
			&entry.LastError,
		); err != nil {
			return nil, fmt.Errorf("failed to scan dead letter entry: %w", err)
		}
		entries = append(entries, entry)
	}

	return entries, rows.Err()
}

// RetryDeadLetterEntry resets retry count for a dead letter entry
func (p *OutboxProcessor) RetryDeadLetterEntry(ctx context.Context, entryID string) error {
	query := `
		UPDATE outbox
		SET retry_count = 0, last_error = NULL
		WHERE id = ? AND processed_at IS NULL
	`

	result, err := p.db.ExecContext(ctx, query, entryID)
	if err != nil {
		return fmt.Errorf("failed to retry dead letter entry: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}

	if affected == 0 {
		return fmt.Errorf("entry not found or already processed")
	}

	return nil
}

// OutboxMigration returns the SQL to create the outbox table
func OutboxMigration() string {
	return `
		CREATE TABLE IF NOT EXISTS outbox (
			id VARCHAR(36) PRIMARY KEY,
			aggregate_type VARCHAR(100) NOT NULL,
			aggregate_id VARCHAR(100) NOT NULL,
			event_type VARCHAR(100) NOT NULL,
			payload JSON NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			processed_at TIMESTAMP NULL,
			retry_count INT NOT NULL DEFAULT 0,
			last_error TEXT NULL,
			INDEX idx_outbox_pending (processed_at, retry_count, created_at),
			INDEX idx_outbox_aggregate (aggregate_type, aggregate_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
	`
}

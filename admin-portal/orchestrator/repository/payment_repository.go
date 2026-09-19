package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/lib/pq"
)

// PaymentRepository handles payment-related database operations
type PaymentRepository struct {
	db *sql.DB
}

// NewPaymentRepository creates a new PaymentRepository
func NewPaymentRepository(db *sql.DB) *PaymentRepository {
	return &PaymentRepository{db: db}
}

// TigerBeetleAccountMapping represents a mapping between external IDs and TigerBeetle accounts
type TigerBeetleAccountMapping struct {
	ID                   int64     `json:"id"`
	ExternalID           string    `json:"externalId"`
	ExternalType         string    `json:"externalType"` // program, beneficiary, fee, treasury, escrow
	TigerBeetleAccountID string    `json:"tigerBeetleAccountId"`
	Ledger               int       `json:"ledger"`
	Code                 int       `json:"code"`
	CreatedAt            time.Time `json:"createdAt"`
	UpdatedAt            time.Time `json:"updatedAt"`
}

// PaymentIntent represents a payment intent for idempotency and tracking
type PaymentIntent struct {
	ID                    int64      `json:"id"`
	IntentID              string     `json:"intentId"`
	DisbursementID        string     `json:"disbursementId,omitempty"`
	BeneficiaryID         string     `json:"beneficiaryId"`
	ProgramID             string     `json:"programId"`
	AmountCents           int64      `json:"amountCents"`
	Currency              string     `json:"currency"`
	FeeCents              int64      `json:"feeCents"`
	Status                string     `json:"status"` // pending, reserved, committed, voided, failed
	TigerBeetlePendingID  string     `json:"tigerBeetlePendingId,omitempty"`
	TigerBeetleCommitID   string     `json:"tigerBeetleCommitId,omitempty"`
	MojaloopQuoteID       string     `json:"mojaloopQuoteId,omitempty"`
	MojaloopTransferID    string     `json:"mojaloopTransferId,omitempty"`
	MojaloopTransferState string     `json:"mojaloopTransferState,omitempty"`
	SettlementCycleID     string     `json:"settlementCycleId,omitempty"`
	ErrorMessage          string     `json:"errorMessage,omitempty"`
	CreatedAt             time.Time  `json:"createdAt"`
	UpdatedAt             time.Time  `json:"updatedAt"`
	ReservedAt            *time.Time `json:"reservedAt,omitempty"`
	CommittedAt           *time.Time `json:"committedAt,omitempty"`
	VoidedAt              *time.Time `json:"voidedAt,omitempty"`
}

// MojaloopCallback represents a callback correlation record
type MojaloopCallback struct {
	ID              int64           `json:"id"`
	CallbackType    string          `json:"callbackType"` // party, quote, transfer, bulk_quote, bulk_transfer
	CorrelationID   string          `json:"correlationId"`
	PartyIDType     string          `json:"partyIdType,omitempty"`
	PartyIdentifier string          `json:"partyIdentifier,omitempty"`
	Status          string          `json:"status"` // pending, received, processed, expired
	RequestPayload  json.RawMessage `json:"requestPayload,omitempty"`
	ResponsePayload json.RawMessage `json:"responsePayload,omitempty"`
	ErrorPayload    json.RawMessage `json:"errorPayload,omitempty"`
	WorkflowRunID   string          `json:"workflowRunId,omitempty"`
	CreatedAt       time.Time       `json:"createdAt"`
	UpdatedAt       time.Time       `json:"updatedAt"`
	ExpiresAt       *time.Time      `json:"expiresAt,omitempty"`
	ReceivedAt      *time.Time      `json:"receivedAt,omitempty"`
}

// PaymentEvent represents a payment lifecycle event
type PaymentEvent struct {
	ID              int64           `json:"id"`
	EventID         string          `json:"eventId"`
	EventType       string          `json:"eventType"`
	PaymentIntentID string          `json:"paymentIntentId"`
	CorrelationID   string          `json:"correlationId,omitempty"`
	Payload         json.RawMessage `json:"payload"`
	PublishedToKafka bool           `json:"publishedToKafka"`
	PublishedAt     *time.Time      `json:"publishedAt,omitempty"`
	CreatedAt       time.Time       `json:"createdAt"`
}

// ActivityIdempotency represents an activity idempotency record
type ActivityIdempotency struct {
	ID             int64           `json:"id"`
	IdempotencyKey string          `json:"idempotencyKey"`
	ActivityType   string          `json:"activityType"`
	WorkflowID     string          `json:"workflowId,omitempty"`
	RunID          string          `json:"runId,omitempty"`
	Status         string          `json:"status"` // processing, completed, failed
	Result         json.RawMessage `json:"result,omitempty"`
	ErrorMessage   string          `json:"errorMessage,omitempty"`
	CreatedAt      time.Time       `json:"createdAt"`
	UpdatedAt      time.Time       `json:"updatedAt"`
	ExpiresAt      *time.Time      `json:"expiresAt,omitempty"`
}

// GetOrCreateAccountMapping gets an existing mapping or creates a new one
func (r *PaymentRepository) GetOrCreateAccountMapping(ctx context.Context, externalID, externalType, tigerBeetleAccountID string, ledger, code int) (*TigerBeetleAccountMapping, error) {
	// Try to get existing mapping first
	mapping, err := r.GetAccountMapping(ctx, externalID, externalType)
	if err == nil && mapping != nil {
		return mapping, nil
	}

	// Create new mapping
	query := `
		INSERT INTO tigerbeetle_account_mappings (external_id, external_type, tigerbeetle_account_id, ledger, code)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (external_id, external_type) DO UPDATE SET updated_at = NOW()
		RETURNING id, external_id, external_type, tigerbeetle_account_id, ledger, code, created_at, updated_at
	`
	mapping = &TigerBeetleAccountMapping{}
	err = r.db.QueryRowContext(ctx, query, externalID, externalType, tigerBeetleAccountID, ledger, code).Scan(
		&mapping.ID, &mapping.ExternalID, &mapping.ExternalType, &mapping.TigerBeetleAccountID,
		&mapping.Ledger, &mapping.Code, &mapping.CreatedAt, &mapping.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create account mapping: %w", err)
	}
	return mapping, nil
}

// GetAccountMapping retrieves an account mapping by external ID and type
func (r *PaymentRepository) GetAccountMapping(ctx context.Context, externalID, externalType string) (*TigerBeetleAccountMapping, error) {
	query := `
		SELECT id, external_id, external_type, tigerbeetle_account_id, ledger, code, created_at, updated_at
		FROM tigerbeetle_account_mappings
		WHERE external_id = $1 AND external_type = $2
	`
	mapping := &TigerBeetleAccountMapping{}
	err := r.db.QueryRowContext(ctx, query, externalID, externalType).Scan(
		&mapping.ID, &mapping.ExternalID, &mapping.ExternalType, &mapping.TigerBeetleAccountID,
		&mapping.Ledger, &mapping.Code, &mapping.CreatedAt, &mapping.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get account mapping: %w", err)
	}
	return mapping, nil
}

// GetAccountMappingByTigerBeetleID retrieves an account mapping by TigerBeetle account ID
func (r *PaymentRepository) GetAccountMappingByTigerBeetleID(ctx context.Context, tigerBeetleAccountID string) (*TigerBeetleAccountMapping, error) {
	query := `
		SELECT id, external_id, external_type, tigerbeetle_account_id, ledger, code, created_at, updated_at
		FROM tigerbeetle_account_mappings
		WHERE tigerbeetle_account_id = $1
	`
	mapping := &TigerBeetleAccountMapping{}
	err := r.db.QueryRowContext(ctx, query, tigerBeetleAccountID).Scan(
		&mapping.ID, &mapping.ExternalID, &mapping.ExternalType, &mapping.TigerBeetleAccountID,
		&mapping.Ledger, &mapping.Code, &mapping.CreatedAt, &mapping.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get account mapping: %w", err)
	}
	return mapping, nil
}

// CreatePaymentIntent creates a new payment intent
func (r *PaymentRepository) CreatePaymentIntent(ctx context.Context, intent *PaymentIntent) error {
	query := `
		INSERT INTO payment_intents (intent_id, disbursement_id, beneficiary_id, program_id, amount_cents, currency, fee_cents, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (intent_id) DO NOTHING
		RETURNING id, created_at, updated_at
	`
	err := r.db.QueryRowContext(ctx, query,
		intent.IntentID, intent.DisbursementID, intent.BeneficiaryID, intent.ProgramID,
		intent.AmountCents, intent.Currency, intent.FeeCents, intent.Status,
	).Scan(&intent.ID, &intent.CreatedAt, &intent.UpdatedAt)
	if err == sql.ErrNoRows {
		// Already exists, fetch it
		return r.getPaymentIntentByID(ctx, intent)
	}
	if err != nil {
		return fmt.Errorf("failed to create payment intent: %w", err)
	}
	return nil
}

func (r *PaymentRepository) getPaymentIntentByID(ctx context.Context, intent *PaymentIntent) error {
	query := `
		SELECT id, intent_id, disbursement_id, beneficiary_id, program_id, amount_cents, currency, fee_cents,
		       status, tigerbeetle_pending_id, tigerbeetle_commit_id, mojaloop_quote_id, mojaloop_transfer_id,
		       mojaloop_transfer_state, settlement_cycle_id, error_message, created_at, updated_at,
		       reserved_at, committed_at, voided_at
		FROM payment_intents WHERE intent_id = $1
	`
	var disbursementID, tbPendingID, tbCommitID, mlQuoteID, mlTransferID, mlTransferState, settlementCycleID, errorMsg sql.NullString
	err := r.db.QueryRowContext(ctx, query, intent.IntentID).Scan(
		&intent.ID, &intent.IntentID, &disbursementID, &intent.BeneficiaryID, &intent.ProgramID,
		&intent.AmountCents, &intent.Currency, &intent.FeeCents, &intent.Status,
		&tbPendingID, &tbCommitID, &mlQuoteID, &mlTransferID, &mlTransferState, &settlementCycleID, &errorMsg,
		&intent.CreatedAt, &intent.UpdatedAt, &intent.ReservedAt, &intent.CommittedAt, &intent.VoidedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to get payment intent: %w", err)
	}
	intent.DisbursementID = disbursementID.String
	intent.TigerBeetlePendingID = tbPendingID.String
	intent.TigerBeetleCommitID = tbCommitID.String
	intent.MojaloopQuoteID = mlQuoteID.String
	intent.MojaloopTransferID = mlTransferID.String
	intent.MojaloopTransferState = mlTransferState.String
	intent.SettlementCycleID = settlementCycleID.String
	intent.ErrorMessage = errorMsg.String
	return nil
}

// GetPaymentIntent retrieves a payment intent by intent ID
func (r *PaymentRepository) GetPaymentIntent(ctx context.Context, intentID string) (*PaymentIntent, error) {
	intent := &PaymentIntent{IntentID: intentID}
	err := r.getPaymentIntentByID(ctx, intent)
	if err != nil {
		return nil, err
	}
	return intent, nil
}

// UpdatePaymentIntentStatus updates the status of a payment intent
func (r *PaymentRepository) UpdatePaymentIntentStatus(ctx context.Context, intentID, status string) error {
	query := `UPDATE payment_intents SET status = $2, updated_at = NOW() WHERE intent_id = $1`
	_, err := r.db.ExecContext(ctx, query, intentID, status)
	return err
}

// UpdatePaymentIntentReserved updates a payment intent when funds are reserved
func (r *PaymentRepository) UpdatePaymentIntentReserved(ctx context.Context, intentID, tigerBeetlePendingID string) error {
	query := `
		UPDATE payment_intents 
		SET status = 'reserved', tigerbeetle_pending_id = $2, reserved_at = NOW(), updated_at = NOW()
		WHERE intent_id = $1
	`
	_, err := r.db.ExecContext(ctx, query, intentID, tigerBeetlePendingID)
	return err
}

// UpdatePaymentIntentCommitted updates a payment intent when committed
func (r *PaymentRepository) UpdatePaymentIntentCommitted(ctx context.Context, intentID, tigerBeetleCommitID, mojaloopTransferID string) error {
	query := `
		UPDATE payment_intents 
		SET status = 'committed', tigerbeetle_commit_id = $2, mojaloop_transfer_id = $3, 
		    mojaloop_transfer_state = 'COMMITTED', committed_at = NOW(), updated_at = NOW()
		WHERE intent_id = $1
	`
	_, err := r.db.ExecContext(ctx, query, intentID, tigerBeetleCommitID, mojaloopTransferID)
	return err
}

// UpdatePaymentIntentVoided updates a payment intent when voided
func (r *PaymentRepository) UpdatePaymentIntentVoided(ctx context.Context, intentID, errorMessage string) error {
	query := `
		UPDATE payment_intents 
		SET status = 'voided', error_message = $2, voided_at = NOW(), updated_at = NOW()
		WHERE intent_id = $1
	`
	_, err := r.db.ExecContext(ctx, query, intentID, errorMessage)
	return err
}

// UpdatePaymentIntentMojaloop updates Mojaloop-related fields
func (r *PaymentRepository) UpdatePaymentIntentMojaloop(ctx context.Context, intentID, quoteID, transferID, transferState string) error {
	query := `
		UPDATE payment_intents 
		SET mojaloop_quote_id = $2, mojaloop_transfer_id = $3, mojaloop_transfer_state = $4, updated_at = NOW()
		WHERE intent_id = $1
	`
	_, err := r.db.ExecContext(ctx, query, intentID, quoteID, transferID, transferState)
	return err
}

// CreateMojaloopCallback creates a callback correlation record
func (r *PaymentRepository) CreateMojaloopCallback(ctx context.Context, callback *MojaloopCallback) error {
	query := `
		INSERT INTO mojaloop_callbacks (callback_type, correlation_id, party_id_type, party_identifier, status, request_payload, workflow_run_id, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (callback_type, correlation_id) DO UPDATE SET updated_at = NOW()
		RETURNING id, created_at, updated_at
	`
	err := r.db.QueryRowContext(ctx, query,
		callback.CallbackType, callback.CorrelationID, callback.PartyIDType, callback.PartyIdentifier,
		callback.Status, callback.RequestPayload, callback.WorkflowRunID, callback.ExpiresAt,
	).Scan(&callback.ID, &callback.CreatedAt, &callback.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to create mojaloop callback: %w", err)
	}
	return nil
}

// GetMojaloopCallback retrieves a callback by type and correlation ID
func (r *PaymentRepository) GetMojaloopCallback(ctx context.Context, callbackType, correlationID string) (*MojaloopCallback, error) {
	query := `
		SELECT id, callback_type, correlation_id, party_id_type, party_identifier, status,
		       request_payload, response_payload, error_payload, workflow_run_id,
		       created_at, updated_at, expires_at, received_at
		FROM mojaloop_callbacks
		WHERE callback_type = $1 AND correlation_id = $2
	`
	callback := &MojaloopCallback{}
	var partyIDType, partyIdentifier, workflowRunID sql.NullString
	err := r.db.QueryRowContext(ctx, query, callbackType, correlationID).Scan(
		&callback.ID, &callback.CallbackType, &callback.CorrelationID,
		&partyIDType, &partyIdentifier, &callback.Status,
		&callback.RequestPayload, &callback.ResponsePayload, &callback.ErrorPayload, &workflowRunID,
		&callback.CreatedAt, &callback.UpdatedAt, &callback.ExpiresAt, &callback.ReceivedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get mojaloop callback: %w", err)
	}
	callback.PartyIDType = partyIDType.String
	callback.PartyIdentifier = partyIdentifier.String
	callback.WorkflowRunID = workflowRunID.String
	return callback, nil
}

// UpdateMojaloopCallbackReceived updates a callback when response is received
func (r *PaymentRepository) UpdateMojaloopCallbackReceived(ctx context.Context, callbackType, correlationID string, responsePayload json.RawMessage) error {
	query := `
		UPDATE mojaloop_callbacks 
		SET status = 'received', response_payload = $3, received_at = NOW(), updated_at = NOW()
		WHERE callback_type = $1 AND correlation_id = $2
	`
	_, err := r.db.ExecContext(ctx, query, callbackType, correlationID, responsePayload)
	return err
}

// UpdateMojaloopCallbackError updates a callback when error is received
func (r *PaymentRepository) UpdateMojaloopCallbackError(ctx context.Context, callbackType, correlationID string, errorPayload json.RawMessage) error {
	query := `
		UPDATE mojaloop_callbacks 
		SET status = 'received', error_payload = $3, received_at = NOW(), updated_at = NOW()
		WHERE callback_type = $1 AND correlation_id = $2
	`
	_, err := r.db.ExecContext(ctx, query, callbackType, correlationID, errorPayload)
	return err
}

// CreatePaymentEvent creates a payment event for Kafka publishing
func (r *PaymentRepository) CreatePaymentEvent(ctx context.Context, event *PaymentEvent) error {
	query := `
		INSERT INTO payment_events (event_id, event_type, payment_intent_id, correlation_id, payload)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at
	`
	err := r.db.QueryRowContext(ctx, query,
		event.EventID, event.EventType, event.PaymentIntentID, event.CorrelationID, event.Payload,
	).Scan(&event.ID, &event.CreatedAt)
	if err != nil {
		return fmt.Errorf("failed to create payment event: %w", err)
	}
	return nil
}

// GetUnpublishedPaymentEvents retrieves events not yet published to Kafka
func (r *PaymentRepository) GetUnpublishedPaymentEvents(ctx context.Context, limit int) ([]*PaymentEvent, error) {
	query := `
		SELECT id, event_id, event_type, payment_intent_id, correlation_id, payload, created_at
		FROM payment_events
		WHERE published_to_kafka = FALSE
		ORDER BY created_at ASC
		LIMIT $1
	`
	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get unpublished events: %w", err)
	}
	defer rows.Close()

	var events []*PaymentEvent
	for rows.Next() {
		event := &PaymentEvent{}
		var correlationID sql.NullString
		err := rows.Scan(&event.ID, &event.EventID, &event.EventType, &event.PaymentIntentID,
			&correlationID, &event.Payload, &event.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan event: %w", err)
		}
		event.CorrelationID = correlationID.String
		events = append(events, event)
	}
	return events, nil
}

// MarkPaymentEventPublished marks an event as published to Kafka
func (r *PaymentRepository) MarkPaymentEventPublished(ctx context.Context, eventID string) error {
	query := `UPDATE payment_events SET published_to_kafka = TRUE, published_at = NOW() WHERE event_id = $1`
	_, err := r.db.ExecContext(ctx, query, eventID)
	return err
}

// GetOrCreateActivityIdempotency gets or creates an activity idempotency record
func (r *PaymentRepository) GetOrCreateActivityIdempotency(ctx context.Context, key, activityType, workflowID, runID string, ttl time.Duration) (*ActivityIdempotency, bool, error) {
	// Try to get existing record
	existing, err := r.GetActivityIdempotency(ctx, key)
	if err == nil && existing != nil {
		return existing, true, nil // Already exists
	}

	// Create new record
	expiresAt := time.Now().Add(ttl)
	query := `
		INSERT INTO activity_idempotency (idempotency_key, activity_type, workflow_id, run_id, status, expires_at)
		VALUES ($1, $2, $3, $4, 'processing', $5)
		ON CONFLICT (idempotency_key) DO NOTHING
		RETURNING id, created_at, updated_at
	`
	record := &ActivityIdempotency{
		IdempotencyKey: key,
		ActivityType:   activityType,
		WorkflowID:     workflowID,
		RunID:          runID,
		Status:         "processing",
		ExpiresAt:      &expiresAt,
	}
	err = r.db.QueryRowContext(ctx, query, key, activityType, workflowID, runID, expiresAt).Scan(
		&record.ID, &record.CreatedAt, &record.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		// Race condition - another process created it
		existing, err := r.GetActivityIdempotency(ctx, key)
		if err != nil {
			return nil, false, err
		}
		return existing, true, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("failed to create activity idempotency: %w", err)
	}
	return record, false, nil
}

// GetActivityIdempotency retrieves an activity idempotency record
func (r *PaymentRepository) GetActivityIdempotency(ctx context.Context, key string) (*ActivityIdempotency, error) {
	query := `
		SELECT id, idempotency_key, activity_type, workflow_id, run_id, status, result, error_message, created_at, updated_at, expires_at
		FROM activity_idempotency
		WHERE idempotency_key = $1 AND (expires_at IS NULL OR expires_at > NOW())
	`
	record := &ActivityIdempotency{}
	var workflowID, runID, errorMsg sql.NullString
	err := r.db.QueryRowContext(ctx, query, key).Scan(
		&record.ID, &record.IdempotencyKey, &record.ActivityType, &workflowID, &runID,
		&record.Status, &record.Result, &errorMsg, &record.CreatedAt, &record.UpdatedAt, &record.ExpiresAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get activity idempotency: %w", err)
	}
	record.WorkflowID = workflowID.String
	record.RunID = runID.String
	record.ErrorMessage = errorMsg.String
	return record, nil
}

// UpdateActivityIdempotencyCompleted marks an activity as completed with result
func (r *PaymentRepository) UpdateActivityIdempotencyCompleted(ctx context.Context, key string, result json.RawMessage) error {
	query := `UPDATE activity_idempotency SET status = 'completed', result = $2, updated_at = NOW() WHERE idempotency_key = $1`
	_, err := r.db.ExecContext(ctx, query, key, result)
	return err
}

// UpdateActivityIdempotencyFailed marks an activity as failed with error
func (r *PaymentRepository) UpdateActivityIdempotencyFailed(ctx context.Context, key, errorMessage string) error {
	query := `UPDATE activity_idempotency SET status = 'failed', error_message = $2, updated_at = NOW() WHERE idempotency_key = $1`
	_, err := r.db.ExecContext(ctx, query, key, errorMessage)
	return err
}

// Global repository instance
var globalPaymentRepo *PaymentRepository

// InitPaymentRepository initializes the global payment repository
func InitPaymentRepository(db *sql.DB) {
	globalPaymentRepo = NewPaymentRepository(db)
}

// GetPaymentRepository returns the global payment repository
func GetPaymentRepository() *PaymentRepository {
	return globalPaymentRepo
}

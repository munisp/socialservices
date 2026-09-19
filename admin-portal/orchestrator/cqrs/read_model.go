package cqrs

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/admin-portal/orchestrator/outbox"
)

// ReadModelProjector handles building and maintaining read models from events
type ReadModelProjector struct {
	db            *sql.DB
	eventConsumer EventConsumer
	projections   map[string]Projection
	mu            sync.RWMutex
	lastPosition  int64
}

// EventConsumer interface for consuming events from Kafka/Fluvio
type EventConsumer interface {
	Subscribe(topics []string) error
	Consume(ctx context.Context) (<-chan Event, error)
	Commit(ctx context.Context, event Event) error
}

// Event represents an event consumed from the message broker
type Event struct {
	ID            string          `json:"id"`
	AggregateType string          `json:"aggregateType"`
	AggregateID   string          `json:"aggregateId"`
	EventType     string          `json:"eventType"`
	Payload       json.RawMessage `json:"payload"`
	Timestamp     time.Time       `json:"timestamp"`
	Position      int64           `json:"position"`
}

// Projection interface for building read models
type Projection interface {
	Name() string
	Topics() []string
	Handle(ctx context.Context, event Event) error
	Rebuild(ctx context.Context) error
}

// NewReadModelProjector creates a new read model projector
func NewReadModelProjector(db *sql.DB, consumer EventConsumer) *ReadModelProjector {
	return &ReadModelProjector{
		db:            db,
		eventConsumer: consumer,
		projections:   make(map[string]Projection),
	}
}

// RegisterProjection registers a projection with the projector
func (p *ReadModelProjector) RegisterProjection(projection Projection) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.projections[projection.Name()] = projection
}

// Start begins consuming events and updating read models
func (p *ReadModelProjector) Start(ctx context.Context) error {
	// Collect all topics from registered projections
	topicSet := make(map[string]bool)
	for _, proj := range p.projections {
		for _, topic := range proj.Topics() {
			topicSet[topic] = true
		}
	}

	topics := make([]string, 0, len(topicSet))
	for topic := range topicSet {
		topics = append(topics, topic)
	}

	if err := p.eventConsumer.Subscribe(topics); err != nil {
		return fmt.Errorf("failed to subscribe to topics: %w", err)
	}

	events, err := p.eventConsumer.Consume(ctx)
	if err != nil {
		return fmt.Errorf("failed to start consuming: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event, ok := <-events:
			if !ok {
				return nil
			}
			if err := p.processEvent(ctx, event); err != nil {
				// Log error but continue processing
				fmt.Printf("Error processing event %s: %v\n", event.ID, err)
			}
		}
	}
}

// processEvent routes an event to the appropriate projections
func (p *ReadModelProjector) processEvent(ctx context.Context, event Event) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	topic := fmt.Sprintf("%s.%s", event.AggregateType, event.EventType)

	for _, proj := range p.projections {
		for _, t := range proj.Topics() {
			if t == topic || t == event.AggregateType+".*" {
				if err := proj.Handle(ctx, event); err != nil {
					return fmt.Errorf("projection %s failed: %w", proj.Name(), err)
				}
			}
		}
	}

	// Commit the event after successful processing
	if err := p.eventConsumer.Commit(ctx, event); err != nil {
		return fmt.Errorf("failed to commit event: %w", err)
	}

	p.lastPosition = event.Position
	return nil
}

// RebuildProjection rebuilds a specific projection from scratch
func (p *ReadModelProjector) RebuildProjection(ctx context.Context, name string) error {
	p.mu.RLock()
	proj, ok := p.projections[name]
	p.mu.RUnlock()

	if !ok {
		return fmt.Errorf("projection %s not found", name)
	}

	return proj.Rebuild(ctx)
}

// ========== Beneficiary Summary Projection ==========

// BeneficiarySummaryProjection maintains a denormalized view of beneficiary data
type BeneficiarySummaryProjection struct {
	db *sql.DB
}

// NewBeneficiarySummaryProjection creates a new beneficiary summary projection
func NewBeneficiarySummaryProjection(db *sql.DB) *BeneficiarySummaryProjection {
	return &BeneficiarySummaryProjection{db: db}
}

func (p *BeneficiarySummaryProjection) Name() string {
	return "beneficiary_summary"
}

func (p *BeneficiarySummaryProjection) Topics() []string {
	return []string{
		"beneficiary.created",
		"beneficiary.updated",
		"beneficiary.enrolled",
		"beneficiary.approved",
		"beneficiary.suspended",
		"disbursement.completed",
		"kyc.verified",
	}
}

func (p *BeneficiarySummaryProjection) Handle(ctx context.Context, event Event) error {
	switch event.EventType {
	case "created":
		return p.handleCreated(ctx, event)
	case "updated":
		return p.handleUpdated(ctx, event)
	case "enrolled":
		return p.handleEnrolled(ctx, event)
	case "approved":
		return p.handleApproved(ctx, event)
	case "suspended":
		return p.handleSuspended(ctx, event)
	case "completed":
		if event.AggregateType == "disbursement" {
			return p.handleDisbursement(ctx, event)
		}
	case "verified":
		if event.AggregateType == "kyc" {
			return p.handleKYCVerified(ctx, event)
		}
	}
	return nil
}

func (p *BeneficiarySummaryProjection) handleCreated(ctx context.Context, event Event) error {
	var data struct {
		ID          string `json:"id"`
		FirstName   string `json:"firstName"`
		LastName    string `json:"lastName"`
		NationalID  string `json:"nationalId"`
		PhoneNumber string `json:"phoneNumber"`
		Email       string `json:"email"`
		City        string `json:"city"`
		State       string `json:"state"`
	}
	if err := json.Unmarshal(event.Payload, &data); err != nil {
		return err
	}

	query := `
		INSERT INTO beneficiary_summary (
			beneficiary_id, first_name, last_name, national_id, phone_number, email,
			city, state, status, total_disbursements, total_amount, kyc_status,
			program_count, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'pending', 0, 0, 'pending', 0, ?, ?)
		ON DUPLICATE KEY UPDATE updated_at = VALUES(updated_at)
	`
	_, err := p.db.ExecContext(ctx, query,
		data.ID, data.FirstName, data.LastName, data.NationalID,
		data.PhoneNumber, data.Email, data.City, data.State,
		event.Timestamp, event.Timestamp,
	)
	return err
}

func (p *BeneficiarySummaryProjection) handleUpdated(ctx context.Context, event Event) error {
	var data struct {
		ID          string `json:"id"`
		FirstName   string `json:"firstName,omitempty"`
		LastName    string `json:"lastName,omitempty"`
		PhoneNumber string `json:"phoneNumber,omitempty"`
		Email       string `json:"email,omitempty"`
		City        string `json:"city,omitempty"`
		State       string `json:"state,omitempty"`
	}
	if err := json.Unmarshal(event.Payload, &data); err != nil {
		return err
	}

	query := `
		UPDATE beneficiary_summary SET
			first_name = COALESCE(NULLIF(?, ''), first_name),
			last_name = COALESCE(NULLIF(?, ''), last_name),
			phone_number = COALESCE(NULLIF(?, ''), phone_number),
			email = COALESCE(NULLIF(?, ''), email),
			city = COALESCE(NULLIF(?, ''), city),
			state = COALESCE(NULLIF(?, ''), state),
			updated_at = ?
		WHERE beneficiary_id = ?
	`
	_, err := p.db.ExecContext(ctx, query,
		data.FirstName, data.LastName, data.PhoneNumber,
		data.Email, data.City, data.State,
		event.Timestamp, data.ID,
	)
	return err
}

func (p *BeneficiarySummaryProjection) handleEnrolled(ctx context.Context, event Event) error {
	var data struct {
		BeneficiaryID string `json:"beneficiaryId"`
		ProgramID     string `json:"programId"`
	}
	if err := json.Unmarshal(event.Payload, &data); err != nil {
		return err
	}

	query := `
		UPDATE beneficiary_summary SET
			program_count = program_count + 1,
			updated_at = ?
		WHERE beneficiary_id = ?
	`
	_, err := p.db.ExecContext(ctx, query, event.Timestamp, data.BeneficiaryID)
	return err
}

func (p *BeneficiarySummaryProjection) handleApproved(ctx context.Context, event Event) error {
	var data struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(event.Payload, &data); err != nil {
		return err
	}

	query := `
		UPDATE beneficiary_summary SET
			status = 'approved',
			updated_at = ?
		WHERE beneficiary_id = ?
	`
	_, err := p.db.ExecContext(ctx, query, event.Timestamp, data.ID)
	return err
}

func (p *BeneficiarySummaryProjection) handleSuspended(ctx context.Context, event Event) error {
	var data struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(event.Payload, &data); err != nil {
		return err
	}

	query := `
		UPDATE beneficiary_summary SET
			status = 'suspended',
			updated_at = ?
		WHERE beneficiary_id = ?
	`
	_, err := p.db.ExecContext(ctx, query, event.Timestamp, data.ID)
	return err
}

func (p *BeneficiarySummaryProjection) handleDisbursement(ctx context.Context, event Event) error {
	var data struct {
		BeneficiaryID string  `json:"beneficiaryId"`
		Amount        float64 `json:"amount"`
	}
	if err := json.Unmarshal(event.Payload, &data); err != nil {
		return err
	}

	query := `
		UPDATE beneficiary_summary SET
			total_disbursements = total_disbursements + 1,
			total_amount = total_amount + ?,
			last_disbursement_at = ?,
			updated_at = ?
		WHERE beneficiary_id = ?
	`
	_, err := p.db.ExecContext(ctx, query, data.Amount, event.Timestamp, event.Timestamp, data.BeneficiaryID)
	return err
}

func (p *BeneficiarySummaryProjection) handleKYCVerified(ctx context.Context, event Event) error {
	var data struct {
		BeneficiaryID string `json:"beneficiaryId"`
		Status        string `json:"status"`
	}
	if err := json.Unmarshal(event.Payload, &data); err != nil {
		return err
	}

	query := `
		UPDATE beneficiary_summary SET
			kyc_status = ?,
			updated_at = ?
		WHERE beneficiary_id = ?
	`
	_, err := p.db.ExecContext(ctx, query, data.Status, event.Timestamp, data.BeneficiaryID)
	return err
}

func (p *BeneficiarySummaryProjection) Rebuild(ctx context.Context) error {
	// Truncate and rebuild from source tables
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Clear existing data
	if _, err := tx.ExecContext(ctx, "TRUNCATE TABLE beneficiary_summary"); err != nil {
		return err
	}

	// Rebuild from source
	query := `
		INSERT INTO beneficiary_summary (
			beneficiary_id, first_name, last_name, national_id, phone_number, email,
			city, state, status, total_disbursements, total_amount, kyc_status,
			program_count, created_at, updated_at
		)
		SELECT 
			b.id,
			b.first_name,
			b.last_name,
			b.national_id,
			b.phone_number,
			b.email,
			b.city,
			b.state,
			b.enrollment_status,
			COALESCE(d.disbursement_count, 0),
			COALESCE(d.total_amount, 0),
			b.kyc_status,
			COALESCE(e.program_count, 0),
			b.created_at,
			NOW()
		FROM beneficiaries b
		LEFT JOIN (
			SELECT beneficiary_id, COUNT(*) as disbursement_count, SUM(amount) as total_amount
			FROM transactions
			WHERE type = 'disbursement' AND status = 'completed'
			GROUP BY beneficiary_id
		) d ON b.id = d.beneficiary_id
		LEFT JOIN (
			SELECT beneficiary_id, COUNT(DISTINCT program_id) as program_count
			FROM program_enrollments
			WHERE status = 'active'
			GROUP BY beneficiary_id
		) e ON b.id = e.beneficiary_id
	`
	if _, err := tx.ExecContext(ctx, query); err != nil {
		return err
	}

	return tx.Commit()
}

// ========== Program Analytics Projection ==========

// ProgramAnalyticsProjection maintains aggregated program statistics
type ProgramAnalyticsProjection struct {
	db *sql.DB
}

func NewProgramAnalyticsProjection(db *sql.DB) *ProgramAnalyticsProjection {
	return &ProgramAnalyticsProjection{db: db}
}

func (p *ProgramAnalyticsProjection) Name() string {
	return "program_analytics"
}

func (p *ProgramAnalyticsProjection) Topics() []string {
	return []string{
		"program.*",
		"beneficiary.enrolled",
		"disbursement.completed",
	}
}

func (p *ProgramAnalyticsProjection) Handle(ctx context.Context, event Event) error {
	switch event.EventType {
	case "created":
		return p.handleProgramCreated(ctx, event)
	case "enrolled":
		return p.handleBeneficiaryEnrolled(ctx, event)
	case "completed":
		if event.AggregateType == "disbursement" {
			return p.handleDisbursementCompleted(ctx, event)
		}
	}
	return nil
}

func (p *ProgramAnalyticsProjection) handleProgramCreated(ctx context.Context, event Event) error {
	var data struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(event.Payload, &data); err != nil {
		return err
	}

	query := `
		INSERT INTO program_analytics (
			program_id, program_name, total_beneficiaries, active_beneficiaries,
			total_disbursements, total_amount, avg_disbursement_amount,
			created_at, updated_at
		) VALUES (?, ?, 0, 0, 0, 0, 0, ?, ?)
		ON DUPLICATE KEY UPDATE updated_at = VALUES(updated_at)
	`
	_, err := p.db.ExecContext(ctx, query, data.ID, data.Name, event.Timestamp, event.Timestamp)
	return err
}

func (p *ProgramAnalyticsProjection) handleBeneficiaryEnrolled(ctx context.Context, event Event) error {
	var data struct {
		ProgramID string `json:"programId"`
	}
	if err := json.Unmarshal(event.Payload, &data); err != nil {
		return err
	}

	query := `
		UPDATE program_analytics SET
			total_beneficiaries = total_beneficiaries + 1,
			active_beneficiaries = active_beneficiaries + 1,
			updated_at = ?
		WHERE program_id = ?
	`
	_, err := p.db.ExecContext(ctx, query, event.Timestamp, data.ProgramID)
	return err
}

func (p *ProgramAnalyticsProjection) handleDisbursementCompleted(ctx context.Context, event Event) error {
	var data struct {
		ProgramID string  `json:"programId"`
		Amount    float64 `json:"amount"`
	}
	if err := json.Unmarshal(event.Payload, &data); err != nil {
		return err
	}

	query := `
		UPDATE program_analytics SET
			total_disbursements = total_disbursements + 1,
			total_amount = total_amount + ?,
			avg_disbursement_amount = (total_amount + ?) / (total_disbursements + 1),
			last_disbursement_at = ?,
			updated_at = ?
		WHERE program_id = ?
	`
	_, err := p.db.ExecContext(ctx, query, data.Amount, data.Amount, event.Timestamp, event.Timestamp, data.ProgramID)
	return err
}

func (p *ProgramAnalyticsProjection) Rebuild(ctx context.Context) error {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, "TRUNCATE TABLE program_analytics"); err != nil {
		return err
	}

	query := `
		INSERT INTO program_analytics (
			program_id, program_name, total_beneficiaries, active_beneficiaries,
			total_disbursements, total_amount, avg_disbursement_amount,
			created_at, updated_at
		)
		SELECT 
			p.id,
			p.name,
			COALESCE(e.total_beneficiaries, 0),
			COALESCE(e.active_beneficiaries, 0),
			COALESCE(d.total_disbursements, 0),
			COALESCE(d.total_amount, 0),
			COALESCE(d.total_amount / NULLIF(d.total_disbursements, 0), 0),
			p.created_at,
			NOW()
		FROM benefit_programs p
		LEFT JOIN (
			SELECT program_id, 
				COUNT(*) as total_beneficiaries,
				SUM(CASE WHEN status = 'active' THEN 1 ELSE 0 END) as active_beneficiaries
			FROM program_enrollments
			GROUP BY program_id
		) e ON p.id = e.program_id
		LEFT JOIN (
			SELECT program_id, COUNT(*) as total_disbursements, SUM(amount) as total_amount
			FROM transactions
			WHERE type = 'disbursement' AND status = 'completed'
			GROUP BY program_id
		) d ON p.id = d.program_id
	`
	if _, err := tx.ExecContext(ctx, query); err != nil {
		return err
	}

	return tx.Commit()
}

// ReadModelMigrations returns SQL to create read model tables
func ReadModelMigrations() string {
	return `
		CREATE TABLE IF NOT EXISTS beneficiary_summary (
			beneficiary_id VARCHAR(36) PRIMARY KEY,
			first_name VARCHAR(100),
			last_name VARCHAR(100),
			national_id VARCHAR(50),
			phone_number VARCHAR(20),
			email VARCHAR(255),
			city VARCHAR(100),
			state VARCHAR(100),
			status VARCHAR(20),
			total_disbursements INT DEFAULT 0,
			total_amount DECIMAL(15,2) DEFAULT 0,
			kyc_status VARCHAR(20),
			program_count INT DEFAULT 0,
			last_disbursement_at TIMESTAMP NULL,
			created_at TIMESTAMP NOT NULL,
			updated_at TIMESTAMP NOT NULL,
			INDEX idx_beneficiary_summary_status (status),
			INDEX idx_beneficiary_summary_city_state (city, state),
			INDEX idx_beneficiary_summary_kyc (kyc_status)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

		CREATE TABLE IF NOT EXISTS program_analytics (
			program_id VARCHAR(36) PRIMARY KEY,
			program_name VARCHAR(255),
			total_beneficiaries INT DEFAULT 0,
			active_beneficiaries INT DEFAULT 0,
			total_disbursements INT DEFAULT 0,
			total_amount DECIMAL(15,2) DEFAULT 0,
			avg_disbursement_amount DECIMAL(15,2) DEFAULT 0,
			last_disbursement_at TIMESTAMP NULL,
			created_at TIMESTAMP NOT NULL,
			updated_at TIMESTAMP NOT NULL,
			INDEX idx_program_analytics_name (program_name)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
	`
}

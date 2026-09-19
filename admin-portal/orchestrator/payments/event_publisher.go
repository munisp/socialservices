package payments

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"social-protection-platform/orchestrator/clients"
	"social-protection-platform/orchestrator/repository"

	"github.com/google/uuid"
)

// PaymentEventType represents the type of payment event
type PaymentEventType string

const (
	// TigerBeetle events
	EventPaymentReserved  PaymentEventType = "payment.reserved"
	EventPaymentCommitted PaymentEventType = "payment.committed"
	EventPaymentVoided    PaymentEventType = "payment.voided"
	EventPaymentFailed    PaymentEventType = "payment.failed"

	// Mojaloop events
	EventQuoteRequested   PaymentEventType = "mojaloop.quote.requested"
	EventQuoteReceived    PaymentEventType = "mojaloop.quote.received"
	EventQuoteFailed      PaymentEventType = "mojaloop.quote.failed"
	EventTransferInitiated PaymentEventType = "mojaloop.transfer.initiated"
	EventTransferCommitted PaymentEventType = "mojaloop.transfer.committed"
	EventTransferAborted   PaymentEventType = "mojaloop.transfer.aborted"
	EventTransferFailed    PaymentEventType = "mojaloop.transfer.failed"

	// Settlement events
	EventSettlementCycleCreated PaymentEventType = "settlement.cycle.created"
	EventSettlementCycleClosed  PaymentEventType = "settlement.cycle.closed"
	EventSettlementCompleted    PaymentEventType = "settlement.completed"
)

// PaymentEvent represents a payment lifecycle event
type PaymentEvent struct {
	EventID       string           `json:"eventId"`
	EventType     PaymentEventType `json:"eventType"`
	Timestamp     time.Time        `json:"timestamp"`
	CorrelationID string           `json:"correlationId,omitempty"`
	PaymentIntent *PaymentIntentData `json:"paymentIntent,omitempty"`
	TigerBeetle   *TigerBeetleData   `json:"tigerBeetle,omitempty"`
	Mojaloop      *MojaloopData      `json:"mojaloop,omitempty"`
	Settlement    *SettlementData    `json:"settlement,omitempty"`
	Error         *ErrorData         `json:"error,omitempty"`
}

// PaymentIntentData contains payment intent information
type PaymentIntentData struct {
	IntentID      string `json:"intentId"`
	BeneficiaryID string `json:"beneficiaryId"`
	ProgramID     string `json:"programId"`
	AmountCents   int64  `json:"amountCents"`
	Currency      string `json:"currency"`
	FeeCents      int64  `json:"feeCents,omitempty"`
}

// TigerBeetleData contains TigerBeetle-specific event data
type TigerBeetleData struct {
	PendingTransferID string `json:"pendingTransferId,omitempty"`
	CommitTransferID  string `json:"commitTransferId,omitempty"`
	DebitAccountID    string `json:"debitAccountId,omitempty"`
	CreditAccountID   string `json:"creditAccountId,omitempty"`
	AmountCents       int64  `json:"amountCents"`
}

// MojaloopData contains Mojaloop-specific event data
type MojaloopData struct {
	QuoteID       string `json:"quoteId,omitempty"`
	TransferID    string `json:"transferId,omitempty"`
	PayerFSP      string `json:"payerFsp,omitempty"`
	PayeeFSP      string `json:"payeeFsp,omitempty"`
	TransferState string `json:"transferState,omitempty"`
	Amount        string `json:"amount,omitempty"`
	Currency      string `json:"currency,omitempty"`
}

// SettlementData contains settlement-specific event data
type SettlementData struct {
	CycleID        string `json:"cycleId"`
	Currency       string `json:"currency"`
	TotalTransfers int    `json:"totalTransfers,omitempty"`
	TotalAmount    int64  `json:"totalAmountCents,omitempty"`
}

// ErrorData contains error information
type ErrorData struct {
	Code        string `json:"code,omitempty"`
	Description string `json:"description"`
}

// PaymentEventPublisher publishes payment events to Kafka
type PaymentEventPublisher struct {
	kafkaClient *clients.KafkaClient
	paymentRepo *repository.PaymentRepository
	topic       string
}

// NewPaymentEventPublisher creates a new PaymentEventPublisher
func NewPaymentEventPublisher(kafkaClient *clients.KafkaClient, paymentRepo *repository.PaymentRepository, topic string) *PaymentEventPublisher {
	if topic == "" {
		topic = "payment-events"
	}
	return &PaymentEventPublisher{
		kafkaClient: kafkaClient,
		paymentRepo: paymentRepo,
		topic:       topic,
	}
}

// PublishPaymentReserved publishes a payment reserved event
func (p *PaymentEventPublisher) PublishPaymentReserved(ctx context.Context, intentID, pendingTransferID, debitAccountID, creditAccountID string, amountCents int64, correlationID string) error {
	event := &PaymentEvent{
		EventID:       uuid.New().String(),
		EventType:     EventPaymentReserved,
		Timestamp:     time.Now().UTC(),
		CorrelationID: correlationID,
		TigerBeetle: &TigerBeetleData{
			PendingTransferID: pendingTransferID,
			DebitAccountID:    debitAccountID,
			CreditAccountID:   creditAccountID,
			AmountCents:       amountCents,
		},
	}
	return p.publishEvent(ctx, intentID, event)
}

// PublishPaymentCommitted publishes a payment committed event
func (p *PaymentEventPublisher) PublishPaymentCommitted(ctx context.Context, intentID, pendingTransferID, commitTransferID string, amountCents int64, correlationID string) error {
	event := &PaymentEvent{
		EventID:       uuid.New().String(),
		EventType:     EventPaymentCommitted,
		Timestamp:     time.Now().UTC(),
		CorrelationID: correlationID,
		TigerBeetle: &TigerBeetleData{
			PendingTransferID: pendingTransferID,
			CommitTransferID:  commitTransferID,
			AmountCents:       amountCents,
		},
	}
	return p.publishEvent(ctx, intentID, event)
}

// PublishPaymentVoided publishes a payment voided event
func (p *PaymentEventPublisher) PublishPaymentVoided(ctx context.Context, intentID, pendingTransferID, reason string, correlationID string) error {
	event := &PaymentEvent{
		EventID:       uuid.New().String(),
		EventType:     EventPaymentVoided,
		Timestamp:     time.Now().UTC(),
		CorrelationID: correlationID,
		TigerBeetle: &TigerBeetleData{
			PendingTransferID: pendingTransferID,
		},
		Error: &ErrorData{
			Description: reason,
		},
	}
	return p.publishEvent(ctx, intentID, event)
}

// PublishMojaloopQuoteRequested publishes a quote requested event
func (p *PaymentEventPublisher) PublishMojaloopQuoteRequested(ctx context.Context, intentID, quoteID, payerFSP, payeeFSP, amount, currency, correlationID string) error {
	event := &PaymentEvent{
		EventID:       uuid.New().String(),
		EventType:     EventQuoteRequested,
		Timestamp:     time.Now().UTC(),
		CorrelationID: correlationID,
		Mojaloop: &MojaloopData{
			QuoteID:  quoteID,
			PayerFSP: payerFSP,
			PayeeFSP: payeeFSP,
			Amount:   amount,
			Currency: currency,
		},
	}
	return p.publishEvent(ctx, intentID, event)
}

// PublishMojaloopTransferCommitted publishes a transfer committed event
func (p *PaymentEventPublisher) PublishMojaloopTransferCommitted(ctx context.Context, intentID, transferID, quoteID, payerFSP, payeeFSP, amount, currency, correlationID string) error {
	event := &PaymentEvent{
		EventID:       uuid.New().String(),
		EventType:     EventTransferCommitted,
		Timestamp:     time.Now().UTC(),
		CorrelationID: correlationID,
		Mojaloop: &MojaloopData{
			TransferID:    transferID,
			QuoteID:       quoteID,
			PayerFSP:      payerFSP,
			PayeeFSP:      payeeFSP,
			TransferState: "COMMITTED",
			Amount:        amount,
			Currency:      currency,
		},
	}
	return p.publishEvent(ctx, intentID, event)
}

// PublishMojaloopTransferFailed publishes a transfer failed event
func (p *PaymentEventPublisher) PublishMojaloopTransferFailed(ctx context.Context, intentID, transferID, errorCode, errorDescription, correlationID string) error {
	event := &PaymentEvent{
		EventID:       uuid.New().String(),
		EventType:     EventTransferFailed,
		Timestamp:     time.Now().UTC(),
		CorrelationID: correlationID,
		Mojaloop: &MojaloopData{
			TransferID:    transferID,
			TransferState: "ABORTED",
		},
		Error: &ErrorData{
			Code:        errorCode,
			Description: errorDescription,
		},
	}
	return p.publishEvent(ctx, intentID, event)
}

// PublishSettlementCycleCreated publishes a settlement cycle created event
func (p *PaymentEventPublisher) PublishSettlementCycleCreated(ctx context.Context, cycleID, currency, correlationID string) error {
	event := &PaymentEvent{
		EventID:       uuid.New().String(),
		EventType:     EventSettlementCycleCreated,
		Timestamp:     time.Now().UTC(),
		CorrelationID: correlationID,
		Settlement: &SettlementData{
			CycleID:  cycleID,
			Currency: currency,
		},
	}
	return p.publishEvent(ctx, cycleID, event)
}

// PublishSettlementCycleClosed publishes a settlement cycle closed event
func (p *PaymentEventPublisher) PublishSettlementCycleClosed(ctx context.Context, cycleID, currency string, totalTransfers int, totalAmountCents int64, correlationID string) error {
	event := &PaymentEvent{
		EventID:       uuid.New().String(),
		EventType:     EventSettlementCycleClosed,
		Timestamp:     time.Now().UTC(),
		CorrelationID: correlationID,
		Settlement: &SettlementData{
			CycleID:        cycleID,
			Currency:       currency,
			TotalTransfers: totalTransfers,
			TotalAmount:    totalAmountCents,
		},
	}
	return p.publishEvent(ctx, cycleID, event)
}

// publishEvent publishes an event to Kafka and stores it in the database
func (p *PaymentEventPublisher) publishEvent(ctx context.Context, key string, event *PaymentEvent) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// Store event in database first (outbox pattern)
	if p.paymentRepo != nil {
		dbEvent := &repository.PaymentEvent{
			EventID:         event.EventID,
			EventType:       string(event.EventType),
			PaymentIntentID: key,
			CorrelationID:   event.CorrelationID,
			Payload:         payload,
		}
		if err := p.paymentRepo.CreatePaymentEvent(ctx, dbEvent); err != nil {
			return fmt.Errorf("failed to store event in database: %w", err)
		}
	}

	// Publish to Kafka
	if p.kafkaClient != nil {
		if err := p.kafkaClient.PublishEvent(ctx, p.topic, key, event); err != nil {
			return fmt.Errorf("failed to publish event to Kafka: %w", err)
		}

		// Mark as published
		if p.paymentRepo != nil {
			_ = p.paymentRepo.MarkPaymentEventPublished(ctx, event.EventID)
		}
	}

	return nil
}

// ProcessUnpublishedEvents processes events that failed to publish to Kafka
// This should be called periodically by a background worker
func (p *PaymentEventPublisher) ProcessUnpublishedEvents(ctx context.Context, batchSize int) (int, error) {
	if p.paymentRepo == nil || p.kafkaClient == nil {
		return 0, nil
	}

	events, err := p.paymentRepo.GetUnpublishedPaymentEvents(ctx, batchSize)
	if err != nil {
		return 0, fmt.Errorf("failed to get unpublished events: %w", err)
	}

	published := 0
	for _, event := range events {
		var paymentEvent PaymentEvent
		if err := json.Unmarshal(event.Payload, &paymentEvent); err != nil {
			continue
		}

		if err := p.kafkaClient.PublishEvent(ctx, p.topic, event.PaymentIntentID, &paymentEvent); err != nil {
			continue
		}

		if err := p.paymentRepo.MarkPaymentEventPublished(ctx, event.EventID); err != nil {
			continue
		}

		published++
	}

	return published, nil
}

// Global event publisher instance
var globalEventPublisher *PaymentEventPublisher

// InitPaymentEventPublisher initializes the global event publisher
func InitPaymentEventPublisher(kafkaClient *clients.KafkaClient, paymentRepo *repository.PaymentRepository, topic string) {
	globalEventPublisher = NewPaymentEventPublisher(kafkaClient, paymentRepo, topic)
}

// GetPaymentEventPublisher returns the global event publisher
func GetPaymentEventPublisher() *PaymentEventPublisher {
	return globalEventPublisher
}

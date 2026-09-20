package payments

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/admin-portal/orchestrator/clients"
)

// DaprPaymentStateStore provides Dapr state store integration for payment state
// This enables distributed state management across replicas with automatic consistency
type DaprPaymentStateStore struct {
	daprClient *clients.DaprClient
	storeName  string
}

// NewDaprPaymentStateStore creates a new Dapr payment state store
func NewDaprPaymentStateStore(daprClient *clients.DaprClient, storeName string) *DaprPaymentStateStore {
	if storeName == "" {
		storeName = "payment-state"
	}
	return &DaprPaymentStateStore{
		daprClient: daprClient,
		storeName:  storeName,
	}
}

// PaymentState represents the state of a payment in the Dapr state store
type PaymentState struct {
	IntentID              string     `json:"intentId"`
	BeneficiaryID         string     `json:"beneficiaryId"`
	ProgramID             string     `json:"programId"`
	AmountCents           int64      `json:"amountCents"`
	Currency              string     `json:"currency"`
	FeeCents              int64      `json:"feeCents,omitempty"`
	Status                string     `json:"status"` // pending, reserved, committed, voided, failed
	TigerBeetlePendingID  string     `json:"tigerBeetlePendingId,omitempty"`
	TigerBeetleCommitID   string     `json:"tigerBeetleCommitId,omitempty"`
	MojaloopQuoteID       string     `json:"mojaloopQuoteId,omitempty"`
	MojaloopTransferID    string     `json:"mojaloopTransferId,omitempty"`
	MojaloopTransferState string     `json:"mojaloopTransferState,omitempty"`
	SettlementCycleID     string     `json:"settlementCycleId,omitempty"`
	WorkflowID            string     `json:"workflowId,omitempty"`
	WorkflowRunID         string     `json:"workflowRunId,omitempty"`
	ErrorMessage          string     `json:"errorMessage,omitempty"`
	CreatedAt             time.Time  `json:"createdAt"`
	UpdatedAt             time.Time  `json:"updatedAt"`
	ReservedAt            *time.Time `json:"reservedAt,omitempty"`
	CommittedAt           *time.Time `json:"committedAt,omitempty"`
	VoidedAt              *time.Time `json:"voidedAt,omitempty"`
	TTLSeconds            int        `json:"ttlSeconds,omitempty"`
}

// CallbackCorrelation represents callback correlation state in Dapr
type CallbackCorrelation struct {
	CallbackType    string          `json:"callbackType"` // party, quote, transfer
	CorrelationID   string          `json:"correlationId"`
	Status          string          `json:"status"` // pending, received, processed, expired
	RequestPayload  json.RawMessage `json:"requestPayload,omitempty"`
	ResponsePayload json.RawMessage `json:"responsePayload,omitempty"`
	ErrorPayload    json.RawMessage `json:"errorPayload,omitempty"`
	WorkflowRunID   string          `json:"workflowRunId,omitempty"`
	CreatedAt       time.Time       `json:"createdAt"`
	ExpiresAt       time.Time       `json:"expiresAt"`
	ReceivedAt      *time.Time      `json:"receivedAt,omitempty"`
}

// SavePaymentState saves payment state to Dapr state store
func (s *DaprPaymentStateStore) SavePaymentState(ctx context.Context, state *PaymentState) error {
	if s.daprClient == nil {
		return fmt.Errorf("Dapr client not initialized")
	}

	state.UpdatedAt = time.Now()
	key := fmt.Sprintf("payment:%s", state.IntentID)

	return s.daprClient.SaveState(ctx, s.storeName, key, state)
}

// GetPaymentState retrieves payment state from Dapr state store
func (s *DaprPaymentStateStore) GetPaymentState(ctx context.Context, intentID string) (*PaymentState, error) {
	if s.daprClient == nil {
		return nil, fmt.Errorf("Dapr client not initialized")
	}

	key := fmt.Sprintf("payment:%s", intentID)
	data, err := s.daprClient.GetState(ctx, s.storeName, key)
	if err != nil {
		return nil, fmt.Errorf("failed to get payment state: %w", err)
	}
	if data == nil || len(data.Value) == 0 {
		return nil, nil
	}

	var state PaymentState
	if err := json.Unmarshal(data.Value, &state); err != nil {
		return nil, fmt.Errorf("failed to unmarshal payment state: %w", err)
	}

	return &state, nil
}

// UpdatePaymentStatus updates the status of a payment
func (s *DaprPaymentStateStore) UpdatePaymentStatus(ctx context.Context, intentID, status string) error {
	state, err := s.GetPaymentState(ctx, intentID)
	if err != nil {
		return err
	}
	if state == nil {
		return fmt.Errorf("payment state not found: %s", intentID)
	}

	state.Status = status
	return s.SavePaymentState(ctx, state)
}

// UpdatePaymentReserved updates payment state when funds are reserved
func (s *DaprPaymentStateStore) UpdatePaymentReserved(ctx context.Context, intentID, tigerBeetlePendingID string) error {
	state, err := s.GetPaymentState(ctx, intentID)
	if err != nil {
		return err
	}
	if state == nil {
		return fmt.Errorf("payment state not found: %s", intentID)
	}

	now := time.Now()
	state.Status = "reserved"
	state.TigerBeetlePendingID = tigerBeetlePendingID
	state.ReservedAt = &now
	return s.SavePaymentState(ctx, state)
}

// UpdatePaymentCommitted updates payment state when committed
func (s *DaprPaymentStateStore) UpdatePaymentCommitted(ctx context.Context, intentID, tigerBeetleCommitID, mojaloopTransferID string) error {
	state, err := s.GetPaymentState(ctx, intentID)
	if err != nil {
		return err
	}
	if state == nil {
		return fmt.Errorf("payment state not found: %s", intentID)
	}

	now := time.Now()
	state.Status = "committed"
	state.TigerBeetleCommitID = tigerBeetleCommitID
	state.MojaloopTransferID = mojaloopTransferID
	state.MojaloopTransferState = "COMMITTED"
	state.CommittedAt = &now
	return s.SavePaymentState(ctx, state)
}

// UpdatePaymentVoided updates payment state when voided
func (s *DaprPaymentStateStore) UpdatePaymentVoided(ctx context.Context, intentID, errorMessage string) error {
	state, err := s.GetPaymentState(ctx, intentID)
	if err != nil {
		return err
	}
	if state == nil {
		return fmt.Errorf("payment state not found: %s", intentID)
	}

	now := time.Now()
	state.Status = "voided"
	state.ErrorMessage = errorMessage
	state.VoidedAt = &now
	return s.SavePaymentState(ctx, state)
}

// SaveCallbackCorrelation saves callback correlation state to Dapr
func (s *DaprPaymentStateStore) SaveCallbackCorrelation(ctx context.Context, correlation *CallbackCorrelation) error {
	if s.daprClient == nil {
		return fmt.Errorf("Dapr client not initialized")
	}

	key := fmt.Sprintf("callback:%s:%s", correlation.CallbackType, correlation.CorrelationID)
	return s.daprClient.SaveState(ctx, s.storeName, key, correlation)
}

// GetCallbackCorrelation retrieves callback correlation state from Dapr
func (s *DaprPaymentStateStore) GetCallbackCorrelation(ctx context.Context, callbackType, correlationID string) (*CallbackCorrelation, error) {
	if s.daprClient == nil {
		return nil, fmt.Errorf("Dapr client not initialized")
	}

	key := fmt.Sprintf("callback:%s:%s", callbackType, correlationID)
	data, err := s.daprClient.GetState(ctx, s.storeName, key)
	if err != nil {
		return nil, fmt.Errorf("failed to get callback correlation: %w", err)
	}
	if data == nil || len(data.Value) == 0 {
		return nil, nil
	}

	var correlation CallbackCorrelation
	if err := json.Unmarshal(data.Value, &correlation); err != nil {
		return nil, fmt.Errorf("failed to unmarshal callback correlation: %w", err)
	}

	return &correlation, nil
}

// UpdateCallbackReceived updates callback correlation when response is received
func (s *DaprPaymentStateStore) UpdateCallbackReceived(ctx context.Context, callbackType, correlationID string, responsePayload json.RawMessage) error {
	correlation, err := s.GetCallbackCorrelation(ctx, callbackType, correlationID)
	if err != nil {
		return err
	}
	if correlation == nil {
		// Create new correlation if not exists
		correlation = &CallbackCorrelation{
			CallbackType:  callbackType,
			CorrelationID: correlationID,
			CreatedAt:     time.Now(),
			ExpiresAt:     time.Now().Add(5 * time.Minute),
		}
	}

	now := time.Now()
	correlation.Status = "received"
	correlation.ResponsePayload = responsePayload
	correlation.ReceivedAt = &now
	return s.SaveCallbackCorrelation(ctx, correlation)
}

// UpdateCallbackError updates callback correlation when error is received
func (s *DaprPaymentStateStore) UpdateCallbackError(ctx context.Context, callbackType, correlationID string, errorPayload json.RawMessage) error {
	correlation, err := s.GetCallbackCorrelation(ctx, callbackType, correlationID)
	if err != nil {
		return err
	}
	if correlation == nil {
		correlation = &CallbackCorrelation{
			CallbackType:  callbackType,
			CorrelationID: correlationID,
			CreatedAt:     time.Now(),
			ExpiresAt:     time.Now().Add(5 * time.Minute),
		}
	}

	now := time.Now()
	correlation.Status = "received"
	correlation.ErrorPayload = errorPayload
	correlation.ReceivedAt = &now
	return s.SaveCallbackCorrelation(ctx, correlation)
}

// RegisterPendingCallback registers a pending callback for correlation
func (s *DaprPaymentStateStore) RegisterPendingCallback(ctx context.Context, callbackType, correlationID, workflowRunID string, ttl time.Duration) error {
	correlation := &CallbackCorrelation{
		CallbackType:  callbackType,
		CorrelationID: correlationID,
		Status:        "pending",
		WorkflowRunID: workflowRunID,
		CreatedAt:     time.Now(),
		ExpiresAt:     time.Now().Add(ttl),
	}
	return s.SaveCallbackCorrelation(ctx, correlation)
}

// WaitForCallback polls Dapr state store for callback response
func (s *DaprPaymentStateStore) WaitForCallback(ctx context.Context, callbackType, correlationID string, timeout time.Duration) (*CallbackCorrelation, error) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	timeoutCh := time.After(timeout)
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timeoutCh:
			return nil, fmt.Errorf("timeout waiting for %s callback: %s", callbackType, correlationID)
		case <-ticker.C:
			correlation, err := s.GetCallbackCorrelation(ctx, callbackType, correlationID)
			if err != nil {
				continue
			}
			if correlation != nil && correlation.Status == "received" {
				return correlation, nil
			}
		}
	}
}

// PublishPaymentEvent publishes a payment event via Dapr pub/sub
func (s *DaprPaymentStateStore) PublishPaymentEvent(ctx context.Context, pubsubName, topic string, event *PaymentEvent) error {
	if s.daprClient == nil {
		return fmt.Errorf("Dapr client not initialized")
	}

	return s.daprClient.PublishEvent(ctx, pubsubName, topic, event)
}

// Global Dapr payment state store instance
var globalDaprPaymentStateStore *DaprPaymentStateStore

// InitDaprPaymentStateStore initializes the global Dapr payment state store
func InitDaprPaymentStateStore(daprClient *clients.DaprClient, storeName string) {
	globalDaprPaymentStateStore = NewDaprPaymentStateStore(daprClient, storeName)
}

// GetDaprPaymentStateStore returns the global Dapr payment state store
func GetDaprPaymentStateStore() *DaprPaymentStateStore {
	return globalDaprPaymentStateStore
}

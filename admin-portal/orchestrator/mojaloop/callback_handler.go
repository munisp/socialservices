package mojaloop

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

// CallbackHandler handles async callbacks from Mojaloop
type CallbackHandler struct {
	fspiop           *FSPIOPClient
	quoteCallbacks   chan *QuoteCallback
	transferCallbacks chan *TransferCallback
	partyCallbacks   chan *PartyCallback
	errorCallbacks   chan *ErrorCallback
	handlers         map[string]CallbackFunc
	mu               sync.RWMutex
}

// CallbackFunc is a function that handles a specific callback type
type CallbackFunc func(ctx context.Context, data interface{}) error

// QuoteCallback represents a quote callback from Mojaloop
type QuoteCallback struct {
	QuoteId       string         `json:"quoteId"`
	Response      *QuoteResponse `json:"response,omitempty"`
	Error         *ErrorInformation `json:"error,omitempty"`
	ReceivedAt    time.Time      `json:"-"`
}

// TransferCallback represents a transfer callback from Mojaloop
type TransferCallback struct {
	TransferId    string            `json:"transferId"`
	Response      *TransferResponse `json:"response,omitempty"`
	Error         *ErrorInformation `json:"error,omitempty"`
	ReceivedAt    time.Time         `json:"-"`
}

// PartyCallback represents a party lookup callback from Mojaloop
type PartyCallback struct {
	PartyIdType   string            `json:"partyIdType"`
	PartyIdentifier string          `json:"partyIdentifier"`
	Party         *Party            `json:"party,omitempty"`
	Error         *ErrorInformation `json:"error,omitempty"`
	ReceivedAt    time.Time         `json:"-"`
}

// ErrorCallback represents an error callback from Mojaloop
type ErrorCallback struct {
	ResourceType  string            `json:"resourceType"`
	ResourceId    string            `json:"resourceId"`
	Error         *ErrorInformation `json:"error"`
	ReceivedAt    time.Time         `json:"-"`
}

// NewCallbackHandler creates a new callback handler
func NewCallbackHandler(fspiop *FSPIOPClient) *CallbackHandler {
	return &CallbackHandler{
		fspiop:            fspiop,
		quoteCallbacks:    make(chan *QuoteCallback, 1000),
		transferCallbacks: make(chan *TransferCallback, 1000),
		partyCallbacks:    make(chan *PartyCallback, 1000),
		errorCallbacks:    make(chan *ErrorCallback, 1000),
		handlers:          make(map[string]CallbackFunc),
	}
}

// RegisterHandler registers a callback handler for a specific event type
func (h *CallbackHandler) RegisterHandler(eventType string, handler CallbackFunc) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.handlers[eventType] = handler
}

// ServeHTTP implements http.Handler for receiving Mojaloop callbacks
func (h *CallbackHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	method := r.Method

	switch {
	case strings.HasPrefix(path, "/parties/") && method == "PUT":
		h.handlePartyCallback(w, r)
	case strings.HasPrefix(path, "/parties/") && strings.Contains(path, "/error") && method == "PUT":
		h.handlePartyErrorCallback(w, r)
	case strings.HasPrefix(path, "/quotes/") && method == "PUT" && !strings.Contains(path, "/error"):
		h.handleQuoteCallback(w, r)
	case strings.HasPrefix(path, "/quotes/") && strings.Contains(path, "/error") && method == "PUT":
		h.handleQuoteErrorCallback(w, r)
	case strings.HasPrefix(path, "/transfers/") && method == "PUT" && !strings.Contains(path, "/error"):
		h.handleTransferCallback(w, r)
	case strings.HasPrefix(path, "/transfers/") && strings.Contains(path, "/error") && method == "PUT":
		h.handleTransferErrorCallback(w, r)
	case strings.HasPrefix(path, "/bulkQuotes/") && method == "PUT":
		h.handleBulkQuoteCallback(w, r)
	case strings.HasPrefix(path, "/bulkTransfers/") && method == "PUT":
		h.handleBulkTransferCallback(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (h *CallbackHandler) handlePartyCallback(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/parties/"), "/")
	if len(parts) < 2 {
		http.Error(w, "Invalid party path", http.StatusBadRequest)
		return
	}
	partyIdType := parts[0]
	partyIdentifier := parts[1]

	var party Party
	if err := json.NewDecoder(r.Body).Decode(&party); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	callback := &PartyCallback{
		PartyIdType:     partyIdType,
		PartyIdentifier: partyIdentifier,
		Party:           &party,
		ReceivedAt:      time.Now(),
	}

	select {
	case h.partyCallbacks <- callback:
		log.Printf("Party callback received: %s/%s -> FSP: %s", partyIdType, partyIdentifier, party.FspId)
	default:
		log.Printf("Party callback channel full, dropping: %s/%s", partyIdType, partyIdentifier)
	}

	h.invokeHandler("party.resolved", callback)
	w.WriteHeader(http.StatusOK)
}

func (h *CallbackHandler) handlePartyErrorCallback(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/parties/"), "/")
	if len(parts) < 2 {
		http.Error(w, "Invalid party path", http.StatusBadRequest)
		return
	}
	partyIdType := parts[0]
	partyIdentifier := parts[1]

	var errorInfo ErrorInformation
	if err := json.NewDecoder(r.Body).Decode(&errorInfo); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	callback := &PartyCallback{
		PartyIdType:     partyIdType,
		PartyIdentifier: partyIdentifier,
		Error:           &errorInfo,
		ReceivedAt:      time.Now(),
	}

	select {
	case h.partyCallbacks <- callback:
		log.Printf("Party error callback received: %s/%s -> Error: %s", partyIdType, partyIdentifier, errorInfo.ErrorCode)
	default:
		log.Printf("Party callback channel full, dropping error: %s/%s", partyIdType, partyIdentifier)
	}

	h.invokeHandler("party.error", callback)
	w.WriteHeader(http.StatusOK)
}

func (h *CallbackHandler) handleQuoteCallback(w http.ResponseWriter, r *http.Request) {
	quoteId := strings.TrimPrefix(r.URL.Path, "/quotes/")
	quoteId = strings.TrimSuffix(quoteId, "/")

	var quoteResp QuoteResponse
	if err := json.NewDecoder(r.Body).Decode(&quoteResp); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	callback := &QuoteCallback{
		QuoteId:    quoteId,
		Response:   &quoteResp,
		ReceivedAt: time.Now(),
	}

	if h.fspiop != nil {
		h.fspiop.UpdateQuoteStatus(quoteId, "COMPLETED")
	}

	select {
	case h.quoteCallbacks <- callback:
		log.Printf("Quote callback received: %s -> Amount: %s %s", quoteId, quoteResp.TransferAmount.Amount, quoteResp.TransferAmount.Currency)
	default:
		log.Printf("Quote callback channel full, dropping: %s", quoteId)
	}

	h.invokeHandler("quote.resolved", callback)
	w.WriteHeader(http.StatusOK)
}

func (h *CallbackHandler) handleQuoteErrorCallback(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/quotes/")
	quoteId := strings.Split(path, "/")[0]

	var errorInfo ErrorInformation
	if err := json.NewDecoder(r.Body).Decode(&errorInfo); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	callback := &QuoteCallback{
		QuoteId:    quoteId,
		Error:      &errorInfo,
		ReceivedAt: time.Now(),
	}

	if h.fspiop != nil {
		h.fspiop.UpdateQuoteStatus(quoteId, "FAILED")
	}

	select {
	case h.quoteCallbacks <- callback:
		log.Printf("Quote error callback received: %s -> Error: %s", quoteId, errorInfo.ErrorCode)
	default:
		log.Printf("Quote callback channel full, dropping error: %s", quoteId)
	}

	h.invokeHandler("quote.error", callback)
	w.WriteHeader(http.StatusOK)
}

func (h *CallbackHandler) handleTransferCallback(w http.ResponseWriter, r *http.Request) {
	transferId := strings.TrimPrefix(r.URL.Path, "/transfers/")
	transferId = strings.TrimSuffix(transferId, "/")

	var transferResp TransferResponse
	if err := json.NewDecoder(r.Body).Decode(&transferResp); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	callback := &TransferCallback{
		TransferId: transferId,
		Response:   &transferResp,
		ReceivedAt: time.Now(),
	}

	if h.fspiop != nil {
		h.fspiop.UpdateTransferStatus(transferId, transferResp.TransferState)
	}

	select {
	case h.transferCallbacks <- callback:
		log.Printf("Transfer callback received: %s -> State: %s", transferId, transferResp.TransferState)
	default:
		log.Printf("Transfer callback channel full, dropping: %s", transferId)
	}

	eventType := "transfer.completed"
	if transferResp.TransferState == TransferStateAborted {
		eventType = "transfer.aborted"
	}
	h.invokeHandler(eventType, callback)
	w.WriteHeader(http.StatusOK)
}

func (h *CallbackHandler) handleTransferErrorCallback(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/transfers/")
	transferId := strings.Split(path, "/")[0]

	var errorInfo ErrorInformation
	if err := json.NewDecoder(r.Body).Decode(&errorInfo); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	callback := &TransferCallback{
		TransferId: transferId,
		Error:      &errorInfo,
		ReceivedAt: time.Now(),
	}

	if h.fspiop != nil {
		h.fspiop.UpdateTransferStatus(transferId, TransferStateAborted)
	}

	select {
	case h.transferCallbacks <- callback:
		log.Printf("Transfer error callback received: %s -> Error: %s", transferId, errorInfo.ErrorCode)
	default:
		log.Printf("Transfer callback channel full, dropping error: %s", transferId)
	}

	h.invokeHandler("transfer.error", callback)
	w.WriteHeader(http.StatusOK)
}

func (h *CallbackHandler) handleBulkQuoteCallback(w http.ResponseWriter, r *http.Request) {
	bulkQuoteId := strings.TrimPrefix(r.URL.Path, "/bulkQuotes/")
	bulkQuoteId = strings.TrimSuffix(bulkQuoteId, "/")

	var bulkQuoteResp BulkQuoteResponse
	if err := json.NewDecoder(r.Body).Decode(&bulkQuoteResp); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	log.Printf("Bulk quote callback received: %s -> %d results", bulkQuoteId, len(bulkQuoteResp.IndividualQuoteResults))
	h.invokeHandler("bulkQuote.resolved", &bulkQuoteResp)
	w.WriteHeader(http.StatusOK)
}

func (h *CallbackHandler) handleBulkTransferCallback(w http.ResponseWriter, r *http.Request) {
	bulkTransferId := strings.TrimPrefix(r.URL.Path, "/bulkTransfers/")
	bulkTransferId = strings.TrimSuffix(bulkTransferId, "/")

	var bulkTransferResp map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&bulkTransferResp); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	log.Printf("Bulk transfer callback received: %s", bulkTransferId)
	h.invokeHandler("bulkTransfer.resolved", bulkTransferResp)
	w.WriteHeader(http.StatusOK)
}

func (h *CallbackHandler) invokeHandler(eventType string, data interface{}) {
	h.mu.RLock()
	handler, ok := h.handlers[eventType]
	h.mu.RUnlock()

	if ok {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := handler(ctx, data); err != nil {
				log.Printf("Callback handler error for %s: %v", eventType, err)
			}
		}()
	}
}

// WaitForQuoteCallback waits for a quote callback with timeout
func (h *CallbackHandler) WaitForQuoteCallback(ctx context.Context, quoteId string) (*QuoteCallback, error) {
	timeout := time.After(5 * time.Minute)
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timeout:
			return nil, fmt.Errorf("timeout waiting for quote callback: %s", quoteId)
		case callback := <-h.quoteCallbacks:
			if callback.QuoteId == quoteId {
				return callback, nil
			}
			h.quoteCallbacks <- callback
		}
	}
}

// WaitForTransferCallback waits for a transfer callback with timeout
func (h *CallbackHandler) WaitForTransferCallback(ctx context.Context, transferId string) (*TransferCallback, error) {
	timeout := time.After(5 * time.Minute)
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timeout:
			return nil, fmt.Errorf("timeout waiting for transfer callback: %s", transferId)
		case callback := <-h.transferCallbacks:
			if callback.TransferId == transferId {
				return callback, nil
			}
			h.transferCallbacks <- callback
		}
	}
}

// WaitForPartyCallback waits for a party callback with timeout
func (h *CallbackHandler) WaitForPartyCallback(ctx context.Context, partyIdType, partyIdentifier string) (*PartyCallback, error) {
	timeout := time.After(30 * time.Second)
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timeout:
			return nil, fmt.Errorf("timeout waiting for party callback: %s/%s", partyIdType, partyIdentifier)
		case callback := <-h.partyCallbacks:
			if callback.PartyIdType == partyIdType && callback.PartyIdentifier == partyIdentifier {
				return callback, nil
			}
			h.partyCallbacks <- callback
		}
	}
}

// QuoteCallbackChan returns the quote callback channel for external consumption
func (h *CallbackHandler) QuoteCallbackChan() <-chan *QuoteCallback {
	return h.quoteCallbacks
}

// TransferCallbackChan returns the transfer callback channel for external consumption
func (h *CallbackHandler) TransferCallbackChan() <-chan *TransferCallback {
	return h.transferCallbacks
}

// PartyCallbackChan returns the party callback channel for external consumption
func (h *CallbackHandler) PartyCallbackChan() <-chan *PartyCallback {
	return h.partyCallbacks
}

// Global callback handler
var globalCallbackHandler *CallbackHandler
var callbackHandlerMu sync.RWMutex

// InitCallbackHandler initializes the global callback handler
func InitCallbackHandler(fspiop *FSPIOPClient) *CallbackHandler {
	callbackHandlerMu.Lock()
	defer callbackHandlerMu.Unlock()
	globalCallbackHandler = NewCallbackHandler(fspiop)
	return globalCallbackHandler
}

// GetCallbackHandler returns the global callback handler
func GetCallbackHandler() *CallbackHandler {
	callbackHandlerMu.RLock()
	defer callbackHandlerMu.RUnlock()
	return globalCallbackHandler
}

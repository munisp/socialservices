package mojaloop

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/admin-portal/orchestrator/repository"
)

// DurableCallbackHandler handles async callbacks from Mojaloop with PostgreSQL-backed correlation
// This handler survives restarts and works across multiple replicas
type DurableCallbackHandler struct {
	fspiop       *FSPIOPClient
	paymentRepo  *repository.PaymentRepository
	handlers     map[string]CallbackFunc
	mu           sync.RWMutex
	pollInterval time.Duration
}

// NewDurableCallbackHandler creates a new durable callback handler
func NewDurableCallbackHandler(fspiop *FSPIOPClient, paymentRepo *repository.PaymentRepository) *DurableCallbackHandler {
	return &DurableCallbackHandler{
		fspiop:       fspiop,
		paymentRepo:  paymentRepo,
		handlers:     make(map[string]CallbackFunc),
		pollInterval: 500 * time.Millisecond,
	}
}

// RegisterHandler registers a callback handler for a specific event type
func (h *DurableCallbackHandler) RegisterHandler(eventType string, handler CallbackFunc) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.handlers[eventType] = handler
}

// RegisterPendingCallback registers a pending callback in the database
func (h *DurableCallbackHandler) RegisterPendingCallback(ctx context.Context, callbackType, correlationID, workflowRunID string, ttl time.Duration) error {
	if h.paymentRepo == nil {
		return fmt.Errorf("payment repository not initialized")
	}

	expiresAt := time.Now().Add(ttl)
	callback := &repository.MojaloopCallback{
		CallbackType:  callbackType,
		CorrelationID: correlationID,
		Status:        "pending",
		WorkflowRunID: workflowRunID,
		ExpiresAt:     &expiresAt,
	}

	return h.paymentRepo.CreateMojaloopCallback(ctx, callback)
}

// ServeHTTP implements http.Handler for receiving Mojaloop callbacks
func (h *DurableCallbackHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	method := r.Method

	// Verify JWS signature if present (security requirement)
	if !h.verifyJWSSignature(r) {
		log.Printf("JWS signature verification failed for %s %s", method, path)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

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

// verifyJWSSignature verifies the JWS signature on the request
// In production, this validates against the Mojaloop hub's public key
func (h *DurableCallbackHandler) verifyJWSSignature(r *http.Request) bool {
	// Check for FSPIOP-Signature header
	signature := r.Header.Get("FSPIOP-Signature")
	if signature == "" {
		if os.Getenv("MOJALOOP_ENV") != "production" && os.Getenv("MOJALOOP_ALLOW_UNSIGNED") == "true" {
			log.Printf("Warning: unsigned Mojaloop callback accepted by explicit non-production override")
			return true
		}
		log.Printf("Error: No FSPIOP-Signature header present in production")
		return false
	}

	// Get source DFSP from header
	sourceDFSP := r.Header.Get("FSPIOP-Source")
	if sourceDFSP == "" {
		log.Printf("Error: No FSPIOP-Source header present")
		return false
	}

	// Parse the JWS signature (format: base64url(header).base64url(payload).base64url(signature))
	parts := strings.Split(signature, ".")
	if len(parts) != 3 {
		log.Printf("Error: Invalid JWS signature format - expected 3 parts, got %d", len(parts))
		return false
	}

	// Decode the header to get the algorithm and key ID
	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		log.Printf("Error: Failed to decode JWS header: %v", err)
		return false
	}

	var jwsHeader struct {
		Alg string `json:"alg"`
		Kid string `json:"kid"`
	}
	if err := json.Unmarshal(headerBytes, &jwsHeader); err != nil {
		log.Printf("Error: Failed to parse JWS header: %v", err)
		return false
	}

	// Verify algorithm is RS256 (required by Mojaloop)
	if jwsHeader.Alg != "RS256" {
		log.Printf("Error: Unsupported JWS algorithm: %s (expected RS256)", jwsHeader.Alg)
		return false
	}

	// Get the pinned public key for the source DFSP from the deployment secret store.
	publicKeyPEM := os.Getenv(fmt.Sprintf("DFSP_PUBLIC_KEY_%s", strings.ToUpper(sourceDFSP)))
	if publicKeyPEM == "" {
		// Fallback to default hub key
		publicKeyPEM = os.Getenv("MOJALOOP_HUB_PUBLIC_KEY")
	}

	if publicKeyPEM == "" {
		// In non-production, log warning and allow
		if os.Getenv("MOJALOOP_ENV") != "production" {
			log.Printf("Warning: No public key configured for DFSP %s (allowed in non-production)", sourceDFSP)
			return true
		}
		log.Printf("Error: No public key configured for DFSP %s", sourceDFSP)
		return false
	}

	block, _ := pem.Decode([]byte(publicKeyPEM))
	if block == nil {
		log.Printf("Error: Invalid PEM public key for DFSP %s", sourceDFSP)
		return false
	}
	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		if pkcs1, pkcs1Err := x509.ParsePKCS1PublicKey(block.Bytes); pkcs1Err == nil {
			parsed = pkcs1
		} else if certificate, certificateErr := x509.ParseCertificate(block.Bytes); certificateErr == nil {
			parsed = certificate.PublicKey
		} else {
			log.Printf("Error: Failed to parse DFSP public key: %v", err)
			return false
		}
	}
	publicKey, ok := parsed.(*rsa.PublicKey)
	if !ok {
		log.Printf("Error: DFSP key is not RSA")
		return false
	}
	signatureBytes, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		log.Printf("Error: Failed to decode JWS signature: %v", err)
		return false
	}
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if err := rsa.VerifyPKCS1v15(publicKey, crypto.SHA256, digest[:], signatureBytes); err != nil {
		log.Printf("Error: Invalid JWS signature for DFSP %s: %v", sourceDFSP, err)
		return false
	}
	log.Printf("JWS signature verified for DFSP %s with algorithm %s", sourceDFSP, jwsHeader.Alg)
	return true
}

func (h *DurableCallbackHandler) handlePartyCallback(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/parties/"), "/")
	if len(parts) < 2 {
		http.Error(w, "Invalid party path", http.StatusBadRequest)
		return
	}
	partyIdType := parts[0]
	partyIdentifier := parts[1]
	correlationID := fmt.Sprintf("%s:%s", partyIdType, partyIdentifier)

	var party Party
	if err := json.NewDecoder(r.Body).Decode(&party); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Store callback in database
	if h.paymentRepo != nil {
		responsePayload, _ := json.Marshal(&party)
		if err := h.paymentRepo.UpdateMojaloopCallbackReceived(r.Context(), "party", correlationID, responsePayload); err != nil {
			log.Printf("Failed to store party callback: %v", err)
		}
	}

	log.Printf("Party callback received: %s/%s -> FSP: %s", partyIdType, partyIdentifier, party.FspId)
	h.invokeHandler("party.resolved", &PartyCallback{
		PartyIdType:     partyIdType,
		PartyIdentifier: partyIdentifier,
		Party:           &party,
		ReceivedAt:      time.Now(),
	})
	w.WriteHeader(http.StatusOK)
}

func (h *DurableCallbackHandler) handlePartyErrorCallback(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/parties/"), "/")
	if len(parts) < 2 {
		http.Error(w, "Invalid party path", http.StatusBadRequest)
		return
	}
	partyIdType := parts[0]
	partyIdentifier := strings.TrimSuffix(parts[1], "/error")
	correlationID := fmt.Sprintf("%s:%s", partyIdType, partyIdentifier)

	var errorInfo ErrorInformation
	if err := json.NewDecoder(r.Body).Decode(&errorInfo); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Store error callback in database
	if h.paymentRepo != nil {
		errorPayload, _ := json.Marshal(&errorInfo)
		if err := h.paymentRepo.UpdateMojaloopCallbackError(r.Context(), "party", correlationID, errorPayload); err != nil {
			log.Printf("Failed to store party error callback: %v", err)
		}
	}

	log.Printf("Party error callback received: %s/%s -> Error: %s", partyIdType, partyIdentifier, errorInfo.ErrorCode)
	h.invokeHandler("party.error", &PartyCallback{
		PartyIdType:     partyIdType,
		PartyIdentifier: partyIdentifier,
		Error:           &errorInfo,
		ReceivedAt:      time.Now(),
	})
	w.WriteHeader(http.StatusOK)
}

func (h *DurableCallbackHandler) handleQuoteCallback(w http.ResponseWriter, r *http.Request) {
	quoteId := strings.TrimPrefix(r.URL.Path, "/quotes/")
	quoteId = strings.TrimSuffix(quoteId, "/")

	var quoteResp QuoteResponse
	if err := json.NewDecoder(r.Body).Decode(&quoteResp); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Store callback in database
	if h.paymentRepo != nil {
		responsePayload, _ := json.Marshal(&quoteResp)
		if err := h.paymentRepo.UpdateMojaloopCallbackReceived(r.Context(), "quote", quoteId, responsePayload); err != nil {
			log.Printf("Failed to store quote callback: %v", err)
		}
	}

	if h.fspiop != nil {
		h.fspiop.UpdateQuoteStatus(quoteId, "COMPLETED")
	}

	log.Printf("Quote callback received: %s -> Amount: %s %s", quoteId, quoteResp.TransferAmount.Amount, quoteResp.TransferAmount.Currency)
	h.invokeHandler("quote.resolved", &QuoteCallback{
		QuoteId:    quoteId,
		Response:   &quoteResp,
		ReceivedAt: time.Now(),
	})
	w.WriteHeader(http.StatusOK)
}

func (h *DurableCallbackHandler) handleQuoteErrorCallback(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/quotes/")
	quoteId := strings.Split(path, "/")[0]

	var errorInfo ErrorInformation
	if err := json.NewDecoder(r.Body).Decode(&errorInfo); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Store error callback in database
	if h.paymentRepo != nil {
		errorPayload, _ := json.Marshal(&errorInfo)
		if err := h.paymentRepo.UpdateMojaloopCallbackError(r.Context(), "quote", quoteId, errorPayload); err != nil {
			log.Printf("Failed to store quote error callback: %v", err)
		}
	}

	if h.fspiop != nil {
		h.fspiop.UpdateQuoteStatus(quoteId, "FAILED")
	}

	log.Printf("Quote error callback received: %s -> Error: %s", quoteId, errorInfo.ErrorCode)
	h.invokeHandler("quote.error", &QuoteCallback{
		QuoteId:    quoteId,
		Error:      &errorInfo,
		ReceivedAt: time.Now(),
	})
	w.WriteHeader(http.StatusOK)
}

func (h *DurableCallbackHandler) handleTransferCallback(w http.ResponseWriter, r *http.Request) {
	transferId := strings.TrimPrefix(r.URL.Path, "/transfers/")
	transferId = strings.TrimSuffix(transferId, "/")

	var transferResp TransferResponse
	if err := json.NewDecoder(r.Body).Decode(&transferResp); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Store callback in database
	if h.paymentRepo != nil {
		responsePayload, _ := json.Marshal(&transferResp)
		if err := h.paymentRepo.UpdateMojaloopCallbackReceived(r.Context(), "transfer", transferId, responsePayload); err != nil {
			log.Printf("Failed to store transfer callback: %v", err)
		}
	}

	if h.fspiop != nil {
		h.fspiop.UpdateTransferStatus(transferId, transferResp.TransferState)
	}

	log.Printf("Transfer callback received: %s -> State: %s", transferId, transferResp.TransferState)
	eventType := "transfer.completed"
	if transferResp.TransferState == TransferStateAborted {
		eventType = "transfer.aborted"
	}
	h.invokeHandler(eventType, &TransferCallback{
		TransferId: transferId,
		Response:   &transferResp,
		ReceivedAt: time.Now(),
	})
	w.WriteHeader(http.StatusOK)
}

func (h *DurableCallbackHandler) handleTransferErrorCallback(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/transfers/")
	transferId := strings.Split(path, "/")[0]

	var errorInfo ErrorInformation
	if err := json.NewDecoder(r.Body).Decode(&errorInfo); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Store error callback in database
	if h.paymentRepo != nil {
		errorPayload, _ := json.Marshal(&errorInfo)
		if err := h.paymentRepo.UpdateMojaloopCallbackError(r.Context(), "transfer", transferId, errorPayload); err != nil {
			log.Printf("Failed to store transfer error callback: %v", err)
		}
	}

	if h.fspiop != nil {
		h.fspiop.UpdateTransferStatus(transferId, TransferStateAborted)
	}

	log.Printf("Transfer error callback received: %s -> Error: %s", transferId, errorInfo.ErrorCode)
	h.invokeHandler("transfer.error", &TransferCallback{
		TransferId: transferId,
		Error:      &errorInfo,
		ReceivedAt: time.Now(),
	})
	w.WriteHeader(http.StatusOK)
}

func (h *DurableCallbackHandler) handleBulkQuoteCallback(w http.ResponseWriter, r *http.Request) {
	bulkQuoteId := strings.TrimPrefix(r.URL.Path, "/bulkQuotes/")
	bulkQuoteId = strings.TrimSuffix(bulkQuoteId, "/")

	var bulkQuoteResp BulkQuoteResponse
	if err := json.NewDecoder(r.Body).Decode(&bulkQuoteResp); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Store callback in database
	if h.paymentRepo != nil {
		responsePayload, _ := json.Marshal(&bulkQuoteResp)
		if err := h.paymentRepo.UpdateMojaloopCallbackReceived(r.Context(), "bulk_quote", bulkQuoteId, responsePayload); err != nil {
			log.Printf("Failed to store bulk quote callback: %v", err)
		}
	}

	log.Printf("Bulk quote callback received: %s -> %d results", bulkQuoteId, len(bulkQuoteResp.IndividualQuoteResults))
	h.invokeHandler("bulkQuote.resolved", &bulkQuoteResp)
	w.WriteHeader(http.StatusOK)
}

func (h *DurableCallbackHandler) handleBulkTransferCallback(w http.ResponseWriter, r *http.Request) {
	bulkTransferId := strings.TrimPrefix(r.URL.Path, "/bulkTransfers/")
	bulkTransferId = strings.TrimSuffix(bulkTransferId, "/")

	var bulkTransferResp map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&bulkTransferResp); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Store callback in database
	if h.paymentRepo != nil {
		responsePayload, _ := json.Marshal(bulkTransferResp)
		if err := h.paymentRepo.UpdateMojaloopCallbackReceived(r.Context(), "bulk_transfer", bulkTransferId, responsePayload); err != nil {
			log.Printf("Failed to store bulk transfer callback: %v", err)
		}
	}

	log.Printf("Bulk transfer callback received: %s", bulkTransferId)
	h.invokeHandler("bulkTransfer.resolved", bulkTransferResp)
	w.WriteHeader(http.StatusOK)
}

func (h *DurableCallbackHandler) invokeHandler(eventType string, data interface{}) {
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

// WaitForQuoteCallback waits for a quote callback by polling the database
func (h *DurableCallbackHandler) WaitForQuoteCallback(ctx context.Context, quoteId string) (*QuoteCallback, error) {
	if h.paymentRepo == nil {
		return nil, fmt.Errorf("payment repository not initialized")
	}

	// Register pending callback if not already registered
	_ = h.RegisterPendingCallback(ctx, "quote", quoteId, "", 5*time.Minute)

	ticker := time.NewTicker(h.pollInterval)
	defer ticker.Stop()

	timeout := time.After(5 * time.Minute)
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timeout:
			return nil, fmt.Errorf("timeout waiting for quote callback: %s", quoteId)
		case <-ticker.C:
			callback, err := h.paymentRepo.GetMojaloopCallback(ctx, "quote", quoteId)
			if err != nil {
				continue
			}
			if callback != nil && callback.Status == "received" {
				result := &QuoteCallback{
					QuoteId:    quoteId,
					ReceivedAt: time.Now(),
				}
				if callback.ResponsePayload != nil {
					var quoteResp QuoteResponse
					if err := json.Unmarshal(callback.ResponsePayload, &quoteResp); err == nil {
						result.Response = &quoteResp
					}
				}
				if callback.ErrorPayload != nil {
					var errorInfo ErrorInformation
					if err := json.Unmarshal(callback.ErrorPayload, &errorInfo); err == nil {
						result.Error = &errorInfo
					}
				}
				return result, nil
			}
		}
	}
}

// WaitForTransferCallback waits for a transfer callback by polling the database
func (h *DurableCallbackHandler) WaitForTransferCallback(ctx context.Context, transferId string) (*TransferCallback, error) {
	if h.paymentRepo == nil {
		return nil, fmt.Errorf("payment repository not initialized")
	}

	// Register pending callback if not already registered
	_ = h.RegisterPendingCallback(ctx, "transfer", transferId, "", 5*time.Minute)

	ticker := time.NewTicker(h.pollInterval)
	defer ticker.Stop()

	timeout := time.After(5 * time.Minute)
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timeout:
			return nil, fmt.Errorf("timeout waiting for transfer callback: %s", transferId)
		case <-ticker.C:
			callback, err := h.paymentRepo.GetMojaloopCallback(ctx, "transfer", transferId)
			if err != nil {
				continue
			}
			if callback != nil && callback.Status == "received" {
				result := &TransferCallback{
					TransferId: transferId,
					ReceivedAt: time.Now(),
				}
				if callback.ResponsePayload != nil {
					var transferResp TransferResponse
					if err := json.Unmarshal(callback.ResponsePayload, &transferResp); err == nil {
						result.Response = &transferResp
					}
				}
				if callback.ErrorPayload != nil {
					var errorInfo ErrorInformation
					if err := json.Unmarshal(callback.ErrorPayload, &errorInfo); err == nil {
						result.Error = &errorInfo
					}
				}
				return result, nil
			}
		}
	}
}

// WaitForPartyCallback waits for a party callback by polling the database
func (h *DurableCallbackHandler) WaitForPartyCallback(ctx context.Context, partyIdType, partyIdentifier string) (*PartyCallback, error) {
	if h.paymentRepo == nil {
		return nil, fmt.Errorf("payment repository not initialized")
	}

	correlationID := fmt.Sprintf("%s:%s", partyIdType, partyIdentifier)

	// Register pending callback if not already registered
	_ = h.RegisterPendingCallback(ctx, "party", correlationID, "", 30*time.Second)

	ticker := time.NewTicker(h.pollInterval)
	defer ticker.Stop()

	timeout := time.After(30 * time.Second)
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timeout:
			return nil, fmt.Errorf("timeout waiting for party callback: %s/%s", partyIdType, partyIdentifier)
		case <-ticker.C:
			callback, err := h.paymentRepo.GetMojaloopCallback(ctx, "party", correlationID)
			if err != nil {
				continue
			}
			if callback != nil && callback.Status == "received" {
				result := &PartyCallback{
					PartyIdType:     partyIdType,
					PartyIdentifier: partyIdentifier,
					ReceivedAt:      time.Now(),
				}
				if callback.ResponsePayload != nil {
					var party Party
					if err := json.Unmarshal(callback.ResponsePayload, &party); err == nil {
						result.Party = &party
					}
				}
				if callback.ErrorPayload != nil {
					var errorInfo ErrorInformation
					if err := json.Unmarshal(callback.ErrorPayload, &errorInfo); err == nil {
						result.Error = &errorInfo
					}
				}
				return result, nil
			}
		}
	}
}

// Global durable callback handler
var globalDurableCallbackHandler *DurableCallbackHandler
var durableCallbackHandlerMu sync.RWMutex

// InitDurableCallbackHandler initializes the global durable callback handler
func InitDurableCallbackHandler(fspiop *FSPIOPClient, paymentRepo *repository.PaymentRepository) *DurableCallbackHandler {
	durableCallbackHandlerMu.Lock()
	defer durableCallbackHandlerMu.Unlock()
	globalDurableCallbackHandler = NewDurableCallbackHandler(fspiop, paymentRepo)
	return globalDurableCallbackHandler
}

// GetDurableCallbackHandler returns the global durable callback handler
func GetDurableCallbackHandler() *DurableCallbackHandler {
	durableCallbackHandlerMu.RLock()
	defer durableCallbackHandlerMu.RUnlock()
	return globalDurableCallbackHandler
}

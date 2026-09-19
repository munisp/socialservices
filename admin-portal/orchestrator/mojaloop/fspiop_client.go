package mojaloop

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// FSPIOPClient implements the Mojaloop FSPIOP API for interoperability
type FSPIOPClient struct {
	baseURL       string
	dfspID        string
	httpClient    *http.Client
	callbackURL   string
	pendingQuotes sync.Map
	pendingTxs    sync.Map
	mu            sync.RWMutex
}

// FSPIOPConfig contains configuration for the FSPIOP client
type FSPIOPConfig struct {
	BaseURL           string
	DFSPID            string
	CallbackURL       string
	TimeoutSeconds    int
	TLSCertPath       string
	TLSKeyPath        string
	JWSSigningKeyPath string
}

// NewFSPIOPClient creates a new FSPIOP client
func NewFSPIOPClient(config FSPIOPConfig) (*FSPIOPClient, error) {
	if config.BaseURL == "" {
		return nil, fmt.Errorf("baseURL is required")
	}
	if config.DFSPID == "" {
		return nil, fmt.Errorf("DFSPID is required")
	}
	timeout := 30
	if config.TimeoutSeconds > 0 {
		timeout = config.TimeoutSeconds
	}
	return &FSPIOPClient{
		baseURL:     config.BaseURL,
		dfspID:      config.DFSPID,
		callbackURL: config.CallbackURL,
		httpClient:  &http.Client{Timeout: time.Duration(timeout) * time.Second},
	}, nil
}

// Party represents a party in the Mojaloop network
type Party struct {
	PartyIdType      string             `json:"partyIdType"`
	PartyIdentifier  string             `json:"partyIdentifier"`
	PartySubIdOrType string             `json:"partySubIdOrType,omitempty"`
	FspId            string             `json:"fspId,omitempty"`
	Name             string             `json:"name,omitempty"`
	PersonalInfo     *PartyPersonalInfo `json:"personalInfo,omitempty"`
}

// PartyPersonalInfo contains personal information about a party
type PartyPersonalInfo struct {
	ComplexName *ComplexName `json:"complexName,omitempty"`
	DateOfBirth string       `json:"dateOfBirth,omitempty"`
}

// ComplexName represents a complex name structure
type ComplexName struct {
	FirstName  string `json:"firstName,omitempty"`
	MiddleName string `json:"middleName,omitempty"`
	LastName   string `json:"lastName,omitempty"`
}

// Money represents a monetary amount
type Money struct {
	Currency string `json:"currency"`
	Amount   string `json:"amount"`
}

// QuoteRequest represents a quote request in FSPIOP
type QuoteRequest struct {
	QuoteId              string          `json:"quoteId"`
	TransactionId        string          `json:"transactionId"`
	TransactionRequestId string          `json:"transactionRequestId,omitempty"`
	Payee                Party           `json:"payee"`
	Payer                Party           `json:"payer"`
	AmountType           string          `json:"amountType"`
	Amount               Money           `json:"amount"`
	Fees                 *Money          `json:"fees,omitempty"`
	TransactionType      TransactionType `json:"transactionType"`
	Note                 string          `json:"note,omitempty"`
	Expiration           string          `json:"expiration,omitempty"`
	ExtensionList        *ExtensionList  `json:"extensionList,omitempty"`
	CreatedAt            time.Time       `json:"-"`
	Status               string          `json:"-"`
}

// QuoteResponse represents a quote response in FSPIOP
type QuoteResponse struct {
	TransferAmount     Money          `json:"transferAmount"`
	PayeeReceiveAmount *Money         `json:"payeeReceiveAmount,omitempty"`
	PayeeFspFee        *Money         `json:"payeeFspFee,omitempty"`
	PayeeFspCommission *Money         `json:"payeeFspCommission,omitempty"`
	Expiration         string         `json:"expiration"`
	IlpPacket          string         `json:"ilpPacket"`
	Condition          string         `json:"condition"`
	ExtensionList      *ExtensionList `json:"extensionList,omitempty"`
}

// TransactionType represents the type of transaction
type TransactionType struct {
	Scenario          string      `json:"scenario"`
	SubScenario       string      `json:"subScenario,omitempty"`
	Initiator         string      `json:"initiator"`
	InitiatorType     string      `json:"initiatorType"`
	RefundInfo        *RefundInfo `json:"refundInfo,omitempty"`
	BalanceOfPayments string      `json:"balanceOfPayments,omitempty"`
}

// RefundInfo contains refund information
type RefundInfo struct {
	OriginalTransactionId string `json:"originalTransactionId"`
	RefundReason          string `json:"refundReason,omitempty"`
}

// ExtensionList contains extension key-value pairs
type ExtensionList struct {
	Extension []Extension `json:"extension"`
}

// Extension represents a single extension
type Extension struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// TransferRequest represents a transfer request in FSPIOP
type TransferRequest struct {
	TransferId    string         `json:"transferId"`
	PayeeFsp      string         `json:"payeeFsp"`
	PayerFsp      string         `json:"payerFsp"`
	Amount        Money          `json:"amount"`
	IlpPacket     string         `json:"ilpPacket"`
	Condition     string         `json:"condition"`
	Expiration    string         `json:"expiration"`
	ExtensionList *ExtensionList `json:"extensionList,omitempty"`
	QuoteId       string         `json:"-"`
	CreatedAt     time.Time      `json:"-"`
	Status        string         `json:"-"`
}

// TransferResponse represents a transfer response (fulfilment) in FSPIOP
type TransferResponse struct {
	Fulfilment         string         `json:"fulfilment,omitempty"`
	CompletedTimestamp string         `json:"completedTimestamp,omitempty"`
	TransferState      string         `json:"transferState"`
	ExtensionList      *ExtensionList `json:"extensionList,omitempty"`
}

// TransferState constants
const (
	TransferStateReceived  = "RECEIVED"
	TransferStatePending   = "PENDING"
	TransferStateCommitted = "COMMITTED"
	TransferStateAborted   = "ABORTED"
)

// ErrorInformation represents an error in FSPIOP
type ErrorInformation struct {
	ErrorCode        string         `json:"errorCode"`
	ErrorDescription string         `json:"errorDescription"`
	ExtensionList    *ExtensionList `json:"extensionList,omitempty"`
}

func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func (c *FSPIOPClient) setFSPIOPHeaders(req *http.Request, destination string) {
	req.Header.Set("Content-Type", "application/vnd.interoperability.quotes+json;version=1.1")
	req.Header.Set("Accept", "application/vnd.interoperability.quotes+json;version=1.1")
	req.Header.Set("Date", time.Now().UTC().Format(http.TimeFormat))
	req.Header.Set("FSPIOP-Source", c.dfspID)
	if destination != "" {
		req.Header.Set("FSPIOP-Destination", destination)
	}
}

// PartyLookup performs a party lookup (GET /parties/{Type}/{ID})
func (c *FSPIOPClient) PartyLookup(ctx context.Context, partyIdType, partyIdentifier string) (*Party, error) {
	url := fmt.Sprintf("%s/parties/%s/%s", c.baseURL, partyIdType, partyIdentifier)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	c.setFSPIOPHeaders(req, "")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("party lookup failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusAccepted {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("party lookup failed with status %d: %s", resp.StatusCode, string(body))
	}
	var party Party
	if err := json.NewDecoder(resp.Body).Decode(&party); err != nil {
		return nil, fmt.Errorf("failed to decode party response: %w", err)
	}
	return &party, nil
}

// RequestQuote initiates a quote request (POST /quotes)
func (c *FSPIOPClient) RequestQuote(ctx context.Context, payer, payee Party, amount Money, transactionType TransactionType) (*QuoteRequest, error) {
	quoteID := generateID()
	transactionID := generateID()
	quoteReq := &QuoteRequest{
		QuoteId: quoteID, TransactionId: transactionID, Payer: payer, Payee: payee,
		AmountType: "SEND", Amount: amount, TransactionType: transactionType,
		Expiration: time.Now().Add(5 * time.Minute).UTC().Format(time.RFC3339),
		CreatedAt: time.Now(), Status: "PENDING",
	}
	body, err := json.Marshal(quoteReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal quote request: %w", err)
	}
	url := fmt.Sprintf("%s/quotes", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	c.setFSPIOPHeaders(req, payee.FspId)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("quote request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("quote request failed with status %d: %s", resp.StatusCode, string(respBody))
	}
	c.pendingQuotes.Store(quoteID, quoteReq)
	return quoteReq, nil
}

// InitiateTransfer initiates a transfer (POST /transfers)
func (c *FSPIOPClient) InitiateTransfer(ctx context.Context, quoteResponse *QuoteResponse, quoteRequest *QuoteRequest) (*TransferRequest, error) {
	transferID := generateID()
	transferReq := &TransferRequest{
		TransferId: transferID, PayeeFsp: quoteRequest.Payee.FspId, PayerFsp: c.dfspID,
		Amount: quoteResponse.TransferAmount, IlpPacket: quoteResponse.IlpPacket,
		Condition: quoteResponse.Condition, Expiration: quoteResponse.Expiration,
		QuoteId: quoteRequest.QuoteId, CreatedAt: time.Now(), Status: TransferStatePending,
	}
	body, err := json.Marshal(transferReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal transfer request: %w", err)
	}
	url := fmt.Sprintf("%s/transfers", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/vnd.interoperability.transfers+json;version=1.1")
	req.Header.Set("Accept", "application/vnd.interoperability.transfers+json;version=1.1")
	req.Header.Set("Date", time.Now().UTC().Format(http.TimeFormat))
	req.Header.Set("FSPIOP-Source", c.dfspID)
	req.Header.Set("FSPIOP-Destination", quoteRequest.Payee.FspId)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("transfer request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("transfer request failed with status %d: %s", resp.StatusCode, string(respBody))
	}
	c.pendingTxs.Store(transferID, transferReq)
	return transferReq, nil
}

// GetTransferStatus retrieves the status of a transfer (GET /transfers/{ID})
func (c *FSPIOPClient) GetTransferStatus(ctx context.Context, transferID string) (*TransferResponse, error) {
	url := fmt.Sprintf("%s/transfers/%s", c.baseURL, transferID)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.interoperability.transfers+json;version=1.1")
	req.Header.Set("Date", time.Now().UTC().Format(http.TimeFormat))
	req.Header.Set("FSPIOP-Source", c.dfspID)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get transfer status failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusAccepted {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get transfer status failed with status %d: %s", resp.StatusCode, string(body))
	}
	var transferResp TransferResponse
	if err := json.NewDecoder(resp.Body).Decode(&transferResp); err != nil {
		return nil, fmt.Errorf("failed to decode transfer response: %w", err)
	}
	return &transferResp, nil
}

// BulkQuoteRequest represents a bulk quote request
type BulkQuoteRequest struct {
	BulkQuoteId      string            `json:"bulkQuoteId"`
	Payer            Party             `json:"payer"`
	IndividualQuotes []IndividualQuote `json:"individualQuotes"`
	Expiration       string            `json:"expiration,omitempty"`
	ExtensionList    *ExtensionList    `json:"extensionList,omitempty"`
}

// IndividualQuote represents a single quote in a bulk request
type IndividualQuote struct {
	QuoteId         string          `json:"quoteId"`
	TransactionId   string          `json:"transactionId"`
	Payee           Party           `json:"payee"`
	AmountType      string          `json:"amountType"`
	Amount          Money           `json:"amount"`
	TransactionType TransactionType `json:"transactionType"`
	Note            string          `json:"note,omitempty"`
	ExtensionList   *ExtensionList  `json:"extensionList,omitempty"`
}

// BulkQuoteResponse represents a bulk quote response
type BulkQuoteResponse struct {
	IndividualQuoteResults []IndividualQuoteResult `json:"individualQuoteResults"`
	Expiration             string                  `json:"expiration"`
	ExtensionList          *ExtensionList          `json:"extensionList,omitempty"`
}

// IndividualQuoteResult represents a single quote result
type IndividualQuoteResult struct {
	QuoteId            string            `json:"quoteId"`
	Payee              *Party            `json:"payee,omitempty"`
	TransferAmount     *Money            `json:"transferAmount,omitempty"`
	PayeeReceiveAmount *Money            `json:"payeeReceiveAmount,omitempty"`
	PayeeFspFee        *Money            `json:"payeeFspFee,omitempty"`
	PayeeFspCommission *Money            `json:"payeeFspCommission,omitempty"`
	IlpPacket          string            `json:"ilpPacket,omitempty"`
	Condition          string            `json:"condition,omitempty"`
	ErrorInformation   *ErrorInformation `json:"errorInformation,omitempty"`
	ExtensionList      *ExtensionList    `json:"extensionList,omitempty"`
}

// RequestBulkQuote initiates a bulk quote request (POST /bulkQuotes)
func (c *FSPIOPClient) RequestBulkQuote(ctx context.Context, payer Party, payees []Party, amounts []Money, transactionType TransactionType) (*BulkQuoteRequest, error) {
	if len(payees) != len(amounts) {
		return nil, fmt.Errorf("payees and amounts must have the same length")
	}
	bulkQuoteID := generateID()
	individualQuotes := make([]IndividualQuote, len(payees))
	for i, payee := range payees {
		individualQuotes[i] = IndividualQuote{
			QuoteId: generateID(), TransactionId: generateID(), Payee: payee,
			AmountType: "SEND", Amount: amounts[i], TransactionType: transactionType,
		}
	}
	bulkQuoteReq := &BulkQuoteRequest{
		BulkQuoteId: bulkQuoteID, Payer: payer, IndividualQuotes: individualQuotes,
		Expiration: time.Now().Add(10 * time.Minute).UTC().Format(time.RFC3339),
	}
	body, err := json.Marshal(bulkQuoteReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal bulk quote request: %w", err)
	}
	url := fmt.Sprintf("%s/bulkQuotes", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/vnd.interoperability.bulkQuotes+json;version=1.1")
	req.Header.Set("Accept", "application/vnd.interoperability.bulkQuotes+json;version=1.1")
	req.Header.Set("Date", time.Now().UTC().Format(http.TimeFormat))
	req.Header.Set("FSPIOP-Source", c.dfspID)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("bulk quote request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("bulk quote request failed with status %d: %s", resp.StatusCode, string(respBody))
	}
	return bulkQuoteReq, nil
}

// BulkTransferRequest represents a bulk transfer request
type BulkTransferRequest struct {
	BulkTransferId      string               `json:"bulkTransferId"`
	BulkQuoteId         string               `json:"bulkQuoteId"`
	PayeeFsp            string               `json:"payeeFsp"`
	PayerFsp            string               `json:"payerFsp"`
	IndividualTransfers []IndividualTransfer `json:"individualTransfers"`
	Expiration          string               `json:"expiration"`
	ExtensionList       *ExtensionList       `json:"extensionList,omitempty"`
}

// IndividualTransfer represents a single transfer in a bulk request
type IndividualTransfer struct {
	TransferId     string         `json:"transferId"`
	TransferAmount Money          `json:"transferAmount"`
	IlpPacket      string         `json:"ilpPacket"`
	Condition      string         `json:"condition"`
	ExtensionList  *ExtensionList `json:"extensionList,omitempty"`
}

// InitiateBulkTransfer initiates a bulk transfer (POST /bulkTransfers)
func (c *FSPIOPClient) InitiateBulkTransfer(ctx context.Context, bulkQuoteResponse *BulkQuoteResponse, bulkQuoteRequest *BulkQuoteRequest, payeeFsp string) (*BulkTransferRequest, error) {
	bulkTransferID := generateID()
	individualTransfers := make([]IndividualTransfer, 0)
	for _, result := range bulkQuoteResponse.IndividualQuoteResults {
		if result.ErrorInformation != nil {
			continue
		}
		individualTransfers = append(individualTransfers, IndividualTransfer{
			TransferId: generateID(), TransferAmount: *result.TransferAmount,
			IlpPacket: result.IlpPacket, Condition: result.Condition,
		})
	}
	bulkTransferReq := &BulkTransferRequest{
		BulkTransferId: bulkTransferID, BulkQuoteId: bulkQuoteRequest.BulkQuoteId,
		PayeeFsp: payeeFsp, PayerFsp: c.dfspID, IndividualTransfers: individualTransfers,
		Expiration: bulkQuoteResponse.Expiration,
	}
	body, err := json.Marshal(bulkTransferReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal bulk transfer request: %w", err)
	}
	url := fmt.Sprintf("%s/bulkTransfers", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/vnd.interoperability.bulkTransfers+json;version=1.1")
	req.Header.Set("Accept", "application/vnd.interoperability.bulkTransfers+json;version=1.1")
	req.Header.Set("Date", time.Now().UTC().Format(http.TimeFormat))
	req.Header.Set("FSPIOP-Source", c.dfspID)
	req.Header.Set("FSPIOP-Destination", payeeFsp)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("bulk transfer request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("bulk transfer request failed with status %d: %s", resp.StatusCode, string(respBody))
	}
	return bulkTransferReq, nil
}

// GetDFSPID returns the configured DFSP ID for this client
func (c *FSPIOPClient) GetDFSPID() string {
	return c.dfspID
}

// GetPendingQuote retrieves a pending quote by ID
func (c *FSPIOPClient) GetPendingQuote(quoteID string) (*QuoteRequest, bool) {
	if val, ok := c.pendingQuotes.Load(quoteID); ok {
		return val.(*QuoteRequest), true
	}
	return nil, false
}

// GetPendingTransfer retrieves a pending transfer by ID
func (c *FSPIOPClient) GetPendingTransfer(transferID string) (*TransferRequest, bool) {
	if val, ok := c.pendingTxs.Load(transferID); ok {
		return val.(*TransferRequest), true
	}
	return nil, false
}

// UpdateQuoteStatus updates the status of a pending quote
func (c *FSPIOPClient) UpdateQuoteStatus(quoteID, status string) {
	if val, ok := c.pendingQuotes.Load(quoteID); ok {
		quote := val.(*QuoteRequest)
		quote.Status = status
	}
}

// UpdateTransferStatus updates the status of a pending transfer
func (c *FSPIOPClient) UpdateTransferStatus(transferID, status string) {
	if val, ok := c.pendingTxs.Load(transferID); ok {
		transfer := val.(*TransferRequest)
		transfer.Status = status
	}
}

// RemovePendingQuote removes a quote from pending tracking
func (c *FSPIOPClient) RemovePendingQuote(quoteID string) {
	c.pendingQuotes.Delete(quoteID)
}

// RemovePendingTransfer removes a transfer from pending tracking
func (c *FSPIOPClient) RemovePendingTransfer(transferID string) {
	c.pendingTxs.Delete(transferID)
}

// Global FSPIOP client instance
var globalFSPIOPClient *FSPIOPClient
var fspiogClientMu sync.RWMutex

// InitFSPIOPClient initializes the global FSPIOP client
func InitFSPIOPClient(config FSPIOPConfig) error {
	fspiogClientMu.Lock()
	defer fspiogClientMu.Unlock()
	client, err := NewFSPIOPClient(config)
	if err != nil {
		return err
	}
	globalFSPIOPClient = client
	return nil
}

// GetFSPIOPClient returns the global FSPIOP client
func GetFSPIOPClient() *FSPIOPClient {
	fspiogClientMu.RLock()
	defer fspiogClientMu.RUnlock()
	return globalFSPIOPClient
}

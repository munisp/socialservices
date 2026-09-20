package federation

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// IdentityProvider represents a national identity provider
type IdentityProvider string

const (
	ProviderAadhaar  IdentityProvider = "aadhaar"  // India
	ProviderNIN      IdentityProvider = "nin"      // Nigeria
	ProviderNIDA     IdentityProvider = "nida"     // Tanzania
	ProviderNPR      IdentityProvider = "npr"      // Nepal
	ProviderNadra    IdentityProvider = "nadra"    // Pakistan
	ProviderDukcapil IdentityProvider = "dukcapil" // Indonesia
	ProviderCPF      IdentityProvider = "cpf"      // Brazil
	ProviderRUT      IdentityProvider = "rut"      // Chile
	ProviderCURP     IdentityProvider = "curp"     // Mexico
	ProviderSSN      IdentityProvider = "ssn"      // USA
	ProviderNHIF     IdentityProvider = "nhif"     // Kenya
	ProviderCustom   IdentityProvider = "custom"   // Custom provider
)

// VerificationStatus represents the status of identity verification
type VerificationStatus string

const (
	VerificationPending  VerificationStatus = "pending"
	VerificationVerified VerificationStatus = "verified"
	VerificationFailed   VerificationStatus = "failed"
	VerificationExpired  VerificationStatus = "expired"
	VerificationRevoked  VerificationStatus = "revoked"
)

// IdentityRecord represents a federated identity record
type IdentityRecord struct {
	ID                 int64              `json:"id" db:"id"`
	BeneficiaryID      string             `json:"beneficiaryId" db:"beneficiary_id"`
	Provider           IdentityProvider   `json:"provider" db:"provider"`
	NationalID         string             `json:"nationalId" db:"national_id"`
	NationalIDHash     string             `json:"nationalIdHash" db:"national_id_hash"`
	VerificationStatus VerificationStatus `json:"verificationStatus" db:"verification_status"`
	VerifiedAt         *time.Time         `json:"verifiedAt,omitempty" db:"verified_at"`
	ExpiresAt          *time.Time         `json:"expiresAt,omitempty" db:"expires_at"`
	VerificationData   json.RawMessage    `json:"verificationData,omitempty" db:"verification_data"`
	BiometricMatch     *float64           `json:"biometricMatch,omitempty" db:"biometric_match"`
	DemographicMatch   *float64           `json:"demographicMatch,omitempty" db:"demographic_match"`
	LastCheckedAt      *time.Time         `json:"lastCheckedAt,omitempty" db:"last_checked_at"`
	CreatedAt          time.Time          `json:"createdAt" db:"created_at"`
	UpdatedAt          time.Time          `json:"updatedAt" db:"updated_at"`
}

// VerificationRequest represents a request to verify identity
type VerificationRequest struct {
	BeneficiaryID string           `json:"beneficiaryId"`
	Provider      IdentityProvider `json:"provider"`
	NationalID    string           `json:"nationalId"`
	FirstName     string           `json:"firstName,omitempty"`
	LastName      string           `json:"lastName,omitempty"`
	DateOfBirth   string           `json:"dateOfBirth,omitempty"`
	Gender        string           `json:"gender,omitempty"`
	BiometricData []byte           `json:"biometricData,omitempty"`
	BiometricType string           `json:"biometricType,omitempty"` // fingerprint, face, iris
	ConsentToken  string           `json:"consentToken"`
	RequestedBy   int64            `json:"requestedBy"`
	Purpose       string           `json:"purpose"`
}

// VerificationResponse represents the response from identity verification
type VerificationResponse struct {
	RequestID        string             `json:"requestId"`
	Status           VerificationStatus `json:"status"`
	MatchScore       float64            `json:"matchScore"`
	BiometricMatch   float64            `json:"biometricMatch,omitempty"`
	DemographicMatch float64            `json:"demographicMatch,omitempty"`
	VerifiedName     string             `json:"verifiedName,omitempty"`
	VerifiedDOB      string             `json:"verifiedDob,omitempty"`
	VerifiedGender   string             `json:"verifiedGender,omitempty"`
	VerifiedAddress  string             `json:"verifiedAddress,omitempty"`
	PhotoURL         string             `json:"photoUrl,omitempty"`
	ErrorCode        string             `json:"errorCode,omitempty"`
	ErrorMessage     string             `json:"errorMessage,omitempty"`
	Timestamp        time.Time          `json:"timestamp"`
	ExpiresAt        time.Time          `json:"expiresAt"`
}

// ProviderConfig represents configuration for an identity provider
type ProviderConfig struct {
	Provider          IdentityProvider `json:"provider"`
	Name              string           `json:"name"`
	BaseURL           string           `json:"baseUrl"`
	AuthURL           string           `json:"authUrl"`
	VerifyURL         string           `json:"verifyUrl"`
	BiometricURL      string           `json:"biometricUrl,omitempty"`
	ClientID          string           `json:"clientId"`
	ClientSecret      string           `json:"clientSecret"`
	APIKey            string           `json:"apiKey,omitempty"`
	CertPath          string           `json:"certPath,omitempty"`
	KeyPath           string           `json:"keyPath,omitempty"`
	CAPath            string           `json:"caPath,omitempty"`
	Timeout           time.Duration    `json:"timeout"`
	RateLimit         int              `json:"rateLimit"` // requests per minute
	RetryAttempts     int              `json:"retryAttempts"`
	Enabled           bool             `json:"enabled"`
	SupportsBiometric bool             `json:"supportsBiometric"`
}

// X-Road style message format
type XRoadRequest struct {
	XMLName        xml.Name `xml:"request"`
	ServiceCode    string   `xml:"serviceCode"`
	ServiceVersion string   `xml:"serviceVersion"`
	ClientID       string   `xml:"client>id"`
	ClientName     string   `xml:"client>name"`
	RequestID      string   `xml:"id"`
	Timestamp      string   `xml:"timestamp"`
	Body           string   `xml:"body"`
	Signature      string   `xml:"signature,omitempty"`
}

type XRoadResponse struct {
	XMLName      xml.Name `xml:"response"`
	RequestID    string   `xml:"id"`
	ResponseCode string   `xml:"responseCode"`
	Timestamp    string   `xml:"timestamp"`
	Body         string   `xml:"body"`
	Signature    string   `xml:"signature,omitempty"`
}

// NationalIDFederationService manages federated identity verification
type NationalIDFederationService struct {
	db          *sql.DB
	providers   map[IdentityProvider]*ProviderConfig
	httpClients map[IdentityProvider]*http.Client
	mu          sync.RWMutex
	hmacKey     []byte
	instanceID  string
}

// NewNationalIDFederationService creates a new federation service
func NewNationalIDFederationService(db *sql.DB, hmacKey []byte, instanceID string) *NationalIDFederationService {
	return &NationalIDFederationService{
		db:          db,
		providers:   make(map[IdentityProvider]*ProviderConfig),
		httpClients: make(map[IdentityProvider]*http.Client),
		hmacKey:     hmacKey,
		instanceID:  instanceID,
	}
}

// RegisterProvider registers an identity provider
func (s *NationalIDFederationService) RegisterProvider(config *ProviderConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Create HTTP client with TLS if certificates provided
	client := &http.Client{
		Timeout: config.Timeout,
	}

	if config.CertPath != "" && config.KeyPath != "" {
		cert, err := tls.LoadX509KeyPair(config.CertPath, config.KeyPath)
		if err != nil {
			return fmt.Errorf("failed to load client certificate: %w", err)
		}

		tlsConfig := &tls.Config{
			Certificates: []tls.Certificate{cert},
		}

		if config.CAPath != "" {
			caCert, err := io.ReadAll(nil) // Would read from file in production
			if err == nil {
				caCertPool := x509.NewCertPool()
				caCertPool.AppendCertsFromPEM(caCert)
				tlsConfig.RootCAs = caCertPool
			}
		}

		client.Transport = &http.Transport{
			TLSClientConfig: tlsConfig,
		}
	}

	s.providers[config.Provider] = config
	s.httpClients[config.Provider] = client

	// Store provider config in database
	configJSON, _ := json.Marshal(config)
	_, err := s.db.Exec(`
		INSERT INTO federation_providers (provider, name, config, enabled, created_at, updated_at)
		VALUES (?, ?, ?, ?, NOW(), NOW())
		ON DUPLICATE KEY UPDATE name = VALUES(name), config = VALUES(config), enabled = VALUES(enabled), updated_at = NOW()
	`, config.Provider, config.Name, configJSON, config.Enabled)

	return err
}

// GetProvider returns a provider configuration
func (s *NationalIDFederationService) GetProvider(provider IdentityProvider) (*ProviderConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	config, ok := s.providers[provider]
	if !ok {
		return nil, fmt.Errorf("provider not registered: %s", provider)
	}
	return config, nil
}

// HashNationalID creates a secure hash of a national ID
func (s *NationalIDFederationService) HashNationalID(nationalID string, provider IdentityProvider) string {
	h := hmac.New(sha256.New, s.hmacKey)
	h.Write([]byte(fmt.Sprintf("%s:%s", provider, nationalID)))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// VerifyIdentity verifies a beneficiary's identity against a national ID system
func (s *NationalIDFederationService) VerifyIdentity(ctx context.Context, req *VerificationRequest) (*VerificationResponse, error) {
	config, err := s.GetProvider(req.Provider)
	if err != nil {
		return nil, err
	}

	if !config.Enabled {
		return nil, fmt.Errorf("provider %s is not enabled", req.Provider)
	}

	// Log the verification request
	requestID := fmt.Sprintf("%s-%d", s.instanceID, time.Now().UnixNano())
	s.logVerificationRequest(ctx, requestID, req)

	// Build and send the verification request based on provider
	var response *VerificationResponse
	switch req.Provider {
	case ProviderAadhaar:
		response, err = s.verifyAadhaar(ctx, config, req, requestID)
	case ProviderNIN:
		response, err = s.verifyNIN(ctx, config, req, requestID)
	case ProviderCPF:
		response, err = s.verifyCPF(ctx, config, req, requestID)
	default:
		response, err = s.verifyGeneric(ctx, config, req, requestID)
	}

	if err != nil {
		s.logVerificationResponse(ctx, requestID, nil, err)
		return nil, err
	}

	// Store the verification result
	s.storeVerificationResult(ctx, req, response)
	s.logVerificationResponse(ctx, requestID, response, nil)

	return response, nil
}

// verifyAadhaar verifies identity against India's Aadhaar system
func (s *NationalIDFederationService) verifyAadhaar(ctx context.Context, config *ProviderConfig, req *VerificationRequest, requestID string) (*VerificationResponse, error) {
	// Build Aadhaar authentication request
	authRequest := map[string]interface{}{
		"uid":       req.NationalID,
		"tid":       "public",
		"ac":        config.ClientID,
		"sa":        config.ClientID,
		"ver":       "2.5",
		"txn":       requestID,
		"lk":        config.APIKey,
		"consent":   "Y",
		"timestamp": time.Now().Format("2006-01-02T15:04:05"),
	}

	// Add demographic data if provided
	if req.FirstName != "" || req.LastName != "" || req.DateOfBirth != "" {
		authRequest["demo"] = map[string]interface{}{
			"name":   fmt.Sprintf("%s %s", req.FirstName, req.LastName),
			"dob":    req.DateOfBirth,
			"gender": req.Gender,
		}
	}

	// Add biometric data if provided
	if len(req.BiometricData) > 0 {
		authRequest["bio"] = map[string]interface{}{
			"type": req.BiometricType,
			"data": base64.StdEncoding.EncodeToString(req.BiometricData),
		}
	}

	// Send request to Aadhaar API
	response, err := s.sendProviderRequest(ctx, config, authRequest)
	if err != nil {
		return nil, err
	}

	return s.parseAadhaarResponse(response, requestID)
}

// verifyNIN verifies identity against Nigeria's NIN system
func (s *NationalIDFederationService) verifyNIN(ctx context.Context, config *ProviderConfig, req *VerificationRequest, requestID string) (*VerificationResponse, error) {
	// Build NIN verification request
	ninRequest := map[string]interface{}{
		"nin":         req.NationalID,
		"firstName":   req.FirstName,
		"lastName":    req.LastName,
		"dateOfBirth": req.DateOfBirth,
		"requestId":   requestID,
		"timestamp":   time.Now().Format(time.RFC3339),
	}

	if len(req.BiometricData) > 0 {
		ninRequest["fingerprint"] = base64.StdEncoding.EncodeToString(req.BiometricData)
	}

	response, err := s.sendProviderRequest(ctx, config, ninRequest)
	if err != nil {
		return nil, err
	}

	return s.parseNINResponse(response, requestID)
}

// verifyCPF verifies identity against Brazil's CPF system
func (s *NationalIDFederationService) verifyCPF(ctx context.Context, config *ProviderConfig, req *VerificationRequest, requestID string) (*VerificationResponse, error) {
	// Build CPF verification request
	cpfRequest := map[string]interface{}{
		"cpf":            req.NationalID,
		"nome":           fmt.Sprintf("%s %s", req.FirstName, req.LastName),
		"dataNascimento": req.DateOfBirth,
		"idRequisicao":   requestID,
	}

	response, err := s.sendProviderRequest(ctx, config, cpfRequest)
	if err != nil {
		return nil, err
	}

	return s.parseCPFResponse(response, requestID)
}

// verifyGeneric performs generic X-Road style verification
func (s *NationalIDFederationService) verifyGeneric(ctx context.Context, config *ProviderConfig, req *VerificationRequest, requestID string) (*VerificationResponse, error) {
	// Build X-Road style request
	bodyData := map[string]interface{}{
		"nationalId":  req.NationalID,
		"firstName":   req.FirstName,
		"lastName":    req.LastName,
		"dateOfBirth": req.DateOfBirth,
		"gender":      req.Gender,
	}
	bodyJSON, _ := json.Marshal(bodyData)

	xroadReq := XRoadRequest{
		ServiceCode:    "identity.verify",
		ServiceVersion: "v1",
		ClientID:       config.ClientID,
		ClientName:     s.instanceID,
		RequestID:      requestID,
		Timestamp:      time.Now().Format(time.RFC3339),
		Body:           base64.StdEncoding.EncodeToString(bodyJSON),
	}

	// Sign the request
	xroadReq.Signature = s.signRequest(xroadReq)

	response, err := s.sendXRoadRequest(ctx, config, xroadReq)
	if err != nil {
		return nil, err
	}

	return s.parseXRoadResponse(response, requestID)
}

// sendProviderRequest sends a request to a provider's API
func (s *NationalIDFederationService) sendProviderRequest(ctx context.Context, config *ProviderConfig, requestData map[string]interface{}) ([]byte, error) {
	s.mu.RLock()
	client := s.httpClients[config.Provider]
	s.mu.RUnlock()

	if client == nil {
		return nil, fmt.Errorf("no HTTP client for provider: %s", config.Provider)
	}

	jsonData, err := json.Marshal(requestData)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", config.VerifyURL, bytes.NewReader(jsonData))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", config.APIKey))
	req.Header.Set("X-Client-ID", config.ClientID)

	response, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("identity provider request failed: %w", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	if err != nil {
		return nil, fmt.Errorf("identity provider response read failed: %w", err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("identity provider returned %d: %s", response.StatusCode, string(body))
	}
	if !json.Valid(body) {
		return nil, fmt.Errorf("identity provider returned invalid JSON")
	}
	return body, nil
}

// sendXRoadRequest sends an X-Road style request
func (s *NationalIDFederationService) sendXRoadRequest(ctx context.Context, config *ProviderConfig, xroadReq XRoadRequest) (*XRoadResponse, error) {
	s.mu.RLock()
	client := s.httpClients[config.Provider]
	s.mu.RUnlock()

	if client == nil {
		return nil, fmt.Errorf("no HTTP client for provider: %s", config.Provider)
	}

	xmlData, err := xml.Marshal(xroadReq)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", config.VerifyURL, bytes.NewReader(xmlData))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/xml")
	req.Header.Set("X-Road-Client", config.ClientID)

	response, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("X-Road request failed: %w", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	if err != nil {
		return nil, fmt.Errorf("X-Road response read failed: %w", err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("X-Road returned %d: %s", response.StatusCode, string(body))
	}
	var parsed XRoadResponse
	if err := xml.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("invalid X-Road response: %w", err)
	}
	if parsed.RequestID != xroadReq.RequestID {
		return nil, fmt.Errorf("X-Road response request ID mismatch")
	}
	return &parsed, nil
}

// signRequest signs an X-Road request
func (s *NationalIDFederationService) signRequest(req XRoadRequest) string {
	data := fmt.Sprintf("%s:%s:%s:%s", req.ServiceCode, req.ClientID, req.RequestID, req.Timestamp)
	h := hmac.New(sha256.New, s.hmacKey)
	h.Write([]byte(data))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// Parse response methods
func (s *NationalIDFederationService) parseAadhaarResponse(data []byte, requestID string) (*VerificationResponse, error) {
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	status := VerificationVerified
	if result["status"] != "verified" {
		status = VerificationFailed
	}

	matchScore := 0.0
	if score, ok := result["matchScore"].(float64); ok {
		matchScore = score
	}

	return &VerificationResponse{
		RequestID:        requestID,
		Status:           status,
		MatchScore:       matchScore,
		BiometricMatch:   getFloat(result, "biometricMatch"),
		DemographicMatch: getFloat(result, "demographicMatch"),
		Timestamp:        time.Now(),
		ExpiresAt:        time.Now().Add(365 * 24 * time.Hour),
	}, nil
}

func (s *NationalIDFederationService) parseNINResponse(data []byte, requestID string) (*VerificationResponse, error) {
	return s.parseAadhaarResponse(data, requestID) // Similar structure
}

func (s *NationalIDFederationService) parseCPFResponse(data []byte, requestID string) (*VerificationResponse, error) {
	return s.parseAadhaarResponse(data, requestID) // Similar structure
}

func (s *NationalIDFederationService) parseXRoadResponse(resp *XRoadResponse, requestID string) (*VerificationResponse, error) {
	bodyData, err := base64.StdEncoding.DecodeString(resp.Body)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(bodyData, &result); err != nil {
		return nil, err
	}

	status := VerificationVerified
	if resp.ResponseCode != "OK" {
		status = VerificationFailed
	}

	return &VerificationResponse{
		RequestID:  requestID,
		Status:     status,
		MatchScore: getFloat(result, "matchScore"),
		Timestamp:  time.Now(),
		ExpiresAt:  time.Now().Add(365 * 24 * time.Hour),
	}, nil
}

func getFloat(m map[string]interface{}, key string) float64 {
	if v, ok := m[key].(float64); ok {
		return v
	}
	return 0
}

// Database operations
func (s *NationalIDFederationService) storeVerificationResult(ctx context.Context, req *VerificationRequest, resp *VerificationResponse) error {
	nationalIDHash := s.HashNationalID(req.NationalID, req.Provider)
	verificationData, _ := json.Marshal(resp)

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO federated_identities (
			beneficiary_id, provider, national_id_hash, verification_status,
			verified_at, expires_at, verification_data, biometric_match,
			demographic_match, last_checked_at, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW(), NOW())
		ON DUPLICATE KEY UPDATE
			verification_status = VALUES(verification_status),
			verified_at = VALUES(verified_at),
			expires_at = VALUES(expires_at),
			verification_data = VALUES(verification_data),
			biometric_match = VALUES(biometric_match),
			demographic_match = VALUES(demographic_match),
			last_checked_at = NOW(),
			updated_at = NOW()
	`,
		req.BeneficiaryID, req.Provider, nationalIDHash, resp.Status,
		resp.Timestamp, resp.ExpiresAt, verificationData,
		resp.BiometricMatch, resp.DemographicMatch,
	)
	return err
}

func (s *NationalIDFederationService) logVerificationRequest(ctx context.Context, requestID string, req *VerificationRequest) {
	reqData, _ := json.Marshal(map[string]interface{}{
		"beneficiaryId": req.BeneficiaryID,
		"provider":      req.Provider,
		"purpose":       req.Purpose,
		"requestedBy":   req.RequestedBy,
	})

	s.db.ExecContext(ctx, `
		INSERT INTO federation_audit_log (request_id, action, beneficiary_id, provider, request_data, created_at)
		VALUES (?, 'verification_request', ?, ?, ?, NOW())
	`, requestID, req.BeneficiaryID, req.Provider, reqData)
}

func (s *NationalIDFederationService) logVerificationResponse(ctx context.Context, requestID string, resp *VerificationResponse, err error) {
	var respData []byte
	var status string
	if err != nil {
		status = "error"
		respData, _ = json.Marshal(map[string]string{"error": err.Error()})
	} else {
		status = string(resp.Status)
		respData, _ = json.Marshal(resp)
	}

	s.db.ExecContext(ctx, `
		UPDATE federation_audit_log 
		SET response_data = ?, status = ?, completed_at = NOW()
		WHERE request_id = ?
	`, respData, status, requestID)
}

// GetVerificationStatus gets the current verification status for a beneficiary
func (s *NationalIDFederationService) GetVerificationStatus(ctx context.Context, beneficiaryID string, provider IdentityProvider) (*IdentityRecord, error) {
	var record IdentityRecord
	err := s.db.QueryRowContext(ctx, `
		SELECT id, beneficiary_id, provider, national_id_hash, verification_status,
			   verified_at, expires_at, verification_data, biometric_match,
			   demographic_match, last_checked_at, created_at, updated_at
		FROM federated_identities
		WHERE beneficiary_id = ? AND provider = ?
	`, beneficiaryID, provider).Scan(
		&record.ID, &record.BeneficiaryID, &record.Provider, &record.NationalIDHash,
		&record.VerificationStatus, &record.VerifiedAt, &record.ExpiresAt,
		&record.VerificationData, &record.BiometricMatch, &record.DemographicMatch,
		&record.LastCheckedAt, &record.CreatedAt, &record.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &record, nil
}

// CheckVerificationExpiry checks if verification has expired
func (s *NationalIDFederationService) CheckVerificationExpiry(ctx context.Context, beneficiaryID string, provider IdentityProvider) (bool, error) {
	record, err := s.GetVerificationStatus(ctx, beneficiaryID, provider)
	if err != nil {
		return false, err
	}

	if record.ExpiresAt == nil {
		return false, nil
	}

	return time.Now().After(*record.ExpiresAt), nil
}

// Temporal Workflows

// IdentityVerificationWorkflowInput is the input for the verification workflow
type IdentityVerificationWorkflowInput struct {
	BeneficiaryID string           `json:"beneficiaryId"`
	Provider      IdentityProvider `json:"provider"`
	NationalID    string           `json:"nationalId"`
	FirstName     string           `json:"firstName"`
	LastName      string           `json:"lastName"`
	DateOfBirth   string           `json:"dateOfBirth"`
	Gender        string           `json:"gender"`
	BiometricData []byte           `json:"biometricData,omitempty"`
	BiometricType string           `json:"biometricType,omitempty"`
	ConsentToken  string           `json:"consentToken"`
	RequestedBy   int64            `json:"requestedBy"`
	Purpose       string           `json:"purpose"`
}

// IdentityVerificationWorkflow orchestrates identity verification
func IdentityVerificationWorkflow(ctx workflow.Context, input IdentityVerificationWorkflowInput) (*VerificationResponse, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting identity verification workflow", "beneficiaryId", input.BeneficiaryID, "provider", input.Provider)

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
		HeartbeatTimeout:    30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    time.Minute,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Validate consent token
	var consentValid bool
	err := workflow.ExecuteActivity(ctx, ValidateConsentActivity, input.ConsentToken, input.BeneficiaryID).Get(ctx, &consentValid)
	if err != nil || !consentValid {
		return nil, fmt.Errorf("consent validation failed")
	}

	// Perform identity verification
	req := &VerificationRequest{
		BeneficiaryID: input.BeneficiaryID,
		Provider:      input.Provider,
		NationalID:    input.NationalID,
		FirstName:     input.FirstName,
		LastName:      input.LastName,
		DateOfBirth:   input.DateOfBirth,
		Gender:        input.Gender,
		BiometricData: input.BiometricData,
		BiometricType: input.BiometricType,
		ConsentToken:  input.ConsentToken,
		RequestedBy:   input.RequestedBy,
		Purpose:       input.Purpose,
	}

	var response *VerificationResponse
	err = workflow.ExecuteActivity(ctx, VerifyIdentityActivity, req).Get(ctx, &response)
	if err != nil {
		return nil, fmt.Errorf("identity verification failed: %w", err)
	}

	// Update beneficiary status based on verification result
	if response.Status == VerificationVerified {
		workflow.ExecuteActivity(ctx, UpdateBeneficiaryVerificationActivity, input.BeneficiaryID, true).Get(ctx, nil)
	}

	// Send notification
	workflow.ExecuteActivity(ctx, NotifyVerificationResultActivity, input.BeneficiaryID, response.Status).Get(ctx, nil)

	logger.Info("Identity verification completed", "beneficiaryId", input.BeneficiaryID, "status", response.Status)
	return response, nil
}

// BulkVerificationWorkflow handles bulk identity verification
func BulkVerificationWorkflow(ctx workflow.Context, beneficiaryIDs []string, provider IdentityProvider) (map[string]VerificationStatus, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting bulk verification workflow", "count", len(beneficiaryIDs), "provider", provider)

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

	results := make(map[string]VerificationStatus)
	var mu sync.Mutex

	// Process in batches with rate limiting
	const batchSize = 10
	for i := 0; i < len(beneficiaryIDs); i += batchSize {
		end := i + batchSize
		if end > len(beneficiaryIDs) {
			end = len(beneficiaryIDs)
		}
		batch := beneficiaryIDs[i:end]

		// Process batch in parallel
		for _, beneficiaryID := range batch {
			bid := beneficiaryID
			workflow.Go(ctx, func(ctx workflow.Context) {
				var status VerificationStatus
				err := workflow.ExecuteActivity(ctx, CheckAndVerifyActivity, bid, provider).Get(ctx, &status)
				mu.Lock()
				if err != nil {
					results[bid] = VerificationFailed
				} else {
					results[bid] = status
				}
				mu.Unlock()
			})
		}

		// Rate limit between batches
		workflow.Sleep(ctx, time.Second)
	}

	return results, nil
}

// Activity implementations

func ValidateConsentActivity(ctx context.Context, consentToken string, beneficiaryID string) (bool, error) {
	// In production, validate the consent token against a consent management system
	// Check that consent is valid, not expired, and covers the requested purpose
	return consentToken != "", nil
}

func VerifyIdentityActivity(ctx context.Context, req *VerificationRequest) (*VerificationResponse, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Verifying identity", "beneficiaryId", req.BeneficiaryID, "provider", req.Provider)

	service := getFederationServiceFromContext(ctx)
	if service == nil {
		return nil, fmt.Errorf("federation service not available")
	}

	return service.VerifyIdentity(ctx, req)
}

func UpdateBeneficiaryVerificationActivity(ctx context.Context, beneficiaryID string, verified bool) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Updating beneficiary verification status", "beneficiaryId", beneficiaryID, "verified", verified)

	service := getFederationServiceFromContext(ctx)
	if service == nil {
		return fmt.Errorf("federation service not available")
	}

	_, err := service.db.ExecContext(ctx, `
		UPDATE beneficiaries SET 
			documents_verified = ?,
			updated_at = NOW()
		WHERE id = ?
	`, verified, beneficiaryID)
	return err
}

func NotifyVerificationResultActivity(ctx context.Context, beneficiaryID string, status VerificationStatus) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Sending verification notification", "beneficiaryId", beneficiaryID, "status", status)
	// In production, send SMS/email notification
	return nil
}

func CheckAndVerifyActivity(ctx context.Context, beneficiaryID string, provider IdentityProvider) (VerificationStatus, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Checking and verifying", "beneficiaryId", beneficiaryID, "provider", provider)

	service := getFederationServiceFromContext(ctx)
	if service == nil {
		return VerificationFailed, fmt.Errorf("federation service not available")
	}

	// Check if already verified and not expired
	record, err := service.GetVerificationStatus(ctx, beneficiaryID, provider)
	if err == nil && record.VerificationStatus == VerificationVerified {
		expired, _ := service.CheckVerificationExpiry(ctx, beneficiaryID, provider)
		if !expired {
			return VerificationVerified, nil
		}
	}

	// Need to re-verify - would need beneficiary data from database
	return VerificationPending, nil
}

// Context key for federation service
type federationServiceKey struct{}

func WithFederationService(ctx context.Context, service *NationalIDFederationService) context.Context {
	return context.WithValue(ctx, federationServiceKey{}, service)
}

func getFederationServiceFromContext(ctx context.Context) *NationalIDFederationService {
	service, _ := ctx.Value(federationServiceKey{}).(*NationalIDFederationService)
	return service
}

// Database migration for federation tables
func GetFederationMigration() string {
	return `
-- Federation providers table
CREATE TABLE IF NOT EXISTS federation_providers (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    provider VARCHAR(64) NOT NULL UNIQUE,
    name VARCHAR(255) NOT NULL,
    config JSON NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_provider (provider),
    INDEX idx_enabled (enabled)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Federated identities table
CREATE TABLE IF NOT EXISTS federated_identities (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    beneficiary_id VARCHAR(64) NOT NULL,
    provider VARCHAR(64) NOT NULL,
    national_id_hash VARCHAR(128) NOT NULL,
    verification_status ENUM('pending', 'verified', 'failed', 'expired', 'revoked') NOT NULL DEFAULT 'pending',
    verified_at TIMESTAMP NULL,
    expires_at TIMESTAMP NULL,
    verification_data JSON,
    biometric_match DECIMAL(5,4),
    demographic_match DECIMAL(5,4),
    last_checked_at TIMESTAMP NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY uk_beneficiary_provider (beneficiary_id, provider),
    INDEX idx_national_id_hash (national_id_hash),
    INDEX idx_status (verification_status),
    INDEX idx_expires (expires_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Federation audit log table
CREATE TABLE IF NOT EXISTS federation_audit_log (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    request_id VARCHAR(128) NOT NULL UNIQUE,
    action VARCHAR(64) NOT NULL,
    beneficiary_id VARCHAR(64),
    provider VARCHAR(64),
    request_data JSON,
    response_data JSON,
    status VARCHAR(32),
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP NULL,
    INDEX idx_request_id (request_id),
    INDEX idx_beneficiary (beneficiary_id),
    INDEX idx_provider (provider),
    INDEX idx_created (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
`
}

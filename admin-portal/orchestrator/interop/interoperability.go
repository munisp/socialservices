package interop

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// SectorType represents different government sectors
type SectorType string

const (
	SectorHealth    SectorType = "health"
	SectorEducation SectorType = "education"
	SectorTax       SectorType = "tax"
	SectorLabor     SectorType = "labor"
	SectorAgriculture SectorType = "agriculture"
	SectorHousing   SectorType = "housing"
	SectorSocial    SectorType = "social"
)

// DataExchangeType represents the type of data exchange
type DataExchangeType string

const (
	ExchangeQuery    DataExchangeType = "query"
	ExchangePush     DataExchangeType = "push"
	ExchangeSubscribe DataExchangeType = "subscribe"
	ExchangeBulk     DataExchangeType = "bulk"
)

// ConsentStatus represents consent for data sharing
type ConsentStatus string

const (
	ConsentGranted  ConsentStatus = "granted"
	ConsentDenied   ConsentStatus = "denied"
	ConsentPending  ConsentStatus = "pending"
	ConsentRevoked  ConsentStatus = "revoked"
	ConsentExpired  ConsentStatus = "expired"
)

// SectorEndpoint represents a sector system endpoint
type SectorEndpoint struct {
	ID            string            `json:"id"`
	Sector        SectorType        `json:"sector"`
	Name          string            `json:"name"`
	Description   string            `json:"description"`
	BaseURL       string            `json:"baseUrl"`
	AuthType      string            `json:"authType"` // oauth2, api_key, mtls, x-road
	AuthConfig    json.RawMessage   `json:"authConfig"`
	DataFormats   []string          `json:"dataFormats"` // fhir, ceds, xbrl, custom
	Capabilities  []string          `json:"capabilities"`
	RateLimit     int               `json:"rateLimit"` // requests per minute
	Timeout       time.Duration     `json:"timeout"`
	Enabled       bool              `json:"enabled"`
	LastHealthCheck *time.Time      `json:"lastHealthCheck,omitempty"`
	HealthStatus  string            `json:"healthStatus"`
}

// DataSharingAgreement represents a data sharing agreement between sectors
type DataSharingAgreement struct {
	ID              int64           `json:"id" db:"id"`
	SourceSector    SectorType      `json:"sourceSector" db:"source_sector"`
	TargetSector    SectorType      `json:"targetSector" db:"target_sector"`
	DataTypes       json.RawMessage `json:"dataTypes" db:"data_types"`
	Purpose         string          `json:"purpose" db:"purpose"`
	LegalBasis      string          `json:"legalBasis" db:"legal_basis"`
	RetentionPeriod int             `json:"retentionPeriod" db:"retention_period"` // days
	Restrictions    json.RawMessage `json:"restrictions,omitempty" db:"restrictions"`
	ValidFrom       time.Time       `json:"validFrom" db:"valid_from"`
	ValidUntil      *time.Time      `json:"validUntil,omitempty" db:"valid_until"`
	Status          string          `json:"status" db:"status"` // active, suspended, terminated
	CreatedAt       time.Time       `json:"createdAt" db:"created_at"`
	UpdatedAt       time.Time       `json:"updatedAt" db:"updated_at"`
}

// BeneficiaryConsent represents individual consent for data sharing
type BeneficiaryConsent struct {
	ID            int64         `json:"id" db:"id"`
	BeneficiaryID string        `json:"beneficiaryId" db:"beneficiary_id"`
	Sector        SectorType    `json:"sector" db:"sector"`
	DataTypes     json.RawMessage `json:"dataTypes" db:"data_types"`
	Purpose       string        `json:"purpose" db:"purpose"`
	Status        ConsentStatus `json:"status" db:"status"`
	GrantedAt     *time.Time    `json:"grantedAt,omitempty" db:"granted_at"`
	ExpiresAt     *time.Time    `json:"expiresAt,omitempty" db:"expires_at"`
	RevokedAt     *time.Time    `json:"revokedAt,omitempty" db:"revoked_at"`
	ConsentToken  string        `json:"consentToken" db:"consent_token"`
	CreatedAt     time.Time     `json:"createdAt" db:"created_at"`
}

// DataExchangeLog represents a log of data exchanges
type DataExchangeLog struct {
	ID              int64            `json:"id" db:"id"`
	ExchangeID      string           `json:"exchangeId" db:"exchange_id"`
	SourceSector    SectorType       `json:"sourceSector" db:"source_sector"`
	TargetSector    SectorType       `json:"targetSector" db:"target_sector"`
	ExchangeType    DataExchangeType `json:"exchangeType" db:"exchange_type"`
	BeneficiaryID   *string          `json:"beneficiaryId,omitempty" db:"beneficiary_id"`
	DataTypes       json.RawMessage  `json:"dataTypes" db:"data_types"`
	RecordCount     int              `json:"recordCount" db:"record_count"`
	Status          string           `json:"status" db:"status"` // pending, completed, failed
	ErrorMessage    *string          `json:"errorMessage,omitempty" db:"error_message"`
	RequestedBy     int64            `json:"requestedBy" db:"requested_by"`
	AgreementID     *int64           `json:"agreementId,omitempty" db:"agreement_id"`
	ConsentID       *int64           `json:"consentId,omitempty" db:"consent_id"`
	StartedAt       time.Time        `json:"startedAt" db:"started_at"`
	CompletedAt     *time.Time       `json:"completedAt,omitempty" db:"completed_at"`
}

// Health sector data structures (FHIR-aligned)
type HealthRecord struct {
	PatientID         string          `json:"patientId"`
	NationalID        string          `json:"nationalId"`
	InsuranceStatus   string          `json:"insuranceStatus"`
	InsuranceType     string          `json:"insuranceType,omitempty"`
	LastVisitDate     *time.Time      `json:"lastVisitDate,omitempty"`
	ChronicConditions []string        `json:"chronicConditions,omitempty"`
	DisabilityStatus  string          `json:"disabilityStatus,omitempty"`
	DisabilityType    string          `json:"disabilityType,omitempty"`
	VaccinationStatus json.RawMessage `json:"vaccinationStatus,omitempty"`
	PregnancyStatus   string          `json:"pregnancyStatus,omitempty"`
	NutritionalStatus string          `json:"nutritionalStatus,omitempty"`
}

// Education sector data structures (CEDS-aligned)
type EducationRecord struct {
	StudentID          string     `json:"studentId"`
	NationalID         string     `json:"nationalId"`
	EnrollmentStatus   string     `json:"enrollmentStatus"`
	SchoolID           string     `json:"schoolId,omitempty"`
	SchoolName         string     `json:"schoolName,omitempty"`
	GradeLevel         string     `json:"gradeLevel,omitempty"`
	AttendanceRate     float64    `json:"attendanceRate,omitempty"`
	LastAttendanceDate *time.Time `json:"lastAttendanceDate,omitempty"`
	ScholarshipStatus  string     `json:"scholarshipStatus,omitempty"`
	MealProgramStatus  string     `json:"mealProgramStatus,omitempty"`
	SpecialNeeds       bool       `json:"specialNeeds"`
}

// Tax sector data structures
type TaxRecord struct {
	TaxpayerID       string   `json:"taxpayerId"`
	NationalID       string   `json:"nationalId"`
	FilingStatus     string   `json:"filingStatus"`
	LastFilingYear   int      `json:"lastFilingYear,omitempty"`
	DeclaredIncome   float64  `json:"declaredIncome,omitempty"`
	TaxBracket       string   `json:"taxBracket,omitempty"`
	PropertyOwnership bool    `json:"propertyOwnership"`
	BusinessOwnership bool    `json:"businessOwnership"`
	FormalEmployment bool     `json:"formalEmployment"`
}

// Labor sector data structures
type LaborRecord struct {
	WorkerID           string     `json:"workerId"`
	NationalID         string     `json:"nationalId"`
	EmploymentStatus   string     `json:"employmentStatus"`
	EmployerID         string     `json:"employerId,omitempty"`
	EmployerName       string     `json:"employerName,omitempty"`
	OccupationType     string     `json:"occupationType,omitempty"`
	SectorOfEmployment string     `json:"sectorOfEmployment,omitempty"`
	ContractType       string     `json:"contractType,omitempty"`
	SocialSecurityStatus string   `json:"socialSecurityStatus,omitempty"`
	LastContributionDate *time.Time `json:"lastContributionDate,omitempty"`
	UnemploymentBenefits bool     `json:"unemploymentBenefits"`
}

// CrossSectorProfile aggregates data from multiple sectors
type CrossSectorProfile struct {
	BeneficiaryID   string           `json:"beneficiaryId"`
	NationalID      string           `json:"nationalId"`
	Health          *HealthRecord    `json:"health,omitempty"`
	Education       []EducationRecord `json:"education,omitempty"` // Multiple children
	Tax             *TaxRecord       `json:"tax,omitempty"`
	Labor           *LaborRecord     `json:"labor,omitempty"`
	LastUpdated     time.Time        `json:"lastUpdated"`
	DataSources     []string         `json:"dataSources"`
	ConsentStatus   map[SectorType]ConsentStatus `json:"consentStatus"`
}

// InteroperabilityService manages cross-sector data exchange
type InteroperabilityService struct {
	db          *sql.DB
	endpoints   map[SectorType]*SectorEndpoint
	httpClients map[SectorType]*http.Client
	hmacKey     []byte
	instanceID  string
	mu          sync.RWMutex
}

// NewInteroperabilityService creates a new interoperability service
func NewInteroperabilityService(db *sql.DB, hmacKey []byte, instanceID string) *InteroperabilityService {
	return &InteroperabilityService{
		db:          db,
		endpoints:   make(map[SectorType]*SectorEndpoint),
		httpClients: make(map[SectorType]*http.Client),
		hmacKey:     hmacKey,
		instanceID:  instanceID,
	}
}

// RegisterEndpoint registers a sector endpoint
func (s *InteroperabilityService) RegisterEndpoint(endpoint *SectorEndpoint) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	client := &http.Client{
		Timeout: endpoint.Timeout,
	}

	s.endpoints[endpoint.Sector] = endpoint
	s.httpClients[endpoint.Sector] = client

	// Store in database
	configJSON, _ := json.Marshal(endpoint)
	_, err := s.db.Exec(`
		INSERT INTO sector_endpoints (id, sector, name, description, base_url, config, enabled, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, NOW(), NOW())
		ON DUPLICATE KEY UPDATE 
			name = VALUES(name), 
			description = VALUES(description),
			base_url = VALUES(base_url),
			config = VALUES(config),
			enabled = VALUES(enabled),
			updated_at = NOW()
	`, endpoint.ID, endpoint.Sector, endpoint.Name, endpoint.Description, endpoint.BaseURL, configJSON, endpoint.Enabled)

	return err
}

// GetEndpoint returns a sector endpoint
func (s *InteroperabilityService) GetEndpoint(sector SectorType) (*SectorEndpoint, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	endpoint, ok := s.endpoints[sector]
	if !ok {
		return nil, fmt.Errorf("endpoint not registered: %s", sector)
	}
	return endpoint, nil
}

// CheckConsent verifies consent for data sharing
func (s *InteroperabilityService) CheckConsent(ctx context.Context, beneficiaryID string, sector SectorType, dataTypes []string) (*BeneficiaryConsent, error) {
	var consent BeneficiaryConsent
	err := s.db.QueryRowContext(ctx, `
		SELECT id, beneficiary_id, sector, data_types, purpose, status, granted_at, expires_at, consent_token
		FROM beneficiary_consents
		WHERE beneficiary_id = ? AND sector = ? AND status = 'granted'
		AND (expires_at IS NULL OR expires_at > NOW())
		ORDER BY granted_at DESC
		LIMIT 1
	`, beneficiaryID, sector).Scan(
		&consent.ID, &consent.BeneficiaryID, &consent.Sector, &consent.DataTypes,
		&consent.Purpose, &consent.Status, &consent.GrantedAt, &consent.ExpiresAt, &consent.ConsentToken,
	)
	if err != nil {
		return nil, err
	}

	// Verify requested data types are covered by consent
	var consentedTypes []string
	json.Unmarshal(consent.DataTypes, &consentedTypes)
	
	for _, requested := range dataTypes {
		found := false
		for _, consented := range consentedTypes {
			if consented == requested || consented == "*" {
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("consent does not cover data type: %s", requested)
		}
	}

	return &consent, nil
}

// GrantConsent grants consent for data sharing
func (s *InteroperabilityService) GrantConsent(ctx context.Context, beneficiaryID string, sector SectorType, dataTypes []string, purpose string, expiresIn time.Duration) (*BeneficiaryConsent, error) {
	dataTypesJSON, _ := json.Marshal(dataTypes)
	consentToken := s.generateConsentToken(beneficiaryID, sector)
	now := time.Now()
	expiresAt := now.Add(expiresIn)

	result, err := s.db.ExecContext(ctx, `
		INSERT INTO beneficiary_consents (beneficiary_id, sector, data_types, purpose, status, granted_at, expires_at, consent_token, created_at)
		VALUES (?, ?, ?, ?, 'granted', ?, ?, ?, NOW())
	`, beneficiaryID, sector, dataTypesJSON, purpose, now, expiresAt, consentToken)
	if err != nil {
		return nil, err
	}

	id, _ := result.LastInsertId()
	return &BeneficiaryConsent{
		ID:            id,
		BeneficiaryID: beneficiaryID,
		Sector:        sector,
		DataTypes:     dataTypesJSON,
		Purpose:       purpose,
		Status:        ConsentGranted,
		GrantedAt:     &now,
		ExpiresAt:     &expiresAt,
		ConsentToken:  consentToken,
		CreatedAt:     now,
	}, nil
}

// RevokeConsent revokes consent for data sharing
func (s *InteroperabilityService) RevokeConsent(ctx context.Context, beneficiaryID string, sector SectorType) error {
	now := time.Now()
	_, err := s.db.ExecContext(ctx, `
		UPDATE beneficiary_consents 
		SET status = 'revoked', revoked_at = ?
		WHERE beneficiary_id = ? AND sector = ? AND status = 'granted'
	`, now, beneficiaryID, sector)
	return err
}

// generateConsentToken generates a consent token
func (s *InteroperabilityService) generateConsentToken(beneficiaryID string, sector SectorType) string {
	data := fmt.Sprintf("%s:%s:%d", beneficiaryID, sector, time.Now().UnixNano())
	h := hmac.New(sha256.New, s.hmacKey)
	h.Write([]byte(data))
	return base64.URLEncoding.EncodeToString(h.Sum(nil))
}

// QueryHealthData queries health sector data
func (s *InteroperabilityService) QueryHealthData(ctx context.Context, beneficiaryID string, nationalID string, consentToken string) (*HealthRecord, error) {
	endpoint, err := s.GetEndpoint(SectorHealth)
	if err != nil {
		return nil, err
	}

	if !endpoint.Enabled {
		return nil, fmt.Errorf("health sector endpoint is not enabled")
	}

	// Build request
	requestData := map[string]interface{}{
		"nationalId":   nationalID,
		"consentToken": consentToken,
		"requestId":    fmt.Sprintf("%s-%d", s.instanceID, time.Now().UnixNano()),
		"dataTypes":    []string{"insurance", "disability", "chronic_conditions"},
	}

	response, err := s.sendSectorRequest(ctx, endpoint, "/api/v1/patient/lookup", requestData)
	if err != nil {
		return nil, err
	}

	var healthRecord HealthRecord
	if err := json.Unmarshal(response, &healthRecord); err != nil {
		return nil, err
	}

	return &healthRecord, nil
}

// QueryEducationData queries education sector data
func (s *InteroperabilityService) QueryEducationData(ctx context.Context, beneficiaryID string, nationalID string, consentToken string) ([]EducationRecord, error) {
	endpoint, err := s.GetEndpoint(SectorEducation)
	if err != nil {
		return nil, err
	}

	if !endpoint.Enabled {
		return nil, fmt.Errorf("education sector endpoint is not enabled")
	}

	requestData := map[string]interface{}{
		"nationalId":   nationalID,
		"consentToken": consentToken,
		"requestId":    fmt.Sprintf("%s-%d", s.instanceID, time.Now().UnixNano()),
		"dataTypes":    []string{"enrollment", "attendance", "scholarship"},
	}

	response, err := s.sendSectorRequest(ctx, endpoint, "/api/v1/student/lookup", requestData)
	if err != nil {
		return nil, err
	}

	var records []EducationRecord
	if err := json.Unmarshal(response, &records); err != nil {
		return nil, err
	}

	return records, nil
}

// QueryTaxData queries tax sector data
func (s *InteroperabilityService) QueryTaxData(ctx context.Context, beneficiaryID string, nationalID string, consentToken string) (*TaxRecord, error) {
	endpoint, err := s.GetEndpoint(SectorTax)
	if err != nil {
		return nil, err
	}

	if !endpoint.Enabled {
		return nil, fmt.Errorf("tax sector endpoint is not enabled")
	}

	requestData := map[string]interface{}{
		"nationalId":   nationalID,
		"consentToken": consentToken,
		"requestId":    fmt.Sprintf("%s-%d", s.instanceID, time.Now().UnixNano()),
		"dataTypes":    []string{"filing_status", "income", "property"},
	}

	response, err := s.sendSectorRequest(ctx, endpoint, "/api/v1/taxpayer/lookup", requestData)
	if err != nil {
		return nil, err
	}

	var taxRecord TaxRecord
	if err := json.Unmarshal(response, &taxRecord); err != nil {
		return nil, err
	}

	return &taxRecord, nil
}

// QueryLaborData queries labor sector data
func (s *InteroperabilityService) QueryLaborData(ctx context.Context, beneficiaryID string, nationalID string, consentToken string) (*LaborRecord, error) {
	endpoint, err := s.GetEndpoint(SectorLabor)
	if err != nil {
		return nil, err
	}

	if !endpoint.Enabled {
		return nil, fmt.Errorf("labor sector endpoint is not enabled")
	}

	requestData := map[string]interface{}{
		"nationalId":   nationalID,
		"consentToken": consentToken,
		"requestId":    fmt.Sprintf("%s-%d", s.instanceID, time.Now().UnixNano()),
		"dataTypes":    []string{"employment", "social_security", "unemployment"},
	}

	response, err := s.sendSectorRequest(ctx, endpoint, "/api/v1/worker/lookup", requestData)
	if err != nil {
		return nil, err
	}

	var laborRecord LaborRecord
	if err := json.Unmarshal(response, &laborRecord); err != nil {
		return nil, err
	}

	return &laborRecord, nil
}

// sendSectorRequest sends a request to a sector endpoint
func (s *InteroperabilityService) sendSectorRequest(ctx context.Context, endpoint *SectorEndpoint, path string, data map[string]interface{}) ([]byte, error) {
	s.mu.RLock()
	client := s.httpClients[endpoint.Sector]
	s.mu.RUnlock()

	if client == nil {
		return nil, fmt.Errorf("no HTTP client for sector: %s", endpoint.Sector)
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	url := endpoint.BaseURL + path
	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Request-ID", data["requestId"].(string))

	// Add authentication based on auth type
	var authConfig map[string]string
	json.Unmarshal(endpoint.AuthConfig, &authConfig)

	switch endpoint.AuthType {
	case "api_key":
		req.Header.Set("X-API-Key", authConfig["apiKey"])
	case "oauth2":
		req.Header.Set("Authorization", "Bearer "+authConfig["accessToken"])
	case "x-road":
		req.Header.Set("X-Road-Client", authConfig["clientId"])
		req.Header.Set("X-Road-Service", authConfig["serviceId"])
	}

	// In production, this would send the actual request
	// For now, return simulated response
	_ = jsonData
	return s.simulateSectorResponse(endpoint.Sector, data)
}

// simulateSectorResponse simulates a sector response
func (s *InteroperabilityService) simulateSectorResponse(sector SectorType, request map[string]interface{}) ([]byte, error) {
	switch sector {
	case SectorHealth:
		return json.Marshal(HealthRecord{
			PatientID:       "H" + request["nationalId"].(string),
			NationalID:      request["nationalId"].(string),
			InsuranceStatus: "active",
			InsuranceType:   "public",
		})
	case SectorEducation:
		return json.Marshal([]EducationRecord{{
			StudentID:        "S" + request["nationalId"].(string),
			NationalID:       request["nationalId"].(string),
			EnrollmentStatus: "enrolled",
			AttendanceRate:   0.92,
		}})
	case SectorTax:
		return json.Marshal(TaxRecord{
			TaxpayerID:   "T" + request["nationalId"].(string),
			NationalID:   request["nationalId"].(string),
			FilingStatus: "filed",
		})
	case SectorLabor:
		return json.Marshal(LaborRecord{
			WorkerID:         "W" + request["nationalId"].(string),
			NationalID:       request["nationalId"].(string),
			EmploymentStatus: "employed",
		})
	default:
		return nil, fmt.Errorf("unknown sector: %s", sector)
	}
}

// BuildCrossSectorProfile builds a comprehensive profile from multiple sectors
func (s *InteroperabilityService) BuildCrossSectorProfile(ctx context.Context, beneficiaryID string, nationalID string) (*CrossSectorProfile, error) {
	profile := &CrossSectorProfile{
		BeneficiaryID: beneficiaryID,
		NationalID:    nationalID,
		DataSources:   []string{},
		ConsentStatus: make(map[SectorType]ConsentStatus),
		LastUpdated:   time.Now(),
	}

	// Check consent and query each sector
	sectors := []SectorType{SectorHealth, SectorEducation, SectorTax, SectorLabor}

	for _, sector := range sectors {
		consent, err := s.CheckConsent(ctx, beneficiaryID, sector, []string{"*"})
		if err != nil {
			profile.ConsentStatus[sector] = ConsentDenied
			continue
		}
		profile.ConsentStatus[sector] = ConsentGranted

		switch sector {
		case SectorHealth:
			health, err := s.QueryHealthData(ctx, beneficiaryID, nationalID, consent.ConsentToken)
			if err == nil {
				profile.Health = health
				profile.DataSources = append(profile.DataSources, "health")
			}
		case SectorEducation:
			education, err := s.QueryEducationData(ctx, beneficiaryID, nationalID, consent.ConsentToken)
			if err == nil {
				profile.Education = education
				profile.DataSources = append(profile.DataSources, "education")
			}
		case SectorTax:
			tax, err := s.QueryTaxData(ctx, beneficiaryID, nationalID, consent.ConsentToken)
			if err == nil {
				profile.Tax = tax
				profile.DataSources = append(profile.DataSources, "tax")
			}
		case SectorLabor:
			labor, err := s.QueryLaborData(ctx, beneficiaryID, nationalID, consent.ConsentToken)
			if err == nil {
				profile.Labor = labor
				profile.DataSources = append(profile.DataSources, "labor")
			}
		}
	}

	return profile, nil
}

// LogDataExchange logs a data exchange
func (s *InteroperabilityService) LogDataExchange(ctx context.Context, log *DataExchangeLog) error {
	dataTypesJSON, _ := json.Marshal(log.DataTypes)
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO data_exchange_logs (
			exchange_id, source_sector, target_sector, exchange_type,
			beneficiary_id, data_types, record_count, status,
			requested_by, agreement_id, consent_id, started_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		log.ExchangeID, log.SourceSector, log.TargetSector, log.ExchangeType,
		log.BeneficiaryID, dataTypesJSON, log.RecordCount, log.Status,
		log.RequestedBy, log.AgreementID, log.ConsentID, log.StartedAt,
	)
	return err
}

// Temporal Workflows

// CrossSectorQueryWorkflowInput is the input for cross-sector query workflow
type CrossSectorQueryWorkflowInput struct {
	BeneficiaryID string       `json:"beneficiaryId"`
	NationalID    string       `json:"nationalId"`
	Sectors       []SectorType `json:"sectors"`
	RequestedBy   int64        `json:"requestedBy"`
	Purpose       string       `json:"purpose"`
}

// CrossSectorQueryWorkflow orchestrates cross-sector data queries
func CrossSectorQueryWorkflow(ctx workflow.Context, input CrossSectorQueryWorkflowInput) (*CrossSectorProfile, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting cross-sector query workflow", "beneficiaryId", input.BeneficiaryID)

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Minute,
		HeartbeatTimeout:    time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    time.Minute,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	profile := &CrossSectorProfile{
		BeneficiaryID: input.BeneficiaryID,
		NationalID:    input.NationalID,
		DataSources:   []string{},
		ConsentStatus: make(map[SectorType]ConsentStatus),
		LastUpdated:   workflow.Now(ctx),
	}

	// Query each sector in parallel
	var futures []workflow.Future
	for _, sector := range input.Sectors {
		sec := sector
		future := workflow.ExecuteActivity(ctx, QuerySectorActivity, input.BeneficiaryID, input.NationalID, sec)
		futures = append(futures, future)
	}

	// Collect results
	for i, future := range futures {
		sector := input.Sectors[i]
		var result json.RawMessage
		err := future.Get(ctx, &result)
		if err != nil {
			profile.ConsentStatus[sector] = ConsentDenied
			continue
		}

		profile.ConsentStatus[sector] = ConsentGranted
		profile.DataSources = append(profile.DataSources, string(sector))

		// Parse result based on sector
		switch sector {
		case SectorHealth:
			var health HealthRecord
			json.Unmarshal(result, &health)
			profile.Health = &health
		case SectorEducation:
			var education []EducationRecord
			json.Unmarshal(result, &education)
			profile.Education = education
		case SectorTax:
			var tax TaxRecord
			json.Unmarshal(result, &tax)
			profile.Tax = &tax
		case SectorLabor:
			var labor LaborRecord
			json.Unmarshal(result, &labor)
			profile.Labor = &labor
		}
	}

	// Log the data exchange
	workflow.ExecuteActivity(ctx, LogExchangeActivity, input, profile).Get(ctx, nil)

	logger.Info("Cross-sector query completed", "beneficiaryId", input.BeneficiaryID, "sources", profile.DataSources)
	return profile, nil
}

// BulkDataExchangeWorkflow handles bulk data exchange between sectors
func BulkDataExchangeWorkflow(ctx workflow.Context, sourceSector SectorType, targetSector SectorType, beneficiaryIDs []string) (int, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting bulk data exchange", "source", sourceSector, "target", targetSector, "count", len(beneficiaryIDs))

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Minute,
		HeartbeatTimeout:    5 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    5 * time.Minute,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Process in batches
	const batchSize = 100
	successCount := 0

	for i := 0; i < len(beneficiaryIDs); i += batchSize {
		end := i + batchSize
		if end > len(beneficiaryIDs) {
			end = len(beneficiaryIDs)
		}
		batch := beneficiaryIDs[i:end]

		var batchSuccess int
		err := workflow.ExecuteActivity(ctx, ProcessBatchExchangeActivity, sourceSector, targetSector, batch).Get(ctx, &batchSuccess)
		if err != nil {
			logger.Warn("Batch processing failed", "batch", i/batchSize, "error", err)
			continue
		}
		successCount += batchSuccess
	}

	return successCount, nil
}

// Activity implementations

func QuerySectorActivity(ctx context.Context, beneficiaryID string, nationalID string, sector SectorType) (json.RawMessage, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Querying sector", "sector", sector, "beneficiaryId", beneficiaryID)

	service := getInteropServiceFromContext(ctx)
	if service == nil {
		return nil, fmt.Errorf("interoperability service not available")
	}

	// Check consent
	consent, err := service.CheckConsent(ctx, beneficiaryID, sector, []string{"*"})
	if err != nil {
		return nil, fmt.Errorf("consent check failed: %w", err)
	}

	var result interface{}
	switch sector {
	case SectorHealth:
		result, err = service.QueryHealthData(ctx, beneficiaryID, nationalID, consent.ConsentToken)
	case SectorEducation:
		result, err = service.QueryEducationData(ctx, beneficiaryID, nationalID, consent.ConsentToken)
	case SectorTax:
		result, err = service.QueryTaxData(ctx, beneficiaryID, nationalID, consent.ConsentToken)
	case SectorLabor:
		result, err = service.QueryLaborData(ctx, beneficiaryID, nationalID, consent.ConsentToken)
	default:
		return nil, fmt.Errorf("unknown sector: %s", sector)
	}

	if err != nil {
		return nil, err
	}

	return json.Marshal(result)
}

func LogExchangeActivity(ctx context.Context, input CrossSectorQueryWorkflowInput, profile *CrossSectorProfile) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Logging data exchange", "beneficiaryId", input.BeneficiaryID)

	service := getInteropServiceFromContext(ctx)
	if service == nil {
		return fmt.Errorf("interoperability service not available")
	}

	for _, sector := range input.Sectors {
		if profile.ConsentStatus[sector] == ConsentGranted {
			log := &DataExchangeLog{
				ExchangeID:    fmt.Sprintf("EX-%d", time.Now().UnixNano()),
				SourceSector:  sector,
				TargetSector:  SectorSocial,
				ExchangeType:  ExchangeQuery,
				BeneficiaryID: &input.BeneficiaryID,
				RecordCount:   1,
				Status:        "completed",
				RequestedBy:   input.RequestedBy,
				StartedAt:     time.Now(),
			}
			service.LogDataExchange(ctx, log)
		}
	}

	return nil
}

func ProcessBatchExchangeActivity(ctx context.Context, sourceSector SectorType, targetSector SectorType, beneficiaryIDs []string) (int, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Processing batch exchange", "source", sourceSector, "target", targetSector, "count", len(beneficiaryIDs))

	// In production, this would process the batch
	// For now, return success count
	return len(beneficiaryIDs), nil
}

// Context key for interoperability service
type interopServiceKey struct{}

func WithInteropService(ctx context.Context, service *InteroperabilityService) context.Context {
	return context.WithValue(ctx, interopServiceKey{}, service)
}

func getInteropServiceFromContext(ctx context.Context) *InteroperabilityService {
	service, _ := ctx.Value(interopServiceKey{}).(*InteroperabilityService)
	return service
}

// Database migration for interoperability tables
func GetInteropMigration() string {
	return `
-- Sector endpoints table
CREATE TABLE IF NOT EXISTS sector_endpoints (
    id VARCHAR(64) PRIMARY KEY,
    sector VARCHAR(32) NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    base_url VARCHAR(512) NOT NULL,
    config JSON NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    last_health_check TIMESTAMP NULL,
    health_status VARCHAR(32) DEFAULT 'unknown',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY uk_sector (sector),
    INDEX idx_enabled (enabled)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Data sharing agreements table
CREATE TABLE IF NOT EXISTS data_sharing_agreements (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    source_sector VARCHAR(32) NOT NULL,
    target_sector VARCHAR(32) NOT NULL,
    data_types JSON NOT NULL,
    purpose TEXT NOT NULL,
    legal_basis VARCHAR(255) NOT NULL,
    retention_period INT NOT NULL DEFAULT 365,
    restrictions JSON,
    valid_from TIMESTAMP NOT NULL,
    valid_until TIMESTAMP NULL,
    status ENUM('active', 'suspended', 'terminated') NOT NULL DEFAULT 'active',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_sectors (source_sector, target_sector),
    INDEX idx_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Beneficiary consents table
CREATE TABLE IF NOT EXISTS beneficiary_consents (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    beneficiary_id VARCHAR(64) NOT NULL,
    sector VARCHAR(32) NOT NULL,
    data_types JSON NOT NULL,
    purpose TEXT NOT NULL,
    status ENUM('granted', 'denied', 'pending', 'revoked', 'expired') NOT NULL DEFAULT 'pending',
    granted_at TIMESTAMP NULL,
    expires_at TIMESTAMP NULL,
    revoked_at TIMESTAMP NULL,
    consent_token VARCHAR(128),
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_beneficiary_sector (beneficiary_id, sector),
    INDEX idx_status (status),
    INDEX idx_expires (expires_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Data exchange logs table
CREATE TABLE IF NOT EXISTS data_exchange_logs (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    exchange_id VARCHAR(64) NOT NULL UNIQUE,
    source_sector VARCHAR(32) NOT NULL,
    target_sector VARCHAR(32) NOT NULL,
    exchange_type ENUM('query', 'push', 'subscribe', 'bulk') NOT NULL,
    beneficiary_id VARCHAR(64),
    data_types JSON NOT NULL,
    record_count INT NOT NULL DEFAULT 0,
    status ENUM('pending', 'completed', 'failed') NOT NULL DEFAULT 'pending',
    error_message TEXT,
    requested_by BIGINT NOT NULL,
    agreement_id BIGINT,
    consent_id BIGINT,
    started_at TIMESTAMP NOT NULL,
    completed_at TIMESTAMP NULL,
    INDEX idx_exchange_id (exchange_id),
    INDEX idx_sectors (source_sector, target_sector),
    INDEX idx_beneficiary (beneficiary_id),
    INDEX idx_status (status),
    INDEX idx_started (started_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Cross-sector profiles cache table
CREATE TABLE IF NOT EXISTS cross_sector_profiles (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    beneficiary_id VARCHAR(64) NOT NULL UNIQUE,
    national_id_hash VARCHAR(128) NOT NULL,
    profile_data JSON NOT NULL,
    data_sources JSON NOT NULL,
    consent_status JSON NOT NULL,
    last_updated TIMESTAMP NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_beneficiary (beneficiary_id),
    INDEX idx_expires (expires_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
`
}

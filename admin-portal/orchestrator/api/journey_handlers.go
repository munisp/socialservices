package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/admin-portal/orchestrator/journeys"
	"github.com/google/uuid"
)

// JourneyHandler handles journey API requests
type JourneyHandler struct {
	registry *journeys.JourneyRegistry
}

// NewJourneyHandler creates a new journey handler
func NewJourneyHandler() *JourneyHandler {
	registry := journeys.NewJourneyRegistry()

	// Register all journeys
	journeys.RegisterEnrollmentJourneys(registry)
	journeys.RegisterPaymentJourneys(registry)
	journeys.RegisterGrievanceJourneys(registry)
	journeys.RegisterLifecycleJourneys(registry)
	journeys.RegisterAdminJourneys(registry)
	journeys.RegisterReportingJourneys(registry)
	journeys.RegisterFraudJourneys(registry)

	return &JourneyHandler{
		registry: registry,
	}
}

// HandleJourneys routes journey API requests
func (h *JourneyHandler) HandleJourneys(w http.ResponseWriter, r *http.Request) {
	if os.Getenv("ENABLE_INCOMPLETE_JOURNEYS") != "true" {
		http.Error(w, "journey execution is disabled until durable activity integrations are configured", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	path := strings.TrimPrefix(r.URL.Path, "/api/journeys/")

	switch {
	case path == "" && r.Method == "GET":
		h.listJourneys(w, r)
	case path == "categories" && r.Method == "GET":
		h.listCategories(w, r)
	case strings.HasSuffix(path, "/start") && r.Method == "POST":
		h.startJourney(w, r)
	case strings.HasSuffix(path, "/status") && r.Method == "GET":
		h.getJourneyStatus(w, r)
	case strings.Contains(path, "/") && r.Method == "GET":
		h.getJourneyDefinition(w, r)
	default:
		http.Error(w, "Not found", http.StatusNotFound)
	}
}

// listJourneys returns all registered journeys
func (h *JourneyHandler) listJourneys(w http.ResponseWriter, r *http.Request) {
	category := r.URL.Query().Get("category")

	var journeyList []*journeys.JourneyDefinition
	if category != "" {
		journeyList = h.registry.ListByCategory(category)
	} else {
		journeyList = h.registry.List()
	}

	// Convert to response format
	response := make([]map[string]interface{}, len(journeyList))
	for i, j := range journeyList {
		response[i] = map[string]interface{}{
			"key":                 j.Key,
			"name":                j.Name,
			"description":         j.Description,
			"category":            j.Category,
			"workflowType":        j.WorkflowType,
			"requiredPermissions": j.RequiredPermissions,
			"uiEntryPoints":       j.UIEntryPoints,
			"bffEndpoints":        j.BFFEndpoints,
			"middlewareHooks":     j.MiddlewareHooks,
		}
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"journeys": response,
		"total":    len(response),
	})
}

// listCategories returns all journey categories
func (h *JourneyHandler) listCategories(w http.ResponseWriter, r *http.Request) {
	categories := []map[string]interface{}{
		{
			"id":           "enrollment",
			"name":         "Enrollment",
			"description":  "Beneficiary enrollment and registration journeys",
			"journeyCount": len(h.registry.ListByCategory("enrollment")),
		},
		{
			"id":           "payments",
			"name":         "Payments",
			"description":  "Disbursement and payment processing journeys",
			"journeyCount": len(h.registry.ListByCategory("payments")),
		},
		{
			"id":           "grievance",
			"name":         "Grievance",
			"description":  "Grievance submission and resolution journeys",
			"journeyCount": len(h.registry.ListByCategory("grievance")),
		},
		{
			"id":           "lifecycle",
			"name":         "Lifecycle",
			"description":  "Beneficiary lifecycle management journeys",
			"journeyCount": len(h.registry.ListByCategory("lifecycle")),
		},
		{
			"id":           "admin",
			"name":         "Admin",
			"description":  "Administrative and operational journeys",
			"journeyCount": len(h.registry.ListByCategory("admin")),
		},
		{
			"id":           "reporting",
			"name":         "Reporting",
			"description":  "Reporting and analytics journeys",
			"journeyCount": len(h.registry.ListByCategory("reporting")),
		},
		{
			"id":           "fraud",
			"name":         "Fraud & Compliance",
			"description":  "Fraud detection and compliance journeys",
			"journeyCount": len(h.registry.ListByCategory("fraud")),
		},
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"categories": categories,
	})
}

// getJourneyDefinition returns a specific journey definition
func (h *JourneyHandler) getJourneyDefinition(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/journeys/")
	journeyKey := strings.TrimSuffix(path, "/status")
	journeyKey = strings.TrimSuffix(journeyKey, "/start")

	journey, ok := h.registry.Get(journeyKey)
	if !ok {
		http.Error(w, "Journey not found", http.StatusNotFound)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"key":                 journey.Key,
		"name":                journey.Name,
		"description":         journey.Description,
		"category":            journey.Category,
		"workflowType":        journey.WorkflowType,
		"requiredPermissions": journey.RequiredPermissions,
		"uiEntryPoints":       journey.UIEntryPoints,
		"bffEndpoints":        journey.BFFEndpoints,
		"middlewareHooks":     journey.MiddlewareHooks,
	})
}

// JourneyStartRequest represents a request to start a journey
type JourneyStartRequest struct {
	Input          map[string]interface{} `json:"input"`
	IdempotencyKey string                 `json:"idempotencyKey,omitempty"`
	CorrelationID  string                 `json:"correlationId,omitempty"`
	TenantID       string                 `json:"tenantId,omitempty"`
	ActorID        string                 `json:"actorId,omitempty"`
	ActorRoles     []string               `json:"actorRoles,omitempty"`
}

// JourneyStartResponse represents the response from starting a journey
type JourneyStartResponse struct {
	JourneyRunID string    `json:"journeyRunId"`
	JourneyKey   string    `json:"journeyKey"`
	WorkflowID   string    `json:"workflowId"`
	Status       string    `json:"status"`
	StartedAt    time.Time `json:"startedAt"`
	TemporalURL  string    `json:"temporalUrl,omitempty"`
}

// startJourney starts a journey workflow
func (h *JourneyHandler) startJourney(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/journeys/")
	journeyKey := strings.TrimSuffix(path, "/start")

	// Get journey definition
	_, ok := h.registry.Get(journeyKey)
	if !ok {
		http.Error(w, fmt.Sprintf("Journey not found: %s", journeyKey), http.StatusNotFound)
		return
	}

	// Parse request
	var req JourneyStartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request: %s", err.Error()), http.StatusBadRequest)
		return
	}

	// Generate IDs
	journeyRunID := uuid.New().String()
	correlationID := req.CorrelationID
	if correlationID == "" {
		correlationID = uuid.New().String()
	}
	idempotencyKey := req.IdempotencyKey
	if idempotencyKey == "" {
		idempotencyKey = uuid.New().String()
	}

	// Create journey context
	jc := journeys.NewJourneyContext(journeyKey, req.TenantID, req.ActorID, "api_client", "api").WithRoles(req.ActorRoles)
	jc.JourneyRunID = journeyRunID
	jc.CorrelationID = correlationID
	jc.IdempotencyKey = idempotencyKey

	// Extract device info from request headers
	jc.IPAddress = r.Header.Get("X-Forwarded-For")
	if jc.IPAddress == "" {
		jc.IPAddress = r.RemoteAddr
	}
	jc.UserAgent = r.Header.Get("User-Agent")
	jc.DeviceID = r.Header.Get("X-Device-ID")

	// Generate deterministic workflow ID for Temporal idempotency
	workflowID := fmt.Sprintf("%s-%s-%s", journeyKey, req.TenantID, idempotencyKey)

	// In production, this would start the Temporal workflow
	// For now, we return the workflow details
	// temporalClient.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
	//     ID:                    workflowID,
	//     TaskQueue:             "admin-portal-orchestrator",
	//     WorkflowIDReusePolicy: enums.WORKFLOW_ID_REUSE_POLICY_REJECT_DUPLICATE,
	// }, journey.WorkflowType, input)

	response := JourneyStartResponse{
		JourneyRunID: journeyRunID,
		JourneyKey:   journeyKey,
		WorkflowID:   workflowID,
		Status:       "started",
		StartedAt:    time.Now(),
		TemporalURL:  fmt.Sprintf("/temporal/namespaces/default/workflows/%s", workflowID),
	}

	// Log journey start
	logJourneyEvent("journey.started", map[string]interface{}{
		"journeyRunId":  journeyRunID,
		"journeyKey":    journeyKey,
		"workflowId":    workflowID,
		"correlationId": correlationID,
		"tenantId":      req.TenantID,
		"actorId":       req.ActorID,
	})

	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(response)
}

// JourneyStatusResponse represents the status of a running journey
type JourneyStatusResponse struct {
	JourneyRunID string                 `json:"journeyRunId"`
	JourneyKey   string                 `json:"journeyKey"`
	WorkflowID   string                 `json:"workflowId"`
	Status       string                 `json:"status"`
	StartedAt    time.Time              `json:"startedAt"`
	CompletedAt  *time.Time             `json:"completedAt,omitempty"`
	Result       map[string]interface{} `json:"result,omitempty"`
	Error        string                 `json:"error,omitempty"`
	CurrentStep  string                 `json:"currentStep,omitempty"`
	Progress     float64                `json:"progress,omitempty"`
}

// getJourneyStatus returns the status of a running journey
func (h *JourneyHandler) getJourneyStatus(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/journeys/")
	parts := strings.Split(path, "/")
	if len(parts) < 2 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	journeyKey := parts[0]
	workflowID := r.URL.Query().Get("workflowId")
	journeyRunID := r.URL.Query().Get("journeyRunId")

	// Get journey definition
	_, ok := h.registry.Get(journeyKey)
	if !ok {
		http.Error(w, fmt.Sprintf("Journey not found: %s", journeyKey), http.StatusNotFound)
		return
	}

	// In production, this would query Temporal for workflow status
	// workflowRun := temporalClient.GetWorkflow(ctx, workflowID, "")
	// status := workflowRun.Get(ctx, &result)

	// For now, return mock status
	response := JourneyStatusResponse{
		JourneyRunID: journeyRunID,
		JourneyKey:   journeyKey,
		WorkflowID:   workflowID,
		Status:       "running",
		StartedAt:    time.Now().Add(-5 * time.Minute),
		CurrentStep:  "Processing",
		Progress:     0.5,
	}

	json.NewEncoder(w).Encode(response)
}

// logJourneyEvent logs a journey event (would publish to Kafka in production)
func logJourneyEvent(eventType string, data map[string]interface{}) {
	data["eventType"] = eventType
	data["timestamp"] = time.Now().UTC().Format(time.RFC3339)

	// In production, this would publish to Kafka
	eventJSON, _ := json.Marshal(data)
	fmt.Printf("[JOURNEY_EVENT] %s\n", string(eventJSON))
}

// ============================================================================
// Journey-Specific Start Handlers
// These provide type-safe request handling for each journey type
// ============================================================================

// StartBeneficiaryEnrollmentRequest represents the request to start beneficiary enrollment
type StartBeneficiaryEnrollmentRequest struct {
	FirstName   string                 `json:"firstName"`
	LastName    string                 `json:"lastName"`
	DateOfBirth string                 `json:"dateOfBirth"`
	Gender      string                 `json:"gender"`
	NationalID  string                 `json:"nationalId,omitempty"`
	Phone       string                 `json:"phone,omitempty"`
	Email       string                 `json:"email,omitempty"`
	Address     map[string]interface{} `json:"address,omitempty"`
	ProgramID   string                 `json:"programId,omitempty"`
	HouseholdID string                 `json:"householdId,omitempty"`
}

// StartDisbursementRequest represents the request to start a disbursement
type StartDisbursementRequest struct {
	ProgramID        string                 `json:"programId"`
	DisbursementDate string                 `json:"disbursementDate"`
	Amount           float64                `json:"amount,omitempty"`
	BeneficiaryIDs   []string               `json:"beneficiaryIds,omitempty"`
	Criteria         map[string]interface{} `json:"criteria,omitempty"`
}

// StartGrievanceRequest represents the request to submit a grievance
type StartGrievanceRequest struct {
	BeneficiaryID string `json:"beneficiaryId"`
	Category      string `json:"category"`
	Priority      string `json:"priority"`
	Subject       string `json:"subject"`
	Description   string `json:"description"`
	Channel       string `json:"channel,omitempty"`
}

// StartFraudInvestigationRequest represents the request to start a fraud investigation
type StartFraudInvestigationRequest struct {
	BeneficiaryID string `json:"beneficiaryId"`
	ReportType    string `json:"reportType"`
	Description   string `json:"description"`
	Priority      string `json:"priority"`
	AlertID       string `json:"alertId,omitempty"`
}

// StartBulkOperationRequest represents the request to start a bulk operation
type StartBulkOperationRequest struct {
	OperationType string                 `json:"operationType"`
	EntityType    string                 `json:"entityType"`
	EntityIDs     []string               `json:"entityIds"`
	Parameters    map[string]interface{} `json:"parameters,omitempty"`
	Reason        string                 `json:"reason"`
}

// StartReportRequest represents the request to generate a report
type StartReportRequest struct {
	ReportType string   `json:"reportType"`
	Period     string   `json:"period"`
	ProgramIDs []string `json:"programIds,omitempty"`
	Regions    []string `json:"regions,omitempty"`
	Format     string   `json:"format"`
}

// ============================================================================
// Journey Contract Definitions
// Maps UI entry points -> BFF endpoints -> Orchestrator endpoints -> Temporal workflows
// ============================================================================

// JourneyContract defines the full contract for a journey
type JourneyContract struct {
	JourneyKey          string   `json:"journeyKey"`
	Name                string   `json:"name"`
	UIEntryPoints       []string `json:"uiEntryPoints"`
	BFFEndpoint         string   `json:"bffEndpoint"`
	OrchestratorPath    string   `json:"orchestratorPath"`
	TemporalWorkflow    string   `json:"temporalWorkflow"`
	RequiredPermissions []string `json:"requiredPermissions"`
	MiddlewareHooks     []string `json:"middlewareHooks"`
}

// GetJourneyContracts returns all journey contracts
func GetJourneyContracts() []JourneyContract {
	return []JourneyContract{
		// Enrollment Journeys (1-5)
		{
			JourneyKey:          "beneficiary_enrollment",
			Name:                "Beneficiary Enrollment",
			UIEntryPoints:       []string{"Admin:BeneficiaryListPage", "Mobile:EnrollmentScreen"},
			BFFEndpoint:         "trpc.beneficiaries.create",
			OrchestratorPath:    "/api/journeys/beneficiary_enrollment/start",
			TemporalWorkflow:    "BeneficiaryEnrollmentJourney",
			RequiredPermissions: []string{"beneficiary:create"},
			MiddlewareHooks:     []string{"kafka", "redis", "permify", "tigerbeetle", "lakehouse"},
		},
		{
			JourneyKey:          "kyc_verification",
			Name:                "KYC Verification",
			UIEntryPoints:       []string{"Admin:BeneficiaryDetailPage", "Mobile:KYCScreen"},
			BFFEndpoint:         "trpc.beneficiaries.verifyKYC",
			OrchestratorPath:    "/api/journeys/kyc_verification/start",
			TemporalWorkflow:    "KYCVerificationJourney",
			RequiredPermissions: []string{"beneficiary:verify_kyc"},
			MiddlewareHooks:     []string{"kafka", "redis", "permify", "federation-service"},
		},
		{
			JourneyKey:          "household_registration",
			Name:                "Household Registration",
			UIEntryPoints:       []string{"Admin:HouseholdListPage", "Mobile:HouseholdScreen"},
			BFFEndpoint:         "trpc.beneficiaries.createHousehold",
			OrchestratorPath:    "/api/journeys/household_registration/start",
			TemporalWorkflow:    "HouseholdRegistrationJourney",
			RequiredPermissions: []string{"household:create"},
			MiddlewareHooks:     []string{"kafka", "redis", "permify", "pmt-service", "lakehouse"},
		},
		{
			JourneyKey:          "program_enrollment",
			Name:                "Program Enrollment",
			UIEntryPoints:       []string{"Admin:BeneficiaryDetailPage", "Admin:ProgramDetailPage"},
			BFFEndpoint:         "trpc.programs.enrollBeneficiary",
			OrchestratorPath:    "/api/journeys/program_enrollment/start",
			TemporalWorkflow:    "ProgramEnrollmentJourney",
			RequiredPermissions: []string{"program:enroll"},
			MiddlewareHooks:     []string{"kafka", "redis", "permify", "lakehouse"},
		},
		{
			JourneyKey:          "card_issuance",
			Name:                "Card Issuance",
			UIEntryPoints:       []string{"Admin:BeneficiaryDetailPage"},
			BFFEndpoint:         "trpc.beneficiaries.issueCard",
			OrchestratorPath:    "/api/journeys/card_issuance/start",
			TemporalWorkflow:    "CardIssuanceJourney",
			RequiredPermissions: []string{"card:issue"},
			MiddlewareHooks:     []string{"kafka", "redis", "permify", "tigerbeetle"},
		},
		// Payment Journeys (6-10)
		{
			JourneyKey:          "disbursement_schedule",
			Name:                "Disbursement Schedule",
			UIEntryPoints:       []string{"Admin:DisbursementsPage"},
			BFFEndpoint:         "trpc.disbursements.schedule",
			OrchestratorPath:    "/api/journeys/disbursement_schedule/start",
			TemporalWorkflow:    "DisbursementScheduleJourney",
			RequiredPermissions: []string{"disbursement:schedule"},
			MiddlewareHooks:     []string{"kafka", "redis", "permify", "tigerbeetle", "lakehouse"},
		},
		{
			JourneyKey:          "disbursement_execute",
			Name:                "Disbursement Execute",
			UIEntryPoints:       []string{"Admin:DisbursementDetailPage"},
			BFFEndpoint:         "trpc.disbursements.execute",
			OrchestratorPath:    "/api/journeys/disbursement_execute/start",
			TemporalWorkflow:    "DisbursementExecuteJourney",
			RequiredPermissions: []string{"disbursement:execute"},
			MiddlewareHooks:     []string{"kafka", "redis", "permify", "tigerbeetle", "mojaloop", "lakehouse"},
		},
		{
			JourneyKey:          "retry_disbursement",
			Name:                "Retry Disbursement",
			UIEntryPoints:       []string{"Admin:DisbursementDetailPage"},
			BFFEndpoint:         "trpc.disbursements.retry",
			OrchestratorPath:    "/api/journeys/retry_disbursement/start",
			TemporalWorkflow:    "RetryDisbursementJourney",
			RequiredPermissions: []string{"disbursement:retry"},
			MiddlewareHooks:     []string{"kafka", "redis", "permify", "tigerbeetle", "mojaloop"},
		},
		{
			JourneyKey:          "reconciliation",
			Name:                "Reconciliation",
			UIEntryPoints:       []string{"Admin:ReconciliationPage"},
			BFFEndpoint:         "trpc.disbursements.reconcile",
			OrchestratorPath:    "/api/journeys/reconciliation/start",
			TemporalWorkflow:    "ReconciliationJourney",
			RequiredPermissions: []string{"reconciliation:execute"},
			MiddlewareHooks:     []string{"kafka", "redis", "permify", "tigerbeetle", "mojaloop", "lakehouse"},
		},
		{
			JourneyKey:          "dispute_resolution",
			Name:                "Dispute Resolution",
			UIEntryPoints:       []string{"Admin:DisputesPage"},
			BFFEndpoint:         "trpc.disbursements.resolveDispute",
			OrchestratorPath:    "/api/journeys/dispute_resolution/start",
			TemporalWorkflow:    "DisputeResolutionJourney",
			RequiredPermissions: []string{"dispute:resolve"},
			MiddlewareHooks:     []string{"kafka", "redis", "permify", "tigerbeetle"},
		},
		// Grievance Journeys (11-13)
		{
			JourneyKey:          "grievance_submission",
			Name:                "Grievance Submission",
			UIEntryPoints:       []string{"Admin:GrievancesPage", "Mobile:GrievanceScreen"},
			BFFEndpoint:         "trpc.grievances.submit",
			OrchestratorPath:    "/api/journeys/grievance_submission/start",
			TemporalWorkflow:    "GrievanceSubmissionJourney",
			RequiredPermissions: []string{"grievance:create"},
			MiddlewareHooks:     []string{"kafka", "redis", "permify", "lakehouse"},
		},
		{
			JourneyKey:          "grievance_resolution",
			Name:                "Grievance Resolution",
			UIEntryPoints:       []string{"Admin:GrievanceDetailPage"},
			BFFEndpoint:         "trpc.grievances.resolve",
			OrchestratorPath:    "/api/journeys/grievance_resolution/start",
			TemporalWorkflow:    "GrievanceResolutionJourney",
			RequiredPermissions: []string{"grievance:resolve"},
			MiddlewareHooks:     []string{"kafka", "redis", "permify", "tigerbeetle", "lakehouse"},
		},
		{
			JourneyKey:          "grievance_escalation",
			Name:                "Grievance Escalation",
			UIEntryPoints:       []string{"Admin:GrievanceDetailPage"},
			BFFEndpoint:         "trpc.grievances.escalate",
			OrchestratorPath:    "/api/journeys/grievance_escalation/start",
			TemporalWorkflow:    "GrievanceEscalationJourney",
			RequiredPermissions: []string{"grievance:escalate"},
			MiddlewareHooks:     []string{"kafka", "redis", "permify"},
		},
		// Lifecycle Journeys (14-18)
		{
			JourneyKey:          "profile_update",
			Name:                "Profile Update",
			UIEntryPoints:       []string{"Admin:BeneficiaryDetailPage", "Mobile:ProfileScreen"},
			BFFEndpoint:         "trpc.beneficiaries.update",
			OrchestratorPath:    "/api/journeys/profile_update/start",
			TemporalWorkflow:    "ProfileUpdateJourney",
			RequiredPermissions: []string{"beneficiary:update"},
			MiddlewareHooks:     []string{"kafka", "redis", "permify"},
		},
		{
			JourneyKey:          "beneficiary_suspension",
			Name:                "Beneficiary Suspension",
			UIEntryPoints:       []string{"Admin:BeneficiaryDetailPage"},
			BFFEndpoint:         "trpc.beneficiaries.suspend",
			OrchestratorPath:    "/api/journeys/beneficiary_suspension/start",
			TemporalWorkflow:    "BeneficiarySuspensionJourney",
			RequiredPermissions: []string{"beneficiary:suspend"},
			MiddlewareHooks:     []string{"kafka", "redis", "permify", "tigerbeetle", "lakehouse"},
		},
		{
			JourneyKey:          "beneficiary_reactivation",
			Name:                "Beneficiary Reactivation",
			UIEntryPoints:       []string{"Admin:BeneficiaryDetailPage"},
			BFFEndpoint:         "trpc.beneficiaries.reactivate",
			OrchestratorPath:    "/api/journeys/beneficiary_reactivation/start",
			TemporalWorkflow:    "BeneficiaryReactivationJourney",
			RequiredPermissions: []string{"beneficiary:reactivate"},
			MiddlewareHooks:     []string{"kafka", "redis", "permify", "tigerbeetle", "lakehouse"},
		},
		{
			JourneyKey:          "beneficiary_exit",
			Name:                "Beneficiary Exit/Graduation",
			UIEntryPoints:       []string{"Admin:BeneficiaryDetailPage"},
			BFFEndpoint:         "trpc.beneficiaries.exit",
			OrchestratorPath:    "/api/journeys/beneficiary_exit/start",
			TemporalWorkflow:    "BeneficiaryExitJourney",
			RequiredPermissions: []string{"beneficiary:exit"},
			MiddlewareHooks:     []string{"kafka", "redis", "permify", "tigerbeetle", "lakehouse"},
		},
		{
			JourneyKey:          "death_registration",
			Name:                "Death Registration",
			UIEntryPoints:       []string{"Admin:BeneficiaryDetailPage"},
			BFFEndpoint:         "trpc.beneficiaries.registerDeath",
			OrchestratorPath:    "/api/journeys/death_registration/start",
			TemporalWorkflow:    "DeathRegistrationJourney",
			RequiredPermissions: []string{"beneficiary:register_death"},
			MiddlewareHooks:     []string{"kafka", "redis", "permify", "tigerbeetle", "lakehouse"},
		},
		// Admin Journeys (19-22)
		{
			JourneyKey:          "approval_delegation",
			Name:                "Approval Delegation",
			UIEntryPoints:       []string{"Admin:ApprovalsPage"},
			BFFEndpoint:         "trpc.approvals.delegate",
			OrchestratorPath:    "/api/journeys/approval_delegation/start",
			TemporalWorkflow:    "ApprovalDelegationJourney",
			RequiredPermissions: []string{"approval:delegate"},
			MiddlewareHooks:     []string{"kafka", "redis", "permify"},
		},
		{
			JourneyKey:          "break_glass_access",
			Name:                "Break Glass Access",
			UIEntryPoints:       []string{"Admin:EmergencyAccessPage"},
			BFFEndpoint:         "trpc.approvals.breakGlass",
			OrchestratorPath:    "/api/journeys/break_glass_access/start",
			TemporalWorkflow:    "BreakGlassAccessJourney",
			RequiredPermissions: []string{"system:break_glass"},
			MiddlewareHooks:     []string{"kafka", "redis", "permify", "lakehouse"},
		},
		{
			JourneyKey:          "bulk_operation",
			Name:                "Bulk Operations",
			UIEntryPoints:       []string{"Admin:BulkOperationsPage"},
			BFFEndpoint:         "trpc.workflow.bulkOperation",
			OrchestratorPath:    "/api/journeys/bulk_operation/start",
			TemporalWorkflow:    "BulkOperationJourney",
			RequiredPermissions: []string{"bulk:execute"},
			MiddlewareHooks:     []string{"kafka", "redis", "permify", "lakehouse"},
		},
		{
			JourneyKey:          "offline_sync",
			Name:                "Offline Sync",
			UIEntryPoints:       []string{"Mobile:SyncScreen", "Admin:SyncStatusPage"},
			BFFEndpoint:         "trpc.worldClass.offlineSync.sync",
			OrchestratorPath:    "/api/journeys/offline_sync/start",
			TemporalWorkflow:    "OfflineSyncJourney",
			RequiredPermissions: []string{"sync:execute"},
			MiddlewareHooks:     []string{"kafka", "redis", "permify", "dapr"},
		},
		// Reporting Journeys (23-26)
		{
			JourneyKey:          "monthly_reporting",
			Name:                "Monthly Reporting",
			UIEntryPoints:       []string{"Admin:ReportsPage"},
			BFFEndpoint:         "trpc.analytics.generateReport",
			OrchestratorPath:    "/api/journeys/monthly_reporting/start",
			TemporalWorkflow:    "MonthlyReportingJourney",
			RequiredPermissions: []string{"report:generate"},
			MiddlewareHooks:     []string{"kafka", "redis", "permify", "lakehouse", "rustfs"},
		},
		{
			JourneyKey:          "program_analytics",
			Name:                "Program Performance Analytics",
			UIEntryPoints:       []string{"Admin:AnalyticsDashboard"},
			BFFEndpoint:         "trpc.analytics.programPerformance",
			OrchestratorPath:    "/api/journeys/program_analytics/start",
			TemporalWorkflow:    "ProgramAnalyticsJourney",
			RequiredPermissions: []string{"analytics:view"},
			MiddlewareHooks:     []string{"kafka", "redis", "permify", "lakehouse"},
		},
		{
			JourneyKey:          "dashboard_refresh",
			Name:                "Dashboard Refresh",
			UIEntryPoints:       []string{"Admin:Dashboard"},
			BFFEndpoint:         "trpc.analytics.refreshDashboard",
			OrchestratorPath:    "/api/journeys/dashboard_refresh/start",
			TemporalWorkflow:    "DashboardRefreshJourney",
			RequiredPermissions: []string{"dashboard:view"},
			MiddlewareHooks:     []string{"redis", "permify"},
		},
		{
			JourneyKey:          "data_export",
			Name:                "Data Export",
			UIEntryPoints:       []string{"Admin:DataExportPage"},
			BFFEndpoint:         "trpc.analytics.export",
			OrchestratorPath:    "/api/journeys/data_export/start",
			TemporalWorkflow:    "DataExportJourney",
			RequiredPermissions: []string{"export:execute"},
			MiddlewareHooks:     []string{"kafka", "redis", "permify", "lakehouse", "rustfs"},
		},
		// Fraud/Compliance Journeys (27-30)
		{
			JourneyKey:          "fraud_investigation",
			Name:                "Fraud Investigation",
			UIEntryPoints:       []string{"Admin:FraudInvestigationPage"},
			BFFEndpoint:         "trpc.workflow.startFraudInvestigation",
			OrchestratorPath:    "/api/journeys/fraud_investigation/start",
			TemporalWorkflow:    "FraudInvestigationJourney",
			RequiredPermissions: []string{"fraud:investigate"},
			MiddlewareHooks:     []string{"kafka", "redis", "permify", "ml-service", "lakehouse"},
		},
		{
			JourneyKey:          "fraud_prediction",
			Name:                "ML Fraud Prediction",
			UIEntryPoints:       []string{"Admin:FraudAlertsPage"},
			BFFEndpoint:         "trpc.workflow.runFraudPrediction",
			OrchestratorPath:    "/api/journeys/fraud_prediction/start",
			TemporalWorkflow:    "FraudPredictionJourney",
			RequiredPermissions: []string{"fraud:predict"},
			MiddlewareHooks:     []string{"kafka", "redis", "permify", "ml-service", "lakehouse"},
		},
		{
			JourneyKey:          "national_id_verification",
			Name:                "National ID Verification",
			UIEntryPoints:       []string{"Mobile:VerifyIDScreen", "Admin:BeneficiaryDetailPage"},
			BFFEndpoint:         "trpc.worldClass.federation.verify",
			OrchestratorPath:    "/api/journeys/national_id_verification/start",
			TemporalWorkflow:    "NationalIDVerificationJourney",
			RequiredPermissions: []string{"identity:verify"},
			MiddlewareHooks:     []string{"kafka", "redis", "permify", "federation-service", "lakehouse"},
		},
		{
			JourneyKey:          "cross_sector_interop",
			Name:                "Cross-Sector Interoperability",
			UIEntryPoints:       []string{"Admin:InteroperabilityPage"},
			BFFEndpoint:         "trpc.worldClass.interop.execute",
			OrchestratorPath:    "/api/journeys/cross_sector_interop/start",
			TemporalWorkflow:    "CrossSectorInteropJourney",
			RequiredPermissions: []string{"interop:execute"},
			MiddlewareHooks:     []string{"kafka", "redis", "permify", "interop-service", "lakehouse"},
		},
	}
}

// HandleJourneyContracts returns all journey contracts
func (h *JourneyHandler) HandleJourneyContracts(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	contracts := GetJourneyContracts()
	json.NewEncoder(w).Encode(map[string]interface{}{
		"contracts": contracts,
		"total":     len(contracts),
	})
}

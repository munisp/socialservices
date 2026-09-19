package journeys

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"
)

// JourneyContext carries cross-cutting context through all journey workflows and activities
// This enables consistent middleware integration (Kafka events, Redis idempotency, Permify authz, audit logging)
type JourneyContext struct {
	// Core identifiers
	JourneyRunID    string    `json:"journeyRunId"`
	JourneyType     string    `json:"journeyType"`
	CorrelationID   string    `json:"correlationId"`
	IdempotencyKey  string    `json:"idempotencyKey"`
	
	// Tenant and actor information
	TenantID        string    `json:"tenantId"`
	ActorID         string    `json:"actorId"`
	ActorType       string    `json:"actorType"` // "admin", "field_officer", "beneficiary", "system"
	ActorRoles      []string  `json:"actorRoles"`
	ActorClaims     map[string]interface{} `json:"actorClaims"`
	
	// Request metadata
	Source          string    `json:"source"` // "web", "mobile", "api", "scheduled"
	DeviceID        string    `json:"deviceId,omitempty"`
	IPAddress       string    `json:"ipAddress,omitempty"`
	UserAgent       string    `json:"userAgent,omitempty"`
	
	// Timing
	StartedAt       time.Time `json:"startedAt"`
	Timeout         time.Duration `json:"timeout"`
	
	// Feature flags and configuration
	FeatureFlags    map[string]bool `json:"featureFlags,omitempty"`
	
	// Audit trail
	AuditEnabled    bool      `json:"auditEnabled"`
	AuditLevel      string    `json:"auditLevel"` // "minimal", "standard", "detailed"
}

// NewJourneyContext creates a new journey context with generated IDs
func NewJourneyContext(journeyType, tenantID, actorID, actorType, source string) *JourneyContext {
	runID := generateID("jrn")
	return &JourneyContext{
		JourneyRunID:   runID,
		JourneyType:    journeyType,
		CorrelationID:  runID,
		IdempotencyKey: runID,
		TenantID:       tenantID,
		ActorID:        actorID,
		ActorType:      actorType,
		ActorRoles:     []string{},
		ActorClaims:    make(map[string]interface{}),
		Source:         source,
		StartedAt:      time.Now(),
		Timeout:        5 * time.Minute,
		FeatureFlags:   make(map[string]bool),
		AuditEnabled:   true,
		AuditLevel:     "standard",
	}
}

// WithIdempotencyKey sets a custom idempotency key (for client-provided keys)
func (jc *JourneyContext) WithIdempotencyKey(key string) *JourneyContext {
	jc.IdempotencyKey = key
	return jc
}

// WithCorrelationID sets a custom correlation ID (for tracing across services)
func (jc *JourneyContext) WithCorrelationID(id string) *JourneyContext {
	jc.CorrelationID = id
	return jc
}

// WithRoles sets the actor's roles
func (jc *JourneyContext) WithRoles(roles []string) *JourneyContext {
	jc.ActorRoles = roles
	return jc
}

// WithClaims sets the actor's claims from JWT
func (jc *JourneyContext) WithClaims(claims map[string]interface{}) *JourneyContext {
	jc.ActorClaims = claims
	return jc
}

// WithTimeout sets the journey timeout
func (jc *JourneyContext) WithTimeout(timeout time.Duration) *JourneyContext {
	jc.Timeout = timeout
	return jc
}

// WithDeviceInfo sets device information for mobile journeys
func (jc *JourneyContext) WithDeviceInfo(deviceID, ipAddress, userAgent string) *JourneyContext {
	jc.DeviceID = deviceID
	jc.IPAddress = ipAddress
	jc.UserAgent = userAgent
	return jc
}

// JourneyDefinition defines a journey's metadata and requirements
type JourneyDefinition struct {
	Key                 string   `json:"key"`
	Name                string   `json:"name"`
	Description         string   `json:"description"`
	Category            string   `json:"category"`
	WorkflowType        string   `json:"workflowType"`
	RequiredPermissions []string `json:"requiredPermissions"`
	UIEntryPoints       []string `json:"uiEntryPoints"`
	BFFEndpoints        []string `json:"bffEndpoints"`
	MiddlewareHooks     []string `json:"middlewareHooks"`
}

// JourneyRegistry holds all registered journey definitions
type JourneyRegistry struct {
	journeys map[string]*JourneyDefinition
}

// NewJourneyRegistry creates a new journey registry
func NewJourneyRegistry() *JourneyRegistry {
	return &JourneyRegistry{
		journeys: make(map[string]*JourneyDefinition),
	}
}

// Register adds a journey definition to the registry
func (r *JourneyRegistry) Register(def *JourneyDefinition) {
	r.journeys[def.Key] = def
}

// Get retrieves a journey definition by key
func (r *JourneyRegistry) Get(key string) (*JourneyDefinition, bool) {
	def, ok := r.journeys[key]
	return def, ok
}

// List returns all registered journey definitions
func (r *JourneyRegistry) List() []*JourneyDefinition {
	defs := make([]*JourneyDefinition, 0, len(r.journeys))
	for _, def := range r.journeys {
		defs = append(defs, def)
	}
	return defs
}

// ListByCategory returns journey definitions filtered by category
func (r *JourneyRegistry) ListByCategory(category string) []*JourneyDefinition {
	defs := make([]*JourneyDefinition, 0)
	for _, def := range r.journeys {
		if def.Category == category {
			defs = append(defs, def)
		}
	}
	return defs
}

// JourneyEvent represents an event emitted during journey execution
type JourneyEvent struct {
	EventID       string                 `json:"eventId"`
	EventType     string                 `json:"eventType"`
	JourneyRunID  string                 `json:"journeyRunId"`
	JourneyType   string                 `json:"journeyType"`
	CorrelationID string                 `json:"correlationId"`
	TenantID      string                 `json:"tenantId"`
	ActorID       string                 `json:"actorId"`
	Timestamp     time.Time              `json:"timestamp"`
	Payload       map[string]interface{} `json:"payload"`
	Metadata      map[string]interface{} `json:"metadata"`
}

// NewJourneyEvent creates a new journey event from context
func NewJourneyEvent(ctx *JourneyContext, eventType string, payload map[string]interface{}) *JourneyEvent {
	return &JourneyEvent{
		EventID:       generateID("evt"),
		EventType:     eventType,
		JourneyRunID:  ctx.JourneyRunID,
		JourneyType:   ctx.JourneyType,
		CorrelationID: ctx.CorrelationID,
		TenantID:      ctx.TenantID,
		ActorID:       ctx.ActorID,
		Timestamp:     time.Now(),
		Payload:       payload,
		Metadata: map[string]interface{}{
			"source":    ctx.Source,
			"deviceId":  ctx.DeviceID,
			"ipAddress": ctx.IPAddress,
		},
	}
}

// JourneyResult represents the result of a journey execution
type JourneyResult struct {
	JourneyRunID  string                 `json:"journeyRunId"`
	JourneyType   string                 `json:"journeyType"`
	Status        string                 `json:"status"` // "completed", "failed", "cancelled", "timeout"
	StartedAt     time.Time              `json:"startedAt"`
	CompletedAt   time.Time              `json:"completedAt"`
	Duration      time.Duration          `json:"duration"`
	Output        map[string]interface{} `json:"output,omitempty"`
	Error         string                 `json:"error,omitempty"`
	StepsExecuted int                    `json:"stepsExecuted"`
	EventsEmitted int                    `json:"eventsEmitted"`
}

// JourneyStartRequest represents a request to start a journey
type JourneyStartRequest struct {
	JourneyKey     string                 `json:"journeyKey"`
	TenantID       string                 `json:"tenantId"`
	ActorID        string                 `json:"actorId"`
	ActorType      string                 `json:"actorType"`
	Source         string                 `json:"source"`
	IdempotencyKey string                 `json:"idempotencyKey,omitempty"`
	CorrelationID  string                 `json:"correlationId,omitempty"`
	Input          map[string]interface{} `json:"input"`
	DeviceID       string                 `json:"deviceId,omitempty"`
	IPAddress      string                 `json:"ipAddress,omitempty"`
	UserAgent      string                 `json:"userAgent,omitempty"`
}

// JourneyStatusResponse represents the status of a running journey
type JourneyStatusResponse struct {
	JourneyRunID   string                 `json:"journeyRunId"`
	JourneyType    string                 `json:"journeyType"`
	Status         string                 `json:"status"`
	CurrentStep    string                 `json:"currentStep,omitempty"`
	Progress       float64                `json:"progress"`
	StartedAt      time.Time              `json:"startedAt"`
	LastUpdatedAt  time.Time              `json:"lastUpdatedAt"`
	Output         map[string]interface{} `json:"output,omitempty"`
	Error          string                 `json:"error,omitempty"`
}

// Helper function to generate unique IDs
func generateID(prefix string) string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return prefix + "_" + hex.EncodeToString(bytes)
}

// ContextKey type for context values
type contextKey string

const (
	JourneyContextKey contextKey = "journeyContext"
)

// WithJourneyContext adds journey context to a Go context
func WithJourneyContext(ctx context.Context, jc *JourneyContext) context.Context {
	return context.WithValue(ctx, JourneyContextKey, jc)
}

// GetJourneyContext retrieves journey context from a Go context
func GetJourneyContext(ctx context.Context) (*JourneyContext, bool) {
	jc, ok := ctx.Value(JourneyContextKey).(*JourneyContext)
	return jc, ok
}

package journeys

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/workflow"
)

// Grievance Journey Definitions (Journeys 11-13)
// These journeys map to existing implemented components:
// - GrievanceSubmissionWorkflow, GrievanceResolutionWorkflow, GrievanceEscalationWorkflow
// - UI: Grievances page
// - BFF: grievances router

// Journey 11: Grievance Submission
// UI Entry: Admin Grievances page, Mobile (beneficiary portal)
// BFF: trpc.grievances.create
// Workflow: GrievanceSubmissionWorkflow
type GrievanceSubmissionInput struct {
	JourneyContext *JourneyContext `json:"journeyContext"`
	BeneficiaryID  string          `json:"beneficiaryId"`
	Category       string          `json:"category"` // "payment", "enrollment", "service", "fraud", "other"
	Priority       string          `json:"priority"` // "low", "medium", "high", "critical"
	Subject        string          `json:"subject"`
	Description    string          `json:"description"`
	Attachments    []string        `json:"attachments,omitempty"`
	ContactMethod  string          `json:"contactMethod"` // "phone", "email", "sms", "in_person"
}

type GrievanceSubmissionResult struct {
	JourneyRunID  string    `json:"journeyRunId"`
	GrievanceID   string    `json:"grievanceId"`
	CaseNumber    string    `json:"caseNumber"`
	Status        string    `json:"status"`
	AssignedTo    string    `json:"assignedTo,omitempty"`
	SLADeadline   time.Time `json:"slaDeadline"`
	SubmittedAt   time.Time `json:"submittedAt"`
}

// GrievanceSubmissionJourney orchestrates grievance submission
func GrievanceSubmissionJourney(ctx workflow.Context, input GrievanceSubmissionInput) (*GrievanceSubmissionResult, error) {
	jc := input.JourneyContext
	
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
		RetryPolicy: &workflow.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    time.Minute,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)
	
	// Step 1: Emit journey started event
	workflow.ExecuteActivity(ctx, EmitJourneyEventActivity, jc, "journey.started", map[string]interface{}{
		"journeyType":   "grievance_submission",
		"beneficiaryId": input.BeneficiaryID,
		"category":      input.Category,
	})
	
	// Step 2: Check authorization
	var authorized bool
	err := workflow.ExecuteActivity(ctx, CheckAuthorizationActivity, jc, "grievance:create").Get(ctx, &authorized)
	if err != nil || !authorized {
		return nil, fmt.Errorf("authorization failed")
	}
	
	// Step 3: Verify beneficiary exists
	var beneficiaryExists bool
	err = workflow.ExecuteActivity(ctx, CheckBeneficiaryExistsActivity, input.BeneficiaryID).Get(ctx, &beneficiaryExists)
	if err != nil || !beneficiaryExists {
		return nil, fmt.Errorf("beneficiary not found")
	}
	
	// Step 4: Generate case number
	var caseNumber string
	err = workflow.ExecuteActivity(ctx, GenerateCaseNumberActivity, input.Category).Get(ctx, &caseNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to generate case number: %v", err)
	}
	
	// Step 5: Calculate SLA deadline based on priority
	var slaDeadline time.Time
	switch input.Priority {
	case "critical":
		slaDeadline = time.Now().Add(4 * time.Hour)
	case "high":
		slaDeadline = time.Now().Add(24 * time.Hour)
	case "medium":
		slaDeadline = time.Now().Add(72 * time.Hour)
	default:
		slaDeadline = time.Now().Add(7 * 24 * time.Hour)
	}
	
	// Step 6: Create grievance record
	var grievanceID string
	err = workflow.ExecuteActivity(ctx, CreateGrievanceRecordActivity, map[string]interface{}{
		"beneficiaryId": input.BeneficiaryID,
		"caseNumber":    caseNumber,
		"category":      input.Category,
		"priority":      input.Priority,
		"subject":       input.Subject,
		"description":   input.Description,
		"attachments":   input.Attachments,
		"contactMethod": input.ContactMethod,
		"status":        "open",
		"slaDeadline":   slaDeadline,
		"createdBy":     jc.ActorID,
		"tenantId":      jc.TenantID,
	}).Get(ctx, &grievanceID)
	if err != nil {
		return nil, fmt.Errorf("failed to create grievance: %v", err)
	}
	
	// Step 7: Auto-assign based on category and priority
	var assignedTo string
	err = workflow.ExecuteActivity(ctx, AutoAssignGrievanceActivity, grievanceID, input.Category, input.Priority).Get(ctx, &assignedTo)
	if err != nil {
		workflow.GetLogger(ctx).Warn("Failed to auto-assign grievance", "error", err)
	}
	
	// Step 8: Send acknowledgment notification
	workflow.ExecuteActivity(ctx, SendGrievanceAcknowledgmentActivity, input.BeneficiaryID, caseNumber, input.ContactMethod)
	
	// Step 9: Publish Kafka event
	workflow.ExecuteActivity(ctx, PublishKafkaEventActivity, "grievance.submitted", map[string]interface{}{
		"grievanceId":   grievanceID,
		"caseNumber":    caseNumber,
		"beneficiaryId": input.BeneficiaryID,
		"category":      input.Category,
		"priority":      input.Priority,
		"assignedTo":    assignedTo,
		"correlationId": jc.CorrelationID,
	})
	
	// Step 10: Create audit log
	workflow.ExecuteActivity(ctx, CreateAuditLogActivity, map[string]interface{}{
		"action":        "grievance_submitted",
		"entityType":    "grievance",
		"entityId":      grievanceID,
		"performedBy":   jc.ActorID,
		"tenantId":      jc.TenantID,
		"correlationId": jc.CorrelationID,
		"details": map[string]interface{}{
			"caseNumber":    caseNumber,
			"beneficiaryId": input.BeneficiaryID,
			"category":      input.Category,
			"priority":      input.Priority,
		},
	})
	
	// Step 11: Write to lakehouse
	workflow.ExecuteActivity(ctx, WriteLakehouseFactActivity, "grievance_facts", map[string]interface{}{
		"grievanceId":    grievanceID,
		"submissionDate": time.Now().Format("2006-01-02"),
		"beneficiaryId":  input.BeneficiaryID,
		"category":       input.Category,
		"priority":       input.Priority,
		"tenantId":       jc.TenantID,
	})
	
	// Step 12: Emit journey completed event
	workflow.ExecuteActivity(ctx, EmitJourneyEventActivity, jc, "journey.completed", map[string]interface{}{
		"grievanceId": grievanceID,
		"caseNumber":  caseNumber,
	})
	
	return &GrievanceSubmissionResult{
		JourneyRunID: jc.JourneyRunID,
		GrievanceID:  grievanceID,
		CaseNumber:   caseNumber,
		Status:       "open",
		AssignedTo:   assignedTo,
		SLADeadline:  slaDeadline,
		SubmittedAt:  time.Now(),
	}, nil
}

// Journey 12: Grievance Resolution
// UI Entry: Admin Grievance detail page
// BFF: trpc.grievances.resolve
// Workflow: GrievanceResolutionWorkflow
type GrievanceResolutionInput struct {
	JourneyContext *JourneyContext `json:"journeyContext"`
	GrievanceID    string          `json:"grievanceId"`
	Resolution     string          `json:"resolution"` // "resolved", "rejected", "duplicate", "withdrawn"
	ResolutionNotes string         `json:"resolutionNotes"`
	ActionsTaken   []string        `json:"actionsTaken,omitempty"`
	Compensation   float64         `json:"compensation,omitempty"`
}

type GrievanceResolutionResult struct {
	JourneyRunID  string    `json:"journeyRunId"`
	GrievanceID   string    `json:"grievanceId"`
	Resolution    string    `json:"resolution"`
	ResolvedAt    time.Time `json:"resolvedAt"`
	ResolutionTime time.Duration `json:"resolutionTime"`
	WithinSLA     bool      `json:"withinSla"`
}

// GrievanceResolutionJourney orchestrates grievance resolution
func GrievanceResolutionJourney(ctx workflow.Context, input GrievanceResolutionInput) (*GrievanceResolutionResult, error) {
	jc := input.JourneyContext
	
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Minute,
		RetryPolicy: &workflow.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    time.Minute,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)
	
	// Step 1: Emit journey started event
	workflow.ExecuteActivity(ctx, EmitJourneyEventActivity, jc, "journey.started", map[string]interface{}{
		"journeyType": "grievance_resolution",
		"grievanceId": input.GrievanceID,
	})
	
	// Step 2: Check authorization
	var authorized bool
	err := workflow.ExecuteActivity(ctx, CheckAuthorizationActivity, jc, "grievance:resolve").Get(ctx, &authorized)
	if err != nil || !authorized {
		return nil, fmt.Errorf("authorization failed")
	}
	
	// Step 3: Get grievance details
	var grievance map[string]interface{}
	err = workflow.ExecuteActivity(ctx, GetGrievanceDetailsActivity, input.GrievanceID).Get(ctx, &grievance)
	if err != nil {
		return nil, fmt.Errorf("grievance not found: %v", err)
	}
	
	status := grievance["status"].(string)
	if status == "resolved" || status == "closed" {
		return nil, fmt.Errorf("grievance already resolved")
	}
	
	// Step 4: Calculate resolution time and SLA compliance
	createdAt := grievance["createdAt"].(time.Time)
	slaDeadline := grievance["slaDeadline"].(time.Time)
	resolutionTime := time.Since(createdAt)
	withinSLA := time.Now().Before(slaDeadline)
	
	// Step 5: Process compensation if applicable
	if input.Compensation > 0 {
		beneficiaryID := grievance["beneficiaryId"].(string)
		workflow.ExecuteActivity(ctx, ProcessGrievanceCompensationActivity, map[string]interface{}{
			"grievanceId":   input.GrievanceID,
			"beneficiaryId": beneficiaryID,
			"amount":        input.Compensation,
			"reason":        input.ResolutionNotes,
		})
	}
	
	// Step 6: Update grievance status
	err = workflow.ExecuteActivity(ctx, UpdateGrievanceStatusActivity, input.GrievanceID, map[string]interface{}{
		"status":          input.Resolution,
		"resolutionNotes": input.ResolutionNotes,
		"actionsTaken":    input.ActionsTaken,
		"compensation":    input.Compensation,
		"resolvedBy":      jc.ActorID,
		"resolvedAt":      time.Now(),
		"withinSla":       withinSLA,
	}).Get(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to update grievance: %v", err)
	}
	
	// Step 7: Send resolution notification
	beneficiaryID := grievance["beneficiaryId"].(string)
	contactMethod := grievance["contactMethod"].(string)
	workflow.ExecuteActivity(ctx, SendGrievanceResolutionNotificationActivity, beneficiaryID, input.GrievanceID, input.Resolution, contactMethod)
	
	// Step 8: Publish Kafka event
	workflow.ExecuteActivity(ctx, PublishKafkaEventActivity, "grievance.resolved", map[string]interface{}{
		"grievanceId":    input.GrievanceID,
		"resolution":     input.Resolution,
		"resolutionTime": resolutionTime.String(),
		"withinSla":      withinSLA,
		"correlationId":  jc.CorrelationID,
	})
	
	// Step 9: Create audit log
	workflow.ExecuteActivity(ctx, CreateAuditLogActivity, map[string]interface{}{
		"action":        "grievance_resolved",
		"entityType":    "grievance",
		"entityId":      input.GrievanceID,
		"performedBy":   jc.ActorID,
		"tenantId":      jc.TenantID,
		"correlationId": jc.CorrelationID,
		"details": map[string]interface{}{
			"resolution":     input.Resolution,
			"resolutionTime": resolutionTime.String(),
			"withinSla":      withinSLA,
			"compensation":   input.Compensation,
		},
	})
	
	// Step 10: Write to lakehouse
	workflow.ExecuteActivity(ctx, WriteLakehouseFactActivity, "grievance_resolution_facts", map[string]interface{}{
		"grievanceId":      input.GrievanceID,
		"resolutionDate":   time.Now().Format("2006-01-02"),
		"resolution":       input.Resolution,
		"resolutionTimeMs": resolutionTime.Milliseconds(),
		"withinSla":        withinSLA,
		"compensation":     input.Compensation,
		"tenantId":         jc.TenantID,
	})
	
	// Step 11: Emit journey completed event
	workflow.ExecuteActivity(ctx, EmitJourneyEventActivity, jc, "journey.completed", map[string]interface{}{
		"grievanceId": input.GrievanceID,
		"resolution":  input.Resolution,
	})
	
	return &GrievanceResolutionResult{
		JourneyRunID:   jc.JourneyRunID,
		GrievanceID:    input.GrievanceID,
		Resolution:     input.Resolution,
		ResolvedAt:     time.Now(),
		ResolutionTime: resolutionTime,
		WithinSLA:      withinSLA,
	}, nil
}

// Journey 13: Grievance Escalation
// UI Entry: Admin Grievance detail page (Escalate button)
// BFF: trpc.grievances.escalate
// Workflow: GrievanceEscalationWorkflow
type GrievanceEscalationInput struct {
	JourneyContext  *JourneyContext `json:"journeyContext"`
	GrievanceID     string          `json:"grievanceId"`
	EscalationLevel int             `json:"escalationLevel"` // 1, 2, 3
	Reason          string          `json:"reason"`
	AssignTo        string          `json:"assignTo,omitempty"`
}

type GrievanceEscalationResult struct {
	JourneyRunID    string    `json:"journeyRunId"`
	GrievanceID     string    `json:"grievanceId"`
	EscalationLevel int       `json:"escalationLevel"`
	AssignedTo      string    `json:"assignedTo"`
	NewSLADeadline  time.Time `json:"newSlaDeadline"`
	EscalatedAt     time.Time `json:"escalatedAt"`
}

// GrievanceEscalationJourney orchestrates grievance escalation
func GrievanceEscalationJourney(ctx workflow.Context, input GrievanceEscalationInput) (*GrievanceEscalationResult, error) {
	jc := input.JourneyContext
	
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
		RetryPolicy: &workflow.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    time.Minute,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)
	
	// Step 1: Emit journey started event
	workflow.ExecuteActivity(ctx, EmitJourneyEventActivity, jc, "journey.started", map[string]interface{}{
		"journeyType":     "grievance_escalation",
		"grievanceId":     input.GrievanceID,
		"escalationLevel": input.EscalationLevel,
	})
	
	// Step 2: Check authorization
	var authorized bool
	err := workflow.ExecuteActivity(ctx, CheckAuthorizationActivity, jc, "grievance:escalate").Get(ctx, &authorized)
	if err != nil || !authorized {
		return nil, fmt.Errorf("authorization failed")
	}
	
	// Step 3: Get grievance details
	var grievance map[string]interface{}
	err = workflow.ExecuteActivity(ctx, GetGrievanceDetailsActivity, input.GrievanceID).Get(ctx, &grievance)
	if err != nil {
		return nil, fmt.Errorf("grievance not found: %v", err)
	}
	
	currentLevel := 0
	if level, ok := grievance["escalationLevel"].(int); ok {
		currentLevel = level
	}
	
	if input.EscalationLevel <= currentLevel {
		return nil, fmt.Errorf("cannot escalate to same or lower level")
	}
	
	// Step 4: Determine new assignee based on escalation level
	var assignedTo string
	if input.AssignTo != "" {
		assignedTo = input.AssignTo
	} else {
		err = workflow.ExecuteActivity(ctx, GetEscalationAssigneeActivity, input.EscalationLevel, grievance["category"]).Get(ctx, &assignedTo)
		if err != nil {
			return nil, fmt.Errorf("failed to find escalation assignee: %v", err)
		}
	}
	
	// Step 5: Calculate new SLA deadline (shorter for higher escalation)
	var newSLADeadline time.Time
	switch input.EscalationLevel {
	case 3:
		newSLADeadline = time.Now().Add(2 * time.Hour)
	case 2:
		newSLADeadline = time.Now().Add(8 * time.Hour)
	default:
		newSLADeadline = time.Now().Add(24 * time.Hour)
	}
	
	// Step 6: Update grievance with escalation
	err = workflow.ExecuteActivity(ctx, UpdateGrievanceEscalationActivity, input.GrievanceID, map[string]interface{}{
		"escalationLevel": input.EscalationLevel,
		"escalationReason": input.Reason,
		"assignedTo":      assignedTo,
		"slaDeadline":     newSLADeadline,
		"escalatedBy":     jc.ActorID,
		"escalatedAt":     time.Now(),
	}).Get(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to escalate grievance: %v", err)
	}
	
	// Step 7: Notify new assignee
	workflow.ExecuteActivity(ctx, SendEscalationNotificationActivity, assignedTo, input.GrievanceID, input.EscalationLevel, input.Reason)
	
	// Step 8: Notify previous assignee
	if previousAssignee, ok := grievance["assignedTo"].(string); ok && previousAssignee != "" {
		workflow.ExecuteActivity(ctx, SendEscalationHandoffNotificationActivity, previousAssignee, input.GrievanceID, assignedTo)
	}
	
	// Step 9: Publish Kafka event
	workflow.ExecuteActivity(ctx, PublishKafkaEventActivity, "grievance.escalated", map[string]interface{}{
		"grievanceId":     input.GrievanceID,
		"escalationLevel": input.EscalationLevel,
		"assignedTo":      assignedTo,
		"reason":          input.Reason,
		"correlationId":   jc.CorrelationID,
	})
	
	// Step 10: Create audit log
	workflow.ExecuteActivity(ctx, CreateAuditLogActivity, map[string]interface{}{
		"action":        "grievance_escalated",
		"entityType":    "grievance",
		"entityId":      input.GrievanceID,
		"performedBy":   jc.ActorID,
		"tenantId":      jc.TenantID,
		"correlationId": jc.CorrelationID,
		"details": map[string]interface{}{
			"escalationLevel": input.EscalationLevel,
			"assignedTo":      assignedTo,
			"reason":          input.Reason,
		},
	})
	
	// Step 11: Write to lakehouse
	workflow.ExecuteActivity(ctx, WriteLakehouseFactActivity, "grievance_escalation_facts", map[string]interface{}{
		"grievanceId":     input.GrievanceID,
		"escalationDate":  time.Now().Format("2006-01-02"),
		"escalationLevel": input.EscalationLevel,
		"reason":          input.Reason,
		"tenantId":        jc.TenantID,
	})
	
	// Step 12: Emit journey completed event
	workflow.ExecuteActivity(ctx, EmitJourneyEventActivity, jc, "journey.completed", map[string]interface{}{
		"grievanceId":     input.GrievanceID,
		"escalationLevel": input.EscalationLevel,
	})
	
	return &GrievanceEscalationResult{
		JourneyRunID:    jc.JourneyRunID,
		GrievanceID:     input.GrievanceID,
		EscalationLevel: input.EscalationLevel,
		AssignedTo:      assignedTo,
		NewSLADeadline:  newSLADeadline,
		EscalatedAt:     time.Now(),
	}, nil
}

// RegisterGrievanceJourneys registers all grievance journey definitions
func RegisterGrievanceJourneys(registry *JourneyRegistry) {
	registry.Register(&JourneyDefinition{
		Key:          "grievance_submission",
		Name:         "Grievance Submission",
		Description:  "Submit a new grievance with auto-assignment and SLA tracking",
		Category:     "grievance",
		WorkflowType: "GrievanceSubmissionJourney",
		RequiredPermissions: []string{"grievance:create"},
		UIEntryPoints:       []string{"Admin:GrievancesPage", "Mobile:BeneficiaryPortal"},
		BFFEndpoints:        []string{"trpc.grievances.create"},
		MiddlewareHooks:     []string{"kafka", "redis", "permify", "lakehouse"},
	})
	
	registry.Register(&JourneyDefinition{
		Key:          "grievance_resolution",
		Name:         "Grievance Resolution",
		Description:  "Resolve a grievance with optional compensation",
		Category:     "grievance",
		WorkflowType: "GrievanceResolutionJourney",
		RequiredPermissions: []string{"grievance:resolve"},
		UIEntryPoints:       []string{"Admin:GrievanceDetailPage"},
		BFFEndpoints:        []string{"trpc.grievances.resolve"},
		MiddlewareHooks:     []string{"kafka", "redis", "permify", "tigerbeetle", "lakehouse"},
	})
	
	registry.Register(&JourneyDefinition{
		Key:          "grievance_escalation",
		Name:         "Grievance Escalation",
		Description:  "Escalate a grievance to higher level with new SLA",
		Category:     "grievance",
		WorkflowType: "GrievanceEscalationJourney",
		RequiredPermissions: []string{"grievance:escalate"},
		UIEntryPoints:       []string{"Admin:GrievanceDetailPage"},
		BFFEndpoints:        []string{"trpc.grievances.escalate"},
		MiddlewareHooks:     []string{"kafka", "redis", "permify", "lakehouse"},
	})
}

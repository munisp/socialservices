// Package openlane provides integration with OpenLane GRC platform
// for compliance automation, evidence collection, and audit management
package openlane

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

// Client handles communication with OpenLane API
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	orgID      string
}

// Evidence represents compliance evidence to be submitted to OpenLane
type Evidence struct {
	ID          string                 `json:"id,omitempty"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Type        string                 `json:"type"`
	ControlID   string                 `json:"control_id,omitempty"`
	Source      string                 `json:"source"`
	CollectedAt time.Time              `json:"collected_at"`
	Data        map[string]interface{} `json:"data"`
	Attachments []Attachment           `json:"attachments,omitempty"`
}

// Attachment represents a file attachment for evidence
type Attachment struct {
	Name        string `json:"name"`
	ContentType string `json:"content_type"`
	URL         string `json:"url"`
	Size        int64  `json:"size"`
}

// Task represents a compliance task in OpenLane
type Task struct {
	ID          string    `json:"id,omitempty"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Type        string    `json:"type"`
	Priority    string    `json:"priority"`
	DueDate     time.Time `json:"due_date,omitempty"`
	AssigneeID  string    `json:"assignee_id,omitempty"`
	ControlID   string    `json:"control_id,omitempty"`
	Status      string    `json:"status,omitempty"`
}

// Control represents a compliance control
type Control struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Standard    string `json:"standard"`
	Category    string `json:"category"`
	Status      string `json:"status"`
}

// Risk represents a risk assessment
type Risk struct {
	ID          string                 `json:"id,omitempty"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Category    string                 `json:"category"`
	Likelihood  string                 `json:"likelihood"`
	Impact      string                 `json:"impact"`
	Score       int                    `json:"score"`
	Mitigation  string                 `json:"mitigation,omitempty"`
	ControlIDs  []string               `json:"control_ids,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// PlatformEvent represents an event from the social protection platform
type PlatformEvent struct {
	EventType   string                 `json:"event_type"`
	Timestamp   time.Time              `json:"timestamp"`
	Source      string                 `json:"source"`
	Actor       string                 `json:"actor,omitempty"`
	Resource    string                 `json:"resource,omitempty"`
	ResourceID  string                 `json:"resource_id,omitempty"`
	Action      string                 `json:"action"`
	Outcome     string                 `json:"outcome"`
	Details     map[string]interface{} `json:"details,omitempty"`
}

// NewClient creates a new OpenLane client
func NewClient() (*Client, error) {
	baseURL := os.Getenv("OPENLANE_API_URL")
	if baseURL == "" {
		baseURL = "http://openlane:17608"
	}

	apiKey := os.Getenv("OPENLANE_API_KEY")
	if apiKey == "" {
		// Try to read from file (Kubernetes secret)
		keyBytes, err := os.ReadFile("/var/run/secrets/openlane/api-key")
		if err == nil {
			apiKey = string(keyBytes)
		}
	}

	orgID := os.Getenv("OPENLANE_ORG_ID")

	return &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		orgID:   orgID,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

// SubmitEvidence submits compliance evidence to OpenLane
func (c *Client) SubmitEvidence(ctx context.Context, evidence *Evidence) (*Evidence, error) {
	evidence.Source = "social-protection-platform"
	if evidence.CollectedAt.IsZero() {
		evidence.CollectedAt = time.Now()
	}

	body, err := json.Marshal(evidence)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal evidence: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/v1/evidence", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var result Evidence
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// CreateTask creates a compliance task in OpenLane
func (c *Client) CreateTask(ctx context.Context, task *Task) (*Task, error) {
	body, err := json.Marshal(task)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal task: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/v1/tasks", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var result Task
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// CreateRisk creates a risk assessment in OpenLane
func (c *Client) CreateRisk(ctx context.Context, risk *Risk) (*Risk, error) {
	body, err := json.Marshal(risk)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal risk: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/v1/risks", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var result Risk
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// GetControls retrieves compliance controls from OpenLane
func (c *Client) GetControls(ctx context.Context, standard string) ([]Control, error) {
	url := c.baseURL + "/api/v1/controls"
	if standard != "" {
		url += "?standard=" + standard
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var result struct {
		Controls []Control `json:"controls"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Controls, nil
}

func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	if c.orgID != "" {
		req.Header.Set("X-Organization-ID", c.orgID)
	}
}

// EventMapping maps platform events to OpenLane evidence/tasks
var EventMapping = map[string]struct {
	EvidenceType string
	ControlIDs   []string
	CreateTask   bool
	TaskType     string
	Priority     string
}{
	"beneficiary.enrolled": {
		EvidenceType: "enrollment_record",
		ControlIDs:   []string{"AC-2", "IA-2"},
		CreateTask:   false,
	},
	"beneficiary.kyc_verified": {
		EvidenceType: "identity_verification",
		ControlIDs:   []string{"IA-2", "IA-5"},
		CreateTask:   false,
	},
	"disbursement.approved": {
		EvidenceType: "payment_approval",
		ControlIDs:   []string{"AC-3", "AU-2"},
		CreateTask:   false,
	},
	"disbursement.completed": {
		EvidenceType: "payment_record",
		ControlIDs:   []string{"AU-2", "AU-3"},
		CreateTask:   false,
	},
	"disbursement.failed": {
		EvidenceType: "payment_failure",
		ControlIDs:   []string{"AU-2", "IR-4"},
		CreateTask:   true,
		TaskType:     "incident_review",
		Priority:     "high",
	},
	"grievance.submitted": {
		EvidenceType: "grievance_record",
		ControlIDs:   []string{"IR-4", "IR-5"},
		CreateTask:   true,
		TaskType:     "grievance_review",
		Priority:     "medium",
	},
	"grievance.resolved": {
		EvidenceType: "grievance_resolution",
		ControlIDs:   []string{"IR-4", "IR-5"},
		CreateTask:   false,
	},
	"fraud.detected": {
		EvidenceType: "fraud_alert",
		ControlIDs:   []string{"SI-4", "IR-4"},
		CreateTask:   true,
		TaskType:     "fraud_investigation",
		Priority:     "critical",
	},
	"audit.access_granted": {
		EvidenceType: "access_log",
		ControlIDs:   []string{"AC-2", "AC-3"},
		CreateTask:   false,
	},
	"audit.data_exported": {
		EvidenceType: "data_export_log",
		ControlIDs:   []string{"AU-2", "AU-3"},
		CreateTask:   false,
	},
	"system.backup_completed": {
		EvidenceType: "backup_record",
		ControlIDs:   []string{"CP-9", "CP-10"},
		CreateTask:   false,
	},
	"security.vulnerability_scan": {
		EvidenceType: "vulnerability_scan",
		ControlIDs:   []string{"RA-5", "SI-2"},
		CreateTask:   true,
		TaskType:     "vulnerability_review",
		Priority:     "medium",
	},
}

// ProcessPlatformEvent processes a platform event and creates evidence/tasks in OpenLane
func (c *Client) ProcessPlatformEvent(ctx context.Context, event *PlatformEvent) error {
	mapping, ok := EventMapping[event.EventType]
	if !ok {
		// Unknown event type, skip
		return nil
	}

	// Create evidence
	evidence := &Evidence{
		Name:        fmt.Sprintf("%s - %s", event.EventType, event.ResourceID),
		Description: fmt.Sprintf("Automated evidence from %s event", event.EventType),
		Type:        mapping.EvidenceType,
		Source:      event.Source,
		CollectedAt: event.Timestamp,
		Data: map[string]interface{}{
			"event_type":  event.EventType,
			"actor":       event.Actor,
			"resource":    event.Resource,
			"resource_id": event.ResourceID,
			"action":      event.Action,
			"outcome":     event.Outcome,
			"details":     event.Details,
		},
	}

	if len(mapping.ControlIDs) > 0 {
		evidence.ControlID = mapping.ControlIDs[0]
	}

	if _, err := c.SubmitEvidence(ctx, evidence); err != nil {
		return fmt.Errorf("failed to submit evidence: %w", err)
	}

	// Create task if required
	if mapping.CreateTask {
		task := &Task{
			Title:       fmt.Sprintf("Review: %s - %s", event.EventType, event.ResourceID),
			Description: fmt.Sprintf("Review required for %s event. Resource: %s, Outcome: %s", event.EventType, event.ResourceID, event.Outcome),
			Type:        mapping.TaskType,
			Priority:    mapping.Priority,
			DueDate:     calculateDueDate(mapping.Priority),
		}

		if len(mapping.ControlIDs) > 0 {
			task.ControlID = mapping.ControlIDs[0]
		}

		if _, err := c.CreateTask(ctx, task); err != nil {
			return fmt.Errorf("failed to create task: %w", err)
		}
	}

	return nil
}

func calculateDueDate(priority string) time.Time {
	now := time.Now()
	switch priority {
	case "critical":
		return now.Add(24 * time.Hour)
	case "high":
		return now.Add(3 * 24 * time.Hour)
	case "medium":
		return now.Add(7 * 24 * time.Hour)
	default:
		return now.Add(14 * 24 * time.Hour)
	}
}

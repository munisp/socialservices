package analytics

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// DeltaLakeClient is a Go client for the Python Delta Lake service
type DeltaLakeClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewDeltaLakeClient creates a new Delta Lake client
func NewDeltaLakeClient(baseURL string) *DeltaLakeClient {
	return &DeltaLakeClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Schema represents a Delta Lake table schema
type Schema struct {
	Fields []SchemaField `json:"fields"`
}

// SchemaField represents a field in the schema
type SchemaField struct {
	Name string `json:"name"`
	Type string `json:"type"` // int64, float64, string, bool, timestamp
}

// CreateTableRequest represents a request to create a table
type CreateTableRequest struct {
	TablePath   string   `json:"table_path"`
	Schema      Schema   `json:"schema"`
	PartitionBy []string `json:"partition_by,omitempty"`
}

// WriteDataRequest represents a request to write data
type WriteDataRequest struct {
	TablePath string                   `json:"table_path"`
	Data      []map[string]interface{} `json:"data"`
	Mode      string                   `json:"mode"` // append, overwrite, error
}

// ReadDataRequest represents a request to read data
type ReadDataRequest struct {
	TablePath string   `json:"table_path"`
	Filters   []string `json:"filters,omitempty"`
	Columns   []string `json:"columns,omitempty"`
}

// TimeTravelRequest represents a request for time travel
type TimeTravelRequest struct {
	TablePath string `json:"table_path"`
	Version   *int   `json:"version,omitempty"`
	Timestamp string `json:"timestamp,omitempty"`
}

// OptimizeRequest represents a request to optimize a table
type OptimizeRequest struct{
	TablePath  string `json:"table_path"`
	TargetSize int    `json:"target_size,omitempty"`
}

// ZOrderRequest represents a request to z-order a table
type ZOrderRequest struct {
	TablePath string   `json:"table_path"`
	Columns   []string `json:"columns"`
}

// VacuumRequest represents a request to vacuum a table
type VacuumRequest struct {
	TablePath      string `json:"table_path"`
	RetentionHours int    `json:"retention_hours,omitempty"`
	DryRun         bool   `json:"dry_run,omitempty"`
}

// MergeRequest represents a request to merge data
type MergeRequest struct {
	TablePath    string                   `json:"table_path"`
	SourceData   []map[string]interface{} `json:"source_data"`
	Predicate    string                   `json:"predicate"`
	UpdateSet    map[string]string        `json:"update_set"`
	InsertValues map[string]string        `json:"insert_values"`
}

// Response represents a generic response
type Response struct {
	Success bool                   `json:"success"`
	Error   string                 `json:"error,omitempty"`
	Data    map[string]interface{} `json:"data,omitempty"`
}

// CreateTable creates a new Delta Lake table
func (c *DeltaLakeClient) CreateTable(req CreateTableRequest) (*Response, error) {
	return c.post("/tables/create", req)
}

// WriteData writes data to a Delta Lake table
func (c *DeltaLakeClient) WriteData(req WriteDataRequest) (*Response, error) {
	return c.post("/tables/write", req)
}

// ReadData reads data from a Delta Lake table
func (c *DeltaLakeClient) ReadData(req ReadDataRequest) (*Response, error) {
	return c.post("/tables/read", req)
}

// TimeTravel queries a historical version of a table
func (c *DeltaLakeClient) TimeTravel(req TimeTravelRequest) (*Response, error) {
	return c.post("/tables/time_travel", req)
}

// Optimize optimizes a table by compacting small files
func (c *DeltaLakeClient) Optimize(req OptimizeRequest) (*Response, error) {
	return c.post("/tables/optimize", req)
}

// ZOrder z-orders a table by specified columns
func (c *DeltaLakeClient) ZOrder(req ZOrderRequest) (*Response, error) {
	return c.post("/tables/z_order", req)
}

// Vacuum removes old files based on retention policy
func (c *DeltaLakeClient) Vacuum(req VacuumRequest) (*Response, error) {
	return c.post("/tables/vacuum", req)
}

// GetHistory gets table history
func (c *DeltaLakeClient) GetHistory(tablePath string, limit int) (*Response, error) {
	return c.post("/tables/history", map[string]interface{}{
		"table_path": tablePath,
		"limit":      limit,
	})
}

// GetSchema gets table schema
func (c *DeltaLakeClient) GetSchema(tablePath string) (*Response, error) {
	return c.post("/tables/schema", map[string]interface{}{
		"table_path": tablePath,
	})
}

// Merge performs an upsert operation
func (c *DeltaLakeClient) Merge(req MergeRequest) (*Response, error) {
	return c.post("/tables/merge", req)
}

// Health checks if the service is healthy
func (c *DeltaLakeClient) Health() error {
	resp, err := c.httpClient.Get(c.baseURL + "/health")
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check failed with status %d", resp.StatusCode)
	}
	
	return nil
}

// post makes a POST request to the service
func (c *DeltaLakeClient) post(endpoint string, payload interface{}) (*Response, error) {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	
	resp, err := c.httpClient.Post(
		c.baseURL+endpoint,
		"application/json",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	
	var result Response
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	
	if !result.Success {
		return &result, fmt.Errorf("operation failed: %s", result.Error)
	}
	
	return &result, nil
}

// Example usage functions

// CreateBeneficiariesTable creates the beneficiaries table in Delta Lake
func CreateBeneficiariesTable(client *DeltaLakeClient) error {
	schema := Schema{
		Fields: []SchemaField{
			{Name: "beneficiary_id", Type: "string"},
			{Name: "name", Type: "string"},
			{Name: "email", Type: "string"},
			{Name: "phone", Type: "string"},
			{Name: "location", Type: "string"},
			{Name: "latitude", Type: "float64"},
			{Name: "longitude", Type: "float64"},
			{Name: "program_id", Type: "string"},
			{Name: "enrollment_date", Type: "timestamp"},
			{Name: "status", Type: "string"},
			{Name: "created_at", Type: "timestamp"},
			{Name: "updated_at", Type: "timestamp"},
		},
	}
	
	_, err := client.CreateTable(CreateTableRequest{
		TablePath:   "silver/beneficiaries",
		Schema:      schema,
		PartitionBy: []string{"program_id"},
	})
	
	return err
}

// CreateDisbursementsTable creates the disbursements table in Delta Lake
func CreateDisbursementsTable(client *DeltaLakeClient) error {
	schema := Schema{
		Fields: []SchemaField{
			{Name: "disbursement_id", Type: "string"},
			{Name: "beneficiary_id", Type: "string"},
			{Name: "program_id", Type: "string"},
			{Name: "amount", Type: "float64"},
			{Name: "currency", Type: "string"},
			{Name: "disbursement_date", Type: "timestamp"},
			{Name: "payment_method", Type: "string"},
			{Name: "status", Type: "string"},
			{Name: "transaction_ref", Type: "string"},
			{Name: "created_at", Type: "timestamp"},
		},
	}
	
	_, err := client.CreateTable(CreateTableRequest{
		TablePath:   "silver/disbursements",
		Schema:      schema,
		PartitionBy: []string{"program_id", "disbursement_date"},
	})
	
	return err
}

// WriteBeneficiary writes a beneficiary record to Delta Lake
func WriteBeneficiary(client *DeltaLakeClient, beneficiary map[string]interface{}) error {
	_, err := client.WriteData(WriteDataRequest{
		TablePath: "silver/beneficiaries",
		Data:      []map[string]interface{}{beneficiary},
		Mode:      "append",
	})
	
	return err
}

// UpsertBeneficiary upserts a beneficiary record
func UpsertBeneficiary(client *DeltaLakeClient, beneficiary map[string]interface{}) error {
	_, err := client.Merge(MergeRequest{
		TablePath:  "silver/beneficiaries",
		SourceData: []map[string]interface{}{beneficiary},
		Predicate:  "target.beneficiary_id = source.beneficiary_id",
		UpdateSet: map[string]string{
			"name":       "source.name",
			"email":      "source.email",
			"phone":      "source.phone",
			"location":   "source.location",
			"latitude":   "source.latitude",
			"longitude":  "source.longitude",
			"status":     "source.status",
			"updated_at": "source.updated_at",
		},
		InsertValues: map[string]string{
			"beneficiary_id":  "source.beneficiary_id",
			"name":            "source.name",
			"email":           "source.email",
			"phone":           "source.phone",
			"location":        "source.location",
			"latitude":        "source.latitude",
			"longitude":       "source.longitude",
			"program_id":      "source.program_id",
			"enrollment_date": "source.enrollment_date",
			"status":          "source.status",
			"created_at":      "source.created_at",
			"updated_at":      "source.updated_at",
		},
	})
	
	return err
}

// OptimizeBeneficiariesTable optimizes the beneficiaries table
func OptimizeBeneficiariesTable(client *DeltaLakeClient) error {
	// Compact small files
	_, err := client.Optimize(OptimizeRequest{
		TablePath:  "silver/beneficiaries",
		TargetSize: 134217728, // 128 MB
	})
	if err != nil {
		return err
	}
	
	// Z-order by beneficiary_id and program_id for better query performance
	_, err = client.ZOrder(ZOrderRequest{
		TablePath: "silver/beneficiaries",
		Columns:   []string{"beneficiary_id", "program_id"},
	})
	
	return err
}

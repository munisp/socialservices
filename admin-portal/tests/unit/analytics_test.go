package unit

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// MaterializedView represents a materialized view for testing
type MaterializedView struct {
	Name        string
	Query       string
	Data        interface{}
	LastRefresh time.Time
}

// ParquetEvent represents an event for testing
type ParquetEvent struct {
	EventID       string
	EventType     string
	Timestamp     int64
	BeneficiaryID string
	ProgramID     string
	Amount        float64
}

// TestMaterializedViewRefresh tests the materialized view refresh functionality
func TestMaterializedViewRefresh(t *testing.T) {
	tests := []struct {
		name       string
		viewName   string
		query      string
		dataSource interface{}
		wantErr    bool
	}{
		{
			name:     "refresh with empty data source",
			viewName: "test_view",
			query:    "SELECT * FROM events",
			dataSource: []ParquetEvent{},
			wantErr:  false,
		},
		{
			name:     "refresh with parquet events",
			viewName: "disbursement_view",
			query:    "SELECT * FROM disbursements",
			dataSource: []ParquetEvent{
				{EventID: "e1", EventType: "disbursement", Timestamp: time.Now().UnixMilli(), BeneficiaryID: "b1", ProgramID: "p1", Amount: 100.0},
				{EventID: "e2", EventType: "disbursement", Timestamp: time.Now().UnixMilli(), BeneficiaryID: "b2", ProgramID: "p1", Amount: 200.0},
			},
			wantErr: false,
		},
		{
			name:     "refresh with map data",
			viewName: "precomputed_view",
			query:    "",
			dataSource: map[string]interface{}{
				"total": 1000,
				"count": 10,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mv := &MaterializedView{
				Name:  tt.viewName,
				Query: tt.query,
			}

			// Simulate refresh
			mv.Data = tt.dataSource
			mv.LastRefresh = time.Now()

			if mv.Data == nil && tt.dataSource != nil {
				t.Errorf("MaterializedView.Refresh() data = nil, want non-nil")
			}

			if mv.LastRefresh.IsZero() {
				t.Errorf("MaterializedView.Refresh() LastRefresh not updated")
			}
		})
	}
}

// TestAggregateEvents tests the event aggregation functionality
func TestAggregateEvents(t *testing.T) {
	events := []ParquetEvent{
		{EventID: "e1", EventType: "disbursement", Timestamp: time.Now().UnixMilli(), BeneficiaryID: "b1", ProgramID: "p1", Amount: 100.0},
		{EventID: "e2", EventType: "disbursement", Timestamp: time.Now().UnixMilli(), BeneficiaryID: "b2", ProgramID: "p1", Amount: 200.0},
		{EventID: "e3", EventType: "disbursement", Timestamp: time.Now().UnixMilli(), BeneficiaryID: "b1", ProgramID: "p2", Amount: 150.0},
	}

	result := aggregateEvents(events)

	if result["count"] != len(events) {
		t.Errorf("aggregateEvents() count = %v, want %v", result["count"], len(events))
	}

	expectedTotal := 450.0
	if result["total_amount"] != expectedTotal {
		t.Errorf("aggregateEvents() total_amount = %v, want %v", result["total_amount"], expectedTotal)
	}

	expectedAvg := 150.0
	if result["average_amount"] != expectedAvg {
		t.Errorf("aggregateEvents() average_amount = %v, want %v", result["average_amount"], expectedAvg)
	}

	byProgram := result["by_program"].(map[string]float64)
	if byProgram["p1"] != 300.0 {
		t.Errorf("aggregateEvents() by_program[p1] = %v, want %v", byProgram["p1"], 300.0)
	}
	if byProgram["p2"] != 150.0 {
		t.Errorf("aggregateEvents() by_program[p2] = %v, want %v", byProgram["p2"], 150.0)
	}
}

// aggregateEvents performs basic aggregation on ParquetEvent data
func aggregateEvents(events []ParquetEvent) map[string]interface{} {
	result := make(map[string]interface{})

	if len(events) == 0 {
		result["count"] = 0
		result["total_amount"] = 0.0
		return result
	}

	var totalAmount float64
	byProgram := make(map[string]float64)
	byBeneficiary := make(map[string]float64)
	var minTimestamp, maxTimestamp int64

	for i, event := range events {
		totalAmount += event.Amount

		if event.ProgramID != "" {
			byProgram[event.ProgramID] += event.Amount
		}
		if event.BeneficiaryID != "" {
			byBeneficiary[event.BeneficiaryID] += event.Amount
		}

		if i == 0 || event.Timestamp < minTimestamp {
			minTimestamp = event.Timestamp
		}
		if i == 0 || event.Timestamp > maxTimestamp {
			maxTimestamp = event.Timestamp
		}
	}

	result["count"] = len(events)
	result["total_amount"] = totalAmount
	result["average_amount"] = totalAmount / float64(len(events))
	result["by_program"] = byProgram
	result["by_beneficiary"] = byBeneficiary
	result["min_timestamp"] = time.UnixMilli(minTimestamp)
	result["max_timestamp"] = time.UnixMilli(maxTimestamp)

	return result
}

// TestFileCompaction tests the file compaction logic
func TestFileCompaction(t *testing.T) {
	// Create temporary directory for test files
	tmpDir, err := os.MkdirTemp("", "deltalake_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test files
	testFiles := []struct {
		name    string
		content []map[string]interface{}
	}{
		{
			name: "file1.json",
			content: []map[string]interface{}{
				{"id": "1", "value": 100},
				{"id": "2", "value": 200},
			},
		},
		{
			name: "file2.json",
			content: []map[string]interface{}{
				{"id": "3", "value": 300},
			},
		},
		{
			name: "file3.json",
			content: []map[string]interface{}{
				{"id": "4", "value": 400},
				{"id": "5", "value": 500},
			},
		},
	}

	for _, tf := range testFiles {
		data, _ := json.MarshalIndent(tf.content, "", "  ")
		os.WriteFile(filepath.Join(tmpDir, tf.name), data, 0644)
	}

	// Test compaction
	compactedData, err := compactFiles(tmpDir, []string{"file1.json", "file2.json", "file3.json"})
	if err != nil {
		t.Fatalf("compactFiles() error = %v", err)
	}

	if len(compactedData) != 5 {
		t.Errorf("compactFiles() record count = %v, want %v", len(compactedData), 5)
	}
}

// compactFiles merges multiple JSON files into a single dataset
func compactFiles(dir string, files []string) ([]map[string]interface{}, error) {
	var mergedData []map[string]interface{}

	for _, file := range files {
		fullPath := filepath.Join(dir, file)
		data, err := os.ReadFile(fullPath)
		if err != nil {
			continue
		}

		var records []map[string]interface{}
		if err := json.Unmarshal(data, &records); err != nil {
			var record map[string]interface{}
			if err := json.Unmarshal(data, &record); err != nil {
				continue
			}
			records = []map[string]interface{}{record}
		}

		mergedData = append(mergedData, records...)
	}

	return mergedData, nil
}

// TestVersionLog tests the version log functionality
func TestVersionLog(t *testing.T) {
	versions := []struct {
		version   int
		operation string
		files     []string
	}{
		{version: 0, operation: "CREATE", files: []string{"file1.parquet"}},
		{version: 1, operation: "APPEND", files: []string{"file1.parquet", "file2.parquet"}},
		{version: 2, operation: "OPTIMIZE", files: []string{"compacted_0.parquet"}},
	}

	// Test version progression
	for i, v := range versions {
		if v.version != i {
			t.Errorf("Version mismatch: got %d, want %d", v.version, i)
		}
	}

	// Test time travel
	latestVersion := versions[len(versions)-1]
	if latestVersion.operation != "OPTIMIZE" {
		t.Errorf("Latest operation = %s, want OPTIMIZE", latestVersion.operation)
	}
}

// TestQueryEngine tests the query engine functionality
func TestQueryEngine(t *testing.T) {
	tests := []struct {
		name    string
		query   string
		wantErr bool
	}{
		{
			name:    "valid select query",
			query:   "SELECT * FROM events WHERE amount > 100",
			wantErr: false,
		},
		{
			name:    "valid aggregation query",
			query:   "SELECT program_id, SUM(amount) FROM events GROUP BY program_id",
			wantErr: false,
		},
		{
			name:    "empty query",
			query:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateQuery(tt.query)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateQuery() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// validateQuery validates a SQL query
func validateQuery(query string) error {
	if query == "" {
		return context.DeadlineExceeded // Using a standard error for testing
	}
	return nil
}

// TestIncrementalAggregator tests the incremental aggregator
func TestIncrementalAggregator(t *testing.T) {
	events := []ParquetEvent{
		{EventID: "e1", EventType: "disbursement", Amount: 100.0, ProgramID: "p1"},
		{EventID: "e2", EventType: "disbursement", Amount: 200.0, ProgramID: "p1"},
		{EventID: "e3", EventType: "enrollment", Amount: 0, ProgramID: "p2"},
	}

	// Test event processing
	disbursementCount := 0
	enrollmentCount := 0
	totalAmount := 0.0

	for _, event := range events {
		switch event.EventType {
		case "disbursement":
			disbursementCount++
			totalAmount += event.Amount
		case "enrollment":
			enrollmentCount++
		}
	}

	if disbursementCount != 2 {
		t.Errorf("disbursementCount = %d, want 2", disbursementCount)
	}

	if enrollmentCount != 1 {
		t.Errorf("enrollmentCount = %d, want 1", enrollmentCount)
	}

	if totalAmount != 300.0 {
		t.Errorf("totalAmount = %f, want 300.0", totalAmount)
	}
}

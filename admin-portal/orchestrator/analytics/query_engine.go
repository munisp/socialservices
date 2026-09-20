package analytics

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3" // SQLite driver for DuckDB-like functionality
)

// QueryEngine provides SQL query capabilities over Parquet files
type QueryEngine struct {
	db         *sql.DB
	dataPath   string
	tableCache map[string]*TableMetadata
	mu         sync.RWMutex
}

// TableMetadata stores metadata about a table
type TableMetadata struct {
	Name       string
	Schema     map[string]string
	Location   string
	RowCount   int64
	LastUpdate time.Time
}

// DataQualityChecker performs data quality validations
type DataQualityChecker struct {
	rules   []*QualityRule
	results map[string]*QualityResult
	mu      sync.RWMutex
}

// QualityRule defines a data quality rule
type QualityRule struct {
	Name        string
	Type        string // "schema", "null", "range", "uniqueness", "freshness"
	Field       string
	Condition   string
	Threshold   float64
	Severity    string // "error", "warning"
	Description string
}

// QualityResult stores the result of a quality check
type QualityResult struct {
	RuleName       string
	Passed         bool
	Message        string
	ViolationCount int64
	CheckedAt      time.Time
}

// CDCProcessor processes Change Data Capture events
type CDCProcessor struct {
	source      string
	destination string
	transformer func(map[string]interface{}) (map[string]interface{}, error)
	buffer      []CDCEvent
	mu          sync.Mutex
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
}

// CDCEvent represents a change data capture event
type CDCEvent struct {
	Operation string // "insert", "update", "delete"
	Table     string
	Before    map[string]interface{}
	After     map[string]interface{}
	Timestamp time.Time
	LSN       string // Log Sequence Number
}

// NewQueryEngine creates a new query engine
func NewQueryEngine(dataPath string) (*QueryEngine, error) {
	// Create in-memory SQLite database for query processing
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		return nil, fmt.Errorf("failed to create query engine: %w", err)
	}

	qe := &QueryEngine{
		db:         db,
		dataPath:   dataPath,
		tableCache: make(map[string]*TableMetadata),
	}

	return qe, nil
}

// RegisterTable registers a Parquet table for querying
func (qe *QueryEngine) RegisterTable(tableName, location string) error {
	qe.mu.Lock()
	defer qe.mu.Unlock()

	// Scan Parquet files to infer schema
	schema, rowCount, err := qe.inferSchema(location)
	if err != nil {
		return fmt.Errorf("failed to infer schema: %w", err)
	}

	// Create SQLite table
	if err := qe.createTable(tableName, schema); err != nil {
		return fmt.Errorf("failed to create table: %w", err)
	}

	// Load data from Parquet files
	if err := qe.loadData(tableName, location); err != nil {
		return fmt.Errorf("failed to load data: %w", err)
	}

	// Cache metadata
	qe.tableCache[tableName] = &TableMetadata{
		Name:       tableName,
		Schema:     schema,
		Location:   location,
		RowCount:   rowCount,
		LastUpdate: time.Now(),
	}

	log.Printf("[QueryEngine] Registered table %s with %d rows", tableName, rowCount)
	return nil
}

// inferSchema infers schema from Parquet files
func (qe *QueryEngine) inferSchema(location string) (map[string]string, int64, error) {
	// Simplified schema inference (in production, parse Parquet metadata)
	schema := map[string]string{
		"event_id":        "TEXT",
		"event_type":      "TEXT",
		"timestamp":       "INTEGER",
		"beneficiary_id":  "TEXT",
		"program_id":      "TEXT",
		"amount":          "REAL",
		"metadata_json":   "TEXT",
		"partition_date":  "TEXT",
		"idempotency_key": "TEXT",
	}

	// Count files
	files, err := filepath.Glob(filepath.Join(location, "**/*.parquet"))
	if err != nil {
		return nil, 0, err
	}

	return schema, int64(len(files) * 1000), nil // Estimate 1000 rows per file
}

// createTable creates a SQLite table
func (qe *QueryEngine) createTable(tableName string, schema map[string]string) error {
	var columns []string
	for col, typ := range schema {
		columns = append(columns, fmt.Sprintf("%s %s", col, typ))
	}

	query := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (%s)", tableName, strings.Join(columns, ", "))
	_, err := qe.db.Exec(query)
	return err
}

// loadData loads data from Parquet files into SQLite
func (qe *QueryEngine) loadData(tableName, location string) error {
	return fmt.Errorf("Parquet-to-SQLite loading is unavailable for table %s at %s; use the production lakehouse query service", tableName, location)
}

// ExecuteQuery executes a SQL query
func (qe *QueryEngine) ExecuteQuery(query string) ([]map[string]interface{}, error) {
	rows, err := qe.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("query execution failed: %w", err)
	}
	defer rows.Close()

	// Get column names
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	// Fetch results
	var results []map[string]interface{}
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, err
		}

		row := make(map[string]interface{})
		for i, col := range columns {
			row[col] = values[i]
		}
		results = append(results, row)
	}

	return results, nil
}

// GetTableMetadata returns metadata for a table
func (qe *QueryEngine) GetTableMetadata(tableName string) (*TableMetadata, error) {
	qe.mu.RLock()
	defer qe.mu.RUnlock()

	if metadata, ok := qe.tableCache[tableName]; ok {
		return metadata, nil
	}

	return nil, fmt.Errorf("table not found: %s", tableName)
}

// Close closes the query engine
func (qe *QueryEngine) Close() error {
	return qe.db.Close()
}

// NewDataQualityChecker creates a new data quality checker
func NewDataQualityChecker() *DataQualityChecker {
	return &DataQualityChecker{
		rules:   make([]*QualityRule, 0),
		results: make(map[string]*QualityResult),
	}
}

// AddRule adds a data quality rule
func (dqc *DataQualityChecker) AddRule(rule *QualityRule) {
	dqc.mu.Lock()
	defer dqc.mu.Unlock()
	dqc.rules = append(dqc.rules, rule)
	log.Printf("[DataQuality] Added rule: %s (%s)", rule.Name, rule.Type)
}

// CheckQuality runs all quality checks on the data
func (dqc *DataQualityChecker) CheckQuality(data []ParquetEvent) map[string]*QualityResult {
	dqc.mu.Lock()
	defer dqc.mu.Unlock()

	for _, rule := range dqc.rules {
		result := dqc.checkRule(rule, data)
		dqc.results[rule.Name] = result
	}

	return dqc.results
}

// checkRule checks a single quality rule
func (dqc *DataQualityChecker) checkRule(rule *QualityRule, data []ParquetEvent) *QualityResult {
	result := &QualityResult{
		RuleName:  rule.Name,
		CheckedAt: time.Now(),
	}

	switch rule.Type {
	case "null":
		result = dqc.checkNullValues(rule, data)
	case "range":
		result = dqc.checkRange(rule, data)
	case "schema":
		result = dqc.checkSchema(rule, data)
	case "uniqueness":
		result = dqc.checkUniqueness(rule, data)
	case "freshness":
		result = dqc.checkFreshness(rule, data)
	default:
		result.Passed = false
		result.Message = fmt.Sprintf("Unknown rule type: %s", rule.Type)
	}

	return result
}

// checkNullValues checks for null values
func (dqc *DataQualityChecker) checkNullValues(rule *QualityRule, data []ParquetEvent) *QualityResult {
	nullCount := int64(0)
	for _, event := range data {
		switch rule.Field {
		case "beneficiary_id":
			if event.BeneficiaryID == "" {
				nullCount++
			}
		case "program_id":
			if event.ProgramID == "" {
				nullCount++
			}
		case "amount":
			if event.Amount == 0 {
				nullCount++
			}
		}
	}

	nullRate := float64(nullCount) / float64(len(data))
	passed := nullRate <= rule.Threshold

	return &QualityResult{
		RuleName:       rule.Name,
		Passed:         passed,
		Message:        fmt.Sprintf("Null rate: %.2f%% (threshold: %.2f%%)", nullRate*100, rule.Threshold*100),
		ViolationCount: nullCount,
		CheckedAt:      time.Now(),
	}
}

// checkRange checks if values are within expected range
func (dqc *DataQualityChecker) checkRange(rule *QualityRule, data []ParquetEvent) *QualityResult {
	violationCount := int64(0)
	for _, event := range data {
		if rule.Field == "amount" {
			if event.Amount < 0 || event.Amount > rule.Threshold {
				violationCount++
			}
		}
	}

	passed := violationCount == 0

	return &QualityResult{
		RuleName:       rule.Name,
		Passed:         passed,
		Message:        fmt.Sprintf("Range violations: %d", violationCount),
		ViolationCount: violationCount,
		CheckedAt:      time.Now(),
	}
}

// checkSchema validates schema compliance
func (dqc *DataQualityChecker) checkSchema(rule *QualityRule, data []ParquetEvent) *QualityResult {
	// Check if all required fields are present
	passed := true
	message := "Schema validation passed"

	if len(data) > 0 {
		// Check first event as sample
		event := data[0]
		if event.EventID == "" || event.EventType == "" {
			passed = false
			message = "Missing required fields"
		}
	}

	return &QualityResult{
		RuleName:  rule.Name,
		Passed:    passed,
		Message:   message,
		CheckedAt: time.Now(),
	}
}

// checkUniqueness checks for duplicate values
func (dqc *DataQualityChecker) checkUniqueness(rule *QualityRule, data []ParquetEvent) *QualityResult {
	seen := make(map[string]bool)
	duplicates := int64(0)

	for _, event := range data {
		var key string
		switch rule.Field {
		case "event_id":
			key = event.EventID
		case "idempotency_key":
			key = event.IdempotencyKey
		}

		if seen[key] {
			duplicates++
		}
		seen[key] = true
	}

	passed := duplicates == 0

	return &QualityResult{
		RuleName:       rule.Name,
		Passed:         passed,
		Message:        fmt.Sprintf("Duplicate values: %d", duplicates),
		ViolationCount: duplicates,
		CheckedAt:      time.Now(),
	}
}

// checkFreshness checks if data is fresh
func (dqc *DataQualityChecker) checkFreshness(rule *QualityRule, data []ParquetEvent) *QualityResult {
	if len(data) == 0 {
		return &QualityResult{
			RuleName:  rule.Name,
			Passed:    false,
			Message:   "No data to check",
			CheckedAt: time.Now(),
		}
	}

	// Find most recent event
	var mostRecent int64
	for _, event := range data {
		if event.Timestamp > mostRecent {
			mostRecent = event.Timestamp
		}
	}

	age := time.Since(time.UnixMilli(mostRecent))
	maxAge := time.Duration(rule.Threshold) * time.Hour
	passed := age <= maxAge

	return &QualityResult{
		RuleName:  rule.Name,
		Passed:    passed,
		Message:   fmt.Sprintf("Data age: %s (max: %s)", age.String(), maxAge.String()),
		CheckedAt: time.Now(),
	}
}

// GetResults returns all quality check results
func (dqc *DataQualityChecker) GetResults() map[string]*QualityResult {
	dqc.mu.RLock()
	defer dqc.mu.RUnlock()

	results := make(map[string]*QualityResult)
	for k, v := range dqc.results {
		results[k] = v
	}
	return results
}

// ExportResults exports quality check results to JSON
func (dqc *DataQualityChecker) ExportResults(filename string) error {
	results := dqc.GetResults()
	data, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filename, data, 0644)
}

// NewCDCProcessor creates a new CDC processor
func NewCDCProcessor(source, destination string) *CDCProcessor {
	ctx, cancel := context.WithCancel(context.Background())

	return &CDCProcessor{
		source:      source,
		destination: destination,
		buffer:      make([]CDCEvent, 0, 1000),
		ctx:         ctx,
		cancel:      cancel,
	}
}

// SetTransformer sets a transformation function for CDC events
func (cdc *CDCProcessor) SetTransformer(fn func(map[string]interface{}) (map[string]interface{}, error)) {
	cdc.transformer = fn
}

// ProcessEvent processes a CDC event
func (cdc *CDCProcessor) ProcessEvent(event CDCEvent) error {
	cdc.mu.Lock()
	defer cdc.mu.Unlock()

	// Apply transformation if set
	if cdc.transformer != nil {
		transformed, err := cdc.transformer(event.After)
		if err != nil {
			return fmt.Errorf("transformation failed: %w", err)
		}
		event.After = transformed
	}

	// Add to buffer
	cdc.buffer = append(cdc.buffer, event)

	// Flush if buffer is full
	if len(cdc.buffer) >= 1000 {
		return cdc.flush()
	}

	return nil
}

// flush writes buffered events to destination
func (cdc *CDCProcessor) flush() error {
	if len(cdc.buffer) == 0 {
		return nil
	}

	// Write to destination (simulate)
	log.Printf("[CDC] Flushing %d events to %s", len(cdc.buffer), cdc.destination)

	// A durable lakehouse writer is not configured here. Retain the buffered events until
	// a concrete writer is added rather than pretending that they were persisted.
	return fmt.Errorf("CDC lakehouse writer is not configured")

	// Clear buffer
	cdc.buffer = cdc.buffer[:0]
	return nil
}

// Start starts the CDC processor
func (cdc *CDCProcessor) Start() error {
	log.Printf("[CDC] Starting CDC processor: %s -> %s", cdc.source, cdc.destination)

	cdc.wg.Add(1)
	go cdc.processLoop()

	return nil
}

// processLoop processes CDC events in a loop
func (cdc *CDCProcessor) processLoop() {
	defer cdc.wg.Done()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-cdc.ctx.Done():
			cdc.flush()
			return
		case <-ticker.C:
			cdc.mu.Lock()
			if len(cdc.buffer) > 0 {
				cdc.flush()
			}
			cdc.mu.Unlock()
		}
	}
}

// Close closes the CDC processor
func (cdc *CDCProcessor) Close() error {
	cdc.cancel()
	cdc.wg.Wait()
	return nil
}

// GetStats returns CDC processor statistics
func (cdc *CDCProcessor) GetStats() map[string]interface{} {
	cdc.mu.Lock()
	defer cdc.mu.Unlock()

	return map[string]interface{}{
		"source":          cdc.source,
		"destination":     cdc.destination,
		"buffer_size":     len(cdc.buffer),
		"has_transformer": cdc.transformer != nil,
	}
}

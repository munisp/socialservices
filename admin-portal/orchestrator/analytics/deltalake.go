package analytics

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// DeltaLakeManager manages Delta Lake tables with ACID transactions
type DeltaLakeManager struct {
	tablePath     string
	versionLog    *VersionLog
	activeWriters map[string]*DeltaWriter
	mu            sync.RWMutex
}

// VersionLog tracks Delta Lake transaction log
type VersionLog struct {
	path     string
	versions []DeltaVersion
	mu       sync.RWMutex
}

// DeltaVersion represents a Delta Lake version
type DeltaVersion struct {
	Version   int64                  `json:"version"`
	Timestamp time.Time              `json:"timestamp"`
	Operation string                 `json:"operation"`
	Files     []string               `json:"files"`
	Metadata  map[string]interface{} `json:"metadata"`
}

// DeltaWriter handles transactional writes to Delta Lake
type DeltaWriter struct {
	tableName string
	version   int64
	files     []string
	mu        sync.Mutex
}

// SchemaRegistry manages event schemas with versioning
type SchemaRegistry struct {
	registryURL string
	schemas     map[string]*SchemaVersion
	mu          sync.RWMutex
}

// SchemaVersion represents a schema version
type SchemaVersion struct {
	Subject string                 `json:"subject"`
	Version int                    `json:"version"`
	Schema  map[string]interface{} `json:"schema"`
	ID      int                    `json:"id"`
}

// NewDeltaLakeManager creates a new Delta Lake manager
func NewDeltaLakeManager(tablePath string) (*DeltaLakeManager, error) {
	// Create table directory
	if err := os.MkdirAll(tablePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create table directory: %w", err)
	}

	// Create _delta_log directory
	deltaLogPath := filepath.Join(tablePath, "_delta_log")
	if err := os.MkdirAll(deltaLogPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create delta log directory: %w", err)
	}

	versionLog, err := NewVersionLog(deltaLogPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create version log: %w", err)
	}

	return &DeltaLakeManager{
		tablePath:     tablePath,
		versionLog:    versionLog,
		activeWriters: make(map[string]*DeltaWriter),
	}, nil
}

// NewVersionLog creates a new version log
func NewVersionLog(path string) (*VersionLog, error) {
	vl := &VersionLog{
		path:     path,
		versions: make([]DeltaVersion, 0),
	}

	// Load existing versions
	if err := vl.load(); err != nil {
		log.Printf("Warning: failed to load version log: %v", err)
	}

	return vl, nil
}

// load loads version log from disk
func (vl *VersionLog) load() error {
	files, err := filepath.Glob(filepath.Join(vl.path, "*.json"))
	if err != nil {
		return err
	}

	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			return err
		}

		var version DeltaVersion
		if err := json.Unmarshal(data, &version); err != nil {
			return err
		}

		vl.versions = append(vl.versions, version)
	}

	return nil
}

// GetLatestVersion returns the latest version number
func (vl *VersionLog) GetLatestVersion() int64 {
	vl.mu.RLock()
	defer vl.mu.RUnlock()

	if len(vl.versions) == 0 {
		return -1
	}
	return vl.versions[len(vl.versions)-1].Version
}

// CommitVersion commits a new version to the log
func (vl *VersionLog) CommitVersion(operation string, files []string, metadata map[string]interface{}) error {
	vl.mu.Lock()
	defer vl.mu.Unlock()

	version := DeltaVersion{
		Version:   vl.GetLatestVersion() + 1,
		Timestamp: time.Now(),
		Operation: operation,
		Files:     files,
		Metadata:  metadata,
	}

	// Write version file
	versionFile := filepath.Join(vl.path, fmt.Sprintf("%020d.json", version.Version))
	data, err := json.MarshalIndent(version, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal version: %w", err)
	}

	if err := os.WriteFile(versionFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write version file: %w", err)
	}

	vl.versions = append(vl.versions, version)
	log.Printf("[DeltaLake] Committed version %d with %d files", version.Version, len(files))
	return nil
}

// GetVersionHistory returns version history with optional time travel
func (vl *VersionLog) GetVersionHistory(fromVersion, toVersion int64) []DeltaVersion {
	vl.mu.RLock()
	defer vl.mu.RUnlock()

	var history []DeltaVersion
	for _, v := range vl.versions {
		if v.Version >= fromVersion && (toVersion == -1 || v.Version <= toVersion) {
			history = append(history, v)
		}
	}
	return history
}

// BeginTransaction starts a new Delta Lake transaction
func (dlm *DeltaLakeManager) BeginTransaction(tableName string) (*DeltaWriter, error) {
	dlm.mu.Lock()
	defer dlm.mu.Unlock()

	writer := &DeltaWriter{
		tableName: tableName,
		version:   dlm.versionLog.GetLatestVersion() + 1,
		files:     make([]string, 0),
	}

	dlm.activeWriters[tableName] = writer
	return writer, nil
}

// AddFile adds a file to the transaction
func (dw *DeltaWriter) AddFile(filename string) {
	dw.mu.Lock()
	defer dw.mu.Unlock()
	dw.files = append(dw.files, filename)
}

// Commit commits the transaction
func (dlm *DeltaLakeManager) CommitTransaction(writer *DeltaWriter, metadata map[string]interface{}) error {
	dlm.mu.Lock()
	defer dlm.mu.Unlock()

	if err := dlm.versionLog.CommitVersion("WRITE", writer.files, metadata); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	delete(dlm.activeWriters, writer.tableName)
	return nil
}

// Rollback rolls back the transaction
func (dlm *DeltaLakeManager) RollbackTransaction(writer *DeltaWriter) error {
	dlm.mu.Lock()
	defer dlm.mu.Unlock()

	// Delete uncommitted files
	for _, file := range writer.files {
		if err := os.Remove(filepath.Join(dlm.tablePath, file)); err != nil {
			log.Printf("Warning: failed to delete file %s: %v", file, err)
		}
	}

	delete(dlm.activeWriters, writer.tableName)
	log.Printf("[DeltaLake] Rolled back transaction for table %s", writer.tableName)
	return nil
}

// TimeTravel returns data at a specific version
func (dlm *DeltaLakeManager) TimeTravel(version int64) ([]string, error) {
	history := dlm.versionLog.GetVersionHistory(0, version)
	if len(history) == 0 {
		return nil, fmt.Errorf("version %d not found", version)
	}

	// Collect all files up to this version
	fileSet := make(map[string]bool)
	for _, v := range history {
		for _, file := range v.Files {
			fileSet[file] = true
		}
	}

	files := make([]string, 0, len(fileSet))
	for file := range fileSet {
		files = append(files, file)
	}

	return files, nil
}

// FileInfo holds metadata about a file for compaction decisions
type FileInfo struct {
	Path string
	Size int64
}

// CompactionConfig holds configuration for file compaction
type CompactionConfig struct {
	MinFileSizeBytes int64 // Files smaller than this are candidates for compaction (default: 128MB)
	MaxFileSizeBytes int64 // Target size for compacted files (default: 1GB)
	MaxFilesPerBatch int   // Maximum files to compact in one batch (default: 100)
}

// DefaultCompactionConfig returns default compaction configuration
func DefaultCompactionConfig() *CompactionConfig {
	return &CompactionConfig{
		MinFileSizeBytes: 128 * 1024 * 1024,  // 128MB
		MaxFileSizeBytes: 1024 * 1024 * 1024, // 1GB
		MaxFilesPerBatch: 100,
	}
}

// Optimize compacts small files into larger ones
func (dlm *DeltaLakeManager) Optimize(ctx context.Context) error {
	return dlm.OptimizeWithConfig(ctx, DefaultCompactionConfig())
}

// OptimizeWithConfig compacts small files with custom configuration
func (dlm *DeltaLakeManager) OptimizeWithConfig(ctx context.Context, config *CompactionConfig) error {
	log.Println("[DeltaLake] Starting table optimization...")

	// Get all current files
	latestVersion := dlm.versionLog.GetLatestVersion()
	files, err := dlm.TimeTravel(latestVersion)
	if err != nil {
		return err
	}

	if len(files) == 0 {
		log.Println("[DeltaLake] No files to optimize")
		return nil
	}

	// Step 1: Analyze files and identify small files for compaction
	smallFiles, err := dlm.identifySmallFiles(files, config.MinFileSizeBytes)
	if err != nil {
		return fmt.Errorf("failed to identify small files: %w", err)
	}

	if len(smallFiles) < 2 {
		log.Printf("[DeltaLake] Not enough small files to compact (found %d)", len(smallFiles))
		return nil
	}

	log.Printf("[DeltaLake] Found %d small files for compaction", len(smallFiles))

	// Step 2: Group files into compaction batches
	batches := dlm.groupFilesIntoBatches(smallFiles, config)

	// Step 3: Compact each batch
	var compactedFiles []string
	var filesToRemove []string

	for i, batch := range batches {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		log.Printf("[DeltaLake] Compacting batch %d/%d with %d files", i+1, len(batches), len(batch))

		compactedFile, err := dlm.compactBatch(batch, i)
		if err != nil {
			log.Printf("[DeltaLake] Warning: failed to compact batch %d: %v", i+1, err)
			continue
		}

		compactedFiles = append(compactedFiles, compactedFile)
		for _, f := range batch {
			filesToRemove = append(filesToRemove, f.Path)
		}
	}

	// Step 4: Commit new version with compacted files
	if len(compactedFiles) > 0 {
		// Create new file list: keep non-compacted files + add compacted files
		newFiles := make([]string, 0)
		removeSet := make(map[string]bool)
		for _, f := range filesToRemove {
			removeSet[f] = true
		}

		for _, f := range files {
			if !removeSet[f] {
				newFiles = append(newFiles, f)
			}
		}
		newFiles = append(newFiles, compactedFiles...)

		metadata := map[string]interface{}{
			"operation":       "OPTIMIZE",
			"files_compacted": len(filesToRemove),
			"files_created":   len(compactedFiles),
			"timestamp":       time.Now().Format(time.RFC3339),
		}

		if err := dlm.versionLog.CommitVersion("OPTIMIZE", newFiles, metadata); err != nil {
			return fmt.Errorf("failed to commit optimized version: %w", err)
		}

		// Step 5: Mark old files for deletion (they will be cleaned up by Vacuum)
		for _, f := range filesToRemove {
			markerFile := filepath.Join(dlm.tablePath, f+".tombstone")
			os.WriteFile(markerFile, []byte(time.Now().Format(time.RFC3339)), 0644)
		}
	}

	log.Printf("[DeltaLake] Optimization complete: compacted %d files into %d files",
		len(filesToRemove), len(compactedFiles))
	return nil
}

// identifySmallFiles returns files smaller than the threshold
func (dlm *DeltaLakeManager) identifySmallFiles(files []string, threshold int64) ([]FileInfo, error) {
	var smallFiles []FileInfo

	for _, file := range files {
		fullPath := filepath.Join(dlm.tablePath, file)
		info, err := os.Stat(fullPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue // File may have been deleted
			}
			return nil, err
		}

		if info.Size() < threshold {
			smallFiles = append(smallFiles, FileInfo{
				Path: file,
				Size: info.Size(),
			})
		}
	}

	return smallFiles, nil
}

// groupFilesIntoBatches groups small files into batches for compaction
func (dlm *DeltaLakeManager) groupFilesIntoBatches(files []FileInfo, config *CompactionConfig) [][]FileInfo {
	var batches [][]FileInfo
	var currentBatch []FileInfo
	var currentSize int64

	for _, file := range files {
		// Start new batch if current would exceed limits
		if len(currentBatch) >= config.MaxFilesPerBatch ||
			(currentSize+file.Size > config.MaxFileSizeBytes && len(currentBatch) > 0) {
			batches = append(batches, currentBatch)
			currentBatch = nil
			currentSize = 0
		}

		currentBatch = append(currentBatch, file)
		currentSize += file.Size
	}

	// Add remaining files
	if len(currentBatch) > 1 {
		batches = append(batches, currentBatch)
	}

	return batches
}

// compactBatch merges multiple files into a single compacted file
func (dlm *DeltaLakeManager) compactBatch(batch []FileInfo, batchIndex int) (string, error) {
	// Generate compacted file name
	compactedFileName := fmt.Sprintf("compacted_%d_%s.parquet",
		batchIndex, time.Now().Format("20060102150405"))
	compactedPath := filepath.Join(dlm.tablePath, compactedFileName)

	// Read and merge all files in the batch
	var mergedData []map[string]interface{}

	for _, file := range batch {
		fullPath := filepath.Join(dlm.tablePath, file.Path)
		data, err := os.ReadFile(fullPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return "", fmt.Errorf("failed to read file %s: %w", file.Path, err)
		}

		// Try to parse as JSON (for JSON-based storage)
		var records []map[string]interface{}
		if err := json.Unmarshal(data, &records); err != nil {
			// Try single record
			var record map[string]interface{}
			if err := json.Unmarshal(data, &record); err != nil {
				log.Printf("[DeltaLake] Warning: could not parse file %s, skipping", file.Path)
				continue
			}
			records = []map[string]interface{}{record}
		}

		mergedData = append(mergedData, records...)
	}

	if len(mergedData) == 0 {
		return "", fmt.Errorf("no data to compact")
	}

	// Write compacted file
	compactedData, err := json.MarshalIndent(mergedData, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal compacted data: %w", err)
	}

	if err := os.WriteFile(compactedPath, compactedData, 0644); err != nil {
		return "", fmt.Errorf("failed to write compacted file: %w", err)
	}

	log.Printf("[DeltaLake] Created compacted file %s with %d records from %d files",
		compactedFileName, len(mergedData), len(batch))

	return compactedFileName, nil
}

// Vacuum removes old files based on retention policy
func (dlm *DeltaLakeManager) Vacuum(retentionHours int) error {
	log.Printf("[DeltaLake] Starting vacuum with %d hour retention...", retentionHours)

	cutoffTime := time.Now().Add(-time.Duration(retentionHours) * time.Hour)

	// Get versions older than retention period
	history := dlm.versionLog.GetVersionHistory(0, -1)
	filesToDelete := make(map[string]bool)

	for _, v := range history {
		if v.Timestamp.Before(cutoffTime) {
			for _, file := range v.Files {
				filesToDelete[file] = true
			}
		}
	}

	// Delete old files
	deletedCount := 0
	for file := range filesToDelete {
		fullPath := filepath.Join(dlm.tablePath, file)
		if err := os.Remove(fullPath); err != nil {
			log.Printf("Warning: failed to delete file %s: %v", file, err)
		} else {
			deletedCount++
		}
	}

	log.Printf("[DeltaLake] Vacuum complete, deleted %d files", deletedCount)
	return nil
}

// NewSchemaRegistry creates a new schema registry client
func NewSchemaRegistry(registryURL string) *SchemaRegistry {
	return &SchemaRegistry{
		registryURL: registryURL,
		schemas:     make(map[string]*SchemaVersion),
	}
}

// RegisterSchema registers a new schema version
func (sr *SchemaRegistry) RegisterSchema(subject string, schema map[string]interface{}) (*SchemaVersion, error) {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	// Get current version
	currentVersion := 0
	if existing, ok := sr.schemas[subject]; ok {
		currentVersion = existing.Version
	}

	schemaVersion := &SchemaVersion{
		Subject: subject,
		Version: currentVersion + 1,
		Schema:  schema,
		ID:      len(sr.schemas) + 1,
	}

	sr.schemas[subject] = schemaVersion

	// Persist to disk (simulate registry)
	if err := sr.persist(schemaVersion); err != nil {
		return nil, err
	}

	log.Printf("[SchemaRegistry] Registered schema %s version %d", subject, schemaVersion.Version)
	return schemaVersion, nil
}

// GetSchema retrieves a schema by subject
func (sr *SchemaRegistry) GetSchema(subject string) (*SchemaVersion, error) {
	sr.mu.RLock()
	defer sr.mu.RUnlock()

	if schema, ok := sr.schemas[subject]; ok {
		return schema, nil
	}

	return nil, fmt.Errorf("schema not found: %s", subject)
}

// ValidateEvent validates an event against a schema
func (sr *SchemaRegistry) ValidateEvent(subject string, event map[string]interface{}) error {
	schema, err := sr.GetSchema(subject)
	if err != nil {
		return err
	}

	// Basic validation (check required fields)
	requiredFields, ok := schema.Schema["required"].([]interface{})
	if !ok {
		return nil // No required fields
	}

	for _, field := range requiredFields {
		fieldName, ok := field.(string)
		if !ok {
			continue
		}

		if _, exists := event[fieldName]; !exists {
			return fmt.Errorf("missing required field: %s", fieldName)
		}
	}

	return nil
}

// persist saves schema to disk
func (sr *SchemaRegistry) persist(schema *SchemaVersion) error {
	schemaDir := "./schemas"
	if err := os.MkdirAll(schemaDir, 0755); err != nil {
		return err
	}

	filename := filepath.Join(schemaDir, fmt.Sprintf("%s-v%d.json", schema.Subject, schema.Version))
	data, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filename, data, 0644)
}

// LoadSchemas loads all schemas from disk
func (sr *SchemaRegistry) LoadSchemas() error {
	schemaDir := "./schemas"
	files, err := filepath.Glob(filepath.Join(schemaDir, "*.json"))
	if err != nil {
		return err
	}

	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			return err
		}

		var schema SchemaVersion
		if err := json.Unmarshal(data, &schema); err != nil {
			return err
		}

		sr.mu.Lock()
		sr.schemas[schema.Subject] = &schema
		sr.mu.Unlock()
	}

	log.Printf("[SchemaRegistry] Loaded %d schemas", len(sr.schemas))
	return nil
}

// GetSchemaEvolution returns the evolution history of a schema
func (sr *SchemaRegistry) GetSchemaEvolution(subject string) ([]*SchemaVersion, error) {
	schemaDir := "./schemas"
	pattern := filepath.Join(schemaDir, fmt.Sprintf("%s-v*.json", subject))
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}

	var evolution []*SchemaVersion
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}

		var schema SchemaVersion
		if err := json.Unmarshal(data, &schema); err != nil {
			continue
		}

		evolution = append(evolution, &schema)
	}

	return evolution, nil
}

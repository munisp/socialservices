package workflows

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// ========== Bulk Operations Workflow ==========

// BulkOperationInput contains the input for bulk operations
type BulkOperationInput struct {
	OperationID   string   `json:"operationId"`
	OperationType string   `json:"operationType"` // import, export, update, delete
	EntityType    string   `json:"entityType"`    // beneficiary, program, transaction
	ItemIDs       []string `json:"itemIds,omitempty"`
	Filters       map[string]interface{} `json:"filters,omitempty"`
	UpdateData    map[string]interface{} `json:"updateData,omitempty"`
	FileURL       string   `json:"fileUrl,omitempty"` // For import operations
	ExportFormat  string   `json:"exportFormat,omitempty"` // csv, xlsx, json
	InitiatedBy   string   `json:"initiatedBy"`
	BatchSize     int      `json:"batchSize"`
	CreatedAt     time.Time `json:"createdAt"`
}

// BulkOperationResult contains the result of bulk operations
type BulkOperationResult struct {
	OperationID     string     `json:"operationId"`
	Status          string     `json:"status"` // completed, partial, failed, cancelled
	TotalItems      int        `json:"totalItems"`
	ProcessedItems  int        `json:"processedItems"`
	SuccessCount    int        `json:"successCount"`
	FailureCount    int        `json:"failureCount"`
	SkippedCount    int        `json:"skippedCount"`
	Errors          []BulkError `json:"errors,omitempty"`
	OutputFileURL   string     `json:"outputFileUrl,omitempty"`
	StartedAt       time.Time  `json:"startedAt"`
	CompletedAt     *time.Time `json:"completedAt,omitempty"`
	Progress        float64    `json:"progress"`
	Message         string     `json:"message"`
}

// BulkError represents an error for a specific item
type BulkError struct {
	ItemID    string `json:"itemId"`
	ItemIndex int    `json:"itemIndex"`
	Error     string `json:"error"`
	Timestamp time.Time `json:"timestamp"`
}

// BulkOperationWorkflow handles bulk operations with progress tracking
func BulkOperationWorkflow(ctx workflow.Context, input BulkOperationInput) (*BulkOperationResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting bulk operation workflow",
		"operationId", input.OperationID,
		"operationType", input.OperationType,
		"entityType", input.EntityType,
	)

	activityOptions := workflow.ActivityOptions{
		StartToCloseTimeout: 60 * time.Minute,
		HeartbeatTimeout:    5 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second * 5,
			BackoffCoefficient: 2.0,
			MaximumInterval:    5 * time.Minute,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, activityOptions)

	result := &BulkOperationResult{
		OperationID: input.OperationID,
		StartedAt:   workflow.Now(ctx),
	}

	// Step 1: Initialize operation record
	err := workflow.ExecuteActivity(ctx, "CreateBulkOperationRecordActivity", input).Get(ctx, nil)
	if err != nil {
		result.Status = "failed"
		result.Message = "Failed to initialize operation"
		return result, err
	}

	// Step 2: Determine items to process
	var itemsToProcess []string
	if len(input.ItemIDs) > 0 {
		itemsToProcess = input.ItemIDs
	} else if input.OperationType == "import" {
		// Parse file to get items
		err = workflow.ExecuteActivity(ctx, "ParseImportFileActivity",
			input.FileURL, input.EntityType).Get(ctx, &itemsToProcess)
		if err != nil {
			result.Status = "failed"
			result.Message = "Failed to parse import file"
			return result, err
		}
	} else {
		// Query items based on filters
		err = workflow.ExecuteActivity(ctx, "QueryItemsByFilterActivity",
			input.EntityType, input.Filters).Get(ctx, &itemsToProcess)
		if err != nil {
			result.Status = "failed"
			result.Message = "Failed to query items"
			return result, err
		}
	}

	result.TotalItems = len(itemsToProcess)
	if result.TotalItems == 0 {
		result.Status = "completed"
		result.Message = "No items to process"
		now := workflow.Now(ctx)
		result.CompletedAt = &now
		return result, nil
	}

	// Step 3: Process items in batches
	batchSize := input.BatchSize
	if batchSize <= 0 {
		batchSize = 100
	}

	var errors []BulkError
	cancelChan := workflow.GetSignalChannel(ctx, "cancel-operation")

	for i := 0; i < len(itemsToProcess); i += batchSize {
		// Check for cancellation signal
		var cancelled bool
		selector := workflow.NewSelector(ctx)
		selector.AddReceive(cancelChan, func(c workflow.ReceiveChannel, more bool) {
			var signal struct{}
			c.Receive(ctx, &signal)
			cancelled = true
		})
		selector.AddDefault(func() {})
		selector.Select(ctx)

		if cancelled {
			result.Status = "cancelled"
			result.Message = "Operation cancelled by user"
			break
		}

		// Get batch
		end := i + batchSize
		if end > len(itemsToProcess) {
			end = len(itemsToProcess)
		}
		batch := itemsToProcess[i:end]

		// Process batch based on operation type
		var batchResult BatchProcessResult
		switch input.OperationType {
		case "import":
			err = workflow.ExecuteActivity(ctx, "ProcessImportBatchActivity",
				input.EntityType, batch, input.InitiatedBy).Get(ctx, &batchResult)
		case "export":
			err = workflow.ExecuteActivity(ctx, "ProcessExportBatchActivity",
				input.EntityType, batch, input.ExportFormat).Get(ctx, &batchResult)
		case "update":
			err = workflow.ExecuteActivity(ctx, "ProcessUpdateBatchActivity",
				input.EntityType, batch, input.UpdateData, input.InitiatedBy).Get(ctx, &batchResult)
		case "delete":
			err = workflow.ExecuteActivity(ctx, "ProcessDeleteBatchActivity",
				input.EntityType, batch, input.InitiatedBy).Get(ctx, &batchResult)
		default:
			result.Status = "failed"
			result.Message = fmt.Sprintf("Unknown operation type: %s", input.OperationType)
			return result, fmt.Errorf("unknown operation type: %s", input.OperationType)
		}

		if err != nil {
			logger.Warn("Batch processing failed", "batchStart", i, "error", err)
			// Continue with next batch
		}

		// Update results
		result.ProcessedItems += len(batch)
		result.SuccessCount += batchResult.SuccessCount
		result.FailureCount += batchResult.FailureCount
		result.SkippedCount += batchResult.SkippedCount
		errors = append(errors, batchResult.Errors...)

		// Update progress
		result.Progress = float64(result.ProcessedItems) / float64(result.TotalItems) * 100

		// Update operation record with progress
		_ = workflow.ExecuteActivity(ctx, "UpdateBulkOperationProgressActivity",
			input.OperationID, result.Progress, result.ProcessedItems, result.SuccessCount, result.FailureCount).Get(ctx, nil)
	}

	// Step 4: Finalize operation
	result.Errors = errors
	now := workflow.Now(ctx)
	result.CompletedAt = &now

	// For export operations, generate output file
	if input.OperationType == "export" {
		var outputURL string
		err = workflow.ExecuteActivity(ctx, "FinalizeExportActivity",
			input.OperationID, input.ExportFormat).Get(ctx, &outputURL)
		if err != nil {
			logger.Warn("Failed to finalize export", "error", err)
		} else {
			result.OutputFileURL = outputURL
		}
	}

	// Determine final status
	if result.Status == "" {
		if result.FailureCount == 0 {
			result.Status = "completed"
			result.Message = fmt.Sprintf("All %d items processed successfully", result.SuccessCount)
		} else if result.SuccessCount == 0 {
			result.Status = "failed"
			result.Message = fmt.Sprintf("All %d items failed", result.FailureCount)
		} else {
			result.Status = "partial"
			result.Message = fmt.Sprintf("%d succeeded, %d failed, %d skipped",
				result.SuccessCount, result.FailureCount, result.SkippedCount)
		}
	}

	// Step 5: Update final operation record
	err = workflow.ExecuteActivity(ctx, "FinalizeBulkOperationActivity",
		input.OperationID, result).Get(ctx, nil)
	if err != nil {
		logger.Warn("Failed to finalize operation record", "error", err)
	}

	// Step 6: Send completion notification
	err = workflow.ExecuteActivity(ctx, "SendBulkOperationNotificationActivity",
		input.InitiatedBy, input.OperationID, result).Get(ctx, nil)
	if err != nil {
		logger.Warn("Failed to send completion notification", "error", err)
	}

	// Step 7: Create audit log
	err = workflow.ExecuteActivity(ctx, "CreateAuditLogActivity",
		"bulk_operation.completed", input.OperationID, input.InitiatedBy, result).Get(ctx, nil)
	if err != nil {
		logger.Warn("Failed to create audit log", "error", err)
	}

	logger.Info("Bulk operation workflow completed",
		"status", result.Status,
		"processed", result.ProcessedItems,
		"success", result.SuccessCount,
		"failure", result.FailureCount,
	)
	return result, nil
}

// BatchProcessResult contains the result of processing a batch
type BatchProcessResult struct {
	SuccessCount int         `json:"successCount"`
	FailureCount int         `json:"failureCount"`
	SkippedCount int         `json:"skippedCount"`
	Errors       []BulkError `json:"errors"`
}

// ========== Bulk Import Workflow ==========

// BulkImportInput contains specific input for bulk import
type BulkImportInput struct {
	ImportID      string            `json:"importId"`
	EntityType    string            `json:"entityType"`
	FileURL       string            `json:"fileUrl"`
	FileFormat    string            `json:"fileFormat"` // csv, xlsx, json
	ColumnMapping map[string]string `json:"columnMapping"`
	ValidationRules []ValidationRule `json:"validationRules"`
	DuplicateHandling string        `json:"duplicateHandling"` // skip, update, error
	InitiatedBy   string            `json:"initiatedBy"`
	BatchSize     int               `json:"batchSize"`
}

// ValidationRule defines a validation rule for import
type ValidationRule struct {
	Field      string `json:"field"`
	Rule       string `json:"rule"` // required, email, phone, regex, range
	Parameters string `json:"parameters,omitempty"`
}

// BulkImportResult contains the result of bulk import
type BulkImportResult struct {
	ImportID        string     `json:"importId"`
	Status          string     `json:"status"`
	TotalRows       int        `json:"totalRows"`
	ValidRows       int        `json:"validRows"`
	InvalidRows     int        `json:"invalidRows"`
	ImportedRows    int        `json:"importedRows"`
	SkippedRows     int        `json:"skippedRows"`
	DuplicateRows   int        `json:"duplicateRows"`
	ValidationErrors []ImportValidationError `json:"validationErrors,omitempty"`
	OutputFileURL   string     `json:"outputFileUrl,omitempty"`
	CompletedAt     *time.Time `json:"completedAt,omitempty"`
	Message         string     `json:"message"`
}

// ImportValidationError represents a validation error for a row
type ImportValidationError struct {
	RowNumber int      `json:"rowNumber"`
	Field     string   `json:"field"`
	Value     string   `json:"value"`
	Error     string   `json:"error"`
}

// BulkImportWorkflow handles bulk import with validation
func BulkImportWorkflow(ctx workflow.Context, input BulkImportInput) (*BulkImportResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting bulk import workflow",
		"importId", input.ImportID,
		"entityType", input.EntityType,
	)

	activityOptions := workflow.ActivityOptions{
		StartToCloseTimeout: 60 * time.Minute,
		HeartbeatTimeout:    5 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second * 5,
			BackoffCoefficient: 2.0,
			MaximumInterval:    5 * time.Minute,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, activityOptions)

	result := &BulkImportResult{
		ImportID: input.ImportID,
	}

	// Step 1: Download and parse file
	var parsedRows []map[string]interface{}
	err := workflow.ExecuteActivity(ctx, "DownloadAndParseFileActivity",
		input.FileURL, input.FileFormat, input.ColumnMapping).Get(ctx, &parsedRows)
	if err != nil {
		result.Status = "failed"
		result.Message = "Failed to parse import file"
		return result, err
	}

	result.TotalRows = len(parsedRows)
	if result.TotalRows == 0 {
		result.Status = "completed"
		result.Message = "No rows to import"
		now := workflow.Now(ctx)
		result.CompletedAt = &now
		return result, nil
	}

	// Step 2: Validate all rows
	var validationResult ValidationBatchResult
	err = workflow.ExecuteActivity(ctx, "ValidateImportRowsActivity",
		parsedRows, input.ValidationRules, input.EntityType).Get(ctx, &validationResult)
	if err != nil {
		result.Status = "failed"
		result.Message = "Validation failed"
		return result, err
	}

	result.ValidRows = validationResult.ValidCount
	result.InvalidRows = validationResult.InvalidCount
	result.ValidationErrors = validationResult.Errors

	if result.ValidRows == 0 {
		result.Status = "failed"
		result.Message = "All rows failed validation"
		now := workflow.Now(ctx)
		result.CompletedAt = &now
		return result, nil
	}

	// Step 3: Check for duplicates
	var duplicateResult DuplicateCheckResult
	err = workflow.ExecuteActivity(ctx, "CheckDuplicatesActivity",
		validationResult.ValidRows, input.EntityType).Get(ctx, &duplicateResult)
	if err != nil {
		logger.Warn("Duplicate check failed", "error", err)
	}

	result.DuplicateRows = duplicateResult.DuplicateCount

	// Step 4: Process valid rows in batches
	batchSize := input.BatchSize
	if batchSize <= 0 {
		batchSize = 100
	}

	rowsToImport := duplicateResult.UniqueRows
	if input.DuplicateHandling == "update" {
		rowsToImport = append(rowsToImport, duplicateResult.DuplicateRows...)
	}

	for i := 0; i < len(rowsToImport); i += batchSize {
		end := i + batchSize
		if end > len(rowsToImport) {
			end = len(rowsToImport)
		}
		batch := rowsToImport[i:end]

		var batchResult ImportBatchResult
		err = workflow.ExecuteActivity(ctx, "ImportBatchActivity",
			input.EntityType, batch, input.DuplicateHandling, input.InitiatedBy).Get(ctx, &batchResult)
		if err != nil {
			logger.Warn("Batch import failed", "batchStart", i, "error", err)
		}

		result.ImportedRows += batchResult.ImportedCount
		result.SkippedRows += batchResult.SkippedCount
	}

	// Step 5: Generate result file
	var outputURL string
	err = workflow.ExecuteActivity(ctx, "GenerateImportResultFileActivity",
		input.ImportID, result).Get(ctx, &outputURL)
	if err != nil {
		logger.Warn("Failed to generate result file", "error", err)
	} else {
		result.OutputFileURL = outputURL
	}

	// Determine final status
	now := workflow.Now(ctx)
	result.CompletedAt = &now
	if result.InvalidRows == 0 && result.SkippedRows == 0 {
		result.Status = "completed"
		result.Message = fmt.Sprintf("Successfully imported %d rows", result.ImportedRows)
	} else {
		result.Status = "partial"
		result.Message = fmt.Sprintf("Imported %d, skipped %d, invalid %d",
			result.ImportedRows, result.SkippedRows, result.InvalidRows)
	}

	// Step 6: Create audit log
	_ = workflow.ExecuteActivity(ctx, "CreateAuditLogActivity",
		"bulk_import.completed", input.ImportID, input.InitiatedBy, result).Get(ctx, nil)

	logger.Info("Bulk import workflow completed", "status", result.Status)
	return result, nil
}

// ValidationBatchResult contains validation results
type ValidationBatchResult struct {
	ValidCount   int                     `json:"validCount"`
	InvalidCount int                     `json:"invalidCount"`
	ValidRows    []map[string]interface{} `json:"validRows"`
	Errors       []ImportValidationError `json:"errors"`
}

// DuplicateCheckResult contains duplicate check results
type DuplicateCheckResult struct {
	UniqueRows     []map[string]interface{} `json:"uniqueRows"`
	DuplicateRows  []map[string]interface{} `json:"duplicateRows"`
	DuplicateCount int                      `json:"duplicateCount"`
}

// ImportBatchResult contains batch import results
type ImportBatchResult struct {
	ImportedCount int `json:"importedCount"`
	SkippedCount  int `json:"skippedCount"`
}

// ========== Bulk Export Workflow ==========

// BulkExportInput contains specific input for bulk export
type BulkExportInput struct {
	ExportID      string                 `json:"exportId"`
	EntityType    string                 `json:"entityType"`
	Filters       map[string]interface{} `json:"filters,omitempty"`
	Fields        []string               `json:"fields,omitempty"` // Fields to export
	Format        string                 `json:"format"` // csv, xlsx, json
	MaskPII       bool                   `json:"maskPii"`
	InitiatedBy   string                 `json:"initiatedBy"`
	UserRole      string                 `json:"userRole"` // For role-based field masking
}

// BulkExportResult contains the result of bulk export
type BulkExportResult struct {
	ExportID      string     `json:"exportId"`
	Status        string     `json:"status"`
	TotalRecords  int        `json:"totalRecords"`
	ExportedRecords int      `json:"exportedRecords"`
	FileURL       string     `json:"fileUrl"`
	FileSize      int64      `json:"fileSize"`
	CompletedAt   *time.Time `json:"completedAt,omitempty"`
	Message       string     `json:"message"`
}

// BulkExportWorkflow handles bulk export with role-based masking
func BulkExportWorkflow(ctx workflow.Context, input BulkExportInput) (*BulkExportResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting bulk export workflow",
		"exportId", input.ExportID,
		"entityType", input.EntityType,
	)

	activityOptions := workflow.ActivityOptions{
		StartToCloseTimeout: 60 * time.Minute,
		HeartbeatTimeout:    5 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second * 5,
			BackoffCoefficient: 2.0,
			MaximumInterval:    5 * time.Minute,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, activityOptions)

	result := &BulkExportResult{
		ExportID: input.ExportID,
	}

	// Step 1: Get allowed fields based on user role
	var allowedFields []string
	err := workflow.ExecuteActivity(ctx, "GetAllowedExportFieldsActivity",
		input.EntityType, input.UserRole, input.Fields).Get(ctx, &allowedFields)
	if err != nil {
		result.Status = "failed"
		result.Message = "Failed to determine allowed fields"
		return result, err
	}

	// Step 2: Query records
	var records []map[string]interface{}
	err = workflow.ExecuteActivity(ctx, "QueryExportRecordsActivity",
		input.EntityType, input.Filters, allowedFields).Get(ctx, &records)
	if err != nil {
		result.Status = "failed"
		result.Message = "Failed to query records"
		return result, err
	}

	result.TotalRecords = len(records)
	if result.TotalRecords == 0 {
		result.Status = "completed"
		result.Message = "No records to export"
		now := workflow.Now(ctx)
		result.CompletedAt = &now
		return result, nil
	}

	// Step 3: Apply PII masking if required
	if input.MaskPII {
		var maskedRecords []map[string]interface{}
		err = workflow.ExecuteActivity(ctx, "MaskPIIFieldsActivity",
			records, input.EntityType, input.UserRole).Get(ctx, &maskedRecords)
		if err != nil {
			logger.Warn("PII masking failed, using original records", "error", err)
		} else {
			records = maskedRecords
		}
	}

	// Step 4: Generate export file
	var exportResult ExportFileResult
	err = workflow.ExecuteActivity(ctx, "GenerateExportFileActivity",
		input.ExportID, records, input.Format, allowedFields).Get(ctx, &exportResult)
	if err != nil {
		result.Status = "failed"
		result.Message = "Failed to generate export file"
		return result, err
	}

	result.FileURL = exportResult.FileURL
	result.FileSize = exportResult.FileSize
	result.ExportedRecords = len(records)

	// Step 5: Finalize
	now := workflow.Now(ctx)
	result.CompletedAt = &now
	result.Status = "completed"
	result.Message = fmt.Sprintf("Successfully exported %d records", result.ExportedRecords)

	// Step 6: Create audit log
	_ = workflow.ExecuteActivity(ctx, "CreateAuditLogActivity",
		"bulk_export.completed", input.ExportID, input.InitiatedBy, result).Get(ctx, nil)

	logger.Info("Bulk export workflow completed", "status", result.Status)
	return result, nil
}

// ExportFileResult contains the result of file generation
type ExportFileResult struct {
	FileURL  string `json:"fileUrl"`
	FileSize int64  `json:"fileSize"`
}

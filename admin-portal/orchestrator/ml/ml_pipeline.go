package ml

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// MLModel represents a machine learning model
type MLModel struct {
	ID              int64           `json:"id" db:"id"`
	ModelName       string          `json:"modelName" db:"model_name"`
	ModelType       string          `json:"modelType" db:"model_type"`
	Version         string          `json:"version" db:"version"`
	Algorithm       string          `json:"algorithm" db:"algorithm"`
	Hyperparameters json.RawMessage `json:"hyperparameters,omitempty" db:"hyperparameters"`
	Metrics         json.RawMessage `json:"metrics,omitempty" db:"metrics"`
	Status          string          `json:"status" db:"status"` // training, active, archived
	CreatedBy       int64           `json:"createdBy" db:"created_by"`
	TrainedAt       *time.Time      `json:"trainedAt,omitempty" db:"trained_at"`
	CreatedAt       time.Time       `json:"createdAt" db:"created_at"`
}

// TrainingJob represents a model training job
type TrainingJob struct {
	ID               int64      `json:"id" db:"id"`
	ModelID          int64      `json:"modelId" db:"model_id"`
	JobType          string     `json:"jobType" db:"job_type"`
	DatasetSize      int        `json:"datasetSize" db:"dataset_size"`
	TrainingDuration *int       `json:"trainingDuration,omitempty" db:"training_duration"`
	Status           string     `json:"status" db:"status"` // pending, running, completed, failed
	ErrorMessage     *string    `json:"errorMessage,omitempty" db:"error_message"`
	StartedAt        *time.Time `json:"startedAt,omitempty" db:"started_at"`
	CompletedAt      *time.Time `json:"completedAt,omitempty" db:"completed_at"`
	CreatedAt        time.Time  `json:"createdAt" db:"created_at"`
}

// ABTest represents an A/B test for model comparison
type ABTest struct {
	ID           int64      `json:"id" db:"id"`
	TestName     string     `json:"testName" db:"test_name"`
	ModelAID     int64      `json:"modelAId" db:"model_a_id"`
	ModelBID     int64      `json:"modelBId" db:"model_b_id"`
	TrafficSplit int        `json:"trafficSplit" db:"traffic_split"`
	Status       string     `json:"status" db:"status"` // running, completed, cancelled
	WinnerID     *int64     `json:"winnerId,omitempty" db:"winner_id"`
	StartedAt    time.Time  `json:"startedAt" db:"started_at"`
	CompletedAt  *time.Time `json:"completedAt,omitempty" db:"completed_at"`
}

// MLServiceClient is a client for the Python ML service
type MLServiceClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewMLServiceClient creates a new ML service client
func NewMLServiceClient(baseURL string) *MLServiceClient {
	return &MLServiceClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// PredictFraud calls the Python ML service for fraud prediction
func (c *MLServiceClient) PredictFraud(ctx context.Context, transactionData map[string]interface{}) (*FraudPredictionResult, error) {
	body, err := json.Marshal(transactionData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/v1/score/transaction", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call ML service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ML service returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result FraudPredictionResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// ValidateDocuments calls the Python ML service for document validation
func (c *MLServiceClient) ValidateDocuments(ctx context.Context, documents []Document) (*DocumentValidationResult, error) {
	body, err := json.Marshal(map[string]interface{}{
		"documents": documents,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/validate", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call ML service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ML service returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result DocumentValidationResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// PredictEnrollment calls the Python ML service for enrollment prediction
func (c *MLServiceClient) PredictEnrollment(ctx context.Context, historicalData map[string]interface{}) (*EnrollmentPredictionResult, error) {
	body, err := json.Marshal(historicalData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/predict-enrollment", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call ML service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ML service returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result EnrollmentPredictionResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// Result types

type FraudPredictionResult struct {
	TransactionID    string                   `json:"transaction_id"`
	FraudScore       float64                  `json:"fraud_score"`
	RiskLevel        string                   `json:"risk_level"`
	Decision         string                   `json:"decision"`
	Confidence       float64                  `json:"confidence"`
	ModelVersion     string                   `json:"model_version"`
	Factors          []map[string]interface{} `json:"factors"`
	Explanation      string                   `json:"explanation"`
	ProcessingTimeMs float64                  `json:"processing_time_ms"`
}

type Document struct {
	Type     string `json:"type"`
	FileURL  string `json:"fileURL"`
	FileName string `json:"fileName"`
}

type DocumentValidationResult struct {
	Valid      bool                   `json:"valid"`
	Confidence float64                `json:"confidence"`
	Details    map[string]interface{} `json:"details"`
	Timestamp  string                 `json:"timestamp"`
}

type EnrollmentPredictionResult struct {
	Predictions []float64 `json:"predictions"`
	Trend       string    `json:"trend"`
	Confidence  float64   `json:"confidence"`
	MonthsAhead int       `json:"months_ahead"`
}

// MLPipelineService manages ML models and training
type MLPipelineService struct {
	db       *sql.DB
	mlClient *MLServiceClient
}

// NewMLPipelineService creates a new MLPipelineService
func NewMLPipelineService(db *sql.DB, mlServiceURL string) *MLPipelineService {
	return &MLPipelineService{
		db:       db,
		mlClient: NewMLServiceClient(mlServiceURL),
	}
}

// CreateModel creates a new ML model record
func (s *MLPipelineService) CreateModel(ctx context.Context, name, modelType, algorithm string, hyperparameters map[string]interface{}, userID int64) (*MLModel, error) {
	hyperparamsBytes, err := json.Marshal(hyperparameters)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal hyperparameters: %w", err)
	}

	version := fmt.Sprintf("v%d", time.Now().Unix())

	result, err := s.db.ExecContext(ctx, `
		INSERT INTO ml_models (model_name, model_type, version, algorithm, hyperparameters, status, created_by, created_at)
		VALUES (?, ?, ?, ?, ?, 'training', ?, NOW())
	`, name, modelType, version, algorithm, hyperparamsBytes, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to insert model: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get insert id: %w", err)
	}

	return &MLModel{
		ID:              id,
		ModelName:       name,
		ModelType:       modelType,
		Version:         version,
		Algorithm:       algorithm,
		Hyperparameters: hyperparamsBytes,
		Status:          "training",
		CreatedBy:       userID,
		CreatedAt:       time.Now(),
	}, nil
}

// CreateTrainingJob creates a new training job
func (s *MLPipelineService) CreateTrainingJob(ctx context.Context, modelID int64, jobType string) (*TrainingJob, error) {
	now := time.Now()
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO ml_training_jobs (model_id, job_type, dataset_size, status, started_at, created_at)
		VALUES (?, ?, 0, 'running', ?, ?)
	`, modelID, jobType, now, now)
	if err != nil {
		return nil, fmt.Errorf("failed to insert training job: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get insert id: %w", err)
	}

	return &TrainingJob{
		ID:        id,
		ModelID:   modelID,
		JobType:   jobType,
		Status:    "running",
		StartedAt: &now,
		CreatedAt: now,
	}, nil
}

// UpdateTrainingJob updates a training job
func (s *MLPipelineService) UpdateTrainingJob(ctx context.Context, jobID int64, status string, datasetSize int, duration *int, errorMsg *string) error {
	now := time.Now()
	_, err := s.db.ExecContext(ctx, `
		UPDATE ml_training_jobs 
		SET status = ?, dataset_size = ?, training_duration = ?, error_message = ?, completed_at = ?
		WHERE id = ?
	`, status, datasetSize, duration, errorMsg, now, jobID)
	return err
}

// UpdateModelStatus updates a model's status and metrics
func (s *MLPipelineService) UpdateModelStatus(ctx context.Context, modelID int64, status string, metrics map[string]interface{}) error {
	metricsBytes, err := json.Marshal(metrics)
	if err != nil {
		return fmt.Errorf("failed to marshal metrics: %w", err)
	}

	now := time.Now()
	_, err = s.db.ExecContext(ctx, `
		UPDATE ml_models SET status = ?, metrics = ?, trained_at = ? WHERE id = ?
	`, status, metricsBytes, now, modelID)
	return err
}

// GetActiveModel gets the active model for a given type
func (s *MLPipelineService) GetActiveModel(ctx context.Context, modelType string) (*MLModel, error) {
	model := &MLModel{}
	err := s.db.QueryRowContext(ctx, `
		SELECT id, model_name, model_type, version, algorithm, hyperparameters, metrics, status, created_by, trained_at, created_at
		FROM ml_models 
		WHERE model_type = ? AND status = 'active'
		ORDER BY trained_at DESC LIMIT 1
	`, modelType).Scan(
		&model.ID, &model.ModelName, &model.ModelType, &model.Version, &model.Algorithm,
		&model.Hyperparameters, &model.Metrics, &model.Status, &model.CreatedBy, &model.TrainedAt, &model.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get model: %w", err)
	}
	return model, nil
}

// CreateABTest creates a new A/B test
func (s *MLPipelineService) CreateABTest(ctx context.Context, testName string, modelAID, modelBID int64, trafficSplit int) (*ABTest, error) {
	now := time.Now()
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO ml_ab_tests (test_name, model_a_id, model_b_id, traffic_split, status, started_at)
		VALUES (?, ?, ?, ?, 'running', ?)
	`, testName, modelAID, modelBID, trafficSplit, now)
	if err != nil {
		return nil, fmt.Errorf("failed to insert A/B test: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get insert id: %w", err)
	}

	return &ABTest{
		ID:           id,
		TestName:     testName,
		ModelAID:     modelAID,
		ModelBID:     modelBID,
		TrafficSplit: trafficSplit,
		Status:       "running",
		StartedAt:    now,
	}, nil
}

// Temporal Workflows

// TrainModelWorkflowInput is the input for the training workflow
type TrainModelWorkflowInput struct {
	ModelName       string                 `json:"modelName"`
	ModelType       string                 `json:"modelType"` // fraud_detection, enrollment_prediction
	Algorithm       string                 `json:"algorithm"`
	Hyperparameters map[string]interface{} `json:"hyperparameters"`
	InitiatedBy     int64                  `json:"initiatedBy"`
}

// TrainModelWorkflowResult is the result of the training workflow
type TrainModelWorkflowResult struct {
	ModelID     int64                  `json:"modelId"`
	JobID       int64                  `json:"jobId"`
	Status      string                 `json:"status"`
	Metrics     map[string]interface{} `json:"metrics,omitempty"`
	Error       string                 `json:"error,omitempty"`
	CompletedAt time.Time              `json:"completedAt"`
}

// TrainModelWorkflow orchestrates model training
func TrainModelWorkflow(ctx workflow.Context, input TrainModelWorkflowInput) (*TrainModelWorkflowResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting model training workflow", "modelType", input.ModelType)

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Minute,
		HeartbeatTimeout:    5 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    time.Minute,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	result := &TrainModelWorkflowResult{}

	// Step 1: Create model record
	var model *MLModel
	err := workflow.ExecuteActivity(ctx, CreateModelActivity, input.ModelName, input.ModelType, input.Algorithm, input.Hyperparameters, input.InitiatedBy).Get(ctx, &model)
	if err != nil {
		result.Error = fmt.Sprintf("failed to create model: %v", err)
		result.Status = "failed"
		return result, nil
	}
	result.ModelID = model.ID

	// Step 2: Create training job
	var job *TrainingJob
	err = workflow.ExecuteActivity(ctx, CreateTrainingJobActivity, model.ID, "initial").Get(ctx, &job)
	if err != nil {
		result.Error = fmt.Sprintf("failed to create training job: %v", err)
		result.Status = "failed"
		return result, nil
	}
	result.JobID = job.ID

	// Step 3: Fetch training data
	var datasetSize int
	err = workflow.ExecuteActivity(ctx, FetchTrainingDataActivity, input.ModelType).Get(ctx, &datasetSize)
	if err != nil {
		logger.Warn("Failed to fetch training data", "error", err)
		datasetSize = 0
	}

	// Step 4: Train model (calls Python ML service)
	var metrics map[string]interface{}
	err = workflow.ExecuteActivity(ctx, TrainModelActivity, input.ModelType, input.Hyperparameters, datasetSize).Get(ctx, &metrics)
	if err != nil {
		// Update job as failed
		errMsg := err.Error()
		workflow.ExecuteActivity(ctx, UpdateTrainingJobActivity, job.ID, "failed", datasetSize, nil, &errMsg).Get(ctx, nil)
		result.Error = fmt.Sprintf("training failed: %v", err)
		result.Status = "failed"
		return result, nil
	}

	// Step 5: Update model and job status
	err = workflow.ExecuteActivity(ctx, UpdateModelStatusActivity, model.ID, "active", metrics).Get(ctx, nil)
	if err != nil {
		logger.Warn("Failed to update model status", "error", err)
	}

	duration := 120 // seconds
	err = workflow.ExecuteActivity(ctx, UpdateTrainingJobActivity, job.ID, "completed", datasetSize, &duration, nil).Get(ctx, nil)
	if err != nil {
		logger.Warn("Failed to update training job", "error", err)
	}

	result.Status = "completed"
	result.Metrics = metrics
	result.CompletedAt = workflow.Now(ctx)

	logger.Info("Model training completed", "modelId", model.ID, "metrics", metrics)
	return result, nil
}

// PredictFraudWorkflowInput is the input for fraud prediction
type PredictFraudWorkflowInput struct {
	TransactionID    string                 `json:"transactionId"`
	BeneficiaryID    string                 `json:"beneficiaryId"`
	ProgramID        string                 `json:"programId"`
	Amount           float64                `json:"amount"`
	TransactionType  string                 `json:"transactionType"`
	TransactionData  map[string]interface{} `json:"transactionData"`
}

// PredictFraudWorkflow orchestrates fraud prediction
func PredictFraudWorkflow(ctx workflow.Context, input PredictFraudWorkflowInput) (*FraudPredictionResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting fraud prediction workflow", "transactionId", input.TransactionID)

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    10 * time.Second,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Call Python ML service for prediction
	var result *FraudPredictionResult
	err := workflow.ExecuteActivity(ctx, PredictFraudActivity, input.TransactionData).Get(ctx, &result)
	if err != nil {
		logger.Error("Fraud prediction failed", "error", err)
		// Return default safe result on failure
		return &FraudPredictionResult{
			TransactionID: input.TransactionID,
			FraudScore:    0,
			RiskLevel:     "unknown",
			Decision:      "review",
			Confidence:    0,
			Explanation:   "Prediction service unavailable",
		}, nil
	}

	return result, nil
}

// ABTestWorkflowInput is the input for A/B test workflow
type ABTestWorkflowInput struct {
	TestName     string `json:"testName"`
	ModelAID     int64  `json:"modelAId"`
	ModelBID     int64  `json:"modelBId"`
	TrafficSplit int    `json:"trafficSplit"`
	DurationDays int    `json:"durationDays"`
}

// ABTestWorkflowResult is the result of A/B test workflow
type ABTestWorkflowResult struct {
	TestID     int64                  `json:"testId"`
	WinnerID   int64                  `json:"winnerId"`
	ModelAStats map[string]interface{} `json:"modelAStats"`
	ModelBStats map[string]interface{} `json:"modelBStats"`
	Confidence float64                `json:"confidence"`
}

// ABTestWorkflow orchestrates A/B testing
func ABTestWorkflow(ctx workflow.Context, input ABTestWorkflowInput) (*ABTestWorkflowResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting A/B test workflow", "testName", input.TestName)

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    time.Minute,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Create A/B test
	var test *ABTest
	err := workflow.ExecuteActivity(ctx, CreateABTestActivity, input.TestName, input.ModelAID, input.ModelBID, input.TrafficSplit).Get(ctx, &test)
	if err != nil {
		return nil, fmt.Errorf("failed to create A/B test: %w", err)
	}

	// Wait for test duration
	duration := time.Duration(input.DurationDays) * 24 * time.Hour
	if err := workflow.Sleep(ctx, duration); err != nil {
		return nil, fmt.Errorf("test interrupted: %w", err)
	}

	// Collect and analyze results
	var results *ABTestWorkflowResult
	err = workflow.ExecuteActivity(ctx, AnalyzeABTestActivity, test.ID).Get(ctx, &results)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze A/B test: %w", err)
	}

	// Update test with winner
	err = workflow.ExecuteActivity(ctx, CompleteABTestActivity, test.ID, results.WinnerID).Get(ctx, nil)
	if err != nil {
		logger.Warn("Failed to complete A/B test", "error", err)
	}

	return results, nil
}

// Activity implementations

type MLActivities struct {
	service *MLPipelineService
}

func NewMLActivities(service *MLPipelineService) *MLActivities {
	return &MLActivities{service: service}
}

func CreateModelActivity(ctx context.Context, name, modelType, algorithm string, hyperparameters map[string]interface{}, userID int64) (*MLModel, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Creating ML model", "name", name, "type", modelType)

	service := getMLServiceFromContext(ctx)
	if service == nil {
		return nil, fmt.Errorf("ML service not available")
	}

	return service.CreateModel(ctx, name, modelType, algorithm, hyperparameters, userID)
}

func CreateTrainingJobActivity(ctx context.Context, modelID int64, jobType string) (*TrainingJob, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Creating training job", "modelId", modelID)

	service := getMLServiceFromContext(ctx)
	if service == nil {
		return nil, fmt.Errorf("ML service not available")
	}

	return service.CreateTrainingJob(ctx, modelID, jobType)
}

func FetchTrainingDataActivity(ctx context.Context, modelType string) (int, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Fetching training data", "modelType", modelType)

	service := getMLServiceFromContext(ctx)
	if service == nil {
		return 0, fmt.Errorf("ML service not available")
	}

	// Query database for training data count
	var count int
	var err error
	switch modelType {
	case "fraud_detection":
		err = service.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM transactions").Scan(&count)
	case "enrollment_prediction":
		err = service.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM beneficiaries").Scan(&count)
	default:
		count = 0
	}

	if err != nil {
		return 0, fmt.Errorf("failed to fetch training data count: %w", err)
	}

	return count, nil
}

func TrainModelActivity(ctx context.Context, modelType string, hyperparameters map[string]interface{}, datasetSize int) (map[string]interface{}, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Training model", "modelType", modelType, "datasetSize", datasetSize)

	// Simulate training metrics (in production, call Python ML service for actual training)
	metrics := map[string]interface{}{
		"trainingSize": datasetSize,
	}

	switch modelType {
	case "fraud_detection":
		metrics["accuracy"] = 0.92
		metrics["precision"] = 0.89
		metrics["recall"] = 0.87
		metrics["f1Score"] = 0.88
	case "enrollment_prediction":
		metrics["mae"] = 12.5
		metrics["rmse"] = 18.3
		metrics["r2Score"] = 0.85
	}

	return metrics, nil
}

func UpdateModelStatusActivity(ctx context.Context, modelID int64, status string, metrics map[string]interface{}) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Updating model status", "modelId", modelID, "status", status)

	service := getMLServiceFromContext(ctx)
	if service == nil {
		return fmt.Errorf("ML service not available")
	}

	return service.UpdateModelStatus(ctx, modelID, status, metrics)
}

func UpdateTrainingJobActivity(ctx context.Context, jobID int64, status string, datasetSize int, duration *int, errorMsg *string) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Updating training job", "jobId", jobID, "status", status)

	service := getMLServiceFromContext(ctx)
	if service == nil {
		return fmt.Errorf("ML service not available")
	}

	return service.UpdateTrainingJob(ctx, jobID, status, datasetSize, duration, errorMsg)
}

func PredictFraudActivity(ctx context.Context, transactionData map[string]interface{}) (*FraudPredictionResult, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Predicting fraud")

	service := getMLServiceFromContext(ctx)
	if service == nil {
		return nil, fmt.Errorf("ML service not available")
	}

	return service.mlClient.PredictFraud(ctx, transactionData)
}

func CreateABTestActivity(ctx context.Context, testName string, modelAID, modelBID int64, trafficSplit int) (*ABTest, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Creating A/B test", "testName", testName)

	service := getMLServiceFromContext(ctx)
	if service == nil {
		return nil, fmt.Errorf("ML service not available")
	}

	return service.CreateABTest(ctx, testName, modelAID, modelBID, trafficSplit)
}

func AnalyzeABTestActivity(ctx context.Context, testID int64) (*ABTestWorkflowResult, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Analyzing A/B test", "testId", testID)

	// Simulate A/B test analysis
	return &ABTestWorkflowResult{
		TestID:   testID,
		WinnerID: 2, // Model B wins
		ModelAStats: map[string]interface{}{
			"requests":   5000,
			"accuracy":   0.92,
			"avgLatency": 45,
		},
		ModelBStats: map[string]interface{}{
			"requests":   5000,
			"accuracy":   0.94,
			"avgLatency": 52,
		},
		Confidence: 0.95,
	}, nil
}

func CompleteABTestActivity(ctx context.Context, testID, winnerID int64) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Completing A/B test", "testId", testID, "winnerId", winnerID)

	service := getMLServiceFromContext(ctx)
	if service == nil {
		return fmt.Errorf("ML service not available")
	}

	now := time.Now()
	_, err := service.db.ExecContext(ctx, `
		UPDATE ml_ab_tests SET status = 'completed', winner_id = ?, completed_at = ? WHERE id = ?
	`, winnerID, now, testID)
	return err
}

// Context key for ML service
type mlServiceKey struct{}

func WithMLService(ctx context.Context, service *MLPipelineService) context.Context {
	return context.WithValue(ctx, mlServiceKey{}, service)
}

func getMLServiceFromContext(ctx context.Context) *MLPipelineService {
	service, _ := ctx.Value(mlServiceKey{}).(*MLPipelineService)
	return service
}

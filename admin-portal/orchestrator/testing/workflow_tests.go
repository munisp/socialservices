package testing

import (
	"context"
	"fmt"
	"testing"
	"time"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

// ========== Mock Activity Implementations ==========

// Mock activities for testing workflows without external dependencies

type MockActivities struct {
	// Configuration for controlling mock behavior
	FailureRate      float64
	DelayMs          int
	SimulateTimeouts bool
}

func NewMockActivities() *MockActivities {
	return &MockActivities{
		FailureRate: 0.0,
		DelayMs:     100,
	}
}

// Journey 1: Enrollment Mock Activities
func (m *MockActivities) ValidateBeneficiaryDataActivity(ctx context.Context, data map[string]interface{}) (bool, error) {
	time.Sleep(time.Duration(m.DelayMs) * time.Millisecond)
	return true, nil
}

func (m *MockActivities) CreateBeneficiaryRecordActivity(ctx context.Context, data map[string]interface{}) (string, error) {
	time.Sleep(time.Duration(m.DelayMs) * time.Millisecond)
	return fmt.Sprintf("BEN-%d", time.Now().Unix()), nil
}

func (m *MockActivities) AssignBeneficiaryIDActivity(ctx context.Context, beneficiaryID string) error {
	time.Sleep(time.Duration(m.DelayMs) * time.Millisecond)
	return nil
}

// Journey 2: KYC Mock Activities
func (m *MockActivities) VerifyIdentityDocumentActivity(ctx context.Context, documentType, documentNumber string) (bool, error) {
	time.Sleep(time.Duration(m.DelayMs) * time.Millisecond)
	return true, nil
}

func (m *MockActivities) PerformBiometricVerificationActivity(ctx context.Context, beneficiaryID string, biometricData map[string]interface{}) (bool, error) {
	time.Sleep(time.Duration(m.DelayMs) * time.Millisecond)
	return true, nil
}

// Journey 11: Disbursement Mock Activities
func (m *MockActivities) ValidateProgramBudgetActivity(ctx context.Context, programID string, amount float64) (bool, error) {
	time.Sleep(time.Duration(m.DelayMs) * time.Millisecond)
	return true, nil
}

func (m *MockActivities) CreateDisbursementBatchActivity(ctx context.Context, disbursementID, programID string, count int, amount float64) (string, error) {
	time.Sleep(time.Duration(m.DelayMs) * time.Millisecond)
	return fmt.Sprintf("BATCH-%d", time.Now().Unix()), nil
}

func (m *MockActivities) ExecuteTigerBeetleTransferActivity(ctx context.Context, fromAccount, toAccount string, amount float64) (string, error) {
	time.Sleep(time.Duration(m.DelayMs) * time.Millisecond)
	return fmt.Sprintf("TX-%d", time.Now().Unix()), nil
}

func (m *MockActivities) ReconcileDisbursementActivity(ctx context.Context, batchID string, transactionIDs []string) (string, error) {
	time.Sleep(time.Duration(m.DelayMs) * time.Millisecond)
	return fmt.Sprintf("RECON-%d", time.Now().Unix()), nil
}

// Journey 12: Grievance Mock Activities
func (m *MockActivities) GenerateCaseNumberActivity(ctx context.Context, category string) (string, error) {
	time.Sleep(time.Duration(m.DelayMs) * time.Millisecond)
	return fmt.Sprintf("CASE-%s-%d", category, time.Now().Unix()), nil
}

func (m *MockActivities) CreateGrievanceRecordActivity(ctx context.Context, grievanceID, caseNumber, beneficiaryID, category, description, priority string) error {
	time.Sleep(time.Duration(m.DelayMs) * time.Millisecond)
	return nil
}

func (m *MockActivities) AssignGrievanceActivity(ctx context.Context, grievanceID, category, priority string) (string, error) {
	time.Sleep(time.Duration(m.DelayMs) * time.Millisecond)
	return "caseworker-001", nil
}

// Journey 13: Fraud Investigation Mock Activities
func (m *MockActivities) GenerateFraudCaseNumberActivity(ctx context.Context, fraudType string) (string, error) {
	time.Sleep(time.Duration(m.DelayMs) * time.Millisecond)
	return fmt.Sprintf("FRAUD-%s-%d", fraudType, time.Now().Unix()), nil
}

func (m *MockActivities) RunMLFraudAnalysisActivity(ctx context.Context, beneficiaryID string, evidenceData map[string]interface{}) (map[string]interface{}, error) {
	time.Sleep(time.Duration(m.DelayMs) * time.Millisecond)
	return map[string]interface{}{
		"risk_score":          0.75,
		"suspicious_patterns": true,
		"confidence":          0.85,
	}, nil
}

func (m *MockActivities) SuspendBeneficiaryAccountActivity(ctx context.Context, beneficiaryID, reason string) error {
	time.Sleep(time.Duration(m.DelayMs) * time.Millisecond)
	return nil
}

// Journey 14: Reporting Mock Activities
func (m *MockActivities) CollectLakehouseDataActivity(ctx context.Context, reportType, month string, programIDs []string) (map[string]interface{}, error) {
	time.Sleep(time.Duration(m.DelayMs) * time.Millisecond)
	return map[string]interface{}{
		"total_disbursements": 1000000,
		"total_beneficiaries": 5000,
		"success_rate":        0.98,
	}, nil
}

func (m *MockActivities) GeneratePDFReportActivity(ctx context.Context, reportID, reportType, month string, metrics map[string]interface{}, visualizations []string) (string, error) {
	time.Sleep(time.Duration(m.DelayMs) * time.Millisecond)
	return fmt.Sprintf("https://storage.example.com/reports/%s.pdf", reportID), nil
}

// Journey 15: Analytics Mock Activities
func (m *MockActivities) RunMLProgramAnalysisActivity(ctx context.Context, programMetrics interface{}, startDate, endDate string) ([]interface{}, error) {
	time.Sleep(time.Duration(m.DelayMs) * time.Millisecond)
	return []interface{}{
		map[string]interface{}{
			"type":        "trend",
			"description": "Enrollment increasing by 15% month-over-month",
			"impact":      "positive",
			"confidence":  0.92,
		},
	}, nil
}

// Generic Mock Activities
func (m *MockActivities) PublishKafkaEventActivity(ctx context.Context, topic, key string, value interface{}) error {
	time.Sleep(time.Duration(m.DelayMs) * time.Millisecond)
	return nil
}

func (m *MockActivities) CreateAuditLogActivity(ctx context.Context, action, entityID, performedBy string, details interface{}) error {
	time.Sleep(time.Duration(m.DelayMs) * time.Millisecond)
	return nil
}

func (m *MockActivities) SendNotificationActivity(ctx context.Context, recipientID, message string) error {
	time.Sleep(time.Duration(m.DelayMs) * time.Millisecond)
	return nil
}

// ========== Workflow Test Helpers ==========

type WorkflowTestSuite struct {
	client         client.Client
	worker         worker.Worker
	mockActivities *MockActivities
	ctx            context.Context
}

func NewWorkflowTestSuite(t *testing.T) *WorkflowTestSuite {
	// Create Temporal test client
	c, err := client.Dial(client.Options{
		HostPort: "localhost:7233",
	})
	if err != nil {
		t.Fatalf("Failed to create Temporal client: %v", err)
	}

	mockActivities := NewMockActivities()

	// Create worker for test task queue
	w := worker.New(c, "test-task-queue", worker.Options{})

	// Register mock activities
	w.RegisterActivity(mockActivities)

	return &WorkflowTestSuite{
		client:         c,
		worker:         w,
		mockActivities: mockActivities,
		ctx:            context.Background(),
	}
}

func (suite *WorkflowTestSuite) Close() {
	suite.worker.Stop()
	suite.client.Close()
}

// ========== Unit Tests ==========

func TestEnrollmentWorkflow(t *testing.T) {
	suite := NewWorkflowTestSuite(t)
	defer suite.Close()

	// Start worker
	err := suite.worker.Start()
	if err != nil {
		t.Fatalf("Failed to start worker: %v", err)
	}

	// Execute workflow
	workflowOptions := client.StartWorkflowOptions{
		ID:        fmt.Sprintf("test-enrollment-%d", time.Now().Unix()),
		TaskQueue: "test-task-queue",
	}

	input := map[string]interface{}{
		"enrollmentID": "TEST-001",
		"name":         "Test Beneficiary",
		"nationalID":   "123456789",
		"phoneNumber":  "+1234567890",
	}

	we, err := suite.client.ExecuteWorkflow(suite.ctx, workflowOptions, "EnrollmentWorkflow", input)
	if err != nil {
		t.Fatalf("Failed to execute workflow: %v", err)
	}

	// Wait for workflow to complete
	var result interface{}
	err = we.Get(suite.ctx, &result)
	if err != nil {
		t.Fatalf("Workflow execution failed: %v", err)
	}

	t.Logf("Enrollment workflow completed: %+v", result)
}

func TestDisbursementWorkflow(t *testing.T) {
	suite := NewWorkflowTestSuite(t)
	defer suite.Close()

	err := suite.worker.Start()
	if err != nil {
		t.Fatalf("Failed to start worker: %v", err)
	}

	workflowOptions := client.StartWorkflowOptions{
		ID:        fmt.Sprintf("test-disbursement-%d", time.Now().Unix()),
		TaskQueue: "test-task-queue",
	}

	input := map[string]interface{}{
		"disbursementID": "DISB-001",
		"programID":      "PROG-001",
		"beneficiaryIDs": []string{"BEN-001", "BEN-002", "BEN-003"},
		"amount":         1000.0,
		"initiatedBy":    "admin",
	}

	we, err := suite.client.ExecuteWorkflow(suite.ctx, workflowOptions, "DisbursementProcessingWorkflow", input)
	if err != nil {
		t.Fatalf("Failed to execute workflow: %v", err)
	}

	var result interface{}
	err = we.Get(suite.ctx, &result)
	if err != nil {
		t.Fatalf("Workflow execution failed: %v", err)
	}

	t.Logf("Disbursement workflow completed: %+v", result)
}

// ========== Performance Benchmarks ==========

func BenchmarkEnrollmentWorkflow(b *testing.B) {
	suite := NewWorkflowTestSuite(&testing.T{})
	defer suite.Close()

	suite.worker.Start()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		workflowOptions := client.StartWorkflowOptions{
			ID:        fmt.Sprintf("bench-enrollment-%d", i),
			TaskQueue: "test-task-queue",
		}

		input := map[string]interface{}{
			"enrollmentID": fmt.Sprintf("BENCH-%d", i),
			"name":         "Benchmark Beneficiary",
			"nationalID":   "123456789",
		}

		we, _ := suite.client.ExecuteWorkflow(suite.ctx, workflowOptions, "EnrollmentWorkflow", input)
		we.Get(suite.ctx, nil)
	}
}

func BenchmarkDisbursementWorkflow(b *testing.B) {
	suite := NewWorkflowTestSuite(&testing.T{})
	defer suite.Close()

	suite.worker.Start()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		workflowOptions := client.StartWorkflowOptions{
			ID:        fmt.Sprintf("bench-disbursement-%d", i),
			TaskQueue: "test-task-queue",
		}

		input := map[string]interface{}{
			"disbursementID": fmt.Sprintf("DISB-%d", i),
			"programID":      "PROG-001",
			"beneficiaryIDs": []string{"BEN-001", "BEN-002"},
			"amount":         1000.0,
		}

		we, _ := suite.client.ExecuteWorkflow(suite.ctx, workflowOptions, "DisbursementProcessingWorkflow", input)
		we.Get(suite.ctx, nil)
	}
}

// ========== Integration Tests ==========

func TestEndToEndBeneficiaryLifecycle(t *testing.T) {
	suite := NewWorkflowTestSuite(t)
	defer suite.Close()

	suite.worker.Start()

	// Step 1: Enrollment
	enrollmentOptions := client.StartWorkflowOptions{
		ID:        fmt.Sprintf("e2e-enrollment-%d", time.Now().Unix()),
		TaskQueue: "test-task-queue",
	}

	enrollmentInput := map[string]interface{}{
		"enrollmentID": "E2E-001",
		"name":         "End-to-End Test Beneficiary",
		"nationalID":   "987654321",
	}

	enrollmentWE, err := suite.client.ExecuteWorkflow(suite.ctx, enrollmentOptions, "EnrollmentWorkflow", enrollmentInput)
	if err != nil {
		t.Fatalf("Enrollment failed: %v", err)
	}

	var enrollmentResult map[string]interface{}
	err = enrollmentWE.Get(suite.ctx, &enrollmentResult)
	if err != nil {
		t.Fatalf("Enrollment execution failed: %v", err)
	}

	beneficiaryID := enrollmentResult["beneficiaryID"].(string)
	t.Logf("Beneficiary enrolled: %s", beneficiaryID)

	// Step 2: KYC Verification
	kycOptions := client.StartWorkflowOptions{
		ID:        fmt.Sprintf("e2e-kyc-%d", time.Now().Unix()),
		TaskQueue: "test-task-queue",
	}

	kycInput := map[string]interface{}{
		"verificationID": "KYC-E2E-001",
		"beneficiaryID":  beneficiaryID,
		"documentType":   "national_id",
		"documentNumber": "987654321",
	}

	kycWE, err := suite.client.ExecuteWorkflow(suite.ctx, kycOptions, "KYCVerificationWorkflow", kycInput)
	if err != nil {
		t.Fatalf("KYC failed: %v", err)
	}

	var kycResult map[string]interface{}
	err = kycWE.Get(suite.ctx, &kycResult)
	if err != nil {
		t.Fatalf("KYC execution failed: %v", err)
	}

	t.Logf("KYC completed: %+v", kycResult)

	// Step 3: Disbursement
	disbursementOptions := client.StartWorkflowOptions{
		ID:        fmt.Sprintf("e2e-disbursement-%d", time.Now().Unix()),
		TaskQueue: "test-task-queue",
	}

	disbursementInput := map[string]interface{}{
		"disbursementID": "DISB-E2E-001",
		"programID":      "PROG-001",
		"beneficiaryIDs": []string{beneficiaryID},
		"amount":         500.0,
		"initiatedBy":    "test-admin",
	}

	disbursementWE, err := suite.client.ExecuteWorkflow(suite.ctx, disbursementOptions, "DisbursementProcessingWorkflow", disbursementInput)
	if err != nil {
		t.Fatalf("Disbursement failed: %v", err)
	}

	var disbursementResult map[string]interface{}
	err = disbursementWE.Get(suite.ctx, &disbursementResult)
	if err != nil {
		t.Fatalf("Disbursement execution failed: %v", err)
	}

	t.Logf("Disbursement completed: %+v", disbursementResult)
	t.Log("End-to-end beneficiary lifecycle test completed successfully")
}

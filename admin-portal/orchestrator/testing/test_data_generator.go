package testing

import (
	"fmt"
	"math/rand"
	"time"
)

// ========== Test Data Generators ==========

type TestDataGenerator struct {
	rand *rand.Rand
}

func NewTestDataGenerator() *TestDataGenerator {
	return &TestDataGenerator{
		rand: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// Generate random beneficiary data
func (g *TestDataGenerator) GenerateBeneficiary() map[string]interface{} {
	id := g.rand.Intn(1000000)
	return map[string]interface{}{
		"enrollmentID": fmt.Sprintf("ENR-%06d", id),
		"name":         fmt.Sprintf("Test Beneficiary %d", id),
		"nationalID":   fmt.Sprintf("%09d", g.rand.Intn(1000000000)),
		"phoneNumber":  fmt.Sprintf("+1%010d", g.rand.Int63n(10000000000)),
		"email":        fmt.Sprintf("beneficiary%d@test.com", id),
		"dateOfBirth":  "1990-01-01",
		"address":      fmt.Sprintf("%d Test Street", g.rand.Intn(1000)),
		"city":         "Test City",
		"state":        "Test State",
		"zipCode":      fmt.Sprintf("%05d", g.rand.Intn(100000)),
	}
}

// Generate random KYC verification data
func (g *TestDataGenerator) GenerateKYCData(beneficiaryID string) map[string]interface{} {
	return map[string]interface{}{
		"verificationID": fmt.Sprintf("KYC-%06d", g.rand.Intn(1000000)),
		"beneficiaryID":  beneficiaryID,
		"documentType":   g.randomChoice([]string{"national_id", "passport", "drivers_license"}),
		"documentNumber": fmt.Sprintf("%09d", g.rand.Intn(1000000000)),
		"biometricData": map[string]interface{}{
			"fingerprint": fmt.Sprintf("FP-%06d", g.rand.Intn(1000000)),
			"faceID":      fmt.Sprintf("FACE-%06d", g.rand.Intn(1000000)),
		},
	}
}

// Generate random disbursement data
func (g *TestDataGenerator) GenerateDisbursement(programID string, beneficiaryIDs []string) map[string]interface{} {
	return map[string]interface{}{
		"disbursementID": fmt.Sprintf("DISB-%06d", g.rand.Intn(1000000)),
		"programID":      programID,
		"beneficiaryIDs": beneficiaryIDs,
		"amount":         float64(g.rand.Intn(1000) + 100),
		"scheduledDate":  time.Now().Add(time.Duration(g.rand.Intn(30)) * 24 * time.Hour).Format("2006-01-02"),
		"disbursementType": g.randomChoice([]string{"regular", "emergency", "supplemental"}),
		"initiatedBy":    "test-admin",
	}
}

// Generate random grievance data
func (g *TestDataGenerator) GenerateGrievance(beneficiaryID string) map[string]interface{} {
	return map[string]interface{}{
		"grievanceID":   fmt.Sprintf("GRIEV-%06d", g.rand.Intn(1000000)),
		"beneficiaryID": beneficiaryID,
		"category":      g.randomChoice([]string{"payment", "service", "eligibility", "fraud", "other"}),
		"description":   "Test grievance description",
		"priority":      g.randomChoice([]string{"low", "medium", "high", "critical"}),
		"attachments":   []string{},
		"submittedBy":   beneficiaryID,
	}
}

// Generate random fraud investigation data
func (g *TestDataGenerator) GenerateFraudInvestigation(beneficiaryID string) map[string]interface{} {
	return map[string]interface{}{
		"investigationID": fmt.Sprintf("FRAUD-%06d", g.rand.Intn(1000000)),
		"alertID":         fmt.Sprintf("ALERT-%06d", g.rand.Intn(1000000)),
		"beneficiaryID":   beneficiaryID,
		"fraudType":       g.randomChoice([]string{"duplicate_identity", "fake_documents", "transaction_fraud", "collusion"}),
		"severity":        g.randomChoice([]string{"low", "medium", "high", "critical"}),
		"evidence":        []string{"evidence1.pdf", "evidence2.pdf"},
		"initiatedBy":     "fraud-detection-system",
	}
}

// Generate random report request
func (g *TestDataGenerator) GenerateReportRequest(reportType string) map[string]interface{} {
	return map[string]interface{}{
		"reportID":    fmt.Sprintf("RPT-%s-%06d", reportType, g.rand.Intn(1000000)),
		"reportType":  reportType,
		"month":       time.Now().AddDate(0, -1, 0).Format("2006-01"),
		"programIDs":  []string{"PROG-001", "PROG-002"},
		"recipients":  []string{"admin@test.com", "manager@test.com"},
		"generatedBy": "test-admin",
	}
}

// Generate random analytics request
func (g *TestDataGenerator) GenerateAnalyticsRequest() map[string]interface{} {
	return map[string]interface{}{
		"analyticsID": fmt.Sprintf("ANALYTICS-%06d", g.rand.Intn(1000000)),
		"programIDs":  []string{"PROG-001", "PROG-002", "PROG-003"},
		"timeRange":   g.randomChoice([]string{"last_7_days", "last_30_days", "last_90_days"}),
		"metrics": []string{
			"enrollment_rate",
			"disbursement_volume",
			"fraud_rate",
			"beneficiary_satisfaction",
		},
		"requestedBy": "test-admin",
	}
}

// Generate batch of beneficiaries
func (g *TestDataGenerator) GenerateBeneficiaryBatch(count int) []map[string]interface{} {
	batch := make([]map[string]interface{}, count)
	for i := 0; i < count; i++ {
		batch[i] = g.GenerateBeneficiary()
	}
	return batch
}

// Generate workflow execution scenario
type WorkflowScenario struct {
	Name        string
	Description string
	Steps       []ScenarioStep
}

type ScenarioStep struct {
	WorkflowName string
	Input        map[string]interface{}
	ExpectedResult string
}

func (g *TestDataGenerator) GenerateTestScenarios() []WorkflowScenario {
	beneficiaryData := g.GenerateBeneficiary()
	beneficiaryID := "BEN-TEST-001"

	return []WorkflowScenario{
		{
			Name:        "Happy Path: Complete Beneficiary Lifecycle",
			Description: "Test successful enrollment, KYC, and disbursement",
			Steps: []ScenarioStep{
				{
					WorkflowName: "EnrollmentWorkflow",
					Input:        beneficiaryData,
					ExpectedResult: "completed",
				},
				{
					WorkflowName: "KYCVerificationWorkflow",
					Input:        g.GenerateKYCData(beneficiaryID),
					ExpectedResult: "verified",
				},
				{
					WorkflowName: "DisbursementProcessingWorkflow",
					Input:        g.GenerateDisbursement("PROG-001", []string{beneficiaryID}),
					ExpectedResult: "completed",
				},
			},
		},
		{
			Name:        "Fraud Detection Path",
			Description: "Test fraud detection and investigation workflow",
			Steps: []ScenarioStep{
				{
					WorkflowName: "EnrollmentWorkflow",
					Input:        beneficiaryData,
					ExpectedResult: "completed",
				},
				{
					WorkflowName: "FraudInvestigationWorkflow",
					Input:        g.GenerateFraudInvestigation(beneficiaryID),
					ExpectedResult: "closed",
				},
			},
		},
		{
			Name:        "Grievance Resolution Path",
			Description: "Test grievance submission and resolution",
			Steps: []ScenarioStep{
				{
					WorkflowName: "GrievanceSubmissionWorkflow",
					Input:        g.GenerateGrievance(beneficiaryID),
					ExpectedResult: "open",
				},
				{
					WorkflowName: "GrievanceResolutionWorkflow",
					Input: map[string]interface{}{
						"grievanceID":    "GRIEV-001",
						"resolution":     "Issue resolved",
						"resolutionType": "resolved",
						"resolvedBy":     "caseworker-001",
					},
					ExpectedResult: "resolved",
				},
			},
		},
		{
			Name:        "Reporting and Analytics Path",
			Description: "Test report generation and analytics",
			Steps: []ScenarioStep{
				{
					WorkflowName: "MonthlyReportingWorkflow",
					Input:        g.GenerateReportRequest("disbursement"),
					ExpectedResult: "completed",
				},
				{
					WorkflowName: "ProgramPerformanceAnalyticsWorkflow",
					Input:        g.GenerateAnalyticsRequest(),
					ExpectedResult: "completed",
				},
			},
		},
	}
}

// Helper function to randomly choose from a slice
func (g *TestDataGenerator) randomChoice(choices []string) string {
	return choices[g.rand.Intn(len(choices))]
}

// Generate load test data
type LoadTestConfig struct {
	ConcurrentWorkflows int
	TotalWorkflows      int
	WorkflowType        string
	Duration            time.Duration
}

func (g *TestDataGenerator) GenerateLoadTestConfig(workflowType string, concurrent, total int) LoadTestConfig {
	return LoadTestConfig{
		ConcurrentWorkflows: concurrent,
		TotalWorkflows:      total,
		WorkflowType:        workflowType,
		Duration:            time.Duration(total/concurrent) * time.Second,
	}
}

// Performance test metrics
type PerformanceMetrics struct {
	TotalExecutions    int
	SuccessfulExecutions int
	FailedExecutions   int
	AverageDuration    time.Duration
	MinDuration        time.Duration
	MaxDuration        time.Duration
	P50Duration        time.Duration
	P95Duration        time.Duration
	P99Duration        time.Duration
	Throughput         float64 // workflows per second
}

func (m *PerformanceMetrics) CalculateThroughput(totalDuration time.Duration) {
	m.Throughput = float64(m.TotalExecutions) / totalDuration.Seconds()
}

func (m *PerformanceMetrics) String() string {
	return fmt.Sprintf(
		"Performance Metrics:\n"+
			"  Total Executions: %d\n"+
			"  Successful: %d (%.2f%%)\n"+
			"  Failed: %d (%.2f%%)\n"+
			"  Average Duration: %v\n"+
			"  Min Duration: %v\n"+
			"  Max Duration: %v\n"+
			"  P50 Duration: %v\n"+
			"  P95 Duration: %v\n"+
			"  P99 Duration: %v\n"+
			"  Throughput: %.2f workflows/sec",
		m.TotalExecutions,
		m.SuccessfulExecutions,
		float64(m.SuccessfulExecutions)/float64(m.TotalExecutions)*100,
		m.FailedExecutions,
		float64(m.FailedExecutions)/float64(m.TotalExecutions)*100,
		m.AverageDuration,
		m.MinDuration,
		m.MaxDuration,
		m.P50Duration,
		m.P95Duration,
		m.P99Duration,
		m.Throughput,
	)
}

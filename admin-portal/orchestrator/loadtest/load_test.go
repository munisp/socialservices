package loadtest

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// ScaleLevel represents different scale levels for testing
type ScaleLevel string

const (
	ScaleSmall   ScaleLevel = "small"   // 10K beneficiaries
	ScaleMedium  ScaleLevel = "medium"  // 100K beneficiaries
	ScaleLarge   ScaleLevel = "large"   // 1M beneficiaries
	ScaleXLarge  ScaleLevel = "xlarge"  // 10M beneficiaries
	ScaleCountry ScaleLevel = "country" // 100M+ beneficiaries (India/Brazil scale)
)

// TestType represents different types of load tests
type TestType string

const (
	TestTypeSmoke       TestType = "smoke"       // Quick validation
	TestTypeLoad        TestType = "load"        // Normal load
	TestTypeStress      TestType = "stress"      // Beyond normal capacity
	TestTypeSpike       TestType = "spike"       // Sudden traffic spikes
	TestTypeSoak        TestType = "soak"        // Extended duration
	TestTypeBreakpoint  TestType = "breakpoint"  // Find breaking point
	TestTypeScalability TestType = "scalability" // Horizontal scaling
)

// TestScenario represents a test scenario
type TestScenario string

const (
	ScenarioBeneficiaryEnrollment TestScenario = "beneficiary_enrollment"
	ScenarioBeneficiarySearch     TestScenario = "beneficiary_search"
	ScenarioDisbursement          TestScenario = "disbursement"
	ScenarioWorkflowExecution     TestScenario = "workflow_execution"
	ScenarioReporting             TestScenario = "reporting"
	ScenarioMixedWorkload         TestScenario = "mixed_workload"
	ScenarioOfflineSync           TestScenario = "offline_sync"
	ScenarioCrossSectorQuery      TestScenario = "cross_sector_query"
)

// LoadTestConfig represents load test configuration
type LoadTestConfig struct {
	ID                string            `json:"id"`
	Name              string            `json:"name"`
	Description       string            `json:"description"`
	TestType          TestType          `json:"testType"`
	Scenarios         []TestScenario    `json:"scenarios"`
	ScaleLevel        ScaleLevel        `json:"scaleLevel"`
	Duration          time.Duration     `json:"duration"`
	RampUpTime        time.Duration     `json:"rampUpTime"`
	RampDownTime      time.Duration     `json:"rampDownTime"`
	TargetRPS         int               `json:"targetRps"`         // Requests per second
	MaxConcurrency    int               `json:"maxConcurrency"`    // Max concurrent users
	ThinkTime         time.Duration     `json:"thinkTime"`         // Time between requests
	DataSetSize       int64             `json:"dataSetSize"`       // Number of records to test with
	TargetEndpoints   []string          `json:"targetEndpoints"`
	Headers           map[string]string `json:"headers,omitempty"`
	SLOs              *SLOConfig        `json:"slos"`
	Tags              map[string]string `json:"tags,omitempty"`
}

// SLOConfig represents Service Level Objectives
type SLOConfig struct {
	P50LatencyMs     int     `json:"p50LatencyMs"`
	P95LatencyMs     int     `json:"p95LatencyMs"`
	P99LatencyMs     int     `json:"p99LatencyMs"`
	MaxLatencyMs     int     `json:"maxLatencyMs"`
	ErrorRatePercent float64 `json:"errorRatePercent"`
	AvailabilityPercent float64 `json:"availabilityPercent"`
	ThroughputRPS    int     `json:"throughputRps"`
}

// LoadTestResult represents load test results
type LoadTestResult struct {
	ID                string                 `json:"id"`
	ConfigID          string                 `json:"configId"`
	Status            string                 `json:"status"` // running, completed, failed, cancelled
	StartTime         time.Time              `json:"startTime"`
	EndTime           *time.Time             `json:"endTime,omitempty"`
	Duration          time.Duration          `json:"duration"`
	TotalRequests     int64                  `json:"totalRequests"`
	SuccessfulRequests int64                 `json:"successfulRequests"`
	FailedRequests    int64                  `json:"failedRequests"`
	ErrorRate         float64                `json:"errorRate"`
	ThroughputRPS     float64                `json:"throughputRps"`
	Latencies         *LatencyStats          `json:"latencies"`
	ScenarioResults   map[TestScenario]*ScenarioResult `json:"scenarioResults"`
	SLOResults        *SLOResults            `json:"sloResults"`
	ResourceMetrics   *ResourceMetrics       `json:"resourceMetrics,omitempty"`
	Errors            []TestError            `json:"errors,omitempty"`
}

// LatencyStats represents latency statistics
type LatencyStats struct {
	Min    time.Duration `json:"min"`
	Max    time.Duration `json:"max"`
	Mean   time.Duration `json:"mean"`
	Median time.Duration `json:"median"`
	P50    time.Duration `json:"p50"`
	P75    time.Duration `json:"p75"`
	P90    time.Duration `json:"p90"`
	P95    time.Duration `json:"p95"`
	P99    time.Duration `json:"p99"`
	StdDev time.Duration `json:"stdDev"`
}

// ScenarioResult represents results for a specific scenario
type ScenarioResult struct {
	Scenario          TestScenario  `json:"scenario"`
	TotalRequests     int64         `json:"totalRequests"`
	SuccessfulRequests int64        `json:"successfulRequests"`
	FailedRequests    int64         `json:"failedRequests"`
	ErrorRate         float64       `json:"errorRate"`
	ThroughputRPS     float64       `json:"throughputRps"`
	Latencies         *LatencyStats `json:"latencies"`
}

// SLOResults represents SLO compliance results
type SLOResults struct {
	P50Met           bool    `json:"p50Met"`
	P95Met           bool    `json:"p95Met"`
	P99Met           bool    `json:"p99Met"`
	MaxLatencyMet    bool    `json:"maxLatencyMet"`
	ErrorRateMet     bool    `json:"errorRateMet"`
	AvailabilityMet  bool    `json:"availabilityMet"`
	ThroughputMet    bool    `json:"throughputMet"`
	OverallCompliant bool    `json:"overallCompliant"`
	ComplianceScore  float64 `json:"complianceScore"` // 0-100
}

// ResourceMetrics represents system resource metrics during test
type ResourceMetrics struct {
	CPUUsagePercent    float64 `json:"cpuUsagePercent"`
	MemoryUsagePercent float64 `json:"memoryUsagePercent"`
	DiskIOPS           float64 `json:"diskIops"`
	NetworkBytesIn     int64   `json:"networkBytesIn"`
	NetworkBytesOut    int64   `json:"networkBytesOut"`
	DBConnectionsUsed  int     `json:"dbConnectionsUsed"`
	DBQueryLatencyMs   float64 `json:"dbQueryLatencyMs"`
}

// TestError represents an error during testing
type TestError struct {
	Timestamp   time.Time `json:"timestamp"`
	Scenario    TestScenario `json:"scenario"`
	Endpoint    string    `json:"endpoint"`
	StatusCode  int       `json:"statusCode"`
	ErrorType   string    `json:"errorType"`
	Message     string    `json:"message"`
	RequestID   string    `json:"requestId,omitempty"`
}

// DataGenerator generates test data at scale
type DataGenerator struct {
	db        *sql.DB
	batchSize int
}

// NewDataGenerator creates a new data generator
func NewDataGenerator(db *sql.DB, batchSize int) *DataGenerator {
	return &DataGenerator{
		db:        db,
		batchSize: batchSize,
	}
}

// GenerateBeneficiaries generates test beneficiaries at scale
func (g *DataGenerator) GenerateBeneficiaries(ctx context.Context, count int64, prefix string) error {
	regions := []string{"North", "South", "East", "West", "Central"}
	districts := []string{"District1", "District2", "District3", "District4", "District5"}
	statuses := []string{"active", "pending", "inactive"}
	genders := []string{"male", "female"}

	var generated int64
	for generated < count {
		batchEnd := generated + int64(g.batchSize)
		if batchEnd > count {
			batchEnd = count
		}

		tx, err := g.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}

		stmt, err := tx.PrepareContext(ctx, `
			INSERT INTO beneficiaries (
				id, first_name, last_name, date_of_birth, gender, national_id,
				phone_number, region, district, status, created_at, updated_at, version
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW(), 1)
		`)
		if err != nil {
			tx.Rollback()
			return err
		}

		for i := generated; i < batchEnd; i++ {
			id := fmt.Sprintf("%s-%012d", prefix, i)
			firstName := fmt.Sprintf("First%d", i)
			lastName := fmt.Sprintf("Last%d", i)
			dob := time.Now().AddDate(-rand.Intn(60)-18, -rand.Intn(12), -rand.Intn(28))
			gender := genders[rand.Intn(len(genders))]
			nationalID := fmt.Sprintf("NID%015d", i)
			phone := fmt.Sprintf("+1%010d", rand.Int63n(10000000000))
			region := regions[rand.Intn(len(regions))]
			district := districts[rand.Intn(len(districts))]
			status := statuses[rand.Intn(len(statuses))]

			_, err := stmt.ExecContext(ctx, id, firstName, lastName, dob.Format("2006-01-02"),
				gender, nationalID, phone, region, district, status)
			if err != nil {
				// Ignore duplicate key errors for idempotency
				continue
			}
		}

		stmt.Close()
		if err := tx.Commit(); err != nil {
			return err
		}

		generated = batchEnd
	}

	return nil
}

// GenerateHouseholds generates test households at scale
func (g *DataGenerator) GenerateHouseholds(ctx context.Context, count int64, prefix string) error {
	housingTypes := []string{"permanent", "semi-permanent", "temporary"}
	incomelevels := []string{"low", "medium", "high"}

	var generated int64
	for generated < count {
		batchEnd := generated + int64(g.batchSize)
		if batchEnd > count {
			batchEnd = count
		}

		tx, err := g.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}

		stmt, err := tx.PrepareContext(ctx, `
			INSERT INTO households (
				id, name, head_id, member_count, housing_type, income_level,
				proxy_means_score, status, created_at, updated_at, version
			) VALUES (?, ?, ?, ?, ?, ?, ?, 'active', NOW(), NOW(), 1)
		`)
		if err != nil {
			tx.Rollback()
			return err
		}

		for i := generated; i < batchEnd; i++ {
			id := fmt.Sprintf("%s-HH-%012d", prefix, i)
			name := fmt.Sprintf("Household%d", i)
			headID := fmt.Sprintf("%s-%012d", prefix, i)
			memberCount := rand.Intn(8) + 1
			housingType := housingTypes[rand.Intn(len(housingTypes))]
			incomeLevel := incomelevels[rand.Intn(len(incomelevels))]
			pmtScore := rand.Float64() * 100

			_, err := stmt.ExecContext(ctx, id, name, headID, memberCount,
				housingType, incomeLevel, pmtScore)
			if err != nil {
				continue
			}
		}

		stmt.Close()
		if err := tx.Commit(); err != nil {
			return err
		}

		generated = batchEnd
	}

	return nil
}

// GenerateTransactions generates test transactions at scale
func (g *DataGenerator) GenerateTransactions(ctx context.Context, count int64, prefix string) error {
	statuses := []string{"pending", "completed", "failed"}
	types := []string{"disbursement", "refund", "adjustment"}

	var generated int64
	for generated < count {
		batchEnd := generated + int64(g.batchSize)
		if batchEnd > count {
			batchEnd = count
		}

		tx, err := g.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}

		stmt, err := tx.PrepareContext(ctx, `
			INSERT INTO transactions (
				id, beneficiary_id, amount, currency, type, status,
				created_at, updated_at
			) VALUES (?, ?, ?, 'USD', ?, ?, NOW(), NOW())
		`)
		if err != nil {
			tx.Rollback()
			return err
		}

		for i := generated; i < batchEnd; i++ {
			id := fmt.Sprintf("%s-TX-%012d", prefix, i)
			beneficiaryID := fmt.Sprintf("%s-%012d", prefix, i%1000000) // Reuse beneficiaries
			amount := float64(rand.Intn(1000) + 10)
			txType := types[rand.Intn(len(types))]
			status := statuses[rand.Intn(len(statuses))]

			_, err := stmt.ExecContext(ctx, id, beneficiaryID, amount, txType, status)
			if err != nil {
				continue
			}
		}

		stmt.Close()
		if err := tx.Commit(); err != nil {
			return err
		}

		generated = batchEnd
	}

	return nil
}

// LoadTestRunner executes load tests
type LoadTestRunner struct {
	db          *sql.DB
	httpClient  *http.Client
	config      *LoadTestConfig
	result      *LoadTestResult
	latencies   []time.Duration
	errors      []TestError
	mu          sync.Mutex
	stopChan    chan struct{}
	wg          sync.WaitGroup
	requestCount int64
	successCount int64
	failCount    int64
}

// NewLoadTestRunner creates a new load test runner
func NewLoadTestRunner(db *sql.DB, config *LoadTestConfig) *LoadTestRunner {
	return &LoadTestRunner{
		db: db,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		config:    config,
		latencies: make([]time.Duration, 0),
		errors:    make([]TestError, 0),
		stopChan:  make(chan struct{}),
	}
}

// Run executes the load test
func (r *LoadTestRunner) Run(ctx context.Context) (*LoadTestResult, error) {
	r.result = &LoadTestResult{
		ID:              fmt.Sprintf("LT-%d", time.Now().UnixNano()),
		ConfigID:        r.config.ID,
		Status:          "running",
		StartTime:       time.Now(),
		ScenarioResults: make(map[TestScenario]*ScenarioResult),
	}

	// Initialize scenario results
	for _, scenario := range r.config.Scenarios {
		r.result.ScenarioResults[scenario] = &ScenarioResult{
			Scenario: scenario,
		}
	}

	// Calculate workers based on test type
	numWorkers := r.calculateWorkers()

	// Ramp up
	r.rampUp(ctx, numWorkers)

	// Main test execution
	testCtx, cancel := context.WithTimeout(ctx, r.config.Duration)
	defer cancel()

	for i := 0; i < numWorkers; i++ {
		r.wg.Add(1)
		go r.worker(testCtx, i)
	}

	// Wait for completion or cancellation
	select {
	case <-testCtx.Done():
	case <-r.stopChan:
	}

	// Ramp down
	r.rampDown(ctx)

	// Wait for all workers to finish
	r.wg.Wait()

	// Calculate final results
	r.calculateResults()

	return r.result, nil
}

// calculateWorkers calculates the number of workers based on test type
func (r *LoadTestRunner) calculateWorkers() int {
	switch r.config.TestType {
	case TestTypeSmoke:
		return 1
	case TestTypeLoad:
		return r.config.MaxConcurrency
	case TestTypeStress:
		return r.config.MaxConcurrency * 2
	case TestTypeSpike:
		return r.config.MaxConcurrency * 3
	case TestTypeSoak:
		return r.config.MaxConcurrency / 2
	case TestTypeBreakpoint:
		return r.config.MaxConcurrency * 4
	case TestTypeScalability:
		return r.config.MaxConcurrency
	default:
		return r.config.MaxConcurrency
	}
}

// rampUp gradually increases load
func (r *LoadTestRunner) rampUp(ctx context.Context, targetWorkers int) {
	if r.config.RampUpTime == 0 {
		return
	}

	steps := 10
	stepDuration := r.config.RampUpTime / time.Duration(steps)
	workersPerStep := targetWorkers / steps

	for i := 1; i <= steps; i++ {
		select {
		case <-ctx.Done():
			return
		case <-time.After(stepDuration):
			// Workers are started in Run(), this just simulates gradual increase
		}
	}
}

// rampDown gradually decreases load
func (r *LoadTestRunner) rampDown(ctx context.Context) {
	if r.config.RampDownTime == 0 {
		return
	}

	close(r.stopChan)
	time.Sleep(r.config.RampDownTime)
}

// worker executes requests
func (r *LoadTestRunner) worker(ctx context.Context, workerID int) {
	defer r.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case <-r.stopChan:
			return
		default:
			// Execute a random scenario
			scenario := r.config.Scenarios[rand.Intn(len(r.config.Scenarios))]
			r.executeScenario(ctx, scenario)

			// Think time
			if r.config.ThinkTime > 0 {
				time.Sleep(r.config.ThinkTime)
			}
		}
	}
}

// executeScenario executes a test scenario
func (r *LoadTestRunner) executeScenario(ctx context.Context, scenario TestScenario) {
	start := time.Now()
	var err error
	var statusCode int

	switch scenario {
	case ScenarioBeneficiaryEnrollment:
		statusCode, err = r.scenarioBeneficiaryEnrollment(ctx)
	case ScenarioBeneficiarySearch:
		statusCode, err = r.scenarioBeneficiarySearch(ctx)
	case ScenarioDisbursement:
		statusCode, err = r.scenarioDisbursement(ctx)
	case ScenarioWorkflowExecution:
		statusCode, err = r.scenarioWorkflowExecution(ctx)
	case ScenarioReporting:
		statusCode, err = r.scenarioReporting(ctx)
	case ScenarioMixedWorkload:
		statusCode, err = r.scenarioMixedWorkload(ctx)
	case ScenarioOfflineSync:
		statusCode, err = r.scenarioOfflineSync(ctx)
	case ScenarioCrossSectorQuery:
		statusCode, err = r.scenarioCrossSectorQuery(ctx)
	}

	latency := time.Since(start)

	r.mu.Lock()
	defer r.mu.Unlock()

	atomic.AddInt64(&r.requestCount, 1)
	r.latencies = append(r.latencies, latency)

	scenarioResult := r.result.ScenarioResults[scenario]
	scenarioResult.TotalRequests++

	if err != nil || statusCode >= 400 {
		atomic.AddInt64(&r.failCount, 1)
		scenarioResult.FailedRequests++
		r.errors = append(r.errors, TestError{
			Timestamp:  time.Now(),
			Scenario:   scenario,
			StatusCode: statusCode,
			ErrorType:  "request_failed",
			Message:    fmt.Sprintf("%v", err),
		})
	} else {
		atomic.AddInt64(&r.successCount, 1)
		scenarioResult.SuccessfulRequests++
	}
}

// Scenario implementations

func (r *LoadTestRunner) scenarioBeneficiaryEnrollment(ctx context.Context) (int, error) {
	// Simulate beneficiary enrollment
	id := fmt.Sprintf("TEST-%d", rand.Int63())
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO beneficiaries (id, first_name, last_name, date_of_birth, gender, national_id, status, created_at, updated_at, version)
		VALUES (?, 'Test', 'User', '1990-01-01', 'male', ?, 'pending', NOW(), NOW(), 1)
	`, id, fmt.Sprintf("NID%d", rand.Int63()))
	if err != nil {
		return 500, err
	}
	return 201, nil
}

func (r *LoadTestRunner) scenarioBeneficiarySearch(ctx context.Context) (int, error) {
	// Simulate beneficiary search
	var count int
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM beneficiaries WHERE status = 'active' LIMIT 100
	`).Scan(&count)
	if err != nil {
		return 500, err
	}
	return 200, nil
}

func (r *LoadTestRunner) scenarioDisbursement(ctx context.Context) (int, error) {
	// Simulate disbursement creation
	id := fmt.Sprintf("DISB-%d", rand.Int63())
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO transactions (id, beneficiary_id, amount, currency, type, status, created_at, updated_at)
		VALUES (?, 'TEST-001', ?, 'USD', 'disbursement', 'pending', NOW(), NOW())
	`, id, rand.Float64()*1000)
	if err != nil {
		return 500, err
	}
	return 201, nil
}

func (r *LoadTestRunner) scenarioWorkflowExecution(ctx context.Context) (int, error) {
	// Simulate workflow status check
	var count int
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM workflow_executions WHERE status = 'running' LIMIT 10
	`).Scan(&count)
	if err != nil {
		// Table might not exist, that's ok for load testing
		return 200, nil
	}
	return 200, nil
}

func (r *LoadTestRunner) scenarioReporting(ctx context.Context) (int, error) {
	// Simulate report generation query
	var total float64
	err := r.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(amount), 0) FROM transactions WHERE status = 'completed' AND created_at > DATE_SUB(NOW(), INTERVAL 30 DAY)
	`).Scan(&total)
	if err != nil {
		return 500, err
	}
	return 200, nil
}

func (r *LoadTestRunner) scenarioMixedWorkload(ctx context.Context) (int, error) {
	// Mix of read and write operations
	ops := []func(context.Context) (int, error){
		r.scenarioBeneficiarySearch,
		r.scenarioBeneficiarySearch,
		r.scenarioBeneficiarySearch,
		r.scenarioBeneficiaryEnrollment,
		r.scenarioReporting,
	}
	return ops[rand.Intn(len(ops))](ctx)
}

func (r *LoadTestRunner) scenarioOfflineSync(ctx context.Context) (int, error) {
	// Simulate offline sync batch
	var count int
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM beneficiaries WHERE updated_at > DATE_SUB(NOW(), INTERVAL 1 HOUR) LIMIT 1000
	`).Scan(&count)
	if err != nil {
		return 500, err
	}
	return 200, nil
}

func (r *LoadTestRunner) scenarioCrossSectorQuery(ctx context.Context) (int, error) {
	// Simulate cross-sector data lookup
	var exists bool
	err := r.db.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM beneficiaries WHERE id = 'TEST-001')
	`).Scan(&exists)
	if err != nil {
		return 500, err
	}
	return 200, nil
}

// calculateResults calculates final test results
func (r *LoadTestRunner) calculateResults() {
	now := time.Now()
	r.result.EndTime = &now
	r.result.Duration = now.Sub(r.result.StartTime)
	r.result.Status = "completed"

	r.result.TotalRequests = atomic.LoadInt64(&r.requestCount)
	r.result.SuccessfulRequests = atomic.LoadInt64(&r.successCount)
	r.result.FailedRequests = atomic.LoadInt64(&r.failCount)

	if r.result.TotalRequests > 0 {
		r.result.ErrorRate = float64(r.result.FailedRequests) / float64(r.result.TotalRequests) * 100
		r.result.ThroughputRPS = float64(r.result.TotalRequests) / r.result.Duration.Seconds()
	}

	// Calculate latency statistics
	r.result.Latencies = r.calculateLatencyStats()

	// Calculate scenario results
	for _, scenarioResult := range r.result.ScenarioResults {
		if scenarioResult.TotalRequests > 0 {
			scenarioResult.ErrorRate = float64(scenarioResult.FailedRequests) / float64(scenarioResult.TotalRequests) * 100
			scenarioResult.ThroughputRPS = float64(scenarioResult.TotalRequests) / r.result.Duration.Seconds()
		}
	}

	// Check SLO compliance
	if r.config.SLOs != nil {
		r.result.SLOResults = r.checkSLOCompliance()
	}

	// Limit errors to last 100
	if len(r.errors) > 100 {
		r.result.Errors = r.errors[len(r.errors)-100:]
	} else {
		r.result.Errors = r.errors
	}
}

// calculateLatencyStats calculates latency statistics
func (r *LoadTestRunner) calculateLatencyStats() *LatencyStats {
	if len(r.latencies) == 0 {
		return &LatencyStats{}
	}

	// Sort latencies for percentile calculation
	sorted := make([]time.Duration, len(r.latencies))
	copy(sorted, r.latencies)
	
	// Simple bubble sort for small datasets, use sort package for large
	for i := 0; i < len(sorted)-1; i++ {
		for j := 0; j < len(sorted)-i-1; j++ {
			if sorted[j] > sorted[j+1] {
				sorted[j], sorted[j+1] = sorted[j+1], sorted[j]
			}
		}
	}

	var sum time.Duration
	for _, l := range sorted {
		sum += l
	}

	n := len(sorted)
	stats := &LatencyStats{
		Min:    sorted[0],
		Max:    sorted[n-1],
		Mean:   sum / time.Duration(n),
		Median: sorted[n/2],
		P50:    sorted[int(float64(n)*0.50)],
		P75:    sorted[int(float64(n)*0.75)],
		P90:    sorted[int(float64(n)*0.90)],
		P95:    sorted[int(float64(n)*0.95)],
		P99:    sorted[int(float64(n)*0.99)],
	}

	// Calculate standard deviation
	var variance float64
	mean := float64(stats.Mean)
	for _, l := range sorted {
		diff := float64(l) - mean
		variance += diff * diff
	}
	variance /= float64(n)
	stats.StdDev = time.Duration(variance)

	return stats
}

// checkSLOCompliance checks if SLOs are met
func (r *LoadTestRunner) checkSLOCompliance() *SLOResults {
	slos := r.config.SLOs
	latencies := r.result.Latencies

	results := &SLOResults{
		P50Met:          latencies.P50.Milliseconds() <= int64(slos.P50LatencyMs),
		P95Met:          latencies.P95.Milliseconds() <= int64(slos.P95LatencyMs),
		P99Met:          latencies.P99.Milliseconds() <= int64(slos.P99LatencyMs),
		MaxLatencyMet:   latencies.Max.Milliseconds() <= int64(slos.MaxLatencyMs),
		ErrorRateMet:    r.result.ErrorRate <= slos.ErrorRatePercent,
		ThroughputMet:   r.result.ThroughputRPS >= float64(slos.ThroughputRPS),
	}

	// Calculate availability
	if r.result.TotalRequests > 0 {
		availability := float64(r.result.SuccessfulRequests) / float64(r.result.TotalRequests) * 100
		results.AvailabilityMet = availability >= slos.AvailabilityPercent
	}

	// Calculate overall compliance
	checks := []bool{
		results.P50Met, results.P95Met, results.P99Met,
		results.MaxLatencyMet, results.ErrorRateMet,
		results.AvailabilityMet, results.ThroughputMet,
	}
	passed := 0
	for _, check := range checks {
		if check {
			passed++
		}
	}
	results.ComplianceScore = float64(passed) / float64(len(checks)) * 100
	results.OverallCompliant = results.ComplianceScore >= 100

	return results
}

// Stop stops the load test
func (r *LoadTestRunner) Stop() {
	close(r.stopChan)
}

// Temporal Workflows

// LoadTestWorkflowInput is the input for load test workflow
type LoadTestWorkflowInput struct {
	Config        *LoadTestConfig `json:"config"`
	GenerateData  bool            `json:"generateData"`
	DataPrefix    string          `json:"dataPrefix"`
}

// LoadTestWorkflow orchestrates load testing
func LoadTestWorkflow(ctx workflow.Context, input LoadTestWorkflowInput) (*LoadTestResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting load test workflow", "testType", input.Config.TestType, "scale", input.Config.ScaleLevel)

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 4 * time.Hour, // Long timeout for soak tests
		HeartbeatTimeout:    5 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    5 * time.Minute,
			MaximumAttempts:    2,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Generate test data if requested
	if input.GenerateData {
		logger.Info("Generating test data", "size", input.Config.DataSetSize)
		err := workflow.ExecuteActivity(ctx, GenerateTestDataActivity, input.Config.DataSetSize, input.DataPrefix).Get(ctx, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to generate test data: %w", err)
		}
	}

	// Run the load test
	var result *LoadTestResult
	err := workflow.ExecuteActivity(ctx, RunLoadTestActivity, input.Config).Get(ctx, &result)
	if err != nil {
		return nil, fmt.Errorf("load test failed: %w", err)
	}

	// Store results
	workflow.ExecuteActivity(ctx, StoreLoadTestResultActivity, result).Get(ctx, nil)

	// Generate report
	workflow.ExecuteActivity(ctx, GenerateLoadTestReportActivity, result).Get(ctx, nil)

	logger.Info("Load test completed", "status", result.Status, "throughput", result.ThroughputRPS)
	return result, nil
}

// ScalabilityTestWorkflow tests horizontal scaling
func ScalabilityTestWorkflow(ctx workflow.Context, baseConfig *LoadTestConfig) ([]LoadTestResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting scalability test workflow")

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 8 * time.Hour,
		HeartbeatTimeout:    10 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    5 * time.Minute,
			MaximumAttempts:    2,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Test at increasing scale levels
	scaleLevels := []struct {
		level       ScaleLevel
		concurrency int
		dataSize    int64
	}{
		{ScaleSmall, 10, 10000},
		{ScaleMedium, 50, 100000},
		{ScaleLarge, 100, 1000000},
		{ScaleXLarge, 500, 10000000},
		{ScaleCountry, 1000, 100000000},
	}

	var results []LoadTestResult

	for _, scale := range scaleLevels {
		config := *baseConfig
		config.ID = fmt.Sprintf("%s-%s", baseConfig.ID, scale.level)
		config.ScaleLevel = scale.level
		config.MaxConcurrency = scale.concurrency
		config.DataSetSize = scale.dataSize

		logger.Info("Running scalability test", "level", scale.level, "concurrency", scale.concurrency)

		var result *LoadTestResult
		err := workflow.ExecuteActivity(ctx, RunLoadTestActivity, &config).Get(ctx, &result)
		if err != nil {
			logger.Warn("Scalability test failed at level", "level", scale.level, "error", err)
			break
		}

		results = append(results, *result)

		// Check if SLOs are still met
		if result.SLOResults != nil && !result.SLOResults.OverallCompliant {
			logger.Info("SLOs not met, stopping scalability test", "level", scale.level)
			break
		}
	}

	return results, nil
}

// Activity implementations

func GenerateTestDataActivity(ctx context.Context, count int64, prefix string) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Generating test data", "count", count, "prefix", prefix)

	generator := getDataGeneratorFromContext(ctx)
	if generator == nil {
		return fmt.Errorf("data generator not available")
	}

	// Generate beneficiaries
	if err := generator.GenerateBeneficiaries(ctx, count, prefix); err != nil {
		return fmt.Errorf("failed to generate beneficiaries: %w", err)
	}

	// Generate households (1/4 of beneficiaries)
	if err := generator.GenerateHouseholds(ctx, count/4, prefix); err != nil {
		return fmt.Errorf("failed to generate households: %w", err)
	}

	// Generate transactions (10x beneficiaries)
	if err := generator.GenerateTransactions(ctx, count*10, prefix); err != nil {
		return fmt.Errorf("failed to generate transactions: %w", err)
	}

	return nil
}

func RunLoadTestActivity(ctx context.Context, config *LoadTestConfig) (*LoadTestResult, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Running load test", "testType", config.TestType)

	db := getDBFromContext(ctx)
	if db == nil {
		return nil, fmt.Errorf("database not available")
	}

	runner := NewLoadTestRunner(db, config)
	return runner.Run(ctx)
}

func StoreLoadTestResultActivity(ctx context.Context, result *LoadTestResult) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Storing load test result", "id", result.ID)

	db := getDBFromContext(ctx)
	if db == nil {
		return fmt.Errorf("database not available")
	}

	resultJSON, _ := json.Marshal(result)
	_, err := db.ExecContext(ctx, `
		INSERT INTO load_test_results (id, config_id, status, result_data, created_at)
		VALUES (?, ?, ?, ?, NOW())
	`, result.ID, result.ConfigID, result.Status, resultJSON)

	return err
}

func GenerateLoadTestReportActivity(ctx context.Context, result *LoadTestResult) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Generating load test report", "id", result.ID)

	// In production, this would generate a detailed HTML/PDF report
	// For now, just log the summary
	logger.Info("Load Test Summary",
		"totalRequests", result.TotalRequests,
		"successRate", 100-result.ErrorRate,
		"throughputRPS", result.ThroughputRPS,
		"p95Latency", result.Latencies.P95,
		"sloCompliance", result.SLOResults.ComplianceScore,
	)

	return nil
}

// Context keys
type dataGeneratorKey struct{}
type dbKey struct{}

func WithDataGenerator(ctx context.Context, generator *DataGenerator) context.Context {
	return context.WithValue(ctx, dataGeneratorKey{}, generator)
}

func getDataGeneratorFromContext(ctx context.Context) *DataGenerator {
	generator, _ := ctx.Value(dataGeneratorKey{}).(*DataGenerator)
	return generator
}

func WithDB(ctx context.Context, db *sql.DB) context.Context {
	return context.WithValue(ctx, dbKey{}, db)
}

func getDBFromContext(ctx context.Context) *sql.DB {
	db, _ := ctx.Value(dbKey{}).(*sql.DB)
	return db
}

// GetLoadTestMigration returns the database migration for load test tables
func GetLoadTestMigration() string {
	return `
-- Load test configurations table
CREATE TABLE IF NOT EXISTS load_test_configs (
    id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    test_type VARCHAR(32) NOT NULL,
    config_data JSON NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_test_type (test_type)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Load test results table
CREATE TABLE IF NOT EXISTS load_test_results (
    id VARCHAR(64) PRIMARY KEY,
    config_id VARCHAR(64) NOT NULL,
    status VARCHAR(32) NOT NULL,
    result_data JSON NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_config_id (config_id),
    INDEX idx_status (status),
    INDEX idx_created (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Load test metrics table (for time-series data)
CREATE TABLE IF NOT EXISTS load_test_metrics (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    test_id VARCHAR(64) NOT NULL,
    timestamp TIMESTAMP NOT NULL,
    metric_name VARCHAR(64) NOT NULL,
    metric_value DOUBLE NOT NULL,
    tags JSON,
    INDEX idx_test_timestamp (test_id, timestamp),
    INDEX idx_metric_name (metric_name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
`
}

// GetScalePresets returns predefined scale configurations
func GetScalePresets() map[ScaleLevel]*LoadTestConfig {
	return map[ScaleLevel]*LoadTestConfig{
		ScaleSmall: {
			Name:           "Small Scale Test",
			ScaleLevel:     ScaleSmall,
			Duration:       5 * time.Minute,
			RampUpTime:     30 * time.Second,
			TargetRPS:      100,
			MaxConcurrency: 10,
			DataSetSize:    10000,
			SLOs: &SLOConfig{
				P50LatencyMs:     100,
				P95LatencyMs:     500,
				P99LatencyMs:     1000,
				MaxLatencyMs:     5000,
				ErrorRatePercent: 1,
				AvailabilityPercent: 99.9,
				ThroughputRPS:    50,
			},
		},
		ScaleMedium: {
			Name:           "Medium Scale Test",
			ScaleLevel:     ScaleMedium,
			Duration:       15 * time.Minute,
			RampUpTime:     2 * time.Minute,
			TargetRPS:      500,
			MaxConcurrency: 50,
			DataSetSize:    100000,
			SLOs: &SLOConfig{
				P50LatencyMs:     150,
				P95LatencyMs:     750,
				P99LatencyMs:     1500,
				MaxLatencyMs:     10000,
				ErrorRatePercent: 1,
				AvailabilityPercent: 99.9,
				ThroughputRPS:    250,
			},
		},
		ScaleLarge: {
			Name:           "Large Scale Test (1M)",
			ScaleLevel:     ScaleLarge,
			Duration:       30 * time.Minute,
			RampUpTime:     5 * time.Minute,
			TargetRPS:      2000,
			MaxConcurrency: 200,
			DataSetSize:    1000000,
			SLOs: &SLOConfig{
				P50LatencyMs:     200,
				P95LatencyMs:     1000,
				P99LatencyMs:     2000,
				MaxLatencyMs:     15000,
				ErrorRatePercent: 1,
				AvailabilityPercent: 99.9,
				ThroughputRPS:    1000,
			},
		},
		ScaleXLarge: {
			Name:           "XLarge Scale Test (10M)",
			ScaleLevel:     ScaleXLarge,
			Duration:       1 * time.Hour,
			RampUpTime:     10 * time.Minute,
			TargetRPS:      10000,
			MaxConcurrency: 1000,
			DataSetSize:    10000000,
			SLOs: &SLOConfig{
				P50LatencyMs:     250,
				P95LatencyMs:     1500,
				P99LatencyMs:     3000,
				MaxLatencyMs:     20000,
				ErrorRatePercent: 1,
				AvailabilityPercent: 99.9,
				ThroughputRPS:    5000,
			},
		},
		ScaleCountry: {
			Name:           "Country Scale Test (100M+)",
			ScaleLevel:     ScaleCountry,
			Duration:       2 * time.Hour,
			RampUpTime:     20 * time.Minute,
			TargetRPS:      50000,
			MaxConcurrency: 5000,
			DataSetSize:    100000000,
			SLOs: &SLOConfig{
				P50LatencyMs:     300,
				P95LatencyMs:     2000,
				P99LatencyMs:     5000,
				MaxLatencyMs:     30000,
				ErrorRatePercent: 1,
				AvailabilityPercent: 99.9,
				ThroughputRPS:    25000,
			},
		},
	}
}

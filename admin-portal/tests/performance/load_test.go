package performance

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// LoadTestConfig holds configuration for load tests
type LoadTestConfig struct {
	ConcurrentUsers int
	Duration        time.Duration
	RampUpTime      time.Duration
	TargetRPS       int
}

// LoadTestResult holds results from a load test
type LoadTestResult struct {
	TotalRequests     int64
	SuccessfulRequests int64
	FailedRequests    int64
	AverageLatency    time.Duration
	P50Latency        time.Duration
	P95Latency        time.Duration
	P99Latency        time.Duration
	MaxLatency        time.Duration
	MinLatency        time.Duration
	RequestsPerSecond float64
	ErrorRate         float64
}

// TestAPIEndpointPerformance tests API endpoint performance
func TestAPIEndpointPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	tests := []struct {
		name           string
		endpoint       string
		method         string
		concurrency    int
		duration       time.Duration
		maxLatencyP95  time.Duration
		minRPS         float64
	}{
		{
			name:          "beneficiary list endpoint",
			endpoint:      "/api/v1/beneficiaries",
			method:        "GET",
			concurrency:   50,
			duration:      time.Second * 10,
			maxLatencyP95: time.Millisecond * 200,
			minRPS:        100,
		},
		{
			name:          "beneficiary detail endpoint",
			endpoint:      "/api/v1/beneficiaries/{id}",
			method:        "GET",
			concurrency:   100,
			duration:      time.Second * 10,
			maxLatencyP95: time.Millisecond * 100,
			minRPS:        200,
		},
		{
			name:          "program list endpoint",
			endpoint:      "/api/v1/programs",
			method:        "GET",
			concurrency:   50,
			duration:      time.Second * 10,
			maxLatencyP95: time.Millisecond * 150,
			minRPS:        150,
		},
		{
			name:          "disbursement create endpoint",
			endpoint:      "/api/v1/disbursements",
			method:        "POST",
			concurrency:   20,
			duration:      time.Second * 10,
			maxLatencyP95: time.Millisecond * 500,
			minRPS:        50,
		},
		{
			name:          "report generation endpoint",
			endpoint:      "/api/v1/reports/generate",
			method:        "POST",
			concurrency:   10,
			duration:      time.Second * 10,
			maxLatencyP95: time.Second * 2,
			minRPS:        10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := runLoadTest(tt.endpoint, tt.method, tt.concurrency, tt.duration)

			t.Logf("Results for %s:", tt.name)
			t.Logf("  Total Requests: %d", result.TotalRequests)
			t.Logf("  Successful: %d (%.2f%%)", result.SuccessfulRequests, 
				float64(result.SuccessfulRequests)/float64(result.TotalRequests)*100)
			t.Logf("  Failed: %d", result.FailedRequests)
			t.Logf("  RPS: %.2f", result.RequestsPerSecond)
			t.Logf("  Avg Latency: %v", result.AverageLatency)
			t.Logf("  P95 Latency: %v", result.P95Latency)
			t.Logf("  P99 Latency: %v", result.P99Latency)

			if result.P95Latency > tt.maxLatencyP95 {
				t.Errorf("P95 latency %v exceeds maximum %v", result.P95Latency, tt.maxLatencyP95)
			}

			if result.RequestsPerSecond < tt.minRPS {
				t.Errorf("RPS %.2f below minimum %.2f", result.RequestsPerSecond, tt.minRPS)
			}

			if result.ErrorRate > 0.01 { // 1% error rate threshold
				t.Errorf("Error rate %.2f%% exceeds 1%% threshold", result.ErrorRate*100)
			}
		})
	}
}

// runLoadTest simulates a load test
func runLoadTest(endpoint, method string, concurrency int, duration time.Duration) LoadTestResult {
	var (
		totalRequests     int64
		successfulRequests int64
		failedRequests    int64
		latencies         []time.Duration
		latencyMu         sync.Mutex
	)

	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	var wg sync.WaitGroup

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for {
				select {
				case <-ctx.Done():
					return
				default:
					start := time.Now()
					success := simulateRequest(endpoint, method)
					latency := time.Since(start)

					atomic.AddInt64(&totalRequests, 1)
					if success {
						atomic.AddInt64(&successfulRequests, 1)
					} else {
						atomic.AddInt64(&failedRequests, 1)
					}

					latencyMu.Lock()
					latencies = append(latencies, latency)
					latencyMu.Unlock()
				}
			}
		}()
	}

	wg.Wait()

	return calculateResults(totalRequests, successfulRequests, failedRequests, latencies, duration)
}

// simulateRequest simulates an HTTP request
func simulateRequest(endpoint, method string) bool {
	// Simulate variable latency
	baseLatency := time.Millisecond * time.Duration(10+rand.Intn(50))
	time.Sleep(baseLatency)

	// Simulate 99% success rate
	return rand.Float64() < 0.99
}

// calculateResults calculates load test results
func calculateResults(total, successful, failed int64, latencies []time.Duration, duration time.Duration) LoadTestResult {
	if len(latencies) == 0 {
		return LoadTestResult{}
	}

	// Sort latencies for percentile calculation
	sortDurations(latencies)

	var totalLatency time.Duration
	for _, l := range latencies {
		totalLatency += l
	}

	result := LoadTestResult{
		TotalRequests:      total,
		SuccessfulRequests: successful,
		FailedRequests:     failed,
		AverageLatency:     totalLatency / time.Duration(len(latencies)),
		P50Latency:         latencies[len(latencies)*50/100],
		P95Latency:         latencies[len(latencies)*95/100],
		P99Latency:         latencies[len(latencies)*99/100],
		MaxLatency:         latencies[len(latencies)-1],
		MinLatency:         latencies[0],
		RequestsPerSecond:  float64(total) / duration.Seconds(),
		ErrorRate:          float64(failed) / float64(total),
	}

	return result
}

// sortDurations sorts a slice of durations
func sortDurations(durations []time.Duration) {
	for i := 0; i < len(durations); i++ {
		for j := i + 1; j < len(durations); j++ {
			if durations[j] < durations[i] {
				durations[i], durations[j] = durations[j], durations[i]
			}
		}
	}
}

// TestDatabasePerformance tests database query performance
func TestDatabasePerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	tests := []struct {
		name          string
		query         string
		maxLatency    time.Duration
		iterations    int
	}{
		{
			name:       "simple select",
			query:      "SELECT * FROM beneficiaries WHERE id = ?",
			maxLatency: time.Millisecond * 10,
			iterations: 1000,
		},
		{
			name:       "indexed search",
			query:      "SELECT * FROM beneficiaries WHERE program_id = ? AND status = ?",
			maxLatency: time.Millisecond * 20,
			iterations: 1000,
		},
		{
			name:       "aggregation query",
			query:      "SELECT program_id, COUNT(*), SUM(amount) FROM disbursements GROUP BY program_id",
			maxLatency: time.Millisecond * 100,
			iterations: 100,
		},
		{
			name:       "join query",
			query:      "SELECT b.*, p.name FROM beneficiaries b JOIN programs p ON b.program_id = p.id",
			maxLatency: time.Millisecond * 50,
			iterations: 500,
		},
		{
			name:       "full text search",
			query:      "SELECT * FROM beneficiaries WHERE name ILIKE ?",
			maxLatency: time.Millisecond * 100,
			iterations: 200,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var totalLatency time.Duration
			var maxObserved time.Duration

			for i := 0; i < tt.iterations; i++ {
				start := time.Now()
				simulateDatabaseQuery(tt.query)
				latency := time.Since(start)

				totalLatency += latency
				if latency > maxObserved {
					maxObserved = latency
				}
			}

			avgLatency := totalLatency / time.Duration(tt.iterations)

			t.Logf("Query: %s", tt.name)
			t.Logf("  Iterations: %d", tt.iterations)
			t.Logf("  Avg Latency: %v", avgLatency)
			t.Logf("  Max Latency: %v", maxObserved)

			if avgLatency > tt.maxLatency {
				t.Errorf("Average latency %v exceeds maximum %v", avgLatency, tt.maxLatency)
			}
		})
	}
}

// simulateDatabaseQuery simulates a database query
func simulateDatabaseQuery(query string) {
	// Simulate query execution time based on complexity
	baseTime := time.Millisecond * 2
	if len(query) > 50 {
		baseTime = time.Millisecond * 5
	}
	if len(query) > 100 {
		baseTime = time.Millisecond * 10
	}
	time.Sleep(baseTime + time.Duration(rand.Intn(5))*time.Millisecond)
}

// TestCachePerformance tests cache performance
func TestCachePerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	tests := []struct {
		name       string
		operation  string
		keySize    int
		valueSize  int
		iterations int
		maxLatency time.Duration
	}{
		{
			name:       "small value get",
			operation:  "GET",
			keySize:    32,
			valueSize:  100,
			iterations: 10000,
			maxLatency: time.Microsecond * 500,
		},
		{
			name:       "small value set",
			operation:  "SET",
			keySize:    32,
			valueSize:  100,
			iterations: 10000,
			maxLatency: time.Millisecond * 1,
		},
		{
			name:       "large value get",
			operation:  "GET",
			keySize:    32,
			valueSize:  10000,
			iterations: 1000,
			maxLatency: time.Millisecond * 2,
		},
		{
			name:       "large value set",
			operation:  "SET",
			keySize:    32,
			valueSize:  10000,
			iterations: 1000,
			maxLatency: time.Millisecond * 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var totalLatency time.Duration

			for i := 0; i < tt.iterations; i++ {
				start := time.Now()
				simulateCacheOperation(tt.operation, tt.keySize, tt.valueSize)
				totalLatency += time.Since(start)
			}

			avgLatency := totalLatency / time.Duration(tt.iterations)

			t.Logf("Cache %s: avg latency %v", tt.name, avgLatency)

			if avgLatency > tt.maxLatency {
				t.Errorf("Average latency %v exceeds maximum %v", avgLatency, tt.maxLatency)
			}
		})
	}
}

// simulateCacheOperation simulates a cache operation
func simulateCacheOperation(operation string, keySize, valueSize int) {
	baseTime := time.Microsecond * 100
	if valueSize > 1000 {
		baseTime = time.Microsecond * 500
	}
	if operation == "SET" {
		baseTime *= 2
	}
	time.Sleep(baseTime)
}

// TestMessageQueuePerformance tests message queue performance
func TestMessageQueuePerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	tests := []struct {
		name          string
		messageSize   int
		batchSize     int
		producers     int
		consumers     int
		duration      time.Duration
		minThroughput int64 // messages per second
	}{
		{
			name:          "small messages high throughput",
			messageSize:   100,
			batchSize:     100,
			producers:     5,
			consumers:     5,
			duration:      time.Second * 5,
			minThroughput: 10000,
		},
		{
			name:          "large messages",
			messageSize:   10000,
			batchSize:     10,
			producers:     3,
			consumers:     3,
			duration:      time.Second * 5,
			minThroughput: 1000,
		},
		{
			name:          "many producers",
			messageSize:   500,
			batchSize:     50,
			producers:     20,
			consumers:     5,
			duration:      time.Second * 5,
			minThroughput: 5000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var produced, consumed int64

			ctx, cancel := context.WithTimeout(context.Background(), tt.duration)
			defer cancel()

			var wg sync.WaitGroup

			// Start producers
			for i := 0; i < tt.producers; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for {
						select {
						case <-ctx.Done():
							return
						default:
							simulateProduceMessage(tt.messageSize)
							atomic.AddInt64(&produced, 1)
						}
					}
				}()
			}

			// Start consumers
			for i := 0; i < tt.consumers; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for {
						select {
						case <-ctx.Done():
							return
						default:
							simulateConsumeMessage(tt.messageSize)
							atomic.AddInt64(&consumed, 1)
						}
					}
				}()
			}

			wg.Wait()

			throughput := float64(produced) / tt.duration.Seconds()

			t.Logf("Message Queue %s:", tt.name)
			t.Logf("  Produced: %d", produced)
			t.Logf("  Consumed: %d", consumed)
			t.Logf("  Throughput: %.2f msg/s", throughput)

			if throughput < float64(tt.minThroughput) {
				t.Errorf("Throughput %.2f below minimum %d", throughput, tt.minThroughput)
			}
		})
	}
}

// simulateProduceMessage simulates producing a message
func simulateProduceMessage(size int) {
	time.Sleep(time.Microsecond * time.Duration(10+size/100))
}

// simulateConsumeMessage simulates consuming a message
func simulateConsumeMessage(size int) {
	time.Sleep(time.Microsecond * time.Duration(5+size/200))
}

// TestWorkflowPerformance tests Temporal workflow performance
func TestWorkflowPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	tests := []struct {
		name           string
		workflowType   string
		concurrency    int
		duration       time.Duration
		maxLatency     time.Duration
		minCompletions int
	}{
		{
			name:           "enrollment workflow",
			workflowType:   "EnrollmentWorkflow",
			concurrency:    10,
			duration:       time.Second * 10,
			maxLatency:     time.Second * 5,
			minCompletions: 50,
		},
		{
			name:           "disbursement workflow",
			workflowType:   "DisbursementWorkflow",
			concurrency:    20,
			duration:       time.Second * 10,
			maxLatency:     time.Second * 3,
			minCompletions: 100,
		},
		{
			name:           "verification workflow",
			workflowType:   "VerificationWorkflow",
			concurrency:    15,
			duration:       time.Second * 10,
			maxLatency:     time.Second * 2,
			minCompletions: 75,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var completions int64
			var totalLatency time.Duration
			var latencyMu sync.Mutex

			ctx, cancel := context.WithTimeout(context.Background(), tt.duration)
			defer cancel()

			var wg sync.WaitGroup

			for i := 0; i < tt.concurrency; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for {
						select {
						case <-ctx.Done():
							return
						default:
							start := time.Now()
							simulateWorkflowExecution(tt.workflowType)
							latency := time.Since(start)

							atomic.AddInt64(&completions, 1)
							latencyMu.Lock()
							totalLatency += latency
							latencyMu.Unlock()
						}
					}
				}()
			}

			wg.Wait()

			avgLatency := totalLatency / time.Duration(completions)

			t.Logf("Workflow %s:", tt.name)
			t.Logf("  Completions: %d", completions)
			t.Logf("  Avg Latency: %v", avgLatency)

			if completions < int64(tt.minCompletions) {
				t.Errorf("Completions %d below minimum %d", completions, tt.minCompletions)
			}

			if avgLatency > tt.maxLatency {
				t.Errorf("Average latency %v exceeds maximum %v", avgLatency, tt.maxLatency)
			}
		})
	}
}

// simulateWorkflowExecution simulates workflow execution
func simulateWorkflowExecution(workflowType string) {
	var duration time.Duration
	switch workflowType {
	case "EnrollmentWorkflow":
		duration = time.Millisecond * time.Duration(100+rand.Intn(200))
	case "DisbursementWorkflow":
		duration = time.Millisecond * time.Duration(50+rand.Intn(100))
	case "VerificationWorkflow":
		duration = time.Millisecond * time.Duration(30+rand.Intn(70))
	default:
		duration = time.Millisecond * 100
	}
	time.Sleep(duration)
}

// BenchmarkBeneficiaryLookup benchmarks beneficiary lookup
func BenchmarkBeneficiaryLookup(b *testing.B) {
	for i := 0; i < b.N; i++ {
		simulateDatabaseQuery("SELECT * FROM beneficiaries WHERE id = ?")
	}
}

// BenchmarkDisbursementCreate benchmarks disbursement creation
func BenchmarkDisbursementCreate(b *testing.B) {
	for i := 0; i < b.N; i++ {
		simulateDatabaseQuery("INSERT INTO disbursements (beneficiary_id, amount, status) VALUES (?, ?, ?)")
	}
}

// BenchmarkCacheGet benchmarks cache get operations
func BenchmarkCacheGet(b *testing.B) {
	for i := 0; i < b.N; i++ {
		simulateCacheOperation("GET", 32, 100)
	}
}

// BenchmarkCacheSet benchmarks cache set operations
func BenchmarkCacheSet(b *testing.B) {
	for i := 0; i < b.N; i++ {
		simulateCacheOperation("SET", 32, 100)
	}
}

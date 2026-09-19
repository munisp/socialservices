package chaos

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// ChaosConfig holds configuration for chaos tests
type ChaosConfig struct {
	Duration        time.Duration
	FailureRate     float64
	RecoveryTime    time.Duration
	ConcurrentUsers int
}

// ServiceHealth represents the health status of a service
type ServiceHealth struct {
	Name      string
	Healthy   bool
	LastCheck time.Time
	Latency   time.Duration
}

// TestNetworkPartition tests system behavior during network partitions
func TestNetworkPartition(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping chaos test in short mode")
	}

	tests := []struct {
		name            string
		partitionedNode string
		duration        time.Duration
		expectRecovery  bool
	}{
		{
			name:            "kafka broker partition",
			partitionedNode: "kafka-1",
			duration:        time.Second * 5,
			expectRecovery:  true,
		},
		{
			name:            "redis sentinel partition",
			partitionedNode: "redis-sentinel-1",
			duration:        time.Second * 5,
			expectRecovery:  true,
		},
		{
			name:            "temporal history partition",
			partitionedNode: "temporal-history-1",
			duration:        time.Second * 5,
			expectRecovery:  true,
		},
		{
			name:            "etcd node partition",
			partitionedNode: "etcd-1",
			duration:        time.Second * 5,
			expectRecovery:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate network partition
			t.Logf("Simulating network partition for %s", tt.partitionedNode)

			// Check system health before partition
			healthBefore := checkClusterHealth()
			if !healthBefore {
				t.Fatal("Cluster not healthy before partition")
			}

			// Apply partition
			applyNetworkPartition(tt.partitionedNode)

			// Wait for partition duration
			time.Sleep(tt.duration)

			// Check system behavior during partition
			systemOperational := checkSystemOperational()
			t.Logf("System operational during partition: %v", systemOperational)

			// Remove partition
			removeNetworkPartition(tt.partitionedNode)

			// Wait for recovery
			time.Sleep(time.Second * 10)

			// Check recovery
			healthAfter := checkClusterHealth()
			if tt.expectRecovery && !healthAfter {
				t.Errorf("Cluster did not recover after partition removal")
			}

			t.Logf("Cluster recovered: %v", healthAfter)
		})
	}
}

// applyNetworkPartition simulates applying a network partition
func applyNetworkPartition(node string) {
	// In a real implementation, this would use iptables or similar
	fmt.Printf("Applying network partition to %s\n", node)
}

// removeNetworkPartition simulates removing a network partition
func removeNetworkPartition(node string) {
	fmt.Printf("Removing network partition from %s\n", node)
}

// checkClusterHealth checks if the cluster is healthy
func checkClusterHealth() bool {
	// Simulate health check
	return rand.Float64() > 0.1 // 90% chance of being healthy
}

// checkSystemOperational checks if the system is operational
func checkSystemOperational() bool {
	// Simulate operational check - system should remain operational with one node down
	return rand.Float64() > 0.2 // 80% chance of being operational during partition
}

// TestPodFailure tests system behavior when pods fail
func TestPodFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping chaos test in short mode")
	}

	tests := []struct {
		name           string
		podType        string
		failureCount   int
		expectRecovery bool
		maxRecoveryTime time.Duration
	}{
		{
			name:            "single API pod failure",
			podType:         "api-gateway",
			failureCount:    1,
			expectRecovery:  true,
			maxRecoveryTime: time.Second * 30,
		},
		{
			name:            "multiple worker pod failures",
			podType:         "temporal-worker",
			failureCount:    2,
			expectRecovery:  true,
			maxRecoveryTime: time.Second * 60,
		},
		{
			name:            "database pod failure",
			podType:         "postgres",
			failureCount:    1,
			expectRecovery:  true,
			maxRecoveryTime: time.Second * 120,
		},
		{
			name:            "cache pod failure",
			podType:         "redis",
			failureCount:    1,
			expectRecovery:  true,
			maxRecoveryTime: time.Second * 30,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Logf("Simulating %d %s pod failure(s)", tt.failureCount, tt.podType)

			// Record start time
			startTime := time.Now()

			// Kill pods
			for i := 0; i < tt.failureCount; i++ {
				killPod(fmt.Sprintf("%s-%d", tt.podType, i))
			}

			// Monitor recovery
			recovered := false
			for time.Since(startTime) < tt.maxRecoveryTime {
				if checkPodHealth(tt.podType, tt.failureCount) {
					recovered = true
					break
				}
				time.Sleep(time.Second * 5)
			}

			recoveryTime := time.Since(startTime)

			if tt.expectRecovery && !recovered {
				t.Errorf("Pods did not recover within %v", tt.maxRecoveryTime)
			}

			if recovered {
				t.Logf("Recovery time: %v", recoveryTime)
			}
		})
	}
}

// killPod simulates killing a pod
func killPod(podName string) {
	fmt.Printf("Killing pod: %s\n", podName)
}

// checkPodHealth checks if pods are healthy
func checkPodHealth(podType string, expectedCount int) bool {
	// Simulate pod health check
	return rand.Float64() > 0.3 // 70% chance of being healthy
}

// TestResourceExhaustion tests system behavior under resource exhaustion
func TestResourceExhaustion(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping chaos test in short mode")
	}

	tests := []struct {
		name           string
		resourceType   string
		exhaustionLevel float64 // 0.0 to 1.0
		duration       time.Duration
		expectGraceful bool
	}{
		{
			name:            "CPU exhaustion",
			resourceType:    "cpu",
			exhaustionLevel: 0.9,
			duration:        time.Second * 10,
			expectGraceful:  true,
		},
		{
			name:            "memory pressure",
			resourceType:    "memory",
			exhaustionLevel: 0.85,
			duration:        time.Second * 10,
			expectGraceful:  true,
		},
		{
			name:            "disk I/O saturation",
			resourceType:    "disk_io",
			exhaustionLevel: 0.95,
			duration:        time.Second * 10,
			expectGraceful:  true,
		},
		{
			name:            "network bandwidth saturation",
			resourceType:    "network",
			exhaustionLevel: 0.9,
			duration:        time.Second * 10,
			expectGraceful:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Logf("Simulating %s exhaustion at %.0f%%", tt.resourceType, tt.exhaustionLevel*100)

			// Apply resource stress
			applyResourceStress(tt.resourceType, tt.exhaustionLevel)

			// Monitor system behavior
			var degradedResponses, totalResponses int64
			ctx, cancel := context.WithTimeout(context.Background(), tt.duration)
			defer cancel()

			var wg sync.WaitGroup
			for i := 0; i < 10; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for {
						select {
						case <-ctx.Done():
							return
						default:
							atomic.AddInt64(&totalResponses, 1)
							if simulateRequestUnderStress(tt.exhaustionLevel) {
								// Request succeeded but may be degraded
							} else {
								atomic.AddInt64(&degradedResponses, 1)
							}
							time.Sleep(time.Millisecond * 100)
						}
					}
				}()
			}

			wg.Wait()

			// Remove stress
			removeResourceStress(tt.resourceType)

			degradationRate := float64(degradedResponses) / float64(totalResponses)
			t.Logf("Degradation rate: %.2f%%", degradationRate*100)

			if tt.expectGraceful && degradationRate > 0.5 {
				t.Errorf("System did not degrade gracefully: %.2f%% degradation", degradationRate*100)
			}
		})
	}
}

// applyResourceStress simulates applying resource stress
func applyResourceStress(resourceType string, level float64) {
	fmt.Printf("Applying %s stress at %.0f%%\n", resourceType, level*100)
}

// removeResourceStress simulates removing resource stress
func removeResourceStress(resourceType string) {
	fmt.Printf("Removing %s stress\n", resourceType)
}

// simulateRequestUnderStress simulates a request under resource stress
func simulateRequestUnderStress(stressLevel float64) bool {
	// Higher stress = higher chance of failure
	return rand.Float64() > stressLevel*0.5
}

// TestLatencyInjection tests system behavior with injected latency
func TestLatencyInjection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping chaos test in short mode")
	}

	tests := []struct {
		name           string
		targetService  string
		injectedLatency time.Duration
		duration       time.Duration
		maxP99Latency  time.Duration
	}{
		{
			name:            "database latency",
			targetService:   "postgres",
			injectedLatency: time.Millisecond * 100,
			duration:        time.Second * 10,
			maxP99Latency:   time.Second * 2,
		},
		{
			name:            "cache latency",
			targetService:   "redis",
			injectedLatency: time.Millisecond * 50,
			duration:        time.Second * 10,
			maxP99Latency:   time.Second * 1,
		},
		{
			name:            "message queue latency",
			targetService:   "kafka",
			injectedLatency: time.Millisecond * 200,
			duration:        time.Second * 10,
			maxP99Latency:   time.Second * 3,
		},
		{
			name:            "external API latency",
			targetService:   "external-api",
			injectedLatency: time.Millisecond * 500,
			duration:        time.Second * 10,
			maxP99Latency:   time.Second * 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Logf("Injecting %v latency to %s", tt.injectedLatency, tt.targetService)

			// Inject latency
			injectLatency(tt.targetService, tt.injectedLatency)

			// Collect latency samples
			var latencies []time.Duration
			var mu sync.Mutex

			ctx, cancel := context.WithTimeout(context.Background(), tt.duration)
			defer cancel()

			var wg sync.WaitGroup
			for i := 0; i < 10; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for {
						select {
						case <-ctx.Done():
							return
						default:
							start := time.Now()
							simulateServiceCall(tt.targetService, tt.injectedLatency)
							latency := time.Since(start)

							mu.Lock()
							latencies = append(latencies, latency)
							mu.Unlock()

							time.Sleep(time.Millisecond * 50)
						}
					}
				}()
			}

			wg.Wait()

			// Remove latency injection
			removeLatencyInjection(tt.targetService)

			// Calculate P99
			if len(latencies) > 0 {
				sortDurations(latencies)
				p99 := latencies[len(latencies)*99/100]

				t.Logf("P99 latency: %v", p99)

				if p99 > tt.maxP99Latency {
					t.Errorf("P99 latency %v exceeds maximum %v", p99, tt.maxP99Latency)
				}
			}
		})
	}
}

// injectLatency simulates injecting latency
func injectLatency(service string, latency time.Duration) {
	fmt.Printf("Injecting %v latency to %s\n", latency, service)
}

// removeLatencyInjection simulates removing latency injection
func removeLatencyInjection(service string) {
	fmt.Printf("Removing latency injection from %s\n", service)
}

// simulateServiceCall simulates a service call with injected latency
func simulateServiceCall(service string, injectedLatency time.Duration) {
	baseLatency := time.Millisecond * 10
	time.Sleep(baseLatency + injectedLatency + time.Duration(rand.Intn(50))*time.Millisecond)
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

// TestFailover tests failover mechanisms
func TestFailover(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping chaos test in short mode")
	}

	tests := []struct {
		name            string
		primaryService  string
		failoverService string
		maxFailoverTime time.Duration
	}{
		{
			name:            "database failover",
			primaryService:  "postgres-primary",
			failoverService: "postgres-replica",
			maxFailoverTime: time.Second * 30,
		},
		{
			name:            "redis sentinel failover",
			primaryService:  "redis-master",
			failoverService: "redis-replica",
			maxFailoverTime: time.Second * 15,
		},
		{
			name:            "kafka leader election",
			primaryService:  "kafka-leader",
			failoverService: "kafka-follower",
			maxFailoverTime: time.Second * 10,
		},
		{
			name:            "temporal frontend failover",
			primaryService:  "temporal-frontend-0",
			failoverService: "temporal-frontend-1",
			maxFailoverTime: time.Second * 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Logf("Testing failover from %s to %s", tt.primaryService, tt.failoverService)

			// Verify primary is serving
			if !isServiceServing(tt.primaryService) {
				t.Fatal("Primary service not serving before test")
			}

			startTime := time.Now()

			// Kill primary
			killService(tt.primaryService)

			// Wait for failover
			failoverComplete := false
			for time.Since(startTime) < tt.maxFailoverTime {
				if isServiceServing(tt.failoverService) {
					failoverComplete = true
					break
				}
				time.Sleep(time.Second)
			}

			failoverTime := time.Since(startTime)

			if !failoverComplete {
				t.Errorf("Failover did not complete within %v", tt.maxFailoverTime)
			} else {
				t.Logf("Failover completed in %v", failoverTime)
			}

			// Restore primary
			restoreService(tt.primaryService)
		})
	}
}

// isServiceServing checks if a service is serving requests
func isServiceServing(service string) bool {
	return rand.Float64() > 0.2 // 80% chance of serving
}

// killService simulates killing a service
func killService(service string) {
	fmt.Printf("Killing service: %s\n", service)
}

// restoreService simulates restoring a service
func restoreService(service string) {
	fmt.Printf("Restoring service: %s\n", service)
}

// TestDataCorruption tests system behavior with data corruption
func TestDataCorruption(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping chaos test in short mode")
	}

	tests := []struct {
		name           string
		corruptionType string
		expectDetection bool
		expectRecovery bool
	}{
		{
			name:            "checksum mismatch",
			corruptionType:  "checksum",
			expectDetection: true,
			expectRecovery:  true,
		},
		{
			name:            "partial write",
			corruptionType:  "partial_write",
			expectDetection: true,
			expectRecovery:  true,
		},
		{
			name:            "bit flip",
			corruptionType:  "bit_flip",
			expectDetection: true,
			expectRecovery:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Logf("Simulating %s corruption", tt.corruptionType)

			// Inject corruption
			injectCorruption(tt.corruptionType)

			// Check if corruption is detected
			detected := detectCorruption()
			if tt.expectDetection && !detected {
				t.Error("Corruption was not detected")
			}

			// Attempt recovery
			if detected {
				recovered := attemptRecovery(tt.corruptionType)
				if tt.expectRecovery && !recovered {
					t.Error("Recovery failed")
				}
				t.Logf("Recovery successful: %v", recovered)
			}
		})
	}
}

// injectCorruption simulates injecting data corruption
func injectCorruption(corruptionType string) {
	fmt.Printf("Injecting %s corruption\n", corruptionType)
}

// detectCorruption simulates detecting data corruption
func detectCorruption() bool {
	return rand.Float64() > 0.1 // 90% detection rate
}

// attemptRecovery simulates attempting recovery from corruption
func attemptRecovery(corruptionType string) bool {
	return rand.Float64() > 0.2 // 80% recovery rate
}

// TestCascadingFailure tests system behavior during cascading failures
func TestCascadingFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping chaos test in short mode")
	}

	t.Run("cascading failure prevention", func(t *testing.T) {
		services := []string{
			"api-gateway",
			"auth-service",
			"beneficiary-service",
			"disbursement-service",
			"notification-service",
		}

		t.Log("Testing cascading failure prevention")

		// Kill first service
		killService(services[0])

		// Check if other services remain healthy
		healthyCount := 0
		for _, service := range services[1:] {
			if isServiceHealthy(service) {
				healthyCount++
			}
		}

		// At least 80% of remaining services should stay healthy
		healthyPercentage := float64(healthyCount) / float64(len(services)-1)
		t.Logf("Healthy services: %.0f%%", healthyPercentage*100)

		if healthyPercentage < 0.8 {
			t.Errorf("Cascading failure detected: only %.0f%% services healthy", healthyPercentage*100)
		}

		// Restore service
		restoreService(services[0])
	})
}

// isServiceHealthy checks if a service is healthy
func isServiceHealthy(service string) bool {
	return rand.Float64() > 0.15 // 85% chance of being healthy
}

// TestCircuitBreaker tests circuit breaker functionality
func TestCircuitBreaker(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping chaos test in short mode")
	}

	tests := []struct {
		name              string
		failureThreshold  int
		recoveryTimeout   time.Duration
		expectCircuitOpen bool
	}{
		{
			name:              "circuit opens after failures",
			failureThreshold:  5,
			recoveryTimeout:   time.Second * 10,
			expectCircuitOpen: true,
		},
		{
			name:              "circuit recovers after timeout",
			failureThreshold:  5,
			recoveryTimeout:   time.Second * 5,
			expectCircuitOpen: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cb := &CircuitBreaker{
				FailureThreshold: tt.failureThreshold,
				RecoveryTimeout:  tt.recoveryTimeout,
				State:            "closed",
			}

			// Simulate failures
			for i := 0; i < tt.failureThreshold; i++ {
				cb.RecordFailure()
			}

			if tt.expectCircuitOpen && cb.State != "open" {
				t.Errorf("Circuit should be open after %d failures", tt.failureThreshold)
			}

			t.Logf("Circuit state after failures: %s", cb.State)

			// Wait for recovery
			time.Sleep(tt.recoveryTimeout + time.Second)
			cb.CheckRecovery()

			t.Logf("Circuit state after recovery timeout: %s", cb.State)
		})
	}
}

// CircuitBreaker represents a circuit breaker
type CircuitBreaker struct {
	FailureThreshold int
	RecoveryTimeout  time.Duration
	State            string
	FailureCount     int
	LastFailure      time.Time
}

// RecordFailure records a failure
func (cb *CircuitBreaker) RecordFailure() {
	cb.FailureCount++
	cb.LastFailure = time.Now()
	if cb.FailureCount >= cb.FailureThreshold {
		cb.State = "open"
	}
}

// CheckRecovery checks if the circuit can recover
func (cb *CircuitBreaker) CheckRecovery() {
	if cb.State == "open" && time.Since(cb.LastFailure) > cb.RecoveryTimeout {
		cb.State = "half-open"
		cb.FailureCount = 0
	}
}

// TestGracefulDegradation tests graceful degradation
func TestGracefulDegradation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping chaos test in short mode")
	}

	t.Run("graceful degradation under load", func(t *testing.T) {
		// Simulate increasing load
		loadLevels := []float64{0.5, 0.7, 0.9, 1.0, 1.2}

		for _, load := range loadLevels {
			response := simulateRequestWithLoad(load)

			t.Logf("Load %.0f%%: Response time %v, Success: %v",
				load*100, response.Latency, response.Success)

			// System should degrade gracefully, not fail completely
			if load <= 1.0 && !response.Success {
				t.Errorf("System failed at %.0f%% load", load*100)
			}

			// Response time should increase but not exponentially
			maxLatency := time.Second * time.Duration(load*5)
			if response.Latency > maxLatency {
				t.Errorf("Response time %v too high at %.0f%% load", response.Latency, load*100)
			}
		}
	})
}

// Response represents a simulated response
type Response struct {
	Success bool
	Latency time.Duration
}

// simulateRequestWithLoad simulates a request under load
func simulateRequestWithLoad(load float64) Response {
	baseLatency := time.Millisecond * 100
	latency := time.Duration(float64(baseLatency) * load * (1 + rand.Float64()))

	success := true
	if load > 1.0 {
		success = rand.Float64() > (load - 1.0)
	}

	return Response{
		Success: success,
		Latency: latency,
	}
}

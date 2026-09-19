# Workflow Testing Suite

Comprehensive testing framework for all 15 user journey workflows in the Social Protection Platform.

## Overview

This testing suite provides:

- **Mock Activities**: Simulated activity implementations for testing without external dependencies
- **Unit Tests**: Individual workflow testing with controlled inputs
- **Integration Tests**: End-to-end beneficiary lifecycle testing
- **Performance Benchmarks**: Throughput and latency measurements
- **Load Tests**: Concurrent workflow execution testing
- **Test Data Generators**: Automated test data creation

## Test Coverage

### Workflows Tested

1. **Journey 1**: Beneficiary Enrollment
2. **Journey 2**: KYC Verification
3. **Journey 3**: Benefit Card Issuance
4. **Journey 4**: Eligibility Re-certification
5. **Journey 5**: Benefit Suspension
6. **Journey 6**: Benefit Reinstatement
7. **Journey 7**: Beneficiary Exit
8. **Journey 8**: Next of Kin Transfer
9. **Journey 9**: Program Enrollment
10. **Journey 10**: Cross-Program Transfer
11. **Journey 11**: Disbursement Processing
12. **Journey 12**: Grievance Submission & Resolution
13. **Journey 13**: Fraud Investigation
14. **Journey 14**: Monthly Reporting
15. **Journey 15**: Program Performance Analytics

## Running Tests

### Prerequisites

```bash
# Start Temporal server
temporal server start-dev

# Start TigerBeetle (for financial transactions)
cd orchestrator
./tigerbeetle start --addresses=3000 0_0.tigerbeetle
```

### Unit Tests

```bash
cd orchestrator/testing
go test -v -run TestEnrollmentWorkflow
go test -v -run TestDisbursementWorkflow
go test -v -run TestGrievanceWorkflow
go test -v -run TestFraudInvestigationWorkflow
```

### Integration Tests

```bash
# End-to-end beneficiary lifecycle
go test -v -run TestEndToEndBeneficiaryLifecycle

# All integration tests
go test -v -run TestIntegration
```

### Performance Benchmarks

```bash
# Benchmark individual workflows
go test -bench=BenchmarkEnrollmentWorkflow -benchtime=10s
go test -bench=BenchmarkDisbursementWorkflow -benchtime=10s

# Benchmark all workflows
go test -bench=. -benchtime=30s
```

### Load Tests

```bash
# Run load test with 100 concurrent workflows
go test -v -run TestLoadTest -concurrent=100 -total=1000

# Stress test with high concurrency
go test -v -run TestStressTest -concurrent=500 -total=5000
```

## Mock Activity Configuration

Mock activities can be configured to simulate various scenarios:

```go
mockActivities := NewMockActivities()

// Simulate 10% failure rate
mockActivities.FailureRate = 0.1

// Add 500ms delay to each activity
mockActivities.DelayMs = 500

// Simulate timeouts
mockActivities.SimulateTimeouts = true
```

## Test Data Generation

Generate realistic test data for workflows:

```go
generator := NewTestDataGenerator()

// Generate single beneficiary
beneficiary := generator.GenerateBeneficiary()

// Generate batch of 1000 beneficiaries
batch := generator.GenerateBeneficiaryBatch(1000)

// Generate complete test scenarios
scenarios := generator.GenerateTestScenarios()
```

## Test Scenarios

### Happy Path Scenarios

1. **Complete Beneficiary Lifecycle**
   - Enrollment → KYC → Program Enrollment → Disbursement
   - Expected: All steps complete successfully

2. **Fraud Detection Path**
   - Enrollment → Fraud Alert → Investigation → Resolution
   - Expected: Fraud detected and handled appropriately

3. **Grievance Resolution Path**
   - Grievance Submission → Assignment → Investigation → Resolution
   - Expected: Grievance resolved within SLA

### Error Scenarios

1. **Failed KYC Verification**
   - Invalid documents → Verification failure → Manual review
   - Expected: Workflow handles failure gracefully

2. **Insufficient Budget**
   - Disbursement request → Budget check fails → Workflow aborts
   - Expected: Clear error message and rollback

3. **Timeout Scenarios**
   - Long-running activity → Timeout → Retry → Success/Failure
   - Expected: Proper retry logic with exponential backoff

## Performance Metrics

Expected performance benchmarks:

| Workflow Type | Avg Duration | P95 Duration | Throughput |
|--------------|--------------|--------------|------------|
| Enrollment | 2-5s | 8s | 200/s |
| KYC Verification | 3-7s | 12s | 150/s |
| Disbursement | 5-10s | 20s | 100/s |
| Grievance | 1-3s | 5s | 300/s |
| Fraud Investigation | 10-30s | 60s | 50/s |
| Reporting | 30-120s | 180s | 10/s |
| Analytics | 60-300s | 600s | 5/s |

## Continuous Testing

### Automated Test Execution

Tests run automatically on:

- Every code commit (via CI/CD)
- Nightly regression testing
- Before production deployments

### Test Monitoring

Test execution metrics are tracked in:

- Workflow monitoring dashboard (`/workflow-monitoring`)
- Test execution logs
- Performance trend analysis

## Troubleshooting

### Common Issues

1. **Temporal Connection Failed**
   ```bash
   # Check Temporal server is running
   temporal server status
   ```

2. **TigerBeetle Connection Failed**
   ```bash
   # Verify TigerBeetle is running on port 3000
   lsof -i :3000
   ```

3. **Test Timeouts**
   ```bash
   # Increase test timeout
   go test -timeout 30m -v
   ```

### Debug Mode

Enable verbose logging:

```go
// In test file
import "log"

func init() {
    log.SetFlags(log.LstdFlags | log.Lshortfile)
}
```

## Best Practices

1. **Isolation**: Each test should be independent and not rely on other tests
2. **Cleanup**: Always clean up test data after execution
3. **Determinism**: Use fixed seeds for random data in critical tests
4. **Timeouts**: Set appropriate timeouts for long-running workflows
5. **Assertions**: Verify both success and failure paths
6. **Metrics**: Collect and analyze performance metrics regularly

## Contributing

When adding new workflows:

1. Create mock activities in `workflow_tests.go`
2. Add unit tests for the workflow
3. Add integration tests if workflow interacts with others
4. Add performance benchmarks
5. Update this README with new workflow details

## Resources

- [Temporal Testing Documentation](https://docs.temporal.io/docs/go/testing)
- [Go Testing Package](https://pkg.go.dev/testing)
- [TigerBeetle Documentation](https://docs.tigerbeetle.com/)

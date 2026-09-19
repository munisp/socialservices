package workflows

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// ========== Journey 14: Monthly Reporting ==========

type MonthlyReportInput struct {
	ReportID     string   `json:"reportID"`
	ReportType   string   `json:"reportType"` // disbursement, enrollment, compliance, fraud, performance
	Month        string   `json:"month"`      // YYYY-MM format
	ProgramIDs   []string `json:"programIDs"` // empty for all programs
	Recipients   []string `json:"recipients"`
	GeneratedBy  string   `json:"generatedBy"`
}

type MonthlyReportResult struct {
	ReportID       string                 `json:"reportID"`
	Status         string                 `json:"status"`
	ReportURL      string                 `json:"reportURL"`
	GeneratedAt    time.Time              `json:"generatedAt"`
	Metrics        map[string]interface{} `json:"metrics"`
	DistributionID string                 `json:"distributionID"`
	Message        string                 `json:"message"`
}

func MonthlyReportingWorkflow(ctx workflow.Context, input MonthlyReportInput) (*MonthlyReportResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting monthly reporting workflow", "reportID", input.ReportID, "reportType", input.ReportType)

	activityOptions := workflow.ActivityOptions{
		StartToCloseTimeout: 60 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    5 * time.Minute,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, activityOptions)

	var result MonthlyReportResult
	result.ReportID = input.ReportID
	result.GeneratedAt = time.Now()

	// Step 1: Collect data from Lakehouse analytics
	var analyticsData map[string]interface{}
	err := workflow.ExecuteActivity(ctx, "CollectLakehouseDataActivity", input.ReportType, input.Month, input.ProgramIDs).Get(ctx, &analyticsData)
	if err != nil {
		return &result, fmt.Errorf("failed to collect analytics data: %w", err)
	}

	// Step 2: Aggregate metrics based on report type
	var metrics map[string]interface{}
	switch input.ReportType {
	case "disbursement":
		err = workflow.ExecuteActivity(ctx, "AggregateDisbursementMetricsActivity", analyticsData).Get(ctx, &metrics)
	case "enrollment":
		err = workflow.ExecuteActivity(ctx, "AggregateEnrollmentMetricsActivity", analyticsData).Get(ctx, &metrics)
	case "compliance":
		err = workflow.ExecuteActivity(ctx, "AggregateComplianceMetricsActivity", analyticsData).Get(ctx, &metrics)
	case "fraud":
		err = workflow.ExecuteActivity(ctx, "AggregateFraudMetricsActivity", analyticsData).Get(ctx, &metrics)
	case "performance":
		err = workflow.ExecuteActivity(ctx, "AggregateProgramPerformanceMetricsActivity", analyticsData).Get(ctx, &metrics)
	default:
		return &result, fmt.Errorf("unknown report type: %s", input.ReportType)
	}

	if err != nil {
		return &result, fmt.Errorf("failed to aggregate metrics: %w", err)
	}
	result.Metrics = metrics

	// Step 3: Generate visualizations (charts, graphs)
	var visualizations []string
	err = workflow.ExecuteActivity(ctx, "GenerateReportVisualizationsActivity", metrics, input.ReportType).Get(ctx, &visualizations)
	if err != nil {
		logger.Warn("Failed to generate visualizations", "error", err)
	}

	// Step 4: Generate PDF report
	var reportURL string
	err = workflow.ExecuteActivity(ctx, "GeneratePDFReportActivity", input.ReportID, input.ReportType, input.Month, metrics, visualizations).Get(ctx, &reportURL)
	if err != nil {
		return &result, fmt.Errorf("failed to generate PDF report: %w", err)
	}
	result.ReportURL = reportURL

	// Step 5: Store report in database
	err = workflow.ExecuteActivity(ctx, "StoreReportMetadataActivity", input.ReportID, input.ReportType, input.Month, reportURL, metrics).Get(ctx, nil)
	if err != nil {
		logger.Warn("Failed to store report metadata", "error", err)
	}

	// Step 6: Distribute report to recipients
	var distributionID string
	err = workflow.ExecuteActivity(ctx, "DistributeReportActivity", input.ReportID, reportURL, input.Recipients).Get(ctx, &distributionID)
	if err != nil {
		logger.Warn("Failed to distribute report", "error", err)
	}
	result.DistributionID = distributionID

	// Step 7: Publish event to Kafka
	err = workflow.ExecuteActivity(ctx, "PublishKafkaEventActivity", "report.generated", input.ReportID, result).Get(ctx, nil)
	if err != nil {
		logger.Warn("Failed to publish Kafka event", "error", err)
	}

	// Step 8: Ingest to Lakehouse for historical tracking
	err = workflow.ExecuteActivity(ctx, "IngestToLakehouseActivity", "report_generated", result).Get(ctx, nil)
	if err != nil {
		logger.Warn("Failed to ingest to Lakehouse", "error", err)
	}

	// Step 9: Create audit log
	err = workflow.ExecuteActivity(ctx, "CreateAuditLogActivity", "report.generated", input.ReportID, input.GeneratedBy, result).Get(ctx, nil)
	if err != nil {
		logger.Warn("Failed to create audit log", "error", err)
	}

	result.Status = "completed"
	result.Message = fmt.Sprintf("Monthly %s report generated successfully for %s", input.ReportType, input.Month)

	logger.Info("Monthly reporting completed", "reportID", input.ReportID, "reportURL", reportURL)
	return &result, nil
}

// ========== Journey 15: Program Performance Analytics ==========

type ProgramAnalyticsInput struct {
	AnalyticsID string   `json:"analyticsID"`
	ProgramIDs  []string `json:"programIDs"`
	TimeRange   string   `json:"timeRange"` // last_7_days, last_30_days, last_90_days, custom
	StartDate   string   `json:"startDate,omitempty"`
	EndDate     string   `json:"endDate,omitempty"`
	Metrics     []string `json:"metrics"` // enrollment_rate, disbursement_volume, fraud_rate, beneficiary_satisfaction, etc.
	RequestedBy string   `json:"requestedBy"`
}

type ProgramAnalyticsResult struct {
	AnalyticsID    string                 `json:"analyticsID"`
	Status         string                 `json:"status"`
	ProgramMetrics []ProgramMetric        `json:"programMetrics"`
	Insights       []Insight              `json:"insights"`
	Recommendations []string              `json:"recommendations"`
	DashboardURL   string                 `json:"dashboardURL"`
	Message        string                 `json:"message"`
}

type ProgramMetric struct {
	ProgramID          string                 `json:"programID"`
	ProgramName        string                 `json:"programName"`
	EnrollmentRate     float64                `json:"enrollmentRate"`
	DisbursementVolume float64                `json:"disbursementVolume"`
	FraudRate          float64                `json:"fraudRate"`
	ComplianceRate     float64                `json:"complianceRate"`
	BeneficiaryCount   int                    `json:"beneficiaryCount"`
	AverageBenefit     float64                `json:"averageBenefit"`
	CustomMetrics      map[string]interface{} `json:"customMetrics"`
}

type Insight struct {
	Type        string    `json:"type"` // trend, anomaly, prediction, comparison
	Description string    `json:"description"`
	Impact      string    `json:"impact"` // positive, negative, neutral
	Confidence  float64   `json:"confidence"`
	Timestamp   time.Time `json:"timestamp"`
}

func ProgramPerformanceAnalyticsWorkflow(ctx workflow.Context, input ProgramAnalyticsInput) (*ProgramAnalyticsResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting program performance analytics workflow", "analyticsID", input.AnalyticsID)

	activityOptions := workflow.ActivityOptions{
		StartToCloseTimeout: 60 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    5 * time.Minute,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, activityOptions)

	var result ProgramAnalyticsResult
	result.AnalyticsID = input.AnalyticsID

	// Step 1: Determine date range
	var startDate, endDate string
	err := workflow.ExecuteActivity(ctx, "CalculateDateRangeActivity", input.TimeRange, input.StartDate, input.EndDate).Get(ctx, &map[string]string{
		"start": startDate,
		"end":   endDate,
	})
	if err != nil {
		return &result, err
	}

	// Step 2: Collect program data from Lakehouse
	var programData []map[string]interface{}
	err = workflow.ExecuteActivity(ctx, "CollectProgramDataActivity", input.ProgramIDs, startDate, endDate).Get(ctx, &programData)
	if err != nil {
		return &result, fmt.Errorf("failed to collect program data: %w", err)
	}

	// Step 3: Calculate metrics for each program
	var programMetrics []ProgramMetric
	for _, programID := range input.ProgramIDs {
		var metric ProgramMetric
		err = workflow.ExecuteActivity(ctx, "CalculateProgramMetricsActivity", programID, startDate, endDate, input.Metrics).Get(ctx, &metric)
		if err != nil {
			logger.Warn("Failed to calculate metrics for program", "programID", programID, "error", err)
			continue
		}
		programMetrics = append(programMetrics, metric)
	}
	result.ProgramMetrics = programMetrics

	// Step 4: Run ML analysis for insights
	var mlInsights []Insight
	err = workflow.ExecuteActivity(ctx, "RunMLProgramAnalysisActivity", programMetrics, startDate, endDate).Get(ctx, &mlInsights)
	if err != nil {
		logger.Warn("ML analysis failed", "error", err)
	} else {
		result.Insights = mlInsights
	}

	// Step 5: Detect anomalies
	var anomalies []Insight
	err = workflow.ExecuteActivity(ctx, "DetectProgramAnomaliesActivity", programMetrics).Get(ctx, &anomalies)
	if err != nil {
		logger.Warn("Anomaly detection failed", "error", err)
	} else {
		result.Insights = append(result.Insights, anomalies...)
	}

	// Step 6: Generate trend analysis
	var trends []Insight
	err = workflow.ExecuteActivity(ctx, "AnalyzeProgramTrendsActivity", programMetrics, startDate, endDate).Get(ctx, &trends)
	if err != nil {
		logger.Warn("Trend analysis failed", "error", err)
	} else {
		result.Insights = append(result.Insights, trends...)
	}

	// Step 7: Generate forecasts
	var forecasts map[string]interface{}
	err = workflow.ExecuteActivity(ctx, "ForecastProgramPerformanceActivity", programMetrics).Get(ctx, &forecasts)
	if err != nil {
		logger.Warn("Forecasting failed", "error", err)
	}

	// Step 8: Generate recommendations based on insights
	var recommendations []string
	err = workflow.ExecuteActivity(ctx, "GenerateRecommendationsActivity", result.Insights, programMetrics).Get(ctx, &recommendations)
	if err != nil {
		logger.Warn("Recommendation generation failed", "error", err)
	} else {
		result.Recommendations = recommendations
	}

	// Step 9: Create interactive dashboard
	var dashboardURL string
	err = workflow.ExecuteActivity(ctx, "CreateAnalyticsDashboardActivity", input.AnalyticsID, programMetrics, result.Insights, forecasts).Get(ctx, &dashboardURL)
	if err != nil {
		logger.Warn("Dashboard creation failed", "error", err)
	}
	result.DashboardURL = dashboardURL

	// Step 10: Store analytics results
	err = workflow.ExecuteActivity(ctx, "StoreAnalyticsResultsActivity", input.AnalyticsID, result).Get(ctx, nil)
	if err != nil {
		logger.Warn("Failed to store analytics results", "error", err)
	}

	// Step 11: Publish event to Kafka
	err = workflow.ExecuteActivity(ctx, "PublishKafkaEventActivity", "analytics.completed", input.AnalyticsID, result).Get(ctx, nil)
	if err != nil {
		logger.Warn("Failed to publish Kafka event", "error", err)
	}

	// Step 12: Ingest to Lakehouse
	err = workflow.ExecuteActivity(ctx, "IngestToLakehouseActivity", "program_analytics", result).Get(ctx, nil)
	if err != nil {
		logger.Warn("Failed to ingest to Lakehouse", "error", err)
	}

	// Step 13: Send notification to requestor
	err = workflow.ExecuteActivity(ctx, "SendNotificationActivity", input.RequestedBy, fmt.Sprintf("Program analytics completed. Dashboard: %s", dashboardURL)).Get(ctx, nil)
	if err != nil {
		logger.Warn("Failed to send notification", "error", err)
	}

	result.Status = "completed"
	result.Message = fmt.Sprintf("Analytics completed for %d programs with %d insights and %d recommendations", len(programMetrics), len(result.Insights), len(result.Recommendations))

	logger.Info("Program performance analytics completed", "analyticsID", input.AnalyticsID, "programCount", len(programMetrics), "insightCount", len(result.Insights))
	return &result, nil
}

// ========== Scheduled Monthly Reporting Workflow ==========

type ScheduledReportConfig struct {
	ReportType  string   `json:"reportType"`
	Recipients  []string `json:"recipients"`
	ProgramIDs  []string `json:"programIDs"`
	DayOfMonth  int      `json:"dayOfMonth"` // 1-28
}

func ScheduledMonthlyReportWorkflow(ctx workflow.Context, config ScheduledReportConfig) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting scheduled monthly report workflow", "reportType", config.ReportType)

	// Calculate previous month
	now := workflow.Now(ctx)
	previousMonth := now.AddDate(0, -1, 0)
	monthStr := previousMonth.Format("2006-01")

	// Generate report ID
	reportID := fmt.Sprintf("SCHEDULED-%s-%s-%s", config.ReportType, monthStr, now.Format("20060102"))

	// Execute monthly reporting workflow
	input := MonthlyReportInput{
		ReportID:    reportID,
		ReportType:  config.ReportType,
		Month:       monthStr,
		ProgramIDs:  config.ProgramIDs,
		Recipients:  config.Recipients,
		GeneratedBy: "system",
	}

	var result MonthlyReportResult
	err := workflow.ExecuteActivity(ctx, "ExecuteMonthlyReportingWorkflow", input).Get(ctx, &result)
	if err != nil {
		logger.Error("Scheduled report generation failed", "error", err)
		return err
	}

	logger.Info("Scheduled monthly report completed", "reportID", reportID, "reportURL", result.ReportURL)
	return nil
}

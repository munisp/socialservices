package journeys

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/workflow"
)

// Reporting Journey Definitions (Journeys 23-26)
// These journeys map to existing implemented components:
// - MonthlyReportingWorkflow, ProgramPerformanceAnalyticsWorkflow
// - UI: Reports page, Analytics dashboard
// - BFF: analytics router, workflow router

// Journey 23: Monthly Reporting
// UI Entry: Admin Reports page
// BFF: trpc.analytics.generateReport
// Workflow: MonthlyReportingWorkflow
type MonthlyReportingInput struct {
	JourneyContext *JourneyContext `json:"journeyContext"`
	ReportType     string          `json:"reportType"` // "monthly", "quarterly", "annual"
	Period         string          `json:"period"`     // "2024-01", "2024-Q1", "2024"
	ProgramIDs     []string        `json:"programIds,omitempty"`
	Regions        []string        `json:"regions,omitempty"`
	Format         string          `json:"format"` // "pdf", "excel", "csv"
}

type MonthlyReportingResult struct {
	JourneyRunID  string    `json:"journeyRunId"`
	ReportID      string    `json:"reportId"`
	ReportType    string    `json:"reportType"`
	Period        string    `json:"period"`
	FileURL       string    `json:"fileUrl"`
	GeneratedAt   time.Time `json:"generatedAt"`
}

// MonthlyReportingJourney orchestrates monthly report generation
func MonthlyReportingJourney(ctx workflow.Context, input MonthlyReportingInput) (*MonthlyReportingResult, error) {
	jc := input.JourneyContext
	
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Minute,
		RetryPolicy: &workflow.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    time.Minute,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)
	
	// Step 1: Emit journey started event
	workflow.ExecuteActivity(ctx, EmitJourneyEventActivity, jc, "journey.started", map[string]interface{}{
		"journeyType": "monthly_reporting",
		"reportType":  input.ReportType,
		"period":      input.Period,
	})
	
	// Step 2: Check authorization
	var authorized bool
	err := workflow.ExecuteActivity(ctx, CheckAuthorizationActivity, jc, "report:generate").Get(ctx, &authorized)
	if err != nil || !authorized {
		return nil, fmt.Errorf("authorization failed")
	}
	
	// Step 3: Create report record
	var reportID string
	err = workflow.ExecuteActivity(ctx, CreateReportRecordActivity, map[string]interface{}{
		"reportType": input.ReportType,
		"period":     input.Period,
		"programIds": input.ProgramIDs,
		"regions":    input.Regions,
		"format":     input.Format,
		"status":     "generating",
		"createdBy":  jc.ActorID,
		"tenantId":   jc.TenantID,
	}).Get(ctx, &reportID)
	if err != nil {
		return nil, fmt.Errorf("failed to create report record: %v", err)
	}
	
	// Step 4: Query lakehouse for enrollment data
	var enrollmentData map[string]interface{}
	err = workflow.ExecuteActivity(ctx, QueryLakehouseEnrollmentDataActivity, input.Period, input.ProgramIDs, input.Regions).Get(ctx, &enrollmentData)
	if err != nil {
		return nil, fmt.Errorf("failed to query enrollment data: %v", err)
	}
	
	// Step 5: Query lakehouse for disbursement data
	var disbursementData map[string]interface{}
	err = workflow.ExecuteActivity(ctx, QueryLakehouseDisbursementDataActivity, input.Period, input.ProgramIDs, input.Regions).Get(ctx, &disbursementData)
	if err != nil {
		return nil, fmt.Errorf("failed to query disbursement data: %v", err)
	}
	
	// Step 6: Query lakehouse for grievance data
	var grievanceData map[string]interface{}
	err = workflow.ExecuteActivity(ctx, QueryLakehouseGrievanceDataActivity, input.Period, input.ProgramIDs, input.Regions).Get(ctx, &grievanceData)
	if err != nil {
		return nil, fmt.Errorf("failed to query grievance data: %v", err)
	}
	
	// Step 7: Calculate KPIs
	var kpis map[string]interface{}
	err = workflow.ExecuteActivity(ctx, CalculateReportKPIsActivity, enrollmentData, disbursementData, grievanceData).Get(ctx, &kpis)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate KPIs: %v", err)
	}
	
	// Step 8: Generate report file
	var fileURL string
	err = workflow.ExecuteActivity(ctx, GenerateReportFileActivity, map[string]interface{}{
		"reportId":         reportID,
		"reportType":       input.ReportType,
		"period":           input.Period,
		"format":           input.Format,
		"enrollmentData":   enrollmentData,
		"disbursementData": disbursementData,
		"grievanceData":    grievanceData,
		"kpis":             kpis,
	}).Get(ctx, &fileURL)
	if err != nil {
		return nil, fmt.Errorf("failed to generate report file: %v", err)
	}
	
	// Step 9: Update report record
	workflow.ExecuteActivity(ctx, UpdateReportRecordActivity, reportID, map[string]interface{}{
		"status":  "completed",
		"fileUrl": fileURL,
	})
	
	// Step 10: Publish Kafka event
	workflow.ExecuteActivity(ctx, PublishKafkaEventActivity, "report.generated", map[string]interface{}{
		"reportId":      reportID,
		"reportType":    input.ReportType,
		"period":        input.Period,
		"fileUrl":       fileURL,
		"correlationId": jc.CorrelationID,
	})
	
	// Step 11: Create audit log
	workflow.ExecuteActivity(ctx, CreateAuditLogActivity, map[string]interface{}{
		"action":        "report_generated",
		"entityType":    "report",
		"entityId":      reportID,
		"performedBy":   jc.ActorID,
		"tenantId":      jc.TenantID,
		"correlationId": jc.CorrelationID,
		"details": map[string]interface{}{
			"reportType": input.ReportType,
			"period":     input.Period,
			"format":     input.Format,
		},
	})
	
	// Step 12: Emit journey completed event
	workflow.ExecuteActivity(ctx, EmitJourneyEventActivity, jc, "journey.completed", map[string]interface{}{
		"reportId": reportID,
		"fileUrl":  fileURL,
	})
	
	return &MonthlyReportingResult{
		JourneyRunID: jc.JourneyRunID,
		ReportID:     reportID,
		ReportType:   input.ReportType,
		Period:       input.Period,
		FileURL:      fileURL,
		GeneratedAt:  time.Now(),
	}, nil
}

// Journey 24: Program Performance Analytics
// UI Entry: Admin Analytics dashboard
// BFF: trpc.analytics.programPerformance
// Workflow: ProgramPerformanceAnalyticsWorkflow
type ProgramAnalyticsInput struct {
	JourneyContext *JourneyContext `json:"journeyContext"`
	ProgramID      string          `json:"programId"`
	StartDate      string          `json:"startDate"`
	EndDate        string          `json:"endDate"`
	Metrics        []string        `json:"metrics"` // "enrollment_rate", "disbursement_rate", "grievance_rate", etc.
}

type ProgramAnalyticsResult struct {
	JourneyRunID  string                 `json:"journeyRunId"`
	ProgramID     string                 `json:"programId"`
	Metrics       map[string]interface{} `json:"metrics"`
	Trends        map[string]interface{} `json:"trends"`
	Comparisons   map[string]interface{} `json:"comparisons"`
	GeneratedAt   time.Time              `json:"generatedAt"`
}

// ProgramAnalyticsJourney orchestrates program performance analytics
func ProgramAnalyticsJourney(ctx workflow.Context, input ProgramAnalyticsInput) (*ProgramAnalyticsResult, error) {
	jc := input.JourneyContext
	
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 15 * time.Minute,
		RetryPolicy: &workflow.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    time.Minute,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)
	
	// Step 1: Emit journey started event
	workflow.ExecuteActivity(ctx, EmitJourneyEventActivity, jc, "journey.started", map[string]interface{}{
		"journeyType": "program_analytics",
		"programId":   input.ProgramID,
	})
	
	// Step 2: Check authorization
	var authorized bool
	err := workflow.ExecuteActivity(ctx, CheckAuthorizationActivity, jc, "analytics:view").Get(ctx, &authorized)
	if err != nil || !authorized {
		return nil, fmt.Errorf("authorization failed")
	}
	
	// Step 3: Verify program exists
	var programExists bool
	err = workflow.ExecuteActivity(ctx, CheckProgramExistsActivity, input.ProgramID).Get(ctx, &programExists)
	if err != nil || !programExists {
		return nil, fmt.Errorf("program not found")
	}
	
	// Step 4: Query lakehouse for program metrics
	var metrics map[string]interface{}
	err = workflow.ExecuteActivity(ctx, QueryProgramMetricsActivity, input.ProgramID, input.StartDate, input.EndDate, input.Metrics).Get(ctx, &metrics)
	if err != nil {
		return nil, fmt.Errorf("failed to query metrics: %v", err)
	}
	
	// Step 5: Calculate trends
	var trends map[string]interface{}
	err = workflow.ExecuteActivity(ctx, CalculateProgramTrendsActivity, input.ProgramID, input.StartDate, input.EndDate).Get(ctx, &trends)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate trends: %v", err)
	}
	
	// Step 6: Get comparison with other programs
	var comparisons map[string]interface{}
	err = workflow.ExecuteActivity(ctx, GetProgramComparisonsActivity, input.ProgramID, input.StartDate, input.EndDate).Get(ctx, &comparisons)
	if err != nil {
		workflow.GetLogger(ctx).Warn("Failed to get comparisons", "error", err)
		comparisons = make(map[string]interface{})
	}
	
	// Step 7: Cache results in Redis
	workflow.ExecuteActivity(ctx, CacheAnalyticsResultsActivity, fmt.Sprintf("program:%s:analytics", input.ProgramID), map[string]interface{}{
		"metrics":     metrics,
		"trends":      trends,
		"comparisons": comparisons,
		"generatedAt": time.Now(),
	})
	
	// Step 8: Publish Kafka event
	workflow.ExecuteActivity(ctx, PublishKafkaEventActivity, "analytics.program_generated", map[string]interface{}{
		"programId":     input.ProgramID,
		"startDate":     input.StartDate,
		"endDate":       input.EndDate,
		"correlationId": jc.CorrelationID,
	})
	
	// Step 9: Create audit log
	workflow.ExecuteActivity(ctx, CreateAuditLogActivity, map[string]interface{}{
		"action":        "program_analytics_generated",
		"entityType":    "program",
		"entityId":      input.ProgramID,
		"performedBy":   jc.ActorID,
		"tenantId":      jc.TenantID,
		"correlationId": jc.CorrelationID,
		"details": map[string]interface{}{
			"startDate": input.StartDate,
			"endDate":   input.EndDate,
			"metrics":   input.Metrics,
		},
	})
	
	// Step 10: Emit journey completed event
	workflow.ExecuteActivity(ctx, EmitJourneyEventActivity, jc, "journey.completed", map[string]interface{}{
		"programId": input.ProgramID,
	})
	
	return &ProgramAnalyticsResult{
		JourneyRunID: jc.JourneyRunID,
		ProgramID:    input.ProgramID,
		Metrics:      metrics,
		Trends:       trends,
		Comparisons:  comparisons,
		GeneratedAt:  time.Now(),
	}, nil
}

// Journey 25: Dashboard Refresh
// UI Entry: Admin Dashboard (automatic refresh)
// BFF: trpc.analytics.refreshDashboard
// Workflow: DashboardRefreshWorkflow
type DashboardRefreshInput struct {
	JourneyContext *JourneyContext `json:"journeyContext"`
	DashboardType  string          `json:"dashboardType"` // "executive", "operations", "program"
	ProgramID      string          `json:"programId,omitempty"`
}

type DashboardRefreshResult struct {
	JourneyRunID  string                 `json:"journeyRunId"`
	DashboardType string                 `json:"dashboardType"`
	Widgets       map[string]interface{} `json:"widgets"`
	RefreshedAt   time.Time              `json:"refreshedAt"`
}

// DashboardRefreshJourney orchestrates dashboard data refresh
func DashboardRefreshJourney(ctx workflow.Context, input DashboardRefreshInput) (*DashboardRefreshResult, error) {
	jc := input.JourneyContext
	
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
		RetryPolicy: &workflow.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    30 * time.Second,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)
	
	// Step 1: Emit journey started event
	workflow.ExecuteActivity(ctx, EmitJourneyEventActivity, jc, "journey.started", map[string]interface{}{
		"journeyType":   "dashboard_refresh",
		"dashboardType": input.DashboardType,
	})
	
	// Step 2: Check authorization
	var authorized bool
	err := workflow.ExecuteActivity(ctx, CheckAuthorizationActivity, jc, "dashboard:view").Get(ctx, &authorized)
	if err != nil || !authorized {
		return nil, fmt.Errorf("authorization failed")
	}
	
	widgets := make(map[string]interface{})
	
	// Step 3: Refresh widgets based on dashboard type
	switch input.DashboardType {
	case "executive":
		// Total beneficiaries
		var totalBeneficiaries int
		workflow.ExecuteActivity(ctx, GetTotalBeneficiariesActivity, jc.TenantID).Get(ctx, &totalBeneficiaries)
		widgets["totalBeneficiaries"] = totalBeneficiaries
		
		// Total disbursements
		var totalDisbursements float64
		workflow.ExecuteActivity(ctx, GetTotalDisbursementsActivity, jc.TenantID).Get(ctx, &totalDisbursements)
		widgets["totalDisbursements"] = totalDisbursements
		
		// Active programs
		var activePrograms int
		workflow.ExecuteActivity(ctx, GetActiveProgramsCountActivity, jc.TenantID).Get(ctx, &activePrograms)
		widgets["activePrograms"] = activePrograms
		
		// Pending grievances
		var pendingGrievances int
		workflow.ExecuteActivity(ctx, GetPendingGrievancesCountActivity, jc.TenantID).Get(ctx, &pendingGrievances)
		widgets["pendingGrievances"] = pendingGrievances
		
		// Monthly trends
		var monthlyTrends map[string]interface{}
		workflow.ExecuteActivity(ctx, GetMonthlyTrendsActivity, jc.TenantID).Get(ctx, &monthlyTrends)
		widgets["monthlyTrends"] = monthlyTrends
		
	case "operations":
		// Pending approvals
		var pendingApprovals int
		workflow.ExecuteActivity(ctx, GetPendingApprovalsCountActivity, jc.TenantID).Get(ctx, &pendingApprovals)
		widgets["pendingApprovals"] = pendingApprovals
		
		// Failed disbursements
		var failedDisbursements int
		workflow.ExecuteActivity(ctx, GetFailedDisbursementsCountActivity, jc.TenantID).Get(ctx, &failedDisbursements)
		widgets["failedDisbursements"] = failedDisbursements
		
		// SLA breaches
		var slaBreaches int
		workflow.ExecuteActivity(ctx, GetSLABreachesCountActivity, jc.TenantID).Get(ctx, &slaBreaches)
		widgets["slaBreaches"] = slaBreaches
		
		// System health
		var systemHealth map[string]interface{}
		workflow.ExecuteActivity(ctx, GetSystemHealthActivity).Get(ctx, &systemHealth)
		widgets["systemHealth"] = systemHealth
		
	case "program":
		if input.ProgramID == "" {
			return nil, fmt.Errorf("programId required for program dashboard")
		}
		
		// Program enrollment
		var enrollment int
		workflow.ExecuteActivity(ctx, GetProgramEnrollmentActivity, input.ProgramID).Get(ctx, &enrollment)
		widgets["enrollment"] = enrollment
		
		// Program disbursements
		var disbursements float64
		workflow.ExecuteActivity(ctx, GetProgramDisbursementsActivity, input.ProgramID).Get(ctx, &disbursements)
		widgets["disbursements"] = disbursements
		
		// Program grievances
		var grievances int
		workflow.ExecuteActivity(ctx, GetProgramGrievancesActivity, input.ProgramID).Get(ctx, &grievances)
		widgets["grievances"] = grievances
		
		// Program budget utilization
		var budgetUtilization float64
		workflow.ExecuteActivity(ctx, GetProgramBudgetUtilizationActivity, input.ProgramID).Get(ctx, &budgetUtilization)
		widgets["budgetUtilization"] = budgetUtilization
	}
	
	// Step 4: Cache dashboard data
	cacheKey := fmt.Sprintf("dashboard:%s:%s", input.DashboardType, jc.TenantID)
	if input.ProgramID != "" {
		cacheKey = fmt.Sprintf("dashboard:%s:%s:%s", input.DashboardType, input.ProgramID, jc.TenantID)
	}
	workflow.ExecuteActivity(ctx, CacheDashboardDataActivity, cacheKey, widgets)
	
	// Step 5: Emit journey completed event
	workflow.ExecuteActivity(ctx, EmitJourneyEventActivity, jc, "journey.completed", map[string]interface{}{
		"dashboardType": input.DashboardType,
		"widgetCount":   len(widgets),
	})
	
	return &DashboardRefreshResult{
		JourneyRunID:  jc.JourneyRunID,
		DashboardType: input.DashboardType,
		Widgets:       widgets,
		RefreshedAt:   time.Now(),
	}, nil
}

// Journey 26: Data Export
// UI Entry: Admin Data export page
// BFF: trpc.analytics.export
// Workflow: DataExportWorkflow
type DataExportInput struct {
	JourneyContext *JourneyContext `json:"journeyContext"`
	ExportType     string          `json:"exportType"` // "beneficiaries", "disbursements", "grievances", "audit"
	Filters        map[string]interface{} `json:"filters,omitempty"`
	Format         string          `json:"format"` // "csv", "excel", "json"
	DateRange      *DateRange      `json:"dateRange,omitempty"`
}

type DateRange struct {
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
}

type DataExportResult struct {
	JourneyRunID  string    `json:"journeyRunId"`
	ExportID      string    `json:"exportId"`
	ExportType    string    `json:"exportType"`
	RecordCount   int       `json:"recordCount"`
	FileURL       string    `json:"fileUrl"`
	ExportedAt    time.Time `json:"exportedAt"`
}

// DataExportJourney orchestrates data export
func DataExportJourney(ctx workflow.Context, input DataExportInput) (*DataExportResult, error) {
	jc := input.JourneyContext
	
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 60 * time.Minute,
		RetryPolicy: &workflow.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    time.Minute,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)
	
	// Step 1: Emit journey started event
	workflow.ExecuteActivity(ctx, EmitJourneyEventActivity, jc, "journey.started", map[string]interface{}{
		"journeyType": "data_export",
		"exportType":  input.ExportType,
	})
	
	// Step 2: Check authorization
	permission := fmt.Sprintf("export:%s", input.ExportType)
	var authorized bool
	err := workflow.ExecuteActivity(ctx, CheckAuthorizationActivity, jc, permission).Get(ctx, &authorized)
	if err != nil || !authorized {
		return nil, fmt.Errorf("authorization failed")
	}
	
	// Step 3: Create export record
	var exportID string
	err = workflow.ExecuteActivity(ctx, CreateExportRecordActivity, map[string]interface{}{
		"exportType": input.ExportType,
		"filters":    input.Filters,
		"format":     input.Format,
		"dateRange":  input.DateRange,
		"status":     "processing",
		"createdBy":  jc.ActorID,
		"tenantId":   jc.TenantID,
	}).Get(ctx, &exportID)
	if err != nil {
		return nil, fmt.Errorf("failed to create export record: %v", err)
	}
	
	// Step 4: Query data based on export type
	var data []map[string]interface{}
	switch input.ExportType {
	case "beneficiaries":
		err = workflow.ExecuteActivity(ctx, QueryBeneficiariesForExportActivity, input.Filters, input.DateRange, jc.TenantID).Get(ctx, &data)
	case "disbursements":
		err = workflow.ExecuteActivity(ctx, QueryDisbursementsForExportActivity, input.Filters, input.DateRange, jc.TenantID).Get(ctx, &data)
	case "grievances":
		err = workflow.ExecuteActivity(ctx, QueryGrievancesForExportActivity, input.Filters, input.DateRange, jc.TenantID).Get(ctx, &data)
	case "audit":
		err = workflow.ExecuteActivity(ctx, QueryAuditLogsForExportActivity, input.Filters, input.DateRange, jc.TenantID).Get(ctx, &data)
	default:
		return nil, fmt.Errorf("unsupported export type: %s", input.ExportType)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query data: %v", err)
	}
	
	// Step 5: Generate export file
	var fileURL string
	err = workflow.ExecuteActivity(ctx, GenerateExportFileActivity, exportID, input.Format, data).Get(ctx, &fileURL)
	if err != nil {
		return nil, fmt.Errorf("failed to generate export file: %v", err)
	}
	
	// Step 6: Update export record
	workflow.ExecuteActivity(ctx, UpdateExportRecordActivity, exportID, map[string]interface{}{
		"status":      "completed",
		"recordCount": len(data),
		"fileUrl":     fileURL,
	})
	
	// Step 7: Publish Kafka event
	workflow.ExecuteActivity(ctx, PublishKafkaEventActivity, "export.completed", map[string]interface{}{
		"exportId":      exportID,
		"exportType":    input.ExportType,
		"recordCount":   len(data),
		"fileUrl":       fileURL,
		"correlationId": jc.CorrelationID,
	})
	
	// Step 8: Create audit log
	workflow.ExecuteActivity(ctx, CreateAuditLogActivity, map[string]interface{}{
		"action":        "data_exported",
		"entityType":    "export",
		"entityId":      exportID,
		"performedBy":   jc.ActorID,
		"tenantId":      jc.TenantID,
		"correlationId": jc.CorrelationID,
		"details": map[string]interface{}{
			"exportType":  input.ExportType,
			"recordCount": len(data),
			"format":      input.Format,
		},
	})
	
	// Step 9: Emit journey completed event
	workflow.ExecuteActivity(ctx, EmitJourneyEventActivity, jc, "journey.completed", map[string]interface{}{
		"exportId":    exportID,
		"recordCount": len(data),
	})
	
	return &DataExportResult{
		JourneyRunID: jc.JourneyRunID,
		ExportID:     exportID,
		ExportType:   input.ExportType,
		RecordCount:  len(data),
		FileURL:      fileURL,
		ExportedAt:   time.Now(),
	}, nil
}

// RegisterReportingJourneys registers all reporting journey definitions
func RegisterReportingJourneys(registry *JourneyRegistry) {
	registry.Register(&JourneyDefinition{
		Key:          "monthly_reporting",
		Name:         "Monthly Reporting",
		Description:  "Generate monthly/quarterly/annual reports",
		Category:     "reporting",
		WorkflowType: "MonthlyReportingJourney",
		RequiredPermissions: []string{"report:generate"},
		UIEntryPoints:       []string{"Admin:ReportsPage"},
		BFFEndpoints:        []string{"trpc.analytics.generateReport"},
		MiddlewareHooks:     []string{"kafka", "redis", "permify", "lakehouse", "rustfs"},
	})
	
	registry.Register(&JourneyDefinition{
		Key:          "program_analytics",
		Name:         "Program Performance Analytics",
		Description:  "Analyze program performance with trends and comparisons",
		Category:     "reporting",
		WorkflowType: "ProgramAnalyticsJourney",
		RequiredPermissions: []string{"analytics:view"},
		UIEntryPoints:       []string{"Admin:AnalyticsDashboard"},
		BFFEndpoints:        []string{"trpc.analytics.programPerformance"},
		MiddlewareHooks:     []string{"kafka", "redis", "permify", "lakehouse"},
	})
	
	registry.Register(&JourneyDefinition{
		Key:          "dashboard_refresh",
		Name:         "Dashboard Refresh",
		Description:  "Refresh dashboard widgets with latest data",
		Category:     "reporting",
		WorkflowType: "DashboardRefreshJourney",
		RequiredPermissions: []string{"dashboard:view"},
		UIEntryPoints:       []string{"Admin:Dashboard"},
		BFFEndpoints:        []string{"trpc.analytics.refreshDashboard"},
		MiddlewareHooks:     []string{"redis", "permify"},
	})
	
	registry.Register(&JourneyDefinition{
		Key:          "data_export",
		Name:         "Data Export",
		Description:  "Export data to CSV/Excel/JSON",
		Category:     "reporting",
		WorkflowType: "DataExportJourney",
		RequiredPermissions: []string{"export:execute"},
		UIEntryPoints:       []string{"Admin:DataExportPage"},
		BFFEndpoints:        []string{"trpc.analytics.export"},
		MiddlewareHooks:     []string{"kafka", "redis", "permify", "lakehouse", "rustfs"},
	})
}

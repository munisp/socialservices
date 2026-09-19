package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/admin-portal/orchestrator/activities"
	"github.com/admin-portal/orchestrator/api"
	"github.com/admin-portal/orchestrator/clients"
	"github.com/admin-portal/orchestrator/config"
	"github.com/admin-portal/orchestrator/journeys"
	"github.com/admin-portal/orchestrator/workflows"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

const (
	TaskQueue = "admin-portal-orchestrator"
)

func main() {
	// Load configuration
	cfg := config.LoadConfig()

	// Initialize middleware clients
	kafkaClient := clients.NewKafkaClient(cfg.KafkaBrokers)
	defer kafkaClient.Close()

	daprClient, err := clients.NewDaprClient()
	if err != nil {
		log.Fatalf("Failed to create Dapr client: %v", err)
	}
	defer daprClient.Close()

	redisClient := clients.NewRedisClient(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
	defer redisClient.Close()

	// Create Temporal client
	temporalClient, err := client.Dial(client.Options{
		HostPort:  cfg.TemporalHostPort,
		Namespace: cfg.TemporalNamespace,
	})
	if err != nil {
		log.Fatalf("Failed to create Temporal client: %v", err)
	}
	defer temporalClient.Close()

	// Create worker
	w := worker.New(temporalClient, TaskQueue, worker.Options{})

	// Register all domain workflows
	w.RegisterWorkflow(workflows.EnrollBeneficiaryWorkflow)
	w.RegisterWorkflow(workflows.KYCVerificationWorkflow)
	w.RegisterWorkflow(workflows.CardIssuanceWorkflow)
	w.RegisterWorkflow(workflows.MonthlyDisbursementWorkflow)
	w.RegisterWorkflow(workflows.FraudInvestigationWorkflow)
	w.RegisterWorkflow(workflows.ProfileUpdateWorkflow)
	w.RegisterWorkflow(workflows.ProgramEnrollmentWorkflow)
	w.RegisterWorkflow(workflows.BeneficiaryLifecycleWorkflow)
	w.RegisterWorkflow(workflows.GrievanceWorkflow)
	w.RegisterWorkflow(workflows.DisbursementWorkflow)
	w.RegisterWorkflow(workflows.ReportingWorkflow)
	w.RegisterWorkflow(workflows.ReconciliationWorkflow)
	w.RegisterWorkflow(workflows.BulkOperationsWorkflow)
	w.RegisterWorkflow(workflows.ApprovalDelegationWorkflow)

	// Register journey workflows (30 user journeys)
	// Enrollment journeys (1-5)
	w.RegisterWorkflow(journeys.BeneficiaryEnrollmentJourney)
	w.RegisterWorkflow(journeys.KYCVerificationJourney)
	w.RegisterWorkflow(journeys.HouseholdRegistrationJourney)
	w.RegisterWorkflow(journeys.ProgramEnrollmentJourney)
	w.RegisterWorkflow(journeys.CardIssuanceJourney)
	
	// Payment journeys (6-10)
	w.RegisterWorkflow(journeys.DisbursementScheduleJourney)
	w.RegisterWorkflow(journeys.DisbursementExecuteJourney)
	w.RegisterWorkflow(journeys.RetryDisbursementJourney)
	w.RegisterWorkflow(journeys.ReconciliationJourney)
	w.RegisterWorkflow(journeys.DisputeResolutionJourney)
	
	// Grievance journeys (11-13)
	w.RegisterWorkflow(journeys.GrievanceSubmissionJourney)
	w.RegisterWorkflow(journeys.GrievanceResolutionJourney)
	w.RegisterWorkflow(journeys.GrievanceEscalationJourney)
	
	// Lifecycle journeys (14-18)
	w.RegisterWorkflow(journeys.ProfileUpdateJourney)
	w.RegisterWorkflow(journeys.BeneficiarySuspensionJourney)
	w.RegisterWorkflow(journeys.BeneficiaryReactivationJourney)
	w.RegisterWorkflow(journeys.BeneficiaryExitJourney)
	w.RegisterWorkflow(journeys.DeathRegistrationJourney)
	
	// Admin journeys (19-22)
	w.RegisterWorkflow(journeys.ApprovalDelegationJourney)
	w.RegisterWorkflow(journeys.BreakGlassAccessJourney)
	w.RegisterWorkflow(journeys.BulkOperationJourney)
	w.RegisterWorkflow(journeys.OfflineSyncJourney)
	
	// Reporting journeys (23-26)
	w.RegisterWorkflow(journeys.MonthlyReportingJourney)
	w.RegisterWorkflow(journeys.ProgramAnalyticsJourney)
	w.RegisterWorkflow(journeys.DashboardRefreshJourney)
	w.RegisterWorkflow(journeys.DataExportJourney)
	
	// Fraud/Compliance journeys (27-30)
	w.RegisterWorkflow(journeys.FraudInvestigationJourney)
	w.RegisterWorkflow(journeys.FraudPredictionJourney)
	w.RegisterWorkflow(journeys.NationalIDVerificationJourney)
	w.RegisterWorkflow(journeys.CrossSectorInteropJourney)

	// Register domain activities
	beneficiaryActivities := activities.NewBeneficiaryActivities(kafkaClient, daprClient, redisClient)
	w.RegisterActivity(beneficiaryActivities.ValidateNationalIDActivity)
	w.RegisterActivity(beneficiaryActivities.CheckDuplicateBeneficiaryActivity)
	w.RegisterActivity(beneficiaryActivities.ValidateDocumentsActivity)
	w.RegisterActivity(beneficiaryActivities.CreateBeneficiaryRecordActivity)
	w.RegisterActivity(beneficiaryActivities.CreateTigerBeetleAccountActivity)
	w.RegisterActivity(beneficiaryActivities.PublishKafkaEventActivity)
	w.RegisterActivity(beneficiaryActivities.CacheBeneficiaryDataActivity)
	w.RegisterActivity(beneficiaryActivities.SendSMSNotificationActivity)
	w.RegisterActivity(beneficiaryActivities.CreateAuditLogActivity)

	// Register journey activities (shared across all journeys)
	w.RegisterActivity(journeys.EmitJourneyEventActivity)
	w.RegisterActivity(journeys.CheckAuthorizationActivity)
	w.RegisterActivity(journeys.CheckIdempotencyActivity)
	w.RegisterActivity(journeys.SetIdempotencyActivity)
	w.RegisterActivity(journeys.PublishKafkaEventActivity)
	w.RegisterActivity(journeys.CreateAuditLogActivity)
	w.RegisterActivity(journeys.WriteLakehouseFactActivity)
	w.RegisterActivity(journeys.CacheBeneficiaryDataActivity)
	
	// Enrollment activities
	w.RegisterActivity(journeys.ValidateBeneficiaryDataActivity)
	w.RegisterActivity(journeys.CreateBeneficiaryRecordActivity)
	w.RegisterActivity(journeys.CreateTigerBeetleAccountActivity)
	w.RegisterActivity(journeys.SendWelcomeNotificationActivity)
	w.RegisterActivity(journeys.VerifyIdentityActivity)
	w.RegisterActivity(journeys.UpdateKYCStatusActivity)
	w.RegisterActivity(journeys.CreateHouseholdRecordActivity)
	w.RegisterActivity(journeys.AddHouseholdMemberActivity)
	w.RegisterActivity(journeys.CalculatePMTScoreActivity)
	w.RegisterActivity(journeys.EnrollInProgramActivity)
	w.RegisterActivity(journeys.IssuePaymentCardActivity)
	
	// Payment activities
	w.RegisterActivity(journeys.ValidateDisbursementBudgetActivity)
	w.RegisterActivity(journeys.GetEligibleBeneficiariesActivity)
	w.RegisterActivity(journeys.CreateDisbursementRecordActivity)
	w.RegisterActivity(journeys.CreatePendingTransferActivity)
	w.RegisterActivity(journeys.ExecuteMojaloopTransferActivity)
	w.RegisterActivity(journeys.PostPendingTransferActivity)
	w.RegisterActivity(journeys.VoidPendingTransferActivity)
	w.RegisterActivity(journeys.UpdateDisbursementStatusActivity)
	w.RegisterActivity(journeys.SendPaymentNotificationActivity)
	
	// Grievance activities
	w.RegisterActivity(journeys.CreateGrievanceRecordActivity)
	w.RegisterActivity(journeys.AutoAssignGrievanceActivity)
	w.RegisterActivity(journeys.CalculateSLADeadlineActivity)
	w.RegisterActivity(journeys.UpdateGrievanceStatusActivity)
	w.RegisterActivity(journeys.ProcessCompensationActivity)
	
	// Lifecycle activities
	w.RegisterActivity(journeys.GetBeneficiaryProfileActivity)
	w.RegisterActivity(journeys.ValidateProfileUpdatesActivity)
	w.RegisterActivity(journeys.UpdateBeneficiaryProfileActivity)
	w.RegisterActivity(journeys.GetBeneficiaryStatusActivity)
	w.RegisterActivity(journeys.SuspendBeneficiaryActivity)
	w.RegisterActivity(journeys.ReactivateBeneficiaryActivity)
	w.RegisterActivity(journeys.BlockTigerBeetleAccountActivity)
	w.RegisterActivity(journeys.UnblockTigerBeetleAccountActivity)
	w.RegisterActivity(journeys.CancelPendingDisbursementsActivity)
	w.RegisterActivity(journeys.SendSMSNotificationActivity)
	w.RegisterActivity(journeys.SendSuspensionNotificationActivity)
	w.RegisterActivity(journeys.SendReactivationNotificationActivity)
	
	// Admin activities
	w.RegisterActivity(journeys.CheckAdminRoleActivity)
	w.RegisterActivity(journeys.CheckUserActiveActivity)
	w.RegisterActivity(journeys.CheckDelegationConflictActivity)
	w.RegisterActivity(journeys.CreateDelegationRecordActivity)
	w.RegisterActivity(journeys.GrantDelegationPermissionsActivity)
	w.RegisterActivity(journeys.SendDelegationNotificationActivity)
	w.RegisterActivity(journeys.CreateBreakGlassRecordActivity)
	w.RegisterActivity(journeys.GrantTemporaryPermissionsActivity)
	w.RegisterActivity(journeys.SendBreakGlassAlertActivity)
	w.RegisterActivity(journeys.ScheduleAccessRevocationActivity)
	w.RegisterActivity(journeys.CreateBulkOperationRecordActivity)
	w.RegisterActivity(journeys.ProcessBulkBatchActivity)
	w.RegisterActivity(journeys.UpdateBulkOperationProgressActivity)
	w.RegisterActivity(journeys.UpdateBulkOperationStatusActivity)
	
	// Sync activities
	w.RegisterActivity(journeys.VerifyDeviceRegistrationActivity)
	w.RegisterActivity(journeys.CreateSyncRecordActivity)
	w.RegisterActivity(journeys.CheckSyncConflictActivity)
	w.RegisterActivity(journeys.ApplySyncChangeActivity)
	w.RegisterActivity(journeys.GetServerVersionActivity)
	w.RegisterActivity(journeys.MergeSyncChangesActivity)
	w.RegisterActivity(journeys.GetServerChangesSinceActivity)
	w.RegisterActivity(journeys.UpdateSyncRecordActivity)
	w.RegisterActivity(journeys.UpdateDeviceLastSyncActivity)
	
	// Reporting activities
	w.RegisterActivity(journeys.CreateReportRecordActivity)
	w.RegisterActivity(journeys.QueryLakehouseEnrollmentDataActivity)
	w.RegisterActivity(journeys.QueryLakehouseDisbursementDataActivity)
	w.RegisterActivity(journeys.QueryLakehouseGrievanceDataActivity)
	w.RegisterActivity(journeys.CalculateReportKPIsActivity)
	w.RegisterActivity(journeys.GenerateReportFileActivity)
	w.RegisterActivity(journeys.UpdateReportRecordActivity)
	
	// Fraud activities
	w.RegisterActivity(journeys.CheckBeneficiaryExistsActivity)
	w.RegisterActivity(journeys.GenerateFraudCaseNumberActivity)
	w.RegisterActivity(journeys.CalculateFraudRiskScoreActivity)
	w.RegisterActivity(journeys.CreateFraudInvestigationActivity)
	w.RegisterActivity(journeys.AutoAssignFraudInvestigatorActivity)
	w.RegisterActivity(journeys.TemporarySuspendBeneficiaryActivity)
	w.RegisterActivity(journeys.GatherTransactionHistoryActivity)
	w.RegisterActivity(journeys.CrossReferenceBeneficiaryActivity)
	w.RegisterActivity(journeys.SendFraudAssignmentNotificationActivity)

	// Start worker
	log.Println("Starting Temporal worker on task queue:", TaskQueue)
	err = w.Start()
	if err != nil {
		log.Fatalf("Failed to start worker: %v", err)
	}

	// Start API server in a goroutine
	apiServer := api.NewAPIServer()
	go func() {
		apiAddr := os.Getenv("API_ADDR")
		if apiAddr == "" {
			apiAddr = ":8090"
		}
		if err := apiServer.Start(apiAddr); err != nil {
			log.Printf("API server error: %v", err)
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down worker...")
	w.Stop()
	log.Println("Worker stopped")
}

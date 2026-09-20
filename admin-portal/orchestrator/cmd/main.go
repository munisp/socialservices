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
	w.RegisterWorkflow(workflows.DisbursementProcessingWorkflow)
	w.RegisterWorkflow(workflows.FraudInvestigationWorkflow)
	w.RegisterWorkflow(workflows.ProfileUpdateWorkflow)
	w.RegisterWorkflow(workflows.ProgramEnrollmentWorkflow)
	w.RegisterWorkflow(workflows.BeneficiarySuspensionWorkflow)
	w.RegisterWorkflow(workflows.GrievanceSubmissionWorkflow)
	w.RegisterWorkflow(workflows.RetryFailedDisbursementsWorkflow)
	w.RegisterWorkflow(workflows.MonthlyReportingWorkflow)
	w.RegisterWorkflow(workflows.PaymentsReconciliationWorkflow)
	w.RegisterWorkflow(workflows.BulkOperationWorkflow)
	w.RegisterWorkflow(workflows.ApprovalDelegationWorkflow)

	enableIncompleteJourneys := os.Getenv("ENABLE_INCOMPLETE_JOURNEYS") == "true"
	if enableIncompleteJourneys && (os.Getenv("ENVIRONMENT") == "production" || os.Getenv("GO_ENV") == "production") {
		log.Fatal("ENABLE_INCOMPLETE_JOURNEYS is prohibited in production")
	}
	if enableIncompleteJourneys {
		// Register journey workflows (30 user journeys). These remain disabled by default until
		// every registered activity has a durable integration rather than a tracing-only body.
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
	} else {
		log.Println("Incomplete journey workflows are disabled; set ENABLE_INCOMPLETE_JOURNEYS=true only in non-production test environments")
	}

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

	if enableIncompleteJourneys {
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

		// Missing integrations remain unavailable and fail closed if incomplete journeys are opt-in.
		w.RegisterActivity(journeys.AdjustTigerBeetleBalanceActivity)
		w.RegisterActivity(journeys.CacheAnalyticsResultsActivity)
		w.RegisterActivity(journeys.CacheDashboardDataActivity)
		w.RegisterActivity(journeys.CalculateFinalPaymentActivity)
		w.RegisterActivity(journeys.CalculateProgramTrendsActivity)
		w.RegisterActivity(journeys.CancelAllDisbursementsActivity)
		w.RegisterActivity(journeys.CancelFutureDisbursementsActivity)
		w.RegisterActivity(journeys.CheckExistingCardActivity)
		w.RegisterActivity(journeys.CheckExistingEnrollmentActivity)
		w.RegisterActivity(journeys.CheckFederationProviderActivity)
		w.RegisterActivity(journeys.CheckProgramExistsActivity)
		w.RegisterActivity(journeys.CloseTigerBeetleAccountActivity)
		w.RegisterActivity(journeys.CommitPendingTransferActivity)
		w.RegisterActivity(journeys.CountFailedDisbursementItemsActivity)
		w.RegisterActivity(journeys.CreateAdjustmentTransactionActivity)
		w.RegisterActivity(journeys.CreateCardRecordActivity)
		w.RegisterActivity(journeys.CreateDisbursementItemActivity)
		w.RegisterActivity(journeys.CreateExportRecordActivity)
		w.RegisterActivity(journeys.CreateFraudAlertActivity)
		w.RegisterActivity(journeys.CreateFraudPredictionRecordActivity)
		w.RegisterActivity(journeys.CreateIdentityVerificationRecordActivity)
		w.RegisterActivity(journeys.CreateInteropRecordActivity)
		w.RegisterActivity(journeys.CreateProgramEnrollmentActivity)
		w.RegisterActivity(journeys.CreateReconciliationRecordActivity)
		w.RegisterActivity(journeys.CreateVerificationRecordActivity)
		w.RegisterActivity(journeys.EndProgramEnrollmentsActivity)
		w.RegisterActivity(journeys.GenerateCardDetailsActivity)
		w.RegisterActivity(journeys.GenerateCaseNumberActivity)
		w.RegisterActivity(journeys.GenerateExportFileActivity)
		w.RegisterActivity(journeys.GetActiveProgramsCountActivity)
		w.RegisterActivity(journeys.GetAllActiveBeneficiaryIDsActivity)
		w.RegisterActivity(journeys.GetBeneficiaryAccountActivity)
		w.RegisterActivity(journeys.GetBeneficiaryDataForSharingActivity)
		w.RegisterActivity(journeys.GetDisbursementDetailsActivity)
		w.RegisterActivity(journeys.GetDisbursementItemsActivity)
		w.RegisterActivity(journeys.GetDisbursementItemsByIDsActivity)
		w.RegisterActivity(journeys.GetDisputeDetailsActivity)
		w.RegisterActivity(journeys.GetEscalationAssigneeActivity)
		w.RegisterActivity(journeys.GetExternalTransactionsActivity)
		w.RegisterActivity(journeys.GetFailedDisbursementItemsActivity)
		w.RegisterActivity(journeys.GetFailedDisbursementsCountActivity)
		w.RegisterActivity(journeys.GetGrievanceDetailsActivity)
		w.RegisterActivity(journeys.GetLatestFraudModelVersionActivity)
		w.RegisterActivity(journeys.GetMonthlyTrendsActivity)
		w.RegisterActivity(journeys.GetPendingApprovalsCountActivity)
		w.RegisterActivity(journeys.GetPendingGrievancesCountActivity)
		w.RegisterActivity(journeys.GetProgramBeneficiaryIDsActivity)
		w.RegisterActivity(journeys.GetProgramBudgetUtilizationActivity)
		w.RegisterActivity(journeys.GetProgramComparisonsActivity)
		w.RegisterActivity(journeys.GetProgramDisbursementsActivity)
		w.RegisterActivity(journeys.GetProgramEnrollmentActivity)
		w.RegisterActivity(journeys.GetProgramGrievancesActivity)
		w.RegisterActivity(journeys.GetProgramStatusActivity)
		w.RegisterActivity(journeys.GetRegionBeneficiaryIDsActivity)
		w.RegisterActivity(journeys.GetSLABreachesCountActivity)
		w.RegisterActivity(journeys.GetSystemHealthActivity)
		w.RegisterActivity(journeys.GetTigerBeetleTransactionsActivity)
		w.RegisterActivity(journeys.GetTotalBeneficiariesActivity)
		w.RegisterActivity(journeys.GetTotalDisbursementsActivity)
		w.RegisterActivity(journeys.InitiateCardDeliveryActivity)
		w.RegisterActivity(journeys.LinkMemberToHouseholdActivity)
		w.RegisterActivity(journeys.MarkExternalRecordAsReconciledActivity)
		w.RegisterActivity(journeys.MatchTransactionsActivity)
		w.RegisterActivity(journeys.NotifyFraudTeamActivity)
		w.RegisterActivity(journeys.ProcessFinalPaymentActivity)
		w.RegisterActivity(journeys.ProcessGrievanceCompensationActivity)
		w.RegisterActivity(journeys.ProcessSurvivorBenefitsActivity)
		w.RegisterActivity(journeys.QueryAuditLogsForExportActivity)
		w.RegisterActivity(journeys.QueryBeneficiariesForExportActivity)
		w.RegisterActivity(journeys.QueryDisbursementsForExportActivity)
		w.RegisterActivity(journeys.QueryGrievancesForExportActivity)
		w.RegisterActivity(journeys.QueryProgramMetricsActivity)
		w.RegisterActivity(journeys.QuerySectorDataActivity)
		w.RegisterActivity(journeys.RunMLFraudPredictionActivity)
		w.RegisterActivity(journeys.SendCardIssuanceNotificationActivity)
		w.RegisterActivity(journeys.SendEscalationHandoffNotificationActivity)
		w.RegisterActivity(journeys.SendEscalationNotificationActivity)
		w.RegisterActivity(journeys.SendExitNotificationActivity)
		w.RegisterActivity(journeys.SendGrievanceAcknowledgmentActivity)
		w.RegisterActivity(journeys.SendGrievanceResolutionNotificationActivity)
		w.RegisterActivity(journeys.ShareDataWithSectorActivity)
		w.RegisterActivity(journeys.StoreConsentRecordActivity)
		w.RegisterActivity(journeys.StoreReconciliationResultsActivity)
		w.RegisterActivity(journeys.UpdateBeneficiaryDeathActivity)
		w.RegisterActivity(journeys.UpdateBeneficiaryExitActivity)
		w.RegisterActivity(journeys.UpdateBeneficiaryKYCStatusActivity)
		w.RegisterActivity(journeys.UpdateDisbursementItemStatusActivity)
		w.RegisterActivity(journeys.UpdateDisputeStatusActivity)
		w.RegisterActivity(journeys.UpdateExportRecordActivity)
		w.RegisterActivity(journeys.UpdateFraudPredictionProgressActivity)
		w.RegisterActivity(journeys.UpdateFraudPredictionRecordActivity)
		w.RegisterActivity(journeys.UpdateGrievanceEscalationActivity)
		w.RegisterActivity(journeys.UpdateHouseholdPMTActivity)
		w.RegisterActivity(journeys.UpdateIdentityVerificationResultActivity)
		w.RegisterActivity(journeys.UpdateIdentityVerificationStatusActivity)
		w.RegisterActivity(journeys.UpdateInteropRecordActivity)
		w.RegisterActivity(journeys.UpdateReconciliationStatusActivity)
		w.RegisterActivity(journeys.ValidateProgramBudgetActivity)
		w.RegisterActivity(journeys.VerifyBeneficiaryInSectorActivity)
		w.RegisterActivity(journeys.VerifyBiometricActivity)
		w.RegisterActivity(journeys.VerifyDemographicActivity)
		w.RegisterActivity(journeys.VerifyIdentityFederationActivity)
		w.RegisterActivity(journeys.VerifyOTPActivity)
	}

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

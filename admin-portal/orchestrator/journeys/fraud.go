package journeys

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/workflow"
)

// Fraud/Compliance Journey Definitions (Journeys 27-30)
// These journeys map to existing implemented components:
// - FraudInvestigationWorkflow, PredictFraudWorkflow
// - ML service for fraud prediction
// - UI: Fraud investigation page
// - BFF: workflow router

// Journey 27: Fraud Investigation
// UI Entry: Admin Fraud investigation page
// BFF: trpc.workflow.startFraudInvestigation
// Workflow: FraudInvestigationWorkflow
type FraudInvestigationInput struct {
	JourneyContext *JourneyContext `json:"journeyContext"`
	BeneficiaryID  string          `json:"beneficiaryId"`
	AlertID        string          `json:"alertId,omitempty"`
	ReportType     string          `json:"reportType"` // "system_alert", "manual_report", "external_tip"
	Description    string          `json:"description"`
	Evidence       []EvidenceItem  `json:"evidence,omitempty"`
	Priority       string          `json:"priority"` // "low", "medium", "high", "critical"
}

type EvidenceItem struct {
	Type        string `json:"type"` // "document", "transaction", "photo", "statement"
	Description string `json:"description"`
	FileURL     string `json:"fileUrl,omitempty"`
}

type FraudInvestigationResult struct {
	JourneyRunID    string    `json:"journeyRunId"`
	InvestigationID string    `json:"investigationId"`
	CaseNumber      string    `json:"caseNumber"`
	Status          string    `json:"status"`
	AssignedTo      string    `json:"assignedTo"`
	RiskScore       float64   `json:"riskScore"`
	CreatedAt       time.Time `json:"createdAt"`
}

// FraudInvestigationJourney orchestrates fraud investigation
func FraudInvestigationJourney(ctx workflow.Context, input FraudInvestigationInput) (*FraudInvestigationResult, error) {
	jc := input.JourneyContext
	
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Minute,
		RetryPolicy: &workflow.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    time.Minute,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)
	
	// Step 1: Emit journey started event (high priority)
	workflow.ExecuteActivity(ctx, EmitJourneyEventActivity, jc, "journey.started", map[string]interface{}{
		"journeyType":   "fraud_investigation",
		"beneficiaryId": input.BeneficiaryID,
		"priority":      input.Priority,
	})
	
	// Step 2: Check authorization
	var authorized bool
	err := workflow.ExecuteActivity(ctx, CheckAuthorizationActivity, jc, "fraud:investigate").Get(ctx, &authorized)
	if err != nil || !authorized {
		return nil, fmt.Errorf("authorization failed")
	}
	
	// Step 3: Verify beneficiary exists
	var beneficiaryExists bool
	err = workflow.ExecuteActivity(ctx, CheckBeneficiaryExistsActivity, input.BeneficiaryID).Get(ctx, &beneficiaryExists)
	if err != nil || !beneficiaryExists {
		return nil, fmt.Errorf("beneficiary not found")
	}
	
	// Step 4: Generate case number
	var caseNumber string
	err = workflow.ExecuteActivity(ctx, GenerateFraudCaseNumberActivity).Get(ctx, &caseNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to generate case number: %v", err)
	}
	
	// Step 5: Calculate initial risk score using ML service
	var riskScore float64
	err = workflow.ExecuteActivity(ctx, CalculateFraudRiskScoreActivity, input.BeneficiaryID, input.ReportType, input.Description).Get(ctx, &riskScore)
	if err != nil {
		workflow.GetLogger(ctx).Warn("Failed to calculate risk score", "error", err)
		riskScore = 0.5 // Default medium risk
	}
	
	// Step 6: Create investigation record
	var investigationID string
	err = workflow.ExecuteActivity(ctx, CreateFraudInvestigationActivity, map[string]interface{}{
		"beneficiaryId": input.BeneficiaryID,
		"alertId":       input.AlertID,
		"caseNumber":    caseNumber,
		"reportType":    input.ReportType,
		"description":   input.Description,
		"evidence":      input.Evidence,
		"priority":      input.Priority,
		"riskScore":     riskScore,
		"status":        "open",
		"createdBy":     jc.ActorID,
		"tenantId":      jc.TenantID,
	}).Get(ctx, &investigationID)
	if err != nil {
		return nil, fmt.Errorf("failed to create investigation: %v", err)
	}
	
	// Step 7: Auto-assign investigator based on priority and workload
	var assignedTo string
	err = workflow.ExecuteActivity(ctx, AutoAssignFraudInvestigatorActivity, investigationID, input.Priority, riskScore).Get(ctx, &assignedTo)
	if err != nil {
		workflow.GetLogger(ctx).Warn("Failed to auto-assign investigator", "error", err)
	}
	
	// Step 8: If high risk, temporarily suspend beneficiary
	if riskScore >= 0.8 || input.Priority == "critical" {
		workflow.ExecuteActivity(ctx, TemporarySuspendBeneficiaryActivity, input.BeneficiaryID, investigationID, "Pending fraud investigation")
	}
	
	// Step 9: Gather transaction history
	workflow.ExecuteActivity(ctx, GatherTransactionHistoryActivity, input.BeneficiaryID, investigationID)
	
	// Step 10: Cross-reference with other beneficiaries (duplicate detection)
	workflow.ExecuteActivity(ctx, CrossReferenceBeneficiaryActivity, input.BeneficiaryID, investigationID)
	
	// Step 11: Send notification to investigator
	if assignedTo != "" {
		workflow.ExecuteActivity(ctx, SendFraudAssignmentNotificationActivity, assignedTo, investigationID, caseNumber, input.Priority)
	}
	
	// Step 12: Publish Kafka event
	workflow.ExecuteActivity(ctx, PublishKafkaEventActivity, "fraud.investigation_opened", map[string]interface{}{
		"investigationId": investigationID,
		"caseNumber":      caseNumber,
		"beneficiaryId":   input.BeneficiaryID,
		"priority":        input.Priority,
		"riskScore":       riskScore,
		"correlationId":   jc.CorrelationID,
	})
	
	// Step 13: Create audit log
	workflow.ExecuteActivity(ctx, CreateAuditLogActivity, map[string]interface{}{
		"action":        "fraud_investigation_opened",
		"entityType":    "fraud_investigation",
		"entityId":      investigationID,
		"performedBy":   jc.ActorID,
		"tenantId":      jc.TenantID,
		"correlationId": jc.CorrelationID,
		"severity":      "high",
		"details": map[string]interface{}{
			"caseNumber":    caseNumber,
			"beneficiaryId": input.BeneficiaryID,
			"reportType":    input.ReportType,
			"priority":      input.Priority,
			"riskScore":     riskScore,
		},
	})
	
	// Step 14: Write to lakehouse
	workflow.ExecuteActivity(ctx, WriteLakehouseFactActivity, "fraud_investigation_facts", map[string]interface{}{
		"investigationId": investigationID,
		"beneficiaryId":   input.BeneficiaryID,
		"openDate":        time.Now().Format("2006-01-02"),
		"reportType":      input.ReportType,
		"priority":        input.Priority,
		"riskScore":       riskScore,
		"tenantId":        jc.TenantID,
	})
	
	// Step 15: Emit journey completed event
	workflow.ExecuteActivity(ctx, EmitJourneyEventActivity, jc, "journey.completed", map[string]interface{}{
		"investigationId": investigationID,
		"caseNumber":      caseNumber,
	})
	
	return &FraudInvestigationResult{
		JourneyRunID:    jc.JourneyRunID,
		InvestigationID: investigationID,
		CaseNumber:      caseNumber,
		Status:          "open",
		AssignedTo:      assignedTo,
		RiskScore:       riskScore,
		CreatedAt:       time.Now(),
	}, nil
}

// Journey 28: ML Fraud Prediction
// UI Entry: Admin Fraud alerts page (automatic)
// BFF: trpc.workflow.runFraudPrediction
// Workflow: PredictFraudWorkflow
type FraudPredictionInput struct {
	JourneyContext *JourneyContext `json:"journeyContext"`
	Scope          string          `json:"scope"` // "all", "program", "region"
	ScopeID        string          `json:"scopeId,omitempty"`
	ModelVersion   string          `json:"modelVersion,omitempty"`
	Threshold      float64         `json:"threshold"` // Risk score threshold for alerts
}

type FraudPredictionResult struct {
	JourneyRunID    string    `json:"journeyRunId"`
	PredictionID    string    `json:"predictionId"`
	TotalScanned    int       `json:"totalScanned"`
	AlertsGenerated int       `json:"alertsGenerated"`
	HighRiskCount   int       `json:"highRiskCount"`
	ModelVersion    string    `json:"modelVersion"`
	CompletedAt     time.Time `json:"completedAt"`
}

// FraudPredictionJourney orchestrates ML-based fraud prediction
func FraudPredictionJourney(ctx workflow.Context, input FraudPredictionInput) (*FraudPredictionResult, error) {
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
		"journeyType": "fraud_prediction",
		"scope":       input.Scope,
	})
	
	// Step 2: Check authorization
	var authorized bool
	err := workflow.ExecuteActivity(ctx, CheckAuthorizationActivity, jc, "fraud:predict").Get(ctx, &authorized)
	if err != nil || !authorized {
		return nil, fmt.Errorf("authorization failed")
	}
	
	// Step 3: Create prediction record
	var predictionID string
	err = workflow.ExecuteActivity(ctx, CreateFraudPredictionRecordActivity, map[string]interface{}{
		"scope":        input.Scope,
		"scopeId":      input.ScopeID,
		"modelVersion": input.ModelVersion,
		"threshold":    input.Threshold,
		"status":       "processing",
		"createdBy":    jc.ActorID,
		"tenantId":     jc.TenantID,
	}).Get(ctx, &predictionID)
	if err != nil {
		return nil, fmt.Errorf("failed to create prediction record: %v", err)
	}
	
	// Step 4: Get beneficiaries to scan based on scope
	var beneficiaryIDs []string
	switch input.Scope {
	case "all":
		err = workflow.ExecuteActivity(ctx, GetAllActiveBeneficiaryIDsActivity, jc.TenantID).Get(ctx, &beneficiaryIDs)
	case "program":
		err = workflow.ExecuteActivity(ctx, GetProgramBeneficiaryIDsActivity, input.ScopeID).Get(ctx, &beneficiaryIDs)
	case "region":
		err = workflow.ExecuteActivity(ctx, GetRegionBeneficiaryIDsActivity, input.ScopeID).Get(ctx, &beneficiaryIDs)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get beneficiaries: %v", err)
	}
	
	// Step 5: Get ML model version
	modelVersion := input.ModelVersion
	if modelVersion == "" {
		workflow.ExecuteActivity(ctx, GetLatestFraudModelVersionActivity).Get(ctx, &modelVersion)
	}
	
	// Step 6: Run predictions in batches
	batchSize := 100
	alertsGenerated := 0
	highRiskCount := 0
	
	for i := 0; i < len(beneficiaryIDs); i += batchSize {
		end := i + batchSize
		if end > len(beneficiaryIDs) {
			end = len(beneficiaryIDs)
		}
		batch := beneficiaryIDs[i:end]
		
		// Call ML service for batch prediction
		var predictions []map[string]interface{}
		err = workflow.ExecuteActivity(ctx, RunMLFraudPredictionActivity, batch, modelVersion).Get(ctx, &predictions)
		if err != nil {
			workflow.GetLogger(ctx).Warn("Batch prediction failed", "batch", i, "error", err)
			continue
		}
		
		// Process predictions
		for _, pred := range predictions {
			riskScore := pred["riskScore"].(float64)
			beneficiaryID := pred["beneficiaryId"].(string)
			
			if riskScore >= input.Threshold {
				// Generate alert
				workflow.ExecuteActivity(ctx, CreateFraudAlertActivity, map[string]interface{}{
					"predictionId":  predictionID,
					"beneficiaryId": beneficiaryID,
					"riskScore":     riskScore,
					"modelVersion":  modelVersion,
					"factors":       pred["factors"],
				})
				alertsGenerated++
				
				if riskScore >= 0.8 {
					highRiskCount++
				}
			}
		}
		
		// Update progress
		progress := float64(end) / float64(len(beneficiaryIDs)) * 100
		workflow.ExecuteActivity(ctx, UpdateFraudPredictionProgressActivity, predictionID, progress)
	}
	
	// Step 7: Update prediction record
	workflow.ExecuteActivity(ctx, UpdateFraudPredictionRecordActivity, predictionID, map[string]interface{}{
		"status":          "completed",
		"totalScanned":    len(beneficiaryIDs),
		"alertsGenerated": alertsGenerated,
		"highRiskCount":   highRiskCount,
		"modelVersion":    modelVersion,
	})
	
	// Step 8: Notify fraud team if high-risk alerts generated
	if highRiskCount > 0 {
		workflow.ExecuteActivity(ctx, NotifyFraudTeamActivity, predictionID, highRiskCount)
	}
	
	// Step 9: Publish Kafka event
	workflow.ExecuteActivity(ctx, PublishKafkaEventActivity, "fraud.prediction_completed", map[string]interface{}{
		"predictionId":    predictionID,
		"totalScanned":    len(beneficiaryIDs),
		"alertsGenerated": alertsGenerated,
		"highRiskCount":   highRiskCount,
		"correlationId":   jc.CorrelationID,
	})
	
	// Step 10: Create audit log
	workflow.ExecuteActivity(ctx, CreateAuditLogActivity, map[string]interface{}{
		"action":        "fraud_prediction_completed",
		"entityType":    "fraud_prediction",
		"entityId":      predictionID,
		"performedBy":   jc.ActorID,
		"tenantId":      jc.TenantID,
		"correlationId": jc.CorrelationID,
		"details": map[string]interface{}{
			"scope":           input.Scope,
			"totalScanned":    len(beneficiaryIDs),
			"alertsGenerated": alertsGenerated,
			"highRiskCount":   highRiskCount,
			"modelVersion":    modelVersion,
		},
	})
	
	// Step 11: Write to lakehouse
	workflow.ExecuteActivity(ctx, WriteLakehouseFactActivity, "fraud_prediction_facts", map[string]interface{}{
		"predictionId":    predictionID,
		"predictionDate":  time.Now().Format("2006-01-02"),
		"scope":           input.Scope,
		"totalScanned":    len(beneficiaryIDs),
		"alertsGenerated": alertsGenerated,
		"highRiskCount":   highRiskCount,
		"modelVersion":    modelVersion,
		"tenantId":        jc.TenantID,
	})
	
	// Step 12: Emit journey completed event
	workflow.ExecuteActivity(ctx, EmitJourneyEventActivity, jc, "journey.completed", map[string]interface{}{
		"predictionId":    predictionID,
		"alertsGenerated": alertsGenerated,
	})
	
	return &FraudPredictionResult{
		JourneyRunID:    jc.JourneyRunID,
		PredictionID:    predictionID,
		TotalScanned:    len(beneficiaryIDs),
		AlertsGenerated: alertsGenerated,
		HighRiskCount:   highRiskCount,
		ModelVersion:    modelVersion,
		CompletedAt:     time.Now(),
	}, nil
}

// Journey 29: National ID Federation Verification
// UI Entry: Mobile VerifyIDScreen, Admin Beneficiary detail
// BFF: trpc.worldClass.federation.verify
// Workflow: NationalIDVerificationWorkflow
type NationalIDVerificationInput struct {
	JourneyContext *JourneyContext `json:"journeyContext"`
	BeneficiaryID  string          `json:"beneficiaryId"`
	ProviderID     string          `json:"providerId"` // "nin", "nida", "aadhaar", etc.
	NationalID     string          `json:"nationalId"`
	VerificationType string        `json:"verificationType"` // "demographic", "biometric", "otp"
	BiometricData  *BiometricInput `json:"biometricData,omitempty"`
	OTP            string          `json:"otp,omitempty"`
}

type NationalIDVerificationResult struct {
	JourneyRunID   string                 `json:"journeyRunId"`
	VerificationID string                 `json:"verificationId"`
	Verified       bool                   `json:"verified"`
	MatchScore     float64                `json:"matchScore"`
	Demographics   map[string]interface{} `json:"demographics,omitempty"`
	VerifiedAt     time.Time              `json:"verifiedAt"`
}

// NationalIDVerificationJourney orchestrates national ID verification via federation
func NationalIDVerificationJourney(ctx workflow.Context, input NationalIDVerificationInput) (*NationalIDVerificationResult, error) {
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
		"journeyType":      "national_id_verification",
		"beneficiaryId":    input.BeneficiaryID,
		"providerId":       input.ProviderID,
		"verificationType": input.VerificationType,
	})
	
	// Step 2: Check authorization
	var authorized bool
	err := workflow.ExecuteActivity(ctx, CheckAuthorizationActivity, jc, "identity:verify").Get(ctx, &authorized)
	if err != nil || !authorized {
		return nil, fmt.Errorf("authorization failed")
	}
	
	// Step 3: Validate provider is supported
	var providerSupported bool
	err = workflow.ExecuteActivity(ctx, CheckFederationProviderActivity, input.ProviderID).Get(ctx, &providerSupported)
	if err != nil || !providerSupported {
		return nil, fmt.Errorf("identity provider not supported: %s", input.ProviderID)
	}
	
	// Step 4: Create verification record
	var verificationID string
	err = workflow.ExecuteActivity(ctx, CreateIdentityVerificationRecordActivity, map[string]interface{}{
		"beneficiaryId":    input.BeneficiaryID,
		"providerId":       input.ProviderID,
		"nationalId":       input.NationalID,
		"verificationType": input.VerificationType,
		"status":           "pending",
		"createdBy":        jc.ActorID,
		"tenantId":         jc.TenantID,
	}).Get(ctx, &verificationID)
	if err != nil {
		return nil, fmt.Errorf("failed to create verification record: %v", err)
	}
	
	// Step 5: Call federation service based on verification type
	var verificationResult map[string]interface{}
	switch input.VerificationType {
	case "demographic":
		err = workflow.ExecuteActivity(ctx, VerifyDemographicActivity, input.ProviderID, input.NationalID).Get(ctx, &verificationResult)
	case "biometric":
		if input.BiometricData == nil {
			return nil, fmt.Errorf("biometric data required for biometric verification")
		}
		err = workflow.ExecuteActivity(ctx, VerifyBiometricActivity, input.ProviderID, input.NationalID, input.BiometricData).Get(ctx, &verificationResult)
	case "otp":
		if input.OTP == "" {
			return nil, fmt.Errorf("OTP required for OTP verification")
		}
		err = workflow.ExecuteActivity(ctx, VerifyOTPActivity, input.ProviderID, input.NationalID, input.OTP).Get(ctx, &verificationResult)
	default:
		return nil, fmt.Errorf("unsupported verification type: %s", input.VerificationType)
	}
	
	if err != nil {
		workflow.ExecuteActivity(ctx, UpdateIdentityVerificationStatusActivity, verificationID, "failed", err.Error())
		return nil, fmt.Errorf("verification failed: %v", err)
	}
	
	verified := verificationResult["verified"].(bool)
	matchScore := verificationResult["matchScore"].(float64)
	demographics, _ := verificationResult["demographics"].(map[string]interface{})
	
	// Step 6: Update verification record
	workflow.ExecuteActivity(ctx, UpdateIdentityVerificationResultActivity, verificationID, map[string]interface{}{
		"status":       "completed",
		"verified":     verified,
		"matchScore":   matchScore,
		"demographics": demographics,
	})
	
	// Step 7: Update beneficiary KYC status if verified
	if verified {
		workflow.ExecuteActivity(ctx, UpdateBeneficiaryKYCStatusActivity, input.BeneficiaryID, map[string]interface{}{
			"kycStatus":   true,
			"kycProvider": input.ProviderID,
			"matchScore":  matchScore,
			"verifiedAt":  time.Now(),
		})
	}
	
	// Step 8: Publish Kafka event
	workflow.ExecuteActivity(ctx, PublishKafkaEventActivity, "identity.verified", map[string]interface{}{
		"verificationId": verificationID,
		"beneficiaryId":  input.BeneficiaryID,
		"providerId":     input.ProviderID,
		"verified":       verified,
		"matchScore":     matchScore,
		"correlationId":  jc.CorrelationID,
	})
	
	// Step 9: Create audit log
	workflow.ExecuteActivity(ctx, CreateAuditLogActivity, map[string]interface{}{
		"action":        "identity_verified",
		"entityType":    "identity_verification",
		"entityId":      verificationID,
		"performedBy":   jc.ActorID,
		"tenantId":      jc.TenantID,
		"correlationId": jc.CorrelationID,
		"details": map[string]interface{}{
			"beneficiaryId":    input.BeneficiaryID,
			"providerId":       input.ProviderID,
			"verificationType": input.VerificationType,
			"verified":         verified,
			"matchScore":       matchScore,
		},
	})
	
	// Step 10: Write to lakehouse
	workflow.ExecuteActivity(ctx, WriteLakehouseFactActivity, "identity_verification_facts", map[string]interface{}{
		"verificationId":   verificationID,
		"beneficiaryId":    input.BeneficiaryID,
		"verificationDate": time.Now().Format("2006-01-02"),
		"providerId":       input.ProviderID,
		"verificationType": input.VerificationType,
		"verified":         verified,
		"matchScore":       matchScore,
		"tenantId":         jc.TenantID,
	})
	
	// Step 11: Emit journey completed event
	workflow.ExecuteActivity(ctx, EmitJourneyEventActivity, jc, "journey.completed", map[string]interface{}{
		"verificationId": verificationID,
		"verified":       verified,
	})
	
	return &NationalIDVerificationResult{
		JourneyRunID:   jc.JourneyRunID,
		VerificationID: verificationID,
		Verified:       verified,
		MatchScore:     matchScore,
		Demographics:   demographics,
		VerifiedAt:     time.Now(),
	}, nil
}

// Journey 30: Cross-Sector Interoperability
// UI Entry: Admin Interoperability page
// BFF: trpc.worldClass.interop
// Workflow: CrossSectorInteropWorkflow
type CrossSectorInteropInput struct {
	JourneyContext *JourneyContext `json:"journeyContext"`
	BeneficiaryID  string          `json:"beneficiaryId"`
	TargetSector   string          `json:"targetSector"` // "health", "education", "tax", "agriculture"
	Operation      string          `json:"operation"`    // "query", "share", "verify"
	DataFields     []string        `json:"dataFields,omitempty"`
	Consent        bool            `json:"consent"`
}

type CrossSectorInteropResult struct {
	JourneyRunID  string                 `json:"journeyRunId"`
	InteropID     string                 `json:"interopId"`
	TargetSector  string                 `json:"targetSector"`
	Operation     string                 `json:"operation"`
	Success       bool                   `json:"success"`
	Data          map[string]interface{} `json:"data,omitempty"`
	CompletedAt   time.Time              `json:"completedAt"`
}

// CrossSectorInteropJourney orchestrates cross-sector data interoperability
func CrossSectorInteropJourney(ctx workflow.Context, input CrossSectorInteropInput) (*CrossSectorInteropResult, error) {
	jc := input.JourneyContext
	
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Minute,
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
		"journeyType":   "cross_sector_interop",
		"beneficiaryId": input.BeneficiaryID,
		"targetSector":  input.TargetSector,
		"operation":     input.Operation,
	})
	
	// Step 2: Check authorization
	permission := fmt.Sprintf("interop:%s:%s", input.TargetSector, input.Operation)
	var authorized bool
	err := workflow.ExecuteActivity(ctx, CheckAuthorizationActivity, jc, permission).Get(ctx, &authorized)
	if err != nil || !authorized {
		return nil, fmt.Errorf("authorization failed for %s", permission)
	}
	
	// Step 3: Verify consent
	if !input.Consent {
		return nil, fmt.Errorf("consent required for cross-sector data sharing")
	}
	
	// Step 4: Verify beneficiary exists
	var beneficiaryExists bool
	err = workflow.ExecuteActivity(ctx, CheckBeneficiaryExistsActivity, input.BeneficiaryID).Get(ctx, &beneficiaryExists)
	if err != nil || !beneficiaryExists {
		return nil, fmt.Errorf("beneficiary not found")
	}
	
	// Step 5: Create interop record
	var interopID string
	err = workflow.ExecuteActivity(ctx, CreateInteropRecordActivity, map[string]interface{}{
		"beneficiaryId": input.BeneficiaryID,
		"targetSector":  input.TargetSector,
		"operation":     input.Operation,
		"dataFields":    input.DataFields,
		"consent":       input.Consent,
		"status":        "processing",
		"createdBy":     jc.ActorID,
		"tenantId":      jc.TenantID,
	}).Get(ctx, &interopID)
	if err != nil {
		return nil, fmt.Errorf("failed to create interop record: %v", err)
	}
	
	// Step 6: Execute interop operation based on sector
	var result map[string]interface{}
	var success bool
	
	switch input.Operation {
	case "query":
		// Query data from target sector
		err = workflow.ExecuteActivity(ctx, QuerySectorDataActivity, input.TargetSector, input.BeneficiaryID, input.DataFields).Get(ctx, &result)
		success = err == nil
	case "share":
		// Share data with target sector
		var beneficiaryData map[string]interface{}
		workflow.ExecuteActivity(ctx, GetBeneficiaryDataForSharingActivity, input.BeneficiaryID, input.DataFields).Get(ctx, &beneficiaryData)
		err = workflow.ExecuteActivity(ctx, ShareDataWithSectorActivity, input.TargetSector, input.BeneficiaryID, beneficiaryData).Get(ctx, &result)
		success = err == nil
	case "verify":
		// Verify beneficiary status in target sector
		err = workflow.ExecuteActivity(ctx, VerifyBeneficiaryInSectorActivity, input.TargetSector, input.BeneficiaryID).Get(ctx, &result)
		success = err == nil
	default:
		return nil, fmt.Errorf("unsupported operation: %s", input.Operation)
	}
	
	// Step 7: Update interop record
	status := "completed"
	if !success {
		status = "failed"
	}
	workflow.ExecuteActivity(ctx, UpdateInteropRecordActivity, interopID, map[string]interface{}{
		"status":  status,
		"success": success,
		"result":  result,
	})
	
	// Step 8: Store consent record
	workflow.ExecuteActivity(ctx, StoreConsentRecordActivity, map[string]interface{}{
		"interopId":     interopID,
		"beneficiaryId": input.BeneficiaryID,
		"targetSector":  input.TargetSector,
		"operation":     input.Operation,
		"dataFields":    input.DataFields,
		"consentGiven":  input.Consent,
		"consentDate":   time.Now(),
	})
	
	// Step 9: Publish Kafka event
	workflow.ExecuteActivity(ctx, PublishKafkaEventActivity, "interop.completed", map[string]interface{}{
		"interopId":     interopID,
		"beneficiaryId": input.BeneficiaryID,
		"targetSector":  input.TargetSector,
		"operation":     input.Operation,
		"success":       success,
		"correlationId": jc.CorrelationID,
	})
	
	// Step 10: Create audit log
	workflow.ExecuteActivity(ctx, CreateAuditLogActivity, map[string]interface{}{
		"action":        "cross_sector_interop",
		"entityType":    "interop",
		"entityId":      interopID,
		"performedBy":   jc.ActorID,
		"tenantId":      jc.TenantID,
		"correlationId": jc.CorrelationID,
		"details": map[string]interface{}{
			"beneficiaryId": input.BeneficiaryID,
			"targetSector":  input.TargetSector,
			"operation":     input.Operation,
			"success":       success,
			"dataFields":    input.DataFields,
		},
	})
	
	// Step 11: Write to lakehouse
	workflow.ExecuteActivity(ctx, WriteLakehouseFactActivity, "interop_facts", map[string]interface{}{
		"interopId":     interopID,
		"beneficiaryId": input.BeneficiaryID,
		"interopDate":   time.Now().Format("2006-01-02"),
		"targetSector":  input.TargetSector,
		"operation":     input.Operation,
		"success":       success,
		"tenantId":      jc.TenantID,
	})
	
	// Step 12: Emit journey completed event
	workflow.ExecuteActivity(ctx, EmitJourneyEventActivity, jc, "journey.completed", map[string]interface{}{
		"interopId": interopID,
		"success":   success,
	})
	
	return &CrossSectorInteropResult{
		JourneyRunID: jc.JourneyRunID,
		InteropID:    interopID,
		TargetSector: input.TargetSector,
		Operation:    input.Operation,
		Success:      success,
		Data:         result,
		CompletedAt:  time.Now(),
	}, nil
}

// RegisterFraudJourneys registers all fraud/compliance journey definitions
func RegisterFraudJourneys(registry *JourneyRegistry) {
	registry.Register(&JourneyDefinition{
		Key:          "fraud_investigation",
		Name:         "Fraud Investigation",
		Description:  "Open and manage fraud investigation case",
		Category:     "fraud",
		WorkflowType: "FraudInvestigationJourney",
		RequiredPermissions: []string{"fraud:investigate"},
		UIEntryPoints:       []string{"Admin:FraudInvestigationPage"},
		BFFEndpoints:        []string{"trpc.workflow.startFraudInvestigation"},
		MiddlewareHooks:     []string{"kafka", "redis", "permify", "ml-service", "lakehouse"},
	})
	
	registry.Register(&JourneyDefinition{
		Key:          "fraud_prediction",
		Name:         "ML Fraud Prediction",
		Description:  "Run ML-based fraud prediction on beneficiaries",
		Category:     "fraud",
		WorkflowType: "FraudPredictionJourney",
		RequiredPermissions: []string{"fraud:predict"},
		UIEntryPoints:       []string{"Admin:FraudAlertsPage"},
		BFFEndpoints:        []string{"trpc.workflow.runFraudPrediction"},
		MiddlewareHooks:     []string{"kafka", "redis", "permify", "ml-service", "lakehouse"},
	})
	
	registry.Register(&JourneyDefinition{
		Key:          "national_id_verification",
		Name:         "National ID Verification",
		Description:  "Verify identity via national ID federation",
		Category:     "fraud",
		WorkflowType: "NationalIDVerificationJourney",
		RequiredPermissions: []string{"identity:verify"},
		UIEntryPoints:       []string{"Mobile:VerifyIDScreen", "Admin:BeneficiaryDetailPage"},
		BFFEndpoints:        []string{"trpc.worldClass.federation.verify"},
		MiddlewareHooks:     []string{"kafka", "redis", "permify", "federation-service", "lakehouse"},
	})
	
	registry.Register(&JourneyDefinition{
		Key:          "cross_sector_interop",
		Name:         "Cross-Sector Interoperability",
		Description:  "Query, share, or verify data with other sectors",
		Category:     "fraud",
		WorkflowType: "CrossSectorInteropJourney",
		RequiredPermissions: []string{"interop:execute"},
		UIEntryPoints:       []string{"Admin:InteroperabilityPage"},
		BFFEndpoints:        []string{"trpc.worldClass.interop.execute"},
		MiddlewareHooks:     []string{"kafka", "redis", "permify", "interop-service", "lakehouse"},
	})
}

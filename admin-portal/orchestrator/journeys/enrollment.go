package journeys

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// Enrollment Journey Definitions (Journeys 1-7)
// These journeys map to existing implemented components:
// - EnrollBeneficiaryWorkflow, KYCVerificationWorkflow, HouseholdRegistrationWorkflow
// - ProgramEnrollmentWorkflow, CardIssuanceWorkflow
// - UI: EnrollBeneficiaryScreen, VerifyIDScreen, HouseholdsScreen
// - BFF: beneficiaries router, worldClass.federation router

// Journey 1: Beneficiary Enrollment
// UI Entry: Mobile EnrollBeneficiaryScreen, Admin Beneficiaries page
// BFF: trpc.beneficiaries.create
// Workflow: EnrollBeneficiaryWorkflow
type BeneficiaryEnrollmentInput struct {
	JourneyContext *JourneyContext `json:"journeyContext"`

	// Personal information
	FirstName      string `json:"firstName"`
	LastName       string `json:"lastName"`
	DateOfBirth    string `json:"dateOfBirth"`
	Gender         string `json:"gender"`
	NationalID     string `json:"nationalId"`
	NationalIDType string `json:"nationalIdType"`

	// Contact information
	Phone   string `json:"phone"`
	Email   string `json:"email,omitempty"`
	Address string `json:"address"`

	// Location
	Region   string `json:"region"`
	District string `json:"district"`
	Ward     string `json:"ward,omitempty"`
	Village  string `json:"village,omitempty"`

	// Household
	HouseholdID       string `json:"householdId,omitempty"`
	IsHeadOfHousehold bool   `json:"isHeadOfHousehold"`

	// Documents
	Documents []DocumentInput `json:"documents,omitempty"`

	// Biometrics
	BiometricData *BiometricInput `json:"biometricData,omitempty"`
}

type DocumentInput struct {
	Type     string `json:"type"`
	Number   string `json:"number"`
	FilePath string `json:"filePath,omitempty"`
}

type BiometricInput struct {
	Type       string    `json:"type"` // "fingerprint", "face", "iris"
	Template   string    `json:"template"`
	Quality    float64   `json:"quality"`
	CapturedAt time.Time `json:"capturedAt"`
}

type BeneficiaryEnrollmentResult struct {
	JourneyRunID  string    `json:"journeyRunId"`
	BeneficiaryID string    `json:"beneficiaryId"`
	Status        string    `json:"status"`
	KYCStatus     string    `json:"kycStatus"`
	AccountID     string    `json:"accountId,omitempty"`
	CardRequested bool      `json:"cardRequested"`
	EnrolledAt    time.Time `json:"enrolledAt"`
	NextSteps     []string  `json:"nextSteps,omitempty"`
}

// BeneficiaryEnrollmentJourney orchestrates the complete beneficiary enrollment process
// This journey composes: EnrollBeneficiaryWorkflow + KYCVerificationWorkflow + CreateTigerBeetleAccount
func BeneficiaryEnrollmentJourney(ctx workflow.Context, input BeneficiaryEnrollmentInput) (*BeneficiaryEnrollmentResult, error) {
	jc := input.JourneyContext

	// Activity options with retry
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    time.Minute,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Step 1: Emit journey started event
	var eventErr error
	workflow.ExecuteActivity(ctx, EmitJourneyEventActivity, jc, "journey.started", map[string]interface{}{
		"journeyType": "beneficiary_enrollment",
		"nationalId":  input.NationalID,
	}).Get(ctx, &eventErr)

	// Step 2: Check authorization via Permify
	var authorized bool
	err := workflow.ExecuteActivity(ctx, CheckAuthorizationActivity, jc, "beneficiary:create").Get(ctx, &authorized)
	if err != nil || !authorized {
		return nil, fmt.Errorf("authorization failed: %v", err)
	}

	// Step 3: Check idempotency via Redis
	var isDuplicate bool
	err = workflow.ExecuteActivity(ctx, CheckIdempotencyActivity, jc.IdempotencyKey, "enrollment").Get(ctx, &isDuplicate)
	if err != nil {
		return nil, fmt.Errorf("idempotency check failed: %v", err)
	}
	if isDuplicate {
		return nil, fmt.Errorf("duplicate enrollment request")
	}

	// Step 4: Validate national ID via federation service
	var idValid bool
	err = workflow.ExecuteActivity(ctx, ValidateNationalIDActivity, input.NationalIDType, input.NationalID).Get(ctx, &idValid)
	if err != nil {
		return nil, fmt.Errorf("national ID validation failed: %v", err)
	}

	// Step 5: Check for duplicate beneficiary
	var existingID string
	err = workflow.ExecuteActivity(ctx, CheckDuplicateBeneficiaryActivity, input.NationalID, input.Phone).Get(ctx, &existingID)
	if err != nil {
		return nil, fmt.Errorf("duplicate check failed: %v", err)
	}
	if existingID != "" {
		return nil, fmt.Errorf("beneficiary already exists with ID: %s", existingID)
	}

	// Step 6: Validate documents via OCR service
	if len(input.Documents) > 0 {
		var docsValid bool
		err = workflow.ExecuteActivity(ctx, ValidateDocumentsActivity, input.Documents).Get(ctx, &docsValid)
		if err != nil {
			return nil, fmt.Errorf("document validation failed: %v", err)
		}
	}

	// Step 7: Verify biometrics via biometric service
	if input.BiometricData != nil {
		var bioValid bool
		err = workflow.ExecuteActivity(ctx, VerifyBiometricsActivity, input.BiometricData).Get(ctx, &bioValid)
		if err != nil {
			return nil, fmt.Errorf("biometric verification failed: %v", err)
		}
	}

	// Step 8: Create beneficiary record in database
	var beneficiaryID string
	err = workflow.ExecuteActivity(ctx, CreateBeneficiaryRecordActivity, map[string]interface{}{
		"firstName":         input.FirstName,
		"lastName":          input.LastName,
		"dateOfBirth":       input.DateOfBirth,
		"gender":            input.Gender,
		"nationalId":        input.NationalID,
		"nationalIdType":    input.NationalIDType,
		"phone":             input.Phone,
		"email":             input.Email,
		"address":           input.Address,
		"region":            input.Region,
		"district":          input.District,
		"ward":              input.Ward,
		"village":           input.Village,
		"householdId":       input.HouseholdID,
		"isHeadOfHousehold": input.IsHeadOfHousehold,
		"status":            "pending_verification",
		"createdBy":         jc.ActorID,
		"tenantId":          jc.TenantID,
	}).Get(ctx, &beneficiaryID)
	if err != nil {
		return nil, fmt.Errorf("failed to create beneficiary record: %v", err)
	}

	// Step 9: Create TigerBeetle account for payments
	var accountID string
	err = workflow.ExecuteActivity(ctx, CreateTigerBeetleAccountActivity, beneficiaryID, jc.TenantID).Get(ctx, &accountID)
	if err != nil {
		// Non-fatal: account can be created later
		workflow.GetLogger(ctx).Warn("Failed to create TigerBeetle account", "error", err)
	}

	// Step 10: Publish Kafka event for downstream systems
	workflow.ExecuteActivity(ctx, PublishKafkaEventActivity, "beneficiary.enrolled", map[string]interface{}{
		"beneficiaryId": beneficiaryID,
		"nationalId":    input.NationalID,
		"tenantId":      jc.TenantID,
		"correlationId": jc.CorrelationID,
		"enrolledBy":    jc.ActorID,
	})

	// Step 11: Cache beneficiary data in Redis
	workflow.ExecuteActivity(ctx, CacheBeneficiaryDataActivity, beneficiaryID, map[string]interface{}{
		"firstName": input.FirstName,
		"lastName":  input.LastName,
		"status":    "pending_verification",
	})

	// Step 12: Send SMS notification
	workflow.ExecuteActivity(ctx, SendSMSNotificationActivity, input.Phone, fmt.Sprintf(
		"Welcome %s! Your enrollment is being processed. Reference: %s",
		input.FirstName, beneficiaryID,
	))

	// Step 13: Create audit log
	workflow.ExecuteActivity(ctx, CreateAuditLogActivity, map[string]interface{}{
		"action":        "beneficiary_enrolled",
		"entityType":    "beneficiary",
		"entityId":      beneficiaryID,
		"performedBy":   jc.ActorID,
		"tenantId":      jc.TenantID,
		"correlationId": jc.CorrelationID,
		"details": map[string]interface{}{
			"nationalId": input.NationalID,
			"source":     jc.Source,
		},
	})

	// Step 14: Write to lakehouse for analytics
	workflow.ExecuteActivity(ctx, WriteLakehouseFactActivity, "enrollment_facts", map[string]interface{}{
		"beneficiaryId":  beneficiaryID,
		"enrollmentDate": time.Now().Format("2006-01-02"),
		"region":         input.Region,
		"district":       input.District,
		"source":         jc.Source,
		"tenantId":       jc.TenantID,
	})

	// Step 15: Emit journey completed event
	workflow.ExecuteActivity(ctx, EmitJourneyEventActivity, jc, "journey.completed", map[string]interface{}{
		"beneficiaryId": beneficiaryID,
		"status":        "success",
	})

	return &BeneficiaryEnrollmentResult{
		JourneyRunID:  jc.JourneyRunID,
		BeneficiaryID: beneficiaryID,
		Status:        "enrolled",
		KYCStatus:     "pending",
		AccountID:     accountID,
		CardRequested: false,
		EnrolledAt:    time.Now(),
		NextSteps: []string{
			"Complete KYC verification",
			"Enroll in benefit program",
			"Request payment card",
		},
	}, nil
}

// Journey 2: KYC Verification
// UI Entry: Mobile VerifyIDScreen, Admin Beneficiary detail page
// BFF: trpc.worldClass.federation.verify
// Workflow: KYCVerificationWorkflow
type KYCVerificationInput struct {
	JourneyContext *JourneyContext `json:"journeyContext"`
	BeneficiaryID  string          `json:"beneficiaryId"`
	ProviderID     string          `json:"providerId"` // "aadhaar", "nin", "nida", etc.
	NationalID     string          `json:"nationalId"`
	OTP            string          `json:"otp,omitempty"`
	BiometricData  *BiometricInput `json:"biometricData,omitempty"`
	Consent        ConsentInput    `json:"consent"`
}

type ConsentInput struct {
	DataSharing      bool `json:"dataSharing"`
	BiometricCapture bool `json:"biometricCapture"`
	TermsAccepted    bool `json:"termsAccepted"`
}

type KYCVerificationResult struct {
	JourneyRunID   string                 `json:"journeyRunId"`
	BeneficiaryID  string                 `json:"beneficiaryId"`
	Verified       bool                   `json:"verified"`
	MatchScore     float64                `json:"matchScore"`
	Demographics   map[string]interface{} `json:"demographics,omitempty"`
	VerifiedAt     time.Time              `json:"verifiedAt"`
	VerificationID string                 `json:"verificationId"`
}

// KYCVerificationJourney orchestrates the KYC verification process
func KYCVerificationJourney(ctx workflow.Context, input KYCVerificationInput) (*KYCVerificationResult, error) {
	jc := input.JourneyContext

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 3 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    30 * time.Second,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Step 1: Emit journey started event
	workflow.ExecuteActivity(ctx, EmitJourneyEventActivity, jc, "journey.started", map[string]interface{}{
		"journeyType":   "kyc_verification",
		"beneficiaryId": input.BeneficiaryID,
		"providerId":    input.ProviderID,
	})

	// Step 2: Check authorization
	var authorized bool
	err := workflow.ExecuteActivity(ctx, CheckAuthorizationActivity, jc, "beneficiary:verify").Get(ctx, &authorized)
	if err != nil || !authorized {
		return nil, fmt.Errorf("authorization failed")
	}

	// Step 3: Validate consent
	if !input.Consent.DataSharing || !input.Consent.TermsAccepted {
		return nil, fmt.Errorf("consent not provided")
	}

	// Step 4: Call federation service to verify identity
	var verificationResult map[string]interface{}
	err = workflow.ExecuteActivity(ctx, VerifyIdentityFederationActivity, map[string]interface{}{
		"providerId":    input.ProviderID,
		"nationalId":    input.NationalID,
		"otp":           input.OTP,
		"biometricData": input.BiometricData,
		"consent":       input.Consent,
	}).Get(ctx, &verificationResult)
	if err != nil {
		return nil, fmt.Errorf("identity verification failed: %v", err)
	}

	verified := verificationResult["verified"].(bool)
	matchScore := verificationResult["matchScore"].(float64)

	// Step 5: Update beneficiary KYC status
	var updateErr error
	workflow.ExecuteActivity(ctx, UpdateBeneficiaryKYCStatusActivity, input.BeneficiaryID, map[string]interface{}{
		"kycStatus":   verified,
		"kycProvider": input.ProviderID,
		"matchScore":  matchScore,
		"verifiedAt":  time.Now(),
	}).Get(ctx, &updateErr)

	// Step 6: Store verification record
	var verificationID string
	workflow.ExecuteActivity(ctx, CreateVerificationRecordActivity, map[string]interface{}{
		"beneficiaryId": input.BeneficiaryID,
		"providerId":    input.ProviderID,
		"nationalId":    input.NationalID,
		"verified":      verified,
		"matchScore":    matchScore,
		"consentGiven":  true,
		"verifiedBy":    jc.ActorID,
	}).Get(ctx, &verificationID)

	// Step 7: Publish Kafka event
	workflow.ExecuteActivity(ctx, PublishKafkaEventActivity, "beneficiary.kyc_completed", map[string]interface{}{
		"beneficiaryId": input.BeneficiaryID,
		"verified":      verified,
		"providerId":    input.ProviderID,
		"correlationId": jc.CorrelationID,
	})

	// Step 8: Create audit log
	workflow.ExecuteActivity(ctx, CreateAuditLogActivity, map[string]interface{}{
		"action":        "kyc_verification",
		"entityType":    "beneficiary",
		"entityId":      input.BeneficiaryID,
		"performedBy":   jc.ActorID,
		"tenantId":      jc.TenantID,
		"correlationId": jc.CorrelationID,
		"details": map[string]interface{}{
			"providerId": input.ProviderID,
			"verified":   verified,
			"matchScore": matchScore,
		},
	})

	// Step 9: Write to lakehouse
	workflow.ExecuteActivity(ctx, WriteLakehouseFactActivity, "kyc_facts", map[string]interface{}{
		"beneficiaryId":    input.BeneficiaryID,
		"verificationDate": time.Now().Format("2006-01-02"),
		"providerId":       input.ProviderID,
		"verified":         verified,
		"matchScore":       matchScore,
		"tenantId":         jc.TenantID,
	})

	// Step 10: Emit journey completed event
	workflow.ExecuteActivity(ctx, EmitJourneyEventActivity, jc, "journey.completed", map[string]interface{}{
		"beneficiaryId": input.BeneficiaryID,
		"verified":      verified,
	})

	demographics, _ := verificationResult["demographics"].(map[string]interface{})

	return &KYCVerificationResult{
		JourneyRunID:   jc.JourneyRunID,
		BeneficiaryID:  input.BeneficiaryID,
		Verified:       verified,
		MatchScore:     matchScore,
		Demographics:   demographics,
		VerifiedAt:     time.Now(),
		VerificationID: verificationID,
	}, nil
}

// Journey 3: Household Registration
// UI Entry: Mobile HouseholdsScreen
// BFF: trpc.beneficiaries (household operations)
// Workflow: HouseholdRegistrationWorkflow
type HouseholdRegistrationInput struct {
	JourneyContext *JourneyContext `json:"journeyContext"`

	// Head of household
	HeadBeneficiaryID string                      `json:"headBeneficiaryId,omitempty"`
	HeadDetails       *BeneficiaryEnrollmentInput `json:"headDetails,omitempty"`

	// Household information
	HouseholdName  string `json:"householdName"`
	Address        string `json:"address"`
	Region         string `json:"region"`
	District       string `json:"district"`
	Ward           string `json:"ward,omitempty"`
	Village        string `json:"village,omitempty"`
	GPSCoordinates string `json:"gpsCoordinates,omitempty"`

	// Members
	Members []HouseholdMemberInput `json:"members"`

	// Characteristics for PMT
	Characteristics *PMTCharacteristics `json:"characteristics,omitempty"`
}

type HouseholdMemberInput struct {
	BeneficiaryID string `json:"beneficiaryId,omitempty"`
	FirstName     string `json:"firstName"`
	LastName      string `json:"lastName"`
	DateOfBirth   string `json:"dateOfBirth"`
	Gender        string `json:"gender"`
	Relationship  string `json:"relationship"` // "spouse", "child", "parent", "sibling", "other"
	NationalID    string `json:"nationalId,omitempty"`
}

type PMTCharacteristics struct {
	HouseholdSize       int      `json:"householdSize"`
	ChildrenUnder5      int      `json:"childrenUnder5"`
	ChildrenUnder18     int      `json:"childrenUnder18"`
	ElderlyOver65       int      `json:"elderlyOver65"`
	DisabledMembers     int      `json:"disabledMembers"`
	HousingType         string   `json:"housingType"`
	WallMaterial        string   `json:"wallMaterial"`
	RoofMaterial        string   `json:"roofMaterial"`
	FloorMaterial       string   `json:"floorMaterial"`
	WaterSource         string   `json:"waterSource"`
	SanitationType      string   `json:"sanitationType"`
	CookingFuel         string   `json:"cookingFuel"`
	ElectricityAccess   bool     `json:"electricityAccess"`
	NumberOfRooms       int      `json:"numberOfRooms"`
	LandOwnership       float64  `json:"landOwnership"`
	LivestockOwnership  float64  `json:"livestockOwnership"`
	VehicleOwnership    bool     `json:"vehicleOwnership"`
	ApplianceOwnership  []string `json:"applianceOwnership"`
	UrbanRural          string   `json:"urbanRural"`
	HasBankAccount      bool     `json:"hasBankAccount"`
	ReceivesRemittances bool     `json:"receivesRemittances"`
	HasHealthInsurance  bool     `json:"hasHealthInsurance"`
	ChildrenInSchool    int      `json:"childrenInSchool"`
	FoodSecurityScore   int      `json:"foodSecurityScore"`
}

type HouseholdRegistrationResult struct {
	JourneyRunID string    `json:"journeyRunId"`
	HouseholdID  string    `json:"householdId"`
	HeadID       string    `json:"headId"`
	MemberCount  int       `json:"memberCount"`
	PMTScore     float64   `json:"pmtScore,omitempty"`
	PMTCategory  string    `json:"pmtCategory,omitempty"`
	RegisteredAt time.Time `json:"registeredAt"`
}

// HouseholdRegistrationJourney orchestrates household registration with PMT scoring
func HouseholdRegistrationJourney(ctx workflow.Context, input HouseholdRegistrationInput) (*HouseholdRegistrationResult, error) {
	jc := input.JourneyContext

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    time.Minute,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Step 1: Emit journey started event
	workflow.ExecuteActivity(ctx, EmitJourneyEventActivity, jc, "journey.started", map[string]interface{}{
		"journeyType":   "household_registration",
		"householdName": input.HouseholdName,
		"memberCount":   len(input.Members),
	})

	// Step 2: Check authorization
	var authorized bool
	err := workflow.ExecuteActivity(ctx, CheckAuthorizationActivity, jc, "household:create").Get(ctx, &authorized)
	if err != nil || !authorized {
		return nil, fmt.Errorf("authorization failed")
	}

	// Step 3: Create or get head of household
	var headID string
	if input.HeadBeneficiaryID != "" {
		headID = input.HeadBeneficiaryID
	} else if input.HeadDetails != nil {
		// Enroll head as new beneficiary
		input.HeadDetails.IsHeadOfHousehold = true
		input.HeadDetails.JourneyContext = jc

		childCtx := workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
			WorkflowID: fmt.Sprintf("enroll-head-%s", jc.JourneyRunID),
		})

		var headResult *BeneficiaryEnrollmentResult
		err = workflow.ExecuteChildWorkflow(childCtx, BeneficiaryEnrollmentJourney, *input.HeadDetails).Get(ctx, &headResult)
		if err != nil {
			return nil, fmt.Errorf("failed to enroll head of household: %v", err)
		}
		headID = headResult.BeneficiaryID
	} else {
		return nil, fmt.Errorf("head of household not specified")
	}

	// Step 4: Create household record
	var householdID string
	err = workflow.ExecuteActivity(ctx, CreateHouseholdRecordActivity, map[string]interface{}{
		"householdName":  input.HouseholdName,
		"headId":         headID,
		"address":        input.Address,
		"region":         input.Region,
		"district":       input.District,
		"ward":           input.Ward,
		"village":        input.Village,
		"gpsCoordinates": input.GPSCoordinates,
		"tenantId":       jc.TenantID,
		"createdBy":      jc.ActorID,
	}).Get(ctx, &householdID)
	if err != nil {
		return nil, fmt.Errorf("failed to create household: %v", err)
	}

	// Step 5: Register household members
	memberCount := 1 // Head already counted
	for i, member := range input.Members {
		var memberID string
		if member.BeneficiaryID != "" {
			memberID = member.BeneficiaryID
		} else {
			// Create new beneficiary for member
			err = workflow.ExecuteActivity(ctx, CreateBeneficiaryRecordActivity, map[string]interface{}{
				"firstName":         member.FirstName,
				"lastName":          member.LastName,
				"dateOfBirth":       member.DateOfBirth,
				"gender":            member.Gender,
				"nationalId":        member.NationalID,
				"householdId":       householdID,
				"isHeadOfHousehold": false,
				"status":            "pending_verification",
				"createdBy":         jc.ActorID,
				"tenantId":          jc.TenantID,
			}).Get(ctx, &memberID)
			if err != nil {
				workflow.GetLogger(ctx).Warn("Failed to create member", "index", i, "error", err)
				continue
			}
		}

		// Link member to household
		workflow.ExecuteActivity(ctx, LinkMemberToHouseholdActivity, householdID, memberID, member.Relationship)
		memberCount++
	}

	// Step 6: Calculate PMT score if characteristics provided
	var pmtScore float64
	var pmtCategory string
	if input.Characteristics != nil {
		var pmtResult map[string]interface{}
		err = workflow.ExecuteActivity(ctx, CalculatePMTScoreActivity, householdID, input.Characteristics).Get(ctx, &pmtResult)
		if err == nil {
			pmtScore = pmtResult["score"].(float64)
			pmtCategory = pmtResult["category"].(string)

			// Update household with PMT score
			workflow.ExecuteActivity(ctx, UpdateHouseholdPMTActivity, householdID, pmtScore, pmtCategory)
		}
	}

	// Step 7: Publish Kafka event
	workflow.ExecuteActivity(ctx, PublishKafkaEventActivity, "household.registered", map[string]interface{}{
		"householdId":   householdID,
		"headId":        headID,
		"memberCount":   memberCount,
		"pmtScore":      pmtScore,
		"correlationId": jc.CorrelationID,
	})

	// Step 8: Create audit log
	workflow.ExecuteActivity(ctx, CreateAuditLogActivity, map[string]interface{}{
		"action":        "household_registered",
		"entityType":    "household",
		"entityId":      householdID,
		"performedBy":   jc.ActorID,
		"tenantId":      jc.TenantID,
		"correlationId": jc.CorrelationID,
		"details": map[string]interface{}{
			"memberCount": memberCount,
			"region":      input.Region,
			"district":    input.District,
		},
	})

	// Step 9: Write to lakehouse
	workflow.ExecuteActivity(ctx, WriteLakehouseFactActivity, "household_facts", map[string]interface{}{
		"householdId":      householdID,
		"registrationDate": time.Now().Format("2006-01-02"),
		"region":           input.Region,
		"district":         input.District,
		"memberCount":      memberCount,
		"pmtScore":         pmtScore,
		"pmtCategory":      pmtCategory,
		"tenantId":         jc.TenantID,
	})

	// Step 10: Emit journey completed event
	workflow.ExecuteActivity(ctx, EmitJourneyEventActivity, jc, "journey.completed", map[string]interface{}{
		"householdId": householdID,
		"memberCount": memberCount,
	})

	return &HouseholdRegistrationResult{
		JourneyRunID: jc.JourneyRunID,
		HouseholdID:  householdID,
		HeadID:       headID,
		MemberCount:  memberCount,
		PMTScore:     pmtScore,
		PMTCategory:  pmtCategory,
		RegisteredAt: time.Now(),
	}, nil
}

// Journey 4: Program Enrollment
// UI Entry: Admin Programs page, Beneficiary detail page
// BFF: trpc.programs
// Workflow: ProgramEnrollmentWorkflow
type ProgramEnrollmentInput struct {
	JourneyContext *JourneyContext `json:"journeyContext"`
	BeneficiaryID  string          `json:"beneficiaryId"`
	ProgramID      string          `json:"programId"`
	Justification  string          `json:"justification,omitempty"`
}

type ProgramEnrollmentResult struct {
	JourneyRunID      string    `json:"journeyRunId"`
	EnrollmentID      string    `json:"enrollmentId"`
	BeneficiaryID     string    `json:"beneficiaryId"`
	ProgramID         string    `json:"programId"`
	Status            string    `json:"status"`
	EnrolledAt        time.Time `json:"enrolledAt"`
	FirstDisbursement time.Time `json:"firstDisbursement,omitempty"`
}

// ProgramEnrollmentJourney orchestrates enrolling a beneficiary in a program
func ProgramEnrollmentJourney(ctx workflow.Context, input ProgramEnrollmentInput) (*ProgramEnrollmentResult, error) {
	jc := input.JourneyContext

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    time.Minute,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Step 1: Emit journey started event
	workflow.ExecuteActivity(ctx, EmitJourneyEventActivity, jc, "journey.started", map[string]interface{}{
		"journeyType":   "program_enrollment",
		"beneficiaryId": input.BeneficiaryID,
		"programId":     input.ProgramID,
	})

	// Step 2: Check authorization
	var authorized bool
	err := workflow.ExecuteActivity(ctx, CheckAuthorizationActivity, jc, "program:enroll").Get(ctx, &authorized)
	if err != nil || !authorized {
		return nil, fmt.Errorf("authorization failed")
	}

	// Step 3: Verify beneficiary exists and is active
	var beneficiaryStatus string
	err = workflow.ExecuteActivity(ctx, GetBeneficiaryStatusActivity, input.BeneficiaryID).Get(ctx, &beneficiaryStatus)
	if err != nil {
		return nil, fmt.Errorf("beneficiary not found: %v", err)
	}
	if beneficiaryStatus != "active" && beneficiaryStatus != "pending_verification" {
		return nil, fmt.Errorf("beneficiary not eligible: status is %s", beneficiaryStatus)
	}

	// Step 4: Verify program exists and is active
	var programStatus string
	err = workflow.ExecuteActivity(ctx, GetProgramStatusActivity, input.ProgramID).Get(ctx, &programStatus)
	if err != nil {
		return nil, fmt.Errorf("program not found: %v", err)
	}
	if programStatus != "active" {
		return nil, fmt.Errorf("program not active: status is %s", programStatus)
	}

	// Step 5: Check if already enrolled
	var existingEnrollment string
	workflow.ExecuteActivity(ctx, CheckExistingEnrollmentActivity, input.BeneficiaryID, input.ProgramID).Get(ctx, &existingEnrollment)
	if existingEnrollment != "" {
		return nil, fmt.Errorf("already enrolled in program: %s", existingEnrollment)
	}

	// Step 6: Create enrollment record
	var enrollmentID string
	err = workflow.ExecuteActivity(ctx, CreateProgramEnrollmentActivity, map[string]interface{}{
		"beneficiaryId": input.BeneficiaryID,
		"programId":     input.ProgramID,
		"status":        "active",
		"enrolledBy":    jc.ActorID,
		"justification": input.Justification,
		"tenantId":      jc.TenantID,
	}).Get(ctx, &enrollmentID)
	if err != nil {
		return nil, fmt.Errorf("failed to create enrollment: %v", err)
	}

	// Step 7: Publish Kafka event
	workflow.ExecuteActivity(ctx, PublishKafkaEventActivity, "beneficiary.program_enrolled", map[string]interface{}{
		"enrollmentId":  enrollmentID,
		"beneficiaryId": input.BeneficiaryID,
		"programId":     input.ProgramID,
		"correlationId": jc.CorrelationID,
	})

	// Step 8: Create audit log
	workflow.ExecuteActivity(ctx, CreateAuditLogActivity, map[string]interface{}{
		"action":        "program_enrollment",
		"entityType":    "enrollment",
		"entityId":      enrollmentID,
		"performedBy":   jc.ActorID,
		"tenantId":      jc.TenantID,
		"correlationId": jc.CorrelationID,
		"details": map[string]interface{}{
			"beneficiaryId": input.BeneficiaryID,
			"programId":     input.ProgramID,
		},
	})

	// Step 9: Write to lakehouse
	workflow.ExecuteActivity(ctx, WriteLakehouseFactActivity, "enrollment_facts", map[string]interface{}{
		"enrollmentId":   enrollmentID,
		"beneficiaryId":  input.BeneficiaryID,
		"programId":      input.ProgramID,
		"enrollmentDate": time.Now().Format("2006-01-02"),
		"tenantId":       jc.TenantID,
	})

	// Step 10: Emit journey completed event
	workflow.ExecuteActivity(ctx, EmitJourneyEventActivity, jc, "journey.completed", map[string]interface{}{
		"enrollmentId": enrollmentID,
	})

	return &ProgramEnrollmentResult{
		JourneyRunID:  jc.JourneyRunID,
		EnrollmentID:  enrollmentID,
		BeneficiaryID: input.BeneficiaryID,
		ProgramID:     input.ProgramID,
		Status:        "active",
		EnrolledAt:    time.Now(),
	}, nil
}

// Journey 5: Card Issuance
// UI Entry: Admin Beneficiary detail page
// BFF: trpc.beneficiaries
// Workflow: CardIssuanceWorkflow
type CardIssuanceInput struct {
	JourneyContext  *JourneyContext `json:"journeyContext"`
	BeneficiaryID   string          `json:"beneficiaryId"`
	CardType        string          `json:"cardType"` // "physical", "virtual"
	DeliveryAddress string          `json:"deliveryAddress,omitempty"`
}

type CardIssuanceResult struct {
	JourneyRunID  string    `json:"journeyRunId"`
	CardID        string    `json:"cardId"`
	BeneficiaryID string    `json:"beneficiaryId"`
	CardType      string    `json:"cardType"`
	Status        string    `json:"status"`
	MaskedNumber  string    `json:"maskedNumber"`
	IssuedAt      time.Time `json:"issuedAt"`
	ExpiresAt     time.Time `json:"expiresAt"`
}

// CardIssuanceJourney orchestrates card issuance for a beneficiary
func CardIssuanceJourney(ctx workflow.Context, input CardIssuanceInput) (*CardIssuanceResult, error) {
	jc := input.JourneyContext

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    time.Minute,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Step 1: Emit journey started event
	workflow.ExecuteActivity(ctx, EmitJourneyEventActivity, jc, "journey.started", map[string]interface{}{
		"journeyType":   "card_issuance",
		"beneficiaryId": input.BeneficiaryID,
		"cardType":      input.CardType,
	})

	// Step 2: Check authorization
	var authorized bool
	err := workflow.ExecuteActivity(ctx, CheckAuthorizationActivity, jc, "card:issue").Get(ctx, &authorized)
	if err != nil || !authorized {
		return nil, fmt.Errorf("authorization failed")
	}

	// Step 3: Verify beneficiary has TigerBeetle account
	var accountID string
	err = workflow.ExecuteActivity(ctx, GetBeneficiaryAccountActivity, input.BeneficiaryID).Get(ctx, &accountID)
	if err != nil || accountID == "" {
		return nil, fmt.Errorf("beneficiary does not have a payment account")
	}

	// Step 4: Check for existing active card
	var existingCard string
	workflow.ExecuteActivity(ctx, CheckExistingCardActivity, input.BeneficiaryID).Get(ctx, &existingCard)
	if existingCard != "" {
		return nil, fmt.Errorf("beneficiary already has an active card: %s", existingCard)
	}

	// Step 5: Generate card details
	var cardDetails map[string]interface{}
	err = workflow.ExecuteActivity(ctx, GenerateCardDetailsActivity, input.BeneficiaryID, input.CardType).Get(ctx, &cardDetails)
	if err != nil {
		return nil, fmt.Errorf("failed to generate card: %v", err)
	}

	// Step 6: Create card record
	var cardID string
	err = workflow.ExecuteActivity(ctx, CreateCardRecordActivity, map[string]interface{}{
		"beneficiaryId":   input.BeneficiaryID,
		"accountId":       accountID,
		"cardType":        input.CardType,
		"maskedNumber":    cardDetails["maskedNumber"],
		"expiresAt":       cardDetails["expiresAt"],
		"deliveryAddress": input.DeliveryAddress,
		"status":          "active",
		"issuedBy":        jc.ActorID,
		"tenantId":        jc.TenantID,
	}).Get(ctx, &cardID)
	if err != nil {
		return nil, fmt.Errorf("failed to create card record: %v", err)
	}

	// Step 7: If physical card, initiate delivery
	if input.CardType == "physical" && input.DeliveryAddress != "" {
		workflow.ExecuteActivity(ctx, InitiateCardDeliveryActivity, cardID, input.DeliveryAddress)
	}

	// Step 8: Send notification
	workflow.ExecuteActivity(ctx, SendCardIssuanceNotificationActivity, input.BeneficiaryID, cardID, input.CardType)

	// Step 9: Publish Kafka event
	workflow.ExecuteActivity(ctx, PublishKafkaEventActivity, "card.issued", map[string]interface{}{
		"cardId":        cardID,
		"beneficiaryId": input.BeneficiaryID,
		"cardType":      input.CardType,
		"correlationId": jc.CorrelationID,
	})

	// Step 10: Create audit log
	workflow.ExecuteActivity(ctx, CreateAuditLogActivity, map[string]interface{}{
		"action":        "card_issued",
		"entityType":    "card",
		"entityId":      cardID,
		"performedBy":   jc.ActorID,
		"tenantId":      jc.TenantID,
		"correlationId": jc.CorrelationID,
		"details": map[string]interface{}{
			"beneficiaryId": input.BeneficiaryID,
			"cardType":      input.CardType,
		},
	})

	// Step 11: Emit journey completed event
	workflow.ExecuteActivity(ctx, EmitJourneyEventActivity, jc, "journey.completed", map[string]interface{}{
		"cardId": cardID,
	})

	expiresAt, _ := cardDetails["expiresAt"].(time.Time)
	maskedNumber, _ := cardDetails["maskedNumber"].(string)

	return &CardIssuanceResult{
		JourneyRunID:  jc.JourneyRunID,
		CardID:        cardID,
		BeneficiaryID: input.BeneficiaryID,
		CardType:      input.CardType,
		Status:        "active",
		MaskedNumber:  maskedNumber,
		IssuedAt:      time.Now(),
		ExpiresAt:     expiresAt,
	}, nil
}

// RegisterEnrollmentJourneys registers all enrollment journey definitions
func RegisterEnrollmentJourneys(registry *JourneyRegistry) {
	registry.Register(&JourneyDefinition{
		Key:                 "beneficiary_enrollment",
		Name:                "Beneficiary Enrollment",
		Description:         "Complete end-to-end beneficiary enrollment with KYC and account creation",
		Category:            "enrollment",
		WorkflowType:        "BeneficiaryEnrollmentJourney",
		RequiredPermissions: []string{"beneficiary:create", "beneficiary:verify"},
		UIEntryPoints:       []string{"Mobile:EnrollBeneficiaryScreen", "Admin:BeneficiariesPage"},
		BFFEndpoints:        []string{"trpc.beneficiaries.create"},
		MiddlewareHooks:     []string{"kafka", "redis", "permify", "tigerbeetle", "lakehouse"},
	})

	registry.Register(&JourneyDefinition{
		Key:                 "kyc_verification",
		Name:                "KYC Verification",
		Description:         "Verify beneficiary identity via national ID federation",
		Category:            "enrollment",
		WorkflowType:        "KYCVerificationJourney",
		RequiredPermissions: []string{"beneficiary:verify"},
		UIEntryPoints:       []string{"Mobile:VerifyIDScreen", "Admin:BeneficiaryDetailPage"},
		BFFEndpoints:        []string{"trpc.worldClass.federation.verify"},
		MiddlewareHooks:     []string{"kafka", "redis", "permify", "lakehouse"},
	})

	registry.Register(&JourneyDefinition{
		Key:                 "household_registration",
		Name:                "Household Registration",
		Description:         "Register household with members and PMT scoring",
		Category:            "enrollment",
		WorkflowType:        "HouseholdRegistrationJourney",
		RequiredPermissions: []string{"household:create", "beneficiary:create"},
		UIEntryPoints:       []string{"Mobile:HouseholdsScreen"},
		BFFEndpoints:        []string{"trpc.beneficiaries.createHousehold"},
		MiddlewareHooks:     []string{"kafka", "redis", "permify", "pmt-service", "lakehouse"},
	})

	registry.Register(&JourneyDefinition{
		Key:                 "program_enrollment",
		Name:                "Program Enrollment",
		Description:         "Enroll beneficiary in a benefit program",
		Category:            "enrollment",
		WorkflowType:        "ProgramEnrollmentJourney",
		RequiredPermissions: []string{"program:enroll"},
		UIEntryPoints:       []string{"Admin:ProgramsPage", "Admin:BeneficiaryDetailPage"},
		BFFEndpoints:        []string{"trpc.programs.enroll"},
		MiddlewareHooks:     []string{"kafka", "redis", "permify", "lakehouse"},
	})

	registry.Register(&JourneyDefinition{
		Key:                 "card_issuance",
		Name:                "Card Issuance",
		Description:         "Issue payment card to beneficiary",
		Category:            "enrollment",
		WorkflowType:        "CardIssuanceJourney",
		RequiredPermissions: []string{"card:issue"},
		UIEntryPoints:       []string{"Admin:BeneficiaryDetailPage"},
		BFFEndpoints:        []string{"trpc.beneficiaries.issueCard"},
		MiddlewareHooks:     []string{"kafka", "redis", "permify", "tigerbeetle"},
	})
}

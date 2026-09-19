package workflows

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// KYCVerificationInput represents the input for KYC verification
type KYCVerificationInput struct {
	BeneficiaryID   string                   `json:"beneficiaryID"`
	Documents       []KYCDocument            `json:"documents"`
	BiometricData   map[string]interface{}   `json:"biometricData"`
	VerificationLevel string                 `json:"verificationLevel"` // basic, standard, enhanced
	VerifiedBy      string                   `json:"verifiedBy"`
}

type KYCDocument struct {
	Type        string `json:"type"`
	FileURL     string `json:"fileURL"`
	FileName    string `json:"fileName"`
	IssueDate   string `json:"issueDate"`
	ExpiryDate  string `json:"expiryDate"`
}

// KYCVerificationResult represents the workflow result
type KYCVerificationResult struct {
	BeneficiaryID     string                 `json:"beneficiaryID"`
	Status            string                 `json:"status"` // approved, rejected, pending_review
	VerificationLevel string                 `json:"verificationLevel"`
	Score             float64                `json:"score"`
	Issues            []string               `json:"issues"`
	ExtractedData     map[string]interface{} `json:"extractedData"`
	Message           string                 `json:"message"`
}

// KYCVerificationWorkflow orchestrates the complete KYC verification process
func KYCVerificationWorkflow(ctx workflow.Context, input KYCVerificationInput) (*KYCVerificationResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting KYC verification workflow", "beneficiaryID", input.BeneficiaryID)

	activityOptions := workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    time.Minute,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, activityOptions)

	var result KYCVerificationResult
	result.BeneficiaryID = input.BeneficiaryID
	result.VerificationLevel = input.VerificationLevel
	result.Issues = []string{}
	result.ExtractedData = make(map[string]interface{})

	// Step 1: Extract text from documents using OCR (Python service)
	var extractedTexts map[string]string
	err := workflow.ExecuteActivity(ctx, "ExtractDocumentTextActivity", input.Documents).Get(ctx, &extractedTexts)
	if err != nil {
		logger.Error("Document text extraction failed", "error", err)
		result.Issues = append(result.Issues, "Failed to extract document text")
		// Continue with manual review
	} else {
		result.ExtractedData["documentTexts"] = extractedTexts
	}

	// Step 2: Validate document authenticity using ML
	var authenticityResults map[string]interface{}
	err = workflow.ExecuteActivity(ctx, "ValidateDocumentAuthenticityActivity", input.Documents, extractedTexts).Get(ctx, &authenticityResults)
	if err != nil {
		logger.Error("Document authenticity validation failed", "error", err)
		result.Issues = append(result.Issues, "Document authenticity check failed")
	} else {
		result.ExtractedData["authenticity"] = authenticityResults
		
		// Check if any documents failed authenticity
		for docType, authResult := range authenticityResults {
			if authData, ok := authResult.(map[string]interface{}); ok {
				if !authData["authentic"].(bool) {
					result.Issues = append(result.Issues, "Document "+docType+" failed authenticity check")
				}
			}
		}
	}

	// Step 3: Verify biometric data (if provided)
	if len(input.BiometricData) > 0 {
		var biometricValid bool
		err = workflow.ExecuteActivity(ctx, "VerifyBiometricDataActivity", input.BeneficiaryID, input.BiometricData).Get(ctx, &biometricValid)
		if err != nil {
			logger.Warn("Biometric verification failed", "error", err)
			result.Issues = append(result.Issues, "Biometric verification failed")
		} else if !biometricValid {
			result.Issues = append(result.Issues, "Biometric data does not match")
		}
		result.ExtractedData["biometricVerified"] = biometricValid
	}

	// Step 4: Cross-check with national ID database
	var nationalIDData map[string]interface{}
	err = workflow.ExecuteActivity(ctx, "CrossCheckNationalIDActivity", input.BeneficiaryID).Get(ctx, &nationalIDData)
	if err != nil {
		logger.Warn("National ID cross-check failed", "error", err)
		result.Issues = append(result.Issues, "National ID verification failed")
	} else {
		result.ExtractedData["nationalIDData"] = nationalIDData
		
		// Verify data consistency
		if !nationalIDData["verified"].(bool) {
			result.Issues = append(result.Issues, "National ID data mismatch")
		}
	}

	// Step 5: Check against sanctions/watchlists (via external API)
	var sanctionsCheck map[string]interface{}
	err = workflow.ExecuteActivity(ctx, "CheckSanctionsWatchlistActivity", input.BeneficiaryID).Get(ctx, &sanctionsCheck)
	if err != nil {
		logger.Warn("Sanctions check failed", "error", err)
		result.Issues = append(result.Issues, "Sanctions check failed")
	} else if sanctionsCheck["flagged"].(bool) {
		result.Status = "rejected"
		result.Message = "Beneficiary flagged on sanctions/watchlist"
		result.Issues = append(result.Issues, "Flagged on watchlist")
		
		// Publish event and return early
		_ = workflow.ExecuteActivity(ctx, "PublishKafkaEventActivity", "kyc.rejected", input.BeneficiaryID, result).Get(ctx, nil)
		_ = workflow.ExecuteActivity(ctx, "CreateAuditLogActivity", "kyc.rejected", input.BeneficiaryID, input.VerifiedBy, result).Get(ctx, nil)
		
		return &result, nil
	}

	// Step 6: Calculate KYC score
	var kycScore float64
	err = workflow.ExecuteActivity(ctx, "CalculateKYCScoreActivity", result.ExtractedData, result.Issues, input.VerificationLevel).Get(ctx, &kycScore)
	if err != nil {
		logger.Error("KYC score calculation failed", "error", err)
		kycScore = 0.0
	}
	result.Score = kycScore

	// Step 7: Determine status based on score and issues
	if len(result.Issues) == 0 && kycScore >= 0.8 {
		result.Status = "approved"
		result.Message = "KYC verification approved"
	} else if kycScore >= 0.6 && len(result.Issues) <= 2 {
		result.Status = "pending_review"
		result.Message = "KYC requires manual review"
	} else {
		result.Status = "rejected"
		result.Message = "KYC verification rejected"
	}

	// Step 8: Update beneficiary KYC status in database
	err = workflow.ExecuteActivity(ctx, "UpdateBeneficiaryKYCStatusActivity", input.BeneficiaryID, result.Status, kycScore).Get(ctx, nil)
	if err != nil {
		logger.Error("Failed to update KYC status", "error", err)
		return nil, err
	}

	// Step 9: Publish Kafka event
	eventTopic := "kyc." + result.Status
	err = workflow.ExecuteActivity(ctx, "PublishKafkaEventActivity", eventTopic, input.BeneficiaryID, map[string]interface{}{
		"beneficiaryID":     input.BeneficiaryID,
		"status":            result.Status,
		"score":             kycScore,
		"verificationLevel": input.VerificationLevel,
		"verifiedBy":        input.VerifiedBy,
		"issues":            result.Issues,
	}).Get(ctx, nil)
	if err != nil {
		logger.Warn("Failed to publish Kafka event", "error", err)
	}

	// Step 10: Send notification
	var notificationMessage string
	if result.Status == "approved" {
		notificationMessage = "Your KYC verification has been approved. You can now access all services."
	} else if result.Status == "pending_review" {
		notificationMessage = "Your KYC verification is under review. We will notify you once complete."
	} else {
		notificationMessage = "Your KYC verification was not successful. Please contact support."
	}
	
	err = workflow.ExecuteActivity(ctx, "SendNotificationActivity", input.BeneficiaryID, notificationMessage).Get(ctx, nil)
	if err != nil {
		logger.Warn("Failed to send notification", "error", err)
	}

	// Step 11: Create audit log
	err = workflow.ExecuteActivity(ctx, "CreateAuditLogActivity", "kyc.verified", input.BeneficiaryID, input.VerifiedBy, result).Get(ctx, nil)
	if err != nil {
		logger.Warn("Failed to create audit log", "error", err)
	}

	// Step 12: Cache KYC result in Redis
	err = workflow.ExecuteActivity(ctx, "CacheKYCResultActivity", input.BeneficiaryID, result).Get(ctx, nil)
	if err != nil {
		logger.Warn("Failed to cache KYC result", "error", err)
	}

	logger.Info("KYC verification completed", "beneficiaryID", input.BeneficiaryID, "status", result.Status, "score", kycScore)
	return &result, nil
}

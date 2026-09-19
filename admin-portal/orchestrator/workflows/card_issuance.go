package workflows

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// CardIssuanceInput represents the input for card issuance
type CardIssuanceInput struct {
	BeneficiaryID   string `json:"beneficiaryID"`
	CardType        string `json:"cardType"` // physical, virtual
	DeliveryAddress string `json:"deliveryAddress"`
	RequestedBy     string `json:"requestedBy"`
}

// CardIssuanceResult represents the workflow result
type CardIssuanceResult struct {
	BeneficiaryID string `json:"beneficiaryID"`
	CardNumber    string `json:"cardNumber"`
	CardType      string `json:"cardType"`
	Status        string `json:"status"` // issued, pending, failed
	TrackingID    string `json:"trackingID"`
	Message       string `json:"message"`
}

// CardIssuanceWorkflow orchestrates the card issuance process
func CardIssuanceWorkflow(ctx workflow.Context, input CardIssuanceInput) (*CardIssuanceResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting card issuance workflow", "beneficiaryID", input.BeneficiaryID)

	activityOptions := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    5 * time.Minute,
			MaximumAttempts:    5,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, activityOptions)

	var result CardIssuanceResult
	result.BeneficiaryID = input.BeneficiaryID
	result.CardType = input.CardType

	// Step 1: Verify beneficiary eligibility
	var eligible bool
	err := workflow.ExecuteActivity(ctx, "CheckBeneficiaryEligibilityActivity", input.BeneficiaryID).Get(ctx, &eligible)
	if err != nil || !eligible {
		result.Status = "failed"
		result.Message = "Beneficiary not eligible for card issuance"
		return &result, err
	}

	// Step 2: Check if beneficiary already has an active card
	var hasActiveCard bool
	err = workflow.ExecuteActivity(ctx, "CheckExistingCardActivity", input.BeneficiaryID).Get(ctx, &hasActiveCard)
	if err != nil {
		logger.Warn("Failed to check existing card", "error", err)
	} else if hasActiveCard {
		result.Status = "failed"
		result.Message = "Beneficiary already has an active card"
		return &result, nil
	}

	// Step 3: Generate card number via TigerBeetle
	var cardNumber string
	err = workflow.ExecuteActivity(ctx, "GenerateCardNumberActivity", input.BeneficiaryID).Get(ctx, &cardNumber)
	if err != nil {
		logger.Error("Card number generation failed", "error", err)
		result.Status = "failed"
		result.Message = "Failed to generate card number"
		return &result, err
	}
	result.CardNumber = cardNumber

	// Step 4: Create TigerBeetle account for the card
	var accountID string
	err = workflow.ExecuteActivity(ctx, "CreateTigerBeetleCardAccountActivity", input.BeneficiaryID, cardNumber).Get(ctx, &accountID)
	if err != nil {
		logger.Error("TigerBeetle account creation failed", "error", err)
		result.Status = "failed"
		result.Message = "Failed to create card account"
		return &result, err
	}

	// Step 5: Register card in database
	err = workflow.ExecuteActivity(ctx, "RegisterCardActivity", input.BeneficiaryID, cardNumber, input.CardType, accountID).Get(ctx, nil)
	if err != nil {
		logger.Error("Card registration failed", "error", err)
		result.Status = "failed"
		result.Message = "Failed to register card"
		return &result, err
	}

	// Step 6: If physical card, initiate production and delivery
	if input.CardType == "physical" {
		var trackingID string
		err = workflow.ExecuteActivity(ctx, "InitiateCardProductionActivity", input.BeneficiaryID, cardNumber, input.DeliveryAddress).Get(ctx, &trackingID)
		if err != nil {
			logger.Warn("Card production initiation failed", "error", err)
			result.Message = "Card registered but production failed"
		} else {
			result.TrackingID = trackingID
			result.Message = "Physical card production initiated"
		}
	} else {
		result.Message = "Virtual card issued successfully"
	}

	result.Status = "issued"

	// Step 7: Publish Kafka event
	err = workflow.ExecuteActivity(ctx, "PublishKafkaEventActivity", "card.issued", input.BeneficiaryID, map[string]interface{}{
		"beneficiaryID": input.BeneficiaryID,
		"cardNumber":    cardNumber,
		"cardType":      input.CardType,
		"trackingID":    result.TrackingID,
	}).Get(ctx, nil)
	if err != nil {
		logger.Warn("Failed to publish Kafka event", "error", err)
	}

	// Step 8: Send notification
	var notificationMsg string
	if input.CardType == "physical" {
		notificationMsg = "Your payment card has been issued and will be delivered to " + input.DeliveryAddress + ". Tracking ID: " + result.TrackingID
	} else {
		notificationMsg = "Your virtual payment card has been issued. Card number: " + maskCardNumber(cardNumber)
	}
	
	err = workflow.ExecuteActivity(ctx, "SendNotificationActivity", input.BeneficiaryID, notificationMsg).Get(ctx, nil)
	if err != nil {
		logger.Warn("Failed to send notification", "error", err)
	}

	// Step 9: Create audit log
	err = workflow.ExecuteActivity(ctx, "CreateAuditLogActivity", "card.issued", input.BeneficiaryID, input.RequestedBy, result).Get(ctx, nil)
	if err != nil {
		logger.Warn("Failed to create audit log", "error", err)
	}

	logger.Info("Card issuance completed", "beneficiaryID", input.BeneficiaryID, "cardNumber", cardNumber)
	return &result, nil
}

func maskCardNumber(cardNumber string) string {
	if len(cardNumber) < 4 {
		return "****"
	}
	return "****-****-****-" + cardNumber[len(cardNumber)-4:]
}

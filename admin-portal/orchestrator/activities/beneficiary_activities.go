package activities

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/admin-portal/orchestrator/clients"
)

type BeneficiaryActivities struct {
	kafkaClient *clients.KafkaClient
	daprClient  *clients.DaprClient
	redisClient *clients.RedisClient
}

func NewBeneficiaryActivities(kafka *clients.KafkaClient, dapr *clients.DaprClient, redis *clients.RedisClient) *BeneficiaryActivities {
	return &BeneficiaryActivities{
		kafkaClient: kafka,
		daprClient:  dapr,
		redisClient: redis,
	}
}

// ValidateNationalIDActivity validates national ID against external API
func (a *BeneficiaryActivities) ValidateNationalIDActivity(ctx context.Context, nationalID string) (bool, error) {
	// Call external national ID verification API via Dapr
	// For now, simple validation logic
	if len(nationalID) < 8 {
		return false, nil
	}

	// Call ID verification service via Dapr
	payload := map[string]interface{}{
		"nationalID": nationalID,
	}

	resp, err := a.daprClient.InvokeService(ctx, "id-verification-service", "verify", payload)
	if err != nil {
		// Graceful degradation - if service unavailable, do basic validation
		return len(nationalID) >= 8, nil
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp, &result); err != nil {
		return false, err
	}

	valid, ok := result["valid"].(bool)
	if !ok {
		return false, fmt.Errorf("invalid response from ID verification service")
	}

	return valid, nil
}

// CheckDuplicateBeneficiaryActivity checks for duplicate beneficiaries
func (a *BeneficiaryActivities) CheckDuplicateBeneficiaryActivity(ctx context.Context, nationalID, phoneNumber string) (bool, error) {
	// Check Redis cache first
	cacheKey := fmt.Sprintf("beneficiary:nationalid:%s", nationalID)
	exists, err := a.redisClient.Exists(ctx, cacheKey)
	if err == nil && exists {
		return true, nil
	}

	// Check database via Dapr service invocation
	payload := map[string]interface{}{
		"nationalID":  nationalID,
		"phoneNumber": phoneNumber,
	}

	resp, err := a.daprClient.InvokeService(ctx, "beneficiary-service", "check-duplicate", payload)
	if err != nil {
		return false, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp, &result); err != nil {
		return false, err
	}

	isDuplicate, ok := result["isDuplicate"].(bool)
	if !ok {
		return false, fmt.Errorf("invalid response from beneficiary service")
	}

	return isDuplicate, nil
}

// ValidateDocumentsActivity validates uploaded documents using Python ML service
func (a *BeneficiaryActivities) ValidateDocumentsActivity(ctx context.Context, documents []interface{}) (bool, error) {
	// Call Python ML service for document validation
	payload := map[string]interface{}{
		"documents": documents,
	}

	resp, err := a.daprClient.InvokeService(ctx, "ml-document-service", "validate", payload)
	if err != nil {
		// Graceful degradation
		return false, nil
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp, &result); err != nil {
		return false, err
	}

	valid, ok := result["valid"].(bool)
	if !ok {
		return false, fmt.Errorf("invalid response from ML service")
	}

	confidence, _ := result["confidence"].(float64)
	
	// Require 80% confidence for auto-approval
	return valid && confidence >= 0.8, nil
}

// CreateBeneficiaryRecordActivity creates beneficiary record in database
func (a *BeneficiaryActivities) CreateBeneficiaryRecordActivity(ctx context.Context, input interface{}) (string, error) {
	// Call beneficiary service to create record
	resp, err := a.daprClient.InvokeService(ctx, "beneficiary-service", "create", input)
	if err != nil {
		return "", err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp, &result); err != nil {
		return "", err
	}

	beneficiaryID, ok := result["beneficiaryID"].(string)
	if !ok {
		return "", fmt.Errorf("invalid response from beneficiary service")
	}

	return beneficiaryID, nil
}

// CreateTigerBeetleAccountActivity creates financial account in TigerBeetle
func (a *BeneficiaryActivities) CreateTigerBeetleAccountActivity(ctx context.Context, beneficiaryID, programID string) (string, error) {
	// Call TigerBeetle service to create account
	payload := map[string]interface{}{
		"beneficiaryID": beneficiaryID,
		"programID":     programID,
		"currency":      "USD",
		"accountType":   "beneficiary",
	}

	resp, err := a.daprClient.InvokeService(ctx, "tigerbeetle-service", "create-account", payload)
	if err != nil {
		return "", err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp, &result); err != nil {
		return "", err
	}

	accountID, ok := result["accountID"].(string)
	if !ok {
		return "", fmt.Errorf("invalid response from TigerBeetle service")
	}

	return accountID, nil
}

// PublishKafkaEventActivity publishes event to Kafka
func (a *BeneficiaryActivities) PublishKafkaEventActivity(ctx context.Context, topic, key string, value interface{}) error {
	return a.kafkaClient.PublishEvent(ctx, topic, key, value)
}

// CacheBeneficiaryDataActivity caches beneficiary data in Redis
func (a *BeneficiaryActivities) CacheBeneficiaryDataActivity(ctx context.Context, beneficiaryID string, data interface{}) error {
	cacheKey := fmt.Sprintf("beneficiary:%s", beneficiaryID)
	return a.redisClient.Set(ctx, cacheKey, data, 24*time.Hour)
}

// SendSMSNotificationActivity sends SMS notification
func (a *BeneficiaryActivities) SendSMSNotificationActivity(ctx context.Context, phoneNumber, message string) error {
	payload := map[string]interface{}{
		"phoneNumber": phoneNumber,
		"message":     message,
	}

	_, err := a.daprClient.InvokeService(ctx, "sms-service", "send", payload)
	return err
}

// CreateAuditLogActivity creates audit log entry
func (a *BeneficiaryActivities) CreateAuditLogActivity(ctx context.Context, eventType, entityID, userID string, metadata interface{}) error {
	payload := map[string]interface{}{
		"eventType": eventType,
		"entityID":  entityID,
		"userID":    userID,
		"metadata":  metadata,
		"timestamp": time.Now().UTC(),
	}

	// Publish to Kafka for audit trail
	return a.kafkaClient.PublishEvent(ctx, "audit.logs", entityID, payload)
}

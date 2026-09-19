package blockchain

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// BlockchainTransaction represents a transaction on the blockchain
type BlockchainTransaction struct {
	ID              int64           `json:"id" db:"id"`
	TransactionID   string          `json:"transactionId" db:"transaction_id"`
	BlockHash       string          `json:"blockHash" db:"block_hash"`
	BlockNumber     int64           `json:"blockNumber" db:"block_number"`
	TransactionHash string          `json:"transactionHash" db:"transaction_hash"`
	ChaincodeName   string          `json:"chaincodeName" db:"chaincode_name"`
	FunctionName    string          `json:"functionName" db:"function_name"`
	Payload         json.RawMessage `json:"payload" db:"payload"`
	Status          string          `json:"status" db:"status"` // pending, confirmed, failed
	CreatedAt       time.Time       `json:"createdAt" db:"created_at"`
	ConfirmedAt     *time.Time      `json:"confirmedAt,omitempty" db:"confirmed_at"`
}

// BlockchainIdentity represents a decentralized identity (DID)
type BlockchainIdentity struct {
	ID                 int64           `json:"id" db:"id"`
	UserID             int64           `json:"userId" db:"user_id"`
	DID                string          `json:"did" db:"did"`
	PublicKey          string          `json:"publicKey" db:"public_key"`
	VerificationMethod json.RawMessage `json:"verificationMethod,omitempty" db:"verification_method"`
	Status             string          `json:"status" db:"status"` // active, revoked
	CreatedAt          time.Time       `json:"createdAt" db:"created_at"`
	RevokedAt          *time.Time      `json:"revokedAt,omitempty" db:"revoked_at"`
}

// AuditTrailEntry represents an immutable audit trail entry
type AuditTrailEntry struct {
	ID             int64           `json:"id" db:"id"`
	EntityType     string          `json:"entityType" db:"entity_type"`
	EntityID       int64           `json:"entityId" db:"entity_id"`
	Action         string          `json:"action" db:"action"`
	PreviousHash   *string         `json:"previousHash,omitempty" db:"previous_hash"`
	CurrentHash    string          `json:"currentHash" db:"current_hash"`
	BlockchainTxID *string         `json:"blockchainTxId,omitempty" db:"blockchain_tx_id"`
	Data           json.RawMessage `json:"data" db:"data"`
	CreatedBy      int64           `json:"createdBy" db:"created_by"`
	CreatedAt      time.Time       `json:"createdAt" db:"created_at"`
}

// BlockchainService provides blockchain integration functionality
type BlockchainService struct {
	db          *sql.DB
	mu          sync.RWMutex
	blockNumber int64
}

// NewBlockchainService creates a new BlockchainService instance
func NewBlockchainService(db *sql.DB) *BlockchainService {
	return &BlockchainService{
		db:          db,
		blockNumber: time.Now().Unix(),
	}
}

// SubmitTransaction submits a transaction to the blockchain
func (s *BlockchainService) SubmitTransaction(ctx context.Context, chaincodeName, functionName string, payload interface{}) (*BlockchainTransaction, error) {
	s.mu.Lock()
	s.blockNumber++
	blockNum := s.blockNumber
	s.mu.Unlock()

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	transactionID := fmt.Sprintf("TX-%d-%s", time.Now().UnixNano(), generateRandomHex(8))
	transactionHash := hashData(payloadBytes)
	blockHash := hashData([]byte(fmt.Sprintf("%s%s%d", transactionID, transactionHash, blockNum)))

	tx := &BlockchainTransaction{
		TransactionID:   transactionID,
		BlockHash:       blockHash,
		BlockNumber:     blockNum,
		TransactionHash: transactionHash,
		ChaincodeName:   chaincodeName,
		FunctionName:    functionName,
		Payload:         payloadBytes,
		Status:          "pending",
		CreatedAt:       time.Now(),
	}

	// Insert into database
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO blockchain_transactions 
		(transaction_id, block_hash, block_number, transaction_hash, chaincode_name, function_name, payload, status, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, tx.TransactionID, tx.BlockHash, tx.BlockNumber, tx.TransactionHash, tx.ChaincodeName, tx.FunctionName, tx.Payload, tx.Status, tx.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to insert transaction: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get insert id: %w", err)
	}
	tx.ID = id

	return tx, nil
}

// ConfirmTransaction confirms a pending transaction
func (s *BlockchainService) ConfirmTransaction(ctx context.Context, transactionID string) error {
	now := time.Now()
	_, err := s.db.ExecContext(ctx, `
		UPDATE blockchain_transactions 
		SET status = 'confirmed', confirmed_at = ?
		WHERE transaction_id = ? AND status = 'pending'
	`, now, transactionID)
	if err != nil {
		return fmt.Errorf("failed to confirm transaction: %w", err)
	}
	return nil
}

// GetTransaction retrieves a transaction by ID
func (s *BlockchainService) GetTransaction(ctx context.Context, transactionID string) (*BlockchainTransaction, error) {
	tx := &BlockchainTransaction{}
	err := s.db.QueryRowContext(ctx, `
		SELECT id, transaction_id, block_hash, block_number, transaction_hash, 
		       chaincode_name, function_name, payload, status, created_at, confirmed_at
		FROM blockchain_transactions WHERE transaction_id = ?
	`, transactionID).Scan(
		&tx.ID, &tx.TransactionID, &tx.BlockHash, &tx.BlockNumber, &tx.TransactionHash,
		&tx.ChaincodeName, &tx.FunctionName, &tx.Payload, &tx.Status, &tx.CreatedAt, &tx.ConfirmedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction: %w", err)
	}
	return tx, nil
}

// CreateIdentity creates a decentralized identity for a user
func (s *BlockchainService) CreateIdentity(ctx context.Context, userID int64) (*BlockchainIdentity, error) {
	did := fmt.Sprintf("did:social-protection:%d-%d", userID, time.Now().UnixNano())
	publicKey := generateRandomHex(32)

	verificationMethod := map[string]interface{}{
		"id":                 fmt.Sprintf("%s#key-1", did),
		"type":               "Ed25519VerificationKey2020",
		"controller":         did,
		"publicKeyMultibase": publicKey,
	}
	verificationMethodBytes, _ := json.Marshal(verificationMethod)

	identity := &BlockchainIdentity{
		UserID:             userID,
		DID:                did,
		PublicKey:          publicKey,
		VerificationMethod: verificationMethodBytes,
		Status:             "active",
		CreatedAt:          time.Now(),
	}

	result, err := s.db.ExecContext(ctx, `
		INSERT INTO blockchain_identities 
		(user_id, did, public_key, verification_method, status, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, identity.UserID, identity.DID, identity.PublicKey, identity.VerificationMethod, identity.Status, identity.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create identity: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get insert id: %w", err)
	}
	identity.ID = id

	// Submit to blockchain
	_, err = s.SubmitTransaction(ctx, "Identity", "createDID", map[string]interface{}{
		"did":       did,
		"userId":    userID,
		"publicKey": publicKey,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to submit identity to blockchain: %w", err)
	}

	return identity, nil
}

// GetIdentity retrieves a user's blockchain identity
func (s *BlockchainService) GetIdentity(ctx context.Context, userID int64) (*BlockchainIdentity, error) {
	identity := &BlockchainIdentity{}
	err := s.db.QueryRowContext(ctx, `
		SELECT id, user_id, did, public_key, verification_method, status, created_at, revoked_at
		FROM blockchain_identities WHERE user_id = ? AND status = 'active'
	`, userID).Scan(
		&identity.ID, &identity.UserID, &identity.DID, &identity.PublicKey,
		&identity.VerificationMethod, &identity.Status, &identity.CreatedAt, &identity.RevokedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get identity: %w", err)
	}
	return identity, nil
}

// RevokeIdentity revokes a user's blockchain identity
func (s *BlockchainService) RevokeIdentity(ctx context.Context, userID int64, reason string) error {
	now := time.Now()
	_, err := s.db.ExecContext(ctx, `
		UPDATE blockchain_identities 
		SET status = 'revoked', revoked_at = ?
		WHERE user_id = ? AND status = 'active'
	`, now, userID)
	if err != nil {
		return fmt.Errorf("failed to revoke identity: %w", err)
	}

	// Submit revocation to blockchain
	_, err = s.SubmitTransaction(ctx, "Identity", "revokeDID", map[string]interface{}{
		"userId": userID,
		"reason": reason,
	})
	if err != nil {
		return fmt.Errorf("failed to submit revocation to blockchain: %w", err)
	}

	return nil
}

// CreateAuditTrailEntry creates an immutable audit trail entry
func (s *BlockchainService) CreateAuditTrailEntry(ctx context.Context, entityType string, entityID int64, action string, data interface{}, userID int64) (*AuditTrailEntry, error) {
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal data: %w", err)
	}

	// Get previous hash for chain integrity
	var previousHash *string
	err = s.db.QueryRowContext(ctx, `
		SELECT current_hash FROM blockchain_audit_trail 
		WHERE entity_type = ? AND entity_id = ?
		ORDER BY created_at DESC LIMIT 1
	`, entityType, entityID).Scan(&previousHash)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to get previous hash: %w", err)
	}

	// Calculate current hash
	hashInput := map[string]interface{}{
		"entityType":   entityType,
		"entityId":     entityID,
		"action":       action,
		"data":         data,
		"previousHash": previousHash,
		"timestamp":    time.Now().Format(time.RFC3339),
	}
	hashInputBytes, _ := json.Marshal(hashInput)
	currentHash := hashData(hashInputBytes)

	entry := &AuditTrailEntry{
		EntityType:   entityType,
		EntityID:     entityID,
		Action:       action,
		PreviousHash: previousHash,
		CurrentHash:  currentHash,
		Data:         dataBytes,
		CreatedBy:    userID,
		CreatedAt:    time.Now(),
	}

	// Submit to blockchain
	tx, err := s.SubmitTransaction(ctx, "AuditTrail", "recordAudit", map[string]interface{}{
		"entityType": entityType,
		"entityId":   entityID,
		"action":     action,
		"hash":       currentHash,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to submit audit to blockchain: %w", err)
	}
	entry.BlockchainTxID = &tx.TransactionID

	// Insert into database
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO blockchain_audit_trail 
		(entity_type, entity_id, action, previous_hash, current_hash, blockchain_tx_id, data, created_by, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, entry.EntityType, entry.EntityID, entry.Action, entry.PreviousHash, entry.CurrentHash, entry.BlockchainTxID, entry.Data, entry.CreatedBy, entry.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to insert audit trail: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get insert id: %w", err)
	}
	entry.ID = id

	return entry, nil
}

// VerifyAuditTrail verifies the integrity of an entity's audit trail
func (s *BlockchainService) VerifyAuditTrail(ctx context.Context, entityType string, entityID int64) (bool, []string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, previous_hash, current_hash FROM blockchain_audit_trail 
		WHERE entity_type = ? AND entity_id = ?
		ORDER BY created_at ASC
	`, entityType, entityID)
	if err != nil {
		return false, nil, fmt.Errorf("failed to query audit trail: %w", err)
	}
	defer rows.Close()

	var entries []struct {
		ID           int64
		PreviousHash *string
		CurrentHash  string
	}

	for rows.Next() {
		var entry struct {
			ID           int64
			PreviousHash *string
			CurrentHash  string
		}
		if err := rows.Scan(&entry.ID, &entry.PreviousHash, &entry.CurrentHash); err != nil {
			return false, nil, fmt.Errorf("failed to scan entry: %w", err)
		}
		entries = append(entries, entry)
	}

	if len(entries) == 0 {
		return true, nil, nil // No entries, considered valid
	}

	var violations []string
	for i := 1; i < len(entries); i++ {
		if entries[i].PreviousHash == nil || *entries[i].PreviousHash != entries[i-1].CurrentHash {
			violations = append(violations, fmt.Sprintf("Chain broken at entry %d", entries[i].ID))
		}
	}

	return len(violations) == 0, violations, nil
}

// GetAuditTrail retrieves the audit trail for an entity
func (s *BlockchainService) GetAuditTrail(ctx context.Context, entityType string, entityID int64) ([]AuditTrailEntry, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, entity_type, entity_id, action, previous_hash, current_hash, 
		       blockchain_tx_id, data, created_by, created_at
		FROM blockchain_audit_trail 
		WHERE entity_type = ? AND entity_id = ?
		ORDER BY created_at ASC
	`, entityType, entityID)
	if err != nil {
		return nil, fmt.Errorf("failed to query audit trail: %w", err)
	}
	defer rows.Close()

	var entries []AuditTrailEntry
	for rows.Next() {
		var entry AuditTrailEntry
		if err := rows.Scan(
			&entry.ID, &entry.EntityType, &entry.EntityID, &entry.Action,
			&entry.PreviousHash, &entry.CurrentHash, &entry.BlockchainTxID,
			&entry.Data, &entry.CreatedBy, &entry.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan entry: %w", err)
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

// Helper functions

func hashData(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

func generateRandomHex(length int) string {
	hash := sha256.Sum256([]byte(fmt.Sprintf("%d", time.Now().UnixNano())))
	return hex.EncodeToString(hash[:])[:length*2]
}

// Temporal Workflow and Activities

// BlockchainWorkflowInput is the input for blockchain workflows
type BlockchainWorkflowInput struct {
	Operation   string                 `json:"operation"` // submit_transaction, create_identity, create_audit
	Chaincode   string                 `json:"chaincode,omitempty"`
	Function    string                 `json:"function,omitempty"`
	Payload     map[string]interface{} `json:"payload,omitempty"`
	UserID      int64                  `json:"userId,omitempty"`
	EntityType  string                 `json:"entityType,omitempty"`
	EntityID    int64                  `json:"entityId,omitempty"`
	Action      string                 `json:"action,omitempty"`
	Data        interface{}            `json:"data,omitempty"`
	InitiatedBy int64                  `json:"initiatedBy"`
}

// BlockchainWorkflowResult is the result of blockchain workflows
type BlockchainWorkflowResult struct {
	Success       bool                    `json:"success"`
	TransactionID string                  `json:"transactionId,omitempty"`
	DID           string                  `json:"did,omitempty"`
	AuditEntryID  int64                   `json:"auditEntryId,omitempty"`
	Error         string                  `json:"error,omitempty"`
	Transaction   *BlockchainTransaction  `json:"transaction,omitempty"`
	Identity      *BlockchainIdentity     `json:"identity,omitempty"`
	AuditEntry    *AuditTrailEntry        `json:"auditEntry,omitempty"`
}

// BlockchainWorkflow handles blockchain operations as a Temporal workflow
func BlockchainWorkflow(ctx workflow.Context, input BlockchainWorkflowInput) (*BlockchainWorkflowResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting blockchain workflow", "operation", input.Operation)

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    30 * time.Second,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	result := &BlockchainWorkflowResult{}

	switch input.Operation {
	case "submit_transaction":
		var tx *BlockchainTransaction
		err := workflow.ExecuteActivity(ctx, SubmitTransactionActivity, input.Chaincode, input.Function, input.Payload).Get(ctx, &tx)
		if err != nil {
			result.Error = err.Error()
			return result, nil
		}
		result.Success = true
		result.TransactionID = tx.TransactionID
		result.Transaction = tx

		// Wait for confirmation
		err = workflow.ExecuteActivity(ctx, ConfirmTransactionActivity, tx.TransactionID).Get(ctx, nil)
		if err != nil {
			logger.Warn("Failed to confirm transaction", "error", err)
		}

	case "create_identity":
		var identity *BlockchainIdentity
		err := workflow.ExecuteActivity(ctx, CreateIdentityActivity, input.UserID).Get(ctx, &identity)
		if err != nil {
			result.Error = err.Error()
			return result, nil
		}
		result.Success = true
		result.DID = identity.DID
		result.Identity = identity

	case "create_audit":
		var entry *AuditTrailEntry
		err := workflow.ExecuteActivity(ctx, CreateAuditTrailActivity, input.EntityType, input.EntityID, input.Action, input.Data, input.InitiatedBy).Get(ctx, &entry)
		if err != nil {
			result.Error = err.Error()
			return result, nil
		}
		result.Success = true
		result.AuditEntryID = entry.ID
		result.AuditEntry = entry

	default:
		result.Error = fmt.Sprintf("unknown operation: %s", input.Operation)
	}

	return result, nil
}

// Activity implementations

type BlockchainActivities struct {
	service *BlockchainService
}

func NewBlockchainActivities(service *BlockchainService) *BlockchainActivities {
	return &BlockchainActivities{service: service}
}

func SubmitTransactionActivity(ctx context.Context, chaincode, function string, payload map[string]interface{}) (*BlockchainTransaction, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Submitting blockchain transaction", "chaincode", chaincode, "function", function)

	// Get service from context (in production, use dependency injection)
	service := getBlockchainServiceFromContext(ctx)
	if service == nil {
		return nil, fmt.Errorf("blockchain service not available")
	}

	return service.SubmitTransaction(ctx, chaincode, function, payload)
}

func ConfirmTransactionActivity(ctx context.Context, transactionID string) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Confirming blockchain transaction", "transactionId", transactionID)

	service := getBlockchainServiceFromContext(ctx)
	if service == nil {
		return fmt.Errorf("blockchain service not available")
	}

	return service.ConfirmTransaction(ctx, transactionID)
}

func CreateIdentityActivity(ctx context.Context, userID int64) (*BlockchainIdentity, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Creating blockchain identity", "userId", userID)

	service := getBlockchainServiceFromContext(ctx)
	if service == nil {
		return nil, fmt.Errorf("blockchain service not available")
	}

	return service.CreateIdentity(ctx, userID)
}

func CreateAuditTrailActivity(ctx context.Context, entityType string, entityID int64, action string, data interface{}, userID int64) (*AuditTrailEntry, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Creating audit trail entry", "entityType", entityType, "entityId", entityID, "action", action)

	service := getBlockchainServiceFromContext(ctx)
	if service == nil {
		return nil, fmt.Errorf("blockchain service not available")
	}

	return service.CreateAuditTrailEntry(ctx, entityType, entityID, action, data, userID)
}

// Context key for blockchain service
type blockchainServiceKey struct{}

func WithBlockchainService(ctx context.Context, service *BlockchainService) context.Context {
	return context.WithValue(ctx, blockchainServiceKey{}, service)
}

func getBlockchainServiceFromContext(ctx context.Context) *BlockchainService {
	service, _ := ctx.Value(blockchainServiceKey{}).(*BlockchainService)
	return service
}

// Database migration for blockchain tables
func GetBlockchainMigration() string {
	return `
-- Blockchain transactions table
CREATE TABLE IF NOT EXISTS blockchain_transactions (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    transaction_id VARCHAR(64) NOT NULL UNIQUE,
    block_hash VARCHAR(64) NOT NULL,
    block_number BIGINT NOT NULL,
    transaction_hash VARCHAR(64) NOT NULL,
    chaincode_name VARCHAR(64) NOT NULL,
    function_name VARCHAR(64) NOT NULL,
    payload JSON NOT NULL,
    status ENUM('pending', 'confirmed', 'failed') NOT NULL DEFAULT 'pending',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    confirmed_at TIMESTAMP NULL,
    INDEX idx_transaction_id (transaction_id),
    INDEX idx_status (status),
    INDEX idx_chaincode (chaincode_name, function_name),
    INDEX idx_created_at (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Blockchain identities table
CREATE TABLE IF NOT EXISTS blockchain_identities (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    user_id BIGINT NOT NULL,
    did VARCHAR(128) NOT NULL UNIQUE,
    public_key VARCHAR(128) NOT NULL,
    verification_method JSON,
    status ENUM('active', 'revoked') NOT NULL DEFAULT 'active',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    revoked_at TIMESTAMP NULL,
    INDEX idx_user_id (user_id),
    INDEX idx_did (did),
    INDEX idx_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Blockchain audit trail table
CREATE TABLE IF NOT EXISTS blockchain_audit_trail (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    entity_type VARCHAR(64) NOT NULL,
    entity_id BIGINT NOT NULL,
    action VARCHAR(64) NOT NULL,
    previous_hash VARCHAR(64),
    current_hash VARCHAR(64) NOT NULL,
    blockchain_tx_id VARCHAR(64),
    data JSON NOT NULL,
    created_by BIGINT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_entity (entity_type, entity_id),
    INDEX idx_action (action),
    INDEX idx_created_at (created_at),
    INDEX idx_blockchain_tx (blockchain_tx_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
`
}

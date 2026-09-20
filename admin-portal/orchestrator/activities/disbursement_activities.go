package activities

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/admin-portal/orchestrator/middleware"
	"github.com/admin-portal/orchestrator/mojaloop"
	"github.com/admin-portal/orchestrator/repository"

	"go.temporal.io/sdk/activity"
)

// DisbursementActivities contains activities for the two-phase commit disbursement workflow
type DisbursementActivities struct {
	tbManager         *middleware.TigerBeetleManager
	fspiop            *mojaloop.FSPIOPClient
	callbackHandler   *mojaloop.CallbackHandler
	settlementManager *mojaloop.SettlementManager
	paymentRepo       *repository.PaymentRepository
}

// NewDisbursementActivities creates a new DisbursementActivities instance
func NewDisbursementActivities() *DisbursementActivities {
	return &DisbursementActivities{
		tbManager:         middleware.GetTigerBeetleManager(),
		fspiop:            mojaloop.GetFSPIOPClient(),
		callbackHandler:   mojaloop.GetCallbackHandler(),
		settlementManager: mojaloop.GetSettlementManager(),
		paymentRepo:       repository.GetPaymentRepository(),
	}
}

// amountToCents converts a float amount to cents with proper rounding
// This avoids floating point precision issues
func amountToCents(amount float64) int64 {
	return int64(math.Round(amount * 100))
}

// generateIdempotencyKey creates a deterministic idempotency key for an activity
func generateIdempotencyKey(activityType, workflowID, runID string, attempt int32) string {
	return fmt.Sprintf("%s:%s:%s:%d", activityType, workflowID, runID, attempt)
}

// ReserveFundsForDisbursementActivity creates a pending transfer to reserve funds (Phase 1)
// This activity is idempotent - retries will return the same pending transfer ID
func (a *DisbursementActivities) ReserveFundsForDisbursementActivity(ctx context.Context, programID, beneficiaryID string, amount, feeAmount float64, feeAccountID string) (string, error) {
	if a.tbManager == nil {
		return "", fmt.Errorf("TigerBeetle manager not initialized")
	}
	if a.paymentRepo == nil {
		return "", fmt.Errorf("payment repository not initialized")
	}

	// Get activity info for idempotency
	activityInfo := activity.GetInfo(ctx)
	idempotencyKey := generateIdempotencyKey("reserve_funds", activityInfo.WorkflowExecution.ID, activityInfo.WorkflowExecution.RunID, activityInfo.Attempt)

	// Check for existing idempotency record
	existing, alreadyExists, err := a.paymentRepo.GetOrCreateActivityIdempotency(ctx, idempotencyKey, "reserve_funds", activityInfo.WorkflowExecution.ID, activityInfo.WorkflowExecution.RunID, 24*time.Hour)
	if err != nil {
		return "", fmt.Errorf("failed to check idempotency: %w", err)
	}
	if alreadyExists && existing.Status == "completed" && existing.Result != nil {
		var result string
		if err := json.Unmarshal(existing.Result, &result); err == nil {
			return result, nil
		}
	}

	// Get account mappings from database - NEVER generate new IDs on parse failure
	programMapping, err := a.paymentRepo.GetAccountMapping(ctx, programID, "program")
	if err != nil {
		return "", fmt.Errorf("failed to get program account mapping: %w", err)
	}
	if programMapping == nil {
		return "", fmt.Errorf("program account mapping not found for ID: %s - account must be created first", programID)
	}

	beneficiaryMapping, err := a.paymentRepo.GetAccountMapping(ctx, beneficiaryID, "beneficiary")
	if err != nil {
		return "", fmt.Errorf("failed to get beneficiary account mapping: %w", err)
	}
	if beneficiaryMapping == nil {
		return "", fmt.Errorf("beneficiary account mapping not found for ID: %s - account must be created first", beneficiaryID)
	}

	// Parse TigerBeetle account IDs from mappings
	programAccountID, err := middleware.UUIDFromString(programMapping.TigerBeetleAccountID)
	if err != nil {
		return "", fmt.Errorf("invalid program TigerBeetle account ID in mapping: %w", err)
	}

	beneficiaryAccountID, err := middleware.UUIDFromString(beneficiaryMapping.TigerBeetleAccountID)
	if err != nil {
		return "", fmt.Errorf("invalid beneficiary TigerBeetle account ID in mapping: %w", err)
	}

	// Convert amount to cents with proper rounding
	amountCents := uint64(amountToCents(amount))

	// Create pending transfer to reserve funds
	pending, err := a.tbManager.ReserveFundsForDisbursement(ctx, programAccountID, beneficiaryAccountID, amountCents)
	if err != nil {
		_ = a.paymentRepo.UpdateActivityIdempotencyFailed(ctx, idempotencyKey, err.Error())
		return "", fmt.Errorf("failed to reserve funds: %w", err)
	}

	result := middleware.Uint128ToString(pending.PendingTransferID)

	// Mark activity as completed with result
	resultJSON, _ := json.Marshal(result)
	_ = a.paymentRepo.UpdateActivityIdempotencyCompleted(ctx, idempotencyKey, resultJSON)

	return result, nil
}

// CommitDisbursementActivity finalizes a pending transfer after successful external payment (Phase 2 success)
func (a *DisbursementActivities) CommitDisbursementActivity(ctx context.Context, pendingTransferID string, amount float64) (string, error) {
	if a.tbManager == nil {
		return "", fmt.Errorf("TigerBeetle manager not initialized")
	}

	pendingID, err := middleware.UUIDFromString(pendingTransferID)
	if err != nil {
		return "", fmt.Errorf("invalid pending transfer ID: %w", err)
	}

	amountCents := uint64(amount * 100)

	// Create a PendingDisbursement struct to commit
	pending := &middleware.PendingDisbursement{
		PendingTransferID: pendingID,
		Amount:            amountCents,
	}

	commitID, err := a.tbManager.CommitDisbursement(ctx, pending)
	if err != nil {
		return "", fmt.Errorf("failed to commit disbursement: %w", err)
	}

	return middleware.Uint128ToString(commitID), nil
}

// RollbackDisbursementActivity cancels a pending transfer after failed external payment (Phase 2 failure)
func (a *DisbursementActivities) RollbackDisbursementActivity(ctx context.Context, pendingTransferID string) error {
	if a.tbManager == nil {
		return fmt.Errorf("TigerBeetle manager not initialized")
	}

	pendingID, err := middleware.UUIDFromString(pendingTransferID)
	if err != nil {
		return fmt.Errorf("invalid pending transfer ID: %w", err)
	}

	pending := &middleware.PendingDisbursement{
		PendingTransferID: pendingID,
	}

	err = a.tbManager.RollbackDisbursement(ctx, pending)
	if err != nil {
		return fmt.Errorf("failed to rollback disbursement: %w", err)
	}

	return nil
}

// ExecuteMojaloopTransferActivity executes a transfer via Mojaloop FSPIOP (Quote -> Transfer -> Fulfilment)
// This activity is idempotent - retries will return the same transfer ID if already completed
func (a *DisbursementActivities) ExecuteMojaloopTransferActivity(ctx context.Context, beneficiaryID string, amount float64, currency, settlementCycleID string) (string, error) {
	if a.fspiop == nil {
		return "", fmt.Errorf("FSPIOP client not initialized")
	}
	if a.paymentRepo == nil {
		return "", fmt.Errorf("payment repository not initialized")
	}

	// Get activity info for idempotency
	activityInfo := activity.GetInfo(ctx)
	idempotencyKey := generateIdempotencyKey("mojaloop_transfer", activityInfo.WorkflowExecution.ID, activityInfo.WorkflowExecution.RunID, activityInfo.Attempt)

	// Check for existing idempotency record
	existing, alreadyExists, err := a.paymentRepo.GetOrCreateActivityIdempotency(ctx, idempotencyKey, "mojaloop_transfer", activityInfo.WorkflowExecution.ID, activityInfo.WorkflowExecution.RunID, 24*time.Hour)
	if err != nil {
		return "", fmt.Errorf("failed to check idempotency: %w", err)
	}
	if alreadyExists && existing.Status == "completed" && existing.Result != nil {
		var result string
		if err := json.Unmarshal(existing.Result, &result); err == nil {
			return result, nil
		}
	}

	// Step 1: Party lookup to resolve beneficiary to DFSP
	party, err := a.fspiop.PartyLookup(ctx, "MSISDN", beneficiaryID)
	if err != nil {
		return "", fmt.Errorf("party lookup failed: %w", err)
	}

	// If async, wait for callback
	if party == nil && a.callbackHandler != nil {
		partyCallback, err := a.callbackHandler.WaitForPartyCallback(ctx, "MSISDN", beneficiaryID)
		if err != nil {
			_ = a.paymentRepo.UpdateActivityIdempotencyFailed(ctx, idempotencyKey, "party callback timeout: "+err.Error())
			return "", fmt.Errorf("party callback timeout: %w", err)
		}
		if partyCallback.Error != nil {
			errMsg := fmt.Sprintf("party lookup error: %s", partyCallback.Error.ErrorDescription)
			_ = a.paymentRepo.UpdateActivityIdempotencyFailed(ctx, idempotencyKey, errMsg)
			return "", fmt.Errorf("%s", errMsg)
		}
		party = partyCallback.Party
	}

	if party == nil || party.FspId == "" {
		_ = a.paymentRepo.UpdateActivityIdempotencyFailed(ctx, idempotencyKey, "could not resolve beneficiary to DFSP")
		return "", fmt.Errorf("could not resolve beneficiary to DFSP")
	}

	// Step 2: Request quote
	// FIX: Get payer FSP ID from the FSPIOP client's configured DFSP ID, not from pending quote lookup
	payer := mojaloop.Party{
		PartyIdType:     "BUSINESS",
		PartyIdentifier: "SOCIAL_PROTECTION_PROGRAM",
		FspId:           a.fspiop.GetDFSPID(), // Use configured DFSP ID directly
	}

	payee := *party

	// Use proper decimal formatting for amount
	amountStr := fmt.Sprintf("%.2f", amount)
	money := mojaloop.Money{Currency: currency, Amount: amountStr}

	transactionType := mojaloop.TransactionType{
		Scenario:      "TRANSFER",
		Initiator:     "PAYER",
		InitiatorType: "BUSINESS",
	}

	quoteReq, err := a.fspiop.RequestQuote(ctx, payer, payee, money, transactionType)
	if err != nil {
		_ = a.paymentRepo.UpdateActivityIdempotencyFailed(ctx, idempotencyKey, "quote request failed: "+err.Error())
		return "", fmt.Errorf("quote request failed: %w", err)
	}

	// Wait for quote callback
	var quoteResp *mojaloop.QuoteResponse
	if a.callbackHandler != nil {
		quoteCallback, err := a.callbackHandler.WaitForQuoteCallback(ctx, quoteReq.QuoteId)
		if err != nil {
			_ = a.paymentRepo.UpdateActivityIdempotencyFailed(ctx, idempotencyKey, "quote callback timeout: "+err.Error())
			return "", fmt.Errorf("quote callback timeout: %w", err)
		}
		if quoteCallback.Error != nil {
			errMsg := fmt.Sprintf("quote error: %s", quoteCallback.Error.ErrorDescription)
			_ = a.paymentRepo.UpdateActivityIdempotencyFailed(ctx, idempotencyKey, errMsg)
			return "", fmt.Errorf("%s", errMsg)
		}
		quoteResp = quoteCallback.Response
	}

	if quoteResp == nil {
		_ = a.paymentRepo.UpdateActivityIdempotencyFailed(ctx, idempotencyKey, "no quote response received")
		return "", fmt.Errorf("no quote response received")
	}

	// Step 3: Initiate transfer
	transferReq, err := a.fspiop.InitiateTransfer(ctx, quoteResp, quoteReq)
	if err != nil {
		_ = a.paymentRepo.UpdateActivityIdempotencyFailed(ctx, idempotencyKey, "transfer initiation failed: "+err.Error())
		return "", fmt.Errorf("transfer initiation failed: %w", err)
	}

	// Wait for transfer callback (fulfilment)
	if a.callbackHandler != nil {
		transferCallback, err := a.callbackHandler.WaitForTransferCallback(ctx, transferReq.TransferId)
		if err != nil {
			_ = a.paymentRepo.UpdateActivityIdempotencyFailed(ctx, idempotencyKey, "transfer callback timeout: "+err.Error())
			return "", fmt.Errorf("transfer callback timeout: %w", err)
		}
		if transferCallback.Error != nil {
			errMsg := fmt.Sprintf("transfer error: %s", transferCallback.Error.ErrorDescription)
			_ = a.paymentRepo.UpdateActivityIdempotencyFailed(ctx, idempotencyKey, errMsg)
			return "", fmt.Errorf("%s", errMsg)
		}
		if transferCallback.Response.TransferState != mojaloop.TransferStateCommitted {
			errMsg := fmt.Sprintf("transfer not committed: %s", transferCallback.Response.TransferState)
			_ = a.paymentRepo.UpdateActivityIdempotencyFailed(ctx, idempotencyKey, errMsg)
			return "", fmt.Errorf("%s", errMsg)
		}
	}

	// Record transfer in settlement cycle
	if a.settlementManager != nil && settlementCycleID != "" {
		transferRecord := &mojaloop.TransferRecord{
			TransferId:    transferReq.TransferId,
			PayerFsp:      transferReq.PayerFsp,
			PayeeFsp:      transferReq.PayeeFsp,
			Amount:        transferReq.Amount,
			Currency:      currency,
			TransferState: mojaloop.TransferStateCommitted,
		}
		_ = a.settlementManager.RecordTransfer(ctx, settlementCycleID, transferRecord)
	}

	result := transferReq.TransferId

	// Mark activity as completed with result
	resultJSON, _ := json.Marshal(result)
	_ = a.paymentRepo.UpdateActivityIdempotencyCompleted(ctx, idempotencyKey, resultJSON)

	return result, nil
}

// CreateSettlementCycleActivity creates a new settlement cycle for tracking
func (a *DisbursementActivities) CreateSettlementCycleActivity(ctx context.Context, currency string) (string, error) {
	if a.settlementManager == nil {
		return "", fmt.Errorf("settlement manager not initialized")
	}

	cycle, err := a.settlementManager.CreateSettlementCycle(ctx, currency)
	if err != nil {
		return "", fmt.Errorf("failed to create settlement cycle: %w", err)
	}

	return cycle.CycleId, nil
}

// CloseSettlementCycleActivity closes a settlement cycle
func (a *DisbursementActivities) CloseSettlementCycleActivity(ctx context.Context, cycleID string) error {
	if a.settlementManager == nil {
		return fmt.Errorf("settlement manager not initialized")
	}

	err := a.settlementManager.CloseSettlementCycle(ctx, cycleID)
	if err != nil {
		return fmt.Errorf("failed to close settlement cycle: %w", err)
	}

	return nil
}

// GenerateSettlementReportActivity generates a settlement report for Treasury/CBN
func (a *DisbursementActivities) GenerateSettlementReportActivity(ctx context.Context, cycleID string) (*mojaloop.SettlementReport, error) {
	if a.settlementManager == nil {
		return nil, fmt.Errorf("settlement manager not initialized")
	}

	report, err := a.settlementManager.GenerateSettlementReport(ctx, cycleID)
	if err != nil {
		return nil, fmt.Errorf("failed to generate settlement report: %w", err)
	}

	return report, nil
}

// DisburseFundsWithFeeActivity creates an atomic disbursement with fee using linked transfers
// This activity requires all account mappings to exist - it will NOT create new accounts
func (a *DisbursementActivities) DisburseFundsWithFeeActivity(ctx context.Context, programID, beneficiaryID, feeAccountID string, amount, feeAmount float64) ([]string, error) {
	if a.tbManager == nil {
		return nil, fmt.Errorf("TigerBeetle manager not initialized")
	}
	if a.paymentRepo == nil {
		return nil, fmt.Errorf("payment repository not initialized")
	}

	// Get account mappings from database - NEVER generate new IDs on parse failure
	programMapping, err := a.paymentRepo.GetAccountMapping(ctx, programID, "program")
	if err != nil {
		return nil, fmt.Errorf("failed to get program account mapping: %w", err)
	}
	if programMapping == nil {
		return nil, fmt.Errorf("program account mapping not found for ID: %s", programID)
	}

	beneficiaryMapping, err := a.paymentRepo.GetAccountMapping(ctx, beneficiaryID, "beneficiary")
	if err != nil {
		return nil, fmt.Errorf("failed to get beneficiary account mapping: %w", err)
	}
	if beneficiaryMapping == nil {
		return nil, fmt.Errorf("beneficiary account mapping not found for ID: %s", beneficiaryID)
	}

	feeMapping, err := a.paymentRepo.GetAccountMapping(ctx, feeAccountID, "fee")
	if err != nil {
		return nil, fmt.Errorf("failed to get fee account mapping: %w", err)
	}
	if feeMapping == nil {
		return nil, fmt.Errorf("fee account mapping not found for ID: %s", feeAccountID)
	}

	// Parse TigerBeetle account IDs from mappings
	programAccountID, err := middleware.UUIDFromString(programMapping.TigerBeetleAccountID)
	if err != nil {
		return nil, fmt.Errorf("invalid program TigerBeetle account ID: %w", err)
	}

	beneficiaryAccountID, err := middleware.UUIDFromString(beneficiaryMapping.TigerBeetleAccountID)
	if err != nil {
		return nil, fmt.Errorf("invalid beneficiary TigerBeetle account ID: %w", err)
	}

	feeAccID, err := middleware.UUIDFromString(feeMapping.TigerBeetleAccountID)
	if err != nil {
		return nil, fmt.Errorf("invalid fee TigerBeetle account ID: %w", err)
	}

	// Convert amounts to cents with proper rounding
	amountCents := uint64(amountToCents(amount))
	feeCents := uint64(amountToCents(feeAmount))

	result, err := a.tbManager.DisburseFundsWithFee(ctx, programAccountID, beneficiaryAccountID, feeAccID, amountCents, feeCents)
	if err != nil {
		return nil, fmt.Errorf("failed to disburse funds with fee: %w", err)
	}

	transferIDs := make([]string, len(result.TransferIDs))
	for i, id := range result.TransferIDs {
		transferIDs[i] = middleware.Uint128ToString(id)
	}

	return transferIDs, nil
}

// BatchDisbursementActivity processes multiple disbursements in a single batch
// This activity requires all account mappings to exist - it will NOT create new accounts
func (a *DisbursementActivities) BatchDisbursementActivity(ctx context.Context, programID string, beneficiaryAmounts map[string]float64) (*middleware.BatchDisbursementResult, error) {
	if a.tbManager == nil {
		return nil, fmt.Errorf("TigerBeetle manager not initialized")
	}
	if a.paymentRepo == nil {
		return nil, fmt.Errorf("payment repository not initialized")
	}

	// Get program account mapping from database - NEVER generate new IDs on parse failure
	programMapping, err := a.paymentRepo.GetAccountMapping(ctx, programID, "program")
	if err != nil {
		return nil, fmt.Errorf("failed to get program account mapping: %w", err)
	}
	if programMapping == nil {
		return nil, fmt.Errorf("program account mapping not found for ID: %s", programID)
	}

	programAccountID, err := middleware.UUIDFromString(programMapping.TigerBeetleAccountID)
	if err != nil {
		return nil, fmt.Errorf("invalid program TigerBeetle account ID: %w", err)
	}

	// Build batch items with validated account mappings
	items := make([]middleware.BatchDisbursementItem, 0, len(beneficiaryAmounts))
	for beneficiaryID, amount := range beneficiaryAmounts {
		// Get beneficiary account mapping from database
		beneficiaryMapping, err := a.paymentRepo.GetAccountMapping(ctx, beneficiaryID, "beneficiary")
		if err != nil {
			return nil, fmt.Errorf("failed to get beneficiary account mapping for %s: %w", beneficiaryID, err)
		}
		if beneficiaryMapping == nil {
			return nil, fmt.Errorf("beneficiary account mapping not found for ID: %s", beneficiaryID)
		}

		beneficiaryAccountID, err := middleware.UUIDFromString(beneficiaryMapping.TigerBeetleAccountID)
		if err != nil {
			return nil, fmt.Errorf("invalid beneficiary TigerBeetle account ID for %s: %w", beneficiaryID, err)
		}

		items = append(items, middleware.BatchDisbursementItem{
			BeneficiaryAccountID: beneficiaryAccountID,
			AmountCents:          uint64(amountToCents(amount)),
			Reference:            beneficiaryID,
		})
	}

	result, err := a.tbManager.DisburseFundsBatch(ctx, programAccountID, items)
	if err != nil {
		return nil, fmt.Errorf("batch disbursement failed: %w", err)
	}

	return result, nil
}

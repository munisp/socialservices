package middleware

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"log"
	"sync"
	"time"

	tb "github.com/tigerbeetle/tigerbeetle-go"
	tb_types "github.com/tigerbeetle/tigerbeetle-go/pkg/types"
)

// TigerBeetleClient wraps the TigerBeetle client for financial operations
type TigerBeetleClient struct {
	client tb.Client
	mu     sync.RWMutex
}

// NewTigerBeetleClient creates a new TigerBeetle client
func NewTigerBeetleClient(addresses []string) (*TigerBeetleClient, error) {
	client, err := tb.NewClient(tb_types.Uint128{}, addresses, 32)
	if err != nil {
		return nil, fmt.Errorf("failed to create TigerBeetle client: %w", err)
	}

	return &TigerBeetleClient{
		client: client,
	}, nil
}

// Close closes the TigerBeetle client connection
func (t *TigerBeetleClient) Close() {
	t.client.Close()
}

// GenerateUUID128 generates a cryptographically secure UUID128 for TigerBeetle
func GenerateUUID128() tb_types.Uint128 {
	var id tb_types.Uint128
	_, err := rand.Read(id[:])
	if err != nil {
		// Fallback to time-based if crypto/rand fails
		binary.LittleEndian.PutUint64(id[:8], uint64(time.Now().UnixNano()))
		binary.LittleEndian.PutUint64(id[8:], uint64(time.Now().UnixNano()>>32))
	}
	return id
}

// UUIDFromString converts a UUID string to TigerBeetle Uint128
func UUIDFromString(uuidStr string) (tb_types.Uint128, error) {
	var id tb_types.Uint128
	clean := ""
	for _, c := range uuidStr {
		if c != '-' {
			clean += string(c)
		}
	}
	if len(clean) != 32 {
		return id, fmt.Errorf("invalid UUID length: expected 32 hex chars, got %d", len(clean))
	}
	for i := 0; i < 16; i++ {
		var b byte
		_, err := fmt.Sscanf(clean[i*2:i*2+2], "%02x", &b)
		if err != nil {
			return id, fmt.Errorf("invalid hex at position %d: %w", i*2, err)
		}
		id[i] = b
	}
	return id, nil
}

// Uint128ToString converts TigerBeetle Uint128 to hex string
func Uint128ToString(id tb_types.Uint128) string {
	return fmt.Sprintf("%032x", id.Bytes())
}

// uint128ToUint64 converts a TigerBeetle amount and rejects amounts this legacy API cannot represent.
func uint128ToUint64(value tb_types.Uint128) (uint64, error) {
	bytes := value.Bytes()
	for _, byteValue := range bytes[8:] {
		if byteValue != 0 {
			return 0, fmt.Errorf("TigerBeetle amount exceeds uint64")
		}
	}
	return binary.LittleEndian.Uint64(bytes[:8]), nil
}

// TransferFlags for TigerBeetle transfer operations
const (
	TransferFlagNone            uint16 = 0
	TransferFlagLinked          uint16 = 1 << 0
	TransferFlagPending         uint16 = 1 << 1
	TransferFlagPostPending     uint16 = 1 << 2
	TransferFlagVoidPending     uint16 = 1 << 3
	TransferFlagBalancingDebit  uint16 = 1 << 4
	TransferFlagBalancingCredit uint16 = 1 << 5
)

// AccountFlags for TigerBeetle account operations
const (
	AccountFlagNone          uint16 = 0
	AccountFlagLinked        uint16 = 1 << 0
	AccountFlagDebitsExceed  uint16 = 1 << 1
	AccountFlagCreditsExceed uint16 = 1 << 2
	AccountFlagClosed        uint16 = 1 << 3
)

// CreateAccount creates a new account in TigerBeetle
func (t *TigerBeetleClient) CreateAccount(accountID tb_types.Uint128, ledger uint32, code uint16) error {
	return t.CreateAccountWithFlags(accountID, ledger, code, AccountFlagNone)
}

// CreateAccountWithFlags creates a new account with specific flags
func (t *TigerBeetleClient) CreateAccountWithFlags(accountID tb_types.Uint128, ledger uint32, code uint16, flags uint16) error {
	accounts := []tb_types.Account{{ID: accountID, Ledger: ledger, Code: code, Flags: flags}}
	results, err := t.client.CreateAccounts(accounts)
	if err != nil {
		return fmt.Errorf("failed to create account: %w", err)
	}
	if len(results) > 0 {
		return fmt.Errorf("account creation failed with result: %v", results[0])
	}
	return nil
}

// CreateAccountsBatch creates multiple accounts in a single batch
func (t *TigerBeetleClient) CreateAccountsBatch(accounts []tb_types.Account) ([]tb_types.AccountEventResult, error) {
	results, err := t.client.CreateAccounts(accounts)
	if err != nil {
		return nil, fmt.Errorf("failed to create accounts batch: %w", err)
	}
	return results, nil
}

// CreateTransfer creates a transfer between two accounts
func (t *TigerBeetleClient) CreateTransfer(transferID tb_types.Uint128, debitAccountID tb_types.Uint128, creditAccountID tb_types.Uint128, amount uint64, ledger uint32, code uint16) error {
	return t.CreateTransferWithFlags(transferID, debitAccountID, creditAccountID, amount, ledger, code, TransferFlagNone, tb_types.Uint128{})
}

// CreateTransferWithFlags creates a transfer with specific flags
func (t *TigerBeetleClient) CreateTransferWithFlags(transferID tb_types.Uint128, debitAccountID tb_types.Uint128, creditAccountID tb_types.Uint128, amount uint64, ledger uint32, code uint16, flags uint16, pendingID tb_types.Uint128) error {
	transfer := tb_types.Transfer{
		ID: transferID, DebitAccountID: debitAccountID, CreditAccountID: creditAccountID,
		Amount: tb_types.ToUint128(amount), Ledger: ledger, Code: code, Flags: flags, Timestamp: 0,
	}
	if flags&TransferFlagPostPending != 0 || flags&TransferFlagVoidPending != 0 {
		transfer.PendingID = pendingID
	}
	results, err := t.client.CreateTransfers([]tb_types.Transfer{transfer})
	if err != nil {
		return fmt.Errorf("failed to create transfer: %w", err)
	}
	if len(results) > 0 {
		return fmt.Errorf("transfer creation failed with result: %v", results[0])
	}
	return nil
}

// CreatePendingTransfer creates a pending (two-phase) transfer that reserves funds
func (t *TigerBeetleClient) CreatePendingTransfer(transferID tb_types.Uint128, debitAccountID tb_types.Uint128, creditAccountID tb_types.Uint128, amount uint64, ledger uint32, code uint16, timeout uint32) error {
	transfer := tb_types.Transfer{
		ID: transferID, DebitAccountID: debitAccountID, CreditAccountID: creditAccountID,
		Amount: tb_types.ToUint128(amount), Ledger: ledger, Code: code, Flags: TransferFlagPending, Timeout: timeout, Timestamp: 0,
	}
	results, err := t.client.CreateTransfers([]tb_types.Transfer{transfer})
	if err != nil {
		return fmt.Errorf("failed to create pending transfer: %w", err)
	}
	if len(results) > 0 {
		return fmt.Errorf("pending transfer creation failed with result: %v", results[0])
	}
	return nil
}

// PostPendingTransfer finalizes a pending transfer (commits the reservation)
func (t *TigerBeetleClient) PostPendingTransfer(postTransferID tb_types.Uint128, pendingTransferID tb_types.Uint128, amount uint64) error {
	transfer := tb_types.Transfer{ID: postTransferID, PendingID: pendingTransferID, Amount: tb_types.ToUint128(amount), Flags: TransferFlagPostPending, Timestamp: 0}
	results, err := t.client.CreateTransfers([]tb_types.Transfer{transfer})
	if err != nil {
		return fmt.Errorf("failed to post pending transfer: %w", err)
	}
	if len(results) > 0 {
		return fmt.Errorf("post pending transfer failed with result: %v", results[0])
	}
	return nil
}

// VoidPendingTransfer cancels a pending transfer (releases the reservation)
func (t *TigerBeetleClient) VoidPendingTransfer(voidTransferID tb_types.Uint128, pendingTransferID tb_types.Uint128) error {
	transfer := tb_types.Transfer{ID: voidTransferID, PendingID: pendingTransferID, Flags: TransferFlagVoidPending, Timestamp: 0}
	results, err := t.client.CreateTransfers([]tb_types.Transfer{transfer})
	if err != nil {
		return fmt.Errorf("failed to void pending transfer: %w", err)
	}
	if len(results) > 0 {
		return fmt.Errorf("void pending transfer failed with result: %v", results[0])
	}
	return nil
}

// LinkedTransfer represents a transfer in a linked chain
type LinkedTransfer struct {
	DebitAccountID  tb_types.Uint128
	CreditAccountID tb_types.Uint128
	Amount          uint64
	Code            uint16
	UserData128     tb_types.Uint128
}

// CreateLinkedTransfers creates multiple transfers atomically (all succeed or all fail)
func (t *TigerBeetleClient) CreateLinkedTransfers(ledger uint32, transfers []LinkedTransfer) ([]tb_types.Uint128, error) {
	if len(transfers) == 0 {
		return nil, fmt.Errorf("no transfers provided")
	}
	tbTransfers := make([]tb_types.Transfer, len(transfers))
	transferIDs := make([]tb_types.Uint128, len(transfers))
	for i, lt := range transfers {
		transferID := GenerateUUID128()
		transferIDs[i] = transferID
		flags := uint16(0)
		if i < len(transfers)-1 {
			flags = TransferFlagLinked
		}
		tbTransfers[i] = tb_types.Transfer{
			ID: transferID, DebitAccountID: lt.DebitAccountID, CreditAccountID: lt.CreditAccountID,
			Amount: tb_types.ToUint128(lt.Amount), Ledger: ledger, Code: lt.Code, Flags: flags, UserData128: lt.UserData128, Timestamp: 0,
		}
	}
	results, err := t.client.CreateTransfers(tbTransfers)
	if err != nil {
		return nil, fmt.Errorf("failed to create linked transfers: %w", err)
	}
	if len(results) > 0 {
		return nil, fmt.Errorf("linked transfers creation failed with result: %v", results[0])
	}
	return transferIDs, nil
}

// CreateTransfersBatch creates multiple transfers in a single batch (not linked)
func (t *TigerBeetleClient) CreateTransfersBatch(transfers []tb_types.Transfer) ([]tb_types.TransferEventResult, error) {
	results, err := t.client.CreateTransfers(transfers)
	if err != nil {
		return nil, fmt.Errorf("failed to create transfers batch: %w", err)
	}
	return results, nil
}

// GetAccountBalance retrieves the balance of an account including pending amounts
func (t *TigerBeetleClient) GetAccountBalance(accountID tb_types.Uint128) (debitsPosted, creditsPosted, debitsPending, creditsPending uint64, err error) {
	accounts, err := t.client.LookupAccounts([]tb_types.Uint128{accountID})
	if err != nil {
		return 0, 0, 0, 0, fmt.Errorf("failed to lookup account: %w", err)
	}
	if len(accounts) == 0 {
		return 0, 0, 0, 0, fmt.Errorf("account not found")
	}
	debitsPosted, err = uint128ToUint64(accounts[0].DebitsPosted)
	if err != nil {
		return 0, 0, 0, 0, err
	}
	creditsPosted, err = uint128ToUint64(accounts[0].CreditsPosted)
	if err != nil {
		return 0, 0, 0, 0, err
	}
	debitsPending, err = uint128ToUint64(accounts[0].DebitsPending)
	if err != nil {
		return 0, 0, 0, 0, err
	}
	creditsPending, err = uint128ToUint64(accounts[0].CreditsPending)
	if err != nil {
		return 0, 0, 0, 0, err
	}
	return debitsPosted, creditsPosted, debitsPending, creditsPending, nil
}

// LookupAccounts retrieves multiple accounts
func (t *TigerBeetleClient) LookupAccounts(accountIDs []tb_types.Uint128) ([]tb_types.Account, error) {
	accounts, err := t.client.LookupAccounts(accountIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to lookup accounts: %w", err)
	}
	return accounts, nil
}

// LookupTransfers retrieves multiple transfers
func (t *TigerBeetleClient) LookupTransfers(transferIDs []tb_types.Uint128) ([]tb_types.Transfer, error) {
	transfers, err := t.client.LookupTransfers(transferIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to lookup transfers: %w", err)
	}
	return transfers, nil
}

// TigerBeetleManager provides high-level operations for the social protection platform
type TigerBeetleManager struct {
	client *TigerBeetleClient
	mu     sync.RWMutex
}

// NewTigerBeetleManager creates a new manager
func NewTigerBeetleManager(addresses []string) (*TigerBeetleManager, error) {
	client, err := NewTigerBeetleClient(addresses)
	if err != nil {
		return nil, err
	}
	return &TigerBeetleManager{client: client}, nil
}

// Close closes the manager
func (m *TigerBeetleManager) Close() {
	m.client.Close()
}

// Ledger codes
const (
	LedgerSocialProtection uint32 = 1
	LedgerFees             uint32 = 2
	LedgerSettlement       uint32 = 3
)

// Account codes
const (
	AccountCodeBeneficiary uint16 = 1
	AccountCodeProgram     uint16 = 2
	AccountCodeGovernment  uint16 = 3
	AccountCodeSettlement  uint16 = 4
	AccountCodeFees        uint16 = 5
	AccountCodeSuspense    uint16 = 6
	AccountCodeFloat       uint16 = 7
	AccountCodeTreasury    uint16 = 8
	AccountCodeEscrow      uint16 = 9
)

// Transfer codes
const (
	TransferCodeDisbursement uint16 = 1
	TransferCodeRefund       uint16 = 2
	TransferCodeSettlement   uint16 = 3
	TransferCodeTransfer     uint16 = 4
	TransferCodeFee          uint16 = 5
	TransferCodeReversal     uint16 = 6
	TransferCodeAdjustment   uint16 = 7
	TransferCodeFunding      uint16 = 8
	TransferCodeWithdrawal   uint16 = 9
	TransferCodeCommitment   uint16 = 12
	TransferCodeEncumbrance  uint16 = 13
	TransferCodeRelease      uint16 = 14
)

// PendingTransferTimeout is the default timeout for pending transfers (5 minutes)
const PendingTransferTimeout uint32 = 300

// PendingDisbursement represents a pending disbursement that can be posted or voided
type PendingDisbursement struct {
	PendingTransferID    tb_types.Uint128
	ProgramAccountID     tb_types.Uint128
	BeneficiaryAccountID tb_types.Uint128
	Amount               uint64
	CreatedAt            time.Time
}

// CreateBeneficiaryAccount creates an account for a beneficiary
func (m *TigerBeetleManager) CreateBeneficiaryAccount(beneficiaryID string) (tb_types.Uint128, error) {
	accountID := GenerateUUID128()
	err := m.client.CreateAccountWithFlags(accountID, LedgerSocialProtection, AccountCodeBeneficiary, AccountFlagCreditsExceed)
	if err != nil {
		return tb_types.Uint128{}, err
	}
	return accountID, nil
}

// CreateProgramAccount creates an account for a program
func (m *TigerBeetleManager) CreateProgramAccount(programID string) (tb_types.Uint128, error) {
	accountID := GenerateUUID128()
	err := m.client.CreateAccountWithFlags(accountID, LedgerSocialProtection, AccountCodeProgram, AccountFlagDebitsExceed)
	if err != nil {
		return tb_types.Uint128{}, err
	}
	return accountID, nil
}

// CreateTreasuryAccount creates a treasury account for government funds
func (m *TigerBeetleManager) CreateTreasuryAccount() (tb_types.Uint128, error) {
	accountID := GenerateUUID128()
	err := m.client.CreateAccountWithFlags(accountID, LedgerSocialProtection, AccountCodeTreasury, AccountFlagDebitsExceed)
	if err != nil {
		return tb_types.Uint128{}, err
	}
	return accountID, nil
}

// CreateFeeAccount creates a fee collection account
func (m *TigerBeetleManager) CreateFeeAccount() (tb_types.Uint128, error) {
	accountID := GenerateUUID128()
	err := m.client.CreateAccountWithFlags(accountID, LedgerFees, AccountCodeFees, AccountFlagCreditsExceed)
	if err != nil {
		return tb_types.Uint128{}, err
	}
	return accountID, nil
}

// CreateSuspenseAccount creates a suspense account for pending transactions
func (m *TigerBeetleManager) CreateSuspenseAccount() (tb_types.Uint128, error) {
	accountID := GenerateUUID128()
	err := m.client.CreateAccount(accountID, LedgerSocialProtection, AccountCodeSuspense)
	if err != nil {
		return tb_types.Uint128{}, err
	}
	return accountID, nil
}

// CreateEscrowAccount creates an escrow account for held funds
func (m *TigerBeetleManager) CreateEscrowAccount() (tb_types.Uint128, error) {
	accountID := GenerateUUID128()
	err := m.client.CreateAccount(accountID, LedgerSocialProtection, AccountCodeEscrow)
	if err != nil {
		return tb_types.Uint128{}, err
	}
	return accountID, nil
}

// ReserveFundsForDisbursement creates a pending transfer to reserve funds before external payment
// This is the first phase of a two-phase commit
func (m *TigerBeetleManager) ReserveFundsForDisbursement(ctx context.Context, programAccountID, beneficiaryAccountID tb_types.Uint128, amountCents uint64) (*PendingDisbursement, error) {
	pendingTransferID := GenerateUUID128()
	err := m.client.CreatePendingTransfer(pendingTransferID, programAccountID, beneficiaryAccountID, amountCents, LedgerSocialProtection, TransferCodeDisbursement, PendingTransferTimeout)
	if err != nil {
		return nil, fmt.Errorf("failed to reserve funds: %w", err)
	}
	return &PendingDisbursement{
		PendingTransferID: pendingTransferID, ProgramAccountID: programAccountID,
		BeneficiaryAccountID: beneficiaryAccountID, Amount: amountCents, CreatedAt: time.Now(),
	}, nil
}

// CommitDisbursement finalizes a pending disbursement after successful external payment
func (m *TigerBeetleManager) CommitDisbursement(ctx context.Context, pending *PendingDisbursement) (tb_types.Uint128, error) {
	postTransferID := GenerateUUID128()
	err := m.client.PostPendingTransfer(postTransferID, pending.PendingTransferID, pending.Amount)
	if err != nil {
		return tb_types.Uint128{}, fmt.Errorf("failed to commit disbursement: %w", err)
	}
	return postTransferID, nil
}

// RollbackDisbursement cancels a pending disbursement after failed external payment
func (m *TigerBeetleManager) RollbackDisbursement(ctx context.Context, pending *PendingDisbursement) error {
	voidTransferID := GenerateUUID128()
	err := m.client.VoidPendingTransfer(voidTransferID, pending.PendingTransferID)
	if err != nil {
		return fmt.Errorf("failed to rollback disbursement: %w", err)
	}
	return nil
}

// DisbursementWithFee represents a disbursement with associated fee
type DisbursementWithFee struct {
	TransferIDs          []tb_types.Uint128
	ProgramAccountID     tb_types.Uint128
	BeneficiaryAccountID tb_types.Uint128
	FeeAccountID         tb_types.Uint128
	DisbursementAmount   uint64
	FeeAmount            uint64
}

// DisburseFundsWithFee creates an atomic multi-leg transfer: program -> beneficiary + program -> fee account
func (m *TigerBeetleManager) DisburseFundsWithFee(ctx context.Context, programAccountID, beneficiaryAccountID, feeAccountID tb_types.Uint128, disbursementAmountCents, feeAmountCents uint64) (*DisbursementWithFee, error) {
	linkedTransfers := []LinkedTransfer{
		{DebitAccountID: programAccountID, CreditAccountID: beneficiaryAccountID, Amount: disbursementAmountCents, Code: TransferCodeDisbursement},
		{DebitAccountID: programAccountID, CreditAccountID: feeAccountID, Amount: feeAmountCents, Code: TransferCodeFee},
	}
	transferIDs, err := m.client.CreateLinkedTransfers(LedgerSocialProtection, linkedTransfers)
	if err != nil {
		return nil, fmt.Errorf("failed to create disbursement with fee: %w", err)
	}
	return &DisbursementWithFee{
		TransferIDs: transferIDs, ProgramAccountID: programAccountID, BeneficiaryAccountID: beneficiaryAccountID,
		FeeAccountID: feeAccountID, DisbursementAmount: disbursementAmountCents, FeeAmount: feeAmountCents,
	}, nil
}

// BatchDisbursementItem represents a single disbursement in a batch
type BatchDisbursementItem struct {
	BeneficiaryAccountID tb_types.Uint128
	AmountCents          uint64
	Reference            string
}

// BatchDisbursementResult represents the result of a batch disbursement
type BatchDisbursementResult struct {
	TotalCount   int
	SuccessCount int
	FailureCount int
	TransferIDs  []tb_types.Uint128
	FailedItems  []int
	TotalAmount  uint64
}

// DisburseFundsBatch creates multiple disbursements in a single batch for high throughput
func (m *TigerBeetleManager) DisburseFundsBatch(ctx context.Context, programAccountID tb_types.Uint128, items []BatchDisbursementItem) (*BatchDisbursementResult, error) {
	if len(items) == 0 {
		return &BatchDisbursementResult{}, nil
	}
	transfers := make([]tb_types.Transfer, len(items))
	transferIDs := make([]tb_types.Uint128, len(items))
	var totalAmount uint64
	for i, item := range items {
		transferID := GenerateUUID128()
		transferIDs[i] = transferID
		totalAmount += item.AmountCents
		transfers[i] = tb_types.Transfer{
			ID: transferID, DebitAccountID: programAccountID, CreditAccountID: item.BeneficiaryAccountID,
			Amount: tb_types.ToUint128(item.AmountCents), Ledger: LedgerSocialProtection, Code: TransferCodeDisbursement, Flags: TransferFlagNone, Timestamp: 0,
		}
	}
	results, err := m.client.CreateTransfersBatch(transfers)
	if err != nil {
		return nil, fmt.Errorf("failed to create batch disbursements: %w", err)
	}
	result := &BatchDisbursementResult{TotalCount: len(items), TransferIDs: transferIDs, TotalAmount: totalAmount}
	failedIndices := make(map[int]bool)
	for _, r := range results {
		failedIndices[int(r.Index)] = true
	}
	for i := range items {
		if failedIndices[i] {
			result.FailureCount++
			result.FailedItems = append(result.FailedItems, i)
		} else {
			result.SuccessCount++
		}
	}
	return result, nil
}

// DisburseFunds transfers funds from program to beneficiary (legacy single-phase)
func (m *TigerBeetleManager) DisburseFunds(ctx context.Context, programID string, beneficiaryID string, amount float64) (string, error) {
	transferID := GenerateUUID128()
	programAccountID := GenerateUUID128()
	beneficiaryAccountID := GenerateUUID128()
	amountCents := uint64(amount * 100)
	err := m.client.CreateTransfer(transferID, programAccountID, beneficiaryAccountID, amountCents, LedgerSocialProtection, TransferCodeDisbursement)
	if err != nil {
		return "", err
	}
	return Uint128ToString(transferID), nil
}

// GetBeneficiaryBalance retrieves beneficiary's balance including pending amounts
func (m *TigerBeetleManager) GetBeneficiaryBalance(beneficiaryAccountID tb_types.Uint128) (available float64, pending float64, err error) {
	debitsPosted, creditsPosted, debitsPending, creditsPending, err := m.client.GetAccountBalance(beneficiaryAccountID)
	if err != nil {
		return 0, 0, err
	}
	availableCents := int64(creditsPosted) - int64(debitsPosted)
	pendingCents := int64(creditsPending) - int64(debitsPending)
	return float64(availableCents) / 100.0, float64(pendingCents) / 100.0, nil
}

// GetProgramBalance retrieves program's balance including pending amounts
func (m *TigerBeetleManager) GetProgramBalance(programAccountID tb_types.Uint128) (available float64, reserved float64, err error) {
	debitsPosted, creditsPosted, debitsPending, creditsPending, err := m.client.GetAccountBalance(programAccountID)
	if err != nil {
		return 0, 0, err
	}
	availableCents := int64(creditsPosted) - int64(debitsPosted)
	reservedCents := int64(debitsPending) - int64(creditsPending)
	return float64(availableCents) / 100.0, float64(reservedCents) / 100.0, nil
}

// AccountMapping stores the mapping between external IDs and TigerBeetle account IDs
type AccountMapping struct {
	ExternalID    string
	TigerBeetleID tb_types.Uint128
	AccountType   string
	CreatedAt     time.Time
}

// Global TigerBeetle manager instance
var globalTBManager *TigerBeetleManager
var tbManagerMu sync.RWMutex

// InitTigerBeetle initializes the global TigerBeetle manager
func InitTigerBeetle(addresses []string) error {
	tbManagerMu.Lock()
	defer tbManagerMu.Unlock()
	manager, err := NewTigerBeetleManager(addresses)
	if err != nil {
		return err
	}
	globalTBManager = manager
	log.Println("TigerBeetle client initialized with two-phase commit support")
	return nil
}

// GetTigerBeetleManager returns the global TigerBeetle manager
func GetTigerBeetleManager() *TigerBeetleManager {
	tbManagerMu.RLock()
	defer tbManagerMu.RUnlock()
	return globalTBManager
}

// CloseTigerBeetle closes the global TigerBeetle manager
func CloseTigerBeetle() {
	tbManagerMu.Lock()
	defer tbManagerMu.Unlock()
	if globalTBManager != nil {
		globalTBManager.Close()
		globalTBManager = nil
	}
}

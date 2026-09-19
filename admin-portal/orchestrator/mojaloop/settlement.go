package mojaloop

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// SettlementManager handles settlement cycle tracking and reporting
type SettlementManager struct {
	fspiop          *FSPIOPClient
	cycles          sync.Map
	positions       sync.Map
	mu              sync.RWMutex
}

// SettlementCycle represents a settlement cycle
type SettlementCycle struct {
	CycleId           string                    `json:"cycleId"`
	StartTime         time.Time                 `json:"startTime"`
	EndTime           *time.Time                `json:"endTime,omitempty"`
	Status            SettlementCycleStatus     `json:"status"`
	Participants      []string                  `json:"participants"`
	TotalTransfers    int                       `json:"totalTransfers"`
	TotalAmount       Money                     `json:"totalAmount"`
	NetPositions      map[string]*NetPosition   `json:"netPositions"`
	SettlementWindows []SettlementWindow        `json:"settlementWindows"`
	CreatedAt         time.Time                 `json:"createdAt"`
	UpdatedAt         time.Time                 `json:"updatedAt"`
}

// SettlementCycleStatus represents the status of a settlement cycle
type SettlementCycleStatus string

const (
	SettlementCycleOpen       SettlementCycleStatus = "OPEN"
	SettlementCycleClosed     SettlementCycleStatus = "CLOSED"
	SettlementCyclePending    SettlementCycleStatus = "PENDING_SETTLEMENT"
	SettlementCycleSettled    SettlementCycleStatus = "SETTLED"
	SettlementCycleAborted    SettlementCycleStatus = "ABORTED"
)

// NetPosition represents a participant's net position in a settlement cycle
type NetPosition struct {
	ParticipantId     string    `json:"participantId"`
	Currency          string    `json:"currency"`
	DebitAmount       int64     `json:"debitAmount"`
	CreditAmount      int64     `json:"creditAmount"`
	NetAmount         int64     `json:"netAmount"`
	TransferCount     int       `json:"transferCount"`
	LastUpdated       time.Time `json:"lastUpdated"`
}

// SettlementWindow represents a settlement window within a cycle
type SettlementWindow struct {
	WindowId          string                    `json:"windowId"`
	State             SettlementWindowState     `json:"state"`
	Reason            string                    `json:"reason,omitempty"`
	CreatedDate       time.Time                 `json:"createdDate"`
	ChangedDate       *time.Time                `json:"changedDate,omitempty"`
}

// SettlementWindowState represents the state of a settlement window
type SettlementWindowState string

const (
	WindowStateOpen       SettlementWindowState = "OPEN"
	WindowStateClosed     SettlementWindowState = "CLOSED"
	WindowStatePending    SettlementWindowState = "PENDING_SETTLEMENT"
	WindowStateSettled    SettlementWindowState = "SETTLED"
	WindowStateAborted    SettlementWindowState = "ABORTED"
)

// TransferRecord represents a transfer for settlement tracking
type TransferRecord struct {
	TransferId        string    `json:"transferId"`
	PayerFsp          string    `json:"payerFsp"`
	PayeeFsp          string    `json:"payeeFsp"`
	Amount            Money     `json:"amount"`
	Currency          string    `json:"currency"`
	TransferState     string    `json:"transferState"`
	CompletedAt       time.Time `json:"completedAt"`
	SettlementCycleId string    `json:"settlementCycleId"`
}

// SettlementReport represents a settlement report for Treasury/CBN
type SettlementReport struct {
	ReportId          string                  `json:"reportId"`
	CycleId           string                  `json:"cycleId"`
	GeneratedAt       time.Time               `json:"generatedAt"`
	ReportingPeriod   ReportingPeriod         `json:"reportingPeriod"`
	Summary           SettlementSummary       `json:"summary"`
	ParticipantDetails []ParticipantDetail    `json:"participantDetails"`
	Reconciliation    ReconciliationStatus    `json:"reconciliation"`
}

// ReportingPeriod represents the time period for a report
type ReportingPeriod struct {
	StartDate         time.Time `json:"startDate"`
	EndDate           time.Time `json:"endDate"`
}

// SettlementSummary contains summary statistics
type SettlementSummary struct {
	TotalTransfers    int       `json:"totalTransfers"`
	TotalVolume       Money     `json:"totalVolume"`
	SuccessfulCount   int       `json:"successfulCount"`
	FailedCount       int       `json:"failedCount"`
	ReversedCount     int       `json:"reversedCount"`
	AverageAmount     Money     `json:"averageAmount"`
	PeakHour          string    `json:"peakHour"`
	PeakVolume        Money     `json:"peakVolume"`
}

// ParticipantDetail contains details for a single participant
type ParticipantDetail struct {
	ParticipantId     string    `json:"participantId"`
	ParticipantName   string    `json:"participantName"`
	TotalDebits       Money     `json:"totalDebits"`
	TotalCredits      Money     `json:"totalCredits"`
	NetPosition       Money     `json:"netPosition"`
	TransferCount     int       `json:"transferCount"`
	SettlementAccount string    `json:"settlementAccount"`
}

// ReconciliationStatus contains reconciliation information
type ReconciliationStatus struct {
	Status            string    `json:"status"`
	MatchedCount      int       `json:"matchedCount"`
	UnmatchedCount    int       `json:"unmatchedCount"`
	Discrepancies     []Discrepancy `json:"discrepancies,omitempty"`
}

// Discrepancy represents a reconciliation discrepancy
type Discrepancy struct {
	TransferId        string    `json:"transferId"`
	Type              string    `json:"type"`
	ExpectedAmount    Money     `json:"expectedAmount"`
	ActualAmount      Money     `json:"actualAmount"`
	Description       string    `json:"description"`
}

// NewSettlementManager creates a new settlement manager
func NewSettlementManager(fspiop *FSPIOPClient) *SettlementManager {
	return &SettlementManager{
		fspiop: fspiop,
	}
}

// CreateSettlementCycle creates a new settlement cycle
func (m *SettlementManager) CreateSettlementCycle(ctx context.Context, currency string) (*SettlementCycle, error) {
	cycleId := fmt.Sprintf("CYCLE-%d", time.Now().UnixNano())
	now := time.Now()
	
	cycle := &SettlementCycle{
		CycleId:        cycleId,
		StartTime:      now,
		Status:         SettlementCycleOpen,
		Participants:   []string{},
		TotalTransfers: 0,
		TotalAmount:    Money{Currency: currency, Amount: "0"},
		NetPositions:   make(map[string]*NetPosition),
		SettlementWindows: []SettlementWindow{
			{
				WindowId:    fmt.Sprintf("WIN-%d", now.UnixNano()),
				State:       WindowStateOpen,
				CreatedDate: now,
			},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
	
	m.cycles.Store(cycleId, cycle)
	return cycle, nil
}

// RecordTransfer records a transfer in the current settlement cycle
func (m *SettlementManager) RecordTransfer(ctx context.Context, cycleId string, transfer *TransferRecord) error {
	val, ok := m.cycles.Load(cycleId)
	if !ok {
		return fmt.Errorf("settlement cycle not found: %s", cycleId)
	}
	
	cycle := val.(*SettlementCycle)
	if cycle.Status != SettlementCycleOpen {
		return fmt.Errorf("settlement cycle is not open: %s", cycleId)
	}
	
	m.mu.Lock()
	defer m.mu.Unlock()
	
	cycle.TotalTransfers++
	transfer.SettlementCycleId = cycleId
	
	// Update payer position (debit)
	payerPos, ok := cycle.NetPositions[transfer.PayerFsp]
	if !ok {
		payerPos = &NetPosition{
			ParticipantId: transfer.PayerFsp,
			Currency:      transfer.Currency,
		}
		cycle.NetPositions[transfer.PayerFsp] = payerPos
	}
	amountCents := parseAmountToCents(transfer.Amount.Amount)
	payerPos.DebitAmount += amountCents
	payerPos.NetAmount = payerPos.CreditAmount - payerPos.DebitAmount
	payerPos.TransferCount++
	payerPos.LastUpdated = time.Now()
	
	// Update payee position (credit)
	payeePos, ok := cycle.NetPositions[transfer.PayeeFsp]
	if !ok {
		payeePos = &NetPosition{
			ParticipantId: transfer.PayeeFsp,
			Currency:      transfer.Currency,
		}
		cycle.NetPositions[transfer.PayeeFsp] = payeePos
	}
	payeePos.CreditAmount += amountCents
	payeePos.NetAmount = payeePos.CreditAmount - payeePos.DebitAmount
	payeePos.TransferCount++
	payeePos.LastUpdated = time.Now()
	
	// Update participants list
	if !contains(cycle.Participants, transfer.PayerFsp) {
		cycle.Participants = append(cycle.Participants, transfer.PayerFsp)
	}
	if !contains(cycle.Participants, transfer.PayeeFsp) {
		cycle.Participants = append(cycle.Participants, transfer.PayeeFsp)
	}
	
	cycle.UpdatedAt = time.Now()
	return nil
}

// CloseSettlementCycle closes a settlement cycle for settlement
func (m *SettlementManager) CloseSettlementCycle(ctx context.Context, cycleId string) error {
	val, ok := m.cycles.Load(cycleId)
	if !ok {
		return fmt.Errorf("settlement cycle not found: %s", cycleId)
	}
	
	cycle := val.(*SettlementCycle)
	if cycle.Status != SettlementCycleOpen {
		return fmt.Errorf("settlement cycle is not open: %s", cycleId)
	}
	
	m.mu.Lock()
	defer m.mu.Unlock()
	
	now := time.Now()
	cycle.EndTime = &now
	cycle.Status = SettlementCycleClosed
	
	// Close all open windows
	for i := range cycle.SettlementWindows {
		if cycle.SettlementWindows[i].State == WindowStateOpen {
			cycle.SettlementWindows[i].State = WindowStateClosed
			cycle.SettlementWindows[i].ChangedDate = &now
		}
	}
	
	cycle.UpdatedAt = now
	return nil
}

// SettleCycle marks a cycle as settled
func (m *SettlementManager) SettleCycle(ctx context.Context, cycleId string) error {
	val, ok := m.cycles.Load(cycleId)
	if !ok {
		return fmt.Errorf("settlement cycle not found: %s", cycleId)
	}
	
	cycle := val.(*SettlementCycle)
	if cycle.Status != SettlementCycleClosed && cycle.Status != SettlementCyclePending {
		return fmt.Errorf("settlement cycle must be closed or pending: %s", cycleId)
	}
	
	m.mu.Lock()
	defer m.mu.Unlock()
	
	now := time.Now()
	cycle.Status = SettlementCycleSettled
	
	for i := range cycle.SettlementWindows {
		if cycle.SettlementWindows[i].State == WindowStateClosed || cycle.SettlementWindows[i].State == WindowStatePending {
			cycle.SettlementWindows[i].State = WindowStateSettled
			cycle.SettlementWindows[i].ChangedDate = &now
		}
	}
	
	cycle.UpdatedAt = now
	return nil
}

// GetSettlementCycle retrieves a settlement cycle by ID
func (m *SettlementManager) GetSettlementCycle(ctx context.Context, cycleId string) (*SettlementCycle, error) {
	val, ok := m.cycles.Load(cycleId)
	if !ok {
		return nil, fmt.Errorf("settlement cycle not found: %s", cycleId)
	}
	return val.(*SettlementCycle), nil
}

// GetNetPositions returns net positions for all participants in a cycle
func (m *SettlementManager) GetNetPositions(ctx context.Context, cycleId string) (map[string]*NetPosition, error) {
	val, ok := m.cycles.Load(cycleId)
	if !ok {
		return nil, fmt.Errorf("settlement cycle not found: %s", cycleId)
	}
	
	cycle := val.(*SettlementCycle)
	return cycle.NetPositions, nil
}

// GenerateSettlementReport generates a settlement report for Treasury/CBN
func (m *SettlementManager) GenerateSettlementReport(ctx context.Context, cycleId string) (*SettlementReport, error) {
	val, ok := m.cycles.Load(cycleId)
	if !ok {
		return nil, fmt.Errorf("settlement cycle not found: %s", cycleId)
	}
	
	cycle := val.(*SettlementCycle)
	
	participantDetails := make([]ParticipantDetail, 0, len(cycle.NetPositions))
	for participantId, pos := range cycle.NetPositions {
		participantDetails = append(participantDetails, ParticipantDetail{
			ParticipantId:   participantId,
			ParticipantName: participantId,
			TotalDebits:     Money{Currency: pos.Currency, Amount: fmt.Sprintf("%.2f", float64(pos.DebitAmount)/100)},
			TotalCredits:    Money{Currency: pos.Currency, Amount: fmt.Sprintf("%.2f", float64(pos.CreditAmount)/100)},
			NetPosition:     Money{Currency: pos.Currency, Amount: fmt.Sprintf("%.2f", float64(pos.NetAmount)/100)},
			TransferCount:   pos.TransferCount,
		})
	}
	
	endTime := time.Now()
	if cycle.EndTime != nil {
		endTime = *cycle.EndTime
	}
	
	report := &SettlementReport{
		ReportId:    fmt.Sprintf("RPT-%d", time.Now().UnixNano()),
		CycleId:     cycleId,
		GeneratedAt: time.Now(),
		ReportingPeriod: ReportingPeriod{
			StartDate: cycle.StartTime,
			EndDate:   endTime,
		},
		Summary: SettlementSummary{
			TotalTransfers:  cycle.TotalTransfers,
			TotalVolume:     cycle.TotalAmount,
			SuccessfulCount: cycle.TotalTransfers,
			FailedCount:     0,
			ReversedCount:   0,
		},
		ParticipantDetails: participantDetails,
		Reconciliation: ReconciliationStatus{
			Status:       "MATCHED",
			MatchedCount: cycle.TotalTransfers,
		},
	}
	
	return report, nil
}

// ListOpenCycles returns all open settlement cycles
func (m *SettlementManager) ListOpenCycles(ctx context.Context) ([]*SettlementCycle, error) {
	var openCycles []*SettlementCycle
	m.cycles.Range(func(key, value interface{}) bool {
		cycle := value.(*SettlementCycle)
		if cycle.Status == SettlementCycleOpen {
			openCycles = append(openCycles, cycle)
		}
		return true
	})
	return openCycles, nil
}

func parseAmountToCents(amount string) int64 {
	var cents int64
	fmt.Sscanf(amount, "%d", &cents)
	return cents * 100
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// Global settlement manager
var globalSettlementManager *SettlementManager
var settlementManagerMu sync.RWMutex

// InitSettlementManager initializes the global settlement manager
func InitSettlementManager(fspiop *FSPIOPClient) *SettlementManager {
	settlementManagerMu.Lock()
	defer settlementManagerMu.Unlock()
	globalSettlementManager = NewSettlementManager(fspiop)
	return globalSettlementManager
}

// GetSettlementManager returns the global settlement manager
func GetSettlementManager() *SettlementManager {
	settlementManagerMu.RLock()
	defer settlementManagerMu.RUnlock()
	return globalSettlementManager
}

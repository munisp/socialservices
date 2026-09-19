package analytics

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// IncrementalAggregator performs incremental aggregations with watermarks
type IncrementalAggregator struct {
	name          string
	watermark     time.Time
	state         *AggregationState
	stateFile     string
	mu            sync.RWMutex
	updateChan    chan ParquetEvent
	flushInterval time.Duration
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
}

// AggregationState stores the current aggregation state
type AggregationState struct {
	Watermark     time.Time              `json:"watermark"`
	LastUpdate    time.Time              `json:"last_update"`
	Metrics       map[string]interface{} `json:"metrics"`
	EventCount    int64                  `json:"event_count"`
	ProcessedKeys map[string]bool        `json:"processed_keys"` // For deduplication
}

// MaterializedView represents a pre-computed view
type MaterializedView struct {
	Name          string
	Query         string
	RefreshPolicy string // "on_demand", "periodic", "incremental"
	LastRefresh   time.Time
	Data          interface{}
	mu            sync.RWMutex
}

// MetricsCollector collects and exposes Prometheus-style metrics
type MetricsCollector struct {
	metrics map[string]*Metric
	mu      sync.RWMutex
}

// Metric represents a single metric
type Metric struct {
	Name      string
	Type      string // "counter", "gauge", "histogram"
	Value     float64
	Labels    map[string]string
	Timestamp time.Time
}

// NewIncrementalAggregator creates a new incremental aggregator
func NewIncrementalAggregator(name string, stateDir string, flushInterval time.Duration) (*IncrementalAggregator, error) {
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create state directory: %w", err)
	}

	stateFile := filepath.Join(stateDir, fmt.Sprintf("%s_state.json", name))
	ctx, cancel := context.WithCancel(context.Background())

	agg := &IncrementalAggregator{
		name:          name,
		stateFile:     stateFile,
		updateChan:    make(chan ParquetEvent, 10000),
		flushInterval: flushInterval,
		ctx:           ctx,
		cancel:        cancel,
		state: &AggregationState{
			Watermark:     time.Time{},
			Metrics:       make(map[string]interface{}),
			ProcessedKeys: make(map[string]bool),
		},
	}

	// Load existing state
	if err := agg.loadState(); err != nil {
		log.Printf("[IncrementalAgg] Warning: failed to load state: %v", err)
	}

	// Start aggregation worker
	agg.wg.Add(1)
	go agg.aggregationWorker()

	return agg, nil
}

// loadState loads aggregation state from disk
func (ia *IncrementalAggregator) loadState() error {
	data, err := os.ReadFile(ia.stateFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No state file yet
		}
		return err
	}

	ia.mu.Lock()
	defer ia.mu.Unlock()
	return json.Unmarshal(data, ia.state)
}

// saveState saves aggregation state to disk
func (ia *IncrementalAggregator) saveState() error {
	ia.mu.RLock()
	data, err := json.MarshalIndent(ia.state, "", "  ")
	ia.mu.RUnlock()

	if err != nil {
		return err
	}

	return os.WriteFile(ia.stateFile, data, 0644)
}

// ProcessEvent processes a single event
func (ia *IncrementalAggregator) ProcessEvent(event ParquetEvent) {
	select {
	case ia.updateChan <- event:
	case <-ia.ctx.Done():
	default:
		log.Printf("[IncrementalAgg] Warning: update channel full, dropping event")
	}
}

// aggregationWorker processes events and updates state
func (ia *IncrementalAggregator) aggregationWorker() {
	defer ia.wg.Done()

	ticker := time.NewTicker(ia.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ia.ctx.Done():
			ia.flush()
			return

		case event := <-ia.updateChan:
			ia.processEventInternal(event)

		case <-ticker.C:
			ia.flush()
		}
	}
}

// processEventInternal processes an event and updates metrics
func (ia *IncrementalAggregator) processEventInternal(event ParquetEvent) {
	ia.mu.Lock()
	defer ia.mu.Unlock()

	// Check if already processed (deduplication)
	if ia.state.ProcessedKeys[event.IdempotencyKey] {
		return
	}

	// Update watermark (event time)
	eventTime := time.UnixMilli(event.Timestamp)
	if eventTime.After(ia.state.Watermark) {
		ia.state.Watermark = eventTime
	}

	// Update metrics based on event type
	switch ia.name {
	case "disbursement_agg":
		ia.updateDisbursementMetrics(event)
	case "enrollment_agg":
		ia.updateEnrollmentMetrics(event)
	default:
		ia.updateGenericMetrics(event)
	}

	// Mark as processed
	ia.state.ProcessedKeys[event.IdempotencyKey] = true
	ia.state.EventCount++
	ia.state.LastUpdate = time.Now()

	// Cleanup old processed keys (keep last 10k)
	if len(ia.state.ProcessedKeys) > 10000 {
		ia.cleanupProcessedKeys()
	}
}

// updateDisbursementMetrics updates disbursement-specific metrics
func (ia *IncrementalAggregator) updateDisbursementMetrics(event ParquetEvent) {
	// Total amount
	if totalAmount, ok := ia.state.Metrics["total_amount"].(float64); ok {
		ia.state.Metrics["total_amount"] = totalAmount + event.Amount
	} else {
		ia.state.Metrics["total_amount"] = event.Amount
	}

	// Count by program
	byProgram, ok := ia.state.Metrics["by_program"].(map[string]interface{})
	if !ok {
		byProgram = make(map[string]interface{})
		ia.state.Metrics["by_program"] = byProgram
	}

	if count, ok := byProgram[event.ProgramID].(float64); ok {
		byProgram[event.ProgramID] = count + 1
	} else {
		byProgram[event.ProgramID] = 1.0
	}
}

// updateEnrollmentMetrics updates enrollment-specific metrics
func (ia *IncrementalAggregator) updateEnrollmentMetrics(event ParquetEvent) {
	// Total enrollments
	if total, ok := ia.state.Metrics["total_enrollments"].(float64); ok {
		ia.state.Metrics["total_enrollments"] = total + 1
	} else {
		ia.state.Metrics["total_enrollments"] = 1.0
	}

	// Count by program
	byProgram, ok := ia.state.Metrics["by_program"].(map[string]interface{})
	if !ok {
		byProgram = make(map[string]interface{})
		ia.state.Metrics["by_program"] = byProgram
	}

	if count, ok := byProgram[event.ProgramID].(float64); ok {
		byProgram[event.ProgramID] = count + 1
	} else {
		byProgram[event.ProgramID] = 1.0
	}
}

// updateGenericMetrics updates generic metrics
func (ia *IncrementalAggregator) updateGenericMetrics(event ParquetEvent) {
	if count, ok := ia.state.Metrics["event_count"].(float64); ok {
		ia.state.Metrics["event_count"] = count + 1
	} else {
		ia.state.Metrics["event_count"] = 1.0
	}
}

// cleanupProcessedKeys removes old processed keys
func (ia *IncrementalAggregator) cleanupProcessedKeys() {
	// Keep only recent 5k entries
	count := 0
	for key := range ia.state.ProcessedKeys {
		delete(ia.state.ProcessedKeys, key)
		count++
		if count >= 5000 {
			break
		}
	}
}

// flush saves current state to disk
func (ia *IncrementalAggregator) flush() {
	if err := ia.saveState(); err != nil {
		log.Printf("[IncrementalAgg] Error saving state: %v", err)
	} else {
		ia.mu.RLock()
		log.Printf("[IncrementalAgg] %s: Flushed state, watermark=%s, events=%d",
			ia.name, ia.state.Watermark.Format(time.RFC3339), ia.state.EventCount)
		ia.mu.RUnlock()
	}
}

// GetMetrics returns current aggregation metrics
func (ia *IncrementalAggregator) GetMetrics() map[string]interface{} {
	ia.mu.RLock()
	defer ia.mu.RUnlock()

	// Deep copy metrics
	metrics := make(map[string]interface{})
	for k, v := range ia.state.Metrics {
		metrics[k] = v
	}
	metrics["watermark"] = ia.state.Watermark
	metrics["event_count"] = ia.state.EventCount
	metrics["last_update"] = ia.state.LastUpdate

	return metrics
}

// Close gracefully shuts down the aggregator
func (ia *IncrementalAggregator) Close() error {
	ia.cancel()
	ia.wg.Wait()
	return ia.saveState()
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		metrics: make(map[string]*Metric),
	}
}

// RecordCounter records a counter metric
func (mc *MetricsCollector) RecordCounter(name string, value float64, labels map[string]string) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	key := mc.metricKey(name, labels)
	if existing, ok := mc.metrics[key]; ok {
		existing.Value += value
		existing.Timestamp = time.Now()
	} else {
		mc.metrics[key] = &Metric{
			Name:      name,
			Type:      "counter",
			Value:     value,
			Labels:    labels,
			Timestamp: time.Now(),
		}
	}
}

// RecordGauge records a gauge metric
func (mc *MetricsCollector) RecordGauge(name string, value float64, labels map[string]string) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	key := mc.metricKey(name, labels)
	mc.metrics[key] = &Metric{
		Name:      name,
		Type:      "gauge",
		Value:     value,
		Labels:    labels,
		Timestamp: time.Now(),
	}
}

// metricKey generates a unique key for a metric
func (mc *MetricsCollector) metricKey(name string, labels map[string]string) string {
	key := name
	for k, v := range labels {
		key += fmt.Sprintf("_%s=%s", k, v)
	}
	return key
}

// GetMetrics returns all metrics in Prometheus text format
func (mc *MetricsCollector) GetMetrics() string {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	var output string
	for _, metric := range mc.metrics {
		// Format: metric_name{label1="value1",label2="value2"} value timestamp
		labels := ""
		if len(metric.Labels) > 0 {
			labels = "{"
			first := true
			for k, v := range metric.Labels {
				if !first {
					labels += ","
				}
				labels += fmt.Sprintf("%s=\"%s\"", k, v)
				first = false
			}
			labels += "}"
		}

		output += fmt.Sprintf("%s%s %.2f %d\n",
			metric.Name, labels, metric.Value, metric.Timestamp.Unix())
	}

	return output
}

// ExportMetrics exports metrics to a file
func (mc *MetricsCollector) ExportMetrics(filename string) error {
	metrics := mc.GetMetrics()
	return os.WriteFile(filename, []byte(metrics), 0644)
}

// NewMaterializedView creates a new materialized view
func NewMaterializedView(name, query, refreshPolicy string) *MaterializedView {
	return &MaterializedView{
		Name:          name,
		Query:         query,
		RefreshPolicy: refreshPolicy,
		LastRefresh:   time.Time{},
	}
}

// Refresh refreshes the materialized view by executing the query against the data source
func (mv *MaterializedView) Refresh(dataSource interface{}) error {
	mv.mu.Lock()
	defer mv.mu.Unlock()

	// Execute query against dataSource based on its type
	switch ds := dataSource.(type) {
	case *QueryEngine:
		// Execute SQL query against QueryEngine
		if mv.Query != "" {
			results, err := ds.ExecuteQuery(mv.Query)
			if err != nil {
				return fmt.Errorf("failed to execute query for view %s: %w", mv.Name, err)
			}
			mv.Data = results
			log.Printf("[MaterializedView] Refreshed view %s with %d rows from QueryEngine", mv.Name, len(results))
		} else {
			mv.Data = nil
			log.Printf("[MaterializedView] View %s has no query defined", mv.Name)
		}

	case []ParquetEvent:
		// Process ParquetEvent slice directly for aggregations
		mv.Data = mv.aggregateEvents(ds)
		log.Printf("[MaterializedView] Refreshed view %s with %d events", mv.Name, len(ds))

	case map[string]interface{}:
		// Direct data assignment for pre-computed results
		mv.Data = ds
		log.Printf("[MaterializedView] Refreshed view %s with pre-computed data", mv.Name)

	case *LakehouseManager:
		// Query from Lakehouse using the view's query as event type
		if mv.Query != "" {
			events, err := ds.readEvents(mv.Query)
			if err != nil {
				return fmt.Errorf("failed to read events for view %s: %w", mv.Name, err)
			}
			mv.Data = mv.aggregateEvents(convertAnalyticsToParquet(events))
			log.Printf("[MaterializedView] Refreshed view %s with %d events from Lakehouse", mv.Name, len(events))
		}

	default:
		// Fallback: store the data source directly
		mv.Data = dataSource
		log.Printf("[MaterializedView] Refreshed view %s with direct data assignment", mv.Name)
	}

	mv.LastRefresh = time.Now()
	return nil
}

// aggregateEvents performs basic aggregation on ParquetEvent data
func (mv *MaterializedView) aggregateEvents(events []ParquetEvent) map[string]interface{} {
	result := make(map[string]interface{})

	if len(events) == 0 {
		result["count"] = 0
		result["total_amount"] = 0.0
		return result
	}

	var totalAmount float64
	byProgram := make(map[string]float64)
	byBeneficiary := make(map[string]float64)
	var minTimestamp, maxTimestamp int64

	for i, event := range events {
		totalAmount += event.Amount

		if event.ProgramID != "" {
			byProgram[event.ProgramID] += event.Amount
		}
		if event.BeneficiaryID != "" {
			byBeneficiary[event.BeneficiaryID] += event.Amount
		}

		if i == 0 || event.Timestamp < minTimestamp {
			minTimestamp = event.Timestamp
		}
		if i == 0 || event.Timestamp > maxTimestamp {
			maxTimestamp = event.Timestamp
		}
	}

	result["count"] = len(events)
	result["total_amount"] = totalAmount
	result["average_amount"] = totalAmount / float64(len(events))
	result["by_program"] = byProgram
	result["by_beneficiary"] = byBeneficiary
	result["min_timestamp"] = time.UnixMilli(minTimestamp)
	result["max_timestamp"] = time.UnixMilli(maxTimestamp)

	return result
}

// convertAnalyticsToParquet converts AnalyticsEvent slice to ParquetEvent slice
func convertAnalyticsToParquet(events []AnalyticsEvent) []ParquetEvent {
	result := make([]ParquetEvent, len(events))
	for i, e := range events {
		result[i] = ParquetEvent{
			EventID:       e.EventID,
			EventType:     e.EventType,
			Timestamp:     e.Timestamp.UnixMilli(),
			BeneficiaryID: e.BeneficiaryID,
			ProgramID:     e.ProgramID,
			Amount:        e.Amount,
		}
	}
	return result
}

// GetData returns the materialized view data
func (mv *MaterializedView) GetData() interface{} {
	mv.mu.RLock()
	defer mv.mu.RUnlock()
	return mv.Data
}

// ShouldRefresh checks if the view should be refreshed
func (mv *MaterializedView) ShouldRefresh(interval time.Duration) bool {
	mv.mu.RLock()
	defer mv.mu.RUnlock()

	if mv.RefreshPolicy == "on_demand" {
		return false
	}

	return time.Since(mv.LastRefresh) >= interval
}

// MaterializedViewManager manages multiple materialized views
type MaterializedViewManager struct {
	views map[string]*MaterializedView
	mu    sync.RWMutex
}

// NewMaterializedViewManager creates a new materialized view manager
func NewMaterializedViewManager() *MaterializedViewManager {
	return &MaterializedViewManager{
		views: make(map[string]*MaterializedView),
	}
}

// RegisterView registers a new materialized view
func (mvm *MaterializedViewManager) RegisterView(view *MaterializedView) {
	mvm.mu.Lock()
	defer mvm.mu.Unlock()
	mvm.views[view.Name] = view
	log.Printf("[MaterializedViewManager] Registered view: %s", view.Name)
}

// GetView returns a materialized view by name
func (mvm *MaterializedViewManager) GetView(name string) (*MaterializedView, error) {
	mvm.mu.RLock()
	defer mvm.mu.RUnlock()

	if view, ok := mvm.views[name]; ok {
		return view, nil
	}

	return nil, fmt.Errorf("view not found: %s", name)
}

// RefreshAll refreshes all views based on their policies
func (mvm *MaterializedViewManager) RefreshAll(dataSource interface{}, interval time.Duration) {
	mvm.mu.RLock()
	views := make([]*MaterializedView, 0, len(mvm.views))
	for _, view := range mvm.views {
		views = append(views, view)
	}
	mvm.mu.RUnlock()

	for _, view := range views {
		if view.ShouldRefresh(interval) {
			if err := view.Refresh(dataSource); err != nil {
				log.Printf("[MaterializedViewManager] Error refreshing view %s: %v", view.Name, err)
			}
		}
	}
}

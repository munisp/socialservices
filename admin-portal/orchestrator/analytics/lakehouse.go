package analytics

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/segmentio/kafka-go"
)

// LakehouseConfig holds configuration for the Lakehouse analytics pipeline
type LakehouseConfig struct {
	DataPath       string
	KafkaBrokers   []string
	FluvioEndpoint string
}

// LakehouseManager manages the Lakehouse analytics pipeline
type LakehouseManager struct {
	config       *LakehouseConfig
	kafkaReaders map[string]*kafka.Reader
}

// NewLakehouseManager creates a new Lakehouse manager
func NewLakehouseManager(config *LakehouseConfig) (*LakehouseManager, error) {
	// Create data directory if it doesn't exist
	if err := os.MkdirAll(config.DataPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	return &LakehouseManager{
		config:       config,
		kafkaReaders: make(map[string]*kafka.Reader),
	}, nil
}

// Event types for analytics
type AnalyticsEvent struct {
	EventID       string                 `json:"event_id"`
	EventType     string                 `json:"event_type"`
	Timestamp     time.Time              `json:"timestamp"`
	BeneficiaryID string                 `json:"beneficiary_id,omitempty"`
	ProgramID     string                 `json:"program_id,omitempty"`
	Amount        float64                `json:"amount,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// IngestFromKafka ingests events from Kafka topics into the Lakehouse
func (lm *LakehouseManager) IngestFromKafka(ctx context.Context, topic string) error {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  lm.config.KafkaBrokers,
		Topic:    topic,
		GroupID:  "lakehouse-ingestion",
		MinBytes: 10e3, // 10KB
		MaxBytes: 10e6, // 10MB
	})

	lm.kafkaReaders[topic] = reader

	log.Printf("Starting Kafka ingestion for topic: %s", topic)

	go func() {
		defer reader.Close()

		for {
			select {
			case <-ctx.Done():
				log.Printf("Stopping Kafka ingestion for topic: %s", topic)
				return
			default:
				msg, err := reader.ReadMessage(ctx)
				if err != nil {
					log.Printf("Error reading from Kafka topic %s: %v", topic, err)
					continue
				}

				// Parse event
				var event AnalyticsEvent
				if err := json.Unmarshal(msg.Value, &event); err != nil {
					log.Printf("Error unmarshaling event: %v", err)
					continue
				}

				// Write to Lakehouse
				if err := lm.writeEvent(event); err != nil {
					log.Printf("Error writing event to Lakehouse: %v", err)
				}
			}
		}
	}()

	return nil
}

// writeEvent writes an event to the Lakehouse (Delta Lake format simulation)
func (lm *LakehouseManager) writeEvent(event AnalyticsEvent) error {
	// Create partition directory based on date
	partition := event.Timestamp.Format("2006-01-02")
	partitionPath := filepath.Join(lm.config.DataPath, event.EventType, partition)

	if err := os.MkdirAll(partitionPath, 0755); err != nil {
		return fmt.Errorf("failed to create partition directory: %w", err)
	}

	// Write event as JSON file (in production, use Parquet format)
	filename := filepath.Join(partitionPath, fmt.Sprintf("%s.json", event.EventID))
	data, err := json.MarshalIndent(event, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write event file: %w", err)
	}

	return nil
}

// AggregationJob represents a scheduled aggregation job
type AggregationJob struct {
	Name      string
	Schedule  string // cron format
	EventType string
	Aggregate func(events []AnalyticsEvent) (interface{}, error)
}

// RunAggregationJob runs an aggregation job on historical data
func (lm *LakehouseManager) RunAggregationJob(ctx context.Context, job AggregationJob) error {
	log.Printf("Running aggregation job: %s", job.Name)

	// Read events from Lakehouse
	events, err := lm.readEvents(job.EventType)
	if err != nil {
		return fmt.Errorf("failed to read events: %w", err)
	}

	// Run aggregation
	result, err := job.Aggregate(events)
	if err != nil {
		return fmt.Errorf("aggregation failed: %w", err)
	}

	// Write result
	resultPath := filepath.Join(lm.config.DataPath, "aggregations", job.Name)
	if err := os.MkdirAll(resultPath, 0755); err != nil {
		return fmt.Errorf("failed to create result directory: %w", err)
	}

	resultFile := filepath.Join(resultPath, fmt.Sprintf("%s.json", time.Now().Format("2006-01-02-15-04-05")))
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal result: %w", err)
	}

	if err := os.WriteFile(resultFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write result: %w", err)
	}

	log.Printf("Aggregation job %s completed successfully", job.Name)
	return nil
}

// readEvents reads all events of a specific type from the Lakehouse
func (lm *LakehouseManager) readEvents(eventType string) ([]AnalyticsEvent, error) {
	var events []AnalyticsEvent

	eventTypePath := filepath.Join(lm.config.DataPath, eventType)
	if _, err := os.Stat(eventTypePath); os.IsNotExist(err) {
		return events, nil // No events yet
	}

	err := filepath.Walk(eventTypePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && filepath.Ext(path) == ".json" {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}

			var event AnalyticsEvent
			if err := json.Unmarshal(data, &event); err != nil {
				return err
			}

			events = append(events, event)
		}

		return nil
	})

	return events, err
}

// Predefined aggregation jobs

// DisbursementAggregation aggregates disbursement data
func DisbursementAggregation(events []AnalyticsEvent) (interface{}, error) {
	type DisbursementStats struct {
		TotalAmount      float64            `json:"total_amount"`
		TotalCount       int                `json:"total_count"`
		ByProgram        map[string]float64 `json:"by_program"`
		ByBeneficiary    map[string]float64 `json:"by_beneficiary"`
		AverageAmount    float64            `json:"average_amount"`
		EarliestDate     time.Time          `json:"earliest_date"`
		LatestDate       time.Time          `json:"latest_date"`
	}

	stats := DisbursementStats{
		ByProgram:     make(map[string]float64),
		ByBeneficiary: make(map[string]float64),
	}

	for _, event := range events {
		stats.TotalAmount += event.Amount
		stats.TotalCount++

		if event.ProgramID != "" {
			stats.ByProgram[event.ProgramID] += event.Amount
		}

		if event.BeneficiaryID != "" {
			stats.ByBeneficiary[event.BeneficiaryID] += event.Amount
		}

		if stats.EarliestDate.IsZero() || event.Timestamp.Before(stats.EarliestDate) {
			stats.EarliestDate = event.Timestamp
		}

		if stats.LatestDate.IsZero() || event.Timestamp.After(stats.LatestDate) {
			stats.LatestDate = event.Timestamp
		}
	}

	if stats.TotalCount > 0 {
		stats.AverageAmount = stats.TotalAmount / float64(stats.TotalCount)
	}

	return stats, nil
}

// EnrollmentAggregation aggregates enrollment data
func EnrollmentAggregation(events []AnalyticsEvent) (interface{}, error) {
	type EnrollmentStats struct {
		TotalEnrollments int                `json:"total_enrollments"`
		ByProgram        map[string]int     `json:"by_program"`
		ByStatus         map[string]int     `json:"by_status"`
		DailyEnrollments map[string]int     `json:"daily_enrollments"`
	}

	stats := EnrollmentStats{
		ByProgram:        make(map[string]int),
		ByStatus:         make(map[string]int),
		DailyEnrollments: make(map[string]int),
	}

	for _, event := range events {
		stats.TotalEnrollments++

		if event.ProgramID != "" {
			stats.ByProgram[event.ProgramID]++
		}

		if status, ok := event.Metadata["status"].(string); ok {
			stats.ByStatus[status]++
		}

		day := event.Timestamp.Format("2006-01-02")
		stats.DailyEnrollments[day]++
	}

	return stats, nil
}

// Close closes all Kafka readers
func (lm *LakehouseManager) Close() {
	for topic, reader := range lm.kafkaReaders {
		log.Printf("Closing Kafka reader for topic: %s", topic)
		reader.Close()
	}
}

// Global Lakehouse manager instance
var globalLakehouseManager *LakehouseManager

// InitLakehouse initializes the global Lakehouse manager
func InitLakehouse(config *LakehouseConfig) error {
	manager, err := NewLakehouseManager(config)
	if err != nil {
		return err
	}
	globalLakehouseManager = manager
	log.Println("Lakehouse analytics pipeline initialized successfully")
	return nil
}

// GetLakehouseManager returns the global Lakehouse manager
func GetLakehouseManager() *LakehouseManager {
	return globalLakehouseManager
}

// CloseLakehouse closes the global Lakehouse manager
func CloseLakehouse() {
	if globalLakehouseManager != nil {
		globalLakehouseManager.Close()
	}
}

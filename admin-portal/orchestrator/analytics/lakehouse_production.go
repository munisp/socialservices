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

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
	"github.com/segmentio/parquet-go"
)

// ProductionLakehouseConfig holds configuration for production Lakehouse
type ProductionLakehouseConfig struct {
	DataPath          string
	KafkaBrokers      []string
	BatchSize         int
	BatchTimeout      time.Duration
	OffsetStoragePath string
	SchemaRegistryURL string
	EnableMetrics     bool
}

// ProductionLakehouseManager manages production-grade Lakehouse analytics
type ProductionLakehouseManager struct {
	config        *ProductionLakehouseConfig
	kafkaReaders  map[string]*kafka.Reader
	eventBuffers  map[string]*EventBuffer
	offsetManager *OffsetManager
	metrics       *LakehouseMetrics
	mu            sync.RWMutex
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
}

// EventBuffer buffers events for batch writing
type EventBuffer struct {
	events    []ParquetEvent
	messages  []kafka.Message
	idempotencyKeys []string
	mu        sync.Mutex
	flushChan chan struct{}
	lastFlush time.Time
}

// ParquetEvent represents an event in Parquet-compatible format
type ParquetEvent struct {
	EventID       string  `parquet:"name=event_id, type=BYTE_ARRAY, convertedtype=UTF8"`
	EventType     string  `parquet:"name=event_type, type=BYTE_ARRAY, convertedtype=UTF8"`
	Timestamp     int64   `parquet:"name=timestamp, type=INT64, convertedtype=TIMESTAMP_MILLIS"`
	BeneficiaryID string  `parquet:"name=beneficiary_id, type=BYTE_ARRAY, convertedtype=UTF8, repetitiontype=OPTIONAL"`
	ProgramID     string  `parquet:"name=program_id, type=BYTE_ARRAY, convertedtype=UTF8, repetitiontype=OPTIONAL"`
	Amount        float64 `parquet:"name=amount, type=DOUBLE, repetitiontype=OPTIONAL"`
	MetadataJSON  string  `parquet:"name=metadata_json, type=BYTE_ARRAY, convertedtype=UTF8, repetitiontype=OPTIONAL"`
	PartitionDate string  `parquet:"name=partition_date, type=BYTE_ARRAY, convertedtype=UTF8"`
	IdempotencyKey string `parquet:"name=idempotency_key, type=BYTE_ARRAY, convertedtype=UTF8"`
}

// OffsetManager manages Kafka offsets for exactly-once semantics
type OffsetManager struct {
	storagePath string
	offsets     map[string]map[int]int64 // topic -> partition -> offset
	mu          sync.RWMutex
	idempotencyCache map[string]bool // cache of processed idempotency keys
	idempotencyMu    sync.RWMutex
}

// LakehouseMetrics tracks performance metrics
type LakehouseMetrics struct {
	EventsIngested    int64
	EventsWritten     int64
	EventsFailed      int64
	BatchesWritten    int64
	LastIngestTime    time.Time
	LastWriteTime     time.Time
	KafkaLag          map[string]int64
	mu                sync.RWMutex
}

// NewProductionLakehouseManager creates a new production Lakehouse manager
func NewProductionLakehouseManager(config *ProductionLakehouseConfig) (*ProductionLakehouseManager, error) {
	// Create data directory
	if err := os.MkdirAll(config.DataPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	// Create offset storage directory
	if err := os.MkdirAll(config.OffsetStoragePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create offset storage: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	offsetManager, err := NewOffsetManager(config.OffsetStoragePath)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create offset manager: %w", err)
	}

	lm := &ProductionLakehouseManager{
		config:        config,
		kafkaReaders:  make(map[string]*kafka.Reader),
		eventBuffers:  make(map[string]*EventBuffer),
		offsetManager: offsetManager,
		metrics:       &LakehouseMetrics{KafkaLag: make(map[string]int64)},
		ctx:           ctx,
		cancel:        cancel,
	}

	// Start metrics reporter if enabled
	if config.EnableMetrics {
		lm.wg.Add(1)
		go lm.reportMetrics()
	}

	return lm, nil
}

// NewOffsetManager creates a new offset manager
func NewOffsetManager(storagePath string) (*OffsetManager, error) {
	om := &OffsetManager{
		storagePath:      storagePath,
		offsets:          make(map[string]map[int]int64),
		idempotencyCache: make(map[string]bool),
	}

	// Load existing offsets
	if err := om.load(); err != nil {
		log.Printf("Warning: failed to load offsets: %v", err)
	}

	return om, nil
}

// load loads offsets from disk
func (om *OffsetManager) load() error {
	offsetFile := filepath.Join(om.storagePath, "offsets.json")
	data, err := os.ReadFile(offsetFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No offsets file yet
		}
		return err
	}

	om.mu.Lock()
	defer om.mu.Unlock()
	return json.Unmarshal(data, &om.offsets)
}

// save saves offsets to disk
func (om *OffsetManager) save() error {
	om.mu.RLock()
	data, err := json.MarshalIndent(om.offsets, "", "  ")
	om.mu.RUnlock()

	if err != nil {
		return err
	}

	offsetFile := filepath.Join(om.storagePath, "offsets.json")
	return os.WriteFile(offsetFile, data, 0644)
}

// GetOffset returns the stored offset for a topic/partition
func (om *OffsetManager) GetOffset(topic string, partition int) int64 {
	om.mu.RLock()
	defer om.mu.RUnlock()

	if partitions, ok := om.offsets[topic]; ok {
		if offset, ok := partitions[partition]; ok {
			return offset
		}
	}
	return kafka.FirstOffset
}

// CommitOffset commits an offset for a topic/partition
func (om *OffsetManager) CommitOffset(topic string, partition int, offset int64) error {
	om.mu.Lock()
	if _, ok := om.offsets[topic]; !ok {
		om.offsets[topic] = make(map[int]int64)
	}
	om.offsets[topic][partition] = offset
	om.mu.Unlock()

	return om.save()
}

// CheckIdempotency checks if an event has been processed
func (om *OffsetManager) CheckIdempotency(key string) bool {
	om.idempotencyMu.RLock()
	defer om.idempotencyMu.RUnlock()
	return om.idempotencyCache[key]
}

// MarkProcessed marks an event as processed
func (om *OffsetManager) MarkProcessed(key string) {
	om.idempotencyMu.Lock()
	om.idempotencyCache[key] = true
	om.idempotencyMu.Unlock()

	// Cleanup old entries (keep last 100k)
	if len(om.idempotencyCache) > 100000 {
		om.cleanupIdempotencyCache()
	}
}

// cleanupIdempotencyCache removes old entries
func (om *OffsetManager) cleanupIdempotencyCache() {
	om.idempotencyMu.Lock()
	defer om.idempotencyMu.Unlock()

	// Keep only recent 50k entries (simple FIFO cleanup)
	if len(om.idempotencyCache) > 50000 {
		count := 0
		for key := range om.idempotencyCache {
			delete(om.idempotencyCache, key)
			count++
			if count >= 50000 {
				break
			}
		}
	}
}

// IngestFromKafka ingests events from Kafka with batch writes and offset management
func (lm *ProductionLakehouseManager) IngestFromKafka(topic string) error {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     lm.config.KafkaBrokers,
		Topic:       topic,
		GroupID:     "lakehouse-production",
		MinBytes:    10e3,
		MaxBytes:    10e6,
		StartOffset: kafka.FirstOffset,
	})

	lm.mu.Lock()
	lm.kafkaReaders[topic] = reader
	lm.eventBuffers[topic] = &EventBuffer{
		events:    make([]ParquetEvent, 0, lm.config.BatchSize),
		messages:  make([]kafka.Message, 0, lm.config.BatchSize),
		idempotencyKeys: make([]string, 0, lm.config.BatchSize),
		flushChan: make(chan struct{}, 1),
		lastFlush: time.Now(),
	}
	lm.mu.Unlock()

	log.Printf("[Lakehouse] Starting Kafka ingestion for topic: %s", topic)

	// Start batch flusher
	lm.wg.Add(1)
	go lm.batchFlusher(topic)

	// Start ingestion loop
	lm.wg.Add(1)
	go func() {
		defer lm.wg.Done()
		defer reader.Close()

		for {
			select {
			case <-lm.ctx.Done():
				log.Printf("[Lakehouse] Stopping ingestion for topic: %s", topic)
				lm.flushBuffer(topic) // Final flush
				return
			default:
				msg, err := reader.FetchMessage(lm.ctx)
				if err != nil {
					if err == context.Canceled {
						return
					}
					log.Printf("[Lakehouse] Error fetching message: %v", err)
					lm.metrics.mu.Lock()
					lm.metrics.EventsFailed++
					lm.metrics.mu.Unlock()
					continue
				}

				// Parse event
				var rawEvent AnalyticsEvent
				if err := json.Unmarshal(msg.Value, &rawEvent); err != nil {
					log.Printf("[Lakehouse] Error unmarshaling event: %v", err)
					reader.CommitMessages(lm.ctx, msg) // Commit to skip bad message
					lm.metrics.mu.Lock()
					lm.metrics.EventsFailed++
					lm.metrics.mu.Unlock()
					continue
				}

				// Generate idempotency key
				idempotencyKey := fmt.Sprintf("%s-%s-%d", topic, rawEvent.EventID, msg.Offset)

				// Check idempotency
				if lm.offsetManager.CheckIdempotency(idempotencyKey) {
					reader.CommitMessages(lm.ctx, msg)
					continue // Skip duplicate
				}

				// Convert to Parquet format
				parquetEvent := ParquetEvent{
					EventID:        rawEvent.EventID,
					EventType:      rawEvent.EventType,
					Timestamp:      rawEvent.Timestamp.UnixMilli(),
					BeneficiaryID:  rawEvent.BeneficiaryID,
					ProgramID:      rawEvent.ProgramID,
					Amount:         rawEvent.Amount,
					PartitionDate:  rawEvent.Timestamp.Format("2006-01-02"),
					IdempotencyKey: idempotencyKey,
				}

				if len(rawEvent.Metadata) > 0 {
					metadataJSON, _ := json.Marshal(rawEvent.Metadata)
					parquetEvent.MetadataJSON = string(metadataJSON)
				}

					// Buffer the message and acknowledge it only after Parquet persistence succeeds.
					lm.addToBuffer(topic, parquetEvent, msg, idempotencyKey)

				lm.metrics.mu.Lock()
				lm.metrics.EventsIngested++
				lm.metrics.LastIngestTime = time.Now()
				lm.metrics.mu.Unlock()
			}
		}
	}()

	return nil
}

// addToBuffer adds an event to the buffer
func (lm *ProductionLakehouseManager) addToBuffer(topic string, event ParquetEvent, message kafka.Message, idempotencyKey string) {
	lm.mu.RLock()
	buffer := lm.eventBuffers[topic]
	lm.mu.RUnlock()

	buffer.mu.Lock()
	buffer.events = append(buffer.events, event)
	buffer.messages = append(buffer.messages, message)
	buffer.idempotencyKeys = append(buffer.idempotencyKeys, idempotencyKey)
	shouldFlush := len(buffer.events) >= lm.config.BatchSize
	buffer.mu.Unlock()

	if shouldFlush {
		select {
		case buffer.flushChan <- struct{}{}:
		default:
		}
	}
}

// batchFlusher periodically flushes buffered events
func (lm *ProductionLakehouseManager) batchFlusher(topic string) {
	defer lm.wg.Done()

	ticker := time.NewTicker(lm.config.BatchTimeout)
	defer ticker.Stop()

	lm.mu.RLock()
	buffer := lm.eventBuffers[topic]
	lm.mu.RUnlock()

	for {
		select {
		case <-lm.ctx.Done():
			return
		case <-ticker.C:
			lm.flushBuffer(topic)
		case <-buffer.flushChan:
			lm.flushBuffer(topic)
		}
	}
}

// flushBuffer writes buffered events to Parquet files
func (lm *ProductionLakehouseManager) flushBuffer(topic string) {
	lm.mu.RLock()
	buffer := lm.eventBuffers[topic]
	lm.mu.RUnlock()

	buffer.mu.Lock()
	if len(buffer.events) == 0 {
		buffer.mu.Unlock()
		return
	}

	events := make([]ParquetEvent, len(buffer.events))
	copy(events, buffer.events)
	messages := make([]kafka.Message, len(buffer.messages))
	copy(messages, buffer.messages)
	idempotencyKeys := make([]string, len(buffer.idempotencyKeys))
	copy(idempotencyKeys, buffer.idempotencyKeys)
	buffer.events = buffer.events[:0]
	buffer.messages = buffer.messages[:0]
	buffer.idempotencyKeys = buffer.idempotencyKeys[:0]
	buffer.lastFlush = time.Now()
	buffer.mu.Unlock()

	// Group by partition date
	partitionGroups := make(map[string][]ParquetEvent)
	for _, event := range events {
		partitionGroups[event.PartitionDate] = append(partitionGroups[event.PartitionDate], event)
	}

	// Write each partition group. Kafka acknowledgement happens only after all writes succeed.
	writeOK := true
	for partitionDate, partitionEvents := range partitionGroups {
		if err := lm.writeParquetBatch(topic, partitionDate, partitionEvents); err != nil {
			writeOK = false
			log.Printf("[Lakehouse] Error writing batch: %v", err)
			lm.metrics.mu.Lock()
			lm.metrics.EventsFailed += int64(len(partitionEvents))
			lm.metrics.mu.Unlock()
		} else {
			lm.metrics.mu.Lock()
			lm.metrics.EventsWritten += int64(len(partitionEvents))
			lm.metrics.BatchesWritten++
			lm.metrics.LastWriteTime = time.Now()
			lm.metrics.mu.Unlock()
		}
	}
	if writeOK {
		for i, msg := range messages {
			if err := lm.kafkaReaders[topic].CommitMessages(lm.ctx, msg); err != nil { log.Printf("[Lakehouse] Kafka commit failed after durable write: %v", err); continue }
			lm.offsetManager.MarkProcessed(idempotencyKeys[i])
			if err := lm.offsetManager.CommitOffset(topic, msg.Partition, msg.Offset); err != nil { log.Printf("[Lakehouse] offset persistence failed: %v", err) }
		}
	} else {
		// Requeue failed batches so a transient storage error cannot silently lose events.
		buffer.mu.Lock()
		buffer.events = append(events, buffer.events...)
		buffer.messages = append(messages, buffer.messages...)
		buffer.idempotencyKeys = append(idempotencyKeys, buffer.idempotencyKeys...)
		buffer.mu.Unlock()
	}

	log.Printf("[Lakehouse] Flushed %d events for topic %s", len(events), topic)
}

// writeParquetBatch writes a batch of events to a Parquet file
func (lm *ProductionLakehouseManager) writeParquetBatch(topic, partitionDate string, events []ParquetEvent) error {
	// Create partition directory
	partitionPath := filepath.Join(lm.config.DataPath, topic, partitionDate)
	if err := os.MkdirAll(partitionPath, 0755); err != nil {
		return fmt.Errorf("failed to create partition directory: %w", err)
	}

	// Generate unique filename
	filename := filepath.Join(partitionPath, fmt.Sprintf("%s-%s.parquet", time.Now().Format("150405"), uuid.New().String()[:8]))

	// Create Parquet file
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create parquet file: %w", err)
	}
	defer file.Close()

	// Write Parquet data
	writer := parquet.NewGenericWriter[ParquetEvent](file,
		parquet.Compression(&parquet.Snappy),
		parquet.PageBufferSize(256*1024),
	)

	if _, err := writer.Write(events); err != nil {
		return fmt.Errorf("failed to write parquet data: %w", err)
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("failed to close parquet writer: %w", err)
	}

	log.Printf("[Lakehouse] Wrote %d events to %s", len(events), filename)
	return nil
}

// reportMetrics periodically reports metrics
func (lm *ProductionLakehouseManager) reportMetrics() {
	defer lm.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-lm.ctx.Done():
			return
		case <-ticker.C:
			lm.metrics.mu.RLock()
			log.Printf("[Lakehouse Metrics] Ingested: %d, Written: %d, Failed: %d, Batches: %d",
				lm.metrics.EventsIngested,
				lm.metrics.EventsWritten,
				lm.metrics.EventsFailed,
				lm.metrics.BatchesWritten,
			)
			lm.metrics.mu.RUnlock()
		}
	}
}

// GetMetrics returns current metrics
func (lm *ProductionLakehouseManager) GetMetrics() LakehouseMetrics {
	lm.metrics.mu.RLock()
	defer lm.metrics.mu.RUnlock()
	return *lm.metrics
}

// Close gracefully shuts down the Lakehouse manager
func (lm *ProductionLakehouseManager) Close() error {
	log.Println("[Lakehouse] Shutting down...")

	lm.cancel()
	lm.wg.Wait()

	// Final flush for all buffers
	lm.mu.RLock()
	for topic := range lm.eventBuffers {
		lm.flushBuffer(topic)
	}
	lm.mu.RUnlock()

	// Close Kafka readers
	for topic, reader := range lm.kafkaReaders {
		log.Printf("[Lakehouse] Closing Kafka reader for topic: %s", topic)
		reader.Close()
	}

	// Save final offsets
	if err := lm.offsetManager.save(); err != nil {
		log.Printf("[Lakehouse] Error saving offsets: %v", err)
	}

	log.Println("[Lakehouse] Shutdown complete")
	return nil
}

// Package openlane provides Kafka consumer for platform events
package openlane

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/segmentio/kafka-go"
)

// KafkaEventConsumer consumes platform events from Kafka and forwards to OpenLane
type KafkaEventConsumer struct {
	reader       *kafka.Reader
	client       *Client
	topics       []string
	consumerGroup string
}

// NewKafkaEventConsumer creates a new Kafka event consumer
func NewKafkaEventConsumer() (*KafkaEventConsumer, error) {
	brokers := os.Getenv("KAFKA_BROKERS")
	if brokers == "" {
		brokers = "kafka:9092"
	}

	topics := os.Getenv("OPENLANE_KAFKA_TOPICS")
	if topics == "" {
		topics = "platform.events,beneficiary.events,disbursement.events,grievance.events,fraud.events,audit.events"
	}

	consumerGroup := os.Getenv("OPENLANE_CONSUMER_GROUP")
	if consumerGroup == "" {
		consumerGroup = "openlane-bridge"
	}

	client, err := NewClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create OpenLane client: %w", err)
	}

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        strings.Split(brokers, ","),
		GroupID:        consumerGroup,
		Topic:          strings.Split(topics, ",")[0], // Primary topic
		MinBytes:       10e3,
		MaxBytes:       10e6,
		MaxWait:        1 * time.Second,
		CommitInterval: 1 * time.Second,
		StartOffset:    kafka.LastOffset,
	})

	return &KafkaEventConsumer{
		reader:        reader,
		client:        client,
		topics:        strings.Split(topics, ","),
		consumerGroup: consumerGroup,
	}, nil
}

// Start begins consuming events from Kafka
func (c *KafkaEventConsumer) Start(ctx context.Context) error {
	log.Printf("[OpenLane Bridge] Starting Kafka consumer for topics: %v", c.topics)

	for {
		select {
		case <-ctx.Done():
			return c.reader.Close()
		default:
			msg, err := c.reader.ReadMessage(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return nil
				}
				log.Printf("[OpenLane Bridge] Error reading message: %v", err)
				continue
			}

			if err := c.processMessage(ctx, msg); err != nil {
				log.Printf("[OpenLane Bridge] Error processing message: %v", err)
			}
		}
	}
}

func (c *KafkaEventConsumer) processMessage(ctx context.Context, msg kafka.Message) error {
	var event PlatformEvent
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		return fmt.Errorf("failed to unmarshal event: %w", err)
	}

	// Extract event type from headers if not in body
	if event.EventType == "" {
		for _, header := range msg.Headers {
			if header.Key == "event_type" {
				event.EventType = string(header.Value)
				break
			}
		}
	}

	// Set source from topic if not specified
	if event.Source == "" {
		event.Source = msg.Topic
	}

	// Set timestamp from Kafka message if not specified
	if event.Timestamp.IsZero() {
		event.Timestamp = msg.Time
	}

	log.Printf("[OpenLane Bridge] Processing event: %s from %s", event.EventType, event.Source)

	return c.client.ProcessPlatformEvent(ctx, &event)
}

// Close closes the Kafka consumer
func (c *KafkaEventConsumer) Close() error {
	return c.reader.Close()
}

// PublishPlatformEvent publishes an event to Kafka for OpenLane processing
func PublishPlatformEvent(ctx context.Context, event *PlatformEvent) error {
	brokers := os.Getenv("KAFKA_BROKERS")
	if brokers == "" {
		brokers = "kafka:9092"
	}

	topic := os.Getenv("OPENLANE_EVENTS_TOPIC")
	if topic == "" {
		topic = "platform.events"
	}

	writer := &kafka.Writer{
		Addr:         kafka.TCP(strings.Split(brokers, ",")...),
		Topic:        topic,
		Balancer:     &kafka.LeastBytes{},
		BatchTimeout: 10 * time.Millisecond,
	}
	defer writer.Close()

	eventBytes, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	return writer.WriteMessages(ctx, kafka.Message{
		Key:   []byte(event.ResourceID),
		Value: eventBytes,
		Headers: []kafka.Header{
			{Key: "event_type", Value: []byte(event.EventType)},
			{Key: "source", Value: []byte(event.Source)},
		},
		Time: event.Timestamp,
	})
}

// SocialProtectionControls defines NIST 800-53 controls relevant to social protection
var SocialProtectionControls = []Control{
	// Access Control
	{ID: "AC-2", Name: "Account Management", Standard: "NIST-800-53", Category: "Access Control"},
	{ID: "AC-3", Name: "Access Enforcement", Standard: "NIST-800-53", Category: "Access Control"},
	{ID: "AC-6", Name: "Least Privilege", Standard: "NIST-800-53", Category: "Access Control"},
	
	// Audit and Accountability
	{ID: "AU-2", Name: "Audit Events", Standard: "NIST-800-53", Category: "Audit"},
	{ID: "AU-3", Name: "Content of Audit Records", Standard: "NIST-800-53", Category: "Audit"},
	{ID: "AU-6", Name: "Audit Review, Analysis, and Reporting", Standard: "NIST-800-53", Category: "Audit"},
	
	// Identification and Authentication
	{ID: "IA-2", Name: "Identification and Authentication", Standard: "NIST-800-53", Category: "Identity"},
	{ID: "IA-5", Name: "Authenticator Management", Standard: "NIST-800-53", Category: "Identity"},
	
	// Incident Response
	{ID: "IR-4", Name: "Incident Handling", Standard: "NIST-800-53", Category: "Incident Response"},
	{ID: "IR-5", Name: "Incident Monitoring", Standard: "NIST-800-53", Category: "Incident Response"},
	
	// System and Information Integrity
	{ID: "SI-2", Name: "Flaw Remediation", Standard: "NIST-800-53", Category: "System Integrity"},
	{ID: "SI-4", Name: "Information System Monitoring", Standard: "NIST-800-53", Category: "System Integrity"},
	
	// Contingency Planning
	{ID: "CP-9", Name: "Information System Backup", Standard: "NIST-800-53", Category: "Contingency"},
	{ID: "CP-10", Name: "Information System Recovery", Standard: "NIST-800-53", Category: "Contingency"},
	
	// Risk Assessment
	{ID: "RA-5", Name: "Vulnerability Scanning", Standard: "NIST-800-53", Category: "Risk Assessment"},
	
	// Data Protection (custom for social protection)
	{ID: "SP-1", Name: "Beneficiary Data Protection", Standard: "Social-Protection", Category: "Data Protection"},
	{ID: "SP-2", Name: "Payment Integrity", Standard: "Social-Protection", Category: "Financial Controls"},
	{ID: "SP-3", Name: "Grievance Management", Standard: "Social-Protection", Category: "Accountability"},
	{ID: "SP-4", Name: "Fraud Prevention", Standard: "Social-Protection", Category: "Financial Controls"},
	{ID: "SP-5", Name: "Program Eligibility Verification", Standard: "Social-Protection", Category: "Program Integrity"},
}

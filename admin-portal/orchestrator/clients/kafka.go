package clients

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/segmentio/kafka-go"
)

type KafkaClient struct {
	writer *kafka.Writer
	readers map[string]*kafka.Reader
	brokers []string
}

func NewKafkaClient(brokers []string) *KafkaClient {
	return &KafkaClient{
		writer: &kafka.Writer{
			Addr:         kafka.TCP(brokers...),
			Balancer:     &kafka.LeastBytes{},
			RequiredAcks: kafka.RequireAll,
			Async:        false,
		},
		readers: make(map[string]*kafka.Reader),
		brokers: brokers,
	}
}

func (k *KafkaClient) PublishEvent(ctx context.Context, topic string, key string, value interface{}) error {
	valueBytes, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	msg := kafka.Message{
		Topic: topic,
		Key:   []byte(key),
		Value: valueBytes,
		Time:  time.Now(),
	}

	return k.writer.WriteMessages(ctx, msg)
}

func (k *KafkaClient) Subscribe(topic string, groupID string) *kafka.Reader {
	if reader, exists := k.readers[topic]; exists {
		return reader
	}

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  k.brokers,
		Topic:    topic,
		GroupID:  groupID,
		MinBytes: 10e3, // 10KB
		MaxBytes: 10e6, // 10MB
	})

	k.readers[topic] = reader
	return reader
}

func (k *KafkaClient) Close() error {
	if err := k.writer.Close(); err != nil {
		return err
	}

	for _, reader := range k.readers {
		if err := reader.Close(); err != nil {
			return err
		}
	}

	return nil
}

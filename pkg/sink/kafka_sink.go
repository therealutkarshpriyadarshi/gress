package sink

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/therealutkarshpriyadarshi/gress/pkg/stream"
	"go.uber.org/zap"
)

// KafkaSink writes events to Kafka topics
type KafkaSink struct {
	producer *kafka.Producer
	topic    string
	logger   *zap.Logger
}

// NewKafkaSink creates a new Kafka sink
func NewKafkaSink(brokers []string, topic string, logger *zap.Logger) (*KafkaSink, error) {
	if len(brokers) == 0 {
		return nil, fmt.Errorf("no Kafka brokers specified")
	}

	config := &kafka.ConfigMap{
		"bootstrap.servers": brokers[0],
		"acks":              "all",
		"retries":           3,
		"idempotence":       true,
	}

	producer, err := kafka.NewProducer(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kafka producer: %w", err)
	}

	// Handle delivery reports in background
	go func() {
		for e := range producer.Events() {
			switch ev := e.(type) {
			case *kafka.Message:
				if ev.TopicPartition.Error != nil {
					logger.Error("Kafka delivery failed",
						zap.Error(ev.TopicPartition.Error))
				}
			}
		}
	}()

	return &KafkaSink{
		producer: producer,
		topic:    topic,
		logger:   logger,
	}, nil
}

// Write sends an event to Kafka
func (k *KafkaSink) Write(ctx context.Context, event *stream.Event) error {
	valueBytes, err := json.Marshal(event.Value)
	if err != nil {
		return fmt.Errorf("failed to marshal event value: %w", err)
	}

	headers := make([]kafka.Header, 0, len(event.Headers))
	for key, value := range event.Headers {
		headers = append(headers, kafka.Header{
			Key:   key,
			Value: []byte(value),
		})
	}

	msg := &kafka.Message{
		TopicPartition: kafka.TopicPartition{
			Topic:     &k.topic,
			Partition: kafka.PartitionAny,
		},
		Key:     []byte(event.Key),
		Value:   valueBytes,
		Headers: headers,
	}

	return k.producer.Produce(msg, nil)
}

// Flush ensures all messages are sent
func (k *KafkaSink) Flush(ctx context.Context) error {
	remaining := k.producer.Flush(30000) // 30 second timeout
	if remaining > 0 {
		k.logger.Warn("Messages still in queue", zap.Int("count", remaining))
	}
	return nil
}

// Close closes the producer
func (k *KafkaSink) Close() error {
	k.logger.Info("Closing Kafka sink")
	k.producer.Close()
	return nil
}

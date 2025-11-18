package sink

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/therealutkarshpriyadarshi/gress/pkg/config"
	"github.com/therealutkarshpriyadarshi/gress/pkg/schema"
	"github.com/therealutkarshpriyadarshi/gress/pkg/stream"
	"go.uber.org/zap"
)

// KafkaSink writes events to Kafka topics
type KafkaSink struct {
	producer     *kafka.Producer
	topic        string
	logger       *zap.Logger
	codecManager *schema.CodecManager
	valueSchema  *config.SinkSchemaConfig
	keySchema    *config.SinkSchemaConfig
}

// KafkaSinkConfig holds Kafka sink configuration
type KafkaSinkConfig struct {
	Brokers      []string
	Topic        string
	CodecManager *schema.CodecManager
	ValueSchema  *config.SinkSchemaConfig
	KeySchema    *config.SinkSchemaConfig
}

// NewKafkaSink creates a new Kafka sink
func NewKafkaSink(cfg KafkaSinkConfig, logger *zap.Logger) (*KafkaSink, error) {
	brokers := cfg.Brokers
	topic := cfg.Topic
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
		producer:     producer,
		topic:        topic,
		logger:       logger,
		codecManager: cfg.CodecManager,
		valueSchema:  cfg.ValueSchema,
		keySchema:    cfg.KeySchema,
	}, nil
}

// Write sends an event to Kafka
func (k *KafkaSink) Write(ctx context.Context, event *stream.Event) error {
	var valueBytes []byte
	var keyBytes []byte
	var err error

	// Encode value using schema if configured
	if k.codecManager != nil && k.valueSchema != nil {
		schemaConfig := &schema.SchemaConfig{
			Subject:         k.valueSchema.Subject,
			SchemaType:      schema.SchemaType(k.valueSchema.SchemaType),
			Version:         k.valueSchema.Version,
			AutoRegister:    k.valueSchema.AutoRegister,
			Compatibility:   schema.CompatibilityMode(k.valueSchema.Compatibility),
			ValidateOnWrite: k.valueSchema.ValidateOnWrite,
		}

		valueBytes, err = k.codecManager.Encode(ctx, k.valueSchema.Subject, event.Value, schemaConfig)
		if err != nil {
			k.logger.Warn("Failed to encode value with schema, falling back to JSON",
				zap.Error(err),
				zap.String("subject", k.valueSchema.Subject),
			)
			// Fallback to JSON
			valueBytes, err = json.Marshal(event.Value)
			if err != nil {
				return fmt.Errorf("failed to marshal event value: %w", err)
			}
		}
	} else {
		// Original behavior: use JSON
		valueBytes, err = json.Marshal(event.Value)
		if err != nil {
			return fmt.Errorf("failed to marshal event value: %w", err)
		}
	}

	// Encode key using schema if configured
	if k.codecManager != nil && k.keySchema != nil && event.Key != nil {
		schemaConfig := &schema.SchemaConfig{
			Subject:         k.keySchema.Subject,
			SchemaType:      schema.SchemaType(k.keySchema.SchemaType),
			Version:         k.keySchema.Version,
			AutoRegister:    k.keySchema.AutoRegister,
			Compatibility:   schema.CompatibilityMode(k.keySchema.Compatibility),
			ValidateOnWrite: k.keySchema.ValidateOnWrite,
		}

		keyBytes, err = k.codecManager.Encode(ctx, k.keySchema.Subject, event.Key, schemaConfig)
		if err != nil {
			k.logger.Warn("Failed to encode key with schema, using string",
				zap.Error(err),
				zap.String("subject", k.keySchema.Subject),
			)
			// Fallback to string conversion
			if keyStr, ok := event.Key.(string); ok {
				keyBytes = []byte(keyStr)
			} else {
				keyBytes, _ = json.Marshal(event.Key)
			}
		}
	} else {
		// Original behavior: convert to string
		if event.Key != nil {
			if keyStr, ok := event.Key.(string); ok {
				keyBytes = []byte(keyStr)
			} else {
				keyBytes, _ = json.Marshal(event.Key)
			}
		}
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
		Key:     keyBytes,
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

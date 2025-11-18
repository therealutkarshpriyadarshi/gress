package ingestion

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/therealutkarshpriyadarshi/gress/pkg/config"
	"github.com/therealutkarshpriyadarshi/gress/pkg/schema"
	"github.com/therealutkarshpriyadarshi/gress/pkg/stream"
	"go.uber.org/zap"
)

// KafkaSource ingests events from Kafka topics
type KafkaSource struct {
	brokers      []string
	topics       []string
	groupID      string
	consumer     *kafka.Consumer
	logger       *zap.Logger
	partitions   int32
	codecManager *schema.CodecManager
	valueSchema  *config.SourceSchemaConfig
	keySchema    *config.SourceSchemaConfig
}

// KafkaSourceConfig holds Kafka source configuration
type KafkaSourceConfig struct {
	Brokers         []string
	Topics          []string
	GroupID         string
	AutoOffsetReset string
	EnableAutoCommit bool
	CodecManager    *schema.CodecManager
	ValueSchema     *config.SourceSchemaConfig
	KeySchema       *config.SourceSchemaConfig
}

// NewKafkaSource creates a new Kafka source
func NewKafkaSource(config KafkaSourceConfig, logger *zap.Logger) (*KafkaSource, error) {
	if len(config.Brokers) == 0 {
		return nil, fmt.Errorf("no Kafka brokers specified")
	}
	if len(config.Topics) == 0 {
		return nil, fmt.Errorf("no Kafka topics specified")
	}
	if config.GroupID == "" {
		config.GroupID = "gress-consumer-group"
	}
	if config.AutoOffsetReset == "" {
		config.AutoOffsetReset = "earliest"
	}

	kafkaConfig := &kafka.ConfigMap{
		"bootstrap.servers":  config.Brokers[0],
		"group.id":           config.GroupID,
		"auto.offset.reset":  config.AutoOffsetReset,
		"enable.auto.commit": config.EnableAutoCommit,
	}

	consumer, err := kafka.NewConsumer(kafkaConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kafka consumer: %w", err)
	}

	return &KafkaSource{
		brokers:      config.Brokers,
		topics:       config.Topics,
		groupID:      config.GroupID,
		consumer:     consumer,
		logger:       logger,
		partitions:   0,
		codecManager: config.CodecManager,
		valueSchema:  config.ValueSchema,
		keySchema:    config.KeySchema,
	}, nil
}

// Start begins consuming events from Kafka
func (k *KafkaSource) Start(ctx context.Context, output chan<- *stream.Event) error {
	k.logger.Info("Starting Kafka source",
		zap.Strings("brokers", k.brokers),
		zap.Strings("topics", k.topics),
		zap.String("group_id", k.groupID))

	if err := k.consumer.SubscribeTopics(k.topics, nil); err != nil {
		return fmt.Errorf("failed to subscribe to topics: %w", err)
	}

	// Get partition count (simplified - use first topic)
	metadata, err := k.consumer.GetMetadata(&k.topics[0], false, 5000)
	if err == nil {
		if topicMeta, ok := metadata.Topics[k.topics[0]]; ok {
			k.partitions = int32(len(topicMeta.Partitions))
			k.logger.Info("Detected partitions", zap.Int32("count", k.partitions))
		}
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				k.logger.Info("Kafka source stopping")
				return
			default:
				msg, err := k.consumer.ReadMessage(100 * time.Millisecond)
				if err != nil {
					if kafkaErr, ok := err.(kafka.Error); ok {
						if kafkaErr.Code() == kafka.ErrTimedOut {
							continue
						}
					}
					k.logger.Error("Error reading Kafka message", zap.Error(err))
					continue
				}

				event := k.messageToEvent(msg)
				select {
				case output <- event:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return nil
}

// messageToEvent converts a Kafka message to a stream event
func (k *KafkaSource) messageToEvent(msg *kafka.Message) *stream.Event {
	var value interface{}
	var key interface{}

	// Decode value using schema if configured
	if k.codecManager != nil && k.valueSchema != nil {
		schemaConfig := &schema.SchemaConfig{
			Subject:        k.valueSchema.Subject,
			SchemaType:     schema.SchemaType(k.valueSchema.SchemaType),
			Version:        k.valueSchema.Version,
			ValidateOnRead: k.valueSchema.ValidateOnRead,
		}

		decoded, err := k.codecManager.Decode(context.Background(), msg.Value, schemaConfig)
		if err != nil {
			k.logger.Warn("Failed to decode value with schema, falling back to JSON",
				zap.Error(err),
				zap.String("subject", k.valueSchema.Subject),
			)
			// Fallback to JSON
			if err := json.Unmarshal(msg.Value, &value); err != nil {
				value = msg.Value
			} else {
				value = decoded
			}
		} else {
			value = decoded
		}
	} else {
		// Original behavior: try JSON, fallback to raw bytes
		if err := json.Unmarshal(msg.Value, &value); err != nil {
			value = msg.Value
		}
	}

	// Decode key using schema if configured
	if k.codecManager != nil && k.keySchema != nil && len(msg.Key) > 0 {
		schemaConfig := &schema.SchemaConfig{
			Subject:        k.keySchema.Subject,
			SchemaType:     schema.SchemaType(k.keySchema.SchemaType),
			Version:        k.keySchema.Version,
			ValidateOnRead: k.keySchema.ValidateOnRead,
		}

		decoded, err := k.codecManager.Decode(context.Background(), msg.Key, schemaConfig)
		if err != nil {
			k.logger.Warn("Failed to decode key with schema, using string",
				zap.Error(err),
				zap.String("subject", k.keySchema.Subject),
			)
			key = string(msg.Key)
		} else {
			key = decoded
		}
	} else {
		// Original behavior: convert to string
		if len(msg.Key) > 0 {
			key = string(msg.Key)
		}
	}

	headers := make(map[string]string)
	for _, h := range msg.Headers {
		headers[h.Key] = string(h.Value)
	}

	event := &stream.Event{
		Key:       key,
		Value:     value,
		EventTime: msg.Timestamp,
		Headers:   headers,
		Offset:    int64(msg.TopicPartition.Offset),
		Partition: msg.TopicPartition.Partition,
	}

	return event
}

// Stop stops the Kafka consumer
func (k *KafkaSource) Stop() error {
	k.logger.Info("Stopping Kafka source")
	return k.consumer.Close()
}

// Name returns the source name
func (k *KafkaSource) Name() string {
	return fmt.Sprintf("kafka-%s", k.topics[0])
}

// Partitions returns the number of partitions
func (k *KafkaSource) Partitions() int32 {
	return k.partitions
}

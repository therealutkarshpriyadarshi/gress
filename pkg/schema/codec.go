package schema

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap"
)

var (
	// Prometheus metrics
	schemaOperations = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gress_schema_operations_total",
			Help: "Total number of schema operations",
		},
		[]string{"operation", "schema_type", "status"},
	)

	schemaOperationDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "gress_schema_operation_duration_seconds",
			Help:    "Duration of schema operations",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"operation", "schema_type"},
	)

	schemaCacheSize = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "gress_schema_cache_size",
			Help: "Number of cached schemas",
		},
		[]string{"codec_type"},
	)
)

// CodecManager manages multiple codecs and provides a unified interface
type CodecManager struct {
	registry RegistryClient
	logger   *zap.Logger

	// Codecs by type
	codecs map[SchemaType]Codec

	// Schema cache
	schemaCache      map[string]*SchemaMetadata // subject -> metadata
	schemaCacheMutex sync.RWMutex

	// Metrics
	stats struct {
		sync.RWMutex
		encodes  int64
		decodes  int64
		errors   int64
		duration time.Duration
	}
}

// NewCodecManager creates a new codec manager
func NewCodecManager(registry RegistryClient, logger *zap.Logger) *CodecManager {
	cm := &CodecManager{
		registry:    registry,
		logger:      logger,
		codecs:      make(map[SchemaType]Codec),
		schemaCache: make(map[string]*SchemaMetadata),
	}

	// Register codecs
	cm.codecs[SchemaTypeAvro] = NewAvroCodec(registry, logger)
	cm.codecs[SchemaTypeProtobuf] = NewProtobufCodec(registry, logger)
	cm.codecs[SchemaTypeJSON] = NewJSONSchemaCodec(registry, logger)

	logger.Info("Codec manager initialized",
		zap.Int("codec_count", len(cm.codecs)),
	)

	return cm
}

// Encode encodes data using the appropriate codec
func (cm *CodecManager) Encode(ctx context.Context, subject string, data interface{}, config *SchemaConfig) ([]byte, error) {
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		cm.recordDuration(duration)
		schemaOperationDuration.WithLabelValues("encode", string(config.SchemaType)).Observe(duration.Seconds())
	}()

	// Get schema metadata
	metadata, err := cm.getSchemaMetadata(ctx, subject, config)
	if err != nil {
		cm.recordError()
		schemaOperations.WithLabelValues("encode", string(config.SchemaType), "error").Inc()
		return nil, fmt.Errorf("failed to get schema metadata: %w", err)
	}

	// Get codec
	codec, err := cm.getCodec(metadata.SchemaType)
	if err != nil {
		cm.recordError()
		schemaOperations.WithLabelValues("encode", string(config.SchemaType), "error").Inc()
		return nil, err
	}

	// Encode
	encoded, err := codec.Encode(data, metadata)
	if err != nil {
		cm.recordError()
		schemaOperations.WithLabelValues("encode", string(config.SchemaType), "error").Inc()
		return nil, fmt.Errorf("failed to encode: %w", err)
	}

	cm.recordEncode()
	schemaOperations.WithLabelValues("encode", string(config.SchemaType), "success").Inc()

	return encoded, nil
}

// Decode decodes data using the appropriate codec
func (cm *CodecManager) Decode(ctx context.Context, data []byte, config *SchemaConfig) (interface{}, error) {
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		cm.recordDuration(duration)
		schemaOperationDuration.WithLabelValues("decode", string(config.SchemaType)).Observe(duration.Seconds())
	}()

	// Decode will automatically fetch schema from registry based on schema ID in the data
	// For now, we'll try each codec until one works
	var lastErr error

	// Try specific codec if schema type is known
	if config.SchemaType != "" {
		codec, err := cm.getCodec(config.SchemaType)
		if err == nil {
			decoded, err := codec.Decode(data, nil)
			if err == nil {
				cm.recordDecode()
				schemaOperations.WithLabelValues("decode", string(config.SchemaType), "success").Inc()
				return decoded, nil
			}
			lastErr = err
		}
	}

	// Try all codecs
	for schemaType, codec := range cm.codecs {
		decoded, err := codec.Decode(data, nil)
		if err == nil {
			cm.recordDecode()
			schemaOperations.WithLabelValues("decode", string(schemaType), "success").Inc()
			return decoded, nil
		}
		lastErr = err
	}

	cm.recordError()
	schemaOperations.WithLabelValues("decode", "unknown", "error").Inc()
	return nil, fmt.Errorf("failed to decode with any codec: %w", lastErr)
}

// ValidateCompatibility validates schema compatibility
func (cm *CodecManager) ValidateCompatibility(ctx context.Context, subject string, schema string, schemaType SchemaType, mode CompatibilityMode) (bool, error) {
	start := time.Now()
	defer func() {
		schemaOperationDuration.WithLabelValues("compatibility_check", string(schemaType)).Observe(time.Since(start).Seconds())
	}()

	// Get latest version
	versions, err := cm.registry.GetVersions(ctx, subject)
	if err != nil {
		schemaOperations.WithLabelValues("compatibility_check", string(schemaType), "error").Inc()
		return false, fmt.Errorf("failed to get versions: %w", err)
	}

	if len(versions) == 0 {
		// No previous versions, always compatible
		schemaOperations.WithLabelValues("compatibility_check", string(schemaType), "success").Inc()
		return true, nil
	}

	// Test compatibility based on mode
	var versionsToCheck []int

	switch mode {
	case CompatibilityBackward, CompatibilityForward, CompatibilityFull:
		// Check against latest version only
		versionsToCheck = []int{versions[len(versions)-1]}

	case CompatibilityBackwardTransitive, CompatibilityForwardTransitive, CompatibilityFullTransitive:
		// Check against all versions
		versionsToCheck = versions

	case CompatibilityNone:
		// No compatibility check needed
		schemaOperations.WithLabelValues("compatibility_check", string(schemaType), "success").Inc()
		return true, nil

	default:
		schemaOperations.WithLabelValues("compatibility_check", string(schemaType), "error").Inc()
		return false, fmt.Errorf("unsupported compatibility mode: %s", mode)
	}

	// Test compatibility with each version
	for _, version := range versionsToCheck {
		compatible, err := cm.registry.TestCompatibility(ctx, subject, schema, schemaType, version)
		if err != nil {
			schemaOperations.WithLabelValues("compatibility_check", string(schemaType), "error").Inc()
			return false, fmt.Errorf("failed to test compatibility with version %d: %w", version, err)
		}

		if !compatible {
			schemaOperations.WithLabelValues("compatibility_check", string(schemaType), "incompatible").Inc()
			return false, nil
		}
	}

	schemaOperations.WithLabelValues("compatibility_check", string(schemaType), "success").Inc()
	return true, nil
}

// RegisterSchema registers a new schema with compatibility checking
func (cm *CodecManager) RegisterSchema(ctx context.Context, subject string, schema string, config *SchemaConfig) (*SchemaMetadata, error) {
	start := time.Now()
	defer func() {
		schemaOperationDuration.WithLabelValues("register", string(config.SchemaType)).Observe(time.Since(start).Seconds())
	}()

	// Validate schema format
	if err := cm.validateSchemaFormat(schema, config.SchemaType); err != nil {
		schemaOperations.WithLabelValues("register", string(config.SchemaType), "error").Inc()
		return nil, fmt.Errorf("invalid schema format: %w", err)
	}

	// Check compatibility if required
	if config.Compatibility != CompatibilityNone {
		compatible, err := cm.ValidateCompatibility(ctx, subject, schema, config.SchemaType, config.Compatibility)
		if err != nil {
			schemaOperations.WithLabelValues("register", string(config.SchemaType), "error").Inc()
			return nil, fmt.Errorf("compatibility check failed: %w", err)
		}

		if !compatible {
			schemaOperations.WithLabelValues("register", string(config.SchemaType), "incompatible").Inc()
			return nil, fmt.Errorf("schema is not compatible with existing versions (mode: %s)", config.Compatibility)
		}
	}

	// Register schema
	metadata, err := cm.registry.RegisterSchema(ctx, subject, schema, config.SchemaType, nil)
	if err != nil {
		schemaOperations.WithLabelValues("register", string(config.SchemaType), "error").Inc()
		return nil, fmt.Errorf("failed to register schema: %w", err)
	}

	// Cache metadata
	cm.cacheSchemaMetadata(subject, metadata)

	schemaOperations.WithLabelValues("register", string(config.SchemaType), "success").Inc()

	cm.logger.Info("Schema registered",
		zap.String("subject", subject),
		zap.Int("schema_id", metadata.ID),
		zap.Int("version", metadata.Version),
		zap.String("schema_type", string(config.SchemaType)),
	)

	return metadata, nil
}

// getSchemaMetadata retrieves schema metadata from cache or registry
func (cm *CodecManager) getSchemaMetadata(ctx context.Context, subject string, config *SchemaConfig) (*SchemaMetadata, error) {
	// Check cache first
	cacheKey := fmt.Sprintf("%s:%d", subject, config.Version)

	cm.schemaCacheMutex.RLock()
	if metadata, exists := cm.schemaCache[cacheKey]; exists {
		cm.schemaCacheMutex.RUnlock()
		return metadata, nil
	}
	cm.schemaCacheMutex.RUnlock()

	// Fetch from registry
	var metadata *SchemaMetadata
	var err error

	if config.Version > 0 {
		metadata, err = cm.registry.GetSchemaByVersion(ctx, subject, config.Version)
	} else {
		metadata, err = cm.registry.GetLatestSchema(ctx, subject)
	}

	if err != nil {
		return nil, err
	}

	// Cache it
	cm.cacheSchemaMetadata(cacheKey, metadata)

	return metadata, nil
}

// cacheSchemaMetadata caches schema metadata
func (cm *CodecManager) cacheSchemaMetadata(key string, metadata *SchemaMetadata) {
	cm.schemaCacheMutex.Lock()
	cm.schemaCache[key] = metadata
	schemaCacheSize.WithLabelValues(string(metadata.SchemaType)).Set(float64(len(cm.schemaCache)))
	cm.schemaCacheMutex.Unlock()
}

// getCodec retrieves a codec for the given schema type
func (cm *CodecManager) getCodec(schemaType SchemaType) (Codec, error) {
	codec, exists := cm.codecs[schemaType]
	if !exists {
		return nil, fmt.Errorf("unsupported schema type: %s", schemaType)
	}
	return codec, nil
}

// validateSchemaFormat validates the schema format
func (cm *CodecManager) validateSchemaFormat(schema string, schemaType SchemaType) error {
	switch schemaType {
	case SchemaTypeAvro:
		return ValidateAvroSchema(schema)
	case SchemaTypeProtobuf:
		return ValidateProtobufSchema([]byte(schema))
	case SchemaTypeJSON:
		return ValidateJSONSchema(schema)
	default:
		return fmt.Errorf("unsupported schema type: %s", schemaType)
	}
}

// GetRegistry returns the underlying registry client
func (cm *CodecManager) GetRegistry() RegistryClient {
	return cm.registry
}

// ClearCache clears the schema cache
func (cm *CodecManager) ClearCache() {
	cm.schemaCacheMutex.Lock()
	cm.schemaCache = make(map[string]*SchemaMetadata)
	schemaCacheSize.WithLabelValues("all").Set(0)
	cm.schemaCacheMutex.Unlock()

	cm.logger.Info("Schema cache cleared")
}

// GetStats returns codec manager statistics
func (cm *CodecManager) GetStats() map[string]interface{} {
	cm.stats.RLock()
	defer cm.stats.RUnlock()

	return map[string]interface{}{
		"encodes":  cm.stats.encodes,
		"decodes":  cm.stats.decodes,
		"errors":   cm.stats.errors,
		"duration": cm.stats.duration.String(),
	}
}

// Metrics recording methods

func (cm *CodecManager) recordEncode() {
	cm.stats.Lock()
	cm.stats.encodes++
	cm.stats.Unlock()
}

func (cm *CodecManager) recordDecode() {
	cm.stats.Lock()
	cm.stats.decodes++
	cm.stats.Unlock()
}

func (cm *CodecManager) recordError() {
	cm.stats.Lock()
	cm.stats.errors++
	cm.stats.Unlock()
}

func (cm *CodecManager) recordDuration(d time.Duration) {
	cm.stats.Lock()
	cm.stats.duration += d
	cm.stats.Unlock()
}

// Close closes the codec manager and all codecs
func (cm *CodecManager) Close() error {
	cm.logger.Info("Closing codec manager",
		zap.Any("stats", cm.GetStats()),
	)

	return cm.registry.Close()
}

// Helper functions

// MustNewCodecManager creates a codec manager or panics
func MustNewCodecManager(config *Config, logger *zap.Logger) *CodecManager {
	registry, err := NewRegistryClient(config, logger)
	if err != nil {
		panic(fmt.Sprintf("failed to create registry client: %v", err))
	}

	return NewCodecManager(registry, logger)
}

// IsSchemaRegistryError checks if an error is from the schema registry
func IsSchemaRegistryError(err error) bool {
	return err != nil && (errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled))
}

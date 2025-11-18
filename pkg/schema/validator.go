package schema

import (
	"context"
	"fmt"

	"github.com/therealutkarshpriyadarshi/gress/pkg/stream"
	"go.uber.org/zap"
)

// ValidatorOperator validates events against a schema
type ValidatorOperator struct {
	codecManager *CodecManager
	config       *SchemaConfig
	logger       *zap.Logger
	stats        struct {
		valid   int64
		invalid int64
	}
}

// NewValidatorOperator creates a new schema validation operator
func NewValidatorOperator(codecManager *CodecManager, config *SchemaConfig, logger *zap.Logger) *ValidatorOperator {
	return &ValidatorOperator{
		codecManager: codecManager,
		config:       config,
		logger:       logger,
	}
}

// Validate validates an event against the configured schema
func (v *ValidatorOperator) Validate(ctx context.Context, event *stream.Event) (*stream.Event, error) {
	// Try to encode and decode to validate
	// Encoding validates the structure and types
	encoded, err := v.codecManager.Encode(ctx, v.config.Subject, event.Value, v.config)
	if err != nil {
		v.stats.invalid++
		v.logger.Warn("Schema validation failed",
			zap.Error(err),
			zap.String("subject", v.config.Subject),
			zap.Any("event_key", event.Key),
		)
		return nil, fmt.Errorf("schema validation failed: %w", err)
	}

	// Decode back to ensure round-trip works
	decoded, err := v.codecManager.Decode(ctx, encoded, v.config)
	if err != nil {
		v.stats.invalid++
		return nil, fmt.Errorf("schema decode validation failed: %w", err)
	}

	v.stats.valid++

	// Update event with validated/normalized value
	event.Value = decoded

	return event, nil
}

// ValidateOrFilter validates and returns the event, or nil if invalid (for filtering)
func (v *ValidatorOperator) ValidateOrFilter(ctx context.Context, event *stream.Event) *stream.Event {
	validated, err := v.Validate(ctx, event)
	if err != nil {
		return nil
	}
	return validated
}

// GetStats returns validation statistics
func (v *ValidatorOperator) GetStats() map[string]int64 {
	return map[string]int64{
		"valid":   v.stats.valid,
		"invalid": v.stats.invalid,
	}
}

// SchemaEvolutionValidator validates schema evolution compatibility
type SchemaEvolutionValidator struct {
	codecManager *CodecManager
	logger       *zap.Logger
}

// NewSchemaEvolutionValidator creates a new schema evolution validator
func NewSchemaEvolutionValidator(codecManager *CodecManager, logger *zap.Logger) *SchemaEvolutionValidator {
	return &SchemaEvolutionValidator{
		codecManager: codecManager,
		logger:       logger,
	}
}

// ValidateEvolution validates that a new schema is compatible with existing versions
func (v *SchemaEvolutionValidator) ValidateEvolution(ctx context.Context, subject string, newSchema string, schemaType SchemaType, mode CompatibilityMode) error {
	compatible, err := v.codecManager.ValidateCompatibility(ctx, subject, newSchema, schemaType, mode)
	if err != nil {
		return fmt.Errorf("failed to check compatibility: %w", err)
	}

	if !compatible {
		return fmt.Errorf("schema is not compatible with existing versions (mode: %s)", mode)
	}

	v.logger.Info("Schema evolution validated successfully",
		zap.String("subject", subject),
		zap.String("schema_type", string(schemaType)),
		zap.String("compatibility_mode", string(mode)),
	)

	return nil
}

// ValidateAndRegister validates and registers a new schema if compatible
func (v *SchemaEvolutionValidator) ValidateAndRegister(ctx context.Context, subject string, newSchema string, config *SchemaConfig) (*SchemaMetadata, error) {
	// Validate compatibility
	if err := v.ValidateEvolution(ctx, subject, newSchema, config.SchemaType, config.Compatibility); err != nil {
		return nil, err
	}

	// Register the schema
	metadata, err := v.codecManager.RegisterSchema(ctx, subject, newSchema, config)
	if err != nil {
		return nil, fmt.Errorf("failed to register schema: %w", err)
	}

	v.logger.Info("Schema registered successfully",
		zap.String("subject", subject),
		zap.Int("schema_id", metadata.ID),
		zap.Int("version", metadata.Version),
	)

	return metadata, nil
}

// MultiSchemaValidator validates events that can match multiple schemas
type MultiSchemaValidator struct {
	validators []*ValidatorOperator
	logger     *zap.Logger
}

// NewMultiSchemaValidator creates a validator that tries multiple schemas
func NewMultiSchemaValidator(validators []*ValidatorOperator, logger *zap.Logger) *MultiSchemaValidator {
	return &MultiSchemaValidator{
		validators: validators,
		logger:     logger,
	}
}

// Validate tries to validate against any of the configured schemas
func (m *MultiSchemaValidator) Validate(ctx context.Context, event *stream.Event) (*stream.Event, error) {
	var lastErr error

	for _, validator := range m.validators {
		validated, err := validator.Validate(ctx, event)
		if err == nil {
			return validated, nil
		}
		lastErr = err
	}

	return nil, fmt.Errorf("event did not match any schema: %w", lastErr)
}

// GetStats returns aggregated statistics from all validators
func (m *MultiSchemaValidator) GetStats() map[string]interface{} {
	stats := make(map[string]interface{})
	for i, validator := range m.validators {
		stats[fmt.Sprintf("validator_%d", i)] = validator.GetStats()
	}
	return stats
}

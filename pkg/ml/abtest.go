package ml

import (
	"context"
	"crypto/md5"
	"encoding/binary"
	"fmt"
	"hash/fnv"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/therealutkarshpriyadarshi/gress/pkg/stream"
)

// ABTestStrategy defines the strategy for A/B testing
type ABTestStrategy string

const (
	ABTestStrategyRandom     ABTestStrategy = "random"
	ABTestStrategyHash       ABTestStrategy = "hash"
	ABTestStrategyPercentage ABTestStrategy = "percentage"
	ABTestStrategyUserID     ABTestStrategy = "user_id"
)

// ModelVariant represents a model variant in an A/B test
type ModelVariant struct {
	Name         string  `yaml:"name" json:"name"`
	ModelName    string  `yaml:"model_name" json:"model_name"`
	ModelVersion string  `yaml:"model_version" json:"model_version"`
	Weight       float64 `yaml:"weight" json:"weight"` // Traffic percentage (0-100)
	Enabled      bool    `yaml:"enabled" json:"enabled"`
}

// ABTestConfig configures A/B testing for models
type ABTestConfig struct {
	Name              string         `yaml:"name" json:"name"`
	Strategy          ABTestStrategy `yaml:"strategy" json:"strategy"`
	Variants          []ModelVariant `yaml:"variants" json:"variants"`
	SplitKey          string         `yaml:"split_key" json:"split_key"` // Field to use for splitting (e.g., "user_id")
	DefaultVariant    string         `yaml:"default_variant" json:"default_variant"`
	EnableShadowMode  bool           `yaml:"enable_shadow_mode" json:"enable_shadow_mode"`   // Run all variants but only return one
	CollectComparison bool           `yaml:"collect_comparison" json:"collect_comparison"` // Compare predictions across variants
}

// ABTestOperator performs A/B testing with multiple model variants
type ABTestOperator struct {
	config   ABTestConfig
	registry *ModelRegistry
	variants map[string]*variantInfo
	metrics  *ABTestMetrics

	mu sync.RWMutex
}

type variantInfo struct {
	config ModelVariant
	model  Model
}

// ABTestMetrics tracks A/B test metrics
type ABTestMetrics struct {
	variantSelections *prometheus.CounterVec
	variantLatency    *prometheus.HistogramVec
	variantErrors     *prometheus.CounterVec
	shadowComparisons *prometheus.CounterVec
}

// NewABTestMetrics creates new A/B test metrics
func NewABTestMetrics(namespace string) *ABTestMetrics {
	return &ABTestMetrics{
		variantSelections: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: "ml_abtest",
				Name:      "variant_selections_total",
				Help:      "Total number of times each variant was selected",
			},
			[]string{"experiment", "variant"},
		),
		variantLatency: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Subsystem: "ml_abtest",
				Name:      "variant_latency_seconds",
				Help:      "Latency of predictions by variant",
				Buckets:   []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5},
			},
			[]string{"experiment", "variant"},
		),
		variantErrors: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: "ml_abtest",
				Name:      "variant_errors_total",
				Help:      "Total number of errors by variant",
			},
			[]string{"experiment", "variant", "error_type"},
		),
		shadowComparisons: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: "ml_abtest",
				Name:      "shadow_comparisons_total",
				Help:      "Total number of shadow mode comparisons",
			},
			[]string{"experiment", "result"},
		),
	}
}

// Register registers metrics with Prometheus
func (m *ABTestMetrics) Register(reg prometheus.Registerer) error {
	collectors := []prometheus.Collector{
		m.variantSelections,
		m.variantLatency,
		m.variantErrors,
		m.shadowComparisons,
	}

	for _, collector := range collectors {
		if err := reg.Register(collector); err != nil {
			return err
		}
	}
	return nil
}

// NewABTestOperator creates a new A/B test operator
func NewABTestOperator(config ABTestConfig, registry *ModelRegistry, metrics *ABTestMetrics) (*ABTestOperator, error) {
	if len(config.Variants) == 0 {
		return nil, fmt.Errorf("at least one variant is required")
	}

	// Validate weights sum to 100
	totalWeight := 0.0
	for _, variant := range config.Variants {
		if variant.Enabled {
			totalWeight += variant.Weight
		}
	}
	if totalWeight < 99.9 || totalWeight > 100.1 { // Allow small floating point errors
		return nil, fmt.Errorf("variant weights must sum to 100, got %.2f", totalWeight)
	}

	// Load models for each variant
	variants := make(map[string]*variantInfo)
	for _, variantConfig := range config.Variants {
		if !variantConfig.Enabled {
			continue
		}

		var model Model
		var err error
		if variantConfig.ModelVersion != "" {
			model, err = registry.Get(variantConfig.ModelName, variantConfig.ModelVersion)
		} else {
			model, err = registry.GetLatest(variantConfig.ModelName)
		}
		if err != nil {
			return nil, fmt.Errorf("failed to load model for variant %s: %w", variantConfig.Name, err)
		}

		variants[variantConfig.Name] = &variantInfo{
			config: variantConfig,
			model:  model,
		}
	}

	if len(variants) == 0 {
		return nil, fmt.Errorf("no enabled variants found")
	}

	// Set default variant if not specified
	if config.DefaultVariant == "" {
		// Use the first enabled variant as default
		for name := range variants {
			config.DefaultVariant = name
			break
		}
	}

	return &ABTestOperator{
		config:   config,
		registry: registry,
		variants: variants,
		metrics:  metrics,
	}, nil
}

// Process processes an event using A/B testing
func (op *ABTestOperator) Process(ctx *stream.ProcessingContext, event *stream.Event) ([]*stream.Event, error) {
	op.mu.RLock()
	defer op.mu.RUnlock()

	// Select variant based on strategy
	selectedVariant := op.selectVariant(event)
	op.metrics.variantSelections.WithLabelValues(op.config.Name, selectedVariant).Inc()

	variantInfo, exists := op.variants[selectedVariant]
	if !exists {
		// Fallback to default variant
		selectedVariant = op.config.DefaultVariant
		variantInfo = op.variants[selectedVariant]
	}

	// Extract features
	features := extractFeatures(event)

	// Run primary prediction
	resp, err := variantInfo.model.Predict(ctx.Ctx, features)
	if err != nil {
		op.metrics.variantErrors.WithLabelValues(op.config.Name, selectedVariant, "prediction").Inc()
		return nil, fmt.Errorf("prediction failed for variant %s: %w", selectedVariant, err)
	}

	op.metrics.variantLatency.WithLabelValues(op.config.Name, selectedVariant).Observe(resp.Latency.Seconds())

	// Shadow mode: run all variants and compare
	if op.config.EnableShadowMode {
		op.runShadowMode(ctx.Ctx, features, selectedVariant, resp)
	}

	// Add prediction and variant info to event
	if event.Value == nil {
		event.Value = make(map[string]interface{})
	}

	valueMap, ok := event.Value.(map[string]interface{})
	if !ok {
		originalValue := event.Value
		valueMap = map[string]interface{}{
			"original": originalValue,
		}
		event.Value = valueMap
	}

	valueMap["prediction"] = resp.Prediction
	valueMap["prediction_metadata"] = resp.Metadata
	valueMap["prediction_latency_ms"] = resp.Latency.Milliseconds()
	valueMap["ab_variant"] = selectedVariant
	valueMap["ab_experiment"] = op.config.Name

	return []*stream.Event{event}, nil
}

// selectVariant selects a variant based on the configured strategy
func (op *ABTestOperator) selectVariant(event *stream.Event) string {
	switch op.config.Strategy {
	case ABTestStrategyRandom:
		return op.selectRandomVariant()

	case ABTestStrategyHash, ABTestStrategyUserID:
		// Extract split key value
		splitValue := op.extractSplitKey(event)
		return op.selectHashedVariant(splitValue)

	case ABTestStrategyPercentage:
		return op.selectPercentageVariant(event)

	default:
		return op.config.DefaultVariant
	}
}

// selectRandomVariant selects a variant randomly based on weights
func (op *ABTestOperator) selectRandomVariant() string {
	// Use event timestamp for deterministic "randomness" within a batch
	// In production, you might want to use a proper random number generator
	totalWeight := 0.0
	for _, variant := range op.variants {
		totalWeight += variant.config.Weight
	}

	// Simple weighted selection (can be improved)
	target := 0.0
	for name, variant := range op.variants {
		target += variant.config.Weight
		if target >= 50.0 { // Simplified: pick based on weight
			return name
		}
	}

	return op.config.DefaultVariant
}

// selectHashedVariant selects a variant based on hash of split key
func (op *ABTestOperator) selectHashedVariant(splitValue string) string {
	if splitValue == "" {
		return op.config.DefaultVariant
	}

	// Hash the split value
	hash := hashString(splitValue)
	percentage := float64(hash%100) + 1 // 1-100

	// Select variant based on percentage
	cumulative := 0.0
	for _, variant := range op.config.Variants {
		if !variant.Enabled {
			continue
		}
		cumulative += variant.Weight
		if percentage <= cumulative {
			return variant.Name
		}
	}

	return op.config.DefaultVariant
}

// selectPercentageVariant selects a variant based on percentage splits
func (op *ABTestOperator) selectPercentageVariant(event *stream.Event) string {
	// Similar to hashed variant but uses event key
	key := fmt.Sprintf("%v", event.Key)
	return op.selectHashedVariant(key)
}

// extractSplitKey extracts the split key value from the event
func (op *ABTestOperator) extractSplitKey(event *stream.Event) string {
	if op.config.SplitKey == "" {
		return fmt.Sprintf("%v", event.Key)
	}

	if valueMap, ok := event.Value.(map[string]interface{}); ok {
		if val, exists := valueMap[op.config.SplitKey]; exists {
			return fmt.Sprintf("%v", val)
		}
	}

	return ""
}

// runShadowMode runs all variants and compares results
func (op *ABTestOperator) runShadowMode(ctx context.Context, features map[string]interface{}, primaryVariant string, primaryResp *PredictionResponse) {
	for name, variant := range op.variants {
		if name == primaryVariant {
			continue
		}

		// Run shadow prediction
		shadowResp, err := variant.model.Predict(ctx, features)
		if err != nil {
			op.metrics.variantErrors.WithLabelValues(op.config.Name, name, "shadow").Inc()
			continue
		}

		op.metrics.variantLatency.WithLabelValues(op.config.Name, name).Observe(shadowResp.Latency.Seconds())

		// Compare predictions
		if op.config.CollectComparison {
			match := comparePredictions(primaryResp.Prediction, shadowResp.Prediction)
			if match {
				op.metrics.shadowComparisons.WithLabelValues(op.config.Name, "match").Inc()
			} else {
				op.metrics.shadowComparisons.WithLabelValues(op.config.Name, "mismatch").Inc()
			}
		}
	}
}

// UpdateVariant updates a variant configuration
func (op *ABTestOperator) UpdateVariant(variantName string, enabled bool, weight float64) error {
	op.mu.Lock()
	defer op.mu.Unlock()

	for i, variant := range op.config.Variants {
		if variant.Name == variantName {
			op.config.Variants[i].Enabled = enabled
			op.config.Variants[i].Weight = weight
			return nil
		}
	}

	return fmt.Errorf("variant %s not found", variantName)
}

// GetVariantStats returns statistics for all variants
func (op *ABTestOperator) GetVariantStats() map[string]interface{} {
	op.mu.RLock()
	defer op.mu.RUnlock()

	stats := make(map[string]interface{})
	for name, variant := range op.variants {
		stats[name] = map[string]interface{}{
			"model":   variant.config.ModelName,
			"version": variant.config.ModelVersion,
			"weight":  variant.config.Weight,
			"enabled": variant.config.Enabled,
		}
	}
	return stats
}

// Close closes the operator
func (op *ABTestOperator) Close() error {
	// Models are managed by the registry, so we don't close them here
	return nil
}

// Helper functions

func extractFeatures(event *stream.Event) map[string]interface{} {
	if valueMap, ok := event.Value.(map[string]interface{}); ok {
		return valueMap
	}
	return make(map[string]interface{})
}

func hashString(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}

func hashStringMD5(s string) uint32 {
	hash := md5.Sum([]byte(s))
	return binary.BigEndian.Uint32(hash[:4])
}

func comparePredictions(pred1, pred2 interface{}) bool {
	// Simple comparison - can be made more sophisticated
	return fmt.Sprintf("%v", pred1) == fmt.Sprintf("%v", pred2)
}

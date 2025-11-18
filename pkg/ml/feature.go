package ml

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/therealutkarshpriyadarshi/gress/pkg/state"
	"github.com/therealutkarshpriyadarshi/gress/pkg/stream"
)

// FeatureType defines the type of feature extraction
type FeatureType string

const (
	FeatureTypeNumerical    FeatureType = "numerical"
	FeatureTypeCategorical  FeatureType = "categorical"
	FeatureTypeText         FeatureType = "text"
	FeatureTypeTemporal     FeatureType = "temporal"
	FeatureTypeAggregation  FeatureType = "aggregation"
	FeatureTypeOneHot       FeatureType = "onehot"
	FeatureTypeNormalize    FeatureType = "normalize"
	FeatureTypeStandardize  FeatureType = "standardize"
	FeatureTypeLog          FeatureType = "log"
	FeatureTypeBucket       FeatureType = "bucket"
)

// FeatureSpec defines a feature extraction specification
type FeatureSpec struct {
	Name       string                 `yaml:"name" json:"name"`
	Type       FeatureType            `yaml:"type" json:"type"`
	Source     string                 `yaml:"source" json:"source"` // Source field name
	Output     string                 `yaml:"output" json:"output"` // Output field name
	Transform  string                 `yaml:"transform" json:"transform,omitempty"`
	Parameters map[string]interface{} `yaml:"parameters" json:"parameters,omitempty"`
}

// FeatureExtractionConfig configures the feature extraction operator
type FeatureExtractionConfig struct {
	Features     []FeatureSpec `yaml:"features" json:"features"`
	DropOriginal bool          `yaml:"drop_original" json:"drop_original"`
	StateBackend string        `yaml:"state_backend" json:"state_backend"` // For stateful features
}

// FeatureExtractionOperator extracts and transforms features from events
type FeatureExtractionOperator struct {
	config       FeatureExtractionConfig
	stateBackend state.StateBackend
	extractors   []FeatureExtractor
}

// FeatureExtractor is an interface for feature extraction
type FeatureExtractor interface {
	Extract(event *stream.Event, stateBackend state.StateBackend) (string, interface{}, error)
	Spec() FeatureSpec
}

// NewFeatureExtractionOperator creates a new feature extraction operator
func NewFeatureExtractionOperator(config FeatureExtractionConfig, stateBackend state.StateBackend) (*FeatureExtractionOperator, error) {
	extractors := make([]FeatureExtractor, 0, len(config.Features))

	for _, spec := range config.Features {
		extractor, err := createFeatureExtractor(spec)
		if err != nil {
			return nil, fmt.Errorf("failed to create extractor for %s: %w", spec.Name, err)
		}
		extractors = append(extractors, extractor)
	}

	return &FeatureExtractionOperator{
		config:       config,
		stateBackend: stateBackend,
		extractors:   extractors,
	}, nil
}

// Process processes an event and extracts features
func (op *FeatureExtractionOperator) Process(ctx *stream.ProcessingContext, event *stream.Event) ([]*stream.Event, error) {
	// Ensure event value is a map
	var valueMap map[string]interface{}
	if event.Value == nil {
		valueMap = make(map[string]interface{})
		event.Value = valueMap
	} else if vm, ok := event.Value.(map[string]interface{}); ok {
		valueMap = vm
	} else {
		// Wrap non-map values
		originalValue := event.Value
		valueMap = map[string]interface{}{
			"original": originalValue,
		}
		event.Value = valueMap
	}

	// Extract features
	features := make(map[string]interface{})
	for _, extractor := range op.extractors {
		outputKey, value, err := extractor.Extract(event, op.stateBackend)
		if err != nil {
			return nil, fmt.Errorf("feature extraction failed for %s: %w", extractor.Spec().Name, err)
		}
		features[outputKey] = value
	}

	// Add features to event
	if op.config.DropOriginal {
		event.Value = features
	} else {
		// Merge features into existing value
		for k, v := range features {
			valueMap[k] = v
		}
	}

	return []*stream.Event{event}, nil
}

// Close closes the operator
func (op *FeatureExtractionOperator) Close() error {
	return nil
}

// createFeatureExtractor creates a feature extractor based on the spec
func createFeatureExtractor(spec FeatureSpec) (FeatureExtractor, error) {
	switch spec.Type {
	case FeatureTypeNumerical:
		return &NumericalExtractor{spec: spec}, nil
	case FeatureTypeCategorical:
		return &CategoricalExtractor{spec: spec}, nil
	case FeatureTypeOneHot:
		return &OneHotExtractor{spec: spec}, nil
	case FeatureTypeNormalize:
		return &NormalizeExtractor{spec: spec}, nil
	case FeatureTypeStandardize:
		return &StandardizeExtractor{spec: spec}, nil
	case FeatureTypeLog:
		return &LogExtractor{spec: spec}, nil
	case FeatureTypeBucket:
		return &BucketExtractor{spec: spec}, nil
	case FeatureTypeTemporal:
		return &TemporalExtractor{spec: spec}, nil
	case FeatureTypeAggregation:
		return &AggregationExtractor{spec: spec}, nil
	case FeatureTypeText:
		return &TextExtractor{spec: spec}, nil
	default:
		return nil, fmt.Errorf("unsupported feature type: %s", spec.Type)
	}
}

// NumericalExtractor extracts numerical features
type NumericalExtractor struct {
	spec FeatureSpec
}

func (e *NumericalExtractor) Extract(event *stream.Event, stateBackend state.StateBackend) (string, interface{}, error) {
	value := getFieldValue(event.Value, e.spec.Source)
	if value == nil {
		return e.spec.Output, 0.0, nil
	}

	numValue, err := toFloat64(value)
	if err != nil {
		return "", nil, fmt.Errorf("failed to convert to float64: %w", err)
	}

	return e.spec.Output, numValue, nil
}

func (e *NumericalExtractor) Spec() FeatureSpec {
	return e.spec
}

// CategoricalExtractor extracts categorical features as integers
type CategoricalExtractor struct {
	spec FeatureSpec
}

func (e *CategoricalExtractor) Extract(event *stream.Event, stateBackend state.StateBackend) (string, interface{}, error) {
	value := getFieldValue(event.Value, e.spec.Source)
	if value == nil {
		return e.spec.Output, "", nil
	}

	strValue := fmt.Sprintf("%v", value)
	return e.spec.Output, strValue, nil
}

func (e *CategoricalExtractor) Spec() FeatureSpec {
	return e.spec
}

// OneHotExtractor performs one-hot encoding
type OneHotExtractor struct {
	spec FeatureSpec
}

func (e *OneHotExtractor) Extract(event *stream.Event, stateBackend state.StateBackend) (string, interface{}, error) {
	value := getFieldValue(event.Value, e.spec.Source)
	if value == nil {
		return e.spec.Output, make(map[string]float64), nil
	}

	categories, ok := e.spec.Parameters["categories"].([]interface{})
	if !ok {
		return "", nil, fmt.Errorf("categories parameter required for one-hot encoding")
	}

	strValue := fmt.Sprintf("%v", value)
	oneHot := make(map[string]float64)
	for _, cat := range categories {
		catStr := fmt.Sprintf("%v", cat)
		if catStr == strValue {
			oneHot[catStr] = 1.0
		} else {
			oneHot[catStr] = 0.0
		}
	}

	return e.spec.Output, oneHot, nil
}

func (e *OneHotExtractor) Spec() FeatureSpec {
	return e.spec
}

// NormalizeExtractor normalizes values to [0, 1]
type NormalizeExtractor struct {
	spec FeatureSpec
}

func (e *NormalizeExtractor) Extract(event *stream.Event, stateBackend state.StateBackend) (string, interface{}, error) {
	value := getFieldValue(event.Value, e.spec.Source)
	if value == nil {
		return e.spec.Output, 0.0, nil
	}

	numValue, err := toFloat64(value)
	if err != nil {
		return "", nil, err
	}

	min, _ := toFloat64(e.spec.Parameters["min"])
	max, _ := toFloat64(e.spec.Parameters["max"])

	if max == min {
		return e.spec.Output, 0.0, nil
	}

	normalized := (numValue - min) / (max - min)
	return e.spec.Output, normalized, nil
}

func (e *NormalizeExtractor) Spec() FeatureSpec {
	return e.spec
}

// StandardizeExtractor standardizes values (z-score normalization)
type StandardizeExtractor struct {
	spec FeatureSpec
}

func (e *StandardizeExtractor) Extract(event *stream.Event, stateBackend state.StateBackend) (string, interface{}, error) {
	value := getFieldValue(event.Value, e.spec.Source)
	if value == nil {
		return e.spec.Output, 0.0, nil
	}

	numValue, err := toFloat64(value)
	if err != nil {
		return "", nil, err
	}

	mean, _ := toFloat64(e.spec.Parameters["mean"])
	stddev, _ := toFloat64(e.spec.Parameters["stddev"])

	if stddev == 0 {
		return e.spec.Output, 0.0, nil
	}

	standardized := (numValue - mean) / stddev
	return e.spec.Output, standardized, nil
}

func (e *StandardizeExtractor) Spec() FeatureSpec {
	return e.spec
}

// LogExtractor applies logarithm transformation
type LogExtractor struct {
	spec FeatureSpec
}

func (e *LogExtractor) Extract(event *stream.Event, stateBackend state.StateBackend) (string, interface{}, error) {
	value := getFieldValue(event.Value, e.spec.Source)
	if value == nil {
		return e.spec.Output, 0.0, nil
	}

	numValue, err := toFloat64(value)
	if err != nil {
		return "", nil, err
	}

	if numValue <= 0 {
		return e.spec.Output, 0.0, nil
	}

	logValue := math.Log(numValue)
	return e.spec.Output, logValue, nil
}

func (e *LogExtractor) Spec() FeatureSpec {
	return e.spec
}

// BucketExtractor buckets values into discrete ranges
type BucketExtractor struct {
	spec FeatureSpec
}

func (e *BucketExtractor) Extract(event *stream.Event, stateBackend state.StateBackend) (string, interface{}, error) {
	value := getFieldValue(event.Value, e.spec.Source)
	if value == nil {
		return e.spec.Output, 0, nil
	}

	numValue, err := toFloat64(value)
	if err != nil {
		return "", nil, err
	}

	boundariesInterface, ok := e.spec.Parameters["boundaries"].([]interface{})
	if !ok {
		return "", nil, fmt.Errorf("boundaries parameter required for bucketing")
	}

	boundaries := make([]float64, len(boundariesInterface))
	for i, b := range boundariesInterface {
		boundaries[i], _ = toFloat64(b)
	}

	sort.Float64s(boundaries)

	bucket := 0
	for i, boundary := range boundaries {
		if numValue > boundary {
			bucket = i + 1
		} else {
			break
		}
	}

	return e.spec.Output, bucket, nil
}

func (e *BucketExtractor) Spec() FeatureSpec {
	return e.spec
}

// TemporalExtractor extracts time-based features
type TemporalExtractor struct {
	spec FeatureSpec
}

func (e *TemporalExtractor) Extract(event *stream.Event, stateBackend state.StateBackend) (string, interface{}, error) {
	t := event.EventTime

	features := make(map[string]interface{})
	features["hour"] = t.Hour()
	features["day_of_week"] = int(t.Weekday())
	features["day_of_month"] = t.Day()
	features["month"] = int(t.Month())
	features["year"] = t.Year()
	features["is_weekend"] = t.Weekday() == time.Saturday || t.Weekday() == time.Sunday
	features["unix_timestamp"] = t.Unix()

	return e.spec.Output, features, nil
}

func (e *TemporalExtractor) Spec() FeatureSpec {
	return e.spec
}

// AggregationExtractor computes aggregations over a window (requires state)
type AggregationExtractor struct {
	spec FeatureSpec
}

func (e *AggregationExtractor) Extract(event *stream.Event, stateBackend state.StateBackend) (string, interface{}, error) {
	if stateBackend == nil {
		return "", nil, fmt.Errorf("state backend required for aggregation features")
	}

	// This is a simplified implementation
	// In production, you'd want to use the windowing operators
	key := fmt.Sprintf("%v", event.Key)
	stateKey := fmt.Sprintf("agg:%s:%s", e.spec.Name, key)

	// Get current value
	value := getFieldValue(event.Value, e.spec.Source)
	numValue, err := toFloat64(value)
	if err != nil {
		return "", nil, err
	}

	// Compute aggregation based on type
	aggType := e.spec.Parameters["type"]
	switch aggType {
	case "sum", "count", "avg", "min", "max":
		// Store and retrieve from state (simplified)
		return e.spec.Output, numValue, nil
	default:
		return "", nil, fmt.Errorf("unsupported aggregation type: %v", aggType)
	}
}

func (e *AggregationExtractor) Spec() FeatureSpec {
	return e.spec
}

// TextExtractor extracts features from text
type TextExtractor struct {
	spec FeatureSpec
}

func (e *TextExtractor) Extract(event *stream.Event, stateBackend state.StateBackend) (string, interface{}, error) {
	value := getFieldValue(event.Value, e.spec.Source)
	if value == nil {
		return e.spec.Output, make(map[string]interface{}), nil
	}

	text := fmt.Sprintf("%v", value)

	features := make(map[string]interface{})
	features["length"] = len(text)
	features["word_count"] = len(strings.Fields(text))
	features["contains_number"] = strings.ContainsAny(text, "0123456789")
	features["lowercase"] = strings.ToLower(text)
	features["uppercase"] = strings.ToUpper(text)

	return e.spec.Output, features, nil
}

func (e *TextExtractor) Spec() FeatureSpec {
	return e.spec
}

// Helper functions

func getFieldValue(value interface{}, fieldPath string) interface{} {
	if fieldPath == "" {
		return value
	}

	// Support nested field access with dot notation
	parts := strings.Split(fieldPath, ".")
	current := value

	for _, part := range parts {
		if m, ok := current.(map[string]interface{}); ok {
			current = m[part]
		} else {
			return nil
		}
	}

	return current
}

func toFloat64(value interface{}) (float64, error) {
	switch v := value.(type) {
	case float64:
		return v, nil
	case float32:
		return float64(v), nil
	case int:
		return float64(v), nil
	case int32:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case uint:
		return float64(v), nil
	case uint32:
		return float64(v), nil
	case uint64:
		return float64(v), nil
	case string:
		// Try to parse string as float
		var f float64
		_, err := fmt.Sscanf(v, "%f", &f)
		return f, err
	default:
		return 0, fmt.Errorf("cannot convert %T to float64", value)
	}
}

package ml

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/therealutkarshpriyadarshi/gress/pkg/stream"
)

// InferenceConfig configures the ML inference operator
type InferenceConfig struct {
	ModelName      string        `yaml:"model_name" json:"model_name"`
	ModelVersion   string        `yaml:"model_version" json:"model_version"`
	BatchSize      int           `yaml:"batch_size" json:"batch_size"`
	BatchTimeout   time.Duration `yaml:"batch_timeout" json:"batch_timeout"`
	MaxConcurrency int           `yaml:"max_concurrency" json:"max_concurrency"`
	FeatureFields  []string      `yaml:"feature_fields" json:"feature_fields"`
	OutputField    string        `yaml:"output_field" json:"output_field"`
}

// InferenceOperator performs ML inference on streaming events
type InferenceOperator struct {
	config   InferenceConfig
	model    Model
	registry *ModelRegistry
	metrics  *InferenceMetrics

	// Batching
	batch      []*batchItem
	batchMu    sync.Mutex
	batchTimer *time.Timer

	// Concurrency control
	sem chan struct{}

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

type batchItem struct {
	event    *stream.Event
	features map[string]interface{}
	result   chan *PredictionResponse
	err      chan error
}

// InferenceMetrics tracks ML inference metrics
type InferenceMetrics struct {
	inferenceLatency   *prometheus.HistogramVec
	predictions        *prometheus.CounterVec
	batchSize          *prometheus.HistogramVec
	errors             *prometheus.CounterVec
	queueDepth         prometheus.Gauge
	activePredictions  prometheus.Gauge
}

// NewInferenceMetrics creates new inference metrics
func NewInferenceMetrics(namespace string) *InferenceMetrics {
	return &InferenceMetrics{
		inferenceLatency: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Subsystem: "ml",
				Name:      "inference_latency_seconds",
				Help:      "ML inference latency in seconds",
				Buckets:   []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5},
			},
			[]string{"model", "version"},
		),
		predictions: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: "ml",
				Name:      "predictions_total",
				Help:      "Total number of ML predictions",
			},
			[]string{"model", "version", "status"},
		),
		batchSize: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Subsystem: "ml",
				Name:      "batch_size",
				Help:      "ML inference batch size",
				Buckets:   []float64{1, 5, 10, 25, 50, 100, 250, 500, 1000},
			},
			[]string{"model", "version"},
		),
		errors: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: "ml",
				Name:      "errors_total",
				Help:      "Total number of ML inference errors",
			},
			[]string{"model", "version", "error_type"},
		),
		queueDepth: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: "ml",
				Name:      "queue_depth",
				Help:      "Current ML inference queue depth",
			},
		),
		activePredictions: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: "ml",
				Name:      "active_predictions",
				Help:      "Number of active ML predictions in flight",
			},
		),
	}
}

// Register registers metrics with Prometheus
func (m *InferenceMetrics) Register(reg prometheus.Registerer) error {
	collectors := []prometheus.Collector{
		m.inferenceLatency,
		m.predictions,
		m.batchSize,
		m.errors,
		m.queueDepth,
		m.activePredictions,
	}

	for _, collector := range collectors {
		if err := reg.Register(collector); err != nil {
			return err
		}
	}
	return nil
}

// NewInferenceOperator creates a new ML inference operator
func NewInferenceOperator(config InferenceConfig, registry *ModelRegistry, metrics *InferenceMetrics) (*InferenceOperator, error) {
	// Get model from registry
	var model Model
	var err error
	if config.ModelVersion != "" {
		model, err = registry.Get(config.ModelName, config.ModelVersion)
	} else {
		model, err = registry.GetLatest(config.ModelName)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get model: %w", err)
	}

	// Set defaults
	if config.BatchSize <= 0 {
		config.BatchSize = 1
	}
	if config.BatchTimeout <= 0 {
		config.BatchTimeout = 10 * time.Millisecond
	}
	if config.MaxConcurrency <= 0 {
		config.MaxConcurrency = 10
	}
	if config.OutputField == "" {
		config.OutputField = "prediction"
	}

	ctx, cancel := context.WithCancel(context.Background())

	op := &InferenceOperator{
		config:   config,
		model:    model,
		registry: registry,
		metrics:  metrics,
		batch:    make([]*batchItem, 0, config.BatchSize),
		sem:      make(chan struct{}, config.MaxConcurrency),
		ctx:      ctx,
		cancel:   cancel,
	}

	// Start batch processor
	op.wg.Add(1)
	go op.batchProcessor()

	return op, nil
}

// Process processes a single event
func (op *InferenceOperator) Process(ctx *stream.ProcessingContext, event *stream.Event) ([]*stream.Event, error) {
	// Extract features from event
	features := op.extractFeatures(event)

	// Create batch item
	item := &batchItem{
		event:    event,
		features: features,
		result:   make(chan *PredictionResponse, 1),
		err:      make(chan error, 1),
	}

	// Add to batch
	op.batchMu.Lock()
	op.batch = append(op.batch, item)
	currentBatchSize := len(op.batch)
	op.metrics.queueDepth.Set(float64(currentBatchSize))

	// Check if we should flush the batch
	shouldFlush := currentBatchSize >= op.config.BatchSize
	if shouldFlush {
		// Reset timer if it exists
		if op.batchTimer != nil {
			op.batchTimer.Stop()
			op.batchTimer = nil
		}
		// Trigger immediate flush
		op.flushBatchLocked()
	} else if op.batchTimer == nil {
		// Start timer for batch timeout
		op.batchTimer = time.AfterFunc(op.config.BatchTimeout, func() {
			op.batchMu.Lock()
			defer op.batchMu.Unlock()
			op.flushBatchLocked()
		})
	}
	op.batchMu.Unlock()

	// Wait for result
	select {
	case resp := <-item.result:
		// Add prediction to event
		if event.Value == nil {
			event.Value = make(map[string]interface{})
		}

		valueMap, ok := event.Value.(map[string]interface{})
		if !ok {
			// If value is not a map, wrap it
			originalValue := event.Value
			valueMap = map[string]interface{}{
				"original": originalValue,
			}
			event.Value = valueMap
		}

		valueMap[op.config.OutputField] = resp.Prediction
		valueMap["prediction_metadata"] = resp.Metadata
		valueMap["prediction_latency_ms"] = resp.Latency.Milliseconds()

		return []*stream.Event{event}, nil

	case err := <-item.err:
		return nil, fmt.Errorf("inference failed: %w", err)

	case <-ctx.Ctx.Done():
		return nil, ctx.Ctx.Err()
	}
}

// extractFeatures extracts features from an event
func (op *InferenceOperator) extractFeatures(event *stream.Event) map[string]interface{} {
	features := make(map[string]interface{})

	// If feature fields are specified, extract only those
	if len(op.config.FeatureFields) > 0 {
		if valueMap, ok := event.Value.(map[string]interface{}); ok {
			for _, field := range op.config.FeatureFields {
				if val, exists := valueMap[field]; exists {
					features[field] = val
				}
			}
		}
	} else {
		// Otherwise, use the entire event value as features
		if valueMap, ok := event.Value.(map[string]interface{}); ok {
			features = valueMap
		}
	}

	return features
}

// flushBatchLocked flushes the current batch (must be called with lock held)
func (op *InferenceOperator) flushBatchLocked() {
	if len(op.batch) == 0 {
		return
	}

	// Copy batch and reset
	batchToProcess := op.batch
	op.batch = make([]*batchItem, 0, op.config.BatchSize)
	op.batchTimer = nil
	op.metrics.queueDepth.Set(0)

	// Process batch asynchronously
	op.wg.Add(1)
	go op.processBatch(batchToProcess)
}

// processBatch processes a batch of items
func (op *InferenceOperator) processBatch(batch []*batchItem) {
	defer op.wg.Done()

	// Acquire semaphore
	op.sem <- struct{}{}
	defer func() { <-op.sem }()

	op.metrics.activePredictions.Inc()
	defer op.metrics.activePredictions.Dec()

	batchSize := len(batch)
	metadata := op.model.GetMetadata()

	// Record batch size
	op.metrics.batchSize.WithLabelValues(metadata.Name, metadata.Version).Observe(float64(batchSize))

	// Prepare batch requests
	requests := make([]*PredictionRequest, batchSize)
	for i, item := range batch {
		requests[i] = &PredictionRequest{
			Features:  item.features,
			Timestamp: item.event.EventTime,
		}
	}

	// Perform batch inference
	start := time.Now()
	responses, err := op.model.PredictBatch(op.ctx, requests)
	latency := time.Since(start)

	// Record metrics
	op.metrics.inferenceLatency.WithLabelValues(metadata.Name, metadata.Version).Observe(latency.Seconds())

	if err != nil {
		// Batch failed - send error to all items
		op.metrics.errors.WithLabelValues(metadata.Name, metadata.Version, "batch_inference").Add(float64(batchSize))
		op.metrics.predictions.WithLabelValues(metadata.Name, metadata.Version, "error").Add(float64(batchSize))

		for _, item := range batch {
			item.err <- err
		}
		return
	}

	// Send results to individual items
	op.metrics.predictions.WithLabelValues(metadata.Name, metadata.Version, "success").Add(float64(batchSize))
	for i, item := range batch {
		if i < len(responses) {
			item.result <- responses[i]
		} else {
			item.err <- fmt.Errorf("no response for batch item %d", i)
		}
	}
}

// batchProcessor processes batches on a timer
func (op *InferenceOperator) batchProcessor() {
	defer op.wg.Done()

	ticker := time.NewTicker(op.config.BatchTimeout)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			op.batchMu.Lock()
			op.flushBatchLocked()
			op.batchMu.Unlock()

		case <-op.ctx.Done():
			// Flush remaining batch
			op.batchMu.Lock()
			op.flushBatchLocked()
			op.batchMu.Unlock()
			return
		}
	}
}

// Close closes the operator
func (op *InferenceOperator) Close() error {
	op.cancel()

	// Flush remaining batch
	op.batchMu.Lock()
	op.flushBatchLocked()
	op.batchMu.Unlock()

	// Wait for all batch processing to complete
	op.wg.Wait()

	return nil
}

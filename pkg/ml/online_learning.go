package ml

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/therealutkarshpriyadarshi/gress/pkg/stream"
)

// OnlineLearningConfig configures online learning
type OnlineLearningConfig struct {
	ModelName         string        `yaml:"model_name" json:"model_name"`
	ModelVersion      string        `yaml:"model_version" json:"model_version"`
	UpdateBatchSize   int           `yaml:"update_batch_size" json:"update_batch_size"`
	UpdateInterval    time.Duration `yaml:"update_interval" json:"update_interval"`
	FeatureFields     []string      `yaml:"feature_fields" json:"feature_fields"`
	TargetField       string        `yaml:"target_field" json:"target_field"`
	CheckpointDir     string        `yaml:"checkpoint_dir" json:"checkpoint_dir"`
	CheckpointInterval time.Duration `yaml:"checkpoint_interval" json:"checkpoint_interval"`
	LearningRate      float64       `yaml:"learning_rate" json:"learning_rate"`
	EnableValidation  bool          `yaml:"enable_validation" json:"enable_validation"`
	ValidationSplit   float64       `yaml:"validation_split" json:"validation_split"`
}

// OnlineLearningOperator performs online learning on streaming data
type OnlineLearningOperator struct {
	config   OnlineLearningConfig
	model    OnlineModel
	registry *ModelRegistry
	metrics  *OnlineLearningMetrics

	// Batching for updates
	updateBatch   []TrainingSample
	updateBatchMu sync.Mutex

	// Checkpointing
	lastCheckpoint time.Time
	checkpointMu   sync.Mutex

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// OnlineLearningMetrics tracks online learning metrics
type OnlineLearningMetrics struct {
	updatesProcessed   *prometheus.CounterVec
	updateLatency      *prometheus.HistogramVec
	updateBatchSize    *prometheus.HistogramVec
	checkpointDuration *prometheus.HistogramVec
	modelScore         *prometheus.GaugeVec
	trainingErrors     *prometheus.CounterVec
}

// NewOnlineLearningMetrics creates new online learning metrics
func NewOnlineLearningMetrics(namespace string) *OnlineLearningMetrics {
	return &OnlineLearningMetrics{
		updatesProcessed: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: "ml_online",
				Name:      "updates_processed_total",
				Help:      "Total number of online learning updates processed",
			},
			[]string{"model", "version"},
		),
		updateLatency: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Subsystem: "ml_online",
				Name:      "update_latency_seconds",
				Help:      "Online learning update latency in seconds",
				Buckets:   []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5},
			},
			[]string{"model", "version"},
		),
		updateBatchSize: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Subsystem: "ml_online",
				Name:      "update_batch_size",
				Help:      "Online learning update batch size",
				Buckets:   []float64{1, 5, 10, 25, 50, 100, 250, 500, 1000},
			},
			[]string{"model", "version"},
		),
		checkpointDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Subsystem: "ml_online",
				Name:      "checkpoint_duration_seconds",
				Help:      "Model checkpoint duration in seconds",
				Buckets:   []float64{.1, .5, 1, 2.5, 5, 10, 30, 60},
			},
			[]string{"model", "version"},
		),
		modelScore: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: "ml_online",
				Name:      "model_score",
				Help:      "Current model validation score",
			},
			[]string{"model", "version", "metric"},
		),
		trainingErrors: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: "ml_online",
				Name:      "training_errors_total",
				Help:      "Total number of online learning training errors",
			},
			[]string{"model", "version", "error_type"},
		),
	}
}

// Register registers metrics with Prometheus
func (m *OnlineLearningMetrics) Register(reg prometheus.Registerer) error {
	collectors := []prometheus.Collector{
		m.updatesProcessed,
		m.updateLatency,
		m.updateBatchSize,
		m.checkpointDuration,
		m.modelScore,
		m.trainingErrors,
	}

	for _, collector := range collectors {
		if err := reg.Register(collector); err != nil {
			return err
		}
	}
	return nil
}

// NewOnlineLearningOperator creates a new online learning operator
func NewOnlineLearningOperator(config OnlineLearningConfig, registry *ModelRegistry, metrics *OnlineLearningMetrics) (*OnlineLearningOperator, error) {
	// Get model from registry
	var baseModel Model
	var err error
	if config.ModelVersion != "" {
		baseModel, err = registry.Get(config.ModelName, config.ModelVersion)
	} else {
		baseModel, err = registry.GetLatest(config.ModelName)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get model: %w", err)
	}

	// Check if model supports online learning
	onlineModel, ok := baseModel.(OnlineModel)
	if !ok {
		return nil, fmt.Errorf("model %s does not support online learning", config.ModelName)
	}

	// Set defaults
	if config.UpdateBatchSize <= 0 {
		config.UpdateBatchSize = 32
	}
	if config.UpdateInterval <= 0 {
		config.UpdateInterval = 5 * time.Second
	}
	if config.CheckpointInterval <= 0 {
		config.CheckpointInterval = 5 * time.Minute
	}
	if config.TargetField == "" {
		config.TargetField = "label"
	}
	if config.ValidationSplit <= 0 {
		config.ValidationSplit = 0.2
	}

	ctx, cancel := context.WithCancel(context.Background())

	op := &OnlineLearningOperator{
		config:         config,
		model:          onlineModel,
		registry:       registry,
		metrics:        metrics,
		updateBatch:    make([]TrainingSample, 0, config.UpdateBatchSize),
		lastCheckpoint: time.Now(),
		ctx:            ctx,
		cancel:         cancel,
	}

	// Start background workers
	op.wg.Add(2)
	go op.updateWorker()
	go op.checkpointWorker()

	return op, nil
}

// Process processes an event and updates the model
func (op *OnlineLearningOperator) Process(ctx *stream.ProcessingContext, event *stream.Event) ([]*stream.Event, error) {
	// Extract features and target
	features := op.extractFeatures(event)
	target := op.extractTarget(event)

	if target == nil {
		// No target available, skip training
		return []*stream.Event{event}, nil
	}

	// Create training sample
	sample := TrainingSample{
		Features:  features,
		Target:    target,
		Timestamp: event.EventTime,
		Weight:    1.0,
	}

	// Add to batch
	op.updateBatchMu.Lock()
	op.updateBatch = append(op.updateBatch, sample)
	batchSize := len(op.updateBatch)

	// Check if we should trigger an update
	if batchSize >= op.config.UpdateBatchSize {
		// Trigger immediate update
		batchToUpdate := op.updateBatch
		op.updateBatch = make([]TrainingSample, 0, op.config.UpdateBatchSize)
		op.updateBatchMu.Unlock()

		// Perform update
		if err := op.performUpdate(batchToUpdate); err != nil {
			return nil, fmt.Errorf("online learning update failed: %w", err)
		}
	} else {
		op.updateBatchMu.Unlock()
	}

	return []*stream.Event{event}, nil
}

// extractFeatures extracts features from an event
func (op *OnlineLearningOperator) extractFeatures(event *stream.Event) map[string]interface{} {
	features := make(map[string]interface{})

	if len(op.config.FeatureFields) > 0 {
		if valueMap, ok := event.Value.(map[string]interface{}); ok {
			for _, field := range op.config.FeatureFields {
				if val, exists := valueMap[field]; exists {
					features[field] = val
				}
			}
		}
	} else {
		if valueMap, ok := event.Value.(map[string]interface{}); ok {
			features = valueMap
		}
	}

	return features
}

// extractTarget extracts the target variable from an event
func (op *OnlineLearningOperator) extractTarget(event *stream.Event) interface{} {
	if valueMap, ok := event.Value.(map[string]interface{}); ok {
		if target, exists := valueMap[op.config.TargetField]; exists {
			return target
		}
	}
	return nil
}

// performUpdate performs a model update with a batch of samples
func (op *OnlineLearningOperator) performUpdate(batch []TrainingSample) error {
	if len(batch) == 0 {
		return nil
	}

	metadata := op.model.GetMetadata()
	start := time.Now()

	// Split into training and validation if enabled
	var trainSamples, valSamples []TrainingSample
	if op.config.EnableValidation {
		splitIdx := int(float64(len(batch)) * (1.0 - op.config.ValidationSplit))
		trainSamples = batch[:splitIdx]
		valSamples = batch[splitIdx:]
	} else {
		trainSamples = batch
	}

	// Perform partial fit
	err := op.model.PartialFitBatch(op.ctx, trainSamples)
	if err != nil {
		op.metrics.trainingErrors.WithLabelValues(metadata.Name, metadata.Version, "partial_fit").Inc()
		return fmt.Errorf("partial fit failed: %w", err)
	}

	latency := time.Since(start)

	// Record metrics
	op.metrics.updatesProcessed.WithLabelValues(metadata.Name, metadata.Version).Add(float64(len(trainSamples)))
	op.metrics.updateLatency.WithLabelValues(metadata.Name, metadata.Version).Observe(latency.Seconds())
	op.metrics.updateBatchSize.WithLabelValues(metadata.Name, metadata.Version).Observe(float64(len(trainSamples)))

	// Validate if enabled
	if op.config.EnableValidation && len(valSamples) > 0 {
		if err := op.validate(valSamples, metadata); err != nil {
			op.metrics.trainingErrors.WithLabelValues(metadata.Name, metadata.Version, "validation").Inc()
		}
	}

	return nil
}

// validate validates the model on validation samples
func (op *OnlineLearningOperator) validate(samples []TrainingSample, metadata ModelMetadata) error {
	correct := 0
	total := len(samples)

	for _, sample := range samples {
		resp, err := op.model.Predict(op.ctx, sample.Features)
		if err != nil {
			continue
		}

		// Simple accuracy for classification (can be extended for regression)
		if fmt.Sprintf("%v", resp.Prediction) == fmt.Sprintf("%v", sample.Target) {
			correct++
		}
	}

	accuracy := float64(correct) / float64(total)
	op.metrics.modelScore.WithLabelValues(metadata.Name, metadata.Version, "accuracy").Set(accuracy)

	return nil
}

// updateWorker periodically flushes the update batch
func (op *OnlineLearningOperator) updateWorker() {
	defer op.wg.Done()

	ticker := time.NewTicker(op.config.UpdateInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			op.updateBatchMu.Lock()
			if len(op.updateBatch) > 0 {
				batchToUpdate := op.updateBatch
				op.updateBatch = make([]TrainingSample, 0, op.config.UpdateBatchSize)
				op.updateBatchMu.Unlock()

				if err := op.performUpdate(batchToUpdate); err != nil {
					// Log error but continue
					fmt.Printf("Online learning update failed: %v\n", err)
				}
			} else {
				op.updateBatchMu.Unlock()
			}

		case <-op.ctx.Done():
			return
		}
	}
}

// checkpointWorker periodically saves model checkpoints
func (op *OnlineLearningOperator) checkpointWorker() {
	defer op.wg.Done()

	ticker := time.NewTicker(op.config.CheckpointInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := op.saveCheckpoint(); err != nil {
				fmt.Printf("Failed to save checkpoint: %v\n", err)
			}

		case <-op.ctx.Done():
			// Save final checkpoint
			if err := op.saveCheckpoint(); err != nil {
				fmt.Printf("Failed to save final checkpoint: %v\n", err)
			}
			return
		}
	}
}

// saveCheckpoint saves a model checkpoint
func (op *OnlineLearningOperator) saveCheckpoint() error {
	op.checkpointMu.Lock()
	defer op.checkpointMu.Unlock()

	if op.config.CheckpointDir == "" {
		return nil // Checkpointing disabled
	}

	metadata := op.model.GetMetadata()
	start := time.Now()

	// Generate checkpoint path
	timestamp := time.Now().Format("20060102-150405")
	checkpointPath := fmt.Sprintf("%s/%s-%s-%s.ckpt",
		op.config.CheckpointDir,
		metadata.Name,
		metadata.Version,
		timestamp,
	)

	// Save checkpoint
	err := op.model.SaveCheckpoint(op.ctx, checkpointPath)
	if err != nil {
		return fmt.Errorf("failed to save checkpoint to %s: %w", checkpointPath, err)
	}

	duration := time.Since(start)
	op.metrics.checkpointDuration.WithLabelValues(metadata.Name, metadata.Version).Observe(duration.Seconds())
	op.lastCheckpoint = time.Now()

	return nil
}

// GetStats returns statistics about the online learning process
func (op *OnlineLearningOperator) GetStats() map[string]interface{} {
	op.updateBatchMu.Lock()
	pendingUpdates := len(op.updateBatch)
	op.updateBatchMu.Unlock()

	op.checkpointMu.Lock()
	lastCheckpoint := op.lastCheckpoint
	op.checkpointMu.Unlock()

	metadata := op.model.GetMetadata()

	return map[string]interface{}{
		"model":                  metadata.Name,
		"version":                metadata.Version,
		"pending_updates":        pendingUpdates,
		"last_checkpoint":        lastCheckpoint,
		"checkpoint_interval":    op.config.CheckpointInterval,
		"update_batch_size":      op.config.UpdateBatchSize,
		"update_interval":        op.config.UpdateInterval,
		"validation_enabled":     op.config.EnableValidation,
	}
}

// Close closes the operator
func (op *OnlineLearningOperator) Close() error {
	op.cancel()

	// Flush remaining updates
	op.updateBatchMu.Lock()
	if len(op.updateBatch) > 0 {
		batchToUpdate := op.updateBatch
		op.updateBatch = nil
		op.updateBatchMu.Unlock()

		if err := op.performUpdate(batchToUpdate); err != nil {
			fmt.Printf("Failed to flush final updates: %v\n", err)
		}
	} else {
		op.updateBatchMu.Unlock()
	}

	// Wait for workers to finish
	op.wg.Wait()

	return nil
}

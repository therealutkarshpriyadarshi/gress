package ml

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// MLMetrics aggregates all ML-related metrics
type MLMetrics struct {
	// Model loading and lifecycle
	modelsLoaded *prometheus.GaugeVec
	modelLoadTime *prometheus.HistogramVec
	modelLoadErrors *prometheus.CounterVec

	// Inference metrics
	inferenceLatency   *prometheus.HistogramVec
	predictions        *prometheus.CounterVec
	batchSize          *prometheus.HistogramVec
	inferenceErrors    *prometheus.CounterVec
	queueDepth         prometheus.Gauge
	activePredictions  prometheus.Gauge

	// Feature extraction metrics
	featureExtractionTime *prometheus.HistogramVec
	featureExtractionErrors *prometheus.CounterVec
	featuresExtracted *prometheus.CounterVec

	// Online learning metrics
	updatesProcessed   *prometheus.CounterVec
	updateLatency      *prometheus.HistogramVec
	updateBatchSize    *prometheus.HistogramVec
	checkpointDuration *prometheus.HistogramVec
	modelScore         *prometheus.GaugeVec
	trainingErrors     *prometheus.CounterVec

	// A/B testing metrics
	variantSelections *prometheus.CounterVec
	variantLatency    *prometheus.HistogramVec
	variantErrors     *prometheus.CounterVec
	shadowComparisons *prometheus.CounterVec

	// Model drift and monitoring
	predictionDistribution *prometheus.HistogramVec
	inputDistribution      *prometheus.HistogramVec
	modelDriftScore        *prometheus.GaugeVec
}

// NewMLMetrics creates a new MLMetrics instance
func NewMLMetrics(namespace string) *MLMetrics {
	return &MLMetrics{
		// Model loading
		modelsLoaded: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: "ml",
				Name:      "models_loaded",
				Help:      "Number of ML models currently loaded",
			},
			[]string{"model", "version", "type"},
		),
		modelLoadTime: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Subsystem: "ml",
				Name:      "model_load_time_seconds",
				Help:      "Time taken to load ML models",
				Buckets:   []float64{.1, .5, 1, 2.5, 5, 10, 30, 60},
			},
			[]string{"model", "version", "type"},
		),
		modelLoadErrors: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: "ml",
				Name:      "model_load_errors_total",
				Help:      "Total number of model loading errors",
			},
			[]string{"model", "version", "type", "error_type"},
		),

		// Inference
		inferenceLatency: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Subsystem: "ml",
				Name:      "inference_latency_seconds",
				Help:      "ML inference latency in seconds",
				Buckets:   []float64{.0001, .0005, .001, .005, .01, .025, .05, .1, .25, .5, 1},
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
		inferenceErrors: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: "ml",
				Name:      "inference_errors_total",
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

		// Feature extraction
		featureExtractionTime: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Subsystem: "ml",
				Name:      "feature_extraction_time_seconds",
				Help:      "Feature extraction time in seconds",
				Buckets:   []float64{.0001, .0005, .001, .005, .01, .025, .05, .1},
			},
			[]string{"feature_set"},
		),
		featureExtractionErrors: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: "ml",
				Name:      "feature_extraction_errors_total",
				Help:      "Total number of feature extraction errors",
			},
			[]string{"feature_set", "error_type"},
		),
		featuresExtracted: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: "ml",
				Name:      "features_extracted_total",
				Help:      "Total number of features extracted",
			},
			[]string{"feature_set", "feature_type"},
		),

		// Online learning
		updatesProcessed: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: "ml",
				Name:      "online_updates_processed_total",
				Help:      "Total number of online learning updates processed",
			},
			[]string{"model", "version"},
		),
		updateLatency: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Subsystem: "ml",
				Name:      "online_update_latency_seconds",
				Help:      "Online learning update latency in seconds",
				Buckets:   []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5},
			},
			[]string{"model", "version"},
		),
		updateBatchSize: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Subsystem: "ml",
				Name:      "online_update_batch_size",
				Help:      "Online learning update batch size",
				Buckets:   []float64{1, 5, 10, 25, 50, 100, 250, 500, 1000},
			},
			[]string{"model", "version"},
		),
		checkpointDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Subsystem: "ml",
				Name:      "checkpoint_duration_seconds",
				Help:      "Model checkpoint duration in seconds",
				Buckets:   []float64{.1, .5, 1, 2.5, 5, 10, 30, 60},
			},
			[]string{"model", "version"},
		),
		modelScore: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: "ml",
				Name:      "model_score",
				Help:      "Current model validation score",
			},
			[]string{"model", "version", "metric"},
		),
		trainingErrors: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: "ml",
				Name:      "training_errors_total",
				Help:      "Total number of online learning training errors",
			},
			[]string{"model", "version", "error_type"},
		),

		// A/B testing
		variantSelections: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: "ml",
				Name:      "abtest_variant_selections_total",
				Help:      "Total number of times each variant was selected",
			},
			[]string{"experiment", "variant"},
		),
		variantLatency: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Subsystem: "ml",
				Name:      "abtest_variant_latency_seconds",
				Help:      "Latency of predictions by variant",
				Buckets:   []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5},
			},
			[]string{"experiment", "variant"},
		),
		variantErrors: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: "ml",
				Name:      "abtest_variant_errors_total",
				Help:      "Total number of errors by variant",
			},
			[]string{"experiment", "variant", "error_type"},
		),
		shadowComparisons: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: "ml",
				Name:      "abtest_shadow_comparisons_total",
				Help:      "Total number of shadow mode comparisons",
			},
			[]string{"experiment", "result"},
		),

		// Model drift and monitoring
		predictionDistribution: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Subsystem: "ml",
				Name:      "prediction_distribution",
				Help:      "Distribution of prediction values",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{"model", "version", "output"},
		),
		inputDistribution: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Subsystem: "ml",
				Name:      "input_distribution",
				Help:      "Distribution of input feature values",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{"model", "version", "feature"},
		),
		modelDriftScore: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: "ml",
				Name:      "model_drift_score",
				Help:      "Model drift detection score",
			},
			[]string{"model", "version", "metric"},
		),
	}
}

// Register registers all ML metrics with Prometheus
func (m *MLMetrics) Register(reg prometheus.Registerer) error {
	collectors := []prometheus.Collector{
		// Model loading
		m.modelsLoaded,
		m.modelLoadTime,
		m.modelLoadErrors,

		// Inference
		m.inferenceLatency,
		m.predictions,
		m.batchSize,
		m.inferenceErrors,
		m.queueDepth,
		m.activePredictions,

		// Feature extraction
		m.featureExtractionTime,
		m.featureExtractionErrors,
		m.featuresExtracted,

		// Online learning
		m.updatesProcessed,
		m.updateLatency,
		m.updateBatchSize,
		m.checkpointDuration,
		m.modelScore,
		m.trainingErrors,

		// A/B testing
		m.variantSelections,
		m.variantLatency,
		m.variantErrors,
		m.shadowComparisons,

		// Monitoring
		m.predictionDistribution,
		m.inputDistribution,
		m.modelDriftScore,
	}

	for _, collector := range collectors {
		if err := reg.Register(collector); err != nil {
			if _, ok := err.(prometheus.AlreadyRegisteredError); !ok {
				return err
			}
		}
	}
	return nil
}

// RecordModelLoad records a model loading event
func (m *MLMetrics) RecordModelLoad(model, version, modelType string, duration time.Duration, err error) {
	if err != nil {
		m.modelLoadErrors.WithLabelValues(model, version, modelType, "load_failed").Inc()
	} else {
		m.modelsLoaded.WithLabelValues(model, version, modelType).Inc()
		m.modelLoadTime.WithLabelValues(model, version, modelType).Observe(duration.Seconds())
	}
}

// RecordModelUnload records a model unloading event
func (m *MLMetrics) RecordModelUnload(model, version, modelType string) {
	m.modelsLoaded.WithLabelValues(model, version, modelType).Dec()
}

// RecordPrediction records a prediction event
func (m *MLMetrics) RecordPrediction(model, version string, duration time.Duration, batchSize int, err error) {
	if err != nil {
		m.predictions.WithLabelValues(model, version, "error").Inc()
		m.inferenceErrors.WithLabelValues(model, version, "prediction_failed").Inc()
	} else {
		m.predictions.WithLabelValues(model, version, "success").Inc()
		m.inferenceLatency.WithLabelValues(model, version).Observe(duration.Seconds())
		if batchSize > 0 {
			m.batchSize.WithLabelValues(model, version).Observe(float64(batchSize))
		}
	}
}

// RecordFeatureExtraction records a feature extraction event
func (m *MLMetrics) RecordFeatureExtraction(featureSet, featureType string, duration time.Duration, err error) {
	if err != nil {
		m.featureExtractionErrors.WithLabelValues(featureSet, "extraction_failed").Inc()
	} else {
		m.featureExtractionTime.WithLabelValues(featureSet).Observe(duration.Seconds())
		m.featuresExtracted.WithLabelValues(featureSet, featureType).Inc()
	}
}

// RecordOnlineUpdate records an online learning update
func (m *MLMetrics) RecordOnlineUpdate(model, version string, duration time.Duration, batchSize int, err error) {
	if err != nil {
		m.trainingErrors.WithLabelValues(model, version, "update_failed").Inc()
	} else {
		m.updatesProcessed.WithLabelValues(model, version).Add(float64(batchSize))
		m.updateLatency.WithLabelValues(model, version).Observe(duration.Seconds())
		m.updateBatchSize.WithLabelValues(model, version).Observe(float64(batchSize))
	}
}

// RecordCheckpoint records a model checkpoint event
func (m *MLMetrics) RecordCheckpoint(model, version string, duration time.Duration) {
	m.checkpointDuration.WithLabelValues(model, version).Observe(duration.Seconds())
}

// RecordModelScore records a model validation score
func (m *MLMetrics) RecordModelScore(model, version, metric string, score float64) {
	m.modelScore.WithLabelValues(model, version, metric).Set(score)
}

// RecordVariantSelection records an A/B test variant selection
func (m *MLMetrics) RecordVariantSelection(experiment, variant string) {
	m.variantSelections.WithLabelValues(experiment, variant).Inc()
}

// RecordVariantPrediction records an A/B test variant prediction
func (m *MLMetrics) RecordVariantPrediction(experiment, variant string, duration time.Duration, err error) {
	if err != nil {
		m.variantErrors.WithLabelValues(experiment, variant, "prediction_failed").Inc()
	} else {
		m.variantLatency.WithLabelValues(experiment, variant).Observe(duration.Seconds())
	}
}

// RecordShadowComparison records a shadow mode comparison result
func (m *MLMetrics) RecordShadowComparison(experiment string, match bool) {
	result := "match"
	if !match {
		result = "mismatch"
	}
	m.shadowComparisons.WithLabelValues(experiment, result).Inc()
}

// RecordPredictionValue records a prediction value for drift detection
func (m *MLMetrics) RecordPredictionValue(model, version, output string, value float64) {
	m.predictionDistribution.WithLabelValues(model, version, output).Observe(value)
}

// RecordInputValue records an input feature value for drift detection
func (m *MLMetrics) RecordInputValue(model, version, feature string, value float64) {
	m.inputDistribution.WithLabelValues(model, version, feature).Observe(value)
}

// RecordDriftScore records a model drift score
func (m *MLMetrics) RecordDriftScore(model, version, metric string, score float64) {
	m.modelDriftScore.WithLabelValues(model, version, metric).Set(score)
}

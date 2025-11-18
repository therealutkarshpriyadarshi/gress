package metrics

import (
	"net/http"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

// Collector holds all Prometheus metrics for the stream processing engine
type Collector struct {
	// Engine metrics
	EventsProcessed   *prometheus.CounterVec
	EventsFiltered    *prometheus.CounterVec
	ProcessingLatency *prometheus.HistogramVec
	BackpressureCount prometheus.Counter
	BufferUtilization prometheus.Gauge

	// Operator metrics
	OperatorEventsIn  *prometheus.CounterVec
	OperatorEventsOut *prometheus.CounterVec
	OperatorLatency   *prometheus.HistogramVec
	OperatorErrors    *prometheus.CounterVec

	// State backend metrics
	StateOperations *prometheus.CounterVec
	StateLatency    *prometheus.HistogramVec
	StateSize       *prometheus.GaugeVec

	// Watermark metrics
	WatermarkLag      *prometheus.GaugeVec
	WatermarkProgress prometheus.Gauge

	// Checkpoint metrics
	CheckpointDuration     prometheus.Histogram
	CheckpointSuccess      prometheus.Counter
	CheckpointFailure      prometheus.Counter
	CheckpointSize         prometheus.Gauge
	TimeSinceLastCheckpoint prometheus.Gauge

	// Window metrics
	WindowEventsProcessed *prometheus.CounterVec
	WindowLateFired       *prometheus.CounterVec
	WindowSize            *prometheus.GaugeVec

	// Error handling metrics
	ErrorMetrics *ErrorMetrics

	// Custom metrics registry for user applications
	customMetrics map[string]prometheus.Collector
	customMu      sync.RWMutex

	registry *prometheus.Registry
	logger   *zap.Logger
}

// NewCollector creates a new Prometheus metrics collector
func NewCollector(logger *zap.Logger) *Collector {
	registry := prometheus.NewRegistry()

	c := &Collector{
		registry:      registry,
		logger:        logger,
		customMetrics: make(map[string]prometheus.Collector),
	}

	c.initMetrics()
	c.registerMetrics()

	// Initialize error metrics
	c.ErrorMetrics = NewErrorMetrics(registry)

	return c
}

// initMetrics initializes all Prometheus metrics
func (c *Collector) initMetrics() {
	// Engine metrics
	c.EventsProcessed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gress_events_processed_total",
			Help: "Total number of events processed by the engine",
		},
		[]string{"source"},
	)

	c.EventsFiltered = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gress_events_filtered_total",
			Help: "Total number of events filtered out",
		},
		[]string{"operator"},
	)

	c.ProcessingLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "gress_processing_latency_seconds",
			Help:    "Event processing latency in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"source"},
	)

	c.BackpressureCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "gress_backpressure_events_total",
			Help: "Total number of backpressure events",
		},
	)

	c.BufferUtilization = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "gress_buffer_utilization_ratio",
			Help: "Current buffer utilization (0.0 to 1.0)",
		},
	)

	// Operator metrics
	c.OperatorEventsIn = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gress_operator_events_in_total",
			Help: "Total number of events received by operator",
		},
		[]string{"operator", "operator_type"},
	)

	c.OperatorEventsOut = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gress_operator_events_out_total",
			Help: "Total number of events emitted by operator",
		},
		[]string{"operator", "operator_type"},
	)

	c.OperatorLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "gress_operator_latency_seconds",
			Help:    "Operator processing latency in seconds",
			Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
		},
		[]string{"operator", "operator_type"},
	)

	c.OperatorErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gress_operator_errors_total",
			Help: "Total number of operator errors",
		},
		[]string{"operator", "operator_type", "error_type"},
	)

	// State backend metrics
	c.StateOperations = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gress_state_operations_total",
			Help: "Total number of state backend operations",
		},
		[]string{"backend", "operation"},
	)

	c.StateLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "gress_state_operation_latency_seconds",
			Help:    "State backend operation latency in seconds",
			Buckets: []float64{.0001, .0005, .001, .005, .01, .05, .1, .5, 1},
		},
		[]string{"backend", "operation"},
	)

	c.StateSize = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "gress_state_size_bytes",
			Help: "Current size of state in bytes",
		},
		[]string{"backend", "operator"},
	)

	// Watermark metrics
	c.WatermarkLag = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "gress_watermark_lag_seconds",
			Help: "Current watermark lag (processing time - event time)",
		},
		[]string{"partition"},
	)

	c.WatermarkProgress = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "gress_watermark_timestamp",
			Help: "Current watermark timestamp (unix seconds)",
		},
	)

	// Checkpoint metrics
	c.CheckpointDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "gress_checkpoint_duration_seconds",
			Help:    "Time taken to complete a checkpoint",
			Buckets: []float64{.1, .5, 1, 2, 5, 10, 30, 60},
		},
	)

	c.CheckpointSuccess = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "gress_checkpoint_success_total",
			Help: "Total number of successful checkpoints",
		},
	)

	c.CheckpointFailure = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "gress_checkpoint_failure_total",
			Help: "Total number of failed checkpoints",
		},
	)

	c.CheckpointSize = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "gress_checkpoint_size_bytes",
			Help: "Size of the last checkpoint in bytes",
		},
	)

	c.TimeSinceLastCheckpoint = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "gress_time_since_last_checkpoint_seconds",
			Help: "Time elapsed since the last successful checkpoint",
		},
	)

	// Window metrics
	c.WindowEventsProcessed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gress_window_events_processed_total",
			Help: "Total number of events processed by windows",
		},
		[]string{"window_type"},
	)

	c.WindowLateFired = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gress_window_late_fired_total",
			Help: "Total number of windows fired with late data",
		},
		[]string{"window_type"},
	)

	c.WindowSize = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "gress_window_size_events",
			Help: "Current number of events in active windows",
		},
		[]string{"window_type", "window_id"},
	)
}

// registerMetrics registers all metrics with the Prometheus registry
func (c *Collector) registerMetrics() {
	// Engine metrics
	c.registry.MustRegister(c.EventsProcessed)
	c.registry.MustRegister(c.EventsFiltered)
	c.registry.MustRegister(c.ProcessingLatency)
	c.registry.MustRegister(c.BackpressureCount)
	c.registry.MustRegister(c.BufferUtilization)

	// Operator metrics
	c.registry.MustRegister(c.OperatorEventsIn)
	c.registry.MustRegister(c.OperatorEventsOut)
	c.registry.MustRegister(c.OperatorLatency)
	c.registry.MustRegister(c.OperatorErrors)

	// State backend metrics
	c.registry.MustRegister(c.StateOperations)
	c.registry.MustRegister(c.StateLatency)
	c.registry.MustRegister(c.StateSize)

	// Watermark metrics
	c.registry.MustRegister(c.WatermarkLag)
	c.registry.MustRegister(c.WatermarkProgress)

	// Checkpoint metrics
	c.registry.MustRegister(c.CheckpointDuration)
	c.registry.MustRegister(c.CheckpointSuccess)
	c.registry.MustRegister(c.CheckpointFailure)
	c.registry.MustRegister(c.CheckpointSize)
	c.registry.MustRegister(c.TimeSinceLastCheckpoint)

	// Window metrics
	c.registry.MustRegister(c.WindowEventsProcessed)
	c.registry.MustRegister(c.WindowLateFired)
	c.registry.MustRegister(c.WindowSize)
}

// RegisterCustomMetric allows applications to register custom metrics
func (c *Collector) RegisterCustomMetric(name string, collector prometheus.Collector) error {
	c.customMu.Lock()
	defer c.customMu.Unlock()

	if _, exists := c.customMetrics[name]; exists {
		return prometheus.AlreadyRegisteredError{}
	}

	if err := c.registry.Register(collector); err != nil {
		return err
	}

	c.customMetrics[name] = collector
	c.logger.Info("Registered custom metric", zap.String("name", name))
	return nil
}

// UnregisterCustomMetric removes a custom metric
func (c *Collector) UnregisterCustomMetric(name string) bool {
	c.customMu.Lock()
	defer c.customMu.Unlock()

	if collector, exists := c.customMetrics[name]; exists {
		c.registry.Unregister(collector)
		delete(c.customMetrics, name)
		c.logger.Info("Unregistered custom metric", zap.String("name", name))
		return true
	}
	return false
}

// UpdateTimeSinceLastCheckpoint updates the checkpoint timing metric
func (c *Collector) UpdateTimeSinceLastCheckpoint(lastCheckpointTime time.Time) {
	if !lastCheckpointTime.IsZero() {
		c.TimeSinceLastCheckpoint.Set(time.Since(lastCheckpointTime).Seconds())
	}
}

// Handler returns an HTTP handler for the /metrics endpoint
func (c *Collector) Handler() http.Handler {
	return promhttp.HandlerFor(c.registry, promhttp.HandlerOpts{
		EnableOpenMetrics: true,
	})
}

// Server creates an HTTP server for metrics exposition
type Server struct {
	collector *Collector
	server    *http.Server
	logger    *zap.Logger
}

// NewServer creates a new metrics HTTP server
func NewServer(addr string, collector *Collector, logger *zap.Logger) *Server {
	mux := http.NewServeMux()
	mux.Handle("/metrics", collector.Handler())

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	return &Server{
		collector: collector,
		server: &http.Server{
			Addr:    addr,
			Handler: mux,
		},
		logger: logger,
	}
}

// Start starts the metrics HTTP server
func (s *Server) Start() error {
	s.logger.Info("Starting metrics server", zap.String("addr", s.server.Addr))

	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Error("Metrics server error", zap.Error(err))
		}
	}()

	return nil
}

// Stop gracefully stops the metrics server
func (s *Server) Stop() error {
	s.logger.Info("Stopping metrics server")
	return s.server.Close()
}

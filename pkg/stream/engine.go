package stream

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/therealutkarshpriyadarshi/gress/pkg/checkpoint"
	"github.com/therealutkarshpriyadarshi/gress/pkg/metrics"
	"github.com/therealutkarshpriyadarshi/gress/pkg/watermark"
	"go.uber.org/zap"
)

// Engine is the core stream processing engine
type Engine struct {
	logger           *zap.Logger
	operators        []Operator
	sources          []Source
	sinks            []Sink
	watermarks       *watermark.Manager
	checkpoint       *checkpoint.Manager
	metrics          *StreamMetrics // Legacy metrics struct
	metricsCollector *metrics.Collector // Prometheus metrics collector

	eventChan      chan *Event
	watermarkChan  chan *Watermark
	checkpointChan chan *Checkpoint

	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup

	config EngineConfig
}

// EngineConfig holds configuration for the stream engine
type EngineConfig struct {
	BufferSize            int
	MaxConcurrency        int
	CheckpointInterval    time.Duration
	WatermarkInterval     time.Duration
	MetricsInterval       time.Duration
	EnableBackpressure    bool
	BackpressureThreshold float64
	MetricsAddr           string // Prometheus metrics server address (e.g., ":9091")
	EnableMetrics         bool   // Enable Prometheus metrics collection
}

// DefaultEngineConfig returns default configuration
func DefaultEngineConfig() EngineConfig {
	return EngineConfig{
		BufferSize:            10000,
		MaxConcurrency:        100,
		CheckpointInterval:    30 * time.Second,
		WatermarkInterval:     5 * time.Second,
		MetricsInterval:       10 * time.Second,
		EnableBackpressure:    true,
		BackpressureThreshold: 0.8,
		MetricsAddr:           ":9091",
		EnableMetrics:         true,
	}
}

// NewEngine creates a new stream processing engine
func NewEngine(config EngineConfig, logger *zap.Logger) *Engine {
	if logger == nil {
		logger, _ = zap.NewProduction()
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Create Prometheus metrics collector if enabled
	var metricsCollector *metrics.Collector
	if config.EnableMetrics {
		metricsCollector = metrics.NewCollector(logger)
	}

	return &Engine{
		logger:           logger,
		operators:        make([]Operator, 0),
		sources:          make([]Source, 0),
		sinks:            make([]Sink, 0),
		eventChan:        make(chan *Event, config.BufferSize),
		watermarkChan:    make(chan *Watermark, 100),
		checkpointChan:   make(chan *Checkpoint, 10),
		ctx:              ctx,
		cancel:           cancel,
		config:           config,
		metricsCollector: metricsCollector,
		metrics: &StreamMetrics{
			EventsProcessed: 0,
			EventsFiltered:  0,
		},
	}
}

// AddSource registers a data source
func (e *Engine) AddSource(source Source) *Engine {
	e.sources = append(e.sources, source)
	e.logger.Info("Added source", zap.String("name", source.Name()))
	return e
}

// AddOperator adds a processing operator
// If metrics are enabled, the operator is automatically wrapped with instrumentation
func (e *Engine) AddOperator(op Operator) *Engine {
	if e.config.EnableMetrics && e.metricsCollector != nil {
		// Auto-instrument the operator
		operatorName := fmt.Sprintf("operator-%d", len(e.operators))
		operatorType := fmt.Sprintf("%T", op)
		instrumentedOp := NewInstrumentedOperator(op, operatorName, operatorType, e.metricsCollector)
		e.operators = append(e.operators, instrumentedOp)
		e.logger.Debug("Added instrumented operator",
			zap.String("name", operatorName),
			zap.String("type", operatorType))
	} else {
		e.operators = append(e.operators, op)
	}
	return e
}

// AddSink registers an output sink
func (e *Engine) AddSink(sink Sink) *Engine {
	e.sinks = append(e.sinks, sink)
	return e
}

// Start begins stream processing
func (e *Engine) Start() error {
	e.logger.Info("Starting stream processing engine",
		zap.Int("sources", len(e.sources)),
		zap.Int("operators", len(e.operators)),
		zap.Int("sinks", len(e.sinks)),
		zap.Bool("metrics_enabled", e.config.EnableMetrics))

	// Start Prometheus metrics server if enabled
	if e.config.EnableMetrics && e.metricsCollector != nil {
		metricsServer := metrics.NewServer(e.config.MetricsAddr, e.metricsCollector, e.logger)
		if err := metricsServer.Start(); err != nil {
			e.logger.Error("Failed to start metrics server", zap.Error(err))
		}
	}

	// Initialize watermark manager
	e.watermarks = watermark.NewManager(e.config.WatermarkInterval, e.logger)
	e.wg.Add(1)
	go e.watermarks.Start(e.ctx, e.watermarkChan)

	// Initialize checkpoint manager
	e.checkpoint = checkpoint.NewManager(e.config.CheckpointInterval, "./checkpoints", e.logger)

	// Start sources
	for _, source := range e.sources {
		e.wg.Add(1)
		go func(s Source) {
			defer e.wg.Done()
			if err := s.Start(e.ctx, e.eventChan); err != nil {
				e.logger.Error("Source error", zap.String("source", s.Name()), zap.Error(err))
			}
		}(source)
	}

	// Start processing pipeline
	e.wg.Add(1)
	go e.processingLoop()

	// Start metrics collector
	e.wg.Add(1)
	go e.metricsLoop()

	// Start checkpoint coordinator
	e.wg.Add(1)
	go e.checkpointLoop()

	return nil
}

// processingLoop is the main event processing loop
func (e *Engine) processingLoop() {
	defer e.wg.Done()

	// Create worker pool for concurrent processing
	semaphore := make(chan struct{}, e.config.MaxConcurrency)

	for {
		select {
		case <-e.ctx.Done():
			e.logger.Info("Processing loop shutting down")
			return

		case event := <-e.eventChan:
			// Check backpressure and update buffer utilization metrics
			bufferUtilization := float64(len(e.eventChan)) / float64(cap(e.eventChan))

			// Update buffer utilization metric
			if e.metricsCollector != nil {
				e.metricsCollector.BufferUtilization.Set(bufferUtilization)
			}

			// Check backpressure
			if e.config.EnableBackpressure && bufferUtilization > e.config.BackpressureThreshold {
				atomic.AddInt64(&e.metrics.BackpressureCount, 1)
				if e.metricsCollector != nil {
					e.metricsCollector.BackpressureCount.Inc()
				}
				e.logger.Warn("Backpressure detected",
					zap.Float64("utilization", bufferUtilization))
			}

			// Acquire semaphore for concurrency control
			semaphore <- struct{}{}

			e.wg.Add(1)
			go func(evt *Event) {
				defer e.wg.Done()
				defer func() { <-semaphore }()

				if err := e.processEvent(evt); err != nil {
					e.logger.Error("Event processing error", zap.Error(err))
				}
			}(event)

		case wm := <-e.watermarkChan:
			e.handleWatermark(wm)
		}
	}
}

// processEvent processes a single event through the pipeline
func (e *Engine) processEvent(event *Event) error {
	startTime := time.Now()

	// Determine source name for metrics
	sourceName := "unknown"
	if len(e.sources) > 0 {
		sourceName = e.sources[0].Name()
	}

	ctx := &ProcessingContext{
		Ctx:       e.ctx,
		Timestamp: event.EventTime,
	}

	// Apply operators in sequence
	events := []*Event{event}
	for _, op := range e.operators {
		var nextEvents []*Event
		for _, evt := range events {
			results, err := op.Process(ctx, evt)
			if err != nil {
				return fmt.Errorf("operator error: %w", err)
			}
			nextEvents = append(nextEvents, results...)
		}
		events = nextEvents

		// If no events remain, they were filtered out
		if len(events) == 0 {
			atomic.AddInt64(&e.metrics.EventsFiltered, 1)
			return nil
		}
	}

	// Write to sinks
	for _, sink := range e.sinks {
		for _, evt := range events {
			if err := sink.Write(e.ctx, evt); err != nil {
				e.logger.Error("Sink write error", zap.Error(err))
			}
		}
	}

	// Update legacy metrics
	atomic.AddInt64(&e.metrics.EventsProcessed, 1)
	latency := time.Since(startTime)
	e.updateLatencyMetrics(latency)

	// Update Prometheus metrics
	if e.metricsCollector != nil {
		e.metricsCollector.EventsProcessed.WithLabelValues(sourceName).Inc()
		e.metricsCollector.ProcessingLatency.WithLabelValues(sourceName).Observe(latency.Seconds())
	}

	return nil
}

// handleWatermark processes a watermark event
func (e *Engine) handleWatermark(wm *Watermark) {
	e.metrics.CurrentWatermark = wm.Timestamp

	// Update Prometheus metrics
	if e.metricsCollector != nil {
		RecordWatermarkProgress(e.metricsCollector, wm.Timestamp, wm.Partition)
	}

	e.logger.Debug("Watermark advanced",
		zap.Time("timestamp", wm.Timestamp),
		zap.Int32("partition", wm.Partition))
}

// metricsLoop periodically logs metrics
func (e *Engine) metricsLoop() {
	defer e.wg.Done()
	ticker := time.NewTicker(e.config.MetricsInterval)
	defer ticker.Stop()

	for {
		select {
		case <-e.ctx.Done():
			return
		case <-ticker.C:
			e.logMetrics()
		}
	}
}

// checkpointLoop periodically creates checkpoints
func (e *Engine) checkpointLoop() {
	defer e.wg.Done()
	ticker := time.NewTicker(e.config.CheckpointInterval)
	defer ticker.Stop()

	for {
		select {
		case <-e.ctx.Done():
			return
		case <-ticker.C:
			if err := e.createCheckpoint(); err != nil {
				e.logger.Error("Checkpoint error", zap.Error(err))
			}
		}
	}
}

// createCheckpoint creates a consistent checkpoint
func (e *Engine) createCheckpoint() error {
	startTime := time.Now()
	e.logger.Info("Creating checkpoint")

	checkpoint := &Checkpoint{
		ID:        fmt.Sprintf("checkpoint-%d", time.Now().Unix()),
		Timestamp: time.Now(),
		Offsets:   make(map[int32]int64),
	}

	// TODO: Collect state from all operators

	e.metrics.LastCheckpointAt = time.Now()

	// Update Prometheus metrics
	duration := time.Since(startTime)
	if e.metricsCollector != nil {
		RecordCheckpointDuration(e.metricsCollector, duration, true)
		e.metricsCollector.UpdateTimeSinceLastCheckpoint(e.metrics.LastCheckpointAt)
		// Estimate checkpoint size (will be more accurate when state collection is implemented)
		e.metricsCollector.CheckpointSize.Set(float64(len(checkpoint.StateData)))
	}

	e.logger.Info("Checkpoint created",
		zap.String("id", checkpoint.ID),
		zap.Duration("duration", duration))

	return nil
}

// updateLatencyMetrics updates latency statistics
func (e *Engine) updateLatencyMetrics(latency time.Duration) {
	latencyMs := float64(latency.Milliseconds())

	// Simple moving average (in production, use proper percentile tracking)
	currentAvg := e.metrics.AvgLatencyMs
	count := atomic.LoadInt64(&e.metrics.EventsProcessed)
	e.metrics.AvgLatencyMs = (currentAvg*float64(count-1) + latencyMs) / float64(count)

	// Update P99 (simplified - use proper histogram in production)
	if latencyMs > e.metrics.P99LatencyMs {
		e.metrics.P99LatencyMs = latencyMs
	}
}

// logMetrics logs current processing metrics
func (e *Engine) logMetrics() {
	processed := atomic.LoadInt64(&e.metrics.EventsProcessed)
	filtered := atomic.LoadInt64(&e.metrics.EventsFiltered)
	backpressure := atomic.LoadInt64(&e.metrics.BackpressureCount)

	e.logger.Info("Stream metrics",
		zap.Int64("events_processed", processed),
		zap.Int64("events_filtered", filtered),
		zap.Float64("avg_latency_ms", e.metrics.AvgLatencyMs),
		zap.Float64("p99_latency_ms", e.metrics.P99LatencyMs),
		zap.Int64("backpressure_events", backpressure),
		zap.Int("buffer_size", len(e.eventChan)),
		zap.Time("current_watermark", e.metrics.CurrentWatermark))
}

// Stop gracefully shuts down the engine
func (e *Engine) Stop() error {
	e.logger.Info("Stopping stream processing engine")

	// Signal shutdown
	e.cancel()

	// Stop sources
	for _, source := range e.sources {
		if err := source.Stop(); err != nil {
			e.logger.Error("Error stopping source", zap.Error(err))
		}
	}

	// Wait for all goroutines
	e.wg.Wait()

	// Flush sinks
	for _, sink := range e.sinks {
		if err := sink.Flush(e.ctx); err != nil {
			e.logger.Error("Error flushing sink", zap.Error(err))
		}
		if err := sink.Close(); err != nil {
			e.logger.Error("Error closing sink", zap.Error(err))
		}
	}

	// Close operators
	for _, op := range e.operators {
		if err := op.Close(); err != nil {
			e.logger.Error("Error closing operator", zap.Error(err))
		}
	}

	e.logger.Info("Stream processing engine stopped")
	return nil
}

// GetMetrics returns current processing metrics
func (e *Engine) GetMetrics() StreamMetrics {
	return StreamMetrics{
		EventsProcessed:   atomic.LoadInt64(&e.metrics.EventsProcessed),
		EventsFiltered:    atomic.LoadInt64(&e.metrics.EventsFiltered),
		AvgLatencyMs:      e.metrics.AvgLatencyMs,
		P99LatencyMs:      e.metrics.P99LatencyMs,
		CurrentWatermark:  e.metrics.CurrentWatermark,
		LastCheckpointAt:  e.metrics.LastCheckpointAt,
		BackpressureCount: atomic.LoadInt64(&e.metrics.BackpressureCount),
	}
}

// GetMetricsCollector returns the Prometheus metrics collector
// This allows applications to register custom metrics
func (e *Engine) GetMetricsCollector() *metrics.Collector {
	return e.metricsCollector
}

package main

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/therealutkarshpriyadarshi/gress/pkg/stream"
	"go.uber.org/zap"
)

// This example demonstrates:
// 1. Built-in Prometheus metrics collection
// 2. Custom application metrics
// 3. Accessing the metrics endpoint

func main() {
	// Initialize logger
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	logger.Info("Starting Gress Metrics Example")
	logger.Info("Metrics endpoint will be available at http://localhost:9091/metrics")
	logger.Info("Health check available at http://localhost:9091/health")

	// Create stream processing engine with metrics enabled
	config := stream.DefaultEngineConfig()
	config.EnableMetrics = true  // Enable Prometheus metrics (default: true)
	config.MetricsAddr = ":9091" // Metrics server address (default: :9091)
	config.CheckpointInterval = 30 * time.Second
	config.WatermarkInterval = 5 * time.Second
	config.MetricsInterval = 10 * time.Second

	engine := stream.NewEngine(config, logger)

	// Register custom application metrics
	registerCustomMetrics(engine, logger)

	// Setup processing pipeline
	setupPipeline(engine, logger)

	// Setup mock data source
	mockSource := &MockSource{
		output: make(chan *stream.Event, 100),
		logger: logger,
	}
	engine.AddSource(mockSource)

	// Add sink
	sink := &MetricsSink{logger: logger}
	engine.AddSink(sink)

	// Start the engine
	if err := engine.Start(); err != nil {
		logger.Fatal("Failed to start engine", zap.Error(err))
	}

	logger.Info("System started successfully!")
	logger.Info("To view metrics:")
	logger.Info("  1. curl http://localhost:9091/metrics")
	logger.Info("  2. Open Prometheus at http://localhost:9090 (if running)")
	logger.Info("  3. Open Grafana at http://localhost:3000 (if running)")

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	logger.Info("Shutting down...")
	if err := engine.Stop(); err != nil {
		logger.Error("Error stopping engine", zap.Error(err))
	}

	// Print final metrics
	printMetrics(engine)
}

func setupPipeline(engine *stream.Engine, logger *zap.Logger) {
	// Filter operator - filters out low-priority events
	filterOp := stream.NewFilterOperator(func(event *stream.Event) bool {
		data, ok := event.Value.(map[string]interface{})
		if !ok {
			return false
		}

		priority, ok := data["priority"].(int)
		if !ok {
			return true
		}

		// Filter out low priority events (< 5)
		return priority >= 5
	})
	engine.AddOperator(filterOp)

	// Map operator - enrich events
	mapOp := stream.NewMapOperator(func(event *stream.Event) (*stream.Event, error) {
		data, ok := event.Value.(map[string]interface{})
		if !ok {
			return event, nil
		}

		// Add processing timestamp
		data["processed_at"] = time.Now().Format(time.RFC3339)
		data["enriched"] = true

		event.Value = data
		return event, nil
	})
	engine.AddOperator(mapOp)

	// KeyBy operator - partition by user_id
	keyByOp := stream.NewKeyByOperator(func(event *stream.Event) string {
		data, ok := event.Value.(map[string]interface{})
		if !ok {
			return "unknown"
		}

		userID, ok := data["user_id"].(string)
		if !ok {
			return "unknown"
		}

		return userID
	})
	engine.AddOperator(keyByOp)
}

func registerCustomMetrics(engine *stream.Engine, logger *zap.Logger) {
	// Get the metrics collector from the engine
	collector := engine.GetMetricsCollector()
	if collector == nil {
		logger.Warn("Metrics collector not available")
		return
	}

	// Create custom counter metric
	customCounter := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "gress_example_events_processed",
		Help: "Total number of events processed by example application",
	})

	// Create custom gauge metric
	customGauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "gress_example_active_users",
		Help: "Number of active users in the system",
	})

	// Create custom histogram metric
	customHistogram := prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "gress_example_event_size_bytes",
		Help:    "Size of events in bytes",
		Buckets: prometheus.ExponentialBuckets(100, 2, 10),
	})

	// Register custom metrics
	if err := collector.RegisterCustomMetric("events_processed", customCounter); err != nil {
		logger.Error("Failed to register custom counter", zap.Error(err))
	}

	if err := collector.RegisterCustomMetric("active_users", customGauge); err != nil {
		logger.Error("Failed to register custom gauge", zap.Error(err))
	}

	if err := collector.RegisterCustomMetric("event_size", customHistogram); err != nil {
		logger.Error("Failed to register custom histogram", zap.Error(err))
	}

	logger.Info("Custom metrics registered successfully")

	// Simulate updating custom metrics
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			customCounter.Inc()
			customGauge.Set(float64(rand.Intn(100)))
			customHistogram.Observe(float64(rand.Intn(10000)))
		}
	}()
}

func printMetrics(engine *stream.Engine) {
	metrics := engine.GetMetrics()

	fmt.Println("\n=== Final Metrics ===")
	fmt.Printf("Events Processed:   %d\n", metrics.EventsProcessed)
	fmt.Printf("Events Filtered:    %d\n", metrics.EventsFiltered)
	fmt.Printf("Avg Latency:        %.2f ms\n", metrics.AvgLatencyMs)
	fmt.Printf("P99 Latency:        %.2f ms\n", metrics.P99LatencyMs)
	fmt.Printf("Backpressure Count: %d\n", metrics.BackpressureCount)
	fmt.Printf("Last Checkpoint:    %s\n", metrics.LastCheckpointAt.Format(time.RFC3339))
	fmt.Println("====================\n")
}

// MockSource generates random events for demonstration
type MockSource struct {
	output chan *stream.Event
	logger *zap.Logger
}

func (m *MockSource) Start(ctx context.Context, output chan<- *stream.Event) error {
	m.logger.Info("Starting mock data source")

	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		userIDs := []string{"user1", "user2", "user3", "user4", "user5"}
		eventTypes := []string{"click", "view", "purchase", "search"}

		for {
			select {
			case <-ctx.Done():
				m.logger.Info("Mock source stopped")
				return
			case <-ticker.C:
				// Generate random event
				event := &stream.Event{
					Key: userIDs[rand.Intn(len(userIDs))],
					Value: map[string]interface{}{
						"user_id":    userIDs[rand.Intn(len(userIDs))],
						"event_type": eventTypes[rand.Intn(len(eventTypes))],
						"priority":   rand.Intn(10),
						"timestamp":  time.Now().Format(time.RFC3339),
						"data_size":  rand.Intn(5000),
					},
					EventTime: time.Now(),
					Headers: map[string]string{
						"source": "mock-generator",
					},
				}

				select {
				case output <- event:
				default:
					// Channel full, skip
				}
			}
		}
	}()

	return nil
}

func (m *MockSource) Stop() error {
	m.logger.Info("Stopping mock source")
	return nil
}

func (m *MockSource) Name() string {
	return "mock-source"
}

func (m *MockSource) Partitions() int32 {
	return 1
}

// MetricsSink writes events to the console
type MetricsSink struct {
	logger  *zap.Logger
	counter int
}

func (s *MetricsSink) Write(ctx context.Context, event *stream.Event) error {
	s.counter++

	// Log every 100th event
	if s.counter%100 == 0 {
		s.logger.Info("Processed events",
			zap.Int("count", s.counter),
			zap.String("key", event.Key))
	}

	return nil
}

func (s *MetricsSink) Flush(ctx context.Context) error {
	return nil
}

func (s *MetricsSink) Close() error {
	s.logger.Info("Total events written", zap.Int("count", s.counter))
	return nil
}

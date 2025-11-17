package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/therealutkarshpriyadarshi/gress/pkg/ingestion"
	"github.com/therealutkarshpriyadarshi/gress/pkg/sink"
	"github.com/therealutkarshpriyadarshi/gress/pkg/stream"
	"github.com/therealutkarshpriyadarshi/gress/pkg/window"
	"go.uber.org/zap"
)

// RideRequest represents an incoming ride request
type RideRequest struct {
	RequestID string    `json:"request_id"`
	UserID    string    `json:"user_id"`
	Location  Location  `json:"location"`
	Timestamp time.Time `json:"timestamp"`
}

// DriverLocation represents a driver's current location
type DriverLocation struct {
	DriverID   string    `json:"driver_id"`
	Location   Location  `json:"location"`
	Available  bool      `json:"available"`
	Timestamp  time.Time `json:"timestamp"`
}

// Location represents a geographic location
type Location struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Area      string  `json:"area"` // e.g., "downtown", "airport"
}

// DemandSupplyMetrics tracks demand/supply ratio per area
type DemandSupplyMetrics struct {
	Area            string
	RideRequests    int
	AvailableDrivers int
	Ratio           float64
	SurgeMultiplier float64
}

func main() {
	// Initialize logger
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	logger.Info("Starting Ride-Sharing Dynamic Pricing System")

	// Create stream processing engine
	config := stream.DefaultEngineConfig()
	config.CheckpointInterval = 30 * time.Second
	config.WatermarkInterval = 5 * time.Second

	engine := stream.NewEngine(config, logger)

	// Setup HTTP source for ride requests
	httpSource := ingestion.NewHTTPSource(":8080", "/ride-requests", logger)
	engine.AddSource(httpSource)

	// Setup WebSocket source for driver locations
	wsSource := ingestion.NewWebSocketSource(":8081", "/driver-locations", logger)
	engine.AddSource(wsSource)

	// Create processing pipeline
	setupPipeline(engine, logger)

	// Start the engine
	if err := engine.Start(); err != nil {
		logger.Fatal("Failed to start engine", zap.Error(err))
	}

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	logger.Info("Shutting down...")
	if err := engine.Stop(); err != nil {
		logger.Error("Error stopping engine", zap.Error(err))
	}
}

func setupPipeline(engine *stream.Engine, logger *zap.Logger) {
	// Filter for ride requests in active areas
	filterActiveAreas := stream.NewFilterOperator(func(event *stream.Event) bool {
		data, ok := event.Value.(map[string]interface{})
		if !ok {
			return false
		}

		location, ok := data["location"].(map[string]interface{})
		if !ok {
			return false
		}

		area, ok := location["area"].(string)
		if !ok {
			return false
		}

		// Only process requests in monitored areas
		activeAreas := map[string]bool{
			"downtown": true,
			"airport":  true,
			"university": true,
			"mall": true,
		}

		return activeAreas[area]
	})
	engine.AddOperator(filterActiveAreas)

	// Extract area as key for partitioning
	keyByArea := stream.NewKeyByOperator(func(event *stream.Event) string {
		data, ok := event.Value.(map[string]interface{})
		if !ok {
			return "unknown"
		}

		location, ok := data["location"].(map[string]interface{})
		if !ok {
			return "unknown"
		}

		area, ok := location["area"].(string)
		if !ok {
			return "unknown"
		}

		return area
	})
	engine.AddOperator(keyByArea)

	// Assign timestamps for event-time processing
	timestampAssigner := stream.NewTimestampAssignerOperator(func(event *stream.Event) time.Time {
		data, ok := event.Value.(map[string]interface{})
		if !ok {
			return time.Now()
		}

		timestampStr, ok := data["timestamp"].(string)
		if !ok {
			return time.Now()
		}

		timestamp, err := time.Parse(time.RFC3339, timestampStr)
		if err != nil {
			return time.Now()
		}

		return timestamp
	})
	engine.AddOperator(timestampAssigner)

	// Create 5-minute tumbling windows for demand calculation
	windowAssigner := window.NewTumblingWindow(5 * time.Minute)

	// Aggregate demand per area
	demandAggregator := stream.NewAggregateOperator(func(accumulator interface{}, event *stream.Event) (interface{}, error) {
		var metrics *DemandSupplyMetrics
		if accumulator == nil {
			metrics = &DemandSupplyMetrics{
				Area: event.Key,
			}
		} else {
			metrics = accumulator.(*DemandSupplyMetrics)
		}

		// Check event type
		data, ok := event.Value.(map[string]interface{})
		if !ok {
			return metrics, nil
		}

		if _, isRideRequest := data["request_id"]; isRideRequest {
			metrics.RideRequests++
		} else if available, isDriver := data["available"].(bool); isDriver {
			if available {
				metrics.AvailableDrivers++
			}
		}

		// Calculate demand/supply ratio
		if metrics.AvailableDrivers > 0 {
			metrics.Ratio = float64(metrics.RideRequests) / float64(metrics.AvailableDrivers)
		} else {
			metrics.Ratio = float64(metrics.RideRequests)
		}

		// Calculate surge multiplier using exponential function
		// No surge: ratio < 1.0 (more drivers than requests)
		// 1.5x surge: ratio = 2.0
		// 2.0x surge: ratio = 4.0
		// 3.0x surge: ratio = 8.0+
		metrics.SurgeMultiplier = calculateSurgeMultiplier(metrics.Ratio)

		return metrics, nil
	})
	engine.AddOperator(demandAggregator)

	// Map to pricing update events
	pricingMapper := stream.NewMapOperator(func(event *stream.Event) (*stream.Event, error) {
		metrics, ok := event.Value.(*DemandSupplyMetrics)
		if !ok {
			return event, nil
		}

		pricingUpdate := map[string]interface{}{
			"area":             metrics.Area,
			"surge_multiplier": metrics.SurgeMultiplier,
			"demand_supply_ratio": metrics.Ratio,
			"ride_requests":    metrics.RideRequests,
			"available_drivers": metrics.AvailableDrivers,
			"timestamp":        time.Now().Format(time.RFC3339),
		}

		logger.Info("Pricing Update",
			zap.String("area", metrics.Area),
			zap.Float64("surge", metrics.SurgeMultiplier),
			zap.Float64("ratio", metrics.Ratio),
			zap.Int("requests", metrics.RideRequests),
			zap.Int("drivers", metrics.AvailableDrivers))

		return &stream.Event{
			Key:       event.Key,
			Value:     pricingUpdate,
			EventTime: event.EventTime,
			Headers:   event.Headers,
		}, nil
	})
	engine.AddOperator(pricingMapper)

	// Add console sink for demonstration
	consoleSink := &ConsoleSink{logger: logger}
	engine.AddSink(consoleSink)
}

// calculateSurgeMultiplier computes the surge pricing multiplier
func calculateSurgeMultiplier(ratio float64) float64 {
	if ratio < 1.0 {
		return 1.0 // No surge
	}

	// Exponential surge: 1.0 + log2(ratio) * 0.5
	surge := 1.0 + (math.Log2(ratio) * 0.5)

	// Cap at 3.0x
	if surge > 3.0 {
		return 3.0
	}

	// Round to 1 decimal place
	return math.Round(surge*10) / 10
}

// ConsoleSink writes events to console
type ConsoleSink struct {
	logger *zap.Logger
}

func (c *ConsoleSink) Write(ctx context.Context, event *stream.Event) error {
	data, _ := json.MarshalIndent(event.Value, "", "  ")
	fmt.Println("=== Pricing Update ===")
	fmt.Println(string(data))
	fmt.Println()
	return nil
}

func (c *ConsoleSink) Flush(ctx context.Context) error {
	return nil
}

func (c *ConsoleSink) Close() error {
	return nil
}

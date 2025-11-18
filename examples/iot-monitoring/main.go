package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/rand"
	"time"

	"github.com/therealutkarshpriyadarshi/gress/pkg/config"
	"github.com/therealutkarshpriyadarshi/gress/pkg/ingestion"
	"github.com/therealutkarshpriyadarshi/gress/pkg/iot"
	"github.com/therealutkarshpriyadarshi/gress/pkg/sink"
	"github.com/therealutkarshpriyadarshi/gress/pkg/stream"
	"github.com/therealutkarshpriyadarshi/gress/pkg/window"
)

/*
IoT Sensor Monitoring System Example

This example demonstrates a comprehensive IoT monitoring system with:
1. Multi-sensor aggregation across devices
2. Threshold-based alerting
3. Predictive maintenance using trend analysis
4. Real-time health monitoring

Architecture:
- WebSocket/MQTT ingestion for sensor readings
- Processing pipelines:
  * Multi-sensor aggregation by device
  * Threshold monitoring and alerting
  * Predictive maintenance analysis
  * Health score calculation
- Alerts and predictions output to Kafka and TimescaleDB

Data Flow:
Sensor Readings -> Aggregator -> Threshold Monitor -> Predictive Maintenance -> Alerts

Sample Event:
{
  "sensor_id": "temp-sensor-001",
  "device_id": "device-123",
  "location": "Factory Floor A",
  "sensor_type": "temperature",
  "value": 75.5,
  "unit": "celsius",
  "timestamp": "2024-01-15T10:30:00Z",
  "quality": "good"
}

Use Cases:
- Manufacturing equipment monitoring
- Data center temperature tracking
- Industrial machinery predictive maintenance
- Building HVAC optimization
*/

func main() {
	log.Println("Starting IoT Sensor Monitoring System...")

	// Load configuration
	cfg, err := config.LoadConfig("configs/iot-monitoring.yaml")
	if err != nil {
		cfg = createDefaultIoTConfig()
	}

	ctx := context.Background()

	// Create stream engine
	engine := stream.NewEngine(cfg)

	// 1. Multi-Sensor Aggregation Pipeline
	aggregationPipeline := createAggregationPipeline(cfg)
	if err := engine.AddPipeline("sensor-aggregation", aggregationPipeline); err != nil {
		log.Fatalf("Failed to add aggregation pipeline: %v", err)
	}

	// 2. Threshold Alerting Pipeline
	alertingPipeline := createAlertingPipeline(cfg)
	if err := engine.AddPipeline("threshold-alerting", alertingPipeline); err != nil {
		log.Fatalf("Failed to add alerting pipeline: %v", err)
	}

	// 3. Predictive Maintenance Pipeline
	maintenancePipeline := createMaintenancePipeline(cfg)
	if err := engine.AddPipeline("predictive-maintenance", maintenancePipeline); err != nil {
		log.Fatalf("Failed to add maintenance pipeline: %v", err)
	}

	// 4. Sensor Health Monitoring Pipeline
	healthPipeline := createHealthMonitoringPipeline(cfg)
	if err := engine.AddPipeline("health-monitoring", healthPipeline); err != nil {
		log.Fatalf("Failed to add health monitoring pipeline: %v", err)
	}

	// Start the engine
	if err := engine.Start(ctx); err != nil {
		log.Fatalf("Failed to start engine: %v", err)
	}

	// Generate sample sensor data
	go generateSampleSensorData(cfg)

	log.Println("IoT Monitoring System is running...")
	log.Println("WebSocket endpoint: ws://localhost:8082/sensors")
	log.Println("HTTP endpoint: http://localhost:8082/readings")
	log.Println("Metrics endpoint: http://localhost:9093/metrics")
	log.Println("Press Ctrl+C to stop")

	// Wait for termination signal
	select {}
}

func createAggregationPipeline(cfg *config.Config) *stream.Pipeline {
	// WebSocket source for real-time sensor readings
	wsSource := ingestion.NewWebSocketSource(ingestion.WebSocketSourceConfig{
		ListenAddr: ":8082",
		Path:       "/sensors",
	})

	// Tumbling window (1 minute aggregation)
	tumblingWindow := window.NewTumblingWindow(1 * time.Minute)

	// Multi-sensor aggregator
	aggregator := iot.MultiSensorAggregationOperator(1 * time.Minute)

	// TimescaleDB sink for historical data
	timescaleSink := sink.NewTimescaleSink(sink.TimescaleSinkConfig{
		ConnectionString: cfg.Sinks.TimescaleDB.ConnectionString,
		Table:            "sensor_aggregates",
		BatchSize:        100,
		FlushInterval:    5 * time.Second,
	})

	return &stream.Pipeline{
		Source: wsSource,
		Operators: []stream.Operator{
			tumblingWindow,
			aggregator,
		},
		Sink: timescaleSink,
	}
}

func createAlertingPipeline(cfg *config.Config) *stream.Pipeline {
	// Kafka source for sensor readings
	kafkaSource := ingestion.NewKafkaSource(ingestion.KafkaSourceConfig{
		Brokers:     cfg.Sources.Kafka.Brokers,
		Topics:      []string{"sensor-readings"},
		GroupID:     "threshold-alerting",
		StartOffset: "latest",
	})

	// Define threshold rules
	rules := []iot.ThresholdRule{
		{
			RuleID:      "temp-high",
			SensorType:  "temperature",
			MinValue:    -10,
			MaxValue:    80,
			Severity:    "critical",
			Description: "Temperature exceeds safe operating range",
		},
		{
			RuleID:      "pressure-high",
			SensorType:  "pressure",
			MinValue:    0,
			MaxValue:    150,
			Severity:    "warning",
			Description: "Pressure above normal threshold",
		},
		{
			RuleID:      "vibration-high",
			SensorType:  "vibration",
			MinValue:    0,
			MaxValue:    50,
			Severity:    "critical",
			Description: "Excessive vibration detected",
		},
		{
			RuleID:      "humidity-range",
			SensorType:  "humidity",
			MinValue:    30,
			MaxValue:    70,
			Severity:    "warning",
			Description: "Humidity outside optimal range",
		},
	}

	// Threshold alert operator
	alertOperator := iot.ThresholdAlertOperator(rules)

	// Kafka sink for alerts
	alertSink := sink.NewKafkaSink(sink.KafkaSinkConfig{
		Brokers: cfg.Sinks.Kafka.Brokers,
		Topic:   "sensor-alerts",
	})

	return &stream.Pipeline{
		Source: kafkaSource,
		Operators: []stream.Operator{
			alertOperator,
		},
		Sink: alertSink,
	}
}

func createMaintenancePipeline(cfg *config.Config) *stream.Pipeline {
	// Kafka source for sensor readings
	kafkaSource := ingestion.NewKafkaSource(ingestion.KafkaSourceConfig{
		Brokers:     cfg.Sources.Kafka.Brokers,
		Topics:      []string{"sensor-readings"},
		GroupID:     "predictive-maintenance",
		StartOffset: "latest",
	})

	// Predictive maintenance operator
	// History size: 100 readings, degradation threshold: 10%
	maintenanceOperator := iot.PredictiveMaintenanceOperator(100, 0.10)

	// Filter only maintenance-required predictions
	maintenanceFilter := stream.FilterOperator(func(event *stream.Event) bool {
		var prediction iot.MaintenancePrediction
		if err := json.Unmarshal(event.Data, &prediction); err != nil {
			return false
		}
		return prediction.RequiresMaintenance
	})

	// Kafka sink for maintenance alerts
	maintenanceSink := sink.NewKafkaSink(sink.KafkaSinkConfig{
		Brokers: cfg.Sinks.Kafka.Brokers,
		Topic:   "maintenance-predictions",
	})

	return &stream.Pipeline{
		Source: kafkaSource,
		Operators: []stream.Operator{
			maintenanceOperator,
			maintenanceFilter,
		},
		Sink: maintenanceSink,
	}
}

func createHealthMonitoringPipeline(cfg *config.Config) *stream.Pipeline {
	// Kafka source for maintenance predictions
	kafkaSource := ingestion.NewKafkaSource(ingestion.KafkaSourceConfig{
		Brokers:     cfg.Sources.Kafka.Brokers,
		Topics:      []string{"maintenance-predictions"},
		GroupID:     "health-monitoring",
		StartOffset: "latest",
	})

	// Enrich with additional context
	enrichmentOperator := stream.MapOperator(func(event *stream.Event) (*stream.Event, error) {
		var prediction iot.MaintenancePrediction
		if err := json.Unmarshal(event.Data, &prediction); err != nil {
			return nil, err
		}

		// Add enrichment
		enriched := map[string]interface{}{
			"prediction":           prediction,
			"notification_sent":    true,
			"maintenance_window":   calculateMaintenanceWindow(prediction),
			"estimated_cost":       estimateMaintenanceCost(prediction),
			"recommended_actions":  getRecommendedActions(prediction),
		}

		enrichedData, err := json.Marshal(enriched)
		if err != nil {
			return nil, err
		}

		event.Data = enrichedData
		return event, nil
	})

	// TimescaleDB sink for tracking
	timescaleSink := sink.NewTimescaleSink(sink.TimescaleSinkConfig{
		ConnectionString: cfg.Sinks.TimescaleDB.ConnectionString,
		Table:            "maintenance_schedule",
		BatchSize:        20,
		FlushInterval:    10 * time.Second,
	})

	return &stream.Pipeline{
		Source: kafkaSource,
		Operators: []stream.Operator{
			enrichmentOperator,
		},
		Sink: timescaleSink,
	}
}

func generateSampleSensorData(cfg *config.Config) {
	time.Sleep(2 * time.Second)

	// Simulate 10 devices with 3-5 sensors each
	devices := make([]string, 10)
	for i := 0; i < 10; i++ {
		devices[i] = fmt.Sprintf("device-%03d", i+1)
	}

	locations := []string{
		"Factory Floor A",
		"Factory Floor B",
		"Data Center Room 1",
		"Data Center Room 2",
		"Warehouse Zone A",
	}

	log.Println("Generating sample sensor data...")

	// Simulate degrading sensors
	degradationFactors := make(map[string]float64)

	for {
		// Generate 50-100 sensor readings per second
		numReadings := rand.Intn(50) + 50

		for i := 0; i < numReadings; i++ {
			deviceID := devices[rand.Intn(len(devices))]
			sensorType := []string{"temperature", "pressure", "vibration", "humidity"}[rand.Intn(4)]
			sensorID := fmt.Sprintf("%s-sensor-%s-%d", sensorType, deviceID, rand.Intn(3)+1)

			// Get or initialize degradation factor
			if _, exists := degradationFactors[sensorID]; !exists {
				degradationFactors[sensorID] = 1.0
			}

			// Slowly degrade some sensors
			if rand.Float64() < 0.01 { // 1% chance to start degrading
				degradationFactors[sensorID] *= 1.01 // 1% increase
			}

			// Generate base value based on sensor type
			var baseValue float64
			var unit string
			var quality string

			switch sensorType {
			case "temperature":
				baseValue = 25 + rand.Float64()*30 // 25-55°C
				unit = "celsius"
			case "pressure":
				baseValue = 100 + rand.Float64()*30 // 100-130 PSI
				unit = "psi"
			case "vibration":
				baseValue = 10 + rand.Float64()*20 // 10-30 Hz
				unit = "hz"
			case "humidity":
				baseValue = 40 + rand.Float64()*30 // 40-70%
				unit = "percent"
			}

			// Apply degradation
			value := baseValue * degradationFactors[sensorID]

			// Add noise
			value += (rand.Float64() - 0.5) * 2

			// Determine quality
			if rand.Float64() < 0.05 {
				quality = "poor"
			} else if rand.Float64() < 0.15 {
				quality = "fair"
			} else {
				quality = "good"
			}

			reading := iot.SensorReading{
				SensorID:   sensorID,
				DeviceID:   deviceID,
				Location:   locations[rand.Intn(len(locations))],
				SensorType: sensorType,
				Value:      math.Round(value*100) / 100, // Round to 2 decimals
				Unit:       unit,
				Timestamp:  time.Now(),
				Quality:    quality,
				Metadata: map[string]interface{}{
					"firmware_version": "1.2.3",
					"battery_level":    rand.Intn(100),
				},
			}

			readingJSON, _ := json.Marshal(reading)
			log.Printf("Generated reading: %s", readingJSON)
		}

		time.Sleep(1 * time.Second)
	}
}

func calculateMaintenanceWindow(prediction iot.MaintenancePrediction) string {
	now := time.Now()
	if prediction.EstimatedDaysToFailure < 7 {
		return fmt.Sprintf("%s - %s (urgent)",
			now.Add(24*time.Hour).Format("2006-01-02"),
			now.Add(48*time.Hour).Format("2006-01-02"))
	} else if prediction.EstimatedDaysToFailure < 30 {
		return fmt.Sprintf("%s - %s (scheduled)",
			now.Add(7*24*time.Hour).Format("2006-01-02"),
			now.Add(14*24*time.Hour).Format("2006-01-02"))
	}
	return fmt.Sprintf("%s - %s (planned)",
		now.Add(30*24*time.Hour).Format("2006-01-02"),
		now.Add(60*24*time.Hour).Format("2006-01-02"))
}

func estimateMaintenanceCost(prediction iot.MaintenancePrediction) float64 {
	baseCost := 500.0

	switch prediction.Priority {
	case "high":
		return baseCost * 2.0 // Urgent maintenance costs more
	case "medium":
		return baseCost * 1.5
	default:
		return baseCost
	}
}

func getRecommendedActions(prediction iot.MaintenancePrediction) []string {
	actions := []string{"Inspect sensor", "Check connections"}

	if prediction.HealthScore < 50 {
		actions = append(actions, "Replace sensor immediately")
	} else if prediction.HealthScore < 75 {
		actions = append(actions, "Schedule sensor replacement")
	}

	if prediction.Priority == "high" {
		actions = append(actions, "Notify operations team", "Prepare replacement parts")
	}

	return actions
}

func createDefaultIoTConfig() *config.Config {
	return &config.Config{
		Application: config.ApplicationConfig{
			Name:        "iot-monitoring",
			Environment: "development",
		},
		Engine: config.EngineConfig{
			BufferSize:            20000,
			MaxConcurrency:        150,
			CheckpointInterval:    30 * time.Second,
			WatermarkInterval:     5 * time.Second,
			EnableInstrumentation: true,
		},
		Sources: config.SourcesConfig{
			Kafka: config.KafkaSourceConfig{
				Brokers: []string{"localhost:9092"},
			},
			WebSocket: config.WebSocketSourceConfig{
				ListenAddr: ":8082",
				Path:       "/sensors",
			},
		},
		Sinks: config.SinksConfig{
			Kafka: config.KafkaSinkConfig{
				Brokers: []string{"localhost:9092"},
			},
			TimescaleDB: config.TimescaleSinkConfig{
				ConnectionString: "postgresql://user:password@localhost:5432/gress",
			},
		},
		Metrics: config.MetricsConfig{
			Enabled:   true,
			Port:      9093,
			Namespace: "iot_monitoring",
		},
	}
}

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/therealutkarshpriyadarshi/gress/pkg/cep"
	"github.com/therealutkarshpriyadarshi/gress/pkg/config"
	"github.com/therealutkarshpriyadarshi/gress/pkg/fraud"
	"github.com/therealutkarshpriyadarshi/gress/pkg/ingestion"
	"github.com/therealutkarshpriyadarshi/gress/pkg/sink"
	"github.com/therealutkarshpriyadarshi/gress/pkg/stream"
)

/*
Fraud Detection System Example

This example demonstrates a comprehensive fraud detection system with:
1. Complex Event Processing (CEP) for pattern detection
2. Anomaly detection using statistical methods
3. Real-time risk scoring
4. Multi-factor fraud analysis

Architecture:
- Kafka/HTTP ingestion for transaction events
- Multiple detection pipelines:
  * CEP pattern matching (e.g., rapid succession, location hopping)
  * Statistical anomaly detection
  * Risk scoring engine
  * Risk aggregation across time windows
- Alerts sent to Kafka for action/review

Data Flow:
Transaction Events -> Fraud Detector -> Risk Scorer -> Alert System

Sample Event:
{
  "event_id": "txn-123",
  "user_id": "user-456",
  "type": "transaction",
  "amount": 500.00,
  "timestamp": "2024-01-15T10:30:00Z",
  "ip_address": "192.168.1.1",
  "location": "New York, US",
  "device_id": "device-789"
}
*/

func main() {
	log.Println("Starting Fraud Detection System...")

	// Load configuration
	cfg, err := config.LoadConfig("configs/fraud-detection.yaml")
	if err != nil {
		cfg = createDefaultFraudConfig()
	}

	ctx := context.Background()

	// Create stream engine
	engine := stream.NewEngine(cfg)

	// 1. Real-time Fraud Detection Pipeline
	fraudPipeline := createFraudDetectionPipeline(cfg)
	if err := engine.AddPipeline("fraud-detection", fraudPipeline); err != nil {
		log.Fatalf("Failed to add fraud detection pipeline: %v", err)
	}

	// 2. CEP Pattern Detection Pipeline
	cepPipeline := createCEPPipeline(cfg)
	if err := engine.AddPipeline("cep-patterns", cepPipeline); err != nil {
		log.Fatalf("Failed to add CEP pipeline: %v", err)
	}

	// 3. Risk Aggregation Pipeline
	riskPipeline := createRiskAggregationPipeline(cfg)
	if err := engine.AddPipeline("risk-aggregation", riskPipeline); err != nil {
		log.Fatalf("Failed to add risk aggregation pipeline: %v", err)
	}

	// Start the engine
	if err := engine.Start(ctx); err != nil {
		log.Fatalf("Failed to start engine: %v", err)
	}

	// Generate sample transaction data
	go generateSampleTransactions(cfg)

	log.Println("Fraud Detection System is running...")
	log.Println("HTTP endpoint: http://localhost:8081/transactions")
	log.Println("Metrics endpoint: http://localhost:9092/metrics")
	log.Println("Press Ctrl+C to stop")

	// Wait for termination signal
	select {}
}

func createFraudDetectionPipeline(cfg *config.Config) *stream.Pipeline {
	// HTTP source for transaction events
	httpSource := ingestion.NewHTTPSource(ingestion.HTTPSourceConfig{
		ListenAddr: ":8081",
		Path:       "/transactions",
	})

	// Fraud detection operator with configurable thresholds
	fraudDetector := fraud.FraudDetectionOperator(
		10,                 // velocity threshold: 10 transactions
		5*time.Minute,      // velocity window: 5 minutes
		10000.0,            // amount threshold: $10,000
	)

	// Filter high-risk transactions
	highRiskFilter := stream.FilterOperator(func(event *stream.Event) bool {
		var score fraud.FraudScore
		if err := json.Unmarshal(event.Data, &score); err != nil {
			return false
		}
		return score.RiskLevel == "high" || score.RiskLevel == "critical"
	})

	// Kafka sink for fraud alerts
	alertsSink := sink.NewKafkaSink(sink.KafkaSinkConfig{
		Brokers: cfg.Sinks.Kafka.Brokers,
		Topic:   "fraud-alerts",
	})

	return &stream.Pipeline{
		Source: httpSource,
		Operators: []stream.Operator{
			fraudDetector,
			highRiskFilter,
		},
		Sink: alertsSink,
	}
}

func createCEPPipeline(cfg *config.Config) *stream.Pipeline {
	// Kafka source for all transaction events
	kafkaSource := ingestion.NewKafkaSource(ingestion.KafkaSourceConfig{
		Brokers:     cfg.Sources.Kafka.Brokers,
		Topics:      []string{"transactions"},
		GroupID:     "cep-pattern-detection",
		StartOffset: "latest",
	})

	// Define CEP patterns to detect
	patterns := []cep.Pattern{
		// Pattern 1: Rapid succession of transactions
		cep.SequencePattern(
			"rapid-succession",
			"Rapid Transaction Succession",
			[]string{"transaction", "transaction", "transaction"},
			1*time.Minute,
		),

		// Pattern 2: Multiple failed login attempts followed by success
		cep.SequencePattern(
			"brute-force-login",
			"Potential Brute Force Login",
			[]string{"login_failed", "login_failed", "login_failed", "login_success"},
			5*time.Minute,
		),

		// Pattern 3: Account changes followed by large transaction
		cep.SequencePattern(
			"account-takeover",
			"Potential Account Takeover",
			[]string{"account_change", "transaction"},
			30*time.Minute,
		),
	}

	// Create pattern detectors
	operators := make([]stream.Operator, 0)
	for _, pattern := range patterns {
		detector := cep.PatternDetector(pattern, 1000)
		operators = append(operators, detector)
	}

	// Kafka sink for pattern matches
	patternSink := sink.NewKafkaSink(sink.KafkaSinkConfig{
		Brokers: cfg.Sinks.Kafka.Brokers,
		Topic:   "fraud-patterns-detected",
	})

	return &stream.Pipeline{
		Source:    kafkaSource,
		Operators: operators,
		Sink:      patternSink,
	}
}

func createRiskAggregationPipeline(cfg *config.Config) *stream.Pipeline {
	// Kafka source for fraud scores
	kafkaSource := ingestion.NewKafkaSource(ingestion.KafkaSourceConfig{
		Brokers:     cfg.Sources.Kafka.Brokers,
		Topics:      []string{"fraud-alerts"},
		GroupID:     "risk-aggregation",
		StartOffset: "latest",
	})

	// Risk aggregator (last 100 transactions per user)
	riskAggregator := fraud.RiskAggregationOperator(100)

	// TimescaleDB sink for historical analysis
	timescaleSink := sink.NewTimescaleSink(sink.TimescaleSinkConfig{
		ConnectionString: cfg.Sinks.TimescaleDB.ConnectionString,
		Table:            "user_risk_scores",
		BatchSize:        50,
		FlushInterval:    5 * time.Second,
	})

	return &stream.Pipeline{
		Source: kafkaSource,
		Operators: []stream.Operator{
			riskAggregator,
		},
		Sink: timescaleSink,
	}
}

func generateSampleTransactions(cfg *config.Config) {
	time.Sleep(2 * time.Second)

	users := []string{"user-1", "user-2", "user-3", "user-4", "user-5"}
	locations := []string{"New York, US", "London, UK", "Tokyo, JP", "Berlin, DE", "Paris, FR"}
	devices := []string{"device-1", "device-2", "device-3", "device-4", "device-5"}

	log.Println("Generating sample transaction events...")

	transactionID := 1

	for {
		// Generate 5-15 transactions per second
		numTransactions := rand.Intn(10) + 5

		for i := 0; i < numTransactions; i++ {
			userID := users[rand.Intn(len(users))]

			// Occasionally generate suspicious patterns
			isSuspicious := rand.Float64() < 0.1 // 10% suspicious

			var amount float64
			var location string
			var deviceID string

			if isSuspicious {
				// Suspicious: high amount, unusual location, new device
				amount = rand.Float64()*9000 + 1000 // $1,000-$10,000
				location = locations[rand.Intn(len(locations))]
				deviceID = fmt.Sprintf("device-new-%d", rand.Intn(100))
			} else {
				// Normal: reasonable amount, typical location
				amount = rand.Float64()*200 + 10 // $10-$200
				location = locations[0]          // Consistent location
				deviceID = devices[rand.Intn(3)] // Known devices
			}

			event := fraud.FraudEvent{
				EventID:   fmt.Sprintf("txn-%d", transactionID),
				UserID:    userID,
				Type:      "transaction",
				Amount:    amount,
				Timestamp: time.Now(),
				IPAddress: fmt.Sprintf("192.168.%d.%d", rand.Intn(255), rand.Intn(255)),
				Location:  location,
				DeviceID:  deviceID,
				Metadata: map[string]interface{}{
					"merchant":     fmt.Sprintf("merchant-%d", rand.Intn(50)),
					"payment_type": []string{"credit_card", "debit_card", "paypal"}[rand.Intn(3)],
					"currency":     "USD",
				},
			}

			transactionID++

			eventJSON, _ := json.Marshal(event)
			log.Printf("Generated transaction: %s (suspicious: %v)", eventJSON, isSuspicious)
		}

		time.Sleep(1 * time.Second)
	}
}

func createDefaultFraudConfig() *config.Config {
	return &config.Config{
		Application: config.ApplicationConfig{
			Name:        "fraud-detection",
			Environment: "development",
		},
		Engine: config.EngineConfig{
			BufferSize:            10000,
			MaxConcurrency:        100,
			CheckpointInterval:    30 * time.Second,
			WatermarkInterval:     5 * time.Second,
			EnableInstrumentation: true,
		},
		Sources: config.SourcesConfig{
			Kafka: config.KafkaSourceConfig{
				Brokers: []string{"localhost:9092"},
			},
			HTTP: config.HTTPSourceConfig{
				ListenAddr: ":8081",
				Path:       "/transactions",
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
			Port:      9092,
			Namespace: "fraud_detection",
		},
	}
}

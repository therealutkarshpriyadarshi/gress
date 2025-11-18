package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/therealutkarshpriyadarshi/gress/pkg/analytics"
	"github.com/therealutkarshpriyadarshi/gress/pkg/config"
	"github.com/therealutkarshpriyadarshi/gress/pkg/ingestion"
	"github.com/therealutkarshpriyadarshi/gress/pkg/sink"
	"github.com/therealutkarshpriyadarshi/gress/pkg/stream"
	"github.com/therealutkarshpriyadarshi/gress/pkg/window"
)

/*
Real-Time Analytics Dashboard Example

This example demonstrates a comprehensive analytics system that tracks:
1. User behavior patterns (page views, clicks, conversions)
2. Live KPI calculations (active users, conversion rate, engagement)
3. Session analysis with windowing

Architecture:
- HTTP ingestion endpoint for user events
- Multiple processing pipelines:
  * Behavior tracking and enrichment
  * Session aggregation with session windows
  * Live KPI calculation with tumbling windows
  * Engagement score computation
- Output to both Kafka and TimescaleDB for dashboarding

Data Flow:
User Events -> Behavior Tracker -> Session Aggregator -> KPI Calculator -> Sink

Sample Event:
{
  "user_id": "user-123",
  "session_id": "session-456",
  "action": "page_view",
  "page": "/products",
  "timestamp": "2024-01-15T10:30:00Z",
  "duration": 5000,
  "ip_address": "192.168.1.1",
  "country": "US",
  "device": "mobile"
}
*/

func main() {
	log.Println("Starting Real-Time Analytics Dashboard...")

	// Load configuration
	cfg, err := config.LoadConfig("configs/analytics-dashboard.yaml")
	if err != nil {
		// Use defaults if config not found
		cfg = createDefaultAnalyticsConfig()
	}

	ctx := context.Background()

	// Create stream engine
	engine := stream.NewEngine(cfg)

	// 1. User Behavior Tracking Pipeline
	behaviorPipeline := createBehaviorTrackingPipeline(cfg)
	if err := engine.AddPipeline("behavior-tracking", behaviorPipeline); err != nil {
		log.Fatalf("Failed to add behavior tracking pipeline: %v", err)
	}

	// 2. Session Analysis Pipeline
	sessionPipeline := createSessionAnalysisPipeline(cfg)
	if err := engine.AddPipeline("session-analysis", sessionPipeline); err != nil {
		log.Fatalf("Failed to add session analysis pipeline: %v", err)
	}

	// 3. Live KPI Calculation Pipeline
	kpiPipeline := createKPICalculationPipeline(cfg)
	if err := engine.AddPipeline("kpi-calculation", kpiPipeline); err != nil {
		log.Fatalf("Failed to add KPI calculation pipeline: %v", err)
	}

	// 4. Page View Analytics Pipeline
	pageViewPipeline := createPageViewAnalyticsPipeline(cfg)
	if err := engine.AddPipeline("page-view-analytics", pageViewPipeline); err != nil {
		log.Fatalf("Failed to add page view analytics pipeline: %v", err)
	}

	// Start the engine
	if err := engine.Start(ctx); err != nil {
		log.Fatalf("Failed to start engine: %v", err)
	}

	// Generate sample data for demonstration
	go generateSampleUserEvents(cfg)

	log.Println("Analytics Dashboard is running...")
	log.Println("HTTP endpoint: http://localhost:8080/ingest")
	log.Println("Metrics endpoint: http://localhost:9091/metrics")
	log.Println("Press Ctrl+C to stop")

	// Wait for termination signal
	select {}
}

func createBehaviorTrackingPipeline(cfg *config.Config) *stream.Pipeline {
	// HTTP source for user events
	httpSource := ingestion.NewHTTPSource(ingestion.HTTPSourceConfig{
		ListenAddr: ":8080",
		Path:       "/ingest",
	})

	// Behavior tracker operator
	behaviorTracker := analytics.BehaviorTracker()

	// Engagement score calculator
	engagementCalculator := analytics.EngagementScoreCalculator()

	// Kafka sink for processed events
	kafkaSink := sink.NewKafkaSink(sink.KafkaSinkConfig{
		Brokers: cfg.Sinks.Kafka.Brokers,
		Topic:   "user-behavior-enriched",
	})

	return &stream.Pipeline{
		Source: httpSource,
		Operators: []stream.Operator{
			behaviorTracker,
			engagementCalculator,
		},
		Sink: kafkaSink,
	}
}

func createSessionAnalysisPipeline(cfg *config.Config) *stream.Pipeline {
	// Kafka source for enriched events
	kafkaSource := ingestion.NewKafkaSource(ingestion.KafkaSourceConfig{
		Brokers:     cfg.Sources.Kafka.Brokers,
		Topics:      []string{"user-behavior-enriched"},
		GroupID:     "session-analysis-group",
		StartOffset: "latest",
	})

	// Session window (5 minute gaps)
	sessionWindow := window.NewSessionWindow(5 * time.Minute)

	// Session aggregator
	sessionAggregator := analytics.SessionAggregator()

	// TimescaleDB sink for session metrics
	timescaleSink := sink.NewTimescaleSink(sink.TimescaleSinkConfig{
		ConnectionString: cfg.Sinks.TimescaleDB.ConnectionString,
		Table:            "session_metrics",
		BatchSize:        100,
		FlushInterval:    5 * time.Second,
	})

	return &stream.Pipeline{
		Source: kafkaSource,
		Operators: []stream.Operator{
			sessionWindow,
			sessionAggregator,
		},
		Sink: timescaleSink,
	}
}

func createKPICalculationPipeline(cfg *config.Config) *stream.Pipeline {
	// Kafka source for user events
	kafkaSource := ingestion.NewKafkaSource(ingestion.KafkaSourceConfig{
		Brokers:     cfg.Sources.Kafka.Brokers,
		Topics:      []string{"user-behavior-enriched"},
		GroupID:     "kpi-calculation-group",
		StartOffset: "latest",
	})

	// Tumbling window (1 minute)
	tumblingWindow := window.NewTumblingWindow(1 * time.Minute)

	// Live KPI calculator
	kpiCalculator := analytics.LiveKPICalculator(1 * time.Minute)

	// Multiple sinks
	kafkaSink := sink.NewKafkaSink(sink.KafkaSinkConfig{
		Brokers: cfg.Sinks.Kafka.Brokers,
		Topic:   "live-kpis",
	})

	return &stream.Pipeline{
		Source: kafkaSource,
		Operators: []stream.Operator{
			tumblingWindow,
			kpiCalculator,
		},
		Sink: kafkaSink,
	}
}

func createPageViewAnalyticsPipeline(cfg *config.Config) *stream.Pipeline {
	// Kafka source
	kafkaSource := ingestion.NewKafkaSource(ingestion.KafkaSourceConfig{
		Brokers:     cfg.Sources.Kafka.Brokers,
		Topics:      []string{"user-behavior-enriched"},
		GroupID:     "page-view-analytics-group",
		StartOffset: "latest",
	})

	// Page view tracker
	pageViewTracker := analytics.PageViewTracker()

	// TimescaleDB sink
	timescaleSink := sink.NewTimescaleSink(sink.TimescaleSinkConfig{
		ConnectionString: cfg.Sinks.TimescaleDB.ConnectionString,
		Table:            "page_view_stats",
		BatchSize:        50,
		FlushInterval:    5 * time.Second,
	})

	return &stream.Pipeline{
		Source: kafkaSource,
		Operators: []stream.Operator{
			pageViewTracker,
		},
		Sink: timescaleSink,
	}
}

func generateSampleUserEvents(cfg *config.Config) {
	time.Sleep(2 * time.Second) // Wait for services to start

	users := []string{"user-1", "user-2", "user-3", "user-4", "user-5"}
	pages := []string{"/home", "/products", "/cart", "/checkout", "/account"}
	actions := []string{"page_view", "click", "scroll", "purchase", "add_to_cart"}
	countries := []string{"US", "UK", "DE", "FR", "JP"}
	devices := []string{"mobile", "desktop", "tablet"}

	log.Println("Generating sample user events...")

	for {
		// Generate 10-50 events per second
		numEvents := rand.Intn(40) + 10

		for i := 0; i < numEvents; i++ {
			event := analytics.UserBehavior{
				UserID:    users[rand.Intn(len(users))],
				SessionID: fmt.Sprintf("session-%d", rand.Intn(20)),
				Action:    actions[rand.Intn(len(actions))],
				Page:      pages[rand.Intn(len(pages))],
				Timestamp: time.Now(),
				Duration:  int64(rand.Intn(30000) + 1000), // 1-30 seconds
				Metadata: map[string]interface{}{
					"browser": "Chrome",
					"version": "120.0",
				},
				IPAddress: fmt.Sprintf("192.168.1.%d", rand.Intn(255)),
				UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64)",
				Country:   countries[rand.Intn(len(countries))],
				Device:    devices[rand.Intn(len(devices))],
			}

			// Add purchase amount for purchase events
			if event.Action == "purchase" {
				event.Metadata["amount"] = rand.Float64() * 100.0
			}

			// Send to HTTP endpoint
			eventJSON, _ := json.Marshal(event)
			// In a real scenario, this would be an HTTP POST
			log.Printf("Generated event: %s", eventJSON)
		}

		time.Sleep(1 * time.Second)
	}
}

func createDefaultAnalyticsConfig() *config.Config {
	return &config.Config{
		Application: config.ApplicationConfig{
			Name:        "analytics-dashboard",
			Environment: "development",
		},
		Engine: config.EngineConfig{
			BufferSize:          10000,
			MaxConcurrency:      100,
			CheckpointInterval:  30 * time.Second,
			WatermarkInterval:   5 * time.Second,
			EnableInstrumentation: true,
		},
		Sources: config.SourcesConfig{
			Kafka: config.KafkaSourceConfig{
				Brokers: []string{"localhost:9092"},
			},
			HTTP: config.HTTPSourceConfig{
				ListenAddr: ":8080",
				Path:       "/ingest",
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
			Port:      9091,
			Namespace: "analytics_dashboard",
		},
	}
}

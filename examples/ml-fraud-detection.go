package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/therealutkarshpriyadarshi/gress/pkg/config"
	"github.com/therealutkarshpriyadarshi/gress/pkg/ml"
	"github.com/therealutkarshpriyadarshi/gress/pkg/state"
	"github.com/therealutkarshpriyadarshi/gress/pkg/stream"
)

// FraudDetectionPipeline demonstrates a real-time fraud detection ML pipeline
//
// Pipeline Flow:
// 1. Kafka Source (transaction events)
// 2. Feature Extraction (transaction features)
// 3. ML Inference (fraud scoring)
// 4. Filter (high-risk transactions)
// 5. Alert Sink (fraud alerts)

func main() {
	// Initialize configuration
	cfg := config.DefaultConfig()
	cfg.ML.Enabled = true

	// Create state backend for feature store
	stateBackend := state.NewMemoryBackend()

	// Create Prometheus registry
	promRegistry := prometheus.NewRegistry()

	// Create ML metrics
	mlMetrics := ml.NewMLMetrics("fraud_detection")
	if err := mlMetrics.Register(promRegistry); err != nil {
		log.Fatalf("Failed to register ML metrics: %v", err)
	}

	// Create model registry
	modelRegistry := ml.NewModelRegistry()

	// Load fraud detection model (ONNX format)
	// Note: In production, you would load an actual ONNX model file
	ctx := context.Background()

	// For this example, we'll use a mock model
	// In production: model, err := ml.LoadModel(ctx, ml.ModelTypeONNX, "/models/fraud-detector-v2.onnx", metadata)
	fraudModel := createMockFraudModel()

	if err := modelRegistry.Register(fraudModel); err != nil {
		log.Fatalf("Failed to register fraud model: %v", err)
	}

	log.Println("✓ Fraud detection model loaded successfully")

	// Create feature extraction operator
	featureConfig := ml.FeatureExtractionConfig{
		Features: []ml.FeatureSpec{
			{
				Name:   "amount_normalized",
				Type:   ml.FeatureTypeNormalize,
				Source: "amount",
				Output: "amount_normalized",
				Parameters: map[string]interface{}{
					"min": 0.0,
					"max": 10000.0,
				},
			},
			{
				Name:   "amount_log",
				Type:   ml.FeatureTypeLog,
				Source: "amount",
				Output: "amount_log",
			},
			{
				Name:   "merchant_category",
				Type:   ml.FeatureTypeCategorical,
				Source: "merchant_category",
				Output: "merchant_category",
			},
			{
				Name:   "temporal_features",
				Type:   ml.FeatureTypeTemporal,
				Source: "",
				Output: "temporal_features",
			},
			{
				Name:   "is_high_risk_country",
				Type:   ml.FeatureTypeCategorical,
				Source: "country",
				Output: "is_high_risk_country",
			},
		},
		DropOriginal: false,
	}

	featureExtractor, err := ml.NewFeatureExtractionOperator(featureConfig, stateBackend)
	if err != nil {
		log.Fatalf("Failed to create feature extractor: %v", err)
	}

	log.Println("✓ Feature extraction operator created")

	// Create ML inference operator with batching
	inferenceMetrics := ml.NewInferenceMetrics("fraud_detection")
	if err := inferenceMetrics.Register(promRegistry); err != nil {
		log.Fatalf("Failed to register inference metrics: %v", err)
	}

	inferenceConfig := ml.InferenceConfig{
		ModelName:      "fraud-detector",
		ModelVersion:   "v2",
		BatchSize:      100,
		BatchTimeout:   50 * time.Millisecond,
		MaxConcurrency: 10,
		FeatureFields: []string{
			"amount_normalized",
			"amount_log",
			"merchant_category",
			"temporal_features",
			"is_high_risk_country",
		},
		OutputField: "fraud_score",
	}

	inferenceOperator, err := ml.NewInferenceOperator(inferenceConfig, modelRegistry, inferenceMetrics)
	if err != nil {
		log.Fatalf("Failed to create inference operator: %v", err)
	}

	log.Println("✓ ML inference operator created with batching (size=100, timeout=50ms)")

	// Example: Process some transaction events
	transactions := []map[string]interface{}{
		{
			"transaction_id":    "tx-001",
			"amount":            500.0,
			"merchant_category": "electronics",
			"country":           "US",
			"user_id":           "user-123",
		},
		{
			"transaction_id":    "tx-002",
			"amount":            9500.0,
			"merchant_category": "jewelry",
			"country":           "NG", // High-risk country
			"user_id":           "user-456",
		},
		{
			"transaction_id":    "tx-003",
			"amount":            50.0,
			"merchant_category": "groceries",
			"country":           "US",
			"user_id":           "user-789",
		},
	}

	log.Println("\n📊 Processing transactions through ML pipeline...")

	for _, txData := range transactions {
		// Create stream event
		event := &stream.Event{
			Key:       txData["transaction_id"],
			Value:     txData,
			EventTime: time.Now(),
		}

		// Create processing context
		ctx := &stream.ProcessingContext{
			Ctx: context.Background(),
		}

		// Step 1: Feature extraction
		events, err := featureExtractor.Process(ctx, event)
		if err != nil {
			log.Printf("Feature extraction failed for %s: %v", txData["transaction_id"], err)
			continue
		}
		extractedEvent := events[0]

		// Step 2: ML inference
		scoredEvents, err := inferenceOperator.Process(ctx, extractedEvent)
		if err != nil {
			log.Printf("Inference failed for %s: %v", txData["transaction_id"], err)
			continue
		}
		scoredEvent := scoredEvents[0]

		// Step 3: Check fraud score
		eventValue := scoredEvent.Value.(map[string]interface{})
		fraudScore := eventValue["fraud_score"]
		predictionLatency := eventValue["prediction_latency_ms"]

		log.Printf("\n  Transaction: %s", txData["transaction_id"])
		log.Printf("  Amount: $%.2f", txData["amount"])
		log.Printf("  Merchant: %s", txData["merchant_category"])
		log.Printf("  Country: %s", txData["country"])
		log.Printf("  Fraud Score: %v", fraudScore)
		log.Printf("  Prediction Latency: %vms", predictionLatency)

		// Step 4: Alert on high fraud score
		// (In production, this would send to an alerting system)
		if score, ok := fraudScore.(float64); ok && score > 0.7 {
			log.Printf("  ⚠️  HIGH FRAUD RISK - Sending alert!")
		} else {
			log.Printf("  ✓ Transaction approved")
		}
	}

	// Cleanup
	log.Println("\n🔄 Cleaning up...")
	inferenceOperator.Close()
	featureExtractor.Close()
	modelRegistry.Close()

	log.Println("✓ Fraud detection pipeline completed successfully")
}

// createMockFraudModel creates a mock fraud detection model for demonstration
func createMockFraudModel() ml.Model {
	return &mockFraudModel{
		metadata: ml.ModelMetadata{
			Name:        "fraud-detector",
			Version:     "v2",
			Type:        ml.ModelTypeONNX,
			InputNames:  []string{"features"},
			OutputNames: []string{"fraud_score"},
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
			Description: "Real-time fraud detection model",
			Tags: map[string]string{
				"task":        "classification",
				"domain":      "fraud_detection",
				"environment": "production",
			},
		},
	}
}

type mockFraudModel struct {
	metadata ml.ModelMetadata
}

func (m *mockFraudModel) Predict(ctx context.Context, features map[string]interface{}) (*ml.PredictionResponse, error) {
	start := time.Now()

	// Simple rule-based scoring for demonstration
	// In production, this would use the actual ONNX model
	score := 0.0

	if amount, ok := features["amount_normalized"].(float64); ok {
		if amount > 0.8 { // High transaction amount
			score += 0.3
		}
	}

	if country, ok := features["is_high_risk_country"].(string); ok {
		if country == "NG" || country == "RU" {
			score += 0.4
		}
	}

	if category, ok := features["merchant_category"].(string); ok {
		if category == "jewelry" || category == "electronics" {
			score += 0.2
		}
	}

	latency := time.Since(start)

	return &ml.PredictionResponse{
		Prediction: score,
		Scores: map[string]float64{
			"fraud_probability": score,
		},
		Latency:   latency,
		ModelInfo: m.metadata,
	}, nil
}

func (m *mockFraudModel) PredictBatch(ctx context.Context, requests []*ml.PredictionRequest) ([]*ml.PredictionResponse, error) {
	responses := make([]*ml.PredictionResponse, len(requests))
	for i, req := range requests {
		resp, err := m.Predict(ctx, req.Features)
		if err != nil {
			return nil, err
		}
		responses[i] = resp
	}
	return responses, nil
}

func (m *mockFraudModel) GetMetadata() ml.ModelMetadata {
	return m.metadata
}

func (m *mockFraudModel) Warmup(ctx context.Context) error {
	return nil
}

func (m *mockFraudModel) Close() error {
	return nil
}

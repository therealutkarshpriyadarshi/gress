package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/therealutkarshpriyadarshi/gress/pkg/ml"
	"github.com/therealutkarshpriyadarshi/gress/pkg/stream"
)

// ABTestingPipeline demonstrates A/B testing with multiple model versions
//
// Scenario: Testing two recommendation model versions
// - Variant A: Current production model (v1.0)
// - Variant B: New experimental model (v2.0)
//
// Traffic split: 80% to v1.0, 20% to v2.0
// Shadow mode enabled to compare predictions

func main() {
	log.Println("🧪 Starting A/B Testing Pipeline Example")

	// Create model registry
	modelRegistry := ml.NewModelRegistry()

	// Load model v1.0 (current production)
	modelV1 := createMockRecommendationModel("recommendation-model", "v1.0", "collaborative-filtering")
	if err := modelRegistry.Register(modelV1); err != nil {
		log.Fatalf("Failed to register model v1.0: %v", err)
	}
	log.Println("✓ Registered model v1.0 (collaborative filtering)")

	// Load model v2.0 (new experimental)
	modelV2 := createMockRecommendationModel("recommendation-model", "v2.0", "deep-learning")
	if err := modelRegistry.Register(modelV2); err != nil {
		log.Fatalf("Failed to register model v2.0: %v", err)
	}
	log.Println("✓ Registered model v2.0 (deep learning)")

	// Create Prometheus registry
	promRegistry := prometheus.NewRegistry()

	// Create A/B test metrics
	abTestMetrics := ml.NewABTestMetrics("recommendation")
	if err := abTestMetrics.Register(promRegistry); err != nil {
		log.Fatalf("Failed to register A/B test metrics: %v", err)
	}

	// Configure A/B test
	abTestConfig := ml.ABTestConfig{
		Name:     "recommendation-model-test",
		Strategy: ml.ABTestStrategyHash, // Use user_id for consistent assignment
		Variants: []ml.ModelVariant{
			{
				Name:         "control",
				ModelName:    "recommendation-model",
				ModelVersion: "v1.0",
				Weight:       80.0, // 80% of traffic
				Enabled:      true,
			},
			{
				Name:         "treatment",
				ModelName:    "recommendation-model",
				ModelVersion: "v2.0",
				Weight:       20.0, // 20% of traffic
				Enabled:      true,
			},
		},
		SplitKey:          "user_id",
		DefaultVariant:    "control",
		EnableShadowMode:  true, // Run both models and compare
		CollectComparison: true, // Track prediction differences
	}

	// Create A/B test operator
	abTestOperator, err := ml.NewABTestOperator(abTestConfig, modelRegistry, abTestMetrics)
	if err != nil {
		log.Fatalf("Failed to create A/B test operator: %v", err)
	}
	defer abTestOperator.Close()

	log.Println("✓ A/B test configured (80/20 split, shadow mode enabled)")

	// Simulate user events
	users := []map[string]interface{}{
		{
			"user_id":           "user-001",
			"browsing_history":  []string{"laptop", "mouse", "keyboard"},
			"purchase_history":  []string{"monitor"},
			"session_duration":  300,
		},
		{
			"user_id":           "user-002",
			"browsing_history":  []string{"book", "pen", "notebook"},
			"purchase_history":  []string{},
			"session_duration":  120,
		},
		{
			"user_id":           "user-003",
			"browsing_history":  []string{"smartphone", "case", "charger"},
			"purchase_history":  []string{"earphones"},
			"session_duration":  450,
		},
		{
			"user_id":           "user-004",
			"browsing_history":  []string{"shoes", "socks", "shirt"},
			"purchase_history":  []string{},
			"session_duration":  200,
		},
		{
			"user_id":           "user-005",
			"browsing_history":  []string{"camera", "lens", "tripod"},
			"purchase_history":  []string{"memory-card"},
			"session_duration":  600,
		},
	}

	log.Println("\n📊 Processing user events through A/B test pipeline...")

	// Track variant selections
	variantCounts := make(map[string]int)

	for _, userData := range users {
		// Create stream event
		event := &stream.Event{
			Key:       userData["user_id"],
			Value:     userData,
			EventTime: time.Now(),
		}

		// Create processing context
		ctx := &stream.ProcessingContext{
			Ctx: context.Background(),
		}

		// Process through A/B test
		processedEvents, err := abTestOperator.Process(ctx, event)
		if err != nil {
			log.Printf("A/B test failed for user %s: %v", userData["user_id"], err)
			continue
		}

		processedEvent := processedEvents[0]
		eventValue := processedEvent.Value.(map[string]interface{})

		// Extract results
		userID := userData["user_id"]
		variant := eventValue["ab_variant"].(string)
		prediction := eventValue["prediction"]
		latency := eventValue["prediction_latency_ms"]

		variantCounts[variant]++

		log.Printf("\n  User: %s", userID)
		log.Printf("  Variant: %s", variant)
		log.Printf("  Recommendations: %v", prediction)
		log.Printf("  Latency: %vms", latency)

		if variant == "treatment" {
			log.Printf("  🆕 User experiencing new model!")
		}
	}

	// Print A/B test statistics
	log.Println("\n📈 A/B Test Statistics:")
	log.Printf("  Total requests: %d", len(users))
	for variant, count := range variantCounts {
		percentage := float64(count) / float64(len(users)) * 100
		log.Printf("  %s: %d requests (%.1f%%)", variant, count, percentage)
	}

	// Get detailed variant stats
	stats := abTestOperator.GetVariantStats()
	log.Println("\n🔍 Variant Details:")
	for name, info := range stats {
		infoMap := info.(map[string]interface{})
		log.Printf("  %s:", name)
		log.Printf("    Model: %s", infoMap["model"])
		log.Printf("    Version: %s", infoMap["version"])
		log.Printf("    Weight: %.1f%%", infoMap["weight"])
		log.Printf("    Enabled: %v", infoMap["enabled"])
	}

	log.Println("\n✓ A/B testing pipeline completed successfully")
	log.Println("\n💡 In production, metrics would be sent to Prometheus for analysis:")
	log.Println("   - Variant selection rates")
	log.Println("   - Prediction latency by variant")
	log.Println("   - Shadow mode comparison results")
	log.Println("   - Error rates by variant")
}

// createMockRecommendationModel creates a mock recommendation model
func createMockRecommendationModel(name, version, algorithm string) ml.Model {
	return &mockRecommendationModel{
		metadata: ml.ModelMetadata{
			Name:        name,
			Version:     version,
			Type:        ml.ModelTypeCustom,
			InputNames:  []string{"user_features"},
			OutputNames: []string{"recommendations"},
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
			Description: fmt.Sprintf("Recommendation model using %s", algorithm),
			Tags: map[string]string{
				"algorithm": algorithm,
				"task":      "recommendation",
			},
		},
		algorithm: algorithm,
	}
}

type mockRecommendationModel struct {
	metadata  ml.ModelMetadata
	algorithm string
}

func (m *mockRecommendationModel) Predict(ctx context.Context, features map[string]interface{}) (*ml.PredictionResponse, error) {
	start := time.Now()

	// Generate different recommendations based on algorithm
	var recommendations []string

	if m.algorithm == "collaborative-filtering" {
		// v1.0 - simpler recommendations
		recommendations = []string{
			"product-A",
			"product-B",
			"product-C",
		}
	} else {
		// v2.0 - more sophisticated recommendations
		recommendations = []string{
			"product-X",
			"product-Y",
			"product-Z",
			"product-W",
			"product-V",
		}
	}

	latency := time.Since(start)

	return &ml.PredictionResponse{
		Prediction: recommendations,
		Metadata: map[string]interface{}{
			"algorithm":           m.algorithm,
			"recommendation_count": len(recommendations),
		},
		Latency:   latency,
		ModelInfo: m.metadata,
	}, nil
}

func (m *mockRecommendationModel) PredictBatch(ctx context.Context, requests []*ml.PredictionRequest) ([]*ml.PredictionResponse, error) {
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

func (m *mockRecommendationModel) GetMetadata() ml.ModelMetadata {
	return m.metadata
}

func (m *mockRecommendationModel) Warmup(ctx context.Context) error {
	return nil
}

func (m *mockRecommendationModel) Close() error {
	return nil
}

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/therealutkarshpriyadarshi/gress/pkg/cep"
)

// UserJourneyExample demonstrates CEP for tracking user journeys and conversion funnels
func main() {
	fmt.Println("=== User Journey Analysis CEP Example ===\n")

	// Create CEP engine
	engine := cep.NewCEPEngine(2000, 10*time.Minute)

	// Register user journey patterns
	registerUserJourneyPatterns(engine)

	// Set up match callback
	engine.OnMatch(func(match cep.MatchedPattern) {
		fmt.Printf("\n📊 Journey Pattern Detected: %s\n", match.PatternName)
		fmt.Printf("   Pattern ID: %s\n", match.PatternID)
		fmt.Printf("   Severity: %s\n", match.Severity)
		fmt.Printf("   Duration: %v\n", match.LastEventTime.Sub(match.FirstEventTime))
		fmt.Printf("   Events in Journey: %d\n", len(match.MatchedEvents))

		// Print journey metadata
		if userID, ok := match.MatchedEvents[0]["user_id"].(string); ok {
			fmt.Printf("   User: %s\n", userID)
		}

		// Print journey steps
		fmt.Println("\n   Journey Steps:")
		for i, event := range match.MatchedEvents {
			eventType := event["type"].(string)
			timestamp := event["timestamp"].(time.Time)
			fmt.Printf("   %d. %s at %s\n", i+1, eventType, timestamp.Format("15:04:05"))

			// Print relevant event details
			if product, ok := event["product_id"].(string); ok {
				fmt.Printf("      - Product: %s\n", product)
			}
			if category, ok := event["category"].(string); ok {
				fmt.Printf("      - Category: %s\n", category)
			}
			if amount, ok := event["amount"].(float64); ok {
				fmt.Printf("      - Amount: $%.2f\n", amount)
			}
		}
		fmt.Println()
	})

	// Simulate user journey scenarios
	fmt.Println("Simulating user journey scenarios...\n")

	// Scenario 1: Successful conversion journey
	fmt.Println("Scenario 1: Successful E-commerce Conversion")
	simulateSuccessfulConversion(engine)

	time.Sleep(2 * time.Second)

	// Scenario 2: Cart abandonment
	fmt.Println("\nScenario 2: Shopping Cart Abandonment")
	simulateCartAbandonment(engine)

	time.Sleep(2 * time.Second)

	// Scenario 3: Browse but no engagement
	fmt.Println("\nScenario 3: Window Shopping (High Browse, No Action)")
	simulateWindowShopping(engine)

	time.Sleep(2 * time.Second)

	// Scenario 4: Product comparison journey
	fmt.Println("\nScenario 4: Product Comparison Journey")
	simulateProductComparison(engine)

	time.Sleep(2 * time.Second)

	// Scenario 5: Re-engagement after abandonment
	fmt.Println("\nScenario 5: Re-engagement Success")
	simulateReEngagement(engine)

	time.Sleep(2 * time.Second)

	// Print statistics
	stats := engine.GetStatistics()
	fmt.Println("\n=== CEP Engine Statistics ===")
	for key, value := range stats {
		fmt.Printf("%s: %v\n", key, value)
	}
}

// registerUserJourneyPatterns registers patterns for user journey analysis
func registerUserJourneyPatterns(engine *cep.CEPEngine) {
	// Pattern 1: Successful conversion (browse -> cart -> checkout -> purchase)
	successfulConversion := cep.NewPattern("journey-001", "Successful Conversion").
		WithDescription("Complete purchase journey from browse to buy").
		Sequence(
			cep.Event("browse", "product_view").AtLeast(1),
			cep.Event("add_cart", "add_to_cart").AtLeast(1),
			cep.Event("checkout", "checkout_initiated"),
			cep.Event("purchase", "purchase_completed").Within(30*time.Minute),
		).
		WithCondition("browse", "add_cart", "user_id", "eq").
		WithCondition("add_cart", "checkout", "user_id", "eq").
		WithCondition("checkout", "purchase", "user_id", "eq").
		Within(2 * time.Hour).
		Build()

	engine.RegisterPattern(successfulConversion)

	// Pattern 2: Cart abandonment (add to cart but no checkout)
	cartAbandonment := cep.NewPattern("journey-002", "Shopping Cart Abandonment").
		WithDescription("User adds items to cart but doesn't complete checkout").
		Sequence(
			cep.Event("browse", "product_view").AtLeast(2),
			cep.Event("add_cart", "add_to_cart").AtLeast(1),
		).
		NotFollowedBy(
			cep.Event("checkout", "checkout_initiated"),
		).
		Within(1 * time.Hour).
		Build()

	engine.RegisterPattern(cartAbandonment)

	// Pattern 3: Window shopping (lots of browsing, no action)
	windowShopping := cep.NewPattern("journey-003", "Window Shopping").
		WithDescription("Extensive browsing without adding to cart").
		Sequence(
			cep.Event("browse", "product_view").AtLeast(5),
		).
		NotFollowedBy(
			cep.Event("add_cart", "add_to_cart"),
		).
		Within(30 * time.Minute).
		Build()

	engine.RegisterPattern(windowShopping)

	// Pattern 4: Product comparison (viewing similar items)
	productComparison := cep.NewPattern("journey-004", "Product Comparison").
		WithDescription("User comparing multiple products in same category").
		Sequence(
			cep.Event("view1", "product_view"),
			cep.Event("view2", "product_view"),
			cep.Event("view3", "product_view"),
			cep.Event("add_cart", "add_to_cart").Within(15*time.Minute),
		).
		Within(45 * time.Minute).
		Build()

	engine.RegisterPattern(productComparison)

	// Pattern 5: Re-engagement success (abandoned cart, return and purchase)
	reEngagement := cep.NewPattern("journey-005", "Re-engagement Success").
		WithDescription("User returns after abandonment and completes purchase").
		Sequence(
			cep.Event("initial_cart", "add_to_cart"),
			cep.Event("abandon", "session_end"),
			cep.Event("return", "session_start").Within(24*time.Hour),
			cep.Event("purchase", "purchase_completed").Within(1*time.Hour),
		).
		WithCondition("initial_cart", "purchase", "user_id", "eq").
		Within(48 * time.Hour).
		Build()

	engine.RegisterPattern(reEngagement)

	// Pattern 6: Checkout abandonment (start checkout but don't complete)
	checkoutAbandonment := cep.NewPattern("journey-006", "Checkout Abandonment").
		WithDescription("User starts checkout but doesn't complete purchase").
		Sequence(
			cep.Event("checkout_start", "checkout_initiated"),
			cep.Event("payment_info", "payment_info_entered").Optional(),
		).
		NotFollowedBy(
			cep.Event("purchase", "purchase_completed"),
		).
		Within(30 * time.Minute).
		Build()

	engine.RegisterPattern(checkoutAbandonment)

	// Pattern 7: Price sensitive behavior
	priceSensitive := cep.NewPattern("journey-007", "Price Sensitive Behavior").
		WithDescription("User checks product multiple times without purchasing").
		Sequence(
			cep.Event("view", "product_view").AtLeast(3),
		).
		NotFollowedBy(
			cep.Event("purchase", "purchase_completed"),
		).
		Within(7 * 24 * time.Hour). // Over a week
		Build()

	engine.RegisterPattern(priceSensitive)
}

// simulateSuccessfulConversion simulates a complete purchase journey
func simulateSuccessfulConversion(engine *cep.CEPEngine) {
	userID := "user001"
	baseTime := time.Now()

	events := []map[string]interface{}{
		{
			"type":       "product_view",
			"user_id":    userID,
			"product_id": "laptop-001",
			"category":   "electronics",
			"timestamp":  baseTime,
		},
		{
			"type":       "product_view",
			"user_id":    userID,
			"product_id": "laptop-002",
			"category":   "electronics",
			"timestamp":  baseTime.Add(2 * time.Minute),
		},
		{
			"type":       "add_to_cart",
			"user_id":    userID,
			"product_id": "laptop-001",
			"price":      1299.99,
			"timestamp":  baseTime.Add(5 * time.Minute),
		},
		{
			"type":       "checkout_initiated",
			"user_id":    userID,
			"cart_value": 1299.99,
			"timestamp":  baseTime.Add(8 * time.Minute),
		},
		{
			"type":       "purchase_completed",
			"user_id":    userID,
			"amount":     1299.99,
			"timestamp":  baseTime.Add(10 * time.Minute),
		},
	}

	for _, event := range events {
		engine.ProcessEvent(event)
		fmt.Printf("  ✓ %s\n", event["type"])
	}
}

// simulateCartAbandonment simulates cart abandonment
func simulateCartAbandonment(engine *cep.CEPEngine) {
	userID := "user002"
	baseTime := time.Now()

	events := []map[string]interface{}{
		{
			"type":       "product_view",
			"user_id":    userID,
			"product_id": "shoes-001",
			"timestamp":  baseTime,
		},
		{
			"type":       "product_view",
			"user_id":    userID,
			"product_id": "shoes-002",
			"timestamp":  baseTime.Add(3 * time.Minute),
		},
		{
			"type":       "add_to_cart",
			"user_id":    userID,
			"product_id": "shoes-001",
			"price":      89.99,
			"timestamp":  baseTime.Add(5 * time.Minute),
		},
		{
			"type":       "add_to_cart",
			"user_id":    userID,
			"product_id": "shoes-002",
			"price":      99.99,
			"timestamp":  baseTime.Add(7 * time.Minute),
		},
		// No checkout - cart abandoned
	}

	for _, event := range events {
		engine.ProcessEvent(event)
		fmt.Printf("  ✓ %s\n", event["type"])
	}
	fmt.Println("  ⚠️  User left without checking out")
}

// simulateWindowShopping simulates extensive browsing without action
func simulateWindowShopping(engine *cep.CEPEngine) {
	userID := "user003"
	baseTime := time.Now()

	// Browse 6 products without adding to cart
	for i := 0; i < 6; i++ {
		event := map[string]interface{}{
			"type":       "product_view",
			"user_id":    userID,
			"product_id": fmt.Sprintf("product-%03d", i),
			"timestamp":  baseTime.Add(time.Duration(i*2) * time.Minute),
		}
		engine.ProcessEvent(event)
		fmt.Printf("  👁️  Viewed product-%03d\n", i)
	}
	fmt.Println("  ⚠️  Extensive browsing, no cart additions")
}

// simulateProductComparison simulates product comparison behavior
func simulateProductComparison(engine *cep.CEPEngine) {
	userID := "user004"
	baseTime := time.Now()
	category := "smartphones"

	events := []map[string]interface{}{
		{
			"type":       "product_view",
			"user_id":    userID,
			"product_id": "phone-a",
			"category":   category,
			"timestamp":  baseTime,
		},
		{
			"type":       "product_view",
			"user_id":    userID,
			"product_id": "phone-b",
			"category":   category,
			"timestamp":  baseTime.Add(3 * time.Minute),
		},
		{
			"type":       "product_view",
			"user_id":    userID,
			"product_id": "phone-c",
			"category":   category,
			"timestamp":  baseTime.Add(6 * time.Minute),
		},
		{
			"type":       "add_to_cart",
			"user_id":    userID,
			"product_id": "phone-b",
			"price":      899.99,
			"timestamp":  baseTime.Add(10 * time.Minute),
		},
	}

	for _, event := range events {
		engine.ProcessEvent(event)
		eventType := event["type"].(string)
		if eventType == "product_view" {
			fmt.Printf("  🔍 Comparing %s\n", event["product_id"])
		} else {
			fmt.Printf("  ✓ Decision made: %s\n", event["product_id"])
		}
	}
}

// simulateReEngagement simulates re-engagement after abandonment
func simulateReEngagement(engine *cep.CEPEngine) {
	userID := "user005"
	baseTime := time.Now()

	events := []map[string]interface{}{
		{
			"type":       "add_to_cart",
			"user_id":    userID,
			"product_id": "watch-001",
			"price":      349.99,
			"timestamp":  baseTime,
		},
		{
			"type":      "session_end",
			"user_id":   userID,
			"timestamp": baseTime.Add(5 * time.Minute),
		},
		// User returns after 12 hours (simulated as 12 minutes for demo)
		{
			"type":      "session_start",
			"user_id":   userID,
			"source":    "email_campaign",
			"timestamp": baseTime.Add(12 * time.Hour),
		},
		{
			"type":       "purchase_completed",
			"user_id":    userID,
			"amount":     349.99,
			"timestamp":  baseTime.Add(12*time.Hour + 5*time.Minute),
		},
	}

	for _, event := range events {
		engine.ProcessEvent(event)
		eventType := event["type"].(string)
		switch eventType {
		case "add_to_cart":
			fmt.Printf("  🛒 Added to cart\n")
		case "session_end":
			fmt.Printf("  👋 User left (abandoned cart)\n")
		case "session_start":
			fmt.Printf("  📧 User returned (via %s)\n", event["source"])
		case "purchase_completed":
			fmt.Printf("  ✅ Purchase completed!\n")
		}
	}
}

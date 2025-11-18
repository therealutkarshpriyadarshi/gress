package main

import (
	"context"
	"fmt"
	"time"

	"github.com/therealutkarshpriyadarshi/gress/pkg/join"
	"github.com/therealutkarshpriyadarshi/gress/pkg/stream"
	"github.com/therealutkarshpriyadarshi/gress/pkg/window"
	"go.uber.org/zap"
)

func main() {
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	fmt.Println("=== Gress Stream Join Examples ===\n")

	// Example 1: Inner Join
	fmt.Println("1. Inner Join Example")
	runInnerJoinExample(logger)

	// Example 2: Left Outer Join
	fmt.Println("\n2. Left Outer Join Example")
	runLeftOuterJoinExample(logger)

	// Example 3: Window-based Join
	fmt.Println("\n3. Window-based Join Example")
	runWindowJoinExample(logger)

	// Example 4: Interval Join
	fmt.Println("\n4. Interval Join Example")
	runIntervalJoinExample(logger)

	// Example 5: Temporal Join
	fmt.Println("\n5. Temporal Join Example")
	runTemporalJoinExample(logger)

	// Example 6: Custom Condition Join
	fmt.Println("\n6. Custom Condition Join Example")
	runCustomConditionJoinExample(logger)
}

// Example 1: Inner Join - Join orders with customers
func runInnerJoinExample(logger *zap.Logger) {
	config := join.DefaultJoinConfig().
		WithJoinType(join.InnerJoin).
		WithTimeWindow(1 * time.Minute)

	joinOp := join.NewJoinOperator(config, logger)
	defer joinOp.Close()

	ctx := &stream.ProcessingContext{Ctx: context.Background()}

	// Customer events (left stream)
	customer1 := &stream.Event{
		Key:       "customer-1",
		Value:     map[string]interface{}{"name": "Alice", "tier": "gold"},
		EventTime: time.Now(),
	}

	// Order events (right stream)
	order1 := &stream.Event{
		Key:       "customer-1",
		Value:     map[string]interface{}{"order_id": "order-123", "amount": 100.0},
		EventTime: time.Now(),
	}

	// Process events
	resultsLeft, _ := joinOp.ProcessLeft(ctx, customer1)
	resultsRight, _ := joinOp.ProcessRight(ctx, order1)

	fmt.Printf("  Customer: %v\n", customer1.Value)
	fmt.Printf("  Order: %v\n", order1.Value)
	fmt.Printf("  Joined results: %d (from left) + %d (from right)\n",
		len(resultsLeft), len(resultsRight))

	// Print joined result
	if len(resultsRight) > 0 {
		joined := resultsRight[0].Value.(map[string]interface{})
		fmt.Printf("  Result: customer=%v, order=%v\n", joined["left"], joined["right"])
	}

	metrics := joinOp.GetMetrics()
	fmt.Printf("  Metrics: %d matches found\n", metrics.MatchesFound)
}

// Example 2: Left Outer Join - Include orders without customer data
func runLeftOuterJoinExample(logger *zap.Logger) {
	config := join.DefaultJoinConfig().
		WithJoinType(join.LeftOuterJoin).
		WithTimeWindow(1 * time.Minute)

	joinOp := join.NewJoinOperator(config, logger)
	defer joinOp.Close()

	ctx := &stream.ProcessingContext{Ctx: context.Background()}

	// Order without matching customer
	order := &stream.Event{
		Key:       "customer-999",
		Value:     map[string]interface{}{"order_id": "order-999", "amount": 50.0},
		EventTime: time.Now(),
	}

	results, _ := joinOp.ProcessLeft(ctx, order)

	fmt.Printf("  Order without customer: %v\n", order.Value)
	fmt.Printf("  Results: %d (includes unmatched)\n", len(results))

	if len(results) > 0 {
		joined := results[0].Value.(map[string]interface{})
		fmt.Printf("  Result: order=%v, customer=%v (null)\n", joined["left"], joined["right"])
	}
}

// Example 3: Window-based Join - Join events in 10-second windows
func runWindowJoinExample(logger *zap.Logger) {
	config := join.DefaultJoinConfig().
		WithJoinType(join.InnerJoin)

	// Create tumbling window assigner (10 seconds)
	assigner := window.NewTumblingWindow(10 * time.Second)

	windowJoinOp := join.NewWindowJoinOperator(config, assigner, logger)
	defer windowJoinOp.Close()

	ctx := &stream.ProcessingContext{Ctx: context.Background()}
	baseTime := time.Now()

	// Events in same window
	click := &stream.Event{
		Key:       "user-1",
		Value:     map[string]interface{}{"page": "/product"},
		EventTime: baseTime,
	}

	purchase := &stream.Event{
		Key:       "user-1",
		Value:     map[string]interface{}{"product_id": "prod-123"},
		EventTime: baseTime.Add(5 * time.Second),
	}

	// Process events
	windowJoinOp.ProcessLeft(ctx, click)
	windowJoinOp.ProcessRight(ctx, purchase)

	// Trigger window by watermark
	watermark := baseTime.Add(11 * time.Second)
	results := windowJoinOp.OnWatermark(watermark)

	fmt.Printf("  Click: %v at %s\n", click.Value, click.EventTime.Format("15:04:05"))
	fmt.Printf("  Purchase: %v at %s\n", purchase.Value, purchase.EventTime.Format("15:04:05"))
	fmt.Printf("  Window fired with %d results\n", len(results))

	if len(results) > 0 {
		joined := results[0].Value.(map[string]interface{})
		fmt.Printf("  Joined: click=%v, purchase=%v\n", joined["left"], joined["right"])
	}
}

// Example 4: Interval Join - Join events with time tolerance
func runIntervalJoinExample(logger *zap.Logger) {
	// Join events where right event occurs 0-30 seconds after left event
	intervalJoin := join.NewIntervalJoin().
		Between(0*time.Second, 30*time.Second).
		WithJoinType(join.InnerJoin).
		WithLogger(logger).
		Build()
	defer intervalJoin.Close()

	ctx := &stream.ProcessingContext{Ctx: context.Background()}
	baseTime := time.Now()

	// User login event
	login := &stream.Event{
		Key:       "user-1",
		Value:     map[string]interface{}{"action": "login"},
		EventTime: baseTime,
	}

	// User action within 30 seconds
	action := &stream.Event{
		Key:       "user-1",
		Value:     map[string]interface{}{"action": "view_profile"},
		EventTime: baseTime.Add(15 * time.Second),
	}

	// Process events
	resultsLeft, _ := intervalJoin.ProcessLeft(ctx, login)
	resultsRight, _ := intervalJoin.ProcessRight(ctx, action)

	fmt.Printf("  Login: %v at %s\n", login.Value, login.EventTime.Format("15:04:05"))
	fmt.Printf("  Action: %v at %s (within interval)\n",
		action.Value, action.EventTime.Format("15:04:05"))
	fmt.Printf("  Interval matches: %d (from left) + %d (from right)\n",
		len(resultsLeft), len(resultsRight))

	if len(resultsRight) > 0 {
		joined := resultsRight[0].Value.(map[string]interface{})
		fmt.Printf("  Joined: login=%v, action=%v\n", joined["left"], joined["right"])
	}
}

// Example 5: Temporal Join - Join with versioned reference data
func runTemporalJoinExample(logger *zap.Logger) {
	temporalJoin := join.NewTemporalJoin().
		WithStateRetention(24 * time.Hour).
		WithLogger(logger).
		Build()
	defer temporalJoin.Close()

	ctx := &stream.ProcessingContext{Ctx: context.Background()}
	baseTime := time.Now()

	// Product catalog versions
	product1 := &stream.Event{
		Key:       "product-1",
		Value:     map[string]interface{}{"name": "Widget", "price": 10.0},
		EventTime: baseTime,
	}

	product2 := &stream.Event{
		Key:       "product-1",
		Value:     map[string]interface{}{"name": "Widget", "price": 12.0},
		EventTime: baseTime.Add(1 * time.Hour),
	}

	// Update reference data
	temporalJoin.UpdateReference(product1)
	temporalJoin.UpdateReference(product2)

	// Order event at different times
	order1 := &stream.Event{
		Key:       "product-1",
		Value:     map[string]interface{}{"order_id": "order-1", "quantity": 5},
		EventTime: baseTime.Add(30 * time.Minute), // Should use price 10.0
	}

	order2 := &stream.Event{
		Key:       "product-1",
		Value:     map[string]interface{}{"order_id": "order-2", "quantity": 3},
		EventTime: baseTime.Add(2 * time.Hour), // Should use price 12.0
	}

	// Process orders
	results1, _ := temporalJoin.ProcessLeft(ctx, order1)
	results2, _ := temporalJoin.ProcessLeft(ctx, order2)

	fmt.Printf("  Product version 1: price=10.0 at %s\n",
		baseTime.Format("15:04:05"))
	fmt.Printf("  Product version 2: price=12.0 at %s\n",
		baseTime.Add(1*time.Hour).Format("15:04:05"))

	if len(results1) > 0 {
		joined := results1[0].Value.(map[string]interface{})
		orderData := joined["left"].(map[string]interface{})
		productData := joined["right"].(map[string]interface{})
		fmt.Printf("  Order 1: qty=%v matched with price=%v\n",
			orderData["quantity"], productData["price"])
	}

	if len(results2) > 0 {
		joined := results2[0].Value.(map[string]interface{})
		orderData := joined["left"].(map[string]interface{})
		productData := joined["right"].(map[string]interface{})
		fmt.Printf("  Order 2: qty=%v matched with price=%v\n",
			orderData["quantity"], productData["price"])
	}

	metrics := temporalJoin.GetMetrics()
	fmt.Printf("  Metrics: %d keys, %d versions\n",
		metrics.TotalKeys, metrics.TotalVersions)
}

// Example 6: Custom Condition Join - Join with complex predicates
func runCustomConditionJoinExample(logger *zap.Logger) {
	// Custom condition: join if amount > 100
	customCondition := join.CustomCondition(func(left, right *stream.Event) bool {
		// Must match key
		if left.Key != right.Key {
			return false
		}

		// Custom logic: only join high-value orders
		rightValue := right.Value.(map[string]interface{})
		if amount, ok := rightValue["amount"].(float64); ok {
			return amount > 100.0
		}
		return false
	})

	config := join.DefaultJoinConfig().
		WithJoinType(join.InnerJoin).
		WithCondition(customCondition)

	joinOp := join.NewJoinOperator(config, logger)
	defer joinOp.Close()

	ctx := &stream.ProcessingContext{Ctx: context.Background()}

	customer := &stream.Event{
		Key:       "customer-1",
		Value:     map[string]interface{}{"name": "Bob"},
		EventTime: time.Now(),
	}

	// Low-value order (won't match)
	lowOrder := &stream.Event{
		Key:       "customer-1",
		Value:     map[string]interface{}{"order_id": "order-1", "amount": 50.0},
		EventTime: time.Now(),
	}

	// High-value order (will match)
	highOrder := &stream.Event{
		Key:       "customer-1",
		Value:     map[string]interface{}{"order_id": "order-2", "amount": 150.0},
		EventTime: time.Now(),
	}

	joinOp.ProcessLeft(ctx, customer)
	results1, _ := joinOp.ProcessRight(ctx, lowOrder)
	results2, _ := joinOp.ProcessRight(ctx, highOrder)

	fmt.Printf("  Customer: %v\n", customer.Value)
	fmt.Printf("  Low-value order ($50): matched=%v\n", len(results1) > 0)
	fmt.Printf("  High-value order ($150): matched=%v\n", len(results2) > 0)

	if len(results2) > 0 {
		joined := results2[0].Value.(map[string]interface{})
		fmt.Printf("  Joined result: customer=%v, order=%v\n",
			joined["left"], joined["right"])
	}
}

// Additional helper function for dual stream join
func dualStreamJoinExample() {
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	// Create input channels
	leftInput := make(chan *stream.Event, 100)
	rightInput := make(chan *stream.Event, 100)

	// Configure join
	config := join.DefaultJoinConfig().
		WithJoinType(join.InnerJoin).
		WithTimeWindow(1 * time.Minute)

	// Create dual stream join
	dualJoin := join.NewDualStreamJoin(leftInput, rightInput, config, logger)
	dualJoin.Start()

	// Send events
	go func() {
		leftInput <- &stream.Event{
			Key:       "key-1",
			Value:     "left-value",
			EventTime: time.Now(),
		}
		close(leftInput)
	}()

	go func() {
		rightInput <- &stream.Event{
			Key:       "key-1",
			Value:     "right-value",
			EventTime: time.Now(),
		}
		close(rightInput)
	}()

	// Consume results
	output := dualJoin.Output()
	for result := range output {
		fmt.Printf("Dual stream result: %v\n", result.Value)
	}

	dualJoin.Stop()
}

package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/therealutkarshpriyadarshi/gress/pkg/errors"
	"github.com/therealutkarshpriyadarshi/gress/pkg/metrics"
	"github.com/therealutkarshpriyadarshi/gress/pkg/stream"
	"go.uber.org/zap"
)

func main() {
	fmt.Println("=== Gress Error Handling Example ===")

	// Initialize logger
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	// 1. Error Classification Example
	demonstrateErrorClassification(logger)

	// 2. Retry Policy Example
	demonstrateRetryPolicy(logger)

	// 3. Circuit Breaker Example
	demonstrateCircuitBreaker(logger)

	// 4. Dead Letter Queue Example
	demonstrateDLQ(logger)

	// 5. Side Output Example
	demonstrateSideOutput(logger)

	// 6. Integrated Error Handling Example
	demonstrateIntegratedErrorHandling(logger)

	fmt.Println("\n=== All examples completed ===")
}

func demonstrateErrorClassification(logger *zap.Logger) {
	fmt.Println("\n--- Error Classification Example ---")

	testErrors := []error{
		fmt.Errorf("connection refused"),
		fmt.Errorf("timeout exceeded"),
		fmt.Errorf("invalid syntax"),
		fmt.Errorf("permission denied"),
	}

	for _, err := range testErrors {
		category := errors.ClassifyError(err)
		fmt.Printf("Error: %v -> Category: %s (Retriable: %v)\n",
			err, category, errors.IsRetriable(err))
	}
}

func demonstrateRetryPolicy(logger *zap.Logger) {
	fmt.Println("\n--- Retry Policy Example ---")

	// Create a retry policy
	policy := &errors.RetryPolicy{
		MaxAttempts:       3,
		InitialBackoff:    100 * time.Millisecond,
		MaxBackoff:        1 * time.Second,
		BackoffMultiplier: 2.0,
		Jitter:            0.1,
		RetriableFunc:     errors.IsRetriable,
	}

	ctx := context.Background()
	attemptCount := 0

	// Simulate an operation that fails twice then succeeds
	result := policy.ExecuteWithCallback(
		ctx,
		func(context.Context) error {
			attemptCount++
			fmt.Printf("  Attempt %d...\n", attemptCount)
			if attemptCount < 3 {
				return fmt.Errorf("temporary error")
			}
			return nil
		},
		func(attempt int, err error, nextBackoff time.Duration) {
			fmt.Printf("  Retry scheduled after %v (attempt %d)\n", nextBackoff, attempt)
		},
	)

	if result.Success {
		fmt.Printf("✓ Operation succeeded after %d attempts\n", result.Attempts)
	} else {
		fmt.Printf("✗ Operation failed after %d attempts: %v\n", result.Attempts, result.LastError)
	}
}

func demonstrateCircuitBreaker(logger *zap.Logger) {
	fmt.Println("\n--- Circuit Breaker Example ---")

	config := &errors.CircuitBreakerConfig{
		Name:             "example-service",
		FailureThreshold: 3,
		SuccessThreshold: 2,
		Timeout:          1 * time.Second,
		OnStateChange: func(name string, from, to errors.CircuitState) {
			fmt.Printf("  Circuit '%s' state: %s -> %s\n", name, from, to)
		},
	}

	cb := errors.NewCircuitBreaker(config)
	ctx := context.Background()

	// Simulate failures to open circuit
	fmt.Println("  Simulating failures...")
	for i := 0; i < 5; i++ {
		err := cb.Execute(ctx, func() error {
			return fmt.Errorf("service unavailable")
		})

		if err != nil {
			var openErr *errors.ErrCircuitOpen
			if e, ok := err.(*errors.ErrCircuitOpen); ok {
				openErr = e
			}
			if openErr != nil {
				fmt.Printf("  Request %d: Circuit is OPEN - request rejected\n", i+1)
			} else {
				fmt.Printf("  Request %d: Failed - %v\n", i+1, err)
			}
		}
	}

	// Wait for timeout
	fmt.Println("  Waiting for circuit timeout...")
	time.Sleep(1100 * time.Millisecond)

	// Try again (should be half-open)
	fmt.Println("  Attempting recovery...")
	for i := 0; i < 2; i++ {
		err := cb.Execute(ctx, func() error {
			return nil // Success
		})
		if err == nil {
			fmt.Printf("  Request %d: Success\n", i+1)
		}
	}

	stats := cb.Stats()
	fmt.Printf("✓ Circuit breaker stats: State=%s, Total Requests=%d, Failures=%d\n",
		stats.State, stats.TotalRequests, stats.TotalFailures)
}

func demonstrateDLQ(logger *zap.Logger) {
	fmt.Println("\n--- Dead Letter Queue Example ---")

	// Create an in-memory DLQ
	dlq := errors.NewInMemoryDLQ(100)
	ctx := context.Background()

	// Write failed events to DLQ
	for i := 0; i < 3; i++ {
		failedEvent := &errors.FailedEvent{
			Key:             fmt.Sprintf("key-%d", i),
			Value:           fmt.Sprintf("data-%d", i),
			EventTime:       time.Now(),
			Headers:         map[string]string{"source": "example"},
			Offset:          int64(i),
			Partition:       0,
			FailureReason:   "processing error",
			FailureCategory: "retriable",
			OperatorName:    "example-operator",
			FailureTime:     time.Now(),
			RetryCount:      3,
		}

		if err := dlq.Write(ctx, failedEvent); err != nil {
			logger.Error("Failed to write to DLQ", zap.Error(err))
		}
	}

	// Read from DLQ
	count, _ := dlq.Count(ctx)
	fmt.Printf("  DLQ size: %d events\n", count)

	events, err := dlq.Read(ctx, 10)
	if err != nil {
		logger.Error("Failed to read from DLQ", zap.Error(err))
		return
	}

	fmt.Printf("✓ Read %d events from DLQ:\n", len(events))
	for _, event := range events {
		fmt.Printf("  - Key: %s, Reason: %s, Retries: %d\n",
			event.Key, event.FailureReason, event.RetryCount)
	}
}

func demonstrateSideOutput(logger *zap.Logger) {
	fmt.Println("\n--- Side Output Example ---")

	// Create side output collector
	collector := errors.NewSideOutputCollector()
	errorChan := make(chan *errors.SideOutputEvent, 10)
	collector.RegisterSideOutput(errors.ErrorSideOutput, errorChan)

	ctx := context.Background()

	// Emit some error events
	for i := 0; i < 3; i++ {
		event := &errors.SideOutputEvent{
			Key:           fmt.Sprintf("key-%d", i),
			Value:         fmt.Sprintf("data-%d", i),
			Error:         fmt.Errorf("processing error %d", i),
			ErrorCategory: errors.CategoryRetriable,
			OperatorName:  "example-operator",
			RetryCount:    1,
			Tag:           errors.ErrorSideOutput,
		}

		collector.Emit(ctx, event)
	}

	// Process side output events
	fmt.Println("  Processing error side output:")
	timeout := time.After(100 * time.Millisecond)
	eventCount := 0
	for {
		select {
		case event := <-errorChan:
			eventCount++
			fmt.Printf("  - Error event %d: %s (Category: %s)\n",
				eventCount, event.Key, event.ErrorCategory.String())
		case <-timeout:
			fmt.Printf("✓ Processed %d error events from side output\n", eventCount)
			collector.Close()
			return
		}
	}
}

func demonstrateIntegratedErrorHandling(logger *zap.Logger) {
	fmt.Println("\n--- Integrated Error Handling Example ---")

	// Create metrics collector
	metricsCollector := metrics.NewCollector(logger)

	// Create error handling config
	config := stream.DefaultEngineErrorConfig()
	config.MaxRetryAttempts = 3
	config.InitialBackoff = 50 * time.Millisecond
	config.EnableDLQ = true
	config.EnableSideOutputs = true

	// Create DLQ
	dlq := config.CreateDLQ()

	// Create side output collector
	sideOutputCollector := config.CreateSideOutputCollector()

	// Create error handling operator config
	operatorConfig := &stream.ErrorHandlingConfig{
		RetryPolicy:         config.CreateRetryPolicy(),
		EnableDLQ:           config.EnableDLQ,
		DLQ:                 dlq,
		EnableSideOutputs:   config.EnableSideOutputs,
		SideOutputCollector: sideOutputCollector,
		OperatorName:        "example-operator",
	}

	// Create a mock operator that sometimes fails
	mockOperator := &mockFailingOperator{
		failureRate: 0.3,
	}

	// Wrap it with error handling
	errorHandlingOp := stream.NewErrorHandlingOperator(
		mockOperator,
		operatorConfig,
		metricsCollector,
		logger,
	)

	// Process some events
	ctx := &stream.ProcessingContext{
		Ctx: context.Background(),
	}

	successCount := 0
	failureCount := 0

	for i := 0; i < 10; i++ {
		event := &stream.Event{
			Key:   fmt.Sprintf("key-%d", i),
			Value: fmt.Sprintf("data-%d", i),
		}

		_, err := errorHandlingOp.Process(ctx, event)
		if err != nil {
			failureCount++
			fmt.Printf("  Event %d: Failed after retries\n", i)
		} else {
			successCount++
		}
	}

	fmt.Printf("✓ Processed 10 events: %d succeeded, %d failed\n", successCount, failureCount)

	// Check DLQ
	dlqCount, _ := dlq.Count(context.Background())
	fmt.Printf("  DLQ contains %d failed events\n", dlqCount)
}

// mockFailingOperator simulates an operator that occasionally fails
type mockFailingOperator struct {
	failureRate float64
}

func (m *mockFailingOperator) Process(ctx *stream.ProcessingContext, event *stream.Event) ([]*stream.Event, error) {
	// Randomly fail based on failure rate
	if rand.Float64() < m.failureRate {
		return nil, fmt.Errorf("random processing error")
	}
	return []*stream.Event{event}, nil
}

func (m *mockFailingOperator) Close() error {
	return nil
}

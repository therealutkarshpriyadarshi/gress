# Enhanced Error Handling

This document describes the enhanced error handling features in Gress, including retry policies, circuit breakers, dead letter queues, and error event side outputs.

## Overview

Gress provides a comprehensive error handling system with the following features:

1. **Error Classification** - Automatic categorization of errors (retriable, fatal, transient)
2. **Retry Policies** - Exponential backoff retry logic for transient failures
3. **Circuit Breakers** - Prevent cascading failures with circuit breaker pattern
4. **Dead Letter Queue (DLQ)** - Capture and replay failed events
5. **Side Outputs** - Route error events to separate streams for monitoring
6. **Per-Operator Metrics** - Detailed error tracking and observability

## Error Classification

Errors are automatically classified into three categories:

- **Retriable** - Can be retried with exponential backoff (e.g., network errors, timeouts)
- **Fatal** - Should not be retried (e.g., invalid input, permission denied)
- **Transient** - Can be retried immediately (e.g., EAGAIN, temporary resource unavailability)

### Example

```go
import "github.com/therealutkarshpriyadarshi/gress/pkg/errors"

err := someOperation()
category := errors.ClassifyError(err)

if errors.IsRetriable(err) {
    // Retry the operation
}
```

## Retry Policies

Retry policies define how operations should be retried on failure.

### Configuration

```go
policy := &errors.RetryPolicy{
    MaxAttempts:       3,                      // Maximum retry attempts
    InitialBackoff:    100 * time.Millisecond, // Initial backoff duration
    MaxBackoff:        30 * time.Second,       // Maximum backoff duration
    BackoffMultiplier: 2.0,                    // Exponential multiplier
    Jitter:            0.1,                    // Add randomness (0.0-1.0)
    RetriableFunc:     errors.IsRetriable,     // Custom retry decision
}
```

### Usage

```go
result := policy.Execute(ctx, func(context.Context) error {
    return performOperation()
})

if result.Success {
    fmt.Printf("Succeeded after %d attempts\n", result.Attempts)
} else {
    fmt.Printf("Failed: %v\n", result.LastError)
}
```

### Retry with Callback

```go
result := policy.ExecuteWithCallback(ctx, operation,
    func(attempt int, err error, nextBackoff time.Duration) {
        log.Printf("Retry %d after %v: %v", attempt, nextBackoff, err)
    },
)
```

## Circuit Breakers

Circuit breakers prevent cascading failures by temporarily blocking requests to failing services.

### States

- **Closed** - Normal operation, requests flow through
- **Open** - Too many failures, requests are rejected immediately
- **Half-Open** - Testing if service has recovered

### Configuration

```go
config := &errors.CircuitBreakerConfig{
    Name:                  "my-service",
    FailureThreshold:      5,              // Failures before opening
    SuccessThreshold:      2,              // Successes to close from half-open
    Timeout:               60 * time.Second, // Time before trying half-open
    MaxConcurrentRequests: 1,              // Concurrent requests in half-open
    OnStateChange: func(name string, from, to errors.CircuitState) {
        log.Printf("Circuit %s: %s -> %s", name, from, to)
    },
}

cb := errors.NewCircuitBreaker(config)
```

### Usage

```go
err := cb.Execute(ctx, func() error {
    return callExternalService()
})

if err != nil {
    var openErr *errors.ErrCircuitOpen
    if errors.As(err, &openErr) {
        // Circuit is open, service unavailable
    }
}
```

### With Sinks

```go
// Wrap a sink with circuit breaker
resilientSink := stream.NewCircuitBreakerSink(
    kafkaSink,
    circuitBreakerConfig,
    metricsCollector,
    logger,
)

// Or use resilient sink with both retry and circuit breaker
resilientSink := stream.NewResilientSink(
    kafkaSink,
    resilientConfig,
    metricsCollector,
    logger,
)
```

## Dead Letter Queue (DLQ)

The DLQ captures events that fail processing after all retry attempts are exhausted.

### In-Memory DLQ

```go
dlq := errors.NewInMemoryDLQ(10000) // Max 10,000 events

// Write failed event
failedEvent := &errors.FailedEvent{
    Key:             event.Key,
    Value:           event.Value,
    FailureReason:   err.Error(),
    FailureCategory: errors.ClassifyError(err).String(),
    OperatorName:    "my-operator",
    FailureTime:     time.Now(),
    RetryCount:      3,
}
dlq.Write(ctx, failedEvent)

// Read for replay
events, err := dlq.Read(ctx, 100)

// Count events
count, err := dlq.Count(ctx)
```

### DLQ Manager

```go
manager := errors.NewDLQManager(dlq)

// Automatically tracks statistics
manager.Write(ctx, failedEvent)

stats := manager.Stats()
fmt.Printf("DLQ Stats: Written=%d, Size=%d\n",
    stats.TotalWritten, stats.CurrentSize)
```

## Side Outputs

Side outputs allow routing error events to separate streams for monitoring and debugging.

### Configuration

```go
collector := errors.NewSideOutputCollector()

// Register error side output
errorChan := make(chan *errors.SideOutputEvent, 1000)
collector.RegisterSideOutput(errors.ErrorSideOutput, errorChan)

// Process error events
go func() {
    for event := range errorChan {
        log.Printf("Error: %s - %v", event.Key, event.Error)
    }
}()
```

### Available Tags

- `ErrorSideOutput` - Operator errors
- `LateEventSideOutput` - Late events (outside watermark)
- `FilteredSideOutput` - Filtered events

## Error Handling Configuration

### Engine-Level Configuration

```go
// Default configuration
config := stream.DefaultEngineErrorConfig()

// Production configuration
config := stream.ProductionEngineErrorConfig()

// Development configuration (fail-fast)
config := stream.DevelopmentEngineErrorConfig()

// Custom configuration
config := &stream.EngineErrorConfig{
    Strategy:                 stream.RetryThenDLQ,
    EnableRetry:              true,
    MaxRetryAttempts:         3,
    InitialBackoff:           100 * time.Millisecond,
    MaxBackoff:               30 * time.Second,
    BackoffMultiplier:        2.0,
    BackoffJitter:            0.1,
    EnableDLQ:                true,
    DLQMaxSize:               10000,
    DLQType:                  "memory",
    EnableCircuitBreaker:     true,
    CircuitBreakerThreshold:  5,
    CircuitBreakerTimeout:    60 * time.Second,
    EnableSideOutputs:        true,
    EnableErrorSideOutput:    true,
}
```

### Operator-Level Configuration

```go
// Create error handling config for operator
operatorConfig := &stream.ErrorHandlingConfig{
    RetryPolicy:         config.CreateRetryPolicy(),
    EnableDLQ:           true,
    DLQ:                 dlq,
    EnableSideOutputs:   true,
    SideOutputCollector: sideOutputCollector,
    OperatorName:        "my-operator",
}

// Wrap operator with error handling
errorHandlingOp := stream.NewErrorHandlingOperator(
    myOperator,
    operatorConfig,
    metricsCollector,
    logger,
)
```

## Metrics

Error handling automatically exports Prometheus metrics:

### Retry Metrics

- `gress_retry_attempts_total` - Total retry attempts
- `gress_retry_successes_total` - Successful retries
- `gress_retry_failures_total` - Failed retries (exhausted)
- `gress_retry_backoff_seconds` - Total backoff time

### Circuit Breaker Metrics

- `gress_circuit_breaker_state` - Current state (0=closed, 1=open, 2=half-open)
- `gress_circuit_breaker_transitions_total` - State transitions
- `gress_circuit_breaker_requests_total` - Requests by result
- `gress_circuit_breaker_failures_total` - Tracked failures

### DLQ Metrics

- `gress_dlq_events_written_total` - Events written to DLQ
- `gress_dlq_events_read_total` - Events read from DLQ
- `gress_dlq_events_deleted_total` - Events deleted from DLQ
- `gress_dlq_size_events` - Current DLQ size

### Side Output Metrics

- `gress_side_output_events_total` - Events emitted to side outputs

### Error Category Metrics

- `gress_errors_by_category_total` - Errors by category
- `gress_error_recoveries_total` - Successful error recoveries

## Complete Example

```go
package main

import (
    "github.com/therealutkarshpriyadarshi/gress/pkg/errors"
    "github.com/therealutkarshpriyadarshi/gress/pkg/stream"
    "go.uber.org/zap"
)

func main() {
    logger, _ := zap.NewProduction()
    metricsCollector := metrics.NewCollector(logger)

    // Configure error handling
    errorConfig := stream.ProductionEngineErrorConfig()

    // Create DLQ
    dlq := errorConfig.CreateDLQ()

    // Create side output collector
    sideOutputCollector := errorConfig.CreateSideOutputCollector()

    // Monitor error side output
    errorChan, _ := sideOutputCollector.GetChannel(errors.ErrorSideOutput)
    go func() {
        for event := range errorChan {
            logger.Error("Event failed",
                zap.String("key", event.Key),
                zap.Error(event.Error),
                zap.String("category", event.ErrorCategory.String()),
            )
        }
    }()

    // Create operator with error handling
    operatorConfig := &stream.ErrorHandlingConfig{
        RetryPolicy:         errorConfig.CreateRetryPolicy(),
        EnableDLQ:           true,
        DLQ:                 dlq,
        EnableSideOutputs:   true,
        SideOutputCollector: sideOutputCollector,
        OperatorName:        "my-processor",
    }

    errorHandlingOp := stream.NewErrorHandlingOperator(
        myOperator,
        operatorConfig,
        metricsCollector,
        logger,
    )

    // Create sink with circuit breaker
    resilientSinkConfig := stream.DefaultResilientSinkConfig("kafka-sink")
    resilientSink := stream.NewResilientSink(
        kafkaSink,
        resilientSinkConfig,
        metricsCollector,
        logger,
    )

    // Start metrics server
    metricsServer := metrics.NewServer(":9091", metricsCollector, logger)
    metricsServer.Start()

    // Use in your pipeline...
}
```

## Best Practices

1. **Choose the Right Strategy**
   - Development: Use `FailFast` to catch errors early
   - Production: Use `RetryThenDLQ` for resilience
   - Critical Data: Use `AtLeastOnce` with DLQ

2. **Configure Appropriate Timeouts**
   - Set reasonable backoff limits to avoid long delays
   - Consider downstream system SLAs

3. **Monitor Error Metrics**
   - Set up alerts on error rates and DLQ size
   - Track circuit breaker state changes

4. **Use Side Outputs**
   - Monitor error events in real-time
   - Build custom error handling logic

5. **DLQ Replay Strategy**
   - Periodically review and replay DLQ events
   - Implement automated replay for transient errors
   - Manual review for fatal errors

6. **Circuit Breaker Tuning**
   - Set thresholds based on error rates
   - Adjust timeout based on recovery time
   - Use state change callbacks for alerts

## See Also

- [Metrics Documentation](METRICS.md)
- [Examples](../examples/error_handling/)
- [API Reference](API.md)

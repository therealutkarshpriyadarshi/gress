# Gress Distributed Tracing

This package provides distributed tracing support for the Gress stream processing framework using OpenTelemetry standards.

## Overview

Distributed tracing helps you:
- Track events as they flow through the processing pipeline
- Identify performance bottlenecks in operators
- Debug complex event processing scenarios
- Correlate logs across distributed components
- Monitor end-to-end latency

## Current Implementation

The current implementation provides a **placeholder/stub** for distributed tracing. This allows the framework to be trace-aware without requiring OpenTelemetry dependencies.

### Features

✅ **Trace Context Propagation**: Headers and context management
✅ **Structured Logging Integration**: Automatic trace ID injection into logs
✅ **Instrumentation Points**: Pre-defined spans for common operations
✅ **Zero Dependencies**: Works without external tracing libraries

## Enabling Full OpenTelemetry Support

To enable complete distributed tracing, add these dependencies to `go.mod`:

```bash
go get go.opentelemetry.io/otel@v1.19.0
go get go.opentelemetry.io/otel/sdk@v1.19.0
go get go.opentelemetry.io/otel/exporters/jaeger@v1.17.0
go get go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp@v1.19.0
```

Then replace the placeholder implementations in `tracing.go` with actual OpenTelemetry calls (see inline comments).

## Usage

### Basic Configuration

```go
import (
    "github.com/therealutkarshpriyadarshi/gress/pkg/tracing"
    "go.uber.org/zap"
)

// Create tracing configuration
tracingConfig := &tracing.Config{
    Enabled:          true,
    ServiceName:      "my-stream-app",
    ServiceVersion:   "1.0.0",
    Environment:      "production",
    SamplingRate:     1.0,  // 100% sampling
    ExporterType:     "jaeger",
    ExporterEndpoint: "http://jaeger:14268/api/traces",
}

// Initialize tracing provider
logger, _ := zap.NewProduction()
provider, err := tracing.NewProvider(tracingConfig, logger)
if err != nil {
    log.Fatal(err)
}
defer provider.Shutdown(context.Background())
```

### Tracing Event Processing

```go
// Start a span for event processing
ctx, span := provider.StartSpan(ctx, "process-event")
defer span.End()

// Add attributes to the span
span.SetAttribute("event.key", event.Key)
span.SetAttribute("event.timestamp", event.EventTime)

// Process event...

// Record errors if any
if err != nil {
    span.RecordError(err)
    span.SetStatus(tracing.StatusCodeError, "processing failed")
} else {
    span.SetStatus(tracing.StatusCodeOk, "success")
}
```

### Tracing State Operations

```go
helper := tracing.NewInstrumentationHelper(provider, logger)

// Trace state backend operations
ctx, span := helper.TraceStateOperation(ctx, "get", "rocksdb", "user:123")
defer span.End()

value, err := stateBackend.Get("user:123")
if err != nil {
    span.RecordError(err)
}
```

### Structured Logging with Trace Context

```go
// Create a trace-aware logger
logConfig := tracing.DefaultStructuredLogConfig()
logConfig.Level = zapcore.InfoLevel
logConfig.InitialFields = map[string]interface{}{
    "service": "gress",
    "version": "1.0.0",
}

logger, err := tracing.NewStructuredLogger(logConfig)
if err != nil {
    panic(err)
}

// Log with automatic trace context
tracing.LogWithTrace(ctx, logger, zapcore.InfoLevel, "Processing event",
    zap.String("key", event.Key),
    zap.Int("size", len(event.Value)))
```

### Trace Context Propagation

```go
// Inject trace context into HTTP headers
headers := make(map[string]string)
tracing.InjectTraceContext(ctx, headers)

// Make HTTP request with headers
req.Header.Set("traceparent", headers["traceparent"])

// Extract trace context from incoming headers
incomingHeaders := map[string]string{
    "traceparent": req.Header.Get("traceparent"),
}
ctx = tracing.ExtractTraceContext(ctx, incomingHeaders)
```

## Integration with Stream Engine

The stream engine automatically traces operations when tracing is enabled:

```go
import (
    "github.com/therealutkarshpriyadarshi/gress/pkg/stream"
    "github.com/therealutkarshpriyadarshi/gress/pkg/tracing"
)

// Configure engine with tracing
engineConfig := stream.DefaultEngineConfig()
engineConfig.EnableMetrics = true

engine := stream.NewEngine(engineConfig, logger)

// Initialize tracing
tracingConfig := tracing.DefaultConfig()
tracingConfig.Enabled = true
provider, _ := tracing.NewProvider(tracingConfig, logger)

// Process events - tracing happens automatically
engine.Start()
```

## Instrumentation Points

### Automatic Spans

The following operations are automatically traced when using the instrumentation helper:

1. **Event Processing**: `process_event.<operator_name>`
2. **State Operations**: `state.<operation>` (get, put, delete, snapshot)
3. **Checkpoints**: `checkpoint.create`
4. **Windows**: `window.<type>` (tumbling, sliding, session)

### Span Attributes

Each span includes relevant attributes:

**Event Processing**:
- `event.key`: Event partition key
- `operator.name`: Operator name
- `component`: "stream_processor"

**State Operations**:
- `state.operation`: get/put/delete/snapshot
- `state.backend`: memory/rocksdb
- `state.key`: State key
- `component`: "state_backend"

**Checkpoints**:
- `checkpoint.id`: Unique checkpoint ID
- `component`: "checkpoint_manager"

**Windows**:
- `window.type`: tumbling/sliding/session
- `window.start`: Window start time
- `window.end`: Window end time
- `component`: "window_manager"

## Supported Exporters

### Jaeger (Default)

```go
tracingConfig := &tracing.Config{
    ExporterType:     "jaeger",
    ExporterEndpoint: "http://localhost:14268/api/traces",
}
```

Access Jaeger UI: `http://localhost:16686`

### Zipkin

```go
tracingConfig := &tracing.Config{
    ExporterType:     "zipkin",
    ExporterEndpoint: "http://localhost:9411/api/v2/spans",
}
```

Access Zipkin UI: `http://localhost:9411`

### OTLP (OpenTelemetry Protocol)

```go
tracingConfig := &tracing.Config{
    ExporterType:     "otlp",
    ExporterEndpoint: "http://localhost:4318",
}
```

Compatible with OpenTelemetry Collector, Honeycomb, Lightstep, etc.

## Sampling Strategies

Control trace sampling to reduce overhead:

```go
// Sample 10% of traces
tracingConfig.SamplingRate = 0.1

// Always sample (development)
tracingConfig.SamplingRate = 1.0

// Never sample (disabled)
tracingConfig.SamplingRate = 0.0
```

## Best Practices

### 1. Use Meaningful Span Names

```go
// Good
ctx, span := provider.StartSpan(ctx, "aggregate-user-events")

// Bad
ctx, span := provider.StartSpan(ctx, "operation")
```

### 2. Add Relevant Attributes

```go
span.SetAttributes(map[string]interface{}{
    "event.count": 100,
    "window.duration": "5m",
    "operator.type": "aggregate",
})
```

### 3. Record Errors Properly

```go
if err != nil {
    span.RecordError(err)
    span.SetStatus(tracing.StatusCodeError, err.Error())
    return err
}
span.SetStatus(tracing.StatusCodeOk, "success")
```

### 4. Use Context Propagation

Always pass `context.Context` through your call stack to maintain trace continuity.

### 5. Close Spans with Defer

```go
ctx, span := provider.StartSpan(ctx, "operation")
defer span.End()  // Ensures span is closed even on error
```

### 6. Don't Over-Trace

Trace significant operations, not every line of code. Focus on:
- Service boundaries
- External calls (state backends, sinks, sources)
- Business logic milestones
- Error paths

## Performance Considerations

- **Sampling**: Use sampling rates < 1.0 in high-throughput production
- **Attributes**: Limit attributes to ~10-20 per span
- **Span Count**: Typical traces should have < 50 spans
- **Exporter**: Use asynchronous exporters to avoid blocking

## Troubleshooting

### Traces Not Appearing

1. **Check if tracing is enabled**: `tracingConfig.Enabled = true`
2. **Verify exporter endpoint**: Ensure the endpoint is reachable
3. **Check sampling rate**: Set to 1.0 for testing
4. **Review logs**: Look for tracing initialization messages

### High Memory Usage

- Reduce sampling rate
- Limit span attributes
- Check for unclosed spans

### Missing Spans

- Ensure context is propagated through call stack
- Verify spans are ended with `defer span.End()`
- Check for errors in tracing provider initialization

## Docker Compose Setup

Add Jaeger to your `docker-compose.yml`:

```yaml
services:
  jaeger:
    image: jaegertracing/all-in-one:latest
    ports:
      - "16686:16686"  # Jaeger UI
      - "14268:14268"  # Jaeger collector
    environment:
      - COLLECTOR_ZIPKIN_HOST_PORT=:9411

  gress-app:
    environment:
      - TRACING_ENABLED=true
      - TRACING_EXPORTER=jaeger
      - TRACING_ENDPOINT=http://jaeger:14268/api/traces
```

## Examples

See `/examples` directory for complete examples:
- `examples/tracing/basic-tracing.go` - Basic tracing setup
- `examples/tracing/state-tracing.go` - Tracing state operations
- `examples/tracing/distributed-tracing.go` - Multi-service tracing

## References

- [OpenTelemetry Go Documentation](https://opentelemetry.io/docs/instrumentation/go/)
- [W3C Trace Context](https://www.w3.org/TR/trace-context/)
- [Jaeger Documentation](https://www.jaegertracing.io/docs/)
- [Zipkin Documentation](https://zipkin.io/)

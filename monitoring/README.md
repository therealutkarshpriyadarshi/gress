# Gress Monitoring & Observability

Comprehensive observability stack for the Gress stream processing system, including metrics collection, distributed tracing, and pre-configured dashboards.

## Features

- **Prometheus Metrics**: 40+ production-ready metrics covering all aspects of stream processing
- **Distributed Tracing**: OpenTelemetry-based tracing with Jaeger integration
- **Pre-configured Grafana Dashboards**: 3 comprehensive dashboards ready to use
- **Alerting Rules**: 13 alert rules for proactive monitoring
- **Runtime Metrics**: Go runtime and system metrics collection
- **Error Tracking**: Detailed error, retry, circuit breaker, and DLQ metrics

## Quick Start

### 1. Start the Observability Stack

```bash
cd monitoring
docker-compose up -d
```

This starts:
- **Prometheus** (http://localhost:9090) - Metrics storage and querying
- **Grafana** (http://localhost:3000) - Visualization (admin/admin)
- **Jaeger** (http://localhost:16686) - Distributed tracing UI
- **OpenTelemetry Collector** (ports 4317/4318) - Trace/metric collection

### 2. Configure Gress

Enable metrics and tracing in your `config.yaml`:

```yaml
# Metrics configuration
metrics:
  enabled: true
  address: ":9091"
  path: "/metrics"

# Tracing configuration
tracing:
  enabled: true
  service_name: "gress"
  service_version: "1.0.0"
  environment: "production"
  sampling_rate: 1.0  # 100% sampling
  exporter_type: "otlp"  # or "jaeger", "stdout"
  exporter_endpoint: "http://localhost:4318/v1/traces"
  otlp_insecure: true
```

### 3. Start Gress

```bash
./bin/gress --config config.yaml
```

### 4. Access Dashboards

Open Grafana at http://localhost:3000 and navigate to the "Gress" folder to find:
- **Gress Stream Processing - Overview**: Main operational dashboard
- **Gress Stream Processing - Runtime & System Metrics**: Go runtime monitoring
- **Gress Stream Processing - Error Handling**: Error tracking and reliability

## Metrics Reference

### Engine Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `gress_events_processed_total` | Counter | Total events processed by source |
| `gress_events_filtered_total` | Counter | Total events filtered by operator |
| `gress_processing_latency_seconds` | Histogram | Event processing latency |
| `gress_backpressure_events_total` | Counter | Backpressure events |
| `gress_buffer_utilization_ratio` | Gauge | Buffer utilization (0.0-1.0) |

### Operator Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `gress_operator_events_in_total` | Counter | Events received by operator |
| `gress_operator_events_out_total` | Counter | Events emitted by operator |
| `gress_operator_latency_seconds` | Histogram | Operator processing latency |
| `gress_operator_errors_total` | Counter | Operator errors by type |

### State Backend Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `gress_state_operations_total` | Counter | State operations (get/put/delete) |
| `gress_state_operation_latency_seconds` | Histogram | State operation latency |
| `gress_state_size_bytes` | Gauge | Current state size |

### Checkpoint Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `gress_checkpoint_duration_seconds` | Histogram | Checkpoint duration |
| `gress_checkpoint_success_total` | Counter | Successful checkpoints |
| `gress_checkpoint_failure_total` | Counter | Failed checkpoints |
| `gress_checkpoint_size_bytes` | Gauge | Last checkpoint size |
| `gress_time_since_last_checkpoint_seconds` | Gauge | Time since last checkpoint |

### Error Handling Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `gress_retry_attempts_total` | Counter | Retry attempts |
| `gress_retry_successes_total` | Counter | Successful retries |
| `gress_retry_failures_total` | Counter | Failed retries |
| `gress_retry_backoff_seconds` | Histogram | Retry backoff duration |
| `gress_circuit_breaker_state` | Gauge | Circuit breaker state (0/1/2) |
| `gress_circuit_breaker_requests_total` | Counter | Circuit breaker requests |
| `gress_dlq_events_written_total` | Counter | Events written to DLQ |
| `gress_dlq_size_events` | Gauge | Current DLQ size |

### Runtime Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `gress_runtime_alloc_bytes` | Gauge | Allocated heap bytes |
| `gress_runtime_sys_bytes` | Gauge | System memory bytes |
| `gress_runtime_goroutines` | Gauge | Active goroutines |
| `gress_runtime_gc_total` | Counter | Total GC cycles |
| `gress_runtime_gc_pause_seconds` | Histogram | GC pause duration |
| `gress_process_uptime_seconds` | Gauge | Process uptime |

## Distributed Tracing

### Exporters

Gress supports three tracing exporters:

1. **OTLP (Recommended)**: Export to OpenTelemetry Collector
   ```yaml
   exporter_type: "otlp"
   exporter_endpoint: "http://localhost:4318/v1/traces"
   ```

2. **Jaeger**: Direct export to Jaeger
   ```yaml
   exporter_type: "jaeger"
   exporter_endpoint: "http://localhost:14268/api/traces"
   ```

3. **Stdout**: Console output for debugging
   ```yaml
   exporter_type: "stdout"
   ```

### Sampling

Control trace sampling with `sampling_rate`:
- `1.0`: Sample all traces (development)
- `0.1`: Sample 10% of traces (production)
- `0.01`: Sample 1% of traces (high volume)

### Using Traces in Code

```go
import "github.com/therealutkarshpriyadarshi/gress/pkg/tracing"

// Start a span
ctx, span := tracingProvider.StartSpan(ctx, "operation-name")
defer span.End()

// Add attributes
tracing.SetSpanAttributes(ctx, map[string]interface{}{
    "event.id": eventID,
    "operator": operatorName,
})

// Record errors
if err != nil {
    tracing.RecordError(ctx, err)
}

// Add events
tracing.AddEvent(ctx, "checkpoint-started", map[string]interface{}{
    "size": checkpointSize,
})
```

## Alerting

Alert rules are defined in `alerts/gress-alerts.yml`. Configure Alertmanager to receive notifications:

```yaml
# alertmanager.yml
route:
  receiver: 'slack'
  group_wait: 30s
  group_interval: 5m
  repeat_interval: 4h

receivers:
  - name: 'slack'
    slack_configs:
      - api_url: 'YOUR_SLACK_WEBHOOK'
        channel: '#alerts'
```

### Key Alerts

- **HighErrorRate**: Error rate > 1/sec
- **CriticalErrorRate**: Error rate > 10/sec
- **HighProcessingLatency**: P99 latency > 100ms
- **CircuitBreakerOpen**: Circuit breaker is open
- **HighDLQSize**: DLQ > 1000 events
- **GressDown**: Service unavailable

## Dashboard Guide

### Overview Dashboard

**Purpose**: High-level operational view

**Key Panels**:
- Events Processed (Total)
- Processing Latency (P99)
- Event Processing Rate by Source
- Operator Events In/Out
- Buffer Utilization
- Checkpoint Success Rate

**Use Cases**:
- Daily operations monitoring
- Performance overview
- Quick health checks

### Runtime Dashboard

**Purpose**: Go runtime and resource monitoring

**Key Panels**:
- Memory Usage (Allocated, System, Heap)
- Goroutines
- GC Activity and Pause Duration
- Heap Objects
- Process Uptime

**Use Cases**:
- Memory leak detection
- Performance tuning
- Capacity planning

### Error Handling Dashboard

**Purpose**: Error tracking and reliability monitoring

**Key Panels**:
- Total Errors
- Retry Success Rate
- Circuit Breaker State
- DLQ Activity
- Errors by Category

**Use Cases**:
- Troubleshooting failures
- Monitoring retry effectiveness
- DLQ management

## Production Best Practices

### 1. Metrics Retention

Configure Prometheus retention:
```yaml
# prometheus.yml
global:
  scrape_interval: 15s
  evaluation_interval: 15s

storage:
  tsdb:
    retention.time: 15d
    retention.size: 50GB
```

### 2. Trace Sampling

Use adaptive sampling in production:
```yaml
tracing:
  sampling_rate: 0.1  # 10% sampling
```

### 3. Alert Tuning

Adjust thresholds based on your workload:
- Start with conservative thresholds
- Monitor false positive rate
- Tune based on baseline performance

### 4. Dashboard Organization

Create custom dashboards for:
- SLA monitoring
- Business metrics
- Capacity planning
- Incident investigation

### 5. Metric Cardinality

Be cautious with high-cardinality labels:
- Avoid user IDs, timestamps in labels
- Use `operator`, `source`, `backend` labels
- Monitor Prometheus memory usage

## Troubleshooting

### Metrics Not Appearing

1. Check Gress is running: `curl http://localhost:9091/metrics`
2. Verify Prometheus target: http://localhost:9090/targets
3. Check Prometheus logs: `docker logs gress-prometheus`

### Traces Not Appearing

1. Verify tracing is enabled in config
2. Check OTLP collector: `curl http://localhost:13133`
3. Check Jaeger UI: http://localhost:16686
4. Review Gress logs for tracing errors

### High Memory Usage

1. Check `gress_runtime_alloc_bytes` metric
2. Review GC pause times
3. Check for goroutine leaks
4. Adjust buffer sizes in config

### Dashboard Not Loading

1. Verify Grafana datasource: Settings > Data Sources
2. Check Prometheus connectivity
3. Import dashboards manually if needed

## Architecture

```
┌─────────────┐
│   Gress     │
│  :9091      │ ─────┐
└─────────────┘      │
                     │ Metrics
                     ▼
              ┌──────────────┐
              │  Prometheus  │
              │    :9090     │
              └──────────────┘
                     │
                     │ Query
                     ▼
              ┌──────────────┐
              │   Grafana    │
              │    :3000     │
              └──────────────┘

┌─────────────┐
│   Gress     │ ─────┐
│ (tracing)   │      │ Traces (OTLP)
└─────────────┘      │
                     ▼
              ┌──────────────┐
              │     OTLP     │
              │  Collector   │
              │  :4317/4318  │
              └──────────────┘
                     │
                     │ Export
                     ▼
              ┌──────────────┐
              │   Jaeger     │
              │   :16686     │
              └──────────────┘
```

## API Endpoints

### Metrics

- **Endpoint**: `GET http://localhost:9091/metrics`
- **Format**: Prometheus exposition format
- **Auth**: None

### Health Check

- **Endpoint**: `GET http://localhost:9091/health`
- **Response**: `OK` (200)

## Custom Metrics

Add custom application metrics:

```go
import "github.com/prometheus/client_golang/prometheus"

// Create custom metric
customCounter := prometheus.NewCounter(prometheus.CounterOpts{
    Name: "my_custom_metric_total",
    Help: "My custom metric description",
})

// Register with metrics collector
err := metricsCollector.RegisterCustomMetric("my_metric", customCounter)

// Use the metric
customCounter.Inc()
```

## Further Reading

- [Prometheus Documentation](https://prometheus.io/docs/)
- [Grafana Documentation](https://grafana.com/docs/)
- [OpenTelemetry Documentation](https://opentelemetry.io/docs/)
- [Jaeger Documentation](https://www.jaegertracing.io/docs/)

## Support

For issues or questions:
- GitHub Issues: https://github.com/therealutkarshpriyadarshi/gress/issues
- Documentation: See `/docs` directory

# Gress Metrics Example

This example demonstrates how to use Prometheus metrics with the Gress stream processing framework.

## Features Demonstrated

1. ✅ **Built-in Metrics**: Automatic collection of engine and operator metrics
2. ✅ **Custom Metrics**: Registering application-specific metrics
3. ✅ **Metrics HTTP Endpoint**: Exposing metrics for Prometheus scraping
4. ✅ **Grafana Dashboards**: Visualizing metrics in pre-built dashboards

## Running the Example

### Prerequisites

- Go 1.21+
- Docker and Docker Compose (for Prometheus and Grafana)

### Option 1: Run with Full Observability Stack

1. **Start the infrastructure** (Prometheus + Grafana):

```bash
cd ../../deployments/docker
docker-compose up -d prometheus grafana
```

2. **Run the example**:

```bash
cd examples/metrics
go run main.go
```

3. **Access the metrics**:

- **Metrics endpoint**: http://localhost:9091/metrics
- **Health check**: http://localhost:9091/health
- **Prometheus UI**: http://localhost:9090
- **Grafana dashboards**: http://localhost:3000 (admin/admin)

### Option 2: Run Standalone (Without Docker)

```bash
cd examples/metrics
go run main.go
```

Then access the metrics endpoint directly:

```bash
curl http://localhost:9091/metrics
```

## What You'll See

### Console Output

```
{"level":"info","timestamp":"2024-01-15T10:30:00.000Z","message":"Starting Gress Metrics Example"}
{"level":"info","timestamp":"2024-01-15T10:30:00.001Z","message":"Metrics endpoint will be available at http://localhost:9091/metrics"}
{"level":"info","timestamp":"2024-01-15T10:30:00.002Z","message":"Starting metrics server","addr":":9091"}
{"level":"info","timestamp":"2024-01-15T10:30:00.003Z","message":"Starting stream processing engine","sources":1,"operators":3,"sinks":1,"metrics_enabled":true}
```

### Metrics Endpoint Sample

```
# HELP gress_events_processed_total Total number of events processed by the engine
# TYPE gress_events_processed_total counter
gress_events_processed_total{source="mock-source"} 1523

# HELP gress_processing_latency_seconds Event processing latency in seconds
# TYPE gress_processing_latency_seconds histogram
gress_processing_latency_seconds_bucket{source="mock-source",le="0.005"} 1450
gress_processing_latency_seconds_bucket{source="mock-source",le="0.01"} 1520
gress_processing_latency_seconds_sum{source="mock-source"} 2.543

# HELP gress_operator_events_in_total Total number of events received by operator
# TYPE gress_operator_events_in_total counter
gress_operator_events_in_total{operator="operator-0",operator_type="*stream.FilterOperator"} 1523
gress_operator_events_in_total{operator="operator-1",operator_type="*stream.MapOperator"} 1234

# HELP gress_buffer_utilization_ratio Current buffer utilization (0.0 to 1.0)
# TYPE gress_buffer_utilization_ratio gauge
gress_buffer_utilization_ratio 0.15
```

## Available Metrics

### Engine Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `gress_events_processed_total` | Counter | Total events processed |
| `gress_events_filtered_total` | Counter | Events filtered out |
| `gress_processing_latency_seconds` | Histogram | End-to-end processing latency |
| `gress_backpressure_events_total` | Counter | Backpressure occurrences |
| `gress_buffer_utilization_ratio` | Gauge | Buffer usage (0.0-1.0) |

### Operator Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `gress_operator_events_in_total` | Counter | Events received by operator |
| `gress_operator_events_out_total` | Counter | Events emitted by operator |
| `gress_operator_latency_seconds` | Histogram | Operator processing time |
| `gress_operator_errors_total` | Counter | Operator errors |

### Checkpoint Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `gress_checkpoint_duration_seconds` | Histogram | Checkpoint creation time |
| `gress_checkpoint_success_total` | Counter | Successful checkpoints |
| `gress_checkpoint_failure_total` | Counter | Failed checkpoints |
| `gress_checkpoint_size_bytes` | Gauge | Checkpoint size |
| `gress_time_since_last_checkpoint_seconds` | Gauge | Time since last checkpoint |

### Custom Metrics (in this example)

| Metric | Type | Description |
|--------|------|-------------|
| `gress_example_events_processed` | Counter | Example app event counter |
| `gress_example_active_users` | Gauge | Simulated active users |
| `gress_example_event_size_bytes` | Histogram | Event size distribution |

## Querying Metrics in Prometheus

### Basic Queries

**Events per second:**
```promql
rate(gress_events_processed_total[1m])
```

**P99 latency:**
```promql
histogram_quantile(0.99, rate(gress_processing_latency_seconds_bucket[5m]))
```

**Buffer utilization:**
```promql
gress_buffer_utilization_ratio
```

**Operator throughput:**
```promql
sum(rate(gress_operator_events_in_total[1m])) by (operator)
```

### Advanced Queries

**Checkpoint success rate:**
```promql
rate(gress_checkpoint_success_total[5m]) /
(rate(gress_checkpoint_success_total[5m]) + rate(gress_checkpoint_failure_total[5m]))
```

**Operator selectivity (output/input ratio):**
```promql
rate(gress_operator_events_out_total[1m]) /
rate(gress_operator_events_in_total[1m])
```

**Processing lag:**
```promql
avg(gress_watermark_lag_seconds)
```

## Visualizing with Grafana

### Pre-built Dashboards

The example includes 4 pre-configured Grafana dashboards:

1. **System Overview** (`gress-system-overview`)
   - High-level system metrics
   - Throughput, latency, backpressure

2. **Operator Metrics** (`gress-operator-metrics`)
   - Per-operator performance
   - Error tracking

3. **Watermark Progression** (`gress-watermark-progression`)
   - Event-time tracking
   - Lag monitoring

4. **Checkpoint Health** (`gress-checkpoint-health`)
   - Fault tolerance monitoring
   - State size tracking

### Accessing Dashboards

1. Open Grafana: http://localhost:3000
2. Login with `admin` / `admin`
3. Navigate to **Dashboards** > **Browse**
4. Select one of the Gress dashboards

## Custom Metrics Registration

To add your own application metrics:

```go
// Get the metrics collector
collector := engine.GetMetricsCollector()

// Create a custom counter
myCounter := prometheus.NewCounter(prometheus.CounterOpts{
    Name: "myapp_events_total",
    Help: "Total events processed by my app",
})

// Register the metric
err := collector.RegisterCustomMetric("my_events", myCounter)

// Use the metric
myCounter.Inc()
```

### Supported Metric Types

- **Counter**: Monotonically increasing value
- **Gauge**: Value that can go up or down
- **Histogram**: Distribution of values
- **Summary**: Similar to histogram with configurable quantiles

## Configuration

Customize metrics collection in the engine config:

```go
config := stream.DefaultEngineConfig()

// Enable/disable metrics
config.EnableMetrics = true

// Change metrics server address
config.MetricsAddr = ":9091"

// Adjust metrics reporting interval
config.MetricsInterval = 10 * time.Second
```

## Alerting

Example Prometheus alert rules:

```yaml
groups:
  - name: gress
    rules:
      - alert: HighBackpressure
        expr: rate(gress_backpressure_events_total[5m]) > 10
        for: 2m
        annotations:
          summary: "High backpressure detected"

      - alert: CheckpointFailures
        expr: increase(gress_checkpoint_failure_total[15m]) > 0
        annotations:
          summary: "Checkpoint failures detected"

      - alert: HighLatency
        expr: histogram_quantile(0.99, rate(gress_processing_latency_seconds_bucket[5m])) > 1
        for: 5m
        annotations:
          summary: "P99 latency exceeds 1 second"
```

## Testing

To generate load and see metrics in action:

```bash
# Run the example
go run main.go

# In another terminal, watch metrics update
watch -n 1 'curl -s http://localhost:9091/metrics | grep gress_events_processed_total'

# Or query Prometheus
curl 'http://localhost:9090/api/v1/query?query=rate(gress_events_processed_total[1m])'
```

## Troubleshooting

### Metrics endpoint not accessible

**Problem**: `curl http://localhost:9091/metrics` fails

**Solutions**:
1. Check if metrics are enabled: `config.EnableMetrics = true`
2. Verify the address is correct: `config.MetricsAddr = ":9091"`
3. Check if port is already in use: `lsof -i :9091`

### No data in Grafana

**Problem**: Dashboards show "No Data"

**Solutions**:
1. Verify Prometheus is scraping: http://localhost:9090/targets
2. Check Prometheus config has the correct target
3. Ensure the data source in Grafana is configured correctly

### High memory usage

**Problem**: Application memory grows over time

**Solutions**:
1. Check for label cardinality (too many unique label combinations)
2. Limit the number of custom metrics
3. Adjust Prometheus retention settings

## Next Steps

1. **Explore Grafana Dashboards**: Check out the pre-built dashboards
2. **Add Custom Metrics**: Track application-specific KPIs
3. **Set Up Alerts**: Configure Prometheus alerting rules
4. **Production Deployment**: Use the Docker Compose setup

## References

- [Prometheus Documentation](https://prometheus.io/docs/)
- [Grafana Dashboards](../../deployments/grafana/dashboards/README.md)
- [Metrics Package](../../pkg/metrics/)
- [Gress Documentation](../../README.md)

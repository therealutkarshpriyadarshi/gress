# Gress Grafana Dashboards

This directory contains pre-built Grafana dashboards for monitoring the Gress stream processing framework.

## Available Dashboards

### 1. System Overview (`gress-system-overview.json`)
**Purpose**: High-level monitoring of the entire stream processing system

**Key Metrics**:
- Events per second throughput
- Buffer utilization percentage
- P99 processing latency
- Backpressure events
- Event throughput by source
- Processing latency percentiles (p50, p95, p99)
- Buffer utilization over time
- Filtered events by operator

**Use Cases**:
- Quick health check of the system
- Identifying performance bottlenecks
- Monitoring overall throughput
- Detecting backpressure issues

---

### 2. Operator Metrics (`gress-operator-metrics.json`)
**Purpose**: Detailed monitoring of individual operators in the processing pipeline

**Key Metrics**:
- Operator input/output rates
- Per-operator processing latency percentiles
- Operator error counts by type
- Operator selectivity (output/input ratio)

**Features**:
- Template variable for filtering by operator
- Multi-select support for comparing operators
- Error tracking with error type breakdown

**Use Cases**:
- Debugging operator performance issues
- Identifying bottlenecks in the pipeline
- Monitoring operator selectivity and filtering efficiency
- Tracking operator-specific errors

---

### 3. Watermark Progression (`gress-watermark-progression.json`)
**Purpose**: Monitoring event-time processing and watermark advancement

**Key Metrics**:
- Average watermark lag
- Current watermark timestamp
- Watermark lag by partition
- Watermark progression over time
- Per-partition watermark lag visualization

**Use Cases**:
- Ensuring timely processing of events
- Detecting partitions with high lag
- Monitoring out-of-order event handling
- Debugging windowing and time-based aggregations

---

### 4. Checkpoint Health (`gress-checkpoint-health.json`)
**Purpose**: Monitoring fault tolerance and state management

**Key Metrics**:
- Time since last checkpoint
- Successful/failed checkpoint counts
- Checkpoint duration percentiles
- Checkpoint size over time
- Checkpoint success rate
- State size by backend

**Use Cases**:
- Ensuring fault tolerance is functioning correctly
- Monitoring checkpoint performance
- Detecting checkpoint failures
- Tracking state growth over time

---

## Installation

### Automatic Provisioning (Recommended)

The dashboards are automatically provisioned when using the provided Docker Compose setup:

```bash
cd deployments/docker
docker-compose up -d
```

Grafana will be available at: `http://localhost:3000`
- Username: `admin`
- Password: `admin`

### Manual Import

1. Access Grafana at `http://localhost:3000`
2. Navigate to **Dashboards > Import**
3. Click **Upload JSON file**
4. Select one of the dashboard JSON files
5. Choose the Prometheus datasource
6. Click **Import**

---

## Dashboard Configuration

### Data Source
All dashboards are configured to use a Prometheus datasource. Ensure Prometheus is configured to scrape metrics from the Gress application:

```yaml
scrape_configs:
  - job_name: 'gress'
    static_configs:
      - targets: ['gress-app:9091']
```

### Refresh Rate
Default refresh rate: **10 seconds**

To change:
1. Click the refresh dropdown in the top-right corner
2. Select your preferred interval

### Time Range
Default time range: **Last 15 minutes**

Recommended ranges:
- Real-time monitoring: 5-15 minutes
- Historical analysis: 1-24 hours
- Trend analysis: 7-30 days

---

## Metrics Reference

### Engine Metrics
- `gress_events_processed_total{source}` - Total events processed by source
- `gress_events_filtered_total{operator}` - Total events filtered by operator
- `gress_processing_latency_seconds` - Processing latency histogram
- `gress_backpressure_events_total` - Total backpressure events
- `gress_buffer_utilization_ratio` - Buffer utilization (0.0-1.0)

### Operator Metrics
- `gress_operator_events_in_total{operator, operator_type}` - Events received by operator
- `gress_operator_events_out_total{operator, operator_type}` - Events emitted by operator
- `gress_operator_latency_seconds{operator, operator_type}` - Operator latency histogram
- `gress_operator_errors_total{operator, operator_type, error_type}` - Operator errors

### State Backend Metrics
- `gress_state_operations_total{backend, operation}` - State backend operation counts
- `gress_state_operation_latency_seconds{backend, operation}` - State operation latency
- `gress_state_size_bytes{backend, operator}` - State size in bytes

### Watermark Metrics
- `gress_watermark_lag_seconds{partition}` - Watermark lag by partition
- `gress_watermark_timestamp` - Current watermark timestamp (unix seconds)

### Checkpoint Metrics
- `gress_checkpoint_duration_seconds` - Checkpoint duration histogram
- `gress_checkpoint_success_total` - Successful checkpoint count
- `gress_checkpoint_failure_total` - Failed checkpoint count
- `gress_checkpoint_size_bytes` - Checkpoint size
- `gress_time_since_last_checkpoint_seconds` - Time since last checkpoint

### Window Metrics
- `gress_window_events_processed_total{window_type}` - Events processed by windows
- `gress_window_late_fired_total{window_type}` - Windows fired with late data
- `gress_window_size_events{window_type, window_id}` - Active window sizes

---

## Alerting

### Recommended Alerts

**High Backpressure**
```promql
rate(gress_backpressure_events_total[5m]) > 10
```
Indicates the system is struggling to keep up with incoming load.

**High Processing Latency**
```promql
histogram_quantile(0.99, rate(gress_processing_latency_seconds_bucket[5m])) > 1
```
P99 latency exceeds 1 second.

**Checkpoint Failures**
```promql
increase(gress_checkpoint_failure_total[15m]) > 0
```
Any checkpoint failures in the last 15 minutes.

**High Watermark Lag**
```promql
max(gress_watermark_lag_seconds) > 300
```
Watermark lag exceeds 5 minutes.

**Operator Errors**
```promql
rate(gress_operator_errors_total[5m]) > 0
```
Any operator errors in the last 5 minutes.

---

## Customization

### Adding Custom Panels

1. Edit the dashboard in Grafana UI
2. Add new panel with your query
3. Export the dashboard JSON
4. Replace the file in this directory

### Variables

The Operator Metrics dashboard includes a template variable `$operator` for filtering. To add more variables:

1. Click dashboard settings (gear icon)
2. Go to **Variables** tab
3. Add new variable with Prometheus query

Example variable query:
```promql
label_values(gress_events_processed_total, source)
```

---

## Troubleshooting

### Dashboard Shows "No Data"

1. **Check Prometheus target**: Navigate to `http://localhost:9090/targets` and verify the Gress app is being scraped
2. **Verify metrics endpoint**: `curl http://localhost:9091/metrics` should return Prometheus metrics
3. **Check time range**: Ensure the dashboard time range includes recent data
4. **Verify datasource**: Dashboard settings > Data source should be set to Prometheus

### Metrics Not Updating

1. **Check metrics enabled**: Ensure `EnableMetrics: true` in engine configuration
2. **Verify scrape interval**: Check Prometheus scrape configuration
3. **Check network**: Ensure Prometheus can reach the metrics endpoint

### High Cardinality Warnings

If you see warnings about high cardinality:
- Limit the number of unique label values (especially for `window_id`)
- Use recording rules to pre-aggregate metrics
- Increase Prometheus retention settings

---

## Best Practices

1. **Monitor all 4 dashboards regularly**: Each provides different insights into system health
2. **Set up alerts**: Don't rely solely on dashboards for critical issues
3. **Adjust time ranges**: Use longer ranges for trend analysis, shorter for real-time debugging
4. **Create custom views**: Duplicate and customize dashboards for specific use cases
5. **Export dashboards**: Regularly export and version control dashboard JSON

---

## Support

For issues or questions:
- GitHub Issues: https://github.com/therealutkarshpriyadarshi/gress/issues
- Documentation: https://github.com/therealutkarshpriyadarshi/gress/blob/main/README.md

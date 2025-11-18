# Real-Time Analytics Dashboard

A comprehensive real-time analytics system built on Gress for tracking user behavior, calculating live KPIs, and analyzing sessions.

## Features

### 1. User Behavior Tracking
- **Event Enrichment**: Automatically enriches user events with computed fields
- **Action Classification**: Identifies conversions, high-value actions
- **Temporal Context**: Adds hour-of-day and day-of-week analysis
- **Multi-dimensional Tracking**: Tracks users across sessions, devices, and locations

### 2. Live KPI Calculations
Real-time calculation of key performance indicators:
- **User Metrics**: Total users, active users, new users
- **Session Metrics**: Total sessions, average session duration
- **Engagement Metrics**: Bounce rate, engagement scores
- **Conversion Metrics**: Conversion rate, conversions per session
- **Revenue Metrics**: Total revenue, revenue per user, average order value
- **Trend Analysis**: Automatic trend detection (up/down/stable)

### 3. Session Analysis
- **Session Windows**: Automatic session boundary detection
- **Session Aggregation**: Aggregates events by user session
- **Behavior Patterns**: Identifies user behavior patterns within sessions
- **Engagement Scoring**: Calculates engagement scores (0-100) based on:
  - Number of events (0-30 points)
  - Page diversity (0-20 points)
  - Time spent (0-30 points)
  - Action diversity (0-20 points)

### 4. Page View Analytics
- Real-time page view statistics
- Unique visitor tracking
- Average time on page
- Popular page identification

## Architecture

```
┌─────────────────┐
│  HTTP Ingestion │
│   /ingest       │
└────────┬────────┘
         │
         ▼
┌─────────────────────────┐
│  Behavior Tracker       │
│  - Enrichment           │
│  - Classification       │
└────────┬────────────────┘
         │
         ├──────────────────────┐
         │                      │
         ▼                      ▼
┌──────────────────┐   ┌─────────────────┐
│ Session          │   │ KPI Calculator  │
│ Aggregator       │   │ (1-min windows) │
└────────┬─────────┘   └────────┬────────┘
         │                      │
         ▼                      ▼
┌──────────────────┐   ┌─────────────────┐
│  TimescaleDB     │   │  Kafka Topics   │
│  - Sessions      │   │  - KPIs         │
│  - Page Views    │   │  - Enriched     │
└──────────────────┘   └─────────────────┘
```

## Data Model

### Input Event
```json
{
  "user_id": "user-123",
  "session_id": "session-456",
  "action": "page_view",
  "page": "/products",
  "timestamp": "2024-01-15T10:30:00Z",
  "duration": 5000,
  "metadata": {
    "browser": "Chrome",
    "version": "120.0"
  },
  "ip_address": "192.168.1.1",
  "user_agent": "Mozilla/5.0...",
  "country": "US",
  "device": "mobile"
}
```

### Enriched Event
```json
{
  "user_id": "user-123",
  "session_id": "session-456",
  "action": "page_view",
  "page": "/products",
  "timestamp": "2024-01-15T10:30:00Z",
  "duration": 5000,
  "is_conversion": false,
  "is_high_value": false,
  "hour_of_day": 10,
  "day_of_week": "Monday",
  ...
}
```

### Session Metrics
```json
{
  "user_id": "user-123",
  "session_id": "session-456",
  "window_start": "2024-01-15T10:00:00Z",
  "window_end": "2024-01-15T10:30:00Z",
  "total_events": 15,
  "unique_pages": 5,
  "total_duration_ms": 75000,
  "average_duration_ms": 5000,
  "actions": {
    "page_view": 10,
    "click": 4,
    "purchase": 1
  },
  "pages": {
    "/home": 1,
    "/products": 5,
    "/checkout": 1
  },
  "device_type": "mobile",
  "country": "US"
}
```

### KPI Metrics
```json
{
  "window_start": "2024-01-15T10:00:00Z",
  "window_end": "2024-01-15T10:01:00Z",
  "total_users": 1500,
  "active_users": 450,
  "new_users": 50,
  "total_sessions": 600,
  "total_page_views": 8000,
  "total_events": 15000,
  "total_revenue": 12500.00,
  "total_conversions": 25,
  "average_session_duration_seconds": 180.5,
  "bounce_rate_percent": 15.5,
  "conversion_rate_percent": 4.2,
  "revenue_per_user": 8.33
}
```

## Usage

### Running the Example

```bash
# Start infrastructure (Kafka, TimescaleDB, Prometheus, Grafana)
cd deployments/docker
docker-compose up -d

# Run the analytics dashboard
cd examples/analytics-dashboard
go run main.go
```

### Sending Events

```bash
# Send a single event
curl -X POST http://localhost:8080/ingest \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "user-123",
    "session_id": "session-456",
    "action": "page_view",
    "page": "/products",
    "duration": 5000,
    "country": "US",
    "device": "mobile"
  }'
```

### Batch Events

```bash
curl -X POST http://localhost:8080/ingest/batch \
  -H "Content-Type: application/json" \
  -d '{
    "events": [
      {
        "user_id": "user-1",
        "action": "page_view",
        "page": "/home"
      },
      {
        "user_id": "user-2",
        "action": "purchase",
        "page": "/checkout",
        "metadata": {"amount": 99.99}
      }
    ]
  }'
```

## Custom Pipelines

### Building Your Own Analytics Pipeline

```go
package main

import (
    "github.com/therealutkarshpriyadarshi/gress/pkg/analytics"
    "github.com/therealutkarshpriyadarshi/gress/pkg/stream"
    "github.com/therealutkarshpriyadarshi/gress/pkg/window"
)

func createCustomPipeline() *stream.Pipeline {
    // Create operators
    behaviorTracker := analytics.BehaviorTracker()
    sessionAgg := analytics.SessionAggregator()
    kpiCalc := analytics.LiveKPICalculator(5 * time.Minute)

    // Create tumbling window (5 minutes)
    window := window.NewTumblingWindow(5 * time.Minute)

    return &stream.Pipeline{
        Source: yourSource,
        Operators: []stream.Operator{
            behaviorTracker,
            window,
            sessionAgg,
            kpiCalc,
        },
        Sink: yourSink,
    }
}
```

### Custom KPI Calculations

```go
// Define custom KPI
type CustomKPI struct {
    Name      string
    Value     float64
    Threshold float64
    Alert     bool
}

// Custom KPI operator
customKPI := stream.MapOperator(func(event *stream.Event) (*stream.Event, error) {
    var metrics analytics.KPIMetrics
    json.Unmarshal(event.Data, &metrics)

    // Calculate custom KPI
    customValue := calculateMyKPI(metrics)

    kpi := CustomKPI{
        Name:      "My Custom KPI",
        Value:     customValue,
        Threshold: 100.0,
        Alert:     customValue > 100.0,
    }

    data, _ := json.Marshal(kpi)
    event.Data = data
    return event, nil
})
```

## Metrics

The analytics dashboard exposes Prometheus metrics:

- `analytics_dashboard_events_processed_total` - Total events processed
- `analytics_dashboard_events_filtered_total` - Events filtered
- `analytics_dashboard_processing_latency` - Processing latency histogram
- `analytics_dashboard_kpi_calculation_duration` - KPI calculation time
- `analytics_dashboard_session_count` - Active session count
- `analytics_dashboard_user_count` - Active user count

Access metrics at: `http://localhost:9091/metrics`

## Grafana Dashboard

The pre-built Grafana dashboard includes:

1. **Overview Panel**
   - Active users (real-time gauge)
   - Conversion rate
   - Average session duration
   - Engagement score

2. **Trends Panel**
   - User actions over time (page views, clicks, conversions)
   - Session activity timeline
   - KPI trends with historical comparison

3. **Analytics Panel**
   - Top pages by views (bar chart)
   - Traffic by device type (pie chart)
   - Geographic distribution

4. **Performance Panel**
   - Processing latency (P50, P95, P99)
   - Throughput
   - Event rates

Access Grafana at: `http://localhost:3000` (default credentials: admin/admin)

## Configuration

Example configuration file `configs/analytics-dashboard.yaml`:

```yaml
application:
  name: analytics-dashboard
  environment: production

engine:
  buffer_size: 10000
  max_concurrency: 100
  checkpoint_interval: 30s
  watermark_interval: 5s

sources:
  kafka:
    brokers:
      - kafka-1:9092
      - kafka-2:9092
    topics:
      - user-events
    group_id: analytics-group

  http:
    listen_addr: ":8080"
    path: "/ingest"
    tls_enabled: false

sinks:
  kafka:
    brokers:
      - kafka-1:9092
    topics:
      user-behavior-enriched: {}
      live-kpis: {}

  timescaledb:
    connection_string: "postgresql://user:pass@timescale:5432/analytics"
    tables:
      - session_metrics
      - page_view_stats
      - kpi_metrics
    batch_size: 100
    flush_interval: 5s

metrics:
  enabled: true
  port: 9091
  namespace: analytics_dashboard
```

## Best Practices

### 1. Window Sizing
- **Real-time dashboards**: Use 1-5 minute tumbling windows
- **Trend analysis**: Use 1-hour sliding windows
- **Session analysis**: Use session windows with 5-30 minute gaps

### 2. State Management
- Use RocksDB for production (handles large state)
- Configure appropriate TTL for user profiles
- Monitor state size with metrics

### 3. Scalability
- Partition by user_id for parallel processing
- Use Kafka topic partitions matching parallelism
- Scale horizontally by adding more instances

### 4. Data Quality
- Validate events at ingestion
- Handle missing/malformed data gracefully
- Use dead letter queues for failed events

### 5. Performance Optimization
- Batch database writes
- Use appropriate buffer sizes
- Enable compression for Kafka
- Index TimescaleDB appropriately

## Troubleshooting

### High Latency
```bash
# Check processing latency
curl http://localhost:9091/metrics | grep processing_latency

# Increase concurrency
config.Engine.MaxConcurrency = 200

# Increase buffer size
config.Engine.BufferSize = 20000
```

### Memory Issues
```bash
# Monitor state size
curl http://localhost:9091/metrics | grep state_size

# Configure TTL for state
stateConfig.TTL = 24 * time.Hour

# Use incremental checkpoints
checkpointConfig.Incremental = true
```

### Missing Events
```bash
# Check Kafka consumer lag
kafka-consumer-groups --bootstrap-server localhost:9092 \
  --group analytics-group --describe

# Verify checkpoint recovery
tail -f logs/analytics-dashboard.log | grep checkpoint
```

## Advanced Features

### A/B Testing Analytics
```go
// Track A/B test performance
abTestKPI := analytics.ABTestKPICalculator("test-id-123")

// Analyze variant performance
// Includes statistical significance testing
```

### Funnel Analysis
```go
// Define conversion funnel
funnel := analytics.ConversionFunnel{
    Stages: []string{
        "landing_page",
        "product_view",
        "add_to_cart",
        "checkout",
        "purchase",
    },
}

// Track users through funnel
// Calculate drop-off rates at each stage
```

### Cohort Analysis
```go
// Group users by signup date
cohortAnalyzer := analytics.CohortAnalyzer{
    CohortDefinition: "signup_date",
    Metrics: []string{
        "retention_rate",
        "lifetime_value",
        "engagement_score",
    },
}
```

## Performance Benchmarks

- **Throughput**: 50,000+ events/second (single instance)
- **Latency (P99)**: < 50ms for enrichment
- **KPI Calculation**: < 100ms for 1-minute windows
- **State Size**: Supports 1M+ active sessions
- **Recovery Time**: < 10s from checkpoint

## Next Steps

1. **Integrate with your data sources**: Replace sample data generator with real events
2. **Customize KPIs**: Add domain-specific metrics
3. **Build custom dashboards**: Create visualizations for your use case
4. **Set up alerting**: Configure Grafana alerts for KPI thresholds
5. **Scale horizontally**: Deploy multiple instances with load balancing

## Related Documentation

- [Complex Event Processing](CEP_GUIDE.md)
- [Stream Processing Fundamentals](STREAM_PROCESSING.md)
- [State Management](STATE_MANAGEMENT.md)
- [Deployment Guide](DEPLOYMENT.md)

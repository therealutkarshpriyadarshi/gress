# Gress Quick Start Guide

Get up and running with Gress stream processing in 5 minutes!

## Installation

```bash
git clone https://github.com/therealutkarshpriyadarshi/gress.git
cd gress
```

## Option 1: Docker Compose (Recommended)

### Start the Full Stack

```bash
cd deployments/docker
docker-compose up -d
```

This starts:
- ✅ Kafka on `localhost:9092`
- ✅ TimescaleDB on `localhost:5432`
- ✅ Grafana on `localhost:3000`
- ✅ Prometheus on `localhost:9090`
- ✅ Gress App on `localhost:8080`

### Check Status

```bash
docker-compose ps
```

### View Logs

```bash
docker-compose logs -f gress-app
```

## Option 2: Local Development

### Prerequisites

- Go 1.21+
- (Optional) Kafka, TimescaleDB for full features

### Run the Ride-Sharing Example

```bash
cd examples/rideshare
go run main.go
```

The example starts HTTP and WebSocket servers for event ingestion.

## Sending Events

### HTTP POST

```bash
curl -X POST http://localhost:8080/ride-requests \
  -H "Content-Type: application/json" \
  -d '{
    "request_id": "req-123",
    "user_id": "user-456",
    "location": {
      "latitude": 37.7749,
      "longitude": -122.4194,
      "area": "downtown"
    },
    "timestamp": "2025-01-15T10:30:00Z"
  }'
```

### Batch Events

```bash
curl -X POST http://localhost:8080/ride-requests \
  -H "Content-Type: application/json" \
  -d '{
    "events": [
      {
        "request_id": "req-1",
        "user_id": "user-1",
        "location": {"latitude": 37.77, "longitude": -122.42, "area": "downtown"},
        "timestamp": "2025-01-15T10:30:00Z"
      },
      {
        "request_id": "req-2",
        "user_id": "user-2",
        "location": {"latitude": 37.78, "longitude": -122.41, "area": "airport"},
        "timestamp": "2025-01-15T10:30:05Z"
      }
    ]
  }'
```

## Viewing Results

### Console Output

Watch the terminal for pricing updates:

```
=== Pricing Update ===
{
  "area": "downtown",
  "surge_multiplier": 1.5,
  "demand_supply_ratio": 2.3,
  "ride_requests": 23,
  "available_drivers": 10
}
```

### Grafana Dashboards

1. Open http://localhost:3000
2. Login: `admin` / `admin`
3. Navigate to "Gress Stream Processing Overview"

### Query TimescaleDB

```bash
docker exec -it gress-timescaledb psql -U gress -d gress

SELECT area, surge_multiplier, time
FROM pricing_updates
ORDER BY time DESC
LIMIT 10;
```

### Kafka Topics

```bash
# List topics
docker exec -it gress-kafka kafka-topics --list --bootstrap-server localhost:9092

# Consume messages
docker exec -it gress-kafka kafka-console-consumer \
  --bootstrap-server localhost:9092 \
  --topic events \
  --from-beginning
```

## Building Your First Pipeline

### 1. Create a Simple Stream Processor

```go
package main

import (
    "time"
    "github.com/therealutkarshpriyadarshi/gress/pkg/stream"
    "github.com/therealutkarshpriyadarshi/gress/pkg/ingestion"
    "go.uber.org/zap"
)

func main() {
    logger, _ := zap.NewProduction()

    // Create engine
    config := stream.DefaultEngineConfig()
    engine := stream.NewEngine(config, logger)

    // Add HTTP source
    httpSource := ingestion.NewHTTPSource(":8080", "/events", logger)
    engine.AddSource(httpSource)

    // Add filter
    filter := stream.NewFilterOperator(func(e *stream.Event) bool {
        // Only process high-priority events
        return e.Headers["priority"] == "high"
    })
    engine.AddOperator(filter)

    // Add map transformation
    mapper := stream.NewMapOperator(func(e *stream.Event) (*stream.Event, error) {
        // Enrich event
        e.Headers["processed_at"] = time.Now().Format(time.RFC3339)
        return e, nil
    })
    engine.AddOperator(mapper)

    // Start processing
    engine.Start()

    // Wait for shutdown
    select {}
}
```

### 2. Add Windowed Aggregation

```go
import "github.com/therealutkarshpriyadarshi/gress/pkg/window"

// 5-minute tumbling window
windowAssigner := window.NewTumblingWindow(5 * time.Minute)

// Count events per key
aggregator := stream.NewAggregateOperator(func(acc interface{}, e *stream.Event) (interface{}, error) {
    count := 0
    if acc != nil {
        count = acc.(int)
    }
    return count + 1, nil
})

engine.AddOperator(aggregator)
```

### 3. Write to TimescaleDB

```go
import "github.com/therealutkarshpriyadarshi/gress/pkg/sink"

timescaleConfig := sink.TimescaleConfig{
    Host:     "localhost",
    Port:     5432,
    Database: "gress",
    User:     "gress",
    Password: "gress123",
    Table:    "events",
}

sink, _ := sink.NewTimescaleSink(timescaleConfig, logger)
engine.AddSink(sink)
```

## Next Steps

1. **Explore Examples**: Check out `examples/` for more use cases
2. **Read Architecture**: See [ARCHITECTURE.md](ARCHITECTURE.md) for design details
3. **Configure**: Customize [engine configuration](README.md#configuration)
4. **Monitor**: Set up Prometheus alerts and Grafana dashboards
5. **Scale**: Deploy to Kubernetes or multi-node setup

## Common Issues

### Port Already in Use

```bash
# Kill existing process
lsof -ti:8080 | xargs kill -9
```

### Docker Compose Issues

```bash
# Clean restart
docker-compose down -v
docker-compose up -d
```

### Kafka Connection Errors

```bash
# Check Kafka is running
docker exec -it gress-kafka kafka-broker-api-versions --bootstrap-server localhost:9092
```

## Getting Help

- 📖 [Full Documentation](README.md)
- 🏗️ [Architecture Guide](ARCHITECTURE.md)
- 💬 [GitHub Issues](https://github.com/therealutkarshpriyadarshi/gress/issues)
- 📧 Contact: your-email@example.com

Happy Streaming! 🚀

# Ride-Sharing Dynamic Pricing Example

This example demonstrates real-time stream processing for dynamic pricing in a ride-sharing application, similar to how Uber calculates surge pricing.

## Features

- **Real-time Demand/Supply Tracking**: Monitors ride requests and available drivers per area
- **Windowed Aggregations**: 5-minute tumbling windows for calculating metrics
- **Dynamic Surge Pricing**: Exponential pricing multiplier based on demand/supply ratio
- **Multi-Source Ingestion**: HTTP for ride requests, WebSockets for driver locations
- **Event-Time Processing**: Uses event timestamps for accurate windowing

## Architecture

```
Ride Requests (HTTP)     Driver Locations (WebSocket)
        │                           │
        └─────────┬─────────────────┘
                  │
           Ingestion Layer
                  │
          Filter Active Areas
                  │
           Key By Area (Partition)
                  │
         Assign Event Timestamps
                  │
      5-min Tumbling Window Aggregation
                  │
      Calculate Demand/Supply Ratio
                  │
      Compute Surge Multiplier
                  │
         Pricing Updates (Output)
```

## Surge Pricing Algorithm

The system calculates surge multiplier based on demand/supply ratio:

- **Ratio < 1.0**: No surge (1.0x) - more drivers than requests
- **Ratio = 2.0**: Moderate surge (1.5x)
- **Ratio = 4.0**: High surge (2.0x)
- **Ratio >= 8.0**: Maximum surge (3.0x - capped)

Formula: `surge = 1.0 + log2(ratio) * 0.5`

## Running the Example

### 1. Start the Stream Processing Application

```bash
cd examples/rideshare
go run main.go simulator.go
```

This starts:
- HTTP server on `:8080` for ride requests
- WebSocket server on `:8081` for driver locations

### 2. Send Test Data

#### Send a Ride Request (HTTP)

```bash
curl -X POST http://localhost:8080/ride-requests \
  -H "Content-Type: application/json" \
  -d '{
    "request_id": "req-12345",
    "user_id": "user-789",
    "location": {
      "latitude": 37.7749,
      "longitude": -122.4194,
      "area": "downtown"
    },
    "timestamp": "'$(date -u +%Y-%m-%dT%H:%M:%SZ)'"
  }'
```

#### Send Batch Requests

```bash
curl -X POST http://localhost:8080/ride-requests \
  -H "Content-Type: application/json" \
  -d '{
    "events": [
      {
        "request_id": "req-1",
        "user_id": "user-1",
        "location": {"latitude": 37.77, "longitude": -122.42, "area": "downtown"},
        "timestamp": "'$(date -u +%Y-%m-%dT%H:%M:%SZ)'"
      },
      {
        "request_id": "req-2",
        "user_id": "user-2",
        "location": {"latitude": 37.78, "longitude": -122.41, "area": "airport"},
        "timestamp": "'$(date -u +%Y-%m-%dT%H:%M:%SZ)'"
      }
    ]
  }'
```

### 3. Run the Simulator

The simulator generates realistic traffic patterns:

```bash
# In simulator.go, uncomment the main function and run:
go run simulator.go
```

## Expected Output

```
=== Pricing Update ===
{
  "area": "downtown",
  "surge_multiplier": 1.5,
  "demand_supply_ratio": 2.3,
  "ride_requests": 23,
  "available_drivers": 10,
  "timestamp": "2025-01-15T10:30:00Z"
}

=== Pricing Update ===
{
  "area": "airport",
  "surge_multiplier": 2.0,
  "demand_supply_ratio": 4.1,
  "ride_requests": 41,
  "available_drivers": 10,
  "timestamp": "2025-01-15T10:30:00Z"
}
```

## Real-World Extensions

1. **Machine Learning Integration**: Predict future demand and pre-adjust pricing
2. **Geographic Clustering**: Use geohashing for finer-grained area definitions
3. **Weather Integration**: Factor in weather conditions affecting demand
4. **Historical Analysis**: Track pricing trends over time in TimescaleDB
5. **Driver Incentives**: Calculate bonuses to attract drivers to high-demand areas
6. **Ride Matching**: Optimize driver-rider pairing based on proximity
7. **Price Optimization**: A/B test different surge algorithms
8. **Fraud Detection**: Identify suspicious patterns in ride requests

## Monitoring

Key metrics to monitor:

- Events processed per second
- Average latency (P50, P95, P99)
- Window firing rates
- Backpressure events
- Surge multiplier distribution
- Area-wise demand patterns

## Scaling

The system can be scaled by:

1. **Horizontal Scaling**: Add more processing nodes
2. **Kafka Partitioning**: Partition by area for parallel processing
3. **State Sharding**: Distribute state across nodes by area
4. **Load Balancing**: Use multiple HTTP/WebSocket instances behind a load balancer

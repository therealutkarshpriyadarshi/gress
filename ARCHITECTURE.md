# Stream Processing System Architecture

## Overview

**Gress** is a high-performance, distributed stream processing system designed for real-time data ingestion, transformation, and analysis with exactly-once semantics.

## Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                        DATA SOURCES                              │
├─────────────────────────────────────────────────────────────────┤
│  Kafka Topics  │  WebSocket Streams  │  HTTP APIs  │  NATS      │
└────────┬────────────────┬───────────────────┬──────────────┬─────┘
         │                │                   │              │
         └────────────────┴───────────────────┴──────────────┘
                                  │
                    ┌─────────────▼──────────────┐
                    │   Ingestion Layer          │
                    │  - Source Connectors       │
                    │  - Schema Registry         │
                    │  - Data Validation         │
                    └─────────────┬──────────────┘
                                  │
                    ┌─────────────▼──────────────┐
                    │   Stream Engine Core       │
                    │  - Event Router            │
                    │  - Watermark Generator     │
                    │  - Backpressure Manager    │
                    └─────────────┬──────────────┘
                                  │
         ┌────────────────────────┼────────────────────────┐
         │                        │                        │
    ┌────▼─────┐          ┌──────▼──────┐         ┌──────▼──────┐
    │Stateless │          │  Stateful   │         │   Window    │
    │Transform │          │ Aggregation │         │  Operations │
    │          │          │             │         │             │
    │- Map     │          │- State      │         │- Tumbling   │
    │- Filter  │          │  Backend    │         │- Sliding    │
    │- FlatMap │          │- RocksDB    │         │- Session    │
    └────┬─────┘          └──────┬──────┘         └──────┬──────┘
         │                       │                       │
         └───────────────────────┼───────────────────────┘
                                 │
                    ┌────────────▼─────────────┐
                    │  Checkpoint Manager      │
                    │  - Exactly-Once          │
                    │  - State Snapshots       │
                    │  - Recovery              │
                    └────────────┬─────────────┘
                                 │
         ┌───────────────────────┼───────────────────────┐
         │                       │                       │
    ┌────▼─────┐         ┌──────▼──────┐        ┌──────▼──────┐
    │TimescaleDB│         │  Kafka      │        │   Custom    │
    │Time-Series│         │  Topics     │        │   Sinks     │
    └───────────┘         └─────────────┘        └─────────────┘
         │
    ┌────▼─────┐
    │ Grafana  │
    │Dashboard │
    └──────────┘
```

## Core Components

### 1. Ingestion Layer
- **Multi-Source Support**: Kafka, NATS JetStream, WebSockets, HTTP
- **Schema Validation**: Protobuf/JSON schema enforcement
- **Rate Limiting**: Per-source rate limits and quotas
- **Backpressure Propagation**: Signals to upstream sources

### 2. Stream Engine Core
- **Event-Driven Architecture**: Async message passing with Go channels
- **Watermark System**: Event-time vs processing-time tracking
- **Partition Management**: Dynamic partition assignment and rebalancing
- **Metrics Collection**: Latency, throughput, lag monitoring

### 3. Processing Operators

#### Stateless Transformations
- **Map**: 1-to-1 transformation
- **Filter**: Predicate-based filtering
- **FlatMap**: 1-to-N transformation

#### Stateful Operations
- **Aggregations**: Count, sum, avg, min, max
- **State Backend**: RocksDB for persistent state
- **State Snapshots**: Incremental checkpointing

#### Windowing
- **Tumbling Windows**: Fixed non-overlapping windows
- **Sliding Windows**: Overlapping time windows
- **Session Windows**: Gap-based windows
- **Late Data Handling**: Configurable lateness tolerance

### 4. Exactly-Once Semantics
- **Two-Phase Commit**: Coordinated checkpointing
- **Idempotent Writes**: Deduplication on output
- **State Consistency**: Atomic state updates
- **Failure Recovery**: Checkpoint-based restart

### 5. Complex Event Processing
- **Pattern Matching**: Sequence detection
- **Event Correlation**: Cross-stream joins
- **Temporal Logic**: Time-based conditions

### 6. Storage Layer
- **TimescaleDB**: Time-series data storage
- **RocksDB**: Local state backend
- **Kafka**: Event log and replay

## Data Flow

1. **Ingestion**: Events arrive from multiple sources
2. **Deserialization**: Events converted to internal format
3. **Watermark Assignment**: Event-time extracted and watermarks generated
4. **Transformation**: Stateless operations applied
5. **Windowing**: Events assigned to time windows
6. **Aggregation**: Stateful computations performed
7. **Checkpoint**: State snapshots created periodically
8. **Output**: Results written to sinks with exactly-once guarantees

## Key Design Decisions

### Go for Performance
- Native concurrency with goroutines
- Low GC overhead for high-throughput
- Efficient memory management
- Strong typing for reliability

### Watermark Strategy
- **Periodic Watermarks**: Generated every N events or T seconds
- **Idle Detection**: Handle slow or stopped partitions
- **Skew Handling**: Per-partition watermark tracking

### State Management
- **Pluggable Backends**:
  - **RocksDB**: Persistent state for production (>RAM state size)
  - **Memory**: In-memory state for development/testing
- **Incremental Checkpoints**: Only changed state persisted
- **State TTL**: Automatic cleanup with configurable expiry (1h-24h typical)
- **Performance**: <10ms P99 latency for Get/Put operations
- **Snapshot/Restore**: Full state recovery from checkpoints
- **Configurable**: Fine-tuned buffer sizes, cache, and compaction

### Backpressure
- **Token Bucket**: Rate limiting at ingestion
- **Channel Buffering**: Bounded buffers with blocking
- **Dynamic Throttling**: Adaptive rate adjustment
- **Spillover**: Temporary disk-based buffering

## Fault Tolerance

1. **Checkpointing**: Periodic consistent snapshots
2. **Replay**: Kafka offset management for replay
3. **State Recovery**: Restore from last checkpoint
4. **Partition Reassignment**: Handle node failures

## Performance Targets

- **Throughput**: 1M+ events/second per node
- **Latency**: P99 < 100ms for stateless ops
- **State Operations**: P99 < 10ms for Get/Put (RocksDB)
- **State Size**: Support for 100GB+ state per node (persistent backend)
- **Availability**: 99.9% uptime with proper deployment
- **Recovery**: < 30s from checkpoint restore

## Real-World Use Cases

### 1. Ride-Sharing Dynamic Pricing
```
Ride Requests → Filter (active areas) → Window (5min tumbling)
→ Aggregate (demand/supply ratio) → Map (calculate surge multiplier)
→ Output (pricing updates)
```

### 2. Real-Time Analytics Dashboard
```
User Events → FlatMap (expand sessions) → Window (1min sliding)
→ Aggregate (count by action type) → TimescaleDB
→ Grafana (live charts)
```

### 3. Fraud Detection
```
Transactions → Pattern Match (suspicious sequences)
→ Join (with user profile stream) → Filter (risk score > threshold)
→ Alert (fraud team)
```

### 4. IoT Sensor Monitoring
```
Sensor Data → Map (normalize units) → Window (10sec tumbling)
→ Aggregate (avg temperature) → Filter (anomaly detection)
→ Alert + TimescaleDB
```

## Technology Stack

- **Core Engine**: Go 1.21+
- **Message Queue**: Kafka / NATS JetStream
- **State Store**: RocksDB (via gorocksdb)
- **Time-Series DB**: TimescaleDB (PostgreSQL extension)
- **Monitoring**: Prometheus + Grafana
- **Serialization**: Protocol Buffers, JSON
- **Testing**: Go testing, testcontainers

## Scalability

- **Horizontal Scaling**: Add nodes to consumer group
- **Partition-Based Parallelism**: Kafka partition = parallelism unit
- **State Sharding**: State partitioned by key
- **Independent Scaling**: Scale ingestion, processing, storage separately

## Future Enhancements

- SQL query interface for streams
- Machine learning model integration
- Multi-datacenter deployment
- Enhanced CEP with neural pattern matching
- Auto-scaling based on load

# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Planned
- RocksDB state backend
- Complete Prometheus metrics implementation
- Complex Event Processing (CEP)
- Stream joins
- SQL interface

## [1.0.0-alpha] - 2025-01-15

### Added

#### Core Engine
- High-performance stream processing engine with async architecture
- Configurable concurrency with goroutine worker pools
- Event-driven message passing using Go channels
- Real-time metrics collection (events processed, latency, backpressure)
- Graceful shutdown with resource cleanup

#### Ingestion Layer
- **Kafka Source**: Multi-topic consumer with partition tracking
- **HTTP Source**: REST API for single and batch event ingestion
- **WebSocket Source**: Real-time bidirectional streaming with client management
- Source abstraction for easy extension

#### Stream Operators
- **Map**: 1-to-1 event transformations
- **Filter**: Predicate-based event filtering
- **FlatMap**: 1-to-N event transformations
- **KeyBy**: Event partitioning by key selector
- **Aggregate**: Stateful aggregation with custom functions
- **Reduce**: Value combination operator
- **Branch**: Split streams based on predicates
- **Union**: Merge multiple streams
- **TimestampAssigner**: Event-time extraction

#### Windowing System
- **Tumbling Windows**: Fixed non-overlapping time windows
- **Sliding Windows**: Overlapping windows with configurable slide
- **Session Windows**: Gap-based dynamic windows
- **Event-time triggers**: Watermark-based window firing
- **Processing-time triggers**: System time-based firing
- **Count triggers**: Count-based window firing
- Window state management with per-key isolation

#### Fault Tolerance
- **Checkpoint Manager**: Periodic state snapshots with configurable intervals
- **Coordinated Checkpointing**: Two-phase commit protocol
- **State Persistence**: Disk-based checkpoint storage
- **Recovery**: Restore from last successful checkpoint
- **Checkpoint Retention**: Keep last N checkpoints

#### Watermark System
- **Per-partition Watermarks**: Independent time progress tracking
- **Idle Partition Handling**: Automatic watermark advancement
- **Global Watermark**: Minimum across all partitions
- **Late Event Detection**: Configurable lateness tolerance
- **Side Outputs**: Route late events separately

#### Backpressure Management
- **Channel Buffering**: Bounded buffers with blocking
- **Utilization Tracking**: Real-time buffer monitoring
- **Backpressure Metrics**: Event counting and logging
- **Dynamic Throttling**: Configurable thresholds

#### Sinks
- **TimescaleDB Sink**:
  - Batch writing for performance
  - Hypertable creation and management
  - Continuous aggregates (1-minute buckets)
  - Retention policies (30-day default)
  - JSONB indexing
- **Kafka Sink**:
  - Idempotent producer
  - Delivery guarantees
  - Header propagation

#### Examples
- **Ride-Sharing Dynamic Pricing**:
  - Multi-source ingestion (HTTP + WebSocket)
  - Geographic partitioning by area
  - 5-minute tumbling windows
  - Demand/supply ratio calculation
  - Surge multiplier algorithm: `1.0 + log2(ratio) * 0.5`
  - Real-time pricing updates

#### Infrastructure
- **Docker Compose Stack**:
  - Kafka + Zookeeper
  - TimescaleDB with hypertables
  - Prometheus for metrics
  - Grafana for dashboards
  - NATS JetStream
  - Kafka UI for management

#### Monitoring & Observability
- Structured logging with Zap
- Metrics structure for Prometheus
- Grafana dashboard templates:
  - Events processed (rate)
  - Processing latency (P95, P99)
  - Backpressure events
  - Checkpoint status
  - Application-specific metrics

#### Documentation
- Comprehensive README with quick start
- ARCHITECTURE.md with system design
- QUICKSTART.md for 5-minute setup
- Example-specific documentation
- Makefile for common operations
- API documentation with godoc comments

#### Testing
- Unit tests for core operators
- Benchmark tests for performance validation
- Test coverage tracking
- Example test patterns

#### Development Tools
- **Makefile** with targets:
  - `make build`: Build binary
  - `make test`: Run tests
  - `make bench`: Run benchmarks
  - `make docker-up`: Start Docker stack
  - `make run-example`: Run examples
  - `make fmt`: Format code
  - `make lint`: Run linter

### Technical Details

#### Performance Characteristics
- Target throughput: 1M+ events/second per node
- Target latency: P99 < 100ms (stateful), < 50ms (stateless)
- State size: Support for in-memory state
- Concurrency: Configurable worker pool (default 100)
- Windowing: Support for 10K+ concurrent windows

#### Dependencies
- Go 1.21+
- Kafka (Confluent client v2.3.0)
- NATS (v1.31.0)
- Gorilla WebSocket (v1.5.1)
- PostgreSQL/TimescaleDB driver (lib/pq v1.10.9)
- Prometheus client (v1.17.0)
- Zap logging (v1.26.0)

### Known Limitations
- State backend is in-memory only (RocksDB planned for v1.1.0)
- No Complex Event Processing patterns yet
- No stream joins
- Limited to single-node deployment (multi-node planned)
- Metrics endpoint not yet exposed

### Breaking Changes
N/A - Initial release

---

## Version History

- **v1.0.0-alpha**: Initial alpha release with core features

---

## Contributors

- Initial implementation: @therealutkarshpriyadarshi

---

## Links

- [GitHub Repository](https://github.com/therealutkarshpriyadarshi/gress)
- [Documentation](README.md)
- [Architecture](ARCHITECTURE.md)
- [Roadmap](ROADMAP.md)

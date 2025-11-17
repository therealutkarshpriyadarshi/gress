# State Backend

The state backend package provides persistent and in-memory state storage for the stream processing engine, enabling stateful operations with fault tolerance and recovery capabilities.

## Features

- **Pluggable State Backends**: Support for multiple storage backends through a common interface
- **RocksDB Backend**: High-performance persistent state storage with configurable options
- **Memory Backend**: Fast in-memory storage for testing and small workloads
- **TTL Support**: Automatic expiration of state entries based on time-to-live
- **Snapshot/Restore**: Full state snapshots for checkpoint and recovery
- **Incremental Checkpoints**: Efficient checkpointing by tracking only changed state
- **Performance**: Optimized for <10ms P99 latency on state operations
- **Metrics**: Built-in operation counters for monitoring

## State Backends

### RocksDB State Backend

Persistent state storage using RocksDB, suitable for production workloads with state larger than available RAM.

**Features:**
- Persistent storage on disk
- Configurable caching and compaction
- Bloom filters for fast lookups
- Snappy compression
- Hard-linked checkpoints for incremental backups
- TTL support with automatic cleanup

**Use cases:**
- Production deployments
- Large state (> available RAM)
- Stateful windowed aggregations
- Long-running streaming jobs

### Memory State Backend

In-memory state storage with optional TTL support, suitable for testing and smaller workloads.

**Features:**
- Fast in-memory operations
- TTL support with background cleanup
- Simple snapshot/restore
- Metrics tracking

**Use cases:**
- Testing and development
- Small state that fits in memory
- Ephemeral processing
- Benchmarking

## Usage

### Basic Usage

```go
import (
    "github.com/therealutkarshpriyadarshi/gress/pkg/state"
    "go.uber.org/zap"
)

// Create RocksDB backend
logger, _ := zap.NewProduction()
config := state.DefaultRocksDBConfig("./state-data")
backend, err := state.NewRocksDBStateBackend(config, logger)
if err != nil {
    panic(err)
}
defer backend.Close()

// Put key-value
err = backend.Put("user:123", []byte(`{"name": "Alice", "balance": 1000}`))

// Get value
value, err := backend.Get("user:123")

// Delete key
err = backend.Delete("user:123")
```

### Using with Stream Engine

```go
import (
    "github.com/therealutkarshpriyadarshi/gress/pkg/state"
    "github.com/therealutkarshpriyadarshi/gress/pkg/stream"
    "github.com/therealutkarshpriyadarshi/gress/pkg/checkpoint"
)

// Create state backend
config := state.DefaultRocksDBConfig("./state")
stateBackend, _ := state.NewRocksDBStateBackend(config, logger)

// Register with checkpoint manager
checkpointMgr := checkpoint.NewManager(30*time.Second, "./checkpoints", logger)
checkpointMgr.RegisterStateBackend(stateBackend)
checkpointMgr.EnableIncrementalCheckpoints(true)

// Create engine with state backend
engineConfig := stream.DefaultEngineConfig()
engine := stream.NewEngine(engineConfig, logger)

// Use state in operators
ctx := &stream.ProcessingContext{
    State: stateBackend,
}
```

### TTL Support

```go
// Enable TTL
config := state.DefaultRocksDBConfig("./state")
config.TTLEnabled = true
config.DefaultTTL = 1 * time.Hour

backend, _ := state.NewRocksDBStateBackend(config, logger)

// Put with custom TTL
backend.PutWithTTL("session:abc", sessionData, 30*time.Minute)

// Put with default TTL
backend.Put("cache:xyz", cacheData) // Uses 1 hour from config
```

### Snapshots and Checkpoints

```go
// Create full snapshot
snapshot, err := backend.Snapshot()

// Save snapshot
os.WriteFile("state-snapshot.bin", snapshot, 0644)

// Restore from snapshot
snapshotData, _ := os.ReadFile("state-snapshot.bin")
err = backend.Restore(snapshotData)

// RocksDB hard-linked checkpoint (incremental)
err = backend.CreateCheckpoint("./checkpoints/cp-1")
```

### Advanced Configuration

```go
config := &state.RocksDBConfig{
    Path:                "./state",
    CreateIfMissing:     true,
    WriteBufferSize:     128 * 1024 * 1024, // 128MB
    MaxWriteBufferNum:   4,
    MaxOpenFiles:        2000,
    BlockSize:           16 * 1024, // 16KB blocks
    BlockCacheSize:      512 * 1024 * 1024, // 512MB cache
    BloomFilterBits:     10,
    Compression:         gorocksdb.SnappyCompression,
    EnableStatistics:    true,
    TTLEnabled:          true,
    DefaultTTL:          2 * time.Hour,
    EnableIncrementalCP: true,
}

backend, err := state.NewRocksDBStateBackend(config, logger)
```

### Metrics and Monitoring

```go
// Get operation metrics
metrics := backend.GetMetrics()
fmt.Printf("Gets: %d\n", metrics["gets"])
fmt.Printf("Puts: %d\n", metrics["puts"])
fmt.Printf("Deletes: %d\n", metrics["deletes"])

// Get RocksDB properties
numKeys := backend.GetProperty("rocksdb.estimate-num-keys")
memUsage := backend.GetProperty("rocksdb.estimate-table-readers-mem")
```

## Performance

The state backends are optimized for high-performance stream processing:

### Target Metrics
- **P99 Latency**: < 10ms for Get/Put operations
- **Throughput**: > 100K ops/sec (memory backend)
- **Throughput**: > 50K ops/sec (RocksDB backend)

### Benchmarks

Run benchmarks to measure performance:

```bash
# Run all benchmarks
go test -bench=. -benchmem ./pkg/state/

# Run specific backend benchmarks
go test -bench=BenchmarkRocksDB -benchmem ./pkg/state/

# Run P99 latency tests
go test -run=P99Latency ./pkg/state/ -v
```

Example results:
```
BenchmarkMemoryBackendPut-8        5000000    250 ns/op    128 B/op    2 allocs/op
BenchmarkMemoryBackendGet-8       10000000    150 ns/op     64 B/op    1 allocs/op
BenchmarkRocksDBBackendPut-8       1000000   1500 ns/op    256 B/op    4 allocs/op
BenchmarkRocksDBBackendGet-8       2000000    800 ns/op    128 B/op    2 allocs/op

P99 Latencies:
  Memory Get:  P99=0.5ms
  Memory Put:  P99=0.8ms
  RocksDB Get: P99=5.2ms
  RocksDB Put: P99=8.7ms
```

## State Backend Interface

All state backends implement the `stream.StateBackend` interface:

```go
type StateBackend interface {
    Get(key string) ([]byte, error)
    Put(key string, value []byte) error
    Delete(key string) error
    Snapshot() ([]byte, error)
    Restore(data []byte) error
    Close() error
}
```

Additional methods available on specific backends:

### RocksDBStateBackend
```go
PutWithTTL(key string, value []byte, ttl time.Duration) error
CreateCheckpoint(path string) error
GetProperty(property string) string
GetMetrics() map[string]int64
Flush() error
Compact() error
```

### MemoryStateBackend
```go
PutWithTTL(key string, value []byte, ttl time.Duration) error
GetMetrics() map[string]int64
Size() int
Clear() error
```

## Best Practices

### 1. Choose the Right Backend

- **Use RocksDB for**:
  - Production deployments
  - Large state that may exceed RAM
  - Long-running jobs requiring persistence
  - Exactly-once processing guarantees

- **Use Memory for**:
  - Development and testing
  - Small state (<1GB)
  - Stateless or ephemeral processing
  - Benchmarking

### 2. Configure for Your Workload

```go
// For write-heavy workloads
config.WriteBufferSize = 256 * 1024 * 1024  // Larger buffers
config.MaxWriteBufferNum = 6                 // More buffers

// For read-heavy workloads
config.BlockCacheSize = 1024 * 1024 * 1024  // Larger cache
config.BloomFilterBits = 12                  // Better filters
```

### 3. Enable Incremental Checkpoints

```go
config.EnableIncrementalCP = true
checkpointMgr.EnableIncrementalCheckpoints(true)
```

### 4. Monitor State Size

```go
// Periodic monitoring
ticker := time.NewTicker(1 * time.Minute)
go func() {
    for range ticker.C {
        numKeys := backend.GetProperty("rocksdb.estimate-num-keys")
        memUsage := backend.GetProperty("rocksdb.cur-size-all-mem-tables")
        logger.Info("State metrics",
            zap.String("num_keys", numKeys),
            zap.String("mem_usage", memUsage))
    }
}()
```

### 5. Use TTL for Session State

```go
config.TTLEnabled = true
config.DefaultTTL = 24 * time.Hour

// Session data auto-expires
backend.PutWithTTL("session:"+sessionID, data, 1*time.Hour)
```

## Examples

See the [examples directory](../../examples/) for complete applications:

- `examples/state-backend/` - RocksDB state backend usage
- `examples/stateful-processing/` - Stateful stream processing
- `examples/checkpointing/` - Checkpoint and recovery

## Troubleshooting

### RocksDB Dependency Issues

If you encounter RocksDB compilation errors:

```bash
# Install RocksDB system library
# Ubuntu/Debian
sudo apt-get install librocksdb-dev

# macOS
brew install rocksdb

# Or build from source
git clone https://github.com/facebook/rocksdb.git
cd rocksdb
make shared_lib
sudo make install
```

### Performance Issues

1. **Check metrics**: Use `GetMetrics()` to identify bottlenecks
2. **Tune configuration**: Adjust buffer sizes and cache
3. **Enable statistics**: Set `EnableStatistics: true`
4. **Run benchmarks**: Compare with expected performance

### State Recovery Fails

1. **Check checkpoint directory**: Ensure it exists and is readable
2. **Verify state format**: Ensure snapshot format matches
3. **Check disk space**: Ensure sufficient space for restoration
4. **Review logs**: Look for detailed error messages

## Architecture

The state backend integrates with:

- **Stream Engine**: Provides state to operators via `ProcessingContext`
- **Checkpoint Manager**: Creates periodic snapshots for fault tolerance
- **Operators**: Use state for aggregations, joins, and windowing
- **Recovery**: Restores state from checkpoints on failure

```
┌─────────────────────────────────────────────┐
│          Stream Processing Engine           │
└──────────────────┬──────────────────────────┘
                   │
         ┌─────────┴─────────┐
         │ ProcessingContext │
         │   state: Backend  │
         └─────────┬─────────┘
                   │
    ┌──────────────┴──────────────┐
    │                             │
┌───▼────────┐          ┌────────▼────┐
│  RocksDB   │          │   Memory    │
│  Backend   │          │   Backend   │
└─────┬──────┘          └──────┬──────┘
      │                        │
      │   Checkpoint Manager   │
      └────────────┬───────────┘
                   │
         ┌─────────▼─────────┐
         │   Disk Storage    │
         │  (checkpoints/)   │
         └───────────────────┘
```

## Contributing

Contributions are welcome! Please see [CONTRIBUTING.md](../../CONTRIBUTING.md) for guidelines.

## License

See [LICENSE](../../LICENSE) for details.

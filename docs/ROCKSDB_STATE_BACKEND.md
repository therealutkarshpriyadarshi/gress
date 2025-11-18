# RocksDB State Backend

## Overview

The RocksDB state backend provides persistent, production-grade state management for Gress stream processing applications. It's optimized for:

- **<10ms P99 latency** for state operations
- **State larger than RAM** with efficient disk-based storage
- **High throughput** with batch operations
- **Production reliability** with checkpointing and recovery

## Quick Start

### Basic Usage

```go
import (
    "github.com/therealutkarshpriyadarshi/gress/pkg/state"
    "go.uber.org/zap"
)

logger, _ := zap.NewProduction()

// Create backend with low-latency preset
config := state.LowLatencyConfig("/var/lib/gress/state")
backend, err := state.NewRocksDBStateBackend(config, logger)
if err != nil {
    log.Fatal(err)
}
defer backend.Close()

// Use the backend
backend.Put("key", []byte("value"))
value, _ := backend.Get("key")
```

## Performance Presets

Gress provides four optimized presets for different use cases:

### 1. Low-Latency Preset (Recommended for Production)

**Target**: <10ms P99 latency

**Use Cases**:
- Real-time stream processing
- Interactive applications
- Low-latency requirements

```go
config := state.LowLatencyConfig("/var/lib/gress/state")
```

**Configuration**:
- Write Buffer: 128 MB
- Block Cache: 512 MB
- Background Jobs: 8
- Memory-mapped reads enabled
- Aggressive compaction settings

**Performance**:
- P99 Get Latency: ~5-8ms
- P99 Put Latency: ~8-10ms
- Throughput: 50K-100K ops/sec

### 2. High-Throughput Preset

**Target**: Maximum operations per second

**Use Cases**:
- Batch processing
- High-volume ingestion
- Analytics workloads

```go
config := state.HighThroughputConfig("/var/lib/gress/state")
```

**Configuration**:
- Write Buffer: 256 MB
- Block Cache: 1 GB
- Background Jobs: 16
- Direct I/O enabled
- Relaxed compaction

**Performance**:
- Throughput: 100K-200K ops/sec
- P99 Latency: ~10-15ms

### 3. Large-State Preset

**Target**: State size > available RAM

**Use Cases**:
- Large stateful aggregations
- Long-term session state
- State exceeding memory capacity

```go
config := state.LargeStateConfig("/var/lib/gress/state")
```

**Configuration**:
- Write Buffer: 256 MB
- Block Cache: 2 GB (critical for large state)
- ZSTD Compression
- Background Jobs: 12
- Optimized for disk access

**Performance**:
- Supports TBs of state
- Throughput: 50K-100K ops/sec
- Compression ratio: 3-5x

### 4. Balanced Preset (Default)

**Target**: General-purpose use

```go
config := state.DefaultRocksDBConfig("/var/lib/gress/state")
```

## Batch Operations

Batch operations provide significantly better throughput than individual operations:

### Batch Put

```go
entries := map[string][]byte{
    "key1": []byte("value1"),
    "key2": []byte("value2"),
    "key3": []byte("value3"),
}
backend.BatchPut(entries)
```

**Performance**: ~10-100x faster than individual Puts for large batches

### Batch Get

```go
keys := []string{"key1", "key2", "key3"}
results, err := backend.BatchGet(keys)
// results is map[string][]byte
```

### Batch Delete

```go
keys := []string{"key1", "key2", "key3"}
backend.BatchDelete(keys)
```

## Advanced Configuration

### Custom Configuration

```go
config := &state.RocksDBConfig{
    Path:                "/var/lib/gress/state",
    Preset:              state.PresetLowLatency,

    // Memory configuration
    WriteBufferSize:     128 * 1024 * 1024, // 128 MB
    MaxWriteBufferNum:   4,
    BlockCacheSize:      512 * 1024 * 1024, // 512 MB

    // Performance tuning
    MaxBackgroundJobs:   8,
    MaxSubCompactions:   4,
    OptimizeFiltersForHits: true,

    // Compaction
    Level0FileNumCompactionTrigger: 2,
    Level0SlowdownWritesTrigger:    8,
    Level0StopWritesTrigger:        12,

    // I/O optimization
    AllowMmapReads:      true,
    UseDirectReads:      false,

    // Monitoring
    EnableLatencyTracking: true,
    EnableStatistics:      true,
}

backend, err := state.NewRocksDBStateBackend(config, logger)
```

### YAML Configuration

```yaml
state:
  backend: rocksdb
  rocksdb:
    path: /var/lib/gress/rocksdb
    preset: low-latency  # or high-throughput, balanced, large-state

    # Advanced tuning (optional)
    write_buffer_size: 134217728  # 128 MB
    max_write_buffer_number: 4
    block_cache_size: 536870912   # 512 MB
    max_background_jobs: 8
    max_sub_compactions: 4
    optimize_filters_for_hits: true
    allow_mmap_reads: true
    enable_latency_tracking: true
```

## Monitoring and Metrics

### Get Metrics

```go
// Simple metrics
metrics := backend.GetMetrics()
fmt.Printf("Operations: %+v\n", metrics)
// Output: map[gets:1000 puts:500 deletes:10 batches:5 errors:0
//             get_latency_avg_us:5000 put_latency_avg_us:8000]

// Detailed metrics
detailed := backend.GetDetailedMetrics()
fmt.Printf("Detailed: %+v\n", detailed)
// Output includes operation counts, latency histograms, configuration info
```

### Metrics Available

- **Operation Counts**: gets, puts, deletes, batches, errors
- **Latency Metrics**:
  - Average latency (microseconds)
  - Max latency (microseconds)
  - P50, P95, P99 percentiles
- **Configuration**: Current preset, paths, settings

### RocksDB Internal Metrics

```go
// Get RocksDB-specific properties
memUsage := backend.GetProperty("rocksdb.estimate-table-readers-mem")
numKeys := backend.GetProperty("rocksdb.estimate-num-keys")
```

## Performance Tuning Guide

### 1. Identify Your Workload

| Workload Type | Recommended Preset | Key Optimizations |
|--------------|-------------------|-------------------|
| Real-time streaming | Low-Latency | Aggressive caching, mmap reads |
| Batch analytics | High-Throughput | Large buffers, direct I/O |
| Large state (>100GB) | Large-State | High compression, large cache |
| Mixed workload | Balanced | Default settings |

### 2. Memory Budget

RocksDB uses memory for:
- **Block Cache**: Hot data in memory (most important)
- **Write Buffers**: In-memory writes before flush
- **Index and Filters**: Bloom filters, block indexes

**Rule of Thumb**:
```
Total Memory = Block Cache + (Write Buffer Size × Num Buffers) + 100MB overhead
```

**Example for 1GB budget**:
- Block Cache: 512 MB
- Write Buffers: 4 × 100 MB = 400 MB
- Overhead: ~100 MB

### 3. Disk I/O Optimization

**For SSDs**:
```go
config.UseDirectReads = true
config.UseDirectIOForFlush = true
config.AllowMmapReads = false
```

**For HDDs**:
```go
config.AllowMmapReads = true
config.UseDirectReads = false
```

### 4. Tuning for Latency

To achieve <10ms P99 latency:

1. **Increase Block Cache** (more hot data in memory)
2. **Enable Memory-Mapped Reads** (`AllowMmapReads: true`)
3. **Aggressive Compaction** (lower Level0 triggers)
4. **More Background Jobs** (faster compaction)
5. **Optimize Bloom Filters** (`OptimizeFiltersForHits: true`)

### 5. Tuning for Throughput

To maximize ops/sec:

1. **Larger Write Buffers** (batch more writes)
2. **More Background Jobs** (parallel compaction)
3. **Relaxed Level0 Triggers** (less frequent compaction)
4. **Use Batch Operations** (10-100x improvement)

### 6. Tuning for Large State

For state > RAM:

1. **Maximum Block Cache** (use most available RAM)
2. **Enable Compression** (ZSTD for best ratio)
3. **Direct I/O** (bypass OS cache)
4. **More Open Files** (better disk access)

## Production Best Practices

### 1. Checkpointing

Enable regular checkpoints for crash recovery:

```go
// Create checkpoint every 30 seconds
checkpointPath := "/var/lib/gress/checkpoints/checkpoint-" + time.Now().Format("20060102-150405")
err := backend.CreateCheckpoint(checkpointPath)
```

### 2. Monitoring

Monitor these metrics in production:

- **Latency**: Should stay <10ms P99
- **Error Rate**: Should be 0
- **Compaction Stats**: Watch for compaction backlogs
- **Memory Usage**: Should stay within budget

### 3. Capacity Planning

**Estimate Disk Space**:
```
Disk Space = Raw State Size / Compression Ratio
Compression Ratio ≈ 3-5x with ZSTD, 2-3x with Snappy
```

**Estimate Memory**:
```
Memory = Block Cache + Write Buffers + Overhead
Recommended: 20-30% of state size for good performance
```

### 4. Backup and Recovery

```go
// Create snapshot
snapshot, err := backend.Snapshot()
// Save snapshot to durable storage (S3, GCS, etc.)

// Restore from snapshot
err = backend.Restore(snapshot)
```

### 5. Graceful Shutdown

Always close the backend properly:

```go
defer func() {
    // Flush pending writes
    backend.Flush()
    // Close and cleanup
    backend.Close()
}()
```

## Troubleshooting

### High Latency (P99 > 10ms)

**Causes**:
- Insufficient block cache
- Compaction backlog
- Disk I/O saturation

**Solutions**:
1. Increase block cache size
2. More background jobs
3. Lower compaction triggers
4. Check disk I/O (iostat)

### High Memory Usage

**Causes**:
- Block cache too large
- Too many write buffers

**Solutions**:
1. Reduce block cache size
2. Reduce write buffer count
3. Monitor with `GetProperty("rocksdb.block-cache-usage")`

### Write Stalls

**Causes**:
- Too many Level0 files
- Compaction can't keep up

**Solutions**:
1. Increase `Level0SlowdownWritesTrigger`
2. More background jobs
3. Larger write buffers

### Poor Compression

**Causes**:
- Wrong compression algorithm
- Data not compressible

**Solutions**:
1. Switch to ZSTD compression
2. Check data characteristics
3. Monitor compression ratio

## Benchmarking

Run benchmarks to validate performance:

```bash
# Run all benchmarks
go test -bench=. ./pkg/state/

# Run specific preset benchmark
go test -bench=BenchmarkRocksDBLowLatencyPreset ./pkg/state/

# Run latency tests
go test -v -run=TestRocksDBLatencyTarget ./pkg/state/
```

### Expected Results

**Low-Latency Preset**:
- Get P99: 5-8ms
- Put P99: 8-10ms
- Throughput: 50K-100K ops/sec

**High-Throughput Preset**:
- Throughput: 100K-200K ops/sec
- Latency: 10-15ms P99

**Batch Operations**:
- 10-100x faster than individual operations
- Batch of 100: ~2-5ms total

## Examples

See complete examples in:
- `examples/rocksdb-state/main.go` - Comprehensive demo of all presets
- `examples/state-backend/main.go` - Basic state usage
- `pkg/state/benchmark_test.go` - Performance benchmarks

## API Reference

### Core Methods

```go
// Get retrieves a value
func (r *RocksDBStateBackend) Get(key string) ([]byte, error)

// Put stores a value
func (r *RocksDBStateBackend) Put(key string, value []byte) error

// Delete removes a key
func (r *RocksDBStateBackend) Delete(key string) error

// BatchGet retrieves multiple values
func (r *RocksDBStateBackend) BatchGet(keys []string) (map[string][]byte, error)

// BatchPut stores multiple key-value pairs
func (r *RocksDBStateBackend) BatchPut(entries map[string][]byte) error

// BatchDelete removes multiple keys
func (r *RocksDBStateBackend) BatchDelete(keys []string) error

// Snapshot creates a full snapshot
func (r *RocksDBStateBackend) Snapshot() ([]byte, error)

// Restore restores from a snapshot
func (r *RocksDBStateBackend) Restore(data []byte) error

// GetMetrics returns operation metrics
func (r *RocksDBStateBackend) GetMetrics() map[string]int64

// GetDetailedMetrics returns detailed metrics with latency stats
func (r *RocksDBStateBackend) GetDetailedMetrics() map[string]interface{}

// Close closes the backend
func (r *RocksDBStateBackend) Close() error
```

## FAQ

### Q: Which preset should I use?

**A**: Start with `low-latency` for production. It's optimized for <10ms P99 latency and works well for most use cases.

### Q: How much memory do I need?

**A**: Minimum 512 MB for block cache. Recommended: 20-30% of your state size for optimal performance.

### Q: Can I change presets after creation?

**A**: No, but you can create a new backend with different settings and restore from a snapshot.

### Q: How do I monitor latency in production?

**A**: Use `GetDetailedMetrics()` to get latency statistics, or enable Prometheus metrics integration.

### Q: Is RocksDB state durable?

**A**: Yes, all writes are persisted to disk. Use checkpoints for point-in-time recovery.

### Q: How do I handle state larger than RAM?

**A**: Use the `large-state` preset with maximum block cache and compression enabled.

## License

Part of the Gress stream processing framework.

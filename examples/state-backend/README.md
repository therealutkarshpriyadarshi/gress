# State Backend Example

This example demonstrates how to use RocksDB and Memory state backends for stateful stream processing with checkpointing and recovery.

## What It Does

The example implements a stateful purchase processing system that:

1. **Maintains User Profiles**: Tracks total purchases, total spent, and user category in persistent state
2. **Categorizes Users**: Automatically categorizes users as Regular, Premium, or VIP based on spending
3. **Checkpointing**: Creates periodic checkpoints every 30 seconds
4. **Recovery**: Restores state from the latest checkpoint on restart
5. **TTL Support**: Automatically expires user profiles after 24 hours of inactivity
6. **Metrics**: Tracks and logs state backend operation metrics

## Features Demonstrated

- ✅ RocksDB persistent state backend
- ✅ In-memory state backend (for comparison)
- ✅ State snapshots and checkpointing
- ✅ Incremental checkpointing
- ✅ TTL (time-to-live) support
- ✅ State recovery on restart
- ✅ Metrics and monitoring

## Running the Example

### Using RocksDB Backend

```bash
# Run with RocksDB (default)
go run examples/state-backend/main.go rocksdb

# Or simply
go run examples/state-backend/main.go
```

### Using Memory Backend

```bash
# Run with in-memory backend
go run examples/state-backend/main.go memory
```

## Example Output

```
INFO  Starting State Backend Example
INFO  Using RocksDB state backend
INFO  RocksDB state backend initialized  path=./state-data ttl_enabled=true
INFO  No checkpoint found, starting fresh
INFO  Updated user profile  user_id=user-2 total_purchases=1 total_spent=234.50 category=Regular
INFO  Updated user profile  user_id=user-1 total_purchases=1 total_spent=567.80 category=Regular
INFO  Updated user profile  user_id=user-3 total_purchases=1 total_spent=123.40 category=Regular
INFO  RocksDB metrics  gets=10 puts=10 deletes=0
INFO  Updated user profile  user_id=user-2 total_purchases=2 total_spent=567.90 category=Regular
INFO  Updated user profile  user_id=user-1 total_purchases=2 total_spent=1234.60 category=Premium
INFO  Checkpoint created  id=checkpoint-1699564823000000000
INFO  Updated user profile  user_id=user-5 total_purchases=5 total_spent=12456.70 category=VIP
...
```

## Testing Recovery

1. **Run the example:**
   ```bash
   go run examples/state-backend/main.go rocksdb
   ```

2. **Wait for a checkpoint to be created** (look for "Checkpoint created" log)

3. **Stop the application** (Ctrl+C)

4. **Restart the application:**
   ```bash
   go run examples/state-backend/main.go rocksdb
   ```

5. **Verify state is restored** (look for "Restoring from checkpoint" and "Successfully restored from checkpoint")

## Comparing Backends

Run with both backends and compare performance:

```bash
# Terminal 1: RocksDB
go run examples/state-backend/main.go rocksdb

# Terminal 2: Memory
go run examples/state-backend/main.go memory
```

### Performance Characteristics

| Backend | Throughput | Latency | Persistence | State Size Limit |
|---------|-----------|---------|-------------|------------------|
| RocksDB | ~50K ops/s | 1-10ms | Yes | > RAM |
| Memory | ~100K ops/s | <1ms | No | < RAM |

## State Directory Structure

After running, you'll see:

```
.
├── state-data/           # RocksDB state files
│   ├── 000003.log
│   ├── CURRENT
│   ├── LOCK
│   ├── MANIFEST-000002
│   └── OPTIONS-000005
└── checkpoints/          # Checkpoint snapshots
    ├── checkpoint-1699564823000000000.json
    ├── checkpoint-1699564853000000000.json
    └── checkpoint-1699564883000000000.json
```

## Checkpoint Format

Checkpoints are stored as JSON files:

```json
{
  "ID": "checkpoint-1699564823000000000",
  "Timestamp": "2024-11-09T15:20:23.123456789Z",
  "Offsets": {
    "0": 1699564823
  },
  "StateData": "eyJwcm92aWRlcl9zdGF0ZSI6e30sImJhY2tlbmRfc25hcHNob3RzIjp7ImJhY2tlbmQtMCI6Int...}"
}
```

## Configuration

### RocksDB Configuration

Customize RocksDB settings:

```go
config := state.DefaultRocksDBConfig(stateDir)
config.WriteBufferSize = 128 * 1024 * 1024  // 128MB write buffer
config.MaxWriteBufferNum = 4                 // 4 write buffers
config.BlockCacheSize = 512 * 1024 * 1024    // 512MB block cache
config.TTLEnabled = true
config.DefaultTTL = 24 * time.Hour
config.EnableIncrementalCP = true
```

### Memory Backend Configuration

```go
config := &state.MemoryStateConfig{
    TTLEnabled: true,
    DefaultTTL: 24 * time.Hour,
}
```

### Checkpoint Configuration

```go
checkpointMgr := checkpoint.NewManager(
    30*time.Second,     // Checkpoint interval
    checkpointDir,      // Checkpoint directory
    logger,
)
checkpointMgr.EnableIncrementalCheckpoints(true)
```

## Monitoring

The example logs state backend metrics every 10 events:

```go
metrics := backend.GetMetrics()
logger.Info("RocksDB metrics",
    zap.Int64("gets", metrics["gets"]),
    zap.Int64("puts", metrics["puts"]),
    zap.Int64("deletes", metrics["deletes"]),
)
```

## Cleanup

Remove state and checkpoint directories:

```bash
rm -rf state-data checkpoints
```

## Next Steps

- Explore [Stateful Processing Example](../stateful-processing/) for windowed aggregations
- Read [State Backend Documentation](../../pkg/state/README.md) for detailed API reference
- Try different checkpoint intervals and TTL settings
- Benchmark with your workload patterns

## Troubleshooting

### RocksDB Not Available

If you see "Skipping RocksDB test (not available)", install RocksDB:

```bash
# Ubuntu/Debian
sudo apt-get install librocksdb-dev

# macOS
brew install rocksdb
```

### Permission Errors

Ensure write permissions for state and checkpoint directories:

```bash
chmod -R 755 state-data checkpoints
```

### Checkpoint Restore Fails

Check that checkpoint files are valid JSON and not corrupted:

```bash
cat checkpoints/checkpoint-*.json | jq .
```

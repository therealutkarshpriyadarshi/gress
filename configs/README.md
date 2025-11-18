# Gress Configuration Examples

This directory contains example configurations for common use cases. You can use these as templates for your own deployments.

## Configuration Files

### `development.yaml`
Optimized for local development and testing:
- Smaller buffer sizes and concurrency limits
- Fail-fast error handling for quick debugging
- Console logging with debug level
- In-memory state backend
- No TLS or authentication

**Use when:** Developing locally, testing new features, debugging issues

### `production.yaml`
Production-ready configuration:
- Large buffer sizes and high concurrency
- Comprehensive error handling with retry and DLQ
- RocksDB for persistent state
- Kafka sources and sinks
- TLS and authentication enabled
- Structured JSON logging

**Use when:** Deploying to production environments

### `rideshare.yaml`
Configuration for ride-sharing dynamic pricing use case:
- HTTP source for ride requests
- WebSocket source for driver locations
- Kafka sink for pricing updates
- RocksDB for demand/supply state
- Optimized for real-time processing

**Use when:** Building dynamic pricing systems, demand forecasting, or similar real-time analytics

### `iot-monitoring.yaml`
Configuration for IoT sensor monitoring and anomaly detection:
- High throughput settings (100K buffer, 1K concurrency)
- Multiple Kafka topics for different sensor types
- TimescaleDB sinks for time-series data
- File-based DLQ for reliability
- Optimized for high-volume sensor data

**Use when:** Processing IoT sensor data, monitoring infrastructure, or time-series analysis

## Usage

### Loading a Configuration File

```go
import "github.com/therealutkarshpriyadarshi/gress/pkg/config"

// Load and validate configuration
cfg, err := config.ValidateAndLoad("configs/production.yaml")
if err != nil {
    log.Fatal(err)
}

// Use the configuration
engineConfig := cfg.ToEngineConfig()
errorConfig := cfg.ToEngineErrorConfig()
```

### Environment Variable Overrides

All configuration values can be overridden using environment variables with the `GRESS_` prefix:

```bash
# Override buffer size
export GRESS_ENGINE_BUFFER_SIZE=50000

# Override log level
export GRESS_LOG_LEVEL=debug

# Override Kafka brokers
export GRESS_ERROR_DLQ_KAFKA_BROKERS=kafka1:9092,kafka2:9092

# Load config with env overrides
cfg, err := config.LoadConfigWithEnv("configs/production.yaml")
```

### Hot Reload

Enable hot reloading to update non-critical settings without restarting:

```go
// Create reloadable config
reloadableConfig, err := config.NewReloadableConfig("configs/production.yaml", logger)
if err != nil {
    log.Fatal(err)
}

// Register callback for config changes
reloadableConfig.OnReload(func(oldCfg, newCfg *config.Config) error {
    logger.Info("Configuration reloaded",
        zap.String("old_level", oldCfg.Logging.Level),
        zap.String("new_level", newCfg.Logging.Level))
    return nil
})

// Start watching for changes
reloadableConfig.Start()
defer reloadableConfig.Stop()

// Get current config
cfg := reloadableConfig.Get()
```

### Hot-Reloadable Settings

The following settings can be changed without restart:
- `engine.checkpoint_interval`
- `engine.watermark_interval`
- `engine.metrics_interval`
- `engine.enable_backpressure`
- `engine.backpressure_threshold`
- `error_handling.strategy`
- `error_handling.enable_retry`
- `error_handling.max_retry_attempts`
- `error_handling.*_backoff`
- `error_handling.circuit_breaker.*`
- `metrics.enabled`
- `logging.level`
- `logging.format`

### Critical Settings (Require Restart)

The following settings require application restart:
- `version`
- `application.name`
- `engine.buffer_size`
- `engine.max_concurrency`
- `engine.checkpoint_dir`
- `state.backend`
- `state.rocksdb.path`
- All `sources.*` configuration
- All `sinks.*` configuration

## Configuration Structure

```yaml
version: v1                    # Configuration schema version

application:
  name: app-name              # Application name
  environment: production     # Environment (development, staging, production)
  tags: {}                    # Custom tags for metadata

engine:
  buffer_size: 10000          # Event buffer size
  max_concurrency: 100        # Max concurrent event processors
  checkpoint_interval: 30s    # Checkpoint interval
  watermark_interval: 5s      # Watermark interval
  metrics_interval: 10s       # Metrics logging interval
  enable_backpressure: true   # Enable backpressure control
  backpressure_threshold: 0.8 # Backpressure threshold (0-1)
  checkpoint_dir: ./checkpoints # Checkpoint directory

error_handling:
  strategy: retry-then-dlq    # fail-fast, retry, send-to-dlq, retry-then-dlq
  enable_retry: true          # Enable retry logic
  max_retry_attempts: 3       # Max retry attempts
  initial_backoff: 100ms      # Initial backoff duration
  max_backoff: 30s            # Maximum backoff duration
  backoff_multiplier: 2.0     # Backoff multiplier
  backoff_jitter: 0.1         # Backoff jitter (0-1)
  enable_dlq: true            # Enable dead letter queue
  dlq_max_size: 10000         # DLQ max size
  dlq_type: memory            # memory, file, kafka
  enable_circuit_breaker: true # Enable circuit breaker
  circuit_breaker:
    failure_threshold: 5      # Failures before opening
    success_threshold: 2      # Successes before closing
    timeout: 60s              # Timeout in open state
  enable_side_outputs: true   # Enable side outputs
  side_output_buffer_size: 1000 # Side output buffer size

sources:
  kafka: []                   # Kafka sources
  http: []                    # HTTP sources
  websocket: []               # WebSocket sources

sinks:
  kafka: []                   # Kafka sinks
  timescaledb: []             # TimescaleDB sinks

state:
  backend: memory             # memory or rocksdb
  rocksdb:                    # RocksDB configuration (if used)
    path: ./data/rocksdb
    write_buffer_size: 67108864
    max_write_buffer_number: 3
    block_cache_size: 536870912

metrics:
  enabled: true               # Enable Prometheus metrics
  address: ":9091"            # Metrics server address
  path: /metrics              # Metrics endpoint path
  namespace: gress            # Prometheus namespace
  subsystem: stream           # Prometheus subsystem

logging:
  level: info                 # debug, info, warn, error
  format: json                # json or console
  output: stdout              # stdout, stderr, file
  output_path: ""             # File path (if output=file)

security:
  enable_tls: false           # Enable TLS
  cert_file: ""               # TLS certificate file
  key_file: ""                # TLS key file
  ca_file: ""                 # CA certificate file
  enable_auth: false          # Enable authentication
  auth_type: none             # none, basic, bearer, mtls
```

## Validation

All configurations are validated on load. Common validation errors:

- **Version mismatch**: Configuration version doesn't match current version
- **Invalid values**: Negative numbers, invalid enums, etc.
- **Missing required fields**: Sources without names, empty broker lists, etc.
- **File not found**: Certificate files, checkpoint directories, etc.
- **Logical errors**: Max backoff < initial backoff, etc.

Run validation manually:

```go
cfg, err := config.LoadConfig("config.yaml")
if err != nil {
    log.Fatal(err)
}

if err := config.Validate(cfg); err != nil {
    log.Fatal(err)
}
```

## Best Practices

1. **Start with an example**: Use one of the provided examples as a template
2. **Use environment variables**: Override sensitive values (passwords, tokens) via environment variables
3. **Validate in CI/CD**: Add configuration validation to your CI/CD pipeline
4. **Version your configs**: Keep configuration files in version control
5. **Document custom settings**: Add comments to explain non-obvious configurations
6. **Test configurations**: Test configuration changes in development before production
7. **Monitor after changes**: Watch metrics and logs after configuration changes
8. **Use hot reload**: Take advantage of hot reload for non-critical settings

## Support

For more information:
- See the main [README](../README.md)
- Check the [ARCHITECTURE](../ARCHITECTURE.md) document
- Review the [API documentation](https://pkg.go.dev/github.com/therealutkarshpriyadarshi/gress)

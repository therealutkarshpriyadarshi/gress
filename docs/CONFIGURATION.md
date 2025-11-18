# Configuration Management

Gress provides a flexible and powerful configuration management system that supports YAML/JSON files, environment variable overrides, validation, hot reload, and versioning.

## Table of Contents

- [Quick Start](#quick-start)
- [Configuration File Format](#configuration-file-format)
- [Environment Variables](#environment-variables)
- [Validation](#validation)
- [Hot Reload](#hot-reload)
- [Configuration Versioning](#configuration-versioning)
- [Example Configurations](#example-configurations)
- [Best Practices](#best-practices)

## Quick Start

### 1. Create a Configuration File

Start with one of the example configurations:

```bash
# Copy an example configuration
cp configs/development.yaml config.yaml

# Edit as needed
vim config.yaml
```

### 2. Load Configuration in Your Application

```go
package main

import (
    "log"
    "github.com/therealutkarshpriyadarshi/gress/pkg/config"
    "github.com/therealutkarshpriyadarshi/gress/pkg/stream"
)

func main() {
    // Load and validate configuration
    cfg, err := config.ValidateAndLoad("config.yaml")
    if err != nil {
        log.Fatal(err)
    }

    // Convert to engine configuration
    engineConfig := cfg.ToEngineConfig()
    errorConfig := cfg.ToEngineErrorConfig()

    // Create stream engine
    engine := stream.NewEngine(engineConfig, logger)

    // Use the configuration...
}
```

### 3. Override with Environment Variables

```bash
# Override specific settings
export GRESS_LOG_LEVEL=debug
export GRESS_ENGINE_BUFFER_SIZE=20000

# Run your application
./gress --config config.yaml
```

## Configuration File Format

Gress supports both YAML and JSON configuration files. The format is determined automatically by the file extension (`.yaml`, `.yml`, or `.json`).

### Complete Configuration Structure

```yaml
# Configuration version (required)
version: v1

# Application metadata
application:
  name: my-app                    # Application name (required)
  environment: production         # Environment: development, staging, production, test
  tags:                           # Custom tags (optional)
    team: platform
    region: us-east-1

# Stream processing engine configuration
engine:
  buffer_size: 10000              # Event buffer size
  max_concurrency: 100            # Maximum concurrent event processors
  checkpoint_interval: 30s        # How often to create checkpoints
  watermark_interval: 5s          # Watermark update interval
  metrics_interval: 10s           # Metrics logging interval
  enable_backpressure: true       # Enable backpressure control
  backpressure_threshold: 0.8     # Backpressure threshold (0.0-1.0)
  checkpoint_dir: ./checkpoints   # Checkpoint storage directory

# Error handling and recovery configuration
error_handling:
  strategy: retry-then-dlq        # fail-fast, retry, send-to-dlq, retry-then-dlq
  enable_retry: true              # Enable automatic retry
  max_retry_attempts: 3           # Maximum retry attempts
  initial_backoff: 100ms          # Initial backoff duration
  max_backoff: 30s                # Maximum backoff duration
  backoff_multiplier: 2.0         # Exponential backoff multiplier
  backoff_jitter: 0.1             # Backoff jitter factor (0.0-1.0)

  # Dead Letter Queue (DLQ) configuration
  enable_dlq: true                # Enable DLQ
  dlq_max_size: 10000             # Maximum DLQ size
  dlq_type: memory                # memory, file, or kafka
  dlq_directory: ./dlq            # Directory for file-based DLQ
  dlq_kafka_topic: dlq-topic      # Topic for Kafka-based DLQ
  dlq_kafka_brokers:              # Brokers for Kafka-based DLQ
    - kafka-1:9092
    - kafka-2:9092

  # Circuit breaker configuration
  enable_circuit_breaker: true    # Enable circuit breaker
  circuit_breaker:
    failure_threshold: 5          # Failures before opening circuit
    success_threshold: 2          # Successes before closing circuit
    timeout: 60s                  # Timeout in open state

  # Side outputs configuration
  enable_side_outputs: true       # Enable side outputs
  side_output_buffer_size: 1000   # Side output buffer size

# Data sources configuration
sources:
  # Kafka sources
  kafka:
    - name: main-kafka              # Source name (required)
      brokers:                      # Kafka brokers (required)
        - kafka-1:9092
        - kafka-2:9092
      topics:                       # Topics to consume (required)
        - events
        - transactions
      group_id: consumer-group      # Consumer group ID (required)
      auto_commit: true             # Enable auto-commit
      commit_interval: 5s           # Commit interval

  # HTTP sources
  http:
    - name: http-ingress            # Source name (required)
      address: ":8080"              # Listen address (required)
      path: /events                 # HTTP path (required)
      tls: false                    # Enable TLS
      cert_file: /path/to/cert.pem  # TLS certificate (if tls=true)
      key_file: /path/to/key.pem    # TLS key (if tls=true)

  # WebSocket sources
  websocket:
    - name: ws-ingress              # Source name (required)
      address: ":8081"              # Listen address (required)
      path: /stream                 # WebSocket path (required)
      tls: false                    # Enable TLS
      cert_file: /path/to/cert.pem  # TLS certificate (if tls=true)
      key_file: /path/to/key.pem    # TLS key (if tls=true)

# Data sinks configuration
sinks:
  # Kafka sinks
  kafka:
    - name: output-kafka            # Sink name (required)
      brokers:                      # Kafka brokers (required)
        - kafka-1:9092
        - kafka-2:9092
      topic: output-topic           # Output topic (required)
      flush_interval: 1s            # Flush interval
      batch_size: 1000              # Batch size

  # TimescaleDB sinks
  timescaledb:
    - name: metrics-db              # Sink name (required)
      connection_string: "..."      # PostgreSQL connection string (required)
      table: metrics                # Table name (required)
      batch_size: 500               # Batch size
      flush_interval: 5s            # Flush interval

# State backend configuration
state:
  backend: rocksdb                  # memory or rocksdb
  rocksdb:                          # RocksDB configuration (if backend=rocksdb)
    path: ./data/rocksdb            # RocksDB data directory
    write_buffer_size: 67108864     # Write buffer size (64 MB)
    max_write_buffer_number: 3      # Number of write buffers
    block_cache_size: 536870912     # Block cache size (512 MB)

# Metrics and monitoring configuration
metrics:
  enabled: true                     # Enable Prometheus metrics
  address: ":9091"                  # Metrics server address
  path: /metrics                    # Metrics endpoint path
  namespace: gress                  # Prometheus namespace
  subsystem: stream                 # Prometheus subsystem

# Logging configuration
logging:
  level: info                       # debug, info, warn, error
  format: json                      # json or console
  output: stdout                    # stdout, stderr, or file
  output_path: /var/log/app.log    # Log file path (if output=file)

# Security configuration
security:
  enable_tls: false                 # Enable TLS
  cert_file: /path/to/cert.pem     # TLS certificate
  key_file: /path/to/key.pem       # TLS key
  ca_file: /path/to/ca.pem         # CA certificate
  enable_auth: false                # Enable authentication
  auth_type: none                   # none, basic, bearer, or mtls
```

## Environment Variables

All configuration values can be overridden using environment variables with the `GRESS_` prefix. The naming convention converts the YAML path to uppercase with underscores.

### Examples

```bash
# Application configuration
export GRESS_APPLICATION_NAME=my-app
export GRESS_APPLICATION_ENVIRONMENT=production

# Engine configuration
export GRESS_ENGINE_BUFFER_SIZE=50000
export GRESS_ENGINE_MAX_CONCURRENCY=500
export GRESS_ENGINE_CHECKPOINT_INTERVAL=30s
export GRESS_ENGINE_ENABLE_BACKPRESSURE=true
export GRESS_ENGINE_BACKPRESSURE_THRESHOLD=0.85

# Error handling
export GRESS_ERROR_STRATEGY=retry-then-dlq
export GRESS_ERROR_ENABLE_RETRY=true
export GRESS_ERROR_MAX_RETRY_ATTEMPTS=5
export GRESS_ERROR_ENABLE_DLQ=true
export GRESS_ERROR_DLQ_TYPE=kafka
export GRESS_ERROR_DLQ_KAFKA_TOPIC=dlq-topic
export GRESS_ERROR_DLQ_KAFKA_BROKERS=kafka1:9092,kafka2:9092

# State backend
export GRESS_STATE_BACKEND=rocksdb
export GRESS_STATE_ROCKSDB_PATH=/data/rocksdb

# Metrics
export GRESS_METRICS_ENABLED=true
export GRESS_METRICS_ADDRESS=:9091

# Logging
export GRESS_LOG_LEVEL=debug
export GRESS_LOG_FORMAT=json
export GRESS_LOG_OUTPUT=stdout

# Security
export GRESS_SECURITY_ENABLE_TLS=true
export GRESS_SECURITY_CERT_FILE=/etc/certs/server.crt
export GRESS_SECURITY_KEY_FILE=/etc/certs/server.key
```

### Loading Configuration with Environment Overrides

```go
// Load configuration with environment variable overrides
cfg, err := config.LoadConfigWithEnv("config.yaml")
if err != nil {
    log.Fatal(err)
}
```

## Validation

All configurations are automatically validated when loaded. Validation checks include:

- **Version compatibility**: Ensures configuration version is compatible
- **Required fields**: Checks all required fields are present
- **Valid values**: Validates enums, ranges, and formats
- **Logical consistency**: Ensures settings make sense together
- **File existence**: Verifies referenced files exist (certificates, etc.)

### Manual Validation

```go
cfg, err := config.LoadConfig("config.yaml")
if err != nil {
    log.Fatal(err)
}

// Validate manually
if err := config.Validate(cfg); err != nil {
    // err contains detailed validation errors
    log.Fatal(err)
}
```

### Validation Errors

Validation errors provide detailed information:

```
found 3 validation error(s):
  - engine.buffer_size: buffer size must be positive
  - error_handling.strategy: invalid strategy 'invalid' (valid: fail-fast, retry, send-to-dlq, retry-then-dlq)
  - sources.kafka[0].brokers: at least one broker is required
```

## Hot Reload

Gress supports hot reloading of non-critical configuration settings without restarting the application.

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
- `error_handling.initial_backoff`
- `error_handling.max_backoff`
- `error_handling.backoff_multiplier`
- `error_handling.backoff_jitter`
- `error_handling.enable_circuit_breaker`
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

### Using Hot Reload

```go
import (
    "github.com/therealutkarshpriyadarshi/gress/pkg/config"
    "go.uber.org/zap"
)

// Create reloadable configuration
reloadableConfig, err := config.NewReloadableConfig("config.yaml", logger)
if err != nil {
    log.Fatal(err)
}

// Register callback for configuration changes
reloadableConfig.OnReload(func(oldCfg, newCfg *config.Config) error {
    logger.Info("Configuration reloaded",
        zap.String("old_log_level", oldCfg.Logging.Level),
        zap.String("new_log_level", newCfg.Logging.Level))

    // Update log level
    updateLogLevel(newCfg.Logging.Level)

    return nil
})

// Set reload check interval (default: 10s)
reloadableConfig.SetReloadInterval(5 * time.Second)

// Start watching for changes
reloadableConfig.Start()
defer reloadableConfig.Stop()

// Get current configuration
cfg := reloadableConfig.Get()

// Manually trigger reload
if err := reloadableConfig.Reload(); err != nil {
    logger.Error("Failed to reload config", zap.Error(err))
}
```

## Configuration Versioning

Gress uses semantic versioning for configuration files to ensure compatibility and enable future migrations.

### Current Version

The current configuration version is `v1`.

### Version Compatibility

- **Major version**: Must match (v1 configs require v1 parser)
- **Minor version**: Can differ (v1.1 configs work with v1.0 parser)

### Version Checking

```go
// Check version compatibility
if err := config.ValidateVersion(cfg); err != nil {
    log.Fatal(err)
}

// Get version information
versionInfo := config.GetVersionInfo()
fmt.Printf("Current version: %s\n", versionInfo.ConfigVersion)
fmt.Printf("Supported versions: %v\n", versionInfo.SupportedVersions)
```

### Future Migrations

When new configuration versions are released, migrations will be provided:

```go
// Migrate configuration to current version
migratedCfg, err := config.MigrateConfig(oldCfg)
if err != nil {
    log.Fatal(err)
}
```

## Example Configurations

Gress provides several example configurations in the `configs/` directory:

- **`development.yaml`**: Local development and testing
- **`production.yaml`**: Production deployments
- **`rideshare.yaml`**: Ride-sharing dynamic pricing use case
- **`iot-monitoring.yaml`**: IoT sensor monitoring and anomaly detection

See [configs/README.md](../configs/README.md) for detailed information.

## Best Practices

### 1. Start with Examples

Use the provided example configurations as templates:

```bash
cp configs/production.yaml config.yaml
```

### 2. Use Environment Variables for Secrets

Never store secrets in configuration files:

```yaml
# Good: Use environment variable placeholder
sinks:
  timescaledb:
    - connection_string: "host=db port=5432 password=$DB_PASSWORD"
```

```bash
# Set secret via environment variable
export DB_PASSWORD=secret123
```

### 3. Validate in CI/CD

Add configuration validation to your CI/CD pipeline:

```bash
# Validate all configurations
for config in configs/*.yaml; do
    gress validate --config $config
done
```

### 4. Version Control Your Configs

- Keep configuration files in version control
- Use separate files for each environment
- Document any non-obvious settings with comments

### 5. Test Configuration Changes

Always test configuration changes in development before production:

```bash
# Test with development config
./gress --config configs/development.yaml

# Verify metrics
curl http://localhost:9091/metrics
```

### 6. Monitor After Changes

Watch metrics and logs after configuration changes:

```bash
# Watch logs
tail -f /var/log/gress/app.log

# Monitor metrics
watch -n 1 'curl -s http://localhost:9091/metrics | grep gress_'
```

### 7. Use Hot Reload When Possible

Take advantage of hot reload for tuning:

- Adjust log levels without restart
- Tune backpressure thresholds
- Modify retry policies
- Update checkpoint intervals

### 8. Document Custom Settings

Add comments to explain non-obvious configurations:

```yaml
engine:
  # Increased for high-throughput scenarios (peak: 50K events/sec)
  buffer_size: 100000

  # Tuned based on CPU cores (32 cores available)
  max_concurrency: 1000
```

## API Reference

### Loading Functions

- `LoadConfig(path string) (*Config, error)` - Load configuration from file
- `LoadConfigWithDefaults(path string) (*Config, error)` - Load with default values
- `LoadConfigWithEnv(path string) (*Config, error)` - Load with environment overrides
- `LoadOrDefault(path string) (*Config, error)` - Load or return default config
- `LoadOrDefaultWithEnv(path string) (*Config, error)` - Load or default with env overrides
- `ValidateAndLoad(path string) (*Config, error)` - Load and validate

### Saving Functions

- `SaveConfig(config *Config, path string) error` - Save configuration to file

### Validation Functions

- `Validate(config *Config) error` - Validate entire configuration
- `ValidateVersion(config *Config) error` - Validate version compatibility

### Conversion Functions

- `ToEngineConfig() stream.EngineConfig` - Convert to engine configuration
- `ToEngineErrorConfig() *stream.EngineErrorConfig` - Convert to error handling configuration

### Utility Functions

- `DefaultConfig() *Config` - Get default configuration
- `ProductionConfig() *Config` - Get production configuration
- `DevelopmentConfig() *Config` - Get development configuration
- `MergeConfigs(configs ...*Config) *Config` - Merge multiple configurations

### Hot Reload Functions

- `NewReloadableConfig(path string, logger *zap.Logger) (*ReloadableConfig, error)` - Create reloadable config
- `Get() *Config` - Get current configuration
- `OnReload(callback ReloadCallback)` - Register reload callback
- `SetReloadInterval(interval time.Duration)` - Set reload check interval
- `Start()` - Start watching for changes
- `Stop()` - Stop watching
- `Reload() error` - Manually trigger reload

## Troubleshooting

### Configuration File Not Found

```
Error: failed to read config file: open config.yaml: no such file or directory
```

**Solution**: Ensure the file path is correct or use `LoadOrDefault` to use defaults if file doesn't exist.

### Validation Errors

```
Error: found 2 validation error(s):
  - engine.buffer_size: buffer size must be positive
  - logging.level: invalid log level 'trace' (valid: debug, info, warn, error)
```

**Solution**: Fix the validation errors in your configuration file.

### Environment Variable Parse Errors

```
Error: invalid GRESS_ENGINE_BUFFER_SIZE: strconv.Atoi: parsing "abc": invalid syntax
```

**Solution**: Ensure environment variables have valid values for their types.

### Hot Reload Failures

```
Error: critical settings changed (requires restart): buffer size changed from 10000 to 20000
```

**Solution**: Restart the application to apply critical setting changes.

### Version Compatibility Errors

```
Error: incompatible configuration version: v2 (current: v1)
```

**Solution**: Update your application or migrate your configuration file.

## Support

For additional help:

- Check the [README](../README.md)
- Review the [ARCHITECTURE](../ARCHITECTURE.md) document
- See the [API documentation](https://pkg.go.dev/github.com/therealutkarshpriyadarshi/gress)
- Open an issue on [GitHub](https://github.com/therealutkarshpriyadarshi/gress/issues)

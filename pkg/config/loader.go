package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// LoadConfig loads configuration from a file
// Supports both YAML and JSON formats
func LoadConfig(path string) (*Config, error) {
	// Read file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Determine format based on file extension
	ext := strings.ToLower(filepath.Ext(path))

	var config Config

	switch ext {
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, &config); err != nil {
			return nil, fmt.Errorf("failed to parse YAML config: %w", err)
		}
	case ".json":
		if err := json.Unmarshal(data, &config); err != nil {
			return nil, fmt.Errorf("failed to parse JSON config: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported config file format: %s (supported: .yaml, .yml, .json)", ext)
	}

	return &config, nil
}

// LoadConfigWithDefaults loads configuration from a file and applies defaults for missing values
func LoadConfigWithDefaults(path string) (*Config, error) {
	config, err := LoadConfig(path)
	if err != nil {
		return nil, err
	}

	// Apply defaults for missing values
	applyDefaults(config)

	return config, nil
}

// LoadOrDefault attempts to load configuration from path, returns default config if file doesn't exist
func LoadOrDefault(path string) (*Config, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return DefaultConfig(), nil
	}

	return LoadConfigWithDefaults(path)
}

// SaveConfig saves configuration to a file
// Format is determined by file extension
func SaveConfig(config *Config, path string) error {
	ext := strings.ToLower(filepath.Ext(path))

	var data []byte
	var err error

	switch ext {
	case ".yaml", ".yml":
		data, err = yaml.Marshal(config)
		if err != nil {
			return fmt.Errorf("failed to marshal config to YAML: %w", err)
		}
	case ".json":
		data, err = json.MarshalIndent(config, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal config to JSON: %w", err)
		}
	default:
		return fmt.Errorf("unsupported config file format: %s (supported: .yaml, .yml, .json)", ext)
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Write file
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// applyDefaults fills in missing values with defaults
func applyDefaults(config *Config) {
	defaults := DefaultConfig()

	// Apply version if missing
	if config.Version == "" {
		config.Version = defaults.Version
	}

	// Apply application defaults
	if config.Application.Name == "" {
		config.Application.Name = defaults.Application.Name
	}
	if config.Application.Environment == "" {
		config.Application.Environment = defaults.Application.Environment
	}
	if config.Application.Tags == nil {
		config.Application.Tags = make(map[string]string)
	}

	// Apply engine defaults
	if config.Engine.BufferSize == 0 {
		config.Engine.BufferSize = defaults.Engine.BufferSize
	}
	if config.Engine.MaxConcurrency == 0 {
		config.Engine.MaxConcurrency = defaults.Engine.MaxConcurrency
	}
	if config.Engine.CheckpointInterval == 0 {
		config.Engine.CheckpointInterval = defaults.Engine.CheckpointInterval
	}
	if config.Engine.WatermarkInterval == 0 {
		config.Engine.WatermarkInterval = defaults.Engine.WatermarkInterval
	}
	if config.Engine.MetricsInterval == 0 {
		config.Engine.MetricsInterval = defaults.Engine.MetricsInterval
	}
	if config.Engine.BackpressureThreshold == 0 {
		config.Engine.BackpressureThreshold = defaults.Engine.BackpressureThreshold
	}
	if config.Engine.CheckpointDir == "" {
		config.Engine.CheckpointDir = defaults.Engine.CheckpointDir
	}

	// Apply error handling defaults
	if config.ErrorHandling.Strategy == "" {
		config.ErrorHandling.Strategy = defaults.ErrorHandling.Strategy
	}
	if config.ErrorHandling.MaxRetryAttempts == 0 {
		config.ErrorHandling.MaxRetryAttempts = defaults.ErrorHandling.MaxRetryAttempts
	}
	if config.ErrorHandling.InitialBackoff == 0 {
		config.ErrorHandling.InitialBackoff = defaults.ErrorHandling.InitialBackoff
	}
	if config.ErrorHandling.MaxBackoff == 0 {
		config.ErrorHandling.MaxBackoff = defaults.ErrorHandling.MaxBackoff
	}
	if config.ErrorHandling.BackoffMultiplier == 0 {
		config.ErrorHandling.BackoffMultiplier = defaults.ErrorHandling.BackoffMultiplier
	}
	if config.ErrorHandling.BackoffJitter == 0 {
		config.ErrorHandling.BackoffJitter = defaults.ErrorHandling.BackoffJitter
	}
	if config.ErrorHandling.DLQMaxSize == 0 {
		config.ErrorHandling.DLQMaxSize = defaults.ErrorHandling.DLQMaxSize
	}
	if config.ErrorHandling.DLQType == "" {
		config.ErrorHandling.DLQType = defaults.ErrorHandling.DLQType
	}
	if config.ErrorHandling.CircuitBreakerConfig.FailureThreshold == 0 {
		config.ErrorHandling.CircuitBreakerConfig.FailureThreshold = defaults.ErrorHandling.CircuitBreakerConfig.FailureThreshold
	}
	if config.ErrorHandling.CircuitBreakerConfig.SuccessThreshold == 0 {
		config.ErrorHandling.CircuitBreakerConfig.SuccessThreshold = defaults.ErrorHandling.CircuitBreakerConfig.SuccessThreshold
	}
	if config.ErrorHandling.CircuitBreakerConfig.Timeout == 0 {
		config.ErrorHandling.CircuitBreakerConfig.Timeout = defaults.ErrorHandling.CircuitBreakerConfig.Timeout
	}
	if config.ErrorHandling.SideOutputBufferSize == 0 {
		config.ErrorHandling.SideOutputBufferSize = defaults.ErrorHandling.SideOutputBufferSize
	}

	// Apply state defaults
	if config.State.Backend == "" {
		config.State.Backend = defaults.State.Backend
	}
	if config.State.RocksDB.Path == "" {
		config.State.RocksDB.Path = defaults.State.RocksDB.Path
	}
	if config.State.RocksDB.WriteBufferSize == 0 {
		config.State.RocksDB.WriteBufferSize = defaults.State.RocksDB.WriteBufferSize
	}
	if config.State.RocksDB.MaxWriteBufferNumber == 0 {
		config.State.RocksDB.MaxWriteBufferNumber = defaults.State.RocksDB.MaxWriteBufferNumber
	}
	if config.State.RocksDB.BlockCacheSize == 0 {
		config.State.RocksDB.BlockCacheSize = defaults.State.RocksDB.BlockCacheSize
	}

	// Apply metrics defaults
	if config.Metrics.Address == "" {
		config.Metrics.Address = defaults.Metrics.Address
	}
	if config.Metrics.Path == "" {
		config.Metrics.Path = defaults.Metrics.Path
	}
	if config.Metrics.Namespace == "" {
		config.Metrics.Namespace = defaults.Metrics.Namespace
	}
	if config.Metrics.Subsystem == "" {
		config.Metrics.Subsystem = defaults.Metrics.Subsystem
	}

	// Apply logging defaults
	if config.Logging.Level == "" {
		config.Logging.Level = defaults.Logging.Level
	}
	if config.Logging.Format == "" {
		config.Logging.Format = defaults.Logging.Format
	}
	if config.Logging.Output == "" {
		config.Logging.Output = defaults.Logging.Output
	}

	// Apply security defaults
	if config.Security.AuthType == "" {
		config.Security.AuthType = defaults.Security.AuthType
	}
}

// MergeConfigs merges multiple configurations, with later configs overriding earlier ones
func MergeConfigs(configs ...*Config) *Config {
	if len(configs) == 0 {
		return DefaultConfig()
	}

	result := configs[0]

	for i := 1; i < len(configs); i++ {
		mergeInto(result, configs[i])
	}

	return result
}

// mergeInto merges source into target, overriding non-zero values
func mergeInto(target, source *Config) {
	// Merge application config
	if source.Application.Name != "" {
		target.Application.Name = source.Application.Name
	}
	if source.Application.Environment != "" {
		target.Application.Environment = source.Application.Environment
	}
	for k, v := range source.Application.Tags {
		target.Application.Tags[k] = v
	}

	// Merge engine config
	if source.Engine.BufferSize != 0 {
		target.Engine.BufferSize = source.Engine.BufferSize
	}
	if source.Engine.MaxConcurrency != 0 {
		target.Engine.MaxConcurrency = source.Engine.MaxConcurrency
	}
	if source.Engine.CheckpointInterval != 0 {
		target.Engine.CheckpointInterval = source.Engine.CheckpointInterval
	}
	if source.Engine.WatermarkInterval != 0 {
		target.Engine.WatermarkInterval = source.Engine.WatermarkInterval
	}
	if source.Engine.MetricsInterval != 0 {
		target.Engine.MetricsInterval = source.Engine.MetricsInterval
	}
	if source.Engine.BackpressureThreshold != 0 {
		target.Engine.BackpressureThreshold = source.Engine.BackpressureThreshold
	}
	if source.Engine.CheckpointDir != "" {
		target.Engine.CheckpointDir = source.Engine.CheckpointDir
	}

	// Merge error handling config
	if source.ErrorHandling.Strategy != "" {
		target.ErrorHandling.Strategy = source.ErrorHandling.Strategy
	}
	// Note: Booleans are tricky - we copy them as-is since false is valid
	target.ErrorHandling.EnableRetry = source.ErrorHandling.EnableRetry
	target.ErrorHandling.EnableDLQ = source.ErrorHandling.EnableDLQ
	target.ErrorHandling.EnableCircuitBreaker = source.ErrorHandling.EnableCircuitBreaker
	target.ErrorHandling.EnableSideOutputs = source.ErrorHandling.EnableSideOutputs

	// Merge sources and sinks (append)
	target.Sources.Kafka = append(target.Sources.Kafka, source.Sources.Kafka...)
	target.Sources.HTTP = append(target.Sources.HTTP, source.Sources.HTTP...)
	target.Sources.WebSocket = append(target.Sources.WebSocket, source.Sources.WebSocket...)
	target.Sinks.Kafka = append(target.Sinks.Kafka, source.Sinks.Kafka...)
	target.Sinks.TimescaleDB = append(target.Sinks.TimescaleDB, source.Sinks.TimescaleDB...)
}

package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// ApplyEnvOverrides applies environment variable overrides to the configuration
// Environment variables follow the pattern: GRESS_<SECTION>_<KEY>
// Example: GRESS_ENGINE_BUFFER_SIZE=20000
func ApplyEnvOverrides(config *Config) error {
	// Application overrides
	if val := os.Getenv("GRESS_APPLICATION_NAME"); val != "" {
		config.Application.Name = val
	}
	if val := os.Getenv("GRESS_APPLICATION_ENVIRONMENT"); val != "" {
		config.Application.Environment = val
	}

	// Engine overrides
	if val := os.Getenv("GRESS_ENGINE_BUFFER_SIZE"); val != "" {
		size, err := strconv.Atoi(val)
		if err != nil {
			return fmt.Errorf("invalid GRESS_ENGINE_BUFFER_SIZE: %w", err)
		}
		config.Engine.BufferSize = size
	}

	if val := os.Getenv("GRESS_ENGINE_MAX_CONCURRENCY"); val != "" {
		concurrency, err := strconv.Atoi(val)
		if err != nil {
			return fmt.Errorf("invalid GRESS_ENGINE_MAX_CONCURRENCY: %w", err)
		}
		config.Engine.MaxConcurrency = concurrency
	}

	if val := os.Getenv("GRESS_ENGINE_CHECKPOINT_INTERVAL"); val != "" {
		duration, err := time.ParseDuration(val)
		if err != nil {
			return fmt.Errorf("invalid GRESS_ENGINE_CHECKPOINT_INTERVAL: %w", err)
		}
		config.Engine.CheckpointInterval = duration
	}

	if val := os.Getenv("GRESS_ENGINE_WATERMARK_INTERVAL"); val != "" {
		duration, err := time.ParseDuration(val)
		if err != nil {
			return fmt.Errorf("invalid GRESS_ENGINE_WATERMARK_INTERVAL: %w", err)
		}
		config.Engine.WatermarkInterval = duration
	}

	if val := os.Getenv("GRESS_ENGINE_METRICS_INTERVAL"); val != "" {
		duration, err := time.ParseDuration(val)
		if err != nil {
			return fmt.Errorf("invalid GRESS_ENGINE_METRICS_INTERVAL: %w", err)
		}
		config.Engine.MetricsInterval = duration
	}

	if val := os.Getenv("GRESS_ENGINE_ENABLE_BACKPRESSURE"); val != "" {
		enabled, err := strconv.ParseBool(val)
		if err != nil {
			return fmt.Errorf("invalid GRESS_ENGINE_ENABLE_BACKPRESSURE: %w", err)
		}
		config.Engine.EnableBackpressure = enabled
	}

	if val := os.Getenv("GRESS_ENGINE_BACKPRESSURE_THRESHOLD"); val != "" {
		threshold, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return fmt.Errorf("invalid GRESS_ENGINE_BACKPRESSURE_THRESHOLD: %w", err)
		}
		config.Engine.BackpressureThreshold = threshold
	}

	if val := os.Getenv("GRESS_ENGINE_CHECKPOINT_DIR"); val != "" {
		config.Engine.CheckpointDir = val
	}

	// Error handling overrides
	if val := os.Getenv("GRESS_ERROR_STRATEGY"); val != "" {
		config.ErrorHandling.Strategy = val
	}

	if val := os.Getenv("GRESS_ERROR_ENABLE_RETRY"); val != "" {
		enabled, err := strconv.ParseBool(val)
		if err != nil {
			return fmt.Errorf("invalid GRESS_ERROR_ENABLE_RETRY: %w", err)
		}
		config.ErrorHandling.EnableRetry = enabled
	}

	if val := os.Getenv("GRESS_ERROR_MAX_RETRY_ATTEMPTS"); val != "" {
		attempts, err := strconv.Atoi(val)
		if err != nil {
			return fmt.Errorf("invalid GRESS_ERROR_MAX_RETRY_ATTEMPTS: %w", err)
		}
		config.ErrorHandling.MaxRetryAttempts = attempts
	}

	if val := os.Getenv("GRESS_ERROR_ENABLE_DLQ"); val != "" {
		enabled, err := strconv.ParseBool(val)
		if err != nil {
			return fmt.Errorf("invalid GRESS_ERROR_ENABLE_DLQ: %w", err)
		}
		config.ErrorHandling.EnableDLQ = enabled
	}

	if val := os.Getenv("GRESS_ERROR_DLQ_TYPE"); val != "" {
		config.ErrorHandling.DLQType = val
	}

	if val := os.Getenv("GRESS_ERROR_DLQ_DIRECTORY"); val != "" {
		config.ErrorHandling.DLQDirectory = val
	}

	if val := os.Getenv("GRESS_ERROR_DLQ_KAFKA_TOPIC"); val != "" {
		config.ErrorHandling.DLQKafkaTopic = val
	}

	if val := os.Getenv("GRESS_ERROR_DLQ_KAFKA_BROKERS"); val != "" {
		config.ErrorHandling.DLQKafkaBrokers = strings.Split(val, ",")
	}

	if val := os.Getenv("GRESS_ERROR_ENABLE_CIRCUIT_BREAKER"); val != "" {
		enabled, err := strconv.ParseBool(val)
		if err != nil {
			return fmt.Errorf("invalid GRESS_ERROR_ENABLE_CIRCUIT_BREAKER: %w", err)
		}
		config.ErrorHandling.EnableCircuitBreaker = enabled
	}

	// State backend overrides
	if val := os.Getenv("GRESS_STATE_BACKEND"); val != "" {
		config.State.Backend = val
	}

	if val := os.Getenv("GRESS_STATE_ROCKSDB_PATH"); val != "" {
		config.State.RocksDB.Path = val
	}

	// Metrics overrides
	if val := os.Getenv("GRESS_METRICS_ENABLED"); val != "" {
		enabled, err := strconv.ParseBool(val)
		if err != nil {
			return fmt.Errorf("invalid GRESS_METRICS_ENABLED: %w", err)
		}
		config.Metrics.Enabled = enabled
	}

	if val := os.Getenv("GRESS_METRICS_ADDRESS"); val != "" {
		config.Metrics.Address = val
	}

	if val := os.Getenv("GRESS_METRICS_PATH"); val != "" {
		config.Metrics.Path = val
	}

	if val := os.Getenv("GRESS_METRICS_NAMESPACE"); val != "" {
		config.Metrics.Namespace = val
	}

	// Logging overrides
	if val := os.Getenv("GRESS_LOG_LEVEL"); val != "" {
		config.Logging.Level = val
	}

	if val := os.Getenv("GRESS_LOG_FORMAT"); val != "" {
		config.Logging.Format = val
	}

	if val := os.Getenv("GRESS_LOG_OUTPUT"); val != "" {
		config.Logging.Output = val
	}

	if val := os.Getenv("GRESS_LOG_OUTPUT_PATH"); val != "" {
		config.Logging.OutputPath = val
	}

	// Security overrides
	if val := os.Getenv("GRESS_SECURITY_ENABLE_TLS"); val != "" {
		enabled, err := strconv.ParseBool(val)
		if err != nil {
			return fmt.Errorf("invalid GRESS_SECURITY_ENABLE_TLS: %w", err)
		}
		config.Security.EnableTLS = enabled
	}

	if val := os.Getenv("GRESS_SECURITY_CERT_FILE"); val != "" {
		config.Security.CertFile = val
	}

	if val := os.Getenv("GRESS_SECURITY_KEY_FILE"); val != "" {
		config.Security.KeyFile = val
	}

	if val := os.Getenv("GRESS_SECURITY_CA_FILE"); val != "" {
		config.Security.CAFile = val
	}

	if val := os.Getenv("GRESS_SECURITY_ENABLE_AUTH"); val != "" {
		enabled, err := strconv.ParseBool(val)
		if err != nil {
			return fmt.Errorf("invalid GRESS_SECURITY_ENABLE_AUTH: %w", err)
		}
		config.Security.EnableAuth = enabled
	}

	if val := os.Getenv("GRESS_SECURITY_AUTH_TYPE"); val != "" {
		config.Security.AuthType = val
	}

	return nil
}

// GetEnvWithDefault retrieves an environment variable or returns a default value
func GetEnvWithDefault(key, defaultValue string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultValue
}

// GetEnvInt retrieves an integer environment variable or returns a default value
func GetEnvInt(key string, defaultValue int) int {
	if val := os.Getenv(key); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil {
			return parsed
		}
	}
	return defaultValue
}

// GetEnvBool retrieves a boolean environment variable or returns a default value
func GetEnvBool(key string, defaultValue bool) bool {
	if val := os.Getenv(key); val != "" {
		if parsed, err := strconv.ParseBool(val); err == nil {
			return parsed
		}
	}
	return defaultValue
}

// GetEnvDuration retrieves a duration environment variable or returns a default value
func GetEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if val := os.Getenv(key); val != "" {
		if parsed, err := time.ParseDuration(val); err == nil {
			return parsed
		}
	}
	return defaultValue
}

// LoadConfigWithEnv loads configuration from file and applies environment variable overrides
func LoadConfigWithEnv(path string) (*Config, error) {
	// Load base configuration
	config, err := LoadConfigWithDefaults(path)
	if err != nil {
		return nil, err
	}

	// Apply environment overrides
	if err := ApplyEnvOverrides(config); err != nil {
		return nil, fmt.Errorf("failed to apply environment overrides: %w", err)
	}

	return config, nil
}

// LoadOrDefaultWithEnv loads configuration from file (or uses default) and applies environment overrides
func LoadOrDefaultWithEnv(path string) (*Config, error) {
	// Load base configuration or use default
	config, err := LoadOrDefault(path)
	if err != nil {
		return nil, err
	}

	// Apply environment overrides
	if err := ApplyEnvOverrides(config); err != nil {
		return nil, fmt.Errorf("failed to apply environment overrides: %w", err)
	}

	return config, nil
}

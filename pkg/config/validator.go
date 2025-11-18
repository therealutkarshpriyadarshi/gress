package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ValidationError represents a configuration validation error
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error for %s: %s", e.Field, e.Message)
}

// ValidationErrors represents multiple validation errors
type ValidationErrors struct {
	Errors []ValidationError
}

func (e *ValidationErrors) Error() string {
	if len(e.Errors) == 0 {
		return "no validation errors"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("found %d validation error(s):\n", len(e.Errors)))
	for _, err := range e.Errors {
		sb.WriteString(fmt.Sprintf("  - %s: %s\n", err.Field, err.Message))
	}
	return sb.String()
}

// HasErrors returns true if there are validation errors
func (e *ValidationErrors) HasErrors() bool {
	return len(e.Errors) > 0
}

// Add adds a validation error
func (e *ValidationErrors) Add(field, message string) {
	e.Errors = append(e.Errors, ValidationError{
		Field:   field,
		Message: message,
	})
}

// Validate validates the entire configuration
func Validate(config *Config) error {
	errs := &ValidationErrors{}

	// Validate version
	validateVersion(config, errs)

	// Validate application config
	validateApplication(config, errs)

	// Validate engine config
	validateEngine(config, errs)

	// Validate error handling config
	validateErrorHandling(config, errs)

	// Validate sources
	validateSources(config, errs)

	// Validate sinks
	validateSinks(config, errs)

	// Validate state config
	validateState(config, errs)

	// Validate metrics config
	validateMetrics(config, errs)

	// Validate logging config
	validateLogging(config, errs)

	// Validate security config
	validateSecurity(config, errs)

	if errs.HasErrors() {
		return errs
	}

	return nil
}

func validateVersion(config *Config, errs *ValidationErrors) {
	if config.Version == "" {
		errs.Add("version", "version is required")
		return
	}

	if config.Version != CurrentConfigVersion {
		errs.Add("version", fmt.Sprintf("unsupported version %s (current: %s)", config.Version, CurrentConfigVersion))
	}
}

func validateApplication(config *Config, errs *ValidationErrors) {
	if config.Application.Name == "" {
		errs.Add("application.name", "application name is required")
	}

	if config.Application.Environment != "" {
		validEnvs := []string{"development", "staging", "production", "test"}
		valid := false
		for _, env := range validEnvs {
			if config.Application.Environment == env {
				valid = true
				break
			}
		}
		if !valid {
			errs.Add("application.environment", fmt.Sprintf("invalid environment %s (valid: %s)",
				config.Application.Environment, strings.Join(validEnvs, ", ")))
		}
	}
}

func validateEngine(config *Config, errs *ValidationErrors) {
	if config.Engine.BufferSize <= 0 {
		errs.Add("engine.buffer_size", "buffer size must be positive")
	}

	if config.Engine.MaxConcurrency <= 0 {
		errs.Add("engine.max_concurrency", "max concurrency must be positive")
	}

	if config.Engine.CheckpointInterval <= 0 {
		errs.Add("engine.checkpoint_interval", "checkpoint interval must be positive")
	}

	if config.Engine.WatermarkInterval <= 0 {
		errs.Add("engine.watermark_interval", "watermark interval must be positive")
	}

	if config.Engine.MetricsInterval <= 0 {
		errs.Add("engine.metrics_interval", "metrics interval must be positive")
	}

	if config.Engine.BackpressureThreshold < 0 || config.Engine.BackpressureThreshold > 1 {
		errs.Add("engine.backpressure_threshold", "backpressure threshold must be between 0 and 1")
	}

	if config.Engine.CheckpointDir == "" {
		errs.Add("engine.checkpoint_dir", "checkpoint directory is required")
	}
}

func validateErrorHandling(config *Config, errs *ValidationErrors) {
	validStrategies := []string{"fail-fast", "retry", "send-to-dlq", "retry-then-dlq"}
	valid := false
	for _, strategy := range validStrategies {
		if config.ErrorHandling.Strategy == strategy {
			valid = true
			break
		}
	}
	if !valid {
		errs.Add("error_handling.strategy", fmt.Sprintf("invalid strategy %s (valid: %s)",
			config.ErrorHandling.Strategy, strings.Join(validStrategies, ", ")))
	}

	if config.ErrorHandling.EnableRetry {
		if config.ErrorHandling.MaxRetryAttempts <= 0 {
			errs.Add("error_handling.max_retry_attempts", "max retry attempts must be positive when retry is enabled")
		}

		if config.ErrorHandling.InitialBackoff <= 0 {
			errs.Add("error_handling.initial_backoff", "initial backoff must be positive when retry is enabled")
		}

		if config.ErrorHandling.MaxBackoff <= 0 {
			errs.Add("error_handling.max_backoff", "max backoff must be positive when retry is enabled")
		}

		if config.ErrorHandling.MaxBackoff < config.ErrorHandling.InitialBackoff {
			errs.Add("error_handling.max_backoff", "max backoff must be >= initial backoff")
		}

		if config.ErrorHandling.BackoffMultiplier <= 1 {
			errs.Add("error_handling.backoff_multiplier", "backoff multiplier must be > 1")
		}

		if config.ErrorHandling.BackoffJitter < 0 || config.ErrorHandling.BackoffJitter > 1 {
			errs.Add("error_handling.backoff_jitter", "backoff jitter must be between 0 and 1")
		}
	}

	if config.ErrorHandling.EnableDLQ {
		validDLQTypes := []string{"memory", "file", "kafka"}
		valid := false
		for _, dlqType := range validDLQTypes {
			if config.ErrorHandling.DLQType == dlqType {
				valid = true
				break
			}
		}
		if !valid {
			errs.Add("error_handling.dlq_type", fmt.Sprintf("invalid DLQ type %s (valid: %s)",
				config.ErrorHandling.DLQType, strings.Join(validDLQTypes, ", ")))
		}

		if config.ErrorHandling.DLQMaxSize <= 0 {
			errs.Add("error_handling.dlq_max_size", "DLQ max size must be positive when DLQ is enabled")
		}

		if config.ErrorHandling.DLQType == "file" && config.ErrorHandling.DLQDirectory == "" {
			errs.Add("error_handling.dlq_directory", "DLQ directory is required for file-based DLQ")
		}

		if config.ErrorHandling.DLQType == "kafka" {
			if config.ErrorHandling.DLQKafkaTopic == "" {
				errs.Add("error_handling.dlq_kafka_topic", "DLQ Kafka topic is required for Kafka-based DLQ")
			}
			if len(config.ErrorHandling.DLQKafkaBrokers) == 0 {
				errs.Add("error_handling.dlq_kafka_brokers", "DLQ Kafka brokers are required for Kafka-based DLQ")
			}
		}
	}

	if config.ErrorHandling.EnableCircuitBreaker {
		if config.ErrorHandling.CircuitBreakerConfig.FailureThreshold == 0 {
			errs.Add("error_handling.circuit_breaker.failure_threshold", "failure threshold must be positive when circuit breaker is enabled")
		}

		if config.ErrorHandling.CircuitBreakerConfig.SuccessThreshold == 0 {
			errs.Add("error_handling.circuit_breaker.success_threshold", "success threshold must be positive when circuit breaker is enabled")
		}

		if config.ErrorHandling.CircuitBreakerConfig.Timeout <= 0 {
			errs.Add("error_handling.circuit_breaker.timeout", "timeout must be positive when circuit breaker is enabled")
		}
	}

	if config.ErrorHandling.EnableSideOutputs && config.ErrorHandling.SideOutputBufferSize <= 0 {
		errs.Add("error_handling.side_output_buffer_size", "side output buffer size must be positive when side outputs are enabled")
	}
}

func validateSources(config *Config, errs *ValidationErrors) {
	// Validate Kafka sources
	for i, kafka := range config.Sources.Kafka {
		prefix := fmt.Sprintf("sources.kafka[%d]", i)

		if kafka.Name == "" {
			errs.Add(prefix+".name", "source name is required")
		}

		if len(kafka.Brokers) == 0 {
			errs.Add(prefix+".brokers", "at least one broker is required")
		}

		if len(kafka.Topics) == 0 {
			errs.Add(prefix+".topics", "at least one topic is required")
		}

		if kafka.GroupID == "" {
			errs.Add(prefix+".group_id", "group ID is required")
		}

		if kafka.AutoCommit && kafka.CommitInterval <= 0 {
			errs.Add(prefix+".commit_interval", "commit interval must be positive when auto-commit is enabled")
		}
	}

	// Validate HTTP sources
	for i, http := range config.Sources.HTTP {
		prefix := fmt.Sprintf("sources.http[%d]", i)

		if http.Name == "" {
			errs.Add(prefix+".name", "source name is required")
		}

		if http.Address == "" {
			errs.Add(prefix+".address", "address is required")
		}

		if http.Path == "" {
			errs.Add(prefix+".path", "path is required")
		}

		if http.TLS {
			if http.CertFile == "" {
				errs.Add(prefix+".cert_file", "cert file is required when TLS is enabled")
			}
			if http.KeyFile == "" {
				errs.Add(prefix+".key_file", "key file is required when TLS is enabled")
			}

			// Check if files exist
			if http.CertFile != "" && !fileExists(http.CertFile) {
				errs.Add(prefix+".cert_file", fmt.Sprintf("cert file does not exist: %s", http.CertFile))
			}
			if http.KeyFile != "" && !fileExists(http.KeyFile) {
				errs.Add(prefix+".key_file", fmt.Sprintf("key file does not exist: %s", http.KeyFile))
			}
		}
	}

	// Validate WebSocket sources
	for i, ws := range config.Sources.WebSocket {
		prefix := fmt.Sprintf("sources.websocket[%d]", i)

		if ws.Name == "" {
			errs.Add(prefix+".name", "source name is required")
		}

		if ws.Address == "" {
			errs.Add(prefix+".address", "address is required")
		}

		if ws.Path == "" {
			errs.Add(prefix+".path", "path is required")
		}

		if ws.TLS {
			if ws.CertFile == "" {
				errs.Add(prefix+".cert_file", "cert file is required when TLS is enabled")
			}
			if ws.KeyFile == "" {
				errs.Add(prefix+".key_file", "key file is required when TLS is enabled")
			}

			// Check if files exist
			if ws.CertFile != "" && !fileExists(ws.CertFile) {
				errs.Add(prefix+".cert_file", fmt.Sprintf("cert file does not exist: %s", ws.CertFile))
			}
			if ws.KeyFile != "" && !fileExists(ws.KeyFile) {
				errs.Add(prefix+".key_file", fmt.Sprintf("key file does not exist: %s", ws.KeyFile))
			}
		}
	}
}

func validateSinks(config *Config, errs *ValidationErrors) {
	// Validate Kafka sinks
	for i, kafka := range config.Sinks.Kafka {
		prefix := fmt.Sprintf("sinks.kafka[%d]", i)

		if kafka.Name == "" {
			errs.Add(prefix+".name", "sink name is required")
		}

		if len(kafka.Brokers) == 0 {
			errs.Add(prefix+".brokers", "at least one broker is required")
		}

		if kafka.Topic == "" {
			errs.Add(prefix+".topic", "topic is required")
		}

		if kafka.FlushInterval <= 0 {
			errs.Add(prefix+".flush_interval", "flush interval must be positive")
		}

		if kafka.BatchSize <= 0 {
			errs.Add(prefix+".batch_size", "batch size must be positive")
		}
	}

	// Validate TimescaleDB sinks
	for i, ts := range config.Sinks.TimescaleDB {
		prefix := fmt.Sprintf("sinks.timescaledb[%d]", i)

		if ts.Name == "" {
			errs.Add(prefix+".name", "sink name is required")
		}

		if ts.ConnectionString == "" {
			errs.Add(prefix+".connection_string", "connection string is required")
		}

		if ts.Table == "" {
			errs.Add(prefix+".table", "table name is required")
		}

		if ts.BatchSize <= 0 {
			errs.Add(prefix+".batch_size", "batch size must be positive")
		}

		if ts.FlushInterval <= 0 {
			errs.Add(prefix+".flush_interval", "flush interval must be positive")
		}
	}
}

func validateState(config *Config, errs *ValidationErrors) {
	validBackends := []string{"memory", "rocksdb"}
	valid := false
	for _, backend := range validBackends {
		if config.State.Backend == backend {
			valid = true
			break
		}
	}
	if !valid {
		errs.Add("state.backend", fmt.Sprintf("invalid backend %s (valid: %s)",
			config.State.Backend, strings.Join(validBackends, ", ")))
	}

	if config.State.Backend == "rocksdb" {
		if config.State.RocksDB.Path == "" {
			errs.Add("state.rocksdb.path", "RocksDB path is required")
		}

		if config.State.RocksDB.WriteBufferSize <= 0 {
			errs.Add("state.rocksdb.write_buffer_size", "write buffer size must be positive")
		}

		if config.State.RocksDB.MaxWriteBufferNumber <= 0 {
			errs.Add("state.rocksdb.max_write_buffer_number", "max write buffer number must be positive")
		}

		if config.State.RocksDB.BlockCacheSize <= 0 {
			errs.Add("state.rocksdb.block_cache_size", "block cache size must be positive")
		}
	}
}

func validateMetrics(config *Config, errs *ValidationErrors) {
	if config.Metrics.Enabled {
		if config.Metrics.Address == "" {
			errs.Add("metrics.address", "metrics address is required when metrics are enabled")
		}

		if config.Metrics.Path == "" {
			errs.Add("metrics.path", "metrics path is required when metrics are enabled")
		}

		if config.Metrics.Namespace == "" {
			errs.Add("metrics.namespace", "metrics namespace is required")
		}
	}
}

func validateLogging(config *Config, errs *ValidationErrors) {
	validLevels := []string{"debug", "info", "warn", "error"}
	valid := false
	for _, level := range validLevels {
		if config.Logging.Level == level {
			valid = true
			break
		}
	}
	if !valid {
		errs.Add("logging.level", fmt.Sprintf("invalid log level %s (valid: %s)",
			config.Logging.Level, strings.Join(validLevels, ", ")))
	}

	validFormats := []string{"json", "console"}
	valid = false
	for _, format := range validFormats {
		if config.Logging.Format == format {
			valid = true
			break
		}
	}
	if !valid {
		errs.Add("logging.format", fmt.Sprintf("invalid log format %s (valid: %s)",
			config.Logging.Format, strings.Join(validFormats, ", ")))
	}

	validOutputs := []string{"stdout", "stderr", "file"}
	valid = false
	for _, output := range validOutputs {
		if config.Logging.Output == output {
			valid = true
			break
		}
	}
	if !valid {
		errs.Add("logging.output", fmt.Sprintf("invalid log output %s (valid: %s)",
			config.Logging.Output, strings.Join(validOutputs, ", ")))
	}

	if config.Logging.Output == "file" && config.Logging.OutputPath == "" {
		errs.Add("logging.output_path", "output path is required when output is 'file'")
	}
}

func validateSecurity(config *Config, errs *ValidationErrors) {
	if config.Security.EnableTLS {
		if config.Security.CertFile == "" {
			errs.Add("security.cert_file", "cert file is required when TLS is enabled")
		}
		if config.Security.KeyFile == "" {
			errs.Add("security.key_file", "key file is required when TLS is enabled")
		}

		// Check if files exist
		if config.Security.CertFile != "" && !fileExists(config.Security.CertFile) {
			errs.Add("security.cert_file", fmt.Sprintf("cert file does not exist: %s", config.Security.CertFile))
		}
		if config.Security.KeyFile != "" && !fileExists(config.Security.KeyFile) {
			errs.Add("security.key_file", fmt.Sprintf("key file does not exist: %s", config.Security.KeyFile))
		}
		if config.Security.CAFile != "" && !fileExists(config.Security.CAFile) {
			errs.Add("security.ca_file", fmt.Sprintf("CA file does not exist: %s", config.Security.CAFile))
		}
	}

	if config.Security.EnableAuth {
		validAuthTypes := []string{"none", "basic", "bearer", "mtls"}
		valid := false
		for _, authType := range validAuthTypes {
			if config.Security.AuthType == authType {
				valid = true
				break
			}
		}
		if !valid {
			errs.Add("security.auth_type", fmt.Sprintf("invalid auth type %s (valid: %s)",
				config.Security.AuthType, strings.Join(validAuthTypes, ", ")))
		}
	}
}

// ValidateAndLoad loads and validates a configuration file
func ValidateAndLoad(path string) (*Config, error) {
	config, err := LoadConfigWithEnv(path)
	if err != nil {
		return nil, err
	}

	if err := Validate(config); err != nil {
		return nil, err
	}

	return config, nil
}

// fileExists checks if a file exists
func fileExists(path string) bool {
	// Expand home directory
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			path = filepath.Join(home, path[2:])
		}
	}

	_, err := os.Stat(path)
	return err == nil
}

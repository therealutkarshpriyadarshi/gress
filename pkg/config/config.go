package config

import (
	"time"
)

// Version represents the configuration file version
const (
	CurrentConfigVersion = "v1"
)

// Config represents the complete gress configuration
type Config struct {
	// Version of the configuration schema
	Version string `yaml:"version" json:"version"`

	// Application metadata
	Application ApplicationConfig `yaml:"application" json:"application"`

	// Engine configuration
	Engine EngineConfig `yaml:"engine" json:"engine"`

	// Error handling configuration
	ErrorHandling ErrorHandlingConfig `yaml:"error_handling" json:"error_handling"`

	// Sources configuration
	Sources SourcesConfig `yaml:"sources" json:"sources"`

	// Sinks configuration
	Sinks SinksConfig `yaml:"sinks" json:"sinks"`

	// State backend configuration
	State StateConfig `yaml:"state" json:"state"`

	// Metrics and monitoring configuration
	Metrics MetricsConfig `yaml:"metrics" json:"metrics"`

	// Logging configuration
	Logging LoggingConfig `yaml:"logging" json:"logging"`

	// Security configuration
	Security SecurityConfig `yaml:"security" json:"security"`
}

// ApplicationConfig holds application-level metadata
type ApplicationConfig struct {
	Name        string            `yaml:"name" json:"name"`
	Environment string            `yaml:"environment" json:"environment"` // development, staging, production
	Tags        map[string]string `yaml:"tags" json:"tags"`
}

// EngineConfig holds stream processing engine configuration
type EngineConfig struct {
	BufferSize            int           `yaml:"buffer_size" json:"buffer_size"`
	MaxConcurrency        int           `yaml:"max_concurrency" json:"max_concurrency"`
	CheckpointInterval    time.Duration `yaml:"checkpoint_interval" json:"checkpoint_interval"`
	WatermarkInterval     time.Duration `yaml:"watermark_interval" json:"watermark_interval"`
	MetricsInterval       time.Duration `yaml:"metrics_interval" json:"metrics_interval"`
	EnableBackpressure    bool          `yaml:"enable_backpressure" json:"enable_backpressure"`
	BackpressureThreshold float64       `yaml:"backpressure_threshold" json:"backpressure_threshold"`
	CheckpointDir         string        `yaml:"checkpoint_dir" json:"checkpoint_dir"`
}

// ErrorHandlingConfig holds error handling configuration
type ErrorHandlingConfig struct {
	Strategy              string        `yaml:"strategy" json:"strategy"` // fail-fast, retry, retry-then-dlq, send-to-dlq
	EnableRetry           bool          `yaml:"enable_retry" json:"enable_retry"`
	MaxRetryAttempts      int           `yaml:"max_retry_attempts" json:"max_retry_attempts"`
	InitialBackoff        time.Duration `yaml:"initial_backoff" json:"initial_backoff"`
	MaxBackoff            time.Duration `yaml:"max_backoff" json:"max_backoff"`
	BackoffMultiplier     float64       `yaml:"backoff_multiplier" json:"backoff_multiplier"`
	BackoffJitter         float64       `yaml:"backoff_jitter" json:"backoff_jitter"`
	EnableDLQ             bool          `yaml:"enable_dlq" json:"enable_dlq"`
	DLQMaxSize            int           `yaml:"dlq_max_size" json:"dlq_max_size"`
	DLQType               string        `yaml:"dlq_type" json:"dlq_type"` // memory, file, kafka
	DLQDirectory          string        `yaml:"dlq_directory" json:"dlq_directory"`
	DLQKafkaTopic         string        `yaml:"dlq_kafka_topic" json:"dlq_kafka_topic"`
	DLQKafkaBrokers       []string      `yaml:"dlq_kafka_brokers" json:"dlq_kafka_brokers"`
	EnableCircuitBreaker  bool          `yaml:"enable_circuit_breaker" json:"enable_circuit_breaker"`
	CircuitBreakerConfig  CircuitBreakerConfig `yaml:"circuit_breaker" json:"circuit_breaker"`
	EnableSideOutputs     bool          `yaml:"enable_side_outputs" json:"enable_side_outputs"`
	SideOutputBufferSize  int           `yaml:"side_output_buffer_size" json:"side_output_buffer_size"`
}

// CircuitBreakerConfig holds circuit breaker configuration
type CircuitBreakerConfig struct {
	FailureThreshold uint32        `yaml:"failure_threshold" json:"failure_threshold"`
	SuccessThreshold uint32        `yaml:"success_threshold" json:"success_threshold"`
	Timeout          time.Duration `yaml:"timeout" json:"timeout"`
}

// SourcesConfig holds data source configurations
type SourcesConfig struct {
	Kafka     []KafkaSourceConfig     `yaml:"kafka" json:"kafka"`
	HTTP      []HTTPSourceConfig      `yaml:"http" json:"http"`
	WebSocket []WebSocketSourceConfig `yaml:"websocket" json:"websocket"`
}

// KafkaSourceConfig holds Kafka source configuration
type KafkaSourceConfig struct {
	Name          string   `yaml:"name" json:"name"`
	Brokers       []string `yaml:"brokers" json:"brokers"`
	Topics        []string `yaml:"topics" json:"topics"`
	GroupID       string   `yaml:"group_id" json:"group_id"`
	AutoCommit    bool     `yaml:"auto_commit" json:"auto_commit"`
	CommitInterval time.Duration `yaml:"commit_interval" json:"commit_interval"`
}

// HTTPSourceConfig holds HTTP source configuration
type HTTPSourceConfig struct {
	Name     string `yaml:"name" json:"name"`
	Address  string `yaml:"address" json:"address"`
	Path     string `yaml:"path" json:"path"`
	TLS      bool   `yaml:"tls" json:"tls"`
	CertFile string `yaml:"cert_file" json:"cert_file"`
	KeyFile  string `yaml:"key_file" json:"key_file"`
}

// WebSocketSourceConfig holds WebSocket source configuration
type WebSocketSourceConfig struct {
	Name     string `yaml:"name" json:"name"`
	Address  string `yaml:"address" json:"address"`
	Path     string `yaml:"path" json:"path"`
	TLS      bool   `yaml:"tls" json:"tls"`
	CertFile string `yaml:"cert_file" json:"cert_file"`
	KeyFile  string `yaml:"key_file" json:"key_file"`
}

// SinksConfig holds data sink configurations
type SinksConfig struct {
	Kafka     []KafkaSinkConfig     `yaml:"kafka" json:"kafka"`
	TimescaleDB []TimescaleSinkConfig `yaml:"timescaledb" json:"timescaledb"`
}

// KafkaSinkConfig holds Kafka sink configuration
type KafkaSinkConfig struct {
	Name          string   `yaml:"name" json:"name"`
	Brokers       []string `yaml:"brokers" json:"brokers"`
	Topic         string   `yaml:"topic" json:"topic"`
	FlushInterval time.Duration `yaml:"flush_interval" json:"flush_interval"`
	BatchSize     int      `yaml:"batch_size" json:"batch_size"`
}

// TimescaleSinkConfig holds TimescaleDB sink configuration
type TimescaleSinkConfig struct {
	Name            string        `yaml:"name" json:"name"`
	ConnectionString string        `yaml:"connection_string" json:"connection_string"`
	Table           string        `yaml:"table" json:"table"`
	BatchSize       int           `yaml:"batch_size" json:"batch_size"`
	FlushInterval   time.Duration `yaml:"flush_interval" json:"flush_interval"`
}

// StateConfig holds state backend configuration
type StateConfig struct {
	Backend  string `yaml:"backend" json:"backend"` // memory, rocksdb
	RocksDB  RocksDBConfig `yaml:"rocksdb" json:"rocksdb"`
}

// RocksDBConfig holds RocksDB-specific configuration
type RocksDBConfig struct {
	Path                 string `yaml:"path" json:"path"`
	WriteBufferSize      int    `yaml:"write_buffer_size" json:"write_buffer_size"`
	MaxWriteBufferNumber int    `yaml:"max_write_buffer_number" json:"max_write_buffer_number"`
	BlockCacheSize       int64  `yaml:"block_cache_size" json:"block_cache_size"`
}

// MetricsConfig holds metrics and monitoring configuration
type MetricsConfig struct {
	Enabled    bool   `yaml:"enabled" json:"enabled"`
	Address    string `yaml:"address" json:"address"`
	Path       string `yaml:"path" json:"path"`
	Namespace  string `yaml:"namespace" json:"namespace"`
	Subsystem  string `yaml:"subsystem" json:"subsystem"`
}

// LoggingConfig holds logging configuration
type LoggingConfig struct {
	Level      string `yaml:"level" json:"level"` // debug, info, warn, error
	Format     string `yaml:"format" json:"format"` // json, console
	Output     string `yaml:"output" json:"output"` // stdout, stderr, file
	OutputPath string `yaml:"output_path" json:"output_path"`
}

// SecurityConfig holds security-related configuration
type SecurityConfig struct {
	EnableTLS       bool   `yaml:"enable_tls" json:"enable_tls"`
	CertFile        string `yaml:"cert_file" json:"cert_file"`
	KeyFile         string `yaml:"key_file" json:"key_file"`
	CAFile          string `yaml:"ca_file" json:"ca_file"`
	EnableAuth      bool   `yaml:"enable_auth" json:"enable_auth"`
	AuthType        string `yaml:"auth_type" json:"auth_type"` // none, basic, bearer, mtls
}


// DefaultConfig returns a default configuration
func DefaultConfig() *Config {
	return &Config{
		Version: CurrentConfigVersion,
		Application: ApplicationConfig{
			Name:        "gress-app",
			Environment: "development",
			Tags:        make(map[string]string),
		},
		Engine: EngineConfig{
			BufferSize:            10000,
			MaxConcurrency:        100,
			CheckpointInterval:    30 * time.Second,
			WatermarkInterval:     5 * time.Second,
			MetricsInterval:       10 * time.Second,
			EnableBackpressure:    true,
			BackpressureThreshold: 0.8,
			CheckpointDir:         "./checkpoints",
		},
		ErrorHandling: ErrorHandlingConfig{
			Strategy:              "retry-then-dlq",
			EnableRetry:           true,
			MaxRetryAttempts:      3,
			InitialBackoff:        100 * time.Millisecond,
			MaxBackoff:            30 * time.Second,
			BackoffMultiplier:     2.0,
			BackoffJitter:         0.1,
			EnableDLQ:             true,
			DLQMaxSize:            10000,
			DLQType:               "memory",
			EnableCircuitBreaker:  true,
			CircuitBreakerConfig: CircuitBreakerConfig{
				FailureThreshold: 5,
				SuccessThreshold: 2,
				Timeout:          60 * time.Second,
			},
			EnableSideOutputs:    true,
			SideOutputBufferSize: 1000,
		},
		Sources: SourcesConfig{
			Kafka:     []KafkaSourceConfig{},
			HTTP:      []HTTPSourceConfig{},
			WebSocket: []WebSocketSourceConfig{},
		},
		Sinks: SinksConfig{
			Kafka:       []KafkaSinkConfig{},
			TimescaleDB: []TimescaleSinkConfig{},
		},
		State: StateConfig{
			Backend: "memory",
			RocksDB: RocksDBConfig{
				Path:                 "./data/rocksdb",
				WriteBufferSize:      64 * 1024 * 1024,
				MaxWriteBufferNumber: 3,
				BlockCacheSize:       512 * 1024 * 1024,
			},
		},
		Metrics: MetricsConfig{
			Enabled:   true,
			Address:   ":9091",
			Path:      "/metrics",
			Namespace: "gress",
			Subsystem: "stream",
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
			Output: "stdout",
		},
		Security: SecurityConfig{
			EnableTLS:  false,
			EnableAuth: false,
			AuthType:   "none",
		},
	}
}

// ProductionConfig returns a production-ready configuration
func ProductionConfig() *Config {
	config := DefaultConfig()
	config.Application.Environment = "production"
	config.Engine.BufferSize = 50000
	config.Engine.MaxConcurrency = 500
	config.ErrorHandling.MaxRetryAttempts = 5
	config.ErrorHandling.MaxBackoff = 60 * time.Second
	config.ErrorHandling.DLQMaxSize = 100000
	config.ErrorHandling.CircuitBreakerConfig.FailureThreshold = 10
	config.ErrorHandling.CircuitBreakerConfig.Timeout = 120 * time.Second
	config.Logging.Level = "warn"
	return config
}

// DevelopmentConfig returns a development-friendly configuration
func DevelopmentConfig() *Config {
	config := DefaultConfig()
	config.Application.Environment = "development"
	config.ErrorHandling.Strategy = "fail-fast"
	config.ErrorHandling.EnableRetry = false
	config.ErrorHandling.EnableCircuitBreaker = false
	config.Logging.Level = "debug"
	config.Logging.Format = "console"
	return config
}

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

	// Distributed tracing configuration
	Tracing TracingConfig `yaml:"tracing" json:"tracing"`

	// Logging configuration
	Logging LoggingConfig `yaml:"logging" json:"logging"`

	// Security configuration
	Security SecurityConfig `yaml:"security" json:"security"`

	// Schema Registry configuration
	SchemaRegistry SchemaRegistryConfig `yaml:"schema_registry" json:"schema_registry"`

	// Machine Learning configuration
	ML MLConfig `yaml:"ml" json:"ml"`
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

	// Schema configuration
	ValueSchema *SourceSchemaConfig `yaml:"value_schema,omitempty" json:"value_schema,omitempty"`
	KeySchema   *SourceSchemaConfig `yaml:"key_schema,omitempty" json:"key_schema,omitempty"`
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

	// Schema configuration
	ValueSchema *SinkSchemaConfig `yaml:"value_schema,omitempty" json:"value_schema,omitempty"`
	KeySchema   *SinkSchemaConfig `yaml:"key_schema,omitempty" json:"key_schema,omitempty"`
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

	// Performance preset: low-latency, high-throughput, balanced, large-state
	// Overrides other settings if specified
	Preset                    string `yaml:"preset" json:"preset"`

	// Advanced performance tuning
	MaxBackgroundJobs         int   `yaml:"max_background_jobs" json:"max_background_jobs"`
	MaxSubCompactions         int   `yaml:"max_sub_compactions" json:"max_sub_compactions"`
	OptimizeFiltersForHits    bool  `yaml:"optimize_filters_for_hits" json:"optimize_filters_for_hits"`
	Level0FileNumCompactionTrigger int `yaml:"level0_file_num_compaction_trigger" json:"level0_file_num_compaction_trigger"`
	TargetFileSizeBase        int64 `yaml:"target_file_size_base" json:"target_file_size_base"`
	MaxBytesForLevelBase      int64 `yaml:"max_bytes_for_level_base" json:"max_bytes_for_level_base"`

	// Latency optimization
	AllowMmapReads            bool  `yaml:"allow_mmap_reads" json:"allow_mmap_reads"`
	UseDirectReads            bool  `yaml:"use_direct_reads" json:"use_direct_reads"`

	// Metrics and monitoring
	EnableLatencyTracking     bool  `yaml:"enable_latency_tracking" json:"enable_latency_tracking"`
}

// MetricsConfig holds metrics and monitoring configuration
type MetricsConfig struct {
	Enabled    bool   `yaml:"enabled" json:"enabled"`
	Address    string `yaml:"address" json:"address"`
	Path       string `yaml:"path" json:"path"`
	Namespace  string `yaml:"namespace" json:"namespace"`
	Subsystem  string `yaml:"subsystem" json:"subsystem"`
}

// TracingConfig holds distributed tracing configuration
type TracingConfig struct {
	Enabled          bool    `yaml:"enabled" json:"enabled"`
	ServiceName      string  `yaml:"service_name" json:"service_name"`
	ServiceVersion   string  `yaml:"service_version" json:"service_version"`
	Environment      string  `yaml:"environment" json:"environment"`
	SamplingRate     float64 `yaml:"sampling_rate" json:"sampling_rate"`
	ExporterType     string  `yaml:"exporter_type" json:"exporter_type"` // "jaeger", "otlp", "stdout"
	ExporterEndpoint string  `yaml:"exporter_endpoint" json:"exporter_endpoint"`
	OTLPHeaders      map[string]string `yaml:"otlp_headers" json:"otlp_headers"`
	OTLPInsecure     bool    `yaml:"otlp_insecure" json:"otlp_insecure"`
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

// SchemaRegistryConfig holds schema registry configuration
type SchemaRegistryConfig struct {
	// Enabled enables schema registry integration
	Enabled bool `yaml:"enabled" json:"enabled"`

	// URL is the schema registry URL
	URL string `yaml:"url" json:"url"`

	// Authentication
	Username string `yaml:"username,omitempty" json:"username,omitempty"`
	Password string `yaml:"password,omitempty" json:"password,omitempty"`

	// Timeouts and caching
	Timeout      time.Duration `yaml:"timeout" json:"timeout"`
	CacheEnabled bool          `yaml:"cache_enabled" json:"cache_enabled"`
	CacheSize    int           `yaml:"cache_size" json:"cache_size"`
	CacheTTL     time.Duration `yaml:"cache_ttl" json:"cache_ttl"`

	// TLS configuration
	TLSEnabled    bool   `yaml:"tls_enabled" json:"tls_enabled"`
	TLSCertPath   string `yaml:"tls_cert_path,omitempty" json:"tls_cert_path,omitempty"`
	TLSKeyPath    string `yaml:"tls_key_path,omitempty" json:"tls_key_path,omitempty"`
	TLSCAPath     string `yaml:"tls_ca_path,omitempty" json:"tls_ca_path,omitempty"`
	TLSSkipVerify bool   `yaml:"tls_skip_verify" json:"tls_skip_verify"`
}

// SourceSchemaConfig holds schema configuration for data sources
type SourceSchemaConfig struct {
	// Subject is the schema registry subject name
	Subject string `yaml:"subject" json:"subject"`

	// SchemaType is the type of schema (AVRO, PROTOBUF, JSON)
	SchemaType string `yaml:"schema_type" json:"schema_type"`

	// Version is the specific version to use (0 for latest)
	Version int `yaml:"version" json:"version"`

	// ValidateOnRead validates data on deserialization
	ValidateOnRead bool `yaml:"validate_on_read" json:"validate_on_read"`
}

// SinkSchemaConfig holds schema configuration for data sinks
type SinkSchemaConfig struct {
	// Subject is the schema registry subject name
	Subject string `yaml:"subject" json:"subject"`

	// SchemaType is the type of schema (AVRO, PROTOBUF, JSON)
	SchemaType string `yaml:"schema_type" json:"schema_type"`

	// Version is the specific version to use (0 for latest)
	Version int `yaml:"version" json:"version"`

	// AutoRegister automatically registers schemas if they don't exist
	AutoRegister bool `yaml:"auto_register" json:"auto_register"`

	// Compatibility is the compatibility mode for the subject
	Compatibility string `yaml:"compatibility" json:"compatibility"`

	// ValidateOnWrite validates data on serialization
	ValidateOnWrite bool `yaml:"validate_on_write" json:"validate_on_write"`
}

// MLConfig holds machine learning configuration
type MLConfig struct {
	// Enabled enables ML integration
	Enabled bool `yaml:"enabled" json:"enabled"`

	// ModelRegistry holds model registry configuration
	ModelRegistry ModelRegistryConfig `yaml:"model_registry" json:"model_registry"`

	// Models holds model configurations
	Models []ModelConfig `yaml:"models" json:"models"`

	// FeatureStore holds feature store configuration
	FeatureStore FeatureStoreConfig `yaml:"feature_store" json:"feature_store"`

	// Inference holds inference configuration
	Inference InferenceConfigSettings `yaml:"inference" json:"inference"`

	// OnlineLearning holds online learning configuration
	OnlineLearning OnlineLearningConfigSettings `yaml:"online_learning" json:"online_learning"`

	// ABTesting holds A/B testing configuration
	ABTesting ABTestingConfig `yaml:"ab_testing" json:"ab_testing"`
}

// ModelRegistryConfig holds model registry configuration
type ModelRegistryConfig struct {
	// Type of model registry (local, remote, s3, gcs)
	Type string `yaml:"type" json:"type"`

	// BasePath is the base path for local model storage
	BasePath string `yaml:"base_path" json:"base_path"`

	// S3 configuration
	S3Bucket    string `yaml:"s3_bucket" json:"s3_bucket"`
	S3Region    string `yaml:"s3_region" json:"s3_region"`
	S3Endpoint  string `yaml:"s3_endpoint" json:"s3_endpoint"`
	S3AccessKey string `yaml:"s3_access_key" json:"s3_access_key"`
	S3SecretKey string `yaml:"s3_secret_key" json:"s3_secret_key"`

	// GCS configuration
	GCSBucket      string `yaml:"gcs_bucket" json:"gcs_bucket"`
	GCSProject     string `yaml:"gcs_project" json:"gcs_project"`
	GCSCredentials string `yaml:"gcs_credentials" json:"gcs_credentials"`
}

// ModelConfig holds model configuration
type ModelConfig struct {
	Name           string            `yaml:"name" json:"name"`
	Version        string            `yaml:"version" json:"version"`
	Type           string            `yaml:"type" json:"type"` // onnx, tensorflow, sklearn
	Path           string            `yaml:"path" json:"path"`
	Tags           map[string]string `yaml:"tags" json:"tags"`
	Description    string            `yaml:"description" json:"description"`
	Enabled        bool              `yaml:"enabled" json:"enabled"`
	WarmupOnLoad   bool              `yaml:"warmup_on_load" json:"warmup_on_load"`
}

// FeatureStoreConfig holds feature store configuration
type FeatureStoreConfig struct {
	// Enabled enables feature store
	Enabled bool `yaml:"enabled" json:"enabled"`

	// Backend type (state, redis, dynamodb)
	Backend string `yaml:"backend" json:"backend"`

	// TTL for cached features
	TTL time.Duration `yaml:"ttl" json:"ttl"`

	// Redis configuration
	RedisAddr     string `yaml:"redis_addr" json:"redis_addr"`
	RedisPassword string `yaml:"redis_password" json:"redis_password"`
	RedisDB       int    `yaml:"redis_db" json:"redis_db"`

	// DynamoDB configuration
	DynamoDBTable  string `yaml:"dynamodb_table" json:"dynamodb_table"`
	DynamoDBRegion string `yaml:"dynamodb_region" json:"dynamodb_region"`
}

// InferenceConfigSettings holds inference configuration
type InferenceConfigSettings struct {
	// Default batch size for inference
	DefaultBatchSize int `yaml:"default_batch_size" json:"default_batch_size"`

	// Default batch timeout
	DefaultBatchTimeout time.Duration `yaml:"default_batch_timeout" json:"default_batch_timeout"`

	// Max concurrency for inference
	MaxConcurrency int `yaml:"max_concurrency" json:"max_concurrency"`

	// Max latency threshold
	MaxLatency time.Duration `yaml:"max_latency" json:"max_latency"`

	// Enable GPU support
	EnableGPU bool `yaml:"enable_gpu" json:"enable_gpu"`

	// GPU device IDs
	GPUDeviceIDs []int `yaml:"gpu_device_ids" json:"gpu_device_ids"`
}

// OnlineLearningConfigSettings holds online learning configuration
type OnlineLearningConfigSettings struct {
	// Enabled enables online learning
	Enabled bool `yaml:"enabled" json:"enabled"`

	// Default update batch size
	DefaultUpdateBatchSize int `yaml:"default_update_batch_size" json:"default_update_batch_size"`

	// Default update interval
	DefaultUpdateInterval time.Duration `yaml:"default_update_interval" json:"default_update_interval"`

	// Default checkpoint interval
	DefaultCheckpointInterval time.Duration `yaml:"default_checkpoint_interval" json:"default_checkpoint_interval"`

	// Checkpoint directory
	CheckpointDir string `yaml:"checkpoint_dir" json:"checkpoint_dir"`

	// Default learning rate
	DefaultLearningRate float64 `yaml:"default_learning_rate" json:"default_learning_rate"`

	// Enable validation
	EnableValidation bool `yaml:"enable_validation" json:"enable_validation"`

	// Validation split
	ValidationSplit float64 `yaml:"validation_split" json:"validation_split"`
}

// ABTestingConfig holds A/B testing configuration
type ABTestingConfig struct {
	// Enabled enables A/B testing
	Enabled bool `yaml:"enabled" json:"enabled"`

	// Experiments holds experiment configurations
	Experiments []ExperimentConfig `yaml:"experiments" json:"experiments"`
}

// ExperimentConfig holds experiment configuration
type ExperimentConfig struct {
	Name              string                   `yaml:"name" json:"name"`
	Strategy          string                   `yaml:"strategy" json:"strategy"` // random, hash, percentage, user_id
	Variants          []VariantConfig          `yaml:"variants" json:"variants"`
	SplitKey          string                   `yaml:"split_key" json:"split_key"`
	DefaultVariant    string                   `yaml:"default_variant" json:"default_variant"`
	EnableShadowMode  bool                     `yaml:"enable_shadow_mode" json:"enable_shadow_mode"`
	CollectComparison bool                     `yaml:"collect_comparison" json:"collect_comparison"`
	Enabled           bool                     `yaml:"enabled" json:"enabled"`
}

// VariantConfig holds variant configuration
type VariantConfig struct {
	Name         string  `yaml:"name" json:"name"`
	ModelName    string  `yaml:"model_name" json:"model_name"`
	ModelVersion string  `yaml:"model_version" json:"model_version"`
	Weight       float64 `yaml:"weight" json:"weight"`
	Enabled      bool    `yaml:"enabled" json:"enabled"`
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
		Tracing: TracingConfig{
			Enabled:          false,
			ServiceName:      "gress",
			ServiceVersion:   "1.0.0",
			Environment:      "development",
			SamplingRate:     1.0,
			ExporterType:     "otlp",
			ExporterEndpoint: "http://localhost:4318/v1/traces",
			OTLPHeaders:      make(map[string]string),
			OTLPInsecure:     true,
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
		SchemaRegistry: SchemaRegistryConfig{
			Enabled:      false,
			URL:          "http://localhost:8081",
			Timeout:      30 * time.Second,
			CacheEnabled: true,
			CacheSize:    1000,
			CacheTTL:     5 * time.Minute,
			TLSEnabled:   false,
			TLSSkipVerify: false,
		},
		ML: MLConfig{
			Enabled: false,
			ModelRegistry: ModelRegistryConfig{
				Type:     "local",
				BasePath: "./models",
			},
			Models: []ModelConfig{},
			FeatureStore: FeatureStoreConfig{
				Enabled: false,
				Backend: "state",
				TTL:     5 * time.Minute,
			},
			Inference: InferenceConfigSettings{
				DefaultBatchSize:    10,
				DefaultBatchTimeout: 10 * time.Millisecond,
				MaxConcurrency:      10,
				MaxLatency:          100 * time.Millisecond,
				EnableGPU:           false,
				GPUDeviceIDs:        []int{},
			},
			OnlineLearning: OnlineLearningConfigSettings{
				Enabled:                   false,
				DefaultUpdateBatchSize:    32,
				DefaultUpdateInterval:     5 * time.Second,
				DefaultCheckpointInterval: 5 * time.Minute,
				CheckpointDir:             "./ml-checkpoints",
				DefaultLearningRate:       0.001,
				EnableValidation:          true,
				ValidationSplit:           0.2,
			},
			ABTesting: ABTestingConfig{
				Enabled:     false,
				Experiments: []ExperimentConfig{},
			},
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

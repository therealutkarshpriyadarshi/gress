package stream

import (
	"time"

	"github.com/therealutkarshpriyadarshi/gress/pkg/errors"
)

// ErrorHandlingStrategy defines how errors should be handled
type ErrorHandlingStrategy int

const (
	// FailFast - fail immediately on errors
	FailFast ErrorHandlingStrategy = iota
	// RetryWithBackoff - retry with exponential backoff
	RetryWithBackoff
	// SendToDLQ - send failed events to DLQ
	SendToDLQ
	// RetryThenDLQ - retry first, then send to DLQ if still failing
	RetryThenDLQ
)

// EngineErrorConfig configures error handling for the entire engine
type EngineErrorConfig struct {
	// Global strategy for error handling
	Strategy ErrorHandlingStrategy

	// Retry configuration
	EnableRetry        bool
	MaxRetryAttempts   int
	InitialBackoff     time.Duration
	MaxBackoff         time.Duration
	BackoffMultiplier  float64
	BackoffJitter      float64

	// Dead Letter Queue configuration
	EnableDLQ         bool
	DLQMaxSize        int
	DLQType           string // "memory", "file", "kafka"
	DLQDirectory      string // for file-based DLQ
	DLQKafkaTopic     string // for Kafka-based DLQ
	DLQKafkaBrokers   []string

	// Circuit breaker configuration
	EnableCircuitBreaker       bool
	CircuitBreakerThreshold    uint32
	CircuitBreakerTimeout      time.Duration
	CircuitBreakerSuccessCount uint32

	// Side output configuration
	EnableSideOutputs        bool
	EnableErrorSideOutput    bool
	EnableLateSideOutput     bool
	EnableFilteredSideOutput bool
	SideOutputBufferSize     int

	// Per-operator configuration overrides
	OperatorConfigs map[string]*OperatorErrorConfig
}

// DefaultEngineErrorConfig returns sensible defaults
func DefaultEngineErrorConfig() *EngineErrorConfig {
	return &EngineErrorConfig{
		Strategy:                   RetryThenDLQ,
		EnableRetry:                true,
		MaxRetryAttempts:           3,
		InitialBackoff:             100 * time.Millisecond,
		MaxBackoff:                 30 * time.Second,
		BackoffMultiplier:          2.0,
		BackoffJitter:              0.1,
		EnableDLQ:                  true,
		DLQMaxSize:                 10000,
		DLQType:                    "memory",
		EnableCircuitBreaker:       true,
		CircuitBreakerThreshold:    5,
		CircuitBreakerTimeout:      60 * time.Second,
		CircuitBreakerSuccessCount: 2,
		EnableSideOutputs:          true,
		EnableErrorSideOutput:      true,
		EnableLateSideOutput:       false,
		EnableFilteredSideOutput:   false,
		SideOutputBufferSize:       1000,
		OperatorConfigs:            make(map[string]*OperatorErrorConfig),
	}
}

// ProductionEngineErrorConfig returns production-ready configuration
func ProductionEngineErrorConfig() *EngineErrorConfig {
	return &EngineErrorConfig{
		Strategy:                   RetryThenDLQ,
		EnableRetry:                true,
		MaxRetryAttempts:           5,
		InitialBackoff:             200 * time.Millisecond,
		MaxBackoff:                 60 * time.Second,
		BackoffMultiplier:          2.0,
		BackoffJitter:              0.2,
		EnableDLQ:                  true,
		DLQMaxSize:                 100000,
		DLQType:                    "kafka",
		DLQKafkaTopic:              "gress-dlq",
		EnableCircuitBreaker:       true,
		CircuitBreakerThreshold:    10,
		CircuitBreakerTimeout:      120 * time.Second,
		CircuitBreakerSuccessCount: 3,
		EnableSideOutputs:          true,
		EnableErrorSideOutput:      true,
		EnableLateSideOutput:       true,
		EnableFilteredSideOutput:   false,
		SideOutputBufferSize:       5000,
		OperatorConfigs:            make(map[string]*OperatorErrorConfig),
	}
}

// DevelopmentEngineErrorConfig returns development-friendly configuration
func DevelopmentEngineErrorConfig() *EngineErrorConfig {
	return &EngineErrorConfig{
		Strategy:                   FailFast,
		EnableRetry:                false,
		MaxRetryAttempts:           0,
		EnableDLQ:                  false,
		EnableCircuitBreaker:       false,
		EnableSideOutputs:          true,
		EnableErrorSideOutput:      true,
		EnableLateSideOutput:       false,
		EnableFilteredSideOutput:   true,
		SideOutputBufferSize:       100,
		OperatorConfigs:            make(map[string]*OperatorErrorConfig),
	}
}

// OperatorErrorConfig configures error handling for a specific operator
type OperatorErrorConfig struct {
	// Override global strategy
	Strategy *ErrorHandlingStrategy

	// Retry configuration
	RetryPolicy *errors.RetryPolicy

	// DLQ configuration
	EnableDLQ bool
	DLQ       errors.DeadLetterQueue

	// Side outputs
	EnableSideOutputs bool

	// Circuit breaker (for sinks)
	CircuitBreakerConfig *errors.CircuitBreakerConfig
}

// CreateRetryPolicy creates a retry policy from engine config
func (c *EngineErrorConfig) CreateRetryPolicy() *errors.RetryPolicy {
	if !c.EnableRetry {
		return errors.NoRetryPolicy()
	}

	return &errors.RetryPolicy{
		MaxAttempts:       c.MaxRetryAttempts,
		InitialBackoff:    c.InitialBackoff,
		MaxBackoff:        c.MaxBackoff,
		BackoffMultiplier: c.BackoffMultiplier,
		Jitter:            c.BackoffJitter,
		RetriableFunc:     errors.IsRetriable,
	}
}

// CreateDLQ creates a DLQ based on configuration
func (c *EngineErrorConfig) CreateDLQ() errors.DeadLetterQueue {
	if !c.EnableDLQ {
		return errors.NewNullDLQ()
	}

	switch c.DLQType {
	case "memory":
		return errors.NewInMemoryDLQ(c.DLQMaxSize)
	case "file":
		// File-based DLQ would be implemented here
		dlq, err := errors.NewFileDLQ(c.DLQDirectory)
		if err != nil {
			return errors.NewNullDLQ()
		}
		return dlq
	case "kafka":
		// Kafka-based DLQ would be implemented here
		// For now, fall back to memory
		return errors.NewInMemoryDLQ(c.DLQMaxSize)
	default:
		return errors.NewInMemoryDLQ(c.DLQMaxSize)
	}
}

// CreateSideOutputCollector creates a side output collector from config
func (c *EngineErrorConfig) CreateSideOutputCollector() *errors.SideOutputCollector {
	if !c.EnableSideOutputs {
		return errors.NewSideOutputCollector()
	}

	config := &errors.SideOutputConfig{
		EnableErrorSideOutput:    c.EnableErrorSideOutput,
		EnableLateSideOutput:     c.EnableLateSideOutput,
		EnableFilteredSideOutput: c.EnableFilteredSideOutput,
		ErrorChannelSize:         c.SideOutputBufferSize,
		LateChannelSize:          c.SideOutputBufferSize,
		FilteredChannelSize:      c.SideOutputBufferSize,
	}

	return errors.CreateSideOutputCollector(config)
}

// CreateCircuitBreakerConfig creates a circuit breaker config for a sink
func (c *EngineErrorConfig) CreateCircuitBreakerConfig(name string) *errors.CircuitBreakerConfig {
	if !c.EnableCircuitBreaker {
		return &errors.CircuitBreakerConfig{
			Name:             name,
			FailureThreshold: 999999, // Effectively disabled
		}
	}

	return &errors.CircuitBreakerConfig{
		Name:                  name,
		FailureThreshold:      c.CircuitBreakerThreshold,
		SuccessThreshold:      c.CircuitBreakerSuccessCount,
		Timeout:               c.CircuitBreakerTimeout,
		MaxConcurrentRequests: 1,
	}
}

// GetOperatorConfig gets the error config for a specific operator
func (c *EngineErrorConfig) GetOperatorConfig(operatorName string) *OperatorErrorConfig {
	if config, exists := c.OperatorConfigs[operatorName]; exists {
		return config
	}

	// Return default config based on engine config
	strategy := c.Strategy
	return &OperatorErrorConfig{
		Strategy:          &strategy,
		RetryPolicy:       c.CreateRetryPolicy(),
		EnableDLQ:         c.EnableDLQ,
		DLQ:               nil, // Will use global DLQ
		EnableSideOutputs: c.EnableSideOutputs,
	}
}

// SetOperatorConfig sets a custom error config for an operator
func (c *EngineErrorConfig) SetOperatorConfig(operatorName string, config *OperatorErrorConfig) {
	c.OperatorConfigs[operatorName] = config
}

// SinkErrorConfig configures error handling for sinks
type SinkErrorConfig struct {
	// Retry configuration
	EnableRetry   bool
	RetryPolicy   *errors.RetryPolicy

	// Circuit breaker configuration
	EnableCircuitBreaker bool
	CircuitBreakerConfig *errors.CircuitBreakerConfig

	// DLQ configuration
	EnableDLQ bool
	DLQ       errors.DeadLetterQueue
}

// DefaultSinkErrorConfig returns default sink error config
func DefaultSinkErrorConfig(name string) *SinkErrorConfig {
	return &SinkErrorConfig{
		EnableRetry:          true,
		RetryPolicy:          errors.DefaultRetryPolicy(),
		EnableCircuitBreaker: true,
		CircuitBreakerConfig: errors.DefaultCircuitBreakerConfig(name),
		EnableDLQ:            false,
		DLQ:                  errors.NewNullDLQ(),
	}
}

// ErrorHandlingMode defines the mode of error handling
type ErrorHandlingMode int

const (
	// BestEffort - log errors but continue processing
	BestEffort ErrorHandlingMode = iota
	// AtLeastOnce - retry until success or DLQ
	AtLeastOnce
	// ExactlyOnce - retry with idempotency guarantees
	ExactlyOnce
)

// String returns the string representation of ErrorHandlingMode
func (m ErrorHandlingMode) String() string {
	switch m {
	case BestEffort:
		return "best-effort"
	case AtLeastOnce:
		return "at-least-once"
	case ExactlyOnce:
		return "exactly-once"
	default:
		return "unknown"
	}
}

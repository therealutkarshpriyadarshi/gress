package stream

import (
	"github.com/therealutkarshpriyadarshi/gress/pkg/config"
)

// FromConfig converts a config.Config to EngineConfig
func FromConfig(c *config.Config) EngineConfig {
	return EngineConfig{
		BufferSize:            c.Engine.BufferSize,
		MaxConcurrency:        c.Engine.MaxConcurrency,
		CheckpointInterval:    c.Engine.CheckpointInterval,
		WatermarkInterval:     c.Engine.WatermarkInterval,
		MetricsInterval:       c.Engine.MetricsInterval,
		EnableBackpressure:    c.Engine.EnableBackpressure,
		BackpressureThreshold: c.Engine.BackpressureThreshold,
		MetricsAddr:           c.Metrics.Address,
		EnableMetrics:         c.Metrics.Enabled,
	}
}

// ErrorConfigFromConfig converts a config.Config to EngineErrorConfig
func ErrorConfigFromConfig(c *config.Config) *EngineErrorConfig {
	// Parse strategy
	var strategy ErrorHandlingStrategy
	switch c.ErrorHandling.Strategy {
	case "fail-fast":
		strategy = FailFast
	case "retry":
		strategy = RetryWithBackoff
	case "send-to-dlq":
		strategy = SendToDLQ
	case "retry-then-dlq":
		strategy = RetryThenDLQ
	default:
		strategy = RetryThenDLQ
	}

	return &EngineErrorConfig{
		Strategy:                   strategy,
		EnableRetry:                c.ErrorHandling.EnableRetry,
		MaxRetryAttempts:           c.ErrorHandling.MaxRetryAttempts,
		InitialBackoff:             c.ErrorHandling.InitialBackoff,
		MaxBackoff:                 c.ErrorHandling.MaxBackoff,
		BackoffMultiplier:          c.ErrorHandling.BackoffMultiplier,
		BackoffJitter:              c.ErrorHandling.BackoffJitter,
		EnableDLQ:                  c.ErrorHandling.EnableDLQ,
		DLQMaxSize:                 c.ErrorHandling.DLQMaxSize,
		DLQType:                    c.ErrorHandling.DLQType,
		DLQDirectory:               c.ErrorHandling.DLQDirectory,
		DLQKafkaTopic:              c.ErrorHandling.DLQKafkaTopic,
		DLQKafkaBrokers:            c.ErrorHandling.DLQKafkaBrokers,
		EnableCircuitBreaker:       c.ErrorHandling.EnableCircuitBreaker,
		CircuitBreakerThreshold:    c.ErrorHandling.CircuitBreakerConfig.FailureThreshold,
		CircuitBreakerTimeout:      c.ErrorHandling.CircuitBreakerConfig.Timeout,
		CircuitBreakerSuccessCount: c.ErrorHandling.CircuitBreakerConfig.SuccessThreshold,
		EnableSideOutputs:          c.ErrorHandling.EnableSideOutputs,
		EnableErrorSideOutput:      true,
		EnableLateSideOutput:       false,
		EnableFilteredSideOutput:   false,
		SideOutputBufferSize:       c.ErrorHandling.SideOutputBufferSize,
		OperatorConfigs:            make(map[string]*OperatorErrorConfig),
	}
}

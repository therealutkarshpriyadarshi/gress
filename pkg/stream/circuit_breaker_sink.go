package stream

import (
	"context"
	"fmt"
	"time"

	"github.com/therealutkarshpriyadarshi/gress/pkg/errors"
	"github.com/therealutkarshpriyadarshi/gress/pkg/metrics"
	"go.uber.org/zap"
)

// CircuitBreakerSink wraps a sink with circuit breaker functionality
type CircuitBreakerSink struct {
	sink           Sink
	circuitBreaker *errors.CircuitBreaker
	metrics        *metrics.Collector
	logger         *zap.Logger
	name           string
}

// NewCircuitBreakerSink creates a new circuit breaker sink wrapper
func NewCircuitBreakerSink(
	sink Sink,
	config *errors.CircuitBreakerConfig,
	metricsCollector *metrics.Collector,
	logger *zap.Logger,
) *CircuitBreakerSink {
	// Set up state change callback to update metrics
	if config.OnStateChange == nil {
		config.OnStateChange = func(name string, from, to errors.CircuitState) {
			if metricsCollector != nil && metricsCollector.ErrorMetrics != nil {
				// Record state transition
				metricsCollector.ErrorMetrics.CircuitBreakerTransitions.WithLabelValues(
					name,
					from.String(),
					to.String(),
				).Inc()

				// Update state gauge
				var stateValue float64
				switch to {
				case errors.StateClosed:
					stateValue = 0
				case errors.StateOpen:
					stateValue = 1
				case errors.StateHalfOpen:
					stateValue = 2
				}
				metricsCollector.ErrorMetrics.CircuitBreakerState.WithLabelValues(name).Set(stateValue)
			}

			if logger != nil {
				logger.Info("Circuit breaker state changed",
					zap.String("circuit", name),
					zap.String("from", from.String()),
					zap.String("to", to.String()),
				)
			}
		}
	}

	cb := errors.NewCircuitBreaker(config)

	return &CircuitBreakerSink{
		sink:           sink,
		circuitBreaker: cb,
		metrics:        metricsCollector,
		logger:         logger,
		name:           config.Name,
	}
}

// Write writes an event through the circuit breaker
func (cbs *CircuitBreakerSink) Write(ctx context.Context, event *Event) error {
	var writeErr error

	// Execute through circuit breaker
	err := cbs.circuitBreaker.Execute(ctx, func() error {
		writeErr = cbs.sink.Write(ctx, event)
		return writeErr
	})

	// Record metrics
	if cbs.metrics != nil && cbs.metrics.ErrorMetrics != nil {
		if err != nil {
			// Check if it's a circuit breaker error
			var cbErr *errors.ErrCircuitOpen
			var tooManyErr *errors.ErrTooManyRequests

			if e, ok := err.(*errors.ErrCircuitOpen); ok {
				cbErr = e
			}
			if e, ok := err.(*errors.ErrTooManyRequests); ok {
				tooManyErr = e
			}

			if cbErr != nil {
				// Circuit is open, request rejected
				cbs.metrics.ErrorMetrics.CircuitBreakerRequests.WithLabelValues(
					cbs.name,
					"rejected_open",
				).Inc()
			} else if tooManyErr != nil {
				// Too many requests in half-open
				cbs.metrics.ErrorMetrics.CircuitBreakerRequests.WithLabelValues(
					cbs.name,
					"rejected_half_open",
				).Inc()
			} else {
				// Actual failure
				cbs.metrics.ErrorMetrics.CircuitBreakerRequests.WithLabelValues(
					cbs.name,
					"failure",
				).Inc()
				cbs.metrics.ErrorMetrics.CircuitBreakerFailures.WithLabelValues(
					cbs.name,
				).Inc()
			}
		} else {
			// Success
			cbs.metrics.ErrorMetrics.CircuitBreakerRequests.WithLabelValues(
				cbs.name,
				"success",
			).Inc()
		}
	}

	// Log errors
	if err != nil && cbs.logger != nil {
		var cbErr *errors.ErrCircuitOpen
		if e, ok := err.(*errors.ErrCircuitOpen); ok {
			cbErr = e
		}

		if cbErr != nil {
			cbs.logger.Warn("Circuit breaker is open, rejecting write",
				zap.String("sink", cbs.name),
				zap.String("key", event.Key),
			)
		} else {
			cbs.logger.Error("Failed to write to sink",
				zap.String("sink", cbs.name),
				zap.String("key", event.Key),
				zap.Error(err),
			)
		}
	}

	return err
}

// Flush flushes the sink through the circuit breaker
func (cbs *CircuitBreakerSink) Flush(ctx context.Context) error {
	return cbs.circuitBreaker.Execute(ctx, func() error {
		return cbs.sink.Flush(ctx)
	})
}

// Close closes the sink
func (cbs *CircuitBreakerSink) Close() error {
	return cbs.sink.Close()
}

// GetCircuitBreaker returns the circuit breaker
func (cbs *CircuitBreakerSink) GetCircuitBreaker() *errors.CircuitBreaker {
	return cbs.circuitBreaker
}

// GetSink returns the underlying sink
func (cbs *CircuitBreakerSink) GetSink() Sink {
	return cbs.sink
}

// ResetCircuitBreaker resets the circuit breaker
func (cbs *CircuitBreakerSink) ResetCircuitBreaker() {
	cbs.circuitBreaker.Reset()
	if cbs.logger != nil {
		cbs.logger.Info("Circuit breaker reset",
			zap.String("sink", cbs.name),
		)
	}
}

// IsAvailable checks if the sink is available (circuit breaker allows requests)
func (cbs *CircuitBreakerSink) IsAvailable() bool {
	return cbs.circuitBreaker.IsAvailable()
}

// Stats returns circuit breaker statistics
func (cbs *CircuitBreakerSink) Stats() errors.CircuitBreakerStats {
	return cbs.circuitBreaker.Stats()
}

// ResilientSinkConfig configures a resilient sink with both retry and circuit breaker
type ResilientSinkConfig struct {
	Name                 string
	RetryPolicy          *errors.RetryPolicy
	CircuitBreakerConfig *errors.CircuitBreakerConfig
	EnableDLQ            bool
	DLQ                  errors.DeadLetterQueue
}

// DefaultResilientSinkConfig returns default resilient sink configuration
func DefaultResilientSinkConfig(name string) *ResilientSinkConfig {
	return &ResilientSinkConfig{
		Name:                 name,
		RetryPolicy:          errors.DefaultRetryPolicy(),
		CircuitBreakerConfig: errors.DefaultCircuitBreakerConfig(name),
		EnableDLQ:            false,
		DLQ:                  errors.NewNullDLQ(),
	}
}

// ResilientSink combines circuit breaker and retry logic for sinks
type ResilientSink struct {
	sink           Sink
	circuitBreaker *errors.CircuitBreaker
	retryPolicy    *errors.RetryPolicy
	dlq            errors.DeadLetterQueue
	enableDLQ      bool
	metrics        *metrics.Collector
	logger         *zap.Logger
	name           string
}

// NewResilientSink creates a new resilient sink with retry and circuit breaker
func NewResilientSink(
	sink Sink,
	config *ResilientSinkConfig,
	metricsCollector *metrics.Collector,
	logger *zap.Logger,
) *ResilientSink {
	// Set up circuit breaker state change callback
	if config.CircuitBreakerConfig.OnStateChange == nil {
		config.CircuitBreakerConfig.OnStateChange = func(name string, from, to errors.CircuitState) {
			if metricsCollector != nil && metricsCollector.ErrorMetrics != nil {
				metricsCollector.ErrorMetrics.CircuitBreakerTransitions.WithLabelValues(
					name, from.String(), to.String(),
				).Inc()

				var stateValue float64
				switch to {
				case errors.StateClosed:
					stateValue = 0
				case errors.StateOpen:
					stateValue = 1
				case errors.StateHalfOpen:
					stateValue = 2
				}
				metricsCollector.ErrorMetrics.CircuitBreakerState.WithLabelValues(name).Set(stateValue)
			}

			if logger != nil {
				logger.Info("Circuit breaker state changed",
					zap.String("circuit", name),
					zap.String("from", from.String()),
					zap.String("to", to.String()),
				)
			}
		}
	}

	cb := errors.NewCircuitBreaker(config.CircuitBreakerConfig)

	return &ResilientSink{
		sink:           sink,
		circuitBreaker: cb,
		retryPolicy:    config.RetryPolicy,
		dlq:            config.DLQ,
		enableDLQ:      config.EnableDLQ,
		metrics:        metricsCollector,
		logger:         logger,
		name:           config.Name,
	}
}

// Write writes an event with retry and circuit breaker
func (rs *ResilientSink) Write(ctx context.Context, event *Event) error {
	var finalErr error
	retryCount := 0

	// Execute with retry policy
	retryResult := rs.retryPolicy.ExecuteWithCallback(
		ctx,
		func(retryCtx context.Context) error {
			// Execute through circuit breaker
			return rs.circuitBreaker.Execute(retryCtx, func() error {
				return rs.sink.Write(retryCtx, event)
			})
		},
		func(attempt int, err error, nextBackoff time.Duration) {
			retryCount = attempt
			if rs.logger != nil {
				rs.logger.Warn("Retrying sink write",
					zap.String("sink", rs.name),
					zap.Int("attempt", attempt),
					zap.Duration("next_backoff", nextBackoff),
					zap.Error(err),
				)
			}
		},
	)

	finalErr = retryResult.LastError

	// Record metrics
	if rs.metrics != nil && rs.metrics.ErrorMetrics != nil {
		if retryResult.Success {
			rs.metrics.ErrorMetrics.CircuitBreakerRequests.WithLabelValues(
				rs.name, "success",
			).Inc()

			if retryResult.Attempts > 1 {
				rs.metrics.ErrorMetrics.RetrySuccesses.WithLabelValues(
					rs.name, "retriable",
				).Inc()
			}
		} else {
			rs.metrics.ErrorMetrics.CircuitBreakerRequests.WithLabelValues(
				rs.name, "failure",
			).Inc()
			rs.metrics.ErrorMetrics.RetryFailures.WithLabelValues(
				rs.name, errors.ClassifyError(finalErr).String(),
			).Inc()
		}
	}

	// If failed, write to DLQ
	if !retryResult.Success && rs.enableDLQ && rs.dlq != nil {
		failedEvent := &errors.FailedEvent{
			Key:             event.Key,
			Value:           event.Value,
			EventTime:       event.EventTime,
			Headers:         event.Headers,
			Offset:          event.Offset,
			Partition:       event.Partition,
			FailureReason:   finalErr.Error(),
			FailureCategory: errors.ClassifyError(finalErr).String(),
			OperatorName:    fmt.Sprintf("sink:%s", rs.name),
			FailureTime:     time.Now(),
			RetryCount:      retryCount,
		}

		if dlqErr := rs.dlq.Write(ctx, failedEvent); dlqErr != nil {
			if rs.logger != nil {
				rs.logger.Error("Failed to write to DLQ",
					zap.String("sink", rs.name),
					zap.Error(dlqErr),
				)
			}
		} else if rs.metrics != nil && rs.metrics.ErrorMetrics != nil {
			rs.metrics.ErrorMetrics.DLQEventsWritten.WithLabelValues(
				rs.name, errors.ClassifyError(finalErr).String(),
			).Inc()
		}
	}

	if !retryResult.Success {
		return finalErr
	}

	return nil
}

// Flush flushes the sink
func (rs *ResilientSink) Flush(ctx context.Context) error {
	return rs.circuitBreaker.Execute(ctx, func() error {
		return rs.sink.Flush(ctx)
	})
}

// Close closes the sink
func (rs *ResilientSink) Close() error {
	return rs.sink.Close()
}

// GetCircuitBreaker returns the circuit breaker
func (rs *ResilientSink) GetCircuitBreaker() *errors.CircuitBreaker {
	return rs.circuitBreaker
}

// ResetCircuitBreaker resets the circuit breaker
func (rs *ResilientSink) ResetCircuitBreaker() {
	rs.circuitBreaker.Reset()
}

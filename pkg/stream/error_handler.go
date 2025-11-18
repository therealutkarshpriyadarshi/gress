package stream

import (
	"context"
	"time"

	"github.com/therealutkarshpriyadarshi/gress/pkg/errors"
	"github.com/therealutkarshpriyadarshi/gress/pkg/metrics"
	"go.uber.org/zap"
)

// ErrorHandlingConfig configures error handling for operators
type ErrorHandlingConfig struct {
	// RetryPolicy defines the retry behavior
	RetryPolicy *errors.RetryPolicy
	// EnableDLQ enables writing failed events to DLQ
	EnableDLQ bool
	// DLQ is the dead letter queue for failed events
	DLQ errors.DeadLetterQueue
	// EnableSideOutputs enables error event side outputs
	EnableSideOutputs bool
	// SideOutputCollector collects error events
	SideOutputCollector *errors.SideOutputCollector
	// OperatorName is the name of the operator for metrics
	OperatorName string
}

// DefaultErrorHandlingConfig returns a default error handling config
func DefaultErrorHandlingConfig(operatorName string) *ErrorHandlingConfig {
	return &ErrorHandlingConfig{
		RetryPolicy:         errors.DefaultRetryPolicy(),
		EnableDLQ:           false,
		DLQ:                 errors.NewNullDLQ(),
		EnableSideOutputs:   false,
		SideOutputCollector: errors.NewSideOutputCollector(),
		OperatorName:        operatorName,
	}
}

// ErrorHandlingOperator wraps an operator with error handling capabilities
type ErrorHandlingOperator struct {
	operator Operator
	config   *ErrorHandlingConfig
	metrics  *metrics.Collector
	logger   *zap.Logger
}

// NewErrorHandlingOperator creates a new error handling operator wrapper
func NewErrorHandlingOperator(
	operator Operator,
	config *ErrorHandlingConfig,
	metricsCollector *metrics.Collector,
	logger *zap.Logger,
) *ErrorHandlingOperator {
	return &ErrorHandlingOperator{
		operator: operator,
		config:   config,
		metrics:  metricsCollector,
		logger:   logger,
	}
}

// Process processes an event with error handling
func (eho *ErrorHandlingOperator) Process(ctx *ProcessingContext, event *Event) ([]*Event, error) {
	var result []*Event
	var lastError error

	// Execute with retry policy
	retryResult := eho.config.RetryPolicy.ExecuteWithCallback(
		ctx.Ctx,
		func(context.Context) error {
			var err error
			result, err = eho.operator.Process(ctx, event)
			lastError = err
			return err
		},
		func(attempt int, err error, nextBackoff time.Duration) {
			// Retry callback - record metrics and log
			category := errors.ClassifyError(err)
			categoryStr := category.String()

			// Record retry metrics
			if eho.metrics != nil && eho.metrics.ErrorMetrics != nil {
				eho.metrics.ErrorMetrics.RetryAttempts.WithLabelValues(
					eho.config.OperatorName,
					categoryStr,
				).Inc()

				if nextBackoff > 0 {
					eho.metrics.ErrorMetrics.RetryBackoffTime.WithLabelValues(
						eho.config.OperatorName,
					).Observe(nextBackoff.Seconds())
				}
			}

			// Log retry attempt
			if eho.logger != nil {
				eho.logger.Warn("Retrying operator after error",
					zap.String("operator", eho.config.OperatorName),
					zap.Int("attempt", attempt),
					zap.Duration("next_backoff", nextBackoff),
					zap.String("error_category", categoryStr),
					zap.Error(err),
				)
			}
		},
	)

	// Record final retry outcome
	if eho.metrics != nil && eho.metrics.ErrorMetrics != nil {
		if retryResult.Success && retryResult.Attempts > 1 {
			// Successful retry
			category := errors.ClassifyError(lastError)
			eho.metrics.ErrorMetrics.RetrySuccesses.WithLabelValues(
				eho.config.OperatorName,
				category.String(),
			).Inc()

			eho.metrics.ErrorMetrics.ErrorRecoveries.WithLabelValues(
				eho.config.OperatorName,
				"retry",
			).Inc()
		} else if !retryResult.Success {
			// Failed after all retries
			category := errors.ClassifyError(retryResult.LastError)
			eho.metrics.ErrorMetrics.RetryFailures.WithLabelValues(
				eho.config.OperatorName,
				category.String(),
			).Inc()
		}
	}

	// If still failing after retries, handle the error
	if !retryResult.Success {
		if err := eho.handleError(ctx, event, retryResult.LastError, retryResult.Attempts); err != nil {
			// Log error handling failure
			if eho.logger != nil {
				eho.logger.Error("Failed to handle error",
					zap.String("operator", eho.config.OperatorName),
					zap.Error(err),
				)
			}
		}

		// Return the error
		return nil, retryResult.LastError
	}

	return result, nil
}

// handleError handles a failed event after retries are exhausted
func (eho *ErrorHandlingOperator) handleError(
	ctx *ProcessingContext,
	event *Event,
	err error,
	retryCount int,
) error {
	category := errors.ClassifyError(err)

	// Record error by category
	if eho.metrics != nil && eho.metrics.ErrorMetrics != nil {
		eho.metrics.ErrorMetrics.ErrorsByCategory.WithLabelValues(
			eho.config.OperatorName,
			category.String(),
		).Inc()
	}

	// Write to DLQ if enabled
	if eho.config.EnableDLQ && eho.config.DLQ != nil {
		failedEvent := &errors.FailedEvent{
			Key:             event.Key,
			Value:           event.Value,
			EventTime:       event.EventTime,
			Headers:         event.Headers,
			Offset:          event.Offset,
			Partition:       event.Partition,
			FailureReason:   err.Error(),
			FailureCategory: category.String(),
			OperatorName:    eho.config.OperatorName,
			FailureTime:     time.Now(),
			RetryCount:      retryCount,
			Metadata:        make(map[string]string),
		}

		if dlqErr := eho.config.DLQ.Write(ctx.Ctx, failedEvent); dlqErr != nil {
			if eho.logger != nil {
				eho.logger.Error("Failed to write to DLQ",
					zap.String("operator", eho.config.OperatorName),
					zap.Error(dlqErr),
				)
			}
			return dlqErr
		}

		// Record DLQ write metrics
		if eho.metrics != nil && eho.metrics.ErrorMetrics != nil {
			eho.metrics.ErrorMetrics.DLQEventsWritten.WithLabelValues(
				"default",
				category.String(),
			).Inc()
		}
	}

	// Emit to side output if enabled
	if eho.config.EnableSideOutputs && eho.config.SideOutputCollector != nil {
		sideOutputEvent := &errors.SideOutputEvent{
			Key:           event.Key,
			Value:         event.Value,
			EventTime:     event.EventTime,
			Headers:       event.Headers,
			Offset:        event.Offset,
			Partition:     event.Partition,
			Error:         err,
			ErrorCategory: category,
			OperatorName:  eho.config.OperatorName,
			RetryCount:    retryCount,
			Tag:           errors.ErrorSideOutput,
			Metadata:      make(map[string]interface{}),
		}

		eho.config.SideOutputCollector.Emit(ctx.Ctx, sideOutputEvent)

		// Record side output metrics
		if eho.metrics != nil && eho.metrics.ErrorMetrics != nil {
			eho.metrics.ErrorMetrics.SideOutputEvents.WithLabelValues(
				string(errors.ErrorSideOutput),
				eho.config.OperatorName,
			).Inc()
		}
	}

	return nil
}

// Close closes the operator
func (eho *ErrorHandlingOperator) Close() error {
	return eho.operator.Close()
}

// GetOperator returns the underlying operator
func (eho *ErrorHandlingOperator) GetOperator() Operator {
	return eho.operator
}

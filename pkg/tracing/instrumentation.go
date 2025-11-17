package tracing

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// TraceLogger wraps zap.Logger with trace context support
type TraceLogger struct {
	*zap.Logger
}

// NewTraceLogger creates a logger with trace context support
func NewTraceLogger(logger *zap.Logger) *TraceLogger {
	return &TraceLogger{Logger: logger}
}

// WithContext returns a logger with trace context fields
func (tl *TraceLogger) WithContext(ctx context.Context) *zap.Logger {
	traceID := TraceID(ctx)
	spanID := SpanID(ctx)

	if traceID != "" || spanID != "" {
		return tl.Logger.With(
			zap.String("trace_id", traceID),
			zap.String("span_id", spanID),
		)
	}

	return tl.Logger
}

// ContextLogger returns a logger with trace context
func ContextLogger(ctx context.Context, logger *zap.Logger) *zap.Logger {
	traceID := TraceID(ctx)
	spanID := SpanID(ctx)

	if traceID != "" && traceID != "placeholder-trace-id" {
		return logger.With(
			zap.String("trace_id", traceID),
			zap.String("span_id", spanID),
		)
	}

	return logger
}

// InstrumentationHelper provides tracing utilities for stream processing
type InstrumentationHelper struct {
	provider *TracerProvider
	logger   *zap.Logger
}

// NewInstrumentationHelper creates a new instrumentation helper
func NewInstrumentationHelper(provider *TracerProvider, logger *zap.Logger) *InstrumentationHelper {
	return &InstrumentationHelper{
		provider: provider,
		logger:   logger,
	}
}

// TraceEventProcessing traces event processing through the pipeline
func (ih *InstrumentationHelper) TraceEventProcessing(ctx context.Context, eventKey string, operatorName string) (context.Context, *Span) {
	spanName := fmt.Sprintf("process_event.%s", operatorName)
	ctx, span := ih.provider.StartSpan(ctx, spanName)

	span.SetAttributes(map[string]interface{}{
		"event.key":      eventKey,
		"operator.name":  operatorName,
		"component":      "stream_processor",
	})

	return ctx, span
}

// TraceStateOperation traces state backend operations
func (ih *InstrumentationHelper) TraceStateOperation(ctx context.Context, operation, backend, key string) (context.Context, *Span) {
	spanName := fmt.Sprintf("state.%s", operation)
	ctx, span := ih.provider.StartSpan(ctx, spanName)

	span.SetAttributes(map[string]interface{}{
		"state.operation": operation,
		"state.backend":   backend,
		"state.key":       key,
		"component":       "state_backend",
	})

	return ctx, span
}

// TraceCheckpoint traces checkpoint operations
func (ih *InstrumentationHelper) TraceCheckpoint(ctx context.Context, checkpointID string) (context.Context, *Span) {
	ctx, span := ih.provider.StartSpan(ctx, "checkpoint.create")

	span.SetAttributes(map[string]interface{}{
		"checkpoint.id": checkpointID,
		"component":     "checkpoint_manager",
	})

	return ctx, span
}

// TraceWindow traces window operations
func (ih *InstrumentationHelper) TraceWindow(ctx context.Context, windowType string, windowStart, windowEnd time.Time) (context.Context, *Span) {
	spanName := fmt.Sprintf("window.%s", windowType)
	ctx, span := ih.provider.StartSpan(ctx, spanName)

	span.SetAttributes(map[string]interface{}{
		"window.type":  windowType,
		"window.start": windowStart.Format(time.RFC3339),
		"window.end":   windowEnd.Format(time.RFC3339),
		"component":    "window_manager",
	})

	return ctx, span
}

// StructuredLogConfig holds configuration for structured logging
type StructuredLogConfig struct {
	Level            zapcore.Level
	Development      bool
	EnableStacktrace bool
	EnableCaller     bool
	OutputPaths      []string
	ErrorOutputPaths []string
	InitialFields    map[string]interface{}
}

// DefaultStructuredLogConfig returns default structured logging configuration
func DefaultStructuredLogConfig() *StructuredLogConfig {
	return &StructuredLogConfig{
		Level:            zapcore.InfoLevel,
		Development:      false,
		EnableStacktrace: false,
		EnableCaller:     true,
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
		InitialFields:    make(map[string]interface{}),
	}
}

// NewStructuredLogger creates a new structured logger with trace support
func NewStructuredLogger(config *StructuredLogConfig) (*zap.Logger, error) {
	if config == nil {
		config = DefaultStructuredLogConfig()
	}

	zapConfig := zap.Config{
		Level:            zap.NewAtomicLevelAt(config.Level),
		Development:      config.Development,
		Encoding:         "json",
		EncoderConfig: zapcore.EncoderConfig{
			TimeKey:        "timestamp",
			LevelKey:       "level",
			NameKey:        "logger",
			CallerKey:      "caller",
			FunctionKey:    zapcore.OmitKey,
			MessageKey:     "message",
			StacktraceKey:  "stacktrace",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.LowercaseLevelEncoder,
			EncodeTime:     zapcore.ISO8601TimeEncoder,
			EncodeDuration: zapcore.SecondsDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		},
		OutputPaths:      config.OutputPaths,
		ErrorOutputPaths: config.ErrorOutputPaths,
	}

	// Add initial fields
	if len(config.InitialFields) > 0 {
		initialFields := make(map[string]interface{})
		for k, v := range config.InitialFields {
			initialFields[k] = v
		}
		zapConfig.InitialFields = initialFields
	}

	logger, err := zapConfig.Build()
	if err != nil {
		return nil, fmt.Errorf("failed to create logger: %w", err)
	}

	if config.EnableCaller {
		logger = logger.WithOptions(zap.AddCaller())
	}

	if config.EnableStacktrace {
		logger = logger.WithOptions(zap.AddStacktrace(zapcore.ErrorLevel))
	}

	return logger, nil
}

// LogWithTrace logs a message with trace context
func LogWithTrace(ctx context.Context, logger *zap.Logger, level zapcore.Level, msg string, fields ...zap.Field) {
	traceID := TraceID(ctx)
	spanID := SpanID(ctx)

	if traceID != "" && traceID != "placeholder-trace-id" {
		fields = append(fields,
			zap.String("trace_id", traceID),
			zap.String("span_id", spanID),
		)
	}

	switch level {
	case zapcore.DebugLevel:
		logger.Debug(msg, fields...)
	case zapcore.InfoLevel:
		logger.Info(msg, fields...)
	case zapcore.WarnLevel:
		logger.Warn(msg, fields...)
	case zapcore.ErrorLevel:
		logger.Error(msg, fields...)
	case zapcore.FatalLevel:
		logger.Fatal(msg, fields...)
	default:
		logger.Info(msg, fields...)
	}
}

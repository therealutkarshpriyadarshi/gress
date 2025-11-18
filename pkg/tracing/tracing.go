package tracing

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// TracerProvider manages distributed tracing configuration
type TracerProvider struct {
	serviceName    string
	serviceVersion string
	environment    string
	logger         *zap.Logger
	enabled        bool
	provider       *sdktrace.TracerProvider
	tracer         trace.Tracer
}

// Config holds tracing configuration
type Config struct {
	Enabled          bool
	ServiceName      string
	ServiceVersion   string
	Environment      string
	SamplingRate     float64
	ExporterType     string // "jaeger", "otlp", "stdout"
	ExporterEndpoint string
	OTLPHeaders      map[string]string
	OTLPInsecure     bool
}

// DefaultConfig returns default tracing configuration
func DefaultConfig() *Config {
	return &Config{
		Enabled:          false,
		ServiceName:      "gress",
		ServiceVersion:   "1.0.0",
		Environment:      "development",
		SamplingRate:     1.0,
		ExporterType:     "otlp",
		ExporterEndpoint: "http://localhost:4318/v1/traces",
		OTLPHeaders:      make(map[string]string),
		OTLPInsecure:     true,
	}
}

// NewProvider creates a new tracing provider with OpenTelemetry
func NewProvider(config *Config, logger *zap.Logger) (*TracerProvider, error) {
	if config == nil {
		config = DefaultConfig()
	}

	provider := &TracerProvider{
		serviceName:    config.ServiceName,
		serviceVersion: config.ServiceVersion,
		environment:    config.Environment,
		logger:         logger,
		enabled:        config.Enabled,
	}

	if !config.Enabled {
		logger.Info("Distributed tracing is disabled")
		return provider, nil
	}

	// Create resource with service information
	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(config.ServiceName),
			semconv.ServiceVersion(config.ServiceVersion),
			attribute.String("environment", config.Environment),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create exporter based on configuration
	var exporter sdktrace.SpanExporter
	switch config.ExporterType {
	case "jaeger":
		exporter, err = jaeger.New(
			jaeger.WithCollectorEndpoint(
				jaeger.WithEndpoint(config.ExporterEndpoint),
			),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create Jaeger exporter: %w", err)
		}
	case "otlp":
		opts := []otlptracehttp.Option{
			otlptracehttp.WithEndpoint(config.ExporterEndpoint),
		}
		if config.OTLPInsecure {
			opts = append(opts, otlptracehttp.WithInsecure())
		}
		if len(config.OTLPHeaders) > 0 {
			opts = append(opts, otlptracehttp.WithHeaders(config.OTLPHeaders))
		}
		exporter, err = otlptracehttp.New(context.Background(), opts...)
		if err != nil {
			return nil, fmt.Errorf("failed to create OTLP exporter: %w", err)
		}
	case "stdout":
		exporter, err = stdouttrace.New(
			stdouttrace.WithPrettyPrint(),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create stdout exporter: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported exporter type: %s", config.ExporterType)
	}

	// Create sampler based on sampling rate
	var sampler sdktrace.Sampler
	if config.SamplingRate >= 1.0 {
		sampler = sdktrace.AlwaysSample()
	} else if config.SamplingRate <= 0.0 {
		sampler = sdktrace.NeverSample()
	} else {
		sampler = sdktrace.TraceIDRatioBased(config.SamplingRate)
	}

	// Create trace provider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter,
			sdktrace.WithBatchTimeout(5*time.Second),
			sdktrace.WithMaxExportBatchSize(512),
		),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
	)

	// Set global trace provider
	otel.SetTracerProvider(tp)

	// Set global propagator for context propagation (W3C Trace Context)
	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		),
	)

	provider.provider = tp
	provider.tracer = tp.Tracer(
		config.ServiceName,
		trace.WithInstrumentationVersion(config.ServiceVersion),
	)

	logger.Info("Distributed tracing initialized",
		zap.String("service", config.ServiceName),
		zap.String("version", config.ServiceVersion),
		zap.String("exporter", config.ExporterType),
		zap.String("endpoint", config.ExporterEndpoint),
		zap.Float64("sampling_rate", config.SamplingRate))

	return provider, nil
}

// StartSpan starts a new tracing span
func (tp *TracerProvider) StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	if !tp.enabled || tp.tracer == nil {
		return ctx, trace.SpanFromContext(ctx)
	}
	return tp.tracer.Start(ctx, name, opts...)
}

// Shutdown gracefully shuts down the tracer provider
func (tp *TracerProvider) Shutdown(ctx context.Context) error {
	if !tp.enabled || tp.provider == nil {
		return nil
	}

	tp.logger.Info("Shutting down tracing provider")

	shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	return tp.provider.Shutdown(shutdownCtx)
}

// GetTracer returns the underlying tracer
func (tp *TracerProvider) GetTracer() trace.Tracer {
	if !tp.enabled || tp.tracer == nil {
		return otel.Tracer("noop")
	}
	return tp.tracer
}

// SpanFromContext extracts the current span from context
func SpanFromContext(ctx context.Context) trace.Span {
	return trace.SpanFromContext(ctx)
}

// TraceID returns the trace ID from the current span in context
func TraceID(ctx context.Context) string {
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().HasTraceID() {
		return span.SpanContext().TraceID().String()
	}
	return ""
}

// SpanID returns the span ID from the current span in context
func SpanID(ctx context.Context) string {
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().HasSpanID() {
		return span.SpanContext().SpanID().String()
	}
	return ""
}

// InjectTraceContext injects trace context into headers for propagation
func InjectTraceContext(ctx context.Context, carrier propagation.TextMapCarrier) {
	otel.GetTextMapPropagator().Inject(ctx, carrier)
}

// ExtractTraceContext extracts trace context from headers
func ExtractTraceContext(ctx context.Context, carrier propagation.TextMapCarrier) context.Context {
	return otel.GetTextMapPropagator().Extract(ctx, carrier)
}

// SetSpanAttributes sets multiple attributes on the current span
func SetSpanAttributes(ctx context.Context, attrs map[string]interface{}) {
	span := trace.SpanFromContext(ctx)
	if !span.IsRecording() {
		return
	}

	for key, value := range attrs {
		switch v := value.(type) {
		case string:
			span.SetAttributes(attribute.String(key, v))
		case int:
			span.SetAttributes(attribute.Int(key, v))
		case int64:
			span.SetAttributes(attribute.Int64(key, v))
		case float64:
			span.SetAttributes(attribute.Float64(key, v))
		case bool:
			span.SetAttributes(attribute.Bool(key, v))
		default:
			span.SetAttributes(attribute.String(key, fmt.Sprintf("%v", v)))
		}
	}
}

// RecordError records an error on the current span
func RecordError(ctx context.Context, err error) {
	if err == nil {
		return
	}
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		span.RecordError(err)
	}
}

// AddEvent adds an event to the current span
func AddEvent(ctx context.Context, name string, attrs map[string]interface{}) {
	span := trace.SpanFromContext(ctx)
	if !span.IsRecording() {
		return
	}

	var otelAttrs []attribute.KeyValue
	for key, value := range attrs {
		switch v := value.(type) {
		case string:
			otelAttrs = append(otelAttrs, attribute.String(key, v))
		case int:
			otelAttrs = append(otelAttrs, attribute.Int(key, v))
		case int64:
			otelAttrs = append(otelAttrs, attribute.Int64(key, v))
		case float64:
			otelAttrs = append(otelAttrs, attribute.Float64(key, v))
		case bool:
			otelAttrs = append(otelAttrs, attribute.Bool(key, v))
		default:
			otelAttrs = append(otelAttrs, attribute.String(key, fmt.Sprintf("%v", v)))
		}
	}

	span.AddEvent(name, trace.WithAttributes(otelAttrs...))
}

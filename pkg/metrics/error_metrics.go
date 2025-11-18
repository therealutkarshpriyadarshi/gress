package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

// ErrorMetrics holds error handling specific metrics
type ErrorMetrics struct {
	// Retry metrics
	RetryAttempts       *prometheus.CounterVec
	RetrySuccesses      *prometheus.CounterVec
	RetryFailures       *prometheus.CounterVec
	RetryBackoffTime    *prometheus.HistogramVec

	// Circuit breaker metrics
	CircuitBreakerState        *prometheus.GaugeVec
	CircuitBreakerTransitions  *prometheus.CounterVec
	CircuitBreakerRequests     *prometheus.CounterVec
	CircuitBreakerFailures     *prometheus.CounterVec

	// Dead Letter Queue metrics
	DLQEventsWritten   *prometheus.CounterVec
	DLQEventsRead      *prometheus.CounterVec
	DLQEventsDeleted   *prometheus.CounterVec
	DLQSize            *prometheus.GaugeVec

	// Side output metrics
	SideOutputEvents   *prometheus.CounterVec

	// Error categorization metrics
	ErrorsByCategory   *prometheus.CounterVec
	ErrorRecoveries    *prometheus.CounterVec
}

// NewErrorMetrics creates a new error metrics collector
func NewErrorMetrics(registry *prometheus.Registry) *ErrorMetrics {
	em := &ErrorMetrics{}
	em.initMetrics()
	em.registerMetrics(registry)
	return em
}

// initMetrics initializes all error-related metrics
func (em *ErrorMetrics) initMetrics() {
	// Retry metrics
	em.RetryAttempts = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gress_retry_attempts_total",
			Help: "Total number of retry attempts",
		},
		[]string{"operator", "error_category"},
	)

	em.RetrySuccesses = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gress_retry_successes_total",
			Help: "Total number of successful retries",
		},
		[]string{"operator", "error_category"},
	)

	em.RetryFailures = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gress_retry_failures_total",
			Help: "Total number of failed retries (exhausted all attempts)",
		},
		[]string{"operator", "error_category"},
	)

	em.RetryBackoffTime = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "gress_retry_backoff_seconds",
			Help:    "Total backoff time spent in retries",
			Buckets: []float64{.01, .05, .1, .5, 1, 5, 10, 30, 60, 120},
		},
		[]string{"operator"},
	)

	// Circuit breaker metrics
	em.CircuitBreakerState = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "gress_circuit_breaker_state",
			Help: "Current state of circuit breaker (0=closed, 1=open, 2=half-open)",
		},
		[]string{"circuit_name"},
	)

	em.CircuitBreakerTransitions = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gress_circuit_breaker_transitions_total",
			Help: "Total number of circuit breaker state transitions",
		},
		[]string{"circuit_name", "from_state", "to_state"},
	)

	em.CircuitBreakerRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gress_circuit_breaker_requests_total",
			Help: "Total number of requests through circuit breaker",
		},
		[]string{"circuit_name", "result"},
	)

	em.CircuitBreakerFailures = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gress_circuit_breaker_failures_total",
			Help: "Total number of failures tracked by circuit breaker",
		},
		[]string{"circuit_name"},
	)

	// Dead Letter Queue metrics
	em.DLQEventsWritten = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gress_dlq_events_written_total",
			Help: "Total number of events written to DLQ",
		},
		[]string{"dlq_name", "error_category"},
	)

	em.DLQEventsRead = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gress_dlq_events_read_total",
			Help: "Total number of events read from DLQ",
		},
		[]string{"dlq_name"},
	)

	em.DLQEventsDeleted = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gress_dlq_events_deleted_total",
			Help: "Total number of events deleted from DLQ",
		},
		[]string{"dlq_name"},
	)

	em.DLQSize = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "gress_dlq_size_events",
			Help: "Current number of events in DLQ",
		},
		[]string{"dlq_name"},
	)

	// Side output metrics
	em.SideOutputEvents = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gress_side_output_events_total",
			Help: "Total number of events emitted to side outputs",
		},
		[]string{"tag", "operator"},
	)

	// Error categorization metrics
	em.ErrorsByCategory = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gress_errors_by_category_total",
			Help: "Total number of errors by category",
		},
		[]string{"operator", "category"},
	)

	em.ErrorRecoveries = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gress_error_recoveries_total",
			Help: "Total number of successful error recoveries",
		},
		[]string{"operator", "recovery_method"},
	)
}

// registerMetrics registers all metrics with the Prometheus registry
func (em *ErrorMetrics) registerMetrics(registry *prometheus.Registry) {
	// Retry metrics
	registry.MustRegister(em.RetryAttempts)
	registry.MustRegister(em.RetrySuccesses)
	registry.MustRegister(em.RetryFailures)
	registry.MustRegister(em.RetryBackoffTime)

	// Circuit breaker metrics
	registry.MustRegister(em.CircuitBreakerState)
	registry.MustRegister(em.CircuitBreakerTransitions)
	registry.MustRegister(em.CircuitBreakerRequests)
	registry.MustRegister(em.CircuitBreakerFailures)

	// Dead Letter Queue metrics
	registry.MustRegister(em.DLQEventsWritten)
	registry.MustRegister(em.DLQEventsRead)
	registry.MustRegister(em.DLQEventsDeleted)
	registry.MustRegister(em.DLQSize)

	// Side output metrics
	registry.MustRegister(em.SideOutputEvents)

	// Error categorization metrics
	registry.MustRegister(em.ErrorsByCategory)
	registry.MustRegister(em.ErrorRecoveries)
}

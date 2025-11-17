package stream

import (
	"fmt"
	"time"

	"github.com/therealutkarshpriyadarshi/gress/pkg/metrics"
)

// InstrumentedOperator wraps an operator with metrics collection
type InstrumentedOperator struct {
	operator     Operator
	name         string
	operatorType string
	collector    *metrics.Collector
}

// NewInstrumentedOperator creates a metrics-aware operator wrapper
func NewInstrumentedOperator(operator Operator, name, operatorType string, collector *metrics.Collector) *InstrumentedOperator {
	return &InstrumentedOperator{
		operator:     operator,
		name:         name,
		operatorType: operatorType,
		collector:    collector,
	}
}

// Process implements the Operator interface with metrics instrumentation
func (io *InstrumentedOperator) Process(ctx *ProcessingContext, event *Event) ([]*Event, error) {
	// Track input events
	io.collector.OperatorEventsIn.WithLabelValues(io.name, io.operatorType).Inc()

	// Measure processing latency
	start := time.Now()
	results, err := io.operator.Process(ctx, event)
	duration := time.Since(start)

	// Record latency
	io.collector.OperatorLatency.WithLabelValues(io.name, io.operatorType).Observe(duration.Seconds())

	// Handle errors
	if err != nil {
		errorType := fmt.Sprintf("%T", err)
		io.collector.OperatorErrors.WithLabelValues(io.name, io.operatorType, errorType).Inc()
		return nil, err
	}

	// Track output events
	outputCount := len(results)
	io.collector.OperatorEventsOut.WithLabelValues(io.name, io.operatorType).Add(float64(outputCount))

	// Track filtered events (when input event produces no output)
	if outputCount == 0 {
		io.collector.EventsFiltered.WithLabelValues(io.name).Inc()
	}

	return results, nil
}

// Close implements the Operator interface
func (io *InstrumentedOperator) Close() error {
	return io.operator.Close()
}

// InstrumentedStateBackend wraps a state backend with metrics collection
type InstrumentedStateBackend struct {
	backend   StateBackend
	name      string
	collector *metrics.Collector
}

// NewInstrumentedStateBackend creates a metrics-aware state backend wrapper
func NewInstrumentedStateBackend(backend StateBackend, name string, collector *metrics.Collector) *InstrumentedStateBackend {
	return &InstrumentedStateBackend{
		backend:   backend,
		name:      name,
		collector: collector,
	}
}

// Get implements StateBackend interface with metrics
func (isb *InstrumentedStateBackend) Get(key string) ([]byte, error) {
	start := time.Now()
	value, err := isb.backend.Get(key)
	duration := time.Since(start)

	isb.collector.StateOperations.WithLabelValues(isb.name, "get").Inc()
	isb.collector.StateLatency.WithLabelValues(isb.name, "get").Observe(duration.Seconds())

	return value, err
}

// Put implements StateBackend interface with metrics
func (isb *InstrumentedStateBackend) Put(key string, value []byte) error {
	start := time.Now()
	err := isb.backend.Put(key, value)
	duration := time.Since(start)

	isb.collector.StateOperations.WithLabelValues(isb.name, "put").Inc()
	isb.collector.StateLatency.WithLabelValues(isb.name, "put").Observe(duration.Seconds())

	if err == nil {
		// Update state size (approximate)
		isb.collector.StateSize.WithLabelValues(isb.name, "unknown").Add(float64(len(value)))
	}

	return err
}

// Delete implements StateBackend interface with metrics
func (isb *InstrumentedStateBackend) Delete(key string) error {
	start := time.Now()
	err := isb.backend.Delete(key)
	duration := time.Since(start)

	isb.collector.StateOperations.WithLabelValues(isb.name, "delete").Inc()
	isb.collector.StateLatency.WithLabelValues(isb.name, "delete").Observe(duration.Seconds())

	return err
}

// Snapshot implements StateBackend interface with metrics
func (isb *InstrumentedStateBackend) Snapshot() ([]byte, error) {
	start := time.Now()
	data, err := isb.backend.Snapshot()
	duration := time.Since(start)

	isb.collector.StateOperations.WithLabelValues(isb.name, "snapshot").Inc()
	isb.collector.StateLatency.WithLabelValues(isb.name, "snapshot").Observe(duration.Seconds())

	if err == nil && data != nil {
		isb.collector.StateSize.WithLabelValues(isb.name, "snapshot").Set(float64(len(data)))
	}

	return data, err
}

// Restore implements StateBackend interface with metrics
func (isb *InstrumentedStateBackend) Restore(data []byte) error {
	start := time.Now()
	err := isb.backend.Restore(data)
	duration := time.Since(start)

	isb.collector.StateOperations.WithLabelValues(isb.name, "restore").Inc()
	isb.collector.StateLatency.WithLabelValues(isb.name, "restore").Observe(duration.Seconds())

	if err == nil && data != nil {
		isb.collector.StateSize.WithLabelValues(isb.name, "restored").Set(float64(len(data)))
	}

	return err
}

// Close implements StateBackend interface
func (isb *InstrumentedStateBackend) Close() error {
	return isb.backend.Close()
}

// Helper functions for common metric patterns

// RecordCheckpointDuration records a checkpoint operation duration
func RecordCheckpointDuration(collector *metrics.Collector, duration time.Duration, success bool) {
	collector.CheckpointDuration.Observe(duration.Seconds())
	if success {
		collector.CheckpointSuccess.Inc()
	} else {
		collector.CheckpointFailure.Inc()
	}
}

// RecordWatermarkProgress updates watermark metrics
func RecordWatermarkProgress(collector *metrics.Collector, watermarkTime time.Time, partition int32) {
	// Update watermark timestamp
	collector.WatermarkProgress.Set(float64(watermarkTime.Unix()))

	// Calculate and record lag
	lag := time.Since(watermarkTime)
	collector.WatermarkLag.WithLabelValues(fmt.Sprintf("%d", partition)).Set(lag.Seconds())
}

// RecordWindowEvent records a window processing event
func RecordWindowEvent(collector *metrics.Collector, windowType string, eventCount int, isLate bool) {
	collector.WindowEventsProcessed.WithLabelValues(windowType).Add(float64(eventCount))
	if isLate {
		collector.WindowLateFired.WithLabelValues(windowType).Inc()
	}
}

// UpdateWindowSize updates the size of an active window
func UpdateWindowSize(collector *metrics.Collector, windowType, windowID string, size int) {
	collector.WindowSize.WithLabelValues(windowType, windowID).Set(float64(size))
}

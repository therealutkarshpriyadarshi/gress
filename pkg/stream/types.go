package stream

import (
	"context"
	"time"
)

// Event represents a single event in the stream
type Event struct {
	Key       string                 // Partition key for routing
	Value     interface{}            // Event payload
	EventTime time.Time              // Event-time timestamp
	Headers   map[string]string      // Metadata headers
	Offset    int64                  // Source offset for replay
	Partition int32                  // Source partition
}

// Watermark represents progress in event time
type Watermark struct {
	Timestamp time.Time // No events with timestamp < this should arrive
	Partition int32     // Partition this watermark applies to
}

// ProcessingContext provides context for event processing
type ProcessingContext struct {
	Ctx       context.Context
	Timestamp time.Time
	Window    *Window
	State     StateBackend
}

// Window represents a time window for aggregation
type Window struct {
	Start time.Time
	End   time.Time
	Type  WindowType
}

// WindowType defines the type of window
type WindowType int

const (
	TumblingWindow WindowType = iota
	SlidingWindow
	SessionWindow
)

// Checkpoint represents a snapshot of processing state
type Checkpoint struct {
	ID        string
	Timestamp time.Time
	Offsets   map[int32]int64 // Partition -> Offset
	StateData []byte          // Serialized state
}

// StreamMetrics holds processing metrics
type StreamMetrics struct {
	EventsProcessed   int64
	EventsFiltered    int64
	AvgLatencyMs      float64
	P99LatencyMs      float64
	CurrentWatermark  time.Time
	LastCheckpointAt  time.Time
	BackpressureCount int64
}

// Operator is the base interface for stream operations
type Operator interface {
	Process(ctx *ProcessingContext, event *Event) ([]*Event, error)
	Close() error
}

// Source produces events into the stream
type Source interface {
	Start(ctx context.Context, output chan<- *Event) error
	Stop() error
	Name() string
	Partitions() int32
}

// Sink consumes events from the stream
type Sink interface {
	Write(ctx context.Context, event *Event) error
	Flush(ctx context.Context) error
	Close() error
}

// StateBackend provides persistent state storage
type StateBackend interface {
	Get(key string) ([]byte, error)
	Put(key string, value []byte) error
	Delete(key string) error
	Snapshot() ([]byte, error)
	Restore(data []byte) error
	Close() error
}

// TransformFunc is a function for stateless transformations
type TransformFunc func(*Event) (*Event, error)

// FilterFunc is a predicate for filtering events
type FilterFunc func(*Event) bool

// FlatMapFunc transforms one event into multiple events
type FlatMapFunc func(*Event) ([]*Event, error)

// AggregateFunc combines multiple events into a single result
type AggregateFunc func(accumulator interface{}, event *Event) (interface{}, error)

// ReduceFunc combines two values into one
type ReduceFunc func(a, b interface{}) (interface{}, error)

// KeySelector extracts a key from an event for partitioning
type KeySelector func(*Event) string

// TimestampExtractor extracts event time from an event
type TimestampExtractor func(*Event) time.Time

package errors

import (
	"context"
	"sync"
)

// SideOutputTag identifies a side output stream
type SideOutputTag string

const (
	// ErrorSideOutput is the tag for error events
	ErrorSideOutput SideOutputTag = "errors"
	// LateEventSideOutput is the tag for late events
	LateEventSideOutput SideOutputTag = "late-events"
	// FilteredSideOutput is the tag for filtered events
	FilteredSideOutput SideOutputTag = "filtered"
)

// SideOutputEvent wraps an event with error metadata for side outputs
type SideOutputEvent struct {
	// Original event fields
	Key       string
	Value     interface{}
	EventTime interface{} // time.Time
	Headers   map[string]string
	Offset    int64
	Partition int32

	// Error metadata
	Error           error
	ErrorCategory   ErrorCategory
	OperatorName    string
	RetryCount      int
	Tag             SideOutputTag
	Metadata        map[string]interface{}
}

// SideOutputCollector collects events for side outputs
type SideOutputCollector struct {
	mu      sync.RWMutex
	outputs map[SideOutputTag]chan *SideOutputEvent
	enabled bool
}

// NewSideOutputCollector creates a new side output collector
func NewSideOutputCollector() *SideOutputCollector {
	return &SideOutputCollector{
		outputs: make(map[SideOutputTag]chan *SideOutputEvent),
		enabled: true,
	}
}

// RegisterSideOutput registers a channel for a side output tag
func (c *SideOutputCollector) RegisterSideOutput(tag SideOutputTag, ch chan *SideOutputEvent) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.outputs[tag] = ch
}

// UnregisterSideOutput removes a side output channel
func (c *SideOutputCollector) UnregisterSideOutput(tag SideOutputTag) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if ch, exists := c.outputs[tag]; exists {
		close(ch)
		delete(c.outputs, tag)
	}
}

// Emit emits an event to a side output (non-blocking)
func (c *SideOutputCollector) Emit(ctx context.Context, event *SideOutputEvent) {
	if !c.enabled {
		return
	}

	c.mu.RLock()
	ch, exists := c.outputs[event.Tag]
	c.mu.RUnlock()

	if !exists {
		return
	}

	// Non-blocking send
	select {
	case ch <- event:
		// Event sent successfully
	case <-ctx.Done():
		// Context cancelled
	default:
		// Channel full, drop event
		// In production, we might want to track dropped events
	}
}

// EmitBlocking emits an event to a side output (blocking)
func (c *SideOutputCollector) EmitBlocking(ctx context.Context, event *SideOutputEvent) error {
	if !c.enabled {
		return nil
	}

	c.mu.RLock()
	ch, exists := c.outputs[event.Tag]
	c.mu.RUnlock()

	if !exists {
		return nil
	}

	select {
	case ch <- event:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// GetChannel returns the channel for a side output tag
func (c *SideOutputCollector) GetChannel(tag SideOutputTag) (chan *SideOutputEvent, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	ch, exists := c.outputs[tag]
	return ch, exists
}

// HasSideOutput checks if a side output is registered
func (c *SideOutputCollector) HasSideOutput(tag SideOutputTag) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, exists := c.outputs[tag]
	return exists
}

// Close closes all side output channels
func (c *SideOutputCollector) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	for tag, ch := range c.outputs {
		close(ch)
		delete(c.outputs, tag)
	}
	c.enabled = false
}

// Enable enables side outputs
func (c *SideOutputCollector) Enable() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.enabled = true
}

// Disable disables side outputs
func (c *SideOutputCollector) Disable() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.enabled = false
}

// IsEnabled returns whether side outputs are enabled
func (c *SideOutputCollector) IsEnabled() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.enabled
}

// Tags returns all registered side output tags
func (c *SideOutputCollector) Tags() []SideOutputTag {
	c.mu.RLock()
	defer c.mu.RUnlock()

	tags := make([]SideOutputTag, 0, len(c.outputs))
	for tag := range c.outputs {
		tags = append(tags, tag)
	}
	return tags
}

// SideOutputHandler processes events from a side output
type SideOutputHandler func(ctx context.Context, event *SideOutputEvent) error

// StartSideOutputProcessor starts processing events from a side output
func StartSideOutputProcessor(
	ctx context.Context,
	collector *SideOutputCollector,
	tag SideOutputTag,
	handler SideOutputHandler,
) error {
	ch, exists := collector.GetChannel(tag)
	if !exists {
		// Register a new channel if it doesn't exist
		ch = make(chan *SideOutputEvent, 1000) // Buffered channel
		collector.RegisterSideOutput(tag, ch)
	}

	// Start processing in a goroutine
	go func() {
		for {
			select {
			case event, ok := <-ch:
				if !ok {
					// Channel closed
					return
				}
				if err := handler(ctx, event); err != nil {
					// Log error or handle it
					// In production, we might want to use a logger here
					_ = err
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	return nil
}

// SideOutputConfig configures side output behavior
type SideOutputConfig struct {
	// EnableErrorSideOutput enables routing errors to side output
	EnableErrorSideOutput bool
	// EnableLateSideOutput enables routing late events to side output
	EnableLateSideOutput bool
	// EnableFilteredSideOutput enables routing filtered events to side output
	EnableFilteredSideOutput bool
	// ErrorChannelSize is the buffer size for error side output channel
	ErrorChannelSize int
	// LateChannelSize is the buffer size for late event side output channel
	LateChannelSize int
	// FilteredChannelSize is the buffer size for filtered event side output channel
	FilteredChannelSize int
}

// DefaultSideOutputConfig returns default configuration
func DefaultSideOutputConfig() *SideOutputConfig {
	return &SideOutputConfig{
		EnableErrorSideOutput:    true,
		EnableLateSideOutput:     false,
		EnableFilteredSideOutput: false,
		ErrorChannelSize:         1000,
		LateChannelSize:          1000,
		FilteredChannelSize:      1000,
	}
}

// CreateSideOutputCollector creates and configures a side output collector
func CreateSideOutputCollector(config *SideOutputConfig) *SideOutputCollector {
	collector := NewSideOutputCollector()

	if config.EnableErrorSideOutput {
		ch := make(chan *SideOutputEvent, config.ErrorChannelSize)
		collector.RegisterSideOutput(ErrorSideOutput, ch)
	}

	if config.EnableLateSideOutput {
		ch := make(chan *SideOutputEvent, config.LateChannelSize)
		collector.RegisterSideOutput(LateEventSideOutput, ch)
	}

	if config.EnableFilteredSideOutput {
		ch := make(chan *SideOutputEvent, config.FilteredChannelSize)
		collector.RegisterSideOutput(FilteredSideOutput, ch)
	}

	return collector
}

// SideOutputStats tracks side output statistics
type SideOutputStats struct {
	mu     sync.RWMutex
	counts map[SideOutputTag]int64
}

// NewSideOutputStats creates a new stats tracker
func NewSideOutputStats() *SideOutputStats {
	return &SideOutputStats{
		counts: make(map[SideOutputTag]int64),
	}
}

// Increment increments the count for a tag
func (s *SideOutputStats) Increment(tag SideOutputTag) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.counts[tag]++
}

// Get returns the count for a tag
func (s *SideOutputStats) Get(tag SideOutputTag) int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.counts[tag]
}

// GetAll returns all counts
func (s *SideOutputStats) GetAll() map[SideOutputTag]int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[SideOutputTag]int64, len(s.counts))
	for k, v := range s.counts {
		result[k] = v
	}
	return result
}

// Reset resets all counts
func (s *SideOutputStats) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k := range s.counts {
		s.counts[k] = 0
	}
}

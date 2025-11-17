package window

import (
	"fmt"
	"sync"
	"time"

	"github.com/therealutkarshpriyadarshi/gress/pkg/stream"
	"go.uber.org/zap"
)

// Assigner assigns events to windows
type Assigner interface {
	AssignWindows(event *stream.Event) []*stream.Window
	IsEventTime() bool
}

// TumblingWindowAssigner assigns events to tumbling (non-overlapping) windows
type TumblingWindowAssigner struct {
	size      time.Duration
	offset    time.Duration
	eventTime bool
}

// NewTumblingWindow creates a tumbling window assigner
func NewTumblingWindow(size time.Duration) *TumblingWindowAssigner {
	return &TumblingWindowAssigner{
		size:      size,
		offset:    0,
		eventTime: true,
	}
}

// WithOffset sets a time offset for the window start
func (t *TumblingWindowAssigner) WithOffset(offset time.Duration) *TumblingWindowAssigner {
	t.offset = offset
	return t
}

// ProcessingTime configures the assigner to use processing time
func (t *TumblingWindowAssigner) ProcessingTime() *TumblingWindowAssigner {
	t.eventTime = false
	return t
}

// AssignWindows assigns an event to a tumbling window
func (t *TumblingWindowAssigner) AssignWindows(event *stream.Event) []*stream.Window {
	timestamp := event.EventTime
	if !t.eventTime {
		timestamp = time.Now()
	}

	// Calculate window start
	start := timestamp.Truncate(t.size).Add(t.offset)
	if start.After(timestamp) {
		start = start.Add(-t.size)
	}

	return []*stream.Window{
		{
			Start: start,
			End:   start.Add(t.size),
			Type:  stream.TumblingWindow,
		},
	}
}

// IsEventTime returns whether this assigner uses event time
func (t *TumblingWindowAssigner) IsEventTime() bool {
	return t.eventTime
}

// SlidingWindowAssigner assigns events to sliding (overlapping) windows
type SlidingWindowAssigner struct {
	size      time.Duration
	slide     time.Duration
	offset    time.Duration
	eventTime bool
}

// NewSlidingWindow creates a sliding window assigner
func NewSlidingWindow(size, slide time.Duration) *SlidingWindowAssigner {
	return &SlidingWindowAssigner{
		size:      size,
		slide:     slide,
		offset:    0,
		eventTime: true,
	}
}

// WithOffset sets a time offset for the window start
func (s *SlidingWindowAssigner) WithOffset(offset time.Duration) *SlidingWindowAssigner {
	s.offset = offset
	return s
}

// ProcessingTime configures the assigner to use processing time
func (s *SlidingWindowAssigner) ProcessingTime() *SlidingWindowAssigner {
	s.eventTime = false
	return s
}

// AssignWindows assigns an event to multiple sliding windows
func (s *SlidingWindowAssigner) AssignWindows(event *stream.Event) []*stream.Window {
	timestamp := event.EventTime
	if !s.eventTime {
		timestamp = time.Now()
	}

	var windows []*stream.Window

	// Find the last window start before the timestamp
	lastStart := timestamp.Truncate(s.slide).Add(s.offset)
	if lastStart.After(timestamp) {
		lastStart = lastStart.Add(-s.slide)
	}

	// Generate all windows that contain this timestamp
	for start := lastStart; start.Add(s.size).After(timestamp); start = start.Add(-s.slide) {
		if start.Add(s.size).After(timestamp) {
			windows = append(windows, &stream.Window{
				Start: start,
				End:   start.Add(s.size),
				Type:  stream.SlidingWindow,
			})
		}
		// Prevent infinite loop
		if len(windows) > 1000 {
			break
		}
	}

	return windows
}

// IsEventTime returns whether this assigner uses event time
func (s *SlidingWindowAssigner) IsEventTime() bool {
	return s.eventTime
}

// SessionWindowAssigner assigns events to session windows based on gaps
type SessionWindowAssigner struct {
	gap       time.Duration
	eventTime bool
}

// NewSessionWindow creates a session window assigner
func NewSessionWindow(gap time.Duration) *SessionWindowAssigner {
	return &SessionWindowAssigner{
		gap:       gap,
		eventTime: true,
	}
}

// ProcessingTime configures the assigner to use processing time
func (s *SessionWindowAssigner) ProcessingTime() *SessionWindowAssigner {
	s.eventTime = false
	return s
}

// AssignWindows assigns an event to a session window
// Note: Session windows are merged dynamically as events arrive
func (s *SessionWindowAssigner) AssignWindows(event *stream.Event) []*stream.Window {
	timestamp := event.EventTime
	if !s.eventTime {
		timestamp = time.Now()
	}

	// Initial window is a point window around the event
	return []*stream.Window{
		{
			Start: timestamp,
			End:   timestamp.Add(s.gap),
			Type:  stream.SessionWindow,
		},
	}
}

// IsEventTime returns whether this assigner uses event time
func (s *SessionWindowAssigner) IsEventTime() bool {
	return s.eventTime
}

// Trigger determines when a window should fire
type Trigger interface {
	OnElement(event *stream.Event, window *stream.Window) TriggerResult
	OnEventTime(time time.Time, window *stream.Window) TriggerResult
	OnProcessingTime(time time.Time, window *stream.Window) TriggerResult
}

// TriggerResult indicates what action to take
type TriggerResult int

const (
	Continue TriggerResult = iota // Continue without firing
	Fire                          // Fire the window
	Purge                         // Clear window state
	FireAndPurge                  // Fire and clear
)

// EventTimeTrigger fires when watermark passes window end
type EventTimeTrigger struct{}

// OnElement is called for each element
func (e *EventTimeTrigger) OnElement(event *stream.Event, window *stream.Window) TriggerResult {
	return Continue
}

// OnEventTime is called when watermark advances
func (e *EventTimeTrigger) OnEventTime(watermark time.Time, window *stream.Window) TriggerResult {
	if watermark.After(window.End) || watermark.Equal(window.End) {
		return Fire
	}
	return Continue
}

// OnProcessingTime is called on processing time timer
func (e *EventTimeTrigger) OnProcessingTime(time time.Time, window *stream.Window) TriggerResult {
	return Continue
}

// ProcessingTimeTrigger fires when processing time passes window end
type ProcessingTimeTrigger struct{}

// OnElement is called for each element
func (p *ProcessingTimeTrigger) OnElement(event *stream.Event, window *stream.Window) TriggerResult {
	return Continue
}

// OnEventTime is called when watermark advances
func (p *ProcessingTimeTrigger) OnEventTime(time time.Time, window *stream.Window) TriggerResult {
	return Continue
}

// OnProcessingTime is called on processing time timer
func (p *ProcessingTimeTrigger) OnProcessingTime(now time.Time, window *stream.Window) TriggerResult {
	if now.After(window.End) || now.Equal(window.End) {
		return Fire
	}
	return Continue
}

// CountTrigger fires when element count reaches threshold
type CountTrigger struct {
	maxCount int
	count    int
	mu       sync.Mutex
}

// NewCountTrigger creates a count-based trigger
func NewCountTrigger(maxCount int) *CountTrigger {
	return &CountTrigger{maxCount: maxCount}
}

// OnElement is called for each element
func (c *CountTrigger) OnElement(event *stream.Event, window *stream.Window) TriggerResult {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.count++
	if c.count >= c.maxCount {
		c.count = 0
		return Fire
	}
	return Continue
}

// OnEventTime is called when watermark advances
func (c *CountTrigger) OnEventTime(time time.Time, window *stream.Window) TriggerResult {
	return Continue
}

// OnProcessingTime is called on processing time timer
func (c *CountTrigger) OnProcessingTime(time time.Time, window *stream.Window) TriggerResult {
	return Continue
}

// WindowOperator manages windowed operations
type WindowOperator struct {
	assigner      Assigner
	trigger       Trigger
	windows       map[string]*WindowState // window key -> state
	aggregateFunc stream.AggregateFunc
	mu            sync.RWMutex
	logger        *zap.Logger
}

// WindowState tracks the state of a single window
type WindowState struct {
	Window      *stream.Window
	Events      []*stream.Event
	Accumulator interface{}
	Count       int
}

// NewWindowOperator creates a new window operator
func NewWindowOperator(assigner Assigner, trigger Trigger, aggregateFunc stream.AggregateFunc, logger *zap.Logger) *WindowOperator {
	return &WindowOperator{
		assigner:      assigner,
		trigger:       trigger,
		windows:       make(map[string]*WindowState),
		aggregateFunc: aggregateFunc,
		logger:        logger,
	}
}

// Process assigns events to windows and triggers computation
func (w *WindowOperator) Process(ctx *stream.ProcessingContext, event *stream.Event) ([]*stream.Event, error) {
	windows := w.assigner.AssignWindows(event)

	var results []*stream.Event

	for _, window := range windows {
		windowKey := w.windowKey(event.Key, window)

		w.mu.Lock()
		state, exists := w.windows[windowKey]
		if !exists {
			state = &WindowState{
				Window: window,
				Events: make([]*stream.Event, 0),
			}
			w.windows[windowKey] = state
		}

		// Add event to window
		state.Events = append(state.Events, event)
		state.Count++

		// Update accumulator
		var err error
		state.Accumulator, err = w.aggregateFunc(state.Accumulator, event)
		if err != nil {
			w.mu.Unlock()
			return nil, fmt.Errorf("aggregation error: %w", err)
		}

		w.mu.Unlock()

		// Check trigger
		triggerResult := w.trigger.OnElement(event, window)
		if triggerResult == Fire || triggerResult == FireAndPurge {
			result := w.fireWindow(windowKey, state)
			if result != nil {
				results = append(results, result)
			}

			if triggerResult == FireAndPurge {
				w.purgeWindow(windowKey)
			}
		}
	}

	return results, nil
}

// OnWatermark handles watermark advancement
func (w *WindowOperator) OnWatermark(watermark time.Time) []*stream.Event {
	w.mu.RLock()
	defer w.mu.RUnlock()

	var results []*stream.Event

	for key, state := range w.windows {
		triggerResult := w.trigger.OnEventTime(watermark, state.Window)
		if triggerResult == Fire || triggerResult == FireAndPurge {
			result := w.fireWindow(key, state)
			if result != nil {
				results = append(results, result)
			}

			if triggerResult == FireAndPurge {
				w.purgeWindow(key)
			}
		}
	}

	return results
}

// fireWindow emits the window result
func (w *WindowOperator) fireWindow(key string, state *WindowState) *stream.Event {
	w.logger.Debug("Firing window",
		zap.String("key", key),
		zap.Time("start", state.Window.Start),
		zap.Time("end", state.Window.End),
		zap.Int("count", state.Count))

	return &stream.Event{
		Key:       key,
		Value:     state.Accumulator,
		EventTime: state.Window.End,
		Headers: map[string]string{
			"window_start": state.Window.Start.Format(time.RFC3339),
			"window_end":   state.Window.End.Format(time.RFC3339),
			"window_count": fmt.Sprintf("%d", state.Count),
		},
	}
}

// purgeWindow cleans up window state
func (w *WindowOperator) purgeWindow(key string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	delete(w.windows, key)
}

// windowKey generates a unique key for a window
func (w *WindowOperator) windowKey(eventKey string, window *stream.Window) string {
	return fmt.Sprintf("%s-%d-%d", eventKey, window.Start.Unix(), window.End.Unix())
}

// Close cleans up resources
func (w *WindowOperator) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.windows = nil
	return nil
}

// GetState returns the current window state
func (w *WindowOperator) GetState() (map[string]interface{}, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	state := make(map[string]interface{})
	for k, v := range w.windows {
		state[k] = v.Accumulator
	}
	return state, nil
}

// RestoreState restores window state
func (w *WindowOperator) RestoreState(state map[string]interface{}) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Note: Full restoration would require serializing WindowState
	// This is a simplified version
	for k, v := range state {
		if ws, ok := w.windows[k]; ok {
			ws.Accumulator = v
		}
	}
	return nil
}

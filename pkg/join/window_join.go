package join

import (
	"fmt"
	"sync"
	"time"

	"github.com/therealutkarshpriyadarshi/gress/pkg/stream"
	"github.com/therealutkarshpriyadarshi/gress/pkg/window"
	"go.uber.org/zap"
)

// WindowJoinOperator performs joins within time windows
type WindowJoinOperator struct {
	config      *JoinConfig
	assigner    window.Assigner
	windowState map[string]*WindowJoinState // window key -> state
	condition   JoinCondition
	logger      *zap.Logger
	mu          sync.RWMutex
}

// WindowJoinState tracks join state for a specific window
type WindowJoinState struct {
	Window      *stream.Window
	LeftEvents  []*stream.Event
	RightEvents []*stream.Event
	Matches     []*JoinResult
	Fired       bool
}

// NewWindowJoinOperator creates a window-based join operator
func NewWindowJoinOperator(
	config *JoinConfig,
	assigner window.Assigner,
	logger *zap.Logger,
) *WindowJoinOperator {
	if config == nil {
		config = DefaultJoinConfig()
	}
	if config.Condition == nil {
		config.Condition = EqualityCondition()
	}

	return &WindowJoinOperator{
		config:      config,
		assigner:    assigner,
		windowState: make(map[string]*WindowJoinState),
		condition:   config.Condition,
		logger:      logger,
	}
}

// ProcessLeft processes an event from the left stream
func (wj *WindowJoinOperator) ProcessLeft(ctx *stream.ProcessingContext, event *stream.Event) ([]*stream.Event, error) {
	wj.mu.Lock()
	defer wj.mu.Unlock()

	// Assign event to windows
	windows := wj.assigner.AssignWindows(event)

	for _, win := range windows {
		windowKey := wj.getWindowKey(event.Key, win)

		// Get or create window state
		state, exists := wj.windowState[windowKey]
		if !exists {
			state = &WindowJoinState{
				Window:      win,
				LeftEvents:  make([]*stream.Event, 0),
				RightEvents: make([]*stream.Event, 0),
				Matches:     make([]*JoinResult, 0),
			}
			wj.windowState[windowKey] = state
		}

		// Add to left events
		state.LeftEvents = append(state.LeftEvents, event)

		// Try to match with right events
		wj.matchInWindow(state, event, LeftSide)
	}

	return []*stream.Event{}, nil // Results emitted on window fire
}

// ProcessRight processes an event from the right stream
func (wj *WindowJoinOperator) ProcessRight(ctx *stream.ProcessingContext, event *stream.Event) ([]*stream.Event, error) {
	wj.mu.Lock()
	defer wj.mu.Unlock()

	// Assign event to windows
	windows := wj.assigner.AssignWindows(event)

	for _, win := range windows {
		windowKey := wj.getWindowKey(event.Key, win)

		// Get or create window state
		state, exists := wj.windowState[windowKey]
		if !exists {
			state = &WindowJoinState{
				Window:      win,
				LeftEvents:  make([]*stream.Event, 0),
				RightEvents: make([]*stream.Event, 0),
				Matches:     make([]*JoinResult, 0),
			}
			wj.windowState[windowKey] = state
		}

		// Add to right events
		state.RightEvents = append(state.RightEvents, event)

		// Try to match with left events
		wj.matchInWindow(state, event, RightSide)
	}

	return []*stream.Event{}, nil // Results emitted on window fire
}

// matchInWindow tries to match an event with existing events in the window
func (wj *WindowJoinOperator) matchInWindow(state *WindowJoinState, event *stream.Event, side Side) {
	if side == LeftSide {
		// Match left event with right events
		for _, rightEvent := range state.RightEvents {
			if wj.condition(event, rightEvent) {
				result := &JoinResult{
					Left:      event,
					Right:     rightEvent,
					JoinKey:   event.Key,
					EventTime: state.Window.End,
				}
				state.Matches = append(state.Matches, result)
			}
		}
	} else {
		// Match right event with left events
		for _, leftEvent := range state.LeftEvents {
			if wj.condition(leftEvent, event) {
				result := &JoinResult{
					Left:      leftEvent,
					Right:     event,
					JoinKey:   event.Key,
					EventTime: state.Window.End,
				}
				state.Matches = append(state.Matches, result)
			}
		}
	}
}

// OnWatermark triggers window evaluation when watermark advances
func (wj *WindowJoinOperator) OnWatermark(watermark time.Time) []*stream.Event {
	wj.mu.Lock()
	defer wj.mu.Unlock()

	var results []*stream.Event

	for windowKey, state := range wj.windowState {
		// Check if window should fire
		if !state.Fired && (watermark.After(state.Window.End) || watermark.Equal(state.Window.End)) {
			windowResults := wj.fireWindow(state)
			results = append(results, windowResults...)
			state.Fired = true

			// Clean up fired windows
			delete(wj.windowState, windowKey)
		}
	}

	return results
}

// fireWindow emits results for a completed window
func (wj *WindowJoinOperator) fireWindow(state *WindowJoinState) []*stream.Event {
	var results []*stream.Event

	switch wj.config.Type {
	case InnerJoin:
		// Emit only matches
		for _, match := range state.Matches {
			results = append(results, match.ToEvent())
		}

	case LeftOuterJoin:
		// Emit matches
		for _, match := range state.Matches {
			results = append(results, match.ToEvent())
		}
		// Emit unmatched left events
		matched := make(map[*stream.Event]bool)
		for _, match := range state.Matches {
			matched[match.Left] = true
		}
		for _, leftEvent := range state.LeftEvents {
			if !matched[leftEvent] {
				result := &JoinResult{
					Left:      leftEvent,
					Right:     nil,
					JoinKey:   leftEvent.Key,
					EventTime: state.Window.End,
				}
				results = append(results, result.ToEvent())
			}
		}

	case RightOuterJoin:
		// Emit matches
		for _, match := range state.Matches {
			results = append(results, match.ToEvent())
		}
		// Emit unmatched right events
		matched := make(map[*stream.Event]bool)
		for _, match := range state.Matches {
			matched[match.Right] = true
		}
		for _, rightEvent := range state.RightEvents {
			if !matched[rightEvent] {
				result := &JoinResult{
					Left:      nil,
					Right:     rightEvent,
					JoinKey:   rightEvent.Key,
					EventTime: state.Window.End,
				}
				results = append(results, result.ToEvent())
			}
		}

	case FullOuterJoin:
		// Emit matches
		for _, match := range state.Matches {
			results = append(results, match.ToEvent())
		}
		// Emit unmatched left events
		matchedLeft := make(map[*stream.Event]bool)
		for _, match := range state.Matches {
			matchedLeft[match.Left] = true
		}
		for _, leftEvent := range state.LeftEvents {
			if !matchedLeft[leftEvent] {
				result := &JoinResult{
					Left:      leftEvent,
					Right:     nil,
					JoinKey:   leftEvent.Key,
					EventTime: state.Window.End,
				}
				results = append(results, result.ToEvent())
			}
		}
		// Emit unmatched right events
		matchedRight := make(map[*stream.Event]bool)
		for _, match := range state.Matches {
			matchedRight[match.Right] = true
		}
		for _, rightEvent := range state.RightEvents {
			if !matchedRight[rightEvent] {
				result := &JoinResult{
					Left:      nil,
					Right:     rightEvent,
					JoinKey:   rightEvent.Key,
					EventTime: state.Window.End,
				}
				results = append(results, result.ToEvent())
			}
		}
	}

	wj.logger.Debug("Window join fired",
		zap.Time("window_start", state.Window.Start),
		zap.Time("window_end", state.Window.End),
		zap.Int("left_events", len(state.LeftEvents)),
		zap.Int("right_events", len(state.RightEvents)),
		zap.Int("matches", len(state.Matches)),
		zap.Int("results", len(results)))

	return results
}

// getWindowKey generates a unique key for a window
func (wj *WindowJoinOperator) getWindowKey(eventKey string, win *stream.Window) string {
	return fmt.Sprintf("%s-%d-%d", eventKey, win.Start.Unix(), win.End.Unix())
}

// Process implements the Operator interface
func (wj *WindowJoinOperator) Process(ctx *stream.ProcessingContext, event *stream.Event) ([]*stream.Event, error) {
	return nil, fmt.Errorf("window join operator requires explicit ProcessLeft or ProcessRight calls")
}

// Close cleans up resources
func (wj *WindowJoinOperator) Close() error {
	wj.mu.Lock()
	defer wj.mu.Unlock()

	wj.windowState = nil
	return nil
}

// GetMetrics returns window join metrics
func (wj *WindowJoinOperator) GetMetrics() WindowJoinMetrics {
	wj.mu.RLock()
	defer wj.mu.RUnlock()

	metrics := WindowJoinMetrics{
		ActiveWindows: len(wj.windowState),
	}

	for _, state := range wj.windowState {
		metrics.TotalLeftEvents += len(state.LeftEvents)
		metrics.TotalRightEvents += len(state.RightEvents)
		metrics.TotalMatches += len(state.Matches)
	}

	return metrics
}

// WindowJoinMetrics tracks window join metrics
type WindowJoinMetrics struct {
	ActiveWindows    int
	TotalLeftEvents  int
	TotalRightEvents int
	TotalMatches     int
}

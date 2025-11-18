package join

import (
	"sync"
	"time"

	"github.com/therealutkarshpriyadarshi/gress/pkg/stream"
)

// JoinState manages state for join operations
type JoinState struct {
	leftState  map[string][]*StoredEvent // key -> events from left stream
	rightState map[string][]*StoredEvent // key -> events from right stream
	config     *JoinConfig
	mu         sync.RWMutex
	metrics    JoinMetrics
}

// JoinMetrics tracks join operation metrics
type JoinMetrics struct {
	LeftEventsReceived  int64
	RightEventsReceived int64
	MatchesFound        int64
	LeftUnmatched       int64
	RightUnmatched      int64
	StateSize           int64
	ExpiredEvents       int64
	CleanupRuns         int64
}

// NewJoinState creates a new join state manager
func NewJoinState(config *JoinConfig) *JoinState {
	return &JoinState{
		leftState:  make(map[string][]*StoredEvent),
		rightState: make(map[string][]*StoredEvent),
		config:     config,
	}
}

// AddLeftEvent stores an event from the left stream
func (js *JoinState) AddLeftEvent(event *stream.Event, expiresAt time.Time) {
	js.mu.Lock()
	defer js.mu.Unlock()

	stored := &StoredEvent{
		Event:     event,
		Side:      LeftSide,
		StoredAt:  time.Now(),
		ExpiresAt: expiresAt,
	}

	key := event.Key
	js.leftState[key] = append(js.leftState[key], stored)
	js.metrics.LeftEventsReceived++
	js.metrics.StateSize++

	// Enforce max state size
	js.enforceMaxStateSize(key, LeftSide)
}

// AddRightEvent stores an event from the right stream
func (js *JoinState) AddRightEvent(event *stream.Event, expiresAt time.Time) {
	js.mu.Lock()
	defer js.mu.Unlock()

	stored := &StoredEvent{
		Event:     event,
		Side:      RightSide,
		StoredAt:  time.Now(),
		ExpiresAt: expiresAt,
	}

	key := event.Key
	js.rightState[key] = append(js.rightState[key], stored)
	js.metrics.RightEventsReceived++
	js.metrics.StateSize++

	// Enforce max state size
	js.enforceMaxStateSize(key, RightSide)
}

// GetLeftEvents returns stored events from left stream for a key
func (js *JoinState) GetLeftEvents(key string) []*StoredEvent {
	js.mu.RLock()
	defer js.mu.RUnlock()

	events := js.leftState[key]
	result := make([]*StoredEvent, len(events))
	copy(result, events)
	return result
}

// GetRightEvents returns stored events from right stream for a key
func (js *JoinState) GetRightEvents(key string) []*StoredEvent {
	js.mu.RLock()
	defer js.mu.RUnlock()

	events := js.rightState[key]
	result := make([]*StoredEvent, len(events))
	copy(result, events)
	return result
}

// GetAllLeftKeys returns all keys in left state
func (js *JoinState) GetAllLeftKeys() []string {
	js.mu.RLock()
	defer js.mu.RUnlock()

	keys := make([]string, 0, len(js.leftState))
	for key := range js.leftState {
		keys = append(keys, key)
	}
	return keys
}

// GetAllRightKeys returns all keys in right state
func (js *JoinState) GetAllRightKeys() []string {
	js.mu.RLock()
	defer js.mu.RUnlock()

	keys := make([]string, 0, len(js.rightState))
	for key := range js.rightState {
		keys = append(keys, key)
	}
	return keys
}

// IncrementMatchCount increments the match count for a stored event
func (js *JoinState) IncrementMatchCount(event *StoredEvent) {
	js.mu.Lock()
	defer js.mu.Unlock()
	event.MatchCount++
	js.metrics.MatchesFound++
}

// RemoveExpiredEvents removes expired events and returns them
func (js *JoinState) RemoveExpiredEvents(now time.Time) (left, right []*StoredEvent) {
	js.mu.Lock()
	defer js.mu.Unlock()

	left = js.removeExpiredFromSide(now, LeftSide)
	right = js.removeExpiredFromSide(now, RightSide)

	js.metrics.ExpiredEvents += int64(len(left) + len(right))
	js.metrics.CleanupRuns++

	return left, right
}

// removeExpiredFromSide removes expired events from one side
func (js *JoinState) removeExpiredFromSide(now time.Time, side Side) []*StoredEvent {
	var expired []*StoredEvent
	stateMap := js.leftState
	if side == RightSide {
		stateMap = js.rightState
	}

	for key, events := range stateMap {
		var kept []*StoredEvent
		for _, se := range events {
			if se.IsExpired(now) {
				expired = append(expired, se)
				js.metrics.StateSize--
			} else {
				kept = append(kept, se)
			}
		}

		if len(kept) == 0 {
			delete(stateMap, key)
		} else {
			stateMap[key] = kept
		}
	}

	return expired
}

// enforceMaxStateSize removes oldest events if state exceeds max size
func (js *JoinState) enforceMaxStateSize(key string, side Side) {
	if js.config.MaxStateSize <= 0 {
		return
	}

	stateMap := js.leftState
	if side == RightSide {
		stateMap = js.rightState
	}

	events := stateMap[key]
	if len(events) > js.config.MaxStateSize {
		// Keep the most recent events
		removed := len(events) - js.config.MaxStateSize
		stateMap[key] = events[removed:]
		js.metrics.StateSize -= int64(removed)
	}
}

// GetMetrics returns current join metrics
func (js *JoinState) GetMetrics() JoinMetrics {
	js.mu.RLock()
	defer js.mu.RUnlock()
	return js.metrics
}

// Clear removes all state
func (js *JoinState) Clear() {
	js.mu.Lock()
	defer js.mu.Unlock()

	js.leftState = make(map[string][]*StoredEvent)
	js.rightState = make(map[string][]*StoredEvent)
	js.metrics.StateSize = 0
}

// Size returns the total number of stored events
func (js *JoinState) Size() int {
	js.mu.RLock()
	defer js.mu.RUnlock()

	total := 0
	for _, events := range js.leftState {
		total += len(events)
	}
	for _, events := range js.rightState {
		total += len(events)
	}
	return total
}

package cep

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/therealutkarshpriyadarshi/gress/pkg/stream"
)

// Pattern represents a Complex Event Processing pattern
type Pattern struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Events      []EventPattern `json:"events"`
	TimeWindow  time.Duration `json:"time_window"`
	Conditions  []Condition   `json:"conditions"`
}

// EventPattern defines a pattern to match against events
type EventPattern struct {
	Name       string                 `json:"name"`
	Type       string                 `json:"type"`
	Predicates map[string]interface{} `json:"predicates"`
	Optional   bool                   `json:"optional"`
}

// Condition represents a condition between events in a pattern
type Condition struct {
	LeftEvent  string `json:"left_event"`
	RightEvent string `json:"right_event"`
	Operator   string `json:"operator"` // "eq", "ne", "gt", "lt", "gte", "lte"
	Field      string `json:"field"`
}

// MatchedPattern represents a detected pattern with matched events
type MatchedPattern struct {
	PatternID     string                   `json:"pattern_id"`
	PatternName   string                   `json:"pattern_name"`
	MatchedEvents []map[string]interface{} `json:"matched_events"`
	FirstEventTime time.Time               `json:"first_event_time"`
	LastEventTime  time.Time               `json:"last_event_time"`
	Severity       string                  `json:"severity"` // "low", "medium", "high", "critical"
	Metadata       map[string]interface{}  `json:"metadata"`
}

// PatternMatcher maintains state for pattern matching
type PatternMatcher struct {
	mu              sync.RWMutex
	pattern         Pattern
	eventBuffer     []map[string]interface{}
	maxBufferSize   int
	matchedPatterns []MatchedPattern
}

// NewPatternMatcher creates a new pattern matcher
func NewPatternMatcher(pattern Pattern, maxBufferSize int) *PatternMatcher {
	return &PatternMatcher{
		pattern:         pattern,
		eventBuffer:     make([]map[string]interface{}, 0),
		maxBufferSize:   maxBufferSize,
		matchedPatterns: make([]MatchedPattern, 0),
	}
}

// AddEvent adds an event to the buffer and checks for pattern matches
func (pm *PatternMatcher) AddEvent(event map[string]interface{}) []MatchedPattern {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// Add event to buffer
	pm.eventBuffer = append(pm.eventBuffer, event)

	// Remove old events outside the time window
	pm.pruneOldEvents()

	// Limit buffer size
	if len(pm.eventBuffer) > pm.maxBufferSize {
		pm.eventBuffer = pm.eventBuffer[len(pm.eventBuffer)-pm.maxBufferSize:]
	}

	// Check for pattern matches
	matches := pm.findMatches()

	return matches
}

// pruneOldEvents removes events outside the time window
func (pm *PatternMatcher) pruneOldEvents() {
	if len(pm.eventBuffer) == 0 {
		return
	}

	now := time.Now()
	cutoffTime := now.Add(-pm.pattern.TimeWindow)

	newBuffer := make([]map[string]interface{}, 0)
	for _, event := range pm.eventBuffer {
		if eventTime, ok := event["timestamp"].(time.Time); ok {
			if eventTime.After(cutoffTime) {
				newBuffer = append(newBuffer, event)
			}
		}
	}

	pm.eventBuffer = newBuffer
}

// findMatches searches for pattern matches in the event buffer
func (pm *PatternMatcher) findMatches() []MatchedPattern {
	matches := make([]MatchedPattern, 0)

	// Simple sequential pattern matching
	// For each possible starting position, try to match the pattern
	for i := 0; i < len(pm.eventBuffer); i++ {
		if match := pm.tryMatch(i); match != nil {
			matches = append(matches, *match)
		}
	}

	return matches
}

// tryMatch attempts to match the pattern starting at the given index
func (pm *PatternMatcher) tryMatch(startIdx int) *MatchedPattern {
	matchedEvents := make([]map[string]interface{}, 0)
	patternIdx := 0

	for i := startIdx; i < len(pm.eventBuffer) && patternIdx < len(pm.pattern.Events); i++ {
		event := pm.eventBuffer[i]
		expectedPattern := pm.pattern.Events[patternIdx]

		if pm.matchesEventPattern(event, expectedPattern) {
			matchedEvents = append(matchedEvents, event)
			patternIdx++
		} else if !expectedPattern.Optional {
			// Required event not matched
			return nil
		}
	}

	// Check if all required events are matched
	if patternIdx < len(pm.pattern.Events) {
		return nil
	}

	// Check conditions between matched events
	if !pm.checkConditions(matchedEvents) {
		return nil
	}

	// Pattern matched!
	firstEventTime := matchedEvents[0]["timestamp"].(time.Time)
	lastEventTime := matchedEvents[len(matchedEvents)-1]["timestamp"].(time.Time)

	return &MatchedPattern{
		PatternID:     pm.pattern.ID,
		PatternName:   pm.pattern.Name,
		MatchedEvents: matchedEvents,
		FirstEventTime: firstEventTime,
		LastEventTime:  lastEventTime,
		Severity:       pm.calculateSeverity(matchedEvents),
		Metadata:       pm.extractMetadata(matchedEvents),
	}
}

// matchesEventPattern checks if an event matches the expected pattern
func (pm *PatternMatcher) matchesEventPattern(event map[string]interface{}, pattern EventPattern) bool {
	// Check event type
	if eventType, ok := event["type"].(string); ok {
		if eventType != pattern.Type {
			return false
		}
	}

	// Check predicates
	for key, expectedValue := range pattern.Predicates {
		if actualValue, exists := event[key]; !exists || actualValue != expectedValue {
			return false
		}
	}

	return true
}

// checkConditions verifies that conditions between events are satisfied
func (pm *PatternMatcher) checkConditions(events []map[string]interface{}) bool {
	eventMap := make(map[string]map[string]interface{})
	for i, event := range events {
		if name, ok := event["name"].(string); ok {
			eventMap[name] = event
		} else {
			eventMap[fmt.Sprintf("event_%d", i)] = event
		}
	}

	for _, condition := range pm.pattern.Conditions {
		leftEvent, leftOk := eventMap[condition.LeftEvent]
		rightEvent, rightOk := eventMap[condition.RightEvent]

		if !leftOk || !rightOk {
			return false
		}

		if !pm.evaluateCondition(leftEvent, rightEvent, condition) {
			return false
		}
	}

	return true
}

// evaluateCondition evaluates a condition between two events
func (pm *PatternMatcher) evaluateCondition(left, right map[string]interface{}, condition Condition) bool {
	leftValue := left[condition.Field]
	rightValue := right[condition.Field]

	switch condition.Operator {
	case "eq":
		return leftValue == rightValue
	case "ne":
		return leftValue != rightValue
	case "gt":
		return compareValues(leftValue, rightValue) > 0
	case "lt":
		return compareValues(leftValue, rightValue) < 0
	case "gte":
		return compareValues(leftValue, rightValue) >= 0
	case "lte":
		return compareValues(leftValue, rightValue) <= 0
	default:
		return false
	}
}

// compareValues compares two values and returns -1, 0, or 1
func compareValues(a, b interface{}) int {
	switch v1 := a.(type) {
	case float64:
		v2, _ := b.(float64)
		if v1 < v2 {
			return -1
		} else if v1 > v2 {
			return 1
		}
		return 0
	case int:
		v2, _ := b.(int)
		if v1 < v2 {
			return -1
		} else if v1 > v2 {
			return 1
		}
		return 0
	case string:
		v2, _ := b.(string)
		if v1 < v2 {
			return -1
		} else if v1 > v2 {
			return 1
		}
		return 0
	case time.Time:
		v2, _ := b.(time.Time)
		if v1.Before(v2) {
			return -1
		} else if v1.After(v2) {
			return 1
		}
		return 0
	default:
		return 0
	}
}

// calculateSeverity determines the severity of a matched pattern
func (pm *PatternMatcher) calculateSeverity(events []map[string]interface{}) string {
	// Simple heuristic: more events in less time = higher severity
	if len(events) == 0 {
		return "low"
	}

	firstTime := events[0]["timestamp"].(time.Time)
	lastTime := events[len(events)-1]["timestamp"].(time.Time)
	duration := lastTime.Sub(firstTime)

	eventRate := float64(len(events)) / duration.Minutes()

	if eventRate > 10 {
		return "critical"
	} else if eventRate > 5 {
		return "high"
	} else if eventRate > 2 {
		return "medium"
	}
	return "low"
}

// extractMetadata extracts relevant metadata from matched events
func (pm *PatternMatcher) extractMetadata(events []map[string]interface{}) map[string]interface{} {
	metadata := make(map[string]interface{})
	metadata["event_count"] = len(events)

	// Extract unique user IDs
	uniqueUsers := make(map[string]bool)
	for _, event := range events {
		if userID, ok := event["user_id"].(string); ok {
			uniqueUsers[userID] = true
		}
	}
	metadata["unique_users"] = len(uniqueUsers)

	// Extract unique IP addresses
	uniqueIPs := make(map[string]bool)
	for _, event := range events {
		if ip, ok := event["ip_address"].(string); ok {
			uniqueIPs[ip] = true
		}
	}
	metadata["unique_ips"] = len(uniqueIPs)

	return metadata
}

// PatternDetector creates a stream operator for pattern detection
func PatternDetector(pattern Pattern, maxBufferSize int) stream.Operator {
	matcher := NewPatternMatcher(pattern, maxBufferSize)

	return stream.MapOperator(func(event *stream.Event) (*stream.Event, error) {
		var eventData map[string]interface{}
		if err := json.Unmarshal(event.Data, &eventData); err != nil {
			return nil, fmt.Errorf("failed to parse event data: %w", err)
		}

		// Add timestamp if not present
		if _, exists := eventData["timestamp"]; !exists {
			eventData["timestamp"] = event.Timestamp
		}

		// Check for pattern matches
		matches := matcher.AddEvent(eventData)

		// If patterns matched, emit them
		if len(matches) > 0 {
			for _, match := range matches {
				matchData, err := json.Marshal(match)
				if err != nil {
					return nil, fmt.Errorf("failed to marshal pattern match: %w", err)
				}

				return &stream.Event{
					Key:       []byte(match.PatternID),
					Data:      matchData,
					Timestamp: match.LastEventTime,
					Headers:   event.Headers,
					Partition: event.Partition,
					Offset:    event.Offset,
				}, nil
			}
		}

		return nil, nil // No match found
	})
}

// SequencePattern creates a pattern for sequential event detection
func SequencePattern(id, name string, eventTypes []string, window time.Duration) Pattern {
	events := make([]EventPattern, len(eventTypes))
	for i, eventType := range eventTypes {
		events[i] = EventPattern{
			Name:     fmt.Sprintf("event_%d", i),
			Type:     eventType,
			Optional: false,
		}
	}

	return Pattern{
		ID:          id,
		Name:        name,
		Description: fmt.Sprintf("Sequential pattern: %v", eventTypes),
		Events:      events,
		TimeWindow:  window,
		Conditions:  []Condition{},
	}
}

// FrequencyPattern creates a pattern for detecting event frequency anomalies
func FrequencyPattern(id, name, eventType string, minCount int, window time.Duration) Pattern {
	return Pattern{
		ID:          id,
		Name:        name,
		Description: fmt.Sprintf("Frequency pattern: %s >= %d times in %s", eventType, minCount, window),
		Events:      []EventPattern{{Type: eventType}},
		TimeWindow:  window,
	}
}

package cep

import (
	"fmt"
	"sync"
	"time"
)

// PartialMatch represents a partial pattern match in progress
type PartialMatch struct {
	ID             string
	PatternID      string
	MatchedEvents  []map[string]interface{}
	CurrentStep    int
	StartTime      time.Time
	LastEventTime  time.Time
	EventCounts    map[int]int // Track event counts for quantified patterns
	NegationBuffer []map[string]interface{}
}

// CEPEngine is an advanced Complex Event Processing engine
type CEPEngine struct {
	mu              sync.RWMutex
	patterns        map[string]Pattern
	partialMatches  map[string][]*PartialMatch
	eventBuffer     []map[string]interface{}
	maxBufferSize   int
	timeoutDuration time.Duration
	matchCallback   func(MatchedPattern)
}

// NewCEPEngine creates a new CEP engine
func NewCEPEngine(maxBufferSize int, timeoutDuration time.Duration) *CEPEngine {
	engine := &CEPEngine{
		patterns:        make(map[string]Pattern),
		partialMatches:  make(map[string][]*PartialMatch),
		eventBuffer:     make([]map[string]interface{}, 0),
		maxBufferSize:   maxBufferSize,
		timeoutDuration: timeoutDuration,
	}

	// Start background timeout checker
	go engine.timeoutChecker()

	return engine
}

// RegisterPattern registers a pattern with the engine
func (e *CEPEngine) RegisterPattern(pattern Pattern) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.patterns[pattern.ID] = pattern
	e.partialMatches[pattern.ID] = make([]*PartialMatch, 0)
}

// UnregisterPattern removes a pattern from the engine
func (e *CEPEngine) UnregisterPattern(patternID string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.patterns, patternID)
	delete(e.partialMatches, patternID)
}

// OnMatch sets a callback for when patterns are matched
func (e *CEPEngine) OnMatch(callback func(MatchedPattern)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.matchCallback = callback
}

// ProcessEvent processes an incoming event
func (e *CEPEngine) ProcessEvent(event map[string]interface{}) []MatchedPattern {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Add timestamp if not present
	if _, exists := event["timestamp"]; !exists {
		event["timestamp"] = time.Now()
	}

	// Add to event buffer
	e.eventBuffer = append(e.eventBuffer, event)
	if len(e.eventBuffer) > e.maxBufferSize {
		e.eventBuffer = e.eventBuffer[len(e.eventBuffer)-e.maxBufferSize:]
	}

	matches := make([]MatchedPattern, 0)

	// Process each registered pattern
	for patternID, pattern := range e.patterns {
		// Check existing partial matches
		e.partialMatches[patternID] = e.advancePartialMatches(
			pattern,
			e.partialMatches[patternID],
			event,
			&matches,
		)

		// Try to start new partial matches
		if e.canStartNewMatch(pattern, event) {
			newMatch := e.createPartialMatch(pattern, event)
			e.partialMatches[patternID] = append(e.partialMatches[patternID], newMatch)
		}
	}

	// Invoke callback for each match
	if e.matchCallback != nil {
		for _, match := range matches {
			e.matchCallback(match)
		}
	}

	return matches
}

// canStartNewMatch checks if an event can start a new partial match
func (e *CEPEngine) canStartNewMatch(pattern Pattern, event map[string]interface{}) bool {
	if len(pattern.Events) == 0 {
		return false
	}

	firstEvent := pattern.Events[0]

	// Cannot start with a negated event
	if firstEvent.Negated {
		return false
	}

	return e.matchesEventPattern(event, firstEvent)
}

// createPartialMatch creates a new partial match
func (e *CEPEngine) createPartialMatch(pattern Pattern, event map[string]interface{}) *PartialMatch {
	eventTime := event["timestamp"].(time.Time)

	// Check if the first event's quantifier is satisfied immediately
	firstPattern := pattern.Events[0]
	currentStep := 0
	if e.isQuantifierSatisfied(firstPattern, 1) {
		currentStep = 1 // Advance to next step if quantifier satisfied
	}

	return &PartialMatch{
		ID:             fmt.Sprintf("%s-%d", pattern.ID, time.Now().UnixNano()),
		PatternID:      pattern.ID,
		MatchedEvents:  []map[string]interface{}{event},
		CurrentStep:    currentStep,
		StartTime:      eventTime,
		LastEventTime:  eventTime,
		EventCounts:    map[int]int{0: 1},
		NegationBuffer: make([]map[string]interface{}, 0),
	}
}

// advancePartialMatches tries to advance partial matches with the new event
func (e *CEPEngine) advancePartialMatches(
	pattern Pattern,
	partialMatches []*PartialMatch,
	event map[string]interface{},
	completedMatches *[]MatchedPattern,
) []*PartialMatch {
	activeMatches := make([]*PartialMatch, 0)
	eventTime := event["timestamp"].(time.Time)

	for _, pm := range partialMatches {
		// Check if match has timed out
		if e.hasTimedOut(pm, pattern) {
			continue
		}

		// Check time window constraint
		if pattern.TimeWindow > 0 {
			if eventTime.Sub(pm.StartTime) > pattern.TimeWindow {
				continue
			}
		}

		// Try to advance this partial match
		advanced, completed := e.tryAdvanceMatch(pattern, pm, event)

		if completed {
			// Pattern fully matched!
			match := e.createMatchedPattern(pattern, pm)
			*completedMatches = append(*completedMatches, match)
		} else if advanced {
			// Keep this partial match active
			activeMatches = append(activeMatches, pm)
		}
		// If not advanced and not completed, drop this partial match
	}

	return activeMatches
}

// tryAdvanceMatch tries to advance a partial match with a new event
func (e *CEPEngine) tryAdvanceMatch(
	pattern Pattern,
	pm *PartialMatch,
	event map[string]interface{},
) (advanced bool, completed bool) {
	if pm.CurrentStep >= len(pattern.Events) {
		return false, true
	}

	currentPattern := pattern.Events[pm.CurrentStep]
	eventTime := event["timestamp"].(time.Time)

	// Handle negated patterns
	if currentPattern.Negated {
		// Store event in negation buffer to check later
		pm.NegationBuffer = append(pm.NegationBuffer, event)

		// Check if any event in negation buffer matches the negated pattern
		for _, negEvent := range pm.NegationBuffer {
			if e.matchesEventPattern(negEvent, currentPattern) {
				// Negation violated - match failed
				return false, false
			}
		}

		// Check temporal constraint for negation
		if currentPattern.TemporalConstraint != nil {
			if eventTime.Sub(pm.LastEventTime) > currentPattern.TemporalConstraint.Duration {
				// Negation period passed without violation - advance
				pm.CurrentStep++
				pm.NegationBuffer = make([]map[string]interface{}, 0)
				return e.checkIfCompleted(pattern, pm)
			}
		}

		return true, false // Keep match active, waiting for negation to complete
	}

	// Handle normal and quantified patterns
	if !e.matchesEventPattern(event, currentPattern) {
		// Event doesn't match current pattern step
		if currentPattern.Optional {
			// Skip optional event and try next step
			pm.CurrentStep++
			return e.tryAdvanceMatch(pattern, pm, event)
		}
		return false, false
	}

	// Event matches! Check temporal constraints
	if currentPattern.TemporalConstraint != nil {
		timeDiff := eventTime.Sub(pm.LastEventTime)
		switch currentPattern.TemporalConstraint.Type {
		case TemporalWithin:
			if timeDiff > currentPattern.TemporalConstraint.Duration {
				return false, false // Temporal constraint violated
			}
		case TemporalAfter:
			if timeDiff < currentPattern.TemporalConstraint.Duration {
				return true, false // Not enough time has passed yet
			}
		case TemporalBefore:
			if timeDiff > currentPattern.TemporalConstraint.Duration {
				return false, false // Too much time has passed
			}
		}
	}

	// Handle quantified patterns
	currentCount := pm.EventCounts[pm.CurrentStep]
	pm.EventCounts[pm.CurrentStep] = currentCount + 1
	pm.MatchedEvents = append(pm.MatchedEvents, event)
	pm.LastEventTime = eventTime

	// Check if quantifier is satisfied
	if e.isQuantifierSatisfied(currentPattern, pm.EventCounts[pm.CurrentStep]) {
		pm.CurrentStep++
		return e.checkIfCompleted(pattern, pm)
	}

	return true, false // Keep accumulating events for quantifier
}

// isQuantifierSatisfied checks if the quantifier condition is met
func (e *CEPEngine) isQuantifierSatisfied(pattern EventPattern, count int) bool {
	switch pattern.Quantifier {
	case QuantifierExactlyOne:
		return count >= 1
	case QuantifierExactly:
		return count >= pattern.MinOccurrences
	case QuantifierAtLeast:
		return count >= pattern.MinOccurrences
	case QuantifierAtMost:
		return count >= 1 && count <= pattern.MaxOccurrences
	case QuantifierRange:
		return count >= pattern.MinOccurrences
	case QuantifierOneOrMore:
		return count >= 1
	case QuantifierZeroOrMore:
		return true // Always satisfied
	default:
		return count >= 1
	}
}

// checkIfCompleted checks if a partial match has completed
func (e *CEPEngine) checkIfCompleted(pattern Pattern, pm *PartialMatch) (advanced bool, completed bool) {
	// Check if we've matched all steps
	if pm.CurrentStep >= len(pattern.Events) {
		// Verify all conditions are satisfied
		if e.checkConditions(pm.MatchedEvents, pattern.Conditions) {
			return false, true
		}
		return false, false
	}
	return true, false
}

// matchesEventPattern checks if an event matches an event pattern
func (e *CEPEngine) matchesEventPattern(event map[string]interface{}, pattern EventPattern) bool {
	// Check event type
	if eventType, ok := event["type"].(string); ok {
		if eventType != pattern.Type {
			return false
		}
	} else {
		return false
	}

	// Check predicates
	for key, expectedValue := range pattern.Predicates {
		actualValue, exists := event[key]
		if !exists {
			return false
		}

		// Handle special predicate operators (e.g., ">10000")
		if strExpected, ok := expectedValue.(string); ok {
			if len(strExpected) > 0 && (strExpected[0] == '>' || strExpected[0] == '<') {
				if !e.evaluatePredicateOperator(actualValue, strExpected) {
					return false
				}
				continue
			}
		}

		if actualValue != expectedValue {
			return false
		}
	}

	return true
}

// evaluatePredicateOperator evaluates special predicate operators like ">10000"
func (e *CEPEngine) evaluatePredicateOperator(value interface{}, predicate string) bool {
	if len(predicate) < 2 {
		return false
	}

	operator := predicate[0]
	operandStr := predicate[1:]

	// Try to parse operand as float
	var operand float64
	fmt.Sscanf(operandStr, "%f", &operand)

	// Convert value to float for comparison
	var floatValue float64
	switch v := value.(type) {
	case float64:
		floatValue = v
	case int:
		floatValue = float64(v)
	case int64:
		floatValue = float64(v)
	default:
		return false
	}

	switch operator {
	case '>':
		return floatValue > operand
	case '<':
		return floatValue < operand
	default:
		return false
	}
}

// checkConditions verifies that conditions between events are satisfied
func (e *CEPEngine) checkConditions(events []map[string]interface{}, conditions []Condition) bool {
	eventMap := make(map[string]map[string]interface{})
	for i, event := range events {
		if name, ok := event["name"].(string); ok {
			eventMap[name] = event
		} else {
			eventMap[fmt.Sprintf("event_%d", i)] = event
		}
	}

	for _, condition := range conditions {
		leftEvent, leftOk := eventMap[condition.LeftEvent]
		rightEvent, rightOk := eventMap[condition.RightEvent]

		if !leftOk || !rightOk {
			return false
		}

		if !evaluateCondition(leftEvent, rightEvent, condition) {
			return false
		}
	}

	return true
}

// createMatchedPattern creates a MatchedPattern from a PartialMatch
func (e *CEPEngine) createMatchedPattern(pattern Pattern, pm *PartialMatch) MatchedPattern {
	return MatchedPattern{
		PatternID:      pattern.ID,
		PatternName:    pattern.Name,
		MatchedEvents:  pm.MatchedEvents,
		FirstEventTime: pm.StartTime,
		LastEventTime:  pm.LastEventTime,
		Severity:       calculateSeverity(pm.MatchedEvents),
		Metadata:       extractMetadata(pm.MatchedEvents),
	}
}

// hasTimedOut checks if a partial match has timed out
func (e *CEPEngine) hasTimedOut(pm *PartialMatch, pattern Pattern) bool {
	if e.timeoutDuration == 0 {
		return false
	}

	elapsed := time.Since(pm.LastEventTime)

	// Use pattern-specific timeout if available, otherwise use engine default
	timeout := e.timeoutDuration
	if pattern.TimeWindow > 0 && pattern.TimeWindow < timeout {
		timeout = pattern.TimeWindow
	}

	return elapsed > timeout
}

// timeoutChecker periodically checks for timed-out partial matches
func (e *CEPEngine) timeoutChecker() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		e.cleanupTimedOutMatches()
	}
}

// cleanupTimedOutMatches removes timed-out partial matches
func (e *CEPEngine) cleanupTimedOutMatches() {
	e.mu.Lock()
	defer e.mu.Unlock()

	for patternID, pattern := range e.patterns {
		activeMatches := make([]*PartialMatch, 0)

		for _, pm := range e.partialMatches[patternID] {
			if !e.hasTimedOut(pm, pattern) {
				activeMatches = append(activeMatches, pm)
			}
		}

		e.partialMatches[patternID] = activeMatches
	}
}

// GetStatistics returns statistics about the CEP engine
func (e *CEPEngine) GetStatistics() map[string]interface{} {
	e.mu.RLock()
	defer e.mu.RUnlock()

	stats := make(map[string]interface{})
	stats["registered_patterns"] = len(e.patterns)
	stats["event_buffer_size"] = len(e.eventBuffer)

	totalPartialMatches := 0
	for _, matches := range e.partialMatches {
		totalPartialMatches += len(matches)
	}
	stats["active_partial_matches"] = totalPartialMatches

	return stats
}

// Helper function for evaluating conditions (used by both engine and matcher)
func evaluateCondition(left, right map[string]interface{}, condition Condition) bool {
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

// Helper functions for severity and metadata (shared)
func calculateSeverity(events []map[string]interface{}) string {
	if len(events) == 0 {
		return "low"
	}

	firstTime := events[0]["timestamp"].(time.Time)
	lastTime := events[len(events)-1]["timestamp"].(time.Time)
	duration := lastTime.Sub(firstTime)

	if duration == 0 {
		return "critical"
	}

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

func extractMetadata(events []map[string]interface{}) map[string]interface{} {
	metadata := make(map[string]interface{})
	metadata["event_count"] = len(events)

	uniqueUsers := make(map[string]bool)
	for _, event := range events {
		if userID, ok := event["user_id"].(string); ok {
			uniqueUsers[userID] = true
		}
	}
	metadata["unique_users"] = len(uniqueUsers)

	uniqueIPs := make(map[string]bool)
	for _, event := range events {
		if ip, ok := event["ip_address"].(string); ok {
			uniqueIPs[ip] = true
		}
	}
	metadata["unique_ips"] = len(uniqueIPs)

	return metadata
}


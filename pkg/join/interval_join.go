package join

import (
	"fmt"
	"time"

	"github.com/therealutkarshpriyadarshi/gress/pkg/stream"
	"go.uber.org/zap"
)

// IntervalJoinOperator joins streams with time interval constraints
// Right events must occur within [left.time + lowerBound, left.time + upperBound]
type IntervalJoinOperator struct {
	*JoinOperator
	lowerBound time.Duration
	upperBound time.Duration
}

// NewIntervalJoinOperator creates an interval join operator
// The bounds define the valid time range for matching right events:
// - lowerBound: minimum time offset (can be negative for past events)
// - upperBound: maximum time offset (typically positive for future events)
func NewIntervalJoinOperator(
	config *JoinConfig,
	lowerBound, upperBound time.Duration,
	logger *zap.Logger,
) *IntervalJoinOperator {
	if config == nil {
		config = DefaultJoinConfig()
	}
	if config.Condition == nil {
		config.Condition = EqualityCondition()
	}

	// Store bounds in config
	config.LowerBound = lowerBound
	config.UpperBound = upperBound

	return &IntervalJoinOperator{
		JoinOperator: NewJoinOperator(config, logger),
		lowerBound:   lowerBound,
		upperBound:   upperBound,
	}
}

// ProcessLeft processes an event from the left stream with interval constraints
func (ij *IntervalJoinOperator) ProcessLeft(ctx *stream.ProcessingContext, event *stream.Event) ([]*stream.Event, error) {
	ij.mu.RLock()
	defer ij.mu.RUnlock()

	// Calculate the valid time range for matching right events
	minTime := event.EventTime.Add(ij.lowerBound)
	maxTime := event.EventTime.Add(ij.upperBound)

	// Calculate expiration based on the upper bound
	expiresAt := maxTime.Add(ij.config.AllowedLateness)

	// Store left event
	ij.state.AddLeftEvent(event, expiresAt)

	// Find matches in right stream within the interval
	var results []*stream.Event
	rightEvents := ij.state.GetRightEvents(event.Key)

	matchFound := false
	for _, rightStored := range rightEvents {
		// Check if right event is within the interval
		if ij.isWithinInterval(event, rightStored.Event, minTime, maxTime) &&
			ij.condition(event, rightStored.Event) {
			matchFound = true

			result := ij.createJoinResult(event, rightStored.Event)
			if result != nil {
				results = append(results, result.ToEvent())
			}

			ij.state.IncrementMatchCount(rightStored)
		}
	}

	// Handle outer join cases
	if !matchFound && (ij.config.Type == LeftOuterJoin || ij.config.Type == FullOuterJoin) {
		// Don't emit immediately - wait until interval expires
		// This will be handled during cleanup
	}

	return results, nil
}

// ProcessRight processes an event from the right stream with interval constraints
func (ij *IntervalJoinOperator) ProcessRight(ctx *stream.ProcessingContext, event *stream.Event) ([]*stream.Event, error) {
	ij.mu.RLock()
	defer ij.mu.RUnlock()

	// For right events, we need to find left events where the right event falls in their interval
	// This means: left.time + lowerBound <= right.time <= left.time + upperBound
	// Rearranging: right.time - upperBound <= left.time <= right.time - lowerBound

	minLeftTime := event.EventTime.Add(-ij.upperBound)
	maxLeftTime := event.EventTime.Add(-ij.lowerBound)

	// Calculate expiration
	expiresAt := event.EventTime.Add(ij.config.StateRetention)

	// Store right event
	ij.state.AddRightEvent(event, expiresAt)

	// Find matches in left stream
	var results []*stream.Event
	leftEvents := ij.state.GetLeftEvents(event.Key)

	matchFound := false
	for _, leftStored := range leftEvents {
		// Check if left event is within the valid range
		if ij.isLeftEventValid(leftStored.Event, event, minLeftTime, maxLeftTime) &&
			ij.condition(leftStored.Event, event) {
			matchFound = true

			result := ij.createJoinResult(leftStored.Event, event)
			if result != nil {
				results = append(results, result.ToEvent())
			}

			ij.state.IncrementMatchCount(leftStored)
		}
	}

	// Handle outer join cases
	if !matchFound && (ij.config.Type == RightOuterJoin || ij.config.Type == FullOuterJoin) {
		// Emit unmatched right event immediately for right outer join
		// since we've already passed the interval where a match could occur
		result := ij.createJoinResult(nil, event)
		if result != nil {
			results = append(results, result.ToEvent())
		}
	}

	return results, nil
}

// isWithinInterval checks if right event is within the interval defined by left event
func (ij *IntervalJoinOperator) isWithinInterval(left, right *stream.Event, minTime, maxTime time.Time) bool {
	rightTime := right.EventTime
	return (rightTime.After(minTime) || rightTime.Equal(minTime)) &&
		(rightTime.Before(maxTime) || rightTime.Equal(maxTime))
}

// isLeftEventValid checks if left event's interval contains the right event
func (ij *IntervalJoinOperator) isLeftEventValid(left, right *stream.Event, minLeftTime, maxLeftTime time.Time) bool {
	leftTime := left.EventTime
	return (leftTime.After(minLeftTime) || leftTime.Equal(minLeftTime)) &&
		(leftTime.Before(maxLeftTime) || leftTime.Equal(maxLeftTime))
}

// Process implements the Operator interface
func (ij *IntervalJoinOperator) Process(ctx *stream.ProcessingContext, event *stream.Event) ([]*stream.Event, error) {
	return nil, fmt.Errorf("interval join operator requires explicit ProcessLeft or ProcessRight calls")
}

// GetIntervalBounds returns the configured interval bounds
func (ij *IntervalJoinOperator) GetIntervalBounds() (lower, upper time.Duration) {
	return ij.lowerBound, ij.upperBound
}

// IntervalJoinBuilder helps construct interval joins with fluent API
type IntervalJoinBuilder struct {
	config     *JoinConfig
	lowerBound time.Duration
	upperBound time.Duration
	logger     *zap.Logger
}

// NewIntervalJoin creates a new interval join builder
func NewIntervalJoin() *IntervalJoinBuilder {
	return &IntervalJoinBuilder{
		config:     DefaultJoinConfig(),
		lowerBound: -5 * time.Minute, // Default: allow 5 minutes in the past
		upperBound: 5 * time.Minute,  // Default: allow 5 minutes in the future
	}
}

// Between sets the time interval bounds
func (b *IntervalJoinBuilder) Between(lower, upper time.Duration) *IntervalJoinBuilder {
	b.lowerBound = lower
	b.upperBound = upper
	return b
}

// WithJoinType sets the join type
func (b *IntervalJoinBuilder) WithJoinType(joinType JoinType) *IntervalJoinBuilder {
	b.config.Type = joinType
	return b
}

// WithCondition sets the join condition
func (b *IntervalJoinBuilder) WithCondition(condition JoinCondition) *IntervalJoinBuilder {
	b.config.Condition = condition
	return b
}

// WithStateRetention sets state retention duration
func (b *IntervalJoinBuilder) WithStateRetention(retention time.Duration) *IntervalJoinBuilder {
	b.config.StateRetention = retention
	return b
}

// WithLogger sets the logger
func (b *IntervalJoinBuilder) WithLogger(logger *zap.Logger) *IntervalJoinBuilder {
	b.logger = logger
	return b
}

// Build creates the interval join operator
func (b *IntervalJoinBuilder) Build() *IntervalJoinOperator {
	if b.logger == nil {
		b.logger = zap.NewNop()
	}
	return NewIntervalJoinOperator(b.config, b.lowerBound, b.upperBound, b.logger)
}

// Example usage patterns:
//
// Join events where right occurs 0-10 seconds after left:
//   NewIntervalJoin().Between(0, 10*time.Second).Build()
//
// Join events where right occurs 5 seconds before to 15 seconds after left:
//   NewIntervalJoin().Between(-5*time.Second, 15*time.Second).Build()
//
// Left outer join with 1 minute interval:
//   NewIntervalJoin().
//     Between(0, time.Minute).
//     WithJoinType(LeftOuterJoin).
//     Build()

package cep

import (
	"fmt"
	"time"
)

// PatternBuilder provides a fluent DSL for building CEP patterns
type PatternBuilder struct {
	pattern Pattern
}

// NewPattern creates a new pattern builder
func NewPattern(id, name string) *PatternBuilder {
	return &PatternBuilder{
		pattern: Pattern{
			ID:         id,
			Name:       name,
			Events:     make([]EventPattern, 0),
			Conditions: make([]Condition, 0),
		},
	}
}

// WithDescription sets the pattern description
func (pb *PatternBuilder) WithDescription(desc string) *PatternBuilder {
	pb.pattern.Description = desc
	return pb
}

// Within sets the time window for the pattern
func (pb *PatternBuilder) Within(duration time.Duration) *PatternBuilder {
	pb.pattern.TimeWindow = duration
	return pb
}

// Sequence adds a sequence of events to the pattern (A followed by B followed by C)
func (pb *PatternBuilder) Sequence(events ...EventPatternBuilder) *PatternBuilder {
	for _, epb := range events {
		pb.pattern.Events = append(pb.pattern.Events, epb.Build())
	}
	return pb
}

// FollowedBy adds an event that must follow the previous event(s)
func (pb *PatternBuilder) FollowedBy(event EventPatternBuilder) *PatternBuilder {
	pb.pattern.Events = append(pb.pattern.Events, event.Build())
	return pb
}

// NotFollowedBy adds a negation pattern (event A should NOT be followed by event B)
func (pb *PatternBuilder) NotFollowedBy(event EventPatternBuilder) *PatternBuilder {
	ep := event.Build()
	ep.Negated = true
	pb.pattern.Events = append(pb.pattern.Events, ep)
	return pb
}

// Where adds a condition between events
func (pb *PatternBuilder) Where(condition Condition) *PatternBuilder {
	pb.pattern.Conditions = append(pb.pattern.Conditions, condition)
	return pb
}

// WithCondition adds a custom condition
func (pb *PatternBuilder) WithCondition(leftEvent, rightEvent, field, operator string) *PatternBuilder {
	pb.pattern.Conditions = append(pb.pattern.Conditions, Condition{
		LeftEvent:  leftEvent,
		RightEvent: rightEvent,
		Field:      field,
		Operator:   operator,
	})
	return pb
}

// Build builds the final pattern
func (pb *PatternBuilder) Build() Pattern {
	return pb.pattern
}

// EventPatternBuilder provides a fluent API for building event patterns
type EventPatternBuilder struct {
	pattern EventPattern
}

// Event creates a new event pattern builder
func Event(name, eventType string) EventPatternBuilder {
	return EventPatternBuilder{
		pattern: EventPattern{
			Name:       name,
			Type:       eventType,
			Predicates: make(map[string]interface{}),
			Quantifier: QuantifierExactlyOne,
		},
	}
}

// With adds a predicate to the event pattern
func (epb EventPatternBuilder) With(key string, value interface{}) EventPatternBuilder {
	epb.pattern.Predicates[key] = value
	return epb
}

// Where adds multiple predicates
func (epb EventPatternBuilder) Where(predicates map[string]interface{}) EventPatternBuilder {
	for k, v := range predicates {
		epb.pattern.Predicates[k] = v
	}
	return epb
}

// Optional marks this event as optional in the pattern
func (epb EventPatternBuilder) Optional() EventPatternBuilder {
	epb.pattern.Optional = true
	return epb
}

// Times specifies that this event should occur exactly N times
func (epb EventPatternBuilder) Times(n int) EventPatternBuilder {
	epb.pattern.Quantifier = QuantifierExactly
	epb.pattern.MinOccurrences = n
	epb.pattern.MaxOccurrences = n
	return epb
}

// AtLeast specifies minimum occurrences
func (epb EventPatternBuilder) AtLeast(n int) EventPatternBuilder {
	epb.pattern.Quantifier = QuantifierAtLeast
	epb.pattern.MinOccurrences = n
	return epb
}

// AtMost specifies maximum occurrences
func (epb EventPatternBuilder) AtMost(n int) EventPatternBuilder {
	epb.pattern.Quantifier = QuantifierAtMost
	epb.pattern.MaxOccurrences = n
	return epb
}

// Between specifies occurrence range
func (epb EventPatternBuilder) Between(min, max int) EventPatternBuilder {
	epb.pattern.Quantifier = QuantifierRange
	epb.pattern.MinOccurrences = min
	epb.pattern.MaxOccurrences = max
	return epb
}

// Within sets a temporal condition for this event relative to the previous event
func (epb EventPatternBuilder) Within(duration time.Duration) EventPatternBuilder {
	epb.pattern.TemporalConstraint = &TemporalConstraint{
		Type:     TemporalWithin,
		Duration: duration,
	}
	return epb
}

// Build builds the event pattern
func (epb EventPatternBuilder) Build() EventPattern {
	return epb.pattern
}

// Quantifier types for event occurrence patterns
type Quantifier int

const (
	QuantifierExactlyOne Quantifier = iota // Default: exactly one occurrence
	QuantifierExactly                       // Exactly N occurrences
	QuantifierAtLeast                       // At least N occurrences
	QuantifierAtMost                        // At most N occurrences
	QuantifierRange                         // Between min and max occurrences
	QuantifierOneOrMore                     // One or more occurrences (N+)
	QuantifierZeroOrMore                    // Zero or more occurrences (N*)
)

// TemporalConstraintType defines temporal constraint types
type TemporalConstraintType int

const (
	TemporalWithin TemporalConstraintType = iota // Must occur within duration
	TemporalAfter                                 // Must occur after duration
	TemporalBefore                                // Must occur before duration
)

// TemporalConstraint represents a temporal condition
type TemporalConstraint struct {
	Type     TemporalConstraintType `json:"type"`
	Duration time.Duration          `json:"duration"`
}

// Enhanced EventPattern with new fields
// Note: This extends the base EventPattern in pattern.go

// ConditionBuilder provides a fluent API for building conditions
type ConditionBuilder struct {
	condition Condition
}

// NewCondition creates a new condition builder
func NewCondition(leftEvent, rightEvent string) *ConditionBuilder {
	return &ConditionBuilder{
		condition: Condition{
			LeftEvent:  leftEvent,
			RightEvent: rightEvent,
		},
	}
}

// Field sets the field to compare
func (cb *ConditionBuilder) Field(field string) *ConditionBuilder {
	cb.condition.Field = field
	return cb
}

// Equals sets equality operator
func (cb *ConditionBuilder) Equals() *ConditionBuilder {
	cb.condition.Operator = "eq"
	return cb
}

// NotEquals sets inequality operator
func (cb *ConditionBuilder) NotEquals() *ConditionBuilder {
	cb.condition.Operator = "ne"
	return cb
}

// GreaterThan sets greater than operator
func (cb *ConditionBuilder) GreaterThan() *ConditionBuilder {
	cb.condition.Operator = "gt"
	return cb
}

// LessThan sets less than operator
func (cb *ConditionBuilder) LessThan() *ConditionBuilder {
	cb.condition.Operator = "lt"
	return cb
}

// GreaterThanOrEqual sets greater than or equal operator
func (cb *ConditionBuilder) GreaterThanOrEqual() *ConditionBuilder {
	cb.condition.Operator = "gte"
	return cb
}

// LessThanOrEqual sets less than or equal operator
func (cb *ConditionBuilder) LessThanOrEqual() *ConditionBuilder {
	cb.condition.Operator = "lte"
	return cb
}

// Build builds the condition
func (cb *ConditionBuilder) Build() Condition {
	return cb.condition
}

// Example DSL usage patterns

// FraudDetectionPattern creates a fraud detection pattern using the DSL
func FraudDetectionPattern() Pattern {
	return NewPattern("fraud-001", "Multiple Failed Logins").
		WithDescription("Detect 3 or more failed login attempts followed by a successful login within 5 minutes").
		Sequence(
			Event("failed_login", "login_failed").
				With("status", "failed").
				Times(3),
			Event("successful_login", "login_success").
				With("status", "success").
				Within(1 * time.Minute),
		).
		WithCondition("failed_login", "successful_login", "user_id", "eq").
		Within(5 * time.Minute).
		Build()
}

// SuspiciousActivityPattern creates a pattern for detecting suspicious user activity
func SuspiciousActivityPattern() Pattern {
	return NewPattern("suspicious-001", "Unusual Access Pattern").
		WithDescription("Login from new location followed by high-value transaction without 2FA").
		Sequence(
			Event("login", "user_login").
				With("new_location", true),
			Event("transaction", "transaction_initiated").
				With("amount_threshold", ">10000").
				Within(10 * time.Minute),
		).
		NotFollowedBy(
			Event("2fa", "two_factor_auth").
				With("verified", true),
		).
		Within(30 * time.Minute).
		Build()
}

// UserJourneyPattern creates a pattern for tracking user journey
func UserJourneyPattern() Pattern {
	return NewPattern("journey-001", "Checkout Abandonment").
		WithDescription("User adds items to cart but doesn't complete checkout").
		Sequence(
			Event("browse", "product_view").
				AtLeast(2),
			Event("add_cart", "add_to_cart").
				AtLeast(1),
			Event("checkout_start", "checkout_initiated").
				Optional(),
		).
		NotFollowedBy(
			Event("purchase", "purchase_completed"),
		).
		Within(1 * time.Hour).
		Build()
}

// IterationPattern creates a pattern for detecting repeated events
func IterationPattern(eventType string, count int, window time.Duration) Pattern {
	return NewPattern(
		fmt.Sprintf("iteration-%s", eventType),
		fmt.Sprintf("%s occurred %d times", eventType, count),
	).
		WithDescription(fmt.Sprintf("Detect %s happening %d times within %s", eventType, count, window)).
		Sequence(
			Event("event", eventType).Times(count),
		).
		Within(window).
		Build()
}

// SequenceWithNegation creates a pattern with negation (A followed by NOT B, then C)
func SequenceWithNegation(a, notB, c string, window time.Duration) Pattern {
	return NewPattern(
		"sequence-with-negation",
		fmt.Sprintf("%s without %s then %s", a, notB, c),
	).
		Sequence(
			Event("a", a),
		).
		NotFollowedBy(
			Event("notB", notB),
		).
		FollowedBy(
			Event("c", c),
		).
		Within(window).
		Build()
}

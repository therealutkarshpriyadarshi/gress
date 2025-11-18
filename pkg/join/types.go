package join

import (
	"time"

	"github.com/therealutkarshpriyadarshi/gress/pkg/stream"
)

// JoinType defines the type of join operation
type JoinType int

const (
	// InnerJoin emits only matching events from both streams
	InnerJoin JoinType = iota
	// LeftOuterJoin emits all left events, with or without matches
	LeftOuterJoin
	// RightOuterJoin emits all right events, with or without matches
	RightOuterJoin
	// FullOuterJoin emits all events from both streams
	FullOuterJoin
)

// String returns string representation of join type
func (j JoinType) String() string {
	switch j {
	case InnerJoin:
		return "INNER"
	case LeftOuterJoin:
		return "LEFT_OUTER"
	case RightOuterJoin:
		return "RIGHT_OUTER"
	case FullOuterJoin:
		return "FULL_OUTER"
	default:
		return "UNKNOWN"
	}
}

// JoinStrategy defines the algorithm used for joining
type JoinStrategy int

const (
	// HashJoin uses hash tables for joining (best for equality joins)
	HashJoin JoinStrategy = iota
	// SortMergeJoin sorts and merges streams (good for range joins)
	SortMergeJoin
	// NestedLoopJoin compares all pairs (fallback for complex predicates)
	NestedLoopJoin
)

// String returns string representation of join strategy
func (j JoinStrategy) String() string {
	switch j {
	case HashJoin:
		return "HASH"
	case SortMergeJoin:
		return "SORT_MERGE"
	case NestedLoopJoin:
		return "NESTED_LOOP"
	default:
		return "UNKNOWN"
	}
}

// JoinCondition defines how events are matched
type JoinCondition func(left, right *stream.Event) bool

// JoinResult represents a joined event pair
type JoinResult struct {
	Left      *stream.Event
	Right     *stream.Event
	EventTime time.Time
	JoinKey   string
}

// ToEvent converts join result to a stream event
func (jr *JoinResult) ToEvent() *stream.Event {
	value := map[string]interface{}{}

	if jr.Left != nil {
		value["left"] = jr.Left.Value
	}
	if jr.Right != nil {
		value["right"] = jr.Right.Value
	}

	headers := make(map[string]string)
	if jr.Left != nil {
		headers["left_key"] = jr.Left.Key
		headers["left_time"] = jr.Left.EventTime.Format(time.RFC3339)
	}
	if jr.Right != nil {
		headers["right_key"] = jr.Right.Key
		headers["right_time"] = jr.Right.EventTime.Format(time.RFC3339)
	}

	return &stream.Event{
		Key:       jr.JoinKey,
		Value:     value,
		EventTime: jr.EventTime,
		Headers:   headers,
	}
}

// JoinConfig configures join behavior
type JoinConfig struct {
	// Type of join (inner, left, right, full)
	Type JoinType

	// Strategy for performing the join
	Strategy JoinStrategy

	// Condition for matching events
	Condition JoinCondition

	// TimeWindow defines the time range for joining events
	// Events are joined if their timestamps are within this window
	TimeWindow time.Duration

	// LowerBound and UpperBound define interval join bounds
	// Right event must be in [left.time + LowerBound, left.time + UpperBound]
	LowerBound time.Duration
	UpperBound time.Duration

	// AllowedLateness is how long to wait for late events
	AllowedLateness time.Duration

	// StateRetention is how long to keep state for unmatched events
	StateRetention time.Duration

	// MaxStateSize limits the number of events kept in state (per key)
	MaxStateSize int

	// EnableCleanup enables periodic state cleanup
	EnableCleanup bool

	// CleanupInterval defines how often to clean up expired state
	CleanupInterval time.Duration
}

// DefaultJoinConfig returns a default join configuration
func DefaultJoinConfig() *JoinConfig {
	return &JoinConfig{
		Type:            InnerJoin,
		Strategy:        HashJoin,
		TimeWindow:      5 * time.Minute,
		AllowedLateness: 30 * time.Second,
		StateRetention:  10 * time.Minute,
		MaxStateSize:    10000,
		EnableCleanup:   true,
		CleanupInterval: 1 * time.Minute,
	}
}

// WithJoinType sets the join type
func (c *JoinConfig) WithJoinType(joinType JoinType) *JoinConfig {
	c.Type = joinType
	return c
}

// WithStrategy sets the join strategy
func (c *JoinConfig) WithStrategy(strategy JoinStrategy) *JoinConfig {
	c.Strategy = strategy
	return c
}

// WithCondition sets the join condition
func (c *JoinConfig) WithCondition(condition JoinCondition) *JoinConfig {
	c.Condition = condition
	return c
}

// WithTimeWindow sets the time window
func (c *JoinConfig) WithTimeWindow(window time.Duration) *JoinConfig {
	c.TimeWindow = window
	return c
}

// WithIntervalBounds sets interval join bounds
func (c *JoinConfig) WithIntervalBounds(lower, upper time.Duration) *JoinConfig {
	c.LowerBound = lower
	c.UpperBound = upper
	return c
}

// WithStateRetention sets state retention duration
func (c *JoinConfig) WithStateRetention(retention time.Duration) *JoinConfig {
	c.StateRetention = retention
	return c
}

// WithMaxStateSize sets maximum state size
func (c *JoinConfig) WithMaxStateSize(size int) *JoinConfig {
	c.MaxStateSize = size
	return c
}

// KeyByFunc extracts a join key from an event
type KeyByFunc func(*stream.Event) string

// EqualityCondition creates a condition that checks key equality
func EqualityCondition() JoinCondition {
	return func(left, right *stream.Event) bool {
		return left.Key == right.Key
	}
}

// CustomCondition creates a condition from a predicate
func CustomCondition(predicate func(left, right *stream.Event) bool) JoinCondition {
	return predicate
}

// Side represents which stream (left or right)
type Side int

const (
	LeftSide Side = iota
	RightSide
)

// StoredEvent wraps an event with metadata for state management
type StoredEvent struct {
	Event      *stream.Event
	Side       Side
	StoredAt   time.Time
	ExpiresAt  time.Time
	MatchCount int
}

// IsExpired checks if the stored event has expired
func (se *StoredEvent) IsExpired(now time.Time) bool {
	return now.After(se.ExpiresAt)
}

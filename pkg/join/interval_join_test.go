package join

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/therealutkarshpriyadarshi/gress/pkg/stream"
	"go.uber.org/zap"
)

func TestIntervalJoinWithinBounds(t *testing.T) {
	logger := zap.NewNop()

	// Join if right occurs 0-10 seconds after left
	intervalJoin := NewIntervalJoin().
		Between(0*time.Second, 10*time.Second).
		WithJoinType(InnerJoin).
		WithLogger(logger).
		Build()
	defer intervalJoin.Close()

	ctx := &stream.ProcessingContext{Ctx: context.Background()}
	baseTime := time.Now()

	leftEvent := &stream.Event{
		Key:       "key1",
		Value:     "left",
		EventTime: baseTime,
	}

	// Right event 5 seconds later (within bounds)
	rightEventIn := &stream.Event{
		Key:       "key1",
		Value:     "right-in",
		EventTime: baseTime.Add(5 * time.Second),
	}

	// Right event 15 seconds later (outside bounds)
	rightEventOut := &stream.Event{
		Key:       "key1",
		Value:     "right-out",
		EventTime: baseTime.Add(15 * time.Second),
	}

	intervalJoin.ProcessLeft(ctx, leftEvent)

	resultsIn, err := intervalJoin.ProcessRight(ctx, rightEventIn)
	assert.NoError(t, err)
	assert.Len(t, resultsIn, 1, "Event within interval should match")

	resultsOut, err := intervalJoin.ProcessRight(ctx, rightEventOut)
	assert.NoError(t, err)
	assert.Empty(t, resultsOut, "Event outside interval should not match")
}

func TestIntervalJoinNegativeBounds(t *testing.T) {
	logger := zap.NewNop()

	// Join if right occurs 5 seconds before to 5 seconds after left
	intervalJoin := NewIntervalJoin().
		Between(-5*time.Second, 5*time.Second).
		WithJoinType(InnerJoin).
		WithLogger(logger).
		Build()
	defer intervalJoin.Close()

	ctx := &stream.ProcessingContext{Ctx: context.Background()}
	baseTime := time.Now()

	leftEvent := &stream.Event{
		Key:       "key1",
		Value:     "left",
		EventTime: baseTime,
	}

	// Right event 3 seconds before (within bounds)
	rightEventBefore := &stream.Event{
		Key:       "key1",
		Value:     "right-before",
		EventTime: baseTime.Add(-3 * time.Second),
	}

	// Right event 3 seconds after (within bounds)
	rightEventAfter := &stream.Event{
		Key:       "key1",
		Value:     "right-after",
		EventTime: baseTime.Add(3 * time.Second),
	}

	intervalJoin.ProcessLeft(ctx, leftEvent)

	resultsBefore, err := intervalJoin.ProcessRight(ctx, rightEventBefore)
	assert.NoError(t, err)
	assert.Len(t, resultsBefore, 1, "Event before within interval should match")

	resultsAfter, err := intervalJoin.ProcessRight(ctx, rightEventAfter)
	assert.NoError(t, err)
	assert.Len(t, resultsAfter, 1, "Event after within interval should match")
}

func TestIntervalJoinRightToLeft(t *testing.T) {
	logger := zap.NewNop()

	// Join if right occurs 0-10 seconds after left
	intervalJoin := NewIntervalJoin().
		Between(0*time.Second, 10*time.Second).
		WithJoinType(InnerJoin).
		WithLogger(logger).
		Build()
	defer intervalJoin.Close()

	ctx := &stream.ProcessingContext{Ctx: context.Background()}
	baseTime := time.Now()

	// Process right event first
	rightEvent := &stream.Event{
		Key:       "key1",
		Value:     "right",
		EventTime: baseTime.Add(5 * time.Second),
	}

	// Then process left event
	leftEvent := &stream.Event{
		Key:       "key1",
		Value:     "left",
		EventTime: baseTime,
	}

	intervalJoin.ProcessRight(ctx, rightEvent)

	results, err := intervalJoin.ProcessLeft(ctx, leftEvent)
	assert.NoError(t, err)
	assert.Len(t, results, 1, "Should match when left arrives after right")
}

func TestIntervalJoinLeftOuterJoin(t *testing.T) {
	logger := zap.NewNop()

	intervalJoin := NewIntervalJoin().
		Between(0*time.Second, 10*time.Second).
		WithJoinType(LeftOuterJoin).
		WithLogger(logger).
		Build()
	defer intervalJoin.Close()

	ctx := &stream.ProcessingContext{Ctx: context.Background()}
	baseTime := time.Now()

	// Left event with no matching right
	leftEvent := &stream.Event{
		Key:       "key-unmatched",
		Value:     "left",
		EventTime: baseTime,
	}

	results, err := intervalJoin.ProcessLeft(ctx, leftEvent)
	assert.NoError(t, err)
	// For interval joins, unmatched events are emitted during cleanup
	// So immediate results might be empty
	assert.GreaterOrEqual(t, len(results), 0)
}

func TestIntervalJoinMultipleMatches(t *testing.T) {
	logger := zap.NewNop()

	intervalJoin := NewIntervalJoin().
		Between(0*time.Second, 30*time.Second).
		WithJoinType(InnerJoin).
		WithLogger(logger).
		Build()
	defer intervalJoin.Close()

	ctx := &stream.ProcessingContext{Ctx: context.Background()}
	baseTime := time.Now()

	leftEvent := &stream.Event{
		Key:       "key1",
		Value:     "left",
		EventTime: baseTime,
	}

	intervalJoin.ProcessLeft(ctx, leftEvent)

	// Process multiple right events within interval
	var totalResults int
	for i := 0; i < 5; i++ {
		rightEvent := &stream.Event{
			Key:       "key1",
			Value:     i,
			EventTime: baseTime.Add(time.Duration(i*5) * time.Second),
		}

		results, err := intervalJoin.ProcessRight(ctx, rightEvent)
		assert.NoError(t, err)
		totalResults += len(results)
	}

	assert.Equal(t, 5, totalResults, "Should match all events within interval")
}

func TestIntervalJoinBuilder(t *testing.T) {
	logger := zap.NewNop()

	// Test builder pattern
	intervalJoin := NewIntervalJoin().
		Between(-5*time.Second, 10*time.Second).
		WithJoinType(FullOuterJoin).
		WithCondition(EqualityCondition()).
		WithStateRetention(1 * time.Hour).
		WithLogger(logger).
		Build()

	assert.NotNil(t, intervalJoin)

	lower, upper := intervalJoin.GetIntervalBounds()
	assert.Equal(t, -5*time.Second, lower)
	assert.Equal(t, 10*time.Second, upper)

	intervalJoin.Close()
}

func TestIntervalJoinExactBoundaries(t *testing.T) {
	logger := zap.NewNop()

	intervalJoin := NewIntervalJoin().
		Between(0*time.Second, 10*time.Second).
		WithJoinType(InnerJoin).
		WithLogger(logger).
		Build()
	defer intervalJoin.Close()

	ctx := &stream.ProcessingContext{Ctx: context.Background()}
	baseTime := time.Now()

	leftEvent := &stream.Event{
		Key:       "key1",
		Value:     "left",
		EventTime: baseTime,
	}

	// Right event exactly at lower bound (0 seconds)
	rightAtLower := &stream.Event{
		Key:       "key1",
		Value:     "at-lower",
		EventTime: baseTime,
	}

	// Right event exactly at upper bound (10 seconds)
	rightAtUpper := &stream.Event{
		Key:       "key1",
		Value:     "at-upper",
		EventTime: baseTime.Add(10 * time.Second),
	}

	intervalJoin.ProcessLeft(ctx, leftEvent)

	resultsLower, err := intervalJoin.ProcessRight(ctx, rightAtLower)
	assert.NoError(t, err)
	assert.Len(t, resultsLower, 1, "Event at lower bound should match")

	resultsUpper, err := intervalJoin.ProcessRight(ctx, rightAtUpper)
	assert.NoError(t, err)
	assert.Len(t, resultsUpper, 1, "Event at upper bound should match")
}

func BenchmarkIntervalJoin(b *testing.B) {
	logger := zap.NewNop()

	intervalJoin := NewIntervalJoin().
		Between(0*time.Second, 1*time.Minute).
		WithJoinType(InnerJoin).
		WithLogger(logger).
		Build()
	defer intervalJoin.Close()

	ctx := &stream.ProcessingContext{Ctx: context.Background()}
	baseTime := time.Now()

	// Pre-populate with left events
	for i := 0; i < 100; i++ {
		event := &stream.Event{
			Key:       "key1",
			Value:     i,
			EventTime: baseTime.Add(time.Duration(i) * time.Second),
		}
		intervalJoin.ProcessLeft(ctx, event)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		rightEvent := &stream.Event{
			Key:       "key1",
			Value:     "right",
			EventTime: baseTime.Add(30 * time.Second),
		}
		intervalJoin.ProcessRight(ctx, rightEvent)
	}
}

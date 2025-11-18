package join

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/therealutkarshpriyadarshi/gress/pkg/stream"
	"go.uber.org/zap"
)

func TestInnerJoin(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultJoinConfig().WithJoinType(InnerJoin)
	joinOp := NewJoinOperator(config, logger)
	defer joinOp.Close()

	ctx := &stream.ProcessingContext{Ctx: context.Background()}
	now := time.Now()

	// Create matching events
	leftEvent := &stream.Event{
		Key:       "key1",
		Value:     "left-value",
		EventTime: now,
	}

	rightEvent := &stream.Event{
		Key:       "key1",
		Value:     "right-value",
		EventTime: now,
	}

	// Process left, then right
	resultsLeft, err := joinOp.ProcessLeft(ctx, leftEvent)
	assert.NoError(t, err)
	assert.Empty(t, resultsLeft) // No match yet

	resultsRight, err := joinOp.ProcessRight(ctx, rightEvent)
	assert.NoError(t, err)
	assert.Len(t, resultsRight, 1) // Should have match

	// Verify result
	result := resultsRight[0].Value.(map[string]interface{})
	assert.Equal(t, "left-value", result["left"])
	assert.Equal(t, "right-value", result["right"])

	// Check metrics
	metrics := joinOp.GetMetrics()
	assert.Equal(t, int64(1), metrics.LeftEventsReceived)
	assert.Equal(t, int64(1), metrics.RightEventsReceived)
	assert.Equal(t, int64(1), metrics.MatchesFound)
}

func TestLeftOuterJoin(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultJoinConfig().WithJoinType(LeftOuterJoin)
	joinOp := NewJoinOperator(config, logger)
	defer joinOp.Close()

	ctx := &stream.ProcessingContext{Ctx: context.Background()}
	now := time.Now()

	// Create left event without matching right
	leftEvent := &stream.Event{
		Key:       "key1",
		Value:     "left-value",
		EventTime: now,
	}

	results, err := joinOp.ProcessLeft(ctx, leftEvent)
	assert.NoError(t, err)
	assert.Len(t, results, 1) // Should emit with null right

	// Verify result has left but no right
	result := results[0].Value.(map[string]interface{})
	assert.Equal(t, "left-value", result["left"])
	assert.Nil(t, result["right"])
}

func TestRightOuterJoin(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultJoinConfig().WithJoinType(RightOuterJoin)
	joinOp := NewJoinOperator(config, logger)
	defer joinOp.Close()

	ctx := &stream.ProcessingContext{Ctx: context.Background()}
	now := time.Now()

	// Create right event without matching left
	rightEvent := &stream.Event{
		Key:       "key1",
		Value:     "right-value",
		EventTime: now,
	}

	results, err := joinOp.ProcessRight(ctx, rightEvent)
	assert.NoError(t, err)
	assert.Len(t, results, 1) // Should emit with null left

	// Verify result has right but no left
	result := results[0].Value.(map[string]interface{})
	assert.Nil(t, result["left"])
	assert.Equal(t, "right-value", result["right"])
}

func TestFullOuterJoin(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultJoinConfig().WithJoinType(FullOuterJoin)
	joinOp := NewJoinOperator(config, logger)
	defer joinOp.Close()

	ctx := &stream.ProcessingContext{Ctx: context.Background()}
	now := time.Now()

	// Test unmatched left
	leftEvent := &stream.Event{
		Key:       "key1",
		Value:     "left-value",
		EventTime: now,
	}

	resultsLeft, err := joinOp.ProcessLeft(ctx, leftEvent)
	assert.NoError(t, err)
	assert.Len(t, resultsLeft, 1)

	// Test unmatched right
	rightEvent := &stream.Event{
		Key:       "key2",
		Value:     "right-value",
		EventTime: now,
	}

	resultsRight, err := joinOp.ProcessRight(ctx, rightEvent)
	assert.NoError(t, err)
	assert.Len(t, resultsRight, 1)
}

func TestJoinWithTimeWindow(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultJoinConfig().
		WithJoinType(InnerJoin).
		WithTimeWindow(1 * time.Minute)

	joinOp := NewJoinOperator(config, logger)
	defer joinOp.Close()

	ctx := &stream.ProcessingContext{Ctx: context.Background()}
	now := time.Now()

	leftEvent := &stream.Event{
		Key:       "key1",
		Value:     "left",
		EventTime: now,
	}

	// Right event within window
	rightEventInWindow := &stream.Event{
		Key:       "key1",
		Value:     "right-in-window",
		EventTime: now.Add(30 * time.Second),
	}

	// Right event outside window
	rightEventOutWindow := &stream.Event{
		Key:       "key1",
		Value:     "right-out-window",
		EventTime: now.Add(2 * time.Minute),
	}

	joinOp.ProcessLeft(ctx, leftEvent)

	// In window - should match
	resultsIn, err := joinOp.ProcessRight(ctx, rightEventInWindow)
	assert.NoError(t, err)
	assert.Len(t, resultsIn, 1)

	// Out of window - should not match
	resultsOut, err := joinOp.ProcessRight(ctx, rightEventOutWindow)
	assert.NoError(t, err)
	assert.Empty(t, resultsOut)
}

func TestCustomJoinCondition(t *testing.T) {
	logger := zap.NewNop()

	// Custom condition: only join if values contain "match"
	customCondition := CustomCondition(func(left, right *stream.Event) bool {
		if left.Key != right.Key {
			return false
		}
		leftStr, lok := left.Value.(string)
		rightStr, rok := right.Value.(string)
		if !lok || !rok {
			return false
		}
		return leftStr == "match" && rightStr == "match"
	})

	config := DefaultJoinConfig().
		WithJoinType(InnerJoin).
		WithCondition(customCondition)

	joinOp := NewJoinOperator(config, logger)
	defer joinOp.Close()

	ctx := &stream.ProcessingContext{Ctx: context.Background()}
	now := time.Now()

	leftEvent := &stream.Event{
		Key:       "key1",
		Value:     "match",
		EventTime: now,
	}

	// Should match
	rightMatch := &stream.Event{
		Key:       "key1",
		Value:     "match",
		EventTime: now,
	}

	// Should not match
	rightNoMatch := &stream.Event{
		Key:       "key1",
		Value:     "no-match",
		EventTime: now,
	}

	joinOp.ProcessLeft(ctx, leftEvent)

	resultsMatch, _ := joinOp.ProcessRight(ctx, rightMatch)
	assert.Len(t, resultsMatch, 1)

	resultsNoMatch, _ := joinOp.ProcessRight(ctx, rightNoMatch)
	assert.Empty(t, resultsNoMatch)
}

func TestHashJoinStrategy(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultJoinConfig().
		WithStrategy(HashJoin).
		WithJoinType(InnerJoin)

	joinOp := NewJoinOperator(config, logger)
	defer joinOp.Close()

	ctx := &stream.ProcessingContext{Ctx: context.Background()}
	now := time.Now()

	// Add multiple events with same key
	for i := 0; i < 5; i++ {
		leftEvent := &stream.Event{
			Key:       "key1",
			Value:     i,
			EventTime: now,
		}
		joinOp.ProcessLeft(ctx, leftEvent)
	}

	// Right event should match all 5 left events
	rightEvent := &stream.Event{
		Key:       "key1",
		Value:     "right",
		EventTime: now,
	}

	results, err := joinOp.ProcessRight(ctx, rightEvent)
	assert.NoError(t, err)
	assert.Len(t, results, 5)
}

func TestNestedLoopJoin(t *testing.T) {
	logger := zap.NewNop()

	// Custom condition that doesn't require key equality
	condition := CustomCondition(func(left, right *stream.Event) bool {
		return true // Match everything
	})

	config := DefaultJoinConfig().
		WithStrategy(NestedLoopJoin).
		WithJoinType(InnerJoin).
		WithCondition(condition)

	joinOp := NewJoinOperator(config, logger)
	defer joinOp.Close()

	ctx := &stream.ProcessingContext{Ctx: context.Background()}
	now := time.Now()

	// Add left events with different keys
	left1 := &stream.Event{Key: "key1", Value: "left1", EventTime: now}
	left2 := &stream.Event{Key: "key2", Value: "left2", EventTime: now}

	joinOp.ProcessLeft(ctx, left1)
	joinOp.ProcessLeft(ctx, left2)

	// Right event with different key should still match due to custom condition
	rightEvent := &stream.Event{
		Key:       "key3",
		Value:     "right",
		EventTime: now,
	}

	results, err := joinOp.ProcessRight(ctx, rightEvent)
	assert.NoError(t, err)
	assert.Len(t, results, 2) // Should match both left events
}

func TestJoinStateManagement(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultJoinConfig().
		WithMaxStateSize(2). // Limit to 2 events per key
		WithJoinType(InnerJoin)

	joinOp := NewJoinOperator(config, logger)
	defer joinOp.Close()

	ctx := &stream.ProcessingContext{Ctx: context.Background()}
	now := time.Now()

	// Add 3 left events (should keep only last 2)
	for i := 0; i < 3; i++ {
		event := &stream.Event{
			Key:       "key1",
			Value:     i,
			EventTime: now.Add(time.Duration(i) * time.Second),
		}
		joinOp.ProcessLeft(ctx, event)
	}

	// Check state size
	stateSize := joinOp.state.Size()
	assert.Equal(t, 2, stateSize) // Should have only 2 events
}

func TestDualStreamJoin(t *testing.T) {
	logger := zap.NewNop()

	leftInput := make(chan *stream.Event, 10)
	rightInput := make(chan *stream.Event, 10)

	config := DefaultJoinConfig().WithJoinType(InnerJoin)
	dualJoin := NewDualStreamJoin(leftInput, rightInput, config, logger)

	dualJoin.Start()

	now := time.Now()

	// Send matching events
	leftInput <- &stream.Event{
		Key:       "key1",
		Value:     "left",
		EventTime: now,
	}

	rightInput <- &stream.Event{
		Key:       "key1",
		Value:     "right",
		EventTime: now,
	}

	// Close inputs
	close(leftInput)
	close(rightInput)

	// Collect results
	var results []*stream.Event
	done := make(chan struct{})

	go func() {
		for result := range dualJoin.Output() {
			results = append(results, result)
		}
		close(done)
	}()

	// Wait a bit for processing
	time.Sleep(100 * time.Millisecond)

	dualJoin.Stop()
	<-done

	assert.NotEmpty(t, results)
}

func TestJoinMetrics(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultJoinConfig().WithJoinType(InnerJoin)
	joinOp := NewJoinOperator(config, logger)
	defer joinOp.Close()

	ctx := &stream.ProcessingContext{Ctx: context.Background()}
	now := time.Now()

	// Add events
	left := &stream.Event{Key: "key1", Value: "left", EventTime: now}
	right := &stream.Event{Key: "key1", Value: "right", EventTime: now}

	joinOp.ProcessLeft(ctx, left)
	joinOp.ProcessRight(ctx, right)

	metrics := joinOp.GetMetrics()
	assert.Equal(t, int64(1), metrics.LeftEventsReceived)
	assert.Equal(t, int64(1), metrics.RightEventsReceived)
	assert.Equal(t, int64(1), metrics.MatchesFound)
	assert.Equal(t, int64(2), metrics.StateSize)
}

func BenchmarkHashJoin(b *testing.B) {
	logger := zap.NewNop()
	config := DefaultJoinConfig().
		WithStrategy(HashJoin).
		WithJoinType(InnerJoin)

	joinOp := NewJoinOperator(config, logger)
	defer joinOp.Close()

	ctx := &stream.ProcessingContext{Ctx: context.Background()}
	now := time.Now()

	// Pre-populate left side
	for i := 0; i < 1000; i++ {
		event := &stream.Event{
			Key:       "key1",
			Value:     i,
			EventTime: now,
		}
		joinOp.ProcessLeft(ctx, event)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		rightEvent := &stream.Event{
			Key:       "key1",
			Value:     "right",
			EventTime: now,
		}
		joinOp.ProcessRight(ctx, rightEvent)
	}
}

func BenchmarkNestedLoopJoin(b *testing.B) {
	logger := zap.NewNop()
	config := DefaultJoinConfig().
		WithStrategy(NestedLoopJoin).
		WithJoinType(InnerJoin)

	joinOp := NewJoinOperator(config, logger)
	defer joinOp.Close()

	ctx := &stream.ProcessingContext{Ctx: context.Background()}
	now := time.Now()

	// Pre-populate left side
	for i := 0; i < 100; i++ {
		event := &stream.Event{
			Key:       "key1",
			Value:     i,
			EventTime: now,
		}
		joinOp.ProcessLeft(ctx, event)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		rightEvent := &stream.Event{
			Key:       "key1",
			Value:     "right",
			EventTime: now,
		}
		joinOp.ProcessRight(ctx, rightEvent)
	}
}

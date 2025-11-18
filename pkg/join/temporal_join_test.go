package join

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/therealutkarshpriyadarshi/gress/pkg/stream"
	"go.uber.org/zap"
)

func TestTemporalJoinBasic(t *testing.T) {
	logger := zap.NewNop()

	temporalJoin := NewTemporalJoin().
		WithLogger(logger).
		Build()
	defer temporalJoin.Close()

	ctx := &stream.ProcessingContext{Ctx: context.Background()}
	baseTime := time.Now()

	// Add reference data
	refEvent := &stream.Event{
		Key:       "product-1",
		Value:     map[string]interface{}{"price": 10.0},
		EventTime: baseTime,
	}

	err := temporalJoin.UpdateReference(refEvent)
	assert.NoError(t, err)

	// Process main stream event
	mainEvent := &stream.Event{
		Key:       "product-1",
		Value:     map[string]interface{}{"order_id": "order-1"},
		EventTime: baseTime.Add(1 * time.Minute),
	}

	results, err := temporalJoin.ProcessLeft(ctx, mainEvent)
	assert.NoError(t, err)
	assert.Len(t, results, 1)

	// Verify join result
	joined := results[0].Value.(map[string]interface{})
	assert.NotNil(t, joined["left"])
	assert.NotNil(t, joined["right"])
}

func TestTemporalJoinVersioning(t *testing.T) {
	logger := zap.NewNop()

	temporalJoin := NewTemporalJoin().
		WithLogger(logger).
		Build()
	defer temporalJoin.Close()

	ctx := &stream.ProcessingContext{Ctx: context.Background()}
	baseTime := time.Now()

	// Add first version
	v1 := &stream.Event{
		Key:       "product-1",
		Value:     map[string]interface{}{"price": 10.0, "version": 1},
		EventTime: baseTime,
	}

	// Add second version
	v2 := &stream.Event{
		Key:       "product-1",
		Value:     map[string]interface{}{"price": 15.0, "version": 2},
		EventTime: baseTime.Add(1 * time.Hour),
	}

	temporalJoin.UpdateReference(v1)
	temporalJoin.UpdateReference(v2)

	// Query at time before v2 - should get v1
	event1 := &stream.Event{
		Key:       "product-1",
		Value:     "query1",
		EventTime: baseTime.Add(30 * time.Minute),
	}

	results1, err := temporalJoin.ProcessLeft(ctx, event1)
	assert.NoError(t, err)
	assert.Len(t, results1, 1)

	joined1 := results1[0].Value.(map[string]interface{})
	rightData1 := joined1["right"].(map[string]interface{})
	assert.Equal(t, 10.0, rightData1["price"])

	// Query at time after v2 - should get v2
	event2 := &stream.Event{
		Key:       "product-1",
		Value:     "query2",
		EventTime: baseTime.Add(2 * time.Hour),
	}

	results2, err := temporalJoin.ProcessLeft(ctx, event2)
	assert.NoError(t, err)
	assert.Len(t, results2, 1)

	joined2 := results2[0].Value.(map[string]interface{})
	rightData2 := joined2["right"].(map[string]interface{})
	assert.Equal(t, 15.0, rightData2["price"])
}

func TestTemporalJoinNoMatch(t *testing.T) {
	logger := zap.NewNop()

	temporalJoin := NewTemporalJoin().
		WithJoinType(LeftOuterJoin).
		WithLogger(logger).
		Build()
	defer temporalJoin.Close()

	ctx := &stream.ProcessingContext{Ctx: context.Background()}
	baseTime := time.Now()

	// Query for non-existent key
	event := &stream.Event{
		Key:       "non-existent",
		Value:     "query",
		EventTime: baseTime,
	}

	results, err := temporalJoin.ProcessLeft(ctx, event)
	assert.NoError(t, err)
	assert.Len(t, results, 1) // Left outer join should still emit

	joined := results[0].Value.(map[string]interface{})
	assert.NotNil(t, joined["left"])
	assert.Nil(t, joined["right"])
}

func TestTemporalJoinMultipleKeys(t *testing.T) {
	logger := zap.NewNop()

	temporalJoin := NewTemporalJoin().
		WithLogger(logger).
		Build()
	defer temporalJoin.Close()

	ctx := &stream.ProcessingContext{Ctx: context.Background()}
	baseTime := time.Now()

	// Add reference data for multiple keys
	products := []string{"product-1", "product-2", "product-3"}
	for i, key := range products {
		refEvent := &stream.Event{
			Key:       key,
			Value:     map[string]interface{}{"price": float64((i + 1) * 10)},
			EventTime: baseTime,
		}
		temporalJoin.UpdateReference(refEvent)
	}

	// Query each product
	for i, key := range products {
		event := &stream.Event{
			Key:       key,
			Value:     map[string]interface{}{"query": i},
			EventTime: baseTime.Add(1 * time.Minute),
		}

		results, err := temporalJoin.ProcessLeft(ctx, event)
		assert.NoError(t, err)
		assert.Len(t, results, 1)
	}

	// Check metrics
	metrics := temporalJoin.GetMetrics()
	assert.Equal(t, 3, metrics.TotalKeys)
	assert.Equal(t, 3, metrics.TotalVersions)
}

func TestTemporalJoinGetVersionHistory(t *testing.T) {
	logger := zap.NewNop()

	temporalJoin := NewTemporalJoin().
		WithLogger(logger).
		Build()
	defer temporalJoin.Close()

	baseTime := time.Now()

	// Add multiple versions
	for i := 0; i < 3; i++ {
		refEvent := &stream.Event{
			Key:       "product-1",
			Value:     map[string]interface{}{"version": i + 1},
			EventTime: baseTime.Add(time.Duration(i) * time.Hour),
		}
		temporalJoin.UpdateReference(refEvent)
	}

	// Get version history
	history, err := temporalJoin.GetVersionHistory("product-1")
	assert.NoError(t, err)
	assert.NotNil(t, history)
	assert.Equal(t, 3, len(history.Versions))

	// Verify versions are ordered
	for i := 0; i < 3; i++ {
		version := history.Versions[i]
		versionData := version.Event.Value.(map[string]interface{})
		assert.Equal(t, i+1, versionData["version"])
	}
}

func TestTemporalJoinGetAllKeys(t *testing.T) {
	logger := zap.NewNop()

	temporalJoin := NewTemporalJoin().
		WithLogger(logger).
		Build()
	defer temporalJoin.Close()

	baseTime := time.Now()

	// Add reference data for multiple keys
	keys := []string{"key1", "key2", "key3"}
	for _, key := range keys {
		refEvent := &stream.Event{
			Key:       key,
			Value:     "data",
			EventTime: baseTime,
		}
		temporalJoin.UpdateReference(refEvent)
	}

	// Get all keys
	allKeys := temporalJoin.GetAllKeys()
	assert.Len(t, allKeys, 3)

	// Verify all keys are present
	keySet := make(map[string]bool)
	for _, key := range allKeys {
		keySet[key] = true
	}
	for _, key := range keys {
		assert.True(t, keySet[key], "Key %s should be present", key)
	}
}

func TestTemporalJoinLookupBeforeFirstVersion(t *testing.T) {
	logger := zap.NewNop()

	temporalJoin := NewTemporalJoin().
		WithLogger(logger).
		Build()
	defer temporalJoin.Close()

	ctx := &stream.ProcessingContext{Ctx: context.Background()}
	baseTime := time.Now()

	// Add reference data
	refEvent := &stream.Event{
		Key:       "product-1",
		Value:     "data",
		EventTime: baseTime.Add(1 * time.Hour),
	}
	temporalJoin.UpdateReference(refEvent)

	// Query before first version
	event := &stream.Event{
		Key:       "product-1",
		Value:     "query",
		EventTime: baseTime, // Before reference data
	}

	results, err := temporalJoin.ProcessLeft(ctx, event)
	assert.NoError(t, err)
	// Should have no match since query is before first version
	assert.Empty(t, results)
}

func TestTemporalJoinExactTimestamp(t *testing.T) {
	logger := zap.NewNop()

	temporalJoin := NewTemporalJoin().
		WithLogger(logger).
		Build()
	defer temporalJoin.Close()

	ctx := &stream.ProcessingContext{Ctx: context.Background()}
	baseTime := time.Now()

	// Add reference data
	refEvent := &stream.Event{
		Key:       "product-1",
		Value:     "data",
		EventTime: baseTime,
	}
	temporalJoin.UpdateReference(refEvent)

	// Query at exact timestamp
	event := &stream.Event{
		Key:       "product-1",
		Value:     "query",
		EventTime: baseTime,
	}

	results, err := temporalJoin.ProcessLeft(ctx, event)
	assert.NoError(t, err)
	assert.Len(t, results, 1)
}

func TestTemporalJoinBuilder(t *testing.T) {
	logger := zap.NewNop()

	// Test builder pattern
	temporalJoin := NewTemporalJoin().
		WithJoinType(InnerJoin).
		WithCondition(EqualityCondition()).
		WithStateRetention(24 * time.Hour).
		WithLogger(logger).
		Build()

	assert.NotNil(t, temporalJoin)
	temporalJoin.Close()
}

func BenchmarkTemporalJoinLookup(b *testing.B) {
	logger := zap.NewNop()

	temporalJoin := NewTemporalJoin().
		WithLogger(logger).
		Build()
	defer temporalJoin.Close()

	ctx := &stream.ProcessingContext{Ctx: context.Background()}
	baseTime := time.Now()

	// Add many versions
	for i := 0; i < 100; i++ {
		refEvent := &stream.Event{
			Key:       "product-1",
			Value:     i,
			EventTime: baseTime.Add(time.Duration(i) * time.Minute),
		}
		temporalJoin.UpdateReference(refEvent)
	}

	b.ResetTimer()

	// Benchmark lookups
	for i := 0; i < b.N; i++ {
		event := &stream.Event{
			Key:       "product-1",
			Value:     "query",
			EventTime: baseTime.Add(50 * time.Minute),
		}
		temporalJoin.ProcessLeft(ctx, event)
	}
}

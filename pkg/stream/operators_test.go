package stream

import (
	"testing"
	"time"
)

func TestMapOperator(t *testing.T) {
	transform := func(e *Event) (*Event, error) {
		if val, ok := e.Value.(int); ok {
			e.Value = val * 2
		}
		return e, nil
	}

	op := NewMapOperator(transform)
	ctx := &ProcessingContext{Timestamp: time.Now()}

	event := &Event{Value: 5}
	results, err := op.Process(ctx, event)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}

	if results[0].Value != 10 {
		t.Errorf("Expected value 10, got %v", results[0].Value)
	}
}

func TestFilterOperator(t *testing.T) {
	predicate := func(e *Event) bool {
		if val, ok := e.Value.(int); ok {
			return val > 5
		}
		return false
	}

	op := NewFilterOperator(predicate)
	ctx := &ProcessingContext{Timestamp: time.Now()}

	// Test passing event
	event1 := &Event{Value: 10}
	results1, err := op.Process(ctx, event1)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if len(results1) != 1 {
		t.Errorf("Expected event to pass filter")
	}

	// Test filtered event
	event2 := &Event{Value: 3}
	results2, err := op.Process(ctx, event2)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if len(results2) != 0 {
		t.Errorf("Expected event to be filtered")
	}
}

func TestFlatMapOperator(t *testing.T) {
	transform := func(e *Event) ([]*Event, error) {
		if val, ok := e.Value.(int); ok {
			var results []*Event
			for i := 0; i < val; i++ {
				results = append(results, &Event{Value: i})
			}
			return results, nil
		}
		return []*Event{e}, nil
	}

	op := NewFlatMapOperator(transform)
	ctx := &ProcessingContext{Timestamp: time.Now()}

	event := &Event{Value: 3}
	results, err := op.Process(ctx, event)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(results) != 3 {
		t.Fatalf("Expected 3 results, got %d", len(results))
	}

	for i, result := range results {
		if result.Value != i {
			t.Errorf("Expected value %d, got %v", i, result.Value)
		}
	}
}

func TestAggregateOperator(t *testing.T) {
	aggregateFunc := func(acc interface{}, e *Event) (interface{}, error) {
		sum := 0
		if acc != nil {
			sum = acc.(int)
		}
		if val, ok := e.Value.(int); ok {
			sum += val
		}
		return sum, nil
	}

	op := NewAggregateOperator(aggregateFunc)
	ctx := &ProcessingContext{Timestamp: time.Now()}

	events := []*Event{
		{Key: "test", Value: 1},
		{Key: "test", Value: 2},
		{Key: "test", Value: 3},
	}

	var lastResult *Event
	for _, event := range events {
		results, err := op.Process(ctx, event)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if len(results) > 0 {
			lastResult = results[0]
		}
	}

	if lastResult == nil {
		t.Fatal("Expected result")
	}

	if lastResult.Value != 6 {
		t.Errorf("Expected sum 6, got %v", lastResult.Value)
	}
}

func TestKeyByOperator(t *testing.T) {
	keySelector := func(e *Event) string {
		if val, ok := e.Value.(map[string]interface{}); ok {
			if id, ok := val["id"].(string); ok {
				return id
			}
		}
		return "default"
	}

	op := NewKeyByOperator(keySelector)
	ctx := &ProcessingContext{Timestamp: time.Now()}

	event := &Event{
		Value: map[string]interface{}{
			"id":   "user-123",
			"data": "test",
		},
	}

	results, err := op.Process(ctx, event)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}

	if results[0].Key != "user-123" {
		t.Errorf("Expected key 'user-123', got '%s'", results[0].Key)
	}
}

func BenchmarkMapOperator(b *testing.B) {
	transform := func(e *Event) (*Event, error) {
		if val, ok := e.Value.(int); ok {
			e.Value = val * 2
		}
		return e, nil
	}

	op := NewMapOperator(transform)
	ctx := &ProcessingContext{Timestamp: time.Now()}
	event := &Event{Value: 5}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		op.Process(ctx, event)
	}
}

func BenchmarkFilterOperator(b *testing.B) {
	predicate := func(e *Event) bool {
		if val, ok := e.Value.(int); ok {
			return val > 5
		}
		return false
	}

	op := NewFilterOperator(predicate)
	ctx := &ProcessingContext{Timestamp: time.Now()}
	event := &Event{Value: 10}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		op.Process(ctx, event)
	}
}

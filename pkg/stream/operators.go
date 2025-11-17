package stream

import (
	"fmt"
)

// MapOperator applies a transformation function to each event
type MapOperator struct {
	transform TransformFunc
}

// NewMapOperator creates a new map operator
func NewMapOperator(transform TransformFunc) *MapOperator {
	return &MapOperator{transform: transform}
}

// Process applies the map transformation
func (m *MapOperator) Process(ctx *ProcessingContext, event *Event) ([]*Event, error) {
	result, err := m.transform(event)
	if err != nil {
		return nil, fmt.Errorf("map transform error: %w", err)
	}
	if result == nil {
		return []*Event{}, nil
	}
	return []*Event{result}, nil
}

// Close cleans up resources
func (m *MapOperator) Close() error {
	return nil
}

// FilterOperator filters events based on a predicate
type FilterOperator struct {
	predicate FilterFunc
}

// NewFilterOperator creates a new filter operator
func NewFilterOperator(predicate FilterFunc) *FilterOperator {
	return &FilterOperator{predicate: predicate}
}

// Process applies the filter predicate
func (f *FilterOperator) Process(ctx *ProcessingContext, event *Event) ([]*Event, error) {
	if f.predicate(event) {
		return []*Event{event}, nil
	}
	return []*Event{}, nil
}

// Close cleans up resources
func (f *FilterOperator) Close() error {
	return nil
}

// FlatMapOperator transforms one event into zero or more events
type FlatMapOperator struct {
	transform FlatMapFunc
}

// NewFlatMapOperator creates a new flatmap operator
func NewFlatMapOperator(transform FlatMapFunc) *FlatMapOperator {
	return &FlatMapOperator{transform: transform}
}

// Process applies the flatmap transformation
func (f *FlatMapOperator) Process(ctx *ProcessingContext, event *Event) ([]*Event, error) {
	results, err := f.transform(event)
	if err != nil {
		return nil, fmt.Errorf("flatmap transform error: %w", err)
	}
	return results, nil
}

// Close cleans up resources
func (f *FlatMapOperator) Close() error {
	return nil
}

// KeyByOperator partitions events by a key selector
type KeyByOperator struct {
	selector KeySelector
}

// NewKeyByOperator creates a new key-by operator
func NewKeyByOperator(selector KeySelector) *KeyByOperator {
	return &KeyByOperator{selector: selector}
}

// Process assigns a key to the event
func (k *KeyByOperator) Process(ctx *ProcessingContext, event *Event) ([]*Event, error) {
	event.Key = k.selector(event)
	return []*Event{event}, nil
}

// Close cleans up resources
func (k *KeyByOperator) Close() error {
	return nil
}

// AggregateOperator performs stateful aggregation
type AggregateOperator struct {
	aggregateFunc AggregateFunc
	state         map[string]interface{} // key -> accumulator
}

// NewAggregateOperator creates a new aggregate operator
func NewAggregateOperator(aggregateFunc AggregateFunc) *AggregateOperator {
	return &AggregateOperator{
		aggregateFunc: aggregateFunc,
		state:         make(map[string]interface{}),
	}
}

// Process applies the aggregation function
func (a *AggregateOperator) Process(ctx *ProcessingContext, event *Event) ([]*Event, error) {
	key := event.Key
	if key == "" {
		key = "default"
	}

	accumulator := a.state[key]
	newAccumulator, err := a.aggregateFunc(accumulator, event)
	if err != nil {
		return nil, fmt.Errorf("aggregate error: %w", err)
	}

	a.state[key] = newAccumulator

	// Create output event with aggregated value
	result := &Event{
		Key:       key,
		Value:     newAccumulator,
		EventTime: event.EventTime,
		Headers:   event.Headers,
	}

	return []*Event{result}, nil
}

// Close cleans up resources
func (a *AggregateOperator) Close() error {
	a.state = nil
	return nil
}

// GetState returns the current aggregation state
func (a *AggregateOperator) GetState() (map[string]interface{}, error) {
	stateCopy := make(map[string]interface{})
	for k, v := range a.state {
		stateCopy[k] = v
	}
	return stateCopy, nil
}

// RestoreState restores aggregation state
func (a *AggregateOperator) RestoreState(state map[string]interface{}) error {
	a.state = make(map[string]interface{})
	for k, v := range state {
		a.state[k] = v
	}
	return nil
}

// ReduceOperator combines events with the same key
type ReduceOperator struct {
	reduceFunc ReduceFunc
	state      map[string]interface{}
}

// NewReduceOperator creates a new reduce operator
func NewReduceOperator(reduceFunc ReduceFunc) *ReduceOperator {
	return &ReduceOperator{
		reduceFunc: reduceFunc,
		state:      make(map[string]interface{}),
	}
}

// Process applies the reduce function
func (r *ReduceOperator) Process(ctx *ProcessingContext, event *Event) ([]*Event, error) {
	key := event.Key
	if key == "" {
		key = "default"
	}

	currentValue := r.state[key]
	if currentValue == nil {
		r.state[key] = event.Value
		return []*Event{event}, nil
	}

	newValue, err := r.reduceFunc(currentValue, event.Value)
	if err != nil {
		return nil, fmt.Errorf("reduce error: %w", err)
	}

	r.state[key] = newValue

	result := &Event{
		Key:       key,
		Value:     newValue,
		EventTime: event.EventTime,
		Headers:   event.Headers,
	}

	return []*Event{result}, nil
}

// Close cleans up resources
func (r *ReduceOperator) Close() error {
	r.state = nil
	return nil
}

// BranchOperator splits a stream into multiple streams based on predicates
type BranchOperator struct {
	branches map[string]FilterFunc
	outputs  map[string]chan *Event
}

// NewBranchOperator creates a new branch operator
func NewBranchOperator(branches map[string]FilterFunc) *BranchOperator {
	outputs := make(map[string]chan *Event)
	for name := range branches {
		outputs[name] = make(chan *Event, 100)
	}

	return &BranchOperator{
		branches: branches,
		outputs:  outputs,
	}
}

// Process routes events to appropriate branches
func (b *BranchOperator) Process(ctx *ProcessingContext, event *Event) ([]*Event, error) {
	for name, predicate := range b.branches {
		if predicate(event) {
			select {
			case b.outputs[name] <- event:
			default:
				// Channel full, skip
			}
		}
	}
	return []*Event{event}, nil
}

// GetOutputStream returns the output channel for a branch
func (b *BranchOperator) GetOutputStream(name string) <-chan *Event {
	return b.outputs[name]
}

// Close cleans up resources
func (b *BranchOperator) Close() error {
	for _, ch := range b.outputs {
		close(ch)
	}
	return nil
}

// UnionOperator merges multiple streams into one
type UnionOperator struct {
	inputs []<-chan *Event
}

// NewUnionOperator creates a new union operator
func NewUnionOperator(inputs []<-chan *Event) *UnionOperator {
	return &UnionOperator{inputs: inputs}
}

// Process is not used for union (handled differently)
func (u *UnionOperator) Process(ctx *ProcessingContext, event *Event) ([]*Event, error) {
	return []*Event{event}, nil
}

// Close cleans up resources
func (u *UnionOperator) Close() error {
	return nil
}

// TimestampAssignerOperator assigns event-time timestamps
type TimestampAssignerOperator struct {
	extractor TimestampExtractor
}

// NewTimestampAssignerOperator creates a new timestamp assigner
func NewTimestampAssignerOperator(extractor TimestampExtractor) *TimestampAssignerOperator {
	return &TimestampAssignerOperator{extractor: extractor}
}

// Process assigns event time
func (t *TimestampAssignerOperator) Process(ctx *ProcessingContext, event *Event) ([]*Event, error) {
	event.EventTime = t.extractor(event)
	return []*Event{event}, nil
}

// Close cleans up resources
func (t *TimestampAssignerOperator) Close() error {
	return nil
}

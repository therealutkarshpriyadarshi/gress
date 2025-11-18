# Stream Joins

Stream joins allow you to combine events from multiple data streams based on keys, time windows, and custom conditions. Gress provides comprehensive join capabilities for real-time stream processing.

## Table of Contents

- [Overview](#overview)
- [Join Types](#join-types)
- [Join Strategies](#join-strategies)
- [Join Variants](#join-variants)
- [Usage Examples](#usage-examples)
- [Performance Considerations](#performance-considerations)
- [Best Practices](#best-practices)

## Overview

Stream joins combine events from two streams (left and right) based on:
- **Keys**: Events are typically joined on matching keys
- **Time**: Events are joined within time windows or intervals
- **Conditions**: Custom predicates determine which events match

### Basic Concepts

```go
import "github.com/therealutkarshpriyadarshi/gress/pkg/join"

// Create a join configuration
config := join.DefaultJoinConfig().
    WithJoinType(join.InnerJoin).
    WithTimeWindow(1 * time.Minute)

// Create join operator
joinOp := join.NewJoinOperator(config, logger)
```

## Join Types

### Inner Join

Emits only events that have matches in both streams.

```go
config := join.DefaultJoinConfig().
    WithJoinType(join.InnerJoin)

joinOp := join.NewJoinOperator(config, logger)

// Process events from both streams
resultsLeft, _ := joinOp.ProcessLeft(ctx, leftEvent)
resultsRight, _ := joinOp.ProcessRight(ctx, rightEvent)
```

**Use Case**: Join orders with customers - only emit when both order and customer exist.

### Left Outer Join

Emits all events from the left stream, with or without matches from the right.

```go
config := join.DefaultJoinConfig().
    WithJoinType(join.LeftOuterJoin)
```

**Use Case**: Process all orders, enriching with customer data when available.

### Right Outer Join

Emits all events from the right stream, with or without matches from the left.

```go
config := join.DefaultJoinConfig().
    WithJoinType(join.RightOuterJoin)
```

**Use Case**: Process all customer updates, including orders when available.

### Full Outer Join

Emits all events from both streams, matched or unmatched.

```go
config := join.DefaultJoinConfig().
    WithJoinType(join.FullOuterJoin)
```

**Use Case**: Reconciliation scenarios where you need all events from both sides.

## Join Strategies

### Hash Join (Default)

Uses hash tables for fast lookups. Best for equality-based joins.

```go
config := join.DefaultJoinConfig().
    WithStrategy(join.HashJoin)
```

**Characteristics**:
- O(1) lookup time
- Best for key-based joins
- Memory efficient for moderate state sizes

### Sort-Merge Join

Sorts and merges streams. Good for range joins and sorted data.

```go
config := join.DefaultJoinConfig().
    WithStrategy(join.SortMergeJoin)
```

**Characteristics**:
- Efficient for sorted streams
- Good for range predicates
- Requires buffering

### Nested Loop Join

Compares all event pairs. Supports complex predicates.

```go
config := join.DefaultJoinConfig().
    WithStrategy(join.NestedLoopJoin)
```

**Characteristics**:
- Supports any join condition
- O(n*m) complexity
- Use only for small state or complex predicates

## Join Variants

### 1. Regular Join

Basic join with time window constraints.

```go
config := join.DefaultJoinConfig().
    WithJoinType(join.InnerJoin).
    WithTimeWindow(5 * time.Minute)

joinOp := join.NewJoinOperator(config, logger)

// Process events
joinOp.ProcessLeft(ctx, leftEvent)
joinOp.ProcessRight(ctx, rightEvent)
```

### 2. Window-Based Join

Join events within specific time windows (tumbling, sliding, or session).

```go
// Create window assigner
assigner := window.NewTumblingWindow(10 * time.Second)

// Create window join
config := join.DefaultJoinConfig().WithJoinType(join.InnerJoin)
windowJoin := join.NewWindowJoinOperator(config, assigner, logger)

// Process events
windowJoin.ProcessLeft(ctx, leftEvent)
windowJoin.ProcessRight(ctx, rightEvent)

// Trigger window on watermark
results := windowJoin.OnWatermark(watermarkTime)
```

**Use Cases**:
- Session analytics (clicks and purchases in same session)
- Fraud detection (multiple events in time window)
- Real-time dashboards (aggregated metrics per window)

### 3. Interval Join

Join events where right event occurs within a time interval relative to left event.

```go
// Join if right occurs 0-30 seconds after left
intervalJoin := join.NewIntervalJoin().
    Between(0*time.Second, 30*time.Second).
    WithJoinType(join.InnerJoin).
    Build()

// Use negative bounds for past events
// Join if right occurs 5 seconds before to 10 seconds after left
intervalJoin := join.NewIntervalJoin().
    Between(-5*time.Second, 10*time.Second).
    Build()
```

**Use Cases**:
- User behavior tracking (action within X seconds of login)
- IoT sensor correlation (related events with time tolerance)
- Alert correlation (related alerts within time bounds)

### 4. Temporal Join

Join stream with versioned reference data, using the version valid at event time.

```go
temporalJoin := join.NewTemporalJoin().
    WithStateRetention(24 * time.Hour).
    Build()

// Update reference data (right side)
temporalJoin.UpdateReference(productCatalogEvent)

// Process main stream (left side) - uses version valid at event time
results, _ := temporalJoin.ProcessLeft(ctx, orderEvent)
```

**Use Cases**:
- Price lookups (use price valid at order time)
- Configuration joins (use config active at event time)
- Slowly changing dimensions (CDC with temporal semantics)

## Usage Examples

### Example 1: Join Orders with Customers

```go
// Configure join
config := join.DefaultJoinConfig().
    WithJoinType(join.InnerJoin).
    WithTimeWindow(1 * time.Minute)

joinOp := join.NewJoinOperator(config, logger)

// Customer event (left stream)
customer := &stream.Event{
    Key:       "customer-123",
    Value:     map[string]interface{}{"name": "Alice", "tier": "gold"},
    EventTime: time.Now(),
}

// Order event (right stream)
order := &stream.Event{
    Key:       "customer-123",
    Value:     map[string]interface{}{"order_id": "order-456", "amount": 100.0},
    EventTime: time.Now(),
}

// Process events
joinOp.ProcessLeft(ctx, customer)
results, _ := joinOp.ProcessRight(ctx, order)

// Result contains joined data
for _, result := range results {
    joined := result.Value.(map[string]interface{})
    customerData := joined["left"]   // customer info
    orderData := joined["right"]     // order info
}
```

### Example 2: Windowed Click-to-Purchase Join

```go
// 10-second tumbling windows
assigner := window.NewTumblingWindow(10 * time.Second)
config := join.DefaultJoinConfig().WithJoinType(join.InnerJoin)
windowJoin := join.NewWindowJoinOperator(config, assigner, logger)

// Click event
click := &stream.Event{
    Key:       "user-1",
    Value:     map[string]interface{}{"page": "/product/123"},
    EventTime: baseTime,
}

// Purchase event (5 seconds later, same window)
purchase := &stream.Event{
    Key:       "user-1",
    Value:     map[string]interface{}{"product_id": "123", "amount": 50.0},
    EventTime: baseTime.Add(5 * time.Second),
}

windowJoin.ProcessLeft(ctx, click)
windowJoin.ProcessRight(ctx, purchase)

// Fire window when watermark passes
watermark := baseTime.Add(11 * time.Second)
results := windowJoin.OnWatermark(watermark)
```

### Example 3: Temporal Product Catalog Join

```go
temporalJoin := join.NewTemporalJoin().
    WithStateRetention(24 * time.Hour).
    Build()

// Update product catalog (version 1)
product1 := &stream.Event{
    Key:       "product-123",
    Value:     map[string]interface{}{"name": "Widget", "price": 10.0},
    EventTime: time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC),
}
temporalJoin.UpdateReference(product1)

// Update product catalog (version 2 - price change)
product2 := &stream.Event{
    Key:       "product-123",
    Value:     map[string]interface{}{"name": "Widget", "price": 12.0},
    EventTime: time.Date(2024, 1, 1, 14, 0, 0, 0, time.UTC),
}
temporalJoin.UpdateReference(product2)

// Order at 12:00 - gets price 10.0
order1 := &stream.Event{
    Key:       "product-123",
    Value:     map[string]interface{}{"order_id": "order-1", "quantity": 5},
    EventTime: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
}
results1, _ := temporalJoin.ProcessLeft(ctx, order1)

// Order at 16:00 - gets price 12.0
order2 := &stream.Event{
    Key:       "product-123",
    Value:     map[string]interface{}{"order_id": "order-2", "quantity": 3},
    EventTime: time.Date(2024, 1, 1, 16, 0, 0, 0, time.UTC),
}
results2, _ := temporalJoin.ProcessLeft(ctx, order2)
```

### Example 4: Custom Condition Join

```go
// Join only high-value orders
customCondition := join.CustomCondition(func(left, right *stream.Event) bool {
    if left.Key != right.Key {
        return false
    }

    orderData := right.Value.(map[string]interface{})
    if amount, ok := orderData["amount"].(float64); ok {
        return amount > 100.0  // Only join orders > $100
    }
    return false
})

config := join.DefaultJoinConfig().
    WithJoinType(join.InnerJoin).
    WithCondition(customCondition)

joinOp := join.NewJoinOperator(config, logger)
```

### Example 5: Dual Stream Join

```go
// Create input channels
leftInput := make(chan *stream.Event, 100)
rightInput := make(chan *stream.Event, 100)

// Configure join
config := join.DefaultJoinConfig().
    WithJoinType(join.InnerJoin).
    WithTimeWindow(1 * time.Minute)

// Create dual stream join
dualJoin := join.NewDualStreamJoin(leftInput, rightInput, config, logger)
dualJoin.Start()

// Send events to input channels
leftInput <- leftEvent
rightInput <- rightEvent

// Consume results
for result := range dualJoin.Output() {
    processJoinedEvent(result)
}

// Cleanup
close(leftInput)
close(rightInput)
dualJoin.Stop()
```

## Performance Considerations

### State Management

```go
config := join.DefaultJoinConfig().
    WithMaxStateSize(10000).           // Limit events per key
    WithStateRetention(10 * time.Minute). // Expire old events
    WithCleanupInterval(1 * time.Minute)  // Cleanup frequency
```

**Recommendations**:
- Set `MaxStateSize` based on expected cardinality
- Use `StateRetention` to prevent unbounded state growth
- Enable cleanup for long-running joins

### Memory Usage

| Join Type | Memory Complexity | Notes |
|-----------|------------------|-------|
| Hash Join | O(n) | n = number of stored events |
| Sort-Merge Join | O(n + m) | Requires buffering both sides |
| Nested Loop | O(n + m) | May scan entire state |
| Window Join | O(w) | w = events in active windows |
| Interval Join | O(r) | r = events in interval range |
| Temporal Join | O(v) | v = total versions |

### Throughput Optimization

1. **Use Hash Join** for key-based joins (fastest)
2. **Limit state size** to prevent memory pressure
3. **Enable cleanup** to remove expired events
4. **Choose appropriate time windows** - smaller windows = less state

### Latency Optimization

1. **Regular Join**: Immediate results when matches found
2. **Window Join**: Results on window close (watermark)
3. **Interval Join**: Immediate for inner join, delayed for outer
4. **Temporal Join**: Immediate lookups (O(log n) with binary search)

## Best Practices

### 1. Choose the Right Join Type

- **Inner Join**: When you only care about matched events
- **Left/Right Outer**: When one side is optional
- **Full Outer**: For reconciliation and completeness

### 2. Configure Time Windows

```go
// Too small: may miss matches
config.WithTimeWindow(1 * time.Second)

// Reasonable: matches most use cases
config.WithTimeWindow(1 * time.Minute)

// Too large: high memory usage
config.WithTimeWindow(1 * time.Hour)
```

### 3. Handle Late Events

```go
config := join.DefaultJoinConfig().
    WithAllowedLateness(30 * time.Second). // Grace period
    WithStateRetention(10 * time.Minute)   // Keep state longer
```

### 4. Monitor Join Metrics

```go
metrics := joinOp.GetMetrics()
fmt.Printf("Matches: %d\n", metrics.MatchesFound)
fmt.Printf("State size: %d\n", metrics.StateSize)
fmt.Printf("Left unmatched: %d\n", metrics.LeftUnmatched)
fmt.Printf("Right unmatched: %d\n", metrics.RightUnmatched)
```

### 5. Key Selection

- Use consistent keys across streams
- Avoid high-cardinality keys (millions of unique keys)
- Consider key transformation for normalization

### 6. Testing

```go
// Test with concurrent events
// Test with out-of-order events
// Test with late arrivals
// Test state cleanup
// Test different time windows
```

## Common Patterns

### Pattern 1: Stream Enrichment

Join a fact stream with dimension data.

```go
// Use temporal join for slowly changing dimensions
temporalJoin := join.NewTemporalJoin().Build()

// Update dimension data
temporalJoin.UpdateReference(customerEvent)

// Enrich transactions
results, _ := temporalJoin.ProcessLeft(ctx, transactionEvent)
```

### Pattern 2: Correlation

Correlate related events within a time window.

```go
// Use interval join for event correlation
intervalJoin := join.NewIntervalJoin().
    Between(0, 30*time.Second).
    Build()
```

### Pattern 3: Session Analysis

Join events within user sessions.

```go
// Use session windows
assigner := window.NewSessionWindow(30 * time.Second)
windowJoin := join.NewWindowJoinOperator(config, assigner, logger)
```

## Error Handling

```go
results, err := joinOp.ProcessLeft(ctx, event)
if err != nil {
    logger.Error("Join processing error", zap.Error(err))
    // Handle error (retry, skip, alert)
}

// Check for context cancellation
if ctx.Ctx.Err() != nil {
    // Context cancelled
    return ctx.Ctx.Err()
}
```

## Advanced Topics

### Multi-Way Joins

Chain multiple join operators:

```go
// Join stream A and B
join1 := join.NewJoinOperator(config1, logger)

// Join result with stream C
join2 := join.NewJoinOperator(config2, logger)

results1, _ := join1.ProcessLeft(ctx, eventA)
results2, _ := join1.ProcessRight(ctx, eventB)

for _, result := range results2 {
    join2.ProcessLeft(ctx, result)
}
join2.ProcessRight(ctx, eventC)
```

### Stateful Joins with Checkpointing

For fault tolerance, serialize join state:

```go
// Get state snapshot
stateSnapshot := joinOp.state.GetMetrics()

// Store to checkpoint
checkpoint.SaveJoinState(stateSnapshot)

// Restore on recovery
// (Implementation depends on state backend)
```

## See Also

- [Window Operations](./WINDOWS.md)
- [Watermarks](./WATERMARKS.md)
- [State Management](./STATE.md)
- [Examples](../examples/join_example.go)

# CEP Examples

This directory contains comprehensive examples demonstrating the Complex Event Processing (CEP) capabilities of Gress.

## Examples

### 1. Fraud Detection (`fraud_detection.go`)

Demonstrates various fraud detection patterns using CEP:

**Patterns Implemented:**
- **Credential Stuffing Attack**: Detects 3+ failed login attempts followed by a successful login
- **Suspicious High-Value Transaction**: Login from new location + high-value transaction without 2FA
- **Automated Account Creation**: 5+ accounts created from same IP within 10 minutes
- **Transaction Velocity Fraud**: Multiple high-value transactions in rapid succession
- **Account Takeover**: Password change → email change → withdrawal sequence

**Run:**
```bash
go run examples/cep/fraud_detection.go
```

**Sample Output:**
```
=== Fraud Detection CEP Example ===

Scenario 1: Credential Stuffing Attack
  ⚠️  Failed login attempt 1 for user user123
  ⚠️  Failed login attempt 2 for user user123
  ⚠️  Failed login attempt 3 for user user123
  ✅ Successful login for user user123

🚨 ALERT: Credential Stuffing Attack detected!
   Pattern ID: fraud-001
   Severity: high
   Time Range: 15:23:10 to 15:23:40
   Events Matched: 4
```

### 2. User Journey Analysis (`user_journey.go`)

Demonstrates user behavior and conversion funnel analysis:

**Patterns Implemented:**
- **Successful Conversion**: Complete purchase journey (browse → cart → checkout → purchase)
- **Shopping Cart Abandonment**: Items added to cart but no checkout
- **Window Shopping**: Extensive browsing without adding to cart
- **Product Comparison**: Viewing multiple products before deciding
- **Re-engagement Success**: User returns after abandonment and completes purchase
- **Checkout Abandonment**: Starts checkout but doesn't complete
- **Price Sensitive Behavior**: Multiple views without purchase over time

**Run:**
```bash
go run examples/cep/user_journey.go
```

**Sample Output:**
```
=== User Journey Analysis CEP Example ===

Scenario 1: Successful E-commerce Conversion
  ✓ product_view
  ✓ product_view
  ✓ add_to_cart
  ✓ checkout_initiated
  ✓ purchase_completed

📊 Journey Pattern Detected: Successful Conversion
   Pattern ID: journey-001
   Duration: 10m0s
   Events in Journey: 5

   Journey Steps:
   1. product_view at 15:30:00
      - Product: laptop-001
      - Category: electronics
   2. product_view at 15:32:00
      - Product: laptop-002
   ...
```

## Key Concepts Demonstrated

### 1. Sequence Detection
Both examples show how to detect events in a specific order:
```go
Sequence(
    Event("a", "event_a"),
    Event("b", "event_b"),
    Event("c", "event_c"),
)
```

### 2. Temporal Constraints
Events must occur within specified time windows:
```go
Event("followup", "follow_up").Within(5 * time.Minute)
```

### 3. Negation Patterns
Detect when events do NOT occur:
```go
NotFollowedBy(Event("unwanted", "unwanted_event"))
```

### 4. Iteration Patterns
Detect repeated occurrences:
```go
Event("repeated", "login_failed").Times(3)
Event("multiple", "page_view").AtLeast(5)
```

### 5. Event Conditions
Compare fields between events:
```go
WithCondition("event1", "event2", "user_id", "eq")
```

### 6. Pattern Callbacks
React to detected patterns:
```go
engine.OnMatch(func(match cep.MatchedPattern) {
    fmt.Printf("Pattern detected: %s\n", match.PatternName)
})
```

## Building and Running

### Run Individual Examples

```bash
# Fraud detection
go run examples/cep/fraud_detection.go

# User journey
go run examples/cep/user_journey.go
```

### Run All Examples

```bash
cd examples/cep
for f in *.go; do
    echo "Running $f..."
    go run "$f"
    echo ""
done
```

## Understanding the Output

### Alert Format

When a pattern is detected, you'll see:

```
🚨 ALERT: Pattern Name detected!
   Pattern ID: pattern-id
   Severity: high
   Time Range: 15:23:10 to 15:23:40
   Events Matched: 4
   Unique Users: 1
   Unique IPs: 2

   Event Details:
   Event 1: {...}
   Event 2: {...}
```

### Severity Levels

Patterns are automatically assigned severity based on event rate:
- **Critical**: > 10 events/minute
- **High**: 5-10 events/minute
- **Medium**: 2-5 events/minute
- **Low**: < 2 events/minute

## Customization

### Modify Patterns

Each example defines patterns at the top of the `main()` function. You can modify these patterns to test different scenarios:

```go
// Change time windows
pattern.Within(10 * time.Minute)  // Instead of 5 minutes

// Change quantifiers
Event("event", "type").Times(5)   // Instead of 3

// Add new conditions
pattern.WithCondition("a", "b", "ip_address", "eq")
```

### Create New Scenarios

Add new simulation functions to test different scenarios:

```go
func simulateMyScenario(engine *cep.CEPEngine) {
    baseTime := time.Now()

    engine.ProcessEvent(map[string]interface{}{
        "type":      "my_event",
        "user_id":   "test_user",
        "timestamp": baseTime,
    })

    // Add more events...
}
```

## Real-World Integration

These examples use in-memory event generation for demonstration. In production, integrate with actual event streams:

### With Kafka

```go
import "github.com/therealutkarshpriyadarshi/gress/pkg/source"

// Create Kafka source
kafkaSource := source.NewKafkaSource(/* config */)

// Process events from Kafka
go func() {
    for event := range kafkaSource.Events() {
        var eventData map[string]interface{}
        json.Unmarshal(event.Data, &eventData)
        engine.ProcessEvent(eventData)
    }
}()
```

### With Stream Processing

```go
import "github.com/therealutkarshpriyadarshi/gress/pkg/stream"

stream := stream.NewStream("events").
    Apply(cep.CEPOperator(engine)).
    ToSink(alertSink)
```

## Use Cases

### Fraud Detection Examples
- **E-commerce**: Detect suspicious purchase patterns
- **Banking**: Identify account takeover attempts
- **Gaming**: Detect bot accounts and cheating
- **Ad Tech**: Identify click fraud

### User Journey Examples
- **E-commerce**: Optimize conversion funnels
- **SaaS**: Track user onboarding journeys
- **Media**: Analyze content consumption patterns
- **Mobile Apps**: Understand user engagement flows

### Additional Possibilities
- **IoT Monitoring**: Sensor anomaly detection
- **Security**: Intrusion detection patterns
- **Operations**: System health monitoring
- **Customer Support**: Escalation pattern detection

## Performance Tips

1. **Event Volume**: These examples use small event volumes for clarity. The CEP engine can handle much higher throughput (1M+ events/second).

2. **Time Simulation**: Examples use `time.Now()` + offsets for simulation. In production, use actual event timestamps.

3. **Buffer Size**: Examples use small buffers (1000-2000 events). Adjust based on your event volume and pattern complexity.

4. **Callbacks**: Example callbacks print to stdout. In production, consider async processing, alerting systems, or database writes.

## Troubleshooting

### Pattern Not Matching

1. **Check event types**: Ensure `type` field matches exactly
2. **Verify time windows**: Events might be outside the time window
3. **Check conditions**: Field names and values must match exactly
4. **Review quantifiers**: Ensure enough events for `Times()` or `AtLeast()`

### Events Not Processing

1. **Verify event format**: Must be `map[string]interface{}`
2. **Check timestamps**: Events with timestamps far in the past may be pruned
3. **Review buffer size**: Buffer might be too small for your patterns

### Memory Usage

1. **Monitor statistics**: Use `engine.GetStatistics()`
2. **Adjust timeout**: Lower timeout for shorter-lived patterns
3. **Reduce buffer**: If memory is constrained

## Further Reading

- Main CEP documentation: [../../pkg/cep/README.md](../../pkg/cep/README.md)
- Pattern DSL reference: [../../pkg/cep/dsl.go](../../pkg/cep/dsl.go)
- CEP engine internals: [../../pkg/cep/engine.go](../../pkg/cep/engine.go)
- Tests and benchmarks: [../../pkg/cep/cep_test.go](../../pkg/cep/cep_test.go)

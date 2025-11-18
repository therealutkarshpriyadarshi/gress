# Complex Event Processing (CEP)

A comprehensive Complex Event Processing engine for detecting patterns in event streams with advanced capabilities including sequence detection, negation patterns, iteration patterns, temporal constraints, and pattern timeout handling.

## Features

- **Pattern Definition DSL**: Fluent API for defining complex event patterns
- **Sequence Detection**: Detect events in specific order (A followed by B followed by C)
- **Temporal Conditions**: Time-based constraints (within N seconds/minutes)
- **Negation Patterns**: Detect when events do NOT occur (A not followed by B)
- **Iteration Patterns**: Detect repeated events (A happens N times)
- **Pattern Timeout Handling**: Automatic cleanup of expired partial matches
- **Quantifiers**: Support for exact, at least, at most, range, and regex-style quantifiers
- **Condition Evaluation**: Compare field values between matched events
- **Severity Scoring**: Automatic severity calculation based on event patterns
- **Metadata Extraction**: Automatic extraction of relevant metadata from matches

## Quick Start

```go
package main

import (
    "time"
    "github.com/therealutkarshpriyadarshi/gress/pkg/cep"
)

func main() {
    // Create CEP engine
    engine := cep.NewCEPEngine(1000, 5*time.Minute)

    // Define a pattern using the DSL
    fraudPattern := cep.NewPattern("fraud-001", "Credential Stuffing").
        WithDescription("Multiple failed logins followed by success").
        Sequence(
            cep.Event("failed", "login_failed").Times(3),
            cep.Event("success", "login_success").Within(1*time.Minute),
        ).
        WithCondition("failed", "success", "user_id", "eq").
        Within(5 * time.Minute).
        Build()

    // Register pattern
    engine.RegisterPattern(fraudPattern)

    // Set up callback for matches
    engine.OnMatch(func(match cep.MatchedPattern) {
        fmt.Printf("Pattern detected: %s\n", match.PatternName)
        fmt.Printf("Severity: %s\n", match.Severity)
    })

    // Process events
    engine.ProcessEvent(map[string]interface{}{
        "type":    "login_failed",
        "user_id": "user123",
    })
}
```

## Pattern DSL Reference

### Basic Pattern Creation

```go
pattern := cep.NewPattern("pattern-id", "Pattern Name").
    WithDescription("Pattern description").
    Within(10 * time.Minute).
    Build()
```

### Event Patterns

```go
// Simple event
cep.Event("event_name", "event_type")

// Event with predicates
cep.Event("login", "user_login").
    With("status", "success").
    With("new_location", true)

// Event with multiple predicates
cep.Event("transaction", "payment").
    Where(map[string]interface{}{
        "amount": ">1000",
        "currency": "USD",
    })
```

### Quantifiers

```go
// Exactly N times
Event("click", "button_click").Times(3)

// At least N times
Event("view", "page_view").AtLeast(2)

// At most N times
Event("retry", "api_retry").AtMost(5)

// Between min and max
Event("attempt", "login_attempt").Between(2, 5)

// Optional event
Event("optional", "optional_step").Optional()
```

### Temporal Constraints

```go
// Event must occur within duration of previous event
Event("followup", "follow_up_action").Within(30 * time.Second)

// Pattern-level time window
pattern.Within(5 * time.Minute)
```

### Sequence Patterns

```go
// Simple sequence: A -> B -> C
pattern.Sequence(
    Event("a", "event_a"),
    Event("b", "event_b"),
    Event("c", "event_c"),
)

// Sequence with FollowedBy
pattern.
    FollowedBy(Event("a", "event_a")).
    FollowedBy(Event("b", "event_b")).
    FollowedBy(Event("c", "event_c"))
```

### Negation Patterns

```go
// A should NOT be followed by B
pattern.
    Sequence(Event("a", "event_a")).
    NotFollowedBy(Event("b", "event_b"))

// A -> NOT B -> C (B should not occur between A and C)
pattern.
    Sequence(Event("a", "event_a")).
    NotFollowedBy(Event("b", "event_b")).
    FollowedBy(Event("c", "event_c"))
```

### Conditions Between Events

```go
// Events must have same user_id
pattern.WithCondition("event1", "event2", "user_id", "eq")

// Amount must increase
pattern.WithCondition("event1", "event2", "amount", "lt")

// Using condition builder
condition := cep.NewCondition("login", "transaction").
    Field("user_id").
    Equals().
    Build()
pattern.Where(condition)
```

## Pattern Examples

### Fraud Detection: Credential Stuffing

```go
credentialStuffing := cep.NewPattern("fraud-001", "Credential Stuffing").
    Sequence(
        cep.Event("failed", "login_failed").Times(3),
        cep.Event("success", "login_success").Within(1*time.Minute),
    ).
    WithCondition("failed", "success", "user_id", "eq").
    Within(5 * time.Minute).
    Build()
```

### Fraud Detection: Suspicious Transaction

```go
suspiciousTransaction := cep.NewPattern("fraud-002", "Suspicious Transaction").
    Sequence(
        cep.Event("login", "user_login").With("new_location", true),
        cep.Event("transaction", "high_value_tx").
            With("amount", ">10000").
            Within(10*time.Minute),
    ).
    NotFollowedBy(
        cep.Event("2fa", "two_factor_auth").With("verified", true),
    ).
    Within(30 * time.Minute).
    Build()
```

### User Journey: Cart Abandonment

```go
cartAbandonment := cep.NewPattern("journey-001", "Cart Abandonment").
    Sequence(
        cep.Event("browse", "product_view").AtLeast(2),
        cep.Event("add_cart", "add_to_cart").AtLeast(1),
    ).
    NotFollowedBy(
        cep.Event("checkout", "checkout_initiated"),
    ).
    Within(1 * time.Hour).
    Build()
```

### User Journey: Successful Conversion

```go
conversion := cep.NewPattern("journey-002", "Successful Conversion").
    Sequence(
        cep.Event("browse", "product_view").AtLeast(1),
        cep.Event("add_cart", "add_to_cart").AtLeast(1),
        cep.Event("checkout", "checkout_initiated"),
        cep.Event("purchase", "purchase_completed").Within(30*time.Minute),
    ).
    Within(2 * time.Hour).
    Build()
```

### IoT: Sensor Anomaly Detection

```go
sensorAnomaly := cep.NewPattern("iot-001", "Sensor Spike").
    Sequence(
        cep.Event("high_reading", "sensor_reading").
            With("threshold_exceeded", true).
            AtLeast(3),
    ).
    Within(5 * time.Minute).
    Build()
```

### Account Takeover

```go
accountTakeover := cep.NewPattern("security-001", "Account Takeover").
    Sequence(
        cep.Event("pwd_change", "password_changed"),
        cep.Event("email_change", "email_changed").Within(5*time.Minute),
        cep.Event("withdrawal", "withdrawal_initiated").Within(10*time.Minute),
    ).
    Within(15 * time.Minute).
    Build()
```

## Engine Configuration

### Creating an Engine

```go
// NewCEPEngine(maxBufferSize, timeoutDuration)
engine := cep.NewCEPEngine(
    1000,              // Maximum events to buffer
    10*time.Minute,    // Pattern timeout duration
)
```

### Registering Patterns

```go
engine.RegisterPattern(pattern)
```

### Unregistering Patterns

```go
engine.UnregisterPattern("pattern-id")
```

### Setting Match Callback

```go
engine.OnMatch(func(match cep.MatchedPattern) {
    fmt.Printf("Pattern matched: %s\n", match.PatternName)
    fmt.Printf("Events: %d\n", len(match.MatchedEvents))
    fmt.Printf("Severity: %s\n", match.Severity)

    // Access matched events
    for i, event := range match.MatchedEvents {
        fmt.Printf("Event %d: %v\n", i+1, event)
    }

    // Access metadata
    fmt.Printf("Unique users: %v\n", match.Metadata["unique_users"])
    fmt.Printf("Unique IPs: %v\n", match.Metadata["unique_ips"])
})
```

### Processing Events

```go
matches := engine.ProcessEvent(map[string]interface{}{
    "type":      "event_type",
    "user_id":   "user123",
    "timestamp": time.Now(),
    // ... other fields
})
```

### Engine Statistics

```go
stats := engine.GetStatistics()
fmt.Printf("Registered patterns: %v\n", stats["registered_patterns"])
fmt.Printf("Active partial matches: %v\n", stats["active_partial_matches"])
fmt.Printf("Event buffer size: %v\n", stats["event_buffer_size"])
```

## Event Format

Events should be provided as `map[string]interface{}` with the following standard fields:

```go
event := map[string]interface{}{
    "type":       "event_type",    // Required: Event type to match
    "name":       "event_name",    // Optional: Used for conditions
    "timestamp":  time.Now(),      // Optional: Auto-added if missing
    "user_id":    "user123",       // Optional: Common field
    "ip_address": "192.168.1.1",   // Optional: Common field
    // ... any other custom fields
}
```

## Matched Pattern Structure

When a pattern matches, you receive a `MatchedPattern`:

```go
type MatchedPattern struct {
    PatternID      string                   // ID of the matched pattern
    PatternName    string                   // Name of the matched pattern
    MatchedEvents  []map[string]interface{} // All events that matched
    FirstEventTime time.Time                // Timestamp of first event
    LastEventTime  time.Time                // Timestamp of last event
    Severity       string                   // "low", "medium", "high", "critical"
    Metadata       map[string]interface{}   // Extracted metadata
}
```

### Severity Levels

Severity is automatically calculated based on event rate:
- **Critical**: > 10 events/minute
- **High**: 5-10 events/minute
- **Medium**: 2-5 events/minute
- **Low**: < 2 events/minute

### Metadata Fields

Automatically extracted metadata:
- `event_count`: Number of events in the match
- `unique_users`: Number of unique user IDs
- `unique_ips`: Number of unique IP addresses

## Advanced Features

### Pattern Timeout Handling

The engine automatically cleans up partial matches that have timed out based on:
1. Engine-level timeout (configured at creation)
2. Pattern-level timeout (pattern's time window)

Timeout checking runs every second in the background.

### Partial Match Tracking

The engine maintains partial matches for in-progress patterns, allowing:
- Long-running pattern detection
- Multiple simultaneous pattern instances
- Efficient memory management

### Predicate Operators

Special predicate operators in event matching:
- `">N"`: Greater than N (e.g., `"amount": ">10000"`)
- `"<N"`: Less than N (e.g., `"age": "<18"`)

### Condition Operators

Available operators for inter-event conditions:
- `eq`: Equal
- `ne`: Not equal
- `gt`: Greater than
- `lt`: Less than
- `gte`: Greater than or equal
- `lte`: Less than or equal

## Integration with Stream Processing

### As a Stream Operator

```go
import "github.com/therealutkarshpriyadarshi/gress/pkg/stream"

// Create CEP engine
engine := cep.NewCEPEngine(1000, 5*time.Minute)
engine.RegisterPattern(pattern)

// Use as stream operator
stream := stream.NewStream("events").
    Apply(cep.CEPOperator(engine)).
    ToSink(sink)
```

### With Legacy PatternDetector

```go
// For simple patterns, use the original PatternDetector
pattern := cep.SequencePattern(
    "simple-seq",
    "Simple Sequence",
    []string{"event_a", "event_b", "event_c"},
    5*time.Minute,
)

operator := cep.PatternDetector(pattern, 100)
```

## Testing

Run tests:

```bash
go test ./pkg/cep -v
```

Run benchmarks:

```bash
go test ./pkg/cep -bench=. -benchmem
```

## Examples

Complete examples are available in `examples/cep/`:

- **fraud_detection.go**: Fraud detection patterns including credential stuffing, suspicious transactions, automated account creation, and velocity fraud
- **user_journey.go**: User journey analysis including conversion funnels, cart abandonment, window shopping, and re-engagement

Run examples:

```bash
go run examples/cep/fraud_detection.go
go run examples/cep/user_journey.go
```

## Performance Considerations

1. **Buffer Size**: Set `maxBufferSize` based on expected event volume and pattern complexity
2. **Timeout Duration**: Balance between pattern detection window and memory usage
3. **Pattern Complexity**: Simpler patterns perform better; avoid overly complex patterns
4. **Event Rate**: Engine is designed to handle high-throughput scenarios (tested with 1M+ events/second)

## Best Practices

1. **Pattern Design**:
   - Keep patterns focused and specific
   - Use appropriate time windows
   - Leverage quantifiers for flexible matching

2. **Event Structure**:
   - Include `timestamp` for accurate temporal matching
   - Include `name` field for condition evaluation
   - Use consistent field names across event types

3. **Callbacks**:
   - Keep callback processing fast
   - Offload heavy processing to separate goroutines
   - Avoid blocking operations in callbacks

4. **Resource Management**:
   - Unregister unused patterns
   - Monitor engine statistics
   - Adjust buffer size based on usage

## License

This CEP engine is part of the Gress stream processing framework.

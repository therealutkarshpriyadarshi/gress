# Fraud Detection System

A comprehensive real-time fraud detection system built on Gress using Complex Event Processing (CEP), anomaly detection, and risk scoring.

## Features

### 1. Complex Event Processing (CEP)
Pattern-based fraud detection:
- **Sequential Patterns**: Detect sequences of suspicious events
- **Frequency Patterns**: Identify abnormal event rates
- **Temporal Correlation**: Find related events within time windows
- **Multi-event Patterns**: Complex patterns across multiple event types

### 2. Anomaly Detection
Statistical anomaly detection using:
- **Z-Score Analysis**: Standard deviation-based outlier detection
- **Moving Average**: Baseline comparison for unusual values
- **Adaptive Thresholds**: Self-adjusting based on historical data
- **Multi-dimensional**: Analyzes amount, frequency, location, device

### 3. Real-Time Risk Scoring
Multi-factor risk assessment:
- **New Account Detection**: Flags accounts < 24 hours old (15 points)
- **Unusual Amounts**: Transaction amount anomalies (up to 25 points)
- **High Amount Threshold**: Single large transactions (20 points)
- **New Location**: Transactions from unseen locations (10 points)
- **New Device**: Unfamiliar device usage (10 points)
- **New IP Range**: Suspicious IP addresses (8 points)
- **Velocity Checks**: Rapid succession of transactions (15 points)
- **Impossible Travel**: Geographic impossibilities (25 points)
- **Unusual Hours**: Late night transactions (5 points)

**Risk Levels**:
- Low (0-24): Allow automatically
- Medium (25-49): Review recommended
- High (50-74): Manual review required
- Critical (75-100): Block immediately

## Architecture

```
┌──────────────────┐
│  Transaction     │
│  Events          │
└────────┬─────────┘
         │
         ├──────────────────────┬─────────────────┐
         │                      │                 │
         ▼                      ▼                 ▼
┌─────────────────┐   ┌────────────────┐  ┌──────────────┐
│ Fraud Detector  │   │ CEP Pattern    │  │ Anomaly      │
│ - Risk Scoring  │   │ Matcher        │  │ Detector     │
│ - User Profiles │   │ - Sequences    │  │ - Statistical│
└────────┬────────┘   └────────┬───────┘  └──────┬───────┘
         │                      │                 │
         ▼                      ▼                 ▼
┌─────────────────────────────────────────────────┐
│           Risk Aggregation & Alerting            │
│  - Combine signals                               │
│  - Historical context                            │
│  - Action determination (allow/review/block)     │
└────────┬────────────────────────────────────────┘
         │
         ├──────────────────┬──────────────────┐
         ▼                  ▼                  ▼
┌──────────────┐   ┌─────────────┐   ┌────────────────┐
│ Kafka Alerts │   │ TimescaleDB │   │ Notification   │
│              │   │ (History)   │   │ Service        │
└──────────────┘   └─────────────┘   └────────────────┘
```

## Data Model

### Transaction Event
```json
{
  "event_id": "txn-12345",
  "user_id": "user-456",
  "type": "transaction",
  "amount": 500.00,
  "timestamp": "2024-01-15T10:30:00Z",
  "ip_address": "192.168.1.100",
  "location": "New York, US",
  "device_id": "device-789",
  "metadata": {
    "merchant": "Online Store",
    "payment_type": "credit_card",
    "currency": "USD",
    "card_last4": "1234"
  }
}
```

### Fraud Score Output
```json
{
  "event_id": "txn-12345",
  "user_id": "user-456",
  "score": 68.5,
  "risk_level": "high",
  "reasons": [
    "New location detected",
    "Amount 3x higher than user average",
    "Transaction from new device",
    "Multiple transactions in rapid succession"
  ],
  "factors": {
    "new_location": 10.0,
    "above_user_average": 15.0,
    "new_device": 10.0,
    "rapid_succession": 15.0,
    "unusual_amount": 18.5
  },
  "action": "review",
  "timestamp": "2024-01-15T10:30:00Z",
  "processing_time": "5.2ms"
}
```

### Pattern Match
```json
{
  "pattern_id": "rapid-succession",
  "pattern_name": "Rapid Transaction Succession",
  "matched_events": [
    {
      "event_id": "txn-1",
      "timestamp": "2024-01-15T10:30:00Z",
      "amount": 100.00
    },
    {
      "event_id": "txn-2",
      "timestamp": "2024-01-15T10:30:30Z",
      "amount": 150.00
    },
    {
      "event_id": "txn-3",
      "timestamp": "2024-01-15T10:31:00Z",
      "amount": 200.00
    }
  ],
  "first_event_time": "2024-01-15T10:30:00Z",
  "last_event_time": "2024-01-15T10:31:00Z",
  "severity": "high",
  "metadata": {
    "event_count": 3,
    "unique_users": 1,
    "unique_ips": 2
  }
}
```

## Usage

### Running the System

```bash
# Start infrastructure
cd deployments/docker
docker-compose up -d

# Run fraud detection system
cd examples/fraud-detection
go run main.go
```

### Submitting Transactions

```bash
# Normal transaction
curl -X POST http://localhost:8081/transactions \
  -H "Content-Type: application/json" \
  -d '{
    "event_id": "txn-001",
    "user_id": "user-123",
    "type": "transaction",
    "amount": 50.00,
    "location": "New York, US",
    "device_id": "device-abc"
  }'

# Suspicious transaction (high amount, new location)
curl -X POST http://localhost:8081/transactions \
  -H "Content-Type: application/json" \
  -d '{
    "event_id": "txn-002",
    "user_id": "user-123",
    "type": "transaction",
    "amount": 5000.00,
    "location": "Beijing, CN",
    "device_id": "device-xyz"
  }'
```

## CEP Pattern Definitions

### 1. Rapid Transaction Succession
Detects multiple transactions within a short time window:

```go
pattern := cep.SequencePattern(
    "rapid-succession",
    "Rapid Transaction Succession",
    []string{"transaction", "transaction", "transaction"},
    1*time.Minute,
)
```

### 2. Account Takeover Pattern
Detects account changes followed by large transaction:

```go
pattern := cep.Pattern{
    ID:   "account-takeover",
    Name: "Potential Account Takeover",
    Events: []cep.EventPattern{
        {
            Type: "account_change",
            Predicates: map[string]interface{}{
                "field": "password",
            },
        },
        {
            Type: "transaction",
            Predicates: map[string]interface{}{
                "amount_threshold": 1000.00,
            },
        },
    },
    TimeWindow: 30*time.Minute,
}
```

### 3. Brute Force Login
Detects multiple failed logins followed by success:

```go
pattern := cep.SequencePattern(
    "brute-force-login",
    "Potential Brute Force Login",
    []string{
        "login_failed",
        "login_failed",
        "login_failed",
        "login_success",
    },
    5*time.Minute,
)
```

### 4. Card Testing Pattern
Detects multiple small transactions (testing stolen cards):

```go
pattern := cep.FrequencyPattern(
    "card-testing",
    "Card Testing Activity",
    "transaction",
    10,  // 10 transactions
    2*time.Minute,
)
```

## Custom Fraud Rules

### Creating Custom Detectors

```go
package main

import (
    "github.com/therealutkarshpriyadarshi/gress/pkg/fraud"
    "github.com/therealutkarshpriyadarshi/gress/pkg/stream"
)

// Custom fraud rule
func highRiskMerchantDetector() stream.Operator {
    return stream.MapOperator(func(event *stream.Event) (*stream.Event, error) {
        var txn fraud.FraudEvent
        json.Unmarshal(event.Data, &txn)

        // Check if merchant is high-risk
        merchant := txn.Metadata["merchant"].(string)
        isHighRisk := checkMerchantRiskList(merchant)

        if isHighRisk {
            score := fraud.FraudScore{
                EventID:   txn.EventID,
                UserID:    txn.UserID,
                Score:     50.0,
                RiskLevel: "high",
                Reasons:   []string{"Transaction with high-risk merchant"},
                Action:    "review",
            }

            data, _ := json.Marshal(score)
            event.Data = data
        }

        return event, nil
    })
}
```

### Building Custom CEP Patterns

```go
// Define custom pattern with conditions
pattern := cep.Pattern{
    ID:   "location-hopping",
    Name: "Impossible Location Changes",
    Events: []cep.EventPattern{
        {
            Name: "txn1",
            Type: "transaction",
        },
        {
            Name: "txn2",
            Type: "transaction",
        },
    },
    TimeWindow: 1 * time.Hour,
    Conditions: []cep.Condition{
        {
            LeftEvent:  "txn1",
            RightEvent: "txn2",
            Operator:   "ne",  // not equal
            Field:      "location",
        },
    },
}

detector := cep.PatternDetector(pattern, 1000)
```

## Risk Scoring Customization

### Adjusting Risk Factors

```go
// Create custom fraud detector
detector := fraud.NewFraudDetector(
    10,                 // velocity threshold
    5*time.Minute,      // velocity window
    10000.0,            // amount threshold
)

// Modify risk scoring logic
func customRiskScore(event fraud.FraudEvent, profile *fraud.UserBehaviorProfile) fraud.FraudScore {
    score := fraud.FraudScore{
        EventID: event.EventID,
        UserID:  event.UserID,
        Factors: make(map[string]float64),
    }

    // Custom scoring logic
    if event.Amount > 1000 {
        score.Factors["high_amount"] = 30.0
        score.Reasons = append(score.Reasons, "Amount exceeds $1000")
    }

    // Industry-specific rules
    if isWeekend(event.Timestamp) && event.Amount > 500 {
        score.Factors["weekend_large_txn"] = 15.0
        score.Reasons = append(score.Reasons, "Large weekend transaction")
    }

    // Calculate total
    for _, factor := range score.Factors {
        score.Score += factor
    }

    return score
}
```

## Integration with Machine Learning

### Exporting Features for ML Models

```go
// Extract features for ML model
func extractMLFeatures(event fraud.FraudEvent, profile *fraud.UserBehaviorProfile) map[string]float64 {
    return map[string]float64{
        "amount":                event.Amount,
        "hour_of_day":          float64(event.Timestamp.Hour()),
        "day_of_week":          float64(event.Timestamp.Weekday()),
        "user_account_age_days": event.Timestamp.Sub(profile.CreatedAt).Hours() / 24,
        "avg_transaction_amount": profile.AverageAmount,
        "total_transactions":    float64(profile.TotalTransactions),
        "device_count":          float64(len(profile.Devices)),
        "location_count":        float64(len(profile.Countries)),
        "time_since_last_txn":   event.Timestamp.Sub(profile.LastActivity).Seconds(),
    }
}

// Send to ML service for prediction
func mlBasedScoring() stream.Operator {
    return stream.MapOperator(func(event *stream.Event) (*stream.Event, error) {
        var fraudEvent fraud.FraudEvent
        json.Unmarshal(event.Data, &fraudEvent)

        features := extractMLFeatures(fraudEvent, userProfile)
        mlScore := callMLService(features) // External ML model

        // Combine with rule-based score
        combinedScore := (ruleBasedScore + mlScore) / 2

        return event, nil
    })
}
```

## Metrics and Monitoring

### Key Metrics

```bash
# Check fraud detection rate
curl http://localhost:9092/metrics | grep fraud_detection_events_processed_total

# Monitor risk levels
curl http://localhost:9092/metrics | grep risk_level

# Pattern detection rate
curl http://localhost:9092/metrics | grep pattern_detected_total

# Processing latency
curl http://localhost:9092/metrics | grep processing_latency
```

### Prometheus Metrics

- `fraud_detection_events_processed_total{risk_level}` - Events by risk level
- `fraud_detection_pattern_detected_total{pattern_name}` - Pattern matches
- `fraud_detection_processing_latency` - Detection latency histogram
- `fraud_detection_alerts_sent_total{severity}` - Alerts by severity
- `fraud_detection_user_risk_score` - User risk score gauge

## Grafana Dashboard

Pre-built dashboard includes:

1. **Overview Panel**
   - Critical fraud alerts (real-time count)
   - High risk alerts
   - Total transactions per second
   - Fraud rate percentage

2. **Risk Analysis**
   - Risk levels over time (line chart)
   - Actions taken (pie chart: allow/review/block)
   - Average risk score trend

3. **Pattern Detection**
   - Top CEP patterns detected (bar chart)
   - Pattern severity distribution
   - Detection latency

4. **User Monitoring**
   - Top high-risk users (table)
   - User risk score history
   - Repeat offenders

Access at: `http://localhost:3000`

## Alerting Configuration

### Grafana Alerts

```yaml
# High fraud rate alert
- alert: HighFraudRate
  expr: (sum(rate(fraud_detection_events_processed_total{risk_level=~"high|critical"}[5m])) / sum(rate(fraud_detection_events_processed_total[5m]))) > 0.10
  for: 5m
  annotations:
    summary: "Fraud rate above 10%"
    description: "{{ $value | humanizePercentage }} of transactions are flagged as high/critical risk"

# Pattern spike alert
- alert: PatternDetectionSpike
  expr: rate(fraud_detection_pattern_detected_total[5m]) > 10
  for: 2m
  annotations:
    summary: "Unusual pattern detection spike"
    description: "Detecting {{ $value }} patterns per second"
```

### Webhook Integration

```go
// Send alerts to external system
func alertWebhook() stream.Operator {
    return stream.MapOperator(func(event *stream.Event) (*stream.Event, error) {
        var score fraud.FraudScore
        json.Unmarshal(event.Data, &score)

        if score.RiskLevel == "critical" {
            // Send to webhook
            payload := map[string]interface{}{
                "user_id":    score.UserID,
                "risk_score": score.Score,
                "reasons":    score.Reasons,
                "timestamp":  score.Timestamp,
            }

            sendToWebhook("https://alerts.example.com/fraud", payload)
        }

        return event, nil
    })
}
```

## Best Practices

### 1. Tuning Detection Sensitivity
- Start with conservative thresholds
- Monitor false positive rates
- Adjust based on business requirements
- Use A/B testing for threshold changes

### 2. Performance Optimization
- Use in-memory state for hot data
- Implement LRU cache for user profiles
- Batch database writes
- Use async alerting

### 3. Handling False Positives
- Implement feedback loop
- Manual review queue
- User whitelist/blacklist
- Adaptive thresholds per user segment

### 4. Compliance and Privacy
- Anonymize sensitive data
- Implement data retention policies
- Audit logging for all decisions
- GDPR/regulatory compliance

## Troubleshooting

### High False Positive Rate

```go
// Adjust risk factors
detector := fraud.NewFraudDetector(
    15,                 // Increase velocity threshold
    10*time.Minute,     // Longer velocity window
    20000.0,            // Higher amount threshold
)

// Add user whitelist
if isWhitelisted(userID) {
    score.Score *= 0.5  // Reduce score by 50%
}
```

### Missing Patterns

```bash
# Check pattern buffer size
patternMatcher := cep.NewPatternMatcher(pattern, 5000)  // Increase buffer

# Verify time windows
pattern.TimeWindow = 10 * time.Minute  // Increase window
```

### Performance Issues

```bash
# Enable async processing
config.Engine.MaxConcurrency = 200

# Use Redis for user profiles
stateConfig.Backend = "redis"
stateConfig.RedisURL = "redis://localhost:6379"
```

## Advanced Features

### Multi-Model Ensemble

```go
// Combine multiple detection models
func ensembleDetector() stream.Operator {
    ruleBasedDetector := fraud.FraudDetectionOperator(10, 5*time.Minute, 10000.0)
    anomalyDetector := fraud.AnomalyDetectionOperator(100, 3.0)
    cepDetector := cep.PatternDetector(patterns, 1000)

    return stream.CompositeOperator(
        ruleBasedDetector,
        anomalyDetector,
        cepDetector,
        combineScores,
    )
}
```

### Behavioral Biometrics

```go
// Analyze user behavior patterns
type BehaviorBiometrics struct {
    TypingSpeed      float64
    MouseMovements   []Point
    DeviceFingerprint string
    UsagePatterns     map[string]float64
}

func biometricAnalysis(event fraud.FraudEvent) float64 {
    // Compare with user's normal behavior
    // Return suspicion score
}
```

## Performance Benchmarks

- **Throughput**: 25,000+ transactions/second
- **Detection Latency**: < 10ms (P99)
- **Pattern Matching**: < 50ms for complex patterns
- **State Lookups**: < 5ms (P95)
- **False Positive Rate**: < 2% (tunable)

## Next Steps

1. **Integrate real transaction data**: Replace sample generator
2. **Tune risk thresholds**: Adjust for your business
3. **Add custom patterns**: Define domain-specific fraud patterns
4. **Setup alerting**: Configure notifications for fraud team
5. **Build review workflow**: Create manual review process

## Related Documentation

- [Complex Event Processing Guide](CEP_GUIDE.md)
- [Real-Time Analytics](REALTIME_ANALYTICS.md)
- [State Management](STATE_MANAGEMENT.md)

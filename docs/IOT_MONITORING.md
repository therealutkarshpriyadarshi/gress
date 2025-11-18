# IoT Sensor Monitoring System

A comprehensive real-time IoT sensor monitoring system with multi-sensor aggregation, threshold alerting, and predictive maintenance capabilities.

## Features

### 1. Multi-Sensor Aggregation
- **Device-Level Aggregation**: Combines readings from multiple sensors per device
- **Time-Window Statistics**: Min, max, average, standard deviation
- **Sensor Type Grouping**: Aggregates by sensor type (temperature, pressure, etc.)
- **Trend Detection**: Identifies increasing/decreasing/stable trends
- **Quality Metrics**: Tracks reading quality and reliability

### 2. Threshold Alerting
- **Configurable Rules**: Define thresholds per sensor type
- **Multi-Level Severity**: Info, warning, critical alerts
- **Real-Time Detection**: Immediate alerting on threshold violations
- **Alert Aggregation**: Prevents alert storms
- **Custom Thresholds**: Per-device, per-location customization

### 3. Predictive Maintenance
- **Trend Analysis**: Linear regression on sensor values
- **Degradation Detection**: Identifies declining sensor health
- **Time-to-Failure Prediction**: Estimates days until maintenance needed
- **Health Scoring**: 0-100 health score per sensor/device
- **Maintenance Scheduling**: Priority-based scheduling recommendations

### 4. Anomaly Detection
- **Statistical Outliers**: Z-score based anomaly detection
- **Baseline Comparison**: Deviations from normal patterns
- **Variance Analysis**: Detects unusual fluctuations
- **Quality-Based Filtering**: Accounts for sensor quality

## Architecture

```
┌──────────────────────────────────────┐
│  IoT Sensors (Temperature, Pressure, │
│  Vibration, Humidity, etc.)          │
└────────────┬─────────────────────────┘
             │
             ▼
┌──────────────────────────────────────┐
│  WebSocket/MQTT Ingestion            │
│  ws://localhost:8082/sensors         │
└────────────┬─────────────────────────┘
             │
             ├────────────────┬────────────────┬─────────────────┐
             │                │                │                 │
             ▼                ▼                ▼                 ▼
┌────────────────┐  ┌─────────────┐  ┌──────────────┐  ┌────────────┐
│ Multi-Sensor   │  │ Threshold   │  │ Predictive   │  │ Anomaly    │
│ Aggregator     │  │ Monitor     │  │ Maintenance  │  │ Detector   │
│ (1-min windows)│  │             │  │ Predictor    │  │            │
└────────┬───────┘  └──────┬──────┘  └──────┬───────┘  └─────┬──────┘
         │                 │                │                │
         ▼                 ▼                ▼                ▼
┌─────────────────────────────────────────────────────────────┐
│              Alert & Maintenance Management                  │
│  - Correlate alerts                                          │
│  - Schedule maintenance                                      │
│  - Send notifications                                        │
└────────────┬────────────────────────────────────────────────┘
             │
             ├──────────────────┬──────────────────┐
             ▼                  ▼                  ▼
┌──────────────────┐  ┌────────────────┐  ┌──────────────┐
│ TimescaleDB      │  │ Kafka Topics   │  │ Notification │
│ - Sensor Data    │  │ - Alerts       │  │ Service      │
│ - Maintenance    │  │ - Predictions  │  │              │
└──────────────────┘  └────────────────┘  └──────────────┘
```

## Data Model

### Sensor Reading
```json
{
  "sensor_id": "temp-sensor-001",
  "device_id": "device-123",
  "location": "Factory Floor A",
  "sensor_type": "temperature",
  "value": 75.5,
  "unit": "celsius",
  "timestamp": "2024-01-15T10:30:00Z",
  "quality": "good",
  "metadata": {
    "firmware_version": "1.2.3",
    "battery_level": 85
  }
}
```

### Aggregated Sensor Data
```json
{
  "device_id": "device-123",
  "location": "Factory Floor A",
  "window_start": "2024-01-15T10:00:00Z",
  "window_end": "2024-01-15T10:01:00Z",
  "sensor_count": 5,
  "reading_count": 60,
  "sensors": {
    "temp-sensor-001": {
      "sensor_id": "temp-sensor-001",
      "sensor_type": "temperature",
      "min": 70.2,
      "max": 76.8,
      "average": 73.5,
      "std_dev": 1.8,
      "count": 12,
      "last_value": 75.5,
      "trend": "stable"
    },
    "pressure-sensor-001": {
      "sensor_id": "pressure-sensor-001",
      "sensor_type": "pressure",
      "min": 98.5,
      "max": 102.3,
      "average": 100.2,
      "std_dev": 0.9,
      "count": 12,
      "last_value": 101.1,
      "trend": "increasing"
    }
  }
}
```

### Threshold Alert
```json
{
  "alert_id": "temp-sensor-001-temp-high-1705318200",
  "sensor_id": "temp-sensor-001",
  "device_id": "device-123",
  "alert_type": "threshold",
  "severity": "critical",
  "message": "critical: Temperature exceeds safe operating range (value: 85.50, expected: -10.00-80.00)",
  "value": 85.5,
  "threshold": 80.0,
  "timestamp": "2024-01-15T10:30:00Z",
  "acknowledged": false,
  "metadata": {
    "rule_id": "temp-high",
    "sensor_type": "temperature",
    "location": "Factory Floor A"
  }
}
```

### Maintenance Prediction
```json
{
  "sensor_id": "vibration-sensor-002",
  "device_id": "device-456",
  "timestamp": "2024-01-15T10:30:00Z",
  "requires_maintenance": true,
  "priority": "high",
  "estimated_days_to_failure": 5,
  "health_score": 45.2,
  "trend": 0.85,
  "degradation_rate": 0.15,
  "reason": "Degradation rate (15.00%) exceeds threshold",
  "recommended_actions": [
    "Inspect sensor",
    "Check connections",
    "Schedule sensor replacement",
    "Notify operations team"
  ]
}
```

## Usage

### Running the System

```bash
# Start infrastructure (Kafka, TimescaleDB, Prometheus, Grafana)
cd deployments/docker
docker-compose up -d

# Run IoT monitoring system
cd examples/iot-monitoring
go run main.go
```

### Sending Sensor Readings

#### WebSocket Connection
```javascript
// JavaScript client example
const ws = new WebSocket('ws://localhost:8082/sensors');

ws.onopen = () => {
  const reading = {
    sensor_id: "temp-sensor-001",
    device_id: "device-123",
    location: "Factory Floor A",
    sensor_type: "temperature",
    value: 75.5,
    unit: "celsius",
    quality: "good"
  };

  ws.send(JSON.stringify(reading));
};
```

#### HTTP Endpoint
```bash
curl -X POST http://localhost:8082/readings \
  -H "Content-Type: application/json" \
  -d '{
    "sensor_id": "temp-sensor-001",
    "device_id": "device-123",
    "sensor_type": "temperature",
    "value": 75.5,
    "unit": "celsius",
    "location": "Factory Floor A",
    "quality": "good"
  }'
```

#### MQTT (via Kafka Bridge)
```bash
# Publish to MQTT broker, bridged to Kafka
mosquitto_pub -t "sensors/temperature" -m '{
  "sensor_id": "temp-sensor-001",
  "value": 75.5
}'
```

## Threshold Configuration

### Defining Threshold Rules

```go
rules := []iot.ThresholdRule{
    {
        RuleID:      "temp-high",
        SensorType:  "temperature",
        MinValue:    -10,
        MaxValue:    80,
        Severity:    "critical",
        Description: "Temperature exceeds safe operating range",
    },
    {
        RuleID:      "pressure-high",
        SensorType:  "pressure",
        MinValue:    0,
        MaxValue:    150,
        Severity:    "warning",
        Description: "Pressure above normal threshold",
    },
    {
        RuleID:      "vibration-high",
        SensorType:  "vibration",
        MinValue:    0,
        MaxValue:    50,
        Severity:    "critical",
        Description: "Excessive vibration detected",
    },
    {
        RuleID:      "humidity-range",
        SensorType:  "humidity",
        MinValue:    30,
        MaxValue:    70,
        Severity:    "warning",
        Description: "Humidity outside optimal range",
    },
}

// Create alerting operator
alertOperator := iot.ThresholdAlertOperator(rules)
```

### Dynamic Thresholds

```go
// Per-device custom thresholds
func customThresholdOperator() stream.Operator {
    return stream.MapOperator(func(event *stream.Event) (*stream.Event, error) {
        var reading iot.SensorReading
        json.Unmarshal(event.Data, &reading)

        // Get device-specific thresholds from database
        threshold := getDeviceThreshold(reading.DeviceID, reading.SensorType)

        if reading.Value > threshold.Max || reading.Value < threshold.Min {
            alert := iot.Alert{
                SensorID:  reading.SensorID,
                DeviceID:  reading.DeviceID,
                AlertType: "threshold",
                Severity:  "warning",
                Message:   fmt.Sprintf("Value %f outside range %f-%f",
                    reading.Value, threshold.Min, threshold.Max),
                Value:     reading.Value,
                Threshold: threshold.Max,
                Timestamp: reading.Timestamp,
            }

            data, _ := json.Marshal(alert)
            event.Data = data
        }

        return event, nil
    })
}
```

## Predictive Maintenance Configuration

### Tuning Prediction Parameters

```go
// Create predictive maintenance operator
// historySize: number of readings to analyze
// degradationThreshold: % degradation to trigger alert
maintenanceOperator := iot.PredictiveMaintenanceOperator(
    100,    // analyze last 100 readings
    0.10,   // 10% degradation threshold
)

// Customize prediction logic
predictor := iot.NewMaintenancePredictor(100, 0.10)

// Adjust per sensor type
func customPredictor(sensorType string) *iot.MaintenancePredictor {
    switch sensorType {
    case "vibration":
        return iot.NewMaintenancePredictor(50, 0.05)  // More sensitive
    case "temperature":
        return iot.NewMaintenancePredictor(200, 0.15) // Less sensitive
    default:
        return iot.NewMaintenancePredictor(100, 0.10)
    }
}
```

### Custom Health Scoring

```go
func customHealthScore(readings []iot.SensorReading) float64 {
    score := 100.0

    // Factor 1: Recent trend
    trend := calculateTrend(readings)
    if trend < 0 {
        score -= math.Abs(trend) * 20
    }

    // Factor 2: Variance
    variance := calculateVariance(readings)
    score -= variance * 10

    // Factor 3: Reading quality
    poorQualityRatio := countPoorQuality(readings) / float64(len(readings))
    score -= poorQualityRatio * 30

    // Factor 4: Age of sensor
    sensorAge := getSensorAge(readings[0].SensorID)
    if sensorAge > 365 {
        score -= 10
    }

    return math.Max(score, 0)
}
```

## Custom Pipelines

### Building a Complete Monitoring Pipeline

```go
func createFullMonitoringPipeline() *stream.Pipeline {
    // 1. Multi-sensor aggregation
    aggregator := iot.MultiSensorAggregationOperator(1 * time.Minute)

    // 2. Threshold monitoring
    rules := defineThresholdRules()
    alertMonitor := iot.ThresholdAlertOperator(rules)

    // 3. Predictive maintenance
    predictor := iot.PredictiveMaintenanceOperator(100, 0.10)

    // 4. Alert correlation
    correlator := createAlertCorrelator()

    // 5. Notification router
    notifier := createNotificationRouter()

    return &stream.Pipeline{
        Source: wsSource,
        Operators: []stream.Operator{
            aggregator,
            alertMonitor,
            predictor,
            correlator,
            notifier,
        },
        Sink: compositeSink,
    }
}
```

### Alert Correlation

```go
// Correlate related alerts to reduce noise
func createAlertCorrelator() stream.Operator {
    alertWindow := make(map[string][]iot.Alert)

    return stream.MapOperator(func(event *stream.Event) (*stream.Event, error) {
        var alert iot.Alert
        json.Unmarshal(event.Data, &alert)

        deviceKey := alert.DeviceID
        alerts, exists := alertWindow[deviceKey]
        if !exists {
            alerts = make([]iot.Alert, 0)
        }

        alerts = append(alerts, alert)
        alertWindow[deviceKey] = alerts

        // Correlate if multiple alerts for same device
        if len(alerts) >= 3 {
            correlatedAlert := iot.Alert{
                AlertID:   fmt.Sprintf("correlated-%s", deviceKey),
                DeviceID:  deviceKey,
                AlertType: "correlated",
                Severity:  "critical",
                Message:   fmt.Sprintf("Multiple alerts for device %s", deviceKey),
                Metadata: map[string]interface{}{
                    "alert_count": len(alerts),
                    "alerts":      alerts,
                },
            }

            data, _ := json.Marshal(correlatedAlert)
            event.Data = data

            // Clear window
            delete(alertWindow, deviceKey)
        }

        return event, nil
    })
}
```

## Metrics and Monitoring

### Prometheus Metrics

```bash
# Active devices
curl http://localhost:9093/metrics | grep iot_monitoring_active_devices

# Sensor readings per second
curl http://localhost:9093/metrics | grep iot_monitoring_sensor_readings_total

# Alert rate
curl http://localhost:9093/metrics | grep iot_monitoring_alerts_total

# Maintenance predictions
curl http://localhost:9093/metrics | grep iot_monitoring_maintenance_required_total
```

### Available Metrics

- `iot_monitoring_sensor_readings_total{sensor_type,quality}` - Total readings
- `iot_monitoring_active_devices` - Currently active devices
- `iot_monitoring_active_sensors` - Currently active sensors
- `iot_monitoring_alerts_total{severity}` - Alerts by severity
- `iot_monitoring_maintenance_required_total{priority}` - Maintenance needed
- `iot_monitoring_processing_latency` - Processing latency histogram
- `iot_monitoring_aggregation_window_size` - Aggregation window sizes

## Grafana Dashboard

Pre-built dashboard includes:

### 1. Overview Panel
- Active devices count
- Active sensors count
- Active alerts (warning + critical)
- Maintenance required count

### 2. Sensor Trends
- Temperature trends (line chart)
- Pressure trends (line chart)
- Vibration levels (line chart)
- Humidity levels (line chart)

### 3. Health Monitoring
- Device health scores (multi-line chart)
- Maintenance schedule (table with priorities)
- Health score distribution (histogram)

### 4. Alert Management
- Alerts by severity (pie chart)
- Alert timeline (time series)
- Top alerting devices (table)

### 5. Analytics
- Sensor activity by location (bar chart)
- Reading quality distribution
- Sensor type distribution

Access at: `http://localhost:3000`

## Configuration

Example configuration `configs/iot-monitoring.yaml`:

```yaml
application:
  name: iot-monitoring
  environment: production

engine:
  buffer_size: 20000
  max_concurrency: 150
  checkpoint_interval: 30s
  watermark_interval: 5s

sources:
  websocket:
    listen_addr: ":8082"
    path: "/sensors"
    max_connections: 10000

  kafka:
    brokers:
      - kafka-1:9092
    topics:
      - sensor-readings
    group_id: iot-monitoring

sinks:
  timescaledb:
    connection_string: "postgresql://user:pass@timescale:5432/iot"
    tables:
      - sensor_aggregates
      - sensor_alerts
      - maintenance_schedule
    batch_size: 100
    flush_interval: 5s

  kafka:
    brokers:
      - kafka-1:9092
    topics:
      sensor-alerts: {}
      maintenance-predictions: {}

thresholds:
  temperature:
    min: -10
    max: 80
    severity: critical

  pressure:
    min: 0
    max: 150
    severity: warning

  vibration:
    min: 0
    max: 50
    severity: critical

predictive_maintenance:
  history_size: 100
  degradation_threshold: 0.10
  min_readings_required: 10

metrics:
  enabled: true
  port: 9093
  namespace: iot_monitoring
```

## Best Practices

### 1. Sensor Configuration
- **Sampling Rate**: Balance between data freshness and system load
  - Critical sensors: 1-5 seconds
  - Non-critical: 30-60 seconds
- **Quality Tracking**: Always include quality indicators
- **Metadata**: Include firmware version, battery level

### 2. Aggregation Windows
- **Real-time dashboards**: 1-minute windows
- **Trend analysis**: 5-15 minute windows
- **Historical analysis**: 1-hour windows

### 3. Alert Management
- **Severity Levels**: Use appropriate severities
  - Info: FYI, no action needed
  - Warning: Action recommended
  - Critical: Immediate action required
- **Alert Suppression**: Prevent alert storms
- **Escalation**: Automatic escalation for unacknowledged criticals

### 4. Predictive Maintenance
- **History Size**: At least 50-100 readings for accurate predictions
- **Calibration**: Regularly calibrate prediction thresholds
- **False Positives**: Track and adjust sensitivity

### 5. Data Retention
- **Raw Data**: 30-90 days
- **Aggregates**: 1-2 years
- **Alerts**: Indefinite (or per compliance requirements)

## Troubleshooting

### High Latency

```bash
# Check processing latency
curl http://localhost:9093/metrics | grep processing_latency

# Increase concurrency
config.Engine.MaxConcurrency = 300

# Increase buffer size
config.Engine.BufferSize = 50000
```

### Missing Sensor Data

```bash
# Check active sensors
curl http://localhost:9093/metrics | grep active_sensors

# Verify WebSocket connections
netstat -an | grep 8082

# Check Kafka consumer lag
kafka-consumer-groups --bootstrap-server localhost:9092 \
  --group iot-monitoring --describe
```

### Alert Storm

```go
// Implement alert throttling
func alertThrottler(maxPerMinute int) stream.Operator {
    alertCounts := make(map[string]int)
    windowStart := time.Now()

    return stream.MapOperator(func(event *stream.Event) (*stream.Event, error) {
        var alert iot.Alert
        json.Unmarshal(event.Data, &alert)

        // Reset window every minute
        if time.Since(windowStart) > time.Minute {
            alertCounts = make(map[string]int)
            windowStart = time.Now()
        }

        key := alert.SensorID
        count := alertCounts[key]

        if count >= maxPerMinute {
            return nil, nil // Drop alert
        }

        alertCounts[key]++
        return event, nil
    })
}
```

### Inaccurate Predictions

```go
// Tune prediction parameters
predictor := iot.NewMaintenancePredictor(
    200,   // Increase history size for more accuracy
    0.05,  // Lower threshold for earlier warnings
)

// Add sensor-specific calibration
calibrationFactors := map[string]float64{
    "vibration": 0.8,  // Vibration sensors degrade faster
    "temperature": 1.2, // Temperature sensors more stable
}
```

## Advanced Features

### Edge Computing Integration

```go
// Process at edge, send only alerts/aggregates
func edgeProcessingPipeline() *stream.Pipeline {
    // Local aggregation at edge
    edgeAggregator := iot.MultiSensorAggregationOperator(1 * time.Minute)

    // Local threshold checking
    localAlertMonitor := iot.ThresholdAlertOperator(rules)

    // Only send alerts and summary to cloud
    cloudFilter := stream.FilterOperator(func(event *stream.Event) bool {
        // Send only critical alerts or periodic summaries
        return isAlertOrSummary(event)
    })

    return &stream.Pipeline{
        Source:    edgeSource,
        Operators: []stream.Operator{edgeAggregator, localAlertMonitor, cloudFilter},
        Sink:      cloudSink,
    }
}
```

### Machine Learning Integration

```go
// Use ML for anomaly detection
func mlAnomalyDetector() stream.Operator {
    model := loadMLModel("anomaly-detector.model")

    return stream.MapOperator(func(event *stream.Event) (*stream.Event, error) {
        var reading iot.SensorReading
        json.Unmarshal(event.Data, &reading)

        // Extract features
        features := extractFeatures(reading)

        // Predict anomaly
        isAnomaly, confidence := model.Predict(features)

        if isAnomaly && confidence > 0.8 {
            alert := createAnomalyAlert(reading, confidence)
            data, _ := json.Marshal(alert)
            event.Data = data
        }

        return event, nil
    })
}
```

### Digital Twin Integration

```go
// Maintain digital twin of physical device
type DigitalTwin struct {
    DeviceID     string
    Sensors      map[string]*SensorState
    HealthScore  float64
    LastUpdated  time.Time
    Simulation   *SimulationModel
}

func updateDigitalTwin(reading iot.SensorReading) {
    twin := getDigitalTwin(reading.DeviceID)

    // Update sensor state
    twin.Sensors[reading.SensorID].Value = reading.Value
    twin.Sensors[reading.SensorID].Timestamp = reading.Timestamp

    // Run simulation
    twin.Simulation.Update(reading)

    // Predict next value
    predicted := twin.Simulation.PredictNext()

    // Compare actual vs predicted
    if math.Abs(reading.Value-predicted) > threshold {
        createAnomalyAlert(reading)
    }
}
```

## Performance Benchmarks

- **Throughput**: 100,000+ readings/second (single instance)
- **Aggregation Latency**: < 100ms (P99)
- **Alert Detection**: < 10ms (P95)
- **Prediction Latency**: < 50ms (P99)
- **State Size**: Supports 10,000+ active devices
- **Concurrent Connections**: 10,000+ WebSocket connections

## Use Cases

1. **Manufacturing**: Monitor production equipment, predict failures
2. **Data Centers**: Track temperature, humidity, power consumption
3. **Smart Buildings**: HVAC optimization, energy management
4. **Transportation**: Fleet monitoring, vehicle diagnostics
5. **Healthcare**: Medical device monitoring, patient vitals
6. **Agriculture**: Soil moisture, temperature, automated irrigation

## Next Steps

1. **Connect real sensors**: Replace sample data generator
2. **Configure thresholds**: Set appropriate limits for your domain
3. **Setup alerts**: Integrate with notification systems
4. **Train ML models**: Build custom anomaly detection
5. **Scale deployment**: Distribute across edge and cloud

## Related Documentation

- [Real-Time Analytics](REALTIME_ANALYTICS.md)
- [Complex Event Processing](CEP_GUIDE.md)
- [State Management](STATE_MANAGEMENT.md)
- [Deployment Guide](DEPLOYMENT.md)

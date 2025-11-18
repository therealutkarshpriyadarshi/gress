package iot

import (
	"encoding/json"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/therealutkarshpriyadarshi/gress/pkg/stream"
)

// SensorReading represents a reading from an IoT sensor
type SensorReading struct {
	SensorID    string                 `json:"sensor_id"`
	DeviceID    string                 `json:"device_id"`
	Location    string                 `json:"location"`
	SensorType  string                 `json:"sensor_type"` // "temperature", "pressure", "humidity", "vibration", etc.
	Value       float64                `json:"value"`
	Unit        string                 `json:"unit"`
	Timestamp   time.Time              `json:"timestamp"`
	Quality     string                 `json:"quality"`     // "good", "fair", "poor"
	Metadata    map[string]interface{} `json:"metadata"`
}

// AggregatedSensorData represents aggregated metrics from multiple sensors
type AggregatedSensorData struct {
	DeviceID       string    `json:"device_id"`
	Location       string    `json:"location"`
	WindowStart    time.Time `json:"window_start"`
	WindowEnd      time.Time `json:"window_end"`
	SensorCount    int       `json:"sensor_count"`
	ReadingCount   int       `json:"reading_count"`
	Sensors        map[string]*SensorStats `json:"sensors"`
}

// SensorStats holds statistics for a single sensor
type SensorStats struct {
	SensorID   string  `json:"sensor_id"`
	SensorType string  `json:"sensor_type"`
	Min        float64 `json:"min"`
	Max        float64 `json:"max"`
	Average    float64 `json:"average"`
	StdDev     float64 `json:"std_dev"`
	Count      int     `json:"count"`
	LastValue  float64 `json:"last_value"`
	Trend      string  `json:"trend"` // "increasing", "decreasing", "stable"
}

// Alert represents a sensor threshold alert
type Alert struct {
	AlertID      string                 `json:"alert_id"`
	SensorID     string                 `json:"sensor_id"`
	DeviceID     string                 `json:"device_id"`
	AlertType    string                 `json:"alert_type"` // "threshold", "anomaly", "failure", "maintenance"
	Severity     string                 `json:"severity"`   // "info", "warning", "critical"
	Message      string                 `json:"message"`
	Value        float64                `json:"value"`
	Threshold    float64                `json:"threshold"`
	Timestamp    time.Time              `json:"timestamp"`
	Acknowledged bool                   `json:"acknowledged"`
	Metadata     map[string]interface{} `json:"metadata"`
}

// ThresholdRule defines a rule for threshold alerting
type ThresholdRule struct {
	RuleID      string  `json:"rule_id"`
	SensorType  string  `json:"sensor_type"`
	MinValue    float64 `json:"min_value"`
	MaxValue    float64 `json:"max_value"`
	Severity    string  `json:"severity"`
	Description string  `json:"description"`
}

// MultiSensorAggregator aggregates data from multiple sensors
type MultiSensorAggregator struct {
	mu              sync.RWMutex
	deviceData      map[string]*AggregatedSensorData
	windowDuration  time.Duration
}

// NewMultiSensorAggregator creates a new multi-sensor aggregator
func NewMultiSensorAggregator(windowDuration time.Duration) *MultiSensorAggregator {
	return &MultiSensorAggregator{
		deviceData:     make(map[string]*AggregatedSensorData),
		windowDuration: windowDuration,
	}
}

// AddReading adds a sensor reading to the aggregator
func (msa *MultiSensorAggregator) AddReading(reading SensorReading) *AggregatedSensorData {
	msa.mu.Lock()
	defer msa.mu.Unlock()

	deviceKey := reading.DeviceID

	data, exists := msa.deviceData[deviceKey]
	if !exists || reading.Timestamp.After(data.WindowEnd) {
		// Create new window
		data = &AggregatedSensorData{
			DeviceID:    reading.DeviceID,
			Location:    reading.Location,
			WindowStart: reading.Timestamp,
			WindowEnd:   reading.Timestamp.Add(msa.windowDuration),
			Sensors:     make(map[string]*SensorStats),
		}
		msa.deviceData[deviceKey] = data
	}

	// Update or create sensor stats
	stats, exists := data.Sensors[reading.SensorID]
	if !exists {
		stats = &SensorStats{
			SensorID:   reading.SensorID,
			SensorType: reading.SensorType,
			Min:        reading.Value,
			Max:        reading.Value,
			Average:    reading.Value,
		}
		data.Sensors[reading.SensorID] = stats
		data.SensorCount++
	}

	// Update stats
	stats.Count++
	data.ReadingCount++

	if reading.Value < stats.Min {
		stats.Min = reading.Value
	}
	if reading.Value > stats.Max {
		stats.Max = reading.Value
	}

	// Update running average
	stats.Average = ((stats.Average * float64(stats.Count-1)) + reading.Value) / float64(stats.Count)
	stats.LastValue = reading.Value

	// Calculate trend (simplified)
	if stats.Count > 1 {
		if reading.Value > stats.Average*1.05 {
			stats.Trend = "increasing"
		} else if reading.Value < stats.Average*0.95 {
			stats.Trend = "decreasing"
		} else {
			stats.Trend = "stable"
		}
	}

	return data
}

// MultiSensorAggregationOperator creates an operator for multi-sensor aggregation
func MultiSensorAggregationOperator(windowDuration time.Duration) stream.Operator {
	aggregator := NewMultiSensorAggregator(windowDuration)

	return stream.MapOperator(func(event *stream.Event) (*stream.Event, error) {
		var reading SensorReading
		if err := json.Unmarshal(event.Data, &reading); err != nil {
			return nil, fmt.Errorf("failed to parse sensor reading: %w", err)
		}

		// Set timestamp if not provided
		if reading.Timestamp.IsZero() {
			reading.Timestamp = event.Timestamp
		}

		// Add reading to aggregator
		aggregatedData := aggregator.AddReading(reading)

		// Marshal aggregated data
		data, err := json.Marshal(aggregatedData)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal aggregated data: %w", err)
		}

		return &stream.Event{
			Key:       []byte(reading.DeviceID),
			Data:      data,
			Timestamp: event.Timestamp,
			Headers:   event.Headers,
			Partition: event.Partition,
			Offset:    event.Offset,
		}, nil
	})
}

// ThresholdMonitor monitors sensor readings against thresholds
type ThresholdMonitor struct {
	mu     sync.RWMutex
	rules  map[string][]ThresholdRule // sensor_type -> rules
	alerts map[string]*Alert           // alert_id -> alert
}

// NewThresholdMonitor creates a new threshold monitor
func NewThresholdMonitor(rules []ThresholdRule) *ThresholdMonitor {
	ruleMap := make(map[string][]ThresholdRule)
	for _, rule := range rules {
		ruleMap[rule.SensorType] = append(ruleMap[rule.SensorType], rule)
	}

	return &ThresholdMonitor{
		rules:  ruleMap,
		alerts: make(map[string]*Alert),
	}
}

// CheckReading checks a sensor reading against threshold rules
func (tm *ThresholdMonitor) CheckReading(reading SensorReading) []Alert {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	alerts := make([]Alert, 0)

	rules, exists := tm.rules[reading.SensorType]
	if !exists {
		return alerts
	}

	for _, rule := range rules {
		if reading.Value < rule.MinValue || reading.Value > rule.MaxValue {
			alertID := fmt.Sprintf("%s-%s-%d", reading.SensorID, rule.RuleID, reading.Timestamp.Unix())

			alert := Alert{
				AlertID:   alertID,
				SensorID:  reading.SensorID,
				DeviceID:  reading.DeviceID,
				AlertType: "threshold",
				Severity:  rule.Severity,
				Message:   fmt.Sprintf("%s: %s (value: %.2f, expected: %.2f-%.2f)", rule.Severity, rule.Description, reading.Value, rule.MinValue, rule.MaxValue),
				Value:     reading.Value,
				Threshold: rule.MaxValue,
				Timestamp: reading.Timestamp,
				Metadata: map[string]interface{}{
					"rule_id":     rule.RuleID,
					"sensor_type": reading.SensorType,
					"location":    reading.Location,
				},
			}

			tm.alerts[alertID] = &alert
			alerts = append(alerts, alert)
		}
	}

	return alerts
}

// ThresholdAlertOperator creates an operator for threshold alerting
func ThresholdAlertOperator(rules []ThresholdRule) stream.Operator {
	monitor := NewThresholdMonitor(rules)

	return stream.MapOperator(func(event *stream.Event) (*stream.Event, error) {
		var reading SensorReading
		if err := json.Unmarshal(event.Data, &reading); err != nil {
			return nil, fmt.Errorf("failed to parse sensor reading: %w", err)
		}

		// Set timestamp if not provided
		if reading.Timestamp.IsZero() {
			reading.Timestamp = event.Timestamp
		}

		// Check for threshold violations
		alerts := monitor.CheckReading(reading)

		// If alerts generated, emit them
		if len(alerts) > 0 {
			for _, alert := range alerts {
				alertData, err := json.Marshal(alert)
				if err != nil {
					return nil, fmt.Errorf("failed to marshal alert: %w", err)
				}

				return &stream.Event{
					Key:       []byte(alert.SensorID),
					Data:      alertData,
					Timestamp: event.Timestamp,
					Headers:   event.Headers,
					Partition: event.Partition,
					Offset:    event.Offset,
				}, nil
			}
		}

		return nil, nil // No alerts
	})
}

// MaintenancePredictor predicts maintenance needs based on sensor trends
type MaintenancePredictor struct {
	mu                  sync.RWMutex
	sensorHistory       map[string][]SensorReading
	historySize         int
	degradationThreshold float64
}

// NewMaintenancePredictor creates a new maintenance predictor
func NewMaintenancePredictor(historySize int, degradationThreshold float64) *MaintenancePredictor {
	return &MaintenancePredictor{
		sensorHistory:        make(map[string][]SensorReading),
		historySize:          historySize,
		degradationThreshold: degradationThreshold,
	}
}

// PredictMaintenance analyzes sensor trends to predict maintenance needs
func (mp *MaintenancePredictor) PredictMaintenance(reading SensorReading) *MaintenancePrediction {
	mp.mu.Lock()
	defer mp.mu.Unlock()

	// Add to history
	history, exists := mp.sensorHistory[reading.SensorID]
	if !exists {
		history = make([]SensorReading, 0, mp.historySize)
	}

	history = append(history, reading)
	if len(history) > mp.historySize {
		history = history[1:]
	}
	mp.sensorHistory[reading.SensorID] = history

	// Need at least 10 readings for prediction
	if len(history) < 10 {
		return nil
	}

	// Analyze trends
	prediction := &MaintenancePrediction{
		SensorID:  reading.SensorID,
		DeviceID:  reading.DeviceID,
		Timestamp: reading.Timestamp,
	}

	// Calculate linear regression for trend
	slope, intercept := linearRegression(history)
	prediction.Trend = slope

	// Calculate degradation rate
	degradationRate := calculateDegradationRate(history)
	prediction.DegradationRate = degradationRate

	// Predict time to failure (simplified)
	if math.Abs(degradationRate) > mp.degradationThreshold {
		prediction.RequiresMaintenance = true
		prediction.Priority = "high"
		prediction.EstimatedDaysToFailure = estimateTimeToFailure(slope, intercept, reading.SensorType)
		prediction.Reason = fmt.Sprintf("Degradation rate (%.2f%%) exceeds threshold", degradationRate*100)
	} else if math.Abs(slope) > 0.5 {
		prediction.RequiresMaintenance = true
		prediction.Priority = "medium"
		prediction.EstimatedDaysToFailure = estimateTimeToFailure(slope, intercept, reading.SensorType)
		prediction.Reason = "Significant trend detected"
	} else {
		prediction.RequiresMaintenance = false
		prediction.Priority = "low"
		prediction.Reason = "Operating normally"
	}

	// Calculate health score (0-100)
	prediction.HealthScore = calculateHealthScore(history, degradationRate)

	return prediction
}

// MaintenancePrediction represents a predictive maintenance assessment
type MaintenancePrediction struct {
	SensorID               string    `json:"sensor_id"`
	DeviceID               string    `json:"device_id"`
	Timestamp              time.Time `json:"timestamp"`
	RequiresMaintenance    bool      `json:"requires_maintenance"`
	Priority               string    `json:"priority"` // "low", "medium", "high"
	EstimatedDaysToFailure int       `json:"estimated_days_to_failure"`
	HealthScore            float64   `json:"health_score"` // 0-100
	Trend                  float64   `json:"trend"`
	DegradationRate        float64   `json:"degradation_rate"`
	Reason                 string    `json:"reason"`
}

// PredictiveMaintenanceOperator creates an operator for predictive maintenance
func PredictiveMaintenanceOperator(historySize int, degradationThreshold float64) stream.Operator {
	predictor := NewMaintenancePredictor(historySize, degradationThreshold)

	return stream.MapOperator(func(event *stream.Event) (*stream.Event, error) {
		var reading SensorReading
		if err := json.Unmarshal(event.Data, &reading); err != nil {
			return nil, fmt.Errorf("failed to parse sensor reading: %w", err)
		}

		// Set timestamp if not provided
		if reading.Timestamp.IsZero() {
			reading.Timestamp = event.Timestamp
		}

		// Predict maintenance needs
		prediction := predictor.PredictMaintenance(reading)

		if prediction == nil {
			return nil, nil // Not enough data yet
		}

		// Marshal prediction
		data, err := json.Marshal(prediction)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal prediction: %w", err)
		}

		return &stream.Event{
			Key:       []byte(reading.SensorID),
			Data:      data,
			Timestamp: event.Timestamp,
			Headers:   event.Headers,
			Partition: event.Partition,
			Offset:    event.Offset,
		}, nil
	})
}

// linearRegression calculates the slope and intercept of sensor readings over time
func linearRegression(readings []SensorReading) (slope, intercept float64) {
	n := len(readings)
	if n < 2 {
		return 0, 0
	}

	var sumX, sumY, sumXY, sumX2 float64
	for i, reading := range readings {
		x := float64(i)
		y := reading.Value
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}

	denominator := float64(n)*sumX2 - sumX*sumX
	if denominator == 0 {
		return 0, sumY / float64(n)
	}

	slope = (float64(n)*sumXY - sumX*sumY) / denominator
	intercept = (sumY - slope*sumX) / float64(n)

	return slope, intercept
}

// calculateDegradationRate computes the rate of degradation
func calculateDegradationRate(readings []SensorReading) float64 {
	if len(readings) < 2 {
		return 0
	}

	firstValue := readings[0].Value
	lastValue := readings[len(readings)-1].Value

	if firstValue == 0 {
		return 0
	}

	return (lastValue - firstValue) / firstValue
}

// estimateTimeToFailure estimates days until failure based on trend
func estimateTimeToFailure(slope, intercept float64, sensorType string) int {
	// Simplified estimation - in production use domain-specific models
	thresholds := map[string]float64{
		"temperature": 100.0,
		"pressure":    150.0,
		"vibration":   50.0,
		"humidity":    95.0,
	}

	threshold, exists := thresholds[sensorType]
	if !exists {
		threshold = 100.0
	}

	if slope == 0 {
		return 365 // Default to 1 year if no trend
	}

	// Calculate days until threshold is reached
	daysToThreshold := (threshold - intercept) / slope

	if daysToThreshold < 0 {
		return 0
	} else if daysToThreshold > 365 {
		return 365
	}

	return int(daysToThreshold)
}

// calculateHealthScore computes an overall health score (0-100)
func calculateHealthScore(readings []SensorReading, degradationRate float64) float64 {
	score := 100.0

	// Deduct points for degradation
	score -= math.Abs(degradationRate) * 100.0

	// Deduct points for variability
	if len(readings) > 1 {
		variance := calculateVariance(readings)
		score -= variance * 5.0
	}

	// Deduct points for poor quality readings
	poorQualityCount := 0
	for _, reading := range readings {
		if reading.Quality == "poor" {
			poorQualityCount++
		}
	}
	poorQualityRatio := float64(poorQualityCount) / float64(len(readings))
	score -= poorQualityRatio * 20.0

	if score < 0 {
		score = 0
	}

	return score
}

// calculateVariance computes the variance of sensor readings
func calculateVariance(readings []SensorReading) float64 {
	if len(readings) == 0 {
		return 0
	}

	sum := 0.0
	for _, reading := range readings {
		sum += reading.Value
	}
	mean := sum / float64(len(readings))

	variance := 0.0
	for _, reading := range readings {
		diff := reading.Value - mean
		variance += diff * diff
	}
	variance /= float64(len(readings))

	return math.Sqrt(variance) / mean // Coefficient of variation
}

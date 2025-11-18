package fraud

import (
	"encoding/json"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/therealutkarshpriyadarshi/gress/pkg/stream"
)

// FraudEvent represents a potentially fraudulent event
type FraudEvent struct {
	EventID      string                 `json:"event_id"`
	UserID       string                 `json:"user_id"`
	Type         string                 `json:"type"` // "transaction", "login", "account_change"
	Amount       float64                `json:"amount,omitempty"`
	Timestamp    time.Time              `json:"timestamp"`
	IPAddress    string                 `json:"ip_address"`
	Location     string                 `json:"location"`
	DeviceID     string                 `json:"device_id"`
	Metadata     map[string]interface{} `json:"metadata"`
}

// FraudScore represents the fraud risk assessment
type FraudScore struct {
	EventID        string                 `json:"event_id"`
	UserID         string                 `json:"user_id"`
	Score          float64                `json:"score"`          // 0-100, higher = more suspicious
	RiskLevel      string                 `json:"risk_level"`     // "low", "medium", "high", "critical"
	Reasons        []string               `json:"reasons"`        // Why it was flagged
	Factors        map[string]float64     `json:"factors"`        // Individual risk factors
	Action         string                 `json:"action"`         // "allow", "review", "block"
	Timestamp      time.Time              `json:"timestamp"`
	ProcessingTime time.Duration          `json:"processing_time"`
}

// AnomalyDetector detects statistical anomalies in event streams
type AnomalyDetector struct {
	mu              sync.RWMutex
	windowSize      int
	values          []float64
	mean            float64
	stdDev          float64
	threshold       float64 // Number of standard deviations
}

// NewAnomalyDetector creates a new anomaly detector
func NewAnomalyDetector(windowSize int, threshold float64) *AnomalyDetector {
	return &AnomalyDetector{
		windowSize: windowSize,
		values:     make([]float64, 0, windowSize),
		threshold:  threshold,
	}
}

// AddValue adds a new value and returns if it's an anomaly
func (ad *AnomalyDetector) AddValue(value float64) (bool, float64) {
	ad.mu.Lock()
	defer ad.mu.Unlock()

	// Add value to window
	ad.values = append(ad.values, value)
	if len(ad.values) > ad.windowSize {
		ad.values = ad.values[1:]
	}

	// Need at least 10 values to compute statistics
	if len(ad.values) < 10 {
		return false, 0
	}

	// Update statistics
	ad.updateStats()

	// Check if current value is an anomaly
	if ad.stdDev == 0 {
		return false, 0
	}

	zScore := math.Abs((value - ad.mean) / ad.stdDev)
	isAnomaly := zScore > ad.threshold

	return isAnomaly, zScore
}

// updateStats computes mean and standard deviation
func (ad *AnomalyDetector) updateStats() {
	sum := 0.0
	for _, v := range ad.values {
		sum += v
	}
	ad.mean = sum / float64(len(ad.values))

	variance := 0.0
	for _, v := range ad.values {
		diff := v - ad.mean
		variance += diff * diff
	}
	variance /= float64(len(ad.values))
	ad.stdDev = math.Sqrt(variance)
}

// UserBehaviorProfile tracks normal behavior for a user
type UserBehaviorProfile struct {
	UserID               string                 `json:"user_id"`
	TypicalTransactionAmount float64            `json:"typical_transaction_amount"`
	TypicalLocations     map[string]int         `json:"typical_locations"`
	TypicalDevices       map[string]int         `json:"typical_devices"`
	TypicalIPRanges      map[string]int         `json:"typical_ip_ranges"`
	TransactionFrequency float64                `json:"transaction_frequency"` // transactions per day
	LastActivity         time.Time              `json:"last_activity"`
	TotalTransactions    int                    `json:"total_transactions"`
	AverageAmount        float64                `json:"average_amount"`
	CreatedAt            time.Time              `json:"created_at"`
}

// FraudDetector manages fraud detection logic
type FraudDetector struct {
	mu                  sync.RWMutex
	userProfiles        map[string]*UserBehaviorProfile
	amountDetector      *AnomalyDetector
	frequencyDetector   *AnomalyDetector
	velocityThreshold   int           // Max transactions per time window
	velocityWindow      time.Duration
	amountThreshold     float64       // Max single transaction amount
}

// NewFraudDetector creates a new fraud detector
func NewFraudDetector(velocityThreshold int, velocityWindow time.Duration, amountThreshold float64) *FraudDetector {
	return &FraudDetector{
		userProfiles:      make(map[string]*UserBehaviorProfile),
		amountDetector:    NewAnomalyDetector(100, 3.0),
		frequencyDetector: NewAnomalyDetector(100, 2.5),
		velocityThreshold: velocityThreshold,
		velocityWindow:    velocityWindow,
		amountThreshold:   amountThreshold,
	}
}

// AnalyzeEvent analyzes an event for fraud indicators
func (fd *FraudDetector) AnalyzeEvent(event FraudEvent) FraudScore {
	startTime := time.Now()

	fd.mu.Lock()
	defer fd.mu.Unlock()

	score := FraudScore{
		EventID:   event.EventID,
		UserID:    event.UserID,
		Timestamp: event.Timestamp,
		Factors:   make(map[string]float64),
		Reasons:   make([]string, 0),
	}

	// Get or create user profile
	profile, exists := fd.userProfiles[event.UserID]
	if !exists {
		profile = &UserBehaviorProfile{
			UserID:           event.UserID,
			TypicalLocations: make(map[string]int),
			TypicalDevices:   make(map[string]int),
			TypicalIPRanges:  make(map[string]int),
			CreatedAt:        event.Timestamp,
		}
		fd.userProfiles[event.UserID] = profile
	}

	// 1. Check for new account
	accountAge := event.Timestamp.Sub(profile.CreatedAt)
	if accountAge < 24*time.Hour {
		score.Factors["new_account"] = 15.0
		score.Reasons = append(score.Reasons, "New account (< 24 hours old)")
	}

	// 2. Check transaction amount
	if event.Amount > 0 {
		isAnomaly, zScore := fd.amountDetector.AddValue(event.Amount)
		if isAnomaly {
			score.Factors["unusual_amount"] = math.Min(zScore*5, 25.0)
			score.Reasons = append(score.Reasons, fmt.Sprintf("Unusual transaction amount (%.2f std devs)", zScore))
		}

		if event.Amount > fd.amountThreshold {
			score.Factors["high_amount"] = 20.0
			score.Reasons = append(score.Reasons, fmt.Sprintf("High transaction amount ($%.2f)", event.Amount))
		}

		// Check against user's typical amount
		if profile.TotalTransactions > 10 && event.Amount > profile.AverageAmount*3 {
			score.Factors["above_user_average"] = 15.0
			score.Reasons = append(score.Reasons, "Amount 3x higher than user average")
		}
	}

	// 3. Check location
	if event.Location != "" {
		if profile.TotalTransactions > 5 {
			if _, known := profile.TypicalLocations[event.Location]; !known {
				score.Factors["new_location"] = 10.0
				score.Reasons = append(score.Reasons, "Transaction from new location")
			}
		}
		profile.TypicalLocations[event.Location]++
	}

	// 4. Check device
	if event.DeviceID != "" {
		if profile.TotalTransactions > 3 {
			if _, known := profile.TypicalDevices[event.DeviceID]; !known {
				score.Factors["new_device"] = 10.0
				score.Reasons = append(score.Reasons, "Transaction from new device")
			}
		}
		profile.TypicalDevices[event.DeviceID]++
	}

	// 5. Check IP address
	if event.IPAddress != "" {
		ipRange := getIPRange(event.IPAddress)
		if profile.TotalTransactions > 5 {
			if _, known := profile.TypicalIPRanges[ipRange]; !known {
				score.Factors["new_ip_range"] = 8.0
				score.Reasons = append(score.Reasons, "Transaction from new IP range")
			}
		}
		profile.TypicalIPRanges[ipRange]++
	}

	// 6. Check velocity (time since last transaction)
	if !profile.LastActivity.IsZero() {
		timeSinceLastActivity := event.Timestamp.Sub(profile.LastActivity)
		if timeSinceLastActivity < 1*time.Minute {
			score.Factors["rapid_succession"] = 15.0
			score.Reasons = append(score.Reasons, "Multiple transactions in rapid succession")
		} else if timeSinceLastActivity < 5*time.Minute {
			score.Factors["quick_succession"] = 8.0
			score.Reasons = append(score.Reasons, "Quick succession of transactions")
		}
	}

	// 7. Check for impossible travel (location change)
	// This would require geolocation data and time-based distance calculation
	// Simplified version: just check if location changed recently
	if profile.TotalTransactions > 0 && event.Location != "" {
		lastLocation := ""
		for loc := range profile.TypicalLocations {
			lastLocation = loc
			break
		}
		if lastLocation != "" && lastLocation != event.Location {
			timeSinceLastActivity := event.Timestamp.Sub(profile.LastActivity)
			if timeSinceLastActivity < 1*time.Hour {
				score.Factors["impossible_travel"] = 25.0
				score.Reasons = append(score.Reasons, "Impossible travel: location changed too quickly")
			}
		}
	}

	// 8. Time-based patterns
	hour := event.Timestamp.Hour()
	if hour >= 2 && hour <= 5 {
		score.Factors["unusual_time"] = 5.0
		score.Reasons = append(score.Reasons, "Transaction during unusual hours (2-5 AM)")
	}

	// Update profile
	profile.TotalTransactions++
	profile.LastActivity = event.Timestamp
	if event.Amount > 0 {
		profile.AverageAmount = ((profile.AverageAmount * float64(profile.TotalTransactions-1)) + event.Amount) / float64(profile.TotalTransactions)
	}

	// Calculate total score
	totalScore := 0.0
	for _, factorScore := range score.Factors {
		totalScore += factorScore
	}
	score.Score = math.Min(totalScore, 100.0)

	// Determine risk level and action
	if score.Score >= 75 {
		score.RiskLevel = "critical"
		score.Action = "block"
	} else if score.Score >= 50 {
		score.RiskLevel = "high"
		score.Action = "review"
	} else if score.Score >= 25 {
		score.RiskLevel = "medium"
		score.Action = "review"
	} else {
		score.RiskLevel = "low"
		score.Action = "allow"
	}

	score.ProcessingTime = time.Since(startTime)

	return score
}

// FraudDetectionOperator creates a stream operator for fraud detection
func FraudDetectionOperator(velocityThreshold int, velocityWindow time.Duration, amountThreshold float64) stream.Operator {
	detector := NewFraudDetector(velocityThreshold, velocityWindow, amountThreshold)

	return stream.MapOperator(func(event *stream.Event) (*stream.Event, error) {
		var fraudEvent FraudEvent
		if err := json.Unmarshal(event.Data, &fraudEvent); err != nil {
			return nil, fmt.Errorf("failed to parse fraud event: %w", err)
		}

		// Set timestamp if not provided
		if fraudEvent.Timestamp.IsZero() {
			fraudEvent.Timestamp = event.Timestamp
		}

		// Analyze for fraud
		score := detector.AnalyzeEvent(fraudEvent)

		// Marshal the score
		scoreData, err := json.Marshal(score)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal fraud score: %w", err)
		}

		return &stream.Event{
			Key:       []byte(fraudEvent.UserID),
			Data:      scoreData,
			Timestamp: event.Timestamp,
			Headers:   event.Headers,
			Partition: event.Partition,
			Offset:    event.Offset,
		}, nil
	})
}

// getIPRange extracts the /24 range from an IP address
func getIPRange(ip string) string {
	// Simple implementation - in production use proper IP parsing
	// Returns first 3 octets
	for i := len(ip) - 1; i >= 0; i-- {
		if ip[i] == '.' {
			return ip[:i] + ".0/24"
		}
	}
	return ip
}

// RiskAggregator aggregates risk scores across time windows
type RiskAggregator struct {
	mu             sync.RWMutex
	userRiskScores map[string][]float64
	windowSize     int
}

// NewRiskAggregator creates a new risk aggregator
func NewRiskAggregator(windowSize int) *RiskAggregator {
	return &RiskAggregator{
		userRiskScores: make(map[string][]float64),
		windowSize:     windowSize,
	}
}

// AddScore adds a risk score for a user
func (ra *RiskAggregator) AddScore(userID string, score float64) float64 {
	ra.mu.Lock()
	defer ra.mu.Unlock()

	scores, exists := ra.userRiskScores[userID]
	if !exists {
		scores = make([]float64, 0, ra.windowSize)
	}

	scores = append(scores, score)
	if len(scores) > ra.windowSize {
		scores = scores[1:]
	}

	ra.userRiskScores[userID] = scores

	// Calculate average risk score
	sum := 0.0
	for _, s := range scores {
		sum += s
	}
	return sum / float64(len(scores))
}

// RiskAggregationOperator creates an operator for risk score aggregation
func RiskAggregationOperator(windowSize int) stream.Operator {
	aggregator := NewRiskAggregator(windowSize)

	return stream.MapOperator(func(event *stream.Event) (*stream.Event, error) {
		var score FraudScore
		if err := json.Unmarshal(event.Data, &score); err != nil {
			return nil, fmt.Errorf("failed to parse fraud score: %w", err)
		}

		// Calculate aggregated risk
		avgRisk := aggregator.AddScore(score.UserID, score.Score)

		result := map[string]interface{}{
			"user_id":           score.UserID,
			"current_score":     score.Score,
			"average_risk":      avgRisk,
			"risk_level":        score.RiskLevel,
			"action":            score.Action,
			"timestamp":         score.Timestamp,
		}

		resultData, err := json.Marshal(result)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal aggregated risk: %w", err)
		}

		return &stream.Event{
			Key:       []byte(score.UserID),
			Data:      resultData,
			Timestamp: event.Timestamp,
			Headers:   event.Headers,
			Partition: event.Partition,
			Offset:    event.Offset,
		}, nil
	})
}

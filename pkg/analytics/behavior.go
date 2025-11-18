package analytics

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/therealutkarshpriyadarshi/gress/pkg/stream"
)

// UserBehavior represents a user action event
type UserBehavior struct {
	UserID    string                 `json:"user_id"`
	SessionID string                 `json:"session_id"`
	Action    string                 `json:"action"`    // e.g., "page_view", "click", "purchase", "login"
	Page      string                 `json:"page"`      // page or resource accessed
	Timestamp time.Time              `json:"timestamp"`
	Duration  int64                  `json:"duration"`  // milliseconds spent on page/action
	Metadata  map[string]interface{} `json:"metadata"`  // additional context
	IPAddress string                 `json:"ip_address"`
	UserAgent string                 `json:"user_agent"`
	Country   string                 `json:"country"`
	Device    string                 `json:"device"` // "mobile", "desktop", "tablet"
}

// BehaviorMetrics aggregates user behavior over a time window
type BehaviorMetrics struct {
	UserID          string    `json:"user_id"`
	SessionID       string    `json:"session_id"`
	WindowStart     time.Time `json:"window_start"`
	WindowEnd       time.Time `json:"window_end"`
	TotalEvents     int       `json:"total_events"`
	UniquePages     int       `json:"unique_pages"`
	TotalDuration   int64     `json:"total_duration_ms"`
	AverageDuration float64   `json:"average_duration_ms"`
	Actions         map[string]int    `json:"actions"` // count by action type
	Pages           map[string]int    `json:"pages"`   // count by page
	FirstSeen       time.Time `json:"first_seen"`
	LastSeen        time.Time `json:"last_seen"`
	DeviceType      string    `json:"device_type"`
	Country         string    `json:"country"`
}

// UserProfile maintains stateful information about a user
type UserProfile struct {
	UserID           string    `json:"user_id"`
	FirstSeen        time.Time `json:"first_seen"`
	LastSeen         time.Time `json:"last_seen"`
	TotalSessions    int       `json:"total_sessions"`
	TotalEvents      int       `json:"total_events"`
	TotalDuration    int64     `json:"total_duration_ms"`
	FavoritePages    []string  `json:"favorite_pages"`
	FavoriteActions  []string  `json:"favorite_actions"`
	Countries        map[string]int `json:"countries"`
	Devices          map[string]int `json:"devices"`
	IsActive         bool      `json:"is_active"`
	RiskScore        float64   `json:"risk_score"` // for fraud detection integration
}

// BehaviorTracker creates an operator that tracks user behavior patterns
func BehaviorTracker() stream.Operator {
	return stream.MapOperator(func(event *stream.Event) (*stream.Event, error) {
		var behavior UserBehavior
		if err := json.Unmarshal(event.Data, &behavior); err != nil {
			return nil, fmt.Errorf("failed to parse user behavior: %w", err)
		}

		// Enrich the event with computed fields
		enriched := map[string]interface{}{
			"user_id":    behavior.UserID,
			"session_id": behavior.SessionID,
			"action":     behavior.Action,
			"page":       behavior.Page,
			"timestamp":  behavior.Timestamp,
			"duration":   behavior.Duration,
			"metadata":   behavior.Metadata,
			"ip_address": behavior.IPAddress,
			"user_agent": behavior.UserAgent,
			"country":    behavior.Country,
			"device":     behavior.Device,
			// Add computed fields
			"is_conversion": isConversionAction(behavior.Action),
			"is_high_value": behavior.Duration > 30000, // > 30 seconds
			"hour_of_day":   behavior.Timestamp.Hour(),
			"day_of_week":   behavior.Timestamp.Weekday().String(),
		}

		enrichedData, err := json.Marshal(enriched)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal enriched behavior: %w", err)
		}

		return &stream.Event{
			Key:       []byte(behavior.UserID),
			Data:      enrichedData,
			Timestamp: behavior.Timestamp,
			Headers:   event.Headers,
			Partition: event.Partition,
			Offset:    event.Offset,
		}, nil
	})
}

// SessionAggregator aggregates user behavior by session
func SessionAggregator() stream.Operator {
	sessionMetrics := make(map[string]*BehaviorMetrics)

	return stream.MapOperator(func(event *stream.Event) (*stream.Event, error) {
		var behavior UserBehavior
		if err := json.Unmarshal(event.Data, &behavior); err != nil {
			return nil, fmt.Errorf("failed to parse user behavior: %w", err)
		}

		sessionKey := fmt.Sprintf("%s:%s", behavior.UserID, behavior.SessionID)

		metrics, exists := sessionMetrics[sessionKey]
		if !exists {
			metrics = &BehaviorMetrics{
				UserID:      behavior.UserID,
				SessionID:   behavior.SessionID,
				WindowStart: behavior.Timestamp,
				Actions:     make(map[string]int),
				Pages:       make(map[string]int),
				FirstSeen:   behavior.Timestamp,
				DeviceType:  behavior.Device,
				Country:     behavior.Country,
			}
			sessionMetrics[sessionKey] = metrics
		}

		// Update metrics
		metrics.TotalEvents++
		metrics.TotalDuration += behavior.Duration
		metrics.Actions[behavior.Action]++
		metrics.Pages[behavior.Page]++
		metrics.LastSeen = behavior.Timestamp
		metrics.WindowEnd = behavior.Timestamp
		metrics.AverageDuration = float64(metrics.TotalDuration) / float64(metrics.TotalEvents)
		metrics.UniquePages = len(metrics.Pages)

		// Marshal the metrics
		metricsData, err := json.Marshal(metrics)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal session metrics: %w", err)
		}

		return &stream.Event{
			Key:       []byte(sessionKey),
			Data:      metricsData,
			Timestamp: behavior.Timestamp,
			Headers:   event.Headers,
			Partition: event.Partition,
			Offset:    event.Offset,
		}, nil
	})
}

// PageViewTracker tracks page view statistics
func PageViewTracker() stream.Operator {
	pageStats := make(map[string]*PageStats)

	return stream.MapOperator(func(event *stream.Event) (*stream.Event, error) {
		var behavior UserBehavior
		if err := json.Unmarshal(event.Data, &behavior); err != nil {
			return nil, fmt.Errorf("failed to parse user behavior: %w", err)
		}

		if behavior.Action != "page_view" {
			return nil, nil // Filter non-page-view events
		}

		stats, exists := pageStats[behavior.Page]
		if !exists {
			stats = &PageStats{
				Page:       behavior.Page,
				FirstSeen:  behavior.Timestamp,
			}
			pageStats[behavior.Page] = stats
		}

		stats.TotalViews++
		stats.UniqueUsers++
		stats.TotalDuration += behavior.Duration
		stats.AverageDuration = float64(stats.TotalDuration) / float64(stats.TotalViews)
		stats.LastSeen = behavior.Timestamp

		statsData, err := json.Marshal(stats)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal page stats: %w", err)
		}

		return &stream.Event{
			Key:       []byte(behavior.Page),
			Data:      statsData,
			Timestamp: behavior.Timestamp,
			Headers:   event.Headers,
			Partition: event.Partition,
			Offset:    event.Offset,
		}, nil
	})
}

// PageStats tracks statistics for a single page
type PageStats struct {
	Page            string    `json:"page"`
	TotalViews      int       `json:"total_views"`
	UniqueUsers     int       `json:"unique_users"`
	TotalDuration   int64     `json:"total_duration_ms"`
	AverageDuration float64   `json:"average_duration_ms"`
	FirstSeen       time.Time `json:"first_seen"`
	LastSeen        time.Time `json:"last_seen"`
}

// ConversionFunnel tracks conversion through a funnel
type ConversionFunnel struct {
	FunnelID   string             `json:"funnel_id"`
	Stages     []string           `json:"stages"`
	UserCounts map[string]int     `json:"user_counts"` // stage -> count
	Timestamp  time.Time          `json:"timestamp"`
}

// isConversionAction determines if an action is a conversion
func isConversionAction(action string) bool {
	conversions := map[string]bool{
		"purchase":    true,
		"signup":      true,
		"subscribe":   true,
		"download":    true,
		"submit_form": true,
	}
	return conversions[action]
}

// EngagementScoreCalculator calculates user engagement scores
func EngagementScoreCalculator() stream.Operator {
	return stream.MapOperator(func(event *stream.Event) (*stream.Event, error) {
		var metrics BehaviorMetrics
		if err := json.Unmarshal(event.Data, &metrics); err != nil {
			return nil, fmt.Errorf("failed to parse behavior metrics: %w", err)
		}

		// Calculate engagement score based on multiple factors
		score := calculateEngagementScore(metrics)

		result := map[string]interface{}{
			"user_id":         metrics.UserID,
			"session_id":      metrics.SessionID,
			"engagement_score": score,
			"total_events":    metrics.TotalEvents,
			"unique_pages":    metrics.UniquePages,
			"total_duration":  metrics.TotalDuration,
			"window_start":    metrics.WindowStart,
			"window_end":      metrics.WindowEnd,
		}

		resultData, err := json.Marshal(result)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal engagement score: %w", err)
		}

		return &stream.Event{
			Key:       []byte(metrics.UserID),
			Data:      resultData,
			Timestamp: metrics.WindowEnd,
			Headers:   event.Headers,
			Partition: event.Partition,
			Offset:    event.Offset,
		}, nil
	})
}

// calculateEngagementScore computes an engagement score (0-100)
func calculateEngagementScore(metrics BehaviorMetrics) float64 {
	score := 0.0

	// Events factor (0-30 points)
	eventScore := float64(metrics.TotalEvents) * 2.0
	if eventScore > 30 {
		eventScore = 30
	}
	score += eventScore

	// Page diversity factor (0-20 points)
	pageScore := float64(metrics.UniquePages) * 4.0
	if pageScore > 20 {
		pageScore = 20
	}
	score += pageScore

	// Duration factor (0-30 points)
	durationScore := float64(metrics.TotalDuration) / 10000.0 // 10 seconds = 1 point
	if durationScore > 30 {
		durationScore = 30
	}
	score += durationScore

	// Action diversity factor (0-20 points)
	actionScore := float64(len(metrics.Actions)) * 5.0
	if actionScore > 20 {
		actionScore = 20
	}
	score += actionScore

	if score > 100 {
		score = 100
	}

	return score
}

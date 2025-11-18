package analytics

import (
	"encoding/json"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/therealutkarshpriyadarshi/gress/pkg/stream"
)

// KPI represents a Key Performance Indicator
type KPI struct {
	Name        string                 `json:"name"`
	Value       float64                `json:"value"`
	Target      float64                `json:"target"`
	Unit        string                 `json:"unit"`        // e.g., "count", "percentage", "seconds", "dollars"
	Trend       string                 `json:"trend"`       // "up", "down", "stable"
	ChangeRate  float64                `json:"change_rate"` // percentage change
	Timestamp   time.Time              `json:"timestamp"`
	WindowStart time.Time              `json:"window_start"`
	WindowEnd   time.Time              `json:"window_end"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// KPIMetrics holds multiple KPI values for a time window
type KPIMetrics struct {
	WindowStart       time.Time `json:"window_start"`
	WindowEnd         time.Time `json:"window_end"`
	TotalUsers        int       `json:"total_users"`
	ActiveUsers       int       `json:"active_users"`
	NewUsers          int       `json:"new_users"`
	TotalSessions     int       `json:"total_sessions"`
	TotalPageViews    int       `json:"total_page_views"`
	TotalEvents       int       `json:"total_events"`
	TotalRevenue      float64   `json:"total_revenue"`
	TotalConversions  int       `json:"total_conversions"`
	AverageSessionDuration float64 `json:"average_session_duration_seconds"`
	BounceRate        float64   `json:"bounce_rate_percent"`
	ConversionRate    float64   `json:"conversion_rate_percent"`
	RevenuePerUser    float64   `json:"revenue_per_user"`
	ChurnRate         float64   `json:"churn_rate_percent"`
	RetentionRate     float64   `json:"retention_rate_percent"`
}

// RevenueMetrics tracks revenue-related KPIs
type RevenueMetrics struct {
	WindowStart          time.Time `json:"window_start"`
	WindowEnd            time.Time `json:"window_end"`
	TotalRevenue         float64   `json:"total_revenue"`
	RevenueByProduct     map[string]float64 `json:"revenue_by_product"`
	RevenueByCountry     map[string]float64 `json:"revenue_by_country"`
	TotalTransactions    int       `json:"total_transactions"`
	AverageOrderValue    float64   `json:"average_order_value"`
	RevenuePerMinute     float64   `json:"revenue_per_minute"`
	TopProducts          []string  `json:"top_products"`
	TopCountries         []string  `json:"top_countries"`
}

// KPICalculator manages real-time KPI calculations
type KPICalculator struct {
	mu              sync.RWMutex
	metrics         *KPIMetrics
	previousMetrics *KPIMetrics
	windowStart     time.Time
	windowDuration  time.Duration
}

// NewKPICalculator creates a new KPI calculator with a specified window duration
func NewKPICalculator(windowDuration time.Duration) *KPICalculator {
	now := time.Now()
	return &KPICalculator{
		metrics: &KPIMetrics{
			WindowStart: now,
			WindowEnd:   now.Add(windowDuration),
		},
		windowStart:    now,
		windowDuration: windowDuration,
	}
}

// LiveKPICalculator creates an operator that calculates KPIs in real-time
func LiveKPICalculator(windowDuration time.Duration) stream.Operator {
	calculator := NewKPICalculator(windowDuration)
	userSessions := make(map[string]bool)
	userFirstSeen := make(map[string]time.Time)
	sessionDurations := make(map[string]int64)

	return stream.MapOperator(func(event *stream.Event) (*stream.Event, error) {
		var behavior UserBehavior
		if err := json.Unmarshal(event.Data, &behavior); err != nil {
			return nil, fmt.Errorf("failed to parse user behavior: %w", err)
		}

		calculator.mu.Lock()
		defer calculator.mu.Unlock()

		// Check if we need to rotate the window
		if event.Timestamp.After(calculator.metrics.WindowEnd) {
			calculator.rotateWindow()
			userSessions = make(map[string]bool)
			userFirstSeen = make(map[string]time.Time)
			sessionDurations = make(map[string]int64)
		}

		// Update metrics
		calculator.metrics.TotalEvents++

		if behavior.Action == "page_view" {
			calculator.metrics.TotalPageViews++
		}

		sessionKey := fmt.Sprintf("%s:%s", behavior.UserID, behavior.SessionID)
		if !userSessions[sessionKey] {
			calculator.metrics.TotalSessions++
			userSessions[sessionKey] = true
		}

		if firstSeen, exists := userFirstSeen[behavior.UserID]; !exists {
			userFirstSeen[behavior.UserID] = event.Timestamp
			calculator.metrics.NewUsers++
			calculator.metrics.TotalUsers++
		} else {
			// Update last seen
			if event.Timestamp.Sub(firstSeen) < 24*time.Hour {
				calculator.metrics.ActiveUsers++
			}
		}

		sessionDurations[sessionKey] += behavior.Duration

		// Calculate conversion
		if isConversionAction(behavior.Action) {
			calculator.metrics.TotalConversions++
		}

		// Calculate derived metrics
		if calculator.metrics.TotalSessions > 0 {
			totalSessionDuration := int64(0)
			for _, duration := range sessionDurations {
				totalSessionDuration += duration
			}
			calculator.metrics.AverageSessionDuration = float64(totalSessionDuration) / float64(calculator.metrics.TotalSessions) / 1000.0 // convert to seconds

			calculator.metrics.ConversionRate = float64(calculator.metrics.TotalConversions) / float64(calculator.metrics.TotalSessions) * 100.0
		}

		if calculator.metrics.TotalUsers > 0 {
			calculator.metrics.RevenuePerUser = calculator.metrics.TotalRevenue / float64(calculator.metrics.TotalUsers)
		}

		// Create KPI snapshot
		kpiData, err := json.Marshal(calculator.metrics)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal KPI metrics: %w", err)
		}

		return &stream.Event{
			Key:       []byte("kpi"),
			Data:      kpiData,
			Timestamp: event.Timestamp,
			Headers:   event.Headers,
			Partition: event.Partition,
			Offset:    event.Offset,
		}, nil
	})
}

// rotateWindow moves to the next time window
func (kc *KPICalculator) rotateWindow() {
	kc.previousMetrics = kc.metrics
	now := time.Now()
	kc.metrics = &KPIMetrics{
		WindowStart: now,
		WindowEnd:   now.Add(kc.windowDuration),
	}
	kc.windowStart = now
}

// CalculateTrends computes KPI trends by comparing current and previous windows
func (kc *KPICalculator) CalculateTrends() []KPI {
	kc.mu.RLock()
	defer kc.mu.RUnlock()

	kpis := []KPI{}

	if kc.previousMetrics == nil {
		return kpis
	}

	// Total Users KPI
	kpis = append(kpis, KPI{
		Name:      "Total Users",
		Value:     float64(kc.metrics.TotalUsers),
		Unit:      "count",
		Trend:     calculateTrend(float64(kc.previousMetrics.TotalUsers), float64(kc.metrics.TotalUsers)),
		ChangeRate: calculateChangeRate(float64(kc.previousMetrics.TotalUsers), float64(kc.metrics.TotalUsers)),
		Timestamp:  time.Now(),
		WindowStart: kc.metrics.WindowStart,
		WindowEnd:   kc.metrics.WindowEnd,
	})

	// Active Users KPI
	kpis = append(kpis, KPI{
		Name:      "Active Users",
		Value:     float64(kc.metrics.ActiveUsers),
		Unit:      "count",
		Trend:     calculateTrend(float64(kc.previousMetrics.ActiveUsers), float64(kc.metrics.ActiveUsers)),
		ChangeRate: calculateChangeRate(float64(kc.previousMetrics.ActiveUsers), float64(kc.metrics.ActiveUsers)),
		Timestamp:  time.Now(),
		WindowStart: kc.metrics.WindowStart,
		WindowEnd:   kc.metrics.WindowEnd,
	})

	// Conversion Rate KPI
	kpis = append(kpis, KPI{
		Name:      "Conversion Rate",
		Value:     kc.metrics.ConversionRate,
		Unit:      "percentage",
		Trend:     calculateTrend(kc.previousMetrics.ConversionRate, kc.metrics.ConversionRate),
		ChangeRate: calculateChangeRate(kc.previousMetrics.ConversionRate, kc.metrics.ConversionRate),
		Timestamp:  time.Now(),
		WindowStart: kc.metrics.WindowStart,
		WindowEnd:   kc.metrics.WindowEnd,
	})

	// Average Session Duration KPI
	kpis = append(kpis, KPI{
		Name:      "Avg Session Duration",
		Value:     kc.metrics.AverageSessionDuration,
		Unit:      "seconds",
		Trend:     calculateTrend(kc.previousMetrics.AverageSessionDuration, kc.metrics.AverageSessionDuration),
		ChangeRate: calculateChangeRate(kc.previousMetrics.AverageSessionDuration, kc.metrics.AverageSessionDuration),
		Timestamp:  time.Now(),
		WindowStart: kc.metrics.WindowStart,
		WindowEnd:   kc.metrics.WindowEnd,
	})

	// Revenue Per User KPI
	kpis = append(kpis, KPI{
		Name:      "Revenue Per User",
		Value:     kc.metrics.RevenuePerUser,
		Unit:      "dollars",
		Trend:     calculateTrend(kc.previousMetrics.RevenuePerUser, kc.metrics.RevenuePerUser),
		ChangeRate: calculateChangeRate(kc.previousMetrics.RevenuePerUser, kc.metrics.RevenuePerUser),
		Timestamp:  time.Now(),
		WindowStart: kc.metrics.WindowStart,
		WindowEnd:   kc.metrics.WindowEnd,
	})

	return kpis
}

// calculateTrend determines if a metric is trending up, down, or stable
func calculateTrend(previous, current float64) string {
	if previous == 0 {
		return "stable"
	}

	changeRate := ((current - previous) / previous) * 100.0

	if changeRate > 5.0 {
		return "up"
	} else if changeRate < -5.0 {
		return "down"
	}
	return "stable"
}

// calculateChangeRate computes the percentage change between two values
func calculateChangeRate(previous, current float64) float64 {
	if previous == 0 {
		return 0
	}
	return ((current - previous) / previous) * 100.0
}

// RevenueCalculator tracks revenue metrics in real-time
func RevenueCalculator(windowDuration time.Duration) stream.Operator {
	metrics := &RevenueMetrics{
		WindowStart:      time.Now(),
		WindowEnd:        time.Now().Add(windowDuration),
		RevenueByProduct: make(map[string]float64),
		RevenueByCountry: make(map[string]float64),
	}

	return stream.MapOperator(func(event *stream.Event) (*stream.Event, error) {
		var data map[string]interface{}
		if err := json.Unmarshal(event.Data, &data); err != nil {
			return nil, fmt.Errorf("failed to parse event data: %w", err)
		}

		// Check if this is a revenue event
		action, _ := data["action"].(string)
		if action != "purchase" {
			return nil, nil // Filter non-purchase events
		}

		// Extract revenue information
		amount, _ := data["amount"].(float64)
		product, _ := data["product"].(string)
		country, _ := data["country"].(string)

		// Update metrics
		metrics.TotalRevenue += amount
		metrics.TotalTransactions++
		metrics.RevenueByProduct[product] += amount
		metrics.RevenueByCountry[country] += amount

		if metrics.TotalTransactions > 0 {
			metrics.AverageOrderValue = metrics.TotalRevenue / float64(metrics.TotalTransactions)
		}

		windowDuration := event.Timestamp.Sub(metrics.WindowStart).Minutes()
		if windowDuration > 0 {
			metrics.RevenuePerMinute = metrics.TotalRevenue / windowDuration
		}

		metricsData, err := json.Marshal(metrics)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal revenue metrics: %w", err)
		}

		return &stream.Event{
			Key:       []byte("revenue"),
			Data:      metricsData,
			Timestamp: event.Timestamp,
			Headers:   event.Headers,
			Partition: event.Partition,
			Offset:    event.Offset,
		}, nil
	})
}

// ABTestAnalyzer analyzes A/B test performance
type ABTestAnalyzer struct {
	TestID      string                 `json:"test_id"`
	Variants    map[string]*VariantStats `json:"variants"`
	WindowStart time.Time              `json:"window_start"`
	WindowEnd   time.Time              `json:"window_end"`
}

// VariantStats holds statistics for an A/B test variant
type VariantStats struct {
	VariantID       string  `json:"variant_id"`
	Users           int     `json:"users"`
	Conversions     int     `json:"conversions"`
	ConversionRate  float64 `json:"conversion_rate"`
	Revenue         float64 `json:"revenue"`
	RevenuePerUser  float64 `json:"revenue_per_user"`
	AvgEngagement   float64 `json:"avg_engagement"`
	StatSignificance float64 `json:"stat_significance"` // p-value
}

// ABTestKPICalculator creates an operator for A/B test KPI calculation
func ABTestKPICalculator(testID string) stream.Operator {
	analyzer := &ABTestAnalyzer{
		TestID:      testID,
		Variants:    make(map[string]*VariantStats),
		WindowStart: time.Now(),
	}

	return stream.MapOperator(func(event *stream.Event) (*stream.Event, error) {
		var data map[string]interface{}
		if err := json.Unmarshal(event.Data, &data); err != nil {
			return nil, fmt.Errorf("failed to parse event data: %w", err)
		}

		variantID, ok := data["variant_id"].(string)
		if !ok {
			return nil, nil // Not an A/B test event
		}

		stats, exists := analyzer.Variants[variantID]
		if !exists {
			stats = &VariantStats{
				VariantID: variantID,
			}
			analyzer.Variants[variantID] = stats
		}

		// Update stats
		stats.Users++

		if action, _ := data["action"].(string); isConversionAction(action) {
			stats.Conversions++
		}

		if amount, ok := data["amount"].(float64); ok {
			stats.Revenue += amount
		}

		// Calculate rates
		if stats.Users > 0 {
			stats.ConversionRate = float64(stats.Conversions) / float64(stats.Users) * 100.0
			stats.RevenuePerUser = stats.Revenue / float64(stats.Users)
		}

		// Calculate statistical significance between variants
		if len(analyzer.Variants) >= 2 {
			calculateStatisticalSignificance(analyzer.Variants)
		}

		analyzerData, err := json.Marshal(analyzer)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal A/B test analyzer: %w", err)
		}

		return &stream.Event{
			Key:       []byte(testID),
			Data:      analyzerData,
			Timestamp: event.Timestamp,
			Headers:   event.Headers,
			Partition: event.Partition,
			Offset:    event.Offset,
		}, nil
	})
}

// calculateStatisticalSignificance computes p-values for variant comparison
func calculateStatisticalSignificance(variants map[string]*VariantStats) {
	// Simplified chi-square test for conversion rates
	// In production, use a proper statistical library
	variantList := make([]*VariantStats, 0, len(variants))
	for _, v := range variants {
		variantList = append(variantList, v)
	}

	if len(variantList) < 2 {
		return
	}

	// Compare first two variants (simplified)
	v1 := variantList[0]
	v2 := variantList[1]

	if v1.Users == 0 || v2.Users == 0 {
		return
	}

	// Calculate pooled probability
	pooledP := float64(v1.Conversions+v2.Conversions) / float64(v1.Users+v2.Users)

	// Calculate standard error
	se := math.Sqrt(pooledP * (1 - pooledP) * (1/float64(v1.Users) + 1/float64(v2.Users)))

	if se == 0 {
		return
	}

	// Calculate z-score
	zScore := math.Abs((v1.ConversionRate/100.0 - v2.ConversionRate/100.0) / se)

	// Simplified p-value (would use proper statistical distribution in production)
	pValue := 2.0 * (1.0 - normalCDF(zScore))

	v1.StatSignificance = pValue
	v2.StatSignificance = pValue
}

// normalCDF approximates the cumulative distribution function of the standard normal distribution
func normalCDF(x float64) float64 {
	return 0.5 * (1.0 + math.Erf(x/math.Sqrt(2.0)))
}

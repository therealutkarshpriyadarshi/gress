package join

import (
	"fmt"
	"sync"
	"time"

	"github.com/therealutkarshpriyadarshi/gress/pkg/stream"
	"go.uber.org/zap"
)

// TemporalJoinOperator joins a stream with a versioned reference data stream
// It maintains a versioned table and looks up the version valid at event time
type TemporalJoinOperator struct {
	config         *JoinConfig
	versionedTable map[string]*VersionHistory // key -> version history
	condition      JoinCondition
	logger         *zap.Logger
	mu             sync.RWMutex
	cleanupTicker  *time.Ticker
	cleanupStop    chan struct{}
}

// VersionHistory maintains the temporal history of a key
type VersionHistory struct {
	Key      string
	Versions []*Version
}

// Version represents a version of reference data valid during a time range
type Version struct {
	Event     *stream.Event
	ValidFrom time.Time
	ValidTo   time.Time // Set when a newer version arrives
}

// NewTemporalJoinOperator creates a temporal join operator
func NewTemporalJoinOperator(config *JoinConfig, logger *zap.Logger) *TemporalJoinOperator {
	if config == nil {
		config = DefaultJoinConfig()
	}
	if config.Condition == nil {
		config.Condition = EqualityCondition()
	}

	tj := &TemporalJoinOperator{
		config:         config,
		versionedTable: make(map[string]*VersionHistory),
		condition:      config.Condition,
		logger:         logger,
		cleanupStop:    make(chan struct{}),
	}

	// Start cleanup if enabled
	if config.EnableCleanup {
		tj.startCleanup()
	}

	return tj
}

// UpdateReference updates the reference data (right side / temporal table)
// This should be called for events from the reference stream
func (tj *TemporalJoinOperator) UpdateReference(event *stream.Event) error {
	tj.mu.Lock()
	defer tj.mu.Unlock()

	key := event.Key
	validFrom := event.EventTime

	// Get or create version history
	history, exists := tj.versionedTable[key]
	if !exists {
		history = &VersionHistory{
			Key:      key,
			Versions: make([]*Version, 0),
		}
		tj.versionedTable[key] = history
	}

	// Set ValidTo for the previous version
	if len(history.Versions) > 0 {
		lastVersion := history.Versions[len(history.Versions)-1]
		lastVersion.ValidTo = validFrom
	}

	// Add new version
	newVersion := &Version{
		Event:     event,
		ValidFrom: validFrom,
		ValidTo:   time.Time{}, // Open-ended until next version
	}
	history.Versions = append(history.Versions, newVersion)

	tj.logger.Debug("Reference data updated",
		zap.String("key", key),
		zap.Time("valid_from", validFrom),
		zap.Int("total_versions", len(history.Versions)))

	return nil
}

// ProcessLeft processes an event from the main stream (left side)
// It looks up the reference data version valid at the event's timestamp
func (tj *TemporalJoinOperator) ProcessLeft(ctx *stream.ProcessingContext, event *stream.Event) ([]*stream.Event, error) {
	tj.mu.RLock()
	defer tj.mu.RUnlock()

	lookupKey := event.Key
	lookupTime := event.EventTime

	// Look up the version valid at the event time
	referenceEvent := tj.lookupVersion(lookupKey, lookupTime)

	var results []*stream.Event

	if referenceEvent != nil {
		// Check custom condition
		if tj.condition(event, referenceEvent) {
			result := &JoinResult{
				Left:      event,
				Right:     referenceEvent,
				JoinKey:   event.Key,
				EventTime: event.EventTime,
			}
			results = append(results, result.ToEvent())
		}
	} else {
		// No matching reference data found
		if tj.config.Type == LeftOuterJoin || tj.config.Type == FullOuterJoin {
			result := &JoinResult{
				Left:      event,
				Right:     nil,
				JoinKey:   event.Key,
				EventTime: event.EventTime,
			}
			results = append(results, result.ToEvent())
		}
	}

	return results, nil
}

// lookupVersion finds the version valid at the given time
func (tj *TemporalJoinOperator) lookupVersion(key string, timestamp time.Time) *stream.Event {
	history, exists := tj.versionedTable[key]
	if !exists {
		return nil
	}

	// Binary search for efficiency (versions are ordered by ValidFrom)
	left, right := 0, len(history.Versions)-1
	var result *Version

	for left <= right {
		mid := (left + right) / 2
		version := history.Versions[mid]

		// Check if timestamp falls within this version's validity range
		if (timestamp.After(version.ValidFrom) || timestamp.Equal(version.ValidFrom)) &&
			(version.ValidTo.IsZero() || timestamp.Before(version.ValidTo)) {
			result = version
			break
		}

		// Navigate search space
		if timestamp.Before(version.ValidFrom) {
			right = mid - 1
		} else {
			left = mid + 1
		}
	}

	// Fallback to linear search if binary search didn't find it
	// (This can happen with overlapping or out-of-order versions)
	if result == nil {
		for _, version := range history.Versions {
			if (timestamp.After(version.ValidFrom) || timestamp.Equal(version.ValidFrom)) &&
				(version.ValidTo.IsZero() || timestamp.Before(version.ValidTo)) {
				result = version
				break
			}
		}
	}

	if result != nil {
		return result.Event
	}
	return nil
}

// GetVersionHistory returns the version history for a key
func (tj *TemporalJoinOperator) GetVersionHistory(key string) (*VersionHistory, error) {
	tj.mu.RLock()
	defer tj.mu.RUnlock()

	history, exists := tj.versionedTable[key]
	if !exists {
		return nil, fmt.Errorf("no version history found for key: %s", key)
	}

	// Return a copy to prevent external modifications
	historyCopy := &VersionHistory{
		Key:      history.Key,
		Versions: make([]*Version, len(history.Versions)),
	}
	copy(historyCopy.Versions, history.Versions)

	return historyCopy, nil
}

// GetAllKeys returns all keys in the versioned table
func (tj *TemporalJoinOperator) GetAllKeys() []string {
	tj.mu.RLock()
	defer tj.mu.RUnlock()

	keys := make([]string, 0, len(tj.versionedTable))
	for key := range tj.versionedTable {
		keys = append(keys, key)
	}
	return keys
}

// startCleanup starts periodic cleanup of old versions
func (tj *TemporalJoinOperator) startCleanup() {
	tj.cleanupTicker = time.NewTicker(tj.config.CleanupInterval)

	go func() {
		for {
			select {
			case <-tj.cleanupTicker.C:
				tj.cleanup()
			case <-tj.cleanupStop:
				return
			}
		}
	}()
}

// cleanup removes old versions beyond the retention period
func (tj *TemporalJoinOperator) cleanup() {
	tj.mu.Lock()
	defer tj.mu.Unlock()

	now := time.Now()
	cutoffTime := now.Add(-tj.config.StateRetention)
	removedCount := 0

	for key, history := range tj.versionedTable {
		// Keep versions that are still within retention or are the latest
		var keptVersions []*Version

		for i, version := range history.Versions {
			isLatest := i == len(history.Versions)-1
			isWithinRetention := version.ValidTo.IsZero() || version.ValidTo.After(cutoffTime)

			if isLatest || isWithinRetention {
				keptVersions = append(keptVersions, version)
			} else {
				removedCount++
			}
		}

		if len(keptVersions) == 0 {
			// Remove key entirely if no versions remain
			delete(tj.versionedTable, key)
		} else {
			history.Versions = keptVersions
		}
	}

	if removedCount > 0 {
		tj.logger.Debug("Temporal join cleanup completed",
			zap.Int("removed_versions", removedCount),
			zap.Int("remaining_keys", len(tj.versionedTable)))
	}
}

// Process implements the Operator interface
func (tj *TemporalJoinOperator) Process(ctx *stream.ProcessingContext, event *stream.Event) ([]*stream.Event, error) {
	return nil, fmt.Errorf("temporal join operator requires explicit ProcessLeft or UpdateReference calls")
}

// Close cleans up resources
func (tj *TemporalJoinOperator) Close() error {
	if tj.cleanupTicker != nil {
		tj.cleanupTicker.Stop()
	}
	if tj.cleanupStop != nil {
		close(tj.cleanupStop)
	}

	tj.mu.Lock()
	defer tj.mu.Unlock()
	tj.versionedTable = nil

	return nil
}

// GetMetrics returns temporal join metrics
func (tj *TemporalJoinOperator) GetMetrics() TemporalJoinMetrics {
	tj.mu.RLock()
	defer tj.mu.RUnlock()

	metrics := TemporalJoinMetrics{
		TotalKeys: len(tj.versionedTable),
	}

	for _, history := range tj.versionedTable {
		metrics.TotalVersions += len(history.Versions)
	}

	return metrics
}

// TemporalJoinMetrics tracks temporal join metrics
type TemporalJoinMetrics struct {
	TotalKeys     int
	TotalVersions int
}

// TemporalJoinBuilder helps construct temporal joins with fluent API
type TemporalJoinBuilder struct {
	config *JoinConfig
	logger *zap.Logger
}

// NewTemporalJoin creates a new temporal join builder
func NewTemporalJoin() *TemporalJoinBuilder {
	config := DefaultJoinConfig()
	// Temporal joins are typically left outer joins
	config.Type = LeftOuterJoin
	return &TemporalJoinBuilder{
		config: config,
	}
}

// WithJoinType sets the join type
func (b *TemporalJoinBuilder) WithJoinType(joinType JoinType) *TemporalJoinBuilder {
	b.config.Type = joinType
	return b
}

// WithCondition sets the join condition
func (b *TemporalJoinBuilder) WithCondition(condition JoinCondition) *TemporalJoinBuilder {
	b.config.Condition = condition
	return b
}

// WithStateRetention sets how long to keep old versions
func (b *TemporalJoinBuilder) WithStateRetention(retention time.Duration) *TemporalJoinBuilder {
	b.config.StateRetention = retention
	return b
}

// WithLogger sets the logger
func (b *TemporalJoinBuilder) WithLogger(logger *zap.Logger) *TemporalJoinBuilder {
	b.logger = logger
	return b
}

// Build creates the temporal join operator
func (b *TemporalJoinBuilder) Build() *TemporalJoinOperator {
	if b.logger == nil {
		b.logger = zap.NewNop()
	}
	return NewTemporalJoinOperator(b.config, b.logger)
}

// Example usage:
//
// Join orders with the latest product catalog at order time:
//   temporalJoin := NewTemporalJoin().
//     WithStateRetention(24 * time.Hour).
//     Build()
//
//   // Update product catalog
//   temporalJoin.UpdateReference(productEvent)
//
//   // Process order with product data valid at order time
//   results, _ := temporalJoin.ProcessLeft(ctx, orderEvent)

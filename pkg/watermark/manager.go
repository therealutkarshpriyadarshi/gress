package watermark

import (
	"context"
	"sync"
	"time"

	"github.com/therealutkarshpriyadarshi/gress/pkg/stream"
	"go.uber.org/zap"
)

// Manager tracks and generates watermarks for stream processing
type Manager struct {
	mu               sync.RWMutex
	partitionMarks   map[int32]time.Time // Per-partition watermarks
	globalWatermark  time.Time           // Global (minimum) watermark
	idleTimeout      time.Duration       // Time before partition considered idle
	lastEventTime    map[int32]time.Time // Last event time per partition
	interval         time.Duration
	logger           *zap.Logger
}

// NewManager creates a new watermark manager
func NewManager(interval time.Duration, logger *zap.Logger) *Manager {
	return &Manager{
		partitionMarks:  make(map[int32]time.Time),
		lastEventTime:   make(map[int32]time.Time),
		interval:        interval,
		idleTimeout:     30 * time.Second,
		globalWatermark: time.Time{},
		logger:          logger,
	}
}

// UpdateEventTime records an event timestamp for a partition
func (m *Manager) UpdateEventTime(partition int32, eventTime time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.lastEventTime[partition] = eventTime

	// Update partition watermark (conservative - subtract tolerance)
	watermarkTime := eventTime.Add(-5 * time.Second)
	if current, exists := m.partitionMarks[partition]; !exists || watermarkTime.After(current) {
		m.partitionMarks[partition] = watermarkTime
	}
}

// GetGlobalWatermark returns the current global watermark
func (m *Manager) GetGlobalWatermark() time.Time {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.globalWatermark
}

// GetPartitionWatermark returns the watermark for a specific partition
func (m *Manager) GetPartitionWatermark(partition int32) time.Time {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if wm, exists := m.partitionMarks[partition]; exists {
		return wm
	}
	return time.Time{}
}

// Start begins periodic watermark generation
func (m *Manager) Start(ctx context.Context, output chan<- *stream.Watermark) {
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			m.logger.Info("Watermark manager stopped")
			return
		case <-ticker.C:
			m.generateWatermarks(output)
		}
	}
}

// generateWatermarks computes and emits watermarks
func (m *Manager) generateWatermarks(output chan<- *stream.Watermark) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	var minWatermark time.Time
	hasActivePartitions := false

	// Compute minimum watermark across all partitions
	for partition, watermark := range m.partitionMarks {
		// Check if partition is idle
		lastEvent := m.lastEventTime[partition]
		if now.Sub(lastEvent) > m.idleTimeout {
			m.logger.Debug("Partition idle, advancing watermark",
				zap.Int32("partition", partition),
				zap.Duration("idle_time", now.Sub(lastEvent)))
			// Advance idle partition watermark to current time
			watermark = now.Add(-m.idleTimeout)
			m.partitionMarks[partition] = watermark
		}

		hasActivePartitions = true
		if minWatermark.IsZero() || watermark.Before(minWatermark) {
			minWatermark = watermark
		}

		// Emit per-partition watermark
		select {
		case output <- &stream.Watermark{
			Timestamp: watermark,
			Partition: partition,
		}:
		default:
			m.logger.Warn("Watermark channel full, skipping")
		}
	}

	// Update global watermark
	if hasActivePartitions && (m.globalWatermark.IsZero() || minWatermark.After(m.globalWatermark)) {
		m.globalWatermark = minWatermark
		m.logger.Debug("Global watermark advanced",
			zap.Time("watermark", m.globalWatermark))
	}
}

// IsLate determines if an event is late based on current watermark
func (m *Manager) IsLate(eventTime time.Time, partition int32) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if wm, exists := m.partitionMarks[partition]; exists {
		return eventTime.Before(wm)
	}
	return false
}

// AllowedLateness represents acceptable lateness for events
type AllowedLateness time.Duration

// LatenessHandler handles late-arriving events
type LatenessHandler interface {
	HandleLateEvent(event *stream.Event, watermark time.Time) error
}

// DropLateEvents is a handler that drops late events
type DropLateEvents struct {
	logger *zap.Logger
}

func (d *DropLateEvents) HandleLateEvent(event *stream.Event, watermark time.Time) error {
	d.logger.Warn("Dropping late event",
		zap.Time("event_time", event.EventTime),
		zap.Time("watermark", watermark),
		zap.Duration("lateness", watermark.Sub(event.EventTime)))
	return nil
}

// SideOutputLateEvents sends late events to a separate channel
type SideOutputLateEvents struct {
	output chan<- *stream.Event
	logger *zap.Logger
}

func (s *SideOutputLateEvents) HandleLateEvent(event *stream.Event, watermark time.Time) error {
	s.logger.Info("Routing late event to side output",
		zap.Time("event_time", event.EventTime),
		zap.Time("watermark", watermark))
	select {
	case s.output <- event:
		return nil
	default:
		return nil
	}
}

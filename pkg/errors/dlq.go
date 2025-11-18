package errors

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// FailedEvent represents an event that failed processing
type FailedEvent struct {
	// Original event data
	Key       string                 `json:"key"`
	Value     interface{}            `json:"value"`
	EventTime time.Time              `json:"event_time"`
	Headers   map[string]string      `json:"headers"`
	Offset    int64                  `json:"offset"`
	Partition int32                  `json:"partition"`

	// Failure metadata
	FailureReason   string            `json:"failure_reason"`
	FailureCategory string            `json:"failure_category"`
	OperatorName    string            `json:"operator_name"`
	FailureTime     time.Time         `json:"failure_time"`
	RetryCount      int               `json:"retry_count"`
	StackTrace      string            `json:"stack_trace,omitempty"`
	Metadata        map[string]string `json:"metadata,omitempty"`
}

// DeadLetterQueue handles failed events
type DeadLetterQueue interface {
	// Write writes a failed event to the DLQ
	Write(ctx context.Context, event *FailedEvent) error
	// Read reads failed events from the DLQ (for replay)
	Read(ctx context.Context, limit int) ([]*FailedEvent, error)
	// Delete removes a failed event from the DLQ
	Delete(ctx context.Context, key string, offset int64) error
	// Count returns the number of events in the DLQ
	Count(ctx context.Context) (int64, error)
	// Close closes the DLQ
	Close() error
}

// InMemoryDLQ is an in-memory implementation of DeadLetterQueue
type InMemoryDLQ struct {
	mu     sync.RWMutex
	events []*FailedEvent
	maxSize int
}

// NewInMemoryDLQ creates a new in-memory DLQ
func NewInMemoryDLQ(maxSize int) *InMemoryDLQ {
	return &InMemoryDLQ{
		events:  make([]*FailedEvent, 0),
		maxSize: maxSize,
	}
}

// Write writes a failed event to the DLQ
func (dlq *InMemoryDLQ) Write(ctx context.Context, event *FailedEvent) error {
	dlq.mu.Lock()
	defer dlq.mu.Unlock()

	// Check if we've reached max size
	if dlq.maxSize > 0 && len(dlq.events) >= dlq.maxSize {
		// Remove oldest event (FIFO)
		dlq.events = dlq.events[1:]
	}

	dlq.events = append(dlq.events, event)
	return nil
}

// Read reads failed events from the DLQ
func (dlq *InMemoryDLQ) Read(ctx context.Context, limit int) ([]*FailedEvent, error) {
	dlq.mu.RLock()
	defer dlq.mu.RUnlock()

	if limit <= 0 || limit > len(dlq.events) {
		limit = len(dlq.events)
	}

	result := make([]*FailedEvent, limit)
	copy(result, dlq.events[:limit])
	return result, nil
}

// Delete removes a failed event from the DLQ
func (dlq *InMemoryDLQ) Delete(ctx context.Context, key string, offset int64) error {
	dlq.mu.Lock()
	defer dlq.mu.Unlock()

	for i, event := range dlq.events {
		if event.Key == key && event.Offset == offset {
			// Remove event
			dlq.events = append(dlq.events[:i], dlq.events[i+1:]...)
			return nil
		}
	}

	return fmt.Errorf("event not found: key=%s, offset=%d", key, offset)
}

// Count returns the number of events in the DLQ
func (dlq *InMemoryDLQ) Count(ctx context.Context) (int64, error) {
	dlq.mu.RLock()
	defer dlq.mu.RUnlock()
	return int64(len(dlq.events)), nil
}

// Close closes the DLQ
func (dlq *InMemoryDLQ) Close() error {
	dlq.mu.Lock()
	defer dlq.mu.Unlock()
	dlq.events = nil
	return nil
}

// GetAll returns all events (for testing)
func (dlq *InMemoryDLQ) GetAll() []*FailedEvent {
	dlq.mu.RLock()
	defer dlq.mu.RUnlock()
	result := make([]*FailedEvent, len(dlq.events))
	copy(result, dlq.events)
	return result
}

// FileDLQ writes failed events to files on disk
type FileDLQ struct {
	mu        sync.Mutex
	directory string
	encoder   *json.Encoder
	file      interface{} // File handle (abstracted for portability)
}

// NewFileDLQ creates a new file-based DLQ
func NewFileDLQ(directory string) (*FileDLQ, error) {
	// Note: Actual file I/O implementation would go here
	// For now, this is a placeholder structure
	return &FileDLQ{
		directory: directory,
	}, nil
}

// Write writes a failed event to a file
func (dlq *FileDLQ) Write(ctx context.Context, event *FailedEvent) error {
	dlq.mu.Lock()
	defer dlq.mu.Unlock()

	// Serialize event to JSON
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// In a real implementation, we would write to a file
	// For now, this is a placeholder
	_ = data

	return nil
}

// Read reads failed events from files
func (dlq *FileDLQ) Read(ctx context.Context, limit int) ([]*FailedEvent, error) {
	// Placeholder implementation
	return nil, nil
}

// Delete removes a failed event file
func (dlq *FileDLQ) Delete(ctx context.Context, key string, offset int64) error {
	// Placeholder implementation
	return nil
}

// Count returns the number of events in the DLQ
func (dlq *FileDLQ) Count(ctx context.Context) (int64, error) {
	// Placeholder implementation
	return 0, nil
}

// Close closes the DLQ
func (dlq *FileDLQ) Close() error {
	return nil
}

// DLQStats tracks DLQ statistics
type DLQStats struct {
	TotalWritten  int64
	TotalRead     int64
	TotalDeleted  int64
	CurrentSize   int64
	OldestEvent   *time.Time
	NewestEvent   *time.Time
}

// DLQManager manages a DLQ with statistics
type DLQManager struct {
	dlq   DeadLetterQueue
	stats *DLQStats
	mu    sync.RWMutex
}

// NewDLQManager creates a new DLQ manager
func NewDLQManager(dlq DeadLetterQueue) *DLQManager {
	return &DLQManager{
		dlq: dlq,
		stats: &DLQStats{},
	}
}

// Write writes a failed event and updates statistics
func (m *DLQManager) Write(ctx context.Context, event *FailedEvent) error {
	err := m.dlq.Write(ctx, event)
	if err != nil {
		return err
	}

	m.mu.Lock()
	m.stats.TotalWritten++
	m.stats.CurrentSize++
	if m.stats.NewestEvent == nil || event.FailureTime.After(*m.stats.NewestEvent) {
		m.stats.NewestEvent = &event.FailureTime
	}
	if m.stats.OldestEvent == nil || event.FailureTime.Before(*m.stats.OldestEvent) {
		m.stats.OldestEvent = &event.FailureTime
	}
	m.mu.Unlock()

	return nil
}

// Read reads failed events
func (m *DLQManager) Read(ctx context.Context, limit int) ([]*FailedEvent, error) {
	events, err := m.dlq.Read(ctx, limit)
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	m.stats.TotalRead += int64(len(events))
	m.mu.Unlock()

	return events, nil
}

// Delete deletes a failed event
func (m *DLQManager) Delete(ctx context.Context, key string, offset int64) error {
	err := m.dlq.Delete(ctx, key, offset)
	if err != nil {
		return err
	}

	m.mu.Lock()
	m.stats.TotalDeleted++
	if m.stats.CurrentSize > 0 {
		m.stats.CurrentSize--
	}
	m.mu.Unlock()

	return nil
}

// Count returns the number of events in the DLQ
func (m *DLQManager) Count(ctx context.Context) (int64, error) {
	return m.dlq.Count(ctx)
}

// Stats returns DLQ statistics
func (m *DLQManager) Stats() DLQStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Create a copy to avoid race conditions
	stats := *m.stats
	return stats
}

// Close closes the DLQ
func (m *DLQManager) Close() error {
	return m.dlq.Close()
}

// DLQ returns the underlying DLQ
func (m *DLQManager) DLQ() DeadLetterQueue {
	return m.dlq
}

// NullDLQ is a no-op DLQ that discards all events
type NullDLQ struct{}

// NewNullDLQ creates a new null DLQ
func NewNullDLQ() *NullDLQ {
	return &NullDLQ{}
}

// Write discards the event
func (dlq *NullDLQ) Write(ctx context.Context, event *FailedEvent) error {
	return nil
}

// Read returns empty slice
func (dlq *NullDLQ) Read(ctx context.Context, limit int) ([]*FailedEvent, error) {
	return []*FailedEvent{}, nil
}

// Delete is a no-op
func (dlq *NullDLQ) Delete(ctx context.Context, key string, offset int64) error {
	return nil
}

// Count returns 0
func (dlq *NullDLQ) Count(ctx context.Context) (int64, error) {
	return 0, nil
}

// Close is a no-op
func (dlq *NullDLQ) Close() error {
	return nil
}

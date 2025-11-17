package state

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/therealutkarshpriyadarshi/gress/pkg/stream"
	"go.uber.org/zap"
)

// MemoryStateBackend provides in-memory state storage for testing and small workloads
type MemoryStateBackend struct {
	data       map[string]valueWithTTL
	mu         sync.RWMutex
	logger     *zap.Logger
	ttlEnabled bool
	defaultTTL time.Duration

	// Metrics
	getCount    int64
	putCount    int64
	deleteCount int64
}

type valueWithTTL struct {
	data       []byte
	expiryTime time.Time
}

// MemoryStateConfig holds configuration for memory backend
type MemoryStateConfig struct {
	TTLEnabled bool
	DefaultTTL time.Duration
}

// DefaultMemoryConfig returns default configuration
func DefaultMemoryConfig() *MemoryStateConfig {
	return &MemoryStateConfig{
		TTLEnabled: false,
		DefaultTTL: 0,
	}
}

// NewMemoryStateBackend creates a new in-memory state backend
func NewMemoryStateBackend(config *MemoryStateConfig, logger *zap.Logger) *MemoryStateBackend {
	if config == nil {
		config = DefaultMemoryConfig()
	}

	backend := &MemoryStateBackend{
		data:       make(map[string]valueWithTTL),
		logger:     logger,
		ttlEnabled: config.TTLEnabled,
		defaultTTL: config.DefaultTTL,
	}

	// Start TTL cleanup goroutine if enabled
	if config.TTLEnabled {
		go backend.cleanupExpired()
	}

	logger.Info("Memory state backend initialized",
		zap.Bool("ttl_enabled", config.TTLEnabled),
	)

	return backend
}

// Get retrieves a value from the state backend
func (m *MemoryStateBackend) Get(key string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	value, exists := m.data[key]
	if !exists {
		return nil, nil
	}

	// Check TTL if enabled
	if m.ttlEnabled && !value.expiryTime.IsZero() {
		if time.Now().After(value.expiryTime) {
			// Key has expired
			go m.Delete(key)
			return nil, nil
		}
	}

	m.getCount++

	// Return a copy to prevent external modifications
	result := make([]byte, len(value.data))
	copy(result, value.data)

	return result, nil
}

// Put stores a value in the state backend
func (m *MemoryStateBackend) Put(key string, value []byte) error {
	return m.PutWithTTL(key, value, m.defaultTTL)
}

// PutWithTTL stores a value with a custom TTL
func (m *MemoryStateBackend) PutWithTTL(key string, value []byte, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Make a copy to prevent external modifications
	data := make([]byte, len(value))
	copy(data, value)

	entry := valueWithTTL{
		data: data,
	}

	if m.ttlEnabled && ttl > 0 {
		entry.expiryTime = time.Now().Add(ttl)
	}

	m.data[key] = entry
	m.putCount++

	return nil
}

// Delete removes a key from the state backend
func (m *MemoryStateBackend) Delete(key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.data, key)
	m.deleteCount++

	return nil
}

// Snapshot creates a snapshot of the current state
func (m *MemoryStateBackend) Snapshot() ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	snapshot := make(map[string][]byte, len(m.data))

	for key, value := range m.data {
		// Skip expired entries
		if m.ttlEnabled && !value.expiryTime.IsZero() {
			if time.Now().After(value.expiryTime) {
				continue
			}
		}

		// Copy the data
		data := make([]byte, len(value.data))
		copy(data, value.data)
		snapshot[key] = data
	}

	// Serialize to JSON
	data, err := json.Marshal(snapshot)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize snapshot: %w", err)
	}

	m.logger.Info("Created memory snapshot",
		zap.Int("keys", len(snapshot)),
		zap.Int("size_bytes", len(data)),
	)

	return data, nil
}

// Restore restores state from a snapshot
func (m *MemoryStateBackend) Restore(data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var snapshot map[string][]byte
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return fmt.Errorf("failed to deserialize snapshot: %w", err)
	}

	// Clear existing data
	m.data = make(map[string]valueWithTTL, len(snapshot))

	// Restore all key-value pairs
	for key, value := range snapshot {
		data := make([]byte, len(value))
		copy(data, value)
		m.data[key] = valueWithTTL{
			data: data,
		}
	}

	m.logger.Info("Restored memory snapshot",
		zap.Int("keys", len(snapshot)),
		zap.Int("size_bytes", len(data)),
	)

	return nil
}

// cleanupExpired periodically removes expired entries
func (m *MemoryStateBackend) cleanupExpired() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		m.mu.Lock()
		now := time.Now()
		expiredKeys := make([]string, 0)

		for key, value := range m.data {
			if !value.expiryTime.IsZero() && now.After(value.expiryTime) {
				expiredKeys = append(expiredKeys, key)
			}
		}

		for _, key := range expiredKeys {
			delete(m.data, key)
		}

		if len(expiredKeys) > 0 {
			m.logger.Debug("Cleaned up expired keys",
				zap.Int("count", len(expiredKeys)),
			)
		}

		m.mu.Unlock()
	}
}

// GetMetrics returns current operation metrics
func (m *MemoryStateBackend) GetMetrics() map[string]int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return map[string]int64{
		"gets":    m.getCount,
		"puts":    m.putCount,
		"deletes": m.deleteCount,
		"keys":    int64(len(m.data)),
	}
}

// Size returns the number of keys in the backend
func (m *MemoryStateBackend) Size() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.data)
}

// Clear removes all data from the backend
func (m *MemoryStateBackend) Clear() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.data = make(map[string]valueWithTTL)
	return nil
}

// Close closes the memory backend
func (m *MemoryStateBackend) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.data = nil

	m.logger.Info("Closed memory state backend",
		zap.Int64("total_gets", m.getCount),
		zap.Int64("total_puts", m.putCount),
		zap.Int64("total_deletes", m.deleteCount),
	)

	return nil
}

// Verify interface implementation
var _ stream.StateBackend = (*MemoryStateBackend)(nil)

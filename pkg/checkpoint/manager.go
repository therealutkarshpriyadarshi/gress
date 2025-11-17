package checkpoint

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/therealutkarshpriyadarshi/gress/pkg/stream"
	"go.uber.org/zap"
)

// Manager coordinates checkpoint creation and recovery
type Manager struct {
	mu              sync.RWMutex
	interval        time.Duration
	checkpointDir   string
	maxCheckpoints  int
	checkpoints     []*stream.Checkpoint
	logger          *zap.Logger
	stateProviders  []StateProvider
}

// StateProvider provides state for checkpointing
type StateProvider interface {
	GetState() (map[string]interface{}, error)
	RestoreState(state map[string]interface{}) error
}

// NewManager creates a new checkpoint manager
func NewManager(interval time.Duration, checkpointDir string, logger *zap.Logger) *Manager {
	if checkpointDir == "" {
		checkpointDir = "./checkpoints"
	}

	// Create checkpoint directory if it doesn't exist
	os.MkdirAll(checkpointDir, 0755)

	return &Manager{
		interval:       interval,
		checkpointDir:  checkpointDir,
		maxCheckpoints: 5, // Keep last 5 checkpoints
		checkpoints:    make([]*stream.Checkpoint, 0),
		logger:         logger,
		stateProviders: make([]StateProvider, 0),
	}
}

// RegisterStateProvider adds a state provider for checkpointing
func (m *Manager) RegisterStateProvider(provider StateProvider) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stateProviders = append(m.stateProviders, provider)
}

// CreateCheckpoint creates a new checkpoint
func (m *Manager) CreateCheckpoint(ctx context.Context, offsets map[int32]int64) (*stream.Checkpoint, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	checkpointID := fmt.Sprintf("checkpoint-%d", time.Now().UnixNano())
	m.logger.Info("Creating checkpoint", zap.String("id", checkpointID))

	// Collect state from all providers
	combinedState := make(map[string]interface{})
	for i, provider := range m.stateProviders {
		state, err := provider.GetState()
		if err != nil {
			return nil, fmt.Errorf("failed to get state from provider %d: %w", i, err)
		}
		for k, v := range state {
			combinedState[k] = v
		}
	}

	// Serialize state
	stateData, err := json.Marshal(combinedState)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize state: %w", err)
	}

	checkpoint := &stream.Checkpoint{
		ID:        checkpointID,
		Timestamp: time.Now(),
		Offsets:   offsets,
		StateData: stateData,
	}

	// Persist checkpoint to disk
	if err := m.persistCheckpoint(checkpoint); err != nil {
		return nil, fmt.Errorf("failed to persist checkpoint: %w", err)
	}

	// Add to in-memory list
	m.checkpoints = append(m.checkpoints, checkpoint)

	// Cleanup old checkpoints
	m.cleanupOldCheckpoints()

	m.logger.Info("Checkpoint created successfully",
		zap.String("id", checkpointID),
		zap.Int("state_size_bytes", len(stateData)))

	return checkpoint, nil
}

// persistCheckpoint saves a checkpoint to disk
func (m *Manager) persistCheckpoint(checkpoint *stream.Checkpoint) error {
	data, err := json.MarshalIndent(checkpoint, "", "  ")
	if err != nil {
		return err
	}

	filename := filepath.Join(m.checkpointDir, checkpoint.ID+".json")
	return os.WriteFile(filename, data, 0644)
}

// LoadLatestCheckpoint loads the most recent checkpoint
func (m *Manager) LoadLatestCheckpoint() (*stream.Checkpoint, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	files, err := filepath.Glob(filepath.Join(m.checkpointDir, "checkpoint-*.json"))
	if err != nil {
		return nil, err
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no checkpoints found")
	}

	// Find most recent checkpoint
	var latestFile string
	var latestTime time.Time

	for _, file := range files {
		info, err := os.Stat(file)
		if err != nil {
			continue
		}
		if info.ModTime().After(latestTime) {
			latestTime = info.ModTime()
			latestFile = file
		}
	}

	// Load checkpoint
	data, err := os.ReadFile(latestFile)
	if err != nil {
		return nil, err
	}

	var checkpoint stream.Checkpoint
	if err := json.Unmarshal(data, &checkpoint); err != nil {
		return nil, err
	}

	m.logger.Info("Loaded checkpoint",
		zap.String("id", checkpoint.ID),
		zap.Time("timestamp", checkpoint.Timestamp))

	return &checkpoint, nil
}

// RestoreFromCheckpoint restores state from a checkpoint
func (m *Manager) RestoreFromCheckpoint(checkpoint *stream.Checkpoint) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.logger.Info("Restoring from checkpoint", zap.String("id", checkpoint.ID))

	// Deserialize state
	var state map[string]interface{}
	if err := json.Unmarshal(checkpoint.StateData, &state); err != nil {
		return fmt.Errorf("failed to deserialize state: %w", err)
	}

	// Restore state to all providers
	for i, provider := range m.stateProviders {
		if err := provider.RestoreState(state); err != nil {
			return fmt.Errorf("failed to restore state to provider %d: %w", i, err)
		}
	}

	m.logger.Info("State restored successfully from checkpoint",
		zap.String("id", checkpoint.ID))

	return nil
}

// cleanupOldCheckpoints removes old checkpoints beyond maxCheckpoints
func (m *Manager) cleanupOldCheckpoints() {
	if len(m.checkpoints) <= m.maxCheckpoints {
		return
	}

	// Remove oldest checkpoints
	toRemove := m.checkpoints[:len(m.checkpoints)-m.maxCheckpoints]
	m.checkpoints = m.checkpoints[len(m.checkpoints)-m.maxCheckpoints:]

	// Delete files
	for _, cp := range toRemove {
		filename := filepath.Join(m.checkpointDir, cp.ID+".json")
		if err := os.Remove(filename); err != nil {
			m.logger.Warn("Failed to remove old checkpoint",
				zap.String("id", cp.ID),
				zap.Error(err))
		} else {
			m.logger.Debug("Removed old checkpoint", zap.String("id", cp.ID))
		}
	}
}

// GetCheckpoints returns all available checkpoints
func (m *Manager) GetCheckpoints() []*stream.Checkpoint {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]*stream.Checkpoint{}, m.checkpoints...)
}

// CoordinatedCheckpoint performs a two-phase commit checkpoint
type CoordinatedCheckpoint struct {
	barriers map[int32]bool // Track barrier alignment per partition
	mu       sync.Mutex
}

// NewCoordinatedCheckpoint creates a new coordinated checkpoint
func NewCoordinatedCheckpoint() *CoordinatedCheckpoint {
	return &CoordinatedCheckpoint{
		barriers: make(map[int32]bool),
	}
}

// AlignBarrier marks a partition as having reached the checkpoint barrier
func (c *CoordinatedCheckpoint) AlignBarrier(partition int32) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.barriers[partition] = true
}

// AllAligned checks if all partitions have reached the barrier
func (c *CoordinatedCheckpoint) AllAligned(partitions []int32) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, p := range partitions {
		if !c.barriers[p] {
			return false
		}
	}
	return true
}

// Reset clears all barrier markers
func (c *CoordinatedCheckpoint) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.barriers = make(map[int32]bool)
}

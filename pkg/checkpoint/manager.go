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
	mu                   sync.RWMutex
	interval             time.Duration
	checkpointDir        string
	maxCheckpoints       int
	checkpoints          []*stream.Checkpoint
	logger               *zap.Logger
	stateProviders       []StateProvider
	stateBackends        []stream.StateBackend
	incrementalEnabled   bool
	lastCheckpointData   map[string][]byte
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
		interval:           interval,
		checkpointDir:      checkpointDir,
		maxCheckpoints:     5, // Keep last 5 checkpoints
		checkpoints:        make([]*stream.Checkpoint, 0),
		logger:             logger,
		stateProviders:     make([]StateProvider, 0),
		stateBackends:      make([]stream.StateBackend, 0),
		incrementalEnabled: false,
		lastCheckpointData: make(map[string][]byte),
	}
}

// RegisterStateProvider adds a state provider for checkpointing
func (m *Manager) RegisterStateProvider(provider StateProvider) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stateProviders = append(m.stateProviders, provider)
}

// RegisterStateBackend adds a state backend for checkpointing
func (m *Manager) RegisterStateBackend(backend stream.StateBackend) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stateBackends = append(m.stateBackends, backend)
	m.logger.Info("Registered state backend for checkpointing")
}

// EnableIncrementalCheckpoints enables incremental checkpoint mode
func (m *Manager) EnableIncrementalCheckpoints(enabled bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.incrementalEnabled = enabled
	m.logger.Info("Incremental checkpointing enabled", zap.Bool("enabled", enabled))
}

// CreateCheckpoint creates a new checkpoint
func (m *Manager) CreateCheckpoint(ctx context.Context, offsets map[int32]int64) (*stream.Checkpoint, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	checkpointID := fmt.Sprintf("checkpoint-%d", time.Now().UnixNano())
	m.logger.Info("Creating checkpoint",
		zap.String("id", checkpointID),
		zap.Bool("incremental", m.incrementalEnabled))

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

	// Collect snapshots from state backends
	backendSnapshots := make(map[string][]byte)
	for i, backend := range m.stateBackends {
		snapshot, err := backend.Snapshot()
		if err != nil {
			return nil, fmt.Errorf("failed to snapshot backend %d: %w", i, err)
		}
		backendKey := fmt.Sprintf("backend-%d", i)
		backendSnapshots[backendKey] = snapshot
	}

	// Combine provider state and backend snapshots
	checkpointData := map[string]interface{}{
		"provider_state":    combinedState,
		"backend_snapshots": backendSnapshots,
	}

	// For incremental checkpoints, only include changed data
	if m.incrementalEnabled {
		checkpointData = m.computeIncrementalChanges(checkpointData, backendSnapshots)
	}

	// Serialize state
	stateData, err := json.Marshal(checkpointData)
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

	// Update last checkpoint data for incremental checkpoints
	if m.incrementalEnabled {
		m.lastCheckpointData = backendSnapshots
	}

	// Add to in-memory list
	m.checkpoints = append(m.checkpoints, checkpoint)

	// Cleanup old checkpoints
	m.cleanupOldCheckpoints()

	m.logger.Info("Checkpoint created successfully",
		zap.String("id", checkpointID),
		zap.Int("state_size_bytes", len(stateData)),
		zap.Int("backends", len(m.stateBackends)),
		zap.Int("providers", len(m.stateProviders)))

	return checkpoint, nil
}

// computeIncrementalChanges computes only the changes since last checkpoint
func (m *Manager) computeIncrementalChanges(fullData map[string]interface{}, currentSnapshots map[string][]byte) map[string]interface{} {
	if len(m.lastCheckpointData) == 0 {
		// First checkpoint, return full data
		return fullData
	}

	// For simplicity, we mark this as an incremental checkpoint
	// In a production system, you would compute actual diffs
	incrementalData := map[string]interface{}{
		"type":           "incremental",
		"provider_state": fullData["provider_state"],
		"changed_keys":   m.computeChangedKeys(currentSnapshots),
	}

	return incrementalData
}

// computeChangedKeys identifies which backend keys have changed
func (m *Manager) computeChangedKeys(currentSnapshots map[string][]byte) map[string]bool {
	changedKeys := make(map[string]bool)

	for key, currentData := range currentSnapshots {
		lastData, exists := m.lastCheckpointData[key]
		if !exists || !bytesEqual(lastData, currentData) {
			changedKeys[key] = true
		}
	}

	return changedKeys
}

// bytesEqual compares two byte slices
func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
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

	// Deserialize checkpoint data
	var checkpointData map[string]interface{}
	if err := json.Unmarshal(checkpoint.StateData, &checkpointData); err != nil {
		return fmt.Errorf("failed to deserialize state: %w", err)
	}

	// Restore provider state
	if providerState, ok := checkpointData["provider_state"].(map[string]interface{}); ok {
		for i, provider := range m.stateProviders {
			if err := provider.RestoreState(providerState); err != nil {
				return fmt.Errorf("failed to restore state to provider %d: %w", i, err)
			}
		}
	}

	// Restore backend snapshots
	if backendSnapshots, ok := checkpointData["backend_snapshots"].(map[string]interface{}); ok {
		for i, backend := range m.stateBackends {
			backendKey := fmt.Sprintf("backend-%d", i)
			if snapshotData, exists := backendSnapshots[backendKey]; exists {
				// Convert interface{} to []byte
				var snapshotBytes []byte
				switch v := snapshotData.(type) {
				case string:
					snapshotBytes = []byte(v)
				case []byte:
					snapshotBytes = v
				default:
					// Try to marshal it
					var err error
					snapshotBytes, err = json.Marshal(v)
					if err != nil {
						return fmt.Errorf("failed to convert snapshot data for backend %d: %w", i, err)
					}
				}

				if err := backend.Restore(snapshotBytes); err != nil {
					return fmt.Errorf("failed to restore backend %d: %w", i, err)
				}
			}
		}
	}

	m.logger.Info("State restored successfully from checkpoint",
		zap.String("id", checkpoint.ID),
		zap.Int("backends", len(m.stateBackends)),
		zap.Int("providers", len(m.stateProviders)))

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

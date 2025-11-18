package config

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"go.uber.org/zap"
)

// ReloadableConfig wraps a configuration with hot-reload capability
type ReloadableConfig struct {
	mu sync.RWMutex

	config   *Config
	path     string
	logger   *zap.Logger
	lastMod  time.Time
	interval time.Duration

	// Callback functions called when configuration is reloaded
	onReload []ReloadCallback

	// Critical settings that cannot be hot-reloaded
	criticalSettings CriticalSettings

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// ReloadCallback is called when configuration is reloaded
type ReloadCallback func(oldConfig, newConfig *Config) error

// CriticalSettings stores settings that cannot be changed during hot reload
type CriticalSettings struct {
	Version             string
	ApplicationName     string
	BufferSize          int
	MaxConcurrency      int
	CheckpointDir       string
	StateBackend        string
	RocksDBPath         string
}

// NewReloadableConfig creates a new reloadable configuration
func NewReloadableConfig(path string, logger *zap.Logger) (*ReloadableConfig, error) {
	if logger == nil {
		logger, _ = zap.NewProduction()
	}

	// Load initial configuration
	config, err := ValidateAndLoad(path)
	if err != nil {
		return nil, fmt.Errorf("failed to load initial configuration: %w", err)
	}

	// Get file modification time
	stat, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to stat config file: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	rc := &ReloadableConfig{
		config:   config,
		path:     path,
		logger:   logger,
		lastMod:  stat.ModTime(),
		interval: 10 * time.Second, // Check for changes every 10 seconds
		onReload: make([]ReloadCallback, 0),
		criticalSettings: CriticalSettings{
			Version:         config.Version,
			ApplicationName: config.Application.Name,
			BufferSize:      config.Engine.BufferSize,
			MaxConcurrency:  config.Engine.MaxConcurrency,
			CheckpointDir:   config.Engine.CheckpointDir,
			StateBackend:    config.State.Backend,
			RocksDBPath:     config.State.RocksDB.Path,
		},
		ctx:    ctx,
		cancel: cancel,
	}

	return rc, nil
}

// Get returns the current configuration (thread-safe)
func (rc *ReloadableConfig) Get() *Config {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	// Return a copy to prevent external modifications
	return copyConfig(rc.config)
}

// OnReload registers a callback to be called when configuration is reloaded
func (rc *ReloadableConfig) OnReload(callback ReloadCallback) {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	rc.onReload = append(rc.onReload, callback)
}

// SetReloadInterval sets the interval for checking configuration changes
func (rc *ReloadableConfig) SetReloadInterval(interval time.Duration) {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	rc.interval = interval
}

// Start begins watching for configuration file changes
func (rc *ReloadableConfig) Start() {
	rc.wg.Add(1)
	go rc.watchLoop()
}

// Stop stops watching for configuration changes
func (rc *ReloadableConfig) Stop() {
	rc.cancel()
	rc.wg.Wait()
}

// watchLoop periodically checks for configuration file changes
func (rc *ReloadableConfig) watchLoop() {
	defer rc.wg.Done()

	ticker := time.NewTicker(rc.interval)
	defer ticker.Stop()

	for {
		select {
		case <-rc.ctx.Done():
			rc.logger.Info("Configuration watcher stopped")
			return

		case <-ticker.C:
			if err := rc.checkAndReload(); err != nil {
				rc.logger.Error("Failed to reload configuration",
					zap.String("path", rc.path),
					zap.Error(err))
			}
		}
	}
}

// checkAndReload checks if the configuration file has changed and reloads it
func (rc *ReloadableConfig) checkAndReload() error {
	// Get current file modification time
	stat, err := os.Stat(rc.path)
	if err != nil {
		return fmt.Errorf("failed to stat config file: %w", err)
	}

	rc.mu.RLock()
	lastMod := rc.lastMod
	rc.mu.RUnlock()

	// Check if file has been modified
	if !stat.ModTime().After(lastMod) {
		return nil // No changes
	}

	rc.logger.Info("Configuration file changed, reloading...",
		zap.String("path", rc.path),
		zap.Time("last_modified", stat.ModTime()))

	// Load new configuration
	newConfig, err := ValidateAndLoad(rc.path)
	if err != nil {
		return fmt.Errorf("failed to load new configuration: %w", err)
	}

	// Validate that critical settings haven't changed
	if err := rc.validateCriticalSettings(newConfig); err != nil {
		return fmt.Errorf("critical settings changed (requires restart): %w", err)
	}

	// Get old config for callbacks
	rc.mu.RLock()
	oldConfig := rc.config
	rc.mu.RUnlock()

	// Call reload callbacks
	for _, callback := range rc.onReload {
		if err := callback(oldConfig, newConfig); err != nil {
			rc.logger.Error("Reload callback failed", zap.Error(err))
			return fmt.Errorf("reload callback failed: %w", err)
		}
	}

	// Update configuration
	rc.mu.Lock()
	rc.config = newConfig
	rc.lastMod = stat.ModTime()
	rc.mu.Unlock()

	rc.logger.Info("Configuration reloaded successfully",
		zap.String("path", rc.path))

	return nil
}

// validateCriticalSettings ensures critical settings haven't changed
func (rc *ReloadableConfig) validateCriticalSettings(newConfig *Config) error {
	cs := rc.criticalSettings

	if newConfig.Version != cs.Version {
		return fmt.Errorf("version changed from %s to %s", cs.Version, newConfig.Version)
	}

	if newConfig.Application.Name != cs.ApplicationName {
		return fmt.Errorf("application name changed from %s to %s", cs.ApplicationName, newConfig.Application.Name)
	}

	if newConfig.Engine.BufferSize != cs.BufferSize {
		return fmt.Errorf("buffer size changed from %d to %d", cs.BufferSize, newConfig.Engine.BufferSize)
	}

	if newConfig.Engine.MaxConcurrency != cs.MaxConcurrency {
		return fmt.Errorf("max concurrency changed from %d to %d", cs.MaxConcurrency, newConfig.Engine.MaxConcurrency)
	}

	if newConfig.Engine.CheckpointDir != cs.CheckpointDir {
		return fmt.Errorf("checkpoint directory changed from %s to %s", cs.CheckpointDir, newConfig.Engine.CheckpointDir)
	}

	if newConfig.State.Backend != cs.StateBackend {
		return fmt.Errorf("state backend changed from %s to %s", cs.StateBackend, newConfig.State.Backend)
	}

	if newConfig.State.Backend == "rocksdb" && newConfig.State.RocksDB.Path != cs.RocksDBPath {
		return fmt.Errorf("RocksDB path changed from %s to %s", cs.RocksDBPath, newConfig.State.RocksDB.Path)
	}

	return nil
}

// Reload manually triggers a configuration reload
func (rc *ReloadableConfig) Reload() error {
	return rc.checkAndReload()
}

// copyConfig creates a deep copy of the configuration
func copyConfig(src *Config) *Config {
	dst := *src

	// Copy maps
	dst.Application.Tags = make(map[string]string)
	for k, v := range src.Application.Tags {
		dst.Application.Tags[k] = v
	}

	// Copy slices
	dst.Sources.Kafka = append([]KafkaSourceConfig{}, src.Sources.Kafka...)
	dst.Sources.HTTP = append([]HTTPSourceConfig{}, src.Sources.HTTP...)
	dst.Sources.WebSocket = append([]WebSocketSourceConfig{}, src.Sources.WebSocket...)
	dst.Sinks.Kafka = append([]KafkaSinkConfig{}, src.Sinks.Kafka...)
	dst.Sinks.TimescaleDB = append([]TimescaleSinkConfig{}, src.Sinks.TimescaleDB...)
	dst.ErrorHandling.DLQKafkaBrokers = append([]string{}, src.ErrorHandling.DLQKafkaBrokers...)

	return &dst
}

// HotReloadableSettings returns a list of settings that can be hot-reloaded
func HotReloadableSettings() []string {
	return []string{
		"engine.checkpoint_interval",
		"engine.watermark_interval",
		"engine.metrics_interval",
		"engine.enable_backpressure",
		"engine.backpressure_threshold",
		"error_handling.strategy",
		"error_handling.enable_retry",
		"error_handling.max_retry_attempts",
		"error_handling.initial_backoff",
		"error_handling.max_backoff",
		"error_handling.backoff_multiplier",
		"error_handling.backoff_jitter",
		"error_handling.enable_circuit_breaker",
		"error_handling.circuit_breaker.*",
		"metrics.enabled",
		"metrics.address",
		"logging.level",
		"logging.format",
	}
}

// CriticalSettingsList returns a list of settings that require restart
func CriticalSettingsList() []string {
	return []string{
		"version",
		"application.name",
		"engine.buffer_size",
		"engine.max_concurrency",
		"engine.checkpoint_dir",
		"state.backend",
		"state.rocksdb.path",
		"sources.*",
		"sinks.*",
	}
}

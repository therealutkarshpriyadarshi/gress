package state

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/tecbot/gorocksdb"
	"github.com/therealutkarshpriyadarshi/gress/pkg/stream"
	"go.uber.org/zap"
)

// RocksDBStateBackend provides persistent state storage using RocksDB
type RocksDBStateBackend struct {
	db           *gorocksdb.DB
	opts         *gorocksdb.Options
	writeOpts    *gorocksdb.WriteOptions
	readOpts     *gorocksdb.ReadOptions
	checkpoint   *gorocksdb.Checkpoint
	logger       *zap.Logger
	mu           sync.RWMutex
	path         string
	ttlEnabled   bool
	defaultTTL   time.Duration
	lastSnapshot []byte

	// Metrics
	getCount     int64
	putCount     int64
	deleteCount  int64
}

// RocksDBConfig holds configuration for RocksDB backend
type RocksDBConfig struct {
	Path                string
	CreateIfMissing     bool
	ErrorIfExists       bool
	WriteBufferSize     int
	MaxWriteBufferNum   int
	MaxOpenFiles        int
	BlockSize           int
	BlockCacheSize      int64
	BloomFilterBits     int
	Compression         gorocksdb.CompressionType
	EnableStatistics    bool
	TTLEnabled          bool
	DefaultTTL          time.Duration
	EnableIncrementalCP bool
}

// DefaultRocksDBConfig returns sensible defaults for RocksDB
func DefaultRocksDBConfig(path string) *RocksDBConfig {
	return &RocksDBConfig{
		Path:                path,
		CreateIfMissing:     true,
		ErrorIfExists:       false,
		WriteBufferSize:     64 * 1024 * 1024, // 64MB
		MaxWriteBufferNum:   3,
		MaxOpenFiles:        1000,
		BlockSize:           4 * 1024,  // 4KB
		BlockCacheSize:      128 * 1024 * 1024, // 128MB
		BloomFilterBits:     10,
		Compression:         gorocksdb.SnappyCompression,
		EnableStatistics:    true,
		TTLEnabled:          false,
		DefaultTTL:          0,
		EnableIncrementalCP: true,
	}
}

// NewRocksDBStateBackend creates a new RocksDB state backend
func NewRocksDBStateBackend(config *RocksDBConfig, logger *zap.Logger) (*RocksDBStateBackend, error) {
	if config == nil {
		config = DefaultRocksDBConfig("./state")
	}

	opts := gorocksdb.NewDefaultOptions()
	opts.SetCreateIfMissing(config.CreateIfMissing)
	opts.SetErrorIfExists(config.ErrorIfExists)
	opts.SetWriteBufferSize(config.WriteBufferSize)
	opts.SetMaxWriteBufferNumber(config.MaxWriteBufferNum)
	opts.SetMaxOpenFiles(config.MaxOpenFiles)
	opts.SetCompression(config.Compression)

	// Block-based table options for better read performance
	blockOpts := gorocksdb.NewDefaultBlockBasedTableOptions()
	blockOpts.SetBlockSize(config.BlockSize)
	blockOpts.SetBlockCache(gorocksdb.NewLRUCache(config.BlockCacheSize))
	blockOpts.SetFilterPolicy(gorocksdb.NewBloomFilter(config.BloomFilterBits))
	opts.SetBlockBasedTableFactory(blockOpts)

	// Enable statistics for monitoring
	if config.EnableStatistics {
		opts.EnableStatistics()
	}

	// Optimize for point lookups and range scans
	opts.SetAllowConcurrentMemtableWrites(true)
	opts.SetEnablePipelinedWrite(true)

	db, err := gorocksdb.OpenDb(opts, config.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to open RocksDB: %w", err)
	}

	// Create checkpoint object for snapshots
	checkpoint, err := gorocksdb.NewCheckpoint(db)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create checkpoint: %w", err)
	}

	writeOpts := gorocksdb.NewDefaultWriteOptions()
	writeOpts.SetSync(false) // Async writes for better performance

	readOpts := gorocksdb.NewDefaultReadOptions()
	readOpts.SetFillCache(true)

	backend := &RocksDBStateBackend{
		db:         db,
		opts:       opts,
		writeOpts:  writeOpts,
		readOpts:   readOpts,
		checkpoint: checkpoint,
		logger:     logger,
		path:       config.Path,
		ttlEnabled: config.TTLEnabled,
		defaultTTL: config.DefaultTTL,
	}

	logger.Info("RocksDB state backend initialized",
		zap.String("path", config.Path),
		zap.Bool("ttl_enabled", config.TTLEnabled),
	)

	return backend, nil
}

// Get retrieves a value from the state backend
func (r *RocksDBStateBackend) Get(key string) ([]byte, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.db == nil {
		return nil, fmt.Errorf("RocksDB is not initialized")
	}

	value, err := r.db.Get(r.readOpts, []byte(key))
	if err != nil {
		return nil, fmt.Errorf("failed to get key %s: %w", key, err)
	}
	defer value.Free()

	if !value.Exists() {
		return nil, nil // Key not found
	}

	data := value.Data()

	// Check TTL if enabled
	if r.ttlEnabled && len(data) >= 8 {
		expiryTime := int64(binary.BigEndian.Uint64(data[:8]))
		if expiryTime > 0 && time.Now().Unix() > expiryTime {
			// Key has expired, delete it asynchronously
			go r.Delete(key)
			return nil, nil
		}
		data = data[8:] // Remove TTL prefix
	}

	r.getCount++

	// Make a copy since value.Data() is only valid while value exists
	result := make([]byte, len(data))
	copy(result, data)

	return result, nil
}

// Put stores a value in the state backend
func (r *RocksDBStateBackend) Put(key string, value []byte) error {
	return r.PutWithTTL(key, value, r.defaultTTL)
}

// PutWithTTL stores a value with a custom TTL
func (r *RocksDBStateBackend) PutWithTTL(key string, value []byte, ttl time.Duration) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.db == nil {
		return fmt.Errorf("RocksDB is not initialized")
	}

	var data []byte

	// Add TTL prefix if enabled
	if r.ttlEnabled && ttl > 0 {
		expiryTime := time.Now().Add(ttl).Unix()
		ttlPrefix := make([]byte, 8)
		binary.BigEndian.PutUint64(ttlPrefix, uint64(expiryTime))
		data = append(ttlPrefix, value...)
	} else {
		data = value
	}

	err := r.db.Put(r.writeOpts, []byte(key), data)
	if err != nil {
		return fmt.Errorf("failed to put key %s: %w", key, err)
	}

	r.putCount++
	return nil
}

// Delete removes a key from the state backend
func (r *RocksDBStateBackend) Delete(key string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.db == nil {
		return fmt.Errorf("RocksDB is not initialized")
	}

	err := r.db.Delete(r.writeOpts, []byte(key))
	if err != nil {
		return fmt.Errorf("failed to delete key %s: %w", key, err)
	}

	r.deleteCount++
	return nil
}

// Snapshot creates a full snapshot of the current state
func (r *RocksDBStateBackend) Snapshot() ([]byte, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.db == nil {
		return nil, fmt.Errorf("RocksDB is not initialized")
	}

	// Create an iterator to read all key-value pairs
	it := r.db.NewIterator(r.readOpts)
	defer it.Close()

	snapshot := make(map[string][]byte)

	for it.SeekToFirst(); it.Valid(); it.Next() {
		key := make([]byte, len(it.Key().Data()))
		copy(key, it.Key().Data())

		value := make([]byte, len(it.Value().Data()))
		copy(value, it.Value().Data())

		// Remove TTL prefix if present
		if r.ttlEnabled && len(value) >= 8 {
			value = value[8:]
		}

		snapshot[string(key)] = value
	}

	if err := it.Err(); err != nil {
		return nil, fmt.Errorf("iterator error during snapshot: %w", err)
	}

	// Serialize snapshot to JSON
	data, err := json.Marshal(snapshot)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize snapshot: %w", err)
	}

	r.lastSnapshot = data

	r.logger.Info("Created RocksDB snapshot",
		zap.Int("keys", len(snapshot)),
		zap.Int("size_bytes", len(data)),
	)

	return data, nil
}

// Restore restores state from a snapshot
func (r *RocksDBStateBackend) Restore(data []byte) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.db == nil {
		return fmt.Errorf("RocksDB is not initialized")
	}

	var snapshot map[string][]byte
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return fmt.Errorf("failed to deserialize snapshot: %w", err)
	}

	// Use a write batch for better performance
	batch := gorocksdb.NewWriteBatch()
	defer batch.Destroy()

	for key, value := range snapshot {
		batch.Put([]byte(key), value)
	}

	err := r.db.Write(r.writeOpts, batch)
	if err != nil {
		return fmt.Errorf("failed to restore snapshot: %w", err)
	}

	r.logger.Info("Restored RocksDB snapshot",
		zap.Int("keys", len(snapshot)),
		zap.Int("size_bytes", len(data)),
	)

	return nil
}

// CreateCheckpoint creates a hard-linked checkpoint for incremental backups
func (r *RocksDBStateBackend) CreateCheckpoint(checkpointPath string) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.checkpoint == nil {
		return fmt.Errorf("checkpoint not initialized")
	}

	err := r.checkpoint.CreateCheckpoint(checkpointPath, 0)
	if err != nil {
		return fmt.Errorf("failed to create checkpoint: %w", err)
	}

	r.logger.Info("Created RocksDB checkpoint",
		zap.String("path", checkpointPath),
	)

	return nil
}

// GetProperty returns RocksDB property value for monitoring
func (r *RocksDBStateBackend) GetProperty(property string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.db == nil {
		return ""
	}

	return r.db.GetProperty(property)
}

// GetMetrics returns current operation metrics
func (r *RocksDBStateBackend) GetMetrics() map[string]int64 {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return map[string]int64{
		"gets":    r.getCount,
		"puts":    r.putCount,
		"deletes": r.deleteCount,
	}
}

// Flush forces a flush of memtable to SST files
func (r *RocksDBStateBackend) Flush() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.db == nil {
		return fmt.Errorf("RocksDB is not initialized")
	}

	flushOpts := gorocksdb.NewDefaultFlushOptions()
	defer flushOpts.Destroy()

	return r.db.Flush(flushOpts)
}

// Compact triggers manual compaction for better read performance
func (r *RocksDBStateBackend) Compact() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.db == nil {
		return fmt.Errorf("RocksDB is not initialized")
	}

	r.db.CompactRange(gorocksdb.Range{Start: nil, Limit: nil})

	r.logger.Info("Compacted RocksDB")
	return nil
}

// Close closes the RocksDB backend
func (r *RocksDBStateBackend) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.db == nil {
		return nil
	}

	// Clean up resources
	if r.checkpoint != nil {
		r.checkpoint.Destroy()
		r.checkpoint = nil
	}

	if r.readOpts != nil {
		r.readOpts.Destroy()
		r.readOpts = nil
	}

	if r.writeOpts != nil {
		r.writeOpts.Destroy()
		r.writeOpts = nil
	}

	if r.db != nil {
		r.db.Close()
		r.db = nil
	}

	if r.opts != nil {
		r.opts.Destroy()
		r.opts = nil
	}

	r.logger.Info("Closed RocksDB state backend",
		zap.String("path", r.path),
		zap.Int64("total_gets", r.getCount),
		zap.Int64("total_puts", r.putCount),
		zap.Int64("total_deletes", r.deleteCount),
	)

	return nil
}

// Verify interface implementation
var _ stream.StateBackend = (*RocksDBStateBackend)(nil)

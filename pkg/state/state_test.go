package state

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestMemoryBackend_BasicOperations(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	backend := NewMemoryStateBackend(DefaultMemoryConfig(), logger)
	defer backend.Close()

	// Test Put and Get
	key := "test-key"
	value := []byte("test-value")

	err := backend.Put(key, value)
	require.NoError(t, err)

	retrieved, err := backend.Get(key)
	require.NoError(t, err)
	assert.Equal(t, value, retrieved)

	// Test non-existent key
	missing, err := backend.Get("non-existent")
	require.NoError(t, err)
	assert.Nil(t, missing)

	// Test Delete
	err = backend.Delete(key)
	require.NoError(t, err)

	deleted, err := backend.Get(key)
	require.NoError(t, err)
	assert.Nil(t, deleted)
}

func TestMemoryBackend_Snapshot(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	backend := NewMemoryStateBackend(DefaultMemoryConfig(), logger)
	defer backend.Close()

	// Populate data
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("key-%d", i)
		value := []byte(fmt.Sprintf("value-%d", i))
		backend.Put(key, value)
	}

	// Create snapshot
	snapshot, err := backend.Snapshot()
	require.NoError(t, err)
	assert.NotNil(t, snapshot)

	// Clear backend
	backend.Clear()
	assert.Equal(t, 0, backend.Size())

	// Restore from snapshot
	err = backend.Restore(snapshot)
	require.NoError(t, err)
	assert.Equal(t, 100, backend.Size())

	// Verify data
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("key-%d", i)
		value, err := backend.Get(key)
		require.NoError(t, err)
		assert.Equal(t, []byte(fmt.Sprintf("value-%d", i)), value)
	}
}

func TestMemoryBackend_TTL(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := &MemoryStateConfig{
		TTLEnabled: true,
		DefaultTTL: 100 * time.Millisecond,
	}
	backend := NewMemoryStateBackend(config, logger)
	defer backend.Close()

	key := "ttl-key"
	value := []byte("ttl-value")

	// Put with TTL
	err := backend.PutWithTTL(key, value, 100*time.Millisecond)
	require.NoError(t, err)

	// Should exist immediately
	retrieved, err := backend.Get(key)
	require.NoError(t, err)
	assert.Equal(t, value, retrieved)

	// Wait for expiry
	time.Sleep(150 * time.Millisecond)

	// Should be expired
	expired, err := backend.Get(key)
	require.NoError(t, err)
	assert.Nil(t, expired)
}

func TestRocksDBBackend_BasicOperations(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	tmpDir := fmt.Sprintf("/tmp/rocksdb-test-%d", time.Now().UnixNano())
	defer os.RemoveAll(tmpDir)

	config := DefaultRocksDBConfig(tmpDir)
	backend, err := NewRocksDBStateBackend(config, logger)
	if err != nil {
		t.Skipf("Skipping RocksDB test (not available): %v", err)
		return
	}
	defer backend.Close()

	// Test Put and Get
	key := "test-key"
	value := []byte("test-value")

	err = backend.Put(key, value)
	require.NoError(t, err)

	retrieved, err := backend.Get(key)
	require.NoError(t, err)
	assert.Equal(t, value, retrieved)

	// Test non-existent key
	missing, err := backend.Get("non-existent")
	require.NoError(t, err)
	assert.Nil(t, missing)

	// Test Delete
	err = backend.Delete(key)
	require.NoError(t, err)

	deleted, err := backend.Get(key)
	require.NoError(t, err)
	assert.Nil(t, deleted)
}

func TestRocksDBBackend_Snapshot(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	tmpDir := fmt.Sprintf("/tmp/rocksdb-snapshot-test-%d", time.Now().UnixNano())
	defer os.RemoveAll(tmpDir)

	config := DefaultRocksDBConfig(tmpDir)
	backend, err := NewRocksDBStateBackend(config, logger)
	if err != nil {
		t.Skipf("Skipping RocksDB test (not available): %v", err)
		return
	}
	defer backend.Close()

	// Populate data
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("key-%d", i)
		value := []byte(fmt.Sprintf("value-%d", i))
		backend.Put(key, value)
	}

	// Create snapshot
	snapshot, err := backend.Snapshot()
	require.NoError(t, err)
	assert.NotNil(t, snapshot)

	// Create a new backend instance
	tmpDir2 := fmt.Sprintf("/tmp/rocksdb-restore-test-%d", time.Now().UnixNano())
	defer os.RemoveAll(tmpDir2)

	config2 := DefaultRocksDBConfig(tmpDir2)
	backend2, err := NewRocksDBStateBackend(config2, logger)
	require.NoError(t, err)
	defer backend2.Close()

	// Restore from snapshot
	err = backend2.Restore(snapshot)
	require.NoError(t, err)

	// Verify data
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("key-%d", i)
		value, err := backend2.Get(key)
		require.NoError(t, err)
		assert.Equal(t, []byte(fmt.Sprintf("value-%d", i)), value)
	}
}

func TestRocksDBBackend_TTL(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	tmpDir := fmt.Sprintf("/tmp/rocksdb-ttl-test-%d", time.Now().UnixNano())
	defer os.RemoveAll(tmpDir)

	config := DefaultRocksDBConfig(tmpDir)
	config.TTLEnabled = true
	backend, err := NewRocksDBStateBackend(config, logger)
	if err != nil {
		t.Skipf("Skipping RocksDB test (not available): %v", err)
		return
	}
	defer backend.Close()

	key := "ttl-key"
	value := []byte("ttl-value")

	// Put with TTL
	err = backend.PutWithTTL(key, value, 100*time.Millisecond)
	require.NoError(t, err)

	// Should exist immediately
	retrieved, err := backend.Get(key)
	require.NoError(t, err)
	assert.Equal(t, value, retrieved)

	// Wait for expiry
	time.Sleep(150 * time.Millisecond)

	// Should be expired
	expired, err := backend.Get(key)
	require.NoError(t, err)
	assert.Nil(t, expired)
}

func TestRocksDBBackend_Checkpoint(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	tmpDir := fmt.Sprintf("/tmp/rocksdb-cp-test-%d", time.Now().UnixNano())
	defer os.RemoveAll(tmpDir)

	config := DefaultRocksDBConfig(tmpDir)
	config.EnableIncrementalCP = true
	backend, err := NewRocksDBStateBackend(config, logger)
	if err != nil {
		t.Skipf("Skipping RocksDB test (not available): %v", err)
		return
	}
	defer backend.Close()

	// Populate data
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("key-%d", i)
		value := []byte(fmt.Sprintf("value-%d", i))
		backend.Put(key, value)
	}

	// Create checkpoint
	checkpointDir := fmt.Sprintf("/tmp/rocksdb-cp-dir-%d", time.Now().UnixNano())
	defer os.RemoveAll(checkpointDir)

	err = backend.CreateCheckpoint(checkpointDir)
	require.NoError(t, err)

	// Verify checkpoint directory exists
	_, err = os.Stat(checkpointDir)
	require.NoError(t, err)
}

func TestRocksDBBackend_Metrics(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	tmpDir := fmt.Sprintf("/tmp/rocksdb-metrics-test-%d", time.Now().UnixNano())
	defer os.RemoveAll(tmpDir)

	config := DefaultRocksDBConfig(tmpDir)
	backend, err := NewRocksDBStateBackend(config, logger)
	if err != nil {
		t.Skipf("Skipping RocksDB test (not available): %v", err)
		return
	}
	defer backend.Close()

	// Perform operations
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("key-%d", i)
		value := []byte(fmt.Sprintf("value-%d", i))
		backend.Put(key, value)
	}

	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("key-%d", i)
		backend.Get(key)
	}

	backend.Delete("key-0")

	// Check metrics
	metrics := backend.GetMetrics()
	assert.Equal(t, int64(5), metrics["gets"])
	assert.Equal(t, int64(10), metrics["puts"])
	assert.Equal(t, int64(1), metrics["deletes"])
}

func TestRocksDBBackend_CompactionAndFlush(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	tmpDir := fmt.Sprintf("/tmp/rocksdb-compact-test-%d", time.Now().UnixNano())
	defer os.RemoveAll(tmpDir)

	config := DefaultRocksDBConfig(tmpDir)
	backend, err := NewRocksDBStateBackend(config, logger)
	if err != nil {
		t.Skipf("Skipping RocksDB test (not available): %v", err)
		return
	}
	defer backend.Close()

	// Populate data
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("key-%d", i)
		value := []byte(fmt.Sprintf("value-%d", i))
		backend.Put(key, value)
	}

	// Flush memtable
	err = backend.Flush()
	require.NoError(t, err)

	// Compact
	err = backend.Compact()
	require.NoError(t, err)

	// Verify data still accessible
	value, err := backend.Get("key-100")
	require.NoError(t, err)
	assert.Equal(t, []byte("value-100"), value)
}

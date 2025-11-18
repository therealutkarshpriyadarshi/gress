package state

import (
	"fmt"
	"math/rand"
	"os"
	"sort"
	"testing"
	"time"

	"go.uber.org/zap"
)

// BenchmarkMemoryBackendPut benchmarks put operations on memory backend
func BenchmarkMemoryBackendPut(b *testing.B) {
	logger, _ := zap.NewProduction()
	backend := NewMemoryStateBackend(DefaultMemoryConfig(), logger)
	defer backend.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key-%d", i)
		value := []byte(fmt.Sprintf("value-%d", i))
		backend.Put(key, value)
	}
}

// BenchmarkMemoryBackendGet benchmarks get operations on memory backend
func BenchmarkMemoryBackendGet(b *testing.B) {
	logger, _ := zap.NewProduction()
	backend := NewMemoryStateBackend(DefaultMemoryConfig(), logger)
	defer backend.Close()

	// Pre-populate data
	numKeys := 10000
	for i := 0; i < numKeys; i++ {
		key := fmt.Sprintf("key-%d", i)
		value := []byte(fmt.Sprintf("value-%d", i))
		backend.Put(key, value)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key-%d", i%numKeys)
		backend.Get(key)
	}
}

// BenchmarkMemoryBackendSnapshot benchmarks snapshot operations
func BenchmarkMemoryBackendSnapshot(b *testing.B) {
	logger, _ := zap.NewProduction()
	backend := NewMemoryStateBackend(DefaultMemoryConfig(), logger)
	defer backend.Close()

	// Pre-populate data
	numKeys := 10000
	for i := 0; i < numKeys; i++ {
		key := fmt.Sprintf("key-%d", i)
		value := []byte(fmt.Sprintf("value-%d", i))
		backend.Put(key, value)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		backend.Snapshot()
	}
}

// BenchmarkRocksDBBackendPut benchmarks put operations on RocksDB backend
func BenchmarkRocksDBBackendPut(b *testing.B) {
	logger, _ := zap.NewProduction()
	tmpDir := fmt.Sprintf("/tmp/rocksdb-bench-put-%d", time.Now().UnixNano())
	defer os.RemoveAll(tmpDir)

	config := DefaultRocksDBConfig(tmpDir)
	backend, err := NewRocksDBStateBackend(config, logger)
	if err != nil {
		b.Skipf("Skipping RocksDB benchmark (not available): %v", err)
		return
	}
	defer backend.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key-%d", i)
		value := []byte(fmt.Sprintf("value-%d", i))
		backend.Put(key, value)
	}
}

// BenchmarkRocksDBBackendGet benchmarks get operations on RocksDB backend
func BenchmarkRocksDBBackendGet(b *testing.B) {
	logger, _ := zap.NewProduction()
	tmpDir := fmt.Sprintf("/tmp/rocksdb-bench-get-%d", time.Now().UnixNano())
	defer os.RemoveAll(tmpDir)

	config := DefaultRocksDBConfig(tmpDir)
	backend, err := NewRocksDBStateBackend(config, logger)
	if err != nil {
		b.Skipf("Skipping RocksDB benchmark (not available): %v", err)
		return
	}
	defer backend.Close()

	// Pre-populate data
	numKeys := 10000
	for i := 0; i < numKeys; i++ {
		key := fmt.Sprintf("key-%d", i)
		value := []byte(fmt.Sprintf("value-%d", i))
		backend.Put(key, value)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key-%d", i%numKeys)
		backend.Get(key)
	}
}

// BenchmarkRocksDBBackendSnapshot benchmarks snapshot operations
func BenchmarkRocksDBBackendSnapshot(b *testing.B) {
	logger, _ := zap.NewProduction()
	tmpDir := fmt.Sprintf("/tmp/rocksdb-bench-snapshot-%d", time.Now().UnixNano())
	defer os.RemoveAll(tmpDir)

	config := DefaultRocksDBConfig(tmpDir)
	backend, err := NewRocksDBStateBackend(config, logger)
	if err != nil {
		b.Skipf("Skipping RocksDB benchmark (not available): %v", err)
		return
	}
	defer backend.Close()

	// Pre-populate data
	numKeys := 10000
	for i := 0; i < numKeys; i++ {
		key := fmt.Sprintf("key-%d", i)
		value := []byte(fmt.Sprintf("value-%d", i))
		backend.Put(key, value)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		backend.Snapshot()
	}
}

// P99LatencyTest measures P99 latency for state operations
func P99LatencyTest(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	testCases := []struct {
		name       string
		backend    string
		targetP99  time.Duration
		numOps     int
	}{
		{"MemoryBackend-Get", "memory", 1 * time.Millisecond, 10000},
		{"MemoryBackend-Put", "memory", 1 * time.Millisecond, 10000},
		{"RocksDBBackend-Get", "rocksdb", 10 * time.Millisecond, 10000},
		{"RocksDBBackend-Put", "rocksdb", 10 * time.Millisecond, 10000},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var backend interface {
				Get(key string) ([]byte, error)
				Put(key string, value []byte) error
				Close() error
			}

			if tc.backend == "memory" {
				backend = NewMemoryStateBackend(DefaultMemoryConfig(), logger)
			} else {
				tmpDir := fmt.Sprintf("/tmp/rocksdb-p99-%d", time.Now().UnixNano())
				defer os.RemoveAll(tmpDir)

				config := DefaultRocksDBConfig(tmpDir)
				rocksBackend, err := NewRocksDBStateBackend(config, logger)
				if err != nil {
					t.Skipf("Skipping RocksDB test (not available): %v", err)
					return
				}
				backend = rocksBackend
			}
			defer backend.Close()

			// Pre-populate for Get tests
			if contains(tc.name, "Get") {
				for i := 0; i < tc.numOps; i++ {
					key := fmt.Sprintf("key-%d", i)
					value := []byte(fmt.Sprintf("value-%d", i))
					backend.Put(key, value)
				}
			}

			// Measure latencies
			latencies := make([]time.Duration, tc.numOps)

			for i := 0; i < tc.numOps; i++ {
				key := fmt.Sprintf("key-%d", rand.Intn(tc.numOps))
				value := []byte(fmt.Sprintf("value-%d", i))

				start := time.Now()
				if contains(tc.name, "Get") {
					backend.Get(key)
				} else {
					backend.Put(key, value)
				}
				latencies[i] = time.Since(start)
			}

			// Calculate P99
			sort.Slice(latencies, func(i, j int) bool {
				return latencies[i] < latencies[j]
			})

			p50 := latencies[int(float64(tc.numOps)*0.50)]
			p95 := latencies[int(float64(tc.numOps)*0.95)]
			p99 := latencies[int(float64(tc.numOps)*0.99)]
			max := latencies[tc.numOps-1]

			t.Logf("Latency percentiles for %s:", tc.name)
			t.Logf("  P50: %v", p50)
			t.Logf("  P95: %v", p95)
			t.Logf("  P99: %v", p99)
			t.Logf("  MAX: %v", max)

			if p99 > tc.targetP99 {
				t.Errorf("P99 latency %v exceeds target %v", p99, tc.targetP99)
			}
		})
	}
}

// BenchmarkConcurrentOperations benchmarks concurrent read/write operations
func BenchmarkConcurrentOperations(b *testing.B) {
	logger, _ := zap.NewProduction()
	backend := NewMemoryStateBackend(DefaultMemoryConfig(), logger)
	defer backend.Close()

	// Pre-populate data
	numKeys := 1000
	for i := 0; i < numKeys; i++ {
		key := fmt.Sprintf("key-%d", i)
		value := []byte(fmt.Sprintf("value-%d", i))
		backend.Put(key, value)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			if i%2 == 0 {
				key := fmt.Sprintf("key-%d", i%numKeys)
				backend.Get(key)
			} else {
				key := fmt.Sprintf("key-%d", i%numKeys)
				value := []byte(fmt.Sprintf("value-%d", i))
				backend.Put(key, value)
			}
			i++
		}
	})
}

// BenchmarkTTLOperations benchmarks operations with TTL
func BenchmarkTTLOperations(b *testing.B) {
	logger, _ := zap.NewProduction()
	config := &MemoryStateConfig{
		TTLEnabled: true,
		DefaultTTL: 10 * time.Second,
	}
	backend := NewMemoryStateBackend(config, logger)
	defer backend.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key-%d", i)
		value := []byte(fmt.Sprintf("value-%d", i))
		backend.PutWithTTL(key, value, 10*time.Second)
	}
}

// BenchmarkRocksDBBatchPut benchmarks batch put operations on RocksDB backend
func BenchmarkRocksDBBatchPut(b *testing.B) {
	logger, _ := zap.NewProduction()
	tmpDir := fmt.Sprintf("/tmp/rocksdb-bench-batch-put-%d", time.Now().UnixNano())
	defer os.RemoveAll(tmpDir)

	config := LowLatencyConfig(tmpDir)
	backend, err := NewRocksDBStateBackend(config, logger)
	if err != nil {
		b.Skipf("Skipping RocksDB benchmark (not available): %v", err)
		return
	}
	defer backend.Close()

	batchSize := 100
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		entries := make(map[string][]byte, batchSize)
		for j := 0; j < batchSize; j++ {
			key := fmt.Sprintf("key-%d-%d", i, j)
			value := []byte(fmt.Sprintf("value-%d-%d", i, j))
			entries[key] = value
		}
		backend.BatchPut(entries)
	}
}

// BenchmarkRocksDBBatchGet benchmarks batch get operations on RocksDB backend
func BenchmarkRocksDBBatchGet(b *testing.B) {
	logger, _ := zap.NewProduction()
	tmpDir := fmt.Sprintf("/tmp/rocksdb-bench-batch-get-%d", time.Now().UnixNano())
	defer os.RemoveAll(tmpDir)

	config := LowLatencyConfig(tmpDir)
	backend, err := NewRocksDBStateBackend(config, logger)
	if err != nil {
		b.Skipf("Skipping RocksDB benchmark (not available): %v", err)
		return
	}
	defer backend.Close()

	// Pre-populate data
	numKeys := 10000
	for i := 0; i < numKeys; i++ {
		key := fmt.Sprintf("key-%d", i)
		value := []byte(fmt.Sprintf("value-%d", i))
		backend.Put(key, value)
	}

	batchSize := 100
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		keys := make([]string, batchSize)
		for j := 0; j < batchSize; j++ {
			keys[j] = fmt.Sprintf("key-%d", (i*batchSize+j)%numKeys)
		}
		backend.BatchGet(keys)
	}
}

// BenchmarkRocksDBLowLatencyPreset benchmarks RocksDB with low-latency preset
func BenchmarkRocksDBLowLatencyPreset(b *testing.B) {
	logger, _ := zap.NewProduction()
	tmpDir := fmt.Sprintf("/tmp/rocksdb-bench-low-latency-%d", time.Now().UnixNano())
	defer os.RemoveAll(tmpDir)

	config := LowLatencyConfig(tmpDir)
	backend, err := NewRocksDBStateBackend(config, logger)
	if err != nil {
		b.Skipf("Skipping RocksDB benchmark (not available): %v", err)
		return
	}
	defer backend.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key-%d", i)
		value := []byte(fmt.Sprintf("value-%d", i))
		backend.Put(key, value)
		backend.Get(key)
	}
}

// BenchmarkRocksDBHighThroughputPreset benchmarks RocksDB with high-throughput preset
func BenchmarkRocksDBHighThroughputPreset(b *testing.B) {
	logger, _ := zap.NewProduction()
	tmpDir := fmt.Sprintf("/tmp/rocksdb-bench-high-throughput-%d", time.Now().UnixNano())
	defer os.RemoveAll(tmpDir)

	config := HighThroughputConfig(tmpDir)
	backend, err := NewRocksDBStateBackend(config, logger)
	if err != nil {
		b.Skipf("Skipping RocksDB benchmark (not available): %v", err)
		return
	}
	defer backend.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key-%d", i)
		value := []byte(fmt.Sprintf("value-%d", i))
		backend.Put(key, value)
	}
}

// BenchmarkRocksDBLargeStatePreset benchmarks RocksDB with large-state preset
func BenchmarkRocksDBLargeStatePreset(b *testing.B) {
	logger, _ := zap.NewProduction()
	tmpDir := fmt.Sprintf("/tmp/rocksdb-bench-large-state-%d", time.Now().UnixNano())
	defer os.RemoveAll(tmpDir)

	config := LargeStateConfig(tmpDir)
	backend, err := NewRocksDBStateBackend(config, logger)
	if err != nil {
		b.Skipf("Skipping RocksDB benchmark (not available): %v", err)
		return
	}
	defer backend.Close()

	// Use larger values to simulate large state
	valueSize := 1024 // 1KB values
	value := make([]byte, valueSize)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key-%d", i)
		backend.Put(key, value)
	}
}

// TestRocksDBLatencyTarget tests that RocksDB meets <10ms P99 latency target
func TestRocksDBLatencyTarget(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	tmpDir := fmt.Sprintf("/tmp/rocksdb-latency-test-%d", time.Now().UnixNano())
	defer os.RemoveAll(tmpDir)

	testCases := []struct {
		name      string
		config    *RocksDBConfig
		targetP99 time.Duration
		numOps    int
	}{
		{"LowLatency-Get", LowLatencyConfig(tmpDir + "-low-latency-get"), 10 * time.Millisecond, 10000},
		{"LowLatency-Put", LowLatencyConfig(tmpDir + "-low-latency-put"), 10 * time.Millisecond, 10000},
		{"HighThroughput-Put", HighThroughputConfig(tmpDir + "-high-throughput"), 15 * time.Millisecond, 10000},
		{"LargeState-Put", LargeStateConfig(tmpDir + "-large-state"), 15 * time.Millisecond, 10000},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			backend, err := NewRocksDBStateBackend(tc.config, logger)
			if err != nil {
				t.Skipf("Skipping RocksDB test (not available): %v", err)
				return
			}
			defer backend.Close()
			defer os.RemoveAll(tc.config.Path)

			// Pre-populate for Get tests
			if contains(tc.name, "Get") {
				for i := 0; i < tc.numOps; i++ {
					key := fmt.Sprintf("key-%d", i)
					value := []byte(fmt.Sprintf("value-%d", i))
					backend.Put(key, value)
				}
			}

			// Measure latencies
			latencies := make([]time.Duration, tc.numOps)

			for i := 0; i < tc.numOps; i++ {
				key := fmt.Sprintf("key-%d", rand.Intn(tc.numOps))
				value := []byte(fmt.Sprintf("value-%d", i))

				start := time.Now()
				if contains(tc.name, "Get") {
					backend.Get(key)
				} else {
					backend.Put(key, value)
				}
				latencies[i] = time.Since(start)
			}

			// Calculate percentiles
			sort.Slice(latencies, func(i, j int) bool {
				return latencies[i] < latencies[j]
			})

			p50 := latencies[int(float64(tc.numOps)*0.50)]
			p95 := latencies[int(float64(tc.numOps)*0.95)]
			p99 := latencies[int(float64(tc.numOps)*0.99)]
			max := latencies[tc.numOps-1]

			t.Logf("Latency percentiles for %s (preset: %s):", tc.name, tc.config.Preset)
			t.Logf("  P50: %v", p50)
			t.Logf("  P95: %v", p95)
			t.Logf("  P99: %v", p99)
			t.Logf("  MAX: %v", max)

			// Print metrics
			metrics := backend.GetDetailedMetrics()
			t.Logf("Backend metrics: %+v", metrics)

			if p99 > tc.targetP99 {
				t.Errorf("P99 latency %v exceeds target %v for preset %s", p99, tc.targetP99, tc.config.Preset)
			} else {
				t.Logf("✓ P99 latency %v meets target %v", p99, tc.targetP99)
			}
		})
	}
}

// TestRocksDBBatchOperationsLatency tests batch operations latency
func TestRocksDBBatchOperationsLatency(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	tmpDir := fmt.Sprintf("/tmp/rocksdb-batch-latency-%d", time.Now().UnixNano())
	defer os.RemoveAll(tmpDir)

	config := LowLatencyConfig(tmpDir)
	backend, err := NewRocksDBStateBackend(config, logger)
	if err != nil {
		t.Skipf("Skipping RocksDB test (not available): %v", err)
		return
	}
	defer backend.Close()

	batchSizes := []int{10, 50, 100, 500, 1000}

	for _, batchSize := range batchSizes {
		t.Run(fmt.Sprintf("BatchSize-%d", batchSize), func(t *testing.T) {
			latencies := make([]time.Duration, 100)

			for i := 0; i < 100; i++ {
				entries := make(map[string][]byte, batchSize)
				for j := 0; j < batchSize; j++ {
					key := fmt.Sprintf("batch-%d-key-%d", i, j)
					value := []byte(fmt.Sprintf("value-%d", j))
					entries[key] = value
				}

				start := time.Now()
				err := backend.BatchPut(entries)
				latencies[i] = time.Since(start)

				if err != nil {
					t.Errorf("BatchPut failed: %v", err)
				}
			}

			sort.Slice(latencies, func(i, j int) bool {
				return latencies[i] < latencies[j]
			})

			p50 := latencies[50]
			p99 := latencies[99]

			t.Logf("Batch size %d - P50: %v, P99: %v", batchSize, p50, p99)

			// Batch operations should complete faster per-item than individual operations
			// Target: <50ms for batches of 100 items
			if batchSize == 100 && p99 > 50*time.Millisecond {
				t.Errorf("Batch P99 latency %v exceeds 50ms for batch size 100", p99)
			}
		})
	}
}

// TestRocksDBConcurrentAccess tests concurrent read/write performance
func TestRocksDBConcurrentAccess(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	tmpDir := fmt.Sprintf("/tmp/rocksdb-concurrent-%d", time.Now().UnixNano())
	defer os.RemoveAll(tmpDir)

	config := LowLatencyConfig(tmpDir)
	backend, err := NewRocksDBStateBackend(config, logger)
	if err != nil {
		t.Skipf("Skipping RocksDB test (not available): %v", err)
		return
	}
	defer backend.Close()

	// Pre-populate
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("key-%d", i)
		value := []byte(fmt.Sprintf("value-%d", i))
		backend.Put(key, value)
	}

	// Concurrent access
	var wg sync.WaitGroup
	numGoroutines := 10
	opsPerGoroutine := 1000

	start := time.Now()

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < opsPerGoroutine; i++ {
				key := fmt.Sprintf("key-%d", rand.Intn(1000))
				if i%2 == 0 {
					backend.Get(key)
				} else {
					value := []byte(fmt.Sprintf("value-%d-%d", id, i))
					backend.Put(key, value)
				}
			}
		}(g)
	}

	wg.Wait()
	elapsed := time.Since(start)

	totalOps := numGoroutines * opsPerGoroutine
	opsPerSec := float64(totalOps) / elapsed.Seconds()

	t.Logf("Concurrent access: %d ops in %v = %.0f ops/sec", totalOps, elapsed, opsPerSec)

	metrics := backend.GetDetailedMetrics()
	t.Logf("Final metrics: %+v", metrics)

	// Should achieve at least 10K ops/sec under concurrent load
	if opsPerSec < 10000 {
		t.Errorf("Throughput %.0f ops/sec is below target 10K ops/sec", opsPerSec)
	} else {
		t.Logf("✓ Throughput %.0f ops/sec meets target", opsPerSec)
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[len(s)-len(substr):] == substr ||
		   (len(s) > len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

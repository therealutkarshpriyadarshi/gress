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

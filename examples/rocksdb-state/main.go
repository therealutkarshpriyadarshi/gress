package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	"github.com/therealutkarshpriyadarshi/gress/pkg/state"
	"go.uber.org/zap"
)

// PurchaseEvent represents an e-commerce purchase event
type PurchaseEvent struct {
	UserID    string  `json:"user_id"`
	ProductID string  `json:"product_id"`
	Amount    float64 `json:"amount"`
	Timestamp int64   `json:"timestamp"`
}

// UserAggregate stores aggregated user purchase data
type UserAggregate struct {
	UserID       string  `json:"user_id"`
	TotalAmount  float64 `json:"total_amount"`
	PurchaseCount int     `json:"purchase_count"`
	LastPurchase int64   `json:"last_purchase"`
}

func main() {
	logger, _ := zap.NewDevelopment()

	fmt.Println("=== RocksDB State Backend Demo ===")
	fmt.Println("Demonstrating production-grade state management with <10ms P99 latency")
	fmt.Println()

	// Demo 1: Low-Latency Configuration
	fmt.Println("Demo 1: Low-Latency Preset (optimized for <10ms P99 latency)")
	demoLowLatency(logger)

	// Demo 2: High-Throughput Configuration
	fmt.Println("\nDemo 2: High-Throughput Preset (optimized for maximum ops/sec)")
	demoHighThroughput(logger)

	// Demo 3: Large State Configuration
	fmt.Println("\nDemo 3: Large-State Preset (optimized for state > RAM)")
	demoLargeState(logger)

	// Demo 4: Batch Operations
	fmt.Println("\nDemo 4: Batch Operations (high-performance bulk operations)")
	demoBatchOperations(logger)
}

func demoLowLatency(logger *zap.Logger) {
	// Create RocksDB backend with low-latency preset
	config := state.LowLatencyConfig("./data/rocksdb-low-latency")
	backend, err := state.NewRocksDBStateBackend(config, logger)
	if err != nil {
		logger.Fatal("Failed to create backend", zap.Error(err))
	}
	defer backend.Close()

	// Process events with low-latency state operations
	numEvents := 1000
	start := time.Now()

	for i := 0; i < numEvents; i++ {
		event := PurchaseEvent{
			UserID:    fmt.Sprintf("user-%d", rand.Intn(100)),
			ProductID: fmt.Sprintf("product-%d", rand.Intn(1000)),
			Amount:    float64(rand.Intn(1000)),
			Timestamp: time.Now().Unix(),
		}

		// Read current aggregate from state
		key := fmt.Sprintf("user:%s", event.UserID)
		data, err := backend.Get(key)

		var aggregate UserAggregate
		if err == nil && data != nil {
			json.Unmarshal(data, &aggregate)
		} else {
			aggregate = UserAggregate{UserID: event.UserID}
		}

		// Update aggregate
		aggregate.TotalAmount += event.Amount
		aggregate.PurchaseCount++
		aggregate.LastPurchase = event.Timestamp

		// Write updated aggregate back to state
		updatedData, _ := json.Marshal(aggregate)
		backend.Put(key, updatedData)
	}

	elapsed := time.Since(start)

	// Get metrics
	metrics := backend.GetDetailedMetrics()
	fmt.Printf("  Processed %d events in %v\n", numEvents, elapsed)
	fmt.Printf("  Throughput: %.0f events/sec\n", float64(numEvents)/elapsed.Seconds())
	fmt.Printf("  Metrics: %+v\n", metrics)
}

func demoHighThroughput(logger *zap.Logger) {
	// Create RocksDB backend with high-throughput preset
	config := state.HighThroughputConfig("./data/rocksdb-high-throughput")
	backend, err := state.NewRocksDBStateBackend(config, logger)
	if err != nil {
		logger.Fatal("Failed to create backend", zap.Error(err))
	}
	defer backend.Close()

	// Process large volume of events
	numEvents := 5000
	start := time.Now()

	for i := 0; i < numEvents; i++ {
		key := fmt.Sprintf("metric:%d", i)
		value := []byte(fmt.Sprintf("value-%d", i))
		backend.Put(key, value)
	}

	elapsed := time.Since(start)

	// Flush to ensure all data is persisted
	backend.Flush()

	metrics := backend.GetDetailedMetrics()
	fmt.Printf("  Processed %d writes in %v\n", numEvents, elapsed)
	fmt.Printf("  Throughput: %.0f writes/sec\n", float64(numEvents)/elapsed.Seconds())
	fmt.Printf("  Metrics: %+v\n", metrics)
}

func demoLargeState(logger *zap.Logger) {
	// Create RocksDB backend optimized for large state
	config := state.LargeStateConfig("./data/rocksdb-large-state")
	backend, err := state.NewRocksDBStateBackend(config, logger)
	if err != nil {
		logger.Fatal("Failed to create backend", zap.Error(err))
	}
	defer backend.Close()

	// Simulate large state with bigger values
	valueSize := 10 * 1024 // 10KB per entry
	numEntries := 1000
	value := make([]byte, valueSize)

	start := time.Now()

	for i := 0; i < numEntries; i++ {
		key := fmt.Sprintf("large-state:%d", i)
		backend.Put(key, value)
	}

	elapsed := time.Since(start)
	totalSize := float64(numEntries * valueSize) / (1024 * 1024) // MB

	// Test retrieval
	readStart := time.Now()
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("large-state:%d", rand.Intn(numEntries))
		backend.Get(key)
	}
	readElapsed := time.Since(readStart)

	metrics := backend.GetDetailedMetrics()
	fmt.Printf("  Wrote %.2f MB in %v\n", totalSize, elapsed)
	fmt.Printf("  Write throughput: %.2f MB/sec\n", totalSize/elapsed.Seconds())
	fmt.Printf("  Random read latency (100 ops): %v avg\n", readElapsed/100)
	fmt.Printf("  Metrics: %+v\n", metrics)
}

func demoBatchOperations(logger *zap.Logger) {
	// Create RocksDB backend with low-latency preset
	config := state.LowLatencyConfig("./data/rocksdb-batch")
	backend, err := state.NewRocksDBStateBackend(config, logger)
	if err != nil {
		logger.Fatal("Failed to create backend", zap.Error(err))
	}
	defer backend.Close()

	// Batch Put - write many entries at once
	batchSize := 1000
	entries := make(map[string][]byte, batchSize)
	for i := 0; i < batchSize; i++ {
		key := fmt.Sprintf("batch-key:%d", i)
		value := []byte(fmt.Sprintf("batch-value-%d", i))
		entries[key] = value
	}

	start := time.Now()
	err = backend.BatchPut(entries)
	if err != nil {
		logger.Error("BatchPut failed", zap.Error(err))
	}
	batchPutDuration := time.Since(start)

	// Batch Get - read many entries at once
	keys := make([]string, 100)
	for i := 0; i < 100; i++ {
		keys[i] = fmt.Sprintf("batch-key:%d", rand.Intn(batchSize))
	}

	start = time.Now()
	results, err := backend.BatchGet(keys)
	if err != nil {
		logger.Error("BatchGet failed", zap.Error(err))
	}
	batchGetDuration := time.Since(start)

	// Batch Delete
	deleteKeys := make([]string, 100)
	for i := 0; i < 100; i++ {
		deleteKeys[i] = fmt.Sprintf("batch-key:%d", i)
	}

	start = time.Now()
	err = backend.BatchDelete(deleteKeys)
	if err != nil {
		logger.Error("BatchDelete failed", zap.Error(err))
	}
	batchDeleteDuration := time.Since(start)

	metrics := backend.GetDetailedMetrics()
	fmt.Printf("  BatchPut (%d entries): %v (%.2f µs/entry)\n", batchSize, batchPutDuration,
		float64(batchPutDuration.Microseconds())/float64(batchSize))
	fmt.Printf("  BatchGet (%d keys): %v, found %d entries (%.2f µs/entry)\n",
		len(keys), batchGetDuration, len(results), float64(batchGetDuration.Microseconds())/float64(len(keys)))
	fmt.Printf("  BatchDelete (%d keys): %v (%.2f µs/entry)\n", len(deleteKeys), batchDeleteDuration,
		float64(batchDeleteDuration.Microseconds())/float64(len(deleteKeys)))
	fmt.Printf("  Metrics: %+v\n", metrics)
}

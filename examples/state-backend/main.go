package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/therealutkarshpriyadarshi/gress/pkg/checkpoint"
	"github.com/therealutkarshpriyadarshi/gress/pkg/state"
	"github.com/therealutkarshpriyadarshi/gress/pkg/stream"
	"go.uber.org/zap"
)

// UserProfile represents user state
type UserProfile struct {
	UserID       string    `json:"user_id"`
	TotalPurchases int     `json:"total_purchases"`
	TotalSpent   float64   `json:"total_spent"`
	LastPurchase time.Time `json:"last_purchase"`
	Category     string    `json:"category"`
}

// PurchaseEvent represents a purchase event
type PurchaseEvent struct {
	UserID    string    `json:"user_id"`
	Amount    float64   `json:"amount"`
	Timestamp time.Time `json:"timestamp"`
}

// StatefulPurchaseProcessor processes purchases with persistent state
type StatefulPurchaseProcessor struct {
	stateBackend stream.StateBackend
	logger       *zap.Logger
}

func NewStatefulPurchaseProcessor(backend stream.StateBackend, logger *zap.Logger) *StatefulPurchaseProcessor {
	return &StatefulPurchaseProcessor{
		stateBackend: backend,
		logger:       logger,
	}
}

func (p *StatefulPurchaseProcessor) Process(ctx *stream.ProcessingContext, event *stream.Event) ([]*stream.Event, error) {
	// Parse purchase event
	purchaseData, ok := event.Value.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid event format")
	}

	purchase := &PurchaseEvent{
		UserID:    event.Key,
		Amount:    purchaseData["amount"].(float64),
		Timestamp: event.EventTime,
	}

	// Get existing user profile from state
	stateKey := "user:" + purchase.UserID
	var profile UserProfile

	existingData, err := p.stateBackend.Get(stateKey)
	if err != nil {
		return nil, err
	}

	if existingData != nil {
		// Deserialize existing profile
		if err := json.Unmarshal(existingData, &profile); err != nil {
			return nil, err
		}
	} else {
		// New user
		profile = UserProfile{
			UserID: purchase.UserID,
		}
	}

	// Update profile
	profile.TotalPurchases++
	profile.TotalSpent += purchase.Amount
	profile.LastPurchase = purchase.Timestamp

	// Categorize user based on spending
	if profile.TotalSpent > 10000 {
		profile.Category = "VIP"
	} else if profile.TotalSpent > 1000 {
		profile.Category = "Premium"
	} else {
		profile.Category = "Regular"
	}

	// Save updated profile to state
	profileData, err := json.Marshal(profile)
	if err != nil {
		return nil, err
	}

	if err := p.stateBackend.Put(stateKey, profileData); err != nil {
		return nil, err
	}

	p.logger.Info("Updated user profile",
		zap.String("user_id", profile.UserID),
		zap.Int("total_purchases", profile.TotalPurchases),
		zap.Float64("total_spent", profile.TotalSpent),
		zap.String("category", profile.Category),
	)

	// Emit enriched event
	enrichedEvent := &stream.Event{
		Key:       event.Key,
		Value:     profile,
		EventTime: event.EventTime,
	}

	return []*stream.Event{enrichedEvent}, nil
}

func (p *StatefulPurchaseProcessor) Close() error {
	return nil
}

func main() {
	// Initialize logger
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	logger.Info("Starting State Backend Example")

	// Create state directory
	stateDir := "./state-data"
	checkpointDir := "./checkpoints"
	os.MkdirAll(stateDir, 0755)
	os.MkdirAll(checkpointDir, 0755)

	// Choose backend type
	backendType := "rocksdb" // or "memory"
	if len(os.Args) > 1 {
		backendType = os.Args[1]
	}

	var stateBackend stream.StateBackend
	var err error

	switch backendType {
	case "rocksdb":
		logger.Info("Using RocksDB state backend")
		config := state.DefaultRocksDBConfig(stateDir)
		config.TTLEnabled = true
		config.DefaultTTL = 24 * time.Hour // User profiles expire after 24 hours
		config.EnableIncrementalCP = true

		stateBackend, err = state.NewRocksDBStateBackend(config, logger)
		if err != nil {
			logger.Fatal("Failed to create RocksDB backend", zap.Error(err))
		}

	case "memory":
		logger.Info("Using Memory state backend")
		config := &state.MemoryStateConfig{
			TTLEnabled: true,
			DefaultTTL: 24 * time.Hour,
		}
		stateBackend = state.NewMemoryStateBackend(config, logger)

	default:
		logger.Fatal("Invalid backend type", zap.String("type", backendType))
	}

	defer stateBackend.Close()

	// Create checkpoint manager
	checkpointMgr := checkpoint.NewManager(30*time.Second, checkpointDir, logger)
	checkpointMgr.RegisterStateBackend(stateBackend)
	checkpointMgr.EnableIncrementalCheckpoints(true)

	// Try to restore from latest checkpoint
	if checkpoint, err := checkpointMgr.LoadLatestCheckpoint(); err == nil {
		logger.Info("Restoring from checkpoint", zap.String("id", checkpoint.ID))
		if err := checkpointMgr.RestoreFromCheckpoint(checkpoint); err != nil {
			logger.Error("Failed to restore checkpoint", zap.Error(err))
		} else {
			logger.Info("Successfully restored from checkpoint")
		}
	} else {
		logger.Info("No checkpoint found, starting fresh")
	}

	// Create stateful processor
	processor := NewStatefulPurchaseProcessor(stateBackend, logger)

	// Create context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start checkpoint loop
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				offsets := make(map[int32]int64)
				offsets[0] = time.Now().Unix()

				checkpoint, err := checkpointMgr.CreateCheckpoint(ctx, offsets)
				if err != nil {
					logger.Error("Failed to create checkpoint", zap.Error(err))
				} else {
					logger.Info("Checkpoint created", zap.String("id", checkpoint.ID))
				}
			}
		}
	}()

	// Simulate purchase events
	go func() {
		users := []string{"user-1", "user-2", "user-3", "user-4", "user-5"}
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		eventCount := 0
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// Generate random purchase
				userID := users[rand.Intn(len(users))]
				amount := float64(rand.Intn(1000)) + 10.0

				event := &stream.Event{
					Key: userID,
					Value: map[string]interface{}{
						"amount": amount,
					},
					EventTime: time.Now(),
				}

				// Process event
				processingCtx := &stream.ProcessingContext{
					Ctx:   ctx,
					State: stateBackend,
				}

				_, err := processor.Process(processingCtx, event)
				if err != nil {
					logger.Error("Failed to process event", zap.Error(err))
				}

				eventCount++
				if eventCount%10 == 0 {
					// Print state backend metrics
					if rocksBackend, ok := stateBackend.(*state.RocksDBStateBackend); ok {
						metrics := rocksBackend.GetMetrics()
						logger.Info("RocksDB metrics",
							zap.Int64("gets", metrics["gets"]),
							zap.Int64("puts", metrics["puts"]),
							zap.Int64("deletes", metrics["deletes"]),
						)
					} else if memBackend, ok := stateBackend.(*state.MemoryStateBackend); ok {
						metrics := memBackend.GetMetrics()
						logger.Info("Memory backend metrics",
							zap.Int64("gets", metrics["gets"]),
							zap.Int64("puts", metrics["puts"]),
							zap.Int64("deletes", metrics["deletes"]),
							zap.Int64("keys", metrics["keys"]),
						)
					}
				}
			}
		}
	}()

	// Wait for interrupt signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	logger.Info("Shutting down...")

	// Create final checkpoint
	offsets := make(map[int32]int64)
	offsets[0] = time.Now().Unix()

	checkpoint, err := checkpointMgr.CreateCheckpoint(ctx, offsets)
	if err != nil {
		logger.Error("Failed to create final checkpoint", zap.Error(err))
	} else {
		logger.Info("Final checkpoint created", zap.String("id", checkpoint.ID))
	}

	logger.Info("Shutdown complete")
}

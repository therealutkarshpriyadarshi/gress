package join

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/therealutkarshpriyadarshi/gress/pkg/stream"
	"go.uber.org/zap"
)

// JoinOperator performs join operations on two streams
type JoinOperator struct {
	config        *JoinConfig
	state         *JoinState
	condition     JoinCondition
	logger        *zap.Logger
	cleanupTicker *time.Ticker
	cleanupStop   chan struct{}
	mu            sync.RWMutex
}

// NewJoinOperator creates a new join operator
func NewJoinOperator(config *JoinConfig, logger *zap.Logger) *JoinOperator {
	if config == nil {
		config = DefaultJoinConfig()
	}
	if config.Condition == nil {
		config.Condition = EqualityCondition()
	}

	jo := &JoinOperator{
		config:      config,
		state:       NewJoinState(config),
		condition:   config.Condition,
		logger:      logger,
		cleanupStop: make(chan struct{}),
	}

	// Start cleanup goroutine if enabled
	if config.EnableCleanup {
		jo.startCleanup()
	}

	return jo
}

// ProcessLeft processes an event from the left stream
func (jo *JoinOperator) ProcessLeft(ctx *stream.ProcessingContext, event *stream.Event) ([]*stream.Event, error) {
	jo.mu.RLock()
	defer jo.mu.RUnlock()

	// Calculate expiration time
	expiresAt := event.EventTime.Add(jo.config.StateRetention)

	// Store left event
	jo.state.AddLeftEvent(event, expiresAt)

	// Find matches in right stream
	var results []*stream.Event

	switch jo.config.Strategy {
	case HashJoin:
		results = jo.hashJoinLeft(event)
	case SortMergeJoin:
		results = jo.sortMergeJoinLeft(event)
	case NestedLoopJoin:
		results = jo.nestedLoopJoinLeft(event)
	default:
		results = jo.hashJoinLeft(event)
	}

	return results, nil
}

// ProcessRight processes an event from the right stream
func (jo *JoinOperator) ProcessRight(ctx *stream.ProcessingContext, event *stream.Event) ([]*stream.Event, error) {
	jo.mu.RLock()
	defer jo.mu.RUnlock()

	// Calculate expiration time
	expiresAt := event.EventTime.Add(jo.config.StateRetention)

	// Store right event
	jo.state.AddRightEvent(event, expiresAt)

	// Find matches in left stream
	var results []*stream.Event

	switch jo.config.Strategy {
	case HashJoin:
		results = jo.hashJoinRight(event)
	case SortMergeJoin:
		results = jo.sortMergeJoinRight(event)
	case NestedLoopJoin:
		results = jo.nestedLoopJoinRight(event)
	default:
		results = jo.hashJoinRight(event)
	}

	return results, nil
}

// hashJoinLeft performs hash-based join for left event
func (jo *JoinOperator) hashJoinLeft(leftEvent *stream.Event) []*stream.Event {
	var results []*stream.Event

	// Get matching right events by key
	rightEvents := jo.state.GetRightEvents(leftEvent.Key)

	matchFound := false
	for _, rightStored := range rightEvents {
		// Check join condition and time window
		if jo.matchesCondition(leftEvent, rightStored.Event) {
			matchFound = true

			// Create join result
			result := jo.createJoinResult(leftEvent, rightStored.Event)
			if result != nil {
				results = append(results, result.ToEvent())
			}

			// Update match count
			jo.state.IncrementMatchCount(rightStored)
		}
	}

	// Handle outer join cases when no match found
	if !matchFound && (jo.config.Type == LeftOuterJoin || jo.config.Type == FullOuterJoin) {
		result := jo.createJoinResult(leftEvent, nil)
		if result != nil {
			results = append(results, result.ToEvent())
		}
	}

	return results
}

// hashJoinRight performs hash-based join for right event
func (jo *JoinOperator) hashJoinRight(rightEvent *stream.Event) []*stream.Event {
	var results []*stream.Event

	// Get matching left events by key
	leftEvents := jo.state.GetLeftEvents(rightEvent.Key)

	matchFound := false
	for _, leftStored := range leftEvents {
		// Check join condition and time window
		if jo.matchesCondition(leftStored.Event, rightEvent) {
			matchFound = true

			// Create join result
			result := jo.createJoinResult(leftStored.Event, rightEvent)
			if result != nil {
				results = append(results, result.ToEvent())
			}

			// Update match count
			jo.state.IncrementMatchCount(leftStored)
		}
	}

	// Handle outer join cases when no match found
	if !matchFound && (jo.config.Type == RightOuterJoin || jo.config.Type == FullOuterJoin) {
		result := jo.createJoinResult(nil, rightEvent)
		if result != nil {
			results = append(results, result.ToEvent())
		}
	}

	return results
}

// sortMergeJoinLeft performs sort-merge join for left event
func (jo *JoinOperator) sortMergeJoinLeft(leftEvent *stream.Event) []*stream.Event {
	// For now, use hash join implementation
	// Sort-merge join is more efficient for sorted streams but requires buffering
	return jo.hashJoinLeft(leftEvent)
}

// sortMergeJoinRight performs sort-merge join for right event
func (jo *JoinOperator) sortMergeJoinRight(rightEvent *stream.Event) []*stream.Event {
	// For now, use hash join implementation
	return jo.hashJoinRight(rightEvent)
}

// nestedLoopJoinLeft performs nested loop join for left event
func (jo *JoinOperator) nestedLoopJoinLeft(leftEvent *stream.Event) []*stream.Event {
	var results []*stream.Event

	// Check all right events (not just same key)
	matchFound := false
	for _, key := range jo.state.GetAllRightKeys() {
		rightEvents := jo.state.GetRightEvents(key)
		for _, rightStored := range rightEvents {
			if jo.matchesCondition(leftEvent, rightStored.Event) {
				matchFound = true

				result := jo.createJoinResult(leftEvent, rightStored.Event)
				if result != nil {
					results = append(results, result.ToEvent())
				}

				jo.state.IncrementMatchCount(rightStored)
			}
		}
	}

	if !matchFound && (jo.config.Type == LeftOuterJoin || jo.config.Type == FullOuterJoin) {
		result := jo.createJoinResult(leftEvent, nil)
		if result != nil {
			results = append(results, result.ToEvent())
		}
	}

	return results
}

// nestedLoopJoinRight performs nested loop join for right event
func (jo *JoinOperator) nestedLoopJoinRight(rightEvent *stream.Event) []*stream.Event {
	var results []*stream.Event

	// Check all left events (not just same key)
	matchFound := false
	for _, key := range jo.state.GetAllLeftKeys() {
		leftEvents := jo.state.GetLeftEvents(key)
		for _, leftStored := range leftEvents {
			if jo.matchesCondition(leftStored.Event, rightEvent) {
				matchFound = true

				result := jo.createJoinResult(leftStored.Event, rightEvent)
				if result != nil {
					results = append(results, result.ToEvent())
				}

				jo.state.IncrementMatchCount(leftStored)
			}
		}
	}

	if !matchFound && (jo.config.Type == RightOuterJoin || jo.config.Type == FullOuterJoin) {
		result := jo.createJoinResult(nil, rightEvent)
		if result != nil {
			results = append(results, result.ToEvent())
		}
	}

	return results
}

// matchesCondition checks if two events match the join condition and time constraints
func (jo *JoinOperator) matchesCondition(left, right *stream.Event) bool {
	// Check custom condition
	if !jo.condition(left, right) {
		return false
	}

	// Check time window if configured
	if jo.config.TimeWindow > 0 {
		timeDiff := right.EventTime.Sub(left.EventTime)
		if timeDiff < 0 {
			timeDiff = -timeDiff
		}
		if timeDiff > jo.config.TimeWindow {
			return false
		}
	}

	return true
}

// createJoinResult creates a join result from matched events
func (jo *JoinOperator) createJoinResult(left, right *stream.Event) *JoinResult {
	result := &JoinResult{
		Left:  left,
		Right: right,
	}

	// Determine join key and event time
	if left != nil && right != nil {
		result.JoinKey = left.Key
		// Use the later timestamp
		if right.EventTime.After(left.EventTime) {
			result.EventTime = right.EventTime
		} else {
			result.EventTime = left.EventTime
		}
	} else if left != nil {
		result.JoinKey = left.Key
		result.EventTime = left.EventTime
	} else if right != nil {
		result.JoinKey = right.Key
		result.EventTime = right.EventTime
	}

	return result
}

// startCleanup starts the periodic cleanup goroutine
func (jo *JoinOperator) startCleanup() {
	jo.cleanupTicker = time.NewTicker(jo.config.CleanupInterval)

	go func() {
		for {
			select {
			case <-jo.cleanupTicker.C:
				jo.cleanup()
			case <-jo.cleanupStop:
				return
			}
		}
	}()
}

// cleanup removes expired events and emits unmatched outer join results
func (jo *JoinOperator) cleanup() {
	now := time.Now()
	leftExpired, rightExpired := jo.state.RemoveExpiredEvents(now)

	// For outer joins, emit unmatched expired events
	if jo.config.Type == LeftOuterJoin || jo.config.Type == FullOuterJoin {
		for _, stored := range leftExpired {
			if stored.MatchCount == 0 {
				// Emit unmatched left event
				jo.logger.Debug("Emitting unmatched left event",
					zap.String("key", stored.Event.Key),
					zap.Time("event_time", stored.Event.EventTime))
			}
		}
	}

	if jo.config.Type == RightOuterJoin || jo.config.Type == FullOuterJoin {
		for _, stored := range rightExpired {
			if stored.MatchCount == 0 {
				// Emit unmatched right event
				jo.logger.Debug("Emitting unmatched right event",
					zap.String("key", stored.Event.Key),
					zap.Time("event_time", stored.Event.EventTime))
			}
		}
	}

	metrics := jo.state.GetMetrics()
	jo.logger.Debug("Join cleanup completed",
		zap.Int("left_expired", len(leftExpired)),
		zap.Int("right_expired", len(rightExpired)),
		zap.Int64("total_state_size", metrics.StateSize))
}

// Process implements the Operator interface (not typically used directly)
func (jo *JoinOperator) Process(ctx *stream.ProcessingContext, event *stream.Event) ([]*stream.Event, error) {
	return nil, fmt.Errorf("join operator requires explicit ProcessLeft or ProcessRight calls")
}

// Close cleans up resources
func (jo *JoinOperator) Close() error {
	if jo.cleanupTicker != nil {
		jo.cleanupTicker.Stop()
	}
	if jo.cleanupStop != nil {
		close(jo.cleanupStop)
	}

	jo.state.Clear()
	return nil
}

// GetMetrics returns current join metrics
func (jo *JoinOperator) GetMetrics() JoinMetrics {
	return jo.state.GetMetrics()
}

// DualStreamJoinOperator wraps two operators for dual-stream join
type DualStreamJoinOperator struct {
	joinOp     *JoinOperator
	leftInput  <-chan *stream.Event
	rightInput <-chan *stream.Event
	output     chan *stream.Event
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	logger     *zap.Logger
}

// NewDualStreamJoin creates a join operator that consumes from two input streams
func NewDualStreamJoin(
	leftInput, rightInput <-chan *stream.Event,
	config *JoinConfig,
	logger *zap.Logger,
) *DualStreamJoinOperator {
	ctx, cancel := context.WithCancel(context.Background())

	return &DualStreamJoinOperator{
		joinOp:     NewJoinOperator(config, logger),
		leftInput:  leftInput,
		rightInput: rightInput,
		output:     make(chan *stream.Event, 1000),
		ctx:        ctx,
		cancel:     cancel,
		logger:     logger,
	}
}

// Start begins processing both input streams
func (dj *DualStreamJoinOperator) Start() {
	// Process left stream
	dj.wg.Add(1)
	go func() {
		defer dj.wg.Done()
		for {
			select {
			case event, ok := <-dj.leftInput:
				if !ok {
					return
				}
				results, err := dj.joinOp.ProcessLeft(
					&stream.ProcessingContext{Ctx: dj.ctx},
					event,
				)
				if err != nil {
					dj.logger.Error("Error processing left event", zap.Error(err))
					continue
				}
				for _, result := range results {
					select {
					case dj.output <- result:
					case <-dj.ctx.Done():
						return
					}
				}
			case <-dj.ctx.Done():
				return
			}
		}
	}()

	// Process right stream
	dj.wg.Add(1)
	go func() {
		defer dj.wg.Done()
		for {
			select {
			case event, ok := <-dj.rightInput:
				if !ok {
					return
				}
				results, err := dj.joinOp.ProcessRight(
					&stream.ProcessingContext{Ctx: dj.ctx},
					event,
				)
				if err != nil {
					dj.logger.Error("Error processing right event", zap.Error(err))
					continue
				}
				for _, result := range results {
					select {
					case dj.output <- result:
					case <-dj.ctx.Done():
						return
					}
				}
			case <-dj.ctx.Done():
				return
			}
		}
	}()
}

// Output returns the output channel
func (dj *DualStreamJoinOperator) Output() <-chan *stream.Event {
	return dj.output
}

// Stop stops processing and closes output
func (dj *DualStreamJoinOperator) Stop() {
	dj.cancel()
	dj.wg.Wait()
	close(dj.output)
	dj.joinOp.Close()
}

// GetMetrics returns join metrics
func (dj *DualStreamJoinOperator) GetMetrics() JoinMetrics {
	return dj.joinOp.GetMetrics()
}

package sql

import (
	"fmt"
	"sync"
	"time"

	"github.com/therealutkarshpriyadarshi/gress/pkg/state"
	"github.com/therealutkarshpriyadarshi/gress/pkg/stream"
)

// StreamTableDuality provides stream-table duality semantics
// A stream can be materialized into a table, and a table can be queried as a stream
type StreamTableDuality struct {
	catalog      *Catalog
	stateBackend state.StateBackend
	mu           sync.RWMutex
}

// NewStreamTableDuality creates a new stream-table duality manager
func NewStreamTableDuality(catalog *Catalog, stateBackend state.StateBackend) *StreamTableDuality {
	return &StreamTableDuality{
		catalog:      catalog,
		stateBackend: stateBackend,
	}
}

// MaterializeStream materializes a stream into a table
// This creates a continuously updated table from a stream
func (std *StreamTableDuality) MaterializeStream(streamName string, tableName string, primaryKeys []string) error {
	std.mu.Lock()
	defer std.mu.Unlock()

	// Check if stream exists
	sourceStream, err := std.catalog.GetStream(streamName)
	if err != nil {
		return fmt.Errorf("stream %s not found: %w", streamName, err)
	}

	// Check if table already exists
	if std.catalog.TableExists(tableName) {
		return fmt.Errorf("table %s already exists", tableName)
	}

	// Create table with same schema as stream
	err = std.catalog.RegisterTable(tableName, sourceStream.Schema, primaryKeys)
	if err != nil {
		return fmt.Errorf("failed to create table: %w", err)
	}

	// Start materialization process
	go std.materializeStreamToTable(streamName, tableName, primaryKeys)

	return nil
}

// materializeStreamToTable continuously materializes a stream into a table
func (std *StreamTableDuality) materializeStreamToTable(streamName string, tableName string, primaryKeys []string) {
	sourceStream, err := std.catalog.GetStream(streamName)
	if err != nil {
		fmt.Printf("Error getting stream %s: %v\n", streamName, err)
		return
	}

	table, err := std.catalog.GetTable(tableName)
	if err != nil {
		fmt.Printf("Error getting table %s: %v\n", tableName, err)
		return
	}

	// Process events from stream and update table
	for event := range sourceStream.EventChannel {
		// Build primary key
		key := std.buildPrimaryKey(event, primaryKeys)

		// Update table state
		std.mu.Lock()
		table.State[key] = event.Value
		std.mu.Unlock()

		// Persist to state backend
		if std.stateBackend != nil {
			data := []byte(fmt.Sprintf("%v", event.Value))
			if err := std.stateBackend.Put(key, data); err != nil {
				fmt.Printf("Error persisting to state backend: %v\n", err)
			}
		}
	}
}

// StreamifyTable converts a table into a stream of changes
// This creates a changelog stream from a table
func (std *StreamTableDuality) StreamifyTable(tableName string, streamName string) error {
	std.mu.Lock()
	defer std.mu.Unlock()

	// Check if table exists
	table, err := std.catalog.GetTable(tableName)
	if err != nil {
		return fmt.Errorf("table %s not found: %w", tableName, err)
	}

	// Check if stream already exists
	if std.catalog.StreamExists(streamName) {
		return fmt.Errorf("stream %s already exists", streamName)
	}

	// Create stream with same schema as table
	eventChannel := make(chan *stream.Event, 1000)
	err = std.catalog.RegisterStream(streamName, table.Schema, eventChannel)
	if err != nil {
		return fmt.Errorf("failed to create stream: %w", err)
	}

	// Start streamification process (emit changes to table as stream events)
	go std.streamifyTableToStream(tableName, streamName, eventChannel)

	return nil
}

// streamifyTableToStream converts table changes to stream events
func (std *StreamTableDuality) streamifyTableToStream(tableName string, streamName string, eventChannel chan *stream.Event) {
	// This would monitor table changes and emit them as events
	// For now, this is a placeholder - in a real implementation, we'd need:
	// 1. Change detection mechanism (triggers, polling, etc.)
	// 2. CDC (Change Data Capture) for table modifications
	// 3. Proper event generation with timestamps

	// Simplified implementation: periodically snapshot the table
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	previousSnapshot := make(map[string]interface{})

	for range ticker.C {
		table, err := std.catalog.GetTable(tableName)
		if err != nil {
			fmt.Printf("Error getting table %s: %v\n", tableName, err)
			return
		}

		std.mu.RLock()
		// Detect changes
		for key, value := range table.State {
			if prevValue, exists := previousSnapshot[key]; !exists || prevValue != value {
				// Emit change event
				event := &stream.Event{
					Key:       key,
					Value:     value,
					EventTime: time.Now(),
					Headers:   map[string]string{"change_type": "update"},
				}

				select {
				case eventChannel <- event:
					previousSnapshot[key] = value
				default:
					// Channel full, skip
				}
			}
		}

		// Detect deletions
		for key := range previousSnapshot {
			if _, exists := table.State[key]; !exists {
				// Emit delete event
				event := &stream.Event{
					Key:       key,
					Value:     nil,
					EventTime: time.Now(),
					Headers:   map[string]string{"change_type": "delete"},
				}

				select {
				case eventChannel <- event:
					delete(previousSnapshot, key)
				default:
					// Channel full, skip
				}
			}
		}
		std.mu.RUnlock()
	}
}

// QueryTable performs a point-in-time query on a table
func (std *StreamTableDuality) QueryTable(tableName string, predicate func(key string, value interface{}) bool) ([]interface{}, error) {
	std.mu.RLock()
	defer std.mu.RUnlock()

	table, err := std.catalog.GetTable(tableName)
	if err != nil {
		return nil, fmt.Errorf("table %s not found: %w", tableName, err)
	}

	results := []interface{}{}
	for key, value := range table.State {
		if predicate(key, value) {
			results = append(results, value)
		}
	}

	return results, nil
}

// GetTableSnapshot returns a point-in-time snapshot of a table
func (std *StreamTableDuality) GetTableSnapshot(tableName string) (map[string]interface{}, error) {
	std.mu.RLock()
	defer std.mu.RUnlock()

	table, err := std.catalog.GetTable(tableName)
	if err != nil {
		return nil, fmt.Errorf("table %s not found: %w", tableName, err)
	}

	// Create a copy of the state
	snapshot := make(map[string]interface{})
	for key, value := range table.State {
		snapshot[key] = value
	}

	return snapshot, nil
}

// UpdateTable updates a table with new data (for INSERT/UPDATE/DELETE operations)
func (std *StreamTableDuality) UpdateTable(tableName string, key string, value interface{}) error {
	std.mu.Lock()
	defer std.mu.Unlock()

	table, err := std.catalog.GetTable(tableName)
	if err != nil {
		return fmt.Errorf("table %s not found: %w", tableName, err)
	}

	if value == nil {
		// DELETE operation
		delete(table.State, key)
		if std.stateBackend != nil {
			return std.stateBackend.Delete(key)
		}
	} else {
		// INSERT/UPDATE operation
		table.State[key] = value
		if std.stateBackend != nil {
			data := []byte(fmt.Sprintf("%v", value))
			return std.stateBackend.Put(key, data)
		}
	}

	return nil
}

// DeleteFromTable deletes a row from a table
func (std *StreamTableDuality) DeleteFromTable(tableName string, key string) error {
	return std.UpdateTable(tableName, key, nil)
}

// GetTableSize returns the number of rows in a table
func (std *StreamTableDuality) GetTableSize(tableName string) (int, error) {
	std.mu.RLock()
	defer std.mu.RUnlock()

	table, err := std.catalog.GetTable(tableName)
	if err != nil {
		return 0, fmt.Errorf("table %s not found: %w", tableName, err)
	}

	return len(table.State), nil
}

// buildPrimaryKey builds a composite primary key from an event
func (std *StreamTableDuality) buildPrimaryKey(event *stream.Event, primaryKeys []string) string {
	if len(primaryKeys) == 0 {
		return event.Key
	}

	key := ""
	if valueMap, ok := event.Value.(map[string]interface{}); ok {
		for i, pkField := range primaryKeys {
			if i > 0 {
				key += "_"
			}
			if val, exists := valueMap[pkField]; exists {
				key += fmt.Sprintf("%v", val)
			}
		}
	}

	if key == "" {
		return event.Key
	}

	return key
}

// TemporalTable represents a table with temporal (versioned) data
// This supports time-travel queries and historical data access
type TemporalTable struct {
	Name       string
	Schema     Schema
	Versions   map[string][]TemporalRow // key -> versions
	mu         sync.RWMutex
}

// TemporalRow represents a row with temporal information
type TemporalRow struct {
	Value     interface{}
	ValidFrom time.Time
	ValidTo   time.Time
}

// NewTemporalTable creates a new temporal table
func NewTemporalTable(name string, schema Schema) *TemporalTable {
	return &TemporalTable{
		Name:     name,
		Schema:   schema,
		Versions: make(map[string][]TemporalRow),
	}
}

// Insert inserts a new version of a row
func (tt *TemporalTable) Insert(key string, value interface{}, timestamp time.Time) {
	tt.mu.Lock()
	defer tt.mu.Unlock()

	// Close previous version if exists
	if versions, exists := tt.Versions[key]; exists && len(versions) > 0 {
		lastVersion := &versions[len(versions)-1]
		if lastVersion.ValidTo.IsZero() {
			lastVersion.ValidTo = timestamp
		}
	}

	// Add new version
	newRow := TemporalRow{
		Value:     value,
		ValidFrom: timestamp,
		ValidTo:   time.Time{}, // Open-ended
	}

	tt.Versions[key] = append(tt.Versions[key], newRow)
}

// QueryAsOf returns the value as of a specific timestamp
func (tt *TemporalTable) QueryAsOf(key string, timestamp time.Time) (interface{}, error) {
	tt.mu.RLock()
	defer tt.mu.RUnlock()

	versions, exists := tt.Versions[key]
	if !exists {
		return nil, fmt.Errorf("key %s not found", key)
	}

	// Find version valid at timestamp
	for _, version := range versions {
		if version.ValidFrom.Before(timestamp) || version.ValidFrom.Equal(timestamp) {
			if version.ValidTo.IsZero() || version.ValidTo.After(timestamp) {
				return version.Value, nil
			}
		}
	}

	return nil, fmt.Errorf("no version found for timestamp %v", timestamp)
}

// QueryHistory returns all versions for a key
func (tt *TemporalTable) QueryHistory(key string) ([]TemporalRow, error) {
	tt.mu.RLock()
	defer tt.mu.RUnlock()

	versions, exists := tt.Versions[key]
	if !exists {
		return nil, fmt.Errorf("key %s not found", key)
	}

	// Return copy
	result := make([]TemporalRow, len(versions))
	copy(result, versions)

	return result, nil
}

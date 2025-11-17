package sink

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/lib/pq"
	"github.com/therealutkarshpriyadarshi/gress/pkg/stream"
	"go.uber.org/zap"
)

// TimescaleSink writes events to TimescaleDB
type TimescaleSink struct {
	db         *sql.DB
	table      string
	logger     *zap.Logger
	batchSize  int
	batch      []*stream.Event
	flushTimer *time.Timer
}

// TimescaleConfig holds TimescaleDB sink configuration
type TimescaleConfig struct {
	Host      string
	Port      int
	Database  string
	User      string
	Password  string
	Table     string
	BatchSize int
}

// NewTimescaleSink creates a new TimescaleDB sink
func NewTimescaleSink(config TimescaleConfig, logger *zap.Logger) (*TimescaleSink, error) {
	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		config.Host, config.Port, config.User, config.Password, config.Database)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to TimescaleDB: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping TimescaleDB: %w", err)
	}

	if config.BatchSize == 0 {
		config.BatchSize = 100
	}

	sink := &TimescaleSink{
		db:        db,
		table:     config.Table,
		logger:    logger,
		batchSize: config.BatchSize,
		batch:     make([]*stream.Event, 0, config.BatchSize),
	}

	// Create table if it doesn't exist
	if err := sink.createTable(); err != nil {
		return nil, err
	}

	return sink, nil
}

// createTable creates the events table if it doesn't exist
func (t *TimescaleSink) createTable() error {
	query := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			time        TIMESTAMPTZ NOT NULL,
			key         TEXT,
			value       JSONB,
			partition   INTEGER,
			offset      BIGINT,
			headers     JSONB
		);

		SELECT create_hypertable('%s', 'time', if_not_exists => TRUE);

		CREATE INDEX IF NOT EXISTS idx_%s_key ON %s (key, time DESC);
		CREATE INDEX IF NOT EXISTS idx_%s_value ON %s USING GIN (value);
	`, t.table, t.table, t.table, t.table, t.table, t.table)

	_, err := t.db.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to create table: %w", err)
	}

	t.logger.Info("TimescaleDB table ready", zap.String("table", t.table))
	return nil
}

// Write adds an event to the batch
func (t *TimescaleSink) Write(ctx context.Context, event *stream.Event) error {
	t.batch = append(t.batch, event)

	if len(t.batch) >= t.batchSize {
		return t.Flush(ctx)
	}

	return nil
}

// Flush writes the batch to TimescaleDB
func (t *TimescaleSink) Flush(ctx context.Context) error {
	if len(t.batch) == 0 {
		return nil
	}

	t.logger.Debug("Flushing batch to TimescaleDB", zap.Int("count", len(t.batch)))

	tx, err := t.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, fmt.Sprintf(
		`INSERT INTO %s (time, key, value, partition, offset, headers) VALUES ($1, $2, $3, $4, $5, $6)`,
		t.table))
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, event := range t.batch {
		valueJSON, err := json.Marshal(event.Value)
		if err != nil {
			t.logger.Warn("Failed to marshal event value", zap.Error(err))
			continue
		}

		headersJSON, err := json.Marshal(event.Headers)
		if err != nil {
			headersJSON = []byte("{}")
		}

		_, err = stmt.ExecContext(ctx,
			event.EventTime,
			event.Key,
			valueJSON,
			event.Partition,
			event.Offset,
			headersJSON,
		)
		if err != nil {
			t.logger.Error("Failed to insert event", zap.Error(err))
			continue
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	t.logger.Info("Batch flushed to TimescaleDB",
		zap.Int("count", len(t.batch)),
		zap.String("table", t.table))

	t.batch = t.batch[:0]
	return nil
}

// Close closes the database connection
func (t *TimescaleSink) Close() error {
	t.logger.Info("Closing TimescaleDB sink")
	return t.db.Close()
}

// Query executes a query on the TimescaleDB table
func (t *TimescaleSink) Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return t.db.QueryContext(ctx, query, args...)
}

-- Initialize TimescaleDB extension
CREATE EXTENSION IF NOT EXISTS timescaledb;

-- Create events table
CREATE TABLE IF NOT EXISTS events (
    time        TIMESTAMPTZ NOT NULL,
    key         TEXT,
    value       JSONB,
    partition   INTEGER,
    offset      BIGINT,
    headers     JSONB
);

-- Create hypertable
SELECT create_hypertable('events', 'time', if_not_exists => TRUE);

-- Create indexes
CREATE INDEX IF NOT EXISTS idx_events_key ON events (key, time DESC);
CREATE INDEX IF NOT EXISTS idx_events_value ON events USING GIN (value);

-- Create continuous aggregates for real-time analytics
CREATE MATERIALIZED VIEW IF NOT EXISTS events_1min
WITH (timescaledb.continuous) AS
SELECT
    time_bucket('1 minute', time) AS bucket,
    key,
    COUNT(*) AS event_count,
    AVG((value->>'value')::NUMERIC) AS avg_value
FROM events
GROUP BY bucket, key
WITH NO DATA;

-- Refresh policy for continuous aggregate
SELECT add_continuous_aggregate_policy('events_1min',
    start_offset => INTERVAL '1 hour',
    end_offset => INTERVAL '1 minute',
    schedule_interval => INTERVAL '1 minute',
    if_not_exists => TRUE);

-- Create retention policy (keep data for 30 days)
SELECT add_retention_policy('events', INTERVAL '30 days', if_not_exists => TRUE);

-- Create metrics table
CREATE TABLE IF NOT EXISTS metrics (
    time                TIMESTAMPTZ NOT NULL,
    metric_name         TEXT NOT NULL,
    metric_value        DOUBLE PRECISION,
    labels              JSONB,
    PRIMARY KEY (time, metric_name)
);

SELECT create_hypertable('metrics', 'time', if_not_exists => TRUE);

-- Create pricing updates table for ride-sharing example
CREATE TABLE IF NOT EXISTS pricing_updates (
    time                TIMESTAMPTZ NOT NULL,
    area                TEXT NOT NULL,
    surge_multiplier    DOUBLE PRECISION,
    demand_supply_ratio DOUBLE PRECISION,
    ride_requests       INTEGER,
    available_drivers   INTEGER
);

SELECT create_hypertable('pricing_updates', 'time', if_not_exists => TRUE);
CREATE INDEX IF NOT EXISTS idx_pricing_area ON pricing_updates (area, time DESC);

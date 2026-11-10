-- +goose Up

-- Telemetry is append-only and read by robot and time range; TimescaleDB
-- splits the measurements table into chunks by time, so such a read only
-- touches the chunks of its range.
CREATE EXTENSION IF NOT EXISTS timescaledb;

-- One robot's state at one moment. The primary key is the contract's
-- idempotency key: a measurement sent twice is stored once. measured_at is
-- the robot's clock; received_at is the system's.
CREATE TABLE measurements (
    robot_id           uuid             NOT NULL REFERENCES robots (id),
    measured_at        timestamptz      NOT NULL,
    received_at        timestamptz      NOT NULL,
    battery_pct        double precision,
    -- The position is all null when it was not reported.
    map_id             text,
    x                  double precision,
    y                  double precision,
    heading_rad        double precision,
    activity           text             NOT NULL,
    active_fault_codes text[]           NOT NULL,
    PRIMARY KEY (robot_id, measured_at)
);

SELECT create_hypertable('measurements', 'measured_at');

-- One occurrence of a fault, reported once when the robot detects it.
-- Faults are rare, so this is a plain table. The primary key is the
-- contract's idempotency key.
CREATE TABLE faults (
    robot_id    uuid        NOT NULL REFERENCES robots (id),
    code        text        NOT NULL,
    occurred_at timestamptz NOT NULL,
    received_at timestamptz NOT NULL,
    severity    text        NOT NULL,
    message     text        NOT NULL,
    PRIMARY KEY (robot_id, code, occurred_at)
);

-- A robot's faults are read by time range.
CREATE INDEX faults_robot_id_occurred_at_idx ON faults (robot_id, occurred_at);

-- +goose Down

DROP TABLE faults;
DROP TABLE measurements;

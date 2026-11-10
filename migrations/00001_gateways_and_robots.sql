-- +goose Up

-- A gateway is registered by an admin; its serial number is the business key.
CREATE TABLE gateways (
    id            uuid        PRIMARY KEY,
    serial_number text        NOT NULL UNIQUE,
    location      text        NOT NULL,
    name          text,
    registered_at timestamptz NOT NULL,
    last_seen_at  timestamptz
);

-- A robot is discovered by its gateway's scan and never moves to another
-- gateway. Its serial number is unique across the fleet.
CREATE TABLE robots (
    id            uuid        PRIMARY KEY,
    serial_number text        NOT NULL UNIQUE,
    gateway_id    uuid        NOT NULL REFERENCES gateways (id),
    first_seen_at timestamptz NOT NULL,
    last_seen_at  timestamptz
);

-- A gateway's robots are listed oldest first.
CREATE INDEX robots_gateway_id_first_seen_at_idx ON robots (gateway_id, first_seen_at);

-- +goose Down

DROP TABLE robots;
DROP TABLE gateways;

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE master_nodes (
    id text PRIMARY KEY,
    node_label text NOT NULL DEFAULT '',
    credential_username text UNIQUE,
    enrollment_token_hash bytea,
    enrolled_at timestamptz,
    disabled_at timestamptz,
    last_seen_at timestamptz
);

CREATE TABLE pots (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    node_label text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE slave_nodes (
    id text PRIMARY KEY,
    master_id text REFERENCES master_nodes(id),
    pot_id uuid NOT NULL UNIQUE REFERENCES pots(id),
    node_label text NOT NULL DEFAULT '',
    transfer_token_hash bytea NOT NULL,
    disabled_at timestamptz,
    last_seen_at timestamptz
);

CREATE TABLE telemetry_batches (
    master_id text NOT NULL REFERENCES master_nodes(id),
    message_id text NOT NULL,
    measured_at timestamptz NOT NULL,
    received_at timestamptz NOT NULL DEFAULT now(),
    firmware_version text NOT NULL,
    PRIMARY KEY (master_id, message_id)
);

CREATE TABLE measurements (
    slave_id text NOT NULL REFERENCES slave_nodes(id),
    measured_at timestamptz NOT NULL,
    received_at timestamptz NOT NULL DEFAULT now(),
    ph double precision NOT NULL CHECK (ph >= 0 AND ph <= 14),
    ec_ms_per_cm double precision NOT NULL CHECK (ec_ms_per_cm >= 0),
    light_lux double precision NOT NULL CHECK (light_lux >= 0),
    soil_moisture_percent double precision NOT NULL CHECK (soil_moisture_percent >= 0 AND soil_moisture_percent <= 100),
    calibration_version text NOT NULL,
    firmware_version text NOT NULL,
    PRIMARY KEY (slave_id, measured_at)
);

CREATE INDEX measurements_measured_at_idx ON measurements (measured_at);

-- +goose Up
CREATE TYPE idempotency_status AS ENUM ('processing', 'completed');

CREATE TABLE idempotency_keys (
    id UUID PRIMARY KEY,
    request_hash BYTEA NOT NULL,
    trip_id UUID NOT NULL,
    status idempotency_status NOT NULL DEFAULT 'processing',
    created_at TIMESTAMPTZ NOT NULL
);

-- +goose Down
DROP TABLE idempotency_keys;
DROP TYPE idempotency_status;
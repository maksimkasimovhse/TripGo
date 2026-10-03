-- +goose Up
CREATE TABLE idempotency_keys (
    id UUID PRIMARY KEY,
    request_hash BYTEA,
    trip_id UUID,
    created_at TIMESTAMPTZ NOT NULL
);

-- +goose Down
DROP TABLE idempotency_keys;

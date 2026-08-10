-- +goose Up
CREATE TABLE if NOT EXISTS outbox(
    id BIGSERIAL PRIMARY KEY,
    topic TEXT NOT NULL,
    key TEXT NOT NULL,
    value JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at TIMESTAMPTZ  DEFAULT NULL
);

CREATE INDEX IF NOT EXISTS idx_outbox_unpublished ON outbox (id) WHERE published_at IS NULL;


-- +goose Down
DROP TABLE IF EXISTS outbox;
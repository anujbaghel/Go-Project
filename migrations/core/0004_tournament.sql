-- +goose Up
CREATE TABLE IF NOT EXISTS tournaments (
    ltid                BIGSERIAL PRIMARY KEY,
    name                TEXT        NOT NULL,
    entry_fee           BIGINT      NOT NULL,
    status              TEXT        NOT NULL DEFAULT 'OPEN',
    instant_min_score   INT,
    is_instant_efc      BOOLEAN NOT NULL DEFAULT false,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS registrations (
    id                  BIGSERIAL   PRIMARY KEY,
    ltid                BIGINT      NOT NULL REFERENCES tournaments(ltid),
    uid                 BIGINT      NOT NULL,
    entry_fee           BIGINT      NOT NULL,
    score               INT         NOT NULL DEFAULT 0,
    bucket_slot         INT         NOT NULL,
    refunded_on         TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE  (ltid, uid) -- single registration per user tournament
);

-- the worker will sweep registrations by (ltid, bucket_slot), so index it
CREATE INDEX IF NOT EXISTS idx_reg_ltid_bucket ON registrations (ltid, bucket_slot);

-- +goose Down
DROP TABLE IF EXISTS tournaments;
DROP TABLE IF EXISTS registrations;
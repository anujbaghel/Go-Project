--Wallets, Schema
-- +goose Up
CREATE TABLE IF NOT EXISTS wallets (
    uid             BIGINT  PRIMARY KEY,
    shar_id         INT     NOT NULL,
    unlock_coins    BIGINT  NOT NULL DEFAULT 0,
    cashback_coins  BIGINT  NOT NULL DEFAULT 0,
    added_coins     BIGINT  NOT NULL DEFAULT 0,
    actual_coinst   BIGINT  NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS passbook (
    tid             BIGSERIAL   PRIMARY KEY,
    uid             BIGINT      NOT NULL,
    constraint_id   TEXT        NOT NULL,
    type            TEXT        NOT NULL,
    amount          BIGINT      NOT NULL,
    revert_tid      BIGSERIAL   NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(uid, constraint_id)
);

-- +goose Down
DROP TABLE IF EXISTS passbook;
DROP TABLE IF EXISTS wallets;
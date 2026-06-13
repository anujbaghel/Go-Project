-- +goose Up
ALTER TABLE wallets RENAME COLUMN shar_id TO shard_id;
ALTER TABLE wallets RENAME COLUMN actual_coinst TO actual_coins;

-- +goose Down
ALTER TABLE wallets RENAME COLUMN shard_id    TO shar_id;
  ALTER TABLE wallets RENAME COLUMN actual_coins TO actual_coinst;
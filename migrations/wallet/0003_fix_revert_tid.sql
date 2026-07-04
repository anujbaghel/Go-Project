-- +goose Up
ALTER TABLE passbook ALTER COLUMN revert_tid DROP NOT NULL;
ALTER TABLE passbook ALTER COLUMN revert_tid DROP DEFAULT;
ALTER TABLE passbook ALTER COLUMN revert_tid TYPE BIGINT;

-- wipe the bogus sequence-generated values (none are real reverts)
UPDATE passbook SET revert_tid = NULL;

-- optional but correct: a refund row must reference a real ledger row
ALTER TABLE passbook ADD CONSTRAINT passbook_revert_fk
    FOREIGN KEY (revert_tid) REFERENCES passbook(tid);

-- +goose Down
ALTER TABLE passbook DROP CONSTRAINT passbook_revert_fk;
ALTER TABLE passbook ALTER COLUMN revert_tid SET NOT NULL;
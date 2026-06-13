package wallet

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const NumVirtualShards = 1024

func ShardID(uid int64) int { return int(uid%NumVirtualShards) / 256 }

type Wallet struct {
	pool *pgxpool.Pool // pointer - all callers share one wallet service
}

var ErrInsufficientFunds = errors.New("Insufficient coins")

func New(pool *pgxpool.Pool) *Wallet { return &Wallet{pool: pool} }

// Ensure Wallet inserts a zero-balance wallet row if the user has none
func (s *Wallet) EnsureWallet(ctx context.Context, uid int64) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO wallets (uid, shard_id) VALUES ($1, $2)
		ON CONFLICT (uid) DO NOTHING`,
		uid,
		ShardID(uid))
	return err
}

// Balance returns the total accross all buckets
func (s *Wallet) Balance(ctx context.Context, uid int64) (int64, error) {
	var unlock, cashback, actualCoins, addedCoins int64
	err := s.pool.QueryRow(ctx,
		`SELECT unlock_coins, cashback_coins, added_coins, actual_coins FROM wallets WHERE uid = $1`, uid).Scan(&unlock, &cashback, &addedCoins, &actualCoins)
	if err != nil {
		return 0, err
	}
	return unlock + cashback + actualCoins + addedCoins, nil
}

func (s *Wallet) CREDIT(ctx context.Context, uid int64, amnt int64, constraintID string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) // Safe : a rollback after successfull commit is a no-op

	_, err = tx.Exec(ctx,
		`INSERT INTO passbook (uid, constraint_id, type, amount)
	 VALUES ($1, $2, $3, $4)`,
		uid,
		constraintID,
		"WINNING_CREDT",
		amnt)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" { // 23505 = unique_violation
			return nil // ALREADY APPLIED earlier → idempotent SUCCESS, not an error
		}
		return err // Some Other DB failure
	}

	_, err = tx.Exec(ctx,
		`UPDATE wallets SET actual_coins = actual_coins + $1 WHERE uid = $2`, amnt, uid)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Wallet) DEBIT(ctx context.Context, uid int64, debitAmnt int64, constraintId string) error {
	if debitAmnt <= 0 {
		return errors.New("amount must be positive")
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `INSERT INTO passbook (uid, constraint_id, type, amount)
						VALUES ($1, $2, $3, $4)`,
		uid, constraintId, "ENTRY_FEE_DEBIT", debitAmnt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil // Already Debited -> Idempotent success
		}
		return err
	}

	// Lock the Wallet row for the rest of transaction
	// A concurrent Debit's "FOR UPDATE"
	// untill we commit/ Rollback
	var unlock, cashback, added, actual int64
	err = tx.QueryRow(ctx, `SELECT unlock_coins, cashback_coins, added_coins, actual_coins
						FROM wallets WHERE uid = $1 FOR UPDATE`,
		uid).Scan(&unlock, &cashback, &added, &actual)
	if err != nil {
		return err
	}

	// Never Negative : reject before mutating anything
	if unlock+cashback+added+actual < debitAmnt {
		return ErrInsufficientFunds
	}

	// Walk buckets in PRIORITY ORDER, talking min (bucket, remaining) from each
	remaining := debitAmnt
	take := func(bucket *int64) { // Pointer so it mutates the local in place
		t := *bucket
		if t > remaining {
			t = remaining
		}
		*bucket -= t
		remaining -= t
	}
	take(&unlock) // Promo
	take(&cashback)
	take(&added)  // deposits
	take(&actual) // Winning last (TDS - liable in the real system)

	// persist the new bucket values
	_, err = tx.Exec(ctx,
		`UPDATE wallets
				SET unlock_coins = $1,
				cashback_coins = $2,
				added_coins = $3,
				actual_coins = $4 WHERE uid = $5`,
		unlock, cashback, added, actual, uid)

	if err != nil {
		return err
	}

	// Debit Successfull
	return tx.Commit(ctx)
}

func (s *Wallet) REVERT(ctx context.Context, originalTid int64, constraintId string, ignoreIfReverted bool) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Load + lock the original ledger now
	var uid, amount int64
	var revert_tid *int64 // nullable -> pointer; nil means "not yet reverted"

	err = tx.QueryRow(ctx, `SELECT uid, amount, revert_tid FROM passbook WHERE tid=$1 FOR UPDATE`, originalTid).Scan(&uid, &amount, &revert_tid)
	if err != nil {
		return err
	}

	if revert_tid != nil {
		// Already reversed
		if ignoreIfReverted {
			return nil
		}
		return errors.New("transaction already reverted")
	}

	// Insert new revert ID
	var refundTid int64
	err = tx.QueryRow(ctx,
		`INSERT INTO passbook (uid, constraint_id, type, amount) 
		VALUES ($1, $2, 'REFUND', $3) RETURNING tid`,
		uid, constraintId, amount).Scan(&refundTid)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil // Refund already recorded
		}
		return err
	}

	_, err = tx.Exec(ctx,
		`UPDATE wallets 
		SET added_coins = added_coins + $1 WHERE uid=$2`, amount, uid)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx,
		`UPDATE passbook SET
		revert_tid=$1 WHERE tid=$2`, refundTid, originalTid)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

package wallet

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestConcurrentDebit(t *testing.T) {
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, "postgres://app:app@localhost:5432/minif2p")
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	w := New(pool)

	const uid = 99999
	// fresh start: wipe this test user, give them exactly 1000 coins
	pool.Exec(ctx, `DELETE FROM passbook WHERE uid=$1`, uid)
	pool.Exec(ctx, `DELETE FROM wallets WHERE uid=$1`, uid)
	_ = w.EnsureWallet(ctx, uid)
	pool.Exec(ctx, `UPDATE wallets SET actual_coins=10000 WHERE uid=$1`, uid)
	// 20 goroutines each try to debit 100 from a balance of 1000.
	// UNIQUE constraintIds → we're testing the balance RACE, not idempotency.
	const N = 20
	var wg sync.WaitGroup
	var success int64
	for i := 0; i < N; i++ {
		wg.Add(1)
		t.Logf("Starting Go Routine %d", i)
		go func(i int) {
			defer wg.Done()
			cid := fmt.Sprintf("debit-test:%d", i)
			t.Logf("Executing Go Routine %d with cid = %s", i, cid)
			if err := w.DEBIT(ctx, uid, 1000, cid); err == nil {
				atomic.AddInt64(&success, 1)
			}
		}(i)
	}
	wg.Wait()

	bal, _ := w.Balance(ctx, uid)
	t.Logf("successes=%d final balance = %d", success, bal)

	if success != 10 {
		t.Fatalf("Expected exactly 10 successful Debits, got %d", success)
	}

	if bal != 0 {
		t.Fatalf("expected balance 0, got balance %d", bal)
	}
}

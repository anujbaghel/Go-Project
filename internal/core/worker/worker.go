package worker

import (
	"Go-project/internal/core/jobs"
	"Go-project/internal/platform/dynconfig"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/bsm/redislock"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

const slotWidth = 256 // 1024 / 256 = 4 slots :[0-255],[255-511],[512-767],[768-1023]

type Worker struct {
	pool   *pgxpool.Pool
	rdb    *redis.Client
	locker *redislock.Client
	client *asynq.Client
	cfg    *dynconfig.Config
}

func NEW(pool *pgxpool.Pool, rdb *redis.Client, client *asynq.Client, cfg *dynconfig.Config) *Worker {
	return &Worker{pool: pool, rdb: rdb, locker: redislock.New(rdb), client: client, cfg: cfg}
}

type tournament struct {
	ltid         int64
	status       string
	entryFee     int64
	instantMin   *int
	isInstantEFC bool
}

// instantEligibile
func instantEligibile(score int, t tournament) bool {
	return t.isInstantEFC && t.instantMin != nil && score >= *t.instantMin
}

// Dummy Reward
func rewardForScore(score int, _ tournament) int64 {
	switch {
	case score >= 200:
		return 500
	case score >= 150:
		return 300
	case score >= 100:
		return 150
	default:
		return 0
	}
}

func (w *Worker) getTournament(ctx context.Context, ltid int64) (tournament, error) {
	t := tournament{ltid: ltid}
	err := w.pool.QueryRow(ctx,
		`SELECT status, entry_fee, instant_min_score, is_instant_efc FROM tournaments
		WHERE ltid=$1`, ltid).Scan(&t.status, &t.entryFee, &t.instantMin, &t.isInstantEFC)
	return t, err
}

func (w *Worker) processTournament(ctx context.Context, ltid int64) error {
	t, err := w.getTournament(ctx, ltid)
	if err != nil {
		return err
	}
	if t.status != "RESULT" && t.status != "CANCELLED" {
		return fmt.Errorf("tournament %d not in payout state (status = %s)", ltid, t.status)
	}

	if t.status == "CANCELLED" && w.cfg.IsFlowRestricted("entryFeeRefund", ltid) {
		slog.Info("entryFeeRefund flow restricted - skipping", "ltid", ltid)
		return nil
	}

	errCount := 0
	for lo := 0; lo < 1024; lo += slotWidth {
		hi := lo + slotWidth - 1
		if err := w.processSlot(ctx, t, lo, hi); err != nil {
			errCount += 1
			slog.Error("slot failed", "ltid", ltid, "slot", fmt.Sprintf("%d-%d", lo, hi), "err", err)
		}
	}
	return nil
}

func (w *Worker) processSlot(ctx context.Context, t tournament, lo, hi int) error {
	lockKey := fmt.Sprintf("payout-lock:%d:%d-%d", t.ltid, lo, hi)
	lock, err := w.locker.Obtain(ctx, lockKey, 5*time.Minute, nil)
	if errors.Is(err, redislock.ErrNotObtained) {
		return nil // another replica owns this slot — skip, don't fight
	}
	if err != nil {
		return err
	}
	defer lock.Release(ctx)

	// 2) COMPLETION FLAG — makes re-running a finished slot a no-op.
	doneKey := fmt.Sprintf("payout-done:%d:%d-%d", t.ltid, lo, hi)
	if v, _ := w.rdb.Get(ctx, doneKey).Result(); v == "1" {
		return nil
	}

	// 3) Read this slot's registrations into memory FIRST (frees the DB connection
	//    before we issue per-row writes below — avoids "conn busy" with the cursor).
	type reg struct {
		id, uid, entryFee int64
		score             int
		refundedOn        *time.Time
	}
	rows, err := w.pool.Query(ctx,
		`SELECT id, uid, entry_fee, score, refunded_on FROM registrations
		WHERE ltid=$1 AND bucket_slot BETWEEN $2 AND $3`, t.ltid, lo, hi)
	if err != nil {
		return err
	}

	var regs []reg
	for rows.Next() {
		var r reg
		if err := rows.Scan(&r.id, &r.uid, &r.entryFee, &r.score, &r.refundedOn); err != nil {
			rows.Close()
			return err
		}
		regs = append(regs, r)
	}
	rows.Close()

	// 4) Decide + enqueue per registration.
	for _, r := range regs {
		if t.status == "CANCELLED" {
			// FLOW B : REFUND
			if r.refundedOn != nil || instantEligibile(r.score, t) {
				continue // already refunded, OR they'll  be paid the fee via winnings
			}
			if _, err := jobs.EnqueueCredit(w.client, jobs.CreditPayload{
				UID:          r.uid,
				Amount:       r.entryFee,
				ConstraintID: fmt.Sprintf("refund:%d:%d", t.ltid, r.uid),
			}); err != nil {
				return err
			}

			// mark AFTER enqueue; if we crash before this, the next run re-enqueues
			// (safe — the refund job is idempotent on its constraintId).
			if _, err := w.pool.Exec(ctx,
				`UPDATE registrations SET refunded_on = now() WHERE id = $1`, r.id); err != nil {
				return err
			}
		} else {
			// FLOW - C - WINNINGS
			win := rewardForScore(r.score, t)
			if instantEligibile(r.score, t) {
				win -= r.entryFee // already got the entry fee refund

				if win < 0 {
					win = 0
				}
			}
			if win > 0 {
				if _, err := jobs.EnqueueCredit(w.client, jobs.CreditPayload{
					UID:          r.uid,
					Amount:       win,
					ConstraintID: fmt.Sprintf("winning:%d:%d", t.ltid, r.uid),
				}); err != nil {
					return err
				}
			}
		}
	}
	return w.rdb.Set(ctx, doneKey, "1", time.Hour).Err()
}

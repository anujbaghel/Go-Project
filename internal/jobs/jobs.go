package jobs

import (
	"context"
	"encoding/json"

	"github.com/hibiken/asynq"
)

// The task type — a stable string identifier (like an HTTP route).
const TypeCreditWallet = "wallet:credit"

// The payload — what the job needs to do its work.
type CreditPayload struct {
	UID          int64  `json:"uid"`
	Amount       int64  `json:"amount"`
	ConstraintID string `json:"constraintId"`
}

type WalletCreditor interface {
	EnsureWallet(ctx context.Context, uid int64) error
	CREDIT(ctx context.Context, UID, Amount int64, ConstraintID string) error
}

func EnqueueCredit(client *asynq.Client, p CreditPayload) (string, error) {
	b, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	info, err := client.Enqueue(
		asynq.NewTask(TypeCreditWallet, b),
		asynq.MaxRetry(5), // after 5 failures -> atchived (a dead letter queue)
	)
	if err != nil {
		return "", err
	}
	return info.ID, nil
}

// ── CONSUMER ────────────────────────────────────────────────────────
// Handlers carries the dependencies the jobs need (here: the wallet).
type Handlers struct {
	wallet WalletCreditor
}

func NewHanlders(wc WalletCreditor) *Handlers {
	return &Handlers{wallet: wc}
}

func (h *Handlers) HandleCredit(ctx context.Context, t *asynq.Task) error {
	var p CreditPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		// A malformed payload will NEVER succeed → tell asynq not to retry it.
		return asynq.SkipRetry
	}
	if err := h.wallet.EnsureWallet(ctx, p.UID); err != nil {
		return err // transient error → asynq retries (returning a normal error = "retry me")
	}
	// THE PAYOFF: idempotent. If this job is delivered twice, the second call
	// hits the same constraintId → 23505 → no-op. Money moves exactly once.
	return h.wallet.CREDIT(ctx, p.UID, p.Amount, p.ConstraintID)
}

// Mux wires task types to handlers (the consumer's router).
func (h *Handlers) Mux() *asynq.ServeMux {
	mux := asynq.NewServeMux()
	mux.HandleFunc(TypeCreditWallet, h.HandleCredit)
	return mux
}

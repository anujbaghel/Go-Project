package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"Go-project/internal/platform/httpx"
	walletcontract "Go-project/internal/wallet/contract"

	"github.com/sony/gobreaker"
)

const (
	defaultCallTimeout = 2 * time.Second
	maxAttempts        = 3
	retryBackoff       = 50 * time.Millisecond
)

type Client struct {
	baseURL string
	secret  string
	http    *http.Client
	cb      *gobreaker.CircuitBreaker
}

func New(url, secret string) *Client {
	cb := gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        "wallet",
		Timeout:     5 * time.Second,
		MaxRequests: 1,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= 3
		},
		OnStateChange: func(name string, from, to gobreaker.State) {
			slog.Info("wallet breaker", "from", from.String(), "to", to.String())
		},
	})
	// No hard timeout on the http.Client itself — each call derives its deadline
	// from the caller's ctx (see move) so timeouts compose with request context.
	return &Client{baseURL: url, secret: secret, cb: cb, http: &http.Client{}}
}

// EnsureWallet is a no-op on the client: every mutating internal route ensures the
// wallet row server-side before moving money, so there is nothing to call remotely.
func (c *Client) EnsureWallet(ctx context.Context, uid int64) error { return nil }

func (c *Client) DEBIT(ctx context.Context, uid, amount int64, constraintId string) error {
	return c.move(ctx, walletcontract.PathDebit,
		walletcontract.MoveRequest{UID: uid, Amount: amount, ConstraintID: constraintId})
}

func (c *Client) CREDIT(ctx context.Context, uid, amount int64, constraintId string) error {
	return c.move(ctx, walletcontract.PathCredit,
		walletcontract.MoveRequest{UID: uid, Amount: amount, ConstraintID: constraintId})
}

// move performs an idempotent money operation with circuit breaking + bounded
// retries. Because every op carries a constraintId (idempotency key), retrying a
// transient failure is safe — at worst the server sees a duplicate and no-ops.
func (c *Client) move(ctx context.Context, path string, body walletcontract.MoveRequest) error {
	payload, _ := json.Marshal(body)

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		result, err := c.cb.Execute(func() (any, error) {
			callCtx, cancel := context.WithTimeout(ctx, defaultCallTimeout)
			defer cancel()

			req, _ := http.NewRequestWithContext(callCtx, http.MethodPost, c.baseURL+path, bytes.NewReader(payload))
			req.Header.Set("Content-Type", "application/json")
			if c.secret != "" {
				req.Header.Set("Authorization", "Bearer "+c.secret)
			}
			if rid := httpx.RequestID(ctx); rid != "" {
				req.Header.Set("X-Request-Id", rid) // propagate the trace across the boundary
			}

			resp, err := c.http.Do(req)
			if err != nil {
				return nil, err // network/timeout -> breaker failure, retryable
			}
			defer resp.Body.Close()

			switch {
			case resp.StatusCode == http.StatusOK:
				return "ok", nil
			case resp.StatusCode == http.StatusPaymentRequired:
				return "insufficient", nil // a valid business answer, not a failure
			case resp.StatusCode == http.StatusUnauthorized:
				return "unauthorized", nil // config error: don't retry, don't trip breaker
			case resp.StatusCode >= 500:
				return nil, fmt.Errorf("wallet 5xx: %d", resp.StatusCode) // server failure -> breaker failure
			default:
				return nil, fmt.Errorf("wallet unexpected status: %d", resp.StatusCode)
			}
		})

		if err == nil {
			switch result {
			case "insufficient":
				return walletcontract.ErrInsufficientFunds
			case "unauthorized":
				return errors.New("walletclient: unauthorized (check WALLET_INTERNAL_SECRET)")
			default:
				return nil
			}
		}

		// Breaker open -> fail fast as unavailable; no point retrying right now.
		if errors.Is(err, gobreaker.ErrOpenState) || errors.Is(err, gobreaker.ErrTooManyRequests) {
			return walletcontract.ErrWalletUnavailable
		}

		lastErr = err
		if attempt < maxAttempts {
			select {
			case <-ctx.Done():
				return walletcontract.ErrWalletUnavailable
			case <-time.After(retryBackoff * time.Duration(attempt)): // linear backoff
			}
		}
	}

	slog.Warn("wallet call failed after retries", "path", path, "err", lastErr)
	return fmt.Errorf("%w: %v", walletcontract.ErrWalletUnavailable, lastErr)
}

package walletclient

import (
	"Go-project/internal/wallet"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/sony/gobreaker"
)

type Client struct {
	baseURL string
	http    *http.Client
	cb      *gobreaker.CircuitBreaker
}

func New(url string) *Client {
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
	return &Client{baseURL: url, cb: cb, http: &http.Client{Timeout: 2 * time.Second}}
}

func (c *Client) EnsureWallet(ctx context.Context, uid int64) error { return nil }

func (c *Client) DEBIT(ctx context.Context, uid, amount int64, constraintId string) error {
	result, err := c.cb.Execute(func() (any, error) {
		body, _ := json.Marshal(map[string]any{"uid": uid, "amount": amount, "constraintId": constraintId})
		req, _ := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/internal/wallet/debit", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.http.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		switch {
		case resp.StatusCode == http.StatusOK:
			return "ok", nil
		case resp.StatusCode == http.StatusPaymentRequired:
			return "insufficient", nil
		case resp.StatusCode >= 500:
			return nil, fmt.Errorf("wallet 5xx: %d", resp.StatusCode) // server failure -> breaker failure
		default:
			return nil, fmt.Errorf("Wallet unexpected status: %d", resp.StatusCode)
		}
	})
	if err != nil {
		return err // includes gobreaker.ErrOpenState when the breaker is OPEN
	}
	if result == "insufficient" {
		return wallet.ErrInsufficientFunds // translate back to the sentinel the routes already handle
	}
	return nil
}

func (c *Client) CREDIT(ctx context.Context, uid, amount int64, constraintId string) error {
	_, err := c.cb.Execute(func() (any, error) {
		body, _ := json.Marshal(map[string]any{"uid": uid, "amount": amount, "constraintId": constraintId})
		req, _ := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/internal/wallet/credit", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.http.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		switch {
		case resp.StatusCode == http.StatusOK:
			return "ok", nil
		case resp.StatusCode >= 500:
			return nil, fmt.Errorf("wallet 5xx: %d", resp.StatusCode) // server failure -> breaker failure
		default:
			return nil, fmt.Errorf("Wallet unexpected status: %d", resp.StatusCode)
		}
	})
	if err != nil {
		return err // includes gobreaker.ErrOpenState when the breaker is OPEN
	}
	return nil
}

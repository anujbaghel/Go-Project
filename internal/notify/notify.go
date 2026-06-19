package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/sony/gobreaker"
)

type Client struct {
	url  string
	http *http.Client
	cb   *gobreaker.CircuitBreaker
}

func New(url string) *Client {
	cb := gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        "notify",
		MaxRequests: 1,
		Timeout:     5 * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= 3 // OPEN after 3 straight failures
		},
		OnStateChange: func(name string, from, to gobreaker.State) {
			slog.Info("Circuit breaker state change", "name", name, "from", from.String(), "to", to.String())
		},
	})
	return &Client{url: url, http: &http.Client{Timeout: 2 * time.Second}, cb: cb}
}

func (c *Client) Send(ctx context.Context, msg string) error {
	_, err := c.cb.Execute(func() (any, error) {
		body, _ := json.Marshal(map[string]string{"message": msg})
		req, _ := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
		resp, err := c.http.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 500 {
			return nil, errors.New("downstream 5xx")
		}
		return nil, nil
	})
	return err // when OPEN, this is gobreaker.ErrOpenState — returned INSTANTLY
}

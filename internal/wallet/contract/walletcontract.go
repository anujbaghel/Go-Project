// Package walletcontract is the neutral wire contract between the wallet service
// and its callers. It holds the sentinel errors, HTTP paths, and request/response
// DTOs for the internal wallet API — and nothing else (no DB, no business logic),
// so both the provider (internal/wallet) and the client (internal/walletclient)
// can depend on it without coupling to each other's implementation.
package walletcontract

import "errors"

// Sentinel errors that travel across the HTTP boundary as status codes and are
// re-materialised by the client so callers can errors.Is() against them.
var (
	// ErrInsufficientFunds maps to HTTP 402 Payment Required.
	ErrInsufficientFunds = errors.New("insufficient coins")
	// ErrWalletUnavailable means the wallet service could not be reached or the
	// circuit breaker is open. Maps to HTTP 503 Service Unavailable. Retryable.
	ErrWalletUnavailable = errors.New("wallet service unavailable")
)

// Internal (service-to-service) route paths.
const (
	PathDebit  = "/internal/wallet/debit"
	PathCredit = "/internal/wallet/credit"
)

// MoveRequest is the body for debit/credit internal calls.
type MoveRequest struct {
	UID          int64  `json:"uid"`
	Amount       int64  `json:"amount"`
	ConstraintID string `json:"constraintId"`
}

// StatusResponse is the success body returned by mutating internal routes.
type StatusResponse struct {
	Status string `json:"status"`
}

type WalletTransactionEvent struct {
	UID          int64  `json:"uid"`
	Type         string `json:"type"`
	Amount       int64  `json:"amount"`
	ConstraintID string `json:"constraintId"`
}

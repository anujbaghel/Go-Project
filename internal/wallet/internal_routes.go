package wallet

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"Go-project/internal/platform/httpx"
	"Go-project/internal/walletcontract"

	"github.com/go-chi/chi/v5"
)

// InternalRoutes mounts the service-to-service wallet API. These endpoints move
// money by uid with no end-user JWT, so they are gated by a shared secret
// (internalAuth) — only the platform's own services know it.
func InternalRoutes(r chi.Router, w *Wallet, secret []byte) {
	r.Group(func(ir chi.Router) {
		ir.Use(internalAuth(secret))

		ir.Post(walletcontract.PathDebit, func(rw http.ResponseWriter, req *http.Request) {
			log := httpx.Logger(req.Context())
			var b walletcontract.MoveRequest
			if err := json.NewDecoder(req.Body).Decode(&b); err != nil || b.ConstraintID == "" || b.Amount <= 0 {
				http.Error(rw, "Invalid Request Params", http.StatusBadRequest)
				return
			}
			if err := w.EnsureWallet(req.Context(), b.UID); err != nil {
				log.Error("debit: ensure wallet", "uid", b.UID, "err", err)
				http.Error(rw, "Ensure user Failed"+err.Error(), http.StatusInternalServerError)
				return
			}
			err := w.DEBIT(req.Context(), b.UID, b.Amount, b.ConstraintID)
			switch {
			case errors.Is(err, ErrInsufficientFunds):
				log.Warn("debit: insufficient funds", "uid", b.UID, "amount", b.Amount, "constraintId", b.ConstraintID)
				http.Error(rw, "Insufficient Funds: "+err.Error(), http.StatusPaymentRequired)
			case err != nil:
				log.Error("debit failed", "uid", b.UID, "amount", b.Amount, "constraintId", b.ConstraintID, "err", err)
				http.Error(rw, err.Error(), http.StatusInternalServerError)
			default:
				log.Info("debit ok", "uid", b.UID, "amount", b.Amount, "constraintId", b.ConstraintID)
				writeJSON(rw, walletcontract.StatusResponse{Status: "ok"})
			}
		})

		ir.Post(walletcontract.PathCredit, func(rw http.ResponseWriter, req *http.Request) {
			log := httpx.Logger(req.Context())
			var b walletcontract.MoveRequest
			if err := json.NewDecoder(req.Body).Decode(&b); err != nil || b.ConstraintID == "" || b.Amount <= 0 {
				http.Error(rw, "Invalid Request Params", http.StatusBadRequest)
				return
			}
			// EnsureWallet first: a credit to a user who has never had a wallet row
			// would otherwise update zero rows and silently not reflect in balance.
			if err := w.EnsureWallet(req.Context(), b.UID); err != nil {
				log.Error("credit: ensure wallet", "uid", b.UID, "err", err)
				http.Error(rw, "Ensure user Failed"+err.Error(), http.StatusInternalServerError)
				return
			}
			err := w.CREDIT(req.Context(), b.UID, b.Amount, b.ConstraintID)
			switch {
			case err != nil:
				log.Error("credit failed", "uid", b.UID, "amount", b.Amount, "constraintId", b.ConstraintID, "err", err)
				http.Error(rw, err.Error(), http.StatusInternalServerError)
			default:
				log.Info("credit ok", "uid", b.UID, "amount", b.Amount, "constraintId", b.ConstraintID)
				writeJSON(rw, walletcontract.StatusResponse{Status: "ok"})
			}
		})
	})
}

// internalAuth checks a shared bearer secret in constant time. Empty secret on the
// server side disables auth (dev only) — guarded against in main with a real default.
func internalAuth(secret []byte) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			raw := strings.TrimPrefix(req.Header.Get("Authorization"), "Bearer ")
			if subtle.ConstantTimeCompare([]byte(raw), secret) != 1 {
				http.Error(rw, "unauthorized", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(rw, req)
		})
	}
}

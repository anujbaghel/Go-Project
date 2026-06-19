package wallet

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
)

func InternalRoutes(r chi.Router, w *Wallet) {
	r.Post("/internal/wallet/debit", func(rw http.ResponseWriter, req *http.Request) {
		var b struct {
			UID          int64  `json:"uid"`
			Amount       int64  `json:"amount"`
			ConstraintId string `json:"constraintId"`
		}
		if err := json.NewDecoder(req.Body).Decode(&b); err != nil || b.ConstraintId == "" || b.Amount <= 0 {
			http.Error(rw, "Invalid Request Params", http.StatusBadRequest)
			return
		}
		if err := w.EnsureWallet(req.Context(), b.UID); err != nil {
			http.Error(rw, "Ensure user Failed"+err.Error(), http.StatusInternalServerError)
			return
		}
		err := w.DEBIT(req.Context(), b.UID, b.Amount, b.ConstraintId)
		switch {
		case errors.Is(err, ErrInsufficientFunds):
			http.Error(rw, "Insufficient Funds: "+err.Error(), http.StatusPaymentRequired)
			return
		case err != nil:
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			return
		default:
			writeJSON(rw, map[string]string{"status": "ok"})
		}
	})

	r.Post("/internal/wallet/credit", func(rw http.ResponseWriter, req *http.Request) {
		var b struct {
			UID          int64  `json:"uid"`
			Amount       int64  `json:"amount"`
			ConstraintId string `json:"constraintId"`
		}
		if err := json.NewDecoder(req.Body).Decode(&b); err != nil || b.ConstraintId == "" || b.Amount <= 0 {
			http.Error(rw, "Invalid Request Params", http.StatusBadRequest)
			return
		}

		err := w.CREDIT(req.Context(), b.UID, b.Amount, b.ConstraintId)
		switch {
		case err != nil:
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			return
		default:
			writeJSON(rw, map[string]string{"status": "ok"})
		}
	})
}

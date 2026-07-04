package transport

import (
	"Go-project/internal/core/gateway"
	walletcontract "Go-project/internal/wallet/contract"
	walletservice "Go-project/internal/wallet/service"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
)

func Routes(router chi.Router, w *walletservice.Wallet, secret []byte) {
	router.Group(func(pr chi.Router) {
		pr.Use(gateway.Auth(secret))
		pr.Post("/wallet/credit", func(rw http.ResponseWriter, req *http.Request) {
			uid, _ := gateway.UserID(req.Context())
			var body struct {
				Amount       int64  `json:"amount"`
				ConstraintID string `json:"constraintId"`
			}

			if err := json.NewDecoder(req.Body).Decode(&body); err != nil || body.ConstraintID == "" {
				http.Error(rw, "amount and Constraing Id Required", http.StatusBadRequest)
				return
			}
			if err := w.EnsureWallet(req.Context(), uid); err != nil {
				http.Error(rw, "Ensure Wallet Failed! "+err.Error(), http.StatusInternalServerError)
				return
			}
			if err := w.CREDIT(req.Context(), uid, body.Amount, body.ConstraintID); err != nil {
				http.Error(rw, "Credit Failed "+err.Error(), http.StatusInternalServerError)
				return
			}

			bal, _ := w.Balance(req.Context(), uid)
			writeJSON(rw, map[string]int64{"balance": bal})
		})
	})

	router.Group(func(pr chi.Router) {
		pr.Use(gateway.Auth(secret))
		pr.Post("/wallet/debit", func(rw http.ResponseWriter, req *http.Request) {
			uid, _ := gateway.UserID(req.Context())
			var body struct {
				Amount       int64  `json:"amount"`
				ConstraintId string `json:"constraintId"`
			}

			if err := json.NewDecoder(req.Body).Decode(&body); err != nil || body.ConstraintId == "" {
				http.Error(rw, "Invalid Constraint ID", http.StatusBadRequest)
				return
			}

			if err := w.DEBIT(req.Context(), uid, body.Amount, body.ConstraintId); err != nil {
				if errors.Is(err, walletcontract.ErrInsufficientFunds) {
					http.Error(rw, err.Error(), http.StatusPaymentRequired)
					return
				}
				http.Error(rw, "Debit Failed: "+err.Error(), http.StatusInternalServerError)
				return
			}

			bal, _ := w.Balance(req.Context(), uid)
			writeJSON(rw, map[string]int64{"balance": bal})
		})
	})

	router.Group(func(pr chi.Router) {
		pr.Use(gateway.Auth(secret))
		pr.Post("/wallet/revert", func(rw http.ResponseWriter, req *http.Request) {
			uid, _ := gateway.UserID(req.Context())
			var body struct {
				OriginalTransactionId int64  `json:"originalTid"`
				ConstraintID          string `json:"constraintId"`
				IgnoreIfReverted      bool   `json:"ignoreIfReverted"`
			}

			if err := json.NewDecoder(req.Body).Decode(&body); err != nil || body.ConstraintID == "" {
				http.Error(rw, "Invalid ConstraintID", http.StatusBadRequest)
				return
			}

			err := w.REVERT(req.Context(), body.OriginalTransactionId, body.ConstraintID, body.IgnoreIfReverted)
			if err != nil {
				http.Error(rw, "Revert Failed "+err.Error(), http.StatusInternalServerError)
				return
			}

			bal, _ := w.Balance(req.Context(), uid)
			writeJSON(rw, map[string]int64{"balance": bal})
		})
	})

	router.Group(func(pr chi.Router) {
		pr.Use(gateway.Auth(secret))
		pr.Post("/wallet/balance", func(rw http.ResponseWriter, req *http.Request) {
			uid, _ := gateway.UserID(req.Context())
			bal, _ := w.Balance(req.Context(), uid)
			writeJSON(rw, map[string]int64{"balance": bal})
		})
	})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

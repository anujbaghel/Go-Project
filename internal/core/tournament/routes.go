package tournament

import (
	"Go-project/internal/core/gateway"
	walletcontract "Go-project/internal/wallet/contract"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

func Routes(r chi.Router, svc *Service, secret []byte) {

	r.Post("/tournaments", func(w http.ResponseWriter, req *http.Request) {
		var body struct {
			Name            string `json:"name"`
			EntryFee        int64  `json:"entryFee"`
			InstantMinScore int    `json:"instantMinScore"`
		}

		_ = json.NewDecoder(req.Body).Decode(&body)
		ltid, err := svc.CREATE(req.Context(), body.Name, body.EntryFee, &body.InstantMinScore)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]int64{"ltid": ltid})
	})

	r.Post("/tournaments/{ltid}/start", lifecycleHandler(svc.Start))
	r.Post("/tournaments/{ltid}/result", lifecycleHandler(svc.DeclareResult))
	r.Post("/tournaments/{ltid}/cancel", lifecycleHandler(svc.Cancel))

	r.Group(func(pr chi.Router) {
		pr.Use(gateway.Auth(secret))
		pr.Post("/tournaments/{ltid}/register", func(w http.ResponseWriter, req *http.Request) {
			uid, _ := gateway.UserID(req.Context())
			ltid, _ := strconv.ParseInt(chi.URLParam(req, "ltid"), 10, 64)
			slog.Info("register attempt", "ltid", ltid, "uid", uid)

			err := svc.Register(req.Context(), ltid, uid)
			switch {
			case errors.Is(err, ErrNotFound):
				{
					slog.Error("register failed Not FOund", "ltid", ltid, "uid", uid, "err", err)
					http.Error(w, err.Error(), http.StatusNotFound)
				}
			case errors.Is(err, ErrRegistrationClosed):
				{
					slog.Error("register failed Registeration Closed", "ltid", ltid, "uid", uid, "err", err)
					http.Error(w, err.Error(), http.StatusConflict)
				}
			case errors.Is(err, walletcontract.ErrInsufficientFunds):
				{
					slog.Error("register failed Insufficient Funds", "ltid", ltid, "uid", uid, "err", err)
					http.Error(w, err.Error(), http.StatusPaymentRequired)
				}
			case errors.Is(err, walletcontract.ErrWalletUnavailable):
				{
					slog.Error("register failed wallet unavailable", "ltid", ltid, "uid", uid, "err", err)
					http.Error(w, err.Error(), http.StatusServiceUnavailable)
				}
			case err != nil:
				{
					slog.Error("register failed Internal Server Error Funds", "ltid", ltid, "uid", uid, "err", err)
					http.Error(w, err.Error(), http.StatusInternalServerError)
				}
			default:
				writeJSON(w, map[string]string{"status": "registered"})
			}
		})
	})
}

func lifecycleHandler(fn func(req_ctx_ctx context.Context, ltid int64) error) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		ltid, _ := strconv.ParseInt(chi.URLParam(req, "ltid"), 10, 64)
		if err := fn(req.Context(), ltid); err != nil {
			http.Error(rw, err.Error(), http.StatusBadRequest)
		}

		writeJSON(rw, map[string]string{"status": "ok"})
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

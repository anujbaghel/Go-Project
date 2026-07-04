package gateway

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

func Routes(router chi.Router, secret []byte) {
	router.Post("/otp/verify", func(w http.ResponseWriter, req *http.Request) {
		var body struct {
			UID int64 `json:"uid"` // pretend the otp step already verify this user
		}
		_ = json.NewDecoder(req.Body).Decode(&body)
		if body.UID == 0 {
			body.UID = 1 // default test user
		}
		tok, err := IssueToken(secret, body.UID)
		if err != nil {
			http.Error(w, "could not issue a token", http.StatusInternalServerError)
			return
		}
		writeJson(w, map[string]string{"token": tok})
	})

	// Protected
	router.Group(func(gr chi.Router) {
		gr.Use(Auth(secret))
		gr.Get("/user", func(w http.ResponseWriter, req *http.Request) {
			uid, ok := UserID(req.Context()) // trusts what Auth injected; no token
			if !ok {
				http.Error(w, "No User Found", http.StatusUnauthorized)
				return
			}
			writeJson(w, map[string]int64{"uid": uid})
		})
	})
}

func writeJson(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

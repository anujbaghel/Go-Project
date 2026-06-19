package httpx

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
)

type ctxKey string

const loggerKey ctxKey = "logger"

func RequestId(base *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := r.Header.Get("X-Request-Id")
			if id == "" {
				// id = newId()
			}
			l := base.With("requestId", id) // a logger that stamps every line with this id
			w.Header().Set("X-Request-Id", id)
			l.Info("request", "method", r.Method, "path", r.URL.Path)

			ctx := context.WithValue(r.Context(), loggerKey, l)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// Logger pulls the request-scoped logger; falls back to the default.
func Logger(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(loggerKey).(*slog.Logger); ok {
		return l
	}
	return slog.Default()
}

func newId() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b) // Random 8 bytes
	return hex.EncodeToString(b)
}

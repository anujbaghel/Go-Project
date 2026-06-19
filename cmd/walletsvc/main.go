package main

import (
	"Go-project/internal/platform/config"
	"Go-project/internal/platform/db"
	"Go-project/internal/wallet"
	"context"
	"log/slog"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	cfg := config.Load()
	pool := db.MustConnect(ctx, cfg.DatabaseURL)
	w := wallet.New(pool)

	r := chi.NewRouter()
	r.Get("/healthz", func(rw http.ResponseWriter, req *http.Request) {
		rw.WriteHeader(200)
	})
	wallet.InternalRoutes(r, w)

	srv := &http.Server{Addr: cfg.WALLET_HTTP_ADDR, Handler: r}
	go func() {
		_ = srv.ListenAndServe()
	}()
	slog.Info("Wallet service listening on :8090")

	<-ctx.Done()
	// Gracefull shutdown
	shCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shCtx)
	pool.Close()
}

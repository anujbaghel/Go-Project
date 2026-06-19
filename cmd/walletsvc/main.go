package main

import (
	"Go-project/internal/platform/config"
	"Go-project/internal/platform/db"
	"Go-project/internal/platform/httpx"
	"Go-project/internal/wallet"
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(log)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	cfg := config.Load()
	pool := db.MustConnect(ctx, cfg.DatabaseURL)
	w := wallet.New(pool)

	r := chi.NewRouter()
	r.Use(httpx.RequestId(log))

	// liveness — process is up
	r.Get("/healthz", func(rw http.ResponseWriter, req *http.Request) {
		rw.WriteHeader(http.StatusOK)
	})
	// readiness — dependencies reachable
	r.Get("/readyz", func(rw http.ResponseWriter, req *http.Request) {
		if err := pool.Ping(req.Context()); err != nil {
			log.Error("readyz: postgres", "err", err)
			rw.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		rw.WriteHeader(http.StatusOK)
	})

	wallet.InternalRoutes(r, w, []byte(cfg.WalletInternalSecret))

	srv := &http.Server{Addr: cfg.WALLET_HTTP_ADDR, Handler: r}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("wallet service failed", "err", err)
		}
	}()
	log.Info("wallet service listening", "addr", cfg.WALLET_HTTP_ADDR)

	<-ctx.Done()
	// Graceful shutdown
	shCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shCtx)
	pool.Close()
	log.Info("wallet service stopped")
}

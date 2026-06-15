package main

import (
	"Go-project/internal/gateway"
	"Go-project/internal/platform/config"
	"Go-project/internal/platform/db"
	"Go-project/internal/platform/redisx"
	"Go-project/internal/tournament"
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

	// 1) ctx that cancels when SIGINT/SIGTERM arrives — the shutdown trigger.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg := config.Load()

	if err := db.RunMigrations(cfg.DatabaseURL); err != nil {
		log.Error("migrations failed", "err", err.Error())
		os.Exit(1)
	}

	// build Dependencies  (this is DI container).
	pool := db.MustConnect(ctx, cfg.DatabaseURL)
	rdb := redisx.MustConnect(ctx, cfg.RedisAddr)
	log.Info("connected to Postgres and redis")

	r := chi.NewRouter()

	// User API Routes
	gateway.Routes(r, []byte(cfg.JWTSecret))

	// Wallet Routes
	w := wallet.New(pool)
	wallet.Routes(r, w, []byte(cfg.JWTSecret))

	// tournament Routes
	tscv := tournament.NEW(pool, w)
	tournament.Routes(r, tscv, []byte(cfg.JWTSecret))

	// liveness: "is the process alive?" — never touches dependencies.
	r.Get("/healthz", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	r.Get("/readyz", func(w http.ResponseWriter, req *http.Request) {
		if err := pool.Ping(req.Context()); err != nil {
			log.Error("readyz: postgres", "err", err)
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		if err := rdb.Ping(req.Context()).Err(); err != nil {
			log.Error("readyz: Redis", "err", err.Error())
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	srv := &http.Server{Addr: cfg.HTTPAddr, Handler: r}

	// Run the server in the background so main can wait for the shutdown signal.
	go func() {
		log.Info("listening", "addr", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("server failed", "err", err)
		}
	}()

	// Block until SIGTERM.
	<-ctx.Done()
	log.Info("Shutting Down")

	// ORDER MATTERS: stop accepting requests FIRST, finish in-flight, THEN close stores.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx) // stops new requests, waits for in flight to finish
	pool.Close()
	_ = rdb.Close()
	log.Info("bye")
}

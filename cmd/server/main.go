package main

import (
	"Go-project/internal/gameplay"
	"Go-project/internal/gateway"
	"Go-project/internal/interfaces/wallet"
	"Go-project/internal/jobs"
	"Go-project/internal/platform/config"
	"Go-project/internal/platform/db"
	"Go-project/internal/platform/dynconfig"
	"Go-project/internal/platform/httpx"
	"Go-project/internal/platform/redisx"
	"Go-project/internal/tournament"
	"Go-project/internal/walletclient"
	"Go-project/internal/worker"
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/hibiken/asynq"
)

var wc wallet.WalletBackend

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(log)

	// 1) ctx that cancels when SIGINT/SIGTERM arrives — the shutdown trigger.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg := config.Load()
	dynCnfg, err := dynconfig.Load("config/dynamic.json")
	if err != nil {
		log.Error("dynamic config load failed", "Err", err)
		os.Exit(1)
	}

	if err := db.RunMigrations(cfg.DatabaseURL); err != nil {
		log.Error("migrations failed", "err", err.Error())
		os.Exit(1)
	}

	// build Dependencies  (this is DI container).
	pool := db.MustConnect(ctx, cfg.DatabaseURL)
	rdb := redisx.MustConnect(ctx, cfg.RedisAddr)
	log.Info("connected to Postgres and redis")

	r := chi.NewRouter()
	r.Use(httpx.RequestId(log))
	// r.Use(middleware.Recoverer)

	// User API Routes
	gateway.Routes(r, []byte(cfg.JWTSecret))

	// Wallet Routes
	if cfg.WalletBaseURL != "" {
		wc = walletclient.New(cfg.WalletBaseURL + cfg.WALLET_HTTP_ADDR) // microservice : call wallet over
		log.Info("wallet: remote", "url", cfg.WalletBaseURL)
	} else {
		// wc = wallet.New(pool)
		log.Info("wallet: in-process")
	}
	// wallet.Routes(r, wc, []byte(cfg.JWTSecret))

	// tournament Routes
	tscv := tournament.NEW(pool, wc)
	tournament.Routes(r, tscv, []byte(cfg.JWTSecret))

	// Gameplay
	gpSvc := gameplay.New(pool, rdb)
	gameplay.Routes(r, gpSvc, []byte(cfg.JWTSecret))

	// Job Queue
	// PRODUCER
	redisOpt := asynq.RedisClientOpt{Addr: cfg.RedisAddr}
	asynqClient := asynq.NewClient(redisOpt)

	// CONSUMER
	jobHandler := jobs.NewHanlders(wc)
	asynqServ := asynq.NewServer(redisOpt, asynq.Config{Concurrency: 10}) // up to 10 jobs at once
	if err := asynqServ.Start(jobHandler.Mux()); err != nil {
		// Start = non-blocking
		log.Error("asynq start failed", "err", err)
		os.Exit(1)
	}

	jobs.Routes(r, asynqClient)

	// Worker
	wk := worker.NEW(pool, rdb, asynqClient, dynCnfg)
	worker.Routes(r, wk)

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
	_ = srv.Shutdown(shutdownCtx) // 1) stops new requests, waits for in flight to finish
	asynqServ.Shutdown()          // 2) stop the worker — BLOCKS until in-flight jobs finish
	_ = asynqClient.Close()       // 3) close the producer
	pool.Close()                  // 4) now safe to close stores
	_ = rdb.Close()
	log.Info("bye")
}

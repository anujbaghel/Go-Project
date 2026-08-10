package main

import (
	"Go-project/internal/platform/config"
	"Go-project/internal/platform/kafkax"
	walletcontract "Go-project/internal/wallet/contract"
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(log)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	cfg := config.Load()

	topic := "wallet-transactions"
	groupId := "notifier"
	c := kafkax.NewConsumer(strings.Split(cfg.KAFKA_BROKERS, ","), topic, groupId)

	defer c.Close()

	for {
		msg, err := c.ReadMessage(ctx) // Block until a message arrives or ctx is canceled
		if err != nil {
			if ctx.Err() != nil { // shutdown signal -> ReadMessage returned because ctx died
				break
			}
			log.Error("Error reading message", "error", err)
			continue
		}

		var e walletcontract.WalletTransactionEvent
		if err := json.Unmarshal(msg.Value, &e); err != nil {
			log.Error("Error unmarshaling message", "error", err)
			continue
		}
		log.Info("Received message", "key", string(msg.Key), "value", string(msg.Value))
		log.Info("notify", "uid", e.UID, "type", e.Type, "amount", e.Amount)
	}

	log.Info("notifier service stopped gracefully")
}

package config

import "os"

type Config struct {
	DatabaseURL          string
	RedisAddr            string
	HTTPAddr             string
	JWTSecret            string
	WalletBaseURL        string
	WALLET_HTTP_ADDR     string
	WalletInternalSecret string
	WALLET_GRPC_ADDR     string
	KAFKA_BROKERS        string
}

func Load() Config {
	return Config{
		DatabaseURL:          env("DATABASE_URL", "postgres://app:app@localhost:5432/minif2p?sslmode=disable"),
		RedisAddr:            env("REDIS_ADDR", "localhost:6379"),
		HTTPAddr:             env("HTTP_ADDR", ":8080"),
		JWTSecret:            env("JWT_SECRET", "dev-secret-change-me"),
		WalletBaseURL:        env("WALLET_BASE_URL", "http://localhost"),
		WALLET_HTTP_ADDR:     env("WALLET_HTTP_ADDR", ":8090"),
		WalletInternalSecret: env("WALLET_INTERNAL_SECRET", "dev-internal-secret-change-me"),
		WALLET_GRPC_ADDR:     env("WALLET_GRPC_ADDR", ":9090"),
		KAFKA_BROKERS:        env("KAFKA_BROKERS", "localhost:9092"),
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

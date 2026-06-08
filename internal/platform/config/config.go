package config

import "os"

type Config struct {
	DatabaseURL string
	RedisAddr   string
	HTTPAddr    string
	JWTSecret   string
}

func Load() Config {
	return Config{
		DatabaseURL: env("DATABASE_URL", "postgres://app:app@localhost:5432/minif2p?sslmode=disable"),
		RedisAddr:   env("REDIS_ADDR", "localhost:6379"),
		HTTPAddr:    env("HTTP_ADDR", ":8080"),
		JWTSecret:   env("JWT_SECRET", "dev-secret-change-me"),
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

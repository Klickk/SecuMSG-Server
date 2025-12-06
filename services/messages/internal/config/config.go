package config

import (
	"log/slog"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Addr             string
	DatabaseURL      string
	WSPollInterval   time.Duration
	DeliveryBatchMax int
	AuthBaseURL      string
}

func Load() Config {
	addr := envOr("MESSAGES_ADDR", ":8084")
	dbURL := envOr("MESSAGES_DATABASE_URL", "postgres://app:app@localhost:5432/messagesdb?sslmode=disable")
	poll := envDuration("MESSAGES_WS_POLL_MS", 500)
	batch := envInt("MESSAGES_DELIVERY_BATCH", 50)
	if batch <= 0 {
		slog.Warn("config: invalid delivery batch, defaulting", "batch", batch)
		batch = 50
	}
	return Config{
		Addr:             addr,
		DatabaseURL:      dbURL,
		WSPollInterval:   poll,
		DeliveryBatchMax: batch,
		AuthBaseURL:      envOr("AUTH_BASE_URL", "http://localhost:8081"),
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envDuration(key string, defaultMillis int) time.Duration {
	if v := os.Getenv(key); v != "" {
		n, err := strconv.Atoi(v)
		if err == nil && n > 0 {
			return time.Duration(n) * time.Millisecond
		}
		slog.Warn("config: invalid duration, using default", "key", key, "value", v, "default_ms", defaultMillis)
	}
	return time.Duration(defaultMillis) * time.Millisecond
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		n, err := strconv.Atoi(v)
		if err == nil {
			return n
		}
		slog.Warn("config: invalid int, using default", "key", key, "value", v, "default", fallback)
	}
	return fallback
}

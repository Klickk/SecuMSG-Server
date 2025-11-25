package config

import (
	"log/slog"
	"os"
	"strconv"
	"time"
)

type Config struct {
	// DB
	DatabaseURL string

	// Tokens / issuer
	Issuer       string
	Audience     string
	AccessTTL    time.Duration
	RefreshTTL   time.Duration
	SigningKey   string // HS256 secret or base64-decoded upstream
	SigningKeyID string // optional kid you may expose via JWKS

	// HTTP
	Addr       string
	TrustProxy bool
}

func Load() Config {
	return Config{
		DatabaseURL:  getenv("DATABASE_URL", "postgres://app:secret@localhost:5432/appdb?sslmode=disable"),
		Issuer:       getenv("ISSUER", "http://localhost:8081"),
		Audience:     getenv("AUDIENCE", "client"),
		AccessTTL:    getdur("ACCESS_TTL", 15*time.Minute),
		RefreshTTL:   getdur("REFRESH_TTL", 30*24*time.Hour),
		SigningKey:   must("SIGNING_KEY"),
		SigningKeyID: getenv("SIGNING_KEY_ID", "kid-1"),

		Addr:       getenv("ADDR", ":8081"),
		TrustProxy: getbool("TRUST_PROXY", true),
	}
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func getbool(k string, def bool) bool {
	if v := os.Getenv(k); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return def
}

func getdur(k string, def time.Duration) time.Duration {
	if v := os.Getenv(k); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
		slog.Warn("invalid duration, using default", "key", k, "value", v, "default", def)
	}
	return def
}

func must(k string) string {
	v := os.Getenv(k)
	if v == "" {
		slog.Error("missing required env", "key", k)
		os.Exit(1)
	}
	return v
}

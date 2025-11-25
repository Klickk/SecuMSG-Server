package main

import (
	"log/slog"
	"net/http"
	"os"
	"time"

	"auth/internal/config"
	"auth/internal/observability/logging"
	"auth/internal/observability/middleware"
	impl "auth/internal/service/impl"
	"auth/internal/store"
	httpx "auth/internal/transport/http"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	env := os.Getenv("ENVIRONMENT")
	if env == "" {
		env = "dev"
	}

	logger := logging.NewLogger(logging.Config{
		ServiceName: "auth",
		Environment: env,
		Level:       os.Getenv("LOG_LEVEL"),
	})

	slog.SetDefault(logger)

	logger.Info("starting service")

	cfg := config.Load()

	// 1) DB (read from env, not hardcoded)
	dsn := cfg.DatabaseURL
	gdb, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		logger.Error("gorm open", "error", err)
		os.Exit(1)
	}

	st := &store.Store{DB: gdb}

	// 2) Services
	pw := impl.NewPasswordServiceArgon2id()

	// HS256 token service — signing key comes from env
	ts := impl.NewTokenServiceHS256(impl.TokenConfig{
		Issuer:     cfg.Issuer,
		Audience:   cfg.Audience, // allow override via env; fallback provided in config.Load()
		AccessTTL:  cfg.AccessTTL,
		RefreshTTL: cfg.RefreshTTL,
		SigningKey: []byte(cfg.SigningKey),
	}, st)

	as := impl.NewAuthServiceImpl(st, pw, ts)
	ds := impl.NewDeviceServiceImpl(st)

	// 3) HTTP router
	mux := httpx.NewRouter(as, ds, ts) // if your router needs cfg (CORS, trust proxy), pass it in here

	handler := middleware.WithRequestAndTrace(mux)

	srv := &http.Server{
		Addr:              cfg.Addr, // e.g. ":8081"
		Handler:           handler,  // <-- was routes(cfg) (undefined). Use mux.
		ReadHeaderTimeout: 10 * time.Second,
	}

	slog.Info("auth service listening", "addr", srv.Addr, "issuer", cfg.Issuer)
	if err := srv.ListenAndServe(); err != nil {
		logger.Error("server error", "error", err)
		os.Exit(1)
	}
}

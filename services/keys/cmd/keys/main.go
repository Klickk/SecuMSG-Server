package main

import (
	"log/slog"
	"net/http"
	"os"
	"time"

	"keys/internal/config"
	"keys/internal/observability/logging"
	"keys/internal/service"
	"keys/internal/store"
	httptransport "keys/internal/transport/http"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	env := os.Getenv("ENVIRONMENT")
	if env == "" {
		env = "dev"
	}

	logger := logging.NewLogger(logging.Config{
		ServiceName: "keys",
		Environment: env,
		Level:       os.Getenv("LOG_LEVEL"),
	})

	slog.SetDefault(logger)

	logger.Info("starting service")

	cfg := config.Load()

	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{})
	if err != nil {
		logger.Error("gorm open", "error", err)
		os.Exit(1)
	}

	st := store.New(db)
	svc := service.New(st)
	mux := httptransport.NewRouter(svc)

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	slog.Info("keys service listening", "addr", cfg.Addr)
	if err := srv.ListenAndServe(); err != nil {
		logger.Error("server error", "error", err)
		os.Exit(1)
	}
}

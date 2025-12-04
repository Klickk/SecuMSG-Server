package main

import (
	"context"
	"log/slog"
	"messages/internal/config"
	"messages/internal/observability/logging"
	"messages/internal/observability/metrics"
	"messages/internal/observability/middleware"
	"messages/internal/service"
	"messages/internal/store"
	transport "messages/internal/transport/http"
	"net/http"
	"os"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	env := os.Getenv("ENVIRONMENT")
	if env == "" {
		env = "dev"
	}

	logger := logging.NewLogger(logging.Config{
		ServiceName: "messages",
		Environment: env,
		Level:       os.Getenv("LOG_LEVEL"),
	})

	slog.SetDefault(logger)
	metrics.MustRegister("messages")

	logger.Info("starting service")

	cfg := config.Load()

	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{})
	if err != nil {
		logger.Error("gorm open", "error", err)
		os.Exit(1)
	}

	st := store.New(db)
	if err := st.AutoMigrate(context.Background()); err != nil {
		logger.Error("auto migrate", "error", err)
		os.Exit(1)
	}

	svc := service.New(st)
	mux := transport.NewRouter(svc, cfg.WSPollInterval, cfg.DeliveryBatchMax)

	handler := middleware.WithRequestAndTrace(middleware.WithMetrics(mux))

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	slog.Info("messages service listening", "addr", cfg.Addr)
	if err := srv.ListenAndServe(); err != nil {
		logger.Error("server error", "error", err)
		os.Exit(1)
	}
}

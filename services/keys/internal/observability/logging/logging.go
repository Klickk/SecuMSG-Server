package logging

import (
	"log/slog"
	"os"
)

type Config struct {
	ServiceName string
	Environment string
	Level       string
}

func NewLogger(cfg Config) *slog.Logger {
	level := new(slog.LevelVar)

	switch cfg.Level {
	case "debug":
		level.Set(slog.LevelDebug)
	case "warn":
		level.Set(slog.LevelWarn)
	case "error":
		level.Set(slog.LevelError)
	default:
		level.Set(slog.LevelInfo)
	}

	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	})

	return slog.New(handler).With(
		slog.String("service", cfg.ServiceName),
		slog.String("env", cfg.Environment),
	)
}

package config

import "os"

type Config struct {
	DatabaseURL string
	Addr        string
	AuthBaseURL string
}

func Load() Config {
	return Config{
		DatabaseURL: getenv("DATABASE_URL", "postgres://app:secret@localhost:5432/appdb?sslmode=disable"),
		Addr:        getenv("ADDR", ":8082"),
		// Default to service DNS name for containerized deploys; override to
		// http://localhost:8081 when running everything on localhost without Docker.
		AuthBaseURL: getenv("AUTH_BASE_URL", "http://auth:8081"),
	}
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

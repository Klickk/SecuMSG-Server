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
		AuthBaseURL: getenv("AUTH_BASE_URL", "http://localhost:8081"),
	}
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

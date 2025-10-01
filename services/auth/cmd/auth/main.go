package main

import (
	"log"
	"net/http"
	"time"

	impl "auth/internal/service/impl"
	"auth/internal/store"
	httpx "auth/internal/transport/http"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	// 1) DB
	dsn := "postgres://postgres:postgres@localhost:5432/auth?sslmode=disable"
	gdb, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil { log.Fatalf("gorm open: %v", err) }

	st := &store.Store{DB: gdb}

	// 2) Services
	pw := impl.NewPasswordServiceArgon2id()
	ts := impl.NewTokenServiceHS256(impl.TokenConfig{
		Issuer:     "auth",
		Audience:   "client",
		AccessTTL:  15 * time.Minute,
		RefreshTTL: 30 * 24 * time.Hour,
		SigningKey: []byte("change-me-dev-secret"),
	}, st)
	as := impl.NewAuthServiceImpl(st, pw, ts)

	// 3) HTTP
	mux := httpx.NewRouter(as, ts)

	srv := &http.Server{
		Addr:              ":8080",
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Printf("auth up on %s", srv.Addr)
	log.Fatal(srv.ListenAndServe())
}

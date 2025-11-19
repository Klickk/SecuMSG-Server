package main

import (
	"log"
	"net/http"
	"time"

	"auth/internal/config"
	impl "auth/internal/service/impl"
	"auth/internal/store"
	httpx "auth/internal/transport/http"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	cfg := config.Load()

	// 1) DB (read from env, not hardcoded)
	dsn := cfg.DatabaseURL
	gdb, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("gorm open: %v", err)
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

	srv := &http.Server{
		Addr:              cfg.Addr, // e.g. ":8081"
		Handler:           mux,      // <-- was routes(cfg) (undefined). Use mux.
		ReadHeaderTimeout: 10 * time.Second,
	}

	log.Printf("auth up on %s (issuer=%s)", srv.Addr, cfg.Issuer)
	log.Fatal(srv.ListenAndServe())
}

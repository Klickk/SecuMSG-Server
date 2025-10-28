package main

import (
	"keys/internal/config"
	"keys/internal/service"
	"keys/internal/store"
	httptransport "keys/internal/transport/http"
	"log"
	"net/http"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	cfg := config.Load()

	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{})
	if err != nil {
		log.Fatalf("gorm open: %v", err)
	}

	st := store.New(db)
	svc := service.New(st)
	mux := httptransport.NewRouter(svc)

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	log.Printf("keys service listening on %s", cfg.Addr)
	log.Fatal(srv.ListenAndServe())
}

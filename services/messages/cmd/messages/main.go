package main

import (
	"context"
	"log"
	"messages/internal/config"
	"messages/internal/service"
	"messages/internal/store"
	transport "messages/internal/transport/http"
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
	if err := st.AutoMigrate(context.Background()); err != nil {
		log.Fatalf("auto migrate: %v", err)
	}

	svc := service.New(st)
	mux := transport.NewRouter(svc, cfg.WSPollInterval, cfg.DeliveryBatchMax)

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	log.Printf("messages service listening on %s", cfg.Addr)
	log.Fatal(srv.ListenAndServe())
}

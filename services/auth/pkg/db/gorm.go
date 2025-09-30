package db

import (
	"log"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

type Config struct {
	DSN             string // e.g. postgres://user:pass@localhost:5432/auth?sslmode=disable
	LogSQL          bool
	DisableFK       bool // set true if you manage FKs via SQL migrations
}

func OpenGorm(cfg Config) (*gorm.DB, error) {
	lvl := logger.Silent
	if cfg.LogSQL {
		lvl = logger.Info
	}
	return gorm.Open(postgres.Open(cfg.DSN), &gorm.Config{
		Logger: logger.New(log.New(log.Writer(), "", log.LstdFlags), logger.Config{
			SlowThreshold:             200 * time.Millisecond,
			LogLevel:                  lvl,
			IgnoreRecordNotFoundError: true,
			Colorful:                  false,
		}),
		NamingStrategy: schema.NamingStrategy{
			SingularTable: true,
		},
		DisableForeignKeyConstraintWhenMigrating: cfg.DisableFK,
	})
}

package store

import (
	"context"

	"gorm.io/gorm"
)

type Store struct {
	DB *gorm.DB
}

func New(db *gorm.DB) *Store { return &Store{DB: db} }

func (s *Store) WithTx(ctx context.Context, fn func(tx *Store) error) error {
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(&Store{DB: tx})
	})
}

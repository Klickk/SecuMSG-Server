package store

import (
	"context"

	"keys/internal/domain"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type UserStore struct{ db *gorm.DB }

func (s *Store) Users() *UserStore { return &UserStore{db: s.DB} }

func (u *UserStore) Ensure(ctx context.Context, id uuid.UUID) error {
	user := domain.User{ID: id}
	return u.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(&user).Error
}

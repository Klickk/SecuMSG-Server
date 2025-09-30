package store

import (
	"context"

	"auth/internal/domain"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type UserStore struct{ db *gorm.DB }

func (s *Store) Users() *UserStore { return &UserStore{db: s.DB} }

func (u *UserStore) Create(ctx context.Context, usr *domain.User) error {
	if usr.ID == uuid.Nil {
		usr.ID = uuid.New()
	}
	return u.db.WithContext(ctx).Create(usr).Error
}

func (u *UserStore) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	var user domain.User
	if err := u.db.WithContext(ctx).First(&user, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrRecordNotFound
		}
		
		return nil, err
	}
	return &user, nil
}

func (u *UserStore) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	var user domain.User
	if err := u.db.WithContext(ctx).First(&user, "email = ?", email).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrRecordNotFound
		}
		return nil, err
	}
	return &user, nil
}

func (u *UserStore) GetByUsername(ctx context.Context, username string) (*domain.User, error) {
	var user domain.User
	if err := u.db.WithContext(ctx).First(&user, "username = ?", username).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrRecordNotFound
		}
		return nil, err
	}
	return &user, nil
}

func (u *UserStore) SetEmailVerified(ctx context.Context, userID uuid.UUID) error {
	return u.db.WithContext(ctx).Model(&domain.User{}).
		Where("id = ?", userID).
		Update("email_verified", true).Error
}

func (u *UserStore) SetDisabled(ctx context.Context, userID uuid.UUID, disabled bool) error {
	return u.db.WithContext(ctx).Model(&domain.User{}).
		Where("id = ?", userID).
		Update("is_disabled", disabled).Error
}

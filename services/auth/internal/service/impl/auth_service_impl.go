package impl

import (
	"auth/internal/domain"
	"auth/internal/dto"
	"auth/internal/store"
	"context"
	"time"

	"github.com/google/uuid"
)

type AuthServiceImpl struct {
	deviceStore store.DeviceStore
	userStore store.UserStore
}

func NewAuthServiceImpl(deviceStore store.DeviceStore, userStore store.UserStore) *AuthServiceImpl {
	return &AuthServiceImpl{
		deviceStore: deviceStore,
		userStore: userStore,
	}
}

func (s *AuthServiceImpl) Register(ctx context.Context, r dto.RegisterRequest, ip, ua string) (*dto.RegisterResponse, error) {
	// Check if user already exists
	existingUser, err := s.userStore.GetByEmail(ctx, r.Email)
	if err != nil && err != store.ErrRecordNotFound {
		return nil, err
	}
	if existingUser != nil {
		return nil, domain.ErrUserAlreadyExists
	}
	existingUser, err = s.userStore.GetByUsername(ctx, r.Username)
	if err != nil && err != store.ErrRecordNotFound {
		return nil, err
	}
	if existingUser != nil {
		return nil, domain.ErrUserAlreadyExists
	}

	// Create new user
	newUser := &domain.User{
		ID:       uuid.New(),
		Email:    r.Email,
		Username: r.Username,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := s.userStore.Create(ctx, newUser); err != nil {
		return nil, err
	}

	return &dto.RegisterResponse{
		UserID: newUser.ID.String(),
		RequiresEmailVerification: true,
	}, nil
}

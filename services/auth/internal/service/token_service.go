package service

import (
	"context"

	"auth/internal/domain"
	"auth/internal/dto"
)

type TokenService interface {
	Issue(ctx context.Context, user *domain.User, deviceID *domain.DeviceID, ip, ua string) (*dto.TokenResponse, error)
	Refresh(ctx context.Context, refreshToken string, ip, ua string) (*dto.TokenResponse, error)
	RevokeSession(ctx context.Context, sessionID domain.SessionID) error
}

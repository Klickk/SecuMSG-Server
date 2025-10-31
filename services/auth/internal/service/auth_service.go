package service

import (
	"auth/internal/dto"
	"context"
)

type AuthService interface {
	Register(ctx context.Context, r dto.RegisterRequest, ip, ua string) (*dto.RegisterResponse, error)
	VerifyEmail(ctx context.Context, token string) error
	Login(ctx context.Context, r dto.LoginRequest, ip, ua string) (*dto.TokenResponse, error)
	Logout(ctx context.Context, refreshToken string) error
}

package service

import (
	"context"

	"auth/internal/domain"
)

type MFAService interface {
	ProvisionTOTP(ctx context.Context, userID domain.UserID) (otpURI string, pngQR []byte, err error)
	VerifyTOTP(ctx context.Context, userID domain.UserID, code string) (bool, error)
	GenerateRecoveryCodes(ctx context.Context, userID domain.UserID, n int) ([]string, error)
}

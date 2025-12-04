package service

import (
	"auth/internal/domain"
	"context"
)

type DeviceService interface {
	Register(
		ctx context.Context,
		userID domain.UserID,
		name, platform string,
	) (*domain.Device, error)
	Revoke(ctx context.Context, deviceID domain.DeviceID) error
}

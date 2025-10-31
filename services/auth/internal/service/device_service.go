package service

import (
	"auth/internal/domain"
	"auth/internal/dto"
	"context"
)

type DeviceService interface {
	Register(ctx context.Context, userID domain.UserID, name, platform string, key dto.DeviceKeyBundleRequest) (*domain.Device, error)
	RotatePreKeys(ctx context.Context, deviceID domain.DeviceID, req dto.RotatePreKeysRequest) error
	Revoke(ctx context.Context, deviceID domain.DeviceID) error
	AllocateOneTimePreKey(ctx context.Context, deviceID domain.DeviceID) ([]byte, error)
}

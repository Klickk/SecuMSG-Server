package impl

import (
	"context"
	"errors"
	"strings"
	"time"

	"auth/internal/domain"
	"auth/internal/service"
	"auth/internal/store"

	"github.com/google/uuid"
)

var _ service.DeviceService = (*DeviceServiceImpl)(nil)

type DeviceServiceImpl struct {
	store *store.Store
	now   func() time.Time
}

func NewDeviceServiceImpl(st *store.Store) *DeviceServiceImpl {
	return &DeviceServiceImpl{
		store: st,
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
}

func (d *DeviceServiceImpl) Register(
	ctx context.Context,
	userID domain.UserID,
	name, platform string,
) (*domain.Device, error) {
	if err := d.ensureStore(); err != nil {
		return nil, err
	}
	if userID == uuid.Nil {
		return nil, ErrInvalidDeviceUserID
	}
	name = strings.TrimSpace(name)
	platform = strings.TrimSpace(platform)
	if name == "" {
		return nil, ErrEmptyDeviceName
	}
	if platform == "" {
		return nil, ErrEmptyDevicePlatform
	}

	now := d.nowTime()

	device := &domain.Device{
		UserID:    userID,
		Name:      name,
		Platform:  platform,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := d.store.WithTx(ctx, func(tx *store.Store) error {
		if err := tx.Devices().Create(ctx, device); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return device, nil
}


func (d *DeviceServiceImpl) Revoke(ctx context.Context, deviceID domain.DeviceID) error {
	if err := d.ensureStore(); err != nil {
		return err
	}
	if deviceID == uuid.Nil {
		return ErrInvalidDeviceID
	}

	return d.store.WithTx(ctx, func(tx *store.Store) error {
		dev, err := tx.Devices().Get(ctx, uuid.UUID(deviceID))
		if err != nil {
			return translateDeviceErr(err)
		}
		if dev.RevokedAt != nil {
			return domain.ErrDeviceRevoked
		}
		if err := tx.Devices().Revoke(ctx, uuid.UUID(deviceID)); err != nil {
			return translateDeviceErr(err)
		}
		return translateDeviceErr(tx.Devices().Revoke(ctx, uuid.UUID(deviceID)))
	})
}


func (d *DeviceServiceImpl) ensureStore() error {
	if d.store == nil {
		return errors.New("device store not configured")
	}
	return nil
}

func (d *DeviceServiceImpl) nowTime() time.Time {
	if d.now != nil {
		return d.now()
	}
	return time.Now().UTC()
}

func translateDeviceErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, store.ErrRecordNotFound) {
		return domain.ErrDeviceNotFound
	}
	return err
}

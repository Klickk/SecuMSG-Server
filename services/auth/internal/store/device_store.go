package store

import (
	"auth/internal/domain"
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type DeviceStore struct{ db *gorm.DB }

func (s *Store) Devices() *DeviceStore { return &DeviceStore{db: s.DB} }

func (d *DeviceStore) Create(ctx context.Context, device *domain.Device) error {
	if device.ID == uuid.Nil {
		device.ID = uuid.New()
	}

	return d.db.WithContext(ctx).Create(device).Error
}

func (d *DeviceStore) Get(ctx context.Context, id uuid.UUID) (*domain.Device, error) {
	var device domain.Device
	if err := d.db.WithContext(ctx).First(&device, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrRecordNotFound
		}
		return nil, err
	}
	return &device, nil
}

func (d *DeviceStore) GetByUserID(ctx context.Context, userID uuid.UUID) ([]*domain.Device, error) {
	var devices []*domain.Device
	if err := d.db.WithContext(ctx).Where("user_id = ?", userID).Find(&devices).Error; err != nil {
		return nil, err
	}
	return devices, nil
}

func (d *DeviceStore) Revoke(ctx context.Context, deviceID uuid.UUID) error {
	res := d.db.WithContext(ctx).
		Model(&domain.Device{}).
		Where("id = ? AND revoked_at IS NULL", deviceID).
		Update("revoked_at", time.Now())
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrRecordNotFound
	}
	return nil
}

func (d *DeviceStore) RevokeAllForUser(ctx context.Context, userID uuid.UUID) (int64, error) {
	tx := d.db.WithContext(ctx).
		Model(&domain.Device{}).
		Where("user_id = ? AND revoked_at IS NULL", userID).
		Update("revoked_at", time.Now())
	return tx.RowsAffected, tx.Error
}
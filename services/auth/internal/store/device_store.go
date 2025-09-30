package store

import (
	"context"
	"time"
	"auth/internal/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type DeviceStore struct{ db *gorm.DB}

func (s *Store) Devices() *DeviceStore { return &DeviceStore{db: s.DB} }

func (d *DeviceStore) Create(ctx context.Context, device *domain.Device) error {
	if device.ID == uuid.Nil {
		device.ID = uuid.New()
	}

	return d.db.WithContext(ctx).Create(device).Error
}

func (d *DeviceStore) GetByUserID(ctx context.Context, userID uuid.UUID) ([]*domain.Device, error) {
	var devices []*domain.Device
	if err := d.db.WithContext(ctx).Where("user_id = ?", userID).Find(&devices).Error; err != nil {
		return nil, err
	}
	return devices, nil
}

func (d *DeviceStore) Revoke(ctx context.Context, deviceID uuid.UUID) error {
	return d.db.WithContext(ctx).
		Model(&domain.Device{}).
		Where("id = ?", deviceID).
		Update("revoked_at", time.Now()).Error
}

func (d *DeviceStore) RevokeAllForUser(ctx context.Context, userID uuid.UUID) (int64, error) {
	tx := d.db.WithContext(ctx).
		Model(&domain.Device{}).
		Where("user_id = ? AND revoked_at IS NULL", userID).
		Update("revoked_at", time.Now())
	return tx.RowsAffected, tx.Error
}

func (d *DeviceStore) UpsertKeyBundle(ctx context.Context, kb *domain.DeviceKeyBundle) error {
	return d.db.WithContext(ctx).
		Clauses(onConflictUpdateAllExcept("created_at")). // helper in credential_store.go
		Create(kb).Error
}

func (d *DeviceStore) GetKeyBundle(ctx context.Context, deviceID uuid.UUID) (*domain.DeviceKeyBundle, error) {
	var kb domain.DeviceKeyBundle
	if err := d.db.WithContext(ctx).First(&kb, "device_id = ?", deviceID).Error; err != nil {
		return nil, err
	}
	return &kb, nil
}

func (d *DeviceStore) DeleteKeyBundle(ctx context.Context, deviceID uuid.UUID) error {
	return d.db.WithContext(ctx).Delete(&domain.DeviceKeyBundle{}, "device_id = ?", deviceID).Error
}
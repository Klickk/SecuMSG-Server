package store

import (
	"context"

	"keys/internal/domain"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type DeviceStore struct{ db *gorm.DB }

func (s *Store) Devices() *DeviceStore { return &DeviceStore{db: s.DB} }

func (d *DeviceStore) Upsert(ctx context.Context, device domain.Device) error {
	return d.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "id"}},
			DoUpdates: clause.Assignments(map[string]any{"user_id": device.UserID}),
		}).
		Create(&device).Error
}

func (d *DeviceStore) Get(ctx context.Context, id uuid.UUID) (*domain.Device, error) {
	var device domain.Device
	if err := d.db.WithContext(ctx).First(&device, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrRecordNotFound
		}
		return nil, err
	}
	return &device, nil
}

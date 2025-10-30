package store

import (
	"context"
	"keys/internal/domain"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type OneTimePreKeyStore struct{ db *gorm.DB }

func (s *Store) OneTimePreKeys() *OneTimePreKeyStore { return &OneTimePreKeyStore{db: s.DB} }

func (o *OneTimePreKeyStore) AddBatch(ctx context.Context, keys []domain.OneTimePrekey) error {
	if len(keys) == 0 {
		return nil
	}
	return o.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(&keys).Error
}

func (o *OneTimePreKeyStore) ConsumeNext(ctx context.Context, deviceID uuid.UUID) (*domain.OneTimePrekey, error) {
	var key domain.OneTimePrekey
	tx := o.db.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
		Where("device_id = ? AND consumed_at IS NULL", deviceID).
		Order("created_at ASC, id ASC")
	if err := tx.First(&key).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	now := time.Now().UTC()
	if err := o.db.WithContext(ctx).Model(&domain.OneTimePrekey{}).
		Where("id = ?", key.ID).
		Update("consumed_at", now).Error; err != nil {
		return nil, err
	}
	key.ConsumedAt = &now
	return &key, nil
}

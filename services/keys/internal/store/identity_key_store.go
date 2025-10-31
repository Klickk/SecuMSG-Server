package store

import (
	"context"

	"keys/internal/domain"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type IdentityKeyStore struct{ db *gorm.DB }

func (s *Store) IdentityKeys() *IdentityKeyStore { return &IdentityKeyStore{db: s.DB} }

func (i *IdentityKeyStore) Upsert(ctx context.Context, key domain.IdentityKey) error {
	return i.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "device_id"}},
			DoUpdates: clause.Assignments(map[string]any{
				"public_key":    key.PublicKey,
				"signature_key": key.SignatureKey,
			}),
		}).
		Create(&key).Error
}

func (i *IdentityKeyStore) GetByDevice(ctx context.Context, deviceID uuid.UUID) (*domain.IdentityKey, error) {
	var key domain.IdentityKey
	if err := i.db.WithContext(ctx).First(&key, "device_id = ?", deviceID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrRecordNotFound
		}
		return nil, err
	}
	return &key, nil
}

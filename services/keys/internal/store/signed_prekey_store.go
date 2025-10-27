package store

import (
	"context"

	"keys/internal/domain"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type SignedPreKeyStore struct{ db *gorm.DB }

func (s *Store) SignedPreKeys() *SignedPreKeyStore { return &SignedPreKeyStore{db: s.DB} }

func (s *SignedPreKeyStore) Upsert(ctx context.Context, key domain.SignedPreKey) error {
	return s.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "device_id"}},
			DoUpdates: clause.Assignments(map[string]any{
				"public_key": key.PublicKey,
				"signature":  key.Signature,
				"created_at": key.CreatedAt,
			}),
		}).
		Create(&key).Error
}

func (s *SignedPreKeyStore) GetByDevice(ctx context.Context, deviceID uuid.UUID) (*domain.SignedPreKey, error) {
	var key domain.SignedPreKey
	if err := s.db.WithContext(ctx).First(&key, "device_id = ?", deviceID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrRecordNotFound
		}
		return nil, err
	}
	return &key, nil
}

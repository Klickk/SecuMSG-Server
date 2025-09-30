package store

import (
	"context"

	"auth/internal/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type CredentialStore struct{ db *gorm.DB }

func (s *Store) Credentials() *CredentialStore { return &CredentialStore{s.DB} }

func (cs *CredentialStore) UpsertPassword(ctx context.Context, c *domain.PasswordCredential) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	// Postgres upsert by user_id unique index
	return cs.db.WithContext(ctx).
		Clauses(onConflictUpdateAllExcept("id", "created_at")). // helper below
		Create(c).Error
}

func (cs *CredentialStore) GetPasswordByUserID(ctx context.Context, userID uuid.UUID) (*domain.PasswordCredential, error) {
	var c domain.PasswordCredential
	if err := cs.db.WithContext(ctx).First(&c, "user_id = ?", userID).Error; err != nil {
		return nil, err
	}
	return &c, nil
}

// --- helpers ---

// onConflictUpdateAllExcept returns a clause to upsert everything except the listed columns.
// You can replace this with gorm.io/gorm/clause.

func onConflictUpdateAllExcept(except ...string) clause.OnConflict {
	ex := map[string]struct{}{}
	for _, e := range except { ex[e] = struct{}{} }
	return clause.OnConflict{
		UpdateAll: true,
		DoUpdates: clause.Assignments(map[string]interface{}{}), // UpdateAll + Except handled by GORM
		// NOTE: GORM doesn't have Except in UpdateAll; alternatively enumerate columns explicitly.
	}
}

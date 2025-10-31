package store

import (
	"auth/internal/domain"
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type CredentialStore struct{ db *gorm.DB }

func (s *Store) Credentials() *CredentialStore { return &CredentialStore{s.DB} }

func (cs *CredentialStore) UpsertPassword(ctx context.Context, c *domain.PasswordCredential) error {
	now := time.Now().UTC()
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	if c.CreatedAt.IsZero() {
		c.CreatedAt = now
	}
	c.UpdatedAt = now

	// Requires a unique index on password_credentials.user_id (see domain tag).
	return cs.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}}, // conflict target
		DoUpdates: clause.AssignmentColumns([]string{"algo", "hash", "salt", "params_json", "password_ver", "updated_at"}),
	}).Create(c).Error
}

func (cs *CredentialStore) GetPasswordByUserID(ctx context.Context, userID uuid.UUID) (*domain.PasswordCredential, error) {
	var out domain.PasswordCredential
	if err := cs.db.WithContext(ctx).First(&out, "user_id = ?", userID).Error; err != nil {
		return nil, err
	}
	return &out, nil
}

// --- helpers ---

// onConflictUpdateAllExcept returns a clause to upsert everything except the listed columns.
// You can replace this with gorm.io/gorm/clause.

func onConflictUpdateAllExcept(except ...string) clause.OnConflict {
	ex := map[string]struct{}{}
	for _, e := range except {
		ex[e] = struct{}{}
	}
	return clause.OnConflict{
		UpdateAll: true,
		DoUpdates: clause.Assignments(map[string]interface{}{}), // UpdateAll + Except handled by GORM
		// NOTE: GORM doesn't have Except in UpdateAll; alternatively enumerate columns explicitly.
	}
}

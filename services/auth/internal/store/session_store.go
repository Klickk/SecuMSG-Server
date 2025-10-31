package store

import (
	"auth/internal/domain"
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type SessionStore struct{ db *gorm.DB }

func (s *Store) Sessions() *SessionStore { return &SessionStore{s.DB} }

func (ss *SessionStore) Create(ctx context.Context, s *domain.Session) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	if s.RefreshID == uuid.Nil {
		s.RefreshID = uuid.New()
	}
	return ss.db.WithContext(ctx).Create(s).Error
}

func (ss *SessionStore) GetByRefreshID(ctx context.Context, rid uuid.UUID) (*domain.Session, error) {
	var s domain.Session
	if err := ss.db.WithContext(ctx).First(&s, "refresh_id = ?", rid).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

func (ss *SessionStore) Revoke(ctx context.Context, id uuid.UUID, at time.Time) error {
	return ss.db.WithContext(ctx).
		Model(&domain.Session{}).
		Where("id = ?", id).
		Update("revoked_at", at).Error
}

func (ss *SessionStore) RevokeAllForUser(ctx context.Context, userID uuid.UUID, at time.Time) (int64, error) {
	tx := ss.db.WithContext(ctx).
		Model(&domain.Session{}).
		Where("user_id = ? AND revoked_at IS NULL", userID).
		Update("revoked_at", at)
	return tx.RowsAffected, tx.Error
}

func (ss *SessionStore) Rotate(ctx context.Context, sessionID uuid.UUID, newRefreshID uuid.UUID, newExpiresAt time.Time, ip, ua string) error {
	return ss.db.WithContext(ctx).
		Model(&domain.Session{}).
		Where("id = ? AND revoked_at IS NULL", sessionID).
		Updates(map[string]interface{}{
			"refresh_id": newRefreshID,
			"expires_at": newExpiresAt,
			"ip":         ip,
			"user_agent": ua,
		}).Error
}

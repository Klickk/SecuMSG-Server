package store

import (
	"auth/internal/domain"
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// DeleteUserData removes the user's record and related data (via cascades) and
// returns counts of affected resources captured before deletion.
func (s *Store) DeleteUserData(ctx context.Context, userID uuid.UUID) (map[string]int64, error) {
	deleted := map[string]int64{}

	err := s.WithTx(ctx, func(tx *Store) error {
		db := tx.DB.WithContext(ctx)

		count := func(label string, query *gorm.DB) error {
			var total int64
			if err := query.Count(&total).Error; err != nil {
				return err
			}
			deleted[label] = total
			return nil
		}

		if err := count("users", db.Model(&domain.User{}).Where("id = ?", userID)); err != nil {
			return err
		}
		if err := count("emailVerifications", db.Model(&domain.EmailVerification{}).Where("user_id = ?", userID)); err != nil {
			return err
		}
		if err := count("passwordCredentials", db.Model(&domain.PasswordCredential{}).Where("user_id = ?", userID)); err != nil {
			return err
		}
		if err := count("webauthnCredentials", db.Model(&domain.WebAuthnCredential{}).Where("user_id = ?", userID)); err != nil {
			return err
		}
		if err := count("devices", db.Model(&domain.Device{}).Where("user_id = ?", userID)); err != nil {
			return err
		}
		if err := count("deviceKeyBundles", db.Table("device_key_bundles").Joins("JOIN devices ON devices.id = device_key_bundles.device_id").Where("devices.user_id = ?", userID)); err != nil {
			return err
		}
		if err := count("sessions", db.Model(&domain.Session{}).Where("user_id = ?", userID)); err != nil {
			return err
		}
		if err := count("totpMfa", db.Model(&domain.TotpMFA{}).Where("user_id = ?", userID)); err != nil {
			return err
		}
		if err := count("recoveryCodes", db.Model(&domain.RecoveryCode{}).Where("user_id = ?", userID)); err != nil {
			return err
		}
		if err := count("auditLogs", db.Model(&domain.AuditLog{}).Where("user_id = ?", userID)); err != nil {
			return err
		}

		if err := db.Where("user_id = ?", userID).Delete(&domain.AuditLog{}).Error; err != nil {
			return err
		}

		return db.Where("id = ?", userID).Delete(&domain.User{}).Error
	})

	return deleted, err
}

package domain

import (
	"time"

	"github.com/google/uuid"
)

type Session struct {
	ID        SessionID  `gorm:"type:uuid;primaryKey" db:"id"`
	UserID    UserID     `gorm:"type:uuid;index" db:"user_id"`
	DeviceID  *DeviceID  `gorm:"type:uuid" db:"device_id"`
	RefreshID uuid.UUID  `gorm:"type:uuid;uniqueIndex:ux_sessions_refreshid" db:"refresh_id"`
	ExpiresAt time.Time  `gorm:"not null" db:"expires_at"`
	RevokedAt *time.Time `db:"revoked_at"`
	CreatedAt time.Time  `gorm:"not null" db:"created_at"`
	IP        string     `gorm:"type:inet" db:"ip"`
	UserAgent string     `gorm:"type:text" db:"user_agent"`
}

func (Session) TableName() string { return "sessions" }

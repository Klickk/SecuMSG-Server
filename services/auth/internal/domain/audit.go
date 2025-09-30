package domain

import "time"

type AuditLog struct {
	ID        [16]byte  `gorm:"type:uuid;primaryKey" db:"id"` // uuid bytes
	UserID    *UserID   `gorm:"type:uuid" db:"user_id"`
	Action    string    `gorm:"type:text;not null" db:"action"`
	Metadata  []byte    `gorm:"type:jsonb" db:"metadata"` // jsonb
	IP        string    `gorm:"type:text" db:"ip"`
	UserAgent string    `gorm:"type:text" db:"user_agent"`
	CreatedAt time.Time `gorm:"not null" db:"created_at"`
}

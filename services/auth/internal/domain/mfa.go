package domain

import "time"

type TotpMFA struct {
	UserID    UserID    `gorm:"not null;primaryKey" db:"user_id"`
	Secret    []byte    `gorm:"type:bytea;not null" db:"secret"`
	IsEnabled bool      `gorm:"not null;default:false" db:"is_enabled"`
	CreatedAt time.Time `gorm:"not null" db:"created_at"`
	UpdatedAt time.Time `gorm:"not null" db:"updated_at"`
}

func (TotpMFA) TableName() string { return "totp_mfa" }

type RecoveryCode struct {
	UserID    UserID     `gorm:"not null;primaryKey" db:"user_id"`
	CodeHash  []byte     `gorm:"type:bytea;not null" db:"code_hash"`
	UsedAt    *time.Time `gorm:"type:timestamp" db:"used_at"`
	CreatedAt time.Time  `gorm:"not null" db:"created_at"`
}

func (RecoveryCode) TableName() string { return "recovery_codes" }
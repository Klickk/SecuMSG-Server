package domain

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	CreatedAt time.Time `gorm:"not null;autoCreateTime"`
}

type Device struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID    uuid.UUID `gorm:"type:uuid;not null;index"`
	CreatedAt time.Time `gorm:"not null;autoCreateTime"`
}

type IdentityKey struct {
	DeviceID  uuid.UUID `gorm:"type:uuid;primaryKey"`
	PublicKey string    `gorm:"type:text;not null"`
	CreatedAt time.Time `gorm:"not null;autoCreateTime"`
	UpdatedAt time.Time `gorm:"not null;autoUpdateTime"`
}

type SignedPreKey struct {
	DeviceID  uuid.UUID `gorm:"type:uuid;primaryKey"`
	PublicKey string    `gorm:"type:text;not null"`
	Signature string    `gorm:"type:text;not null"`
	CreatedAt time.Time `gorm:"not null"`
}

type OneTimePreKey struct {
	ID         uuid.UUID  `gorm:"type:uuid;primaryKey"`
	DeviceID   uuid.UUID  `gorm:"type:uuid;not null;index"`
	PublicKey  string     `gorm:"type:text;not null"`
	ConsumedAt *time.Time `gorm:"type:timestamptz"`
	CreatedAt  time.Time  `gorm:"not null;autoCreateTime"`
}

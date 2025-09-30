package domain

import "time"


type Device struct {
	ID        DeviceID `gorm:"type:uuid;primaryKey" db:"id" json:"id"`
	UserID    UserID    `gorm:"type:uuid;index" db:"user_id" json:"userId"`
	Name      string     `gorm:"type:text;not null" db:"name" json:"name"`
	Platform  string     `gorm:"type:text;not null" db:"platform" json:"platform"`
	PushToken *string    `gorm:"type:text" db:"push_token" json:"pushToken"`
	CreatedAt time.Time  `gorm:"not null" db:"created_at"`
	UpdatedAt time.Time  `gorm:"not null" db:"updated_at"`
	RevokedAt *time.Time `db:"revoked_at"`
}

func (Device) TableName() string { return "devices" }

type DeviceKeyBundle struct {
	DeviceID        DeviceID `gorm:"type:uuid;primaryKey" db:"device_id"`
	IdentityKeyPub  []byte    `gorm:"type:bytea;not null" db:"identity_key_pub"`
	SignedPreKeyPub []byte    `gorm:"type:bytea;not null" db:"signed_prekey_pub"`
	SignedPreKeySig []byte    `gorm:"type:bytea;not null" db:"signed_prekey_sig"`
	// OneTimePreKeys: store as JSONB array; for heavy usage, put in a separate table.
	OneTimePreKeys []byte    `gorm:"type:jsonb;not null" db:"one_time_prekeys"`
	LastRotatedAt  time.Time `gorm:"not null" db:"last_rotated_at"`
	CreatedAt      time.Time `gorm:"not null" db:"created_at"`
}

func (DeviceKeyBundle) TableName() string { return "device_key_bundles" }

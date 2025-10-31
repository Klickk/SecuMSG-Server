package domain

import "time"

type User struct {
	ID            UserID    `gorm:"type:uuid;primaryKey" db:"id" json:"id"`
	Email         string    `gorm:"type:citext;uniqueIndex:ux_users_email" db:"email" json:"email"`
	EmailVerified bool      `gorm:"not null;default:false" db:"email_verified" json:"emailVerified"`
	Username      string    `gorm:"type:citext;uniqueIndex:ux_users_username" db:"username" json:"username"`
	IsDisabled    bool      `gorm:"not null;default:false" db:"is_disabled" json:"isDisabled"`
	CreatedAt     time.Time `gorm:"not null" db:"created_at" json:"createdAt"`
	UpdatedAt     time.Time `gorm:"not null" db:"updated_at" json:"updatedAt"`
}

func (User) TableName() string { return "users" }

type EmailVerification struct {
	UserID    UserID    `gorm:"type:uuid;index" db:"user_id"`
	Token     string    `gorm:"type:text;uniqueIndex" db:"token"`
	ExpiresAt time.Time `gorm:"not null" db:"expires_at"`
	Consumed  bool      `gorm:"not null;default:false" db:"consumed"`
	CreatedAt time.Time `gorm:"not null" db:"created_at"`
}

func (EmailVerification) TableName() string { return "email_verifications" }

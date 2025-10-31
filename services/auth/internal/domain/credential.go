package domain

import "time"

type PasswordCredential struct {
	ID          CredentialID `gorm:"type:uuid;primaryKey" db:"id"`
	UserID      UserID       `gorm:"type:uuid;uniqueIndex:ux_pwd_user" db:"user_id"`
	Algo        string       `gorm:"type:text;not null" db:"algo"`
	Hash        []byte       `gorm:"type:bytea;not null" db:"hash"`
	Salt        []byte       `gorm:"type:bytea;not null" db:"salt"`
	ParamsJSON  []byte       `gorm:"type:jsonb;not null" db:"params_json"`
	PasswordVer int          `gorm:"not null;default:1" db:"password_ver"`
	CreatedAt   time.Time    `gorm:"not null" db:"created_at"`
	UpdatedAt   time.Time    `gorm:"not null" db:"updated_at"`
}

func (PasswordCredential) TableName() string { return "password_credentials" }

func (p *PasswordCredential) GetAlgo() string       { return p.Algo }
func (p *PasswordCredential) GetHash() []byte       { return p.Hash }
func (p *PasswordCredential) GetSalt() []byte       { return p.Salt }
func (p *PasswordCredential) GetParamsJSON() []byte { return p.ParamsJSON }
func (p *PasswordCredential) GetPasswordVer() int   { return p.PasswordVer }

type WebAuthnCredential struct {
	ID           CredentialID `gorm:"type:uuid;primaryKey" db:"id"`
	UserID       UserID       `gorm:"type:uuid;index" db:"user_id"`
	CredentialID []byte       `gorm:"type:bytea;uniqueIndex:ux_webauthn_credid" db:"credential_id"`
	PublicKey    []byte       `gorm:"type:bytea;not null" db:"public_key"`
	SignCount    uint32       `gorm:"not null;default:0" db:"sign_count"`
	AAGUID       []byte       `gorm:"type:bytea" db:"aaguid"`
	CreatedAt    time.Time    `gorm:"not null" db:"created_at"`
	UpdatedAt    time.Time    `gorm:"not null" db:"updated_at"`
}

func (WebAuthnCredential) TableName() string { return "webauthn_credentials" }

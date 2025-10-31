package dto

import "time"

type SignedPreKey struct {
	PublicKey string    `json:"publicKey"`
	Signature string    `json:"signature"`
	CreatedAt time.Time `json:"createdAt"`
}

type OneTimePreKey struct {
	ID        string `json:"id"`
	PublicKey string `json:"publicKey"`
}

type RegisterDeviceRequest struct {
	UserID               string          `json:"userId"`
	DeviceID             string          `json:"deviceId"`
	IdentityKey          string          `json:"identityKey"`
	IdentitySignatureKey string          `json:"identitySignatureKey"`
	SignedPreKey         SignedPreKey    `json:"signedPreKey"`
	OneTimePreKeys       []OneTimePreKey `json:"oneTimePreKeys"`
}

type RegisterDeviceResponse struct {
	UserID         string `json:"userId"`
	DeviceID       string `json:"deviceId"`
	OneTimePreKeys int    `json:"oneTimePreKeys"`
}

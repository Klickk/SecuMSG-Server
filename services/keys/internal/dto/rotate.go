package dto

type RotateSignedPreKeyRequest struct {
	DeviceID       string          `json:"deviceId"`
	SignedPreKey   SignedPreKey    `json:"signedPreKey"`
	OneTimePreKeys []OneTimePreKey `json:"oneTimePreKeys"`
}

type RotateSignedPreKeyResponse struct {
	DeviceID         string       `json:"deviceId"`
	SignedPreKey     SignedPreKey `json:"signedPreKey"`
	AddedOneTimeKeys int          `json:"addedOneTimePreKeys"`
}

package dto

type DeviceRegisterRequest struct {
	UserID    string                 `json:"userId"`
	Name      string                 `json:"name"`
	Platform  string                 `json:"platform"`
	KeyBundle DeviceKeyBundleRequest `json:"keyBundle"`
}

type RotatePreKeysRequest struct {
	DeviceID        string   `json:"deviceId"`
	NewSignedPreKey string   `json:"newSignedPreKey"`
	NewSignedPKSig  string   `json:"newSignedPreKeySig"`
	OneTimePreKeys  []string `json:"oneTimePreKeys"`
}

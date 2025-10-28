package dto

type PreKeyBundleResponse struct {
	DeviceID      string         `json:"deviceId"`
	IdentityKey   string         `json:"identityKey"`
	SignedPreKey  SignedPreKey   `json:"signedPreKey"`
	OneTimePreKey *OneTimePreKey `json:"oneTimePreKey,omitempty"`
}

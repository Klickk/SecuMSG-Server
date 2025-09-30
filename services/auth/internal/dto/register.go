package dto

type RegisterRequest struct {
	Email      string                  `json:"email"`
	Username   string                  `json:"username"`
	Password   string                  `json:"password,omitempty"`
	DeviceName string                  `json:"deviceName,omitempty"`
	Platform   string                  `json:"platform,omitempty"`
	KeyBundle  *DeviceKeyBundleRequest `json:"keyBundle,omitempty"`
}

type DeviceKeyBundleRequest struct {
	IdentityKeyPub  string   `json:"identityKeyPub"`
	SignedPreKeyPub string   `json:"signedPreKeyPub"`
	SignedPreKeySig string   `json:"signedPreKeySig"`
	OneTimePreKeys  []string `json:"oneTimePreKeys"`
}

type RegisterResponse struct {
	UserID                  string `json:"userId"`
	RequiresEmailVerification bool `json:"requiresEmailVerification"`
}

package dto

type LoginRequest struct {
	EmailOrUsername string `json:"emailOrUsername"`
	Password        string `json:"password,omitempty"`
	WebAuthn        *struct {
		CredentialID string `json:"credentialId"`
		ClientData   string `json:"clientData"`
		AuthData     string `json:"authData"`
		Signature    string `json:"signature"`
	} `json:"webauthn,omitempty"`
	DeviceID *string `json:"deviceId,omitempty"`
}

type LoginMFARequest struct {
	Otp string `json:"otp"`
}

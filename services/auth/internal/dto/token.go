package dto

type TokenResponse struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	ExpiresIn    int64  `json:"expiresIn"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refreshToken"`
}

type VerifyRequest struct {
	Token    string `json:"token"`
	DeviceID string `json:"deviceId,omitempty"`
}

type VerifyResponse struct {
	Valid            bool   `json:"valid"`
	UserID           string `json:"userId,omitempty"`
	SessionID        string `json:"sessionId,omitempty"`
	TokenDeviceID    string `json:"tokenDeviceId,omitempty"`
	DeviceAuthorized bool   `json:"deviceAuthorized"`
}

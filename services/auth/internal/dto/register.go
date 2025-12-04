package dto

type RegisterRequest struct {
	Email      string `json:"email"`
	Username   string `json:"username"`
	Password   string `json:"password,omitempty"`
	DeviceName string `json:"deviceName,omitempty"`
	Platform   string `json:"platform,omitempty"`
}

type RegisterResponse struct {
	UserID                    string `json:"userId"`
	RequiresEmailVerification bool   `json:"requiresEmailVerification"`
}

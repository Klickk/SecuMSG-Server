package dto

type ResolveUserRequest struct {
	Username string `json:"username"`
}

type ResolveUserResponse struct {
	UserID   string `json:"userId"`
	Username string `json:"username"`
	DeviceID string `json:"deviceId"`
}

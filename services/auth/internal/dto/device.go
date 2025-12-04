package dto

type DeviceRegisterRequest struct {
	UserID    string                 `json:"userId"`
	Name      string                 `json:"name"`
	Platform  string                 `json:"platform"`
}

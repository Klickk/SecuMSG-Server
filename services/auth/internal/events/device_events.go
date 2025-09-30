package events

import "time"

type DeviceRegistered struct {
	DeviceID string    `json:"deviceId"`
	UserID   string    `json:"userId"`
	At       time.Time `json:"at"`
}

type DeviceRevoked struct {
	DeviceID string    `json:"deviceId"`
	UserID   string    `json:"userId"`
	At       time.Time `json:"at"`
}

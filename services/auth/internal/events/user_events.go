package events

import "time"

type UserRegistered struct {
	UserID string    `json:"userId"`
	Email  string    `json:"email"`
	At     time.Time `json:"at"`
}

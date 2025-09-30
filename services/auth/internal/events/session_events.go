package events

import "time"

type SessionRevoked struct {
	SessionID string    `json:"sessionId"`
	UserID    string    `json:"userId"`
	At        time.Time `json:"at"`
}

package domain

import "errors"

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrEmailNotVerified   = errors.New("email not verified")
	ErrUserDisabled       = errors.New("user disabled")
	ErrDeviceRevoked      = errors.New("device revoked")
	ErrRateLimited        = errors.New("rate limited")
	ErrUserAlreadyExists  = errors.New("user already exists")
	ErrInvalidToken       = errors.New("invalid token")
	ErrTokenExpired       = errors.New("token expired")
	ErrTokenConsumed      = errors.New("token already consumed")
	ErrMFARequired        = errors.New("multi-factor authentication required")
	ErrMFAMethodNotFound  = errors.New("multi-factor authentication method not found")
	ErrMFAMethodExists    = errors.New("multi-factor authentication method already exists")
	ErrSessionNotFound    = errors.New("session not found")
	ErrDeviceNotFound     = errors.New("device not found")
	ErrRecordNotFound     = errors.New("record not found")
)

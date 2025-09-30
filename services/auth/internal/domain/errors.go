package domain

import "errors"

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrEmailNotVerified   = errors.New("email not verified")
	ErrUserDisabled       = errors.New("user disabled")
	ErrDeviceRevoked      = errors.New("device revoked")
	ErrRateLimited        = errors.New("rate limited")
)

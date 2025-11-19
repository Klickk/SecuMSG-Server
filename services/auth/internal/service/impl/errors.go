package impl

import "errors"

var (
	ErrEmptyPassword       = errors.New("empty password")
	ErrEmptyCredential     = errors.New("empty credential(s)")
	ErrInvalidCred         = errors.New("invalid credential")
	ErrEmptyUsername       = errors.New("empty username")
	ErrEmptyEmail          = errors.New("empty email")
	ErrPasswordLength      = errors.New("password too short")
	ErrEmptyDeviceName     = errors.New("empty device name")
	ErrEmptyDevicePlatform = errors.New("empty device platform")
	ErrInvalidDeviceUserID = errors.New("invalid device user id")
	ErrInvalidDeviceID     = errors.New("invalid device id")
)

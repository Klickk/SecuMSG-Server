package service

import "errors"

var (
	ErrInvalidRequest = errors.New("invalid request")
	ErrDeviceNotFound = errors.New("device not found")
)

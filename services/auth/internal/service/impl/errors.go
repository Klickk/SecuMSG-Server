package impl

import "errors"

var (
	ErrEmptyPassword   = errors.New("empty password")
	ErrEmptyCredential = errors.New("empty credential(s)")
	ErrInvalidCred     = errors.New("invalid credential")
	ErrEmptyUsername   = errors.New("empty username")
	ErrEmptyEmail      = errors.New("empty email")
	ErrPasswordLength  = errors.New("password too short")
)

package cryptocore

import "errors"

var (
	ErrInvalidPrekeySignature = errors.New("cryptocore: invalid prekey signature")
	ErrMissingOneTimeKey      = errors.New("cryptocore: missing one-time prekey")
	ErrInvalidRemoteKey       = errors.New("cryptocore: invalid remote ratchet key")
	ErrDuplicateMessage       = errors.New("cryptocore: duplicate message")
	ErrDecryptionFailed       = errors.New("cryptocore: message authentication failed")
)

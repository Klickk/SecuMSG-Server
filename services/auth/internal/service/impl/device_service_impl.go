package impl

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"auth/internal/domain"
	"auth/internal/dto"
	"auth/internal/service"
	"auth/internal/store"

	"github.com/google/uuid"
)

var _ service.DeviceService = (*DeviceServiceImpl)(nil)

type DeviceServiceImpl struct {
	store *store.Store
	now   func() time.Time
}

func NewDeviceServiceImpl(st *store.Store) *DeviceServiceImpl {
	return &DeviceServiceImpl{
		store: st,
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
}

func (d *DeviceServiceImpl) Register(
	ctx context.Context,
	userID domain.UserID,
	name, platform string,
	key dto.DeviceKeyBundleRequest,
) (*domain.Device, error) {
	if err := d.ensureStore(); err != nil {
		return nil, err
	}
	if userID == uuid.Nil {
		return nil, ErrInvalidDeviceUserID
	}
	name = strings.TrimSpace(name)
	platform = strings.TrimSpace(platform)
	if name == "" {
		return nil, ErrEmptyDeviceName
	}
	if platform == "" {
		return nil, ErrEmptyDevicePlatform
	}

	now := d.nowTime()
	kb, err := d.keyBundleFromRequest(key, now)
	if err != nil {
		return nil, err
	}

	device := &domain.Device{
		UserID:    userID,
		Name:      name,
		Platform:  platform,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := d.store.WithTx(ctx, func(tx *store.Store) error {
		if err := tx.Devices().Create(ctx, device); err != nil {
			return err
		}
		kb.DeviceID = device.ID
		return tx.Devices().UpsertKeyBundle(ctx, kb)
	}); err != nil {
		return nil, err
	}

	return device, nil
}

func (d *DeviceServiceImpl) RotatePreKeys(ctx context.Context, deviceID domain.DeviceID, req dto.RotatePreKeysRequest) error {
	if err := d.ensureStore(); err != nil {
		return err
	}
	if deviceID == uuid.Nil {
		return ErrInvalidDeviceID
	}

	return d.store.WithTx(ctx, func(tx *store.Store) error {
		if err := d.ensureActiveDevice(ctx, tx, deviceID); err != nil {
			return err
		}
		kb, err := tx.Devices().GetKeyBundle(ctx, uuid.UUID(deviceID))
		if err != nil {
			return translateDeviceErr(err)
		}

		signedPreKey, err := decodeBase64Field(req.NewSignedPreKey, "newSignedPreKey")
		if err != nil {
			return err
		}
		signedPreKeySig, err := decodeBase64Field(req.NewSignedPKSig, "newSignedPreKeySig")
		if err != nil {
			return err
		}
		otkJSON, err := encodeOneTimePreKeys(req.OneTimePreKeys)
		if err != nil {
			return err
		}

		kb.SignedPreKeyPub = signedPreKey
		kb.SignedPreKeySig = signedPreKeySig
		kb.OneTimePreKeys = otkJSON
		kb.LastRotatedAt = d.nowTime()

		return tx.Devices().UpsertKeyBundle(ctx, kb)
	})
}

func (d *DeviceServiceImpl) Revoke(ctx context.Context, deviceID domain.DeviceID) error {
	if err := d.ensureStore(); err != nil {
		return err
	}
	if deviceID == uuid.Nil {
		return ErrInvalidDeviceID
	}

	return d.store.WithTx(ctx, func(tx *store.Store) error {
		dev, err := tx.Devices().Get(ctx, uuid.UUID(deviceID))
		if err != nil {
			return translateDeviceErr(err)
		}
		if dev.RevokedAt != nil {
			return domain.ErrDeviceRevoked
		}
		if err := tx.Devices().Revoke(ctx, uuid.UUID(deviceID)); err != nil {
			return translateDeviceErr(err)
		}
		return tx.Devices().DeleteKeyBundle(ctx, uuid.UUID(deviceID))
	})
}

func (d *DeviceServiceImpl) AllocateOneTimePreKey(ctx context.Context, deviceID domain.DeviceID) ([]byte, error) {
	if err := d.ensureStore(); err != nil {
		return nil, err
	}
	if deviceID == uuid.Nil {
		return nil, ErrInvalidDeviceID
	}

	var out []byte
	err := d.store.WithTx(ctx, func(tx *store.Store) error {
		if err := d.ensureActiveDevice(ctx, tx, deviceID); err != nil {
			return err
		}
		kb, err := tx.Devices().GetKeyBundle(ctx, uuid.UUID(deviceID))
		if err != nil {
			return translateDeviceErr(err)
		}
		keys, err := decodeOneTimePreKeys(kb.OneTimePreKeys)
		if err != nil {
			return err
		}
		if len(keys) == 0 {
			return domain.ErrNoOneTimePrekeys
		}
		next := keys[0]
		remaining := keys[1:]
		buf, err := json.Marshal(remaining)
		if err != nil {
			return fmt.Errorf("encode remaining one-time prekeys: %w", err)
		}
		kb.OneTimePreKeys = buf
		if err := tx.Devices().UpsertKeyBundle(ctx, kb); err != nil {
			return err
		}
		val, err := base64.StdEncoding.DecodeString(next)
		if err != nil {
			return fmt.Errorf("decode one-time prekey: %w", err)
		}
		out = val
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (d *DeviceServiceImpl) ensureStore() error {
	if d.store == nil {
		return errors.New("device store not configured")
	}
	return nil
}

func (d *DeviceServiceImpl) nowTime() time.Time {
	if d.now != nil {
		return d.now()
	}
	return time.Now().UTC()
}

func (d *DeviceServiceImpl) keyBundleFromRequest(req dto.DeviceKeyBundleRequest, now time.Time) (*domain.DeviceKeyBundle, error) {
	identityKey, err := decodeBase64Field(req.IdentityKeyPub, "identityKeyPub")
	if err != nil {
		return nil, err
	}
	signedPreKey, err := decodeBase64Field(req.SignedPreKeyPub, "signedPreKeyPub")
	if err != nil {
		return nil, err
	}
	signedPreKeySig, err := decodeBase64Field(req.SignedPreKeySig, "signedPreKeySig")
	if err != nil {
		return nil, err
	}
	otkJSON, err := encodeOneTimePreKeys(req.OneTimePreKeys)
	if err != nil {
		return nil, err
	}

	return &domain.DeviceKeyBundle{
		IdentityKeyPub:  identityKey,
		SignedPreKeyPub: signedPreKey,
		SignedPreKeySig: signedPreKeySig,
		OneTimePreKeys:  otkJSON,
		LastRotatedAt:   now,
		CreatedAt:       now,
	}, nil
}

func (d *DeviceServiceImpl) ensureActiveDevice(ctx context.Context, st *store.Store, deviceID domain.DeviceID) error {
	dev, err := st.Devices().Get(ctx, uuid.UUID(deviceID))
	if err != nil {
		return translateDeviceErr(err)
	}
	if dev.RevokedAt != nil {
		return domain.ErrDeviceRevoked
	}
	return nil
}

func decodeBase64Field(value, field string) ([]byte, error) {
	val := strings.TrimSpace(value)
	if val == "" {
		return nil, fmt.Errorf("%s is required", field)
	}
	decoded, err := base64.StdEncoding.DecodeString(val)
	if err != nil {
		return nil, fmt.Errorf("decode %s: %w", field, err)
	}
	return decoded, nil
}

func encodeOneTimePreKeys(keys []string) ([]byte, error) {
	if len(keys) == 0 {
		return nil, domain.ErrNoOneTimePrekeys
	}
	normalized := make([]string, 0, len(keys))
	for i, key := range keys {
		val := strings.TrimSpace(key)
		if val == "" {
			return nil, fmt.Errorf("one-time prekey[%d] is empty", i)
		}
		if _, err := base64.StdEncoding.DecodeString(val); err != nil {
			return nil, fmt.Errorf("decode one-time prekey[%d]: %w", i, err)
		}
		normalized = append(normalized, val)
	}
	buf, err := json.Marshal(normalized)
	if err != nil {
		return nil, fmt.Errorf("encode one-time prekeys: %w", err)
	}
	return buf, nil
}

func decodeOneTimePreKeys(data []byte) ([]string, error) {
	if len(data) == 0 {
		return nil, nil
	}
	var keys []string
	if err := json.Unmarshal(data, &keys); err != nil {
		return nil, fmt.Errorf("decode one-time prekeys: %w", err)
	}
	normalized := make([]string, 0, len(keys))
	for _, key := range keys {
		if trimmed := strings.TrimSpace(key); trimmed != "" {
			normalized = append(normalized, trimmed)
		}
	}
	return normalized, nil
}

func translateDeviceErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, store.ErrRecordNotFound) {
		return domain.ErrDeviceNotFound
	}
	return err
}

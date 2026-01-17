package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"keys/internal/domain"
	"keys/internal/dto"
	"keys/internal/store"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Service struct {
	store *store.Store
}

func New(store *store.Store) *Service {
	return &Service{store: store}
}

func (s *Service) RegisterDevice(ctx context.Context, req dto.RegisterDeviceRequest) (dto.RegisterDeviceResponse, error) {
	if req.IdentityKey == "" || req.IdentitySignatureKey == "" || req.SignedPreKey.PublicKey == "" || req.SignedPreKey.Signature == "" {
		return dto.RegisterDeviceResponse{}, fmt.Errorf("%w: missing key material", ErrInvalidRequest)
	}

	userID, err := parseOrGenerate(req.UserID)
	if err != nil {
		return dto.RegisterDeviceResponse{}, fmt.Errorf("%w: invalid userId", ErrInvalidRequest)
	}
	deviceID, err := parseOrGenerate(req.DeviceID)
	if err != nil {
		return dto.RegisterDeviceResponse{}, fmt.Errorf("%w: invalid deviceId", ErrInvalidRequest)
	}

	createdAt := req.SignedPreKey.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}

	otks := make([]domain.OneTimePrekey, 0, len(req.OneTimePreKeys))
	for _, k := range req.OneTimePreKeys {
		if k.PublicKey == "" {
			return dto.RegisterDeviceResponse{}, fmt.Errorf("%w: one-time prekey missing publicKey", ErrInvalidRequest)
		}
		id, err := parseOrGenerate(k.ID)
		if err != nil {
			return dto.RegisterDeviceResponse{}, fmt.Errorf("%w: invalid one-time prekey id", ErrInvalidRequest)
		}
		otks = append(otks, domain.OneTimePrekey{
			ID:        id,
			DeviceID:  deviceID,
			PublicKey: k.PublicKey,
		})
	}

	err = s.store.WithTx(ctx, func(tx *store.Store) error {
		if err := tx.Users().Ensure(ctx, userID); err != nil {
			return err
		}
		if err := tx.Devices().Upsert(ctx, domain.Device{ID: deviceID, UserID: userID}); err != nil {
			return err
		}
		if err := tx.IdentityKeys().Upsert(ctx, domain.IdentityKey{DeviceID: deviceID, PublicKey: req.IdentityKey, SignatureKey: req.IdentitySignatureKey}); err != nil {
			return err
		}
		if err := tx.SignedPreKeys().Upsert(ctx, domain.SignedPreKey{DeviceID: deviceID, PublicKey: req.SignedPreKey.PublicKey, Signature: req.SignedPreKey.Signature, CreatedAt: createdAt}); err != nil {
			return err
		}
		return tx.OneTimePreKeys().AddBatch(ctx, otks)
	})
	if err != nil {
		return dto.RegisterDeviceResponse{}, err
	}

	return dto.RegisterDeviceResponse{
		UserID:         userID.String(),
		DeviceID:       deviceID.String(),
		OneTimePreKeys: len(otks),
	}, nil
}

func (s *Service) GetPreKeyBundle(ctx context.Context, deviceID uuid.UUID) (dto.PreKeyBundleResponse, error) {
	var (
		identity *domain.IdentityKey
		signed   *domain.SignedPreKey
		otk      *domain.OneTimePrekey
	)

	err := s.store.WithTx(ctx, func(tx *store.Store) error {
		var err error
		identity, err = tx.IdentityKeys().GetByDevice(ctx, deviceID)
		if err != nil {
			if errors.Is(err, store.ErrRecordNotFound) {
				return ErrDeviceNotFound
			}
			return err
		}
		signed, err = tx.SignedPreKeys().GetByDevice(ctx, deviceID)
		if err != nil {
			if errors.Is(err, store.ErrRecordNotFound) {
				return ErrDeviceNotFound
			}
			return err
		}
		otk, err = tx.OneTimePreKeys().ConsumeNext(ctx, deviceID)
		return err
	})
	if err != nil {
		if errors.Is(err, ErrDeviceNotFound) {
			return dto.PreKeyBundleResponse{}, ErrDeviceNotFound
		}
		return dto.PreKeyBundleResponse{}, err
	}

	resp := dto.PreKeyBundleResponse{
		DeviceID:             deviceID.String(),
		IdentityKey:          identity.PublicKey,
		IdentitySignatureKey: identity.SignatureKey,
		SignedPreKey: dto.SignedPreKey{
			PublicKey: signed.PublicKey,
			Signature: signed.Signature,
			CreatedAt: signed.CreatedAt,
		},
	}
	if otk != nil {
		resp.OneTimePreKey = &dto.OneTimePreKey{
			ID:        otk.ID.String(),
			PublicKey: otk.PublicKey,
		}
	}
	return resp, nil
}

func (s *Service) RotateSignedPreKey(ctx context.Context, req dto.RotateSignedPreKeyRequest) (dto.RotateSignedPreKeyResponse, error) {
	deviceID, err := uuid.Parse(req.DeviceID)
	if err != nil {
		return dto.RotateSignedPreKeyResponse{}, fmt.Errorf("%w: invalid deviceId", ErrInvalidRequest)
	}
	if req.SignedPreKey.PublicKey == "" || req.SignedPreKey.Signature == "" {
		return dto.RotateSignedPreKeyResponse{}, fmt.Errorf("%w: missing signed prekey", ErrInvalidRequest)
	}

	createdAt := req.SignedPreKey.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}

	otks := make([]domain.OneTimePrekey, 0, len(req.OneTimePreKeys))
	for _, k := range req.OneTimePreKeys {
		if k.PublicKey == "" {
			return dto.RotateSignedPreKeyResponse{}, fmt.Errorf("%w: one-time prekey missing publicKey", ErrInvalidRequest)
		}
		id, err := parseOrGenerate(k.ID)
		if err != nil {
			return dto.RotateSignedPreKeyResponse{}, fmt.Errorf("%w: invalid one-time prekey id", ErrInvalidRequest)
		}
		otks = append(otks, domain.OneTimePrekey{ID: id, DeviceID: deviceID, PublicKey: k.PublicKey})
	}

	err = s.store.WithTx(ctx, func(tx *store.Store) error {
		if _, err := tx.Devices().Get(ctx, deviceID); err != nil {
			if errors.Is(err, store.ErrRecordNotFound) {
				return ErrDeviceNotFound
			}
			return err
		}
		if err := tx.SignedPreKeys().Upsert(ctx, domain.SignedPreKey{DeviceID: deviceID, PublicKey: req.SignedPreKey.PublicKey, Signature: req.SignedPreKey.Signature, CreatedAt: createdAt}); err != nil {
			return err
		}
		return tx.OneTimePreKeys().AddBatch(ctx, otks)
	})
	if err != nil {
		if errors.Is(err, ErrDeviceNotFound) {
			return dto.RotateSignedPreKeyResponse{}, ErrDeviceNotFound
		}
		return dto.RotateSignedPreKeyResponse{}, err
	}

	return dto.RotateSignedPreKeyResponse{
		DeviceID: req.DeviceID,
		SignedPreKey: dto.SignedPreKey{
			PublicKey: req.SignedPreKey.PublicKey,
			Signature: req.SignedPreKey.Signature,
			CreatedAt: createdAt,
		},
		AddedOneTimeKeys: len(otks),
	}, nil
}

func parseOrGenerate(id string) (uuid.UUID, error) {
	if id == "" {
		return uuid.New(), nil
	}
	parsed, err := uuid.Parse(id)
	if err != nil {
		return uuid.UUID{}, err
	}
	return parsed, nil
}

func (s *Service) DeleteUserData(ctx context.Context, userID uuid.UUID) (map[string]int64, error) {
	deleted := map[string]int64{}
	err := s.store.WithTx(ctx, func(tx *store.Store) error {
		db := tx.DB.WithContext(ctx)

		count := func(label string, query *gorm.DB) error {
			var total int64
			if err := query.Count(&total).Error; err != nil {
				return err
			}
			deleted[label] = total
			return nil
		}

		if err := count("users", db.Model(&domain.User{}).Where("id = ?", userID)); err != nil {
			return err
		}
		if err := count("devices", db.Model(&domain.Device{}).Where("user_id = ?", userID)); err != nil {
			return err
		}
		if err := count("identityKeys", db.Table("identity_keys").Joins("JOIN devices ON devices.id = identity_keys.device_id").Where("devices.user_id = ?", userID)); err != nil {
			return err
		}
		if err := count("signedPreKeys", db.Table("signed_pre_keys").Joins("JOIN devices ON devices.id = signed_pre_keys.device_id").Where("devices.user_id = ?", userID)); err != nil {
			return err
		}
		if err := count("oneTimePrekeys", db.Table("one_time_prekeys").Joins("JOIN devices ON devices.id = one_time_prekeys.device_id").Where("devices.user_id = ?", userID)); err != nil {
			return err
		}

		return db.Where("id = ?", userID).Delete(&domain.User{}).Error
	})

	return deleted, err
}

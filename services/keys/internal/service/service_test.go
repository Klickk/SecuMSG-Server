package service_test

import (
	"context"
	"testing"
	"time"

	"keys/internal/domain"
	"keys/internal/dto"
	"keys/internal/service"
	"keys/internal/store"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupService(t *testing.T) (*service.Service, *gorm.DB) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	if err := db.AutoMigrate(&domain.User{}, &domain.Device{}, &domain.IdentityKey{}, &domain.SignedPreKey{}, &domain.OneTimePreKey{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}

	svc := service.New(store.New(db))
	return svc, db
}

func TestRegisterAndFetchBundles(t *testing.T) {
	svc, db := setupService(t)

	userID := uuid.New().String()
	deviceID := uuid.New().String()

	req := dto.RegisterDeviceRequest{
		UserID:      userID,
		DeviceID:    deviceID,
		IdentityKey: "identity-1",
		SignedPreKey: dto.SignedPreKey{
			PublicKey: "signed-1",
			Signature: "sig-1",
			CreatedAt: time.Now().UTC(),
		},
		OneTimePreKeys: []dto.OneTimePreKey{
			{ID: uuid.New().String(), PublicKey: "otk-1"},
			{ID: uuid.New().String(), PublicKey: "otk-2"},
		},
	}

	resp, err := svc.RegisterDevice(context.Background(), req)
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if resp.UserID != userID || resp.DeviceID != deviceID {
		t.Fatalf("unexpected ids in response: %+v", resp)
	}
	if resp.OneTimePreKeys != 2 {
		t.Fatalf("expected 2 one-time prekeys recorded, got %d", resp.OneTimePreKeys)
	}

	id, _ := uuid.Parse(deviceID)

	bundle1, err := svc.GetPreKeyBundle(context.Background(), id)
	if err != nil {
		t.Fatalf("bundle1: %v", err)
	}
	if bundle1.IdentityKey != req.IdentityKey {
		t.Fatalf("expected identity key %s, got %s", req.IdentityKey, bundle1.IdentityKey)
	}
	if bundle1.SignedPreKey.PublicKey != req.SignedPreKey.PublicKey {
		t.Fatalf("expected signed prekey %s, got %s", req.SignedPreKey.PublicKey, bundle1.SignedPreKey.PublicKey)
	}
	if bundle1.OneTimePreKey == nil {
		t.Fatalf("expected a one-time prekey in first bundle")
	}

	firstPreKeyID := bundle1.OneTimePreKey.ID

	bundle2, err := svc.GetPreKeyBundle(context.Background(), id)
	if err != nil {
		t.Fatalf("bundle2: %v", err)
	}
	if bundle2.OneTimePreKey == nil {
		t.Fatalf("expected a one-time prekey in second bundle")
	}
	if bundle2.OneTimePreKey.ID == firstPreKeyID {
		t.Fatalf("expected different prekey on second bundle fetch")
	}

	bundle3, err := svc.GetPreKeyBundle(context.Background(), id)
	if err != nil {
		t.Fatalf("bundle3: %v", err)
	}
	if bundle3.OneTimePreKey != nil {
		t.Fatalf("expected no one-time prekey remaining")
	}

	var count int64
	if err := db.Model(&domain.OneTimePreKey{}).Where("device_id = ?", id).Count(&count).Error; err != nil {
		t.Fatalf("count prekeys: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 one-time prekeys stored, got %d", count)
	}
}

func TestRotateSignedPreKey(t *testing.T) {
	svc, db := setupService(t)

	deviceID := uuid.New().String()

	_, err := svc.RegisterDevice(context.Background(), dto.RegisterDeviceRequest{
		DeviceID:    deviceID,
		IdentityKey: "identity-rotate",
		SignedPreKey: dto.SignedPreKey{
			PublicKey: "signed-initial",
			Signature: "sig-initial",
			CreatedAt: time.Now().UTC(),
		},
		OneTimePreKeys: []dto.OneTimePreKey{{ID: uuid.New().String(), PublicKey: "otk-initial"}},
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	rotateResp, err := svc.RotateSignedPreKey(context.Background(), dto.RotateSignedPreKeyRequest{
		DeviceID: deviceID,
		SignedPreKey: dto.SignedPreKey{
			PublicKey: "signed-rotated",
			Signature: "sig-rotated",
			CreatedAt: time.Now().UTC(),
		},
		OneTimePreKeys: []dto.OneTimePreKey{{ID: uuid.New().String(), PublicKey: "otk-rotated"}},
	})
	if err != nil {
		t.Fatalf("rotate: %v", err)
	}
	if rotateResp.SignedPreKey.PublicKey != "signed-rotated" {
		t.Fatalf("rotate response missing updated key")
	}
	if rotateResp.AddedOneTimeKeys != 1 {
		t.Fatalf("expected 1 added one-time key, got %d", rotateResp.AddedOneTimeKeys)
	}

	id, _ := uuid.Parse(deviceID)
	bundle, err := svc.GetPreKeyBundle(context.Background(), id)
	if err != nil {
		t.Fatalf("bundle after rotate: %v", err)
	}
	if bundle.SignedPreKey.PublicKey != "signed-rotated" {
		t.Fatalf("expected rotated signed prekey in bundle, got %s", bundle.SignedPreKey.PublicKey)
	}

	var signedCount int64
	if err := db.Model(&domain.SignedPreKey{}).Where("device_id = ?", id).Count(&signedCount).Error; err != nil {
		t.Fatalf("count signed prekeys: %v", err)
	}
	if signedCount != 1 {
		t.Fatalf("expected a single active signed prekey, got %d", signedCount)
	}
}

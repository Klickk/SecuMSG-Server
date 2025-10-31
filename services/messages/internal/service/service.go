package service

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"messages/internal/msgjson"
	"messages/internal/store"
	"time"

	"github.com/google/uuid"
)

type Service struct {
	store *store.Store
	now   func() time.Time
}

type SendInput struct {
	ConvID       uuid.UUID
	FromDeviceID uuid.UUID
	ToDeviceID   uuid.UUID
	Ciphertext   []byte
	Header       json.RawMessage
}

var ErrInvalidRequest = errors.New("service: invalid request")

func New(st *store.Store) *Service {
	return &Service{store: st, now: time.Now}
}

func (s *Service) Enqueue(ctx context.Context, in SendInput) (store.Message, error) {
	if in.ConvID == uuid.Nil || in.FromDeviceID == uuid.Nil || in.ToDeviceID == uuid.Nil {
		return store.Message{}, ErrInvalidRequest
	}
	if len(in.Ciphertext) == 0 || len(in.Header) == 0 {
		return store.Message{}, ErrInvalidRequest
	}
	msg := store.Message{
		ConvID:       in.ConvID,
		FromDeviceID: in.FromDeviceID,
		ToDeviceID:   in.ToDeviceID,
		Ciphertext:   append([]byte(nil), in.Ciphertext...),
		Header:       msgjson.JSON(append([]byte(nil), in.Header...)),
		SentAt:       s.now().UTC(),
	}
	log.Printf("enqueue msg: %+v", msg)
	if err := s.store.Create(ctx, &msg); err != nil {
		return store.Message{}, err
	}
	return msg, nil
}

func (s *Service) Pending(ctx context.Context, deviceID uuid.UUID, limit int) ([]store.Message, error) {
	if deviceID == uuid.Nil {
		return nil, ErrInvalidRequest
	}
	return s.store.PendingForDevice(ctx, deviceID, limit)
}

func (s *Service) MarkDelivered(ctx context.Context, ids []uuid.UUID) error {
	if len(ids) == 0 {
		return nil
	}
	return s.store.MarkDelivered(ctx, ids, s.now().UTC())
}

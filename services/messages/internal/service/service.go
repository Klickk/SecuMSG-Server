package service

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"messages/internal/msgjson"
	"messages/internal/observability/metrics"
	"messages/internal/observability/middleware"
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
	if err := s.store.Create(ctx, &msg); err != nil {
		return store.Message{}, err
	}
	chatType := "unknown"
	reqID := middleware.RequestIDFromContext(ctx)
	traceID := middleware.TraceIDFromContext(ctx)
	slog.Info("stored ciphertext", "conv_id", msg.ConvID, "from_device_id", msg.FromDeviceID, "to_device_id", msg.ToDeviceID, "ciphertext_len", len(msg.Ciphertext), "request_id", reqID, "trace_id", traceID)
	metrics.MessagesStoredTotal.WithLabelValues(chatType).Inc()
	metrics.MessagesCiphertextBytes.WithLabelValues(chatType).Observe(float64(len(msg.Ciphertext)))
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

func (s *Service) History(ctx context.Context, deviceID uuid.UUID, since time.Time, convID uuid.UUID, limit int) ([]store.Message, error) {
	if deviceID == uuid.Nil {
		return nil, ErrInvalidRequest
	}
	scope := "all"
	if convID != uuid.Nil {
		scope = "conversation"
	}
	metrics.MessageHistoryFetchedTotal.WithLabelValues(scope).Inc()
	reqID := middleware.RequestIDFromContext(ctx)
	traceID := middleware.TraceIDFromContext(ctx)
	slog.Info("fetching message history", "device_id", deviceID, "conv_id", convID, "since", since, "limit", limit, "request_id", reqID, "trace_id", traceID)
	return s.store.History(ctx, store.HistoryFilter{
		DeviceID: deviceID,
		ConvID:   convID,
		Since:    since,
		Limit:    limit,
	})
}

func (s *Service) Conversations(ctx context.Context, deviceID uuid.UUID) ([]uuid.UUID, error) {
	if deviceID == uuid.Nil {
		return nil, ErrInvalidRequest
	}
	return s.store.ConversationsForDevice(ctx, deviceID)
}

func (s *Service) DeleteForDevice(ctx context.Context, deviceID uuid.UUID) (int64, error) {
	if deviceID == uuid.Nil {
		return 0, ErrInvalidRequest
	}
	return s.store.DeleteForDevice(ctx, deviceID)
}

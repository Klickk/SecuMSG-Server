package store

import (
	"context"
	"messages/internal/msgjson"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Message struct {
	ID           uuid.UUID      `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	ConvID       uuid.UUID      `gorm:"type:uuid;not null"`
	FromDeviceID uuid.UUID      `gorm:"type:uuid;not null"`
	ToDeviceID   uuid.UUID      `gorm:"type:uuid;not null;index:idx_messages_to_device_sent,priority:1"`
	Ciphertext   []byte         `gorm:"type:bytea;not null"`
	Header       msgjson.JSON   `gorm:"type:jsonb;not null"`
	SentAt       time.Time      `gorm:"not null;default:now();index:idx_messages_to_device_sent,priority:2"`
	ReceivedAt   *time.Time     `gorm:"type:timestamptz"`
	DeliveredAt  *time.Time     `gorm:"type:timestamptz"`
	DeletedAt    gorm.DeletedAt `gorm:"index"`
}

type Store struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Store {
	return &Store{db: db}
}

func (s *Store) AutoMigrate(ctx context.Context) error {
	return s.db.WithContext(ctx).AutoMigrate(&Message{})
}

func (s *Store) Create(ctx context.Context, msg *Message) error {
	return s.db.WithContext(ctx).Create(msg).Error
}

func (s *Store) PendingForDevice(ctx context.Context, deviceID uuid.UUID, limit int) ([]Message, error) {
	var msgs []Message
	tx := s.db.WithContext(ctx).
		Where("to_device_id = ? AND delivered_at IS NULL", deviceID).
		Order("sent_at asc")
	if limit > 0 {
		tx = tx.Limit(limit)
	}
	if err := tx.Find(&msgs).Error; err != nil {
		return nil, err
	}
	return msgs, nil
}

func (s *Store) MarkDelivered(ctx context.Context, ids []uuid.UUID, at time.Time) error {
	if len(ids) == 0 {
		return nil
	}
	return s.db.WithContext(ctx).
		Model(&Message{}).
		Where("id IN ?", ids).
		Update("delivered_at", at).
		Error
}

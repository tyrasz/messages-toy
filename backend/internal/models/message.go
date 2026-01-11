package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type MessageStatus string

const (
	MessageStatusSent      MessageStatus = "sent"
	MessageStatusDelivered MessageStatus = "delivered"
	MessageStatusRead      MessageStatus = "read"
)

type Message struct {
	ID          string        `gorm:"primaryKey" json:"id"`
	SenderID    string        `gorm:"not null;index" json:"sender_id"`
	RecipientID string        `gorm:"not null;index" json:"recipient_id"`
	Content     string        `json:"content,omitempty"`
	MediaID     *string       `json:"media_id,omitempty"`
	Status      MessageStatus `gorm:"default:sent" json:"status"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`

	Sender    User   `gorm:"foreignKey:SenderID" json:"-"`
	Recipient User   `gorm:"foreignKey:RecipientID" json:"-"`
	Media     *Media `gorm:"foreignKey:MediaID" json:"media,omitempty"`
}

func (m *Message) BeforeCreate(tx *gorm.DB) error {
	if m.ID == "" {
		m.ID = uuid.New().String()
	}
	if m.Status == "" {
		m.Status = MessageStatusSent
	}
	return nil
}

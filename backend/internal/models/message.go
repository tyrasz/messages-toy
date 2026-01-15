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
	ID            string        `gorm:"primaryKey" json:"id"`
	SenderID      string        `gorm:"not null;index" json:"sender_id"`
	RecipientID   *string       `gorm:"index" json:"recipient_id,omitempty"` // For DMs
	GroupID       *string       `gorm:"index" json:"group_id,omitempty"`     // For group messages
	ReplyToID     *string       `gorm:"index" json:"reply_to_id,omitempty"`  // For replies
	Content       string        `json:"content,omitempty"`
	MediaID       *string       `json:"media_id,omitempty"`
	ForwardedFrom *string       `json:"forwarded_from,omitempty"`            // Original sender name if forwarded
	Latitude      *float64      `json:"latitude,omitempty"`                  // For location sharing
	Longitude     *float64      `json:"longitude,omitempty"`                 // For location sharing
	LocationName  *string       `json:"location_name,omitempty"`             // Optional place name
	ScheduledAt   *time.Time    `gorm:"index" json:"scheduled_at,omitempty"` // For scheduled messages
	Status        MessageStatus `gorm:"default:sent" json:"status"`
	EditedAt      *time.Time    `json:"edited_at,omitempty"`                 // When message was edited
	DeletedAt     *time.Time    `json:"deleted_at,omitempty"`                // Soft delete for "delete for everyone"
	ExpiresAt     *time.Time    `gorm:"index" json:"expires_at,omitempty"`   // For disappearing messages
	CreatedAt     time.Time     `json:"created_at"`
	UpdatedAt     time.Time     `json:"updated_at"`

	Sender    User     `gorm:"foreignKey:SenderID" json:"-"`
	Recipient *User    `gorm:"foreignKey:RecipientID" json:"-"`
	Group     *Group   `gorm:"foreignKey:GroupID" json:"-"`
	Media     *Media   `gorm:"foreignKey:MediaID" json:"media,omitempty"`
	ReplyTo   *Message `gorm:"foreignKey:ReplyToID" json:"reply_to,omitempty"`
}

// MessageDeletion tracks "delete for me" operations
type MessageDeletion struct {
	ID        string    `gorm:"primaryKey" json:"id"`
	MessageID string    `gorm:"not null;index;uniqueIndex:idx_msg_user" json:"message_id"`
	UserID    string    `gorm:"not null;index;uniqueIndex:idx_msg_user" json:"user_id"`
	DeletedAt time.Time `json:"deleted_at"`

	Message Message `gorm:"foreignKey:MessageID" json:"-"`
	User    User    `gorm:"foreignKey:UserID" json:"-"`
}

func (md *MessageDeletion) BeforeCreate(tx *gorm.DB) error {
	if md.ID == "" {
		md.ID = uuid.New().String()
	}
	return nil
}

func (m *Message) IsGroupMessage() bool {
	return m.GroupID != nil && *m.GroupID != ""
}

func (m *Message) IsDeleted() bool {
	return m.DeletedAt != nil
}

func (m *Message) IsEdited() bool {
	return m.EditedAt != nil
}

func (m *Message) IsExpired() bool {
	if m.ExpiresAt == nil {
		return false
	}
	return m.ExpiresAt.Before(time.Now())
}

func (m *Message) IsDisappearing() bool {
	return m.ExpiresAt != nil
}

func (m *Message) IsLocation() bool {
	return m.Latitude != nil && m.Longitude != nil
}

func (m *Message) IsScheduled() bool {
	return m.ScheduledAt != nil && m.ScheduledAt.After(time.Now())
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

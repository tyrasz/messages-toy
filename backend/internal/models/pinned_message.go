package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// PinnedMessage represents a pinned message in a conversation or group
type PinnedMessage struct {
	ID          string    `gorm:"primaryKey" json:"id"`
	MessageID   string    `gorm:"not null;index" json:"message_id"`
	GroupID     *string   `gorm:"index;uniqueIndex:idx_pinned_conversation" json:"group_id,omitempty"`
	// For DMs, we store both user IDs sorted to create a unique conversation identifier
	ConversationKey *string   `gorm:"index;uniqueIndex:idx_pinned_conversation" json:"conversation_key,omitempty"`
	PinnedByID      string    `gorm:"not null" json:"pinned_by_id"`
	PinnedAt        time.Time `json:"pinned_at"`

	Message  Message `gorm:"foreignKey:MessageID" json:"message,omitempty"`
	PinnedBy User    `gorm:"foreignKey:PinnedByID" json:"-"`
}

func (p *PinnedMessage) BeforeCreate(tx *gorm.DB) error {
	if p.ID == "" {
		p.ID = uuid.New().String()
	}
	p.PinnedAt = time.Now()
	return nil
}

// MakeConversationKey creates a consistent key for a DM conversation
func MakeConversationKey(userID1, userID2 string) string {
	if userID1 < userID2 {
		return userID1 + ":" + userID2
	}
	return userID2 + ":" + userID1
}

// PinMessage pins a message in a conversation or group
func PinMessage(db *gorm.DB, messageID, pinnedByID string, groupID *string, userID1, userID2 *string) (*PinnedMessage, error) {
	// Verify message exists
	var message Message
	if err := db.First(&message, "id = ?", messageID).Error; err != nil {
		return nil, err
	}

	pinned := &PinnedMessage{
		MessageID:  messageID,
		PinnedByID: pinnedByID,
	}

	if groupID != nil {
		pinned.GroupID = groupID
		// Remove any existing pin in this group
		db.Where("group_id = ?", *groupID).Delete(&PinnedMessage{})
	} else if userID1 != nil && userID2 != nil {
		key := MakeConversationKey(*userID1, *userID2)
		pinned.ConversationKey = &key
		// Remove any existing pin in this conversation
		db.Where("conversation_key = ?", key).Delete(&PinnedMessage{})
	}

	if err := db.Create(pinned).Error; err != nil {
		return nil, err
	}

	// Load the message for response
	db.Preload("Message").First(pinned, "id = ?", pinned.ID)

	return pinned, nil
}

// UnpinMessage removes a pinned message
func UnpinMessage(db *gorm.DB, groupID *string, userID1, userID2 *string) error {
	if groupID != nil {
		return db.Where("group_id = ?", *groupID).Delete(&PinnedMessage{}).Error
	} else if userID1 != nil && userID2 != nil {
		key := MakeConversationKey(*userID1, *userID2)
		return db.Where("conversation_key = ?", key).Delete(&PinnedMessage{}).Error
	}
	return nil
}

// GetPinnedMessage gets the pinned message for a conversation or group
func GetPinnedMessage(db *gorm.DB, groupID *string, userID1, userID2 *string) (*PinnedMessage, error) {
	var pinned PinnedMessage

	query := db.Preload("Message")

	if groupID != nil {
		query = query.Where("group_id = ?", *groupID)
	} else if userID1 != nil && userID2 != nil {
		key := MakeConversationKey(*userID1, *userID2)
		query = query.Where("conversation_key = ?", key)
	} else {
		return nil, gorm.ErrRecordNotFound
	}

	if err := query.First(&pinned).Error; err != nil {
		return nil, err
	}

	return &pinned, nil
}

// PinnedMessageResponse is the API response for a pinned message
type PinnedMessageResponse struct {
	ID         string `json:"id"`
	MessageID  string `json:"message_id"`
	Content    string `json:"content,omitempty"`
	SenderID   string `json:"sender_id"`
	PinnedByID string `json:"pinned_by_id"`
	PinnedAt   string `json:"pinned_at"`
}

func (p *PinnedMessage) ToResponse() PinnedMessageResponse {
	return PinnedMessageResponse{
		ID:         p.ID,
		MessageID:  p.MessageID,
		Content:    p.Message.Content,
		SenderID:   p.Message.SenderID,
		PinnedByID: p.PinnedByID,
		PinnedAt:   p.PinnedAt.Format(time.RFC3339),
	}
}

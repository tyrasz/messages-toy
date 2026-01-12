package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type StarredMessage struct {
	ID        string    `gorm:"primaryKey" json:"id"`
	UserID    string    `gorm:"not null;index;uniqueIndex:idx_user_message" json:"user_id"`
	MessageID string    `gorm:"not null;index;uniqueIndex:idx_user_message" json:"message_id"`
	CreatedAt time.Time `json:"created_at"`

	User    User    `gorm:"foreignKey:UserID" json:"-"`
	Message Message `gorm:"foreignKey:MessageID" json:"message,omitempty"`
}

func (sm *StarredMessage) BeforeCreate(tx *gorm.DB) error {
	if sm.ID == "" {
		sm.ID = uuid.New().String()
	}
	return nil
}

// StarMessage adds a message to user's starred messages
func StarMessage(db *gorm.DB, userID, messageID string) (*StarredMessage, error) {
	// Check if already starred
	var existing StarredMessage
	err := db.Where("user_id = ? AND message_id = ?", userID, messageID).First(&existing).Error
	if err == nil {
		return &existing, nil // Already starred
	}

	starred := StarredMessage{
		UserID:    userID,
		MessageID: messageID,
	}
	if err := db.Create(&starred).Error; err != nil {
		return nil, err
	}
	return &starred, nil
}

// UnstarMessage removes a message from user's starred messages
func UnstarMessage(db *gorm.DB, userID, messageID string) error {
	return db.Where("user_id = ? AND message_id = ?", userID, messageID).Delete(&StarredMessage{}).Error
}

// IsMessageStarred checks if a message is starred by user
func IsMessageStarred(db *gorm.DB, userID, messageID string) bool {
	var count int64
	db.Model(&StarredMessage{}).Where("user_id = ? AND message_id = ?", userID, messageID).Count(&count)
	return count > 0
}

// GetStarredMessages returns user's starred messages with message details
func GetStarredMessages(db *gorm.DB, userID string, limit, offset int) ([]StarredMessage, error) {
	var starred []StarredMessage
	err := db.Preload("Message").Preload("Message.Media").
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&starred).Error
	return starred, err
}

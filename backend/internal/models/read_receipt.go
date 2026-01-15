package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// MessageReadReceipt tracks when users read messages (primarily for groups)
type MessageReadReceipt struct {
	ID        string    `gorm:"primaryKey" json:"id"`
	MessageID string    `gorm:"not null;index;uniqueIndex:idx_read_receipt" json:"message_id"`
	UserID    string    `gorm:"not null;index;uniqueIndex:idx_read_receipt" json:"user_id"`
	ReadAt    time.Time `json:"read_at"`

	Message Message `gorm:"foreignKey:MessageID" json:"-"`
	User    User    `gorm:"foreignKey:UserID" json:"-"`
}

func (r *MessageReadReceipt) BeforeCreate(tx *gorm.DB) error {
	if r.ID == "" {
		r.ID = uuid.New().String()
	}
	if r.ReadAt.IsZero() {
		r.ReadAt = time.Now()
	}
	return nil
}

// MarkMessagesAsRead marks multiple messages as read by a user
func MarkMessagesAsRead(db *gorm.DB, userID string, messageIDs []string) error {
	now := time.Now()
	for _, msgID := range messageIDs {
		receipt := MessageReadReceipt{
			MessageID: msgID,
			UserID:    userID,
			ReadAt:    now,
		}
		// Use ON CONFLICT to avoid duplicates
		result := db.Where("message_id = ? AND user_id = ?", msgID, userID).FirstOrCreate(&receipt)
		if result.Error != nil {
			return result.Error
		}
	}
	return nil
}

// GetReadReceipts returns all read receipts for a message
func GetReadReceipts(db *gorm.DB, messageID string) ([]ReadReceiptResponse, error) {
	var receipts []MessageReadReceipt
	err := db.Preload("User").Where("message_id = ?", messageID).Find(&receipts).Error
	if err != nil {
		return nil, err
	}

	var responses []ReadReceiptResponse
	for _, r := range receipts {
		responses = append(responses, ReadReceiptResponse{
			UserID:      r.UserID,
			Username:    r.User.Username,
			DisplayName: r.User.DisplayName,
			ReadAt:      r.ReadAt,
		})
	}
	return responses, nil
}

// GetReadReceiptsForMessages returns read receipts for multiple messages
func GetReadReceiptsForMessages(db *gorm.DB, messageIDs []string) (map[string][]ReadReceiptResponse, error) {
	var receipts []MessageReadReceipt
	err := db.Preload("User").Where("message_id IN ?", messageIDs).Find(&receipts).Error
	if err != nil {
		return nil, err
	}

	result := make(map[string][]ReadReceiptResponse)
	for _, r := range receipts {
		result[r.MessageID] = append(result[r.MessageID], ReadReceiptResponse{
			UserID:      r.UserID,
			Username:    r.User.Username,
			DisplayName: r.User.DisplayName,
			ReadAt:      r.ReadAt,
		})
	}
	return result, nil
}

// GetUnreadCount returns the number of unread messages in a conversation for a user
func GetUnreadCount(db *gorm.DB, userID string, groupID *string, otherUserID *string) (int64, error) {
	query := db.Model(&Message{}).
		Where("sender_id != ?", userID).
		Where("deleted_at IS NULL")

	if groupID != nil {
		query = query.Where("group_id = ?", *groupID)
	} else if otherUserID != nil {
		query = query.Where(
			"(sender_id = ? AND recipient_id = ?) OR (sender_id = ? AND recipient_id = ?)",
			*otherUserID, userID, userID, *otherUserID,
		)
	}

	// Exclude messages the user has already read
	subQuery := db.Model(&MessageReadReceipt{}).Select("message_id").Where("user_id = ?", userID)
	query = query.Where("id NOT IN (?)", subQuery)

	var count int64
	err := query.Count(&count).Error
	return count, err
}

type ReadReceiptResponse struct {
	UserID      string    `json:"user_id"`
	Username    string    `json:"username"`
	DisplayName string    `json:"display_name,omitempty"`
	ReadAt      time.Time `json:"read_at"`
}

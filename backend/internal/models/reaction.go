package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Reaction struct {
	ID        string    `gorm:"primaryKey" json:"id"`
	MessageID string    `gorm:"not null;index;uniqueIndex:idx_message_user" json:"message_id"`
	UserID    string    `gorm:"not null;index;uniqueIndex:idx_message_user" json:"user_id"`
	Emoji     string    `gorm:"not null" json:"emoji"`
	CreatedAt time.Time `json:"created_at"`

	Message Message `gorm:"foreignKey:MessageID" json:"-"`
	User    User    `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

func (r *Reaction) BeforeCreate(tx *gorm.DB) error {
	if r.ID == "" {
		r.ID = uuid.New().String()
	}
	return nil
}

// AddReaction adds or updates a reaction to a message
func AddReaction(db *gorm.DB, messageID, userID, emoji string) (*Reaction, error) {
	var existing Reaction
	result := db.Where("message_id = ? AND user_id = ?", messageID, userID).First(&existing)

	if result.Error == nil {
		// Update existing reaction
		existing.Emoji = emoji
		if err := db.Save(&existing).Error; err != nil {
			return nil, err
		}
		return &existing, nil
	}

	// Create new reaction
	reaction := Reaction{
		MessageID: messageID,
		UserID:    userID,
		Emoji:     emoji,
	}
	if err := db.Create(&reaction).Error; err != nil {
		return nil, err
	}
	return &reaction, nil
}

// RemoveReaction removes a user's reaction from a message
func RemoveReaction(db *gorm.DB, messageID, userID string) error {
	return db.Where("message_id = ? AND user_id = ?", messageID, userID).Delete(&Reaction{}).Error
}

// GetMessageReactions returns all reactions for a message
func GetMessageReactions(db *gorm.DB, messageID string) ([]Reaction, error) {
	var reactions []Reaction
	err := db.Preload("User").Where("message_id = ?", messageID).Find(&reactions).Error
	return reactions, err
}

// GetReactionSummary returns a summary of reactions for a message (emoji -> count)
func GetReactionSummary(db *gorm.DB, messageID string) (map[string]int, error) {
	var results []struct {
		Emoji string
		Count int
	}

	err := db.Model(&Reaction{}).
		Select("emoji, COUNT(*) as count").
		Where("message_id = ?", messageID).
		Group("emoji").
		Find(&results).Error

	if err != nil {
		return nil, err
	}

	summary := make(map[string]int)
	for _, r := range results {
		summary[r.Emoji] = r.Count
	}
	return summary, nil
}

// ReactionInfo contains reaction data for API responses
type ReactionInfo struct {
	Emoji  string   `json:"emoji"`
	Count  int      `json:"count"`
	Users  []string `json:"users"` // User IDs who reacted with this emoji
}

// GetMessageReactionInfo returns detailed reaction info for a message
func GetMessageReactionInfo(db *gorm.DB, messageID string) ([]ReactionInfo, error) {
	var reactions []Reaction
	if err := db.Where("message_id = ?", messageID).Find(&reactions).Error; err != nil {
		return nil, err
	}

	// Group by emoji
	emojiUsers := make(map[string][]string)
	for _, r := range reactions {
		emojiUsers[r.Emoji] = append(emojiUsers[r.Emoji], r.UserID)
	}

	// Convert to slice
	var info []ReactionInfo
	for emoji, users := range emojiUsers {
		info = append(info, ReactionInfo{
			Emoji: emoji,
			Count: len(users),
			Users: users,
		})
	}
	return info, nil
}

package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ChatTheme represents a user's theme preferences
type ChatTheme struct {
	ID                   string    `gorm:"primaryKey" json:"id"`
	UserID               string    `gorm:"not null;index" json:"user_id"`
	ConversationID       *string   `gorm:"index" json:"conversation_id,omitempty"` // User ID or Group ID, nil for global
	ConversationType     *string   `json:"conversation_type,omitempty"`            // "dm" or "group"
	PrimaryColor         string    `json:"primary_color,omitempty"`
	SecondaryColor       string    `json:"secondary_color,omitempty"`
	BackgroundColor      string    `json:"background_color,omitempty"`
	MessageBubbleColor   string    `json:"message_bubble_color,omitempty"`
	MessageTextColor     string    `json:"message_text_color,omitempty"`
	BackgroundImage      string    `json:"background_image,omitempty"` // URL or preset name
	FontSize             string    `json:"font_size,omitempty"`        // "small", "medium", "large"
	BubbleStyle          string    `json:"bubble_style,omitempty"`     // "rounded", "sharp", "minimal"
	DarkMode             *bool     `json:"dark_mode,omitempty"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`

	User User `gorm:"foreignKey:UserID" json:"-"`
}

func (t *ChatTheme) BeforeCreate(tx *gorm.DB) error {
	if t.ID == "" {
		t.ID = uuid.New().String()
	}
	return nil
}

// GetUserTheme gets a user's global theme
func GetUserTheme(db *gorm.DB, userID string) (*ChatTheme, error) {
	var theme ChatTheme
	err := db.Where("user_id = ? AND conversation_id IS NULL", userID).First(&theme).Error
	if err != nil {
		return nil, err
	}
	return &theme, nil
}

// GetConversationTheme gets theme for a specific conversation
func GetConversationTheme(db *gorm.DB, userID, conversationID, conversationType string) (*ChatTheme, error) {
	var theme ChatTheme
	err := db.Where("user_id = ? AND conversation_id = ? AND conversation_type = ?",
		userID, conversationID, conversationType).First(&theme).Error
	if err != nil {
		return nil, err
	}
	return &theme, nil
}

// GetEffectiveTheme returns the theme for a conversation, falling back to global
func GetEffectiveTheme(db *gorm.DB, userID, conversationID, conversationType string) (*ChatTheme, error) {
	// Try conversation-specific theme first
	theme, err := GetConversationTheme(db, userID, conversationID, conversationType)
	if err == nil {
		return theme, nil
	}

	// Fall back to global theme
	return GetUserTheme(db, userID)
}

// SetUserTheme creates or updates a user's theme
func SetUserTheme(db *gorm.DB, userID string, conversationID, conversationType *string, settings map[string]interface{}) (*ChatTheme, error) {
	var theme ChatTheme
	var err error

	if conversationID != nil {
		err = db.Where("user_id = ? AND conversation_id = ? AND conversation_type = ?",
			userID, *conversationID, *conversationType).First(&theme).Error
	} else {
		err = db.Where("user_id = ? AND conversation_id IS NULL", userID).First(&theme).Error
	}

	if err != nil {
		// Create new theme
		theme = ChatTheme{
			UserID:           userID,
			ConversationID:   conversationID,
			ConversationType: conversationType,
		}
	}

	// Apply settings
	if v, ok := settings["primary_color"].(string); ok {
		theme.PrimaryColor = v
	}
	if v, ok := settings["secondary_color"].(string); ok {
		theme.SecondaryColor = v
	}
	if v, ok := settings["background_color"].(string); ok {
		theme.BackgroundColor = v
	}
	if v, ok := settings["message_bubble_color"].(string); ok {
		theme.MessageBubbleColor = v
	}
	if v, ok := settings["message_text_color"].(string); ok {
		theme.MessageTextColor = v
	}
	if v, ok := settings["background_image"].(string); ok {
		theme.BackgroundImage = v
	}
	if v, ok := settings["font_size"].(string); ok {
		theme.FontSize = v
	}
	if v, ok := settings["bubble_style"].(string); ok {
		theme.BubbleStyle = v
	}
	if v, ok := settings["dark_mode"].(bool); ok {
		theme.DarkMode = &v
	}

	if theme.ID == "" {
		err = db.Create(&theme).Error
	} else {
		err = db.Save(&theme).Error
	}

	if err != nil {
		return nil, err
	}

	return &theme, nil
}

// DeleteConversationTheme removes a conversation-specific theme
func DeleteConversationTheme(db *gorm.DB, userID, conversationID, conversationType string) error {
	return db.Where("user_id = ? AND conversation_id = ? AND conversation_type = ?",
		userID, conversationID, conversationType).Delete(&ChatTheme{}).Error
}

// Preset themes
var PresetThemes = map[string]ChatTheme{
	"default": {
		PrimaryColor:       "#007AFF",
		BackgroundColor:    "#FFFFFF",
		MessageBubbleColor: "#E5E5EA",
		MessageTextColor:   "#000000",
		FontSize:           "medium",
		BubbleStyle:        "rounded",
	},
	"dark": {
		PrimaryColor:       "#0A84FF",
		BackgroundColor:    "#000000",
		MessageBubbleColor: "#2C2C2E",
		MessageTextColor:   "#FFFFFF",
		FontSize:           "medium",
		BubbleStyle:        "rounded",
	},
	"ocean": {
		PrimaryColor:       "#006994",
		BackgroundColor:    "#E8F4F8",
		MessageBubbleColor: "#B8D4E3",
		MessageTextColor:   "#1A3A4A",
		FontSize:           "medium",
		BubbleStyle:        "rounded",
	},
	"forest": {
		PrimaryColor:       "#228B22",
		BackgroundColor:    "#F0F8F0",
		MessageBubbleColor: "#C8E6C9",
		MessageTextColor:   "#1B4332",
		FontSize:           "medium",
		BubbleStyle:        "rounded",
	},
	"sunset": {
		PrimaryColor:       "#FF6B35",
		BackgroundColor:    "#FFF8F0",
		MessageBubbleColor: "#FFE0CC",
		MessageTextColor:   "#5D2E0C",
		FontSize:           "medium",
		BubbleStyle:        "rounded",
	},
	"lavender": {
		PrimaryColor:       "#7B68EE",
		BackgroundColor:    "#F8F0FF",
		MessageBubbleColor: "#E6D5F2",
		MessageTextColor:   "#4A3668",
		FontSize:           "medium",
		BubbleStyle:        "rounded",
	},
}

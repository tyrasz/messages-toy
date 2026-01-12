package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// DisappearingDuration represents preset durations for disappearing messages
type DisappearingDuration int

const (
	DisappearingOff      DisappearingDuration = 0
	Disappearing24Hours  DisappearingDuration = 86400    // 24 hours in seconds
	Disappearing7Days    DisappearingDuration = 604800   // 7 days in seconds
	Disappearing90Days   DisappearingDuration = 7776000  // 90 days in seconds
)

// ConversationSettings stores per-conversation settings like disappearing messages
type ConversationSettings struct {
	ID                    string    `gorm:"primaryKey" json:"id"`
	UserID                string    `gorm:"not null;index;uniqueIndex:idx_user_conversation" json:"user_id"`
	OtherUserID           *string   `gorm:"index;uniqueIndex:idx_user_conversation" json:"other_user_id,omitempty"` // For DMs
	GroupID               *string   `gorm:"index;uniqueIndex:idx_user_conversation" json:"group_id,omitempty"`      // For groups
	DisappearingSeconds   int       `gorm:"default:0" json:"disappearing_seconds"`                                   // 0 = off
	MutedUntil            *time.Time `json:"muted_until,omitempty"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
}

func (cs *ConversationSettings) BeforeCreate(tx *gorm.DB) error {
	if cs.ID == "" {
		cs.ID = uuid.New().String()
	}
	return nil
}

func (cs *ConversationSettings) IsDisappearingEnabled() bool {
	return cs.DisappearingSeconds > 0
}

func (cs *ConversationSettings) IsMuted() bool {
	if cs.MutedUntil == nil {
		return false
	}
	return cs.MutedUntil.After(time.Now())
}

// GetOrCreateDMSettings gets or creates settings for a DM conversation
func GetOrCreateDMSettings(db *gorm.DB, userID, otherUserID string) (*ConversationSettings, error) {
	var settings ConversationSettings
	err := db.Where("user_id = ? AND other_user_id = ?", userID, otherUserID).First(&settings).Error

	if err == gorm.ErrRecordNotFound {
		settings = ConversationSettings{
			UserID:      userID,
			OtherUserID: &otherUserID,
		}
		if err := db.Create(&settings).Error; err != nil {
			return nil, err
		}
		return &settings, nil
	}

	return &settings, err
}

// GetOrCreateGroupSettings gets or creates settings for a group conversation
func GetOrCreateGroupSettings(db *gorm.DB, userID, groupID string) (*ConversationSettings, error) {
	var settings ConversationSettings
	err := db.Where("user_id = ? AND group_id = ?", userID, groupID).First(&settings).Error

	if err == gorm.ErrRecordNotFound {
		settings = ConversationSettings{
			UserID:  userID,
			GroupID: &groupID,
		}
		if err := db.Create(&settings).Error; err != nil {
			return nil, err
		}
		return &settings, nil
	}

	return &settings, err
}

// SetDisappearingTimer sets the disappearing message timer for a conversation
func SetDisappearingTimer(db *gorm.DB, userID string, otherUserID *string, groupID *string, seconds int) error {
	var settings ConversationSettings
	var err error

	if otherUserID != nil {
		err = db.Where("user_id = ? AND other_user_id = ?", userID, *otherUserID).First(&settings).Error
	} else if groupID != nil {
		err = db.Where("user_id = ? AND group_id = ?", userID, *groupID).First(&settings).Error
	}

	if err == gorm.ErrRecordNotFound {
		settings = ConversationSettings{
			UserID:              userID,
			OtherUserID:         otherUserID,
			GroupID:             groupID,
			DisappearingSeconds: seconds,
		}
		return db.Create(&settings).Error
	}

	if err != nil {
		return err
	}

	return db.Model(&settings).Update("disappearing_seconds", seconds).Error
}

// GetDisappearingTimer returns the disappearing timer for a conversation
func GetDisappearingTimer(db *gorm.DB, userID string, otherUserID *string, groupID *string) int {
	var settings ConversationSettings
	var err error

	if otherUserID != nil {
		err = db.Where("user_id = ? AND other_user_id = ?", userID, *otherUserID).First(&settings).Error
	} else if groupID != nil {
		err = db.Where("user_id = ? AND group_id = ?", userID, *groupID).First(&settings).Error
	}

	if err != nil {
		return 0
	}

	return settings.DisappearingSeconds
}

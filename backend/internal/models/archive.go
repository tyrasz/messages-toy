package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ArchivedConversation tracks conversations that a user has archived
type ArchivedConversation struct {
	ID              string    `gorm:"primaryKey" json:"id"`
	UserID          string    `gorm:"not null;index" json:"user_id"`
	OtherUserID     *string   `gorm:"index;uniqueIndex:idx_archived_dm" json:"other_user_id,omitempty"` // For DMs
	GroupID         *string   `gorm:"index;uniqueIndex:idx_archived_group" json:"group_id,omitempty"`   // For groups
	ConversationKey *string   `gorm:"uniqueIndex:idx_archived_dm" json:"-"`                             // Sorted user IDs for DMs
	ArchivedAt      time.Time `json:"archived_at"`

	User      User   `gorm:"foreignKey:UserID" json:"-"`
	OtherUser *User  `gorm:"foreignKey:OtherUserID" json:"-"`
	Group     *Group `gorm:"foreignKey:GroupID" json:"-"`
}

func (a *ArchivedConversation) BeforeCreate(tx *gorm.DB) error {
	if a.ID == "" {
		a.ID = uuid.New().String()
	}
	if a.ArchivedAt.IsZero() {
		a.ArchivedAt = time.Now()
	}
	// Generate conversation key for DMs
	if a.OtherUserID != nil && a.ConversationKey == nil {
		key := MakeConversationKey(a.UserID, *a.OtherUserID)
		a.ConversationKey = &key
	}
	return nil
}

// ArchiveConversation archives a conversation for a user
func ArchiveConversation(db *gorm.DB, userID string, otherUserID *string, groupID *string) (*ArchivedConversation, error) {
	archive := &ArchivedConversation{
		UserID:      userID,
		OtherUserID: otherUserID,
		GroupID:     groupID,
		ArchivedAt:  time.Now(),
	}

	// Check if already archived
	query := db.Where("user_id = ?", userID)
	if groupID != nil {
		query = query.Where("group_id = ?", *groupID)
	} else if otherUserID != nil {
		key := MakeConversationKey(userID, *otherUserID)
		query = query.Where("conversation_key = ?", key)
	}

	var existing ArchivedConversation
	if err := query.First(&existing).Error; err == nil {
		return &existing, nil // Already archived
	}

	if err := db.Create(archive).Error; err != nil {
		return nil, err
	}
	return archive, nil
}

// UnarchiveConversation removes a conversation from archives
func UnarchiveConversation(db *gorm.DB, userID string, otherUserID *string, groupID *string) error {
	query := db.Where("user_id = ?", userID)
	if groupID != nil {
		query = query.Where("group_id = ?", *groupID)
	} else if otherUserID != nil {
		key := MakeConversationKey(userID, *otherUserID)
		query = query.Where("conversation_key = ?", key)
	}
	return query.Delete(&ArchivedConversation{}).Error
}

// IsConversationArchived checks if a conversation is archived
func IsConversationArchived(db *gorm.DB, userID string, otherUserID *string, groupID *string) bool {
	query := db.Model(&ArchivedConversation{}).Where("user_id = ?", userID)
	if groupID != nil {
		query = query.Where("group_id = ?", *groupID)
	} else if otherUserID != nil {
		key := MakeConversationKey(userID, *otherUserID)
		query = query.Where("conversation_key = ?", key)
	}
	var count int64
	query.Count(&count)
	return count > 0
}

// GetArchivedConversations returns all archived conversations for a user
func GetArchivedConversations(db *gorm.DB, userID string) ([]ArchivedConversation, error) {
	var archives []ArchivedConversation
	err := db.Preload("OtherUser").Preload("Group").
		Where("user_id = ?", userID).
		Order("archived_at DESC").
		Find(&archives).Error
	return archives, err
}

// GetArchivedGroupIDs returns IDs of archived groups for a user
func GetArchivedGroupIDs(db *gorm.DB, userID string) ([]string, error) {
	var archives []ArchivedConversation
	err := db.Where("user_id = ? AND group_id IS NOT NULL", userID).Find(&archives).Error
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(archives))
	for _, a := range archives {
		if a.GroupID != nil {
			ids = append(ids, *a.GroupID)
		}
	}
	return ids, nil
}

// GetArchivedDMUserIDs returns IDs of users in archived DM conversations
func GetArchivedDMUserIDs(db *gorm.DB, userID string) ([]string, error) {
	var archives []ArchivedConversation
	err := db.Where("user_id = ? AND other_user_id IS NOT NULL", userID).Find(&archives).Error
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(archives))
	for _, a := range archives {
		if a.OtherUserID != nil {
			ids = append(ids, *a.OtherUserID)
		}
	}
	return ids, nil
}

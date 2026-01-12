package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Block struct {
	ID        string    `gorm:"primaryKey" json:"id"`
	BlockerID string    `gorm:"not null;index;uniqueIndex:idx_blocker_blocked" json:"blocker_id"`
	BlockedID string    `gorm:"not null;index;uniqueIndex:idx_blocker_blocked" json:"blocked_id"`
	CreatedAt time.Time `json:"created_at"`

	Blocker User `gorm:"foreignKey:BlockerID" json:"-"`
	Blocked User `gorm:"foreignKey:BlockedID" json:"blocked_user,omitempty"`
}

func (b *Block) BeforeCreate(tx *gorm.DB) error {
	if b.ID == "" {
		b.ID = uuid.New().String()
	}
	return nil
}

// IsBlocked checks if blockerID has blocked blockedID
func IsBlocked(db *gorm.DB, blockerID, blockedID string) bool {
	var count int64
	db.Model(&Block{}).Where("blocker_id = ? AND blocked_id = ?", blockerID, blockedID).Count(&count)
	return count > 0
}

// IsEitherBlocked checks if either user has blocked the other
func IsEitherBlocked(db *gorm.DB, userID1, userID2 string) bool {
	var count int64
	db.Model(&Block{}).Where(
		"(blocker_id = ? AND blocked_id = ?) OR (blocker_id = ? AND blocked_id = ?)",
		userID1, userID2, userID2, userID1,
	).Count(&count)
	return count > 0
}

package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// BroadcastList represents a list of recipients for broadcasting messages
type BroadcastList struct {
	ID        string    `gorm:"primaryKey" json:"id"`
	OwnerID   string    `gorm:"not null;index" json:"owner_id"`
	Name      string    `gorm:"not null" json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	Owner      User                    `gorm:"foreignKey:OwnerID" json:"-"`
	Recipients []BroadcastListRecipient `gorm:"foreignKey:BroadcastListID" json:"recipients,omitempty"`
}

func (b *BroadcastList) BeforeCreate(tx *gorm.DB) error {
	if b.ID == "" {
		b.ID = uuid.New().String()
	}
	return nil
}

// BroadcastListRecipient represents a recipient in a broadcast list
type BroadcastListRecipient struct {
	ID              string    `gorm:"primaryKey" json:"id"`
	BroadcastListID string    `gorm:"not null;index;uniqueIndex:idx_list_recipient" json:"broadcast_list_id"`
	RecipientID     string    `gorm:"not null;index;uniqueIndex:idx_list_recipient" json:"recipient_id"`
	CreatedAt       time.Time `json:"created_at"`

	Recipient User `gorm:"foreignKey:RecipientID" json:"recipient,omitempty"`
}

func (r *BroadcastListRecipient) BeforeCreate(tx *gorm.DB) error {
	if r.ID == "" {
		r.ID = uuid.New().String()
	}
	return nil
}

// GetBroadcastLists returns all broadcast lists for a user
func GetBroadcastLists(db *gorm.DB, userID string) ([]BroadcastList, error) {
	var lists []BroadcastList
	err := db.Preload("Recipients.Recipient").Where("owner_id = ?", userID).Find(&lists).Error
	return lists, err
}

// GetBroadcastList returns a specific broadcast list with recipients
func GetBroadcastList(db *gorm.DB, listID, userID string) (*BroadcastList, error) {
	var list BroadcastList
	err := db.Preload("Recipients.Recipient").Where("id = ? AND owner_id = ?", listID, userID).First(&list).Error
	if err != nil {
		return nil, err
	}
	return &list, nil
}

// CreateBroadcastList creates a new broadcast list
func CreateBroadcastList(db *gorm.DB, ownerID, name string, recipientIDs []string) (*BroadcastList, error) {
	list := &BroadcastList{
		OwnerID: ownerID,
		Name:    name,
	}

	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(list).Error; err != nil {
			return err
		}

		// Add recipients
		for _, recipientID := range recipientIDs {
			// Skip self
			if recipientID == ownerID {
				continue
			}
			recipient := BroadcastListRecipient{
				BroadcastListID: list.ID,
				RecipientID:     recipientID,
			}
			if err := tx.Create(&recipient).Error; err != nil {
				// Skip duplicates
				continue
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Reload with recipients
	return GetBroadcastList(db, list.ID, ownerID)
}

// AddRecipientToBroadcastList adds a recipient to a broadcast list
func AddRecipientToBroadcastList(db *gorm.DB, listID, recipientID string) error {
	recipient := BroadcastListRecipient{
		BroadcastListID: listID,
		RecipientID:     recipientID,
	}
	return db.Create(&recipient).Error
}

// RemoveRecipientFromBroadcastList removes a recipient from a broadcast list
func RemoveRecipientFromBroadcastList(db *gorm.DB, listID, recipientID string) error {
	return db.Where("broadcast_list_id = ? AND recipient_id = ?", listID, recipientID).Delete(&BroadcastListRecipient{}).Error
}

// DeleteBroadcastList deletes a broadcast list and its recipients
func DeleteBroadcastList(db *gorm.DB, listID, ownerID string) error {
	return db.Transaction(func(tx *gorm.DB) error {
		// Delete recipients first
		if err := tx.Where("broadcast_list_id = ?", listID).Delete(&BroadcastListRecipient{}).Error; err != nil {
			return err
		}
		// Delete list
		return tx.Where("id = ? AND owner_id = ?", listID, ownerID).Delete(&BroadcastList{}).Error
	})
}

// GetBroadcastListRecipientIDs returns all recipient IDs for a broadcast list
func GetBroadcastListRecipientIDs(db *gorm.DB, listID string) ([]string, error) {
	var recipientIDs []string
	err := db.Model(&BroadcastListRecipient{}).Where("broadcast_list_id = ?", listID).Pluck("recipient_id", &recipientIDs).Error
	return recipientIDs, err
}

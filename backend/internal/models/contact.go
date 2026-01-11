package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Contact struct {
	ID        string    `gorm:"primaryKey" json:"id"`
	UserID    string    `gorm:"not null;index;uniqueIndex:idx_user_contact" json:"user_id"`
	ContactID string    `gorm:"not null;index;uniqueIndex:idx_user_contact" json:"contact_id"`
	Nickname  string    `json:"nickname,omitempty"`
	CreatedAt time.Time `json:"created_at"`

	User        User `gorm:"foreignKey:UserID" json:"-"`
	ContactUser User `gorm:"foreignKey:ContactID" json:"contact,omitempty"`
}

func (c *Contact) BeforeCreate(tx *gorm.DB) error {
	if c.ID == "" {
		c.ID = uuid.New().String()
	}
	return nil
}

type ContactWithStatus struct {
	Contact
	Online bool `json:"online"`
}

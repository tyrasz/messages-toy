package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type User struct {
	ID           string    `gorm:"primaryKey" json:"id"`
	Username     string    `gorm:"uniqueIndex;not null" json:"username"`
	Phone        *string   `gorm:"uniqueIndex" json:"phone,omitempty"`
	PasswordHash string    `gorm:"not null" json:"-"`
	DisplayName  string    `json:"display_name,omitempty"`
	AvatarURL    string    `json:"avatar_url,omitempty"`
	About        string    `json:"about,omitempty"`         // Status/bio text
	StatusEmoji  string    `json:"status_emoji,omitempty"`  // Optional status emoji
	LastSeen     time.Time `json:"last_seen,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.ID == "" {
		u.ID = uuid.New().String()
	}
	return nil
}

type UserResponse struct {
	ID          string    `json:"id"`
	Username    string    `json:"username"`
	DisplayName string    `json:"display_name,omitempty"`
	AvatarURL   string    `json:"avatar_url,omitempty"`
	About       string    `json:"about,omitempty"`
	StatusEmoji string    `json:"status_emoji,omitempty"`
	LastSeen    time.Time `json:"last_seen,omitempty"`
	Online      bool      `json:"online"`
}

func (u *User) ToResponse(online bool) UserResponse {
	return UserResponse{
		ID:          u.ID,
		Username:    u.Username,
		DisplayName: u.DisplayName,
		AvatarURL:   u.AvatarURL,
		About:       u.About,
		StatusEmoji: u.StatusEmoji,
		LastSeen:    u.LastSeen,
		Online:      online,
	}
}

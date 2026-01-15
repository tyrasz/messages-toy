package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type UserRole string

const (
	UserRoleUser      UserRole = "user"
	UserRoleModerator UserRole = "moderator"
	UserRoleAdmin     UserRole = "admin"
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
	Role         UserRole  `gorm:"default:user" json:"role,omitempty"`
	LastSeen     time.Time `json:"last_seen,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// IsAdmin returns true if the user has admin role
func (u *User) IsAdmin() bool {
	return u.Role == UserRoleAdmin
}

// IsModerator returns true if the user has moderator or admin role
func (u *User) IsModerator() bool {
	return u.Role == UserRoleModerator || u.Role == UserRoleAdmin
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

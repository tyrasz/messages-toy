package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type DevicePlatform string

const (
	PlatformIOS     DevicePlatform = "ios"
	PlatformAndroid DevicePlatform = "android"
	PlatformWeb     DevicePlatform = "web"
)

type DeviceToken struct {
	ID        string         `gorm:"primaryKey" json:"id"`
	UserID    string         `gorm:"not null;index" json:"user_id"`
	Token     string         `gorm:"not null;uniqueIndex" json:"token"`
	Platform  DevicePlatform `gorm:"not null" json:"platform"`
	DeviceID  string         `json:"device_id,omitempty"` // Optional device identifier
	AppVersion string        `json:"app_version,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`

	User User `gorm:"foreignKey:UserID" json:"-"`
}

func (dt *DeviceToken) BeforeCreate(tx *gorm.DB) error {
	if dt.ID == "" {
		dt.ID = uuid.New().String()
	}
	return nil
}

// GetUserTokens retrieves all device tokens for a user
func GetUserTokens(db *gorm.DB, userID string) ([]DeviceToken, error) {
	var tokens []DeviceToken
	err := db.Where("user_id = ?", userID).Find(&tokens).Error
	return tokens, err
}

// RegisterToken creates or updates a device token for a user
func RegisterToken(db *gorm.DB, userID, token string, platform DevicePlatform, deviceID, appVersion string) (*DeviceToken, error) {
	var existingToken DeviceToken

	// Check if token already exists
	result := db.Where("token = ?", token).First(&existingToken)
	if result.Error == nil {
		// Token exists, update it
		existingToken.UserID = userID
		existingToken.Platform = platform
		existingToken.DeviceID = deviceID
		existingToken.AppVersion = appVersion
		if err := db.Save(&existingToken).Error; err != nil {
			return nil, err
		}
		return &existingToken, nil
	}

	// Create new token
	newToken := DeviceToken{
		UserID:     userID,
		Token:      token,
		Platform:   platform,
		DeviceID:   deviceID,
		AppVersion: appVersion,
	}
	if err := db.Create(&newToken).Error; err != nil {
		return nil, err
	}
	return &newToken, nil
}

// UnregisterToken removes a device token
func UnregisterToken(db *gorm.DB, token string) error {
	return db.Where("token = ?", token).Delete(&DeviceToken{}).Error
}

// UnregisterUserTokens removes all device tokens for a user (on logout)
func UnregisterUserTokens(db *gorm.DB, userID string) error {
	return db.Where("user_id = ?", userID).Delete(&DeviceToken{}).Error
}

package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// EncryptionDevice represents a device registered for E2EE
// This is separate from DeviceToken (push notifications)
type EncryptionDevice struct {
	ID           string    `gorm:"primaryKey" json:"id"`
	UserID       string    `gorm:"not null;index:idx_enc_device_user" json:"user_id"`
	DeviceID     string    `gorm:"not null;uniqueIndex:idx_enc_device_user_device" json:"device_id"`
	Name         string    `json:"name,omitempty"`       // e.g., "iPhone 15", "Chrome on Mac"
	Platform     string    `json:"platform,omitempty"`   // "ios", "android", "web"
	LastActiveAt time.Time `json:"last_active_at"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`

	User User `gorm:"foreignKey:UserID" json:"-"`
}

func (ed *EncryptionDevice) BeforeCreate(tx *gorm.DB) error {
	if ed.ID == "" {
		ed.ID = uuid.New().String()
	}
	return nil
}

// EncryptionDeviceResponse is the API response format
type EncryptionDeviceResponse struct {
	DeviceID     string    `json:"device_id"`
	Name         string    `json:"name,omitempty"`
	Platform     string    `json:"platform,omitempty"`
	LastActiveAt time.Time `json:"last_active_at"`
	CreatedAt    time.Time `json:"created_at"`
	IsCurrent    bool      `json:"is_current"`
}

func (ed *EncryptionDevice) ToResponse(currentDeviceID string) EncryptionDeviceResponse {
	return EncryptionDeviceResponse{
		DeviceID:     ed.DeviceID,
		Name:         ed.Name,
		Platform:     ed.Platform,
		LastActiveAt: ed.LastActiveAt,
		CreatedAt:    ed.CreatedAt,
		IsCurrent:    ed.DeviceID == currentDeviceID,
	}
}

// GetUserDevices retrieves all encryption devices for a user
func GetUserDevices(db *gorm.DB, userID string) ([]EncryptionDevice, error) {
	var devices []EncryptionDevice
	err := db.Where("user_id = ?", userID).Order("last_active_at DESC").Find(&devices).Error
	return devices, err
}

// GetDevice retrieves a specific device
func GetDevice(db *gorm.DB, userID, deviceID string) (*EncryptionDevice, error) {
	var device EncryptionDevice
	err := db.Where("user_id = ? AND device_id = ?", userID, deviceID).First(&device).Error
	return &device, err
}

// RegisterDevice creates or updates an encryption device
func RegisterDevice(db *gorm.DB, userID, deviceID, name, platform string) (*EncryptionDevice, error) {
	var existing EncryptionDevice
	result := db.Where("user_id = ? AND device_id = ?", userID, deviceID).First(&existing)

	now := time.Now()

	if result.Error == nil {
		// Update existing
		existing.Name = name
		existing.Platform = platform
		existing.LastActiveAt = now
		if err := db.Save(&existing).Error; err != nil {
			return nil, err
		}
		return &existing, nil
	}

	// Create new
	device := EncryptionDevice{
		UserID:       userID,
		DeviceID:     deviceID,
		Name:         name,
		Platform:     platform,
		LastActiveAt: now,
	}
	if err := db.Create(&device).Error; err != nil {
		return nil, err
	}
	return &device, nil
}

// UpdateDeviceActivity updates the last active timestamp
func UpdateDeviceActivity(db *gorm.DB, userID, deviceID string) error {
	return db.Model(&EncryptionDevice{}).
		Where("user_id = ? AND device_id = ?", userID, deviceID).
		Update("last_active_at", time.Now()).Error
}

// RemoveDevice removes a device and all its encryption keys
func RemoveDevice(db *gorm.DB, userID, deviceID string) error {
	// Delete all keys first
	if err := DeleteDeviceKeys(db, userID, deviceID); err != nil {
		return err
	}

	// Delete sender keys for this device
	if err := db.Where("user_id = ? AND device_id = ?", userID, deviceID).Delete(&SenderKey{}).Error; err != nil {
		return err
	}

	// Delete the device record
	return db.Where("user_id = ? AND device_id = ?", userID, deviceID).Delete(&EncryptionDevice{}).Error
}

// CountUserDevices returns the number of devices for a user
func CountUserDevices(db *gorm.DB, userID string) (int64, error) {
	var count int64
	err := db.Model(&EncryptionDevice{}).Where("user_id = ?", userID).Count(&count).Error
	return count, err
}

package models

import (
	"encoding/base64"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// IdentityKey stores the public identity key for a user's device
// Private keys never leave the client device
type IdentityKey struct {
	ID             string    `gorm:"primaryKey" json:"id"`
	UserID         string    `gorm:"not null;uniqueIndex:idx_identity_user_device" json:"user_id"`
	DeviceID       string    `gorm:"not null;uniqueIndex:idx_identity_user_device" json:"device_id"`
	RegistrationID uint32    `gorm:"not null" json:"registration_id"`
	PublicKey      []byte    `gorm:"not null;size:32" json:"-"` // X25519 public key (32 bytes)
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`

	User User `gorm:"foreignKey:UserID" json:"-"`
}

func (ik *IdentityKey) BeforeCreate(tx *gorm.DB) error {
	if ik.ID == "" {
		ik.ID = uuid.New().String()
	}
	return nil
}

// PublicKeyBase64 returns the public key as a base64 string
func (ik *IdentityKey) PublicKeyBase64() string {
	return base64.StdEncoding.EncodeToString(ik.PublicKey)
}

// SetPublicKeyFromBase64 sets the public key from a base64 string
func (ik *IdentityKey) SetPublicKeyFromBase64(b64 string) error {
	key, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return err
	}
	ik.PublicKey = key
	return nil
}

// IdentityKeyResponse is the API response format for identity keys
type IdentityKeyResponse struct {
	UserID         string `json:"user_id"`
	DeviceID       string `json:"device_id"`
	RegistrationID uint32 `json:"registration_id"`
	PublicKey      string `json:"public_key"` // Base64 encoded
}

func (ik *IdentityKey) ToResponse() IdentityKeyResponse {
	return IdentityKeyResponse{
		UserID:         ik.UserID,
		DeviceID:       ik.DeviceID,
		RegistrationID: ik.RegistrationID,
		PublicKey:      ik.PublicKeyBase64(),
	}
}

// GetIdentityKey retrieves an identity key for a user's device
func GetIdentityKey(db *gorm.DB, userID, deviceID string) (*IdentityKey, error) {
	var key IdentityKey
	err := db.Where("user_id = ? AND device_id = ?", userID, deviceID).First(&key).Error
	return &key, err
}

// GetUserIdentityKeys retrieves all identity keys for a user (all devices)
func GetUserIdentityKeys(db *gorm.DB, userID string) ([]IdentityKey, error) {
	var keys []IdentityKey
	err := db.Where("user_id = ?", userID).Find(&keys).Error
	return keys, err
}

// SaveIdentityKey creates or updates an identity key for a device
func SaveIdentityKey(db *gorm.DB, userID, deviceID string, registrationID uint32, publicKey []byte) (*IdentityKey, error) {
	var existing IdentityKey
	result := db.Where("user_id = ? AND device_id = ?", userID, deviceID).First(&existing)

	if result.Error == nil {
		// Update existing
		existing.RegistrationID = registrationID
		existing.PublicKey = publicKey
		if err := db.Save(&existing).Error; err != nil {
			return nil, err
		}
		return &existing, nil
	}

	// Create new
	key := IdentityKey{
		UserID:         userID,
		DeviceID:       deviceID,
		RegistrationID: registrationID,
		PublicKey:      publicKey,
	}
	if err := db.Create(&key).Error; err != nil {
		return nil, err
	}
	return &key, nil
}

// DeleteIdentityKey removes an identity key (when device is removed)
func DeleteIdentityKey(db *gorm.DB, userID, deviceID string) error {
	return db.Where("user_id = ? AND device_id = ?", userID, deviceID).Delete(&IdentityKey{}).Error
}

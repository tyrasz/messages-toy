package models

import (
	"encoding/base64"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// SenderKey stores sender keys for efficient group message encryption
// Using Signal's Sender Keys protocol, each group member has their own sender key
// that they use to encrypt messages to the group
type SenderKey struct {
	ID         string    `gorm:"primaryKey" json:"id"`
	GroupID    string    `gorm:"not null;uniqueIndex:idx_sender_key_group_user_device" json:"group_id"`
	UserID     string    `gorm:"not null;uniqueIndex:idx_sender_key_group_user_device;index:idx_sender_key_user" json:"user_id"`
	DeviceID   string    `gorm:"not null;uniqueIndex:idx_sender_key_group_user_device" json:"device_id"`
	KeyID      uint32    `gorm:"not null" json:"key_id"`
	ChainKey   []byte    `gorm:"not null;size:32" json:"-"` // Current chain key
	SigningKey []byte    `gorm:"not null;size:32" json:"-"` // Public signing key
	Iteration  uint32    `gorm:"not null" json:"iteration"` // Current chain iteration
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`

	User  User  `gorm:"foreignKey:UserID" json:"-"`
	Group Group `gorm:"foreignKey:GroupID" json:"-"`
}

func (sk *SenderKey) BeforeCreate(tx *gorm.DB) error {
	if sk.ID == "" {
		sk.ID = uuid.New().String()
	}
	return nil
}

// ChainKeyBase64 returns the chain key as a base64 string
func (sk *SenderKey) ChainKeyBase64() string {
	return base64.StdEncoding.EncodeToString(sk.ChainKey)
}

// SigningKeyBase64 returns the signing key as a base64 string
func (sk *SenderKey) SigningKeyBase64() string {
	return base64.StdEncoding.EncodeToString(sk.SigningKey)
}

// SenderKeyResponse is the API response format
type SenderKeyResponse struct {
	GroupID    string `json:"group_id"`
	UserID     string `json:"user_id"`
	DeviceID   string `json:"device_id"`
	KeyID      uint32 `json:"key_id"`
	ChainKey   string `json:"chain_key"`   // Base64 encoded
	SigningKey string `json:"signing_key"` // Base64 encoded
	Iteration  uint32 `json:"iteration"`
}

func (sk *SenderKey) ToResponse() SenderKeyResponse {
	return SenderKeyResponse{
		GroupID:    sk.GroupID,
		UserID:     sk.UserID,
		DeviceID:   sk.DeviceID,
		KeyID:      sk.KeyID,
		ChainKey:   sk.ChainKeyBase64(),
		SigningKey: sk.SigningKeyBase64(),
		Iteration:  sk.Iteration,
	}
}

// SenderKeyDistribution represents a sender key distribution message
// These are sent to group members encrypted with pairwise sessions
type SenderKeyDistribution struct {
	GroupID    string `json:"group_id"`
	SenderID   string `json:"sender_id"`
	DeviceID   string `json:"device_id"`
	KeyID      uint32 `json:"key_id"`
	ChainKey   string `json:"chain_key"`   // Base64 encoded
	SigningKey string `json:"signing_key"` // Base64 encoded
}

// GetSenderKey retrieves a sender key for a group member's device
func GetSenderKey(db *gorm.DB, groupID, userID, deviceID string) (*SenderKey, error) {
	var key SenderKey
	err := db.Where("group_id = ? AND user_id = ? AND device_id = ?", groupID, userID, deviceID).First(&key).Error
	return &key, err
}

// GetGroupSenderKeys retrieves all sender keys for a group (for new members)
func GetGroupSenderKeys(db *gorm.DB, groupID string) ([]SenderKey, error) {
	var keys []SenderKey
	err := db.Where("group_id = ?", groupID).Find(&keys).Error
	return keys, err
}

// SaveSenderKey creates or updates a sender key
func SaveSenderKey(db *gorm.DB, groupID, userID, deviceID string, keyID uint32, chainKey, signingKey []byte) (*SenderKey, error) {
	var existing SenderKey
	result := db.Where("group_id = ? AND user_id = ? AND device_id = ?", groupID, userID, deviceID).First(&existing)

	if result.Error == nil {
		// Update existing
		existing.KeyID = keyID
		existing.ChainKey = chainKey
		existing.SigningKey = signingKey
		existing.Iteration = 0
		if err := db.Save(&existing).Error; err != nil {
			return nil, err
		}
		return &existing, nil
	}

	// Create new
	key := SenderKey{
		GroupID:    groupID,
		UserID:     userID,
		DeviceID:   deviceID,
		KeyID:      keyID,
		ChainKey:   chainKey,
		SigningKey: signingKey,
		Iteration:  0,
	}
	if err := db.Create(&key).Error; err != nil {
		return nil, err
	}
	return &key, nil
}

// UpdateSenderKeyIteration updates the chain iteration after decryption
func UpdateSenderKeyIteration(db *gorm.DB, id string, newIteration uint32, newChainKey []byte) error {
	return db.Model(&SenderKey{}).Where("id = ?", id).Updates(map[string]interface{}{
		"iteration": newIteration,
		"chain_key": newChainKey,
	}).Error
}

// DeleteGroupSenderKeys removes all sender keys for a group (when leaving group)
func DeleteGroupSenderKeys(db *gorm.DB, groupID, userID string) error {
	return db.Where("group_id = ? AND user_id = ?", groupID, userID).Delete(&SenderKey{}).Error
}

// DeleteAllGroupSenderKeys removes all sender keys for a group (when group is deleted)
func DeleteAllGroupSenderKeys(db *gorm.DB, groupID string) error {
	return db.Where("group_id = ?", groupID).Delete(&SenderKey{}).Error
}

// GetUserSenderKeys retrieves all sender keys created by a user (for backup)
func GetUserSenderKeys(db *gorm.DB, userID string) ([]SenderKey, error) {
	var keys []SenderKey
	err := db.Where("user_id = ?", userID).Find(&keys).Error
	return keys, err
}

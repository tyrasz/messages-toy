package models

import (
	"encoding/base64"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// PreKey stores one-time prekeys for X3DH key agreement
// Each prekey can only be used once for establishing a session
type PreKey struct {
	ID        string    `gorm:"primaryKey" json:"id"`
	UserID    string    `gorm:"not null;index:idx_prekey_user" json:"user_id"`
	DeviceID  string    `gorm:"not null;index:idx_prekey_user_device" json:"device_id"`
	KeyID     uint32    `gorm:"not null;uniqueIndex:idx_prekey_user_device_keyid,priority:3" json:"key_id"`
	PublicKey []byte    `gorm:"not null;size:32" json:"-"` // X25519 public key
	CreatedAt time.Time `json:"created_at"`

	User User `gorm:"foreignKey:UserID" json:"-"`
}

func (pk *PreKey) BeforeCreate(tx *gorm.DB) error {
	if pk.ID == "" {
		pk.ID = uuid.New().String()
	}
	return nil
}

// PublicKeyBase64 returns the public key as a base64 string
func (pk *PreKey) PublicKeyBase64() string {
	return base64.StdEncoding.EncodeToString(pk.PublicKey)
}

// PreKeyResponse is the API response format
type PreKeyResponse struct {
	KeyID     uint32 `json:"key_id"`
	PublicKey string `json:"public_key"` // Base64 encoded
}

func (pk *PreKey) ToResponse() PreKeyResponse {
	return PreKeyResponse{
		KeyID:     pk.KeyID,
		PublicKey: pk.PublicKeyBase64(),
	}
}

// SignedPreKey stores the signed prekey for X3DH
// Unlike one-time prekeys, signed prekeys are rotated periodically (not per-session)
type SignedPreKey struct {
	ID        string    `gorm:"primaryKey" json:"id"`
	UserID    string    `gorm:"not null;uniqueIndex:idx_signed_prekey_user_device" json:"user_id"`
	DeviceID  string    `gorm:"not null;uniqueIndex:idx_signed_prekey_user_device" json:"device_id"`
	KeyID     uint32    `gorm:"not null" json:"key_id"`
	PublicKey []byte    `gorm:"not null;size:32" json:"-"` // X25519 public key
	Signature []byte    `gorm:"not null;size:64" json:"-"` // Ed25519 signature
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	User User `gorm:"foreignKey:UserID" json:"-"`
}

func (spk *SignedPreKey) BeforeCreate(tx *gorm.DB) error {
	if spk.ID == "" {
		spk.ID = uuid.New().String()
	}
	return nil
}

// PublicKeyBase64 returns the public key as a base64 string
func (spk *SignedPreKey) PublicKeyBase64() string {
	return base64.StdEncoding.EncodeToString(spk.PublicKey)
}

// SignatureBase64 returns the signature as a base64 string
func (spk *SignedPreKey) SignatureBase64() string {
	return base64.StdEncoding.EncodeToString(spk.Signature)
}

// SignedPreKeyResponse is the API response format
type SignedPreKeyResponse struct {
	KeyID     uint32 `json:"key_id"`
	PublicKey string `json:"public_key"` // Base64 encoded
	Signature string `json:"signature"`  // Base64 encoded
}

func (spk *SignedPreKey) ToResponse() SignedPreKeyResponse {
	return SignedPreKeyResponse{
		KeyID:     spk.KeyID,
		PublicKey: spk.PublicKeyBase64(),
		Signature: spk.SignatureBase64(),
	}
}

// PreKeyBundle is the complete bundle needed to establish a session
type PreKeyBundle struct {
	IdentityKey  IdentityKeyResponse  `json:"identity_key"`
	SignedPreKey SignedPreKeyResponse `json:"signed_prekey"`
	PreKey       *PreKeyResponse      `json:"prekey,omitempty"` // May be nil if exhausted
}

// SavePreKeys saves a batch of one-time prekeys
func SavePreKeys(db *gorm.DB, userID, deviceID string, prekeys []struct {
	KeyID     uint32
	PublicKey []byte
}) error {
	for _, pk := range prekeys {
		prekey := PreKey{
			UserID:    userID,
			DeviceID:  deviceID,
			KeyID:     pk.KeyID,
			PublicKey: pk.PublicKey,
		}
		if err := db.Create(&prekey).Error; err != nil {
			return err
		}
	}
	return nil
}

// GetAndConsumePreKey atomically retrieves and deletes a prekey
// Returns nil if no prekeys are available
func GetAndConsumePreKey(db *gorm.DB, userID, deviceID string) (*PreKey, error) {
	var prekey PreKey
	err := db.Where("user_id = ? AND device_id = ?", userID, deviceID).
		Order("created_at ASC").
		First(&prekey).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil // No prekeys available
		}
		return nil, err
	}

	// Delete the consumed prekey
	if err := db.Delete(&prekey).Error; err != nil {
		return nil, err
	}

	return &prekey, nil
}

// CountPreKeys returns the number of available prekeys for a device
func CountPreKeys(db *gorm.DB, userID, deviceID string) (int64, error) {
	var count int64
	err := db.Model(&PreKey{}).Where("user_id = ? AND device_id = ?", userID, deviceID).Count(&count).Error
	return count, err
}

// SaveSignedPreKey creates or updates a signed prekey
func SaveSignedPreKey(db *gorm.DB, userID, deviceID string, keyID uint32, publicKey, signature []byte) (*SignedPreKey, error) {
	var existing SignedPreKey
	result := db.Where("user_id = ? AND device_id = ?", userID, deviceID).First(&existing)

	if result.Error == nil {
		// Update existing
		existing.KeyID = keyID
		existing.PublicKey = publicKey
		existing.Signature = signature
		if err := db.Save(&existing).Error; err != nil {
			return nil, err
		}
		return &existing, nil
	}

	// Create new
	spk := SignedPreKey{
		UserID:    userID,
		DeviceID:  deviceID,
		KeyID:     keyID,
		PublicKey: publicKey,
		Signature: signature,
	}
	if err := db.Create(&spk).Error; err != nil {
		return nil, err
	}
	return &spk, nil
}

// GetSignedPreKey retrieves a signed prekey for a device
func GetSignedPreKey(db *gorm.DB, userID, deviceID string) (*SignedPreKey, error) {
	var spk SignedPreKey
	err := db.Where("user_id = ? AND device_id = ?", userID, deviceID).First(&spk).Error
	return &spk, err
}

// GetPreKeyBundle retrieves a complete prekey bundle for establishing a session
func GetPreKeyBundle(db *gorm.DB, userID, deviceID string) (*PreKeyBundle, error) {
	// Get identity key
	identityKey, err := GetIdentityKey(db, userID, deviceID)
	if err != nil {
		return nil, err
	}

	// Get signed prekey
	signedPreKey, err := GetSignedPreKey(db, userID, deviceID)
	if err != nil {
		return nil, err
	}

	// Get and consume a one-time prekey (may be nil)
	prekey, err := GetAndConsumePreKey(db, userID, deviceID)
	if err != nil {
		return nil, err
	}

	bundle := &PreKeyBundle{
		IdentityKey:  identityKey.ToResponse(),
		SignedPreKey: signedPreKey.ToResponse(),
	}

	if prekey != nil {
		resp := prekey.ToResponse()
		bundle.PreKey = &resp
	}

	return bundle, nil
}

// DeleteDeviceKeys removes all keys for a device (when device is removed)
func DeleteDeviceKeys(db *gorm.DB, userID, deviceID string) error {
	if err := db.Where("user_id = ? AND device_id = ?", userID, deviceID).Delete(&PreKey{}).Error; err != nil {
		return err
	}
	if err := db.Where("user_id = ? AND device_id = ?", userID, deviceID).Delete(&SignedPreKey{}).Error; err != nil {
		return err
	}
	return DeleteIdentityKey(db, userID, deviceID)
}

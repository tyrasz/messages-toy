package services

import (
	"encoding/base64"
	"errors"

	"gorm.io/gorm"
	"messenger/internal/models"
)

// KeyService handles E2EE key management operations
type KeyService struct {
	db *gorm.DB
}

func NewKeyService(db *gorm.DB) *KeyService {
	return &KeyService{db: db}
}

// KeyRegistrationInput represents a request to register encryption keys
type KeyRegistrationInput struct {
	DeviceID       string          `json:"device_id"`
	DeviceName     string          `json:"device_name,omitempty"`
	Platform       string          `json:"platform,omitempty"`
	RegistrationID uint32          `json:"registration_id"`
	IdentityKey    string          `json:"identity_key"`     // Base64
	SignedPreKey   SignedPreKeyInput `json:"signed_prekey"`
	PreKeys        []PreKeyInput   `json:"prekeys"`
}

type SignedPreKeyInput struct {
	KeyID     uint32 `json:"key_id"`
	PublicKey string `json:"public_key"` // Base64
	Signature string `json:"signature"`  // Base64
}

type PreKeyInput struct {
	KeyID     uint32 `json:"key_id"`
	PublicKey string `json:"public_key"` // Base64
}

// RegisterKeys registers a device with its encryption keys
func (s *KeyService) RegisterKeys(userID string, input KeyRegistrationInput) error {
	// Decode identity key
	identityKey, err := base64.StdEncoding.DecodeString(input.IdentityKey)
	if err != nil {
		return errors.New("invalid identity key encoding")
	}
	if len(identityKey) != 32 {
		return errors.New("identity key must be 32 bytes")
	}

	// Decode signed prekey
	signedPreKeyPublic, err := base64.StdEncoding.DecodeString(input.SignedPreKey.PublicKey)
	if err != nil {
		return errors.New("invalid signed prekey encoding")
	}
	if len(signedPreKeyPublic) != 32 {
		return errors.New("signed prekey must be 32 bytes")
	}

	signedPreKeySignature, err := base64.StdEncoding.DecodeString(input.SignedPreKey.Signature)
	if err != nil {
		return errors.New("invalid signature encoding")
	}

	// Start transaction
	return s.db.Transaction(func(tx *gorm.DB) error {
		// Register or update device
		_, err := models.RegisterDevice(tx, userID, input.DeviceID, input.DeviceName, input.Platform)
		if err != nil {
			return err
		}

		// Save identity key
		_, err = models.SaveIdentityKey(tx, userID, input.DeviceID, input.RegistrationID, identityKey)
		if err != nil {
			return err
		}

		// Save signed prekey
		_, err = models.SaveSignedPreKey(tx, userID, input.DeviceID, input.SignedPreKey.KeyID, signedPreKeyPublic, signedPreKeySignature)
		if err != nil {
			return err
		}

		// Save one-time prekeys
		for _, pk := range input.PreKeys {
			publicKey, err := base64.StdEncoding.DecodeString(pk.PublicKey)
			if err != nil {
				return errors.New("invalid prekey encoding")
			}
			if len(publicKey) != 32 {
				return errors.New("prekey must be 32 bytes")
			}

			prekey := models.PreKey{
				UserID:    userID,
				DeviceID:  input.DeviceID,
				KeyID:     pk.KeyID,
				PublicKey: publicKey,
			}
			if err := tx.Create(&prekey).Error; err != nil {
				return err
			}
		}

		return nil
	})
}

// UploadPreKeys uploads additional one-time prekeys
func (s *KeyService) UploadPreKeys(userID, deviceID string, prekeys []PreKeyInput) error {
	// Verify device exists
	_, err := models.GetDevice(s.db, userID, deviceID)
	if err != nil {
		return errors.New("device not registered")
	}

	for _, pk := range prekeys {
		publicKey, err := base64.StdEncoding.DecodeString(pk.PublicKey)
		if err != nil {
			return errors.New("invalid prekey encoding")
		}
		if len(publicKey) != 32 {
			return errors.New("prekey must be 32 bytes")
		}

		prekey := models.PreKey{
			UserID:    userID,
			DeviceID:  deviceID,
			KeyID:     pk.KeyID,
			PublicKey: publicKey,
		}
		if err := s.db.Create(&prekey).Error; err != nil {
			return err
		}
	}

	return nil
}

// GetPreKeyBundle retrieves a prekey bundle for establishing a session
func (s *KeyService) GetPreKeyBundle(userID, deviceID string) (*models.PreKeyBundle, error) {
	return models.GetPreKeyBundle(s.db, userID, deviceID)
}

// GetUserDevices retrieves all devices for a user
func (s *KeyService) GetUserDevices(userID string) ([]models.EncryptionDeviceResponse, error) {
	devices, err := models.GetUserDevices(s.db, userID)
	if err != nil {
		return nil, err
	}

	responses := make([]models.EncryptionDeviceResponse, len(devices))
	for i, d := range devices {
		responses[i] = d.ToResponse("")
	}
	return responses, nil
}

// GetUserDevicesWithCurrent retrieves all devices, marking the current one
func (s *KeyService) GetUserDevicesWithCurrent(userID, currentDeviceID string) ([]models.EncryptionDeviceResponse, error) {
	devices, err := models.GetUserDevices(s.db, userID)
	if err != nil {
		return nil, err
	}

	responses := make([]models.EncryptionDeviceResponse, len(devices))
	for i, d := range devices {
		responses[i] = d.ToResponse(currentDeviceID)
	}
	return responses, nil
}

// RemoveDevice removes a device and all its keys
func (s *KeyService) RemoveDevice(userID, deviceID string) error {
	return models.RemoveDevice(s.db, userID, deviceID)
}

// GetPreKeyCount returns the number of remaining one-time prekeys
func (s *KeyService) GetPreKeyCount(userID, deviceID string) (int64, error) {
	return models.CountPreKeys(s.db, userID, deviceID)
}

// UpdateDeviceActivity updates the last active timestamp for a device
func (s *KeyService) UpdateDeviceActivity(userID, deviceID string) error {
	return models.UpdateDeviceActivity(s.db, userID, deviceID)
}

// GetRecipientDevices retrieves all devices for a recipient (for message fanout)
func (s *KeyService) GetRecipientDevices(userID string) ([]models.IdentityKey, error) {
	return models.GetUserIdentityKeys(s.db, userID)
}

// VerifyIdentityKey checks if a user's identity key matches the expected key
// Used for safety number verification
func (s *KeyService) VerifyIdentityKey(userID, deviceID string, expectedKey string) (bool, error) {
	identityKey, err := models.GetIdentityKey(s.db, userID, deviceID)
	if err != nil {
		return false, err
	}

	expectedKeyBytes, err := base64.StdEncoding.DecodeString(expectedKey)
	if err != nil {
		return false, errors.New("invalid key encoding")
	}

	// Compare keys
	if len(identityKey.PublicKey) != len(expectedKeyBytes) {
		return false, nil
	}
	for i := range identityKey.PublicKey {
		if identityKey.PublicKey[i] != expectedKeyBytes[i] {
			return false, nil
		}
	}
	return true, nil
}

// SaveSenderKey stores a sender key for group encryption
func (s *KeyService) SaveSenderKey(groupID, userID, deviceID string, keyID uint32, chainKey, signingKey string) error {
	chainKeyBytes, err := base64.StdEncoding.DecodeString(chainKey)
	if err != nil {
		return errors.New("invalid chain key encoding")
	}

	signingKeyBytes, err := base64.StdEncoding.DecodeString(signingKey)
	if err != nil {
		return errors.New("invalid signing key encoding")
	}

	_, err = models.SaveSenderKey(s.db, groupID, userID, deviceID, keyID, chainKeyBytes, signingKeyBytes)
	return err
}

// GetSenderKey retrieves a sender key for group decryption
func (s *KeyService) GetSenderKey(groupID, userID, deviceID string) (*models.SenderKeyResponse, error) {
	key, err := models.GetSenderKey(s.db, groupID, userID, deviceID)
	if err != nil {
		return nil, err
	}
	resp := key.ToResponse()
	return &resp, nil
}

// GetGroupSenderKeys retrieves all sender keys for a group
func (s *KeyService) GetGroupSenderKeys(groupID string) ([]models.SenderKeyResponse, error) {
	keys, err := models.GetGroupSenderKeys(s.db, groupID)
	if err != nil {
		return nil, err
	}

	responses := make([]models.SenderKeyResponse, len(keys))
	for i, k := range keys {
		responses[i] = k.ToResponse()
	}
	return responses, nil
}

// DeleteGroupSenderKeys removes sender keys when leaving a group
func (s *KeyService) DeleteGroupSenderKeys(groupID, userID string) error {
	return models.DeleteGroupSenderKeys(s.db, groupID, userID)
}

// MinPreKeyCount is the threshold below which clients should upload more prekeys
const MinPreKeyCount = 10

// CheckPreKeyCount checks if the user needs to upload more prekeys
func (s *KeyService) CheckPreKeyCount(userID, deviceID string) (needsMore bool, count int64, err error) {
	count, err = models.CountPreKeys(s.db, userID, deviceID)
	if err != nil {
		return false, 0, err
	}
	return count < MinPreKeyCount, count, nil
}

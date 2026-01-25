package handlers

import (
	"github.com/gofiber/fiber/v2"
	"messenger/internal/api/middleware"
	"messenger/internal/database"
	"messenger/internal/services"
)

type KeysHandler struct {
	keyService *services.KeyService
}

func NewKeysHandler() *KeysHandler {
	return &KeysHandler{
		keyService: services.NewKeyService(database.DB),
	}
}

// RegisterKeys registers encryption keys for a device
func (h *KeysHandler) RegisterKeys(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	var input services.KeyRegistrationInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if input.DeviceID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Device ID is required",
		})
	}

	if input.IdentityKey == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Identity key is required",
		})
	}

	if input.SignedPreKey.PublicKey == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Signed prekey is required",
		})
	}

	if len(input.PreKeys) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "At least one prekey is required",
		})
	}

	if err := h.keyService.RegisterKeys(userID, input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Check prekey count
	count, _ := h.keyService.GetPreKeyCount(userID, input.DeviceID)

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"message":      "Keys registered successfully",
		"device_id":    input.DeviceID,
		"prekey_count": count,
	})
}

// UploadPreKeys uploads additional one-time prekeys
func (h *KeysHandler) UploadPreKeys(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	deviceID := c.Params("deviceId")

	if deviceID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Device ID is required",
		})
	}

	var input struct {
		PreKeys []services.PreKeyInput `json:"prekeys"`
	}
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if len(input.PreKeys) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "At least one prekey is required",
		})
	}

	if err := h.keyService.UploadPreKeys(userID, deviceID, input.PreKeys); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	count, _ := h.keyService.GetPreKeyCount(userID, deviceID)

	return c.JSON(fiber.Map{
		"message":      "Prekeys uploaded successfully",
		"prekey_count": count,
	})
}

// GetPreKeyBundle retrieves a prekey bundle for establishing a session
func (h *KeysHandler) GetPreKeyBundle(c *fiber.Ctx) error {
	targetUserID := c.Params("userId")
	deviceID := c.Query("device_id")

	if targetUserID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "User ID is required",
		})
	}

	// If no device ID specified, get all devices
	if deviceID == "" {
		devices, err := h.keyService.GetUserDevices(targetUserID)
		if err != nil || len(devices) == 0 {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "No registered devices found for user",
			})
		}

		// Get bundles for all devices
		bundles := make([]fiber.Map, 0)
		for _, device := range devices {
			bundle, err := h.keyService.GetPreKeyBundle(targetUserID, device.DeviceID)
			if err == nil {
				bundles = append(bundles, fiber.Map{
					"device_id": device.DeviceID,
					"bundle":    bundle,
				})
			}
		}

		if len(bundles) == 0 {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "No prekey bundles available",
			})
		}

		return c.JSON(fiber.Map{
			"bundles": bundles,
		})
	}

	// Get bundle for specific device
	bundle, err := h.keyService.GetPreKeyBundle(targetUserID, deviceID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Prekey bundle not found",
		})
	}

	return c.JSON(bundle)
}

// GetUserDevices retrieves all devices for a user
func (h *KeysHandler) GetUserDevices(c *fiber.Ctx) error {
	targetUserID := c.Params("userId")

	if targetUserID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "User ID is required",
		})
	}

	devices, err := h.keyService.GetUserDevices(targetUserID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch devices",
		})
	}

	return c.JSON(fiber.Map{
		"devices": devices,
	})
}

// GetMyDevices retrieves all devices for the current user
func (h *KeysHandler) GetMyDevices(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	currentDeviceID := c.Get("X-Device-ID")

	devices, err := h.keyService.GetUserDevicesWithCurrent(userID, currentDeviceID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch devices",
		})
	}

	return c.JSON(fiber.Map{
		"devices": devices,
	})
}

// RemoveDevice removes a device and all its keys
func (h *KeysHandler) RemoveDevice(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	deviceID := c.Params("deviceId")

	if deviceID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Device ID is required",
		})
	}

	if err := h.keyService.RemoveDevice(userID, deviceID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to remove device",
		})
	}

	return c.JSON(fiber.Map{
		"message": "Device removed successfully",
	})
}

// GetPreKeyCount returns the number of remaining one-time prekeys
func (h *KeysHandler) GetPreKeyCount(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	deviceID := c.Params("deviceId")

	if deviceID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Device ID is required",
		})
	}

	needsMore, count, err := h.keyService.CheckPreKeyCount(userID, deviceID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get prekey count",
		})
	}

	return c.JSON(fiber.Map{
		"count":         count,
		"needs_upload":  needsMore,
		"min_threshold": services.MinPreKeyCount,
	})
}

// VerifyIdentityKey verifies a user's identity key for safety number verification
func (h *KeysHandler) VerifyIdentityKey(c *fiber.Ctx) error {
	targetUserID := c.Params("userId")
	deviceID := c.Query("device_id")

	var input struct {
		IdentityKey string `json:"identity_key"` // Base64
	}
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if input.IdentityKey == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Identity key is required",
		})
	}

	matches, err := h.keyService.VerifyIdentityKey(targetUserID, deviceID, input.IdentityKey)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Identity key not found",
		})
	}

	return c.JSON(fiber.Map{
		"verified": matches,
	})
}

// StoreSenderKey stores a sender key for group encryption
func (h *KeysHandler) StoreSenderKey(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	groupID := c.Params("groupId")
	deviceID := c.Get("X-Device-ID")

	if groupID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Group ID is required",
		})
	}

	if deviceID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Device ID header is required",
		})
	}

	var input struct {
		KeyID      uint32 `json:"key_id"`
		ChainKey   string `json:"chain_key"`   // Base64
		SigningKey string `json:"signing_key"` // Base64
	}
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if err := h.keyService.SaveSenderKey(groupID, userID, deviceID, input.KeyID, input.ChainKey, input.SigningKey); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"message": "Sender key stored successfully",
	})
}

// GetSenderKey retrieves a sender key for group decryption
func (h *KeysHandler) GetSenderKey(c *fiber.Ctx) error {
	groupID := c.Params("groupId")
	senderID := c.Params("userId")
	deviceID := c.Query("device_id")

	if groupID == "" || senderID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Group ID and user ID are required",
		})
	}

	if deviceID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Device ID is required",
		})
	}

	key, err := h.keyService.GetSenderKey(groupID, senderID, deviceID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Sender key not found",
		})
	}

	return c.JSON(key)
}

// GetGroupSenderKeys retrieves all sender keys for a group
func (h *KeysHandler) GetGroupSenderKeys(c *fiber.Ctx) error {
	groupID := c.Params("groupId")

	if groupID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Group ID is required",
		})
	}

	keys, err := h.keyService.GetGroupSenderKeys(groupID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch sender keys",
		})
	}

	return c.JSON(fiber.Map{
		"sender_keys": keys,
	})
}

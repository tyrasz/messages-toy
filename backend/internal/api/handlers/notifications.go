package handlers

import (
	"github.com/gofiber/fiber/v2"
	"messenger/internal/api/middleware"
	"messenger/internal/database"
	"messenger/internal/models"
	"messenger/internal/services"
)

type NotificationsHandler struct{}

func NewNotificationsHandler() *NotificationsHandler {
	return &NotificationsHandler{}
}

type RegisterTokenRequest struct {
	Token      string `json:"token" validate:"required"`
	Platform   string `json:"platform" validate:"required,oneof=ios android web"`
	DeviceID   string `json:"device_id"`
	AppVersion string `json:"app_version"`
}

// RegisterToken registers a device token for push notifications
func (h *NotificationsHandler) RegisterToken(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	var req RegisterTokenRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if req.Token == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Token is required",
		})
	}

	platform := models.DevicePlatform(req.Platform)
	if platform != models.PlatformIOS &&
		platform != models.PlatformAndroid &&
		platform != models.PlatformWeb {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Platform must be ios, android, or web",
		})
	}

	token, err := models.RegisterToken(
		database.DB,
		userID,
		req.Token,
		platform,
		req.DeviceID,
		req.AppVersion,
	)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to register token",
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"id":       token.ID,
		"platform": token.Platform,
		"message":  "Token registered successfully",
	})
}

type UnregisterTokenRequest struct {
	Token string `json:"token" validate:"required"`
}

// UnregisterToken removes a device token
func (h *NotificationsHandler) UnregisterToken(c *fiber.Ctx) error {
	var req UnregisterTokenRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if req.Token == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Token is required",
		})
	}

	if err := models.UnregisterToken(database.DB, req.Token); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to unregister token",
		})
	}

	return c.JSON(fiber.Map{
		"message": "Token unregistered successfully",
	})
}

// UnregisterAllTokens removes all device tokens for the current user (logout)
func (h *NotificationsHandler) UnregisterAllTokens(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	if err := models.UnregisterUserTokens(database.DB, userID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to unregister tokens",
		})
	}

	return c.JSON(fiber.Map{
		"message": "All tokens unregistered successfully",
	})
}

// GetTokens returns all registered tokens for the current user
func (h *NotificationsHandler) GetTokens(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	tokens, err := models.GetUserTokens(database.DB, userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get tokens",
		})
	}

	return c.JSON(tokens)
}

// TestNotification sends a test push notification to verify setup
func (h *NotificationsHandler) TestNotification(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	pushService := services.GetPushService()
	if !pushService.IsEnabled() {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"error": "Push notifications are not configured",
		})
	}

	tokens, err := models.GetUserTokens(database.DB, userID)
	if err != nil || len(tokens) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "No registered devices found",
		})
	}

	// Send test to first token
	if err := pushService.SendTestNotification(tokens[0].Token); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to send test notification",
		})
	}

	return c.JSON(fiber.Map{
		"message": "Test notification sent",
	})
}

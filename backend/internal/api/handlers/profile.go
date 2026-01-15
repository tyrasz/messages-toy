package handlers

import (
	"github.com/gofiber/fiber/v2"
	"messenger/internal/api/middleware"
	"messenger/internal/database"
	"messenger/internal/models"
	"messenger/internal/websocket"
)

type ProfileHandler struct {
	hub *websocket.Hub
}

func NewProfileHandler(hub *websocket.Hub) *ProfileHandler {
	return &ProfileHandler{hub: hub}
}

// GetProfile returns the current user's profile
func (h *ProfileHandler) GetProfile(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	var user models.User
	if err := database.DB.First(&user, "id = ?", userID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "User not found",
		})
	}

	return c.JSON(user.ToResponse(true))
}

// GetUserProfile returns another user's profile
func (h *ProfileHandler) GetUserProfile(c *fiber.Ctx) error {
	targetUserID := c.Params("userId")

	var user models.User
	if err := database.DB.First(&user, "id = ?", targetUserID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "User not found",
		})
	}

	online := h.hub.IsOnline(targetUserID)
	return c.JSON(user.ToResponse(online))
}

type UpdateProfileRequest struct {
	DisplayName *string `json:"display_name,omitempty"`
	About       *string `json:"about,omitempty"`
	StatusEmoji *string `json:"status_emoji,omitempty"`
}

// UpdateProfile updates the current user's profile
func (h *ProfileHandler) UpdateProfile(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	var req UpdateProfileRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	updates := make(map[string]interface{})
	if req.DisplayName != nil {
		updates["display_name"] = *req.DisplayName
	}
	if req.About != nil {
		// Limit about to 500 characters
		about := *req.About
		if len(about) > 500 {
			about = about[:500]
		}
		updates["about"] = about
	}
	if req.StatusEmoji != nil {
		updates["status_emoji"] = *req.StatusEmoji
	}

	if len(updates) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "No updates provided",
		})
	}

	if err := database.DB.Model(&models.User{}).Where("id = ?", userID).Updates(updates).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to update profile",
		})
	}

	// Get updated user
	var user models.User
	database.DB.First(&user, "id = ?", userID)

	// Broadcast profile update to contacts
	h.broadcastProfileUpdate(&user)

	return c.JSON(user.ToResponse(true))
}

func (h *ProfileHandler) broadcastProfileUpdate(user *models.User) {
	// Get user's contacts
	var contacts []models.Contact
	database.DB.Where("contact_id = ?", user.ID).Find(&contacts)

	update := map[string]interface{}{
		"type": "profile_update",
		"user": user.ToResponse(true),
	}

	for _, contact := range contacts {
		h.hub.SendJSONToUser(contact.UserID, update)
	}
}

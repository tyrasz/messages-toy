package handlers

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"messenger/internal/api/middleware"
	"messenger/internal/database"
	"messenger/internal/models"
)

type SettingsHandler struct{}

func NewSettingsHandler() *SettingsHandler {
	return &SettingsHandler{}
}

type SetDisappearingRequest struct {
	OtherUserID *string `json:"other_user_id,omitempty"`
	GroupID     *string `json:"group_id,omitempty"`
	Seconds     int     `json:"seconds"` // 0 to disable
}

// GetConversationSettings returns settings for a conversation
func (h *SettingsHandler) GetConversationSettings(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	otherUserID := c.Query("other_user_id")
	groupID := c.Query("group_id")

	if otherUserID == "" && groupID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Either other_user_id or group_id is required",
		})
	}

	var settings *models.ConversationSettings
	var err error

	if otherUserID != "" {
		settings, err = models.GetOrCreateDMSettings(database.DB, userID, otherUserID)
	} else {
		settings, err = models.GetOrCreateGroupSettings(database.DB, userID, groupID)
	}

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get settings",
		})
	}

	return c.JSON(fiber.Map{
		"settings": settings,
	})
}

// SetDisappearingMessages sets the disappearing messages timer
func (h *SettingsHandler) SetDisappearingMessages(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	var req SetDisappearingRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if req.OtherUserID == nil && req.GroupID == nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Either other_user_id or group_id is required",
		})
	}

	// Validate seconds (allowed values: 0, 86400, 604800, 7776000)
	validDurations := map[int]bool{
		0:       true, // Off
		86400:   true, // 24 hours
		604800:  true, // 7 days
		7776000: true, // 90 days
	}

	if !validDurations[req.Seconds] {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid duration. Allowed: 0 (off), 86400 (24h), 604800 (7d), 7776000 (90d)",
		})
	}

	if err := models.SetDisappearingTimer(database.DB, userID, req.OtherUserID, req.GroupID, req.Seconds); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to update settings",
		})
	}

	// Format human-readable duration
	var durationText string
	switch req.Seconds {
	case 0:
		durationText = "off"
	case 86400:
		durationText = "24 hours"
	case 604800:
		durationText = "7 days"
	case 7776000:
		durationText = "90 days"
	}

	return c.JSON(fiber.Map{
		"success":              true,
		"disappearing_seconds": req.Seconds,
		"disappearing_text":    durationText,
	})
}

type MuteRequest struct {
	OtherUserID *string `json:"other_user_id,omitempty"`
	GroupID     *string `json:"group_id,omitempty"`
	Hours       int     `json:"hours"` // 0 to unmute
}

// MuteConversation mutes notifications for a conversation
func (h *SettingsHandler) MuteConversation(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	var req MuteRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if req.OtherUserID == nil && req.GroupID == nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Either other_user_id or group_id is required",
		})
	}

	var settings *models.ConversationSettings
	var err error

	if req.OtherUserID != nil {
		settings, err = models.GetOrCreateDMSettings(database.DB, userID, *req.OtherUserID)
	} else {
		settings, err = models.GetOrCreateGroupSettings(database.DB, userID, *req.GroupID)
	}

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get settings",
		})
	}

	var mutedUntil *time.Time
	if req.Hours > 0 {
		t := time.Now().Add(time.Duration(req.Hours) * time.Hour)
		mutedUntil = &t
	}

	if err := database.DB.Model(settings).Update("muted_until", mutedUntil).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to update mute settings",
		})
	}

	return c.JSON(fiber.Map{
		"success":     true,
		"muted":       mutedUntil != nil,
		"muted_until": mutedUntil,
	})
}

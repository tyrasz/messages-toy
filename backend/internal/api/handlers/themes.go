package handlers

import (
	"github.com/gofiber/fiber/v2"
	"messenger/internal/api/middleware"
	"messenger/internal/database"
	"messenger/internal/models"
)

type ThemesHandler struct{}

func NewThemesHandler() *ThemesHandler {
	return &ThemesHandler{}
}

// GetTheme gets a user's theme (global or for a specific conversation)
func (h *ThemesHandler) GetTheme(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	conversationID := c.Query("conversation_id")
	conversationType := c.Query("conversation_type")

	var theme *models.ChatTheme
	var err error

	if conversationID != "" && conversationType != "" {
		theme, err = models.GetEffectiveTheme(database.DB, userID, conversationID, conversationType)
	} else {
		theme, err = models.GetUserTheme(database.DB, userID)
	}

	if err != nil {
		// Return default theme if none set
		defaultTheme := models.PresetThemes["default"]
		return c.JSON(fiber.Map{
			"theme":     defaultTheme,
			"is_custom": false,
		})
	}

	return c.JSON(fiber.Map{
		"theme":     theme,
		"is_custom": true,
	})
}

type SetThemeRequest struct {
	ConversationID   *string `json:"conversation_id,omitempty"`
	ConversationType *string `json:"conversation_type,omitempty"`
	Preset           string  `json:"preset,omitempty"` // Use a preset theme
	// Custom colors
	PrimaryColor       string `json:"primary_color,omitempty"`
	SecondaryColor     string `json:"secondary_color,omitempty"`
	BackgroundColor    string `json:"background_color,omitempty"`
	MessageBubbleColor string `json:"message_bubble_color,omitempty"`
	MessageTextColor   string `json:"message_text_color,omitempty"`
	BackgroundImage    string `json:"background_image,omitempty"`
	FontSize           string `json:"font_size,omitempty"`
	BubbleStyle        string `json:"bubble_style,omitempty"`
	DarkMode           *bool  `json:"dark_mode,omitempty"`
}

// SetTheme sets a user's theme
func (h *ThemesHandler) SetTheme(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	var req SetThemeRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	settings := make(map[string]interface{})

	// Apply preset if specified
	if req.Preset != "" {
		preset, exists := models.PresetThemes[req.Preset]
		if !exists {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error":            "Invalid preset theme",
				"available_presets": getPresetNames(),
			})
		}
		settings["primary_color"] = preset.PrimaryColor
		settings["secondary_color"] = preset.SecondaryColor
		settings["background_color"] = preset.BackgroundColor
		settings["message_bubble_color"] = preset.MessageBubbleColor
		settings["message_text_color"] = preset.MessageTextColor
		settings["font_size"] = preset.FontSize
		settings["bubble_style"] = preset.BubbleStyle
	}

	// Override with custom values
	if req.PrimaryColor != "" {
		settings["primary_color"] = req.PrimaryColor
	}
	if req.SecondaryColor != "" {
		settings["secondary_color"] = req.SecondaryColor
	}
	if req.BackgroundColor != "" {
		settings["background_color"] = req.BackgroundColor
	}
	if req.MessageBubbleColor != "" {
		settings["message_bubble_color"] = req.MessageBubbleColor
	}
	if req.MessageTextColor != "" {
		settings["message_text_color"] = req.MessageTextColor
	}
	if req.BackgroundImage != "" {
		settings["background_image"] = req.BackgroundImage
	}
	if req.FontSize != "" {
		settings["font_size"] = req.FontSize
	}
	if req.BubbleStyle != "" {
		settings["bubble_style"] = req.BubbleStyle
	}
	if req.DarkMode != nil {
		settings["dark_mode"] = *req.DarkMode
	}

	theme, err := models.SetUserTheme(database.DB, userID, req.ConversationID, req.ConversationType, settings)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to save theme",
		})
	}

	return c.JSON(fiber.Map{
		"theme":   theme,
		"message": "Theme updated successfully",
	})
}

// DeleteTheme removes a conversation-specific theme
func (h *ThemesHandler) DeleteTheme(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	conversationID := c.Query("conversation_id")
	conversationType := c.Query("conversation_type")

	if conversationID == "" || conversationType == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "conversation_id and conversation_type are required",
		})
	}

	if err := models.DeleteConversationTheme(database.DB, userID, conversationID, conversationType); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to delete theme",
		})
	}

	return c.JSON(fiber.Map{
		"message": "Theme deleted, will use global theme",
	})
}

// GetPresets returns available preset themes
func (h *ThemesHandler) GetPresets(c *fiber.Ctx) error {
	presets := make([]fiber.Map, 0, len(models.PresetThemes))
	for name, theme := range models.PresetThemes {
		presets = append(presets, fiber.Map{
			"name":                 name,
			"primary_color":        theme.PrimaryColor,
			"background_color":     theme.BackgroundColor,
			"message_bubble_color": theme.MessageBubbleColor,
			"message_text_color":   theme.MessageTextColor,
		})
	}
	return c.JSON(fiber.Map{
		"presets": presets,
	})
}

func getPresetNames() []string {
	names := make([]string, 0, len(models.PresetThemes))
	for name := range models.PresetThemes {
		names = append(names, name)
	}
	return names
}

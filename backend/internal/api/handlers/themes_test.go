package handlers

import (
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v2"
	"messenger/internal/api/middleware"
	"messenger/internal/database"
	"messenger/internal/models"
)

func setupThemesTestApp() *fiber.App {
	app := fiber.New()
	handler := NewThemesHandler()

	protected := app.Group("", middleware.AuthRequired())
	themes := protected.Group("/themes")
	themes.Get("/", handler.GetTheme)
	themes.Post("/", handler.SetTheme)
	themes.Delete("/", handler.DeleteTheme)
	themes.Get("/presets", handler.GetPresets)

	return app
}

func TestGetTheme_Default(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	_, token := createTestUser(t, "testuser", "password123")
	app := setupThemesTestApp()

	resp, body := makeRequest(app, testRequest{
		Method: "GET",
		Path:   "/themes",
		Token:  token,
	})

	assertStatus(t, resp, http.StatusOK)

	data := parseResponse(body)
	assertJSONField(t, data, "is_custom", false)
	assertJSONFieldExists(t, data, "theme")
}

func TestSetTheme_Preset(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	_, token := createTestUser(t, "testuser", "password123")
	app := setupThemesTestApp()

	resp, body := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/themes",
		Body: map[string]interface{}{
			"preset": "ocean",
		},
		Token: token,
	})

	assertStatus(t, resp, http.StatusOK)

	data := parseResponse(body)
	assertJSONFieldExists(t, data, "theme")
	assertJSONFieldExists(t, data, "message")
}

func TestSetTheme_Custom(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	_, token := createTestUser(t, "testuser", "password123")
	app := setupThemesTestApp()

	resp, body := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/themes",
		Body: map[string]interface{}{
			"primary_color":        "#ff0000",
			"background_color":     "#000000",
			"message_bubble_color": "#333333",
		},
		Token: token,
	})

	assertStatus(t, resp, http.StatusOK)

	data := parseResponse(body)
	assertJSONFieldExists(t, data, "theme")
}

func TestSetTheme_InvalidPreset(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	_, token := createTestUser(t, "testuser", "password123")
	app := setupThemesTestApp()

	resp, body := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/themes",
		Body: map[string]interface{}{
			"preset": "nonexistent",
		},
		Token: token,
	})

	assertStatus(t, resp, http.StatusBadRequest)

	data := parseResponse(body)
	assertJSONFieldExists(t, data, "error")
	assertJSONFieldExists(t, data, "available_presets")
}

func TestSetTheme_ConversationSpecific(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user, token := createTestUser(t, "testuser", "password123")
	user2, _ := createTestUser(t, "testuser2", "password123")
	app := setupThemesTestApp()

	// Set conversation-specific theme
	resp, body := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/themes",
		Body: map[string]interface{}{
			"conversation_id":   user2.ID,
			"conversation_type": "direct",
			"preset":            "sunset",
		},
		Token: token,
	})

	assertStatus(t, resp, http.StatusOK)

	// Verify the theme was saved for the conversation
	var theme models.ChatTheme
	database.DB.Where("user_id = ? AND conversation_id = ?", user.ID, user2.ID).First(&theme)
	if theme.ID == "" {
		t.Error("Expected conversation-specific theme to be saved")
	}

	data := parseResponse(body)
	assertJSONFieldExists(t, data, "theme")
}

func TestGetTheme_ConversationSpecific(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	_, token := createTestUser(t, "testuser", "password123")
	user2, _ := createTestUser(t, "testuser2", "password123")
	app := setupThemesTestApp()

	// First set a conversation-specific theme
	makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/themes",
		Body: map[string]interface{}{
			"conversation_id":   user2.ID,
			"conversation_type": "direct",
			"preset":            "forest",
		},
		Token: token,
	})

	// Get the conversation-specific theme
	resp, body := makeRequest(app, testRequest{
		Method: "GET",
		Path:   "/themes?conversation_id=" + user2.ID + "&conversation_type=direct",
		Token:  token,
	})

	assertStatus(t, resp, http.StatusOK)

	data := parseResponse(body)
	assertJSONField(t, data, "is_custom", true)
}

func TestDeleteTheme(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user, token := createTestUser(t, "testuser", "password123")
	user2, _ := createTestUser(t, "testuser2", "password123")
	app := setupThemesTestApp()

	// Set a conversation theme first
	makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/themes",
		Body: map[string]interface{}{
			"conversation_id":   user2.ID,
			"conversation_type": "direct",
			"preset":            "lavender",
		},
		Token: token,
	})

	// Delete it
	resp, body := makeRequest(app, testRequest{
		Method: "DELETE",
		Path:   "/themes?conversation_id=" + user2.ID + "&conversation_type=direct",
		Token:  token,
	})

	assertStatus(t, resp, http.StatusOK)

	// Verify it was deleted
	var theme models.ChatTheme
	result := database.DB.Where("user_id = ? AND conversation_id = ?", user.ID, user2.ID).First(&theme)
	if result.Error == nil {
		t.Error("Expected theme to be deleted")
	}

	data := parseResponse(body)
	assertJSONFieldExists(t, data, "message")
}

func TestDeleteTheme_MissingParams(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	_, token := createTestUser(t, "testuser", "password123")
	app := setupThemesTestApp()

	resp, body := makeRequest(app, testRequest{
		Method: "DELETE",
		Path:   "/themes",
		Token:  token,
	})

	assertStatus(t, resp, http.StatusBadRequest)

	data := parseResponse(body)
	assertJSONFieldExists(t, data, "error")
}

func TestGetPresets(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	_, token := createTestUser(t, "testuser", "password123")
	app := setupThemesTestApp()

	resp, body := makeRequest(app, testRequest{
		Method: "GET",
		Path:   "/themes/presets",
		Token:  token,
	})

	assertStatus(t, resp, http.StatusOK)

	data := parseResponse(body)
	presets, ok := data["presets"].([]interface{})
	if !ok {
		t.Fatal("Expected presets to be an array")
	}

	if len(presets) < 3 {
		t.Errorf("Expected at least 3 presets, got %d", len(presets))
	}
}

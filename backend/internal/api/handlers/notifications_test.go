package handlers

import (
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v2"
	"messenger/internal/api/middleware"
	"messenger/internal/database"
	"messenger/internal/models"
)

func setupNotificationsTestApp() *fiber.App {
	app := fiber.New()
	handler := NewNotificationsHandler()

	protected := app.Group("", middleware.AuthRequired())
	notifications := protected.Group("/notifications")
	notifications.Post("/register", handler.RegisterToken)
	notifications.Post("/unregister", handler.UnregisterToken)
	notifications.Delete("/all", handler.UnregisterAllTokens)
	notifications.Get("/tokens", handler.GetTokens)
	notifications.Post("/test", handler.TestNotification)

	return app
}

func TestRegisterToken_iOS(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user, token := createTestUser(t, "testuser", "password123")
	app := setupNotificationsTestApp()

	resp, body := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/notifications/register",
		Body: map[string]interface{}{
			"token":       "ios-device-token-12345",
			"platform":    "ios",
			"device_id":   "iphone-123",
			"app_version": "1.0.0",
		},
		Token: token,
	})

	assertStatus(t, resp, http.StatusCreated)

	data := parseResponse(body)
	assertJSONFieldExists(t, data, "id")
	assertJSONField(t, data, "platform", "ios")

	// Verify token was saved
	var deviceToken models.DeviceToken
	result := database.DB.Where("user_id = ?", user.ID).First(&deviceToken)
	if result.Error != nil {
		t.Error("Expected token to be saved in database")
	}
	if deviceToken.Token != "ios-device-token-12345" {
		t.Errorf("Expected token 'ios-device-token-12345', got '%s'", deviceToken.Token)
	}
}

func TestRegisterToken_Android(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	_, token := createTestUser(t, "testuser", "password123")
	app := setupNotificationsTestApp()

	resp, body := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/notifications/register",
		Body: map[string]interface{}{
			"token":    "android-fcm-token-67890",
			"platform": "android",
		},
		Token: token,
	})

	assertStatus(t, resp, http.StatusCreated)

	data := parseResponse(body)
	assertJSONField(t, data, "platform", "android")
}

func TestRegisterToken_Web(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	_, token := createTestUser(t, "testuser", "password123")
	app := setupNotificationsTestApp()

	resp, body := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/notifications/register",
		Body: map[string]interface{}{
			"token":    "web-push-token-abcde",
			"platform": "web",
		},
		Token: token,
	})

	assertStatus(t, resp, http.StatusCreated)

	data := parseResponse(body)
	assertJSONField(t, data, "platform", "web")
}

func TestRegisterToken_MissingToken(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	_, token := createTestUser(t, "testuser", "password123")
	app := setupNotificationsTestApp()

	resp, body := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/notifications/register",
		Body: map[string]interface{}{
			"platform": "ios",
		},
		Token: token,
	})

	assertStatus(t, resp, http.StatusBadRequest)

	data := parseResponse(body)
	assertJSONFieldExists(t, data, "error")
}

func TestRegisterToken_InvalidPlatform(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	_, token := createTestUser(t, "testuser", "password123")
	app := setupNotificationsTestApp()

	resp, body := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/notifications/register",
		Body: map[string]interface{}{
			"token":    "some-token",
			"platform": "windows",
		},
		Token: token,
	})

	assertStatus(t, resp, http.StatusBadRequest)

	data := parseResponse(body)
	assertJSONFieldExists(t, data, "error")
}

func TestRegisterToken_UpdateExisting(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user, token := createTestUser(t, "testuser", "password123")
	app := setupNotificationsTestApp()

	// Register first token
	makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/notifications/register",
		Body: map[string]interface{}{
			"token":    "token-12345",
			"platform": "ios",
		},
		Token: token,
	})

	// Register same token again (should update, not duplicate)
	resp, _ := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/notifications/register",
		Body: map[string]interface{}{
			"token":       "token-12345",
			"platform":    "ios",
			"app_version": "2.0.0",
		},
		Token: token,
	})

	assertStatus(t, resp, http.StatusCreated)

	// Verify only one token exists
	var count int64
	database.DB.Model(&models.DeviceToken{}).Where("user_id = ?", user.ID).Count(&count)
	if count != 1 {
		t.Errorf("Expected 1 token, got %d", count)
	}
}

func TestUnregisterToken(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user, token := createTestUser(t, "testuser", "password123")
	app := setupNotificationsTestApp()

	// First register a token
	database.DB.Create(&models.DeviceToken{
		UserID:   user.ID,
		Token:    "token-to-delete",
		Platform: models.PlatformIOS,
	})

	resp, body := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/notifications/unregister",
		Body: map[string]interface{}{
			"token": "token-to-delete",
		},
		Token: token,
	})

	assertStatus(t, resp, http.StatusOK)

	data := parseResponse(body)
	assertJSONFieldExists(t, data, "message")

	// Verify token was deleted
	var deviceToken models.DeviceToken
	result := database.DB.Where("token = ?", "token-to-delete").First(&deviceToken)
	if result.Error == nil {
		t.Error("Expected token to be deleted")
	}
}

func TestUnregisterToken_MissingToken(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	_, token := createTestUser(t, "testuser", "password123")
	app := setupNotificationsTestApp()

	resp, body := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/notifications/unregister",
		Body:   map[string]interface{}{},
		Token:  token,
	})

	assertStatus(t, resp, http.StatusBadRequest)

	data := parseResponse(body)
	assertJSONFieldExists(t, data, "error")
}

func TestUnregisterAllTokens(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user, token := createTestUser(t, "testuser", "password123")
	app := setupNotificationsTestApp()

	// Register multiple tokens
	for i := 0; i < 3; i++ {
		database.DB.Create(&models.DeviceToken{
			UserID:   user.ID,
			Token:    "token-" + string(rune('A'+i)),
			Platform: models.PlatformIOS,
		})
	}

	resp, body := makeRequest(app, testRequest{
		Method: "DELETE",
		Path:   "/notifications/all",
		Token:  token,
	})

	assertStatus(t, resp, http.StatusOK)

	data := parseResponse(body)
	assertJSONFieldExists(t, data, "message")

	// Verify all tokens were deleted
	var count int64
	database.DB.Model(&models.DeviceToken{}).Where("user_id = ?", user.ID).Count(&count)
	if count != 0 {
		t.Errorf("Expected 0 tokens, got %d", count)
	}
}

func TestGetTokens(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user, token := createTestUser(t, "testuser", "password123")
	app := setupNotificationsTestApp()

	// Register multiple tokens
	database.DB.Create(&models.DeviceToken{
		UserID:   user.ID,
		Token:    "ios-token",
		Platform: models.PlatformIOS,
	})
	database.DB.Create(&models.DeviceToken{
		UserID:   user.ID,
		Token:    "android-token",
		Platform: models.PlatformAndroid,
	})

	resp, body := makeRequest(app, testRequest{
		Method: "GET",
		Path:   "/notifications/tokens",
		Token:  token,
	})

	assertStatus(t, resp, http.StatusOK)

	var tokens []models.DeviceToken
	if err := parseResponseArray(body, &tokens); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if len(tokens) != 2 {
		t.Errorf("Expected 2 tokens, got %d", len(tokens))
	}
}

func TestGetTokens_Empty(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	_, token := createTestUser(t, "testuser", "password123")
	app := setupNotificationsTestApp()

	resp, body := makeRequest(app, testRequest{
		Method: "GET",
		Path:   "/notifications/tokens",
		Token:  token,
	})

	assertStatus(t, resp, http.StatusOK)

	var tokens []models.DeviceToken
	if err := parseResponseArray(body, &tokens); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if len(tokens) != 0 {
		t.Errorf("Expected 0 tokens, got %d", len(tokens))
	}
}

func TestTestNotification_PushNotConfigured(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	_, token := createTestUser(t, "testuser", "password123")
	app := setupNotificationsTestApp()

	resp, body := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/notifications/test",
		Token:  token,
	})

	// Should fail because push service is not configured in test env
	assertStatus(t, resp, http.StatusServiceUnavailable)

	data := parseResponse(body)
	assertJSONFieldExists(t, data, "error")
}

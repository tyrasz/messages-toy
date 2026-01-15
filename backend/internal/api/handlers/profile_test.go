package handlers

import (
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v2"
	"messenger/internal/api/middleware"
	"messenger/internal/database"
	"messenger/internal/models"
	"messenger/internal/websocket"
)

func TestProfileHandler_GetProfile(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	hub := websocket.NewHub()
	app := fiber.New()
	profileHandler := NewProfileHandler(hub)

	app.Use(middleware.AuthRequired())
	app.Get("/profile", profileHandler.GetProfile)

	// Create a test user
	user, token := createTestUser(t, "profileuser", "password123")

	tests := []struct {
		name           string
		token          string
		expectedStatus int
		checkResponse  func(t *testing.T, data map[string]interface{})
	}{
		{
			name:           "get own profile",
			token:          token,
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, data map[string]interface{}) {
				assertJSONField(t, data, "id", user.ID)
				assertJSONField(t, data, "username", "profileuser")
			},
		},
		{
			name:           "unauthorized - no token",
			token:          "",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "unauthorized - invalid token",
			token:          "invalid-token",
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, body := makeRequest(app, testRequest{
				Method: "GET",
				Path:   "/profile",
				Token:  tt.token,
			})

			assertStatus(t, resp, tt.expectedStatus)

			if tt.checkResponse != nil {
				data := parseResponse(body)
				tt.checkResponse(t, data)
			}
		})
	}
}

func TestProfileHandler_GetUserProfile(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	hub := websocket.NewHub()
	app := fiber.New()
	profileHandler := NewProfileHandler(hub)

	app.Use(middleware.AuthRequired())
	app.Get("/profile/:userId", profileHandler.GetUserProfile)

	// Create test users
	user1, token := createTestUser(t, "user1", "password123")
	user2, _ := createTestUser(t, "user2", "password123")

	// Update user2's profile
	database.DB.Model(&models.User{}).Where("id = ?", user2.ID).Updates(map[string]interface{}{
		"display_name": "User Two",
		"about":        "Test bio",
	})

	tests := []struct {
		name           string
		userID         string
		token          string
		expectedStatus int
		checkResponse  func(t *testing.T, data map[string]interface{})
	}{
		{
			name:           "get another user's profile",
			userID:         user2.ID,
			token:          token,
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, data map[string]interface{}) {
				assertJSONField(t, data, "id", user2.ID)
				assertJSONField(t, data, "username", "user2")
				assertJSONField(t, data, "display_name", "User Two")
				assertJSONField(t, data, "about", "Test bio")
			},
		},
		{
			name:           "get own profile via userId",
			userID:         user1.ID,
			token:          token,
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, data map[string]interface{}) {
				assertJSONField(t, data, "id", user1.ID)
			},
		},
		{
			name:           "non-existent user",
			userID:         "non-existent-id",
			token:          token,
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, body := makeRequest(app, testRequest{
				Method: "GET",
				Path:   "/profile/" + tt.userID,
				Token:  tt.token,
			})

			assertStatus(t, resp, tt.expectedStatus)

			if tt.checkResponse != nil {
				data := parseResponse(body)
				tt.checkResponse(t, data)
			}
		})
	}
}

func TestProfileHandler_UpdateProfile(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	hub := websocket.NewHub()
	app := fiber.New()
	profileHandler := NewProfileHandler(hub)

	app.Use(middleware.AuthRequired())
	app.Put("/profile", profileHandler.UpdateProfile)

	// Create a test user
	_, token := createTestUser(t, "updateuser", "password123")

	tests := []struct {
		name           string
		body           map[string]interface{}
		token          string
		expectedStatus int
		checkResponse  func(t *testing.T, data map[string]interface{})
	}{
		{
			name: "update display name",
			body: map[string]interface{}{
				"display_name": "New Display Name",
			},
			token:          token,
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, data map[string]interface{}) {
				assertJSONField(t, data, "display_name", "New Display Name")
			},
		},
		{
			name: "update about",
			body: map[string]interface{}{
				"about": "This is my new bio",
			},
			token:          token,
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, data map[string]interface{}) {
				assertJSONField(t, data, "about", "This is my new bio")
			},
		},
		{
			name: "update status emoji",
			body: map[string]interface{}{
				"status_emoji": "😀",
			},
			token:          token,
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, data map[string]interface{}) {
				assertJSONField(t, data, "status_emoji", "😀")
			},
		},
		{
			name: "update multiple fields",
			body: map[string]interface{}{
				"display_name": "Multi Update",
				"about":        "Updated bio",
				"status_emoji": "🎉",
			},
			token:          token,
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, data map[string]interface{}) {
				assertJSONField(t, data, "display_name", "Multi Update")
				assertJSONField(t, data, "about", "Updated bio")
				assertJSONField(t, data, "status_emoji", "🎉")
			},
		},
		{
			name:           "no updates provided",
			body:           map[string]interface{}{},
			token:          token,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "unauthorized",
			body:           map[string]interface{}{"display_name": "Test"},
			token:          "",
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, body := makeRequest(app, testRequest{
				Method: "PUT",
				Path:   "/profile",
				Body:   tt.body,
				Token:  tt.token,
			})

			assertStatus(t, resp, tt.expectedStatus)

			if tt.checkResponse != nil {
				data := parseResponse(body)
				tt.checkResponse(t, data)
			}
		})
	}
}

func TestProfileHandler_UpdateProfile_LongAbout(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	hub := websocket.NewHub()
	app := fiber.New()
	profileHandler := NewProfileHandler(hub)

	app.Use(middleware.AuthRequired())
	app.Put("/profile", profileHandler.UpdateProfile)

	_, token := createTestUser(t, "longbio", "password123")

	// Create a string longer than 500 characters
	longAbout := ""
	for i := 0; i < 600; i++ {
		longAbout += "a"
	}

	resp, body := makeRequest(app, testRequest{
		Method: "PUT",
		Path:   "/profile",
		Body: map[string]interface{}{
			"about": longAbout,
		},
		Token: token,
	})

	assertStatus(t, resp, http.StatusOK)

	data := parseResponse(body)
	about := data["about"].(string)
	if len(about) > 500 {
		t.Errorf("About should be truncated to 500 characters, got %d", len(about))
	}
}

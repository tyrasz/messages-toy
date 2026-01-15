package handlers

import (
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v2"
	"messenger/internal/api/middleware"
	"messenger/internal/database"
	"messenger/internal/models"
)

func TestSettingsHandler_GetConversationSettings_DM(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	app := fiber.New()
	handler := NewSettingsHandler()

	app.Use(middleware.AuthRequired())
	app.Get("/settings", handler.GetConversationSettings)

	user1, token := createTestUser(t, "settingsuser1", "password123")
	user2, _ := createTestUser(t, "settingsuser2", "password123")

	_ = user1 // avoid unused warning

	resp, body := makeRequest(app, testRequest{
		Method: "GET",
		Path:   "/settings?other_user_id=" + user2.ID,
		Token:  token,
	})

	assertStatus(t, resp, http.StatusOK)

	data := parseResponse(body)
	settings := data["settings"].(map[string]interface{})
	assertJSONFieldExists(t, settings, "id")
	assertJSONField(t, settings, "other_user_id", user2.ID)
}

func TestSettingsHandler_GetConversationSettings_Group(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	app := fiber.New()
	handler := NewSettingsHandler()

	app.Use(middleware.AuthRequired())
	app.Get("/settings", handler.GetConversationSettings)

	user, token := createTestUser(t, "groupsettings", "password123")

	// Create a group
	group := models.Group{Name: "Settings Group", CreatedBy: user.ID}
	database.DB.Create(&group)
	database.DB.Create(&models.GroupMember{GroupID: group.ID, UserID: user.ID, Role: models.GroupRoleOwner})

	resp, body := makeRequest(app, testRequest{
		Method: "GET",
		Path:   "/settings?group_id=" + group.ID,
		Token:  token,
	})

	assertStatus(t, resp, http.StatusOK)

	data := parseResponse(body)
	settings := data["settings"].(map[string]interface{})
	assertJSONFieldExists(t, settings, "id")
	assertJSONField(t, settings, "group_id", group.ID)
}

func TestSettingsHandler_GetConversationSettings_MissingParams(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	app := fiber.New()
	handler := NewSettingsHandler()

	app.Use(middleware.AuthRequired())
	app.Get("/settings", handler.GetConversationSettings)

	_, token := createTestUser(t, "missingparams", "password123")

	resp, _ := makeRequest(app, testRequest{
		Method: "GET",
		Path:   "/settings",
		Token:  token,
	})

	assertStatus(t, resp, http.StatusBadRequest)
}

func TestSettingsHandler_SetDisappearingMessages(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	app := fiber.New()
	handler := NewSettingsHandler()

	app.Use(middleware.AuthRequired())
	app.Post("/settings/disappearing", handler.SetDisappearingMessages)

	user1, token := createTestUser(t, "disappearing1", "password123")
	user2, _ := createTestUser(t, "disappearing2", "password123")

	_ = user1 // avoid unused warning

	tests := []struct {
		name           string
		body           map[string]interface{}
		token          string
		expectedStatus int
		checkResponse  func(t *testing.T, data map[string]interface{})
	}{
		{
			name: "set 24 hours",
			body: map[string]interface{}{
				"other_user_id": user2.ID,
				"seconds":       86400,
			},
			token:          token,
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, data map[string]interface{}) {
				assertJSONField(t, data, "success", true)
				assertJSONField(t, data, "disappearing_seconds", float64(86400))
				assertJSONField(t, data, "disappearing_text", "24 hours")
			},
		},
		{
			name: "set 7 days",
			body: map[string]interface{}{
				"other_user_id": user2.ID,
				"seconds":       604800,
			},
			token:          token,
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, data map[string]interface{}) {
				assertJSONField(t, data, "disappearing_text", "7 days")
			},
		},
		{
			name: "set 90 days",
			body: map[string]interface{}{
				"other_user_id": user2.ID,
				"seconds":       7776000,
			},
			token:          token,
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, data map[string]interface{}) {
				assertJSONField(t, data, "disappearing_text", "90 days")
			},
		},
		{
			name: "disable (0)",
			body: map[string]interface{}{
				"other_user_id": user2.ID,
				"seconds":       0,
			},
			token:          token,
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, data map[string]interface{}) {
				assertJSONField(t, data, "disappearing_text", "off")
			},
		},
		{
			name: "invalid duration",
			body: map[string]interface{}{
				"other_user_id": user2.ID,
				"seconds":       12345,
			},
			token:          token,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "missing context",
			body: map[string]interface{}{
				"seconds": 86400,
			},
			token:          token,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "unauthorized",
			body: map[string]interface{}{
				"other_user_id": user2.ID,
				"seconds":       86400,
			},
			token:          "",
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, body := makeRequest(app, testRequest{
				Method: "POST",
				Path:   "/settings/disappearing",
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

func TestSettingsHandler_SetDisappearingMessages_Group(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	app := fiber.New()
	handler := NewSettingsHandler()

	app.Use(middleware.AuthRequired())
	app.Post("/settings/disappearing", handler.SetDisappearingMessages)

	user, token := createTestUser(t, "groupdisappear", "password123")

	// Create a group
	group := models.Group{Name: "Disappearing Group", CreatedBy: user.ID}
	database.DB.Create(&group)
	database.DB.Create(&models.GroupMember{GroupID: group.ID, UserID: user.ID, Role: models.GroupRoleOwner})

	resp, body := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/settings/disappearing",
		Body: map[string]interface{}{
			"group_id": group.ID,
			"seconds":  86400,
		},
		Token: token,
	})

	assertStatus(t, resp, http.StatusOK)

	data := parseResponse(body)
	assertJSONField(t, data, "success", true)
	assertJSONField(t, data, "disappearing_seconds", float64(86400))
}

func TestSettingsHandler_MuteConversation(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	app := fiber.New()
	handler := NewSettingsHandler()

	app.Use(middleware.AuthRequired())
	app.Post("/settings/mute", handler.MuteConversation)

	user1, token := createTestUser(t, "muter1", "password123")
	user2, _ := createTestUser(t, "muter2", "password123")

	_ = user1 // avoid unused warning

	tests := []struct {
		name           string
		body           map[string]interface{}
		token          string
		expectedStatus int
		checkResponse  func(t *testing.T, data map[string]interface{})
	}{
		{
			name: "mute for 8 hours",
			body: map[string]interface{}{
				"other_user_id": user2.ID,
				"hours":         8,
			},
			token:          token,
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, data map[string]interface{}) {
				assertJSONField(t, data, "success", true)
				assertJSONField(t, data, "muted", true)
				assertJSONFieldExists(t, data, "muted_until")
			},
		},
		{
			name: "unmute (0 hours)",
			body: map[string]interface{}{
				"other_user_id": user2.ID,
				"hours":         0,
			},
			token:          token,
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, data map[string]interface{}) {
				assertJSONField(t, data, "success", true)
				assertJSONField(t, data, "muted", false)
			},
		},
		{
			name: "missing context",
			body: map[string]interface{}{
				"hours": 8,
			},
			token:          token,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "unauthorized",
			body: map[string]interface{}{
				"other_user_id": user2.ID,
				"hours":         8,
			},
			token:          "",
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, body := makeRequest(app, testRequest{
				Method: "POST",
				Path:   "/settings/mute",
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

func TestSettingsHandler_MuteConversation_Group(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	app := fiber.New()
	handler := NewSettingsHandler()

	app.Use(middleware.AuthRequired())
	app.Post("/settings/mute", handler.MuteConversation)

	user, token := createTestUser(t, "groupmuter", "password123")

	// Create a group
	group := models.Group{Name: "Mute Group", CreatedBy: user.ID}
	database.DB.Create(&group)
	database.DB.Create(&models.GroupMember{GroupID: group.ID, UserID: user.ID, Role: models.GroupRoleOwner})

	resp, body := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/settings/mute",
		Body: map[string]interface{}{
			"group_id": group.ID,
			"hours":    24,
		},
		Token: token,
	})

	assertStatus(t, resp, http.StatusOK)

	data := parseResponse(body)
	assertJSONField(t, data, "success", true)
	assertJSONField(t, data, "muted", true)
	assertJSONFieldExists(t, data, "muted_until")
}

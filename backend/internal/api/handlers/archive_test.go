package handlers

import (
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v2"
	"messenger/internal/api/middleware"
	"messenger/internal/database"
	"messenger/internal/models"
)

func TestArchiveHandler_Archive(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	app := fiber.New()
	archiveHandler := NewArchiveHandler()

	app.Use(middleware.AuthRequired())
	app.Post("/archive", archiveHandler.Archive)

	// Create test users
	user1, token := createTestUser(t, "archiver", "password123")
	user2, _ := createTestUser(t, "archivee", "password123")

	tests := []struct {
		name           string
		body           map[string]interface{}
		token          string
		expectedStatus int
		checkResponse  func(t *testing.T, data map[string]interface{})
	}{
		{
			name: "archive DM conversation",
			body: map[string]interface{}{
				"other_user_id": user2.ID,
			},
			token:          token,
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, data map[string]interface{}) {
				assertJSONField(t, data, "archived", true)
				assertJSONFieldExists(t, data, "archived_at")
			},
		},
		{
			name: "archive same conversation again (idempotent)",
			body: map[string]interface{}{
				"other_user_id": user2.ID,
			},
			token:          token,
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, data map[string]interface{}) {
				assertJSONField(t, data, "archived", true)
			},
		},
		{
			name: "missing both user and group",
			body: map[string]interface{}{},
			token: token,
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, data map[string]interface{}) {
				assertJSONFieldExists(t, data, "error")
			},
		},
		{
			name:           "unauthorized",
			body:           map[string]interface{}{"other_user_id": user2.ID},
			token:          "",
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, body := makeRequest(app, testRequest{
				Method: "POST",
				Path:   "/archive",
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

	// Verify archive was created in database
	archived := models.IsConversationArchived(database.DB, user1.ID, &user2.ID, nil)
	if !archived {
		t.Error("Expected conversation to be archived in database")
	}
}

func TestArchiveHandler_ArchiveGroup(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	app := fiber.New()
	archiveHandler := NewArchiveHandler()

	app.Use(middleware.AuthRequired())
	app.Post("/archive", archiveHandler.Archive)

	user, token := createTestUser(t, "grouparchiver", "password123")

	// Create a test group
	group := models.Group{
		Name:      "Test Group",
		CreatedBy: user.ID,
	}
	database.DB.Create(&group)

	resp, body := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/archive",
		Body: map[string]interface{}{
			"group_id": group.ID,
		},
		Token: token,
	})

	assertStatus(t, resp, http.StatusOK)

	data := parseResponse(body)
	assertJSONField(t, data, "archived", true)

	// Verify in database
	archived := models.IsConversationArchived(database.DB, user.ID, nil, &group.ID)
	if !archived {
		t.Error("Expected group to be archived in database")
	}
}

func TestArchiveHandler_Unarchive(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	app := fiber.New()
	archiveHandler := NewArchiveHandler()

	app.Use(middleware.AuthRequired())
	app.Post("/archive", archiveHandler.Archive)
	app.Delete("/archive", archiveHandler.Unarchive)

	user1, token := createTestUser(t, "unarchiver", "password123")
	user2, _ := createTestUser(t, "unarchivee", "password123")

	// First archive the conversation
	_, _ = makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/archive",
		Body: map[string]interface{}{
			"other_user_id": user2.ID,
		},
		Token: token,
	})

	tests := []struct {
		name           string
		query          string
		token          string
		expectedStatus int
		checkResponse  func(t *testing.T, data map[string]interface{})
	}{
		{
			name:           "unarchive conversation",
			query:          "other_user_id=" + user2.ID,
			token:          token,
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, data map[string]interface{}) {
				assertJSONField(t, data, "archived", false)
			},
		},
		{
			name:           "unarchive non-existent (no error)",
			query:          "other_user_id=" + user2.ID,
			token:          token,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "missing both user and group",
			query:          "",
			token:          token,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "unauthorized",
			query:          "other_user_id=" + user2.ID,
			token:          "",
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := "/archive"
			if tt.query != "" {
				path += "?" + tt.query
			}

			resp, body := makeRequest(app, testRequest{
				Method: "DELETE",
				Path:   path,
				Token:  tt.token,
			})

			assertStatus(t, resp, tt.expectedStatus)

			if tt.checkResponse != nil {
				data := parseResponse(body)
				tt.checkResponse(t, data)
			}
		})
	}

	// Verify archive was removed from database
	archived := models.IsConversationArchived(database.DB, user1.ID, &user2.ID, nil)
	if archived {
		t.Error("Expected conversation to NOT be archived after unarchive")
	}
}

func TestArchiveHandler_List(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	app := fiber.New()
	archiveHandler := NewArchiveHandler()

	app.Use(middleware.AuthRequired())
	app.Post("/archive", archiveHandler.Archive)
	app.Get("/archive", archiveHandler.List)

	user1, token := createTestUser(t, "lister", "password123")
	user2, _ := createTestUser(t, "contact1", "password123")
	user3, _ := createTestUser(t, "contact2", "password123")

	// Create a group
	group := models.Group{
		Name:      "Archived Group",
		CreatedBy: user1.ID,
	}
	database.DB.Create(&group)

	// Archive some conversations
	_, _ = makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/archive",
		Body:   map[string]interface{}{"other_user_id": user2.ID},
		Token:  token,
	})
	_, _ = makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/archive",
		Body:   map[string]interface{}{"other_user_id": user3.ID},
		Token:  token,
	})
	_, _ = makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/archive",
		Body:   map[string]interface{}{"group_id": group.ID},
		Token:  token,
	})

	// Test listing
	resp, body := makeRequest(app, testRequest{
		Method: "GET",
		Path:   "/archive",
		Token:  token,
	})

	assertStatus(t, resp, http.StatusOK)

	data := parseResponse(body)
	archived := data["archived"].([]interface{})

	if len(archived) != 3 {
		t.Errorf("Expected 3 archived conversations, got %d", len(archived))
	}

	// Check that we have both DMs and groups
	hasGroup := false
	dmCount := 0
	for _, item := range archived {
		itemMap := item.(map[string]interface{})
		if itemMap["type"] == "group" {
			hasGroup = true
		} else if itemMap["type"] == "dm" {
			dmCount++
		}
	}

	if !hasGroup {
		t.Error("Expected at least one group in archived list")
	}
	if dmCount != 2 {
		t.Errorf("Expected 2 DMs in archived list, got %d", dmCount)
	}
}

func TestArchiveHandler_IsArchived(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	app := fiber.New()
	archiveHandler := NewArchiveHandler()

	app.Use(middleware.AuthRequired())
	app.Post("/archive", archiveHandler.Archive)
	app.Get("/archive/check", archiveHandler.IsArchived)

	_, token := createTestUser(t, "checker", "password123")
	user2, _ := createTestUser(t, "checked", "password123")

	// Check before archiving
	resp1, body1 := makeRequest(app, testRequest{
		Method: "GET",
		Path:   "/archive/check?other_user_id=" + user2.ID,
		Token:  token,
	})

	assertStatus(t, resp1, http.StatusOK)
	data1 := parseResponse(body1)
	assertJSONField(t, data1, "archived", false)

	// Archive the conversation
	_, _ = makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/archive",
		Body:   map[string]interface{}{"other_user_id": user2.ID},
		Token:  token,
	})

	// Check after archiving
	resp2, body2 := makeRequest(app, testRequest{
		Method: "GET",
		Path:   "/archive/check?other_user_id=" + user2.ID,
		Token:  token,
	})

	assertStatus(t, resp2, http.StatusOK)
	data2 := parseResponse(body2)
	assertJSONField(t, data2, "archived", true)
}

func TestArchiveHandler_IsArchived_MissingParams(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	app := fiber.New()
	archiveHandler := NewArchiveHandler()

	app.Use(middleware.AuthRequired())
	app.Get("/archive/check", archiveHandler.IsArchived)

	_, token := createTestUser(t, "missingparam", "password123")

	resp, body := makeRequest(app, testRequest{
		Method: "GET",
		Path:   "/archive/check",
		Token:  token,
	})

	assertStatus(t, resp, http.StatusBadRequest)
	data := parseResponse(body)
	assertJSONFieldExists(t, data, "error")
}

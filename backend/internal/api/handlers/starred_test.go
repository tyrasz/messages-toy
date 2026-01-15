package handlers

import (
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v2"
	"messenger/internal/api/middleware"
	"messenger/internal/database"
	"messenger/internal/models"
)

func TestStarredHandler_Star(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	app := fiber.New()
	handler := NewStarredHandler()

	app.Use(middleware.AuthRequired())
	app.Post("/starred/:messageId", handler.Star)

	user1, token := createTestUser(t, "starrer", "password123")
	user2, _ := createTestUser(t, "msgauthor", "password123")

	// Create a message that user1 received
	msg := createTestMessage(t, user2.ID, &user1.ID, nil, "Star this message")

	tests := []struct {
		name           string
		messageID      string
		token          string
		expectedStatus int
		checkResponse  func(t *testing.T, data map[string]interface{})
	}{
		{
			name:           "star message as recipient",
			messageID:      msg.ID,
			token:          token,
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, data map[string]interface{}) {
				assertJSONField(t, data, "starred", true)
				assertJSONField(t, data, "message_id", msg.ID)
				assertJSONFieldExists(t, data, "id")
			},
		},
		{
			name:           "star same message again (idempotent)",
			messageID:      msg.ID,
			token:          token,
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, data map[string]interface{}) {
				assertJSONField(t, data, "starred", true)
			},
		},
		{
			name:           "star non-existent message",
			messageID:      "non-existent-id",
			token:          token,
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "unauthorized",
			messageID:      msg.ID,
			token:          "",
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, body := makeRequest(app, testRequest{
				Method: "POST",
				Path:   "/starred/" + tt.messageID,
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

func TestStarredHandler_Star_AsSender(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	app := fiber.New()
	handler := NewStarredHandler()

	app.Use(middleware.AuthRequired())
	app.Post("/starred/:messageId", handler.Star)

	user1, token := createTestUser(t, "sender", "password123")
	user2, _ := createTestUser(t, "recipient", "password123")

	// Create a message that user1 sent
	msg := createTestMessage(t, user1.ID, &user2.ID, nil, "My own message")

	resp, body := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/starred/" + msg.ID,
		Token:  token,
	})

	assertStatus(t, resp, http.StatusOK)

	data := parseResponse(body)
	assertJSONField(t, data, "starred", true)
}

func TestStarredHandler_Star_GroupMessage(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	app := fiber.New()
	handler := NewStarredHandler()

	app.Use(middleware.AuthRequired())
	app.Post("/starred/:messageId", handler.Star)

	user1, token := createTestUser(t, "groupmember1", "password123")
	user2, _ := createTestUser(t, "groupmember2", "password123")

	// Create a group and add both users
	group := models.Group{Name: "Star Group", CreatedBy: user1.ID}
	database.DB.Create(&group)
	database.DB.Create(&models.GroupMember{GroupID: group.ID, UserID: user1.ID, Role: models.GroupRoleOwner})
	database.DB.Create(&models.GroupMember{GroupID: group.ID, UserID: user2.ID, Role: models.GroupRoleMember})

	// Create a group message
	msg := createTestMessage(t, user2.ID, nil, &group.ID, "Group message to star")

	resp, body := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/starred/" + msg.ID,
		Token:  token,
	})

	assertStatus(t, resp, http.StatusOK)

	data := parseResponse(body)
	assertJSONField(t, data, "starred", true)
}

func TestStarredHandler_Star_NoAccess(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	app := fiber.New()
	handler := NewStarredHandler()

	app.Use(middleware.AuthRequired())
	app.Post("/starred/:messageId", handler.Star)

	user1, _ := createTestUser(t, "sender1", "password123")
	user2, _ := createTestUser(t, "recipient1", "password123")
	_, token3 := createTestUser(t, "outsider", "password123")

	// Create a message between user1 and user2
	msg := createTestMessage(t, user1.ID, &user2.ID, nil, "Private message")

	// User3 tries to star it (no access)
	resp, _ := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/starred/" + msg.ID,
		Token:  token3,
	})

	assertStatus(t, resp, http.StatusForbidden)
}

func TestStarredHandler_Unstar(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	app := fiber.New()
	handler := NewStarredHandler()

	app.Use(middleware.AuthRequired())
	app.Post("/starred/:messageId", handler.Star)
	app.Delete("/starred/:messageId", handler.Unstar)

	user1, token := createTestUser(t, "unstarrer", "password123")
	user2, _ := createTestUser(t, "unstarauthor", "password123")

	msg := createTestMessage(t, user2.ID, &user1.ID, nil, "Unstar me")

	// Star first
	_, _ = makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/starred/" + msg.ID,
		Token:  token,
	})

	// Unstar
	resp, body := makeRequest(app, testRequest{
		Method: "DELETE",
		Path:   "/starred/" + msg.ID,
		Token:  token,
	})

	assertStatus(t, resp, http.StatusOK)

	data := parseResponse(body)
	assertJSONField(t, data, "starred", false)
	assertJSONField(t, data, "message_id", msg.ID)

	// Verify unstarred in database
	isStarred := models.IsMessageStarred(database.DB, user1.ID, msg.ID)
	if isStarred {
		t.Error("Expected message to be unstarred")
	}
}

func TestStarredHandler_Unstar_NotStarred(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	app := fiber.New()
	handler := NewStarredHandler()

	app.Use(middleware.AuthRequired())
	app.Delete("/starred/:messageId", handler.Unstar)

	user1, token := createTestUser(t, "unstarnonexist", "password123")
	user2, _ := createTestUser(t, "unstarauthor2", "password123")

	msg := createTestMessage(t, user2.ID, &user1.ID, nil, "Never starred")

	// Unstar (never was starred - should still succeed)
	resp, body := makeRequest(app, testRequest{
		Method: "DELETE",
		Path:   "/starred/" + msg.ID,
		Token:  token,
	})

	assertStatus(t, resp, http.StatusOK)

	data := parseResponse(body)
	assertJSONField(t, data, "starred", false)
}

func TestStarredHandler_IsStarred(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	app := fiber.New()
	handler := NewStarredHandler()

	app.Use(middleware.AuthRequired())
	app.Post("/starred/:messageId", handler.Star)
	app.Get("/starred/:messageId", handler.IsStarred)

	user1, token := createTestUser(t, "isstarreduser", "password123")
	user2, _ := createTestUser(t, "isstarredauthor", "password123")

	msg := createTestMessage(t, user2.ID, &user1.ID, nil, "Check if starred")

	// Check before starring
	resp1, body1 := makeRequest(app, testRequest{
		Method: "GET",
		Path:   "/starred/" + msg.ID,
		Token:  token,
	})

	assertStatus(t, resp1, http.StatusOK)
	data1 := parseResponse(body1)
	assertJSONField(t, data1, "starred", false)

	// Star
	_, _ = makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/starred/" + msg.ID,
		Token:  token,
	})

	// Check after starring
	resp2, body2 := makeRequest(app, testRequest{
		Method: "GET",
		Path:   "/starred/" + msg.ID,
		Token:  token,
	})

	assertStatus(t, resp2, http.StatusOK)
	data2 := parseResponse(body2)
	assertJSONField(t, data2, "starred", true)
}

func TestStarredHandler_List(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	app := fiber.New()
	handler := NewStarredHandler()

	app.Use(middleware.AuthRequired())
	app.Post("/starred/:messageId", handler.Star)
	app.Get("/starred", handler.List)

	user1, token := createTestUser(t, "liststarrer", "password123")
	user2, _ := createTestUser(t, "listauthor", "password123")

	// Create and star multiple messages
	for i := 0; i < 3; i++ {
		msg := createTestMessage(t, user2.ID, &user1.ID, nil, "Message to star")
		_, _ = makeRequest(app, testRequest{
			Method: "POST",
			Path:   "/starred/" + msg.ID,
			Token:  token,
		})
	}

	// List starred
	resp, body := makeRequest(app, testRequest{
		Method: "GET",
		Path:   "/starred",
		Token:  token,
	})

	assertStatus(t, resp, http.StatusOK)

	data := parseResponse(body)
	starred := data["starred"].([]interface{})

	if len(starred) != 3 {
		t.Errorf("Expected 3 starred messages, got %d", len(starred))
	}

	// Check each starred item has expected fields
	for _, s := range starred {
		item := s.(map[string]interface{})
		assertJSONFieldExists(t, item, "id")
		assertJSONFieldExists(t, item, "message")
		assertJSONFieldExists(t, item, "starred_at")
	}
}

func TestStarredHandler_List_Empty(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	app := fiber.New()
	handler := NewStarredHandler()

	app.Use(middleware.AuthRequired())
	app.Get("/starred", handler.List)

	_, token := createTestUser(t, "nostarred", "password123")

	resp, body := makeRequest(app, testRequest{
		Method: "GET",
		Path:   "/starred",
		Token:  token,
	})

	assertStatus(t, resp, http.StatusOK)

	data := parseResponse(body)
	starred := data["starred"]

	// Empty or null is acceptable
	if starred != nil {
		starredList := starred.([]interface{})
		if len(starredList) != 0 {
			t.Errorf("Expected 0 starred messages, got %d", len(starredList))
		}
	}
}

func TestStarredHandler_List_Pagination(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	app := fiber.New()
	handler := NewStarredHandler()

	app.Use(middleware.AuthRequired())
	app.Post("/starred/:messageId", handler.Star)
	app.Get("/starred", handler.List)

	user1, token := createTestUser(t, "pagestarrer", "password123")
	user2, _ := createTestUser(t, "pageauthor", "password123")

	// Create and star 10 messages
	for i := 0; i < 10; i++ {
		msg := createTestMessage(t, user2.ID, &user1.ID, nil, "Paginated message")
		_, _ = makeRequest(app, testRequest{
			Method: "POST",
			Path:   "/starred/" + msg.ID,
			Token:  token,
		})
	}

	// Request with pagination
	resp, body := makeRequest(app, testRequest{
		Method: "GET",
		Path:   "/starred?limit=5&offset=0",
		Token:  token,
	})

	assertStatus(t, resp, http.StatusOK)

	data := parseResponse(body)
	starred := data["starred"].([]interface{})

	if len(starred) != 5 {
		t.Errorf("Expected 5 starred messages with limit=5, got %d", len(starred))
	}

	assertJSONField(t, data, "limit", float64(5))
	assertJSONField(t, data, "offset", float64(0))
}

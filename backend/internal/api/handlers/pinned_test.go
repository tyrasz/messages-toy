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

func TestPinnedHandler_Pin_DM(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	hub := websocket.NewHub()
	app := fiber.New()
	handler := NewPinnedHandler(hub)

	app.Use(middleware.AuthRequired())
	app.Post("/pinned", handler.Pin)

	user1, token := createTestUser(t, "pinner1", "password123")
	user2, _ := createTestUser(t, "pinner2", "password123")

	// Create a message between user1 and user2
	msg := createTestMessage(t, user1.ID, &user2.ID, nil, "Pin this message")

	tests := []struct {
		name           string
		body           map[string]interface{}
		token          string
		expectedStatus int
		checkResponse  func(t *testing.T, data map[string]interface{})
	}{
		{
			name: "pin message in DM",
			body: map[string]interface{}{
				"message_id":    msg.ID,
				"other_user_id": user2.ID,
			},
			token:          token,
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, data map[string]interface{}) {
				assertJSONFieldExists(t, data, "id")
				assertJSONField(t, data, "message_id", msg.ID)
				assertJSONField(t, data, "pinned_by_id", user1.ID)
			},
		},
		{
			name: "pin replaces previous pin",
			body: map[string]interface{}{
				"message_id":    msg.ID,
				"other_user_id": user2.ID,
			},
			token:          token,
			expectedStatus: http.StatusOK,
		},
		{
			name: "missing message_id",
			body: map[string]interface{}{
				"other_user_id": user2.ID,
			},
			token:          token,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "missing context",
			body: map[string]interface{}{
				"message_id": msg.ID,
			},
			token:          token,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "unauthorized",
			body: map[string]interface{}{
				"message_id":    msg.ID,
				"other_user_id": user2.ID,
			},
			token:          "",
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, body := makeRequest(app, testRequest{
				Method: "POST",
				Path:   "/pinned",
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

func TestPinnedHandler_Pin_Group(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	hub := websocket.NewHub()
	app := fiber.New()
	handler := NewPinnedHandler(hub)

	app.Use(middleware.AuthRequired())
	app.Post("/pinned", handler.Pin)

	user, token := createTestUser(t, "grouppinner", "password123")

	// Create a group
	group := models.Group{Name: "Pin Group", CreatedBy: user.ID}
	database.DB.Create(&group)
	database.DB.Create(&models.GroupMember{GroupID: group.ID, UserID: user.ID, Role: models.GroupRoleOwner})

	// Create a group message
	msg := createTestMessage(t, user.ID, nil, &group.ID, "Group pin message")

	resp, body := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/pinned",
		Body: map[string]interface{}{
			"message_id": msg.ID,
			"group_id":   group.ID,
		},
		Token: token,
	})

	assertStatus(t, resp, http.StatusOK)

	data := parseResponse(body)
	assertJSONField(t, data, "message_id", msg.ID)
}

func TestPinnedHandler_Pin_WrongGroup(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	hub := websocket.NewHub()
	app := fiber.New()
	handler := NewPinnedHandler(hub)

	app.Use(middleware.AuthRequired())
	app.Post("/pinned", handler.Pin)

	user, token := createTestUser(t, "wronggrouppinner", "password123")

	// Create two groups
	group1 := models.Group{Name: "Group 1", CreatedBy: user.ID}
	database.DB.Create(&group1)
	database.DB.Create(&models.GroupMember{GroupID: group1.ID, UserID: user.ID, Role: models.GroupRoleOwner})

	group2 := models.Group{Name: "Group 2", CreatedBy: user.ID}
	database.DB.Create(&group2)
	database.DB.Create(&models.GroupMember{GroupID: group2.ID, UserID: user.ID, Role: models.GroupRoleOwner})

	// Create a message in group1
	msg := createTestMessage(t, user.ID, nil, &group1.ID, "Group 1 message")

	// Try to pin it in group2 (should fail)
	resp, _ := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/pinned",
		Body: map[string]interface{}{
			"message_id": msg.ID,
			"group_id":   group2.ID,
		},
		Token: token,
	})

	assertStatus(t, resp, http.StatusBadRequest)
}

func TestPinnedHandler_Pin_NotGroupMember(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	hub := websocket.NewHub()
	app := fiber.New()
	handler := NewPinnedHandler(hub)

	app.Use(middleware.AuthRequired())
	app.Post("/pinned", handler.Pin)

	owner, _ := createTestUser(t, "pinowner", "password123")
	_, nonMemberToken := createTestUser(t, "pinnonmember", "password123")

	// Create a group
	group := models.Group{Name: "Private Pin Group", CreatedBy: owner.ID}
	database.DB.Create(&group)
	database.DB.Create(&models.GroupMember{GroupID: group.ID, UserID: owner.ID, Role: models.GroupRoleOwner})

	// Create a message
	msg := createTestMessage(t, owner.ID, nil, &group.ID, "Group message")

	// Non-member tries to pin
	resp, _ := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/pinned",
		Body: map[string]interface{}{
			"message_id": msg.ID,
			"group_id":   group.ID,
		},
		Token: nonMemberToken,
	})

	assertStatus(t, resp, http.StatusForbidden)
}

func TestPinnedHandler_Pin_NonExistentMessage(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	hub := websocket.NewHub()
	app := fiber.New()
	handler := NewPinnedHandler(hub)

	app.Use(middleware.AuthRequired())
	app.Post("/pinned", handler.Pin)

	user1, token := createTestUser(t, "nomsgsender", "password123")
	user2, _ := createTestUser(t, "nomsgrecv", "password123")

	resp, _ := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/pinned",
		Body: map[string]interface{}{
			"message_id":    "nonexistent-id",
			"other_user_id": user2.ID,
		},
		Token: token,
	})

	assertStatus(t, resp, http.StatusNotFound)
	_ = user1 // avoid unused warning
}

func TestPinnedHandler_Unpin_DM(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	hub := websocket.NewHub()
	app := fiber.New()
	handler := NewPinnedHandler(hub)

	app.Use(middleware.AuthRequired())
	app.Post("/pinned", handler.Pin)
	app.Delete("/pinned", handler.Unpin)

	user1, token := createTestUser(t, "unpinner1", "password123")
	user2, _ := createTestUser(t, "unpinner2", "password123")

	// Create and pin a message
	msg := createTestMessage(t, user1.ID, &user2.ID, nil, "Unpin me")
	_, _ = makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/pinned",
		Body: map[string]interface{}{
			"message_id":    msg.ID,
			"other_user_id": user2.ID,
		},
		Token: token,
	})

	// Unpin
	resp, body := makeRequest(app, testRequest{
		Method: "DELETE",
		Path:   "/pinned?other_user_id=" + user2.ID,
		Token:  token,
	})

	assertStatus(t, resp, http.StatusOK)

	data := parseResponse(body)
	assertJSONField(t, data, "success", true)
}

func TestPinnedHandler_Unpin_Group(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	hub := websocket.NewHub()
	app := fiber.New()
	handler := NewPinnedHandler(hub)

	app.Use(middleware.AuthRequired())
	app.Post("/pinned", handler.Pin)
	app.Delete("/pinned", handler.Unpin)

	user, token := createTestUser(t, "groupunpinner", "password123")

	// Create a group
	group := models.Group{Name: "Unpin Group", CreatedBy: user.ID}
	database.DB.Create(&group)
	database.DB.Create(&models.GroupMember{GroupID: group.ID, UserID: user.ID, Role: models.GroupRoleOwner})

	// Create and pin a message
	msg := createTestMessage(t, user.ID, nil, &group.ID, "Group unpin")
	_, _ = makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/pinned",
		Body: map[string]interface{}{
			"message_id": msg.ID,
			"group_id":   group.ID,
		},
		Token: token,
	})

	// Unpin
	resp, body := makeRequest(app, testRequest{
		Method: "DELETE",
		Path:   "/pinned?group_id=" + group.ID,
		Token:  token,
	})

	assertStatus(t, resp, http.StatusOK)

	data := parseResponse(body)
	assertJSONField(t, data, "success", true)
}

func TestPinnedHandler_Unpin_MissingParams(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	hub := websocket.NewHub()
	app := fiber.New()
	handler := NewPinnedHandler(hub)

	app.Use(middleware.AuthRequired())
	app.Delete("/pinned", handler.Unpin)

	_, token := createTestUser(t, "unpinmissing", "password123")

	resp, _ := makeRequest(app, testRequest{
		Method: "DELETE",
		Path:   "/pinned",
		Token:  token,
	})

	assertStatus(t, resp, http.StatusBadRequest)
}

func TestPinnedHandler_Get_DM(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	hub := websocket.NewHub()
	app := fiber.New()
	handler := NewPinnedHandler(hub)

	app.Use(middleware.AuthRequired())
	app.Post("/pinned", handler.Pin)
	app.Get("/pinned", handler.Get)

	user1, token := createTestUser(t, "pingetter1", "password123")
	user2, _ := createTestUser(t, "pingetter2", "password123")

	// Create and pin a message
	msg := createTestMessage(t, user1.ID, &user2.ID, nil, "Get pinned")
	_, _ = makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/pinned",
		Body: map[string]interface{}{
			"message_id":    msg.ID,
			"other_user_id": user2.ID,
		},
		Token: token,
	})

	// Get pinned
	resp, body := makeRequest(app, testRequest{
		Method: "GET",
		Path:   "/pinned?other_user_id=" + user2.ID,
		Token:  token,
	})

	assertStatus(t, resp, http.StatusOK)

	data := parseResponse(body)
	assertJSONField(t, data, "message_id", msg.ID)
}

func TestPinnedHandler_Get_Group(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	hub := websocket.NewHub()
	app := fiber.New()
	handler := NewPinnedHandler(hub)

	app.Use(middleware.AuthRequired())
	app.Post("/pinned", handler.Pin)
	app.Get("/pinned", handler.Get)

	user, token := createTestUser(t, "grouppingetter", "password123")

	// Create a group
	group := models.Group{Name: "Get Pin Group", CreatedBy: user.ID}
	database.DB.Create(&group)
	database.DB.Create(&models.GroupMember{GroupID: group.ID, UserID: user.ID, Role: models.GroupRoleOwner})

	// Create and pin a message
	msg := createTestMessage(t, user.ID, nil, &group.ID, "Group pinned")
	_, _ = makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/pinned",
		Body: map[string]interface{}{
			"message_id": msg.ID,
			"group_id":   group.ID,
		},
		Token: token,
	})

	// Get pinned
	resp, body := makeRequest(app, testRequest{
		Method: "GET",
		Path:   "/pinned?group_id=" + group.ID,
		Token:  token,
	})

	assertStatus(t, resp, http.StatusOK)

	data := parseResponse(body)
	assertJSONField(t, data, "message_id", msg.ID)
}

func TestPinnedHandler_Get_NoPinnedMessage(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	hub := websocket.NewHub()
	app := fiber.New()
	handler := NewPinnedHandler(hub)

	app.Use(middleware.AuthRequired())
	app.Get("/pinned", handler.Get)

	user1, token := createTestUser(t, "nopinget1", "password123")
	user2, _ := createTestUser(t, "nopinget2", "password123")

	_ = user1 // avoid unused warning

	resp, _ := makeRequest(app, testRequest{
		Method: "GET",
		Path:   "/pinned?other_user_id=" + user2.ID,
		Token:  token,
	})

	assertStatus(t, resp, http.StatusNotFound)
}

func TestPinnedHandler_Get_MissingParams(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	hub := websocket.NewHub()
	app := fiber.New()
	handler := NewPinnedHandler(hub)

	app.Use(middleware.AuthRequired())
	app.Get("/pinned", handler.Get)

	_, token := createTestUser(t, "getmissing", "password123")

	resp, _ := makeRequest(app, testRequest{
		Method: "GET",
		Path:   "/pinned",
		Token:  token,
	})

	assertStatus(t, resp, http.StatusBadRequest)
}

func TestPinnedHandler_Get_NotGroupMember(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	hub := websocket.NewHub()
	app := fiber.New()
	handler := NewPinnedHandler(hub)

	app.Use(middleware.AuthRequired())
	app.Get("/pinned", handler.Get)

	owner, _ := createTestUser(t, "pinnedowner", "password123")
	_, nonMemberToken := createTestUser(t, "pinnednonmember", "password123")

	// Create a group
	group := models.Group{Name: "Private Pinned", CreatedBy: owner.ID}
	database.DB.Create(&group)
	database.DB.Create(&models.GroupMember{GroupID: group.ID, UserID: owner.ID, Role: models.GroupRoleOwner})

	// Non-member tries to get pinned
	resp, _ := makeRequest(app, testRequest{
		Method: "GET",
		Path:   "/pinned?group_id=" + group.ID,
		Token:  nonMemberToken,
	})

	assertStatus(t, resp, http.StatusForbidden)
}

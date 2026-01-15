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

func TestGroupsHandler_Create(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	hub := websocket.NewHub()
	app := fiber.New()
	handler := NewGroupsHandler(hub)

	app.Use(middleware.AuthRequired())
	app.Post("/groups", handler.Create)

	user, token := createTestUser(t, "groupcreator", "password123")

	tests := []struct {
		name           string
		body           map[string]interface{}
		token          string
		expectedStatus int
		checkResponse  func(t *testing.T, data map[string]interface{})
	}{
		{
			name: "create group",
			body: map[string]interface{}{
				"name": "Test Group",
			},
			token:          token,
			expectedStatus: http.StatusCreated,
			checkResponse: func(t *testing.T, data map[string]interface{}) {
				assertJSONFieldExists(t, data, "id")
				assertJSONField(t, data, "name", "Test Group")
				assertJSONField(t, data, "created_by", user.ID)
				assertJSONField(t, data, "my_role", "owner")
			},
		},
		{
			name: "create group with description",
			body: map[string]interface{}{
				"name":        "Described Group",
				"description": "A group with a description",
			},
			token:          token,
			expectedStatus: http.StatusCreated,
			checkResponse: func(t *testing.T, data map[string]interface{}) {
				assertJSONField(t, data, "description", "A group with a description")
			},
		},
		{
			name: "missing name",
			body: map[string]interface{}{
				"description": "No name",
			},
			token:          token,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "empty body",
			body:           map[string]interface{}{},
			token:          token,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "unauthorized",
			body: map[string]interface{}{
				"name": "Test",
			},
			token:          "",
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, body := makeRequest(app, testRequest{
				Method: "POST",
				Path:   "/groups",
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

func TestGroupsHandler_Create_WithMembers(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	hub := websocket.NewHub()
	app := fiber.New()
	handler := NewGroupsHandler(hub)

	app.Use(middleware.AuthRequired())
	app.Post("/groups", handler.Create)

	_, token := createTestUser(t, "grouphost", "password123")
	member1, _ := createTestUser(t, "member1", "password123")
	member2, _ := createTestUser(t, "member2", "password123")

	resp, body := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/groups",
		Body: map[string]interface{}{
			"name":       "Group with Members",
			"member_ids": []string{member1.ID, member2.ID},
		},
		Token: token,
	})

	assertStatus(t, resp, http.StatusCreated)

	data := parseResponse(body)
	// Should have 3 members (creator + 2 added)
	assertJSONField(t, data, "member_count", float64(3))
}

func TestGroupsHandler_List(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	hub := websocket.NewHub()
	app := fiber.New()
	handler := NewGroupsHandler(hub)

	app.Use(middleware.AuthRequired())
	app.Post("/groups", handler.Create)
	app.Get("/groups", handler.List)

	_, token := createTestUser(t, "grouplister", "password123")

	// Create some groups
	for i := 0; i < 3; i++ {
		_, _ = makeRequest(app, testRequest{
			Method: "POST",
			Path:   "/groups",
			Body:   map[string]interface{}{"name": "Group " + string(rune('A'+i))},
			Token:  token,
		})
	}

	// List groups
	resp, body := makeRequest(app, testRequest{
		Method: "GET",
		Path:   "/groups",
		Token:  token,
	})

	assertStatus(t, resp, http.StatusOK)

	data := parseResponse(body)
	groups := data["groups"].([]interface{})

	if len(groups) != 3 {
		t.Errorf("Expected 3 groups, got %d", len(groups))
	}

	// Check each group has expected fields
	for _, g := range groups {
		group := g.(map[string]interface{})
		assertJSONFieldExists(t, group, "id")
		assertJSONFieldExists(t, group, "name")
		assertJSONFieldExists(t, group, "member_count")
		assertJSONFieldExists(t, group, "my_role")
	}
}

func TestGroupsHandler_List_Empty(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	hub := websocket.NewHub()
	app := fiber.New()
	handler := NewGroupsHandler(hub)

	app.Use(middleware.AuthRequired())
	app.Get("/groups", handler.List)

	_, token := createTestUser(t, "nogroups", "password123")

	resp, body := makeRequest(app, testRequest{
		Method: "GET",
		Path:   "/groups",
		Token:  token,
	})

	assertStatus(t, resp, http.StatusOK)

	data := parseResponse(body)
	groups := data["groups"].([]interface{})

	if len(groups) != 0 {
		t.Errorf("Expected 0 groups, got %d", len(groups))
	}
}

func TestGroupsHandler_Get(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	hub := websocket.NewHub()
	app := fiber.New()
	handler := NewGroupsHandler(hub)

	app.Use(middleware.AuthRequired())
	app.Post("/groups", handler.Create)
	app.Get("/groups/:id", handler.Get)

	_, token := createTestUser(t, "groupgetter", "password123")

	// Create a group
	resp1, body1 := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/groups",
		Body:   map[string]interface{}{"name": "Fetchable Group", "description": "Test desc"},
		Token:  token,
	})
	assertStatus(t, resp1, http.StatusCreated)

	data1 := parseResponse(body1)
	groupID := data1["id"].(string)

	// Get the group
	resp2, body2 := makeRequest(app, testRequest{
		Method: "GET",
		Path:   "/groups/" + groupID,
		Token:  token,
	})

	assertStatus(t, resp2, http.StatusOK)

	data2 := parseResponse(body2)
	assertJSONField(t, data2, "id", groupID)
	assertJSONField(t, data2, "name", "Fetchable Group")
	assertJSONField(t, data2, "description", "Test desc")
	assertJSONFieldExists(t, data2, "members")
}

func TestGroupsHandler_Get_NotMember(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	hub := websocket.NewHub()
	app := fiber.New()
	handler := NewGroupsHandler(hub)

	app.Use(middleware.AuthRequired())
	app.Post("/groups", handler.Create)
	app.Get("/groups/:id", handler.Get)

	_, token1 := createTestUser(t, "owner", "password123")
	_, token2 := createTestUser(t, "nonmember", "password123")

	// Create a group
	_, body1 := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/groups",
		Body:   map[string]interface{}{"name": "Private Group"},
		Token:  token1,
	})
	data1 := parseResponse(body1)
	groupID := data1["id"].(string)

	// Try to get as non-member
	resp2, body2 := makeRequest(app, testRequest{
		Method: "GET",
		Path:   "/groups/" + groupID,
		Token:  token2,
	})

	assertStatus(t, resp2, http.StatusForbidden)
	data2 := parseResponse(body2)
	assertJSONFieldExists(t, data2, "error")
}

func TestGroupsHandler_AddMember(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	hub := websocket.NewHub()
	app := fiber.New()
	handler := NewGroupsHandler(hub)

	app.Use(middleware.AuthRequired())
	app.Post("/groups", handler.Create)
	app.Post("/groups/:id/members", handler.AddMember)

	_, ownerToken := createTestUser(t, "addmemberowner", "password123")
	newMember, _ := createTestUser(t, "newmember", "password123")

	// Create a group
	_, body1 := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/groups",
		Body:   map[string]interface{}{"name": "Add Member Test"},
		Token:  ownerToken,
	})
	data1 := parseResponse(body1)
	groupID := data1["id"].(string)

	// Add member
	resp2, body2 := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/groups/" + groupID + "/members",
		Body:   map[string]interface{}{"user_id": newMember.ID},
		Token:  ownerToken,
	})

	assertStatus(t, resp2, http.StatusCreated)
	data2 := parseResponse(body2)
	assertJSONFieldExists(t, data2, "message")
	assertJSONFieldExists(t, data2, "member")

	// Verify in database
	var count int64
	database.DB.Model(&models.GroupMember{}).Where("group_id = ?", groupID).Count(&count)
	if count != 2 {
		t.Errorf("Expected 2 members (owner + new), got %d", count)
	}
}

func TestGroupsHandler_AddMember_NotAdmin(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	hub := websocket.NewHub()
	app := fiber.New()
	handler := NewGroupsHandler(hub)

	app.Use(middleware.AuthRequired())
	app.Post("/groups", handler.Create)
	app.Post("/groups/:id/members", handler.AddMember)

	_, ownerToken := createTestUser(t, "memberowner", "password123")
	member, memberToken := createTestUser(t, "regularmember", "password123")
	newUser, _ := createTestUser(t, "wannabemember", "password123")

	// Create a group with member
	_, body1 := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/groups",
		Body:   map[string]interface{}{"name": "Test Group", "member_ids": []string{member.ID}},
		Token:  ownerToken,
	})
	data1 := parseResponse(body1)
	groupID := data1["id"].(string)

	// Try to add as regular member (should fail)
	resp2, body2 := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/groups/" + groupID + "/members",
		Body:   map[string]interface{}{"user_id": newUser.ID},
		Token:  memberToken,
	})

	assertStatus(t, resp2, http.StatusForbidden)
	data2 := parseResponse(body2)
	assertJSONFieldExists(t, data2, "error")
}

func TestGroupsHandler_AddMember_AlreadyMember(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	hub := websocket.NewHub()
	app := fiber.New()
	handler := NewGroupsHandler(hub)

	app.Use(middleware.AuthRequired())
	app.Post("/groups", handler.Create)
	app.Post("/groups/:id/members", handler.AddMember)

	_, ownerToken := createTestUser(t, "dupowner", "password123")
	member, _ := createTestUser(t, "dupmember", "password123")

	// Create a group with member
	_, body1 := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/groups",
		Body:   map[string]interface{}{"name": "Dup Test", "member_ids": []string{member.ID}},
		Token:  ownerToken,
	})
	data1 := parseResponse(body1)
	groupID := data1["id"].(string)

	// Try to add same member again
	resp2, _ := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/groups/" + groupID + "/members",
		Body:   map[string]interface{}{"user_id": member.ID},
		Token:  ownerToken,
	})

	assertStatus(t, resp2, http.StatusConflict)
}

func TestGroupsHandler_RemoveMember(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	hub := websocket.NewHub()
	app := fiber.New()
	handler := NewGroupsHandler(hub)

	app.Use(middleware.AuthRequired())
	app.Post("/groups", handler.Create)
	app.Delete("/groups/:id/members/:userId", handler.RemoveMember)

	_, ownerToken := createTestUser(t, "removeowner", "password123")
	member, _ := createTestUser(t, "removablemember", "password123")

	// Create a group with member
	_, body1 := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/groups",
		Body:   map[string]interface{}{"name": "Remove Test", "member_ids": []string{member.ID}},
		Token:  ownerToken,
	})
	data1 := parseResponse(body1)
	groupID := data1["id"].(string)

	// Remove member
	resp2, body2 := makeRequest(app, testRequest{
		Method: "DELETE",
		Path:   "/groups/" + groupID + "/members/" + member.ID,
		Token:  ownerToken,
	})

	assertStatus(t, resp2, http.StatusOK)
	data2 := parseResponse(body2)
	assertJSONFieldExists(t, data2, "message")

	// Verify removed from database
	var count int64
	database.DB.Model(&models.GroupMember{}).Where("group_id = ? AND user_id = ?", groupID, member.ID).Count(&count)
	if count != 0 {
		t.Error("Expected member to be removed from database")
	}
}

func TestGroupsHandler_RemoveMember_CannotRemoveOwner(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	hub := websocket.NewHub()
	app := fiber.New()
	handler := NewGroupsHandler(hub)

	app.Use(middleware.AuthRequired())
	app.Post("/groups", handler.Create)
	app.Delete("/groups/:id/members/:userId", handler.RemoveMember)

	owner, ownerToken := createTestUser(t, "selfremoveowner", "password123")

	// Create a group
	_, body1 := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/groups",
		Body:   map[string]interface{}{"name": "Owner Remove Test"},
		Token:  ownerToken,
	})
	data1 := parseResponse(body1)
	groupID := data1["id"].(string)

	// Try to remove owner
	resp2, _ := makeRequest(app, testRequest{
		Method: "DELETE",
		Path:   "/groups/" + groupID + "/members/" + owner.ID,
		Token:  ownerToken,
	})

	assertStatus(t, resp2, http.StatusForbidden)
}

func TestGroupsHandler_Leave(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	hub := websocket.NewHub()
	app := fiber.New()
	handler := NewGroupsHandler(hub)

	app.Use(middleware.AuthRequired())
	app.Post("/groups", handler.Create)
	app.Post("/groups/:id/leave", handler.Leave)

	_, ownerToken := createTestUser(t, "leaveowner", "password123")
	member, memberToken := createTestUser(t, "leavingmember", "password123")

	// Create a group with member
	_, body1 := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/groups",
		Body:   map[string]interface{}{"name": "Leave Test", "member_ids": []string{member.ID}},
		Token:  ownerToken,
	})
	data1 := parseResponse(body1)
	groupID := data1["id"].(string)

	// Member leaves
	resp2, body2 := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/groups/" + groupID + "/leave",
		Token:  memberToken,
	})

	assertStatus(t, resp2, http.StatusOK)
	data2 := parseResponse(body2)
	assertJSONFieldExists(t, data2, "message")

	// Verify left
	var count int64
	database.DB.Model(&models.GroupMember{}).Where("group_id = ? AND user_id = ?", groupID, member.ID).Count(&count)
	if count != 0 {
		t.Error("Expected member to have left")
	}
}

func TestGroupsHandler_Leave_OwnerCannotLeave(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	hub := websocket.NewHub()
	app := fiber.New()
	handler := NewGroupsHandler(hub)

	app.Use(middleware.AuthRequired())
	app.Post("/groups", handler.Create)
	app.Post("/groups/:id/leave", handler.Leave)

	_, ownerToken := createTestUser(t, "stuckowner", "password123")
	member, _ := createTestUser(t, "othermember", "password123")

	// Create a group with member
	_, body1 := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/groups",
		Body:   map[string]interface{}{"name": "Owner Leave Test", "member_ids": []string{member.ID}},
		Token:  ownerToken,
	})
	data1 := parseResponse(body1)
	groupID := data1["id"].(string)

	// Owner tries to leave (should fail since there are other members)
	resp2, _ := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/groups/" + groupID + "/leave",
		Token:  ownerToken,
	})

	assertStatus(t, resp2, http.StatusBadRequest)
}

func TestGroupsHandler_Leave_OwnerDeletesEmptyGroup(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	hub := websocket.NewHub()
	app := fiber.New()
	handler := NewGroupsHandler(hub)

	app.Use(middleware.AuthRequired())
	app.Post("/groups", handler.Create)
	app.Post("/groups/:id/leave", handler.Leave)

	_, ownerToken := createTestUser(t, "soleowner", "password123")

	// Create a group (owner only)
	_, body1 := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/groups",
		Body:   map[string]interface{}{"name": "Solo Group"},
		Token:  ownerToken,
	})
	data1 := parseResponse(body1)
	groupID := data1["id"].(string)

	// Owner leaves (should delete group)
	resp2, body2 := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/groups/" + groupID + "/leave",
		Token:  ownerToken,
	})

	assertStatus(t, resp2, http.StatusOK)
	data2 := parseResponse(body2)
	if msg, ok := data2["message"].(string); ok {
		if msg != "Group deleted (you were the only member)" {
			t.Errorf("Unexpected message: %s", msg)
		}
	}

	// Verify group deleted
	var count int64
	database.DB.Model(&models.Group{}).Where("id = ?", groupID).Count(&count)
	if count != 0 {
		t.Error("Expected group to be deleted")
	}
}

func TestGroupsHandler_GetMessages(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	hub := websocket.NewHub()
	app := fiber.New()
	handler := NewGroupsHandler(hub)

	app.Use(middleware.AuthRequired())
	app.Post("/groups", handler.Create)
	app.Get("/groups/:id/messages", handler.GetMessages)

	user, token := createTestUser(t, "groupmsger", "password123")

	// Create a group
	_, body1 := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/groups",
		Body:   map[string]interface{}{"name": "Message Group"},
		Token:  token,
	})
	data1 := parseResponse(body1)
	groupID := data1["id"].(string)

	// Create some messages
	for i := 0; i < 5; i++ {
		createTestMessage(t, user.ID, nil, &groupID, "Group message")
	}

	// Get messages
	resp2, body2 := makeRequest(app, testRequest{
		Method: "GET",
		Path:   "/groups/" + groupID + "/messages",
		Token:  token,
	})

	assertStatus(t, resp2, http.StatusOK)

	data2 := parseResponse(body2)
	messages := data2["messages"].([]interface{})

	if len(messages) != 5 {
		t.Errorf("Expected 5 messages, got %d", len(messages))
	}
}

func TestGroupsHandler_GetMessages_NotMember(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	hub := websocket.NewHub()
	app := fiber.New()
	handler := NewGroupsHandler(hub)

	app.Use(middleware.AuthRequired())
	app.Post("/groups", handler.Create)
	app.Get("/groups/:id/messages", handler.GetMessages)

	_, ownerToken := createTestUser(t, "msgowner", "password123")
	_, nonMemberToken := createTestUser(t, "msgnonmember", "password123")

	// Create a group
	_, body1 := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/groups",
		Body:   map[string]interface{}{"name": "Private Messages"},
		Token:  ownerToken,
	})
	data1 := parseResponse(body1)
	groupID := data1["id"].(string)

	// Non-member tries to get messages
	resp2, _ := makeRequest(app, testRequest{
		Method: "GET",
		Path:   "/groups/" + groupID + "/messages",
		Token:  nonMemberToken,
	})

	assertStatus(t, resp2, http.StatusForbidden)
}

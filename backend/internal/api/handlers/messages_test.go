package handlers

import (
	"net/http"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"messenger/internal/api/middleware"
	"messenger/internal/database"
	"messenger/internal/models"
)

func TestMessagesHandler_GetHistory(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	app := fiber.New()
	handler := NewMessagesHandler(nil)

	app.Use(middleware.AuthRequired())
	app.Get("/messages/:userId", handler.GetHistory)

	// Create test users
	user1, token := createTestUser(t, "messager1", "password123")
	user2, _ := createTestUser(t, "messager2", "password123")

	// Create some messages between users
	for i := 0; i < 5; i++ {
		createTestMessage(t, user1.ID, &user2.ID, nil, "Hello from user1")
		createTestMessage(t, user2.ID, &user1.ID, nil, "Hello from user2")
	}

	resp, body := makeRequest(app, testRequest{
		Method: "GET",
		Path:   "/messages/" + user2.ID,
		Token:  token,
	})

	assertStatus(t, resp, http.StatusOK)

	data := parseResponse(body)
	messages := data["messages"].([]interface{})

	if len(messages) != 10 {
		t.Errorf("Expected 10 messages, got %d", len(messages))
	}

	// Check pagination info
	assertJSONField(t, data, "limit", float64(50))
	assertJSONField(t, data, "offset", float64(0))
}

func TestMessagesHandler_GetHistory_Pagination(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	app := fiber.New()
	handler := NewMessagesHandler(nil)

	app.Use(middleware.AuthRequired())
	app.Get("/messages/:userId", handler.GetHistory)

	user1, token := createTestUser(t, "paginateuser1", "password123")
	user2, _ := createTestUser(t, "paginateuser2", "password123")

	// Create 20 messages
	for i := 0; i < 20; i++ {
		createTestMessage(t, user1.ID, &user2.ID, nil, "Message")
	}

	// Request with pagination
	resp, body := makeRequest(app, testRequest{
		Method: "GET",
		Path:   "/messages/" + user2.ID + "?limit=5&offset=0",
		Token:  token,
	})

	assertStatus(t, resp, http.StatusOK)

	data := parseResponse(body)
	messages := data["messages"].([]interface{})

	if len(messages) != 5 {
		t.Errorf("Expected 5 messages with limit=5, got %d", len(messages))
	}
}

func TestMessagesHandler_GetHistory_LimitMax(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	app := fiber.New()
	handler := NewMessagesHandler(nil)

	app.Use(middleware.AuthRequired())
	app.Get("/messages/:userId", handler.GetHistory)

	user1, token := createTestUser(t, "maxlimituser1", "password123")
	user2, _ := createTestUser(t, "maxlimituser2", "password123")

	createTestMessage(t, user1.ID, &user2.ID, nil, "Test")

	// Request with limit > 100
	resp, body := makeRequest(app, testRequest{
		Method: "GET",
		Path:   "/messages/" + user2.ID + "?limit=200",
		Token:  token,
	})

	assertStatus(t, resp, http.StatusOK)

	data := parseResponse(body)
	// Limit should be capped at 100
	assertJSONField(t, data, "limit", float64(100))
}

func TestMessagesHandler_GetHistory_Empty(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	app := fiber.New()
	handler := NewMessagesHandler(nil)

	app.Use(middleware.AuthRequired())
	app.Get("/messages/:userId", handler.GetHistory)

	_, token := createTestUser(t, "emptyhistory1", "password123")
	user2, _ := createTestUser(t, "emptyhistory2", "password123")

	resp, body := makeRequest(app, testRequest{
		Method: "GET",
		Path:   "/messages/" + user2.ID,
		Token:  token,
	})

	assertStatus(t, resp, http.StatusOK)

	data := parseResponse(body)
	messages := data["messages"].([]interface{})

	if len(messages) != 0 {
		t.Errorf("Expected 0 messages, got %d", len(messages))
	}
}

func TestMessagesHandler_GetConversations(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	app := fiber.New()
	handler := NewMessagesHandler(nil)

	app.Use(middleware.AuthRequired())
	app.Get("/messages/conversations", handler.GetConversations)

	user1, token := createTestUser(t, "convuser1", "password123")
	user2, _ := createTestUser(t, "convuser2", "password123")
	user3, _ := createTestUser(t, "convuser3", "password123")

	// Create messages with different users
	createTestMessage(t, user1.ID, &user2.ID, nil, "Hello user2")
	createTestMessage(t, user2.ID, &user1.ID, nil, "Hello back")
	createTestMessage(t, user3.ID, &user1.ID, nil, "Hello from user3")

	resp, body := makeRequest(app, testRequest{
		Method: "GET",
		Path:   "/messages/conversations",
		Token:  token,
	})

	assertStatus(t, resp, http.StatusOK)

	data := parseResponse(body)
	conversations := data["conversations"].([]interface{})

	if len(conversations) != 2 {
		t.Errorf("Expected 2 conversations, got %d", len(conversations))
	}

	// Check that each conversation has expected fields
	for _, c := range conversations {
		conv := c.(map[string]interface{})
		assertJSONFieldExists(t, conv, "user")
		assertJSONFieldExists(t, conv, "last_message")
		assertJSONFieldExists(t, conv, "unread_count")
	}
}

func TestMessagesHandler_GetConversations_Empty(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	app := fiber.New()
	handler := NewMessagesHandler(nil)

	app.Use(middleware.AuthRequired())
	app.Get("/messages/conversations", handler.GetConversations)

	_, token := createTestUser(t, "noconvuser", "password123")

	resp, body := makeRequest(app, testRequest{
		Method: "GET",
		Path:   "/messages/conversations",
		Token:  token,
	})

	assertStatus(t, resp, http.StatusOK)

	data := parseResponse(body)
	conversations := data["conversations"]

	// Empty or null is acceptable
	if conversations != nil {
		convList := conversations.([]interface{})
		if len(convList) != 0 {
			t.Errorf("Expected 0 conversations, got %d", len(convList))
		}
	}
}

func TestMessagesHandler_Search(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	app := fiber.New()
	handler := NewMessagesHandler(nil)

	app.Use(middleware.AuthRequired())
	app.Get("/messages/search", handler.Search)

	user1, token := createTestUser(t, "searcher", "password123")
	user2, _ := createTestUser(t, "searchee", "password123")

	// Create messages with different content
	createTestMessage(t, user1.ID, &user2.ID, nil, "Hello world")
	createTestMessage(t, user1.ID, &user2.ID, nil, "How are you")
	createTestMessage(t, user1.ID, &user2.ID, nil, "World cup")

	tests := []struct {
		name           string
		query          string
		expectedStatus int
		expectedCount  int
	}{
		{
			name:           "search for world",
			query:          "world",
			expectedStatus: http.StatusOK,
			expectedCount:  2,
		},
		{
			name:           "search for hello",
			query:          "Hello",
			expectedStatus: http.StatusOK,
			expectedCount:  1,
		},
		{
			name:           "search for non-existent",
			query:          "xyz123",
			expectedStatus: http.StatusOK,
			expectedCount:  0,
		},
		{
			name:           "query too short",
			query:          "a",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "empty query",
			query:          "",
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, body := makeRequest(app, testRequest{
				Method: "GET",
				Path:   "/messages/search?q=" + tt.query,
				Token:  token,
			})

			assertStatus(t, resp, tt.expectedStatus)

			if tt.expectedStatus == http.StatusOK {
				data := parseResponse(body)
				results := data["results"]
				if results == nil {
					if tt.expectedCount != 0 {
						t.Errorf("Expected %d results, got nil", tt.expectedCount)
					}
				} else {
					resultList := results.([]interface{})
					if len(resultList) != tt.expectedCount {
						t.Errorf("Expected %d results, got %d", tt.expectedCount, len(resultList))
					}
				}
			}
		})
	}
}

func TestMessagesHandler_Search_Pagination(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	app := fiber.New()
	handler := NewMessagesHandler(nil)

	app.Use(middleware.AuthRequired())
	app.Get("/messages/search", handler.Search)

	user1, token := createTestUser(t, "searchpager", "password123")
	user2, _ := createTestUser(t, "searchpagee", "password123")

	// Create many messages
	for i := 0; i < 30; i++ {
		createTestMessage(t, user1.ID, &user2.ID, nil, "Test message content")
	}

	resp, body := makeRequest(app, testRequest{
		Method: "GET",
		Path:   "/messages/search?q=Test&limit=10",
		Token:  token,
	})

	assertStatus(t, resp, http.StatusOK)

	data := parseResponse(body)
	assertJSONField(t, data, "limit", float64(10))
}

func TestMessagesHandler_Search_InGroups(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	app := fiber.New()
	handler := NewMessagesHandler(nil)

	app.Use(middleware.AuthRequired())
	app.Get("/messages/search", handler.Search)

	user1, token := createTestUser(t, "groupsearcher", "password123")

	// Create a group and add user as member
	group := models.Group{
		Name:      "Test Group",
		CreatedBy: user1.ID,
	}
	database.DB.Create(&group)

	// Add user as member
	database.DB.Create(&models.GroupMember{
		GroupID: group.ID,
		UserID:  user1.ID,
		Role:    "admin",
	})

	// Create group message
	createTestMessage(t, user1.ID, nil, &group.ID, "Group message searchable")

	resp, body := makeRequest(app, testRequest{
		Method: "GET",
		Path:   "/messages/search?q=searchable",
		Token:  token,
	})

	assertStatus(t, resp, http.StatusOK)

	data := parseResponse(body)
	results := data["results"].([]interface{})

	if len(results) != 1 {
		t.Errorf("Expected 1 result from group search, got %d", len(results))
	}

	// Should include group info
	result := results[0].(map[string]interface{})
	if _, hasGroup := result["group"]; !hasGroup {
		t.Error("Expected result to include group info")
	}
}

func TestMessagesHandler_Unauthorized(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	app := fiber.New()
	handler := NewMessagesHandler(nil)

	app.Use(middleware.AuthRequired())
	app.Get("/messages/:userId", handler.GetHistory)
	app.Get("/messages/conversations", handler.GetConversations)
	app.Get("/messages/search", handler.Search)

	tests := []struct {
		name string
		path string
	}{
		{"get history", "/messages/some-user-id"},
		{"get conversations", "/messages/conversations"},
		{"search", "/messages/search?q=test"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, _ := makeRequest(app, testRequest{
				Method: "GET",
				Path:   tt.path,
				Token:  "",
			})
			assertStatus(t, resp, http.StatusUnauthorized)
		})
	}
}

func TestMessagesHandler_Forward(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	app := fiber.New()
	handler := NewMessagesHandler(nil)

	app.Use(middleware.AuthRequired())
	app.Post("/messages/:id/forward", handler.Forward)

	// Create test users
	user1, token := createTestUser(t, "forwarder", "password123")
	user2, _ := createTestUser(t, "forwardee", "password123")
	user3, _ := createTestUser(t, "forwardtarget", "password123")

	// Create a message to forward
	originalMsg := createTestMessage(t, user1.ID, &user2.ID, nil, "Original message to forward")

	tests := []struct {
		name           string
		messageID      string
		body           map[string]interface{}
		expectedStatus int
		checkResponse  func(t *testing.T, data map[string]interface{})
	}{
		{
			name:      "forward to user",
			messageID: originalMsg.ID,
			body: map[string]interface{}{
				"user_ids": []string{user3.ID},
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, data map[string]interface{}) {
				assertJSONField(t, data, "success", true)
				assertJSONField(t, data, "forwarded_count", float64(1))

				forwardedMsgs := data["forwarded_messages"].([]interface{})
				if len(forwardedMsgs) != 1 {
					t.Errorf("Expected 1 forwarded message, got %d", len(forwardedMsgs))
				}
			},
		},
		{
			name:      "forward to multiple users",
			messageID: originalMsg.ID,
			body: map[string]interface{}{
				"user_ids": []string{user2.ID, user3.ID},
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, data map[string]interface{}) {
				assertJSONField(t, data, "success", true)
				assertJSONField(t, data, "forwarded_count", float64(2))
			},
		},
		{
			name:      "message not found",
			messageID: "non-existent-id",
			body: map[string]interface{}{
				"user_ids": []string{user3.ID},
			},
			expectedStatus: http.StatusNotFound,
		},
		{
			name:      "no recipients",
			messageID: originalMsg.ID,
			body:      map[string]interface{}{},
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, body := makeRequest(app, testRequest{
				Method: "POST",
				Path:   "/messages/" + tt.messageID + "/forward",
				Body:   tt.body,
				Token:  token,
			})

			assertStatus(t, resp, tt.expectedStatus)

			if tt.checkResponse != nil && resp.StatusCode == http.StatusOK {
				data := parseResponse(body)
				tt.checkResponse(t, data)
			}
		})
	}
}

func TestMessagesHandler_Forward_ToGroup(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	app := fiber.New()
	handler := NewMessagesHandler(nil)

	app.Use(middleware.AuthRequired())
	app.Post("/messages/:id/forward", handler.Forward)

	user1, token := createTestUser(t, "groupfwder", "password123")
	user2, _ := createTestUser(t, "groupfwdee", "password123")

	// Create a group and add user1 as member
	group := models.Group{
		Name:      "Forward Test Group",
		CreatedBy: user1.ID,
	}
	database.DB.Create(&group)
	database.DB.Create(&models.GroupMember{
		GroupID: group.ID,
		UserID:  user1.ID,
		Role:    "admin",
	})

	// Create message to forward
	originalMsg := createTestMessage(t, user1.ID, &user2.ID, nil, "Message to forward to group")

	resp, body := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/messages/" + originalMsg.ID + "/forward",
		Body: map[string]interface{}{
			"group_ids": []string{group.ID},
		},
		Token: token,
	})

	assertStatus(t, resp, http.StatusOK)

	data := parseResponse(body)
	assertJSONField(t, data, "success", true)
	assertJSONField(t, data, "forwarded_count", float64(1))

	forwardedMsgs := data["forwarded_messages"].([]interface{})
	fwdMsg := forwardedMsgs[0].(map[string]interface{})
	assertJSONField(t, fwdMsg, "type", "group")
	assertJSONField(t, fwdMsg, "group_id", group.ID)
}

func TestMessagesHandler_Forward_SkipsSelf(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	app := fiber.New()
	handler := NewMessagesHandler(nil)

	app.Use(middleware.AuthRequired())
	app.Post("/messages/:id/forward", handler.Forward)

	user1, token := createTestUser(t, "selfforwarder", "password123")
	user2, _ := createTestUser(t, "selfforwardee", "password123")

	originalMsg := createTestMessage(t, user1.ID, &user2.ID, nil, "Try to forward to self")

	// Try to forward to self and user2
	resp, body := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/messages/" + originalMsg.ID + "/forward",
		Body: map[string]interface{}{
			"user_ids": []string{user1.ID, user2.ID},
		},
		Token: token,
	})

	assertStatus(t, resp, http.StatusOK)

	data := parseResponse(body)
	// Should only forward to user2, not to self
	assertJSONField(t, data, "forwarded_count", float64(1))
}

func TestMessagesHandler_Forward_RespectsBlocking(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	app := fiber.New()
	handler := NewMessagesHandler(nil)

	app.Use(middleware.AuthRequired())
	app.Post("/messages/:id/forward", handler.Forward)

	user1, token := createTestUser(t, "blockfwder", "password123")
	user2, _ := createTestUser(t, "blockfwdee", "password123")
	blockedUser, _ := createTestUser(t, "blockeduser", "password123")

	// User1 blocks blockedUser
	database.DB.Create(&models.Block{
		BlockerID: user1.ID,
		BlockedID: blockedUser.ID,
	})

	originalMsg := createTestMessage(t, user1.ID, &user2.ID, nil, "Try to forward to blocked")

	resp, body := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/messages/" + originalMsg.ID + "/forward",
		Body: map[string]interface{}{
			"user_ids": []string{blockedUser.ID},
		},
		Token: token,
	})

	assertStatus(t, resp, http.StatusOK)

	data := parseResponse(body)
	// Should not forward to blocked user
	assertJSONField(t, data, "forwarded_count", float64(0))

	// Should have error message
	if errors, ok := data["errors"].([]interface{}); ok {
		if len(errors) == 0 {
			t.Error("Expected error about blocked user")
		}
	}
}

func TestMessagesHandler_Forward_AccessDenied(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	app := fiber.New()
	handler := NewMessagesHandler(nil)

	app.Use(middleware.AuthRequired())
	app.Post("/messages/:id/forward", handler.Forward)

	user1, _ := createTestUser(t, "owner", "password123")
	user2, _ := createTestUser(t, "recipient", "password123")
	_, token3 := createTestUser(t, "outsider", "password123")
	user4, _ := createTestUser(t, "target", "password123")

	// Create a message between user1 and user2
	originalMsg := createTestMessage(t, user1.ID, &user2.ID, nil, "Private message")

	// User3 tries to forward a message they don't have access to
	resp, _ := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/messages/" + originalMsg.ID + "/forward",
		Body: map[string]interface{}{
			"user_ids": []string{user4.ID},
		},
		Token: token3,
	})

	assertStatus(t, resp, http.StatusForbidden)
}

func TestMessagesHandler_Forward_ChecksForwardedFrom(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	app := fiber.New()
	handler := NewMessagesHandler(nil)

	app.Use(middleware.AuthRequired())
	app.Post("/messages/:id/forward", handler.Forward)

	// Create sender with display name
	user1, token := createTestUser(t, "sender", "password123")
	database.DB.Model(&models.User{}).Where("id = ?", user1.ID).Update("display_name", "John Doe")

	user2, _ := createTestUser(t, "receiver", "password123")
	user3, _ := createTestUser(t, "forwardtarget2", "password123")

	// Create message from user1
	originalMsg := createTestMessage(t, user1.ID, &user2.ID, nil, "Message with display name")

	resp, body := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/messages/" + originalMsg.ID + "/forward",
		Body: map[string]interface{}{
			"user_ids": []string{user3.ID},
		},
		Token: token,
	})

	assertStatus(t, resp, http.StatusOK)

	// Verify forwarded message has forwarded_from set
	var forwardedMsg models.Message
	database.DB.Where("sender_id = ? AND recipient_id = ? AND forwarded_from IS NOT NULL", user1.ID, user3.ID).First(&forwardedMsg)

	if forwardedMsg.ForwardedFrom == nil {
		t.Error("Expected forwarded_from to be set")
	} else if *forwardedMsg.ForwardedFrom != "John Doe" {
		t.Errorf("Expected forwarded_from to be 'John Doe', got '%s'", *forwardedMsg.ForwardedFrom)
	}

	_ = body // avoid unused variable
}

func TestMessagesHandler_Forward_NotGroupMember(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	app := fiber.New()
	handler := NewMessagesHandler(nil)

	app.Use(middleware.AuthRequired())
	app.Post("/messages/:id/forward", handler.Forward)

	user1, token := createTestUser(t, "nonmember", "password123")
	user2, _ := createTestUser(t, "otheruser", "password123")

	// Create a group that user1 is NOT a member of
	group := models.Group{
		Name:      "Private Group",
		CreatedBy: user2.ID,
	}
	database.DB.Create(&group)
	database.DB.Create(&models.GroupMember{
		GroupID: group.ID,
		UserID:  user2.ID,
		Role:    "admin",
	})

	// Create message to forward
	originalMsg := createTestMessage(t, user1.ID, &user2.ID, nil, "Try to forward to group I'm not in")

	resp, body := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/messages/" + originalMsg.ID + "/forward",
		Body: map[string]interface{}{
			"group_ids": []string{group.ID},
		},
		Token: token,
	})

	assertStatus(t, resp, http.StatusOK)

	data := parseResponse(body)
	// Should fail to forward (not a member)
	assertJSONField(t, data, "forwarded_count", float64(0))

	// Should have error
	if errors, ok := data["errors"].([]interface{}); ok {
		if len(errors) == 0 {
			t.Error("Expected error about not being group member")
		}
	}
}

func TestMessagesHandler_Export_DM(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	app := fiber.New()
	handler := NewMessagesHandler(nil)

	app.Use(middleware.AuthRequired())
	app.Get("/messages/export", handler.Export)

	user1, token := createTestUser(t, "exporter1", "password123")
	user2, _ := createTestUser(t, "exportee1", "password123")

	// Create messages
	createTestMessage(t, user1.ID, &user2.ID, nil, "Hello from user1")
	createTestMessage(t, user2.ID, &user1.ID, nil, "Hello from user2")
	createTestMessage(t, user1.ID, &user2.ID, nil, "Another message")

	resp, body := makeRequest(app, testRequest{
		Method: "GET",
		Path:   "/messages/export?user_id=" + user2.ID,
		Token:  token,
	})

	assertStatus(t, resp, http.StatusOK)

	data := parseResponse(body)
	assertJSONField(t, data, "type", "dm")
	assertJSONField(t, data, "message_count", float64(3))
	assertJSONFieldExists(t, data, "exported_at")
	assertJSONFieldExists(t, data, "participants")
	assertJSONFieldExists(t, data, "messages")

	messages := data["messages"].([]interface{})
	if len(messages) != 3 {
		t.Errorf("Expected 3 messages, got %d", len(messages))
	}

	// Check first message has required fields
	firstMsg := messages[0].(map[string]interface{})
	assertJSONFieldExists(t, firstMsg, "id")
	assertJSONFieldExists(t, firstMsg, "sender_id")
	assertJSONFieldExists(t, firstMsg, "sender_name")
	assertJSONFieldExists(t, firstMsg, "content")
	assertJSONFieldExists(t, firstMsg, "created_at")
}

func TestMessagesHandler_Export_Group(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	app := fiber.New()
	handler := NewMessagesHandler(nil)

	app.Use(middleware.AuthRequired())
	app.Get("/messages/export", handler.Export)

	user1, token := createTestUser(t, "groupexporter", "password123")
	user2, _ := createTestUser(t, "groupmember", "password123")

	// Create group
	group := models.Group{
		Name:      "Export Test Group",
		CreatedBy: user1.ID,
	}
	database.DB.Create(&group)
	database.DB.Create(&models.GroupMember{GroupID: group.ID, UserID: user1.ID, Role: "admin"})
	database.DB.Create(&models.GroupMember{GroupID: group.ID, UserID: user2.ID, Role: "member"})

	// Create group messages
	createTestMessage(t, user1.ID, nil, &group.ID, "Group message 1")
	createTestMessage(t, user2.ID, nil, &group.ID, "Group message 2")

	resp, body := makeRequest(app, testRequest{
		Method: "GET",
		Path:   "/messages/export?group_id=" + group.ID,
		Token:  token,
	})

	assertStatus(t, resp, http.StatusOK)

	data := parseResponse(body)
	assertJSONField(t, data, "type", "group")
	assertJSONField(t, data, "group_name", "Export Test Group")
	assertJSONField(t, data, "message_count", float64(2))

	participants := data["participants"].([]interface{})
	if len(participants) != 2 {
		t.Errorf("Expected 2 participants, got %d", len(participants))
	}
}

func TestMessagesHandler_Export_TextFormat(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	app := fiber.New()
	handler := NewMessagesHandler(nil)

	app.Use(middleware.AuthRequired())
	app.Get("/messages/export", handler.Export)

	user1, token := createTestUser(t, "txtexporter", "password123")
	user2, _ := createTestUser(t, "txtexportee", "password123")

	createTestMessage(t, user1.ID, &user2.ID, nil, "Test message for text export")

	resp, body := makeRequest(app, testRequest{
		Method: "GET",
		Path:   "/messages/export?user_id=" + user2.ID + "&format=txt",
		Token:  token,
	})

	assertStatus(t, resp, http.StatusOK)

	// Check content type
	contentType := resp.Header.Get("Content-Type")
	if contentType != "text/plain; charset=utf-8" {
		t.Errorf("Expected text/plain content type, got %s", contentType)
	}

	// Check content disposition (download)
	contentDisp := resp.Header.Get("Content-Disposition")
	if contentDisp != "attachment; filename=\"chat-export.txt\"" {
		t.Errorf("Expected attachment content disposition, got %s", contentDisp)
	}

	// Check body contains expected text
	bodyStr := string(body)
	if !contains(bodyStr, "=== Chat Export ===") {
		t.Error("Expected text export header")
	}
	if !contains(bodyStr, "Test message for text export") {
		t.Error("Expected message content in export")
	}
}

func TestMessagesHandler_Export_DateRange(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	app := fiber.New()
	handler := NewMessagesHandler(nil)

	app.Use(middleware.AuthRequired())
	app.Get("/messages/export", handler.Export)

	user1, token := createTestUser(t, "dateexporter", "password123")
	user2, _ := createTestUser(t, "dateexportee", "password123")

	// Create messages at different times
	msg1 := createTestMessage(t, user1.ID, &user2.ID, nil, "Old message")
	msg2 := createTestMessage(t, user1.ID, &user2.ID, nil, "New message")

	// Manually update msg1 to be older
	oldDate := time.Now().AddDate(0, -1, 0) // 1 month ago
	database.DB.Model(&msg1).Update("created_at", oldDate)

	// Export with date range (last week only)
	fromDate := time.Now().AddDate(0, 0, -7).Format("2006-01-02")
	resp, body := makeRequest(app, testRequest{
		Method: "GET",
		Path:   "/messages/export?user_id=" + user2.ID + "&from=" + fromDate,
		Token:  token,
	})

	assertStatus(t, resp, http.StatusOK)

	data := parseResponse(body)
	// Should only include the new message
	assertJSONField(t, data, "message_count", float64(1))

	messages := data["messages"].([]interface{})
	firstMsg := messages[0].(map[string]interface{})
	if firstMsg["content"] != "New message" {
		t.Error("Expected only new message to be included")
	}

	_ = msg2 // used implicitly (created in DB)
}

func TestMessagesHandler_Export_NotGroupMember(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	app := fiber.New()
	handler := NewMessagesHandler(nil)

	app.Use(middleware.AuthRequired())
	app.Get("/messages/export", handler.Export)

	_, token := createTestUser(t, "nonmemberexp", "password123")
	user2, _ := createTestUser(t, "groupowner", "password123")

	// Create group without the first user
	group := models.Group{
		Name:      "Private Group",
		CreatedBy: user2.ID,
	}
	database.DB.Create(&group)
	database.DB.Create(&models.GroupMember{GroupID: group.ID, UserID: user2.ID, Role: "admin"})

	resp, _ := makeRequest(app, testRequest{
		Method: "GET",
		Path:   "/messages/export?group_id=" + group.ID,
		Token:  token,
	})

	assertStatus(t, resp, http.StatusForbidden)
}

func TestMessagesHandler_Export_MissingParams(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	app := fiber.New()
	handler := NewMessagesHandler(nil)

	app.Use(middleware.AuthRequired())
	app.Get("/messages/export", handler.Export)

	_, token := createTestUser(t, "paramtest", "password123")

	tests := []struct {
		name           string
		path           string
		expectedStatus int
	}{
		{
			name:           "no user_id or group_id",
			path:           "/messages/export",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "both user_id and group_id",
			path:           "/messages/export?user_id=123&group_id=456",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "non-existent user",
			path:           "/messages/export?user_id=non-existent",
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "non-existent group",
			path:           "/messages/export?group_id=non-existent",
			expectedStatus: http.StatusForbidden, // Can't access group = forbidden
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, _ := makeRequest(app, testRequest{
				Method: "GET",
				Path:   tt.path,
				Token:  token,
			})
			assertStatus(t, resp, tt.expectedStatus)
		})
	}
}

func TestMessagesHandler_Export_ExcludesDeleted(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	app := fiber.New()
	handler := NewMessagesHandler(nil)

	app.Use(middleware.AuthRequired())
	app.Get("/messages/export", handler.Export)

	user1, token := createTestUser(t, "delexporter", "password123")
	user2, _ := createTestUser(t, "delexportee", "password123")

	// Create messages
	msg1 := createTestMessage(t, user1.ID, &user2.ID, nil, "Visible message")
	msg2 := createTestMessage(t, user1.ID, &user2.ID, nil, "Deleted message")

	// Soft delete msg2
	now := time.Now()
	database.DB.Model(&msg2).Update("deleted_at", now)

	resp, body := makeRequest(app, testRequest{
		Method: "GET",
		Path:   "/messages/export?user_id=" + user2.ID,
		Token:  token,
	})

	assertStatus(t, resp, http.StatusOK)

	data := parseResponse(body)
	assertJSONField(t, data, "message_count", float64(1))

	messages := data["messages"].([]interface{})
	firstMsg := messages[0].(map[string]interface{})
	if firstMsg["content"] != "Visible message" {
		t.Error("Expected only non-deleted message")
	}

	_ = msg1 // used implicitly
}

// Helper function for string contains check
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestMessagesHandler_AddReaction(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	app := fiber.New()
	handler := NewMessagesHandler(nil)

	app.Use(middleware.AuthRequired())
	app.Post("/messages/:id/reactions", handler.AddReaction)

	user1, token := createTestUser(t, "reactor1", "password123")
	user2, _ := createTestUser(t, "reactor2", "password123")

	msg := createTestMessage(t, user1.ID, &user2.ID, nil, "React to this!")

	tests := []struct {
		name           string
		messageID      string
		body           map[string]interface{}
		expectedStatus int
	}{
		{
			name:      "add reaction",
			messageID: msg.ID,
			body: map[string]interface{}{
				"emoji": "👍",
			},
			expectedStatus: http.StatusCreated,
		},
		{
			name:      "missing emoji",
			messageID: msg.ID,
			body:      map[string]interface{}{},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:      "message not found",
			messageID: "non-existent",
			body: map[string]interface{}{
				"emoji": "👍",
			},
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, body := makeRequest(app, testRequest{
				Method: "POST",
				Path:   "/messages/" + tt.messageID + "/reactions",
				Body:   tt.body,
				Token:  token,
			})

			assertStatus(t, resp, tt.expectedStatus)

			if tt.expectedStatus == http.StatusCreated {
				data := parseResponse(body)
				assertJSONField(t, data, "emoji", "👍")
				assertJSONField(t, data, "message_id", msg.ID)
				assertJSONFieldExists(t, data, "id")
			}
		})
	}
}

func TestMessagesHandler_AddReaction_UpdateExisting(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	app := fiber.New()
	handler := NewMessagesHandler(nil)

	app.Use(middleware.AuthRequired())
	app.Post("/messages/:id/reactions", handler.AddReaction)

	user1, token := createTestUser(t, "updater1", "password123")
	user2, _ := createTestUser(t, "updater2", "password123")

	msg := createTestMessage(t, user1.ID, &user2.ID, nil, "Change your reaction")

	// Add first reaction
	resp, _ := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/messages/" + msg.ID + "/reactions",
		Body:   map[string]interface{}{"emoji": "👍"},
		Token:  token,
	})
	assertStatus(t, resp, http.StatusCreated)

	// Update to different emoji
	resp, body := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/messages/" + msg.ID + "/reactions",
		Body:   map[string]interface{}{"emoji": "❤️"},
		Token:  token,
	})
	assertStatus(t, resp, http.StatusCreated)

	data := parseResponse(body)
	assertJSONField(t, data, "emoji", "❤️")

	// Verify only one reaction exists
	var count int64
	database.DB.Model(&models.Reaction{}).Where("message_id = ? AND user_id = ?", msg.ID, user1.ID).Count(&count)
	if count != 1 {
		t.Errorf("Expected 1 reaction, got %d", count)
	}
}

func TestMessagesHandler_GetReactions(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	app := fiber.New()
	handler := NewMessagesHandler(nil)

	app.Use(middleware.AuthRequired())
	app.Get("/messages/:id/reactions", handler.GetReactions)
	app.Post("/messages/:id/reactions", handler.AddReaction)

	user1, token1 := createTestUser(t, "getreact1", "password123")
	user2, token2 := createTestUser(t, "getreact2", "password123")

	msg := createTestMessage(t, user1.ID, &user2.ID, nil, "Get reactions for this")

	// Add reactions from both users
	makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/messages/" + msg.ID + "/reactions",
		Body:   map[string]interface{}{"emoji": "👍"},
		Token:  token1,
	})
	makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/messages/" + msg.ID + "/reactions",
		Body:   map[string]interface{}{"emoji": "👍"},
		Token:  token2,
	})

	// Get reactions
	resp, body := makeRequest(app, testRequest{
		Method: "GET",
		Path:   "/messages/" + msg.ID + "/reactions",
		Token:  token1,
	})

	assertStatus(t, resp, http.StatusOK)

	data := parseResponse(body)
	assertJSONField(t, data, "message_id", msg.ID)
	assertJSONFieldExists(t, data, "reactions")

	reactions := data["reactions"].([]interface{})
	if len(reactions) != 1 {
		t.Errorf("Expected 1 emoji type, got %d", len(reactions))
	}

	firstReaction := reactions[0].(map[string]interface{})
	assertJSONField(t, firstReaction, "emoji", "👍")
	assertJSONField(t, firstReaction, "count", float64(2))

	users := firstReaction["users"].([]interface{})
	if len(users) != 2 {
		t.Errorf("Expected 2 users, got %d", len(users))
	}
}

func TestMessagesHandler_RemoveReaction(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	app := fiber.New()
	handler := NewMessagesHandler(nil)

	app.Use(middleware.AuthRequired())
	app.Post("/messages/:id/reactions", handler.AddReaction)
	app.Delete("/messages/:id/reactions", handler.RemoveReaction)

	user1, token := createTestUser(t, "remover1", "password123")
	user2, _ := createTestUser(t, "remover2", "password123")

	msg := createTestMessage(t, user1.ID, &user2.ID, nil, "Remove reaction from this")

	// Add reaction
	makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/messages/" + msg.ID + "/reactions",
		Body:   map[string]interface{}{"emoji": "👍"},
		Token:  token,
	})

	// Verify reaction exists
	var count int64
	database.DB.Model(&models.Reaction{}).Where("message_id = ?", msg.ID).Count(&count)
	if count != 1 {
		t.Errorf("Expected 1 reaction before removal, got %d", count)
	}

	// Remove reaction
	resp, _ := makeRequest(app, testRequest{
		Method: "DELETE",
		Path:   "/messages/" + msg.ID + "/reactions",
		Token:  token,
	})

	assertStatus(t, resp, http.StatusOK)

	// Verify reaction removed
	database.DB.Model(&models.Reaction{}).Where("message_id = ?", msg.ID).Count(&count)
	if count != 0 {
		t.Errorf("Expected 0 reactions after removal, got %d", count)
	}
}

func TestMessagesHandler_Reaction_AccessDenied(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	app := fiber.New()
	handler := NewMessagesHandler(nil)

	app.Use(middleware.AuthRequired())
	app.Get("/messages/:id/reactions", handler.GetReactions)
	app.Post("/messages/:id/reactions", handler.AddReaction)

	user1, _ := createTestUser(t, "owner1", "password123")
	user2, _ := createTestUser(t, "owner2", "password123")
	_, outsiderToken := createTestUser(t, "outsider", "password123")

	// Create message between user1 and user2
	msg := createTestMessage(t, user1.ID, &user2.ID, nil, "Private message")

	// Outsider tries to add reaction
	resp, _ := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/messages/" + msg.ID + "/reactions",
		Body:   map[string]interface{}{"emoji": "👍"},
		Token:  outsiderToken,
	})
	assertStatus(t, resp, http.StatusForbidden)

	// Outsider tries to get reactions
	resp, _ = makeRequest(app, testRequest{
		Method: "GET",
		Path:   "/messages/" + msg.ID + "/reactions",
		Token:  outsiderToken,
	})
	assertStatus(t, resp, http.StatusForbidden)
}

func TestMessagesHandler_Reaction_GroupMessage(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	app := fiber.New()
	handler := NewMessagesHandler(nil)

	app.Use(middleware.AuthRequired())
	app.Post("/messages/:id/reactions", handler.AddReaction)
	app.Get("/messages/:id/reactions", handler.GetReactions)

	user1, token := createTestUser(t, "groupreact1", "password123")
	user2, _ := createTestUser(t, "groupreact2", "password123")

	// Create group
	group := models.Group{
		Name:      "Reaction Test Group",
		CreatedBy: user1.ID,
	}
	database.DB.Create(&group)
	database.DB.Create(&models.GroupMember{GroupID: group.ID, UserID: user1.ID, Role: "admin"})
	database.DB.Create(&models.GroupMember{GroupID: group.ID, UserID: user2.ID, Role: "member"})

	// Create group message
	msg := createTestMessage(t, user1.ID, nil, &group.ID, "Group message to react to")

	// Add reaction
	resp, body := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/messages/" + msg.ID + "/reactions",
		Body:   map[string]interface{}{"emoji": "🎉"},
		Token:  token,
	})

	assertStatus(t, resp, http.StatusCreated)

	data := parseResponse(body)
	assertJSONField(t, data, "emoji", "🎉")

	// Get reactions
	resp, body = makeRequest(app, testRequest{
		Method: "GET",
		Path:   "/messages/" + msg.ID + "/reactions",
		Token:  token,
	})

	assertStatus(t, resp, http.StatusOK)

	data = parseResponse(body)
	reactions := data["reactions"].([]interface{})
	if len(reactions) != 1 {
		t.Errorf("Expected 1 reaction, got %d", len(reactions))
	}
}

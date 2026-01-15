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

// createTestMessage creates a test message in the database
func createTestMessage(t *testing.T, senderID string, recipientID *string, groupID *string, content string) *models.Message {
	msg := &models.Message{
		SenderID:    senderID,
		RecipientID: recipientID,
		GroupID:     groupID,
		Content:     content,
		Status:      models.MessageStatusSent,
	}
	if err := database.DB.Create(msg).Error; err != nil {
		t.Fatalf("Failed to create test message: %v", err)
	}
	return msg
}

func TestReadReceiptHandler_MarkRead(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	hub := websocket.NewHub()
	app := fiber.New()
	handler := NewReadReceiptHandler(hub)

	app.Use(middleware.AuthRequired())
	app.Post("/receipts/read", handler.MarkRead)

	// Create test users
	user1, token := createTestUser(t, "reader", "password123")
	user2, _ := createTestUser(t, "sender", "password123")

	// Create test messages
	msg1 := createTestMessage(t, user2.ID, &user1.ID, nil, "Hello!")
	msg2 := createTestMessage(t, user2.ID, &user1.ID, nil, "How are you?")

	tests := []struct {
		name           string
		body           map[string]interface{}
		token          string
		expectedStatus int
		checkResponse  func(t *testing.T, data map[string]interface{})
	}{
		{
			name: "mark single message as read",
			body: map[string]interface{}{
				"message_ids": []string{msg1.ID},
			},
			token:          token,
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, data map[string]interface{}) {
				assertJSONField(t, data, "success", true)
			},
		},
		{
			name: "mark multiple messages as read",
			body: map[string]interface{}{
				"message_ids": []string{msg1.ID, msg2.ID},
			},
			token:          token,
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, data map[string]interface{}) {
				assertJSONField(t, data, "success", true)
			},
		},
		{
			name: "empty message_ids",
			body: map[string]interface{}{
				"message_ids": []string{},
			},
			token:          token,
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, data map[string]interface{}) {
				assertJSONFieldExists(t, data, "error")
			},
		},
		{
			name:           "missing message_ids",
			body:           map[string]interface{}{},
			token:          token,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "unauthorized",
			body: map[string]interface{}{
				"message_ids": []string{msg1.ID},
			},
			token:          "",
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, body := makeRequest(app, testRequest{
				Method: "POST",
				Path:   "/receipts/read",
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

	// Verify receipts were created in database
	receipts, _ := models.GetReadReceipts(database.DB, msg1.ID)
	if len(receipts) == 0 {
		t.Error("Expected read receipt to be created")
	}
}

func TestReadReceiptHandler_MarkRead_Idempotent(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	hub := websocket.NewHub()
	app := fiber.New()
	handler := NewReadReceiptHandler(hub)

	app.Use(middleware.AuthRequired())
	app.Post("/receipts/read", handler.MarkRead)

	user1, token := createTestUser(t, "idempotent", "password123")
	user2, _ := createTestUser(t, "msgsender", "password123")

	msg := createTestMessage(t, user2.ID, &user1.ID, nil, "Test message")

	// Mark as read twice
	for i := 0; i < 2; i++ {
		resp, _ := makeRequest(app, testRequest{
			Method: "POST",
			Path:   "/receipts/read",
			Body: map[string]interface{}{
				"message_ids": []string{msg.ID},
			},
			Token: token,
		})
		assertStatus(t, resp, http.StatusOK)
	}

	// Should only have one receipt
	receipts, _ := models.GetReadReceipts(database.DB, msg.ID)
	if len(receipts) != 1 {
		t.Errorf("Expected exactly 1 receipt, got %d", len(receipts))
	}
}

func TestReadReceiptHandler_GetReceipts(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	hub := websocket.NewHub()
	app := fiber.New()
	handler := NewReadReceiptHandler(hub)

	app.Use(middleware.AuthRequired())
	app.Post("/receipts/read", handler.MarkRead)
	app.Get("/receipts/:messageId", handler.GetReceipts)

	user1, token1 := createTestUser(t, "reader1", "password123")
	user2, token2 := createTestUser(t, "reader2", "password123")
	user3, _ := createTestUser(t, "sender3", "password123")

	// Create a message
	msg := createTestMessage(t, user3.ID, nil, nil, "Group message")

	// User1 reads the message
	_, _ = makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/receipts/read",
		Body:   map[string]interface{}{"message_ids": []string{msg.ID}},
		Token:  token1,
	})

	// User2 reads the message
	_, _ = makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/receipts/read",
		Body:   map[string]interface{}{"message_ids": []string{msg.ID}},
		Token:  token2,
	})

	// Get receipts
	resp, body := makeRequest(app, testRequest{
		Method: "GET",
		Path:   "/receipts/" + msg.ID,
		Token:  token1,
	})

	assertStatus(t, resp, http.StatusOK)

	data := parseResponse(body)
	receipts := data["receipts"].([]interface{})

	if len(receipts) != 2 {
		t.Errorf("Expected 2 receipts, got %d", len(receipts))
	}

	// Check receipt contains expected fields
	for _, r := range receipts {
		receipt := r.(map[string]interface{})
		assertJSONFieldExists(t, receipt, "user_id")
		assertJSONFieldExists(t, receipt, "username")
		assertJSONFieldExists(t, receipt, "read_at")
	}

	// Verify both users are in the receipts
	userIDs := make(map[string]bool)
	for _, r := range receipts {
		receipt := r.(map[string]interface{})
		userIDs[receipt["user_id"].(string)] = true
	}
	if !userIDs[user1.ID] || !userIDs[user2.ID] {
		t.Error("Expected both users to be in receipts")
	}
}

func TestReadReceiptHandler_GetUnreadCount(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	hub := websocket.NewHub()
	app := fiber.New()
	handler := NewReadReceiptHandler(hub)

	app.Use(middleware.AuthRequired())
	app.Post("/receipts/read", handler.MarkRead)
	app.Get("/receipts/unread", handler.GetUnreadCount)

	user1, token := createTestUser(t, "counter", "password123")
	user2, _ := createTestUser(t, "othersender", "password123")

	// Create some messages from user2 to user1
	msg1 := createTestMessage(t, user2.ID, &user1.ID, nil, "Message 1")
	_ = createTestMessage(t, user2.ID, &user1.ID, nil, "Message 2")
	_ = createTestMessage(t, user2.ID, &user1.ID, nil, "Message 3")

	// Initially should have 3 unread
	resp1, body1 := makeRequest(app, testRequest{
		Method: "GET",
		Path:   "/receipts/unread?other_user_id=" + user2.ID,
		Token:  token,
	})

	assertStatus(t, resp1, http.StatusOK)
	data1 := parseResponse(body1)

	count1 := int(data1["unread_count"].(float64))
	if count1 != 3 {
		t.Errorf("Expected 3 unread messages, got %d", count1)
	}

	// Read one message
	_, _ = makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/receipts/read",
		Body:   map[string]interface{}{"message_ids": []string{msg1.ID}},
		Token:  token,
	})

	// Now should have 2 unread
	resp2, body2 := makeRequest(app, testRequest{
		Method: "GET",
		Path:   "/receipts/unread?other_user_id=" + user2.ID,
		Token:  token,
	})

	assertStatus(t, resp2, http.StatusOK)
	data2 := parseResponse(body2)

	count2 := int(data2["unread_count"].(float64))
	if count2 != 2 {
		t.Errorf("Expected 2 unread messages after reading one, got %d", count2)
	}
}

func TestReadReceiptHandler_GetUnreadCount_Group(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	hub := websocket.NewHub()
	app := fiber.New()
	handler := NewReadReceiptHandler(hub)

	app.Use(middleware.AuthRequired())
	app.Post("/receipts/read", handler.MarkRead)
	app.Get("/receipts/unread", handler.GetUnreadCount)

	user1, token := createTestUser(t, "groupcounter", "password123")
	user2, _ := createTestUser(t, "groupsender", "password123")

	// Create a group
	group := models.Group{
		Name:      "Test Group",
		CreatedBy: user1.ID,
	}
	database.DB.Create(&group)

	// Create messages in the group
	_ = createTestMessage(t, user2.ID, nil, &group.ID, "Group msg 1")
	_ = createTestMessage(t, user2.ID, nil, &group.ID, "Group msg 2")

	// Get unread count for group
	resp, body := makeRequest(app, testRequest{
		Method: "GET",
		Path:   "/receipts/unread?group_id=" + group.ID,
		Token:  token,
	})

	assertStatus(t, resp, http.StatusOK)
	data := parseResponse(body)

	count := int(data["unread_count"].(float64))
	if count != 2 {
		t.Errorf("Expected 2 unread group messages, got %d", count)
	}
}

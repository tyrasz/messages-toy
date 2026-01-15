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

func TestBroadcastHandler_Create(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	hub := websocket.NewHub()
	app := fiber.New()
	handler := NewBroadcastHandler(hub)

	app.Use(middleware.AuthRequired())
	app.Post("/broadcast", handler.Create)

	user, token := createTestUser(t, "broadcaster", "password123")
	recipient1, _ := createTestUser(t, "recipient1", "password123")
	recipient2, _ := createTestUser(t, "recipient2", "password123")

	_ = user // avoid unused

	tests := []struct {
		name           string
		body           map[string]interface{}
		token          string
		expectedStatus int
		checkResponse  func(t *testing.T, data map[string]interface{})
	}{
		{
			name: "create broadcast list",
			body: map[string]interface{}{
				"name":          "My Broadcast",
				"recipient_ids": []string{recipient1.ID, recipient2.ID},
			},
			token:          token,
			expectedStatus: http.StatusCreated,
			checkResponse: func(t *testing.T, data map[string]interface{}) {
				assertJSONFieldExists(t, data, "id")
				assertJSONField(t, data, "name", "My Broadcast")
				assertJSONField(t, data, "recipient_count", float64(2))
			},
		},
		{
			name: "missing name",
			body: map[string]interface{}{
				"recipient_ids": []string{recipient1.ID},
			},
			token:          token,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "missing recipients",
			body: map[string]interface{}{
				"name": "Empty List",
			},
			token:          token,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "unauthorized",
			body: map[string]interface{}{
				"name":          "Test",
				"recipient_ids": []string{recipient1.ID},
			},
			token:          "",
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, body := makeRequest(app, testRequest{
				Method: "POST",
				Path:   "/broadcast",
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

func TestBroadcastHandler_Create_ExcludesSelf(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	hub := websocket.NewHub()
	app := fiber.New()
	handler := NewBroadcastHandler(hub)

	app.Use(middleware.AuthRequired())
	app.Post("/broadcast", handler.Create)

	user, token := createTestUser(t, "selfbroadcast", "password123")
	other, _ := createTestUser(t, "otheruser", "password123")

	// Try to include self in recipients
	resp, body := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/broadcast",
		Body: map[string]interface{}{
			"name":          "Self Include Test",
			"recipient_ids": []string{user.ID, other.ID},
		},
		Token: token,
	})

	assertStatus(t, resp, http.StatusCreated)

	data := parseResponse(body)
	// Should only have 1 recipient (other, not self)
	assertJSONField(t, data, "recipient_count", float64(1))
}

func TestBroadcastHandler_Create_ExcludesBlocked(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	hub := websocket.NewHub()
	app := fiber.New()
	handler := NewBroadcastHandler(hub)

	app.Use(middleware.AuthRequired())
	app.Post("/broadcast", handler.Create)

	user, token := createTestUser(t, "blockerbroadcast", "password123")
	blocked, _ := createTestUser(t, "blockedbroadcast", "password123")
	other, _ := createTestUser(t, "otherbroadcast", "password123")

	// Block one user
	database.DB.Create(&models.Block{BlockerID: user.ID, BlockedID: blocked.ID})

	// Try to include blocked user in recipients
	resp, body := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/broadcast",
		Body: map[string]interface{}{
			"name":          "Block Test",
			"recipient_ids": []string{blocked.ID, other.ID},
		},
		Token: token,
	})

	assertStatus(t, resp, http.StatusCreated)

	data := parseResponse(body)
	// Should only have 1 recipient (other, not blocked)
	assertJSONField(t, data, "recipient_count", float64(1))
}

func TestBroadcastHandler_List(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	hub := websocket.NewHub()
	app := fiber.New()
	handler := NewBroadcastHandler(hub)

	app.Use(middleware.AuthRequired())
	app.Post("/broadcast", handler.Create)
	app.Get("/broadcast", handler.List)

	_, token := createTestUser(t, "listbroadcast", "password123")
	r1, _ := createTestUser(t, "listr1", "password123")
	r2, _ := createTestUser(t, "listr2", "password123")

	// Create some broadcast lists
	for i := 0; i < 3; i++ {
		_, _ = makeRequest(app, testRequest{
			Method: "POST",
			Path:   "/broadcast",
			Body: map[string]interface{}{
				"name":          "List " + string(rune('A'+i)),
				"recipient_ids": []string{r1.ID, r2.ID},
			},
			Token: token,
		})
	}

	// List
	resp, body := makeRequest(app, testRequest{
		Method: "GET",
		Path:   "/broadcast",
		Token:  token,
	})

	assertStatus(t, resp, http.StatusOK)

	data := parseResponse(body)
	lists := data["broadcast_lists"].([]interface{})

	if len(lists) != 3 {
		t.Errorf("Expected 3 broadcast lists, got %d", len(lists))
	}
}

func TestBroadcastHandler_Get(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	hub := websocket.NewHub()
	app := fiber.New()
	handler := NewBroadcastHandler(hub)

	app.Use(middleware.AuthRequired())
	app.Post("/broadcast", handler.Create)
	app.Get("/broadcast/:id", handler.Get)

	_, token := createTestUser(t, "getbroadcast", "password123")
	r1, _ := createTestUser(t, "getr1", "password123")

	// Create a list
	resp1, body1 := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/broadcast",
		Body: map[string]interface{}{
			"name":          "Get Test",
			"recipient_ids": []string{r1.ID},
		},
		Token: token,
	})
	assertStatus(t, resp1, http.StatusCreated)

	data1 := parseResponse(body1)
	listID := data1["id"].(string)

	// Get the list
	resp2, body2 := makeRequest(app, testRequest{
		Method: "GET",
		Path:   "/broadcast/" + listID,
		Token:  token,
	})

	assertStatus(t, resp2, http.StatusOK)

	data2 := parseResponse(body2)
	assertJSONField(t, data2, "id", listID)
	assertJSONField(t, data2, "name", "Get Test")
}

func TestBroadcastHandler_Get_NotFound(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	hub := websocket.NewHub()
	app := fiber.New()
	handler := NewBroadcastHandler(hub)

	app.Use(middleware.AuthRequired())
	app.Get("/broadcast/:id", handler.Get)

	_, token := createTestUser(t, "notfoundbroadcast", "password123")

	resp, _ := makeRequest(app, testRequest{
		Method: "GET",
		Path:   "/broadcast/nonexistent-id",
		Token:  token,
	})

	assertStatus(t, resp, http.StatusNotFound)
}

func TestBroadcastHandler_Delete(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	hub := websocket.NewHub()
	app := fiber.New()
	handler := NewBroadcastHandler(hub)

	app.Use(middleware.AuthRequired())
	app.Post("/broadcast", handler.Create)
	app.Delete("/broadcast/:id", handler.Delete)

	_, token := createTestUser(t, "deletebroadcast", "password123")
	r1, _ := createTestUser(t, "deleter1", "password123")

	// Create a list
	resp1, body1 := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/broadcast",
		Body: map[string]interface{}{
			"name":          "Delete Test",
			"recipient_ids": []string{r1.ID},
		},
		Token: token,
	})
	assertStatus(t, resp1, http.StatusCreated)

	data1 := parseResponse(body1)
	listID := data1["id"].(string)

	// Delete the list
	resp2, body2 := makeRequest(app, testRequest{
		Method: "DELETE",
		Path:   "/broadcast/" + listID,
		Token:  token,
	})

	assertStatus(t, resp2, http.StatusOK)

	data2 := parseResponse(body2)
	assertJSONFieldExists(t, data2, "message")

	// Verify deleted
	var count int64
	database.DB.Model(&models.BroadcastList{}).Where("id = ?", listID).Count(&count)
	if count != 0 {
		t.Error("Expected broadcast list to be deleted")
	}
}

func TestBroadcastHandler_Update(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	hub := websocket.NewHub()
	app := fiber.New()
	handler := NewBroadcastHandler(hub)

	app.Use(middleware.AuthRequired())
	app.Post("/broadcast", handler.Create)
	app.Put("/broadcast/:id", handler.Update)

	_, token := createTestUser(t, "updatebroadcast", "password123")
	r1, _ := createTestUser(t, "updater1", "password123")

	// Create a list
	resp1, body1 := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/broadcast",
		Body: map[string]interface{}{
			"name":          "Original Name",
			"recipient_ids": []string{r1.ID},
		},
		Token: token,
	})
	assertStatus(t, resp1, http.StatusCreated)

	data1 := parseResponse(body1)
	listID := data1["id"].(string)

	// Update the name
	resp2, body2 := makeRequest(app, testRequest{
		Method: "PUT",
		Path:   "/broadcast/" + listID,
		Body: map[string]interface{}{
			"name": "Updated Name",
		},
		Token: token,
	})

	assertStatus(t, resp2, http.StatusOK)

	data2 := parseResponse(body2)
	assertJSONField(t, data2, "name", "Updated Name")
}

func TestBroadcastHandler_AddRecipient(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	hub := websocket.NewHub()
	app := fiber.New()
	handler := NewBroadcastHandler(hub)

	app.Use(middleware.AuthRequired())
	app.Post("/broadcast", handler.Create)
	app.Post("/broadcast/:id/recipients", handler.AddRecipient)

	_, token := createTestUser(t, "addrecipient", "password123")
	r1, _ := createTestUser(t, "addr1", "password123")
	r2, _ := createTestUser(t, "addr2", "password123")

	// Create a list with one recipient
	resp1, body1 := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/broadcast",
		Body: map[string]interface{}{
			"name":          "Add Recipient Test",
			"recipient_ids": []string{r1.ID},
		},
		Token: token,
	})
	assertStatus(t, resp1, http.StatusCreated)

	data1 := parseResponse(body1)
	listID := data1["id"].(string)

	// Add another recipient
	resp2, body2 := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/broadcast/" + listID + "/recipients",
		Body: map[string]interface{}{
			"recipient_id": r2.ID,
		},
		Token: token,
	})

	assertStatus(t, resp2, http.StatusCreated)

	data2 := parseResponse(body2)
	assertJSONField(t, data2, "recipient_count", float64(2))
}

func TestBroadcastHandler_RemoveRecipient(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	hub := websocket.NewHub()
	app := fiber.New()
	handler := NewBroadcastHandler(hub)

	app.Use(middleware.AuthRequired())
	app.Post("/broadcast", handler.Create)
	app.Delete("/broadcast/:id/recipients/:recipientId", handler.RemoveRecipient)

	_, token := createTestUser(t, "removerecipient", "password123")
	r1, _ := createTestUser(t, "remover1", "password123")
	r2, _ := createTestUser(t, "remover2", "password123")

	// Create a list with two recipients
	resp1, body1 := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/broadcast",
		Body: map[string]interface{}{
			"name":          "Remove Recipient Test",
			"recipient_ids": []string{r1.ID, r2.ID},
		},
		Token: token,
	})
	assertStatus(t, resp1, http.StatusCreated)

	data1 := parseResponse(body1)
	listID := data1["id"].(string)

	// Remove one recipient
	resp2, body2 := makeRequest(app, testRequest{
		Method: "DELETE",
		Path:   "/broadcast/" + listID + "/recipients/" + r2.ID,
		Token:  token,
	})

	assertStatus(t, resp2, http.StatusOK)

	data2 := parseResponse(body2)
	assertJSONField(t, data2, "recipient_count", float64(1))
}

func TestBroadcastHandler_Send(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	hub := websocket.NewHub()
	app := fiber.New()
	handler := NewBroadcastHandler(hub)

	app.Use(middleware.AuthRequired())
	app.Post("/broadcast", handler.Create)
	app.Post("/broadcast/:id/send", handler.Send)

	_, token := createTestUser(t, "sendbroadcast", "password123")
	r1, _ := createTestUser(t, "sendr1", "password123")
	r2, _ := createTestUser(t, "sendr2", "password123")

	// Create a list
	resp1, body1 := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/broadcast",
		Body: map[string]interface{}{
			"name":          "Send Test",
			"recipient_ids": []string{r1.ID, r2.ID},
		},
		Token: token,
	})
	assertStatus(t, resp1, http.StatusCreated)

	data1 := parseResponse(body1)
	listID := data1["id"].(string)

	// Send a broadcast message
	resp2, body2 := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/broadcast/" + listID + "/send",
		Body: map[string]interface{}{
			"content": "Hello broadcast recipients!",
		},
		Token: token,
	})

	assertStatus(t, resp2, http.StatusOK)

	data2 := parseResponse(body2)
	assertJSONField(t, data2, "success", true)
	assertJSONField(t, data2, "messages_sent", float64(2))

	messageIDs := data2["message_ids"].([]interface{})
	if len(messageIDs) != 2 {
		t.Errorf("Expected 2 message IDs, got %d", len(messageIDs))
	}

	// Verify messages in database
	var count int64
	database.DB.Model(&models.Message{}).Where("content = ?", "Hello broadcast recipients!").Count(&count)
	if count != 2 {
		t.Errorf("Expected 2 messages in database, got %d", count)
	}
}

func TestBroadcastHandler_Send_ExcludesBlocked(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	hub := websocket.NewHub()
	app := fiber.New()
	handler := NewBroadcastHandler(hub)

	app.Use(middleware.AuthRequired())
	app.Post("/broadcast", handler.Create)
	app.Post("/broadcast/:id/send", handler.Send)

	user, token := createTestUser(t, "blocksendbroadcast", "password123")
	r1, _ := createTestUser(t, "blocksendr1", "password123")
	r2, _ := createTestUser(t, "blocksendr2", "password123")

	// Create a list with both recipients
	resp1, body1 := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/broadcast",
		Body: map[string]interface{}{
			"name":          "Block Send Test",
			"recipient_ids": []string{r1.ID, r2.ID},
		},
		Token: token,
	})
	assertStatus(t, resp1, http.StatusCreated)

	data1 := parseResponse(body1)
	listID := data1["id"].(string)

	// Block r2 after creating the list
	database.DB.Create(&models.Block{BlockerID: user.ID, BlockedID: r2.ID})

	// Send a broadcast message
	resp2, body2 := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/broadcast/" + listID + "/send",
		Body: map[string]interface{}{
			"content": "Only r1 should get this",
		},
		Token: token,
	})

	assertStatus(t, resp2, http.StatusOK)

	data2 := parseResponse(body2)
	// Should only send to 1 recipient (r1)
	assertJSONField(t, data2, "messages_sent", float64(1))
}

func TestBroadcastHandler_Send_EmptyContent(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	hub := websocket.NewHub()
	app := fiber.New()
	handler := NewBroadcastHandler(hub)

	app.Use(middleware.AuthRequired())
	app.Post("/broadcast", handler.Create)
	app.Post("/broadcast/:id/send", handler.Send)

	_, token := createTestUser(t, "emptybroadcast", "password123")
	r1, _ := createTestUser(t, "emptyr1", "password123")

	// Create a list
	resp1, body1 := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/broadcast",
		Body: map[string]interface{}{
			"name":          "Empty Content Test",
			"recipient_ids": []string{r1.ID},
		},
		Token: token,
	})
	assertStatus(t, resp1, http.StatusCreated)

	data1 := parseResponse(body1)
	listID := data1["id"].(string)

	// Try to send with empty content
	resp2, _ := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/broadcast/" + listID + "/send",
		Body:   map[string]interface{}{},
		Token:  token,
	})

	assertStatus(t, resp2, http.StatusBadRequest)
}

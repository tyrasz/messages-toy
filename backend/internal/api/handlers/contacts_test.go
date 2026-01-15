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

func TestContactsHandler_List(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	hub := websocket.NewHub()
	app := fiber.New()
	handler := NewContactsHandler(hub)

	app.Use(middleware.AuthRequired())
	app.Get("/contacts", handler.List)

	// Create test users
	user1, token := createTestUser(t, "contactlister", "password123")
	user2, _ := createTestUser(t, "contact1", "password123")
	user3, _ := createTestUser(t, "contact2", "password123")

	// Create contacts
	database.DB.Create(&models.Contact{
		UserID:    user1.ID,
		ContactID: user2.ID,
		Nickname:  "Friend 1",
	})
	database.DB.Create(&models.Contact{
		UserID:    user1.ID,
		ContactID: user3.ID,
	})

	resp, body := makeRequest(app, testRequest{
		Method: "GET",
		Path:   "/contacts",
		Token:  token,
	})

	assertStatus(t, resp, http.StatusOK)

	data := parseResponse(body)
	contacts := data["contacts"].([]interface{})

	if len(contacts) != 2 {
		t.Errorf("Expected 2 contacts, got %d", len(contacts))
	}

	// Check that contacts have expected fields
	for _, c := range contacts {
		contact := c.(map[string]interface{})
		assertJSONFieldExists(t, contact, "id")
		assertJSONFieldExists(t, contact, "contact_id")
		assertJSONFieldExists(t, contact, "user")
		assertJSONFieldExists(t, contact, "created_at")
	}
}

func TestContactsHandler_List_Empty(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	hub := websocket.NewHub()
	app := fiber.New()
	handler := NewContactsHandler(hub)

	app.Use(middleware.AuthRequired())
	app.Get("/contacts", handler.List)

	_, token := createTestUser(t, "nocontacts", "password123")

	resp, body := makeRequest(app, testRequest{
		Method: "GET",
		Path:   "/contacts",
		Token:  token,
	})

	assertStatus(t, resp, http.StatusOK)

	data := parseResponse(body)
	contacts := data["contacts"].([]interface{})

	if len(contacts) != 0 {
		t.Errorf("Expected 0 contacts, got %d", len(contacts))
	}
}

func TestContactsHandler_Add(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	hub := websocket.NewHub()
	app := fiber.New()
	handler := NewContactsHandler(hub)

	app.Use(middleware.AuthRequired())
	app.Post("/contacts", handler.Add)

	_, token := createTestUser(t, "adder", "password123")
	user2, _ := createTestUser(t, "addee", "password123")

	tests := []struct {
		name           string
		body           map[string]interface{}
		token          string
		expectedStatus int
		checkResponse  func(t *testing.T, data map[string]interface{})
	}{
		{
			name: "add contact by username",
			body: map[string]interface{}{
				"username": "addee",
			},
			token:          token,
			expectedStatus: http.StatusCreated,
			checkResponse: func(t *testing.T, data map[string]interface{}) {
				assertJSONFieldExists(t, data, "id")
				assertJSONField(t, data, "contact_id", user2.ID)
				assertJSONFieldExists(t, data, "user")
			},
		},
		{
			name: "add contact with nickname",
			body: map[string]interface{}{
				"username": "addee",
				"nickname": "Best Friend",
			},
			token:          token,
			expectedStatus: http.StatusConflict, // Already added
		},
		{
			name: "add non-existent user",
			body: map[string]interface{}{
				"username": "nonexistent",
			},
			token:          token,
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "missing username",
			body:           map[string]interface{}{},
			token:          token,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "unauthorized",
			body: map[string]interface{}{
				"username": "addee",
			},
			token:          "",
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, body := makeRequest(app, testRequest{
				Method: "POST",
				Path:   "/contacts",
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

func TestContactsHandler_Add_Self(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	hub := websocket.NewHub()
	app := fiber.New()
	handler := NewContactsHandler(hub)

	app.Use(middleware.AuthRequired())
	app.Post("/contacts", handler.Add)

	_, token := createTestUser(t, "selfadder", "password123")

	resp, body := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/contacts",
		Body: map[string]interface{}{
			"username": "selfadder",
		},
		Token: token,
	})

	assertStatus(t, resp, http.StatusBadRequest)

	data := parseResponse(body)
	assertJSONFieldExists(t, data, "error")
}

func TestContactsHandler_Remove(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	hub := websocket.NewHub()
	app := fiber.New()
	handler := NewContactsHandler(hub)

	app.Use(middleware.AuthRequired())
	app.Post("/contacts", handler.Add)
	app.Delete("/contacts/:id", handler.Remove)

	user1, token := createTestUser(t, "remover", "password123")
	_, _ = createTestUser(t, "removee", "password123")

	// Add contact first
	resp1, body1 := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/contacts",
		Body:   map[string]interface{}{"username": "removee"},
		Token:  token,
	})
	assertStatus(t, resp1, http.StatusCreated)

	data := parseResponse(body1)
	contactID := data["id"].(string)

	// Remove contact
	resp2, body2 := makeRequest(app, testRequest{
		Method: "DELETE",
		Path:   "/contacts/" + contactID,
		Token:  token,
	})

	assertStatus(t, resp2, http.StatusOK)

	data2 := parseResponse(body2)
	assertJSONFieldExists(t, data2, "message")

	// Verify removed from database
	var count int64
	database.DB.Model(&models.Contact{}).Where("user_id = ?", user1.ID).Count(&count)
	if count != 0 {
		t.Error("Expected contact to be removed from database")
	}
}

func TestContactsHandler_Remove_NotFound(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	hub := websocket.NewHub()
	app := fiber.New()
	handler := NewContactsHandler(hub)

	app.Use(middleware.AuthRequired())
	app.Delete("/contacts/:id", handler.Remove)

	_, token := createTestUser(t, "removenotfound", "password123")

	resp, body := makeRequest(app, testRequest{
		Method: "DELETE",
		Path:   "/contacts/nonexistent-id",
		Token:  token,
	})

	assertStatus(t, resp, http.StatusNotFound)

	data := parseResponse(body)
	assertJSONFieldExists(t, data, "error")
}

func TestContactsHandler_Block(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	hub := websocket.NewHub()
	app := fiber.New()
	handler := NewContactsHandler(hub)

	app.Use(middleware.AuthRequired())
	app.Post("/blocks/:userId", handler.Block)

	_, token := createTestUser(t, "blocker", "password123")
	user2, _ := createTestUser(t, "blockee", "password123")

	tests := []struct {
		name           string
		userID         string
		token          string
		expectedStatus int
		checkResponse  func(t *testing.T, data map[string]interface{})
	}{
		{
			name:           "block user",
			userID:         user2.ID,
			token:          token,
			expectedStatus: http.StatusCreated,
			checkResponse: func(t *testing.T, data map[string]interface{}) {
				assertJSONFieldExists(t, data, "id")
				assertJSONField(t, data, "blocked_id", user2.ID)
			},
		},
		{
			name:           "block already blocked user",
			userID:         user2.ID,
			token:          token,
			expectedStatus: http.StatusConflict,
		},
		{
			name:           "block non-existent user",
			userID:         "nonexistent-id",
			token:          token,
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, body := makeRequest(app, testRequest{
				Method: "POST",
				Path:   "/blocks/" + tt.userID,
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

func TestContactsHandler_Block_Self(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	hub := websocket.NewHub()
	app := fiber.New()
	handler := NewContactsHandler(hub)

	app.Use(middleware.AuthRequired())
	app.Post("/blocks/:userId", handler.Block)

	user, token := createTestUser(t, "selfblocker", "password123")

	resp, body := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/blocks/" + user.ID,
		Token:  token,
	})

	assertStatus(t, resp, http.StatusBadRequest)

	data := parseResponse(body)
	assertJSONFieldExists(t, data, "error")
}

func TestContactsHandler_Unblock(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	hub := websocket.NewHub()
	app := fiber.New()
	handler := NewContactsHandler(hub)

	app.Use(middleware.AuthRequired())
	app.Post("/blocks/:userId", handler.Block)
	app.Delete("/blocks/:userId", handler.Unblock)

	user1, token := createTestUser(t, "unblocker", "password123")
	user2, _ := createTestUser(t, "unblockee", "password123")

	// Block first
	_, _ = makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/blocks/" + user2.ID,
		Token:  token,
	})

	// Unblock
	resp, body := makeRequest(app, testRequest{
		Method: "DELETE",
		Path:   "/blocks/" + user2.ID,
		Token:  token,
	})

	assertStatus(t, resp, http.StatusOK)

	data := parseResponse(body)
	assertJSONFieldExists(t, data, "message")

	// Verify removed from database
	blocked := models.IsBlocked(database.DB, user1.ID, user2.ID)
	if blocked {
		t.Error("Expected block to be removed from database")
	}
}

func TestContactsHandler_Unblock_NotFound(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	hub := websocket.NewHub()
	app := fiber.New()
	handler := NewContactsHandler(hub)

	app.Use(middleware.AuthRequired())
	app.Delete("/blocks/:userId", handler.Unblock)

	_, token := createTestUser(t, "unblocknotfound", "password123")
	user2, _ := createTestUser(t, "neverblocked", "password123")

	resp, body := makeRequest(app, testRequest{
		Method: "DELETE",
		Path:   "/blocks/" + user2.ID,
		Token:  token,
	})

	assertStatus(t, resp, http.StatusNotFound)

	data := parseResponse(body)
	assertJSONFieldExists(t, data, "error")
}

func TestContactsHandler_ListBlocked(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	hub := websocket.NewHub()
	app := fiber.New()
	handler := NewContactsHandler(hub)

	app.Use(middleware.AuthRequired())
	app.Post("/blocks/:userId", handler.Block)
	app.Get("/blocks", handler.ListBlocked)

	_, token := createTestUser(t, "blocklister", "password123")
	user2, _ := createTestUser(t, "blocked1", "password123")
	user3, _ := createTestUser(t, "blocked2", "password123")

	// Block some users
	_, _ = makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/blocks/" + user2.ID,
		Token:  token,
	})
	_, _ = makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/blocks/" + user3.ID,
		Token:  token,
	})

	// List blocked
	resp, body := makeRequest(app, testRequest{
		Method: "GET",
		Path:   "/blocks",
		Token:  token,
	})

	assertStatus(t, resp, http.StatusOK)

	data := parseResponse(body)
	blocked := data["blocked"].([]interface{})

	if len(blocked) != 2 {
		t.Errorf("Expected 2 blocked users, got %d", len(blocked))
	}
}

func TestContactsHandler_IsBlocked(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	hub := websocket.NewHub()
	app := fiber.New()
	handler := NewContactsHandler(hub)

	app.Use(middleware.AuthRequired())
	app.Post("/blocks/:userId", handler.Block)
	app.Get("/blocks/:userId", handler.IsBlocked)

	_, token := createTestUser(t, "isblocked", "password123")
	user2, _ := createTestUser(t, "checkeduser", "password123")

	// Check before blocking
	resp1, body1 := makeRequest(app, testRequest{
		Method: "GET",
		Path:   "/blocks/" + user2.ID,
		Token:  token,
	})

	assertStatus(t, resp1, http.StatusOK)
	data1 := parseResponse(body1)
	assertJSONField(t, data1, "blocked", false)

	// Block
	_, _ = makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/blocks/" + user2.ID,
		Token:  token,
	})

	// Check after blocking
	resp2, body2 := makeRequest(app, testRequest{
		Method: "GET",
		Path:   "/blocks/" + user2.ID,
		Token:  token,
	})

	assertStatus(t, resp2, http.StatusOK)
	data2 := parseResponse(body2)
	assertJSONField(t, data2, "blocked", true)
}

func TestContactsHandler_SearchUsers(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	hub := websocket.NewHub()
	app := fiber.New()
	handler := NewContactsHandler(hub)

	app.Use(middleware.AuthRequired())
	app.Get("/users/search", handler.SearchUsers)

	user1, token := createTestUser(t, "searcher", "password123")
	user2, _ := createTestUser(t, "johnsmith", "password123")
	user3, _ := createTestUser(t, "johndoe", "password123")
	user4, _ := createTestUser(t, "janedoe", "password123")

	// Set display names
	database.DB.Model(&user3).Update("display_name", "Johnny D")

	// Add user2 as contact
	database.DB.Create(&models.Contact{UserID: user1.ID, ContactID: user2.ID})

	tests := []struct {
		name           string
		query          string
		expectedStatus int
		checkResponse  func(t *testing.T, data map[string]interface{})
	}{
		{
			name:           "search by username prefix",
			query:          "john",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, data map[string]interface{}) {
				users := data["users"].([]interface{})
				if len(users) != 2 {
					t.Errorf("Expected 2 users matching 'john', got %d", len(users))
				}
			},
		},
		{
			name:           "search by display name",
			query:          "Johnny",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, data map[string]interface{}) {
				users := data["users"].([]interface{})
				if len(users) != 1 {
					t.Errorf("Expected 1 user matching 'Johnny', got %d", len(users))
				}
				if len(users) > 0 {
					item := users[0].(map[string]interface{})
					assertJSONFieldExists(t, item, "is_contact")
				}
			},
		},
		{
			name:           "search marks contacts",
			query:          "johnsmith",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, data map[string]interface{}) {
				users := data["users"].([]interface{})
				if len(users) == 1 {
					item := users[0].(map[string]interface{})
					assertJSONField(t, item, "is_contact", true)
				}
			},
		},
		{
			name:           "search no results",
			query:          "zzzzz",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, data map[string]interface{}) {
				users := data["users"].([]interface{})
				if len(users) != 0 {
					t.Errorf("Expected 0 users, got %d", len(users))
				}
			},
		},
		{
			name:           "query too short",
			query:          "j",
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, body := makeRequest(app, testRequest{
				Method: "GET",
				Path:   "/users/search?q=" + tt.query,
				Token:  token,
			})

			assertStatus(t, resp, tt.expectedStatus)

			if tt.checkResponse != nil {
				data := parseResponse(body)
				tt.checkResponse(t, data)
			}
		})
	}

	// Avoid unused warnings
	_ = user4
}

func TestContactsHandler_SearchUsers_ExcludesBlocked(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	hub := websocket.NewHub()
	app := fiber.New()
	handler := NewContactsHandler(hub)

	app.Use(middleware.AuthRequired())
	app.Get("/users/search", handler.SearchUsers)

	user1, token := createTestUser(t, "searchblocker", "password123")
	user2, _ := createTestUser(t, "blockeduser", "password123")

	// Block user2
	database.DB.Create(&models.Block{BlockerID: user1.ID, BlockedID: user2.ID})

	resp, body := makeRequest(app, testRequest{
		Method: "GET",
		Path:   "/users/search?q=blockeduser",
		Token:  token,
	})

	assertStatus(t, resp, http.StatusOK)

	data := parseResponse(body)
	users := data["users"].([]interface{})
	if len(users) != 0 {
		t.Error("Expected blocked user to be excluded from search")
	}
}

func TestContactsHandler_SearchUsers_Pagination(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	hub := websocket.NewHub()
	app := fiber.New()
	handler := NewContactsHandler(hub)

	app.Use(middleware.AuthRequired())
	app.Get("/users/search", handler.SearchUsers)

	_, token := createTestUser(t, "pagesearcher", "password123")

	// Create multiple users with same prefix
	for i := 0; i < 10; i++ {
		createTestUser(t, "testuser"+string(rune('a'+i)), "password123")
	}

	resp, body := makeRequest(app, testRequest{
		Method: "GET",
		Path:   "/users/search?q=testuser&limit=5&offset=0",
		Token:  token,
	})

	assertStatus(t, resp, http.StatusOK)

	data := parseResponse(body)
	users := data["users"].([]interface{})
	if len(users) != 5 {
		t.Errorf("Expected 5 users with limit=5, got %d", len(users))
	}
	assertJSONField(t, data, "limit", float64(5))
	assertJSONField(t, data, "offset", float64(0))
	// Total should be 10
	if total := int(data["total"].(float64)); total != 10 {
		t.Errorf("Expected total=10, got %d", total)
	}
}

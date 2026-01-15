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

func TestPollHandler_Create(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	hub := websocket.NewHub()
	app := fiber.New()
	handler := NewPollHandler(hub)

	app.Use(middleware.AuthRequired())
	app.Post("/polls", handler.Create)

	_, token := createTestUser(t, "pollcreator", "password123")

	tests := []struct {
		name           string
		body           map[string]interface{}
		token          string
		expectedStatus int
		checkResponse  func(t *testing.T, data map[string]interface{})
	}{
		{
			name: "create basic poll",
			body: map[string]interface{}{
				"question": "What's your favorite color?",
				"options":  []string{"Red", "Blue", "Green"},
			},
			token:          token,
			expectedStatus: http.StatusCreated,
			checkResponse: func(t *testing.T, data map[string]interface{}) {
				assertJSONFieldExists(t, data, "id")
				assertJSONField(t, data, "question", "What's your favorite color?")
				assertJSONField(t, data, "multi_select", false)
				assertJSONField(t, data, "anonymous", false)
				assertJSONField(t, data, "closed", false)

				options := data["options"].([]interface{})
				if len(options) != 3 {
					t.Errorf("Expected 3 options, got %d", len(options))
				}
			},
		},
		{
			name: "create multi-select poll",
			body: map[string]interface{}{
				"question":     "Select all that apply:",
				"options":      []string{"Option A", "Option B"},
				"multi_select": true,
			},
			token:          token,
			expectedStatus: http.StatusCreated,
			checkResponse: func(t *testing.T, data map[string]interface{}) {
				assertJSONField(t, data, "multi_select", true)
			},
		},
		{
			name: "create anonymous poll",
			body: map[string]interface{}{
				"question":  "Anonymous vote",
				"options":   []string{"Yes", "No"},
				"anonymous": true,
			},
			token:          token,
			expectedStatus: http.StatusCreated,
			checkResponse: func(t *testing.T, data map[string]interface{}) {
				assertJSONField(t, data, "anonymous", true)
			},
		},
		{
			name: "missing question",
			body: map[string]interface{}{
				"options": []string{"A", "B"},
			},
			token:          token,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "not enough options",
			body: map[string]interface{}{
				"question": "Only one option",
				"options":  []string{"Single"},
			},
			token:          token,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "too many options",
			body: map[string]interface{}{
				"question": "Too many",
				"options":  []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11"},
			},
			token:          token,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "unauthorized",
			body: map[string]interface{}{
				"question": "Test",
				"options":  []string{"A", "B"},
			},
			token:          "",
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, body := makeRequest(app, testRequest{
				Method: "POST",
				Path:   "/polls",
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

func TestPollHandler_Create_GroupPoll(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	hub := websocket.NewHub()
	app := fiber.New()
	handler := NewPollHandler(hub)

	app.Use(middleware.AuthRequired())
	app.Post("/polls", handler.Create)

	user, token := createTestUser(t, "grouppollcreator", "password123")

	// Create a group
	group := models.Group{Name: "Poll Group", CreatedBy: user.ID}
	database.DB.Create(&group)
	database.DB.Create(&models.GroupMember{GroupID: group.ID, UserID: user.ID, Role: models.GroupRoleOwner})

	resp, body := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/polls",
		Body: map[string]interface{}{
			"question": "Group poll question",
			"options":  []string{"Yes", "No"},
			"group_id": group.ID,
		},
		Token: token,
	})

	assertStatus(t, resp, http.StatusCreated)

	data := parseResponse(body)
	assertJSONFieldExists(t, data, "id")
}

func TestPollHandler_Create_NotGroupMember(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	hub := websocket.NewHub()
	app := fiber.New()
	handler := NewPollHandler(hub)

	app.Use(middleware.AuthRequired())
	app.Post("/polls", handler.Create)

	owner, _ := createTestUser(t, "groupowner", "password123")
	_, nonMemberToken := createTestUser(t, "nonmember", "password123")

	// Create a group (non-member not in it)
	group := models.Group{Name: "Private Group", CreatedBy: owner.ID}
	database.DB.Create(&group)
	database.DB.Create(&models.GroupMember{GroupID: group.ID, UserID: owner.ID, Role: models.GroupRoleOwner})

	resp, _ := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/polls",
		Body: map[string]interface{}{
			"question": "Forbidden poll",
			"options":  []string{"A", "B"},
			"group_id": group.ID,
		},
		Token: nonMemberToken,
	})

	assertStatus(t, resp, http.StatusForbidden)
}

func TestPollHandler_Get(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	hub := websocket.NewHub()
	app := fiber.New()
	handler := NewPollHandler(hub)

	app.Use(middleware.AuthRequired())
	app.Post("/polls", handler.Create)
	app.Get("/polls/:id", handler.Get)

	_, token := createTestUser(t, "pollgetter", "password123")

	// Create a poll
	resp1, body1 := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/polls",
		Body: map[string]interface{}{
			"question": "Get this poll",
			"options":  []string{"Option 1", "Option 2"},
		},
		Token: token,
	})
	assertStatus(t, resp1, http.StatusCreated)

	data1 := parseResponse(body1)
	pollID := data1["id"].(string)

	// Get the poll
	resp2, body2 := makeRequest(app, testRequest{
		Method: "GET",
		Path:   "/polls/" + pollID,
		Token:  token,
	})

	assertStatus(t, resp2, http.StatusOK)

	data2 := parseResponse(body2)
	assertJSONField(t, data2, "id", pollID)
	assertJSONField(t, data2, "question", "Get this poll")
}

func TestPollHandler_Get_NotFound(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	hub := websocket.NewHub()
	app := fiber.New()
	handler := NewPollHandler(hub)

	app.Use(middleware.AuthRequired())
	app.Get("/polls/:id", handler.Get)

	_, token := createTestUser(t, "pollgetter404", "password123")

	resp, _ := makeRequest(app, testRequest{
		Method: "GET",
		Path:   "/polls/nonexistent-id",
		Token:  token,
	})

	assertStatus(t, resp, http.StatusNotFound)
}

func TestPollHandler_Vote(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	hub := websocket.NewHub()
	app := fiber.New()
	handler := NewPollHandler(hub)

	app.Use(middleware.AuthRequired())
	app.Post("/polls", handler.Create)
	app.Post("/polls/:id/vote", handler.Vote)

	_, token := createTestUser(t, "voter", "password123")

	// Create a poll
	_, body1 := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/polls",
		Body: map[string]interface{}{
			"question": "Vote on this",
			"options":  []string{"Option A", "Option B"},
		},
		Token: token,
	})
	data1 := parseResponse(body1)
	pollID := data1["id"].(string)
	options := data1["options"].([]interface{})
	optionID := options[0].(map[string]interface{})["id"].(string)

	// Vote
	resp2, body2 := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/polls/" + pollID + "/vote",
		Body:   map[string]interface{}{"option_id": optionID},
		Token:  token,
	})

	assertStatus(t, resp2, http.StatusOK)

	data2 := parseResponse(body2)
	assertJSONField(t, data2, "total_votes", float64(1))

	myVotes := data2["my_votes"].([]interface{})
	if len(myVotes) != 1 || myVotes[0] != optionID {
		t.Error("Expected my_votes to contain the voted option")
	}
}

func TestPollHandler_Vote_Toggle(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	hub := websocket.NewHub()
	app := fiber.New()
	handler := NewPollHandler(hub)

	app.Use(middleware.AuthRequired())
	app.Post("/polls", handler.Create)
	app.Post("/polls/:id/vote", handler.Vote)

	_, token := createTestUser(t, "toggler", "password123")

	// Create a poll
	_, body1 := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/polls",
		Body: map[string]interface{}{
			"question": "Toggle vote",
			"options":  []string{"A", "B"},
		},
		Token: token,
	})
	data1 := parseResponse(body1)
	pollID := data1["id"].(string)
	options := data1["options"].([]interface{})
	optionID := options[0].(map[string]interface{})["id"].(string)

	// Vote
	_, _ = makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/polls/" + pollID + "/vote",
		Body:   map[string]interface{}{"option_id": optionID},
		Token:  token,
	})

	// Vote again (should toggle off)
	resp2, body2 := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/polls/" + pollID + "/vote",
		Body:   map[string]interface{}{"option_id": optionID},
		Token:  token,
	})

	assertStatus(t, resp2, http.StatusOK)

	data2 := parseResponse(body2)
	assertJSONField(t, data2, "total_votes", float64(0))

	myVotes := data2["my_votes"]
	if myVotes != nil {
		myVotesList := myVotes.([]interface{})
		if len(myVotesList) != 0 {
			t.Error("Expected my_votes to be empty after toggling off")
		}
	}
}

func TestPollHandler_Vote_SingleSelect(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	hub := websocket.NewHub()
	app := fiber.New()
	handler := NewPollHandler(hub)

	app.Use(middleware.AuthRequired())
	app.Post("/polls", handler.Create)
	app.Post("/polls/:id/vote", handler.Vote)

	_, token := createTestUser(t, "singlevoter", "password123")

	// Create a single-select poll
	_, body1 := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/polls",
		Body: map[string]interface{}{
			"question":     "Single select",
			"options":      []string{"A", "B", "C"},
			"multi_select": false,
		},
		Token: token,
	})
	data1 := parseResponse(body1)
	pollID := data1["id"].(string)
	options := data1["options"].([]interface{})
	optionA := options[0].(map[string]interface{})["id"].(string)
	optionB := options[1].(map[string]interface{})["id"].(string)

	// Vote for A
	_, _ = makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/polls/" + pollID + "/vote",
		Body:   map[string]interface{}{"option_id": optionA},
		Token:  token,
	})

	// Vote for B (should replace vote for A)
	resp2, body2 := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/polls/" + pollID + "/vote",
		Body:   map[string]interface{}{"option_id": optionB},
		Token:  token,
	})

	assertStatus(t, resp2, http.StatusOK)

	data2 := parseResponse(body2)
	assertJSONField(t, data2, "total_votes", float64(1))

	myVotes := data2["my_votes"].([]interface{})
	if len(myVotes) != 1 || myVotes[0] != optionB {
		t.Error("Expected vote to switch to option B")
	}
}

func TestPollHandler_Vote_MultiSelect(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	hub := websocket.NewHub()
	app := fiber.New()
	handler := NewPollHandler(hub)

	app.Use(middleware.AuthRequired())
	app.Post("/polls", handler.Create)
	app.Post("/polls/:id/vote", handler.Vote)

	_, token := createTestUser(t, "multivoter", "password123")

	// Create a multi-select poll
	_, body1 := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/polls",
		Body: map[string]interface{}{
			"question":     "Multi select",
			"options":      []string{"A", "B", "C"},
			"multi_select": true,
		},
		Token: token,
	})
	data1 := parseResponse(body1)
	pollID := data1["id"].(string)
	options := data1["options"].([]interface{})
	optionA := options[0].(map[string]interface{})["id"].(string)
	optionB := options[1].(map[string]interface{})["id"].(string)

	// Vote for A
	_, _ = makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/polls/" + pollID + "/vote",
		Body:   map[string]interface{}{"option_id": optionA},
		Token:  token,
	})

	// Vote for B (should add to existing vote)
	resp2, body2 := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/polls/" + pollID + "/vote",
		Body:   map[string]interface{}{"option_id": optionB},
		Token:  token,
	})

	assertStatus(t, resp2, http.StatusOK)

	data2 := parseResponse(body2)
	assertJSONField(t, data2, "total_votes", float64(2))

	myVotes := data2["my_votes"].([]interface{})
	if len(myVotes) != 2 {
		t.Errorf("Expected 2 votes in multi-select, got %d", len(myVotes))
	}
}

func TestPollHandler_Vote_ClosedPoll(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	hub := websocket.NewHub()
	app := fiber.New()
	handler := NewPollHandler(hub)

	app.Use(middleware.AuthRequired())
	app.Post("/polls", handler.Create)
	app.Post("/polls/:id/vote", handler.Vote)
	app.Post("/polls/:id/close", handler.Close)

	_, token := createTestUser(t, "closedvoter", "password123")

	// Create and close a poll
	_, body1 := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/polls",
		Body: map[string]interface{}{
			"question": "Soon to be closed",
			"options":  []string{"A", "B"},
		},
		Token: token,
	})
	data1 := parseResponse(body1)
	pollID := data1["id"].(string)
	options := data1["options"].([]interface{})
	optionID := options[0].(map[string]interface{})["id"].(string)

	// Close the poll
	_, _ = makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/polls/" + pollID + "/close",
		Token:  token,
	})

	// Try to vote
	resp2, _ := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/polls/" + pollID + "/vote",
		Body:   map[string]interface{}{"option_id": optionID},
		Token:  token,
	})

	assertStatus(t, resp2, http.StatusBadRequest)
}

func TestPollHandler_Close(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	hub := websocket.NewHub()
	app := fiber.New()
	handler := NewPollHandler(hub)

	app.Use(middleware.AuthRequired())
	app.Post("/polls", handler.Create)
	app.Post("/polls/:id/close", handler.Close)

	_, token := createTestUser(t, "pollcloser", "password123")

	// Create a poll
	_, body1 := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/polls",
		Body: map[string]interface{}{
			"question": "Close me",
			"options":  []string{"A", "B"},
		},
		Token: token,
	})
	data1 := parseResponse(body1)
	pollID := data1["id"].(string)

	// Close the poll
	resp2, body2 := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/polls/" + pollID + "/close",
		Token:  token,
	})

	assertStatus(t, resp2, http.StatusOK)

	data2 := parseResponse(body2)
	assertJSONField(t, data2, "closed", true)
}

func TestPollHandler_Close_NotCreator(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	hub := websocket.NewHub()
	app := fiber.New()
	handler := NewPollHandler(hub)

	app.Use(middleware.AuthRequired())
	app.Post("/polls", handler.Create)
	app.Post("/polls/:id/close", handler.Close)

	_, creatorToken := createTestUser(t, "creator", "password123")
	_, otherToken := createTestUser(t, "other", "password123")

	// Create a poll
	_, body1 := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/polls",
		Body: map[string]interface{}{
			"question": "Only creator can close",
			"options":  []string{"A", "B"},
		},
		Token: creatorToken,
	})
	data1 := parseResponse(body1)
	pollID := data1["id"].(string)

	// Other user tries to close
	resp2, _ := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/polls/" + pollID + "/close",
		Token:  otherToken,
	})

	assertStatus(t, resp2, http.StatusForbidden)
}

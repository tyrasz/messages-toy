package handlers

import (
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v2"
	"messenger/internal/api/middleware"
	"messenger/internal/database"
	"messenger/internal/models"
)

func setupStoriesTestApp() *fiber.App {
	app := fiber.New()
	handler := NewStoriesHandler(nil) // No hub for tests

	protected := app.Group("", middleware.AuthRequired())
	stories := protected.Group("/stories")
	stories.Post("/", handler.Create)
	stories.Get("/", handler.List)
	stories.Get("/mine", handler.GetMyStories)
	stories.Get("/:id", handler.Get)
	stories.Post("/:id/view", handler.View)
	stories.Get("/:id/views", handler.GetViews)
	stories.Delete("/:id", handler.Delete)

	return app
}

func TestCreateStory_TextOnly(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	_, token := createTestUser(t, "testuser", "password123")
	app := setupStoriesTestApp()

	resp, body := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/stories",
		Body: map[string]interface{}{
			"content":          "Hello, this is my story!",
			"background_color": "#ff5733",
			"text_color":       "#ffffff",
		},
		Token: token,
	})

	assertStatus(t, resp, http.StatusCreated)

	data := parseResponse(body)
	assertJSONFieldExists(t, data, "id")
	assertJSONField(t, data, "content", "Hello, this is my story!")
	assertJSONField(t, data, "background_color", "#ff5733")
}

func TestCreateStory_RequiresContent(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	_, token := createTestUser(t, "testuser", "password123")
	app := setupStoriesTestApp()

	resp, body := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/stories",
		Body:   map[string]interface{}{},
		Token:  token,
	})

	assertStatus(t, resp, http.StatusBadRequest)

	data := parseResponse(body)
	assertJSONFieldExists(t, data, "error")
}

func TestCreateStory_DefaultPrivacy(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	_, token := createTestUser(t, "testuser", "password123")
	app := setupStoriesTestApp()

	resp, body := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/stories",
		Body: map[string]interface{}{
			"content": "Test story",
		},
		Token: token,
	})

	assertStatus(t, resp, http.StatusCreated)

	data := parseResponse(body)
	assertJSONField(t, data, "privacy", "contacts")
}

func TestListStories(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user1, token1 := createTestUser(t, "testuser1", "password123")
	user2, token2 := createTestUser(t, "testuser2", "password123")
	app := setupStoriesTestApp()

	// User2 creates a story with "everyone" privacy
	makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/stories",
		Body: map[string]interface{}{
			"content": "User2's public story",
			"privacy": "everyone",
		},
		Token: token2,
	})

	// Add user2 as contact of user1
	database.DB.Create(&models.Contact{
		UserID:    user1.ID,
		ContactID: user2.ID,
	})

	// User2 creates a contacts-only story
	makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/stories",
		Body: map[string]interface{}{
			"content": "User2's contacts story",
			"privacy": "contacts",
		},
		Token: token2,
	})

	// User1 lists stories
	resp, body := makeRequest(app, testRequest{
		Method: "GET",
		Path:   "/stories",
		Token:  token1,
	})

	assertStatus(t, resp, http.StatusOK)

	data := parseResponse(body)
	assertJSONFieldExists(t, data, "story_users")
}

func TestGetMyStories(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	_, token := createTestUser(t, "testuser", "password123")
	app := setupStoriesTestApp()

	// Create a few stories
	for i := 0; i < 3; i++ {
		makeRequest(app, testRequest{
			Method: "POST",
			Path:   "/stories",
			Body: map[string]interface{}{
				"content": "Story content",
			},
			Token: token,
		})
	}

	resp, body := makeRequest(app, testRequest{
		Method: "GET",
		Path:   "/stories/mine",
		Token:  token,
	})

	assertStatus(t, resp, http.StatusOK)

	data := parseResponse(body)
	stories, ok := data["stories"].([]interface{})
	if !ok {
		t.Fatal("Expected stories to be an array")
	}

	if len(stories) != 3 {
		t.Errorf("Expected 3 stories, got %d", len(stories))
	}
}

func TestGetStory(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	_, token := createTestUser(t, "testuser", "password123")
	app := setupStoriesTestApp()

	// Create a story
	resp1, body1 := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/stories",
		Body: map[string]interface{}{
			"content": "Test story",
		},
		Token: token,
	})

	assertStatus(t, resp1, http.StatusCreated)
	storyData := parseResponse(body1)
	storyID := storyData["id"].(string)

	// Get the story
	resp, body := makeRequest(app, testRequest{
		Method: "GET",
		Path:   "/stories/" + storyID,
		Token:  token,
	})

	assertStatus(t, resp, http.StatusOK)

	data := parseResponse(body)
	assertJSONField(t, data, "id", storyID)
	assertJSONField(t, data, "content", "Test story")
}

func TestViewStory(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user1, token1 := createTestUser(t, "testuser1", "password123")
	_, token2 := createTestUser(t, "testuser2", "password123")
	app := setupStoriesTestApp()

	// User1 creates a story
	resp1, body1 := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/stories",
		Body: map[string]interface{}{
			"content": "Test story",
			"privacy": "everyone",
		},
		Token: token1,
	})

	assertStatus(t, resp1, http.StatusCreated)
	storyData := parseResponse(body1)
	storyID := storyData["id"].(string)

	// User2 views the story
	resp, body := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/stories/" + storyID + "/view",
		Token:  token2,
	})

	assertStatus(t, resp, http.StatusOK)

	data := parseResponse(body)
	assertJSONFieldExists(t, data, "message")

	// Verify view count increased
	var story models.Story
	database.DB.First(&story, "id = ?", storyID)
	if story.ViewCount != 1 {
		t.Errorf("Expected view count 1, got %d", story.ViewCount)
	}

	// Verify view was recorded
	var view models.StoryView
	result := database.DB.Where("story_id = ?", storyID).First(&view)
	if result.Error != nil {
		t.Error("Expected story view to be recorded")
	}

	// Check views endpoint (only owner can access)
	resp3, body3 := makeRequest(app, testRequest{
		Method: "GET",
		Path:   "/stories/" + storyID + "/views",
		Token:  token1,
	})

	assertStatus(t, resp3, http.StatusOK)
	viewsData := parseResponse(body3)
	assertJSONField(t, viewsData, "view_count", float64(1))

	// Non-owner cannot view viewers
	resp4, _ := makeRequest(app, testRequest{
		Method: "GET",
		Path:   "/stories/" + storyID + "/views",
		Token:  token2,
	})
	assertStatus(t, resp4, http.StatusForbidden)

	_ = user1 // Ensure user1 is used
}

func TestDeleteStory(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	_, token := createTestUser(t, "testuser", "password123")
	app := setupStoriesTestApp()

	// Create a story
	resp1, body1 := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/stories",
		Body: map[string]interface{}{
			"content": "Test story",
		},
		Token: token,
	})

	assertStatus(t, resp1, http.StatusCreated)
	storyData := parseResponse(body1)
	storyID := storyData["id"].(string)

	// Delete it
	resp, body := makeRequest(app, testRequest{
		Method: "DELETE",
		Path:   "/stories/" + storyID,
		Token:  token,
	})

	assertStatus(t, resp, http.StatusOK)

	data := parseResponse(body)
	assertJSONFieldExists(t, data, "message")

	// Verify it was deleted
	var story models.Story
	result := database.DB.First(&story, "id = ?", storyID)
	if result.Error == nil {
		t.Error("Expected story to be deleted")
	}
}

func TestDeleteStory_NotOwner(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	_, token1 := createTestUser(t, "testuser1", "password123")
	_, token2 := createTestUser(t, "testuser2", "password123")
	app := setupStoriesTestApp()

	// User1 creates a story
	resp1, body1 := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/stories",
		Body: map[string]interface{}{
			"content": "Test story",
		},
		Token: token1,
	})

	assertStatus(t, resp1, http.StatusCreated)
	storyData := parseResponse(body1)
	storyID := storyData["id"].(string)

	// User2 tries to delete it
	resp, _ := makeRequest(app, testRequest{
		Method: "DELETE",
		Path:   "/stories/" + storyID,
		Token:  token2,
	})

	assertStatus(t, resp, http.StatusNotFound)
}

func TestGetStory_NotFound(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	_, token := createTestUser(t, "testuser", "password123")
	app := setupStoriesTestApp()

	resp, body := makeRequest(app, testRequest{
		Method: "GET",
		Path:   "/stories/nonexistent-id",
		Token:  token,
	})

	assertStatus(t, resp, http.StatusNotFound)

	data := parseResponse(body)
	assertJSONFieldExists(t, data, "error")
}

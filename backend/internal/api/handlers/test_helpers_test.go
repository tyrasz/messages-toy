package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"messenger/internal/database"
	"messenger/internal/models"
	"messenger/internal/services"
)

// setupTestDB creates an in-memory SQLite database for testing
func setupTestDB(t *testing.T) func() {
	var err error
	database.DB, err = gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Run migrations
	err = database.DB.AutoMigrate(
		&models.User{},
		&models.Contact{},
		&models.Message{},
		&models.Group{},
		&models.GroupMember{},
		&models.Block{},
		&models.StarredMessage{},
		&models.Poll{},
		&models.PollOption{},
		&models.PollVote{},
		&models.PinnedMessage{},
		&models.ArchivedConversation{},
		&models.MessageReadReceipt{},
		&models.ConversationSettings{},
		&models.BroadcastList{},
		&models.BroadcastListRecipient{},
		&models.Reaction{},
		&models.ChatTheme{},
		&models.Media{},
		&models.Story{},
		&models.StoryView{},
	)
	if err != nil {
		t.Fatalf("Failed to migrate test database: %v", err)
	}

	return func() {
		sqlDB, _ := database.DB.DB()
		sqlDB.Close()
	}
}

// createTestUser creates a user and returns the user and auth token
func createTestUser(t *testing.T, username, password string) (*models.User, string) {
	authService := services.NewAuthService()
	resp, err := authService.Register(services.RegisterInput{
		Username: username,
		Password: password,
	})
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	var user models.User
	database.DB.Where("username = ?", username).First(&user)

	return &user, resp.AccessToken
}

// testRequest is a helper to make test HTTP requests
type testRequest struct {
	Method  string
	Path    string
	Body    interface{}
	Token   string
	Headers map[string]string
}

// makeRequest executes a test request against the Fiber app
func makeRequest(app *fiber.App, req testRequest) (*http.Response, []byte) {
	var bodyReader io.Reader
	if req.Body != nil {
		bodyBytes, _ := json.Marshal(req.Body)
		bodyReader = bytes.NewReader(bodyBytes)
	}

	httpReq := httptest.NewRequest(req.Method, req.Path, bodyReader)
	httpReq.Header.Set("Content-Type", "application/json")

	if req.Token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+req.Token)
	}

	for key, value := range req.Headers {
		httpReq.Header.Set(key, value)
	}

	resp, _ := app.Test(httpReq, -1)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	return resp, body
}

// parseResponse parses JSON response body into a map
func parseResponse(body []byte) map[string]interface{} {
	var result map[string]interface{}
	json.Unmarshal(body, &result)
	return result
}

// assertStatus checks that the response has the expected status code
func assertStatus(t *testing.T, resp *http.Response, expected int) {
	t.Helper()
	if resp.StatusCode != expected {
		t.Errorf("Expected status %d, got %d", expected, resp.StatusCode)
	}
}

// assertJSONField checks that a field exists and has the expected value
func assertJSONField(t *testing.T, data map[string]interface{}, field string, expected interface{}) {
	t.Helper()
	value, exists := data[field]
	if !exists {
		t.Errorf("Expected field %q to exist in response", field)
		return
	}
	if value != expected {
		t.Errorf("Expected field %q to be %v, got %v", field, expected, value)
	}
}

// assertJSONFieldExists checks that a field exists
func assertJSONFieldExists(t *testing.T, data map[string]interface{}, field string) {
	t.Helper()
	if _, exists := data[field]; !exists {
		t.Errorf("Expected field %q to exist in response", field)
	}
}

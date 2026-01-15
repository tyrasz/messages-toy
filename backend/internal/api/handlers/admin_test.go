package handlers

import (
	"net/http"
	"os"
	"testing"

	"github.com/gofiber/fiber/v2"
	"messenger/internal/api/middleware"
	"messenger/internal/database"
	"messenger/internal/models"
)

func setupAdminTestApp() *fiber.App {
	app := fiber.New()
	handler := NewAdminHandler()

	protected := app.Group("", middleware.AuthRequired())
	admin := protected.Group("/admin")
	admin.Get("/review", handler.GetPendingReview)
	admin.Post("/review/:id", handler.Review)

	return app
}

func TestGetPendingReview_Empty(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	_, token := createTestUser(t, "admin", "password123")
	app := setupAdminTestApp()

	resp, body := makeRequest(app, testRequest{
		Method: "GET",
		Path:   "/admin/review",
		Token:  token,
	})

	assertStatus(t, resp, http.StatusOK)

	data := parseResponse(body)
	assertJSONField(t, data, "count", float64(0))
}

func TestGetPendingReview_WithItems(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user, token := createTestUser(t, "admin", "password123")
	app := setupAdminTestApp()

	// Create some media items pending review
	for i := 0; i < 3; i++ {
		database.DB.Create(&models.Media{
			UploaderID:  user.ID,
			Filename:    "test.jpg",
			ContentType: "image/jpeg",
			MediaType:   models.MediaTypeImage,
			Status:      models.MediaStatusReview,
			StoragePath: "/tmp/test.jpg",
		})
	}

	resp, body := makeRequest(app, testRequest{
		Method: "GET",
		Path:   "/admin/review",
		Token:  token,
	})

	assertStatus(t, resp, http.StatusOK)

	data := parseResponse(body)
	assertJSONField(t, data, "count", float64(3))

	items, ok := data["items"].([]interface{})
	if !ok {
		t.Fatal("Expected items to be an array")
	}
	if len(items) != 3 {
		t.Errorf("Expected 3 items, got %d", len(items))
	}
}

func TestReview_Approve(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user, token := createTestUser(t, "admin", "password123")
	app := setupAdminTestApp()

	// Create temp directory and file for the test
	os.MkdirAll("./uploads/quarantine", 0755)
	os.MkdirAll("./uploads/approved", 0755)
	defer os.RemoveAll("./uploads")

	tempFile := "./uploads/quarantine/test-approve.jpg"
	os.WriteFile(tempFile, []byte("test image data"), 0644)

	// Create media item pending review
	media := models.Media{
		UploaderID:  user.ID,
		Filename:    "test-approve.jpg",
		ContentType: "image/jpeg",
		MediaType:   models.MediaTypeImage,
		Status:      models.MediaStatusReview,
		StoragePath: tempFile,
	}
	database.DB.Create(&media)

	resp, body := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/admin/review/" + media.ID,
		Body: map[string]interface{}{
			"action": "approve",
		},
		Token: token,
	})

	assertStatus(t, resp, http.StatusOK)

	data := parseResponse(body)
	assertJSONFieldExists(t, data, "message")

	// Verify media was approved
	var updated models.Media
	database.DB.First(&updated, "id = ?", media.ID)
	if updated.Status != models.MediaStatusApproved {
		t.Errorf("Expected status 'approved', got '%s'", updated.Status)
	}
}

func TestReview_Reject(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user, token := createTestUser(t, "admin", "password123")
	app := setupAdminTestApp()

	// Create temp file
	os.MkdirAll("./uploads/quarantine", 0755)
	defer os.RemoveAll("./uploads")

	tempFile := "./uploads/quarantine/test-reject.jpg"
	os.WriteFile(tempFile, []byte("test image data"), 0644)

	// Create media item pending review
	media := models.Media{
		UploaderID:  user.ID,
		Filename:    "test-reject.jpg",
		ContentType: "image/jpeg",
		MediaType:   models.MediaTypeImage,
		Status:      models.MediaStatusReview,
		StoragePath: tempFile,
	}
	database.DB.Create(&media)

	resp, body := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/admin/review/" + media.ID,
		Body: map[string]interface{}{
			"action": "reject",
			"reason": "Inappropriate content",
		},
		Token: token,
	})

	assertStatus(t, resp, http.StatusOK)

	data := parseResponse(body)
	assertJSONFieldExists(t, data, "message")

	// Verify media was rejected
	var updated models.Media
	database.DB.First(&updated, "id = ?", media.ID)
	if updated.Status != models.MediaStatusRejected {
		t.Errorf("Expected status 'rejected', got '%s'", updated.Status)
	}

	// Verify file was deleted
	if _, err := os.Stat(tempFile); !os.IsNotExist(err) {
		t.Error("Expected file to be deleted")
	}
}

func TestReview_InvalidAction(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user, token := createTestUser(t, "admin", "password123")
	app := setupAdminTestApp()

	// Create media item
	media := models.Media{
		UploaderID:  user.ID,
		Filename:    "test.jpg",
		ContentType: "image/jpeg",
		MediaType:   models.MediaTypeImage,
		Status:      models.MediaStatusReview,
		StoragePath: "/tmp/test.jpg",
	}
	database.DB.Create(&media)

	resp, body := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/admin/review/" + media.ID,
		Body: map[string]interface{}{
			"action": "invalid",
		},
		Token: token,
	})

	assertStatus(t, resp, http.StatusBadRequest)

	data := parseResponse(body)
	assertJSONFieldExists(t, data, "error")
}

func TestReview_NotPendingReview(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user, token := createTestUser(t, "admin", "password123")
	app := setupAdminTestApp()

	// Create already approved media item
	media := models.Media{
		UploaderID:  user.ID,
		Filename:    "test.jpg",
		ContentType: "image/jpeg",
		MediaType:   models.MediaTypeImage,
		Status:      models.MediaStatusApproved,
		StoragePath: "/tmp/test.jpg",
	}
	database.DB.Create(&media)

	resp, body := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/admin/review/" + media.ID,
		Body: map[string]interface{}{
			"action": "approve",
		},
		Token: token,
	})

	assertStatus(t, resp, http.StatusBadRequest)

	data := parseResponse(body)
	assertJSONFieldExists(t, data, "error")
}

func TestReview_NotFound(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	_, token := createTestUser(t, "admin", "password123")
	app := setupAdminTestApp()

	resp, body := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/admin/review/nonexistent-id",
		Body: map[string]interface{}{
			"action": "approve",
		},
		Token: token,
	})

	assertStatus(t, resp, http.StatusNotFound)

	data := parseResponse(body)
	assertJSONFieldExists(t, data, "error")
}

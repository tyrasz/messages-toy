package handlers

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"testing"

	"github.com/gofiber/fiber/v2"
	"messenger/internal/api/middleware"
	"messenger/internal/database"
	"messenger/internal/models"
)

func setupMediaTestApp() *fiber.App {
	app := fiber.New()
	handler := NewMediaHandler(nil) // No hub for tests

	protected := app.Group("", middleware.AuthRequired())
	media := protected.Group("/media")
	media.Post("/upload", handler.Upload)
	media.Get("/:id", handler.Get)
	media.Get("/:id/thumbnail", handler.GetThumbnail)

	return app
}

func createMultipartRequest(t *testing.T, fieldname, filename, contentType string, content []byte) (*bytes.Buffer, string) {
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)

	// Create form file with proper content type header
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, fieldname, filename))
	h.Set("Content-Type", contentType)

	part, err := writer.CreatePart(h)
	if err != nil {
		t.Fatalf("Failed to create form file: %v", err)
	}

	_, err = io.Copy(part, bytes.NewReader(content))
	if err != nil {
		t.Fatalf("Failed to copy content: %v", err)
	}

	err = writer.Close()
	if err != nil {
		t.Fatalf("Failed to close writer: %v", err)
	}

	return body, writer.FormDataContentType()
}

func TestUpload_NoFile(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	_, token := createTestUser(t, "testuser", "password123")
	app := setupMediaTestApp()

	resp, body := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/media/upload",
		Token:  token,
	})

	assertStatus(t, resp, http.StatusBadRequest)

	data := parseResponse(body)
	assertJSONFieldExists(t, data, "error")
}

func TestUpload_Image(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// Create upload directories
	os.MkdirAll("./uploads/quarantine", 0755)
	os.MkdirAll("./uploads/approved", 0755)
	defer os.RemoveAll("./uploads")

	_, token := createTestUser(t, "testuser", "password123")
	app := setupMediaTestApp()

	// Create a fake image file
	imageContent := []byte{0xFF, 0xD8, 0xFF, 0xE0} // JPEG header
	imageContent = append(imageContent, make([]byte, 1000)...)

	body, contentType := createMultipartRequest(t, "file", "test.jpg", "image/jpeg", imageContent)

	req := httptest.NewRequest("POST", "/media/upload", body)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	if resp.StatusCode != http.StatusAccepted {
		respBody, _ := io.ReadAll(resp.Body)
		t.Errorf("Expected status 202, got %d: %s", resp.StatusCode, string(respBody))
	}
}

func TestUpload_InvalidType(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	os.MkdirAll("./uploads/quarantine", 0755)
	defer os.RemoveAll("./uploads")

	_, token := createTestUser(t, "testuser", "password123")
	app := setupMediaTestApp()

	// Create a fake executable file
	content := []byte("MZ") // DOS header
	content = append(content, make([]byte, 1000)...)

	body, contentType := createMultipartRequest(t, "file", "test.exe", "application/x-msdownload", content)

	req := httptest.NewRequest("POST", "/media/upload", body)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", resp.StatusCode)
	}
}

func TestGetMedia_NotFound(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	_, token := createTestUser(t, "testuser", "password123")
	app := setupMediaTestApp()

	resp, body := makeRequest(app, testRequest{
		Method: "GET",
		Path:   "/media/nonexistent-id",
		Token:  token,
	})

	assertStatus(t, resp, http.StatusNotFound)

	data := parseResponse(body)
	assertJSONFieldExists(t, data, "error")
}

func TestGetMedia_Approved(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	os.MkdirAll("./uploads/approved", 0755)
	defer os.RemoveAll("./uploads")

	// Create test file
	testFile := "./uploads/approved/test-file.txt"
	os.WriteFile(testFile, []byte("test content"), 0644)

	user, token := createTestUser(t, "testuser", "password123")
	app := setupMediaTestApp()

	// Create approved media record
	media := models.Media{
		UploaderID:  user.ID,
		Filename:    "test-file.txt",
		ContentType: "text/plain",
		MediaType:   models.MediaTypeDocument,
		Status:      models.MediaStatusApproved,
		StoragePath: testFile,
	}
	database.DB.Create(&media)

	resp, _ := makeRequest(app, testRequest{
		Method: "GET",
		Path:   "/media/" + media.ID,
		Token:  token,
	})

	assertStatus(t, resp, http.StatusOK)
}

func TestGetMedia_RejectedByUploader(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user, token := createTestUser(t, "testuser", "password123")
	app := setupMediaTestApp()

	// Create rejected media record
	media := models.Media{
		UploaderID:  user.ID,
		Filename:    "rejected.jpg",
		ContentType: "image/jpeg",
		MediaType:   models.MediaTypeImage,
		Status:      models.MediaStatusRejected,
		StoragePath: "",
	}
	database.DB.Create(&media)

	resp, body := makeRequest(app, testRequest{
		Method: "GET",
		Path:   "/media/" + media.ID,
		Token:  token,
	})

	assertStatus(t, resp, http.StatusGone)

	data := parseResponse(body)
	assertJSONFieldExists(t, data, "error")
}

func TestGetMedia_PendingByUploader(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	os.MkdirAll("./uploads/quarantine", 0755)
	defer os.RemoveAll("./uploads")

	testFile := "./uploads/quarantine/pending-file.txt"
	os.WriteFile(testFile, []byte("test content"), 0644)

	user, token := createTestUser(t, "testuser", "password123")
	app := setupMediaTestApp()

	// Create pending media record
	media := models.Media{
		UploaderID:  user.ID,
		Filename:    "pending-file.txt",
		ContentType: "text/plain",
		MediaType:   models.MediaTypeDocument,
		Status:      models.MediaStatusPending,
		StoragePath: testFile,
	}
	database.DB.Create(&media)

	// Uploader can see their own pending media
	resp, _ := makeRequest(app, testRequest{
		Method: "GET",
		Path:   "/media/" + media.ID,
		Token:  token,
	})

	assertStatus(t, resp, http.StatusOK)
}

func TestGetMedia_PendingByOther(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user1, _ := createTestUser(t, "testuser1", "password123")
	_, token2 := createTestUser(t, "testuser2", "password123")
	app := setupMediaTestApp()

	// Create pending media record owned by user1
	media := models.Media{
		UploaderID:  user1.ID,
		Filename:    "pending-file.txt",
		ContentType: "text/plain",
		MediaType:   models.MediaTypeDocument,
		Status:      models.MediaStatusPending,
		StoragePath: "/tmp/pending.txt",
	}
	database.DB.Create(&media)

	// User2 cannot see user1's pending media
	resp, body := makeRequest(app, testRequest{
		Method: "GET",
		Path:   "/media/" + media.ID,
		Token:  token2,
	})

	assertStatus(t, resp, http.StatusForbidden)

	data := parseResponse(body)
	assertJSONFieldExists(t, data, "error")
}

func TestGetThumbnail_NotFound(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	_, token := createTestUser(t, "testuser", "password123")
	app := setupMediaTestApp()

	resp, body := makeRequest(app, testRequest{
		Method: "GET",
		Path:   "/media/nonexistent-id/thumbnail",
		Token:  token,
	})

	assertStatus(t, resp, http.StatusNotFound)

	data := parseResponse(body)
	assertJSONFieldExists(t, data, "error")
}

func TestGetThumbnail_NoThumbnail(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	user, token := createTestUser(t, "testuser", "password123")
	app := setupMediaTestApp()

	// Create media without thumbnail
	media := models.Media{
		UploaderID:    user.ID,
		Filename:      "test.jpg",
		ContentType:   "image/jpeg",
		MediaType:     models.MediaTypeImage,
		Status:        models.MediaStatusApproved,
		StoragePath:   "/tmp/test.jpg",
		ThumbnailPath: "", // No thumbnail
	}
	database.DB.Create(&media)

	resp, body := makeRequest(app, testRequest{
		Method: "GET",
		Path:   "/media/" + media.ID + "/thumbnail",
		Token:  token,
	})

	assertStatus(t, resp, http.StatusNotFound)

	data := parseResponse(body)
	assertJSONFieldExists(t, data, "error")
}

func TestGetThumbnail_HasThumbnail(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	os.MkdirAll("./uploads/thumbnails", 0755)
	defer os.RemoveAll("./uploads")

	// Create thumbnail file
	thumbnailFile := "./uploads/thumbnails/thumb.jpg"
	os.WriteFile(thumbnailFile, []byte("thumbnail data"), 0644)

	user, token := createTestUser(t, "testuser", "password123")
	app := setupMediaTestApp()

	// Create media with thumbnail
	media := models.Media{
		UploaderID:    user.ID,
		Filename:      "video.mp4",
		ContentType:   "video/mp4",
		MediaType:     models.MediaTypeVideo,
		Status:        models.MediaStatusApproved,
		StoragePath:   "/tmp/video.mp4",
		ThumbnailPath: thumbnailFile,
	}
	database.DB.Create(&media)

	resp, _ := makeRequest(app, testRequest{
		Method: "GET",
		Path:   "/media/" + media.ID + "/thumbnail",
		Token:  token,
	})

	assertStatus(t, resp, http.StatusOK)
}

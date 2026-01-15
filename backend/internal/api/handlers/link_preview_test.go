package handlers

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/gofiber/fiber/v2"
	"messenger/internal/api/middleware"
	"messenger/internal/database"
	"messenger/internal/models"
)

func setupLinkPreviewTestApp() *fiber.App {
	app := fiber.New()
	handler := NewLinkPreviewHandler()

	protected := app.Group("", middleware.AuthRequired())
	links := protected.Group("/links")
	links.Post("/preview", handler.FetchPreview)
	links.Get("/preview", handler.GetPreview)

	return app
}

func TestGetPreview_Found(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	_, token := createTestUser(t, "testuser", "password123")
	app := setupLinkPreviewTestApp()

	// Create a preview in the database
	preview := models.LinkPreview{
		URL:         "https://example.com",
		Title:       "Example Domain",
		Description: "This domain is for use in examples.",
		SiteName:    "Example",
	}
	database.DB.Create(&preview)

	resp, body := makeRequest(app, testRequest{
		Method: "GET",
		Path:   "/links/preview?url=" + url.QueryEscape("https://example.com"),
		Token:  token,
	})

	assertStatus(t, resp, http.StatusOK)

	data := parseResponse(body)
	assertJSONFieldExists(t, data, "preview")

	previewData := data["preview"].(map[string]interface{})
	if previewData["title"] != "Example Domain" {
		t.Errorf("Expected title 'Example Domain', got '%v'", previewData["title"])
	}
}

func TestGetPreview_NotFound(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	_, token := createTestUser(t, "testuser", "password123")
	app := setupLinkPreviewTestApp()

	resp, body := makeRequest(app, testRequest{
		Method: "GET",
		Path:   "/links/preview?url=" + url.QueryEscape("https://nonexistent.com"),
		Token:  token,
	})

	assertStatus(t, resp, http.StatusNotFound)

	data := parseResponse(body)
	assertJSONFieldExists(t, data, "error")
}

func TestGetPreview_MissingURL(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	_, token := createTestUser(t, "testuser", "password123")
	app := setupLinkPreviewTestApp()

	resp, body := makeRequest(app, testRequest{
		Method: "GET",
		Path:   "/links/preview",
		Token:  token,
	})

	assertStatus(t, resp, http.StatusBadRequest)

	data := parseResponse(body)
	assertJSONFieldExists(t, data, "error")
}

func TestFetchPreview_MissingURL(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	_, token := createTestUser(t, "testuser", "password123")
	app := setupLinkPreviewTestApp()

	resp, body := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/links/preview",
		Body:   map[string]interface{}{},
		Token:  token,
	})

	assertStatus(t, resp, http.StatusBadRequest)

	data := parseResponse(body)
	assertJSONFieldExists(t, data, "error")
}

func TestFetchPreview_InvalidURL(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	_, token := createTestUser(t, "testuser", "password123")
	app := setupLinkPreviewTestApp()

	resp, body := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/links/preview",
		Body: map[string]interface{}{
			"url": "not-a-valid-url",
		},
		Token: token,
	})

	assertStatus(t, resp, http.StatusBadRequest)

	data := parseResponse(body)
	assertJSONFieldExists(t, data, "error")
}

func TestFetchPreview_NonHTTPURL(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	_, token := createTestUser(t, "testuser", "password123")
	app := setupLinkPreviewTestApp()

	resp, body := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/links/preview",
		Body: map[string]interface{}{
			"url": "ftp://example.com/file",
		},
		Token: token,
	})

	assertStatus(t, resp, http.StatusBadRequest)

	data := parseResponse(body)
	assertJSONFieldExists(t, data, "error")
}

func TestFetchPreview_CachedResult(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	_, token := createTestUser(t, "testuser", "password123")
	app := setupLinkPreviewTestApp()

	// Pre-populate the cache with a complete preview
	preview := models.LinkPreview{
		URL:         "https://cached-example.com",
		Title:       "Cached Example",
		Description: "This is a cached preview",
		SiteName:    "CachedSite",
	}
	database.DB.Create(&preview)

	resp, body := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/links/preview",
		Body: map[string]interface{}{
			"url": "https://cached-example.com",
		},
		Token: token,
	})

	assertStatus(t, resp, http.StatusOK)

	data := parseResponse(body)
	assertJSONField(t, data, "cached", true)

	previewData := data["preview"].(map[string]interface{})
	if previewData["title"] != "Cached Example" {
		t.Errorf("Expected title 'Cached Example', got '%v'", previewData["title"])
	}
}

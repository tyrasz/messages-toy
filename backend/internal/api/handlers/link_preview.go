package handlers

import (
	"net/url"

	"github.com/gofiber/fiber/v2"
	"messenger/internal/database"
	"messenger/internal/models"
	"messenger/internal/services"
)

type LinkPreviewHandler struct{}

func NewLinkPreviewHandler() *LinkPreviewHandler {
	return &LinkPreviewHandler{}
}

type FetchPreviewRequest struct {
	URL string `json:"url"`
}

// FetchPreview fetches metadata for a URL
func (h *LinkPreviewHandler) FetchPreview(c *fiber.Ctx) error {
	var req FetchPreviewRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Validate URL
	if req.URL == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "URL is required",
		})
	}

	parsedURL, err := url.Parse(req.URL)
	if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid URL",
		})
	}

	// Check cache first
	preview, isNew, err := models.GetOrCreateLinkPreview(database.DB, req.URL)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to process URL",
		})
	}

	// If cached and has title, return immediately
	if !isNew && preview.Title != "" {
		return c.JSON(fiber.Map{
			"preview": preview,
			"cached":  true,
		})
	}

	// Fetch metadata
	metadata, err := services.FetchLinkMetadata(req.URL)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Failed to fetch URL",
		})
	}

	// Update preview in database
	if err := models.UpdateLinkPreview(
		database.DB,
		preview.ID,
		metadata.Title,
		metadata.Description,
		metadata.ImageURL,
		metadata.SiteName,
		metadata.FaviconURL,
	); err != nil {
		// Log error but continue
	}

	// Return response
	return c.JSON(fiber.Map{
		"preview": fiber.Map{
			"id":          preview.ID,
			"url":         req.URL,
			"title":       metadata.Title,
			"description": metadata.Description,
			"image_url":   metadata.ImageURL,
			"site_name":   metadata.SiteName,
			"favicon_url": metadata.FaviconURL,
		},
		"cached": false,
	})
}

// GetPreview gets a cached preview by URL
func (h *LinkPreviewHandler) GetPreview(c *fiber.Ctx) error {
	targetURL := c.Query("url")
	if targetURL == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "URL is required",
		})
	}

	var preview models.LinkPreview
	if err := database.DB.Where("url = ?", targetURL).First(&preview).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Preview not found",
		})
	}

	return c.JSON(fiber.Map{
		"preview": preview,
	})
}

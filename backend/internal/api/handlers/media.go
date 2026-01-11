package handlers

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"messenger/internal/api/middleware"
	"messenger/internal/database"
	"messenger/internal/models"
	"messenger/internal/services"
	"messenger/internal/websocket"
)

type MediaHandler struct {
	hub               *websocket.Hub
	moderationService *services.ModerationService
}

func NewMediaHandler(hub *websocket.Hub) *MediaHandler {
	return &MediaHandler{
		hub:               hub,
		moderationService: services.NewModerationService(),
	}
}

func (h *MediaHandler) Upload(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	file, err := c.FormFile("file")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "No file uploaded",
		})
	}

	// Validate file type
	contentType := file.Header.Get("Content-Type")
	if !isAllowedMediaType(contentType) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "File type not allowed. Allowed types: jpg, png, gif, webp",
		})
	}

	// Validate file size (10MB max)
	if file.Size > 10*1024*1024 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "File too large. Maximum size is 10MB",
		})
	}

	// Generate unique filename
	ext := filepath.Ext(file.Filename)
	filename := fmt.Sprintf("%s%s", uuid.New().String(), ext)

	// Create quarantine directory if not exists
	quarantineDir := "./uploads/quarantine"
	if err := os.MkdirAll(quarantineDir, 0755); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create upload directory",
		})
	}

	// Save to quarantine
	quarantinePath := filepath.Join(quarantineDir, filename)
	if err := c.SaveFile(file, quarantinePath); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to save file",
		})
	}

	// Create media record
	media := models.Media{
		UploaderID:  userID,
		Filename:    filename,
		ContentType: contentType,
		Size:        file.Size,
		Status:      models.MediaStatusPending,
		StoragePath: quarantinePath,
	}

	if err := database.DB.Create(&media).Error; err != nil {
		os.Remove(quarantinePath)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create media record",
		})
	}

	// Process moderation asynchronously
	go h.processModeration(&media)

	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
		"id":      media.ID,
		"status":  media.Status,
		"message": "File uploaded and pending moderation",
	})
}

func (h *MediaHandler) processModeration(media *models.Media) {
	result, err := h.moderationService.ScanImage(media.StoragePath)
	if err != nil {
		// If moderation fails, mark for manual review
		database.DB.Model(media).Updates(map[string]interface{}{
			"status":      models.MediaStatusReview,
			"scan_result": fmt.Sprintf("Scan error: %v", err),
		})
		return
	}

	// Update media with scan result
	database.DB.Model(media).Updates(map[string]interface{}{
		"status":      result.Status,
		"scan_result": result.RawResult,
	})

	if result.Status == models.MediaStatusApproved {
		// Move from quarantine to approved storage
		approvedDir := "./uploads/approved"
		os.MkdirAll(approvedDir, 0755)

		newPath := filepath.Join(approvedDir, media.Filename)
		os.Rename(media.StoragePath, newPath)

		database.DB.Model(media).Updates(map[string]interface{}{
			"storage_path": newPath,
			"url":          fmt.Sprintf("/media/%s", media.ID),
		})
	} else if result.Status == models.MediaStatusRejected {
		// Delete the file
		os.Remove(media.StoragePath)
		database.DB.Model(media).Update("storage_path", "")
	}
	// If status is "review", keep in quarantine for manual review
}

func (h *MediaHandler) Get(c *fiber.Ctx) error {
	mediaID := c.Params("id")
	userID := middleware.GetUserID(c)

	var media models.Media
	if err := database.DB.First(&media, "id = ?", mediaID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Media not found",
		})
	}

	// Only uploader can see pending/review media
	if media.Status != models.MediaStatusApproved && media.UploaderID != userID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Access denied",
		})
	}

	if media.Status == models.MediaStatusRejected {
		return c.Status(fiber.StatusGone).JSON(fiber.Map{
			"error": "Media has been removed",
		})
	}

	// Serve the file
	return c.SendFile(media.StoragePath)
}

func isAllowedMediaType(contentType string) bool {
	allowed := []string{
		"image/jpeg",
		"image/png",
		"image/gif",
		"image/webp",
	}
	for _, a := range allowed {
		if strings.EqualFold(contentType, a) {
			return true
		}
	}
	return false
}

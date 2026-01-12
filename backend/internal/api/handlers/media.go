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

// Size limits per media type
const (
	MaxImageSize    = 10 * 1024 * 1024  // 10MB
	MaxVideoSize    = 100 * 1024 * 1024 // 100MB
	MaxAudioSize    = 50 * 1024 * 1024  // 50MB
	MaxDocumentSize = 50 * 1024 * 1024  // 50MB
)

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
	mediaType := getMediaType(contentType)
	if mediaType == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "File type not allowed. Allowed types: images (jpg, png, gif, webp), videos (mp4, webm, mov), audio (mp3, m4a, ogg, wav), documents (pdf, doc, docx, xls, xlsx)",
		})
	}

	// Validate file size based on media type
	maxSize := getMaxSize(mediaType)
	if file.Size > maxSize {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("File too large. Maximum size for %s is %dMB", mediaType, maxSize/1024/1024),
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
		MediaType:   mediaType,
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
		"id":         media.ID,
		"status":     media.Status,
		"media_type": media.MediaType,
		"message":    "File uploaded and pending moderation",
	})
}

func (h *MediaHandler) processModeration(media *models.Media) {
	var result *services.ScanResult
	var err error

	switch media.MediaType {
	case models.MediaTypeImage:
		// Scan images with GCP Vision
		result, err = h.moderationService.ScanImage(media.StoragePath)
	case models.MediaTypeVideo:
		// For videos, we could extract frames and scan them
		// For now, auto-approve with scan note
		result = &services.ScanResult{
			Status:    models.MediaStatusApproved,
			RawResult: "Video auto-approved (frame scanning not implemented)",
		}
	case models.MediaTypeAudio:
		// For audio, auto-approve (could implement speech-to-text moderation later)
		result = &services.ScanResult{
			Status:    models.MediaStatusApproved,
			RawResult: "Audio auto-approved",
		}
	case models.MediaTypeDocument:
		// For documents, auto-approve (could implement PDF preview scanning later)
		result = &services.ScanResult{
			Status:    models.MediaStatusApproved,
			RawResult: "Document auto-approved",
		}
	default:
		result = &services.ScanResult{
			Status:    models.MediaStatusReview,
			RawResult: "Unknown media type",
		}
	}

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

// getMediaType returns the media type category for a given content type
func getMediaType(contentType string) models.MediaType {
	ct := strings.ToLower(contentType)

	// Image types
	imageTypes := []string{
		"image/jpeg", "image/png", "image/gif", "image/webp",
	}
	for _, t := range imageTypes {
		if ct == t {
			return models.MediaTypeImage
		}
	}

	// Video types
	videoTypes := []string{
		"video/mp4", "video/webm", "video/quicktime", "video/x-msvideo",
	}
	for _, t := range videoTypes {
		if ct == t {
			return models.MediaTypeVideo
		}
	}

	// Audio types
	audioTypes := []string{
		"audio/mpeg", "audio/mp4", "audio/aac", "audio/ogg",
		"audio/webm", "audio/wav", "audio/x-wav",
	}
	for _, t := range audioTypes {
		if ct == t {
			return models.MediaTypeAudio
		}
	}

	// Document types
	documentTypes := []string{
		"application/pdf",
		"application/msword",
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		"application/vnd.ms-excel",
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		"text/plain",
	}
	for _, t := range documentTypes {
		if ct == t {
			return models.MediaTypeDocument
		}
	}

	return "" // Unknown/not allowed
}

// getMaxSize returns the maximum file size for a given media type
func getMaxSize(mediaType models.MediaType) int64 {
	switch mediaType {
	case models.MediaTypeImage:
		return MaxImageSize
	case models.MediaTypeVideo:
		return MaxVideoSize
	case models.MediaTypeAudio:
		return MaxAudioSize
	case models.MediaTypeDocument:
		return MaxDocumentSize
	default:
		return MaxImageSize // Default to image size
	}
}

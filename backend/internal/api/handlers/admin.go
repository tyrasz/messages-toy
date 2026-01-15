package handlers

import (
	"os"

	"github.com/gofiber/fiber/v2"
	"messenger/internal/api/middleware"
	"messenger/internal/database"
	"messenger/internal/models"
)

type AdminHandler struct{}

func NewAdminHandler() *AdminHandler {
	return &AdminHandler{}
}

// GetPendingReview returns media items pending human review
// Note: ModeratorRequired middleware handles role verification
func (h *AdminHandler) GetPendingReview(c *fiber.Ctx) error {

	var media []models.Media
	if err := database.DB.
		Where("status = ?", models.MediaStatusReview).
		Order("created_at ASC").
		Limit(50).
		Find(&media).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch pending reviews",
		})
	}

	return c.JSON(fiber.Map{
		"items": media,
		"count": len(media),
	})
}

type ReviewInput struct {
	Action string `json:"action"` // "approve" or "reject"
	Reason string `json:"reason,omitempty"`
}

// Review allows admin to approve or reject flagged media
// Note: ModeratorRequired middleware handles role verification
func (h *AdminHandler) Review(c *fiber.Ctx) error {
	reviewerID := middleware.GetUserID(c)
	mediaID := c.Params("id")

	var input ReviewInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if input.Action != "approve" && input.Action != "reject" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Action must be 'approve' or 'reject'",
		})
	}

	var media models.Media
	if err := database.DB.First(&media, "id = ?", mediaID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Media not found",
		})
	}

	if media.Status != models.MediaStatusReview {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Media is not pending review",
		})
	}

	if input.Action == "approve" {
		// Move to approved storage
		approvedDir := "./uploads/approved"
		os.MkdirAll(approvedDir, 0755)

		newPath := approvedDir + "/" + media.Filename
		if err := os.Rename(media.StoragePath, newPath); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to move file",
			})
		}

		database.DB.Model(&media).Updates(map[string]interface{}{
			"status":       models.MediaStatusApproved,
			"storage_path": newPath,
			"url":          "/media/" + media.ID,
			"scan_result":  media.ScanResult + ` | Manually approved by ` + reviewerID,
		})
	} else {
		// Delete the file
		os.Remove(media.StoragePath)

		database.DB.Model(&media).Updates(map[string]interface{}{
			"status":       models.MediaStatusRejected,
			"storage_path": "",
			"scan_result":  media.ScanResult + ` | Manually rejected by ` + reviewerID + ": " + input.Reason,
		})

		// TODO: Log for NCMEC reporting if applicable
		// TODO: Consider account suspension based on severity
	}

	return c.JSON(fiber.Map{
		"message": "Review completed",
		"status":  media.Status,
	})
}

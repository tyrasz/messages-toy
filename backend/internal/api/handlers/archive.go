package handlers

import (
	"github.com/gofiber/fiber/v2"
	"messenger/internal/api/middleware"
	"messenger/internal/database"
	"messenger/internal/models"
)

type ArchiveHandler struct{}

func NewArchiveHandler() *ArchiveHandler {
	return &ArchiveHandler{}
}

type ArchiveRequest struct {
	OtherUserID *string `json:"other_user_id,omitempty"`
	GroupID     *string `json:"group_id,omitempty"`
}

// Archive archives a conversation
func (h *ArchiveHandler) Archive(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	var req ArchiveRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if req.OtherUserID == nil && req.GroupID == nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Either other_user_id or group_id is required",
		})
	}

	archive, err := models.ArchiveConversation(database.DB, userID, req.OtherUserID, req.GroupID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to archive conversation",
		})
	}

	return c.JSON(fiber.Map{
		"archived":    true,
		"archived_at": archive.ArchivedAt,
	})
}

// Unarchive removes a conversation from archives
func (h *ArchiveHandler) Unarchive(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	otherUserID := c.Query("other_user_id")
	groupID := c.Query("group_id")

	if otherUserID == "" && groupID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Either other_user_id or group_id is required",
		})
	}

	var otherUserPtr, groupPtr *string
	if otherUserID != "" {
		otherUserPtr = &otherUserID
	}
	if groupID != "" {
		groupPtr = &groupID
	}

	if err := models.UnarchiveConversation(database.DB, userID, otherUserPtr, groupPtr); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to unarchive conversation",
		})
	}

	return c.JSON(fiber.Map{
		"archived": false,
	})
}

// List returns all archived conversations
func (h *ArchiveHandler) List(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	archives, err := models.GetArchivedConversations(database.DB, userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get archived conversations",
		})
	}

	var results []map[string]interface{}
	for _, a := range archives {
		item := map[string]interface{}{
			"id":          a.ID,
			"archived_at": a.ArchivedAt,
		}
		if a.GroupID != nil && a.Group != nil {
			item["type"] = "group"
			item["group"] = map[string]interface{}{
				"id":   a.Group.ID,
				"name": a.Group.Name,
			}
		} else if a.OtherUserID != nil && a.OtherUser != nil {
			item["type"] = "dm"
			item["user"] = map[string]interface{}{
				"id":           a.OtherUser.ID,
				"username":     a.OtherUser.Username,
				"display_name": a.OtherUser.DisplayName,
			}
		}
		results = append(results, item)
	}

	return c.JSON(fiber.Map{
		"archived": results,
	})
}

// IsArchived checks if a conversation is archived
func (h *ArchiveHandler) IsArchived(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	otherUserID := c.Query("other_user_id")
	groupID := c.Query("group_id")

	if otherUserID == "" && groupID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Either other_user_id or group_id is required",
		})
	}

	var otherUserPtr, groupPtr *string
	if otherUserID != "" {
		otherUserPtr = &otherUserID
	}
	if groupID != "" {
		groupPtr = &groupID
	}

	archived := models.IsConversationArchived(database.DB, userID, otherUserPtr, groupPtr)
	return c.JSON(fiber.Map{
		"archived": archived,
	})
}

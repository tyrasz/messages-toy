package handlers

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"messenger/internal/api/middleware"
	"messenger/internal/database"
	"messenger/internal/models"
)

type StarredHandler struct{}

func NewStarredHandler() *StarredHandler {
	return &StarredHandler{}
}

// List returns user's starred messages
func (h *StarredHandler) List(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	// Pagination
	limit, _ := strconv.Atoi(c.Query("limit", "50"))
	offset, _ := strconv.Atoi(c.Query("offset", "0"))

	if limit > 100 {
		limit = 100
	}

	starred, err := models.GetStarredMessages(database.DB, userID, limit, offset)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch starred messages",
		})
	}

	// Enrich with conversation info
	var results []fiber.Map
	for _, s := range starred {
		result := fiber.Map{
			"id":         s.ID,
			"message":    s.Message,
			"starred_at": s.CreatedAt,
		}

		// Add conversation context
		if s.Message.GroupID != nil {
			var group models.Group
			if err := database.DB.First(&group, "id = ?", *s.Message.GroupID).Error; err == nil {
				result["group"] = fiber.Map{
					"id":   group.ID,
					"name": group.Name,
				}
			}
		} else if s.Message.RecipientID != nil {
			// DM - get the other user
			otherUserID := s.Message.SenderID
			if s.Message.SenderID == userID {
				otherUserID = *s.Message.RecipientID
			}
			var otherUser models.User
			if err := database.DB.First(&otherUser, "id = ?", otherUserID).Error; err == nil {
				result["user"] = otherUser.ToResponse(false)
			}
		}

		results = append(results, result)
	}

	return c.JSON(fiber.Map{
		"starred": results,
		"limit":   limit,
		"offset":  offset,
	})
}

// Star adds a message to starred
func (h *StarredHandler) Star(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	messageID := c.Params("messageId")

	// Verify user has access to this message
	var message models.Message
	if err := database.DB.First(&message, "id = ?", messageID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Message not found",
		})
	}

	// Check access - user must be sender, recipient, or group member
	hasAccess := message.SenderID == userID
	if !hasAccess && message.RecipientID != nil && *message.RecipientID == userID {
		hasAccess = true
	}
	if !hasAccess && message.GroupID != nil {
		var count int64
		database.DB.Model(&models.GroupMember{}).
			Where("group_id = ? AND user_id = ?", *message.GroupID, userID).
			Count(&count)
		hasAccess = count > 0
	}

	if !hasAccess {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Access denied",
		})
	}

	starred, err := models.StarMessage(database.DB, userID, messageID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to star message",
		})
	}

	return c.JSON(fiber.Map{
		"starred":    true,
		"id":         starred.ID,
		"message_id": messageID,
	})
}

// Unstar removes a message from starred
func (h *StarredHandler) Unstar(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	messageID := c.Params("messageId")

	if err := models.UnstarMessage(database.DB, userID, messageID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to unstar message",
		})
	}

	return c.JSON(fiber.Map{
		"starred":    false,
		"message_id": messageID,
	})
}

// IsStarred checks if a message is starred
func (h *StarredHandler) IsStarred(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	messageID := c.Params("messageId")

	isStarred := models.IsMessageStarred(database.DB, userID, messageID)

	return c.JSON(fiber.Map{
		"starred":    isStarred,
		"message_id": messageID,
	})
}

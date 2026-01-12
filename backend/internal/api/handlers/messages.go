package handlers

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"messenger/internal/api/middleware"
	"messenger/internal/database"
	"messenger/internal/models"
)

type MessagesHandler struct{}

func NewMessagesHandler() *MessagesHandler {
	return &MessagesHandler{}
}

func (h *MessagesHandler) GetHistory(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	otherUserID := c.Params("userId")

	// Pagination
	limit, _ := strconv.Atoi(c.Query("limit", "50"))
	offset, _ := strconv.Atoi(c.Query("offset", "0"))

	if limit > 100 {
		limit = 100
	}

	var messages []models.Message
	err := database.DB.
		Preload("Media").
		Where(
			"group_id IS NULL AND ((sender_id = ? AND recipient_id = ?) OR (sender_id = ? AND recipient_id = ?))",
			userID, otherUserID, otherUserID, userID,
		).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&messages).Error

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch messages",
		})
	}

	// Mark messages as read
	database.DB.Model(&models.Message{}).
		Where("sender_id = ? AND recipient_id = ? AND status != ?", otherUserID, userID, models.MessageStatusRead).
		Update("status", models.MessageStatusRead)

	return c.JSON(fiber.Map{
		"messages": messages,
		"limit":    limit,
		"offset":   offset,
	})
}

func (h *MessagesHandler) GetConversations(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	// Raw query to get DM conversations with latest message (exclude group messages)
	rows, err := database.DB.Raw(`
		SELECT
			CASE
				WHEN sender_id = ? THEN recipient_id
				ELSE sender_id
			END as other_user_id,
			MAX(created_at) as last_message_time
		FROM messages
		WHERE group_id IS NULL AND (sender_id = ? OR recipient_id = ?)
		GROUP BY other_user_id
		ORDER BY last_message_time DESC
	`, userID, userID, userID).Rows()

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch conversations",
		})
	}
	defer rows.Close()

	var result []fiber.Map
	for rows.Next() {
		var otherUserID string
		var lastMessageTime string
		rows.Scan(&otherUserID, &lastMessageTime)

		// Get last message
		var lastMessage models.Message
		database.DB.Preload("Media").
			Where("(sender_id = ? AND recipient_id = ?) OR (sender_id = ? AND recipient_id = ?)",
				userID, otherUserID, otherUserID, userID).
			Order("created_at DESC").
			First(&lastMessage)

		// Get unread count
		var unreadCount int64
		database.DB.Model(&models.Message{}).
			Where("sender_id = ? AND recipient_id = ? AND status != ?", otherUserID, userID, models.MessageStatusRead).
			Count(&unreadCount)

		// Get other user
		var otherUser models.User
		database.DB.First(&otherUser, "id = ?", otherUserID)

		result = append(result, fiber.Map{
			"user":         otherUser.ToResponse(false), // TODO: check online status
			"last_message": lastMessage,
			"unread_count": unreadCount,
		})
	}

	return c.JSON(fiber.Map{
		"conversations": result,
	})
}

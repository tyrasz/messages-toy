package handlers

import (
	"encoding/json"

	"github.com/gofiber/fiber/v2"
	"messenger/internal/api/middleware"
	"messenger/internal/database"
	"messenger/internal/models"
	"messenger/internal/websocket"
)

type ReadReceiptHandler struct {
	hub *websocket.Hub
}

func NewReadReceiptHandler(hub *websocket.Hub) *ReadReceiptHandler {
	return &ReadReceiptHandler{hub: hub}
}

type MarkReadRequest struct {
	MessageIDs []string `json:"message_ids"`
	GroupID    *string  `json:"group_id,omitempty"`
}

// MarkRead marks messages as read
func (h *ReadReceiptHandler) MarkRead(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	var req MarkReadRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if len(req.MessageIDs) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "message_ids is required",
		})
	}

	// Mark messages as read
	if err := models.MarkMessagesAsRead(database.DB, userID, req.MessageIDs); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to mark messages as read",
		})
	}

	// Broadcast read receipts
	h.broadcastReadReceipts(userID, req.MessageIDs, req.GroupID)

	return c.JSON(fiber.Map{
		"success": true,
	})
}

// GetReceipts returns read receipts for a message
func (h *ReadReceiptHandler) GetReceipts(c *fiber.Ctx) error {
	messageID := c.Params("messageId")

	receipts, err := models.GetReadReceipts(database.DB, messageID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get read receipts",
		})
	}

	return c.JSON(fiber.Map{
		"receipts": receipts,
	})
}

// GetUnreadCount returns the number of unread messages
func (h *ReadReceiptHandler) GetUnreadCount(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	otherUserID := c.Query("other_user_id")
	groupID := c.Query("group_id")

	var otherUserPtr, groupPtr *string
	if otherUserID != "" {
		otherUserPtr = &otherUserID
	}
	if groupID != "" {
		groupPtr = &groupID
	}

	count, err := models.GetUnreadCount(database.DB, userID, groupPtr, otherUserPtr)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get unread count",
		})
	}

	return c.JSON(fiber.Map{
		"unread_count": count,
	})
}

func (h *ReadReceiptHandler) broadcastReadReceipts(userID string, messageIDs []string, groupID *string) {
	// Get message details to find senders
	var messages []models.Message
	database.DB.Where("id IN ?", messageIDs).Find(&messages)

	event := map[string]interface{}{
		"type":        "messages_read",
		"reader_id":   userID,
		"message_ids": messageIDs,
	}
	if groupID != nil {
		event["group_id"] = *groupID
	}

	eventBytes, _ := json.Marshal(event)

	if groupID != nil {
		// Broadcast to all group members
		h.hub.SendToGroup(*groupID, userID, eventBytes)
	} else {
		// Send to message senders (for DMs)
		sentTo := make(map[string]bool)
		for _, msg := range messages {
			if msg.SenderID != userID && !sentTo[msg.SenderID] {
				h.hub.SendToUser(msg.SenderID, eventBytes)
				sentTo[msg.SenderID] = true
			}
		}
	}
}

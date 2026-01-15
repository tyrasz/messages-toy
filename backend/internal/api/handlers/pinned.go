package handlers

import (
	"encoding/json"

	"github.com/gofiber/fiber/v2"
	"messenger/internal/api/middleware"
	"messenger/internal/database"
	"messenger/internal/models"
	"messenger/internal/websocket"
)

type PinnedHandler struct {
	hub *websocket.Hub
}

func NewPinnedHandler(hub *websocket.Hub) *PinnedHandler {
	return &PinnedHandler{hub: hub}
}

type PinMessageRequest struct {
	MessageID   string  `json:"message_id"`
	GroupID     *string `json:"group_id,omitempty"`
	OtherUserID *string `json:"other_user_id,omitempty"` // For DM pins
}

// Pin pins a message in a conversation or group
func (h *PinnedHandler) Pin(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	var req PinMessageRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if req.MessageID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Message ID is required",
		})
	}

	// Verify message exists and user has access
	var message models.Message
	if err := database.DB.First(&message, "id = ?", req.MessageID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Message not found",
		})
	}

	// Check access
	if req.GroupID != nil {
		// Verify group membership
		var membership models.GroupMember
		if err := database.DB.Where("group_id = ? AND user_id = ?", *req.GroupID, userID).First(&membership).Error; err != nil {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "You are not a member of this group",
			})
		}
		// Verify message belongs to this group
		if message.GroupID == nil || *message.GroupID != *req.GroupID {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Message does not belong to this group",
			})
		}
	} else if req.OtherUserID != nil {
		// Verify message belongs to this DM conversation
		if message.GroupID != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Message is a group message",
			})
		}
		// Check if user is part of this conversation
		isSender := message.SenderID == userID
		isRecipient := message.RecipientID != nil && *message.RecipientID == userID
		otherIsSender := message.SenderID == *req.OtherUserID
		otherIsRecipient := message.RecipientID != nil && *message.RecipientID == *req.OtherUserID

		if !((isSender && otherIsRecipient) || (isRecipient && otherIsSender)) {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "Message does not belong to this conversation",
			})
		}
	} else {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Either group_id or other_user_id is required",
		})
	}

	// Pin the message
	var pinned *models.PinnedMessage
	var err error

	if req.GroupID != nil {
		pinned, err = models.PinMessage(database.DB, req.MessageID, userID, req.GroupID, nil, nil)
	} else {
		pinned, err = models.PinMessage(database.DB, req.MessageID, userID, nil, &userID, req.OtherUserID)
	}

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to pin message",
		})
	}

	response := pinned.ToResponse()

	// Broadcast pin event
	h.broadcastPinEvent("message_pinned", &response, req.GroupID, &userID, req.OtherUserID)

	return c.JSON(response)
}

// Unpin removes a pinned message
func (h *PinnedHandler) Unpin(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	groupID := c.Query("group_id")
	otherUserID := c.Query("other_user_id")

	if groupID == "" && otherUserID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Either group_id or other_user_id is required",
		})
	}

	// Check access
	if groupID != "" {
		var membership models.GroupMember
		if err := database.DB.Where("group_id = ? AND user_id = ?", groupID, userID).First(&membership).Error; err != nil {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "You are not a member of this group",
			})
		}

		if err := models.UnpinMessage(database.DB, &groupID, nil, nil); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to unpin message",
			})
		}

		h.broadcastPinEvent("message_unpinned", nil, &groupID, nil, nil)
	} else {
		if err := models.UnpinMessage(database.DB, nil, &userID, &otherUserID); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to unpin message",
			})
		}

		h.broadcastPinEvent("message_unpinned", nil, nil, &userID, &otherUserID)
	}

	return c.JSON(fiber.Map{
		"success": true,
	})
}

// Get retrieves the pinned message for a conversation or group
func (h *PinnedHandler) Get(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	groupID := c.Query("group_id")
	otherUserID := c.Query("other_user_id")

	if groupID == "" && otherUserID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Either group_id or other_user_id is required",
		})
	}

	var pinned *models.PinnedMessage
	var err error

	if groupID != "" {
		// Verify membership
		var membership models.GroupMember
		if err := database.DB.Where("group_id = ? AND user_id = ?", groupID, userID).First(&membership).Error; err != nil {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "You are not a member of this group",
			})
		}
		pinned, err = models.GetPinnedMessage(database.DB, &groupID, nil, nil)
	} else {
		pinned, err = models.GetPinnedMessage(database.DB, nil, &userID, &otherUserID)
	}

	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "No pinned message",
		})
	}

	return c.JSON(pinned.ToResponse())
}

func (h *PinnedHandler) broadcastPinEvent(eventType string, pinned *models.PinnedMessageResponse, groupID, userID, otherUserID *string) {
	event := map[string]interface{}{
		"type": eventType,
	}
	if pinned != nil {
		event["pinned"] = pinned
	}
	if groupID != nil {
		event["group_id"] = *groupID
	}

	eventBytes, _ := json.Marshal(event)

	if groupID != nil {
		h.hub.SendToGroup(*groupID, "", eventBytes)
	} else if userID != nil && otherUserID != nil {
		h.hub.SendToUser(*userID, eventBytes)
		h.hub.SendToUser(*otherUserID, eventBytes)
	}
}

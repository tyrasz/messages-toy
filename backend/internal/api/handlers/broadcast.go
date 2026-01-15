package handlers

import (
	"encoding/json"
	"time"

	"github.com/gofiber/fiber/v2"
	"messenger/internal/api/middleware"
	"messenger/internal/database"
	"messenger/internal/models"
	"messenger/internal/services"
	"messenger/internal/websocket"
)

type BroadcastHandler struct {
	hub *websocket.Hub
}

func NewBroadcastHandler(hub *websocket.Hub) *BroadcastHandler {
	return &BroadcastHandler{hub: hub}
}

type CreateBroadcastListRequest struct {
	Name         string   `json:"name"`
	RecipientIDs []string `json:"recipient_ids"`
}

// Create creates a new broadcast list
func (h *BroadcastHandler) Create(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	var req CreateBroadcastListRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if req.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Name is required",
		})
	}

	if len(req.RecipientIDs) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "At least one recipient is required",
		})
	}

	// Filter out blocked users
	validRecipients := make([]string, 0)
	for _, recipientID := range req.RecipientIDs {
		if recipientID == userID {
			continue // Skip self
		}
		if models.IsEitherBlocked(database.DB, userID, recipientID) {
			continue // Skip blocked
		}
		// Verify user exists
		var user models.User
		if err := database.DB.First(&user, "id = ?", recipientID).Error; err != nil {
			continue // Skip non-existent
		}
		validRecipients = append(validRecipients, recipientID)
	}

	if len(validRecipients) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "No valid recipients provided",
		})
	}

	list, err := models.CreateBroadcastList(database.DB, userID, req.Name, validRecipients)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create broadcast list",
		})
	}

	return c.Status(fiber.StatusCreated).JSON(h.formatListResponse(list))
}

// List returns all broadcast lists for the current user
func (h *BroadcastHandler) List(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	lists, err := models.GetBroadcastLists(database.DB, userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch broadcast lists",
		})
	}

	response := make([]fiber.Map, len(lists))
	for i, list := range lists {
		response[i] = h.formatListResponse(&list)
	}

	return c.JSON(fiber.Map{
		"broadcast_lists": response,
	})
}

// Get returns a specific broadcast list
func (h *BroadcastHandler) Get(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	listID := c.Params("id")

	list, err := models.GetBroadcastList(database.DB, listID, userID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Broadcast list not found",
		})
	}

	return c.JSON(h.formatListResponse(list))
}

// Delete removes a broadcast list
func (h *BroadcastHandler) Delete(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	listID := c.Params("id")

	// Verify ownership
	_, err := models.GetBroadcastList(database.DB, listID, userID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Broadcast list not found",
		})
	}

	if err := models.DeleteBroadcastList(database.DB, listID, userID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to delete broadcast list",
		})
	}

	return c.JSON(fiber.Map{
		"message": "Broadcast list deleted successfully",
	})
}

type UpdateBroadcastListRequest struct {
	Name string `json:"name,omitempty"`
}

// Update updates a broadcast list name
func (h *BroadcastHandler) Update(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	listID := c.Params("id")

	var req UpdateBroadcastListRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Verify ownership
	list, err := models.GetBroadcastList(database.DB, listID, userID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Broadcast list not found",
		})
	}

	if req.Name != "" {
		if err := database.DB.Model(list).Update("name", req.Name).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to update broadcast list",
			})
		}
		list.Name = req.Name
	}

	return c.JSON(h.formatListResponse(list))
}

type AddRecipientRequest struct {
	RecipientID string `json:"recipient_id"`
}

// AddRecipient adds a recipient to a broadcast list
func (h *BroadcastHandler) AddRecipient(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	listID := c.Params("id")

	var req AddRecipientRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if req.RecipientID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Recipient ID is required",
		})
	}

	// Verify ownership
	_, err := models.GetBroadcastList(database.DB, listID, userID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Broadcast list not found",
		})
	}

	// Can't add self
	if req.RecipientID == userID {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Cannot add yourself to broadcast list",
		})
	}

	// Check if blocked
	if models.IsEitherBlocked(database.DB, userID, req.RecipientID) {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Cannot add blocked user",
		})
	}

	// Verify recipient exists
	var recipient models.User
	if err := database.DB.First(&recipient, "id = ?", req.RecipientID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "User not found",
		})
	}

	if err := models.AddRecipientToBroadcastList(database.DB, listID, req.RecipientID); err != nil {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"error": "Recipient already in list",
		})
	}

	// Get updated list
	list, _ := models.GetBroadcastList(database.DB, listID, userID)

	return c.Status(fiber.StatusCreated).JSON(h.formatListResponse(list))
}

// RemoveRecipient removes a recipient from a broadcast list
func (h *BroadcastHandler) RemoveRecipient(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	listID := c.Params("id")
	recipientID := c.Params("recipientId")

	// Verify ownership
	_, err := models.GetBroadcastList(database.DB, listID, userID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Broadcast list not found",
		})
	}

	if err := models.RemoveRecipientFromBroadcastList(database.DB, listID, recipientID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to remove recipient",
		})
	}

	// Get updated list
	list, _ := models.GetBroadcastList(database.DB, listID, userID)

	return c.JSON(h.formatListResponse(list))
}

type SendBroadcastRequest struct {
	Content string  `json:"content"`
	MediaID *string `json:"media_id,omitempty"`
}

// Send sends a message to all recipients in a broadcast list
func (h *BroadcastHandler) Send(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	listID := c.Params("id")

	var req SendBroadcastRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if req.Content == "" && req.MediaID == nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Content or media_id is required",
		})
	}

	// Verify ownership and get list
	list, err := models.GetBroadcastList(database.DB, listID, userID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Broadcast list not found",
		})
	}

	if len(list.Recipients) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Broadcast list has no recipients",
		})
	}

	// Check if media is approved (if attached)
	if req.MediaID != nil {
		var media models.Media
		if err := database.DB.First(&media, "id = ?", *req.MediaID).Error; err != nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Media not found",
			})
		}
		if media.Status != models.MediaStatusApproved {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Media is not approved",
			})
		}
	}

	// Send to each recipient
	messagesSent := 0
	messageIDs := make([]string, 0)

	for _, recipient := range list.Recipients {
		// Skip if blocked
		if models.IsEitherBlocked(database.DB, userID, recipient.RecipientID) {
			continue
		}

		// Create message
		message := &models.Message{
			SenderID:    userID,
			RecipientID: &recipient.RecipientID,
			Content:     req.Content,
			MediaID:     req.MediaID,
			Status:      models.MessageStatusSent,
		}

		if err := database.DB.Create(message).Error; err != nil {
			continue
		}

		messageIDs = append(messageIDs, message.ID)
		messagesSent++

		// Send via WebSocket if recipient is online
		if h.hub != nil && h.hub.IsOnline(recipient.RecipientID) {
			outMsg := map[string]interface{}{
				"type":       "message",
				"id":         message.ID,
				"from":       userID,
				"content":    req.Content,
				"media_id":   req.MediaID,
				"created_at": message.CreatedAt.Format(time.RFC3339),
			}
			msgBytes, _ := json.Marshal(outMsg)
			h.hub.SendToUser(recipient.RecipientID, msgBytes)
		} else {
			// Send push notification to offline users
			services.PushMessageToOfflineUser(database.DB, recipient.RecipientID, userID, req.Content, false, recipient.RecipientID)
		}
	}

	return c.JSON(fiber.Map{
		"success":       true,
		"messages_sent": messagesSent,
		"message_ids":   messageIDs,
		"broadcast_list": fiber.Map{
			"id":   list.ID,
			"name": list.Name,
		},
	})
}

// formatListResponse formats a broadcast list for the API response
func (h *BroadcastHandler) formatListResponse(list *models.BroadcastList) fiber.Map {
	recipients := make([]fiber.Map, len(list.Recipients))
	for i, r := range list.Recipients {
		online := false
		if h.hub != nil {
			online = h.hub.IsOnline(r.RecipientID)
		}
		recipients[i] = fiber.Map{
			"id":        r.ID,
			"user":      r.Recipient.ToResponse(online),
			"added_at":  r.CreatedAt,
		}
	}

	return fiber.Map{
		"id":              list.ID,
		"name":            list.Name,
		"recipient_count": len(list.Recipients),
		"recipients":      recipients,
		"created_at":      list.CreatedAt,
		"updated_at":      list.UpdatedAt,
	}
}

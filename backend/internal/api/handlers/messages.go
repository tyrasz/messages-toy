package handlers

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"messenger/internal/api/middleware"
	"messenger/internal/database"
	"messenger/internal/models"
	"messenger/internal/services"
	"messenger/internal/websocket"
)

type MessagesHandler struct {
	hub *websocket.Hub
}

func NewMessagesHandler(hub *websocket.Hub) *MessagesHandler {
	return &MessagesHandler{hub: hub}
}

func (h *MessagesHandler) GetHistory(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	otherUserID := c.Params("userId")

	// Pagination
	limit, _ := strconv.Atoi(c.Query("limit", "50"))
	offset, _ := strconv.Atoi(c.Query("offset", "0"))
	sinceStr := c.Query("since")

	if limit > 100 {
		limit = 100
	}

	query := database.DB.
		Preload("Media").
		Preload("ReplyTo").
		Where(
			"group_id IS NULL AND ((sender_id = ? AND recipient_id = ?) OR (sender_id = ? AND recipient_id = ?))",
			userID, otherUserID, otherUserID, userID,
		)

	// Filter by timestamp if provided (for sync)
	if sinceStr != "" {
		sinceTime, err := time.Parse(time.RFC3339, sinceStr)
		if err == nil {
			query = query.Where("created_at > ?", sinceTime)
		}
	}

	var messages []models.Message
	err := query.
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

		online := h.hub != nil && h.hub.IsOnline(otherUserID)
		result = append(result, fiber.Map{
			"user":         otherUser.ToResponse(online),
			"last_message": lastMessage,
			"unread_count": unreadCount,
		})
	}

	return c.JSON(fiber.Map{
		"conversations": result,
	})
}

// Search searches messages across all conversations
func (h *MessagesHandler) Search(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	query := c.Query("q")

	if query == "" || len(query) < 2 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Search query must be at least 2 characters",
		})
	}

	// Pagination
	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	offset, _ := strconv.Atoi(c.Query("offset", "0"))

	if limit > 50 {
		limit = 50
	}

	// Search in messages where user is sender or recipient (DMs) or member of group
	var messages []models.Message

	// Get user's group IDs
	var groupIDs []string
	database.DB.Model(&models.GroupMember{}).
		Where("user_id = ?", userID).
		Pluck("group_id", &groupIDs)

	// Search query
	searchPattern := "%" + query + "%"

	err := database.DB.
		Preload("Media").
		Where(`
			deleted_at IS NULL AND
			content LIKE ? AND (
				(group_id IS NULL AND (sender_id = ? OR recipient_id = ?)) OR
				(group_id IN (?))
			)
		`, searchPattern, userID, userID, groupIDs).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&messages).Error

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Search failed",
		})
	}

	// Enrich results with conversation info
	var results []fiber.Map
	for _, msg := range messages {
		result := fiber.Map{
			"message": msg,
		}

		if msg.GroupID != nil {
			// Group message - include group info
			var group models.Group
			if err := database.DB.First(&group, "id = ?", *msg.GroupID).Error; err == nil {
				result["group"] = fiber.Map{
					"id":   group.ID,
					"name": group.Name,
				}
			}
		} else if msg.RecipientID != nil {
			// DM - include other user info
			otherUserID := msg.SenderID
			if msg.SenderID == userID {
				otherUserID = *msg.RecipientID
			}
			var otherUser models.User
			if err := database.DB.First(&otherUser, "id = ?", otherUserID).Error; err == nil {
				result["user"] = otherUser.ToResponse(false)
			}
		}

		results = append(results, result)
	}

	return c.JSON(fiber.Map{
		"results": results,
		"query":   query,
		"limit":   limit,
		"offset":  offset,
	})
}

type ForwardMessageRequest struct {
	UserIDs  []string `json:"user_ids,omitempty"`
	GroupIDs []string `json:"group_ids,omitempty"`
}

// Forward forwards a message to one or more users/groups
func (h *MessagesHandler) Forward(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	messageID := c.Params("id")

	var req ForwardMessageRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if len(req.UserIDs) == 0 && len(req.GroupIDs) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "At least one user_id or group_id is required",
		})
	}

	// Find the original message
	var originalMessage models.Message
	if err := database.DB.Preload("Media").First(&originalMessage, "id = ?", messageID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Message not found",
		})
	}

	// Check if user has access to this message
	if originalMessage.GroupID != nil {
		// Group message - check membership
		var membership models.GroupMember
		if err := database.DB.Where("group_id = ? AND user_id = ?", *originalMessage.GroupID, userID).First(&membership).Error; err != nil {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "You don't have access to this message",
			})
		}
	} else {
		// DM - must be sender or recipient
		if originalMessage.SenderID != userID && (originalMessage.RecipientID == nil || *originalMessage.RecipientID != userID) {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "You don't have access to this message",
			})
		}
	}

	// Check if message is deleted
	if originalMessage.IsDeleted() {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Cannot forward a deleted message",
		})
	}

	// Get original sender's name for attribution
	var originalSender models.User
	if err := database.DB.First(&originalSender, "id = ?", originalMessage.SenderID).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get original sender",
		})
	}
	forwardedFrom := originalSender.DisplayName
	if forwardedFrom == "" {
		forwardedFrom = originalSender.Username
	}

	forwardedMessages := make([]fiber.Map, 0)
	var forwardErrors []string

	// Forward to users (DMs)
	for _, targetUserID := range req.UserIDs {
		if targetUserID == userID {
			continue // Skip self
		}

		// Check blocking
		if models.IsEitherBlocked(database.DB, userID, targetUserID) {
			forwardErrors = append(forwardErrors, "Cannot forward to blocked user")
			continue
		}

		// Verify user exists
		var targetUser models.User
		if err := database.DB.First(&targetUser, "id = ?", targetUserID).Error; err != nil {
			forwardErrors = append(forwardErrors, "User not found: "+targetUserID)
			continue
		}

		// Create forwarded message
		message := &models.Message{
			SenderID:      userID,
			RecipientID:   &targetUserID,
			Content:       originalMessage.Content,
			MediaID:       originalMessage.MediaID,
			ForwardedFrom: &forwardedFrom,
			Status:        models.MessageStatusSent,
		}

		if err := database.DB.Create(message).Error; err != nil {
			forwardErrors = append(forwardErrors, "Failed to forward to user")
			continue
		}

		// Send via WebSocket
		if h.hub != nil && h.hub.IsOnline(targetUserID) {
			outMsg := map[string]interface{}{
				"type":           "message",
				"id":             message.ID,
				"from":           userID,
				"to":             targetUserID,
				"content":        message.Content,
				"media_id":       message.MediaID,
				"forwarded_from": forwardedFrom,
				"created_at":     message.CreatedAt.Format(time.RFC3339),
			}
			msgBytes, _ := json.Marshal(outMsg)
			h.hub.SendToUser(targetUserID, msgBytes)

			// Update to delivered
			database.DB.Model(message).Update("status", models.MessageStatusDelivered)
		} else {
			// Push notification for offline user
			services.PushMessageToOfflineUser(database.DB, targetUserID, userID, message.Content, false, targetUserID)
		}

		forwardedMessages = append(forwardedMessages, fiber.Map{
			"id":         message.ID,
			"type":       "dm",
			"to_user_id": targetUserID,
			"created_at": message.CreatedAt,
		})
	}

	// Forward to groups
	for _, groupID := range req.GroupIDs {
		// Check membership
		var membership models.GroupMember
		if err := database.DB.Where("group_id = ? AND user_id = ?", groupID, userID).First(&membership).Error; err != nil {
			forwardErrors = append(forwardErrors, "Not a member of group: "+groupID)
			continue
		}

		// Create forwarded message
		message := &models.Message{
			SenderID:      userID,
			GroupID:       &groupID,
			Content:       originalMessage.Content,
			MediaID:       originalMessage.MediaID,
			ForwardedFrom: &forwardedFrom,
			Status:        models.MessageStatusSent,
		}

		if err := database.DB.Create(message).Error; err != nil {
			forwardErrors = append(forwardErrors, "Failed to forward to group")
			continue
		}

		// Broadcast to group members via WebSocket
		if h.hub != nil {
			outMsg := map[string]interface{}{
				"type":           "message",
				"id":             message.ID,
				"from":           userID,
				"group_id":       groupID,
				"content":        message.Content,
				"media_id":       message.MediaID,
				"forwarded_from": forwardedFrom,
				"created_at":     message.CreatedAt.Format(time.RFC3339),
			}
			msgBytes, _ := json.Marshal(outMsg)
			sentCount := h.hub.SendToGroup(groupID, userID, msgBytes)

			if sentCount > 0 {
				database.DB.Model(message).Update("status", models.MessageStatusDelivered)
			}

			// Push to offline group members
			offlineMembers := h.hub.GetOfflineGroupMemberIDs(groupID, userID)
			for _, memberID := range offlineMembers {
				services.PushMessageToOfflineUser(database.DB, memberID, userID, message.Content, true, groupID)
			}
		}

		forwardedMessages = append(forwardedMessages, fiber.Map{
			"id":         message.ID,
			"type":       "group",
			"group_id":   groupID,
			"created_at": message.CreatedAt,
		})
	}

	response := fiber.Map{
		"success":            len(forwardedMessages) > 0,
		"forwarded_count":    len(forwardedMessages),
		"forwarded_messages": forwardedMessages,
		"original_message": fiber.Map{
			"id":      originalMessage.ID,
			"content": originalMessage.Content,
		},
	}

	if len(forwardErrors) > 0 {
		response["errors"] = forwardErrors
	}

	return c.JSON(response)
}

// ExportMessage represents a message in the export format
type ExportMessage struct {
	ID            string     `json:"id"`
	SenderID      string     `json:"sender_id"`
	SenderName    string     `json:"sender_name"`
	Content       string     `json:"content"`
	MediaID       *string    `json:"media_id,omitempty"`
	MediaURL      string     `json:"media_url,omitempty"`
	MediaType     string     `json:"media_type,omitempty"`
	ForwardedFrom *string    `json:"forwarded_from,omitempty"`
	ReplyToID     *string    `json:"reply_to_id,omitempty"`
	EditedAt      *time.Time `json:"edited_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}

// ExportConversation represents the full export data
type ExportConversation struct {
	ExportedAt    time.Time       `json:"exported_at"`
	ExportedBy    string          `json:"exported_by"`
	Type          string          `json:"type"` // "dm" or "group"
	Participants  []ExportUser    `json:"participants"`
	GroupName     string          `json:"group_name,omitempty"`
	MessageCount  int             `json:"message_count"`
	DateRange     ExportDateRange `json:"date_range"`
	Messages      []ExportMessage `json:"messages"`
}

type ExportUser struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name,omitempty"`
}

type ExportDateRange struct {
	From *time.Time `json:"from,omitempty"`
	To   *time.Time `json:"to,omitempty"`
}

// Export exports a conversation (DM or group) in the requested format
func (h *MessagesHandler) Export(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	format := c.Query("format", "json") // json or txt

	// Get conversation identifiers
	otherUserID := c.Query("user_id")
	groupID := c.Query("group_id")

	if otherUserID == "" && groupID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Either user_id or group_id is required",
		})
	}

	if otherUserID != "" && groupID != "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Cannot specify both user_id and group_id",
		})
	}

	// Parse date range
	var fromDate, toDate *time.Time
	if from := c.Query("from"); from != "" {
		if t, err := time.Parse("2006-01-02", from); err == nil {
			fromDate = &t
		}
	}
	if to := c.Query("to"); to != "" {
		if t, err := time.Parse("2006-01-02", to); err == nil {
			endOfDay := t.Add(24*time.Hour - time.Second)
			toDate = &endOfDay
		}
	}

	var messages []models.Message
	var export ExportConversation
	export.ExportedAt = time.Now()
	export.DateRange = ExportDateRange{From: fromDate, To: toDate}

	// Get exporter info
	var exporter models.User
	database.DB.First(&exporter, "id = ?", userID)
	export.ExportedBy = exporter.Username

	if groupID != "" {
		// Export group conversation
		var membership models.GroupMember
		if err := database.DB.Where("group_id = ? AND user_id = ?", groupID, userID).First(&membership).Error; err != nil {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "You are not a member of this group",
			})
		}

		var group models.Group
		if err := database.DB.First(&group, "id = ?", groupID).Error; err != nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Group not found",
			})
		}

		export.Type = "group"
		export.GroupName = group.Name

		// Get group members as participants
		var members []models.GroupMember
		database.DB.Preload("User").Where("group_id = ?", groupID).Find(&members)
		for _, m := range members {
			export.Participants = append(export.Participants, ExportUser{
				ID:          m.User.ID,
				Username:    m.User.Username,
				DisplayName: m.User.DisplayName,
			})
		}

		// Build query for messages
		query := database.DB.Preload("Media").Where("group_id = ? AND deleted_at IS NULL", groupID)
		if fromDate != nil {
			query = query.Where("created_at >= ?", *fromDate)
		}
		if toDate != nil {
			query = query.Where("created_at <= ?", *toDate)
		}
		query.Order("created_at ASC").Find(&messages)

	} else {
		// Export DM conversation
		var otherUser models.User
		if err := database.DB.First(&otherUser, "id = ?", otherUserID).Error; err != nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "User not found",
			})
		}

		export.Type = "dm"
		export.Participants = []ExportUser{
			{ID: exporter.ID, Username: exporter.Username, DisplayName: exporter.DisplayName},
			{ID: otherUser.ID, Username: otherUser.Username, DisplayName: otherUser.DisplayName},
		}

		// Build query for messages
		query := database.DB.Preload("Media").Where(
			"group_id IS NULL AND deleted_at IS NULL AND ((sender_id = ? AND recipient_id = ?) OR (sender_id = ? AND recipient_id = ?))",
			userID, otherUserID, otherUserID, userID,
		)
		if fromDate != nil {
			query = query.Where("created_at >= ?", *fromDate)
		}
		if toDate != nil {
			query = query.Where("created_at <= ?", *toDate)
		}
		query.Order("created_at ASC").Find(&messages)
	}

	// Build username lookup for message senders
	usernames := make(map[string]string)
	for _, p := range export.Participants {
		name := p.DisplayName
		if name == "" {
			name = p.Username
		}
		usernames[p.ID] = name
	}

	// Convert messages to export format
	for _, msg := range messages {
		senderName := usernames[msg.SenderID]
		if senderName == "" {
			var sender models.User
			if database.DB.First(&sender, "id = ?", msg.SenderID).Error == nil {
				senderName = sender.DisplayName
				if senderName == "" {
					senderName = sender.Username
				}
				usernames[msg.SenderID] = senderName
			}
		}

		exportMsg := ExportMessage{
			ID:            msg.ID,
			SenderID:      msg.SenderID,
			SenderName:    senderName,
			Content:       msg.Content,
			MediaID:       msg.MediaID,
			ForwardedFrom: msg.ForwardedFrom,
			ReplyToID:     msg.ReplyToID,
			EditedAt:      msg.EditedAt,
			CreatedAt:     msg.CreatedAt,
		}

		// Include media info if present
		if msg.Media != nil {
			exportMsg.MediaURL = msg.Media.URL
			exportMsg.MediaType = msg.Media.ContentType
		}

		export.Messages = append(export.Messages, exportMsg)
	}

	export.MessageCount = len(export.Messages)

	// Return based on format
	if format == "txt" {
		return h.exportAsText(c, export)
	}

	// Default: JSON
	return c.JSON(export)
}

// exportAsText returns the conversation as a plain text file
func (h *MessagesHandler) exportAsText(c *fiber.Ctx, export ExportConversation) error {
	var text string

	// Header
	text += "=== Chat Export ===\n"
	text += "Exported: " + export.ExportedAt.Format("2006-01-02 15:04:05") + "\n"
	text += "Exported by: " + export.ExportedBy + "\n"

	if export.Type == "group" {
		text += "Group: " + export.GroupName + "\n"
	} else {
		text += "Conversation with: "
		for i, p := range export.Participants {
			if p.Username != export.ExportedBy {
				if i > 0 {
					text += ", "
				}
				name := p.DisplayName
				if name == "" {
					name = p.Username
				}
				text += name
			}
		}
		text += "\n"
	}

	text += "Messages: " + strconv.Itoa(export.MessageCount) + "\n"
	if export.DateRange.From != nil || export.DateRange.To != nil {
		text += "Date range: "
		if export.DateRange.From != nil {
			text += export.DateRange.From.Format("2006-01-02")
		} else {
			text += "start"
		}
		text += " to "
		if export.DateRange.To != nil {
			text += export.DateRange.To.Format("2006-01-02")
		} else {
			text += "now"
		}
		text += "\n"
	}
	text += "\n=== Messages ===\n\n"

	// Messages
	for _, msg := range export.Messages {
		// Timestamp and sender
		text += "[" + msg.CreatedAt.Format("2006-01-02 15:04:05") + "] "
		text += msg.SenderName

		if msg.ForwardedFrom != nil {
			text += " (forwarded from " + *msg.ForwardedFrom + ")"
		}
		if msg.EditedAt != nil {
			text += " (edited)"
		}
		text += ":\n"

		// Content
		if msg.Content != "" {
			text += msg.Content + "\n"
		}

		// Media
		if msg.MediaID != nil {
			text += "[Media: " + msg.MediaType + "]\n"
		}

		text += "\n"
	}

	// Set headers for file download
	c.Set("Content-Type", "text/plain; charset=utf-8")
	c.Set("Content-Disposition", "attachment; filename=\"chat-export.txt\"")

	return c.SendString(text)
}

// GetReactions returns all reactions for a message
func (h *MessagesHandler) GetReactions(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	messageID := c.Params("id")

	// Find the message
	var message models.Message
	if err := database.DB.First(&message, "id = ?", messageID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Message not found",
		})
	}

	// Check access
	if err := h.checkMessageAccess(userID, &message); err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Get reactions
	reactionInfo, err := models.GetMessageReactionInfo(database.DB, messageID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get reactions",
		})
	}

	// Get user details for each reaction
	type ReactionWithUsers struct {
		Emoji string                   `json:"emoji"`
		Count int                      `json:"count"`
		Users []map[string]interface{} `json:"users"`
	}

	var result []ReactionWithUsers
	for _, ri := range reactionInfo {
		rwu := ReactionWithUsers{
			Emoji: ri.Emoji,
			Count: ri.Count,
			Users: make([]map[string]interface{}, 0),
		}

		for _, uid := range ri.Users {
			var user models.User
			if database.DB.First(&user, "id = ?", uid).Error == nil {
				userInfo := map[string]interface{}{
					"id":       user.ID,
					"username": user.Username,
				}
				if user.DisplayName != "" {
					userInfo["display_name"] = user.DisplayName
				}
				userInfo["online"] = h.hub != nil && h.hub.IsOnline(user.ID)
				rwu.Users = append(rwu.Users, userInfo)
			}
		}
		result = append(result, rwu)
	}

	return c.JSON(fiber.Map{
		"message_id": messageID,
		"reactions":  result,
	})
}

type AddReactionRequest struct {
	Emoji string `json:"emoji"`
}

// AddReaction adds a reaction to a message
func (h *MessagesHandler) AddReaction(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	messageID := c.Params("id")

	var req AddReactionRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if req.Emoji == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Emoji is required",
		})
	}

	// Find the message
	var message models.Message
	if err := database.DB.First(&message, "id = ?", messageID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Message not found",
		})
	}

	// Check access
	if err := h.checkMessageAccess(userID, &message); err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Add reaction
	reaction, err := models.AddReaction(database.DB, messageID, userID, req.Emoji)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to add reaction",
		})
	}

	// Broadcast via WebSocket
	if h.hub != nil {
		reactionInfo, _ := models.GetMessageReactionInfo(database.DB, messageID)
		event := map[string]interface{}{
			"type":       "reaction",
			"message_id": messageID,
			"user_id":    userID,
			"emoji":      req.Emoji,
			"action":     "added",
			"reactions":  reactionInfo,
		}
		eventBytes, _ := json.Marshal(event)

		if message.IsGroupMessage() {
			h.hub.SendToGroup(*message.GroupID, "", eventBytes)
		} else if message.RecipientID != nil {
			h.hub.SendToUser(*message.RecipientID, eventBytes)
			h.hub.SendToUser(message.SenderID, eventBytes)
		}
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"id":         reaction.ID,
		"message_id": messageID,
		"user_id":    userID,
		"emoji":      req.Emoji,
		"created_at": reaction.CreatedAt,
	})
}

// RemoveReaction removes a user's reaction from a message
func (h *MessagesHandler) RemoveReaction(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	messageID := c.Params("id")

	// Find the message
	var message models.Message
	if err := database.DB.First(&message, "id = ?", messageID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Message not found",
		})
	}

	// Check access
	if err := h.checkMessageAccess(userID, &message); err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Remove reaction
	if err := models.RemoveReaction(database.DB, messageID, userID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to remove reaction",
		})
	}

	// Broadcast via WebSocket
	if h.hub != nil {
		reactionInfo, _ := models.GetMessageReactionInfo(database.DB, messageID)
		event := map[string]interface{}{
			"type":       "reaction",
			"message_id": messageID,
			"user_id":    userID,
			"action":     "removed",
			"reactions":  reactionInfo,
		}
		eventBytes, _ := json.Marshal(event)

		if message.IsGroupMessage() {
			h.hub.SendToGroup(*message.GroupID, "", eventBytes)
		} else if message.RecipientID != nil {
			h.hub.SendToUser(*message.RecipientID, eventBytes)
			h.hub.SendToUser(message.SenderID, eventBytes)
		}
	}

	return c.JSON(fiber.Map{
		"message":    "Reaction removed",
		"message_id": messageID,
	})
}

// checkMessageAccess verifies the user can access the message
func (h *MessagesHandler) checkMessageAccess(userID string, message *models.Message) error {
	if message.IsGroupMessage() {
		var membership models.GroupMember
		if err := database.DB.Where("group_id = ? AND user_id = ?", *message.GroupID, userID).First(&membership).Error; err != nil {
			return fiber.NewError(fiber.StatusForbidden, "You are not a member of this group")
		}
	} else {
		if message.SenderID != userID && (message.RecipientID == nil || *message.RecipientID != userID) {
			return fiber.NewError(fiber.StatusForbidden, "You don't have access to this message")
		}
	}
	return nil
}

// === Location Sharing ===

type SendLocationRequest struct {
	UserID       string   `json:"user_id,omitempty"`
	GroupID      string   `json:"group_id,omitempty"`
	Latitude     float64  `json:"latitude"`
	Longitude    float64  `json:"longitude"`
	LocationName *string  `json:"location_name,omitempty"`
}

// SendLocation sends a location message
func (h *MessagesHandler) SendLocation(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	var req SendLocationRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if req.UserID == "" && req.GroupID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Either user_id or group_id is required",
		})
	}

	if req.Latitude < -90 || req.Latitude > 90 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid latitude (must be between -90 and 90)",
		})
	}

	if req.Longitude < -180 || req.Longitude > 180 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid longitude (must be between -180 and 180)",
		})
	}

	var message *models.Message

	if req.GroupID != "" {
		// Group message
		var membership models.GroupMember
		if err := database.DB.Where("group_id = ? AND user_id = ?", req.GroupID, userID).First(&membership).Error; err != nil {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "You are not a member of this group",
			})
		}

		message = &models.Message{
			SenderID:     userID,
			GroupID:      &req.GroupID,
			Latitude:     &req.Latitude,
			Longitude:    &req.Longitude,
			LocationName: req.LocationName,
			Status:       models.MessageStatusSent,
		}
	} else {
		// DM
		if models.IsEitherBlocked(database.DB, userID, req.UserID) {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "Cannot send message to this user",
			})
		}

		message = &models.Message{
			SenderID:     userID,
			RecipientID:  &req.UserID,
			Latitude:     &req.Latitude,
			Longitude:    &req.Longitude,
			LocationName: req.LocationName,
			Status:       models.MessageStatusSent,
		}
	}

	if err := database.DB.Create(message).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to send location",
		})
	}

	// Send via WebSocket
	if h.hub != nil {
		outMsg := map[string]interface{}{
			"type":       "message",
			"id":         message.ID,
			"from":       userID,
			"latitude":   req.Latitude,
			"longitude":  req.Longitude,
			"created_at": message.CreatedAt.Format(time.RFC3339),
		}
		if req.LocationName != nil {
			outMsg["location_name"] = *req.LocationName
		}

		if req.GroupID != "" {
			outMsg["group_id"] = req.GroupID
			msgBytes, _ := json.Marshal(outMsg)
			h.hub.SendToGroup(req.GroupID, userID, msgBytes)
		} else {
			outMsg["to"] = req.UserID
			msgBytes, _ := json.Marshal(outMsg)
			if h.hub.SendToUser(req.UserID, msgBytes) {
				database.DB.Model(message).Update("status", models.MessageStatusDelivered)
			} else {
				services.PushMessageToOfflineUser(database.DB, req.UserID, userID, "📍 Shared a location", false, req.UserID)
			}
		}
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"id":            message.ID,
		"latitude":      req.Latitude,
		"longitude":     req.Longitude,
		"location_name": req.LocationName,
		"created_at":    message.CreatedAt,
	})
}

// === Scheduled Messages ===

type ScheduleMessageRequest struct {
	UserID      string  `json:"user_id,omitempty"`
	GroupID     string  `json:"group_id,omitempty"`
	Content     string  `json:"content,omitempty"`
	MediaID     *string `json:"media_id,omitempty"`
	ScheduledAt string  `json:"scheduled_at"` // RFC3339 format
}

// ScheduleMessage creates a scheduled message
func (h *MessagesHandler) ScheduleMessage(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	var req ScheduleMessageRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if req.UserID == "" && req.GroupID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Either user_id or group_id is required",
		})
	}

	if req.Content == "" && req.MediaID == nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Content or media_id is required",
		})
	}

	scheduledAt, err := time.Parse(time.RFC3339, req.ScheduledAt)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid scheduled_at format (use RFC3339)",
		})
	}

	if scheduledAt.Before(time.Now()) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Scheduled time must be in the future",
		})
	}

	// Validate target
	if req.GroupID != "" {
		var membership models.GroupMember
		if err := database.DB.Where("group_id = ? AND user_id = ?", req.GroupID, userID).First(&membership).Error; err != nil {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "You are not a member of this group",
			})
		}
	} else {
		if models.IsEitherBlocked(database.DB, userID, req.UserID) {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "Cannot send message to this user",
			})
		}
	}

	var recipientID, groupID *string
	if req.UserID != "" {
		recipientID = &req.UserID
	}
	if req.GroupID != "" {
		groupID = &req.GroupID
	}

	message, err := services.ScheduleMessage(userID, recipientID, groupID, req.Content, req.MediaID, scheduledAt)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to schedule message",
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"id":           message.ID,
		"content":      message.Content,
		"scheduled_at": scheduledAt.Format(time.RFC3339),
		"user_id":      req.UserID,
		"group_id":     req.GroupID,
	})
}

// GetScheduledMessages returns all pending scheduled messages for the user
func (h *MessagesHandler) GetScheduledMessages(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	messages, err := services.GetScheduledMessages(userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get scheduled messages",
		})
	}

	result := make([]fiber.Map, len(messages))
	for i, msg := range messages {
		item := fiber.Map{
			"id":           msg.ID,
			"content":      msg.Content,
			"scheduled_at": msg.ScheduledAt.Format(time.RFC3339),
			"created_at":   msg.CreatedAt.Format(time.RFC3339),
		}
		if msg.RecipientID != nil {
			item["user_id"] = *msg.RecipientID
		}
		if msg.GroupID != nil {
			item["group_id"] = *msg.GroupID
		}
		result[i] = item
	}

	return c.JSON(fiber.Map{
		"scheduled_messages": result,
	})
}

// CancelScheduledMessage cancels a scheduled message
func (h *MessagesHandler) CancelScheduledMessage(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	messageID := c.Params("id")

	if err := services.CancelScheduledMessage(messageID, userID); err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Scheduled message not found or already sent",
		})
	}

	return c.JSON(fiber.Map{
		"message": "Scheduled message cancelled",
	})
}

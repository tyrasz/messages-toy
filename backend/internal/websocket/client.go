package websocket

import (
	"encoding/json"
	"log"
	"time"

	"github.com/gofiber/contrib/websocket"
	"messenger/internal/database"
	"messenger/internal/models"
	"messenger/internal/services"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 65536
)

type Client struct {
	Hub      *Hub
	Conn     *websocket.Conn
	UserID   string
	Username string
	Send     chan []byte
}

func NewClient(hub *Hub, conn *websocket.Conn, userID, username string) *Client {
	return &Client{
		Hub:      hub,
		Conn:     conn,
		UserID:   userID,
		Username: username,
		Send:     make(chan []byte, 256),
	}
}

func (c *Client) ReadPump() {
	defer func() {
		c.Hub.unregister <- c
		c.Conn.Close()
	}()

	c.Conn.SetReadLimit(maxMessageSize)
	c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		c.handleMessage(message)
	}
}

func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.Conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}

		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) handleMessage(data []byte) {
	var base BaseMessage
	if err := json.Unmarshal(data, &base); err != nil {
		c.sendError("Invalid message format")
		return
	}

	switch base.Type {
	case "message":
		c.handleChatMessage(data)
	case "typing":
		c.handleTypingMessage(data)
	case "ack":
		c.handleAckMessage(data)
	case "message_edit":
		c.handleMessageEdit(data)
	case "message_delete":
		c.handleMessageDelete(data)
	case "reaction":
		c.handleReaction(data)
	default:
		c.sendError("Unknown message type")
	}
}

func (c *Client) handleChatMessage(data []byte) {
	var msg ChatMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		c.sendError("Invalid message format")
		return
	}

	// Must have either recipient (DM) or group_id (group message)
	if msg.To == "" && msg.GroupID == "" {
		c.sendError("Recipient or group_id is required")
		return
	}

	if msg.Content == "" && msg.MediaID == nil {
		c.sendError("Message content or media is required")
		return
	}

	// Check if media is approved (if attached)
	if msg.MediaID != nil {
		var media models.Media
		if err := database.DB.First(&media, "id = ?", *msg.MediaID).Error; err != nil {
			c.sendError("Media not found")
			return
		}
		if media.Status != models.MediaStatusApproved {
			c.sendError("Media is not approved for sending")
			return
		}
	}

	// Handle group message
	if msg.GroupID != "" {
		c.handleGroupMessage(msg)
		return
	}

	// Handle DM
	c.handleDirectMessage(msg)
}

func (c *Client) handleDirectMessage(msg ChatMessage) {
	// Check if messaging the bot
	if msg.To == services.BotUserID {
		c.handleBotMessage(msg)
		return
	}

	// Check if either user has blocked the other
	if models.IsEitherBlocked(database.DB, c.UserID, msg.To) {
		c.sendError("Cannot send message to this user")
		return
	}

	// Check for disappearing messages setting
	var expiresAt *time.Time
	disappearingSeconds := models.GetDisappearingTimer(database.DB, c.UserID, &msg.To, nil)
	if disappearingSeconds > 0 {
		t := time.Now().Add(time.Duration(disappearingSeconds) * time.Second)
		expiresAt = &t
	}

	// Save message to database
	message := models.Message{
		SenderID:      c.UserID,
		RecipientID:   &msg.To,
		Content:       msg.Content,
		MediaID:       msg.MediaID,
		ReplyToID:     msg.ReplyToID,
		ForwardedFrom: msg.ForwardedFrom,
		ExpiresAt:     expiresAt,
		Status:        models.MessageStatusSent,
	}

	if err := database.DB.Create(&message).Error; err != nil {
		c.sendError("Failed to save message")
		return
	}

	// Prepare outgoing message
	// Format expires_at if set
	var expiresAtStr *string
	if message.ExpiresAt != nil {
		s := message.ExpiresAt.Format(time.RFC3339)
		expiresAtStr = &s
	}

	outMsg := ChatMessage{
		Type:          "message",
		ID:            message.ID,
		From:          c.UserID,
		To:            msg.To,
		Content:       msg.Content,
		MediaID:       msg.MediaID,
		ReplyToID:     msg.ReplyToID,
		ForwardedFrom: msg.ForwardedFrom,
		ExpiresAt:     expiresAtStr,
		CreatedAt:     message.CreatedAt.Format(time.RFC3339),
	}

	// Include reply preview if replying to a message
	if msg.ReplyToID != nil {
		var replyMsg models.Message
		if err := database.DB.First(&replyMsg, "id = ?", *msg.ReplyToID).Error; err == nil {
			outMsg.ReplyTo = &ReplyPreview{
				ID:       replyMsg.ID,
				SenderID: replyMsg.SenderID,
				Content:  replyMsg.Content,
			}
		}
	}

	msgBytes, _ := json.Marshal(outMsg)

	// Send to recipient
	if c.Hub.SendToUser(msg.To, msgBytes) {
		// Update status to delivered
		database.DB.Model(&message).Update("status", models.MessageStatusDelivered)

		// Send delivery ack to sender
		ack := AckMessage{
			Type:      "ack",
			MessageID: message.ID,
			Status:    "delivered",
		}
		ackBytes, _ := json.Marshal(ack)
		c.Send <- ackBytes
	} else {
		// Recipient offline, send sent ack
		ack := AckMessage{
			Type:      "ack",
			MessageID: message.ID,
			Status:    "sent",
		}
		ackBytes, _ := json.Marshal(ack)
		c.Send <- ackBytes

		// Send push notification to offline user
		go services.PushMessageToOfflineUser(
			database.DB,
			msg.To,
			c.UserID,
			msg.Content,
			false,
			msg.To, // conversationID for DM is the other user's ID
		)
	}
}

func (c *Client) handleGroupMessage(msg ChatMessage) {
	// Check if user is a member of the group
	var membership models.GroupMember
	if err := database.DB.Where("group_id = ? AND user_id = ?", msg.GroupID, c.UserID).First(&membership).Error; err != nil {
		c.sendError("You are not a member of this group")
		return
	}

	// Check for disappearing messages setting
	var expiresAt *time.Time
	disappearingSeconds := models.GetDisappearingTimer(database.DB, c.UserID, nil, &msg.GroupID)
	if disappearingSeconds > 0 {
		t := time.Now().Add(time.Duration(disappearingSeconds) * time.Second)
		expiresAt = &t
	}

	// Save message to database
	message := models.Message{
		SenderID:      c.UserID,
		GroupID:       &msg.GroupID,
		Content:       msg.Content,
		MediaID:       msg.MediaID,
		ReplyToID:     msg.ReplyToID,
		ForwardedFrom: msg.ForwardedFrom,
		ExpiresAt:     expiresAt,
		Status:        models.MessageStatusSent,
	}

	if err := database.DB.Create(&message).Error; err != nil {
		c.sendError("Failed to save message")
		return
	}

	// Format expires_at if set
	var expiresAtStr *string
	if message.ExpiresAt != nil {
		s := message.ExpiresAt.Format(time.RFC3339)
		expiresAtStr = &s
	}

	// Prepare outgoing message
	outMsg := ChatMessage{
		Type:          "message",
		ID:            message.ID,
		From:          c.UserID,
		GroupID:       msg.GroupID,
		Content:       msg.Content,
		MediaID:       msg.MediaID,
		ReplyToID:     msg.ReplyToID,
		ForwardedFrom: msg.ForwardedFrom,
		ExpiresAt:     expiresAtStr,
		CreatedAt:     message.CreatedAt.Format(time.RFC3339),
	}

	// Include reply preview if replying to a message
	if msg.ReplyToID != nil {
		var replyMsg models.Message
		if err := database.DB.First(&replyMsg, "id = ?", *msg.ReplyToID).Error; err == nil {
			outMsg.ReplyTo = &ReplyPreview{
				ID:       replyMsg.ID,
				SenderID: replyMsg.SenderID,
				Content:  replyMsg.Content,
			}
		}
	}

	msgBytes, _ := json.Marshal(outMsg)

	// Broadcast to all group members (except sender)
	sentCount := c.Hub.SendToGroup(msg.GroupID, c.UserID, msgBytes)

	// Send ack to sender
	status := "sent"
	if sentCount > 0 {
		status = "delivered"
		database.DB.Model(&message).Update("status", models.MessageStatusDelivered)
	}

	ack := AckMessage{
		Type:      "ack",
		MessageID: message.ID,
		Status:    status,
	}
	ackBytes, _ := json.Marshal(ack)
	c.Send <- ackBytes

	// Send push notifications to offline group members
	offlineMembers := c.Hub.GetOfflineGroupMemberIDs(msg.GroupID, c.UserID)
	if len(offlineMembers) > 0 {
		go func() {
			for _, memberID := range offlineMembers {
				services.PushMessageToOfflineUser(
					database.DB,
					memberID,
					c.UserID,
					msg.Content,
					true,
					msg.GroupID,
				)
			}
		}()
	}
}

func (c *Client) handleTypingMessage(data []byte) {
	var msg TypingMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return
	}

	// Handle group typing indicator
	if msg.GroupID != "" {
		// Check if user is a member of the group
		var membership models.GroupMember
		if err := database.DB.Where("group_id = ? AND user_id = ?", msg.GroupID, c.UserID).First(&membership).Error; err != nil {
			return
		}

		// Forward typing indicator to all group members
		outMsg := struct {
			Type    string `json:"type"`
			GroupID string `json:"group_id"`
			From    string `json:"from"`
			Typing  bool   `json:"typing"`
		}{
			Type:    "typing",
			GroupID: msg.GroupID,
			From:    c.UserID,
			Typing:  msg.Typing,
		}

		msgBytes, _ := json.Marshal(outMsg)
		c.Hub.SendToGroup(msg.GroupID, c.UserID, msgBytes)
		return
	}

	// Handle DM typing indicator
	if msg.To == "" {
		return
	}

	// Don't forward typing indicator if blocked
	if models.IsEitherBlocked(database.DB, c.UserID, msg.To) {
		return
	}

	// Forward typing indicator to recipient
	outMsg := struct {
		Type   string `json:"type"`
		From   string `json:"from"`
		Typing bool   `json:"typing"`
	}{
		Type:   "typing",
		From:   c.UserID,
		Typing: msg.Typing,
	}

	msgBytes, _ := json.Marshal(outMsg)
	c.Hub.SendToUser(msg.To, msgBytes)
}

func (c *Client) handleAckMessage(data []byte) {
	var msg AckMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return
	}

	// Update message status (e.g., mark as read)
	if msg.Status == "read" {
		database.DB.Model(&models.Message{}).
			Where("id = ? AND recipient_id = ?", msg.MessageID, c.UserID).
			Update("status", models.MessageStatusRead)

		// Notify sender that message was read
		var message models.Message
		if err := database.DB.First(&message, "id = ?", msg.MessageID).Error; err == nil {
			readAck := AckMessage{
				Type:      "ack",
				MessageID: msg.MessageID,
				Status:    "read",
			}
			ackBytes, _ := json.Marshal(readAck)
			c.Hub.SendToUser(message.SenderID, ackBytes)
		}
	}
}

func (c *Client) handleMessageEdit(data []byte) {
	var msg EditMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		c.sendError("Invalid message format")
		return
	}

	if msg.MessageID == "" || msg.Content == "" {
		c.sendError("Message ID and content are required")
		return
	}

	// Find the message
	var message models.Message
	if err := database.DB.First(&message, "id = ?", msg.MessageID).Error; err != nil {
		c.sendError("Message not found")
		return
	}

	// Verify sender owns the message
	if message.SenderID != c.UserID {
		c.sendError("You can only edit your own messages")
		return
	}

	// Check if message is deleted
	if message.IsDeleted() {
		c.sendError("Cannot edit a deleted message")
		return
	}

	// Update the message
	now := time.Now()
	if err := database.DB.Model(&message).Updates(map[string]interface{}{
		"content":   msg.Content,
		"edited_at": now,
	}).Error; err != nil {
		c.sendError("Failed to edit message")
		return
	}

	// Prepare edit event
	editEvent := MessageEditedEvent{
		Type:      "message_edited",
		MessageID: message.ID,
		Content:   msg.Content,
		EditedAt:  now.Format(time.RFC3339),
	}
	eventBytes, _ := json.Marshal(editEvent)

	// Send to self
	c.Send <- eventBytes

	// Broadcast to recipient(s)
	if message.IsGroupMessage() {
		c.Hub.SendToGroup(*message.GroupID, c.UserID, eventBytes)
	} else if message.RecipientID != nil {
		c.Hub.SendToUser(*message.RecipientID, eventBytes)
	}
}

func (c *Client) handleMessageDelete(data []byte) {
	var msg DeleteMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		c.sendError("Invalid message format")
		return
	}

	if msg.MessageID == "" {
		c.sendError("Message ID is required")
		return
	}

	// Find the message
	var message models.Message
	if err := database.DB.First(&message, "id = ?", msg.MessageID).Error; err != nil {
		c.sendError("Message not found")
		return
	}

	// Handle "delete for me"
	if msg.DeleteFor == "me" {
		// Create a deletion record for this user
		deletion := models.MessageDeletion{
			MessageID: msg.MessageID,
			UserID:    c.UserID,
			DeletedAt: time.Now(),
		}
		if err := database.DB.Create(&deletion).Error; err != nil {
			// Might already exist, that's okay
		}

		// Send delete event only to self
		deleteEvent := MessageDeletedEvent{
			Type:      "message_deleted",
			MessageID: message.ID,
		}
		eventBytes, _ := json.Marshal(deleteEvent)
		c.Send <- eventBytes
		return
	}

	// Handle "delete for everyone" - only sender can do this
	if message.SenderID != c.UserID {
		c.sendError("You can only delete your own messages for everyone")
		return
	}

	// Soft delete the message
	now := time.Now()
	if err := database.DB.Model(&message).Update("deleted_at", now).Error; err != nil {
		c.sendError("Failed to delete message")
		return
	}

	// Prepare delete event
	deleteEvent := MessageDeletedEvent{
		Type:      "message_deleted",
		MessageID: message.ID,
	}
	eventBytes, _ := json.Marshal(deleteEvent)

	// Send to self
	c.Send <- eventBytes

	// Broadcast to recipient(s)
	if message.IsGroupMessage() {
		c.Hub.SendToGroup(*message.GroupID, c.UserID, eventBytes)
	} else if message.RecipientID != nil {
		c.Hub.SendToUser(*message.RecipientID, eventBytes)
	}
}

func (c *Client) handleReaction(data []byte) {
	var msg ReactionMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		c.sendError("Invalid reaction format")
		return
	}

	if msg.MessageID == "" {
		c.sendError("Message ID is required")
		return
	}

	if msg.Action != "add" && msg.Action != "remove" {
		c.sendError("Action must be 'add' or 'remove'")
		return
	}

	if msg.Action == "add" && msg.Emoji == "" {
		c.sendError("Emoji is required for add action")
		return
	}

	// Find the message
	var message models.Message
	if err := database.DB.First(&message, "id = ?", msg.MessageID).Error; err != nil {
		c.sendError("Message not found")
		return
	}

	// Check if user has access to this message
	if message.IsGroupMessage() {
		// Check group membership
		var membership models.GroupMember
		if err := database.DB.Where("group_id = ? AND user_id = ?", *message.GroupID, c.UserID).First(&membership).Error; err != nil {
			c.sendError("You don't have access to this message")
			return
		}
	} else {
		// For DMs, user must be sender or recipient
		if message.SenderID != c.UserID && (message.RecipientID == nil || *message.RecipientID != c.UserID) {
			c.sendError("You don't have access to this message")
			return
		}
	}

	var action string
	if msg.Action == "add" {
		_, err := models.AddReaction(database.DB, msg.MessageID, c.UserID, msg.Emoji)
		if err != nil {
			c.sendError("Failed to add reaction")
			return
		}
		action = "added"
	} else {
		if err := models.RemoveReaction(database.DB, msg.MessageID, c.UserID); err != nil {
			c.sendError("Failed to remove reaction")
			return
		}
		action = "removed"
	}

	// Get updated reaction info
	reactionInfo, _ := models.GetMessageReactionInfo(database.DB, msg.MessageID)

	// Convert to websocket type
	var wsReactions []ReactionInfo
	for _, ri := range reactionInfo {
		wsReactions = append(wsReactions, ReactionInfo{
			Emoji: ri.Emoji,
			Count: ri.Count,
			Users: ri.Users,
		})
	}

	// Prepare reaction event
	event := ReactionEvent{
		Type:      "reaction",
		MessageID: msg.MessageID,
		UserID:    c.UserID,
		Emoji:     msg.Emoji,
		Action:    action,
		Reactions: wsReactions,
	}
	eventBytes, _ := json.Marshal(event)

	// Send to self
	c.Send <- eventBytes

	// Broadcast to other participants
	if message.IsGroupMessage() {
		c.Hub.SendToGroup(*message.GroupID, c.UserID, eventBytes)
	} else if message.RecipientID != nil {
		// Send to the other person in the DM
		otherUserID := message.SenderID
		if message.SenderID == c.UserID {
			otherUserID = *message.RecipientID
		}
		c.Hub.SendToUser(otherUserID, eventBytes)
	}
}

func (c *Client) sendError(message string) {
	errMsg := ErrorMessage{
		Type:  "error",
		Error: message,
	}
	errBytes, _ := json.Marshal(errMsg)
	c.Send <- errBytes
}

// handleBotMessage handles messages sent to the AI bot
func (c *Client) handleBotMessage(msg ChatMessage) {
	botService := services.NewBotService()

	// Save the user's message to database
	userMessage := models.Message{
		SenderID:    c.UserID,
		RecipientID: &msg.To,
		Content:     msg.Content,
		Status:      models.MessageStatusSent,
	}

	if err := database.DB.Create(&userMessage).Error; err != nil {
		c.sendError("Failed to save message")
		return
	}

	// Send ack for user message
	userMsgOut := ChatMessage{
		Type:      "message",
		ID:        userMessage.ID,
		From:      c.UserID,
		To:        msg.To,
		Content:   msg.Content,
		CreatedAt: userMessage.CreatedAt.Format(time.RFC3339),
	}
	userMsgBytes, _ := json.Marshal(userMsgOut)
	c.Send <- userMsgBytes

	ack := AckMessage{
		Type:      "ack",
		MessageID: userMessage.ID,
		Status:    "delivered",
	}
	ackBytes, _ := json.Marshal(ack)
	c.Send <- ackBytes

	// Get conversation history for context
	var historyMessages []models.Message
	database.DB.Where(
		"(sender_id = ? AND recipient_id = ?) OR (sender_id = ? AND recipient_id = ?)",
		c.UserID, services.BotUserID,
		services.BotUserID, c.UserID,
	).Order("created_at ASC").Limit(20).Find(&historyMessages)

	// Convert to bot service message format
	var history []services.Message
	for _, m := range historyMessages {
		role := "user"
		if m.SenderID == services.BotUserID {
			role = "assistant"
		}
		history = append(history, services.Message{
			Role:    role,
			Content: m.Content,
		})
	}

	// Generate bot response asynchronously
	go func() {
		response, err := botService.GenerateResponse(msg.Content, history)
		if err != nil {
			log.Printf("Bot response error: %v", err)
			return
		}

		// Save bot response to database
		botMessage := models.Message{
			SenderID:    services.BotUserID,
			RecipientID: &c.UserID,
			Content:     response.Content,
			Status:      models.MessageStatusDelivered,
		}

		if err := database.DB.Create(&botMessage).Error; err != nil {
			log.Printf("Failed to save bot message: %v", err)
			return
		}

		// Send bot response to user
		botMsgOut := ChatMessage{
			Type:      "message",
			ID:        botMessage.ID,
			From:      services.BotUserID,
			To:        c.UserID,
			Content:   response.Content,
			CreatedAt: botMessage.CreatedAt.Format(time.RFC3339),
		}
		botMsgBytes, _ := json.Marshal(botMsgOut)

		// Send to user if still connected
		c.Hub.SendToUser(c.UserID, botMsgBytes)
	}()
}

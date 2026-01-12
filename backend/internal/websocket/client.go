package websocket

import (
	"encoding/json"
	"log"
	"time"

	"github.com/gofiber/contrib/websocket"
	"messenger/internal/database"
	"messenger/internal/models"
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
	// Save message to database
	message := models.Message{
		SenderID:    c.UserID,
		RecipientID: &msg.To,
		Content:     msg.Content,
		MediaID:     msg.MediaID,
		ReplyToID:   msg.ReplyToID,
		Status:      models.MessageStatusSent,
	}

	if err := database.DB.Create(&message).Error; err != nil {
		c.sendError("Failed to save message")
		return
	}

	// Prepare outgoing message
	outMsg := ChatMessage{
		Type:      "message",
		ID:        message.ID,
		From:      c.UserID,
		To:        msg.To,
		Content:   msg.Content,
		MediaID:   msg.MediaID,
		ReplyToID: msg.ReplyToID,
		CreatedAt: message.CreatedAt.Format(time.RFC3339),
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
	}
}

func (c *Client) handleGroupMessage(msg ChatMessage) {
	// Check if user is a member of the group
	var membership models.GroupMember
	if err := database.DB.Where("group_id = ? AND user_id = ?", msg.GroupID, c.UserID).First(&membership).Error; err != nil {
		c.sendError("You are not a member of this group")
		return
	}

	// Save message to database
	message := models.Message{
		SenderID:  c.UserID,
		GroupID:   &msg.GroupID,
		Content:   msg.Content,
		MediaID:   msg.MediaID,
		ReplyToID: msg.ReplyToID,
		Status:    models.MessageStatusSent,
	}

	if err := database.DB.Create(&message).Error; err != nil {
		c.sendError("Failed to save message")
		return
	}

	// Prepare outgoing message
	outMsg := ChatMessage{
		Type:      "message",
		ID:        message.ID,
		From:      c.UserID,
		GroupID:   msg.GroupID,
		Content:   msg.Content,
		MediaID:   msg.MediaID,
		ReplyToID: msg.ReplyToID,
		CreatedAt: message.CreatedAt.Format(time.RFC3339),
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
}

func (c *Client) handleTypingMessage(data []byte) {
	var msg TypingMessage
	if err := json.Unmarshal(data, &msg); err != nil {
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

func (c *Client) sendError(message string) {
	errMsg := ErrorMessage{
		Type:  "error",
		Error: message,
	}
	errBytes, _ := json.Marshal(errMsg)
	c.Send <- errBytes
}

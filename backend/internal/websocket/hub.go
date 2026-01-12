package websocket

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"messenger/internal/database"
	"messenger/internal/models"
)

type Hub struct {
	clients    map[string]*Client // userID -> Client
	register   chan *Client
	unregister chan *Client
	broadcast  chan []byte
	mutex      sync.RWMutex
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[string]*Client),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan []byte),
	}
}

func (h *Hub) Register(client *Client) {
	h.register <- client
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mutex.Lock()
			// Close existing connection if any
			if existing, ok := h.clients[client.UserID]; ok {
				existing.Conn.Close()
			}
			h.clients[client.UserID] = client
			h.mutex.Unlock()
			log.Printf("Client connected: %s", client.UserID)

			// Notify contacts about online status
			h.broadcastPresence(client.UserID, true)

		case client := <-h.unregister:
			h.mutex.Lock()
			if existing, ok := h.clients[client.UserID]; ok && existing == client {
				delete(h.clients, client.UserID)
				close(client.Send)
			}
			h.mutex.Unlock()
			log.Printf("Client disconnected: %s", client.UserID)

			// Update last seen
			database.DB.Model(&models.User{}).Where("id = ?", client.UserID).Update("last_seen", time.Now())

			// Notify contacts about offline status
			h.broadcastPresence(client.UserID, false)
		}
	}
}

func (h *Hub) IsOnline(userID string) bool {
	h.mutex.RLock()
	defer h.mutex.RUnlock()
	_, ok := h.clients[userID]
	return ok
}

func (h *Hub) GetClient(userID string) *Client {
	h.mutex.RLock()
	defer h.mutex.RUnlock()
	return h.clients[userID]
}

func (h *Hub) SendToUser(userID string, message []byte) bool {
	h.mutex.RLock()
	client, ok := h.clients[userID]
	h.mutex.RUnlock()

	if ok {
		select {
		case client.Send <- message:
			return true
		default:
			return false
		}
	}
	return false
}

// SendToGroup sends a message to all online members of a group
func (h *Hub) SendToGroup(groupID string, excludeUserID string, message []byte) int {
	// Get group members
	var members []models.GroupMember
	database.DB.Where("group_id = ?", groupID).Find(&members)

	sent := 0
	for _, member := range members {
		if member.UserID == excludeUserID {
			continue
		}
		if h.SendToUser(member.UserID, message) {
			sent++
		}
	}
	return sent
}

// GetGroupMemberIDs returns all member IDs of a group
func (h *Hub) GetGroupMemberIDs(groupID string) []string {
	var members []models.GroupMember
	database.DB.Where("group_id = ?", groupID).Find(&members)

	ids := make([]string, len(members))
	for i, m := range members {
		ids[i] = m.UserID
	}
	return ids
}

// GetOfflineGroupMemberIDs returns IDs of group members who are not currently connected
func (h *Hub) GetOfflineGroupMemberIDs(groupID string, excludeUserID string) []string {
	memberIDs := h.GetGroupMemberIDs(groupID)

	h.mutex.RLock()
	defer h.mutex.RUnlock()

	var offlineIDs []string
	for _, id := range memberIDs {
		if id == excludeUserID {
			continue
		}
		if _, online := h.clients[id]; !online {
			offlineIDs = append(offlineIDs, id)
		}
	}
	return offlineIDs
}

func (h *Hub) broadcastPresence(userID string, online bool) {
	// Get user's contacts
	var contacts []models.Contact
	database.DB.Where("contact_id = ?", userID).Find(&contacts)

	presenceMsg := PresenceMessage{
		Type:   "presence",
		UserID: userID,
		Online: online,
	}

	if !online {
		presenceMsg.LastSeen = time.Now().Format(time.RFC3339)
	}

	msgBytes, _ := json.Marshal(presenceMsg)

	// Notify each contact (unless blocked)
	for _, contact := range contacts {
		// Don't send presence to blocked users or users who blocked this user
		if models.IsEitherBlocked(database.DB, userID, contact.UserID) {
			continue
		}
		h.SendToUser(contact.UserID, msgBytes)
	}
}

// Message types for WebSocket communication

type BaseMessage struct {
	Type string `json:"type"`
}

type ChatMessage struct {
	Type      string  `json:"type"`
	ID        string  `json:"id,omitempty"`
	To        string  `json:"to,omitempty"`           // For DMs: recipient user ID
	GroupID   string  `json:"group_id,omitempty"`     // For groups: group ID
	From      string  `json:"from,omitempty"`         // Sender ID (for incoming messages)
	Content   string  `json:"content,omitempty"`
	MediaID   *string `json:"media_id,omitempty"`
	ReplyToID *string `json:"reply_to_id,omitempty"`  // ID of message being replied to
	ReplyTo   *ReplyPreview `json:"reply_to,omitempty"` // Preview of replied message
	CreatedAt string  `json:"created_at,omitempty"`
}

// ReplyPreview contains a summary of the message being replied to
type ReplyPreview struct {
	ID       string `json:"id"`
	SenderID string `json:"sender_id"`
	Content  string `json:"content,omitempty"`
}

type TypingMessage struct {
	Type   string `json:"type"`
	To     string `json:"to"`
	Typing bool   `json:"typing"`
}

type PresenceMessage struct {
	Type     string `json:"type"`
	UserID   string `json:"user_id"`
	Online   bool   `json:"online"`
	LastSeen string `json:"last_seen,omitempty"`
}

type AckMessage struct {
	Type      string `json:"type"`
	MessageID string `json:"message_id"`
	Status    string `json:"status"`
}

type ErrorMessage struct {
	Type  string `json:"type"`
	Error string `json:"error"`
}

// Message editing types
type EditMessage struct {
	Type      string `json:"type"`
	MessageID string `json:"message_id"`
	Content   string `json:"content"`
}

type MessageEditedEvent struct {
	Type      string `json:"type"`
	MessageID string `json:"message_id"`
	Content   string `json:"content"`
	EditedAt  string `json:"edited_at"`
}

// Message deletion types
type DeleteMessage struct {
	Type      string `json:"type"`
	MessageID string `json:"message_id"`
	DeleteFor string `json:"delete_for"` // "me" or "everyone"
}

type MessageDeletedEvent struct {
	Type      string `json:"type"`
	MessageID string `json:"message_id"`
}

// Reaction types
type ReactionMessage struct {
	Type      string `json:"type"`
	MessageID string `json:"message_id"`
	Emoji     string `json:"emoji,omitempty"` // Empty for remove
	Action    string `json:"action"`          // "add" or "remove"
}

type ReactionEvent struct {
	Type      string          `json:"type"`
	MessageID string          `json:"message_id"`
	UserID    string          `json:"user_id"`
	Emoji     string          `json:"emoji,omitempty"`
	Action    string          `json:"action"` // "added" or "removed"
	Reactions []ReactionInfo  `json:"reactions,omitempty"`
}

type ReactionInfo struct {
	Emoji string   `json:"emoji"`
	Count int      `json:"count"`
	Users []string `json:"users"`
}

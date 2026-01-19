package websocket

import (
	"encoding/json"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"messenger/internal/database"
	"messenger/internal/models"
)

func setupClientTestDB(t *testing.T) func() {
	var err error
	database.DB, err = gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Migrate required models
	database.DB.AutoMigrate(
		&models.User{},
		&models.Contact{},
		&models.Group{},
		&models.GroupMember{},
		&models.Block{},
		&models.Message{},
		&models.Media{},
		&models.Reaction{},
		&models.MessageDeletion{},
		&models.ConversationSettings{},
	)

	return func() {
		sqlDB, _ := database.DB.DB()
		sqlDB.Close()
	}
}

func createTestClientWithHub(userID string, hub *Hub) *Client {
	client := &Client{
		UserID: userID,
		Conn:   nil,
		Send:   make(chan []byte, 256),
		Hub:    hub,
	}
	return client
}

func TestClient_HandleMessage_InvalidJSON(t *testing.T) {
	cleanup := setupClientTestDB(t)
	defer cleanup()

	hub := NewHub()
	client := createTestClientWithHub("user1", hub)

	// Send invalid JSON
	client.handleMessage([]byte("not json"))

	// Should receive error
	select {
	case msg := <-client.Send:
		var errMsg ErrorMessage
		if err := json.Unmarshal(msg, &errMsg); err != nil {
			t.Fatalf("Failed to unmarshal error: %v", err)
		}
		if errMsg.Type != "error" {
			t.Errorf("Expected error type, got %s", errMsg.Type)
		}
	case <-time.After(time.Second):
		t.Error("Expected error message")
	}
}

func TestClient_HandleMessage_UnknownType(t *testing.T) {
	cleanup := setupClientTestDB(t)
	defer cleanup()

	hub := NewHub()
	client := createTestClientWithHub("user1", hub)

	// Send unknown message type
	msg := `{"type": "unknown_type"}`
	client.handleMessage([]byte(msg))

	// Should receive error
	select {
	case msg := <-client.Send:
		var errMsg ErrorMessage
		if err := json.Unmarshal(msg, &errMsg); err != nil {
			t.Fatalf("Failed to unmarshal error: %v", err)
		}
		if errMsg.Error != "Unknown message type" {
			t.Errorf("Expected 'Unknown message type', got %s", errMsg.Error)
		}
	case <-time.After(time.Second):
		t.Error("Expected error message")
	}
}

func TestClient_HandleChatMessage_NoRecipient(t *testing.T) {
	cleanup := setupClientTestDB(t)
	defer cleanup()

	hub := NewHub()
	client := createTestClientWithHub("user1", hub)

	// Send message without recipient
	msg := `{"type": "message", "content": "Hello"}`
	client.handleMessage([]byte(msg))

	// Should receive error
	select {
	case msg := <-client.Send:
		var errMsg ErrorMessage
		json.Unmarshal(msg, &errMsg)
		if errMsg.Error != "Recipient or group_id is required" {
			t.Errorf("Expected recipient error, got %s", errMsg.Error)
		}
	case <-time.After(time.Second):
		t.Error("Expected error message")
	}
}

func TestClient_HandleChatMessage_NoContent(t *testing.T) {
	cleanup := setupClientTestDB(t)
	defer cleanup()

	hub := NewHub()
	client := createTestClientWithHub("user1", hub)

	// Create recipient user
	user := &models.User{Username: "recipient"}
	database.DB.Create(user)

	// Send message without content
	msg := `{"type": "message", "to": "` + user.ID + `"}`
	client.handleMessage([]byte(msg))

	// Should receive error
	select {
	case msg := <-client.Send:
		var errMsg ErrorMessage
		json.Unmarshal(msg, &errMsg)
		if errMsg.Error != "Message content or media is required" {
			t.Errorf("Expected content error, got %s", errMsg.Error)
		}
	case <-time.After(time.Second):
		t.Error("Expected error message")
	}
}

func TestClient_HandleChatMessage_BlockedUser(t *testing.T) {
	cleanup := setupClientTestDB(t)
	defer cleanup()

	hub := NewHub()

	// Create users
	sender := &models.User{Username: "sender"}
	recipient := &models.User{Username: "recipient"}
	database.DB.Create(sender)
	database.DB.Create(recipient)

	// Block sender
	block := &models.Block{BlockerID: recipient.ID, BlockedID: sender.ID}
	database.DB.Create(block)

	client := createTestClientWithHub(sender.ID, hub)

	// Send message to blocked user
	msg := `{"type": "message", "to": "` + recipient.ID + `", "content": "Hello"}`
	client.handleMessage([]byte(msg))

	// Should receive error
	select {
	case msg := <-client.Send:
		var errMsg ErrorMessage
		json.Unmarshal(msg, &errMsg)
		if errMsg.Error != "Cannot send message to this user" {
			t.Errorf("Expected blocked error, got %s", errMsg.Error)
		}
	case <-time.After(time.Second):
		t.Error("Expected error message")
	}
}

func TestClient_HandleChatMessage_Success(t *testing.T) {
	cleanup := setupClientTestDB(t)
	defer cleanup()

	hub := NewHub()

	// Create users
	sender := &models.User{Username: "sender"}
	recipient := &models.User{Username: "recipient"}
	database.DB.Create(sender)
	database.DB.Create(recipient)

	// Register recipient in hub
	recipientClient := createTestClientWithHub(recipient.ID, hub)
	hub.mutex.Lock()
	hub.clients[recipient.ID] = recipientClient
	hub.mutex.Unlock()

	senderClient := createTestClientWithHub(sender.ID, hub)

	// Send message
	msg := `{"type": "message", "to": "` + recipient.ID + `", "content": "Hello!"}`
	senderClient.handleMessage([]byte(msg))

	// Sender should receive ack
	select {
	case ackData := <-senderClient.Send:
		var ack AckMessage
		json.Unmarshal(ackData, &ack)
		if ack.Type != "ack" {
			t.Errorf("Expected ack type, got %s", ack.Type)
		}
		if ack.Status != "delivered" {
			t.Errorf("Expected delivered status, got %s", ack.Status)
		}
	case <-time.After(time.Second):
		t.Error("Expected ack message")
	}

	// Recipient should receive message
	select {
	case msgData := <-recipientClient.Send:
		var chatMsg ChatMessage
		json.Unmarshal(msgData, &chatMsg)
		if chatMsg.Type != "message" {
			t.Errorf("Expected message type, got %s", chatMsg.Type)
		}
		if chatMsg.Content != "Hello!" {
			t.Errorf("Expected 'Hello!', got %s", chatMsg.Content)
		}
		if chatMsg.From != sender.ID {
			t.Errorf("Expected from %s, got %s", sender.ID, chatMsg.From)
		}
	case <-time.After(time.Second):
		t.Error("Expected chat message")
	}

	// Verify message was saved to database
	var savedMsg models.Message
	if err := database.DB.First(&savedMsg).Error; err != nil {
		t.Error("Message was not saved to database")
	}
}

func TestClient_HandleChatMessage_OfflineRecipient(t *testing.T) {
	cleanup := setupClientTestDB(t)
	defer cleanup()

	hub := NewHub()

	// Create users
	sender := &models.User{Username: "sender"}
	recipient := &models.User{Username: "recipient"}
	database.DB.Create(sender)
	database.DB.Create(recipient)

	// Recipient is NOT registered (offline)
	senderClient := createTestClientWithHub(sender.ID, hub)

	// Send message
	msg := `{"type": "message", "to": "` + recipient.ID + `", "content": "Hello offline!"}`
	senderClient.handleMessage([]byte(msg))

	// Sender should receive "sent" ack (not delivered)
	select {
	case ackData := <-senderClient.Send:
		var ack AckMessage
		json.Unmarshal(ackData, &ack)
		if ack.Status != "sent" {
			t.Errorf("Expected sent status for offline recipient, got %s", ack.Status)
		}
	case <-time.After(time.Second):
		t.Error("Expected ack message")
	}
}

func TestClient_HandleGroupMessage_NotMember(t *testing.T) {
	cleanup := setupClientTestDB(t)
	defer cleanup()

	hub := NewHub()

	// Create user and group
	user := &models.User{Username: "outsider"}
	creator := &models.User{Username: "creator"}
	database.DB.Create(user)
	database.DB.Create(creator)

	group := &models.Group{Name: "Test Group", CreatedBy: creator.ID}
	database.DB.Create(group)

	// User is NOT a member
	client := createTestClientWithHub(user.ID, hub)

	// Send group message
	msg := `{"type": "message", "group_id": "` + group.ID + `", "content": "Hello group!"}`
	client.handleMessage([]byte(msg))

	// Should receive error
	select {
	case msgData := <-client.Send:
		var errMsg ErrorMessage
		json.Unmarshal(msgData, &errMsg)
		if errMsg.Error != "You are not a member of this group" {
			t.Errorf("Expected not member error, got %s", errMsg.Error)
		}
	case <-time.After(time.Second):
		t.Error("Expected error message")
	}
}

func TestClient_HandleGroupMessage_Success(t *testing.T) {
	cleanup := setupClientTestDB(t)
	defer cleanup()

	hub := NewHub()

	// Create users
	user1 := &models.User{Username: "user1"}
	user2 := &models.User{Username: "user2"}
	user3 := &models.User{Username: "user3"}
	database.DB.Create(user1)
	database.DB.Create(user2)
	database.DB.Create(user3)

	// Create group with all members
	group := &models.Group{Name: "Test Group", CreatedBy: user1.ID}
	database.DB.Create(group)

	database.DB.Create(&models.GroupMember{GroupID: group.ID, UserID: user1.ID, Role: "admin"})
	database.DB.Create(&models.GroupMember{GroupID: group.ID, UserID: user2.ID, Role: "member"})
	database.DB.Create(&models.GroupMember{GroupID: group.ID, UserID: user3.ID, Role: "member"})

	// Register all users in hub
	client1 := createTestClientWithHub(user1.ID, hub)
	client2 := createTestClientWithHub(user2.ID, hub)
	client3 := createTestClientWithHub(user3.ID, hub)

	hub.mutex.Lock()
	hub.clients[user1.ID] = client1
	hub.clients[user2.ID] = client2
	hub.clients[user3.ID] = client3
	hub.mutex.Unlock()

	// User1 sends group message
	msg := `{"type": "message", "group_id": "` + group.ID + `", "content": "Hello everyone!"}`
	client1.handleMessage([]byte(msg))

	// User1 should receive ack
	select {
	case ackData := <-client1.Send:
		var ack AckMessage
		json.Unmarshal(ackData, &ack)
		if ack.Status != "delivered" {
			t.Errorf("Expected delivered status, got %s", ack.Status)
		}
	case <-time.After(time.Second):
		t.Error("Expected ack message")
	}

	// User2 should receive message
	select {
	case msgData := <-client2.Send:
		var chatMsg ChatMessage
		json.Unmarshal(msgData, &chatMsg)
		if chatMsg.Content != "Hello everyone!" {
			t.Errorf("User2 expected 'Hello everyone!', got %s", chatMsg.Content)
		}
	case <-time.After(time.Second):
		t.Error("User2 expected message")
	}

	// User3 should receive message
	select {
	case msgData := <-client3.Send:
		var chatMsg ChatMessage
		json.Unmarshal(msgData, &chatMsg)
		if chatMsg.Content != "Hello everyone!" {
			t.Errorf("User3 expected 'Hello everyone!', got %s", chatMsg.Content)
		}
	case <-time.After(time.Second):
		t.Error("User3 expected message")
	}
}

func TestClient_HandleTypingMessage_DM(t *testing.T) {
	cleanup := setupClientTestDB(t)
	defer cleanup()

	hub := NewHub()

	// Create users
	sender := &models.User{Username: "sender"}
	recipient := &models.User{Username: "recipient"}
	database.DB.Create(sender)
	database.DB.Create(recipient)

	// Register both
	senderClient := createTestClientWithHub(sender.ID, hub)
	recipientClient := createTestClientWithHub(recipient.ID, hub)

	hub.mutex.Lock()
	hub.clients[sender.ID] = senderClient
	hub.clients[recipient.ID] = recipientClient
	hub.mutex.Unlock()

	// Send typing indicator
	msg := `{"type": "typing", "to": "` + recipient.ID + `", "typing": true}`
	senderClient.handleMessage([]byte(msg))

	// Recipient should receive typing indicator
	select {
	case msgData := <-recipientClient.Send:
		var typingMsg map[string]interface{}
		json.Unmarshal(msgData, &typingMsg)
		if typingMsg["type"] != "typing" {
			t.Errorf("Expected typing type, got %v", typingMsg["type"])
		}
		if typingMsg["from"] != sender.ID {
			t.Errorf("Expected from %s, got %v", sender.ID, typingMsg["from"])
		}
		if typingMsg["typing"] != true {
			t.Errorf("Expected typing true, got %v", typingMsg["typing"])
		}
	case <-time.After(time.Second):
		t.Error("Expected typing message")
	}
}

func TestClient_HandleTypingMessage_Blocked(t *testing.T) {
	cleanup := setupClientTestDB(t)
	defer cleanup()

	hub := NewHub()

	// Create users
	sender := &models.User{Username: "sender"}
	recipient := &models.User{Username: "recipient"}
	database.DB.Create(sender)
	database.DB.Create(recipient)

	// Block sender
	database.DB.Create(&models.Block{BlockerID: recipient.ID, BlockedID: sender.ID})

	// Register both
	senderClient := createTestClientWithHub(sender.ID, hub)
	recipientClient := createTestClientWithHub(recipient.ID, hub)

	hub.mutex.Lock()
	hub.clients[sender.ID] = senderClient
	hub.clients[recipient.ID] = recipientClient
	hub.mutex.Unlock()

	// Send typing indicator
	msg := `{"type": "typing", "to": "` + recipient.ID + `", "typing": true}`
	senderClient.handleMessage([]byte(msg))

	// Recipient should NOT receive anything (blocked)
	select {
	case <-recipientClient.Send:
		t.Error("Blocked user should not receive typing indicator")
	case <-time.After(100 * time.Millisecond):
		// Good - no message
	}
}

func TestClient_HandleAckMessage_MarkRead(t *testing.T) {
	cleanup := setupClientTestDB(t)
	defer cleanup()

	hub := NewHub()

	// Create users
	sender := &models.User{Username: "sender"}
	recipient := &models.User{Username: "recipient"}
	database.DB.Create(sender)
	database.DB.Create(recipient)

	// Create message
	message := &models.Message{
		SenderID:    sender.ID,
		RecipientID: &recipient.ID,
		Content:     "Hello",
		Status:      models.MessageStatusDelivered,
	}
	database.DB.Create(message)

	// Register sender
	senderClient := createTestClientWithHub(sender.ID, hub)
	hub.mutex.Lock()
	hub.clients[sender.ID] = senderClient
	hub.mutex.Unlock()

	// Recipient marks as read
	recipientClient := createTestClientWithHub(recipient.ID, hub)
	ack := `{"type": "ack", "message_id": "` + message.ID + `", "status": "read"}`
	recipientClient.handleMessage([]byte(ack))

	// Verify message status updated
	var updatedMsg models.Message
	database.DB.First(&updatedMsg, "id = ?", message.ID)
	if updatedMsg.Status != models.MessageStatusRead {
		t.Errorf("Expected status read, got %s", updatedMsg.Status)
	}

	// Sender should receive read ack
	select {
	case ackData := <-senderClient.Send:
		var readAck AckMessage
		json.Unmarshal(ackData, &readAck)
		if readAck.Status != "read" {
			t.Errorf("Expected read status, got %s", readAck.Status)
		}
	case <-time.After(time.Second):
		t.Error("Expected read ack")
	}
}

func TestClient_HandleMessageEdit_Success(t *testing.T) {
	cleanup := setupClientTestDB(t)
	defer cleanup()

	hub := NewHub()

	// Create users
	sender := &models.User{Username: "sender"}
	recipient := &models.User{Username: "recipient"}
	database.DB.Create(sender)
	database.DB.Create(recipient)

	// Create message
	message := &models.Message{
		SenderID:    sender.ID,
		RecipientID: &recipient.ID,
		Content:     "Original content",
		Status:      models.MessageStatusDelivered,
	}
	database.DB.Create(message)

	// Register both
	senderClient := createTestClientWithHub(sender.ID, hub)
	recipientClient := createTestClientWithHub(recipient.ID, hub)

	hub.mutex.Lock()
	hub.clients[sender.ID] = senderClient
	hub.clients[recipient.ID] = recipientClient
	hub.mutex.Unlock()

	// Edit message
	edit := `{"type": "message_edit", "message_id": "` + message.ID + `", "content": "Edited content"}`
	senderClient.handleMessage([]byte(edit))

	// Sender should receive edit event
	select {
	case eventData := <-senderClient.Send:
		var event MessageEditedEvent
		json.Unmarshal(eventData, &event)
		if event.Type != "message_edited" {
			t.Errorf("Expected message_edited type, got %s", event.Type)
		}
		if event.Content != "Edited content" {
			t.Errorf("Expected 'Edited content', got %s", event.Content)
		}
	case <-time.After(time.Second):
		t.Error("Expected edit event")
	}

	// Recipient should receive edit event
	select {
	case eventData := <-recipientClient.Send:
		var event MessageEditedEvent
		json.Unmarshal(eventData, &event)
		if event.Content != "Edited content" {
			t.Errorf("Recipient expected 'Edited content', got %s", event.Content)
		}
	case <-time.After(time.Second):
		t.Error("Recipient expected edit event")
	}

	// Verify database updated
	var updatedMsg models.Message
	database.DB.First(&updatedMsg, "id = ?", message.ID)
	if updatedMsg.Content != "Edited content" {
		t.Errorf("Database expected 'Edited content', got %s", updatedMsg.Content)
	}
	if updatedMsg.EditedAt == nil {
		t.Error("EditedAt should be set")
	}
}

func TestClient_HandleMessageEdit_NotOwner(t *testing.T) {
	cleanup := setupClientTestDB(t)
	defer cleanup()

	hub := NewHub()

	// Create users
	sender := &models.User{Username: "sender"}
	other := &models.User{Username: "other"}
	database.DB.Create(sender)
	database.DB.Create(other)

	// Create message by sender
	message := &models.Message{
		SenderID:    sender.ID,
		RecipientID: &other.ID,
		Content:     "Original content",
		Status:      models.MessageStatusDelivered,
	}
	database.DB.Create(message)

	// Other user tries to edit
	otherClient := createTestClientWithHub(other.ID, hub)
	edit := `{"type": "message_edit", "message_id": "` + message.ID + `", "content": "Hacked content"}`
	otherClient.handleMessage([]byte(edit))

	// Should receive error
	select {
	case errData := <-otherClient.Send:
		var errMsg ErrorMessage
		json.Unmarshal(errData, &errMsg)
		if errMsg.Error != "You can only edit your own messages" {
			t.Errorf("Expected ownership error, got %s", errMsg.Error)
		}
	case <-time.After(time.Second):
		t.Error("Expected error")
	}
}

func TestClient_HandleMessageDelete_ForMe(t *testing.T) {
	cleanup := setupClientTestDB(t)
	defer cleanup()

	hub := NewHub()

	// Create users
	sender := &models.User{Username: "sender"}
	recipient := &models.User{Username: "recipient"}
	database.DB.Create(sender)
	database.DB.Create(recipient)

	// Create message
	message := &models.Message{
		SenderID:    sender.ID,
		RecipientID: &recipient.ID,
		Content:     "Delete me",
		Status:      models.MessageStatusDelivered,
	}
	database.DB.Create(message)

	// Recipient deletes for self
	recipientClient := createTestClientWithHub(recipient.ID, hub)
	del := `{"type": "message_delete", "message_id": "` + message.ID + `", "delete_for": "me"}`
	recipientClient.handleMessage([]byte(del))

	// Should receive delete event
	select {
	case eventData := <-recipientClient.Send:
		var event MessageDeletedEvent
		json.Unmarshal(eventData, &event)
		if event.Type != "message_deleted" {
			t.Errorf("Expected message_deleted type, got %s", event.Type)
		}
	case <-time.After(time.Second):
		t.Error("Expected delete event")
	}

	// Verify deletion record created
	var deletion models.MessageDeletion
	err := database.DB.Where("message_id = ? AND user_id = ?", message.ID, recipient.ID).First(&deletion).Error
	if err != nil {
		t.Error("Deletion record should be created")
	}

	// Original message should NOT be deleted
	var originalMsg models.Message
	database.DB.First(&originalMsg, "id = ?", message.ID)
	if originalMsg.DeletedAt != nil {
		t.Error("Original message should not be soft deleted for 'delete for me'")
	}
}

func TestClient_HandleMessageDelete_ForEveryone(t *testing.T) {
	cleanup := setupClientTestDB(t)
	defer cleanup()

	hub := NewHub()

	// Create users
	sender := &models.User{Username: "sender"}
	recipient := &models.User{Username: "recipient"}
	database.DB.Create(sender)
	database.DB.Create(recipient)

	// Create message
	message := &models.Message{
		SenderID:    sender.ID,
		RecipientID: &recipient.ID,
		Content:     "Delete for everyone",
		Status:      models.MessageStatusDelivered,
	}
	database.DB.Create(message)

	// Register recipient
	recipientClient := createTestClientWithHub(recipient.ID, hub)
	hub.mutex.Lock()
	hub.clients[recipient.ID] = recipientClient
	hub.mutex.Unlock()

	// Sender deletes for everyone
	senderClient := createTestClientWithHub(sender.ID, hub)
	del := `{"type": "message_delete", "message_id": "` + message.ID + `", "delete_for": "everyone"}`
	senderClient.handleMessage([]byte(del))

	// Sender should receive delete event
	select {
	case eventData := <-senderClient.Send:
		var event MessageDeletedEvent
		json.Unmarshal(eventData, &event)
		if event.Type != "message_deleted" {
			t.Errorf("Expected message_deleted type, got %s", event.Type)
		}
	case <-time.After(time.Second):
		t.Error("Expected delete event for sender")
	}

	// Recipient should also receive delete event
	select {
	case eventData := <-recipientClient.Send:
		var event MessageDeletedEvent
		json.Unmarshal(eventData, &event)
		if event.Type != "message_deleted" {
			t.Errorf("Recipient expected message_deleted type, got %s", event.Type)
		}
	case <-time.After(time.Second):
		t.Error("Recipient expected delete event")
	}

	// Message should be soft deleted
	var deletedMsg models.Message
	database.DB.First(&deletedMsg, "id = ?", message.ID)
	if deletedMsg.DeletedAt == nil {
		t.Error("Message should be soft deleted")
	}
}

func TestClient_HandleReaction_Add(t *testing.T) {
	cleanup := setupClientTestDB(t)
	defer cleanup()

	hub := NewHub()

	// Create users
	sender := &models.User{Username: "sender"}
	recipient := &models.User{Username: "recipient"}
	database.DB.Create(sender)
	database.DB.Create(recipient)

	// Create message
	message := &models.Message{
		SenderID:    sender.ID,
		RecipientID: &recipient.ID,
		Content:     "React to me",
		Status:      models.MessageStatusDelivered,
	}
	database.DB.Create(message)

	// Register sender
	senderClient := createTestClientWithHub(sender.ID, hub)
	hub.mutex.Lock()
	hub.clients[sender.ID] = senderClient
	hub.mutex.Unlock()

	// Recipient adds reaction
	recipientClient := createTestClientWithHub(recipient.ID, hub)
	reaction := `{"type": "reaction", "message_id": "` + message.ID + `", "emoji": "👍", "action": "add"}`
	recipientClient.handleMessage([]byte(reaction))

	// Recipient should receive reaction event
	select {
	case eventData := <-recipientClient.Send:
		var event ReactionEvent
		json.Unmarshal(eventData, &event)
		if event.Type != "reaction" {
			t.Errorf("Expected reaction type, got %s", event.Type)
		}
		if event.Action != "added" {
			t.Errorf("Expected action 'added', got %s", event.Action)
		}
		if event.Emoji != "👍" {
			t.Errorf("Expected emoji 👍, got %s", event.Emoji)
		}
	case <-time.After(time.Second):
		t.Error("Expected reaction event")
	}

	// Sender should also receive reaction event
	select {
	case eventData := <-senderClient.Send:
		var event ReactionEvent
		json.Unmarshal(eventData, &event)
		if event.Type != "reaction" {
			t.Errorf("Sender expected reaction type, got %s", event.Type)
		}
	case <-time.After(time.Second):
		t.Error("Sender expected reaction event")
	}

	// Verify reaction in database
	var reactions []models.Reaction
	database.DB.Where("message_id = ?", message.ID).Find(&reactions)
	if len(reactions) != 1 {
		t.Errorf("Expected 1 reaction, got %d", len(reactions))
	}
}

func TestClient_HandleReaction_Remove(t *testing.T) {
	cleanup := setupClientTestDB(t)
	defer cleanup()

	hub := NewHub()

	// Create users
	sender := &models.User{Username: "sender"}
	recipient := &models.User{Username: "recipient"}
	database.DB.Create(sender)
	database.DB.Create(recipient)

	// Create message
	message := &models.Message{
		SenderID:    sender.ID,
		RecipientID: &recipient.ID,
		Content:     "React to me",
		Status:      models.MessageStatusDelivered,
	}
	database.DB.Create(message)

	// Add reaction first
	database.DB.Create(&models.Reaction{
		MessageID: message.ID,
		UserID:    recipient.ID,
		Emoji:     "👍",
	})

	// Register sender
	senderClient := createTestClientWithHub(sender.ID, hub)
	hub.mutex.Lock()
	hub.clients[sender.ID] = senderClient
	hub.mutex.Unlock()

	// Recipient removes reaction
	recipientClient := createTestClientWithHub(recipient.ID, hub)
	reaction := `{"type": "reaction", "message_id": "` + message.ID + `", "action": "remove"}`
	recipientClient.handleMessage([]byte(reaction))

	// Should receive reaction event
	select {
	case eventData := <-recipientClient.Send:
		var event ReactionEvent
		json.Unmarshal(eventData, &event)
		if event.Action != "removed" {
			t.Errorf("Expected action 'removed', got %s", event.Action)
		}
	case <-time.After(time.Second):
		t.Error("Expected reaction event")
	}

	// Verify reaction removed from database
	var count int64
	database.DB.Model(&models.Reaction{}).Where("message_id = ?", message.ID).Count(&count)
	if count != 0 {
		t.Errorf("Expected 0 reactions, got %d", count)
	}
}

func TestClient_HandleReaction_InvalidAction(t *testing.T) {
	cleanup := setupClientTestDB(t)
	defer cleanup()

	hub := NewHub()
	client := createTestClientWithHub("user1", hub)

	// Send invalid action
	reaction := `{"type": "reaction", "message_id": "some-id", "action": "invalid"}`
	client.handleMessage([]byte(reaction))

	// Should receive error
	select {
	case errData := <-client.Send:
		var errMsg ErrorMessage
		json.Unmarshal(errData, &errMsg)
		if errMsg.Error != "Action must be 'add' or 'remove'" {
			t.Errorf("Expected action error, got %s", errMsg.Error)
		}
	case <-time.After(time.Second):
		t.Error("Expected error")
	}
}

func TestClient_HandleChatMessage_WithReply(t *testing.T) {
	cleanup := setupClientTestDB(t)
	defer cleanup()

	hub := NewHub()

	// Create users
	user1 := &models.User{Username: "user1"}
	user2 := &models.User{Username: "user2"}
	database.DB.Create(user1)
	database.DB.Create(user2)

	// Create original message
	originalMsg := &models.Message{
		SenderID:    user1.ID,
		RecipientID: &user2.ID,
		Content:     "Original message",
		Status:      models.MessageStatusDelivered,
	}
	database.DB.Create(originalMsg)

	// Register user1
	client1 := createTestClientWithHub(user1.ID, hub)
	hub.mutex.Lock()
	hub.clients[user1.ID] = client1
	hub.mutex.Unlock()

	// User2 sends reply
	client2 := createTestClientWithHub(user2.ID, hub)
	msg := `{"type": "message", "to": "` + user1.ID + `", "content": "This is a reply", "reply_to_id": "` + originalMsg.ID + `"}`
	client2.handleMessage([]byte(msg))

	// User2 should receive ack
	select {
	case <-client2.Send:
		// Got ack, continue
	case <-time.After(time.Second):
		t.Error("Expected ack")
	}

	// User1 should receive message with reply preview
	select {
	case msgData := <-client1.Send:
		var chatMsg ChatMessage
		json.Unmarshal(msgData, &chatMsg)
		if chatMsg.ReplyToID == nil || *chatMsg.ReplyToID != originalMsg.ID {
			t.Error("Expected reply_to_id to be set")
		}
		if chatMsg.ReplyTo == nil {
			t.Error("Expected reply preview")
		} else {
			if chatMsg.ReplyTo.Content != "Original message" {
				t.Errorf("Expected reply preview content 'Original message', got %s", chatMsg.ReplyTo.Content)
			}
		}
	case <-time.After(time.Second):
		t.Error("Expected message with reply")
	}
}

func TestClient_SendError(t *testing.T) {
	hub := NewHub()
	client := createTestClientWithHub("user1", hub)

	client.sendError("Test error message")

	select {
	case errData := <-client.Send:
		var errMsg ErrorMessage
		json.Unmarshal(errData, &errMsg)
		if errMsg.Type != "error" {
			t.Errorf("Expected error type, got %s", errMsg.Type)
		}
		if errMsg.Error != "Test error message" {
			t.Errorf("Expected 'Test error message', got %s", errMsg.Error)
		}
	case <-time.After(time.Second):
		t.Error("Expected error message")
	}
}

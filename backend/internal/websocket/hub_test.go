package websocket

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"messenger/internal/database"
	"messenger/internal/models"
)

func setupTestDB(t *testing.T) func() {
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
	)

	return func() {
		sqlDB, _ := database.DB.DB()
		sqlDB.Close()
	}
}

func createTestClient(userID string) *Client {
	client := &Client{
		UserID: userID,
		Conn:   nil, // No actual connection for unit tests
		Send:   make(chan []byte, 256),
		Hub:    nil,
	}
	return client
}

func TestNewHub(t *testing.T) {
	hub := NewHub()

	if hub == nil {
		t.Fatal("NewHub returned nil")
	}

	if hub.clients == nil {
		t.Error("Hub clients map not initialized")
	}

	if hub.register == nil {
		t.Error("Hub register channel not initialized")
	}

	if hub.unregister == nil {
		t.Error("Hub unregister channel not initialized")
	}
}

func TestHub_IsOnline(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	hub := NewHub()

	// Create test user
	user := &models.User{Username: "testuser"}
	database.DB.Create(user)

	// User should not be online initially
	if hub.IsOnline(user.ID) {
		t.Error("User should not be online before registration")
	}

	// Register client
	client := createTestClient(user.ID)
	hub.mutex.Lock()
	hub.clients[user.ID] = client
	hub.mutex.Unlock()

	// User should be online now
	if !hub.IsOnline(user.ID) {
		t.Error("User should be online after registration")
	}

	// Unregister
	hub.mutex.Lock()
	delete(hub.clients, user.ID)
	hub.mutex.Unlock()

	// User should be offline again
	if hub.IsOnline(user.ID) {
		t.Error("User should be offline after unregistration")
	}
}

func TestHub_GetClient(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	hub := NewHub()

	user := &models.User{Username: "getclientuser"}
	database.DB.Create(user)

	// Should return nil for non-existent client
	if hub.GetClient(user.ID) != nil {
		t.Error("GetClient should return nil for non-existent user")
	}

	// Register client
	client := createTestClient(user.ID)
	hub.mutex.Lock()
	hub.clients[user.ID] = client
	hub.mutex.Unlock()

	// Should return the client
	got := hub.GetClient(user.ID)
	if got != client {
		t.Error("GetClient should return the registered client")
	}
}

func TestHub_SendToUser(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	hub := NewHub()

	user := &models.User{Username: "sendtouser"}
	database.DB.Create(user)

	// Sending to non-existent user should return false
	if hub.SendToUser(user.ID, []byte("test")) {
		t.Error("SendToUser should return false for non-existent user")
	}

	// Register client
	client := createTestClient(user.ID)
	hub.mutex.Lock()
	hub.clients[user.ID] = client
	hub.mutex.Unlock()

	// Sending should succeed
	testMsg := []byte(`{"type":"test"}`)
	if !hub.SendToUser(user.ID, testMsg) {
		t.Error("SendToUser should return true for existing user")
	}

	// Verify message was sent to channel
	select {
	case msg := <-client.Send:
		if string(msg) != string(testMsg) {
			t.Errorf("Expected message %s, got %s", testMsg, msg)
		}
	case <-time.After(time.Second):
		t.Error("Message was not sent to client channel")
	}
}

func TestHub_SendJSONToUser(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	hub := NewHub()

	user := &models.User{Username: "sendjsonuser"}
	database.DB.Create(user)

	client := createTestClient(user.ID)
	hub.mutex.Lock()
	hub.clients[user.ID] = client
	hub.mutex.Unlock()

	// Send JSON data
	testData := map[string]string{"type": "test", "message": "hello"}
	if !hub.SendJSONToUser(user.ID, testData) {
		t.Error("SendJSONToUser should return true for existing user")
	}

	// Verify JSON was serialized correctly
	select {
	case msg := <-client.Send:
		var received map[string]string
		if err := json.Unmarshal(msg, &received); err != nil {
			t.Errorf("Failed to unmarshal sent message: %v", err)
		}
		if received["type"] != "test" || received["message"] != "hello" {
			t.Errorf("Unexpected message content: %v", received)
		}
	case <-time.After(time.Second):
		t.Error("Message was not sent to client channel")
	}
}

func TestHub_SendToGroup(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	hub := NewHub()

	// Create test users
	user1 := &models.User{Username: "groupuser1"}
	user2 := &models.User{Username: "groupuser2"}
	user3 := &models.User{Username: "groupuser3"}
	database.DB.Create(user1)
	database.DB.Create(user2)
	database.DB.Create(user3)

	// Create a group with all users
	group := &models.Group{Name: "Test Group", CreatedBy: user1.ID}
	database.DB.Create(group)

	database.DB.Create(&models.GroupMember{GroupID: group.ID, UserID: user1.ID, Role: "admin"})
	database.DB.Create(&models.GroupMember{GroupID: group.ID, UserID: user2.ID, Role: "member"})
	database.DB.Create(&models.GroupMember{GroupID: group.ID, UserID: user3.ID, Role: "member"})

	// Register all clients
	client1 := createTestClient(user1.ID)
	client2 := createTestClient(user2.ID)
	client3 := createTestClient(user3.ID)

	hub.mutex.Lock()
	hub.clients[user1.ID] = client1
	hub.clients[user2.ID] = client2
	hub.clients[user3.ID] = client3
	hub.mutex.Unlock()

	// Send to group excluding user1
	testMsg := []byte(`{"type":"group_message"}`)
	sent := hub.SendToGroup(group.ID, user1.ID, testMsg)

	if sent != 2 {
		t.Errorf("Expected 2 messages sent, got %d", sent)
	}

	// Verify user1 did not receive the message
	select {
	case <-client1.Send:
		t.Error("User1 should not have received the message (was excluded)")
	case <-time.After(100 * time.Millisecond):
		// Good - no message
	}

	// Verify user2 received the message
	select {
	case msg := <-client2.Send:
		if string(msg) != string(testMsg) {
			t.Errorf("User2 received wrong message")
		}
	case <-time.After(time.Second):
		t.Error("User2 should have received the message")
	}

	// Verify user3 received the message
	select {
	case msg := <-client3.Send:
		if string(msg) != string(testMsg) {
			t.Errorf("User3 received wrong message")
		}
	case <-time.After(time.Second):
		t.Error("User3 should have received the message")
	}
}

func TestHub_GetGroupMemberIDs(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	hub := NewHub()

	user1 := &models.User{Username: "member1"}
	user2 := &models.User{Username: "member2"}
	database.DB.Create(user1)
	database.DB.Create(user2)

	group := &models.Group{Name: "Members Test", CreatedBy: user1.ID}
	database.DB.Create(group)

	database.DB.Create(&models.GroupMember{GroupID: group.ID, UserID: user1.ID, Role: "admin"})
	database.DB.Create(&models.GroupMember{GroupID: group.ID, UserID: user2.ID, Role: "member"})

	ids := hub.GetGroupMemberIDs(group.ID)

	if len(ids) != 2 {
		t.Errorf("Expected 2 members, got %d", len(ids))
	}

	// Verify both IDs are present
	found1, found2 := false, false
	for _, id := range ids {
		if id == user1.ID {
			found1 = true
		}
		if id == user2.ID {
			found2 = true
		}
	}
	if !found1 || !found2 {
		t.Error("Not all member IDs were returned")
	}
}

func TestHub_GetOfflineGroupMemberIDs(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	hub := NewHub()

	user1 := &models.User{Username: "online1"}
	user2 := &models.User{Username: "offline1"}
	user3 := &models.User{Username: "offline2"}
	database.DB.Create(user1)
	database.DB.Create(user2)
	database.DB.Create(user3)

	group := &models.Group{Name: "Offline Test", CreatedBy: user1.ID}
	database.DB.Create(group)

	database.DB.Create(&models.GroupMember{GroupID: group.ID, UserID: user1.ID, Role: "admin"})
	database.DB.Create(&models.GroupMember{GroupID: group.ID, UserID: user2.ID, Role: "member"})
	database.DB.Create(&models.GroupMember{GroupID: group.ID, UserID: user3.ID, Role: "member"})

	// Only user1 is online
	client1 := createTestClient(user1.ID)
	hub.mutex.Lock()
	hub.clients[user1.ID] = client1
	hub.mutex.Unlock()

	// Get offline members excluding user1
	offlineIDs := hub.GetOfflineGroupMemberIDs(group.ID, user1.ID)

	if len(offlineIDs) != 2 {
		t.Errorf("Expected 2 offline members, got %d", len(offlineIDs))
	}

	// Verify user1 is not in the list (online + excluded)
	for _, id := range offlineIDs {
		if id == user1.ID {
			t.Error("User1 should not be in offline list")
		}
	}
}

func TestHub_ConcurrentAccess(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	hub := NewHub()

	// Create multiple users
	var users []*models.User
	for i := 0; i < 10; i++ {
		user := &models.User{Username: "concurrent" + string(rune('0'+i))}
		database.DB.Create(user)
		users = append(users, user)
	}

	// Concurrently register and unregister
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			client := createTestClient(users[idx].ID)

			hub.mutex.Lock()
			hub.clients[users[idx].ID] = client
			hub.mutex.Unlock()

			time.Sleep(10 * time.Millisecond)

			hub.IsOnline(users[idx].ID)
			hub.GetClient(users[idx].ID)

			hub.mutex.Lock()
			delete(hub.clients, users[idx].ID)
			hub.mutex.Unlock()
		}(i)
	}

	wg.Wait()
	// Should complete without deadlock or panic
}

func TestMessageTypes(t *testing.T) {
	// Test that message types can be properly marshaled/unmarshaled

	tests := []struct {
		name string
		msg  interface{}
	}{
		{
			name: "ChatMessage",
			msg: ChatMessage{
				Type:    "message",
				ID:      "msg123",
				To:      "user456",
				Content: "Hello",
			},
		},
		{
			name: "TypingMessage",
			msg: TypingMessage{
				Type:   "typing",
				To:     "user456",
				Typing: true,
			},
		},
		{
			name: "PresenceMessage",
			msg: PresenceMessage{
				Type:   "presence",
				UserID: "user123",
				Online: true,
			},
		},
		{
			name: "AckMessage",
			msg: AckMessage{
				Type:      "ack",
				MessageID: "msg123",
				Status:    "delivered",
			},
		},
		{
			name: "ReactionMessage",
			msg: ReactionMessage{
				Type:      "reaction",
				MessageID: "msg123",
				Emoji:     "👍",
				Action:    "add",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.msg)
			if err != nil {
				t.Errorf("Failed to marshal %s: %v", tt.name, err)
			}

			var base BaseMessage
			if err := json.Unmarshal(data, &base); err != nil {
				t.Errorf("Failed to unmarshal %s as BaseMessage: %v", tt.name, err)
			}

			if base.Type == "" {
				t.Errorf("%s has empty type after marshal/unmarshal", tt.name)
			}
		})
	}
}

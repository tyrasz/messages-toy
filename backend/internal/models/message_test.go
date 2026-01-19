package models

import (
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupMessageTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	db.AutoMigrate(&User{}, &Message{}, &MessageDeletion{}, &Group{})

	return db
}

func TestMessage_IsGroupMessage(t *testing.T) {
	tests := []struct {
		name    string
		groupID *string
		want    bool
	}{
		{"nil group", nil, false},
		{"empty group", strPtr(""), false},
		{"valid group", strPtr("group-123"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Message{GroupID: tt.groupID}
			if got := m.IsGroupMessage(); got != tt.want {
				t.Errorf("IsGroupMessage() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMessage_IsDeleted(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name      string
		deletedAt *time.Time
		want      bool
	}{
		{"not deleted", nil, false},
		{"deleted", &now, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Message{DeletedAt: tt.deletedAt}
			if got := m.IsDeleted(); got != tt.want {
				t.Errorf("IsDeleted() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMessage_IsEdited(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		editedAt *time.Time
		want     bool
	}{
		{"not edited", nil, false},
		{"edited", &now, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Message{EditedAt: tt.editedAt}
			if got := m.IsEdited(); got != tt.want {
				t.Errorf("IsEdited() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMessage_IsExpired(t *testing.T) {
	past := time.Now().Add(-time.Hour)
	future := time.Now().Add(time.Hour)

	tests := []struct {
		name      string
		expiresAt *time.Time
		want      bool
	}{
		{"no expiration", nil, false},
		{"expired", &past, true},
		{"not expired", &future, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Message{ExpiresAt: tt.expiresAt}
			if got := m.IsExpired(); got != tt.want {
				t.Errorf("IsExpired() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMessage_IsDisappearing(t *testing.T) {
	future := time.Now().Add(time.Hour)

	tests := []struct {
		name      string
		expiresAt *time.Time
		want      bool
	}{
		{"not disappearing", nil, false},
		{"disappearing", &future, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Message{ExpiresAt: tt.expiresAt}
			if got := m.IsDisappearing(); got != tt.want {
				t.Errorf("IsDisappearing() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMessage_IsLocation(t *testing.T) {
	lat := 37.7749
	lng := -122.4194

	tests := []struct {
		name string
		lat  *float64
		lng  *float64
		want bool
	}{
		{"no location", nil, nil, false},
		{"only lat", &lat, nil, false},
		{"only lng", nil, &lng, false},
		{"full location", &lat, &lng, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Message{Latitude: tt.lat, Longitude: tt.lng}
			if got := m.IsLocation(); got != tt.want {
				t.Errorf("IsLocation() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMessage_IsScheduled(t *testing.T) {
	past := time.Now().Add(-time.Hour)
	future := time.Now().Add(time.Hour)

	tests := []struct {
		name        string
		scheduledAt *time.Time
		want        bool
	}{
		{"not scheduled", nil, false},
		{"scheduled past", &past, false},
		{"scheduled future", &future, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Message{ScheduledAt: tt.scheduledAt}
			if got := m.IsScheduled(); got != tt.want {
				t.Errorf("IsScheduled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMessage_BeforeCreate(t *testing.T) {
	db := setupMessageTestDB(t)

	// Create sender user
	sender := &User{Username: "sender"}
	db.Create(sender)

	t.Run("generates ID if empty", func(t *testing.T) {
		m := &Message{
			SenderID: sender.ID,
			Content:  "Test message",
		}
		err := db.Create(m).Error
		if err != nil {
			t.Fatalf("Failed to create message: %v", err)
		}
		if m.ID == "" {
			t.Error("ID should be generated")
		}
	})

	t.Run("preserves existing ID", func(t *testing.T) {
		customID := "custom-id-123"
		m := &Message{
			ID:       customID,
			SenderID: sender.ID,
			Content:  "Test message",
		}
		err := db.Create(m).Error
		if err != nil {
			t.Fatalf("Failed to create message: %v", err)
		}
		if m.ID != customID {
			t.Errorf("ID should be preserved, got %s", m.ID)
		}
	})

	t.Run("sets default status", func(t *testing.T) {
		m := &Message{
			SenderID: sender.ID,
			Content:  "Test message",
		}
		err := db.Create(m).Error
		if err != nil {
			t.Fatalf("Failed to create message: %v", err)
		}
		if m.Status != MessageStatusSent {
			t.Errorf("Status should be 'sent', got %s", m.Status)
		}
	})
}

func TestMessageDeletion_BeforeCreate(t *testing.T) {
	db := setupMessageTestDB(t)

	// Create user and message
	user := &User{Username: "testuser"}
	db.Create(user)

	msg := &Message{
		SenderID: user.ID,
		Content:  "Test",
	}
	db.Create(msg)

	t.Run("generates ID if empty", func(t *testing.T) {
		deletion := &MessageDeletion{
			MessageID: msg.ID,
			UserID:    user.ID,
			DeletedAt: time.Now(),
		}
		err := db.Create(deletion).Error
		if err != nil {
			t.Fatalf("Failed to create deletion: %v", err)
		}
		if deletion.ID == "" {
			t.Error("ID should be generated")
		}
	})
}

// Helper function
func strPtr(s string) *string {
	return &s
}

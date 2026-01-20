package models

import (
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupUserTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	db.AutoMigrate(&User{})

	return db
}

func TestUser_IsAdmin(t *testing.T) {
	tests := []struct {
		name string
		role UserRole
		want bool
	}{
		{"admin user", UserRoleAdmin, true},
		{"moderator user", UserRoleModerator, false},
		{"regular user", UserRoleUser, false},
		{"empty role", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &User{Role: tt.role}
			if got := u.IsAdmin(); got != tt.want {
				t.Errorf("IsAdmin() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUser_IsModerator(t *testing.T) {
	tests := []struct {
		name string
		role UserRole
		want bool
	}{
		{"admin user", UserRoleAdmin, true},
		{"moderator user", UserRoleModerator, true},
		{"regular user", UserRoleUser, false},
		{"empty role", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &User{Role: tt.role}
			if got := u.IsModerator(); got != tt.want {
				t.Errorf("IsModerator() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUser_BeforeCreate(t *testing.T) {
	db := setupUserTestDB(t)

	t.Run("generates ID if empty", func(t *testing.T) {
		u := &User{
			Username:     "testuser",
			PasswordHash: "hash",
		}
		err := db.Create(u).Error
		if err != nil {
			t.Fatalf("Failed to create user: %v", err)
		}
		if u.ID == "" {
			t.Error("ID should be generated")
		}
	})

	t.Run("preserves existing ID", func(t *testing.T) {
		customID := "custom-user-id"
		u := &User{
			ID:           customID,
			Username:     "testuser2",
			PasswordHash: "hash",
		}
		err := db.Create(u).Error
		if err != nil {
			t.Fatalf("Failed to create user: %v", err)
		}
		if u.ID != customID {
			t.Errorf("ID should be preserved, got %s", u.ID)
		}
	})
}

func TestUser_ToResponse(t *testing.T) {
	now := time.Now()
	phone := "+1234567890"

	u := &User{
		ID:          "user-123",
		Username:    "johndoe",
		Phone:       &phone,
		DisplayName: "John Doe",
		AvatarURL:   "https://example.com/avatar.png",
		About:       "Hello world",
		StatusEmoji: "🎉",
		LastSeen:    now,
	}

	t.Run("online user", func(t *testing.T) {
		resp := u.ToResponse(true)

		if resp.ID != u.ID {
			t.Errorf("Expected ID %s, got %s", u.ID, resp.ID)
		}
		if resp.Username != u.Username {
			t.Errorf("Expected Username %s, got %s", u.Username, resp.Username)
		}
		if resp.DisplayName != u.DisplayName {
			t.Errorf("Expected DisplayName %s, got %s", u.DisplayName, resp.DisplayName)
		}
		if resp.AvatarURL != u.AvatarURL {
			t.Errorf("Expected AvatarURL %s, got %s", u.AvatarURL, resp.AvatarURL)
		}
		if resp.About != u.About {
			t.Errorf("Expected About %s, got %s", u.About, resp.About)
		}
		if resp.StatusEmoji != u.StatusEmoji {
			t.Errorf("Expected StatusEmoji %s, got %s", u.StatusEmoji, resp.StatusEmoji)
		}
		if !resp.Online {
			t.Error("Expected Online to be true")
		}
	})

	t.Run("offline user", func(t *testing.T) {
		resp := u.ToResponse(false)

		if resp.Online {
			t.Error("Expected Online to be false")
		}
	})
}

func TestUser_UniqueConstraints(t *testing.T) {
	db := setupUserTestDB(t)

	// Create first user
	u1 := &User{
		Username:     "uniqueuser",
		PasswordHash: "hash",
	}
	if err := db.Create(u1).Error; err != nil {
		t.Fatalf("Failed to create first user: %v", err)
	}

	t.Run("duplicate username fails", func(t *testing.T) {
		u2 := &User{
			Username:     "uniqueuser", // Same username
			PasswordHash: "hash",
		}
		err := db.Create(u2).Error
		if err == nil {
			t.Error("Should fail with duplicate username")
		}
	})

	t.Run("duplicate phone fails", func(t *testing.T) {
		phone := "+9876543210"

		u3 := &User{
			Username:     "user3",
			Phone:        &phone,
			PasswordHash: "hash",
		}
		if err := db.Create(u3).Error; err != nil {
			t.Fatalf("Failed to create user with phone: %v", err)
		}

		u4 := &User{
			Username:     "user4",
			Phone:        &phone, // Same phone
			PasswordHash: "hash",
		}
		err := db.Create(u4).Error
		if err == nil {
			t.Error("Should fail with duplicate phone")
		}
	})
}

func TestUserRoleConstants(t *testing.T) {
	if UserRoleUser != "user" {
		t.Errorf("Expected UserRoleUser to be 'user', got '%s'", UserRoleUser)
	}
	if UserRoleModerator != "moderator" {
		t.Errorf("Expected UserRoleModerator to be 'moderator', got '%s'", UserRoleModerator)
	}
	if UserRoleAdmin != "admin" {
		t.Errorf("Expected UserRoleAdmin to be 'admin', got '%s'", UserRoleAdmin)
	}
}

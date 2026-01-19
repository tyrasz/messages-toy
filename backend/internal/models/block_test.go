package models

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupBlockTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	db.AutoMigrate(&User{}, &Block{})

	return db
}

func TestBlock_BeforeCreate(t *testing.T) {
	db := setupBlockTestDB(t)

	// Create users
	user1 := &User{Username: "user1"}
	user2 := &User{Username: "user2"}
	db.Create(user1)
	db.Create(user2)

	t.Run("generates ID if empty", func(t *testing.T) {
		block := &Block{
			BlockerID: user1.ID,
			BlockedID: user2.ID,
		}
		err := db.Create(block).Error
		if err != nil {
			t.Fatalf("Failed to create block: %v", err)
		}
		if block.ID == "" {
			t.Error("ID should be generated")
		}
	})

	t.Run("preserves existing ID", func(t *testing.T) {
		// Create new users to avoid unique constraint
		user3 := &User{Username: "user3"}
		user4 := &User{Username: "user4"}
		db.Create(user3)
		db.Create(user4)

		customID := "custom-block-id"
		block := &Block{
			ID:        customID,
			BlockerID: user3.ID,
			BlockedID: user4.ID,
		}
		err := db.Create(block).Error
		if err != nil {
			t.Fatalf("Failed to create block: %v", err)
		}
		if block.ID != customID {
			t.Errorf("ID should be preserved, got %s", block.ID)
		}
	})
}

func TestIsBlocked(t *testing.T) {
	db := setupBlockTestDB(t)

	// Create users
	user1 := &User{Username: "blocker"}
	user2 := &User{Username: "blocked"}
	user3 := &User{Username: "unrelated"}
	db.Create(user1)
	db.Create(user2)
	db.Create(user3)

	// User1 blocks User2
	block := &Block{
		BlockerID: user1.ID,
		BlockedID: user2.ID,
	}
	db.Create(block)

	tests := []struct {
		name      string
		blockerID string
		blockedID string
		want      bool
	}{
		{"user1 blocked user2", user1.ID, user2.ID, true},
		{"user2 did not block user1", user2.ID, user1.ID, false},
		{"user1 did not block user3", user1.ID, user3.ID, false},
		{"user3 did not block anyone", user3.ID, user1.ID, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsBlocked(db, tt.blockerID, tt.blockedID); got != tt.want {
				t.Errorf("IsBlocked() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsEitherBlocked(t *testing.T) {
	db := setupBlockTestDB(t)

	// Create users
	alice := &User{Username: "alice"}
	bob := &User{Username: "bob"}
	charlie := &User{Username: "charlie"}
	db.Create(alice)
	db.Create(bob)
	db.Create(charlie)

	// Alice blocks Bob
	block := &Block{
		BlockerID: alice.ID,
		BlockedID: bob.ID,
	}
	db.Create(block)

	tests := []struct {
		name    string
		userID1 string
		userID2 string
		want    bool
	}{
		{"alice blocked bob - check alice,bob", alice.ID, bob.ID, true},
		{"alice blocked bob - check bob,alice", bob.ID, alice.ID, true},
		{"alice and charlie - no block", alice.ID, charlie.ID, false},
		{"bob and charlie - no block", bob.ID, charlie.ID, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsEitherBlocked(db, tt.userID1, tt.userID2); got != tt.want {
				t.Errorf("IsEitherBlocked() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBlock_UniqueConstraint(t *testing.T) {
	db := setupBlockTestDB(t)

	// Create users
	user1 := &User{Username: "user1"}
	user2 := &User{Username: "user2"}
	db.Create(user1)
	db.Create(user2)

	// First block should succeed
	block1 := &Block{
		BlockerID: user1.ID,
		BlockedID: user2.ID,
	}
	err := db.Create(block1).Error
	if err != nil {
		t.Fatalf("First block should succeed: %v", err)
	}

	// Duplicate block should fail
	block2 := &Block{
		BlockerID: user1.ID,
		BlockedID: user2.ID,
	}
	err = db.Create(block2).Error
	if err == nil {
		t.Error("Duplicate block should fail due to unique constraint")
	}
}

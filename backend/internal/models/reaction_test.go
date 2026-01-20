package models

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupReactionTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	db.AutoMigrate(&User{}, &Message{}, &Reaction{})

	return db
}

func createTestUserAndMessage(t *testing.T, db *gorm.DB, username string) (*User, *Message) {
	user := &User{Username: username, PasswordHash: "hash"}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	msg := &Message{SenderID: user.ID, Content: "Test message"}
	if err := db.Create(msg).Error; err != nil {
		t.Fatalf("Failed to create message: %v", err)
	}

	return user, msg
}

func TestReaction_BeforeCreate(t *testing.T) {
	db := setupReactionTestDB(t)
	user, msg := createTestUserAndMessage(t, db, "reactionuser")

	t.Run("generates ID if empty", func(t *testing.T) {
		r := &Reaction{
			MessageID: msg.ID,
			UserID:    user.ID,
			Emoji:     "👍",
		}
		if err := db.Create(r).Error; err != nil {
			t.Fatalf("Failed to create reaction: %v", err)
		}
		if r.ID == "" {
			t.Error("ID should be generated")
		}
	})
}

func TestAddReaction(t *testing.T) {
	db := setupReactionTestDB(t)
	user, msg := createTestUserAndMessage(t, db, "addreactionuser")

	t.Run("creates new reaction", func(t *testing.T) {
		reaction, err := AddReaction(db, msg.ID, user.ID, "👍")
		if err != nil {
			t.Fatalf("AddReaction failed: %v", err)
		}

		if reaction.Emoji != "👍" {
			t.Errorf("Expected emoji 👍, got %s", reaction.Emoji)
		}
		if reaction.MessageID != msg.ID {
			t.Errorf("Expected message ID %s, got %s", msg.ID, reaction.MessageID)
		}
		if reaction.UserID != user.ID {
			t.Errorf("Expected user ID %s, got %s", user.ID, reaction.UserID)
		}
	})

	t.Run("updates existing reaction", func(t *testing.T) {
		// First reaction already exists from previous test
		reaction, err := AddReaction(db, msg.ID, user.ID, "❤️")
		if err != nil {
			t.Fatalf("AddReaction update failed: %v", err)
		}

		if reaction.Emoji != "❤️" {
			t.Errorf("Expected emoji ❤️, got %s", reaction.Emoji)
		}

		// Should still be only one reaction from this user
		var count int64
		db.Model(&Reaction{}).Where("message_id = ? AND user_id = ?", msg.ID, user.ID).Count(&count)
		if count != 1 {
			t.Errorf("Expected 1 reaction, got %d", count)
		}
	})
}

func TestRemoveReaction(t *testing.T) {
	db := setupReactionTestDB(t)
	user, msg := createTestUserAndMessage(t, db, "removeuser")

	// Add a reaction first
	_, err := AddReaction(db, msg.ID, user.ID, "👍")
	if err != nil {
		t.Fatalf("Failed to add reaction: %v", err)
	}

	t.Run("removes existing reaction", func(t *testing.T) {
		err := RemoveReaction(db, msg.ID, user.ID)
		if err != nil {
			t.Fatalf("RemoveReaction failed: %v", err)
		}

		// Verify it's gone
		var count int64
		db.Model(&Reaction{}).Where("message_id = ? AND user_id = ?", msg.ID, user.ID).Count(&count)
		if count != 0 {
			t.Errorf("Expected 0 reactions, got %d", count)
		}
	})

	t.Run("no error for non-existent reaction", func(t *testing.T) {
		err := RemoveReaction(db, msg.ID, user.ID)
		if err != nil {
			t.Errorf("RemoveReaction should not error for non-existent reaction: %v", err)
		}
	})
}

func TestGetMessageReactions(t *testing.T) {
	db := setupReactionTestDB(t)
	user1, msg := createTestUserAndMessage(t, db, "getuser1")

	user2 := &User{Username: "getuser2", PasswordHash: "hash"}
	db.Create(user2)

	// Add reactions
	AddReaction(db, msg.ID, user1.ID, "👍")
	AddReaction(db, msg.ID, user2.ID, "❤️")

	reactions, err := GetMessageReactions(db, msg.ID)
	if err != nil {
		t.Fatalf("GetMessageReactions failed: %v", err)
	}

	if len(reactions) != 2 {
		t.Errorf("Expected 2 reactions, got %d", len(reactions))
	}
}

func TestGetReactionSummary(t *testing.T) {
	db := setupReactionTestDB(t)
	user1, msg := createTestUserAndMessage(t, db, "sumuser1")

	user2 := &User{Username: "sumuser2", PasswordHash: "hash"}
	user3 := &User{Username: "sumuser3", PasswordHash: "hash"}
	db.Create(user2)
	db.Create(user3)

	// Add reactions - 2 thumbs up, 1 heart
	AddReaction(db, msg.ID, user1.ID, "👍")
	AddReaction(db, msg.ID, user2.ID, "👍")
	AddReaction(db, msg.ID, user3.ID, "❤️")

	summary, err := GetReactionSummary(db, msg.ID)
	if err != nil {
		t.Fatalf("GetReactionSummary failed: %v", err)
	}

	if summary["👍"] != 2 {
		t.Errorf("Expected 2 thumbs up, got %d", summary["👍"])
	}
	if summary["❤️"] != 1 {
		t.Errorf("Expected 1 heart, got %d", summary["❤️"])
	}
}

func TestGetMessageReactionInfo(t *testing.T) {
	db := setupReactionTestDB(t)
	user1, msg := createTestUserAndMessage(t, db, "infouser1")

	user2 := &User{Username: "infouser2", PasswordHash: "hash"}
	user3 := &User{Username: "infouser3", PasswordHash: "hash"}
	db.Create(user2)
	db.Create(user3)

	// Add reactions
	AddReaction(db, msg.ID, user1.ID, "👍")
	AddReaction(db, msg.ID, user2.ID, "👍")
	AddReaction(db, msg.ID, user3.ID, "❤️")

	info, err := GetMessageReactionInfo(db, msg.ID)
	if err != nil {
		t.Fatalf("GetMessageReactionInfo failed: %v", err)
	}

	if len(info) != 2 {
		t.Errorf("Expected 2 unique emojis, got %d", len(info))
	}

	// Find thumbs up info
	var thumbsUpInfo *ReactionInfo
	for i := range info {
		if info[i].Emoji == "👍" {
			thumbsUpInfo = &info[i]
			break
		}
	}

	if thumbsUpInfo == nil {
		t.Fatal("Should have thumbs up reaction info")
	}

	if thumbsUpInfo.Count != 2 {
		t.Errorf("Expected count 2 for thumbs up, got %d", thumbsUpInfo.Count)
	}

	if len(thumbsUpInfo.Users) != 2 {
		t.Errorf("Expected 2 users for thumbs up, got %d", len(thumbsUpInfo.Users))
	}
}

func TestReaction_UniqueConstraint(t *testing.T) {
	db := setupReactionTestDB(t)
	user, msg := createTestUserAndMessage(t, db, "uniqueuser")

	// First reaction should succeed
	r1 := &Reaction{
		MessageID: msg.ID,
		UserID:    user.ID,
		Emoji:     "👍",
	}
	if err := db.Create(r1).Error; err != nil {
		t.Fatalf("First reaction should succeed: %v", err)
	}

	// Duplicate should fail (same user, same message)
	r2 := &Reaction{
		MessageID: msg.ID,
		UserID:    user.ID,
		Emoji:     "❤️", // Different emoji, but same user+message
	}
	err := db.Create(r2).Error
	if err == nil {
		t.Error("Duplicate reaction should fail due to unique constraint")
	}
}

func TestGetMessageReactions_EmptyResult(t *testing.T) {
	db := setupReactionTestDB(t)
	_, msg := createTestUserAndMessage(t, db, "emptyuser")

	// Get reactions for message with no reactions
	reactions, err := GetMessageReactions(db, msg.ID)
	if err != nil {
		t.Fatalf("GetMessageReactions failed: %v", err)
	}

	if len(reactions) != 0 {
		t.Errorf("Expected 0 reactions, got %d", len(reactions))
	}
}

func TestGetReactionSummary_EmptyResult(t *testing.T) {
	db := setupReactionTestDB(t)
	_, msg := createTestUserAndMessage(t, db, "emptysumuser")

	summary, err := GetReactionSummary(db, msg.ID)
	if err != nil {
		t.Fatalf("GetReactionSummary failed: %v", err)
	}

	if len(summary) != 0 {
		t.Errorf("Expected empty summary, got %d items", len(summary))
	}
}

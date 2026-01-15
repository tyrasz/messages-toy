package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Story represents a status/story that expires after 24 hours
type Story struct {
	ID          string    `gorm:"primaryKey" json:"id"`
	UserID      string    `gorm:"not null;index" json:"user_id"`
	Content     string    `json:"content,omitempty"`
	MediaID     *string   `json:"media_id,omitempty"`
	MediaURL    string    `json:"media_url,omitempty"`
	MediaType   string    `json:"media_type,omitempty"` // "image", "video"
	BackgroundColor string `json:"background_color,omitempty"`
	TextColor   string    `json:"text_color,omitempty"`
	FontStyle   string    `json:"font_style,omitempty"`
	Privacy     string    `gorm:"default:contacts" json:"privacy"` // "everyone", "contacts", "close_friends"
	ViewCount   int       `json:"view_count"`
	ExpiresAt   time.Time `gorm:"not null;index" json:"expires_at"`
	CreatedAt   time.Time `json:"created_at"`

	User   User      `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Media  *Media    `gorm:"foreignKey:MediaID" json:"media,omitempty"`
	Views  []StoryView `gorm:"foreignKey:StoryID" json:"views,omitempty"`
}

func (s *Story) BeforeCreate(tx *gorm.DB) error {
	if s.ID == "" {
		s.ID = uuid.New().String()
	}
	if s.ExpiresAt.IsZero() {
		s.ExpiresAt = time.Now().Add(24 * time.Hour)
	}
	return nil
}

func (s *Story) IsExpired() bool {
	return s.ExpiresAt.Before(time.Now())
}

// StoryView tracks who has viewed a story
type StoryView struct {
	ID        string    `gorm:"primaryKey" json:"id"`
	StoryID   string    `gorm:"not null;index;uniqueIndex:idx_story_viewer" json:"story_id"`
	ViewerID  string    `gorm:"not null;index;uniqueIndex:idx_story_viewer" json:"viewer_id"`
	ViewedAt  time.Time `json:"viewed_at"`

	Story  Story `gorm:"foreignKey:StoryID" json:"-"`
	Viewer User  `gorm:"foreignKey:ViewerID" json:"viewer,omitempty"`
}

func (v *StoryView) BeforeCreate(tx *gorm.DB) error {
	if v.ID == "" {
		v.ID = uuid.New().String()
	}
	if v.ViewedAt.IsZero() {
		v.ViewedAt = time.Now()
	}
	return nil
}

// CreateStory creates a new story
func CreateStory(db *gorm.DB, userID, content string, mediaID *string, backgroundColor, textColor, fontStyle, privacy string) (*Story, error) {
	story := &Story{
		UserID:          userID,
		Content:         content,
		MediaID:         mediaID,
		BackgroundColor: backgroundColor,
		TextColor:       textColor,
		FontStyle:       fontStyle,
		Privacy:         privacy,
		ExpiresAt:       time.Now().Add(24 * time.Hour),
	}

	if err := db.Create(story).Error; err != nil {
		return nil, err
	}

	// Load media if present
	if mediaID != nil {
		db.Preload("Media").First(story, "id = ?", story.ID)
		if story.Media != nil {
			story.MediaURL = story.Media.URL
			story.MediaType = string(story.Media.MediaType)
		}
	}

	return story, nil
}

// GetActiveStories gets all non-expired stories for users the viewer can see
func GetActiveStories(db *gorm.DB, viewerID string) ([]Story, error) {
	var stories []Story
	now := time.Now()

	// Get stories from contacts or everyone
	err := db.
		Preload("User").
		Preload("Media").
		Where("expires_at > ?", now).
		Where(`
			user_id = ? OR
			(privacy = 'everyone') OR
			(privacy = 'contacts' AND user_id IN (
				SELECT contact_id FROM contacts WHERE user_id = ?
			))
		`, viewerID, viewerID).
		Order("created_at DESC").
		Find(&stories).Error

	if err != nil {
		return nil, err
	}

	// Populate media URL and type
	for i := range stories {
		if stories[i].Media != nil {
			stories[i].MediaURL = stories[i].Media.URL
			stories[i].MediaType = string(stories[i].Media.MediaType)
		}
	}

	return stories, err
}

// GetUserStories gets all active stories for a specific user
func GetUserStories(db *gorm.DB, userID string) ([]Story, error) {
	var stories []Story
	err := db.
		Preload("Media").
		Where("user_id = ? AND expires_at > ?", userID, time.Now()).
		Order("created_at DESC").
		Find(&stories).Error

	for i := range stories {
		if stories[i].Media != nil {
			stories[i].MediaURL = stories[i].Media.URL
			stories[i].MediaType = string(stories[i].Media.MediaType)
		}
	}

	return stories, err
}

// ViewStory records that a user viewed a story
func ViewStory(db *gorm.DB, storyID, viewerID string) error {
	view := &StoryView{
		StoryID:  storyID,
		ViewerID: viewerID,
	}

	// Try to create, ignore if already exists
	result := db.Create(view)
	if result.Error == nil && result.RowsAffected > 0 {
		// Increment view count
		db.Model(&Story{}).Where("id = ?", storyID).UpdateColumn("view_count", gorm.Expr("view_count + 1"))
	}

	return nil
}

// GetStoryViews gets all viewers of a story
func GetStoryViews(db *gorm.DB, storyID string) ([]StoryView, error) {
	var views []StoryView
	err := db.Preload("Viewer").Where("story_id = ?", storyID).Order("viewed_at DESC").Find(&views).Error
	return views, err
}

// DeleteStory deletes a story
func DeleteStory(db *gorm.DB, storyID, userID string) error {
	return db.Transaction(func(tx *gorm.DB) error {
		// Delete views first
		if err := tx.Where("story_id = ?", storyID).Delete(&StoryView{}).Error; err != nil {
			return err
		}
		// Delete story
		return tx.Where("id = ? AND user_id = ?", storyID, userID).Delete(&Story{}).Error
	})
}

// CleanupExpiredStories removes expired stories
func CleanupExpiredStories(db *gorm.DB) error {
	now := time.Now()

	// Get expired story IDs
	var expiredIDs []string
	db.Model(&Story{}).Where("expires_at < ?", now).Pluck("id", &expiredIDs)

	if len(expiredIDs) == 0 {
		return nil
	}

	// Delete views for expired stories
	db.Where("story_id IN ?", expiredIDs).Delete(&StoryView{})

	// Delete expired stories
	return db.Where("expires_at < ?", now).Delete(&Story{}).Error
}

// GetStoriesByUsers returns stories grouped by user
func GetStoriesByUsers(db *gorm.DB, viewerID string) (map[string][]Story, error) {
	stories, err := GetActiveStories(db, viewerID)
	if err != nil {
		return nil, err
	}

	grouped := make(map[string][]Story)
	for _, story := range stories {
		grouped[story.UserID] = append(grouped[story.UserID], story)
	}

	return grouped, nil
}

// HasViewedStory checks if a user has viewed a story
func HasViewedStory(db *gorm.DB, storyID, viewerID string) bool {
	var count int64
	db.Model(&StoryView{}).Where("story_id = ? AND viewer_id = ?", storyID, viewerID).Count(&count)
	return count > 0
}

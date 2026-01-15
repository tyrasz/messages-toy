package handlers

import (
	"encoding/json"
	"time"

	"github.com/gofiber/fiber/v2"
	"messenger/internal/api/middleware"
	"messenger/internal/database"
	"messenger/internal/models"
	"messenger/internal/websocket"
)

type StoriesHandler struct {
	hub *websocket.Hub
}

func NewStoriesHandler(hub *websocket.Hub) *StoriesHandler {
	return &StoriesHandler{hub: hub}
}

type CreateStoryRequest struct {
	Content         string  `json:"content,omitempty"`
	MediaID         *string `json:"media_id,omitempty"`
	BackgroundColor string  `json:"background_color,omitempty"`
	TextColor       string  `json:"text_color,omitempty"`
	FontStyle       string  `json:"font_style,omitempty"`
	Privacy         string  `json:"privacy,omitempty"` // "everyone", "contacts", "close_friends"
}

// Create creates a new story
func (h *StoriesHandler) Create(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	var req CreateStoryRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if req.Content == "" && req.MediaID == nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Content or media_id is required",
		})
	}

	// Validate media if provided
	if req.MediaID != nil {
		var media models.Media
		if err := database.DB.First(&media, "id = ?", *req.MediaID).Error; err != nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Media not found",
			})
		}
		if media.Status != models.MediaStatusApproved {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Media not approved",
			})
		}
	}

	// Set defaults
	if req.Privacy == "" {
		req.Privacy = "contacts"
	}
	if req.BackgroundColor == "" {
		req.BackgroundColor = "#1a1a1a"
	}
	if req.TextColor == "" {
		req.TextColor = "#ffffff"
	}

	story, err := models.CreateStory(
		database.DB,
		userID,
		req.Content,
		req.MediaID,
		req.BackgroundColor,
		req.TextColor,
		req.FontStyle,
		req.Privacy,
	)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create story",
		})
	}

	// Notify contacts about new story via WebSocket
	if h.hub != nil {
		h.broadcastNewStory(userID, story)
	}

	return c.Status(fiber.StatusCreated).JSON(h.formatStoryResponse(story, userID))
}

// List returns all active stories the user can see, grouped by user
func (h *StoriesHandler) List(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	stories, err := models.GetActiveStories(database.DB, userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch stories",
		})
	}

	// Group by user
	userStories := make(map[string][]fiber.Map)
	userInfo := make(map[string]fiber.Map)

	for _, story := range stories {
		storyMap := h.formatStoryResponse(&story, userID)

		if _, exists := userStories[story.UserID]; !exists {
			userStories[story.UserID] = make([]fiber.Map, 0)
			online := h.hub != nil && h.hub.IsOnline(story.UserID)
			userInfo[story.UserID] = fiber.Map{
				"id":           story.UserID,
				"username":     story.User.Username,
				"display_name": story.User.DisplayName,
				"avatar_url":   story.User.AvatarURL,
				"online":       online,
			}
		}
		userStories[story.UserID] = append(userStories[story.UserID], storyMap)
	}

	// Build response
	result := make([]fiber.Map, 0, len(userStories))
	for uid, stories := range userStories {
		result = append(result, fiber.Map{
			"user":    userInfo[uid],
			"stories": stories,
		})
	}

	return c.JSON(fiber.Map{
		"story_users": result,
	})
}

// GetMyStories returns the current user's stories
func (h *StoriesHandler) GetMyStories(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	stories, err := models.GetUserStories(database.DB, userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch stories",
		})
	}

	result := make([]fiber.Map, len(stories))
	for i, story := range stories {
		result[i] = h.formatStoryResponse(&story, userID)
		// Include view count for own stories
		result[i]["view_count"] = story.ViewCount
	}

	return c.JSON(fiber.Map{
		"stories": result,
	})
}

// Get returns a specific story
func (h *StoriesHandler) Get(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	storyID := c.Params("id")

	var story models.Story
	if err := database.DB.Preload("User").Preload("Media").First(&story, "id = ?", storyID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Story not found",
		})
	}

	if story.IsExpired() {
		return c.Status(fiber.StatusGone).JSON(fiber.Map{
			"error": "Story has expired",
		})
	}

	return c.JSON(h.formatStoryResponse(&story, userID))
}

// View marks a story as viewed
func (h *StoriesHandler) View(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	storyID := c.Params("id")

	var story models.Story
	if err := database.DB.First(&story, "id = ?", storyID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Story not found",
		})
	}

	if story.IsExpired() {
		return c.Status(fiber.StatusGone).JSON(fiber.Map{
			"error": "Story has expired",
		})
	}

	// Record view
	models.ViewStory(database.DB, storyID, userID)

	// Notify story owner via WebSocket
	if h.hub != nil && story.UserID != userID {
		viewEvent := map[string]interface{}{
			"type":      "story_view",
			"story_id":  storyID,
			"viewer_id": userID,
			"viewed_at": time.Now().Format(time.RFC3339),
		}
		eventBytes, _ := json.Marshal(viewEvent)
		h.hub.SendToUser(story.UserID, eventBytes)
	}

	return c.JSON(fiber.Map{
		"message": "Story viewed",
	})
}

// GetViews returns who viewed a story (only for story owner)
func (h *StoriesHandler) GetViews(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	storyID := c.Params("id")

	var story models.Story
	if err := database.DB.First(&story, "id = ?", storyID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Story not found",
		})
	}

	if story.UserID != userID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "You can only view viewers of your own stories",
		})
	}

	views, err := models.GetStoryViews(database.DB, storyID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch views",
		})
	}

	result := make([]fiber.Map, len(views))
	for i, view := range views {
		online := h.hub != nil && h.hub.IsOnline(view.ViewerID)
		result[i] = fiber.Map{
			"viewer": fiber.Map{
				"id":           view.Viewer.ID,
				"username":     view.Viewer.Username,
				"display_name": view.Viewer.DisplayName,
				"avatar_url":   view.Viewer.AvatarURL,
				"online":       online,
			},
			"viewed_at": view.ViewedAt,
		}
	}

	return c.JSON(fiber.Map{
		"story_id":   storyID,
		"view_count": len(views),
		"views":      result,
	})
}

// Delete removes a story
func (h *StoriesHandler) Delete(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	storyID := c.Params("id")

	if err := models.DeleteStory(database.DB, storyID, userID); err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Story not found or not yours",
		})
	}

	return c.JSON(fiber.Map{
		"message": "Story deleted",
	})
}

func (h *StoriesHandler) formatStoryResponse(story *models.Story, viewerID string) fiber.Map {
	response := fiber.Map{
		"id":               story.ID,
		"user_id":          story.UserID,
		"content":          story.Content,
		"media_id":         story.MediaID,
		"media_url":        story.MediaURL,
		"media_type":       story.MediaType,
		"background_color": story.BackgroundColor,
		"text_color":       story.TextColor,
		"font_style":       story.FontStyle,
		"privacy":          story.Privacy,
		"expires_at":       story.ExpiresAt,
		"created_at":       story.CreatedAt,
		"viewed":           models.HasViewedStory(database.DB, story.ID, viewerID),
	}

	if story.User.ID != "" {
		online := h.hub != nil && h.hub.IsOnline(story.UserID)
		response["user"] = fiber.Map{
			"id":           story.User.ID,
			"username":     story.User.Username,
			"display_name": story.User.DisplayName,
			"avatar_url":   story.User.AvatarURL,
			"online":       online,
		}
	}

	return response
}

func (h *StoriesHandler) broadcastNewStory(userID string, story *models.Story) {
	// Get user's contacts to notify
	var contacts []models.Contact
	database.DB.Where("user_id = ?", userID).Find(&contacts)

	event := map[string]interface{}{
		"type":     "new_story",
		"user_id":  userID,
		"story_id": story.ID,
	}
	eventBytes, _ := json.Marshal(event)

	for _, contact := range contacts {
		h.hub.SendToUser(contact.ContactID, eventBytes)
	}
}

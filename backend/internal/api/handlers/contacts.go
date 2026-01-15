package handlers

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"messenger/internal/api/middleware"
	"messenger/internal/database"
	"messenger/internal/models"
	"messenger/internal/websocket"
)

type ContactsHandler struct {
	hub *websocket.Hub
}

func NewContactsHandler(hub *websocket.Hub) *ContactsHandler {
	return &ContactsHandler{hub: hub}
}

func (h *ContactsHandler) List(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	var contacts []models.Contact
	if err := database.DB.Preload("ContactUser").Where("user_id = ?", userID).Find(&contacts).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch contacts",
		})
	}

	// Add online status
	response := make([]fiber.Map, len(contacts))
	for i, contact := range contacts {
		online := false
		if h.hub != nil {
			online = h.hub.IsOnline(contact.ContactID)
		}
		response[i] = fiber.Map{
			"id":         contact.ID,
			"contact_id": contact.ContactID,
			"nickname":   contact.Nickname,
			"user":       contact.ContactUser.ToResponse(online),
			"created_at": contact.CreatedAt,
		}
	}

	return c.JSON(fiber.Map{
		"contacts": response,
	})
}

type AddContactInput struct {
	Username string `json:"username"`
	Nickname string `json:"nickname,omitempty"`
}

func (h *ContactsHandler) Add(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	var input AddContactInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if input.Username == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Username is required",
		})
	}

	// Find user to add
	var contactUser models.User
	if err := database.DB.Where("username = ?", input.Username).First(&contactUser).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "User not found",
		})
	}

	// Can't add yourself
	if contactUser.ID == userID {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Cannot add yourself as a contact",
		})
	}

	// Check if already a contact
	var existingContact models.Contact
	if err := database.DB.Where("user_id = ? AND contact_id = ?", userID, contactUser.ID).First(&existingContact).Error; err == nil {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"error": "Contact already exists",
		})
	}

	// Create contact
	contact := models.Contact{
		UserID:    userID,
		ContactID: contactUser.ID,
		Nickname:  input.Nickname,
	}

	if err := database.DB.Create(&contact).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to add contact",
		})
	}

	online := false
	if h.hub != nil {
		online = h.hub.IsOnline(contactUser.ID)
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"id":         contact.ID,
		"contact_id": contact.ContactID,
		"nickname":   contact.Nickname,
		"user":       contactUser.ToResponse(online),
		"created_at": contact.CreatedAt,
	})
}

func (h *ContactsHandler) Remove(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	contactID := c.Params("id")

	result := database.DB.Where("id = ? AND user_id = ?", contactID, userID).Delete(&models.Contact{})
	if result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to remove contact",
		})
	}

	if result.RowsAffected == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Contact not found",
		})
	}

	return c.JSON(fiber.Map{
		"message": "Contact removed successfully",
	})
}

// Block adds a user to the blocker's block list
func (h *ContactsHandler) Block(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	blockedUserID := c.Params("userId")

	if blockedUserID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "User ID is required",
		})
	}

	// Can't block yourself
	if blockedUserID == userID {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Cannot block yourself",
		})
	}

	// Check if user exists
	var blockedUser models.User
	if err := database.DB.First(&blockedUser, "id = ?", blockedUserID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "User not found",
		})
	}

	// Check if already blocked
	var existingBlock models.Block
	if err := database.DB.Where("blocker_id = ? AND blocked_id = ?", userID, blockedUserID).First(&existingBlock).Error; err == nil {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"error": "User is already blocked",
		})
	}

	// Create block
	block := models.Block{
		BlockerID: userID,
		BlockedID: blockedUserID,
	}

	if err := database.DB.Create(&block).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to block user",
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"id":           block.ID,
		"blocked_id":   block.BlockedID,
		"blocked_user": blockedUser.ToResponse(false),
		"created_at":   block.CreatedAt,
	})
}

// Unblock removes a user from the blocker's block list
func (h *ContactsHandler) Unblock(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	blockedUserID := c.Params("userId")

	result := database.DB.Where("blocker_id = ? AND blocked_id = ?", userID, blockedUserID).Delete(&models.Block{})
	if result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to unblock user",
		})
	}

	if result.RowsAffected == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Block not found",
		})
	}

	return c.JSON(fiber.Map{
		"message": "User unblocked successfully",
	})
}

// ListBlocked returns all users blocked by the current user
func (h *ContactsHandler) ListBlocked(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	var blocks []models.Block
	if err := database.DB.Preload("Blocked").Where("blocker_id = ?", userID).Find(&blocks).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch blocked users",
		})
	}

	response := make([]fiber.Map, len(blocks))
	for i, block := range blocks {
		response[i] = fiber.Map{
			"id":           block.ID,
			"blocked_id":   block.BlockedID,
			"blocked_user": block.Blocked.ToResponse(false),
			"created_at":   block.CreatedAt,
		}
	}

	return c.JSON(fiber.Map{
		"blocked": response,
	})
}

// IsBlocked checks if either user has blocked the other (for internal use)
func (h *ContactsHandler) IsBlocked(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	otherUserID := c.Params("userId")

	blocked := models.IsEitherBlocked(database.DB, userID, otherUserID)

	return c.JSON(fiber.Map{
		"blocked": blocked,
	})
}

// SearchUsers searches for users by username or display name
func (h *ContactsHandler) SearchUsers(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	query := c.Query("q")

	if len(query) < 2 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Search query must be at least 2 characters",
		})
	}

	limit := c.QueryInt("limit", 20)
	if limit > 50 {
		limit = 50
	}
	offset := c.QueryInt("offset", 0)

	// Search for users matching the query (exclude self and blocked users)
	var users []models.User

	// Get IDs of users who have blocked the current user or vice versa
	var blocks []models.Block
	database.DB.Where("blocker_id = ? OR blocked_id = ?", userID, userID).Find(&blocks)

	blockedIDs := make([]string, 0)
	for _, block := range blocks {
		if block.BlockerID == userID {
			blockedIDs = append(blockedIDs, block.BlockedID)
		} else {
			blockedIDs = append(blockedIDs, block.BlockerID)
		}
	}

	// Build query - search username and display_name, exclude self and blocked
	// Use LOWER() for case-insensitive search (works with both PostgreSQL and SQLite)
	lowerPattern := "%" + strings.ToLower(query) + "%"
	db := database.DB.Where("id != ?", userID).
		Where("(LOWER(username) LIKE ? OR LOWER(COALESCE(display_name, '')) LIKE ?)", lowerPattern, lowerPattern)

	if len(blockedIDs) > 0 {
		db = db.Where("id NOT IN ?", blockedIDs)
	}

	// Get total count
	var total int64
	db.Model(&models.User{}).Count(&total)

	// Get results with pagination
	if err := db.Limit(limit).Offset(offset).Find(&users).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to search users",
		})
	}

	// Check which users are already contacts
	var contactIDs []string
	database.DB.Model(&models.Contact{}).Where("user_id = ?", userID).Pluck("contact_id", &contactIDs)
	contactMap := make(map[string]bool)
	for _, id := range contactIDs {
		contactMap[id] = true
	}

	// Build response
	response := make([]fiber.Map, len(users))
	for i, user := range users {
		online := false
		if h.hub != nil {
			online = h.hub.IsOnline(user.ID)
		}
		response[i] = fiber.Map{
			"user":       user.ToResponse(online),
			"is_contact": contactMap[user.ID],
		}
	}

	return c.JSON(fiber.Map{
		"users":  response,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

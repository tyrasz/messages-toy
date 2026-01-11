package handlers

import (
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

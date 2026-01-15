package middleware

import (
	"github.com/gofiber/fiber/v2"
	"messenger/internal/database"
	"messenger/internal/models"
)

// ModeratorRequired ensures the user has moderator or admin role
func ModeratorRequired() fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID := GetUserID(c)
		if userID == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Authentication required",
			})
		}

		var user models.User
		if err := database.DB.First(&user, "id = ?", userID).Error; err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "User not found",
			})
		}

		if !user.IsModerator() {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "Moderator privileges required",
			})
		}

		// Store role in context for handlers
		c.Locals("userRole", user.Role)

		return c.Next()
	}
}

// AdminRequired ensures the user has admin role
func AdminRequired() fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID := GetUserID(c)
		if userID == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Authentication required",
			})
		}

		var user models.User
		if err := database.DB.First(&user, "id = ?", userID).Error; err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "User not found",
			})
		}

		if !user.IsAdmin() {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "Admin privileges required",
			})
		}

		// Store role in context for handlers
		c.Locals("userRole", user.Role)

		return c.Next()
	}
}

// GetUserRole returns the user's role from context
func GetUserRole(c *fiber.Ctx) models.UserRole {
	role, _ := c.Locals("userRole").(models.UserRole)
	return role
}

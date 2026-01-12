package api

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/contrib/websocket"
	"messenger/internal/api/handlers"
	"messenger/internal/api/middleware"
	"messenger/internal/services"
	ws "messenger/internal/websocket"
)

func SetupRoutes(app *fiber.App, hub *ws.Hub) {
	// Health check
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	// API routes
	api := app.Group("/api")

	// Auth routes (public)
	authHandler := handlers.NewAuthHandler()
	auth := api.Group("/auth")
	auth.Post("/register", authHandler.Register)
	auth.Post("/login", authHandler.Login)
	auth.Post("/refresh", authHandler.Refresh)

	// Protected routes
	protected := api.Group("", middleware.AuthRequired())

	// Contacts
	contactsHandler := handlers.NewContactsHandler(hub)
	contacts := protected.Group("/contacts")
	contacts.Get("/", contactsHandler.List)
	contacts.Post("/", contactsHandler.Add)
	contacts.Delete("/:id", contactsHandler.Remove)

	// Messages
	messagesHandler := handlers.NewMessagesHandler()
	messages := protected.Group("/messages")
	messages.Get("/conversations", messagesHandler.GetConversations)
	messages.Get("/:userId", messagesHandler.GetHistory)

	// Groups
	groupsHandler := handlers.NewGroupsHandler(hub)
	groups := protected.Group("/groups")
	groups.Post("/", groupsHandler.Create)
	groups.Get("/", groupsHandler.List)
	groups.Get("/:id", groupsHandler.Get)
	groups.Post("/:id/members", groupsHandler.AddMember)
	groups.Delete("/:id/members/:userId", groupsHandler.RemoveMember)
	groups.Post("/:id/leave", groupsHandler.Leave)
	groups.Get("/:id/messages", groupsHandler.GetMessages)

	// Media
	mediaHandler := handlers.NewMediaHandler(hub)
	media := protected.Group("/media")
	media.Post("/upload", mediaHandler.Upload)
	media.Get("/:id", mediaHandler.Get)

	// Admin routes (for moderation review)
	adminHandler := handlers.NewAdminHandler()
	admin := protected.Group("/admin")
	admin.Get("/review", adminHandler.GetPendingReview)
	admin.Post("/review/:id", adminHandler.Review)

	// WebSocket endpoint
	app.Use("/ws", func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})

	app.Get("/ws", websocket.New(func(conn *websocket.Conn) {
		// Get token from query param
		token := conn.Query("token")
		if token == "" {
			conn.Close()
			return
		}

		claims, err := services.ValidateToken(token)
		if err != nil {
			conn.Close()
			return
		}

		client := ws.NewClient(hub, conn, claims.UserID, claims.Username)
		hub.Register(client)

		go client.WritePump()
		client.ReadPump()
	}))
}

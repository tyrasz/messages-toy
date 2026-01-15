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

	// Auth routes (public) - rate limited to prevent brute force
	authHandler := handlers.NewAuthHandler()
	auth := api.Group("/auth", middleware.AuthLimiter)
	auth.Post("/register", authHandler.Register)
	auth.Post("/login", authHandler.Login)
	auth.Post("/refresh", authHandler.Refresh)

	// Protected routes with general API rate limiting
	protected := api.Group("", middleware.AuthRequired(), middleware.APILimiter)

	// Contacts
	contactsHandler := handlers.NewContactsHandler(hub)
	contacts := protected.Group("/contacts")
	contacts.Get("/", contactsHandler.List)
	contacts.Post("/", contactsHandler.Add)
	contacts.Delete("/:id", contactsHandler.Remove)

	// Blocks
	blocks := protected.Group("/blocks")
	blocks.Get("/", contactsHandler.ListBlocked)
	blocks.Post("/:userId", contactsHandler.Block)
	blocks.Delete("/:userId", contactsHandler.Unblock)
	blocks.Get("/:userId", contactsHandler.IsBlocked)

	// Users (search)
	users := protected.Group("/users")
	users.Get("/search", contactsHandler.SearchUsers)

	// Messages
	messagesHandler := handlers.NewMessagesHandler(hub)
	messages := protected.Group("/messages")
	messages.Get("/conversations", messagesHandler.GetConversations)
	messages.Get("/search", messagesHandler.Search)
	messages.Get("/export", messagesHandler.Export)
	messages.Post("/location", messagesHandler.SendLocation)
	messages.Get("/scheduled", messagesHandler.GetScheduledMessages)
	messages.Post("/scheduled", messagesHandler.ScheduleMessage)
	messages.Delete("/scheduled/:id", messagesHandler.CancelScheduledMessage)
	messages.Get("/:userId", messagesHandler.GetHistory)
	messages.Post("/:id/forward", messagesHandler.Forward)
	messages.Get("/:id/reactions", messagesHandler.GetReactions)
	messages.Post("/:id/reactions", messagesHandler.AddReaction)
	messages.Delete("/:id/reactions", messagesHandler.RemoveReaction)

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

	// Media - with stricter rate limiting for uploads
	mediaHandler := handlers.NewMediaHandler(hub)
	media := protected.Group("/media")
	media.Post("/upload", middleware.MediaLimiter, mediaHandler.Upload)
	media.Get("/:id", mediaHandler.Get)
	media.Get("/:id/thumbnail", mediaHandler.GetThumbnail)

	// Notifications (push)
	notificationsHandler := handlers.NewNotificationsHandler()
	notifications := protected.Group("/notifications")
	notifications.Post("/register", notificationsHandler.RegisterToken)
	notifications.Post("/unregister", notificationsHandler.UnregisterToken)
	notifications.Delete("/all", notificationsHandler.UnregisterAllTokens)
	notifications.Get("/tokens", notificationsHandler.GetTokens)
	notifications.Post("/test", notificationsHandler.TestNotification)

	// Link previews
	linkPreviewHandler := handlers.NewLinkPreviewHandler()
	links := protected.Group("/links")
	links.Post("/preview", linkPreviewHandler.FetchPreview)
	links.Get("/preview", linkPreviewHandler.GetPreview)

	// Starred messages
	starredHandler := handlers.NewStarredHandler()
	starred := protected.Group("/starred")
	starred.Get("/", starredHandler.List)
	starred.Post("/:messageId", starredHandler.Star)
	starred.Delete("/:messageId", starredHandler.Unstar)
	starred.Get("/:messageId", starredHandler.IsStarred)

	// Conversation settings (disappearing messages, mute)
	settingsHandler := handlers.NewSettingsHandler()
	settings := protected.Group("/settings")
	settings.Get("/conversation", settingsHandler.GetConversationSettings)
	settings.Post("/disappearing", settingsHandler.SetDisappearingMessages)
	settings.Post("/mute", settingsHandler.MuteConversation)

	// Themes
	themesHandler := handlers.NewThemesHandler()
	themes := protected.Group("/themes")
	themes.Get("/", themesHandler.GetTheme)
	themes.Post("/", themesHandler.SetTheme)
	themes.Delete("/", themesHandler.DeleteTheme)
	themes.Get("/presets", themesHandler.GetPresets)

	// Stories/Status
	storiesHandler := handlers.NewStoriesHandler(hub)
	stories := protected.Group("/stories")
	stories.Post("/", storiesHandler.Create)
	stories.Get("/", storiesHandler.List)
	stories.Get("/mine", storiesHandler.GetMyStories)
	stories.Get("/:id", storiesHandler.Get)
	stories.Post("/:id/view", storiesHandler.View)
	stories.Get("/:id/views", storiesHandler.GetViews)
	stories.Delete("/:id", storiesHandler.Delete)

	// Polls
	pollHandler := handlers.NewPollHandler(hub)
	polls := protected.Group("/polls")
	polls.Post("/", pollHandler.Create)
	polls.Get("/:id", pollHandler.Get)
	polls.Post("/:id/vote", pollHandler.Vote)
	polls.Post("/:id/close", pollHandler.Close)

	// Pinned messages
	pinnedHandler := handlers.NewPinnedHandler(hub)
	pinned := protected.Group("/pinned")
	pinned.Post("/", pinnedHandler.Pin)
	pinned.Delete("/", pinnedHandler.Unpin)
	pinned.Get("/", pinnedHandler.Get)

	// Admin routes (for moderation review) - requires moderator role
	adminHandler := handlers.NewAdminHandler()
	admin := protected.Group("/admin", middleware.ModeratorRequired())
	admin.Get("/review", adminHandler.GetPendingReview)
	admin.Post("/review/:id", adminHandler.Review)

	// Profile routes
	profileHandler := handlers.NewProfileHandler(hub)
	profile := protected.Group("/profile")
	profile.Get("/", profileHandler.GetProfile)
	profile.Put("/", profileHandler.UpdateProfile)
	profile.Get("/:userId", profileHandler.GetUserProfile)

	// Archive routes
	archiveHandler := handlers.NewArchiveHandler()
	archive := protected.Group("/archive")
	archive.Post("/", archiveHandler.Archive)
	archive.Delete("/", archiveHandler.Unarchive)
	archive.Get("/", archiveHandler.List)
	archive.Get("/check", archiveHandler.IsArchived)

	// Read receipts
	readReceiptHandler := handlers.NewReadReceiptHandler(hub)
	receipts := protected.Group("/receipts")
	receipts.Post("/read", readReceiptHandler.MarkRead)
	receipts.Get("/:messageId", readReceiptHandler.GetReceipts)
	receipts.Get("/unread", readReceiptHandler.GetUnreadCount)

	// Broadcast lists
	broadcastHandler := handlers.NewBroadcastHandler(hub)
	broadcast := protected.Group("/broadcast")
	broadcast.Post("/", broadcastHandler.Create)
	broadcast.Get("/", broadcastHandler.List)
	broadcast.Get("/:id", broadcastHandler.Get)
	broadcast.Put("/:id", broadcastHandler.Update)
	broadcast.Delete("/:id", broadcastHandler.Delete)
	broadcast.Post("/:id/send", broadcastHandler.Send)
	broadcast.Post("/:id/recipients", broadcastHandler.AddRecipient)
	broadcast.Delete("/:id/recipients/:recipientId", broadcastHandler.RemoveRecipient)

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

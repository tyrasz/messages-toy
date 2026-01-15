package main

import (
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"messenger/internal/api"
	"messenger/internal/database"
	"messenger/internal/models"
	"messenger/internal/services"
	"messenger/internal/websocket"
)

func main() {
	// Initialize database
	database.Init()
	database.Migrate(
		&models.User{},
		&models.Message{},
		&models.MessageDeletion{},
		&models.Contact{},
		&models.Media{},
		&models.Group{},
		&models.GroupMember{},
		&models.Block{},
		&models.DeviceToken{},
		&models.Reaction{},
		&models.LinkPreview{},
		&models.StarredMessage{},
		&models.ConversationSettings{},
		&models.Poll{},
		&models.PollOption{},
		&models.PollVote{},
		&models.PinnedMessage{},
		&models.MessageReadReceipt{},
		&models.ArchivedConversation{},
		&models.BroadcastList{},
		&models.BroadcastListRecipient{},
		&models.ChatTheme{},
		&models.Story{},
		&models.StoryView{},
	)

	// Create WebSocket hub
	hub := websocket.NewHub()
	go hub.Run()

	// Create bot user if not exists
	createBotUser()

	// Start message cleanup service (for disappearing messages)
	cleanupService := services.NewMessageCleanupService(database.DB, 1*time.Minute)
	cleanupService.Start()

	// Start scheduled message service
	schedulerService := services.NewSchedulerService(func(msg *models.Message) {
		deliverScheduledMessage(hub, msg)
	})
	schedulerService.Start()

	// Create Fiber app
	app := fiber.New(fiber.Config{
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}
			return c.Status(code).JSON(fiber.Map{
				"error": err.Error(),
			})
		},
	})

	// Middleware
	app.Use(recover.New())
	app.Use(logger.New(logger.Config{
		Format: "${time} ${status} ${method} ${path} ${latency}\n",
	}))
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowHeaders: "Origin, Content-Type, Accept, Authorization",
	}))

	// Serve uploaded files
	app.Static("/uploads", "./uploads/approved")

	// Setup routes
	api.SetupRoutes(app, hub)

	// Create upload directories
	os.MkdirAll("./uploads/quarantine", 0755)
	os.MkdirAll("./uploads/approved", 0755)

	// Get port from environment or default
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		log.Println("Shutting down server...")
		app.Shutdown()
	}()

	// Start server
	log.Printf("Server starting on port %s", port)
	if err := app.Listen(":" + port); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}

// deliverScheduledMessage delivers a scheduled message via WebSocket
func deliverScheduledMessage(hub *websocket.Hub, msg *models.Message) {
	outMsg := map[string]interface{}{
		"type":       "message",
		"id":         msg.ID,
		"from":       msg.SenderID,
		"content":    msg.Content,
		"created_at": msg.CreatedAt.Format(time.RFC3339),
	}

	if msg.MediaID != nil {
		outMsg["media_id"] = *msg.MediaID
	}
	if msg.Latitude != nil {
		outMsg["latitude"] = *msg.Latitude
	}
	if msg.Longitude != nil {
		outMsg["longitude"] = *msg.Longitude
	}
	if msg.LocationName != nil {
		outMsg["location_name"] = *msg.LocationName
	}

	if msg.GroupID != nil {
		outMsg["group_id"] = *msg.GroupID
		msgBytes, _ := json.Marshal(outMsg)
		sentCount := hub.SendToGroup(*msg.GroupID, msg.SenderID, msgBytes)
		if sentCount > 0 {
			database.DB.Model(msg).Update("status", models.MessageStatusDelivered)
		}
		// Push to offline members
		offlineMembers := hub.GetOfflineGroupMemberIDs(*msg.GroupID, msg.SenderID)
		for _, memberID := range offlineMembers {
			services.PushMessageToOfflineUser(database.DB, memberID, msg.SenderID, msg.Content, true, *msg.GroupID)
		}
	} else if msg.RecipientID != nil {
		outMsg["to"] = *msg.RecipientID
		msgBytes, _ := json.Marshal(outMsg)
		if hub.SendToUser(*msg.RecipientID, msgBytes) {
			database.DB.Model(msg).Update("status", models.MessageStatusDelivered)
		} else {
			services.PushMessageToOfflineUser(database.DB, *msg.RecipientID, msg.SenderID, msg.Content, false, *msg.RecipientID)
		}
	}
}

// createBotUser ensures the bot user exists in the database
func createBotUser() {
	var existingBot models.User
	result := database.DB.Where("id = ?", services.BotUserID).First(&existingBot)

	if result.Error != nil {
		// Create bot user
		botUser := models.User{
			ID:           services.BotUserID,
			Username:     services.BotUsername,
			DisplayName:  services.BotDisplayName,
			PasswordHash: "", // Bot doesn't need a password
		}
		if err := database.DB.Create(&botUser).Error; err != nil {
			log.Printf("Warning: Could not create bot user: %v", err)
		} else {
			log.Println("Bot user created successfully")
		}
	}
}

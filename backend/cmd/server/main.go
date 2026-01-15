package main

import (
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
	)

	// Create WebSocket hub
	hub := websocket.NewHub()
	go hub.Run()

	// Create bot user if not exists
	createBotUser()

	// Start message cleanup service (for disappearing messages)
	cleanupService := services.NewMessageCleanupService(database.DB, 1*time.Minute)
	cleanupService.Start()

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

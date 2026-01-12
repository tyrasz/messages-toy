package services

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"google.golang.org/api/option"
	"gorm.io/gorm"
	"messenger/internal/database"
	"messenger/internal/models"
)

type PushService struct {
	client *messaging.Client
	mu     sync.RWMutex
}

var (
	pushService     *PushService
	pushServiceOnce sync.Once
)

// GetPushService returns the singleton push service instance
func GetPushService() *PushService {
	pushServiceOnce.Do(func() {
		pushService = &PushService{}
		if err := pushService.initialize(); err != nil {
			log.Printf("Warning: Push notifications disabled - %v", err)
		}
	})
	return pushService
}

func (ps *PushService) initialize() error {
	credPath := os.Getenv("FIREBASE_CREDENTIALS_PATH")
	if credPath == "" {
		return fmt.Errorf("FIREBASE_CREDENTIALS_PATH not set")
	}

	opt := option.WithCredentialsFile(credPath)
	app, err := firebase.NewApp(context.Background(), nil, opt)
	if err != nil {
		return fmt.Errorf("failed to create Firebase app: %w", err)
	}

	client, err := app.Messaging(context.Background())
	if err != nil {
		return fmt.Errorf("failed to get messaging client: %w", err)
	}

	ps.mu.Lock()
	ps.client = client
	ps.mu.Unlock()

	log.Println("Push notification service initialized successfully")
	return nil
}

// IsEnabled returns whether push notifications are enabled
func (ps *PushService) IsEnabled() bool {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return ps.client != nil
}

// SendToUser sends a push notification to all devices of a user
func (ps *PushService) SendToUser(userID string, notification *Notification) error {
	if !ps.IsEnabled() {
		return nil // Silently skip if push is not configured
	}

	tokens, err := models.GetUserTokens(database.DB, userID)
	if err != nil {
		return fmt.Errorf("failed to get user tokens: %w", err)
	}

	if len(tokens) == 0 {
		return nil // User has no registered devices
	}

	// Extract token strings
	tokenStrings := make([]string, len(tokens))
	for i, t := range tokens {
		tokenStrings[i] = t.Token
	}

	return ps.SendToTokens(tokenStrings, notification)
}

// SendToTokens sends a push notification to specific device tokens
func (ps *PushService) SendToTokens(tokens []string, notification *Notification) error {
	if !ps.IsEnabled() {
		return nil
	}

	if len(tokens) == 0 {
		return nil
	}

	ps.mu.RLock()
	client := ps.client
	ps.mu.RUnlock()

	message := &messaging.MulticastMessage{
		Tokens: tokens,
		Notification: &messaging.Notification{
			Title: notification.Title,
			Body:  notification.Body,
		},
		Data: notification.Data,
		Android: &messaging.AndroidConfig{
			Priority: "high",
			Notification: &messaging.AndroidNotification{
				ClickAction: "FLUTTER_NOTIFICATION_CLICK",
				ChannelID:   "messages",
			},
		},
		APNS: &messaging.APNSConfig{
			Payload: &messaging.APNSPayload{
				Aps: &messaging.Aps{
					Badge:            notification.Badge,
					Sound:            "default",
					ContentAvailable: true,
					MutableContent:   true,
				},
			},
		},
	}

	response, err := client.SendEachForMulticast(context.Background(), message)
	if err != nil {
		return fmt.Errorf("failed to send multicast message: %w", err)
	}

	// Log failures and clean up invalid tokens
	if response.FailureCount > 0 {
		for i, resp := range response.Responses {
			if !resp.Success {
				log.Printf("Failed to send to token %s: %v", tokens[i], resp.Error)
				// Remove invalid tokens
				if isInvalidTokenError(resp.Error) {
					models.UnregisterToken(database.DB, tokens[i])
				}
			}
		}
	}

	log.Printf("Push notification sent: %d success, %d failures",
		response.SuccessCount, response.FailureCount)

	return nil
}

// Notification represents a push notification to send
type Notification struct {
	Title string
	Body  string
	Data  map[string]string
	Badge *int
}

// NewMessageNotification creates a notification for a new message
func NewMessageNotification(senderName, content, conversationID string, isGroup bool) *Notification {
	title := senderName
	body := content
	if len(body) > 100 {
		body = body[:97] + "..."
	}
	if body == "" {
		body = "Sent an attachment"
	}

	data := map[string]string{
		"type":            "new_message",
		"conversation_id": conversationID,
	}
	if isGroup {
		data["is_group"] = "true"
	}

	return &Notification{
		Title: title,
		Body:  body,
		Data:  data,
	}
}

// isInvalidTokenError checks if the error indicates an invalid token
func isInvalidTokenError(err error) bool {
	if err == nil {
		return false
	}
	// FCM returns specific error codes for invalid tokens
	errStr := err.Error()
	return errStr == "registration-token-not-registered" ||
		errStr == "invalid-registration-token" ||
		errStr == "invalid-argument"
}

// SendTestNotification sends a test notification to a specific token
func (ps *PushService) SendTestNotification(token string) error {
	if !ps.IsEnabled() {
		return fmt.Errorf("push notifications not enabled")
	}

	return ps.SendToTokens([]string{token}, &Notification{
		Title: "Test Notification",
		Body:  "Push notifications are working!",
		Data:  map[string]string{"type": "test"},
	})
}

// PushMessageToOfflineUser is a helper to send push notification for a message
// when the recipient is not connected via WebSocket
func PushMessageToOfflineUser(db *gorm.DB, recipientID string, senderID string, content string, isGroup bool, conversationID string) {
	pushSvc := GetPushService()
	if !pushSvc.IsEnabled() {
		return
	}

	// Get sender info
	var sender models.User
	if err := db.First(&sender, "id = ?", senderID).Error; err != nil {
		log.Printf("Failed to get sender for push notification: %v", err)
		return
	}

	senderName := sender.DisplayName
	if senderName == "" {
		senderName = sender.Username
	}

	notification := NewMessageNotification(senderName, content, conversationID, isGroup)
	if err := pushSvc.SendToUser(recipientID, notification); err != nil {
		log.Printf("Failed to send push notification: %v", err)
	}
}

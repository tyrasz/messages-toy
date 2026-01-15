package services

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"

	"gorm.io/gorm"
	"messenger/internal/database"
	"messenger/internal/models"
)

// PushService coordinates push notifications across multiple providers
// It abstracts away the specific push provider (FCM, APNs, Web Push, etc.)
type PushService struct {
	registry *ProviderRegistry
	mu       sync.RWMutex
}

var (
	pushService     *PushService
	pushServiceOnce sync.Once
)

// GetPushService returns the singleton push service instance
func GetPushService() *PushService {
	pushServiceOnce.Do(func() {
		pushService = &PushService{
			registry: NewProviderRegistry(),
		}
		pushService.initializeProviders()
	})
	return pushService
}

// initializeProviders sets up all configured push providers
func (ps *PushService) initializeProviders() {
	ctx := context.Background()

	// Initialize Firebase/FCM provider if configured
	if os.Getenv("FIREBASE_CREDENTIALS_PATH") != "" || os.Getenv("FIREBASE_CREDENTIALS_JSON") != "" {
		fcm := NewFirebasePushProvider()
		if err := fcm.Initialize(ctx); err != nil {
			log.Printf("Warning: Firebase push provider failed to initialize - %v", err)
		} else {
			ps.registry.Register(fcm)
		}
	}

	// Initialize APNs provider if configured (direct Apple Push without Firebase)
	if os.Getenv("APNS_BUNDLE_ID") != "" {
		apns := NewAPNsPushProvider()
		if err := apns.Initialize(ctx); err != nil {
			log.Printf("Warning: APNs push provider failed to initialize - %v", err)
		} else {
			ps.registry.Register(apns)
		}
	}

	// Initialize Web Push provider if configured (browsers without Firebase)
	if os.Getenv("VAPID_PUBLIC_KEY") != "" && os.Getenv("VAPID_PRIVATE_KEY") != "" {
		webpush := NewWebPushProvider()
		if err := webpush.Initialize(ctx); err != nil {
			log.Printf("Warning: Web Push provider failed to initialize - %v", err)
		} else {
			ps.registry.Register(webpush)
		}
	}

	enabledCount := len(ps.registry.GetEnabled())
	if enabledCount == 0 {
		log.Println("Warning: No push providers configured - push notifications disabled")
	} else {
		log.Printf("Push service initialized with %d provider(s)", enabledCount)
	}
}

// RegisterProvider allows registering custom push providers at runtime
func (ps *PushService) RegisterProvider(provider PushProvider) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.registry.Register(provider)
}

// GetProvider returns a specific provider by name
func (ps *PushService) GetProvider(name string) (PushProvider, bool) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return ps.registry.Get(name)
}

// IsEnabled returns whether any push provider is available
func (ps *PushService) IsEnabled() bool {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return len(ps.registry.GetEnabled()) > 0
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

	// Group tokens by platform/provider
	tokensByPlatform := make(map[models.DevicePlatform][]string)
	for _, t := range tokens {
		platform := t.Platform
		if platform == "" {
			platform = models.PlatformAndroid // Default
		}
		tokensByPlatform[platform] = append(tokensByPlatform[platform], t.Token)
	}

	// Send to each platform's provider
	var lastErr error
	for platform, platformTokens := range tokensByPlatform {
		provider, ok := ps.registry.Get(mapPlatformToProvider(string(platform)))
		if !ok || !provider.IsEnabled() {
			// Fall back to FCM for unknown platforms
			provider, ok = ps.registry.Get("fcm")
			if !ok || !provider.IsEnabled() {
				continue
			}
		}

		failedTokens, err := provider.Send(context.Background(), platformTokens, notification)
		if err != nil {
			log.Printf("Push provider %s error: %v", provider.Name(), err)
			lastErr = err
		}

		// Clean up invalid tokens
		for _, token := range failedTokens {
			models.UnregisterToken(database.DB, token)
		}
	}

	return lastErr
}

// SendToTokens sends a push notification to specific device tokens
func (ps *PushService) SendToTokens(tokens []string, notification *Notification) error {
	if !ps.IsEnabled() {
		return nil
	}

	if len(tokens) == 0 {
		return nil
	}

	// Use the first available provider (typically FCM)
	providers := ps.registry.GetEnabled()
	if len(providers) == 0 {
		return ErrNoProvidersAvailable
	}

	provider := providers[0]
	failedTokens, err := provider.Send(context.Background(), tokens, notification)
	if err != nil {
		return err
	}

	// Clean up invalid tokens
	for _, token := range failedTokens {
		models.UnregisterToken(database.DB, token)
	}

	return nil
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

// mapPlatformToProvider maps device platform to push provider name
func mapPlatformToProvider(platform string) string {
	switch platform {
	case "ios":
		return "fcm" // FCM handles APNs routing
	case "android":
		return "fcm"
	case "web":
		return "fcm" // Could be "webpush" if that provider is implemented
	default:
		return "fcm"
	}
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

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
)

// FirebasePushProvider implements PushProvider using Firebase Cloud Messaging
type FirebasePushProvider struct {
	client *messaging.Client
	mu     sync.RWMutex
}

// NewFirebasePushProvider creates a new Firebase push provider
func NewFirebasePushProvider() *FirebasePushProvider {
	return &FirebasePushProvider{}
}

func (p *FirebasePushProvider) Name() string {
	return "fcm"
}

func (p *FirebasePushProvider) Initialize(ctx context.Context) error {
	credPath := os.Getenv("FIREBASE_CREDENTIALS_PATH")
	if credPath == "" {
		// Also check for inline credentials
		credJSON := os.Getenv("FIREBASE_CREDENTIALS_JSON")
		if credJSON == "" {
			return fmt.Errorf("FIREBASE_CREDENTIALS_PATH or FIREBASE_CREDENTIALS_JSON not set")
		}
		return p.initializeWithJSON(ctx, []byte(credJSON))
	}

	return p.initializeWithFile(ctx, credPath)
}

func (p *FirebasePushProvider) initializeWithFile(ctx context.Context, credPath string) error {
	opt := option.WithCredentialsFile(credPath)
	return p.initializeWithOption(ctx, opt)
}

func (p *FirebasePushProvider) initializeWithJSON(ctx context.Context, credJSON []byte) error {
	opt := option.WithCredentialsJSON(credJSON)
	return p.initializeWithOption(ctx, opt)
}

func (p *FirebasePushProvider) initializeWithOption(ctx context.Context, opt option.ClientOption) error {
	app, err := firebase.NewApp(ctx, nil, opt)
	if err != nil {
		return fmt.Errorf("failed to create Firebase app: %w", err)
	}

	client, err := app.Messaging(ctx)
	if err != nil {
		return fmt.Errorf("failed to get messaging client: %w", err)
	}

	p.mu.Lock()
	p.client = client
	p.mu.Unlock()

	log.Println("Firebase push provider initialized")
	return nil
}

func (p *FirebasePushProvider) IsEnabled() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.client != nil
}

func (p *FirebasePushProvider) SupportsMulticast() bool {
	return true
}

func (p *FirebasePushProvider) Send(ctx context.Context, tokens []string, notification *Notification) ([]string, error) {
	if !p.IsEnabled() {
		return nil, ErrProviderNotConfigured
	}

	if len(tokens) == 0 {
		return nil, nil
	}

	p.mu.RLock()
	client := p.client
	p.mu.RUnlock()

	message := p.buildMessage(tokens, notification)

	response, err := client.SendEachForMulticast(ctx, message)
	if err != nil {
		return nil, fmt.Errorf("failed to send multicast message: %w", err)
	}

	// Collect failed tokens
	var failedTokens []string
	if response.FailureCount > 0 {
		for i, resp := range response.Responses {
			if !resp.Success {
				log.Printf("FCM: Failed to send to token: %v", resp.Error)
				if p.isInvalidTokenError(resp.Error) {
					failedTokens = append(failedTokens, tokens[i])
				}
			}
		}
	}

	log.Printf("FCM: %d success, %d failures", response.SuccessCount, response.FailureCount)
	return failedTokens, nil
}

func (p *FirebasePushProvider) buildMessage(tokens []string, notification *Notification) *messaging.MulticastMessage {
	msg := &messaging.MulticastMessage{
		Tokens: tokens,
		Notification: &messaging.Notification{
			Title:    notification.Title,
			Body:     notification.Body,
			ImageURL: notification.ImageURL,
		},
		Data: notification.Data,
	}

	// Android config
	if notification.Android != nil {
		msg.Android = &messaging.AndroidConfig{
			Priority: notification.Android.Priority,
			Notification: &messaging.AndroidNotification{
				ClickAction: notification.Android.ClickAction,
				ChannelID:   notification.Android.ChannelID,
				Icon:        notification.Android.Icon,
				Color:       notification.Android.Color,
			},
		}
	} else {
		msg.Android = &messaging.AndroidConfig{
			Priority: "high",
			Notification: &messaging.AndroidNotification{
				ClickAction: "FLUTTER_NOTIFICATION_CLICK",
				ChannelID:   "messages",
			},
		}
	}

	// iOS/APNs config
	if notification.IOS != nil {
		msg.APNS = &messaging.APNSConfig{
			Payload: &messaging.APNSPayload{
				Aps: &messaging.Aps{
					Badge:            notification.IOS.Badge,
					Sound:            notification.IOS.Sound,
					Category:         notification.IOS.Category,
					ThreadID:         notification.IOS.ThreadID,
					ContentAvailable: notification.IOS.ContentAvailable,
					MutableContent:   notification.IOS.MutableContent,
				},
			},
		}
	} else {
		msg.APNS = &messaging.APNSConfig{
			Payload: &messaging.APNSPayload{
				Aps: &messaging.Aps{
					Badge:            notification.Badge,
					Sound:            "default",
					ContentAvailable: true,
					MutableContent:   true,
				},
			},
		}
	}

	return msg
}

func (p *FirebasePushProvider) isInvalidTokenError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return errStr == "registration-token-not-registered" ||
		errStr == "invalid-registration-token" ||
		errStr == "invalid-argument"
}

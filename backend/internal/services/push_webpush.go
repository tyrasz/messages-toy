package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"

	webpush "github.com/SherClockHolmes/webpush-go"
)

// WebPushProvider implements PushProvider using the standard Web Push protocol (VAPID)
// This sends push notifications to browsers without requiring Firebase
type WebPushProvider struct {
	vapidPublicKey  string
	vapidPrivateKey string
	subscriber      string // Usually your email
	mu              sync.RWMutex
	enabled         bool
}

// NewWebPushProvider creates a new Web Push provider
func NewWebPushProvider() *WebPushProvider {
	return &WebPushProvider{}
}

func (p *WebPushProvider) Name() string {
	return "webpush"
}

func (p *WebPushProvider) Initialize(ctx context.Context) error {
	publicKey := os.Getenv("VAPID_PUBLIC_KEY")
	privateKey := os.Getenv("VAPID_PRIVATE_KEY")
	subscriber := os.Getenv("VAPID_SUBSCRIBER")

	if publicKey == "" || privateKey == "" {
		return fmt.Errorf("VAPID_PUBLIC_KEY and VAPID_PRIVATE_KEY not set")
	}

	if subscriber == "" {
		subscriber = "mailto:admin@example.com"
	}

	p.mu.Lock()
	p.vapidPublicKey = publicKey
	p.vapidPrivateKey = privateKey
	p.subscriber = subscriber
	p.enabled = true
	p.mu.Unlock()

	log.Println("Web Push provider initialized")
	return nil
}

// GetVAPIDPublicKey returns the public key for client-side subscription
func (p *WebPushProvider) GetVAPIDPublicKey() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.vapidPublicKey
}

func (p *WebPushProvider) IsEnabled() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.enabled
}

func (p *WebPushProvider) SupportsMulticast() bool {
	return false // Web Push requires individual sends
}

func (p *WebPushProvider) Send(ctx context.Context, tokens []string, notification *Notification) ([]string, error) {
	if !p.IsEnabled() {
		return nil, ErrProviderNotConfigured
	}

	if len(tokens) == 0 {
		return nil, nil
	}

	p.mu.RLock()
	publicKey := p.vapidPublicKey
	privateKey := p.vapidPrivateKey
	subscriber := p.subscriber
	p.mu.RUnlock()

	var failedTokens []string
	var successCount, failureCount int

	for _, subscriptionJSON := range tokens {
		// Web Push tokens are stored as JSON subscription objects
		var subscription webpush.Subscription
		if err := json.Unmarshal([]byte(subscriptionJSON), &subscription); err != nil {
			log.Printf("WebPush: Invalid subscription format: %v", err)
			failedTokens = append(failedTokens, subscriptionJSON)
			failureCount++
			continue
		}

		payload := p.buildPayload(notification)

		resp, err := webpush.SendNotification(payload, &subscription, &webpush.Options{
			Subscriber:      subscriber,
			VAPIDPublicKey:  publicKey,
			VAPIDPrivateKey: privateKey,
			TTL:             86400, // 24 hours
		})

		if err != nil {
			log.Printf("WebPush: Failed to send: %v", err)
			failureCount++
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusOK {
			successCount++
		} else if resp.StatusCode == http.StatusGone || resp.StatusCode == http.StatusNotFound {
			// Subscription is no longer valid
			log.Printf("WebPush: Subscription expired (status %d)", resp.StatusCode)
			failedTokens = append(failedTokens, subscriptionJSON)
			failureCount++
		} else {
			log.Printf("WebPush: Unexpected status %d", resp.StatusCode)
			failureCount++
		}
	}

	log.Printf("WebPush: %d success, %d failures", successCount, failureCount)
	return failedTokens, nil
}

func (p *WebPushProvider) buildPayload(notification *Notification) []byte {
	payload := map[string]interface{}{
		"title": notification.Title,
		"body":  notification.Body,
		"data":  notification.Data,
	}

	if notification.Web != nil {
		if notification.Web.Icon != "" {
			payload["icon"] = notification.Web.Icon
		}
		if notification.Web.Badge != "" {
			payload["badge"] = notification.Web.Badge
		}
		if len(notification.Web.Actions) > 0 {
			payload["actions"] = notification.Web.Actions
		}
		if len(notification.Web.Vibrate) > 0 {
			payload["vibrate"] = notification.Web.Vibrate
		}
	}

	if notification.ImageURL != "" {
		payload["image"] = notification.ImageURL
	}

	data, _ := json.Marshal(payload)
	return data
}

// GenerateVAPIDKeys generates a new VAPID key pair
// Call this once to generate keys, then store them in environment variables
func GenerateVAPIDKeys() (publicKey, privateKey string, err error) {
	privateKey, publicKey, err = webpush.GenerateVAPIDKeys()
	return
}

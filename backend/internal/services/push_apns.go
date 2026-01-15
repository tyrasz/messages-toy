package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/sideshow/apns2"
	"github.com/sideshow/apns2/certificate"
	"github.com/sideshow/apns2/token"
)

// APNsPushProvider implements PushProvider using Apple Push Notification service directly
// Use this when you want to send to iOS devices without Firebase
type APNsPushProvider struct {
	client   *apns2.Client
	bundleID string
	mu       sync.RWMutex
}

// NewAPNsPushProvider creates a new APNs push provider
func NewAPNsPushProvider() *APNsPushProvider {
	return &APNsPushProvider{}
}

func (p *APNsPushProvider) Name() string {
	return "apns"
}

func (p *APNsPushProvider) Initialize(ctx context.Context) error {
	bundleID := os.Getenv("APNS_BUNDLE_ID")
	if bundleID == "" {
		return fmt.Errorf("APNS_BUNDLE_ID not set")
	}
	p.bundleID = bundleID

	// Try token-based auth first (recommended)
	keyPath := os.Getenv("APNS_KEY_PATH")
	keyID := os.Getenv("APNS_KEY_ID")
	teamID := os.Getenv("APNS_TEAM_ID")

	if keyPath != "" && keyID != "" && teamID != "" {
		return p.initializeWithToken(keyPath, keyID, teamID)
	}

	// Fall back to certificate-based auth
	certPath := os.Getenv("APNS_CERT_PATH")
	certPassword := os.Getenv("APNS_CERT_PASSWORD")

	if certPath != "" {
		return p.initializeWithCertificate(certPath, certPassword)
	}

	return fmt.Errorf("APNs credentials not configured - set APNS_KEY_PATH/APNS_KEY_ID/APNS_TEAM_ID or APNS_CERT_PATH")
}

func (p *APNsPushProvider) initializeWithToken(keyPath, keyID, teamID string) error {
	authKey, err := token.AuthKeyFromFile(keyPath)
	if err != nil {
		return fmt.Errorf("failed to load APNs auth key: %w", err)
	}

	authToken := &token.Token{
		AuthKey: authKey,
		KeyID:   keyID,
		TeamID:  teamID,
	}

	// Use production by default, set APNS_DEVELOPMENT=true for sandbox
	client := apns2.NewTokenClient(authToken)
	if os.Getenv("APNS_DEVELOPMENT") == "true" {
		client = client.Development()
	} else {
		client = client.Production()
	}

	p.mu.Lock()
	p.client = client
	p.mu.Unlock()

	log.Println("APNs push provider initialized with token auth")
	return nil
}

func (p *APNsPushProvider) initializeWithCertificate(certPath, password string) error {
	cert, err := certificate.FromP12File(certPath, password)
	if err != nil {
		return fmt.Errorf("failed to load APNs certificate: %w", err)
	}

	// Use production by default
	client := apns2.NewClient(cert)
	if os.Getenv("APNS_DEVELOPMENT") == "true" {
		client = client.Development()
	} else {
		client = client.Production()
	}

	p.mu.Lock()
	p.client = client
	p.mu.Unlock()

	log.Println("APNs push provider initialized with certificate auth")
	return nil
}

func (p *APNsPushProvider) IsEnabled() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.client != nil
}

func (p *APNsPushProvider) SupportsMulticast() bool {
	return false // APNs requires individual sends
}

func (p *APNsPushProvider) Send(ctx context.Context, tokens []string, notification *Notification) ([]string, error) {
	if !p.IsEnabled() {
		return nil, ErrProviderNotConfigured
	}

	if len(tokens) == 0 {
		return nil, nil
	}

	p.mu.RLock()
	client := p.client
	p.mu.RUnlock()

	var failedTokens []string
	var successCount, failureCount int

	for _, deviceToken := range tokens {
		payload := p.buildPayload(notification)

		apnsNotification := &apns2.Notification{
			DeviceToken: deviceToken,
			Topic:       p.bundleID,
			Payload:     payload,
		}

		// Set push type for iOS 13+
		apnsNotification.PushType = apns2.PushTypeAlert

		resp, err := client.Push(apnsNotification)
		if err != nil {
			log.Printf("APNs: Failed to send to device: %v", err)
			failureCount++
			continue
		}

		if resp.Sent() {
			successCount++
		} else {
			log.Printf("APNs: Failed to send: %s - %s", resp.Reason, resp.ApnsID)
			failureCount++

			// Check if token is invalid
			if p.isInvalidTokenReason(resp.Reason) {
				failedTokens = append(failedTokens, deviceToken)
			}
		}
	}

	log.Printf("APNs: %d success, %d failures", successCount, failureCount)
	return failedTokens, nil
}

func (p *APNsPushProvider) buildPayload(notification *Notification) []byte {
	aps := map[string]interface{}{
		"alert": map[string]string{
			"title": notification.Title,
			"body":  notification.Body,
		},
		"sound": "default",
	}

	if notification.IOS != nil {
		if notification.IOS.Sound != "" {
			aps["sound"] = notification.IOS.Sound
		}
		if notification.IOS.Badge != nil {
			aps["badge"] = *notification.IOS.Badge
		}
		if notification.IOS.Category != "" {
			aps["category"] = notification.IOS.Category
		}
		if notification.IOS.ThreadID != "" {
			aps["thread-id"] = notification.IOS.ThreadID
		}
		if notification.IOS.ContentAvailable {
			aps["content-available"] = 1
		}
		if notification.IOS.MutableContent {
			aps["mutable-content"] = 1
		}
	} else if notification.Badge != nil {
		aps["badge"] = *notification.Badge
	}

	payload := map[string]interface{}{
		"aps": aps,
	}

	// Add custom data
	for key, value := range notification.Data {
		payload[key] = value
	}

	data, _ := json.Marshal(payload)
	return data
}

func (p *APNsPushProvider) isInvalidTokenReason(reason string) bool {
	return reason == apns2.ReasonBadDeviceToken ||
		reason == apns2.ReasonUnregistered ||
		reason == apns2.ReasonDeviceTokenNotForTopic
}

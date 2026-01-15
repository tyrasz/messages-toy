package services

import (
	"context"
	"fmt"
)

// PushProvider defines the interface for push notification providers
// Implement this interface to add support for different push services
type PushProvider interface {
	// Name returns the provider name (e.g., "fcm", "apns", "webpush")
	Name() string

	// Initialize sets up the provider with necessary credentials
	Initialize(ctx context.Context) error

	// Send delivers a notification to the specified tokens
	// Returns a list of failed tokens that should be removed
	Send(ctx context.Context, tokens []string, notification *Notification) (failedTokens []string, err error)

	// IsEnabled returns whether this provider is properly configured
	IsEnabled() bool

	// SupportsMulticast returns whether this provider can send to multiple tokens at once
	SupportsMulticast() bool
}

// PushResult contains the result of a push notification send
type PushResult struct {
	SuccessCount int
	FailureCount int
	FailedTokens []string
	Errors       []error
}

// Notification represents a push notification payload
// Provider-agnostic structure that each provider maps to its native format
type Notification struct {
	Title    string
	Body     string
	Data     map[string]string
	Badge    *int
	Sound    string
	ImageURL string
	// Platform-specific overrides (optional)
	Android *AndroidConfig
	IOS     *IOSConfig
	Web     *WebConfig
}

// AndroidConfig contains Android-specific notification options
type AndroidConfig struct {
	ChannelID   string
	Priority    string // "high" or "normal"
	ClickAction string
	Icon        string
	Color       string
}

// IOSConfig contains iOS-specific notification options
type IOSConfig struct {
	Sound            string
	Badge            *int
	Category         string
	ThreadID         string
	ContentAvailable bool
	MutableContent   bool
}

// WebConfig contains web push specific options
type WebConfig struct {
	Icon    string
	Badge   string
	Actions []WebAction
	Vibrate []int
}

// WebAction represents a notification action button
type WebAction struct {
	Action string
	Title  string
	Icon   string
}

// ProviderRegistry holds all registered push providers
type ProviderRegistry struct {
	providers map[string]PushProvider
}

// NewProviderRegistry creates a new provider registry
func NewProviderRegistry() *ProviderRegistry {
	return &ProviderRegistry{
		providers: make(map[string]PushProvider),
	}
}

// Register adds a provider to the registry
func (r *ProviderRegistry) Register(provider PushProvider) {
	r.providers[provider.Name()] = provider
}

// Get returns a provider by name
func (r *ProviderRegistry) Get(name string) (PushProvider, bool) {
	p, ok := r.providers[name]
	return p, ok
}

// GetEnabled returns all enabled providers
func (r *ProviderRegistry) GetEnabled() []PushProvider {
	var enabled []PushProvider
	for _, p := range r.providers {
		if p.IsEnabled() {
			enabled = append(enabled, p)
		}
	}
	return enabled
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
		Sound: "default",
		Android: &AndroidConfig{
			ChannelID:   "messages",
			Priority:    "high",
			ClickAction: "FLUTTER_NOTIFICATION_CLICK",
		},
		IOS: &IOSConfig{
			Sound:            "default",
			ContentAvailable: true,
			MutableContent:   true,
		},
	}
}

// ErrProviderNotConfigured is returned when a provider is not properly configured
var ErrProviderNotConfigured = fmt.Errorf("push provider not configured")

// ErrNoProvidersAvailable is returned when no push providers are enabled
var ErrNoProvidersAvailable = fmt.Errorf("no push providers available")

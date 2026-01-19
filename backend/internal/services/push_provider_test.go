package services

import (
	"context"
	"testing"
)

// MockPushProvider implements PushProvider for testing
type MockPushProvider struct {
	name           string
	enabled        bool
	multicast      bool
	sendError      error
	failedTokens   []string
	sentCount      int
	initializeErr  error
}

func (m *MockPushProvider) Name() string {
	return m.name
}

func (m *MockPushProvider) Initialize(ctx context.Context) error {
	return m.initializeErr
}

func (m *MockPushProvider) Send(ctx context.Context, tokens []string, notification *Notification) ([]string, error) {
	m.sentCount += len(tokens)
	return m.failedTokens, m.sendError
}

func (m *MockPushProvider) IsEnabled() bool {
	return m.enabled
}

func (m *MockPushProvider) SupportsMulticast() bool {
	return m.multicast
}

func TestNewProviderRegistry(t *testing.T) {
	registry := NewProviderRegistry()
	if registry == nil {
		t.Error("NewProviderRegistry should return non-nil registry")
	}
	if registry.providers == nil {
		t.Error("Registry providers map should be initialized")
	}
}

func TestProviderRegistry_Register(t *testing.T) {
	registry := NewProviderRegistry()

	provider := &MockPushProvider{name: "test-provider", enabled: true}
	registry.Register(provider)

	got, ok := registry.Get("test-provider")
	if !ok {
		t.Error("Provider should be registered")
	}
	if got != provider {
		t.Error("Retrieved provider should match registered provider")
	}
}

func TestProviderRegistry_Get(t *testing.T) {
	registry := NewProviderRegistry()

	t.Run("existing provider", func(t *testing.T) {
		provider := &MockPushProvider{name: "fcm", enabled: true}
		registry.Register(provider)

		got, ok := registry.Get("fcm")
		if !ok {
			t.Error("Should find registered provider")
		}
		if got.Name() != "fcm" {
			t.Errorf("Expected name 'fcm', got '%s'", got.Name())
		}
	})

	t.Run("non-existent provider", func(t *testing.T) {
		_, ok := registry.Get("nonexistent")
		if ok {
			t.Error("Should not find non-existent provider")
		}
	})
}

func TestProviderRegistry_GetEnabled(t *testing.T) {
	registry := NewProviderRegistry()

	// Register mix of enabled and disabled providers
	registry.Register(&MockPushProvider{name: "enabled1", enabled: true})
	registry.Register(&MockPushProvider{name: "disabled1", enabled: false})
	registry.Register(&MockPushProvider{name: "enabled2", enabled: true})
	registry.Register(&MockPushProvider{name: "disabled2", enabled: false})

	enabled := registry.GetEnabled()

	if len(enabled) != 2 {
		t.Errorf("Expected 2 enabled providers, got %d", len(enabled))
	}

	// Verify all returned providers are enabled
	for _, p := range enabled {
		if !p.IsEnabled() {
			t.Errorf("Provider %s should be enabled", p.Name())
		}
	}
}

func TestProviderRegistry_GetEnabled_Empty(t *testing.T) {
	registry := NewProviderRegistry()

	// Register only disabled providers
	registry.Register(&MockPushProvider{name: "disabled", enabled: false})

	enabled := registry.GetEnabled()
	if len(enabled) != 0 {
		t.Errorf("Expected 0 enabled providers, got %d", len(enabled))
	}
}

func TestNewMessageNotification(t *testing.T) {
	t.Run("basic message", func(t *testing.T) {
		n := NewMessageNotification("John", "Hello world", "conv-123", false)

		if n.Title != "John" {
			t.Errorf("Expected title 'John', got '%s'", n.Title)
		}
		if n.Body != "Hello world" {
			t.Errorf("Expected body 'Hello world', got '%s'", n.Body)
		}
		if n.Data["type"] != "new_message" {
			t.Errorf("Expected type 'new_message', got '%s'", n.Data["type"])
		}
		if n.Data["conversation_id"] != "conv-123" {
			t.Errorf("Expected conversation_id 'conv-123', got '%s'", n.Data["conversation_id"])
		}
		if _, ok := n.Data["is_group"]; ok {
			t.Error("Should not have is_group for non-group message")
		}
	})

	t.Run("group message", func(t *testing.T) {
		n := NewMessageNotification("Jane", "Group hello", "group-456", true)

		if n.Data["is_group"] != "true" {
			t.Errorf("Expected is_group 'true', got '%s'", n.Data["is_group"])
		}
	})

	t.Run("long content truncated", func(t *testing.T) {
		longContent := "This is a very long message that exceeds one hundred characters and should be truncated to fit within the notification body limit..."
		n := NewMessageNotification("Sender", longContent, "conv-789", false)

		if len(n.Body) > 100 {
			t.Errorf("Body should be truncated to 100 chars, got %d", len(n.Body))
		}
		if n.Body[len(n.Body)-3:] != "..." {
			t.Error("Truncated body should end with '...'")
		}
	})

	t.Run("empty content becomes attachment text", func(t *testing.T) {
		n := NewMessageNotification("Sender", "", "conv-000", false)

		if n.Body != "Sent an attachment" {
			t.Errorf("Expected 'Sent an attachment', got '%s'", n.Body)
		}
	})

	t.Run("android config set", func(t *testing.T) {
		n := NewMessageNotification("Sender", "Message", "conv", false)

		if n.Android == nil {
			t.Fatal("Android config should be set")
		}
		if n.Android.ChannelID != "messages" {
			t.Errorf("Expected channel 'messages', got '%s'", n.Android.ChannelID)
		}
		if n.Android.Priority != "high" {
			t.Errorf("Expected priority 'high', got '%s'", n.Android.Priority)
		}
	})

	t.Run("ios config set", func(t *testing.T) {
		n := NewMessageNotification("Sender", "Message", "conv", false)

		if n.IOS == nil {
			t.Fatal("iOS config should be set")
		}
		if n.IOS.Sound != "default" {
			t.Errorf("Expected sound 'default', got '%s'", n.IOS.Sound)
		}
		if !n.IOS.ContentAvailable {
			t.Error("ContentAvailable should be true")
		}
		if !n.IOS.MutableContent {
			t.Error("MutableContent should be true")
		}
	})
}

func TestNotificationStructs(t *testing.T) {
	t.Run("notification with all fields", func(t *testing.T) {
		badge := 5
		n := &Notification{
			Title:    "Title",
			Body:     "Body",
			Data:     map[string]string{"key": "value"},
			Badge:    &badge,
			Sound:    "custom.wav",
			ImageURL: "https://example.com/image.png",
			Android: &AndroidConfig{
				ChannelID:   "test-channel",
				Priority:    "high",
				ClickAction: "OPEN_APP",
				Icon:        "ic_notification",
				Color:       "#FF0000",
			},
			IOS: &IOSConfig{
				Sound:            "custom.caf",
				Badge:            &badge,
				Category:         "MESSAGE",
				ThreadID:         "thread-1",
				ContentAvailable: true,
				MutableContent:   true,
			},
			Web: &WebConfig{
				Icon:  "/icon.png",
				Badge: "/badge.png",
				Actions: []WebAction{
					{Action: "reply", Title: "Reply", Icon: "/reply.png"},
					{Action: "dismiss", Title: "Dismiss", Icon: "/dismiss.png"},
				},
				Vibrate: []int{100, 50, 100},
			},
		}

		if n.Title != "Title" {
			t.Error("Title not set correctly")
		}
		if *n.Badge != 5 {
			t.Error("Badge not set correctly")
		}
		if len(n.Web.Actions) != 2 {
			t.Error("Web actions not set correctly")
		}
	})
}

func TestErrors(t *testing.T) {
	t.Run("ErrProviderNotConfigured", func(t *testing.T) {
		if ErrProviderNotConfigured == nil {
			t.Error("ErrProviderNotConfigured should not be nil")
		}
		if ErrProviderNotConfigured.Error() != "push provider not configured" {
			t.Errorf("Unexpected error message: %s", ErrProviderNotConfigured.Error())
		}
	})

	t.Run("ErrNoProvidersAvailable", func(t *testing.T) {
		if ErrNoProvidersAvailable == nil {
			t.Error("ErrNoProvidersAvailable should not be nil")
		}
		if ErrNoProvidersAvailable.Error() != "no push providers available" {
			t.Errorf("Unexpected error message: %s", ErrNoProvidersAvailable.Error())
		}
	})
}

func TestMockPushProvider(t *testing.T) {
	t.Run("basic functionality", func(t *testing.T) {
		provider := &MockPushProvider{
			name:      "test",
			enabled:   true,
			multicast: true,
		}

		if provider.Name() != "test" {
			t.Errorf("Expected name 'test', got '%s'", provider.Name())
		}
		if !provider.IsEnabled() {
			t.Error("Provider should be enabled")
		}
		if !provider.SupportsMulticast() {
			t.Error("Provider should support multicast")
		}
	})

	t.Run("send tracks count", func(t *testing.T) {
		provider := &MockPushProvider{
			name:    "test",
			enabled: true,
		}

		tokens := []string{"token1", "token2", "token3"}
		notification := &Notification{Title: "Test", Body: "Body"}

		_, _ = provider.Send(context.Background(), tokens, notification)

		if provider.sentCount != 3 {
			t.Errorf("Expected sent count 3, got %d", provider.sentCount)
		}
	})

	t.Run("send returns failed tokens", func(t *testing.T) {
		provider := &MockPushProvider{
			name:         "test",
			enabled:      true,
			failedTokens: []string{"bad-token"},
		}

		failed, _ := provider.Send(context.Background(), []string{"token1"}, &Notification{})

		if len(failed) != 1 || failed[0] != "bad-token" {
			t.Error("Should return configured failed tokens")
		}
	})
}

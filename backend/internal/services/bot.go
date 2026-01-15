package services

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

// BotService handles AI-powered bot responses
type BotService struct {
	apiKey     string
	apiURL     string
	modelID    string
	httpClient *http.Client
}

// BotResponse contains the bot's reply
type BotResponse struct {
	Content string
	Error   error
}

// NewBotService creates a new bot service
func NewBotService() *BotService {
	return &BotService{
		apiKey:  os.Getenv("ANTHROPIC_API_KEY"),
		apiURL:  "https://api.anthropic.com/v1/messages",
		modelID: "claude-3-haiku-20240307", // Fast, affordable model for chat
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// IsEnabled checks if the bot service is configured
func (s *BotService) IsEnabled() bool {
	return s.apiKey != ""
}

// GenerateResponse generates a bot response to a user message
func (s *BotService) GenerateResponse(userMessage string, conversationHistory []Message) (*BotResponse, error) {
	if !s.IsEnabled() {
		return s.generateFallbackResponse(userMessage), nil
	}

	// Build messages for API
	messages := s.buildMessages(userMessage, conversationHistory)

	// Call Claude API
	response, err := s.callClaudeAPI(messages)
	if err != nil {
		// Fall back to simple responses if API fails
		return s.generateFallbackResponse(userMessage), nil
	}

	return &BotResponse{Content: response}, nil
}

// Message represents a conversation message for the API
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func (s *BotService) buildMessages(userMessage string, history []Message) []Message {
	messages := make([]Message, 0, len(history)+1)

	// Add conversation history (last 10 messages for context)
	start := 0
	if len(history) > 10 {
		start = len(history) - 10
	}
	messages = append(messages, history[start:]...)

	// Add current user message
	messages = append(messages, Message{
		Role:    "user",
		Content: userMessage,
	})

	return messages
}

func (s *BotService) callClaudeAPI(messages []Message) (string, error) {
	requestBody := map[string]interface{}{
		"model":      s.modelID,
		"max_tokens": 1024,
		"system":     "You are a helpful, friendly assistant in a messaging app. Keep responses concise and conversational. Be helpful but brief - this is a chat app, not a document. Use casual language appropriate for texting.",
		"messages":   messages,
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", s.apiURL, strings.NewReader(string(jsonBody)))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", s.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var apiResponse struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}

	if err := json.Unmarshal(body, &apiResponse); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if len(apiResponse.Content) == 0 {
		return "", fmt.Errorf("empty response from API")
	}

	return apiResponse.Content[0].Text, nil
}

// generateFallbackResponse provides simple pattern-based responses when API is unavailable
func (s *BotService) generateFallbackResponse(userMessage string) *BotResponse {
	msg := strings.ToLower(strings.TrimSpace(userMessage))

	// Greeting patterns
	greetings := []string{"hi", "hello", "hey", "howdy", "hola", "greetings"}
	for _, g := range greetings {
		if strings.HasPrefix(msg, g) {
			return &BotResponse{Content: "Hey there! How can I help you today?"}
		}
	}

	// Question patterns
	if strings.HasPrefix(msg, "how are") || strings.HasPrefix(msg, "how're") {
		return &BotResponse{Content: "I'm doing great, thanks for asking! What's on your mind?"}
	}

	if strings.HasPrefix(msg, "what is") || strings.HasPrefix(msg, "what's") {
		return &BotResponse{Content: "That's an interesting question! I'd need more context to give you a proper answer. Could you tell me more?"}
	}

	if strings.HasPrefix(msg, "who is") || strings.HasPrefix(msg, "who's") {
		return &BotResponse{Content: "I'm not sure who you're asking about. Could you give me more details?"}
	}

	if strings.HasPrefix(msg, "why") {
		return &BotResponse{Content: "Good question! The answer often depends on context. What specifically would you like to know?"}
	}

	if strings.HasPrefix(msg, "when") {
		return &BotResponse{Content: "Timing can be tricky! What event or deadline are you asking about?"}
	}

	if strings.HasPrefix(msg, "where") {
		return &BotResponse{Content: "Location, location, location! What place are you trying to find?"}
	}

	if strings.Contains(msg, "help") {
		return &BotResponse{Content: "I'm here to help! Just tell me what you need assistance with."}
	}

	if strings.Contains(msg, "thank") {
		return &BotResponse{Content: "You're welcome! Let me know if there's anything else."}
	}

	if strings.Contains(msg, "bye") || strings.Contains(msg, "goodbye") {
		return &BotResponse{Content: "Goodbye! Feel free to message me anytime."}
	}

	// Weather
	if strings.Contains(msg, "weather") {
		return &BotResponse{Content: "I don't have access to real-time weather data, but you can check your local weather app or website!"}
	}

	// Time
	if strings.Contains(msg, "time") && (strings.Contains(msg, "what") || strings.Contains(msg, "current")) {
		return &BotResponse{Content: fmt.Sprintf("The current server time is %s", time.Now().Format("3:04 PM"))}
	}

	// Date
	if strings.Contains(msg, "date") && strings.Contains(msg, "what") {
		return &BotResponse{Content: fmt.Sprintf("Today is %s", time.Now().Format("Monday, January 2, 2006"))}
	}

	// Jokes
	if strings.Contains(msg, "joke") || strings.Contains(msg, "funny") {
		jokes := []string{
			"Why do programmers prefer dark mode? Because light attracts bugs!",
			"Why did the developer go broke? Because he used up all his cache!",
			"There are only 10 types of people: those who understand binary and those who don't.",
			"A SQL query walks into a bar, walks up to two tables and asks... 'Can I join you?'",
		}
		return &BotResponse{Content: jokes[time.Now().UnixNano()%int64(len(jokes))]}
	}

	// Math - simple calculations
	mathPattern := regexp.MustCompile(`(\d+)\s*([+\-*/])\s*(\d+)`)
	if matches := mathPattern.FindStringSubmatch(msg); len(matches) == 4 {
		var a, b int
		fmt.Sscanf(matches[1], "%d", &a)
		fmt.Sscanf(matches[3], "%d", &b)
		var result int
		switch matches[2] {
		case "+":
			result = a + b
		case "-":
			result = a - b
		case "*":
			result = a * b
		case "/":
			if b != 0 {
				result = a / b
			} else {
				return &BotResponse{Content: "Can't divide by zero!"}
			}
		}
		return &BotResponse{Content: fmt.Sprintf("%d %s %d = %d", a, matches[2], b, result)}
	}

	// Default response
	defaultResponses := []string{
		"Interesting! Tell me more about that.",
		"I see what you mean. What else is on your mind?",
		"That's a thought! Anything else you'd like to discuss?",
		"Got it! Feel free to ask me anything.",
		"I'm listening. What would you like to talk about?",
	}
	return &BotResponse{Content: defaultResponses[time.Now().UnixNano()%int64(len(defaultResponses))]}
}

// BotUserID is the constant ID for the bot user
const BotUserID = "bot-assistant"
const BotUsername = "assistant"
const BotDisplayName = "AI Assistant"

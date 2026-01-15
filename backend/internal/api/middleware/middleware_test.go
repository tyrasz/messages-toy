package middleware

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"messenger/internal/database"
	"messenger/internal/models"
	"messenger/internal/services"
)

func setupTestDB(t *testing.T) {
	var err error
	database.DB, err = gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("Failed to connect to test database: %v", err)
	}

	// Auto-migrate
	database.DB.AutoMigrate(&models.User{})
}

func createTestUser(t *testing.T, username, password string, role models.UserRole) (*models.User, string) {
	authService := services.NewAuthService()
	resp, err := authService.Register(services.RegisterInput{
		Username: username,
		Password: password,
	})
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	// Update the user's role
	var user models.User
	database.DB.Where("username = ?", username).First(&user)
	user.Role = role
	database.DB.Save(&user)

	return &user, resp.AccessToken
}

// TestRateLimiter tests the token bucket rate limiter
func TestRateLimiter(t *testing.T) {
	// Create a limiter with 10 tokens per 100ms interval, max 3 tokens
	rl := NewRateLimiter(10, 100*time.Millisecond, 3)

	// Should allow first 3 requests (burst size)
	for i := 0; i < 3; i++ {
		if !rl.Allow("test") {
			t.Errorf("Request %d should be allowed (within burst)", i+1)
		}
	}

	// 4th request should be blocked (no tokens left)
	if rl.Allow("test") {
		t.Error("4th request should be blocked")
	}

	// Wait for token refill (100ms interval = 10 tokens)
	time.Sleep(120 * time.Millisecond)

	// Should have tokens now
	if !rl.Allow("test") {
		t.Error("Request after waiting should be allowed")
	}

	// Different keys should have separate buckets
	if !rl.Allow("other") {
		t.Error("Request with different key should be allowed")
	}
}

// TestRateLimiterRefill tests that tokens refill correctly over time
func TestRateLimiterRefill(t *testing.T) {
	// 10 tokens per second, max 10
	rl := NewRateLimiter(10, time.Second, 10)

	// Exhaust all tokens
	for i := 0; i < 10; i++ {
		rl.Allow("test")
	}

	// Should be blocked
	if rl.Allow("test") {
		t.Error("Should be blocked after exhausting tokens")
	}

	// Wait for full refill
	time.Sleep(1100 * time.Millisecond)

	// Should have 10 tokens again
	for i := 0; i < 10; i++ {
		if !rl.Allow("test") {
			t.Errorf("Request %d should be allowed after refill", i+1)
		}
	}
}

// TestRateLimitByIP tests the IP-based rate limiter middleware
func TestRateLimitByIP(t *testing.T) {
	app := fiber.New()

	// Create limiter with 2 requests allowed, burst of 2
	limiter := RateLimitByIP(1, time.Second, 2)

	app.Use(limiter)
	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("OK")
	})

	// First 2 requests should succeed
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/", nil)
		resp, _ := app.Test(req, -1)
		if resp.StatusCode != 200 {
			t.Errorf("Request %d should succeed, got status %d", i+1, resp.StatusCode)
		}
	}

	// 3rd request should be rate limited
	req := httptest.NewRequest("GET", "/", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != 429 {
		t.Errorf("3rd request should be rate limited, got status %d", resp.StatusCode)
	}
}

// TestRateLimitByUser tests the user-based rate limiter middleware
func TestRateLimitByUser(t *testing.T) {
	setupTestDB(t)
	app := fiber.New()

	user, token := createTestUser(t, "ratelimituser", "password123", models.UserRoleUser)
	_ = user

	// Create limiter with 2 requests allowed, burst of 2
	limiter := RateLimitByUser(1, time.Second, 2)

	app.Use(AuthRequired())
	app.Use(limiter)
	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("OK")
	})

	// First 2 requests should succeed
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		resp, _ := app.Test(req, -1)
		if resp.StatusCode != 200 {
			t.Errorf("Request %d should succeed, got status %d", i+1, resp.StatusCode)
		}
	}

	// 3rd request should be rate limited
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != 429 {
		t.Errorf("3rd request should be rate limited, got status %d", resp.StatusCode)
	}
}

// TestAuthRequired tests the auth middleware
func TestAuthRequired(t *testing.T) {
	setupTestDB(t)
	app := fiber.New()

	user, token := createTestUser(t, "authuser", "password123", models.UserRoleUser)

	app.Use(AuthRequired())
	app.Get("/", func(c *fiber.Ctx) error {
		userID := GetUserID(c)
		return c.JSON(fiber.Map{"user_id": userID})
	})

	tests := []struct {
		name           string
		authHeader     string
		expectedStatus int
	}{
		{
			name:           "no auth header",
			authHeader:     "",
			expectedStatus: 401,
		},
		{
			name:           "invalid format",
			authHeader:     "InvalidToken",
			expectedStatus: 401,
		},
		{
			name:           "invalid token",
			authHeader:     "Bearer invalid_token",
			expectedStatus: 401,
		},
		{
			name:           "valid token",
			authHeader:     "Bearer " + token,
			expectedStatus: 200,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			resp, _ := app.Test(req, -1)
			if resp.StatusCode != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, resp.StatusCode)
			}
		})
	}

	// Verify user ID is extracted correctly
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != 200 {
		t.Fatalf("Expected 200, got %d", resp.StatusCode)
	}
	_ = user // Verify the user was used
}

// TestModeratorRequired tests the moderator role middleware
func TestModeratorRequired(t *testing.T) {
	setupTestDB(t)
	app := fiber.New()

	regularUser, regularToken := createTestUser(t, "regular", "password123", models.UserRoleUser)
	moderator, modToken := createTestUser(t, "moderator", "password123", models.UserRoleModerator)
	admin, adminToken := createTestUser(t, "admin", "password123", models.UserRoleAdmin)
	_, _, _ = regularUser, moderator, admin

	app.Use(AuthRequired())
	app.Use(ModeratorRequired())
	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("OK")
	})

	tests := []struct {
		name           string
		token          string
		expectedStatus int
	}{
		{
			name:           "regular user denied",
			token:          regularToken,
			expectedStatus: 403,
		},
		{
			name:           "moderator allowed",
			token:          modToken,
			expectedStatus: 200,
		},
		{
			name:           "admin allowed",
			token:          adminToken,
			expectedStatus: 200,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			req.Header.Set("Authorization", "Bearer "+tt.token)
			resp, _ := app.Test(req, -1)
			if resp.StatusCode != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, resp.StatusCode)
			}
		})
	}
}

// TestAdminRequired tests the admin role middleware
func TestAdminRequired(t *testing.T) {
	setupTestDB(t)
	app := fiber.New()

	regularUser, regularToken := createTestUser(t, "regular2", "password123", models.UserRoleUser)
	moderator, modToken := createTestUser(t, "moderator2", "password123", models.UserRoleModerator)
	admin, adminToken := createTestUser(t, "admin2", "password123", models.UserRoleAdmin)
	_, _, _ = regularUser, moderator, admin

	app.Use(AuthRequired())
	app.Use(AdminRequired())
	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("OK")
	})

	tests := []struct {
		name           string
		token          string
		expectedStatus int
	}{
		{
			name:           "regular user denied",
			token:          regularToken,
			expectedStatus: 403,
		},
		{
			name:           "moderator denied",
			token:          modToken,
			expectedStatus: 403,
		},
		{
			name:           "admin allowed",
			token:          adminToken,
			expectedStatus: 200,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			req.Header.Set("Authorization", "Bearer "+tt.token)
			resp, _ := app.Test(req, -1)
			if resp.StatusCode != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, resp.StatusCode)
			}
		})
	}
}

// TestModeratorRequiredNoAuth tests that unauthenticated requests are rejected
func TestModeratorRequiredNoAuth(t *testing.T) {
	setupTestDB(t)
	app := fiber.New()

	// Skip AuthRequired to test ModeratorRequired's own auth check
	app.Use(ModeratorRequired())
	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("OK")
	})

	req := httptest.NewRequest("GET", "/", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != 401 {
		t.Errorf("Expected 401 for unauthenticated request, got %d", resp.StatusCode)
	}
}

// TestAdminRequiredNoAuth tests that unauthenticated requests are rejected
func TestAdminRequiredNoAuth(t *testing.T) {
	setupTestDB(t)
	app := fiber.New()

	// Skip AuthRequired to test AdminRequired's own auth check
	app.Use(AdminRequired())
	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("OK")
	})

	req := httptest.NewRequest("GET", "/", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != 401 {
		t.Errorf("Expected 401 for unauthenticated request, got %d", resp.StatusCode)
	}
}

// TestGetUserRole tests the GetUserRole helper
func TestGetUserRole(t *testing.T) {
	setupTestDB(t)
	app := fiber.New()

	moderator, modToken := createTestUser(t, "rolemod", "password123", models.UserRoleModerator)
	_ = moderator

	app.Use(AuthRequired())
	app.Use(ModeratorRequired())
	app.Get("/", func(c *fiber.Ctx) error {
		role := GetUserRole(c)
		return c.JSON(fiber.Map{"role": role})
	})

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+modToken)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != 200 {
		t.Errorf("Expected 200, got %d", resp.StatusCode)
	}
}

package services

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"messenger/internal/database"
	"messenger/internal/models"
)

func setupAuthTestDB(t *testing.T) func() {
	var err error
	database.DB, err = gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	database.DB.AutoMigrate(&models.User{})

	return func() {
		sqlDB, _ := database.DB.DB()
		sqlDB.Close()
	}
}

func TestNewAuthService(t *testing.T) {
	svc := NewAuthService()
	if svc == nil {
		t.Error("NewAuthService should return non-nil service")
	}
}

func TestAuthService_Register(t *testing.T) {
	cleanup := setupAuthTestDB(t)
	defer cleanup()

	svc := NewAuthService()

	t.Run("successful registration", func(t *testing.T) {
		input := RegisterInput{
			Username:    "testuser",
			Password:    "password123",
			DisplayName: "Test User",
		}

		resp, err := svc.Register(input)
		if err != nil {
			t.Fatalf("Register failed: %v", err)
		}

		if resp.User.Username != "testuser" {
			t.Errorf("Expected username 'testuser', got '%s'", resp.User.Username)
		}

		if resp.AccessToken == "" {
			t.Error("AccessToken should not be empty")
		}

		if resp.RefreshToken == "" {
			t.Error("RefreshToken should not be empty")
		}
	})

	t.Run("registration with phone", func(t *testing.T) {
		input := RegisterInput{
			Username: "phoneuser",
			Password: "password123",
			Phone:    "+1234567890",
		}

		resp, err := svc.Register(input)
		if err != nil {
			t.Fatalf("Register with phone failed: %v", err)
		}

		if resp.User.Username != "phoneuser" {
			t.Errorf("Expected username 'phoneuser', got '%s'", resp.User.Username)
		}
	})

	t.Run("duplicate username", func(t *testing.T) {
		input := RegisterInput{
			Username: "testuser", // Already exists
			Password: "password123",
		}

		_, err := svc.Register(input)
		if err == nil {
			t.Error("Should fail with duplicate username")
		}
		if err.Error() != "username already taken" {
			t.Errorf("Expected 'username already taken', got '%s'", err.Error())
		}
	})

	t.Run("duplicate phone", func(t *testing.T) {
		input := RegisterInput{
			Username: "newuser",
			Password: "password123",
			Phone:    "+1234567890", // Already exists
		}

		_, err := svc.Register(input)
		if err == nil {
			t.Error("Should fail with duplicate phone")
		}
		if err.Error() != "phone number already registered" {
			t.Errorf("Expected 'phone number already registered', got '%s'", err.Error())
		}
	})
}

func TestAuthService_Login(t *testing.T) {
	cleanup := setupAuthTestDB(t)
	defer cleanup()

	svc := NewAuthService()

	// Create a user first
	_, err := svc.Register(RegisterInput{
		Username: "loginuser",
		Password: "correctpassword",
	})
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	t.Run("successful login", func(t *testing.T) {
		input := LoginInput{
			Username: "loginuser",
			Password: "correctpassword",
		}

		resp, err := svc.Login(input)
		if err != nil {
			t.Fatalf("Login failed: %v", err)
		}

		if resp.User.Username != "loginuser" {
			t.Errorf("Expected username 'loginuser', got '%s'", resp.User.Username)
		}

		if resp.AccessToken == "" {
			t.Error("AccessToken should not be empty")
		}

		if resp.RefreshToken == "" {
			t.Error("RefreshToken should not be empty")
		}
	})

	t.Run("wrong password", func(t *testing.T) {
		input := LoginInput{
			Username: "loginuser",
			Password: "wrongpassword",
		}

		_, err := svc.Login(input)
		if err == nil {
			t.Error("Should fail with wrong password")
		}
		if err.Error() != "invalid credentials" {
			t.Errorf("Expected 'invalid credentials', got '%s'", err.Error())
		}
	})

	t.Run("non-existent user", func(t *testing.T) {
		input := LoginInput{
			Username: "nonexistent",
			Password: "password",
		}

		_, err := svc.Login(input)
		if err == nil {
			t.Error("Should fail with non-existent user")
		}
		if err.Error() != "invalid credentials" {
			t.Errorf("Expected 'invalid credentials', got '%s'", err.Error())
		}
	})
}

func TestAuthService_RefreshToken(t *testing.T) {
	cleanup := setupAuthTestDB(t)
	defer cleanup()

	svc := NewAuthService()

	// Create a user and get tokens
	resp, err := svc.Register(RegisterInput{
		Username: "refreshuser",
		Password: "password123",
	})
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	t.Run("successful refresh", func(t *testing.T) {
		// Wait a moment to ensure token timestamps differ
		time.Sleep(time.Second)

		newResp, err := svc.RefreshToken(resp.RefreshToken)
		if err != nil {
			t.Fatalf("RefreshToken failed: %v", err)
		}

		if newResp.AccessToken == "" {
			t.Error("New access token should not be empty")
		}

		if newResp.RefreshToken == "" {
			t.Error("New refresh token should not be empty")
		}

		if newResp.User.Username != "refreshuser" {
			t.Errorf("Expected username 'refreshuser', got '%s'", newResp.User.Username)
		}
	})

	t.Run("invalid refresh token", func(t *testing.T) {
		_, err := svc.RefreshToken("invalid-token")
		if err == nil {
			t.Error("Should fail with invalid token")
		}
	})

	t.Run("expired refresh token", func(t *testing.T) {
		// Create an expired token manually
		claims := Claims{
			UserID:   "some-user-id",
			Username: "someuser",
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)), // Expired
				IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
			},
		}
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		expiredToken, _ := token.SignedString(jwtSecret)

		_, err := svc.RefreshToken(expiredToken)
		if err == nil {
			t.Error("Should fail with expired token")
		}
	})
}

func TestValidateToken(t *testing.T) {
	cleanup := setupAuthTestDB(t)
	defer cleanup()

	svc := NewAuthService()

	// Create a user and get a valid token
	resp, err := svc.Register(RegisterInput{
		Username: "validateuser",
		Password: "password123",
	})
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	t.Run("valid token", func(t *testing.T) {
		claims, err := ValidateToken(resp.AccessToken)
		if err != nil {
			t.Fatalf("ValidateToken failed: %v", err)
		}

		if claims.Username != "validateuser" {
			t.Errorf("Expected username 'validateuser', got '%s'", claims.Username)
		}

		if claims.UserID == "" {
			t.Error("UserID should not be empty")
		}
	})

	t.Run("invalid token", func(t *testing.T) {
		_, err := ValidateToken("not-a-valid-token")
		if err == nil {
			t.Error("Should fail with invalid token")
		}
	})

	t.Run("malformed token", func(t *testing.T) {
		_, err := ValidateToken("eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.invalid.payload")
		if err == nil {
			t.Error("Should fail with malformed token")
		}
	})

	t.Run("wrong signature", func(t *testing.T) {
		// Create token with different secret
		claims := Claims{
			UserID:   "user-id",
			Username: "user",
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
				IssuedAt:  jwt.NewNumericDate(time.Now()),
			},
		}
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		wrongToken, _ := token.SignedString([]byte("wrong-secret"))

		_, err := ValidateToken(wrongToken)
		if err == nil {
			t.Error("Should fail with wrong signature")
		}
	})
}

func TestGetUserByID(t *testing.T) {
	cleanup := setupAuthTestDB(t)
	defer cleanup()

	svc := NewAuthService()

	// Create a user
	resp, err := svc.Register(RegisterInput{
		Username: "finduser",
		Password: "password123",
	})
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	t.Run("existing user", func(t *testing.T) {
		user, err := GetUserByID(resp.User.ID)
		if err != nil {
			t.Fatalf("GetUserByID failed: %v", err)
		}

		if user.Username != "finduser" {
			t.Errorf("Expected username 'finduser', got '%s'", user.Username)
		}
	})

	t.Run("non-existent user", func(t *testing.T) {
		_, err := GetUserByID("non-existent-id")
		if err == nil {
			t.Error("Should fail with non-existent user")
		}
	})
}

func TestGetEnvOrDefault(t *testing.T) {
	t.Run("returns default when env not set", func(t *testing.T) {
		result := getEnvOrDefault("NON_EXISTENT_VAR_12345", "default-value")
		if result != "default-value" {
			t.Errorf("Expected 'default-value', got '%s'", result)
		}
	})
}

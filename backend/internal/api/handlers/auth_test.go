package handlers

import (
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestAuthHandler_Register(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	app := fiber.New()
	authHandler := NewAuthHandler()

	app.Post("/auth/register", authHandler.Register)

	tests := []struct {
		name           string
		body           map[string]interface{}
		expectedStatus int
		checkResponse  func(t *testing.T, data map[string]interface{})
	}{
		{
			name: "successful registration",
			body: map[string]interface{}{
				"username": "testuser",
				"password": "password123",
			},
			expectedStatus: http.StatusCreated,
			checkResponse: func(t *testing.T, data map[string]interface{}) {
				assertJSONFieldExists(t, data, "access_token")
				assertJSONFieldExists(t, data, "refresh_token")
				assertJSONFieldExists(t, data, "user")

				user := data["user"].(map[string]interface{})
				assertJSONField(t, user, "username", "testuser")
			},
		},
		{
			name: "registration with display name",
			body: map[string]interface{}{
				"username":     "userwithdisplay",
				"password":     "password123",
				"display_name": "Test User",
			},
			expectedStatus: http.StatusCreated,
			checkResponse: func(t *testing.T, data map[string]interface{}) {
				user := data["user"].(map[string]interface{})
				assertJSONField(t, user, "display_name", "Test User")
			},
		},
		{
			name: "missing username",
			body: map[string]interface{}{
				"password": "password123",
			},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, data map[string]interface{}) {
				assertJSONFieldExists(t, data, "error")
			},
		},
		{
			name: "missing password",
			body: map[string]interface{}{
				"username": "nopassword",
			},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, data map[string]interface{}) {
				assertJSONFieldExists(t, data, "error")
			},
		},
		{
			name: "password too short",
			body: map[string]interface{}{
				"username": "shortpass",
				"password": "12345",
			},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, data map[string]interface{}) {
				assertJSONFieldExists(t, data, "error")
			},
		},
		{
			name:           "empty body",
			body:           map[string]interface{}{},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, data map[string]interface{}) {
				assertJSONFieldExists(t, data, "error")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, body := makeRequest(app, testRequest{
				Method: "POST",
				Path:   "/auth/register",
				Body:   tt.body,
			})

			assertStatus(t, resp, tt.expectedStatus)

			if tt.checkResponse != nil {
				data := parseResponse(body)
				tt.checkResponse(t, data)
			}
		})
	}
}

func TestAuthHandler_Register_DuplicateUsername(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	app := fiber.New()
	authHandler := NewAuthHandler()

	app.Post("/auth/register", authHandler.Register)

	// First registration should succeed
	resp1, _ := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/auth/register",
		Body: map[string]interface{}{
			"username": "duplicateuser",
			"password": "password123",
		},
	})
	assertStatus(t, resp1, http.StatusCreated)

	// Second registration with same username should fail
	resp2, body2 := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/auth/register",
		Body: map[string]interface{}{
			"username": "duplicateuser",
			"password": "password123",
		},
	})
	assertStatus(t, resp2, http.StatusConflict)

	data := parseResponse(body2)
	assertJSONFieldExists(t, data, "error")
}

func TestAuthHandler_Login(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	app := fiber.New()
	authHandler := NewAuthHandler()

	app.Post("/auth/register", authHandler.Register)
	app.Post("/auth/login", authHandler.Login)

	// First create a user
	_, _ = makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/auth/register",
		Body: map[string]interface{}{
			"username": "loginuser",
			"password": "password123",
		},
	})

	tests := []struct {
		name           string
		body           map[string]interface{}
		expectedStatus int
		checkResponse  func(t *testing.T, data map[string]interface{})
	}{
		{
			name: "successful login",
			body: map[string]interface{}{
				"username": "loginuser",
				"password": "password123",
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, data map[string]interface{}) {
				assertJSONFieldExists(t, data, "access_token")
				assertJSONFieldExists(t, data, "refresh_token")
				assertJSONFieldExists(t, data, "user")
			},
		},
		{
			name: "wrong password",
			body: map[string]interface{}{
				"username": "loginuser",
				"password": "wrongpassword",
			},
			expectedStatus: http.StatusUnauthorized,
			checkResponse: func(t *testing.T, data map[string]interface{}) {
				assertJSONFieldExists(t, data, "error")
			},
		},
		{
			name: "non-existent user",
			body: map[string]interface{}{
				"username": "nonexistent",
				"password": "password123",
			},
			expectedStatus: http.StatusUnauthorized,
			checkResponse: func(t *testing.T, data map[string]interface{}) {
				assertJSONFieldExists(t, data, "error")
			},
		},
		{
			name: "missing credentials",
			body: map[string]interface{}{},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, data map[string]interface{}) {
				assertJSONFieldExists(t, data, "error")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, body := makeRequest(app, testRequest{
				Method: "POST",
				Path:   "/auth/login",
				Body:   tt.body,
			})

			assertStatus(t, resp, tt.expectedStatus)

			if tt.checkResponse != nil {
				data := parseResponse(body)
				tt.checkResponse(t, data)
			}
		})
	}
}

func TestAuthHandler_Refresh(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	app := fiber.New()
	authHandler := NewAuthHandler()

	app.Post("/auth/register", authHandler.Register)
	app.Post("/auth/refresh", authHandler.Refresh)

	// Create a user and get tokens
	_, body := makeRequest(app, testRequest{
		Method: "POST",
		Path:   "/auth/register",
		Body: map[string]interface{}{
			"username": "refreshuser",
			"password": "password123",
		},
	})

	data := parseResponse(body)
	refreshToken := data["refresh_token"].(string)

	tests := []struct {
		name           string
		body           map[string]interface{}
		expectedStatus int
		checkResponse  func(t *testing.T, data map[string]interface{})
	}{
		{
			name: "successful refresh",
			body: map[string]interface{}{
				"refresh_token": refreshToken,
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, data map[string]interface{}) {
				assertJSONFieldExists(t, data, "access_token")
				assertJSONFieldExists(t, data, "refresh_token")
			},
		},
		{
			name: "invalid refresh token",
			body: map[string]interface{}{
				"refresh_token": "invalid-token",
			},
			expectedStatus: http.StatusUnauthorized,
			checkResponse: func(t *testing.T, data map[string]interface{}) {
				assertJSONFieldExists(t, data, "error")
			},
		},
		{
			name: "missing refresh token",
			body: map[string]interface{}{},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, data map[string]interface{}) {
				assertJSONFieldExists(t, data, "error")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, body := makeRequest(app, testRequest{
				Method: "POST",
				Path:   "/auth/refresh",
				Body:   tt.body,
			})

			assertStatus(t, resp, tt.expectedStatus)

			if tt.checkResponse != nil {
				data := parseResponse(body)
				tt.checkResponse(t, data)
			}
		})
	}
}

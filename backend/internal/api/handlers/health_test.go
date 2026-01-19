package handlers

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"messenger/internal/database"
	ws "messenger/internal/websocket"
)

func setupHealthTestDB(t *testing.T) func() {
	var err error
	database.DB, err = gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	return func() {
		sqlDB, _ := database.DB.DB()
		sqlDB.Close()
	}
}

func TestHealthHandler_Health(t *testing.T) {
	cleanup := setupHealthTestDB(t)
	defer cleanup()

	hub := ws.NewHub()
	handler := NewHealthHandler(hub)

	app := fiber.New()
	app.Get("/health", handler.Health)

	req := httptest.NewRequest("GET", "/health", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if result["status"] != "healthy" {
		t.Errorf("Expected status 'healthy', got %v", result["status"])
	}

	if result["timestamp"] == nil {
		t.Error("Expected timestamp in response")
	}

	if result["uptime"] == nil {
		t.Error("Expected uptime in response")
	}

	checks, ok := result["checks"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected checks in response")
	}

	if checks["database"] != "up" {
		t.Errorf("Expected database to be 'up', got %v", checks["database"])
	}
}

func TestHealthHandler_Liveness(t *testing.T) {
	hub := ws.NewHub()
	handler := NewHealthHandler(hub)

	app := fiber.New()
	app.Get("/healthz", handler.Liveness)

	req := httptest.NewRequest("GET", "/healthz", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if result["status"] != "ok" {
		t.Errorf("Expected status 'ok', got %v", result["status"])
	}
}

func TestHealthHandler_Readiness(t *testing.T) {
	cleanup := setupHealthTestDB(t)
	defer cleanup()

	hub := ws.NewHub()
	handler := NewHealthHandler(hub)

	app := fiber.New()
	app.Get("/readyz", handler.Readiness)

	req := httptest.NewRequest("GET", "/readyz", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if result["status"] != "ready" {
		t.Errorf("Expected status 'ready', got %v", result["status"])
	}
}

func TestHealthHandler_Metrics(t *testing.T) {
	cleanup := setupHealthTestDB(t)
	defer cleanup()

	hub := ws.NewHub()
	handler := NewHealthHandler(hub)

	app := fiber.New()
	app.Get("/metrics", handler.Metrics)

	req := httptest.NewRequest("GET", "/metrics", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Check required fields
	requiredFields := []string{"timestamp", "uptime", "uptime_seconds", "runtime", "memory", "connections", "counters"}
	for _, field := range requiredFields {
		if result[field] == nil {
			t.Errorf("Expected field '%s' in response", field)
		}
	}

	// Check runtime info
	runtime, ok := result["runtime"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected runtime in response")
	}
	if runtime["go_version"] == nil {
		t.Error("Expected go_version in runtime")
	}
	if runtime["num_goroutine"] == nil {
		t.Error("Expected num_goroutine in runtime")
	}

	// Check memory info
	memory, ok := result["memory"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected memory in response")
	}
	if memory["alloc_mb"] == nil {
		t.Error("Expected alloc_mb in memory")
	}

	// Check connections
	connections, ok := result["connections"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected connections in response")
	}
	if connections["websocket_active"] == nil {
		t.Error("Expected websocket_active in connections")
	}
}

func TestHealthHandler_PrometheusMetrics(t *testing.T) {
	cleanup := setupHealthTestDB(t)
	defer cleanup()

	hub := ws.NewHub()
	handler := NewHealthHandler(hub)

	app := fiber.New()
	app.Get("/metrics/prometheus", handler.PrometheusMetrics)

	req := httptest.NewRequest("GET", "/metrics/prometheus", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Check content type
	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "text/plain") {
		t.Errorf("Expected text/plain content type, got %s", contentType)
	}

	// Read body
	buf := make([]byte, 4096)
	n, _ := resp.Body.Read(buf)
	body := string(buf[:n])

	// Check for expected metrics
	expectedMetrics := []string{
		"messenger_uptime_seconds",
		"messenger_goroutines",
		"messenger_memory_alloc_bytes",
		"messenger_websocket_connections",
		"messenger_messages_received_total",
	}

	for _, metric := range expectedMetrics {
		if !strings.Contains(body, metric) {
			t.Errorf("Expected metric '%s' in Prometheus output", metric)
		}
	}
}

func TestMetrics_Counters(t *testing.T) {
	// Reset counters
	AppMetrics.MessagesReceived.Store(0)
	AppMetrics.MessagesSent.Store(0)
	AppMetrics.APIRequests.Store(0)
	AppMetrics.Errors.Store(0)

	// Increment counters
	AppMetrics.MessagesReceived.Add(10)
	AppMetrics.MessagesSent.Add(5)
	AppMetrics.APIRequests.Add(100)
	AppMetrics.Errors.Add(2)

	// Verify values
	if AppMetrics.MessagesReceived.Load() != 10 {
		t.Errorf("Expected MessagesReceived 10, got %d", AppMetrics.MessagesReceived.Load())
	}
	if AppMetrics.MessagesSent.Load() != 5 {
		t.Errorf("Expected MessagesSent 5, got %d", AppMetrics.MessagesSent.Load())
	}
	if AppMetrics.APIRequests.Load() != 100 {
		t.Errorf("Expected APIRequests 100, got %d", AppMetrics.APIRequests.Load())
	}
	if AppMetrics.Errors.Load() != 2 {
		t.Errorf("Expected Errors 2, got %d", AppMetrics.Errors.Load())
	}
}

package handlers

import (
	"fmt"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/gofiber/fiber/v2"
	"messenger/internal/database"
	"messenger/internal/services"
	ws "messenger/internal/websocket"
)

// Metrics tracks application metrics
type Metrics struct {
	MessagesReceived atomic.Uint64
	MessagesSent     atomic.Uint64
	WebSocketConns   atomic.Int64
	APIRequests      atomic.Uint64
	Errors           atomic.Uint64
}

var AppMetrics = &Metrics{}

type HealthHandler struct {
	hub       *ws.Hub
	startTime time.Time
}

func NewHealthHandler(hub *ws.Hub) *HealthHandler {
	return &HealthHandler{
		hub:       hub,
		startTime: time.Now(),
	}
}

// Health returns detailed health status
func (h *HealthHandler) Health(c *fiber.Ctx) error {
	status := "healthy"
	checks := make(map[string]interface{})

	// Check database connection
	dbStatus := "up"
	if database.DB != nil {
		sqlDB, err := database.DB.DB()
		if err != nil {
			dbStatus = "error: " + err.Error()
			status = "degraded"
		} else if err := sqlDB.Ping(); err != nil {
			dbStatus = "error: " + err.Error()
			status = "degraded"
		}
	} else {
		dbStatus = "not configured"
		status = "degraded"
	}
	checks["database"] = dbStatus

	// Check push notifications
	pushStatus := "disabled"
	pushSvc := services.GetPushService()
	if pushSvc.IsEnabled() {
		pushStatus = "enabled"
	}
	checks["push_notifications"] = pushStatus

	// Check WebSocket hub
	wsStatus := "running"
	if h.hub == nil {
		wsStatus = "not initialized"
		status = "degraded"
	}
	checks["websocket"] = wsStatus

	return c.JSON(fiber.Map{
		"status":    status,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"uptime":    time.Since(h.startTime).String(),
		"checks":    checks,
	})
}

// Liveness returns a simple liveness check for Kubernetes
func (h *HealthHandler) Liveness(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"status": "ok",
	})
}

// Readiness returns readiness status for Kubernetes
func (h *HealthHandler) Readiness(c *fiber.Ctx) error {
	// Check database connection
	if database.DB != nil {
		sqlDB, err := database.DB.DB()
		if err != nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
				"status": "not ready",
				"reason": "database error",
			})
		}
		if err := sqlDB.Ping(); err != nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
				"status": "not ready",
				"reason": "database unreachable",
			})
		}
	}

	return c.JSON(fiber.Map{
		"status": "ready",
	})
}

// Metrics returns application metrics
func (h *HealthHandler) Metrics(c *fiber.Ctx) error {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	activeConnections := int64(0)
	if h.hub != nil {
		activeConnections = h.hub.GetActiveConnectionCount()
	}

	return c.JSON(fiber.Map{
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"uptime":    time.Since(h.startTime).String(),
		"uptime_seconds": time.Since(h.startTime).Seconds(),
		"runtime": fiber.Map{
			"go_version":    runtime.Version(),
			"num_goroutine": runtime.NumGoroutine(),
			"num_cpu":       runtime.NumCPU(),
			"gomaxprocs":    runtime.GOMAXPROCS(0),
		},
		"memory": fiber.Map{
			"alloc_mb":       float64(memStats.Alloc) / 1024 / 1024,
			"total_alloc_mb": float64(memStats.TotalAlloc) / 1024 / 1024,
			"sys_mb":         float64(memStats.Sys) / 1024 / 1024,
			"num_gc":         memStats.NumGC,
		},
		"connections": fiber.Map{
			"websocket_active": activeConnections,
		},
		"counters": fiber.Map{
			"messages_received": AppMetrics.MessagesReceived.Load(),
			"messages_sent":     AppMetrics.MessagesSent.Load(),
			"api_requests":      AppMetrics.APIRequests.Load(),
			"errors":            AppMetrics.Errors.Load(),
		},
	})
}

// PrometheusMetrics returns metrics in Prometheus format
func (h *HealthHandler) PrometheusMetrics(c *fiber.Ctx) error {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	activeConnections := int64(0)
	if h.hub != nil {
		activeConnections = h.hub.GetActiveConnectionCount()
	}

	c.Set("Content-Type", "text/plain; charset=utf-8")

	metrics := ""
	metrics += "# HELP messenger_uptime_seconds Time since application start\n"
	metrics += "# TYPE messenger_uptime_seconds gauge\n"
	metrics += "messenger_uptime_seconds " + formatFloat(time.Since(h.startTime).Seconds()) + "\n\n"

	metrics += "# HELP messenger_goroutines Number of goroutines\n"
	metrics += "# TYPE messenger_goroutines gauge\n"
	metrics += "messenger_goroutines " + formatInt(int64(runtime.NumGoroutine())) + "\n\n"

	metrics += "# HELP messenger_memory_alloc_bytes Allocated memory in bytes\n"
	metrics += "# TYPE messenger_memory_alloc_bytes gauge\n"
	metrics += "messenger_memory_alloc_bytes " + formatInt(int64(memStats.Alloc)) + "\n\n"

	metrics += "# HELP messenger_memory_sys_bytes System memory in bytes\n"
	metrics += "# TYPE messenger_memory_sys_bytes gauge\n"
	metrics += "messenger_memory_sys_bytes " + formatInt(int64(memStats.Sys)) + "\n\n"

	metrics += "# HELP messenger_gc_count Total number of GC cycles\n"
	metrics += "# TYPE messenger_gc_count counter\n"
	metrics += "messenger_gc_count " + formatInt(int64(memStats.NumGC)) + "\n\n"

	metrics += "# HELP messenger_websocket_connections Active WebSocket connections\n"
	metrics += "# TYPE messenger_websocket_connections gauge\n"
	metrics += "messenger_websocket_connections " + formatInt(activeConnections) + "\n\n"

	metrics += "# HELP messenger_messages_received_total Total messages received\n"
	metrics += "# TYPE messenger_messages_received_total counter\n"
	metrics += "messenger_messages_received_total " + formatUint(AppMetrics.MessagesReceived.Load()) + "\n\n"

	metrics += "# HELP messenger_messages_sent_total Total messages sent\n"
	metrics += "# TYPE messenger_messages_sent_total counter\n"
	metrics += "messenger_messages_sent_total " + formatUint(AppMetrics.MessagesSent.Load()) + "\n\n"

	metrics += "# HELP messenger_api_requests_total Total API requests\n"
	metrics += "# TYPE messenger_api_requests_total counter\n"
	metrics += "messenger_api_requests_total " + formatUint(AppMetrics.APIRequests.Load()) + "\n\n"

	metrics += "# HELP messenger_errors_total Total errors\n"
	metrics += "# TYPE messenger_errors_total counter\n"
	metrics += "messenger_errors_total " + formatUint(AppMetrics.Errors.Load()) + "\n"

	return c.SendString(metrics)
}

func formatFloat(f float64) string {
	return fmt.Sprintf("%.6f", f)
}

func formatInt(i int64) string {
	return fmt.Sprintf("%d", i)
}

func formatUint(u uint64) string {
	return fmt.Sprintf("%d", u)
}

package middleware

import (
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
)

// RateLimiter implements a token bucket rate limiter
type RateLimiter struct {
	mu       sync.Mutex
	buckets  map[string]*bucket
	rate     int           // tokens per interval
	interval time.Duration // refill interval
	burst    int           // max tokens (bucket size)
}

type bucket struct {
	tokens     int
	lastRefill time.Time
}

// NewRateLimiter creates a new rate limiter
// rate: number of requests allowed per interval
// interval: time period for rate limit
// burst: maximum burst size (bucket capacity)
func NewRateLimiter(rate int, interval time.Duration, burst int) *RateLimiter {
	rl := &RateLimiter{
		buckets:  make(map[string]*bucket),
		rate:     rate,
		interval: interval,
		burst:    burst,
	}

	// Clean up old buckets periodically
	go rl.cleanup()

	return rl
}

func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	for range ticker.C {
		rl.mu.Lock()
		threshold := time.Now().Add(-10 * time.Minute)
		for key, b := range rl.buckets {
			if b.lastRefill.Before(threshold) {
				delete(rl.buckets, key)
			}
		}
		rl.mu.Unlock()
	}
}

func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	b, exists := rl.buckets[key]
	if !exists {
		b = &bucket{
			tokens:     rl.burst,
			lastRefill: time.Now(),
		}
		rl.buckets[key] = b
	}

	// Refill tokens based on elapsed time
	now := time.Now()
	elapsed := now.Sub(b.lastRefill)
	tokensToAdd := int(elapsed / rl.interval) * rl.rate

	if tokensToAdd > 0 {
		b.tokens = min(rl.burst, b.tokens+tokensToAdd)
		b.lastRefill = now
	}

	// Check if request is allowed
	if b.tokens > 0 {
		b.tokens--
		return true
	}

	return false
}

// RateLimitByIP creates middleware that rate limits by IP address
func RateLimitByIP(rate int, interval time.Duration, burst int) fiber.Handler {
	limiter := NewRateLimiter(rate, interval, burst)

	return func(c *fiber.Ctx) error {
		key := c.IP()

		if !limiter.Allow(key) {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error": "Rate limit exceeded. Please try again later.",
			})
		}

		return c.Next()
	}
}

// RateLimitByUser creates middleware that rate limits by user ID
func RateLimitByUser(rate int, interval time.Duration, burst int) fiber.Handler {
	limiter := NewRateLimiter(rate, interval, burst)

	return func(c *fiber.Ctx) error {
		userID := GetUserID(c)
		if userID == "" {
			// Fall back to IP if not authenticated
			userID = "ip:" + c.IP()
		}

		if !limiter.Allow(userID) {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error": "Rate limit exceeded. Please try again later.",
			})
		}

		return c.Next()
	}
}

// Preset rate limiters for different endpoints
var (
	// AuthLimiter: 5 attempts per minute (login/register)
	AuthLimiter = RateLimitByIP(5, time.Minute, 10)

	// APILimiter: 100 requests per minute for general API
	APILimiter = RateLimitByUser(100, time.Minute, 150)

	// MediaLimiter: 10 uploads per minute
	MediaLimiter = RateLimitByUser(10, time.Minute, 15)

	// MessageLimiter: 60 messages per minute
	MessageLimiter = RateLimitByUser(60, time.Minute, 100)
)

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

package middleware

import (
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// RateLimiter implements a simple in-memory rate limiter
type RateLimiter struct {
	requests map[string][]time.Time
	mu       sync.RWMutex
	window   time.Duration
	maxReqs  int
	logger   *slog.Logger
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(maxReqs int, window time.Duration, logger *slog.Logger) *RateLimiter {
	rl := &RateLimiter{
		requests: make(map[string][]time.Time),
		window:   window,
		maxReqs:  maxReqs,
		logger:   logger,
	}

	// Start cleanup goroutine
	go rl.cleanup()

	return rl
}

// Allow checks if a request is allowed for the given identifier
func (rl *RateLimiter) Allow(identifier string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	// Get existing requests for this identifier
	requests, exists := rl.requests[identifier]
	if !exists {
		requests = []time.Time{}
	}

	// Remove old requests
	validRequests := []time.Time{}
	for _, req := range requests {
		if req.After(cutoff) {
			validRequests = append(validRequests, req)
		}
	}

	// Check if we're under the limit
	if len(validRequests) >= rl.maxReqs {
		return false
	}

	// Add current request
	validRequests = append(validRequests, now)
	rl.requests[identifier] = validRequests

	return true
}

// cleanup removes expired entries from the rate limiter
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(rl.window)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		cutoff := now.Add(-rl.window)

		for identifier, requests := range rl.requests {
			validRequests := []time.Time{}
			for _, req := range requests {
				if req.After(cutoff) {
					validRequests = append(validRequests, req)
				}
			}

			if len(validRequests) == 0 {
				delete(rl.requests, identifier)
			} else {
				rl.requests[identifier] = validRequests
			}
		}
		rl.mu.Unlock()
	}
}

// RateLimit middleware that limits requests per IP address
func RateLimit(maxReqs int, window time.Duration, logger *slog.Logger) gin.HandlerFunc {
	limiter := NewRateLimiter(maxReqs, window, logger)

	return func(c *gin.Context) {
		// Use IP address as identifier
		identifier := c.ClientIP()

		if !limiter.Allow(identifier) {
			logger.Warn("rate limit exceeded",
				"ip", identifier,
				"max_requests", maxReqs,
				"window", window,
			)

			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":       "Rate limit exceeded",
				"retry_after": int(window.Seconds()),
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// AuthRateLimit creates rate limiting middleware specifically for auth endpoints
func AuthRateLimit(logger *slog.Logger) gin.HandlerFunc {
	// 10 requests per hour for auth endpoints
	return RateLimit(10, time.Hour, logger)
}

// RegisterRateLimit creates rate limiting middleware for registration endpoint
func RegisterRateLimit(logger *slog.Logger) gin.HandlerFunc {
	// 5 registrations per hour
	return RateLimit(5, time.Hour, logger)
}

// RegisterRateLimitForTest creates permissive rate limiting middleware for registration endpoint in test mode
func RegisterRateLimitForTest(logger *slog.Logger) gin.HandlerFunc {
	// 1000 registrations per hour for testing
	return RateLimit(1000, time.Hour, logger)
}

// AuthRateLimitForTest creates permissive rate limiting middleware for auth endpoints in test mode
func AuthRateLimitForTest(logger *slog.Logger) gin.HandlerFunc {
	// 1000 requests per hour for testing
	return RateLimit(1000, time.Hour, logger)
}

// RefreshRateLimit creates rate limiting middleware for token refresh endpoint
func RefreshRateLimit(logger *slog.Logger) gin.HandlerFunc {
	// 100 refresh requests per hour
	return RateLimit(100, time.Hour, logger)
}

// RateLimitByUserID middleware that limits requests per authenticated user
func RateLimitByUserID(maxReqs int, window time.Duration, logger *slog.Logger) gin.HandlerFunc {
	limiter := NewRateLimiter(maxReqs, window, logger)

	return func(c *gin.Context) {
		// Get user ID from context (requires auth middleware to run first)
		userID, exists := c.Get("user_id")
		if !exists {
			// If no user ID, fall back to IP-based limiting
			identifier := c.ClientIP()
			if !limiter.Allow(identifier) {
				logger.Warn("rate limit exceeded for unauthenticated user",
					"ip", identifier,
					"max_requests", maxReqs,
					"window", window,
				)

				c.JSON(http.StatusTooManyRequests, gin.H{
					"error":       "Rate limit exceeded",
					"retry_after": int(window.Seconds()),
				})
				c.Abort()
				return
			}
		} else {
			// userID is uuid.UUID type, convert to string properly
			identifier := userID.(uuid.UUID).String()
			if !limiter.Allow(identifier) {
				logger.Warn("rate limit exceeded for authenticated user",
					"user_id", identifier,
					"max_requests", maxReqs,
					"window", window,
				)

				c.JSON(http.StatusTooManyRequests, gin.H{
					"error":       "Rate limit exceeded",
					"retry_after": int(window.Seconds()),
				})
				c.Abort()
				return
			}
		}

		c.Next()
	}
}

// TaskRateLimit creates rate limiting middleware for task endpoints
func TaskRateLimit(logger *slog.Logger) gin.HandlerFunc {
	// 100 task operations per hour per user
	return RateLimitByUserID(100, time.Hour, logger)
}

// TaskExecutionRateLimit creates rate limiting middleware for execution endpoints
func TaskExecutionRateLimit(logger *slog.Logger) gin.HandlerFunc {
	// 50 execution operations per hour per user
	return RateLimitByUserID(50, time.Hour, logger)
}

// TaskCreationRateLimit creates rate limiting middleware specifically for task creation
func TaskCreationRateLimit(logger *slog.Logger) gin.HandlerFunc {
	// 20 task creations per hour per user (more restrictive)
	return RateLimitByUserID(20, time.Hour, logger)
}

// ExecutionCreationRateLimit creates rate limiting middleware for execution creation
func ExecutionCreationRateLimit(logger *slog.Logger) gin.HandlerFunc {
	// 30 execution starts per hour per user
	return RateLimitByUserID(30, time.Hour, logger)
}

// TaskRateLimitForTest creates permissive rate limiting middleware for task endpoints in test mode
func TaskRateLimitForTest(logger *slog.Logger) gin.HandlerFunc {
	// 1000 task operations per hour per user for testing
	return RateLimitByUserID(1000, time.Hour, logger)
}

// TaskCreationRateLimitForTest creates permissive rate limiting middleware for task creation in test mode
func TaskCreationRateLimitForTest(logger *slog.Logger) gin.HandlerFunc {
	// 1000 task creations per hour per user for testing
	return RateLimitByUserID(1000, time.Hour, logger)
}

// TaskExecutionRateLimitForTest creates permissive rate limiting middleware for execution endpoints in test mode
func TaskExecutionRateLimitForTest(logger *slog.Logger) gin.HandlerFunc {
	// 1000 execution operations per hour per user for testing
	return RateLimitByUserID(1000, time.Hour, logger)
}

// ExecutionCreationRateLimitForTest creates permissive rate limiting middleware for execution creation in test mode
func ExecutionCreationRateLimitForTest(logger *slog.Logger) gin.HandlerFunc {
	// 1000 execution starts per hour per user for testing
	return RateLimitByUserID(1000, time.Hour, logger)
}

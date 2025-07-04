package middleware

import (
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
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
				"error":     "Rate limit exceeded",
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
			identifier := userID.(string)
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
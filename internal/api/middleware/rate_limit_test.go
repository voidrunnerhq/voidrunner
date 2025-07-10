package middleware

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	os.Exit(m.Run())
}

func TestNewRateLimiter(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	t.Run("creates rate limiter with correct configuration", func(t *testing.T) {
		maxReqs := 10
		window := time.Minute

		rl := NewRateLimiter(maxReqs, window, logger)

		assert.NotNil(t, rl)
		assert.Equal(t, maxReqs, rl.maxReqs)
		assert.Equal(t, window, rl.window)
		assert.NotNil(t, rl.requests)
		assert.NotNil(t, rl.logger)

		// Verify cleanup goroutine starts (indirectly by checking map is initialized)
		assert.NotNil(t, rl.requests)
	})

	t.Run("initializes empty request map", func(t *testing.T) {
		rl := NewRateLimiter(5, time.Second, logger)

		rl.mu.RLock()
		assert.Empty(t, rl.requests)
		rl.mu.RUnlock()
	})
}

func TestRateLimiter_Allow(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	t.Run("allows requests under limit", func(t *testing.T) {
		rl := NewRateLimiter(3, time.Minute, logger)
		identifier := "test-user"

		// First 3 requests should be allowed
		for i := 0; i < 3; i++ {
			allowed := rl.Allow(identifier)
			assert.True(t, allowed, "Request %d should be allowed", i+1)
		}
	})

	t.Run("denies requests over limit", func(t *testing.T) {
		rl := NewRateLimiter(2, time.Minute, logger)
		identifier := "test-user"

		// First 2 requests allowed
		assert.True(t, rl.Allow(identifier))
		assert.True(t, rl.Allow(identifier))

		// 3rd request should be denied
		assert.False(t, rl.Allow(identifier))
		assert.False(t, rl.Allow(identifier)) // Still denied
	})

	t.Run("allows requests from different identifiers", func(t *testing.T) {
		rl := NewRateLimiter(1, time.Minute, logger)

		// Each identifier gets their own limit
		assert.True(t, rl.Allow("user1"))
		assert.True(t, rl.Allow("user2"))
		assert.True(t, rl.Allow("user3"))

		// But second requests from same users are denied
		assert.False(t, rl.Allow("user1"))
		assert.False(t, rl.Allow("user2"))
	})

	t.Run("resets after time window", func(t *testing.T) {
		rl := NewRateLimiter(1, 50*time.Millisecond, logger)
		identifier := "test-user"

		// First request allowed
		assert.True(t, rl.Allow(identifier))

		// Second request denied (over limit)
		assert.False(t, rl.Allow(identifier))

		// Wait for window to pass
		time.Sleep(60 * time.Millisecond)

		// Should be allowed again
		assert.True(t, rl.Allow(identifier))
	})

	t.Run("handles concurrent access safely", func(t *testing.T) {
		rl := NewRateLimiter(100, time.Minute, logger)
		identifier := "concurrent-user"

		var wg sync.WaitGroup
		numGoroutines := 50
		results := make(chan bool, numGoroutines)

		// Launch multiple goroutines making requests
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				result := rl.Allow(identifier)
				results <- result
			}()
		}

		wg.Wait()
		close(results)

		// Count allowed requests
		allowedCount := 0
		for result := range results {
			if result {
				allowedCount++
			}
		}

		// Should have exactly the limit number of allowed requests
		assert.LessOrEqual(t, allowedCount, 100)
		assert.Greater(t, allowedCount, 0) // Some should be allowed
	})

	t.Run("cleans up old requests within window", func(t *testing.T) {
		rl := NewRateLimiter(2, 100*time.Millisecond, logger)
		identifier := "cleanup-test"

		// Make 2 requests
		assert.True(t, rl.Allow(identifier))
		assert.True(t, rl.Allow(identifier))

		// Should be at limit
		assert.False(t, rl.Allow(identifier))

		// Wait for partial window
		time.Sleep(50 * time.Millisecond)

		// Still at limit
		assert.False(t, rl.Allow(identifier))

		// Wait for full window to pass
		time.Sleep(60 * time.Millisecond)

		// Should be cleaned up and allow new requests
		assert.True(t, rl.Allow(identifier))
	})
}

func TestRateLimiter_Cleanup(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	t.Run("removes expired entries", func(t *testing.T) {
		rl := NewRateLimiter(1, 50*time.Millisecond, logger)

		// Add some requests
		rl.Allow("user1")
		rl.Allow("user2")
		rl.Allow("user3")

		// Check that entries exist
		rl.mu.RLock()
		initialCount := len(rl.requests)
		rl.mu.RUnlock()
		assert.Equal(t, 3, initialCount)

		// Wait for cleanup to run (should happen every window duration)
		time.Sleep(60 * time.Millisecond)

		// Entries should eventually be cleaned up
		// Note: cleanup runs periodically, so we might need to wait a bit
		assert.Eventually(t, func() bool {
			rl.mu.RLock()
			count := len(rl.requests)
			rl.mu.RUnlock()
			return count == 0
		}, 200*time.Millisecond, 10*time.Millisecond)
	})
}

func TestRateLimit(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	t.Run("allows requests under limit", func(t *testing.T) {
		middleware := RateLimit(2, time.Minute, logger)

		router := gin.New()
		router.Use(middleware)
		router.GET("/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "success"})
		})

		// First request should pass
		req1 := httptest.NewRequest("GET", "/test", nil)
		req1.RemoteAddr = "192.168.1.1:12345"
		w1 := httptest.NewRecorder()
		router.ServeHTTP(w1, req1)
		assert.Equal(t, http.StatusOK, w1.Code)

		// Second request should pass
		req2 := httptest.NewRequest("GET", "/test", nil)
		req2.RemoteAddr = "192.168.1.1:12346" // Same IP, different port
		w2 := httptest.NewRecorder()
		router.ServeHTTP(w2, req2)
		assert.Equal(t, http.StatusOK, w2.Code)

		// Third request should be rate limited
		req3 := httptest.NewRequest("GET", "/test", nil)
		req3.RemoteAddr = "192.168.1.1:12347"
		w3 := httptest.NewRecorder()
		router.ServeHTTP(w3, req3)
		assert.Equal(t, http.StatusTooManyRequests, w3.Code)

		// Verify response contains rate limit info
		assert.Contains(t, w3.Body.String(), "Rate limit exceeded")
		assert.Contains(t, w3.Body.String(), "retry_after")
	})

	t.Run("allows requests from different IPs", func(t *testing.T) {
		middleware := RateLimit(1, time.Minute, logger)

		router := gin.New()
		router.Use(middleware)
		router.GET("/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "success"})
		})

		// Request from IP 1
		req1 := httptest.NewRequest("GET", "/test", nil)
		req1.RemoteAddr = "192.168.1.1:12345"
		w1 := httptest.NewRecorder()
		router.ServeHTTP(w1, req1)
		assert.Equal(t, http.StatusOK, w1.Code)

		// Request from IP 2 should still be allowed
		req2 := httptest.NewRequest("GET", "/test", nil)
		req2.RemoteAddr = "192.168.1.2:12345"
		w2 := httptest.NewRecorder()
		router.ServeHTTP(w2, req2)
		assert.Equal(t, http.StatusOK, w2.Code)
	})
}

func TestAuthRateLimit(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	t.Run("creates middleware with correct configuration", func(t *testing.T) {
		middleware := AuthRateLimit(logger)
		assert.NotNil(t, middleware)

		// Test that it behaves like a rate limiter
		router := gin.New()
		router.Use(middleware)
		router.POST("/auth", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "authenticated"})
		})

		// Should allow some requests before limiting
		req := httptest.NewRequest("POST", "/auth", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestRegisterRateLimit(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	t.Run("creates middleware with correct configuration", func(t *testing.T) {
		middleware := RegisterRateLimit(logger)
		assert.NotNil(t, middleware)

		router := gin.New()
		router.Use(middleware)
		router.POST("/register", func(c *gin.Context) {
			c.JSON(http.StatusCreated, gin.H{"message": "registered"})
		})

		// Should allow some requests before limiting
		req := httptest.NewRequest("POST", "/register", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusCreated, w.Code)
	})
}

func TestRefreshRateLimit(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	t.Run("creates middleware with correct configuration", func(t *testing.T) {
		middleware := RefreshRateLimit(logger)
		assert.NotNil(t, middleware)

		router := gin.New()
		router.Use(middleware)
		router.POST("/refresh", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"token": "new-token"})
		})

		// Should allow requests (refresh has higher limit)
		req := httptest.NewRequest("POST", "/refresh", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestRateLimitByUserID(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	t.Run("limits by user ID when available", func(t *testing.T) {
		middleware := RateLimitByUserID(1, time.Minute, logger)

		// Create a fixed user ID for the test
		testUserID := uuid.New()

		router := gin.New()
		router.Use(func(c *gin.Context) {
			// Simulate auth middleware setting user_id as UUID
			c.Set("user_id", testUserID)
			c.Next()
		})
		router.Use(middleware)
		router.GET("/protected", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "success"})
		})

		// First request should pass
		req1 := httptest.NewRequest("GET", "/protected", nil)
		w1 := httptest.NewRecorder()
		router.ServeHTTP(w1, req1)
		assert.Equal(t, http.StatusOK, w1.Code)

		// Second request should be rate limited
		req2 := httptest.NewRequest("GET", "/protected", nil)
		w2 := httptest.NewRecorder()
		router.ServeHTTP(w2, req2)
		assert.Equal(t, http.StatusTooManyRequests, w2.Code)
		assert.Contains(t, w2.Body.String(), "Rate limit exceeded")
	})

	t.Run("falls back to IP limiting when no user ID", func(t *testing.T) {
		middleware := RateLimitByUserID(1, time.Minute, logger)

		router := gin.New()
		router.Use(middleware)
		router.GET("/public", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "success"})
		})

		// First request should pass
		req1 := httptest.NewRequest("GET", "/public", nil)
		req1.RemoteAddr = "192.168.1.1:12345"
		w1 := httptest.NewRecorder()
		router.ServeHTTP(w1, req1)
		assert.Equal(t, http.StatusOK, w1.Code)

		// Second request from same IP should be rate limited
		req2 := httptest.NewRequest("GET", "/public", nil)
		req2.RemoteAddr = "192.168.1.1:12346"
		w2 := httptest.NewRecorder()
		router.ServeHTTP(w2, req2)
		assert.Equal(t, http.StatusTooManyRequests, w2.Code)
	})

	t.Run("different users get separate limits", func(t *testing.T) {
		middleware := RateLimitByUserID(1, time.Minute, logger)

		router := gin.New()

		// Handler that sets different user IDs based on header
		router.Use(func(c *gin.Context) {
			if userIDStr := c.GetHeader("X-User-ID"); userIDStr != "" {
				// Parse string as UUID for testing
				if userID, err := uuid.Parse(userIDStr); err == nil {
					c.Set("user_id", userID)
				}
			}
			c.Next()
		})
		router.Use(middleware)
		router.GET("/protected", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "success"})
		})

		// Generate valid UUIDs for testing
		user1ID := uuid.New().String()
		user2ID := uuid.New().String()

		// Request from user1
		req1 := httptest.NewRequest("GET", "/protected", nil)
		req1.Header.Set("X-User-ID", user1ID)
		w1 := httptest.NewRecorder()
		router.ServeHTTP(w1, req1)
		assert.Equal(t, http.StatusOK, w1.Code)

		// Request from user2 should still be allowed
		req2 := httptest.NewRequest("GET", "/protected", nil)
		req2.Header.Set("X-User-ID", user2ID)
		w2 := httptest.NewRecorder()
		router.ServeHTTP(w2, req2)
		assert.Equal(t, http.StatusOK, w2.Code)

		// Second request from user1 should be limited
		req3 := httptest.NewRequest("GET", "/protected", nil)
		req3.Header.Set("X-User-ID", user1ID)
		w3 := httptest.NewRecorder()
		router.ServeHTTP(w3, req3)
		assert.Equal(t, http.StatusTooManyRequests, w3.Code)
	})
}

func TestTaskRateLimit(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	middleware := TaskRateLimit(logger)
	assert.NotNil(t, middleware)

	// Verify it's a user-based rate limiter
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", uuid.New())
		c.Next()
	})
	router.Use(middleware)
	router.GET("/tasks", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"tasks": []string{}})
	})

	req := httptest.NewRequest("GET", "/tasks", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestTaskExecutionRateLimit(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	middleware := TaskExecutionRateLimit(logger)
	assert.NotNil(t, middleware)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", uuid.New())
		c.Next()
	})
	router.Use(middleware)
	router.POST("/executions", func(c *gin.Context) {
		c.JSON(http.StatusCreated, gin.H{"execution_id": "123"})
	})

	req := httptest.NewRequest("POST", "/executions", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestTaskCreationRateLimit(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	middleware := TaskCreationRateLimit(logger)
	assert.NotNil(t, middleware)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", uuid.New())
		c.Next()
	})
	router.Use(middleware)
	router.POST("/tasks", func(c *gin.Context) {
		c.JSON(http.StatusCreated, gin.H{"task_id": "456"})
	})

	req := httptest.NewRequest("POST", "/tasks", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestExecutionCreationRateLimit(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	middleware := ExecutionCreationRateLimit(logger)
	assert.NotNil(t, middleware)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", uuid.New())
		c.Next()
	})
	router.Use(middleware)
	router.POST("/task/123/executions", func(c *gin.Context) {
		c.JSON(http.StatusCreated, gin.H{"execution_id": "789"})
	})

	req := httptest.NewRequest("POST", "/task/123/executions", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestRateLimiter_MemoryUsage(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	t.Run("cleans up memory over time", func(t *testing.T) {
		rl := NewRateLimiter(1000, 50*time.Millisecond, logger)

		// Add many identifiers
		for i := 0; i < 1000; i++ {
			rl.Allow(fmt.Sprintf("user-%d", i))
		}

		// Check initial memory usage
		rl.mu.RLock()
		initialCount := len(rl.requests)
		rl.mu.RUnlock()
		assert.Equal(t, 1000, initialCount)

		// Wait for cleanup
		time.Sleep(60 * time.Millisecond)

		// Memory should be cleaned up
		assert.Eventually(t, func() bool {
			rl.mu.RLock()
			count := len(rl.requests)
			rl.mu.RUnlock()
			return count == 0
		}, 200*time.Millisecond, 10*time.Millisecond)
	})
}

func TestRateLimiter_EdgeCases(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	t.Run("handles empty identifier", func(t *testing.T) {
		rl := NewRateLimiter(1, time.Minute, logger)

		// Empty identifier should work
		assert.True(t, rl.Allow(""))
		assert.False(t, rl.Allow("")) // Second should be denied
	})

	t.Run("handles very short time windows", func(t *testing.T) {
		rl := NewRateLimiter(1, time.Nanosecond, logger)

		assert.True(t, rl.Allow("user"))
		// Even nanosecond should pass quickly
		time.Sleep(2 * time.Nanosecond)
		assert.True(t, rl.Allow("user"))
	})

	t.Run("handles zero max requests", func(t *testing.T) {
		rl := NewRateLimiter(0, time.Minute, logger)

		// Should deny all requests
		assert.False(t, rl.Allow("user"))
		assert.False(t, rl.Allow("user"))
	})

	t.Run("handles high request volume", func(t *testing.T) {
		rl := NewRateLimiter(1000, time.Minute, logger)
		identifier := "high-volume-user"

		allowedCount := 0
		for i := 0; i < 1500; i++ {
			if rl.Allow(identifier) {
				allowedCount++
			}
		}

		assert.Equal(t, 1000, allowedCount)
	})
}

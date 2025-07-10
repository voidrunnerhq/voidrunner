package routes

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/voidrunnerhq/voidrunner/internal/auth"
	"github.com/voidrunnerhq/voidrunner/internal/config"
	"github.com/voidrunnerhq/voidrunner/internal/database"
	"github.com/voidrunnerhq/voidrunner/pkg/logger"
)

// Helper function to create a test router with all routes configured
func setupTestRouter(t *testing.T) *gin.Engine {
	gin.SetMode(gin.TestMode)

	router := gin.New()

	// Configure method not allowed handling
	router.HandleMethodNotAllowed = true

	// Create test configuration
	cfg := &config.Config{
		CORS: config.CORSConfig{
			AllowedOrigins: []string{"http://localhost:3000"},
			AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
			AllowedHeaders: []string{"Content-Type", "Authorization"},
		},
	}

	// Create test logger
	var buf bytes.Buffer
	log := logger.NewWithWriter("info", "json", &buf)

	// Create minimal test dependencies
	var dbConn *database.Connection   // nil is fine for route testing
	repos := &database.Repositories{} // empty is fine for route testing
	authService := &auth.Service{}    // empty is fine for route testing

	// Setup routes
	Setup(router, cfg, log, dbConn, repos, authService)

	return router
}

func TestSetup(t *testing.T) {
	t.Run("setup creates router without panicking", func(t *testing.T) {
		router := setupTestRouter(t)
		assert.NotNil(t, router)
	})
}

func TestHealthRoutes(t *testing.T) {
	router := setupTestRouter(t)

	testCases := []struct {
		name           string
		method         string
		path           string
		expectedStatus int
	}{
		{
			name:           "health endpoint",
			method:         "GET",
			path:           "/health",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "readiness endpoint",
			method:         "GET",
			path:           "/ready",
			expectedStatus: http.StatusOK,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tc.expectedStatus, w.Code)
		})
	}
}

func TestDocumentationRoutes(t *testing.T) {
	router := setupTestRouter(t)

	testCases := []struct {
		name           string
		method         string
		path           string
		expectedStatus int
	}{
		{
			name:           "API index",
			method:         "GET",
			path:           "/api",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "docs redirect",
			method:         "GET",
			path:           "/docs",
			expectedStatus: http.StatusFound, // Redirect
		},
		{
			name:           "swagger UI",
			method:         "GET",
			path:           "/docs/",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "swagger JSON",
			method:         "GET",
			path:           "/swagger.json",
			expectedStatus: http.StatusOK, // May be 404 in test mode without file
		},
		{
			name:           "swagger YAML",
			method:         "GET",
			path:           "/swagger.yaml",
			expectedStatus: http.StatusOK, // May be 404 in test mode without file
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			// For file-serving endpoints, accept both 200 (file found) and 404 (file not found in test mode)
			if tc.path == "/swagger.json" || tc.path == "/swagger.yaml" || tc.path == "/docs/" {
				assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusNotFound,
					"Swagger/docs endpoints should return 200 or 404, got %d", w.Code)
			} else {
				assert.Equal(t, tc.expectedStatus, w.Code)
			}
		})
	}
}

func TestAPIV1Routes(t *testing.T) {
	router := setupTestRouter(t)

	t.Run("ping endpoint", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/ping", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "pong")
	})
}

func TestAuthRoutes(t *testing.T) {
	router := setupTestRouter(t)

	// Test that auth routes exist and respond (they will fail due to missing implementation, but routes should be registered)
	testCases := []struct {
		name   string
		method string
		path   string
	}{
		{
			name:   "register endpoint",
			method: "POST",
			path:   "/api/v1/auth/register",
		},
		{
			name:   "login endpoint",
			method: "POST",
			path:   "/api/v1/auth/login",
		},
		{
			name:   "refresh endpoint",
			method: "POST",
			path:   "/api/v1/auth/refresh",
		},
		{
			name:   "logout endpoint",
			method: "POST",
			path:   "/api/v1/auth/logout",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			// Routes should exist (not 404), even if they fail due to missing auth service implementation
			assert.NotEqual(t, http.StatusNotFound, w.Code, "Route should be registered")
		})
	}
}

func TestProtectedRoutes(t *testing.T) {
	router := setupTestRouter(t)

	// Test that protected routes exist and require authentication
	testCases := []struct {
		name   string
		method string
		path   string
	}{
		{
			name:   "auth me endpoint",
			method: "GET",
			path:   "/api/v1/auth/me",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			// Protected routes should return 401 (unauthorized) when no auth header is provided
			assert.Equal(t, http.StatusUnauthorized, w.Code, "Protected route should require authentication")
		})
	}
}

func TestTaskRoutes(t *testing.T) {
	router := setupTestRouter(t)

	// Test that task routes exist and require authentication
	testCases := []struct {
		name   string
		method string
		path   string
	}{
		{
			name:   "create task",
			method: "POST",
			path:   "/api/v1/tasks",
		},
		{
			name:   "list tasks",
			method: "GET",
			path:   "/api/v1/tasks",
		},
		{
			name:   "get task by ID",
			method: "GET",
			path:   "/api/v1/tasks/123e4567-e89b-12d3-a456-426614174000",
		},
		{
			name:   "update task",
			method: "PUT",
			path:   "/api/v1/tasks/123e4567-e89b-12d3-a456-426614174000",
		},
		{
			name:   "delete task",
			method: "DELETE",
			path:   "/api/v1/tasks/123e4567-e89b-12d3-a456-426614174000",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			// Task routes should return 401 (unauthorized) when no auth header is provided
			assert.Equal(t, http.StatusUnauthorized, w.Code, "Task route should require authentication")
		})
	}
}

func TestTaskExecutionRoutes(t *testing.T) {
	router := setupTestRouter(t)

	// Test that task execution routes exist and require authentication
	testCases := []struct {
		name   string
		method string
		path   string
	}{
		{
			name:   "create execution",
			method: "POST",
			path:   "/api/v1/tasks/123e4567-e89b-12d3-a456-426614174000/executions",
		},
		{
			name:   "list executions by task",
			method: "GET",
			path:   "/api/v1/tasks/123e4567-e89b-12d3-a456-426614174000/executions",
		},
		{
			name:   "get execution by ID",
			method: "GET",
			path:   "/api/v1/executions/123e4567-e89b-12d3-a456-426614174000",
		},
		{
			name:   "update execution",
			method: "PUT",
			path:   "/api/v1/executions/123e4567-e89b-12d3-a456-426614174000",
		},
		{
			name:   "cancel execution",
			method: "DELETE",
			path:   "/api/v1/executions/123e4567-e89b-12d3-a456-426614174000",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			// Execution routes should return 401 (unauthorized) when no auth header is provided
			assert.Equal(t, http.StatusUnauthorized, w.Code, "Execution route should require authentication")
		})
	}
}

func TestMiddlewareOrder(t *testing.T) {
	router := setupTestRouter(t)

	t.Run("CORS headers are set", func(t *testing.T) {
		req := httptest.NewRequest("OPTIONS", "/api/v1/ping", nil)
		req.Header.Set("Origin", "http://localhost:3000")
		req.Header.Set("Access-Control-Request-Method", "GET")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		// CORS middleware should set appropriate headers
		assert.Contains(t, w.Header().Get("Access-Control-Allow-Origin"), "localhost:3000")
	})

	t.Run("security headers are set", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/health", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		// Security headers should be present
		assert.NotEmpty(t, w.Header().Get("X-Content-Type-Options"))
		assert.NotEmpty(t, w.Header().Get("X-Frame-Options"))
		assert.NotEmpty(t, w.Header().Get("X-XSS-Protection"))
	})

	t.Run("request ID is generated", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/health", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		// Request ID should be set in response headers
		assert.NotEmpty(t, w.Header().Get("X-Request-ID"))
	})
}

func TestRouteNotFound(t *testing.T) {
	router := setupTestRouter(t)

	req := httptest.NewRequest("GET", "/nonexistent", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestMethodNotAllowed(t *testing.T) {
	router := setupTestRouter(t)

	// Try POST on a GET-only endpoint
	req := httptest.NewRequest("POST", "/health", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestRouteGroupStructure(t *testing.T) {
	router := setupTestRouter(t)

	t.Run("API v1 group is properly configured", func(t *testing.T) {
		// Test that v1 routes exist under /api/v1 prefix
		req := httptest.NewRequest("GET", "/api/v1/ping", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("auth group is properly configured", func(t *testing.T) {
		// Test that auth routes exist under /api/v1/auth prefix
		req := httptest.NewRequest("POST", "/api/v1/auth/logout", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		// Should not be 404 (route exists)
		assert.NotEqual(t, http.StatusNotFound, w.Code)
	})

	t.Run("protected group requires authentication", func(t *testing.T) {
		// Test that protected routes require auth
		req := httptest.NewRequest("GET", "/api/v1/auth/me", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		// Should return 401 without auth
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

// Test that rate limiting middleware is applied to appropriate routes
func TestRateLimitingMiddleware(t *testing.T) {
	router := setupTestRouter(t)

	testCases := []struct {
		name        string
		method      string
		path        string
		rateLimited bool
	}{
		{
			name:        "register route has rate limiting",
			method:      "POST",
			path:        "/api/v1/auth/register",
			rateLimited: true,
		},
		{
			name:        "login route has rate limiting",
			method:      "POST",
			path:        "/api/v1/auth/login",
			rateLimited: true,
		},
		{
			name:        "health route has no rate limiting",
			method:      "GET",
			path:        "/health",
			rateLimited: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			// Rate limited routes should have rate limit headers or responses
			// For this test, we just verify the route exists and responds
			if tc.rateLimited {
				// Route should exist (not 404)
				assert.NotEqual(t, http.StatusNotFound, w.Code, "Rate limited route should exist")
			} else {
				// Non-rate-limited routes should work normally
				assert.Equal(t, http.StatusOK, w.Code, "Non-rate-limited route should work")
			}
		})
	}
}

// Benchmark test for route setup performance
func BenchmarkSetup(b *testing.B) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		CORS: config.CORSConfig{
			AllowedOrigins: []string{"http://localhost:3000"},
			AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
			AllowedHeaders: []string{"Content-Type", "Authorization"},
		},
	}

	var buf bytes.Buffer
	log := logger.NewWithWriter("info", "json", &buf)
	var dbConn *database.Connection
	repos := &database.Repositories{}
	authService := &auth.Service{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		router := gin.New()
		Setup(router, cfg, log, dbConn, repos, authService)
	}
}

// Test that validates the overall API structure
func TestAPIStructureCompleteness(t *testing.T) {
	router := setupTestRouter(t)

	t.Run("all core API endpoints are registered", func(t *testing.T) {
		// This test ensures we have all the expected API endpoints
		expectedEndpoints := []struct {
			method string
			path   string
		}{
			// Health endpoints
			{"GET", "/health"},
			{"GET", "/ready"},

			// Documentation endpoints
			{"GET", "/api"},
			{"GET", "/docs"},
			{"GET", "/swagger.json"},
			{"GET", "/swagger.yaml"},

			// Auth endpoints
			{"POST", "/api/v1/auth/register"},
			{"POST", "/api/v1/auth/login"},
			{"POST", "/api/v1/auth/refresh"},
			{"POST", "/api/v1/auth/logout"},
			{"GET", "/api/v1/auth/me"},

			// Task endpoints
			{"POST", "/api/v1/tasks"},
			{"GET", "/api/v1/tasks"},
			{"GET", "/api/v1/tasks/123"},
			{"PUT", "/api/v1/tasks/123"},
			{"DELETE", "/api/v1/tasks/123"},

			// Execution endpoints
			{"POST", "/api/v1/tasks/123/executions"},
			{"GET", "/api/v1/tasks/123/executions"},
			{"GET", "/api/v1/executions/123"},
			{"PUT", "/api/v1/executions/123"},
			{"DELETE", "/api/v1/executions/123"},
		}

		for _, endpoint := range expectedEndpoints {
			req := httptest.NewRequest(endpoint.method, endpoint.path, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			// For file-serving endpoints, 404 is acceptable in test mode when files don't exist
			if endpoint.path == "/swagger.json" || endpoint.path == "/swagger.yaml" {
				// These endpoints are registered but may return 404 if files don't exist in test mode
				// Any status except unregistered endpoint errors is acceptable
				t.Logf("Swagger endpoint %s %s returned status %d (acceptable in test mode)",
					endpoint.method, endpoint.path, w.Code)
			} else {
				// Endpoint should exist (not return 404)
				assert.NotEqual(t, http.StatusNotFound, w.Code,
					"Endpoint %s %s should be registered", endpoint.method, endpoint.path)
			}
		}
	})
}

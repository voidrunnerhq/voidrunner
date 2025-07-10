package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockHealthChecker is a mock implementation of HealthChecker
type MockHealthChecker struct {
	status string
	err    error
}

func (m *MockHealthChecker) CheckHealth() (string, error) {
	return m.status, m.err
}

func TestHealthHandler_Health(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	handler := NewHealthHandler()
	router.GET("/health", handler.Health)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response HealthResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "ok", response.Status)
	assert.Equal(t, "voidrunner-api", response.Service)
	assert.Equal(t, "1.0.0", response.Version)
	assert.NotEmpty(t, response.Uptime)
	assert.NotZero(t, response.Timestamp)
}

func TestHealthHandler_Readiness(t *testing.T) {
	tests := []struct {
		name           string
		healthChecks   map[string]HealthChecker
		expectedStatus int
		expectedReady  bool
		expectedChecks map[string]string
	}{
		{
			name:           "no health checks - only server",
			healthChecks:   nil,
			expectedStatus: http.StatusOK,
			expectedReady:  true,
			expectedChecks: map[string]string{
				"server": "ready",
			},
		},
		{
			name: "all health checks healthy",
			healthChecks: map[string]HealthChecker{
				"database": &MockHealthChecker{status: "ready", err: nil},
				"redis":    &MockHealthChecker{status: "ready", err: nil},
			},
			expectedStatus: http.StatusOK,
			expectedReady:  true,
			expectedChecks: map[string]string{
				"server":   "ready",
				"database": "ready",
				"redis":    "ready",
			},
		},
		{
			name: "one health check fails with error",
			healthChecks: map[string]HealthChecker{
				"database": &MockHealthChecker{status: "ready", err: nil},
				"redis":    &MockHealthChecker{status: "", err: errors.New("connection failed")},
			},
			expectedStatus: http.StatusServiceUnavailable,
			expectedReady:  false,
			expectedChecks: map[string]string{
				"server":   "ready",
				"database": "ready",
				"redis":    "unhealthy",
			},
		},
		{
			name: "one health check returns non-ready status",
			healthChecks: map[string]HealthChecker{
				"database": &MockHealthChecker{status: "ready", err: nil},
				"redis":    &MockHealthChecker{status: "degraded", err: nil},
			},
			expectedStatus: http.StatusServiceUnavailable,
			expectedReady:  false,
			expectedChecks: map[string]string{
				"server":   "ready",
				"database": "ready",
				"redis":    "unhealthy",
			},
		},
		{
			name: "multiple health checks fail",
			healthChecks: map[string]HealthChecker{
				"database": &MockHealthChecker{status: "", err: errors.New("db connection lost")},
				"redis":    &MockHealthChecker{status: "down", err: nil},
				"queue":    &MockHealthChecker{status: "ready", err: nil},
			},
			expectedStatus: http.StatusServiceUnavailable,
			expectedReady:  false,
			expectedChecks: map[string]string{
				"server":   "ready",
				"database": "unhealthy",
				"redis":    "unhealthy",
				"queue":    "ready",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)

			router := gin.New()
			handler := NewHealthHandler()

			// Add health checks if any
			for name, checker := range tt.healthChecks {
				handler.AddHealthCheck(name, checker)
			}

			router.GET("/ready", handler.Readiness)

			req := httptest.NewRequest(http.MethodGet, "/ready", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			var response ReadinessResponse
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			if tt.expectedReady {
				assert.Equal(t, "ready", response.Status)
			} else {
				assert.Equal(t, "not ready", response.Status)
			}

			assert.NotZero(t, response.Timestamp)

			// Verify all expected checks are present
			for expectedName, expectedStatus := range tt.expectedChecks {
				assert.Equal(t, expectedStatus, response.Checks[expectedName],
					"Check %s should have status %s", expectedName, expectedStatus)
			}

			// Verify no unexpected checks are present
			assert.Equal(t, len(tt.expectedChecks), len(response.Checks),
				"Number of checks should match expected")
		})
	}
}

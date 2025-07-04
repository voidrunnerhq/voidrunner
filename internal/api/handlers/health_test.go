package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
	
	assert.Equal(t, "healthy", response.Status)
	assert.Equal(t, "voidrunner-api", response.Service)
	assert.Equal(t, "1.0.0", response.Version)
	assert.NotEmpty(t, response.Uptime)
	assert.NotZero(t, response.Timestamp)
}

func TestHealthHandler_Readiness(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	router := gin.New()
	handler := NewHealthHandler()
	router.GET("/ready", handler.Readiness)

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	
	var response ReadinessResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	
	assert.Equal(t, "ready", response.Status)
	assert.NotEmpty(t, response.Checks)
	assert.Equal(t, "ready", response.Checks["server"])
	assert.NotZero(t, response.Timestamp)
}
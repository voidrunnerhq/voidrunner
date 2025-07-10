package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// HealthChecker defines an interface for health check dependencies
type HealthChecker interface {
	CheckHealth() (status string, err error)
}

type HealthHandler struct {
	startTime    time.Time
	healthChecks map[string]HealthChecker
}

func NewHealthHandler() *HealthHandler {
	return &HealthHandler{
		startTime:    time.Now(),
		healthChecks: make(map[string]HealthChecker),
	}
}

// AddHealthCheck adds a health check for a specific dependency
func (h *HealthHandler) AddHealthCheck(name string, checker HealthChecker) {
	h.healthChecks[name] = checker
}

type HealthResponse struct {
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
	Uptime    string    `json:"uptime"`
	Version   string    `json:"version,omitempty"`
	Service   string    `json:"service"`
}

type ReadinessResponse struct {
	Status    string            `json:"status"`
	Checks    map[string]string `json:"checks"`
	Timestamp time.Time         `json:"timestamp"`
}

// Health performs a basic health check
//
//	@Summary		Health check
//	@Description	Returns the health status of the API service
//	@Tags			Health
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	HealthResponse	"Service is healthy"
//	@Router			/health [get]
func (h *HealthHandler) Health(c *gin.Context) {
	uptime := time.Since(h.startTime)

	response := HealthResponse{
		Status:    "ok",
		Timestamp: time.Now(),
		Uptime:    uptime.String(),
		Version:   "1.0.0",
		Service:   "voidrunner-api",
	}

	c.JSON(http.StatusOK, response)
}

// Readiness performs a readiness check
//
//	@Summary		Readiness check
//	@Description	Returns the readiness status of the API service and its dependencies
//	@Tags			Health
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	ReadinessResponse	"Service is ready"
//	@Failure		503	{object}	ReadinessResponse	"Service is not ready"
//	@Router			/ready [get]
func (h *HealthHandler) Readiness(c *gin.Context) {
	checks := make(map[string]string)

	// Always include basic server check
	checks["server"] = "ready"

	// Run all registered health checks
	allHealthy := true
	for name, checker := range h.healthChecks {
		status, err := checker.CheckHealth()
		if err != nil || status != "ready" {
			checks[name] = "unhealthy"
			allHealthy = false
		} else {
			checks[name] = status
		}
	}

	// Check if all components are healthy
	for _, status := range checks {
		if status != "ready" {
			allHealthy = false
			break
		}
	}

	response := ReadinessResponse{
		Status:    "ready",
		Checks:    checks,
		Timestamp: time.Now(),
	}

	if !allHealthy {
		response.Status = "not ready"
		c.JSON(http.StatusServiceUnavailable, response)
		return
	}

	c.JSON(http.StatusOK, response)
}

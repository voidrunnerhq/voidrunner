package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type HealthHandler struct {
	startTime time.Time
}

func NewHealthHandler() *HealthHandler {
	return &HealthHandler{
		startTime: time.Now(),
	}
}

type HealthResponse struct {
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
	Uptime    string    `json:"uptime"`
	Version   string    `json:"version,omitempty"`
	Service   string    `json:"service"`
}

type ReadinessResponse struct {
	Status   string            `json:"status"`
	Checks   map[string]string `json:"checks"`
	Timestamp time.Time        `json:"timestamp"`
}

func (h *HealthHandler) Health(c *gin.Context) {
	uptime := time.Since(h.startTime)
	
	response := HealthResponse{
		Status:    "healthy",
		Timestamp: time.Now(),
		Uptime:    uptime.String(),
		Version:   "1.0.0",
		Service:   "voidrunner-api",
	}

	c.JSON(http.StatusOK, response)
}

func (h *HealthHandler) Readiness(c *gin.Context) {
	checks := make(map[string]string)
	
	checks["server"] = "ready"
	
	allHealthy := true
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
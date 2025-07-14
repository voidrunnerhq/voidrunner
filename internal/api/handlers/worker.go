package handlers

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/voidrunnerhq/voidrunner/internal/worker"
)

type WorkerHandler struct {
	manager worker.WorkerManager
	logger  *slog.Logger
}

func NewWorkerHandler(manager worker.WorkerManager, logger *slog.Logger) *WorkerHandler {
	return &WorkerHandler{
		manager: manager,
		logger:  logger,
	}
}

type WorkerStatusResponse struct {
	Status        string              `json:"status"`
	Timestamp     time.Time           `json:"timestamp"`
	WorkerManager WorkerManagerStatus `json:"worker_manager"`
	WorkerPool    WorkerPoolStatus    `json:"worker_pool"`
	Concurrency   ConcurrencyStatus   `json:"concurrency"`
}

type WorkerManagerStatus struct {
	IsRunning bool `json:"is_running"`
	IsHealthy bool `json:"is_healthy"`
}

type WorkerPoolStatus struct {
	PoolSize             int           `json:"pool_size"`
	ActiveWorkers        int           `json:"active_workers"`
	IdleWorkers          int           `json:"idle_workers"`
	UnhealthyWorkers     int           `json:"unhealthy_workers"`
	TotalTasksProcessed  int64         `json:"total_tasks_processed"`
	TotalTasksSuccessful int64         `json:"total_tasks_successful"`
	TotalTasksFailed     int64         `json:"total_tasks_failed"`
	AverageTaskTime      time.Duration `json:"average_task_time" swaggertype:"string" example:"1m30s"`
}

type ConcurrencyStatus struct {
	TotalActiveSlots   int   `json:"total_active_slots"`
	AvailableSlots     int   `json:"available_slots"`
	SlotsAcquiredTotal int64 `json:"slots_acquired_total"`
	SlotsReleasedTotal int64 `json:"slots_released_total"`
}

// GetWorkerStatus returns detailed status information about the worker system
//
//	@Summary		Worker status
//	@Description	Returns detailed status information about the embedded worker system
//	@Tags			Health
//	@Produce		json
//	@Success		200	{object}	WorkerStatusResponse
//	@Failure		503	{object}	map[string]string
//	@Router			/health/workers [get]
func (w *WorkerHandler) GetWorkerStatus(c *gin.Context) {
	if w.manager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "worker manager not available (embedded workers may be disabled)",
		})
		return
	}

	stats := w.manager.GetStats()

	status := "healthy"
	if !stats.IsRunning || !stats.IsHealthy {
		status = "unhealthy"
	}

	response := WorkerStatusResponse{
		Status:    status,
		Timestamp: time.Now(),
		WorkerManager: WorkerManagerStatus{
			IsRunning: stats.IsRunning,
			IsHealthy: stats.IsHealthy,
		},
		WorkerPool: WorkerPoolStatus{
			PoolSize:             stats.WorkerPoolStats.PoolSize,
			ActiveWorkers:        stats.WorkerPoolStats.ActiveWorkers,
			IdleWorkers:          stats.WorkerPoolStats.IdleWorkers,
			UnhealthyWorkers:     stats.WorkerPoolStats.UnhealthyWorkers,
			TotalTasksProcessed:  stats.WorkerPoolStats.TotalTasksProcessed,
			TotalTasksSuccessful: stats.WorkerPoolStats.TotalTasksSuccessful,
			TotalTasksFailed:     stats.WorkerPoolStats.TotalTasksFailed,
			AverageTaskTime:      stats.WorkerPoolStats.AverageTaskTime,
		},
		Concurrency: ConcurrencyStatus{
			TotalActiveSlots:   stats.ConcurrencyStats.TotalActiveSlots,
			AvailableSlots:     stats.ConcurrencyStats.AvailableSlots,
			SlotsAcquiredTotal: stats.ConcurrencyStats.SlotsAcquiredTotal,
			SlotsReleasedTotal: stats.ConcurrencyStats.SlotsReleasedTotal,
		},
	}

	httpStatus := http.StatusOK
	if status == "unhealthy" {
		httpStatus = http.StatusServiceUnavailable
	}

	c.JSON(httpStatus, response)
}

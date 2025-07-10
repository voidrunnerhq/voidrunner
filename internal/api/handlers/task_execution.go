package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/voidrunnerhq/voidrunner/internal/api/middleware"
	"github.com/voidrunnerhq/voidrunner/internal/database"
	"github.com/voidrunnerhq/voidrunner/internal/models"
)

// TaskExecutionServiceInterface defines the interface for task execution services
type TaskExecutionServiceInterface interface {
	CreateExecutionAndUpdateTaskStatus(ctx context.Context, taskID uuid.UUID, userID uuid.UUID) (*models.TaskExecution, error)
	CancelExecutionAndResetTaskStatus(ctx context.Context, executionID uuid.UUID, userID uuid.UUID) error
	CompleteExecutionAndFinalizeTaskStatus(ctx context.Context, execution *models.TaskExecution, taskStatus models.TaskStatus, userID uuid.UUID) error
}

// TaskExecutionHandler handles task execution-related API endpoints
type TaskExecutionHandler struct {
	taskRepo         database.TaskRepository
	executionRepo    database.TaskExecutionRepository
	executionService TaskExecutionServiceInterface
	logger           *slog.Logger
}

// NewTaskExecutionHandler creates a new task execution handler
func NewTaskExecutionHandler(taskRepo database.TaskRepository, executionRepo database.TaskExecutionRepository, executionService TaskExecutionServiceInterface, logger *slog.Logger) *TaskExecutionHandler {
	return &TaskExecutionHandler{
		taskRepo:         taskRepo,
		executionRepo:    executionRepo,
		executionService: executionService,
		logger:           logger,
	}
}

// Create handles creating a new task execution
//
//	@Summary		Start task execution
//	@Description	Starts execution of the specified task
//	@Tags			Executions
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			task_id	path	string	true	"Task ID"
//	@Success		201		{object}	models.TaskExecutionResponse	"Execution started successfully"
//	@Failure		400		{object}	models.ErrorResponse			"Invalid task ID"
//	@Failure		401		{object}	models.ErrorResponse			"Unauthorized"
//	@Failure		403		{object}	models.ErrorResponse			"Forbidden"
//	@Failure		404		{object}	models.ErrorResponse			"Task not found"
//	@Failure		409		{object}	models.ErrorResponse			"Task is already running"
//	@Failure		429		{object}	models.ErrorResponse			"Rate limit exceeded"
//	@Router			/tasks/{task_id}/executions [post]
func (h *TaskExecutionHandler) Create(c *gin.Context) {
	taskIDStr := c.Param("id")
	taskID, err := uuid.Parse(taskIDStr)
	if err != nil {
		h.logger.Warn("invalid task ID", "task_id", taskIDStr)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid task ID format",
		})
		return
	}

	// Get user from context
	user := middleware.GetUserFromContext(c)
	if user == nil {
		h.logger.Error("user not found in context")
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Unauthorized",
		})
		return
	}

	// Use service layer to atomically create execution and update task status
	execution, err := h.executionService.CreateExecutionAndUpdateTaskStatus(c.Request.Context(), taskID, user.ID)
	if err != nil {
		h.logger.Error("failed to create execution and update task status", "error", err, "task_id", taskID, "user_id", user.ID)

		// Map service errors to appropriate HTTP status codes
		switch err.Error() {
		case "task not found":
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Task not found",
			})
		case "access denied: task does not belong to user":
			c.JSON(http.StatusForbidden, gin.H{
				"error": "Access denied",
			})
		case "task is already running":
			c.JSON(http.StatusConflict, gin.H{
				"error": "Task is already running",
			})
		default:
			if strings.HasPrefix(err.Error(), "cannot execute task with status:") {
				c.JSON(http.StatusConflict, gin.H{
					"error": err.Error(),
				})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": "Failed to create task execution",
				})
			}
		}
		return
	}

	h.logger.Info("task execution created successfully", "execution_id", execution.ID, "task_id", taskID, "user_id", user.ID)
	c.JSON(http.StatusCreated, execution.ToResponse())
}

// GetByID handles retrieving a task execution by ID
func (h *TaskExecutionHandler) GetByID(c *gin.Context) {
	executionIDStr := c.Param("id")
	executionID, err := uuid.Parse(executionIDStr)
	if err != nil {
		h.logger.Warn("invalid execution ID", "execution_id", executionIDStr)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid execution ID format",
		})
		return
	}

	// Get user from context
	user := middleware.GetUserFromContext(c)
	if user == nil {
		h.logger.Error("user not found in context")
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Unauthorized",
		})
		return
	}

	// Get execution from database
	execution, err := h.executionRepo.GetByID(c.Request.Context(), executionID)
	if err != nil {
		if err == database.ErrExecutionNotFound {
			h.logger.Warn("execution not found", "execution_id", executionID, "user_id", user.ID)
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Execution not found",
			})
			return
		}
		h.logger.Error("failed to get execution", "error", err, "execution_id", executionID)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve execution",
		})
		return
	}

	// Get task to verify ownership
	task, err := h.taskRepo.GetByID(c.Request.Context(), execution.TaskID)
	if err != nil {
		h.logger.Error("failed to get task for execution", "error", err, "task_id", execution.TaskID)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve task",
		})
		return
	}

	// Check if user owns the task
	if task.UserID != user.ID {
		h.logger.Warn("user attempted to access another user's execution",
			"user_id", user.ID, "execution_id", executionID, "task_owner_id", task.UserID)
		c.JSON(http.StatusForbidden, gin.H{
			"error": "Access denied",
		})
		return
	}

	h.logger.Debug("execution retrieved successfully", "execution_id", executionID, "user_id", user.ID)
	c.JSON(http.StatusOK, execution.ToResponse())
}

// ListByTaskID handles listing executions for a specific task
func (h *TaskExecutionHandler) ListByTaskID(c *gin.Context) {
	taskIDStr := c.Param("id")
	taskID, err := uuid.Parse(taskIDStr)
	if err != nil {
		h.logger.Warn("invalid task ID", "task_id", taskIDStr)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid task ID format",
		})
		return
	}

	// Get user from context
	user := middleware.GetUserFromContext(c)
	if user == nil {
		h.logger.Error("user not found in context")
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Unauthorized",
		})
		return
	}

	// Get task to verify ownership
	task, err := h.taskRepo.GetByID(c.Request.Context(), taskID)
	if err != nil {
		if err == database.ErrTaskNotFound {
			h.logger.Warn("task not found", "task_id", taskID, "user_id", user.ID)
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Task not found",
			})
			return
		}
		h.logger.Error("failed to get task", "error", err, "task_id", taskID)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve task",
		})
		return
	}

	// Check if user owns the task
	if task.UserID != user.ID {
		h.logger.Warn("user attempted to access another user's task executions",
			"user_id", user.ID, "task_id", taskID, "task_owner_id", task.UserID)
		c.JSON(http.StatusForbidden, gin.H{
			"error": "Access denied",
		})
		return
	}

	// Parse pagination parameters
	limit, offset, err := h.parsePagination(c)
	if err != nil {
		h.logger.Warn("invalid pagination parameters", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	// Get executions from database
	executions, err := h.executionRepo.GetByTaskID(c.Request.Context(), taskID, limit, offset)
	if err != nil {
		h.logger.Error("failed to get task executions", "error", err, "task_id", taskID)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve executions",
		})
		return
	}

	// Get total count
	total, err := h.executionRepo.CountByTaskID(c.Request.Context(), taskID)
	if err != nil {
		h.logger.Error("failed to count task executions", "error", err, "task_id", taskID)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to count executions",
		})
		return
	}

	// Convert to response format
	executionResponses := make([]models.TaskExecutionResponse, len(executions))
	for i, execution := range executions {
		executionResponses[i] = execution.ToResponse()
	}

	h.logger.Debug("task executions retrieved successfully", "task_id", taskID, "user_id", user.ID, "count", len(executions))
	c.JSON(http.StatusOK, gin.H{
		"executions": executionResponses,
		"total":      total,
		"limit":      limit,
		"offset":     offset,
	})
}

// Cancel handles canceling a task execution
func (h *TaskExecutionHandler) Cancel(c *gin.Context) {
	executionIDStr := c.Param("id")
	executionID, err := uuid.Parse(executionIDStr)
	if err != nil {
		h.logger.Warn("invalid execution ID", "execution_id", executionIDStr)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid execution ID format",
		})
		return
	}

	// Get user from context
	user := middleware.GetUserFromContext(c)
	if user == nil {
		h.logger.Error("user not found in context")
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Unauthorized",
		})
		return
	}

	// Use service layer to atomically cancel execution and reset task status
	err = h.executionService.CancelExecutionAndResetTaskStatus(c.Request.Context(), executionID, user.ID)
	if err != nil {
		h.logger.Error("failed to cancel execution and reset task status", "error", err, "execution_id", executionID, "user_id", user.ID)

		// Map service errors to appropriate HTTP status codes
		switch err.Error() {
		case "execution not found":
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Execution not found",
			})
		case "access denied: task does not belong to user":
			c.JSON(http.StatusForbidden, gin.H{
				"error": "Access denied",
			})
		default:
			if strings.HasPrefix(err.Error(), "cannot cancel execution with status:") {
				c.JSON(http.StatusConflict, gin.H{
					"error": err.Error(),
				})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": "Failed to cancel execution",
				})
			}
		}
		return
	}

	h.logger.Info("execution cancelled successfully", "execution_id", executionID, "user_id", user.ID)
	c.JSON(http.StatusOK, gin.H{
		"message": "Execution cancelled successfully",
	})
}

// Update handles updating execution status and results (typically called by the execution system)
func (h *TaskExecutionHandler) Update(c *gin.Context) {
	executionIDStr := c.Param("id")
	executionID, err := uuid.Parse(executionIDStr)
	if err != nil {
		h.logger.Warn("invalid execution ID", "execution_id", executionIDStr)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid execution ID format",
		})
		return
	}

	// Get validated request from middleware
	validatedBody, exists := c.Get("validated_body")
	if !exists {
		// Fallback to manual validation if middleware wasn't used
		var req models.UpdateTaskExecutionRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			h.logger.Warn("invalid execution update request", "error", err)
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "Invalid request format",
				"details": err.Error(),
			})
			return
		}
		validatedBody = &req
	}

	req := *validatedBody.(*models.UpdateTaskExecutionRequest)

	// Get user from context
	user := middleware.GetUserFromContext(c)
	if user == nil {
		h.logger.Error("user not found in context")
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Unauthorized",
		})
		return
	}

	// Get execution from database to apply updates
	execution, err := h.executionRepo.GetByID(c.Request.Context(), executionID)
	if err != nil {
		if err == database.ErrExecutionNotFound {
			h.logger.Warn("execution not found", "execution_id", executionID, "user_id", user.ID)
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Execution not found",
			})
			return
		}
		h.logger.Error("failed to get execution", "error", err, "execution_id", executionID)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve execution",
		})
		return
	}

	// Apply updates to execution
	if err := h.applyExecutionUpdates(execution, req); err != nil {
		h.logger.Warn("execution update validation failed", "error", err, "execution_id", executionID)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	// Check if this update makes the execution terminal
	isTerminalUpdate := execution.IsTerminal()

	if isTerminalUpdate {
		// Use service layer for atomic completion
		var taskStatus models.TaskStatus
		switch execution.Status {
		case models.ExecutionStatusCompleted:
			taskStatus = models.TaskStatusCompleted
		case models.ExecutionStatusFailed:
			taskStatus = models.TaskStatusFailed
		case models.ExecutionStatusTimeout:
			taskStatus = models.TaskStatusTimeout
		case models.ExecutionStatusCancelled:
			taskStatus = models.TaskStatusCancelled
		default:
			taskStatus = models.TaskStatusPending // fallback
		}

		err = h.executionService.CompleteExecutionAndFinalizeTaskStatus(c.Request.Context(), execution, taskStatus, user.ID)
		if err != nil {
			h.logger.Error("failed to complete execution and finalize task status", "error", err, "execution_id", executionID, "user_id", user.ID)

			// Map service errors to appropriate HTTP status codes
			switch err.Error() {
			case "execution not found":
				c.JSON(http.StatusNotFound, gin.H{
					"error": "Execution not found",
				})
			case "access denied: task does not belong to user":
				c.JSON(http.StatusForbidden, gin.H{
					"error": "Access denied",
				})
			default:
				if strings.HasPrefix(err.Error(), "cannot complete execution with status:") {
					c.JSON(http.StatusConflict, gin.H{
						"error": err.Error(),
					})
				} else {
					c.JSON(http.StatusInternalServerError, gin.H{
						"error": "Failed to update execution",
					})
				}
			}
			return
		}
	} else {
		// For non-terminal updates, just update the execution (no task status change needed)
		// First verify user has access to this execution
		task, err := h.taskRepo.GetByID(c.Request.Context(), execution.TaskID)
		if err != nil {
			h.logger.Error("failed to get task for execution", "error", err, "task_id", execution.TaskID)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to retrieve task",
			})
			return
		}

		if task.UserID != user.ID {
			h.logger.Warn("user attempted to update another user's execution",
				"user_id", user.ID, "execution_id", executionID, "task_owner_id", task.UserID)
			c.JSON(http.StatusForbidden, gin.H{
				"error": "Access denied",
			})
			return
		}

		// Simple execution update without task status change
		if err := h.executionRepo.Update(c.Request.Context(), execution); err != nil {
			h.logger.Error("failed to update execution", "error", err, "execution_id", executionID)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to update execution",
			})
			return
		}
	}

	h.logger.Info("execution updated successfully", "execution_id", executionID, "user_id", user.ID)
	c.JSON(http.StatusOK, execution.ToResponse())
}

// applyExecutionUpdates applies the update request to the execution
func (h *TaskExecutionHandler) applyExecutionUpdates(execution *models.TaskExecution, req models.UpdateTaskExecutionRequest) error {
	if req.Status != nil {
		if err := models.ValidateExecutionStatus(*req.Status); err != nil {
			return err
		}
		execution.Status = *req.Status
	}

	if req.ReturnCode != nil {
		execution.ReturnCode = req.ReturnCode
	}

	if req.Stdout != nil {
		execution.Stdout = req.Stdout
	}

	if req.Stderr != nil {
		execution.Stderr = req.Stderr
	}

	if req.ExecutionTimeMs != nil {
		execution.ExecutionTimeMs = req.ExecutionTimeMs
	}

	if req.MemoryUsageBytes != nil {
		execution.MemoryUsageBytes = req.MemoryUsageBytes
	}

	if req.StartedAt != nil {
		execution.StartedAt = req.StartedAt
	}

	if req.CompletedAt != nil {
		execution.CompletedAt = req.CompletedAt
	}

	return nil
}

// parsePagination parses pagination parameters from query string
func (h *TaskExecutionHandler) parsePagination(c *gin.Context) (limit, offset int, err error) {
	// Default values
	limit = 20
	offset = 0

	// Parse limit
	if limitStr := c.Query("limit"); limitStr != "" {
		limit, err = strconv.Atoi(limitStr)
		if err != nil {
			return 0, 0, fmt.Errorf("invalid limit parameter: %w", err)
		}
		if limit < 1 || limit > 100 {
			return 0, 0, fmt.Errorf("limit must be between 1 and 100")
		}
	}

	// Parse offset
	if offsetStr := c.Query("offset"); offsetStr != "" {
		offset, err = strconv.Atoi(offsetStr)
		if err != nil {
			return 0, 0, fmt.Errorf("invalid offset parameter: %w", err)
		}
		if offset < 0 {
			return 0, 0, fmt.Errorf("offset must be non-negative")
		}
	}

	return limit, offset, nil
}

package handlers

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/voidrunnerhq/voidrunner/internal/api/middleware"
	"github.com/voidrunnerhq/voidrunner/internal/config"
	"github.com/voidrunnerhq/voidrunner/internal/database"
	"github.com/voidrunnerhq/voidrunner/internal/models"
)

// TaskHandler handles task-related API endpoints
type TaskHandler struct {
	taskRepo database.TaskRepository
	logger   *slog.Logger
}

// NewTaskHandler creates a new task handler
func NewTaskHandler(taskRepo database.TaskRepository, logger *slog.Logger) *TaskHandler {
	return &TaskHandler{
		taskRepo: taskRepo,
		logger:   logger,
	}
}

// Create handles task creation
//
//	@Summary		Create a new task
//	@Description	Creates a new task with the specified script content and configuration
//	@Tags			Tasks
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			request	body		models.CreateTaskRequest	true	"Task creation details"
//	@Success		201		{object}	models.TaskResponse			"Task created successfully"
//	@Failure		400		{object}	models.ErrorResponse		"Invalid request format or validation error"
//	@Failure		401		{object}	models.ErrorResponse		"Unauthorized"
//	@Failure		429		{object}	models.ErrorResponse		"Rate limit exceeded"
//	@Router			/tasks [post]
func (h *TaskHandler) Create(c *gin.Context) {
	// Get validated request from middleware
	validatedBody, exists := c.Get("validated_body")
	if !exists {
		// Fallback to manual validation if middleware wasn't used
		var req models.CreateTaskRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			h.logger.Warn("invalid task creation request", "error", err)
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "Invalid request format",
				"details": err.Error(),
			})
			return
		}
		validatedBody = &req
	}

	req := *validatedBody.(*models.CreateTaskRequest)

	// Get user from context
	user := middleware.GetUserFromContext(c)
	if user == nil {
		h.logger.Error("user not found in context")
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Unauthorized",
		})
		return
	}

	// Validate request
	if err := h.validateCreateRequest(req); err != nil {
		h.logger.Warn("task creation validation failed", "error", err, "user_id", user.ID)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	// Create task model
	task := &models.Task{
		BaseModel: models.BaseModel{
			ID: uuid.New(),
		},
		UserID:         user.ID,
		Name:           req.Name,
		Description:    req.Description,
		ScriptContent:  req.ScriptContent,
		ScriptType:     req.ScriptType,
		Status:         models.TaskStatusPending,
		Priority:       5, // Default priority
		TimeoutSeconds: config.DefaultTaskTimeout,
		Metadata:       req.Metadata,
	}

	// Set optional fields
	if req.Priority != nil {
		task.Priority = *req.Priority
	}
	if req.TimeoutSeconds != nil {
		task.TimeoutSeconds = *req.TimeoutSeconds
	}

	// Create task in database
	if err := h.taskRepo.Create(c.Request.Context(), task); err != nil {
		h.logger.Error("failed to create task", "error", err, "user_id", user.ID)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to create task",
		})
		return
	}

	h.logger.Info("task created successfully", "task_id", task.ID, "user_id", user.ID)
	c.JSON(http.StatusCreated, task.ToResponse())
}

// GetByID handles retrieving a task by ID
//
//	@Summary		Get task details
//	@Description	Retrieves detailed information about a specific task
//	@Tags			Tasks
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		string	true	"Task ID"
//	@Success		200	{object}	models.TaskResponse		"Task retrieved successfully"
//	@Failure		400	{object}	models.ErrorResponse	"Invalid task ID"
//	@Failure		401	{object}	models.ErrorResponse	"Unauthorized"
//	@Failure		403	{object}	models.ErrorResponse	"Forbidden"
//	@Failure		404	{object}	models.ErrorResponse	"Task not found"
//	@Failure		429	{object}	models.ErrorResponse	"Rate limit exceeded"
//	@Router			/tasks/{id} [get]
func (h *TaskHandler) GetByID(c *gin.Context) {
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

	// Get task from database
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
		h.logger.Warn("user attempted to access another user's task",
			"user_id", user.ID, "task_id", taskID, "task_owner_id", task.UserID)
		c.JSON(http.StatusForbidden, gin.H{
			"error": "Access denied",
		})
		return
	}

	h.logger.Debug("task retrieved successfully", "task_id", taskID, "user_id", user.ID)
	c.JSON(http.StatusOK, task.ToResponse())
}

// List handles listing user's tasks with pagination
//
//	@Summary		List user's tasks
//	@Description	Retrieves a paginated list of tasks owned by the authenticated user
//	@Tags			Tasks
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			limit	query	int	false	"Maximum number of tasks to return"	default(20)
//	@Param			offset	query	int	false	"Number of tasks to skip"	default(0)
//	@Success		200		{object}	models.TaskListResponse	"Tasks retrieved successfully"
//	@Failure		400		{object}	models.ErrorResponse	"Invalid query parameters"
//	@Failure		401		{object}	models.ErrorResponse	"Unauthorized"
//	@Failure		429		{object}	models.ErrorResponse	"Rate limit exceeded"
//	@Router			/tasks [get]
func (h *TaskHandler) List(c *gin.Context) {
	// Get user from context
	user := middleware.GetUserFromContext(c)
	if user == nil {
		h.logger.Error("user not found in context")
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Unauthorized",
		})
		return
	}

	// Try to parse cursor pagination first
	cursorReq, useCursor, err := h.parseCursorPagination(c)
	if err != nil {
		h.logger.Warn("invalid cursor pagination parameters", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	if useCursor {
		// Use cursor-based pagination
		tasks, paginationResp, err := h.taskRepo.GetByUserIDCursor(c.Request.Context(), user.ID, cursorReq)
		if err != nil {
			h.logger.Error("failed to get user tasks with cursor", "error", err, "user_id", user.ID)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to retrieve tasks",
			})
			return
		}

		// Convert to response format
		taskResponses := make([]models.TaskResponse, len(tasks))
		for i, task := range tasks {
			taskResponses[i] = task.ToResponse()
		}

		h.logger.Debug("tasks retrieved successfully with cursor", "user_id", user.ID, "count", len(tasks))
		c.JSON(http.StatusOK, gin.H{
			"tasks":      taskResponses,
			"pagination": paginationResp,
			"limit":      cursorReq.Limit,
			"sort_order": cursorReq.SortOrder,
			"sort_field": cursorReq.SortField,
		})
	} else {
		// Use offset-based pagination (legacy)
		limit, offset, err := h.parsePagination(c)
		if err != nil {
			h.logger.Warn("invalid offset pagination parameters", "error", err)
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return
		}

		// Get tasks from database
		tasks, err := h.taskRepo.GetByUserID(c.Request.Context(), user.ID, limit, offset)
		if err != nil {
			h.logger.Error("failed to get user tasks", "error", err, "user_id", user.ID)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to retrieve tasks",
			})
			return
		}

		// Get total count for offset pagination
		total, err := h.taskRepo.CountByUserID(c.Request.Context(), user.ID)
		if err != nil {
			h.logger.Error("failed to count user tasks", "error", err, "user_id", user.ID)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to count tasks",
			})
			return
		}

		// Convert to response format
		taskResponses := make([]models.TaskResponse, len(tasks))
		for i, task := range tasks {
			taskResponses[i] = task.ToResponse()
		}

		h.logger.Debug("tasks retrieved successfully with offset", "user_id", user.ID, "count", len(tasks))
		c.JSON(http.StatusOK, gin.H{
			"tasks":  taskResponses,
			"total":  total,
			"limit":  limit,
			"offset": offset,
		})
	}
}

// Update handles updating a task
func (h *TaskHandler) Update(c *gin.Context) {
	taskIDStr := c.Param("id")
	taskID, err := uuid.Parse(taskIDStr)
	if err != nil {
		h.logger.Warn("invalid task ID", "task_id", taskIDStr)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid task ID format",
		})
		return
	}

	// Get validated request from middleware
	validatedBody, exists := c.Get("validated_body")
	if !exists {
		// Fallback to manual validation if middleware wasn't used
		var req models.UpdateTaskRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			h.logger.Warn("invalid task update request", "error", err)
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "Invalid request format",
				"details": err.Error(),
			})
			return
		}
		validatedBody = &req
	}

	req := *validatedBody.(*models.UpdateTaskRequest)

	// Get user from context
	user := middleware.GetUserFromContext(c)
	if user == nil {
		h.logger.Error("user not found in context")
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Unauthorized",
		})
		return
	}

	// Get existing task
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
		h.logger.Warn("user attempted to update another user's task",
			"user_id", user.ID, "task_id", taskID, "task_owner_id", task.UserID)
		c.JSON(http.StatusForbidden, gin.H{
			"error": "Access denied",
		})
		return
	}

	// Check if task is running (cannot update running tasks)
	if task.Status == models.TaskStatusRunning {
		h.logger.Warn("attempted to update running task", "task_id", taskID, "user_id", user.ID)
		c.JSON(http.StatusConflict, gin.H{
			"error": "Cannot update running task",
		})
		return
	}

	// Apply updates
	if err := h.applyTaskUpdates(task, req); err != nil {
		h.logger.Warn("task update validation failed", "error", err, "task_id", taskID)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	// Update task in database
	if err := h.taskRepo.Update(c.Request.Context(), task); err != nil {
		h.logger.Error("failed to update task", "error", err, "task_id", taskID)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to update task",
		})
		return
	}

	h.logger.Info("task updated successfully", "task_id", taskID, "user_id", user.ID)
	c.JSON(http.StatusOK, task.ToResponse())
}

// Delete handles deleting a task
func (h *TaskHandler) Delete(c *gin.Context) {
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

	// Get existing task
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
		h.logger.Warn("user attempted to delete another user's task",
			"user_id", user.ID, "task_id", taskID, "task_owner_id", task.UserID)
		c.JSON(http.StatusForbidden, gin.H{
			"error": "Access denied",
		})
		return
	}

	// Check if task is running (cannot delete running tasks)
	if task.Status == models.TaskStatusRunning {
		h.logger.Warn("attempted to delete running task", "task_id", taskID, "user_id", user.ID)
		c.JSON(http.StatusConflict, gin.H{
			"error": "Cannot delete running task",
		})
		return
	}

	// Delete task from database
	if err := h.taskRepo.Delete(c.Request.Context(), taskID); err != nil {
		h.logger.Error("failed to delete task", "error", err, "task_id", taskID)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to delete task",
		})
		return
	}

	h.logger.Info("task deleted successfully", "task_id", taskID, "user_id", user.ID)
	c.JSON(http.StatusOK, gin.H{
		"message": "Task deleted successfully",
	})
}

// validateCreateRequest validates the create task request
func (h *TaskHandler) validateCreateRequest(req models.CreateTaskRequest) error {
	if err := models.ValidateTaskName(req.Name); err != nil {
		return err
	}

	if err := models.ValidateScriptType(req.ScriptType); err != nil {
		return err
	}

	if err := models.ValidateScriptContent(req.ScriptContent); err != nil {
		return err
	}

	if req.Priority != nil {
		if err := models.ValidatePriority(*req.Priority); err != nil {
			return err
		}
	}

	if req.TimeoutSeconds != nil {
		if err := models.ValidateTimeout(*req.TimeoutSeconds); err != nil {
			return err
		}
	}

	return nil
}

// applyTaskUpdates applies the update request to the task
func (h *TaskHandler) applyTaskUpdates(task *models.Task, req models.UpdateTaskRequest) error {
	if req.Name != nil {
		if err := models.ValidateTaskName(*req.Name); err != nil {
			return err
		}
		task.Name = *req.Name
	}

	if req.Description != nil {
		task.Description = req.Description
	}

	if req.ScriptContent != nil {
		if err := models.ValidateScriptContent(*req.ScriptContent); err != nil {
			return err
		}
		task.ScriptContent = *req.ScriptContent
	}

	if req.ScriptType != nil {
		if err := models.ValidateScriptType(*req.ScriptType); err != nil {
			return err
		}
		task.ScriptType = *req.ScriptType
	}

	if req.Priority != nil {
		if err := models.ValidatePriority(*req.Priority); err != nil {
			return err
		}
		task.Priority = *req.Priority
	}

	if req.TimeoutSeconds != nil {
		if err := models.ValidateTimeout(*req.TimeoutSeconds); err != nil {
			return err
		}
		task.TimeoutSeconds = *req.TimeoutSeconds
	}

	if req.Metadata != nil {
		task.Metadata = req.Metadata
	}

	return nil
}

// parsePagination parses pagination parameters from query string
func (h *TaskHandler) parsePagination(c *gin.Context) (limit, offset int, err error) {
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

// parseCursorPagination parses cursor pagination parameters from query string
func (h *TaskHandler) parseCursorPagination(c *gin.Context) (database.CursorPaginationRequest, bool, error) {
	cursor := c.Query("cursor")
	limitStr := c.Query("limit")
	sortOrder := c.Query("sort_order")
	sortField := c.Query("sort_field")

	// Only use cursor pagination if a cursor is actually provided
	if cursor == "" {
		return database.CursorPaginationRequest{}, false, nil
	}

	req := database.CursorPaginationRequest{
		Limit:     20,           // default
		SortOrder: "desc",       // default
		SortField: "created_at", // default
	}

	// Parse limit
	if limitStr != "" {
		limit, err := strconv.Atoi(limitStr)
		if err != nil {
			return req, false, fmt.Errorf("invalid limit parameter: %w", err)
		}
		req.Limit = limit
	}

	// Parse cursor (already validated to be non-empty above)
	req.Cursor = &cursor

	// Parse sort order
	if sortOrder != "" {
		req.SortOrder = sortOrder
	}

	// Parse sort field
	if sortField != "" {
		// Validate sort field
		validSortFields := map[string]bool{
			"created_at": true,
			"updated_at": true,
			"priority":   true,
			"name":       true,
		}
		if !validSortFields[sortField] {
			return req, false, fmt.Errorf("invalid sort_field parameter: must be one of created_at, updated_at, priority, name")
		}
		req.SortField = sortField
	}

	// Validate the request
	database.ValidatePaginationRequest(&req)

	return req, true, nil
}

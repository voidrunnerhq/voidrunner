package handlers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/voidrunnerhq/voidrunner/internal/api/middleware"
	"github.com/voidrunnerhq/voidrunner/internal/database"
	"github.com/voidrunnerhq/voidrunner/internal/logging"
)

// LogHandler handles log-related API endpoints
type LogHandler struct {
	taskRepo         database.TaskRepository
	executionRepo    database.TaskExecutionRepository
	streamingService logging.StreamingService
	logStorage       logging.LogStorage
	logger           *slog.Logger
}

// NewLogHandler creates a new log handler
func NewLogHandler(
	taskRepo database.TaskRepository,
	executionRepo database.TaskExecutionRepository,
	streamingService logging.StreamingService,
	logStorage logging.LogStorage,
	logger *slog.Logger,
) *LogHandler {
	return &LogHandler{
		taskRepo:         taskRepo,
		executionRepo:    executionRepo,
		streamingService: streamingService,
		logStorage:       logStorage,
		logger:           logger.With("component", "log_handler"),
	}
}

// StreamTaskLogs handles Server-Sent Events streaming of task logs
//
//	@Summary		Stream task logs in real-time
//	@Description	Streams task execution logs via Server-Sent Events (SSE)
//	@Tags			Logs
//	@Accept			json
//	@Produce		text/event-stream
//	@Security		BearerAuth
//	@Param			task_id	path	string	true	"Task ID"
//	@Param			execution_id	query	string	false	"Execution ID (optional, streams all executions if not provided)"
//	@Success		200	{string}	string	"SSE stream"
//	@Failure		400	{object}	models.ErrorResponse	"Invalid task ID or parameters"
//	@Failure		401	{object}	models.ErrorResponse	"Unauthorized"
//	@Failure		403	{object}	models.ErrorResponse	"Forbidden"
//	@Failure		404	{object}	models.ErrorResponse	"Task not found"
//	@Router			/tasks/{task_id}/logs/stream [get]
func (h *LogHandler) StreamTaskLogs(c *gin.Context) {
	taskIDStr := c.Param("id")
	taskID, err := uuid.Parse(taskIDStr)
	if err != nil {
		h.logger.Warn("invalid task ID for log streaming", "task_id", taskIDStr)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid task ID format",
		})
		return
	}

	// Get user from context
	user := middleware.GetUserFromContext(c)
	if user == nil {
		h.logger.Error("user not found in context for log streaming")
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Unauthorized",
		})
		return
	}

	// Verify task ownership
	task, err := h.taskRepo.GetByID(c.Request.Context(), taskID)
	if err != nil {
		if err == database.ErrTaskNotFound {
			h.logger.Warn("task not found for log streaming", "task_id", taskID, "user_id", user.ID)
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Task not found",
			})
			return
		}
		h.logger.Error("failed to get task for log streaming", "error", err, "task_id", taskID)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve task",
		})
		return
	}

	// Check if user owns the task
	if task.UserID != user.ID {
		h.logger.Warn("user attempted to stream logs for another user's task",
			"user_id", user.ID, "task_id", taskID, "task_owner_id", task.UserID)
		c.JSON(http.StatusForbidden, gin.H{
			"error": "Access denied",
		})
		return
	}

	// Check if streaming service is available
	if h.streamingService == nil {
		h.logger.Warn("streaming service not available", "task_id", taskID)
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Log streaming service is not available",
		})
		return
	}

	// Set SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("Access-Control-Allow-Headers", "Cache-Control")

	// Subscribe to log stream
	logChan, err := h.streamingService.Subscribe(c.Request.Context(), taskID, user.ID)
	if err != nil {
		h.logger.Error("failed to subscribe to log stream", "error", err, "task_id", taskID, "user_id", user.ID)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to start log streaming",
		})
		return
	}

	h.logger.Info("started log streaming", "task_id", taskID, "user_id", user.ID)

	// Clean up subscription when connection closes
	defer func() {
		if err := h.streamingService.Unsubscribe(taskID, logChan); err != nil {
			h.logger.Error("failed to unsubscribe from log stream", "error", err, "task_id", taskID)
		}
		h.logger.Info("stopped log streaming", "task_id", taskID, "user_id", user.ID)
	}()

	// Send initial connection message
	initialMsg := gin.H{
		"event":   "connected",
		"task_id": taskID,
		"message": "Log streaming started",
	}
	h.sendSSEMessage(c, "connected", initialMsg, "")

	// Stream logs
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		h.logger.Error("streaming not supported", "task_id", taskID)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Streaming not supported",
		})
		return
	}

	// Keep connection alive and stream logs
	ticker := time.NewTicker(30 * time.Second) // Send keepalive every 30 seconds
	defer ticker.Stop()

	for {
		select {
		case logEntry, ok := <-logChan:
			if !ok {
				// Channel was closed
				h.sendSSEMessage(c, "closed", gin.H{"message": "Log stream closed"}, "")
				flusher.Flush()
				return
			}

			// Send log entry
			h.sendSSEMessage(c, "log", logEntry, fmt.Sprintf("%d", logEntry.SequenceNumber))
			flusher.Flush()

		case <-ticker.C:
			// Send keepalive
			h.sendSSEMessage(c, "keepalive", gin.H{"timestamp": time.Now()}, "")
			flusher.Flush()

		case <-c.Request.Context().Done():
			// Client disconnected
			h.logger.Debug("log streaming client disconnected", "task_id", taskID, "user_id", user.ID)
			return
		}
	}
}

// GetTaskLogs handles retrieving historical task logs
//
//	@Summary		Get task logs
//	@Description	Retrieves historical logs for a task with filtering and pagination
//	@Tags			Logs
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			task_id	path	string	true	"Task ID"
//	@Param			execution_id	query	string	false	"Execution ID filter"
//	@Param			stream	query	string	false	"Stream filter (stdout, stderr)"
//	@Param			start_time	query	string	false	"Start time filter (ISO 8601)"
//	@Param			end_time	query	string	false	"End time filter (ISO 8601)"
//	@Param			search	query	string	false	"Full-text search query"
//	@Param			limit	query	int	false	"Limit (default: 100, max: 1000)"
//	@Param			offset	query	int	false	"Offset (default: 0)"
//	@Success		200	{object}	logging.LogResponse	"Log entries retrieved successfully"
//	@Failure		400	{object}	models.ErrorResponse	"Invalid parameters"
//	@Failure		401	{object}	models.ErrorResponse	"Unauthorized"
//	@Failure		403	{object}	models.ErrorResponse	"Forbidden"
//	@Failure		404	{object}	models.ErrorResponse	"Task not found"
//	@Router			/tasks/{task_id}/logs [get]
func (h *LogHandler) GetTaskLogs(c *gin.Context) {
	taskIDStr := c.Param("id")
	taskID, err := uuid.Parse(taskIDStr)
	if err != nil {
		h.logger.Warn("invalid task ID for log retrieval", "task_id", taskIDStr)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid task ID format",
		})
		return
	}

	// Get user from context
	user := middleware.GetUserFromContext(c)
	if user == nil {
		h.logger.Error("user not found in context for log retrieval")
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Unauthorized",
		})
		return
	}

	// Verify task ownership
	task, err := h.taskRepo.GetByID(c.Request.Context(), taskID)
	if err != nil {
		if err == database.ErrTaskNotFound {
			h.logger.Warn("task not found for log retrieval", "task_id", taskID, "user_id", user.ID)
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Task not found",
			})
			return
		}
		h.logger.Error("failed to get task for log retrieval", "error", err, "task_id", taskID)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve task",
		})
		return
	}

	// Check if user owns the task
	if task.UserID != user.ID {
		h.logger.Warn("user attempted to access logs for another user's task",
			"user_id", user.ID, "task_id", taskID, "task_owner_id", task.UserID)
		c.JSON(http.StatusForbidden, gin.H{
			"error": "Access denied",
		})
		return
	}

	// Check if log storage is available
	if h.logStorage == nil {
		h.logger.Warn("log storage not available", "task_id", taskID)
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Log storage service is not available",
		})
		return
	}

	// Parse filter parameters
	filter, err := h.parseLogFilter(c, taskID)
	if err != nil {
		h.logger.Warn("invalid log filter parameters", "error", err, "task_id", taskID)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	// Handle search query separately if provided
	if filter.SearchQuery != "" {
		logs, err := h.logStorage.SearchLogs(c.Request.Context(), taskID, filter.SearchQuery, filter.Limit)
		if err != nil {
			h.logger.Error("failed to search logs", "error", err, "task_id", taskID)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to search logs",
			})
			return
		}

		response := logging.LogResponse{
			Logs:    logs,
			Total:   int64(len(logs)),
			Limit:   filter.Limit,
			Offset:  0,
			HasMore: len(logs) == filter.Limit,
		}

		h.logger.Debug("log search completed", "task_id", taskID, "query", filter.SearchQuery, "results", len(logs))
		c.JSON(http.StatusOK, response)
		return
	}

	// Get regular filtered logs
	logs, err := h.logStorage.GetLogs(c.Request.Context(), filter)
	if err != nil {
		h.logger.Error("failed to get logs", "error", err, "task_id", taskID)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve logs",
		})
		return
	}

	// Get total count for pagination
	total, err := h.logStorage.GetLogCount(c.Request.Context(), taskID, filter.ExecutionID)
	if err != nil {
		h.logger.Error("failed to count logs", "error", err, "task_id", taskID)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to count logs",
		})
		return
	}

	response := logging.LogResponse{
		Logs:    logs,
		Total:   total,
		Limit:   filter.Limit,
		Offset:  filter.Offset,
		HasMore: int64(filter.Offset+len(logs)) < total,
	}

	h.logger.Debug("log retrieval completed", "task_id", taskID, "user_id", user.ID, "count", len(logs))
	c.JSON(http.StatusOK, response)
}

// sendSSEMessage sends a Server-Sent Events message
func (h *LogHandler) sendSSEMessage(c *gin.Context, event string, data interface{}, id string) {
	if id != "" {
		_, _ = fmt.Fprintf(c.Writer, "id: %s\n", id)
	}

	if event != "" {
		_, _ = fmt.Fprintf(c.Writer, "event: %s\n", event)
	}

	// Serialize data to JSON
	jsonData, err := json.Marshal(data)
	if err != nil {
		h.logger.Error("failed to marshal SSE data", "error", err)
		return
	}

	_, _ = fmt.Fprintf(c.Writer, "data: %s\n\n", string(jsonData))
}

// parseLogFilter parses log filter parameters from query string
func (h *LogHandler) parseLogFilter(c *gin.Context, taskID uuid.UUID) (logging.LogFilter, error) {
	filter := logging.LogFilter{
		TaskID: taskID,
		Limit:  100, // Default limit
		Offset: 0,   // Default offset
	}

	// Parse execution_id
	if executionIDStr := c.Query("execution_id"); executionIDStr != "" {
		executionID, err := uuid.Parse(executionIDStr)
		if err != nil {
			return filter, fmt.Errorf("invalid execution_id format")
		}
		filter.ExecutionID = &executionID
	}

	// Parse stream filter
	if stream := c.Query("stream"); stream != "" {
		if stream != "stdout" && stream != "stderr" {
			return filter, fmt.Errorf("stream must be 'stdout' or 'stderr'")
		}
		filter.Stream = stream
	}

	// Parse time filters
	if startTimeStr := c.Query("start_time"); startTimeStr != "" {
		startTime, err := time.Parse(time.RFC3339, startTimeStr)
		if err != nil {
			return filter, fmt.Errorf("invalid start_time format, expected ISO 8601")
		}
		filter.StartTime = &startTime
	}

	if endTimeStr := c.Query("end_time"); endTimeStr != "" {
		endTime, err := time.Parse(time.RFC3339, endTimeStr)
		if err != nil {
			return filter, fmt.Errorf("invalid end_time format, expected ISO 8601")
		}
		filter.EndTime = &endTime
	}

	// Parse search query
	filter.SearchQuery = strings.TrimSpace(c.Query("search"))

	// Parse limit
	if limitStr := c.Query("limit"); limitStr != "" {
		limit, err := strconv.Atoi(limitStr)
		if err != nil {
			return filter, fmt.Errorf("invalid limit parameter")
		}
		if limit < 1 || limit > 1000 {
			return filter, fmt.Errorf("limit must be between 1 and 1000")
		}
		filter.Limit = limit
	}

	// Parse offset
	if offsetStr := c.Query("offset"); offsetStr != "" {
		offset, err := strconv.Atoi(offsetStr)
		if err != nil {
			return filter, fmt.Errorf("invalid offset parameter")
		}
		if offset < 0 {
			return filter, fmt.Errorf("offset must be non-negative")
		}
		filter.Offset = offset
	}

	// Validate the complete filter
	if err := filter.Validate(); err != nil {
		return filter, err
	}

	return filter, nil
}

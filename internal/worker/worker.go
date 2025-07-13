package worker

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/voidrunnerhq/voidrunner/internal/database"
	"github.com/voidrunnerhq/voidrunner/internal/executor"
	"github.com/voidrunnerhq/voidrunner/internal/models"
	"github.com/voidrunnerhq/voidrunner/internal/queue"
)

// BaseWorker implements the Worker interface for background task processing
type BaseWorker struct {
	// Core components
	id          string
	queue       queue.TaskQueue
	executor    executor.TaskExecutor
	repos       *database.Repositories
	concurrency ConcurrencyManager
	config      WorkerConfig
	logger      *slog.Logger

	// State management
	mu         sync.RWMutex
	isRunning  bool
	isHealthy  bool
	ctx        context.Context
	cancel     context.CancelFunc
	shutdownCh chan struct{}

	// Statistics
	stats       WorkerStats
	statsMu     sync.RWMutex
	startedAt   time.Time
	currentTask *uuid.UUID

	// Health tracking
	lastHeartbeat   time.Time
	heartbeatTicker *time.Ticker
}

// NewWorker creates a new worker instance
func NewWorker(
	queue queue.TaskQueue,
	executor executor.TaskExecutor,
	repos *database.Repositories,
	concurrency ConcurrencyManager,
	config WorkerConfig,
	logger *slog.Logger,
) Worker {
	workerID := fmt.Sprintf("%s-%s", config.WorkerIDPrefix, uuid.New().String()[:8])

	return &BaseWorker{
		id:          workerID,
		queue:       queue,
		executor:    executor,
		repos:       repos,
		concurrency: concurrency,
		config:      config,
		logger:      logger.With("worker_id", workerID),
		shutdownCh:  make(chan struct{}),
		isHealthy:   true,
		stats: WorkerStats{
			WorkerID:  workerID,
			IsRunning: false,
			IsHealthy: true,
		},
	}
}

// Start begins the worker's processing loop
func (w *BaseWorker) Start(ctx context.Context) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.isRunning {
		return ErrWorkerAlreadyRunning
	}

	w.logger.Info("starting worker")

	// Create worker context
	w.ctx, w.cancel = context.WithCancel(ctx)
	w.isRunning = true
	w.startedAt = time.Now()
	w.lastHeartbeat = time.Now()

	// Update stats
	w.updateStats(func(stats *WorkerStats) {
		stats.IsRunning = true
		stats.StartedAt = w.startedAt
		stats.LastHeartbeat = w.lastHeartbeat
	})

	// Start heartbeat ticker
	w.heartbeatTicker = time.NewTicker(w.config.HeartbeatInterval)

	// Start processing goroutines
	go w.processingLoop()
	go w.heartbeatLoop()
	go w.healthCheckLoop()

	w.logger.Info("worker started successfully")
	return nil
}

// Stop gracefully stops the worker
func (w *BaseWorker) Stop(ctx context.Context) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.isRunning {
		return ErrWorkerNotRunning
	}

	w.logger.Info("stopping worker")

	// Signal shutdown
	w.cancel()
	w.isRunning = false

	// Stop heartbeat
	if w.heartbeatTicker != nil {
		w.heartbeatTicker.Stop()
	}

	// Wait for graceful shutdown or timeout
	select {
	case <-w.shutdownCh:
		w.logger.Info("worker stopped gracefully")
	case <-time.After(w.config.ShutdownTimeout):
		w.logger.Warn("worker shutdown timeout reached")
	case <-ctx.Done():
		w.logger.Warn("worker shutdown cancelled by context")
	}

	// Update stats
	w.updateStats(func(stats *WorkerStats) {
		stats.IsRunning = false
	})

	return nil
}

// IsRunning returns true if the worker is currently running
func (w *BaseWorker) IsRunning() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.isRunning
}

// GetID returns the unique worker ID
func (w *BaseWorker) GetID() string {
	return w.id
}

// GetStats returns worker statistics
func (w *BaseWorker) GetStats() WorkerStats {
	w.statsMu.RLock()
	defer w.statsMu.RUnlock()

	stats := w.stats

	// Calculate derived statistics
	if w.stats.TasksProcessed > 0 {
		stats.AverageTaskTime = w.stats.TotalProcessingTime / time.Duration(w.stats.TasksProcessed)
	}

	return stats
}

// IsHealthy checks if the worker is healthy
func (w *BaseWorker) IsHealthy() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.isHealthy
}

// processingLoop is the main worker loop that processes tasks
func (w *BaseWorker) processingLoop() {
	defer func() {
		w.shutdownCh <- struct{}{}
	}()

	w.logger.Info("starting processing loop")

	for {
		select {
		case <-w.ctx.Done():
			w.logger.Info("processing loop stopped by context")
			return
		default:
			if err := w.processNextTask(); err != nil {
				w.logger.Error("error processing task", "error", err)

				// Check if error should cause unhealthy state
				if !IsRetryableWorkerError(err) {
					w.setUnhealthy("critical processing error")
				}

				// Brief pause before retrying
				select {
				case <-time.After(time.Second):
				case <-w.ctx.Done():
					return
				}
			}
		}
	}
}

// processNextTask dequeues and processes a single task
func (w *BaseWorker) processNextTask() error {
	// Dequeue message
	messages, err := w.queue.Dequeue(w.ctx, 1)
	if err != nil {
		return NewWorkerError(w.id, "dequeue", err, true)
	}

	if len(messages) == 0 {
		// No messages available, wait a moment
		select {
		case <-time.After(time.Second):
		case <-w.ctx.Done():
		}
		return nil
	}

	message := messages[0]
	w.logger.Info("processing task", "task_id", message.TaskID, "user_id", message.UserID)

	return w.processTask(message)
}

// processTask processes a single task message
func (w *BaseWorker) processTask(message *queue.TaskMessage) error {
	startTime := time.Now()

	// Update current task
	w.setCurrentTask(&message.TaskID)
	defer w.setCurrentTask(nil)

	// Acquire processing slot
	slot, err := w.concurrency.AcquireSlot(w.ctx, message.UserID)
	if err != nil {
		// If concurrency limit reached, put message back and wait
		if err == ErrConcurrencyLimitReached {
			w.logger.Debug("concurrency limit reached, waiting")
			select {
			case <-time.After(5 * time.Second):
			case <-w.ctx.Done():
			}
			return nil
		}
		return NewWorkerError(w.id, "acquire_slot", err, true)
	}
	defer func() {
		if err := w.concurrency.ReleaseSlot(slot); err != nil {
			w.logger.Error("failed to release concurrency slot", "slot_id", slot.ID, "error", err)
		}
	}()

	// Get task from database
	task, err := w.repos.Tasks.GetByID(w.ctx, message.TaskID)
	if err != nil {
		w.deleteMessage(message)
		return NewWorkerError(w.id, "get_task", err, false)
	}

	// Create task execution record
	execution, err := w.createExecution(task)
	if err != nil {
		return NewWorkerError(w.id, "create_execution", err, true)
	}

	// Update task status to running
	if err := w.updateTaskStatus(task.ID, models.TaskStatusRunning); err != nil {
		return NewWorkerError(w.id, "update_task_status", err, true)
	}

	// Execute task
	result, execErr := w.executeTask(task, execution)

	// Process execution result
	if err := w.processExecutionResult(task, execution, result, execErr, message); err != nil {
		w.logger.Error("failed to process execution result", "error", err)
		// Don't return error here as the task was executed
	}

	// Update statistics
	duration := time.Since(startTime)
	w.updateTaskStats(duration, execErr == nil)

	// Delete message from queue
	w.deleteMessage(message)

	return nil
}

// executeTask executes the task using the executor
func (w *BaseWorker) executeTask(task *models.Task, execution *models.TaskExecution) (*executor.ExecutionResult, error) {
	// Create execution context
	execCtx := &executor.ExecutionContext{
		Task:      task,
		Execution: execution,
		Context:   w.ctx,
		Timeout:   time.Duration(task.TimeoutSeconds) * time.Second,
		ResourceLimits: executor.ResourceLimits{
			MemoryLimitBytes: 512 * 1024 * 1024, // 512MB default
			CPUQuota:         100000,            // 1 CPU core
			PidsLimit:        128,               // Max processes
			TimeoutSeconds:   int(task.TimeoutSeconds),
		},
	}

	w.logger.Info("executing task", "task_id", task.ID, "script_type", task.ScriptType)

	result, err := w.executor.Execute(w.ctx, execCtx)
	if err != nil {
		w.logger.Error("task execution failed", "task_id", task.ID, "error", err)
		return nil, err
	}

	w.logger.Info("task execution completed",
		"task_id", task.ID,
		"status", result.Status,
		"execution_time_ms", result.ExecutionTimeMs)

	return result, nil
}

// createExecution creates a new task execution record
func (w *BaseWorker) createExecution(task *models.Task) (*models.TaskExecution, error) {
	execution := &models.TaskExecution{
		ID:        models.NewID(),
		TaskID:    task.ID,
		Status:    models.ExecutionStatusPending,
		StartedAt: new(time.Time),
	}
	*execution.StartedAt = time.Now()

	if err := w.repos.TaskExecutions.Create(w.ctx, execution); err != nil {
		return nil, fmt.Errorf("failed to create execution: %w", err)
	}

	return execution, nil
}

// processExecutionResult processes the result of task execution
func (w *BaseWorker) processExecutionResult(
	task *models.Task,
	execution *models.TaskExecution,
	result *executor.ExecutionResult,
	execErr error,
	message *queue.TaskMessage,
) error {
	if execErr != nil {
		// Execution failed
		return w.handleExecutionFailure(task, execution, execErr, message)
	}

	// Execution succeeded, update records
	return w.handleExecutionSuccess(task, execution, result)
}

// handleExecutionSuccess handles successful task execution
func (w *BaseWorker) handleExecutionSuccess(
	task *models.Task,
	execution *models.TaskExecution,
	result *executor.ExecutionResult,
) error {
	// Update execution record
	now := time.Now()
	execution.Status = result.Status
	execution.ReturnCode = result.ReturnCode
	execution.Stdout = result.Stdout
	execution.Stderr = result.Stderr
	execution.ExecutionTimeMs = result.ExecutionTimeMs
	execution.MemoryUsageBytes = result.MemoryUsageBytes
	execution.CompletedAt = &now

	if err := w.repos.TaskExecutions.Update(w.ctx, execution); err != nil {
		return fmt.Errorf("failed to update execution: %w", err)
	}

	// Update task status
	var taskStatus models.TaskStatus
	switch result.Status {
	case models.ExecutionStatusCompleted:
		taskStatus = models.TaskStatusCompleted
	case models.ExecutionStatusFailed:
		taskStatus = models.TaskStatusFailed
	case models.ExecutionStatusTimeout:
		taskStatus = models.TaskStatusTimeout
	default:
		taskStatus = models.TaskStatusFailed
	}

	return w.updateTaskStatus(task.ID, taskStatus)
}

// handleExecutionFailure handles failed task execution
func (w *BaseWorker) handleExecutionFailure(
	task *models.Task,
	execution *models.TaskExecution,
	execErr error,
	message *queue.TaskMessage,
) error {
	// Update execution record with failure
	now := time.Now()
	execution.Status = models.ExecutionStatusFailed
	execution.CompletedAt = &now
	stderr := execErr.Error()
	execution.Stderr = &stderr

	if err := w.repos.TaskExecutions.Update(w.ctx, execution); err != nil {
		w.logger.Error("failed to update failed execution", "error", err)
	}

	// Update task status to failed
	if err := w.updateTaskStatus(task.ID, models.TaskStatusFailed); err != nil {
		w.logger.Error("failed to update task status to failed", "error", err)
	}

	// Handle retry logic if needed
	if message.Attempts < w.config.MaxRetryAttempts {
		w.logger.Info("task will be retried",
			"task_id", task.ID,
			"attempt", message.Attempts,
			"max_attempts", w.config.MaxRetryAttempts)
		// Task will be retried by retry processor
	}

	return nil
}

// updateTaskStatus updates the task status
func (w *BaseWorker) updateTaskStatus(taskID uuid.UUID, status models.TaskStatus) error {
	if err := w.repos.Tasks.UpdateStatus(w.ctx, taskID, status); err != nil {
		return fmt.Errorf("failed to update task status: %w", err)
	}
	return nil
}

// deleteMessage deletes a processed message from the queue
func (w *BaseWorker) deleteMessage(message *queue.TaskMessage) {
	if message.ReceiptHandle != nil {
		if err := w.queue.DeleteMessage(w.ctx, *message.ReceiptHandle); err != nil {
			w.logger.Error("failed to delete message from queue", "error", err)
		}
	}
}

// setCurrentTask updates the current task being processed
func (w *BaseWorker) setCurrentTask(taskID *uuid.UUID) {
	w.statsMu.Lock()
	defer w.statsMu.Unlock()

	w.currentTask = taskID
	if taskID != nil {
		now := time.Now()
		w.stats.CurrentTask = taskID
		w.stats.LastTaskStarted = &now
	} else {
		w.stats.CurrentTask = nil
		if w.stats.LastTaskStarted != nil {
			now := time.Now()
			w.stats.LastTaskCompleted = &now
		}
	}
}

// updateTaskStats updates task processing statistics
func (w *BaseWorker) updateTaskStats(duration time.Duration, success bool) {
	w.statsMu.Lock()
	defer w.statsMu.Unlock()

	w.stats.TasksProcessed++
	w.stats.TotalProcessingTime += duration

	if success {
		w.stats.TasksSuccessful++
	} else {
		w.stats.TasksFailed++
	}
}

// updateStats safely updates worker statistics
func (w *BaseWorker) updateStats(fn func(*WorkerStats)) {
	w.statsMu.Lock()
	defer w.statsMu.Unlock()
	fn(&w.stats)
}

// heartbeatLoop maintains worker heartbeat
func (w *BaseWorker) heartbeatLoop() {
	for {
		select {
		case <-w.ctx.Done():
			return
		case <-w.heartbeatTicker.C:
			w.updateHeartbeat()
		}
	}
}

// updateHeartbeat updates the last heartbeat time
func (w *BaseWorker) updateHeartbeat() {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.lastHeartbeat = time.Now()

	w.updateStats(func(stats *WorkerStats) {
		stats.LastHeartbeat = w.lastHeartbeat
	})
}

// healthCheckLoop periodically checks worker health
func (w *BaseWorker) healthCheckLoop() {
	ticker := time.NewTicker(w.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-w.ctx.Done():
			return
		case <-ticker.C:
			w.performHealthCheck()
		}
	}
}

// performHealthCheck checks worker health status
func (w *BaseWorker) performHealthCheck() {
	// Check if executor is healthy
	if err := w.executor.IsHealthy(w.ctx); err != nil {
		w.setUnhealthy(fmt.Sprintf("executor unhealthy: %v", err))
		return
	}

	// Check if queue is healthy
	if err := w.queue.IsHealthy(w.ctx); err != nil {
		w.setUnhealthy(fmt.Sprintf("queue unhealthy: %v", err))
		return
	}

	// Check for stale tasks (optional)
	if w.currentTask != nil && w.stats.LastTaskStarted != nil {
		if time.Since(*w.stats.LastTaskStarted) > w.config.StaleTaskThreshold {
			w.logger.Warn("detected stale task",
				"task_id", *w.currentTask,
				"duration", time.Since(*w.stats.LastTaskStarted))
		}
	}

	// Mark as healthy if all checks pass
	w.setHealthy()
}

// setHealthy marks the worker as healthy
func (w *BaseWorker) setHealthy() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.isHealthy {
		w.logger.Info("worker is now healthy")
		w.isHealthy = true
		w.updateStats(func(stats *WorkerStats) {
			stats.IsHealthy = true
		})
	}
}

// setUnhealthy marks the worker as unhealthy
func (w *BaseWorker) setUnhealthy(reason string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.isHealthy {
		w.logger.Warn("worker is now unhealthy", "reason", reason)
		w.isHealthy = false
		w.updateStats(func(stats *WorkerStats) {
			stats.IsHealthy = false
		})
	}
}

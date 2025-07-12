package services

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/voidrunnerhq/voidrunner/internal/database"
	"github.com/voidrunnerhq/voidrunner/internal/executor"
	"github.com/voidrunnerhq/voidrunner/internal/models"
)

// TaskExecutorService integrates the task execution engine with the executor
type TaskExecutorService struct {
	taskExecutionService *TaskExecutionService
	taskRepo             database.TaskRepository
	executor             executor.TaskExecutor
	cleanupManager       *executor.CleanupManager
	logger               *slog.Logger
}

// NewTaskExecutorService creates a new task executor service
func NewTaskExecutorService(
	taskExecutionService *TaskExecutionService,
	taskRepo database.TaskRepository,
	executor executor.TaskExecutor,
	cleanupManager *executor.CleanupManager,
	logger *slog.Logger,
) *TaskExecutorService {
	return &TaskExecutorService{
		taskExecutionService: taskExecutionService,
		taskRepo:             taskRepo,
		executor:             executor,
		cleanupManager:       cleanupManager,
		logger:               logger,
	}
}

// ExecuteTask executes a task using the container executor
func (s *TaskExecutorService) ExecuteTask(ctx context.Context, taskID uuid.UUID, userID uuid.UUID) (*models.TaskExecution, error) {
	logger := s.logger.With(
		"task_id", taskID.String(),
		"user_id", userID.String(),
		"operation", "execute_task",
	)

	logger.Info("starting task execution")

	// First, create the execution record and update task status
	execution, err := s.taskExecutionService.CreateExecutionAndUpdateTaskStatus(ctx, taskID, userID)
	if err != nil {
		logger.Error("failed to create execution record", "error", err)
		return nil, fmt.Errorf("failed to create execution record: %w", err)
	}

	// Get the task details for execution
	task, err := s.getTaskByID(ctx, taskID)
	if err != nil {
		// Rollback the execution if we can't get task details
		if rollbackErr := s.rollbackExecution(ctx, execution.ID, userID); rollbackErr != nil {
			logger.Error("failed to rollback execution after task fetch error", "rollback_error", rollbackErr)
		}
		return nil, fmt.Errorf("failed to get task details: %w", err)
	}

	// Start background execution
	go s.executeTaskAsync(ctx, task, execution, userID)

	logger.Info("task execution started asynchronously", "execution_id", execution.ID.String())
	return execution, nil
}

// executeTaskAsync performs the actual task execution in the background
func (s *TaskExecutorService) executeTaskAsync(ctx context.Context, task *models.Task, execution *models.TaskExecution, userID uuid.UUID) {
	logger := s.logger.With(
		"task_id", task.ID.String(),
		"execution_id", execution.ID.String(),
		"user_id", userID.String(),
		"operation", "execute_task_async",
	)

	logger.Info("starting background task execution")

	// Note: Container registration will happen in the executor after container creation

	// Mark execution as running
	if err := s.updateExecutionStatus(ctx, execution.ID, models.ExecutionStatusRunning, userID); err != nil {
		logger.Error("failed to mark execution as running", "error", err)
		// Continue with execution anyway
	}

	// Build execution context
	execCtx := &executor.ExecutionContext{
		Task:      task,
		Execution: execution,
		Context:   ctx,
		Timeout:   time.Duration(task.TimeoutSeconds) * time.Second,
		ResourceLimits: executor.ResourceLimits{
			MemoryLimitBytes: 128 * 1024 * 1024, // 128MB default
			CPUQuota:         50000,             // 0.5 CPU core
			PidsLimit:        128,               // Max 128 processes
			TimeoutSeconds:   task.TimeoutSeconds,
		},
	}

	// Execute the task
	result, err := s.executor.Execute(ctx, execCtx)
	if err != nil {
		logger.Error("task execution failed", "error", err)
		if result == nil {
			result = &executor.ExecutionResult{
				Status: models.ExecutionStatusFailed,
				Stderr: stringPtr(fmt.Sprintf("Execution error: %s", err.Error())),
			}
		}
	}

	// Update execution with results
	if err := s.updateExecutionWithResults(ctx, execution.ID, result, userID); err != nil {
		logger.Error("failed to update execution with results", "error", err)
	}

	// Cleanup resources (skip if cleanup manager not available, e.g., for mock executor)
	if s.cleanupManager != nil {
		if err := s.cleanupManager.CleanupExecution(ctx, execution.ID); err != nil {
			logger.Error("failed to cleanup execution resources", "error", err)
		}
	}

	logger.Info("background task execution completed",
		"status", result.Status,
		"duration_ms", result.ExecutionTimeMs)
}

// updateExecutionStatus updates the execution status
func (s *TaskExecutorService) updateExecutionStatus(ctx context.Context, executionID uuid.UUID, status models.ExecutionStatus, userID uuid.UUID) error {
	// Create repositories from the connection
	repos := database.NewRepositories(s.taskExecutionService.conn)
	return repos.TaskExecutions.UpdateStatus(ctx, executionID, status)
}

// updateExecutionWithResults updates the execution with the execution results
func (s *TaskExecutorService) updateExecutionWithResults(ctx context.Context, executionID uuid.UUID, result *executor.ExecutionResult, userID uuid.UUID) error {
	// Convert executor result to task execution model
	execution := &models.TaskExecution{
		ID:               executionID,
		Status:           result.Status,
		ReturnCode:       result.ReturnCode,
		Stdout:           result.Stdout,
		Stderr:           result.Stderr,
		ExecutionTimeMs:  result.ExecutionTimeMs,
		MemoryUsageBytes: result.MemoryUsageBytes,
		StartedAt:        result.StartedAt,
		CompletedAt:      result.CompletedAt,
	}

	// Determine task status based on execution status
	var taskStatus models.TaskStatus
	switch result.Status {
	case models.ExecutionStatusCompleted:
		taskStatus = models.TaskStatusCompleted
	case models.ExecutionStatusFailed:
		taskStatus = models.TaskStatusFailed
	case models.ExecutionStatusTimeout:
		taskStatus = models.TaskStatusTimeout
	case models.ExecutionStatusCancelled:
		taskStatus = models.TaskStatusCancelled
	default:
		taskStatus = models.TaskStatusFailed // Fallback
	}

	// Use the existing service method to update both execution and task status atomically
	return s.taskExecutionService.CompleteExecutionAndFinalizeTaskStatus(ctx, execution, taskStatus, userID)
}

// rollbackExecution rolls back an execution when something goes wrong during setup
func (s *TaskExecutorService) rollbackExecution(ctx context.Context, executionID uuid.UUID, userID uuid.UUID) error {
	return s.taskExecutionService.CancelExecutionAndResetTaskStatus(ctx, executionID, userID)
}

// CancelTaskExecution cancels a running task execution
func (s *TaskExecutorService) CancelTaskExecution(ctx context.Context, executionID uuid.UUID, userID uuid.UUID) error {
	logger := s.logger.With(
		"execution_id", executionID.String(),
		"user_id", userID.String(),
		"operation", "cancel_execution",
	)

	logger.Info("cancelling task execution")

	// Cancel in the executor
	if err := s.executor.Cancel(ctx, executionID); err != nil {
		logger.Warn("executor cancellation failed", "error", err)
		// Continue with database cancellation anyway
	}

	// Cleanup resources (skip if cleanup manager not available, e.g., for mock executor)
	if s.cleanupManager != nil {
		if err := s.cleanupManager.CleanupExecution(ctx, executionID); err != nil {
			logger.Warn("cleanup failed during cancellation", "error", err)
		}
	}

	// Cancel in the database
	if err := s.taskExecutionService.CancelExecutionAndResetTaskStatus(ctx, executionID, userID); err != nil {
		logger.Error("failed to cancel execution in database", "error", err)
		return fmt.Errorf("failed to cancel execution: %w", err)
	}

	logger.Info("task execution cancelled successfully")
	return nil
}

// GetExecutorHealth checks if the executor is healthy
func (s *TaskExecutorService) GetExecutorHealth(ctx context.Context) error {
	return s.executor.IsHealthy(ctx)
}

// GetExecutionStats returns statistics about running executions
func (s *TaskExecutorService) GetExecutionStats() executor.CleanupStats {
	if s.cleanupManager != nil {
		return s.cleanupManager.GetStats()
	}
	// Return empty stats for mock executor
	return executor.CleanupStats{}
}

// CleanupStaleExecutions cleans up executions that have been running too long
func (s *TaskExecutorService) CleanupStaleExecutions(ctx context.Context, maxAge time.Duration) error {
	logger := s.logger.With("operation", "cleanup_stale_executions", "max_age", maxAge.String())

	logger.Info("starting stale execution cleanup")

	// Skip cleanup if cleanup manager not available (e.g., for mock executor)
	if s.cleanupManager != nil {
		if err := s.cleanupManager.CleanupStaleContainers(ctx, maxAge); err != nil {
			logger.Error("stale execution cleanup failed", "error", err)
			return fmt.Errorf("failed to cleanup stale executions: %w", err)
		}
	} else {
		logger.Debug("cleanup manager not available, skipping stale container cleanup")
	}

	logger.Info("stale execution cleanup completed")
	return nil
}

// Shutdown gracefully shuts down the executor service
func (s *TaskExecutorService) Shutdown(ctx context.Context) error {
	logger := s.logger.With("operation", "shutdown")

	logger.Info("shutting down task executor service")

	// Cleanup all remaining resources (skip if cleanup manager not available)
	if s.cleanupManager != nil {
		if err := s.cleanupManager.Stop(ctx); err != nil {
			logger.Error("cleanup manager shutdown failed", "error", err)
		}
	}

	// Shutdown executor
	if err := s.executor.Cleanup(ctx); err != nil {
		logger.Error("executor cleanup failed", "error", err)
		return fmt.Errorf("failed to cleanup executor: %w", err)
	}

	logger.Info("task executor service shutdown completed")
	return nil
}

// getTaskByID is a helper to get task details
func (s *TaskExecutorService) getTaskByID(ctx context.Context, taskID uuid.UUID) (*models.Task, error) {
	task, err := s.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		if err == database.ErrTaskNotFound {
			return nil, fmt.Errorf("task not found")
		}
		return nil, fmt.Errorf("failed to get task: %w", err)
	}
	return task, nil
}

// Interface implementation methods for TaskExecutionServiceInterface compatibility

// CreateExecutionAndUpdateTaskStatus creates an execution and starts actual task execution
func (s *TaskExecutorService) CreateExecutionAndUpdateTaskStatus(ctx context.Context, taskID uuid.UUID, userID uuid.UUID) (*models.TaskExecution, error) {
	// This method starts actual execution, not just database operations
	return s.ExecuteTask(ctx, taskID, userID)
}

// CancelExecutionAndResetTaskStatus cancels an execution and resets task status
func (s *TaskExecutorService) CancelExecutionAndResetTaskStatus(ctx context.Context, executionID uuid.UUID, userID uuid.UUID) error {
	return s.CancelTaskExecution(ctx, executionID, userID)
}

// CompleteExecutionAndFinalizeTaskStatus is already handled internally by executeTaskAsync
func (s *TaskExecutorService) CompleteExecutionAndFinalizeTaskStatus(ctx context.Context, execution *models.TaskExecution, taskStatus models.TaskStatus, userID uuid.UUID) error {
	// This is already handled internally by the async execution process
	// For external callers, we can delegate to the internal service
	return s.taskExecutionService.CompleteExecutionAndFinalizeTaskStatus(ctx, execution, taskStatus, userID)
}

// Helper function to create string pointer
func stringPtr(s string) *string {
	return &s
}

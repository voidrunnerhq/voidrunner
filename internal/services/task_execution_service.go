package services

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/voidrunnerhq/voidrunner/internal/database"
	"github.com/voidrunnerhq/voidrunner/internal/models"
	"github.com/voidrunnerhq/voidrunner/internal/queue"
)

// TaskExecutionService handles business logic for task execution operations
type TaskExecutionService struct {
	conn         *database.Connection
	queueManager queue.QueueManager
	logger       *slog.Logger
}

// NewTaskExecutionService creates a new task execution service
func NewTaskExecutionService(conn *database.Connection, queueManager queue.QueueManager, logger *slog.Logger) *TaskExecutionService {
	return &TaskExecutionService{
		conn:         conn,
		queueManager: queueManager,
		logger:       logger,
	}
}

// CreateExecutionAndUpdateTaskStatus atomically creates a task execution and enqueues it for processing
func (s *TaskExecutionService) CreateExecutionAndUpdateTaskStatus(ctx context.Context, taskID uuid.UUID, userID uuid.UUID) (*models.TaskExecution, error) {
	var execution *models.TaskExecution
	var task *models.Task

	err := s.conn.WithTransaction(ctx, func(tx database.Transaction) error {
		repos := tx.Repositories()

		// First, verify the task exists and belongs to the user
		var err error
		task, err = repos.Tasks.GetByID(ctx, taskID)
		if err != nil {
			if err == database.ErrTaskNotFound {
				return fmt.Errorf("task not found")
			}
			return fmt.Errorf("failed to get task: %w", err)
		}

		// Check if user owns the task
		if task.UserID != userID {
			return fmt.Errorf("access denied: task does not belong to user")
		}

		// Check if task is already running
		if task.Status == models.TaskStatusRunning {
			return fmt.Errorf("task is already running")
		}

		// Check if task can be executed (not completed, failed, or cancelled)
		if task.Status == models.TaskStatusCompleted ||
			task.Status == models.TaskStatusFailed ||
			task.Status == models.TaskStatusCancelled {
			return fmt.Errorf("cannot execute task with status: %s", task.Status)
		}

		// Create task execution
		execution = &models.TaskExecution{
			ID:     uuid.New(),
			TaskID: taskID,
			Status: models.ExecutionStatusPending,
		}

		if err := repos.TaskExecutions.Create(ctx, execution); err != nil {
			return fmt.Errorf("failed to create task execution: %w", err)
		}

		// Update task status to pending (will be set to running by worker)
		if err := repos.Tasks.UpdateStatus(ctx, taskID, models.TaskStatusPending); err != nil {
			return fmt.Errorf("failed to update task status: %w", err)
		}

		s.logger.Info("task execution created and task status updated atomically",
			"execution_id", execution.ID,
			"task_id", taskID,
			"user_id", userID,
		)

		return nil
	})

	if err != nil {
		s.logger.Error("failed to create execution and update task status",
			"error", err,
			"task_id", taskID,
			"user_id", userID,
		)
		return nil, err
	}

	// After successful database transaction, enqueue the task for processing
	if err := s.enqueueTask(ctx, task, execution); err != nil {
		// If enqueue fails, we should rollback the task status or retry
		s.logger.Error("failed to enqueue task after creating execution",
			"error", err,
			"task_id", taskID,
			"execution_id", execution.ID,
		)

		// Attempt to rollback task status to previous state
		if rollbackErr := s.rollbackTaskStatus(ctx, taskID); rollbackErr != nil {
			s.logger.Error("failed to rollback task status after enqueue failure",
				"rollback_error", rollbackErr,
				"task_id", taskID,
			)
		}

		return nil, fmt.Errorf("failed to enqueue task for execution: %w", err)
	}

	s.logger.Info("task successfully enqueued for execution",
		"task_id", taskID,
		"execution_id", execution.ID,
		"user_id", userID,
	)

	return execution, nil
}

// enqueueTask enqueues a task for processing by workers
func (s *TaskExecutionService) enqueueTask(ctx context.Context, task *models.Task, execution *models.TaskExecution) error {
	// Check queue manager health before enqueuing
	if err := s.queueManager.IsHealthy(ctx); err != nil {
		return fmt.Errorf("queue manager is not healthy: %w", err)
	}

	// Create task message for queue
	message := &queue.TaskMessage{
		TaskID:    task.ID,
		UserID:    task.UserID,
		Priority:  determinePriority(task),
		QueuedAt:  time.Now(),
		Attempts:  0,
		MessageID: fmt.Sprintf("task-%s-exec-%s", task.ID, execution.ID),
		Attributes: map[string]string{
			"execution_id": execution.ID.String(),
			"script_type":  string(task.ScriptType),
			"priority":     fmt.Sprintf("%d", task.Priority),
		},
	}

	// Enqueue to task queue
	if err := s.queueManager.TaskQueue().Enqueue(ctx, message); err != nil {
		return fmt.Errorf("failed to enqueue task message: %w", err)
	}

	return nil
}

// determinePriority determines queue priority from task priority
func determinePriority(task *models.Task) int {
	// Map task priority (1-10) to queue priority constants
	switch task.Priority {
	case 1, 2:
		return queue.PriorityLowest
	case 3, 4:
		return queue.PriorityLow
	case 5, 6:
		return queue.PriorityNormal
	case 7, 8:
		return queue.PriorityHigh
	case 9, 10:
		return queue.PriorityHighest
	default:
		return queue.PriorityNormal
	}
}

// rollbackTaskStatus attempts to rollback task status after enqueue failure
func (s *TaskExecutionService) rollbackTaskStatus(ctx context.Context, taskID uuid.UUID) error {
	return s.conn.WithTransaction(ctx, func(tx database.Transaction) error {
		repos := tx.Repositories()

		// Get current task to determine previous status
		task, err := repos.Tasks.GetByID(ctx, taskID)
		if err != nil {
			return fmt.Errorf("failed to get task for rollback: %w", err)
		}

		// Set back to a reasonable previous state
		previousStatus := models.TaskStatusPending
		if task.Status == models.TaskStatusPending {
			// If it's already pending, we don't need to change it
			return nil
		}

		if err := repos.Tasks.UpdateStatus(ctx, taskID, previousStatus); err != nil {
			return fmt.Errorf("failed to rollback task status: %w", err)
		}

		return nil
	})
}

// UpdateExecutionAndTaskStatus atomically updates both execution and task status
func (s *TaskExecutionService) UpdateExecutionAndTaskStatus(ctx context.Context, executionID uuid.UUID, executionStatus models.ExecutionStatus, taskID uuid.UUID, taskStatus models.TaskStatus, userID uuid.UUID) error {
	err := s.conn.WithTransaction(ctx, func(tx database.Transaction) error {
		repos := tx.Repositories()

		// First, verify the execution exists and belongs to the user's task
		execution, err := repos.TaskExecutions.GetByID(ctx, executionID)
		if err != nil {
			if err == database.ErrExecutionNotFound {
				return fmt.Errorf("execution not found")
			}
			return fmt.Errorf("failed to get execution: %w", err)
		}

		// Verify the task belongs to the user
		task, err := repos.Tasks.GetByID(ctx, execution.TaskID)
		if err != nil {
			return fmt.Errorf("failed to get task: %w", err)
		}

		if task.UserID != userID {
			return fmt.Errorf("access denied: task does not belong to user")
		}

		// Verify the task ID matches
		if execution.TaskID != taskID {
			return fmt.Errorf("execution does not belong to the specified task")
		}

		// Update execution status
		if err := repos.TaskExecutions.UpdateStatus(ctx, executionID, executionStatus); err != nil {
			return fmt.Errorf("failed to update execution status: %w", err)
		}

		// Update task status
		if err := repos.Tasks.UpdateStatus(ctx, taskID, taskStatus); err != nil {
			return fmt.Errorf("failed to update task status: %w", err)
		}

		s.logger.Info("execution and task status updated atomically",
			"execution_id", executionID,
			"execution_status", executionStatus,
			"task_id", taskID,
			"task_status", taskStatus,
			"user_id", userID,
		)

		return nil
	})

	if err != nil {
		s.logger.Error("failed to update execution and task status",
			"error", err,
			"execution_id", executionID,
			"task_id", taskID,
			"user_id", userID,
		)
		return err
	}

	return nil
}

// CancelExecutionAndResetTaskStatus atomically cancels an execution and resets task status
func (s *TaskExecutionService) CancelExecutionAndResetTaskStatus(ctx context.Context, executionID uuid.UUID, userID uuid.UUID) error {
	err := s.conn.WithTransaction(ctx, func(tx database.Transaction) error {
		repos := tx.Repositories()

		// First, verify the execution exists and belongs to the user's task
		execution, err := repos.TaskExecutions.GetByID(ctx, executionID)
		if err != nil {
			if err == database.ErrExecutionNotFound {
				return fmt.Errorf("execution not found")
			}
			return fmt.Errorf("failed to get execution: %w", err)
		}

		// Verify the task belongs to the user
		task, err := repos.Tasks.GetByID(ctx, execution.TaskID)
		if err != nil {
			return fmt.Errorf("failed to get task: %w", err)
		}

		if task.UserID != userID {
			return fmt.Errorf("access denied: task does not belong to user")
		}

		// Check if execution can be cancelled
		if execution.Status == models.ExecutionStatusCompleted ||
			execution.Status == models.ExecutionStatusFailed ||
			execution.Status == models.ExecutionStatusCancelled {
			return fmt.Errorf("cannot cancel execution with status: %s", execution.Status)
		}

		// Update execution status to cancelled
		if err := repos.TaskExecutions.UpdateStatus(ctx, executionID, models.ExecutionStatusCancelled); err != nil {
			return fmt.Errorf("failed to cancel execution: %w", err)
		}

		// Reset task status to pending
		if err := repos.Tasks.UpdateStatus(ctx, execution.TaskID, models.TaskStatusPending); err != nil {
			return fmt.Errorf("failed to reset task status: %w", err)
		}

		s.logger.Info("execution cancelled and task status reset atomically",
			"execution_id", executionID,
			"task_id", execution.TaskID,
			"user_id", userID,
		)

		return nil
	})

	if err != nil {
		s.logger.Error("failed to cancel execution and reset task status",
			"error", err,
			"execution_id", executionID,
			"user_id", userID,
		)
		return err
	}

	return nil
}

// CompleteExecutionAndFinalizeTaskStatus atomically completes an execution with results and finalizes task status
func (s *TaskExecutionService) CompleteExecutionAndFinalizeTaskStatus(ctx context.Context, execution *models.TaskExecution, taskStatus models.TaskStatus, userID uuid.UUID) error {
	err := s.conn.WithTransaction(ctx, func(tx database.Transaction) error {
		repos := tx.Repositories()

		// First, verify the execution exists and belongs to the user's task
		existingExecution, err := repos.TaskExecutions.GetByID(ctx, execution.ID)
		if err != nil {
			if err == database.ErrExecutionNotFound {
				return fmt.Errorf("execution not found")
			}
			return fmt.Errorf("failed to get execution: %w", err)
		}

		// Verify the task belongs to the user
		task, err := repos.Tasks.GetByID(ctx, existingExecution.TaskID)
		if err != nil {
			return fmt.Errorf("failed to get task: %w", err)
		}

		if task.UserID != userID {
			return fmt.Errorf("access denied: task does not belong to user")
		}

		// Check if execution can be completed
		if existingExecution.Status == models.ExecutionStatusCompleted ||
			existingExecution.Status == models.ExecutionStatusCancelled {
			return fmt.Errorf("cannot complete execution with status: %s", existingExecution.Status)
		}

		// Update execution with results
		if err := repos.TaskExecutions.Update(ctx, execution); err != nil {
			return fmt.Errorf("failed to update execution: %w", err)
		}

		// Update task status
		if err := repos.Tasks.UpdateStatus(ctx, existingExecution.TaskID, taskStatus); err != nil {
			return fmt.Errorf("failed to update task status: %w", err)
		}

		s.logger.Info("execution completed and task status finalized atomically",
			"execution_id", execution.ID,
			"execution_status", execution.Status,
			"task_id", existingExecution.TaskID,
			"task_status", taskStatus,
			"user_id", userID,
		)

		return nil
	})

	if err != nil {
		s.logger.Error("failed to complete execution and finalize task status",
			"error", err,
			"execution_id", execution.ID,
			"user_id", userID,
		)
		return err
	}

	return nil
}

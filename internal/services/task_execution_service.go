package services

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/voidrunnerhq/voidrunner/internal/database"
	"github.com/voidrunnerhq/voidrunner/internal/models"
)

// TaskExecutionService handles business logic for task execution operations
type TaskExecutionService struct {
	conn   *database.Connection
	logger *slog.Logger
}

// NewTaskExecutionService creates a new task execution service
func NewTaskExecutionService(conn *database.Connection, logger *slog.Logger) *TaskExecutionService {
	return &TaskExecutionService{
		conn:   conn,
		logger: logger,
	}
}

// CreateExecutionAndUpdateTaskStatus atomically creates a task execution and updates the task status
func (s *TaskExecutionService) CreateExecutionAndUpdateTaskStatus(ctx context.Context, taskID uuid.UUID, userID uuid.UUID) (*models.TaskExecution, error) {
	var execution *models.TaskExecution
	
	err := s.conn.WithTransaction(ctx, func(tx database.Transaction) error {
		repos := tx.Repositories()
		
		// First, verify the task exists and belongs to the user
		task, err := repos.Tasks.GetByID(ctx, taskID)
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
		
		// Update task status to running
		if err := repos.Tasks.UpdateStatus(ctx, taskID, models.TaskStatusRunning); err != nil {
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
	
	return execution, nil
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
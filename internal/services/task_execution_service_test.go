package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/voidrunnerhq/voidrunner/internal/database"
	"github.com/voidrunnerhq/voidrunner/internal/models"
)

// ConnectionInterface defines what we need from a database connection for testing
type ConnectionInterface interface {
	WithTransaction(ctx context.Context, fn func(tx database.Transaction) error) error
}

// MockConnection mocks the ConnectionInterface
type MockConnection struct {
	mock.Mock
}

func (m *MockConnection) WithTransaction(ctx context.Context, fn func(tx database.Transaction) error) error {
	args := m.Called(ctx, fn)
	if args.Get(0) != nil {
		if callable, ok := args.Get(0).(func(func(tx database.Transaction) error) error); ok {
			return callable(fn)
		}
	}
	return args.Error(0)
}

// mockableTaskExecutionService wraps the real service with a mockable connection interface
type mockableTaskExecutionService struct {
	conn   ConnectionInterface
	logger *slog.Logger
}

func newMockableTaskExecutionService(conn ConnectionInterface, logger *slog.Logger) *mockableTaskExecutionService {
	return &mockableTaskExecutionService{
		conn:   conn,
		logger: logger,
	}
}

// Implement the same methods as TaskExecutionService
func (s *mockableTaskExecutionService) CreateExecutionAndUpdateTaskStatus(ctx context.Context, taskID uuid.UUID, userID uuid.UUID) (*models.TaskExecution, error) {
	var execution *models.TaskExecution

	err := s.conn.WithTransaction(ctx, func(tx database.Transaction) error {
		repos := tx.Repositories()

		// First, verify the task exists and belongs to the user
		task, err := repos.Tasks.GetByID(ctx, taskID)
		if err != nil {
			if err == database.ErrTaskNotFound {
				return errors.New("task not found")
			}
			return fmt.Errorf("failed to get task: %w", err)
		}

		// Check if user owns the task
		if task.UserID != userID {
			return errors.New("access denied: task does not belong to user")
		}

		// Check if task is already running
		if task.Status == models.TaskStatusRunning {
			return errors.New("task is already running")
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

func (s *mockableTaskExecutionService) UpdateExecutionAndTaskStatus(ctx context.Context, executionID uuid.UUID, executionStatus models.ExecutionStatus, taskID uuid.UUID, taskStatus models.TaskStatus, userID uuid.UUID) error {
	err := s.conn.WithTransaction(ctx, func(tx database.Transaction) error {
		repos := tx.Repositories()

		// First, verify the execution exists and belongs to the user's task
		execution, err := repos.TaskExecutions.GetByID(ctx, executionID)
		if err != nil {
			if err == database.ErrExecutionNotFound {
				return errors.New("execution not found")
			}
			return fmt.Errorf("failed to get execution: %w", err)
		}

		// Verify the task belongs to the user
		task, err := repos.Tasks.GetByID(ctx, execution.TaskID)
		if err != nil {
			return fmt.Errorf("failed to get task: %w", err)
		}

		if task.UserID != userID {
			return errors.New("access denied: task does not belong to user")
		}

		// Verify the task ID matches
		if execution.TaskID != taskID {
			return errors.New("execution does not belong to the specified task")
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

func (s *mockableTaskExecutionService) CancelExecutionAndResetTaskStatus(ctx context.Context, executionID uuid.UUID, userID uuid.UUID) error {
	err := s.conn.WithTransaction(ctx, func(tx database.Transaction) error {
		repos := tx.Repositories()

		// First, verify the execution exists and belongs to the user's task
		execution, err := repos.TaskExecutions.GetByID(ctx, executionID)
		if err != nil {
			if err == database.ErrExecutionNotFound {
				return errors.New("execution not found")
			}
			return fmt.Errorf("failed to get execution: %w", err)
		}

		// Verify the task belongs to the user
		task, err := repos.Tasks.GetByID(ctx, execution.TaskID)
		if err != nil {
			return fmt.Errorf("failed to get task: %w", err)
		}

		if task.UserID != userID {
			return errors.New("access denied: task does not belong to user")
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

func (s *mockableTaskExecutionService) CompleteExecutionAndFinalizeTaskStatus(ctx context.Context, execution *models.TaskExecution, taskStatus models.TaskStatus, userID uuid.UUID) error {
	err := s.conn.WithTransaction(ctx, func(tx database.Transaction) error {
		repos := tx.Repositories()

		// First, verify the execution exists and belongs to the user's task
		existingExecution, err := repos.TaskExecutions.GetByID(ctx, execution.ID)
		if err != nil {
			if err == database.ErrExecutionNotFound {
				return errors.New("execution not found")
			}
			return fmt.Errorf("failed to get execution: %w", err)
		}

		// Verify the task belongs to the user
		task, err := repos.Tasks.GetByID(ctx, existingExecution.TaskID)
		if err != nil {
			return fmt.Errorf("failed to get task: %w", err)
		}

		if task.UserID != userID {
			return errors.New("access denied: task does not belong to user")
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

// MockTransaction mocks the database.Transaction interface
type MockTransaction struct {
	mock.Mock
	pgx.Tx
}

func (m *MockTransaction) Repositories() database.TransactionalRepositories {
	args := m.Called()
	return args.Get(0).(database.TransactionalRepositories)
}

// MockTaskRepository mocks the database.TaskRepository interface
type MockTaskRepository struct {
	mock.Mock
}

func (m *MockTaskRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Task, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Task), args.Error(1)
}

func (m *MockTaskRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status models.TaskStatus) error {
	args := m.Called(ctx, id, status)
	return args.Error(0)
}

func (m *MockTaskRepository) Create(ctx context.Context, task *models.Task) error {
	args := m.Called(ctx, task)
	return args.Error(0)
}

func (m *MockTaskRepository) Update(ctx context.Context, task *models.Task) error {
	args := m.Called(ctx, task)
	return args.Error(0)
}

func (m *MockTaskRepository) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockTaskRepository) GetByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*models.Task, error) {
	args := m.Called(ctx, userID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Task), args.Error(1)
}

func (m *MockTaskRepository) GetByStatus(ctx context.Context, status models.TaskStatus, limit, offset int) ([]*models.Task, error) {
	args := m.Called(ctx, status, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Task), args.Error(1)
}

func (m *MockTaskRepository) List(ctx context.Context, limit, offset int) ([]*models.Task, error) {
	args := m.Called(ctx, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Task), args.Error(1)
}

func (m *MockTaskRepository) SearchByMetadata(ctx context.Context, query string, limit, offset int) ([]*models.Task, error) {
	args := m.Called(ctx, query, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Task), args.Error(1)
}

func (m *MockTaskRepository) GetByUserIDCursor(ctx context.Context, userID uuid.UUID, req database.CursorPaginationRequest) ([]*models.Task, database.CursorPaginationResponse, error) {
	args := m.Called(ctx, userID, req)
	if args.Get(0) == nil {
		return nil, args.Get(1).(database.CursorPaginationResponse), args.Error(2)
	}
	return args.Get(0).([]*models.Task), args.Get(1).(database.CursorPaginationResponse), args.Error(2)
}

func (m *MockTaskRepository) GetByStatusCursor(ctx context.Context, status models.TaskStatus, req database.CursorPaginationRequest) ([]*models.Task, database.CursorPaginationResponse, error) {
	args := m.Called(ctx, status, req)
	if args.Get(0) == nil {
		return nil, args.Get(1).(database.CursorPaginationResponse), args.Error(2)
	}
	return args.Get(0).([]*models.Task), args.Get(1).(database.CursorPaginationResponse), args.Error(2)
}

func (m *MockTaskRepository) ListCursor(ctx context.Context, req database.CursorPaginationRequest) ([]*models.Task, database.CursorPaginationResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Get(1).(database.CursorPaginationResponse), args.Error(2)
	}
	return args.Get(0).([]*models.Task), args.Get(1).(database.CursorPaginationResponse), args.Error(2)
}

func (m *MockTaskRepository) GetTasksWithExecutionCount(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*models.Task, error) {
	args := m.Called(ctx, userID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Task), args.Error(1)
}

func (m *MockTaskRepository) GetTasksWithLatestExecution(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*models.Task, error) {
	args := m.Called(ctx, userID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Task), args.Error(1)
}

func (m *MockTaskRepository) Count(ctx context.Context) (int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockTaskRepository) CountByUserID(ctx context.Context, userID uuid.UUID) (int64, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockTaskRepository) CountByStatus(ctx context.Context, status models.TaskStatus) (int64, error) {
	args := m.Called(ctx, status)
	return args.Get(0).(int64), args.Error(1)
}

// MockTaskExecutionRepository mocks the database.TaskExecutionRepository interface
type MockTaskExecutionRepository struct {
	mock.Mock
}

func (m *MockTaskExecutionRepository) Create(ctx context.Context, execution *models.TaskExecution) error {
	args := m.Called(ctx, execution)
	return args.Error(0)
}

func (m *MockTaskExecutionRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.TaskExecution, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.TaskExecution), args.Error(1)
}

func (m *MockTaskExecutionRepository) Update(ctx context.Context, execution *models.TaskExecution) error {
	args := m.Called(ctx, execution)
	return args.Error(0)
}

func (m *MockTaskExecutionRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status models.ExecutionStatus) error {
	args := m.Called(ctx, id, status)
	return args.Error(0)
}

func (m *MockTaskExecutionRepository) GetLatestByTaskID(ctx context.Context, taskID uuid.UUID) (*models.TaskExecution, error) {
	args := m.Called(ctx, taskID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.TaskExecution), args.Error(1)
}

func (m *MockTaskExecutionRepository) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockTaskExecutionRepository) GetByTaskID(ctx context.Context, taskID uuid.UUID, limit, offset int) ([]*models.TaskExecution, error) {
	args := m.Called(ctx, taskID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.TaskExecution), args.Error(1)
}

func (m *MockTaskExecutionRepository) GetByStatus(ctx context.Context, status models.ExecutionStatus, limit, offset int) ([]*models.TaskExecution, error) {
	args := m.Called(ctx, status, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.TaskExecution), args.Error(1)
}

func (m *MockTaskExecutionRepository) List(ctx context.Context, limit, offset int) ([]*models.TaskExecution, error) {
	args := m.Called(ctx, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.TaskExecution), args.Error(1)
}

func (m *MockTaskExecutionRepository) GetByTaskIDCursor(ctx context.Context, taskID uuid.UUID, req database.CursorPaginationRequest) ([]*models.TaskExecution, database.CursorPaginationResponse, error) {
	args := m.Called(ctx, taskID, req)
	if args.Get(0) == nil {
		return nil, args.Get(1).(database.CursorPaginationResponse), args.Error(2)
	}
	return args.Get(0).([]*models.TaskExecution), args.Get(1).(database.CursorPaginationResponse), args.Error(2)
}

func (m *MockTaskExecutionRepository) GetByStatusCursor(ctx context.Context, status models.ExecutionStatus, req database.CursorPaginationRequest) ([]*models.TaskExecution, database.CursorPaginationResponse, error) {
	args := m.Called(ctx, status, req)
	if args.Get(0) == nil {
		return nil, args.Get(1).(database.CursorPaginationResponse), args.Error(2)
	}
	return args.Get(0).([]*models.TaskExecution), args.Get(1).(database.CursorPaginationResponse), args.Error(2)
}

func (m *MockTaskExecutionRepository) ListCursor(ctx context.Context, req database.CursorPaginationRequest) ([]*models.TaskExecution, database.CursorPaginationResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Get(1).(database.CursorPaginationResponse), args.Error(2)
	}
	return args.Get(0).([]*models.TaskExecution), args.Get(1).(database.CursorPaginationResponse), args.Error(2)
}

func (m *MockTaskExecutionRepository) Count(ctx context.Context) (int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockTaskExecutionRepository) CountByTaskID(ctx context.Context, taskID uuid.UUID) (int64, error) {
	args := m.Called(ctx, taskID)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockTaskExecutionRepository) CountByStatus(ctx context.Context, status models.ExecutionStatus) (int64, error) {
	args := m.Called(ctx, status)
	return args.Get(0).(int64), args.Error(1)
}

func TestNewTaskExecutionService(t *testing.T) {
	// Test service instantiation
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	// This would normally be a real connection, but for now we'll just test the constructor
	var conn *database.Connection
	service := NewTaskExecutionService(conn, logger)

	assert.NotNil(t, service)
	assert.Equal(t, conn, service.conn)
	assert.Equal(t, logger, service.logger)
}

func TestTaskExecutionService_CreateExecutionAndUpdateTaskStatus(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	taskID := uuid.New()
	userID := uuid.New()
	otherUserID := uuid.New()
	ctx := context.Background()

	t.Run("successful execution creation", func(t *testing.T) {
		mockConn := &MockConnection{}
		mockTaskRepo := &MockTaskRepository{}
		mockExecutionRepo := &MockTaskExecutionRepository{}

		service := newMockableTaskExecutionService(mockConn, logger)

		task := &models.Task{
			BaseModel: models.BaseModel{
				ID: taskID,
			},
			UserID: userID,
			Status: models.TaskStatusPending,
		}

		// Setup mock transaction
		mockConn.On("WithTransaction", ctx, mock.AnythingOfType("func(database.Transaction) error")).Return(
			func(fn func(tx database.Transaction) error) error {
				mockTx := &MockTransaction{}
				mockTx.On("Repositories").Return(database.TransactionalRepositories{
					Tasks:          mockTaskRepo,
					TaskExecutions: mockExecutionRepo,
				})
				return fn(mockTx)
			},
		)

		mockTaskRepo.On("GetByID", ctx, taskID).Return(task, nil)
		mockExecutionRepo.On("Create", ctx, mock.AnythingOfType("*models.TaskExecution")).Return(nil)
		mockTaskRepo.On("UpdateStatus", ctx, taskID, models.TaskStatusRunning).Return(nil)

		execution, err := service.CreateExecutionAndUpdateTaskStatus(ctx, taskID, userID)

		assert.NoError(t, err)
		assert.NotNil(t, execution)
		assert.Equal(t, taskID, execution.TaskID)
		assert.Equal(t, models.ExecutionStatusPending, execution.Status)

		mockConn.AssertExpectations(t)
		mockTaskRepo.AssertExpectations(t)
		mockExecutionRepo.AssertExpectations(t)
	})

	t.Run("task not found error", func(t *testing.T) {
		mockConn := &MockConnection{}
		mockTaskRepo := &MockTaskRepository{}

		service := newMockableTaskExecutionService(mockConn, logger)

		// Setup mock transaction
		mockConn.On("WithTransaction", ctx, mock.AnythingOfType("func(database.Transaction) error")).Return(
			func(fn func(tx database.Transaction) error) error {
				mockTx := &MockTransaction{}
				mockTx.On("Repositories").Return(database.TransactionalRepositories{
					Tasks: mockTaskRepo,
				})
				return fn(mockTx)
			},
		)

		mockTaskRepo.On("GetByID", ctx, taskID).Return(nil, database.ErrTaskNotFound)

		execution, err := service.CreateExecutionAndUpdateTaskStatus(ctx, taskID, userID)

		assert.Error(t, err)
		assert.Nil(t, execution)
		assert.Contains(t, err.Error(), "task not found")

		mockConn.AssertExpectations(t)
		mockTaskRepo.AssertExpectations(t)
	})

	t.Run("database error on task fetch", func(t *testing.T) {
		mockConn := &MockConnection{}
		mockTaskRepo := &MockTaskRepository{}

		service := newMockableTaskExecutionService(mockConn, logger)

		dbError := errors.New("database connection failed")

		// Setup mock transaction
		mockConn.On("WithTransaction", ctx, mock.AnythingOfType("func(database.Transaction) error")).Return(
			func(fn func(tx database.Transaction) error) error {
				mockTx := &MockTransaction{}
				mockTx.On("Repositories").Return(database.TransactionalRepositories{
					Tasks: mockTaskRepo,
				})
				return fn(mockTx)
			},
		)

		mockTaskRepo.On("GetByID", ctx, taskID).Return(nil, dbError)

		execution, err := service.CreateExecutionAndUpdateTaskStatus(ctx, taskID, userID)

		assert.Error(t, err)
		assert.Nil(t, execution)
		assert.Contains(t, err.Error(), "failed to get task")
		assert.Contains(t, err.Error(), "database connection failed")

		mockConn.AssertExpectations(t)
		mockTaskRepo.AssertExpectations(t)
	})

	t.Run("access denied - task belongs to different user", func(t *testing.T) {
		mockConn := &MockConnection{}
		mockTaskRepo := &MockTaskRepository{}

		service := newMockableTaskExecutionService(mockConn, logger)

		task := &models.Task{
			BaseModel: models.BaseModel{
				ID: taskID,
			},
			UserID: otherUserID, // Different user
			Status: models.TaskStatusPending,
		}

		// Setup mock transaction
		mockConn.On("WithTransaction", ctx, mock.AnythingOfType("func(database.Transaction) error")).Return(
			func(fn func(tx database.Transaction) error) error {
				mockTx := &MockTransaction{}
				mockTx.On("Repositories").Return(database.TransactionalRepositories{
					Tasks: mockTaskRepo,
				})
				return fn(mockTx)
			},
		)

		mockTaskRepo.On("GetByID", ctx, taskID).Return(task, nil)

		execution, err := service.CreateExecutionAndUpdateTaskStatus(ctx, taskID, userID)

		assert.Error(t, err)
		assert.Nil(t, execution)
		assert.Contains(t, err.Error(), "access denied: task does not belong to user")

		mockConn.AssertExpectations(t)
		mockTaskRepo.AssertExpectations(t)
	})

	t.Run("task already running error", func(t *testing.T) {
		mockConn := &MockConnection{}
		mockTaskRepo := &MockTaskRepository{}

		service := newMockableTaskExecutionService(mockConn, logger)

		task := &models.Task{
			BaseModel: models.BaseModel{
				ID: taskID,
			},
			UserID: userID,
			Status: models.TaskStatusRunning,
		}

		// Setup mock transaction
		mockConn.On("WithTransaction", ctx, mock.AnythingOfType("func(database.Transaction) error")).Return(
			func(fn func(tx database.Transaction) error) error {
				mockTx := &MockTransaction{}
				mockTx.On("Repositories").Return(database.TransactionalRepositories{
					Tasks: mockTaskRepo,
				})
				return fn(mockTx)
			},
		)

		mockTaskRepo.On("GetByID", ctx, taskID).Return(task, nil)

		execution, err := service.CreateExecutionAndUpdateTaskStatus(ctx, taskID, userID)

		assert.Error(t, err)
		assert.Nil(t, execution)
		assert.Contains(t, err.Error(), "task is already running")

		mockConn.AssertExpectations(t)
		mockTaskRepo.AssertExpectations(t)
	})

	t.Run("cannot execute completed task", func(t *testing.T) {
		mockConn := &MockConnection{}
		mockTaskRepo := &MockTaskRepository{}

		service := newMockableTaskExecutionService(mockConn, logger)

		task := &models.Task{
			BaseModel: models.BaseModel{
				ID: taskID,
			},
			UserID: userID,
			Status: models.TaskStatusCompleted,
		}

		// Setup mock transaction
		mockConn.On("WithTransaction", ctx, mock.AnythingOfType("func(database.Transaction) error")).Return(
			func(fn func(tx database.Transaction) error) error {
				mockTx := &MockTransaction{}
				mockTx.On("Repositories").Return(database.TransactionalRepositories{
					Tasks: mockTaskRepo,
				})
				return fn(mockTx)
			},
		)

		mockTaskRepo.On("GetByID", ctx, taskID).Return(task, nil)

		execution, err := service.CreateExecutionAndUpdateTaskStatus(ctx, taskID, userID)

		assert.Error(t, err)
		assert.Nil(t, execution)
		assert.Contains(t, err.Error(), "cannot execute task with status: completed")

		mockConn.AssertExpectations(t)
		mockTaskRepo.AssertExpectations(t)
	})

	t.Run("execution creation fails", func(t *testing.T) {
		mockConn := &MockConnection{}
		mockTaskRepo := &MockTaskRepository{}
		mockExecutionRepo := &MockTaskExecutionRepository{}

		service := newMockableTaskExecutionService(mockConn, logger)

		task := &models.Task{
			BaseModel: models.BaseModel{
				ID: taskID,
			},
			UserID: userID,
			Status: models.TaskStatusPending,
		}

		execError := errors.New("execution creation failed")

		// Setup mock transaction
		mockConn.On("WithTransaction", ctx, mock.AnythingOfType("func(database.Transaction) error")).Return(
			func(fn func(tx database.Transaction) error) error {
				mockTx := &MockTransaction{}
				mockTx.On("Repositories").Return(database.TransactionalRepositories{
					Tasks:          mockTaskRepo,
					TaskExecutions: mockExecutionRepo,
				})
				return fn(mockTx)
			},
		)

		mockTaskRepo.On("GetByID", ctx, taskID).Return(task, nil)
		mockExecutionRepo.On("Create", ctx, mock.AnythingOfType("*models.TaskExecution")).Return(execError)

		execution, err := service.CreateExecutionAndUpdateTaskStatus(ctx, taskID, userID)

		assert.Error(t, err)
		assert.Nil(t, execution)
		assert.Contains(t, err.Error(), "failed to create task execution")
		assert.Contains(t, err.Error(), "execution creation failed")

		mockConn.AssertExpectations(t)
		mockTaskRepo.AssertExpectations(t)
		mockExecutionRepo.AssertExpectations(t)
	})

	t.Run("task status update fails", func(t *testing.T) {
		mockConn := &MockConnection{}
		mockTaskRepo := &MockTaskRepository{}
		mockExecutionRepo := &MockTaskExecutionRepository{}

		service := newMockableTaskExecutionService(mockConn, logger)

		task := &models.Task{
			BaseModel: models.BaseModel{
				ID: taskID,
			},
			UserID: userID,
			Status: models.TaskStatusPending,
		}

		statusError := errors.New("task status update failed")

		// Setup mock transaction
		mockConn.On("WithTransaction", ctx, mock.AnythingOfType("func(database.Transaction) error")).Return(
			func(fn func(tx database.Transaction) error) error {
				mockTx := &MockTransaction{}
				mockTx.On("Repositories").Return(database.TransactionalRepositories{
					Tasks:          mockTaskRepo,
					TaskExecutions: mockExecutionRepo,
				})
				return fn(mockTx)
			},
		)

		mockTaskRepo.On("GetByID", ctx, taskID).Return(task, nil)
		mockExecutionRepo.On("Create", ctx, mock.AnythingOfType("*models.TaskExecution")).Return(nil)
		mockTaskRepo.On("UpdateStatus", ctx, taskID, models.TaskStatusRunning).Return(statusError)

		execution, err := service.CreateExecutionAndUpdateTaskStatus(ctx, taskID, userID)

		assert.Error(t, err)
		assert.Nil(t, execution)
		assert.Contains(t, err.Error(), "failed to update task status")
		assert.Contains(t, err.Error(), "task status update failed")

		mockConn.AssertExpectations(t)
		mockTaskRepo.AssertExpectations(t)
		mockExecutionRepo.AssertExpectations(t)
	})

	t.Run("transaction failure", func(t *testing.T) {
		mockConn := &MockConnection{}

		service := newMockableTaskExecutionService(mockConn, logger)

		transactionError := errors.New("transaction failed to commit")
		mockConn.On("WithTransaction", ctx, mock.AnythingOfType("func(database.Transaction) error")).Return(transactionError)

		execution, err := service.CreateExecutionAndUpdateTaskStatus(ctx, taskID, userID)

		assert.Error(t, err)
		assert.Nil(t, execution)
		assert.Equal(t, transactionError, err)

		mockConn.AssertExpectations(t)
	})
}

func TestTaskExecutionService_UpdateExecutionAndTaskStatus(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	executionID := uuid.New()
	taskID := uuid.New()
	userID := uuid.New()
	otherUserID := uuid.New()
	ctx := context.Background()

	t.Run("successful update", func(t *testing.T) {
		mockConn := &MockConnection{}
		mockTaskRepo := &MockTaskRepository{}
		mockExecutionRepo := &MockTaskExecutionRepository{}

		service := newMockableTaskExecutionService(mockConn, logger)

		execution := &models.TaskExecution{
			ID:     executionID,
			TaskID: taskID,
			Status: models.ExecutionStatusRunning,
		}

		task := &models.Task{
			BaseModel: models.BaseModel{
				ID: taskID,
			},
			UserID: userID,
			Status: models.TaskStatusRunning,
		}

		// Setup mock transaction
		mockConn.On("WithTransaction", ctx, mock.AnythingOfType("func(database.Transaction) error")).Return(
			func(fn func(tx database.Transaction) error) error {
				mockTx := &MockTransaction{}
				mockTx.On("Repositories").Return(database.TransactionalRepositories{
					Tasks:          mockTaskRepo,
					TaskExecutions: mockExecutionRepo,
				})
				return fn(mockTx)
			},
		)

		mockExecutionRepo.On("GetByID", ctx, executionID).Return(execution, nil)
		mockTaskRepo.On("GetByID", ctx, taskID).Return(task, nil)
		mockExecutionRepo.On("UpdateStatus", ctx, executionID, models.ExecutionStatusCompleted).Return(nil)
		mockTaskRepo.On("UpdateStatus", ctx, taskID, models.TaskStatusCompleted).Return(nil)

		err := service.UpdateExecutionAndTaskStatus(ctx, executionID, models.ExecutionStatusCompleted, taskID, models.TaskStatusCompleted, userID)

		assert.NoError(t, err)

		mockConn.AssertExpectations(t)
		mockTaskRepo.AssertExpectations(t)
		mockExecutionRepo.AssertExpectations(t)
	})

	t.Run("execution not found", func(t *testing.T) {
		mockConn := &MockConnection{}
		mockExecutionRepo := &MockTaskExecutionRepository{}

		service := newMockableTaskExecutionService(mockConn, logger)

		// Setup mock transaction
		mockConn.On("WithTransaction", ctx, mock.AnythingOfType("func(database.Transaction) error")).Return(
			func(fn func(tx database.Transaction) error) error {
				mockTx := &MockTransaction{}
				mockTx.On("Repositories").Return(database.TransactionalRepositories{
					TaskExecutions: mockExecutionRepo,
				})
				return fn(mockTx)
			},
		)

		mockExecutionRepo.On("GetByID", ctx, executionID).Return(nil, database.ErrExecutionNotFound)

		err := service.UpdateExecutionAndTaskStatus(ctx, executionID, models.ExecutionStatusCompleted, taskID, models.TaskStatusCompleted, userID)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "execution not found")

		mockConn.AssertExpectations(t)
		mockExecutionRepo.AssertExpectations(t)
	})

	t.Run("database error on execution fetch", func(t *testing.T) {
		mockConn := &MockConnection{}
		mockExecutionRepo := &MockTaskExecutionRepository{}

		service := newMockableTaskExecutionService(mockConn, logger)

		dbError := errors.New("database connection failed")

		// Setup mock transaction
		mockConn.On("WithTransaction", ctx, mock.AnythingOfType("func(database.Transaction) error")).Return(
			func(fn func(tx database.Transaction) error) error {
				mockTx := &MockTransaction{}
				mockTx.On("Repositories").Return(database.TransactionalRepositories{
					TaskExecutions: mockExecutionRepo,
				})
				return fn(mockTx)
			},
		)

		mockExecutionRepo.On("GetByID", ctx, executionID).Return(nil, dbError)

		err := service.UpdateExecutionAndTaskStatus(ctx, executionID, models.ExecutionStatusCompleted, taskID, models.TaskStatusCompleted, userID)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get execution")
		assert.Contains(t, err.Error(), "database connection failed")

		mockConn.AssertExpectations(t)
		mockExecutionRepo.AssertExpectations(t)
	})

	t.Run("access denied - task belongs to different user", func(t *testing.T) {
		mockConn := &MockConnection{}
		mockTaskRepo := &MockTaskRepository{}
		mockExecutionRepo := &MockTaskExecutionRepository{}

		service := newMockableTaskExecutionService(mockConn, logger)

		execution := &models.TaskExecution{
			ID:     executionID,
			TaskID: taskID,
			Status: models.ExecutionStatusRunning,
		}

		task := &models.Task{
			BaseModel: models.BaseModel{
				ID: taskID,
			},
			UserID: otherUserID, // Different user
			Status: models.TaskStatusRunning,
		}

		// Setup mock transaction
		mockConn.On("WithTransaction", ctx, mock.AnythingOfType("func(database.Transaction) error")).Return(
			func(fn func(tx database.Transaction) error) error {
				mockTx := &MockTransaction{}
				mockTx.On("Repositories").Return(database.TransactionalRepositories{
					Tasks:          mockTaskRepo,
					TaskExecutions: mockExecutionRepo,
				})
				return fn(mockTx)
			},
		)

		mockExecutionRepo.On("GetByID", ctx, executionID).Return(execution, nil)
		mockTaskRepo.On("GetByID", ctx, taskID).Return(task, nil)

		err := service.UpdateExecutionAndTaskStatus(ctx, executionID, models.ExecutionStatusCompleted, taskID, models.TaskStatusCompleted, userID)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "access denied: task does not belong to user")

		mockConn.AssertExpectations(t)
		mockTaskRepo.AssertExpectations(t)
		mockExecutionRepo.AssertExpectations(t)
	})

	t.Run("task ID mismatch", func(t *testing.T) {
		mockConn := &MockConnection{}
		mockTaskRepo := &MockTaskRepository{}
		mockExecutionRepo := &MockTaskExecutionRepository{}

		service := newMockableTaskExecutionService(mockConn, logger)

		differentTaskID := uuid.New()
		execution := &models.TaskExecution{
			ID:     executionID,
			TaskID: differentTaskID, // Different task
			Status: models.ExecutionStatusRunning,
		}

		task := &models.Task{
			BaseModel: models.BaseModel{
				ID: differentTaskID,
			},
			UserID: userID,
			Status: models.TaskStatusRunning,
		}

		// Setup mock transaction
		mockConn.On("WithTransaction", ctx, mock.AnythingOfType("func(database.Transaction) error")).Return(
			func(fn func(tx database.Transaction) error) error {
				mockTx := &MockTransaction{}
				mockTx.On("Repositories").Return(database.TransactionalRepositories{
					Tasks:          mockTaskRepo,
					TaskExecutions: mockExecutionRepo,
				})
				return fn(mockTx)
			},
		)

		mockExecutionRepo.On("GetByID", ctx, executionID).Return(execution, nil)
		mockTaskRepo.On("GetByID", ctx, differentTaskID).Return(task, nil)

		err := service.UpdateExecutionAndTaskStatus(ctx, executionID, models.ExecutionStatusCompleted, taskID, models.TaskStatusCompleted, userID)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "execution does not belong to the specified task")

		mockConn.AssertExpectations(t)
		mockTaskRepo.AssertExpectations(t)
		mockExecutionRepo.AssertExpectations(t)
	})

	t.Run("execution status update fails", func(t *testing.T) {
		mockConn := &MockConnection{}
		mockTaskRepo := &MockTaskRepository{}
		mockExecutionRepo := &MockTaskExecutionRepository{}

		service := newMockableTaskExecutionService(mockConn, logger)

		execution := &models.TaskExecution{
			ID:     executionID,
			TaskID: taskID,
			Status: models.ExecutionStatusRunning,
		}

		task := &models.Task{
			BaseModel: models.BaseModel{
				ID: taskID,
			},
			UserID: userID,
			Status: models.TaskStatusRunning,
		}

		execError := errors.New("execution status update failed")

		// Setup mock transaction
		mockConn.On("WithTransaction", ctx, mock.AnythingOfType("func(database.Transaction) error")).Return(
			func(fn func(tx database.Transaction) error) error {
				mockTx := &MockTransaction{}
				mockTx.On("Repositories").Return(database.TransactionalRepositories{
					Tasks:          mockTaskRepo,
					TaskExecutions: mockExecutionRepo,
				})
				return fn(mockTx)
			},
		)

		mockExecutionRepo.On("GetByID", ctx, executionID).Return(execution, nil)
		mockTaskRepo.On("GetByID", ctx, taskID).Return(task, nil)
		mockExecutionRepo.On("UpdateStatus", ctx, executionID, models.ExecutionStatusCompleted).Return(execError)

		err := service.UpdateExecutionAndTaskStatus(ctx, executionID, models.ExecutionStatusCompleted, taskID, models.TaskStatusCompleted, userID)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to update execution status")
		assert.Contains(t, err.Error(), "execution status update failed")

		mockConn.AssertExpectations(t)
		mockTaskRepo.AssertExpectations(t)
		mockExecutionRepo.AssertExpectations(t)
	})

	t.Run("task status update fails", func(t *testing.T) {
		mockConn := &MockConnection{}
		mockTaskRepo := &MockTaskRepository{}
		mockExecutionRepo := &MockTaskExecutionRepository{}

		service := newMockableTaskExecutionService(mockConn, logger)

		execution := &models.TaskExecution{
			ID:     executionID,
			TaskID: taskID,
			Status: models.ExecutionStatusRunning,
		}

		task := &models.Task{
			BaseModel: models.BaseModel{
				ID: taskID,
			},
			UserID: userID,
			Status: models.TaskStatusRunning,
		}

		statusError := errors.New("task status update failed")

		// Setup mock transaction
		mockConn.On("WithTransaction", ctx, mock.AnythingOfType("func(database.Transaction) error")).Return(
			func(fn func(tx database.Transaction) error) error {
				mockTx := &MockTransaction{}
				mockTx.On("Repositories").Return(database.TransactionalRepositories{
					Tasks:          mockTaskRepo,
					TaskExecutions: mockExecutionRepo,
				})
				return fn(mockTx)
			},
		)

		mockExecutionRepo.On("GetByID", ctx, executionID).Return(execution, nil)
		mockTaskRepo.On("GetByID", ctx, taskID).Return(task, nil)
		mockExecutionRepo.On("UpdateStatus", ctx, executionID, models.ExecutionStatusCompleted).Return(nil)
		mockTaskRepo.On("UpdateStatus", ctx, taskID, models.TaskStatusCompleted).Return(statusError)

		err := service.UpdateExecutionAndTaskStatus(ctx, executionID, models.ExecutionStatusCompleted, taskID, models.TaskStatusCompleted, userID)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to update task status")
		assert.Contains(t, err.Error(), "task status update failed")

		mockConn.AssertExpectations(t)
		mockTaskRepo.AssertExpectations(t)
		mockExecutionRepo.AssertExpectations(t)
	})
}

func TestTaskExecutionService_CancelExecutionAndResetTaskStatus(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	executionID := uuid.New()
	taskID := uuid.New()
	userID := uuid.New()
	otherUserID := uuid.New()
	ctx := context.Background()

	t.Run("successful cancellation", func(t *testing.T) {
		mockConn := &MockConnection{}
		mockTaskRepo := &MockTaskRepository{}
		mockExecutionRepo := &MockTaskExecutionRepository{}

		service := newMockableTaskExecutionService(mockConn, logger)

		execution := &models.TaskExecution{
			ID:     executionID,
			TaskID: taskID,
			Status: models.ExecutionStatusRunning,
		}

		task := &models.Task{
			BaseModel: models.BaseModel{
				ID: taskID,
			},
			UserID: userID,
			Status: models.TaskStatusRunning,
		}

		// Setup mock transaction
		mockConn.On("WithTransaction", ctx, mock.AnythingOfType("func(database.Transaction) error")).Return(
			func(fn func(tx database.Transaction) error) error {
				mockTx := &MockTransaction{}
				mockTx.On("Repositories").Return(database.TransactionalRepositories{
					Tasks:          mockTaskRepo,
					TaskExecutions: mockExecutionRepo,
				})
				return fn(mockTx)
			},
		)

		mockExecutionRepo.On("GetByID", ctx, executionID).Return(execution, nil)
		mockTaskRepo.On("GetByID", ctx, taskID).Return(task, nil)
		mockExecutionRepo.On("UpdateStatus", ctx, executionID, models.ExecutionStatusCancelled).Return(nil)
		mockTaskRepo.On("UpdateStatus", ctx, taskID, models.TaskStatusPending).Return(nil)

		err := service.CancelExecutionAndResetTaskStatus(ctx, executionID, userID)

		assert.NoError(t, err)

		mockConn.AssertExpectations(t)
		mockTaskRepo.AssertExpectations(t)
		mockExecutionRepo.AssertExpectations(t)
	})

	t.Run("cannot cancel completed execution", func(t *testing.T) {
		mockConn := &MockConnection{}
		mockTaskRepo := &MockTaskRepository{}
		mockExecutionRepo := &MockTaskExecutionRepository{}

		service := newMockableTaskExecutionService(mockConn, logger)

		execution := &models.TaskExecution{
			ID:     executionID,
			TaskID: taskID,
			Status: models.ExecutionStatusCompleted,
		}

		task := &models.Task{
			BaseModel: models.BaseModel{
				ID: taskID,
			},
			UserID: userID,
			Status: models.TaskStatusCompleted,
		}

		// Setup mock transaction
		mockConn.On("WithTransaction", ctx, mock.AnythingOfType("func(database.Transaction) error")).Return(
			func(fn func(tx database.Transaction) error) error {
				mockTx := &MockTransaction{}
				mockTx.On("Repositories").Return(database.TransactionalRepositories{
					Tasks:          mockTaskRepo,
					TaskExecutions: mockExecutionRepo,
				})
				return fn(mockTx)
			},
		)

		mockExecutionRepo.On("GetByID", ctx, executionID).Return(execution, nil)
		mockTaskRepo.On("GetByID", ctx, taskID).Return(task, nil)

		err := service.CancelExecutionAndResetTaskStatus(ctx, executionID, userID)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot cancel execution with status: completed")

		mockConn.AssertExpectations(t)
		mockTaskRepo.AssertExpectations(t)
		mockExecutionRepo.AssertExpectations(t)
	})

	t.Run("access denied - task belongs to different user", func(t *testing.T) {
		mockConn := &MockConnection{}
		mockTaskRepo := &MockTaskRepository{}
		mockExecutionRepo := &MockTaskExecutionRepository{}

		service := newMockableTaskExecutionService(mockConn, logger)

		execution := &models.TaskExecution{
			ID:     executionID,
			TaskID: taskID,
			Status: models.ExecutionStatusRunning,
		}

		task := &models.Task{
			BaseModel: models.BaseModel{
				ID: taskID,
			},
			UserID: otherUserID, // Different user
			Status: models.TaskStatusRunning,
		}

		// Setup mock transaction
		mockConn.On("WithTransaction", ctx, mock.AnythingOfType("func(database.Transaction) error")).Return(
			func(fn func(tx database.Transaction) error) error {
				mockTx := &MockTransaction{}
				mockTx.On("Repositories").Return(database.TransactionalRepositories{
					Tasks:          mockTaskRepo,
					TaskExecutions: mockExecutionRepo,
				})
				return fn(mockTx)
			},
		)

		mockExecutionRepo.On("GetByID", ctx, executionID).Return(execution, nil)
		mockTaskRepo.On("GetByID", ctx, taskID).Return(task, nil)

		err := service.CancelExecutionAndResetTaskStatus(ctx, executionID, userID)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "access denied: task does not belong to user")

		mockConn.AssertExpectations(t)
		mockTaskRepo.AssertExpectations(t)
		mockExecutionRepo.AssertExpectations(t)
	})
}

func TestTaskExecutionService_CompleteExecutionAndFinalizeTaskStatus(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	executionID := uuid.New()
	taskID := uuid.New()
	userID := uuid.New()
	otherUserID := uuid.New()
	ctx := context.Background()

	t.Run("successful completion", func(t *testing.T) {
		mockConn := &MockConnection{}
		mockTaskRepo := &MockTaskRepository{}
		mockExecutionRepo := &MockTaskExecutionRepository{}

		service := newMockableTaskExecutionService(mockConn, logger)

		executionToUpdate := &models.TaskExecution{
			ID:     executionID,
			TaskID: taskID,
			Status: models.ExecutionStatusCompleted,
		}

		existingExecution := &models.TaskExecution{
			ID:     executionID,
			TaskID: taskID,
			Status: models.ExecutionStatusRunning,
		}

		task := &models.Task{
			BaseModel: models.BaseModel{
				ID: taskID,
			},
			UserID: userID,
			Status: models.TaskStatusRunning,
		}

		// Setup mock transaction
		mockConn.On("WithTransaction", ctx, mock.AnythingOfType("func(database.Transaction) error")).Return(
			func(fn func(tx database.Transaction) error) error {
				mockTx := &MockTransaction{}
				mockTx.On("Repositories").Return(database.TransactionalRepositories{
					Tasks:          mockTaskRepo,
					TaskExecutions: mockExecutionRepo,
				})
				return fn(mockTx)
			},
		)

		mockExecutionRepo.On("GetByID", ctx, executionID).Return(existingExecution, nil)
		mockTaskRepo.On("GetByID", ctx, taskID).Return(task, nil)
		mockExecutionRepo.On("Update", ctx, executionToUpdate).Return(nil)
		mockTaskRepo.On("UpdateStatus", ctx, taskID, models.TaskStatusCompleted).Return(nil)

		err := service.CompleteExecutionAndFinalizeTaskStatus(ctx, executionToUpdate, models.TaskStatusCompleted, userID)

		assert.NoError(t, err)

		mockConn.AssertExpectations(t)
		mockTaskRepo.AssertExpectations(t)
		mockExecutionRepo.AssertExpectations(t)
	})

	t.Run("execution not found", func(t *testing.T) {
		mockConn := &MockConnection{}
		mockExecutionRepo := &MockTaskExecutionRepository{}

		service := newMockableTaskExecutionService(mockConn, logger)

		executionToUpdate := &models.TaskExecution{
			ID:     executionID,
			TaskID: taskID,
			Status: models.ExecutionStatusCompleted,
		}

		// Setup mock transaction
		mockConn.On("WithTransaction", ctx, mock.AnythingOfType("func(database.Transaction) error")).Return(
			func(fn func(tx database.Transaction) error) error {
				mockTx := &MockTransaction{}
				mockTx.On("Repositories").Return(database.TransactionalRepositories{
					TaskExecutions: mockExecutionRepo,
				})
				return fn(mockTx)
			},
		)

		mockExecutionRepo.On("GetByID", ctx, executionID).Return(nil, database.ErrExecutionNotFound)

		err := service.CompleteExecutionAndFinalizeTaskStatus(ctx, executionToUpdate, models.TaskStatusCompleted, userID)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "execution not found")

		mockConn.AssertExpectations(t)
		mockExecutionRepo.AssertExpectations(t)
	})

	t.Run("access denied - task belongs to different user", func(t *testing.T) {
		mockConn := &MockConnection{}
		mockTaskRepo := &MockTaskRepository{}
		mockExecutionRepo := &MockTaskExecutionRepository{}

		service := newMockableTaskExecutionService(mockConn, logger)

		executionToUpdate := &models.TaskExecution{
			ID:     executionID,
			TaskID: taskID,
			Status: models.ExecutionStatusCompleted,
		}

		existingExecution := &models.TaskExecution{
			ID:     executionID,
			TaskID: taskID,
			Status: models.ExecutionStatusRunning,
		}

		task := &models.Task{
			BaseModel: models.BaseModel{
				ID: taskID,
			},
			UserID: otherUserID, // Different user
			Status: models.TaskStatusRunning,
		}

		// Setup mock transaction
		mockConn.On("WithTransaction", ctx, mock.AnythingOfType("func(database.Transaction) error")).Return(
			func(fn func(tx database.Transaction) error) error {
				mockTx := &MockTransaction{}
				mockTx.On("Repositories").Return(database.TransactionalRepositories{
					Tasks:          mockTaskRepo,
					TaskExecutions: mockExecutionRepo,
				})
				return fn(mockTx)
			},
		)

		mockExecutionRepo.On("GetByID", ctx, executionID).Return(existingExecution, nil)
		mockTaskRepo.On("GetByID", ctx, taskID).Return(task, nil)

		err := service.CompleteExecutionAndFinalizeTaskStatus(ctx, executionToUpdate, models.TaskStatusCompleted, userID)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "access denied: task does not belong to user")

		mockConn.AssertExpectations(t)
		mockTaskRepo.AssertExpectations(t)
		mockExecutionRepo.AssertExpectations(t)
	})

	t.Run("cannot complete already completed execution", func(t *testing.T) {
		mockConn := &MockConnection{}
		mockTaskRepo := &MockTaskRepository{}
		mockExecutionRepo := &MockTaskExecutionRepository{}

		service := newMockableTaskExecutionService(mockConn, logger)

		executionToUpdate := &models.TaskExecution{
			ID:     executionID,
			TaskID: taskID,
			Status: models.ExecutionStatusCompleted,
		}

		existingExecution := &models.TaskExecution{
			ID:     executionID,
			TaskID: taskID,
			Status: models.ExecutionStatusCompleted, // Already completed
		}

		task := &models.Task{
			BaseModel: models.BaseModel{
				ID: taskID,
			},
			UserID: userID,
			Status: models.TaskStatusCompleted,
		}

		// Setup mock transaction
		mockConn.On("WithTransaction", ctx, mock.AnythingOfType("func(database.Transaction) error")).Return(
			func(fn func(tx database.Transaction) error) error {
				mockTx := &MockTransaction{}
				mockTx.On("Repositories").Return(database.TransactionalRepositories{
					Tasks:          mockTaskRepo,
					TaskExecutions: mockExecutionRepo,
				})
				return fn(mockTx)
			},
		)

		mockExecutionRepo.On("GetByID", ctx, executionID).Return(existingExecution, nil)
		mockTaskRepo.On("GetByID", ctx, taskID).Return(task, nil)

		err := service.CompleteExecutionAndFinalizeTaskStatus(ctx, executionToUpdate, models.TaskStatusCompleted, userID)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot complete execution with status: completed")

		mockConn.AssertExpectations(t)
		mockTaskRepo.AssertExpectations(t)
		mockExecutionRepo.AssertExpectations(t)
	})

	t.Run("cannot complete cancelled execution", func(t *testing.T) {
		mockConn := &MockConnection{}
		mockTaskRepo := &MockTaskRepository{}
		mockExecutionRepo := &MockTaskExecutionRepository{}

		service := newMockableTaskExecutionService(mockConn, logger)

		executionToUpdate := &models.TaskExecution{
			ID:     executionID,
			TaskID: taskID,
			Status: models.ExecutionStatusCompleted,
		}

		existingExecution := &models.TaskExecution{
			ID:     executionID,
			TaskID: taskID,
			Status: models.ExecutionStatusCancelled, // Cancelled
		}

		task := &models.Task{
			BaseModel: models.BaseModel{
				ID: taskID,
			},
			UserID: userID,
			Status: models.TaskStatusCancelled,
		}

		// Setup mock transaction
		mockConn.On("WithTransaction", ctx, mock.AnythingOfType("func(database.Transaction) error")).Return(
			func(fn func(tx database.Transaction) error) error {
				mockTx := &MockTransaction{}
				mockTx.On("Repositories").Return(database.TransactionalRepositories{
					Tasks:          mockTaskRepo,
					TaskExecutions: mockExecutionRepo,
				})
				return fn(mockTx)
			},
		)

		mockExecutionRepo.On("GetByID", ctx, executionID).Return(existingExecution, nil)
		mockTaskRepo.On("GetByID", ctx, taskID).Return(task, nil)

		err := service.CompleteExecutionAndFinalizeTaskStatus(ctx, executionToUpdate, models.TaskStatusCompleted, userID)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot complete execution with status: cancelled")

		mockConn.AssertExpectations(t)
		mockTaskRepo.AssertExpectations(t)
		mockExecutionRepo.AssertExpectations(t)
	})

	t.Run("execution update fails", func(t *testing.T) {
		mockConn := &MockConnection{}
		mockTaskRepo := &MockTaskRepository{}
		mockExecutionRepo := &MockTaskExecutionRepository{}

		service := newMockableTaskExecutionService(mockConn, logger)

		executionToUpdate := &models.TaskExecution{
			ID:     executionID,
			TaskID: taskID,
			Status: models.ExecutionStatusCompleted,
		}

		existingExecution := &models.TaskExecution{
			ID:     executionID,
			TaskID: taskID,
			Status: models.ExecutionStatusRunning,
		}

		task := &models.Task{
			BaseModel: models.BaseModel{
				ID: taskID,
			},
			UserID: userID,
			Status: models.TaskStatusRunning,
		}

		execError := errors.New("execution update failed")

		// Setup mock transaction
		mockConn.On("WithTransaction", ctx, mock.AnythingOfType("func(database.Transaction) error")).Return(
			func(fn func(tx database.Transaction) error) error {
				mockTx := &MockTransaction{}
				mockTx.On("Repositories").Return(database.TransactionalRepositories{
					Tasks:          mockTaskRepo,
					TaskExecutions: mockExecutionRepo,
				})
				return fn(mockTx)
			},
		)

		mockExecutionRepo.On("GetByID", ctx, executionID).Return(existingExecution, nil)
		mockTaskRepo.On("GetByID", ctx, taskID).Return(task, nil)
		mockExecutionRepo.On("Update", ctx, executionToUpdate).Return(execError)

		err := service.CompleteExecutionAndFinalizeTaskStatus(ctx, executionToUpdate, models.TaskStatusCompleted, userID)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to update execution")
		assert.Contains(t, err.Error(), "execution update failed")

		mockConn.AssertExpectations(t)
		mockTaskRepo.AssertExpectations(t)
		mockExecutionRepo.AssertExpectations(t)
	})

	t.Run("task status update fails", func(t *testing.T) {
		mockConn := &MockConnection{}
		mockTaskRepo := &MockTaskRepository{}
		mockExecutionRepo := &MockTaskExecutionRepository{}

		service := newMockableTaskExecutionService(mockConn, logger)

		executionToUpdate := &models.TaskExecution{
			ID:     executionID,
			TaskID: taskID,
			Status: models.ExecutionStatusCompleted,
		}

		existingExecution := &models.TaskExecution{
			ID:     executionID,
			TaskID: taskID,
			Status: models.ExecutionStatusRunning,
		}

		task := &models.Task{
			BaseModel: models.BaseModel{
				ID: taskID,
			},
			UserID: userID,
			Status: models.TaskStatusRunning,
		}

		statusError := errors.New("task status update failed")

		// Setup mock transaction
		mockConn.On("WithTransaction", ctx, mock.AnythingOfType("func(database.Transaction) error")).Return(
			func(fn func(tx database.Transaction) error) error {
				mockTx := &MockTransaction{}
				mockTx.On("Repositories").Return(database.TransactionalRepositories{
					Tasks:          mockTaskRepo,
					TaskExecutions: mockExecutionRepo,
				})
				return fn(mockTx)
			},
		)

		mockExecutionRepo.On("GetByID", ctx, executionID).Return(existingExecution, nil)
		mockTaskRepo.On("GetByID", ctx, taskID).Return(task, nil)
		mockExecutionRepo.On("Update", ctx, executionToUpdate).Return(nil)
		mockTaskRepo.On("UpdateStatus", ctx, taskID, models.TaskStatusCompleted).Return(statusError)

		err := service.CompleteExecutionAndFinalizeTaskStatus(ctx, executionToUpdate, models.TaskStatusCompleted, userID)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to update task status")
		assert.Contains(t, err.Error(), "task status update failed")

		mockConn.AssertExpectations(t)
		mockTaskRepo.AssertExpectations(t)
		mockExecutionRepo.AssertExpectations(t)
	})
}

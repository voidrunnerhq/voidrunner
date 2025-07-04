package database

import (
	"context"

	"github.com/google/uuid"
	"github.com/voidrunnerhq/voidrunner/internal/models"
)

// UserRepository defines the interface for user data operations
type UserRepository interface {
	Create(ctx context.Context, user *models.User) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.User, error)
	GetByEmail(ctx context.Context, email string) (*models.User, error)
	Update(ctx context.Context, user *models.User) error
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context, limit, offset int) ([]*models.User, error)
	Count(ctx context.Context) (int64, error)
}

// TaskRepository defines the interface for task data operations
type TaskRepository interface {
	Create(ctx context.Context, task *models.Task) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Task, error)
	GetByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*models.Task, error)
	GetByStatus(ctx context.Context, status models.TaskStatus, limit, offset int) ([]*models.Task, error)
	Update(ctx context.Context, task *models.Task) error
	UpdateStatus(ctx context.Context, id uuid.UUID, status models.TaskStatus) error
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context, limit, offset int) ([]*models.Task, error)
	Count(ctx context.Context) (int64, error)
	CountByUserID(ctx context.Context, userID uuid.UUID) (int64, error)
	CountByStatus(ctx context.Context, status models.TaskStatus) (int64, error)
	SearchByMetadata(ctx context.Context, query string, limit, offset int) ([]*models.Task, error)
}

// TaskExecutionRepository defines the interface for task execution data operations
type TaskExecutionRepository interface {
	Create(ctx context.Context, execution *models.TaskExecution) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.TaskExecution, error)
	GetByTaskID(ctx context.Context, taskID uuid.UUID, limit, offset int) ([]*models.TaskExecution, error)
	GetLatestByTaskID(ctx context.Context, taskID uuid.UUID) (*models.TaskExecution, error)
	GetByStatus(ctx context.Context, status models.ExecutionStatus, limit, offset int) ([]*models.TaskExecution, error)
	Update(ctx context.Context, execution *models.TaskExecution) error
	UpdateStatus(ctx context.Context, id uuid.UUID, status models.ExecutionStatus) error
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context, limit, offset int) ([]*models.TaskExecution, error)
	Count(ctx context.Context) (int64, error)
	CountByTaskID(ctx context.Context, taskID uuid.UUID) (int64, error)
	CountByStatus(ctx context.Context, status models.ExecutionStatus) (int64, error)
}

// Repositories aggregates all repository interfaces
type Repositories struct {
	Users          UserRepository
	Tasks          TaskRepository
	TaskExecutions TaskExecutionRepository
}

// NewRepositories creates a new repositories instance
func NewRepositories(conn *Connection) *Repositories {
	return &Repositories{
		Users:          NewUserRepository(conn),
		Tasks:          NewTaskRepository(conn),
		TaskExecutions: NewTaskExecutionRepository(conn),
	}
}
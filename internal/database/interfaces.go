package database

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/voidrunnerhq/voidrunner/internal/models"
)

// Common errors
var (
	ErrUserNotFound = errors.New("user not found")
	ErrTaskNotFound = errors.New("task not found")
	ErrExecutionNotFound = errors.New("execution not found")
	ErrInvalidCursor = errors.New("invalid cursor")
)

// CursorPaginationRequest represents a cursor-based pagination request
type CursorPaginationRequest struct {
	Limit      int       `json:"limit"`
	Cursor     *string   `json:"cursor,omitempty"`
	SortOrder  string    `json:"sort_order"` // "asc" or "desc"
}

// CursorPaginationResponse represents a cursor-based pagination response
type CursorPaginationResponse struct {
	HasMore    bool    `json:"has_more"`
	NextCursor *string `json:"next_cursor,omitempty"`
	PrevCursor *string `json:"prev_cursor,omitempty"`
}

// TaskCursor represents a cursor for task pagination
type TaskCursor struct {
	CreatedAt time.Time `json:"created_at"`
	ID        uuid.UUID `json:"id"`
	Priority  *int      `json:"priority,omitempty"`
}

// ExecutionCursor represents a cursor for execution pagination
type ExecutionCursor struct {
	CreatedAt time.Time `json:"created_at"`
	ID        uuid.UUID `json:"id"`
}

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
	// Basic CRUD operations
	Create(ctx context.Context, task *models.Task) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Task, error)
	Update(ctx context.Context, task *models.Task) error
	UpdateStatus(ctx context.Context, id uuid.UUID, status models.TaskStatus) error
	Delete(ctx context.Context, id uuid.UUID) error

	// Offset-based pagination (legacy)
	GetByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*models.Task, error)
	GetByStatus(ctx context.Context, status models.TaskStatus, limit, offset int) ([]*models.Task, error)
	List(ctx context.Context, limit, offset int) ([]*models.Task, error)
	SearchByMetadata(ctx context.Context, query string, limit, offset int) ([]*models.Task, error)

	// Cursor-based pagination (optimized)
	GetByUserIDCursor(ctx context.Context, userID uuid.UUID, req CursorPaginationRequest) ([]*models.Task, CursorPaginationResponse, error)
	GetByStatusCursor(ctx context.Context, status models.TaskStatus, req CursorPaginationRequest) ([]*models.Task, CursorPaginationResponse, error)
	ListCursor(ctx context.Context, req CursorPaginationRequest) ([]*models.Task, CursorPaginationResponse, error)

	// Optimized bulk operations
	GetTasksWithExecutionCount(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*models.Task, error)
	GetTasksWithLatestExecution(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*models.Task, error)

	// Count operations
	Count(ctx context.Context) (int64, error)
	CountByUserID(ctx context.Context, userID uuid.UUID) (int64, error)
	CountByStatus(ctx context.Context, status models.TaskStatus) (int64, error)
}

// TaskExecutionRepository defines the interface for task execution data operations
type TaskExecutionRepository interface {
	// Basic CRUD operations
	Create(ctx context.Context, execution *models.TaskExecution) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.TaskExecution, error)
	GetLatestByTaskID(ctx context.Context, taskID uuid.UUID) (*models.TaskExecution, error)
	Update(ctx context.Context, execution *models.TaskExecution) error
	UpdateStatus(ctx context.Context, id uuid.UUID, status models.ExecutionStatus) error
	Delete(ctx context.Context, id uuid.UUID) error

	// Offset-based pagination (legacy)
	GetByTaskID(ctx context.Context, taskID uuid.UUID, limit, offset int) ([]*models.TaskExecution, error)
	GetByStatus(ctx context.Context, status models.ExecutionStatus, limit, offset int) ([]*models.TaskExecution, error)
	List(ctx context.Context, limit, offset int) ([]*models.TaskExecution, error)

	// Cursor-based pagination (optimized)
	GetByTaskIDCursor(ctx context.Context, taskID uuid.UUID, req CursorPaginationRequest) ([]*models.TaskExecution, CursorPaginationResponse, error)
	GetByStatusCursor(ctx context.Context, status models.ExecutionStatus, req CursorPaginationRequest) ([]*models.TaskExecution, CursorPaginationResponse, error)
	ListCursor(ctx context.Context, req CursorPaginationRequest) ([]*models.TaskExecution, CursorPaginationResponse, error)

	// Count operations
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
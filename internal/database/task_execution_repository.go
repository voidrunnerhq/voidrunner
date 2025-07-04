package database

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/voidrunnerhq/voidrunner/internal/models"
)

// taskExecutionRepository implements TaskExecutionRepository interface
type taskExecutionRepository struct {
	conn *Connection
}

// NewTaskExecutionRepository creates a new task execution repository
func NewTaskExecutionRepository(conn *Connection) TaskExecutionRepository {
	return &taskExecutionRepository{
		conn: conn,
	}
}

// Create creates a new task execution
func (r *taskExecutionRepository) Create(ctx context.Context, execution *models.TaskExecution) error {
	if execution == nil {
		return fmt.Errorf("task execution cannot be nil")
	}

	if execution.ID == uuid.Nil {
		execution.ID = models.NewID()
	}

	query := `
		INSERT INTO task_executions (id, task_id, status, return_code, stdout, stderr, execution_time_ms, memory_usage_bytes, started_at, completed_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW())
		RETURNING created_at
	`

	err := r.conn.Pool.QueryRow(ctx, query,
		execution.ID,
		execution.TaskID,
		execution.Status,
		execution.ReturnCode,
		execution.Stdout,
		execution.Stderr,
		execution.ExecutionTimeMs,
		execution.MemoryUsageBytes,
		execution.StartedAt,
		execution.CompletedAt,
	).Scan(&execution.CreatedAt)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Code {
			case "23505": // unique_violation
				return fmt.Errorf("task execution with ID %s already exists", execution.ID)
			case "23503": // foreign_key_violation
				if strings.Contains(pgErr.Detail, "task_id") {
					return fmt.Errorf("task with ID %s does not exist", execution.TaskID)
				}
			case "23514": // check_violation
				return fmt.Errorf("task execution validation failed: %s", pgErr.Detail)
			}
		}
		return fmt.Errorf("failed to create task execution: %w", err)
	}

	return nil
}

// GetByID retrieves a task execution by ID
func (r *taskExecutionRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.TaskExecution, error) {
	query := `
		SELECT id, task_id, status, return_code, stdout, stderr, execution_time_ms, memory_usage_bytes, started_at, completed_at, created_at
		FROM task_executions
		WHERE id = $1
	`

	var execution models.TaskExecution
	err := r.conn.Pool.QueryRow(ctx, query, id).Scan(
		&execution.ID,
		&execution.TaskID,
		&execution.Status,
		&execution.ReturnCode,
		&execution.Stdout,
		&execution.Stderr,
		&execution.ExecutionTimeMs,
		&execution.MemoryUsageBytes,
		&execution.StartedAt,
		&execution.CompletedAt,
		&execution.CreatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("task execution with ID %s not found", id)
		}
		return nil, fmt.Errorf("failed to get task execution by ID: %w", err)
	}

	return &execution, nil
}

// GetByTaskID retrieves task executions by task ID with pagination
func (r *taskExecutionRepository) GetByTaskID(ctx context.Context, taskID uuid.UUID, limit, offset int) ([]*models.TaskExecution, error) {
	if limit <= 0 {
		limit = 10
	}
	if offset < 0 {
		offset = 0
	}

	query := `
		SELECT id, task_id, status, return_code, stdout, stderr, execution_time_ms, memory_usage_bytes, started_at, completed_at, created_at
		FROM task_executions
		WHERE task_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.conn.Pool.Query(ctx, query, taskID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get task executions by task ID: %w", err)
	}
	defer rows.Close()

	return r.scanTaskExecutions(rows)
}

// GetLatestByTaskID retrieves the latest task execution for a task
func (r *taskExecutionRepository) GetLatestByTaskID(ctx context.Context, taskID uuid.UUID) (*models.TaskExecution, error) {
	query := `
		SELECT id, task_id, status, return_code, stdout, stderr, execution_time_ms, memory_usage_bytes, started_at, completed_at, created_at
		FROM task_executions
		WHERE task_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`

	var execution models.TaskExecution
	err := r.conn.Pool.QueryRow(ctx, query, taskID).Scan(
		&execution.ID,
		&execution.TaskID,
		&execution.Status,
		&execution.ReturnCode,
		&execution.Stdout,
		&execution.Stderr,
		&execution.ExecutionTimeMs,
		&execution.MemoryUsageBytes,
		&execution.StartedAt,
		&execution.CompletedAt,
		&execution.CreatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("no task executions found for task ID %s", taskID)
		}
		return nil, fmt.Errorf("failed to get latest task execution by task ID: %w", err)
	}

	return &execution, nil
}

// GetByStatus retrieves task executions by status with pagination
func (r *taskExecutionRepository) GetByStatus(ctx context.Context, status models.ExecutionStatus, limit, offset int) ([]*models.TaskExecution, error) {
	if limit <= 0 {
		limit = 10
	}
	if offset < 0 {
		offset = 0
	}

	query := `
		SELECT id, task_id, status, return_code, stdout, stderr, execution_time_ms, memory_usage_bytes, started_at, completed_at, created_at
		FROM task_executions
		WHERE status = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.conn.Pool.Query(ctx, query, status, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get task executions by status: %w", err)
	}
	defer rows.Close()

	return r.scanTaskExecutions(rows)
}

// Update updates a task execution
func (r *taskExecutionRepository) Update(ctx context.Context, execution *models.TaskExecution) error {
	if execution == nil {
		return fmt.Errorf("task execution cannot be nil")
	}

	query := `
		UPDATE task_executions
		SET status = $2, return_code = $3, stdout = $4, stderr = $5, execution_time_ms = $6, memory_usage_bytes = $7, started_at = $8, completed_at = $9
		WHERE id = $1
	`

	result, err := r.conn.Pool.Exec(ctx, query,
		execution.ID,
		execution.Status,
		execution.ReturnCode,
		execution.Stdout,
		execution.Stderr,
		execution.ExecutionTimeMs,
		execution.MemoryUsageBytes,
		execution.StartedAt,
		execution.CompletedAt,
	)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23514" {
			return fmt.Errorf("task execution validation failed: %s", pgErr.Detail)
		}
		return fmt.Errorf("failed to update task execution: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("task execution with ID %s not found", execution.ID)
	}

	return nil
}

// UpdateStatus updates only the status of a task execution
func (r *taskExecutionRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status models.ExecutionStatus) error {
	query := `
		UPDATE task_executions
		SET status = $2
		WHERE id = $1
	`

	result, err := r.conn.Pool.Exec(ctx, query, id, status)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23514" {
			return fmt.Errorf("invalid task execution status: %s", status)
		}
		return fmt.Errorf("failed to update task execution status: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("task execution with ID %s not found", id)
	}

	return nil
}

// Delete deletes a task execution
func (r *taskExecutionRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM task_executions WHERE id = $1`

	result, err := r.conn.Pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete task execution: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("task execution with ID %s not found", id)
	}

	return nil
}

// List retrieves task executions with pagination
func (r *taskExecutionRepository) List(ctx context.Context, limit, offset int) ([]*models.TaskExecution, error) {
	if limit <= 0 {
		limit = 10
	}
	if offset < 0 {
		offset = 0
	}

	query := `
		SELECT id, task_id, status, return_code, stdout, stderr, execution_time_ms, memory_usage_bytes, started_at, completed_at, created_at
		FROM task_executions
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := r.conn.Pool.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list task executions: %w", err)
	}
	defer rows.Close()

	return r.scanTaskExecutions(rows)
}

// Count returns the total number of task executions
func (r *taskExecutionRepository) Count(ctx context.Context) (int64, error) {
	query := `SELECT COUNT(*) FROM task_executions`

	var count int64
	err := r.conn.Pool.QueryRow(ctx, query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count task executions: %w", err)
	}

	return count, nil
}

// CountByTaskID returns the total number of task executions for a task
func (r *taskExecutionRepository) CountByTaskID(ctx context.Context, taskID uuid.UUID) (int64, error) {
	query := `SELECT COUNT(*) FROM task_executions WHERE task_id = $1`

	var count int64
	err := r.conn.Pool.QueryRow(ctx, query, taskID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count task executions by task ID: %w", err)
	}

	return count, nil
}

// CountByStatus returns the total number of task executions with a specific status
func (r *taskExecutionRepository) CountByStatus(ctx context.Context, status models.ExecutionStatus) (int64, error) {
	query := `SELECT COUNT(*) FROM task_executions WHERE status = $1`

	var count int64
	err := r.conn.Pool.QueryRow(ctx, query, status).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count task executions by status: %w", err)
	}

	return count, nil
}

// scanTaskExecutions is a helper function to scan task execution rows
func (r *taskExecutionRepository) scanTaskExecutions(rows pgx.Rows) ([]*models.TaskExecution, error) {
	var executions []*models.TaskExecution
	for rows.Next() {
		var execution models.TaskExecution
		err := rows.Scan(
			&execution.ID,
			&execution.TaskID,
			&execution.Status,
			&execution.ReturnCode,
			&execution.Stdout,
			&execution.Stderr,
			&execution.ExecutionTimeMs,
			&execution.MemoryUsageBytes,
			&execution.StartedAt,
			&execution.CompletedAt,
			&execution.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan task execution row: %w", err)
		}
		executions = append(executions, &execution)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating task execution rows: %w", err)
	}

	return executions, nil
}
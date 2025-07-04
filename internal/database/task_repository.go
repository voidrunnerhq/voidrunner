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

// taskRepository implements TaskRepository interface
type taskRepository struct {
	conn *Connection
}

// NewTaskRepository creates a new task repository
func NewTaskRepository(conn *Connection) TaskRepository {
	return &taskRepository{
		conn: conn,
	}
}

// Create creates a new task
func (r *taskRepository) Create(ctx context.Context, task *models.Task) error {
	if task == nil {
		return fmt.Errorf("task cannot be nil")
	}

	if task.ID == uuid.Nil {
		task.ID = models.NewID()
	}

	query := `
		INSERT INTO tasks (id, user_id, name, description, script_content, script_type, status, priority, timeout_seconds, metadata, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW(), NOW())
		RETURNING created_at, updated_at
	`

	err := r.conn.Pool.QueryRow(ctx, query,
		task.ID,
		task.UserID,
		task.Name,
		task.Description,
		task.ScriptContent,
		task.ScriptType,
		task.Status,
		task.Priority,
		task.TimeoutSeconds,
		task.Metadata,
	).Scan(&task.CreatedAt, &task.UpdatedAt)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Code {
			case "23505": // unique_violation
				return fmt.Errorf("task with ID %s already exists", task.ID)
			case "23503": // foreign_key_violation
				if strings.Contains(pgErr.Detail, "user_id") {
					return fmt.Errorf("user with ID %s does not exist", task.UserID)
				}
			case "23514": // check_violation
				return fmt.Errorf("task validation failed: %s", pgErr.Detail)
			}
		}
		return fmt.Errorf("failed to create task: %w", err)
	}

	return nil
}

// GetByID retrieves a task by ID
func (r *taskRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Task, error) {
	query := `
		SELECT id, user_id, name, description, script_content, script_type, status, priority, timeout_seconds, metadata, created_at, updated_at
		FROM tasks
		WHERE id = $1
	`

	var task models.Task
	err := r.conn.Pool.QueryRow(ctx, query, id).Scan(
		&task.ID,
		&task.UserID,
		&task.Name,
		&task.Description,
		&task.ScriptContent,
		&task.ScriptType,
		&task.Status,
		&task.Priority,
		&task.TimeoutSeconds,
		&task.Metadata,
		&task.CreatedAt,
		&task.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("task with ID %s not found", id)
		}
		return nil, fmt.Errorf("failed to get task by ID: %w", err)
	}

	return &task, nil
}

// GetByUserID retrieves tasks by user ID with pagination
func (r *taskRepository) GetByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*models.Task, error) {
	if limit <= 0 {
		limit = 10
	}
	if offset < 0 {
		offset = 0
	}

	query := `
		SELECT id, user_id, name, description, script_content, script_type, status, priority, timeout_seconds, metadata, created_at, updated_at
		FROM tasks
		WHERE user_id = $1
		ORDER BY priority DESC, created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.conn.Pool.Query(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get tasks by user ID: %w", err)
	}
	defer rows.Close()

	return r.scanTasks(rows)
}

// GetByStatus retrieves tasks by status with pagination
func (r *taskRepository) GetByStatus(ctx context.Context, status models.TaskStatus, limit, offset int) ([]*models.Task, error) {
	if limit <= 0 {
		limit = 10
	}
	if offset < 0 {
		offset = 0
	}

	query := `
		SELECT id, user_id, name, description, script_content, script_type, status, priority, timeout_seconds, metadata, created_at, updated_at
		FROM tasks
		WHERE status = $1
		ORDER BY priority DESC, created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.conn.Pool.Query(ctx, query, status, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get tasks by status: %w", err)
	}
	defer rows.Close()

	return r.scanTasks(rows)
}

// Update updates a task
func (r *taskRepository) Update(ctx context.Context, task *models.Task) error {
	if task == nil {
		return fmt.Errorf("task cannot be nil")
	}

	query := `
		UPDATE tasks
		SET name = $2, description = $3, script_content = $4, script_type = $5, status = $6, priority = $7, timeout_seconds = $8, metadata = $9, updated_at = NOW()
		WHERE id = $1
		RETURNING updated_at
	`

	err := r.conn.Pool.QueryRow(ctx, query,
		task.ID,
		task.Name,
		task.Description,
		task.ScriptContent,
		task.ScriptType,
		task.Status,
		task.Priority,
		task.TimeoutSeconds,
		task.Metadata,
	).Scan(&task.UpdatedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("task with ID %s not found", task.ID)
		}

		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Code {
			case "23514": // check_violation
				return fmt.Errorf("task validation failed: %s", pgErr.Detail)
			}
		}
		return fmt.Errorf("failed to update task: %w", err)
	}

	return nil
}

// UpdateStatus updates only the status of a task
func (r *taskRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status models.TaskStatus) error {
	query := `
		UPDATE tasks
		SET status = $2, updated_at = NOW()
		WHERE id = $1
	`

	result, err := r.conn.Pool.Exec(ctx, query, id, status)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23514" {
			return fmt.Errorf("invalid task status: %s", status)
		}
		return fmt.Errorf("failed to update task status: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("task with ID %s not found", id)
	}

	return nil
}

// Delete deletes a task
func (r *taskRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM tasks WHERE id = $1`

	result, err := r.conn.Pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete task: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("task with ID %s not found", id)
	}

	return nil
}

// List retrieves tasks with pagination
func (r *taskRepository) List(ctx context.Context, limit, offset int) ([]*models.Task, error) {
	if limit <= 0 {
		limit = 10
	}
	if offset < 0 {
		offset = 0
	}

	query := `
		SELECT id, user_id, name, description, script_content, script_type, status, priority, timeout_seconds, metadata, created_at, updated_at
		FROM tasks
		ORDER BY priority DESC, created_at DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := r.conn.Pool.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list tasks: %w", err)
	}
	defer rows.Close()

	return r.scanTasks(rows)
}

// Count returns the total number of tasks
func (r *taskRepository) Count(ctx context.Context) (int64, error) {
	query := `SELECT COUNT(*) FROM tasks`

	var count int64
	err := r.conn.Pool.QueryRow(ctx, query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count tasks: %w", err)
	}

	return count, nil
}

// CountByUserID returns the total number of tasks for a user
func (r *taskRepository) CountByUserID(ctx context.Context, userID uuid.UUID) (int64, error) {
	query := `SELECT COUNT(*) FROM tasks WHERE user_id = $1`

	var count int64
	err := r.conn.Pool.QueryRow(ctx, query, userID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count tasks by user ID: %w", err)
	}

	return count, nil
}

// CountByStatus returns the total number of tasks with a specific status
func (r *taskRepository) CountByStatus(ctx context.Context, status models.TaskStatus) (int64, error) {
	query := `SELECT COUNT(*) FROM tasks WHERE status = $1`

	var count int64
	err := r.conn.Pool.QueryRow(ctx, query, status).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count tasks by status: %w", err)
	}

	return count, nil
}

// SearchByMetadata searches tasks by metadata using JSON operators
func (r *taskRepository) SearchByMetadata(ctx context.Context, query string, limit, offset int) ([]*models.Task, error) {
	if limit <= 0 {
		limit = 10
	}
	if offset < 0 {
		offset = 0
	}

	sqlQuery := `
		SELECT id, user_id, name, description, script_content, script_type, status, priority, timeout_seconds, metadata, created_at, updated_at
		FROM tasks
		WHERE metadata @> $1
		ORDER BY priority DESC, created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.conn.Pool.Query(ctx, sqlQuery, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to search tasks by metadata: %w", err)
	}
	defer rows.Close()

	return r.scanTasks(rows)
}

// scanTasks is a helper function to scan task rows
func (r *taskRepository) scanTasks(rows pgx.Rows) ([]*models.Task, error) {
	var tasks []*models.Task
	for rows.Next() {
		var task models.Task
		err := rows.Scan(
			&task.ID,
			&task.UserID,
			&task.Name,
			&task.Description,
			&task.ScriptContent,
			&task.ScriptType,
			&task.Status,
			&task.Priority,
			&task.TimeoutSeconds,
			&task.Metadata,
			&task.CreatedAt,
			&task.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan task row: %w", err)
		}
		tasks = append(tasks, &task)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating task rows: %w", err)
	}

	return tasks, nil
}
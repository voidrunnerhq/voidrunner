package database

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/voidrunnerhq/voidrunner/internal/models"
)

// Querier interface for both *pgxpool.Pool and pgx.Tx
type Querier interface {
	Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row
	Exec(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error)
}

// taskRepository implements TaskRepository interface
type taskRepository struct {
	querier       Querier
	cursorEncoder *CursorEncoder
}

// NewTaskRepository creates a new task repository
func NewTaskRepository(conn *Connection) TaskRepository {
	return &taskRepository{
		querier:       conn.Pool,
		cursorEncoder: NewCursorEncoder(),
	}
}

// NewTaskRepositoryWithTx creates a new task repository with transaction
func NewTaskRepositoryWithTx(tx pgx.Tx) TaskRepository {
	return &taskRepository{
		querier:       tx,
		cursorEncoder: NewCursorEncoder(),
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

	err := r.querier.QueryRow(ctx, query,
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
	err := r.querier.QueryRow(ctx, query, id).Scan(
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

	rows, err := r.querier.Query(ctx, query, userID, limit, offset)
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

	rows, err := r.querier.Query(ctx, query, status, limit, offset)
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

	err := r.querier.QueryRow(ctx, query,
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

	result, err := r.querier.Exec(ctx, query, id, status)
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

	result, err := r.querier.Exec(ctx, query, id)
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

	rows, err := r.querier.Query(ctx, query, limit, offset)
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
	err := r.querier.QueryRow(ctx, query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count tasks: %w", err)
	}

	return count, nil
}

// CountByUserID returns the total number of tasks for a user
func (r *taskRepository) CountByUserID(ctx context.Context, userID uuid.UUID) (int64, error) {
	query := `SELECT COUNT(*) FROM tasks WHERE user_id = $1`

	var count int64
	err := r.querier.QueryRow(ctx, query, userID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count tasks by user ID: %w", err)
	}

	return count, nil
}

// CountByStatus returns the total number of tasks with a specific status
func (r *taskRepository) CountByStatus(ctx context.Context, status models.TaskStatus) (int64, error) {
	query := `SELECT COUNT(*) FROM tasks WHERE status = $1`

	var count int64
	err := r.querier.QueryRow(ctx, query, status).Scan(&count)
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

	rows, err := r.querier.Query(ctx, sqlQuery, query, limit, offset)
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

// GetByUserIDCursor retrieves tasks by user ID using cursor-based pagination
func (r *taskRepository) GetByUserIDCursor(ctx context.Context, userID uuid.UUID, req CursorPaginationRequest) ([]*models.Task, CursorPaginationResponse, error) {
	ValidatePaginationRequest(&req)

	var cursor *TaskCursor
	var err error

	// Decode cursor if provided
	if req.Cursor != nil {
		decodedCursor, err := r.cursorEncoder.DecodeTaskCursor(*req.Cursor)
		if err != nil {
			return nil, CursorPaginationResponse{}, fmt.Errorf("invalid cursor: %w", err)
		}
		cursor = &decodedCursor
	}

	// Build dynamic ORDER BY clause based on sort field
	orderClause := buildOrderByClause(req.SortField, req.SortOrder)

	whereClause, args := BuildTaskCursorWhere(cursor, req.SortOrder, req.SortField, &userID, nil)
	
	query := fmt.Sprintf(`
		SELECT id, user_id, name, description, script_content, script_type, status, priority, timeout_seconds, metadata, created_at, updated_at
		FROM tasks
		%s
		%s
		LIMIT $%d
	`, whereClause, orderClause, len(args)+1)

	args = append(args, req.Limit+1) // Fetch one extra to check if there are more results

	rows, err := r.querier.Query(ctx, query, args...)
	if err != nil {
		return nil, CursorPaginationResponse{}, fmt.Errorf("failed to get tasks by user ID with cursor: %w", err)
	}
	defer rows.Close()

	tasks, err := r.scanTasks(rows)
	if err != nil {
		return nil, CursorPaginationResponse{}, err
	}

	// Build pagination response
	response := CursorPaginationResponse{
		HasMore: len(tasks) > req.Limit,
	}

	// Remove extra task if we fetched more than requested
	if response.HasMore {
		tasks = tasks[:req.Limit]
	}

	// Generate next cursor if there are more results
	if response.HasMore && len(tasks) > 0 {
		lastTask := tasks[len(tasks)-1]
		nextCursor := CreateTaskCursor(lastTask.ID, lastTask.CreatedAt, &lastTask.Priority)
		encoded, err := r.cursorEncoder.EncodeTaskCursor(nextCursor)
		if err != nil {
			return nil, CursorPaginationResponse{}, fmt.Errorf("failed to encode next cursor: %w", err)
		}
		response.NextCursor = &encoded
	}

	return tasks, response, nil
}

// GetByStatusCursor retrieves tasks by status using cursor-based pagination
func (r *taskRepository) GetByStatusCursor(ctx context.Context, status models.TaskStatus, req CursorPaginationRequest) ([]*models.Task, CursorPaginationResponse, error) {
	ValidatePaginationRequest(&req)

	var cursor *TaskCursor
	var err error

	// Decode cursor if provided
	if req.Cursor != nil {
		decodedCursor, err := r.cursorEncoder.DecodeTaskCursor(*req.Cursor)
		if err != nil {
			return nil, CursorPaginationResponse{}, fmt.Errorf("invalid cursor: %w", err)
		}
		cursor = &decodedCursor
	}

	// Build dynamic ORDER BY clause based on sort field
	orderClause := buildOrderByClause(req.SortField, req.SortOrder)

	statusStr := string(status)
	whereClause, args := BuildTaskCursorWhere(cursor, req.SortOrder, req.SortField, nil, &statusStr)
	
	query := fmt.Sprintf(`
		SELECT id, user_id, name, description, script_content, script_type, status, priority, timeout_seconds, metadata, created_at, updated_at
		FROM tasks
		%s
		%s
		LIMIT $%d
	`, whereClause, orderClause, len(args)+1)

	args = append(args, req.Limit+1)

	rows, err := r.querier.Query(ctx, query, args...)
	if err != nil {
		return nil, CursorPaginationResponse{}, fmt.Errorf("failed to get tasks by status with cursor: %w", err)
	}
	defer rows.Close()

	tasks, err := r.scanTasks(rows)
	if err != nil {
		return nil, CursorPaginationResponse{}, err
	}

	// Build pagination response
	response := CursorPaginationResponse{
		HasMore: len(tasks) > req.Limit,
	}

	if response.HasMore {
		tasks = tasks[:req.Limit]
	}

	if response.HasMore && len(tasks) > 0 {
		lastTask := tasks[len(tasks)-1]
		nextCursor := CreateTaskCursor(lastTask.ID, lastTask.CreatedAt, &lastTask.Priority)
		encoded, err := r.cursorEncoder.EncodeTaskCursor(nextCursor)
		if err != nil {
			return nil, CursorPaginationResponse{}, fmt.Errorf("failed to encode next cursor: %w", err)
		}
		response.NextCursor = &encoded
	}

	return tasks, response, nil
}

// ListCursor retrieves all tasks using cursor-based pagination
func (r *taskRepository) ListCursor(ctx context.Context, req CursorPaginationRequest) ([]*models.Task, CursorPaginationResponse, error) {
	ValidatePaginationRequest(&req)

	var cursor *TaskCursor
	var err error

	// Decode cursor if provided
	if req.Cursor != nil {
		decodedCursor, err := r.cursorEncoder.DecodeTaskCursor(*req.Cursor)
		if err != nil {
			return nil, CursorPaginationResponse{}, fmt.Errorf("invalid cursor: %w", err)
		}
		cursor = &decodedCursor
	}

	// Build dynamic ORDER BY clause based on sort field
	orderClause := buildOrderByClause(req.SortField, req.SortOrder)

	whereClause, args := BuildTaskCursorWhere(cursor, req.SortOrder, req.SortField, nil, nil)
	
	query := fmt.Sprintf(`
		SELECT id, user_id, name, description, script_content, script_type, status, priority, timeout_seconds, metadata, created_at, updated_at
		FROM tasks
		%s
		%s
		LIMIT $%d
	`, whereClause, orderClause, len(args)+1)

	args = append(args, req.Limit+1)

	rows, err := r.querier.Query(ctx, query, args...)
	if err != nil {
		return nil, CursorPaginationResponse{}, fmt.Errorf("failed to list tasks with cursor: %w", err)
	}
	defer rows.Close()

	tasks, err := r.scanTasks(rows)
	if err != nil {
		return nil, CursorPaginationResponse{}, err
	}

	// Build pagination response
	response := CursorPaginationResponse{
		HasMore: len(tasks) > req.Limit,
	}

	if response.HasMore {
		tasks = tasks[:req.Limit]
	}

	if response.HasMore && len(tasks) > 0 {
		lastTask := tasks[len(tasks)-1]
		nextCursor := CreateTaskCursor(lastTask.ID, lastTask.CreatedAt, &lastTask.Priority)
		encoded, err := r.cursorEncoder.EncodeTaskCursor(nextCursor)
		if err != nil {
			return nil, CursorPaginationResponse{}, fmt.Errorf("failed to encode next cursor: %w", err)
		}
		response.NextCursor = &encoded
	}

	return tasks, response, nil
}

// GetTasksWithExecutionCount retrieves tasks with their execution count using a single optimized query
func (r *taskRepository) GetTasksWithExecutionCount(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*models.Task, error) {
	if limit <= 0 {
		limit = 10
	}
	if offset < 0 {
		offset = 0
	}

	query := `
		SELECT 
			t.id, t.user_id, t.name, t.description, t.script_content, t.script_type, 
			t.status, t.priority, t.timeout_seconds, t.metadata, t.created_at, t.updated_at,
			COALESCE(COUNT(e.id), 0) as execution_count
		FROM tasks t
		LEFT JOIN task_executions e ON t.id = e.task_id
		WHERE t.user_id = $1
		GROUP BY t.id, t.user_id, t.name, t.description, t.script_content, t.script_type, 
				 t.status, t.priority, t.timeout_seconds, t.metadata, t.created_at, t.updated_at
		ORDER BY t.priority DESC, t.created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.querier.Query(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get tasks with execution count: %w", err)
	}
	defer rows.Close()

	var tasks []*models.Task
	for rows.Next() {
		var task models.Task
		var executionCount int64
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
			&executionCount,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan task with execution count: %w", err)
		}
		
		// Add execution count to metadata for now (could be added to model later)
		metadata := make(map[string]interface{})
		if task.Metadata != nil {
			json.Unmarshal(task.Metadata, &metadata)
		}
		metadata["execution_count"] = executionCount
		
		updatedMetadata, err := json.Marshal(metadata)
		if err == nil {
			task.Metadata = updatedMetadata
		}
		
		tasks = append(tasks, &task)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating task rows with execution count: %w", err)
	}

	return tasks, nil
}

// GetTasksWithLatestExecution retrieves tasks with their latest execution using a single optimized query
func (r *taskRepository) GetTasksWithLatestExecution(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*models.Task, error) {
	if limit <= 0 {
		limit = 10
	}
	if offset < 0 {
		offset = 0
	}

	query := `
		SELECT 
			t.id, t.user_id, t.name, t.description, t.script_content, t.script_type, 
			t.status, t.priority, t.timeout_seconds, t.metadata, t.created_at, t.updated_at,
			e.id as latest_execution_id, e.status as latest_execution_status, 
			e.created_at as latest_execution_created_at
		FROM tasks t
		LEFT JOIN LATERAL (
			SELECT id, status, created_at
			FROM task_executions
			WHERE task_id = t.id
			ORDER BY created_at DESC
			LIMIT 1
		) e ON true
		WHERE t.user_id = $1
		ORDER BY t.priority DESC, t.created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.querier.Query(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get tasks with latest execution: %w", err)
	}
	defer rows.Close()

	var tasks []*models.Task
	for rows.Next() {
		var task models.Task
		var latestExecutionID *uuid.UUID
		var latestExecutionStatus *string
		var latestExecutionCreatedAt *time.Time
		
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
			&latestExecutionID,
			&latestExecutionStatus,
			&latestExecutionCreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan task with latest execution: %w", err)
		}
		
		// Add latest execution info to metadata
		metadata := make(map[string]interface{})
		if task.Metadata != nil {
			json.Unmarshal(task.Metadata, &metadata)
		}
		
		if latestExecutionID != nil {
			metadata["latest_execution"] = map[string]interface{}{
				"id":         *latestExecutionID,
				"status":     *latestExecutionStatus,
				"created_at": *latestExecutionCreatedAt,
			}
		}
		
		updatedMetadata, err := json.Marshal(metadata)
		if err == nil {
			task.Metadata = updatedMetadata
		}
		
		tasks = append(tasks, &task)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating task rows with latest execution: %w", err)
	}

	return tasks, nil
}

// buildOrderByClause creates the ORDER BY clause based on sort field and order
func buildOrderByClause(sortField string, sortOrder string) string {
	direction := "DESC"
	if sortOrder == "asc" {
		direction = "ASC"
	}
	
	switch sortField {
	case "priority":
		// Sort by priority first, then created_at, then id for consistent ordering
		return fmt.Sprintf("ORDER BY priority %s, created_at %s, id %s", direction, direction, direction)
	case "created_at":
		// Sort by created_at, then id
		return fmt.Sprintf("ORDER BY created_at %s, id %s", direction, direction)
	case "updated_at":
		// Sort by updated_at, then id
		return fmt.Sprintf("ORDER BY updated_at %s, id %s", direction, direction)
	case "name":
		// Sort by name, then created_at, then id
		return fmt.Sprintf("ORDER BY name %s, created_at %s, id %s", direction, direction, direction)
	default:
		// Default to created_at
		return fmt.Sprintf("ORDER BY created_at %s, id %s", direction, direction)
	}
}
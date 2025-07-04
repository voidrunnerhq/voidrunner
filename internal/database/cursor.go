package database

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// CursorEncoder handles encoding and decoding of cursors
type CursorEncoder struct{}

// NewCursorEncoder creates a new cursor encoder
func NewCursorEncoder() *CursorEncoder {
	return &CursorEncoder{}
}

// EncodeTaskCursor encodes a task cursor to a base64 string
func (ce *CursorEncoder) EncodeTaskCursor(cursor TaskCursor) (string, error) {
	data, err := json.Marshal(cursor)
	if err != nil {
		return "", fmt.Errorf("failed to marshal task cursor: %w", err)
	}
	return base64.URLEncoding.EncodeToString(data), nil
}

// DecodeTaskCursor decodes a base64 string to a task cursor
func (ce *CursorEncoder) DecodeTaskCursor(encoded string) (TaskCursor, error) {
	if encoded == "" {
		return TaskCursor{}, ErrInvalidCursor
	}

	data, err := base64.URLEncoding.DecodeString(encoded)
	if err != nil {
		return TaskCursor{}, fmt.Errorf("failed to decode cursor: %w", err)
	}

	var cursor TaskCursor
	if err := json.Unmarshal(data, &cursor); err != nil {
		return TaskCursor{}, fmt.Errorf("failed to unmarshal task cursor: %w", err)
	}

	// Validate cursor fields
	if cursor.ID == uuid.Nil || cursor.CreatedAt.IsZero() {
		return TaskCursor{}, ErrInvalidCursor
	}

	return cursor, nil
}

// EncodeExecutionCursor encodes an execution cursor to a base64 string
func (ce *CursorEncoder) EncodeExecutionCursor(cursor ExecutionCursor) (string, error) {
	data, err := json.Marshal(cursor)
	if err != nil {
		return "", fmt.Errorf("failed to marshal execution cursor: %w", err)
	}
	return base64.URLEncoding.EncodeToString(data), nil
}

// DecodeExecutionCursor decodes a base64 string to an execution cursor
func (ce *CursorEncoder) DecodeExecutionCursor(encoded string) (ExecutionCursor, error) {
	if encoded == "" {
		return ExecutionCursor{}, ErrInvalidCursor
	}

	data, err := base64.URLEncoding.DecodeString(encoded)
	if err != nil {
		return ExecutionCursor{}, fmt.Errorf("failed to decode cursor: %w", err)
	}

	var cursor ExecutionCursor
	if err := json.Unmarshal(data, &cursor); err != nil {
		return ExecutionCursor{}, fmt.Errorf("failed to unmarshal execution cursor: %w", err)
	}

	// Validate cursor fields
	if cursor.ID == uuid.Nil || cursor.CreatedAt.IsZero() {
		return ExecutionCursor{}, ErrInvalidCursor
	}

	return cursor, nil
}

// CreateTaskCursor creates a task cursor from a task
func CreateTaskCursor(id uuid.UUID, createdAt time.Time, priority *int) TaskCursor {
	return TaskCursor{
		ID:        id,
		CreatedAt: createdAt,
		Priority:  priority,
	}
}

// CreateExecutionCursor creates an execution cursor from an execution
func CreateExecutionCursor(id uuid.UUID, createdAt time.Time) ExecutionCursor {
	return ExecutionCursor{
		ID:        id,
		CreatedAt: createdAt,
	}
}

// ValidatePaginationRequest validates and sets defaults for pagination request
func ValidatePaginationRequest(req *CursorPaginationRequest) {
	// Set default limit
	if req.Limit <= 0 {
		req.Limit = 20
	}
	
	// Cap maximum limit
	if req.Limit > 100 {
		req.Limit = 100
	}

	// Set default sort order
	if req.SortOrder == "" {
		req.SortOrder = "desc"
	}

	// Validate sort order
	if req.SortOrder != "asc" && req.SortOrder != "desc" {
		req.SortOrder = "desc"
	}
}

// BuildTaskCursorWhere builds WHERE clause for cursor-based pagination
func BuildTaskCursorWhere(cursor *TaskCursor, sortOrder string, userID *uuid.UUID, status *string) (string, []interface{}) {
	var conditions []string
	var args []interface{}
	argIndex := 1

	// Add user filter if provided
	if userID != nil {
		conditions = append(conditions, fmt.Sprintf("user_id = $%d", argIndex))
		args = append(args, *userID)
		argIndex++
	}

	// Add status filter if provided
	if status != nil {
		conditions = append(conditions, fmt.Sprintf("status = $%d", argIndex))
		args = append(args, *status)
		argIndex++
	}

	// Add cursor condition if provided
	if cursor != nil {
		if sortOrder == "asc" {
			// For ascending order: created_at > cursor.created_at OR (created_at = cursor.created_at AND id > cursor.id)
			conditions = append(conditions, fmt.Sprintf("(created_at > $%d OR (created_at = $%d AND id > $%d))", argIndex, argIndex, argIndex+1))
		} else {
			// For descending order: created_at < cursor.created_at OR (created_at = cursor.created_at AND id < cursor.id)
			conditions = append(conditions, fmt.Sprintf("(created_at < $%d OR (created_at = $%d AND id < $%d))", argIndex, argIndex, argIndex+1))
		}
		args = append(args, cursor.CreatedAt, cursor.CreatedAt, cursor.ID)
		argIndex += 3
	}

	if len(conditions) == 0 {
		return "", args
	}

	whereClause := "WHERE " + conditions[0]
	for i := 1; i < len(conditions); i++ {
		whereClause += " AND " + conditions[i]
	}
	return whereClause, args
}

// BuildExecutionCursorWhere builds WHERE clause for execution cursor-based pagination
func BuildExecutionCursorWhere(cursor *ExecutionCursor, sortOrder string, taskID *uuid.UUID, status *string) (string, []interface{}) {
	var conditions []string
	var args []interface{}
	argIndex := 1

	// Add task filter if provided
	if taskID != nil {
		conditions = append(conditions, fmt.Sprintf("task_id = $%d", argIndex))
		args = append(args, *taskID)
		argIndex++
	}

	// Add status filter if provided
	if status != nil {
		conditions = append(conditions, fmt.Sprintf("status = $%d", argIndex))
		args = append(args, *status)
		argIndex++
	}

	// Add cursor condition if provided
	if cursor != nil {
		if sortOrder == "asc" {
			conditions = append(conditions, fmt.Sprintf("(created_at > $%d OR (created_at = $%d AND id > $%d))", argIndex, argIndex, argIndex+1))
		} else {
			conditions = append(conditions, fmt.Sprintf("(created_at < $%d OR (created_at = $%d AND id < $%d))", argIndex, argIndex, argIndex+1))
		}
		args = append(args, cursor.CreatedAt, cursor.CreatedAt, cursor.ID)
		argIndex += 3
	}

	if len(conditions) == 0 {
		return "", args
	}

	whereClause := "WHERE " + conditions[0]
	for i := 1; i < len(conditions); i++ {
		whereClause += " AND " + conditions[i]
	}
	return whereClause, args
}
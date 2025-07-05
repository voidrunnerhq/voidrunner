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

	// Set default sort field
	if req.SortField == "" {
		req.SortField = "created_at"
	}

	// Validate sort field
	validSortFields := map[string]bool{
		"created_at": true,
		"updated_at": true,
		"priority":   true,
		"name":       true,
	}
	if !validSortFields[req.SortField] {
		req.SortField = "created_at"
	}
}

// BuildTaskCursorWhere builds WHERE clause for cursor-based pagination
func BuildTaskCursorWhere(cursor *TaskCursor, sortOrder string, sortField string, userID *uuid.UUID, status *string) (string, []interface{}) {
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
		cursorCondition, cursorArgs := buildCursorCondition(cursor, sortOrder, sortField, argIndex)
		if cursorCondition != "" {
			conditions = append(conditions, cursorCondition)
			args = append(args, cursorArgs...)
			argIndex += len(cursorArgs)
		}
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

// buildCursorCondition builds the cursor comparison condition based on sort field
func buildCursorCondition(cursor *TaskCursor, sortOrder string, sortField string, startArgIndex int) (string, []interface{}) {
	var args []interface{}
	var condition string
	
	// Determine comparison operators based on sort order
	primaryOp, secondaryOp := "<", "<"
	if sortOrder == "asc" {
		primaryOp, secondaryOp = ">", ">"
	}
	
	argIndex := startArgIndex
	
	switch sortField {
	case "priority":
		if cursor.Priority == nil {
			// Cannot use priority cursor without priority value, fallback to created_at
			condition = fmt.Sprintf("(created_at %s $%d OR (created_at = $%d AND id %s $%d))", 
				primaryOp, argIndex, argIndex, secondaryOp, argIndex+1)
			args = append(args, cursor.CreatedAt, cursor.CreatedAt, cursor.ID)
		} else {
			// Priority-based cursor: priority, then created_at, then id
			condition = fmt.Sprintf(`(priority %s $%d OR 
				(priority = $%d AND created_at %s $%d) OR 
				(priority = $%d AND created_at = $%d AND id %s $%d))`,
				primaryOp, argIndex,     // priority comparison
				argIndex, primaryOp, argIndex+1,  // priority = and created_at comparison  
				argIndex, argIndex+1, secondaryOp, argIndex+2) // priority = and created_at = and id comparison
			args = append(args, *cursor.Priority, *cursor.Priority, cursor.CreatedAt, 
				*cursor.Priority, cursor.CreatedAt, cursor.ID)
		}
		
	case "created_at":
		// Created_at-based cursor: created_at, then id
		condition = fmt.Sprintf("(created_at %s $%d OR (created_at = $%d AND id %s $%d))", 
			primaryOp, argIndex, argIndex, secondaryOp, argIndex+1)
		args = append(args, cursor.CreatedAt, cursor.CreatedAt, cursor.ID)
		
	case "updated_at":
		// Updated_at-based cursor: updated_at, then id (using created_at as proxy for updated_at in cursor)
		condition = fmt.Sprintf("(updated_at %s $%d OR (updated_at = $%d AND id %s $%d))", 
			primaryOp, argIndex, argIndex, secondaryOp, argIndex+1)
		args = append(args, cursor.CreatedAt, cursor.CreatedAt, cursor.ID)
		
	case "name":
		// Name-based cursor: name, then created_at, then id (using created_at as proxy for name in cursor)
		condition = fmt.Sprintf(`(name %s $%d OR 
			(name = $%d AND created_at %s $%d) OR 
			(name = $%d AND created_at = $%d AND id %s $%d))`,
			primaryOp, argIndex,     // name comparison
			argIndex, primaryOp, argIndex+1,  // name = and created_at comparison
			argIndex, argIndex+1, secondaryOp, argIndex+2) // name = and created_at = and id comparison
		// Note: For name sorting, we'd need to store the name in the cursor too
		// For now, fallback to created_at-based sorting
		condition = fmt.Sprintf("(created_at %s $%d OR (created_at = $%d AND id %s $%d))", 
			primaryOp, argIndex, argIndex, secondaryOp, argIndex+1)
		args = append(args, cursor.CreatedAt, cursor.CreatedAt, cursor.ID)
		
	default:
		// Default to created_at-based cursor
		condition = fmt.Sprintf("(created_at %s $%d OR (created_at = $%d AND id %s $%d))", 
			primaryOp, argIndex, argIndex, secondaryOp, argIndex+1)
		args = append(args, cursor.CreatedAt, cursor.CreatedAt, cursor.ID)
	}
	
	return condition, args
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
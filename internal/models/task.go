package models

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// TaskStatus represents the status of a task
type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
	TaskStatusTimeout   TaskStatus = "timeout"
	TaskStatusCancelled TaskStatus = "cancelled"
)

// ScriptType represents the type of script
type ScriptType string

const (
	ScriptTypePython     ScriptType = "python"
	ScriptTypeJavaScript ScriptType = "javascript"
	ScriptTypeBash       ScriptType = "bash"
	ScriptTypeGo         ScriptType = "go"
)

// Task represents a task in the system
type Task struct {
	BaseModel
	UserID         uuid.UUID  `json:"user_id" db:"user_id"`
	Name           string     `json:"name" db:"name"`
	Description    *string    `json:"description,omitempty" db:"description"`
	ScriptContent  string     `json:"script_content" db:"script_content"`
	ScriptType     ScriptType `json:"script_type" db:"script_type"`
	Status         TaskStatus `json:"status" db:"status"`
	Priority       int        `json:"priority" db:"priority"`
	TimeoutSeconds int        `json:"timeout_seconds" db:"timeout_seconds"`
	Metadata       JSONB      `json:"metadata" db:"metadata"`
}

// CreateTaskRequest represents the request to create a new task
type CreateTaskRequest struct {
	Name           string     `json:"name" validate:"required,task_name,min=1,max=255"`
	Description    *string    `json:"description,omitempty" validate:"omitempty,max=1000"`
	ScriptContent  string     `json:"script_content" validate:"required,script_content,min=1,max=65535"`
	ScriptType     ScriptType `json:"script_type" validate:"required,script_type"`
	Priority       *int       `json:"priority,omitempty" validate:"omitempty,min=0,max=10"`
	TimeoutSeconds *int       `json:"timeout_seconds,omitempty" validate:"omitempty,min=1,max=3600"`
	Metadata       JSONB      `json:"metadata,omitempty"`
}

// UpdateTaskRequest represents the request to update a task
type UpdateTaskRequest struct {
	Name           *string     `json:"name,omitempty" validate:"omitempty,task_name,min=1,max=255"`
	Description    *string     `json:"description,omitempty" validate:"omitempty,max=1000"`
	ScriptContent  *string     `json:"script_content,omitempty" validate:"omitempty,script_content,min=1,max=65535"`
	ScriptType     *ScriptType `json:"script_type,omitempty" validate:"omitempty,script_type"`
	Priority       *int        `json:"priority,omitempty" validate:"omitempty,min=0,max=10"`
	TimeoutSeconds *int        `json:"timeout_seconds,omitempty" validate:"omitempty,min=1,max=3600"`
	Metadata       JSONB       `json:"metadata,omitempty"`
}

// TaskResponse represents the task response
type TaskResponse struct {
	ID             uuid.UUID  `json:"id"`
	UserID         uuid.UUID  `json:"user_id"`
	Name           string     `json:"name"`
	Description    *string    `json:"description,omitempty"`
	ScriptContent  string     `json:"script_content"`
	ScriptType     ScriptType `json:"script_type"`
	Status         TaskStatus `json:"status"`
	Priority       int        `json:"priority"`
	TimeoutSeconds int        `json:"timeout_seconds"`
	Metadata       JSONB      `json:"metadata"`
	CreatedAt      string     `json:"created_at"`
	UpdatedAt      string     `json:"updated_at"`
}

// ToResponse converts Task to TaskResponse
func (t *Task) ToResponse() TaskResponse {
	return TaskResponse{
		ID:             t.ID,
		UserID:         t.UserID,
		Name:           t.Name,
		Description:    t.Description,
		ScriptContent:  t.ScriptContent,
		ScriptType:     t.ScriptType,
		Status:         t.Status,
		Priority:       t.Priority,
		TimeoutSeconds: t.TimeoutSeconds,
		Metadata:       t.Metadata,
		CreatedAt:      t.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:      t.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

// ValidateTaskName validates the task name
func ValidateTaskName(name string) error {
	if name == "" {
		return fmt.Errorf("task name is required")
	}

	name = strings.TrimSpace(name)
	if len(name) == 0 {
		return fmt.Errorf("task name cannot be empty")
	}

	if len(name) > 255 {
		return fmt.Errorf("task name is too long (max 255 characters)")
	}

	return nil
}

// ValidateScriptType validates the script type
func ValidateScriptType(scriptType ScriptType) error {
	switch scriptType {
	case ScriptTypePython, ScriptTypeJavaScript, ScriptTypeBash, ScriptTypeGo:
		return nil
	default:
		return fmt.Errorf("invalid script type: %s", scriptType)
	}
}

// ValidateScriptContent validates the script content
func ValidateScriptContent(content string) error {
	if content == "" {
		return fmt.Errorf("script content is required")
	}

	content = strings.TrimSpace(content)
	if len(content) == 0 {
		return fmt.Errorf("script content cannot be empty")
	}

	if len(content) > 65535 {
		return fmt.Errorf("script content is too long (max 65535 characters)")
	}

	// Basic security checks
	if strings.Contains(strings.ToLower(content), "rm -rf") {
		return fmt.Errorf("potentially dangerous script content detected")
	}

	return nil
}

// ValidateTaskStatus validates the task status
func ValidateTaskStatus(status TaskStatus) error {
	switch status {
	case TaskStatusPending, TaskStatusRunning, TaskStatusCompleted, TaskStatusFailed, TaskStatusTimeout, TaskStatusCancelled:
		return nil
	default:
		return fmt.Errorf("invalid task status: %s", status)
	}
}

// ValidatePriority validates the priority value
func ValidatePriority(priority int) error {
	if priority < 0 || priority > 10 {
		return fmt.Errorf("priority must be between 0 and 10")
	}
	return nil
}

// ValidateTimeout validates the timeout value
func ValidateTimeout(timeout int) error {
	if timeout <= 0 {
		return fmt.Errorf("timeout must be greater than 0")
	}
	if timeout > 3600 {
		return fmt.Errorf("timeout cannot exceed 3600 seconds")
	}
	return nil
}

// TaskListResponse represents the response for listing tasks
type TaskListResponse struct {
	Tasks  []TaskResponse `json:"tasks"`
	Total  int64          `json:"total"`
	Limit  int            `json:"limit"`
	Offset int            `json:"offset"`
}

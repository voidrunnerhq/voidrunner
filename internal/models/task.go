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

// State transition definitions for task status
var taskStatusTransitions = map[TaskStatus][]TaskStatus{
	TaskStatusPending: {
		TaskStatusRunning,
		TaskStatusCancelled,
	},
	TaskStatusRunning: {
		TaskStatusCompleted,
		TaskStatusFailed,
		TaskStatusTimeout,
		TaskStatusCancelled,
	},
	TaskStatusCompleted: {
		// Terminal state - no transitions allowed
	},
	TaskStatusFailed: {
		TaskStatusPending, // Allow retry by resetting to pending
	},
	TaskStatusTimeout: {
		TaskStatusPending, // Allow retry by resetting to pending
	},
	TaskStatusCancelled: {
		TaskStatusPending, // Allow restart by resetting to pending
	},
}

// ValidateTaskStatusTransition validates if a status transition is allowed
func ValidateTaskStatusTransition(currentStatus, newStatus TaskStatus) error {
	// Validate both statuses are valid
	if err := ValidateTaskStatus(currentStatus); err != nil {
		return fmt.Errorf("invalid current status: %w", err)
	}
	
	if err := ValidateTaskStatus(newStatus); err != nil {
		return fmt.Errorf("invalid new status: %w", err)
	}

	// Allow staying in the same status (idempotent updates)
	if currentStatus == newStatus {
		return nil
	}

	// Check if transition is allowed
	allowedTransitions, exists := taskStatusTransitions[currentStatus]
	if !exists {
		return fmt.Errorf("no transitions defined for status: %s", currentStatus)
	}

	for _, allowedStatus := range allowedTransitions {
		if newStatus == allowedStatus {
			return nil // Transition is allowed
		}
	}

	return fmt.Errorf("invalid status transition from %s to %s", currentStatus, newStatus)
}

// IsTerminalStatus returns true if the status is terminal (no further transitions)
func (t *Task) IsTerminalStatus() bool {
	return IsTaskStatusTerminal(t.Status)
}

// IsTaskStatusTerminal returns true if the given status is terminal
func IsTaskStatusTerminal(status TaskStatus) bool {
	switch status {
	case TaskStatusCompleted, TaskStatusFailed, TaskStatusTimeout, TaskStatusCancelled:
		return true
	default:
		return false
	}
}

// IsRetryableStatus returns true if the task can be retried from this status
func (t *Task) IsRetryableStatus() bool {
	return IsTaskStatusRetryable(t.Status)
}

// IsTaskStatusRetryable returns true if the given status allows retry
func IsTaskStatusRetryable(status TaskStatus) bool {
	switch status {
	case TaskStatusFailed, TaskStatusTimeout:
		return true
	default:
		return false
	}
}

// CanExecute returns true if the task can be executed (not already running or completed)
func (t *Task) CanExecute() bool {
	switch t.Status {
	case TaskStatusPending:
		return true
	case TaskStatusFailed, TaskStatusTimeout, TaskStatusCancelled:
		return true // Can be retried
	default:
		return false
	}
}

// IsRunning returns true if the task is currently running
func (t *Task) IsRunning() bool {
	return t.Status == TaskStatusRunning
}

// IsPending returns true if the task is pending execution
func (t *Task) IsPending() bool {
	return t.Status == TaskStatusPending
}

// IsCompleted returns true if the task completed successfully
func (t *Task) IsCompleted() bool {
	return t.Status == TaskStatusCompleted
}

// HasFailed returns true if the task failed, timed out, or was cancelled
func (t *Task) HasFailed() bool {
	switch t.Status {
	case TaskStatusFailed, TaskStatusTimeout, TaskStatusCancelled:
		return true
	default:
		return false
	}
}

// GetAllowedTransitions returns all allowed status transitions from current status
func (t *Task) GetAllowedTransitions() []TaskStatus {
	allowedTransitions, exists := taskStatusTransitions[t.Status]
	if !exists {
		return []TaskStatus{}
	}
	
	// Return a copy to prevent modification
	result := make([]TaskStatus, len(allowedTransitions))
	copy(result, allowedTransitions)
	return result
}

// CanTransitionTo checks if the task can transition to the given status
func (t *Task) CanTransitionTo(newStatus TaskStatus) bool {
	return ValidateTaskStatusTransition(t.Status, newStatus) == nil
}

// TransitionTo attempts to transition the task to a new status with validation
func (t *Task) TransitionTo(newStatus TaskStatus) error {
	if err := ValidateTaskStatusTransition(t.Status, newStatus); err != nil {
		return err
	}
	
	oldStatus := t.Status
	t.Status = newStatus
	
	// Log the transition for debugging
	// Note: In a real system, you might want to inject a logger here
	fmt.Printf("Task %s transitioned from %s to %s\n", t.ID, oldStatus, newStatus)
	
	return nil
}

// StatusTransitionInfo provides information about a status transition
type StatusTransitionInfo struct {
	FromStatus      TaskStatus `json:"from_status"`
	ToStatus        TaskStatus `json:"to_status"`
	IsValid         bool       `json:"is_valid"`
	IsTerminal      bool       `json:"is_terminal"`
	IsRetryable     bool       `json:"is_retryable"`
	ErrorMessage    string     `json:"error_message,omitempty"`
}

// GetStatusTransitionInfo returns detailed information about a potential status transition
func GetStatusTransitionInfo(fromStatus, toStatus TaskStatus) *StatusTransitionInfo {
	info := &StatusTransitionInfo{
		FromStatus: fromStatus,
		ToStatus:   toStatus,
		IsTerminal: IsTaskStatusTerminal(toStatus),
		IsRetryable: IsTaskStatusRetryable(fromStatus),
	}
	
	err := ValidateTaskStatusTransition(fromStatus, toStatus)
	if err != nil {
		info.IsValid = false
		info.ErrorMessage = err.Error()
	} else {
		info.IsValid = true
	}
	
	return info
}

// GetAllTaskStatusTransitions returns all valid transitions for all statuses
func GetAllTaskStatusTransitions() map[TaskStatus][]TaskStatus {
	// Return a deep copy to prevent modification
	result := make(map[TaskStatus][]TaskStatus)
	for status, transitions := range taskStatusTransitions {
		transitionsCopy := make([]TaskStatus, len(transitions))
		copy(transitionsCopy, transitions)
		result[status] = transitionsCopy
	}
	return result
}

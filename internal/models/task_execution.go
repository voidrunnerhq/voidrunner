package models

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ExecutionStatus represents the status of a task execution
type ExecutionStatus string

const (
	ExecutionStatusPending   ExecutionStatus = "pending"
	ExecutionStatusRunning   ExecutionStatus = "running"
	ExecutionStatusCompleted ExecutionStatus = "completed"
	ExecutionStatusFailed    ExecutionStatus = "failed"
	ExecutionStatusTimeout   ExecutionStatus = "timeout"
	ExecutionStatusCancelled ExecutionStatus = "cancelled"
)

// TaskExecution represents a task execution in the system
type TaskExecution struct {
	ID               uuid.UUID        `json:"id" db:"id"`
	TaskID           uuid.UUID        `json:"task_id" db:"task_id"`
	Status           ExecutionStatus  `json:"status" db:"status"`
	ReturnCode       *int             `json:"return_code,omitempty" db:"return_code"`
	Stdout           *string          `json:"stdout,omitempty" db:"stdout"`
	Stderr           *string          `json:"stderr,omitempty" db:"stderr"`
	ExecutionTimeMs  *int             `json:"execution_time_ms,omitempty" db:"execution_time_ms"`
	MemoryUsageBytes *int64           `json:"memory_usage_bytes,omitempty" db:"memory_usage_bytes"`
	StartedAt        *time.Time       `json:"started_at,omitempty" db:"started_at"`
	CompletedAt      *time.Time       `json:"completed_at,omitempty" db:"completed_at"`
	CreatedAt        time.Time        `json:"created_at" db:"created_at"`
}

// CreateTaskExecutionRequest represents the request to create a new task execution
type CreateTaskExecutionRequest struct {
	TaskID uuid.UUID `json:"task_id" validate:"required"`
}

// UpdateTaskExecutionRequest represents the request to update a task execution
type UpdateTaskExecutionRequest struct {
	Status           *ExecutionStatus `json:"status,omitempty"`
	ReturnCode       *int             `json:"return_code,omitempty" validate:"omitempty,min=0,max=255"`
	Stdout           *string          `json:"stdout,omitempty"`
	Stderr           *string          `json:"stderr,omitempty"`
	ExecutionTimeMs  *int             `json:"execution_time_ms,omitempty" validate:"omitempty,min=0"`
	MemoryUsageBytes *int64           `json:"memory_usage_bytes,omitempty" validate:"omitempty,min=0"`
	StartedAt        *time.Time       `json:"started_at,omitempty"`
	CompletedAt      *time.Time       `json:"completed_at,omitempty"`
}

// TaskExecutionResponse represents the task execution response
type TaskExecutionResponse struct {
	ID               uuid.UUID        `json:"id"`
	TaskID           uuid.UUID        `json:"task_id"`
	Status           ExecutionStatus  `json:"status"`
	ReturnCode       *int             `json:"return_code,omitempty"`
	Stdout           *string          `json:"stdout,omitempty"`
	Stderr           *string          `json:"stderr,omitempty"`
	ExecutionTimeMs  *int             `json:"execution_time_ms,omitempty"`
	MemoryUsageBytes *int64           `json:"memory_usage_bytes,omitempty"`
	StartedAt        *string          `json:"started_at,omitempty"`
	CompletedAt      *string          `json:"completed_at,omitempty"`
	CreatedAt        string           `json:"created_at"`
}

// ToResponse converts TaskExecution to TaskExecutionResponse
func (te *TaskExecution) ToResponse() TaskExecutionResponse {
	response := TaskExecutionResponse{
		ID:               te.ID,
		TaskID:           te.TaskID,
		Status:           te.Status,
		ReturnCode:       te.ReturnCode,
		Stdout:           te.Stdout,
		Stderr:           te.Stderr,
		ExecutionTimeMs:  te.ExecutionTimeMs,
		MemoryUsageBytes: te.MemoryUsageBytes,
		CreatedAt:        te.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}

	if te.StartedAt != nil {
		startedAtStr := te.StartedAt.Format("2006-01-02T15:04:05Z07:00")
		response.StartedAt = &startedAtStr
	}

	if te.CompletedAt != nil {
		completedAtStr := te.CompletedAt.Format("2006-01-02T15:04:05Z07:00")
		response.CompletedAt = &completedAtStr
	}

	return response
}

// ValidateExecutionStatus validates the execution status
func ValidateExecutionStatus(status ExecutionStatus) error {
	switch status {
	case ExecutionStatusPending, ExecutionStatusRunning, ExecutionStatusCompleted, 
		 ExecutionStatusFailed, ExecutionStatusTimeout, ExecutionStatusCancelled:
		return nil
	default:
		return fmt.Errorf("invalid execution status: %s", status)
	}
}

// IsTerminal returns true if the execution status is terminal (completed, failed, timeout, cancelled)
func (te *TaskExecution) IsTerminal() bool {
	return te.Status == ExecutionStatusCompleted || 
		   te.Status == ExecutionStatusFailed || 
		   te.Status == ExecutionStatusTimeout || 
		   te.Status == ExecutionStatusCancelled
}

// IsRunning returns true if the execution is currently running
func (te *TaskExecution) IsRunning() bool {
	return te.Status == ExecutionStatusRunning
}

// IsPending returns true if the execution is pending
func (te *TaskExecution) IsPending() bool {
	return te.Status == ExecutionStatusPending
}

// GetDuration returns the execution duration in milliseconds
func (te *TaskExecution) GetDuration() *int {
	if te.StartedAt != nil && te.CompletedAt != nil {
		duration := int(te.CompletedAt.Sub(*te.StartedAt).Milliseconds())
		return &duration
	}
	return te.ExecutionTimeMs
}
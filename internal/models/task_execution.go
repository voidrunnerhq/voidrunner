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
	ID               uuid.UUID       `json:"id" db:"id"`
	TaskID           uuid.UUID       `json:"task_id" db:"task_id"`
	Status           ExecutionStatus `json:"status" db:"status"`
	ReturnCode       *int            `json:"return_code,omitempty" db:"return_code"`
	Stdout           *string         `json:"stdout,omitempty" db:"stdout"`
	Stderr           *string         `json:"stderr,omitempty" db:"stderr"`
	ExecutionTimeMs  *int            `json:"execution_time_ms,omitempty" db:"execution_time_ms"`
	MemoryUsageBytes *int64          `json:"memory_usage_bytes,omitempty" db:"memory_usage_bytes"`
	StartedAt        *time.Time      `json:"started_at,omitempty" db:"started_at"`
	CompletedAt      *time.Time      `json:"completed_at,omitempty" db:"completed_at"`
	CreatedAt        time.Time       `json:"created_at" db:"created_at"`
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
	ID               uuid.UUID       `json:"id"`
	TaskID           uuid.UUID       `json:"task_id"`
	Status           ExecutionStatus `json:"status"`
	ReturnCode       *int            `json:"return_code,omitempty"`
	Stdout           *string         `json:"stdout,omitempty"`
	Stderr           *string         `json:"stderr,omitempty"`
	ExecutionTimeMs  *int            `json:"execution_time_ms,omitempty"`
	MemoryUsageBytes *int64          `json:"memory_usage_bytes,omitempty"`
	StartedAt        *string         `json:"started_at,omitempty"`
	CompletedAt      *string         `json:"completed_at,omitempty"`
	CreatedAt        string          `json:"created_at"`
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

// ExecutionListResponse represents the response for listing task executions
type ExecutionListResponse struct {
	Executions []TaskExecutionResponse `json:"executions"`
	Total      int64                   `json:"total"`
	Limit      int                     `json:"limit"`
	Offset     int                     `json:"offset"`
}

// State transition definitions for execution status
var executionStatusTransitions = map[ExecutionStatus][]ExecutionStatus{
	ExecutionStatusPending: {
		ExecutionStatusRunning,
		ExecutionStatusCancelled,
	},
	ExecutionStatusRunning: {
		ExecutionStatusCompleted,
		ExecutionStatusFailed,
		ExecutionStatusTimeout,
		ExecutionStatusCancelled,
	},
	ExecutionStatusCompleted: {
		// Terminal state - no transitions allowed
	},
	ExecutionStatusFailed: {
		// Terminal state - no transitions allowed
	},
	ExecutionStatusTimeout: {
		// Terminal state - no transitions allowed
	},
	ExecutionStatusCancelled: {
		// Terminal state - no transitions allowed
	},
}

// ValidateExecutionStatusTransition validates if an execution status transition is allowed
func ValidateExecutionStatusTransition(currentStatus, newStatus ExecutionStatus) error {
	// Validate both statuses are valid
	if err := ValidateExecutionStatus(currentStatus); err != nil {
		return fmt.Errorf("invalid current execution status: %w", err)
	}

	if err := ValidateExecutionStatus(newStatus); err != nil {
		return fmt.Errorf("invalid new execution status: %w", err)
	}

	// Allow staying in the same status (idempotent updates)
	if currentStatus == newStatus {
		return nil
	}

	// Check if transition is allowed
	allowedTransitions, exists := executionStatusTransitions[currentStatus]
	if !exists {
		return fmt.Errorf("no transitions defined for execution status: %s", currentStatus)
	}

	for _, allowedStatus := range allowedTransitions {
		if newStatus == allowedStatus {
			return nil // Transition is allowed
		}
	}

	return fmt.Errorf("invalid execution status transition from %s to %s", currentStatus, newStatus)
}

// IsExecutionStatusTerminal returns true if the given execution status is terminal
func IsExecutionStatusTerminal(status ExecutionStatus) bool {
	switch status {
	case ExecutionStatusCompleted, ExecutionStatusFailed, ExecutionStatusTimeout, ExecutionStatusCancelled:
		return true
	default:
		return false
	}
}

// IsExecutionStatusSuccessful returns true if the execution completed successfully
func IsExecutionStatusSuccessful(status ExecutionStatus) bool {
	return status == ExecutionStatusCompleted
}

// IsExecutionStatusFailed returns true if the execution failed for any reason
func IsExecutionStatusFailed(status ExecutionStatus) bool {
	switch status {
	case ExecutionStatusFailed, ExecutionStatusTimeout, ExecutionStatusCancelled:
		return true
	default:
		return false
	}
}

// CanTransitionTo checks if the execution can transition to the given status
func (te *TaskExecution) CanTransitionTo(newStatus ExecutionStatus) bool {
	return ValidateExecutionStatusTransition(te.Status, newStatus) == nil
}

// TransitionTo attempts to transition the execution to a new status with validation
func (te *TaskExecution) TransitionTo(newStatus ExecutionStatus) error {
	if err := ValidateExecutionStatusTransition(te.Status, newStatus); err != nil {
		return err
	}

	oldStatus := te.Status
	te.Status = newStatus

	// Update timestamps based on status
	now := time.Now()
	switch newStatus {
	case ExecutionStatusRunning:
		if te.StartedAt == nil {
			te.StartedAt = &now
		}
	case ExecutionStatusCompleted, ExecutionStatusFailed, ExecutionStatusTimeout, ExecutionStatusCancelled:
		if te.CompletedAt == nil {
			te.CompletedAt = &now
		}
		// Calculate execution time if both timestamps are available
		if te.StartedAt != nil && te.ExecutionTimeMs == nil {
			duration := int(te.CompletedAt.Sub(*te.StartedAt).Milliseconds())
			te.ExecutionTimeMs = &duration
		}
	}

	// Log the transition for debugging
	fmt.Printf("Execution %s transitioned from %s to %s\n", te.ID, oldStatus, newStatus)

	return nil
}

// GetAllowedExecutionTransitions returns all allowed status transitions from current status
func (te *TaskExecution) GetAllowedTransitions() []ExecutionStatus {
	allowedTransitions, exists := executionStatusTransitions[te.Status]
	if !exists {
		return []ExecutionStatus{}
	}

	// Return a copy to prevent modification
	result := make([]ExecutionStatus, len(allowedTransitions))
	copy(result, allowedTransitions)
	return result
}

// IsSuccessful returns true if the execution completed successfully
func (te *TaskExecution) IsSuccessful() bool {
	return IsExecutionStatusSuccessful(te.Status)
}

// HasFailed returns true if the execution failed for any reason
func (te *TaskExecution) HasFailed() bool {
	return IsExecutionStatusFailed(te.Status)
}

// ExecutionStatusTransitionInfo provides information about an execution status transition
type ExecutionStatusTransitionInfo struct {
	FromStatus   ExecutionStatus `json:"from_status"`
	ToStatus     ExecutionStatus `json:"to_status"`
	IsValid      bool            `json:"is_valid"`
	IsTerminal   bool            `json:"is_terminal"`
	IsSuccessful bool            `json:"is_successful"`
	ErrorMessage string          `json:"error_message,omitempty"`
}

// GetExecutionStatusTransitionInfo returns detailed information about a potential execution status transition
func GetExecutionStatusTransitionInfo(fromStatus, toStatus ExecutionStatus) *ExecutionStatusTransitionInfo {
	info := &ExecutionStatusTransitionInfo{
		FromStatus:   fromStatus,
		ToStatus:     toStatus,
		IsTerminal:   IsExecutionStatusTerminal(toStatus),
		IsSuccessful: IsExecutionStatusSuccessful(toStatus),
	}

	err := ValidateExecutionStatusTransition(fromStatus, toStatus)
	if err != nil {
		info.IsValid = false
		info.ErrorMessage = err.Error()
	} else {
		info.IsValid = true
	}

	return info
}

// GetAllExecutionStatusTransitions returns all valid transitions for all execution statuses
func GetAllExecutionStatusTransitions() map[ExecutionStatus][]ExecutionStatus {
	// Return a deep copy to prevent modification
	result := make(map[ExecutionStatus][]ExecutionStatus)
	for status, transitions := range executionStatusTransitions {
		transitionsCopy := make([]ExecutionStatus, len(transitions))
		copy(transitionsCopy, transitions)
		result[status] = transitionsCopy
	}
	return result
}

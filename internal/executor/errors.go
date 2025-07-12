package executor

import (
	"errors"
	"fmt"
)

// Common executor errors
var (
	// ErrDockerUnavailable indicates that Docker daemon is not available
	ErrDockerUnavailable = errors.New("docker daemon is not available")

	// ErrExecutionTimeout indicates that execution exceeded the timeout
	ErrExecutionTimeout = errors.New("execution timeout exceeded")

	// ErrExecutionCancelled indicates that execution was cancelled
	ErrExecutionCancelled = errors.New("execution was cancelled")

	// ErrExecutionFailed indicates that execution failed
	ErrExecutionFailed = errors.New("execution failed")

	// ErrResourceExhausted indicates that system resources are exhausted
	ErrResourceExhausted = errors.New("system resources exhausted")

	// ErrInvalidScriptType indicates an unsupported script type
	ErrInvalidScriptType = errors.New("invalid script type")

	// ErrContainerNotFound indicates that a container was not found
	ErrContainerNotFound = errors.New("container not found")

	// ErrImageNotFound indicates that a container image was not found
	ErrImageNotFound = errors.New("container image not found")

	// ErrPermissionDenied indicates insufficient permissions
	ErrPermissionDenied = errors.New("permission denied")

	// ErrNetworkUnavailable indicates network connectivity issues
	ErrNetworkUnavailable = errors.New("network unavailable")
)

// ExecutorError represents a structured error from the executor
type ExecutorError struct {
	Operation string
	Reason    string
	Cause     error
}

// Error implements the error interface
func (e *ExecutorError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("executor error in %s: %s: %v", e.Operation, e.Reason, e.Cause)
	}
	return fmt.Sprintf("executor error in %s: %s", e.Operation, e.Reason)
}

// Unwrap returns the underlying cause
func (e *ExecutorError) Unwrap() error {
	return e.Cause
}

// NewExecutorError creates a new executor error
func NewExecutorError(operation, reason string, cause error) *ExecutorError {
	return &ExecutorError{
		Operation: operation,
		Reason:    reason,
		Cause:     cause,
	}
}

// ContainerError represents a container-specific error
type ContainerError struct {
	ContainerID string
	Operation   string
	Reason      string
	Cause       error
}

// Error implements the error interface
func (e *ContainerError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("container error in %s for %s: %s: %v", e.Operation, e.ContainerID, e.Reason, e.Cause)
	}
	return fmt.Sprintf("container error in %s for %s: %s", e.Operation, e.ContainerID, e.Reason)
}

// Unwrap returns the underlying cause
func (e *ContainerError) Unwrap() error {
	return e.Cause
}

// NewContainerError creates a new container error
func NewContainerError(containerID, operation, reason string, cause error) *ContainerError {
	return &ContainerError{
		ContainerID: containerID,
		Operation:   operation,
		Reason:      reason,
		Cause:       cause,
	}
}

// ConfigError represents a configuration error
type ConfigError struct {
	Field  string
	Reason string
}

// Error implements the error interface
func (e *ConfigError) Error() string {
	return fmt.Sprintf("configuration error in field %s: %s", e.Field, e.Reason)
}

// ErrInvalidConfig creates a configuration error
func ErrInvalidConfig(reason string) error {
	return &ConfigError{
		Field:  "config",
		Reason: reason,
	}
}

// ErrInvalidConfigField creates a configuration error for a specific field
func ErrInvalidConfigField(field, reason string) error {
	return &ConfigError{
		Field:  field,
		Reason: reason,
	}
}

// SecurityError represents a security-related error
type SecurityError struct {
	Operation string
	Reason    string
	Cause     error
}

// Error implements the error interface
func (e *SecurityError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("security error in %s: %s: %v", e.Operation, e.Reason, e.Cause)
	}
	return fmt.Sprintf("security error in %s: %s", e.Operation, e.Reason)
}

// Unwrap returns the underlying cause
func (e *SecurityError) Unwrap() error {
	return e.Cause
}

// NewSecurityError creates a new security error
func NewSecurityError(operation, reason string, cause error) *SecurityError {
	return &SecurityError{
		Operation: operation,
		Reason:    reason,
		Cause:     cause,
	}
}

// IsTimeoutError checks if an error is a timeout error
func IsTimeoutError(err error) bool {
	return errors.Is(err, ErrExecutionTimeout)
}

// IsCancelledError checks if an error is a cancellation error
func IsCancelledError(err error) bool {
	return errors.Is(err, ErrExecutionCancelled)
}

// IsDockerError checks if an error is related to Docker
func IsDockerError(err error) bool {
	return errors.Is(err, ErrDockerUnavailable) ||
		errors.Is(err, ErrContainerNotFound) ||
		errors.Is(err, ErrImageNotFound)
}

// IsResourceError checks if an error is related to resource exhaustion
func IsResourceError(err error) bool {
	return errors.Is(err, ErrResourceExhausted)
}

// IsSecurityError checks if an error is security-related
func IsSecurityError(err error) bool {
	var secErr *SecurityError
	return errors.As(err, &secErr)
}

// IsConfigError checks if an error is configuration-related
func IsConfigError(err error) bool {
	var confErr *ConfigError
	return errors.As(err, &confErr)
}

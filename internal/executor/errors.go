package executor

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ErrorType represents different categories of execution errors
type ErrorType string

const (
	// ErrorTypeInfrastructure indicates infrastructure-related failures
	ErrorTypeInfrastructure ErrorType = "infrastructure"
	
	// ErrorTypeResource indicates resource exhaustion or limits
	ErrorTypeResource ErrorType = "resource"
	
	// ErrorTypeTimeout indicates execution timeout
	ErrorTypeTimeout ErrorType = "timeout"
	
	// ErrorTypeUserCode indicates user script execution errors
	ErrorTypeUserCode ErrorType = "user_code"
	
	// ErrorTypeValidation indicates input validation errors
	ErrorTypeValidation ErrorType = "validation"
	
	// ErrorTypeSecurity indicates security policy violations
	ErrorTypeSecurity ErrorType = "security"
	
	// ErrorTypeNetwork indicates network connectivity issues
	ErrorTypeNetwork ErrorType = "network"
	
	// ErrorTypeRateLimit indicates rate limiting enforcement
	ErrorTypeRateLimit ErrorType = "rate_limit"
	
	// ErrorTypeQuota indicates quota enforcement
	ErrorTypeQuota ErrorType = "quota"
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

	// ErrRateLimitExceeded indicates rate limit has been exceeded
	ErrRateLimitExceeded = errors.New("rate limit exceeded")

	// ErrQuotaExceeded indicates quota has been exceeded
	ErrQuotaExceeded = errors.New("quota exceeded")

	// ErrSystemOverloaded indicates system is under high load
	ErrSystemOverloaded = errors.New("system overloaded")
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

// ExecutionError represents a comprehensive error with classification and context
type ExecutionError struct {
	Type        ErrorType              `json:"type"`
	Code        string                 `json:"code"`
	Message     string                 `json:"message"`
	Details     string                 `json:"details,omitempty"`
	Retryable   bool                   `json:"retryable"`
	TaskID      string                 `json:"task_id"`
	ExecutionID string                 `json:"execution_id,omitempty"`
	ContainerID string                 `json:"container_id,omitempty"`
	Timestamp   time.Time              `json:"timestamp"`
	Context     map[string]interface{} `json:"context,omitempty"`
	Cause       error                  `json:"-"`
}

// Error implements the error interface
func (e *ExecutionError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s [%s:%s]: %s: %v", e.Type, e.Code, e.TaskID, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s [%s:%s]: %s", e.Type, e.Code, e.TaskID, e.Message)
}

// Unwrap returns the underlying cause
func (e *ExecutionError) Unwrap() error {
	return e.Cause
}

// NewExecutionError creates a new execution error with classification
func NewExecutionError(errorType ErrorType, code, message string, taskID uuid.UUID) *ExecutionError {
	return &ExecutionError{
		Type:      errorType,
		Code:      code,
		Message:   message,
		TaskID:    taskID.String(),
		Timestamp: time.Now(),
		Context:   make(map[string]interface{}),
	}
}

// WithCause adds a cause to the execution error
func (e *ExecutionError) WithCause(cause error) *ExecutionError {
	e.Cause = cause
	return e
}

// WithDetails adds additional details to the execution error
func (e *ExecutionError) WithDetails(details string) *ExecutionError {
	e.Details = details
	return e
}

// WithContext adds context information to the execution error
func (e *ExecutionError) WithContext(key string, value interface{}) *ExecutionError {
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	e.Context[key] = value
	return e
}

// WithExecutionID adds execution ID to the execution error
func (e *ExecutionError) WithExecutionID(executionID uuid.UUID) *ExecutionError {
	e.ExecutionID = executionID.String()
	return e
}

// WithContainerID adds container ID to the execution error
func (e *ExecutionError) WithContainerID(containerID string) *ExecutionError {
	e.ContainerID = containerID
	return e
}

// SetRetryable sets whether the error is retryable
func (e *ExecutionError) SetRetryable(retryable bool) *ExecutionError {
	e.Retryable = retryable
	return e
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

// IsExecutionError checks if an error is an ExecutionError
func IsExecutionError(err error) bool {
	var execErr *ExecutionError
	return errors.As(err, &execErr)
}

// ClassifyError analyzes an error and creates a classified ExecutionError
func ClassifyError(err error, taskID uuid.UUID, context string) *ExecutionError {
	if err == nil {
		return nil
	}

	// Check if it's already an ExecutionError
	if execErr, ok := err.(*ExecutionError); ok {
		return execErr
	}

	errMsg := err.Error()
	errMsgLower := strings.ToLower(errMsg)
	
	// Default execution error
	execErr := NewExecutionError(ErrorTypeInfrastructure, "UNKNOWN_ERROR", "Unknown execution error", taskID).
		WithCause(err).
		WithContext("classification_context", context)

	// Classify based on error message patterns
	switch {
	// Docker daemon errors
	case strings.Contains(errMsgLower, "docker daemon") || strings.Contains(errMsgLower, "connection refused"):
		execErr.Type = ErrorTypeInfrastructure
		execErr.Code = "DOCKER_DAEMON_UNAVAILABLE"
		execErr.Message = "Docker daemon is unavailable"
		execErr.SetRetryable(true)

	// Container not found errors
	case strings.Contains(errMsgLower, "no such container"):
		execErr.Type = ErrorTypeInfrastructure
		execErr.Code = "CONTAINER_NOT_FOUND"
		execErr.Message = "Container not found"
		execErr.SetRetryable(false)

	// Image not found errors
	case strings.Contains(errMsgLower, "no such image") || strings.Contains(errMsgLower, "pull access denied"):
		execErr.Type = ErrorTypeInfrastructure
		execErr.Code = "IMAGE_NOT_FOUND"
		execErr.Message = "Container image not found or access denied"
		execErr.SetRetryable(false)

	// Timeout errors
	case strings.Contains(errMsgLower, "timeout") || strings.Contains(errMsgLower, "deadline exceeded"):
		execErr.Type = ErrorTypeTimeout
		execErr.Code = "EXECUTION_TIMEOUT"
		execErr.Message = "Task execution timed out"
		execErr.SetRetryable(true)

	// Resource exhaustion errors
	case strings.Contains(errMsgLower, "out of memory") || strings.Contains(errMsgLower, "oom"):
		execErr.Type = ErrorTypeResource
		execErr.Code = "OUT_OF_MEMORY"
		execErr.Message = "Container ran out of memory"
		execErr.SetRetryable(false)

	case strings.Contains(errMsgLower, "no space left"):
		execErr.Type = ErrorTypeResource
		execErr.Code = "OUT_OF_DISK_SPACE"
		execErr.Message = "Insufficient disk space"
		execErr.SetRetryable(true)

	case strings.Contains(errMsgLower, "cpu quota"):
		execErr.Type = ErrorTypeResource
		execErr.Code = "CPU_QUOTA_EXCEEDED"
		execErr.Message = "CPU quota exceeded"
		execErr.SetRetryable(true)

	// Network errors
	case strings.Contains(errMsgLower, "network") || strings.Contains(errMsgLower, "dns"):
		execErr.Type = ErrorTypeNetwork
		execErr.Code = "NETWORK_ERROR"
		execErr.Message = "Network connectivity issue"
		execErr.SetRetryable(true)

	// Permission errors
	case strings.Contains(errMsgLower, "permission denied") || strings.Contains(errMsgLower, "access denied"):
		execErr.Type = ErrorTypeSecurity
		execErr.Code = "PERMISSION_DENIED"
		execErr.Message = "Permission denied"
		execErr.SetRetryable(false)

	// Rate limiting errors
	case strings.Contains(errMsgLower, "rate limit") || strings.Contains(errMsgLower, "too many requests"):
		execErr.Type = ErrorTypeRateLimit
		execErr.Code = "RATE_LIMIT_EXCEEDED"
		execErr.Message = "Rate limit exceeded"
		execErr.SetRetryable(true)

	// Quota errors
	case strings.Contains(errMsgLower, "quota") || strings.Contains(errMsgLower, "limit exceeded"):
		execErr.Type = ErrorTypeQuota
		execErr.Code = "QUOTA_EXCEEDED"
		execErr.Message = "Quota exceeded"
		execErr.SetRetryable(false)

	// Validation errors
	case strings.Contains(errMsgLower, "invalid") || strings.Contains(errMsgLower, "malformed"):
		execErr.Type = ErrorTypeValidation
		execErr.Code = "VALIDATION_ERROR"
		execErr.Message = "Input validation failed"
		execErr.SetRetryable(false)

	// User code errors (script execution failures)
	case strings.Contains(errMsgLower, "exit status") || strings.Contains(errMsgLower, "command failed"):
		execErr.Type = ErrorTypeUserCode
		execErr.Code = "USER_CODE_ERROR"
		execErr.Message = "User script execution failed"
		execErr.SetRetryable(false)

	// Cancellation errors
	case strings.Contains(errMsgLower, "cancelled") || strings.Contains(errMsgLower, "canceled"):
		execErr.Type = ErrorTypeTimeout
		execErr.Code = "EXECUTION_CANCELLED"
		execErr.Message = "Task execution was cancelled"
		execErr.SetRetryable(false)

	default:
		// Keep default classification for unknown errors
		execErr.SetRetryable(true) // Conservative approach - assume retryable unless known otherwise
	}

	return execErr
}

// ClassifyDockerError specifically classifies Docker-related errors
func ClassifyDockerError(err error, taskID uuid.UUID, containerID string) *ExecutionError {
	execErr := ClassifyError(err, taskID, "docker_operation")
	
	if containerID != "" {
		execErr.WithContainerID(containerID)
	}

	// Add Docker-specific context
	execErr.WithContext("error_source", "docker_client")
	
	return execErr
}

// IsRetryableError determines if an error should trigger a retry
func IsRetryableError(err error) bool {
	if execErr, ok := err.(*ExecutionError); ok {
		return execErr.Retryable
	}

	// For non-ExecutionError types, use existing classification
	return IsTimeoutError(err) || IsDockerError(err) || IsResourceError(err)
}

// GetErrorType extracts the error type from an error
func GetErrorType(err error) ErrorType {
	if execErr, ok := err.(*ExecutionError); ok {
		return execErr.Type
	}

	// Legacy error type classification
	switch {
	case IsTimeoutError(err):
		return ErrorTypeTimeout
	case IsDockerError(err):
		return ErrorTypeInfrastructure
	case IsResourceError(err):
		return ErrorTypeResource
	case IsSecurityError(err):
		return ErrorTypeSecurity
	case IsConfigError(err):
		return ErrorTypeValidation
	default:
		return ErrorTypeInfrastructure
	}
}

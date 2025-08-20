package executor

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecutorError(t *testing.T) {
	tests := []struct {
		name      string
		operation string
		reason    string
		cause     error
		expected  string
	}{
		{
			name:      "Error without cause",
			operation: "test_operation",
			reason:    "test failed",
			cause:     nil,
			expected:  "executor error in test_operation: test failed",
		},
		{
			name:      "Error with cause",
			operation: "test_operation",
			reason:    "test failed",
			cause:     errors.New("underlying error"),
			expected:  "executor error in test_operation: test failed: underlying error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewExecutorError(tt.operation, tt.reason, tt.cause)
			assert.Equal(t, tt.expected, err.Error())
			assert.Equal(t, tt.operation, err.Operation)
			assert.Equal(t, tt.reason, err.Reason)
			assert.Equal(t, tt.cause, err.Cause)

			if tt.cause != nil {
				assert.Equal(t, tt.cause, err.Unwrap())
			} else {
				assert.Nil(t, err.Unwrap())
			}
		})
	}
}

func TestContainerError(t *testing.T) {
	tests := []struct {
		name        string
		containerID string
		operation   string
		reason      string
		cause       error
		expected    string
	}{
		{
			name:        "Container error without cause",
			containerID: "abc123",
			operation:   "start_container",
			reason:      "container not found",
			cause:       nil,
			expected:    "container error in start_container for abc123: container not found",
		},
		{
			name:        "Container error with cause",
			containerID: "abc123",
			operation:   "start_container",
			reason:      "container not found",
			cause:       errors.New("Docker daemon error"),
			expected:    "container error in start_container for abc123: container not found: Docker daemon error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewContainerError(tt.containerID, tt.operation, tt.reason, tt.cause)
			assert.Equal(t, tt.expected, err.Error())
			assert.Equal(t, tt.containerID, err.ContainerID)
			assert.Equal(t, tt.operation, err.Operation)
			assert.Equal(t, tt.reason, err.Reason)
			assert.Equal(t, tt.cause, err.Cause)

			if tt.cause != nil {
				assert.Equal(t, tt.cause, err.Unwrap())
			} else {
				assert.Nil(t, err.Unwrap())
			}
		})
	}
}

func TestConfigError(t *testing.T) {
	tests := []struct {
		name     string
		field    string
		reason   string
		expected string
	}{
		{
			name:     "Generic config error",
			field:    "config",
			reason:   "invalid configuration",
			expected: "configuration error in field config: invalid configuration",
		},
		{
			name:     "Specific field error",
			field:    "memory_limit",
			reason:   "must be positive",
			expected: "configuration error in field memory_limit: must be positive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var err error
			if tt.field == "config" {
				err = ErrInvalidConfig(tt.reason)
			} else {
				err = ErrInvalidConfigField(tt.field, tt.reason)
			}

			assert.Equal(t, tt.expected, err.Error())
		})
	}
}

func TestSecurityError(t *testing.T) {
	tests := []struct {
		name      string
		operation string
		reason    string
		cause     error
		expected  string
	}{
		{
			name:      "Security error without cause",
			operation: "validate_script",
			reason:    "dangerous pattern detected",
			cause:     nil,
			expected:  "security error in validate_script: dangerous pattern detected",
		},
		{
			name:      "Security error with cause",
			operation: "validate_script",
			reason:    "dangerous pattern detected",
			cause:     errors.New("rm -rf found"),
			expected:  "security error in validate_script: dangerous pattern detected: rm -rf found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewSecurityError(tt.operation, tt.reason, tt.cause)
			assert.Equal(t, tt.expected, err.Error())
			assert.Equal(t, tt.operation, err.Operation)
			assert.Equal(t, tt.reason, err.Reason)
			assert.Equal(t, tt.cause, err.Cause)

			if tt.cause != nil {
				assert.Equal(t, tt.cause, err.Unwrap())
			} else {
				assert.Nil(t, err.Unwrap())
			}
		})
	}
}

func TestErrorTypeCheckers(t *testing.T) {
	t.Run("IsTimeoutError", func(t *testing.T) {
		assert.True(t, IsTimeoutError(ErrExecutionTimeout))
		assert.False(t, IsTimeoutError(ErrExecutionCancelled))
		assert.False(t, IsTimeoutError(errors.New("other error")))
	})

	t.Run("IsCancelledError", func(t *testing.T) {
		assert.True(t, IsCancelledError(ErrExecutionCancelled))
		assert.False(t, IsCancelledError(ErrExecutionTimeout))
		assert.False(t, IsCancelledError(errors.New("other error")))
	})

	t.Run("IsDockerError", func(t *testing.T) {
		assert.True(t, IsDockerError(ErrDockerUnavailable))
		assert.True(t, IsDockerError(ErrContainerNotFound))
		assert.True(t, IsDockerError(ErrImageNotFound))
		assert.False(t, IsDockerError(ErrExecutionTimeout))
		assert.False(t, IsDockerError(errors.New("other error")))
	})

	t.Run("IsResourceError", func(t *testing.T) {
		assert.True(t, IsResourceError(ErrResourceExhausted))
		assert.False(t, IsResourceError(ErrExecutionTimeout))
		assert.False(t, IsResourceError(errors.New("other error")))
	})

	t.Run("IsSecurityError", func(t *testing.T) {
		secErr := NewSecurityError("test", "test error", nil)
		assert.True(t, IsSecurityError(secErr))
		assert.False(t, IsSecurityError(ErrExecutionTimeout))
		assert.False(t, IsSecurityError(errors.New("other error")))
	})

	t.Run("IsConfigError", func(t *testing.T) {
		confErr := ErrInvalidConfig("test error")
		assert.True(t, IsConfigError(confErr))
		assert.False(t, IsConfigError(ErrExecutionTimeout))
		assert.False(t, IsConfigError(errors.New("other error")))
	})
}

func TestErrorWrapping(t *testing.T) {
	baseErr := errors.New("base error")

	t.Run("ExecutorError wrapping", func(t *testing.T) {
		wrappedErr := NewExecutorError("test", "wrapped error", baseErr)
		assert.True(t, errors.Is(wrappedErr, baseErr))
	})

	t.Run("ContainerError wrapping", func(t *testing.T) {
		wrappedErr := NewContainerError("container123", "test", "wrapped error", baseErr)
		assert.True(t, errors.Is(wrappedErr, baseErr))
	})

	t.Run("SecurityError wrapping", func(t *testing.T) {
		wrappedErr := NewSecurityError("test", "wrapped error", baseErr)
		assert.True(t, errors.Is(wrappedErr, baseErr))
	})
}

func TestExecutionError(t *testing.T) {
	taskID := uuid.New()
	executionID := uuid.New()
	containerID := "container123"

	t.Run("NewExecutionError creates error with required fields", func(t *testing.T) {
		err := NewExecutionError(ErrorTypeTimeout, "TIMEOUT_001", "Execution timed out", taskID)

		assert.Equal(t, ErrorTypeTimeout, err.Type)
		assert.Equal(t, "TIMEOUT_001", err.Code)
		assert.Equal(t, "Execution timed out", err.Message)
		assert.Equal(t, taskID.String(), err.TaskID)
		assert.False(t, err.Retryable) // Default is false
		assert.NotNil(t, err.Context)
		assert.WithinDuration(t, time.Now(), err.Timestamp, time.Second)
	})

	t.Run("ExecutionError builder methods work correctly", func(t *testing.T) {
		cause := errors.New("underlying error")

		err := NewExecutionError(ErrorTypeResource, "MEM_001", "Out of memory", taskID).
			WithCause(cause).
			WithDetails("Container exceeded 512MB limit").
			WithExecutionID(executionID).
			WithContainerID(containerID).
			WithContext("memory_limit", "512MB").
			SetRetryable(true)

		assert.Equal(t, cause, err.Cause)
		assert.Equal(t, "Container exceeded 512MB limit", err.Details)
		assert.Equal(t, executionID.String(), err.ExecutionID)
		assert.Equal(t, containerID, err.ContainerID)
		assert.Equal(t, "512MB", err.Context["memory_limit"])
		assert.True(t, err.Retryable)
	})

	t.Run("ExecutionError Error() method formats correctly", func(t *testing.T) {
		// Without cause
		err1 := NewExecutionError(ErrorTypeTimeout, "TIMEOUT_001", "Execution timed out", taskID)
		expected1 := "timeout [TIMEOUT_001:" + taskID.String() + "]: Execution timed out"
		assert.Equal(t, expected1, err1.Error())

		// With cause
		cause := errors.New("underlying error")
		err2 := NewExecutionError(ErrorTypeTimeout, "TIMEOUT_001", "Execution timed out", taskID).WithCause(cause)
		expected2 := "timeout [TIMEOUT_001:" + taskID.String() + "]: Execution timed out: underlying error"
		assert.Equal(t, expected2, err2.Error())
	})

	t.Run("ExecutionError Unwrap works correctly", func(t *testing.T) {
		cause := errors.New("underlying error")
		err := NewExecutionError(ErrorTypeTimeout, "TIMEOUT_001", "Execution timed out", taskID).WithCause(cause)

		assert.Equal(t, cause, err.Unwrap())
		assert.True(t, errors.Is(err, cause))
	})
}

func TestClassifyError(t *testing.T) {
	taskID := uuid.New()

	tests := []struct {
		name          string
		inputError    error
		context       string
		expectedType  ErrorType
		expectedCode  string
		expectedRetry bool
	}{
		{
			name:          "Docker daemon unavailable",
			inputError:    errors.New("Cannot connect to the Docker daemon"),
			context:       "container_create",
			expectedType:  ErrorTypeInfrastructure,
			expectedCode:  "DOCKER_DAEMON_UNAVAILABLE",
			expectedRetry: true,
		},
		{
			name:          "Container not found",
			inputError:    errors.New("No such container: abc123"),
			context:       "container_stop",
			expectedType:  ErrorTypeInfrastructure,
			expectedCode:  "CONTAINER_NOT_FOUND",
			expectedRetry: false,
		},
		{
			name:          "Image not found",
			inputError:    errors.New("pull access denied for python:latest"),
			context:       "image_pull",
			expectedType:  ErrorTypeInfrastructure,
			expectedCode:  "IMAGE_NOT_FOUND",
			expectedRetry: false,
		},
		{
			name:          "Execution timeout",
			inputError:    errors.New("context deadline exceeded"),
			context:       "task_execution",
			expectedType:  ErrorTypeTimeout,
			expectedCode:  "EXECUTION_TIMEOUT",
			expectedRetry: true,
		},
		{
			name:          "Out of memory",
			inputError:    errors.New("container killed due to OOM"),
			context:       "task_execution",
			expectedType:  ErrorTypeResource,
			expectedCode:  "OUT_OF_MEMORY",
			expectedRetry: false,
		},
		{
			name:          "Disk space exhausted",
			inputError:    errors.New("no space left on device"),
			context:       "container_create",
			expectedType:  ErrorTypeResource,
			expectedCode:  "OUT_OF_DISK_SPACE",
			expectedRetry: true,
		},
		{
			name:          "CPU quota exceeded",
			inputError:    errors.New("CPU quota exceeded for container"),
			context:       "task_execution",
			expectedType:  ErrorTypeResource,
			expectedCode:  "CPU_QUOTA_EXCEEDED",
			expectedRetry: true,
		},
		{
			name:          "Network error",
			inputError:    errors.New("network connection failed"),
			context:       "container_start",
			expectedType:  ErrorTypeNetwork,
			expectedCode:  "NETWORK_ERROR",
			expectedRetry: true,
		},
		{
			name:          "Permission denied",
			inputError:    errors.New("permission denied: cannot access file"),
			context:       "file_access",
			expectedType:  ErrorTypeSecurity,
			expectedCode:  "PERMISSION_DENIED",
			expectedRetry: false,
		},
		{
			name:          "Rate limit exceeded",
			inputError:    errors.New("rate limit exceeded for API calls"),
			context:       "api_call",
			expectedType:  ErrorTypeRateLimit,
			expectedCode:  "RATE_LIMIT_EXCEEDED",
			expectedRetry: true,
		},
		{
			name:          "Quota exceeded",
			inputError:    errors.New("quota limit exceeded for user"),
			context:       "resource_allocation",
			expectedType:  ErrorTypeQuota,
			expectedCode:  "QUOTA_EXCEEDED",
			expectedRetry: false,
		},
		{
			name:          "Validation error",
			inputError:    errors.New("invalid input format provided"),
			context:       "input_validation",
			expectedType:  ErrorTypeValidation,
			expectedCode:  "VALIDATION_ERROR",
			expectedRetry: false,
		},
		{
			name:          "User code error",
			inputError:    errors.New("command failed with exit status 1"),
			context:       "script_execution",
			expectedType:  ErrorTypeUserCode,
			expectedCode:  "USER_CODE_ERROR",
			expectedRetry: false,
		},
		{
			name:          "Cancellation error",
			inputError:    errors.New("operation was cancelled by user"),
			context:       "task_execution",
			expectedType:  ErrorTypeTimeout,
			expectedCode:  "EXECUTION_CANCELLED",
			expectedRetry: false,
		},
		{
			name:          "Unknown error",
			inputError:    errors.New("something completely unexpected happened"),
			context:       "unknown_operation",
			expectedType:  ErrorTypeInfrastructure,
			expectedCode:  "UNKNOWN_ERROR",
			expectedRetry: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			execErr := ClassifyError(tt.inputError, taskID, tt.context)

			require.NotNil(t, execErr)
			assert.Equal(t, tt.expectedType, execErr.Type)
			assert.Equal(t, tt.expectedCode, execErr.Code)
			assert.Equal(t, tt.expectedRetry, execErr.Retryable)
			assert.Equal(t, taskID.String(), execErr.TaskID)
			assert.Equal(t, tt.inputError, execErr.Cause)
			assert.Equal(t, tt.context, execErr.Context["classification_context"])
		})
	}

	t.Run("Nil error returns nil", func(t *testing.T) {
		result := ClassifyError(nil, taskID, "test")
		assert.Nil(t, result)
	})

	t.Run("Already classified ExecutionError returns as-is", func(t *testing.T) {
		original := NewExecutionError(ErrorTypeValidation, "TEST_001", "Test error", taskID)
		result := ClassifyError(original, taskID, "test")

		assert.Equal(t, original, result)
	})
}

func TestClassifyDockerError(t *testing.T) {
	taskID := uuid.New()
	containerID := "container123"

	t.Run("Classifies Docker error with container ID", func(t *testing.T) {
		err := errors.New("Docker daemon connection refused")
		execErr := ClassifyDockerError(err, taskID, containerID)

		require.NotNil(t, execErr)
		assert.Equal(t, ErrorTypeInfrastructure, execErr.Type)
		assert.Equal(t, "DOCKER_DAEMON_UNAVAILABLE", execErr.Code)
		assert.Equal(t, containerID, execErr.ContainerID)
		assert.Equal(t, "docker_client", execErr.Context["error_source"])
		assert.Equal(t, "docker_operation", execErr.Context["classification_context"])
	})

	t.Run("Handles empty container ID", func(t *testing.T) {
		err := errors.New("No such container")
		execErr := ClassifyDockerError(err, taskID, "")

		require.NotNil(t, execErr)
		assert.Equal(t, ErrorTypeInfrastructure, execErr.Type)
		assert.Equal(t, "CONTAINER_NOT_FOUND", execErr.Code)
		assert.Empty(t, execErr.ContainerID)
	})
}

func TestIsRetryableError(t *testing.T) {
	taskID := uuid.New()

	t.Run("ExecutionError retryable flag is respected", func(t *testing.T) {
		retryableErr := NewExecutionError(ErrorTypeTimeout, "TIMEOUT_001", "Timeout", taskID).SetRetryable(true)
		nonRetryableErr := NewExecutionError(ErrorTypeUserCode, "USER_001", "User error", taskID).SetRetryable(false)

		assert.True(t, IsRetryableError(retryableErr))
		assert.False(t, IsRetryableError(nonRetryableErr))
	})

	t.Run("Legacy error types use existing classification", func(t *testing.T) {
		assert.True(t, IsRetryableError(ErrExecutionTimeout))
		assert.True(t, IsRetryableError(ErrDockerUnavailable))
		assert.True(t, IsRetryableError(ErrResourceExhausted))
		assert.False(t, IsRetryableError(errors.New("unknown error")))
	})
}

func TestGetErrorType(t *testing.T) {
	taskID := uuid.New()

	t.Run("ExecutionError type is returned", func(t *testing.T) {
		err := NewExecutionError(ErrorTypeNetwork, "NET_001", "Network error", taskID)
		assert.Equal(t, ErrorTypeNetwork, GetErrorType(err))
	})

	t.Run("Legacy error types are classified", func(t *testing.T) {
		assert.Equal(t, ErrorTypeTimeout, GetErrorType(ErrExecutionTimeout))
		assert.Equal(t, ErrorTypeInfrastructure, GetErrorType(ErrDockerUnavailable))
		assert.Equal(t, ErrorTypeResource, GetErrorType(ErrResourceExhausted))

		secErr := NewSecurityError("test", "test", nil)
		assert.Equal(t, ErrorTypeSecurity, GetErrorType(secErr))

		confErr := ErrInvalidConfig("test")
		assert.Equal(t, ErrorTypeValidation, GetErrorType(confErr))

		unknownErr := errors.New("unknown")
		assert.Equal(t, ErrorTypeInfrastructure, GetErrorType(unknownErr))
	})
}

func TestIsExecutionError(t *testing.T) {
	taskID := uuid.New()

	t.Run("Identifies ExecutionError correctly", func(t *testing.T) {
		execErr := NewExecutionError(ErrorTypeTimeout, "TIMEOUT_001", "Timeout", taskID)
		assert.True(t, IsExecutionError(execErr))
	})

	t.Run("Rejects non-ExecutionError types", func(t *testing.T) {
		assert.False(t, IsExecutionError(ErrExecutionTimeout))
		assert.False(t, IsExecutionError(errors.New("standard error")))

		containerErr := NewContainerError("container123", "test", "test", nil)
		assert.False(t, IsExecutionError(containerErr))
	})
}

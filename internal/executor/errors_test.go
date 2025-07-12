package executor

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
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

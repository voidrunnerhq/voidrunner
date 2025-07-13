package queue

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidationError(t *testing.T) {
	tests := []struct {
		name     string
		field    string
		value    interface{}
		message  string
		expected func(*testing.T, *ValidationError)
	}{
		{
			name:    "string value",
			field:   "name",
			value:   "invalid-name",
			message: "test reason",
			expected: func(t *testing.T, err *ValidationError) {
				assert.Equal(t, "name", err.Field)
				assert.Equal(t, "invalid-name", err.Value)
				assert.Equal(t, "test reason", err.Message)
				assert.Contains(t, err.Error(), "validation failed for field 'name'")
				assert.Contains(t, err.Error(), "invalid-name")
				assert.Contains(t, err.Error(), "test reason")
			},
		},
		{
			name:    "integer value",
			field:   "priority",
			value:   15,
			message: "must be between 0 and 10",
			expected: func(t *testing.T, err *ValidationError) {
				assert.Equal(t, "priority", err.Field)
				assert.Equal(t, 15, err.Value)
				assert.Equal(t, "must be between 0 and 10", err.Message)
				assert.Contains(t, err.Error(), "validation failed for field 'priority'")
				assert.Contains(t, err.Error(), "15")
			},
		},
		{
			name:    "nil value",
			field:   "config",
			value:   nil,
			message: "cannot be nil",
			expected: func(t *testing.T, err *ValidationError) {
				assert.Equal(t, "config", err.Field)
				assert.Nil(t, err.Value)
				assert.Equal(t, "cannot be nil", err.Message)
				assert.Contains(t, err.Error(), "validation failed for field 'config'")
				assert.Contains(t, err.Error(), "<nil>")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewValidationError(tt.field, tt.value, tt.message)
			tt.expected(t, err)
		})
	}
}

func TestQueueOperationError(t *testing.T) {
	tests := []struct {
		name      string
		operation string
		queueName string
		messageID string
		err       error
		retryable bool
		expected  func(*testing.T, *QueueOperationError)
	}{
		{
			name:      "with message ID",
			operation: "enqueue",
			queueName: "test-queue",
			messageID: "msg-123",
			err:       errors.New("connection failed"),
			retryable: true,
			expected: func(t *testing.T, qErr *QueueOperationError) {
				assert.Equal(t, "enqueue", qErr.Operation)
				assert.Equal(t, "test-queue", qErr.QueueName)
				assert.Equal(t, "msg-123", qErr.MessageID)
				assert.True(t, qErr.Retryable)
				assert.Contains(t, qErr.Error(), "queue operation 'enqueue' failed for queue 'test-queue', message 'msg-123'")
				assert.Contains(t, qErr.Error(), "connection failed")
				assert.Equal(t, qErr.Err, qErr.Unwrap())
			},
		},
		{
			name:      "without message ID",
			operation: "stats",
			queueName: "retry-queue",
			messageID: "",
			err:       errors.New("timeout"),
			retryable: false,
			expected: func(t *testing.T, qErr *QueueOperationError) {
				assert.Equal(t, "stats", qErr.Operation)
				assert.Equal(t, "retry-queue", qErr.QueueName)
				assert.Equal(t, "", qErr.MessageID)
				assert.False(t, qErr.Retryable)
				assert.Contains(t, qErr.Error(), "queue operation 'stats' failed for queue 'retry-queue'")
				assert.NotContains(t, qErr.Error(), "message")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			qErr := NewQueueOperationError(tt.operation, tt.queueName, tt.messageID, tt.err, tt.retryable)
			tt.expected(t, qErr)
		})
	}
}

func TestRedisError(t *testing.T) {
	tests := []struct {
		name      string
		operation string
		key       string
		err       error
		retryable bool
		expected  func(*testing.T, *RedisError)
	}{
		{
			name:      "with key",
			operation: "set",
			key:       "test:key",
			err:       errors.New("redis connection failed"),
			retryable: true,
			expected: func(t *testing.T, rErr *RedisError) {
				assert.Equal(t, "set", rErr.Operation)
				assert.Equal(t, "test:key", rErr.Key)
				assert.True(t, rErr.Retryable)
				assert.Contains(t, rErr.Error(), "redis operation 'set' failed for key 'test:key'")
				assert.Contains(t, rErr.Error(), "redis connection failed")
				assert.Equal(t, rErr.Err, rErr.Unwrap())
			},
		},
		{
			name:      "without key",
			operation: "ping",
			key:       "",
			err:       errors.New("timeout"),
			retryable: false,
			expected: func(t *testing.T, rErr *RedisError) {
				assert.Equal(t, "ping", rErr.Operation)
				assert.Equal(t, "", rErr.Key)
				assert.False(t, rErr.Retryable)
				assert.Contains(t, rErr.Error(), "redis operation 'ping' failed")
				assert.NotContains(t, rErr.Error(), "key")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rErr := NewRedisError(tt.operation, tt.key, tt.err, tt.retryable)
			tt.expected(t, rErr)
		})
	}
}

func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "retryable queue error",
			err:      NewQueueError("test", errors.New("temp failure"), true),
			expected: true,
		},
		{
			name:     "non-retryable queue error",
			err:      NewQueueError("test", errors.New("permanent failure"), false),
			expected: false,
		},
		{
			name:     "retryable operation error",
			err:      NewQueueOperationError("enqueue", "queue", "msg", errors.New("temp"), true),
			expected: true,
		},
		{
			name:     "non-retryable operation error",
			err:      NewQueueOperationError("validate", "queue", "msg", errors.New("invalid"), false),
			expected: false,
		},
		{
			name:     "retryable redis error",
			err:      NewRedisError("get", "key", errors.New("timeout"), true),
			expected: true,
		},
		{
			name:     "non-retryable redis error",
			err:      NewRedisError("set", "key", errors.New("invalid"), false),
			expected: false,
		},
		{
			name:     "unknown error defaults to retryable",
			err:      errors.New("unknown error"),
			expected: true,
		},
		{
			name:     "nil error",
			err:      nil,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsRetryableError(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsConnectionError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "connection refused",
			err:      errors.New("connection refused"),
			expected: true,
		},
		{
			name:     "connection reset",
			err:      errors.New("connection reset"),
			expected: true,
		},
		{
			name:     "connection timeout",
			err:      errors.New("connection timeout"),
			expected: true,
		},
		{
			name:     "network unreachable",
			err:      errors.New("network is unreachable"),
			expected: true,
		},
		{
			name:     "no route to host",
			err:      errors.New("no route to host"),
			expected: true,
		},
		{
			name:     "broken pipe",
			err:      errors.New("broken pipe"),
			expected: true,
		},
		{
			name:     "i/o timeout",
			err:      errors.New("i/o timeout"),
			expected: true,
		},
		{
			name:     "non-connection error",
			err:      errors.New("validation failed"),
			expected: false,
		},
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsConnectionError(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsTimeoutError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "timeout error",
			err:      errors.New("timeout"),
			expected: true,
		},
		{
			name:     "deadline exceeded",
			err:      errors.New("deadline exceeded"),
			expected: true,
		},
		{
			name:     "context deadline exceeded",
			err:      errors.New("context deadline exceeded"),
			expected: true,
		},
		{
			name:     "context canceled",
			err:      errors.New("context canceled"),
			expected: true,
		},
		{
			name:     "non-timeout error",
			err:      errors.New("validation failed"),
			expected: false,
		},
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsTimeoutError(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestWrapError(t *testing.T) {
	originalErr := errors.New("original error")
	
	tests := []struct {
		name      string
		operation string
		queueName string
		err       error
		expected  func(*testing.T, error)
	}{
		{
			name:      "wrap normal error",
			operation: "enqueue",
			queueName: "test-queue",
			err:       originalErr,
			expected: func(t *testing.T, wrappedErr error) {
				var opErr *QueueOperationError
				assert.True(t, errors.As(wrappedErr, &opErr))
				assert.Equal(t, "enqueue", opErr.Operation)
				assert.Equal(t, "test-queue", opErr.QueueName)
				assert.Equal(t, originalErr, opErr.Unwrap())
			},
		},
		{
			name:      "don't double-wrap queue error",
			operation: "test",
			queueName: "queue",
			err:       NewQueueError("existing", originalErr, true),
			expected: func(t *testing.T, wrappedErr error) {
				var qErr *QueueError
				assert.True(t, errors.As(wrappedErr, &qErr))
				assert.Equal(t, "existing", qErr.Operation)
			},
		},
		{
			name:      "don't double-wrap operation error",
			operation: "test",
			queueName: "queue",
			err:       NewQueueOperationError("existing", "existing-queue", "", originalErr, false),
			expected: func(t *testing.T, wrappedErr error) {
				var opErr *QueueOperationError
				assert.True(t, errors.As(wrappedErr, &opErr))
				assert.Equal(t, "existing", opErr.Operation)
				assert.Equal(t, "existing-queue", opErr.QueueName)
			},
		},
		{
			name:      "nil error returns nil",
			operation: "test",
			queueName: "queue",
			err:       nil,
			expected: func(t *testing.T, wrappedErr error) {
				assert.Nil(t, wrappedErr)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := WrapError(tt.operation, tt.queueName, tt.err)
			tt.expected(t, result)
		})
	}
}

func TestCommonQueueErrors(t *testing.T) {
	// Test that all common queue errors are defined
	commonErrors := []error{
		ErrQueueNotFound,
		ErrMessageNotFound,
		ErrInvalidMessage,
		ErrInvalidPriority,
		ErrInvalidReceiptHandle,
		ErrQueueClosed,
		ErrQueueNotHealthy,
		ErrMaxRetriesExceeded,
		ErrMessageExpired,
		ErrDuplicateMessage,
		ErrInvalidConfiguration,
	}

	for i, err := range commonErrors {
		t.Run(string(rune('A'+i)), func(t *testing.T) {
			assert.NotNil(t, err)
			assert.NotEmpty(t, err.Error())
		})
	}
}

func TestQueueOperationErrors(t *testing.T) {
	// Test that all queue operation errors are defined
	operationErrors := []error{
		ErrEnqueueFailed,
		ErrDequeueFailed,
		ErrDeleteFailed,
		ErrVisibilityFailed,
		ErrStatsFailed,
		ErrHealthCheckFailed,
	}

	for i, err := range operationErrors {
		t.Run(string(rune('A'+i)), func(t *testing.T) {
			assert.NotNil(t, err)
			assert.NotEmpty(t, err.Error())
		})
	}
}

func TestRedisErrors(t *testing.T) {
	// Test that all Redis-specific errors are defined
	redisErrors := []error{
		ErrRedisConnection,
		ErrRedisTimeout,
		ErrRedisScript,
		ErrRedisPipeline,
		ErrRedisTransaction,
	}

	for i, err := range redisErrors {
		t.Run(string(rune('A'+i)), func(t *testing.T) {
			assert.NotNil(t, err)
			assert.NotEmpty(t, err.Error())
		})
	}
}
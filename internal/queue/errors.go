package queue

import (
	"errors"
	"fmt"
)

// Common queue errors
var (
	ErrQueueNotFound        = errors.New("queue not found")
	ErrMessageNotFound      = errors.New("message not found")
	ErrInvalidMessage       = errors.New("invalid message format")
	ErrInvalidPriority      = errors.New("invalid priority value")
	ErrInvalidReceiptHandle = errors.New("invalid receipt handle")
	ErrQueueClosed          = errors.New("queue is closed")
	ErrQueueNotHealthy      = errors.New("queue is not healthy")
	ErrMaxRetriesExceeded   = errors.New("maximum retries exceeded")
	ErrMessageExpired       = errors.New("message has expired")
	ErrDuplicateMessage     = errors.New("duplicate message")
	ErrInvalidConfiguration = errors.New("invalid queue configuration")
)

// Queue operation errors
var (
	ErrEnqueueFailed     = errors.New("failed to enqueue message")
	ErrDequeueFailed     = errors.New("failed to dequeue message")
	ErrDeleteFailed      = errors.New("failed to delete message")
	ErrVisibilityFailed  = errors.New("failed to extend visibility")
	ErrStatsFailed       = errors.New("failed to get queue statistics")
	ErrHealthCheckFailed = errors.New("queue health check failed")
)

// Redis-specific errors
var (
	ErrRedisConnection  = errors.New("redis connection error")
	ErrRedisTimeout     = errors.New("redis operation timeout")
	ErrRedisScript      = errors.New("redis script execution error")
	ErrRedisPipeline    = errors.New("redis pipeline execution error")
	ErrRedisTransaction = errors.New("redis transaction error")
)

// QueueOperationError wraps queue operation errors with context
type QueueOperationError struct {
	Operation string
	QueueName string
	MessageID string
	Err       error
	Retryable bool
}

func (e *QueueOperationError) Error() string {
	if e.MessageID != "" {
		return fmt.Sprintf("queue operation '%s' failed for queue '%s', message '%s': %v",
			e.Operation, e.QueueName, e.MessageID, e.Err)
	}
	return fmt.Sprintf("queue operation '%s' failed for queue '%s': %v",
		e.Operation, e.QueueName, e.Err)
}

func (e *QueueOperationError) Unwrap() error {
	return e.Err
}

// IsRetryable returns true if the error is retryable
func (e *QueueOperationError) IsRetryable() bool {
	return e.Retryable
}

// NewQueueOperationError creates a new queue operation error
func NewQueueOperationError(operation, queueName, messageID string, err error, retryable bool) *QueueOperationError {
	return &QueueOperationError{
		Operation: operation,
		QueueName: queueName,
		MessageID: messageID,
		Err:       err,
		Retryable: retryable,
	}
}

// RedisError wraps Redis-specific errors
type RedisError struct {
	Operation string
	Key       string
	Err       error
	Retryable bool
}

func (e *RedisError) Error() string {
	if e.Key != "" {
		return fmt.Sprintf("redis operation '%s' failed for key '%s': %v",
			e.Operation, e.Key, e.Err)
	}
	return fmt.Sprintf("redis operation '%s' failed: %v", e.Operation, e.Err)
}

func (e *RedisError) Unwrap() error {
	return e.Err
}

// IsRetryable returns true if the error is retryable
func (e *RedisError) IsRetryable() bool {
	return e.Retryable
}

// NewRedisError creates a new Redis error
func NewRedisError(operation, key string, err error, retryable bool) *RedisError {
	return &RedisError{
		Operation: operation,
		Key:       key,
		Err:       err,
		Retryable: retryable,
	}
}

// ValidationError represents a validation error
type ValidationError struct {
	Field   string
	Value   interface{}
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation failed for field '%s' with value '%v': %s",
		e.Field, e.Value, e.Message)
}

// NewValidationError creates a new validation error
func NewValidationError(field string, value interface{}, message string) *ValidationError {
	return &ValidationError{
		Field:   field,
		Value:   value,
		Message: message,
	}
}

// IsRetryableError checks if an error is retryable
func IsRetryableError(err error) bool {
	var queueErr *QueueError
	if errors.As(err, &queueErr) {
		return queueErr.Retryable
	}

	var operationErr *QueueOperationError
	if errors.As(err, &operationErr) {
		return operationErr.Retryable
	}

	var redisErr *RedisError
	if errors.As(err, &redisErr) {
		return redisErr.Retryable
	}

	// Default to retryable for unknown errors
	return true
}

// IsConnectionError checks if an error is a connection-related error
func IsConnectionError(err error) bool {
	if err == nil {
		return false
	}

	// Check for common connection error patterns
	connectionErrors := []string{
		"connection refused",
		"connection reset",
		"connection timeout",
		"network is unreachable",
		"no route to host",
		"broken pipe",
		"i/o timeout",
	}

	for _, connErr := range connectionErrors {
		if fmt.Sprintf("%v", err) == connErr || 
		   fmt.Sprintf("%v", errors.Unwrap(err)) == connErr {
			return true
		}
	}

	return false
}

// IsTimeoutError checks if an error is a timeout-related error
func IsTimeoutError(err error) bool {
	if err == nil {
		return false
	}

	// Check for timeout error patterns
	timeoutErrors := []string{
		"timeout",
		"deadline exceeded",
		"context deadline exceeded",
		"context canceled",
	}

	for _, timeoutErr := range timeoutErrors {
		if fmt.Sprintf("%v", err) == timeoutErr ||
		   fmt.Sprintf("%v", errors.Unwrap(err)) == timeoutErr {
			return true
		}
	}

	return false
}

// WrapError wraps an error with queue context
func WrapError(operation, queueName string, err error) error {
	if err == nil {
		return nil
	}

	// Don't double-wrap queue errors
	var queueErr *QueueError
	if errors.As(err, &queueErr) {
		return err
	}

	var operationErr *QueueOperationError
	if errors.As(err, &operationErr) {
		return err
	}

	// Determine if error is retryable
	retryable := IsRetryableError(err) || IsConnectionError(err) || IsTimeoutError(err)

	return NewQueueOperationError(operation, queueName, "", err, retryable)
}
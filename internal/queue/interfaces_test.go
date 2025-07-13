package queue

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTaskMessage(t *testing.T) {
	tests := []struct {
		name     string
		message  *TaskMessage
		expected func(*testing.T, *TaskMessage)
	}{
		{
			name: "valid task message",
			message: &TaskMessage{
				TaskID:     uuid.New(),
				UserID:     uuid.New(),
				Priority:   PriorityNormal,
				QueuedAt:   time.Now(),
				Attempts:   0,
				MessageID:  "test-message-123",
				Attributes: map[string]string{"key": "value"},
			},
			expected: func(t *testing.T, msg *TaskMessage) {
				assert.NotEqual(t, uuid.Nil, msg.TaskID)
				assert.NotEqual(t, uuid.Nil, msg.UserID)
				assert.Equal(t, PriorityNormal, msg.Priority)
				assert.Equal(t, 0, msg.Attempts)
				assert.Equal(t, "test-message-123", msg.MessageID)
				assert.Equal(t, "value", msg.Attributes["key"])
				assert.Nil(t, msg.LastAttempt)
				assert.Nil(t, msg.NextRetryAt)
				assert.Nil(t, msg.FailureReason)
				assert.Nil(t, msg.ReceiptHandle)
			},
		},
		{
			name: "task message with retry info",
			message: &TaskMessage{
				TaskID:        uuid.New(),
				UserID:        uuid.New(),
				Priority:      PriorityHigh,
				QueuedAt:      time.Now(),
				Attempts:      2,
				LastAttempt:   timePtrHelper(time.Now().Add(-10 * time.Minute)),
				NextRetryAt:   timePtrHelper(time.Now().Add(5 * time.Minute)),
				FailureReason: stringPtrHelper("execution timeout"),
				MessageID:     "retry-message-456",
				ReceiptHandle: stringPtrHelper("receipt-123"),
			},
			expected: func(t *testing.T, msg *TaskMessage) {
				assert.NotEqual(t, uuid.Nil, msg.TaskID)
				assert.NotEqual(t, uuid.Nil, msg.UserID)
				assert.Equal(t, PriorityHigh, msg.Priority)
				assert.Equal(t, 2, msg.Attempts)
				assert.NotNil(t, msg.LastAttempt)
				assert.NotNil(t, msg.NextRetryAt)
				assert.NotNil(t, msg.FailureReason)
				assert.Equal(t, "execution timeout", *msg.FailureReason)
				assert.NotNil(t, msg.ReceiptHandle)
				assert.Equal(t, "receipt-123", *msg.ReceiptHandle)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.expected(t, tt.message)
		})
	}
}

func TestQueueError(t *testing.T) {
	tests := []struct {
		name      string
		operation string
		err       error
		retryable bool
		expected  func(*testing.T, *QueueError)
	}{
		{
			name:      "retryable error",
			operation: "enqueue",
			err:       assert.AnError,
			retryable: true,
			expected: func(t *testing.T, qErr *QueueError) {
				assert.Equal(t, "enqueue", qErr.Operation)
				assert.Equal(t, assert.AnError, qErr.Err)
				assert.True(t, qErr.Retryable)
				assert.Equal(t, assert.AnError.Error(), qErr.Error())
				assert.Equal(t, assert.AnError, qErr.Unwrap())
			},
		},
		{
			name:      "non-retryable error",
			operation: "validate",
			err:       assert.AnError,
			retryable: false,
			expected: func(t *testing.T, qErr *QueueError) {
				assert.Equal(t, "validate", qErr.Operation)
				assert.Equal(t, assert.AnError, qErr.Err)
				assert.False(t, qErr.Retryable)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			qErr := NewQueueError(tt.operation, tt.err, tt.retryable)
			tt.expected(t, qErr)
		})
	}
}

func TestPriorityConstants(t *testing.T) {
	tests := []struct {
		name     string
		priority int
		expected int
	}{
		{"lowest priority", PriorityLowest, 0},
		{"low priority", PriorityLow, 2},
		{"normal priority", PriorityNormal, 5},
		{"high priority", PriorityHigh, 8},
		{"highest priority", PriorityHighest, 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.priority)
		})
	}

	// Verify priority ordering
	assert.Less(t, PriorityLowest, PriorityLow)
	assert.Less(t, PriorityLow, PriorityNormal)
	assert.Less(t, PriorityNormal, PriorityHigh)
	assert.Less(t, PriorityHigh, PriorityHighest)
}

func TestQueueNameConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant string
		expected string
	}{
		{"task queue name", DefaultTaskQueueName, "voidrunner:tasks"},
		{"retry queue name", DefaultRetryQueueName, "voidrunner:tasks:retry"},
		{"dead letter queue name", DefaultDeadLetterQueueName, "voidrunner:tasks:dead"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.constant)
		})
	}
}

func TestQueueStats(t *testing.T) {
	stats := &QueueStats{
		Name:                "test-queue",
		ApproximateMessages: 100,
		MessagesInFlight:    5,
		MessagesDelayed:     10,
		OldestMessageAge:    &[]time.Duration{5 * time.Minute}[0],
	}

	assert.Equal(t, "test-queue", stats.Name)
	assert.Equal(t, int64(100), stats.ApproximateMessages)
	assert.Equal(t, int64(5), stats.MessagesInFlight)
	assert.Equal(t, int64(10), stats.MessagesDelayed)
	assert.NotNil(t, stats.OldestMessageAge)
	assert.Equal(t, 5*time.Minute, *stats.OldestMessageAge)
}

func TestRetryStats(t *testing.T) {
	retryStats := &RetryStats{
		QueueStats: QueueStats{
			Name:                "retry-queue",
			ApproximateMessages: 20,
		},
		PendingRetries:  15,
		ReadyForRetry:   5,
		AverageRetryAge: &[]time.Duration{2 * time.Minute}[0],
	}

	assert.Equal(t, "retry-queue", retryStats.Name)
	assert.Equal(t, int64(20), retryStats.ApproximateMessages)
	assert.Equal(t, int64(15), retryStats.PendingRetries)
	assert.Equal(t, int64(5), retryStats.ReadyForRetry)
	assert.NotNil(t, retryStats.AverageRetryAge)
	assert.Equal(t, 2*time.Minute, *retryStats.AverageRetryAge)
}

func TestDeadLetterStats(t *testing.T) {
	dlqStats := &DeadLetterStats{
		QueueStats: QueueStats{
			Name:                "dead-letter-queue",
			ApproximateMessages: 10,
		},
		TotalFailedTasks:  100,
		AverageFailureAge: &[]time.Duration{1 * time.Hour}[0],
		FailureReasons: map[string]int64{
			"timeout":         50,
			"memory_limit":    30,
			"execution_error": 20,
		},
	}

	assert.Equal(t, "dead-letter-queue", dlqStats.Name)
	assert.Equal(t, int64(10), dlqStats.ApproximateMessages)
	assert.Equal(t, int64(100), dlqStats.TotalFailedTasks)
	assert.NotNil(t, dlqStats.AverageFailureAge)
	assert.Equal(t, 1*time.Hour, *dlqStats.AverageFailureAge)

	require.Len(t, dlqStats.FailureReasons, 3)
	assert.Equal(t, int64(50), dlqStats.FailureReasons["timeout"])
	assert.Equal(t, int64(30), dlqStats.FailureReasons["memory_limit"])
	assert.Equal(t, int64(20), dlqStats.FailureReasons["execution_error"])
}

func TestQueueManagerStatsStructure(t *testing.T) {
	now := time.Now()
	uptime := 2 * time.Hour

	stats := &QueueManagerStats{
		TaskQueue: &QueueStats{
			Name:                "task-queue",
			ApproximateMessages: 50,
		},
		RetryQueue: &RetryStats{
			QueueStats: QueueStats{
				Name:                "retry-queue",
				ApproximateMessages: 10,
			},
			PendingRetries: 8,
		},
		DeadLetterQueue: &DeadLetterStats{
			QueueStats: QueueStats{
				Name:                "dlq",
				ApproximateMessages: 5,
			},
			TotalFailedTasks: 25,
		},
		TotalThroughput: 1000,
		Uptime:          uptime,
		LastUpdated:     now,
	}

	assert.NotNil(t, stats.TaskQueue)
	assert.Equal(t, "task-queue", stats.TaskQueue.Name)
	assert.Equal(t, int64(50), stats.TaskQueue.ApproximateMessages)

	assert.NotNil(t, stats.RetryQueue)
	assert.Equal(t, "retry-queue", stats.RetryQueue.Name)
	assert.Equal(t, int64(8), stats.RetryQueue.PendingRetries)

	assert.NotNil(t, stats.DeadLetterQueue)
	assert.Equal(t, "dlq", stats.DeadLetterQueue.Name)
	assert.Equal(t, int64(25), stats.DeadLetterQueue.TotalFailedTasks)

	assert.Equal(t, int64(1000), stats.TotalThroughput)
	assert.Equal(t, uptime, stats.Uptime)
	assert.Equal(t, now, stats.LastUpdated)
}

// Helper functions for tests
func timePtrHelper(t time.Time) *time.Time {
	return &t
}

func stringPtrHelper(s string) *string {
	return &s
}

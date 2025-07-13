package queue

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// TaskMessage represents a task message in the queue
type TaskMessage struct {
	// Task details
	TaskID   uuid.UUID `json:"task_id"`
	UserID   uuid.UUID `json:"user_id"`
	Priority int       `json:"priority"`

	// Queue metadata
	QueuedAt    time.Time  `json:"queued_at"`
	Attempts    int        `json:"attempts"`
	LastAttempt *time.Time `json:"last_attempt,omitempty"`

	// Retry information
	NextRetryAt   *time.Time `json:"next_retry_at,omitempty"`
	FailureReason *string    `json:"failure_reason,omitempty"`

	// Message metadata
	MessageID     string            `json:"message_id"`
	ReceiptHandle *string           `json:"receipt_handle,omitempty"`
	Attributes    map[string]string `json:"attributes,omitempty"`
}

// TaskQueue defines the interface for task queue operations
type TaskQueue interface {
	// Enqueue adds a task to the queue with priority
	Enqueue(ctx context.Context, message *TaskMessage) error

	// Dequeue retrieves tasks from the queue
	Dequeue(ctx context.Context, maxMessages int) ([]*TaskMessage, error)

	// DeleteMessage removes a processed message from the queue
	DeleteMessage(ctx context.Context, receiptHandle string) error

	// ExtendVisibility extends the visibility timeout for a message
	ExtendVisibility(ctx context.Context, receiptHandle string, timeout time.Duration) error

	// GetQueueStats returns queue statistics
	GetQueueStats(ctx context.Context) (*QueueStats, error)

	// IsHealthy checks if the queue is healthy
	IsHealthy(ctx context.Context) error

	// Close closes the queue connection
	Close() error
}

// RetryQueue defines the interface for retry queue operations
type RetryQueue interface {
	// EnqueueForRetry adds a failed task to the retry queue
	EnqueueForRetry(ctx context.Context, message *TaskMessage, retryAt time.Time) error

	// DequeueReadyForRetry retrieves tasks ready for retry
	DequeueReadyForRetry(ctx context.Context, maxMessages int) ([]*TaskMessage, error)

	// GetRetryStats returns retry queue statistics
	GetRetryStats(ctx context.Context) (*RetryStats, error)
}

// DeadLetterQueue defines the interface for dead letter queue operations
type DeadLetterQueue interface {
	// EnqueueFailedTask adds a permanently failed task to the dead letter queue
	EnqueueFailedTask(ctx context.Context, message *TaskMessage) error

	// GetFailedTasks retrieves failed tasks from the dead letter queue
	GetFailedTasks(ctx context.Context, limit int, offset int) ([]*TaskMessage, error)

	// RequeueTask moves a task from dead letter back to main queue
	RequeueTask(ctx context.Context, messageID string) error

	// GetDeadLetterStats returns dead letter queue statistics
	GetDeadLetterStats(ctx context.Context) (*DeadLetterStats, error)
}

// QueueManager manages all queue operations
type QueueManager interface {
	// Task queue operations
	TaskQueue() TaskQueue
	RetryQueue() RetryQueue
	DeadLetterQueue() DeadLetterQueue

	// Health and monitoring
	IsHealthy(ctx context.Context) error
	GetStats(ctx context.Context) (*QueueManagerStats, error)

	// Lifecycle management
	Start(ctx context.Context) error
	Stop(ctx context.Context) error

	// Background processes
	StartRetryProcessor(ctx context.Context) error
	StopRetryProcessor() error
}

// QueueStats represents statistics for a queue
type QueueStats struct {
	Name                string         `json:"name"`
	ApproximateMessages int64          `json:"approximate_messages"`
	MessagesInFlight    int64          `json:"messages_in_flight"`
	MessagesDelayed     int64          `json:"messages_delayed"`
	OldestMessageAge    *time.Duration `json:"oldest_message_age,omitempty"`
}

// RetryStats represents statistics for the retry queue
type RetryStats struct {
	QueueStats
	PendingRetries  int64          `json:"pending_retries"`
	ReadyForRetry   int64          `json:"ready_for_retry"`
	AverageRetryAge *time.Duration `json:"average_retry_age,omitempty"`
}

// DeadLetterStats represents statistics for the dead letter queue
type DeadLetterStats struct {
	QueueStats
	TotalFailedTasks  int64            `json:"total_failed_tasks"`
	AverageFailureAge *time.Duration   `json:"average_failure_age,omitempty"`
	FailureReasons    map[string]int64 `json:"failure_reasons"`
}

// QueueManagerStats represents overall queue manager statistics
type QueueManagerStats struct {
	TaskQueue       *QueueStats      `json:"task_queue"`
	RetryQueue      *RetryStats      `json:"retry_queue"`
	DeadLetterQueue *DeadLetterStats `json:"dead_letter_queue"`
	TotalThroughput int64            `json:"total_throughput"`
	Uptime          time.Duration    `json:"uptime"`
	LastUpdated     time.Time        `json:"last_updated"`
}

// QueueError represents a queue-specific error
type QueueError struct {
	Operation string
	Err       error
	Retryable bool
}

func (e *QueueError) Error() string {
	return e.Err.Error()
}

func (e *QueueError) Unwrap() error {
	return e.Err
}

// NewQueueError creates a new queue error
func NewQueueError(operation string, err error, retryable bool) *QueueError {
	return &QueueError{
		Operation: operation,
		Err:       err,
		Retryable: retryable,
	}
}

// Priority constants for task prioritization
const (
	PriorityLowest  = 0
	PriorityLow     = 2
	PriorityNormal  = 5
	PriorityHigh    = 8
	PriorityHighest = 10
)

// Queue name constants
const (
	DefaultTaskQueueName       = "voidrunner:tasks"
	DefaultRetryQueueName      = "voidrunner:tasks:retry"
	DefaultDeadLetterQueueName = "voidrunner:tasks:dead"
)

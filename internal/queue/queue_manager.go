package queue

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/voidrunnerhq/voidrunner/internal/config"
)

// RedisQueueManager manages all queue operations using Redis
type RedisQueueManager struct {
	client *RedisClient
	config *config.QueueConfig
	logger *slog.Logger

	// Queue instances
	taskQueue       TaskQueue
	retryQueue      RetryQueue
	deadLetterQueue DeadLetterQueue

	// Background processes
	retryProcessor *RetryProcessor

	// State management
	mu            sync.RWMutex
	started       bool
	closed        bool
	startTime     time.Time
	cleanupCancel context.CancelFunc
	cleanupDone   chan struct{}
}

// NewRedisQueueManager creates a new Redis-based queue manager
func NewRedisQueueManager(redisConfig *config.RedisConfig, queueConfig *config.QueueConfig, logger *slog.Logger) (*RedisQueueManager, error) {
	if redisConfig == nil {
		return nil, fmt.Errorf("redis config is required")
	}

	if queueConfig == nil {
		return nil, fmt.Errorf("queue config is required")
	}

	if logger == nil {
		logger = slog.Default()
	}

	// Create Redis client
	client, err := NewRedisClient(redisConfig, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create Redis client: %w", err)
	}

	// Test Redis connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.IsHealthy(ctx); err != nil {
		if closeErr := client.Close(); closeErr != nil {
			logger.Error("failed to close Redis client after health check failure", "error", closeErr)
		}
		return nil, fmt.Errorf("Redis health check failed: %w", err)
	}

	// Create task queue
	taskQueue, err := NewRedisTaskQueue(client, queueConfig, logger)
	if err != nil {
		if closeErr := client.Close(); closeErr != nil {
			logger.Error("failed to close Redis client after task queue creation failure", "error", closeErr)
		}
		return nil, fmt.Errorf("failed to create task queue: %w", err)
	}

	// Create retry queue
	retryQueue, err := NewRedisRetryQueue(client, queueConfig, logger)
	if err != nil {
		if closeErr := client.Close(); closeErr != nil {
			logger.Error("failed to close Redis client after retry queue creation failure", "error", closeErr)
		}
		return nil, fmt.Errorf("failed to create retry queue: %w", err)
	}

	// Create dead letter queue
	deadLetterQueue, err := NewRedisDeadLetterQueue(client, queueConfig, logger)
	if err != nil {
		if closeErr := client.Close(); closeErr != nil {
			logger.Error("failed to close Redis client after dead letter queue creation failure", "error", closeErr)
		}
		return nil, fmt.Errorf("failed to create dead letter queue: %w", err)
	}

	// Create retry processor
	retryProcessor := NewRetryProcessor(retryQueue, taskQueue, queueConfig, logger)

	manager := &RedisQueueManager{
		client:          client,
		config:          queueConfig,
		logger:          logger,
		taskQueue:       taskQueue,
		retryQueue:      retryQueue,
		deadLetterQueue: deadLetterQueue,
		retryProcessor:  retryProcessor,
		started:         false,
		closed:          false,
		cleanupDone:     make(chan struct{}),
	}

	return manager, nil
}

// TaskQueue returns the task queue instance
func (qm *RedisQueueManager) TaskQueue() TaskQueue {
	return qm.taskQueue
}

// RetryQueue returns the retry queue instance
func (qm *RedisQueueManager) RetryQueue() RetryQueue {
	return qm.retryQueue
}

// DeadLetterQueue returns the dead letter queue instance
func (qm *RedisQueueManager) DeadLetterQueue() DeadLetterQueue {
	return qm.deadLetterQueue
}

// IsHealthy checks if all queues are healthy
func (qm *RedisQueueManager) IsHealthy(ctx context.Context) error {
	qm.mu.RLock()
	defer qm.mu.RUnlock()

	return qm.isHealthyUnsafe(ctx)
}

// isHealthyUnsafe checks if all queues are healthy without acquiring locks
// This method should only be called when the caller already holds the appropriate lock
func (qm *RedisQueueManager) isHealthyUnsafe(ctx context.Context) error {
	if qm.closed {
		return ErrQueueClosed
	}

	// Check Redis client health
	if err := qm.client.IsHealthy(ctx); err != nil {
		return fmt.Errorf("Redis client health check failed: %w", err)
	}

	// Check task queue health
	if err := qm.taskQueue.IsHealthy(ctx); err != nil {
		return fmt.Errorf("task queue health check failed: %w", err)
	}

	// Check retry queue health (if available)
	if retryQueue, ok := qm.retryQueue.(*RedisRetryQueue); ok {
		if err := retryQueue.IsHealthy(ctx); err != nil {
			return fmt.Errorf("retry queue health check failed: %w", err)
		}
	}

	// Check dead letter queue health (if available)
	if deadQueue, ok := qm.deadLetterQueue.(*RedisDeadLetterQueue); ok {
		if err := deadQueue.IsHealthy(ctx); err != nil {
			return fmt.Errorf("dead letter queue health check failed: %w", err)
		}
	}

	return nil
}

// GetStats returns comprehensive queue manager statistics
func (qm *RedisQueueManager) GetStats(ctx context.Context) (*QueueManagerStats, error) {
	qm.mu.RLock()
	defer qm.mu.RUnlock()

	if qm.closed {
		return nil, ErrQueueClosed
	}

	// Get task queue stats
	taskStats, err := qm.taskQueue.GetQueueStats(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get task queue stats: %w", err)
	}

	// Get retry queue stats
	retryStats, err := qm.retryQueue.GetRetryStats(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get retry queue stats: %w", err)
	}

	// Get dead letter queue stats
	deadLetterStats, err := qm.deadLetterQueue.GetDeadLetterStats(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get dead letter queue stats: %w", err)
	}

	// Calculate total throughput (rough estimate)
	totalThroughput := taskStats.ApproximateMessages + retryStats.ApproximateMessages + deadLetterStats.ApproximateMessages

	// Calculate uptime
	var uptime time.Duration
	if qm.started {
		uptime = time.Since(qm.startTime)
	}

	stats := &QueueManagerStats{
		TaskQueue:       taskStats,
		RetryQueue:      retryStats,
		DeadLetterQueue: deadLetterStats,
		TotalThroughput: totalThroughput,
		Uptime:          uptime,
		LastUpdated:     time.Now(),
	}

	return stats, nil
}

// Start initializes the queue manager and starts background processes
func (qm *RedisQueueManager) Start(ctx context.Context) error {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	if qm.closed {
		return ErrQueueClosed
	}

	if qm.started {
		return nil // Already started
	}

	qm.logger.Info("starting queue manager")

	// Test all queue health before starting (use unsafe version since we already hold the lock)
	if err := qm.isHealthyUnsafe(ctx); err != nil {
		return fmt.Errorf("queue health check failed during startup: %w", err)
	}

	// Start background cleanup for expired messages with dedicated context
	cleanupCtx, cleanupCancel := context.WithCancel(context.Background())
	qm.cleanupCancel = cleanupCancel
	go qm.backgroundCleanup(cleanupCtx)

	qm.started = true
	qm.startTime = time.Now()

	qm.logger.Info("queue manager started successfully")
	return nil
}

// Stop gracefully stops the queue manager and background processes
func (qm *RedisQueueManager) Stop(ctx context.Context) error {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	if qm.closed {
		return nil // Already stopped
	}

	qm.logger.Info("stopping queue manager")

	// Stop background cleanup process
	if qm.cleanupCancel != nil {
		qm.cleanupCancel()
		// Wait for cleanup goroutine to finish with timeout
		select {
		case <-qm.cleanupDone:
			qm.logger.Debug("background cleanup stopped gracefully")
		case <-time.After(5 * time.Second):
			qm.logger.Warn("background cleanup stop timed out")
		}
	}

	// Stop retry processor if running
	if qm.retryProcessor != nil {
		if err := qm.retryProcessor.Stop(); err != nil {
			qm.logger.Error("failed to stop retry processor", "error", err)
		}
	}

	// Close all queues
	if err := qm.taskQueue.Close(); err != nil {
		qm.logger.Error("failed to close task queue", "error", err)
	}

	if retryQueue, ok := qm.retryQueue.(*RedisRetryQueue); ok {
		if err := retryQueue.Close(); err != nil {
			qm.logger.Error("failed to close retry queue", "error", err)
		}
	}

	if deadQueue, ok := qm.deadLetterQueue.(*RedisDeadLetterQueue); ok {
		if err := deadQueue.Close(); err != nil {
			qm.logger.Error("failed to close dead letter queue", "error", err)
		}
	}

	// Close Redis client
	if err := qm.client.Close(); err != nil {
		qm.logger.Error("failed to close Redis client", "error", err)
	}

	qm.closed = true
	qm.started = false

	qm.logger.Info("queue manager stopped successfully")
	return nil
}

// StartRetryProcessor starts the background retry processor
func (qm *RedisQueueManager) StartRetryProcessor(ctx context.Context) error {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	if qm.closed {
		return ErrQueueClosed
	}

	if qm.retryProcessor == nil {
		return fmt.Errorf("retry processor not available")
	}

	qm.logger.Info("starting retry processor")
	return qm.retryProcessor.Start(ctx)
}

// StopRetryProcessor stops the background retry processor
func (qm *RedisQueueManager) StopRetryProcessor() error {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	if qm.retryProcessor == nil {
		return nil // No processor to stop
	}

	qm.logger.Info("stopping retry processor")
	return qm.retryProcessor.Stop()
}

// backgroundCleanup runs periodic cleanup of expired messages
func (qm *RedisQueueManager) backgroundCleanup(ctx context.Context) {
	defer close(qm.cleanupDone) // Signal completion when goroutine exits

	ticker := time.NewTicker(5 * time.Minute) // Cleanup every 5 minutes
	defer ticker.Stop()

	qm.logger.Debug("starting background cleanup process")

	for {
		select {
		case <-ctx.Done():
			qm.logger.Debug("background cleanup stopped due to context cancellation")
			return
		case <-ticker.C:
			// Check if manager was closed before performing cleanup
			qm.mu.RLock()
			closed := qm.closed
			qm.mu.RUnlock()

			if closed {
				qm.logger.Debug("background cleanup stopped due to manager closure")
				return
			}

			qm.performCleanup(ctx)
		}
	}
}

// performCleanup performs the actual cleanup operations
func (qm *RedisQueueManager) performCleanup(ctx context.Context) {
	qm.logger.Debug("performing periodic queue cleanup")

	// Create context with timeout for cleanup operations
	cleanupCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	// Cleanup expired messages in task queue
	if taskQueue, ok := qm.taskQueue.(*RedisTaskQueue); ok {
		if err := taskQueue.CleanupExpiredMessages(cleanupCtx); err != nil {
			qm.logger.Error("failed to cleanup expired task queue messages", "error", err)
		}
	}

	// Cleanup expired messages in retry queue
	if retryQueue, ok := qm.retryQueue.(*RedisRetryQueue); ok {
		if err := retryQueue.CleanupExpiredMessages(cleanupCtx); err != nil {
			qm.logger.Error("failed to cleanup expired retry queue messages", "error", err)
		}
	}

	// Cleanup old dead letter messages (optional, based on configuration)
	if deadQueue, ok := qm.deadLetterQueue.(*RedisDeadLetterQueue); ok {
		if err := deadQueue.CleanupOldMessages(cleanupCtx, 7*24*time.Hour); err != nil {
			qm.logger.Error("failed to cleanup old dead letter messages", "error", err)
		}
	}

	qm.logger.Debug("periodic queue cleanup completed")
}

// EnqueueTask is a convenience method to enqueue a task with proper error handling
func (qm *RedisQueueManager) EnqueueTask(ctx context.Context, message *TaskMessage) error {
	qm.mu.RLock()
	defer qm.mu.RUnlock()

	if qm.closed {
		return ErrQueueClosed
	}

	// Validate message before enqueuing
	if err := ValidateTaskMessage(message); err != nil {
		return fmt.Errorf("task message validation failed: %w", err)
	}

	// Set default priority if not specified
	if message.Priority == 0 {
		message.Priority = qm.config.DefaultPriority
	}

	// Enqueue to task queue
	if err := qm.taskQueue.Enqueue(ctx, message); err != nil {
		qm.logger.Error("failed to enqueue task",
			"task_id", message.TaskID,
			"user_id", message.UserID,
			"error", err,
		)
		return fmt.Errorf("failed to enqueue task: %w", err)
	}

	qm.logger.Debug("task enqueued successfully",
		"task_id", message.TaskID,
		"user_id", message.UserID,
		"priority", message.Priority,
	)

	return nil
}

// DequeueTask is a convenience method to dequeue tasks with proper error handling
func (qm *RedisQueueManager) DequeueTask(ctx context.Context, maxMessages int) ([]*TaskMessage, error) {
	qm.mu.RLock()
	defer qm.mu.RUnlock()

	if qm.closed {
		return nil, ErrQueueClosed
	}

	messages, err := qm.taskQueue.Dequeue(ctx, maxMessages)
	if err != nil {
		qm.logger.Error("failed to dequeue tasks", "error", err)
		return nil, fmt.Errorf("failed to dequeue tasks: %w", err)
	}

	qm.logger.Debug("tasks dequeued successfully", "count", len(messages))
	return messages, nil
}

// CompleteTask marks a task as completed and removes it from the queue
func (qm *RedisQueueManager) CompleteTask(ctx context.Context, receiptHandle string) error {
	qm.mu.RLock()
	defer qm.mu.RUnlock()

	if qm.closed {
		return ErrQueueClosed
	}

	if err := qm.taskQueue.DeleteMessage(ctx, receiptHandle); err != nil {
		qm.logger.Error("failed to complete task", "receipt_handle", receiptHandle, "error", err)
		return fmt.Errorf("failed to complete task: %w", err)
	}

	qm.logger.Debug("task completed successfully", "receipt_handle", receiptHandle)
	return nil
}

// FailTask handles a failed task by either retrying or moving to dead letter queue
func (qm *RedisQueueManager) FailTask(ctx context.Context, message *TaskMessage, failureReason string) error {
	qm.mu.RLock()
	defer qm.mu.RUnlock()

	if qm.closed {
		return ErrQueueClosed
	}

	// Set failure reason
	message.FailureReason = &failureReason

	// Check if task is eligible for retry
	if IsRetryEligible(message, qm.config.MaxRetries) {
		// Calculate retry time
		retryAt := CalculateRetryAt(message, qm.config.RetryDelay, qm.config.RetryBackoffFactor, qm.config.MaxRetryDelay)

		// Enqueue for retry
		if err := qm.retryQueue.EnqueueForRetry(ctx, message, retryAt); err != nil {
			qm.logger.Error("failed to enqueue task for retry",
				"task_id", message.TaskID,
				"attempts", message.Attempts,
				"error", err,
			)
			return fmt.Errorf("failed to enqueue task for retry: %w", err)
		}

		qm.logger.Info("task scheduled for retry",
			"task_id", message.TaskID,
			"attempts", message.Attempts,
			"retry_at", retryAt,
		)
	} else {
		// Move to dead letter queue
		if err := qm.deadLetterQueue.EnqueueFailedTask(ctx, message); err != nil {
			qm.logger.Error("failed to enqueue task to dead letter queue",
				"task_id", message.TaskID,
				"attempts", message.Attempts,
				"error", err,
			)
			return fmt.Errorf("failed to enqueue task to dead letter queue: %w", err)
		}

		qm.logger.Warn("task moved to dead letter queue",
			"task_id", message.TaskID,
			"attempts", message.Attempts,
			"failure_reason", failureReason,
		)
	}

	// Remove from task queue
	if message.ReceiptHandle != nil {
		if err := qm.taskQueue.DeleteMessage(ctx, *message.ReceiptHandle); err != nil {
			qm.logger.Error("failed to delete failed task from queue",
				"receipt_handle", *message.ReceiptHandle,
				"error", err,
			)
			// Don't return error here as the task has already been handled
		}
	}

	return nil
}

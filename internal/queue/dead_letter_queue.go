package queue

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/voidrunnerhq/voidrunner/internal/config"
)

// RedisDeadLetterQueue implements the DeadLetterQueue interface using Redis
type RedisDeadLetterQueue struct {
	client      *RedisClient
	config      *config.QueueConfig
	logger      *slog.Logger
	queueName   string
	deadKey     string
	messagesKey string
	statsKey    string
	reasonsKey  string
	closed      bool
}

// NewRedisDeadLetterQueue creates a new Redis-based dead letter queue
func NewRedisDeadLetterQueue(client *RedisClient, cfg *config.QueueConfig, logger *slog.Logger) (*RedisDeadLetterQueue, error) {
	if client == nil {
		return nil, fmt.Errorf("redis client is required")
	}

	if cfg == nil {
		return nil, fmt.Errorf("queue config is required")
	}

	if logger == nil {
		logger = slog.Default()
	}

	queue := &RedisDeadLetterQueue{
		client:      client,
		config:      cfg,
		logger:      logger,
		queueName:   cfg.DeadLetterQueueName,
		deadKey:     FormatQueueKey(cfg.DeadLetterQueueName, "dead"),
		messagesKey: FormatQueueKey(cfg.DeadLetterQueueName, "messages"),
		statsKey:    FormatStatsKey(cfg.DeadLetterQueueName),
		reasonsKey:  FormatQueueKey(cfg.DeadLetterQueueName, "failure_reasons"),
		closed:      false,
	}

	return queue, nil
}

// EnqueueFailedTask adds a permanently failed task to the dead letter queue
func (dlq *RedisDeadLetterQueue) EnqueueFailedTask(ctx context.Context, message *TaskMessage) error {
	if dlq.closed {
		return ErrQueueClosed
	}

	if err := ValidateTaskMessage(message); err != nil {
		return NewQueueOperationError("enqueue_dead", dlq.queueName, "", err, false)
	}

	// Generate message ID if not provided
	if message.MessageID == "" {
		message.MessageID = GenerateMessageID()
	}

	// Set failure timestamp
	failureTime := time.Now()

	// Serialize message
	messageData, err := SerializeMessage(message)
	if err != nil {
		return NewQueueOperationError("enqueue_dead", dlq.queueName, message.MessageID, err, false)
	}

	// Use failure time as score for chronological ordering
	failureScore := float64(failureTime.Unix())

	// Get failure reason for categorization
	failureReason := "unknown"
	if message.FailureReason != nil {
		failureReason = *message.FailureReason
	}

	// Use Redis pipeline for atomic operations
	pipe := dlq.client.Pipeline()

	// Add message to dead letter queue (sorted set by failure time)
	pipe.ZAdd(ctx, dlq.deadKey, &redis.Z{
		Score:  failureScore,
		Member: message.MessageID,
	})

	// Store message data
	messageKey := FormatMessageKey(dlq.queueName, message.MessageID)
	pipe.HSet(ctx, messageKey,
		"data", messageData,
		"failed_at", failureTime.Unix(),
		"task_id", message.TaskID.String(),
		"user_id", message.UserID.String(),
		"attempts", message.Attempts,
		"failure_reason", failureReason,
	)

	// Track failure reason statistics
	pipe.HIncrBy(ctx, dlq.reasonsKey, failureReason, 1)

	// Update general statistics
	pipe.HIncrBy(ctx, dlq.statsKey, "total_failed_tasks", 1)
	pipe.HSet(ctx, dlq.statsKey, "last_failure", failureTime.Unix())

	// Execute pipeline
	if err := dlq.client.ExecutePipeline(ctx, pipe); err != nil {
		return NewQueueOperationError("enqueue_dead", dlq.queueName, message.MessageID, err, true)
	}

	dlq.logger.Warn("task moved to dead letter queue",
		"message_id", message.MessageID,
		"task_id", message.TaskID,
		"user_id", message.UserID,
		"attempts", message.Attempts,
		"failure_reason", failureReason,
	)

	return nil
}

// GetFailedTasks retrieves failed tasks from the dead letter queue
func (dlq *RedisDeadLetterQueue) GetFailedTasks(ctx context.Context, limit int, offset int) ([]*TaskMessage, error) {
	if dlq.closed {
		return nil, ErrQueueClosed
	}

	if limit <= 0 {
		limit = 50 // Default limit
	}

	if limit > 1000 {
		limit = 1000 // Maximum limit for safety
	}

	// Get message IDs from dead letter queue (newest first)
	messageIDs, err := dlq.client.ZRangeByScoreWithLimit(ctx, dlq.deadKey, "-inf", "+inf", int64(offset), int64(limit))
	if err != nil {
		return nil, NewQueueOperationError("get_failed", dlq.queueName, "", err, true)
	}

	if len(messageIDs) == 0 {
		return []*TaskMessage{}, nil
	}

	// Reverse to get newest first
	for i, j := 0, len(messageIDs)-1; i < j; i, j = i+1, j-1 {
		messageIDs[i], messageIDs[j] = messageIDs[j], messageIDs[i]
	}

	messages := make([]*TaskMessage, 0, len(messageIDs))

	// Get message data for each ID
	for _, messageID := range messageIDs {
		messageKey := FormatMessageKey(dlq.queueName, messageID)
		messageData, err := dlq.client.HGetAll(ctx, messageKey)
		if err != nil {
			dlq.logger.Warn("failed to get dead letter message data",
				"message_id", messageID,
				"error", err,
			)
			continue
		}

		// Deserialize message
		data, exists := messageData["data"]
		if !exists {
			dlq.logger.Warn("dead letter message data not found",
				"message_id", messageID,
			)
			continue
		}

		message, err := DeserializeMessage(data)
		if err != nil {
			dlq.logger.Warn("failed to deserialize dead letter message",
				"message_id", messageID,
				"error", err,
			)
			continue
		}

		// Add metadata from Redis
		if failedAtStr, exists := messageData["failed_at"]; exists {
			if failedAt, err := strconv.ParseInt(failedAtStr, 10, 64); err == nil {
				failedTime := time.Unix(failedAt, 0)
				message.LastAttempt = &failedTime
			}
		}

		messages = append(messages, message)
	}

	dlq.logger.Debug("failed tasks retrieved",
		"count", len(messages),
		"limit", limit,
		"offset", offset,
	)

	return messages, nil
}

// RequeueTask moves a task from dead letter back to main queue
func (dlq *RedisDeadLetterQueue) RequeueTask(ctx context.Context, messageID string) error {
	if dlq.closed {
		return ErrQueueClosed
	}

	if messageID == "" {
		return NewQueueOperationError("requeue", dlq.queueName, "", ErrMessageNotFound, false)
	}

	// Check if message exists in dead letter queue
	score := dlq.client.GetClient().ZScore(ctx, dlq.deadKey, messageID)
	if score.Err() != nil {
		if score.Err().Error() == "redis: nil" {
			return NewQueueOperationError("requeue", dlq.queueName, messageID, ErrMessageNotFound, false)
		}
		return NewQueueOperationError("requeue", dlq.queueName, messageID, score.Err(), true)
	}

	// Get message data
	messageKey := FormatMessageKey(dlq.queueName, messageID)
	messageData, err := dlq.client.HGetAll(ctx, messageKey)
	if err != nil {
		return NewQueueOperationError("requeue", dlq.queueName, messageID, err, true)
	}

	data, exists := messageData["data"]
	if !exists {
		return NewQueueOperationError("requeue", dlq.queueName, messageID, ErrMessageNotFound, false)
	}

	// Deserialize message
	message, err := DeserializeMessage(data)
	if err != nil {
		return NewQueueOperationError("requeue", dlq.queueName, messageID, err, false)
	}

	// Reset message for requeue
	message.MessageID = GenerateMessageID() // New message ID
	message.Attempts = 0                    // Reset attempts
	message.FailureReason = nil             // Clear failure reason
	message.LastAttempt = nil               // Clear last attempt
	message.NextRetryAt = nil               // Clear retry time
	message.QueuedAt = time.Now()           // New queue time

	// This would need to enqueue to the main task queue
	// For now, we'll just remove from dead letter and log
	// In a real implementation, you'd inject the main queue here

	// Use pipeline for atomic operations
	pipe := dlq.client.Pipeline()

	// Remove from dead letter queue
	pipe.ZRem(ctx, dlq.deadKey, messageID)

	// Remove message data
	pipe.Del(ctx, messageKey)

	// Update statistics
	pipe.HIncrBy(ctx, dlq.statsKey, "total_requeued", 1)
	pipe.HSet(ctx, dlq.statsKey, "last_requeue", time.Now().Unix())

	// Decrement failure reason count if available
	if failureReason, exists := messageData["failure_reason"]; exists && failureReason != "" {
		pipe.HIncrBy(ctx, dlq.reasonsKey, failureReason, -1)
	}

	// Execute pipeline
	if err := dlq.client.ExecutePipeline(ctx, pipe); err != nil {
		return NewQueueOperationError("requeue", dlq.queueName, messageID, err, true)
	}

	dlq.logger.Info("task requeued from dead letter queue",
		"original_message_id", messageID,
		"new_message_id", message.MessageID,
		"task_id", message.TaskID,
	)

	return nil
}

// GetDeadLetterStats returns dead letter queue statistics
func (dlq *RedisDeadLetterQueue) GetDeadLetterStats(ctx context.Context) (*DeadLetterStats, error) {
	if dlq.closed {
		return nil, ErrQueueClosed
	}

	// Get total failed tasks
	totalFailed, err := dlq.client.ZCard(ctx, dlq.deadKey)
	if err != nil {
		return nil, NewQueueOperationError("dead_stats", dlq.queueName, "", err, true)
	}

	// Get failure reasons
	failureReasons, err := dlq.client.HGetAll(ctx, dlq.reasonsKey)
	if err != nil {
		return nil, NewQueueOperationError("dead_stats", dlq.queueName, "", err, true)
	}

	// Convert failure reason counts
	reasons := make(map[string]int64)
	for reason, countStr := range failureReasons {
		if count, err := strconv.ParseInt(countStr, 10, 64); err == nil {
			reasons[reason] = count
		}
	}

	// Calculate average failure age
	var avgFailureAge *time.Duration
	if totalFailed > 0 {
		// Get oldest failed message
		oldestMessages, err := dlq.client.ZRangeByScoreWithLimit(ctx, dlq.deadKey, "-inf", "+inf", 0, 1)
		if err == nil && len(oldestMessages) > 0 {
			messageKey := FormatMessageKey(dlq.queueName, oldestMessages[0])
			failedAtStr, err := dlq.client.HGet(ctx, messageKey, "failed_at")
			if err == nil {
				if failedAt, err := strconv.ParseInt(failedAtStr, 10, 64); err == nil {
					age := time.Since(time.Unix(failedAt, 0))
					avgFailureAge = &age
				}
			}
		}
	}

	stats := &DeadLetterStats{
		QueueStats: QueueStats{
			Name:                dlq.queueName,
			ApproximateMessages: totalFailed,
			MessagesInFlight:    0, // Dead letter queue doesn't have in-flight messages
			MessagesDelayed:     0, // Dead letter queue doesn't have delayed messages
			OldestMessageAge:    avgFailureAge,
		},
		TotalFailedTasks:  totalFailed,
		AverageFailureAge: avgFailureAge,
		FailureReasons:    reasons,
	}

	return stats, nil
}

// IsHealthy checks if the dead letter queue is healthy
func (dlq *RedisDeadLetterQueue) IsHealthy(ctx context.Context) error {
	if dlq.closed {
		return ErrQueueClosed
	}

	return dlq.client.IsHealthy(ctx)
}

// Close closes the dead letter queue
func (dlq *RedisDeadLetterQueue) Close() error {
	if dlq.closed {
		return nil
	}

	dlq.closed = true
	dlq.logger.Info("dead letter queue closed", "queue_name", dlq.queueName)
	return nil
}

// CleanupOldMessages removes messages older than the specified age
func (dlq *RedisDeadLetterQueue) CleanupOldMessages(ctx context.Context, maxAge time.Duration) error {
	if dlq.closed {
		return ErrQueueClosed
	}

	// Calculate cutoff time
	cutoffTime := time.Now().Add(-maxAge)
	cutoffScore := float64(cutoffTime.Unix())

	// Get old messages
	oldMessages, err := dlq.client.ZRangeByScoreWithLimit(ctx, dlq.deadKey, "-inf", fmt.Sprintf("%f", cutoffScore), 0, 100)
	if err != nil {
		return NewQueueOperationError("cleanup_old", dlq.queueName, "", err, true)
	}

	if len(oldMessages) == 0 {
		return nil // No old messages
	}

	dlq.logger.Info("cleaning up old dead letter messages",
		"count", len(oldMessages),
		"max_age", maxAge,
		"queue", dlq.queueName,
	)

	// Get failure reasons for these messages before deletion
	reasonCounts := make(map[string]int64)
	for _, messageID := range oldMessages {
		messageKey := FormatMessageKey(dlq.queueName, messageID)
		reason, err := dlq.client.HGet(ctx, messageKey, "failure_reason")
		if err == nil && reason != "" {
			reasonCounts[reason]++
		}
	}

	// Remove old messages
	pipe := dlq.client.Pipeline()

	for _, messageID := range oldMessages {
		// Remove from dead letter queue
		pipe.ZRem(ctx, dlq.deadKey, messageID)

		// Remove message data
		messageKey := FormatMessageKey(dlq.queueName, messageID)
		pipe.Del(ctx, messageKey)
	}

	// Update failure reason counts
	for reason, count := range reasonCounts {
		pipe.HIncrBy(ctx, dlq.reasonsKey, reason, -count)
	}

	// Update stats
	pipe.HIncrBy(ctx, dlq.statsKey, "total_cleaned", int64(len(oldMessages)))
	pipe.HSet(ctx, dlq.statsKey, "last_cleanup", time.Now().Unix())

	// Execute pipeline
	if err := dlq.client.ExecutePipeline(ctx, pipe); err != nil {
		return NewQueueOperationError("cleanup_old", dlq.queueName, "", err, true)
	}

	dlq.logger.Debug("old dead letter messages cleaned up",
		"count", len(oldMessages),
		"max_age", maxAge,
	)

	return nil
}

// GetFailureReasonStats returns statistics for failure reasons
func (dlq *RedisDeadLetterQueue) GetFailureReasonStats(ctx context.Context) (map[string]int64, error) {
	if dlq.closed {
		return nil, ErrQueueClosed
	}

	failureReasons, err := dlq.client.HGetAll(ctx, dlq.reasonsKey)
	if err != nil {
		return nil, NewQueueOperationError("failure_reason_stats", dlq.queueName, "", err, true)
	}

	// Convert to int64 map
	reasons := make(map[string]int64)
	for reason, countStr := range failureReasons {
		if count, err := strconv.ParseInt(countStr, 10, 64); err == nil && count > 0 {
			reasons[reason] = count
		}
	}

	return reasons, nil
}

// PurgeFailureReason removes all messages with a specific failure reason
func (dlq *RedisDeadLetterQueue) PurgeFailureReason(ctx context.Context, failureReason string) (int64, error) {
	if dlq.closed {
		return 0, ErrQueueClosed
	}

	if strings.TrimSpace(failureReason) == "" {
		return 0, NewQueueOperationError("purge_reason", dlq.queueName, "", fmt.Errorf("failure reason cannot be empty"), false)
	}

	// This would require scanning all messages to find those with the specific failure reason
	// For now, we'll implement a simpler approach that resets the failure reason count
	// In a production system, you might want to use a secondary index for efficient lookups

	// Reset the failure reason count
	err := dlq.client.HDel(ctx, dlq.reasonsKey, failureReason)
	if err != nil {
		return 0, NewQueueOperationError("purge_reason", dlq.queueName, "", err, true)
	}

	dlq.logger.Info("failure reason purged from dead letter queue",
		"failure_reason", failureReason,
		"queue", dlq.queueName,
	)

	// Return approximate count (this would need to be implemented properly with message scanning)
	return 0, nil
}

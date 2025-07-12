package queue

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/voidrunnerhq/voidrunner/internal/config"
)

// RedisRetryQueue implements the RetryQueue interface using Redis sorted sets
type RedisRetryQueue struct {
	client      *RedisClient
	config      *config.QueueConfig
	logger      *slog.Logger
	queueName   string
	retryKey    string
	messagesKey string
	statsKey    string
	closed      bool
}

// NewRedisRetryQueue creates a new Redis-based retry queue
func NewRedisRetryQueue(client *RedisClient, cfg *config.QueueConfig, logger *slog.Logger) (*RedisRetryQueue, error) {
	if client == nil {
		return nil, fmt.Errorf("redis client is required")
	}

	if cfg == nil {
		return nil, fmt.Errorf("queue config is required")
	}

	if logger == nil {
		logger = slog.Default()
	}

	queue := &RedisRetryQueue{
		client:      client,
		config:      cfg,
		logger:      logger,
		queueName:   cfg.RetryQueueName,
		retryKey:    FormatQueueKey(cfg.RetryQueueName, "retry"),
		messagesKey: FormatQueueKey(cfg.RetryQueueName, "messages"),
		statsKey:    FormatStatsKey(cfg.RetryQueueName),
		closed:      false,
	}

	return queue, nil
}

// EnqueueForRetry adds a failed task to the retry queue
func (rq *RedisRetryQueue) EnqueueForRetry(ctx context.Context, message *TaskMessage, retryAt time.Time) error {
	if rq.closed {
		return ErrQueueClosed
	}

	if err := ValidateTaskMessage(message); err != nil {
		return NewQueueOperationError("enqueue_retry", rq.queueName, "", err, false)
	}

	// Create retry message
	retryMessage := CreateRetryMessage(message)
	retryMessage.NextRetryAt = &retryAt

	// Serialize message
	messageData, err := SerializeMessage(retryMessage)
	if err != nil {
		return NewQueueOperationError("enqueue_retry", rq.queueName, retryMessage.MessageID, err, false)
	}

	// Use retry time as score for sorting
	retryScore := float64(retryAt.Unix())

	// Use Redis pipeline for atomic operations
	pipe := rq.client.Pipeline()

	// Add message to retry queue (sorted set by retry time)
	pipe.ZAdd(ctx, rq.retryKey, &redis.Z{
		Score:  retryScore,
		Member: retryMessage.MessageID,
	})

	// Store message data
	messageKey := FormatMessageKey(rq.queueName, retryMessage.MessageID)
	pipe.HSet(ctx, messageKey,
		"data", messageData,
		"retry_at", retryAt.Unix(),
		"original_task_id", message.TaskID.String(),
		"attempts", retryMessage.Attempts,
		"failure_reason", getStringValue(message.FailureReason),
	)

	// Set TTL for message data
	if rq.config.MessageTTL > 0 {
		pipe.Expire(ctx, messageKey, rq.config.MessageTTL)
	}

	// Update statistics
	pipe.HIncrBy(ctx, rq.statsKey, "total_retries_scheduled", 1)
	pipe.HSet(ctx, rq.statsKey, "last_retry_scheduled", time.Now().Unix())

	// Execute pipeline
	if err := rq.client.ExecutePipeline(ctx, pipe); err != nil {
		return NewQueueOperationError("enqueue_retry", rq.queueName, retryMessage.MessageID, err, true)
	}

	rq.logger.Debug("message scheduled for retry",
		"message_id", retryMessage.MessageID,
		"task_id", message.TaskID,
		"retry_at", retryAt,
		"attempts", retryMessage.Attempts,
	)

	return nil
}

// DequeueReadyForRetry retrieves tasks ready for retry
func (rq *RedisRetryQueue) DequeueReadyForRetry(ctx context.Context, maxMessages int) ([]*TaskMessage, error) {
	if rq.closed {
		return nil, ErrQueueClosed
	}

	if maxMessages <= 0 {
		maxMessages = rq.config.BatchSize
	}

	// Limit batch size to configuration
	if maxMessages > rq.config.BatchSize {
		maxMessages = rq.config.BatchSize
	}

	currentTime := time.Now().Unix()

	// Use Lua script for atomic dequeue operation
	script := `
		-- Get messages ready for retry (score <= current time)
		local messageIds = redis.call('ZRANGEBYSCORE', KEYS[1], '-inf', ARGV[1], 'LIMIT', 0, ARGV[2])
		if #messageIds == 0 then
			return {}
		end
		
		local results = {}
		
		for i, messageId in ipairs(messageIds) do
			-- Remove from retry queue
			redis.call('ZREM', KEYS[1], messageId)
			
			-- Update stats
			redis.call('HINCRBY', KEYS[3], 'total_retries_processed', 1)
			
			table.insert(results, messageId)
		end
		
		redis.call('HSET', KEYS[3], 'last_retry_processed', ARGV[1])
		return results
	`

	keys := []string{
		rq.retryKey,  // KEYS[1]: retry queue
		rq.messagesKey, // KEYS[2]: message data (not used in script but kept for consistency)
		rq.statsKey,  // KEYS[3]: stats key
	}

	args := []interface{}{
		currentTime,
		maxMessages,
	}

	result, err := rq.client.ExecuteLuaScript(ctx, script, keys, args...)
	if err != nil {
		return nil, NewQueueOperationError("dequeue_retry", rq.queueName, "", err, true)
	}

	// Parse script results
	scriptResults, ok := result.([]interface{})
	if !ok {
		return []*TaskMessage{}, nil // No messages ready for retry
	}

	messages := make([]*TaskMessage, 0, len(scriptResults))

	for _, item := range scriptResults {
		messageID, ok := item.(string)
		if !ok {
			continue
		}

		// Get message data
		messageKey := FormatMessageKey(rq.queueName, messageID)
		messageData, err := rq.client.HGetAll(ctx, messageKey)
		if err != nil {
			rq.logger.Warn("failed to get retry message data",
				"message_id", messageID,
				"error", err,
			)
			continue
		}

		// Deserialize message
		data, exists := messageData["data"]
		if !exists {
			rq.logger.Warn("retry message data not found",
				"message_id", messageID,
			)
			continue
		}

		message, err := DeserializeMessage(data)
		if err != nil {
			rq.logger.Warn("failed to deserialize retry message",
				"message_id", messageID,
				"error", err,
			)
			continue
		}

		// Clean up message data after successful dequeue
		go func(key string) {
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := rq.client.Del(cleanupCtx, key); err != nil {
				rq.logger.Warn("failed to cleanup retry message data", "key", key, "error", err)
			}
		}(messageKey)

		messages = append(messages, message)
	}

	rq.logger.Debug("retry messages dequeued successfully",
		"count", len(messages),
		"requested", maxMessages,
	)

	return messages, nil
}

// GetRetryStats returns retry queue statistics
func (rq *RedisRetryQueue) GetRetryStats(ctx context.Context) (*RetryStats, error) {
	if rq.closed {
		return nil, ErrQueueClosed
	}

	// Get total messages in retry queue
	totalRetries, err := rq.client.ZCard(ctx, rq.retryKey)
	if err != nil {
		return nil, NewQueueOperationError("retry_stats", rq.queueName, "", err, true)
	}

	// Get messages ready for retry (score <= current time)
	currentTime := time.Now().Unix()
	readyForRetry, err := rq.client.ZRangeByScoreWithLimit(ctx, rq.retryKey, "-inf", fmt.Sprintf("%d", currentTime), 0, -1)
	if err != nil {
		return nil, NewQueueOperationError("retry_stats", rq.queueName, "", err, true)
	}

	readyCount := int64(len(readyForRetry))
	pendingCount := totalRetries - readyCount

	// Calculate average retry age for ready messages
	var avgRetryAge *time.Duration
	if readyCount > 0 {
		// Get oldest ready message
		if len(readyForRetry) > 0 {
			messageKey := FormatMessageKey(rq.queueName, readyForRetry[0])
			retryAtStr, err := rq.client.HGet(ctx, messageKey, "retry_at")
			if err == nil {
				if retryAt, err := strconv.ParseInt(retryAtStr, 10, 64); err == nil {
					age := time.Since(time.Unix(retryAt, 0))
					avgRetryAge = &age
				}
			}
		}
	}

	stats := &RetryStats{
		QueueStats: QueueStats{
			Name:              rq.queueName,
			ApproximateMessages: totalRetries,
			MessagesInFlight:   0, // Retry queue doesn't have in-flight messages
			MessagesDelayed:    pendingCount,
			OldestMessageAge:   avgRetryAge,
		},
		PendingRetries:  pendingCount,
		ReadyForRetry:   readyCount,
		AverageRetryAge: avgRetryAge,
	}

	return stats, nil
}

// IsHealthy checks if the retry queue is healthy
func (rq *RedisRetryQueue) IsHealthy(ctx context.Context) error {
	if rq.closed {
		return ErrQueueClosed
	}

	return rq.client.IsHealthy(ctx)
}

// Close closes the retry queue
func (rq *RedisRetryQueue) Close() error {
	if rq.closed {
		return nil
	}

	rq.closed = true
	rq.logger.Info("retry queue closed", "queue_name", rq.queueName)
	return nil
}

// CleanupExpiredMessages removes expired retry messages
func (rq *RedisRetryQueue) CleanupExpiredMessages(ctx context.Context) error {
	if rq.closed {
		return ErrQueueClosed
	}

	// Calculate expiry time (messages older than max retry delay should be cleaned up)
	expiryTime := time.Now().Add(-rq.config.MaxRetryDelay * 2) // 2x max retry delay for safety
	expiryScore := float64(expiryTime.Unix())

	// Get expired messages
	expiredMessages, err := rq.client.ZRangeByScoreWithLimit(ctx, rq.retryKey, "-inf", fmt.Sprintf("%f", expiryScore), 0, 100)
	if err != nil {
		return NewQueueOperationError("cleanup_expired", rq.queueName, "", err, true)
	}

	if len(expiredMessages) == 0 {
		return nil // No expired messages
	}

	rq.logger.Info("cleaning up expired retry messages",
		"count", len(expiredMessages),
		"queue", rq.queueName,
	)

	// Remove expired messages
	pipe := rq.client.Pipeline()

	for _, messageID := range expiredMessages {
		// Remove from retry queue
		pipe.ZRem(ctx, rq.retryKey, messageID)

		// Remove message data
		messageKey := FormatMessageKey(rq.queueName, messageID)
		pipe.Del(ctx, messageKey)
	}

	// Update stats
	pipe.HIncrBy(ctx, rq.statsKey, "total_expired_cleaned", int64(len(expiredMessages)))
	pipe.HSet(ctx, rq.statsKey, "last_cleanup", time.Now().Unix())

	// Execute pipeline
	if err := rq.client.ExecutePipeline(ctx, pipe); err != nil {
		return NewQueueOperationError("cleanup_expired", rq.queueName, "", err, true)
	}

	rq.logger.Debug("expired retry messages cleaned up",
		"count", len(expiredMessages),
	)

	return nil
}

// getStringValue safely gets string value from pointer
func getStringValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
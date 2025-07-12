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

// RedisTaskQueue implements the TaskQueue interface using Redis sorted sets
type RedisTaskQueue struct {
	client       *RedisClient
	config       *config.QueueConfig
	logger       *slog.Logger
	queueName    string
	messagesKey  string
	inFlightKey  string
	statsKey     string
	closed       bool
}

// NewRedisTaskQueue creates a new Redis-based task queue
func NewRedisTaskQueue(client *RedisClient, cfg *config.QueueConfig, logger *slog.Logger) (*RedisTaskQueue, error) {
	if client == nil {
		return nil, fmt.Errorf("redis client is required")
	}

	if cfg == nil {
		return nil, fmt.Errorf("queue config is required")
	}

	if logger == nil {
		logger = slog.Default()
	}

	queue := &RedisTaskQueue{
		client:       client,
		config:       cfg,
		logger:       logger,
		queueName:    cfg.TaskQueueName,
		messagesKey:  FormatQueueKey(cfg.TaskQueueName, "queue"),
		inFlightKey:  FormatQueueKey(cfg.TaskQueueName, "inflight"),
		statsKey:     FormatStatsKey(cfg.TaskQueueName),
		closed:       false,
	}

	return queue, nil
}

// Enqueue adds a task to the queue with priority
func (q *RedisTaskQueue) Enqueue(ctx context.Context, message *TaskMessage) error {
	if q.closed {
		return ErrQueueClosed
	}

	if err := ValidateTaskMessage(message); err != nil {
		return NewQueueOperationError("enqueue", q.queueName, "", err, false)
	}

	// Generate message ID if not provided
	if message.MessageID == "" {
		message.MessageID = GenerateMessageID()
	}

	// Set queued timestamp if not provided
	if message.QueuedAt.IsZero() {
		message.QueuedAt = time.Now()
	}

	// Calculate priority score for sorted set
	priorityScore := CalculatePriorityScore(message.Priority, message.QueuedAt)

	// Serialize message
	messageData, err := SerializeMessage(message)
	if err != nil {
		return NewQueueOperationError("enqueue", q.queueName, message.MessageID, err, false)
	}

	// Use Redis pipeline for atomic operations
	pipe := q.client.Pipeline()

	// Add message to priority queue (sorted set)
	pipe.ZAdd(ctx, q.messagesKey, &redis.Z{
		Score:  priorityScore,
		Member: message.MessageID,
	})

	// Store message data in hash
	messageKey := FormatMessageKey(q.queueName, message.MessageID)
	pipe.HSet(ctx, messageKey, 
		"data", messageData,
		"priority", message.Priority,
		"queued_at", message.QueuedAt.Unix(),
		"attempts", message.Attempts,
	)

	// Set TTL for message data
	if q.config.MessageTTL > 0 {
		pipe.Expire(ctx, messageKey, q.config.MessageTTL)
	}

	// Update queue statistics
	pipe.HIncrBy(ctx, q.statsKey, "total_enqueued", 1)
	pipe.HSet(ctx, q.statsKey, "last_enqueue", time.Now().Unix())

	// Execute pipeline
	if err := q.client.ExecutePipeline(ctx, pipe); err != nil {
		return NewQueueOperationError("enqueue", q.queueName, message.MessageID, err, true)
	}

	q.logger.Debug("message enqueued successfully",
		"message_id", message.MessageID,
		"task_id", message.TaskID,
		"priority", message.Priority,
		"priority_score", priorityScore,
	)

	return nil
}

// Dequeue retrieves tasks from the queue
func (q *RedisTaskQueue) Dequeue(ctx context.Context, maxMessages int) ([]*TaskMessage, error) {
	if q.closed {
		return nil, ErrQueueClosed
	}

	if maxMessages <= 0 {
		maxMessages = q.config.BatchSize
	}

	// Limit batch size to configuration
	if maxMessages > q.config.BatchSize {
		maxMessages = q.config.BatchSize
	}

	// Use Lua script for atomic dequeue operation
	script := `
		-- Get messages from priority queue (lowest score first)
		local messageIds = redis.call('ZRANGE', KEYS[1], 0, ARGV[1] - 1)
		if #messageIds == 0 then
			return {}
		end
		
		-- Move messages to in-flight queue
		local currentTime = tonumber(ARGV[2])
		local visibilityTimeout = tonumber(ARGV[3])
		local results = {}
		
		for i, messageId in ipairs(messageIds) do
			-- Remove from main queue
			redis.call('ZREM', KEYS[1], messageId)
			
			-- Add to in-flight with visibility timeout
			local visibilityScore = currentTime + visibilityTimeout
			redis.call('ZADD', KEYS[2], visibilityScore, messageId)
			
			-- Generate receipt handle
			local receiptHandle = messageId .. ':' .. currentTime .. ':' .. math.random(10000, 99999)
			
			-- Store receipt handle
			local messageKey = KEYS[3] .. ':' .. messageId
			redis.call('HSET', messageKey, 'receipt_handle', receiptHandle, 'dequeued_at', currentTime)
			
			-- Update stats
			redis.call('HINCRBY', KEYS[4], 'total_dequeued', 1)
			
			table.insert(results, {messageId, receiptHandle})
		end
		
		redis.call('HSET', KEYS[4], 'last_dequeue', currentTime)
		return results
	`

	keys := []string{
		q.messagesKey,                                    // KEYS[1]: main queue
		q.inFlightKey,                                    // KEYS[2]: in-flight queue
		FormatQueueKey(q.queueName, "messages"),          // KEYS[3]: message data prefix
		q.statsKey,                                       // KEYS[4]: stats key
	}

	args := []interface{}{
		maxMessages,
		time.Now().Unix(),
		int64(q.config.VisibilityTimeout.Seconds()),
	}

	result, err := q.client.ExecuteLuaScript(ctx, script, keys, args...)
	if err != nil {
		return nil, NewQueueOperationError("dequeue", q.queueName, "", err, true)
	}

	// Parse script results
	scriptResults, ok := result.([]interface{})
	if !ok {
		return []*TaskMessage{}, nil // No messages available
	}

	messages := make([]*TaskMessage, 0, len(scriptResults))

	for _, item := range scriptResults {
		pair, ok := item.([]interface{})
		if !ok || len(pair) != 2 {
			continue
		}

		messageID, ok1 := pair[0].(string)
		receiptHandle, ok2 := pair[1].(string)
		if !ok1 || !ok2 {
			continue
		}

		// Get message data
		messageKey := FormatMessageKey(q.queueName, messageID)
		messageData, err := q.client.HGetAll(ctx, messageKey)
		if err != nil {
			q.logger.Warn("failed to get message data during dequeue",
				"message_id", messageID,
				"error", err,
			)
			continue
		}

		// Deserialize message
		data, exists := messageData["data"]
		if !exists {
			q.logger.Warn("message data not found",
				"message_id", messageID,
			)
			continue
		}

		message, err := DeserializeMessage(data)
		if err != nil {
			q.logger.Warn("failed to deserialize message",
				"message_id", messageID,
				"error", err,
			)
			continue
		}

		// Set receipt handle for message processing
		message.ReceiptHandle = &receiptHandle

		messages = append(messages, message)
	}

	q.logger.Debug("messages dequeued successfully",
		"count", len(messages),
		"requested", maxMessages,
	)

	return messages, nil
}

// DeleteMessage removes a processed message from the queue
func (q *RedisTaskQueue) DeleteMessage(ctx context.Context, receiptHandle string) error {
	if q.closed {
		return ErrQueueClosed
	}

	if receiptHandle == "" {
		return NewQueueOperationError("delete", q.queueName, "", ErrInvalidReceiptHandle, false)
	}

	// Parse receipt handle to get message ID
	messageID, _, err := ParseReceiptHandle(receiptHandle)
	if err != nil {
		return NewQueueOperationError("delete", q.queueName, "", err, false)
	}

	// Check if receipt handle is expired
	if IsReceiptHandleExpired(receiptHandle, q.config.VisibilityTimeout) {
		return NewQueueOperationError("delete", q.queueName, messageID, ErrMessageExpired, false)
	}

	// Verify receipt handle matches stored one
	messageKey := FormatMessageKey(q.queueName, messageID)
	storedHandle, err := q.client.HGet(ctx, messageKey, "receipt_handle")
	if err != nil {
		return NewQueueOperationError("delete", q.queueName, messageID, err, true)
	}

	if storedHandle != receiptHandle {
		return NewQueueOperationError("delete", q.queueName, messageID, ErrInvalidReceiptHandle, false)
	}

	// Use pipeline for atomic deletion
	pipe := q.client.Pipeline()

	// Remove from in-flight queue
	pipe.ZRem(ctx, q.inFlightKey, messageID)

	// Delete message data
	pipe.Del(ctx, messageKey)

	// Update statistics
	pipe.HIncrBy(ctx, q.statsKey, "total_deleted", 1)
	pipe.HSet(ctx, q.statsKey, "last_delete", time.Now().Unix())

	// Execute pipeline
	if err := q.client.ExecutePipeline(ctx, pipe); err != nil {
		return NewQueueOperationError("delete", q.queueName, messageID, err, true)
	}

	q.logger.Debug("message deleted successfully",
		"message_id", messageID,
	)

	return nil
}

// ExtendVisibility extends the visibility timeout for a message
func (q *RedisTaskQueue) ExtendVisibility(ctx context.Context, receiptHandle string, timeout time.Duration) error {
	if q.closed {
		return ErrQueueClosed
	}

	if receiptHandle == "" {
		return NewQueueOperationError("extend_visibility", q.queueName, "", ErrInvalidReceiptHandle, false)
	}

	// Parse receipt handle to get message ID
	messageID, _, err := ParseReceiptHandle(receiptHandle)
	if err != nil {
		return NewQueueOperationError("extend_visibility", q.queueName, "", err, false)
	}

	// Verify receipt handle matches stored one
	messageKey := FormatMessageKey(q.queueName, messageID)
	storedHandle, err := q.client.HGet(ctx, messageKey, "receipt_handle")
	if err != nil {
		return NewQueueOperationError("extend_visibility", q.queueName, messageID, err, true)
	}

	if storedHandle != receiptHandle {
		return NewQueueOperationError("extend_visibility", q.queueName, messageID, ErrInvalidReceiptHandle, false)
	}

	// Update visibility score in in-flight queue
	newVisibilityScore := float64(time.Now().Add(timeout).Unix())
	err = q.client.ZAddWithScore(ctx, q.inFlightKey, newVisibilityScore, messageID)
	if err != nil {
		return NewQueueOperationError("extend_visibility", q.queueName, messageID, err, true)
	}

	q.logger.Debug("message visibility extended",
		"message_id", messageID,
		"timeout", timeout,
	)

	return nil
}

// GetQueueStats returns queue statistics
func (q *RedisTaskQueue) GetQueueStats(ctx context.Context) (*QueueStats, error) {
	if q.closed {
		return nil, ErrQueueClosed
	}

	// Get counts using pipeline
	pipe := q.client.Pipeline()
	mainQueueCount := pipe.ZCard(ctx, q.messagesKey)
	inFlightCount := pipe.ZCard(ctx, q.inFlightKey)
	
	// Execute pipeline
	if err := q.client.ExecutePipeline(ctx, pipe); err != nil {
		return nil, NewQueueOperationError("stats", q.queueName, "", err, true)
	}

	mainCount := mainQueueCount.Val()
	flightCount := inFlightCount.Val()

	// Get oldest message age
	var oldestAge *time.Duration
	if mainCount > 0 {
		// Get oldest message (lowest score)
		messageIDs, err := q.client.ZRangeByScoreWithLimit(ctx, q.messagesKey, "-inf", "+inf", 0, 1)
		if err == nil && len(messageIDs) > 0 {
			messageKey := FormatMessageKey(q.queueName, messageIDs[0])
			queuedAtStr, err := q.client.HGet(ctx, messageKey, "queued_at")
			if err == nil {
				if queuedAt, err := strconv.ParseInt(queuedAtStr, 10, 64); err == nil {
					age := time.Since(time.Unix(queuedAt, 0))
					oldestAge = &age
				}
			}
		}
	}

	stats := &QueueStats{
		Name:              q.queueName,
		ApproximateMessages: mainCount,
		MessagesInFlight:   flightCount,
		MessagesDelayed:    0, // Redis doesn't have delayed messages in this implementation
		OldestMessageAge:   oldestAge,
	}

	return stats, nil
}

// IsHealthy checks if the queue is healthy
func (q *RedisTaskQueue) IsHealthy(ctx context.Context) error {
	if q.closed {
		return ErrQueueClosed
	}

	return q.client.IsHealthy(ctx)
}

// Close closes the queue connection
func (q *RedisTaskQueue) Close() error {
	if q.closed {
		return nil
	}

	q.closed = true
	q.logger.Info("task queue closed", "queue_name", q.queueName)
	return nil
}

// CleanupExpiredMessages removes expired in-flight messages and returns them to main queue
func (q *RedisTaskQueue) CleanupExpiredMessages(ctx context.Context) error {
	if q.closed {
		return ErrQueueClosed
	}

	currentTime := time.Now().Unix()

	// Get expired in-flight messages
	expiredMessages, err := q.client.ZRangeByScoreWithLimit(ctx, q.inFlightKey, "-inf", fmt.Sprintf("%d", currentTime), 0, -1)
	if err != nil {
		return NewQueueOperationError("cleanup", q.queueName, "", err, true)
	}

	if len(expiredMessages) == 0 {
		return nil // No expired messages
	}

	q.logger.Info("cleaning up expired in-flight messages",
		"count", len(expiredMessages),
		"queue", q.queueName,
	)

	// Process expired messages in batches
	batchSize := 100
	for i := 0; i < len(expiredMessages); i += batchSize {
		end := i + batchSize
		if end > len(expiredMessages) {
			end = len(expiredMessages)
		}

		batch := expiredMessages[i:end]
		if err := q.processExpiredBatch(ctx, batch, currentTime); err != nil {
			q.logger.Error("failed to process expired message batch",
				"error", err,
				"batch_size", len(batch),
			)
			// Continue with next batch
		}
	}

	return nil
}

// processExpiredBatch processes a batch of expired messages
func (q *RedisTaskQueue) processExpiredBatch(ctx context.Context, messageIDs []string, currentTime int64) error {
	script := `
		local currentTime = tonumber(ARGV[1])
		local restored = 0
		
		for i = 2, #ARGV do
			local messageId = ARGV[i]
			local messageKey = KEYS[3] .. ':' .. messageId
			
			-- Check if message still exists
			if redis.call('EXISTS', messageKey) == 1 then
				-- Get message data
				local messageData = redis.call('HMGET', messageKey, 'data', 'priority', 'queued_at')
				if messageData[1] and messageData[2] and messageData[3] then
					-- Calculate priority score
					local priority = tonumber(messageData[2])
					local queuedAt = tonumber(messageData[3])
					local priorityScore = (10 - priority) * 1000000 + (queuedAt / 1000000)
					
					-- Remove from in-flight
					redis.call('ZREM', KEYS[2], messageId)
					
					-- Add back to main queue
					redis.call('ZADD', KEYS[1], priorityScore, messageId)
					
					-- Clear receipt handle
					redis.call('HDEL', messageKey, 'receipt_handle', 'dequeued_at')
					
					restored = restored + 1
				else
					-- Message data incomplete, remove from in-flight
					redis.call('ZREM', KEYS[2], messageId)
					redis.call('DEL', messageKey)
				end
			else
				-- Message doesn't exist, remove from in-flight
				redis.call('ZREM', KEYS[2], messageId)
			end
		end
		
		-- Update stats
		if restored > 0 then
			redis.call('HINCRBY', KEYS[4], 'total_restored', restored)
			redis.call('HSET', KEYS[4], 'last_cleanup', currentTime)
		end
		
		return restored
	`

	keys := []string{
		q.messagesKey,                           // KEYS[1]: main queue
		q.inFlightKey,                           // KEYS[2]: in-flight queue
		FormatQueueKey(q.queueName, "messages"), // KEYS[3]: message data prefix
		q.statsKey,                              // KEYS[4]: stats key
	}

	args := make([]interface{}, len(messageIDs)+1)
	args[0] = currentTime
	for i, messageID := range messageIDs {
		args[i+1] = messageID
	}

	result, err := q.client.ExecuteLuaScript(ctx, script, keys, args...)
	if err != nil {
		return err
	}

	restoredCount, ok := result.(int64)
	if ok && restoredCount > 0 {
		q.logger.Debug("expired messages restored to queue",
			"restored_count", restoredCount,
			"batch_size", len(messageIDs),
		)
	}

	return nil
}
package logging

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/voidrunnerhq/voidrunner/internal/queue"
)

// RedisStreamingService implements StreamingService using Redis pub/sub
type RedisStreamingService struct {
	redisClient *queue.RedisClient
	config      *LogConfig
	logger      *slog.Logger

	// Subscription management
	subscriptions map[uuid.UUID]map[string]*StreamSubscription // taskID -> subscriptionID -> subscription
	mu            sync.RWMutex

	// Background goroutines
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Metrics
	totalMessages     int64
	lastMessageTime   time.Time
	errorCount        int64
}

// NewRedisStreamingService creates a new Redis-based streaming service
func NewRedisStreamingService(redisClient *queue.RedisClient, config *LogConfig, logger *slog.Logger) (*RedisStreamingService, error) {
	if redisClient == nil {
		return nil, fmt.Errorf("redis client is required")
	}
	
	if config == nil {
		config = DefaultLogConfig()
	}
	
	if logger == nil {
		logger = slog.Default()
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid log config: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	service := &RedisStreamingService{
		redisClient:   redisClient,
		config:        config,
		logger:        logger.With("component", "streaming_service"),
		subscriptions: make(map[uuid.UUID]map[string]*StreamSubscription),
		ctx:           ctx,
		cancel:        cancel,
	}

	// Start background cleanup goroutine
	service.wg.Add(1)
	go service.cleanupRoutine()

	return service, nil
}

// Subscribe creates a new log subscription for a task
func (s *RedisStreamingService) Subscribe(ctx context.Context, taskID uuid.UUID, userID uuid.UUID) (<-chan LogEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if we're at the subscription limit
	totalSubs := s.getTotalSubscriptionsLocked()
	if totalSubs >= s.config.MaxConcurrentStreams {
		return nil, fmt.Errorf("maximum concurrent streams reached (%d)", s.config.MaxConcurrentStreams)
	}

	// Create subscription
	subscriptionID := uuid.New().String()
	channel := make(chan LogEntry, s.config.BufferSize)
	
	subscription := &StreamSubscription{
		ID:        subscriptionID,
		TaskID:    taskID,
		UserID:    userID,
		CreatedAt: time.Now(),
		Channel:   channel,
	}

	// Add to subscriptions map
	if s.subscriptions[taskID] == nil {
		s.subscriptions[taskID] = make(map[string]*StreamSubscription)
	}
	s.subscriptions[taskID][subscriptionID] = subscription

	s.logger.Info("created log subscription",
		"subscription_id", subscriptionID,
		"task_id", taskID,
		"user_id", userID,
		"total_subscriptions", totalSubs+1)

	// Start monitoring goroutine for this subscription
	s.wg.Add(1)
	go s.monitorSubscription(ctx, subscription)

	return channel, nil
}

// Unsubscribe removes a log subscription
func (s *RedisStreamingService) Unsubscribe(taskID uuid.UUID, ch <-chan LogEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	taskSubs, exists := s.subscriptions[taskID]
	if !exists {
		return fmt.Errorf("no subscriptions found for task %s", taskID)
	}

	// Find the subscription by channel
	var subscriptionID string
	for id, sub := range taskSubs {
		if sub.Channel == ch {
			subscriptionID = id
			break
		}
	}

	if subscriptionID == "" {
		return fmt.Errorf("subscription not found for task %s", taskID)
	}

	// Remove subscription
	delete(taskSubs, subscriptionID)
	
	// Clean up empty task map
	if len(taskSubs) == 0 {
		delete(s.subscriptions, taskID)
	}

	// Close the channel
	close(taskSubs[subscriptionID].Channel)

	s.logger.Info("removed log subscription",
		"subscription_id", subscriptionID,
		"task_id", taskID,
		"remaining_subscriptions", s.getTotalSubscriptionsLocked())

	return nil
}

// PublishLog sends a log entry to all subscribers of the task
func (s *RedisStreamingService) PublishLog(ctx context.Context, entry LogEntry) error {
	// Validate the log entry
	if err := entry.Validate(); err != nil {
		s.errorCount++
		return fmt.Errorf("invalid log entry: %w", err)
	}

	// Truncate content if it exceeds max size
	if len(entry.Content) > s.config.MaxLogLineSize {
		entry.Content = entry.Content[:s.config.MaxLogLineSize-3] + "..."
		s.logger.Warn("truncated log line due to size limit",
			"task_id", entry.TaskID,
			"original_size", len(entry.Content)+3,
			"max_size", s.config.MaxLogLineSize)
	}

	// Serialize the log entry for Redis
	data, err := json.Marshal(entry)
	if err != nil {
		s.errorCount++
		return fmt.Errorf("failed to marshal log entry: %w", err)
	}

	// Publish to Redis channel
	channelName := s.getChannelName(entry.TaskID)
	if err := s.redisClient.Publish(ctx, channelName, string(data)); err != nil {
		s.errorCount++
		return fmt.Errorf("failed to publish log entry to Redis: %w", err)
	}

	// Also distribute to local subscribers (for same-process subscriptions)
	s.distributeToLocalSubscribers(entry)

	// Update metrics
	s.totalMessages++
	s.lastMessageTime = time.Now()

	return nil
}

// GetActiveSubscriptions returns the count of active subscriptions for a task
func (s *RedisStreamingService) GetActiveSubscriptions(taskID uuid.UUID) int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	taskSubs, exists := s.subscriptions[taskID]
	if !exists {
		return 0
	}

	return len(taskSubs)
}

// GetTotalSubscriptions returns the total number of active subscriptions
func (s *RedisStreamingService) GetTotalSubscriptions() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.getTotalSubscriptionsLocked()
}

// Close shuts down the streaming service and cleans up resources
func (s *RedisStreamingService) Close() error {
	s.logger.Info("shutting down streaming service")

	// Cancel context to stop background goroutines
	s.cancel()

	// Close all subscription channels
	s.mu.Lock()
	for taskID, taskSubs := range s.subscriptions {
		for subID, sub := range taskSubs {
			close(sub.Channel)
			s.logger.Debug("closed subscription channel",
				"subscription_id", subID,
				"task_id", taskID)
		}
	}
	s.subscriptions = make(map[uuid.UUID]map[string]*StreamSubscription)
	s.mu.Unlock()

	// Wait for all goroutines to finish
	s.wg.Wait()

	s.logger.Info("streaming service shutdown complete")
	return nil
}

// distributeToLocalSubscribers sends log entry to local subscribers
func (s *RedisStreamingService) distributeToLocalSubscribers(entry LogEntry) {
	s.mu.RLock()
	taskSubs, exists := s.subscriptions[entry.TaskID]
	if !exists {
		s.mu.RUnlock()
		return
	}

	// Create a copy of the subscriptions to avoid holding the lock
	subs := make([]*StreamSubscription, 0, len(taskSubs))
	for _, sub := range taskSubs {
		subs = append(subs, sub)
	}
	s.mu.RUnlock()

	// Send to each subscriber without blocking
	for _, sub := range subs {
		select {
		case sub.Channel <- entry:
			// Successfully sent
		default:
			// Channel is full, log warning but don't block
			s.logger.Warn("subscription channel full, dropping log entry",
				"subscription_id", sub.ID,
				"task_id", sub.TaskID,
				"user_id", sub.UserID)
		}
	}
}

// monitorSubscription monitors a subscription for timeouts and cleanup
func (s *RedisStreamingService) monitorSubscription(ctx context.Context, sub *StreamSubscription) {
	defer s.wg.Done()

	ticker := time.NewTicker(s.config.SubscriberKeepalive)
	defer ticker.Stop()

	timeout := time.NewTimer(s.config.StreamTimeout)
	defer timeout.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.ctx.Done():
			return
		case <-timeout.C:
			// Subscription timed out
			s.logger.Info("subscription timed out",
				"subscription_id", sub.ID,
				"task_id", sub.TaskID,
				"user_id", sub.UserID,
				"duration", time.Since(sub.CreatedAt))
			
			// Remove the subscription
			s.mu.Lock()
			if taskSubs, exists := s.subscriptions[sub.TaskID]; exists {
				delete(taskSubs, sub.ID)
				if len(taskSubs) == 0 {
					delete(s.subscriptions, sub.TaskID)
				}
			}
			s.mu.Unlock()
			
			close(sub.Channel)
			return
		case <-ticker.C:
			// Periodic health check
			// Check if channel is still being read from
			select {
			case sub.Channel <- LogEntry{}: // Try to send a keepalive
				// Channel is healthy, continue
			default:
				// Channel might be blocked, but that's okay for now
			}
		}
	}
}

// cleanupRoutine runs periodic cleanup tasks
func (s *RedisStreamingService) cleanupRoutine() {
	defer s.wg.Done()

	ticker := time.NewTicker(time.Minute) // Run cleanup every minute
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.performCleanup()
		}
	}
}

// performCleanup removes stale subscriptions and updates metrics
func (s *RedisStreamingService) performCleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()

	staleCount := 0
	now := time.Now()

	for taskID, taskSubs := range s.subscriptions {
		for subID, sub := range taskSubs {
			// Check if subscription is stale (no activity for a long time)
			age := now.Sub(sub.CreatedAt)
			if age > s.config.StreamTimeout {
				close(sub.Channel)
				delete(taskSubs, subID)
				staleCount++
				
				s.logger.Debug("removed stale subscription",
					"subscription_id", subID,
					"task_id", taskID,
					"age", age)
			}
		}
		
		// Clean up empty task maps
		if len(taskSubs) == 0 {
			delete(s.subscriptions, taskID)
		}
	}

	if staleCount > 0 {
		s.logger.Info("cleaned up stale subscriptions",
			"removed_count", staleCount,
			"remaining_subscriptions", s.getTotalSubscriptionsLocked())
	}
}

// getTotalSubscriptionsLocked returns total subscriptions (must hold lock)
func (s *RedisStreamingService) getTotalSubscriptionsLocked() int {
	total := 0
	for _, taskSubs := range s.subscriptions {
		total += len(taskSubs)
	}
	return total
}

// getChannelName returns the Redis channel name for a task
func (s *RedisStreamingService) getChannelName(taskID uuid.UUID) string {
	return s.config.RedisChannelPrefix + taskID.String()
}

// GetStats returns statistics about the streaming service
func (s *RedisStreamingService) GetStats() *LogCollectorStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := &LogCollectorStats{
		ActiveStreams:       s.getTotalSubscriptionsLocked(),
		TotalLinesCollected: s.totalMessages,
		StreamsByContainer:  make(map[string]StreamInfo),
	}

	// Calculate collection rate
	if !s.lastMessageTime.IsZero() {
		duration := time.Since(s.lastMessageTime)
		if duration > 0 {
			stats.CollectionRate = float64(s.totalMessages) / duration.Seconds()
		}
	}

	return stats
}
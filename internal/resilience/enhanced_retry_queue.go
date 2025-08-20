package resilience

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
)

// EnhancedRetryQueue extends the basic retry queue with advanced retry strategies
type EnhancedRetryQueue struct {
	mu       sync.RWMutex
	config   *EnhancedRetryConfig
	strategy *RetryStrategy
	executor *RetryExecutor
	logger   *slog.Logger

	// Queue state
	pendingRetries   map[string]*EnhancedRetryMessage
	scheduledRetries map[string]*ScheduledRetry

	// Background processing
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Statistics
	stats EnhancedRetryStats
}

// EnhancedRetryConfig defines configuration for enhanced retry queue
type EnhancedRetryConfig struct {
	// Basic configuration
	QueueName            string        `json:"queue_name"`
	ProcessingInterval   time.Duration `json:"processing_interval"`
	MaxConcurrentRetries int           `json:"max_concurrent_retries"`

	// Retry strategy configuration
	RetryStrategy *RetryStrategyConfig `json:"retry_strategy"`

	// Persistence configuration
	PersistRetries bool          `json:"persist_retries"`
	RetryTTL       time.Duration `json:"retry_ttl"`

	// Monitoring configuration
	EnableMetrics       bool          `json:"enable_metrics"`
	StatsReportInterval time.Duration `json:"stats_report_interval"`
}

// DefaultEnhancedRetryConfig returns sensible defaults
func DefaultEnhancedRetryConfig() *EnhancedRetryConfig {
	return &EnhancedRetryConfig{
		QueueName:            "enhanced_retry_queue",
		ProcessingInterval:   10 * time.Second,
		MaxConcurrentRetries: 100,
		RetryStrategy:        DefaultRetryStrategyConfig(),
		PersistRetries:       true,
		RetryTTL:             24 * time.Hour,
		EnableMetrics:        true,
		StatsReportInterval:  1 * time.Minute,
	}
}

// Validate checks if the configuration is valid
func (config *EnhancedRetryConfig) Validate() error {
	if config.ProcessingInterval <= 0 {
		return fmt.Errorf("processing interval must be positive")
	}
	if config.MaxConcurrentRetries <= 0 {
		return fmt.Errorf("max concurrent retries must be positive")
	}
	if config.RetryTTL <= 0 {
		return fmt.Errorf("retry TTL must be positive")
	}
	if config.RetryStrategy != nil {
		if err := config.RetryStrategy.Validate(); err != nil {
			return fmt.Errorf("invalid retry strategy: %w", err)
		}
	}
	return nil
}

// EnhancedRetryMessage represents a message in the enhanced retry queue
type EnhancedRetryMessage struct {
	// Basic message information
	ID         string    `json:"id"`
	OriginalID string    `json:"original_id"`
	TaskID     uuid.UUID `json:"task_id"`
	UserID     uuid.UUID `json:"user_id"`

	// Retry information
	Attempts      int        `json:"attempts"`
	MaxAttempts   int        `json:"max_attempts"`
	NextRetryAt   time.Time  `json:"next_retry_at"`
	CreatedAt     time.Time  `json:"created_at"`
	LastAttemptAt *time.Time `json:"last_attempt_at,omitempty"`

	// Error information
	LastError     string   `json:"last_error,omitempty"`
	ErrorHistory  []string `json:"error_history"`
	FailureReason string   `json:"failure_reason,omitempty"`

	// Strategy information
	StrategyType    RetryStrategyType `json:"strategy_type"`
	CalculatedDelay time.Duration     `json:"calculated_delay"`
	ActualDelay     time.Duration     `json:"actual_delay"`

	// Metadata
	Priority int                    `json:"priority"`
	Context  map[string]interface{} `json:"context,omitempty"`
	Labels   map[string]string      `json:"labels,omitempty"`

	// Operation information
	OperationType string          `json:"operation_type"`
	OperationData json.RawMessage `json:"operation_data,omitempty"`
}

// ScheduledRetry represents a retry scheduled for execution
type ScheduledRetry struct {
	Message          *EnhancedRetryMessage `json:"message"`
	ScheduledAt      time.Time             `json:"scheduled_at"`
	ExecutionContext *ExecutionContext     `json:"execution_context,omitempty"`
}

// EnhancedRetryStats holds statistics for the enhanced retry queue
type EnhancedRetryStats struct {
	// Queue statistics
	TotalMessages     int64 `json:"total_messages"`
	PendingRetries    int64 `json:"pending_retries"`
	ScheduledRetries  int64 `json:"scheduled_retries"`
	ConcurrentRetries int64 `json:"concurrent_retries"`

	// Success/failure statistics
	SuccessfulRetries int64 `json:"successful_retries"`
	FailedRetries     int64 `json:"failed_retries"`
	PermanentFailures int64 `json:"permanent_failures"`

	// Timing statistics
	AverageRetryDelay time.Duration `json:"average_retry_delay"`
	MinRetryDelay     time.Duration `json:"min_retry_delay"`
	MaxRetryDelay     time.Duration `json:"max_retry_delay"`

	// Strategy statistics
	StrategyStats map[RetryStrategyType]int64 `json:"strategy_stats"`

	// Error statistics
	ErrorDistribution map[string]int64 `json:"error_distribution"`

	// Performance statistics
	ProcessingRate  float64    `json:"processing_rate"` // messages per second
	LastProcessedAt *time.Time `json:"last_processed_at,omitempty"`

	// Budget statistics
	BudgetUtilization float64 `json:"budget_utilization"`
}

// NewEnhancedRetryQueue creates a new enhanced retry queue
func NewEnhancedRetryQueue(config *EnhancedRetryConfig, logger *slog.Logger) (*EnhancedRetryQueue, error) {
	if config == nil {
		config = DefaultEnhancedRetryConfig()
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid enhanced retry config: %w", err)
	}

	if logger == nil {
		logger = slog.Default()
	}

	// Create retry strategy
	strategy, err := NewRetryStrategy(config.RetryStrategy, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create retry strategy: %w", err)
	}

	// Create retry executor
	executor, err := NewRetryExecutor(config.RetryStrategy, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create retry executor: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	queue := &EnhancedRetryQueue{
		config:           config,
		strategy:         strategy,
		executor:         executor,
		logger:           logger.With("component", "enhanced_retry_queue"),
		pendingRetries:   make(map[string]*EnhancedRetryMessage),
		scheduledRetries: make(map[string]*ScheduledRetry),
		ctx:              ctx,
		cancel:           cancel,
		stats: EnhancedRetryStats{
			StrategyStats:     make(map[RetryStrategyType]int64),
			ErrorDistribution: make(map[string]int64),
		},
	}

	return queue, nil
}

// Start starts the enhanced retry queue background processing
func (eq *EnhancedRetryQueue) Start(ctx context.Context) error {
	eq.logger.Info("starting enhanced retry queue",
		"queue_name", eq.config.QueueName,
		"processing_interval", eq.config.ProcessingInterval)

	// Start processing loop
	eq.wg.Add(1)
	go eq.processingLoop()

	// Start metrics reporting if enabled
	if eq.config.EnableMetrics {
		eq.wg.Add(1)
		go eq.metricsReportingLoop()
	}

	eq.logger.Info("enhanced retry queue started successfully")
	return nil
}

// Stop stops the enhanced retry queue
func (eq *EnhancedRetryQueue) Stop(ctx context.Context) error {
	eq.logger.Info("stopping enhanced retry queue")

	// Cancel context
	eq.cancel()

	// Wait for background goroutines to complete
	done := make(chan struct{})
	go func() {
		eq.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		eq.logger.Info("enhanced retry queue stopped successfully")
	case <-time.After(30 * time.Second):
		eq.logger.Warn("timeout waiting for enhanced retry queue to stop")
	}

	// Stop retry executor
	return eq.executor.Stop(ctx)
}

// EnqueueForRetry adds a message to the enhanced retry queue
func (eq *EnhancedRetryQueue) EnqueueForRetry(ctx context.Context, message *EnhancedRetryMessage) error {
	if message == nil {
		return fmt.Errorf("message cannot be nil")
	}

	// Validate message
	if message.ID == "" {
		message.ID = uuid.New().String()
	}
	if message.CreatedAt.IsZero() {
		message.CreatedAt = time.Now()
	}

	// Calculate next retry time using strategy
	delay := eq.strategy.CalculateDelay(message.Attempts + 1)
	message.NextRetryAt = time.Now().Add(delay)
	message.CalculatedDelay = delay
	message.StrategyType = eq.config.RetryStrategy.Strategy

	eq.mu.Lock()
	defer eq.mu.Unlock()

	// Add to pending retries
	eq.pendingRetries[message.ID] = message
	eq.stats.TotalMessages++
	eq.stats.PendingRetries++
	eq.stats.StrategyStats[message.StrategyType]++

	// Update error distribution
	if message.LastError != "" {
		eq.stats.ErrorDistribution[message.LastError]++
	}

	eq.logger.Info("message enqueued for retry",
		"message_id", message.ID,
		"task_id", message.TaskID,
		"attempts", message.Attempts,
		"next_retry_at", message.NextRetryAt,
		"delay", delay,
		"strategy", message.StrategyType)

	return nil
}

// processingLoop is the main processing loop
func (eq *EnhancedRetryQueue) processingLoop() {
	defer eq.wg.Done()

	ticker := time.NewTicker(eq.config.ProcessingInterval)
	defer ticker.Stop()

	eq.logger.Debug("enhanced retry queue processing loop started")

	for {
		select {
		case <-eq.ctx.Done():
			eq.logger.Debug("enhanced retry queue processing loop stopped")
			return
		case <-ticker.C:
			eq.processReadyRetries()
		}
	}
}

// processReadyRetries processes retries that are ready for execution
func (eq *EnhancedRetryQueue) processReadyRetries() {
	now := time.Now()
	var readyMessages []*EnhancedRetryMessage

	eq.mu.Lock()
	// Find messages ready for retry
	for id, message := range eq.pendingRetries {
		if message.NextRetryAt.Before(now) || message.NextRetryAt.Equal(now) {
			readyMessages = append(readyMessages, message)
			delete(eq.pendingRetries, id)
			eq.stats.PendingRetries--
		}
	}
	eq.mu.Unlock()

	if len(readyMessages) == 0 {
		return
	}

	eq.logger.Debug("processing ready retries", "count", len(readyMessages))

	// Process messages concurrently up to the limit
	semaphore := make(chan struct{}, eq.config.MaxConcurrentRetries)

	for _, message := range readyMessages {
		select {
		case semaphore <- struct{}{}:
			go eq.processRetryMessage(message, semaphore)
		case <-eq.ctx.Done():
			return
		}
	}
}

// processRetryMessage processes a single retry message
func (eq *EnhancedRetryQueue) processRetryMessage(message *EnhancedRetryMessage, semaphore chan struct{}) {
	defer func() { <-semaphore }()

	startTime := time.Now()

	eq.mu.Lock()
	eq.stats.ConcurrentRetries++
	eq.stats.ScheduledRetries++

	// Create scheduled retry entry
	scheduled := &ScheduledRetry{
		Message:     message,
		ScheduledAt: startTime,
	}
	eq.scheduledRetries[message.ID] = scheduled
	eq.mu.Unlock()

	// Clean up on completion
	defer func() {
		eq.mu.Lock()
		delete(eq.scheduledRetries, message.ID)
		eq.stats.ConcurrentRetries--
		eq.stats.ScheduledRetries--
		eq.mu.Unlock()
	}()

	eq.logger.Info("processing retry message",
		"message_id", message.ID,
		"task_id", message.TaskID,
		"attempt", message.Attempts+1)

	// Execute the retry operation
	operationID := fmt.Sprintf("%s_retry_%d", message.ID, message.Attempts+1)

	err := eq.executor.Execute(eq.ctx, operationID, func(ctx context.Context, attempt int) error {
		// This is where the actual retry operation would be performed
		// For now, we'll simulate the operation based on the message type
		return eq.executeRetryOperation(ctx, message, attempt)
	})

	// Update message and statistics
	message.Attempts++
	message.LastAttemptAt = &startTime
	actualDelay := startTime.Sub(message.CreatedAt)
	message.ActualDelay = actualDelay

	if err != nil {
		message.LastError = err.Error()
		message.ErrorHistory = append(message.ErrorHistory, err.Error())

		eq.mu.Lock()
		eq.stats.FailedRetries++
		eq.stats.ErrorDistribution[err.Error()]++
		eq.mu.Unlock()

		// Check if we should retry again
		if eq.strategy.ShouldRetry(eq.ctx, message.Attempts, err) &&
			message.Attempts < message.MaxAttempts {
			// Schedule for another retry
			_ = eq.EnqueueForRetry(eq.ctx, message)
		} else {
			// Permanent failure
			eq.mu.Lock()
			eq.stats.PermanentFailures++
			eq.mu.Unlock()

			eq.logger.Error("message permanently failed",
				"message_id", message.ID,
				"task_id", message.TaskID,
				"total_attempts", message.Attempts,
				"error", err)
		}
	} else {
		// Success
		eq.mu.Lock()
		eq.stats.SuccessfulRetries++
		eq.stats.LastProcessedAt = &startTime
		eq.mu.Unlock()

		eq.logger.Info("retry message processed successfully",
			"message_id", message.ID,
			"task_id", message.TaskID,
			"total_attempts", message.Attempts,
			"total_duration", time.Since(message.CreatedAt))
	}

	// Update timing statistics
	eq.updateTimingStats(actualDelay)
}

// executeRetryOperation simulates executing the actual retry operation
func (eq *EnhancedRetryQueue) executeRetryOperation(ctx context.Context, message *EnhancedRetryMessage, attempt int) error {
	// This is a placeholder implementation
	// In a real system, this would delegate to the appropriate service
	// based on the operation type in the message

	eq.logger.Debug("executing retry operation",
		"message_id", message.ID,
		"operation_type", message.OperationType,
		"attempt", attempt)

	// Simulate some work
	select {
	case <-time.After(100 * time.Millisecond):
		// For demonstration, randomly succeed or fail
		if time.Now().UnixNano()%3 == 0 {
			return fmt.Errorf("simulated operation failure")
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// updateTimingStats updates timing statistics
func (eq *EnhancedRetryQueue) updateTimingStats(delay time.Duration) {
	eq.mu.Lock()
	defer eq.mu.Unlock()

	if eq.stats.MinRetryDelay == 0 || delay < eq.stats.MinRetryDelay {
		eq.stats.MinRetryDelay = delay
	}
	if delay > eq.stats.MaxRetryDelay {
		eq.stats.MaxRetryDelay = delay
	}

	// Calculate moving average
	if eq.stats.SuccessfulRetries > 0 {
		totalDelay := eq.stats.AverageRetryDelay * time.Duration(eq.stats.SuccessfulRetries)
		eq.stats.AverageRetryDelay = (totalDelay + delay) / time.Duration(eq.stats.SuccessfulRetries+1)
	} else {
		eq.stats.AverageRetryDelay = delay
	}
}

// metricsReportingLoop periodically reports metrics
func (eq *EnhancedRetryQueue) metricsReportingLoop() {
	defer eq.wg.Done()

	ticker := time.NewTicker(eq.config.StatsReportInterval)
	defer ticker.Stop()

	for {
		select {
		case <-eq.ctx.Done():
			return
		case <-ticker.C:
			eq.reportMetrics()
		}
	}
}

// reportMetrics reports current metrics
func (eq *EnhancedRetryQueue) reportMetrics() {
	stats := eq.GetStats()

	eq.logger.Info("enhanced retry queue metrics",
		"total_messages", stats.TotalMessages,
		"pending_retries", stats.PendingRetries,
		"concurrent_retries", stats.ConcurrentRetries,
		"successful_retries", stats.SuccessfulRetries,
		"failed_retries", stats.FailedRetries,
		"permanent_failures", stats.PermanentFailures,
		"average_delay", stats.AverageRetryDelay,
		"processing_rate", stats.ProcessingRate,
		"budget_utilization", stats.BudgetUtilization)
}

// GetStats returns current statistics
func (eq *EnhancedRetryQueue) GetStats() EnhancedRetryStats {
	eq.mu.RLock()
	defer eq.mu.RUnlock()

	stats := eq.stats

	// Calculate processing rate
	if stats.LastProcessedAt != nil {
		duration := time.Since(*stats.LastProcessedAt)
		if duration > 0 {
			stats.ProcessingRate = float64(stats.SuccessfulRetries) / duration.Seconds()
		}
	}

	// Calculate budget utilization
	strategyStats := eq.strategy.GetStats()
	if strategyStats.BudgetRemaining+strategyStats.BudgetUsed > 0 {
		stats.BudgetUtilization = float64(strategyStats.BudgetUsed) /
			float64(strategyStats.BudgetRemaining+strategyStats.BudgetUsed) * 100
	}

	return stats
}

// GetMessage returns a specific message by ID
func (eq *EnhancedRetryQueue) GetMessage(messageID string) (*EnhancedRetryMessage, bool) {
	eq.mu.RLock()
	defer eq.mu.RUnlock()

	if message, exists := eq.pendingRetries[messageID]; exists {
		return message, true
	}

	if scheduled, exists := eq.scheduledRetries[messageID]; exists {
		return scheduled.Message, true
	}

	return nil, false
}

// RemoveMessage removes a message from the queue
func (eq *EnhancedRetryQueue) RemoveMessage(messageID string) bool {
	eq.mu.Lock()
	defer eq.mu.Unlock()

	if _, exists := eq.pendingRetries[messageID]; exists {
		delete(eq.pendingRetries, messageID)
		eq.stats.PendingRetries--
		return true
	}

	return false
}

// GetPendingMessages returns all pending messages
func (eq *EnhancedRetryQueue) GetPendingMessages() []*EnhancedRetryMessage {
	eq.mu.RLock()
	defer eq.mu.RUnlock()

	messages := make([]*EnhancedRetryMessage, 0, len(eq.pendingRetries))
	for _, message := range eq.pendingRetries {
		messages = append(messages, message)
	}

	return messages
}

// GetScheduledRetries returns all currently scheduled retries
func (eq *EnhancedRetryQueue) GetScheduledRetries() []*ScheduledRetry {
	eq.mu.RLock()
	defer eq.mu.RUnlock()

	retries := make([]*ScheduledRetry, 0, len(eq.scheduledRetries))
	for _, retry := range eq.scheduledRetries {
		retries = append(retries, retry)
	}

	return retries
}

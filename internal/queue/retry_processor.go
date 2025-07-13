package queue

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/voidrunnerhq/voidrunner/internal/config"
)

// RetryProcessor handles background processing of retry queue
type RetryProcessor struct {
	retryQueue RetryQueue
	taskQueue  TaskQueue
	config     *config.QueueConfig
	logger     *slog.Logger

	// State management
	mu       sync.RWMutex
	running  bool
	stopChan chan struct{}
	doneChan chan struct{}
}

// NewRetryProcessor creates a new retry processor
func NewRetryProcessor(retryQueue RetryQueue, taskQueue TaskQueue, config *config.QueueConfig, logger *slog.Logger) *RetryProcessor {
	if logger == nil {
		logger = slog.Default()
	}

	return &RetryProcessor{
		retryQueue: retryQueue,
		taskQueue:  taskQueue,
		config:     config,
		logger:     logger,
		running:    false,
		stopChan:   make(chan struct{}),
		doneChan:   make(chan struct{}),
	}
}

// Start begins background processing of retry queue
func (rp *RetryProcessor) Start(ctx context.Context) error {
	rp.mu.Lock()
	defer rp.mu.Unlock()

	if rp.running {
		return nil // Already running
	}

	rp.logger.Info("starting retry processor")

	rp.running = true
	rp.stopChan = make(chan struct{})
	rp.doneChan = make(chan struct{})

	// Start background goroutine
	go rp.processRetries(ctx)

	rp.logger.Info("retry processor started successfully")
	return nil
}

// Stop gracefully stops the retry processor
func (rp *RetryProcessor) Stop() error {
	rp.mu.Lock()
	defer rp.mu.Unlock()

	if !rp.running {
		return nil // Already stopped
	}

	rp.logger.Info("stopping retry processor")

	// Signal stop
	close(rp.stopChan)

	// Wait for processing to complete with timeout
	select {
	case <-rp.doneChan:
		rp.logger.Info("retry processor stopped successfully")
	case <-time.After(30 * time.Second):
		rp.logger.Warn("retry processor stop timeout")
	}

	rp.running = false
	return nil
}

// IsRunning returns true if the retry processor is running
func (rp *RetryProcessor) IsRunning() bool {
	rp.mu.RLock()
	defer rp.mu.RUnlock()
	return rp.running
}

// processRetries is the main processing loop
func (rp *RetryProcessor) processRetries(ctx context.Context) {
	defer close(rp.doneChan)

	// Process retries every 30 seconds
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	rp.logger.Debug("retry processor loop started")

	for {
		select {
		case <-ctx.Done():
			rp.logger.Debug("retry processor stopped due to context cancellation")
			return
		case <-rp.stopChan:
			rp.logger.Debug("retry processor stopped due to stop signal")
			return
		case <-ticker.C:
			rp.processReadyRetries(ctx)
		}
	}
}

// processReadyRetries processes messages ready for retry
func (rp *RetryProcessor) processReadyRetries(ctx context.Context) {
	// Create context with timeout for this processing cycle
	processCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	rp.logger.Debug("processing ready retries")

	// Get messages ready for retry
	messages, err := rp.retryQueue.DequeueReadyForRetry(processCtx, rp.config.BatchSize)
	if err != nil {
		rp.logger.Error("failed to dequeue retry messages", "error", err)
		return
	}

	if len(messages) == 0 {
		rp.logger.Debug("no messages ready for retry")
		return
	}

	rp.logger.Info("processing retry messages", "count", len(messages))

	successCount := 0
	errorCount := 0

	// Process each message
	for _, message := range messages {
		if err := rp.processRetryMessage(processCtx, message); err != nil {
			rp.logger.Error("failed to process retry message",
				"message_id", message.MessageID,
				"task_id", message.TaskID,
				"error", err,
			)
			errorCount++
		} else {
			successCount++
		}

		// Check if we should stop
		select {
		case <-rp.stopChan:
			rp.logger.Info("retry processing stopped during batch",
				"processed", successCount+errorCount,
				"total", len(messages),
			)
			return
		case <-processCtx.Done():
			rp.logger.Warn("retry processing timeout during batch",
				"processed", successCount+errorCount,
				"total", len(messages),
			)
			return
		default:
			// Continue processing
		}
	}

	rp.logger.Info("retry processing batch completed",
		"total", len(messages),
		"success", successCount,
		"errors", errorCount,
	)
}

// processRetryMessage processes a single retry message
func (rp *RetryProcessor) processRetryMessage(ctx context.Context, message *TaskMessage) error {
	// Reset message for retry
	retryMessage := &TaskMessage{
		TaskID:      message.TaskID,
		UserID:      message.UserID,
		Priority:    message.Priority,
		QueuedAt:    time.Now(), // New queue time
		Attempts:    message.Attempts,
		LastAttempt: message.LastAttempt,
		MessageID:   GenerateMessageID(), // New message ID
		Attributes:  copyAttributes(message.Attributes),
	}

	// Enqueue to main task queue
	if err := rp.taskQueue.Enqueue(ctx, retryMessage); err != nil {
		return fmt.Errorf("failed to enqueue retry message to task queue: %w", err)
	}

	rp.logger.Debug("retry message requeued successfully",
		"original_message_id", message.MessageID,
		"new_message_id", retryMessage.MessageID,
		"task_id", message.TaskID,
		"attempts", message.Attempts,
	)

	return nil
}

// GetStats returns retry processor statistics
func (rp *RetryProcessor) GetStats() RetryProcessorStats {
	rp.mu.RLock()
	defer rp.mu.RUnlock()

	return RetryProcessorStats{
		Running:   rp.running,
		LastCheck: time.Now(), // This would need to be tracked properly
	}
}

// RetryProcessorStats represents retry processor statistics
type RetryProcessorStats struct {
	Running   bool      `json:"running"`
	LastCheck time.Time `json:"last_check"`
}

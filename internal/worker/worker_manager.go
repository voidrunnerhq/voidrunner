package worker

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/voidrunnerhq/voidrunner/internal/database"
	"github.com/voidrunnerhq/voidrunner/internal/executor"
	"github.com/voidrunnerhq/voidrunner/internal/models"
	"github.com/voidrunnerhq/voidrunner/internal/queue"
)

// BaseWorkerManager implements WorkerManager interface
type BaseWorkerManager struct {
	// Core components
	queueManager      queue.QueueManager
	executor          executor.TaskExecutor
	repos             *database.Repositories
	concurrency       ConcurrencyManager
	processorRegistry *ProcessorRegistry
	config            WorkerConfig
	logger            *slog.Logger

	// Worker pool management
	workerPool WorkerPool
	poolMu     sync.RWMutex

	// State management
	mu         sync.RWMutex
	isRunning  bool
	isHealthy  bool
	ctx        context.Context
	cancel     context.CancelFunc
	shutdownCh chan struct{}

	// Statistics and monitoring
	stats     WorkerManagerStats
	statsMu   sync.RWMutex
	startedAt time.Time

	// Background processing
	retryProcessor *RetryProcessor
	cleanupTicker  *time.Ticker
	healthTicker   *time.Ticker
}

// NewWorkerManager creates a new worker manager
func NewWorkerManager(
	queueManager queue.QueueManager,
	executor executor.TaskExecutor,
	repos *database.Repositories,
	config WorkerConfig,
	logger *slog.Logger,
) WorkerManager {
	// Create concurrency manager
	concurrencyLimits := ConcurrencyLimits{
		MaxConcurrentTasks:     config.GetMaxConcurrentTasks(),
		MaxUserConcurrentTasks: config.GetMaxUserConcurrentTasks(),
		MaxWorkers:             config.GetMaxWorkers(),
		MinWorkers:             config.GetMinWorkers(),
	}

	concurrency := NewRedisConcurrencyManager(
		concurrencyLimits,
		config.ProcessingSlotTTL,
		5*time.Minute, // cleanup interval
		logger,
	)

	// Create processor registry
	processorRegistry := NewProcessorRegistry(logger)

	// Create worker pool
	workerPool := NewWorkerPool(
		queueManager.TaskQueue(),
		executor,
		repos,
		concurrency,
		config,
		logger,
	)

	return &BaseWorkerManager{
		queueManager:      queueManager,
		executor:          executor,
		repos:             repos,
		concurrency:       concurrency,
		processorRegistry: processorRegistry,
		workerPool:        workerPool,
		config:            config,
		logger:            logger.With("component", "worker_manager"),
		shutdownCh:        make(chan struct{}),
		isHealthy:         true,
		stats: WorkerManagerStats{
			IsRunning: false,
			IsHealthy: true,
		},
	}
}

// Start starts the worker manager and all pools
func (wm *BaseWorkerManager) Start(ctx context.Context) error {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	if wm.isRunning {
		return fmt.Errorf("worker manager is already running")
	}

	wm.logger.Info("starting worker manager")

	// Create manager context
	wm.ctx, wm.cancel = context.WithCancel(ctx)
	wm.isRunning = true
	wm.startedAt = time.Now()

	// Start queue manager
	if err := wm.queueManager.Start(wm.ctx); err != nil {
		wm.cancel()
		wm.isRunning = false
		return fmt.Errorf("failed to start queue manager: %w", err)
	}

	// Register default processors
	if err := wm.registerDefaultProcessors(); err != nil {
		wm.cancel()
		wm.isRunning = false
		return fmt.Errorf("failed to register processors: %w", err)
	}

	// Start worker pool
	if err := wm.workerPool.Start(wm.ctx); err != nil {
		wm.cancel()
		wm.isRunning = false
		return fmt.Errorf("failed to start worker pool: %w", err)
	}

	// Start retry processor
	if err := wm.startRetryProcessor(); err != nil {
		wm.logger.Error("failed to start retry processor", "error", err)
		// Don't fail startup for retry processor
	}

	// Start monitoring
	wm.startMonitoring()

	// Update initial statistics
	wm.updateStats()

	wm.logger.Info("worker manager started successfully")
	return nil
}

// Stop gracefully stops the worker manager and all pools
func (wm *BaseWorkerManager) Stop(ctx context.Context) error {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	if !wm.isRunning {
		return fmt.Errorf("worker manager is not running")
	}

	wm.logger.Info("stopping worker manager")

	// Stop monitoring
	wm.stopMonitoring()

	// Signal shutdown
	wm.cancel()
	wm.isRunning = false

	// Stop components in order
	var wg sync.WaitGroup

	// Stop worker pool
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := wm.workerPool.Stop(ctx); err != nil {
			wm.logger.Error("failed to stop worker pool", "error", err)
		}
	}()

	// Stop retry processor
	if wm.retryProcessor != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := wm.retryProcessor.Stop(); err != nil {
				wm.logger.Error("failed to stop retry processor", "error", err)
			}
		}()
	}

	// Stop queue manager
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := wm.queueManager.Stop(ctx); err != nil {
			wm.logger.Error("failed to stop queue manager", "error", err)
		}
	}()

	// Wait for all components to stop or timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		wm.logger.Info("worker manager stopped gracefully")
	case <-time.After(wm.config.ShutdownTimeout):
		wm.logger.Warn("worker manager shutdown timeout reached")
	case <-ctx.Done():
		wm.logger.Warn("worker manager shutdown cancelled by context")
	}

	// Final statistics update
	wm.updateStats()

	// Send shutdown signal
	select {
	case wm.shutdownCh <- struct{}{}:
	default:
	}

	return nil
}

// IsRunning returns true if the manager is running
func (wm *BaseWorkerManager) IsRunning() bool {
	wm.mu.RLock()
	defer wm.mu.RUnlock()
	return wm.isRunning
}

// GetWorkerPool returns the worker pool
func (wm *BaseWorkerManager) GetWorkerPool() WorkerPool {
	wm.poolMu.RLock()
	defer wm.poolMu.RUnlock()
	return wm.workerPool
}

// GetStats returns comprehensive manager statistics
func (wm *BaseWorkerManager) GetStats() WorkerManagerStats {
	wm.statsMu.RLock()
	defer wm.statsMu.RUnlock()

	// Create a copy and update derived fields
	stats := wm.stats
	stats.LastUpdated = time.Now()

	return stats
}

// IsHealthy checks if the worker manager is healthy
func (wm *BaseWorkerManager) IsHealthy() bool {
	wm.mu.RLock()
	defer wm.mu.RUnlock()

	if !wm.isRunning {
		return false
	}

	// Check queue manager health
	if err := wm.queueManager.IsHealthy(wm.ctx); err != nil {
		return false
	}

	// Check worker pool health
	if !wm.workerPool.IsHealthy() {
		return false
	}

	// Check executor health
	if err := wm.executor.IsHealthy(wm.ctx); err != nil {
		return false
	}

	return wm.isHealthy
}

// HandleTaskExecution processes a task execution request
func (wm *BaseWorkerManager) HandleTaskExecution(ctx context.Context, message *queue.TaskMessage) error {
	wm.logger.Debug("handling task execution", "task_id", message.TaskID, "user_id", message.UserID)

	// Find appropriate processor
	processor, err := wm.processorRegistry.GetProcessor(message)
	if err != nil {
		return fmt.Errorf("no suitable processor found: %w", err)
	}

	// Process the task
	if err := processor.ProcessTask(ctx, message); err != nil {
		wm.logger.Error("task processing failed",
			"task_id", message.TaskID,
			"processor_type", processor.GetProcessorType(),
			"error", err)
		return err
	}

	wm.logger.Debug("task execution completed", "task_id", message.TaskID)
	return nil
}

// HandleTaskCancellation handles task cancellation
func (wm *BaseWorkerManager) HandleTaskCancellation(ctx context.Context, executionID uuid.UUID) error {
	wm.logger.Info("handling task cancellation", "execution_id", executionID)

	// Get execution from database
	execution, err := wm.repos.TaskExecutions.GetByID(ctx, executionID)
	if err != nil {
		return fmt.Errorf("failed to get execution: %w", err)
	}

	// Check if execution is cancellable
	if models.IsExecutionStatusTerminal(execution.Status) {
		return fmt.Errorf("execution %s is not active (status: %s)", executionID, execution.Status)
	}

	// Cancel the execution via executor
	if err := wm.executor.Cancel(ctx, executionID); err != nil {
		wm.logger.Error("failed to cancel execution", "execution_id", executionID, "error", err)
		// Continue to update status even if cancellation fails
	}

	// Update execution status to cancelled
	execution.Status = models.ExecutionStatusCancelled
	now := time.Now()
	execution.CompletedAt = &now

	if err := wm.repos.TaskExecutions.Update(ctx, execution); err != nil {
		return fmt.Errorf("failed to update execution status: %w", err)
	}

	// Update task status to cancelled
	if err := wm.repos.Tasks.UpdateStatus(ctx, execution.TaskID, models.TaskStatusCancelled); err != nil {
		wm.logger.Error("failed to update task status to cancelled", "task_id", execution.TaskID, "error", err)
	}

	wm.logger.Info("task execution cancelled", "execution_id", executionID, "task_id", execution.TaskID)
	return nil
}

// GetConcurrencyLimits returns current concurrency limits
func (wm *BaseWorkerManager) GetConcurrencyLimits() ConcurrencyLimits {
	return wm.concurrency.GetLimits()
}

// UpdateConcurrencyLimits updates concurrency limits
func (wm *BaseWorkerManager) UpdateConcurrencyLimits(limits ConcurrencyLimits) error {
	if err := wm.concurrency.UpdateLimits(limits); err != nil {
		return fmt.Errorf("failed to update concurrency limits: %w", err)
	}

	wm.logger.Info("concurrency limits updated",
		"max_concurrent_tasks", limits.MaxConcurrentTasks,
		"max_user_concurrent_tasks", limits.MaxUserConcurrentTasks)

	return nil
}

// registerDefaultProcessors registers the default task processors
func (wm *BaseWorkerManager) registerDefaultProcessors() error {
	// Default resource limits for all processors
	resourceLimits := executor.ResourceLimits{
		MemoryLimitBytes: 512 * 1024 * 1024, // 512MB
		CPUQuota:         100000,            // 1 CPU core
		PidsLimit:        128,               // Max processes
		TimeoutSeconds:   int(wm.config.TaskTimeout.Seconds()),
	}

	// Register general processor (handles all task types)
	generalProcessor := NewTaskProcessor(
		ProcessorTypeGeneral,
		wm.executor,
		wm.repos,
		wm.config.TaskTimeout,
		resourceLimits,
		wm.logger,
	)
	wm.processorRegistry.RegisterProcessor(ProcessorTypeGeneral, generalProcessor)

	// Register specialized processors (optional, for future optimization)
	processors := []struct {
		Type ProcessorType
		Name string
	}{
		{ProcessorTypePython, "Python"},
		{ProcessorTypeBash, "Bash"},
		{ProcessorTypeGo, "Go"},
		{ProcessorTypeJS, "JavaScript"},
	}

	for _, proc := range processors {
		processor := NewTaskProcessor(
			proc.Type,
			wm.executor,
			wm.repos,
			wm.config.TaskTimeout,
			resourceLimits,
			wm.logger,
		)
		wm.processorRegistry.RegisterProcessor(proc.Type, processor)
		wm.logger.Debug("registered processor", "type", proc.Type, "name", proc.Name)
	}

	return nil
}

// startRetryProcessor starts the background retry processor
func (wm *BaseWorkerManager) startRetryProcessor() error {
	retryConfig := RetryProcessorConfig{
		CheckInterval:    30 * time.Second,
		BatchSize:        10,
		MaxRetryAttempts: wm.config.MaxRetryAttempts,
		Logger:           wm.logger,
	}

	retryProcessor := NewRetryProcessor(
		wm.queueManager.RetryQueue(),
		wm.queueManager.TaskQueue(),
		wm.queueManager.DeadLetterQueue(),
		retryConfig,
	)

	if err := retryProcessor.Start(wm.ctx); err != nil {
		return fmt.Errorf("failed to start retry processor: %w", err)
	}

	wm.retryProcessor = retryProcessor
	return nil
}

// startMonitoring starts background monitoring routines
func (wm *BaseWorkerManager) startMonitoring() {
	// Health monitoring
	wm.healthTicker = time.NewTicker(wm.config.HealthCheckInterval)
	go wm.healthMonitoringLoop()

	// Cleanup monitoring
	wm.cleanupTicker = time.NewTicker(10 * time.Minute) // Cleanup every 10 minutes
	go wm.cleanupLoop()

	// Statistics update
	go wm.statsUpdateLoop()
}

// stopMonitoring stops monitoring routines
func (wm *BaseWorkerManager) stopMonitoring() {
	if wm.healthTicker != nil {
		wm.healthTicker.Stop()
	}
	if wm.cleanupTicker != nil {
		wm.cleanupTicker.Stop()
	}
}

// healthMonitoringLoop monitors overall system health
func (wm *BaseWorkerManager) healthMonitoringLoop() {
	for {
		select {
		case <-wm.ctx.Done():
			return
		case <-wm.healthTicker.C:
			wm.performHealthCheck()
		}
	}
}

// performHealthCheck checks the health of all components
func (wm *BaseWorkerManager) performHealthCheck() {
	healthy := true

	// Check queue manager
	if err := wm.queueManager.IsHealthy(wm.ctx); err != nil {
		wm.logger.Warn("queue manager unhealthy", "error", err)
		healthy = false
	}

	// Check worker pool
	if !wm.workerPool.IsHealthy() {
		wm.logger.Warn("worker pool unhealthy")
		healthy = false
	}

	// Check executor
	if err := wm.executor.IsHealthy(wm.ctx); err != nil {
		wm.logger.Warn("executor unhealthy", "error", err)
		healthy = false
	}

	// Check processors
	if !wm.processorRegistry.IsHealthy() {
		wm.logger.Warn("some processors unhealthy")
		healthy = false
	}

	// Update health status
	wm.mu.Lock()
	wm.isHealthy = healthy
	wm.mu.Unlock()

	wm.updateStats()
}

// cleanupLoop performs periodic cleanup tasks
func (wm *BaseWorkerManager) cleanupLoop() {
	for {
		select {
		case <-wm.ctx.Done():
			return
		case <-wm.cleanupTicker.C:
			wm.performCleanup()
		}
	}
}

// performCleanup performs periodic cleanup tasks
func (wm *BaseWorkerManager) performCleanup() {
	wm.logger.Debug("performing periodic cleanup")

	// Cleanup executor resources
	if err := wm.executor.Cleanup(wm.ctx); err != nil {
		wm.logger.Warn("executor cleanup failed", "error", err)
	}

	// Additional cleanup tasks can be added here
}

// statsUpdateLoop periodically updates statistics
func (wm *BaseWorkerManager) statsUpdateLoop() {
	ticker := time.NewTicker(30 * time.Second) // Update every 30 seconds
	defer ticker.Stop()

	for {
		select {
		case <-wm.ctx.Done():
			return
		case <-ticker.C:
			wm.updateStats()
		}
	}
}

// updateStats updates comprehensive statistics
func (wm *BaseWorkerManager) updateStats() {
	wm.statsMu.Lock()
	defer wm.statsMu.Unlock()

	wm.stats.IsRunning = wm.isRunning
	wm.stats.IsHealthy = wm.isHealthy
	wm.stats.StartedAt = wm.startedAt
	wm.stats.LastUpdated = time.Now()

	// Get worker pool stats
	wm.stats.WorkerPoolStats = wm.workerPool.GetStats()

	// Get concurrency stats
	wm.stats.ConcurrencyStats = wm.concurrency.GetStats()

	// Get processing slots (simplified for now)
	wm.stats.ProcessingSlots = make([]ProcessingSlot, 0)
}

// Additional configuration methods for WorkerConfig
func (c WorkerConfig) GetMaxConcurrentTasks() int {
	// This should be properly configured, defaulting for now
	if c.TaskTimeout > 0 {
		return 50 // Default to 50 concurrent tasks
	}
	return 50
}

func (c WorkerConfig) GetMaxUserConcurrentTasks() int {
	// This should be properly configured, defaulting for now
	return 5 // Default to 5 concurrent tasks per user
}

// RetryProcessorConfig represents configuration for the retry processor
type RetryProcessorConfig struct {
	CheckInterval    time.Duration
	BatchSize        int
	MaxRetryAttempts int
	Logger           *slog.Logger
}

// RetryProcessor handles retry logic for failed tasks
type RetryProcessor struct {
	retryQueue      queue.RetryQueue
	taskQueue       queue.TaskQueue
	deadLetterQueue queue.DeadLetterQueue
	config          RetryProcessorConfig

	ctx       context.Context
	cancel    context.CancelFunc
	isRunning bool
	mu        sync.Mutex
}

// NewRetryProcessor creates a new retry processor
func NewRetryProcessor(
	retryQueue queue.RetryQueue,
	taskQueue queue.TaskQueue,
	deadLetterQueue queue.DeadLetterQueue,
	config RetryProcessorConfig,
) *RetryProcessor {
	return &RetryProcessor{
		retryQueue:      retryQueue,
		taskQueue:       taskQueue,
		deadLetterQueue: deadLetterQueue,
		config:          config,
	}
}

// Start starts the retry processor
func (rp *RetryProcessor) Start(ctx context.Context) error {
	rp.mu.Lock()
	defer rp.mu.Unlock()

	if rp.isRunning {
		return fmt.Errorf("retry processor already running")
	}

	rp.ctx, rp.cancel = context.WithCancel(ctx)
	rp.isRunning = true

	go rp.processLoop()

	rp.config.Logger.Info("retry processor started")
	return nil
}

// Stop stops the retry processor
func (rp *RetryProcessor) Stop() error {
	rp.mu.Lock()
	defer rp.mu.Unlock()

	if !rp.isRunning {
		return fmt.Errorf("retry processor not running")
	}

	rp.cancel()
	rp.isRunning = false

	rp.config.Logger.Info("retry processor stopped")
	return nil
}

// processLoop is the main retry processing loop
func (rp *RetryProcessor) processLoop() {
	ticker := time.NewTicker(rp.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-rp.ctx.Done():
			return
		case <-ticker.C:
			if err := rp.processRetries(); err != nil {
				rp.config.Logger.Error("retry processing failed", "error", err)
			}
		}
	}
}

// processRetries processes ready retry messages
func (rp *RetryProcessor) processRetries() error {
	messages, err := rp.retryQueue.DequeueReadyForRetry(rp.ctx, rp.config.BatchSize)
	if err != nil {
		return fmt.Errorf("failed to dequeue retry messages: %w", err)
	}

	if len(messages) == 0 {
		return nil
	}

	rp.config.Logger.Debug("processing retry messages", "count", len(messages))

	for _, message := range messages {
		if err := rp.processRetryMessage(message); err != nil {
			rp.config.Logger.Error("failed to process retry message",
				"task_id", message.TaskID,
				"error", err)
		}
	}

	return nil
}

// processRetryMessage processes a single retry message
func (rp *RetryProcessor) processRetryMessage(message *queue.TaskMessage) error {
	// Check if we've exceeded max retry attempts
	if message.Attempts >= rp.config.MaxRetryAttempts {
		// Move to dead letter queue
		if err := rp.deadLetterQueue.EnqueueFailedTask(rp.ctx, message); err != nil {
			return fmt.Errorf("failed to enqueue to dead letter queue: %w", err)
		}

		rp.config.Logger.Info("task moved to dead letter queue after max retries",
			"task_id", message.TaskID,
			"attempts", message.Attempts)
		return nil
	}

	// Re-enqueue to main task queue
	message.Attempts++
	now := time.Now()
	message.LastAttempt = &now

	if err := rp.taskQueue.Enqueue(rp.ctx, message); err != nil {
		return fmt.Errorf("failed to re-enqueue task: %w", err)
	}

	rp.config.Logger.Debug("task re-enqueued for retry",
		"task_id", message.TaskID,
		"attempt", message.Attempts)

	return nil
}

package resilience

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// RetryExecutor provides enhanced retry functionality with strategies and budgets
type RetryExecutor struct {
	strategy       *RetryStrategy
	circuitBreaker *CircuitBreaker
	logger         *slog.Logger

	// Execution state
	mu        sync.RWMutex
	executing map[string]*ExecutionContext // Track ongoing executions
}

// ExecutionContext tracks the context of a retry execution
type ExecutionContext struct {
	ID             string             `json:"id"`
	StartTime      time.Time          `json:"start_time"`
	CurrentAttempt int                `json:"current_attempt"`
	LastError      error              `json:"last_error,omitempty"`
	Attempts       []RetryAttempt     `json:"attempts"`
	TotalDelay     time.Duration      `json:"total_delay"`
	Context        context.Context    `json:"-"`
	CancelFunc     context.CancelFunc `json:"-"`
}

// OperationFunc represents an operation that can be retried
type OperationFunc func(ctx context.Context, attempt int) error

// NewRetryExecutor creates a new retry executor
func NewRetryExecutor(config *RetryStrategyConfig, logger *slog.Logger) (*RetryExecutor, error) {
	strategy, err := NewRetryStrategy(config, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create retry strategy: %w", err)
	}

	if logger == nil {
		logger = slog.Default()
	}

	executor := &RetryExecutor{
		strategy:  strategy,
		logger:    logger.With("component", "retry_executor"),
		executing: make(map[string]*ExecutionContext),
	}

	// Initialize circuit breaker if configured
	if config.UseCircuitBreaker && config.CircuitBreakerName != "" {
		cbConfig := DefaultCircuitBreakerConfig()
		executor.circuitBreaker, err = NewCircuitBreaker(config.CircuitBreakerName, cbConfig, logger)
		if err != nil {
			return nil, fmt.Errorf("failed to create circuit breaker: %w", err)
		}
	}

	return executor, nil
}

// Execute executes an operation with retry logic
func (re *RetryExecutor) Execute(ctx context.Context, operationID string, operation OperationFunc) error {
	// Create execution context
	execCtx, cancel := context.WithCancel(ctx)
	execution := &ExecutionContext{
		ID:             operationID,
		StartTime:      time.Now(),
		CurrentAttempt: 1,
		Attempts:       make([]RetryAttempt, 0),
		Context:        execCtx,
		CancelFunc:     cancel,
	}

	// Track execution
	re.mu.Lock()
	re.executing[operationID] = execution
	re.mu.Unlock()

	// Cleanup on completion
	defer func() {
		re.mu.Lock()
		delete(re.executing, operationID)
		re.mu.Unlock()
		cancel()
	}()

	re.logger.Info("starting retry execution",
		"operation_id", operationID,
		"max_attempts", re.strategy.config.MaxAttempts)

	var lastError error

	for attempt := 1; attempt <= re.strategy.config.MaxAttempts; attempt++ {
		execution.CurrentAttempt = attempt

		// Check if context is cancelled
		select {
		case <-ctx.Done():
			re.logger.Info("retry execution cancelled",
				"operation_id", operationID,
				"attempt", attempt)
			return ctx.Err()
		default:
		}

		// Execute with circuit breaker if configured
		attemptStart := time.Now()
		var err error

		if re.circuitBreaker != nil {
			err = re.circuitBreaker.Execute(execCtx, func(cbCtx context.Context) error {
				return operation(cbCtx, attempt)
			})
		} else {
			err = operation(execCtx, attempt)
		}

		attemptEnd := time.Now()
		duration := attemptEnd.Sub(attemptStart)

		// Record attempt
		retryAttempt := RetryAttempt{
			Attempt:   attempt,
			StartTime: attemptStart,
			EndTime:   attemptEnd,
			Duration:  duration,
			Success:   err == nil,
			Error:     err,
		}

		execution.Attempts = append(execution.Attempts, retryAttempt)
		execution.LastError = err

		// Success case
		if err == nil {
			re.strategy.RecordAttempt(retryAttempt)
			re.logger.Info("retry execution succeeded",
				"operation_id", operationID,
				"attempt", attempt,
				"duration", duration,
				"total_duration", time.Since(execution.StartTime))
			return nil
		}

		lastError = err

		re.logger.Warn("retry execution attempt failed",
			"operation_id", operationID,
			"attempt", attempt,
			"error", err,
			"duration", duration)

		// Check if we should retry
		if !re.strategy.ShouldRetry(ctx, attempt, err) {
			re.logger.Info("retry execution stopped - no more retries",
				"operation_id", operationID,
				"attempt", attempt,
				"reason", "should_not_retry")
			break
		}

		// Last attempt - don't calculate delay
		if attempt >= re.strategy.config.MaxAttempts {
			break
		}

		// Calculate delay for next attempt
		delay := re.strategy.CalculateDelay(attempt + 1)
		retryAttempt.Delay = delay
		execution.TotalDelay += delay

		re.strategy.RecordAttempt(retryAttempt)

		re.logger.Info("retry execution scheduling next attempt",
			"operation_id", operationID,
			"next_attempt", attempt+1,
			"delay", delay)

		// Wait for delay or context cancellation
		select {
		case <-ctx.Done():
			re.logger.Info("retry execution cancelled during delay",
				"operation_id", operationID,
				"attempt", attempt)
			return ctx.Err()
		case <-time.After(delay):
			// Continue to next attempt
		}
	}

	// Record final failed attempt
	finalAttempt := RetryAttempt{
		Attempt: execution.CurrentAttempt,
		Success: false,
		Error:   lastError,
	}
	re.strategy.RecordAttempt(finalAttempt)

	re.logger.Error("retry execution failed after all attempts",
		"operation_id", operationID,
		"total_attempts", execution.CurrentAttempt,
		"total_duration", time.Since(execution.StartTime),
		"total_delay", execution.TotalDelay,
		"final_error", lastError)

	return fmt.Errorf("operation failed after %d attempts: %w", execution.CurrentAttempt, lastError)
}

// ExecuteWithTimeout executes an operation with retry logic and an overall timeout
func (re *RetryExecutor) ExecuteWithTimeout(ctx context.Context, operationID string, timeout time.Duration, operation OperationFunc) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	return re.Execute(timeoutCtx, operationID, operation)
}

// GetExecutionStatus returns the status of an ongoing execution
func (re *RetryExecutor) GetExecutionStatus(operationID string) (*ExecutionContext, bool) {
	re.mu.RLock()
	defer re.mu.RUnlock()

	execution, exists := re.executing[operationID]
	if !exists {
		return nil, false
	}

	// Return a copy to avoid race conditions
	copy := &ExecutionContext{
		ID:             execution.ID,
		StartTime:      execution.StartTime,
		CurrentAttempt: execution.CurrentAttempt,
		LastError:      execution.LastError,
		TotalDelay:     execution.TotalDelay,
		Attempts:       make([]RetryAttempt, len(execution.Attempts)),
	}

	// Deep copy attempts
	for i, attempt := range execution.Attempts {
		copy.Attempts[i] = attempt
	}

	return copy, true
}

// CancelExecution cancels an ongoing execution
func (re *RetryExecutor) CancelExecution(operationID string) bool {
	re.mu.RLock()
	execution, exists := re.executing[operationID]
	re.mu.RUnlock()

	if !exists {
		return false
	}

	execution.CancelFunc()
	re.logger.Info("retry execution cancelled", "operation_id", operationID)
	return true
}

// GetActiveExecutions returns all currently active executions
func (re *RetryExecutor) GetActiveExecutions() map[string]*ExecutionContext {
	re.mu.RLock()
	defer re.mu.RUnlock()

	result := make(map[string]*ExecutionContext)
	for id, execution := range re.executing {
		result[id] = &ExecutionContext{
			ID:             execution.ID,
			StartTime:      execution.StartTime,
			CurrentAttempt: execution.CurrentAttempt,
			LastError:      execution.LastError,
			TotalDelay:     execution.TotalDelay,
		}
	}

	return result
}

// GetStats returns retry executor statistics
func (re *RetryExecutor) GetStats() RetryExecutorStats {
	strategyStats := re.strategy.GetStats()

	re.mu.RLock()
	activeExecutions := len(re.executing)
	re.mu.RUnlock()

	stats := RetryExecutorStats{
		RetryStats:       strategyStats,
		ActiveExecutions: activeExecutions,
	}

	// Add circuit breaker stats if available
	if re.circuitBreaker != nil {
		cbStats := re.circuitBreaker.GetStats()
		stats.CircuitBreakerStats = &cbStats
	}

	return stats
}

// RetryExecutorStats holds statistics for the retry executor
type RetryExecutorStats struct {
	RetryStats          RetryStats           `json:"retry_stats"`
	ActiveExecutions    int                  `json:"active_executions"`
	CircuitBreakerStats *CircuitBreakerStats `json:"circuit_breaker_stats,omitempty"`
}

// Reset resets all statistics and state
func (re *RetryExecutor) Reset() {
	// Cancel all active executions
	re.mu.Lock()
	for id, execution := range re.executing {
		execution.CancelFunc()
		re.logger.Info("cancelled execution during reset", "operation_id", id)
	}
	re.executing = make(map[string]*ExecutionContext)
	re.mu.Unlock()

	// Reset strategy statistics
	re.strategy.Reset()

	// Reset circuit breaker if present
	if re.circuitBreaker != nil {
		re.circuitBreaker.Reset()
	}

	re.logger.Info("retry executor reset completed")
}

// Stop gracefully stops the retry executor
func (re *RetryExecutor) Stop(ctx context.Context) error {
	re.logger.Info("stopping retry executor")

	// Cancel all active executions
	re.mu.Lock()
	activeCount := len(re.executing)
	for id, execution := range re.executing {
		execution.CancelFunc()
		re.logger.Debug("cancelled execution during stop", "operation_id", id)
	}
	re.mu.Unlock()

	if activeCount > 0 {
		re.logger.Info("cancelled active executions during stop", "count", activeCount)

		// Wait a bit for executions to complete gracefully
		select {
		case <-time.After(5 * time.Second):
			re.logger.Warn("timeout waiting for executions to complete")
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	re.logger.Info("retry executor stopped successfully")
	return nil
}

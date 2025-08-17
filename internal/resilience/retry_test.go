package resilience

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRetryStrategyConfig(t *testing.T) {
	t.Run("DefaultRetryStrategyConfig", func(t *testing.T) {
		config := DefaultRetryStrategyConfig()
		
		assert.Equal(t, StrategyExponentialBackoff, config.Strategy)
		assert.Equal(t, JitterEqual, config.Jitter)
		assert.Equal(t, 5, config.MaxAttempts)
		assert.Equal(t, 1*time.Second, config.BaseDelay)
		assert.NotNil(t, config.Budget)
		
		err := config.Validate()
		assert.NoError(t, err)
	})

	t.Run("Validation", func(t *testing.T) {
		tests := []struct {
			name      string
			config    *RetryStrategyConfig
			expectErr bool
		}{
			{
				name:      "Valid config",
				config:    DefaultRetryStrategyConfig(),
				expectErr: false,
			},
			{
				name: "Invalid max attempts",
				config: &RetryStrategyConfig{
					MaxAttempts: 0,
					BaseDelay:   1 * time.Second,
					MaxDelay:    60 * time.Second,
					BackoffFactor: 2.0,
					JitterRange: 0.1,
				},
				expectErr: true,
			},
			{
				name: "Invalid jitter range",
				config: &RetryStrategyConfig{
					MaxAttempts: 5,
					BaseDelay:   1 * time.Second,
					MaxDelay:    60 * time.Second,
					BackoffFactor: 2.0,
					JitterRange: 1.5, // Invalid
				},
				expectErr: true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := tt.config.Validate()
				if tt.expectErr {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})
}

func TestRetryBudget(t *testing.T) {
	logger := slog.Default()
	
	t.Run("NewRetryBudget", func(t *testing.T) {
		config := &RetryBudgetConfig{
			MaxRetries:       100,
			TimeWindow:       1 * time.Hour,
			BudgetPercentage: 10.0,
		}
		
		budget := NewRetryBudget(config, logger)
		assert.NotNil(t, budget)
		assert.Equal(t, config, budget.config)
	})

	t.Run("CanRetry and ConsumeRetry", func(t *testing.T) {
		config := &RetryBudgetConfig{
			MaxRetries:       10,
			TimeWindow:       1 * time.Hour,
			BudgetPercentage: 100.0, // Allow all retries
		}
		
		budget := NewRetryBudget(config, logger)
		ctx := context.Background()
		
		// Should be able to consume up to max retries
		for i := 0; i < 10; i++ {
			assert.True(t, budget.CanRetry(ctx))
			assert.True(t, budget.ConsumeRetry(ctx))
		}
		
		// Should not be able to consume more
		assert.False(t, budget.CanRetry(ctx))
		assert.False(t, budget.ConsumeRetry(ctx))
	})

	t.Run("RecordOperation", func(t *testing.T) {
		config := &RetryBudgetConfig{
			MaxRetries:       100,
			TimeWindow:       1 * time.Hour,
			BudgetPercentage: 10.0,
		}
		
		budget := NewRetryBudget(config, logger)
		
		// Record some operations
		budget.RecordOperation(true)
		budget.RecordOperation(true)
		budget.RecordOperation(false)
		
		stats := budget.GetStats()
		assert.Equal(t, 2.0/3.0, stats.SuccessRate)
	})
}

func TestRetryStrategy(t *testing.T) {
	logger := slog.Default()
	
	t.Run("NewRetryStrategy", func(t *testing.T) {
		config := DefaultRetryStrategyConfig()
		strategy, err := NewRetryStrategy(config, logger)
		
		require.NoError(t, err)
		assert.NotNil(t, strategy)
		assert.Equal(t, config, strategy.config)
	})

	t.Run("CalculateDelay - ExponentialBackoff", func(t *testing.T) {
		config := &RetryStrategyConfig{
			Strategy:      StrategyExponentialBackoff,
			Jitter:        JitterNone,
			MaxAttempts:   5,
			BaseDelay:     1 * time.Second,
			MaxDelay:      60 * time.Second,
			BackoffFactor: 2.0,
			JitterRange:   0.0,
		}
		
		strategy, err := NewRetryStrategy(config, logger)
		require.NoError(t, err)
		
		// Test exponential backoff
		assert.Equal(t, time.Duration(0), strategy.CalculateDelay(0))
		assert.Equal(t, 1*time.Second, strategy.CalculateDelay(1))
		assert.Equal(t, 2*time.Second, strategy.CalculateDelay(2))
		assert.Equal(t, 4*time.Second, strategy.CalculateDelay(3))
		assert.Equal(t, 8*time.Second, strategy.CalculateDelay(4))
	})

	t.Run("CalculateDelay - LinearBackoff", func(t *testing.T) {
		config := &RetryStrategyConfig{
			Strategy:      StrategyLinearBackoff,
			Jitter:        JitterNone,
			MaxAttempts:   5,
			BaseDelay:     1 * time.Second,
			MaxDelay:      60 * time.Second,
			BackoffFactor: 2.0,
			JitterRange:   0.0,
		}
		
		strategy, err := NewRetryStrategy(config, logger)
		require.NoError(t, err)
		
		// Test linear backoff
		assert.Equal(t, time.Duration(0), strategy.CalculateDelay(0))
		assert.Equal(t, 1*time.Second, strategy.CalculateDelay(1))
		assert.Equal(t, 2*time.Second, strategy.CalculateDelay(2))
		assert.Equal(t, 3*time.Second, strategy.CalculateDelay(3))
	})

	t.Run("CalculateDelay - FixedDelay", func(t *testing.T) {
		config := &RetryStrategyConfig{
			Strategy:      StrategyFixedDelay,
			Jitter:        JitterNone,
			MaxAttempts:   5,
			BaseDelay:     5 * time.Second,
			MaxDelay:      60 * time.Second,
			BackoffFactor: 2.0,
			JitterRange:   0.0,
		}
		
		strategy, err := NewRetryStrategy(config, logger)
		require.NoError(t, err)
		
		// Test fixed delay
		assert.Equal(t, time.Duration(0), strategy.CalculateDelay(0))
		assert.Equal(t, 5*time.Second, strategy.CalculateDelay(1))
		assert.Equal(t, 5*time.Second, strategy.CalculateDelay(2))
		assert.Equal(t, 5*time.Second, strategy.CalculateDelay(3))
	})

	t.Run("CalculateDelay - FibonacciBackoff", func(t *testing.T) {
		config := &RetryStrategyConfig{
			Strategy:      StrategyFibonacciBackoff,
			Jitter:        JitterNone,
			MaxAttempts:   6,
			BaseDelay:     1 * time.Second,
			MaxDelay:      60 * time.Second,
			BackoffFactor: 2.0,
			JitterRange:   0.0,
		}
		
		strategy, err := NewRetryStrategy(config, logger)
		require.NoError(t, err)
		
		// Test Fibonacci backoff (1, 1, 2, 3, 5, 8...)
		assert.Equal(t, time.Duration(0), strategy.CalculateDelay(0))
		assert.Equal(t, 1*time.Second, strategy.CalculateDelay(1)) // fib(1) = 1
		assert.Equal(t, 1*time.Second, strategy.CalculateDelay(2)) // fib(2) = 1
		assert.Equal(t, 2*time.Second, strategy.CalculateDelay(3)) // fib(3) = 2
		assert.Equal(t, 3*time.Second, strategy.CalculateDelay(4)) // fib(4) = 3
		assert.Equal(t, 5*time.Second, strategy.CalculateDelay(5)) // fib(5) = 5
	})

	t.Run("MaxDelay Limit", func(t *testing.T) {
		config := &RetryStrategyConfig{
			Strategy:      StrategyExponentialBackoff,
			Jitter:        JitterNone,
			MaxAttempts:   10,
			BaseDelay:     1 * time.Second,
			MaxDelay:      5 * time.Second, // Low max delay
			BackoffFactor: 2.0,
			JitterRange:   0.0,
		}
		
		strategy, err := NewRetryStrategy(config, logger)
		require.NoError(t, err)
		
		// Even with exponential backoff, should not exceed max delay
		delay := strategy.CalculateDelay(10) // Would be 512 seconds without limit
		assert.Equal(t, 5*time.Second, delay)
	})

	t.Run("ShouldRetry", func(t *testing.T) {
		config := &RetryStrategyConfig{
			Strategy:    StrategyExponentialBackoff,
			MaxAttempts: 3,
			BaseDelay:   1 * time.Second,
			MaxDelay:    60 * time.Second,
			BackoffFactor: 2.0,
			RetryConditions: []RetryCondition{
				{
					RetryableErrorTypes: []string{"timeout", "network"},
				},
			},
		}
		
		strategy, err := NewRetryStrategy(config, logger)
		require.NoError(t, err)
		
		ctx := context.Background()
		
		// Should retry for retryable errors within attempt limit
		assert.True(t, strategy.ShouldRetry(ctx, 1, errors.New("timeout error")))
		assert.True(t, strategy.ShouldRetry(ctx, 2, errors.New("network failure")))
		
		// Should not retry beyond max attempts
		assert.False(t, strategy.ShouldRetry(ctx, 3, errors.New("timeout error")))
		
		// Should not retry for non-retryable errors
		assert.False(t, strategy.ShouldRetry(ctx, 1, errors.New("validation error")))
		
		// Should not retry for nil error
		assert.False(t, strategy.ShouldRetry(ctx, 1, nil))
	})

	t.Run("RecordAttempt and GetStats", func(t *testing.T) {
		config := DefaultRetryStrategyConfig()
		strategy, err := NewRetryStrategy(config, logger)
		require.NoError(t, err)
		
		// Record some attempts
		strategy.RecordAttempt(RetryAttempt{
			Attempt: 1,
			Success: true,
			Delay:   1 * time.Second,
			Duration: 100 * time.Millisecond,
		})
		
		strategy.RecordAttempt(RetryAttempt{
			Attempt: 2,
			Success: false,
			Delay:   2 * time.Second,
			Duration: 200 * time.Millisecond,
			Error:   errors.New("test error"),
		})
		
		stats := strategy.GetStats()
		assert.Equal(t, int64(2), stats.TotalAttempts)
		assert.Equal(t, int64(1), stats.TotalRetries)
		assert.Equal(t, int64(0), stats.SuccessfulRetries)
		assert.Equal(t, int64(1), stats.FailedRetries)
		assert.Equal(t, 2*time.Second, stats.TotalDelay)
		assert.Equal(t, 2*time.Second, stats.AverageDelay)
	})
}

func TestRetryExecutor(t *testing.T) {
	logger := slog.Default()
	
	t.Run("NewRetryExecutor", func(t *testing.T) {
		config := DefaultRetryStrategyConfig()
		executor, err := NewRetryExecutor(config, logger)
		
		require.NoError(t, err)
		assert.NotNil(t, executor)
	})

	t.Run("Execute - Success", func(t *testing.T) {
		config := DefaultRetryStrategyConfig()
		config.MaxAttempts = 3
		
		executor, err := NewRetryExecutor(config, logger)
		require.NoError(t, err)
		
		ctx := context.Background()
		operationID := "test-success"
		
		callCount := 0
		operation := func(ctx context.Context, attempt int) error {
			callCount++
			return nil // Always succeed
		}
		
		err = executor.Execute(ctx, operationID, operation)
		assert.NoError(t, err)
		assert.Equal(t, 1, callCount) // Should only call once
	})

	t.Run("Execute - Retry and Success", func(t *testing.T) {
		config := &RetryStrategyConfig{
			Strategy:    StrategyFixedDelay,
			Jitter:      JitterNone,
			MaxAttempts: 3,
			BaseDelay:   10 * time.Millisecond, // Short delay for testing
			MaxDelay:    1 * time.Second,
			BackoffFactor: 2.0,
			RetryConditions: []RetryCondition{
				{
					RetryableErrorTypes: []string{"temp"},
				},
			},
		}
		
		executor, err := NewRetryExecutor(config, logger)
		require.NoError(t, err)
		
		ctx := context.Background()
		operationID := "test-retry-success"
		
		callCount := 0
		operation := func(ctx context.Context, attempt int) error {
			callCount++
			if callCount < 3 {
				return errors.New("temp error") // Fail first two attempts
			}
			return nil // Succeed on third attempt
		}
		
		err = executor.Execute(ctx, operationID, operation)
		assert.NoError(t, err)
		assert.Equal(t, 3, callCount)
	})

	t.Run("Execute - Max Attempts Exceeded", func(t *testing.T) {
		config := &RetryStrategyConfig{
			Strategy:    StrategyFixedDelay,
			Jitter:      JitterNone,
			MaxAttempts: 2,
			BaseDelay:   10 * time.Millisecond,
			MaxDelay:    1 * time.Second,
			BackoffFactor: 2.0,
			RetryConditions: []RetryCondition{
				{
					RetryableErrorTypes: []string{"temp"},
				},
			},
		}
		
		executor, err := NewRetryExecutor(config, logger)
		require.NoError(t, err)
		
		ctx := context.Background()
		operationID := "test-max-attempts"
		
		callCount := 0
		operation := func(ctx context.Context, attempt int) error {
			callCount++
			return errors.New("temp error") // Always fail
		}
		
		err = executor.Execute(ctx, operationID, operation)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "operation failed after 2 attempts")
		assert.Equal(t, 2, callCount)
	})

	t.Run("ExecuteWithTimeout", func(t *testing.T) {
		config := DefaultRetryStrategyConfig()
		executor, err := NewRetryExecutor(config, logger)
		require.NoError(t, err)
		
		ctx := context.Background()
		operationID := "test-timeout"
		timeout := 50 * time.Millisecond
		
		operation := func(ctx context.Context, attempt int) error {
			select {
			case <-time.After(100 * time.Millisecond): // Longer than timeout
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		
		err = executor.ExecuteWithTimeout(ctx, operationID, timeout, operation)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "context deadline exceeded")
	})

	t.Run("GetExecutionStatus", func(t *testing.T) {
		config := DefaultRetryStrategyConfig()
		executor, err := NewRetryExecutor(config, logger)
		require.NoError(t, err)
		
		// Should return false for non-existent execution
		_, exists := executor.GetExecutionStatus("non-existent")
		assert.False(t, exists)
	})

	t.Run("CancelExecution", func(t *testing.T) {
		config := DefaultRetryStrategyConfig()
		executor, err := NewRetryExecutor(config, logger)
		require.NoError(t, err)
		
		// Should return false for non-existent execution
		cancelled := executor.CancelExecution("non-existent")
		assert.False(t, cancelled)
	})

	t.Run("GetStats", func(t *testing.T) {
		config := DefaultRetryStrategyConfig()
		executor, err := NewRetryExecutor(config, logger)
		require.NoError(t, err)
		
		stats := executor.GetStats()
		assert.Equal(t, 0, stats.ActiveExecutions)
		assert.NotNil(t, stats.RetryStats)
	})
}

func TestEnhancedRetryQueue(t *testing.T) {
	logger := slog.Default()
	
	t.Run("NewEnhancedRetryQueue", func(t *testing.T) {
		config := DefaultEnhancedRetryConfig()
		queue, err := NewEnhancedRetryQueue(config, logger)
		
		require.NoError(t, err)
		assert.NotNil(t, queue)
		assert.Equal(t, config, queue.config)
	})

	t.Run("EnqueueForRetry", func(t *testing.T) {
		config := DefaultEnhancedRetryConfig()
		queue, err := NewEnhancedRetryQueue(config, logger)
		require.NoError(t, err)
		
		ctx := context.Background()
		message := &EnhancedRetryMessage{
			TaskID:        uuid.New(),
			UserID:        uuid.New(),
			Attempts:      1,
			MaxAttempts:   3,
			OperationType: "test_operation",
			LastError:     "timeout",
		}
		
		err = queue.EnqueueForRetry(ctx, message)
		assert.NoError(t, err)
		assert.NotEmpty(t, message.ID)
		assert.False(t, message.CreatedAt.IsZero())
		assert.False(t, message.NextRetryAt.IsZero())
		
		stats := queue.GetStats()
		assert.Equal(t, int64(1), stats.TotalMessages)
		assert.Equal(t, int64(1), stats.PendingRetries)
	})

	t.Run("GetMessage", func(t *testing.T) {
		config := DefaultEnhancedRetryConfig()
		queue, err := NewEnhancedRetryQueue(config, logger)
		require.NoError(t, err)
		
		ctx := context.Background()
		message := &EnhancedRetryMessage{
			ID:            "test-message",
			TaskID:        uuid.New(),
			UserID:        uuid.New(),
			Attempts:      1,
			MaxAttempts:   3,
			OperationType: "test_operation",
		}
		
		err = queue.EnqueueForRetry(ctx, message)
		require.NoError(t, err)
		
		retrieved, exists := queue.GetMessage("test-message")
		assert.True(t, exists)
		assert.Equal(t, message.ID, retrieved.ID)
		assert.Equal(t, message.TaskID, retrieved.TaskID)
	})

	t.Run("RemoveMessage", func(t *testing.T) {
		config := DefaultEnhancedRetryConfig()
		queue, err := NewEnhancedRetryQueue(config, logger)
		require.NoError(t, err)
		
		ctx := context.Background()
		message := &EnhancedRetryMessage{
			ID:            "test-remove",
			TaskID:        uuid.New(),
			UserID:        uuid.New(),
			Attempts:      1,
			MaxAttempts:   3,
			OperationType: "test_operation",
		}
		
		err = queue.EnqueueForRetry(ctx, message)
		require.NoError(t, err)
		
		removed := queue.RemoveMessage("test-remove")
		assert.True(t, removed)
		
		_, exists := queue.GetMessage("test-remove")
		assert.False(t, exists)
	})

	t.Run("GetPendingMessages", func(t *testing.T) {
		config := DefaultEnhancedRetryConfig()
		queue, err := NewEnhancedRetryQueue(config, logger)
		require.NoError(t, err)
		
		ctx := context.Background()
		
		// Add multiple messages
		for i := 0; i < 3; i++ {
			message := &EnhancedRetryMessage{
				TaskID:        uuid.New(),
				UserID:        uuid.New(),
				Attempts:      1,
				MaxAttempts:   3,
				OperationType: "test_operation",
			}
			err = queue.EnqueueForRetry(ctx, message)
			require.NoError(t, err)
		}
		
		pending := queue.GetPendingMessages()
		assert.Len(t, pending, 3)
	})

	t.Run("GetStats", func(t *testing.T) {
		config := DefaultEnhancedRetryConfig()
		queue, err := NewEnhancedRetryQueue(config, logger)
		require.NoError(t, err)
		
		stats := queue.GetStats()
		assert.Equal(t, int64(0), stats.TotalMessages)
		assert.Equal(t, int64(0), stats.PendingRetries)
		assert.NotNil(t, stats.StrategyStats)
		assert.NotNil(t, stats.ErrorDistribution)
	})
}

func TestEnhancedRetryConfig(t *testing.T) {
	t.Run("DefaultEnhancedRetryConfig", func(t *testing.T) {
		config := DefaultEnhancedRetryConfig()
		
		assert.Equal(t, "enhanced_retry_queue", config.QueueName)
		assert.Equal(t, 10*time.Second, config.ProcessingInterval)
		assert.Equal(t, 100, config.MaxConcurrentRetries)
		assert.NotNil(t, config.RetryStrategy)
		assert.True(t, config.PersistRetries)
		assert.True(t, config.EnableMetrics)
		
		err := config.Validate()
		assert.NoError(t, err)
	})

	t.Run("Validation", func(t *testing.T) {
		tests := []struct {
			name      string
			config    *EnhancedRetryConfig
			expectErr bool
		}{
			{
				name:      "Valid config",
				config:    DefaultEnhancedRetryConfig(),
				expectErr: false,
			},
			{
				name: "Invalid processing interval",
				config: &EnhancedRetryConfig{
					ProcessingInterval:   0,
					MaxConcurrentRetries: 100,
					RetryTTL:             24 * time.Hour,
				},
				expectErr: true,
			},
			{
				name: "Invalid max concurrent retries",
				config: &EnhancedRetryConfig{
					ProcessingInterval:   10 * time.Second,
					MaxConcurrentRetries: 0,
					RetryTTL:             24 * time.Hour,
				},
				expectErr: true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := tt.config.Validate()
				if tt.expectErr {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})
}
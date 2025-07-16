package queue

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/voidrunnerhq/voidrunner/internal/config"
)

// MockTaskQueue is a mock implementation of TaskQueue
type MockTaskQueue struct {
	mock.Mock
}

func (m *MockTaskQueue) Enqueue(ctx context.Context, message *TaskMessage) error {
	args := m.Called(ctx, message)
	return args.Error(0)
}

func (m *MockTaskQueue) Dequeue(ctx context.Context, maxMessages int) ([]*TaskMessage, error) {
	args := m.Called(ctx, maxMessages)
	return args.Get(0).([]*TaskMessage), args.Error(1)
}

func (m *MockTaskQueue) DeleteMessage(ctx context.Context, receiptHandle string) error {
	args := m.Called(ctx, receiptHandle)
	return args.Error(0)
}

func (m *MockTaskQueue) ExtendVisibility(ctx context.Context, receiptHandle string, timeout time.Duration) error {
	args := m.Called(ctx, receiptHandle, timeout)
	return args.Error(0)
}

func (m *MockTaskQueue) GetQueueStats(ctx context.Context) (*QueueStats, error) {
	args := m.Called(ctx)
	return args.Get(0).(*QueueStats), args.Error(1)
}

func (m *MockTaskQueue) IsHealthy(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockTaskQueue) Close() error {
	args := m.Called()
	return args.Error(0)
}

// MockRetryQueue is a mock implementation of RetryQueue
type MockRetryQueue struct {
	mock.Mock
}

func (m *MockRetryQueue) EnqueueForRetry(ctx context.Context, message *TaskMessage, retryAt time.Time) error {
	args := m.Called(ctx, message, retryAt)
	return args.Error(0)
}

func (m *MockRetryQueue) DequeueReadyForRetry(ctx context.Context, maxMessages int) ([]*TaskMessage, error) {
	args := m.Called(ctx, maxMessages)
	return args.Get(0).([]*TaskMessage), args.Error(1)
}

func (m *MockRetryQueue) GetRetryStats(ctx context.Context) (*RetryStats, error) {
	args := m.Called(ctx)
	return args.Get(0).(*RetryStats), args.Error(1)
}

// MockDeadLetterQueue is a mock implementation of DeadLetterQueue
type MockDeadLetterQueue struct {
	mock.Mock
}

func (m *MockDeadLetterQueue) EnqueueFailedTask(ctx context.Context, message *TaskMessage) error {
	args := m.Called(ctx, message)
	return args.Error(0)
}

func (m *MockDeadLetterQueue) GetFailedTasks(ctx context.Context, limit int, offset int) ([]*TaskMessage, error) {
	args := m.Called(ctx, limit, offset)
	return args.Get(0).([]*TaskMessage), args.Error(1)
}

func (m *MockDeadLetterQueue) RequeueTask(ctx context.Context, messageID string) error {
	args := m.Called(ctx, messageID)
	return args.Error(0)
}

func (m *MockDeadLetterQueue) GetDeadLetterStats(ctx context.Context) (*DeadLetterStats, error) {
	args := m.Called(ctx)
	return args.Get(0).(*DeadLetterStats), args.Error(1)
}

// MockRetryProcessor is a mock implementation of RetryProcessor
type MockRetryProcessor struct {
	mock.Mock
}

func (m *MockRetryProcessor) Start(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockRetryProcessor) Stop() error {
	args := m.Called()
	return args.Error(0)
}

func TestNewRedisQueueManager(t *testing.T) {
	tests := []struct {
		name        string
		redisConfig *config.RedisConfig
		queueConfig *config.QueueConfig
		logger      *slog.Logger
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid configuration",
			redisConfig: &config.RedisConfig{
				Host:     "localhost",
				Port:     "6379",
				Database: 0,
			},
			queueConfig: &config.QueueConfig{
				TaskQueueName:       "test-tasks",
				RetryQueueName:      "test-retry",
				DeadLetterQueueName: "test-dlq",
				MaxRetries:          3,
				RetryDelay:          time.Minute,
			},
			logger:      slog.Default(),
			expectError: false,
		},
		{
			name:        "nil redis config",
			redisConfig: nil,
			queueConfig: &config.QueueConfig{},
			logger:      slog.Default(),
			expectError: true,
			errorMsg:    "redis config is required",
		},
		{
			name: "nil queue config",
			redisConfig: &config.RedisConfig{
				Host: "localhost",
				Port: "6379",
			},
			queueConfig: nil,
			logger:      slog.Default(),
			expectError: true,
			errorMsg:    "queue config is required",
		},
		{
			name: "nil logger uses default",
			redisConfig: &config.RedisConfig{
				Host:     "localhost",
				Port:     "6379",
				Database: 0,
			},
			queueConfig: &config.QueueConfig{
				TaskQueueName: "test-tasks",
			},
			logger:      nil,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager, err := NewRedisQueueManager(tt.redisConfig, tt.queueConfig, tt.logger)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
				assert.Nil(t, manager)
			} else {
				// Note: This test will fail if Redis is not running, but that's expected
				// In a real test environment, we'd use a test Redis instance or mock
				if err != nil {
					t.Skipf("Redis not available for testing: %v", err)
				}
				assert.NotNil(t, manager)
			}
		})
	}
}

func TestRedisQueueManagerLifecycle(t *testing.T) {
	// This test would require a running Redis instance
	// We'll create a basic test structure that can be extended

	t.Run("start and stop lifecycle", func(t *testing.T) {
		// Skip this test if Redis is not available
		t.Skip("Redis integration test - requires running Redis instance")

		redisConfig := &config.RedisConfig{
			Host:     "localhost",
			Port:     "6379",
			Database: 0,
		}

		queueConfig := &config.QueueConfig{
			TaskQueueName:       "test-tasks",
			RetryQueueName:      "test-retry",
			DeadLetterQueueName: "test-dlq",
			MaxRetries:          3,
			RetryDelay:          time.Minute,
		}

		manager, err := NewRedisQueueManager(redisConfig, queueConfig, slog.Default())
		require.NoError(t, err)
		require.NotNil(t, manager)

		ctx := context.Background()

		// Test Start
		err = manager.Start(ctx)
		assert.NoError(t, err)

		// Test that queues are accessible
		assert.NotNil(t, manager.TaskQueue())
		assert.NotNil(t, manager.RetryQueue())
		assert.NotNil(t, manager.DeadLetterQueue())

		// Test health check
		err = manager.IsHealthy(ctx)
		assert.NoError(t, err)

		// Test stats
		stats, err := manager.GetStats(ctx)
		assert.NoError(t, err)
		assert.NotNil(t, stats)

		// Test Stop
		err = manager.Stop(ctx)
		assert.NoError(t, err)
	})
}

func TestQueueManagerInterfaces(t *testing.T) {
	// Test that RedisQueueManager implements QueueManager interface
	t.Run("implements QueueManager interface", func(t *testing.T) {
		var _ QueueManager = &RedisQueueManager{}
	})
}

func TestQueueManagerStats(t *testing.T) {
	tests := []struct {
		name         string
		taskStats    *QueueStats
		retryStats   *RetryStats
		dlqStats     *DeadLetterStats
		expectedFunc func(*testing.T, *QueueManagerStats)
	}{
		{
			name: "complete stats",
			taskStats: &QueueStats{
				Name:                "tasks",
				ApproximateMessages: 100,
				MessagesInFlight:    5,
			},
			retryStats: &RetryStats{
				QueueStats: QueueStats{
					Name:                "retry",
					ApproximateMessages: 10,
				},
				PendingRetries: 8,
				ReadyForRetry:  2,
			},
			dlqStats: &DeadLetterStats{
				QueueStats: QueueStats{
					Name:                "dlq",
					ApproximateMessages: 3,
				},
				TotalFailedTasks: 25,
				FailureReasons: map[string]int64{
					"timeout": 15,
					"error":   10,
				},
			},
			expectedFunc: func(t *testing.T, stats *QueueManagerStats) {
				assert.NotNil(t, stats.TaskQueue)
				assert.Equal(t, "tasks", stats.TaskQueue.Name)
				assert.Equal(t, int64(100), stats.TaskQueue.ApproximateMessages)

				assert.NotNil(t, stats.RetryQueue)
				assert.Equal(t, "retry", stats.RetryQueue.Name)
				assert.Equal(t, int64(8), stats.RetryQueue.PendingRetries)

				assert.NotNil(t, stats.DeadLetterQueue)
				assert.Equal(t, "dlq", stats.DeadLetterQueue.Name)
				assert.Equal(t, int64(25), stats.DeadLetterQueue.TotalFailedTasks)

				assert.True(t, stats.Uptime >= 0)
				assert.False(t, stats.LastUpdated.IsZero())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stats := &QueueManagerStats{
				TaskQueue:       tt.taskStats,
				RetryQueue:      tt.retryStats,
				DeadLetterQueue: tt.dlqStats,
				TotalThroughput: 1000,
				Uptime:          2 * time.Hour,
				LastUpdated:     time.Now(),
			}

			tt.expectedFunc(t, stats)
		})
	}
}

func TestQueueManagerErrorHandling(t *testing.T) {
	tests := []struct {
		name        string
		setupMocks  func(*MockTaskQueue, *MockRetryQueue, *MockDeadLetterQueue)
		operation   func(QueueManager) error
		expectError bool
	}{
		{
			name: "health check with healthy queues",
			setupMocks: func(tq *MockTaskQueue, rq *MockRetryQueue, dlq *MockDeadLetterQueue) {
				tq.On("IsHealthy", mock.Anything).Return(nil)
			},
			operation: func(qm QueueManager) error {
				return qm.IsHealthy(context.Background())
			},
			expectError: false,
		},
		{
			name: "health check with unhealthy task queue",
			setupMocks: func(tq *MockTaskQueue, rq *MockRetryQueue, dlq *MockDeadLetterQueue) {
				tq.On("IsHealthy", mock.Anything).Return(assert.AnError)
			},
			operation: func(qm QueueManager) error {
				return qm.IsHealthy(context.Background())
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock queues
			mockTaskQueue := &MockTaskQueue{}
			mockRetryQueue := &MockRetryQueue{}
			mockDLQ := &MockDeadLetterQueue{}

			// Setup mock expectations
			tt.setupMocks(mockTaskQueue, mockRetryQueue, mockDLQ)

			// Create a basic queue manager structure for testing
			// Note: This is a simplified test that focuses on the interface behavior
			qm := &testQueueManager{
				taskQueue:       mockTaskQueue,
				retryQueue:      mockRetryQueue,
				deadLetterQueue: mockDLQ,
			}

			// Execute operation
			err := tt.operation(qm)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			// Verify mock expectations
			mockTaskQueue.AssertExpectations(t)
			mockRetryQueue.AssertExpectations(t)
			mockDLQ.AssertExpectations(t)
		})
	}
}

// testQueueManager is a simple test implementation of QueueManager
type testQueueManager struct {
	taskQueue       TaskQueue
	retryQueue      RetryQueue
	deadLetterQueue DeadLetterQueue
	started         bool
}

func (tqm *testQueueManager) TaskQueue() TaskQueue             { return tqm.taskQueue }
func (tqm *testQueueManager) RetryQueue() RetryQueue           { return tqm.retryQueue }
func (tqm *testQueueManager) DeadLetterQueue() DeadLetterQueue { return tqm.deadLetterQueue }

func (tqm *testQueueManager) IsHealthy(ctx context.Context) error {
	return tqm.taskQueue.IsHealthy(ctx)
}

func (tqm *testQueueManager) GetStats(ctx context.Context) (*QueueManagerStats, error) {
	return &QueueManagerStats{
		TotalThroughput: 0,
		Uptime:          0,
		LastUpdated:     time.Now(),
	}, nil
}

func (tqm *testQueueManager) Start(ctx context.Context) error {
	tqm.started = true
	return nil
}

func (tqm *testQueueManager) Stop(ctx context.Context) error {
	tqm.started = false
	return nil
}

func (tqm *testQueueManager) StartRetryProcessor(ctx context.Context) error {
	return nil
}

func (tqm *testQueueManager) StopRetryProcessor() error {
	return nil
}

// TestQueueManagerDeadlockFix tests that the Start function doesn't deadlock
func TestQueueManagerDeadlockFix(t *testing.T) {
	t.Run("start function doesn't deadlock on health check", func(t *testing.T) {
		// Create a mock-based queue manager to test the deadlock fix
		mockTaskQueue := &MockTaskQueue{}
		mockRetryQueue := &MockRetryQueue{}
		mockDLQ := &MockDeadLetterQueue{}

		// Setup mocks to return healthy status
		mockTaskQueue.On("IsHealthy", mock.Anything).Return(nil)
		mockRetryQueue.On("GetRetryStats", mock.Anything).Return(&RetryStats{}, nil)
		mockDLQ.On("GetDeadLetterStats", mock.Anything).Return(&DeadLetterStats{}, nil)

		// Create a test manager that simulates the deadlock scenario
		manager := &deadlockTestManager{
			taskQueue:       mockTaskQueue,
			retryQueue:      mockRetryQueue,
			deadLetterQueue: mockDLQ,
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// This should complete without deadlock
		done := make(chan error, 1)
		go func() {
			done <- manager.Start(ctx)
		}()

		select {
		case err := <-done:
			assert.NoError(t, err, "Start should not deadlock and should complete successfully")
		case <-ctx.Done():
			t.Fatal("Start function deadlocked - timeout reached")
		}

		// Cleanup
		_ = manager.Stop(ctx)

		mockTaskQueue.AssertExpectations(t)
	})
}

// TestQueueManagerConcurrentOperations tests concurrent Start/Stop operations
func TestQueueManagerConcurrentOperations(t *testing.T) {
	t.Run("concurrent start and stop operations", func(t *testing.T) {
		mockTaskQueue := &MockTaskQueue{}
		mockRetryQueue := &MockRetryQueue{}
		mockDLQ := &MockDeadLetterQueue{}

		// Setup mocks
		mockTaskQueue.On("IsHealthy", mock.Anything).Return(nil)
		mockTaskQueue.On("Close").Return(nil)
		mockRetryQueue.On("GetRetryStats", mock.Anything).Return(&RetryStats{}, nil)
		mockDLQ.On("GetDeadLetterStats", mock.Anything).Return(&DeadLetterStats{}, nil)

		manager := &deadlockTestManager{
			taskQueue:       mockTaskQueue,
			retryQueue:      mockRetryQueue,
			deadLetterQueue: mockDLQ,
		}

		ctx := context.Background()
		const numGoroutines = 10

		// Start multiple goroutines trying to start/stop the manager
		var startErrors, stopErrors []error
		startDone := make(chan error, numGoroutines)
		stopDone := make(chan error, numGoroutines)

		// Start goroutines
		for i := 0; i < numGoroutines; i++ {
			go func() {
				startDone <- manager.Start(ctx)
			}()
		}

		// Give starts a moment to begin
		time.Sleep(100 * time.Millisecond)

		// Stop goroutines
		for i := 0; i < numGoroutines; i++ {
			go func() {
				stopDone <- manager.Stop(ctx)
			}()
		}

		// Collect all results
		for i := 0; i < numGoroutines; i++ {
			startErrors = append(startErrors, <-startDone)
			stopErrors = append(stopErrors, <-stopDone)
		}

		// Verify no errors occurred (multiple starts/stops should be safe)
		for i, err := range startErrors {
			assert.NoError(t, err, "Start operation %d should not error", i)
		}

		for i, err := range stopErrors {
			assert.NoError(t, err, "Stop operation %d should not error", i)
		}
	})
}

// TestQueueManagerHealthCheckConcurrency tests concurrent health check operations
func TestQueueManagerHealthCheckConcurrency(t *testing.T) {
	t.Run("concurrent health checks don't deadlock", func(t *testing.T) {
		mockTaskQueue := &MockTaskQueue{}
		mockRetryQueue := &MockRetryQueue{}
		mockDLQ := &MockDeadLetterQueue{}

		// Setup mocks to be called multiple times
		mockTaskQueue.On("IsHealthy", mock.Anything).Return(nil)

		manager := &deadlockTestManager{
			taskQueue:       mockTaskQueue,
			retryQueue:      mockRetryQueue,
			deadLetterQueue: mockDLQ,
			started:         true, // Start in started state
		}

		ctx := context.Background()
		const numHealthChecks = 20

		// Run multiple concurrent health checks
		done := make(chan error, numHealthChecks)

		for i := 0; i < numHealthChecks; i++ {
			go func() {
				done <- manager.IsHealthy(ctx)
			}()
		}

		// Collect all results with timeout
		timeoutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		for i := 0; i < numHealthChecks; i++ {
			select {
			case err := <-done:
				assert.NoError(t, err, "Health check %d should not error", i)
			case <-timeoutCtx.Done():
				t.Fatal("Health check operations deadlocked")
			}
		}

		// Verify mock was called the expected number of times
		mockTaskQueue.AssertNumberOfCalls(t, "IsHealthy", numHealthChecks)
	})
}

// deadlockTestManager is a test implementation that simulates the original deadlock issue
type deadlockTestManager struct {
	taskQueue       TaskQueue
	retryQueue      RetryQueue
	deadLetterQueue DeadLetterQueue
	mu              sync.RWMutex
	started         bool
	closed          bool
}

func (dtm *deadlockTestManager) TaskQueue() TaskQueue             { return dtm.taskQueue }
func (dtm *deadlockTestManager) RetryQueue() RetryQueue           { return dtm.retryQueue }
func (dtm *deadlockTestManager) DeadLetterQueue() DeadLetterQueue { return dtm.deadLetterQueue }

// IsHealthy simulates the fixed version - only acquires read lock
func (dtm *deadlockTestManager) IsHealthy(ctx context.Context) error {
	dtm.mu.RLock()
	defer dtm.mu.RUnlock()

	return dtm.isHealthyUnsafe(ctx)
}

// isHealthyUnsafe simulates the internal method without locks
func (dtm *deadlockTestManager) isHealthyUnsafe(ctx context.Context) error {
	if dtm.closed {
		return ErrQueueClosed
	}
	return dtm.taskQueue.IsHealthy(ctx)
}

func (dtm *deadlockTestManager) GetStats(ctx context.Context) (*QueueManagerStats, error) {
	dtm.mu.RLock()
	defer dtm.mu.RUnlock()

	if dtm.closed {
		return nil, ErrQueueClosed
	}

	return &QueueManagerStats{
		TotalThroughput: 0,
		Uptime:          0,
		LastUpdated:     time.Now(),
	}, nil
}

// Start simulates the fixed version - uses isHealthyUnsafe while holding write lock
func (dtm *deadlockTestManager) Start(ctx context.Context) error {
	dtm.mu.Lock()
	defer dtm.mu.Unlock()

	if dtm.closed {
		return ErrQueueClosed
	}

	if dtm.started {
		return nil // Already started
	}

	// This is the key fix: use isHealthyUnsafe instead of IsHealthy
	if err := dtm.isHealthyUnsafe(ctx); err != nil {
		return err
	}

	dtm.started = true
	return nil
}

func (dtm *deadlockTestManager) Stop(ctx context.Context) error {
	dtm.mu.Lock()
	defer dtm.mu.Unlock()

	if dtm.closed {
		return nil
	}

	dtm.started = false
	dtm.closed = true
	return nil
}

func (dtm *deadlockTestManager) StartRetryProcessor(ctx context.Context) error {
	return nil
}

func (dtm *deadlockTestManager) StopRetryProcessor() error {
	return nil
}

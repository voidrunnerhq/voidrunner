package worker

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/voidrunnerhq/voidrunner/internal/database"
	"github.com/voidrunnerhq/voidrunner/internal/executor"
	"github.com/voidrunnerhq/voidrunner/internal/queue"
)

// MockQueueManager implements QueueManager for testing
type MockQueueManager struct {
	mock.Mock
	taskQueue       *MockTaskQueue
	retryQueue      *MockRetryQueue
	deadLetterQueue *MockDeadLetterQueue
}

func NewMockQueueManager() *MockQueueManager {
	return &MockQueueManager{
		taskQueue:       NewMockTaskQueue(),
		retryQueue:      NewMockRetryQueue(),
		deadLetterQueue: NewMockDeadLetterQueue(),
	}
}

func (m *MockQueueManager) TaskQueue() queue.TaskQueue {
	return m.taskQueue
}

func (m *MockQueueManager) RetryQueue() queue.RetryQueue {
	return m.retryQueue
}

func (m *MockQueueManager) DeadLetterQueue() queue.DeadLetterQueue {
	return m.deadLetterQueue
}

func (m *MockQueueManager) Start(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockQueueManager) Stop(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockQueueManager) IsHealthy(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockQueueManager) GetStats(ctx context.Context) (*queue.QueueManagerStats, error) {
	args := m.Called(ctx)
	return args.Get(0).(*queue.QueueManagerStats), args.Error(1)
}

func (m *MockQueueManager) StartRetryProcessor(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockQueueManager) StopRetryProcessor() error {
	args := m.Called()
	return args.Error(0)
}

// MockTaskQueue implements TaskQueue for testing
type MockTaskQueue struct {
	mock.Mock
}

func NewMockTaskQueue() *MockTaskQueue {
	return &MockTaskQueue{}
}

func (m *MockTaskQueue) Enqueue(ctx context.Context, message *queue.TaskMessage) error {
	args := m.Called(ctx, message)
	return args.Error(0)
}

func (m *MockTaskQueue) Dequeue(ctx context.Context, maxMessages int) ([]*queue.TaskMessage, error) {
	args := m.Called(ctx, maxMessages)
	return args.Get(0).([]*queue.TaskMessage), args.Error(1)
}

func (m *MockTaskQueue) DeleteMessage(ctx context.Context, receiptHandle string) error {
	args := m.Called(ctx, receiptHandle)
	return args.Error(0)
}

func (m *MockTaskQueue) ExtendVisibility(ctx context.Context, receiptHandle string, timeout time.Duration) error {
	args := m.Called(ctx, receiptHandle, timeout)
	return args.Error(0)
}

func (m *MockTaskQueue) GetQueueStats(ctx context.Context) (*queue.QueueStats, error) {
	args := m.Called(ctx)
	return args.Get(0).(*queue.QueueStats), args.Error(1)
}

func (m *MockTaskQueue) IsHealthy(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockTaskQueue) Close() error {
	args := m.Called()
	return args.Error(0)
}

// MockRetryQueue implements RetryQueue for testing
type MockRetryQueue struct {
	mock.Mock
}

func NewMockRetryQueue() *MockRetryQueue {
	return &MockRetryQueue{}
}

func (m *MockRetryQueue) EnqueueForRetry(ctx context.Context, message *queue.TaskMessage, retryAt time.Time) error {
	args := m.Called(ctx, message, retryAt)
	return args.Error(0)
}

func (m *MockRetryQueue) DequeueReadyForRetry(ctx context.Context, maxMessages int) ([]*queue.TaskMessage, error) {
	args := m.Called(ctx, maxMessages)
	return args.Get(0).([]*queue.TaskMessage), args.Error(1)
}

func (m *MockRetryQueue) ProcessRetries(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockRetryQueue) GetRetryStats(ctx context.Context) (*queue.RetryStats, error) {
	args := m.Called(ctx)
	return args.Get(0).(*queue.RetryStats), args.Error(1)
}

func (m *MockRetryQueue) IsHealthy(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockRetryQueue) Close() error {
	args := m.Called()
	return args.Error(0)
}

// MockDeadLetterQueue implements DeadLetterQueue for testing
type MockDeadLetterQueue struct {
	mock.Mock
}

func NewMockDeadLetterQueue() *MockDeadLetterQueue {
	return &MockDeadLetterQueue{}
}

func (m *MockDeadLetterQueue) EnqueueFailedTask(ctx context.Context, message *queue.TaskMessage) error {
	args := m.Called(ctx, message)
	return args.Error(0)
}

func (m *MockDeadLetterQueue) GetFailedTasks(ctx context.Context, limit int, offset int) ([]*queue.TaskMessage, error) {
	args := m.Called(ctx, limit, offset)
	return args.Get(0).([]*queue.TaskMessage), args.Error(1)
}

func (m *MockDeadLetterQueue) RequeueTask(ctx context.Context, messageID string) error {
	args := m.Called(ctx, messageID)
	return args.Error(0)
}

func (m *MockDeadLetterQueue) GetDeadLetterStats(ctx context.Context) (*queue.DeadLetterStats, error) {
	args := m.Called(ctx)
	return args.Get(0).(*queue.DeadLetterStats), args.Error(1)
}

// MockTaskExecutor implements TaskExecutor for testing
type MockTaskExecutor struct {
	mock.Mock
}

func (m *MockTaskExecutor) Execute(ctx context.Context, execCtx *executor.ExecutionContext) (*executor.ExecutionResult, error) {
	args := m.Called(ctx, execCtx)
	return args.Get(0).(*executor.ExecutionResult), args.Error(1)
}

func (m *MockTaskExecutor) IsHealthy(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockTaskExecutor) Cancel(ctx context.Context, executionID uuid.UUID) error {
	args := m.Called(ctx, executionID)
	return args.Error(0)
}

func (m *MockTaskExecutor) Cleanup(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

// Test fixtures
func createTestWorkerManager(t *testing.T) (*BaseWorkerManager, *MockQueueManager, *MockTaskExecutor, *database.Repositories) {
	queueManager := NewMockQueueManager()
	taskExecutor := &MockTaskExecutor{}
	repos := &database.Repositories{} // For testing, we can use empty repos

	config := WorkerConfig{
		WorkerIDPrefix:       "test-worker",
		HeartbeatInterval:    5 * time.Second,
		TaskTimeout:          30 * time.Second,
		HealthCheckInterval:  10 * time.Second,
		ShutdownTimeout:      15 * time.Second,
		MaxRetryAttempts:     3,
		ProcessingSlotTTL:    5 * time.Minute,
		StaleTaskThreshold:   1 * time.Minute,
		EnableAutoScaling:    true,
		ScalingCheckInterval: 30 * time.Second,
	}

	logger := slog.Default()

	wm := NewWorkerManager(queueManager, taskExecutor, repos, config, logger).(*BaseWorkerManager)

	return wm, queueManager, taskExecutor, repos
}

func TestNewWorkerManager(t *testing.T) {
	wm, queueManager, taskExecutor, repos := createTestWorkerManager(t)

	assert.NotNil(t, wm)
	assert.Equal(t, queueManager, wm.queueManager)
	assert.Equal(t, taskExecutor, wm.executor)
	assert.Equal(t, repos, wm.repos)
	assert.NotNil(t, wm.concurrency)
	assert.NotNil(t, wm.workerPool)
	assert.NotNil(t, wm.processorRegistry)
	assert.False(t, wm.isRunning)
	assert.True(t, wm.isHealthy)
}

func TestWorkerManager_StartAndStop(t *testing.T) {
	wm, queueManager, taskExecutor, _ := createTestWorkerManager(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Mock queue manager start
	queueManager.On("Start", mock.Anything).Return(nil)
	queueManager.On("IsHealthy", mock.Anything).Return(nil)
	taskExecutor.On("IsHealthy", mock.Anything).Return(nil)
	queueManager.taskQueue.On("IsHealthy", mock.Anything).Return(nil)

	// Start the worker manager
	err := wm.Start(ctx)
	require.NoError(t, err)

	// Verify it's running
	assert.True(t, wm.IsRunning())
	assert.True(t, wm.IsHealthy())

	// Stop the worker manager
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer stopCancel()

	err = wm.Stop(stopCtx)
	require.NoError(t, err)

	// Verify it's stopped
	assert.False(t, wm.IsRunning())

	queueManager.AssertExpectations(t)
	taskExecutor.AssertExpectations(t)
}

func TestWorkerManager_StartAlreadyRunning(t *testing.T) {
	wm, queueManager, taskExecutor, _ := createTestWorkerManager(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Mock dependencies
	queueManager.On("Start", mock.Anything).Return(nil)
	queueManager.On("IsHealthy", mock.Anything).Return(nil)
	taskExecutor.On("IsHealthy", mock.Anything).Return(nil)
	queueManager.taskQueue.On("IsHealthy", mock.Anything).Return(nil)

	// Start the worker manager
	err := wm.Start(ctx)
	require.NoError(t, err)

	// Try to start again - should return error
	err = wm.Start(ctx)
	assert.Error(t, err)
	assert.Equal(t, ErrWorkerManagerAlreadyRunning, err)

	// Cleanup
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()
	_ = wm.Stop(stopCtx)
}

func TestWorkerManager_StopNotRunning(t *testing.T) {
	wm, _, _, _ := createTestWorkerManager(t)

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()

	// Try to stop when not running
	err := wm.Stop(stopCtx)
	assert.Error(t, err)
	assert.Equal(t, ErrWorkerManagerNotRunning, err)
}

func TestWorkerManager_GetStats(t *testing.T) {
	wm, queueManager, taskExecutor, _ := createTestWorkerManager(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Mock dependencies
	queueManager.On("Start", mock.Anything).Return(nil)
	queueManager.On("IsHealthy", mock.Anything).Return(nil)
	taskExecutor.On("IsHealthy", mock.Anything).Return(nil)
	queueManager.taskQueue.On("IsHealthy", mock.Anything).Return(nil)

	// Start the worker manager
	err := wm.Start(ctx)
	require.NoError(t, err)
	defer func() {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer stopCancel()
		_ = wm.Stop(stopCtx)
	}()

	// Get stats
	stats := wm.GetStats()

	assert.True(t, stats.IsRunning)
	assert.True(t, stats.IsHealthy)
	assert.NotZero(t, stats.StartedAt)
	assert.NotNil(t, stats.WorkerPoolStats)
	assert.NotNil(t, stats.ConcurrencyStats)
}

func TestWorkerManager_ConcurrentStartStop(t *testing.T) {
	wm, queueManager, taskExecutor, _ := createTestWorkerManager(t)

	// Mock dependencies
	queueManager.On("Start", mock.Anything).Return(nil)
	queueManager.On("IsHealthy", mock.Anything).Return(nil)
	taskExecutor.On("IsHealthy", mock.Anything).Return(nil)
	queueManager.taskQueue.On("IsHealthy", mock.Anything).Return(nil)

	var wg sync.WaitGroup
	var startErr, stopErr error

	// Concurrent start operations
	wg.Add(2)
	go func() {
		defer wg.Done()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		startErr = wm.Start(ctx)
	}()

	// Small delay to ensure first start begins
	time.Sleep(10 * time.Millisecond)

	go func() {
		defer wg.Done()
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer stopCancel()
		time.Sleep(50 * time.Millisecond) // Let start complete first
		stopErr = wm.Stop(stopCtx)
	}()

	wg.Wait()

	// One should succeed, depends on timing
	assert.NoError(t, startErr)
	assert.NoError(t, stopErr)
	assert.False(t, wm.IsRunning())
}

func TestWorkerManager_HealthCheck(t *testing.T) {
	wm, queueManager, taskExecutor, _ := createTestWorkerManager(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Mock healthy dependencies
	queueManager.On("Start", mock.Anything).Return(nil)
	queueManager.On("IsHealthy", mock.Anything).Return(nil)
	taskExecutor.On("IsHealthy", mock.Anything).Return(nil)
	queueManager.taskQueue.On("IsHealthy", mock.Anything).Return(nil)

	// Start the worker manager
	err := wm.Start(ctx)
	require.NoError(t, err)
	defer func() {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer stopCancel()
		_ = wm.Stop(stopCtx)
	}()

	// Initially should be healthy
	assert.True(t, wm.IsHealthy())

	// Wait a bit for health checks to run
	time.Sleep(100 * time.Millisecond)

	// Should still be healthy
	assert.True(t, wm.IsHealthy())
}

func TestWorkerManager_GetWorkerPool(t *testing.T) {
	wm, _, _, _ := createTestWorkerManager(t)

	pool := wm.GetWorkerPool()
	assert.NotNil(t, pool)
	assert.Equal(t, wm.workerPool, pool)
}

func TestWorkerManager_GetConcurrencyLimits(t *testing.T) {
	wm, _, _, _ := createTestWorkerManager(t)

	limits := wm.GetConcurrencyLimits()
	assert.NotNil(t, limits)
}

func TestWorkerManager_ProcessorRegistry(t *testing.T) {
	wm, _, _, _ := createTestWorkerManager(t)

	// Test processor registry functionality
	registry := wm.processorRegistry
	assert.NotNil(t, registry)

	// The registry should be accessible through the worker manager
	// This tests that the worker manager properly initializes its components
}

func TestWorkerManager_ConfigurationValidation(t *testing.T) {
	tests := []struct {
		name          string
		config        WorkerConfig
		expectedValid bool
	}{
		{
			name: "valid configuration",
			config: WorkerConfig{
				WorkerIDPrefix:       "valid-worker",
				HeartbeatInterval:    5 * time.Second,
				TaskTimeout:          30 * time.Second,
				HealthCheckInterval:  10 * time.Second,
				ShutdownTimeout:      15 * time.Second,
				MaxRetryAttempts:     3,
				ProcessingSlotTTL:    5 * time.Minute,
				StaleTaskThreshold:   1 * time.Minute,
				EnableAutoScaling:    true,
				ScalingCheckInterval: 30 * time.Second,
			},
			expectedValid: true,
		},
		{
			name: "empty worker ID prefix",
			config: WorkerConfig{
				WorkerIDPrefix:       "",
				HeartbeatInterval:    5 * time.Second,
				TaskTimeout:          30 * time.Second,
				HealthCheckInterval:  10 * time.Second,
				ShutdownTimeout:      15 * time.Second,
				MaxRetryAttempts:     3,
				ProcessingSlotTTL:    5 * time.Minute,
				StaleTaskThreshold:   1 * time.Minute,
				EnableAutoScaling:    true,
				ScalingCheckInterval: 30 * time.Second,
			},
			expectedValid: true, // Empty prefix should use default
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			queueManager := NewMockQueueManager()
			taskExecutor := &MockTaskExecutor{}
			repos := &database.Repositories{}
			logger := slog.Default()

			// Creating worker manager should not panic with any valid config
			wm := NewWorkerManager(queueManager, taskExecutor, repos, tt.config, logger)
			assert.NotNil(t, wm)
		})
	}
}

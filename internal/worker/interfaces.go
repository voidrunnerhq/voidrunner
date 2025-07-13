package worker

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/voidrunnerhq/voidrunner/internal/executor"
	"github.com/voidrunnerhq/voidrunner/internal/queue"
)

// Worker defines the interface for background task workers
type Worker interface {
	// Start begins the worker's processing loop
	Start(ctx context.Context) error

	// Stop gracefully stops the worker
	Stop(ctx context.Context) error

	// IsRunning returns true if the worker is currently running
	IsRunning() bool

	// GetID returns the unique worker ID
	GetID() string

	// GetStats returns worker statistics
	GetStats() WorkerStats

	// IsHealthy checks if the worker is healthy
	IsHealthy() bool
}

// WorkerPool defines the interface for managing multiple workers
type WorkerPool interface {
	// Start starts all workers in the pool
	Start(ctx context.Context) error

	// Stop gracefully stops all workers in the pool
	Stop(ctx context.Context) error

	// IsRunning returns true if the pool is running
	IsRunning() bool

	// GetWorkerCount returns the number of workers in the pool
	GetWorkerCount() int

	// GetActiveWorkers returns the number of actively processing workers
	GetActiveWorkers() int

	// GetStats returns pool statistics
	GetStats() WorkerPoolStats

	// IsHealthy checks if the worker pool is healthy
	IsHealthy() bool

	// AddWorker adds a new worker to the pool
	AddWorker() error

	// RemoveWorker removes a worker from the pool
	RemoveWorker() error

	// ScaleUp increases the number of workers
	ScaleUp(count int) error

	// ScaleDown decreases the number of workers
	ScaleDown(count int) error
}

// WorkerManager manages worker pools and provides coordination
type WorkerManager interface {
	// Start starts the worker manager and all pools
	Start(ctx context.Context) error

	// Stop gracefully stops the worker manager and all pools
	Stop(ctx context.Context) error

	// IsRunning returns true if the manager is running
	IsRunning() bool

	// GetWorkerPool returns the worker pool
	GetWorkerPool() WorkerPool

	// GetStats returns comprehensive manager statistics
	GetStats() WorkerManagerStats

	// IsHealthy checks if the worker manager is healthy
	IsHealthy() bool

	// HandleTaskExecution processes a task execution request
	HandleTaskExecution(ctx context.Context, message *queue.TaskMessage) error

	// HandleTaskCancellation handles task cancellation
	HandleTaskCancellation(ctx context.Context, executionID uuid.UUID) error

	// GetConcurrencyLimits returns current concurrency limits
	GetConcurrencyLimits() ConcurrencyLimits

	// UpdateConcurrencyLimits updates concurrency limits
	UpdateConcurrencyLimits(limits ConcurrencyLimits) error
}

// TaskProcessor defines the interface for processing individual tasks
type TaskProcessor interface {
	// ProcessTask processes a single task message
	ProcessTask(ctx context.Context, message *queue.TaskMessage) error

	// CanProcessTask checks if the processor can handle the given task
	CanProcessTask(message *queue.TaskMessage) bool

	// GetProcessorType returns the type of processor
	GetProcessorType() ProcessorType

	// IsHealthy checks if the processor is healthy
	IsHealthy() bool
}

// ConcurrencyManager manages task concurrency limits
type ConcurrencyManager interface {
	// AcquireSlot attempts to acquire a processing slot
	AcquireSlot(ctx context.Context, userID uuid.UUID) (*ProcessingSlot, error)

	// ReleaseSlot releases a processing slot
	ReleaseSlot(slot *ProcessingSlot) error

	// GetUserConcurrency returns current concurrency for a user
	GetUserConcurrency(userID uuid.UUID) int

	// GetTotalConcurrency returns total active processing slots
	GetTotalConcurrency() int

	// GetLimits returns current concurrency limits
	GetLimits() ConcurrencyLimits

	// UpdateLimits updates concurrency limits
	UpdateLimits(limits ConcurrencyLimits) error

	// IsUserAtLimit checks if user has reached concurrency limit
	IsUserAtLimit(userID uuid.UUID) bool

	// GetStats returns concurrency statistics
	GetStats() ConcurrencyStats
}

// ProcessingSlot represents an acquired processing slot
type ProcessingSlot struct {
	ID         uuid.UUID `json:"id"`
	UserID     uuid.UUID `json:"user_id"`
	TaskID     uuid.UUID `json:"task_id"`
	WorkerID   string    `json:"worker_id"`
	AcquiredAt time.Time `json:"acquired_at"`
	LastActive time.Time `json:"last_active"`
}

// WorkerStats represents statistics for a single worker
type WorkerStats struct {
	WorkerID            string        `json:"worker_id"`
	IsRunning           bool          `json:"is_running"`
	IsHealthy           bool          `json:"is_healthy"`
	TasksProcessed      int64         `json:"tasks_processed"`
	TasksSuccessful     int64         `json:"tasks_successful"`
	TasksFailed         int64         `json:"tasks_failed"`
	CurrentTask         *uuid.UUID    `json:"current_task,omitempty"`
	LastTaskStarted     *time.Time    `json:"last_task_started,omitempty"`
	LastTaskCompleted   *time.Time    `json:"last_task_completed,omitempty"`
	AverageTaskTime     time.Duration `json:"average_task_time"`
	TotalProcessingTime time.Duration `json:"total_processing_time"`
	StartedAt           time.Time     `json:"started_at"`
	LastHeartbeat       time.Time     `json:"last_heartbeat"`
}

// WorkerPoolStats represents statistics for a worker pool
type WorkerPoolStats struct {
	PoolSize             int           `json:"pool_size"`
	ActiveWorkers        int           `json:"active_workers"`
	IdleWorkers          int           `json:"idle_workers"`
	UnhealthyWorkers     int           `json:"unhealthy_workers"`
	TotalTasksProcessed  int64         `json:"total_tasks_processed"`
	TotalTasksSuccessful int64         `json:"total_tasks_successful"`
	TotalTasksFailed     int64         `json:"total_tasks_failed"`
	AverageTaskTime      time.Duration `json:"average_task_time"`
	TotalUptime          time.Duration `json:"total_uptime"`
	StartedAt            time.Time     `json:"started_at"`
	LastUpdated          time.Time     `json:"last_updated"`
}

// WorkerManagerStats represents comprehensive worker manager statistics
type WorkerManagerStats struct {
	IsRunning        bool             `json:"is_running"`
	IsHealthy        bool             `json:"is_healthy"`
	WorkerPoolStats  WorkerPoolStats  `json:"worker_pool_stats"`
	ConcurrencyStats ConcurrencyStats `json:"concurrency_stats"`
	ProcessingSlots  []ProcessingSlot `json:"processing_slots"`
	StartedAt        time.Time        `json:"started_at"`
	LastUpdated      time.Time        `json:"last_updated"`
}

// ConcurrencyLimits defines concurrency constraints
type ConcurrencyLimits struct {
	MaxConcurrentTasks     int `json:"max_concurrent_tasks"`
	MaxUserConcurrentTasks int `json:"max_user_concurrent_tasks"`
	MaxWorkers             int `json:"max_workers"`
	MinWorkers             int `json:"min_workers"`
}

// ConcurrencyStats represents concurrency statistics
type ConcurrencyStats struct {
	TotalActiveSlots       int            `json:"total_active_slots"`
	UserConcurrency        map[string]int `json:"user_concurrency"`
	AvailableSlots         int            `json:"available_slots"`
	MaxConcurrentTasks     int            `json:"max_concurrent_tasks"`
	MaxUserConcurrentTasks int            `json:"max_user_concurrent_tasks"`
	SlotsAcquiredTotal     int64          `json:"slots_acquired_total"`
	SlotsReleasedTotal     int64          `json:"slots_released_total"`
	AverageSlotDuration    time.Duration  `json:"average_slot_duration"`
	LastUpdated            time.Time      `json:"last_updated"`
}

// ProcessorType defines different types of task processors
type ProcessorType string

const (
	ProcessorTypeGeneral ProcessorType = "general"
	ProcessorTypePython  ProcessorType = "python"
	ProcessorTypeBash    ProcessorType = "bash"
	ProcessorTypeGo      ProcessorType = "go"
	ProcessorTypeJS      ProcessorType = "javascript"
)

// TaskExecutionRequest represents a task execution request
type TaskExecutionRequest struct {
	Message          *queue.TaskMessage         `json:"message"`
	ExecutionContext *executor.ExecutionContext `json:"execution_context"`
	ProcessingSlot   *ProcessingSlot            `json:"processing_slot"`
	StartedAt        time.Time                  `json:"started_at"`
	Timeout          time.Duration              `json:"timeout"`
}

// TaskExecutionResult represents the result of task execution
type TaskExecutionResult struct {
	Success         bool                      `json:"success"`
	ExecutionResult *executor.ExecutionResult `json:"execution_result,omitempty"`
	Error           error                     `json:"error,omitempty"`
	Duration        time.Duration             `json:"duration"`
	ProcessedBy     string                    `json:"processed_by"`
	CompletedAt     time.Time                 `json:"completed_at"`
}

// WorkerConfig represents configuration for workers
type WorkerConfig struct {
	WorkerIDPrefix       string        `json:"worker_id_prefix"`
	HeartbeatInterval    time.Duration `json:"heartbeat_interval"`
	TaskTimeout          time.Duration `json:"task_timeout"`
	HealthCheckInterval  time.Duration `json:"health_check_interval"`
	ShutdownTimeout      time.Duration `json:"shutdown_timeout"`
	MaxRetryAttempts     int           `json:"max_retry_attempts"`
	ProcessingSlotTTL    time.Duration `json:"processing_slot_ttl"`
	StaleTaskThreshold   time.Duration `json:"stale_task_threshold"`
	EnableAutoScaling    bool          `json:"enable_auto_scaling"`
	ScalingCheckInterval time.Duration `json:"scaling_check_interval"`
}

// WorkerError represents a worker-specific error
type WorkerError struct {
	WorkerID  string
	Operation string
	Err       error
	Retryable bool
}

func (e *WorkerError) Error() string {
	return e.Err.Error()
}

func (e *WorkerError) Unwrap() error {
	return e.Err
}

// NewWorkerError creates a new worker error
func NewWorkerError(workerID, operation string, err error, retryable bool) *WorkerError {
	return &WorkerError{
		WorkerID:  workerID,
		Operation: operation,
		Err:       err,
		Retryable: retryable,
	}
}

// Common worker errors
var (
	ErrWorkerNotRunning        = &WorkerError{Operation: "worker_state", Err: fmt.Errorf("worker not running"), Retryable: false}
	ErrWorkerAlreadyRunning    = &WorkerError{Operation: "worker_state", Err: fmt.Errorf("worker already running"), Retryable: false}
	ErrWorkerPoolClosed        = &WorkerError{Operation: "pool_state", Err: fmt.Errorf("worker pool is closed"), Retryable: false}
	ErrConcurrencyLimitReached = &WorkerError{Operation: "concurrency", Err: fmt.Errorf("concurrency limit reached"), Retryable: true}
	ErrInvalidTaskMessage      = &WorkerError{Operation: "task_validation", Err: fmt.Errorf("invalid task message"), Retryable: false}
	ErrTaskProcessingTimeout   = &WorkerError{Operation: "task_execution", Err: fmt.Errorf("task processing timeout"), Retryable: true}
	ErrWorkerUnhealthy         = &WorkerError{Operation: "health_check", Err: fmt.Errorf("worker is unhealthy"), Retryable: true}
)

// IsRetryableWorkerError checks if a worker error is retryable
func IsRetryableWorkerError(err error) bool {
	var workerErr *WorkerError
	if errors.As(err, &workerErr) {
		return workerErr.Retryable
	}
	return false
}

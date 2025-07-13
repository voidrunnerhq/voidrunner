package worker

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/voidrunnerhq/voidrunner/internal/database"
	"github.com/voidrunnerhq/voidrunner/internal/executor"
	"github.com/voidrunnerhq/voidrunner/internal/models"
	"github.com/voidrunnerhq/voidrunner/internal/queue"
)

// BaseTaskProcessor implements TaskProcessor for executing tasks
type BaseTaskProcessor struct {
	processorType ProcessorType
	executor      executor.TaskExecutor
	repos         *database.Repositories
	logger        *slog.Logger

	// Configuration
	timeout        time.Duration
	resourceLimits executor.ResourceLimits

	// Health tracking
	isHealthy       bool
	lastExecution   time.Time
	totalExecutions int64
	successfulExecs int64
	failedExecs     int64
}

// NewTaskProcessor creates a new task processor
func NewTaskProcessor(
	processorType ProcessorType,
	executor executor.TaskExecutor,
	repos *database.Repositories,
	timeout time.Duration,
	resourceLimits executor.ResourceLimits,
	logger *slog.Logger,
) TaskProcessor {
	return &BaseTaskProcessor{
		processorType:  processorType,
		executor:       executor,
		repos:          repos,
		timeout:        timeout,
		resourceLimits: resourceLimits,
		logger:         logger.With("processor_type", processorType),
		isHealthy:      true,
	}
}

// ProcessTask processes a single task message
func (p *BaseTaskProcessor) ProcessTask(ctx context.Context, message *queue.TaskMessage) error {
	startTime := time.Now()
	defer func() {
		p.lastExecution = time.Now()
		p.totalExecutions++
	}()

	p.logger.Info("processing task",
		"task_id", message.TaskID,
		"user_id", message.UserID,
		"attempt", message.Attempts)

	// Get task from database
	task, err := p.repos.Tasks.GetByID(ctx, message.TaskID)
	if err != nil {
		p.failedExecs++
		if err == database.ErrTaskNotFound {
			p.logger.Warn("task not found, skipping", "task_id", message.TaskID)
			return nil // Don't retry for non-existent tasks
		}
		return fmt.Errorf("failed to get task: %w", err)
	}

	// Validate that this processor can handle the task
	if !p.canProcessTask(task) {
		p.failedExecs++
		return fmt.Errorf("processor %s cannot handle task type %s", p.processorType, task.ScriptType)
	}

	// Create execution record
	execution, err := p.createExecution(ctx, task)
	if err != nil {
		p.failedExecs++
		return fmt.Errorf("failed to create execution: %w", err)
	}

	// Update task status to running
	if err := p.repos.Tasks.UpdateStatus(ctx, task.ID, models.TaskStatusRunning); err != nil {
		p.failedExecs++
		return fmt.Errorf("failed to update task status: %w", err)
	}

	// Execute the task
	result, execErr := p.executeTask(ctx, task, execution)

	// Process the result
	if err := p.processResult(ctx, task, execution, result, execErr); err != nil {
		p.logger.Error("failed to process execution result", "error", err, "task_id", task.ID)
		// Don't return error here as the task was executed
	}

	// Update statistics
	if execErr == nil && result != nil && result.Status == models.ExecutionStatusCompleted {
		p.successfulExecs++
	} else {
		p.failedExecs++
	}

	duration := time.Since(startTime)
	p.logger.Info("task processing completed",
		"task_id", task.ID,
		"duration", duration,
		"success", execErr == nil)

	return execErr
}

// CanProcessTask checks if the processor can handle the given task
func (p *BaseTaskProcessor) CanProcessTask(message *queue.TaskMessage) bool {
	// For general processor, we need to check the actual task
	if p.processorType == ProcessorTypeGeneral {
		return true
	}

	// Get task to check script type
	task, err := p.repos.Tasks.GetByID(context.Background(), message.TaskID)
	if err != nil {
		p.logger.Error("failed to get task for compatibility check", "error", err, "task_id", message.TaskID)
		return false
	}

	return p.canProcessTask(task)
}

// GetProcessorType returns the type of processor
func (p *BaseTaskProcessor) GetProcessorType() ProcessorType {
	return p.processorType
}

// IsHealthy checks if the processor is healthy
func (p *BaseTaskProcessor) IsHealthy() bool {
	// Check executor health
	if err := p.executor.IsHealthy(context.Background()); err != nil {
		p.isHealthy = false
		return false
	}

	p.isHealthy = true
	return true
}

// canProcessTask checks if this processor can handle the task type
func (p *BaseTaskProcessor) canProcessTask(task *models.Task) bool {
	switch p.processorType {
	case ProcessorTypeGeneral:
		return true // General processor handles all types
	case ProcessorTypePython:
		return task.ScriptType == models.ScriptTypePython
	case ProcessorTypeBash:
		return task.ScriptType == models.ScriptTypeBash
	case ProcessorTypeGo:
		return task.ScriptType == models.ScriptTypeGo
	case ProcessorTypeJS:
		return task.ScriptType == models.ScriptTypeJavaScript
	default:
		return false
	}
}

// createExecution creates a new task execution record
func (p *BaseTaskProcessor) createExecution(ctx context.Context, task *models.Task) (*models.TaskExecution, error) {
	execution := &models.TaskExecution{
		ID:        models.NewID(),
		TaskID:    task.ID,
		Status:    models.ExecutionStatusPending,
		StartedAt: new(time.Time),
	}
	*execution.StartedAt = time.Now()

	if err := p.repos.TaskExecutions.Create(ctx, execution); err != nil {
		return nil, fmt.Errorf("failed to create execution record: %w", err)
	}

	return execution, nil
}

// executeTask executes the task using the executor
func (p *BaseTaskProcessor) executeTask(
	ctx context.Context,
	task *models.Task,
	execution *models.TaskExecution,
) (*executor.ExecutionResult, error) {
	// Create execution context with timeout
	execCtx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	// Create executor context
	executorCtx := &executor.ExecutionContext{
		Task:           task,
		Execution:      execution,
		Context:        execCtx,
		Timeout:        p.timeout,
		ResourceLimits: p.resourceLimits,
	}

	p.logger.Debug("executing task",
		"task_id", task.ID,
		"script_type", task.ScriptType,
		"timeout", p.timeout)

	// Execute the task
	result, err := p.executor.Execute(execCtx, executorCtx)
	if err != nil {
		p.logger.Error("task execution failed",
			"task_id", task.ID,
			"error", err)
		return nil, fmt.Errorf("execution failed: %w", err)
	}

	p.logger.Debug("task execution completed",
		"task_id", task.ID,
		"status", result.Status,
		"execution_time_ms", result.ExecutionTimeMs)

	return result, nil
}

// processResult processes the execution result and updates database records
func (p *BaseTaskProcessor) processResult(
	ctx context.Context,
	task *models.Task,
	execution *models.TaskExecution,
	result *executor.ExecutionResult,
	execErr error,
) error {
	now := time.Now()

	if execErr != nil {
		// Execution failed - update execution record
		execution.Status = models.ExecutionStatusFailed
		execution.CompletedAt = &now
		stderr := execErr.Error()
		execution.Stderr = &stderr

		// Update execution in database
		if err := p.repos.TaskExecutions.Update(ctx, execution); err != nil {
			p.logger.Error("failed to update failed execution", "error", err)
		}

		// Update task status
		if err := p.repos.Tasks.UpdateStatus(ctx, task.ID, models.TaskStatusFailed); err != nil {
			p.logger.Error("failed to update task status to failed", "error", err)
		}

		return nil
	}

	// Execution succeeded - update with results
	execution.Status = result.Status
	execution.ReturnCode = result.ReturnCode
	execution.Stdout = result.Stdout
	execution.Stderr = result.Stderr
	execution.ExecutionTimeMs = result.ExecutionTimeMs
	execution.MemoryUsageBytes = result.MemoryUsageBytes
	execution.CompletedAt = &now

	// Update execution in database
	if err := p.repos.TaskExecutions.Update(ctx, execution); err != nil {
		return fmt.Errorf("failed to update execution: %w", err)
	}

	// Determine task status based on execution result
	var taskStatus models.TaskStatus
	switch result.Status {
	case models.ExecutionStatusCompleted:
		taskStatus = models.TaskStatusCompleted
	case models.ExecutionStatusFailed:
		taskStatus = models.TaskStatusFailed
	case models.ExecutionStatusTimeout:
		taskStatus = models.TaskStatusTimeout
	case models.ExecutionStatusCancelled:
		taskStatus = models.TaskStatusCancelled
	default:
		taskStatus = models.TaskStatusFailed
	}

	// Update task status
	if err := p.repos.Tasks.UpdateStatus(ctx, task.ID, taskStatus); err != nil {
		return fmt.Errorf("failed to update task status: %w", err)
	}

	return nil
}

// GetStats returns processor statistics
func (p *BaseTaskProcessor) GetStats() TaskProcessorStats {
	successRate := float64(0)
	if p.totalExecutions > 0 {
		successRate = float64(p.successfulExecs) / float64(p.totalExecutions) * 100
	}

	return TaskProcessorStats{
		ProcessorType:        p.processorType,
		IsHealthy:            p.isHealthy,
		TotalExecutions:      p.totalExecutions,
		SuccessfulExecutions: p.successfulExecs,
		FailedExecutions:     p.failedExecs,
		SuccessRate:          successRate,
		LastExecution:        p.lastExecution,
	}
}

// TaskProcessorStats represents statistics for a task processor
type TaskProcessorStats struct {
	ProcessorType        ProcessorType `json:"processor_type"`
	IsHealthy            bool          `json:"is_healthy"`
	TotalExecutions      int64         `json:"total_executions"`
	SuccessfulExecutions int64         `json:"successful_executions"`
	FailedExecutions     int64         `json:"failed_executions"`
	SuccessRate          float64       `json:"success_rate"`
	LastExecution        time.Time     `json:"last_execution"`
}

// ProcessorRegistry manages multiple task processors
type ProcessorRegistry struct {
	processors map[ProcessorType]TaskProcessor
	logger     *slog.Logger
}

// NewProcessorRegistry creates a new processor registry
func NewProcessorRegistry(logger *slog.Logger) *ProcessorRegistry {
	return &ProcessorRegistry{
		processors: make(map[ProcessorType]TaskProcessor),
		logger:     logger,
	}
}

// RegisterProcessor registers a processor for a specific type
func (r *ProcessorRegistry) RegisterProcessor(processorType ProcessorType, processor TaskProcessor) {
	r.processors[processorType] = processor
	r.logger.Info("processor registered", "type", processorType)
}

// GetProcessor gets a processor for the given message
func (r *ProcessorRegistry) GetProcessor(message *queue.TaskMessage) (TaskProcessor, error) {
	// First try to find a specific processor
	for _, processor := range r.processors {
		if processor.CanProcessTask(message) && processor.GetProcessorType() != ProcessorTypeGeneral {
			return processor, nil
		}
	}

	// Fall back to general processor
	if generalProcessor, exists := r.processors[ProcessorTypeGeneral]; exists {
		return generalProcessor, nil
	}

	return nil, fmt.Errorf("no suitable processor found for task %s", message.TaskID)
}

// GetAllProcessors returns all registered processors
func (r *ProcessorRegistry) GetAllProcessors() map[ProcessorType]TaskProcessor {
	result := make(map[ProcessorType]TaskProcessor)
	for k, v := range r.processors {
		result[k] = v
	}
	return result
}

// IsHealthy checks if all processors are healthy
func (r *ProcessorRegistry) IsHealthy() bool {
	for _, processor := range r.processors {
		if !processor.IsHealthy() {
			return false
		}
	}
	return true
}

// GetProcessorForScriptType returns the best processor for a script type
func GetProcessorTypeForScriptType(scriptType models.ScriptType) ProcessorType {
	switch scriptType {
	case models.ScriptTypePython:
		return ProcessorTypePython
	case models.ScriptTypeBash:
		return ProcessorTypeBash
	case models.ScriptTypeGo:
		return ProcessorTypeGo
	case models.ScriptTypeJavaScript:
		return ProcessorTypeJS
	default:
		return ProcessorTypeGeneral
	}
}

// ValidateProcessorType checks if a processor type is valid
func ValidateProcessorType(processorType ProcessorType) bool {
	validTypes := []ProcessorType{
		ProcessorTypeGeneral,
		ProcessorTypePython,
		ProcessorTypeBash,
		ProcessorTypeGo,
		ProcessorTypeJS,
	}

	for _, validType := range validTypes {
		if processorType == validType {
			return true
		}
	}
	return false
}

// ProcessorTypeFromString converts a string to ProcessorType
func ProcessorTypeFromString(s string) (ProcessorType, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "general":
		return ProcessorTypeGeneral, nil
	case "python":
		return ProcessorTypePython, nil
	case "bash":
		return ProcessorTypeBash, nil
	case "go":
		return ProcessorTypeGo, nil
	case "javascript", "js":
		return ProcessorTypeJS, nil
	default:
		return "", fmt.Errorf("unknown processor type: %s", s)
	}
}

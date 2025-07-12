package executor

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/voidrunnerhq/voidrunner/internal/models"
)

// Executor implements the TaskExecutor interface
type Executor struct {
	client          ContainerClient
	config          *Config
	securityManager *SecurityManager
	cleanupManager  *CleanupManager
	logger          *slog.Logger
}

// NewExecutor creates a new executor with the given configuration
func NewExecutor(config *Config, logger *slog.Logger) (*Executor, error) {
	if config == nil {
		config = NewDefaultConfig()
	}

	if logger == nil {
		logger = slog.Default()
	}

	// Create Docker client
	dockerClient, err := NewDockerClient(config, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	// Create security manager
	securityManager := NewSecurityManager(config)

	// Create cleanup manager
	cleanupManager := NewCleanupManager(dockerClient, logger)

	executor := &Executor{
		client:          dockerClient,
		config:          config,
		securityManager: securityManager,
		cleanupManager:  cleanupManager,
		logger:          logger,
	}

	return executor, nil
}

// Execute runs the given task and returns the execution result
func (e *Executor) Execute(ctx context.Context, execCtx *ExecutionContext) (*ExecutionResult, error) {
	if execCtx == nil || execCtx.Task == nil {
		return nil, NewExecutorError("execute", "execution context or task is nil", nil)
	}

	task := execCtx.Task
	logger := e.logger.With(
		"task_id", task.ID.String(),
		"script_type", string(task.ScriptType),
		"operation", "execute",
	)

	logger.Info("starting task execution")

	// Validate script content for security
	if err := e.securityManager.ValidateScriptContent(task.ScriptContent, task.ScriptType); err != nil {
		logger.Error("script security validation failed", "error", err)
		return &ExecutionResult{
			Status: models.ExecutionStatusFailed,
			Stderr: stringPtr(fmt.Sprintf("Security validation failed: %s", err.Error())),
		}, err
	}

	// Build container configuration
	containerConfig, err := e.buildContainerConfig(task, execCtx.ResourceLimits, execCtx.Timeout)
	if err != nil {
		logger.Error("failed to build container configuration", "error", err)
		return &ExecutionResult{
			Status: models.ExecutionStatusFailed,
			Stderr: stringPtr(fmt.Sprintf("Configuration error: %s", err.Error())),
		}, err
	}

	// Validate container configuration
	if err := e.securityManager.ValidateContainerConfig(containerConfig); err != nil {
		logger.Error("container configuration validation failed", "error", err)
		return &ExecutionResult{
			Status: models.ExecutionStatusFailed,
			Stderr: stringPtr(fmt.Sprintf("Security validation failed: %s", err.Error())),
		}, err
	}

	// Create execution context with timeout
	execTimeout := execCtx.Timeout
	if execTimeout == 0 {
		execTimeout = e.config.GetTimeoutForTask(task)
	}

	ctxWithTimeout, cancel := context.WithTimeout(ctx, execTimeout)
	defer cancel()

	// Execute the container
	result, err := e.executeContainer(ctxWithTimeout, containerConfig, execCtx, logger)
	if err != nil {
		logger.Error("container execution failed", "error", err)
		if result == nil {
			return &ExecutionResult{
				Status: models.ExecutionStatusFailed,
				Stderr: stringPtr(fmt.Sprintf("Execution error: %s", err.Error())),
			}, err
		}
	}

	logger.Info("task execution completed",
		"status", result.Status,
		"duration_ms", result.ExecutionTimeMs,
		"return_code", result.ReturnCode)

	return result, err
}

// executeContainer executes a single container and returns the result
func (e *Executor) executeContainer(ctx context.Context, config *ContainerConfig, execCtx *ExecutionContext, logger *slog.Logger) (*ExecutionResult, error) {
	startTime := time.Now()

	result := &ExecutionResult{
		Status:    models.ExecutionStatusRunning,
		StartedAt: &startTime,
	}

	// Create container
	logger.Debug("creating container", "image", config.Image)
	containerID, err := e.client.CreateContainer(ctx, config)
	if err != nil {
		result.Status = models.ExecutionStatusFailed
		return result, NewExecutorError("execute_container", "failed to create container", err)
	}

	logger = logger.With("container_id", containerID[:12]) // Short container ID for logging

	// Register container with cleanup manager for tracking
	if err := e.cleanupManager.RegisterContainer(containerID, execCtx.Task.ID, execCtx.Execution.ID, config.Image); err != nil {
		logger.Error("failed to register container for tracking", "error", err)
		// Continue execution but log the error - cleanup tracking is not critical for execution
	}

	// Ensure container cleanup
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()

		if err := e.client.RemoveContainer(cleanupCtx, containerID, true); err != nil {
			logger.Error("failed to cleanup container", "error", err)
		}
	}()

	// Start container
	logger.Debug("starting container")
	if err := e.client.StartContainer(ctx, containerID); err != nil {
		result.Status = models.ExecutionStatusFailed
		return result, NewExecutorError("execute_container", "failed to start container", err)
	}

	// Mark container as started
	e.cleanupManager.MarkContainerStarted(containerID)

	// Wait for container to finish
	logger.Debug("waiting for container to complete")
	exitCode, err := e.client.WaitContainer(ctx, containerID)

	endTime := time.Now()
	result.CompletedAt = &endTime
	duration := int(endTime.Sub(startTime).Milliseconds())
	result.ExecutionTimeMs = &duration

	if err != nil {
		if IsTimeoutError(err) || ctx.Err() == context.DeadlineExceeded {
			result.Status = models.ExecutionStatusTimeout
			logger.Warn("container execution timed out")
		} else if IsCancelledError(err) || ctx.Err() == context.Canceled {
			result.Status = models.ExecutionStatusCancelled
			logger.Info("container execution cancelled")
		} else {
			result.Status = models.ExecutionStatusFailed
			logger.Error("container execution failed", "error", err)
		}
	} else {
		result.ReturnCode = &exitCode
		if exitCode == 0 {
			result.Status = models.ExecutionStatusCompleted
		} else {
			result.Status = models.ExecutionStatusFailed
		}
	}

	// Get container logs
	logger.Debug("retrieving container logs")
	stdout, stderr, logErr := e.client.GetContainerLogs(ctx, containerID)
	if logErr != nil {
		logger.Error("failed to get container logs", "error", logErr)
		// Don't fail the execution just because we couldn't get logs
		stderr = fmt.Sprintf("Failed to retrieve logs: %s", logErr.Error())
	}

	if stdout != "" {
		result.Stdout = &stdout
	}
	if stderr != "" {
		result.Stderr = &stderr
	}

	// Mark container as completed with final status
	e.cleanupManager.MarkContainerCompleted(containerID, string(result.Status))

	return result, err
}

// buildContainerConfig creates a container configuration for the given task
func (e *Executor) buildContainerConfig(task *models.Task, resourceLimits ResourceLimits, timeout time.Duration) (*ContainerConfig, error) {
	// Get appropriate image for script type
	image := e.config.GetImageForScriptType(task.ScriptType)

	// Validate image security
	if err := e.securityManager.CheckImageSecurity(image); err != nil {
		return nil, fmt.Errorf("image security check failed: %w", err)
	}

	// Get resource limits (use provided or get from config)
	if resourceLimits.MemoryLimitBytes == 0 {
		resourceLimits = e.config.GetResourceLimitsForTask(task)
	}

	// Get security configuration
	securityConfig := e.config.GetSecurityConfigForTask(task)

	// Get execution timeout
	if timeout == 0 {
		timeout = e.config.GetTimeoutForTask(task)
	}

	// Sanitize environment variables
	environment := e.securityManager.SanitizeEnvironment([]string{
		"PATH=/usr/local/bin:/usr/bin:/bin",
		"HOME=/tmp",
		"USER=executor",
		"PYTHONIOENCODING=utf-8",
	})

	config := &ContainerConfig{
		Image:          image,
		ScriptType:     task.ScriptType,
		ScriptContent:  task.ScriptContent,
		Environment:    environment,
		WorkingDir:     "/tmp/workspace",
		ResourceLimits: resourceLimits,
		SecurityConfig: securityConfig,
		Timeout:        timeout,
	}

	return config, nil
}

// Cancel cancels a running execution
func (e *Executor) Cancel(ctx context.Context, executionID uuid.UUID) error {
	logger := e.logger.With("execution_id", executionID.String(), "operation", "cancel")
	logger.Info("execution cancellation requested")

	// Use cleanup manager to cancel all containers for this execution
	if err := e.cleanupManager.CleanupExecution(ctx, executionID); err != nil {
		logger.Error("failed to cleanup execution containers", "error", err)
		return NewExecutorError("cancel", "failed to cancel execution containers", err)
	}

	logger.Info("execution cancellation completed")
	return nil
}

// IsHealthy checks if the executor is healthy and ready to execute tasks
func (e *Executor) IsHealthy(ctx context.Context) error {
	// Check Docker client health
	if err := e.client.IsHealthy(ctx); err != nil {
		return fmt.Errorf("Docker client health check failed: %w", err)
	}

	// Check if required images are available
	requiredImages := []string{
		e.config.Images.Python,
		e.config.Images.Bash,
	}

	for _, image := range requiredImages {
		if err := e.ensureImageAvailable(ctx, image); err != nil {
			e.logger.Warn("image not available, will be pulled on demand",
				"image", image, "error", err)
		}
	}

	// Validate configuration
	if err := e.config.Validate(); err != nil {
		return fmt.Errorf("configuration validation failed: %w", err)
	}

	return nil
}

// ensureImageAvailable checks if an image is available locally
func (e *Executor) ensureImageAvailable(ctx context.Context, image string) error {
	// For now, we'll rely on Docker to pull images as needed
	// In a production environment, we might want to pre-pull critical images
	return nil
}

// Cleanup performs any necessary cleanup of resources
func (e *Executor) Cleanup(ctx context.Context) error {
	e.logger.Info("cleaning up executor resources")

	// Stop cleanup manager and cleanup all containers
	if err := e.cleanupManager.Stop(ctx); err != nil {
		e.logger.Error("failed to stop cleanup manager", "error", err)
	}

	// Close Docker client if it implements Close()
	if closer, ok := e.client.(interface{ Close() error }); ok {
		if err := closer.Close(); err != nil {
			e.logger.Error("failed to close Docker client", "error", err)
			return err
		}
	}

	return nil
}

// GetExecutorInfo returns information about the executor
func (e *Executor) GetExecutorInfo(ctx context.Context) (*ExecutorInfo, error) {
	dockerInfo, err := e.client.(interface {
		GetDockerInfo(context.Context) (interface{}, error)
	}).GetDockerInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get Docker info: %w", err)
	}

	return &ExecutorInfo{
		Version:    "1.0.0",
		DockerInfo: dockerInfo,
		Config:     e.config,
		IsHealthy:  e.IsHealthy(ctx) == nil,
	}, nil
}

// ExecutorInfo contains information about the executor
type ExecutorInfo struct {
	Version    string      `json:"version"`
	DockerInfo interface{} `json:"docker_info"`
	Config     *Config     `json:"config"`
	IsHealthy  bool        `json:"is_healthy"`
}

// Helper function to create string pointer
func stringPtr(s string) *string {
	return &s
}

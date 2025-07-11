package services

import (
	"context"
	"fmt"
	"log/slog"

	"time"

	"github.com/google/uuid"
	"github.com/voidrunnerhq/voidrunner/internal/config" // Import app config
	"github.com/voidrunnerhq/voidrunner/internal/database"
	"github.com/voidrunnerhq/voidrunner/internal/docker" // Import the new Docker package
	"github.com/voidrunnerhq/voidrunner/internal/models"
)

// TaskExecutionService handles business logic for task execution operations
type TaskExecutionService struct {
	conn         *database.Connection
	dockerClient *docker.Client // Add Docker client
	logger       *slog.Logger
	appConfig    *config.Config // To access Docker image names, resource limits etc.
}

// NewTaskExecutionService creates a new task execution service
func NewTaskExecutionService(
	conn *database.Connection,
	dockerClient *docker.Client,
	logger *slog.Logger,
	appConfig *config.Config,
) *TaskExecutionService {
	return &TaskExecutionService{
		conn:         conn,
		dockerClient: dockerClient,
		logger:       logger,
		appConfig:    appConfig,
	}
}

// executeTaskInContainer is run in a goroutine to perform the actual Docker execution.
func (s *TaskExecutionService) executeTaskInContainer(
	parentCtx context.Context, // Use a background context or one derived from app lifecycle
	task models.Task,
	execution models.TaskExecution,
) {
	// Use the default execution timeout from config for the overall Docker operation context
	execCtx, cancel := context.WithTimeout(parentCtx, s.appConfig.Docker.DefaultExecTimeout)
	defer cancel()

	s.logger.Info("starting container execution", "task_id", task.ID, "execution_id", execution.ID)

	var imageName string
	var cmd []string

	// Determine image and command based on task.Language (assuming task model has Language field)
	// For now, assuming task.Language is a string like "python", "bash"
	// This part needs the Task model to have a "Language" field.
	// If task.Language is not present, this will need adjustment.
	// Let's assume task.Language exists for this implementation.

	// TODO: The `task` model currently doesn't have a `Language` field.
	// For now, I'll default to Python as in previous steps, but this is a key area for future enhancement.
	// If Task model is updated, the following switch can be used:
	/*
	switch task.Language { // Assuming task.Language exists
	case "python":
		imageName = s.appConfig.Docker.PythonExecutorImage
		cmd = []string{"python3", "-c", "%CODE%"}
	case "bash":
		imageName = s.appConfig.Docker.BashExecutorImage
		cmd = []string{"/bin/sh", "-c", "%CODE%"} // Using /bin/sh for Alpine
	default:
		errMsg := fmt.Sprintf("unsupported language: %s", task.Language)
		s.logger.Error("unsupported language for execution", "task_id", task.ID, "execution_id", execution.ID, "language", task.Language)
		s.updateFailedExecution(parentCtx, execution.ID, task.ID, task.UserID, errMsg, "UnsupportedLanguage")
		return
	}
	*/

	// Defaulting to Python as task.Language is not yet in the model
	imageName = s.appConfig.Docker.PythonExecutorImage
	cmd = []string{"python3", "-c", "%CODE%"} // Placeholder for user code
	userCode := task.ScriptContent             // Use ScriptContent field from model

	params := &docker.ExecuteContainerParams{
		Ctx:            execCtx,
		ImageName:      imageName,
		Cmd:            cmd,
		UserCode:       userCode,
		User:           "1000:1000", // Standard non-root user
		WorkingDir:     "/tmp/workspace",
		EnvVars:        []string{"HOME=/tmp"},
		MemoryMB:       s.appConfig.Docker.DefaultMemoryMB,
		CPUQuota:       s.appConfig.Docker.DefaultCPUQuota,
		PidsLimit:      s.appConfig.Docker.DefaultPidsLimit,
		NetworkMode:    "none",
		ReadOnlyRootfs: true,
		Tmpfs:          map[string]string{"/tmp": "rw,noexec,nosuid,size=100m"}, // Standard tmpfs for workspace
		AutoRemove:     true,
		SeccompProfile: s.appConfig.Docker.SeccompProfilePath,
	}

	containerID, err := s.dockerClient.CreateAndStartContainer(params)
	if err != nil {
		s.logger.Error("failed to create or start container", "task_id", task.ID, "execution_id", execution.ID, "error", err)
		errMsg := err.Error()
		execution.Status = models.ExecutionStatusFailed
		execution.Stderr = &errMsg
		returnCode := -1
		execution.ReturnCode = &returnCode
		// This needs to be an atomic update, similar to CompleteExecutionAndFinalizeTaskStatus
		// but requires careful handling of context and potential user_id if service methods enforce it.
		// For now, logging the failure. Robust status update needed.
		// Consider a dedicated method: s.finalizeExecution(ctx, executionID, taskID, models.ExecutionStatusFailed, models.TaskStatusFailed, output, error)
		s.updateFailedExecution(parentCtx, execution.ID, task.ID, task.UserID, errMsg, "ContainerStartError")
		return
	}

	s.logger.Info("container started for execution", "task_id", task.ID, "execution_id", execution.ID, "container_id", containerID)

	exitCode, waitErr := s.dockerClient.WaitForContainer(execCtx, containerID)
	// Log wait error, but proceed to get logs if possible, as container might have run but wait failed (e.g. context timeout)
	if waitErr != nil {
		s.logger.Warn("error waiting for container, will attempt to get logs", "task_id", task.ID, "execution_id", execution.ID, "container_id", containerID, "error", waitErr)
		// If context timed out, exitCode might be unreliable or -1.
		// The actual execution might have finished or might still be running.
		// If it's still running, logs might be incomplete.
	}

	// Use a separate context for log retrieval, possibly shorter timeout, or background if main execCtx timed out.
	logCtx, logCancel := context.WithTimeout(parentCtx, 30*time.Second) // Shorter timeout for logs
	defer logCancel()
	stdout, stderr, logErr := s.dockerClient.GetContainerLogs(logCtx, containerID)
	if logErr != nil {
		s.logger.Error("failed to get container logs", "task_id", task.ID, "execution_id", execution.ID, "container_id", containerID, "error", logErr)
		// Append to stderr if it's empty or add a note about log failure.
		if stderr == "" {
			stderr = fmt.Sprintf("Failed to retrieve logs: %v", logErr)
		} else {
			stderr = fmt.Sprintf("%s\n\nFailed to retrieve logs: %v", stderr, logErr)
		}
	}
	
	// AutoRemove is true, so container should be gone. If not, try to remove explicitly.
	// This is a fallback, not strictly necessary if AutoRemove works reliably.
	// if !params.AutoRemove { // Or if AutoRemove failed for some reason
	// 	rmCtx, rmCancel := context.WithTimeout(parentCtx, 10*time.Second)
	// 	defer rmCancel()
	// 	if err := s.dockerClient.RemoveContainer(rmCtx, containerID, true, true); err != nil {
	// 		s.logger.Warn("failed to remove container after execution", "container_id", containerID, "error", err)
	// 	}
	// }

	// Determine final execution status
	execution.Stdout = &stdout
	execution.Stderr = &stderr
	exitCodeInt := int(exitCode)
	execution.ReturnCode = &exitCodeInt

	finalTaskStatus := models.TaskStatusCompleted
	finalExecStatus := models.ExecutionStatusCompleted

	if waitErr != nil { // If waiting for container failed (e.g. timeout)
		finalTaskStatus = models.TaskStatusFailed
		finalExecStatus = models.ExecutionStatusFailed
		errMsg := fmt.Sprintf("Execution failed or timed out: %v", waitErr)
		if execution.Stderr == nil {
			execution.Stderr = &errMsg
		} else {
			appendErr := fmt.Sprintf("%s; %s", *execution.Stderr, errMsg)
			execution.Stderr = &appendErr
		}
		if exitCode == -1 && execCtx.Err() == context.DeadlineExceeded { // Common case for timeout
			timeoutExitCode := 137 // Simulate SIGKILL due to timeout (common convention)
			execution.ReturnCode = &timeoutExitCode
			timeoutMsg := "Execution timed out."
			if execution.Stderr == nil || *execution.Stderr == "" {
				execution.Stderr = &timeoutMsg
			} else {
				newStderr := fmt.Sprintf("%s\n%s", *execution.Stderr, timeoutMsg)
				execution.Stderr = &newStderr
			}
		}
	} else if exitCode != 0 { // Container ran but exited with non-zero status
		finalTaskStatus = models.TaskStatusFailed
		finalExecStatus = models.ExecutionStatusFailed
		errMsg := fmt.Sprintf("Container exited with code %d", exitCode)
		if execution.Stderr == nil {
			execution.Stderr = &errMsg
		} else {
			appendErr := fmt.Sprintf("%s; %s", *execution.Stderr, errMsg)
			execution.Stderr = &appendErr
		}
	}

	execution.Status = finalExecStatus
	now := time.Now().UTC()
	execution.CompletedAt = &now


	// Atomically update the execution record and the parent task's status.
	// Pass task.UserID for permission checks in CompleteExecutionAndFinalizeTaskStatus
	err = s.CompleteExecutionAndFinalizeTaskStatus(parentCtx, &execution, finalTaskStatus, task.UserID)
	if err != nil {
		s.logger.Error("failed to finalize execution and task status in database",
			"task_id", task.ID, "execution_id", execution.ID, "container_id", containerID, "error", err)
		// This is a critical error. The execution happened, but DB state is inconsistent.
		// May need a retry mechanism or a background job to fix such inconsistencies.
	} else {
		s.logger.Info("container execution finished and status updated",
			"task_id", task.ID, "execution_id", execution.ID, "container_id", containerID,
			"exit_code", exitCode, "final_task_status", finalTaskStatus, "final_exec_status", finalExecStatus)
	}
}


// CreateExecutionAndUpdateTaskStatus atomically creates a task execution and updates the task status.
// It now also triggers the asynchronous container execution.
func (s *TaskExecutionService) CreateExecutionAndUpdateTaskStatus(ctx context.Context, taskID uuid.UUID, userID uuid.UUID) (*models.TaskExecution, error) {
	var execution *models.TaskExecution
	var taskDetails models.Task

	err := s.conn.WithTransaction(ctx, func(tx database.Transaction) error {
		repos := tx.Repositories()

		// First, verify the task exists and belongs to the user
		task, err := repos.Tasks.GetByID(ctx, taskID)
		if err != nil {
			if err == database.ErrTaskNotFound {
				return fmt.Errorf("task not found")
			}
			return fmt.Errorf("failed to get task: %w", err)
		}
		taskDetails = *task // Store for goroutine

		// Check if user owns the task
		if task.UserID != userID {
			return fmt.Errorf("access denied: task does not belong to user")
		}

		// Check if task is already running
		if task.Status == models.TaskStatusRunning {
			return fmt.Errorf("task is already running")
		}

		// Check if task can be executed (not completed, failed, or cancelled)
		if task.Status == models.TaskStatusCompleted ||
			task.Status == models.TaskStatusFailed ||
			task.Status == models.TaskStatusCancelled {
			return fmt.Errorf("cannot execute task with status: %s", task.Status)
		}

		// Create task execution
		execution = &models.TaskExecution{
			ID:     uuid.New(),
			TaskID: taskID,
			Status: models.ExecutionStatusPending, // Will be updated to Running by executeTaskInContainer or similar
		}

		if err := repos.TaskExecutions.Create(ctx, execution); err != nil {
			return fmt.Errorf("failed to create task execution: %w", err)
		}

		// Update task status to running
		if err := repos.Tasks.UpdateStatus(ctx, taskID, models.TaskStatusRunning); err != nil {
			return fmt.Errorf("failed to update task status: %w", err)
		}

		s.logger.Info("task execution record created and task status updated to running",
			"execution_id", execution.ID,
			"task_id", taskID,
			"user_id", userID,
		)

		return nil
	})

	if err != nil {
		s.logger.Error("failed to create execution record and update task status",
			"error", err,
			"task_id", taskID,
			"user_id", userID,
		)
		return nil, err
	}

	// Launch the actual container execution in a goroutine
	// Use context.Background() for the goroutine as the original request context (ctx) may be cancelled.
	// The executeTaskInContainer method should manage its own timeouts.
	go s.executeTaskInContainer(context.Background(), taskDetails, *execution)

	s.logger.Info("dispatched task for container execution", "task_id", taskDetails.ID, "execution_id", execution.ID)

	return execution, nil
}


// updateFailedExecution is a helper to quickly update status in case of pre-execution failures (e.g., container start)
func (s *TaskExecutionService) updateFailedExecution(
	ctx context.Context,
	executionID uuid.UUID,
	taskID uuid.UUID,
	userID uuid.UUID, // UserID is needed if CompleteExecutionAndFinalizeTaskStatus requires it for auth
	errorMessage string,
	errorType string, // e.g., "ContainerStartError", "UnsupportedLanguage"
) {
	s.logger.Error("updating failed execution", "execution_id", executionID, "task_id", taskID, "error_type", errorType, "message", errorMessage)

	now := time.Now().UTC()
	returnCode := -1 // Generic error exit code

	failedExecution := models.TaskExecution{
		ID:          executionID,
		TaskID:      taskID,
		Status:      models.ExecutionStatusFailed,
		Stderr:      &errorMessage,
		ReturnCode:  &returnCode,
		CompletedAt: &now,
		// Stdout/Stderr might be empty or have partial data if applicable
	}

	// Use the existing robust method for updating, ensuring atomicity
	err := s.CompleteExecutionAndFinalizeTaskStatus(ctx, &failedExecution, models.TaskStatusFailed, userID)
	if err != nil {
		s.logger.Error("CRITICAL: failed to update database for a failed execution",
			"execution_id", executionID,
			"task_id", taskID,
			"original_error", errorMessage,
			"db_update_error", err.Error(),
		)
		// This situation (execution failed, and DB update also failed) is problematic
		// and might require manual intervention or a reconciliation process.
	}
}


// UpdateExecutionAndTaskStatus atomically updates both execution and task status
// This method is typically called by an admin or a system process that has results,
// not directly part of the initial execution flow triggered by a user.
// The primary flow now uses executeTaskInContainer which calls CompleteExecutionAndFinalizeTaskStatus.
func (s *TaskExecutionService) UpdateExecutionAndTaskStatus(ctx context.Context, executionID uuid.UUID, executionStatus models.ExecutionStatus, taskID uuid.UUID, taskStatus models.TaskStatus, userID uuid.UUID) error {
	err := s.conn.WithTransaction(ctx, func(tx database.Transaction) error {
		repos := tx.Repositories()

		// First, verify the execution exists and belongs to the user's task
		execution, err := repos.TaskExecutions.GetByID(ctx, executionID)
		if err != nil {
			if err == database.ErrExecutionNotFound {
				return fmt.Errorf("execution not found")
			}
			return fmt.Errorf("failed to get execution: %w", err)
		}

		// Verify the task belongs to the user
		task, err := repos.Tasks.GetByID(ctx, execution.TaskID)
		if err != nil {
			return fmt.Errorf("failed to get task: %w", err)
		}

		if task.UserID != userID {
			return fmt.Errorf("access denied: task does not belong to user")
		}

		// Verify the task ID matches
		if execution.TaskID != taskID {
			return fmt.Errorf("execution does not belong to the specified task")
		}

		// Update execution status
		if err := repos.TaskExecutions.UpdateStatus(ctx, executionID, executionStatus); err != nil {
			return fmt.Errorf("failed to update execution status: %w", err)
		}

		// Update task status
		if err := repos.Tasks.UpdateStatus(ctx, taskID, taskStatus); err != nil {
			return fmt.Errorf("failed to update task status: %w", err)
		}

		s.logger.Info("execution and task status updated atomically",
			"execution_id", executionID,
			"execution_status", executionStatus,
			"task_id", taskID,
			"task_status", taskStatus,
			"user_id", userID,
		)

		return nil
	})

	if err != nil {
		s.logger.Error("failed to update execution and task status",
			"error", err,
			"execution_id", executionID,
			"task_id", taskID,
			"user_id", userID,
		)
		return err
	}

	return nil
}

// CancelExecutionAndResetTaskStatus atomically cancels an execution and resets task status
func (s *TaskExecutionService) CancelExecutionAndResetTaskStatus(ctx context.Context, executionID uuid.UUID, userID uuid.UUID) error {
	err := s.conn.WithTransaction(ctx, func(tx database.Transaction) error {
		repos := tx.Repositories()

		// First, verify the execution exists and belongs to the user's task
		execution, err := repos.TaskExecutions.GetByID(ctx, executionID)
		if err != nil {
			if err == database.ErrExecutionNotFound {
				return fmt.Errorf("execution not found")
			}
			return fmt.Errorf("failed to get execution: %w", err)
		}

		// Verify the task belongs to the user
		task, err := repos.Tasks.GetByID(ctx, execution.TaskID)
		if err != nil {
			return fmt.Errorf("failed to get task: %w", err)
		}

		if task.UserID != userID {
			return fmt.Errorf("access denied: task does not belong to user")
		}

		// Check if execution can be cancelled
		if execution.Status == models.ExecutionStatusCompleted ||
			execution.Status == models.ExecutionStatusFailed ||
			execution.Status == models.ExecutionStatusCancelled {
			return fmt.Errorf("cannot cancel execution with status: %s", execution.Status)
		}

		// Update execution status to cancelled
		if err := repos.TaskExecutions.UpdateStatus(ctx, executionID, models.ExecutionStatusCancelled); err != nil {
			return fmt.Errorf("failed to cancel execution: %w", err)
		}

		// Reset task status to pending
		if err := repos.Tasks.UpdateStatus(ctx, execution.TaskID, models.TaskStatusPending); err != nil {
			return fmt.Errorf("failed to reset task status: %w", err)
		}

		s.logger.Info("execution cancelled and task status reset atomically",
			"execution_id", executionID,
			"task_id", execution.TaskID,
			"user_id", userID,
		)

		return nil
	})

	if err != nil {
		s.logger.Error("failed to cancel execution and reset task status",
			"error", err,
			"execution_id", executionID,
			"user_id", userID,
		)
		return err
	}

	return nil
}

// CompleteExecutionAndFinalizeTaskStatus atomically completes an execution with results and finalizes task status
func (s *TaskExecutionService) CompleteExecutionAndFinalizeTaskStatus(ctx context.Context, execution *models.TaskExecution, taskStatus models.TaskStatus, userID uuid.UUID) error {
	err := s.conn.WithTransaction(ctx, func(tx database.Transaction) error {
		repos := tx.Repositories()

		// First, verify the execution exists and belongs to the user's task
		existingExecution, err := repos.TaskExecutions.GetByID(ctx, execution.ID)
		if err != nil {
			if err == database.ErrExecutionNotFound {
				return fmt.Errorf("execution not found")
			}
			return fmt.Errorf("failed to get execution: %w", err)
		}

		// Verify the task belongs to the user
		task, err := repos.Tasks.GetByID(ctx, existingExecution.TaskID)
		if err != nil {
			return fmt.Errorf("failed to get task: %w", err)
		}

		if task.UserID != userID {
			return fmt.Errorf("access denied: task does not belong to user")
		}

		// Check if execution can be completed
		if existingExecution.Status == models.ExecutionStatusCompleted ||
			existingExecution.Status == models.ExecutionStatusCancelled {
			return fmt.Errorf("cannot complete execution with status: %s", existingExecution.Status)
		}

		// Update execution with results
		if err := repos.TaskExecutions.Update(ctx, execution); err != nil {
			return fmt.Errorf("failed to update execution: %w", err)
		}

		// Update task status
		if err := repos.Tasks.UpdateStatus(ctx, existingExecution.TaskID, taskStatus); err != nil {
			return fmt.Errorf("failed to update task status: %w", err)
		}

		s.logger.Info("execution completed and task status finalized atomically",
			"execution_id", execution.ID,
			"execution_status", execution.Status,
			"task_id", existingExecution.TaskID,
			"task_status", taskStatus,
			"user_id", userID,
		)

		return nil
	})

	if err != nil {
		s.logger.Error("failed to complete execution and finalize task status",
			"error", err,
			"execution_id", execution.ID,
			"user_id", userID,
		)
		return err
	}

	return nil
}

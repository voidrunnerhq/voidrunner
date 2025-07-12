package executor

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/voidrunnerhq/voidrunner/internal/models"
)

// MockExecutor implements TaskExecutor interface for testing environments
// where Docker is not available or desired
type MockExecutor struct {
	config     *Config
	logger     *slog.Logger
	executions map[uuid.UUID]*mockExecution
}

type mockExecution struct {
	id        uuid.UUID
	status    models.ExecutionStatus
	startedAt time.Time
	result    *ExecutionResult
}

// NewMockExecutor creates a new mock executor for testing
func NewMockExecutor(config *Config, logger *slog.Logger) *MockExecutor {
	if config == nil {
		config = NewDefaultConfig()
	}
	if logger == nil {
		logger = slog.Default()
	}

	return &MockExecutor{
		config:     config,
		logger:     logger,
		executions: make(map[uuid.UUID]*mockExecution),
	}
}

// Execute simulates task execution and returns a mock result
func (m *MockExecutor) Execute(ctx context.Context, execCtx *ExecutionContext) (*ExecutionResult, error) {
	if execCtx == nil || execCtx.Task == nil || execCtx.Execution == nil {
		return nil, NewExecutorError("mock_execute", "invalid execution context", nil)
	}

	logger := m.logger.With(
		"task_id", execCtx.Task.ID.String(),
		"execution_id", execCtx.Execution.ID.String(),
		"operation", "mock_execute",
	)

	logger.Info("starting mock task execution")

	// Register the execution
	mockExec := &mockExecution{
		id:        execCtx.Execution.ID,
		status:    models.ExecutionStatusRunning,
		startedAt: time.Now(),
	}
	m.executions[execCtx.Execution.ID] = mockExec

	// Simulate execution time (short for tests)
	executionTime := 100 * time.Millisecond
	if execCtx.Timeout > 0 && execCtx.Timeout < executionTime {
		executionTime = execCtx.Timeout
	}

	select {
	case <-time.After(executionTime):
		// Normal completion
		result := m.generateMockResult(execCtx)
		mockExec.result = result
		mockExec.status = result.Status
		logger.Info("mock task execution completed successfully")
		return result, nil

	case <-ctx.Done():
		// Cancelled
		result := &ExecutionResult{
			Status:          models.ExecutionStatusCancelled,
			ReturnCode:      intPtr(-1),
			Stdout:          stringPtr(""),
			Stderr:          stringPtr("execution cancelled"),
			ExecutionTimeMs: intPtr(int(time.Since(mockExec.startedAt).Milliseconds())),
			StartedAt:       &mockExec.startedAt,
			CompletedAt:     timePtr(time.Now()),
		}
		mockExec.result = result
		mockExec.status = models.ExecutionStatusCancelled
		logger.Info("mock task execution cancelled")
		return result, ctx.Err()
	}
}

// Cancel simulates cancelling a running execution
func (m *MockExecutor) Cancel(ctx context.Context, executionID uuid.UUID) error {
	logger := m.logger.With(
		"execution_id", executionID.String(),
		"operation", "mock_cancel",
	)

	mockExec, exists := m.executions[executionID]
	if !exists {
		logger.Warn("attempted to cancel non-existent execution")
		return NewExecutorError("mock_cancel", "execution not found", nil)
	}

	if mockExec.status != models.ExecutionStatusRunning {
		logger.Warn("attempted to cancel non-running execution", "status", mockExec.status)
		return NewExecutorError("mock_cancel", "execution not running", nil)
	}

	mockExec.status = models.ExecutionStatusCancelled
	if mockExec.result == nil {
		mockExec.result = &ExecutionResult{
			Status:          models.ExecutionStatusCancelled,
			ReturnCode:      intPtr(-1),
			Stdout:          stringPtr(""),
			Stderr:          stringPtr("execution cancelled"),
			ExecutionTimeMs: intPtr(int(time.Since(mockExec.startedAt).Milliseconds())),
			StartedAt:       &mockExec.startedAt,
			CompletedAt:     timePtr(time.Now()),
		}
	}

	logger.Info("mock execution cancelled successfully")
	return nil
}

// IsHealthy always returns healthy for mock executor
func (m *MockExecutor) IsHealthy(ctx context.Context) error {
	m.logger.Debug("mock executor health check - always healthy")
	return nil
}

// Cleanup cleans up mock execution records
func (m *MockExecutor) Cleanup(ctx context.Context) error {
	logger := m.logger.With("operation", "mock_cleanup")

	executionCount := len(m.executions)
	m.executions = make(map[uuid.UUID]*mockExecution)

	logger.Info("mock executor cleanup completed", "cleaned_executions", executionCount)
	return nil
}

// generateMockResult creates a realistic mock execution result
func (m *MockExecutor) generateMockResult(execCtx *ExecutionContext) *ExecutionResult {
	now := time.Now()
	startedAt := now.Add(-100 * time.Millisecond)

	// Generate mock output based on script type
	var stdout, stderr string
	var returnCode int

	switch execCtx.Task.ScriptType {
	case models.ScriptTypePython:
		stdout = "Mock Python execution output\n"
		stderr = ""
		returnCode = 0
	case models.ScriptTypeBash:
		stdout = "Mock Bash execution output\n"
		stderr = ""
		returnCode = 0
	case models.ScriptTypeJavaScript:
		stdout = "Mock JavaScript execution output\n"
		stderr = ""
		returnCode = 0
	default:
		stdout = "Mock execution output\n"
		stderr = ""
		returnCode = 0
	}

	// Simulate different execution outcomes based on script content
	if execCtx.Task.ScriptContent != "" {
		// Check for common error patterns
		content := execCtx.Task.ScriptContent
		if containsErrorPatterns(content) {
			stdout = ""
			stderr = "Mock execution error\n"
			returnCode = 1
		} else if containsTimeoutPatterns(content) {
			return &ExecutionResult{
				Status:          models.ExecutionStatusTimeout,
				ReturnCode:      intPtr(-1),
				Stdout:          stringPtr(""),
				Stderr:          stringPtr("execution timed out"),
				ExecutionTimeMs: intPtr(int(execCtx.Timeout.Milliseconds())),
				StartedAt:       &startedAt,
				CompletedAt:     &now,
			}
		}
	}

	status := models.ExecutionStatusCompleted
	if returnCode != 0 {
		status = models.ExecutionStatusFailed
	}

	return &ExecutionResult{
		Status:           status,
		ReturnCode:       &returnCode,
		Stdout:           &stdout,
		Stderr:           &stderr,
		ExecutionTimeMs:  intPtr(100),           // 100ms mock execution time
		MemoryUsageBytes: int64Ptr(1024 * 1024), // 1MB mock memory usage
		StartedAt:        &startedAt,
		CompletedAt:      &now,
	}
}

// containsErrorPatterns checks if script content should simulate an error
func containsErrorPatterns(content string) bool {
	errorPatterns := []string{
		"exit 1",
		"raise Exception",
		"throw new Error",
		"panic(",
	}

	for _, pattern := range errorPatterns {
		if len(content) > len(pattern) && content[:len(pattern)] == pattern {
			return true
		}
	}
	return false
}

// containsTimeoutPatterns checks if script content should simulate a timeout
func containsTimeoutPatterns(content string) bool {
	timeoutPatterns := []string{
		"sleep(",
		"time.sleep",
		"setTimeout",
		"Thread.sleep",
	}

	for _, pattern := range timeoutPatterns {
		if len(content) > len(pattern) && content[:len(pattern)] == pattern {
			return true
		}
	}
	return false
}

// Helper functions for pointer creation
func intPtr(i int) *int {
	return &i
}

func int64Ptr(i int64) *int64 {
	return &i
}

func timePtr(t time.Time) *time.Time {
	return &t
}

package testutil

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/voidrunnerhq/voidrunner/internal/models"
)

// UserFactory provides fluent API for creating test users
type UserFactory struct {
	user *models.User
}

// NewUserFactory creates a new user factory with default values
func NewUserFactory() *UserFactory {
	return &UserFactory{
		user: &models.User{
			BaseModel: models.BaseModel{
				ID:        uuid.New(),
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			Email:        fmt.Sprintf("user-%s@test.com", uuid.New().String()[:8]),
			PasswordHash: "$2a$12$LQv3c1yqBWVHxkd0LHAkCOYz6TtxMQJqhN8/LewdBPj4KTrkYEoA2", // TestPass123!
			Name:         "Test User",
		},
	}
}

// WithID sets the user ID
func (f *UserFactory) WithID(id uuid.UUID) *UserFactory {
	f.user.ID = id
	return f
}

// WithEmail sets the user email
func (f *UserFactory) WithEmail(email string) *UserFactory {
	f.user.Email = email
	return f
}

// WithName sets the user name
func (f *UserFactory) WithName(name string) *UserFactory {
	f.user.Name = name
	return f
}

// WithPasswordHash sets the password hash
func (f *UserFactory) WithPasswordHash(hash string) *UserFactory {
	f.user.PasswordHash = hash
	return f
}

// WithCreatedAt sets the creation time
func (f *UserFactory) WithCreatedAt(t time.Time) *UserFactory {
	f.user.CreatedAt = t
	return f
}

// WithUpdatedAt sets the update time
func (f *UserFactory) WithUpdatedAt(t time.Time) *UserFactory {
	f.user.UpdatedAt = t
	return f
}

// Admin creates an admin user
func (f *UserFactory) Admin() *UserFactory {
	return f.WithEmail("admin@voidrunner.dev").WithName("Admin User")
}

// Regular creates a regular user
func (f *UserFactory) Regular() *UserFactory {
	return f.WithEmail("user@voidrunner.dev").WithName("Regular User")
}

// Build returns the constructed user
func (f *UserFactory) Build() *models.User {
	// Create a copy to avoid mutation issues
	return &models.User{
		BaseModel:    f.user.BaseModel,
		Email:        f.user.Email,
		PasswordHash: f.user.PasswordHash,
		Name:         f.user.Name,
	}
}

// TaskFactory provides fluent API for creating test tasks
type TaskFactory struct {
	task *models.Task
}

// NewTaskFactory creates a new task factory with default values
func NewTaskFactory(userID uuid.UUID) *TaskFactory {
	return &TaskFactory{
		task: &models.Task{
			BaseModel: models.BaseModel{
				ID:        uuid.New(),
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			UserID:         userID,
			Name:           fmt.Sprintf("Test Task %s", uuid.New().String()[:8]),
			Description:    stringPtr("A test task"),
			ScriptContent:  "print('Hello World')",
			ScriptType:     models.ScriptTypePython,
			Status:         models.TaskStatusPending,
			Priority:       5,
			TimeoutSeconds: 30,
			Metadata:       models.JSONB{"test": true},
		},
	}
}

// WithID sets the task ID
func (f *TaskFactory) WithID(id uuid.UUID) *TaskFactory {
	f.task.ID = id
	return f
}

// WithUserID sets the user ID
func (f *TaskFactory) WithUserID(userID uuid.UUID) *TaskFactory {
	f.task.UserID = userID
	return f
}

// WithName sets the task name
func (f *TaskFactory) WithName(name string) *TaskFactory {
	f.task.Name = name
	return f
}

// WithDescription sets the task description
func (f *TaskFactory) WithDescription(description string) *TaskFactory {
	f.task.Description = &description
	return f
}

// WithScript sets the script content and type
func (f *TaskFactory) WithScript(content string, scriptType models.ScriptType) *TaskFactory {
	f.task.ScriptContent = content
	f.task.ScriptType = scriptType
	return f
}

// WithPythonScript sets a Python script
func (f *TaskFactory) WithPythonScript(content string) *TaskFactory {
	return f.WithScript(content, models.ScriptTypePython)
}

// WithJavaScriptScript sets a JavaScript script
func (f *TaskFactory) WithJavaScriptScript(content string) *TaskFactory {
	return f.WithScript(content, models.ScriptTypeJavaScript)
}

// WithBashScript sets a Bash script
func (f *TaskFactory) WithBashScript(content string) *TaskFactory {
	return f.WithScript(content, models.ScriptTypeBash)
}

// WithStatus sets the task status
func (f *TaskFactory) WithStatus(status models.TaskStatus) *TaskFactory {
	f.task.Status = status
	return f
}

// Pending sets the task status to pending
func (f *TaskFactory) Pending() *TaskFactory {
	return f.WithStatus(models.TaskStatusPending)
}

// Running sets the task status to running
func (f *TaskFactory) Running() *TaskFactory {
	return f.WithStatus(models.TaskStatusRunning)
}

// Completed sets the task status to completed
func (f *TaskFactory) Completed() *TaskFactory {
	return f.WithStatus(models.TaskStatusCompleted)
}

// Failed sets the task status to failed
func (f *TaskFactory) Failed() *TaskFactory {
	return f.WithStatus(models.TaskStatusFailed)
}

// WithPriority sets the task priority
func (f *TaskFactory) WithPriority(priority int) *TaskFactory {
	f.task.Priority = priority
	return f
}

// HighPriority sets high priority (8-10)
func (f *TaskFactory) HighPriority() *TaskFactory {
	return f.WithPriority(10)
}

// LowPriority sets low priority (1-3)
func (f *TaskFactory) LowPriority() *TaskFactory {
	return f.WithPriority(2)
}

// WithTimeout sets the timeout in seconds
func (f *TaskFactory) WithTimeout(seconds int) *TaskFactory {
	f.task.TimeoutSeconds = seconds
	return f
}

// WithMetadata sets the metadata
func (f *TaskFactory) WithMetadata(metadata models.JSONB) *TaskFactory {
	f.task.Metadata = metadata
	return f
}

// WithMetadataField adds a single metadata field
func (f *TaskFactory) WithMetadataField(key string, value interface{}) *TaskFactory {
	if f.task.Metadata == nil {
		f.task.Metadata = make(models.JSONB)
	}
	f.task.Metadata[key] = value
	return f
}

// WithCreatedAt sets the creation time
func (f *TaskFactory) WithCreatedAt(t time.Time) *TaskFactory {
	f.task.CreatedAt = t
	return f
}

// WithUpdatedAt sets the update time
func (f *TaskFactory) WithUpdatedAt(t time.Time) *TaskFactory {
	f.task.UpdatedAt = t
	return f
}

// Build returns the constructed task
func (f *TaskFactory) Build() *models.Task {
	// Create a copy to avoid mutation issues
	metadata := make(models.JSONB)
	for k, v := range f.task.Metadata {
		metadata[k] = v
	}

	return &models.Task{
		BaseModel:      f.task.BaseModel,
		UserID:         f.task.UserID,
		Name:           f.task.Name,
		Description:    f.task.Description,
		ScriptContent:  f.task.ScriptContent,
		ScriptType:     f.task.ScriptType,
		Status:         f.task.Status,
		Priority:       f.task.Priority,
		TimeoutSeconds: f.task.TimeoutSeconds,
		Metadata:       metadata,
	}
}

// ExecutionFactory provides fluent API for creating test executions
type ExecutionFactory struct {
	execution *models.TaskExecution
}

// NewExecutionFactory creates a new execution factory with default values
func NewExecutionFactory(taskID uuid.UUID) *ExecutionFactory {
	return &ExecutionFactory{
		execution: &models.TaskExecution{
			ID:        uuid.New(),
			TaskID:    taskID,
			Status:    models.ExecutionStatusPending,
			CreatedAt: time.Now(),
		},
	}
}

// WithID sets the execution ID
func (f *ExecutionFactory) WithID(id uuid.UUID) *ExecutionFactory {
	f.execution.ID = id
	return f
}

// WithTaskID sets the task ID
func (f *ExecutionFactory) WithTaskID(taskID uuid.UUID) *ExecutionFactory {
	f.execution.TaskID = taskID
	return f
}

// WithStatus sets the execution status
func (f *ExecutionFactory) WithStatus(status models.ExecutionStatus) *ExecutionFactory {
	f.execution.Status = status
	return f
}

// Pending sets the execution status to pending
func (f *ExecutionFactory) Pending() *ExecutionFactory {
	return f.WithStatus(models.ExecutionStatusPending)
}

// Running sets the execution status to running
func (f *ExecutionFactory) Running() *ExecutionFactory {
	return f.WithStatus(models.ExecutionStatusRunning)
}

// Completed sets the execution status to completed
func (f *ExecutionFactory) Completed() *ExecutionFactory {
	return f.WithStatus(models.ExecutionStatusCompleted)
}

// Failed sets the execution status to failed
func (f *ExecutionFactory) Failed() *ExecutionFactory {
	return f.WithStatus(models.ExecutionStatusFailed)
}

// Timeout sets the execution status to timeout
func (f *ExecutionFactory) Timeout() *ExecutionFactory {
	return f.WithStatus(models.ExecutionStatusTimeout)
}

// WithReturnCode sets the return code
func (f *ExecutionFactory) WithReturnCode(code int) *ExecutionFactory {
	f.execution.ReturnCode = &code
	return f
}

// WithOutput sets stdout and stderr
func (f *ExecutionFactory) WithOutput(stdout, stderr string) *ExecutionFactory {
	f.execution.Stdout = &stdout
	f.execution.Stderr = &stderr
	return f
}

// WithStdout sets the stdout
func (f *ExecutionFactory) WithStdout(stdout string) *ExecutionFactory {
	f.execution.Stdout = &stdout
	return f
}

// WithStderr sets the stderr
func (f *ExecutionFactory) WithStderr(stderr string) *ExecutionFactory {
	f.execution.Stderr = &stderr
	return f
}

// WithExecutionTime sets the execution time in milliseconds
func (f *ExecutionFactory) WithExecutionTime(ms int) *ExecutionFactory {
	f.execution.ExecutionTimeMs = &ms
	return f
}

// WithMemoryUsage sets the memory usage in bytes
func (f *ExecutionFactory) WithMemoryUsage(bytes int64) *ExecutionFactory {
	f.execution.MemoryUsageBytes = &bytes
	return f
}

// WithTimes sets the start and completion times
func (f *ExecutionFactory) WithTimes(startedAt, completedAt time.Time) *ExecutionFactory {
	f.execution.StartedAt = &startedAt
	f.execution.CompletedAt = &completedAt
	return f
}

// WithStartedAt sets the start time
func (f *ExecutionFactory) WithStartedAt(t time.Time) *ExecutionFactory {
	f.execution.StartedAt = &t
	return f
}

// WithCompletedAt sets the completion time
func (f *ExecutionFactory) WithCompletedAt(t time.Time) *ExecutionFactory {
	f.execution.CompletedAt = &t
	return f
}

// WithCreatedAt sets the creation time
func (f *ExecutionFactory) WithCreatedAt(t time.Time) *ExecutionFactory {
	f.execution.CreatedAt = t
	return f
}

// Successful creates a successful execution
func (f *ExecutionFactory) Successful() *ExecutionFactory {
	now := time.Now()
	return f.Completed().
		WithReturnCode(0).
		WithOutput("Success output", "").
		WithExecutionTime(1000).
		WithMemoryUsage(1048576). // 1MB
		WithTimes(now.Add(-5*time.Minute), now.Add(-4*time.Minute))
}

// FailedExecution creates a failed execution
func (f *ExecutionFactory) FailedExecution() *ExecutionFactory {
	now := time.Now()
	return f.Failed().
		WithReturnCode(1).
		WithOutput("", "Error output").
		WithExecutionTime(500).
		WithMemoryUsage(524288). // 512KB
		WithTimes(now.Add(-3*time.Minute), now.Add(-2*time.Minute))
}

// Build returns the constructed execution
func (f *ExecutionFactory) Build() *models.TaskExecution {
	// Create a copy to avoid mutation issues
	return &models.TaskExecution{
		ID:               f.execution.ID,
		TaskID:           f.execution.TaskID,
		Status:           f.execution.Status,
		ReturnCode:       f.execution.ReturnCode,
		Stdout:           f.execution.Stdout,
		Stderr:           f.execution.Stderr,
		ExecutionTimeMs:  f.execution.ExecutionTimeMs,
		MemoryUsageBytes: f.execution.MemoryUsageBytes,
		StartedAt:        f.execution.StartedAt,
		CompletedAt:      f.execution.CompletedAt,
		CreatedAt:        f.execution.CreatedAt,
	}
}

// RequestFactory provides fluent API for creating test requests
type RequestFactory struct{}

// NewRequestFactory creates a new request factory
func NewRequestFactory() *RequestFactory {
	return &RequestFactory{}
}

// Auth Request Methods

// ValidRegisterRequest creates a valid registration request
func (f *RequestFactory) ValidRegisterRequest() models.RegisterRequest {
	return models.RegisterRequest{
		Email:    fmt.Sprintf("user-%s@example.com", uuid.New().String()[:8]),
		Password: "SecurePassword123!",
		Name:     "Test User",
	}
}

// ValidRegisterRequestWithEmail creates a valid registration request with specific email
func (f *RequestFactory) ValidRegisterRequestWithEmail(email string) models.RegisterRequest {
	return models.RegisterRequest{
		Email:    email,
		Password: "SecurePassword123!",
		Name:     "Test User",
	}
}

// ValidLoginRequest creates a valid login request
func (f *RequestFactory) ValidLoginRequest() models.LoginRequest {
	return models.LoginRequest{
		Email:    "user@example.com",
		Password: "SecurePassword123!",
	}
}

// ValidLoginRequestWithCredentials creates a valid login request with specific credentials
func (f *RequestFactory) ValidLoginRequestWithCredentials(email, password string) models.LoginRequest {
	return models.LoginRequest{
		Email:    email,
		Password: password,
	}
}

// ValidRefreshTokenRequest creates a valid refresh token request
func (f *RequestFactory) ValidRefreshTokenRequest(refreshToken string) models.RefreshTokenRequest {
	return models.RefreshTokenRequest{
		RefreshToken: refreshToken,
	}
}

// Invalid Auth Requests

// InvalidRegisterRequestEmptyEmail creates an invalid registration request with empty email
func (f *RequestFactory) InvalidRegisterRequestEmptyEmail() models.RegisterRequest {
	return models.RegisterRequest{
		Email:    "",
		Password: "SecurePassword123!",
		Name:     "Test User",
	}
}

// InvalidRegisterRequestWeakPassword creates an invalid registration request with weak password
func (f *RequestFactory) InvalidRegisterRequestWeakPassword() models.RegisterRequest {
	return models.RegisterRequest{
		Email:    "user@example.com",
		Password: "123",
		Name:     "Test User",
	}
}

// Task Request Methods

// ValidCreateTaskRequest creates a valid task creation request
func (f *RequestFactory) ValidCreateTaskRequest() models.CreateTaskRequest {
	priority := 5
	timeout := 300
	description := "A test task created by factory"

	return models.CreateTaskRequest{
		Name:           fmt.Sprintf("Factory Test Task %s", uuid.New().String()[:8]),
		Description:    &description,
		ScriptContent:  "print('Hello from factory!')",
		ScriptType:     models.ScriptTypePython,
		Priority:       &priority,
		TimeoutSeconds: &timeout,
		Metadata:       map[string]interface{}{"source": "factory"},
	}
}

// ValidCreateTaskRequestWithName creates a valid task creation request with specific name
func (f *RequestFactory) ValidCreateTaskRequestWithName(name string) models.CreateTaskRequest {
	req := f.ValidCreateTaskRequest()
	req.Name = name
	return req
}

// ValidCreateTaskRequestWithScript creates a valid task creation request with specific script
func (f *RequestFactory) ValidCreateTaskRequestWithScript(scriptContent string, scriptType models.ScriptType) models.CreateTaskRequest {
	req := f.ValidCreateTaskRequest()
	req.ScriptContent = scriptContent
	req.ScriptType = scriptType
	return req
}

// ValidCreateTaskRequestMinimal creates a minimal valid task creation request
func (f *RequestFactory) ValidCreateTaskRequestMinimal() models.CreateTaskRequest {
	return models.CreateTaskRequest{
		Name:          "Minimal Task",
		ScriptContent: "print('minimal')",
		ScriptType:    models.ScriptTypePython,
	}
}

// ValidCreateTaskRequestWithTimeout creates a valid task creation request with specific timeout
func (f *RequestFactory) ValidCreateTaskRequestWithTimeout(timeoutSeconds int) models.CreateTaskRequest {
	req := f.ValidCreateTaskRequest()
	req.TimeoutSeconds = &timeoutSeconds
	return req
}

// ValidCreateTaskRequestWithMetadata creates a valid task creation request with custom metadata
func (f *RequestFactory) ValidCreateTaskRequestWithMetadata(metadata map[string]interface{}) models.CreateTaskRequest {
	req := f.ValidCreateTaskRequest()
	req.Metadata = metadata
	return req
}

// ValidUpdateTaskRequest creates a valid task update request
func (f *RequestFactory) ValidUpdateTaskRequest() models.UpdateTaskRequest {
	name := "Updated Task Name"
	description := "Updated task description"
	priority := 7

	return models.UpdateTaskRequest{
		Name:        &name,
		Description: &description,
		Priority:    &priority,
	}
}

// ValidUpdateTaskRequestPartial creates a partial task update request
func (f *RequestFactory) ValidUpdateTaskRequestPartial() models.UpdateTaskRequest {
	name := "Partially Updated Task"
	return models.UpdateTaskRequest{
		Name: &name,
	}
}

// Invalid Task Requests

// InvalidCreateTaskRequestEmptyName creates an invalid task creation request with empty name
func (f *RequestFactory) InvalidCreateTaskRequestEmptyName() models.CreateTaskRequest {
	return models.CreateTaskRequest{
		Name:          "",
		ScriptContent: "print('test')",
		ScriptType:    models.ScriptTypePython,
	}
}

// InvalidCreateTaskRequestDangerousScript creates an invalid task creation request with dangerous script
func (f *RequestFactory) InvalidCreateTaskRequestDangerousScript() models.CreateTaskRequest {
	return models.CreateTaskRequest{
		Name:          "Dangerous Task",
		ScriptContent: "rm -rf /",
		ScriptType:    models.ScriptTypeBash,
	}
}

// CreateLargeTaskRequest creates a task request with large but valid content
func (f *RequestFactory) CreateLargeTaskRequest() models.CreateTaskRequest {
	// Create large script that's just under the limit (65535 characters)
	largeScript := make([]byte, 65000)
	for i := range largeScript {
		largeScript[i] = 'a'
	}

	req := f.ValidCreateTaskRequest()
	req.Name = "Large Script Task"
	req.ScriptContent = string(largeScript)
	return req
}

// Task Execution Request Methods

// ValidCreateTaskExecutionRequest creates a valid task execution creation request
func (f *RequestFactory) ValidCreateTaskExecutionRequest(taskID uuid.UUID) models.CreateTaskExecutionRequest {
	return models.CreateTaskExecutionRequest{
		TaskID: taskID,
	}
}

// ValidUpdateTaskExecutionRequest creates a valid task execution update request
func (f *RequestFactory) ValidUpdateTaskExecutionRequest() models.UpdateTaskExecutionRequest {
	status := models.ExecutionStatusCompleted
	returnCode := 0
	stdout := "Task completed successfully\n"
	executionTime := 1500
	memoryUsage := int64(2048576) // 2MB

	return models.UpdateTaskExecutionRequest{
		Status:           &status,
		ReturnCode:       &returnCode,
		Stdout:           &stdout,
		ExecutionTimeMs:  &executionTime,
		MemoryUsageBytes: &memoryUsage,
	}
}

// ValidUpdateTaskExecutionRequestFailed creates a valid failed task execution update request
func (f *RequestFactory) ValidUpdateTaskExecutionRequestFailed() models.UpdateTaskExecutionRequest {
	status := models.ExecutionStatusFailed
	returnCode := 1
	stderr := "Error: Task failed with exception\n"
	executionTime := 500

	return models.UpdateTaskExecutionRequest{
		Status:          &status,
		ReturnCode:      &returnCode,
		Stderr:          &stderr,
		ExecutionTimeMs: &executionTime,
	}
}

// ValidUpdateTaskExecutionRequestTimeout creates a valid timeout task execution update request
func (f *RequestFactory) ValidUpdateTaskExecutionRequestTimeout() models.UpdateTaskExecutionRequest {
	status := models.ExecutionStatusTimeout
	stderr := "Error: Task timed out after 30 seconds\n"
	executionTime := 30000

	return models.UpdateTaskExecutionRequest{
		Status:          &status,
		Stderr:          &stderr,
		ExecutionTimeMs: &executionTime,
	}
}

// Utility Methods

// GetCommonHeaders returns common HTTP headers for API requests
func (f *RequestFactory) GetCommonHeaders() map[string]string {
	return map[string]string{
		"Content-Type": "application/json",
		"Accept":       "application/json",
	}
}

// GetAuthHeaders returns authorization headers with the given token
func (f *RequestFactory) GetAuthHeaders(accessToken string) map[string]string {
	headers := f.GetCommonHeaders()
	headers["Authorization"] = fmt.Sprintf("Bearer %s", accessToken)
	return headers
}

// Legacy methods for backward compatibility

// CreateTaskRequest creates a task creation request (legacy)
func (f *RequestFactory) CreateTaskRequest() *models.CreateTaskRequest {
	req := f.ValidCreateTaskRequest()
	return &req
}

// UpdateTaskRequest creates a task update request (legacy)
func (f *RequestFactory) UpdateTaskRequest() *models.UpdateTaskRequest {
	req := f.ValidUpdateTaskRequest()
	return &req
}

// RegisterRequest creates a user registration request (legacy)
func (f *RequestFactory) RegisterRequest() *models.RegisterRequest {
	req := f.ValidRegisterRequest()
	return &req
}

// LoginRequest creates a user login request (legacy)
func (f *RequestFactory) LoginRequest(email, password string) *models.LoginRequest {
	req := f.ValidLoginRequestWithCredentials(email, password)
	return &req
}

// StringPtr returns a pointer to a string value
func StringPtr(s string) *string {
	return &s
}

// stringPtr is kept for backwards compatibility
func stringPtr(s string) *string {
	return StringPtr(s)
}

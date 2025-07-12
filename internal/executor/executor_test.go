package executor

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/voidrunnerhq/voidrunner/internal/models"
)

// MockContainerClient is a mock implementation of ContainerClient for testing
type MockContainerClient struct {
	mock.Mock
}

func (m *MockContainerClient) CreateContainer(ctx context.Context, config *ContainerConfig) (string, error) {
	args := m.Called(ctx, config)
	return args.String(0), args.Error(1)
}

func (m *MockContainerClient) StartContainer(ctx context.Context, containerID string) error {
	args := m.Called(ctx, containerID)
	return args.Error(0)
}

func (m *MockContainerClient) WaitContainer(ctx context.Context, containerID string) (int, error) {
	args := m.Called(ctx, containerID)
	return args.Int(0), args.Error(1)
}

func (m *MockContainerClient) GetContainerLogs(ctx context.Context, containerID string) (stdout, stderr string, err error) {
	args := m.Called(ctx, containerID)
	return args.String(0), args.String(1), args.Error(2)
}

func (m *MockContainerClient) RemoveContainer(ctx context.Context, containerID string, force bool) error {
	args := m.Called(ctx, containerID, force)
	return args.Error(0)
}

func (m *MockContainerClient) StopContainer(ctx context.Context, containerID string, timeout time.Duration) error {
	args := m.Called(ctx, containerID, timeout)
	return args.Error(0)
}

func (m *MockContainerClient) IsHealthy(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockContainerClient) ListContainers(ctx context.Context, all bool) ([]ContainerSummary, error) {
	args := m.Called(ctx, all)
	return args.Get(0).([]ContainerSummary), args.Error(1)
}

func (m *MockContainerClient) GetDockerInfo(ctx context.Context) (interface{}, error) {
	args := m.Called(ctx)
	return args.Get(0), args.Error(1)
}

func (m *MockContainerClient) GetDockerVersion(ctx context.Context) (interface{}, error) {
	args := m.Called(ctx)
	return args.Get(0), args.Error(1)
}

func (m *MockContainerClient) GetContainerInfo(ctx context.Context, containerID string) (interface{}, error) {
	args := m.Called(ctx, containerID)
	return args.Get(0), args.Error(1)
}

func (m *MockContainerClient) PullImage(ctx context.Context, imageName string) error {
	args := m.Called(ctx, imageName)
	return args.Error(0)
}

func TestNewExecutor(t *testing.T) {
	tests := []struct {
		name      string
		config    *Config
		expectErr bool
	}{
		{
			name:      "Valid configuration",
			config:    NewDefaultConfig(),
			expectErr: false,
		},
		{
			name:      "Nil configuration uses default",
			config:    nil,
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor, err := NewExecutor(tt.config, nil)
			if tt.expectErr {
				require.Error(t, err)
				assert.Nil(t, executor)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, executor)
				assert.NotNil(t, executor.config)
				assert.NotNil(t, executor.securityManager)
				assert.NotNil(t, executor.cleanupManager)
				assert.NotNil(t, executor.logger)
			}
		})
	}
}

func TestExecutor_Execute(t *testing.T) {
	// Create a test executor with mocked dependencies
	config := NewDefaultConfig()
	// Disable seccomp for tests to avoid file system dependencies
	config.Security.EnableSeccomp = false
	executor := &Executor{
		config:          config,
		securityManager: NewSecurityManager(config),
		cleanupManager:  NewCleanupManager(nil, nil),
		logger:          slog.Default(),
	}

	// Test cases
	tests := []struct {
		name           string
		execCtx        *ExecutionContext
		mockSetup      func(*MockContainerClient)
		expectErr      bool
		expectedStatus models.ExecutionStatus
	}{
		{
			name: "Successful Python execution",
			execCtx: &ExecutionContext{
				Task: &models.Task{
					BaseModel: models.BaseModel{
						ID: uuid.New(),
					},
					ScriptType:    models.ScriptTypePython,
					ScriptContent: "print('Hello, World!')",
				},
				Execution: &models.TaskExecution{
					ID: uuid.New(),
				},
				Context: context.Background(),
				Timeout: 30 * time.Second,
				ResourceLimits: ResourceLimits{
					MemoryLimitBytes: 128 * 1024 * 1024,
					CPUQuota:         50000,
					PidsLimit:        128,
					TimeoutSeconds:   30,
				},
			},
			mockSetup: func(m *MockContainerClient) {
				m.On("CreateContainer", mock.Anything, mock.Anything).Return("container123", nil)
				m.On("StartContainer", mock.Anything, "container123").Return(nil)
				m.On("WaitContainer", mock.Anything, "container123").Return(0, nil)
				m.On("GetContainerLogs", mock.Anything, "container123").Return("Hello, World!", "", nil)
				m.On("RemoveContainer", mock.Anything, "container123", true).Return(nil)
			},
			expectErr:      false,
			expectedStatus: models.ExecutionStatusCompleted,
		},
		{
			name: "Successful Bash execution",
			execCtx: &ExecutionContext{
				Task: &models.Task{
					BaseModel: models.BaseModel{
						ID: uuid.New(),
					},
					ScriptType:    models.ScriptTypeBash,
					ScriptContent: "echo 'Hello, World!'",
				},
				Execution: &models.TaskExecution{
					ID: uuid.New(),
				},
				Context: context.Background(),
				Timeout: 30 * time.Second,
				ResourceLimits: ResourceLimits{
					MemoryLimitBytes: 128 * 1024 * 1024,
					CPUQuota:         50000,
					PidsLimit:        128,
					TimeoutSeconds:   30,
				},
			},
			mockSetup: func(m *MockContainerClient) {
				m.On("CreateContainer", mock.Anything, mock.Anything).Return("container456", nil)
				m.On("StartContainer", mock.Anything, "container456").Return(nil)
				m.On("WaitContainer", mock.Anything, "container456").Return(0, nil)
				m.On("GetContainerLogs", mock.Anything, "container456").Return("Hello, World!", "", nil)
				m.On("RemoveContainer", mock.Anything, "container456", true).Return(nil)
			},
			expectErr:      false,
			expectedStatus: models.ExecutionStatusCompleted,
		},
		{
			name: "Script with non-zero exit code",
			execCtx: &ExecutionContext{
				Task: &models.Task{
					BaseModel: models.BaseModel{
						ID: uuid.New(),
					},
					ScriptType:    models.ScriptTypePython,
					ScriptContent: "exit(1)",
				},
				Execution: &models.TaskExecution{
					ID: uuid.New(),
				},
				Context: context.Background(),
				Timeout: 30 * time.Second,
				ResourceLimits: ResourceLimits{
					MemoryLimitBytes: 128 * 1024 * 1024,
					CPUQuota:         50000,
					PidsLimit:        128,
					TimeoutSeconds:   30,
				},
			},
			mockSetup: func(m *MockContainerClient) {
				m.On("CreateContainer", mock.Anything, mock.Anything).Return("container789", nil)
				m.On("StartContainer", mock.Anything, "container789").Return(nil)
				m.On("WaitContainer", mock.Anything, "container789").Return(1, nil)
				m.On("GetContainerLogs", mock.Anything, "container789").Return("", "", nil)
				m.On("RemoveContainer", mock.Anything, "container789", true).Return(nil)
			},
			expectErr:      false,
			expectedStatus: models.ExecutionStatusFailed,
		},
		{
			name: "Container creation failure",
			execCtx: &ExecutionContext{
				Task: &models.Task{
					BaseModel: models.BaseModel{
						ID: uuid.New(),
					},
					ScriptType:    models.ScriptTypePython,
					ScriptContent: "print('Hello, World!')",
				},
				Execution: &models.TaskExecution{
					ID: uuid.New(),
				},
				Context: context.Background(),
				Timeout: 30 * time.Second,
				ResourceLimits: ResourceLimits{
					MemoryLimitBytes: 128 * 1024 * 1024,
					CPUQuota:         50000,
					PidsLimit:        128,
					TimeoutSeconds:   30,
				},
			},
			mockSetup: func(m *MockContainerClient) {
				m.On("CreateContainer", mock.Anything, mock.Anything).Return("", errors.New("failed to create container"))
			},
			expectErr:      true,
			expectedStatus: models.ExecutionStatusFailed,
		},
		{
			name: "Container start failure",
			execCtx: &ExecutionContext{
				Task: &models.Task{
					BaseModel: models.BaseModel{
						ID: uuid.New(),
					},
					ScriptType:    models.ScriptTypePython,
					ScriptContent: "print('Hello, World!')",
				},
				Execution: &models.TaskExecution{
					ID: uuid.New(),
				},
				Context: context.Background(),
				Timeout: 30 * time.Second,
				ResourceLimits: ResourceLimits{
					MemoryLimitBytes: 128 * 1024 * 1024,
					CPUQuota:         50000,
					PidsLimit:        128,
					TimeoutSeconds:   30,
				},
			},
			mockSetup: func(m *MockContainerClient) {
				m.On("CreateContainer", mock.Anything, mock.Anything).Return("containerABC", nil)
				m.On("StartContainer", mock.Anything, "containerABC").Return(errors.New("failed to start container"))
				m.On("RemoveContainer", mock.Anything, "containerABC", true).Return(nil)
			},
			expectErr:      true,
			expectedStatus: models.ExecutionStatusFailed,
		},
		{
			name: "Context timeout",
			execCtx: &ExecutionContext{
				Task: &models.Task{
					BaseModel: models.BaseModel{
						ID: uuid.New(),
					},
					ScriptType:    models.ScriptTypePython,
					ScriptContent: "import time; time.sleep(10)",
				},
				Execution: &models.TaskExecution{
					ID: uuid.New(),
				},
				Context: context.Background(),
				Timeout: 1 * time.Second,
				ResourceLimits: ResourceLimits{
					MemoryLimitBytes: 128 * 1024 * 1024,
					CPUQuota:         50000,
					PidsLimit:        128,
					TimeoutSeconds:   1,
				},
			},
			mockSetup: func(m *MockContainerClient) {
				m.On("CreateContainer", mock.Anything, mock.Anything).Return("containerDEF", nil)
				m.On("StartContainer", mock.Anything, "containerDEF").Return(nil)
				m.On("WaitContainer", mock.Anything, "containerDEF").Return(-1, context.DeadlineExceeded)
				m.On("GetContainerLogs", mock.Anything, "containerDEF").Return("", "", nil)
				m.On("RemoveContainer", mock.Anything, "containerDEF", true).Return(nil)
			},
			expectErr:      true,
			expectedStatus: models.ExecutionStatusFailed,
		},
		{
			name:    "Nil execution context",
			execCtx: nil,
			mockSetup: func(m *MockContainerClient) {
				// No mock setup needed for this test
			},
			expectErr:      true,
			expectedStatus: models.ExecutionStatusFailed,
		},
		{
			name: "Nil task in execution context",
			execCtx: &ExecutionContext{
				Task: nil,
				Execution: &models.TaskExecution{
					ID: uuid.New(),
				},
				Context: context.Background(),
				Timeout: 30 * time.Second,
			},
			mockSetup: func(m *MockContainerClient) {
				// No mock setup needed for this test
			},
			expectErr:      true,
			expectedStatus: models.ExecutionStatusFailed,
		},
		{
			name: "Dangerous script content",
			execCtx: &ExecutionContext{
				Task: &models.Task{
					BaseModel: models.BaseModel{
						ID: uuid.New(),
					},
					ScriptType:    models.ScriptTypePython,
					ScriptContent: "import os; os.system('rm -rf /')",
				},
				Execution: &models.TaskExecution{
					ID: uuid.New(),
				},
				Context: context.Background(),
				Timeout: 30 * time.Second,
				ResourceLimits: ResourceLimits{
					MemoryLimitBytes: 128 * 1024 * 1024,
					CPUQuota:         50000,
					PidsLimit:        128,
					TimeoutSeconds:   30,
				},
			},
			mockSetup: func(m *MockContainerClient) {
				// No mock setup needed for this test as it should fail validation
			},
			expectErr:      true,
			expectedStatus: models.ExecutionStatusFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock client
			mockClient := new(MockContainerClient)
			tt.mockSetup(mockClient)

			// Set mock client in executor
			executor.client = mockClient

			// Execute
			result, err := executor.Execute(context.Background(), tt.execCtx)

			// Verify results
			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			if result != nil {
				assert.Equal(t, tt.expectedStatus, result.Status)
			}

			// Verify all expected calls were made
			mockClient.AssertExpectations(t)
		})
	}
}

func TestExecutor_Cancel(t *testing.T) {
	config := NewDefaultConfig()
	executor := &Executor{
		config:          config,
		securityManager: NewSecurityManager(config),
		cleanupManager:  NewCleanupManager(nil, nil),
		logger:          slog.Default(),
	}

	// Test successful cancellation
	executionID := uuid.New()
	err := executor.Cancel(context.Background(), executionID)

	// Should not error even if there are no containers to cancel
	assert.NoError(t, err)
}

func TestExecutor_IsHealthy(t *testing.T) {
	config := NewDefaultConfig()
	executor := &Executor{
		config:          config,
		securityManager: NewSecurityManager(config),
		cleanupManager:  NewCleanupManager(nil, nil),
		logger:          slog.Default(),
	}

	// Test health check
	mockClient := new(MockContainerClient)
	mockClient.On("IsHealthy", mock.Anything).Return(nil)
	executor.client = mockClient

	err := executor.IsHealthy(context.Background())
	assert.NoError(t, err)
	mockClient.AssertExpectations(t)

	// Test health check failure
	mockClient2 := new(MockContainerClient)
	mockClient2.On("IsHealthy", mock.Anything).Return(errors.New("docker daemon not available"))
	executor.client = mockClient2

	err = executor.IsHealthy(context.Background())
	assert.Error(t, err)
	mockClient2.AssertExpectations(t)
}

func TestExecutor_Cleanup(t *testing.T) {
	config := NewDefaultConfig()
	executor := &Executor{
		config:          config,
		securityManager: NewSecurityManager(config),
		cleanupManager:  NewCleanupManager(nil, nil),
		logger:          slog.Default(),
	}

	// Test cleanup
	err := executor.Cleanup(context.Background())
	assert.NoError(t, err)
}

func TestExecutor_GetExecutorInfo(t *testing.T) {
	config := NewDefaultConfig()
	executor := &Executor{
		config:          config,
		securityManager: NewSecurityManager(config),
		cleanupManager:  NewCleanupManager(nil, nil),
		logger:          slog.Default(),
	}

	// Mock client that implements GetDockerInfo
	mockClient := new(MockContainerClient)

	// Mock the required methods
	mockClient.On("GetDockerInfo", mock.Anything).Return(map[string]interface{}{
		"ServerVersion": "20.10.7",
		"Architecture":  "x86_64",
	}, nil)
	mockClient.On("IsHealthy", mock.Anything).Return(nil)

	executor.client = mockClient

	// Test the GetExecutorInfo functionality
	info, err := executor.GetExecutorInfo(context.Background())

	// Verify the result
	require.NoError(t, err)
	assert.NotNil(t, info)
	assert.Equal(t, "1.0.0", info.Version)
	assert.NotNil(t, info.Config)

	// Verify mock expectations
	mockClient.AssertExpectations(t)
}

func TestExecutor_buildContainerConfig(t *testing.T) {
	config := NewDefaultConfig()
	executor := &Executor{
		config:          config,
		securityManager: NewSecurityManager(config),
		cleanupManager:  NewCleanupManager(nil, nil),
		logger:          slog.Default(),
	}

	task := &models.Task{
		BaseModel: models.BaseModel{
			ID: uuid.New(),
		},
		ScriptType:    models.ScriptTypePython,
		ScriptContent: "print('test')",
	}

	resourceLimits := ResourceLimits{
		MemoryLimitBytes: 128 * 1024 * 1024,
		CPUQuota:         50000,
		PidsLimit:        128,
		TimeoutSeconds:   300,
	}

	containerConfig, err := executor.buildContainerConfig(task, resourceLimits, 300*time.Second)

	require.NoError(t, err)
	assert.NotNil(t, containerConfig)
	assert.Equal(t, config.Images.Python, containerConfig.Image)
	assert.Equal(t, models.ScriptTypePython, containerConfig.ScriptType)
	assert.Equal(t, "print('test')", containerConfig.ScriptContent)
	assert.Equal(t, resourceLimits, containerConfig.ResourceLimits)
	assert.Equal(t, 300*time.Second, containerConfig.Timeout)
	assert.NotEmpty(t, containerConfig.Environment)
	assert.Equal(t, "/tmp/workspace", containerConfig.WorkingDir)
}

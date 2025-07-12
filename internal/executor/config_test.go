package executor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/voidrunnerhq/voidrunner/internal/models"
)

func TestNewDefaultConfig(t *testing.T) {
	config := NewDefaultConfig()

	assert.NotNil(t, config)
	assert.Equal(t, "unix:///var/run/docker.sock", config.DockerEndpoint)
	assert.Equal(t, int64(128*1024*1024), config.DefaultResourceLimits.MemoryLimitBytes)
	assert.Equal(t, int64(50000), config.DefaultResourceLimits.CPUQuota)
	assert.Equal(t, int64(128), config.DefaultResourceLimits.PidsLimit)
	assert.Equal(t, 300, config.DefaultTimeoutSeconds)
	assert.Equal(t, "python:3.11-alpine", config.Images.Python)
	assert.Equal(t, "alpine:latest", config.Images.Bash)
	assert.True(t, config.Security.EnableSeccomp)
	assert.Equal(t, "1000:1000", config.Security.ExecutionUser)
}

func TestConfig_GetImageForScriptType(t *testing.T) {
	config := NewDefaultConfig()

	tests := []struct {
		name       string
		scriptType models.ScriptType
		expected   string
	}{
		{
			name:       "Python script",
			scriptType: models.ScriptTypePython,
			expected:   "python:3.11-alpine",
		},
		{
			name:       "Bash script",
			scriptType: models.ScriptTypeBash,
			expected:   "alpine:latest",
		},
		{
			name:       "JavaScript script",
			scriptType: models.ScriptTypeJavaScript,
			expected:   "node:18-alpine",
		},
		{
			name:       "Go script",
			scriptType: models.ScriptTypeGo,
			expected:   "golang:1.21-alpine",
		},
		{
			name:       "Unknown script type",
			scriptType: "unknown",
			expected:   "python:3.11-alpine", // Should default to Python
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := config.GetImageForScriptType(tt.scriptType)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConfig_GetResourceLimitsForTask(t *testing.T) {
	config := NewDefaultConfig()

	tests := []struct {
		name            string
		task            *models.Task
		expectedMemory  int64
		expectedCPU     int64
		expectedTimeout int
	}{
		{
			name: "Low priority task",
			task: &models.Task{
				Priority:       1,
				TimeoutSeconds: 0, // Use default
			},
			expectedMemory:  64 * 1024 * 1024, // Half of default
			expectedCPU:     25000,            // Half of default
			expectedTimeout: 300,              // Default
		},
		{
			name: "Normal priority task",
			task: &models.Task{
				Priority:       5,
				TimeoutSeconds: 600,
			},
			expectedMemory:  128 * 1024 * 1024, // Default
			expectedCPU:     50000,             // Default
			expectedTimeout: 600,               // Custom
		},
		{
			name: "High priority task",
			task: &models.Task{
				Priority:       8,
				TimeoutSeconds: 0,
			},
			expectedMemory:  256 * 1024 * 1024, // Double default
			expectedCPU:     100000,            // Double default
			expectedTimeout: 300,               // Default
		},
		{
			name: "Critical priority task",
			task: &models.Task{
				Priority:       10,
				TimeoutSeconds: 1800,
			},
			expectedMemory:  512 * 1024 * 1024, // Quadruple default
			expectedCPU:     100000,            // Double default (capped)
			expectedTimeout: 1800,              // Custom
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limits := config.GetResourceLimitsForTask(tt.task)
			assert.Equal(t, tt.expectedMemory, limits.MemoryLimitBytes)
			assert.Equal(t, tt.expectedCPU, limits.CPUQuota)
			assert.Equal(t, tt.expectedTimeout, limits.TimeoutSeconds)
		})
	}
}

func TestConfig_GetTimeoutForTask(t *testing.T) {
	config := NewDefaultConfig()

	tests := []struct {
		name            string
		task            *models.Task
		expectedTimeout time.Duration
	}{
		{
			name: "Task with custom timeout",
			task: &models.Task{
				TimeoutSeconds: 600,
			},
			expectedTimeout: 600 * time.Second,
		},
		{
			name: "Task with zero timeout (use default)",
			task: &models.Task{
				TimeoutSeconds: 0,
			},
			expectedTimeout: 300 * time.Second,
		},
		{
			name: "Task with negative timeout (use default)",
			task: &models.Task{
				TimeoutSeconds: -1,
			},
			expectedTimeout: 300 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			timeout := config.GetTimeoutForTask(tt.task)
			assert.Equal(t, tt.expectedTimeout, timeout)
		})
	}
}

func TestConfig_GetSecurityConfigForTask(t *testing.T) {
	config := NewDefaultConfig()
	task := &models.Task{
		ScriptType: models.ScriptTypePython,
	}

	securityConfig := config.GetSecurityConfigForTask(task)

	assert.Equal(t, "1000:1000", securityConfig.User)
	assert.True(t, securityConfig.NoNewPrivileges)
	assert.True(t, securityConfig.ReadOnlyRootfs)
	assert.True(t, securityConfig.NetworkDisabled)
	assert.True(t, securityConfig.DropAllCapabilities)
	assert.Contains(t, securityConfig.SecurityOpts, "no-new-privileges")
	assert.NotEmpty(t, securityConfig.TmpfsMounts)
	assert.Contains(t, securityConfig.TmpfsMounts, "/tmp")
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name      string
		config    *Config
		expectErr bool
		errMsg    string
	}{
		{
			name:      "Valid default config",
			config:    NewDefaultConfig(),
			expectErr: false,
		},
		{
			name: "Invalid memory limit",
			config: &Config{
				DefaultResourceLimits: ResourceLimits{
					MemoryLimitBytes: 0,
					CPUQuota:         50000,
					PidsLimit:        128,
				},
				DefaultTimeoutSeconds: 300,
				Images: ImageConfig{
					Python: "python:3.11-alpine",
					Bash:   "alpine:latest",
				},
			},
			expectErr: true,
			errMsg:    "memory limit must be positive",
		},
		{
			name: "Invalid CPU quota",
			config: &Config{
				DefaultResourceLimits: ResourceLimits{
					MemoryLimitBytes: 128 * 1024 * 1024,
					CPUQuota:         0,
					PidsLimit:        128,
				},
				DefaultTimeoutSeconds: 300,
				Images: ImageConfig{
					Python: "python:3.11-alpine",
					Bash:   "alpine:latest",
				},
			},
			expectErr: true,
			errMsg:    "CPU quota must be positive",
		},
		{
			name: "Invalid PID limit",
			config: &Config{
				DefaultResourceLimits: ResourceLimits{
					MemoryLimitBytes: 128 * 1024 * 1024,
					CPUQuota:         50000,
					PidsLimit:        0,
				},
				DefaultTimeoutSeconds: 300,
				Images: ImageConfig{
					Python: "python:3.11-alpine",
					Bash:   "alpine:latest",
				},
			},
			expectErr: true,
			errMsg:    "PID limit must be positive",
		},
		{
			name: "Invalid timeout",
			config: &Config{
				DefaultResourceLimits: ResourceLimits{
					MemoryLimitBytes: 128 * 1024 * 1024,
					CPUQuota:         50000,
					PidsLimit:        128,
				},
				DefaultTimeoutSeconds: 0,
				Images: ImageConfig{
					Python: "python:3.11-alpine",
					Bash:   "alpine:latest",
				},
			},
			expectErr: true,
			errMsg:    "timeout must be positive",
		},
		{
			name: "Missing Python image",
			config: &Config{
				DefaultResourceLimits: ResourceLimits{
					MemoryLimitBytes: 128 * 1024 * 1024,
					CPUQuota:         50000,
					PidsLimit:        128,
				},
				DefaultTimeoutSeconds: 300,
				Images: ImageConfig{
					Python: "",
					Bash:   "alpine:latest",
				},
			},
			expectErr: true,
			errMsg:    "Python image must be specified",
		},
		{
			name: "Missing Bash image",
			config: &Config{
				DefaultResourceLimits: ResourceLimits{
					MemoryLimitBytes: 128 * 1024 * 1024,
					CPUQuota:         50000,
					PidsLimit:        128,
				},
				DefaultTimeoutSeconds: 300,
				Images: ImageConfig{
					Python: "python:3.11-alpine",
					Bash:   "",
				},
			},
			expectErr: true,
			errMsg:    "Bash image must be specified",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expectErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

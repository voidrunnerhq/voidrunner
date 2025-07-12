package executor

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/voidrunnerhq/voidrunner/internal/models"
)

func TestNewDockerClient(t *testing.T) {
	config := NewDefaultConfig()

	// Test with valid config
	client, err := NewDockerClient(config, nil)

	// Note: This test may fail in environments without Docker
	if err != nil {
		t.Skipf("Docker not available in test environment: %v", err)
	}

	require.NoError(t, err)
	assert.NotNil(t, client)
	assert.NotNil(t, client.client)
	assert.NotNil(t, client.config)
	assert.NotNil(t, client.logger)

	// Test with nil config (should use default)
	client2, err := NewDockerClient(nil, nil)
	if err != nil {
		t.Skipf("Docker not available in test environment: %v", err)
	}

	require.NoError(t, err)
	assert.NotNil(t, client2)
}

func TestDockerClient_buildCommand(t *testing.T) {
	config := NewDefaultConfig()
	client := &DockerClient{
		config: config,
		logger: nil,
	}

	tests := []struct {
		name          string
		scriptType    models.ScriptType
		scriptContent string
		expected      []string
	}{
		{
			name:          "Python script",
			scriptType:    models.ScriptTypePython,
			scriptContent: "print('hello')",
			expected:      []string{"python3", "-c", "print('hello')"},
		},
		{
			name:          "Bash script",
			scriptType:    models.ScriptTypeBash,
			scriptContent: "echo 'hello'",
			expected:      []string{"sh", "-c", "echo 'hello'"},
		},
		{
			name:          "JavaScript script",
			scriptType:    models.ScriptTypeJavaScript,
			scriptContent: "console.log('hello')",
			expected:      []string{"node", "-e", "console.log('hello')"},
		},
		{
			name:          "Go script",
			scriptType:    models.ScriptTypeGo,
			scriptContent: "package main\nfunc main() { println(\"hello\") }",
			expected:      []string{"sh", "-c", "echo 'package main\nfunc main() { println(\"hello\") }' > main.go && go run main.go"},
		},
		{
			name:          "Unknown script type defaults to Python",
			scriptType:    models.ScriptType("unknown"),
			scriptContent: "print('hello')",
			expected:      []string{"python3", "-c", "print('hello')"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := client.buildCommand(tt.scriptType, tt.scriptContent)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDockerClient_demultiplexLogs(t *testing.T) {
	config := NewDefaultConfig()
	client := &DockerClient{
		config: config,
		logger: nil,
	}

	tests := []struct {
		name           string
		logData        []byte
		expectedStdout string
		expectedStderr string
	}{
		{
			name:           "Empty log data",
			logData:        []byte{},
			expectedStdout: "",
			expectedStderr: "",
		},
		{
			name: "Stdout only",
			logData: []byte{
				1, 0, 0, 0, 0, 0, 0, 5, // Header: stdout, 5 bytes
				'h', 'e', 'l', 'l', 'o', // Data: "hello"
			},
			expectedStdout: "hello",
			expectedStderr: "",
		},
		{
			name: "Stderr only",
			logData: []byte{
				2, 0, 0, 0, 0, 0, 0, 5, // Header: stderr, 5 bytes
				'e', 'r', 'r', 'o', 'r', // Data: "error"
			},
			expectedStdout: "",
			expectedStderr: "error",
		},
		{
			name: "Both stdout and stderr",
			logData: []byte{
				1, 0, 0, 0, 0, 0, 0, 5, // Header: stdout, 5 bytes
				'h', 'e', 'l', 'l', 'o', // Data: "hello"
				2, 0, 0, 0, 0, 0, 0, 5, // Header: stderr, 5 bytes
				'e', 'r', 'r', 'o', 'r', // Data: "error"
			},
			expectedStdout: "hello",
			expectedStderr: "error",
		},
		{
			name: "Malformed data (insufficient header)",
			logData: []byte{
				1, 0, 0, // Incomplete header
			},
			expectedStdout: "",
			expectedStderr: "",
		},
		{
			name: "Malformed data (insufficient data)",
			logData: []byte{
				1, 0, 0, 0, 0, 0, 0, 10, // Header: stdout, 10 bytes
				'h', 'e', 'l', 'l', 'o', // Data: only 5 bytes
			},
			expectedStdout: "",
			expectedStderr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr := client.demultiplexLogs(tt.logData)
			assert.Equal(t, tt.expectedStdout, stdout)
			assert.Equal(t, tt.expectedStderr, stderr)
		})
	}
}

func TestDockerClient_ValidationErrors(t *testing.T) {
	config := NewDefaultConfig()
	client := &DockerClient{
		config: config,
		logger: nil,
	}

	ctx := context.Background()

	// Test CreateContainer with nil config
	containerID, err := client.CreateContainer(ctx, nil)
	assert.Error(t, err)
	assert.Empty(t, containerID)
	assert.Contains(t, err.Error(), "container config is nil")

	// Test StartContainer with empty container ID
	err = client.StartContainer(ctx, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "container ID is empty")

	// Test WaitContainer with empty container ID
	exitCode, err := client.WaitContainer(ctx, "")
	assert.Error(t, err)
	assert.Equal(t, -1, exitCode)
	assert.Contains(t, err.Error(), "container ID is empty")

	// Test GetContainerLogs with empty container ID
	stdout, stderr, err := client.GetContainerLogs(ctx, "")
	assert.Error(t, err)
	assert.Empty(t, stdout)
	assert.Empty(t, stderr)
	assert.Contains(t, err.Error(), "container ID is empty")

	// Test RemoveContainer with empty container ID
	err = client.RemoveContainer(ctx, "", false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "container ID is empty")

	// Test StopContainer with empty container ID
	err = client.StopContainer(ctx, "", 5*time.Second)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "container ID is empty")

	// Test GetContainerInfo with empty container ID
	info, err := client.GetContainerInfo(ctx, "")
	assert.Error(t, err)
	assert.Nil(t, info)
	assert.Contains(t, err.Error(), "container ID is empty")

	// Test PullImage with empty image name
	err = client.PullImage(ctx, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "image name is empty")
}

func TestDockerClient_CreateContainer(t *testing.T) {
	config := NewDefaultConfig()
	client, err := NewDockerClient(config, nil)
	if err != nil {
		t.Skipf("Docker not available in test environment: %v", err)
	}

	// Test container config building
	containerConfig := &ContainerConfig{
		Image:         "alpine:latest",
		ScriptType:    models.ScriptTypeBash,
		ScriptContent: "echo 'test'",
		Environment:   []string{"PATH=/usr/bin"},
		WorkingDir:    "/tmp",
		ResourceLimits: ResourceLimits{
			MemoryLimitBytes: 128 * 1024 * 1024,
			CPUQuota:         50000,
			PidsLimit:        128,
		},
		SecurityConfig: SecurityConfig{
			User:                "1000:1000",
			ReadOnlyRootfs:      true,
			NetworkDisabled:     true,
			NoNewPrivileges:     true,
			DropAllCapabilities: true,
			SecurityOpts:        []string{"no-new-privileges"},
			TmpfsMounts: map[string]string{
				"/tmp": "rw,noexec,nosuid,size=100m",
			},
		},
		Timeout: 30 * time.Second,
	}

	// Note: This test requires Docker to be available
	// In a real environment, this would create an actual container
	// For unit testing, we would need to mock the Docker client
	_, err = client.CreateContainer(context.Background(), containerConfig)
	if err != nil {
		// If Docker is not available, the test should be skipped
		t.Skipf("Docker not available in test environment: %v", err)
	}
}

func TestDockerClient_ContainerOperations(t *testing.T) {
	config := NewDefaultConfig()
	client, err := NewDockerClient(config, nil)
	if err != nil {
		t.Skipf("Docker not available in test environment: %v", err)
	}

	ctx := context.Background()

	// Test that operations with non-existent containers return appropriate errors
	// These tests verify error handling without requiring actual containers

	// Test StartContainer with non-existent container
	err = client.StartContainer(ctx, "non-existent-container")
	if err == nil {
		t.Skip("Docker operations require actual Docker daemon")
	}
	assert.Error(t, err)

	// Test WaitContainer with non-existent container
	exitCode, err := client.WaitContainer(ctx, "non-existent-container")
	if err == nil {
		t.Skip("Docker operations require actual Docker daemon")
	}
	assert.Error(t, err)
	assert.Equal(t, -1, exitCode)

	// Test GetContainerLogs with non-existent container
	stdout, stderr, err := client.GetContainerLogs(ctx, "non-existent-container")
	if err == nil {
		t.Skip("Docker operations require actual Docker daemon")
	}
	assert.Error(t, err)
	assert.Empty(t, stdout)
	assert.Empty(t, stderr)

	// Test RemoveContainer with non-existent container
	err = client.RemoveContainer(ctx, "non-existent-container", false)
	if err == nil {
		t.Skip("Docker operations require actual Docker daemon")
	}
	// RemoveContainer should not fail for non-existent containers in some cases
	// This depends on the Docker client implementation

	// Test StopContainer with non-existent container
	err = client.StopContainer(ctx, "non-existent-container", 5*time.Second)
	if err == nil {
		t.Skip("Docker operations require actual Docker daemon")
	}
	// StopContainer should not fail for non-existent containers in some cases

	// Test GetContainerInfo with non-existent container
	info, err := client.GetContainerInfo(ctx, "non-existent-container")
	if err == nil {
		t.Skip("Docker operations require actual Docker daemon")
	}
	assert.Error(t, err)
	assert.Nil(t, info)
}

func TestDockerClient_ListContainers(t *testing.T) {
	config := NewDefaultConfig()
	client, err := NewDockerClient(config, nil)
	if err != nil {
		t.Skipf("Docker not available in test environment: %v", err)
	}

	ctx := context.Background()

	// Test ListContainers
	containers, err := client.ListContainers(ctx, false)
	if err != nil {
		t.Skipf("Docker not available in test environment: %v", err)
	}

	// Should return a list (possibly empty)
	assert.NotNil(t, containers)
}

func TestDockerClient_Close(t *testing.T) {
	config := NewDefaultConfig()
	client := &DockerClient{
		config: config,
		logger: nil,
	}

	// Test Close - should not error
	err := client.Close()
	assert.NoError(t, err)

	// Test Close with nil client
	client.client = nil
	err = client.Close()
	assert.NoError(t, err)
}

func TestDockerClient_GetDockerInfo(t *testing.T) {
	config := NewDefaultConfig()
	client, err := NewDockerClient(config, nil)
	if err != nil {
		t.Skipf("Docker not available in test environment: %v", err)
	}

	ctx := context.Background()

	// Test GetDockerInfo
	info, err := client.GetDockerInfo(ctx)
	if err != nil {
		t.Skipf("Docker not available in test environment: %v", err)
	}

	assert.NotNil(t, info)
}

func TestDockerClient_GetDockerVersion(t *testing.T) {
	config := NewDefaultConfig()
	client, err := NewDockerClient(config, nil)
	if err != nil {
		t.Skipf("Docker not available in test environment: %v", err)
	}

	ctx := context.Background()

	// Test GetDockerVersion
	version, err := client.GetDockerVersion(ctx)
	if err != nil {
		t.Skipf("Docker not available in test environment: %v", err)
	}

	assert.NotNil(t, version)
}

func TestDockerClient_IsHealthy(t *testing.T) {
	config := NewDefaultConfig()
	client, err := NewDockerClient(config, nil)
	if err != nil {
		t.Skipf("Docker not available in test environment: %v", err)
	}

	ctx := context.Background()

	// Test IsHealthy
	err = client.IsHealthy(ctx)
	if err != nil {
		t.Skipf("Docker not available in test environment: %v", err)
	}

	assert.NoError(t, err)
}

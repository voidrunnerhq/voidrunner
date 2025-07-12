package executor

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/voidrunnerhq/voidrunner/internal/models"
)

// ExecutionResult represents the result of a code execution
type ExecutionResult struct {
	// Status of the execution
	Status models.ExecutionStatus

	// Return code from the executed script
	ReturnCode *int

	// Standard output from the execution
	Stdout *string

	// Standard error from the execution
	Stderr *string

	// Duration of the execution in milliseconds
	ExecutionTimeMs *int

	// Memory usage in bytes
	MemoryUsageBytes *int64

	// Time when execution started
	StartedAt *time.Time

	// Time when execution completed
	CompletedAt *time.Time
}

// ExecutionContext represents the context for executing a task
type ExecutionContext struct {
	// Task to execute
	Task *models.Task

	// Execution record
	Execution *models.TaskExecution

	// Context for cancellation
	Context context.Context

	// Maximum execution time
	Timeout time.Duration

	// Resource limits
	ResourceLimits ResourceLimits
}

// ResourceLimits defines resource constraints for execution
type ResourceLimits struct {
	// Memory limit in bytes
	MemoryLimitBytes int64

	// CPU quota (100000 = 1 CPU core)
	CPUQuota int64

	// Maximum number of processes/threads
	PidsLimit int64

	// Execution timeout
	TimeoutSeconds int
}

// TaskExecutor defines the interface for executing tasks in containers
type TaskExecutor interface {
	// Execute runs the given task and returns the execution result
	Execute(ctx context.Context, execCtx *ExecutionContext) (*ExecutionResult, error)

	// Cancel cancels a running execution
	Cancel(ctx context.Context, executionID uuid.UUID) error

	// IsHealthy checks if the executor is healthy and ready to execute tasks
	IsHealthy(ctx context.Context) error

	// Cleanup performs any necessary cleanup of resources
	Cleanup(ctx context.Context) error
}

// ContainerClient defines the interface for Docker operations
type ContainerClient interface {
	// CreateContainer creates a new container with the specified configuration
	CreateContainer(ctx context.Context, config *ContainerConfig) (string, error)

	// StartContainer starts the specified container
	StartContainer(ctx context.Context, containerID string) error

	// WaitContainer waits for the container to finish and returns the exit code
	WaitContainer(ctx context.Context, containerID string) (int, error)

	// GetContainerLogs retrieves logs from the specified container
	GetContainerLogs(ctx context.Context, containerID string) (stdout, stderr string, err error)

	// RemoveContainer removes the specified container
	RemoveContainer(ctx context.Context, containerID string, force bool) error

	// StopContainer stops the specified container
	StopContainer(ctx context.Context, containerID string, timeout time.Duration) error

	// IsHealthy checks if the Docker daemon is accessible
	IsHealthy(ctx context.Context) error

	// ListContainers returns a list of containers
	ListContainers(ctx context.Context, all bool) ([]ContainerSummary, error)

	// PullImage pulls a container image
	PullImage(ctx context.Context, image string) error

	// GetDockerInfo returns Docker system information
	GetDockerInfo(ctx context.Context) (interface{}, error)

	// GetDockerVersion returns Docker version information
	GetDockerVersion(ctx context.Context) (interface{}, error)
}

// ContainerConfig represents the configuration for creating a container
type ContainerConfig struct {
	// Container image to use
	Image string

	// Script type (python, bash, etc.)
	ScriptType models.ScriptType

	// Script content to execute
	ScriptContent string

	// Environment variables
	Environment []string

	// Working directory inside container
	WorkingDir string

	// Resource limits
	ResourceLimits ResourceLimits

	// Security configuration
	SecurityConfig SecurityConfig

	// Execution timeout
	Timeout time.Duration
}

// SecurityConfig represents security settings for container execution
type SecurityConfig struct {
	// Run as non-root user (UID:GID)
	User string

	// Disable privilege escalation
	NoNewPrivileges bool

	// Use read-only root filesystem
	ReadOnlyRootfs bool

	// Disable network access
	NetworkDisabled bool

	// Security options (seccomp, apparmor)
	SecurityOpts []string

	// Tmpfs mounts for writable directories
	TmpfsMounts map[string]string

	// Drop all capabilities
	DropAllCapabilities bool
}

// ContainerSummary represents a summary of container information
type ContainerSummary struct {
	ID      string
	Names   []string
	Image   string
	Created int64
	State   string
	Status  string
}

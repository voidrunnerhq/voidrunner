package executor

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/containerd/errdefs"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/voidrunnerhq/voidrunner/internal/models"
)

// DockerClient implements the ContainerClient interface
type DockerClient struct {
	client *client.Client
	config *Config
	logger *slog.Logger
}

// Container ID validation patterns
var (
	// Docker short container IDs are 12 character hex strings
	shortContainerIDPattern = regexp.MustCompile(`^[a-f0-9]{12,64}$`)
)

// NewDockerClient creates a new Docker client with the given configuration
func NewDockerClient(config *Config, logger *slog.Logger) (*DockerClient, error) {
	if config == nil {
		config = NewDefaultConfig()
	}

	if logger == nil {
		logger = slog.Default()
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	// Create Docker client
	cli, err := client.NewClientWithOpts(
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return nil, NewExecutorError("docker_client_init", "failed to create Docker client", err)
	}

	dockerClient := &DockerClient{
		client: cli,
		config: config,
		logger: logger,
	}

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := dockerClient.IsHealthy(ctx); err != nil {
		return nil, fmt.Errorf("docker health check failed: %w", err)
	}

	return dockerClient, nil
}

// validateContainerID performs comprehensive validation of a container ID
func (dc *DockerClient) validateContainerID(containerID string) error {
	if containerID == "" {
		return NewContainerError("", "validate_container_id", "container ID is empty", nil)
	}

	// Check for whitespace or control characters
	if strings.TrimSpace(containerID) != containerID {
		return NewContainerError(containerID, "validate_container_id", "container ID contains invalid whitespace", nil)
	}

	// Check minimum length (Docker allows partial IDs of at least 12 characters)
	if len(containerID) < 12 {
		return NewContainerError(containerID, "validate_container_id",
			fmt.Sprintf("container ID too short (%d characters), must be at least 12", len(containerID)), nil)
	}

	// Check maximum length (full Docker IDs are 64 characters)
	if len(containerID) > 64 {
		return NewContainerError(containerID, "validate_container_id",
			fmt.Sprintf("container ID too long (%d characters), must be at most 64", len(containerID)), nil)
	}

	// Check for valid hexadecimal characters
	if !shortContainerIDPattern.MatchString(containerID) {
		return NewContainerError(containerID, "validate_container_id",
			"container ID contains invalid characters, must be lowercase hexadecimal", nil)
	}

	return nil
}

// CreateContainer creates a new container with the specified configuration
func (dc *DockerClient) CreateContainer(ctx context.Context, config *ContainerConfig) (string, error) {
	if config == nil {
		return "", NewExecutorError("create_container", "container config is nil", nil)
	}

	// Build container configuration
	containerConfig := &container.Config{
		Image:        config.Image,
		User:         config.SecurityConfig.User,
		WorkingDir:   config.WorkingDir,
		Env:          config.Environment,
		AttachStdout: true,
		AttachStderr: true,
	}

	// Set command based on script type
	containerConfig.Cmd = dc.buildCommand(config.ScriptType, config.ScriptContent)

	// Build host configuration with security and resource limits
	hostConfig := &container.HostConfig{
		Resources: container.Resources{
			Memory:    config.ResourceLimits.MemoryLimitBytes,
			CPUQuota:  config.ResourceLimits.CPUQuota,
			PidsLimit: &config.ResourceLimits.PidsLimit,
		},
		SecurityOpt:    config.SecurityConfig.SecurityOpts,
		ReadonlyRootfs: config.SecurityConfig.ReadOnlyRootfs,
		AutoRemove:     true, // Automatically remove container when it exits
		Tmpfs:          config.SecurityConfig.TmpfsMounts,
	}

	// Disable networking if configured
	if config.SecurityConfig.NetworkDisabled {
		hostConfig.NetworkMode = "none"
	}

	// Drop all capabilities for security
	if config.SecurityConfig.DropAllCapabilities {
		hostConfig.CapDrop = []string{"ALL"}
	}

	// Create the container
	resp, err := dc.client.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, "")
	if err != nil {
		return "", NewContainerError("", "create_container", "failed to create container", err)
	}

	if len(resp.Warnings) > 0 {
		// Log warnings but don't fail
		for _, warning := range resp.Warnings {
			dc.logger.Warn("container creation warning", "warning", warning)
		}
	}

	return resp.ID, nil
}

// StartContainer starts the specified container
func (dc *DockerClient) StartContainer(ctx context.Context, containerID string) error {
	if err := dc.validateContainerID(containerID); err != nil {
		return fmt.Errorf("start_container validation failed: %w", err)
	}

	err := dc.client.ContainerStart(ctx, containerID, container.StartOptions{})
	if err != nil {
		return NewContainerError(containerID, "start_container", "failed to start container", err)
	}

	return nil
}

// WaitContainer waits for the container to finish and returns the exit code
func (dc *DockerClient) WaitContainer(ctx context.Context, containerID string) (int, error) {
	if err := dc.validateContainerID(containerID); err != nil {
		return -1, fmt.Errorf("wait_container validation failed: %w", err)
	}

	// Wait for container to finish
	statusCh, errCh := dc.client.ContainerWait(ctx, containerID, container.WaitConditionNotRunning)

	select {
	case err := <-errCh:
		if err != nil {
			return -1, NewContainerError(containerID, "wait_container", "error waiting for container", err)
		}
	case status := <-statusCh:
		return int(status.StatusCode), nil
	case <-ctx.Done():
		return -1, NewContainerError(containerID, "wait_container", "context cancelled", ctx.Err())
	}

	return -1, NewContainerError(containerID, "wait_container", "unexpected wait completion", nil)
}

// GetContainerLogs retrieves logs from the specified container
func (dc *DockerClient) GetContainerLogs(ctx context.Context, containerID string) (stdout, stderr string, err error) {
	if err := dc.validateContainerID(containerID); err != nil {
		return "", "", fmt.Errorf("get_container_logs validation failed: %w", err)
	}

	options := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     false,
		Timestamps: false,
	}

	logs, err := dc.client.ContainerLogs(ctx, containerID, options)
	if err != nil {
		return "", "", NewContainerError(containerID, "get_logs", "failed to get container logs", err)
	}
	defer logs.Close()

	// Read all logs
	logBytes, err := io.ReadAll(logs)
	if err != nil {
		return "", "", NewContainerError(containerID, "get_logs", "failed to read container logs", err)
	}

	// Docker multiplexes stdout and stderr in a single stream
	// We need to demultiplex them
	stdout, stderr = dc.demultiplexLogs(logBytes)

	return stdout, stderr, nil
}

// RemoveContainer removes the specified container
func (dc *DockerClient) RemoveContainer(ctx context.Context, containerID string, force bool) error {
	if err := dc.validateContainerID(containerID); err != nil {
		return fmt.Errorf("remove_container validation failed: %w", err)
	}

	options := container.RemoveOptions{
		Force:         force,
		RemoveVolumes: true,
	}

	err := dc.client.ContainerRemove(ctx, containerID, options)
	if err != nil {
		// Don't fail if container is already removed
		if errdefs.IsNotFound(err) {
			return nil
		}
		return NewContainerError(containerID, "remove_container", "failed to remove container", err)
	}

	return nil
}

// StopContainer stops the specified container
func (dc *DockerClient) StopContainer(ctx context.Context, containerID string, timeout time.Duration) error {
	if err := dc.validateContainerID(containerID); err != nil {
		return fmt.Errorf("stop_container validation failed: %w", err)
	}

	timeoutInt := int(timeout.Seconds())
	options := container.StopOptions{
		Timeout: &timeoutInt,
	}

	err := dc.client.ContainerStop(ctx, containerID, options)
	if err != nil {
		// Don't fail if container is already stopped
		if errdefs.IsNotFound(err) {
			return nil
		}
		return NewContainerError(containerID, "stop_container", "failed to stop container", err)
	}

	return nil
}

// IsHealthy checks if the Docker daemon is accessible
func (dc *DockerClient) IsHealthy(ctx context.Context) error {
	_, err := dc.client.Ping(ctx)
	if err != nil {
		return NewExecutorError("health_check", "Docker daemon is not accessible", err)
	}

	return nil
}

// Close closes the Docker client connection
func (dc *DockerClient) Close() error {
	if dc.client != nil {
		return dc.client.Close()
	}
	return nil
}

// buildCommand builds the appropriate command for the given script type and content
func (dc *DockerClient) buildCommand(scriptType models.ScriptType, scriptContent string) []string {
	switch scriptType {
	case models.ScriptTypePython:
		return []string{"python3", "-c", scriptContent}
	case models.ScriptTypeBash:
		return []string{"sh", "-c", scriptContent}
	case models.ScriptTypeJavaScript:
		return []string{"node", "-e", scriptContent}
	case models.ScriptTypeGo:
		// For Go, we'd need a more complex setup to compile and run
		// For now, treat it as a shell script that writes and compiles Go code
		return []string{"sh", "-c", fmt.Sprintf("echo '%s' > main.go && go run main.go", scriptContent)}
	default:
		// Default to Python
		return []string{"python3", "-c", scriptContent}
	}
}

// demultiplexLogs separates stdout and stderr from Docker's multiplexed log stream
func (dc *DockerClient) demultiplexLogs(logData []byte) (stdout, stderr string) {
	var stdoutBuilder, stderrBuilder strings.Builder

	i := 0
	for i < len(logData) {
		if i+8 > len(logData) {
			break
		}

		// Docker log format: [STREAM_TYPE][RESERVED][SIZE][DATA]
		// STREAM_TYPE: 1 byte (0=stdin, 1=stdout, 2=stderr)
		// RESERVED: 3 bytes
		// SIZE: 4 bytes (big-endian)
		// DATA: SIZE bytes

		streamType := logData[i]
		// Skip reserved bytes (i+1, i+2, i+3)
		size := int(logData[i+4])<<24 | int(logData[i+5])<<16 | int(logData[i+6])<<8 | int(logData[i+7])

		dataStart := i + 8
		dataEnd := dataStart + size

		if dataEnd > len(logData) {
			break
		}

		data := string(logData[dataStart:dataEnd])

		switch streamType {
		case 1: // stdout
			stdoutBuilder.WriteString(data)
		case 2: // stderr
			stderrBuilder.WriteString(data)
		}

		i = dataEnd
	}

	return stdoutBuilder.String(), stderrBuilder.String()
}

// GetContainerInfo returns information about a container
func (dc *DockerClient) GetContainerInfo(ctx context.Context, containerID string) (*container.InspectResponse, error) {
	if err := dc.validateContainerID(containerID); err != nil {
		return nil, fmt.Errorf("get_container_info validation failed: %w", err)
	}

	info, err := dc.client.ContainerInspect(ctx, containerID)
	if err != nil {
		return nil, NewContainerError(containerID, "get_info", "failed to inspect container", err)
	}

	return &info, nil
}

// ListContainers returns a list of containers
func (dc *DockerClient) ListContainers(ctx context.Context, all bool) ([]ContainerSummary, error) {
	options := container.ListOptions{
		All: all,
	}

	containers, err := dc.client.ContainerList(ctx, options)
	if err != nil {
		return nil, NewExecutorError("list_containers", "failed to list containers", err)
	}

	// Convert to our interface type
	summaries := make([]ContainerSummary, len(containers))
	for i, c := range containers {
		summaries[i] = ContainerSummary{
			ID:      c.ID,
			Names:   c.Names,
			Image:   c.Image,
			Created: c.Created,
			State:   c.State,
			Status:  c.Status,
		}
	}

	return summaries, nil
}

// PullImage pulls a container image
func (dc *DockerClient) PullImage(ctx context.Context, imageName string) error {
	if imageName == "" {
		return NewExecutorError("pull_image", "image name is empty", nil)
	}

	reader, err := dc.client.ImagePull(ctx, imageName, image.PullOptions{})
	if err != nil {
		return NewExecutorError("pull_image", "failed to pull image", err)
	}
	defer reader.Close()

	// Read the pull output to ensure completion
	_, err = io.ReadAll(reader)
	if err != nil {
		return NewExecutorError("pull_image", "failed to read pull output", err)
	}

	return nil
}

// GetDockerInfo returns Docker system information
func (dc *DockerClient) GetDockerInfo(ctx context.Context) (interface{}, error) {
	info, err := dc.client.Info(ctx)
	if err != nil {
		return nil, NewExecutorError("get_docker_info", "failed to get Docker info", err)
	}

	return info, nil
}

// GetDockerVersion returns Docker version information
func (dc *DockerClient) GetDockerVersion(ctx context.Context) (interface{}, error) {
	version, err := dc.client.ServerVersion(ctx)
	if err != nil {
		return nil, NewExecutorError("get_docker_version", "failed to get Docker version", err)
	}

	return version, nil
}

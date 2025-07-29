package executor

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/containerd/errdefs"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/google/uuid"
	"github.com/voidrunnerhq/voidrunner/internal/logging"
	"github.com/voidrunnerhq/voidrunner/internal/models"
)

// DockerClient implements the ContainerClient interface
type DockerClient struct {
	client *client.Client
	config *Config
	logger *slog.Logger

	// Log streaming support
	streamingService logging.StreamingService
	logStorage       logging.LogStorage
	activeStreams    map[string]*LogStream // containerID -> LogStream
	streamsMutex     sync.RWMutex
}

// LogStream represents an active container log stream
type LogStream struct {
	ContainerID string
	TaskID      uuid.UUID
	ExecutionID uuid.UUID
	StartTime   time.Time
	Cancel      context.CancelFunc
	Done        chan struct{}
}

// LogStreamingOptions configures log streaming behavior
type LogStreamingOptions struct {
	Follow     bool
	Timestamps bool
	Since      string
	Until      string
	Tail       string
}

// Container ID validation patterns
var (
	// Docker short container IDs are 12 character hex strings
	shortContainerIDPattern = regexp.MustCompile(`^[a-f0-9]{12,64}$`)
)

// NewDockerClient creates a new Docker client with the given configuration
func NewDockerClient(config *Config, logger *slog.Logger) (*DockerClient, error) {
	return NewDockerClientWithLogging(config, logger, nil, nil)
}

// NewDockerClientWithLogging creates a new Docker client with optional logging services
func NewDockerClientWithLogging(config *Config, logger *slog.Logger, streamingService logging.StreamingService, logStorage logging.LogStorage) (*DockerClient, error) {
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
		client:           cli,
		config:           config,
		logger:           logger,
		streamingService: streamingService,
		logStorage:       logStorage,
		activeStreams:    make(map[string]*LogStream),
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

// StartContainerLogStreaming starts streaming logs from a container in real-time
func (dc *DockerClient) StartContainerLogStreaming(ctx context.Context, containerID string, taskID uuid.UUID, executionID uuid.UUID, options LogStreamingOptions) error {
	if err := dc.validateContainerID(containerID); err != nil {
		return fmt.Errorf("start_log_streaming validation failed: %w", err)
	}

	if dc.streamingService == nil || dc.logStorage == nil {
		dc.logger.Debug("log streaming services not available, skipping", "container_id", containerID)
		return nil // Not an error - just means streaming is disabled
	}

	dc.streamsMutex.Lock()
	defer dc.streamsMutex.Unlock()

	// Check if already streaming for this container
	if _, exists := dc.activeStreams[containerID]; exists {
		return fmt.Errorf("log streaming already active for container %s", containerID)
	}

	// Create cancellable context for this stream
	streamCtx, cancel := context.WithCancel(ctx)

	// Start the streaming goroutine
	stream := &LogStream{
		ContainerID: containerID,
		TaskID:      taskID,
		ExecutionID: executionID,
		StartTime:   time.Now(),
		Cancel:      cancel,
		Done:        make(chan struct{}),
	}

	dc.activeStreams[containerID] = stream

	// Start streaming in background
	go dc.streamContainerLogs(streamCtx, stream, options)

	dc.logger.Info("started container log streaming",
		"container_id", containerID[:12],
		"task_id", taskID,
		"execution_id", executionID)

	return nil
}

// StopContainerLogStreaming stops streaming logs for a container
func (dc *DockerClient) StopContainerLogStreaming(containerID string) error {
	if err := dc.validateContainerID(containerID); err != nil {
		return fmt.Errorf("stop_log_streaming validation failed: %w", err)
	}

	dc.streamsMutex.Lock()
	defer dc.streamsMutex.Unlock()

	stream, exists := dc.activeStreams[containerID]
	if !exists {
		return nil // Already stopped or never started
	}

	// Cancel the streaming context
	stream.Cancel()

	// Wait for the streaming goroutine to finish
	select {
	case <-stream.Done:
		// Stream finished
	case <-time.After(5 * time.Second):
		dc.logger.Warn("timeout waiting for log stream to stop", "container_id", containerID[:12])
	}

	// Remove from active streams
	delete(dc.activeStreams, containerID)

	dc.logger.Info("stopped container log streaming", "container_id", containerID[:12])
	return nil
}

// IsStreamingLogs returns true if logs are being streamed for the container
func (dc *DockerClient) IsStreamingLogs(containerID string) bool {
	dc.streamsMutex.RLock()
	defer dc.streamsMutex.RUnlock()

	_, exists := dc.activeStreams[containerID]
	return exists
}

// GetActiveLogStreams returns the number of active log streams
func (dc *DockerClient) GetActiveLogStreams() int {
	dc.streamsMutex.RLock()
	defer dc.streamsMutex.RUnlock()

	return len(dc.activeStreams)
}

// streamContainerLogs handles the actual streaming of container logs
func (dc *DockerClient) streamContainerLogs(ctx context.Context, stream *LogStream, options LogStreamingOptions) {
	defer close(stream.Done)

	logger := dc.logger.With(
		"container_id", stream.ContainerID[:12],
		"task_id", stream.TaskID,
		"execution_id", stream.ExecutionID,
	)

	logger.Debug("starting container log stream processing")

	// Set up Docker log options
	dockerOptions := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     options.Follow,
		Timestamps: options.Timestamps,
		Since:      options.Since,
		Until:      options.Until,
		Tail:       options.Tail,
	}

	// Get log stream from Docker
	logs, err := dc.client.ContainerLogs(ctx, stream.ContainerID, dockerOptions)
	if err != nil {
		logger.Error("failed to get container log stream", "error", err)
		return
	}
	defer logs.Close()

	// Process log stream
	sequenceNumber := int64(1)
	scanner := bufio.NewScanner(logs)
	
	// Set a reasonable buffer size for log lines
	const maxLogLineSize = 64 * 1024 // 64KB per line
	buf := make([]byte, maxLogLineSize)
	scanner.Buffer(buf, maxLogLineSize)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			logger.Debug("log streaming context cancelled")
			return
		default:
		}

		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		// Parse Docker log format and create log entries
		entries := dc.parseDockerLogLine(line, stream.TaskID, stream.ExecutionID, sequenceNumber)
		
		for _, entry := range entries {
			// Send to streaming service
			if err := dc.streamingService.PublishLog(ctx, entry); err != nil {
				logger.Error("failed to publish log entry", "error", err)
			}

			// Store in database
			if err := dc.logStorage.StoreLogs(ctx, []logging.LogEntry{entry}); err != nil {
				logger.Error("failed to store log entry", "error", err)
			}

			sequenceNumber++
		}
	}

	if err := scanner.Err(); err != nil {
		logger.Error("error reading container log stream", "error", err)
	}

	logger.Debug("container log stream processing completed")
}

// parseDockerLogLine parses a Docker log line and creates LogEntry objects
func (dc *DockerClient) parseDockerLogLine(line []byte, taskID, executionID uuid.UUID, sequenceNumber int64) []logging.LogEntry {
	if len(line) < 8 {
		// Not enough data for Docker log format
		return nil
	}

	// Docker log format: [STREAM_TYPE][RESERVED][SIZE][DATA]
	// STREAM_TYPE: 1 byte (0=stdin, 1=stdout, 2=stderr)
	// RESERVED: 3 bytes
	// SIZE: 4 bytes (big-endian)
	// DATA: SIZE bytes

	streamType := line[0]
	size := int(line[4])<<24 | int(line[5])<<16 | int(line[6])<<8 | int(line[7])

	if len(line) < 8+size {
		// Invalid log format
		return nil
	}

	content := string(line[8 : 8+size])
	var stream string

	switch streamType {
	case 1:
		stream = "stdout"
	case 2:
		stream = "stderr"
	default:
		// Unknown stream type, default to stdout
		stream = "stdout"
	}

	// Split content by newlines to create separate log entries
	lines := strings.Split(strings.TrimRight(content, "\n\r"), "\n")
	entries := make([]logging.LogEntry, 0, len(lines))

	for i, logLine := range lines {
		if strings.TrimSpace(logLine) == "" {
			continue // Skip empty lines
		}

		entry := logging.LogEntry{
			TaskID:         taskID,
			ExecutionID:    executionID,
			Content:        logLine,
			Stream:         stream,
			SequenceNumber: sequenceNumber + int64(i),
			Timestamp:      time.Now(),
			CreatedAt:      time.Now(),
		}

		entries = append(entries, entry)
	}

	return entries
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
	// Stop all active log streams
	dc.streamsMutex.Lock()
	for containerID, stream := range dc.activeStreams {
		dc.logger.Debug("stopping log stream during shutdown", "container_id", containerID[:12])
		stream.Cancel()
		
		// Wait briefly for stream to stop
		select {
		case <-stream.Done:
		case <-time.After(2 * time.Second):
			dc.logger.Warn("timeout waiting for log stream to stop during shutdown", "container_id", containerID[:12])
		}
	}
	dc.activeStreams = make(map[string]*LogStream)
	dc.streamsMutex.Unlock()

	// Close Docker client
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

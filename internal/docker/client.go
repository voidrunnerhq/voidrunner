package docker

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/voidrunnerhq/voidrunner/internal/config"
)

// Client wraps the Docker SDK client and provides higher-level methods
// for interacting with Docker, tailored for Voidrunner's needs.
type Client struct {
	cli    *client.Client
	logger *slog.Logger
	config *config.DockerConfig
}

// NewClient creates and initializes a new Docker client.
// It will attempt to connect to the Docker daemon using environment variables
// (DOCKER_HOST, DOCKER_TLS_VERIFY, DOCKER_CERT_PATH) or default to the local socket.
// It can also use an explicit host from the config.
func NewClient(logger *slog.Logger, cfg *config.DockerConfig) (*Client, error) {
	opts := []client.Opt{client.FromEnv, client.WithAPIVersionNegotiation()}
	if cfg.Host != "" {
		opts = append(opts, client.WithHost(cfg.Host))
		logger.Info("using explicit Docker host from config", "host", cfg.Host)
	} else {
		logger.Info("using Docker host from environment (DOCKER_HOST or default socket)")
	}

	cli, err := client.NewClientWithOpts(opts...)
	if err != nil {
		logger.Error("failed to create Docker client", "error", err)
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	// Ping the Docker daemon to ensure connectivity
	// Use a context with a timeout for the ping operation
	pingCtx, cancel := context.WithTimeout(context.Background(), cfg.DefaultExecTimeout)
	defer cancel()

	ping, err := cli.Ping(pingCtx)
	if err != nil {
		logger.Error("failed to ping Docker daemon", "error", err)
		return nil, fmt.Errorf("failed to connect to Docker daemon: %w", err)
	}

	logger.Info("successfully connected to Docker daemon",
		"api_version", ping.APIVersion,
		"os_type", ping.OSType,
		"experimental", ping.Experimental,
		"builder_version", ping.BuilderVersion,
	)

	return &Client{
		cli:    cli,
		logger: logger,
		config: cfg,
	}, nil
}

// Close closes the Docker client connection.
// The underlying Docker client doesn't always require explicit closing for HTTP connections,
// but it's good practice if other transport types are used or for future-proofing.
func (c *Client) Close() error {
	if c.cli != nil {
		// client.Client.Close() is a method on the client struct
		// but it might not do much for the default HTTP transport.
		// For now, we'll call it if it exists and doesn't error.
		// return c.cli.Close() // This method doesn't exist on the version being used.
		// For now, there's no explicit close needed for the default client.
	}
	return nil
}

// GetClient returns the underlying Docker SDK client if direct access is needed.
func (c *Client) GetClient() *client.Client {
	return c.cli
}

const (
	DefaultAPIVersion = "1.41" // A common Docker API version, can be made configurable
	DefaultTimeout    = 30 * time.Second
)

// ExecuteContainerParams groups all parameters for executing a command in a container.
type ExecuteContainerParams struct {
	Ctx          context.Context
	ImageName    string
	Cmd          []string
	UserCode     string   // Used if Cmd needs to incorporate user code, e.g. python -c "USER_CODE"
	EnvVars      []string // e.g., ["VAR1=value1", "VAR2=value2"]
	User         string   // e.g., "1000:1000"
	WorkingDir   string   // e.g., "/tmp/workspace"
	SeccompProfile string   // JSON content of the seccomp profile or path to it (if handled by Docker daemon)

	// Resource Limits
	MemoryMB  int64 // Memory limit in MB
	CPUQuota  int64 // CPU quota in microseconds (e.g., 50000 for 0.5 CPU)
	PidsLimit int64 // PID limit

	// Network
	NetworkMode string // e.g., "none", "bridge"

	// Filesystem
	ReadOnlyRootfs bool
	Tmpfs          map[string]string // e.g., {"/tmp": "rw,noexec,nosuid,size=100m"}

	// AutoRemove container after execution
	AutoRemove bool
}

// PullImageIfNotExists pulls a Docker image if it's not already present locally.
func (c *Client) PullImageIfNotExists(ctx context.Context, imageName string) error {
	_, _, err := c.cli.ImageInspectWithRaw(ctx, imageName)
	if err == nil {
		c.logger.Info("image already exists locally", "image", imageName)
		return nil // Image exists
	}

	if !client.IsErrNotFound(err) {
		c.logger.Error("failed to inspect image", "image", imageName, "error", err)
		return fmt.Errorf("failed to inspect image %s: %w", imageName, err)
	}

	c.logger.Info("image not found locally, pulling from registry", "image", imageName)
	reader, err := c.cli.ImagePull(ctx, imageName, image.PullOptions{})
	if err != nil {
		c.logger.Error("failed to pull image", "image", imageName, "error", err)
		return fmt.Errorf("failed to pull image %s: %w", imageName, err)
	}
	defer reader.Close()

	// Pipe the pull output to stdout or a logger
	// For simplicity, we're sending to os.Stdout; in a real app, use a structured logger
	if _, err := io.Copy(os.Stdout, reader); err != nil {
		c.logger.Warn("failed to stream image pull output", "image", imageName, "error", err)
		// Not returning error here as the pull might have succeeded partially or fully
	}

	c.logger.Info("successfully pulled image", "image", imageName)
	return nil
}

// ptr is a helper function to get a pointer to a value.
// Useful for Docker SDK fields that require pointers (e.g., PidsLimit).
func ptr[T any](v T) *T {
	return &v
}


// CreateAndStartContainer creates and starts a container based on the provided parameters.
// It returns the container ID and any error encountered.
// This combines container creation and starting for simplicity in the service layer.
func (c *Client) CreateAndStartContainer(params *ExecuteContainerParams) (string, error) {
	err := c.PullImageIfNotExists(params.Ctx, params.ImageName)
	if err != nil {
		return "", err // Error already logged by PullImageIfNotExists
	}

	finalCmd := params.Cmd
	// If UserCode is provided and Cmd is structured for it (e.g., ["python3", "-c", "%CODE%"])
	// Replace placeholder with actual user code. This is a simple approach.
	// A more robust solution might involve templating or specific logic per executor type.
	if params.UserCode != "" {
		for i, segment := range finalCmd {
			if strings.Contains(segment, "%CODE%") { // Define a clear placeholder convention
				finalCmd[i] = strings.Replace(segment, "%CODE%", params.UserCode, 1)
			}
		}
	}


	containerConfig := &container.Config{
		Image:        params.ImageName,
		Cmd:          finalCmd,
		User:         params.User,
		WorkingDir:   params.WorkingDir,
		Env:          append(os.Environ(), params.EnvVars...), // Inherit env and add custom ones
		Tty:          false, // Typically false for non-interactive execution
		AttachStdin:  false,
		AttachStdout: true,
		AttachStderr: true,
		OpenStdin:    false,
		// ExposedPorts, Volumes, etc. can be added if needed, but for secure execution, they are often minimal.
	}

	hostConfig := &container.HostConfig{
		Resources: container.Resources{
			Memory:    params.MemoryMB * 1024 * 1024, // Convert MB to bytes
			CPUQuota:  params.CPUQuota,
			PidsLimit: ptr(params.PidsLimit),
		},
		SecurityOpt:    []string{"no-new-privileges"}, // Default security option
		NetworkMode:    container.NetworkMode(params.NetworkMode),
		ReadonlyRootfs: params.ReadOnlyRootfs,
		Tmpfs:          params.Tmpfs,
		AutoRemove:     params.AutoRemove,
		// Mounts: if persistent storage or specific file access is needed.
		// For seccomp, Docker expects the profile content or a path recognized by the daemon.
		// If params.SeccompProfile is the JSON content:
		// This is complex because the client can't directly pass JSON content for seccomp.
		// The profile usually needs to be on the Docker host.
		// A common pattern is to have pre-configured profiles on the host referenced by name/path.
		// Or, for some setups, the daemon might allow loading it (requires specific config).
		// For now, we assume seccomp profile path is handled by Docker daemon configuration or a pre-loaded profile name.
		// If params.SeccompProfile is a path like "/opt/voidrunner/seccomp-profile.json" (on the host),
		// it can be added to SecurityOpt.
	}

	if params.SeccompProfile != "" {
		// Assuming SeccompProfile is a path known to the Docker daemon
		// or a pre-loaded profile name.
		// Example: "seccomp=/path/to/profile.json" or "seccomp=unconfined" (not recommended for security)
		// The issue specifies "/opt/voidrunner/seccomp-profile.json"
		hostConfig.SecurityOpt = append(hostConfig.SecurityOpt, fmt.Sprintf("seccomp=%s", params.SeccompProfile))
	}


	// Create the container
	resp, err := c.cli.ContainerCreate(params.Ctx, containerConfig, hostConfig, nil, nil, "") // No specific networking config, no platform, no container name
	if err != nil {
		c.logger.Error("failed to create container", "image", params.ImageName, "error", err)
		return "", fmt.Errorf("failed to create container for image %s: %w", params.ImageName, err)
	}
	c.logger.Info("container created successfully", "id", resp.ID, "image", params.ImageName)

	// Start the container
	if err := c.cli.ContainerStart(params.Ctx, resp.ID, container.StartOptions{}); err != nil {
		c.logger.Error("failed to start container", "id", resp.ID, "error", err)
		// Attempt to remove the created container if start fails and AutoRemove is not set (or might fail)
		// This is a best-effort cleanup.
		if !params.AutoRemove {
			rmErr := c.cli.ContainerRemove(context.Background(), resp.ID, container.RemoveOptions{Force: true})
			if rmErr != nil {
				c.logger.Error("failed to remove container after start failure", "id", resp.ID, "remove_error", rmErr)
			}
		}
		return "", fmt.Errorf("failed to start container %s: %w", resp.ID, err)
	}

	c.logger.Info("container started successfully", "id", resp.ID)
	return resp.ID, nil
}

// WaitForContainer waits for a container to complete and returns its exit code.
func (c *Client) WaitForContainer(ctx context.Context, containerID string) (int64, error) {
	statusCh, errCh := c.cli.ContainerWait(ctx, containerID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			c.logger.Error("error waiting for container", "id", containerID, "error", err)
			return -1, fmt.Errorf("error waiting for container %s: %w", containerID, err)
		}
	case status := <-statusCh:
		c.logger.Info("container finished", "id", containerID, "status_code", status.StatusCode, "error", status.Error)
		if status.Error != nil {
			// This error is from the container's execution, not the wait operation itself.
			return status.StatusCode, fmt.Errorf("container %s exited with error: %s", containerID, status.Error.Message)
		}
		return status.StatusCode, nil
	}
	// Should not be reached if context is managed correctly (e.g. with timeout)
	return -1, fmt.Errorf("container wait for %s did not complete as expected", containerID)
}

// GetContainerLogs retrieves the stdout and stderr logs from a container.
func (c *Client) GetContainerLogs(ctx context.Context, containerID string) (stdout, stderr string, err error) {
	options := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     false, // Get logs up to current point, don't stream
		Timestamps: true,  // Good for debugging
	}

	logReader, err := c.cli.ContainerLogs(ctx, containerID, options)
	if err != nil {
		c.logger.Error("failed to retrieve container logs", "id", containerID, "error", err)
		return "", "", fmt.Errorf("failed to retrieve logs for container %s: %w", containerID, err)
	}
	defer logReader.Close()

	// Docker multiplexes stdout and stderr streams. We need to demultiplex them.
	// The stdcopy.StdCopy function does this.
	// For simplicity, we'll use strings.Builder. For large logs, consider streaming or temporary files.
	var stdoutBuf, stderrBuf strings.Builder

	// The log stream from Docker SDK includes a header indicating stream type (stdout/stderr) and length.
	// We need to demultiplex this. The `stdcopy.StdCopy` function is designed for this.
	// However, it requires separate io.Writer for stdout and stderr.
	// A simpler way for non-TTY logs (which is our case) is to read the raw stream.
	// The raw stream might interleave stdout and stderr if TTY is false and streams are not separated by Docker.
	// If TTY is false (as in our config), ContainerLogs returns a single stream for stdout and stderr.
	// The issue description implies a non-TTY setup.
	// A common approach is to capture all output and then decide if it's an error based on exit code.
	// For now, let's assume combined output. If separation is critical, stdcopy.StdCopy is the way.

	// Simpler approach: read all logs into one buffer if TTY is false.
	// If separate stdout/stderr is strictly needed without TTY, then `ContainerAttach` might be
	// more suitable before starting, but `ContainerLogs` is simpler for post-execution retrieval.
	// The issue's example config doesn't use TTY.
	// Let's assume for now that logs are combined or primarily stdout.
	// For robust separation, one would use `hijackedResponse.Reader` from `ContainerAttach`
	// and `stdcopy.StdCopy` to demultiplex.
	// Given `AttachStdout: true`, `AttachStderr: true`, `Tty: false`,
	// the logs are multiplexed. We should use `stdcopy.StdCopy`.

	// Correct way to demultiplex logs:
	_, err = DemultiplexStream(&stdoutBuf, &stderrBuf, logReader)
	if err != nil && err != io.EOF { // EOF is expected
		c.logger.Error("failed to demultiplex container logs", "id", containerID, "error", err)
		// Return what we have, plus the error
		return stdoutBuf.String(), stderrBuf.String(), fmt.Errorf("failed to demultiplex logs for container %s: %w", containerID, err)
	}

	c.logger.Info("retrieved container logs", "id", containerID, "stdout_len", stdoutBuf.Len(), "stderr_len", stderrBuf.Len())
	return stdoutBuf.String(), stderrBuf.String(), nil
}


// RemoveContainer removes a container.
// It includes options to force removal and remove associated anonymous volumes.
func (c *Client) RemoveContainer(ctx context.Context, containerID string, force bool, removeVolumes bool) error {
	options := container.RemoveOptions{
		Force:         force,
		RemoveVolumes: removeVolumes,
	}
	if err := c.cli.ContainerRemove(ctx, containerID, options); err != nil {
		// Don't error if container is already gone (e.g. due to AutoRemove)
		if client.IsErrNotFound(err) {
			c.logger.Info("container not found for removal, likely already removed", "id", containerID)
			return nil
		}
		c.logger.Error("failed to remove container", "id", containerID, "error", err)
		return fmt.Errorf("failed to remove container %s: %w", containerID, err)
	}
	c.logger.Info("container removed successfully", "id", containerID)
	return nil
}


// StopContainer stops a running container.
// Timeout specifies how long to wait for a graceful stop before SIGKILL.
// Docker's default is 10 seconds if nil.
func (c *Client) StopContainer(ctx context.Context, containerID string, timeout *time.Duration) error {
	var t *int
	if timeout != nil {
		timeoutSeconds := int(timeout.Seconds())
		t = &timeoutSeconds
	}

	// ContainerStop sends SIGTERM then SIGKILL after timeout.
	// The timeout parameter for ContainerStop is an *int representing seconds.
	// We need to convert time.Duration to *int (seconds).
	// The SDK's `container.StopOptions` has `Timeout` as `*int`.
	stopOptions := container.StopOptions{}
	if t != nil {
		stopOptions.Timeout = t
	}


	if err := c.cli.ContainerStop(ctx, containerID, stopOptions); err != nil {
		// Ignore "not found" if container already stopped and possibly removed (e.g. AutoRemove)
		if client.IsErrNotFound(err) {
			c.logger.Info("container not found for stopping, likely already stopped/removed", "id", containerID)
			return nil
		}
		// Ignore "container not running" errors if we are trying to stop it.
		// This can happen in race conditions or if it exited quickly.
		if strings.Contains(err.Error(), "is not running") {
			 c.logger.Warn("attempted to stop container that was not running", "id", containerID, "error", err)
			 return nil
		}
		c.logger.Error("failed to stop container", "id", containerID, "error", err)
		return fmt.Errorf("failed to stop container %s: %w", containerID, err)
	}
	c.logger.Info("container stopped successfully", "id", containerID)
	return nil
}

// DemultiplexStream copies demultiplexed Docker logs to stdout and stderr writers.
// Docker stream format: https://docs.docker.com/engine/api/v1.41/#tag/Container/operation/ContainerAttach
// [8-byte header][payload]
// Header: {STREAM_TYPE, 0, 0, 0, SIZE1, SIZE2, SIZE3, SIZE4}
// STREAM_TYPE: 0 for stdin, 1 for stdout, 2 for stderr
// SIZE: Unsigned 32-bit integer (BigEndian) representing payload size.
func DemultiplexStream(stdout, stderr io.Writer, stream io.Reader) (written int64, err error) {
	header := make([]byte, 8)
	var count int64

	for {
		// Read the 8-byte header
		n, err := io.ReadFull(stream, header)
		if err == io.EOF {
			return count, nil // Clean EOF means no more data
		}
		if err != nil {
			return count, fmt.Errorf("failed to read stream header (read %d bytes): %w", n, err)
		}
		count += int64(n)

		// Determine stream type and size
		var destWriter io.Writer
		streamType := header[0]
		switch streamType {
		case 1: // stdout
			destWriter = stdout
		case 2: // stderr
			destWriter = stderr
		default:
			// Potentially stdin (0) or other types. For logs, we usually only care about stdout/stderr.
			// If we encounter an unexpected stream type, we can skip or error.
			// For now, let's try to skip the payload if it's not stdout/stderr.
			payloadSize := (uint32(header[4]) << 24) | (uint32(header[5]) << 16) | (uint32(header[6]) << 8) | uint32(header[7])
			if payloadSize > 0 {
				nDiscard, discardErr := io.CopyN(io.Discard, stream, int64(payloadSize))
				count += nDiscard
				if discardErr != nil {
					return count, fmt.Errorf("failed to discard payload for unexpected stream type %d: %w", streamType, discardErr)
				}
			}
			continue // Move to next header
		}

		payloadSize := (uint32(header[4]) << 24) | (uint32(header[5]) << 16) | (uint32(header[6]) << 8) | uint32(header[7])

		if payloadSize == 0 {
			continue // No payload for this header, though unusual for stdout/stderr
		}

		// Copy the payload to the appropriate writer
		// Use io.LimitReader to avoid reading past the current payload
		lr := io.LimitReader(stream, int64(payloadSize))
		nCopied, err := io.Copy(destWriter, lr)
		count += nCopied
		if err != nil {
			return count, fmt.Errorf("failed to copy payload for stream type %d (copied %d bytes of %d): %w", streamType, nCopied, payloadSize, err)
		}

		if nCopied != int64(payloadSize) {
			return count, fmt.Errorf("payload size mismatch for stream type %d: expected %d, copied %d", streamType, payloadSize, nCopied)
		}
	}
}



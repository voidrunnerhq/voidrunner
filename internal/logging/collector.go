package logging

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/google/uuid"
)

// DockerLogCollector implements LogCollector using Docker API
type DockerLogCollector struct {
	dockerClient     *client.Client
	streamingService StreamingService
	logStorage       LogStorage
	config           *LogConfig
	logger           *slog.Logger

	// Active streams tracking
	activeStreams map[string]*StreamInfo
	mu            sync.RWMutex

	// Background goroutines
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Metrics
	totalLinesCollected int64
	lastCollectionTime  time.Time
	errorCount          int64
}

// NewDockerLogCollector creates a new Docker-based log collector
func NewDockerLogCollector(
	dockerClient *client.Client,
	streamingService StreamingService,
	logStorage LogStorage,
	config *LogConfig,
	logger *slog.Logger,
) (*DockerLogCollector, error) {
	if dockerClient == nil {
		return nil, fmt.Errorf("docker client is required")
	}

	if config == nil {
		config = DefaultLogConfig()
	}

	if logger == nil {
		logger = slog.Default()
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid log config: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	collector := &DockerLogCollector{
		dockerClient:     dockerClient,
		streamingService: streamingService,
		logStorage:       logStorage,
		config:           config,
		logger:           logger.With("component", "log_collector"),
		activeStreams:    make(map[string]*StreamInfo),
		ctx:              ctx,
		cancel:           cancel,
	}

	return collector, nil
}

// StartStreaming begins collecting logs from a container and forwards them
func (c *DockerLogCollector) StartStreaming(ctx context.Context, containerID string, taskID uuid.UUID, executionID uuid.UUID) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if already streaming
	if stream, exists := c.activeStreams[containerID]; exists && stream.IsActive {
		return fmt.Errorf("already streaming logs for container %s", containerID)
	}

	// Check concurrent stream limit
	if len(c.activeStreams) >= c.config.MaxConcurrentStreams {
		return fmt.Errorf("maximum concurrent streams reached (%d)", c.config.MaxConcurrentStreams)
	}

	// Create stream info
	streamInfo := &StreamInfo{
		ContainerID:    containerID,
		TaskID:         taskID,
		ExecutionID:    executionID,
		StartTime:      time.Now(),
		LinesCollected: 0,
		IsActive:       true,
	}

	c.activeStreams[containerID] = streamInfo

	// Start streaming in background
	c.wg.Add(1)
	go c.streamContainerLogs(ctx, streamInfo)

	c.logger.Info("started log streaming",
		"container_id", containerID,
		"task_id", taskID,
		"execution_id", executionID,
		"active_streams", len(c.activeStreams))

	return nil
}

// IsStreaming returns true if actively collecting logs for the container
func (c *DockerLogCollector) IsStreaming(containerID string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	stream, exists := c.activeStreams[containerID]
	return exists && stream.IsActive
}

// StopStreaming stops log collection for a specific container
func (c *DockerLogCollector) StopStreaming(containerID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	stream, exists := c.activeStreams[containerID]
	if !exists {
		return fmt.Errorf("no active stream found for container %s", containerID)
	}

	// Mark as inactive
	stream.IsActive = false

	c.logger.Info("stopped log streaming",
		"container_id", containerID,
		"task_id", stream.TaskID,
		"lines_collected", stream.LinesCollected,
		"duration", time.Since(stream.StartTime))

	return nil
}

// GetActiveStreams returns the number of active log streams
func (c *DockerLogCollector) GetActiveStreams() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	activeCount := 0
	for _, stream := range c.activeStreams {
		if stream.IsActive {
			activeCount++
		}
	}
	return activeCount
}

// Close shuts down all active log streams
func (c *DockerLogCollector) Close() error {
	c.logger.Info("shutting down log collector")

	// Cancel context to stop all streaming goroutines
	c.cancel()

	// Mark all streams as inactive
	c.mu.Lock()
	for _, stream := range c.activeStreams {
		stream.IsActive = false
	}
	c.mu.Unlock()

	// Wait for all goroutines to finish
	c.wg.Wait()

	c.logger.Info("log collector shutdown complete")
	return nil
}

// streamContainerLogs streams logs from a specific container
func (c *DockerLogCollector) streamContainerLogs(ctx context.Context, streamInfo *StreamInfo) {
	defer c.wg.Done()

	// Set up log options
	logOptions := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		Timestamps: true,
	}

	// Get container logs stream
	logs, err := c.dockerClient.ContainerLogs(ctx, streamInfo.ContainerID, logOptions)
	if err != nil {
		c.logger.Error("failed to get container logs",
			"container_id", streamInfo.ContainerID,
			"task_id", streamInfo.TaskID,
			"error", err)
		c.mu.Lock()
		c.errorCount++
		streamInfo.IsActive = false
		c.mu.Unlock()
		return
	}
	defer func() {
		if closeErr := logs.Close(); closeErr != nil {
			c.logger.Error("failed to close container log stream", "container_id", streamInfo.ContainerID, "error", closeErr)
		}
	}()

	// Process log stream
	if err := c.processLogStream(ctx, logs, streamInfo); err != nil {
		c.logger.Error("error processing log stream",
			"container_id", streamInfo.ContainerID,
			"task_id", streamInfo.TaskID,
			"error", err)
	}

	// Mark stream as inactive when done
	c.mu.Lock()
	streamInfo.IsActive = false
	delete(c.activeStreams, streamInfo.ContainerID)
	c.mu.Unlock()

	c.logger.Debug("log streaming completed",
		"container_id", streamInfo.ContainerID,
		"task_id", streamInfo.TaskID,
		"lines_collected", streamInfo.LinesCollected,
		"duration", time.Since(streamInfo.StartTime))
}

// processLogStream processes the Docker log stream and forwards entries
func (c *DockerLogCollector) processLogStream(ctx context.Context, logs io.ReadCloser, streamInfo *StreamInfo) error {
	scanner := bufio.NewScanner(logs)
	sequenceNumber := int64(1)

	// Create a batch for storage
	batch := make([]LogEntry, 0, c.config.BatchInsertSize)
	batchTimer := time.NewTicker(c.config.BatchInsertInterval)
	defer batchTimer.Stop()

	for {
		select {
		case <-ctx.Done():
			// Flush any remaining batch
			if len(batch) > 0 {
				c.flushBatch(ctx, batch)
			}
			return ctx.Err()

		case <-c.ctx.Done():
			// Service shutting down
			if len(batch) > 0 {
				c.flushBatch(ctx, batch)
			}
			return nil

		case <-batchTimer.C:
			// Flush batch on timer
			if len(batch) > 0 {
				c.flushBatch(ctx, batch)
				batch = batch[:0] // Reset batch
			}

		default:
			// Check if stream is still active
			c.mu.RLock()
			isActive := streamInfo.IsActive
			c.mu.RUnlock()

			if !isActive {
				if len(batch) > 0 {
					c.flushBatch(ctx, batch)
				}
				return nil
			}

			// Try to scan next line
			if !scanner.Scan() {
				if err := scanner.Err(); err != nil {
					c.mu.Lock()
					c.errorCount++
					c.mu.Unlock()
					return fmt.Errorf("scanner error: %w", err)
				}
				// End of stream
				if len(batch) > 0 {
					c.flushBatch(ctx, batch)
				}
				return nil
			}

			// Parse log line
			logLine := scanner.Text()
			entry, err := c.parseDockerLogLine(logLine, streamInfo, sequenceNumber)
			if err != nil {
				c.logger.Warn("failed to parse log line",
					"container_id", streamInfo.ContainerID,
					"error", err,
					"line", logLine)
				continue
			}

			// Update metrics
			c.mu.Lock()
			streamInfo.LinesCollected++
			c.totalLinesCollected++
			c.lastCollectionTime = time.Now()
			c.mu.Unlock()

			// Send to streaming service (real-time)
			if c.streamingService != nil {
				if err := c.streamingService.PublishLog(ctx, *entry); err != nil {
					c.logger.Warn("failed to publish log to streaming service",
						"container_id", streamInfo.ContainerID,
						"error", err)
				}
			}

			// Add to batch for storage
			if c.logStorage != nil {
				batch = append(batch, *entry)

				// Flush batch if it's full
				if len(batch) >= c.config.BatchInsertSize {
					c.flushBatch(ctx, batch)
					batch = batch[:0] // Reset batch
				}
			}

			sequenceNumber++
		}
	}
}

// parseDockerLogLine parses a Docker log line and creates a LogEntry
func (c *DockerLogCollector) parseDockerLogLine(logLine string, streamInfo *StreamInfo, sequenceNumber int64) (*LogEntry, error) {
	if len(logLine) < 8 {
		return nil, fmt.Errorf("log line too short")
	}

	// Docker log format: 8-byte header + content
	// Header: [STREAM_TYPE][0][0][0][SIZE_BYTES_4]
	header := logLine[:8]
	content := logLine[8:]

	// Determine stream type from header
	var streamType string
	switch header[0] {
	case 1:
		streamType = "stdout"
	case 2:
		streamType = "stderr"
	default:
		// If we can't parse the header, assume stdout and use the whole line
		streamType = "stdout"
		content = logLine
	}

	// Truncate content if it exceeds max size
	if len(content) > c.config.MaxLogLineSize {
		content = content[:c.config.MaxLogLineSize-3] + "..."
	}

	// Create log entry
	entry := &LogEntry{
		TaskID:         streamInfo.TaskID,
		ExecutionID:    streamInfo.ExecutionID,
		Content:        content,
		Stream:         streamType,
		SequenceNumber: sequenceNumber,
		Timestamp:      time.Now(),
		CreatedAt:      time.Now(),
	}

	// Validate entry
	if err := entry.Validate(); err != nil {
		return nil, fmt.Errorf("invalid log entry: %w", err)
	}

	return entry, nil
}

// flushBatch sends a batch of log entries to storage
func (c *DockerLogCollector) flushBatch(ctx context.Context, batch []LogEntry) {
	if len(batch) == 0 {
		return
	}

	if c.logStorage == nil {
		return
	}

	if err := c.logStorage.StoreLogs(ctx, batch); err != nil {
		c.logger.Error("failed to store log batch",
			"batch_size", len(batch),
			"error", err)
		c.mu.Lock()
		c.errorCount++
		c.mu.Unlock()
	} else {
		c.logger.Debug("stored log batch", "batch_size", len(batch))
	}
}

// GetStats returns statistics about the log collector
func (c *DockerLogCollector) GetStats() *LogCollectorStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	stats := &LogCollectorStats{
		ActiveStreams:       c.GetActiveStreams(),
		TotalLinesCollected: c.totalLinesCollected,
		StreamsByContainer:  make(map[string]StreamInfo),
	}

	// Copy stream info
	for containerID, stream := range c.activeStreams {
		stats.StreamsByContainer[containerID] = *stream
	}

	// Calculate collection rate
	if !c.lastCollectionTime.IsZero() {
		duration := time.Since(c.lastCollectionTime)
		if duration > 0 {
			stats.CollectionRate = float64(c.totalLinesCollected) / duration.Seconds()
		}
	}

	return stats
}
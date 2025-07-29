package logging

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// StreamingService manages real-time log distribution to subscribers
type StreamingService interface {
	// Subscribe creates a new log subscription for a task
	// Returns a channel that will receive log entries and an error if subscription fails
	Subscribe(ctx context.Context, taskID uuid.UUID, userID uuid.UUID) (<-chan LogEntry, error)

	// Unsubscribe removes a log subscription
	// The channel should be closed after calling this method
	Unsubscribe(taskID uuid.UUID, ch <-chan LogEntry) error

	// PublishLog sends a log entry to all subscribers of the task
	PublishLog(ctx context.Context, entry LogEntry) error

	// GetActiveSubscriptions returns the count of active subscriptions for a task
	GetActiveSubscriptions(taskID uuid.UUID) int

	// GetTotalSubscriptions returns the total number of active subscriptions across all tasks
	GetTotalSubscriptions() int

	// Close shuts down the streaming service and cleans up resources
	Close() error
}

// LogStorage handles persistent storage and retrieval of log entries
type LogStorage interface {
	// StoreLogs persists a batch of log entries to the database
	StoreLogs(ctx context.Context, entries []LogEntry) error

	// GetLogs retrieves historical logs with filtering and pagination
	GetLogs(ctx context.Context, filter LogFilter) ([]LogEntry, error)

	// SearchLogs performs full-text search on log content
	SearchLogs(ctx context.Context, taskID uuid.UUID, query string, limit int) ([]LogEntry, error)

	// GetLogCount returns the total number of log entries for a task/execution
	GetLogCount(ctx context.Context, taskID uuid.UUID, executionID *uuid.UUID) (int64, error)

	// CleanupOldLogs removes logs older than the retention period
	// Returns the number of log entries deleted
	CleanupOldLogs(ctx context.Context, retentionDays int) (int64, error)

	// CreatePartition creates a new daily partition for the specified date
	CreatePartition(ctx context.Context, date time.Time) error

	// GetPartitionStats returns statistics about log partitions for monitoring
	GetPartitionStats(ctx context.Context) ([]PartitionStats, error)

	// Close shuts down the storage service and cleans up connections
	Close() error
}

// LogCollector streams logs from Docker containers during execution
type LogCollector interface {
	// StartStreaming begins collecting logs from a container and forwards them
	// to the streaming and storage services
	StartStreaming(ctx context.Context, containerID string, taskID uuid.UUID, executionID uuid.UUID) error

	// IsStreaming returns true if actively collecting logs for the container
	IsStreaming(containerID string) bool

	// StopStreaming stops log collection for a specific container
	StopStreaming(containerID string) error

	// GetActiveStreams returns the number of active log streams
	GetActiveStreams() int

	// Close shuts down all active log streams
	Close() error
}

// LogManager coordinates all logging services and provides a unified interface
type LogManager interface {
	// GetStreamingService returns the streaming service instance
	GetStreamingService() StreamingService

	// GetLogStorage returns the storage service instance
	GetLogStorage() LogStorage

	// GetLogCollector returns the collector service instance
	GetLogCollector() LogCollector

	// IsHealthy performs health checks on all logging components
	IsHealthy(ctx context.Context) error

	// GetStats returns comprehensive statistics about the logging system
	GetStats(ctx context.Context) (*LogSystemStats, error)

	// Close shuts down all logging services
	Close() error
}

// UserAccessValidator validates that users can access specific logs
type UserAccessValidator interface {
	// CanAccessTaskLogs checks if a user can access logs for a specific task
	CanAccessTaskLogs(ctx context.Context, userID uuid.UUID, taskID uuid.UUID) error

	// CanAccessExecutionLogs checks if a user can access logs for a specific execution
	CanAccessExecutionLogs(ctx context.Context, userID uuid.UUID, executionID uuid.UUID) error
}

// LogFormatter handles formatting log entries for different output formats
type LogFormatter interface {
	// FormatForSSE formats a log entry for Server-Sent Events
	FormatForSSE(entry LogEntry) (string, error)

	// FormatForJSON formats a log entry for JSON API responses
	FormatForJSON(entry LogEntry) ([]byte, error)

	// FormatBatch formats multiple log entries efficiently
	FormatBatch(entries []LogEntry, format string) ([]byte, error)
}

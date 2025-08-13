package logging

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/docker/docker/client"
	"github.com/google/uuid"
	"github.com/voidrunnerhq/voidrunner/internal/database"
	"github.com/voidrunnerhq/voidrunner/internal/queue"
)

// DefaultLogManager implements LogManager
type DefaultLogManager struct {
	streamingService StreamingService
	logStorage       LogStorage
	logCollector     LogCollector
	config           *LogConfig
	logger           *slog.Logger
}

// NewLogManager creates a new log manager with all logging services
func NewLogManager(
	redisClient *queue.RedisClient,
	dbConn *database.Connection,
	dockerClient *client.Client,
	config *LogConfig,
	logger *slog.Logger,
) (*DefaultLogManager, error) {
	if config == nil {
		config = DefaultLogConfig()
	}

	if logger == nil {
		logger = slog.Default()
	}

	// Create streaming service
	streamingService, err := NewRedisStreamingService(redisClient, config, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create streaming service: %w", err)
	}

	// Create log storage
	logStorage, err := NewPostgreSQLLogStorage(dbConn, config, logger)
	if err != nil {
		_ = streamingService.Close() // Clean up on error
		return nil, fmt.Errorf("failed to create log storage: %w", err)
	}

	// Create log collector
	logCollector, err := NewDockerLogCollector(dockerClient, streamingService, logStorage, config, logger)
	if err != nil {
		_ = streamingService.Close() // Clean up on error
		_ = logStorage.Close()
		return nil, fmt.Errorf("failed to create log collector: %w", err)
	}

	manager := &DefaultLogManager{
		streamingService: streamingService,
		logStorage:       logStorage,
		logCollector:     logCollector,
		config:           config,
		logger:           logger.With("component", "log_manager"),
	}

	return manager, nil
}

// GetStreamingService returns the streaming service instance
func (m *DefaultLogManager) GetStreamingService() StreamingService {
	return m.streamingService
}

// GetLogStorage returns the storage service instance
func (m *DefaultLogManager) GetLogStorage() LogStorage {
	return m.logStorage
}

// GetLogCollector returns the collector service instance
func (m *DefaultLogManager) GetLogCollector() LogCollector {
	return m.logCollector
}

// IsHealthy performs health checks on all logging components
func (m *DefaultLogManager) IsHealthy(ctx context.Context) error {
	// Check if services are available
	if m.streamingService == nil {
		return fmt.Errorf("streaming service is nil")
	}

	if m.logStorage == nil {
		return fmt.Errorf("log storage is nil")
	}

	if m.logCollector == nil {
		return fmt.Errorf("log collector is nil")
	}

	// For now, we just check if the services exist
	// In the future, we could add more sophisticated health checks
	return nil
}

// GetStats returns comprehensive statistics about the logging system
func (m *DefaultLogManager) GetStats(ctx context.Context) (*LogSystemStats, error) {
	stats := &LogSystemStats{
		IsHealthy:       true,
		LastHealthCheck: time.Now(),
	}

	// Get streaming statistics
	if m.streamingService != nil {
		stats.ActiveSubscriptions = m.streamingService.GetTotalSubscriptions()
		stats.TotalSubscriptionsByTask = make(map[string]int)
		// Note: We'd need to extend the streaming service interface to get per-task stats
	}

	// Get storage statistics
	if m.logStorage != nil {
		partitionStats, err := m.logStorage.GetPartitionStats(ctx)
		if err != nil {
			m.logger.Warn("failed to get partition stats", "error", err)
		} else {
			stats.PartitionStats = partitionStats
			stats.PartitionCount = len(partitionStats)

			// Calculate total storage size and entries
			for _, partition := range partitionStats {
				stats.StorageSize += partition.SizeBytes
				stats.TotalLogEntries += partition.RowCount
			}
		}
	}

	// Get collector statistics
	if m.logCollector != nil {
		stats.ActiveStreams = m.logCollector.GetActiveStreams()
	}

	// Check overall health
	if err := m.IsHealthy(ctx); err != nil {
		stats.IsHealthy = false
	}

	return stats, nil
}

// Close shuts down all logging services
func (m *DefaultLogManager) Close() error {
	m.logger.Info("shutting down log manager")

	var errors []error

	// Close streaming service
	if m.streamingService != nil {
		if err := m.streamingService.Close(); err != nil {
			errors = append(errors, fmt.Errorf("streaming service close error: %w", err))
		}
	}

	// Close log storage
	if m.logStorage != nil {
		if err := m.logStorage.Close(); err != nil {
			errors = append(errors, fmt.Errorf("log storage close error: %w", err))
		}
	}

	// Close log collector
	if m.logCollector != nil {
		if err := m.logCollector.Close(); err != nil {
			errors = append(errors, fmt.Errorf("log collector close error: %w", err))
		}
	}

	// Return combined errors if any
	if len(errors) > 0 {
		var errorMsg string
		for i, err := range errors {
			if i > 0 {
				errorMsg += "; "
			}
			errorMsg += err.Error()
		}
		return fmt.Errorf("log manager close errors: %s", errorMsg)
	}

	m.logger.Info("log manager shutdown complete")
	return nil
}

// UserAccessValidatorImpl implements UserAccessValidator
type UserAccessValidatorImpl struct {
	taskRepo      database.TaskRepository
	executionRepo database.TaskExecutionRepository
	logger        *slog.Logger
}

// NewUserAccessValidator creates a new user access validator
func NewUserAccessValidator(
	taskRepo database.TaskRepository,
	executionRepo database.TaskExecutionRepository,
	logger *slog.Logger,
) *UserAccessValidatorImpl {
	return &UserAccessValidatorImpl{
		taskRepo:      taskRepo,
		executionRepo: executionRepo,
		logger:        logger.With("component", "user_access_validator"),
	}
}

// CanAccessTaskLogs checks if a user can access logs for a specific task
func (v *UserAccessValidatorImpl) CanAccessTaskLogs(ctx context.Context, userID, taskID uuid.UUID) error {
	task, err := v.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		if err == database.ErrTaskNotFound {
			return fmt.Errorf("task not found")
		}
		return fmt.Errorf("failed to get task: %w", err)
	}

	if task.UserID != userID {
		return fmt.Errorf("access denied: task does not belong to user")
	}

	return nil
}

// CanAccessExecutionLogs checks if a user can access logs for a specific execution
func (v *UserAccessValidatorImpl) CanAccessExecutionLogs(ctx context.Context, userID, executionID uuid.UUID) error {
	execution, err := v.executionRepo.GetByID(ctx, executionID)
	if err != nil {
		if err == database.ErrExecutionNotFound {
			return fmt.Errorf("execution not found")
		}
		return fmt.Errorf("failed to get execution: %w", err)
	}

	// Check task ownership
	return v.CanAccessTaskLogs(ctx, userID, execution.TaskID)
}

// LogFormatterImpl implements LogFormatter
type LogFormatterImpl struct {
	logger *slog.Logger
}

// NewLogFormatter creates a new log formatter
func NewLogFormatter(logger *slog.Logger) *LogFormatterImpl {
	return &LogFormatterImpl{
		logger: logger.With("component", "log_formatter"),
	}
}

// FormatForSSE formats a log entry for Server-Sent Events
func (f *LogFormatterImpl) FormatForSSE(entry LogEntry) (string, error) {
	return entry.ToSSEData()
}

// FormatForJSON formats a log entry for JSON API responses
func (f *LogFormatterImpl) FormatForJSON(entry LogEntry) ([]byte, error) {
	return json.Marshal(entry)
}

// FormatBatch formats multiple log entries efficiently
func (f *LogFormatterImpl) FormatBatch(entries []LogEntry, format string) ([]byte, error) {
	switch format {
	case "json":
		return json.Marshal(entries)
	case "sse":
		// For SSE, we format each entry separately
		var result []string
		for _, entry := range entries {
			formatted, err := f.FormatForSSE(entry)
			if err != nil {
				return nil, fmt.Errorf("failed to format entry for SSE: %w", err)
			}
			result = append(result, formatted)
		}
		return []byte(strings.Join(result, "\n")), nil
	default:
		return nil, fmt.Errorf("unsupported format: %s", format)
	}
}

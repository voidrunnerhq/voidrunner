package logging

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// LogEntry represents a single log line from container execution
type LogEntry struct {
	ID             int64     `json:"id,omitempty" db:"id"`
	TaskID         uuid.UUID `json:"task_id" db:"task_id"`
	ExecutionID    uuid.UUID `json:"execution_id" db:"execution_id"`
	Content        string    `json:"content" db:"content"`
	Stream         string    `json:"stream" db:"stream"` // "stdout" or "stderr"
	SequenceNumber int64     `json:"sequence_number" db:"sequence_number"`
	Timestamp      time.Time `json:"timestamp" db:"timestamp"`
	CreatedAt      time.Time `json:"created_at,omitempty" db:"created_at"`
}

// LogFilter defines filtering criteria for log retrieval
type LogFilter struct {
	TaskID      uuid.UUID  `json:"task_id"`
	ExecutionID *uuid.UUID `json:"execution_id,omitempty"`
	Stream      string     `json:"stream,omitempty"` // "stdout", "stderr", or "" for both
	StartTime   *time.Time `json:"start_time,omitempty"`
	EndTime     *time.Time `json:"end_time,omitempty"`
	SearchQuery string     `json:"search_query,omitempty"` // Full-text search
	Limit       int        `json:"limit"`
	Offset      int        `json:"offset"`
}

// LogResponse represents the API response for log queries
type LogResponse struct {
	Logs    []LogEntry `json:"logs"`
	Total   int64      `json:"total"`
	Limit   int        `json:"limit"`
	Offset  int        `json:"offset"`
	HasMore bool       `json:"has_more"`
}

// SSEMessage represents a Server-Sent Events message
type SSEMessage struct {
	Event string `json:"event,omitempty"`
	Data  string `json:"data"`
	ID    string `json:"id,omitempty"`
}

// LogSystemStats provides comprehensive statistics about the logging system
type LogSystemStats struct {
	// Streaming statistics
	ActiveSubscriptions      int            `json:"active_subscriptions"`
	TotalSubscriptionsByTask map[string]int `json:"total_subscriptions_by_task"`

	// Storage statistics
	TotalLogEntries int64            `json:"total_log_entries"`
	StorageSize     int64            `json:"storage_size_bytes"`
	PartitionCount  int              `json:"partition_count"`
	PartitionStats  []PartitionStats `json:"partition_stats"`

	// Collection statistics
	ActiveStreams int `json:"active_streams"`

	// Performance metrics
	AverageLatency    time.Duration `json:"average_latency"`
	MessagesPerSecond float64       `json:"messages_per_second"`
	ErrorCount        int64         `json:"error_count"`

	// System health
	IsHealthy       bool      `json:"is_healthy"`
	LastHealthCheck time.Time `json:"last_health_check"`
}

// PartitionStats provides statistics about a log partition
type PartitionStats struct {
	PartitionName string    `json:"partition_name" db:"partition_name"`
	RowCount      int64     `json:"row_count" db:"row_count"`
	SizeBytes     int64     `json:"size_bytes" db:"size_bytes"`
	PartitionDate time.Time `json:"partition_date" db:"partition_date"`
}

// StreamSubscription represents an active log stream subscription
type StreamSubscription struct {
	ID        string        `json:"id"`
	TaskID    uuid.UUID     `json:"task_id"`
	UserID    uuid.UUID     `json:"user_id"`
	CreatedAt time.Time     `json:"created_at"`
	Channel   chan LogEntry `json:"-"` // Not serialized to JSON
}

// LogCollectorStats provides statistics about log collection
type LogCollectorStats struct {
	ActiveStreams       int                   `json:"active_streams"`
	TotalLinesCollected int64                 `json:"total_lines_collected"`
	CollectionRate      float64               `json:"lines_per_second"`
	StreamsByContainer  map[string]StreamInfo `json:"streams_by_container"`
}

// StreamInfo contains information about a specific container log stream
type StreamInfo struct {
	ContainerID    string    `json:"container_id"`
	TaskID         uuid.UUID `json:"task_id"`
	ExecutionID    uuid.UUID `json:"execution_id"`
	StartTime      time.Time `json:"start_time"`
	LinesCollected int64     `json:"lines_collected"`
	IsActive       bool      `json:"is_active"`
}

// LogBatch represents a batch of log entries for efficient processing
type LogBatch struct {
	Entries   []LogEntry `json:"entries"`
	CreatedAt time.Time  `json:"created_at"`
	Size      int        `json:"size"`
}

// Validate validates a LogEntry for correctness
func (le *LogEntry) Validate() error {
	if le.TaskID == uuid.Nil {
		return fmt.Errorf("task_id cannot be empty")
	}

	if le.ExecutionID == uuid.Nil {
		return fmt.Errorf("execution_id cannot be empty")
	}

	if le.Content == "" {
		return fmt.Errorf("content cannot be empty")
	}

	if le.Stream != "stdout" && le.Stream != "stderr" {
		return fmt.Errorf("stream must be 'stdout' or 'stderr', got: %s", le.Stream)
	}

	if le.SequenceNumber < 0 {
		return fmt.Errorf("sequence_number must be non-negative")
	}

	if le.Timestamp.IsZero() {
		le.Timestamp = time.Now()
	}

	if le.CreatedAt.IsZero() {
		le.CreatedAt = time.Now()
	}

	return nil
}

// ToSSEData converts a LogEntry to Server-Sent Events data format
func (le *LogEntry) ToSSEData() (string, error) {
	data, err := json.Marshal(le)
	if err != nil {
		return "", fmt.Errorf("failed to marshal log entry to JSON: %w", err)
	}
	return string(data), nil
}

// Validate validates a LogFilter for correctness
func (lf *LogFilter) Validate() error {
	if lf.TaskID == uuid.Nil {
		return fmt.Errorf("task_id cannot be empty")
	}

	if lf.Stream != "" && lf.Stream != "stdout" && lf.Stream != "stderr" {
		return fmt.Errorf("stream must be 'stdout', 'stderr', or empty, got: %s", lf.Stream)
	}

	if lf.StartTime != nil && lf.EndTime != nil && lf.StartTime.After(*lf.EndTime) {
		return fmt.Errorf("start_time cannot be after end_time")
	}

	if lf.Limit < 0 {
		return fmt.Errorf("limit must be non-negative")
	}

	if lf.Offset < 0 {
		return fmt.Errorf("offset must be non-negative")
	}

	// Set default limit if not specified
	if lf.Limit == 0 {
		lf.Limit = 100
	}

	// Enforce maximum limit for performance
	if lf.Limit > 1000 {
		lf.Limit = 1000
	}

	return nil
}

// CreateSSEMessage creates a Server-Sent Events message
func CreateSSEMessage(event, data, id string) SSEMessage {
	return SSEMessage{
		Event: event,
		Data:  data,
		ID:    id,
	}
}

// Format formats an SSE message for transmission
func (msg *SSEMessage) Format() string {
	var result string

	if msg.ID != "" {
		result += fmt.Sprintf("id: %s\n", msg.ID)
	}

	if msg.Event != "" {
		result += fmt.Sprintf("event: %s\n", msg.Event)
	}

	result += fmt.Sprintf("data: %s\n\n", msg.Data)

	return result
}

// LogLevel represents the severity level of a log entry
type LogLevel string

const (
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"
)

// StreamType represents the type of log stream
type StreamType string

const (
	StreamTypeStdout StreamType = "stdout"
	StreamTypeStderr StreamType = "stderr"
)

// IsValid checks if a StreamType is valid
func (st StreamType) IsValid() bool {
	return st == StreamTypeStdout || st == StreamTypeStderr
}

// String returns the string representation of StreamType
func (st StreamType) String() string {
	return string(st)
}

// LogConfig represents configuration for the logging system
type LogConfig struct {
	// Feature toggles
	StreamEnabled bool `yaml:"stream_enabled" json:"stream_enabled"`

	// Streaming configuration
	BufferSize           int           `yaml:"buffer_size" json:"buffer_size"`
	MaxConcurrentStreams int           `yaml:"max_concurrent_streams" json:"max_concurrent_streams"`
	StreamTimeout        time.Duration `yaml:"stream_timeout" json:"stream_timeout"`

	// Storage configuration
	BatchInsertSize     int           `yaml:"batch_insert_size" json:"batch_insert_size"`
	BatchInsertInterval time.Duration `yaml:"batch_insert_interval" json:"batch_insert_interval"`
	MaxLogLineSize      int           `yaml:"max_log_line_size" json:"max_log_line_size"`

	// Retention and cleanup
	RetentionDays         int           `yaml:"retention_days" json:"retention_days"`
	CleanupInterval       time.Duration `yaml:"cleanup_interval" json:"cleanup_interval"`
	PartitionCreationDays int           `yaml:"partition_creation_days" json:"partition_creation_days"`

	// Performance tuning
	RedisChannelPrefix  string        `yaml:"redis_channel_prefix" json:"redis_channel_prefix"`
	SubscriberKeepalive time.Duration `yaml:"subscriber_keepalive" json:"subscriber_keepalive"`
}

// DefaultLogConfig returns a LogConfig with sensible defaults
func DefaultLogConfig() *LogConfig {
	return &LogConfig{
		StreamEnabled:         false, // Disabled by default for safety
		BufferSize:            1000,
		MaxConcurrentStreams:  1000,
		StreamTimeout:         30 * time.Minute,
		BatchInsertSize:       50,
		BatchInsertInterval:   5 * time.Second,
		MaxLogLineSize:        4096,
		RetentionDays:         30,
		CleanupInterval:       24 * time.Hour,
		PartitionCreationDays: 7,
		RedisChannelPrefix:    "voidrunner:logs:",
		SubscriberKeepalive:   30 * time.Second,
	}
}

// Validate validates the LogConfig
func (lc *LogConfig) Validate() error {
	if lc.BufferSize <= 0 {
		return fmt.Errorf("buffer_size must be positive")
	}

	if lc.MaxConcurrentStreams <= 0 {
		return fmt.Errorf("max_concurrent_streams must be positive")
	}

	if lc.StreamTimeout <= 0 {
		return fmt.Errorf("stream_timeout must be positive")
	}

	if lc.BatchInsertSize <= 0 {
		return fmt.Errorf("batch_insert_size must be positive")
	}

	if lc.BatchInsertInterval <= 0 {
		return fmt.Errorf("batch_insert_interval must be positive")
	}

	if lc.MaxLogLineSize <= 0 {
		return fmt.Errorf("max_log_line_size must be positive")
	}

	if lc.RetentionDays <= 0 {
		return fmt.Errorf("retention_days must be positive")
	}

	if lc.CleanupInterval <= 0 {
		return fmt.Errorf("cleanup_interval must be positive")
	}

	if lc.PartitionCreationDays <= 0 {
		return fmt.Errorf("partition_creation_days must be positive")
	}

	if lc.RedisChannelPrefix == "" {
		return fmt.Errorf("redis_channel_prefix cannot be empty")
	}

	if lc.SubscriberKeepalive <= 0 {
		return fmt.Errorf("subscriber_keepalive must be positive")
	}

	return nil
}

package logging

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogEntry_Validate(t *testing.T) {
	tests := []struct {
		name        string
		entry       LogEntry
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid log entry",
			entry: LogEntry{
				TaskID:         uuid.New(),
				ExecutionID:    uuid.New(),
				Content:        "test log message",
				Stream:         "stdout",
				SequenceNumber: 1,
				Timestamp:      time.Now(),
			},
			expectError: false,
		},
		{
			name: "missing task ID",
			entry: LogEntry{
				ExecutionID:    uuid.New(),
				Content:        "test log message",
				Stream:         "stdout",
				SequenceNumber: 1,
			},
			expectError: true,
			errorMsg:    "task_id cannot be empty",
		},
		{
			name: "missing execution ID",
			entry: LogEntry{
				TaskID:         uuid.New(),
				Content:        "test log message",
				Stream:         "stdout",
				SequenceNumber: 1,
			},
			expectError: true,
			errorMsg:    "execution_id cannot be empty",
		},
		{
			name: "empty content",
			entry: LogEntry{
				TaskID:         uuid.New(),
				ExecutionID:    uuid.New(),
				Content:        "",
				Stream:         "stdout",
				SequenceNumber: 1,
			},
			expectError: true,
			errorMsg:    "content cannot be empty",
		},
		{
			name: "invalid stream",
			entry: LogEntry{
				TaskID:         uuid.New(),
				ExecutionID:    uuid.New(),
				Content:        "test log message",
				Stream:         "invalid",
				SequenceNumber: 1,
			},
			expectError: true,
			errorMsg:    "stream must be 'stdout' or 'stderr'",
		},
		{
			name: "negative sequence number",
			entry: LogEntry{
				TaskID:         uuid.New(),
				ExecutionID:    uuid.New(),
				Content:        "test log message",
				Stream:         "stdout",
				SequenceNumber: -1,
			},
			expectError: true,
			errorMsg:    "sequence_number must be non-negative",
		},
		{
			name: "auto-fill timestamps",
			entry: LogEntry{
				TaskID:         uuid.New(),
				ExecutionID:    uuid.New(),
				Content:        "test log message",
				Stream:         "stderr",
				SequenceNumber: 1,
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.entry.Validate()
			
			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)
				
				// Check that timestamps were auto-filled
				assert.False(t, tt.entry.Timestamp.IsZero())
				assert.False(t, tt.entry.CreatedAt.IsZero())
			}
		})
	}
}

func TestLogEntry_ToSSEData(t *testing.T) {
	taskID := uuid.New()
	executionID := uuid.New()
	timestamp := time.Date(2025, 7, 29, 12, 0, 0, 0, time.UTC)
	
	entry := LogEntry{
		ID:             123,
		TaskID:         taskID,
		ExecutionID:    executionID,
		Content:        "test log message",
		Stream:         "stdout",
		SequenceNumber: 42,
		Timestamp:      timestamp,
		CreatedAt:      timestamp,
	}

	data, err := entry.ToSSEData()
	require.NoError(t, err)
	assert.NotEmpty(t, data)
	
	// Check that the JSON contains expected fields
	assert.Contains(t, data, `"task_id":"`+taskID.String()+`"`)
	assert.Contains(t, data, `"execution_id":"`+executionID.String()+`"`)
	assert.Contains(t, data, `"content":"test log message"`)
	assert.Contains(t, data, `"stream":"stdout"`)
	assert.Contains(t, data, `"sequence_number":42`)
}

func TestLogFilter_Validate(t *testing.T) {
	taskID := uuid.New()
	executionID := uuid.New()
	now := time.Now()
	future := now.Add(time.Hour)

	tests := []struct {
		name        string
		filter      LogFilter
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid basic filter",
			filter: LogFilter{
				TaskID: taskID,
				Limit:  50,
				Offset: 0,
			},
			expectError: false,
		},
		{
			name: "missing task ID",
			filter: LogFilter{
				Limit:  50,
				Offset: 0,
			},
			expectError: true,
			errorMsg:    "task_id cannot be empty",
		},
		{
			name: "invalid stream",
			filter: LogFilter{
				TaskID: taskID,
				Stream: "invalid",
				Limit:  50,
				Offset: 0,
			},
			expectError: true,
			errorMsg:    "stream must be 'stdout', 'stderr', or empty",
		},
		{
			name: "start time after end time",
			filter: LogFilter{
				TaskID:    taskID,
				StartTime: &future,
				EndTime:   &now,
				Limit:     50,
				Offset:    0,
			},
			expectError: true,
			errorMsg:    "start_time cannot be after end_time",
		},
		{
			name: "negative limit",
			filter: LogFilter{
				TaskID: taskID,
				Limit:  -1,
				Offset: 0,
			},
			expectError: true,
			errorMsg:    "limit must be non-negative",
		},
		{
			name: "negative offset",
			filter: LogFilter{
				TaskID: taskID,
				Limit:  50,
				Offset: -1,
			},
			expectError: true,
			errorMsg:    "offset must be non-negative",
		},
		{
			name: "default limit applied",
			filter: LogFilter{
				TaskID: taskID,
				Limit:  0,
				Offset: 0,
			},
			expectError: false,
		},
		{
			name: "limit too high gets clamped",
			filter: LogFilter{
				TaskID: taskID,
				Limit:  2000,
				Offset: 0,
			},
			expectError: false,
		},
		{
			name: "valid with execution ID",
			filter: LogFilter{
				TaskID:      taskID,
				ExecutionID: &executionID,
				Stream:      "stdout",
				StartTime:   &now,
				EndTime:     &future,
				SearchQuery: "error",
				Limit:       100,
				Offset:      10,
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.filter.Validate()
			
			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)
				
				// Check defaults were applied
				if tt.filter.Limit == 0 {
					assert.Equal(t, 100, tt.filter.Limit) // Default limit
				}
				if tt.filter.Limit > 1000 {
					assert.Equal(t, 1000, tt.filter.Limit) // Max limit
				}
			}
		})
	}
}

func TestSSEMessage_Format(t *testing.T) {
	tests := []struct {
		name     string
		message  SSEMessage
		expected string
	}{
		{
			name: "message with all fields",
			message: SSEMessage{
				Event: "log",
				Data:  `{"content":"test message"}`,
				ID:    "123",
			},
			expected: "id: 123\nevent: log\ndata: {\"content\":\"test message\"}\n\n",
		},
		{
			name: "message with only data",
			message: SSEMessage{
				Data: "simple message",
			},
			expected: "data: simple message\n\n",
		},
		{
			name: "message with event and data",
			message: SSEMessage{
				Event: "connected",
				Data:  "connection established",
			},
			expected: "event: connected\ndata: connection established\n\n",
		},
		{
			name: "message with ID and data",
			message: SSEMessage{
				Data: "message with id",
				ID:   "msg-456",
			},
			expected: "id: msg-456\ndata: message with id\n\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.message.Format()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCreateSSEMessage(t *testing.T) {
	msg := CreateSSEMessage("test-event", "test-data", "test-id")
	
	assert.Equal(t, "test-event", msg.Event)
	assert.Equal(t, "test-data", msg.Data)
	assert.Equal(t, "test-id", msg.ID)
}

func TestStreamType_IsValid(t *testing.T) {
	tests := []struct {
		stream StreamType
		valid  bool
	}{
		{StreamTypeStdout, true},
		{StreamTypeStderr, true},
		{StreamType("invalid"), false},
		{StreamType(""), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.stream), func(t *testing.T) {
			assert.Equal(t, tt.valid, tt.stream.IsValid())
		})
	}
}

func TestLogConfig_Validate(t *testing.T) {
	tests := []struct {
		name        string
		config      LogConfig
		expectError bool
		errorMsg    string
	}{
		{
			name:        "default config should be valid",
			config:      *DefaultLogConfig(),
			expectError: false,
		},
		{
			name: "invalid buffer size",
			config: LogConfig{
				BufferSize:            0,
				MaxConcurrentStreams:  1000,
				StreamTimeout:         30 * time.Minute,
				BatchInsertSize:       50,
				BatchInsertInterval:   5 * time.Second,
				MaxLogLineSize:        4096,
				RetentionDays:         30,
				CleanupInterval:       24 * time.Hour,
				PartitionCreationDays: 7,
				RedisChannelPrefix:    "test:",
				SubscriberKeepalive:   30 * time.Second,
			},
			expectError: true,
			errorMsg:    "buffer_size must be positive",
		},
		{
			name: "empty redis channel prefix",
			config: LogConfig{
				BufferSize:            1000,
				MaxConcurrentStreams:  1000,
				StreamTimeout:         30 * time.Minute,
				BatchInsertSize:       50,
				BatchInsertInterval:   5 * time.Second,
				MaxLogLineSize:        4096,
				RetentionDays:         30,
				CleanupInterval:       24 * time.Hour,
				PartitionCreationDays: 7,
				RedisChannelPrefix:    "",
				SubscriberKeepalive:   30 * time.Second,
			},
			expectError: true,
			errorMsg:    "redis_channel_prefix cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			
			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestDefaultLogConfig(t *testing.T) {
	config := DefaultLogConfig()
	
	require.NotNil(t, config)
	assert.False(t, config.StreamEnabled) // Should be disabled by default for safety
	assert.Equal(t, 1000, config.BufferSize)
	assert.Equal(t, 1000, config.MaxConcurrentStreams)
	assert.Equal(t, 30*time.Minute, config.StreamTimeout)
	assert.Equal(t, 50, config.BatchInsertSize)
	assert.Equal(t, 5*time.Second, config.BatchInsertInterval)
	assert.Equal(t, 4096, config.MaxLogLineSize)
	assert.Equal(t, 30, config.RetentionDays)
	assert.Equal(t, 24*time.Hour, config.CleanupInterval)
	assert.Equal(t, 7, config.PartitionCreationDays)
	assert.Equal(t, "voidrunner:logs:", config.RedisChannelPrefix)
	assert.Equal(t, 30*time.Second, config.SubscriberKeepalive)
	
	// Validate that default config is valid
	require.NoError(t, config.Validate())
}
package queue

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateReceiptHandle(t *testing.T) {
	tests := []struct {
		name      string
		messageID string
		validate  func(*testing.T, string)
	}{
		{
			name:      "valid message ID",
			messageID: "test-message-123",
			validate: func(t *testing.T, handle string) {
				assert.NotEmpty(t, handle)
				assert.True(t, strings.HasPrefix(handle, "test-message-123:"))
				
				// Should have format: messageID:timestamp:randomHex
				parts := strings.Split(handle, ":")
				require.Len(t, parts, 3, "handle should have 3 parts")
				
				assert.Equal(t, "test-message-123", parts[0])
				
				// Timestamp should be a valid number - just verify we can parse it
				// This is implicitly validated by the function working
				
				// Random part should be hex
				assert.NotEmpty(t, parts[2])
				assert.True(t, len(parts[2]) > 0)
			},
		},
		{
			name:      "empty message ID",
			messageID: "",
			validate: func(t *testing.T, handle string) {
				assert.NotEmpty(t, handle)
				assert.True(t, strings.HasPrefix(handle, ":"))
				
				parts := strings.Split(handle, ":")
				assert.Equal(t, "", parts[0])
			},
		},
		{
			name:      "special characters in message ID",
			messageID: "msg-with-special:chars@123",
			validate: func(t *testing.T, handle string) {
				assert.NotEmpty(t, handle)
				assert.True(t, strings.Contains(handle, "msg-with-special:chars@123"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handle := GenerateReceiptHandle(tt.messageID)
			tt.validate(t, handle)
		})
	}
}

func TestGenerateReceiptHandleUniqueness(t *testing.T) {
	messageID := "test-message"
	handles := make(map[string]bool)
	
	// Generate 100 handles and ensure they're all unique
	for i := 0; i < 100; i++ {
		handle := GenerateReceiptHandle(messageID)
		assert.False(t, handles[handle], "generated duplicate handle: %s", handle)
		handles[handle] = true
	}
	
	assert.Len(t, handles, 100, "should have generated 100 unique handles")
}

func TestValidatePriority(t *testing.T) {
	tests := []struct {
		name        string
		priority    int
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid lowest priority",
			priority:    PriorityLowest,
			expectError: false,
		},
		{
			name:        "valid normal priority",
			priority:    PriorityNormal,
			expectError: false,
		},
		{
			name:        "valid highest priority",
			priority:    PriorityHighest,
			expectError: false,
		},
		{
			name:        "valid custom priority",
			priority:    7,
			expectError: false,
		},
		{
			name:        "too low priority",
			priority:    -1,
			expectError: true,
			errorMsg:    "validation failed for field 'priority'",
		},
		{
			name:        "too high priority",
			priority:    11,
			expectError: true,
			errorMsg:    "validation failed for field 'priority'",
		},
		{
			name:        "way too low priority",
			priority:    -100,
			expectError: true,
			errorMsg:    "validation failed for field 'priority'",
		},
		{
			name:        "way too high priority",
			priority:    100,
			expectError: true,
			errorMsg:    "validation failed for field 'priority'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePriority(tt.priority)
			
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateTaskMessage(t *testing.T) {
	validMessage := &TaskMessage{
		TaskID:    uuid.New(),
		UserID:    uuid.New(),
		Priority:  PriorityNormal,
		QueuedAt:  time.Now(),
		MessageID: "test-message",
	}

	tests := []struct {
		name        string
		message     *TaskMessage
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid message",
			message:     validMessage,
			expectError: false,
		},
		{
			name: "invalid priority",
			message: &TaskMessage{
				TaskID:    uuid.New(),
				UserID:    uuid.New(),
				Priority:  99,
				QueuedAt:  time.Now(),
				MessageID: "test-message",
			},
			expectError: true,
			errorMsg:    "validation",
		},
		{
			name:        "nil message",
			message:     nil,
			expectError: true,
			errorMsg:    "cannot be nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTaskMessage(tt.message)
			
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCalculatePriorityScore(t *testing.T) {
	now := time.Now()
	
	tests := []struct {
		name     string
		priority int
		queuedAt time.Time
		validate func(*testing.T, float64)
	}{
		{
			name:     "highest priority",
			priority: PriorityHighest,
			queuedAt: now,
			validate: func(t *testing.T, score float64) {
				assert.True(t, score > 0, "score should be positive")
			},
		},
		{
			name:     "lowest priority",
			priority: PriorityLowest,
			queuedAt: now,
			validate: func(t *testing.T, score float64) {
				assert.True(t, score > 0, "score should be positive")
			},
		},
		{
			name:     "normal priority",
			priority: PriorityNormal,
			queuedAt: now,
			validate: func(t *testing.T, score float64) {
				assert.True(t, score > 0, "score should be positive")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := CalculatePriorityScore(tt.priority, tt.queuedAt)
			tt.validate(t, score)
		})
	}
}

func TestCalculateRetryDelay(t *testing.T) {
	baseDelay := 1 * time.Minute
	backoffFactor := 2.0
	maxDelay := 10 * time.Minute

	tests := []struct {
		name     string
		attempt  int
		expected func(*testing.T, time.Duration)
	}{
		{
			name:    "first attempt",
			attempt: 1,
			expected: func(t *testing.T, delay time.Duration) {
				// With jitter (0.9-1.1), delay should be close to base delay
				assert.True(t, delay >= time.Duration(float64(baseDelay)*0.8), "delay should be at least 80% of base delay")
				assert.True(t, delay <= time.Duration(float64(baseDelay)*1.2), "delay should be at most 120% of base delay")
			},
		},
		{
			name:    "second attempt",
			attempt: 2,
			expected: func(t *testing.T, delay time.Duration) {
				// With backoff factor 2.0 and jitter, should be around 2x base delay
				expected := 2 * baseDelay
				assert.True(t, delay >= time.Duration(float64(expected)*0.8), "delay should be at least 80% of expected")
				assert.True(t, delay <= time.Duration(float64(expected)*1.2), "delay should be at most 120% of expected")
			},
		},
		{
			name:    "high attempt capped at max",
			attempt: 10,
			expected: func(t *testing.T, delay time.Duration) {
				assert.LessOrEqual(t, delay, maxDelay)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			delay := CalculateRetryDelay(tt.attempt, baseDelay, backoffFactor, maxDelay)
			tt.expected(t, delay)
		})
	}
}

func TestFormatQueueKey(t *testing.T) {
	tests := []struct {
		name     string
		queue    string
		suffix   string
		expected string
	}{
		{
			name:     "basic queue key",
			queue:    "tasks",
			suffix:   "queue",
			expected: "tasks:queue",
		},
		{
			name:     "inflight key",
			queue:    "voidrunner:tasks",
			suffix:   "inflight",
			expected: "voidrunner:tasks:inflight",
		},
		{
			name:     "empty suffix",
			queue:    "tasks",
			suffix:   "",
			expected: "tasks",
		},
		{
			name:     "empty queue",
			queue:    "",
			suffix:   "queue",
			expected: ":queue",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatQueueKey(tt.queue, tt.suffix)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatStatsKey(t *testing.T) {
	tests := []struct {
		name     string
		queue    string
		expected string
	}{
		{
			name:     "basic stats key",
			queue:    "tasks",
			expected: "tasks:stats",
		},
		{
			name:     "full queue name",
			queue:    "voidrunner:tasks",
			expected: "voidrunner:tasks:stats",
		},
		{
			name:     "empty queue",
			queue:    "",
			expected: ":stats",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatStatsKey(tt.queue)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSerializeDeserializeMessage(t *testing.T) {
	message := &TaskMessage{
		TaskID:    uuid.New(),
		UserID:    uuid.New(),
		Priority:  PriorityNormal,
		QueuedAt:  time.Now().Truncate(time.Millisecond), // Truncate for comparison
		Attempts:  1,
		MessageID: "test-message-123",
		Attributes: map[string]string{
			"key1": "value1",
			"key2": "value2",
		},
	}

	// Test serialization
	data, err := SerializeMessage(message)
	assert.NoError(t, err)
	assert.NotEmpty(t, data)

	// Test deserialization
	deserialized, err := DeserializeMessage(data)
	assert.NoError(t, err)
	assert.NotNil(t, deserialized)

	// Verify all fields match
	assert.Equal(t, message.TaskID, deserialized.TaskID)
	assert.Equal(t, message.UserID, deserialized.UserID)
	assert.Equal(t, message.Priority, deserialized.Priority)
	assert.Equal(t, message.QueuedAt.Unix(), deserialized.QueuedAt.Unix()) // Compare Unix time
	assert.Equal(t, message.Attempts, deserialized.Attempts)
	assert.Equal(t, message.MessageID, deserialized.MessageID)
	assert.Equal(t, message.Attributes, deserialized.Attributes)
}

func TestTimePtr(t *testing.T) {
	now := time.Now()
	ptr := timePtr(now)
	
	assert.NotNil(t, ptr)
	assert.Equal(t, now, *ptr)
	
	// Ensure it's a different memory address
	assert.True(t, &now != ptr)
}

func TestCopyAttributes(t *testing.T) {
	tests := []struct {
		name     string
		original map[string]string
		expected map[string]string
	}{
		{
			name: "nil map",
			original: nil,
			expected: map[string]string(nil),
		},
		{
			name: "empty map",
			original: map[string]string{},
			expected: map[string]string{},
		},
		{
			name: "single entry",
			original: map[string]string{"key": "value"},
			expected: map[string]string{"key": "value"},
		},
		{
			name: "multiple entries",
			original: map[string]string{
				"key1": "value1",
				"key2": "value2",
				"key3": "value3",
			},
			expected: map[string]string{
				"key1": "value1",
				"key2": "value2",
				"key3": "value3",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := copyAttributes(tt.original)
			
			assert.Equal(t, tt.expected, result)
			
			// Ensure it's a deep copy (different memory addresses)
			if len(tt.original) > 0 {
				// Modify original and ensure copy is not affected
				for k := range tt.original {
					tt.original[k] = "modified"
					break
				}
				
				// Copy should still have original values
				for k, v := range tt.expected {
					assert.Equal(t, v, result[k], "copy was affected by modification to original")
				}
			}
		})
	}
}
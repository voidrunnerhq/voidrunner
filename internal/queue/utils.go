package queue

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
)

// GenerateMessageID generates a unique message ID
func GenerateMessageID() string {
	// Generate a random 16-byte ID
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to UUID if random generation fails
		return uuid.New().String()
	}
	return hex.EncodeToString(bytes)
}

// GenerateReceiptHandle generates a unique receipt handle for message processing
func GenerateReceiptHandle(messageID string) string {
	// Combine message ID with timestamp and random component
	timestamp := time.Now().Unix()
	randomBytes := make([]byte, 8)
	if _, err := rand.Read(randomBytes); err != nil {
		// Fallback to timestamp-only if random generation fails
		return fmt.Sprintf("%s:%d", messageID, timestamp)
	}

	return fmt.Sprintf("%s:%d:%s", messageID, timestamp, hex.EncodeToString(randomBytes))
}

// ValidatePriority validates task priority value
func ValidatePriority(priority int) error {
	if priority < PriorityLowest || priority > PriorityHighest {
		return NewValidationError("priority", priority,
			fmt.Sprintf("must be between %d and %d", PriorityLowest, PriorityHighest))
	}
	return nil
}

// ValidateTaskMessage validates a task message
func ValidateTaskMessage(message *TaskMessage) error {
	if message == nil {
		return NewValidationError("message", nil, "cannot be nil")
	}

	if message.TaskID == uuid.Nil {
		return NewValidationError("task_id", message.TaskID, "cannot be empty")
	}

	if message.UserID == uuid.Nil {
		return NewValidationError("user_id", message.UserID, "cannot be empty")
	}

	if err := ValidatePriority(message.Priority); err != nil {
		return err
	}

	if message.QueuedAt.IsZero() {
		return NewValidationError("queued_at", message.QueuedAt, "cannot be empty")
	}

	if message.Attempts < 0 {
		return NewValidationError("attempts", message.Attempts, "cannot be negative")
	}

	return nil
}

// SerializeMessage serializes a task message to JSON
func SerializeMessage(message *TaskMessage) (string, error) {
	if err := ValidateTaskMessage(message); err != nil {
		return "", fmt.Errorf("message validation failed: %w", err)
	}

	data, err := json.Marshal(message)
	if err != nil {
		return "", fmt.Errorf("failed to serialize message: %w", err)
	}

	return string(data), nil
}

// DeserializeMessage deserializes a task message from JSON
func DeserializeMessage(data string) (*TaskMessage, error) {
	if data == "" {
		return nil, NewValidationError("data", data, "cannot be empty")
	}

	var message TaskMessage
	if err := json.Unmarshal([]byte(data), &message); err != nil {
		return nil, fmt.Errorf("failed to deserialize message: %w", err)
	}

	if err := ValidateTaskMessage(&message); err != nil {
		return nil, fmt.Errorf("deserialized message validation failed: %w", err)
	}

	return &message, nil
}

// CalculatePriorityScore calculates a score for priority-based queue ordering
// Higher priority tasks get lower scores (sorted ascending)
// Tasks with the same priority are ordered by queued time (FIFO)
func CalculatePriorityScore(priority int, queuedAt time.Time) float64 {
	// Validate priority
	if priority < PriorityLowest {
		priority = PriorityLowest
	}
	if priority > PriorityHighest {
		priority = PriorityHighest
	}

	// Invert priority so higher priority gets lower score
	priorityScore := float64(PriorityHighest - priority)

	// Add timestamp component for FIFO within same priority
	// Use microseconds to ensure uniqueness and proper ordering
	timestampScore := float64(queuedAt.UnixMicro()) / 1e12 // Scale down to avoid overflow

	// Combine: priority has much higher weight than timestamp
	return priorityScore*1e6 + timestampScore
}

// CalculateRetryDelay calculates the delay for retry using exponential backoff
func CalculateRetryDelay(attempt int, baseDelay time.Duration, backoffFactor float64, maxDelay time.Duration) time.Duration {
	if attempt <= 0 {
		return baseDelay
	}

	if backoffFactor <= 1.0 {
		backoffFactor = 2.0 // Default backoff factor
	}

	// Calculate exponential backoff: baseDelay * (backoffFactor ^ (attempt - 1))
	delay := float64(baseDelay) * math.Pow(backoffFactor, float64(attempt-1))

	// Add some jitter to prevent thundering herd (±10%)
	// Use crypto/rand for secure random number generation
	jitterBytes := make([]byte, 8)
	if _, err := rand.Read(jitterBytes); err != nil {
		// Fallback to deterministic jitter if random generation fails
		jitter := 1.0
		delay *= jitter
	} else {
		// Convert random bytes to float64 in range [0.9, 1.1]
		randomUint64 := uint64(jitterBytes[0])<<56 | uint64(jitterBytes[1])<<48 |
			uint64(jitterBytes[2])<<40 | uint64(jitterBytes[3])<<32 |
			uint64(jitterBytes[4])<<24 | uint64(jitterBytes[5])<<16 |
			uint64(jitterBytes[6])<<8 | uint64(jitterBytes[7])
		randomFloat := float64(randomUint64) / float64(^uint64(0)) // Normalize to [0,1]
		jitter := 0.9 + (0.2 * randomFloat)                        // Scale to [0.9, 1.1]
		delay *= jitter
	}

	// Ensure we don't exceed max delay
	if time.Duration(delay) > maxDelay {
		return maxDelay
	}

	return time.Duration(delay)
}

// CalculateRetryAt calculates when a task should be retried
func CalculateRetryAt(message *TaskMessage, baseDelay time.Duration, backoffFactor float64, maxDelay time.Duration) time.Time {
	delay := CalculateRetryDelay(message.Attempts, baseDelay, backoffFactor, maxDelay)

	baseTime := time.Now()
	if message.LastAttempt != nil {
		baseTime = *message.LastAttempt
	}

	return baseTime.Add(delay)
}

// IsRetryEligible checks if a task is eligible for retry
func IsRetryEligible(message *TaskMessage, maxRetries int) bool {
	if message == nil {
		return false
	}

	return message.Attempts < maxRetries
}

// CreateRetryMessage creates a copy of the message for retry
func CreateRetryMessage(original *TaskMessage) *TaskMessage {
	if original == nil {
		return nil
	}

	retry := &TaskMessage{
		TaskID:        original.TaskID,
		UserID:        original.UserID,
		Priority:      original.Priority,
		QueuedAt:      original.QueuedAt,
		Attempts:      original.Attempts + 1,
		LastAttempt:   timePtr(time.Now()),
		FailureReason: original.FailureReason,
		MessageID:     GenerateMessageID(), // Generate new message ID for retry
		Attributes:    copyAttributes(original.Attributes),
	}

	return retry
}

// FormatQueueKey formats a Redis key for queue operations
func FormatQueueKey(queueName, suffix string) string {
	if suffix == "" {
		return queueName
	}
	return fmt.Sprintf("%s:%s", queueName, suffix)
}

// FormatMessageKey formats a Redis key for message storage
func FormatMessageKey(queueName, messageID string) string {
	return fmt.Sprintf("%s:messages:%s", queueName, messageID)
}

// FormatStatsKey formats a Redis key for queue statistics
func FormatStatsKey(queueName string) string {
	return fmt.Sprintf("%s:stats", queueName)
}

// timePtr returns a pointer to a time value
func timePtr(t time.Time) *time.Time {
	return &t
}

// copyAttributes creates a copy of attributes map
func copyAttributes(original map[string]string) map[string]string {
	if original == nil {
		return nil
	}

	copy := make(map[string]string, len(original))
	for k, v := range original {
		copy[k] = v
	}
	return copy
}

// GetCurrentUnixMilli returns current time in milliseconds since Unix epoch
func GetCurrentUnixMilli() int64 {
	return time.Now().UnixMilli()
}

// UnixMilliToTime converts Unix milliseconds to time.Time
func UnixMilliToTime(unixMilli int64) time.Time {
	return time.UnixMilli(unixMilli)
}

// TimeToUnixMilli converts time.Time to Unix milliseconds
func TimeToUnixMilli(t time.Time) int64 {
	return t.UnixMilli()
}

// ParseReceiptHandle parses a receipt handle to extract message ID and metadata
func ParseReceiptHandle(receiptHandle string) (messageID string, timestamp int64, err error) {
	if receiptHandle == "" {
		return "", 0, NewValidationError("receipt_handle", receiptHandle, "cannot be empty")
	}

	// Receipt handle format: messageID:timestamp:random
	var randomPart string
	n, err := fmt.Sscanf(receiptHandle, "%s:%d:%s", &messageID, &timestamp, &randomPart)
	if err != nil || n != 3 {
		return "", 0, NewValidationError("receipt_handle", receiptHandle, "invalid format")
	}

	return messageID, timestamp, nil
}

// IsReceiptHandleExpired checks if a receipt handle has expired
func IsReceiptHandleExpired(receiptHandle string, visibilityTimeout time.Duration) bool {
	_, timestamp, err := ParseReceiptHandle(receiptHandle)
	if err != nil {
		return true // Invalid handles are considered expired
	}

	issueTime := time.Unix(timestamp, 0)
	expiryTime := issueTime.Add(visibilityTimeout)

	return time.Now().After(expiryTime)
}

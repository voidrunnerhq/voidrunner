//go:build integration

package integration_test

import (
	"context"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/voidrunnerhq/voidrunner/internal/logging"
	"github.com/voidrunnerhq/voidrunner/tests/testutil"
)

// getTestLoggingConfig returns logging configuration for integration tests
func getTestLoggingConfig() *logging.LogConfig {
	testConfig := testutil.GetTestConfig()

	// Manual conversion from config.LoggingConfig to logging.LogConfig
	// This avoids import cycle between config and logging packages
	config := &logging.LogConfig{
		StreamEnabled:         testConfig.Logging.StreamEnabled,
		BufferSize:            testConfig.Logging.BufferSize,
		MaxConcurrentStreams:  testConfig.Logging.MaxConcurrentStreams,
		StreamTimeout:         testConfig.Logging.StreamTimeout,
		BatchInsertSize:       1, // Disable batching for integration tests to ensure immediate writes
		BatchInsertInterval:   testConfig.Logging.BatchInsertInterval,
		MaxLogLineSize:        testConfig.Logging.MaxLogLineSize,
		RetentionDays:         testConfig.Logging.RetentionDays,
		CleanupInterval:       testConfig.Logging.CleanupInterval,
		PartitionCreationDays: testConfig.Logging.PartitionCreationDays,
		RedisChannelPrefix:    testConfig.Logging.RedisChannelPrefix,
		SubscriberKeepalive:   testConfig.Logging.SubscriberKeepalive,
	}
	return config
}

// TestLoggingServiceDependencies tests that logging services handle dependency failures gracefully
func TestLoggingServiceDependencies(t *testing.T) {
	t.Run("handles nil Redis client gracefully", func(t *testing.T) {
		// Test that NewRedisStreamingService handles nil Redis client
		service, err := logging.NewRedisStreamingService(nil, getTestLoggingConfig(), slog.Default())

		assert.Nil(t, service)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "redis client is required")
	})

	t.Run("handles nil database connection gracefully", func(t *testing.T) {
		// Test that NewPostgreSQLLogStorage handles nil database connection
		storage, err := logging.NewPostgreSQLLogStorage(nil, getTestLoggingConfig(), slog.Default())

		assert.Nil(t, storage)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "database connection is required")
	})
}

// TestLoggingServiceIntegration tests real service interactions with dependencies
func TestLoggingServiceIntegration(t *testing.T) {
	testutil.WithTestDatabase(t, func(db *testutil.DatabaseHelper) {
		t.Run("PostgreSQL log storage integration", func(t *testing.T) {
			// Test with real database connection
			storage, err := logging.NewPostgreSQLLogStorage(db.DB, getTestLoggingConfig(), slog.Default())
			require.NoError(t, err)
			require.NotNil(t, storage)
			defer storage.Close()

			// Create test user, task, and execution records for foreign key constraints
			userID := uuid.New()
			taskID := uuid.New()
			executionID := uuid.New()

			// Insert user
			_, err = db.DB.Pool.Exec(context.Background(),
				"INSERT INTO users (id, email, password_hash) VALUES ($1, $2, $3)",
				userID, "testuser@example.com", "hashedpassword")
			require.NoError(t, err)

			// Insert task
			_, err = db.DB.Pool.Exec(context.Background(),
				"INSERT INTO tasks (id, user_id, name, script_content) VALUES ($1, $2, $3, $4)",
				taskID, userID, "test-task", "print('hello')")
			require.NoError(t, err)

			// Insert task execution
			_, err = db.DB.Pool.Exec(context.Background(),
				"INSERT INTO task_executions (id, task_id, status) VALUES ($1, $2, $3)",
				executionID, taskID, "running")
			require.NoError(t, err)

			logs := []logging.LogEntry{
				{
					TaskID:         taskID,
					ExecutionID:    executionID,
					Content:        "Test log line 1",
					Stream:         "stdout",
					SequenceNumber: 1,
					Timestamp:      time.Now(),
					CreatedAt:      time.Now(),
				},
				{
					TaskID:         taskID,
					ExecutionID:    executionID,
					Content:        "Test log line 2",
					Stream:         "stderr",
					SequenceNumber: 2,
					Timestamp:      time.Now(),
					CreatedAt:      time.Now(),
				},
			}

			// Store logs
			err = storage.StoreLogs(context.Background(), logs)
			assert.NoError(t, err)

			// Retrieve logs
			filter := logging.LogFilter{
				TaskID: taskID,
				Limit:  10,
				Offset: 0,
			}

			retrievedLogs, err := storage.GetLogs(context.Background(), filter)
			assert.NoError(t, err)
			require.Len(t, retrievedLogs, 2, "Expected 2 logs to be retrieved, but got %d", len(retrievedLogs))

			// Defensive programming: verify array has expected elements before accessing
			if len(retrievedLogs) >= 2 {
				assert.Equal(t, "Test log line 1", retrievedLogs[0].Content)
				assert.Equal(t, "Test log line 2", retrievedLogs[1].Content)
			} else {
				t.Errorf("Retrieved logs array has insufficient elements: got %d, want 2", len(retrievedLogs))
			}

			// Test log count
			count, err := storage.GetLogCount(context.Background(), taskID, &executionID)
			assert.NoError(t, err)
			assert.Equal(t, int64(2), count)
		})

		t.Run("service startup and shutdown integration", func(t *testing.T) {
			// Test complete service lifecycle
			storage, err := logging.NewPostgreSQLLogStorage(db.DB, getTestLoggingConfig(), slog.Default())
			require.NoError(t, err)
			require.NotNil(t, storage)

			// Test that service can be created and closed multiple times
			err = storage.Close()
			assert.NoError(t, err)

			// Test that service handles double close gracefully
			err = storage.Close()
			assert.NoError(t, err)
		})
	})
}

// TestLoggingServiceFailureScenarios tests various failure modes
func TestLoggingServiceFailureScenarios(t *testing.T) {
	testutil.WithTestDatabase(t, func(db *testutil.DatabaseHelper) {
		t.Run("storage service handles invalid data gracefully", func(t *testing.T) {
			storage, err := logging.NewPostgreSQLLogStorage(db.DB, getTestLoggingConfig(), slog.Default())
			require.NoError(t, err)
			defer storage.Close()

			// Test with invalid log entries
			invalidLogs := []logging.LogEntry{
				{}, // Empty entry - should fail validation
			}

			err = storage.StoreLogs(context.Background(), invalidLogs)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "invalid log entry")
		})

		t.Run("storage service handles context cancellation", func(t *testing.T) {
			storage, err := logging.NewPostgreSQLLogStorage(db.DB, getTestLoggingConfig(), slog.Default())
			require.NoError(t, err)
			defer storage.Close()

			// Test with cancelled context
			ctx, cancel := context.WithCancel(context.Background())
			cancel() // Cancel immediately

			logs := []logging.LogEntry{
				{
					TaskID:         uuid.New(),
					ExecutionID:    uuid.New(),
					Content:        "Test log",
					Stream:         "stdout",
					SequenceNumber: 1,
					Timestamp:      time.Now(),
					CreatedAt:      time.Now(),
				},
			}

			err = storage.StoreLogs(ctx, logs)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "context canceled")
		})

		t.Run("storage service handles timeout scenarios", func(t *testing.T) {
			storage, err := logging.NewPostgreSQLLogStorage(db.DB, getTestLoggingConfig(), slog.Default())
			require.NoError(t, err)
			defer storage.Close()

			// Test with timeout context
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Microsecond)
			defer cancel()

			// Let the timeout expire
			time.Sleep(2 * time.Microsecond)

			logs := []logging.LogEntry{
				{
					TaskID:         uuid.New(),
					ExecutionID:    uuid.New(),
					Content:        "Test log",
					Stream:         "stdout",
					SequenceNumber: 1,
					Timestamp:      time.Now(),
					CreatedAt:      time.Now(),
				},
			}

			err = storage.StoreLogs(ctx, logs)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "context deadline exceeded")
		})
	})
}

// TestLoggingServiceResourceCleanup tests proper resource management
func TestLoggingServiceResourceCleanup(t *testing.T) {
	testutil.WithTestDatabase(t, func(db *testutil.DatabaseHelper) {
		t.Run("storage service properly cleans up resources", func(t *testing.T) {
			// Create multiple storage instances to test resource cleanup
			for i := 0; i < 5; i++ {
				storage, err := logging.NewPostgreSQLLogStorage(db.DB, getTestLoggingConfig(), slog.Default())
				require.NoError(t, err)
				require.NotNil(t, storage)

				// Immediately close each instance
				err = storage.Close()
				assert.NoError(t, err)
			}
		})

		t.Run("storage service handles concurrent operations", func(t *testing.T) {
			storage, err := logging.NewPostgreSQLLogStorage(db.DB, getTestLoggingConfig(), slog.Default())
			require.NoError(t, err)
			defer storage.Close()

			// Create test user, task, and execution records for foreign key constraints
			userID := uuid.New()
			taskID := uuid.New()
			executionID := uuid.New()

			// Insert user
			_, err = db.DB.Pool.Exec(context.Background(),
				"INSERT INTO users (id, email, password_hash) VALUES ($1, $2, $3)",
				userID, "testuser2@example.com", "hashedpassword")
			require.NoError(t, err)

			// Insert task
			_, err = db.DB.Pool.Exec(context.Background(),
				"INSERT INTO tasks (id, user_id, name, script_content) VALUES ($1, $2, $3, $4)",
				taskID, userID, "test-task", "print('hello')")
			require.NoError(t, err)

			// Insert task execution
			_, err = db.DB.Pool.Exec(context.Background(),
				"INSERT INTO task_executions (id, task_id, status) VALUES ($1, $2, $3)",
				executionID, taskID, "running")
			require.NoError(t, err)

			// Start multiple goroutines storing logs concurrently
			done := make(chan bool, 3)
			for i := 0; i < 3; i++ {
				go func(seqNum int) {
					defer func() { done <- true }()

					logs := []logging.LogEntry{
						{
							TaskID:         taskID,
							ExecutionID:    executionID,
							Content:        fmt.Sprintf("Concurrent log %d", seqNum),
							Stream:         "stdout",
							SequenceNumber: int64(seqNum),
							Timestamp:      time.Now(),
							CreatedAt:      time.Now(),
						},
					}

					err := storage.StoreLogs(context.Background(), logs)
					assert.NoError(t, err)
				}(i)
			}

			// Wait for all goroutines to complete
			for i := 0; i < 3; i++ {
				<-done
			}

			// Verify all logs were stored
			count, err := storage.GetLogCount(context.Background(), taskID, &executionID)
			assert.NoError(t, err)
			assert.Equal(t, int64(3), count)
		})
	})
}

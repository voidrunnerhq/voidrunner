package database

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/voidrunnerhq/voidrunner/internal/config"
	"github.com/voidrunnerhq/voidrunner/internal/models"
)

// TestMain sets up integration test environment
func TestMain(m *testing.M) {
	// Skip integration tests if no database connection is available
	if os.Getenv("INTEGRATION_TESTS") != "true" {
		os.Exit(0)
	}

	// Run integration tests
	code := m.Run()
	os.Exit(code)
}

// setupTestDatabase creates a test database connection and runs migrations
func setupTestDatabase(t *testing.T) (*Connection, *Repositories) {
	t.Helper()

	// Skip if not running integration tests
	if os.Getenv("INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration test - set INTEGRATION_TESTS=true to run")
	}

	// Load test database configuration
	cfg := &config.DatabaseConfig{
		Host:     getEnvOrDefault("TEST_DB_HOST", "localhost"),
		Port:     getEnvOrDefault("TEST_DB_PORT", "5432"),
		User:     getEnvOrDefault("TEST_DB_USER", "postgres"),
		Password: getEnvOrDefault("TEST_DB_PASSWORD", ""),
		Database: getEnvOrDefault("TEST_DB_NAME", "voidrunner_test"),
		SSLMode:  getEnvOrDefault("TEST_DB_SSL_MODE", "disable"),
	}

	// Create database connection
	conn, err := NewConnection(cfg, nil)
	require.NoError(t, err, "Failed to create database connection")

	// Run migrations
	migrateConfig := &MigrateConfig{
		DatabaseConfig: cfg,
		MigrationsPath: "file://../../migrations",
		Logger:         nil,
	}

	err = MigrateUp(migrateConfig)
	require.NoError(t, err, "Failed to run database migrations")

	// Create repositories
	repos := NewRepositories(conn)

	// Clean up function
	t.Cleanup(func() {
		cleanupTestData(t, repos)
		conn.Close()
	})

	return conn, repos
}

// cleanupTestData removes all test data from the database
func cleanupTestData(t *testing.T, repos *Repositories) {
	t.Helper()
	
	// Clean up in reverse order due to foreign key constraints
	// Note: In a real test environment, you might want to use transactions
	// or a separate test database that gets reset between tests
}

func TestUserRepository_Integration(t *testing.T) {
	_, repos := setupTestDatabase(t)
	ctx := context.Background()

	t.Run("user CRUD operations", func(t *testing.T) {
		// Create a test user
		user := &models.User{
			Email:        "integration.test@example.com",
			PasswordHash: "hashed_password_123",
		}

		// Test Create
		err := repos.Users.Create(ctx, user)
		require.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, user.ID)
		assert.False(t, user.CreatedAt.IsZero())
		assert.False(t, user.UpdatedAt.IsZero())

		// Test GetByID
		retrievedUser, err := repos.Users.GetByID(ctx, user.ID)
		require.NoError(t, err)
		assert.Equal(t, user.Email, retrievedUser.Email)
		assert.Equal(t, user.PasswordHash, retrievedUser.PasswordHash)

		// Test GetByEmail
		userByEmail, err := repos.Users.GetByEmail(ctx, user.Email)
		require.NoError(t, err)
		assert.Equal(t, user.ID, userByEmail.ID)

		// Test Update
		user.Email = "updated.integration.test@example.com"
		err = repos.Users.Update(ctx, user)
		require.NoError(t, err)

		updatedUser, err := repos.Users.GetByID(ctx, user.ID)
		require.NoError(t, err)
		assert.Equal(t, "updated.integration.test@example.com", updatedUser.Email)
		assert.True(t, updatedUser.UpdatedAt.After(updatedUser.CreatedAt))

		// Test Count
		count, err := repos.Users.Count(ctx)
		require.NoError(t, err)
		assert.Greater(t, count, int64(0))

		// Test List
		users, err := repos.Users.List(ctx, 10, 0)
		require.NoError(t, err)
		assert.NotEmpty(t, users)

		// Test Delete
		err = repos.Users.Delete(ctx, user.ID)
		require.NoError(t, err)

		// Verify deletion
		_, err = repos.Users.GetByID(ctx, user.ID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestTaskRepository_Integration(t *testing.T) {
	_, repos := setupTestDatabase(t)
	ctx := context.Background()

	t.Run("task CRUD operations", func(t *testing.T) {
		// First create a user
		user := &models.User{
			Email:        "task.test@example.com",
			PasswordHash: "hashed_password_123",
		}
		err := repos.Users.Create(ctx, user)
		require.NoError(t, err)

		// Clean up user at the end
		defer repos.Users.Delete(ctx, user.ID)

		// Create test metadata
		metadata, _ := json.Marshal(map[string]interface{}{
			"environment": "test",
			"priority":    "high",
			"tags":        []string{"integration", "test"},
		})

		// Create a test task
		task := &models.Task{
			UserID:         user.ID,
			Name:           "Integration Test Task",
			Description:    stringPtr("Test task for integration testing"),
			ScriptContent:  "print('Hello from integration test')",
			ScriptType:     models.ScriptTypePython,
			Status:         models.TaskStatusPending,
			Priority:       1,
			TimeoutSeconds: 30,
			Metadata:       metadata,
		}

		// Test Create
		err = repos.Tasks.Create(ctx, task)
		require.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, task.ID)
		assert.False(t, task.CreatedAt.IsZero())
		assert.False(t, task.UpdatedAt.IsZero())

		// Test GetByID
		retrievedTask, err := repos.Tasks.GetByID(ctx, task.ID)
		require.NoError(t, err)
		assert.Equal(t, task.Name, retrievedTask.Name)
		assert.Equal(t, task.ScriptContent, retrievedTask.ScriptContent)
		assert.Equal(t, task.ScriptType, retrievedTask.ScriptType)
		assert.JSONEq(t, string(task.Metadata), string(retrievedTask.Metadata))

		// Test GetByUserID
		userTasks, err := repos.Tasks.GetByUserID(ctx, user.ID, 10, 0)
		require.NoError(t, err)
		assert.Len(t, userTasks, 1)
		assert.Equal(t, task.ID, userTasks[0].ID)

		// Test GetByStatus
		pendingTasks, err := repos.Tasks.GetByStatus(ctx, models.TaskStatusPending, 10, 0)
		require.NoError(t, err)
		assert.NotEmpty(t, pendingTasks)

		// Test UpdateStatus
		err = repos.Tasks.UpdateStatus(ctx, task.ID, models.TaskStatusRunning)
		require.NoError(t, err)

		updatedTask, err := repos.Tasks.GetByID(ctx, task.ID)
		require.NoError(t, err)
		assert.Equal(t, models.TaskStatusRunning, updatedTask.Status)

		// Test SearchByMetadata
		metadataQuery := `{"environment": "test"}`
		searchResults, err := repos.Tasks.SearchByMetadata(ctx, metadataQuery, 10, 0)
		require.NoError(t, err)
		assert.NotEmpty(t, searchResults)

		// Test Count operations
		totalCount, err := repos.Tasks.Count(ctx)
		require.NoError(t, err)
		assert.Greater(t, totalCount, int64(0))

		userTaskCount, err := repos.Tasks.CountByUserID(ctx, user.ID)
		require.NoError(t, err)
		assert.Equal(t, int64(1), userTaskCount)

		runningTaskCount, err := repos.Tasks.CountByStatus(ctx, models.TaskStatusRunning)
		require.NoError(t, err)
		assert.Greater(t, runningTaskCount, int64(0))

		// Test Delete
		err = repos.Tasks.Delete(ctx, task.ID)
		require.NoError(t, err)

		// Verify deletion
		_, err = repos.Tasks.GetByID(ctx, task.ID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestTaskExecutionRepository_Integration(t *testing.T) {
	_, repos := setupTestDatabase(t)
	ctx := context.Background()

	t.Run("task execution CRUD operations", func(t *testing.T) {
		// First create a user and task
		user := &models.User{
			Email:        "execution.test@example.com",
			PasswordHash: "hashed_password_123",
		}
		err := repos.Users.Create(ctx, user)
		require.NoError(t, err)

		task := &models.Task{
			UserID:         user.ID,
			Name:           "Execution Test Task",
			ScriptContent:  "print('test')",
			ScriptType:     models.ScriptTypePython,
			Status:         models.TaskStatusPending,
			Priority:       1,
			TimeoutSeconds: 30,
			Metadata:       json.RawMessage(`{}`),
		}
		err = repos.Tasks.Create(ctx, task)
		require.NoError(t, err)

		// Clean up at the end
		defer func() {
			repos.Tasks.Delete(ctx, task.ID)
			repos.Users.Delete(ctx, user.ID)
		}()

		// Create a test execution
		execution := &models.TaskExecution{
			TaskID: task.ID,
			Status: models.ExecutionStatusPending,
		}

		// Test Create
		err = repos.TaskExecutions.Create(ctx, execution)
		require.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, execution.ID)
		assert.False(t, execution.CreatedAt.IsZero())

		// Test GetByID
		retrievedExecution, err := repos.TaskExecutions.GetByID(ctx, execution.ID)
		require.NoError(t, err)
		assert.Equal(t, execution.TaskID, retrievedExecution.TaskID)
		assert.Equal(t, execution.Status, retrievedExecution.Status)

		// Test GetByTaskID
		taskExecutions, err := repos.TaskExecutions.GetByTaskID(ctx, task.ID, 10, 0)
		require.NoError(t, err)
		assert.Len(t, taskExecutions, 1)
		assert.Equal(t, execution.ID, taskExecutions[0].ID)

		// Test GetLatestByTaskID
		latestExecution, err := repos.TaskExecutions.GetLatestByTaskID(ctx, task.ID)
		require.NoError(t, err)
		assert.Equal(t, execution.ID, latestExecution.ID)

		// Test Update with execution results
		startTime := time.Now()
		endTime := startTime.Add(2 * time.Second)
		execution.Status = models.ExecutionStatusCompleted
		execution.ReturnCode = intPtr(0)
		execution.Stdout = stringPtr("Test output")
		execution.Stderr = stringPtr("")
		execution.ExecutionTimeMs = intPtr(2000)
		execution.MemoryUsageBytes = int64Ptr(1024 * 1024) // 1MB
		execution.StartedAt = &startTime
		execution.CompletedAt = &endTime

		err = repos.TaskExecutions.Update(ctx, execution)
		require.NoError(t, err)

		updatedExecution, err := repos.TaskExecutions.GetByID(ctx, execution.ID)
		require.NoError(t, err)
		assert.Equal(t, models.ExecutionStatusCompleted, updatedExecution.Status)
		assert.Equal(t, 0, *updatedExecution.ReturnCode)
		assert.Equal(t, "Test output", *updatedExecution.Stdout)

		// Test GetByStatus
		completedExecutions, err := repos.TaskExecutions.GetByStatus(ctx, models.ExecutionStatusCompleted, 10, 0)
		require.NoError(t, err)
		assert.NotEmpty(t, completedExecutions)

		// Test Count operations
		totalCount, err := repos.TaskExecutions.Count(ctx)
		require.NoError(t, err)
		assert.Greater(t, totalCount, int64(0))

		taskExecutionCount, err := repos.TaskExecutions.CountByTaskID(ctx, task.ID)
		require.NoError(t, err)
		assert.Equal(t, int64(1), taskExecutionCount)

		completedCount, err := repos.TaskExecutions.CountByStatus(ctx, models.ExecutionStatusCompleted)
		require.NoError(t, err)
		assert.Greater(t, completedCount, int64(0))

		// Test Delete
		err = repos.TaskExecutions.Delete(ctx, execution.ID)
		require.NoError(t, err)

		// Verify deletion
		_, err = repos.TaskExecutions.GetByID(ctx, execution.ID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

// BenchmarkDatabaseOperations provides performance benchmarks
func BenchmarkDatabaseOperations(b *testing.B) {
	if os.Getenv("INTEGRATION_TESTS") != "true" {
		b.Skip("Skipping benchmark - set INTEGRATION_TESTS=true to run")
	}

	_, repos := setupBenchmarkDatabase(b)
	ctx := context.Background()

	// Create test user for benchmarks
	user := &models.User{
		Email:        "benchmark@example.com",
		PasswordHash: "hashed_password",
	}
	repos.Users.Create(ctx, user)
	defer repos.Users.Delete(ctx, user.ID)

	b.Run("UserRepository_Create", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			user := &models.User{
				Email:        "bench.user." + string(rune(i)) + "@example.com",
				PasswordHash: "hashed_password",
			}
			repos.Users.Create(ctx, user)
			repos.Users.Delete(ctx, user.ID) // Clean up
		}
	})

	b.Run("UserRepository_GetByID", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			repos.Users.GetByID(ctx, user.ID)
		}
	})

	b.Run("TaskRepository_Create", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			task := &models.Task{
				UserID:         user.ID,
				Name:           "Benchmark Task",
				ScriptContent:  "print('benchmark')",
				ScriptType:     models.ScriptTypePython,
				Status:         models.TaskStatusPending,
				Priority:       1,
				TimeoutSeconds: 30,
				Metadata:       json.RawMessage(`{}`),
			}
			repos.Tasks.Create(ctx, task)
			repos.Tasks.Delete(ctx, task.ID) // Clean up
		}
	})
}

// Helper functions
func setupBenchmarkDatabase(b *testing.B) (*Connection, *Repositories) {
	b.Helper()

	cfg := &config.DatabaseConfig{
		Host:     getEnvOrDefault("TEST_DB_HOST", "localhost"),
		Port:     getEnvOrDefault("TEST_DB_PORT", "5432"),
		User:     getEnvOrDefault("TEST_DB_USER", "postgres"),
		Password: getEnvOrDefault("TEST_DB_PASSWORD", ""),
		Database: getEnvOrDefault("TEST_DB_NAME", "voidrunner_test"),
		SSLMode:  getEnvOrDefault("TEST_DB_SSL_MODE", "disable"),
	}

	conn, err := NewConnection(cfg, nil)
	if err != nil {
		b.Fatalf("Failed to create database connection: %v", err)
	}

	repos := NewRepositories(conn)

	b.Cleanup(func() {
		conn.Close()
	})

	return conn, repos
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
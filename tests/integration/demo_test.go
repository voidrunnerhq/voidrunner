//go:build integration

package integration_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/voidrunnerhq/voidrunner/internal/models"
	"github.com/voidrunnerhq/voidrunner/tests/testutil"
)

// TestDatabaseIntegrationDemo demonstrates the database integration testing infrastructure
func TestDatabaseIntegrationDemo(t *testing.T) {
	// This test demonstrates various patterns for database integration testing
	// It will be skipped if no test database is available

	t.Run("simple database test with helper", func(t *testing.T) {
		testutil.WithTestDatabase(t, func(db *testutil.DatabaseHelper) {
			ctx := context.Background()

			// Create a user using the factory
			user := testutil.NewUserFactory().
				WithEmail("demo@example.com").
				WithName("Demo User").
				Build()

			err := db.Repositories.Users.Create(ctx, user)
			require.NoError(t, err)

			// Verify the user was created
			retrieved, err := db.Repositories.Users.GetByEmail(ctx, user.Email)
			require.NoError(t, err)
			assert.Equal(t, user.Name, retrieved.Name)
		})
	})

	t.Run("seeded database test", func(t *testing.T) {
		fixtures := testutil.NewAllFixtures()

		testutil.WithSeededTestDatabase(t, fixtures, func(db *testutil.DatabaseHelper) {
			ctx := context.Background()

			// Test with pre-seeded data
			user, err := db.Repositories.Users.GetByEmail(ctx, fixtures.Users.AdminUser.Email)
			require.NoError(t, err)
			assert.Equal(t, "Admin User", user.Name)

			// Test task relationships
			task, err := db.Repositories.Tasks.GetByID(ctx, fixtures.Tasks.PendingTask.ID)
			require.NoError(t, err)
			assert.Equal(t, models.TaskStatusPending, task.Status)
			assert.Equal(t, fixtures.Users.RegularUser.ID, task.UserID)
		})
	})

	t.Run("complete workflow test", func(t *testing.T) {
		testutil.WithTestDatabase(t, func(db *testutil.DatabaseHelper) {
			ctx := context.Background()

			// Create user
			user := db.CreateMinimalUser(t, ctx, "workflow@example.com", "Workflow User")

			// Create task
			task := testutil.NewTaskFactory(user.ID).
				WithName("Demo Workflow Task").
				WithPythonScript("print('Hello from demo')").
				HighPriority().
				Build()

			err := db.Repositories.Tasks.Create(ctx, task)
			require.NoError(t, err)

			// Create execution
			execution := testutil.NewExecutionFactory(task.ID).
				Successful().
				WithStdout("Hello from demo\n").
				WithExecutionTime(200).
				Build()

			err = db.Repositories.TaskExecutions.Create(ctx, execution)
			require.NoError(t, err)

			// Validate the execution was created successfully
			assert.NotEmpty(t, execution.ID)
			assert.Equal(t, task.ID, execution.TaskID)

			// Test that we can retrieve the execution by task
			executions, err := db.Repositories.TaskExecutions.GetByTaskID(ctx, task.ID, 10, 0)
			require.NoError(t, err)
			assert.Len(t, executions, 1)
			assert.Equal(t, execution.ID, executions[0].ID)
		})
	})

	t.Run("error handling validation", func(t *testing.T) {
		testutil.WithTestDatabase(t, func(db *testutil.DatabaseHelper) {
			ctx := context.Background()

			// Test foreign key constraint - task with invalid user ID
			invalidTask := testutil.NewTaskFactory(testutil.NewUserFactory().Build().ID).Build()
			err := db.Repositories.Tasks.Create(ctx, invalidTask)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "does not exist")
		})
	})
}

// TestFactoryPatterns demonstrates the factory patterns for test data creation
func TestFactoryPatterns(t *testing.T) {
	t.Run("user factory patterns", func(t *testing.T) {
		// Basic user
		user := testutil.NewUserFactory().Build()
		assert.NotEmpty(t, user.ID)
		assert.NotEmpty(t, user.Email)
		assert.NotEmpty(t, user.PasswordHash)

		// Admin user (Admin() method overrides email and name)
		adminUser := testutil.NewUserFactory().
			WithEmail("custom@example.com").
			WithName("Custom User").
			Admin().
			Build()

		assert.Equal(t, "admin@voidrunner.dev", adminUser.Email)
		assert.Equal(t, "Admin User", adminUser.Name)

		// Customized user (without Admin())
		customUser := testutil.NewUserFactory().
			WithEmail("custom@example.com").
			WithName("Custom User").
			Build()

		assert.Equal(t, "custom@example.com", customUser.Email)
		assert.Equal(t, "Custom User", customUser.Name)
	})

	t.Run("task factory patterns", func(t *testing.T) {
		userID := testutil.NewUserFactory().Build().ID

		// Basic task
		task := testutil.NewTaskFactory(userID).Build()
		assert.NotEmpty(t, task.ID)
		assert.Equal(t, userID, task.UserID)
		assert.Equal(t, models.ScriptTypePython, task.ScriptType) // Default

		// Complex task
		complexTask := testutil.NewTaskFactory(userID).
			WithName("Complex Task").
			WithJavaScriptScript("console.log('Hello')").
			HighPriority().
			WithTimeout(120).
			WithMetadataField("environment", "test").
			WithMetadataField("version", "1.0").
			Completed().
			Build()

		assert.Equal(t, "Complex Task", complexTask.Name)
		assert.Equal(t, models.ScriptTypeJavaScript, complexTask.ScriptType)
		assert.Equal(t, models.TaskStatusCompleted, complexTask.Status)
		assert.Equal(t, 10, complexTask.Priority) // High priority
		assert.Equal(t, 120, complexTask.TimeoutSeconds)
		assert.Equal(t, "test", complexTask.Metadata["environment"])
		assert.Equal(t, "1.0", complexTask.Metadata["version"])
	})

	t.Run("execution factory patterns", func(t *testing.T) {
		taskID := testutil.NewTaskFactory(testutil.NewUserFactory().Build().ID).Build().ID

		// Successful execution
		successExec := testutil.NewExecutionFactory(taskID).
			Successful().
			WithStdout("Success output").
			WithExecutionTime(1500).
			WithMemoryUsage(2048576). // 2MB
			Build()

		assert.Equal(t, models.ExecutionStatusCompleted, successExec.Status)
		assert.Equal(t, 0, *successExec.ReturnCode)
		assert.Equal(t, "Success output", *successExec.Stdout)
		assert.Equal(t, 1500, *successExec.ExecutionTimeMs)
		assert.Equal(t, int64(2048576), *successExec.MemoryUsageBytes)

		// Failed execution
		failedExec := testutil.NewExecutionFactory(taskID).
			Failed().
			WithReturnCode(1).
			WithStderr("Error occurred").
			Build()

		assert.Equal(t, models.ExecutionStatusFailed, failedExec.Status)
		assert.Equal(t, 1, *failedExec.ReturnCode)
		assert.Equal(t, "Error occurred", *failedExec.Stderr)

		// Timeout execution
		timeoutExec := testutil.NewExecutionFactory(taskID).
			Timeout().
			WithExecutionTime(30000).
			WithStderr("Process timed out").
			Build()

		assert.Equal(t, models.ExecutionStatusTimeout, timeoutExec.Status)
		assert.Equal(t, 30000, *timeoutExec.ExecutionTimeMs)
		assert.Equal(t, "Process timed out", *timeoutExec.Stderr)
	})
}

// TestFixturesUsage demonstrates using pre-defined fixtures
func TestFixturesUsage(t *testing.T) {
	fixtures := testutil.NewAllFixtures()

	t.Run("user fixtures", func(t *testing.T) {
		assert.Equal(t, "admin@voidrunner.dev", fixtures.Users.AdminUser.Email)
		assert.Equal(t, "Admin User", fixtures.Users.AdminUser.Name)

		assert.Equal(t, "user@voidrunner.dev", fixtures.Users.RegularUser.Email)
		assert.Equal(t, "Regular User", fixtures.Users.RegularUser.Name)
	})

	t.Run("task fixtures", func(t *testing.T) {
		assert.Equal(t, models.TaskStatusPending, fixtures.Tasks.PendingTask.Status)
		assert.Equal(t, models.TaskStatusRunning, fixtures.Tasks.RunningTask.Status)
		assert.Equal(t, models.TaskStatusCompleted, fixtures.Tasks.CompletedTask.Status)
		assert.Equal(t, models.TaskStatusFailed, fixtures.Tasks.FailedTask.Status)

		// All tasks should belong to the regular user
		assert.Equal(t, fixtures.Users.RegularUser.ID, fixtures.Tasks.PendingTask.UserID)
		assert.Equal(t, fixtures.Users.RegularUser.ID, fixtures.Tasks.RunningTask.UserID)
	})

	t.Run("execution fixtures", func(t *testing.T) {
		assert.Equal(t, models.ExecutionStatusCompleted, fixtures.Executions.SuccessfulExecution.Status)
		assert.Equal(t, models.ExecutionStatusFailed, fixtures.Executions.FailedExecution.Status)
		assert.Equal(t, models.ExecutionStatusTimeout, fixtures.Executions.TimeoutExecution.Status)

		// Successful execution should be linked to completed task
		assert.Equal(t, fixtures.Tasks.CompletedTask.ID, fixtures.Executions.SuccessfulExecution.TaskID)
		assert.Equal(t, 0, *fixtures.Executions.SuccessfulExecution.ReturnCode)

		// Failed execution should be linked to failed task
		assert.Equal(t, fixtures.Tasks.FailedTask.ID, fixtures.Executions.FailedExecution.TaskID)
		assert.Equal(t, 1, *fixtures.Executions.FailedExecution.ReturnCode)
	})
}

// BenchmarkIntegrationInfrastructure benchmarks the testing infrastructure performance
func BenchmarkIntegrationInfrastructure(b *testing.B) {
	if testing.Short() {
		b.Skip("Skipping benchmarks in short mode")
	}

	b.Run("factory creation", func(b *testing.B) {
		userID := testutil.NewUserFactory().Build().ID

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			user := testutil.NewUserFactory().Build()
			task := testutil.NewTaskFactory(userID).Build()
			_ = testutil.NewExecutionFactory(task.ID).Build()
			_ = user // Use variables to avoid optimization
		}
	})

	b.Run("fixture creation", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = testutil.NewAllFixtures()
		}
	})
}

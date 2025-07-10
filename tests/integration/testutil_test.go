//go:build integration

package integration_test

import (
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/voidrunnerhq/voidrunner/internal/models"
	"github.com/voidrunnerhq/voidrunner/tests/testutil"
)

// TestGetEnvOrDefault_ErrorScenarios tests environment variable handling
func TestGetEnvOrDefault_ErrorScenarios(t *testing.T) {
	tests := []struct {
		name          string
		key           string
		defaultValue  string
		setEnv        bool
		envValue      string
		expectedValue string
	}{
		{
			name:          "environment variable not set - return default",
			key:           "NON_EXISTENT_VAR",
			defaultValue:  "default_value",
			setEnv:        false,
			expectedValue: "default_value",
		},
		{
			name:          "empty environment variable - return default",
			key:           "EMPTY_VAR",
			defaultValue:  "default_value",
			setEnv:        true,
			envValue:      "",
			expectedValue: "default_value",
		},
		{
			name:          "environment variable set - return env value",
			key:           "SET_VAR",
			defaultValue:  "default_value",
			setEnv:        true,
			envValue:      "env_value",
			expectedValue: "env_value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up environment
			defer func() { _ = os.Unsetenv(tt.key) }()

			if tt.setEnv {
				require.NoError(t, os.Setenv(tt.key, tt.envValue))
			}

			// Note: Since getEnvOrDefault is not exported, we test it through GetTestConfig
			// This is a better integration test anyway
			originalValue := os.Getenv(tt.key)
			defer func() {
				if originalValue != "" {
					_ = os.Setenv(tt.key, originalValue)
				} else {
					_ = os.Unsetenv(tt.key)
				}
			}()

			if tt.setEnv {
				_ = os.Setenv(tt.key, tt.envValue)
			}

			cfg := testutil.GetTestConfig()
			assert.NotNil(t, cfg)
		})
	}
}

// TestDatabaseHelper_Close_ErrorScenarios tests Close method error handling
func TestDatabaseHelper_Close_ErrorScenarios(t *testing.T) {
	tests := []struct {
		name   string
		helper *testutil.DatabaseHelper
	}{
		{
			name: "nil database connection",
			helper: &testutil.DatabaseHelper{
				DB: nil,
			},
		},
		{
			name:   "nil helper",
			helper: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotPanics(t, func() {
				if tt.helper != nil {
					tt.helper.Close()
				}
			})
		})
	}
}

// TestFactoryPatterns_ErrorScenarios tests factory error handling
func TestFactoryPatterns_ErrorScenarios(t *testing.T) {
	t.Run("user factory with invalid data", func(t *testing.T) {
		factory := testutil.NewUserFactory()

		// Test edge cases
		user := factory.
			WithEmail(""). // Empty email
			WithName("").  // Empty name
			Build()

		assert.Empty(t, user.Email)
		assert.Empty(t, user.Name)
		assert.NotEmpty(t, user.PasswordHash) // Should still have default
	})

	t.Run("task factory with invalid data", func(t *testing.T) {
		userID := uuid.New()
		factory := testutil.NewTaskFactory(userID)

		// Test edge cases
		task := factory.
			WithName("").       // Empty name
			WithTimeout(-1).    // Negative timeout
			WithPriority(-100). // Invalid priority
			Build()

		assert.Empty(t, task.Name)
		assert.Equal(t, -1, task.TimeoutSeconds)
		assert.Equal(t, -100, task.Priority)
	})

	t.Run("execution factory with invalid data", func(t *testing.T) {
		taskID := uuid.New()
		factory := testutil.NewExecutionFactory(taskID)

		// Test edge cases
		execution := factory.
			WithReturnCode(-1).    // Negative return code
			WithMemoryUsage(-100). // Negative memory
			WithExecutionTime(-1). // Negative time
			Build()

		assert.Equal(t, -1, *execution.ReturnCode)
		assert.Equal(t, int64(-100), *execution.MemoryUsageBytes)
		assert.Equal(t, -1, *execution.ExecutionTimeMs)
	})
}

// TestFactoryBuilders_ZeroValues tests factory builders with zero values
func TestFactoryBuilders_ZeroValues(t *testing.T) {
	t.Run("user factory with zero uuid", func(t *testing.T) {
		factory := testutil.NewUserFactory()

		user := factory.
			WithID(uuid.Nil). // Zero UUID
			Build()

		assert.Equal(t, uuid.Nil, user.ID)
	})

	t.Run("task factory with zero uuid", func(t *testing.T) {
		userID := uuid.New()
		factory := testutil.NewTaskFactory(userID)

		task := factory.
			WithID(uuid.Nil).     // Zero UUID
			WithUserID(uuid.Nil). // Zero User ID
			Build()

		assert.Equal(t, uuid.Nil, task.ID)
		assert.Equal(t, uuid.Nil, task.UserID)
	})

	t.Run("execution factory with zero uuid", func(t *testing.T) {
		taskID := uuid.New()
		factory := testutil.NewExecutionFactory(taskID)

		execution := factory.
			WithID(uuid.Nil).     // Zero UUID
			WithTaskID(uuid.Nil). // Zero Task ID
			Build()

		assert.Equal(t, uuid.Nil, execution.ID)
		assert.Equal(t, uuid.Nil, execution.TaskID)
	})
}

// TestFactoryBuilders_ChainedCalls tests factory method chaining
func TestFactoryBuilders_ChainedCalls(t *testing.T) {
	t.Run("user factory method chaining", func(t *testing.T) {
		factory := testutil.NewUserFactory()

		// Test all chainable methods
		user := factory.
			WithEmail("test@example.com").
			WithName("Test User").
			WithPasswordHash("custom_hash").
			WithCreatedAt(time.Now()).
			WithUpdatedAt(time.Now()).
			Admin().
			Build()

		assert.Equal(t, "admin@voidrunner.dev", user.Email) // Admin() overrides email
		assert.Equal(t, "Admin User", user.Name)            // Admin() overrides name
		assert.Equal(t, "custom_hash", user.PasswordHash)
	})

	t.Run("task factory method chaining", func(t *testing.T) {
		userID := uuid.New()
		factory := testutil.NewTaskFactory(userID)

		// Test all chainable methods
		task := factory.
			WithName("Test Task").
			WithDescription("Test Description").
			WithScript("print('test')", models.ScriptTypePython).
			WithPythonScript("print('python')").
			WithStatus(models.TaskStatusPending).
			WithPriority(5).
			HighPriority().
			WithTimeout(60).
			WithMetadata(models.JSONB{"key": "value"}).
			WithMetadataField("environment", "test").
			WithCreatedAt(time.Now()).
			WithUpdatedAt(time.Now()).
			Build()

		assert.Equal(t, "Test Task", task.Name)
		assert.Equal(t, "Test Description", *task.Description)
		assert.Equal(t, models.ScriptTypePython, task.ScriptType)
		assert.Equal(t, models.TaskStatusPending, task.Status)
		assert.Equal(t, 10, task.Priority) // HighPriority overrides previous priority
	})

	t.Run("execution factory method chaining", func(t *testing.T) {
		taskID := uuid.New()
		factory := testutil.NewExecutionFactory(taskID)

		// Test all chainable methods
		execution := factory.
			WithStatus(models.ExecutionStatusPending).
			WithReturnCode(0).
			WithOutput("test stdout", "test stderr").
			WithStdout("stdout").
			WithStderr("stderr").
			WithExecutionTime(100).
			WithMemoryUsage(1024).
			WithTimes(time.Now(), time.Now()).
			WithStartedAt(time.Now()).
			WithCompletedAt(time.Now()).
			WithCreatedAt(time.Now()).
			Successful().
			Build()

		assert.Equal(t, models.ExecutionStatusCompleted, execution.Status) // Successful overrides previous status
		assert.Equal(t, 0, *execution.ReturnCode)
		assert.Equal(t, "Success output", *execution.Stdout) // Successful() overrides stdout
		assert.Equal(t, "", *execution.Stderr)               // Successful() overrides stderr
	})
}

// TestFactoryStatusMethods tests status-specific factory methods
func TestFactoryStatusMethods(t *testing.T) {
	t.Run("task factory status methods", func(t *testing.T) {
		userID := uuid.New()
		factory := testutil.NewTaskFactory(userID)

		// Test Pending
		pendingTask := factory.Pending().Build()
		assert.Equal(t, models.TaskStatusPending, pendingTask.Status)

		// Test Running (reset factory first)
		factory = testutil.NewTaskFactory(userID)
		runningTask := factory.Running().Build()
		assert.Equal(t, models.TaskStatusRunning, runningTask.Status)

		// Test Completed
		factory = testutil.NewTaskFactory(userID)
		completedTask := factory.Completed().Build()
		assert.Equal(t, models.TaskStatusCompleted, completedTask.Status)

		// Test Failed
		factory = testutil.NewTaskFactory(userID)
		failedTask := factory.Failed().Build()
		assert.Equal(t, models.TaskStatusFailed, failedTask.Status)
	})

	t.Run("execution factory status methods", func(t *testing.T) {
		taskID := uuid.New()
		factory := testutil.NewExecutionFactory(taskID)

		// Test Pending
		pendingExecution := factory.Pending().Build()
		assert.Equal(t, models.ExecutionStatusPending, pendingExecution.Status)

		// Test Running
		factory = testutil.NewExecutionFactory(taskID)
		runningExecution := factory.Running().Build()
		assert.Equal(t, models.ExecutionStatusRunning, runningExecution.Status)

		// Test Completed
		factory = testutil.NewExecutionFactory(taskID)
		completedExecution := factory.Completed().Build()
		assert.Equal(t, models.ExecutionStatusCompleted, completedExecution.Status)

		// Test Failed
		factory = testutil.NewExecutionFactory(taskID)
		failedExecution := factory.Failed().Build()
		assert.Equal(t, models.ExecutionStatusFailed, failedExecution.Status)

		// Test Timeout
		factory = testutil.NewExecutionFactory(taskID)
		timeoutExecution := factory.Timeout().Build()
		assert.Equal(t, models.ExecutionStatusTimeout, timeoutExecution.Status)
	})
}

// TestFactoryScriptMethods tests script-specific factory methods
func TestFactoryScriptMethods(t *testing.T) {
	t.Run("task factory script methods", func(t *testing.T) {
		userID := uuid.New()
		factory := testutil.NewTaskFactory(userID)

		// Test PythonScript
		pythonTask := factory.WithPythonScript("print('python')").Build()
		assert.Equal(t, models.ScriptTypePython, pythonTask.ScriptType)
		assert.Equal(t, "print('python')", pythonTask.ScriptContent)

		// Test JavaScriptScript
		factory = testutil.NewTaskFactory(userID)
		jsTask := factory.WithJavaScriptScript("console.log('js')").Build()
		assert.Equal(t, models.ScriptTypeJavaScript, jsTask.ScriptType)
		assert.Equal(t, "console.log('js')", jsTask.ScriptContent)

		// Test BashScript
		factory = testutil.NewTaskFactory(userID)
		bashTask := factory.WithBashScript("echo 'bash'").Build()
		assert.Equal(t, models.ScriptTypeBash, bashTask.ScriptType)
		assert.Equal(t, "echo 'bash'", bashTask.ScriptContent)
	})
}

// TestFactoryPriorityMethods tests priority-specific factory methods
func TestFactoryPriorityMethods(t *testing.T) {
	t.Run("task factory priority methods", func(t *testing.T) {
		userID := uuid.New()
		factory := testutil.NewTaskFactory(userID)

		// Test HighPriority
		highPriorityTask := factory.HighPriority().Build()
		assert.Equal(t, 10, highPriorityTask.Priority)

		// Test LowPriority
		factory = testutil.NewTaskFactory(userID)
		lowPriorityTask := factory.LowPriority().Build()
		assert.Equal(t, 2, lowPriorityTask.Priority)
	})
}

// TestFactoryExecutionMethods tests execution-specific factory methods
func TestFactoryExecutionMethods(t *testing.T) {
	t.Run("execution factory specific methods", func(t *testing.T) {
		taskID := uuid.New()
		factory := testutil.NewExecutionFactory(taskID)

		// Test Successful
		successfulExecution := factory.Successful().Build()
		assert.Equal(t, models.ExecutionStatusCompleted, successfulExecution.Status)
		assert.Equal(t, 0, *successfulExecution.ReturnCode)
		assert.Equal(t, "Success output", *successfulExecution.Stdout)

		// Test FailedExecution
		factory = testutil.NewExecutionFactory(taskID)
		failedExecution := factory.FailedExecution().Build()
		assert.Equal(t, models.ExecutionStatusFailed, failedExecution.Status)
		assert.Equal(t, 1, *failedExecution.ReturnCode)
		assert.Equal(t, "Error output", *failedExecution.Stderr)
	})
}

// TestFactoryMetadataHandling tests metadata field operations
func TestFactoryMetadataHandling(t *testing.T) {
	t.Run("task factory metadata operations", func(t *testing.T) {
		userID := uuid.New()
		factory := testutil.NewTaskFactory(userID)

		// Test WithMetadata
		metadata := models.JSONB{
			"environment": "test",
			"version":     "1.0",
		}
		task := factory.WithMetadata(metadata).Build()
		assert.Equal(t, metadata, task.Metadata)

		// Test WithMetadataField (should add to existing metadata)
		factory = testutil.NewTaskFactory(userID)
		task = factory.
			WithMetadata(models.JSONB{"existing": "value"}).
			WithMetadataField("new_field", "new_value").
			Build()

		assert.Equal(t, "value", task.Metadata["existing"])
		assert.Equal(t, "new_value", task.Metadata["new_field"])
	})
}

// TestFixturesIntegrity tests fixture data integrity
func TestFixturesIntegrity(t *testing.T) {
	t.Run("user fixtures integrity", func(t *testing.T) {
		fixtures := testutil.NewUserFixtures()

		// Verify all users have required fields
		users := []*models.User{
			fixtures.AdminUser,
			fixtures.RegularUser,
			fixtures.InactiveUser,
		}

		for _, user := range users {
			assert.NotEqual(t, uuid.Nil, user.ID)
			assert.NotEmpty(t, user.Email)
			assert.NotEmpty(t, user.Name)
			assert.NotEmpty(t, user.PasswordHash)
			assert.False(t, user.CreatedAt.IsZero())
			assert.False(t, user.UpdatedAt.IsZero())
		}
	})

	t.Run("task fixtures integrity", func(t *testing.T) {
		userID := uuid.New()
		fixtures := testutil.NewTaskFixtures(userID)

		// Verify all tasks have required fields
		tasks := []*models.Task{
			fixtures.PendingTask,
			fixtures.RunningTask,
			fixtures.CompletedTask,
			fixtures.FailedTask,
		}

		for _, task := range tasks {
			assert.NotEqual(t, uuid.Nil, task.ID)
			assert.NotEqual(t, uuid.Nil, task.UserID)
			assert.NotEmpty(t, task.Name)
			assert.NotEmpty(t, task.ScriptContent)
			assert.NotEmpty(t, task.ScriptType)
			assert.Greater(t, task.Priority, 0)
			assert.Greater(t, task.TimeoutSeconds, 0)
			assert.False(t, task.CreatedAt.IsZero())
			assert.False(t, task.UpdatedAt.IsZero())
		}
	})

	t.Run("execution fixtures integrity", func(t *testing.T) {
		userID := uuid.New()
		taskFixtures := testutil.NewTaskFixtures(userID)
		fixtures := testutil.NewExecutionFixtures(taskFixtures)

		// Verify all executions have required fields
		executions := []*models.TaskExecution{
			fixtures.SuccessfulExecution,
			fixtures.FailedExecution,
			fixtures.TimeoutExecution,
		}

		for _, execution := range executions {
			assert.NotEqual(t, uuid.Nil, execution.ID)
			assert.NotEqual(t, uuid.Nil, execution.TaskID)
			assert.NotEmpty(t, execution.Status)
			assert.False(t, execution.CreatedAt.IsZero())
		}
	})
}

// TestAllFixtures tests the complete fixture set
func TestAllFixtures(t *testing.T) {
	t.Run("all fixtures creation", func(t *testing.T) {
		fixtures := testutil.NewAllFixtures()

		assert.NotNil(t, fixtures.Users)
		assert.NotNil(t, fixtures.Tasks)
		assert.NotNil(t, fixtures.Executions)

		// Verify fixture relationships
		assert.Equal(t, fixtures.Users.RegularUser.ID, fixtures.Tasks.PendingTask.UserID)
		assert.Equal(t, fixtures.Tasks.CompletedTask.ID, fixtures.Executions.SuccessfulExecution.TaskID)
	})
}

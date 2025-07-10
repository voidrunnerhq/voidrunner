//go:build integration

package integration_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/voidrunnerhq/voidrunner/internal/models"
	"github.com/voidrunnerhq/voidrunner/tests/testutil"
)

// DatabaseIntegrationSuite provides enhanced database integration testing
type DatabaseIntegrationSuite struct {
	suite.Suite
	DB       *testutil.DatabaseHelper
	Fixtures *testutil.AllFixtures
}

// SetupSuite runs once before all tests
func (s *DatabaseIntegrationSuite) SetupSuite() {
	s.DB = testutil.NewDatabaseHelper(s.T())
	s.Fixtures = testutil.NewAllFixtures()
}

// TearDownSuite runs once after all tests
func (s *DatabaseIntegrationSuite) TearDownSuite() {
	if s.DB != nil {
		s.DB.Close()
	}
}

// SetupTest runs before each test
func (s *DatabaseIntegrationSuite) SetupTest() {
	s.DB.CleanupDatabase(s.T())
}

// TestDatabaseIntegrationSuite runs the enhanced integration test suite
func TestDatabaseIntegrationSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping database integration tests in short mode")
	}

	suite.Run(t, new(DatabaseIntegrationSuite))
}

// TestUserLifecycle validates complete user lifecycle with real database
func (s *DatabaseIntegrationSuite) TestUserLifecycle() {
	ctx := context.Background()

	s.Run("create user with factory", func() {
		user := testutil.NewUserFactory().
			WithEmail("lifecycle@test.com").
			WithName("Lifecycle User").
			Build()

		err := s.DB.Repositories.Users.Create(ctx, user)
		require.NoError(s.T(), err)
		assert.NotEmpty(s.T(), user.ID)
		assert.NotEmpty(s.T(), user.CreatedAt)

		// Verify user was created
		retrieved, err := s.DB.Repositories.Users.GetByEmail(ctx, user.Email)
		require.NoError(s.T(), err)
		assert.Equal(s.T(), user.Name, retrieved.Name)
		assert.Equal(s.T(), user.Email, retrieved.Email)
	})

	s.Run("user email uniqueness constraint", func() {
		user1 := testutil.NewUserFactory().
			WithEmail("unique@test.com").
			Build()

		err := s.DB.Repositories.Users.Create(ctx, user1)
		require.NoError(s.T(), err)

		// Try to create another user with same email
		user2 := testutil.NewUserFactory().
			WithEmail("unique@test.com").
			Build()

		err = s.DB.Repositories.Users.Create(ctx, user2)
		assert.Error(s.T(), err)
		assert.Contains(s.T(), err.Error(), "already exists") // PostgreSQL constraint error
	})
}

// TestTaskLifecycle validates complete task lifecycle with real database
func (s *DatabaseIntegrationSuite) TestTaskLifecycle() {
	ctx := context.Background()

	s.Run("complete task workflow", func() {
		// Create user first
		user := s.DB.CreateMinimalUser(s.T(), ctx, "task-workflow@test.com", "Task User")

		// Create task using factory
		task := testutil.NewTaskFactory(user.ID).
			WithName("Workflow Task").
			WithPythonScript("print('Hello, World!')").
			HighPriority().
			WithTimeout(60).
			WithMetadataField("environment", "integration").
			WithMetadataField("test_type", "workflow").
			Build()

		// Create task
		err := s.DB.Repositories.Tasks.Create(ctx, task)
		require.NoError(s.T(), err)
		assert.NotEmpty(s.T(), task.ID)

		// Update task status to running
		err = s.DB.Repositories.Tasks.UpdateStatus(ctx, task.ID, models.TaskStatusRunning)
		require.NoError(s.T(), err)

		// Verify status update
		retrieved, err := s.DB.Repositories.Tasks.GetByID(ctx, task.ID)
		require.NoError(s.T(), err)
		assert.Equal(s.T(), models.TaskStatusRunning, retrieved.Status)

		// Create task execution
		execution := testutil.NewExecutionFactory(task.ID).
			Running().
			Build()

		err = s.DB.Repositories.TaskExecutions.Create(ctx, execution)
		require.NoError(s.T(), err)

		// Complete the execution
		execution.Status = models.ExecutionStatusCompleted
		execution.ReturnCode = testutil.IntPtr(0)
		execution.Stdout = testutil.StringPtr("Hello, World!\n")
		execution.ExecutionTimeMs = testutil.IntPtr(150)

		err = s.DB.Repositories.TaskExecutions.Update(ctx, execution)
		require.NoError(s.T(), err)

		// Update task to completed
		err = s.DB.Repositories.Tasks.UpdateStatus(ctx, task.ID, models.TaskStatusCompleted)
		require.NoError(s.T(), err)

		// Verify final state
		finalTask, err := s.DB.Repositories.Tasks.GetByID(ctx, task.ID)
		require.NoError(s.T(), err)
		assert.Equal(s.T(), models.TaskStatusCompleted, finalTask.Status)

		finalExecution, err := s.DB.Repositories.TaskExecutions.GetLatestByTaskID(ctx, task.ID)
		require.NoError(s.T(), err)
		assert.Equal(s.T(), models.ExecutionStatusCompleted, finalExecution.Status)
		assert.Equal(s.T(), "Hello, World!\n", *finalExecution.Stdout)
	})
}

// TestTaskExecutionScenarios validates various execution scenarios
func (s *DatabaseIntegrationSuite) TestTaskExecutionScenarios() {
	ctx := context.Background()

	s.Run("failed execution scenario", func() {
		user := s.DB.CreateMinimalUser(s.T(), ctx, "failed-exec@test.com", "Failed User")
		task := s.DB.CreateMinimalTask(s.T(), ctx, user.ID, "Failed Task")

		execution := testutil.NewExecutionFactory(task.ID).
			Failed().
			WithReturnCode(1).
			WithStderr("Error: Something went wrong\n").
			WithExecutionTime(500).
			Build()

		err := s.DB.Repositories.TaskExecutions.Create(ctx, execution)
		require.NoError(s.T(), err)

		// Verify failed execution
		retrieved, err := s.DB.Repositories.TaskExecutions.GetByID(ctx, execution.ID)
		require.NoError(s.T(), err)
		assert.Equal(s.T(), models.ExecutionStatusFailed, retrieved.Status)
		assert.Equal(s.T(), 1, *retrieved.ReturnCode)
		assert.Contains(s.T(), *retrieved.Stderr, "Something went wrong")
	})

	s.Run("timeout execution scenario", func() {
		user := s.DB.CreateMinimalUser(s.T(), ctx, "timeout-exec@test.com", "Timeout User")
		task := s.DB.CreateMinimalTask(s.T(), ctx, user.ID, "Timeout Task")

		execution := testutil.NewExecutionFactory(task.ID).
			Timeout().
			WithExecutionTime(30000). // 30 seconds
			WithStderr("Process terminated due to timeout\n").
			Build()

		err := s.DB.Repositories.TaskExecutions.Create(ctx, execution)
		require.NoError(s.T(), err)

		retrieved, err := s.DB.Repositories.TaskExecutions.GetByID(ctx, execution.ID)
		require.NoError(s.T(), err)
		assert.Equal(s.T(), models.ExecutionStatusTimeout, retrieved.Status)
		assert.Equal(s.T(), 30000, *retrieved.ExecutionTimeMs)
	})
}

// TestConcurrentAccess validates database behavior under concurrent access
func (s *DatabaseIntegrationSuite) TestConcurrentAccess() {
	ctx := context.Background()

	s.Run("concurrent task creation", func() {
		user := s.DB.CreateMinimalUser(s.T(), ctx, "concurrent@test.com", "Concurrent User")

		// Create multiple tasks concurrently
		numTasks := 10
		done := make(chan error, numTasks)

		for i := 0; i < numTasks; i++ {
			go func(index int) {
				task := testutil.NewTaskFactory(user.ID).
					WithName(fmt.Sprintf("Concurrent Task %d", index)).
					Build()

				err := s.DB.Repositories.Tasks.Create(ctx, task)
				done <- err
			}(i)
		}

		// Wait for all tasks to complete
		for i := 0; i < numTasks; i++ {
			err := <-done
			assert.NoError(s.T(), err)
		}

		// Verify all tasks were created
		tasks, err := s.DB.Repositories.Tasks.GetByUserID(ctx, user.ID, 20, 0)
		require.NoError(s.T(), err)
		assert.Len(s.T(), tasks, numTasks)
	})
}

// TestTransactionBehavior validates transaction consistency
func (s *DatabaseIntegrationSuite) TestTransactionBehavior() {
	ctx := context.Background()

	s.Run("task and execution creation in transaction", func() {
		user := s.DB.CreateMinimalUser(s.T(), ctx, "transaction@test.com", "Transaction User")

		// This would normally be done in a service layer with transaction management
		// For now, we test that individual operations maintain consistency
		task := s.DB.CreateMinimalTask(s.T(), ctx, user.ID, "Transaction Task")

		execution := testutil.NewExecutionFactory(task.ID).
			Pending().
			Build()

		err := s.DB.Repositories.TaskExecutions.Create(ctx, execution)
		require.NoError(s.T(), err)

		// Verify both exist and are linked correctly
		retrievedTask, err := s.DB.Repositories.Tasks.GetByID(ctx, task.ID)
		require.NoError(s.T(), err)

		retrievedExecution, err := s.DB.Repositories.TaskExecutions.GetByID(ctx, execution.ID)
		require.NoError(s.T(), err)

		assert.Equal(s.T(), retrievedTask.ID, retrievedExecution.TaskID)
		assert.Equal(s.T(), user.ID, retrievedTask.UserID)
	})
}

// TestComplexQueries validates complex database queries and operations
func (s *DatabaseIntegrationSuite) TestComplexQueries() {
	ctx := context.Background()

	s.Run("metadata search functionality", func() {
		user := s.DB.CreateMinimalUser(s.T(), ctx, "search@test.com", "Search User")

		// Create tasks with different metadata
		task1 := testutil.NewTaskFactory(user.ID).
			WithName("Python Task").
			WithMetadataField("language", "python").
			WithMetadataField("environment", "test").
			Build()

		task2 := testutil.NewTaskFactory(user.ID).
			WithName("JavaScript Task").
			WithMetadataField("language", "javascript").
			WithMetadataField("environment", "test").
			Build()

		task3 := testutil.NewTaskFactory(user.ID).
			WithName("Production Task").
			WithMetadataField("language", "python").
			WithMetadataField("environment", "production").
			Build()

		// Create all tasks
		require.NoError(s.T(), s.DB.Repositories.Tasks.Create(ctx, task1))
		require.NoError(s.T(), s.DB.Repositories.Tasks.Create(ctx, task2))
		require.NoError(s.T(), s.DB.Repositories.Tasks.Create(ctx, task3))

		// Search for Python tasks
		pythonQuery := `{"language": "python"}`
		pythonTasks, err := s.DB.Repositories.Tasks.SearchByMetadata(ctx, pythonQuery, 10, 0)
		require.NoError(s.T(), err)
		assert.Len(s.T(), pythonTasks, 2)

		// Search for test environment tasks
		testQuery := `{"environment": "test"}`
		testTasks, err := s.DB.Repositories.Tasks.SearchByMetadata(ctx, testQuery, 10, 0)
		require.NoError(s.T(), err)
		assert.Len(s.T(), testTasks, 2)
	})

	s.Run("pagination and ordering", func() {
		user := s.DB.CreateMinimalUser(s.T(), ctx, "pagination@test.com", "Pagination User")

		// Create multiple tasks
		numTasks := 25
		for i := 0; i < numTasks; i++ {
			task := testutil.NewTaskFactory(user.ID).
				WithName("Task " + string(rune(48+i))). // '0', '1', '2', etc.
				Build()
			require.NoError(s.T(), s.DB.Repositories.Tasks.Create(ctx, task))
		}

		// Test pagination
		page1, err := s.DB.Repositories.Tasks.GetByUserID(ctx, user.ID, 10, 0)
		require.NoError(s.T(), err)
		assert.Len(s.T(), page1, 10)

		page2, err := s.DB.Repositories.Tasks.GetByUserID(ctx, user.ID, 10, 10)
		require.NoError(s.T(), err)
		assert.Len(s.T(), page2, 10)

		page3, err := s.DB.Repositories.Tasks.GetByUserID(ctx, user.ID, 10, 20)
		require.NoError(s.T(), err)
		assert.Len(s.T(), page3, 5) // Remaining 5

		// Verify no overlap between pages
		page1IDs := make(map[string]bool)
		for _, task := range page1 {
			page1IDs[task.ID.String()] = true
		}

		for _, task := range page2 {
			assert.False(s.T(), page1IDs[task.ID.String()], "Page 2 should not contain Page 1 tasks")
		}
	})
}

// TestSeededDataScenarios validates testing with pre-seeded data
func (s *DatabaseIntegrationSuite) TestSeededDataScenarios() {
	s.Run("working with fixtures", func() {
		s.DB.WithSeededDatabase(s.T(), s.Fixtures, func() {
			ctx := context.Background()

			// Verify admin user exists
			adminUser, err := s.DB.Repositories.Users.GetByEmail(ctx, s.Fixtures.Users.AdminUser.Email)
			require.NoError(s.T(), err)
			assert.Equal(s.T(), "Admin User", adminUser.Name)

			// Verify pending task exists
			pendingTask, err := s.DB.Repositories.Tasks.GetByID(ctx, s.Fixtures.Tasks.PendingTask.ID)
			require.NoError(s.T(), err)
			assert.Equal(s.T(), models.TaskStatusPending, pendingTask.Status)

			// Verify successful execution exists
			execution, err := s.DB.Repositories.TaskExecutions.GetByID(ctx, s.Fixtures.Executions.SuccessfulExecution.ID)
			require.NoError(s.T(), err)
			assert.Equal(s.T(), models.ExecutionStatusCompleted, execution.Status)
			assert.Equal(s.T(), 0, *execution.ReturnCode)
		})
	})
}

// TestDatabaseConstraints validates database constraints and error handling
func (s *DatabaseIntegrationSuite) TestDatabaseConstraints() {
	ctx := context.Background()

	s.Run("foreign key constraints", func() {
		// Try to create a task with non-existent user ID
		nonExistentUserID := testutil.NewUserFactory().Build().ID
		task := testutil.NewTaskFactory(nonExistentUserID).Build()

		err := s.DB.Repositories.Tasks.Create(ctx, task)
		assert.Error(s.T(), err)
		assert.Contains(s.T(), err.Error(), "does not exist") // Repository returns custom error message
	})

	s.Run("task execution foreign key constraint", func() {
		// Try to create execution with non-existent task ID
		nonExistentTaskID := testutil.NewTaskFactory(testutil.NewUserFactory().Build().ID).Build().ID
		execution := testutil.NewExecutionFactory(nonExistentTaskID).Build()

		err := s.DB.Repositories.TaskExecutions.Create(ctx, execution)
		assert.Error(s.T(), err)
		assert.Contains(s.T(), err.Error(), "does not exist")
	})
}

// TestPerformanceCharacteristics validates database performance expectations
func (s *DatabaseIntegrationSuite) TestPerformanceCharacteristics() {
	if testing.Short() {
		s.T().Skip("Skipping performance tests in short mode")
	}

	ctx := context.Background()

	s.Run("bulk operations performance", func() {
		user := s.DB.CreateMinimalUser(s.T(), ctx, "perf@test.com", "Performance User")

		// Measure time to create 100 tasks
		numTasks := 100
		start := time.Now()

		for i := 0; i < numTasks; i++ {
			task := testutil.NewTaskFactory(user.ID).
				WithName(fmt.Sprintf("Perf Task %d", i)).
				Build()
			require.NoError(s.T(), s.DB.Repositories.Tasks.Create(ctx, task))
		}

		duration := time.Since(start)
		s.T().Logf("Created %d tasks in %v (%.2fms per task)",
			numTasks, duration, float64(duration.Nanoseconds())/float64(numTasks)/1e6)

		// Performance expectation: should be able to create tasks at reasonable speed
		avgTimePerTask := duration / time.Duration(numTasks)
		assert.Less(s.T(), avgTimePerTask.Milliseconds(), int64(50),
			"Task creation should average less than 50ms per task")
	})
}

func int64Ptr(i int64) *int64 {
	return &i
}

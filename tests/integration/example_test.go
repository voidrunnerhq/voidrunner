//go:build integration

package integration_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/voidrunnerhq/voidrunner/internal/models"
	"github.com/voidrunnerhq/voidrunner/tests/testutil"
)

// ExampleIntegrationTestSuite demonstrates how to use the integration test suite
type ExampleIntegrationTestSuite struct {
	testutil.IntegrationTestSuite
}

// TestIntegrationSuite runs the integration test suite
func TestIntegrationSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	suite.Run(t, new(ExampleIntegrationTestSuite))
}

// TestUserWorkflow demonstrates a complete user workflow test
func (s *ExampleIntegrationTestSuite) TestUserWorkflow() {
	// Test user registration
	registerReq := s.Factory.RegisterRequest()

	resp := s.HTTP.POST(s.T(), "/api/v1/auth/register", registerReq).
		ExpectCreated().
		ExpectJSON()

	var authResp models.AuthResponse
	resp.UnmarshalResponse(&authResp)

	testutil.ValidateAuthTokens(s.T(), &authResp)
	assert.Equal(s.T(), registerReq.Email, authResp.User.Email)
	assert.Equal(s.T(), registerReq.Name, authResp.User.Name)

	// Test authenticated request
	auth := &testutil.AuthContext{
		User: &models.User{
			BaseModel: models.BaseModel{ID: authResp.User.ID},
			Email:     authResp.User.Email,
			Name:      authResp.User.Name,
		},
		AccessToken: authResp.AccessToken,
	}

	// Create a task
	createTaskReq := s.Factory.CreateTaskRequest()

	taskResp := s.HTTP.AuthenticatedPOST(s.T(), "/api/v1/tasks", createTaskReq, auth).
		ExpectCreated().
		ExpectJSON()

	var task models.TaskResponse
	taskResp.UnmarshalResponse(&task)

	assert.Equal(s.T(), createTaskReq.Name, task.Name)
	assert.Equal(s.T(), authResp.User.ID, task.UserID)

	// Get tasks
	s.HTTP.AuthenticatedGET(s.T(), "/api/v1/tasks", auth).
		ExpectOK().
		ExpectJSON()
}

// TestTaskCRUDOperations demonstrates CRUD operations on tasks
func (s *ExampleIntegrationTestSuite) TestTaskCRUDOperations() {
	// Create authenticated user for potential future use
	_ = s.CreateAuthenticatedUser("test@example.com", "Test User")

	// Create task using factory
	user := testutil.NewUserFactory().
		WithEmail("factory@example.com").
		WithName("Factory User").
		Build()

	err := s.DB.Repositories.Users.Create(context.Background(), user)
	require.NoError(s.T(), err)

	task := testutil.NewTaskFactory(user.ID).
		WithName("Factory Task").
		WithPythonScript("print('Factory test')").
		HighPriority().
		Build()

	err = s.DB.Repositories.Tasks.Create(context.Background(), task)
	require.NoError(s.T(), err)

	// Verify task was created
	retrievedTask, err := s.DB.Repositories.Tasks.GetByID(context.Background(), task.ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), task.Name, retrievedTask.Name)
	assert.Equal(s.T(), task.Priority, retrievedTask.Priority)
}

// TestWithSeededData demonstrates testing with pre-seeded data
func (s *ExampleIntegrationTestSuite) TestWithSeededData() {
	s.WithSeededData(func() {
		// Test with pre-seeded fixtures
		ctx := context.Background()

		// Verify users exist
		user, err := s.DB.Repositories.Users.GetByEmail(ctx, s.Fixtures.Users.AdminUser.Email)
		require.NoError(s.T(), err)
		assert.Equal(s.T(), s.Fixtures.Users.AdminUser.Name, user.Name)

		// Verify tasks exist
		task, err := s.DB.Repositories.Tasks.GetByID(ctx, s.Fixtures.Tasks.PendingTask.ID)
		require.NoError(s.T(), err)
		assert.Equal(s.T(), models.TaskStatusPending, task.Status)

		// Verify executions exist
		execution, err := s.DB.Repositories.TaskExecutions.GetByID(ctx, s.Fixtures.Executions.SuccessfulExecution.ID)
		require.NoError(s.T(), err)
		assert.Equal(s.T(), models.ExecutionStatusCompleted, execution.Status)
	})
}

// TestDatabaseHelper demonstrates standalone database testing
func TestDatabaseHelper(t *testing.T) {
	testutil.WithTestDatabase(t, func(db *testutil.DatabaseHelper) {
		ctx := context.Background()

		// Create user using factory
		user := testutil.NewUserFactory().
			WithEmail("db-test@example.com").
			WithName("DB Test User").
			Build()

		err := db.Repositories.Users.Create(ctx, user)
		require.NoError(t, err)

		// Verify user was created
		retrievedUser, err := db.Repositories.Users.GetByEmail(ctx, user.Email)
		require.NoError(t, err)
		assert.Equal(t, user.Name, retrievedUser.Name)
	})
}

// TestFactories demonstrates using test data factories
func TestFactories(t *testing.T) {
	// Test user factory
	user := testutil.NewUserFactory().
		Admin().
		WithEmail("factory@test.com").
		WithName("Factory User").
		Build()

	assert.Equal(t, "factory@test.com", user.Email)
	assert.Equal(t, "Factory User", user.Name)
	assert.NotEmpty(t, user.ID)
	assert.NotEmpty(t, user.PasswordHash)

	// Test task factory
	task := testutil.NewTaskFactory(user.ID).
		WithName("Test Task").
		WithPythonScript("print('test')").
		HighPriority().
		Completed().
		WithTimeout(60).
		WithMetadataField("environment", "test").
		Build()

	assert.Equal(t, "Test Task", task.Name)
	assert.Equal(t, models.ScriptTypePython, task.ScriptType)
	assert.Equal(t, models.TaskStatusCompleted, task.Status)
	assert.Equal(t, 10, task.Priority) // High priority
	assert.Equal(t, 60, task.TimeoutSeconds)
	assert.Equal(t, "test", task.Metadata["environment"])

	// Test execution factory
	execution := testutil.NewExecutionFactory(task.ID).
		Successful().
		WithExecutionTime(2000).
		WithMemoryUsage(2048576). // 2MB
		Build()

	assert.Equal(t, task.ID, execution.TaskID)
	assert.Equal(t, models.ExecutionStatusCompleted, execution.Status)
	assert.Equal(t, 0, *execution.ReturnCode)
	assert.Equal(t, 2000, *execution.ExecutionTimeMs)
	assert.Equal(t, int64(2048576), *execution.MemoryUsageBytes)
}

// TestFixtures demonstrates using pre-defined fixtures
func TestFixtures(t *testing.T) {
	fixtures := testutil.NewAllFixtures()

	// Test user fixtures
	assert.Equal(t, "admin@voidrunner.dev", fixtures.Users.AdminUser.Email)
	assert.Equal(t, "user@voidrunner.dev", fixtures.Users.RegularUser.Email)
	assert.Equal(t, "inactive@voidrunner.dev", fixtures.Users.InactiveUser.Email)

	// Test task fixtures
	assert.Equal(t, models.TaskStatusPending, fixtures.Tasks.PendingTask.Status)
	assert.Equal(t, models.TaskStatusRunning, fixtures.Tasks.RunningTask.Status)
	assert.Equal(t, models.TaskStatusCompleted, fixtures.Tasks.CompletedTask.Status)
	assert.Equal(t, models.TaskStatusFailed, fixtures.Tasks.FailedTask.Status)

	// Test execution fixtures
	assert.Equal(t, models.ExecutionStatusCompleted, fixtures.Executions.SuccessfulExecution.Status)
	assert.Equal(t, models.ExecutionStatusFailed, fixtures.Executions.FailedExecution.Status)
	assert.Equal(t, models.ExecutionStatusTimeout, fixtures.Executions.TimeoutExecution.Status)
}

// BenchmarkTaskCreation demonstrates benchmark testing
func BenchmarkTaskCreation(b *testing.B) {
	testutil.RunBenchmark(b, func(helper *testutil.BenchmarkHelper) {
		ctx := context.Background()

		// Create user once
		user := testutil.NewUserFactory().Build()
		err := helper.DB.Repositories.Users.Create(ctx, user)
		require.NoError(b, err)

		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			task := testutil.NewTaskFactory(user.ID).Build()
			err := helper.DB.Repositories.Tasks.Create(ctx, task)
			require.NoError(b, err)
		}
	})
}

// TestUsagePatterns demonstrates typical usage patterns
func TestUsagePatterns(t *testing.T) {
	// For integration tests with HTTP and database
	testutil.RunIntegrationTests(t, func(suite *testutil.IntegrationTestSuite) {
		// Test with seeded data
		suite.WithSeededData(func() {
			// Your test code here
		})
	})

	// For database-only tests
	testutil.WithTestDatabase(t, func(db *testutil.DatabaseHelper) {
		// Your database test code here
	})

	// For tests with specific fixtures
	fixtures := testutil.NewAllFixtures()
	testutil.WithSeededTestDatabase(t, fixtures, func(db *testutil.DatabaseHelper) {
		// Your test code with seeded data here
	})

	// For unit tests with test data
	helper := testutil.NewUnitTestHelper()
	user := helper.Fixtures.Users.RegularUser
	task := testutil.NewTaskFactory(user.ID).HighPriority().Build()
	_ = task // Use in your test
}

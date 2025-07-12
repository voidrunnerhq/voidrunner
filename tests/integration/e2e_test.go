//go:build integration

package integration_test

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/voidrunnerhq/voidrunner/internal/api/routes"
	"github.com/voidrunnerhq/voidrunner/internal/auth"
	"github.com/voidrunnerhq/voidrunner/internal/config"
	"github.com/voidrunnerhq/voidrunner/internal/executor"
	"github.com/voidrunnerhq/voidrunner/internal/models"
	"github.com/voidrunnerhq/voidrunner/internal/services"
	"github.com/voidrunnerhq/voidrunner/pkg/logger"
	"github.com/voidrunnerhq/voidrunner/tests/testutil"
)

// E2EIntegrationSuite provides comprehensive end-to-end testing with complete user workflows
type E2EIntegrationSuite struct {
	suite.Suite
	DB          *testutil.DatabaseHelper
	HTTP        *testutil.HTTPHelper
	Auth        *testutil.AuthHelper
	Factory     *testutil.RequestFactory
	Config      *config.Config
	AuthService *auth.Service
}

// SetupSuite initializes the E2E test suite
func (s *E2EIntegrationSuite) SetupSuite() {
	gin.SetMode(gin.TestMode)

	// Initialize test configuration
	s.Config = testutil.GetTestConfig()

	// Initialize logger
	log := logger.New("test", "debug")

	// Initialize database helper
	s.DB = testutil.NewDatabaseHelper(s.T())

	// Initialize JWT service and auth service
	jwtSvc := auth.NewJWTService(&s.Config.JWT)
	s.AuthService = auth.NewService(s.DB.Repositories.Users, jwtSvc, log.Logger, s.Config)

	// Setup router with full middleware stack
	router := gin.New()
	taskExecutionService := services.NewTaskExecutionService(s.DB.DB, log.Logger)

	// Create mock executor for e2e tests
	executorConfig := executor.NewDefaultConfig()
	mockExecutor := executor.NewMockExecutor(executorConfig, log.Logger)
	taskExecutorService := services.NewTaskExecutorService(
		taskExecutionService,
		s.DB.Repositories.Tasks,
		mockExecutor,
		nil, // cleanup manager not needed for mock executor
		log.Logger,
	)

	routes.Setup(router, s.Config, log, s.DB.DB, s.DB.Repositories, s.AuthService, taskExecutionService, taskExecutorService)

	// Initialize helpers
	s.HTTP = testutil.NewHTTPHelper(router, s.AuthService)
	s.Auth = testutil.NewAuthHelper(s.AuthService)
	s.Factory = testutil.NewRequestFactory()
}

// TearDownSuite cleans up the E2E test suite
func (s *E2EIntegrationSuite) TearDownSuite() {
	if s.DB != nil {
		s.DB.Close()
	}
}

// SetupTest runs before each test
func (s *E2EIntegrationSuite) SetupTest() {
	s.DB.CleanupDatabase(s.T())
}

// TestE2EIntegrationSuite runs the enhanced E2E integration test suite
func TestE2EIntegrationSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E integration tests in short mode")
	}

	suite.Run(t, new(E2EIntegrationSuite))
}

// TestCompleteUserJourney tests the complete user journey from registration to task completion
func (s *E2EIntegrationSuite) TestCompleteUserJourney() {
	s.Run("complete workflow: registration to task completion", func() {
		ctx := context.Background()

		// Step 1: User Registration
		registerReq := s.Factory.ValidRegisterRequestWithEmail("journey@example.com")
		authCtx := s.HTTP.RegisterUser(s.T(), registerReq.Email, registerReq.Password, registerReq.Name)

		assert.NotEmpty(s.T(), authCtx.AccessToken)
		assert.Equal(s.T(), registerReq.Email, authCtx.User.Email)
		assert.Equal(s.T(), registerReq.Name, authCtx.User.Name)

		// Verify user exists in database
		dbUser, err := s.DB.Repositories.Users.GetByEmail(ctx, registerReq.Email)
		require.NoError(s.T(), err)
		assert.Equal(s.T(), authCtx.User.ID, dbUser.ID)

		// Step 2: Create Multiple Tasks
		tasks := make([]models.TaskResponse, 3)
		taskNames := []string{"Journey Task 1", "Journey Task 2", "Journey Task 3"}

		for i, name := range taskNames {
			createReq := s.Factory.ValidCreateTaskRequestWithName(name)
			resp := s.HTTP.AuthenticatedPOST(s.T(), "/api/v1/tasks", createReq, authCtx).ExpectCreated()

			var task models.TaskResponse
			resp.UnmarshalResponse(&task)
			tasks[i] = task

			assert.Equal(s.T(), name, task.Name)
			assert.Equal(s.T(), models.TaskStatusPending, task.Status)
			assert.Equal(s.T(), authCtx.User.ID, task.UserID)
		}

		// Step 3: List Tasks (verify all tasks are visible)
		resp := s.HTTP.AuthenticatedGET(s.T(), "/api/v1/tasks?limit=10&offset=0", authCtx).ExpectOK()

		var taskList models.TaskListResponse
		resp.UnmarshalResponse(&taskList)

		assert.GreaterOrEqual(s.T(), len(taskList.Tasks), 3)
		assert.GreaterOrEqual(s.T(), taskList.Total, int64(3))

		// Step 4: Execute First Task
		firstTask := tasks[0]
		execResp := s.HTTP.AuthenticatedPOST(s.T(), fmt.Sprintf("/api/v1/tasks/%s/executions", firstTask.ID), nil, authCtx).ExpectCreated()

		var execution models.TaskExecutionResponse
		execResp.UnmarshalResponse(&execution)

		assert.Equal(s.T(), firstTask.ID, execution.TaskID)
		assert.Equal(s.T(), models.ExecutionStatusPending, execution.Status)

		// Step 5: Complete the Execution
		updateReq := s.Factory.ValidUpdateTaskExecutionRequest()
		updateResp := s.HTTP.AuthenticatedPUT(s.T(), fmt.Sprintf("/api/v1/executions/%s", execution.ID), updateReq, authCtx).ExpectOK()

		var completedExecution models.TaskExecutionResponse
		updateResp.UnmarshalResponse(&completedExecution)

		assert.Equal(s.T(), models.ExecutionStatusCompleted, completedExecution.Status)
		assert.Equal(s.T(), *updateReq.ReturnCode, *completedExecution.ReturnCode)
		assert.Equal(s.T(), *updateReq.Stdout, *completedExecution.Stdout)

		// Step 6: Verify Task Status Updated
		taskResp := s.HTTP.AuthenticatedGET(s.T(), fmt.Sprintf("/api/v1/tasks/%s", firstTask.ID), authCtx).ExpectOK()

		var updatedTask models.TaskResponse
		taskResp.UnmarshalResponse(&updatedTask)

		assert.Equal(s.T(), models.TaskStatusCompleted, updatedTask.Status)

		// Step 7: List Task Executions
		execListResp := s.HTTP.AuthenticatedGET(s.T(), fmt.Sprintf("/api/v1/tasks/%s/executions", firstTask.ID), authCtx).ExpectOK()

		var execList models.ExecutionListResponse
		execListResp.UnmarshalResponse(&execList)

		assert.GreaterOrEqual(s.T(), len(execList.Executions), 1)
		assert.Equal(s.T(), execution.ID, execList.Executions[0].ID)

		// Step 8: Update Task Metadata
		updateTaskReq := s.Factory.ValidUpdateTaskRequestPartial()
		updateTaskResp := s.HTTP.AuthenticatedPUT(s.T(), fmt.Sprintf("/api/v1/tasks/%s", firstTask.ID), updateTaskReq, authCtx).ExpectOK()

		var finalTask models.TaskResponse
		updateTaskResp.UnmarshalResponse(&finalTask)

		assert.Equal(s.T(), *updateTaskReq.Name, finalTask.Name)

		// Step 9: Clean up (delete completed task)
		s.HTTP.AuthenticatedDELETE(s.T(), fmt.Sprintf("/api/v1/tasks/%s", firstTask.ID), authCtx).ExpectOK()

		// Step 10: Verify task deleted
		s.HTTP.AuthenticatedGET(s.T(), fmt.Sprintf("/api/v1/tasks/%s", firstTask.ID), authCtx).ExpectNotFound()
	})
}

// TestMultiUserWorkflow tests workflows with multiple users and access control
func (s *E2EIntegrationSuite) TestMultiUserWorkflow() {
	s.Run("multi-user access control and isolation", func() {
		// Create two users
		user1Auth := s.HTTP.RegisterUser(s.T(), "user1@example.com", "Password123!", "User One")
		user2Auth := s.HTTP.RegisterUser(s.T(), "user2@example.com", "Password123!", "User Two")

		// User 1 creates tasks
		user1Task1 := s.Factory.ValidCreateTaskRequestWithName("User 1 Task 1")
		resp1 := s.HTTP.AuthenticatedPOST(s.T(), "/api/v1/tasks", user1Task1, user1Auth).ExpectCreated()

		var task1 models.TaskResponse
		resp1.UnmarshalResponse(&task1)

		// User 2 creates tasks
		user2Task1 := s.Factory.ValidCreateTaskRequestWithName("User 2 Task 1")
		resp2 := s.HTTP.AuthenticatedPOST(s.T(), "/api/v1/tasks", user2Task1, user2Auth).ExpectCreated()

		var task2 models.TaskResponse
		resp2.UnmarshalResponse(&task2)

		// User 1 can access their own task
		s.HTTP.AuthenticatedGET(s.T(), fmt.Sprintf("/api/v1/tasks/%s", task1.ID), user1Auth).ExpectOK()

		// User 1 cannot access User 2's task
		s.HTTP.AuthenticatedGET(s.T(), fmt.Sprintf("/api/v1/tasks/%s", task2.ID), user1Auth).ExpectForbidden()

		// User 2 can access their own task
		s.HTTP.AuthenticatedGET(s.T(), fmt.Sprintf("/api/v1/tasks/%s", task2.ID), user2Auth).ExpectOK()

		// User 2 cannot access User 1's task
		s.HTTP.AuthenticatedGET(s.T(), fmt.Sprintf("/api/v1/tasks/%s", task1.ID), user2Auth).ExpectForbidden()

		// Each user only sees their own tasks in listings
		user1List := s.HTTP.AuthenticatedGET(s.T(), "/api/v1/tasks", user1Auth).ExpectOK()
		var user1TaskList models.TaskListResponse
		user1List.UnmarshalResponse(&user1TaskList)

		for _, task := range user1TaskList.Tasks {
			assert.Equal(s.T(), user1Auth.User.ID, task.UserID)
		}

		user2List := s.HTTP.AuthenticatedGET(s.T(), "/api/v1/tasks", user2Auth).ExpectOK()
		var user2TaskList models.TaskListResponse
		user2List.UnmarshalResponse(&user2TaskList)

		for _, task := range user2TaskList.Tasks {
			assert.Equal(s.T(), user2Auth.User.ID, task.UserID)
		}

		// Clean up
		s.HTTP.AuthenticatedDELETE(s.T(), fmt.Sprintf("/api/v1/tasks/%s", task1.ID), user1Auth).ExpectOK()
		s.HTTP.AuthenticatedDELETE(s.T(), fmt.Sprintf("/api/v1/tasks/%s", task2.ID), user2Auth).ExpectOK()
	})
}

// TestConcurrentTaskExecution tests concurrent task execution scenarios
func (s *E2EIntegrationSuite) TestConcurrentTaskExecution() {
	s.Run("concurrent task execution by multiple users", func() {
		// Create test user
		userAuth := s.HTTP.RegisterUser(s.T(), "concurrent@example.com", "Password123!", "Concurrent User")

		// Create multiple tasks concurrently
		numTasks := 5
		taskChan := make(chan models.TaskResponse, numTasks)
		var wg sync.WaitGroup

		for i := 0; i < numTasks; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()

				createReq := s.Factory.ValidCreateTaskRequestWithName(fmt.Sprintf("Concurrent Task %d", index))
				resp := s.HTTP.AuthenticatedPOST(s.T(), "/api/v1/tasks", createReq, userAuth).ExpectCreated()

				var task models.TaskResponse
				resp.UnmarshalResponse(&task)
				taskChan <- task
			}(i)
		}

		wg.Wait()
		close(taskChan)

		// Collect all created tasks
		var tasks []models.TaskResponse
		for task := range taskChan {
			tasks = append(tasks, task)
		}

		assert.Len(s.T(), tasks, numTasks)

		// Execute all tasks concurrently
		execChan := make(chan models.TaskExecutionResponse, numTasks)

		for _, task := range tasks {
			wg.Add(1)
			go func(t models.TaskResponse) {
				defer wg.Done()

				execResp := s.HTTP.AuthenticatedPOST(s.T(), fmt.Sprintf("/api/v1/tasks/%s/executions", t.ID), nil, userAuth).ExpectCreated()

				var execution models.TaskExecutionResponse
				execResp.UnmarshalResponse(&execution)
				execChan <- execution
			}(task)
		}

		wg.Wait()
		close(execChan)

		// Complete all executions concurrently
		for execution := range execChan {
			wg.Add(1)
			go func(exec models.TaskExecutionResponse) {
				defer wg.Done()

				updateReq := s.Factory.ValidUpdateTaskExecutionRequest()
				s.HTTP.AuthenticatedPUT(s.T(), fmt.Sprintf("/api/v1/executions/%s", exec.ID), updateReq, userAuth).ExpectOK()
			}(execution)
		}

		wg.Wait()

		// Verify all tasks completed successfully
		for _, task := range tasks {
			taskResp := s.HTTP.AuthenticatedGET(s.T(), fmt.Sprintf("/api/v1/tasks/%s", task.ID), userAuth).ExpectOK()

			var finalTask models.TaskResponse
			taskResp.UnmarshalResponse(&finalTask)

			assert.Equal(s.T(), models.TaskStatusCompleted, finalTask.Status)
		}
	})
}

// TestComplexWorkflowWithErrors tests workflow with error scenarios and recovery
func (s *E2EIntegrationSuite) TestComplexWorkflowWithErrors() {
	s.Run("error scenarios and recovery workflow", func() {
		userAuth := s.HTTP.RegisterUser(s.T(), "errors@example.com", "Password123!", "Error Test User")

		// Create a task
		createReq := s.Factory.ValidCreateTaskRequest()
		resp := s.HTTP.AuthenticatedPOST(s.T(), "/api/v1/tasks", createReq, userAuth).ExpectCreated()

		var task models.TaskResponse
		resp.UnmarshalResponse(&task)

		// Start execution
		execResp := s.HTTP.AuthenticatedPOST(s.T(), fmt.Sprintf("/api/v1/tasks/%s/executions", task.ID), nil, userAuth).ExpectCreated()

		var execution models.TaskExecutionResponse
		execResp.UnmarshalResponse(&execution)

		// Try to start another execution while one is running (should fail)
		s.HTTP.AuthenticatedPOST(s.T(), fmt.Sprintf("/api/v1/tasks/%s/executions", task.ID), nil, userAuth).ExpectStatus(409) // Conflict

		// Update execution to failed state
		failedUpdateReq := s.Factory.ValidUpdateTaskExecutionRequestFailed()
		failResp := s.HTTP.AuthenticatedPUT(s.T(), fmt.Sprintf("/api/v1/executions/%s", execution.ID), failedUpdateReq, userAuth).ExpectOK()

		var failedExecution models.TaskExecutionResponse
		failResp.UnmarshalResponse(&failedExecution)

		assert.Equal(s.T(), models.ExecutionStatusFailed, failedExecution.Status)
		assert.Equal(s.T(), *failedUpdateReq.ReturnCode, *failedExecution.ReturnCode)

		// Verify task status updated to failed
		taskResp := s.HTTP.AuthenticatedGET(s.T(), fmt.Sprintf("/api/v1/tasks/%s", task.ID), userAuth).ExpectOK()

		var failedTask models.TaskResponse
		taskResp.UnmarshalResponse(&failedTask)

		assert.Equal(s.T(), models.TaskStatusFailed, failedTask.Status)

		// Try to cancel a completed/failed execution (should fail)
		s.HTTP.AuthenticatedDELETE(s.T(), fmt.Sprintf("/api/v1/executions/%s", execution.ID), userAuth).ExpectStatus(409) // Conflict

		// Try to start a new execution on a failed task (should fail - failed tasks cannot be re-executed)
		s.HTTP.AuthenticatedPOST(s.T(), fmt.Sprintf("/api/v1/tasks/%s/executions", task.ID), nil, userAuth).ExpectStatus(409) // Conflict

		// Verify final task status remains failed (expected behavior)
		finalTaskResp := s.HTTP.AuthenticatedGET(s.T(), fmt.Sprintf("/api/v1/tasks/%s", task.ID), userAuth).ExpectOK()

		var finalTask models.TaskResponse
		finalTaskResp.UnmarshalResponse(&finalTask)

		assert.Equal(s.T(), models.TaskStatusFailed, finalTask.Status)

		// Verify execution history shows only the original failed execution
		execListResp := s.HTTP.AuthenticatedGET(s.T(), fmt.Sprintf("/api/v1/tasks/%s/executions", task.ID), userAuth).ExpectOK()

		var execList models.ExecutionListResponse
		execListResp.UnmarshalResponse(&execList)

		assert.GreaterOrEqual(s.T(), len(execList.Executions), 1)
		assert.GreaterOrEqual(s.T(), execList.Total, int64(1))
	})
}

// TestAuthenticationFlowsWithRealJWT tests complete authentication flows with real JWT tokens
func (s *E2EIntegrationSuite) TestAuthenticationFlowsWithRealJWT() {
	s.Run("complete authentication flow with real JWT validation", func() {
		// Register user
		registerReq := s.Factory.ValidRegisterRequestWithEmail("jwt@example.com")
		registerResp := s.HTTP.POST(s.T(), "/api/v1/auth/register", registerReq).ExpectCreated()

		var authResponse models.AuthResponse
		registerResp.UnmarshalResponse(&authResponse)

		// Validate token response structure
		testutil.ValidateAuthTokens(s.T(), &authResponse)

		// Use access token to access protected endpoint
		headers := s.Factory.GetAuthHeaders(authResponse.AccessToken)
		meReq := testutil.NewRequest("GET", "/api/v1/auth/me")
		for key, value := range headers {
			meReq.WithHeader(key, value)
		}

		meResp := s.HTTP.Do(s.T(), meReq).ExpectOK()

		var meResponseWrapper map[string]models.UserResponse
		meResp.UnmarshalResponse(&meResponseWrapper)

		userResponse := meResponseWrapper["user"]
		assert.Equal(s.T(), authResponse.User.ID, userResponse.ID)
		assert.Equal(s.T(), authResponse.User.Email, userResponse.Email)

		// Test token refresh
		refreshReq := s.Factory.ValidRefreshTokenRequest(authResponse.RefreshToken)
		refreshResp := s.HTTP.POST(s.T(), "/api/v1/auth/refresh", refreshReq).ExpectOK()

		var refreshResponse models.AuthResponse
		refreshResp.UnmarshalResponse(&refreshResponse)

		// New access token should be valid (may be same if refreshed quickly in tests)
		assert.NotEmpty(s.T(), refreshResponse.AccessToken)
		assert.Equal(s.T(), "Bearer", refreshResponse.TokenType)

		// Use new token to access protected endpoint
		newHeaders := s.Factory.GetAuthHeaders(refreshResponse.AccessToken)
		newMeReq := testutil.NewRequest("GET", "/api/v1/auth/me")
		for key, value := range newHeaders {
			newMeReq.WithHeader(key, value)
		}

		s.HTTP.Do(s.T(), newMeReq).ExpectOK()

		// Old token should still work (until expiry)
		oldMeReq := testutil.NewRequest("GET", "/api/v1/auth/me")
		for key, value := range headers {
			oldMeReq.WithHeader(key, value)
		}

		s.HTTP.Do(s.T(), oldMeReq).ExpectOK()

		// Test logout
		logoutReq := testutil.NewRequest("POST", "/api/v1/auth/logout")
		for key, value := range newHeaders {
			logoutReq.WithHeader(key, value)
		}

		s.HTTP.Do(s.T(), logoutReq).ExpectOK()

		// Login again
		loginReq := s.Factory.ValidLoginRequestWithCredentials(registerReq.Email, registerReq.Password)
		loginResp := s.HTTP.POST(s.T(), "/api/v1/auth/login", loginReq).ExpectOK()

		var loginResponse models.AuthResponse
		loginResp.UnmarshalResponse(&loginResponse)

		testutil.ValidateAuthTokens(s.T(), &loginResponse)
	})
}

// TestDataConsistencyAcrossOperations tests data consistency across multiple operations
func (s *E2EIntegrationSuite) TestDataConsistencyAcrossOperations() {
	s.Run("data consistency across complex operations", func() {
		ctx := context.Background()
		userAuth := s.HTTP.RegisterUser(s.T(), "consistency@example.com", "Password123!", "Consistency User")

		// Create task with metadata
		metadata := map[string]interface{}{
			"environment": "integration",
			"version":     "1.0",
			"priority":    "high",
		}

		createReq := s.Factory.ValidCreateTaskRequestWithMetadata(metadata)
		resp := s.HTTP.AuthenticatedPOST(s.T(), "/api/v1/tasks", createReq, userAuth).ExpectCreated()

		var task models.TaskResponse
		resp.UnmarshalResponse(&task)

		// Verify in database
		dbTask, err := s.DB.Repositories.Tasks.GetByID(ctx, task.ID)
		require.NoError(s.T(), err)

		assert.Equal(s.T(), task.Name, dbTask.Name)
		assert.Equal(s.T(), task.ScriptContent, dbTask.ScriptContent)
		assert.Equal(s.T(), task.ScriptType, dbTask.ScriptType)
		assert.Equal(s.T(), metadata["environment"], dbTask.Metadata["environment"])

		// Start execution
		execResp := s.HTTP.AuthenticatedPOST(s.T(), fmt.Sprintf("/api/v1/tasks/%s/executions", task.ID), nil, userAuth).ExpectCreated()

		var execution models.TaskExecutionResponse
		execResp.UnmarshalResponse(&execution)

		// Verify task status changed in database
		dbTaskAfterExec, err := s.DB.Repositories.Tasks.GetByID(ctx, task.ID)
		require.NoError(s.T(), err)
		assert.Equal(s.T(), models.TaskStatusRunning, dbTaskAfterExec.Status)

		// Verify execution exists in database
		dbExecution, err := s.DB.Repositories.TaskExecutions.GetByID(ctx, execution.ID)
		require.NoError(s.T(), err)

		assert.Equal(s.T(), execution.TaskID, dbExecution.TaskID)
		assert.Equal(s.T(), execution.Status, dbExecution.Status)

		// Complete execution
		updateReq := s.Factory.ValidUpdateTaskExecutionRequest()
		s.HTTP.AuthenticatedPUT(s.T(), fmt.Sprintf("/api/v1/executions/%s", execution.ID), updateReq, userAuth).ExpectOK()

		// Verify both task and execution updated in database
		finalDbTask, err := s.DB.Repositories.Tasks.GetByID(ctx, task.ID)
		require.NoError(s.T(), err)
		assert.Equal(s.T(), models.TaskStatusCompleted, finalDbTask.Status)

		finalDbExecution, err := s.DB.Repositories.TaskExecutions.GetByID(ctx, execution.ID)
		require.NoError(s.T(), err)
		assert.Equal(s.T(), models.ExecutionStatusCompleted, finalDbExecution.Status)
		assert.Equal(s.T(), *updateReq.ReturnCode, *finalDbExecution.ReturnCode)
		assert.Equal(s.T(), *updateReq.Stdout, *finalDbExecution.Stdout)

		// Verify API response matches database state
		apiTaskResp := s.HTTP.AuthenticatedGET(s.T(), fmt.Sprintf("/api/v1/tasks/%s", task.ID), userAuth).ExpectOK()

		var apiTask models.TaskResponse
		apiTaskResp.UnmarshalResponse(&apiTask)

		assert.Equal(s.T(), finalDbTask.Status, apiTask.Status)
		assert.Equal(s.T(), finalDbTask.Name, apiTask.Name)
	})
}

// TestLargeDataHandling tests handling of large data scenarios
func (s *E2EIntegrationSuite) TestLargeDataHandling() {
	s.Run("large data handling and pagination", func() {
		userAuth := s.HTTP.RegisterUser(s.T(), "largedata@example.com", "Password123!", "Large Data User")

		// Create many tasks
		numTasks := 25
		for i := 0; i < numTasks; i++ {
			createReq := s.Factory.ValidCreateTaskRequestWithName(fmt.Sprintf("Large Data Task %d", i+1))
			s.HTTP.AuthenticatedPOST(s.T(), "/api/v1/tasks", createReq, userAuth).ExpectCreated()
		}

		// Test pagination
		page1 := s.HTTP.AuthenticatedGET(s.T(), "/api/v1/tasks?limit=10&offset=0", userAuth).ExpectOK()

		var page1List models.TaskListResponse
		page1.UnmarshalResponse(&page1List)

		assert.Len(s.T(), page1List.Tasks, 10)
		assert.Equal(s.T(), int64(numTasks), page1List.Total)

		page2 := s.HTTP.AuthenticatedGET(s.T(), "/api/v1/tasks?limit=10&offset=10", userAuth).ExpectOK()

		var page2List models.TaskListResponse
		page2.UnmarshalResponse(&page2List)

		assert.Len(s.T(), page2List.Tasks, 10)

		page3 := s.HTTP.AuthenticatedGET(s.T(), "/api/v1/tasks?limit=10&offset=20", userAuth).ExpectOK()

		var page3List models.TaskListResponse
		page3.UnmarshalResponse(&page3List)

		assert.Len(s.T(), page3List.Tasks, 5) // Remaining tasks

		// Verify no duplicate tasks across pages
		allTaskIDs := make(map[uuid.UUID]bool)

		for _, task := range page1List.Tasks {
			assert.False(s.T(), allTaskIDs[task.ID], "Task ID should not be duplicated")
			allTaskIDs[task.ID] = true
		}

		for _, task := range page2List.Tasks {
			assert.False(s.T(), allTaskIDs[task.ID], "Task ID should not be duplicated")
			allTaskIDs[task.ID] = true
		}

		for _, task := range page3List.Tasks {
			assert.False(s.T(), allTaskIDs[task.ID], "Task ID should not be duplicated")
			allTaskIDs[task.ID] = true
		}

		assert.Len(s.T(), allTaskIDs, numTasks)

		// Test large script handling
		largeReq := s.Factory.CreateLargeTaskRequest()
		s.HTTP.AuthenticatedPOST(s.T(), "/api/v1/tasks", largeReq, userAuth).ExpectCreated()
	})
}

// TestTimeoutAndCancellationScenarios tests timeout and cancellation workflows
func (s *E2EIntegrationSuite) TestTimeoutAndCancellationScenarios() {
	s.Run("timeout and cancellation scenarios", func() {
		userAuth := s.HTTP.RegisterUser(s.T(), "timeout@example.com", "Password123!", "Timeout User")

		// Create task with short timeout
		createReq := s.Factory.ValidCreateTaskRequestWithTimeout(1) // 1 second timeout
		resp := s.HTTP.AuthenticatedPOST(s.T(), "/api/v1/tasks", createReq, userAuth).ExpectCreated()

		var task models.TaskResponse
		resp.UnmarshalResponse(&task)

		// Start execution
		execResp := s.HTTP.AuthenticatedPOST(s.T(), fmt.Sprintf("/api/v1/tasks/%s/executions", task.ID), nil, userAuth).ExpectCreated()

		var execution models.TaskExecutionResponse
		execResp.UnmarshalResponse(&execution)

		// Cancel the execution
		s.HTTP.AuthenticatedDELETE(s.T(), fmt.Sprintf("/api/v1/executions/%s", execution.ID), userAuth).ExpectOK()

		// Verify execution was cancelled
		execResp2 := s.HTTP.AuthenticatedGET(s.T(), fmt.Sprintf("/api/v1/executions/%s", execution.ID), userAuth).ExpectOK()

		var cancelledExecution models.TaskExecutionResponse
		execResp2.UnmarshalResponse(&cancelledExecution)

		assert.Equal(s.T(), models.ExecutionStatusCancelled, cancelledExecution.Status)

		// Create another execution for timeout scenario
		execResp3 := s.HTTP.AuthenticatedPOST(s.T(), fmt.Sprintf("/api/v1/tasks/%s/executions", task.ID), nil, userAuth).ExpectCreated()

		var timeoutExecution models.TaskExecutionResponse
		execResp3.UnmarshalResponse(&timeoutExecution)

		// Simulate timeout
		timeoutReq := s.Factory.ValidUpdateTaskExecutionRequestTimeout()
		s.HTTP.AuthenticatedPUT(s.T(), fmt.Sprintf("/api/v1/executions/%s", timeoutExecution.ID), timeoutReq, userAuth).ExpectOK()

		// Verify task status and execution history
		finalTaskResp := s.HTTP.AuthenticatedGET(s.T(), fmt.Sprintf("/api/v1/tasks/%s", task.ID), userAuth).ExpectOK()

		var finalTask models.TaskResponse
		finalTaskResp.UnmarshalResponse(&finalTask)

		assert.Equal(s.T(), models.TaskStatusTimeout, finalTask.Status)

		// Verify execution history
		execListResp := s.HTTP.AuthenticatedGET(s.T(), fmt.Sprintf("/api/v1/tasks/%s/executions", task.ID), userAuth).ExpectOK()

		var execList models.ExecutionListResponse
		execListResp.UnmarshalResponse(&execList)

		assert.GreaterOrEqual(s.T(), len(execList.Executions), 2)
	})
}

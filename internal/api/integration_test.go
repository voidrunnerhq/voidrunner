package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/voidrunnerhq/voidrunner/internal/api/routes"
	"github.com/voidrunnerhq/voidrunner/internal/auth"
	"github.com/voidrunnerhq/voidrunner/internal/config"
	"github.com/voidrunnerhq/voidrunner/internal/database"
	"github.com/voidrunnerhq/voidrunner/internal/models"
	"github.com/voidrunnerhq/voidrunner/pkg/logger"
)

// IntegrationTestSuite contains all integration tests
type IntegrationTestSuite struct {
	suite.Suite
	router      *gin.Engine
	repos       *database.Repositories
	authService *auth.Service
	testUser    *models.User
	accessToken string
	db          *database.Connection
}

// SetupSuite initializes the test suite
func (s *IntegrationTestSuite) SetupSuite() {
	gin.SetMode(gin.TestMode)

	// Initialize test configuration
	cfg := &config.Config{
		Database: config.DatabaseConfig{
			Host:     "localhost",
			Port:     5432,
			Name:     "voidrunner_test",
			User:     "postgres",
			Password: "password",
			SSLMode:  "disable",
		},
		JWT: config.JWTConfig{
			SecretKey:      "test-secret-key-for-integration-tests",
			AccessExpiry:   time.Hour,
			RefreshExpiry:  24 * time.Hour,
		},
		CORS: config.CORSConfig{
			AllowedOrigins: []string{"*"},
			AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
			AllowedHeaders: []string{"*"},
		},
	}

	// Initialize logger
	log := logger.New("test", "debug")

	// Connect to test database
	var err error
	s.db, err = database.Connect(cfg.Database)
	if err != nil {
		s.T().Skip("Test database not available:", err)
		return
	}

	// Run migrations
	if err := database.Migrate(s.db); err != nil {
		s.T().Fatal("Failed to run migrations:", err)
	}

	// Initialize repositories
	s.repos = database.NewRepositories(s.db)

	// Initialize auth service
	s.authService = auth.NewService(s.repos.Users, cfg.JWT.SecretKey, cfg.JWT.AccessExpiry, cfg.JWT.RefreshExpiry)

	// Setup router
	s.router = gin.New()
	routes.Setup(s.router, cfg, log, s.repos, s.authService)

	// Create test user
	s.createTestUser()
}

// TearDownSuite cleans up after tests
func (s *IntegrationTestSuite) TearDownSuite() {
	if s.db != nil {
		// Clean up test data
		s.cleanupTestData()
		s.db.Close()
	}
}

// SetupTest runs before each test
func (s *IntegrationTestSuite) SetupTest() {
	// Clean up any existing test data
	s.cleanupTestData()
}

// createTestUser creates a test user and gets access token
func (s *IntegrationTestSuite) createTestUser() {
	// Create test user
	registerReq := models.RegisterRequest{
		Email:    "test@example.com",
		Password: "testpassword123",
		Name:     "Test User",
	}

	reqBody, _ := json.Marshal(registerReq)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewBuffer(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	s.router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		s.T().Fatalf("Failed to create test user: %v", w.Body.String())
	}

	var authResponse models.AuthResponse
	err := json.Unmarshal(w.Body.Bytes(), &authResponse)
	require.NoError(s.T(), err)

	s.testUser = &authResponse.User
	s.accessToken = authResponse.AccessToken
}

// cleanupTestData removes test data from database
func (s *IntegrationTestSuite) cleanupTestData() {
	if s.testUser != nil {
		// Delete all task executions for test user
		ctx := context.Background()
		tasks, _ := s.repos.Tasks.GetByUserID(ctx, s.testUser.ID, 1000, 0)
		for _, task := range tasks {
			executions, _ := s.repos.TaskExecutions.GetByTaskID(ctx, task.ID, 1000, 0)
			for _, execution := range executions {
				s.repos.TaskExecutions.Delete(ctx, execution.ID)
			}
			s.repos.Tasks.Delete(ctx, task.ID)
		}
		
		// Delete test user
		s.repos.Users.Delete(ctx, s.testUser.ID)
	}
}

// makeAuthenticatedRequest creates an authenticated HTTP request
func (s *IntegrationTestSuite) makeAuthenticatedRequest(method, path string, body interface{}) *httptest.ResponseRecorder {
	var reqBody *bytes.Buffer
	if body != nil {
		jsonBody, _ := json.Marshal(body)
		reqBody = bytes.NewBuffer(jsonBody)
	} else {
		reqBody = bytes.NewBuffer(nil)
	}

	req := httptest.NewRequest(method, path, reqBody)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.accessToken)
	
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)
	
	return w
}

// TestTaskLifecycle tests the complete task lifecycle
func (s *IntegrationTestSuite) TestTaskLifecycle() {
	// 1. Create a task
	createReq := models.CreateTaskRequest{
		Name:          "Integration Test Task",
		Description:   stringPtr("A test task for integration testing"),
		ScriptContent: "print('Hello, World!')",
		ScriptType:    models.ScriptTypePython,
		Priority:      intPtr(5),
		TimeoutSeconds: intPtr(300),
	}

	w := s.makeAuthenticatedRequest(http.MethodPost, "/api/v1/tasks", createReq)
	assert.Equal(s.T(), http.StatusCreated, w.Code)

	var createdTask models.TaskResponse
	err := json.Unmarshal(w.Body.Bytes(), &createdTask)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), createReq.Name, createdTask.Name)
	assert.Equal(s.T(), createReq.ScriptContent, createdTask.ScriptContent)
	assert.Equal(s.T(), models.TaskStatusPending, createdTask.Status)

	taskID := createdTask.ID

	// 2. Get the created task
	w = s.makeAuthenticatedRequest(http.MethodGet, fmt.Sprintf("/api/v1/tasks/%s", taskID), nil)
	assert.Equal(s.T(), http.StatusOK, w.Code)

	var retrievedTask models.TaskResponse
	err = json.Unmarshal(w.Body.Bytes(), &retrievedTask)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), taskID, retrievedTask.ID)
	assert.Equal(s.T(), createReq.Name, retrievedTask.Name)

	// 3. List tasks (should include our task)
	w = s.makeAuthenticatedRequest(http.MethodGet, "/api/v1/tasks?limit=10&offset=0", nil)
	assert.Equal(s.T(), http.StatusOK, w.Code)

	var taskList models.TaskListResponse
	err = json.Unmarshal(w.Body.Bytes(), &taskList)
	require.NoError(s.T(), err)
	assert.GreaterOrEqual(s.T(), len(taskList.Tasks), 1)
	assert.GreaterOrEqual(s.T(), taskList.Total, int64(1))

	// Find our task in the list
	found := false
	for _, task := range taskList.Tasks {
		if task.ID == taskID {
			found = true
			break
		}
	}
	assert.True(s.T(), found, "Created task should be in the list")

	// 4. Update the task
	updateReq := models.UpdateTaskRequest{
		Name:        stringPtr("Updated Integration Test Task"),
		Description: stringPtr("An updated test task"),
	}

	w = s.makeAuthenticatedRequest(http.MethodPut, fmt.Sprintf("/api/v1/tasks/%s", taskID), updateReq)
	assert.Equal(s.T(), http.StatusOK, w.Code)

	var updatedTask models.TaskResponse
	err = json.Unmarshal(w.Body.Bytes(), &updatedTask)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), *updateReq.Name, updatedTask.Name)
	assert.Equal(s.T(), *updateReq.Description, *updatedTask.Description)

	// 5. Start task execution
	w = s.makeAuthenticatedRequest(http.MethodPost, fmt.Sprintf("/api/v1/tasks/%s/executions", taskID), nil)
	assert.Equal(s.T(), http.StatusCreated, w.Code)

	var execution models.TaskExecutionResponse
	err = json.Unmarshal(w.Body.Bytes(), &execution)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), taskID, execution.TaskID)
	assert.Equal(s.T(), models.ExecutionStatusPending, execution.Status)

	executionID := execution.ID

	// 6. Get execution details
	w = s.makeAuthenticatedRequest(http.MethodGet, fmt.Sprintf("/api/v1/executions/%s", executionID), nil)
	assert.Equal(s.T(), http.StatusOK, w.Code)

	var retrievedExecution models.TaskExecutionResponse
	err = json.Unmarshal(w.Body.Bytes(), &retrievedExecution)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), executionID, retrievedExecution.ID)
	assert.Equal(s.T(), taskID, retrievedExecution.TaskID)

	// 7. Update execution status (simulate completion)
	updateExecReq := models.UpdateTaskExecutionRequest{
		Status:           statusPtr(models.ExecutionStatusCompleted),
		ReturnCode:       intPtr(0),
		Stdout:           stringPtr("Hello, World!\n"),
		ExecutionTimeMs:  intPtr(1250),
		MemoryUsageBytes: int64Ptr(15728640),
	}

	w = s.makeAuthenticatedRequest(http.MethodPut, fmt.Sprintf("/api/v1/executions/%s", executionID), updateExecReq)
	assert.Equal(s.T(), http.StatusOK, w.Code)

	var completedExecution models.TaskExecutionResponse
	err = json.Unmarshal(w.Body.Bytes(), &completedExecution)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), models.ExecutionStatusCompleted, completedExecution.Status)
	assert.Equal(s.T(), *updateExecReq.ReturnCode, *completedExecution.ReturnCode)
	assert.Equal(s.T(), *updateExecReq.Stdout, *completedExecution.Stdout)

	// 8. List task executions
	w = s.makeAuthenticatedRequest(http.MethodGet, fmt.Sprintf("/api/v1/tasks/%s/executions", taskID), nil)
	assert.Equal(s.T(), http.StatusOK, w.Code)

	var executionList models.ExecutionListResponse
	err = json.Unmarshal(w.Body.Bytes(), &executionList)
	require.NoError(s.T(), err)
	assert.GreaterOrEqual(s.T(), len(executionList.Executions), 1)
	assert.GreaterOrEqual(s.T(), executionList.Total, int64(1))

	// 9. Delete the task
	w = s.makeAuthenticatedRequest(http.MethodDelete, fmt.Sprintf("/api/v1/tasks/%s", taskID), nil)
	assert.Equal(s.T(), http.StatusOK, w.Code)

	// 10. Verify task is deleted
	w = s.makeAuthenticatedRequest(http.MethodGet, fmt.Sprintf("/api/v1/tasks/%s", taskID), nil)
	assert.Equal(s.T(), http.StatusNotFound, w.Code)
}

// TestAuthenticationFlow tests the authentication workflow
func (s *IntegrationTestSuite) TestAuthenticationFlow() {
	// 1. Try to access protected endpoint without token
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks", nil)
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)
	assert.Equal(s.T(), http.StatusUnauthorized, w.Code)

	// 2. Register a new user
	registerReq := models.RegisterRequest{
		Email:    "auth_test@example.com",
		Password: "testpassword123",
		Name:     "Auth Test User",
	}

	reqBody, _ := json.Marshal(registerReq)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewBuffer(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	s.router.ServeHTTP(w, req)
	assert.Equal(s.T(), http.StatusCreated, w.Code)

	var authResponse models.AuthResponse
	err := json.Unmarshal(w.Body.Bytes(), &authResponse)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), registerReq.Email, authResponse.User.Email)
	assert.NotEmpty(s.T(), authResponse.AccessToken)
	assert.NotEmpty(s.T(), authResponse.RefreshToken)

	// 3. Use access token to access protected endpoint
	req = httptest.NewRequest(http.MethodGet, "/api/v1/tasks", nil)
	req.Header.Set("Authorization", "Bearer "+authResponse.AccessToken)
	w = httptest.NewRecorder()
	s.router.ServeHTTP(w, req)
	assert.Equal(s.T(), http.StatusOK, w.Code)

	// 4. Test refresh token
	refreshReq := models.RefreshTokenRequest{
		RefreshToken: authResponse.RefreshToken,
	}

	reqBody, _ = json.Marshal(refreshReq)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", bytes.NewBuffer(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	s.router.ServeHTTP(w, req)
	assert.Equal(s.T(), http.StatusOK, w.Code)

	var refreshResponse models.AuthResponse
	err = json.Unmarshal(w.Body.Bytes(), &refreshResponse)
	require.NoError(s.T(), err)
	assert.NotEmpty(s.T(), refreshResponse.AccessToken)
	assert.NotEqual(s.T(), authResponse.AccessToken, refreshResponse.AccessToken)

	// Clean up - delete test user
	s.repos.Users.Delete(context.Background(), authResponse.User.ID)
}

// TestValidationErrors tests input validation
func (s *IntegrationTestSuite) TestValidationErrors() {
	// Test invalid task creation
	invalidReq := models.CreateTaskRequest{
		Name:          "", // Empty name should fail
		ScriptContent: "rm -rf /", // Dangerous script should fail
		ScriptType:    "invalid", // Invalid script type
	}

	w := s.makeAuthenticatedRequest(http.MethodPost, "/api/v1/tasks", invalidReq)
	assert.Equal(s.T(), http.StatusBadRequest, w.Code)

	var errorResponse models.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &errorResponse)
	require.NoError(s.T(), err)
	assert.Contains(s.T(), errorResponse.Error, "Validation failed")
	assert.NotEmpty(s.T(), errorResponse.ValidationErrors)
}

// TestAccessControl tests that users can only access their own resources
func (s *IntegrationTestSuite) TestAccessControl() {
	// Create another user
	registerReq := models.RegisterRequest{
		Email:    "other_user@example.com",
		Password: "testpassword123",
		Name:     "Other User",
	}

	reqBody, _ := json.Marshal(registerReq)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewBuffer(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)
	require.Equal(s.T(), http.StatusCreated, w.Code)

	var otherUserAuth models.AuthResponse
	err := json.Unmarshal(w.Body.Bytes(), &otherUserAuth)
	require.NoError(s.T(), err)

	// Create a task with the other user
	createReq := models.CreateTaskRequest{
		Name:          "Other User's Task",
		ScriptContent: "print('other user task')",
		ScriptType:    models.ScriptTypePython,
	}

	reqBody, _ = json.Marshal(createReq)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/tasks", bytes.NewBuffer(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+otherUserAuth.AccessToken)
	w = httptest.NewRecorder()
	s.router.ServeHTTP(w, req)
	require.Equal(s.T(), http.StatusCreated, w.Code)

	var otherUserTask models.TaskResponse
	err = json.Unmarshal(w.Body.Bytes(), &otherUserTask)
	require.NoError(s.T(), err)

	// Try to access other user's task with our token (should fail)
	w = s.makeAuthenticatedRequest(http.MethodGet, fmt.Sprintf("/api/v1/tasks/%s", otherUserTask.ID), nil)
	assert.Equal(s.T(), http.StatusForbidden, w.Code)

	// Clean up
	s.repos.Tasks.Delete(context.Background(), otherUserTask.ID)
	s.repos.Users.Delete(context.Background(), otherUserAuth.User.ID)
}

// TestRateLimit tests rate limiting functionality
func (s *IntegrationTestSuite) TestRateLimit() {
	// Note: This test might be slow as it needs to make many requests
	// You could reduce rate limits for testing or skip this test in CI
	s.T().Skip("Rate limiting test skipped - would be too slow for regular testing")

	// Create many tasks quickly to trigger rate limit
	for i := 0; i < 25; i++ { // More than the 20/hour limit
		createReq := models.CreateTaskRequest{
			Name:          fmt.Sprintf("Rate Limit Test Task %d", i),
			ScriptContent: "print('rate limit test')",
			ScriptType:    models.ScriptTypePython,
		}

		w := s.makeAuthenticatedRequest(http.MethodPost, "/api/v1/tasks", createReq)
		if i < 20 {
			assert.Equal(s.T(), http.StatusCreated, w.Code)
		} else {
			// Should be rate limited
			assert.Equal(s.T(), http.StatusTooManyRequests, w.Code)
		}
	}
}

// Helper functions
func stringPtr(s string) *string {
	return &s
}

func intPtr(i int) *int {
	return &i
}

func int64Ptr(i int64) *int64 {
	return &i
}

func statusPtr(s models.ExecutionStatus) *models.ExecutionStatus {
	return &s
}

// TestIntegrationSuite runs the integration test suite
func TestIntegrationSuite(t *testing.T) {
	suite.Run(t, new(IntegrationTestSuite))
}
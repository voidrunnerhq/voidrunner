//go:build integration

package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/voidrunnerhq/voidrunner/internal/api/routes"
	"github.com/voidrunnerhq/voidrunner/internal/auth"
	"github.com/voidrunnerhq/voidrunner/internal/database"
	"github.com/voidrunnerhq/voidrunner/internal/models"
	"github.com/voidrunnerhq/voidrunner/pkg/logger"
	"github.com/voidrunnerhq/voidrunner/tests/testutil"
)

// ContractTestSuite provides OpenAPI contract validation tests
type ContractTestSuite struct {
	suite.Suite
	router      *gin.Engine
	repos       *database.Repositories
	authService *auth.Service
	db          *database.Connection
	validator   *testutil.OpenAPIValidator
	dbHelper    *testutil.DatabaseHelper
}

// SetupSuite initializes the contract test suite
func (s *ContractTestSuite) SetupSuite() {
	gin.SetMode(gin.TestMode)

	// Initialize test configuration using testutil helper
	cfg := testutil.GetTestConfig()

	// Initialize logger
	log := logger.New("contract-test", "debug")

	// Initialize database helper (handles connection, migrations, etc.)
	s.dbHelper = testutil.NewDatabaseHelper(s.T())
	if s.dbHelper == nil {
		s.T().Skip("Test database not available")
		return
	}

	s.db = s.dbHelper.DB
	s.repos = s.dbHelper.Repositories

	// Initialize JWT service and auth service
	jwtSvc := auth.NewJWTService(&cfg.JWT)
	s.authService = auth.NewService(s.repos.Users, jwtSvc, log.Logger, cfg)

	// Setup router
	s.router = gin.New()
	routes.Setup(s.router, cfg, log, s.db, s.repos, s.authService)

	// Initialize OpenAPI validator
	s.validator = testutil.NewOpenAPIValidator()
}

// TearDownSuite cleans up after tests
func (s *ContractTestSuite) TearDownSuite() {
	if s.dbHelper != nil {
		s.dbHelper.Close()
	}
}

// SetupTest runs before each test
func (s *ContractTestSuite) SetupTest() {
	// Clean up any existing test data
	if s.dbHelper != nil {
		s.dbHelper.CleanupDatabase(s.T())
	}
}

// createTestUserWithEmail creates a test user with specific email and returns auth response
func (s *ContractTestSuite) createTestUserWithEmail(email string) *models.AuthResponse {
	// Create test user
	registerReq := models.RegisterRequest{
		Email:    email,
		Password: "ContractPassword123!",
		Name:     "Contract Test User",
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

	return &authResponse
}

// createTestUser creates a test user with default email and returns auth response
func (s *ContractTestSuite) createTestUser() *models.AuthResponse {
	return s.createTestUserWithEmail("contract@example.com")
}

// makeRequest creates an HTTP request and validates it against OpenAPI spec
func (s *ContractTestSuite) makeRequest(method, path string, body interface{}, headers map[string]string) *testutil.HTTPResponseValidator {
	var reqBody *bytes.Buffer
	if body != nil {
		jsonBody, _ := json.Marshal(body)
		reqBody = bytes.NewBuffer(jsonBody)
	} else {
		reqBody = bytes.NewBuffer(nil)
	}

	req := httptest.NewRequest(method, path, reqBody)

	// Set default headers
	req.Header.Set("Content-Type", "application/json")

	// Set additional headers
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)

	return testutil.NewHTTPResponseValidator(s.T(), w.Result()).
		ExpectOpenAPICompliance(method, path).
		ExpectCommonHeaders()
}

// makeAuthenticatedRequest creates an authenticated HTTP request
func (s *ContractTestSuite) makeAuthenticatedRequest(method, path string, body interface{}, accessToken string) *testutil.HTTPResponseValidator {
	headers := map[string]string{
		"Authorization": "Bearer " + accessToken,
	}
	return s.makeRequest(method, path, body, headers)
}

// TestContractTestSuite runs the contract test suite
func TestContractTestSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping contract tests in short mode")
	}

	suite.Run(t, new(ContractTestSuite))
}

// TestAuthenticationEndpointsContract tests authentication endpoints against OpenAPI spec
func (s *ContractTestSuite) TestAuthenticationEndpointsContract() {
	s.Run("register endpoint contract", func() {
		registerReq := models.RegisterRequest{
			Email:    "contract_register@example.com",
			Password: "ContractPassword123!",
			Name:     "Contract Register User",
		}

		validator := s.makeRequest("POST", "/api/v1/auth/register", registerReq, nil).
			ExpectStatus(http.StatusCreated).
			ExpectContentType("application/json").
			ExpectValidJSON()

		var authResponse models.AuthResponse
		validator.UnmarshalResponse(&authResponse)

		// Validate response structure matches OpenAPI spec
		assert.NotEmpty(s.T(), authResponse.User.ID)
		assert.Equal(s.T(), registerReq.Email, authResponse.User.Email)
		assert.Equal(s.T(), registerReq.Name, authResponse.User.Name)
		assert.NotEmpty(s.T(), authResponse.AccessToken)
		assert.NotEmpty(s.T(), authResponse.RefreshToken)

		// Cleanup
		s.repos.Users.Delete(context.Background(), authResponse.User.ID)
	})

	s.Run("register validation error contract", func() {
		invalidReq := models.RegisterRequest{
			Email:    "",      // Invalid: empty email
			Password: "short", // Invalid: too short
			Name:     "",      // Invalid: empty name
		}

		validator := s.makeRequest("POST", "/api/v1/auth/register", invalidReq, nil).
			ExpectStatus(http.StatusBadRequest).
			ExpectContentType("application/json").
			ExpectValidJSON()

		var errorResponse models.ErrorResponse
		validator.UnmarshalResponse(&errorResponse)

		// Validate error response structure
		assert.NotEmpty(s.T(), errorResponse.Error)
		assert.Contains(s.T(), errorResponse.Error, "invalid email")
	})

	s.Run("login endpoint contract", func() {
		// First register a user
		registerReq := models.RegisterRequest{
			Email:    "contract_login@example.com",
			Password: "ContractPassword123!",
			Name:     "Contract Login User",
		}

		registerValidator := s.makeRequest("POST", "/api/v1/auth/register", registerReq, nil).
			ExpectStatus(http.StatusCreated)

		var registerResponse models.AuthResponse
		registerValidator.UnmarshalResponse(&registerResponse)

		// Then test login
		loginReq := models.LoginRequest{
			Email:    registerReq.Email,
			Password: registerReq.Password,
		}

		loginValidator := s.makeRequest("POST", "/api/v1/auth/login", loginReq, nil).
			ExpectStatus(http.StatusOK).
			ExpectContentType("application/json").
			ExpectValidJSON()

		var loginResponse models.AuthResponse
		loginValidator.UnmarshalResponse(&loginResponse)

		// Validate response structure
		assert.Equal(s.T(), registerResponse.User.ID, loginResponse.User.ID)
		assert.Equal(s.T(), registerReq.Email, loginResponse.User.Email)
		assert.NotEmpty(s.T(), loginResponse.AccessToken)
		assert.NotEmpty(s.T(), loginResponse.RefreshToken)

		// Cleanup
		s.repos.Users.Delete(context.Background(), registerResponse.User.ID)
	})
}

// TestTaskEndpointsContract tests task endpoints against OpenAPI spec
func (s *ContractTestSuite) TestTaskEndpointsContract() {
	s.Run("create task endpoint contract", func() {
		// Create test user for authentication
		authResp := s.createTestUserWithEmail("task-create@example.com")

		createReq := models.CreateTaskRequest{
			Name:           "Contract Test Task",
			Description:    testutil.StringPtr("A task for contract testing"),
			ScriptContent:  "print('Contract test')",
			ScriptType:     models.ScriptTypePython,
			Priority:       testutil.IntPtr(5),
			TimeoutSeconds: testutil.IntPtr(300),
		}

		validator := s.makeAuthenticatedRequest("POST", "/api/v1/tasks", createReq, authResp.AccessToken).
			ExpectStatus(http.StatusCreated).
			ExpectContentType("application/json").
			ExpectValidJSON()

		var taskResponse models.TaskResponse
		validator.UnmarshalResponse(&taskResponse)

		// Validate response structure matches OpenAPI spec
		assert.NotEmpty(s.T(), taskResponse.ID)
		assert.Equal(s.T(), createReq.Name, taskResponse.Name)
		assert.Equal(s.T(), createReq.ScriptContent, taskResponse.ScriptContent)
		assert.Equal(s.T(), createReq.ScriptType, taskResponse.ScriptType)
		assert.Equal(s.T(), models.TaskStatusPending, taskResponse.Status)
		assert.NotEmpty(s.T(), taskResponse.CreatedAt)

		// Cleanup
		s.repos.Tasks.Delete(context.Background(), taskResponse.ID)
	})

	s.Run("list tasks endpoint contract", func() {
		// Create test user for authentication
		authResp := s.createTestUserWithEmail("task-list@example.com")

		// Create a test task first
		createReq := models.CreateTaskRequest{
			Name:          "Contract List Test Task",
			ScriptContent: "print('Contract list test')",
			ScriptType:    models.ScriptTypePython,
		}

		createValidator := s.makeAuthenticatedRequest("POST", "/api/v1/tasks", createReq, authResp.AccessToken).
			ExpectStatus(http.StatusCreated)

		var createdTask models.TaskResponse
		createValidator.UnmarshalResponse(&createdTask)

		// Test list endpoint
		listValidator := s.makeAuthenticatedRequest("GET", "/api/v1/tasks?limit=10&offset=0", nil, authResp.AccessToken).
			ExpectStatus(http.StatusOK).
			ExpectContentType("application/json").
			ExpectValidJSON()

		var listResponse models.TaskListResponse
		listValidator.UnmarshalResponse(&listResponse)

		// Validate response structure
		assert.NotNil(s.T(), listResponse.Tasks)
		assert.GreaterOrEqual(s.T(), listResponse.Total, int64(1))
		assert.Equal(s.T(), 10, listResponse.Limit)
		assert.Equal(s.T(), 0, listResponse.Offset)

		// Validate task structure in list
		if len(listResponse.Tasks) > 0 {
			task := listResponse.Tasks[0]
			assert.NotEmpty(s.T(), task.ID)
			assert.NotEmpty(s.T(), task.Name)
			assert.NotEmpty(s.T(), task.Status)
			assert.NotEmpty(s.T(), task.ScriptContent)
			assert.NotEmpty(s.T(), task.ScriptType)
			assert.NotEmpty(s.T(), task.CreatedAt)
		}

		// Cleanup
		s.repos.Tasks.Delete(context.Background(), createdTask.ID)
	})
}

// TestHealthEndpointContract tests health endpoints against OpenAPI spec
func (s *ContractTestSuite) TestHealthEndpointContract() {
	s.Run("health endpoint contract", func() {
		validator := s.makeRequest("GET", "/health", nil, nil).
			ExpectStatus(http.StatusOK).
			ExpectContentType("application/json").
			ExpectValidJSON()

		var healthResponse map[string]interface{}
		validator.UnmarshalResponse(&healthResponse)

		// Validate response structure
		assert.Contains(s.T(), healthResponse, "status")
		assert.Contains(s.T(), healthResponse, "timestamp")
		assert.Equal(s.T(), "ok", healthResponse["status"])
	})

	s.Run("readiness endpoint contract", func() {
		validator := s.makeRequest("GET", "/ready", nil, nil).
			ExpectStatus(http.StatusOK).
			ExpectContentType("application/json").
			ExpectValidJSON()

		var readyResponse map[string]interface{}
		validator.UnmarshalResponse(&readyResponse)

		// Validate response structure
		assert.Contains(s.T(), readyResponse, "status")
		assert.Contains(s.T(), readyResponse, "timestamp")
	})
}

// TestErrorResponsesContract tests error responses against OpenAPI spec
func (s *ContractTestSuite) TestErrorResponsesContract() {
	s.Run("unauthorized access contract", func() {
		// Try to access protected endpoint without token
		validator := s.makeRequest("GET", "/api/v1/tasks", nil, nil).
			ExpectStatus(http.StatusUnauthorized).
			ExpectContentType("application/json").
			ExpectValidJSON()

		var errorResponse models.ErrorResponse
		validator.UnmarshalResponse(&errorResponse)

		// Validate error response structure
		assert.NotEmpty(s.T(), errorResponse.Error)
		assert.Contains(s.T(), errorResponse.Error, "authorization")
	})

	s.Run("invalid request body contract", func() {
		// Create test user for authentication
		authResp := s.createTestUserWithEmail("invalid-body@example.com")

		invalidJSON := `{"name": "test", "invalid": json}`

		req := httptest.NewRequest("POST", "/api/v1/tasks", bytes.NewBufferString(invalidJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+authResp.AccessToken)

		w := httptest.NewRecorder()
		s.router.ServeHTTP(w, req)

		validator := testutil.NewHTTPResponseValidator(s.T(), w.Result()).
			ExpectStatus(http.StatusBadRequest).
			ExpectContentType("application/json").
			ExpectValidJSON()

		var errorResponse models.ErrorResponse
		validator.UnmarshalResponse(&errorResponse)

		// Validate error response structure
		assert.NotEmpty(s.T(), errorResponse.Error)
	})

	s.Run("resource not found contract", func() {
		// Create test user for authentication
		authResp := s.createTestUserWithEmail("task-notfound@example.com")

		nonExistentID := "123e4567-e89b-12d3-a456-426614174000"

		validator := s.makeAuthenticatedRequest("GET", "/api/v1/tasks/"+nonExistentID, nil, authResp.AccessToken).
			ExpectStatus(http.StatusNotFound).
			ExpectContentType("application/json").
			ExpectValidJSON()

		var errorResponse models.ErrorResponse
		validator.UnmarshalResponse(&errorResponse)

		// Validate error response structure
		assert.NotEmpty(s.T(), errorResponse.Error)
		assert.Contains(s.T(), errorResponse.Error, "not found")
	})
}

// TestSecurityHeadersContract tests security headers compliance
func (s *ContractTestSuite) TestSecurityHeadersContract() {
	s.Run("auth endpoints security headers", func() {
		req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewBuffer([]byte(`{}`)))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		s.router.ServeHTTP(w, req)

		response := w.Result()

		// Validate security headers
		s.validator.ValidateCommonHeaders(s.T(), response)
	})
}

package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/voidrunnerhq/voidrunner/internal/api/middleware"
	"github.com/voidrunnerhq/voidrunner/internal/models"
	"github.com/voidrunnerhq/voidrunner/pkg/logger"
)

// HandlerIntegrationTest tests the interaction between handlers and middleware
func TestHandlerIntegration_TaskWithValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Setup mocks
	mockTaskRepo := new(MockTaskRepository)
	mockExecutionRepo := new(MockTaskExecutionRepository)
	logger := logger.New("test", "debug")

	// Setup handlers
	mockExecutionService := new(MockTaskExecutionService)
	taskHandler := NewTaskHandler(mockTaskRepo, logger.Logger)
	executionHandler := NewTaskExecutionHandler(mockTaskRepo, mockExecutionRepo, mockExecutionService, logger.Logger)
	validationMiddleware := middleware.TaskValidation(logger.Logger)

	// Setup router with middleware
	router := gin.New()

	// Add test user to context
	userID := uuid.New()
	router.Use(func(c *gin.Context) {
		user := &models.User{
			BaseModel: models.BaseModel{
				ID: userID,
			},
			Email: "test@example.com",
		}
		c.Set("user", user)
		c.Set("user_id", userID)
		c.Next()
	})

	// Setup routes with validation
	router.POST("/tasks",
		middleware.RequestSizeLimit(logger.Logger),
		validationMiddleware.ValidateTaskCreation(),
		taskHandler.Create,
	)
	router.PUT("/tasks/:id",
		middleware.RequestSizeLimit(logger.Logger),
		validationMiddleware.ValidateTaskUpdate(),
		taskHandler.Update,
	)
	router.POST("/tasks/:id/executions", executionHandler.Create)
	router.PUT("/executions/:id",
		validationMiddleware.ValidateTaskExecutionUpdate(),
		executionHandler.Update,
	)

	t.Run("Valid Task Creation with Validation", func(t *testing.T) {
		// Setup mock expectations
		mockTaskRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.Task")).Return(nil)

		// Create valid request
		priority := 5
		timeout := 300
		description := "A valid task"
		req := models.CreateTaskRequest{
			Name:           "Valid Task",
			Description:    &description,
			ScriptContent:  "print('Hello, World!')",
			ScriptType:     models.ScriptTypePython,
			Priority:       &priority,
			TimeoutSeconds: &timeout,
		}

		reqBody, _ := json.Marshal(req)
		httpReq := httptest.NewRequest(http.MethodPost, "/tasks", bytes.NewBuffer(reqBody))
		httpReq.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, httpReq)

		assert.Equal(t, http.StatusCreated, w.Code)
		mockTaskRepo.AssertExpectations(t)
	})

	t.Run("Invalid Task Creation - Dangerous Script", func(t *testing.T) {
		// Create request with dangerous script
		req := models.CreateTaskRequest{
			Name:          "Dangerous Task",
			ScriptContent: "rm -rf /",
			ScriptType:    models.ScriptTypePython,
		}

		reqBody, _ := json.Marshal(req)
		httpReq := httptest.NewRequest(http.MethodPost, "/tasks", bytes.NewBuffer(reqBody))
		httpReq.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, httpReq)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var errorResponse map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &errorResponse)
		require.NoError(t, err)
		assert.Contains(t, errorResponse["error"], "Validation failed")
	})

	t.Run("Invalid Task Creation - Empty Name", func(t *testing.T) {
		// Create request with empty name
		req := models.CreateTaskRequest{
			Name:          "",
			ScriptContent: "print('Hello')",
			ScriptType:    models.ScriptTypePython,
		}

		reqBody, _ := json.Marshal(req)
		httpReq := httptest.NewRequest(http.MethodPost, "/tasks", bytes.NewBuffer(reqBody))
		httpReq.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, httpReq)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("Request Size Limit", func(t *testing.T) {
		// Create a very large request (over 1MB)
		largeScript := make([]byte, 1024*1024+1) // 1MB + 1 byte
		for i := range largeScript {
			largeScript[i] = 'a'
		}

		req := models.CreateTaskRequest{
			Name:          "Large Task",
			ScriptContent: string(largeScript),
			ScriptType:    models.ScriptTypePython,
		}

		reqBody, _ := json.Marshal(req)
		httpReq := httptest.NewRequest(http.MethodPost, "/tasks", bytes.NewBuffer(reqBody))
		httpReq.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, httpReq)

		assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
	})
}

// TestValidationMiddlewareIntegration tests validation middleware integration
func TestValidationMiddlewareIntegration(t *testing.T) {
	gin.SetMode(gin.TestMode)

	logger := logger.New("test", "debug")
	validationMiddleware := middleware.TaskValidation(logger.Logger)

	router := gin.New()

	router.POST("/validate-task",
		validationMiddleware.ValidateTaskCreation(),
		func(c *gin.Context) {
			// Get validated body from middleware
			validatedBody, exists := c.Get("validated_body")
			if !exists {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "validation middleware failed"})
				return
			}

			req := validatedBody.(*models.CreateTaskRequest)
			c.JSON(http.StatusOK, gin.H{
				"message": "validation passed",
				"name":    req.Name,
			})
		},
	)

	t.Run("Validation Middleware Stores Validated Body", func(t *testing.T) {
		req := models.CreateTaskRequest{
			Name:          "Valid Task",
			ScriptContent: "print('hello')",
			ScriptType:    models.ScriptTypePython,
		}

		reqBody, _ := json.Marshal(req)
		httpReq := httptest.NewRequest(http.MethodPost, "/validate-task", bytes.NewBuffer(reqBody))
		httpReq.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, httpReq)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "validation passed", response["message"])
		assert.Equal(t, req.Name, response["name"])
	})

	t.Run("Validation Middleware Blocks Invalid Data", func(t *testing.T) {
		invalidReq := models.CreateTaskRequest{
			Name:          "",         // Invalid: empty name
			ScriptContent: "rm -rf /", // Invalid: dangerous script
			ScriptType:    "invalid",  // Invalid: bad script type
		}

		reqBody, _ := json.Marshal(invalidReq)
		httpReq := httptest.NewRequest(http.MethodPost, "/validate-task", bytes.NewBuffer(reqBody))
		httpReq.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, httpReq)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var errorResponse map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &errorResponse)
		require.NoError(t, err)
		assert.Contains(t, errorResponse["error"], "Validation failed")
		assert.NotNil(t, errorResponse["validation_errors"])

		// Check that we have multiple validation errors
		validationErrors := errorResponse["validation_errors"].([]interface{})
		assert.GreaterOrEqual(t, len(validationErrors), 2)
	})
}

package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/voidrunnerhq/voidrunner/internal/database"
	"github.com/voidrunnerhq/voidrunner/internal/models"
)

// MockTaskExecutionRepository is a mock implementation of TaskExecutionRepository
type MockTaskExecutionRepository struct {
	mock.Mock
}

func (m *MockTaskExecutionRepository) Create(ctx context.Context, execution *models.TaskExecution) error {
	args := m.Called(ctx, execution)
	return args.Error(0)
}

func (m *MockTaskExecutionRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.TaskExecution, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.TaskExecution), args.Error(1)
}

func (m *MockTaskExecutionRepository) GetByTaskID(ctx context.Context, taskID uuid.UUID, limit, offset int) ([]*models.TaskExecution, error) {
	args := m.Called(ctx, taskID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.TaskExecution), args.Error(1)
}

func (m *MockTaskExecutionRepository) GetLatestByTaskID(ctx context.Context, taskID uuid.UUID) (*models.TaskExecution, error) {
	args := m.Called(ctx, taskID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.TaskExecution), args.Error(1)
}

func (m *MockTaskExecutionRepository) GetByStatus(ctx context.Context, status models.ExecutionStatus, limit, offset int) ([]*models.TaskExecution, error) {
	args := m.Called(ctx, status, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.TaskExecution), args.Error(1)
}

func (m *MockTaskExecutionRepository) Update(ctx context.Context, execution *models.TaskExecution) error {
	args := m.Called(ctx, execution)
	return args.Error(0)
}

func (m *MockTaskExecutionRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status models.ExecutionStatus) error {
	args := m.Called(ctx, id, status)
	return args.Error(0)
}

func (m *MockTaskExecutionRepository) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockTaskExecutionRepository) List(ctx context.Context, limit, offset int) ([]*models.TaskExecution, error) {
	args := m.Called(ctx, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.TaskExecution), args.Error(1)
}

func (m *MockTaskExecutionRepository) Count(ctx context.Context) (int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockTaskExecutionRepository) CountByTaskID(ctx context.Context, taskID uuid.UUID) (int64, error) {
	args := m.Called(ctx, taskID)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockTaskExecutionRepository) CountByStatus(ctx context.Context, status models.ExecutionStatus) (int64, error) {
	args := m.Called(ctx, status)
	return args.Get(0).(int64), args.Error(1)
}

// Cursor-based pagination methods
func (m *MockTaskExecutionRepository) GetByTaskIDCursor(ctx context.Context, taskID uuid.UUID, req database.CursorPaginationRequest) ([]*models.TaskExecution, database.CursorPaginationResponse, error) {
	args := m.Called(ctx, taskID, req)
	if args.Get(0) == nil {
		return nil, database.CursorPaginationResponse{}, args.Error(2)
	}
	return args.Get(0).([]*models.TaskExecution), args.Get(1).(database.CursorPaginationResponse), args.Error(2)
}

func (m *MockTaskExecutionRepository) GetByStatusCursor(ctx context.Context, status models.ExecutionStatus, req database.CursorPaginationRequest) ([]*models.TaskExecution, database.CursorPaginationResponse, error) {
	args := m.Called(ctx, status, req)
	if args.Get(0) == nil {
		return nil, database.CursorPaginationResponse{}, args.Error(2)
	}
	return args.Get(0).([]*models.TaskExecution), args.Get(1).(database.CursorPaginationResponse), args.Error(2)
}

func (m *MockTaskExecutionRepository) ListCursor(ctx context.Context, req database.CursorPaginationRequest) ([]*models.TaskExecution, database.CursorPaginationResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, database.CursorPaginationResponse{}, args.Error(2)
	}
	return args.Get(0).([]*models.TaskExecution), args.Get(1).(database.CursorPaginationResponse), args.Error(2)
}

func setupTaskExecutionHandlerTest() (*gin.Engine, *MockTaskRepository, *MockTaskExecutionRepository, *MockTaskExecutionService, *TaskExecutionHandler) {
	gin.SetMode(gin.TestMode)
	
	mockTaskRepo := new(MockTaskRepository)
	mockExecutionRepo := new(MockTaskExecutionRepository)
	mockExecutionService := new(MockTaskExecutionService)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	handler := NewTaskExecutionHandler(mockTaskRepo, mockExecutionRepo, mockExecutionService, logger)
	
	router := gin.New()
	// Add middleware to set user context
	router.Use(func(c *gin.Context) {
		user := &models.User{
			BaseModel: models.BaseModel{
				ID: uuid.New(),
			},
			Email: "test@example.com",
		}
		c.Set("user", user)
		c.Next()
	})
	
	return router, mockTaskRepo, mockExecutionRepo, mockExecutionService, handler
}

func TestTaskExecutionHandler_Create(t *testing.T) {
	taskID := uuid.New()
	userID := uuid.New()
	
	tests := []struct {
		name           string
		taskID         string
		mockSetup      func(*MockTaskRepository, *MockTaskExecutionRepository, *MockTaskExecutionService)
		wantStatus     int
		wantError      string
	}{
		{
			name:   "successful execution creation",
			taskID: taskID.String(),
			mockSetup: func(mt *MockTaskRepository, me *MockTaskExecutionRepository, ms *MockTaskExecutionService) {
				expectedExecution := &models.TaskExecution{
					ID:     uuid.New(),
					TaskID: taskID,
					Status: models.ExecutionStatusPending,
				}
				// The Create handler now only calls the service
				ms.On("CreateExecutionAndUpdateTaskStatus", mock.Anything, taskID, userID).Return(expectedExecution, nil)
			},
			wantStatus: http.StatusCreated,
		},
		{
			name:   "invalid task ID",
			taskID: "invalid-uuid",
			mockSetup: func(mt *MockTaskRepository, me *MockTaskExecutionRepository, ms *MockTaskExecutionService) {
				// No mock calls expected
			},
			wantStatus: http.StatusBadRequest,
			wantError:  "Invalid task ID format",
		},
		{
			name:   "task not found",
			taskID: taskID.String(),
			mockSetup: func(mt *MockTaskRepository, me *MockTaskExecutionRepository, ms *MockTaskExecutionService) {
				ms.On("CreateExecutionAndUpdateTaskStatus", mock.Anything, taskID, userID).Return(nil, fmt.Errorf("task not found"))
			},
			wantStatus: http.StatusNotFound,
			wantError:  "Task not found",
		},
		{
			name:   "access denied - different user",
			taskID: taskID.String(),
			mockSetup: func(mt *MockTaskRepository, me *MockTaskExecutionRepository, ms *MockTaskExecutionService) {
				ms.On("CreateExecutionAndUpdateTaskStatus", mock.Anything, taskID, userID).Return(nil, fmt.Errorf("access denied: task does not belong to user"))
			},
			wantStatus: http.StatusForbidden,
			wantError:  "Access denied",
		},
		{
			name:   "task already running",
			taskID: taskID.String(),
			mockSetup: func(mt *MockTaskRepository, me *MockTaskExecutionRepository, ms *MockTaskExecutionService) {
				ms.On("CreateExecutionAndUpdateTaskStatus", mock.Anything, taskID, userID).Return(nil, fmt.Errorf("task is already running"))
			},
			wantStatus: http.StatusConflict,
			wantError:  "Task is already running",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router, mockTaskRepo, mockExecutionRepo, mockExecutionService, handler := setupTaskExecutionHandlerTest()
			tt.mockSetup(mockTaskRepo, mockExecutionRepo, mockExecutionService)
			
			// Override the user context with known user ID
			router.Use(func(c *gin.Context) {
				user := &models.User{
					BaseModel: models.BaseModel{
						ID: userID,
					},
					Email: "test@example.com",
				}
				c.Set("user", user)
				c.Next()
			})
			
			router.POST("/tasks/:task_id/executions", handler.Create)

			req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/tasks/%s/executions", tt.taskID), nil)
			w := httptest.NewRecorder()
			
			router.ServeHTTP(w, req)
			
			assert.Equal(t, tt.wantStatus, w.Code)
			
			if tt.wantError != "" {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Contains(t, response["error"], tt.wantError)
			} else if tt.wantStatus == http.StatusCreated {
				var response models.TaskExecutionResponse
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Equal(t, taskID, response.TaskID)
				assert.Equal(t, models.ExecutionStatusPending, response.Status)
			}
			
			mockTaskRepo.AssertExpectations(t)
			mockExecutionRepo.AssertExpectations(t)
			mockExecutionService.AssertExpectations(t)
		})
	}
}

func TestTaskExecutionHandler_GetByID(t *testing.T) {
	executionID := uuid.New()
	taskID := uuid.New()
	userID := uuid.New()
	
	tests := []struct {
		name           string
		executionID    string
		mockSetup      func(*MockTaskRepository, *MockTaskExecutionRepository, *MockTaskExecutionService)
		wantStatus     int
		wantError      string
	}{
		{
			name:        "successful execution retrieval",
			executionID: executionID.String(),
			mockSetup: func(mt *MockTaskRepository, me *MockTaskExecutionRepository, ms *MockTaskExecutionService) {
				execution := &models.TaskExecution{
					ID:     executionID,
					TaskID: taskID,
					Status: models.ExecutionStatusRunning,
				}
				task := &models.Task{
					BaseModel: models.BaseModel{
						ID: taskID,
					},
					UserID: userID,
				}
				me.On("GetByID", mock.Anything, executionID).Return(execution, nil)
				mt.On("GetByID", mock.Anything, taskID).Return(task, nil)
			},
			wantStatus: http.StatusOK,
		},
		{
			name:        "invalid execution ID",
			executionID: "invalid-uuid",
			mockSetup: func(mt *MockTaskRepository, me *MockTaskExecutionRepository, ms *MockTaskExecutionService) {
				// No mock calls expected
			},
			wantStatus: http.StatusBadRequest,
			wantError:  "Invalid execution ID format",
		},
		{
			name:        "execution not found",
			executionID: executionID.String(),
			mockSetup: func(mt *MockTaskRepository, me *MockTaskExecutionRepository, ms *MockTaskExecutionService) {
				me.On("GetByID", mock.Anything, executionID).Return(nil, database.ErrExecutionNotFound)
			},
			wantStatus: http.StatusNotFound,
			wantError:  "Execution not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router, mockTaskRepo, mockExecutionRepo, mockExecutionService, handler := setupTaskExecutionHandlerTest()
			tt.mockSetup(mockTaskRepo, mockExecutionRepo, mockExecutionService)
			
			// Override the user context with known user ID
			router.Use(func(c *gin.Context) {
				user := &models.User{
					BaseModel: models.BaseModel{
						ID: userID,
					},
					Email: "test@example.com",
				}
				c.Set("user", user)
				c.Next()
			})
			
			router.GET("/executions/:id", handler.GetByID)

			req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/executions/%s", tt.executionID), nil)
			w := httptest.NewRecorder()
			
			router.ServeHTTP(w, req)
			
			assert.Equal(t, tt.wantStatus, w.Code)
			
			if tt.wantError != "" {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Contains(t, response["error"], tt.wantError)
			}
			
			mockTaskRepo.AssertExpectations(t)
			mockExecutionRepo.AssertExpectations(t)
			mockExecutionService.AssertExpectations(t)
		})
	}
}

func TestTaskExecutionHandler_ListByTaskID(t *testing.T) {
	taskID := uuid.New()
	userID := uuid.New()
	
	tests := []struct {
		name           string
		taskID         string
		query          string
		mockSetup      func(*MockTaskRepository, *MockTaskExecutionRepository, *MockTaskExecutionService)
		wantStatus     int
		wantError      string
	}{
		{
			name:   "successful execution listing",
			taskID: taskID.String(),
			query:  "?limit=10&offset=0",
			mockSetup: func(mt *MockTaskRepository, me *MockTaskExecutionRepository, ms *MockTaskExecutionService) {
				task := &models.Task{
					BaseModel: models.BaseModel{
						ID: taskID,
					},
					UserID: userID,
				}
				executions := []*models.TaskExecution{
					{
						ID:        uuid.New(),
						TaskID:    taskID,
						Status:    models.ExecutionStatusCompleted,
						CreatedAt: time.Now(),
					},
				}
				mt.On("GetByID", mock.Anything, taskID).Return(task, nil)
				me.On("GetByTaskID", mock.Anything, taskID, 10, 0).Return(executions, nil)
				me.On("CountByTaskID", mock.Anything, taskID).Return(int64(1), nil)
			},
			wantStatus: http.StatusOK,
		},
		{
			name:   "invalid task ID",
			taskID: "invalid-uuid",
			query:  "",
			mockSetup: func(mt *MockTaskRepository, me *MockTaskExecutionRepository, ms *MockTaskExecutionService) {
				// No mock calls expected
			},
			wantStatus: http.StatusBadRequest,
			wantError:  "Invalid task ID format",
		},
		{
			name:   "task not found",
			taskID: taskID.String(),
			query:  "",
			mockSetup: func(mt *MockTaskRepository, me *MockTaskExecutionRepository, ms *MockTaskExecutionService) {
				mt.On("GetByID", mock.Anything, taskID).Return(nil, database.ErrTaskNotFound)
			},
			wantStatus: http.StatusNotFound,
			wantError:  "Task not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router, mockTaskRepo, mockExecutionRepo, mockExecutionService, handler := setupTaskExecutionHandlerTest()
			tt.mockSetup(mockTaskRepo, mockExecutionRepo, mockExecutionService)
			
			// Override the user context with known user ID
			router.Use(func(c *gin.Context) {
				user := &models.User{
					BaseModel: models.BaseModel{
						ID: userID,
					},
					Email: "test@example.com",
				}
				c.Set("user", user)
				c.Next()
			})
			
			router.GET("/tasks/:task_id/executions", handler.ListByTaskID)

			req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/tasks/%s/executions%s", tt.taskID, tt.query), nil)
			w := httptest.NewRecorder()
			
			router.ServeHTTP(w, req)
			
			assert.Equal(t, tt.wantStatus, w.Code)
			
			if tt.wantError != "" {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Contains(t, response["error"], tt.wantError)
			}
			
			mockTaskRepo.AssertExpectations(t)
			mockExecutionRepo.AssertExpectations(t)
			mockExecutionService.AssertExpectations(t)
		})
	}
}

func TestTaskExecutionHandler_Cancel(t *testing.T) {
	executionID := uuid.New()
	userID := uuid.New()
	
	tests := []struct {
		name           string
		executionID    string
		mockSetup      func(*MockTaskRepository, *MockTaskExecutionRepository, *MockTaskExecutionService)
		wantStatus     int
		wantError      string
	}{
		{
			name:        "successful execution cancellation",
			executionID: executionID.String(),
			mockSetup: func(mt *MockTaskRepository, me *MockTaskExecutionRepository, ms *MockTaskExecutionService) {
				// The Cancel handler only calls the service
				ms.On("CancelExecutionAndResetTaskStatus", mock.Anything, executionID, userID).Return(nil)
			},
			wantStatus: http.StatusOK,
		},
		{
			name:        "cannot cancel completed execution",
			executionID: executionID.String(),
			mockSetup: func(mt *MockTaskRepository, me *MockTaskExecutionRepository, ms *MockTaskExecutionService) {
				// The Cancel handler only calls the service, which returns an error for completed executions
				ms.On("CancelExecutionAndResetTaskStatus", mock.Anything, executionID, userID).Return(fmt.Errorf("cannot cancel execution with status: completed"))
			},
			wantStatus: http.StatusConflict,
			wantError:  "cannot cancel execution with status:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router, mockTaskRepo, mockExecutionRepo, mockExecutionService, handler := setupTaskExecutionHandlerTest()
			tt.mockSetup(mockTaskRepo, mockExecutionRepo, mockExecutionService)
			
			// Override the user context with known user ID
			router.Use(func(c *gin.Context) {
				user := &models.User{
					BaseModel: models.BaseModel{
						ID: userID,
					},
					Email: "test@example.com",
				}
				c.Set("user", user)
				c.Next()
			})
			
			router.DELETE("/executions/:id", handler.Cancel)

			req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/executions/%s", tt.executionID), nil)
			w := httptest.NewRecorder()
			
			router.ServeHTTP(w, req)
			
			assert.Equal(t, tt.wantStatus, w.Code)
			
			if tt.wantError != "" {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Contains(t, response["error"], tt.wantError)
			}
			
			mockTaskRepo.AssertExpectations(t)
			mockExecutionRepo.AssertExpectations(t)
			mockExecutionService.AssertExpectations(t)
		})
	}
}

func TestTaskExecutionHandler_Update(t *testing.T) {
	executionID := uuid.New()
	taskID := uuid.New()
	userID := uuid.New()
	
	tests := []struct {
		name           string
		executionID    string
		request        models.UpdateTaskExecutionRequest
		mockSetup      func(*MockTaskRepository, *MockTaskExecutionRepository, *MockTaskExecutionService)
		wantStatus     int
		wantError      string
	}{
		{
			name:        "successful execution update",
			executionID: executionID.String(),
			request: models.UpdateTaskExecutionRequest{
				Status: statusPtr(models.ExecutionStatusCompleted),
			},
			mockSetup: func(mt *MockTaskRepository, me *MockTaskExecutionRepository, ms *MockTaskExecutionService) {
				execution := &models.TaskExecution{
					ID:     executionID,
					TaskID: taskID,
					Status: models.ExecutionStatusRunning,
				}
				// For terminal updates, the Update handler only calls executionRepo.GetByID, then the service
				// The service handles task validation internally, so no taskRepo.GetByID is called by the handler
				me.On("GetByID", mock.Anything, executionID).Return(execution, nil)
				// For terminal updates, it calls the service for atomic completion
				ms.On("CompleteExecutionAndFinalizeTaskStatus", mock.Anything, mock.AnythingOfType("*models.TaskExecution"), models.TaskStatusCompleted, userID).Return(nil)
			},
			wantStatus: http.StatusOK,
		},
		{
			name:        "invalid execution ID",
			executionID: "invalid-uuid",
			request: models.UpdateTaskExecutionRequest{
				Status: statusPtr(models.ExecutionStatusCompleted),
			},
			mockSetup: func(mt *MockTaskRepository, me *MockTaskExecutionRepository, ms *MockTaskExecutionService) {
				// No mock calls expected
			},
			wantStatus: http.StatusBadRequest,
			wantError:  "Invalid execution ID format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router, mockTaskRepo, mockExecutionRepo, mockExecutionService, handler := setupTaskExecutionHandlerTest()
			tt.mockSetup(mockTaskRepo, mockExecutionRepo, mockExecutionService)
			
			// Override the user context with known user ID
			router.Use(func(c *gin.Context) {
				user := &models.User{
					BaseModel: models.BaseModel{
						ID: userID,
					},
					Email: "test@example.com",
				}
				c.Set("user", user)
				c.Next()
			})
			
			router.PUT("/executions/:id", handler.Update)

			reqBody, _ := json.Marshal(tt.request)
			req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/executions/%s", tt.executionID), bytes.NewBuffer(reqBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			
			router.ServeHTTP(w, req)
			
			assert.Equal(t, tt.wantStatus, w.Code)
			
			if tt.wantError != "" {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Contains(t, response["error"], tt.wantError)
			}
			
			mockTaskRepo.AssertExpectations(t)
			mockExecutionRepo.AssertExpectations(t)
			mockExecutionService.AssertExpectations(t)
		})
	}
}
// Helper function to create ExecutionStatus pointers
func statusPtr(s models.ExecutionStatus) *models.ExecutionStatus {
	return &s
}

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

func setupTaskExecutionHandlerTest() (*gin.Engine, *MockTaskRepository, *MockTaskExecutionRepository, *TaskExecutionHandler) {
	gin.SetMode(gin.TestMode)
	
	mockTaskRepo := new(MockTaskRepository)
	mockExecutionRepo := new(MockTaskExecutionRepository)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	handler := NewTaskExecutionHandler(mockTaskRepo, mockExecutionRepo, logger)
	
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
	
	return router, mockTaskRepo, mockExecutionRepo, handler
}

func TestTaskExecutionHandler_Create(t *testing.T) {
	taskID := uuid.New()
	userID := uuid.New()
	
	tests := []struct {
		name           string
		taskID         string
		mockSetup      func(*MockTaskRepository, *MockTaskExecutionRepository)
		wantStatus     int
		wantError      string
	}{
		{
			name:   "successful execution creation",
			taskID: taskID.String(),
			mockSetup: func(mt *MockTaskRepository, me *MockTaskExecutionRepository) {
				task := &models.Task{
					BaseModel: models.BaseModel{
						ID: taskID,
					},
					UserID: userID,
					Status: models.TaskStatusPending,
				}
				mt.On("GetByID", mock.Anything, taskID).Return(task, nil)
				me.On("Create", mock.Anything, mock.AnythingOfType("*models.TaskExecution")).Return(nil)
				mt.On("UpdateStatus", mock.Anything, taskID, models.TaskStatusRunning).Return(nil)
			},
			wantStatus: http.StatusCreated,
		},
		{
			name:   "invalid task ID",
			taskID: "invalid-uuid",
			mockSetup: func(mt *MockTaskRepository, me *MockTaskExecutionRepository) {
				// No mock calls expected
			},
			wantStatus: http.StatusBadRequest,
			wantError:  "Invalid task ID format",
		},
		{
			name:   "task not found",
			taskID: taskID.String(),
			mockSetup: func(mt *MockTaskRepository, me *MockTaskExecutionRepository) {
				mt.On("GetByID", mock.Anything, taskID).Return(nil, database.ErrTaskNotFound)
			},
			wantStatus: http.StatusNotFound,
			wantError:  "Task not found",
		},
		{
			name:   "access denied - different user",
			taskID: taskID.String(),
			mockSetup: func(mt *MockTaskRepository, me *MockTaskExecutionRepository) {
				task := &models.Task{
					BaseModel: models.BaseModel{
						ID: taskID,
					},
					UserID: uuid.New(), // Different user
					Status: models.TaskStatusPending,
				}
				mt.On("GetByID", mock.Anything, taskID).Return(task, nil)
			},
			wantStatus: http.StatusForbidden,
			wantError:  "Access denied",
		},
		{
			name:   "task already running",
			taskID: taskID.String(),
			mockSetup: func(mt *MockTaskRepository, me *MockTaskExecutionRepository) {
				task := &models.Task{
					BaseModel: models.BaseModel{
						ID: taskID,
					},
					UserID: userID,
					Status: models.TaskStatusRunning, // Already running
				}
				mt.On("GetByID", mock.Anything, taskID).Return(task, nil)
			},
			wantStatus: http.StatusConflict,
			wantError:  "Task is already running",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router, mockTaskRepo, mockExecutionRepo, handler := setupTaskExecutionHandlerTest()
			tt.mockSetup(mockTaskRepo, mockExecutionRepo)
			
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
		mockSetup      func(*MockTaskRepository, *MockTaskExecutionRepository)
		wantStatus     int
		wantError      string
	}{
		{
			name:        "successful execution retrieval",
			executionID: executionID.String(),
			mockSetup: func(mt *MockTaskRepository, me *MockTaskExecutionRepository) {
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
			mockSetup: func(mt *MockTaskRepository, me *MockTaskExecutionRepository) {
				// No mock calls expected
			},
			wantStatus: http.StatusBadRequest,
			wantError:  "Invalid execution ID format",
		},
		{
			name:        "execution not found",
			executionID: executionID.String(),
			mockSetup: func(mt *MockTaskRepository, me *MockTaskExecutionRepository) {
				me.On("GetByID", mock.Anything, executionID).Return(nil, database.ErrExecutionNotFound)
			},
			wantStatus: http.StatusNotFound,
			wantError:  "Execution not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router, mockTaskRepo, mockExecutionRepo, handler := setupTaskExecutionHandlerTest()
			tt.mockSetup(mockTaskRepo, mockExecutionRepo)
			
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
		mockSetup      func(*MockTaskRepository, *MockTaskExecutionRepository)
		wantStatus     int
		wantError      string
	}{
		{
			name:   "successful execution listing",
			taskID: taskID.String(),
			query:  "?limit=10&offset=0",
			mockSetup: func(mt *MockTaskRepository, me *MockTaskExecutionRepository) {
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
			mockSetup: func(mt *MockTaskRepository, me *MockTaskExecutionRepository) {
				// No mock calls expected
			},
			wantStatus: http.StatusBadRequest,
			wantError:  "Invalid task ID format",
		},
		{
			name:   "task not found",
			taskID: taskID.String(),
			query:  "",
			mockSetup: func(mt *MockTaskRepository, me *MockTaskExecutionRepository) {
				mt.On("GetByID", mock.Anything, taskID).Return(nil, database.ErrTaskNotFound)
			},
			wantStatus: http.StatusNotFound,
			wantError:  "Task not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router, mockTaskRepo, mockExecutionRepo, handler := setupTaskExecutionHandlerTest()
			tt.mockSetup(mockTaskRepo, mockExecutionRepo)
			
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
		})
	}
}

func TestTaskExecutionHandler_Cancel(t *testing.T) {
	executionID := uuid.New()
	taskID := uuid.New()
	userID := uuid.New()
	
	tests := []struct {
		name           string
		executionID    string
		mockSetup      func(*MockTaskRepository, *MockTaskExecutionRepository)
		wantStatus     int
		wantError      string
	}{
		{
			name:        "successful execution cancellation",
			executionID: executionID.String(),
			mockSetup: func(mt *MockTaskRepository, me *MockTaskExecutionRepository) {
				execution := &models.TaskExecution{
					ID:     executionID,
					TaskID: taskID,
					Status: models.ExecutionStatusRunning, // Can be cancelled
				}
				task := &models.Task{
					BaseModel: models.BaseModel{
						ID: taskID,
					},
					UserID: userID,
				}
				me.On("GetByID", mock.Anything, executionID).Return(execution, nil)
				mt.On("GetByID", mock.Anything, taskID).Return(task, nil)
				me.On("UpdateStatus", mock.Anything, executionID, models.ExecutionStatusCancelled).Return(nil)
				mt.On("UpdateStatus", mock.Anything, taskID, models.TaskStatusPending).Return(nil)
			},
			wantStatus: http.StatusOK,
		},
		{
			name:        "cannot cancel completed execution",
			executionID: executionID.String(),
			mockSetup: func(mt *MockTaskRepository, me *MockTaskExecutionRepository) {
				execution := &models.TaskExecution{
					ID:     executionID,
					TaskID: taskID,
					Status: models.ExecutionStatusCompleted, // Terminal state
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
			wantStatus: http.StatusConflict,
			wantError:  "Cannot cancel completed execution",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router, mockTaskRepo, mockExecutionRepo, handler := setupTaskExecutionHandlerTest()
			tt.mockSetup(mockTaskRepo, mockExecutionRepo)
			
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
		mockSetup      func(*MockTaskRepository, *MockTaskExecutionRepository)
		wantStatus     int
		wantError      string
	}{
		{
			name:        "successful execution update",
			executionID: executionID.String(),
			request: models.UpdateTaskExecutionRequest{
				Status: statusPtr(models.ExecutionStatusCompleted),
			},
			mockSetup: func(mt *MockTaskRepository, me *MockTaskExecutionRepository) {
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
				me.On("Update", mock.Anything, mock.AnythingOfType("*models.TaskExecution")).Return(nil)
				mt.On("UpdateStatus", mock.Anything, taskID, models.TaskStatusCompleted).Return(nil)
			},
			wantStatus: http.StatusOK,
		},
		{
			name:        "invalid execution ID",
			executionID: "invalid-uuid",
			request: models.UpdateTaskExecutionRequest{
				Status: statusPtr(models.ExecutionStatusCompleted),
			},
			mockSetup: func(mt *MockTaskRepository, me *MockTaskExecutionRepository) {
				// No mock calls expected
			},
			wantStatus: http.StatusBadRequest,
			wantError:  "Invalid execution ID format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router, mockTaskRepo, mockExecutionRepo, handler := setupTaskExecutionHandlerTest()
			tt.mockSetup(mockTaskRepo, mockExecutionRepo)
			
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
		})
	}
}
// Helper function to create ExecutionStatus pointers
func statusPtr(s models.ExecutionStatus) *models.ExecutionStatus {
	return &s
}

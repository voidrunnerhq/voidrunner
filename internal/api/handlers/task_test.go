package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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

// MockTaskRepository is a mock implementation of TaskRepository
type MockTaskRepository struct {
	mock.Mock
}

func (m *MockTaskRepository) Create(ctx context.Context, task *models.Task) error {
	args := m.Called(ctx, task)
	return args.Error(0)
}

func (m *MockTaskRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Task, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Task), args.Error(1)
}

func (m *MockTaskRepository) GetByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*models.Task, error) {
	args := m.Called(ctx, userID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Task), args.Error(1)
}

func (m *MockTaskRepository) GetByStatus(ctx context.Context, status models.TaskStatus, limit, offset int) ([]*models.Task, error) {
	args := m.Called(ctx, status, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Task), args.Error(1)
}

func (m *MockTaskRepository) Update(ctx context.Context, task *models.Task) error {
	args := m.Called(ctx, task)
	return args.Error(0)
}

func (m *MockTaskRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status models.TaskStatus) error {
	args := m.Called(ctx, id, status)
	return args.Error(0)
}

func (m *MockTaskRepository) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockTaskRepository) List(ctx context.Context, limit, offset int) ([]*models.Task, error) {
	args := m.Called(ctx, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Task), args.Error(1)
}

func (m *MockTaskRepository) Count(ctx context.Context) (int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockTaskRepository) CountByUserID(ctx context.Context, userID uuid.UUID) (int64, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockTaskRepository) CountByStatus(ctx context.Context, status models.TaskStatus) (int64, error) {
	args := m.Called(ctx, status)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockTaskRepository) SearchByMetadata(ctx context.Context, query string, limit, offset int) ([]*models.Task, error) {
	args := m.Called(ctx, query, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Task), args.Error(1)
}

// Cursor-based pagination methods
func (m *MockTaskRepository) GetByUserIDCursor(ctx context.Context, userID uuid.UUID, req database.CursorPaginationRequest) ([]*models.Task, database.CursorPaginationResponse, error) {
	args := m.Called(ctx, userID, req)
	if args.Get(0) == nil {
		return nil, database.CursorPaginationResponse{}, args.Error(2)
	}
	return args.Get(0).([]*models.Task), args.Get(1).(database.CursorPaginationResponse), args.Error(2)
}

func (m *MockTaskRepository) GetByStatusCursor(ctx context.Context, status models.TaskStatus, req database.CursorPaginationRequest) ([]*models.Task, database.CursorPaginationResponse, error) {
	args := m.Called(ctx, status, req)
	if args.Get(0) == nil {
		return nil, database.CursorPaginationResponse{}, args.Error(2)
	}
	return args.Get(0).([]*models.Task), args.Get(1).(database.CursorPaginationResponse), args.Error(2)
}

func (m *MockTaskRepository) ListCursor(ctx context.Context, req database.CursorPaginationRequest) ([]*models.Task, database.CursorPaginationResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, database.CursorPaginationResponse{}, args.Error(2)
	}
	return args.Get(0).([]*models.Task), args.Get(1).(database.CursorPaginationResponse), args.Error(2)
}

// Optimized bulk operations
func (m *MockTaskRepository) GetTasksWithExecutionCount(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*models.Task, error) {
	args := m.Called(ctx, userID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Task), args.Error(1)
}

func (m *MockTaskRepository) GetTasksWithLatestExecution(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*models.Task, error) {
	args := m.Called(ctx, userID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Task), args.Error(1)
}

func setupTaskHandlerTest() (*gin.Engine, *MockTaskRepository, *TaskHandler) {
	gin.SetMode(gin.TestMode)
	
	mockRepo := new(MockTaskRepository)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	handler := NewTaskHandler(mockRepo, logger)
	
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
	
	return router, mockRepo, handler
}

func TestTaskHandler_Create(t *testing.T) {
	tests := []struct {
		name       string
		request    models.CreateTaskRequest
		mockSetup  func(*MockTaskRepository)
		wantStatus int
		wantError  string
	}{
		{
			name: "successful task creation",
			request: models.CreateTaskRequest{
				Name:          "Test Task",
				ScriptContent: "print('hello world')",
				ScriptType:    models.ScriptTypePython,
			},
			mockSetup: func(m *MockTaskRepository) {
				m.On("Create", mock.Anything, mock.AnythingOfType("*models.Task")).Return(nil)
			},
			wantStatus: http.StatusCreated,
		},
		{
			name: "invalid request - empty name",
			request: models.CreateTaskRequest{
				Name:          "",
				ScriptContent: "print('hello world')",
				ScriptType:    models.ScriptTypePython,
			},
			mockSetup:  func(m *MockTaskRepository) {},
			wantStatus: http.StatusBadRequest,
			wantError:  "task name is required",
		},
		{
			name: "invalid request - invalid script type",
			request: models.CreateTaskRequest{
				Name:          "Test Task",
				ScriptContent: "print('hello world')",
				ScriptType:    "invalid",
			},
			mockSetup:  func(m *MockTaskRepository) {},
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid script type",
		},
		{
			name: "repository error",
			request: models.CreateTaskRequest{
				Name:          "Test Task",
				ScriptContent: "print('hello world')",
				ScriptType:    models.ScriptTypePython,
			},
			mockSetup: func(m *MockTaskRepository) {
				m.On("Create", mock.Anything, mock.AnythingOfType("*models.Task")).Return(errors.New("database error"))
			},
			wantStatus: http.StatusInternalServerError,
			wantError:  "Failed to create task",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router, mockRepo, handler := setupTaskHandlerTest()
			tt.mockSetup(mockRepo)
			
			router.POST("/tasks", handler.Create)

			reqBody, _ := json.Marshal(tt.request)
			req := httptest.NewRequest(http.MethodPost, "/tasks", bytes.NewBuffer(reqBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			
			router.ServeHTTP(w, req)
			
			assert.Equal(t, tt.wantStatus, w.Code)
			
			if tt.wantError != "" {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Contains(t, response["error"], tt.wantError)
			} else if tt.wantStatus == http.StatusCreated {
				var response models.TaskResponse
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Equal(t, tt.request.Name, response.Name)
				assert.Equal(t, tt.request.ScriptContent, response.ScriptContent)
				assert.Equal(t, tt.request.ScriptType, response.ScriptType)
			}
			
			mockRepo.AssertExpectations(t)
		})
	}
}

func TestTaskHandler_GetByID(t *testing.T) {
	taskID := uuid.New()
	userID := uuid.New()
	
	tests := []struct {
		name       string
		taskID     string
		mockSetup  func(*MockTaskRepository)
		wantStatus int
		wantError  string
	}{
		{
			name:   "successful task retrieval",
			taskID: taskID.String(),
			mockSetup: func(m *MockTaskRepository) {
				task := &models.Task{
					BaseModel: models.BaseModel{
						ID:        taskID,
						CreatedAt: time.Now(),
						UpdatedAt: time.Now(),
					},
					UserID:         userID,
					Name:           "Test Task",
					ScriptContent:  "print('hello world')",
					ScriptType:     models.ScriptTypePython,
					Status:         models.TaskStatusPending,
					Priority:       1,
					TimeoutSeconds: 30,
				}
				m.On("GetByID", mock.Anything, taskID).Return(task, nil)
			},
			wantStatus: http.StatusOK,
		},
		{
			name:   "invalid task ID",
			taskID: "invalid-uuid",
			mockSetup: func(m *MockTaskRepository) {
				// No mock calls expected
			},
			wantStatus: http.StatusBadRequest,
			wantError:  "Invalid task ID format",
		},
		{
			name:   "task not found",
			taskID: taskID.String(),
			mockSetup: func(m *MockTaskRepository) {
				m.On("GetByID", mock.Anything, taskID).Return(nil, database.ErrTaskNotFound)
			},
			wantStatus: http.StatusNotFound,
			wantError:  "Task not found",
		},
		{
			name:   "access denied - different user",
			taskID: taskID.String(),
			mockSetup: func(m *MockTaskRepository) {
				task := &models.Task{
					BaseModel: models.BaseModel{
						ID:        taskID,
						CreatedAt: time.Now(),
						UpdatedAt: time.Now(),
					},
					UserID:         uuid.New(), // Different user
					Name:           "Test Task",
					ScriptContent:  "print('hello world')",
					ScriptType:     models.ScriptTypePython,
					Status:         models.TaskStatusPending,
					Priority:       1,
					TimeoutSeconds: 30,
				}
				m.On("GetByID", mock.Anything, taskID).Return(task, nil)
			},
			wantStatus: http.StatusForbidden,
			wantError:  "Access denied",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router, mockRepo, handler := setupTaskHandlerTest()
			tt.mockSetup(mockRepo)
			
			// Override the user context with known user ID for access tests
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
			
			router.GET("/tasks/:id", handler.GetByID)

			req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/tasks/%s", tt.taskID), nil)
			w := httptest.NewRecorder()
			
			router.ServeHTTP(w, req)
			
			assert.Equal(t, tt.wantStatus, w.Code)
			
			if tt.wantError != "" {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Contains(t, response["error"], tt.wantError)
			}
			
			mockRepo.AssertExpectations(t)
		})
	}
}

func TestTaskHandler_List(t *testing.T) {
	userID := uuid.New()
	
	tests := []struct {
		name       string
		query      string
		mockSetup  func(*MockTaskRepository)
		wantStatus int
		wantError  string
	}{
		{
			name:  "successful task listing",
			query: "?limit=10&offset=0",
			mockSetup: func(m *MockTaskRepository) {
				tasks := []*models.Task{
					{
						BaseModel: models.BaseModel{
							ID:        uuid.New(),
							CreatedAt: time.Now(),
							UpdatedAt: time.Now(),
						},
						UserID:         userID,
						Name:           "Test Task 1",
						ScriptContent:  "print('hello world')",
						ScriptType:     models.ScriptTypePython,
						Status:         models.TaskStatusPending,
						Priority:       1,
						TimeoutSeconds: 30,
					},
				}
				m.On("GetByUserID", mock.Anything, userID, 10, 0).Return(tasks, nil)
				m.On("CountByUserID", mock.Anything, userID).Return(int64(1), nil)
			},
			wantStatus: http.StatusOK,
		},
		{
			name:  "invalid pagination - negative offset",
			query: "?limit=10&offset=-1",
			mockSetup: func(m *MockTaskRepository) {
				// No mock calls expected
			},
			wantStatus: http.StatusBadRequest,
			wantError:  "offset must be non-negative",
		},
		{
			name:  "invalid pagination - limit too high",
			query: "?limit=200&offset=0",
			mockSetup: func(m *MockTaskRepository) {
				// No mock calls expected
			},
			wantStatus: http.StatusBadRequest,
			wantError:  "limit must be between 1 and 100",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router, mockRepo, handler := setupTaskHandlerTest()
			tt.mockSetup(mockRepo)
			
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
			
			router.GET("/tasks", handler.List)

			req := httptest.NewRequest(http.MethodGet, "/tasks"+tt.query, nil)
			w := httptest.NewRecorder()
			
			router.ServeHTTP(w, req)
			
			assert.Equal(t, tt.wantStatus, w.Code)
			
			if tt.wantError != "" {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Contains(t, response["error"], tt.wantError)
			}
			
			mockRepo.AssertExpectations(t)
		})
	}
}

func TestTaskHandler_Update(t *testing.T) {
	taskID := uuid.New()
	userID := uuid.New()
	
	tests := []struct {
		name       string
		taskID     string
		request    models.UpdateTaskRequest
		mockSetup  func(*MockTaskRepository)
		wantStatus int
		wantError  string
	}{
		{
			name:   "successful task update",
			taskID: taskID.String(),
			request: models.UpdateTaskRequest{
				Name: stringPtr("Updated Task"),
			},
			mockSetup: func(m *MockTaskRepository) {
				task := &models.Task{
					BaseModel: models.BaseModel{
						ID:        taskID,
						CreatedAt: time.Now(),
						UpdatedAt: time.Now(),
					},
					UserID:         userID,
					Name:           "Test Task",
					ScriptContent:  "print('hello world')",
					ScriptType:     models.ScriptTypePython,
					Status:         models.TaskStatusPending,
					Priority:       1,
					TimeoutSeconds: 30,
				}
				m.On("GetByID", mock.Anything, taskID).Return(task, nil)
				m.On("Update", mock.Anything, mock.AnythingOfType("*models.Task")).Return(nil)
			},
			wantStatus: http.StatusOK,
		},
		{
			name:   "cannot update running task",
			taskID: taskID.String(),
			request: models.UpdateTaskRequest{
				Name: stringPtr("Updated Task"),
			},
			mockSetup: func(m *MockTaskRepository) {
				task := &models.Task{
					BaseModel: models.BaseModel{
						ID:        taskID,
						CreatedAt: time.Now(),
						UpdatedAt: time.Now(),
					},
					UserID:         userID,
					Name:           "Test Task",
					ScriptContent:  "print('hello world')",
					ScriptType:     models.ScriptTypePython,
					Status:         models.TaskStatusRunning, // Running task
					Priority:       1,
					TimeoutSeconds: 30,
				}
				m.On("GetByID", mock.Anything, taskID).Return(task, nil)
			},
			wantStatus: http.StatusConflict,
			wantError:  "Cannot update running task",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router, mockRepo, handler := setupTaskHandlerTest()
			tt.mockSetup(mockRepo)
			
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
			
			router.PUT("/tasks/:id", handler.Update)

			reqBody, _ := json.Marshal(tt.request)
			req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/tasks/%s", tt.taskID), bytes.NewBuffer(reqBody))
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
			
			mockRepo.AssertExpectations(t)
		})
	}
}

func TestTaskHandler_Delete(t *testing.T) {
	taskID := uuid.New()
	userID := uuid.New()
	
	tests := []struct {
		name       string
		taskID     string
		mockSetup  func(*MockTaskRepository)
		wantStatus int
		wantError  string
	}{
		{
			name:   "successful task deletion",
			taskID: taskID.String(),
			mockSetup: func(m *MockTaskRepository) {
				task := &models.Task{
					BaseModel: models.BaseModel{
						ID:        taskID,
						CreatedAt: time.Now(),
						UpdatedAt: time.Now(),
					},
					UserID:         userID,
					Name:           "Test Task",
					ScriptContent:  "print('hello world')",
					ScriptType:     models.ScriptTypePython,
					Status:         models.TaskStatusPending,
					Priority:       1,
					TimeoutSeconds: 30,
				}
				m.On("GetByID", mock.Anything, taskID).Return(task, nil)
				m.On("Delete", mock.Anything, taskID).Return(nil)
			},
			wantStatus: http.StatusOK,
		},
		{
			name:   "cannot delete running task",
			taskID: taskID.String(),
			mockSetup: func(m *MockTaskRepository) {
				task := &models.Task{
					BaseModel: models.BaseModel{
						ID:        taskID,
						CreatedAt: time.Now(),
						UpdatedAt: time.Now(),
					},
					UserID:         userID,
					Name:           "Test Task",
					ScriptContent:  "print('hello world')",
					ScriptType:     models.ScriptTypePython,
					Status:         models.TaskStatusRunning, // Running task
					Priority:       1,
					TimeoutSeconds: 30,
				}
				m.On("GetByID", mock.Anything, taskID).Return(task, nil)
			},
			wantStatus: http.StatusConflict,
			wantError:  "Cannot delete running task",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router, mockRepo, handler := setupTaskHandlerTest()
			tt.mockSetup(mockRepo)
			
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
			
			router.DELETE("/tasks/:id", handler.Delete)

			req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/tasks/%s", tt.taskID), nil)
			w := httptest.NewRecorder()
			
			router.ServeHTTP(w, req)
			
			assert.Equal(t, tt.wantStatus, w.Code)
			
			if tt.wantError != "" {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Contains(t, response["error"], tt.wantError)
			}
			
			mockRepo.AssertExpectations(t)
		})
	}
}

// Helper function to create string pointers
func stringPtr(s string) *string {
	return &s
}
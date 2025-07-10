package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/voidrunnerhq/voidrunner/internal/models"
)

func TestTaskRepository_Create(t *testing.T) {
	tests := []struct {
		name      string
		task      *models.Task
		mockSetup func(*MockQuerier)
		wantError bool
		errorMsg  string
	}{
		{
			name: "successful task creation",
			task: &models.Task{
				UserID:         uuid.New(),
				Name:           "Test Task",
				ScriptContent:  "print('hello world')",
				ScriptType:     models.ScriptTypePython,
				Status:         models.TaskStatusPending,
				Priority:       1,
				TimeoutSeconds: 30,
				Metadata:       models.JSONB{"key": "value"},
			},
			mockSetup: func(mq *MockQuerier) {
				row := &MockRow{}
				mq.On("QueryRow", mock.Anything, mock.AnythingOfType("string"), mock.Anything).Return(row)
			},
			wantError: false,
		},
		{
			name: "nil task",
			task: nil,
			mockSetup: func(mq *MockQuerier) {
				// No database calls expected for validation errors
			},
			wantError: true,
			errorMsg:  "task cannot be nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockQuerier := new(MockQuerier)
			repo := &taskRepository{
				querier:       mockQuerier,
				cursorEncoder: NewCursorEncoder(),
			}

			tt.mockSetup(mockQuerier)

			err := repo.Create(context.Background(), tt.task)

			if tt.wantError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}

			mockQuerier.AssertExpectations(t)
		})
	}
}

func TestTaskRepository_GetByID(t *testing.T) {
	tests := []struct {
		name      string
		taskID    uuid.UUID
		mockSetup func(*MockQuerier)
		wantError bool
		errorMsg  string
	}{
		{
			name:   "successful get by ID",
			taskID: uuid.New(),
			mockSetup: func(mq *MockQuerier) {
				taskID := uuid.New()
				row := &MockRow{
					data: []interface{}{
						taskID, uuid.New(), "Test Task", "description",
						"print('test')", "python", "pending", 1, 30,
						models.JSONB{}, time.Now(), time.Now(),
					},
				}
				mq.On("QueryRow", mock.Anything, mock.AnythingOfType("string"), mock.Anything).Return(row)
			},
			wantError: false,
		},
		{
			name:   "task not found",
			taskID: uuid.New(),
			mockSetup: func(mq *MockQuerier) {
				row := &MockRow{err: pgx.ErrNoRows}
				mq.On("QueryRow", mock.Anything, mock.AnythingOfType("string"), mock.Anything).Return(row)
			},
			wantError: true,
			errorMsg:  "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockQuerier := new(MockQuerier)
			repo := &taskRepository{
				querier:       mockQuerier,
				cursorEncoder: NewCursorEncoder(),
			}

			tt.mockSetup(mockQuerier)

			task, err := repo.GetByID(context.Background(), tt.taskID)

			if tt.wantError {
				assert.Error(t, err)
				assert.Nil(t, task)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, task)
			}

			mockQuerier.AssertExpectations(t)
		})
	}
}

func TestTaskRepository_GetByUserID(t *testing.T) {
	tests := []struct {
		name      string
		userID    uuid.UUID
		limit     int
		offset    int
		mockSetup func(*MockQuerier)
		wantError bool
	}{
		{
			name:   "successful get by user ID",
			userID: uuid.New(),
			limit:  10,
			offset: 0,
			mockSetup: func(mq *MockQuerier) {
				mockRows := &MockRows{
					rows: [][]interface{}{
						{uuid.New(), uuid.New(), "Test Task", "description", "print('test')", "python", "pending", 1, 30, models.JSONB{}, time.Now(), time.Now()},
					},
				}
				mq.On("Query", mock.Anything, mock.AnythingOfType("string"), mock.Anything).Return(mockRows, nil)
			},
			wantError: false,
		},
		{
			name:   "default limit for zero limit",
			userID: uuid.New(),
			limit:  0,
			offset: 0,
			mockSetup: func(mq *MockQuerier) {
				mockRows := &MockRows{
					rows: [][]interface{}{
						{uuid.New(), uuid.New(), "Test Task 2", "description", "print('test2')", "python", "pending", 1, 30, models.JSONB{}, time.Now(), time.Now()},
					},
				}
				mq.On("Query", mock.Anything, mock.AnythingOfType("string"), mock.Anything).Return(mockRows, nil)
			},
			wantError: false,
		},
		{
			name:   "database error",
			userID: uuid.New(),
			limit:  10,
			offset: 0,
			mockSetup: func(mq *MockQuerier) {
				mq.On("Query", mock.Anything, mock.AnythingOfType("string"), mock.Anything).Return(nil, errors.New("database error"))
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockQuerier := new(MockQuerier)
			repo := &taskRepository{
				querier:       mockQuerier,
				cursorEncoder: NewCursorEncoder(),
			}

			tt.mockSetup(mockQuerier)

			tasks, err := repo.GetByUserID(context.Background(), tt.userID, tt.limit, tt.offset)

			if tt.wantError {
				assert.Error(t, err)
				assert.Nil(t, tasks)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, tasks)
			}

			mockQuerier.AssertExpectations(t)
		})
	}
}

func TestTaskRepository_GetByStatus(t *testing.T) {
	tests := []struct {
		name      string
		status    models.TaskStatus
		limit     int
		offset    int
		mockSetup func(*MockQuerier)
		wantError bool
	}{
		{
			name:   "successful get by status",
			status: models.TaskStatusPending,
			limit:  10,
			offset: 0,
			mockSetup: func(mq *MockQuerier) {
				mockRows := &MockRows{
					rows: [][]interface{}{
						{uuid.New(), uuid.New(), "Pending Task", "description", "print('pending')", "python", "pending", 1, 30, models.JSONB{}, time.Now(), time.Now()},
					},
				}
				mq.On("Query", mock.Anything, mock.AnythingOfType("string"), mock.Anything).Return(mockRows, nil)
			},
			wantError: false,
		},
		{
			name:   "database error",
			status: models.TaskStatusCompleted,
			limit:  5,
			offset: 0,
			mockSetup: func(mq *MockQuerier) {
				mq.On("Query", mock.Anything, mock.AnythingOfType("string"), mock.Anything).Return(nil, errors.New("database error"))
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockQuerier := new(MockQuerier)
			repo := &taskRepository{
				querier:       mockQuerier,
				cursorEncoder: NewCursorEncoder(),
			}

			tt.mockSetup(mockQuerier)

			tasks, err := repo.GetByStatus(context.Background(), tt.status, tt.limit, tt.offset)

			if tt.wantError {
				assert.Error(t, err)
				assert.Nil(t, tasks)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, tasks)
			}

			mockQuerier.AssertExpectations(t)
		})
	}
}

func TestTaskRepository_Update(t *testing.T) {
	tests := []struct {
		name      string
		task      *models.Task
		mockSetup func(*MockQuerier)
		wantError bool
		errorMsg  string
	}{
		{
			name: "successful update",
			task: &models.Task{
				BaseModel: models.BaseModel{
					ID: uuid.New(),
				},
				UserID:         uuid.New(),
				Name:           "Updated Task",
				ScriptContent:  "print('updated')",
				ScriptType:     models.ScriptTypePython,
				Status:         models.TaskStatusRunning,
				Priority:       2,
				TimeoutSeconds: 60,
			},
			mockSetup: func(mq *MockQuerier) {
				row := &MockRow{}
				mq.On("QueryRow", mock.Anything, mock.AnythingOfType("string"), mock.Anything).Return(row)
			},
			wantError: false,
		},
		{
			name: "nil task",
			task: nil,
			mockSetup: func(mq *MockQuerier) {
				// No database calls expected for validation errors
			},
			wantError: true,
			errorMsg:  "task cannot be nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockQuerier := new(MockQuerier)
			repo := &taskRepository{
				querier:       mockQuerier,
				cursorEncoder: NewCursorEncoder(),
			}

			tt.mockSetup(mockQuerier)

			err := repo.Update(context.Background(), tt.task)

			if tt.wantError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}

			mockQuerier.AssertExpectations(t)
		})
	}
}

func TestTaskRepository_UpdateStatus(t *testing.T) {
	tests := []struct {
		name      string
		taskID    uuid.UUID
		status    models.TaskStatus
		mockSetup func(*MockQuerier)
		wantError bool
		errorMsg  string
	}{
		{
			name:   "successful status update",
			taskID: uuid.New(),
			status: models.TaskStatusRunning,
			mockSetup: func(mq *MockQuerier) {
				cmdTag := pgconn.NewCommandTag("UPDATE 1")
				mq.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).Return(cmdTag, nil)
			},
			wantError: false,
		},
		{
			name:   "task not found",
			taskID: uuid.New(),
			status: models.TaskStatusCompleted,
			mockSetup: func(mq *MockQuerier) {
				cmdTag := pgconn.NewCommandTag("UPDATE 0")
				mq.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).Return(cmdTag, nil)
			},
			wantError: true,
			errorMsg:  "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockQuerier := new(MockQuerier)
			repo := &taskRepository{
				querier:       mockQuerier,
				cursorEncoder: NewCursorEncoder(),
			}

			tt.mockSetup(mockQuerier)

			err := repo.UpdateStatus(context.Background(), tt.taskID, tt.status)

			if tt.wantError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}

			mockQuerier.AssertExpectations(t)
		})
	}
}

func TestTaskRepository_SearchByMetadata(t *testing.T) {
	tests := []struct {
		name      string
		query     string
		limit     int
		offset    int
		mockSetup func(*MockQuerier)
		wantError bool
	}{
		{
			name:   "successful metadata search",
			query:  `{"environment": "production"}`,
			limit:  10,
			offset: 0,
			mockSetup: func(mq *MockQuerier) {
				mockRows := &MockRows{
					rows: [][]interface{}{
						{uuid.New(), uuid.New(), "Prod Task", "description", "print('prod')", "python", "pending", 1, 30, models.JSONB{"environment": "production"}, time.Now(), time.Now()},
					},
				}
				mq.On("Query", mock.Anything, mock.AnythingOfType("string"), mock.Anything).Return(mockRows, nil)
			},
			wantError: false,
		},
		{
			name:   "invalid JSON query",
			query:  `invalid json`,
			limit:  10,
			offset: 0,
			mockSetup: func(mq *MockQuerier) {
				pgErr := &pgconn.PgError{Code: "22P02"} // invalid_text_representation
				mq.On("Query", mock.Anything, mock.AnythingOfType("string"), mock.Anything).Return(nil, pgErr)
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockQuerier := new(MockQuerier)
			repo := &taskRepository{
				querier:       mockQuerier,
				cursorEncoder: NewCursorEncoder(),
			}

			tt.mockSetup(mockQuerier)

			tasks, err := repo.SearchByMetadata(context.Background(), tt.query, tt.limit, tt.offset)

			if tt.wantError {
				assert.Error(t, err)
				assert.Nil(t, tasks)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, tasks)
			}

			mockQuerier.AssertExpectations(t)
		})
	}
}

func TestTaskRepository_Delete(t *testing.T) {
	tests := []struct {
		name      string
		taskID    uuid.UUID
		mockSetup func(*MockQuerier)
		wantError bool
		errorMsg  string
	}{
		{
			name:   "successful delete",
			taskID: uuid.New(),
			mockSetup: func(mq *MockQuerier) {
				cmdTag := pgconn.NewCommandTag("DELETE 1")
				mq.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).Return(cmdTag, nil)
			},
			wantError: false,
		},
		{
			name:   "task not found",
			taskID: uuid.New(),
			mockSetup: func(mq *MockQuerier) {
				cmdTag := pgconn.NewCommandTag("DELETE 0")
				mq.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).Return(cmdTag, nil)
			},
			wantError: true,
			errorMsg:  "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockQuerier := new(MockQuerier)
			repo := &taskRepository{
				querier:       mockQuerier,
				cursorEncoder: NewCursorEncoder(),
			}

			tt.mockSetup(mockQuerier)

			err := repo.Delete(context.Background(), tt.taskID)

			if tt.wantError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}

			mockQuerier.AssertExpectations(t)
		})
	}
}

func TestTaskRepository_Count(t *testing.T) {
	t.Run("successful count", func(t *testing.T) {
		mockQuerier := new(MockQuerier)
		repo := &taskRepository{
			querier:       mockQuerier,
			cursorEncoder: NewCursorEncoder(),
		}

		row := &MockRow{data: []interface{}{int64(42)}}
		mockQuerier.On("QueryRow", mock.Anything, mock.AnythingOfType("string"), mock.Anything).Return(row)

		count, err := repo.Count(context.Background())
		assert.NoError(t, err)
		assert.Equal(t, int64(42), count)

		mockQuerier.AssertExpectations(t)
	})
}

func TestTaskRepository_CountByUserID(t *testing.T) {
	t.Run("successful count by user ID", func(t *testing.T) {
		mockQuerier := new(MockQuerier)
		repo := &taskRepository{
			querier:       mockQuerier,
			cursorEncoder: NewCursorEncoder(),
		}

		row := &MockRow{data: []interface{}{int64(5)}}
		mockQuerier.On("QueryRow", mock.Anything, mock.AnythingOfType("string"), mock.Anything).Return(row)

		userID := uuid.New()
		count, err := repo.CountByUserID(context.Background(), userID)
		assert.NoError(t, err)
		assert.Equal(t, int64(5), count)

		mockQuerier.AssertExpectations(t)
	})
}

func TestTaskRepository_CountByStatus(t *testing.T) {
	t.Run("successful count by status", func(t *testing.T) {
		mockQuerier := new(MockQuerier)
		repo := &taskRepository{
			querier:       mockQuerier,
			cursorEncoder: NewCursorEncoder(),
		}

		row := &MockRow{data: []interface{}{int64(10)}}
		mockQuerier.On("QueryRow", mock.Anything, mock.AnythingOfType("string"), mock.Anything).Return(row)

		count, err := repo.CountByStatus(context.Background(), models.TaskStatusPending)
		assert.NoError(t, err)
		assert.Equal(t, int64(10), count)

		mockQuerier.AssertExpectations(t)
	})
}

// MockQuerier is a mock implementation of the Querier interface
type MockQuerier struct {
	mock.Mock
}

func (m *MockQuerier) Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
	arguments := m.Called(ctx, sql, args)
	if arguments.Get(0) == nil {
		return nil, arguments.Error(1)
	}
	return arguments.Get(0).(pgx.Rows), arguments.Error(1)
}

func (m *MockQuerier) QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row {
	arguments := m.Called(ctx, sql, args)
	return arguments.Get(0).(pgx.Row)
}

func (m *MockQuerier) Exec(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error) {
	arguments := m.Called(ctx, sql, args)
	if arguments.Get(0) == nil {
		return pgconn.NewCommandTag(""), arguments.Error(1)
	}
	return arguments.Get(0).(pgconn.CommandTag), arguments.Error(1)
}

// MockRow is a mock implementation of pgx.Row
type MockRow struct {
	mock.Mock
	err  error
	data []interface{}
}

// MockRows is a mock implementation of pgx.Rows
type MockRows struct {
	mock.Mock
	rows [][]interface{}
	pos  int
	err  error
}

func (m *MockRows) Next() bool {
	if m.err != nil {
		return false
	}
	return m.pos < len(m.rows)
}

func (m *MockRows) Scan(dest ...interface{}) error {
	if m.err != nil {
		return m.err
	}
	if m.pos >= len(m.rows) {
		return pgx.ErrNoRows
	}

	row := m.rows[m.pos]
	m.pos++

	// Copy data to destination pointers
	for i := range dest {
		if i < len(row) {
			switch v := dest[i].(type) {
			case *uuid.UUID:
				if val, ok := row[i].(uuid.UUID); ok {
					*v = val
				}
			case *string:
				if val, ok := row[i].(string); ok {
					*v = val
				}
			case *int:
				if val, ok := row[i].(int); ok {
					*v = val
				}
			case *time.Time:
				if val, ok := row[i].(time.Time); ok {
					*v = val
				}
			}
		}
	}
	return nil
}

func (m *MockRows) Close() {
	// Mock implementation - no cleanup needed
}

func (m *MockRows) Err() error {
	return m.err
}

// Additional methods to satisfy pgx.Rows interface
func (m *MockRows) CommandTag() pgconn.CommandTag {
	return pgconn.NewCommandTag("SELECT 0")
}

func (m *MockRows) FieldDescriptions() []pgconn.FieldDescription {
	return nil
}

func (m *MockRows) Values() ([]interface{}, error) {
	if m.pos > 0 && m.pos <= len(m.rows) {
		return m.rows[m.pos-1], nil
	}
	return nil, nil
}

func (m *MockRows) RawValues() [][]byte {
	return nil
}

func (m *MockRows) Conn() *pgx.Conn {
	return nil
}

func (m *MockRow) Scan(dest ...interface{}) error {
	if m.err != nil {
		return m.err
	}
	// Copy mock data to dest pointers
	for i := range dest {
		if i < len(m.data) {
			switch v := dest[i].(type) {
			case *uuid.UUID:
				if val, ok := m.data[i].(uuid.UUID); ok {
					*v = val
				}
			case *string:
				if val, ok := m.data[i].(string); ok {
					*v = val
				}
			case *int:
				if val, ok := m.data[i].(int); ok {
					*v = val
				}
			case *int64:
				if val, ok := m.data[i].(int64); ok {
					*v = val
				}
			case *time.Time:
				if val, ok := m.data[i].(time.Time); ok {
					*v = val
				}
			case *models.JSONB:
				if val, ok := m.data[i].(models.JSONB); ok {
					*v = val
				}
			case *models.TaskStatus:
				if val, ok := m.data[i].(string); ok {
					*v = models.TaskStatus(val)
				}
			case *models.ScriptType:
				if val, ok := m.data[i].(string); ok {
					*v = models.ScriptType(val)
				}
			}
		}
	}
	return nil
}

// TestTaskRepository_CreateValidation tests validation logic in Create method
func TestTaskRepository_CreateValidation(t *testing.T) {
	repo := &taskRepository{
		querier:       nil, // Will cause nil pointer if we reach database calls
		cursorEncoder: NewCursorEncoder(),
	}

	t.Run("nil task validation", func(t *testing.T) {
		err := repo.Create(context.Background(), nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "task cannot be nil")
	})
}

// TestTaskRepository_CreateErrorScenarios tests database error scenarios for Create method
func TestTaskRepository_CreateErrorScenarios(t *testing.T) {
	tests := []struct {
		name      string
		task      *models.Task
		mockSetup func(*MockQuerier)
		wantError string
	}{
		{
			name: "nil task validation",
			task: nil,
			mockSetup: func(mq *MockQuerier) {
				// No database calls expected
			},
			wantError: "task cannot be nil",
		},
		{
			name: "database connection error",
			task: &models.Task{
				UserID:         uuid.New(),
				Name:           "Test Task",
				ScriptContent:  "print('hello')",
				ScriptType:     models.ScriptTypePython,
				Status:         models.TaskStatusPending,
				Priority:       1,
				TimeoutSeconds: 30,
			},
			mockSetup: func(mq *MockQuerier) {
				row := &MockRow{err: errors.New("connection refused")}
				mq.On("QueryRow", mock.Anything, mock.AnythingOfType("string"), mock.Anything).Return(row)
			},
			wantError: "failed to create task",
		},
		{
			name: "unique constraint violation - task ID already exists",
			task: &models.Task{
				UserID:         uuid.New(),
				Name:           "Test Task",
				ScriptContent:  "print('hello')",
				ScriptType:     models.ScriptTypePython,
				Status:         models.TaskStatusPending,
				Priority:       1,
				TimeoutSeconds: 30,
			},
			mockSetup: func(mq *MockQuerier) {
				pgErr := &pgconn.PgError{
					Code: "23505", // unique_violation
				}
				row := &MockRow{err: pgErr}
				mq.On("QueryRow", mock.Anything, mock.AnythingOfType("string"), mock.Anything).Return(row)
			},
			wantError: "already exists",
		},
		{
			name: "foreign key constraint violation - user does not exist",
			task: &models.Task{
				UserID:         uuid.New(),
				Name:           "Test Task",
				ScriptContent:  "print('hello')",
				ScriptType:     models.ScriptTypePython,
				Status:         models.TaskStatusPending,
				Priority:       1,
				TimeoutSeconds: 30,
			},
			mockSetup: func(mq *MockQuerier) {
				pgErr := &pgconn.PgError{
					Code:   "23503", // foreign_key_violation
					Detail: "Key (user_id)=(123e4567-e89b-12d3-a456-426614174000) is not present in table \"users\".",
				}
				row := &MockRow{err: pgErr}
				mq.On("QueryRow", mock.Anything, mock.AnythingOfType("string"), mock.Anything).Return(row)
			},
			wantError: "does not exist",
		},
		{
			name: "check constraint violation - validation failed",
			task: &models.Task{
				UserID:         uuid.New(),
				Name:           "Test Task",
				ScriptContent:  "print('hello')",
				ScriptType:     models.ScriptTypePython,
				Status:         models.TaskStatusPending,
				Priority:       -1, // Invalid priority
				TimeoutSeconds: 30,
			},
			mockSetup: func(mq *MockQuerier) {
				pgErr := &pgconn.PgError{
					Code:   "23514", // check_violation
					Detail: "new row for relation \"tasks\" violates check constraint \"tasks_priority_check\"",
				}
				row := &MockRow{err: pgErr}
				mq.On("QueryRow", mock.Anything, mock.AnythingOfType("string"), mock.Anything).Return(row)
			},
			wantError: "validation failed",
		},
		{
			name: "context timeout error",
			task: &models.Task{
				UserID:         uuid.New(),
				Name:           "Test Task",
				ScriptContent:  "print('hello')",
				ScriptType:     models.ScriptTypePython,
				Status:         models.TaskStatusPending,
				Priority:       1,
				TimeoutSeconds: 30,
			},
			mockSetup: func(mq *MockQuerier) {
				row := &MockRow{err: context.DeadlineExceeded}
				mq.On("QueryRow", mock.Anything, mock.AnythingOfType("string"), mock.Anything).Return(row)
			},
			wantError: "failed to create task",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockQuerier := new(MockQuerier)
			repo := &taskRepository{
				querier:       mockQuerier,
				cursorEncoder: NewCursorEncoder(),
			}

			tt.mockSetup(mockQuerier)

			err := repo.Create(context.Background(), tt.task)

			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantError)
			mockQuerier.AssertExpectations(t)
		})
	}
}

// TestTaskRepository_GetByIDErrorScenarios tests database error scenarios for GetByID method
func TestTaskRepository_GetByIDErrorScenarios(t *testing.T) {
	tests := []struct {
		name      string
		taskID    uuid.UUID
		mockSetup func(*MockQuerier)
		wantError string
	}{
		{
			name:   "task not found",
			taskID: uuid.New(),
			mockSetup: func(mq *MockQuerier) {
				row := &MockRow{err: pgx.ErrNoRows}
				mq.On("QueryRow", mock.Anything, mock.AnythingOfType("string"), mock.Anything).Return(row)
			},
			wantError: "not found",
		},
		{
			name:   "database connection error",
			taskID: uuid.New(),
			mockSetup: func(mq *MockQuerier) {
				row := &MockRow{err: errors.New("connection lost")}
				mq.On("QueryRow", mock.Anything, mock.AnythingOfType("string"), mock.Anything).Return(row)
			},
			wantError: "failed to get task by ID",
		},
		{
			name:   "scan error - corrupted data",
			taskID: uuid.New(),
			mockSetup: func(mq *MockQuerier) {
				row := &MockRow{err: errors.New("cannot scan NULL into string")}
				mq.On("QueryRow", mock.Anything, mock.AnythingOfType("string"), mock.Anything).Return(row)
			},
			wantError: "failed to get task by ID",
		},
		{
			name:   "context cancelled",
			taskID: uuid.New(),
			mockSetup: func(mq *MockQuerier) {
				row := &MockRow{err: context.Canceled}
				mq.On("QueryRow", mock.Anything, mock.AnythingOfType("string"), mock.Anything).Return(row)
			},
			wantError: "failed to get task by ID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockQuerier := new(MockQuerier)
			repo := &taskRepository{
				querier:       mockQuerier,
				cursorEncoder: NewCursorEncoder(),
			}

			tt.mockSetup(mockQuerier)

			task, err := repo.GetByID(context.Background(), tt.taskID)

			assert.Error(t, err)
			assert.Nil(t, task)
			assert.Contains(t, err.Error(), tt.wantError)
			mockQuerier.AssertExpectations(t)
		})
	}
}

// TestTaskRepository_UpdateValidation tests validation logic in Update method
func TestTaskRepository_UpdateValidation(t *testing.T) {
	repo := &taskRepository{
		querier:       nil, // Will cause nil pointer if we reach database calls
		cursorEncoder: NewCursorEncoder(),
	}

	t.Run("nil task validation", func(t *testing.T) {
		err := repo.Update(context.Background(), nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "task cannot be nil")
	})
}

// TestTaskRepository_UpdateErrorScenarios tests database error scenarios for Update method
func TestTaskRepository_UpdateErrorScenarios(t *testing.T) {
	tests := []struct {
		name      string
		task      *models.Task
		mockSetup func(*MockQuerier)
		wantError string
	}{
		{
			name: "nil task validation",
			task: nil,
			mockSetup: func(mq *MockQuerier) {
				// No database calls expected
			},
			wantError: "task cannot be nil",
		},
		{
			name: "task not found",
			task: &models.Task{
				BaseModel: models.BaseModel{ID: uuid.New()},
				UserID:    uuid.New(),
				Name:      "Updated Task",
			},
			mockSetup: func(mq *MockQuerier) {
				row := &MockRow{err: pgx.ErrNoRows}
				mq.On("QueryRow", mock.Anything, mock.AnythingOfType("string"), mock.Anything).Return(row)
			},
			wantError: "not found",
		},
		{
			name: "check constraint violation",
			task: &models.Task{
				BaseModel:      models.BaseModel{ID: uuid.New()},
				UserID:         uuid.New(),
				Name:           "Updated Task",
				Priority:       -5, // Invalid priority
				TimeoutSeconds: 30,
			},
			mockSetup: func(mq *MockQuerier) {
				pgErr := &pgconn.PgError{
					Code:   "23514", // check_violation
					Detail: "new row for relation \"tasks\" violates check constraint \"tasks_priority_check\"",
				}
				row := &MockRow{err: pgErr}
				mq.On("QueryRow", mock.Anything, mock.AnythingOfType("string"), mock.Anything).Return(row)
			},
			wantError: "validation failed",
		},
		{
			name: "database connection lost",
			task: &models.Task{
				BaseModel: models.BaseModel{ID: uuid.New()},
				UserID:    uuid.New(),
				Name:      "Updated Task",
			},
			mockSetup: func(mq *MockQuerier) {
				row := &MockRow{err: errors.New("connection reset by peer")}
				mq.On("QueryRow", mock.Anything, mock.AnythingOfType("string"), mock.Anything).Return(row)
			},
			wantError: "failed to update task",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockQuerier := new(MockQuerier)
			repo := &taskRepository{
				querier:       mockQuerier,
				cursorEncoder: NewCursorEncoder(),
			}

			tt.mockSetup(mockQuerier)

			err := repo.Update(context.Background(), tt.task)

			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantError)
			mockQuerier.AssertExpectations(t)
		})
	}
}

// TestTaskRepository_UpdateStatusErrorScenarios tests database error scenarios for UpdateStatus method
func TestTaskRepository_UpdateStatusErrorScenarios(t *testing.T) {
	tests := []struct {
		name      string
		taskID    uuid.UUID
		status    models.TaskStatus
		mockSetup func(*MockQuerier)
		wantError string
	}{
		{
			name:   "task not found - no rows affected",
			taskID: uuid.New(),
			status: models.TaskStatusCompleted,
			mockSetup: func(mq *MockQuerier) {
				cmdTag := pgconn.NewCommandTag("UPDATE 0")
				mq.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).Return(cmdTag, nil)
			},
			wantError: "not found",
		},
		{
			name:   "invalid task status - check constraint",
			taskID: uuid.New(),
			status: "invalid_status",
			mockSetup: func(mq *MockQuerier) {
				pgErr := &pgconn.PgError{Code: "23514"} // check_violation
				mq.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).Return(nil, pgErr)
			},
			wantError: "invalid task status",
		},
		{
			name:   "database timeout error",
			taskID: uuid.New(),
			status: models.TaskStatusCompleted,
			mockSetup: func(mq *MockQuerier) {
				mq.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).Return(nil, context.DeadlineExceeded)
			},
			wantError: "failed to update task status",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockQuerier := new(MockQuerier)
			repo := &taskRepository{
				querier:       mockQuerier,
				cursorEncoder: NewCursorEncoder(),
			}

			tt.mockSetup(mockQuerier)

			err := repo.UpdateStatus(context.Background(), tt.taskID, tt.status)

			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantError)
			mockQuerier.AssertExpectations(t)
		})
	}
}

// TestTaskRepository_DeleteErrorScenarios tests database error scenarios for Delete method
func TestTaskRepository_DeleteErrorScenarios(t *testing.T) {
	tests := []struct {
		name      string
		taskID    uuid.UUID
		mockSetup func(*MockQuerier)
		wantError string
	}{
		{
			name:   "task not found - no rows affected",
			taskID: uuid.New(),
			mockSetup: func(mq *MockQuerier) {
				cmdTag := pgconn.NewCommandTag("DELETE 0")
				mq.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).Return(cmdTag, nil)
			},
			wantError: "not found",
		},
		{
			name:   "foreign key constraint violation - task has executions",
			taskID: uuid.New(),
			mockSetup: func(mq *MockQuerier) {
				pgErr := &pgconn.PgError{Code: "23503"} // foreign_key_violation
				mq.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).Return(nil, pgErr)
			},
			wantError: "failed to delete task",
		},
		{
			name:   "database connection error",
			taskID: uuid.New(),
			mockSetup: func(mq *MockQuerier) {
				mq.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).Return(nil, errors.New("server closed the connection unexpectedly"))
			},
			wantError: "failed to delete task",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockQuerier := new(MockQuerier)
			repo := &taskRepository{
				querier:       mockQuerier,
				cursorEncoder: NewCursorEncoder(),
			}

			tt.mockSetup(mockQuerier)

			err := repo.Delete(context.Background(), tt.taskID)

			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantError)
			mockQuerier.AssertExpectations(t)
		})
	}
}

// TestTaskRepository_GetByUserIDErrorScenarios tests database error scenarios for GetByUserID method
func TestTaskRepository_GetByUserIDErrorScenarios(t *testing.T) {
	tests := []struct {
		name      string
		userID    uuid.UUID
		limit     int
		offset    int
		mockSetup func(*MockQuerier)
		wantError string
	}{
		{
			name:   "database query error",
			userID: uuid.New(),
			limit:  10,
			offset: 0,
			mockSetup: func(mq *MockQuerier) {
				mq.On("Query", mock.Anything, mock.AnythingOfType("string"), mock.Anything).Return(nil, errors.New("query execution failed"))
			},
			wantError: "failed to get tasks by user ID",
		},
		{
			name:   "context cancellation during query",
			userID: uuid.New(),
			limit:  10,
			offset: 0,
			mockSetup: func(mq *MockQuerier) {
				mq.On("Query", mock.Anything, mock.AnythingOfType("string"), mock.Anything).Return(nil, context.Canceled)
			},
			wantError: "failed to get tasks by user ID",
		},
		{
			name:   "database connection timeout",
			userID: uuid.New(),
			limit:  10,
			offset: 0,
			mockSetup: func(mq *MockQuerier) {
				mq.On("Query", mock.Anything, mock.AnythingOfType("string"), mock.Anything).Return(nil, context.DeadlineExceeded)
			},
			wantError: "failed to get tasks by user ID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockQuerier := new(MockQuerier)
			repo := &taskRepository{
				querier:       mockQuerier,
				cursorEncoder: NewCursorEncoder(),
			}

			tt.mockSetup(mockQuerier)

			tasks, err := repo.GetByUserID(context.Background(), tt.userID, tt.limit, tt.offset)

			assert.Error(t, err)
			assert.Nil(t, tasks)
			assert.Contains(t, err.Error(), tt.wantError)
			mockQuerier.AssertExpectations(t)
		})
	}
}

// TestTaskRepository_CountErrorScenarios tests database error scenarios for Count methods
func TestTaskRepository_CountErrorScenarios(t *testing.T) {
	tests := []struct {
		name      string
		method    string
		mockSetup func(*MockQuerier)
		testFunc  func(*taskRepository) error
		wantError string
	}{
		{
			name:   "Count - database connection error",
			method: "Count",
			mockSetup: func(mq *MockQuerier) {
				row := &MockRow{err: errors.New("connection pool exhausted")}
				mq.On("QueryRow", mock.Anything, mock.AnythingOfType("string"), mock.Anything).Return(row)
			},
			testFunc: func(repo *taskRepository) error {
				_, err := repo.Count(context.Background())
				return err
			},
			wantError: "failed to count tasks",
		},
		{
			name:   "CountByUserID - scan error",
			method: "CountByUserID",
			mockSetup: func(mq *MockQuerier) {
				row := &MockRow{err: errors.New("cannot scan NULL into int64")}
				mq.On("QueryRow", mock.Anything, mock.AnythingOfType("string"), mock.Anything).Return(row)
			},
			testFunc: func(repo *taskRepository) error {
				_, err := repo.CountByUserID(context.Background(), uuid.New())
				return err
			},
			wantError: "failed to count tasks by user ID",
		},
		{
			name:   "CountByStatus - context timeout",
			method: "CountByStatus",
			mockSetup: func(mq *MockQuerier) {
				row := &MockRow{err: context.DeadlineExceeded}
				mq.On("QueryRow", mock.Anything, mock.AnythingOfType("string"), mock.Anything).Return(row)
			},
			testFunc: func(repo *taskRepository) error {
				_, err := repo.CountByStatus(context.Background(), models.TaskStatusPending)
				return err
			},
			wantError: "failed to count tasks by status",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockQuerier := new(MockQuerier)
			repo := &taskRepository{
				querier:       mockQuerier,
				cursorEncoder: NewCursorEncoder(),
			}

			tt.mockSetup(mockQuerier)

			err := tt.testFunc(repo)

			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantError)
			mockQuerier.AssertExpectations(t)
		})
	}
}

// TestTaskRepository_SearchByMetadataErrorScenarios tests database error scenarios for SearchByMetadata method
func TestTaskRepository_SearchByMetadataErrorScenarios(t *testing.T) {
	tests := []struct {
		name      string
		query     string
		limit     int
		offset    int
		mockSetup func(*MockQuerier)
		wantError string
	}{
		{
			name:   "invalid JSON query syntax",
			query:  "invalid json",
			limit:  10,
			offset: 0,
			mockSetup: func(mq *MockQuerier) {
				pgErr := &pgconn.PgError{
					Code:    "22P02", // invalid_text_representation
					Message: "invalid input syntax for type json",
				}
				mq.On("Query", mock.Anything, mock.AnythingOfType("string"), mock.Anything).Return(nil, pgErr)
			},
			wantError: "failed to search tasks by metadata",
		},
		{
			name:   "database connection failure",
			query:  `{"environment": "production"}`,
			limit:  10,
			offset: 0,
			mockSetup: func(mq *MockQuerier) {
				mq.On("Query", mock.Anything, mock.AnythingOfType("string"), mock.Anything).Return(nil, errors.New("server is not accepting connections"))
			},
			wantError: "failed to search tasks by metadata",
		},
		{
			name:   "query execution timeout",
			query:  `{"complex": "query"}`,
			limit:  10,
			offset: 0,
			mockSetup: func(mq *MockQuerier) {
				mq.On("Query", mock.Anything, mock.AnythingOfType("string"), mock.Anything).Return(nil, context.DeadlineExceeded)
			},
			wantError: "failed to search tasks by metadata",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockQuerier := new(MockQuerier)
			repo := &taskRepository{
				querier:       mockQuerier,
				cursorEncoder: NewCursorEncoder(),
			}

			tt.mockSetup(mockQuerier)

			tasks, err := repo.SearchByMetadata(context.Background(), tt.query, tt.limit, tt.offset)

			assert.Error(t, err)
			assert.Nil(t, tasks)
			assert.Contains(t, err.Error(), tt.wantError)
			mockQuerier.AssertExpectations(t)
		})
	}
}

// TestTaskRepository_CursorPaginationErrorScenarios tests database error scenarios for cursor pagination methods
func TestTaskRepository_CursorPaginationErrorScenarios(t *testing.T) {
	tests := []struct {
		name      string
		testFunc  func(*taskRepository) error
		mockSetup func(*MockQuerier)
		wantError string
	}{
		{
			name: "GetByUserIDCursor - invalid cursor",
			testFunc: func(repo *taskRepository) error {
				req := CursorPaginationRequest{
					Limit:  10,
					Cursor: stringPtr("invalid-cursor-data"),
				}
				_, _, err := repo.GetByUserIDCursor(context.Background(), uuid.New(), req)
				return err
			},
			mockSetup: func(mq *MockQuerier) {
				// Mock expects no calls since cursor validation fails first
			},
			wantError: "invalid cursor",
		},
		{
			name: "ListCursor - database query failure",
			testFunc: func(repo *taskRepository) error {
				req := CursorPaginationRequest{Limit: 10}
				_, _, err := repo.ListCursor(context.Background(), req)
				return err
			},
			mockSetup: func(mq *MockQuerier) {
				mq.On("Query", mock.Anything, mock.AnythingOfType("string"), mock.Anything).Return(nil, errors.New("database server error"))
			},
			wantError: "failed to list tasks with cursor",
		},
		{
			name: "GetByStatusCursor - connection timeout",
			testFunc: func(repo *taskRepository) error {
				req := CursorPaginationRequest{Limit: 10}
				_, _, err := repo.GetByStatusCursor(context.Background(), models.TaskStatusPending, req)
				return err
			},
			mockSetup: func(mq *MockQuerier) {
				mq.On("Query", mock.Anything, mock.AnythingOfType("string"), mock.Anything).Return(nil, context.DeadlineExceeded)
			},
			wantError: "failed to get tasks by status with cursor",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockQuerier := new(MockQuerier)
			repo := &taskRepository{
				querier:       mockQuerier,
				cursorEncoder: NewCursorEncoder(),
			}

			tt.mockSetup(mockQuerier)

			err := tt.testFunc(repo)

			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantError)
			mockQuerier.AssertExpectations(t)
		})
	}
}

// Benchmark tests for unit test performance
func BenchmarkTaskRepository_CreateValidation(b *testing.B) {
	repo := &taskRepository{
		querier:       nil,
		cursorEncoder: NewCursorEncoder(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = repo.Create(context.Background(), nil)
	}
}

func BenchmarkTaskRepository_MockGetByID(b *testing.B) {
	mockQuerier := new(MockQuerier)
	repo := &taskRepository{
		querier:       mockQuerier,
		cursorEncoder: NewCursorEncoder(),
	}

	row := &MockRow{data: []interface{}{uuid.New(), uuid.New(), "Test", "desc", "code", "python", "pending", 1, 30, models.JSONB{}, time.Now(), time.Now()}}
	mockQuerier.On("QueryRow", mock.Anything, mock.AnythingOfType("string"), mock.Anything).Return(row)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = repo.GetByID(context.Background(), uuid.New())
	}
}

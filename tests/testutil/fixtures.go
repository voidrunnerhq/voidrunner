package testutil

import (
	"time"

	"github.com/google/uuid"
	"github.com/voidrunnerhq/voidrunner/internal/models"
)

// UserFixtures contains predefined user test data
type UserFixtures struct {
	AdminUser    *models.User
	RegularUser  *models.User
	InactiveUser *models.User
}

// TaskFixtures contains predefined task test data
type TaskFixtures struct {
	PendingTask   *models.Task
	RunningTask   *models.Task
	CompletedTask *models.Task
	FailedTask    *models.Task
}

// ExecutionFixtures contains predefined execution test data
type ExecutionFixtures struct {
	SuccessfulExecution *models.TaskExecution
	FailedExecution     *models.TaskExecution
	TimeoutExecution    *models.TaskExecution
}

// NewUserFixtures creates standardized user test data
func NewUserFixtures() *UserFixtures {
	now := time.Now()

	return &UserFixtures{
		AdminUser: &models.User{
			BaseModel: models.BaseModel{
				ID:        uuid.MustParse("12345678-1234-1234-1234-123456789000"),
				CreatedAt: now,
				UpdatedAt: now,
			},
			Email:        "admin@voidrunner.dev",
			PasswordHash: "$2a$12$LQv3c1yqBWVHxkd0LHAkCOYz6TtxMQJqhN8/LewdBPj4KTrkYEoA2", // password: AdminPass123!
			Name:         "Admin User",
		},
		RegularUser: &models.User{
			BaseModel: models.BaseModel{
				ID:        uuid.MustParse("12345678-1234-1234-1234-123456789001"),
				CreatedAt: now,
				UpdatedAt: now,
			},
			Email:        "user@voidrunner.dev",
			PasswordHash: "$2a$12$LQv3c1yqBWVHxkd0LHAkCOYz6TtxMQJqhN8/LewdBPj4KTrkYEoA2", // password: UserPass123!
			Name:         "Regular User",
		},
		InactiveUser: &models.User{
			BaseModel: models.BaseModel{
				ID:        uuid.MustParse("12345678-1234-1234-1234-123456789002"),
				CreatedAt: now.Add(-24 * time.Hour),
				UpdatedAt: now.Add(-24 * time.Hour),
			},
			Email:        "inactive@voidrunner.dev",
			PasswordHash: "$2a$12$LQv3c1yqBWVHxkd0LHAkCOYz6TtxMQJqhN8/LewdBPj4KTrkYEoA2", // password: InactivePass123!
			Name:         "Inactive User",
		},
	}
}

// NewTaskFixtures creates standardized task test data for a given user
func NewTaskFixtures(userID uuid.UUID) *TaskFixtures {
	now := time.Now()

	return &TaskFixtures{
		PendingTask: &models.Task{
			BaseModel: models.BaseModel{
				ID:        uuid.MustParse("22345678-1234-1234-1234-123456789000"),
				CreatedAt: now,
				UpdatedAt: now,
			},
			UserID:         userID,
			Name:           "Test Pending Task",
			Description:    stringPtr("A task waiting to be executed"),
			ScriptContent:  "print('Hello from pending task')",
			ScriptType:     models.ScriptTypePython,
			Status:         models.TaskStatusPending,
			Priority:       5,
			TimeoutSeconds: 30,
			Metadata: models.JSONB{
				"environment": "test",
				"tags":        []string{"pending", "test"},
			},
		},
		RunningTask: &models.Task{
			BaseModel: models.BaseModel{
				ID:        uuid.MustParse("22345678-1234-1234-1234-123456789001"),
				CreatedAt: now.Add(-5 * time.Minute),
				UpdatedAt: now.Add(-1 * time.Minute),
			},
			UserID:         userID,
			Name:           "Test Running Task",
			Description:    stringPtr("A task currently being executed"),
			ScriptContent:  "import time; time.sleep(10); print('Long running task')",
			ScriptType:     models.ScriptTypePython,
			Status:         models.TaskStatusRunning,
			Priority:       8,
			TimeoutSeconds: 60,
			Metadata: models.JSONB{
				"environment":  "test",
				"tags":         []string{"running", "test"},
				"started_by":   "test_suite",
				"resource_req": map[string]interface{}{"cpu": "100m", "memory": "128Mi"},
			},
		},
		CompletedTask: &models.Task{
			BaseModel: models.BaseModel{
				ID:        uuid.MustParse("22345678-1234-1234-1234-123456789002"),
				CreatedAt: now.Add(-1 * time.Hour),
				UpdatedAt: now.Add(-45 * time.Minute),
			},
			UserID:         userID,
			Name:           "Test Completed Task",
			Description:    stringPtr("A successfully completed task"),
			ScriptContent:  "print('Task completed successfully')",
			ScriptType:     models.ScriptTypePython,
			Status:         models.TaskStatusCompleted,
			Priority:       3,
			TimeoutSeconds: 15,
			Metadata: models.JSONB{
				"environment": "test",
				"tags":        []string{"completed", "test"},
				"result":      "success",
			},
		},
		FailedTask: &models.Task{
			BaseModel: models.BaseModel{
				ID:        uuid.MustParse("22345678-1234-1234-1234-123456789003"),
				CreatedAt: now.Add(-2 * time.Hour),
				UpdatedAt: now.Add(-90 * time.Minute),
			},
			UserID:         userID,
			Name:           "Test Failed Task",
			Description:    stringPtr("A task that failed during execution"),
			ScriptContent:  "import sys; sys.exit(1)",
			ScriptType:     models.ScriptTypePython,
			Status:         models.TaskStatusFailed,
			Priority:       7,
			TimeoutSeconds: 20,
			Metadata: models.JSONB{
				"environment": "test",
				"tags":        []string{"failed", "test"},
				"error_type":  "user_error",
			},
		},
	}
}

// NewExecutionFixtures creates standardized execution test data for given tasks
func NewExecutionFixtures(taskFixtures *TaskFixtures) *ExecutionFixtures {
	now := time.Now()

	return &ExecutionFixtures{
		SuccessfulExecution: &models.TaskExecution{
			ID:               uuid.MustParse("32345678-1234-1234-1234-123456789000"),
			TaskID:           taskFixtures.CompletedTask.ID,
			Status:           models.ExecutionStatusCompleted,
			ReturnCode:       intPtr(0),
			Stdout:           stringPtr("Task completed successfully\n"),
			Stderr:           stringPtr(""),
			ExecutionTimeMs:  intPtr(1250),
			MemoryUsageBytes: int64Ptr(1048576), // 1MB
			StartedAt:        timePtr(now.Add(-46 * time.Minute)),
			CompletedAt:      timePtr(now.Add(-45 * time.Minute)),
			CreatedAt:        now.Add(-45 * time.Minute),
		},
		FailedExecution: &models.TaskExecution{
			ID:               uuid.MustParse("32345678-1234-1234-1234-123456789001"),
			TaskID:           taskFixtures.FailedTask.ID,
			Status:           models.ExecutionStatusFailed,
			ReturnCode:       intPtr(1),
			Stdout:           stringPtr(""),
			Stderr:           stringPtr("Process exited with code 1\n"),
			ExecutionTimeMs:  intPtr(500),
			MemoryUsageBytes: int64Ptr(524288), // 512KB
			StartedAt:        timePtr(now.Add(-91 * time.Minute)),
			CompletedAt:      timePtr(now.Add(-90 * time.Minute)),
			CreatedAt:        now.Add(-90 * time.Minute),
		},
		TimeoutExecution: &models.TaskExecution{
			ID:               uuid.MustParse("32345678-1234-1234-1234-123456789002"),
			TaskID:           taskFixtures.PendingTask.ID, // Reference to pending task
			Status:           models.ExecutionStatusTimeout,
			ReturnCode:       intPtr(124), // SIGTERM exit code
			Stdout:           stringPtr("Starting long running process...\n"),
			Stderr:           stringPtr("Process terminated due to timeout\n"),
			ExecutionTimeMs:  intPtr(30000),     // 30 seconds
			MemoryUsageBytes: int64Ptr(2097152), // 2MB
			StartedAt:        timePtr(now.Add(-120 * time.Minute)),
			CompletedAt:      timePtr(now.Add(-119 * time.Minute)),
			CreatedAt:        now.Add(-2 * time.Hour),
		},
	}
}

// AllFixtures combines all fixture types for comprehensive testing
type AllFixtures struct {
	Users      *UserFixtures
	Tasks      *TaskFixtures
	Executions *ExecutionFixtures
}

// NewAllFixtures creates a complete set of test fixtures
func NewAllFixtures() *AllFixtures {
	users := NewUserFixtures()
	tasks := NewTaskFixtures(users.RegularUser.ID)
	executions := NewExecutionFixtures(tasks)

	return &AllFixtures{
		Users:      users,
		Tasks:      tasks,
		Executions: executions,
	}
}

// IntPtr returns a pointer to an int value
func IntPtr(i int) *int {
	return &i
}

// intPtr is kept for backwards compatibility
func intPtr(i int) *int {
	return IntPtr(i)
}

func int64Ptr(i int64) *int64 {
	return &i
}

func timePtr(t time.Time) *time.Time {
	return &t
}

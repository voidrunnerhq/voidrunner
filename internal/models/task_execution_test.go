package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestValidateExecutionStatus(t *testing.T) {
	tests := []struct {
		name    string
		status  ExecutionStatus
		wantErr bool
	}{
		{
			name:    "valid pending status",
			status:  ExecutionStatusPending,
			wantErr: false,
		},
		{
			name:    "valid running status",
			status:  ExecutionStatusRunning,
			wantErr: false,
		},
		{
			name:    "valid completed status",
			status:  ExecutionStatusCompleted,
			wantErr: false,
		},
		{
			name:    "valid failed status",
			status:  ExecutionStatusFailed,
			wantErr: false,
		},
		{
			name:    "valid timeout status",
			status:  ExecutionStatusTimeout,
			wantErr: false,
		},
		{
			name:    "valid cancelled status",
			status:  ExecutionStatusCancelled,
			wantErr: false,
		},
		{
			name:    "invalid status",
			status:  "invalid",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateExecutionStatus(tt.status)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestTaskExecution_IsTerminal(t *testing.T) {
	tests := []struct {
		name     string
		status   ExecutionStatus
		expected bool
	}{
		{
			name:     "pending is not terminal",
			status:   ExecutionStatusPending,
			expected: false,
		},
		{
			name:     "running is not terminal",
			status:   ExecutionStatusRunning,
			expected: false,
		},
		{
			name:     "completed is terminal",
			status:   ExecutionStatusCompleted,
			expected: true,
		},
		{
			name:     "failed is terminal",
			status:   ExecutionStatusFailed,
			expected: true,
		},
		{
			name:     "timeout is terminal",
			status:   ExecutionStatusTimeout,
			expected: true,
		},
		{
			name:     "cancelled is terminal",
			status:   ExecutionStatusCancelled,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			te := &TaskExecution{Status: tt.status}
			assert.Equal(t, tt.expected, te.IsTerminal())
		})
	}
}

func TestTaskExecution_IsRunning(t *testing.T) {
	tests := []struct {
		name     string
		status   ExecutionStatus
		expected bool
	}{
		{
			name:     "running status",
			status:   ExecutionStatusRunning,
			expected: true,
		},
		{
			name:     "pending status",
			status:   ExecutionStatusPending,
			expected: false,
		},
		{
			name:     "completed status",
			status:   ExecutionStatusCompleted,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			te := &TaskExecution{Status: tt.status}
			assert.Equal(t, tt.expected, te.IsRunning())
		})
	}
}

func TestTaskExecution_IsPending(t *testing.T) {
	tests := []struct {
		name     string
		status   ExecutionStatus
		expected bool
	}{
		{
			name:     "pending status",
			status:   ExecutionStatusPending,
			expected: true,
		},
		{
			name:     "running status",
			status:   ExecutionStatusRunning,
			expected: false,
		},
		{
			name:     "completed status",
			status:   ExecutionStatusCompleted,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			te := &TaskExecution{Status: tt.status}
			assert.Equal(t, tt.expected, te.IsPending())
		})
	}
}

func TestTaskExecution_GetDuration(t *testing.T) {
	t.Run("with started and completed times", func(t *testing.T) {
		startTime := time.Now()
		endTime := startTime.Add(2 * time.Second)
		
		te := &TaskExecution{
			StartedAt:   &startTime,
			CompletedAt: &endTime,
		}

		duration := te.GetDuration()
		assert.NotNil(t, duration)
		assert.Equal(t, 2000, *duration) // 2 seconds in milliseconds
	})

	t.Run("with execution time ms set", func(t *testing.T) {
		executionTime := 1500
		te := &TaskExecution{
			ExecutionTimeMs: &executionTime,
		}

		duration := te.GetDuration()
		assert.NotNil(t, duration)
		assert.Equal(t, 1500, *duration)
	})

	t.Run("without timing information", func(t *testing.T) {
		te := &TaskExecution{}

		duration := te.GetDuration()
		assert.Nil(t, duration)
	})
}

func TestTaskExecution_ToResponse(t *testing.T) {
	startTime := time.Now()
	endTime := startTime.Add(time.Second)
	returnCode := 0
	stdout := "test output"
	stderr := ""
	executionTime := 1000
	memoryUsage := int64(1024 * 1024)

	te := &TaskExecution{
		ID:               NewID(),
		TaskID:           NewID(),
		Status:           ExecutionStatusCompleted,
		ReturnCode:       &returnCode,
		Stdout:           &stdout,
		Stderr:           &stderr,
		ExecutionTimeMs:  &executionTime,
		MemoryUsageBytes: &memoryUsage,
		StartedAt:        &startTime,
		CompletedAt:      &endTime,
		CreatedAt:        time.Now(),
	}

	response := te.ToResponse()

	assert.Equal(t, te.ID, response.ID)
	assert.Equal(t, te.TaskID, response.TaskID)
	assert.Equal(t, te.Status, response.Status)
	assert.Equal(t, *te.ReturnCode, *response.ReturnCode)
	assert.Equal(t, *te.Stdout, *response.Stdout)
	assert.Equal(t, *te.Stderr, *response.Stderr)
	assert.Equal(t, *te.ExecutionTimeMs, *response.ExecutionTimeMs)
	assert.Equal(t, *te.MemoryUsageBytes, *response.MemoryUsageBytes)
	assert.NotNil(t, response.StartedAt)
	assert.NotNil(t, response.CompletedAt)
	assert.NotEmpty(t, response.CreatedAt)
}

func TestTaskExecution_ToResponse_WithNilValues(t *testing.T) {
	te := &TaskExecution{
		ID:        NewID(),
		TaskID:    NewID(),
		Status:    ExecutionStatusPending,
		CreatedAt: time.Now(),
	}

	response := te.ToResponse()

	assert.Equal(t, te.ID, response.ID)
	assert.Equal(t, te.TaskID, response.TaskID)
	assert.Equal(t, te.Status, response.Status)
	assert.Nil(t, response.ReturnCode)
	assert.Nil(t, response.Stdout)
	assert.Nil(t, response.Stderr)
	assert.Nil(t, response.ExecutionTimeMs)
	assert.Nil(t, response.MemoryUsageBytes)
	assert.Nil(t, response.StartedAt)
	assert.Nil(t, response.CompletedAt)
	assert.NotEmpty(t, response.CreatedAt)
}
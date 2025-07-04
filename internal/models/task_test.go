package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateTaskName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid task name",
			input:   "Valid Task Name",
			wantErr: false,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
			errMsg:  "task name is required",
		},
		{
			name:    "whitespace only",
			input:   "   ",
			wantErr: true,
			errMsg:  "task name cannot be empty",
		},
		{
			name:    "too long name",
			input:   string(make([]byte, 256)),
			wantErr: true,
			errMsg:  "task name is too long",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTaskName(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateScriptType(t *testing.T) {
	tests := []struct {
		name       string
		scriptType ScriptType
		wantErr    bool
	}{
		{
			name:       "valid python",
			scriptType: ScriptTypePython,
			wantErr:    false,
		},
		{
			name:       "valid javascript",
			scriptType: ScriptTypeJavaScript,
			wantErr:    false,
		},
		{
			name:       "valid bash",
			scriptType: ScriptTypeBash,
			wantErr:    false,
		},
		{
			name:       "valid go",
			scriptType: ScriptTypeGo,
			wantErr:    false,
		},
		{
			name:       "invalid script type",
			scriptType: "invalid",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateScriptType(tt.scriptType)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateScriptContent(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid script content",
			content: "print('hello world')",
			wantErr: false,
		},
		{
			name:    "empty content",
			content: "",
			wantErr: true,
			errMsg:  "script content is required",
		},
		{
			name:    "whitespace only content",
			content: "   ",
			wantErr: true,
			errMsg:  "script content cannot be empty",
		},
		{
			name:    "too long content",
			content: string(make([]byte, 65536)),
			wantErr: true,
			errMsg:  "script content is too long",
		},
		{
			name:    "dangerous content",
			content: "rm -rf /",
			wantErr: true,
			errMsg:  "potentially dangerous script content detected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateScriptContent(tt.content)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateTaskStatus(t *testing.T) {
	tests := []struct {
		name    string
		status  TaskStatus
		wantErr bool
	}{
		{
			name:    "valid pending status",
			status:  TaskStatusPending,
			wantErr: false,
		},
		{
			name:    "valid running status",
			status:  TaskStatusRunning,
			wantErr: false,
		},
		{
			name:    "valid completed status",
			status:  TaskStatusCompleted,
			wantErr: false,
		},
		{
			name:    "valid failed status",
			status:  TaskStatusFailed,
			wantErr: false,
		},
		{
			name:    "valid timeout status",
			status:  TaskStatusTimeout,
			wantErr: false,
		},
		{
			name:    "valid cancelled status",
			status:  TaskStatusCancelled,
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
			err := ValidateTaskStatus(tt.status)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidatePriority(t *testing.T) {
	tests := []struct {
		name     string
		priority int
		wantErr  bool
	}{
		{
			name:     "valid priority 0",
			priority: 0,
			wantErr:  false,
		},
		{
			name:     "valid priority 5",
			priority: 5,
			wantErr:  false,
		},
		{
			name:     "valid priority 10",
			priority: 10,
			wantErr:  false,
		},
		{
			name:     "invalid negative priority",
			priority: -1,
			wantErr:  true,
		},
		{
			name:     "invalid too high priority",
			priority: 11,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePriority(tt.priority)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateTimeout(t *testing.T) {
	tests := []struct {
		name    string
		timeout int
		wantErr bool
	}{
		{
			name:    "valid timeout",
			timeout: 30,
			wantErr: false,
		},
		{
			name:    "minimum valid timeout",
			timeout: 1,
			wantErr: false,
		},
		{
			name:    "maximum valid timeout",
			timeout: 3600,
			wantErr: false,
		},
		{
			name:    "invalid zero timeout",
			timeout: 0,
			wantErr: true,
		},
		{
			name:    "invalid negative timeout",
			timeout: -1,
			wantErr: true,
		},
		{
			name:    "invalid too large timeout",
			timeout: 3601,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTimeout(tt.timeout)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestTask_ToResponse(t *testing.T) {
	task := &Task{
		BaseModel: BaseModel{
			ID: NewID(),
		},
		UserID:         NewID(),
		Name:           "Test Task",
		ScriptContent:  "print('test')",
		ScriptType:     ScriptTypePython,
		Status:         TaskStatusPending,
		Priority:       1,
		TimeoutSeconds: 30,
	}

	response := task.ToResponse()

	assert.Equal(t, task.ID, response.ID)
	assert.Equal(t, task.UserID, response.UserID)
	assert.Equal(t, task.Name, response.Name)
	assert.Equal(t, task.ScriptContent, response.ScriptContent)
	assert.Equal(t, task.ScriptType, response.ScriptType)
	assert.Equal(t, task.Status, response.Status)
	assert.Equal(t, task.Priority, response.Priority)
	assert.Equal(t, task.TimeoutSeconds, response.TimeoutSeconds)
	assert.NotEmpty(t, response.CreatedAt)
	assert.NotEmpty(t, response.UpdatedAt)
}
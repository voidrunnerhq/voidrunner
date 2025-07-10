package middleware

import (
	"fmt"
	"log/slog"
	"net/http"
	"reflect"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/voidrunnerhq/voidrunner/internal/models"
)

// ValidationMiddleware handles request validation
type ValidationMiddleware struct {
	validator *validator.Validate
	logger    *slog.Logger
}

// NewValidationMiddleware creates a new validation middleware
func NewValidationMiddleware(logger *slog.Logger) *ValidationMiddleware {
	v := validator.New()

	// Register custom validators
	_ = v.RegisterValidation("script_content", validateScriptContent)
	_ = v.RegisterValidation("script_type", validateScriptType)
	_ = v.RegisterValidation("task_name", validateTaskName)

	return &ValidationMiddleware{
		validator: v,
		logger:    logger,
	}
}

// ValidateJSON validates JSON request body against struct tags
func (vm *ValidationMiddleware) ValidateJSON(modelType interface{}) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Create new instance of the model type
		model := reflect.New(reflect.TypeOf(modelType)).Interface()

		// Bind JSON to model
		if err := c.ShouldBindJSON(model); err != nil {
			vm.logger.Warn("JSON binding failed", "error", err)
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "Invalid request format",
				"details": err.Error(),
			})
			c.Abort()
			return
		}

		// Validate the model
		if err := vm.validator.Struct(model); err != nil {
			vm.logger.Warn("validation failed", "error", err)

			// Format validation errors nicely
			validationErrors := vm.formatValidationErrors(err)
			c.JSON(http.StatusBadRequest, gin.H{
				"error":             "Validation failed",
				"validation_errors": validationErrors,
			})
			c.Abort()
			return
		}

		// Store validated model in context
		c.Set("validated_body", model)
		c.Next()
	}
}

// ValidateTaskCreation validates task creation requests
func (vm *ValidationMiddleware) ValidateTaskCreation() gin.HandlerFunc {
	return vm.ValidateJSON(models.CreateTaskRequest{})
}

// ValidateTaskUpdate validates task update requests
func (vm *ValidationMiddleware) ValidateTaskUpdate() gin.HandlerFunc {
	return vm.ValidateJSON(models.UpdateTaskRequest{})
}

// ValidateTaskExecutionUpdate validates task execution update requests
func (vm *ValidationMiddleware) ValidateTaskExecutionUpdate() gin.HandlerFunc {
	return vm.ValidateJSON(models.UpdateTaskExecutionRequest{})
}

// ValidateRequestSize validates request body size
func (vm *ValidationMiddleware) ValidateRequestSize(maxSize int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check Content-Length header
		if c.Request.ContentLength > maxSize {
			vm.logger.Warn("request body too large",
				"content_length", c.Request.ContentLength,
				"max_size", maxSize,
			)
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{
				"error": fmt.Sprintf("Request body too large. Maximum size: %d bytes", maxSize),
			})
			c.Abort()
			return
		}

		// Limit the request body reader
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxSize)

		c.Next()
	}
}

// formatValidationErrors formats validator errors into a user-friendly format
func (vm *ValidationMiddleware) formatValidationErrors(err error) []map[string]string {
	var errors []map[string]string

	for _, err := range err.(validator.ValidationErrors) {
		fieldError := map[string]string{
			"field":   err.Field(),
			"value":   fmt.Sprintf("%v", err.Value()),
			"tag":     err.Tag(),
			"message": vm.getValidationMessage(err),
		}
		errors = append(errors, fieldError)
	}

	return errors
}

// getValidationMessage returns a user-friendly validation message
func (vm *ValidationMiddleware) getValidationMessage(err validator.FieldError) string {
	switch err.Tag() {
	case "required":
		return fmt.Sprintf("%s is required", err.Field())
	case "min":
		return fmt.Sprintf("%s must be at least %s characters", err.Field(), err.Param())
	case "max":
		return fmt.Sprintf("%s must be at most %s characters", err.Field(), err.Param())
	case "email":
		return fmt.Sprintf("%s must be a valid email address", err.Field())
	case "oneof":
		return fmt.Sprintf("%s must be one of: %s", err.Field(), err.Param())
	case "script_content":
		return "Script content contains potentially dangerous patterns"
	case "script_type":
		return "Invalid script type. Supported types: python, javascript, bash, go"
	case "task_name":
		return "Task name contains invalid characters or is too long"
	default:
		return fmt.Sprintf("%s failed validation: %s", err.Field(), err.Tag())
	}
}

// Custom validation functions

// validateScriptContent validates script content for security
func validateScriptContent(fl validator.FieldLevel) bool {
	content := fl.Field().String()
	content = strings.ToLower(strings.TrimSpace(content))

	if content == "" {
		return false
	}

	// List of dangerous patterns
	dangerousPatterns := []string{
		"rm -rf",
		"rm -r",
		"rm -f",
		"rmdir",
		"del /f",
		"del /s",
		"format c:",
		"mkfs",
		"dd if=",
		":(){ :|:& };:", // Fork bomb
		"chmod 777",
		"chmod +x",
		"/etc/passwd",
		"/etc/shadow",
		"sudo",
		"su -",
		"passwd",
		"useradd",
		"userdel",
		"curl",
		"wget",
		"nc -",
		"netcat",
		"telnet",
		"ssh",
		"scp",
		"rsync",
		"ping -f",
		"iptables",
		"firewall",
		"kill -9",
		"killall",
		"pkill",
		"reboot",
		"shutdown",
		"halt",
		"poweroff",
		"mount",
		"umount",
		"fdisk",
		"crontab",
		"at ",
		"batch",
		"nohup",
		"disown",
		"exec(",
		"eval(",
		"system(",
		"shell_exec",
		"passthru",
		"proc_open",
		"popen",
		"file_get_contents",
		"file_put_contents",
		"fopen",
		"fwrite",
		"include(",
		"require(",
		"import os",
		"import subprocess",
		"import sys",
		"__import__",
		"exec(",
		"eval(",
		"compile(",
		"open(",
		"input(",
		"raw_input(",
		"execfile(",
		"reload(",
		"exit(",
		"quit(",
	}

	// Check for dangerous patterns
	for _, pattern := range dangerousPatterns {
		if strings.Contains(content, pattern) {
			return false
		}
	}

	// Check for suspicious file paths
	suspiciousPaths := []string{
		"/bin/",
		"/sbin/",
		"/usr/bin/",
		"/usr/sbin/",
		"/etc/",
		"/var/",
		"/tmp/",
		"/proc/",
		"/sys/",
		"/dev/",
		"/root/",
		"/home/",
		"c:\\",
		"c:/",
		"../",
		"./",
		"~/",
	}

	for _, path := range suspiciousPaths {
		if strings.Contains(content, path) {
			return false
		}
	}

	// Check for base64 encoded content that might hide malicious code
	if strings.Contains(content, "base64") || strings.Contains(content, "b64decode") {
		return false
	}

	// Check for hex encoded content
	if strings.Contains(content, "\\x") || strings.Contains(content, "0x") {
		return false
	}

	return true
}

// validateScriptType validates script type
func validateScriptType(fl validator.FieldLevel) bool {
	scriptType := fl.Field().String()
	validTypes := []string{"python", "javascript", "bash", "go"}

	for _, validType := range validTypes {
		if scriptType == validType {
			return true
		}
	}

	return false
}

// validateTaskName validates task name
func validateTaskName(fl validator.FieldLevel) bool {
	name := strings.TrimSpace(fl.Field().String())

	if name == "" || len(name) > 255 {
		return false
	}

	// Check for invalid characters
	invalidChars := []string{
		"<", ">", "\"", "'", "&", ";", "|", "`", "$", "(", ")", "{", "}", "[", "]",
		"\\", "/", ":", "*", "?", "\n", "\r", "\t",
	}

	for _, char := range invalidChars {
		if strings.Contains(name, char) {
			return false
		}
	}

	return true
}

// Common validation middleware factories

// TaskValidation returns validation middleware for task endpoints
func TaskValidation(logger *slog.Logger) *ValidationMiddleware {
	return NewValidationMiddleware(logger)
}

// RequestSizeLimit returns middleware that limits request body size to 1MB
func RequestSizeLimit(logger *slog.Logger) gin.HandlerFunc {
	vm := NewValidationMiddleware(logger)
	return vm.ValidateRequestSize(1024 * 1024) // 1MB limit
}

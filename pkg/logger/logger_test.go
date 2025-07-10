package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	testCases := []struct {
		name        string
		level       string
		format      string
		expectedMsg bool
	}{
		{
			name:        "debug level json format",
			level:       "debug",
			format:      "json",
			expectedMsg: true,
		},
		{
			name:        "info level text format",
			level:       "info",
			format:      "text",
			expectedMsg: true,
		},
		{
			name:        "warn level default format",
			level:       "warn",
			format:      "",
			expectedMsg: false, // warn level won't show info logs
		},
		{
			name:        "error level",
			level:       "error",
			format:      "json",
			expectedMsg: false, // error level won't show info logs
		},
		{
			name:        "invalid level defaults to info",
			level:       "invalid",
			format:      "json",
			expectedMsg: true,
		},
		{
			name:        "uppercase level",
			level:       "DEBUG",
			format:      "JSON",
			expectedMsg: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Use buffer to capture log output
			var buf bytes.Buffer
			logger := NewWithWriter(tc.level, tc.format, &buf)
			assert.NotNil(t, logger)
			assert.NotNil(t, logger.Logger)

			// Test logging
			logger.Info("test message", "key", "value")

			outputStr := buf.String()

			if tc.expectedMsg {
				assert.Contains(t, outputStr, "test message")
				assert.Contains(t, outputStr, "key")
				assert.Contains(t, outputStr, "value")
			} else {
				// Error level shouldn't show info messages
				assert.Empty(t, outputStr)
			}

			// Test format
			if tc.format == "text" && tc.expectedMsg {
				// Text format should not be JSON
				assert.False(t, json.Valid([]byte(outputStr)))
			} else if tc.expectedMsg {
				// Should be JSON format
				lines := strings.Split(strings.TrimSpace(outputStr), "\n")
				if len(lines) > 0 && lines[0] != "" {
					assert.True(t, json.Valid([]byte(lines[0])), "Output should be valid JSON: %s", lines[0])
				}
			}
		})
	}
}

func TestLogger_WithRequestID(t *testing.T) {
	var buf bytes.Buffer
	logger := NewWithWriter("info", "json", &buf)

	loggerWithRequestID := logger.WithRequestID("test-request-123")
	assert.NotNil(t, loggerWithRequestID)
	assert.NotEqual(t, logger, loggerWithRequestID) // Should be a new instance

	// Test logging with request ID
	loggerWithRequestID.Info("test message")

	outputStr := buf.String()

	assert.Contains(t, outputStr, "test-request-123")
	assert.Contains(t, outputStr, "request_id")
}

func TestLogger_WithContext(t *testing.T) {
	t.Run("context with request_id", func(t *testing.T) {
		var buf bytes.Buffer
		logger := NewWithWriter("info", "json", &buf)

		ctx := context.WithValue(context.Background(), requestIDKey, "ctx-request-456")

		loggerWithCtx := logger.WithContext(ctx)
		assert.NotNil(t, loggerWithCtx)

		loggerWithCtx.Info("context test")

		outputStr := buf.String()

		assert.Contains(t, outputStr, "ctx-request-456")
	})

	t.Run("context without request_id", func(t *testing.T) {
		logger := New("info", "json")
		ctx := context.Background()

		loggerWithCtx := logger.WithContext(ctx)
		assert.NotNil(t, loggerWithCtx)

		// Should return the same logger instance
		assert.Equal(t, logger, loggerWithCtx)
	})

	t.Run("context with non-string request_id", func(t *testing.T) {
		logger := New("info", "json")
		ctx := context.WithValue(context.Background(), requestIDKey, 123)

		loggerWithCtx := logger.WithContext(ctx)
		assert.NotNil(t, loggerWithCtx)

		// Should return the same logger instance
		assert.Equal(t, logger, loggerWithCtx)
	})
}

func TestLogger_WithUserID(t *testing.T) {
	var buf bytes.Buffer
	logger := NewWithWriter("info", "json", &buf)

	loggerWithUserID := logger.WithUserID("user-789")
	assert.NotNil(t, loggerWithUserID)

	loggerWithUserID.Info("user test")

	outputStr := buf.String()

	assert.Contains(t, outputStr, "user-789")
	assert.Contains(t, outputStr, "user_id")
}

func TestLogger_WithOperation(t *testing.T) {
	var buf bytes.Buffer
	logger := NewWithWriter("info", "json", &buf)

	loggerWithOp := logger.WithOperation("create_task")
	assert.NotNil(t, loggerWithOp)

	loggerWithOp.Info("operation test")

	outputStr := buf.String()

	assert.Contains(t, outputStr, "create_task")
	assert.Contains(t, outputStr, "operation")
}

func TestLogger_WithError(t *testing.T) {
	var buf bytes.Buffer
	logger := NewWithWriter("info", "json", &buf)

	testErr := errors.New("test error message")
	loggerWithErr := logger.WithError(testErr)
	assert.NotNil(t, loggerWithErr)

	loggerWithErr.Info("error test")

	outputStr := buf.String()

	assert.Contains(t, outputStr, "test error message")
	assert.Contains(t, outputStr, "error")
}

func TestLogger_GinLogger(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var buf bytes.Buffer
	logger := NewWithWriter("info", "json", &buf)
	middleware := logger.GinLogger()
	assert.NotNil(t, middleware)

	// Create test router
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("request_id", "gin-test-123")
		c.Next()
	})
	router.Use(middleware)
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	// Make request
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("User-Agent", "test-agent")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	outputStr := buf.String()

	// Verify log contains expected fields
	assert.Contains(t, outputStr, "request completed")
	assert.Contains(t, outputStr, "gin-test-123")
	assert.Contains(t, outputStr, "GET")
	assert.Contains(t, outputStr, "/test")
	assert.Contains(t, outputStr, "200")
	assert.Contains(t, outputStr, "test-agent")
	assert.Contains(t, outputStr, "duration_ms")
	assert.Contains(t, outputStr, "client_ip")

	// Verify response
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestLogger_GinRecovery(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var buf bytes.Buffer
	logger := NewWithWriter("error", "json", &buf) // Use error level to capture panic logs
	middleware := logger.GinRecovery()
	assert.NotNil(t, middleware)

	// Create test router
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("request_id", "panic-test-456")
		c.Next()
	})
	router.Use(middleware)
	router.GET("/panic", func(c *gin.Context) {
		panic("test panic")
	})

	// Make request that will panic
	req := httptest.NewRequest("GET", "/panic", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	outputStr := buf.String()

	// Verify panic was logged
	assert.Contains(t, outputStr, "panic recovered")
	assert.Contains(t, outputStr, "panic-test-456")
	assert.Contains(t, outputStr, "test panic")
	assert.Contains(t, outputStr, "GET")
	assert.Contains(t, outputStr, "/panic")

	// Verify response
	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "Internal server error", response["error"])
}

func TestLogger_GinRecovery_NoRequestID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var buf bytes.Buffer
	logger := NewWithWriter("error", "json", &buf)
	middleware := logger.GinRecovery()

	// Create test router without setting request_id
	router := gin.New()
	router.Use(middleware)
	router.GET("/panic", func(c *gin.Context) {
		panic("test panic without request id")
	})

	// Make request that will panic
	req := httptest.NewRequest("GET", "/panic", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	outputStr := buf.String()

	// Verify panic was logged even without request_id
	assert.Contains(t, outputStr, "panic recovered")
	assert.Contains(t, outputStr, "test panic without request id")

	// Verify response
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestLogger_ChainedMethods(t *testing.T) {
	var buf bytes.Buffer
	logger := NewWithWriter("info", "json", &buf)

	// Test method chaining
	chainedLogger := logger.
		WithRequestID("chain-123").
		WithUserID("user-456").
		WithOperation("chained_test")

	assert.NotNil(t, chainedLogger)

	chainedLogger.Info("chained logger test")

	outputStr := buf.String()

	// Verify all chained values are present
	assert.Contains(t, outputStr, "chain-123")
	assert.Contains(t, outputStr, "user-456")
	assert.Contains(t, outputStr, "chained_test")
	assert.Contains(t, outputStr, "request_id")
	assert.Contains(t, outputStr, "user_id")
	assert.Contains(t, outputStr, "operation")
}

func TestLogger_DifferentLogLevels(t *testing.T) {
	testCases := []struct {
		name         string
		loggerLevel  string
		logMethod    func(*Logger)
		shouldAppear bool
	}{
		{
			name:         "debug logger with debug message",
			loggerLevel:  "debug",
			logMethod:    func(l *Logger) { l.Debug("debug message") },
			shouldAppear: true,
		},
		{
			name:         "info logger with debug message",
			loggerLevel:  "info",
			logMethod:    func(l *Logger) { l.Debug("debug message") },
			shouldAppear: false,
		},
		{
			name:         "info logger with info message",
			loggerLevel:  "info",
			logMethod:    func(l *Logger) { l.Info("info message") },
			shouldAppear: true,
		},
		{
			name:         "warn logger with info message",
			loggerLevel:  "warn",
			logMethod:    func(l *Logger) { l.Info("info message") },
			shouldAppear: false,
		},
		{
			name:         "error logger with warn message",
			loggerLevel:  "error",
			logMethod:    func(l *Logger) { l.Warn("warn message") },
			shouldAppear: false,
		},
		{
			name:         "error logger with error message",
			loggerLevel:  "error",
			logMethod:    func(l *Logger) { l.Error("error message") },
			shouldAppear: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := NewWithWriter(tc.loggerLevel, "json", &buf)

			tc.logMethod(logger)

			outputStr := buf.String()

			if tc.shouldAppear {
				assert.NotEmpty(t, outputStr, "Expected log message to appear")
			} else {
				assert.Empty(t, outputStr, "Expected log message to be filtered out")
			}
		})
	}
}

// Benchmark tests
func BenchmarkLogger_New(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = New("info", "json")
	}
}

func BenchmarkLogger_WithRequestID(b *testing.B) {
	logger := New("info", "json")
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = logger.WithRequestID("test-id")
	}
}

func BenchmarkLogger_Info(b *testing.B) {
	logger := New("info", "json")

	// Redirect to /dev/null to avoid I/O overhead in benchmark
	devNull, _ := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	originalStdout := os.Stdout
	os.Stdout = devNull
	defer func() {
		os.Stdout = originalStdout
		_ = devNull.Close()
	}()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("benchmark message", "key", "value")
	}
}

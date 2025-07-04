package logger

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type Logger struct {
	*slog.Logger
}

func New(level, format string) *Logger {
	var logLevel slog.Level
	switch strings.ToLower(level) {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	var handler slog.Handler
	opts := &slog.HandlerOptions{
		Level: logLevel,
	}

	switch strings.ToLower(format) {
	case "json":
		handler = slog.NewJSONHandler(os.Stdout, opts)
	case "text":
		handler = slog.NewTextHandler(os.Stdout, opts)
	default:
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}

	logger := slog.New(handler)
	return &Logger{Logger: logger}
}

func (l *Logger) WithRequestID(requestID string) *Logger {
	return &Logger{Logger: l.Logger.With("request_id", requestID)}
}

func (l *Logger) WithContext(ctx context.Context) *Logger {
	if requestID := ctx.Value("request_id"); requestID != nil {
		if reqIDStr, ok := requestID.(string); ok {
			return l.WithRequestID(reqIDStr)
		}
	}
	return l
}

func (l *Logger) WithUserID(userID string) *Logger {
	return &Logger{Logger: l.Logger.With("user_id", userID)}
}

func (l *Logger) WithOperation(operation string) *Logger {
	return &Logger{Logger: l.Logger.With("operation", operation)}
}

func (l *Logger) WithError(err error) *Logger {
	return &Logger{Logger: l.Logger.With("error", err.Error())}
}

func (l *Logger) GinLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		requestID := c.GetString("request_id")
		
		c.Next()

		duration := time.Since(start)
		
		l.WithRequestID(requestID).Info("request completed",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"duration_ms", duration.Milliseconds(),
			"user_agent", c.Request.UserAgent(),
			"client_ip", c.ClientIP(),
		)
	}
}

func (l *Logger) GinRecovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				requestID := c.GetString("request_id")
				l.WithRequestID(requestID).Error("panic recovered",
					"error", err,
					"method", c.Request.Method,
					"path", c.Request.URL.Path,
					"client_ip", c.ClientIP(),
				)
				c.JSON(500, gin.H{"error": "Internal server error"})
				c.Abort()
			}
		}()
		c.Next()
	}
}
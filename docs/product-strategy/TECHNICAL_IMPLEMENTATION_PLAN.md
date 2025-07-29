# VoidRunner Technical Implementation Plan
**Issue #11: Real-time Log Streaming Implementation**

**Date**: July 28, 2025  
**Technical Lead Consultation**: Complete  
**Priority**: CRITICAL (Blocks Epic 3)  
**Estimated Duration**: 7-8 weeks total

---

## Executive Summary

This plan addresses the critical Issue #11 (Real-time Log Streaming) that currently blocks Epic 3 frontend development. Based on technical lead consultation, we've identified a phased approach that prioritizes unblocking frontend development while establishing robust real-time infrastructure for future features.

**Key Technical Decisions:**
- **SSE over WebSocket** for MVP simplicity and HTTP compatibility
- **Redis pub/sub** for scalable real-time log distribution  
- **PostgreSQL partitioning** for high-performance log storage
- **Docker Compose** for streamlined self-hosted deployment

---

# Phase 1: Critical Dependencies Resolution (3-4 weeks)

## Week 1: Infrastructure Foundation

### Database Schema Design - Partitioned task_logs Table

**Migration File**: `005_task_logs_partitioned.up.sql`

```sql
-- Create partitioned task_logs table for performance at scale
CREATE TABLE task_logs (
    id BIGSERIAL,
    task_id UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    task_execution_id UUID REFERENCES task_executions(id) ON DELETE CASCADE,
    content TEXT NOT NULL,
    stream VARCHAR(10) NOT NULL CHECK (stream IN ('stdout', 'stderr')),
    sequence_number INTEGER NOT NULL, -- Order within execution
    timestamp TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    
    -- Composite primary key for partitioning
    PRIMARY KEY (id, created_at)
) PARTITION BY RANGE (created_at);

-- Create initial monthly partitions (auto-create via pg_partman or similar)
CREATE TABLE task_logs_2025_07 PARTITION OF task_logs
FOR VALUES FROM ('2025-07-01') TO ('2025-08-01');

CREATE TABLE task_logs_2025_08 PARTITION OF task_logs  
FOR VALUES FROM ('2025-08-01') TO ('2025-09-01');

-- Indexes for efficient queries
CREATE INDEX idx_task_logs_task_id_timestamp ON task_logs (task_id, timestamp DESC);
CREATE INDEX idx_task_logs_execution_id ON task_logs (task_execution_id, sequence_number);
CREATE INDEX idx_task_logs_content_gin ON task_logs USING GIN (to_tsvector('english', content));

-- User access control - users can only access logs for their tasks
CREATE POLICY task_logs_user_access ON task_logs
FOR ALL TO authenticated_users
USING (
    task_id IN (
        SELECT id FROM tasks WHERE user_id = current_setting('app.current_user_id')::UUID
    )
);

ALTER TABLE task_logs ENABLE ROW LEVEL SECURITY;
```

**Rollback Migration**: `005_task_logs_partitioned.down.sql`

```sql
DROP TABLE IF EXISTS task_logs CASCADE;
```

### Redis Configuration for Log Streaming

**Redis Pub/Sub Channels:**
- `task_logs:{task_id}` - Real-time log entries for specific task
- `task_status:{user_id}` - Task status updates for user dashboard
- `system_metrics` - System-wide metrics for admin dashboard

**Redis Configuration** (add to `config/development.yaml`):

```yaml
# Redis Configuration for Log Streaming
redis:
  # ... existing config ...
  pub_sub:
    enabled: true
    channels:
      task_logs_prefix: "voidrunner:logs:task:"
      task_status_prefix: "voidrunner:status:user:"
      system_metrics: "voidrunner:metrics:system"
  connection_pool:
    max_idle: 10
    max_active: 50
    idle_timeout: "5m"
```

### Log Aggregation Service Architecture

**New Go Package**: `internal/logging/`

```go
// internal/logging/interfaces.go
package logging

import (
    "context"
    "time"
)

type LogEntry struct {
    ID              int64                  `json:"id"`
    TaskID          string                 `json:"task_id"`
    TaskExecutionID *string               `json:"task_execution_id,omitempty"`
    Content         string                 `json:"content"`
    Stream          string                 `json:"stream"` // "stdout" or "stderr"
    SequenceNumber  int                    `json:"sequence_number"`
    Timestamp       time.Time              `json:"timestamp"`
    CreatedAt       time.Time              `json:"created_at"`
}

type LogCollector interface {
    CollectLogs(ctx context.Context, containerID, taskID string) error
    Stop() error
}

type LogPublisher interface {
    PublishLog(ctx context.Context, entry LogEntry) error
    Subscribe(ctx context.Context, taskID string) (<-chan LogEntry, error)
    Unsubscribe(taskID string, channel <-chan LogEntry) error
}

type LogRepository interface {
    CreateLog(ctx context.Context, entry LogEntry) error
    CreateLogsBatch(ctx context.Context, entries []LogEntry) error
    GetLogs(ctx context.Context, filter LogFilter) ([]LogEntry, error)
    GetLogsPaginated(ctx context.Context, filter LogFilter, cursor string, limit int) ([]LogEntry, string, error)
}

type LogFilter struct {
    TaskID          string
    TaskExecutionID *string
    Stream          string    // "", "stdout", "stderr"
    StartTime       *time.Time
    EndTime         *time.Time
    Search          string    // Full-text search
}
```

---

## Week 2: Core Streaming Pipeline

### Docker Log Collection Service

**File**: `internal/logging/docker_collector.go`

```go
package logging

import (
    "bufio"
    "context"
    "encoding/binary"
    "fmt"
    "io"
    "strings"
    "time"
    
    "github.com/docker/docker/api/types"
    "github.com/docker/docker/client"
    "github.com/voidrunnerhq/voidrunner/pkg/logger"
)

type DockerLogCollector struct {
    dockerClient *client.Client
    publisher    LogPublisher
    repository   LogRepository
    logger       *logger.Logger
    
    // Buffer for batching log entries to database
    logBuffer    []LogEntry
    bufferSize   int
    flushTicker  *time.Ticker
}

func NewDockerLogCollector(dockerClient *client.Client, publisher LogPublisher, repo LogRepository, logger *logger.Logger) *DockerLogCollector {
    return &DockerLogCollector{
        dockerClient: dockerClient,
        publisher:    publisher,
        repository:   repo,
        logger:       logger,
        bufferSize:   100, // Batch size for database writes
    }
}

func (dc *DockerLogCollector) CollectLogs(ctx context.Context, containerID, taskID string) error {
    logOptions := types.ContainerLogsOptions{
        ShowStdout:   true,
        ShowStderr:   true,
        Follow:       true,
        Timestamps:   true,
    }
    
    logs, err := dc.dockerClient.ContainerLogs(ctx, containerID, logOptions)
    if err != nil {
        return fmt.Errorf("failed to get container logs: %w", err)
    }
    defer logs.Close()
    
    // Start buffer flushing goroutine
    dc.startBufferFlushing(ctx)
    defer dc.stopBufferFlushing()
    
    return dc.processLogStream(ctx, logs, taskID)
}

func (dc *DockerLogCollector) processLogStream(ctx context.Context, logs io.ReadCloser, taskID string) error {
    scanner := bufio.NewScanner(logs)
    sequenceNumber := 0
    
    for scanner.Scan() {
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
        }
        
        logLine := scanner.Bytes()
        if len(logLine) < 8 {
            continue // Skip malformed log entries
        }
        
        // Parse Docker log format: 8-byte header + content
        stream := dc.parseDockerLogStream(logLine[0])
        timestamp := time.Now() // TODO: Parse from Docker timestamp
        content := strings.TrimSpace(string(logLine[8:]))
        
        if content == "" {
            continue // Skip empty log lines
        }
        
        entry := LogEntry{
            TaskID:         taskID,
            Content:        content,
            Stream:         stream,
            SequenceNumber: sequenceNumber,
            Timestamp:      timestamp,
            CreatedAt:      time.Now(),
        }
        
        // Publish to real-time subscribers immediately
        if err := dc.publisher.PublishLog(ctx, entry); err != nil {
            dc.logger.Warn("failed to publish log entry", "error", err, "task_id", taskID)
        }
        
        // Buffer for batch database insertion
        dc.bufferLogEntry(entry)
        sequenceNumber++
    }
    
    return scanner.Err()
}

func (dc *DockerLogCollector) parseDockerLogStream(header byte) string {
    switch header {
    case 1:
        return "stdout"
    case 2:
        return "stderr"
    default:
        return "stdout" // Default to stdout for unknown streams
    }
}

func (dc *DockerLogCollector) bufferLogEntry(entry LogEntry) {
    dc.logBuffer = append(dc.logBuffer, entry)
    
    if len(dc.logBuffer) >= dc.bufferSize {
        dc.flushLogBuffer()
    }
}

func (dc *DockerLogCollector) flushLogBuffer() {
    if len(dc.logBuffer) == 0 {
        return
    }
    
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    
    if err := dc.repository.CreateLogsBatch(ctx, dc.logBuffer); err != nil {
        dc.logger.Error("failed to flush log buffer to database", "error", err, "entries", len(dc.logBuffer))
        // TODO: Implement retry logic or dead letter queue
    } else {
        dc.logger.Debug("flushed log buffer to database", "entries", len(dc.logBuffer))
    }
    
    dc.logBuffer = dc.logBuffer[:0] // Reset buffer
}

func (dc *DockerLogCollector) startBufferFlushing(ctx context.Context) {
    dc.flushTicker = time.NewTicker(5 * time.Second)
    
    go func() {
        for {
            select {
            case <-ctx.Done():
                return
            case <-dc.flushTicker.C:
                dc.flushLogBuffer()
            }
        }
    }()
}

func (dc *DockerLogCollector) stopBufferFlushing() {
    if dc.flushTicker != nil {
        dc.flushTicker.Stop()
    }
    dc.flushLogBuffer() // Final flush
}
```

### Redis Pub/Sub Implementation

**File**: `internal/logging/redis_publisher.go`

```go
package logging

import (
    "context"
    "encoding/json"
    "fmt"
    "sync"
    
    "github.com/redis/go-redis/v9"
    "github.com/voidrunnerhq/voidrunner/pkg/logger"
)

type RedisLogPublisher struct {
    client      *redis.Client
    logger      *logger.Logger
    subscribers map[string][]chan LogEntry
    mutex       sync.RWMutex
    
    // Channel prefix for Redis pub/sub
    logChannelPrefix string
}

func NewRedisLogPublisher(client *redis.Client, logger *logger.Logger) *RedisLogPublisher {
    return &RedisLogPublisher{
        client:           client,
        logger:           logger,
        subscribers:      make(map[string][]chan LogEntry),
        logChannelPrefix: "voidrunner:logs:task:",
    }
}

func (rp *RedisLogPublisher) PublishLog(ctx context.Context, entry LogEntry) error {
    channelName := rp.logChannelPrefix + entry.TaskID
    
    data, err := json.Marshal(entry)
    if err != nil {
        return fmt.Errorf("failed to marshal log entry: %w", err)
    }
    
    // Publish to Redis channel
    if err := rp.client.Publish(ctx, channelName, data).Err(); err != nil {
        return fmt.Errorf("failed to publish to Redis channel %s: %w", channelName, err)
    }
    
    // Also publish to local subscribers (for same-process consumers)
    rp.publishToLocalSubscribers(entry)
    
    return nil
}

func (rp *RedisLogPublisher) Subscribe(ctx context.Context, taskID string) (<-chan LogEntry, error) {
    channelName := rp.logChannelPrefix + taskID
    subscriber := make(chan LogEntry, 100) // Buffered channel
    
    // Add to local subscribers
    rp.mutex.Lock()
    rp.subscribers[taskID] = append(rp.subscribers[taskID], subscriber)
    rp.mutex.Unlock()
    
    // Subscribe to Redis channel
    pubsub := rp.client.Subscribe(ctx, channelName)
    
    go func() {
        defer func() {
            pubsub.Close()
            close(subscriber)
            rp.removeLocalSubscriber(taskID, subscriber)
        }()
        
        for {
            select {
            case <-ctx.Done():
                return
            case msg := <-pubsub.Channel():
                if msg == nil {
                    return
                }
                
                var entry LogEntry
                if err := json.Unmarshal([]byte(msg.Payload), &entry); err != nil {
                    rp.logger.Warn("failed to unmarshal log entry from Redis", "error", err)
                    continue
                }
                
                select {
                case subscriber <- entry:
                case <-ctx.Done():
                    return
                default:
                    // Channel full, drop message
                    rp.logger.Warn("subscriber channel full, dropping log entry", "task_id", taskID)
                }
            }
        }
    }()
    
    return subscriber, nil
}

func (rp *RedisLogPublisher) Unsubscribe(taskID string, channel <-chan LogEntry) error {
    rp.removeLocalSubscriber(taskID, channel)
    return nil
}

func (rp *RedisLogPublisher) publishToLocalSubscribers(entry LogEntry) {
    rp.mutex.RLock()
    subscribers, exists := rp.subscribers[entry.TaskID]
    rp.mutex.RUnlock()
    
    if !exists {
        return
    }
    
    for _, subscriber := range subscribers {
        select {
        case subscriber <- entry:
        default:
            // Channel full, skip
        }
    }
}

func (rp *RedisLogPublisher) removeLocalSubscriber(taskID string, channel <-chan LogEntry) {
    rp.mutex.Lock()
    defer rp.mutex.Unlock()
    
    subscribers, exists := rp.subscribers[taskID]
    if !exists {
        return
    }
    
    // Remove the channel from subscribers list
    for i, sub := range subscribers {
        if sub == channel {
            rp.subscribers[taskID] = append(subscribers[:i], subscribers[i+1:]...)
            break
        }
    }
    
    // Clean up empty task entries
    if len(rp.subscribers[taskID]) == 0 {
        delete(rp.subscribers, taskID)
    }
}
```

---

## Week 3: SSE API Endpoints & Security

### Server-Sent Events Handler

**File**: `internal/api/handlers/logs.go`

```go
package handlers

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "strconv"
    "time"
    
    "github.com/gin-gonic/gin"
    "github.com/voidrunnerhq/voidrunner/internal/database"
    "github.com/voidrunnerhq/voidrunner/internal/logging"
    "github.com/voidrunnerhq/voidrunner/pkg/logger"
)

type LogHandler struct {
    taskRepo       database.TaskRepository
    logRepo        logging.LogRepository
    logPublisher   logging.LogPublisher
    logger         *logger.Logger
}

func NewLogHandler(taskRepo database.TaskRepository, logRepo logging.LogRepository, publisher logging.LogPublisher, logger *logger.Logger) *LogHandler {
    return &LogHandler{
        taskRepo:     taskRepo,
        logRepo:      logRepo,
        logPublisher: publisher,
        logger:       logger,
    }
}

// StreamTaskLogs streams real-time logs for a specific task via SSE
func (h *LogHandler) StreamTaskLogs(c *gin.Context) {
    taskID := c.Param("id")
    userID := c.GetString("user_id")
    
    // Verify task ownership
    task, err := h.taskRepo.GetByID(c.Request.Context(), taskID, userID)
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "Task not found"})
        return
    }
    
    // Set SSE headers
    c.Header("Content-Type", "text/event-stream")
    c.Header("Cache-Control", "no-cache")
    c.Header("Connection", "keep-alive")
    c.Header("Access-Control-Allow-Origin", "*")
    c.Header("Access-Control-Allow-Headers", "Cache-Control")
    
    // Send initial connection confirmation
    fmt.Fprintf(c.Writer, "data: {\"type\":\"connected\",\"task_id\":\"%s\"}\n\n", taskID)
    c.Writer.Flush()
    
    // Subscribe to real-time logs
    ctx, cancel := context.WithCancel(c.Request.Context())
    defer cancel()
    
    logChan, err := h.logPublisher.Subscribe(ctx, taskID)
    if err != nil {
        h.logger.Error("failed to subscribe to task logs", "error", err, "task_id", taskID)
        fmt.Fprintf(c.Writer, "data: {\"type\":\"error\",\"message\":\"Failed to subscribe to logs\"}\n\n")
        c.Writer.Flush()
        return
    }
    defer h.logPublisher.Unsubscribe(taskID, logChan)
    
    // Send historical logs first (optional - based on query param)
    if c.Query("include_history") == "true" {
        if err := h.sendHistoricalLogs(c, taskID); err != nil {
            h.logger.Warn("failed to send historical logs", "error", err, "task_id", taskID)
        }
    }
    
    // Stream real-time logs
    h.streamRealTimeLogs(c, logChan, taskID)
}

func (h *LogHandler) sendHistoricalLogs(c *gin.Context, taskID string) error {
    filter := logging.LogFilter{
        TaskID: taskID,
        // Limit to recent logs to avoid overwhelming the connection
    }
    
    logs, err := h.logRepo.GetLogs(c.Request.Context(), filter)
    if err != nil {
        return err
    }
    
    for _, entry := range logs {
        data, err := json.Marshal(map[string]interface{}{
            "type":            "log",
            "historical":      true,
            "content":         entry.Content,
            "stream":          entry.Stream,
            "sequence_number": entry.SequenceNumber,
            "timestamp":       entry.Timestamp,
        })
        if err != nil {
            h.logger.Warn("failed to marshal historical log entry", "error", err)
            continue
        }
        
        fmt.Fprintf(c.Writer, "data: %s\n\n", data)
        c.Writer.Flush()
    }
    
    return nil
}

func (h *LogHandler) streamRealTimeLogs(c *gin.Context, logChan <-chan logging.LogEntry, taskID string) {
    heartbeatTicker := time.NewTicker(30 * time.Second)
    defer heartbeatTicker.Stop()
    
    for {
        select {
        case <-c.Request.Context().Done():
            h.logger.Debug("SSE client disconnected", "task_id", taskID)
            return
            
        case entry, ok := <-logChan:
            if !ok {
                h.logger.Debug("log channel closed", "task_id", taskID)
                return
            }
            
            data, err := json.Marshal(map[string]interface{}{
                "type":            "log",
                "historical":      false,
                "content":         entry.Content,
                "stream":          entry.Stream,
                "sequence_number": entry.SequenceNumber,
                "timestamp":       entry.Timestamp,
            })
            if err != nil {
                h.logger.Warn("failed to marshal log entry", "error", err)
                continue
            }
            
            fmt.Fprintf(c.Writer, "data: %s\n\n", data)
            c.Writer.Flush()
            
        case <-heartbeatTicker.C:
            // Send heartbeat to keep connection alive
            fmt.Fprintf(c.Writer, "data: {\"type\":\"heartbeat\",\"timestamp\":\"%s\"}\n\n", time.Now().Format(time.RFC3339))
            c.Writer.Flush()
        }
    }
}

// GetTaskLogs returns historical task logs with pagination
func (h *LogHandler) GetTaskLogs(c *gin.Context) {
    taskID := c.Param("id")
    userID := c.GetString("user_id")
    
    // Verify task ownership
    _, err := h.taskRepo.GetByID(c.Request.Context(), taskID, userID)
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "Task not found"})
        return
    }
    
    // Parse query parameters
    filter := logging.LogFilter{
        TaskID: taskID,
        Stream: c.Query("stream"), // Optional: filter by stdout/stderr
    }
    
    // Parse time range
    if startStr := c.Query("start_time"); startStr != "" {
        if startTime, err := time.Parse(time.RFC3339, startStr); err == nil {
            filter.StartTime = &startTime
        }
    }
    
    if endStr := c.Query("end_time"); endStr != "" {
        if endTime, err := time.Parse(time.RFC3339, endStr); err == nil {
            filter.EndTime = &endTime
        }
    }
    
    if search := c.Query("search"); search != "" {
        filter.Search = search
    }
    
    // Pagination
    cursor := c.Query("cursor")
    limit := 100 // Default limit
    if limitStr := c.Query("limit"); limitStr != "" {
        if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 1000 {
            limit = l
        }
    }
    
    logs, nextCursor, err := h.logRepo.GetLogsPaginated(c.Request.Context(), filter, cursor, limit)
    if err != nil {
        h.logger.Error("failed to get task logs", "error", err, "task_id", taskID)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve logs"})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{
        "logs":        logs,
        "next_cursor": nextCursor,
        "has_more":    nextCursor != "",
    })
}
```

### API Route Registration

**Update**: `internal/api/routes/routes.go`

```go
// Add to setupRoutes function
func setupRoutes(router *gin.Engine, cfg *config.Config, log *logger.Logger, dbConn *database.Connection, repos *database.Repositories, authService *auth.Service, taskExecutionService *services.TaskExecutionService, taskExecutorService *services.TaskExecutorService, workerManager worker.WorkerManager) {
    // ... existing code ...
    
    // Initialize log publisher and repository
    redisClient := redis.NewClient(&redis.Options{
        Addr:     fmt.Sprintf("%s:%s", cfg.Redis.Host, cfg.Redis.Port),
        Password: cfg.Redis.Password,
        DB:       cfg.Redis.Database,
    })
    
    logPublisher := logging.NewRedisLogPublisher(redisClient, log.Logger)
    logRepo := logging.NewPostgreSQLRepository(dbConn, log.Logger)
    logHandler := handlers.NewLogHandler(repos.Tasks, logRepo, logPublisher, log.Logger)
    
    // Protected log endpoints
    protected.GET("/tasks/:id/logs", 
        taskRateLimit,
        logHandler.GetTaskLogs,
    )
    protected.GET("/tasks/:id/logs/stream",
        middleware.SSERateLimit(log.Logger), // Special rate limit for SSE
        logHandler.StreamTaskLogs,
    )
}
```

### Rate Limiting & Security for SSE

**File**: `internal/api/middleware/sse_rate_limit.go`

```go
package middleware

import (
    "fmt"
    "net/http"
    "sync"
    "time"
    
    "github.com/gin-gonic/gin"
    "github.com/voidrunnerhq/voidrunner/pkg/logger"
)

// SSE-specific rate limiting to prevent abuse of real-time connections
type SSELimiter struct {
    connections map[string]int       // user_id -> connection count
    lastCleanup time.Time
    mutex       sync.RWMutex
    maxConnections int
    cleanupInterval time.Duration
}

func NewSSELimiter() *SSELimiter {
    return &SSELimiter{
        connections:     make(map[string]int),
        maxConnections:  5,  // Max 5 concurrent SSE connections per user
        cleanupInterval: 5 * time.Minute,
    }
}

func SSERateLimit(logger *logger.Logger) gin.HandlerFunc {
    limiter := NewSSELimiter()
    
    return func(c *gin.Context) {
        userID := c.GetString("user_id")
        if userID == "" {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
            c.Abort()
            return
        }
        
        // Check connection limit
        limiter.mutex.Lock()
        currentConnections := limiter.connections[userID]
        if currentConnections >= limiter.maxConnections {
            limiter.mutex.Unlock()
            c.JSON(http.StatusTooManyRequests, gin.H{
                "error": fmt.Sprintf("Too many concurrent connections. Maximum %d allowed.", limiter.maxConnections),
            })
            c.Abort()
            return
        }
        
        // Increment connection count
        limiter.connections[userID]++
        limiter.mutex.Unlock()
        
        // Cleanup old entries periodically
        if time.Since(limiter.lastCleanup) > limiter.cleanupInterval {
            go limiter.cleanup()
        }
        
        // Set up cleanup when connection closes
        c.Header("X-Connection-ID", fmt.Sprintf("%s-%d", userID, time.Now().Unix()))
        
        defer func() {
            limiter.mutex.Lock()
            limiter.connections[userID]--
            if limiter.connections[userID] <= 0 {
                delete(limiter.connections, userID)
            }
            limiter.mutex.Unlock()
        }()
        
        c.Next()
    }
}

func (s *SSELimiter) cleanup() {
    s.mutex.Lock()
    defer s.mutex.Unlock()
    
    // Remove entries with 0 connections
    for userID, count := range s.connections {
        if count <= 0 {
            delete(s.connections, userID)
        }
    }
    
    s.lastCleanup = time.Now()
}
```

---

## Week 4: Testing & Performance Optimization

### Integration Tests

**File**: `tests/integration/log_streaming_test.go`

```go
package integration

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "net/http/httptest"
    "strings"
    "testing"
    "time"
    
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "github.com/voidrunnerhq/voidrunner/internal/logging"
    "github.com/voidrunnerhq/voidrunner/tests/testutil"
)

func TestLogStreamingIntegration(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test in short mode")
    }
    
    suite := testutil.NewTestSuite(t)
    defer suite.Cleanup()
    
    // Create test user and task
    user := suite.CreateTestUser()
    task := suite.CreateTestTask(user.ID, map[string]interface{}{
        "name":        "log-streaming-test",
        "language":    "python",
        "code":        "print('test log line')",
    })
    
    t.Run("SSE Log Streaming", func(t *testing.T) {
        // Start SSE connection
        url := fmt.Sprintf("/api/v1/tasks/%s/logs/stream", task.ID)
        req := httptest.NewRequest("GET", url, nil)
        req.Header.Set("Authorization", "Bearer "+user.AccessToken)
        
        recorder := httptest.NewRecorder()
        
        // Process request in goroutine
        go func() {
            suite.Router.ServeHTTP(recorder, req)
        }()
        
        // Wait for SSE headers
        time.Sleep(100 * time.Millisecond)
        
        // Verify SSE headers
        assert.Equal(t, "text/event-stream", recorder.Header().Get("Content-Type"))
        assert.Equal(t, "no-cache", recorder.Header().Get("Cache-Control"))
        assert.Equal(t, "keep-alive", recorder.Header().Get("Connection"))
        
        // Simulate log generation
        logEntry := logging.LogEntry{
            TaskID:         task.ID,
            Content:        "test log message",
            Stream:         "stdout",
            SequenceNumber: 1,
            Timestamp:      time.Now(),
        }
        
        // Publish log entry
        err := suite.LogPublisher.PublishLog(context.Background(), logEntry)
        require.NoError(t, err)
        
        // Wait for log to be streamed
        time.Sleep(200 * time.Millisecond)
        
        // Verify log was streamed
        responseBody := recorder.Body.String()
        assert.Contains(t, responseBody, "test log message")
        assert.Contains(t, responseBody, "\"stream\":\"stdout\"")
    })
    
    t.Run("Historical Log Retrieval", func(t *testing.T) {
        // Insert test logs
        testLogs := []logging.LogEntry{
            {
                TaskID:         task.ID,
                Content:        "log line 1",
                Stream:         "stdout",
                SequenceNumber: 1,
                Timestamp:      time.Now().Add(-5 * time.Minute),
            },
            {
                TaskID:         task.ID,
                Content:        "error line 1",
                Stream:         "stderr",
                SequenceNumber: 2,
                Timestamp:      time.Now().Add(-4 * time.Minute),
            },
        }
        
        for _, entry := range testLogs {
            err := suite.LogRepo.CreateLog(context.Background(), entry)
            require.NoError(t, err)
        }
        
        // Test log retrieval
        url := fmt.Sprintf("/api/v1/tasks/%s/logs", task.ID)
        req := httptest.NewRequest("GET", url, nil)
        req.Header.Set("Authorization", "Bearer "+user.AccessToken)
        
        recorder := httptest.NewRecorder()
        suite.Router.ServeHTTP(recorder, req)
        
        assert.Equal(t, http.StatusOK, recorder.Code)
        
        var response map[string]interface{}
        err := json.Unmarshal(recorder.Body.Bytes(), &response)
        require.NoError(t, err)
        
        logs, ok := response["logs"].([]interface{})
        require.True(t, ok)
        assert.Len(t, logs, 2)
    })
    
    t.Run("Log Stream Filtering", func(t *testing.T) {
        // Test stderr filtering
        url := fmt.Sprintf("/api/v1/tasks/%s/logs?stream=stderr", task.ID)
        req := httptest.NewRequest("GET", url, nil)
        req.Header.Set("Authorization", "Bearer "+user.AccessToken)
        
        recorder := httptest.NewRecorder()
        suite.Router.ServeHTTP(recorder, req)
        
        assert.Equal(t, http.StatusOK, recorder.Code)
        
        var response map[string]interface{}
        err := json.Unmarshal(recorder.Body.Bytes(), &response)
        require.NoError(t, err)
        
        logs, ok := response["logs"].([]interface{})
        require.True(t, ok)
        
        // Should only contain stderr logs
        for _, logEntry := range logs {
            logMap := logEntry.(map[string]interface{})
            assert.Equal(t, "stderr", logMap["stream"])
        }
    })
}

func TestLogStreamingPerformance(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping performance test in short mode")
    }
    
    suite := testutil.NewTestSuite(t)
    defer suite.Cleanup()
    
    user := suite.CreateTestUser()
    task := suite.CreateTestTask(user.ID, map[string]interface{}{
        "name":     "performance-test",
        "language": "python",
        "code":     "for i in range(1000): print(f'log line {i}')",
    })
    
    t.Run("High Volume Log Streaming", func(t *testing.T) {
        start := time.Now()
        
        // Generate 1000 log entries
        for i := 0; i < 1000; i++ {
            entry := logging.LogEntry{
                TaskID:         task.ID,
                Content:        fmt.Sprintf("log line %d", i),
                Stream:         "stdout",
                SequenceNumber: i,
                Timestamp:      time.Now(),
            }
            
            err := suite.LogPublisher.PublishLog(context.Background(), entry)
            require.NoError(t, err)
        }
        
        duration := time.Since(start)
        
        // Should handle 1000 log entries in under 5 seconds
        assert.Less(t, duration, 5*time.Second, "High volume log publishing took too long")
        
        // Verify logs were persisted
        filter := logging.LogFilter{TaskID: task.ID}
        logs, err := suite.LogRepo.GetLogs(context.Background(), filter)
        require.NoError(t, err)
        assert.GreaterOrEqual(t, len(logs), 1000)
    })
    
    t.Run("Concurrent SSE Connections", func(t *testing.T) {
        // Test multiple concurrent SSE connections
        const numConnections = 10
        
        var connections []*httptest.ResponseRecorder
        
        for i := 0; i < numConnections; i++ {
            url := fmt.Sprintf("/api/v1/tasks/%s/logs/stream", task.ID)
            req := httptest.NewRequest("GET", url, nil)
            req.Header.Set("Authorization", "Bearer "+user.AccessToken)
            
            recorder := httptest.NewRecorder()
            connections = append(connections, recorder)
            
            go func(rec *httptest.ResponseRecorder) {
                suite.Router.ServeHTTP(rec, req)
            }(recorder)
        }
        
        // Wait for connections to establish
        time.Sleep(500 * time.Millisecond)
        
        // Publish a log entry
        entry := logging.LogEntry{
            TaskID:         task.ID,
            Content:        "concurrent test message",
            Stream:         "stdout",
            SequenceNumber: 1,
            Timestamp:      time.Now(),
        }
        
        err := suite.LogPublisher.PublishLog(context.Background(), entry)
        require.NoError(t, err)
        
        // Wait for propagation
        time.Sleep(500 * time.Millisecond)
        
        // Verify all connections received the message
        for i, recorder := range connections {
            responseBody := recorder.Body.String()
            assert.Contains(t, responseBody, "concurrent test message", 
                "Connection %d did not receive log message", i)
        }
    })
}
```

### Performance Benchmarks

**File**: `internal/logging/benchmark_test.go`

```go
package logging_test

import (
    "context"
    "testing"
    "time"
    
    "github.com/voidrunnerhq/voidrunner/internal/logging"
    "github.com/voidrunnerhq/voidrunner/tests/testutil"
)

func BenchmarkLogPublishing(b *testing.B) {
    suite := testutil.NewTestSuite(b)
    defer suite.Cleanup()
    
    entry := logging.LogEntry{
        TaskID:         "benchmark-task-id",
        Content:        "benchmark log message",
        Stream:         "stdout",
        SequenceNumber: 1,
        Timestamp:      time.Now(),
    }
    
    b.ResetTimer()
    
    for i := 0; i < b.N; i++ {
        err := suite.LogPublisher.PublishLog(context.Background(), entry)
        if err != nil {
            b.Fatalf("Failed to publish log: %v", err)
        }
    }
}

func BenchmarkLogBatchInsertion(b *testing.B) {
    suite := testutil.NewTestSuite(b)
    defer suite.Cleanup()
    
    // Prepare batch of log entries
    batchSize := 100
    entries := make([]logging.LogEntry, batchSize)
    
    for i := 0; i < batchSize; i++ {
        entries[i] = logging.LogEntry{
            TaskID:         "benchmark-task-id",
            Content:        "benchmark log message",
            Stream:         "stdout",
            SequenceNumber: i,
            Timestamp:      time.Now(),
        }
    }
    
    b.ResetTimer()
    
    for i := 0; i < b.N; i++ {
        err := suite.LogRepo.CreateLogsBatch(context.Background(), entries)
        if err != nil {
            b.Fatalf("Failed to insert log batch: %v", err)
        }
    }
}

func BenchmarkLogRetrieval(b *testing.B) {
    suite := testutil.NewTestSuite(b)
    defer suite.Cleanup()
    
    // Insert test data
    taskID := "benchmark-task-id"
    for i := 0; i < 1000; i++ {
        entry := logging.LogEntry{
            TaskID:         taskID,
            Content:        "benchmark log message",
            Stream:         "stdout",
            SequenceNumber: i,
            Timestamp:      time.Now(),
        }
        
        err := suite.LogRepo.CreateLog(context.Background(), entry)
        if err != nil {
            b.Fatalf("Failed to insert test log: %v", err)
        }
    }
    
    filter := logging.LogFilter{TaskID: taskID}
    
    b.ResetTimer()
    
    for i := 0; i < b.N; i++ {
        _, err := suite.LogRepo.GetLogs(context.Background(), filter)
        if err != nil {
            b.Fatalf("Failed to retrieve logs: %v", err)
        }
    }
}
```

---

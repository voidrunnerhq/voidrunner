package reporting

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/voidrunnerhq/voidrunner/internal/executor"
)

// Mock notification handler for testing
type MockNotificationHandler struct {
	notifications []*ErrorNotification
	shouldFail    bool
}

func (m *MockNotificationHandler) SendNotification(ctx context.Context, notification *ErrorNotification) error {
	if m.shouldFail {
		return assert.AnError
	}
	m.notifications = append(m.notifications, notification)
	return nil
}

func TestErrorAggregator(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	t.Run("NewErrorAggregator", func(t *testing.T) {
		config := DefaultErrorAggregatorConfig()
		aggregator := NewErrorAggregator(config, logger)
		
		assert.NotNil(t, aggregator)
		assert.Equal(t, config, aggregator.config)
		
		// Cleanup
		aggregator.Stop(context.Background())
	})

	t.Run("RecordError", func(t *testing.T) {
		config := DefaultErrorAggregatorConfig()
		aggregator := NewErrorAggregator(config, logger)
		defer func() { _ = aggregator.Stop(context.Background()) }()
		
		ctx := context.Background()
		execError := &executor.ExecutionError{
			Type:        executor.ErrorTypeTimeout,
			Code:        "TIMEOUT_001",
			Message:     "Operation timed out",
			Retryable:   true,
			TaskID:      "task-123",
			ExecutionID: "exec-456",
			Timestamp:   time.Now(),
			Context:     map[string]interface{}{"timeout": "30s"},
		}
		
		aggregator.RecordError(ctx, execError, "task-123", "user-456")
		
		// Verify error was recorded
		stats := aggregator.GetStats()
		assert.Equal(t, 1, stats["total_error_types"])
		assert.Equal(t, 1, stats["total_error_codes"])
		assert.Equal(t, 1, stats["recent_errors_count"])
	})

	t.Run("GenerateReport", func(t *testing.T) {
		config := DefaultErrorAggregatorConfig()
		aggregator := NewErrorAggregator(config, logger)
		defer func() { _ = aggregator.Stop(context.Background()) }()
		
		ctx := context.Background()
		
		// Record some errors
		for i := 0; i < 5; i++ {
			execError := &executor.ExecutionError{
				Type:        executor.ErrorTypeTimeout,
				Code:        "TIMEOUT_001",
				Message:     "Operation timed out",
				Retryable:   true,
				TaskID:      "task-123",
				ExecutionID: "exec-456",
				Timestamp:   time.Now(),
				Context:     map[string]interface{}{"timeout": "30s"},
			}
			aggregator.RecordError(ctx, execError, "task-123", "user-456")
		}
		
		// Generate report
		timeRange := TimeRange{
			Start: time.Now().Add(-1 * time.Hour),
			End:   time.Now(),
		}
		
		report, err := aggregator.GenerateReport(ctx, timeRange)
		require.NoError(t, err)
		assert.NotNil(t, report)
		assert.NotEmpty(t, report.ID)
		assert.Equal(t, int64(5), report.TotalErrors)
		assert.Equal(t, 1, report.UniqueErrors)
		assert.Len(t, report.ErrorsByType, 1)
		assert.Len(t, report.ErrorsByCode, 1)
	})

	t.Run("ExportData", func(t *testing.T) {
		config := DefaultErrorAggregatorConfig()
		aggregator := NewErrorAggregator(config, logger)
		defer func() { _ = aggregator.Stop(context.Background()) }()
		
		ctx := context.Background()
		
		data, err := aggregator.ExportData(ctx, "json")
		require.NoError(t, err)
		assert.NotEmpty(t, data)
		
		// Test invalid format
		_, err = aggregator.ExportData(ctx, "invalid")
		assert.Error(t, err)
	})
}

func TestReportingService(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	t.Run("NewReportingService", func(t *testing.T) {
		config := DefaultReportingServiceConfig()
		service, err := NewReportingService(config, logger)
		
		require.NoError(t, err)
		assert.NotNil(t, service)
		assert.Equal(t, config, service.config)
		
		// Cleanup
		service.Stop(context.Background())
	})

	t.Run("RecordError", func(t *testing.T) {
		config := DefaultReportingServiceConfig()
		service, err := NewReportingService(config, logger)
		require.NoError(t, err)
		defer func() { _ = service.Stop(context.Background()) }()
		
		ctx := context.Background()
		execError := &executor.ExecutionError{
			Type:        executor.ErrorTypeValidation,
			Code:        "VALIDATION_001",
			Message:     "Invalid input",
			Retryable:   false,
			TaskID:      "task-123",
			ExecutionID: "exec-456",
			Timestamp:   time.Now(),
		}
		
		err = service.RecordError(ctx, execError, "task-123", "user-456")
		assert.NoError(t, err)
		
		// Test nil error
		err = service.RecordError(ctx, nil, "task-123", "user-456")
		assert.Error(t, err)
	})

	t.Run("GenerateReport", func(t *testing.T) {
		config := DefaultReportingServiceConfig()
		service, err := NewReportingService(config, logger)
		require.NoError(t, err)
		defer func() { _ = service.Stop(context.Background()) }()
		
		ctx := context.Background()
		
		// Record some errors
		for i := 0; i < 10; i++ {
			execError := &executor.ExecutionError{
				Type:        executor.ErrorTypeResource,
				Code:        "RESOURCE_001",
				Message:     "Resource limit exceeded",
				Retryable:   true,
				TaskID:      "task-123",
				ExecutionID: "exec-456",
				Timestamp:   time.Now(),
			}
			service.RecordError(ctx, execError, "task-123", "user-456")
		}
		
		// Wait a bit for aggregation
		time.Sleep(100 * time.Millisecond)
		
		timeRange := TimeRange{
			Start: time.Now().Add(-1 * time.Hour),
			End:   time.Now(),
		}
		
		report, err := service.GenerateReport(ctx, timeRange)
		require.NoError(t, err)
		assert.NotNil(t, report)
		assert.True(t, report.TotalErrors > 0)
	})

	t.Run("QuickReport", func(t *testing.T) {
		config := DefaultReportingServiceConfig()
		service, err := NewReportingService(config, logger)
		require.NoError(t, err)
		defer func() { _ = service.Stop(context.Background()) }()
		
		ctx := context.Background()
		
		report, err := service.GenerateQuickReport(ctx)
		require.NoError(t, err)
		assert.NotNil(t, report)
	})

	t.Run("ReportHistory", func(t *testing.T) {
		config := DefaultReportingServiceConfig()
		service, err := NewReportingService(config, logger)
		require.NoError(t, err)
		defer func() { _ = service.Stop(context.Background()) }()
		
		ctx := context.Background()
		
		// Generate a report
		timeRange := TimeRange{
			Start: time.Now().Add(-1 * time.Hour),
			End:   time.Now(),
		}
		
		report, err := service.GenerateReport(ctx, timeRange)
		require.NoError(t, err)
		
		// Check history
		history := service.GetReportHistory(10)
		assert.Len(t, history, 1)
		assert.Equal(t, report.ID, history[0].ID)
		
		// Test GetReportByID
		retrievedReport, err := service.GetReportByID(report.ID)
		require.NoError(t, err)
		assert.Equal(t, report.ID, retrievedReport.ID)
		
		// Test non-existent report
		_, err = service.GetReportByID("non-existent")
		assert.Error(t, err)
	})

	t.Run("NotificationHandlers", func(t *testing.T) {
		config := DefaultReportingServiceConfig()
		service, err := NewReportingService(config, logger)
		require.NoError(t, err)
		defer func() { _ = service.Stop(context.Background()) }()
		
		mockHandler := &MockNotificationHandler{}
		
		// Add handler
		service.AddNotificationHandler(mockHandler)
		
		// Record a security error to trigger notification
		ctx := context.Background()
		execError := &executor.ExecutionError{
			Type:        executor.ErrorTypeSecurity,
			Code:        "SECURITY_001",
			Message:     "Unauthorized access attempt",
			Retryable:   false,
			TaskID:      "task-123",
			ExecutionID: "exec-456",
			Timestamp:   time.Now(),
		}
		
		err = service.RecordError(ctx, execError, "task-123", "user-456")
		assert.NoError(t, err)
		
		// Wait for notification
		time.Sleep(100 * time.Millisecond)
		
		// Check that notification was sent
		assert.Len(t, mockHandler.notifications, 1)
		assert.Equal(t, "critical", mockHandler.notifications[0].Severity)
		
		// Remove handler
		service.RemoveNotificationHandler(mockHandler)
	})

	t.Run("ExportReports", func(t *testing.T) {
		config := DefaultReportingServiceConfig()
		service, err := NewReportingService(config, logger)
		require.NoError(t, err)
		defer func() { _ = service.Stop(context.Background()) }()
		
		ctx := context.Background()
		
		// Export with no reports
		data, err := service.ExportReports(ctx, "json", nil)
		require.NoError(t, err)
		assert.NotEmpty(t, data)
		
		// Test invalid format
		_, err = service.ExportReports(ctx, "invalid", nil)
		assert.Error(t, err)
	})

	t.Run("GetStats", func(t *testing.T) {
		config := DefaultReportingServiceConfig()
		service, err := NewReportingService(config, logger)
		require.NoError(t, err)
		defer func() { _ = service.Stop(context.Background()) }()
		
		stats := service.GetStats()
		assert.NotNil(t, stats)
		assert.Contains(t, stats, "report_history_count")
		assert.Contains(t, stats, "persisted_reports_count")
		assert.Contains(t, stats, "notification_handlers")
		assert.Contains(t, stats, "aggregator_stats")
	})
}

func TestNotificationHandlers(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	t.Run("LogNotificationHandler", func(t *testing.T) {
		handler := NewLogNotificationHandler(logger, slog.LevelInfo)
		
		notification := &ErrorNotification{
			ID:         "test-notification",
			Timestamp:  time.Now(),
			Severity:   "high",
			Title:      "Test Notification",
			Message:    "This is a test notification",
			ErrorCount: 5,
		}
		
		err := handler.SendNotification(context.Background(), notification)
		assert.NoError(t, err)
	})

	t.Run("WebhookNotificationHandler", func(t *testing.T) {
		config := &WebhookConfig{
			URL:     "https://httpbin.org/post",
			Timeout: 10 * time.Second,
			Headers: map[string]string{
				"Authorization": "Bearer test-token",
			},
		}
		
		handler := NewWebhookNotificationHandler(config, logger)
		require.NotNil(t, handler)
		
		notification := &ErrorNotification{
			ID:         "test-notification",
			Timestamp:  time.Now(),
			Severity:   "medium",
			Title:      "Test Webhook Notification",
			Message:    "This is a test webhook notification",
			ErrorCount: 3,
		}
		
		// Verify notification is properly structured
		assert.Equal(t, "test-notification", notification.ID)
		assert.Equal(t, "medium", notification.Severity)
		
		// This will actually make an HTTP request to httpbin.org
		// Comment out if you don't want external requests in tests
		// err := handler.SendNotification(context.Background(), notification)
		// assert.NoError(t, err)
	})

	t.Run("FileNotificationHandler", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "voidrunner-notifications-*")
		require.NoError(t, err)
		defer func() { _ = os.RemoveAll(tempDir) }()
		
		handler, err := NewFileNotificationHandler(tempDir, logger)
		require.NoError(t, err)
		
		notification := &ErrorNotification{
			ID:         "test-notification",
			Timestamp:  time.Now(),
			Severity:   "low",
			Title:      "Test File Notification",
			Message:    "This is a test file notification",
			ErrorCount: 1,
			TimeRange: TimeRange{
				Start: time.Now().Add(-1 * time.Hour),
				End:   time.Now(),
			},
		}
		
		err = handler.SendNotification(context.Background(), notification)
		assert.NoError(t, err)
		
		// Check that file was created
		files, err := os.ReadDir(tempDir)
		require.NoError(t, err)
		assert.Len(t, files, 1)
	})

	t.Run("CompositeNotificationHandler", func(t *testing.T) {
		mockHandler1 := &MockNotificationHandler{}
		mockHandler2 := &MockNotificationHandler{}
		
		handlers := []NotificationHandler{mockHandler1, mockHandler2}
		composite := NewCompositeNotificationHandler(handlers, logger)
		
		notification := &ErrorNotification{
			ID:         "test-notification",
			Timestamp:  time.Now(),
			Severity:   "high",
			Title:      "Test Composite Notification",
			Message:    "This is a test composite notification",
			ErrorCount: 8,
		}
		
		err := composite.SendNotification(context.Background(), notification)
		assert.NoError(t, err)
		
		// Check that both handlers received the notification
		assert.Len(t, mockHandler1.notifications, 1)
		assert.Len(t, mockHandler2.notifications, 1)
		
		// Test with failing handler
		mockHandler2.shouldFail = true
		err = composite.SendNotification(context.Background(), notification)
		assert.Error(t, err)
	})
}

func TestDefaultConfigurations(t *testing.T) {
	t.Run("DefaultErrorAggregatorConfig", func(t *testing.T) {
		config := DefaultErrorAggregatorConfig()
		
		assert.NotNil(t, config)
		assert.True(t, config.ShortTermWindow > 0)
		assert.True(t, config.MediumTermWindow > 0)
		assert.True(t, config.LongTermWindow > 0)
		assert.True(t, config.MaxErrorsPerType > 0)
		assert.True(t, config.MaxContextKeys > 0)
		assert.True(t, config.RetentionPeriod > 0)
		assert.True(t, config.MinSamplesForTrend > 0)
		assert.True(t, config.TrendThreshold > 0)
		assert.True(t, config.EnableAnomalyDetection)
		assert.True(t, config.AnomalyThreshold > 0)
		assert.True(t, config.AutoReportInterval > 0)
		assert.True(t, config.MaxRecommendations > 0)
	})

	t.Run("DefaultReportingServiceConfig", func(t *testing.T) {
		config := DefaultReportingServiceConfig()
		
		assert.NotNil(t, config)
		assert.NotNil(t, config.AggregatorConfig)
		assert.True(t, config.EnablePersistence)
		assert.NotEmpty(t, config.PersistencePath)
		assert.True(t, config.MaxReportHistory > 0)
		assert.True(t, config.EnableNotifications)
		assert.True(t, config.NotificationThreshold > 0)
		assert.True(t, config.NotificationInterval > 0)
		assert.True(t, config.MaxConcurrentReports > 0)
		assert.True(t, config.ReportTimeout > 0)
	})
}

func TestErrorMetricsAndReports(t *testing.T) {
	t.Run("ErrorMetricsAggregation", func(t *testing.T) {
		logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
		config := DefaultErrorAggregatorConfig()
		aggregator := NewErrorAggregator(config, logger)
		defer func() { _ = aggregator.Stop(context.Background()) }()
		
		ctx := context.Background()
		
		// Record errors with different patterns
		errorTypes := []executor.ErrorType{
			executor.ErrorTypeTimeout,
			executor.ErrorTypeResource,
			executor.ErrorTypeValidation,
		}
		
		for i := 0; i < 30; i++ {
			errorType := errorTypes[i%len(errorTypes)]
			execError := &executor.ExecutionError{
				Type:        errorType,
				Code:        fmt.Sprintf("%s_%03d", errorType, i%5),
				Message:     fmt.Sprintf("Error %d", i),
				Retryable:   true,
				TaskID:      fmt.Sprintf("task-%d", i%10),
				ExecutionID: fmt.Sprintf("exec-%d", i),
				Timestamp:   time.Now().Add(-time.Duration(i) * time.Minute),
				Context:     map[string]interface{}{"index": i},
			}
			
			aggregator.RecordError(ctx, execError, execError.TaskID, fmt.Sprintf("user-%d", i%5))
		}
		
		// Generate comprehensive report
		timeRange := TimeRange{
			Start: time.Now().Add(-2 * time.Hour),
			End:   time.Now(),
		}
		
		report, err := aggregator.GenerateReport(ctx, timeRange)
		require.NoError(t, err)
		
		// Verify report completeness
		assert.Equal(t, int64(30), report.TotalErrors)
		assert.Equal(t, 3, len(report.ErrorsByType))
		assert.True(t, len(report.ErrorsByCode) >= 3)
		assert.True(t, len(report.TopErrors) > 0)
		assert.NotEmpty(t, report.Recommendations)
		
		// Verify trend analysis
		assert.NotEmpty(t, report.TrendAnalysis.OverallTrend)
		assert.True(t, report.TrendAnalysis.PeakHour >= 0 && report.TrendAnalysis.PeakHour <= 23)
		
		// Verify recommendations contain high-priority items for resource errors
		hasResourceRecommendation := false
		for _, rec := range report.Recommendations {
			if rec.Category == "performance" && rec.Priority == "high" {
				hasResourceRecommendation = true
				break
			}
		}
		assert.True(t, hasResourceRecommendation)
	})

	t.Run("ReportGenerationPerformance", func(t *testing.T) {
		logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))
		config := DefaultReportingServiceConfig()
		config.ReportTimeout = 5 * time.Second
		
		service, err := NewReportingService(config, logger)
		require.NoError(t, err)
		defer func() { _ = service.Stop(context.Background()) }()
		
		ctx := context.Background()
		
		// Record many errors
		for i := 0; i < 1000; i++ {
			execError := &executor.ExecutionError{
				Type:        executor.ErrorTypeTimeout,
				Code:        fmt.Sprintf("TIMEOUT_%03d", i%10),
				Message:     fmt.Sprintf("Timeout error %d", i),
				Retryable:   true,
				TaskID:      fmt.Sprintf("task-%d", i),
				ExecutionID: fmt.Sprintf("exec-%d", i),
				Timestamp:   time.Now(),
			}
			
			service.RecordError(ctx, execError, execError.TaskID, fmt.Sprintf("user-%d", i%100))
		}
		
		// Measure report generation time
		start := time.Now()
		timeRange := TimeRange{
			Start: time.Now().Add(-1 * time.Hour),
			End:   time.Now(),
		}
		
		report, err := service.GenerateReport(ctx, timeRange)
		generationTime := time.Since(start)
		
		require.NoError(t, err)
		assert.NotNil(t, report)
		assert.True(t, generationTime < 5*time.Second, "Report generation took too long: %v", generationTime)
		
		t.Logf("Generated report with %d errors in %v", report.TotalErrors, generationTime)
	})
}

func TestConcurrentOperations(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))
	config := DefaultReportingServiceConfig()
	config.MaxConcurrentReports = 2
	
	service, err := NewReportingService(config, logger)
	require.NoError(t, err)
	defer service.Stop(context.Background())
	
	ctx := context.Background()
	
	// Record some errors
	for i := 0; i < 50; i++ {
		execError := &executor.ExecutionError{
			Type:        executor.ErrorTypeNetwork,
			Code:        "NETWORK_001",
			Message:     "Network error",
			Retryable:   true,
			TaskID:      "task-123",
			ExecutionID: "exec-456",
			Timestamp:   time.Now(),
		}
		
		service.RecordError(ctx, execError, "task-123", "user-456")
	}
	
	// Test concurrent report generation
	timeRange := TimeRange{
		Start: time.Now().Add(-1 * time.Hour),
		End:   time.Now(),
	}
	
	// Start multiple report generations concurrently
	type reportResult struct {
		report *ErrorReport
		err    error
	}
	
	results := make(chan reportResult, 5)
	
	for i := 0; i < 5; i++ {
		go func() {
			report, err := service.GenerateReport(ctx, timeRange)
			results <- reportResult{report: report, err: err}
		}()
	}
	
	// Collect results
	successCount := 0
	limitExceededCount := 0
	
	for i := 0; i < 5; i++ {
		result := <-results
		if result.err == nil {
			successCount++
			assert.NotNil(t, result.report)
		} else {
			if result.err.Error() == "max concurrent reports limit reached: 2" {
				limitExceededCount++
			}
		}
	}
	
	// Should have some successful reports and some that hit the limit
	assert.True(t, successCount > 0, "Expected at least some successful reports")
	assert.True(t, limitExceededCount > 0, "Expected some reports to hit the concurrent limit")
	
	t.Logf("Successful reports: %d, Limited reports: %d", successCount, limitExceededCount)
}
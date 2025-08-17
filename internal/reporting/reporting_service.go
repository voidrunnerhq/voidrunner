package reporting

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/VoidRunnerHQ/voidrunner/internal/executor"
)

// ReportingServiceConfig defines configuration for the reporting service
type ReportingServiceConfig struct {
	// Error aggregation settings
	AggregatorConfig  *ErrorAggregatorConfig `json:"aggregator_config"`
	
	// Report persistence settings
	EnablePersistence bool          `json:"enable_persistence"`
	PersistencePath   string        `json:"persistence_path"`
	MaxReportHistory  int           `json:"max_report_history"`
	
	// Notification settings
	EnableNotifications   bool          `json:"enable_notifications"`
	NotificationThreshold int64         `json:"notification_threshold"` // Error count threshold
	NotificationInterval  time.Duration `json:"notification_interval"`
	
	// Performance settings
	MaxConcurrentReports int `json:"max_concurrent_reports"`
	ReportTimeout        time.Duration `json:"report_timeout"`
}

// DefaultReportingServiceConfig returns sensible defaults
func DefaultReportingServiceConfig() *ReportingServiceConfig {
	return &ReportingServiceConfig{
		AggregatorConfig:      DefaultErrorAggregatorConfig(),
		EnablePersistence:     true,
		PersistencePath:       "./reports",
		MaxReportHistory:      100,
		EnableNotifications:   true,
		NotificationThreshold: 50,
		NotificationInterval:  15 * time.Minute,
		MaxConcurrentReports:  5,
		ReportTimeout:        30 * time.Second,
	}
}

// NotificationHandler defines the interface for handling error notifications
type NotificationHandler interface {
	SendNotification(ctx context.Context, notification *ErrorNotification) error
}

// ErrorNotification represents an error notification
type ErrorNotification struct {
	ID          string             `json:"id"`
	Timestamp   time.Time          `json:"timestamp"`
	Severity    string             `json:"severity"`
	Title       string             `json:"title"`
	Message     string             `json:"message"`
	ErrorCount  int64              `json:"error_count"`
	TimeRange   TimeRange          `json:"time_range"`
	TopErrors   []ErrorSummary     `json:"top_errors"`
	Recommendations []Recommendation `json:"recommendations"`
}

// ReportingService provides comprehensive error reporting capabilities
type ReportingService struct {
	mu                    sync.RWMutex
	config                *ReportingServiceConfig
	logger                *slog.Logger
	aggregator            *ErrorAggregator
	notificationHandlers  []NotificationHandler
	
	// Report storage
	reportHistory         []*ErrorReport
	persistedReports      map[string]*ErrorReport
	
	// Background processing
	ctx                   context.Context
	cancel                context.CancelFunc
	
	// Notification tracking
	lastNotificationTime  time.Time
	notificationCount     int64
	
	// Performance metrics
	reportGenerationTimes []time.Duration
	concurrentReports     int
}

// NewReportingService creates a new reporting service
func NewReportingService(config *ReportingServiceConfig, logger *slog.Logger) (*ReportingService, error) {
	if config == nil {
		config = DefaultReportingServiceConfig()
	}
	
	if logger == nil {
		logger = slog.Default()
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	
	// Create error aggregator
	aggregator := NewErrorAggregator(config.AggregatorConfig, logger)
	
	service := &ReportingService{
		config:               config,
		logger:               logger.With("component", "reporting_service"),
		aggregator:           aggregator,
		notificationHandlers: make([]NotificationHandler, 0),
		reportHistory:        make([]*ErrorReport, 0),
		persistedReports:     make(map[string]*ErrorReport),
		ctx:                  ctx,
		cancel:               cancel,
		lastNotificationTime: time.Now(),
		reportGenerationTimes: make([]time.Duration, 0, 100),
	}
	
	// Start background processing
	go service.backgroundProcessor()
	
	return service, nil
}

// RecordError records an error for reporting and analysis
func (rs *ReportingService) RecordError(ctx context.Context, execError *executor.ExecutionError, taskID, userID string) error {
	// Validate input
	if execError == nil {
		return fmt.Errorf("execution error cannot be nil")
	}
	
	// Record in aggregator
	rs.aggregator.RecordError(ctx, execError, taskID, userID)
	
	// Check for immediate notification triggers
	go rs.checkNotificationTriggers(ctx, execError)
	
	rs.logger.Debug("error recorded for reporting",
		"error_type", execError.Type,
		"error_code", execError.Code,
		"task_id", taskID,
		"user_id", userID)
	
	return nil
}

// GenerateReport generates a comprehensive error report
func (rs *ReportingService) GenerateReport(ctx context.Context, timeRange TimeRange) (*ErrorReport, error) {
	// Check concurrent report limit
	rs.mu.Lock()
	if rs.concurrentReports >= rs.config.MaxConcurrentReports {
		rs.mu.Unlock()
		return nil, fmt.Errorf("max concurrent reports limit reached: %d", rs.config.MaxConcurrentReports)
	}
	rs.concurrentReports++
	rs.mu.Unlock()
	
	defer func() {
		rs.mu.Lock()
		rs.concurrentReports--
		rs.mu.Unlock()
	}()
	
	// Generate report with timeout
	reportCtx, cancel := context.WithTimeout(ctx, rs.config.ReportTimeout)
	defer cancel()
	
	startTime := time.Now()
	
	report, err := rs.aggregator.GenerateReport(reportCtx, timeRange)
	if err != nil {
		rs.logger.Error("failed to generate report", "error", err)
		return nil, fmt.Errorf("failed to generate report: %w", err)
	}
	
	generationTime := time.Since(startTime)
	
	// Track performance
	rs.mu.Lock()
	rs.reportGenerationTimes = append(rs.reportGenerationTimes, generationTime)
	if len(rs.reportGenerationTimes) > 100 {
		rs.reportGenerationTimes = rs.reportGenerationTimes[1:]
	}
	rs.mu.Unlock()
	
	// Store in history
	rs.storeReport(report)
	
	// Persist if enabled
	if rs.config.EnablePersistence {
		go rs.persistReport(report)
	}
	
	rs.logger.Info("report generated successfully",
		"report_id", report.ID,
		"generation_time", generationTime,
		"total_errors", report.TotalErrors)
	
	return report, nil
}

// GenerateQuickReport generates a quick report for the last hour
func (rs *ReportingService) GenerateQuickReport(ctx context.Context) (*ErrorReport, error) {
	timeRange := TimeRange{
		Start: time.Now().Add(-1 * time.Hour),
		End:   time.Now(),
	}
	
	return rs.GenerateReport(ctx, timeRange)
}

// GenerateDailyReport generates a daily error report
func (rs *ReportingService) GenerateDailyReport(ctx context.Context) (*ErrorReport, error) {
	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	
	timeRange := TimeRange{
		Start: startOfDay,
		End:   now,
	}
	
	return rs.GenerateReport(ctx, timeRange)
}

// GenerateWeeklyReport generates a weekly error report
func (rs *ReportingService) GenerateWeeklyReport(ctx context.Context) (*ErrorReport, error) {
	now := time.Now()
	startOfWeek := now.AddDate(0, 0, -7)
	
	timeRange := TimeRange{
		Start: startOfWeek,
		End:   now,
	}
	
	return rs.GenerateReport(ctx, timeRange)
}

// GetReportHistory returns recent report history
func (rs *ReportingService) GetReportHistory(limit int) []*ErrorReport {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	
	if limit <= 0 || limit > len(rs.reportHistory) {
		limit = len(rs.reportHistory)
	}
	
	// Return most recent reports
	result := make([]*ErrorReport, limit)
	for i := 0; i < limit; i++ {
		result[i] = rs.reportHistory[len(rs.reportHistory)-1-i]
	}
	
	return result
}

// GetReportByID retrieves a specific report by ID
func (rs *ReportingService) GetReportByID(reportID string) (*ErrorReport, error) {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	
	// Check in-memory history first
	for _, report := range rs.reportHistory {
		if report.ID == reportID {
			return report, nil
		}
	}
	
	// Check persisted reports
	if report, exists := rs.persistedReports[reportID]; exists {
		return report, nil
	}
	
	return nil, fmt.Errorf("report not found: %s", reportID)
}

// AddNotificationHandler adds a notification handler
func (rs *ReportingService) AddNotificationHandler(handler NotificationHandler) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	
	rs.notificationHandlers = append(rs.notificationHandlers, handler)
	rs.logger.Info("notification handler added", "total_handlers", len(rs.notificationHandlers))
}

// RemoveNotificationHandler removes a notification handler
func (rs *ReportingService) RemoveNotificationHandler(handler NotificationHandler) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	
	for i, h := range rs.notificationHandlers {
		if h == handler {
			rs.notificationHandlers = append(rs.notificationHandlers[:i], rs.notificationHandlers[i+1:]...)
			rs.logger.Info("notification handler removed", "total_handlers", len(rs.notificationHandlers))
			break
		}
	}
}

// GetStats returns service statistics
func (rs *ReportingService) GetStats() map[string]interface{} {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	
	avgGenerationTime := time.Duration(0)
	if len(rs.reportGenerationTimes) > 0 {
		total := time.Duration(0)
		for _, duration := range rs.reportGenerationTimes {
			total += duration
		}
		avgGenerationTime = total / time.Duration(len(rs.reportGenerationTimes))
	}
	
	aggregatorStats := rs.aggregator.GetStats()
	
	return map[string]interface{}{
		"report_history_count":      len(rs.reportHistory),
		"persisted_reports_count":   len(rs.persistedReports),
		"notification_handlers":     len(rs.notificationHandlers),
		"last_notification_time":    rs.lastNotificationTime,
		"notification_count":        rs.notificationCount,
		"concurrent_reports":        rs.concurrentReports,
		"avg_generation_time":       avgGenerationTime,
		"aggregator_stats":          aggregatorStats,
	}
}

// ExportReports exports reports in various formats
func (rs *ReportingService) ExportReports(ctx context.Context, format string, timeRange *TimeRange) ([]byte, error) {
	var reports []*ErrorReport
	
	rs.mu.RLock()
	if timeRange == nil {
		// Export all reports
		reports = make([]*ErrorReport, len(rs.reportHistory))
		copy(reports, rs.reportHistory)
	} else {
		// Filter by time range
		for _, report := range rs.reportHistory {
			if report.GeneratedAt.After(timeRange.Start) && report.GeneratedAt.Before(timeRange.End) {
				reports = append(reports, report)
			}
		}
	}
	rs.mu.RUnlock()
	
	switch format {
	case "json":
		return rs.exportJSON(reports)
	case "csv":
		return rs.exportCSV(reports)
	default:
		return nil, fmt.Errorf("unsupported export format: %s", format)
	}
}

// backgroundProcessor handles background processing tasks
func (rs *ReportingService) backgroundProcessor() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	
	// Process auto-generated reports from aggregator
	go rs.processAutoReports()
	
	for {
		select {
		case <-rs.ctx.Done():
			rs.logger.Info("reporting service background processor stopped")
			return
			
		case <-ticker.C:
			rs.performBackgroundTasks()
		}
	}
}

// processAutoReports processes auto-generated reports from the aggregator
func (rs *ReportingService) processAutoReports() {
	reportChan := rs.aggregator.GetReportChannel()
	
	for {
		select {
		case <-rs.ctx.Done():
			return
			
		case report, ok := <-reportChan:
			if !ok {
				return // Channel closed
			}
			
			rs.logger.Info("processing auto-generated report", "report_id", report.ID)
			
			// Store the report
			rs.storeReport(report)
			
			// Check for notification triggers
			if rs.shouldSendNotification(report) {
				go rs.sendReportNotification(report)
			}
		}
	}
}

// performBackgroundTasks performs periodic background maintenance
func (rs *ReportingService) performBackgroundTasks() {
	// Cleanup old reports
	rs.cleanupOldReports()
	
	// Check for periodic notifications
	if rs.config.EnableNotifications {
		rs.checkPeriodicNotifications()
	}
}

// storeReport stores a report in history
func (rs *ReportingService) storeReport(report *ErrorReport) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	
	rs.reportHistory = append(rs.reportHistory, report)
	
	// Limit history size
	if len(rs.reportHistory) > rs.config.MaxReportHistory {
		rs.reportHistory = rs.reportHistory[len(rs.reportHistory)-rs.config.MaxReportHistory:]
	}
}

// persistReport persists a report to storage
func (rs *ReportingService) persistReport(report *ErrorReport) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	
	// For now, just store in memory
	// In a real implementation, this would write to file system or database
	rs.persistedReports[report.ID] = report
	
	rs.logger.Debug("report persisted", "report_id", report.ID)
}

// checkNotificationTriggers checks if an error should trigger immediate notifications
func (rs *ReportingService) checkNotificationTriggers(ctx context.Context, execError *executor.ExecutionError) {
	if !rs.config.EnableNotifications {
		return
	}
	
	// Check for critical errors that need immediate notification
	if execError.Type == executor.ErrorTypeSecurity {
		notification := &ErrorNotification{
			ID:        fmt.Sprintf("critical-%d", time.Now().Unix()),
			Timestamp: time.Now(),
			Severity:  "critical",
			Title:     "Critical Security Error Detected",
			Message:   fmt.Sprintf("Security error: %s", execError.Message),
			ErrorCount: 1,
		}
		
		rs.sendNotification(ctx, notification)
	}
}

// shouldSendNotification determines if a report should trigger a notification
func (rs *ReportingService) shouldSendNotification(report *ErrorReport) bool {
	if !rs.config.EnableNotifications {
		return false
	}
	
	// Check time interval
	if time.Since(rs.lastNotificationTime) < rs.config.NotificationInterval {
		return false
	}
	
	// Check error threshold
	if report.TotalErrors >= rs.config.NotificationThreshold {
		return true
	}
	
	// Check for high-priority recommendations
	for _, rec := range report.Recommendations {
		if rec.Priority == "critical" || rec.Priority == "high" {
			return true
		}
	}
	
	return false
}

// sendReportNotification sends a notification for a report
func (rs *ReportingService) sendReportNotification(report *ErrorReport) {
	severity := "medium"
	if report.TotalErrors > rs.config.NotificationThreshold*2 {
		severity = "high"
	}
	
	// Check for critical recommendations
	for _, rec := range report.Recommendations {
		if rec.Priority == "critical" {
			severity = "critical"
			break
		}
	}
	
	notification := &ErrorNotification{
		ID:        fmt.Sprintf("report-%s", report.ID),
		Timestamp: time.Now(),
		Severity:  severity,
		Title:     "Error Report Generated",
		Message:   fmt.Sprintf("Generated error report with %d errors (%d unique)", report.TotalErrors, report.UniqueErrors),
		ErrorCount: report.TotalErrors,
		TimeRange:  report.TimeRange,
		TopErrors:  report.TopErrors[:min(5, len(report.TopErrors))],
		Recommendations: report.Recommendations[:min(3, len(report.Recommendations))],
	}
	
	rs.sendNotification(context.Background(), notification)
}

// sendNotification sends a notification using all registered handlers
func (rs *ReportingService) sendNotification(ctx context.Context, notification *ErrorNotification) {
	rs.mu.Lock()
	handlers := make([]NotificationHandler, len(rs.notificationHandlers))
	copy(handlers, rs.notificationHandlers)
	rs.lastNotificationTime = time.Now()
	rs.notificationCount++
	rs.mu.Unlock()
	
	for _, handler := range handlers {
		go func(h NotificationHandler) {
			if err := h.SendNotification(ctx, notification); err != nil {
				rs.logger.Error("failed to send notification", "error", err, "notification_id", notification.ID)
			}
		}(handler)
	}
	
	rs.logger.Info("notification sent",
		"notification_id", notification.ID,
		"severity", notification.Severity,
		"handlers", len(handlers))
}

// checkPeriodicNotifications checks for periodic notification requirements
func (rs *ReportingService) checkPeriodicNotifications() {
	// Implementation for periodic health check notifications
	// This could include daily/weekly summaries, etc.
}

// cleanupOldReports removes old reports from memory
func (rs *ReportingService) cleanupOldReports() {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	
	// Remove old persisted reports (keep only recent ones)
	cutoff := time.Now().AddDate(0, 0, -30) // Keep 30 days
	for id, report := range rs.persistedReports {
		if report.GeneratedAt.Before(cutoff) {
			delete(rs.persistedReports, id)
		}
	}
	
	rs.logger.Debug("old reports cleanup completed", "persisted_count", len(rs.persistedReports))
}

// exportJSON exports reports as JSON
func (rs *ReportingService) exportJSON(reports []*ErrorReport) ([]byte, error) {
	return rs.aggregator.ExportData(context.Background(), "json")
}

// exportCSV exports reports as CSV (placeholder implementation)
func (rs *ReportingService) exportCSV(reports []*ErrorReport) ([]byte, error) {
	// Placeholder for CSV export implementation
	return []byte("CSV export not implemented yet"), nil
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Stop gracefully stops the reporting service
func (rs *ReportingService) Stop(ctx context.Context) error {
	rs.logger.Info("stopping reporting service")
	
	rs.cancel()
	
	// Stop aggregator
	if err := rs.aggregator.Stop(ctx); err != nil {
		rs.logger.Error("failed to stop aggregator", "error", err)
		return err
	}
	
	rs.logger.Info("reporting service stopped successfully")
	return nil
}
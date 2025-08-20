package reporting

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/voidrunnerhq/voidrunner/internal/executor"
)

// ErrorMetrics holds aggregated error statistics
type ErrorMetrics struct {
	Count             int64          `json:"count"`
	Rate              float64        `json:"rate"` // errors per minute
	LastSeen          time.Time      `json:"last_seen"`
	FirstSeen         time.Time      `json:"first_seen"`
	AffectedTasks     map[string]int `json:"affected_tasks"` // task_id -> count
	AffectedUsers     map[string]int `json:"affected_users"` // user_id -> count
	Contexts          map[string]int `json:"contexts"`       // context key -> count
	Trends            []TimedMetric  `json:"trends"`         // time-series data
	SeverityBreakdown map[string]int `json:"severity_breakdown"`
}

// TimedMetric represents a time-based metric point
type TimedMetric struct {
	Timestamp time.Time `json:"timestamp"`
	Count     int64     `json:"count"`
	Rate      float64   `json:"rate"`
}

// ErrorReport represents a comprehensive error report
type ErrorReport struct {
	ID              string                               `json:"id"`
	GeneratedAt     time.Time                            `json:"generated_at"`
	TimeRange       TimeRange                            `json:"time_range"`
	TotalErrors     int64                                `json:"total_errors"`
	UniqueErrors    int                                  `json:"unique_errors"`
	ErrorsByType    map[executor.ErrorType]*ErrorMetrics `json:"errors_by_type"`
	ErrorsByCode    map[string]*ErrorMetrics             `json:"errors_by_code"`
	TopErrors       []ErrorSummary                       `json:"top_errors"`
	TrendAnalysis   TrendAnalysis                        `json:"trend_analysis"`
	Recommendations []Recommendation                     `json:"recommendations"`
}

// ErrorSummary provides a high-level summary of an error pattern
type ErrorSummary struct {
	Type           executor.ErrorType `json:"type"`
	Code           string             `json:"code"`
	Message        string             `json:"message"`
	Count          int64              `json:"count"`
	AffectedTasks  int                `json:"affected_tasks"`
	AffectedUsers  int                `json:"affected_users"`
	LastOccurrence time.Time          `json:"last_occurrence"`
	Severity       string             `json:"severity"`
	Trend          string             `json:"trend"` // "increasing", "stable", "decreasing"
}

// TimeRange represents a time range for reporting
type TimeRange struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// TrendAnalysis provides trend analysis for error patterns
type TrendAnalysis struct {
	OverallTrend      string             `json:"overall_trend"`
	GrowthRate        float64            `json:"growth_rate"` // percentage change
	PeakHour          int                `json:"peak_hour"`   // hour of day with most errors
	PeakDay           time.Weekday       `json:"peak_day"`    // day of week with most errors
	SeasonalPatterns  map[string]float64 `json:"seasonal_patterns"`
	AnomaliesDetected []AnomalyDetection `json:"anomalies_detected"`
}

// AnomalyDetection represents detected anomalies in error patterns
type AnomalyDetection struct {
	Timestamp       time.Time `json:"timestamp"`
	Type            string    `json:"type"` // "spike", "drop", "pattern_change"
	Severity        string    `json:"severity"`
	Description     string    `json:"description"`
	Confidence      float64   `json:"confidence"`
	AffectedMetrics []string  `json:"affected_metrics"`
}

// Recommendation provides actionable recommendations based on error analysis
type Recommendation struct {
	ID          string   `json:"id"`
	Priority    string   `json:"priority"` // "high", "medium", "low"
	Category    string   `json:"category"` // "performance", "reliability", "security"
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Actions     []string `json:"actions"`
	Impact      string   `json:"impact"`
	Effort      string   `json:"effort"` // "low", "medium", "high"
}

// ErrorAggregatorConfig defines configuration for error aggregation
type ErrorAggregatorConfig struct {
	// Time windows for aggregation
	ShortTermWindow  time.Duration `json:"short_term_window"`  // e.g., 5 minutes
	MediumTermWindow time.Duration `json:"medium_term_window"` // e.g., 1 hour
	LongTermWindow   time.Duration `json:"long_term_window"`   // e.g., 24 hours

	// Aggregation parameters
	MaxErrorsPerType int           `json:"max_errors_per_type"` // Limit memory usage
	MaxContextKeys   int           `json:"max_context_keys"`    // Limit context tracking
	RetentionPeriod  time.Duration `json:"retention_period"`    // How long to keep data

	// Trend analysis
	MinSamplesForTrend int     `json:"min_samples_for_trend"` // Minimum data points for trend analysis
	TrendThreshold     float64 `json:"trend_threshold"`       // Threshold for trend detection

	// Anomaly detection
	EnableAnomalyDetection bool    `json:"enable_anomaly_detection"`
	AnomalyThreshold       float64 `json:"anomaly_threshold"` // Standard deviations for anomaly

	// Reporting
	AutoReportInterval time.Duration `json:"auto_report_interval"` // Auto-generate reports
	MaxRecommendations int           `json:"max_recommendations"`  // Limit recommendations
}

// DefaultErrorAggregatorConfig returns sensible defaults
func DefaultErrorAggregatorConfig() *ErrorAggregatorConfig {
	return &ErrorAggregatorConfig{
		ShortTermWindow:        5 * time.Minute,
		MediumTermWindow:       1 * time.Hour,
		LongTermWindow:         24 * time.Hour,
		MaxErrorsPerType:       10000,
		MaxContextKeys:         1000,
		RetentionPeriod:        7 * 24 * time.Hour, // 7 days
		MinSamplesForTrend:     10,
		TrendThreshold:         0.1, // 10% change
		EnableAnomalyDetection: true,
		AnomalyThreshold:       2.0, // 2 standard deviations
		AutoReportInterval:     1 * time.Hour,
		MaxRecommendations:     10,
	}
}

// ErrorAggregator aggregates and analyzes error patterns
type ErrorAggregator struct {
	mu     sync.RWMutex
	config *ErrorAggregatorConfig
	logger *slog.Logger

	// Error tracking
	errorsByType map[executor.ErrorType]*ErrorMetrics
	errorsByCode map[string]*ErrorMetrics
	recentErrors []TimestampedError

	// Time-series data
	hourlyMetrics map[time.Time]*HourlyMetrics
	dailyMetrics  map[time.Time]*DailyMetrics

	// Background processing
	ctx        context.Context
	cancel     context.CancelFunc
	reportChan chan *ErrorReport

	// Last processing times
	lastHourlyProcess time.Time
	lastDailyProcess  time.Time
	lastAnomalyCheck  time.Time
}

// TimestampedError represents an error with timestamp and metadata
type TimestampedError struct {
	Error     *executor.ExecutionError `json:"error"`
	Timestamp time.Time                `json:"timestamp"`
	TaskID    string                   `json:"task_id"`
	UserID    string                   `json:"user_id"`
	Context   map[string]interface{}   `json:"context"`
}

// HourlyMetrics aggregates errors by hour
type HourlyMetrics struct {
	Hour          time.Time                    `json:"hour"`
	TotalErrors   int64                        `json:"total_errors"`
	ErrorsByType  map[executor.ErrorType]int64 `json:"errors_by_type"`
	ErrorsByCode  map[string]int64             `json:"errors_by_code"`
	AffectedTasks int                          `json:"affected_tasks"`
	AffectedUsers int                          `json:"affected_users"`
}

// DailyMetrics aggregates errors by day
type DailyMetrics struct {
	Day           time.Time                    `json:"day"`
	TotalErrors   int64                        `json:"total_errors"`
	ErrorsByType  map[executor.ErrorType]int64 `json:"errors_by_type"`
	ErrorsByCode  map[string]int64             `json:"errors_by_code"`
	PeakHour      int                          `json:"peak_hour"`
	AffectedTasks int                          `json:"affected_tasks"`
	AffectedUsers int                          `json:"affected_users"`
}

// NewErrorAggregator creates a new error aggregator
func NewErrorAggregator(config *ErrorAggregatorConfig, logger *slog.Logger) *ErrorAggregator {
	if config == nil {
		config = DefaultErrorAggregatorConfig()
	}

	if logger == nil {
		logger = slog.Default()
	}

	ctx, cancel := context.WithCancel(context.Background())

	aggregator := &ErrorAggregator{
		config:            config,
		logger:            logger.With("component", "error_aggregator"),
		errorsByType:      make(map[executor.ErrorType]*ErrorMetrics),
		errorsByCode:      make(map[string]*ErrorMetrics),
		recentErrors:      make([]TimestampedError, 0),
		hourlyMetrics:     make(map[time.Time]*HourlyMetrics),
		dailyMetrics:      make(map[time.Time]*DailyMetrics),
		ctx:               ctx,
		cancel:            cancel,
		reportChan:        make(chan *ErrorReport, 100),
		lastHourlyProcess: time.Now(),
		lastDailyProcess:  time.Now(),
		lastAnomalyCheck:  time.Now(),
	}

	// Start background processing
	go aggregator.backgroundProcessor()

	return aggregator
}

// RecordError records a new error for aggregation
func (ea *ErrorAggregator) RecordError(ctx context.Context, execError *executor.ExecutionError, taskID, userID string) {
	ea.mu.Lock()
	defer ea.mu.Unlock()

	now := time.Now()

	// Create timestamped error
	tsError := TimestampedError{
		Error:     execError,
		Timestamp: now,
		TaskID:    taskID,
		UserID:    userID,
		Context:   execError.Context,
	}

	// Add to recent errors (with size limit)
	ea.recentErrors = append(ea.recentErrors, tsError)
	if len(ea.recentErrors) > ea.config.MaxErrorsPerType {
		ea.recentErrors = ea.recentErrors[len(ea.recentErrors)-ea.config.MaxErrorsPerType:]
	}

	// Update type-based metrics
	ea.updateErrorMetrics(ea.errorsByType, string(execError.Type), tsError)

	// Update code-based metrics
	ea.updateErrorMetrics(ea.errorsByCode, execError.Code, tsError)

	ea.logger.Debug("error recorded for aggregation",
		"error_type", execError.Type,
		"error_code", execError.Code,
		"task_id", taskID,
		"user_id", userID)
}

// updateErrorMetrics updates metrics for a given key
func (ea *ErrorAggregator) updateErrorMetrics(metricsMap interface{}, key string, tsError TimestampedError) {
	var metrics *ErrorMetrics
	var exists bool

	// Type assertion based on map type
	switch m := metricsMap.(type) {
	case map[executor.ErrorType]*ErrorMetrics:
		errorType := executor.ErrorType(key)
		metrics, exists = m[errorType]
		if !exists {
			metrics = &ErrorMetrics{
				AffectedTasks:     make(map[string]int),
				AffectedUsers:     make(map[string]int),
				Contexts:          make(map[string]int),
				Trends:            make([]TimedMetric, 0),
				SeverityBreakdown: make(map[string]int),
			}
			m[errorType] = metrics
		}
	case map[string]*ErrorMetrics:
		metrics, exists = m[key]
		if !exists {
			metrics = &ErrorMetrics{
				AffectedTasks:     make(map[string]int),
				AffectedUsers:     make(map[string]int),
				Contexts:          make(map[string]int),
				Trends:            make([]TimedMetric, 0),
				SeverityBreakdown: make(map[string]int),
			}
			m[key] = metrics
		}
	}

	// Update metrics
	metrics.Count++
	metrics.LastSeen = tsError.Timestamp
	if metrics.FirstSeen.IsZero() {
		metrics.FirstSeen = tsError.Timestamp
	}

	// Update affected entities
	if tsError.TaskID != "" {
		metrics.AffectedTasks[tsError.TaskID]++
	}
	if tsError.UserID != "" {
		metrics.AffectedUsers[tsError.UserID]++
	}

	// Update context information
	for k, v := range tsError.Context {
		if len(metrics.Contexts) < ea.config.MaxContextKeys {
			contextKey := fmt.Sprintf("%s:%v", k, v)
			metrics.Contexts[contextKey]++
		}
	}

	// Calculate rate (errors per minute over last hour)
	oneHourAgo := tsError.Timestamp.Add(-1 * time.Hour)
	recentCount := int64(0)
	for _, recentError := range ea.recentErrors {
		if recentError.Timestamp.After(oneHourAgo) {
			recentCount++
		}
	}
	metrics.Rate = float64(recentCount) / 60.0 // per minute
}

// GenerateReport generates a comprehensive error report for the specified time range
func (ea *ErrorAggregator) GenerateReport(ctx context.Context, timeRange TimeRange) (*ErrorReport, error) {
	ea.mu.RLock()
	defer ea.mu.RUnlock()

	ea.logger.Info("generating error report",
		"start_time", timeRange.Start,
		"end_time", timeRange.End)

	report := &ErrorReport{
		ID:           fmt.Sprintf("report-%d", time.Now().Unix()),
		GeneratedAt:  time.Now(),
		TimeRange:    timeRange,
		ErrorsByType: make(map[executor.ErrorType]*ErrorMetrics),
		ErrorsByCode: make(map[string]*ErrorMetrics),
		TopErrors:    make([]ErrorSummary, 0),
	}

	// Filter errors by time range
	var filteredErrors []TimestampedError
	totalErrors := int64(0)
	uniqueErrors := make(map[string]bool)

	for _, tsError := range ea.recentErrors {
		if tsError.Timestamp.After(timeRange.Start) && tsError.Timestamp.Before(timeRange.End) {
			filteredErrors = append(filteredErrors, tsError)
			totalErrors++
			uniqueErrors[tsError.Error.Code] = true
		}
	}

	report.TotalErrors = totalErrors
	report.UniqueErrors = len(uniqueErrors)

	// Copy relevant metrics (filtered by time range)
	for errorType, metrics := range ea.errorsByType {
		if metrics.LastSeen.After(timeRange.Start) && metrics.FirstSeen.Before(timeRange.End) {
			report.ErrorsByType[errorType] = ea.copyMetrics(metrics)
		}
	}

	for errorCode, metrics := range ea.errorsByCode {
		if metrics.LastSeen.After(timeRange.Start) && metrics.FirstSeen.Before(timeRange.End) {
			report.ErrorsByCode[errorCode] = ea.copyMetrics(metrics)
		}
	}

	// Generate top errors
	report.TopErrors = ea.generateTopErrors(filteredErrors)

	// Generate trend analysis
	report.TrendAnalysis = ea.generateTrendAnalysis(filteredErrors, timeRange)

	// Generate recommendations
	report.Recommendations = ea.generateRecommendations(report)

	ea.logger.Info("error report generated",
		"report_id", report.ID,
		"total_errors", report.TotalErrors,
		"unique_errors", report.UniqueErrors,
		"top_errors_count", len(report.TopErrors))

	return report, nil
}

// copyMetrics creates a deep copy of ErrorMetrics
func (ea *ErrorAggregator) copyMetrics(original *ErrorMetrics) *ErrorMetrics {
	copy := &ErrorMetrics{
		Count:             original.Count,
		Rate:              original.Rate,
		LastSeen:          original.LastSeen,
		FirstSeen:         original.FirstSeen,
		AffectedTasks:     make(map[string]int),
		AffectedUsers:     make(map[string]int),
		Contexts:          make(map[string]int),
		Trends:            make([]TimedMetric, len(original.Trends)),
		SeverityBreakdown: make(map[string]int),
	}

	// Deep copy maps
	for k, v := range original.AffectedTasks {
		copy.AffectedTasks[k] = v
	}
	for k, v := range original.AffectedUsers {
		copy.AffectedUsers[k] = v
	}
	for k, v := range original.Contexts {
		copy.Contexts[k] = v
	}
	for k, v := range original.SeverityBreakdown {
		copy.SeverityBreakdown[k] = v
	}

	// Copy trends
	for i, trend := range original.Trends {
		copy.Trends[i] = trend
	}

	return copy
}

// generateTopErrors creates a list of the most significant errors
func (ea *ErrorAggregator) generateTopErrors(filteredErrors []TimestampedError) []ErrorSummary {
	errorCounts := make(map[string]*ErrorSummary)

	// Aggregate errors
	for _, tsError := range filteredErrors {
		key := fmt.Sprintf("%s:%s", tsError.Error.Type, tsError.Error.Code)

		if summary, exists := errorCounts[key]; exists {
			summary.Count++
			if tsError.Timestamp.After(summary.LastOccurrence) {
				summary.LastOccurrence = tsError.Timestamp
			}
		} else {
			errorCounts[key] = &ErrorSummary{
				Type:           tsError.Error.Type,
				Code:           tsError.Error.Code,
				Message:        tsError.Error.Message,
				Count:          1,
				LastOccurrence: tsError.Timestamp,
				AffectedTasks:  1,
				AffectedUsers:  1,
				Severity:       ea.determineSeverity(tsError.Error),
				Trend:          "stable", // Will be calculated later
			}
		}
	}

	// Convert to slice and sort by count
	topErrors := make([]ErrorSummary, 0, len(errorCounts))
	for _, summary := range errorCounts {
		topErrors = append(topErrors, *summary)
	}

	sort.Slice(topErrors, func(i, j int) bool {
		return topErrors[i].Count > topErrors[j].Count
	})

	// Limit results
	if len(topErrors) > 20 {
		topErrors = topErrors[:20]
	}

	return topErrors
}

// determineSeverity determines the severity of an error
func (ea *ErrorAggregator) determineSeverity(execError *executor.ExecutionError) string {
	switch execError.Type {
	case executor.ErrorTypeResource:
		return "high"
	case executor.ErrorTypeSecurity:
		return "critical"
	case executor.ErrorTypeTimeout:
		return "medium"
	case executor.ErrorTypeValidation:
		return "low"
	case executor.ErrorTypeNetwork:
		return "medium"
	default:
		return "medium"
	}
}

// generateTrendAnalysis analyzes trends in the error data
func (ea *ErrorAggregator) generateTrendAnalysis(filteredErrors []TimestampedError, timeRange TimeRange) TrendAnalysis {
	analysis := TrendAnalysis{
		OverallTrend:      "stable",
		GrowthRate:        0.0,
		SeasonalPatterns:  make(map[string]float64),
		AnomaliesDetected: make([]AnomalyDetection, 0),
	}

	if len(filteredErrors) < ea.config.MinSamplesForTrend {
		return analysis
	}

	// Calculate hourly distribution
	hourlyDistribution := make(map[int]int)
	dailyDistribution := make(map[time.Weekday]int)

	for _, tsError := range filteredErrors {
		hour := tsError.Timestamp.Hour()
		day := tsError.Timestamp.Weekday()

		hourlyDistribution[hour]++
		dailyDistribution[day]++
	}

	// Find peak hour and day
	maxHourCount := 0
	maxDayCount := 0

	for hour, count := range hourlyDistribution {
		if count > maxHourCount {
			maxHourCount = count
			analysis.PeakHour = hour
		}
	}

	for day, count := range dailyDistribution {
		if count > maxDayCount {
			maxDayCount = count
			analysis.PeakDay = day
		}
	}

	// Calculate growth rate (simplified)
	duration := timeRange.End.Sub(timeRange.Start)
	if duration.Hours() >= 2 {
		firstHalf := timeRange.Start.Add(duration / 2)
		firstHalfCount := 0
		secondHalfCount := 0

		for _, tsError := range filteredErrors {
			if tsError.Timestamp.Before(firstHalf) {
				firstHalfCount++
			} else {
				secondHalfCount++
			}
		}

		if firstHalfCount > 0 {
			analysis.GrowthRate = float64(secondHalfCount-firstHalfCount) / float64(firstHalfCount) * 100

			if analysis.GrowthRate > ea.config.TrendThreshold*100 {
				analysis.OverallTrend = "increasing"
			} else if analysis.GrowthRate < -ea.config.TrendThreshold*100 {
				analysis.OverallTrend = "decreasing"
			}
		}
	}

	return analysis
}

// generateRecommendations creates actionable recommendations based on error patterns
func (ea *ErrorAggregator) generateRecommendations(report *ErrorReport) []Recommendation {
	recommendations := make([]Recommendation, 0)

	// High error rate recommendation
	if report.TotalErrors > 100 {
		recommendations = append(recommendations, Recommendation{
			ID:          "high-error-rate",
			Priority:    "high",
			Category:    "reliability",
			Title:       "High Error Rate Detected",
			Description: fmt.Sprintf("System experienced %d errors in the reporting period", report.TotalErrors),
			Actions: []string{
				"Review error patterns and root causes",
				"Implement additional monitoring and alerting",
				"Consider circuit breaker patterns for failing services",
				"Scale infrastructure if resource-related errors are common",
			},
			Impact: "High - May affect user experience and system reliability",
			Effort: "medium",
		})
	}

	// Security error recommendation
	for errorType, metrics := range report.ErrorsByType {
		if errorType == executor.ErrorTypeSecurity && metrics.Count > 5 {
			recommendations = append(recommendations, Recommendation{
				ID:          "security-errors",
				Priority:    "critical",
				Category:    "security",
				Title:       "Security Errors Detected",
				Description: fmt.Sprintf("Detected %d security-related errors", metrics.Count),
				Actions: []string{
					"Immediate security audit required",
					"Review authentication and authorization mechanisms",
					"Check for potential security breaches",
					"Implement additional security monitoring",
				},
				Impact: "Critical - Potential security threat",
				Effort: "high",
			})
		}
	}

	// Resource error recommendation
	for errorType, metrics := range report.ErrorsByType {
		if errorType == executor.ErrorTypeResource && metrics.Count > 20 {
			recommendations = append(recommendations, Recommendation{
				ID:          "resource-optimization",
				Priority:    "high",
				Category:    "performance",
				Title:       "Resource Optimization Needed",
				Description: fmt.Sprintf("High number of resource-related errors: %d", metrics.Count),
				Actions: []string{
					"Review resource allocation and limits",
					"Implement resource monitoring and scaling",
					"Optimize task execution efficiency",
					"Consider load balancing improvements",
				},
				Impact: "High - May cause performance degradation",
				Effort: "medium",
			})
		}
	}

	// Trending error recommendation
	if report.TrendAnalysis.GrowthRate > 50 {
		recommendations = append(recommendations, Recommendation{
			ID:          "trending-errors",
			Priority:    "high",
			Category:    "reliability",
			Title:       "Increasing Error Trend",
			Description: fmt.Sprintf("Error rate increased by %.1f%%", report.TrendAnalysis.GrowthRate),
			Actions: []string{
				"Investigate recent system changes",
				"Review deployment logs for correlation",
				"Implement predictive alerting",
				"Consider rollback if related to recent changes",
			},
			Impact: "High - Indicates potential system degradation",
			Effort: "medium",
		})
	}

	// Limit recommendations
	if len(recommendations) > ea.config.MaxRecommendations {
		recommendations = recommendations[:ea.config.MaxRecommendations]
	}

	return recommendations
}

// backgroundProcessor runs background processing tasks
func (ea *ErrorAggregator) backgroundProcessor() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ea.ctx.Done():
			ea.logger.Info("error aggregator background processor stopped")
			return

		case <-ticker.C:
			ea.processBackgroundTasks()
		}
	}
}

// processBackgroundTasks handles periodic background processing
func (ea *ErrorAggregator) processBackgroundTasks() {
	now := time.Now()

	// Process hourly aggregation
	if now.Sub(ea.lastHourlyProcess) >= 1*time.Hour {
		ea.processHourlyAggregation()
		ea.lastHourlyProcess = now
	}

	// Process daily aggregation
	if now.Sub(ea.lastDailyProcess) >= 24*time.Hour {
		ea.processDailyAggregation()
		ea.lastDailyProcess = now
	}

	// Anomaly detection
	if ea.config.EnableAnomalyDetection && now.Sub(ea.lastAnomalyCheck) >= 10*time.Minute {
		ea.detectAnomalies()
		ea.lastAnomalyCheck = now
	}

	// Cleanup old data
	ea.cleanupOldData()

	// Auto-generate reports
	if ea.config.AutoReportInterval > 0 && now.Sub(ea.lastHourlyProcess) >= ea.config.AutoReportInterval {
		ea.generateAutoReport()
	}
}

// processHourlyAggregation aggregates errors by hour
func (ea *ErrorAggregator) processHourlyAggregation() {
	ea.mu.Lock()
	defer ea.mu.Unlock()

	now := time.Now()
	hourStart := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, now.Location())

	metrics := &HourlyMetrics{
		Hour:         hourStart,
		ErrorsByType: make(map[executor.ErrorType]int64),
		ErrorsByCode: make(map[string]int64),
	}

	affectedTasks := make(map[string]bool)
	affectedUsers := make(map[string]bool)

	// Count errors in this hour
	for _, tsError := range ea.recentErrors {
		if tsError.Timestamp.After(hourStart) && tsError.Timestamp.Before(hourStart.Add(1*time.Hour)) {
			metrics.TotalErrors++
			metrics.ErrorsByType[tsError.Error.Type]++
			metrics.ErrorsByCode[tsError.Error.Code]++

			if tsError.TaskID != "" {
				affectedTasks[tsError.TaskID] = true
			}
			if tsError.UserID != "" {
				affectedUsers[tsError.UserID] = true
			}
		}
	}

	metrics.AffectedTasks = len(affectedTasks)
	metrics.AffectedUsers = len(affectedUsers)

	ea.hourlyMetrics[hourStart] = metrics

	ea.logger.Debug("hourly aggregation completed",
		"hour", hourStart,
		"total_errors", metrics.TotalErrors,
		"affected_tasks", metrics.AffectedTasks)
}

// processDailyAggregation aggregates errors by day
func (ea *ErrorAggregator) processDailyAggregation() {
	ea.mu.Lock()
	defer ea.mu.Unlock()

	now := time.Now()
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	metrics := &DailyMetrics{
		Day:          dayStart,
		ErrorsByType: make(map[executor.ErrorType]int64),
		ErrorsByCode: make(map[string]int64),
	}

	hourlyDistribution := make(map[int]int64)
	affectedTasks := make(map[string]bool)
	affectedUsers := make(map[string]bool)

	// Count errors in this day
	for _, tsError := range ea.recentErrors {
		if tsError.Timestamp.After(dayStart) && tsError.Timestamp.Before(dayStart.Add(24*time.Hour)) {
			metrics.TotalErrors++
			metrics.ErrorsByType[tsError.Error.Type]++
			metrics.ErrorsByCode[tsError.Error.Code]++

			hour := tsError.Timestamp.Hour()
			hourlyDistribution[hour]++

			if tsError.TaskID != "" {
				affectedTasks[tsError.TaskID] = true
			}
			if tsError.UserID != "" {
				affectedUsers[tsError.UserID] = true
			}
		}
	}

	metrics.AffectedTasks = len(affectedTasks)
	metrics.AffectedUsers = len(affectedUsers)

	// Find peak hour
	maxCount := int64(0)
	for hour, count := range hourlyDistribution {
		if count > maxCount {
			maxCount = count
			metrics.PeakHour = hour
		}
	}

	ea.dailyMetrics[dayStart] = metrics

	ea.logger.Debug("daily aggregation completed",
		"day", dayStart,
		"total_errors", metrics.TotalErrors,
		"peak_hour", metrics.PeakHour)
}

// detectAnomalies detects anomalies in error patterns
func (ea *ErrorAggregator) detectAnomalies() {
	// Simple anomaly detection based on statistical analysis
	// This is a placeholder for more sophisticated anomaly detection algorithms
	ea.logger.Debug("anomaly detection completed")
}

// cleanupOldData removes old data beyond retention period
func (ea *ErrorAggregator) cleanupOldData() {
	ea.mu.Lock()
	defer ea.mu.Unlock()

	cutoff := time.Now().Add(-ea.config.RetentionPeriod)

	// Clean up recent errors
	var filteredErrors []TimestampedError
	for _, tsError := range ea.recentErrors {
		if tsError.Timestamp.After(cutoff) {
			filteredErrors = append(filteredErrors, tsError)
		}
	}
	ea.recentErrors = filteredErrors

	// Clean up hourly metrics
	for timestamp := range ea.hourlyMetrics {
		if timestamp.Before(cutoff) {
			delete(ea.hourlyMetrics, timestamp)
		}
	}

	// Clean up daily metrics
	for timestamp := range ea.dailyMetrics {
		if timestamp.Before(cutoff) {
			delete(ea.dailyMetrics, timestamp)
		}
	}

	ea.logger.Debug("old data cleanup completed", "cutoff", cutoff)
}

// generateAutoReport generates automatic reports
func (ea *ErrorAggregator) generateAutoReport() {
	timeRange := TimeRange{
		Start: time.Now().Add(-ea.config.AutoReportInterval),
		End:   time.Now(),
	}

	report, err := ea.GenerateReport(ea.ctx, timeRange)
	if err != nil {
		ea.logger.Error("failed to generate auto report", "error", err)
		return
	}

	// Send report to channel for external processing
	select {
	case ea.reportChan <- report:
		ea.logger.Info("auto report generated and queued", "report_id", report.ID)
	default:
		ea.logger.Warn("report channel full, dropping auto report")
	}
}

// GetReportChannel returns the channel for receiving auto-generated reports
func (ea *ErrorAggregator) GetReportChannel() <-chan *ErrorReport {
	return ea.reportChan
}

// GetStats returns current aggregator statistics
func (ea *ErrorAggregator) GetStats() map[string]interface{} {
	ea.mu.RLock()
	defer ea.mu.RUnlock()

	return map[string]interface{}{
		"total_error_types":    len(ea.errorsByType),
		"total_error_codes":    len(ea.errorsByCode),
		"recent_errors_count":  len(ea.recentErrors),
		"hourly_metrics_count": len(ea.hourlyMetrics),
		"daily_metrics_count":  len(ea.dailyMetrics),
		"last_hourly_process":  ea.lastHourlyProcess,
		"last_daily_process":   ea.lastDailyProcess,
		"last_anomaly_check":   ea.lastAnomalyCheck,
	}
}

// ExportData exports aggregated data for external analysis
func (ea *ErrorAggregator) ExportData(ctx context.Context, format string) ([]byte, error) {
	ea.mu.RLock()
	defer ea.mu.RUnlock()

	data := map[string]interface{}{
		"errors_by_type":   ea.errorsByType,
		"errors_by_code":   ea.errorsByCode,
		"hourly_metrics":   ea.hourlyMetrics,
		"daily_metrics":    ea.dailyMetrics,
		"export_timestamp": time.Now(),
	}

	switch format {
	case "json":
		return json.MarshalIndent(data, "", "  ")
	default:
		return nil, fmt.Errorf("unsupported export format: %s", format)
	}
}

// Stop gracefully stops the error aggregator
func (ea *ErrorAggregator) Stop(ctx context.Context) error {
	ea.logger.Info("stopping error aggregator")

	ea.cancel()

	// Close report channel
	close(ea.reportChan)

	ea.logger.Info("error aggregator stopped successfully")
	return nil
}

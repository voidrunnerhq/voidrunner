package monitoring

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// Alert represents a monitoring alert
type Alert struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"`
	Level       AlertLevel             `json:"level"`
	Title       string                 `json:"title"`
	Message     string                 `json:"message"`
	Timestamp   time.Time              `json:"timestamp"`
	Context     map[string]interface{} `json:"context,omitempty"`
	Resolved    bool                   `json:"resolved"`
	ResolvedAt  *time.Time             `json:"resolved_at,omitempty"`
}

// AlertHandler defines the interface for handling alerts
type AlertHandler interface {
	HandleAlert(ctx context.Context, alert *Alert) error
}

// LogAlertHandler implements AlertHandler by logging alerts
type LogAlertHandler struct {
	logger *slog.Logger
}

// NewLogAlertHandler creates a new log-based alert handler
func NewLogAlertHandler(logger *slog.Logger) *LogAlertHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &LogAlertHandler{logger: logger}
}

// HandleAlert logs the alert
func (h *LogAlertHandler) HandleAlert(ctx context.Context, alert *Alert) error {
	logLevel := slog.LevelInfo
	switch alert.Level {
	case AlertLevelWarning:
		logLevel = slog.LevelWarn
	case AlertLevelCritical:
		logLevel = slog.LevelError
	}

	h.logger.Log(ctx, logLevel, alert.Message,
		"alert_id", alert.ID,
		"alert_type", alert.Type,
		"alert_level", alert.Level,
		"alert_title", alert.Title,
		"timestamp", alert.Timestamp,
		"context", alert.Context,
	)

	return nil
}

// AlertManager manages alerts and notifications
type AlertManager struct {
	mu               sync.RWMutex
	handlers         []AlertHandler
	alerts           map[string]*Alert
	lastAlertTime    map[string]time.Time // For cooldown tracking
	config           *MonitoringConfig
	logger           *slog.Logger
	alertCounter     int64
	lastCleanup      time.Time
}

// NewAlertManager creates a new alert manager
func NewAlertManager(config *MonitoringConfig, logger *slog.Logger) *AlertManager {
	if config == nil {
		config = DefaultMonitoringConfig()
	}
	if logger == nil {
		logger = slog.Default()
	}

	am := &AlertManager{
		handlers:      make([]AlertHandler, 0),
		alerts:        make(map[string]*Alert),
		lastAlertTime: make(map[string]time.Time),
		config:        config,
		logger:        logger.With("component", "alert_manager"),
		lastCleanup:   time.Now(),
	}

	// Add default log handler
	am.AddHandler(NewLogAlertHandler(logger))

	return am
}

// AddHandler adds an alert handler
func (am *AlertManager) AddHandler(handler AlertHandler) {
	am.mu.Lock()
	defer am.mu.Unlock()
	am.handlers = append(am.handlers, handler)
}

// SendAlert creates and sends an alert
func (am *AlertManager) SendAlert(ctx context.Context, alertType, title, message string, level AlertLevel, context map[string]interface{}) error {
	if !am.config.EnableAlerting {
		return nil
	}

	// Check cooldown period
	if am.isInCooldown(alertType) {
		am.logger.Debug("alert suppressed due to cooldown period", 
			"alert_type", alertType, 
			"cooldown_period", am.config.AlertCooldownPeriod)
		return nil
	}

	// Generate unique alert ID
	am.mu.Lock()
	am.alertCounter++
	alertID := fmt.Sprintf("alert_%d_%d", time.Now().Unix(), am.alertCounter)
	am.mu.Unlock()

	alert := &Alert{
		ID:        alertID,
		Type:      alertType,
		Level:     level,
		Title:     title,
		Message:   message,
		Timestamp: time.Now(),
		Context:   context,
		Resolved:  false,
	}

	// Store alert
	am.mu.Lock()
	am.alerts[alertID] = alert
	am.lastAlertTime[alertType] = alert.Timestamp
	am.mu.Unlock()

	// Send to all handlers
	for _, handler := range am.handlers {
		if err := handler.HandleAlert(ctx, alert); err != nil {
			am.logger.Error("failed to handle alert", 
				"handler", fmt.Sprintf("%T", handler), 
				"alert_id", alertID, 
				"error", err)
		}
	}

	am.logger.Info("alert sent successfully", 
		"alert_id", alertID, 
		"alert_type", alertType, 
		"level", level)

	// Periodic cleanup
	am.maybeCleanupOldAlerts()

	return nil
}

// isInCooldown checks if an alert type is in cooldown period
func (am *AlertManager) isInCooldown(alertType string) bool {
	am.mu.RLock()
	defer am.mu.RUnlock()
	
	if lastTime, exists := am.lastAlertTime[alertType]; exists {
		return time.Since(lastTime) < am.config.AlertCooldownPeriod
	}
	return false
}

// ResolveAlert marks an alert as resolved
func (am *AlertManager) ResolveAlert(alertID string) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	alert, exists := am.alerts[alertID]
	if !exists {
		return fmt.Errorf("alert %s not found", alertID)
	}

	if alert.Resolved {
		return fmt.Errorf("alert %s is already resolved", alertID)
	}

	now := time.Now()
	alert.Resolved = true
	alert.ResolvedAt = &now

	am.logger.Info("alert resolved", "alert_id", alertID, "type", alert.Type)
	return nil
}

// GetActiveAlerts returns all active (unresolved) alerts
func (am *AlertManager) GetActiveAlerts() []*Alert {
	am.mu.RLock()
	defer am.mu.RUnlock()

	var activeAlerts []*Alert
	for _, alert := range am.alerts {
		if !alert.Resolved {
			// Return a copy to avoid data races
			alertCopy := *alert
			activeAlerts = append(activeAlerts, &alertCopy)
		}
	}

	return activeAlerts
}

// GetAlertHistory returns alerts within a time range
func (am *AlertManager) GetAlertHistory(since time.Time, until time.Time) []*Alert {
	am.mu.RLock()
	defer am.mu.RUnlock()

	var alerts []*Alert
	for _, alert := range am.alerts {
		if alert.Timestamp.After(since) && alert.Timestamp.Before(until) {
			// Return a copy to avoid data races
			alertCopy := *alert
			alerts = append(alerts, &alertCopy)
		}
	}

	return alerts
}

// GetAlertStats returns statistics about alerts
func (am *AlertManager) GetAlertStats() AlertStats {
	am.mu.RLock()
	defer am.mu.RUnlock()

	stats := AlertStats{
		TotalAlerts:     len(am.alerts),
		ActiveAlerts:    0,
		ResolvedAlerts:  0,
		CriticalAlerts:  0,
		WarningAlerts:   0,
		InfoAlerts:      0,
		AlertsByType:    make(map[string]int),
	}

	for _, alert := range am.alerts {
		if alert.Resolved {
			stats.ResolvedAlerts++
		} else {
			stats.ActiveAlerts++
		}

		switch alert.Level {
		case AlertLevelCritical:
			stats.CriticalAlerts++
		case AlertLevelWarning:
			stats.WarningAlerts++
		case AlertLevelInfo:
			stats.InfoAlerts++
		}

		stats.AlertsByType[alert.Type]++
	}

	return stats
}

// maybeCleanupOldAlerts periodically cleans up old alerts
func (am *AlertManager) maybeCleanupOldAlerts() {
	if time.Since(am.lastCleanup) < time.Hour {
		return
	}

	am.mu.Lock()
	defer am.mu.Unlock()

	cutoff := time.Now().Add(-am.config.MetricsRetentionTime)
	deletedCount := 0

	for alertID, alert := range am.alerts {
		if alert.Timestamp.Before(cutoff) {
			delete(am.alerts, alertID)
			deletedCount++
		}
	}

	am.lastCleanup = time.Now()

	if deletedCount > 0 {
		am.logger.Debug("cleaned up old alerts", "deleted_count", deletedCount)
	}
}

// CleanupOldAlerts manually triggers cleanup of old alerts
func (am *AlertManager) CleanupOldAlerts() {
	am.mu.Lock()
	defer am.mu.Unlock()

	cutoff := time.Now().Add(-am.config.MetricsRetentionTime)
	deletedCount := 0

	for alertID, alert := range am.alerts {
		if alert.Timestamp.Before(cutoff) {
			delete(am.alerts, alertID)
			deletedCount++
		}
	}

	am.logger.Info("cleaned up old alerts", "deleted_count", deletedCount)
}

// Stop gracefully stops the alert manager
func (am *AlertManager) Stop() {
	am.logger.Info("alert manager stopped")
}

// AlertStats contains statistics about alerts
type AlertStats struct {
	TotalAlerts     int            `json:"total_alerts"`
	ActiveAlerts    int            `json:"active_alerts"`
	ResolvedAlerts  int            `json:"resolved_alerts"`
	CriticalAlerts  int            `json:"critical_alerts"`
	WarningAlerts   int            `json:"warning_alerts"`
	InfoAlerts      int            `json:"info_alerts"`
	AlertsByType    map[string]int `json:"alerts_by_type"`
}

// WebhookAlertHandler implements AlertHandler by sending alerts to a webhook
type WebhookAlertHandler struct {
	webhookURL string
	timeout    time.Duration
	logger     *slog.Logger
}

// NewWebhookAlertHandler creates a new webhook-based alert handler
func NewWebhookAlertHandler(webhookURL string, timeout time.Duration, logger *slog.Logger) *WebhookAlertHandler {
	if logger == nil {
		logger = slog.Default()
	}
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	
	return &WebhookAlertHandler{
		webhookURL: webhookURL,
		timeout:    timeout,
		logger:     logger,
	}
}

// HandleAlert sends the alert to the webhook
func (h *WebhookAlertHandler) HandleAlert(ctx context.Context, alert *Alert) error {
	// TODO: Implement webhook HTTP POST
	// For now, just log that we would send a webhook
	h.logger.Info("would send webhook alert", 
		"webhook_url", h.webhookURL,
		"alert_id", alert.ID,
		"alert_type", alert.Type,
		"level", alert.Level)
	
	return nil
}

// EmailAlertHandler implements AlertHandler by sending email alerts
type EmailAlertHandler struct {
	smtpHost     string
	smtpPort     int
	fromEmail    string
	toEmails     []string
	logger       *slog.Logger
}

// NewEmailAlertHandler creates a new email-based alert handler
func NewEmailAlertHandler(smtpHost string, smtpPort int, fromEmail string, toEmails []string, logger *slog.Logger) *EmailAlertHandler {
	if logger == nil {
		logger = slog.Default()
	}
	
	return &EmailAlertHandler{
		smtpHost:  smtpHost,
		smtpPort:  smtpPort,
		fromEmail: fromEmail,
		toEmails:  toEmails,
		logger:    logger,
	}
}

// HandleAlert sends the alert via email
func (h *EmailAlertHandler) HandleAlert(ctx context.Context, alert *Alert) error {
	// TODO: Implement email sending
	// For now, just log that we would send an email
	h.logger.Info("would send email alert", 
		"smtp_host", h.smtpHost,
		"from_email", h.fromEmail,
		"to_emails", h.toEmails,
		"alert_id", alert.ID,
		"alert_type", alert.Type,
		"level", alert.Level)
	
	return nil
}
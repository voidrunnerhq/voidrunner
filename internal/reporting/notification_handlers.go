package reporting

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/smtp"
	"os"
	"path/filepath"
	"time"
)

// LogNotificationHandler logs notifications to the system logger
type LogNotificationHandler struct {
	logger *slog.Logger
	level  slog.Level
}

// NewLogNotificationHandler creates a new log notification handler
func NewLogNotificationHandler(logger *slog.Logger, level slog.Level) *LogNotificationHandler {
	if logger == nil {
		logger = slog.Default()
	}
	
	return &LogNotificationHandler{
		logger: logger.With("component", "log_notification_handler"),
		level:  level,
	}
}

// SendNotification logs the notification
func (h *LogNotificationHandler) SendNotification(ctx context.Context, notification *ErrorNotification) error {
	h.logger.Log(ctx, h.level, "error notification",
		"notification_id", notification.ID,
		"severity", notification.Severity,
		"title", notification.Title,
		"message", notification.Message,
		"error_count", notification.ErrorCount,
		"top_errors_count", len(notification.TopErrors),
		"recommendations_count", len(notification.Recommendations))
	
	return nil
}

// WebhookNotificationHandler sends notifications via HTTP webhooks
type WebhookNotificationHandler struct {
	url     string
	client  *http.Client
	headers map[string]string
	logger  *slog.Logger
}

// WebhookConfig configures webhook notifications
type WebhookConfig struct {
	URL             string            `json:"url"`
	Timeout         time.Duration     `json:"timeout"`
	Headers         map[string]string `json:"headers"`
	RetryAttempts   int               `json:"retry_attempts"`
	RetryDelay      time.Duration     `json:"retry_delay"`
}

// NewWebhookNotificationHandler creates a new webhook notification handler
func NewWebhookNotificationHandler(config *WebhookConfig, logger *slog.Logger) *WebhookNotificationHandler {
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	
	if logger == nil {
		logger = slog.Default()
	}
	
	return &WebhookNotificationHandler{
		url: config.URL,
		client: &http.Client{
			Timeout: config.Timeout,
		},
		headers: config.Headers,
		logger:  logger.With("component", "webhook_notification_handler"),
	}
}

// SendNotification sends the notification via webhook
func (h *WebhookNotificationHandler) SendNotification(ctx context.Context, notification *ErrorNotification) error {
	payload, err := json.Marshal(notification)
	if err != nil {
		return fmt.Errorf("failed to marshal notification: %w", err)
	}
	
	req, err := http.NewRequestWithContext(ctx, "POST", h.url, bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("Content-Type", "application/json")
	
	// Add custom headers
	for key, value := range h.headers {
		req.Header.Set(key, value)
	}
	
	resp, err := h.client.Do(req)
	if err != nil {
		h.logger.Error("webhook request failed",
			"url", h.url,
			"notification_id", notification.ID,
			"error", err)
		return fmt.Errorf("webhook request failed: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		h.logger.Error("webhook returned error status",
			"url", h.url,
			"notification_id", notification.ID,
			"status_code", resp.StatusCode)
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}
	
	h.logger.Info("webhook notification sent successfully",
		"url", h.url,
		"notification_id", notification.ID,
		"status_code", resp.StatusCode)
	
	return nil
}

// EmailNotificationHandler sends notifications via email
type EmailNotificationHandler struct {
	config *EmailConfig
	logger *slog.Logger
}

// EmailConfig configures email notifications
type EmailConfig struct {
	SMTPHost     string   `json:"smtp_host"`
	SMTPPort     string   `json:"smtp_port"`
	Username     string   `json:"username"`
	Password     string   `json:"password"`
	FromAddress  string   `json:"from_address"`
	ToAddresses  []string `json:"to_addresses"`
	Subject      string   `json:"subject"`
	UseStartTLS  bool     `json:"use_start_tls"`
}

// NewEmailNotificationHandler creates a new email notification handler
func NewEmailNotificationHandler(config *EmailConfig, logger *slog.Logger) *EmailNotificationHandler {
	if logger == nil {
		logger = slog.Default()
	}
	
	return &EmailNotificationHandler{
		config: config,
		logger: logger.With("component", "email_notification_handler"),
	}
}

// SendNotification sends the notification via email
func (h *EmailNotificationHandler) SendNotification(ctx context.Context, notification *ErrorNotification) error {
	// Create email content
	subject := h.config.Subject
	if subject == "" {
		subject = fmt.Sprintf("[VoidRunner] %s - %s", notification.Severity, notification.Title)
	}
	
	body := h.createEmailBody(notification)
	
	// Create message
	message := fmt.Sprintf("To: %s\r\n", h.config.ToAddresses[0])
	message += fmt.Sprintf("Subject: %s\r\n", subject)
	message += "Content-Type: text/html; charset=UTF-8\r\n"
	message += "\r\n"
	message += body
	
	// Setup authentication
	auth := smtp.PlainAuth("", h.config.Username, h.config.Password, h.config.SMTPHost)
	
	// Send email
	addr := fmt.Sprintf("%s:%s", h.config.SMTPHost, h.config.SMTPPort)
	err := smtp.SendMail(addr, auth, h.config.FromAddress, h.config.ToAddresses, []byte(message))
	if err != nil {
		h.logger.Error("failed to send email notification",
			"notification_id", notification.ID,
			"error", err)
		return fmt.Errorf("failed to send email: %w", err)
	}
	
	h.logger.Info("email notification sent successfully",
		"notification_id", notification.ID,
		"recipients", len(h.config.ToAddresses))
	
	return nil
}

// createEmailBody creates an HTML email body
func (h *EmailNotificationHandler) createEmailBody(notification *ErrorNotification) string {
	severityColor := "#007bff" // Default blue
	switch notification.Severity {
	case "critical":
		severityColor = "#dc3545" // Red
	case "high":
		severityColor = "#fd7e14" // Orange
	case "medium":
		severityColor = "#ffc107" // Yellow
	case "low":
		severityColor = "#28a745" // Green
	}
	
	html := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .header { background-color: %s; color: white; padding: 20px; border-radius: 5px; }
        .content { padding: 20px; }
        .section { margin-bottom: 20px; }
        .error-list { background-color: #f8f9fa; padding: 15px; border-radius: 5px; }
        .recommendation { background-color: #e7f3ff; padding: 10px; margin: 5px 0; border-radius: 3px; }
        .footer { font-size: 12px; color: #666; margin-top: 30px; }
    </style>
</head>
<body>
    <div class="header">
        <h1>%s</h1>
        <p>Severity: %s | Time: %s</p>
    </div>
    
    <div class="content">
        <div class="section">
            <h2>Summary</h2>
            <p>%s</p>
            <p><strong>Error Count:</strong> %d</p>
            <p><strong>Time Range:</strong> %s to %s</p>
        </div>`,
		severityColor,
		notification.Title,
		notification.Severity,
		notification.Timestamp.Format("2006-01-02 15:04:05"),
		notification.Message,
		notification.ErrorCount,
		notification.TimeRange.Start.Format("2006-01-02 15:04:05"),
		notification.TimeRange.End.Format("2006-01-02 15:04:05"))
	
	// Add top errors if any
	if len(notification.TopErrors) > 0 {
		html += `
        <div class="section">
            <h2>Top Errors</h2>
            <div class="error-list">`
		
		for _, topError := range notification.TopErrors {
			html += fmt.Sprintf(`
                <div>
                    <strong>%s (%s):</strong> %d occurrences<br>
                    <em>%s</em><br>
                    Last seen: %s
                </div><br>`,
				topError.Code,
				topError.Type,
				topError.Count,
				topError.Message,
				topError.LastOccurrence.Format("2006-01-02 15:04:05"))
		}
		
		html += `
            </div>
        </div>`
	}
	
	// Add recommendations if any
	if len(notification.Recommendations) > 0 {
		html += `
        <div class="section">
            <h2>Recommendations</h2>`
		
		for _, rec := range notification.Recommendations {
			html += fmt.Sprintf(`
            <div class="recommendation">
                <strong>%s</strong> (Priority: %s)<br>
                %s<br>
                <em>Impact:</em> %s | <em>Effort:</em> %s
            </div>`,
				rec.Title,
				rec.Priority,
				rec.Description,
				rec.Impact,
				rec.Effort)
		}
		
		html += `
        </div>`
	}
	
	html += `
    </div>
    
    <div class="footer">
        <p>This notification was generated by VoidRunner Error Reporting System.<br>
        Notification ID: ` + notification.ID + `</p>
    </div>
</body>
</html>`
	
	return html
}

// FileNotificationHandler writes notifications to files
type FileNotificationHandler struct {
	directory string
	logger    *slog.Logger
}

// NewFileNotificationHandler creates a new file notification handler
func NewFileNotificationHandler(directory string, logger *slog.Logger) (*FileNotificationHandler, error) {
	if logger == nil {
		logger = slog.Default()
	}
	
	// Ensure directory exists
	if err := os.MkdirAll(directory, 0755); err != nil {
		return nil, fmt.Errorf("failed to create notification directory: %w", err)
	}
	
	return &FileNotificationHandler{
		directory: directory,
		logger:    logger.With("component", "file_notification_handler"),
	}, nil
}

// SendNotification writes the notification to a file
func (h *FileNotificationHandler) SendNotification(ctx context.Context, notification *ErrorNotification) error {
	filename := fmt.Sprintf("notification-%s-%s.json",
		notification.Timestamp.Format("20060102-150405"),
		notification.ID)
	
	filepath := filepath.Join(h.directory, filename)
	
	data, err := json.MarshalIndent(notification, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal notification: %w", err)
	}
	
	err = os.WriteFile(filepath, data, 0644)
	if err != nil {
		h.logger.Error("failed to write notification file",
			"filepath", filepath,
			"notification_id", notification.ID,
			"error", err)
		return fmt.Errorf("failed to write notification file: %w", err)
	}
	
	h.logger.Info("notification written to file",
		"filepath", filepath,
		"notification_id", notification.ID)
	
	return nil
}

// SlackNotificationHandler sends notifications to Slack
type SlackNotificationHandler struct {
	webhookURL string
	channel    string
	username   string
	client     *http.Client
	logger     *slog.Logger
}

// SlackConfig configures Slack notifications
type SlackConfig struct {
	WebhookURL string        `json:"webhook_url"`
	Channel    string        `json:"channel"`
	Username   string        `json:"username"`
	Timeout    time.Duration `json:"timeout"`
}

// SlackMessage represents a Slack message
type SlackMessage struct {
	Channel     string            `json:"channel,omitempty"`
	Username    string            `json:"username,omitempty"`
	Text        string            `json:"text"`
	Attachments []SlackAttachment `json:"attachments,omitempty"`
}

// SlackAttachment represents a Slack message attachment
type SlackAttachment struct {
	Color      string              `json:"color,omitempty"`
	Title      string              `json:"title,omitempty"`
	Text       string              `json:"text,omitempty"`
	Fields     []SlackField        `json:"fields,omitempty"`
	Footer     string              `json:"footer,omitempty"`
	Timestamp  int64               `json:"ts,omitempty"`
}

// SlackField represents a field in a Slack attachment
type SlackField struct {
	Title string `json:"title"`
	Value string `json:"value"`
	Short bool   `json:"short"`
}

// NewSlackNotificationHandler creates a new Slack notification handler
func NewSlackNotificationHandler(config *SlackConfig, logger *slog.Logger) *SlackNotificationHandler {
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	
	if logger == nil {
		logger = slog.Default()
	}
	
	return &SlackNotificationHandler{
		webhookURL: config.WebhookURL,
		channel:    config.Channel,
		username:   config.Username,
		client: &http.Client{
			Timeout: config.Timeout,
		},
		logger: logger.With("component", "slack_notification_handler"),
	}
}

// SendNotification sends the notification to Slack
func (h *SlackNotificationHandler) SendNotification(ctx context.Context, notification *ErrorNotification) error {
	color := "#36a64f" // Green default
	switch notification.Severity {
	case "critical":
		color = "#ff0000" // Red
	case "high":
		color = "#ff9900" // Orange
	case "medium":
		color = "#ffcc00" // Yellow
	}
	
	fields := []SlackField{
		{Title: "Severity", Value: notification.Severity, Short: true},
		{Title: "Error Count", Value: fmt.Sprintf("%d", notification.ErrorCount), Short: true},
		{Title: "Time Range", Value: fmt.Sprintf("%s to %s",
			notification.TimeRange.Start.Format("15:04:05"),
			notification.TimeRange.End.Format("15:04:05")), Short: false},
	}
	
	// Add top errors
	if len(notification.TopErrors) > 0 {
		topErrorsText := ""
		for i, topError := range notification.TopErrors {
			if i >= 3 { // Limit to top 3
				break
			}
			topErrorsText += fmt.Sprintf("• %s: %d occurrences\n", topError.Code, topError.Count)
		}
		fields = append(fields, SlackField{
			Title: "Top Errors",
			Value: topErrorsText,
			Short: false,
		})
	}
	
	attachment := SlackAttachment{
		Color:     color,
		Title:     notification.Title,
		Text:      notification.Message,
		Fields:    fields,
		Footer:    "VoidRunner Error Reporting",
		Timestamp: notification.Timestamp.Unix(),
	}
	
	message := SlackMessage{
		Channel:     h.channel,
		Username:    h.username,
		Text:        fmt.Sprintf("🚨 *%s Error Notification*", notification.Severity),
		Attachments: []SlackAttachment{attachment},
	}
	
	payload, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal Slack message: %w", err)
	}
	
	req, err := http.NewRequestWithContext(ctx, "POST", h.webhookURL, bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("failed to create Slack request: %w", err)
	}
	
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := h.client.Do(req)
	if err != nil {
		h.logger.Error("Slack webhook request failed",
			"notification_id", notification.ID,
			"error", err)
		return fmt.Errorf("Slack webhook request failed: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		h.logger.Error("Slack webhook returned error status",
			"notification_id", notification.ID,
			"status_code", resp.StatusCode)
		return fmt.Errorf("Slack webhook returned status %d", resp.StatusCode)
	}
	
	h.logger.Info("Slack notification sent successfully",
		"notification_id", notification.ID,
		"channel", h.channel)
	
	return nil
}

// CompositeNotificationHandler combines multiple notification handlers
type CompositeNotificationHandler struct {
	handlers []NotificationHandler
	logger   *slog.Logger
}

// NewCompositeNotificationHandler creates a new composite notification handler
func NewCompositeNotificationHandler(handlers []NotificationHandler, logger *slog.Logger) *CompositeNotificationHandler {
	if logger == nil {
		logger = slog.Default()
	}
	
	return &CompositeNotificationHandler{
		handlers: handlers,
		logger:   logger.With("component", "composite_notification_handler"),
	}
}

// SendNotification sends the notification using all handlers
func (h *CompositeNotificationHandler) SendNotification(ctx context.Context, notification *ErrorNotification) error {
	var errors []error
	
	for i, handler := range h.handlers {
		if err := handler.SendNotification(ctx, notification); err != nil {
			h.logger.Error("handler failed to send notification",
				"handler_index", i,
				"notification_id", notification.ID,
				"error", err)
			errors = append(errors, err)
		}
	}
	
	if len(errors) > 0 {
		return fmt.Errorf("failed to send notification with %d handlers: %v", len(errors), errors)
	}
	
	h.logger.Info("notification sent successfully to all handlers",
		"notification_id", notification.ID,
		"handler_count", len(h.handlers))
	
	return nil
}

// AddHandler adds a new notification handler
func (h *CompositeNotificationHandler) AddHandler(handler NotificationHandler) {
	h.handlers = append(h.handlers, handler)
}

// RemoveHandler removes a notification handler
func (h *CompositeNotificationHandler) RemoveHandler(handler NotificationHandler) {
	for i, existingHandler := range h.handlers {
		if existingHandler == handler {
			h.handlers = append(h.handlers[:i], h.handlers[i+1:]...)
			break
		}
	}
}
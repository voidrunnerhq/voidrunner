package monitoring

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// ResourceMonitor monitors system resources and triggers alerts
type ResourceMonitor struct {
	mu             sync.RWMutex
	config         *MonitoringConfig
	logger         *slog.Logger
	alertManager   *AlertManager
	metricsCollector *MetricsCollector
	
	// State management
	isRunning      bool
	ctx            context.Context
	cancel         context.CancelFunc
	ticker         *time.Ticker
	
	// Monitoring state
	lastMetrics    *SystemMetrics
	healthStatus   HealthStatus
	startTime      time.Time
}

// HealthStatus represents the overall health of the system
type HealthStatus struct {
	Healthy        bool                   `json:"healthy"`
	LastCheck      time.Time              `json:"last_check"`
	Issues         []string               `json:"issues,omitempty"`
	ComponentStats map[string]interface{} `json:"component_stats"`
}

// NewResourceMonitor creates a new resource monitor
func NewResourceMonitor(
	config *MonitoringConfig,
	alertManager *AlertManager,
	metricsCollector *MetricsCollector,
	logger *slog.Logger,
) *ResourceMonitor {
	if config == nil {
		config = DefaultMonitoringConfig()
	}
	if logger == nil {
		logger = slog.Default()
	}

	return &ResourceMonitor{
		config:           config,
		logger:           logger.With("component", "resource_monitor"),
		alertManager:     alertManager,
		metricsCollector: metricsCollector,
		healthStatus: HealthStatus{
			Healthy:        true,
			ComponentStats: make(map[string]interface{}),
		},
		startTime: time.Now(),
	}
}

// Start begins resource monitoring
func (rm *ResourceMonitor) Start(ctx context.Context) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if rm.isRunning {
		return fmt.Errorf("resource monitor is already running")
	}

	// Validate configuration
	if err := rm.config.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	rm.ctx, rm.cancel = context.WithCancel(ctx)
	rm.ticker = time.NewTicker(rm.config.CheckInterval)
	rm.isRunning = true

	rm.logger.Info("starting resource monitor", 
		"check_interval", rm.config.CheckInterval,
		"thresholds", rm.config.Thresholds)

	// Start monitoring goroutine
	go rm.monitoringLoop()

	// Perform initial check
	go func() {
		if err := rm.performHealthCheck(); err != nil {
			rm.logger.Error("initial health check failed", "error", err)
		}
	}()

	return nil
}

// Stop stops resource monitoring
func (rm *ResourceMonitor) Stop() error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if !rm.isRunning {
		return fmt.Errorf("resource monitor is not running")
	}

	rm.logger.Info("stopping resource monitor")

	rm.cancel()
	if rm.ticker != nil {
		rm.ticker.Stop()
	}
	rm.isRunning = false

	return nil
}

// IsRunning returns whether the monitor is currently running
func (rm *ResourceMonitor) IsRunning() bool {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.isRunning
}

// GetHealthStatus returns the current health status
func (rm *ResourceMonitor) GetHealthStatus() HealthStatus {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	
	// Return a copy to avoid data races
	statusCopy := rm.healthStatus
	statusCopy.ComponentStats = make(map[string]interface{})
	for k, v := range rm.healthStatus.ComponentStats {
		statusCopy.ComponentStats[k] = v
	}
	
	return statusCopy
}

// GetCurrentMetrics returns the most recent metrics
func (rm *ResourceMonitor) GetCurrentMetrics() *SystemMetrics {
	return rm.metricsCollector.GetCurrentMetrics()
}

// GetMetricsHistory returns historical metrics
func (rm *ResourceMonitor) GetMetricsHistory(since time.Time) []SystemMetrics {
	return rm.metricsCollector.GetMetricsHistory(since)
}

// ForceHealthCheck manually triggers a health check
func (rm *ResourceMonitor) ForceHealthCheck() error {
	return rm.performHealthCheck()
}

// monitoringLoop is the main monitoring loop
func (rm *ResourceMonitor) monitoringLoop() {
	defer func() {
		if r := recover(); r != nil {
			rm.logger.Error("monitoring loop panic recovered", "panic", r)
		}
	}()

	for {
		select {
		case <-rm.ctx.Done():
			rm.logger.Info("monitoring loop stopped")
			return
		case <-rm.ticker.C:
			if err := rm.performHealthCheck(); err != nil {
				rm.logger.Error("health check failed", "error", err)
				
				// Send critical alert for monitoring failure
				if rm.alertManager != nil {
					_ = rm.alertManager.SendAlert(
						rm.ctx,
						"MONITORING_FAILURE",
						"Resource Monitoring Failed",
						fmt.Sprintf("Failed to perform health check: %v", err),
						AlertLevelCritical,
						map[string]interface{}{
							"error": err.Error(),
							"component": "resource_monitor",
						},
					)
				}
			}
		}
	}
}

// performHealthCheck collects metrics and evaluates thresholds
func (rm *ResourceMonitor) performHealthCheck() error {
	checkStart := time.Now()
	
	// Collect current metrics
	metrics, err := rm.metricsCollector.CollectMetrics(rm.ctx)
	if err != nil {
		return fmt.Errorf("failed to collect metrics: %w", err)
	}

	rm.mu.Lock()
	rm.lastMetrics = metrics
	rm.mu.Unlock()

	// Evaluate thresholds and send alerts
	if err := rm.evaluateThresholds(metrics); err != nil {
		rm.logger.Error("threshold evaluation failed", "error", err)
	}

	// Update health status
	rm.updateHealthStatus(metrics, time.Since(checkStart))

	rm.logger.Debug("health check completed", 
		"duration", time.Since(checkStart),
		"cpu_percent", metrics.CPUPercent,
		"memory_percent", metrics.MemoryPercent,
		"disk_percent", metrics.DiskPercent)

	return nil
}

// evaluateThresholds checks metrics against configured thresholds
func (rm *ResourceMonitor) evaluateThresholds(metrics *SystemMetrics) error {
	thresholds := rm.config.Thresholds
	
	// CPU threshold evaluation
	if rm.config.EnableCPUMonitoring {
		cpuLevel := thresholds.EvaluateCPUThreshold(metrics.CPUPercent)
		if cpuLevel != AlertLevelInfo {
			err := rm.alertManager.SendAlert(
				rm.ctx,
				"HIGH_CPU_USAGE",
				"High CPU Usage",
				fmt.Sprintf("CPU usage is %.2f%% (threshold: %.2f%%)", 
					metrics.CPUPercent, 
					getThresholdValue(cpuLevel, thresholds.CPUWarningPercent, thresholds.CPUCriticalPercent)),
				cpuLevel,
				map[string]interface{}{
					"cpu_percent": metrics.CPUPercent,
					"cpu_cores": metrics.CPUCores,
					"load_average_1min": metrics.LoadAverage1Min,
				},
			)
			if err != nil {
				rm.logger.Error("failed to send CPU alert", "error", err)
			}
		}
	}

	// Memory threshold evaluation
	if rm.config.EnableMemoryMonitoring {
		memoryLevel := thresholds.EvaluateMemoryThreshold(metrics.MemoryPercent)
		if memoryLevel != AlertLevelInfo {
			err := rm.alertManager.SendAlert(
				rm.ctx,
				"HIGH_MEMORY_USAGE",
				"High Memory Usage",
				fmt.Sprintf("Memory usage is %.2f%% (%s used of %s total)", 
					metrics.MemoryPercent,
					FormatBytes(metrics.MemoryUsedBytes),
					FormatBytes(metrics.MemoryTotalBytes)),
				memoryLevel,
				map[string]interface{}{
					"memory_percent": metrics.MemoryPercent,
					"memory_used_bytes": metrics.MemoryUsedBytes,
					"memory_total_bytes": metrics.MemoryTotalBytes,
				},
			)
			if err != nil {
				rm.logger.Error("failed to send memory alert", "error", err)
			}
		}
	}

	// Disk threshold evaluation
	if rm.config.EnableDiskMonitoring {
		diskLevel := thresholds.EvaluateDiskThreshold(metrics.DiskPercent)
		if diskLevel != AlertLevelInfo {
			err := rm.alertManager.SendAlert(
				rm.ctx,
				"HIGH_DISK_USAGE",
				"High Disk Usage",
				fmt.Sprintf("Disk usage is %.2f%% (%s used of %s total)", 
					metrics.DiskPercent,
					FormatBytes(metrics.DiskUsedBytes),
					FormatBytes(metrics.DiskTotalBytes)),
				diskLevel,
				map[string]interface{}{
					"disk_percent": metrics.DiskPercent,
					"disk_used_bytes": metrics.DiskUsedBytes,
					"disk_total_bytes": metrics.DiskTotalBytes,
				},
			)
			if err != nil {
				rm.logger.Error("failed to send disk alert", "error", err)
			}
		}
	}

	// Container count threshold evaluation
	if rm.config.EnableDockerMonitoring {
		containerLevel := thresholds.EvaluateContainerThreshold(metrics.ContainerCount)
		if containerLevel != AlertLevelInfo {
			err := rm.alertManager.SendAlert(
				rm.ctx,
				"HIGH_CONTAINER_COUNT",
				"High Container Count",
				fmt.Sprintf("Container count is %d (threshold: %d)", 
					metrics.ContainerCount,
					getThresholdValueInt(containerLevel, thresholds.ContainerWarningCount, thresholds.ContainerCriticalCount)),
				containerLevel,
				map[string]interface{}{
					"container_count": metrics.ContainerCount,
					"running_container_count": metrics.RunningContainerCount,
				},
			)
			if err != nil {
				rm.logger.Error("failed to send container count alert", "error", err)
			}
		}

		// Docker daemon responsiveness
		if !metrics.DockerDaemonResponsive {
			err := rm.alertManager.SendAlert(
				rm.ctx,
				"DOCKER_DAEMON_UNRESPONSIVE",
				"Docker Daemon Unresponsive",
				fmt.Sprintf("Docker daemon failed to respond within timeout"),
				AlertLevelCritical,
				map[string]interface{}{
					"docker_response_time": metrics.DockerResponseTime.String(),
				},
			)
			if err != nil {
				rm.logger.Error("failed to send Docker daemon alert", "error", err)
			}
		} else {
			// Check Docker response time
			responseTimeMs := int(metrics.DockerResponseTime.Milliseconds())
			responseTimeLevel := thresholds.EvaluateDockerResponseTimeThreshold(responseTimeMs)
			if responseTimeLevel != AlertLevelInfo {
				err := rm.alertManager.SendAlert(
					rm.ctx,
					"SLOW_DOCKER_RESPONSE",
					"Slow Docker Response Time",
					fmt.Sprintf("Docker response time is %dms (threshold: %dms)", 
						responseTimeMs,
						getThresholdValueInt(responseTimeLevel, thresholds.DockerResponseTimeWarningMs, thresholds.DockerResponseTimeCriticalMs)),
					responseTimeLevel,
					map[string]interface{}{
						"docker_response_time_ms": responseTimeMs,
					},
				)
				if err != nil {
					rm.logger.Error("failed to send Docker response time alert", "error", err)
				}
			}
		}
	}

	// Error rate threshold evaluation
	if rm.config.EnableErrorRateMonitoring {
		errorRateLevel := thresholds.EvaluateErrorRateThreshold(metrics.ErrorRate)
		if errorRateLevel != AlertLevelInfo {
			err := rm.alertManager.SendAlert(
				rm.ctx,
				"HIGH_ERROR_RATE",
				"High Error Rate",
				fmt.Sprintf("Error rate is %.2f%% (%d errors of %d requests)", 
					metrics.ErrorRate,
					metrics.TotalErrors,
					metrics.TotalRequests),
				errorRateLevel,
				map[string]interface{}{
					"error_rate": metrics.ErrorRate,
					"total_errors": metrics.TotalErrors,
					"total_requests": metrics.TotalRequests,
				},
			)
			if err != nil {
				rm.logger.Error("failed to send error rate alert", "error", err)
			}
		}
	}

	return nil
}

// updateHealthStatus updates the overall health status
func (rm *ResourceMonitor) updateHealthStatus(metrics *SystemMetrics, checkDuration time.Duration) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	rm.healthStatus.LastCheck = time.Now()
	rm.healthStatus.Issues = nil
	rm.healthStatus.Healthy = true

	thresholds := rm.config.Thresholds

	// Check if any metrics exceed critical thresholds
	if rm.config.EnableCPUMonitoring && metrics.CPUPercent >= thresholds.CPUCriticalPercent {
		rm.healthStatus.Healthy = false
		rm.healthStatus.Issues = append(rm.healthStatus.Issues, 
			fmt.Sprintf("CPU usage critical: %.2f%%", metrics.CPUPercent))
	}

	if rm.config.EnableMemoryMonitoring && metrics.MemoryPercent >= thresholds.MemoryCriticalPercent {
		rm.healthStatus.Healthy = false
		rm.healthStatus.Issues = append(rm.healthStatus.Issues, 
			fmt.Sprintf("Memory usage critical: %.2f%%", metrics.MemoryPercent))
	}

	if rm.config.EnableDiskMonitoring && metrics.DiskPercent >= thresholds.DiskCriticalPercent {
		rm.healthStatus.Healthy = false
		rm.healthStatus.Issues = append(rm.healthStatus.Issues, 
			fmt.Sprintf("Disk usage critical: %.2f%%", metrics.DiskPercent))
	}

	if rm.config.EnableDockerMonitoring && !metrics.DockerDaemonResponsive {
		rm.healthStatus.Healthy = false
		rm.healthStatus.Issues = append(rm.healthStatus.Issues, "Docker daemon unresponsive")
	}

	if rm.config.EnableErrorRateMonitoring && metrics.ErrorRate >= thresholds.ErrorRateCriticalPercent {
		rm.healthStatus.Healthy = false
		rm.healthStatus.Issues = append(rm.healthStatus.Issues, 
			fmt.Sprintf("Error rate critical: %.2f%%", metrics.ErrorRate))
	}

	// Update component stats
	rm.healthStatus.ComponentStats = map[string]interface{}{
		"uptime_seconds":      time.Since(rm.startTime).Seconds(),
		"check_duration_ms":   checkDuration.Milliseconds(),
		"last_check":          metrics.Timestamp,
		"metrics_collected":   true,
		"cpu_percent":         metrics.CPUPercent,
		"memory_percent":      metrics.MemoryPercent,
		"disk_percent":        metrics.DiskPercent,
		"container_count":     metrics.ContainerCount,
		"docker_responsive":   metrics.DockerDaemonResponsive,
		"error_rate":          metrics.ErrorRate,
	}
}

// GetUptime returns how long the monitor has been running
func (rm *ResourceMonitor) GetUptime() time.Duration {
	return time.Since(rm.startTime)
}

// RecordError increments the error counter
func (rm *ResourceMonitor) RecordError() {
	rm.metricsCollector.RecordError()
}

// RecordRequest increments the request counter
func (rm *ResourceMonitor) RecordRequest() {
	rm.metricsCollector.RecordRequest()
}

// Helper functions

func getThresholdValue(level AlertLevel, warning, critical float64) float64 {
	if level == AlertLevelCritical {
		return critical
	}
	return warning
}

func getThresholdValueInt(level AlertLevel, warning, critical int) int {
	if level == AlertLevelCritical {
		return critical
	}
	return warning
}
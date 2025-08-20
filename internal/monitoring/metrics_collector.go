package monitoring

import (
	"context"
	"fmt"
	"log/slog"
	"runtime"
	"sync"
	"syscall"
	"time"
)

// SystemMetrics represents system resource metrics
type SystemMetrics struct {
	Timestamp time.Time `json:"timestamp"`

	// CPU metrics
	CPUPercent       float64 `json:"cpu_percent"`
	CPUCores         int     `json:"cpu_cores"`
	LoadAverage1Min  float64 `json:"load_average_1min"`
	LoadAverage5Min  float64 `json:"load_average_5min"`
	LoadAverage15Min float64 `json:"load_average_15min"`

	// Memory metrics
	MemoryUsedBytes      uint64  `json:"memory_used_bytes"`
	MemoryTotalBytes     uint64  `json:"memory_total_bytes"`
	MemoryPercent        float64 `json:"memory_percent"`
	MemoryAvailableBytes uint64  `json:"memory_available_bytes"`

	// Disk metrics
	DiskUsedBytes      uint64  `json:"disk_used_bytes"`
	DiskTotalBytes     uint64  `json:"disk_total_bytes"`
	DiskPercent        float64 `json:"disk_percent"`
	DiskAvailableBytes uint64  `json:"disk_available_bytes"`

	// Container metrics
	ContainerCount        int `json:"container_count"`
	RunningContainerCount int `json:"running_container_count"`

	// Docker metrics
	DockerDaemonResponsive bool          `json:"docker_daemon_responsive"`
	DockerResponseTime     time.Duration `json:"docker_response_time"`

	// Error metrics
	ErrorRate     float64 `json:"error_rate"`
	TotalErrors   int64   `json:"total_errors"`
	TotalRequests int64   `json:"total_requests"`
}

// MetricsCollector collects system and application metrics
type MetricsCollector struct {
	mu     sync.RWMutex
	logger *slog.Logger
	config *MonitoringConfig

	// Metrics storage
	currentMetrics *SystemMetrics
	metricsHistory []SystemMetrics

	// Docker client for container metrics
	dockerClient interface {
		IsHealthy(ctx context.Context) error
		ListContainers(ctx context.Context, includeAll bool) ([]ContainerInfo, error)
	}

	// Error tracking
	errorCount   int64
	requestCount int64

	// Last collection time for rate calculations
	lastCPUTimes    CPUTimes
	lastCollectTime time.Time
}

// CPUTimes represents CPU time measurements
type CPUTimes struct {
	User   uint64
	System uint64
	Idle   uint64
	Total  uint64
}

// ContainerInfo represents basic container information
type ContainerInfo struct {
	ID    string
	State string
	Names []string
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector(config *MonitoringConfig, dockerClient interface {
	IsHealthy(ctx context.Context) error
	ListContainers(ctx context.Context, includeAll bool) ([]ContainerInfo, error)
}, logger *slog.Logger) *MetricsCollector {
	if config == nil {
		config = DefaultMonitoringConfig()
	}
	if logger == nil {
		logger = slog.Default()
	}

	return &MetricsCollector{
		logger:          logger.With("component", "metrics_collector"),
		config:          config,
		dockerClient:    dockerClient,
		metricsHistory:  make([]SystemMetrics, 0),
		lastCollectTime: time.Now(),
	}
}

// CollectMetrics gathers current system metrics
func (mc *MetricsCollector) CollectMetrics(ctx context.Context) (*SystemMetrics, error) {
	metrics := &SystemMetrics{
		Timestamp: time.Now(),
	}

	var wg sync.WaitGroup
	var errors []error
	var errorsMu sync.Mutex

	addError := func(err error) {
		if err != nil {
			errorsMu.Lock()
			errors = append(errors, err)
			errorsMu.Unlock()
		}
	}

	// Collect CPU metrics
	if mc.config.EnableCPUMonitoring {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := mc.collectCPUMetrics(metrics); err != nil {
				addError(fmt.Errorf("CPU metrics collection failed: %w", err))
			}
		}()
	}

	// Collect memory metrics
	if mc.config.EnableMemoryMonitoring {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := mc.collectMemoryMetrics(metrics); err != nil {
				addError(fmt.Errorf("memory metrics collection failed: %w", err))
			}
		}()
	}

	// Collect disk metrics
	if mc.config.EnableDiskMonitoring {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := mc.collectDiskMetrics(metrics); err != nil {
				addError(fmt.Errorf("disk metrics collection failed: %w", err))
			}
		}()
	}

	// Collect Docker metrics
	if mc.config.EnableDockerMonitoring && mc.dockerClient != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := mc.collectDockerMetrics(ctx, metrics); err != nil {
				addError(fmt.Errorf("docker metrics collection failed: %w", err))
			}
		}()
	}

	// Collect error metrics
	if mc.config.EnableErrorRateMonitoring {
		wg.Add(1)
		go func() {
			defer wg.Done()
			mc.collectErrorMetrics(metrics)
		}()
	}

	wg.Wait()

	// Update stored metrics
	mc.mu.Lock()
	mc.currentMetrics = metrics
	mc.metricsHistory = append(mc.metricsHistory, *metrics)

	// Keep only recent history
	maxHistorySize := int(mc.config.MetricsRetentionTime / mc.config.CheckInterval)
	if len(mc.metricsHistory) > maxHistorySize {
		mc.metricsHistory = mc.metricsHistory[len(mc.metricsHistory)-maxHistorySize:]
	}
	mc.mu.Unlock()

	// Return any collection errors
	if len(errors) > 0 {
		return metrics, fmt.Errorf("metrics collection errors: %v", errors)
	}

	return metrics, nil
}

// collectCPUMetrics gathers CPU usage metrics
func (mc *MetricsCollector) collectCPUMetrics(metrics *SystemMetrics) error {
	// Get CPU count
	metrics.CPUCores = runtime.NumCPU()

	// Get CPU times for percentage calculation
	cpuTimes, err := mc.getCPUTimes()
	if err != nil {
		return fmt.Errorf("failed to get CPU times: %w", err)
	}

	// Calculate CPU percentage if we have previous measurement
	mc.mu.Lock()
	if mc.lastCPUTimes.Total > 0 {
		totalDiff := cpuTimes.Total - mc.lastCPUTimes.Total
		idleDiff := cpuTimes.Idle - mc.lastCPUTimes.Idle

		if totalDiff > 0 {
			metrics.CPUPercent = 100.0 * (1.0 - float64(idleDiff)/float64(totalDiff))
		}
	}

	mc.lastCPUTimes = cpuTimes
	mc.mu.Unlock()

	// Get load averages (Unix/Linux specific)
	if err := mc.getLoadAverages(metrics); err != nil {
		mc.logger.Debug("failed to get load averages", "error", err)
		// Not critical on all platforms
	}

	return nil
}

// collectMemoryMetrics gathers memory usage metrics
func (mc *MetricsCollector) collectMemoryMetrics(metrics *SystemMetrics) error {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	// Get system memory info
	if err := mc.getSystemMemoryInfo(metrics); err != nil {
		// Fallback to runtime stats
		metrics.MemoryUsedBytes = memStats.Sys
		metrics.MemoryTotalBytes = memStats.Sys * 2 // Rough estimate
		mc.logger.Debug("using runtime memory stats as fallback", "error", err)
	}

	if metrics.MemoryTotalBytes > 0 {
		metrics.MemoryPercent = 100.0 * float64(metrics.MemoryUsedBytes) / float64(metrics.MemoryTotalBytes)
		metrics.MemoryAvailableBytes = metrics.MemoryTotalBytes - metrics.MemoryUsedBytes
	}

	return nil
}

// collectDiskMetrics gathers disk usage metrics
func (mc *MetricsCollector) collectDiskMetrics(metrics *SystemMetrics) error {
	if err := mc.getDiskUsage(".", metrics); err != nil {
		return fmt.Errorf("failed to get disk usage: %w", err)
	}

	if metrics.DiskTotalBytes > 0 {
		metrics.DiskPercent = 100.0 * float64(metrics.DiskUsedBytes) / float64(metrics.DiskTotalBytes)
		metrics.DiskAvailableBytes = metrics.DiskTotalBytes - metrics.DiskUsedBytes
	}

	return nil
}

// collectDockerMetrics gathers Docker-related metrics
func (mc *MetricsCollector) collectDockerMetrics(ctx context.Context, metrics *SystemMetrics) error {
	start := time.Now()

	// Check Docker daemon health
	err := mc.dockerClient.IsHealthy(ctx)
	metrics.DockerResponseTime = time.Since(start)
	metrics.DockerDaemonResponsive = (err == nil)

	if err != nil {
		mc.logger.Debug("Docker daemon health check failed", "error", err)
		return nil // Don't fail the entire collection
	}

	// Get container counts
	containers, err := mc.dockerClient.ListContainers(ctx, true)
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}

	metrics.ContainerCount = len(containers)
	runningCount := 0
	for _, container := range containers {
		if container.State == "running" {
			runningCount++
		}
	}
	metrics.RunningContainerCount = runningCount

	return nil
}

// collectErrorMetrics gathers error rate metrics
func (mc *MetricsCollector) collectErrorMetrics(metrics *SystemMetrics) {
	mc.mu.RLock()
	metrics.TotalErrors = mc.errorCount
	metrics.TotalRequests = mc.requestCount
	mc.mu.RUnlock()

	if metrics.TotalRequests > 0 {
		metrics.ErrorRate = 100.0 * float64(metrics.TotalErrors) / float64(metrics.TotalRequests)
	}
}

// RecordError increments the error counter
func (mc *MetricsCollector) RecordError() {
	mc.mu.Lock()
	mc.errorCount++
	mc.mu.Unlock()
}

// RecordRequest increments the request counter
func (mc *MetricsCollector) RecordRequest() {
	mc.mu.Lock()
	mc.requestCount++
	mc.mu.Unlock()
}

// GetCurrentMetrics returns the most recent metrics
func (mc *MetricsCollector) GetCurrentMetrics() *SystemMetrics {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	if mc.currentMetrics == nil {
		return nil
	}

	// Return a copy to avoid data races
	metricsCopy := *mc.currentMetrics
	return &metricsCopy
}

// GetMetricsHistory returns historical metrics
func (mc *MetricsCollector) GetMetricsHistory(since time.Time) []SystemMetrics {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	var result []SystemMetrics
	for _, metrics := range mc.metricsHistory {
		if metrics.Timestamp.After(since) {
			result = append(result, metrics)
		}
	}

	return result
}

// ResetCounters resets error and request counters
func (mc *MetricsCollector) ResetCounters() {
	mc.mu.Lock()
	mc.errorCount = 0
	mc.requestCount = 0
	mc.mu.Unlock()
}

// Platform-specific implementations

// getCPUTimes gets CPU time information (Unix/Linux implementation)
func (mc *MetricsCollector) getCPUTimes() (CPUTimes, error) {
	// This is a simplified implementation
	// In a production system, you'd read from /proc/stat on Linux
	// or use appropriate system calls on other platforms

	// For now, return dummy data
	now := time.Now().Unix()
	if now < 0 {
		now = 0
	}
	
	// Safe conversion to avoid integer overflow - now is guaranteed >= 0
	var baseTime uint64
	if now >= 0 {
		baseTime = uint64(now)
	} else {
		baseTime = 0
	}
	return CPUTimes{
		User:   baseTime * 1000,
		System: baseTime * 100,
		Idle:   baseTime * 10000,
		Total:  baseTime * 11100,
	}, nil
}

// getLoadAverages gets system load averages
func (mc *MetricsCollector) getLoadAverages(metrics *SystemMetrics) error {
	// Platform-specific implementation would go here
	// For now, return dummy data
	metrics.LoadAverage1Min = 0.5
	metrics.LoadAverage5Min = 0.3
	metrics.LoadAverage15Min = 0.2
	return nil
}

// getSystemMemoryInfo gets system memory information
func (mc *MetricsCollector) getSystemMemoryInfo(metrics *SystemMetrics) error {
	// Platform-specific implementation would go here
	// For macOS, we can use syscall to get memory info
	// For now, return dummy data for cross-platform compatibility
	metrics.MemoryTotalBytes = 8 * 1024 * 1024 * 1024 // 8GB
	metrics.MemoryUsedBytes = 4 * 1024 * 1024 * 1024  // 4GB
	return nil
}

// getDiskUsage gets disk usage for a path
func (mc *MetricsCollector) getDiskUsage(path string, metrics *SystemMetrics) error {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return fmt.Errorf("failed to get disk usage for %s: %w", path, err)
	}

	// Calculate disk usage
	blockSize := uint64(stat.Bsize)
	metrics.DiskTotalBytes = stat.Blocks * blockSize
	metrics.DiskAvailableBytes = stat.Bavail * blockSize
	metrics.DiskUsedBytes = metrics.DiskTotalBytes - metrics.DiskAvailableBytes

	return nil
}

// Utility function to convert bytes to human-readable format
func FormatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// GetMemoryInfo returns current Go runtime memory statistics
func (mc *MetricsCollector) GetMemoryInfo() runtime.MemStats {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	return memStats
}

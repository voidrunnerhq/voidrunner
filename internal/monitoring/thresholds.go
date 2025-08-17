package monitoring

import (
	"fmt"
	"time"
)

// ResourceThresholds defines thresholds for resource monitoring
type ResourceThresholds struct {
	// CPU thresholds
	CPUWarningPercent  float64 `json:"cpu_warning_percent"`  // 70%
	CPUCriticalPercent float64 `json:"cpu_critical_percent"` // 85%

	// Memory thresholds
	MemoryWarningPercent  float64 `json:"memory_warning_percent"`  // 75%
	MemoryCriticalPercent float64 `json:"memory_critical_percent"` // 90%

	// Disk thresholds
	DiskWarningPercent  float64 `json:"disk_warning_percent"`  // 80%
	DiskCriticalPercent float64 `json:"disk_critical_percent"` // 95%

	// Container thresholds
	ContainerWarningCount  int `json:"container_warning_count"`  // 800
	ContainerCriticalCount int `json:"container_critical_count"` // 1000

	// Network thresholds
	NetworkLatencyWarningMs  int `json:"network_latency_warning_ms"`  // 1000ms
	NetworkLatencyCriticalMs int `json:"network_latency_critical_ms"` // 5000ms

	// Docker daemon thresholds
	DockerResponseTimeWarningMs  int `json:"docker_response_time_warning_ms"`  // 2000ms
	DockerResponseTimeCriticalMs int `json:"docker_response_time_critical_ms"` // 10000ms

	// Error rate thresholds
	ErrorRateWarningPercent  float64 `json:"error_rate_warning_percent"`  // 5%
	ErrorRateCriticalPercent float64 `json:"error_rate_critical_percent"` // 15%
}

// DefaultResourceThresholds returns sensible default thresholds
func DefaultResourceThresholds() *ResourceThresholds {
	return &ResourceThresholds{
		CPUWarningPercent:            70.0,
		CPUCriticalPercent:           85.0,
		MemoryWarningPercent:         75.0,
		MemoryCriticalPercent:        90.0,
		DiskWarningPercent:           80.0,
		DiskCriticalPercent:          95.0,
		ContainerWarningCount:        800,
		ContainerCriticalCount:       1000,
		NetworkLatencyWarningMs:      1000,
		NetworkLatencyCriticalMs:     5000,
		DockerResponseTimeWarningMs:  2000,
		DockerResponseTimeCriticalMs: 10000,
		ErrorRateWarningPercent:      5.0,
		ErrorRateCriticalPercent:     15.0,
	}
}

// Validate checks if the thresholds are valid
func (rt *ResourceThresholds) Validate() error {
	if rt.CPUWarningPercent >= rt.CPUCriticalPercent {
		return fmt.Errorf("CPU warning threshold (%v) must be less than critical threshold (%v)", 
			rt.CPUWarningPercent, rt.CPUCriticalPercent)
	}
	
	if rt.MemoryWarningPercent >= rt.MemoryCriticalPercent {
		return fmt.Errorf("memory warning threshold (%v) must be less than critical threshold (%v)", 
			rt.MemoryWarningPercent, rt.MemoryCriticalPercent)
	}
	
	if rt.DiskWarningPercent >= rt.DiskCriticalPercent {
		return fmt.Errorf("disk warning threshold (%v) must be less than critical threshold (%v)", 
			rt.DiskWarningPercent, rt.DiskCriticalPercent)
	}
	
	if rt.ContainerWarningCount >= rt.ContainerCriticalCount {
		return fmt.Errorf("container warning threshold (%d) must be less than critical threshold (%d)", 
			rt.ContainerWarningCount, rt.ContainerCriticalCount)
	}
	
	if rt.NetworkLatencyWarningMs >= rt.NetworkLatencyCriticalMs {
		return fmt.Errorf("network latency warning threshold (%d) must be less than critical threshold (%d)", 
			rt.NetworkLatencyWarningMs, rt.NetworkLatencyCriticalMs)
	}
	
	if rt.DockerResponseTimeWarningMs >= rt.DockerResponseTimeCriticalMs {
		return fmt.Errorf("Docker response time warning threshold (%d) must be less than critical threshold (%d)", 
			rt.DockerResponseTimeWarningMs, rt.DockerResponseTimeCriticalMs)
	}
	
	if rt.ErrorRateWarningPercent >= rt.ErrorRateCriticalPercent {
		return fmt.Errorf("error rate warning threshold (%v) must be less than critical threshold (%v)", 
			rt.ErrorRateWarningPercent, rt.ErrorRateCriticalPercent)
	}

	// Check that all values are positive
	if rt.CPUWarningPercent <= 0 || rt.CPUCriticalPercent <= 0 ||
		rt.MemoryWarningPercent <= 0 || rt.MemoryCriticalPercent <= 0 ||
		rt.DiskWarningPercent <= 0 || rt.DiskCriticalPercent <= 0 ||
		rt.ContainerWarningCount <= 0 || rt.ContainerCriticalCount <= 0 ||
		rt.NetworkLatencyWarningMs <= 0 || rt.NetworkLatencyCriticalMs <= 0 ||
		rt.DockerResponseTimeWarningMs <= 0 || rt.DockerResponseTimeCriticalMs <= 0 ||
		rt.ErrorRateWarningPercent <= 0 || rt.ErrorRateCriticalPercent <= 0 {
		return fmt.Errorf("all thresholds must be positive values")
	}

	// Check that percentage values are reasonable (0-100)
	if rt.CPUWarningPercent > 100 || rt.CPUCriticalPercent > 100 ||
		rt.MemoryWarningPercent > 100 || rt.MemoryCriticalPercent > 100 ||
		rt.DiskWarningPercent > 100 || rt.DiskCriticalPercent > 100 ||
		rt.ErrorRateWarningPercent > 100 || rt.ErrorRateCriticalPercent > 100 {
		return fmt.Errorf("percentage thresholds must be between 0 and 100")
	}

	return nil
}

// AlertLevel represents the severity level of an alert
type AlertLevel string

const (
	// AlertLevelInfo represents informational alerts
	AlertLevelInfo AlertLevel = "info"
	
	// AlertLevelWarning represents warning-level alerts
	AlertLevelWarning AlertLevel = "warning"
	
	// AlertLevelCritical represents critical alerts requiring immediate attention
	AlertLevelCritical AlertLevel = "critical"
)

// EvaluateCPUThreshold checks CPU usage against thresholds
func (rt *ResourceThresholds) EvaluateCPUThreshold(cpuPercent float64) AlertLevel {
	if cpuPercent >= rt.CPUCriticalPercent {
		return AlertLevelCritical
	}
	if cpuPercent >= rt.CPUWarningPercent {
		return AlertLevelWarning
	}
	return AlertLevelInfo
}

// EvaluateMemoryThreshold checks memory usage against thresholds
func (rt *ResourceThresholds) EvaluateMemoryThreshold(memoryPercent float64) AlertLevel {
	if memoryPercent >= rt.MemoryCriticalPercent {
		return AlertLevelCritical
	}
	if memoryPercent >= rt.MemoryWarningPercent {
		return AlertLevelWarning
	}
	return AlertLevelInfo
}

// EvaluateDiskThreshold checks disk usage against thresholds
func (rt *ResourceThresholds) EvaluateDiskThreshold(diskPercent float64) AlertLevel {
	if diskPercent >= rt.DiskCriticalPercent {
		return AlertLevelCritical
	}
	if diskPercent >= rt.DiskWarningPercent {
		return AlertLevelWarning
	}
	return AlertLevelInfo
}

// EvaluateContainerThreshold checks container count against thresholds
func (rt *ResourceThresholds) EvaluateContainerThreshold(containerCount int) AlertLevel {
	if containerCount >= rt.ContainerCriticalCount {
		return AlertLevelCritical
	}
	if containerCount >= rt.ContainerWarningCount {
		return AlertLevelWarning
	}
	return AlertLevelInfo
}

// EvaluateNetworkLatencyThreshold checks network latency against thresholds
func (rt *ResourceThresholds) EvaluateNetworkLatencyThreshold(latencyMs int) AlertLevel {
	if latencyMs >= rt.NetworkLatencyCriticalMs {
		return AlertLevelCritical
	}
	if latencyMs >= rt.NetworkLatencyWarningMs {
		return AlertLevelWarning
	}
	return AlertLevelInfo
}

// EvaluateDockerResponseTimeThreshold checks Docker response time against thresholds
func (rt *ResourceThresholds) EvaluateDockerResponseTimeThreshold(responseTimeMs int) AlertLevel {
	if responseTimeMs >= rt.DockerResponseTimeCriticalMs {
		return AlertLevelCritical
	}
	if responseTimeMs >= rt.DockerResponseTimeWarningMs {
		return AlertLevelWarning
	}
	return AlertLevelInfo
}

// EvaluateErrorRateThreshold checks error rate against thresholds
func (rt *ResourceThresholds) EvaluateErrorRateThreshold(errorRatePercent float64) AlertLevel {
	if errorRatePercent >= rt.ErrorRateCriticalPercent {
		return AlertLevelCritical
	}
	if errorRatePercent >= rt.ErrorRateWarningPercent {
		return AlertLevelWarning
	}
	return AlertLevelInfo
}

// MonitoringConfig holds configuration for the monitoring system
type MonitoringConfig struct {
	// Monitoring intervals
	CheckInterval        time.Duration       `json:"check_interval"`        // How often to check resources
	AlertCooldownPeriod  time.Duration       `json:"alert_cooldown_period"` // Minimum time between duplicate alerts
	MetricsRetentionTime time.Duration       `json:"metrics_retention_time"` // How long to keep metrics
	
	// Resource thresholds
	Thresholds *ResourceThresholds `json:"thresholds"`
	
	// Feature flags
	EnableCPUMonitoring    bool `json:"enable_cpu_monitoring"`
	EnableMemoryMonitoring bool `json:"enable_memory_monitoring"`
	EnableDiskMonitoring   bool `json:"enable_disk_monitoring"`
	EnableDockerMonitoring bool `json:"enable_docker_monitoring"`
	EnableErrorRateMonitoring bool `json:"enable_error_rate_monitoring"`
	
	// Alert configuration
	EnableAlerting        bool `json:"enable_alerting"`
	MaxAlertsPerInterval  int  `json:"max_alerts_per_interval"` // Rate limiting for alerts
}

// DefaultMonitoringConfig returns a sensible default configuration
func DefaultMonitoringConfig() *MonitoringConfig {
	return &MonitoringConfig{
		CheckInterval:        30 * time.Second,
		AlertCooldownPeriod:  5 * time.Minute,
		MetricsRetentionTime: 24 * time.Hour,
		Thresholds:           DefaultResourceThresholds(),
		EnableCPUMonitoring:    true,
		EnableMemoryMonitoring: true,
		EnableDiskMonitoring:   true,
		EnableDockerMonitoring: true,
		EnableErrorRateMonitoring: true,
		EnableAlerting:        true,
		MaxAlertsPerInterval:  10,
	}
}

// Validate checks if the monitoring configuration is valid
func (mc *MonitoringConfig) Validate() error {
	if mc.CheckInterval <= 0 {
		return fmt.Errorf("check interval must be positive")
	}
	
	if mc.AlertCooldownPeriod < 0 {
		return fmt.Errorf("alert cooldown period cannot be negative")
	}
	
	if mc.MetricsRetentionTime <= 0 {
		return fmt.Errorf("metrics retention time must be positive")
	}
	
	if mc.MaxAlertsPerInterval < 0 {
		return fmt.Errorf("max alerts per interval cannot be negative")
	}
	
	if mc.Thresholds == nil {
		return fmt.Errorf("thresholds configuration is required")
	}
	
	return mc.Thresholds.Validate()
}
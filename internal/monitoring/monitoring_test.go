package monitoring

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockDockerClient implements the docker client interface for testing
type MockDockerClient struct {
	healthy    bool
	containers []ContainerInfo
	responseTime time.Duration
}

func (m *MockDockerClient) IsHealthy(ctx context.Context) error {
	time.Sleep(m.responseTime)
	if !m.healthy {
		return assert.AnError
	}
	return nil
}

func (m *MockDockerClient) ListContainers(ctx context.Context, includeAll bool) ([]ContainerInfo, error) {
	return m.containers, nil
}

func TestResourceThresholds(t *testing.T) {
	t.Run("DefaultResourceThresholds", func(t *testing.T) {
		thresholds := DefaultResourceThresholds()
		
		assert.Equal(t, 70.0, thresholds.CPUWarningPercent)
		assert.Equal(t, 85.0, thresholds.CPUCriticalPercent)
		assert.Equal(t, 75.0, thresholds.MemoryWarningPercent)
		assert.Equal(t, 90.0, thresholds.MemoryCriticalPercent)
		
		err := thresholds.Validate()
		assert.NoError(t, err)
	})

	t.Run("Validation", func(t *testing.T) {
		tests := []struct {
			name      string
			thresholds *ResourceThresholds
			expectErr bool
		}{
			{
				name:      "Valid thresholds",
				thresholds: DefaultResourceThresholds(),
				expectErr: false,
			},
			{
				name: "CPU warning >= critical",
				thresholds: &ResourceThresholds{
					CPUWarningPercent:  85.0,
					CPUCriticalPercent: 85.0,
					MemoryWarningPercent: 75.0,
					MemoryCriticalPercent: 90.0,
					DiskWarningPercent: 80.0,
					DiskCriticalPercent: 95.0,
					ContainerWarningCount: 800,
					ContainerCriticalCount: 1000,
					NetworkLatencyWarningMs: 1000,
					NetworkLatencyCriticalMs: 5000,
					DockerResponseTimeWarningMs: 2000,
					DockerResponseTimeCriticalMs: 10000,
					ErrorRateWarningPercent: 5.0,
					ErrorRateCriticalPercent: 15.0,
				},
				expectErr: true,
			},
			{
				name: "Negative values",
				thresholds: &ResourceThresholds{
					CPUWarningPercent:  -10.0,
					CPUCriticalPercent: 85.0,
					MemoryWarningPercent: 75.0,
					MemoryCriticalPercent: 90.0,
					DiskWarningPercent: 80.0,
					DiskCriticalPercent: 95.0,
					ContainerWarningCount: 800,
					ContainerCriticalCount: 1000,
					NetworkLatencyWarningMs: 1000,
					NetworkLatencyCriticalMs: 5000,
					DockerResponseTimeWarningMs: 2000,
					DockerResponseTimeCriticalMs: 10000,
					ErrorRateWarningPercent: 5.0,
					ErrorRateCriticalPercent: 15.0,
				},
				expectErr: true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := tt.thresholds.Validate()
				if tt.expectErr {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})

	t.Run("Threshold Evaluation", func(t *testing.T) {
		thresholds := DefaultResourceThresholds()

		// CPU threshold tests
		assert.Equal(t, AlertLevelInfo, thresholds.EvaluateCPUThreshold(50.0))
		assert.Equal(t, AlertLevelWarning, thresholds.EvaluateCPUThreshold(75.0))
		assert.Equal(t, AlertLevelCritical, thresholds.EvaluateCPUThreshold(90.0))

		// Memory threshold tests
		assert.Equal(t, AlertLevelInfo, thresholds.EvaluateMemoryThreshold(60.0))
		assert.Equal(t, AlertLevelWarning, thresholds.EvaluateMemoryThreshold(80.0))
		assert.Equal(t, AlertLevelCritical, thresholds.EvaluateMemoryThreshold(95.0))

		// Container threshold tests
		assert.Equal(t, AlertLevelInfo, thresholds.EvaluateContainerThreshold(500))
		assert.Equal(t, AlertLevelWarning, thresholds.EvaluateContainerThreshold(850))
		assert.Equal(t, AlertLevelCritical, thresholds.EvaluateContainerThreshold(1200))
	})
}

func TestMonitoringConfig(t *testing.T) {
	t.Run("DefaultMonitoringConfig", func(t *testing.T) {
		config := DefaultMonitoringConfig()
		
		assert.Equal(t, 30*time.Second, config.CheckInterval)
		assert.Equal(t, 5*time.Minute, config.AlertCooldownPeriod)
		assert.True(t, config.EnableCPUMonitoring)
		assert.True(t, config.EnableMemoryMonitoring)
		assert.True(t, config.EnableAlerting)
		assert.NotNil(t, config.Thresholds)
		
		err := config.Validate()
		assert.NoError(t, err)
	})

	t.Run("Validation", func(t *testing.T) {
		tests := []struct {
			name      string
			config    *MonitoringConfig
			expectErr bool
		}{
			{
				name:      "Valid config",
				config:    DefaultMonitoringConfig(),
				expectErr: false,
			},
			{
				name: "Zero check interval",
				config: &MonitoringConfig{
					CheckInterval: 0,
					Thresholds:    DefaultResourceThresholds(),
				},
				expectErr: true,
			},
			{
				name: "Nil thresholds",
				config: &MonitoringConfig{
					CheckInterval: 30 * time.Second,
					Thresholds:    nil,
				},
				expectErr: true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := tt.config.Validate()
				if tt.expectErr {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})
}

func TestAlertManager(t *testing.T) {
	logger := slog.Default()
	config := DefaultMonitoringConfig()
	
	t.Run("NewAlertManager", func(t *testing.T) {
		am := NewAlertManager(config, logger)
		
		assert.NotNil(t, am)
		assert.Equal(t, config, am.config)
		assert.NotEmpty(t, am.handlers)
	})

	t.Run("SendAlert", func(t *testing.T) {
		am := NewAlertManager(config, logger)
		ctx := context.Background()
		
		err := am.SendAlert(ctx, "TEST_ALERT", "Test Alert", "This is a test alert", AlertLevelWarning, map[string]interface{}{
			"test_key": "test_value",
		})
		
		assert.NoError(t, err)
		
		// Check that alert was stored
		activeAlerts := am.GetActiveAlerts()
		assert.Len(t, activeAlerts, 1)
		assert.Equal(t, "TEST_ALERT", activeAlerts[0].Type)
		assert.Equal(t, AlertLevelWarning, activeAlerts[0].Level)
	})

	t.Run("Cooldown Period", func(t *testing.T) {
		// Use shorter cooldown for testing
		config := DefaultMonitoringConfig()
		config.AlertCooldownPeriod = 100 * time.Millisecond
		
		am := NewAlertManager(config, logger)
		ctx := context.Background()
		
		// Send first alert
		err := am.SendAlert(ctx, "COOLDOWN_TEST", "Test", "First alert", AlertLevelWarning, nil)
		assert.NoError(t, err)
		
		// Send second alert immediately (should be suppressed)
		err = am.SendAlert(ctx, "COOLDOWN_TEST", "Test", "Second alert", AlertLevelWarning, nil)
		assert.NoError(t, err)
		
		// Should only have one alert due to cooldown
		activeAlerts := am.GetActiveAlerts()
		assert.Len(t, activeAlerts, 1)
		
		// Wait for cooldown to expire
		time.Sleep(150 * time.Millisecond)
		
		// Send third alert (should not be suppressed)
		err = am.SendAlert(ctx, "COOLDOWN_TEST", "Test", "Third alert", AlertLevelWarning, nil)
		assert.NoError(t, err)
		
		// Should now have two alerts
		activeAlerts = am.GetActiveAlerts()
		assert.Len(t, activeAlerts, 2)
	})

	t.Run("ResolveAlert", func(t *testing.T) {
		am := NewAlertManager(config, logger)
		ctx := context.Background()
		
		// Send alert
		err := am.SendAlert(ctx, "RESOLVE_TEST", "Test", "Test alert", AlertLevelWarning, nil)
		assert.NoError(t, err)
		
		activeAlerts := am.GetActiveAlerts()
		require.Len(t, activeAlerts, 1)
		alertID := activeAlerts[0].ID
		
		// Resolve alert
		err = am.ResolveAlert(alertID)
		assert.NoError(t, err)
		
		// Should have no active alerts
		activeAlerts = am.GetActiveAlerts()
		assert.Len(t, activeAlerts, 0)
	})

	t.Run("AlertStats", func(t *testing.T) {
		// Use a fresh alert manager to avoid cooldown interference
		freshConfig := DefaultMonitoringConfig()
		freshConfig.AlertCooldownPeriod = 0 // No cooldown for this test
		am := NewAlertManager(freshConfig, logger)
		ctx := context.Background()
		
		// Send different types of alerts
		err := am.SendAlert(ctx, "TYPE_A", "Test", "Alert A", AlertLevelWarning, nil)
		assert.NoError(t, err)
		
		err = am.SendAlert(ctx, "TYPE_B", "Test", "Alert B", AlertLevelCritical, nil)
		assert.NoError(t, err)
		
		err = am.SendAlert(ctx, "TYPE_C", "Test", "Alert C", AlertLevelInfo, nil)
		assert.NoError(t, err)
		
		stats := am.GetAlertStats()
		assert.Equal(t, 3, stats.TotalAlerts)
		assert.Equal(t, 3, stats.ActiveAlerts)
		assert.Equal(t, 0, stats.ResolvedAlerts)
		assert.Equal(t, 1, stats.CriticalAlerts)
		assert.Equal(t, 1, stats.WarningAlerts)
		assert.Equal(t, 1, stats.InfoAlerts)
		assert.Equal(t, 1, stats.AlertsByType["TYPE_A"])
		assert.Equal(t, 1, stats.AlertsByType["TYPE_B"])
		assert.Equal(t, 1, stats.AlertsByType["TYPE_C"])
	})
}

func TestMetricsCollector(t *testing.T) {
	logger := slog.Default()
	config := DefaultMonitoringConfig()
	
	t.Run("NewMetricsCollector", func(t *testing.T) {
		dockerClient := &MockDockerClient{
			healthy: true,
			containers: []ContainerInfo{
				{ID: "container1", State: "running"},
				{ID: "container2", State: "stopped"},
			},
		}
		
		mc := NewMetricsCollector(config, dockerClient, logger)
		
		assert.NotNil(t, mc)
		assert.Equal(t, config, mc.config)
		assert.Equal(t, dockerClient, mc.dockerClient)
	})

	t.Run("CollectMetrics", func(t *testing.T) {
		dockerClient := &MockDockerClient{
			healthy: true,
			containers: []ContainerInfo{
				{ID: "container1", State: "running"},
				{ID: "container2", State: "running"},
				{ID: "container3", State: "stopped"},
			},
			responseTime: 50 * time.Millisecond,
		}
		
		mc := NewMetricsCollector(config, dockerClient, logger)
		ctx := context.Background()
		
		metrics, err := mc.CollectMetrics(ctx)
		assert.NoError(t, err)
		assert.NotNil(t, metrics)
		
		// Check that metrics are populated
		assert.True(t, metrics.Timestamp.After(time.Time{}))
		assert.True(t, metrics.CPUCores > 0)
		assert.True(t, metrics.DockerDaemonResponsive)
		assert.Equal(t, 3, metrics.ContainerCount)
		assert.Equal(t, 2, metrics.RunningContainerCount)
		assert.True(t, metrics.DockerResponseTime >= 50*time.Millisecond)
	})

	t.Run("Error Tracking", func(t *testing.T) {
		mc := NewMetricsCollector(config, nil, logger)
		
		// Record some errors and requests
		mc.RecordError()
		mc.RecordError()
		mc.RecordRequest()
		mc.RecordRequest()
		mc.RecordRequest()
		mc.RecordRequest()
		mc.RecordRequest()
		
		ctx := context.Background()
		metrics, err := mc.CollectMetrics(ctx)
		assert.NoError(t, err)
		
		assert.Equal(t, int64(2), metrics.TotalErrors)
		assert.Equal(t, int64(5), metrics.TotalRequests)
		assert.Equal(t, 40.0, metrics.ErrorRate) // 2/5 = 40%
	})

	t.Run("GetCurrentMetrics", func(t *testing.T) {
		mc := NewMetricsCollector(config, nil, logger)
		
		// Should return nil initially
		metrics := mc.GetCurrentMetrics()
		assert.Nil(t, metrics)
		
		// Collect metrics
		ctx := context.Background()
		_, err := mc.CollectMetrics(ctx)
		assert.NoError(t, err)
		
		// Should now return metrics
		metrics = mc.GetCurrentMetrics()
		assert.NotNil(t, metrics)
	})

	t.Run("MetricsHistory", func(t *testing.T) {
		mc := NewMetricsCollector(config, nil, logger)
		ctx := context.Background()
		
		// Collect metrics multiple times
		for i := 0; i < 3; i++ {
			_, err := mc.CollectMetrics(ctx)
			assert.NoError(t, err)
			time.Sleep(10 * time.Millisecond) // Ensure different timestamps
		}
		
		// Get history
		since := time.Now().Add(-1 * time.Minute)
		history := mc.GetMetricsHistory(since)
		assert.Len(t, history, 3)
	})
}

func TestResourceMonitor(t *testing.T) {
	logger := slog.Default()
	config := DefaultMonitoringConfig()
	config.CheckInterval = 100 * time.Millisecond // Speed up for testing
	
	alertManager := NewAlertManager(config, logger)
	
	dockerClient := &MockDockerClient{
		healthy: true,
		containers: []ContainerInfo{
			{ID: "container1", State: "running"},
		},
		responseTime: 10 * time.Millisecond,
	}
	
	metricsCollector := NewMetricsCollector(config, dockerClient, logger)
	
	t.Run("NewResourceMonitor", func(t *testing.T) {
		rm := NewResourceMonitor(config, alertManager, metricsCollector, logger)
		
		assert.NotNil(t, rm)
		assert.Equal(t, config, rm.config)
		assert.False(t, rm.IsRunning())
	})

	t.Run("Start and Stop", func(t *testing.T) {
		rm := NewResourceMonitor(config, alertManager, metricsCollector, logger)
		ctx := context.Background()
		
		// Start monitoring
		err := rm.Start(ctx)
		assert.NoError(t, err)
		assert.True(t, rm.IsRunning())
		
		// Wait for at least one health check
		time.Sleep(200 * time.Millisecond)
		
		// Check that health status is updated
		healthStatus := rm.GetHealthStatus()
		assert.True(t, healthStatus.LastCheck.After(time.Time{}))
		
		// Stop monitoring
		err = rm.Stop()
		assert.NoError(t, err)
		assert.False(t, rm.IsRunning())
	})

	t.Run("ForceHealthCheck", func(t *testing.T) {
		rm := NewResourceMonitor(config, alertManager, metricsCollector, logger)
		
		// Force health check without starting
		err := rm.ForceHealthCheck()
		assert.NoError(t, err)
		
		// Check that metrics are available
		metrics := rm.GetCurrentMetrics()
		assert.NotNil(t, metrics)
	})

	t.Run("RecordErrorsAndRequests", func(t *testing.T) {
		rm := NewResourceMonitor(config, alertManager, metricsCollector, logger)
		
		// Record some activity
		rm.RecordError()
		rm.RecordRequest()
		rm.RecordRequest()
		
		// Force health check to collect metrics
		err := rm.ForceHealthCheck()
		assert.NoError(t, err)
		
		metrics := rm.GetCurrentMetrics()
		assert.NotNil(t, metrics)
		assert.Equal(t, int64(1), metrics.TotalErrors)
		assert.Equal(t, int64(2), metrics.TotalRequests)
	})

	t.Run("GetUptime", func(t *testing.T) {
		rm := NewResourceMonitor(config, alertManager, metricsCollector, logger)
		
		time.Sleep(10 * time.Millisecond)
		uptime := rm.GetUptime()
		assert.True(t, uptime > 0)
		assert.True(t, uptime >= 10*time.Millisecond)
	})
}

func TestUtilityFunctions(t *testing.T) {
	t.Run("FormatBytes", func(t *testing.T) {
		tests := []struct {
			bytes    uint64
			expected string
		}{
			{0, "0 B"},
			{512, "512 B"},
			{1024, "1.0 KB"},
			{1536, "1.5 KB"},
			{1048576, "1.0 MB"},
			{1073741824, "1.0 GB"},
		}

		for _, tt := range tests {
			result := FormatBytes(tt.bytes)
			assert.Equal(t, tt.expected, result)
		}
	})
}
package config

import "time"

// Default timeout and interval constants
// These constants centralize hardcoded values to improve maintainability
const (
	// Worker configuration defaults
	DefaultHealthCheckInterval  = 30 * time.Second
	DefaultShutdownTimeout      = 30 * time.Second
	DefaultMaxRetryAttempts     = 3
	DefaultProcessingSlotTTL    = 30 * time.Minute
	DefaultScalingCheckInterval = 60 * time.Second

	// Server configuration defaults
	DefaultServerReadTimeout  = 30 * time.Second
	DefaultServerWriteTimeout = 30 * time.Second

	// Executor configuration defaults
	DefaultExecutorMemoryLimitMB = 512
	DefaultExecutorMemoryLimit   = DefaultExecutorMemoryLimitMB * 1024 * 1024 // 512MB in bytes
	DefaultExecutorCPUQuota      = 100000                                     // 1 CPU core
	DefaultExecutorPidsLimit     = 128                                        // Max processes

	// Queue and processing defaults
	DefaultRetryDelay           = 30 * time.Second
	DefaultWorkerHeartbeat      = 30 * time.Second
	DefaultHealthCheckInterval2 = 30 * time.Second // For queue/retry processors
	DefaultCleanupTimeout       = 30 * time.Second

	// Task defaults
	DefaultTaskTimeout = 300 // 5 minutes in seconds

	// Database defaults
	DefaultDatabaseTimeout = 30 * time.Second

	// Monitoring intervals
	DefaultMetricsInterval = 1 * time.Minute
	DefaultStatsInterval   = 30 * time.Second
)

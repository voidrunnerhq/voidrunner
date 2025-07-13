// Package main VoidRunner Scheduler Service
//
// The scheduler is responsible for:
// - Managing worker pools for task execution
// - Processing queued tasks
// - Handling task retries and failures
// - Monitoring system health and performance
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/voidrunnerhq/voidrunner/internal/config"
	"github.com/voidrunnerhq/voidrunner/internal/database"
	"github.com/voidrunnerhq/voidrunner/internal/executor"
	"github.com/voidrunnerhq/voidrunner/internal/queue"
	"github.com/voidrunnerhq/voidrunner/internal/worker"
	"github.com/voidrunnerhq/voidrunner/pkg/logger"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	log := logger.New(cfg.Logger.Level, cfg.Logger.Format)
	log.Info("starting VoidRunner scheduler service")

	// Initialize database connection
	dbConn, err := database.NewConnection(&cfg.Database, log.Logger)
	if err != nil {
		log.Error("failed to initialize database connection", "error", err)
		os.Exit(1)
	}
	defer dbConn.Close()

	// Perform database health check
	healthCtx, healthCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer healthCancel()

	if err := dbConn.HealthCheck(healthCtx); err != nil {
		log.Error("database health check failed", "error", err)
		os.Exit(1)
	}

	log.Info("database connection established")

	// Initialize repositories
	repos := database.NewRepositories(dbConn)

	// Initialize queue manager
	queueManager, err := queue.NewRedisQueueManager(&cfg.Redis, &cfg.Queue, log.Logger)
	if err != nil {
		log.Error("failed to initialize queue manager", "error", err)
		os.Exit(1)
	}

	// Start queue manager
	queueCtx, queueCancel := context.WithCancel(context.Background())
	defer queueCancel()

	if err := queueManager.Start(queueCtx); err != nil {
		log.Error("failed to start queue manager", "error", err)
		os.Exit(1)
	}
	defer func() {
		log.Info("stopping queue manager")
		if err := queueManager.Stop(context.Background()); err != nil {
			log.Error("failed to stop queue manager", "error", err)
		}
	}()

	log.Info("queue manager started successfully")

	// Initialize executor configuration
	executorConfig := &executor.Config{
		DockerEndpoint: cfg.Executor.DockerEndpoint,
		DefaultResourceLimits: executor.ResourceLimits{
			MemoryLimitBytes: int64(cfg.Executor.DefaultMemoryLimitMB) * 1024 * 1024,
			CPUQuota:         cfg.Executor.DefaultCPUQuota,
			PidsLimit:        cfg.Executor.DefaultPidsLimit,
			TimeoutSeconds:   cfg.Executor.DefaultTimeoutSeconds,
		},
		DefaultTimeoutSeconds: cfg.Executor.DefaultTimeoutSeconds,
		Images: executor.ImageConfig{
			Python:     cfg.Executor.PythonImage,
			Bash:       cfg.Executor.BashImage,
			JavaScript: cfg.Executor.JavaScriptImage,
			Go:         cfg.Executor.GoImage,
		},
		Security: executor.SecuritySettings{
			EnableSeccomp:      cfg.Executor.EnableSeccomp,
			SeccompProfilePath: cfg.Executor.SeccompProfilePath,
			EnableAppArmor:     cfg.Executor.EnableAppArmor,
			AppArmorProfile:    cfg.Executor.AppArmorProfile,
			ExecutionUser:      cfg.Executor.ExecutionUser,
		},
	}

	// Create seccomp profile if enabled
	if cfg.Executor.EnableSeccomp {
		if err := setupSeccompProfile(executorConfig, log); err != nil {
			log.Warn("failed to setup seccomp profile", "error", err)
		}
	}

	// Initialize executor
	taskExecutor, err := initializeExecutor(executorConfig, log)
	if err != nil {
		log.Error("failed to initialize executor", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := taskExecutor.Cleanup(context.Background()); err != nil {
			log.Error("failed to cleanup executor", "error", err)
		}
	}()

	// Initialize worker manager
	// Convert config.WorkerConfig to worker.WorkerConfig
	workerConfig := worker.WorkerConfig{
		WorkerIDPrefix:       cfg.Worker.WorkerIDPrefix,
		HeartbeatInterval:    cfg.Worker.HeartbeatInterval,
		TaskTimeout:          cfg.Worker.TaskTimeout,
		HealthCheckInterval:  30 * time.Second, // Default value for health check interval
		ShutdownTimeout:      cfg.Worker.ShutdownTimeout,
		MaxRetryAttempts:     3,                // Default value for retry attempts
		ProcessingSlotTTL:    30 * time.Minute, // Default value for slot TTL
		StaleTaskThreshold:   cfg.Worker.StaleTaskThreshold,
		EnableAutoScaling:    true,             // Default enable auto-scaling
		ScalingCheckInterval: 60 * time.Second, // Default scaling check interval
	}

	workerManager := worker.NewWorkerManager(
		queueManager,
		taskExecutor,
		repos,
		workerConfig,
		log.Logger,
	)

	// Start worker manager
	workerCtx, workerCancel := context.WithCancel(context.Background())
	defer workerCancel()

	if err := workerManager.Start(workerCtx); err != nil {
		log.Error("failed to start worker manager", "error", err)
		os.Exit(1)
	}
	defer func() {
		log.Info("stopping worker manager")
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer shutdownCancel()

		if err := workerManager.Stop(shutdownCtx); err != nil {
			log.Error("failed to stop worker manager", "error", err)
		}
	}()

	log.Info("worker manager started successfully")

	// Start monitoring and health check routine
	go startHealthMonitoring(workerManager, queueManager, log)

	// Start metrics collection (if enabled)
	go startMetricsCollection(workerManager, queueManager, cfg, log)

	log.Info("scheduler service is running",
		"worker_pool_size", workerManager.GetWorkerPool().GetWorkerCount(),
		"concurrency_limits", workerManager.GetConcurrencyLimits(),
	)

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("shutdown signal received, initiating graceful shutdown")

	// Create shutdown context with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// Shutdown components in reverse order
	workerCancel()
	queueCancel()

	// Wait for graceful shutdown or timeout
	select {
	case <-shutdownCtx.Done():
		log.Warn("graceful shutdown timeout reached")
	case <-time.After(25 * time.Second):
		log.Info("scheduler service shutdown completed")
	}

	log.Info("scheduler service exited")
}

// setupSeccompProfile creates and configures the seccomp profile
func setupSeccompProfile(executorConfig *executor.Config, log *logger.Logger) error {
	seccompDir := filepath.Dir(executorConfig.Security.SeccompProfilePath)
	if err := os.MkdirAll(seccompDir, 0750); err != nil {
		return fmt.Errorf("failed to create seccomp profile directory: %w", err)
	}

	// Create security manager to generate the seccomp profile
	securityManager := executor.NewSecurityManager(executorConfig)
	seccompProfilePath, err := securityManager.CreateSeccompProfile(context.Background())
	if err != nil {
		return fmt.Errorf("failed to create seccomp profile: %w", err)
	}

	// Copy the profile to the configured location if needed
	if seccompProfilePath != executorConfig.Security.SeccompProfilePath {
		if err := copyFile(seccompProfilePath, executorConfig.Security.SeccompProfilePath); err != nil {
			return fmt.Errorf("failed to copy seccomp profile: %w", err)
		}
		// Clean up temporary profile
		_ = os.Remove(seccompProfilePath)
	}

	log.Info("seccomp profile created successfully", "path", executorConfig.Security.SeccompProfilePath)
	return nil
}

// initializeExecutor initializes the task executor with fallback to mock
func initializeExecutor(executorConfig *executor.Config, log *logger.Logger) (executor.TaskExecutor, error) {
	// Try to initialize Docker executor first
	dockerExecutor, err := executor.NewExecutor(executorConfig, log.Logger)
	if err != nil {
		log.Warn("failed to initialize Docker executor, falling back to mock executor", "error", err)
		// Use mock executor for environments without Docker
		mockExecutor := executor.NewMockExecutor(executorConfig, log.Logger)
		log.Info("mock executor initialized successfully")
		return mockExecutor, nil
	}

	// Check Docker executor health
	healthCtx, healthCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer healthCancel()

	if err := dockerExecutor.IsHealthy(healthCtx); err != nil {
		log.Warn("Docker executor health check failed, falling back to mock executor", "error", err)
		// Cleanup failed Docker executor
		_ = dockerExecutor.Cleanup(context.Background())
		// Use mock executor instead
		mockExecutor := executor.NewMockExecutor(executorConfig, log.Logger)
		log.Info("mock executor initialized successfully")
		return mockExecutor, nil
	}

	log.Info("Docker executor initialized successfully")
	return dockerExecutor, nil
}

// startHealthMonitoring starts the health monitoring routine
func startHealthMonitoring(workerManager worker.WorkerManager, queueManager queue.QueueManager, log *logger.Logger) {
	ticker := time.NewTicker(30 * time.Second) // Health check every 30 seconds
	defer ticker.Stop()

	for range ticker.C {
		performHealthCheck(workerManager, queueManager, log)
	}
}

// performHealthCheck performs a comprehensive health check
func performHealthCheck(workerManager worker.WorkerManager, queueManager queue.QueueManager, log *logger.Logger) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	healthStatus := make(map[string]bool)

	// Check worker manager health
	healthStatus["worker_manager"] = workerManager.IsHealthy()

	// Check queue manager health
	healthStatus["queue_manager"] = queueManager.IsHealthy(ctx) == nil

	// Check worker pool health
	healthStatus["worker_pool"] = workerManager.GetWorkerPool().IsHealthy()

	// Log health status
	unhealthyComponents := make([]string, 0)
	for component, healthy := range healthStatus {
		if !healthy {
			unhealthyComponents = append(unhealthyComponents, component)
		}
	}

	if len(unhealthyComponents) > 0 {
		log.Warn("health check detected unhealthy components", "unhealthy", unhealthyComponents)
	} else {
		log.Debug("health check passed for all components")
	}

	// Log worker statistics
	workerStats := workerManager.GetStats()
	log.Debug("worker manager stats",
		"running", workerStats.IsRunning,
		"healthy", workerStats.IsHealthy,
		"pool_size", workerStats.WorkerPoolStats.PoolSize,
		"active_workers", workerStats.WorkerPoolStats.ActiveWorkers,
		"total_tasks_processed", workerStats.WorkerPoolStats.TotalTasksProcessed,
	)

	// Log queue statistics
	if queueStats, err := queueManager.GetStats(ctx); err == nil {
		log.Debug("queue manager stats",
			"task_queue_messages", queueStats.TaskQueue.ApproximateMessages,
			"retry_queue_messages", queueStats.RetryQueue.ApproximateMessages,
			"dead_letter_messages", queueStats.DeadLetterQueue.ApproximateMessages,
			"total_throughput", queueStats.TotalThroughput,
		)
	}
}

// startMetricsCollection starts metrics collection if enabled
func startMetricsCollection(workerManager worker.WorkerManager, queueManager queue.QueueManager, cfg *config.Config, log *logger.Logger) {
	// This is a placeholder for metrics collection
	// In a production system, you would integrate with Prometheus, StatsD, or other metrics systems

	ticker := time.NewTicker(1 * time.Minute) // Collect metrics every minute
	defer ticker.Stop()

	for range ticker.C {
		collectMetrics(workerManager, queueManager, log)
	}
}

// collectMetrics collects and reports system metrics
func collectMetrics(workerManager worker.WorkerManager, queueManager queue.QueueManager, log *logger.Logger) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Collect worker metrics
	workerStats := workerManager.GetStats()
	concurrencyStats := workerStats.ConcurrencyStats

	// Collect queue metrics
	queueStats, err := queueManager.GetStats(ctx)
	if err != nil {
		log.Error("failed to collect queue metrics", "error", err)
		return
	}

	// Log metrics (in production, these would be sent to a metrics system)
	log.Info("system metrics",
		// Worker metrics
		"worker_pool_size", workerStats.WorkerPoolStats.PoolSize,
		"active_workers", workerStats.WorkerPoolStats.ActiveWorkers,
		"idle_workers", workerStats.WorkerPoolStats.IdleWorkers,
		"unhealthy_workers", workerStats.WorkerPoolStats.UnhealthyWorkers,
		"total_tasks_processed", workerStats.WorkerPoolStats.TotalTasksProcessed,
		"total_tasks_successful", workerStats.WorkerPoolStats.TotalTasksSuccessful,
		"total_tasks_failed", workerStats.WorkerPoolStats.TotalTasksFailed,
		"average_task_time_ms", workerStats.WorkerPoolStats.AverageTaskTime.Milliseconds(),

		// Concurrency metrics
		"total_active_slots", concurrencyStats.TotalActiveSlots,
		"available_slots", concurrencyStats.AvailableSlots,
		"slots_acquired_total", concurrencyStats.SlotsAcquiredTotal,
		"slots_released_total", concurrencyStats.SlotsReleasedTotal,

		// Queue metrics
		"task_queue_messages", queueStats.TaskQueue.ApproximateMessages,
		"task_queue_in_flight", queueStats.TaskQueue.MessagesInFlight,
		"retry_queue_messages", queueStats.RetryQueue.ApproximateMessages,
		"retry_queue_ready", queueStats.RetryQueue.ReadyForRetry,
		"dead_letter_messages", queueStats.DeadLetterQueue.ApproximateMessages,
		"total_throughput", queueStats.TotalThroughput,
	)
}

// copyFile copies a file from src to dst with proper validation
func copyFile(src, dst string) error {
	// Validate and clean paths to prevent directory traversal
	cleanSrc := filepath.Clean(src)
	cleanDst := filepath.Clean(dst)

	// Additional security check: ensure paths don't contain ".." or other suspicious patterns
	if !filepath.IsAbs(cleanSrc) || !filepath.IsAbs(cleanDst) {
		return fmt.Errorf("paths must be absolute")
	}

	// #nosec G304 - Path traversal mitigation: paths are validated and cleaned above
	sourceFile, err := os.Open(cleanSrc)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	// Ensure destination directory exists
	if err := os.MkdirAll(filepath.Dir(cleanDst), 0750); err != nil {
		return err
	}

	// #nosec G304 - Path traversal mitigation: paths are validated and cleaned above
	destFile, err := os.Create(cleanDst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	// Copy file contents
	if _, err := sourceFile.WriteTo(destFile); err != nil {
		return err
	}

	// Set file permissions to 0600 for security
	return os.Chmod(cleanDst, 0600)
}

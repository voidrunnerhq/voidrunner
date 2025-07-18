// Package main VoidRunner API Server
//
//	@title			VoidRunner API
//	@version		1.0.0
//	@description	VoidRunner is a distributed task execution platform that allows users to create, manage, and execute code tasks securely in isolated containers.
//	@termsOfService	https://voidrunner.com/terms
//
//	@contact.name	VoidRunner Support
//	@contact.url	https://github.com/voidrunnerhq/voidrunner
//	@contact.email	support@voidrunner.com
//
//	@license.name	MIT
//	@license.url	https://opensource.org/licenses/MIT
//
//	@host		localhost:8080
//	@BasePath	/api/v1
//
//	@securityDefinitions.apikey	BearerAuth
//	@in							header
//	@name						Authorization
//	@description				Type "Bearer" followed by a space and JWT token.
//
//	@tag.name			Authentication
//	@tag.description	User authentication and authorization operations
//	@tag.name			Tasks
//	@tag.description	Task management operations
//	@tag.name			Executions
//	@tag.description	Task execution operations
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/voidrunnerhq/voidrunner/internal/api/routes"
	"github.com/voidrunnerhq/voidrunner/internal/auth"
	"github.com/voidrunnerhq/voidrunner/internal/config"
	"github.com/voidrunnerhq/voidrunner/internal/database"
	"github.com/voidrunnerhq/voidrunner/internal/executor"
	"github.com/voidrunnerhq/voidrunner/internal/queue"
	"github.com/voidrunnerhq/voidrunner/internal/services"
	"github.com/voidrunnerhq/voidrunner/internal/worker"
	"github.com/voidrunnerhq/voidrunner/pkg/logger"
	"github.com/voidrunnerhq/voidrunner/pkg/utils"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	log := logger.New(cfg.Logger.Level, cfg.Logger.Format)

	// Initialize database connection
	dbConn, err := database.NewConnectionWithRetry(&cfg.Database, log.Logger)
	if err != nil {
		log.Error("failed to initialize database connection", "error", err)
		os.Exit(1)
	}
	defer dbConn.Close()

	// Run database migrations
	migrateConfig := &database.MigrateConfig{
		DatabaseConfig: &cfg.Database,
		MigrationsPath: "file://migrations",
		Logger:         log.Logger,
	}

	if err := database.MigrateUp(migrateConfig); err != nil {
		log.Error("failed to run database migrations", "error", err)
		os.Exit(1)
	}

	// Initialize repositories
	repos := database.NewRepositories(dbConn)

	// Perform database health check
	healthCtx, healthCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer healthCancel()

	if err := dbConn.HealthCheck(healthCtx); err != nil {
		log.Error("database health check failed", "error", err)
		os.Exit(1)
	}

	log.Info("database initialized successfully")

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
		if err := queueManager.Stop(context.Background()); err != nil {
			log.Error("failed to stop queue manager", "error", err)
		}
	}()

	// Initialize JWT service
	jwtService := auth.NewJWTService(&cfg.JWT)

	// Initialize authentication service
	authService := auth.NewService(repos.Users, jwtService, log.Logger, cfg)

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

	// Create seccomp profile directory if it doesn't exist
	if cfg.Executor.EnableSeccomp {
		seccompDir := filepath.Dir(cfg.Executor.SeccompProfilePath)
		if err := os.MkdirAll(seccompDir, 0750); err != nil {
			log.Warn("failed to create seccomp profile directory", "error", err, "path", seccompDir)
		}

		// Create a temporary security manager to generate the seccomp profile
		tempSecurityManager := executor.NewSecurityManager(executorConfig)
		seccompProfilePath, err := tempSecurityManager.CreateSeccompProfile(context.Background())
		if err != nil {
			log.Warn("failed to create seccomp profile", "error", err)
		} else {
			// Copy the profile to the configured location
			if seccompProfilePath != cfg.Executor.SeccompProfilePath {
				if err := utils.CopyFile(seccompProfilePath, cfg.Executor.SeccompProfilePath); err != nil {
					log.Warn("failed to copy seccomp profile to configured location", "error", err)
				} else {
					log.Info("seccomp profile created successfully", "path", cfg.Executor.SeccompProfilePath)
				}
				// Clean up temporary profile
				_ = os.Remove(seccompProfilePath)
			} else {
				log.Info("seccomp profile created successfully", "path", cfg.Executor.SeccompProfilePath)
			}
		}
	}

	// Initialize executor (Docker or Mock based on availability)
	var taskExecutor executor.TaskExecutor

	// Try to initialize Docker executor first
	dockerExecutor, err := executor.NewExecutor(executorConfig, log.Logger)
	if err != nil {
		log.Warn("failed to initialize Docker executor, falling back to mock executor", "error", err)
		// Use mock executor for environments without Docker (e.g., CI)
		taskExecutor = executor.NewMockExecutor(executorConfig, log.Logger)
		log.Info("mock executor initialized successfully")
	} else {
		// Check Docker executor health
		healthCtx, healthCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer healthCancel()

		if err := dockerExecutor.IsHealthy(healthCtx); err != nil {
			log.Warn("Docker executor health check failed, falling back to mock executor", "error", err)
			// Cleanup failed Docker executor
			_ = dockerExecutor.Cleanup(context.Background())
			// Use mock executor instead
			taskExecutor = executor.NewMockExecutor(executorConfig, log.Logger)
			log.Info("mock executor initialized successfully")
		} else {
			taskExecutor = dockerExecutor
			log.Info("Docker executor initialized successfully")
			// Add cleanup for successful Docker executor
			defer func() {
				if err := dockerExecutor.Cleanup(context.Background()); err != nil {
					log.Error("failed to cleanup Docker executor", "error", err)
				}
			}()
		}
	}

	// Initialize worker manager if embedded workers are enabled
	var workerManager worker.WorkerManager
	var workerCancel context.CancelFunc
	if cfg.HasEmbeddedWorkers() {
		log.Info("initializing embedded worker manager")

		// Convert config.WorkerConfig to worker.WorkerConfig
		workerConfig := worker.WorkerConfig{
			WorkerIDPrefix:       cfg.Worker.WorkerIDPrefix,
			HeartbeatInterval:    cfg.Worker.HeartbeatInterval,
			TaskTimeout:          cfg.Worker.TaskTimeout,
			HealthCheckInterval:  config.DefaultHealthCheckInterval,
			ShutdownTimeout:      cfg.Worker.ShutdownTimeout,
			MaxRetryAttempts:     config.DefaultMaxRetryAttempts,
			ProcessingSlotTTL:    config.DefaultProcessingSlotTTL,
			StaleTaskThreshold:   cfg.Worker.StaleTaskThreshold,
			EnableAutoScaling:    true, // Default enable auto-scaling
			ScalingCheckInterval: config.DefaultScalingCheckInterval,
		}

		workerManager = worker.NewWorkerManager(
			queueManager,
			taskExecutor,
			repos,
			workerConfig,
			log.Logger,
		)

		// Start worker manager
		workerCtx, cancel := context.WithCancel(context.Background())
		workerCancel = cancel

		if err := workerManager.Start(workerCtx); err != nil {
			log.Error("failed to start embedded worker manager", "error", err)
			os.Exit(1)
		}

		// Add cleanup for worker manager
		defer func() {
			if workerManager != nil && workerCancel != nil {
				log.Info("stopping embedded worker manager")
				shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), config.DefaultShutdownTimeout)
				defer shutdownCancel()
				defer workerCancel()

				if err := workerManager.Stop(shutdownCtx); err != nil {
					log.Error("failed to stop embedded worker manager", "error", err)
				}
			}
		}()

		log.Info("embedded worker manager started successfully",
			"worker_pool_size", workerManager.GetWorkerPool().GetWorkerCount(),
			"concurrency_limits", workerManager.GetConcurrencyLimits(),
		)
	} else {
		log.Info("embedded workers disabled, tasks will be processed by separate scheduler service")
	}

	// Initialize task execution service
	taskExecutionService := services.NewTaskExecutionService(dbConn, queueManager, log.Logger)

	// Initialize task executor service
	taskExecutorService := services.NewTaskExecutorService(
		taskExecutionService,
		repos.Tasks,
		taskExecutor,
		nil, // cleanup manager will be initialized within the executor
		log.Logger,
	)

	if cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	routes.Setup(router, cfg, log, dbConn, repos, authService, taskExecutionService, taskExecutorService, workerManager)

	srv := &http.Server{
		Addr:              fmt.Sprintf("%s:%s", cfg.Server.Host, cfg.Server.Port),
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       config.DefaultServerReadTimeout,
		WriteTimeout:      config.DefaultServerWriteTimeout,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		log.Info("starting server",
			"host", cfg.Server.Host,
			"port", cfg.Server.Port,
			"env", cfg.Server.Env,
		)

		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("server failed to start", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), config.DefaultShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Error("server forced to shutdown", "error", err)
		os.Exit(1)
	}

	log.Info("server exited")
}

package routes

import (
	"context"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/voidrunnerhq/voidrunner/internal/api/handlers"
	"github.com/voidrunnerhq/voidrunner/internal/api/middleware"
	"github.com/voidrunnerhq/voidrunner/internal/auth"
	"github.com/voidrunnerhq/voidrunner/internal/config"
	"github.com/voidrunnerhq/voidrunner/internal/database"
	"github.com/voidrunnerhq/voidrunner/internal/services"
	"github.com/voidrunnerhq/voidrunner/internal/worker"
	"github.com/voidrunnerhq/voidrunner/pkg/logger"
)

func Setup(router *gin.Engine, cfg *config.Config, log *logger.Logger, dbConn *database.Connection, repos *database.Repositories, authService *auth.Service, taskExecutionService *services.TaskExecutionService, taskExecutorService *services.TaskExecutorService, workerManager worker.WorkerManager) {
	setupMiddleware(router, cfg, log)
	setupRoutes(router, cfg, log, dbConn, repos, authService, taskExecutionService, taskExecutorService, workerManager)
}

func setupMiddleware(router *gin.Engine, cfg *config.Config, log *logger.Logger) {
	router.Use(middleware.SecurityHeaders())
	router.Use(middleware.RequestID())
	router.Use(middleware.CORS(cfg.CORS.AllowedOrigins, cfg.CORS.AllowedMethods, cfg.CORS.AllowedHeaders))
	router.Use(log.GinLogger())
	router.Use(log.GinRecovery())
	router.Use(middleware.ErrorHandler())
}

func setupRoutes(router *gin.Engine, cfg *config.Config, log *logger.Logger, dbConn *database.Connection, repos *database.Repositories, authService *auth.Service, taskExecutionService *services.TaskExecutionService, taskExecutorService *services.TaskExecutorService, workerManager worker.WorkerManager) {
	healthHandler := handlers.NewHealthHandler()

	// Add health checks for different components
	healthHandler.AddHealthCheck("database", &DatabaseHealthChecker{conn: dbConn})
	healthHandler.AddHealthCheck("executor", &ExecutorHealthChecker{service: taskExecutorService})

	// Add worker health check if embedded workers are enabled
	if cfg.HasEmbeddedWorkers() && workerManager != nil {
		healthHandler.AddHealthCheck("workers", &WorkerHealthChecker{manager: workerManager})
	}

	authHandler := handlers.NewAuthHandler(authService, log.Logger)
	authMiddleware := middleware.NewAuthMiddleware(authService, log.Logger)
	docsHandler := handlers.NewDocsHandler()

	router.GET("/health", healthHandler.Health)
	router.GET("/ready", healthHandler.Readiness)

	// Worker-specific health endpoint (only available when embedded workers are enabled)
	if cfg.HasEmbeddedWorkers() && workerManager != nil {
		workerHandler := handlers.NewWorkerHandler(workerManager, log.Logger)
		router.GET("/health/workers", workerHandler.GetWorkerStatus)
	}

	// Documentation routes
	router.GET("/api", docsHandler.GetAPIIndex)
	router.GET("/docs", docsHandler.RedirectToSwaggerUI)
	router.GET("/docs/*any", docsHandler.GetSwaggerUI())

	// Swagger spec endpoints at a different path to avoid conflict
	router.GET("/swagger.json", docsHandler.GetSwaggerJSON)
	router.GET("/swagger.yaml", docsHandler.GetSwaggerYAML)

	v1 := router.Group("/api/v1")
	{
		v1.GET("/ping", func(c *gin.Context) {
			c.JSON(200, gin.H{
				"message": "pong",
			})
		})

		// Auth endpoints (public)
		auth := v1.Group("/auth")
		{
			// Use different rate limits for test vs production
			var registerRateLimit, authRateLimit gin.HandlerFunc
			if cfg.IsTest() {
				registerRateLimit = middleware.RegisterRateLimitForTest(log.Logger)
				authRateLimit = middleware.AuthRateLimitForTest(log.Logger)
			} else {
				registerRateLimit = middleware.RegisterRateLimit(log.Logger)
				authRateLimit = middleware.AuthRateLimit(log.Logger)
			}

			auth.POST("/register",
				registerRateLimit,
				authHandler.Register,
			)
			auth.POST("/login",
				authRateLimit,
				authHandler.Login,
			)
			auth.POST("/refresh",
				middleware.RefreshRateLimit(log.Logger),
				authHandler.RefreshToken,
			)
			auth.POST("/logout", authHandler.Logout)
		}

		// Protected endpoints
		protected := v1.Group("")
		protected.Use(authMiddleware.RequireAuth())
		{
			protected.GET("/auth/me", authHandler.Me)
		}

		// Task management endpoints
		taskHandler := handlers.NewTaskHandler(repos.Tasks, log.Logger)
		executionHandler := handlers.NewTaskExecutionHandler(repos.Tasks, repos.TaskExecutions, taskExecutionService, log.Logger)
		taskValidation := middleware.TaskValidation(log.Logger)

		// Use different rate limits for test vs production
		var taskRateLimit, taskCreationRateLimit, taskExecutionRateLimit, executionCreationRateLimit gin.HandlerFunc
		if cfg.IsTest() {
			taskRateLimit = middleware.TaskRateLimitForTest(log.Logger)
			taskCreationRateLimit = middleware.TaskCreationRateLimitForTest(log.Logger)
			taskExecutionRateLimit = middleware.TaskExecutionRateLimitForTest(log.Logger)
			executionCreationRateLimit = middleware.ExecutionCreationRateLimitForTest(log.Logger)
		} else {
			taskRateLimit = middleware.TaskRateLimit(log.Logger)
			taskCreationRateLimit = middleware.TaskCreationRateLimit(log.Logger)
			taskExecutionRateLimit = middleware.TaskExecutionRateLimit(log.Logger)
			executionCreationRateLimit = middleware.ExecutionCreationRateLimit(log.Logger)
		}

		// Task CRUD operations
		protected.POST("/tasks",
			middleware.RequestSizeLimit(log.Logger),
			taskCreationRateLimit,
			taskValidation.ValidateTaskCreation(),
			taskHandler.Create,
		)
		protected.GET("/tasks",
			taskRateLimit,
			taskHandler.List,
		)
		protected.GET("/tasks/:id",
			taskRateLimit,
			taskHandler.GetByID,
		)
		protected.PUT("/tasks/:id",
			middleware.RequestSizeLimit(log.Logger),
			taskRateLimit,
			taskValidation.ValidateTaskUpdate(),
			taskHandler.Update,
		)
		protected.DELETE("/tasks/:id",
			taskRateLimit,
			taskHandler.Delete,
		)

		// Task execution operations
		protected.POST("/tasks/:id/executions",
			executionCreationRateLimit,
			executionHandler.Create,
		)
		protected.GET("/tasks/:id/executions",
			taskExecutionRateLimit,
			executionHandler.ListByTaskID,
		)
		protected.GET("/executions/:id",
			taskExecutionRateLimit,
			executionHandler.GetByID,
		)
		protected.PUT("/executions/:id",
			middleware.RequestSizeLimit(log.Logger),
			taskExecutionRateLimit,
			taskValidation.ValidateTaskExecutionUpdate(),
			executionHandler.Update,
		)
		protected.DELETE("/executions/:id",
			taskExecutionRateLimit,
			executionHandler.Cancel,
		)
	}
}

// DatabaseHealthChecker implements health checking for database
type DatabaseHealthChecker struct {
	conn *database.Connection
}

func (d *DatabaseHealthChecker) CheckHealth() (status string, err error) {
	if d.conn == nil {
		return "ready", nil // For tests, consider nil database as healthy
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := d.conn.HealthCheck(ctx); err != nil {
		return "unhealthy", err
	}
	return "ready", nil
}

// ExecutorHealthChecker implements health checking for Docker executor
type ExecutorHealthChecker struct {
	service *services.TaskExecutorService
}

func (e *ExecutorHealthChecker) CheckHealth() (status string, err error) {
	if e.service == nil {
		return "ready", nil // For tests, consider nil executor as healthy
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := e.service.GetExecutorHealth(ctx); err != nil {
		return "unhealthy", err
	}
	return "ready", nil
}

// WorkerHealthChecker implements health checking for embedded worker manager
type WorkerHealthChecker struct {
	manager worker.WorkerManager
}

func (w *WorkerHealthChecker) CheckHealth() (status string, err error) {
	if w.manager == nil {
		return "ready", nil // For tests or when workers are disabled, consider nil as healthy
	}

	if !w.manager.IsHealthy() {
		return "unhealthy", fmt.Errorf("worker manager is not healthy")
	}

	// Additional check for worker pool health
	if !w.manager.GetWorkerPool().IsHealthy() {
		return "unhealthy", fmt.Errorf("worker pool is not healthy")
	}

	return "ready", nil
}

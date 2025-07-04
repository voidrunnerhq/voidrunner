package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/voidrunnerhq/voidrunner/internal/api/handlers"
	"github.com/voidrunnerhq/voidrunner/internal/api/middleware"
	"github.com/voidrunnerhq/voidrunner/internal/auth"
	"github.com/voidrunnerhq/voidrunner/internal/config"
	"github.com/voidrunnerhq/voidrunner/internal/database"
	"github.com/voidrunnerhq/voidrunner/pkg/logger"
)

func Setup(router *gin.Engine, cfg *config.Config, log *logger.Logger, repos *database.Repositories, authService *auth.Service) {
	setupMiddleware(router, cfg, log)
	setupRoutes(router, cfg, log, repos, authService)
}

func setupMiddleware(router *gin.Engine, cfg *config.Config, log *logger.Logger) {
	router.Use(middleware.SecurityHeaders())
	router.Use(middleware.RequestID())
	router.Use(middleware.CORS(cfg.CORS.AllowedOrigins, cfg.CORS.AllowedMethods, cfg.CORS.AllowedHeaders))
	router.Use(log.GinLogger())
	router.Use(log.GinRecovery())
	router.Use(middleware.ErrorHandler())
}

func setupRoutes(router *gin.Engine, cfg *config.Config, log *logger.Logger, repos *database.Repositories, authService *auth.Service) {
	healthHandler := handlers.NewHealthHandler()
	authHandler := handlers.NewAuthHandler(authService, log.Logger)
	authMiddleware := middleware.NewAuthMiddleware(authService, log.Logger)

	router.GET("/health", healthHandler.Health)
	router.GET("/ready", healthHandler.Readiness)

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
			auth.POST("/register", 
				middleware.RegisterRateLimit(log.Logger),
				authHandler.Register,
			)
			auth.POST("/login", 
				middleware.AuthRateLimit(log.Logger),
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
		executionHandler := handlers.NewTaskExecutionHandler(repos.Tasks, repos.TaskExecutions, log.Logger)
		taskValidation := middleware.TaskValidation(log.Logger)
		
		// Task CRUD operations
		protected.POST("/tasks", 
			middleware.RequestSizeLimit(log.Logger),
			middleware.TaskCreationRateLimit(log.Logger),
			taskValidation.ValidateTaskCreation(),
			taskHandler.Create,
		)
		protected.GET("/tasks", 
			middleware.TaskRateLimit(log.Logger),
			taskHandler.List,
		)
		protected.GET("/tasks/:id", 
			middleware.TaskRateLimit(log.Logger),
			taskHandler.GetByID,
		)
		protected.PUT("/tasks/:id", 
			middleware.RequestSizeLimit(log.Logger),
			middleware.TaskRateLimit(log.Logger),
			taskValidation.ValidateTaskUpdate(),
			taskHandler.Update,
		)
		protected.DELETE("/tasks/:id", 
			middleware.TaskRateLimit(log.Logger),
			taskHandler.Delete,
		)
		
		// Task execution operations
		protected.POST("/tasks/:task_id/executions", 
			middleware.ExecutionCreationRateLimit(log.Logger),
			executionHandler.Create,
		)
		protected.GET("/tasks/:task_id/executions", 
			middleware.TaskExecutionRateLimit(log.Logger),
			executionHandler.ListByTaskID,
		)
		protected.GET("/executions/:id", 
			middleware.TaskExecutionRateLimit(log.Logger),
			executionHandler.GetByID,
		)
		protected.PUT("/executions/:id", 
			middleware.RequestSizeLimit(log.Logger),
			middleware.TaskExecutionRateLimit(log.Logger),
			taskValidation.ValidateTaskExecutionUpdate(),
			executionHandler.Update,
		)
		protected.DELETE("/executions/:id", 
			middleware.TaskExecutionRateLimit(log.Logger),
			executionHandler.Cancel,
		)
	}
}
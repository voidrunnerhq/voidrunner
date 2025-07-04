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

		// Future API routes will use repos here
		// userHandler := handlers.NewUserHandler(repos.Users)
		// taskHandler := handlers.NewTaskHandler(repos.Tasks)
		// executionHandler := handlers.NewTaskExecutionHandler(repos.TaskExecutions)
		
		// protected.POST("/users", userHandler.Create)
		// protected.GET("/users/:id", userHandler.GetByID)
		// protected.POST("/tasks", taskHandler.Create)
		// protected.GET("/tasks/:id", taskHandler.GetByID)
		// protected.POST("/tasks/:id/executions", executionHandler.Create)
		// protected.GET("/executions/:id", executionHandler.GetByID)
	}
}
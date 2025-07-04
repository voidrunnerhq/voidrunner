package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/voidrunnerhq/voidrunner/internal/api/handlers"
	"github.com/voidrunnerhq/voidrunner/internal/api/middleware"
	"github.com/voidrunnerhq/voidrunner/internal/config"
	"github.com/voidrunnerhq/voidrunner/pkg/logger"
)

func Setup(router *gin.Engine, cfg *config.Config, log *logger.Logger) {
	setupMiddleware(router, cfg, log)
	setupRoutes(router)
}

func setupMiddleware(router *gin.Engine, cfg *config.Config, log *logger.Logger) {
	router.Use(middleware.SecurityHeaders())
	router.Use(middleware.RequestID())
	router.Use(middleware.CORS(cfg.CORS.AllowedOrigins, cfg.CORS.AllowedMethods, cfg.CORS.AllowedHeaders))
	router.Use(log.GinLogger())
	router.Use(log.GinRecovery())
	router.Use(middleware.ErrorHandler())
}

func setupRoutes(router *gin.Engine) {
	healthHandler := handlers.NewHealthHandler()

	router.GET("/health", healthHandler.Health)
	router.GET("/ready", healthHandler.Readiness)

	v1 := router.Group("/api/v1")
	{
		v1.GET("/ping", func(c *gin.Context) {
			c.JSON(200, gin.H{
				"message": "pong",
			})
		})
	}
}
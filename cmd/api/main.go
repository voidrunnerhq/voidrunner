package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/voidrunnerhq/voidrunner/internal/api/routes"
	"github.com/voidrunnerhq/voidrunner/internal/auth"
	"github.com/voidrunnerhq/voidrunner/internal/config"
	"github.com/voidrunnerhq/voidrunner/internal/database"
	"github.com/voidrunnerhq/voidrunner/pkg/logger"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	log := logger.New(cfg.Logger.Level, cfg.Logger.Format)

	// Initialize database connection
	dbConn, err := database.NewConnection(&cfg.Database, log.Logger)
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

	// Initialize JWT service
	jwtService := auth.NewJWTService(&cfg.JWT)

	// Initialize authentication service
	authService := auth.NewService(repos.Users, jwtService, log.Logger, cfg)

	if cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	routes.Setup(router, cfg, log, dbConn, repos, authService)

	srv := &http.Server{
		Addr:    fmt.Sprintf("%s:%s", cfg.Server.Host, cfg.Server.Port),
		Handler: router,
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

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Error("server forced to shutdown", "error", err)
		os.Exit(1)
	}

	log.Info("server exited")
}
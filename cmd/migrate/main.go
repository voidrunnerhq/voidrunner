package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/voidrunnerhq/voidrunner/internal/config"
	"github.com/voidrunnerhq/voidrunner/internal/database"
	"github.com/voidrunnerhq/voidrunner/pkg/logger"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run ./cmd/migrate <command>")
		fmt.Println("Commands:")
		fmt.Println("  up     - Apply all pending migrations")
		fmt.Println("  down   - Roll back one migration")
		fmt.Println("  reset  - Roll back all migrations")
		os.Exit(1)
	}

	command := os.Args[1]

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	log := logger.New("migrate", cfg.Logger.Level)

	// Get the absolute path to migrations directory
	migrationsPath := "file://migrations"
	if absPath, err := filepath.Abs("migrations"); err == nil {
		migrationsPath = fmt.Sprintf("file://%s", absPath)
	}

	// Create migration config
	migrateConfig := &database.MigrateConfig{
		DatabaseConfig: &cfg.Database,
		MigrationsPath: migrationsPath,
		Logger:         log.Logger,
	}

	// Execute the command
	switch command {
	case "up":
		if err := database.MigrateUp(migrateConfig); err != nil {
			fmt.Fprintf(os.Stderr, "Migration failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Migrations applied successfully")

	case "down":
		if err := database.MigrateDown(migrateConfig); err != nil {
			fmt.Fprintf(os.Stderr, "Migration rollback failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Migration rolled back successfully")

	case "reset":
		if err := database.MigrateReset(migrateConfig); err != nil {
			fmt.Fprintf(os.Stderr, "Database reset failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Database reset successfully")

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		fmt.Println("Available commands: up, down, reset")
		os.Exit(1)
	}
}

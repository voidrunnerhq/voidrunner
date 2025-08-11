package main

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/voidrunnerhq/voidrunner/internal/config"
	"github.com/voidrunnerhq/voidrunner/internal/database"
	"github.com/voidrunnerhq/voidrunner/pkg/logger"
	_ "github.com/jackc/pgx/v5/stdlib"
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
		// Enhanced debugging for migration up
		fmt.Println("=== Database Migration Debug Info ===")
		fmt.Printf("Database: %s@%s:%s/%s (ssl=%s)\n", 
			cfg.Database.User, cfg.Database.Host, cfg.Database.Port, 
			cfg.Database.Database, cfg.Database.SSLMode)
		fmt.Printf("Migrations path: %s\n", migrationsPath)
		
		// Check current migration state before applying
		if err := debugMigrationState(migrateConfig); err != nil {
			log.Warn("Failed to get migration state before applying", "error", err)
		}
		
		if err := database.MigrateUp(migrateConfig); err != nil {
			fmt.Fprintf(os.Stderr, "Migration failed: %v\n", err)
			os.Exit(1)
		}
		
		// Check migration state after applying
		if err := debugMigrationState(migrateConfig); err != nil {
			log.Warn("Failed to get migration state after applying", "error", err)
		}
		
		// Validate critical database objects exist
		if err := validateDatabaseObjects(migrateConfig); err != nil {
			fmt.Fprintf(os.Stderr, "Database object validation failed: %v\n", err)
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

// debugMigrationState prints current migration version and state
func debugMigrationState(cfg *database.MigrateConfig) error {
	migrator, err := database.NewMigrator(cfg)
	if err != nil {
		return fmt.Errorf("failed to create migrator for debugging: %w", err)
	}
	defer func() { _ = migrator.Close() }()

	version, dirty, err := migrator.Version()
	if err != nil {
		fmt.Printf("Migration version: <none applied>\n")
	} else {
		fmt.Printf("Migration version: %d (dirty: %t)\n", version, dirty)
	}

	return nil
}

// validateDatabaseObjects checks that critical database objects exist after migration
func validateDatabaseObjects(cfg *database.MigrateConfig) error {
	// Create database connection
	connStr := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s",
		cfg.DatabaseConfig.User,
		cfg.DatabaseConfig.Password,
		cfg.DatabaseConfig.Host,
		cfg.DatabaseConfig.Port,
		cfg.DatabaseConfig.Database,
		cfg.DatabaseConfig.SSLMode,
	)

	db, err := sql.Open("pgx", connStr)
	if err != nil {
		return fmt.Errorf("failed to connect to database for validation: %w", err)
	}
	defer func() { _ = db.Close() }()

	// Check if task_logs table exists
	var taskLogsExists bool
	err = db.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables 
			WHERE table_schema = 'public' AND table_name = 'task_logs'
		)
	`).Scan(&taskLogsExists)
	if err != nil {
		return fmt.Errorf("failed to check task_logs table existence: %w", err)
	}

	fmt.Printf("task_logs table exists: %t\n", taskLogsExists)

	// Check if create_task_logs_partition function exists
	var functionExists bool
	err = db.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.routines 
			WHERE routine_schema = 'public' AND routine_name = 'create_task_logs_partition'
		)
	`).Scan(&functionExists)
	if err != nil {
		return fmt.Errorf("failed to check create_task_logs_partition function existence: %w", err)
	}

	fmt.Printf("create_task_logs_partition function exists: %t\n", functionExists)

	// Count existing partitions
	var partitionCount int
	err = db.QueryRow(`
		SELECT COUNT(*) 
		FROM pg_tables 
		WHERE tablename LIKE 'task_logs_%' 
		AND schemaname = 'public'
	`).Scan(&partitionCount)
	if err != nil {
		return fmt.Errorf("failed to count task_logs partitions: %w", err)
	}

	fmt.Printf("task_logs partitions created: %d\n", partitionCount)

	// Validate that all required objects exist
	if !taskLogsExists {
		return fmt.Errorf("task_logs table was not created by migration")
	}

	if !functionExists {
		return fmt.Errorf("create_task_logs_partition function was not created by migration")
	}

	if partitionCount == 0 {
		fmt.Println("WARNING: No task_logs partitions were created, this might cause issues")
	}

	fmt.Println("Database object validation passed")
	return nil
}

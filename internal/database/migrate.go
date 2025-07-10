package database

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/jackc/pgx/v5/stdlib" // pgx driver for database/sql
	"github.com/voidrunnerhq/voidrunner/internal/config"
)

// MigrateConfig holds migration configuration
type MigrateConfig struct {
	DatabaseConfig *config.DatabaseConfig
	MigrationsPath string
	Logger         *slog.Logger
}

// Migrator handles database migrations
type Migrator struct {
	migrate *migrate.Migrate
	logger  *slog.Logger
}

// NewMigrator creates a new database migrator
func NewMigrator(cfg *MigrateConfig) (*Migrator, error) {
	if cfg == nil {
		return nil, fmt.Errorf("migration configuration is required")
	}

	if cfg.DatabaseConfig == nil {
		return nil, fmt.Errorf("database configuration is required")
	}

	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	if cfg.MigrationsPath == "" {
		cfg.MigrationsPath = "file://migrations"
	}

	// Create database connection string for sql.DB
	connStr := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s",
		cfg.DatabaseConfig.User,
		cfg.DatabaseConfig.Password,
		cfg.DatabaseConfig.Host,
		cfg.DatabaseConfig.Port,
		cfg.DatabaseConfig.Database,
		cfg.DatabaseConfig.SSLMode,
	)

	// Open database connection using database/sql
	db, err := sql.Open("pgx", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	// Test the connection
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to create postgres driver: %w", err)
	}

	m, err := migrate.NewWithDatabaseInstance(
		cfg.MigrationsPath,
		"postgres",
		driver,
	)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to create migrator: %w", err)
	}

	return &Migrator{
		migrate: m,
		logger:  cfg.Logger,
	}, nil
}

// Up applies all pending migrations
func (m *Migrator) Up() error {
	m.logger.Info("applying database migrations")

	err := m.migrate.Up()
	if err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			m.logger.Info("no migrations to apply")
			return nil
		}
		return fmt.Errorf("failed to apply migrations: %w", err)
	}

	m.logger.Info("database migrations applied successfully")
	return nil
}

// Down rolls back one migration
func (m *Migrator) Down() error {
	m.logger.Info("rolling back database migration")

	err := m.migrate.Steps(-1)
	if err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			m.logger.Info("no migrations to roll back")
			return nil
		}
		return fmt.Errorf("failed to roll back migration: %w", err)
	}

	m.logger.Info("database migration rolled back successfully")
	return nil
}

// Reset rolls back all migrations
func (m *Migrator) Reset() error {
	m.logger.Info("resetting database (rolling back all migrations)")

	err := m.migrate.Drop()
	if err != nil {
		return fmt.Errorf("failed to reset database: %w", err)
	}

	m.logger.Info("database reset successfully")
	return nil
}

// Version returns the current migration version
func (m *Migrator) Version() (uint, bool, error) {
	version, dirty, err := m.migrate.Version()
	if err != nil {
		if errors.Is(err, migrate.ErrNilVersion) {
			return 0, false, nil
		}
		return 0, false, fmt.Errorf("failed to get migration version: %w", err)
	}

	return version, dirty, nil
}

// ForceVersion forces the migration version (use with caution)
func (m *Migrator) ForceVersion(version int) error {
	m.logger.Warn("forcing migration version", "version", version)

	err := m.migrate.Force(version)
	if err != nil {
		return fmt.Errorf("failed to force migration version: %w", err)
	}

	m.logger.Info("migration version forced successfully", "version", version)
	return nil
}

// Close closes the migrator
func (m *Migrator) Close() error {
	if m.migrate != nil {
		sourceErr, dbErr := m.migrate.Close()
		if sourceErr != nil {
			return fmt.Errorf("failed to close migration source: %w", sourceErr)
		}
		if dbErr != nil {
			return fmt.Errorf("failed to close migration database: %w", dbErr)
		}
	}
	return nil
}

// MigrateUp is a convenience function to apply migrations
func MigrateUp(cfg *MigrateConfig) error {
	migrator, err := NewMigrator(cfg)
	if err != nil {
		return err
	}
	defer func() { _ = migrator.Close() }()

	return migrator.Up()
}

// MigrateDown is a convenience function to roll back migrations
func MigrateDown(cfg *MigrateConfig) error {
	migrator, err := NewMigrator(cfg)
	if err != nil {
		return err
	}
	defer func() { _ = migrator.Close() }()

	return migrator.Down()
}

// MigrateReset is a convenience function to reset the database
func MigrateReset(cfg *MigrateConfig) error {
	migrator, err := NewMigrator(cfg)
	if err != nil {
		return err
	}
	defer func() { _ = migrator.Close() }()

	return migrator.Reset()
}

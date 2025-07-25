package database

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/voidrunnerhq/voidrunner/internal/config"
)

// Connection represents a database connection pool
type Connection struct {
	Pool   *pgxpool.Pool
	logger *slog.Logger
}

// NewConnection creates a new database connection pool
func NewConnection(cfg *config.DatabaseConfig, logger *slog.Logger) (*Connection, error) {
	if cfg == nil {
		return nil, fmt.Errorf("database configuration is required")
	}

	if logger == nil {
		logger = slog.Default()
	}

	connStr := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s",
		cfg.User,
		cfg.Password,
		cfg.Host,
		cfg.Port,
		cfg.Database,
		cfg.SSLMode,
	)

	poolConfig, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database connection string: %w", err)
	}

	// Configure connection pool settings for optimal performance
	poolConfig.MaxConns = 25                       // Maximum number of connections
	poolConfig.MinConns = 5                        // Minimum number of connections
	poolConfig.MaxConnLifetime = time.Hour * 1     // Maximum connection lifetime
	poolConfig.MaxConnIdleTime = time.Minute * 30  // Maximum connection idle time
	poolConfig.HealthCheckPeriod = time.Minute * 5 // Health check frequency

	// Connection timeout settings
	poolConfig.ConnConfig.ConnectTimeout = time.Second * 10
	poolConfig.ConnConfig.RuntimeParams["statement_timeout"] = "30s"
	poolConfig.ConnConfig.RuntimeParams["idle_in_transaction_session_timeout"] = "60s"

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create database pool: %w", err)
	}

	// Test the connection
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	logger.Info("database connection pool created successfully",
		"host", cfg.Host,
		"port", cfg.Port,
		"database", cfg.Database,
		"max_conns", poolConfig.MaxConns,
		"min_conns", poolConfig.MinConns,
	)

	return &Connection{
		Pool:   pool,
		logger: logger,
	}, nil
}

// NewConnectionWithRetry creates a new database connection pool with retry logic
func NewConnectionWithRetry(cfg *config.DatabaseConfig, logger *slog.Logger) (*Connection, error) {
	if logger == nil {
		logger = slog.Default()
	}

	maxRetries := 5
	retryDelay := time.Second * 2

	for attempt := 1; attempt <= maxRetries; attempt++ {
		logger.Info("attempting database connection",
			"attempt", attempt,
			"max_retries", maxRetries,
			"host", cfg.Host,
			"port", cfg.Port,
			"database", cfg.Database,
		)

		conn, err := NewConnection(cfg, logger)
		if err == nil {
			logger.Info("database connection established successfully", "attempt", attempt)
			return conn, nil
		}

		logger.Warn("database connection failed, retrying...",
			"attempt", attempt,
			"error", err,
			"retry_delay", retryDelay,
		)

		if attempt < maxRetries {
			time.Sleep(retryDelay)
			// Exponential backoff
			retryDelay = time.Duration(float64(retryDelay) * 1.5)
		}
	}

	return nil, fmt.Errorf("failed to establish database connection after %d attempts", maxRetries)
}

// Close closes the database connection pool
func (c *Connection) Close() {
	if c.Pool != nil {
		c.logger.Info("closing database connection pool")
		c.Pool.Close()
	}
}

// Ping checks if the database connection is alive
func (c *Connection) Ping(ctx context.Context) error {
	return c.Pool.Ping(ctx)
}

// Stats returns connection pool statistics
func (c *Connection) Stats() *pgxpool.Stat {
	return c.Pool.Stat()
}

// LogStats logs connection pool statistics
func (c *Connection) LogStats() {
	stats := c.Stats()
	c.logger.Info("database connection pool stats",
		"total_conns", stats.TotalConns(),
		"idle_conns", stats.IdleConns(),
		"acquired_conns", stats.AcquiredConns(),
		"constructing_conns", stats.ConstructingConns(),
		"acquire_count", stats.AcquireCount(),
		"acquire_duration", stats.AcquireDuration(),
		"acquired_conns_duration", stats.AcquiredConns(),
		"canceled_acquire_count", stats.CanceledAcquireCount(),
		"empty_acquire_count", stats.EmptyAcquireCount(),
		"max_conns", stats.MaxConns(),
		"new_conns_count", stats.NewConnsCount(),
	)
}

// HealthCheck performs a comprehensive health check of the database connection
func (c *Connection) HealthCheck(ctx context.Context) error {
	// Check if pool is available
	if c.Pool == nil {
		return fmt.Errorf("database pool is not initialized")
	}

	// Ping the database
	if err := c.Pool.Ping(ctx); err != nil {
		return fmt.Errorf("database ping failed: %w", err)
	}

	// Check pool statistics
	stats := c.Stats()
	if stats.TotalConns() == 0 {
		return fmt.Errorf("no database connections available")
	}

	// Execute a simple query to ensure the database is responsive
	var result int
	err := c.Pool.QueryRow(ctx, "SELECT 1").Scan(&result)
	if err != nil {
		return fmt.Errorf("database query test failed: %w", err)
	}

	if result != 1 {
		return fmt.Errorf("unexpected database query result: %d", result)
	}

	return nil
}

// Transaction interface represents a database transaction
type Transaction interface {
	pgx.Tx
	// Repositories provides access to transaction-aware repositories
	Repositories() TransactionalRepositories
}

// TransactionalRepositories provides transaction-aware repository interfaces
type TransactionalRepositories struct {
	Tasks          TaskRepository
	TaskExecutions TaskExecutionRepository
	Users          UserRepository
}

// transaction implements the Transaction interface
type transaction struct {
	pgx.Tx
	conn *Connection
}

// Repositories returns transaction-aware repositories
func (t *transaction) Repositories() TransactionalRepositories {
	return TransactionalRepositories{
		Tasks:          NewTaskRepositoryWithTx(t.Tx),
		TaskExecutions: NewTaskExecutionRepositoryWithTx(t.Tx),
		Users:          NewUserRepositoryWithTx(t.Tx),
	}
}

// BeginTx starts a new database transaction
func (c *Connection) BeginTx(ctx context.Context) (Transaction, error) {
	tx, err := c.Pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}

	return &transaction{
		Tx:   tx,
		conn: c,
	}, nil
}

// WithTransaction executes a function within a database transaction
// If the function returns an error, the transaction is rolled back
// Otherwise, the transaction is committed
func (c *Connection) WithTransaction(ctx context.Context, fn func(tx Transaction) error) error {
	tx, err := c.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if err := tx.Rollback(ctx); err != nil && err != pgx.ErrTxClosed {
			c.logger.Error("failed to rollback transaction", "error", err)
		}
	}()

	if err := fn(tx); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

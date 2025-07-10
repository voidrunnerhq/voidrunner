package testutil

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/voidrunnerhq/voidrunner/internal/config"
	"github.com/voidrunnerhq/voidrunner/internal/database"
	"github.com/voidrunnerhq/voidrunner/internal/models"
	"github.com/voidrunnerhq/voidrunner/pkg/logger"
)

const (
	// TestDatabaseTimeout for database operations in tests
	TestDatabaseTimeout = 30 * time.Second

	// TestDBName default test database name
	TestDBName = "voidrunner_test"
)

// getMigrationsPath returns the absolute path to the migrations directory
func getMigrationsPath() string {
	// Get the current file's directory
	_, filename, _, _ := runtime.Caller(0)
	dir := filepath.Dir(filename)

	// Navigate up to the project root and find migrations directory
	for {
		migrationsPath := filepath.Join(dir, "migrations")
		if _, err := os.Stat(migrationsPath); err == nil {
			return "file://" + migrationsPath
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root, fallback to relative path
			return "file://../../migrations"
		}
		dir = parent
	}
}

// DatabaseHelper provides utilities for database testing
type DatabaseHelper struct {
	DB           *database.Connection
	Repositories *database.Repositories
	Config       *config.Config
}

// NewDatabaseHelper creates a new database helper for testing
func NewDatabaseHelper(t *testing.T) *DatabaseHelper {
	t.Helper()

	cfg := GetTestConfig()

	// Skip test if database is not available
	if !isDatabaseAvailable(cfg) {
		t.Skip("Test database not available")
	}

	// Create logger
	log := logger.New("error", "json") // Reduce noise in tests

	// Connect to database
	db, err := database.NewConnection(&cfg.Database, log.Logger)
	require.NoError(t, err, "failed to connect to test database")

	// Run migrations
	migrateCfg := &database.MigrateConfig{
		DatabaseConfig: &cfg.Database,
		MigrationsPath: getMigrationsPath(),
		Logger:         log.Logger,
	}
	err = database.MigrateUp(migrateCfg)
	require.NoError(t, err, "failed to run migrations")

	// Create repositories
	repos := database.NewRepositories(db)

	return &DatabaseHelper{
		DB:           db,
		Repositories: repos,
		Config:       cfg,
	}
}

// GetTestConfig returns configuration for testing
func GetTestConfig() *config.Config {
	return &config.Config{
		Server: config.ServerConfig{
			Port: getEnvOrDefault("TEST_SERVER_PORT", "8080"),
			Host: getEnvOrDefault("TEST_SERVER_HOST", "localhost"),
			Env:  "test",
		},
		Database: config.DatabaseConfig{
			Host:     getEnvOrDefault("TEST_DB_HOST", "localhost"),
			Port:     getEnvOrDefault("TEST_DB_PORT", "5432"),
			Database: getEnvOrDefault("TEST_DB_NAME", TestDBName),
			User:     getEnvOrDefault("TEST_DB_USER", "testuser"),
			Password: getEnvOrDefault("TEST_DB_PASSWORD", "testpassword"),
			SSLMode:  getEnvOrDefault("TEST_DB_SSLMODE", "disable"),
		},
		JWT: config.JWTConfig{
			SecretKey:            getEnvOrDefault("JWT_SECRET_KEY", "test-secret-key-for-integration"),
			AccessTokenDuration:  15 * time.Minute,
			RefreshTokenDuration: 24 * time.Hour,
			Issuer:               "voidrunner-test",
			Audience:             "voidrunner-api-test",
		},
		CORS: config.CORSConfig{
			AllowedOrigins: []string{"http://localhost:3000", "http://localhost:5173"},
			AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
			AllowedHeaders: []string{"Content-Type", "Authorization", "X-Request-ID"},
		},
	}
}

// CleanupDatabase removes all test data from database tables
func (h *DatabaseHelper) CleanupDatabase(t *testing.T) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), TestDatabaseTimeout)
	defer cancel()

	// Delete in correct order to avoid foreign key constraints
	queries := []string{
		"DELETE FROM task_executions",
		"DELETE FROM tasks",
		"DELETE FROM users",
	}

	for _, query := range queries {
		_, err := h.DB.Pool.Exec(ctx, query)
		require.NoError(t, err, "failed to cleanup database: %s", query)
	}
}

// SeedDatabase populates the database with test fixtures
func (h *DatabaseHelper) SeedDatabase(t *testing.T, fixtures *AllFixtures) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), TestDatabaseTimeout)
	defer cancel()

	// Seed users first
	h.SeedUsers(t, ctx, fixtures.Users)

	// Then tasks
	h.SeedTasks(t, ctx, fixtures.Tasks)

	// Finally executions
	h.SeedExecutions(t, ctx, fixtures.Executions)
}

// SeedUsers inserts user fixtures into the database
func (h *DatabaseHelper) SeedUsers(t *testing.T, ctx context.Context, users *UserFixtures) {
	t.Helper()

	usersList := []*models.User{
		users.AdminUser,
		users.RegularUser,
		users.InactiveUser,
	}

	for _, user := range usersList {
		err := h.Repositories.Users.Create(ctx, user)
		require.NoError(t, err, "failed to seed user: %s", user.Email)
	}
}

// SeedTasks inserts task fixtures into the database
func (h *DatabaseHelper) SeedTasks(t *testing.T, ctx context.Context, tasks *TaskFixtures) {
	t.Helper()

	tasksList := []*models.Task{
		tasks.PendingTask,
		tasks.RunningTask,
		tasks.CompletedTask,
		tasks.FailedTask,
	}

	for _, task := range tasksList {
		err := h.Repositories.Tasks.Create(ctx, task)
		require.NoError(t, err, "failed to seed task: %s", task.Name)
	}
}

// SeedExecutions inserts execution fixtures into the database
func (h *DatabaseHelper) SeedExecutions(t *testing.T, ctx context.Context, executions *ExecutionFixtures) {
	t.Helper()

	executionsList := []*models.TaskExecution{
		executions.SuccessfulExecution,
		executions.FailedExecution,
		executions.TimeoutExecution,
	}

	for _, execution := range executionsList {
		err := h.Repositories.TaskExecutions.Create(ctx, execution)
		require.NoError(t, err, "failed to seed execution: %s", execution.ID)
	}
}

// CreateMinimalUser creates a minimal user for testing
func (h *DatabaseHelper) CreateMinimalUser(t *testing.T, ctx context.Context, email, name string) *models.User {
	t.Helper()

	user := &models.User{
		Email:        email,
		PasswordHash: "$2a$12$LQv3c1yqBWVHxkd0LHAkCOYz6TtxMQJqhN8/LewdBPj4KTrkYEoA2", // TestPass123!
		Name:         name,
	}

	err := h.Repositories.Users.Create(ctx, user)
	require.NoError(t, err, "failed to create minimal user")

	return user
}

// CreateMinimalTask creates a minimal task for testing
func (h *DatabaseHelper) CreateMinimalTask(t *testing.T, ctx context.Context, userID uuid.UUID, name string) *models.Task {
	t.Helper()

	task := &models.Task{
		UserID:         userID,
		Name:           name,
		ScriptContent:  "print('test')",
		ScriptType:     models.ScriptTypePython,
		Status:         models.TaskStatusPending,
		Priority:       1,
		TimeoutSeconds: 30,
		Metadata:       models.JSONB{},
	}

	err := h.Repositories.Tasks.Create(ctx, task)
	require.NoError(t, err, "failed to create minimal task")

	return task
}

// CreateMinimalExecution creates a minimal execution for testing
func (h *DatabaseHelper) CreateMinimalExecution(t *testing.T, ctx context.Context, taskID uuid.UUID) *models.TaskExecution {
	t.Helper()

	execution := &models.TaskExecution{
		TaskID: taskID,
		Status: models.ExecutionStatusPending,
	}

	err := h.Repositories.TaskExecutions.Create(ctx, execution)
	require.NoError(t, err, "failed to create minimal execution")

	return execution
}

// WithCleanDatabase executes a test function with a clean database
func (h *DatabaseHelper) WithCleanDatabase(t *testing.T, testFn func()) {
	t.Helper()

	// Clean before
	h.CleanupDatabase(t)

	// Ensure cleanup after test
	defer h.CleanupDatabase(t)

	// Run test
	testFn()
}

// WithSeededDatabase executes a test function with seeded database
func (h *DatabaseHelper) WithSeededDatabase(t *testing.T, fixtures *AllFixtures, testFn func()) {
	t.Helper()

	h.WithCleanDatabase(t, func() {
		h.SeedDatabase(t, fixtures)
		testFn()
	})
}

// Close closes the database connection
func (h *DatabaseHelper) Close() {
	if h.DB != nil {
		h.DB.Close()
	}
}

// isDatabaseAvailable checks if the test database is available
func isDatabaseAvailable(cfg *config.Config) bool {
	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		cfg.Database.Host, cfg.Database.Port, cfg.Database.User,
		cfg.Database.Password, cfg.Database.Database, cfg.Database.SSLMode)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return false
	}
	defer func() { _ = db.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return false
	}

	return true
}

// getEnvOrDefault returns environment variable value or default
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// SetupTestDatabase creates a test database if it doesn't exist
func SetupTestDatabase() error {
	cfg := GetTestConfig()

	// Connect to postgres database to create test database
	adminConnStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=postgres sslmode=%s",
		cfg.Database.Host, cfg.Database.Port, cfg.Database.User,
		cfg.Database.Password, cfg.Database.SSLMode)

	db, err := sql.Open("postgres", adminConnStr)
	if err != nil {
		return fmt.Errorf("failed to connect to postgres: %w", err)
	}
	defer func() { _ = db.Close() }()

	// Check if test database exists
	var exists bool
	err = db.QueryRow("SELECT EXISTS(SELECT datname FROM pg_catalog.pg_database WHERE datname = $1)", cfg.Database.Database).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to check if database exists: %w", err)
	}

	// Create test database if it doesn't exist
	if !exists {
		_, err = db.Exec(fmt.Sprintf("CREATE DATABASE %s", cfg.Database.Database))
		if err != nil {
			return fmt.Errorf("failed to create test database: %w", err)
		}
		log.Printf("Created test database: %s", cfg.Database.Database)
	}

	return nil
}

// TeardownTestDatabase drops the test database
func TeardownTestDatabase() error {
	cfg := GetTestConfig()

	// Connect to postgres database to drop test database
	adminConnStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=postgres sslmode=%s",
		cfg.Database.Host, cfg.Database.Port, cfg.Database.User,
		cfg.Database.Password, cfg.Database.SSLMode)

	db, err := sql.Open("postgres", adminConnStr)
	if err != nil {
		return fmt.Errorf("failed to connect to postgres: %w", err)
	}
	defer func() { _ = db.Close() }()

	// Terminate existing connections to the test database
	_, err = db.Exec("SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = $1 AND pid <> pg_backend_pid()", cfg.Database.Database)
	if err != nil {
		log.Printf("Warning: failed to terminate connections to test database: %v", err)
	}

	// Drop test database
	_, err = db.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS %s", cfg.Database.Database))
	if err != nil {
		return fmt.Errorf("failed to drop test database: %w", err)
	}

	log.Printf("Dropped test database: %s", cfg.Database.Database)
	return nil
}

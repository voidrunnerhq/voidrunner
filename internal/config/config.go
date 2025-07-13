package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Server          ServerConfig
	Database        DatabaseConfig
	Logger          LoggerConfig
	CORS            CORSConfig
	JWT             JWTConfig
	Executor        ExecutorConfig
	Redis           RedisConfig
	Queue           QueueConfig
	Worker          WorkerConfig
	EmbeddedWorkers bool // Enable worker pool in API server process
}

type ServerConfig struct {
	Port string
	Host string
	Env  string
}

type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Database string
	SSLMode  string
}

type LoggerConfig struct {
	Level  string
	Format string
}

type CORSConfig struct {
	AllowedOrigins []string
	AllowedMethods []string
	AllowedHeaders []string
}

type JWTConfig struct {
	SecretKey            string
	AccessTokenDuration  time.Duration
	RefreshTokenDuration time.Duration
	Issuer               string
	Audience             string
}

type ExecutorConfig struct {
	DockerEndpoint        string
	DefaultMemoryLimitMB  int
	DefaultCPUQuota       int64
	DefaultPidsLimit      int64
	DefaultTimeoutSeconds int
	PythonImage           string
	BashImage             string
	JavaScriptImage       string
	GoImage               string
	EnableSeccomp         bool
	SeccompProfilePath    string
	EnableAppArmor        bool
	AppArmorProfile       string
	ExecutionUser         string
}

type RedisConfig struct {
	Host               string
	Port               string
	Password           string
	Database           int
	PoolSize           int
	MinIdleConnections int
	MaxRetries         int
	DialTimeout        time.Duration
	ReadTimeout        time.Duration
	WriteTimeout       time.Duration
	IdleTimeout        time.Duration
}

type QueueConfig struct {
	TaskQueueName       string
	DeadLetterQueueName string
	RetryQueueName      string
	DefaultPriority     int
	MaxRetries          int
	RetryDelay          time.Duration
	RetryBackoffFactor  float64
	MaxRetryDelay       time.Duration
	VisibilityTimeout   time.Duration
	MessageTTL          time.Duration
	BatchSize           int
}

type WorkerConfig struct {
	PoolSize               int
	MaxConcurrentTasks     int
	MaxUserConcurrentTasks int
	TaskTimeout            time.Duration
	HeartbeatInterval      time.Duration
	ShutdownTimeout        time.Duration
	CleanupInterval        time.Duration
	StaleTaskThreshold     time.Duration
	WorkerIDPrefix         string
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	config := &Config{
		Server: ServerConfig{
			Port: getEnv("SERVER_PORT", "8080"),
			Host: getEnv("SERVER_HOST", "localhost"),
			Env:  getEnv("SERVER_ENV", "development"),
		},
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnv("DB_PORT", "5432"),
			User:     getEnv("DB_USER", "postgres"),
			Password: getEnv("DB_PASSWORD", ""),
			Database: getEnv("DB_NAME", "voidrunner"),
			SSLMode:  getEnv("DB_SSL_MODE", "disable"),
		},
		Logger: LoggerConfig{
			Level:  getEnv("LOG_LEVEL", "info"),
			Format: getEnv("LOG_FORMAT", "json"),
		},
		CORS: CORSConfig{
			AllowedOrigins: getEnvSlice("CORS_ALLOWED_ORIGINS", []string{"http://localhost:3000", "http://localhost:5173"}),
			AllowedMethods: getEnvSlice("CORS_ALLOWED_METHODS", []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}),
			AllowedHeaders: getEnvSlice("CORS_ALLOWED_HEADERS", []string{"Content-Type", "Authorization", "X-Request-ID"}),
		},
		JWT: JWTConfig{
			SecretKey:            getEnv("JWT_SECRET_KEY", "your-secret-key-change-in-production"),
			AccessTokenDuration:  getEnvDuration("JWT_ACCESS_TOKEN_DURATION", 15*time.Minute),
			RefreshTokenDuration: getEnvDuration("JWT_REFRESH_TOKEN_DURATION", 7*24*time.Hour),
			Issuer:               getEnv("JWT_ISSUER", "voidrunner"),
			Audience:             getEnv("JWT_AUDIENCE", "voidrunner-api"),
		},
		Executor: ExecutorConfig{
			DockerEndpoint:        getEnv("DOCKER_ENDPOINT", "unix:///var/run/docker.sock"),
			DefaultMemoryLimitMB:  getEnvInt("EXECUTOR_DEFAULT_MEMORY_LIMIT_MB", 128),
			DefaultCPUQuota:       getEnvInt64("EXECUTOR_DEFAULT_CPU_QUOTA", 50000),
			DefaultPidsLimit:      getEnvInt64("EXECUTOR_DEFAULT_PIDS_LIMIT", 128),
			DefaultTimeoutSeconds: getEnvInt("EXECUTOR_DEFAULT_TIMEOUT_SECONDS", 300),
			PythonImage:           getEnv("EXECUTOR_PYTHON_IMAGE", "python:3.11-alpine"),
			BashImage:             getEnv("EXECUTOR_BASH_IMAGE", "alpine:latest"),
			JavaScriptImage:       getEnv("EXECUTOR_JAVASCRIPT_IMAGE", "node:18-alpine"),
			GoImage:               getEnv("EXECUTOR_GO_IMAGE", "golang:1.21-alpine"),
			EnableSeccomp:         getEnvBool("EXECUTOR_ENABLE_SECCOMP", true),
			SeccompProfilePath:    getEnv("EXECUTOR_SECCOMP_PROFILE_PATH", "/opt/voidrunner/seccomp-profile.json"),
			EnableAppArmor:        getEnvBool("EXECUTOR_ENABLE_APPARMOR", false),
			AppArmorProfile:       getEnv("EXECUTOR_APPARMOR_PROFILE", "voidrunner-executor"),
			ExecutionUser:         getEnv("EXECUTOR_EXECUTION_USER", "1000:1000"),
		},
		Redis: RedisConfig{
			Host:               getEnv("REDIS_HOST", "localhost"),
			Port:               getEnv("REDIS_PORT", "6379"),
			Password:           getEnv("REDIS_PASSWORD", ""),
			Database:           getEnvInt("REDIS_DATABASE", 0),
			PoolSize:           getEnvInt("REDIS_POOL_SIZE", 10),
			MinIdleConnections: getEnvInt("REDIS_MIN_IDLE_CONNECTIONS", 5),
			MaxRetries:         getEnvInt("REDIS_MAX_RETRIES", 3),
			DialTimeout:        getEnvDuration("REDIS_DIAL_TIMEOUT", 5*time.Second),
			ReadTimeout:        getEnvDuration("REDIS_READ_TIMEOUT", 3*time.Second),
			WriteTimeout:       getEnvDuration("REDIS_WRITE_TIMEOUT", 3*time.Second),
			IdleTimeout:        getEnvDuration("REDIS_IDLE_TIMEOUT", 5*time.Minute),
		},
		Queue: QueueConfig{
			TaskQueueName:       getEnv("QUEUE_TASK_QUEUE_NAME", "voidrunner:tasks"),
			DeadLetterQueueName: getEnv("QUEUE_DEAD_LETTER_QUEUE_NAME", "voidrunner:tasks:dead"),
			RetryQueueName:      getEnv("QUEUE_RETRY_QUEUE_NAME", "voidrunner:tasks:retry"),
			DefaultPriority:     getEnvInt("QUEUE_DEFAULT_PRIORITY", 5),
			MaxRetries:          getEnvInt("QUEUE_MAX_RETRIES", 3),
			RetryDelay:          getEnvDuration("QUEUE_RETRY_DELAY", 30*time.Second),
			RetryBackoffFactor:  getEnvFloat64("QUEUE_RETRY_BACKOFF_FACTOR", 2.0),
			MaxRetryDelay:       getEnvDuration("QUEUE_MAX_RETRY_DELAY", 15*time.Minute),
			VisibilityTimeout:   getEnvDuration("QUEUE_VISIBILITY_TIMEOUT", 30*time.Minute),
			MessageTTL:          getEnvDuration("QUEUE_MESSAGE_TTL", 24*time.Hour),
			BatchSize:           getEnvInt("QUEUE_BATCH_SIZE", 10),
		},
		Worker: WorkerConfig{
			PoolSize:               getEnvInt("WORKER_POOL_SIZE", 5),
			MaxConcurrentTasks:     getEnvInt("WORKER_MAX_CONCURRENT_TASKS", 10),
			MaxUserConcurrentTasks: getEnvInt("WORKER_MAX_USER_CONCURRENT_TASKS", 3),
			TaskTimeout:            getEnvDuration("WORKER_TASK_TIMEOUT", 1*time.Hour),
			HeartbeatInterval:      getEnvDuration("WORKER_HEARTBEAT_INTERVAL", 30*time.Second),
			ShutdownTimeout:        getEnvDuration("WORKER_SHUTDOWN_TIMEOUT", 30*time.Second),
			CleanupInterval:        getEnvDuration("WORKER_CLEANUP_INTERVAL", 5*time.Minute),
			StaleTaskThreshold:     getEnvDuration("WORKER_STALE_TASK_THRESHOLD", 2*time.Hour),
			WorkerIDPrefix:         getEnv("WORKER_ID_PREFIX", "voidrunner-worker"),
		},
		EmbeddedWorkers: getEnvBool("EMBEDDED_WORKERS", true), // Default true for development simplicity
	}

	if err := config.validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return config, nil
}

func (c *Config) validate() error {
	if c.Server.Port == "" {
		return fmt.Errorf("server port is required")
	}

	if _, err := strconv.Atoi(c.Server.Port); err != nil {
		return fmt.Errorf("invalid server port: %s", c.Server.Port)
	}

	if c.Database.Host == "" {
		return fmt.Errorf("database host is required")
	}

	if c.Database.User == "" {
		return fmt.Errorf("database user is required")
	}

	if c.Database.Database == "" {
		return fmt.Errorf("database name is required")
	}

	if c.JWT.SecretKey == "" {
		return fmt.Errorf("JWT secret key is required")
	}

	if c.JWT.AccessTokenDuration <= 0 {
		return fmt.Errorf("JWT access token duration must be positive")
	}

	if c.JWT.RefreshTokenDuration <= 0 {
		return fmt.Errorf("JWT refresh token duration must be positive")
	}

	if c.Executor.DefaultMemoryLimitMB <= 0 {
		return fmt.Errorf("executor default memory limit must be positive")
	}

	if c.Executor.DefaultCPUQuota <= 0 {
		return fmt.Errorf("executor default CPU quota must be positive")
	}

	if c.Executor.DefaultPidsLimit <= 0 {
		return fmt.Errorf("executor default PID limit must be positive")
	}

	if c.Executor.DefaultTimeoutSeconds <= 0 {
		return fmt.Errorf("executor default timeout must be positive")
	}

	if c.Executor.PythonImage == "" {
		return fmt.Errorf("executor Python image must be specified")
	}

	if c.Executor.BashImage == "" {
		return fmt.Errorf("executor Bash image must be specified")
	}

	// Redis validation
	if c.Redis.Host == "" {
		return fmt.Errorf("Redis host is required")
	}

	if c.Redis.Port == "" {
		return fmt.Errorf("Redis port is required")
	}

	if c.Redis.PoolSize <= 0 {
		return fmt.Errorf("Redis pool size must be positive")
	}

	if c.Redis.MinIdleConnections < 0 {
		return fmt.Errorf("Redis min idle connections must be non-negative")
	}

	if c.Redis.MaxRetries < 0 {
		return fmt.Errorf("Redis max retries must be non-negative")
	}

	// Queue validation
	if c.Queue.TaskQueueName == "" {
		return fmt.Errorf("task queue name is required")
	}

	if c.Queue.DeadLetterQueueName == "" {
		return fmt.Errorf("dead letter queue name is required")
	}

	if c.Queue.RetryQueueName == "" {
		return fmt.Errorf("retry queue name is required")
	}

	if c.Queue.DefaultPriority < 0 || c.Queue.DefaultPriority > 10 {
		return fmt.Errorf("default priority must be between 0 and 10")
	}

	if c.Queue.MaxRetries < 0 {
		return fmt.Errorf("max retries must be non-negative")
	}

	if c.Queue.RetryBackoffFactor <= 1.0 {
		return fmt.Errorf("retry backoff factor must be greater than 1.0")
	}

	if c.Queue.BatchSize <= 0 {
		return fmt.Errorf("batch size must be positive")
	}

	// Worker validation
	if c.Worker.PoolSize <= 0 {
		return fmt.Errorf("worker pool size must be positive")
	}

	if c.Worker.MaxConcurrentTasks <= 0 {
		return fmt.Errorf("max concurrent tasks must be positive")
	}

	if c.Worker.MaxUserConcurrentTasks <= 0 {
		return fmt.Errorf("max user concurrent tasks must be positive")
	}

	if c.Worker.TaskTimeout <= 0 {
		return fmt.Errorf("worker task timeout must be positive")
	}

	if c.Worker.HeartbeatInterval <= 0 {
		return fmt.Errorf("worker heartbeat interval must be positive")
	}

	if c.Worker.ShutdownTimeout <= 0 {
		return fmt.Errorf("worker shutdown timeout must be positive")
	}

	if c.Worker.CleanupInterval <= 0 {
		return fmt.Errorf("worker cleanup interval must be positive")
	}

	if c.Worker.StaleTaskThreshold <= 0 {
		return fmt.Errorf("worker stale task threshold must be positive")
	}

	if c.Worker.WorkerIDPrefix == "" {
		return fmt.Errorf("worker ID prefix is required")
	}

	// Embedded workers validation
	if c.EmbeddedWorkers {
		// When embedded workers are enabled, Redis and Queue must be properly configured
		// since workers need the queue system to process tasks
		if c.Redis.Host == "" {
			return fmt.Errorf("embedded workers require Redis host to be configured")
		}
		if c.Queue.TaskQueueName == "" {
			return fmt.Errorf("embedded workers require task queue name to be configured")
		}
	}

	return nil
}

func (c *Config) IsProduction() bool {
	return strings.ToLower(c.Server.Env) == "production"
}

func (c *Config) IsDevelopment() bool {
	return strings.ToLower(c.Server.Env) == "development"
}

func (c *Config) IsTest() bool {
	return strings.ToLower(c.Server.Env) == "test"
}

func (c *Config) HasEmbeddedWorkers() bool {
	return c.EmbeddedWorkers
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvSlice(key string, defaultValue []string) []string {
	if value := os.Getenv(key); value != "" {
		result := strings.Split(value, ",")
		for i, v := range result {
			result[i] = strings.TrimSpace(v)
		}
		return result
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		duration, err := time.ParseDuration(value)
		if err == nil {
			return duration
		}
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		intValue, err := strconv.Atoi(value)
		if err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvInt64(key string, defaultValue int64) int64 {
	if value := os.Getenv(key); value != "" {
		intValue, err := strconv.ParseInt(value, 10, 64)
		if err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		boolValue, err := strconv.ParseBool(value)
		if err == nil {
			return boolValue
		}
	}
	return defaultValue
}

func getEnvFloat64(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		floatValue, err := strconv.ParseFloat(value, 64)
		if err == nil {
			return floatValue
		}
	}
	return defaultValue
}

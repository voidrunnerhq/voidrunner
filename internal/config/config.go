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
	Server   ServerConfig
	Database DatabaseConfig
	Logger   LoggerConfig
	CORS     CORSConfig
	JWT      JWTConfig
	Executor ExecutorConfig
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

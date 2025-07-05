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
	Docker   DockerConfig // Added DockerConfig
}

// DockerConfig holds Docker-specific settings
type DockerConfig struct {
	Host                string        // Optional, client uses DOCKER_HOST from env if empty
	PythonExecutorImage string
	BashExecutorImage   string
	DefaultMemoryMB     int64
	DefaultCPUQuota     int64 // In microseconds (e.g., 50000 for 0.5 CPU)
	DefaultPidsLimit    int64
	SeccompProfilePath  string // Path on the host or where daemon can access
	DefaultExecTimeout  time.Duration
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
		Docker: DockerConfig{
			Host:                getEnv("DOCKER_HOST", ""), // Let Docker client handle default from env
			PythonExecutorImage: getEnv("PYTHON_EXECUTOR_IMAGE", "voidrunner/python-executor:v1.0"),
			BashExecutorImage:   getEnv("BASH_EXECUTOR_IMAGE", "voidrunner/bash-executor:v1.0"),
			DefaultMemoryMB:     getEnvInt64("DEFAULT_MEMORY_MB", 128),
			DefaultCPUQuota:     getEnvInt64("DEFAULT_CPU_QUOTA", 50000),    // 0.5 CPU
			DefaultPidsLimit:    getEnvInt64("DEFAULT_PIDS_LIMIT", 128),
			SeccompProfilePath:  getEnv("SECCOMP_PROFILE_PATH", "/opt/voidrunner/seccomp-profile.json"), // Path on host
			DefaultExecTimeout:  getEnvDuration("DEFAULT_EXEC_TIMEOUT", 60*time.Second),
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

	// Docker Config Validation
	if c.Docker.PythonExecutorImage == "" {
		return fmt.Errorf("docker python executor image is required")
	}
	if c.Docker.BashExecutorImage == "" {
		return fmt.Errorf("docker bash executor image is required")
	}
	if c.Docker.DefaultMemoryMB <= 0 {
		return fmt.Errorf("docker default memory MB must be positive")
	}
	if c.Docker.DefaultCPUQuota < 0 { // 0 might be valid (no limit), but typically positive or specific -1
		return fmt.Errorf("docker default CPU quota must be non-negative")
	}
	if c.Docker.DefaultPidsLimit <= 0 && c.Docker.DefaultPidsLimit != -1 { // -1 often means unlimited
		return fmt.Errorf("docker default PIDs limit must be positive or -1 (unlimited)")
	}
	if c.Docker.SeccompProfilePath == "" {
		// Allow empty if seccomp is optional or handled differently, but issue implies it's used.
		// For now, let's consider it required if specified in issue.
		// However, Docker defaults to a standard seccomp profile if not overridden.
		// Perhaps warning if empty, or make it truly optional.
		// For now, let's not make it strictly required here, as Docker has defaults.
		// If a specific custom profile is always expected, then make this an error.
	}
	if c.Docker.DefaultExecTimeout <= 0 {
		return fmt.Errorf("docker default exec timeout must be positive")
	}

	return nil
}

func (c *Config) IsProduction() bool {
	return strings.ToLower(c.Server.Env) == "production"
}

func (c *Config) IsDevelopment() bool {
	return strings.ToLower(c.Server.Env) == "development"
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt64(key string, defaultValue int64) int64 {
	if valueStr := os.Getenv(key); valueStr != "" {
		if valueInt, err := strconv.ParseInt(valueStr, 10, 64); err == nil {
			return valueInt
		}
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
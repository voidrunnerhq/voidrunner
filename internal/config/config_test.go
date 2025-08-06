package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad(t *testing.T) {
	t.Run("loads with defaults when no env file", func(t *testing.T) {
		config, err := Load()
		require.NoError(t, err)

		assert.Equal(t, "8080", config.Server.Port)
		assert.Equal(t, "localhost", config.Server.Host)
		assert.Equal(t, "development", config.Server.Env)
		assert.True(t, config.IsDevelopment())
		assert.False(t, config.IsProduction())
	})

	t.Run("loads from environment variables", func(t *testing.T) {
		require.NoError(t, os.Setenv("SERVER_PORT", "9000"))
		require.NoError(t, os.Setenv("SERVER_ENV", "production"))
		defer func() {
			_ = os.Unsetenv("SERVER_PORT")
			_ = os.Unsetenv("SERVER_ENV")
		}()

		config, err := Load()
		require.NoError(t, err)

		assert.Equal(t, "9000", config.Server.Port)
		assert.Equal(t, "production", config.Server.Env)
		assert.True(t, config.IsProduction())
		assert.False(t, config.IsDevelopment())
	})

	t.Run("validates port number", func(t *testing.T) {
		require.NoError(t, os.Setenv("SERVER_PORT", "invalid"))
		defer func() { _ = os.Unsetenv("SERVER_PORT") }()

		_, err := Load()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid server port")
	})

	t.Run("parses CORS origins with spaces", func(t *testing.T) {
		require.NoError(t, os.Setenv("CORS_ALLOWED_ORIGINS", "http://localhost:3000, http://localhost:5173 , https://app.example.com"))
		defer func() { _ = os.Unsetenv("CORS_ALLOWED_ORIGINS") }()

		config, err := Load()
		require.NoError(t, err)

		expected := []string{"http://localhost:3000", "http://localhost:5173", "https://app.example.com"}
		assert.Equal(t, expected, config.CORS.AllowedOrigins)
	})
}

func TestConfigValidation(t *testing.T) {
	t.Run("requires database configuration", func(t *testing.T) {
		config := &Config{
			Server: ServerConfig{Port: "8080"},
			Database: DatabaseConfig{
				Host:     "",
				User:     "postgres",
				Database: "voidrunner",
			},
		}

		err := config.validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "database host is required")
	})
}

func TestLoggingConfigValidation(t *testing.T) {
	t.Run("validates logging configuration fields", func(t *testing.T) {
		tests := []struct {
			name          string
			config        LoggingConfig
			expectedError string
		}{
			{
				name: "valid configuration",
				config: LoggingConfig{
					StreamEnabled:         true,
					BufferSize:            100,
					MaxConcurrentStreams:  10,
					StreamTimeout:         time.Minute,
					BatchInsertSize:       5,
					BatchInsertInterval:   time.Second,
					MaxLogLineSize:        1024,
					RetentionDays:         7,
					CleanupInterval:       time.Hour,
					PartitionCreationDays: 3,
					RedisChannelPrefix:    "test:",
					SubscriberKeepalive:   time.Second * 30,
				},
				expectedError: "",
			},
			{
				name: "invalid buffer size",
				config: LoggingConfig{
					BufferSize:            -1, // Invalid
					MaxConcurrentStreams:  10,
					StreamTimeout:         time.Minute,
					BatchInsertSize:       5,
					BatchInsertInterval:   time.Second,
					MaxLogLineSize:        1024,
					RetentionDays:         7,
					CleanupInterval:       time.Hour,
					PartitionCreationDays: 3,
					RedisChannelPrefix:    "test:",
					SubscriberKeepalive:   time.Second * 30,
				},
				expectedError: "buffer_size must be positive",
			},
			{
				name: "invalid stream timeout",
				config: LoggingConfig{
					BufferSize:            100,
					MaxConcurrentStreams:  10,
					StreamTimeout:         -1, // Invalid
					BatchInsertSize:       5,
					BatchInsertInterval:   time.Second,
					MaxLogLineSize:        1024,
					RetentionDays:         7,
					CleanupInterval:       time.Hour,
					PartitionCreationDays: 3,
					RedisChannelPrefix:    "test:",
					SubscriberKeepalive:   time.Second * 30,
				},
				expectedError: "stream_timeout must be positive",
			},
			{
				name: "empty redis channel prefix",
				config: LoggingConfig{
					BufferSize:            100,
					MaxConcurrentStreams:  10,
					StreamTimeout:         time.Minute,
					BatchInsertSize:       5,
					BatchInsertInterval:   time.Second,
					MaxLogLineSize:        1024,
					RetentionDays:         7,
					CleanupInterval:       time.Hour,
					PartitionCreationDays: 3,
					RedisChannelPrefix:    "", // Invalid
					SubscriberKeepalive:   time.Second * 30,
				},
				expectedError: "redis_channel_prefix cannot be empty",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := tt.config.Validate()
				if tt.expectedError == "" {
					assert.NoError(t, err, "Expected no error for valid config")
				} else {
					assert.Error(t, err, "Expected error for invalid config")
					assert.Contains(t, err.Error(), tt.expectedError)
				}
			})
		}
	})

	t.Run("validates with environment variables", func(t *testing.T) {
		// Test loading config with logging environment variables
		require.NoError(t, os.Setenv("LOG_STREAM_ENABLED", "true"))
		require.NoError(t, os.Setenv("LOG_BUFFER_SIZE", "200"))
		require.NoError(t, os.Setenv("LOG_MAX_CONCURRENT_STREAMS", "20"))
		defer func() {
			_ = os.Unsetenv("LOG_STREAM_ENABLED")
			_ = os.Unsetenv("LOG_BUFFER_SIZE")
			_ = os.Unsetenv("LOG_MAX_CONCURRENT_STREAMS")
		}()

		config, err := Load()
		require.NoError(t, err)

		assert.True(t, config.Logging.StreamEnabled)
		assert.Equal(t, 200, config.Logging.BufferSize)
		assert.Equal(t, 20, config.Logging.MaxConcurrentStreams)
	})

	t.Run("handles logging disabled scenario", func(t *testing.T) {
		require.NoError(t, os.Setenv("LOG_STREAM_ENABLED", "false"))
		defer func() {
			_ = os.Unsetenv("LOG_STREAM_ENABLED")
		}()

		config, err := Load()
		require.NoError(t, err)

		assert.False(t, config.Logging.StreamEnabled)
		// Other logging config should still be valid for when it's re-enabled
		assert.Greater(t, config.Logging.BufferSize, 0)
		assert.Greater(t, config.Logging.MaxConcurrentStreams, 0)
	})
}

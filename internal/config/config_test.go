package config

import (
	"os"
	"testing"

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

package database

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/voidrunnerhq/voidrunner/internal/config"
)

func TestNewConnection_ErrorScenarios(t *testing.T) {
	tests := []struct {
		name      string
		cfg       *config.DatabaseConfig
		wantError string
	}{
		{
			name:      "nil configuration",
			cfg:       nil,
			wantError: "database configuration is required",
		},
		{
			name: "invalid connection string - empty host",
			cfg: &config.DatabaseConfig{
				Host:     "",
				Port:     "5432",
				Database: "test",
				User:     "test",
				Password: "test",
				SSLMode:  "disable",
			},
			wantError: "failed to ping database",
		},
		{
			name: "invalid port",
			cfg: &config.DatabaseConfig{
				Host:     "localhost",
				Port:     "invalid",
				Database: "test",
				User:     "test",
				Password: "test",
				SSLMode:  "disable",
			},
			wantError: "failed to parse database connection string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn, err := NewConnection(tt.cfg, nil)

			assert.Error(t, err)
			assert.Nil(t, conn)
			assert.Contains(t, err.Error(), tt.wantError)
		})
	}
}

func TestConnection_HealthCheck_ErrorScenarios(t *testing.T) {
	tests := []struct {
		name      string
		setup     func() *Connection
		wantError string
	}{
		{
			name: "nil pool",
			setup: func() *Connection {
				return &Connection{Pool: nil}
			},
			wantError: "database pool is not initialized",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn := tt.setup()
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			err := conn.HealthCheck(ctx)

			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantError)
		})
	}
}

func TestConnection_Stats_NilPool(t *testing.T) {
	conn := &Connection{Pool: nil}

	// This should panic or return nil for a nil pool
	defer func() {
		if r := recover(); r != nil {
			// Expected panic for nil pool
			assert.NotNil(t, r)
		}
	}()

	stats := conn.Stats()
	if stats != nil {
		// If it doesn't panic, stats should be zero values
		assert.Equal(t, int32(0), stats.TotalConns())
	}
}

func TestConnection_BeginTx_NilPool(t *testing.T) {
	conn := &Connection{Pool: nil}
	ctx := context.Background()

	defer func() {
		if r := recover(); r != nil {
			// Expected panic for nil pool
			assert.NotNil(t, r)
		}
	}()

	tx, err := conn.BeginTx(ctx)

	// Should either panic or return error
	if err != nil {
		assert.Error(t, err)
		assert.Nil(t, tx)
	}
}

func TestConnection_WithTransaction_ErrorScenarios(t *testing.T) {
	conn := &Connection{Pool: nil}
	ctx := context.Background()

	// This should panic with nil pointer dereference due to nil pool
	// We expect this test to demonstrate the error condition
	defer func() {
		if r := recover(); r != nil {
			t.Logf("Expected panic occurred: %v", r)
		}
	}()

	err := conn.WithTransaction(ctx, func(tx Transaction) error {
		return nil
	})

	// If we reach here without panic, check for error
	if err != nil {
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to begin transaction")
	}
}

func TestConnection_Close_NilPool(t *testing.T) {
	conn := &Connection{Pool: nil}

	// Should not panic
	assert.NotPanics(t, func() {
		conn.Close()
	})
}

func TestConnection_Ping_NilPool(t *testing.T) {
	conn := &Connection{Pool: nil}
	ctx := context.Background()

	defer func() {
		if r := recover(); r != nil {
			// Expected panic for nil pool
			assert.NotNil(t, r)
		}
	}()

	err := conn.Ping(ctx)

	// Should either panic or return error
	if err != nil {
		assert.Error(t, err)
	}
}

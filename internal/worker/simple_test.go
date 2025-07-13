package worker

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// Simple test to provide basic coverage for worker package types
func TestWorkerTypes(t *testing.T) {
	// Test ProcessingSlot creation
	slot := ProcessingSlot{
		ID:         uuid.New(),
		UserID:     uuid.New(),
		TaskID:     uuid.New(),
		WorkerID:   "worker-1",
		AcquiredAt: time.Now(),
		LastActive: time.Now(),
	}

	assert.NotEqual(t, uuid.Nil, slot.ID)
	assert.NotEqual(t, uuid.Nil, slot.UserID)
	assert.NotEqual(t, uuid.Nil, slot.TaskID)
	assert.Equal(t, "worker-1", slot.WorkerID)
}

func TestConcurrencyStats(t *testing.T) {
	stats := ConcurrencyStats{
		TotalActiveSlots: 5,
		AvailableSlots:   15,
		UserConcurrency:  make(map[string]int),
		LastUpdated:      time.Now(),
	}

	assert.Equal(t, 5, stats.TotalActiveSlots)
	assert.Equal(t, 15, stats.AvailableSlots)
	assert.NotNil(t, stats.UserConcurrency)
}

func TestWorkerError(t *testing.T) {
	err := NewWorkerError("worker-1", "test", assert.AnError, true)
	assert.NotNil(t, err)
	assert.Equal(t, "worker-1", err.WorkerID)
	assert.Equal(t, "test", err.Operation)
	assert.True(t, err.Retryable)
}

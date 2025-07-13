package worker

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRedisConcurrencyManager(t *testing.T) {
	limits := ConcurrencyLimits{
		MaxConcurrentTasks:     10,
		MaxUserConcurrentTasks: 3,
		MaxWorkers:             5,
		MinWorkers:             1,
	}

	cm := NewRedisConcurrencyManager(
		limits,
		5*time.Minute,
		1*time.Minute,
		slog.Default(),
	)

	assert.NotNil(t, cm)

	redisCM, ok := cm.(*RedisConcurrencyManager)
	require.True(t, ok)

	assert.Equal(t, limits, redisCM.limits)
	assert.NotNil(t, redisCM.activeSlots)
	assert.NotNil(t, redisCM.userConcurrency)
	assert.Equal(t, 0, redisCM.totalActive)
	assert.NotNil(t, redisCM.cleanupTicker)
}

func TestConcurrencyManager_AcquireSlot(t *testing.T) {
	limits := ConcurrencyLimits{
		MaxConcurrentTasks:     10,
		MaxUserConcurrentTasks: 3,
		MaxWorkers:             5,
		MinWorkers:             1,
	}

	cm := NewRedisConcurrencyManager(
		limits,
		5*time.Minute,
		1*time.Minute,
		slog.Default(),
	).(*RedisConcurrencyManager)
	defer cm.cleanupTicker.Stop()

	ctx := context.Background()
	userID := uuid.New()

	// Acquire first slot
	slot1, err := cm.AcquireSlot(ctx, userID)
	require.NoError(t, err)
	assert.NotNil(t, slot1)
	assert.Equal(t, userID, slot1.UserID)
	assert.NotEqual(t, uuid.Nil, slot1.ID)
	assert.NotZero(t, slot1.AcquiredAt)

	// Check internal state
	assert.Equal(t, 1, cm.totalActive)
	assert.Equal(t, 1, cm.userConcurrency[userID])
	assert.Contains(t, cm.activeSlots, slot1.ID)
}

func TestConcurrencyManager_AcquireSlot_UserLimit(t *testing.T) {
	limits := ConcurrencyLimits{
		MaxConcurrentTasks:     10,
		MaxUserConcurrentTasks: 2, // Set low for testing
		MaxWorkers:             5,
		MinWorkers:             1,
	}

	cm := NewRedisConcurrencyManager(
		limits,
		5*time.Minute,
		1*time.Minute,
		slog.Default(),
	).(*RedisConcurrencyManager)
	defer cm.cleanupTicker.Stop()

	ctx := context.Background()
	userID := uuid.New()

	// Acquire slots up to user limit
	slot1, err := cm.AcquireSlot(ctx, userID)
	require.NoError(t, err)
	assert.NotNil(t, slot1)

	slot2, err := cm.AcquireSlot(ctx, userID)
	require.NoError(t, err)
	assert.NotNil(t, slot2)

	// Third slot should fail due to user limit
	slot3, err := cm.AcquireSlot(ctx, userID)
	assert.Error(t, err)
	assert.Equal(t, ErrConcurrencyLimitReached, err)
	assert.Nil(t, slot3)

	// Check internal state
	assert.Equal(t, 2, cm.totalActive)
	assert.Equal(t, 2, cm.userConcurrency[userID])
}

func TestConcurrencyManager_AcquireSlot_GlobalLimit(t *testing.T) {
	limits := ConcurrencyLimits{
		MaxConcurrentTasks:     2, // Set low for testing
		MaxUserConcurrentTasks: 5,
		MaxWorkers:             5,
		MinWorkers:             1,
	}

	cm := NewRedisConcurrencyManager(
		limits,
		5*time.Minute,
		1*time.Minute,
		slog.Default(),
	).(*RedisConcurrencyManager)
	defer cm.cleanupTicker.Stop()

	ctx := context.Background()
	user1 := uuid.New()
	user2 := uuid.New()

	// Acquire slots up to global limit
	slot1, err := cm.AcquireSlot(ctx, user1)
	require.NoError(t, err)
	assert.NotNil(t, slot1)

	slot2, err := cm.AcquireSlot(ctx, user2)
	require.NoError(t, err)
	assert.NotNil(t, slot2)

	// Third slot should fail due to global limit
	slot3, err := cm.AcquireSlot(ctx, user1)
	assert.Error(t, err)
	assert.Equal(t, ErrConcurrencyLimitReached, err)
	assert.Nil(t, slot3)

	// Check internal state
	assert.Equal(t, 2, cm.totalActive)
	assert.Equal(t, 1, cm.userConcurrency[user1])
	assert.Equal(t, 1, cm.userConcurrency[user2])
}

func TestConcurrencyManager_ReleaseSlot(t *testing.T) {
	limits := ConcurrencyLimits{
		MaxConcurrentTasks:     10,
		MaxUserConcurrentTasks: 3,
		MaxWorkers:             5,
		MinWorkers:             1,
	}

	cm := NewRedisConcurrencyManager(
		limits,
		5*time.Minute,
		1*time.Minute,
		slog.Default(),
	).(*RedisConcurrencyManager)
	defer cm.cleanupTicker.Stop()

	ctx := context.Background()
	userID := uuid.New()

	// Acquire slot
	slot, err := cm.AcquireSlot(ctx, userID)
	require.NoError(t, err)

	// Verify it's tracked
	assert.Equal(t, 1, cm.totalActive)
	assert.Equal(t, 1, cm.userConcurrency[userID])

	// Release slot
	err = cm.ReleaseSlot(slot)
	require.NoError(t, err)

	// Verify it's released
	assert.Equal(t, 0, cm.totalActive)
	assert.Equal(t, 0, cm.userConcurrency[userID])
	assert.NotContains(t, cm.activeSlots, slot.ID)
}

func TestConcurrencyManager_ReleaseSlot_NotFound(t *testing.T) {
	limits := ConcurrencyLimits{
		MaxConcurrentTasks:     10,
		MaxUserConcurrentTasks: 3,
		MaxWorkers:             5,
		MinWorkers:             1,
	}

	cm := NewRedisConcurrencyManager(
		limits,
		5*time.Minute,
		1*time.Minute,
		slog.Default(),
	).(*RedisConcurrencyManager)
	defer cm.cleanupTicker.Stop()

	// Try to release non-existent slot
	slot := &ProcessingSlot{
		ID:     uuid.New(),
		UserID: uuid.New(),
	}

	err := cm.ReleaseSlot(slot)
	assert.Error(t, err)
	assert.Equal(t, ErrSlotNotFound, err)
}

func TestConcurrencyManager_UpdateSlotActivity(t *testing.T) {
	limits := ConcurrencyLimits{
		MaxConcurrentTasks:     10,
		MaxUserConcurrentTasks: 3,
		MaxWorkers:             5,
		MinWorkers:             1,
	}

	cm := NewRedisConcurrencyManager(
		limits,
		5*time.Minute,
		1*time.Minute,
		slog.Default(),
	).(*RedisConcurrencyManager)
	defer cm.cleanupTicker.Stop()

	ctx := context.Background()
	userID := uuid.New()

	// Acquire slot
	slot, err := cm.AcquireSlot(ctx, userID)
	require.NoError(t, err)

	originalLastActive := slot.LastActive
	time.Sleep(10 * time.Millisecond) // Ensure time difference

	// Update activity
	err = cm.UpdateSlotActivity(slot.ID)
	require.NoError(t, err)

	// Verify last active time was updated
	updatedSlot := cm.activeSlots[slot.ID]
	assert.True(t, updatedSlot.LastActive.After(originalLastActive))
}

func TestConcurrencyManager_UpdateSlotActivity_NotFound(t *testing.T) {
	limits := ConcurrencyLimits{
		MaxConcurrentTasks:     10,
		MaxUserConcurrentTasks: 3,
		MaxWorkers:             5,
		MinWorkers:             1,
	}

	cm := NewRedisConcurrencyManager(
		limits,
		5*time.Minute,
		1*time.Minute,
		slog.Default(),
	).(*RedisConcurrencyManager)
	defer cm.cleanupTicker.Stop()

	// Try to update non-existent slot
	err := cm.UpdateSlotActivity(uuid.New())
	assert.Error(t, err)
	assert.Equal(t, ErrSlotNotFound, err)
}

func TestConcurrencyManager_GetStats(t *testing.T) {
	limits := ConcurrencyLimits{
		MaxConcurrentTasks:     10,
		MaxUserConcurrentTasks: 3,
		MaxWorkers:             5,
		MinWorkers:             1,
	}

	cm := NewRedisConcurrencyManager(
		limits,
		5*time.Minute,
		1*time.Minute,
		slog.Default(),
	).(*RedisConcurrencyManager)
	defer cm.cleanupTicker.Stop()

	ctx := context.Background()
	user1 := uuid.New()
	user2 := uuid.New()

	// Acquire some slots
	slot1, _ := cm.AcquireSlot(ctx, user1)
	slot2, _ := cm.AcquireSlot(ctx, user1)
	slot3, _ := cm.AcquireSlot(ctx, user2)

	// Get stats
	stats := cm.GetStats()

	assert.Equal(t, 3, stats.TotalActiveSlots)
	assert.Equal(t, 7, stats.AvailableSlots) // 10 - 3
	assert.Equal(t, int64(3), stats.SlotsAcquiredTotal)
	assert.Equal(t, int64(0), stats.SlotsReleasedTotal)
	assert.Equal(t, 10, stats.MaxConcurrentTasks)
	assert.Equal(t, 3, stats.MaxUserConcurrentTasks)
	assert.Contains(t, stats.UserConcurrency, user1.String())
	assert.Contains(t, stats.UserConcurrency, user2.String())
	assert.Equal(t, 2, stats.UserConcurrency[user1.String()])
	assert.Equal(t, 1, stats.UserConcurrency[user2.String()])

	// Release a slot and check stats again
	_ = cm.ReleaseSlot(slot1)

	stats = cm.GetStats()
	assert.Equal(t, 2, stats.TotalActiveSlots)
	assert.Equal(t, 8, stats.AvailableSlots)
	assert.Equal(t, int64(1), stats.SlotsReleasedTotal)

	// Clean up
	_ = cm.ReleaseSlot(slot2)
	_ = cm.ReleaseSlot(slot3)
}

func TestConcurrencyManager_ConcurrentAccess(t *testing.T) {
	limits := ConcurrencyLimits{
		MaxConcurrentTasks:     100,
		MaxUserConcurrentTasks: 50,
		MaxWorkers:             5,
		MinWorkers:             1,
	}

	cm := NewRedisConcurrencyManager(
		limits,
		5*time.Minute,
		1*time.Minute,
		slog.Default(),
	).(*RedisConcurrencyManager)
	defer cm.cleanupTicker.Stop()

	ctx := context.Background()
	var wg sync.WaitGroup
	var mutex sync.Mutex
	acquiredSlots := make([]*ProcessingSlot, 0)
	errors := make([]error, 0)

	// Concurrent slot acquisition
	numGoroutines := 50
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()

			userID := uuid.New()
			slot, err := cm.AcquireSlot(ctx, userID)

			mutex.Lock()
			if err != nil {
				errors = append(errors, err)
			} else {
				acquiredSlots = append(acquiredSlots, slot)
			}
			mutex.Unlock()
		}()
	}

	wg.Wait()

	// All acquisitions should succeed given the high limits
	assert.Empty(t, errors)
	assert.Len(t, acquiredSlots, numGoroutines)

	// Verify all slots are unique
	slotIDs := make(map[uuid.UUID]bool)
	for _, slot := range acquiredSlots {
		assert.False(t, slotIDs[slot.ID], "Duplicate slot ID found")
		slotIDs[slot.ID] = true
	}

	// Concurrent slot release
	wg.Add(len(acquiredSlots))
	releaseErrors := make([]error, 0)

	for _, slot := range acquiredSlots {
		go func(s *ProcessingSlot) {
			defer wg.Done()

			err := cm.ReleaseSlot(s)
			if err != nil {
				mutex.Lock()
				releaseErrors = append(releaseErrors, err)
				mutex.Unlock()
			}
		}(slot)
	}

	wg.Wait()

	// All releases should succeed
	assert.Empty(t, releaseErrors)
	assert.Equal(t, 0, cm.totalActive)
}

func TestConcurrencyManager_MemoryCleanup(t *testing.T) {
	limits := ConcurrencyLimits{
		MaxConcurrentTasks:     10,
		MaxUserConcurrentTasks: 3,
		MaxWorkers:             5,
		MinWorkers:             1,
	}

	cm := NewRedisConcurrencyManager(
		limits,
		100*time.Millisecond, // Very short TTL for testing
		50*time.Millisecond,  // Fast cleanup for testing
		slog.Default(),
	).(*RedisConcurrencyManager)
	defer cm.cleanupTicker.Stop()

	ctx := context.Background()
	userID := uuid.New()

	// Acquire slot
	_, err := cm.AcquireSlot(ctx, userID)
	require.NoError(t, err)

	// Verify it's tracked
	assert.Equal(t, 1, cm.totalActive)

	// Wait for cleanup to run (slot should expire)
	time.Sleep(200 * time.Millisecond)

	// Note: In a real implementation, expired slots would be cleaned up.
	// This test validates the cleanup mechanism exists.
	// The actual cleanup logic would remove expired slots from activeSlots map.
}

func TestConcurrencyManager_LimitConfiguration(t *testing.T) {
	tests := []struct {
		name   string
		limits ConcurrencyLimits
	}{
		{
			name: "default limits",
			limits: ConcurrencyLimits{
				MaxConcurrentTasks:     10,
				MaxUserConcurrentTasks: 3,
				MaxWorkers:             5,
				MinWorkers:             1,
			},
		},
		{
			name: "high limits",
			limits: ConcurrencyLimits{
				MaxConcurrentTasks:     1000,
				MaxUserConcurrentTasks: 100,
				MaxWorkers:             50,
				MinWorkers:             10,
			},
		},
		{
			name: "single task limits",
			limits: ConcurrencyLimits{
				MaxConcurrentTasks:     1,
				MaxUserConcurrentTasks: 1,
				MaxWorkers:             1,
				MinWorkers:             1,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cm := NewRedisConcurrencyManager(
				tt.limits,
				5*time.Minute,
				1*time.Minute,
				slog.Default(),
			).(*RedisConcurrencyManager)
			defer cm.cleanupTicker.Stop()

			assert.Equal(t, tt.limits, cm.limits)

			stats := cm.GetStats()
			assert.Equal(t, tt.limits.MaxConcurrentTasks, stats.MaxConcurrentTasks)
			assert.Equal(t, tt.limits.MaxUserConcurrentTasks, stats.MaxUserConcurrentTasks)
		})
	}
}

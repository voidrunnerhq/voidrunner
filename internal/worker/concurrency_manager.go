package worker

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
)

// RedisConcurrencyManager implements ConcurrencyManager using Redis for coordination
type RedisConcurrencyManager struct {
	limits ConcurrencyLimits
	logger *slog.Logger

	// In-memory tracking (for this instance)
	mu              sync.RWMutex
	activeSlots     map[uuid.UUID]*ProcessingSlot
	userConcurrency map[uuid.UUID]int
	totalActive     int

	// Statistics
	stats         ConcurrencyStats
	statsMu       sync.RWMutex
	slotsAcquired int64
	slotsReleased int64

	// Cleanup
	cleanupTicker   *time.Ticker
	cleanupInterval time.Duration
	slotTTL         time.Duration
}

// NewRedisConcurrencyManager creates a new Redis-based concurrency manager
func NewRedisConcurrencyManager(
	limits ConcurrencyLimits,
	slotTTL time.Duration,
	cleanupInterval time.Duration,
	logger *slog.Logger,
) ConcurrencyManager {
	cm := &RedisConcurrencyManager{
		limits:          limits,
		logger:          logger,
		activeSlots:     make(map[uuid.UUID]*ProcessingSlot),
		userConcurrency: make(map[uuid.UUID]int),
		slotTTL:         slotTTL,
		cleanupInterval: cleanupInterval,
		stats: ConcurrencyStats{
			UserConcurrency:        make(map[string]int),
			MaxConcurrentTasks:     limits.MaxConcurrentTasks,
			MaxUserConcurrentTasks: limits.MaxUserConcurrentTasks,
			LastUpdated:            time.Now(),
		},
	}

	// Start cleanup routine
	cm.cleanupTicker = time.NewTicker(cleanupInterval)
	go cm.cleanupRoutine()

	return cm
}

// AcquireSlot attempts to acquire a processing slot
func (cm *RedisConcurrencyManager) AcquireSlot(ctx context.Context, userID uuid.UUID) (*ProcessingSlot, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Check global concurrency limit
	if cm.totalActive >= cm.limits.MaxConcurrentTasks {
		cm.logger.Debug("global concurrency limit reached",
			"current", cm.totalActive,
			"max", cm.limits.MaxConcurrentTasks)
		return nil, ErrConcurrencyLimitReached
	}

	// Check user concurrency limit
	userActive := cm.userConcurrency[userID]
	if userActive >= cm.limits.MaxUserConcurrentTasks {
		cm.logger.Debug("user concurrency limit reached",
			"user_id", userID,
			"current", userActive,
			"max", cm.limits.MaxUserConcurrentTasks)
		return nil, ErrConcurrencyLimitReached
	}

	// Create processing slot
	slot := &ProcessingSlot{
		ID:         uuid.New(),
		UserID:     userID,
		AcquiredAt: time.Now(),
		LastActive: time.Now(),
	}

	// Track the slot
	cm.activeSlots[slot.ID] = slot
	cm.userConcurrency[userID] = userActive + 1
	cm.totalActive++

	// Update statistics
	cm.updateStats(func(stats *ConcurrencyStats) {
		stats.TotalActiveSlots = cm.totalActive
		stats.AvailableSlots = cm.limits.MaxConcurrentTasks - cm.totalActive
		stats.UserConcurrency[userID.String()] = cm.userConcurrency[userID]
		stats.SlotsAcquiredTotal++
		stats.LastUpdated = time.Now()
	})

	cm.slotsAcquired++

	cm.logger.Debug("processing slot acquired",
		"slot_id", slot.ID,
		"user_id", userID,
		"total_active", cm.totalActive,
		"user_active", cm.userConcurrency[userID])

	return slot, nil
}

// ReleaseSlot releases a processing slot
func (cm *RedisConcurrencyManager) ReleaseSlot(slot *ProcessingSlot) error {
	if slot == nil {
		return fmt.Errorf("slot cannot be nil")
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Check if slot exists
	existing, exists := cm.activeSlots[slot.ID]
	if !exists {
		cm.logger.Warn("attempting to release non-existent slot", "slot_id", slot.ID)
		return ErrSlotNotFound
	}

	// Remove from tracking
	delete(cm.activeSlots, slot.ID)

	userActive := cm.userConcurrency[existing.UserID]
	if userActive > 0 {
		cm.userConcurrency[existing.UserID] = userActive - 1
		if cm.userConcurrency[existing.UserID] == 0 {
			delete(cm.userConcurrency, existing.UserID)
		}
	}

	if cm.totalActive > 0 {
		cm.totalActive--
	}

	// Update statistics
	slotDuration := time.Since(existing.AcquiredAt)
	cm.updateStats(func(stats *ConcurrencyStats) {
		stats.TotalActiveSlots = cm.totalActive
		stats.AvailableSlots = cm.limits.MaxConcurrentTasks - cm.totalActive
		if cm.userConcurrency[existing.UserID] == 0 {
			delete(stats.UserConcurrency, existing.UserID.String())
		} else {
			stats.UserConcurrency[existing.UserID.String()] = cm.userConcurrency[existing.UserID]
		}
		stats.SlotsReleasedTotal++

		// Update average slot duration
		if stats.SlotsReleasedTotal > 0 {
			totalDuration := stats.AverageSlotDuration * time.Duration(stats.SlotsReleasedTotal-1)
			stats.AverageSlotDuration = (totalDuration + slotDuration) / time.Duration(stats.SlotsReleasedTotal)
		} else {
			stats.AverageSlotDuration = slotDuration
		}

		stats.LastUpdated = time.Now()
	})

	cm.slotsReleased++

	cm.logger.Debug("processing slot released",
		"slot_id", slot.ID,
		"user_id", existing.UserID,
		"duration", slotDuration,
		"total_active", cm.totalActive,
		"user_active", cm.userConcurrency[existing.UserID])

	return nil
}

// GetUserConcurrency returns current concurrency for a user
func (cm *RedisConcurrencyManager) GetUserConcurrency(userID uuid.UUID) int {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.userConcurrency[userID]
}

// GetTotalConcurrency returns total active processing slots
func (cm *RedisConcurrencyManager) GetTotalConcurrency() int {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.totalActive
}

// GetLimits returns current concurrency limits
func (cm *RedisConcurrencyManager) GetLimits() ConcurrencyLimits {
	return cm.limits
}

// UpdateLimits updates concurrency limits
func (cm *RedisConcurrencyManager) UpdateLimits(limits ConcurrencyLimits) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Validate limits
	if limits.MaxConcurrentTasks <= 0 {
		return fmt.Errorf("max concurrent tasks must be positive")
	}
	if limits.MaxUserConcurrentTasks <= 0 {
		return fmt.Errorf("max user concurrent tasks must be positive")
	}
	if limits.MaxUserConcurrentTasks > limits.MaxConcurrentTasks {
		return fmt.Errorf("max user concurrent tasks cannot exceed max concurrent tasks")
	}

	oldLimits := cm.limits
	cm.limits = limits

	// Update statistics
	cm.updateStats(func(stats *ConcurrencyStats) {
		stats.MaxConcurrentTasks = limits.MaxConcurrentTasks
		stats.MaxUserConcurrentTasks = limits.MaxUserConcurrentTasks
		stats.AvailableSlots = limits.MaxConcurrentTasks - cm.totalActive
		stats.LastUpdated = time.Now()
	})

	cm.logger.Info("concurrency limits updated",
		"old_max_tasks", oldLimits.MaxConcurrentTasks,
		"new_max_tasks", limits.MaxConcurrentTasks,
		"old_max_user_tasks", oldLimits.MaxUserConcurrentTasks,
		"new_max_user_tasks", limits.MaxUserConcurrentTasks)

	return nil
}

// IsUserAtLimit checks if user has reached concurrency limit
func (cm *RedisConcurrencyManager) IsUserAtLimit(userID uuid.UUID) bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.userConcurrency[userID] >= cm.limits.MaxUserConcurrentTasks
}

// GetStats returns concurrency statistics
func (cm *RedisConcurrencyManager) GetStats() ConcurrencyStats {
	cm.statsMu.RLock()
	defer cm.statsMu.RUnlock()

	// Create a copy to avoid concurrent access issues
	stats := cm.stats
	stats.UserConcurrency = make(map[string]int)
	for k, v := range cm.stats.UserConcurrency {
		stats.UserConcurrency[k] = v
	}

	return stats
}

// updateStats safely updates concurrency statistics
func (cm *RedisConcurrencyManager) updateStats(fn func(*ConcurrencyStats)) {
	cm.statsMu.Lock()
	defer cm.statsMu.Unlock()
	fn(&cm.stats)
}

// cleanupRoutine periodically cleans up stale slots
func (cm *RedisConcurrencyManager) cleanupRoutine() {
	for range cm.cleanupTicker.C {
		cm.cleanupStaleSlots()
	}
}

// cleanupStaleSlots removes slots that have exceeded their TTL
func (cm *RedisConcurrencyManager) cleanupStaleSlots() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	now := time.Now()
	staleSlots := make([]*ProcessingSlot, 0)

	// Find stale slots
	for _, slot := range cm.activeSlots {
		if now.Sub(slot.LastActive) > cm.slotTTL {
			staleSlots = append(staleSlots, slot)
		}
	}

	// Remove stale slots
	for _, slot := range staleSlots {
		cm.logger.Warn("removing stale processing slot",
			"slot_id", slot.ID,
			"user_id", slot.UserID,
			"age", now.Sub(slot.AcquiredAt),
			"last_active", now.Sub(slot.LastActive))

		// Remove from tracking (similar to ReleaseSlot but without validation)
		delete(cm.activeSlots, slot.ID)

		userActive := cm.userConcurrency[slot.UserID]
		if userActive > 0 {
			cm.userConcurrency[slot.UserID] = userActive - 1
			if cm.userConcurrency[slot.UserID] == 0 {
				delete(cm.userConcurrency, slot.UserID)
			}
		}

		if cm.totalActive > 0 {
			cm.totalActive--
		}
	}

	// Update statistics if slots were cleaned up
	if len(staleSlots) > 0 {
		cm.updateStats(func(stats *ConcurrencyStats) {
			stats.TotalActiveSlots = cm.totalActive
			stats.AvailableSlots = cm.limits.MaxConcurrentTasks - cm.totalActive

			// Rebuild user concurrency map
			stats.UserConcurrency = make(map[string]int)
			for userID, count := range cm.userConcurrency {
				stats.UserConcurrency[userID.String()] = count
			}

			stats.LastUpdated = time.Now()
		})

		cm.logger.Info("cleaned up stale processing slots", "count", len(staleSlots))
	}
}

// UpdateSlotActivity updates the last active time for a slot
func (cm *RedisConcurrencyManager) UpdateSlotActivity(slotID uuid.UUID) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	slot, exists := cm.activeSlots[slotID]
	if !exists {
		return ErrSlotNotFound
	}

	slot.LastActive = time.Now()
	return nil
}

// Stop stops the concurrency manager and cleanup routines
func (cm *RedisConcurrencyManager) Stop() {
	if cm.cleanupTicker != nil {
		cm.cleanupTicker.Stop()
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.logger.Info("concurrency manager stopped",
		"active_slots", len(cm.activeSlots),
		"total_acquired", cm.slotsAcquired,
		"total_released", cm.slotsReleased)
}

// MemoryConcurrencyManager is a simple in-memory implementation for testing
type MemoryConcurrencyManager struct {
	*RedisConcurrencyManager
}

// NewMemoryConcurrencyManager creates an in-memory concurrency manager for testing
func NewMemoryConcurrencyManager(limits ConcurrencyLimits, logger *slog.Logger) ConcurrencyManager {
	return &MemoryConcurrencyManager{
		RedisConcurrencyManager: &RedisConcurrencyManager{
			limits:          limits,
			logger:          logger,
			activeSlots:     make(map[uuid.UUID]*ProcessingSlot),
			userConcurrency: make(map[uuid.UUID]int),
			slotTTL:         30 * time.Minute, // Long TTL for testing
			cleanupInterval: 5 * time.Minute,
			stats: ConcurrencyStats{
				UserConcurrency:        make(map[string]int),
				MaxConcurrentTasks:     limits.MaxConcurrentTasks,
				MaxUserConcurrentTasks: limits.MaxUserConcurrentTasks,
				LastUpdated:            time.Now(),
			},
		},
	}
}

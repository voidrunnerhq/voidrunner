package executor

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
)

// CleanupManager handles resource cleanup and container management
type CleanupManager struct {
	client     ContainerClient
	logger     *slog.Logger
	mu         sync.RWMutex
	containers map[string]*ContainerInfo
}

// ContainerInfo tracks information about running containers
type ContainerInfo struct {
	ID          string
	TaskID      uuid.UUID
	ExecutionID uuid.UUID
	CreatedAt   time.Time
	StartedAt   *time.Time
	Status      string
	Image       string
}

// NewCleanupManager creates a new cleanup manager
func NewCleanupManager(client ContainerClient, logger *slog.Logger) *CleanupManager {
	if logger == nil {
		logger = slog.Default()
	}

	cm := &CleanupManager{
		client:     client,
		logger:     logger,
		containers: make(map[string]*ContainerInfo),
	}

	// Start periodic cleanup
	cm.startPeriodicCleanup()

	return cm
}

// safeContainerID returns a safe version of container ID for logging
func (cm *CleanupManager) safeContainerID(containerID string) string {
	if len(containerID) <= 12 {
		return containerID
	}
	return containerID[:12]
}

// RegisterContainer registers a container for tracking
func (cm *CleanupManager) RegisterContainer(containerID string, taskID, executionID uuid.UUID, image string) error {
	// Validate input parameters
	if containerID == "" {
		err := fmt.Errorf("cannot register container with empty ID")
		cm.logger.Error(err.Error())
		return err
	}

	// Validate UUIDs
	if taskID == uuid.Nil {
		err := fmt.Errorf("cannot register container with nil task ID")
		cm.logger.Error(err.Error())
		return err
	}

	if executionID == uuid.Nil {
		err := fmt.Errorf("cannot register container with nil execution ID")
		cm.logger.Error(err.Error())
		return err
	}

	// Validate image name
	if image == "" {
		err := fmt.Errorf("cannot register container with empty image name")
		cm.logger.Error(err.Error())
		return err
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Check if container is already registered
	if _, exists := cm.containers[containerID]; exists {
		err := fmt.Errorf("container %s is already registered", cm.safeContainerID(containerID))
		cm.logger.Warn(err.Error())
		return err
	}

	cm.containers[containerID] = &ContainerInfo{
		ID:          containerID,
		TaskID:      taskID,
		ExecutionID: executionID,
		CreatedAt:   time.Now(),
		Status:      "created",
		Image:       image,
	}

	cm.logger.Debug("registered container for tracking",
		"container_id", cm.safeContainerID(containerID),
		"task_id", taskID.String(),
		"execution_id", executionID.String())

	return nil
}

// MarkContainerStarted marks a container as started
func (cm *CleanupManager) MarkContainerStarted(containerID string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if info, exists := cm.containers[containerID]; exists {
		now := time.Now()
		info.StartedAt = &now
		info.Status = "running"
		cm.logger.Debug("marked container as started", "container_id", cm.safeContainerID(containerID))
	}
}

// MarkContainerCompleted marks a container as completed
func (cm *CleanupManager) MarkContainerCompleted(containerID string, status string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if info, exists := cm.containers[containerID]; exists {
		info.Status = status
		cm.logger.Debug("marked container as completed",
			"container_id", cm.safeContainerID(containerID),
			"status", status)
	}
}

// UnregisterContainer removes a container from tracking
func (cm *CleanupManager) UnregisterContainer(containerID string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	delete(cm.containers, containerID)
	cm.logger.Debug("unregistered container", "container_id", cm.safeContainerID(containerID))
}

// CleanupContainer performs cleanup for a specific container
func (cm *CleanupManager) CleanupContainer(ctx context.Context, containerID string, force bool) error {
	logger := cm.logger.With("container_id", cm.safeContainerID(containerID))
	logger.Debug("starting container cleanup", "force", force)

	// First try to stop the container gracefully
	if !force {
		stopCtx, stopCancel := context.WithTimeout(ctx, 10*time.Second)
		defer stopCancel()

		if err := cm.client.StopContainer(stopCtx, containerID, 5*time.Second); err != nil {
			logger.Warn("failed to stop container gracefully", "error", err)
			force = true
		} else {
			logger.Debug("container stopped gracefully")
		}
	}

	// Remove the container
	removeCtx, removeCancel := context.WithTimeout(ctx, 30*time.Second)
	defer removeCancel()

	if err := cm.client.RemoveContainer(removeCtx, containerID, force); err != nil {
		logger.Error("failed to remove container", "error", err)
		return NewContainerError(containerID, "cleanup", "failed to remove container", err)
	}

	// Unregister from tracking
	cm.UnregisterContainer(containerID)

	logger.Info("container cleanup completed successfully")
	return nil
}

// CleanupExecution cleans up all containers associated with an execution
func (cm *CleanupManager) CleanupExecution(ctx context.Context, executionID uuid.UUID) error {
	cm.mu.RLock()
	var containersToCleanup []string
	for containerID, info := range cm.containers {
		if info.ExecutionID == executionID {
			containersToCleanup = append(containersToCleanup, containerID)
		}
	}
	cm.mu.RUnlock()

	if len(containersToCleanup) == 0 {
		cm.logger.Debug("no containers to cleanup for execution", "execution_id", executionID.String())
		return nil
	}

	cm.logger.Info("cleaning up containers for execution",
		"execution_id", executionID.String(),
		"container_count", len(containersToCleanup))

	var lastErr error
	for _, containerID := range containersToCleanup {
		if err := cm.CleanupContainer(ctx, containerID, true); err != nil {
			lastErr = err
			cm.logger.Error("failed to cleanup container in execution cleanup",
				"container_id", cm.safeContainerID(containerID),
				"execution_id", executionID.String(),
				"error", err)
		}
	}

	return lastErr
}

// CleanupTask cleans up all containers associated with a task
func (cm *CleanupManager) CleanupTask(ctx context.Context, taskID uuid.UUID) error {
	cm.mu.RLock()
	var containersToCleanup []string
	for containerID, info := range cm.containers {
		if info.TaskID == taskID {
			containersToCleanup = append(containersToCleanup, containerID)
		}
	}
	cm.mu.RUnlock()

	if len(containersToCleanup) == 0 {
		cm.logger.Debug("no containers to cleanup for task", "task_id", taskID.String())
		return nil
	}

	cm.logger.Info("cleaning up containers for task",
		"task_id", taskID.String(),
		"container_count", len(containersToCleanup))

	var lastErr error
	for _, containerID := range containersToCleanup {
		if err := cm.CleanupContainer(ctx, containerID, true); err != nil {
			lastErr = err
			cm.logger.Error("failed to cleanup container in task cleanup",
				"container_id", cm.safeContainerID(containerID),
				"task_id", taskID.String(),
				"error", err)
		}
	}

	return lastErr
}

// CleanupStaleContainers removes containers that have been running too long
func (cm *CleanupManager) CleanupStaleContainers(ctx context.Context, maxAge time.Duration) error {
	cm.mu.RLock()
	now := time.Now()
	var staleContainers []string

	for containerID, info := range cm.containers {
		age := now.Sub(info.CreatedAt)
		if age > maxAge {
			staleContainers = append(staleContainers, containerID)
		}
	}
	cm.mu.RUnlock()

	if len(staleContainers) == 0 {
		return nil
	}

	cm.logger.Info("cleaning up stale containers",
		"count", len(staleContainers),
		"max_age", maxAge.String())

	var lastErr error
	for _, containerID := range staleContainers {
		if err := cm.CleanupContainer(ctx, containerID, true); err != nil {
			lastErr = err
		}
	}

	return lastErr
}

// CleanupAll removes all tracked containers
func (cm *CleanupManager) CleanupAll(ctx context.Context) error {
	cm.mu.RLock()
	var allContainers []string
	for containerID := range cm.containers {
		allContainers = append(allContainers, containerID)
	}
	cm.mu.RUnlock()

	if len(allContainers) == 0 {
		return nil
	}

	cm.logger.Info("cleaning up all containers", "count", len(allContainers))

	var lastErr error
	for _, containerID := range allContainers {
		if err := cm.CleanupContainer(ctx, containerID, true); err != nil {
			lastErr = err
		}
	}

	return lastErr
}

// GetContainerInfo returns information about a tracked container
func (cm *CleanupManager) GetContainerInfo(containerID string) (*ContainerInfo, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	info, exists := cm.containers[containerID]
	if !exists {
		return nil, false
	}

	// Return a copy to avoid data races
	infoCopy := *info
	return &infoCopy, true
}

// GetTrackedContainers returns all currently tracked containers
func (cm *CleanupManager) GetTrackedContainers() []*ContainerInfo {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	containers := make([]*ContainerInfo, 0, len(cm.containers))
	for _, info := range cm.containers {
		// Add copy to avoid data races
		infoCopy := *info
		containers = append(containers, &infoCopy)
	}

	return containers
}

// GetStats returns cleanup manager statistics
func (cm *CleanupManager) GetStats() CleanupStats {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	stats := CleanupStats{
		TotalTracked: len(cm.containers),
	}

	for _, info := range cm.containers {
		switch info.Status {
		case "created":
			stats.Created++
		case "running":
			stats.Running++
		case "completed":
			stats.Completed++
		case "failed":
			stats.Failed++
		case "stopped":
			stats.Stopped++
		}
	}

	return stats
}

// CleanupStats contains statistics about tracked containers
type CleanupStats struct {
	TotalTracked int `json:"total_tracked"`
	Created      int `json:"created"`
	Running      int `json:"running"`
	Completed    int `json:"completed"`
	Failed       int `json:"failed"`
	Stopped      int `json:"stopped"`
}

// startPeriodicCleanup starts a background goroutine for periodic cleanup
func (cm *CleanupManager) startPeriodicCleanup() {
	go func() {
		ticker := time.NewTicker(5 * time.Minute) // Cleanup every 5 minutes
		defer ticker.Stop()

		for range ticker.C {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)

			// Cleanup containers older than 1 hour
			if err := cm.CleanupStaleContainers(ctx, 1*time.Hour); err != nil {
				cm.logger.Error("periodic stale container cleanup failed", "error", err)
			}

			cancel()
		}
	}()

	cm.logger.Info("started periodic cleanup background task")
}

// Stop stops the cleanup manager and cleans up all resources
func (cm *CleanupManager) Stop(ctx context.Context) error {
	cm.logger.Info("stopping cleanup manager")

	// Cleanup all remaining containers
	if err := cm.CleanupAll(ctx); err != nil {
		cm.logger.Error("failed to cleanup all containers during shutdown", "error", err)
		return err
	}

	cm.logger.Info("cleanup manager stopped successfully")
	return nil
}

// ForceCleanupOrphanedContainers finds and removes VoidRunner containers that aren't tracked
func (cm *CleanupManager) ForceCleanupOrphanedContainers(ctx context.Context) error {
	cm.logger.Info("starting orphaned container cleanup")

	// List all containers
	containers, err := cm.client.ListContainers(ctx, true)
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}

	var orphanedContainers []string
	for _, container := range containers {
		// Check if this is a VoidRunner container
		for _, name := range container.Names {
			if len(name) > 0 && name[0] == '/' {
				name = name[1:] // Remove leading slash
			}

			if len(name) > 10 && name[:10] == "voidrunner" {
				// Check if we're tracking this container
				cm.mu.RLock()
				_, tracked := cm.containers[container.ID]
				cm.mu.RUnlock()

				if !tracked {
					orphanedContainers = append(orphanedContainers, container.ID)
				}
				break
			}
		}
	}

	if len(orphanedContainers) == 0 {
		cm.logger.Debug("no orphaned containers found")
		return nil
	}

	cm.logger.Info("found orphaned containers", "count", len(orphanedContainers))

	var lastErr error
	for _, containerID := range orphanedContainers {
		if err := cm.CleanupContainer(ctx, containerID, true); err != nil {
			lastErr = err
			cm.logger.Error("failed to cleanup orphaned container",
				"container_id", cm.safeContainerID(containerID),
				"error", err)
		}
	}

	return lastErr
}

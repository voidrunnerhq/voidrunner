package executor

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockContainerClientForCleanup extends the mock for cleanup-specific methods
type MockContainerClientForCleanup struct {
	MockContainerClient
}

func (m *MockContainerClientForCleanup) ListContainers(ctx context.Context, all bool) ([]ContainerSummary, error) {
	args := m.Called(ctx, all)
	return args.Get(0).([]ContainerSummary), args.Error(1)
}

func (m *MockContainerClientForCleanup) GetDockerInfo(ctx context.Context) (interface{}, error) {
	args := m.Called(ctx)
	return args.Get(0), args.Error(1)
}

func (m *MockContainerClientForCleanup) GetDockerVersion(ctx context.Context) (interface{}, error) {
	args := m.Called(ctx)
	return args.Get(0), args.Error(1)
}

func (m *MockContainerClientForCleanup) GetContainerInfo(ctx context.Context, containerID string) (interface{}, error) {
	args := m.Called(ctx, containerID)
	return args.Get(0), args.Error(1)
}

func (m *MockContainerClientForCleanup) PullImage(ctx context.Context, imageName string) error {
	args := m.Called(ctx, imageName)
	return args.Error(0)
}

func TestNewCleanupManager(t *testing.T) {
	mockClient := new(MockContainerClientForCleanup)

	cm := NewCleanupManager(mockClient, nil)

	assert.NotNil(t, cm)
	assert.NotNil(t, cm.client)
	assert.NotNil(t, cm.logger)
	assert.NotNil(t, cm.containers)
	assert.Equal(t, 0, len(cm.containers))
}

func TestCleanupManager_RegisterContainer(t *testing.T) {
	mockClient := new(MockContainerClientForCleanup)
	cm := NewCleanupManager(mockClient, nil)

	taskID := uuid.New()
	executionID := uuid.New()
	containerID := "container1234567890111123456789012"
	image := "alpine:latest"

	// Register container
	err := cm.RegisterContainer(containerID, taskID, executionID, image)
	require.NoError(t, err)

	// Verify container was registered
	assert.Equal(t, 1, len(cm.containers))

	info, exists := cm.GetContainerInfo(containerID)
	require.True(t, exists)
	assert.Equal(t, containerID, info.ID)
	assert.Equal(t, taskID, info.TaskID)
	assert.Equal(t, executionID, info.ExecutionID)
	assert.Equal(t, image, info.Image)
	assert.Equal(t, "created", info.Status)
	assert.NotZero(t, info.CreatedAt)
	assert.Nil(t, info.StartedAt)
}

func TestCleanupManager_MarkContainerStarted(t *testing.T) {
	mockClient := new(MockContainerClientForCleanup)
	cm := NewCleanupManager(mockClient, nil)

	taskID := uuid.New()
	executionID := uuid.New()
	containerID := "container1234567890111123456789012"
	image := "alpine:latest"

	// Register and start container
	require.NoError(t, cm.RegisterContainer(containerID, taskID, executionID, image))
	cm.MarkContainerStarted(containerID)

	// Verify container status was updated
	info, exists := cm.GetContainerInfo(containerID)
	require.True(t, exists)
	assert.Equal(t, "running", info.Status)
	assert.NotNil(t, info.StartedAt)
}

func TestCleanupManager_MarkContainerCompleted(t *testing.T) {
	mockClient := new(MockContainerClientForCleanup)
	cm := NewCleanupManager(mockClient, nil)

	taskID := uuid.New()
	executionID := uuid.New()
	containerID := "container1234567890111123456789012"
	image := "alpine:latest"

	// Register and complete container
	require.NoError(t, cm.RegisterContainer(containerID, taskID, executionID, image))
	cm.MarkContainerCompleted(containerID, "completed")

	// Verify container status was updated
	info, exists := cm.GetContainerInfo(containerID)
	require.True(t, exists)
	assert.Equal(t, "completed", info.Status)
}

func TestCleanupManager_UnregisterContainer(t *testing.T) {
	mockClient := new(MockContainerClientForCleanup)
	cm := NewCleanupManager(mockClient, nil)

	taskID := uuid.New()
	executionID := uuid.New()
	containerID := "container1234567890111123456789012"
	image := "alpine:latest"

	// Register and unregister container
	require.NoError(t, cm.RegisterContainer(containerID, taskID, executionID, image))
	assert.Equal(t, 1, len(cm.containers))

	cm.UnregisterContainer(containerID)
	assert.Equal(t, 0, len(cm.containers))

	_, exists := cm.GetContainerInfo(containerID)
	assert.False(t, exists)
}

func TestCleanupManager_CleanupContainer(t *testing.T) {
	mockClient := new(MockContainerClientForCleanup)
	cm := NewCleanupManager(mockClient, nil)

	taskID := uuid.New()
	executionID := uuid.New()
	containerID := "container1234567890111123456789012"
	image := "alpine:latest"

	// Register container
	err := cm.RegisterContainer(containerID, taskID, executionID, image)
	require.NoError(t, err)

	tests := []struct {
		name      string
		force     bool
		mockSetup func()
		expectErr bool
	}{
		{
			name:  "Successful graceful cleanup",
			force: false,
			mockSetup: func() {
				mockClient.On("StopContainer", mock.Anything, containerID, 5*time.Second).Return(nil)
				mockClient.On("RemoveContainer", mock.Anything, containerID, false).Return(nil)
			},
			expectErr: false,
		},
		{
			name:  "Successful forced cleanup",
			force: true,
			mockSetup: func() {
				mockClient.On("RemoveContainer", mock.Anything, containerID, true).Return(nil)
			},
			expectErr: false,
		},
		{
			name:  "Stop fails, force cleanup",
			force: false,
			mockSetup: func() {
				mockClient.On("StopContainer", mock.Anything, containerID, 5*time.Second).Return(errors.New("stop failed"))
				mockClient.On("RemoveContainer", mock.Anything, containerID, true).Return(nil)
			},
			expectErr: false,
		},
		{
			name:  "Remove fails",
			force: false,
			mockSetup: func() {
				mockClient.On("StopContainer", mock.Anything, containerID, 5*time.Second).Return(nil)
				mockClient.On("RemoveContainer", mock.Anything, containerID, false).Return(errors.New("remove failed"))
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset mock for each test
			mockClient.ExpectedCalls = nil
			tt.mockSetup()

			err := cm.CleanupContainer(context.Background(), containerID, tt.force)

			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			mockClient.AssertExpectations(t)
		})
	}
}

func TestCleanupManager_CleanupExecution(t *testing.T) {
	mockClient := new(MockContainerClientForCleanup)
	cm := NewCleanupManager(mockClient, nil)

	taskID1 := uuid.New()
	taskID2 := uuid.New()
	executionID1 := uuid.New()
	executionID2 := uuid.New()

	// Register containers for different executions
	require.NoError(t, cm.RegisterContainer("container12345678901111234567890124567890", taskID1, executionID1, "alpine:latest"))
	require.NoError(t, cm.RegisterContainer("container123456789022", taskID1, executionID1, "alpine:latest"))
	require.NoError(t, cm.RegisterContainer("container123456789033", taskID2, executionID2, "alpine:latest"))

	// Mock cleanup calls for containers belonging to executionID1 (force=true, no StopContainer calls)
	mockClient.On("RemoveContainer", mock.Anything, "container12345678901111234567890124567890", true).Return(nil)
	mockClient.On("RemoveContainer", mock.Anything, "container123456789022", true).Return(nil)

	// Cleanup execution1
	err := cm.CleanupExecution(context.Background(), executionID1)
	assert.NoError(t, err)

	// Verify only container123456789033 remains
	assert.Equal(t, 1, len(cm.containers))
	_, exists := cm.GetContainerInfo("container123456789033")
	assert.True(t, exists)

	mockClient.AssertExpectations(t)
}

func TestCleanupManager_CleanupTask(t *testing.T) {
	mockClient := new(MockContainerClientForCleanup)
	cm := NewCleanupManager(mockClient, nil)

	taskID1 := uuid.New()
	taskID2 := uuid.New()
	executionID1 := uuid.New()
	executionID2 := uuid.New()

	// Register containers for different tasks
	require.NoError(t, cm.RegisterContainer("container12345678901111", taskID1, executionID1, "alpine:latest"))
	require.NoError(t, cm.RegisterContainer("container2", taskID1, executionID2, "alpine:latest"))
	require.NoError(t, cm.RegisterContainer("container3", taskID2, executionID1, "alpine:latest"))

	// Mock cleanup calls for containers belonging to taskID1 (force=true, no StopContainer calls)
	mockClient.On("RemoveContainer", mock.Anything, "container12345678901111", true).Return(nil)
	mockClient.On("RemoveContainer", mock.Anything, "container2", true).Return(nil)

	// Cleanup task1
	err := cm.CleanupTask(context.Background(), taskID1)
	assert.NoError(t, err)

	// Verify only container3 remains
	assert.Equal(t, 1, len(cm.containers))
	_, exists := cm.GetContainerInfo("container3")
	assert.True(t, exists)

	mockClient.AssertExpectations(t)
}

func TestCleanupManager_CleanupStaleContainers(t *testing.T) {
	mockClient := new(MockContainerClientForCleanup)
	cm := NewCleanupManager(mockClient, nil)

	taskID := uuid.New()
	executionID := uuid.New()

	// Register containers with different ages
	require.NoError(t, cm.RegisterContainer("new-container123456789", taskID, executionID, "alpine:latest"))
	require.NoError(t, cm.RegisterContainer("old-container123456789", taskID, executionID, "alpine:latest"))

	// Manually set created time for old container
	cm.mu.Lock()
	cm.containers["old-container123456789"].CreatedAt = time.Now().Add(-2 * time.Hour)
	cm.mu.Unlock()

	// Mock cleanup for old container only (force=true, no StopContainer calls)
	mockClient.On("RemoveContainer", mock.Anything, "old-container123456789", true).Return(nil)

	// Cleanup stale containers (older than 1 hour)
	err := cm.CleanupStaleContainers(context.Background(), 1*time.Hour)
	assert.NoError(t, err)

	// Verify only new container remains
	assert.Equal(t, 1, len(cm.containers))
	_, exists := cm.GetContainerInfo("new-container123456789")
	assert.True(t, exists)
	_, exists = cm.GetContainerInfo("old-container123456789")
	assert.False(t, exists)

	mockClient.AssertExpectations(t)
}

func TestCleanupManager_CleanupAll(t *testing.T) {
	mockClient := new(MockContainerClientForCleanup)
	cm := NewCleanupManager(mockClient, nil)

	taskID := uuid.New()
	executionID := uuid.New()

	// Register multiple containers
	require.NoError(t, cm.RegisterContainer("container12345678901111", taskID, executionID, "alpine:latest"))
	require.NoError(t, cm.RegisterContainer("container2", taskID, executionID, "alpine:latest"))

	// Mock cleanup calls for all containers (force=true, no StopContainer calls)
	mockClient.On("RemoveContainer", mock.Anything, "container12345678901111", true).Return(nil)
	mockClient.On("RemoveContainer", mock.Anything, "container2", true).Return(nil)

	// Cleanup all containers
	err := cm.CleanupAll(context.Background())
	assert.NoError(t, err)

	// Verify all containers are removed
	assert.Equal(t, 0, len(cm.containers))

	mockClient.AssertExpectations(t)
}

func TestCleanupManager_GetTrackedContainers(t *testing.T) {
	mockClient := new(MockContainerClientForCleanup)
	cm := NewCleanupManager(mockClient, nil)

	taskID := uuid.New()
	executionID := uuid.New()

	// Register containers
	require.NoError(t, cm.RegisterContainer("container12345678901111", taskID, executionID, "alpine:latest"))
	require.NoError(t, cm.RegisterContainer("container2", taskID, executionID, "python:3.9"))

	// Get tracked containers
	containers := cm.GetTrackedContainers()

	assert.Equal(t, 2, len(containers))

	// Verify container data is copied (not referenced)
	for _, container := range containers {
		assert.NotNil(t, container)
		assert.True(t, container.ID == "container12345678901111" || container.ID == "container2")
	}
}

func TestCleanupManager_GetStats(t *testing.T) {
	mockClient := new(MockContainerClientForCleanup)
	cm := NewCleanupManager(mockClient, nil)

	taskID := uuid.New()
	executionID := uuid.New()

	// Register containers with different statuses
	require.NoError(t, cm.RegisterContainer("created-container123456789", taskID, executionID, "alpine:latest"))
	require.NoError(t, cm.RegisterContainer("running-container123456789", taskID, executionID, "alpine:latest"))
	require.NoError(t, cm.RegisterContainer("completed-container123456789", taskID, executionID, "alpine:latest"))
	require.NoError(t, cm.RegisterContainer("failed-container123456789", taskID, executionID, "alpine:latest"))

	// Set different statuses
	cm.MarkContainerStarted("running-container123456789")
	cm.MarkContainerCompleted("completed-container123456789", "completed")
	cm.MarkContainerCompleted("failed-container123456789", "failed")

	// Get stats
	stats := cm.GetStats()

	assert.Equal(t, 4, stats.TotalTracked)
	assert.Equal(t, 1, stats.Created)
	assert.Equal(t, 1, stats.Running)
	assert.Equal(t, 1, stats.Completed)
	assert.Equal(t, 1, stats.Failed)
	assert.Equal(t, 0, stats.Stopped)
}

func TestCleanupManager_Stop(t *testing.T) {
	mockClient := new(MockContainerClientForCleanup)
	cm := NewCleanupManager(mockClient, nil)

	taskID := uuid.New()
	executionID := uuid.New()

	// Register containers
	require.NoError(t, cm.RegisterContainer("container12345678901111", taskID, executionID, "alpine:latest"))
	require.NoError(t, cm.RegisterContainer("container2", taskID, executionID, "alpine:latest"))

	// Mock cleanup calls for all containers (force=true, no StopContainer calls)
	mockClient.On("RemoveContainer", mock.Anything, "container12345678901111", true).Return(nil)
	mockClient.On("RemoveContainer", mock.Anything, "container2", true).Return(nil)

	// Stop cleanup manager
	err := cm.Stop(context.Background())
	assert.NoError(t, err)

	// Verify all containers are cleaned up
	assert.Equal(t, 0, len(cm.containers))

	mockClient.AssertExpectations(t)
}

func TestCleanupManager_ForceCleanupOrphanedContainers(t *testing.T) {
	mockClient := new(MockContainerClientForCleanup)
	cm := NewCleanupManager(mockClient, nil)

	// Register a tracked container
	taskID := uuid.New()
	executionID := uuid.New()
	require.NoError(t, cm.RegisterContainer("tracked-container123456789", taskID, executionID, "alpine:latest"))

	// Mock ListContainers to return both tracked and orphaned containers
	containerSummaries := []ContainerSummary{
		{
			ID:    "tracked-container123456789",
			Names: []string{"/voidrunner-tracked"},
			Image: "alpine:latest",
		},
		{
			ID:    "orphaned-container123456789",
			Names: []string{"/voidrunner-orphaned"},
			Image: "alpine:latest",
		},
		{
			ID:    "non-voidrunner-container",
			Names: []string{"/other-container123456789"},
			Image: "alpine:latest",
		},
	}

	mockClient.On("ListContainers", mock.Anything, true).Return(containerSummaries, nil)

	// Mock cleanup for orphaned container only (force=true, no StopContainer calls)
	mockClient.On("RemoveContainer", mock.Anything, "orphaned-container123456789", true).Return(nil)

	// Force cleanup orphaned containers
	err := cm.ForceCleanupOrphanedContainers(context.Background())
	assert.NoError(t, err)

	// Verify tracked container is still registered
	assert.Equal(t, 1, len(cm.containers))
	_, exists := cm.GetContainerInfo("tracked-container123456789")
	assert.True(t, exists)

	mockClient.AssertExpectations(t)
}

func TestCleanupManager_ForceCleanupOrphanedContainers_ListError(t *testing.T) {
	mockClient := new(MockContainerClientForCleanup)
	cm := NewCleanupManager(mockClient, nil)

	// Mock ListContainers to return error
	mockClient.On("ListContainers", mock.Anything, true).Return([]ContainerSummary{}, errors.New("list failed"))

	// Force cleanup orphaned containers
	err := cm.ForceCleanupOrphanedContainers(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to list containers")

	mockClient.AssertExpectations(t)
}

func TestCleanupManager_ForceCleanupOrphanedContainers_NoOrphans(t *testing.T) {
	mockClient := new(MockContainerClientForCleanup)
	cm := NewCleanupManager(mockClient, nil)

	// Mock ListContainers to return no voidrunner containers
	containerSummaries := []ContainerSummary{
		{
			ID:    "other-container123456789",
			Names: []string{"/other-container123456789"},
			Image: "alpine:latest",
		},
	}

	mockClient.On("ListContainers", mock.Anything, true).Return(containerSummaries, nil)

	// Force cleanup orphaned containers
	err := cm.ForceCleanupOrphanedContainers(context.Background())
	assert.NoError(t, err)

	mockClient.AssertExpectations(t)
}

func TestCleanupManager_EmptyOperations(t *testing.T) {
	mockClient := new(MockContainerClientForCleanup)
	cm := NewCleanupManager(mockClient, nil)

	// Test operations with no containers
	err := cm.CleanupExecution(context.Background(), uuid.New())
	assert.NoError(t, err)

	err = cm.CleanupTask(context.Background(), uuid.New())
	assert.NoError(t, err)

	err = cm.CleanupStaleContainers(context.Background(), 1*time.Hour)
	assert.NoError(t, err)

	err = cm.CleanupAll(context.Background())
	assert.NoError(t, err)

	// Verify stats for empty manager
	stats := cm.GetStats()
	assert.Equal(t, 0, stats.TotalTracked)
	assert.Equal(t, 0, stats.Created)
	assert.Equal(t, 0, stats.Running)
	assert.Equal(t, 0, stats.Completed)
	assert.Equal(t, 0, stats.Failed)
	assert.Equal(t, 0, stats.Stopped)

	// Verify empty tracked containers
	containers := cm.GetTrackedContainers()
	assert.Equal(t, 0, len(containers))
}

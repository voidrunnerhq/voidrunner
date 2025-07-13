package worker

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/voidrunnerhq/voidrunner/internal/database"
	"github.com/voidrunnerhq/voidrunner/internal/executor"
	"github.com/voidrunnerhq/voidrunner/internal/queue"
)

// BaseWorkerPool implements WorkerPool interface
type BaseWorkerPool struct {
	// Core components
	queue       queue.TaskQueue
	executor    executor.TaskExecutor
	repos       *database.Repositories
	concurrency ConcurrencyManager
	config      WorkerConfig
	logger      *slog.Logger

	// Worker management
	mu        sync.RWMutex
	workers   []Worker
	isRunning bool
	ctx       context.Context
	cancel    context.CancelFunc

	// Statistics tracking
	stats     WorkerPoolStats
	statsMu   sync.RWMutex
	startedAt time.Time

	// Auto-scaling
	scalingMu        sync.Mutex
	scalingTicker    *time.Ticker
	lastScalingCheck time.Time

	// Health monitoring
	healthTicker   *time.Ticker
	healthCheckMu  sync.Mutex
	unhealthyCount int32
}

// NewWorkerPool creates a new worker pool
func NewWorkerPool(
	queue queue.TaskQueue,
	executor executor.TaskExecutor,
	repos *database.Repositories,
	concurrency ConcurrencyManager,
	config WorkerConfig,
	logger *slog.Logger,
) WorkerPool {
	return &BaseWorkerPool{
		queue:       queue,
		executor:    executor,
		repos:       repos,
		concurrency: concurrency,
		config:      config,
		logger:      logger.With("component", "worker_pool"),
		workers:     make([]Worker, 0),
		stats: WorkerPoolStats{
			StartedAt:   time.Now(),
			LastUpdated: time.Now(),
		},
	}
}

// Start starts all workers in the pool
func (p *BaseWorkerPool) Start(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.isRunning {
		return fmt.Errorf("worker pool is already running")
	}

	p.logger.Info("starting worker pool")

	// Create pool context
	p.ctx, p.cancel = context.WithCancel(ctx)
	p.isRunning = true
	p.startedAt = time.Now()

	// Initialize with minimum number of workers
	minWorkers := p.config.GetMinWorkers()
	if minWorkers <= 0 {
		minWorkers = 1 // At least one worker
	}

	for i := 0; i < minWorkers; i++ {
		if err := p.addWorkerLocked(); err != nil {
			p.logger.Error("failed to add initial worker", "worker_index", i, "error", err)
			// Continue with other workers
		}
	}

	// Start all workers
	for _, worker := range p.workers {
		if err := worker.Start(p.ctx); err != nil {
			p.logger.Error("failed to start worker", "worker_id", worker.GetID(), "error", err)
		}
	}

	// Start monitoring routines
	p.startMonitoring()

	// Update statistics
	p.updateStats()

	p.logger.Info("worker pool started", "initial_workers", len(p.workers))
	return nil
}

// Stop gracefully stops all workers in the pool
func (p *BaseWorkerPool) Stop(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.isRunning {
		return fmt.Errorf("worker pool is not running")
	}

	p.logger.Info("stopping worker pool", "worker_count", len(p.workers))

	// Stop monitoring
	p.stopMonitoring()

	// Signal shutdown
	p.cancel()
	p.isRunning = false

	// Stop all workers concurrently
	var wg sync.WaitGroup
	for _, worker := range p.workers {
		wg.Add(1)
		go func(w Worker) {
			defer wg.Done()
			if err := w.Stop(ctx); err != nil {
				p.logger.Error("failed to stop worker", "worker_id", w.GetID(), "error", err)
			}
		}(worker)
	}

	// Wait for all workers to stop or timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		p.logger.Info("all workers stopped gracefully")
	case <-time.After(p.config.ShutdownTimeout):
		p.logger.Warn("worker pool shutdown timeout reached")
	case <-ctx.Done():
		p.logger.Warn("worker pool shutdown cancelled by context")
	}

	// Clear workers
	p.workers = nil
	p.updateStats()

	return nil
}

// IsRunning returns true if the pool is running
func (p *BaseWorkerPool) IsRunning() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.isRunning
}

// GetWorkerCount returns the number of workers in the pool
func (p *BaseWorkerPool) GetWorkerCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.workers)
}

// GetActiveWorkers returns the number of actively processing workers
func (p *BaseWorkerPool) GetActiveWorkers() int {
	p.mu.RLock()
	defer p.mu.RUnlock()

	activeCount := 0
	for _, worker := range p.workers {
		stats := worker.GetStats()
		if stats.CurrentTask != nil {
			activeCount++
		}
	}

	return activeCount
}

// GetStats returns pool statistics
func (p *BaseWorkerPool) GetStats() WorkerPoolStats {
	p.statsMu.RLock()
	defer p.statsMu.RUnlock()

	// Create a copy and update derived fields
	stats := p.stats
	stats.LastUpdated = time.Now()

	if p.isRunning {
		stats.TotalUptime = time.Since(p.startedAt)
	}

	return stats
}

// IsHealthy checks if the worker pool is healthy
func (p *BaseWorkerPool) IsHealthy() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.isRunning {
		return false
	}

	// Check if we have sufficient healthy workers
	healthyWorkers := 0
	for _, worker := range p.workers {
		if worker.IsHealthy() {
			healthyWorkers++
		}
	}

	// Consider healthy if at least 50% of workers are healthy
	minHealthyWorkers := len(p.workers) / 2
	if minHealthyWorkers == 0 {
		minHealthyWorkers = 1
	}

	return healthyWorkers >= minHealthyWorkers
}

// AddWorker adds a new worker to the pool
func (p *BaseWorkerPool) AddWorker() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.isRunning {
		return ErrWorkerPoolClosed
	}

	return p.addWorkerLocked()
}

// RemoveWorker removes a worker from the pool
func (p *BaseWorkerPool) RemoveWorker() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.isRunning {
		return ErrWorkerPoolClosed
	}

	if len(p.workers) <= p.config.GetMinWorkers() {
		return fmt.Errorf("cannot remove worker: pool at minimum size")
	}

	// Find and remove an idle worker
	for i, worker := range p.workers {
		stats := worker.GetStats()
		if stats.CurrentTask == nil { // Worker is idle
			// Stop the worker
			if err := worker.Stop(p.ctx); err != nil {
				p.logger.Error("failed to stop worker during removal", "worker_id", worker.GetID(), "error", err)
			}

			// Remove from slice
			p.workers = append(p.workers[:i], p.workers[i+1:]...)
			p.updateStats()

			p.logger.Info("worker removed from pool", "worker_id", worker.GetID(), "remaining_workers", len(p.workers))
			return nil
		}
	}

	return fmt.Errorf("no idle workers available for removal")
}

// ScaleUp increases the number of workers
func (p *BaseWorkerPool) ScaleUp(count int) error {
	if count <= 0 {
		return fmt.Errorf("scale up count must be positive")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.isRunning {
		return ErrWorkerPoolClosed
	}

	maxWorkers := p.config.GetMaxWorkers()
	currentCount := len(p.workers)

	// Check if scaling would exceed maximum
	if currentCount+count > maxWorkers {
		count = maxWorkers - currentCount
		if count <= 0 {
			return fmt.Errorf("cannot scale up: already at maximum workers (%d)", maxWorkers)
		}
	}

	p.logger.Info("scaling up worker pool", "current_workers", currentCount, "adding", count)

	successCount := 0
	for i := 0; i < count; i++ {
		if err := p.addWorkerLocked(); err != nil {
			p.logger.Error("failed to add worker during scale up", "error", err)
			continue
		}

		// Start the new worker
		newWorker := p.workers[len(p.workers)-1]
		if err := newWorker.Start(p.ctx); err != nil {
			p.logger.Error("failed to start new worker during scale up", "worker_id", newWorker.GetID(), "error", err)
			// Remove the failed worker
			p.workers = p.workers[:len(p.workers)-1]
			continue
		}

		successCount++
	}

	p.updateStats()

	if successCount == 0 {
		return fmt.Errorf("failed to add any workers during scale up")
	}

	p.logger.Info("worker pool scaled up", "added_workers", successCount, "total_workers", len(p.workers))
	return nil
}

// ScaleDown decreases the number of workers
func (p *BaseWorkerPool) ScaleDown(count int) error {
	if count <= 0 {
		return fmt.Errorf("scale down count must be positive")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.isRunning {
		return ErrWorkerPoolClosed
	}

	minWorkers := p.config.GetMinWorkers()
	currentCount := len(p.workers)

	// Check if scaling would go below minimum
	if currentCount-count < minWorkers {
		count = currentCount - minWorkers
		if count <= 0 {
			return fmt.Errorf("cannot scale down: already at minimum workers (%d)", minWorkers)
		}
	}

	p.logger.Info("scaling down worker pool", "current_workers", currentCount, "removing", count)

	removedCount := 0
	for i := 0; i < count; i++ {
		if err := p.removeIdleWorkerLocked(); err != nil {
			p.logger.Warn("failed to remove worker during scale down", "error", err)
			break // Stop trying if we can't remove idle workers
		}
		removedCount++
	}

	p.updateStats()

	if removedCount == 0 {
		return fmt.Errorf("failed to remove any workers during scale down")
	}

	p.logger.Info("worker pool scaled down", "removed_workers", removedCount, "total_workers", len(p.workers))
	return nil
}

// addWorkerLocked adds a new worker (must be called with lock held)
func (p *BaseWorkerPool) addWorkerLocked() error {
	worker := NewWorker(
		p.queue,
		p.executor,
		p.repos,
		p.concurrency,
		p.config,
		p.logger,
	)

	p.workers = append(p.workers, worker)
	return nil
}

// removeIdleWorkerLocked removes an idle worker (must be called with lock held)
func (p *BaseWorkerPool) removeIdleWorkerLocked() error {
	for i, worker := range p.workers {
		stats := worker.GetStats()
		if stats.CurrentTask == nil { // Worker is idle
			// Stop the worker
			if err := worker.Stop(p.ctx); err != nil {
				p.logger.Error("failed to stop worker during removal", "worker_id", worker.GetID(), "error", err)
			}

			// Remove from slice
			p.workers = append(p.workers[:i], p.workers[i+1:]...)
			p.logger.Debug("idle worker removed", "worker_id", worker.GetID())
			return nil
		}
	}

	return fmt.Errorf("no idle workers available")
}

// updateStats updates pool statistics
func (p *BaseWorkerPool) updateStats() {
	p.statsMu.Lock()
	defer p.statsMu.Unlock()

	poolSize := len(p.workers)
	activeWorkers := 0
	idleWorkers := 0
	unhealthyWorkers := 0

	var totalTasksProcessed, totalTasksSuccessful, totalTasksFailed int64
	var totalProcessingTime time.Duration

	for _, worker := range p.workers {
		stats := worker.GetStats()

		if stats.CurrentTask != nil {
			activeWorkers++
		} else {
			idleWorkers++
		}

		if !stats.IsHealthy {
			unhealthyWorkers++
		}

		totalTasksProcessed += stats.TasksProcessed
		totalTasksSuccessful += stats.TasksSuccessful
		totalTasksFailed += stats.TasksFailed
		totalProcessingTime += stats.TotalProcessingTime
	}

	// Calculate average task time
	var averageTaskTime time.Duration
	if totalTasksProcessed > 0 {
		averageTaskTime = totalProcessingTime / time.Duration(totalTasksProcessed)
	}

	p.stats = WorkerPoolStats{
		PoolSize:             poolSize,
		ActiveWorkers:        activeWorkers,
		IdleWorkers:          idleWorkers,
		UnhealthyWorkers:     unhealthyWorkers,
		TotalTasksProcessed:  totalTasksProcessed,
		TotalTasksSuccessful: totalTasksSuccessful,
		TotalTasksFailed:     totalTasksFailed,
		AverageTaskTime:      averageTaskTime,
		StartedAt:            p.startedAt,
		LastUpdated:          time.Now(),
	}

	if p.isRunning {
		p.stats.TotalUptime = time.Since(p.startedAt)
	}
}

// startMonitoring starts monitoring routines
func (p *BaseWorkerPool) startMonitoring() {
	// Start health monitoring
	p.healthTicker = time.NewTicker(p.config.HealthCheckInterval)
	go p.healthMonitoringLoop()

	// Start auto-scaling if enabled
	if p.config.EnableAutoScaling {
		p.scalingTicker = time.NewTicker(p.config.ScalingCheckInterval)
		go p.autoScalingLoop()
	}

	// Start statistics update routine
	go p.statsUpdateLoop()
}

// stopMonitoring stops monitoring routines
func (p *BaseWorkerPool) stopMonitoring() {
	if p.healthTicker != nil {
		p.healthTicker.Stop()
	}
	if p.scalingTicker != nil {
		p.scalingTicker.Stop()
	}
}

// healthMonitoringLoop monitors worker health
func (p *BaseWorkerPool) healthMonitoringLoop() {
	for {
		select {
		case <-p.ctx.Done():
			return
		case <-p.healthTicker.C:
			p.performHealthCheck()
		}
	}
}

// performHealthCheck checks health of all workers
func (p *BaseWorkerPool) performHealthCheck() {
	p.healthCheckMu.Lock()
	defer p.healthCheckMu.Unlock()

	p.mu.RLock()
	workers := make([]Worker, len(p.workers))
	copy(workers, p.workers)
	p.mu.RUnlock()

	unhealthyCount := int32(0)
	for _, worker := range workers {
		if !worker.IsHealthy() {
			atomic.AddInt32(&unhealthyCount, 1)
		}
	}

	atomic.StoreInt32(&p.unhealthyCount, unhealthyCount)

	// Update statistics
	p.updateStats()
}

// autoScalingLoop handles automatic scaling decisions
func (p *BaseWorkerPool) autoScalingLoop() {
	for {
		select {
		case <-p.ctx.Done():
			return
		case <-p.scalingTicker.C:
			p.performAutoScaling()
		}
	}
}

// performAutoScaling makes scaling decisions based on current load
func (p *BaseWorkerPool) performAutoScaling() {
	p.scalingMu.Lock()
	defer p.scalingMu.Unlock()

	p.lastScalingCheck = time.Now()

	stats := p.GetStats()

	// Simple scaling logic: scale up if all workers are active, scale down if too many idle
	scaleUpThreshold := 0.9   // Scale up if >90% workers active
	scaleDownThreshold := 0.3 // Scale down if <30% workers active

	if stats.PoolSize == 0 {
		return
	}

	activeRatio := float64(stats.ActiveWorkers) / float64(stats.PoolSize)

	p.logger.Debug("auto-scaling check",
		"pool_size", stats.PoolSize,
		"active_workers", stats.ActiveWorkers,
		"active_ratio", activeRatio,
		"unhealthy_workers", stats.UnhealthyWorkers)

	// Scale up if high utilization
	if activeRatio > scaleUpThreshold {
		if err := p.ScaleUp(1); err != nil {
			p.logger.Debug("auto scale up failed", "error", err)
		} else {
			p.logger.Info("auto scaled up", "new_size", p.GetWorkerCount())
		}
		return
	}

	// Scale down if low utilization (but not too frequently)
	if activeRatio < scaleDownThreshold {
		if err := p.ScaleDown(1); err != nil {
			p.logger.Debug("auto scale down failed", "error", err)
		} else {
			p.logger.Info("auto scaled down", "new_size", p.GetWorkerCount())
		}
	}
}

// statsUpdateLoop periodically updates statistics
func (p *BaseWorkerPool) statsUpdateLoop() {
	ticker := time.NewTicker(10 * time.Second) // Update stats every 10 seconds
	defer ticker.Stop()

	for {
		select {
		case <-p.ctx.Done():
			return
		case <-ticker.C:
			p.updateStats()
		}
	}
}

// GetWorkerStats returns statistics for all workers
func (p *BaseWorkerPool) GetWorkerStats() []WorkerStats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	stats := make([]WorkerStats, len(p.workers))
	for i, worker := range p.workers {
		stats[i] = worker.GetStats()
	}

	return stats
}

// Extension methods for WorkerConfig to provide defaults
func (c WorkerConfig) GetMinWorkers() int {
	// This should be added to the config struct, but for now we'll default
	return 1
}

func (c WorkerConfig) GetMaxWorkers() int {
	// This should be added to the config struct, but for now we'll default
	return 10
}

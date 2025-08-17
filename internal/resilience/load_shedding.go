package resilience

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

// LoadSheddingConfig holds configuration for load shedding
type LoadSheddingConfig struct {
	// Maximum number of concurrent requests
	MaxConcurrentRequests int `json:"max_concurrent_requests"`
	
	// CPU threshold to start shedding load (percentage 0-100)
	CPUThreshold float64 `json:"cpu_threshold"`
	
	// Memory threshold to start shedding load (percentage 0-100)
	MemoryThreshold float64 `json:"memory_threshold"`
	
	// Queue size threshold to start shedding load
	QueueSizeThreshold int `json:"queue_size_threshold"`
	
	// Error rate threshold to start shedding load (percentage 0-100)
	ErrorRateThreshold float64 `json:"error_rate_threshold"`
	
	// Check interval for resource monitoring
	CheckInterval time.Duration `json:"check_interval"`
	
	// Shedding percentage when thresholds are exceeded (0-100)
	SheddingPercentage float64 `json:"shedding_percentage"`
	
	// Priority levels for request handling
	EnablePriorityQueues bool `json:"enable_priority_queues"`
}

// DefaultLoadSheddingConfig returns sensible defaults
func DefaultLoadSheddingConfig() *LoadSheddingConfig {
	return &LoadSheddingConfig{
		MaxConcurrentRequests: 1000,
		CPUThreshold:          80.0,
		MemoryThreshold:       85.0,
		QueueSizeThreshold:    500,
		ErrorRateThreshold:    20.0,
		CheckInterval:         5 * time.Second,
		SheddingPercentage:    50.0,
		EnablePriorityQueues:  true,
	}
}

// Validate checks if the configuration is valid
func (config *LoadSheddingConfig) Validate() error {
	if config.MaxConcurrentRequests <= 0 {
		return fmt.Errorf("max concurrent requests must be positive")
	}
	if config.CPUThreshold < 0 || config.CPUThreshold > 100 {
		return fmt.Errorf("CPU threshold must be between 0 and 100")
	}
	if config.MemoryThreshold < 0 || config.MemoryThreshold > 100 {
		return fmt.Errorf("memory threshold must be between 0 and 100")
	}
	if config.QueueSizeThreshold < 0 {
		return fmt.Errorf("queue size threshold cannot be negative")
	}
	if config.ErrorRateThreshold < 0 || config.ErrorRateThreshold > 100 {
		return fmt.Errorf("error rate threshold must be between 0 and 100")
	}
	if config.CheckInterval <= 0 {
		return fmt.Errorf("check interval must be positive")
	}
	if config.SheddingPercentage < 0 || config.SheddingPercentage > 100 {
		return fmt.Errorf("shedding percentage must be between 0 and 100")
	}
	return nil
}

// RequestPriority represents the priority level of a request
type RequestPriority int

const (
	// PriorityLow represents low-priority requests (first to be shed)
	PriorityLow RequestPriority = iota
	
	// PriorityNormal represents normal-priority requests
	PriorityNormal
	
	// PriorityHigh represents high-priority requests (last to be shed)
	PriorityHigh
	
	// PriorityCritical represents critical requests (never shed)
	PriorityCritical
)

// LoadSheddingDecision represents the decision made by the load shedder
type LoadSheddingDecision struct {
	Allow    bool                   `json:"allow"`
	Reason   string                 `json:"reason,omitempty"`
	Priority RequestPriority        `json:"priority"`
	Context  map[string]interface{} `json:"context,omitempty"`
}

// SystemMetrics represents current system resource usage
type SystemMetrics struct {
	CPUPercent    float64 `json:"cpu_percent"`
	MemoryPercent float64 `json:"memory_percent"`
	QueueSize     int     `json:"queue_size"`
	ErrorRate     float64 `json:"error_rate"`
	Timestamp     time.Time `json:"timestamp"`
}

// LoadSheddingStats holds statistics about load shedding
type LoadSheddingStats struct {
	TotalRequests      int64             `json:"total_requests"`
	AcceptedRequests   int64             `json:"accepted_requests"`
	ShedRequests       int64             `json:"shed_requests"`
	CurrentLoad        int64             `json:"current_load"`
	MaxLoad            int               `json:"max_load"`
	SheddingActive     bool              `json:"shedding_active"`
	SheddingReason     string            `json:"shedding_reason,omitempty"`
	LastSheddingEvent  *time.Time        `json:"last_shedding_event,omitempty"`
	RequestsByPriority map[RequestPriority]int64 `json:"requests_by_priority"`
	ShedByPriority     map[RequestPriority]int64 `json:"shed_by_priority"`
}

// MetricsProvider interface for getting system metrics
type MetricsProvider interface {
	GetCurrentMetrics() *SystemMetrics
}

// LoadShedder implements load shedding based on system resources
type LoadShedder struct {
	mu             sync.RWMutex
	config         *LoadSheddingConfig
	logger         *slog.Logger
	metricsProvider MetricsProvider
	
	// Counters
	currentLoad        int64
	totalRequests      int64
	acceptedRequests   int64
	shedRequests       int64
	requestsByPriority map[RequestPriority]int64
	shedByPriority     map[RequestPriority]int64
	
	// Shedding state
	sheddingActive    bool
	sheddingReason    string
	lastSheddingEvent *time.Time
	
	// Monitoring
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewLoadShedder creates a new load shedder
func NewLoadShedder(config *LoadSheddingConfig, metricsProvider MetricsProvider, logger *slog.Logger) (*LoadShedder, error) {
	if config == nil {
		config = DefaultLoadSheddingConfig()
	}
	
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid load shedding config: %w", err)
	}
	
	if logger == nil {
		logger = slog.Default()
	}

	ctx, cancel := context.WithCancel(context.Background())

	ls := &LoadShedder{
		config:             config,
		logger:             logger.With("component", "load_shedder"),
		metricsProvider:    metricsProvider,
		requestsByPriority: make(map[RequestPriority]int64),
		shedByPriority:     make(map[RequestPriority]int64),
		ctx:                ctx,
		cancel:             cancel,
	}

	// Start monitoring if metrics provider is available
	if metricsProvider != nil {
		ls.startMonitoring()
	}

	return ls, nil
}

// ShouldAcceptRequest determines if a request should be accepted based on current load
func (ls *LoadShedder) ShouldAcceptRequest(priority RequestPriority) LoadSheddingDecision {
	ls.mu.Lock()
	defer ls.mu.Unlock()

	ls.totalRequests++
	ls.requestsByPriority[priority]++

	decision := LoadSheddingDecision{
		Allow:    true,
		Priority: priority,
		Context:  make(map[string]interface{}),
	}

	// Critical requests are never shed
	if priority == PriorityCritical {
		ls.acceptedRequests++
		atomic.AddInt64(&ls.currentLoad, 1)
		return decision
	}

	// Check concurrent request limit
	currentLoad := atomic.LoadInt64(&ls.currentLoad)
	if currentLoad >= int64(ls.config.MaxConcurrentRequests) {
		return ls.shedRequest(priority, "max_concurrent_requests_exceeded", 
			map[string]interface{}{
				"current_load": currentLoad,
				"max_load": ls.config.MaxConcurrentRequests,
			})
	}

	// Check if shedding is active due to resource constraints
	if ls.sheddingActive {
		// Shed based on priority and shedding percentage
		shouldShed := ls.shouldShedBasedOnPriority(priority)
		if shouldShed {
			return ls.shedRequest(priority, ls.sheddingReason, 
				map[string]interface{}{
					"shedding_percentage": ls.config.SheddingPercentage,
				})
		}
	}

	// Accept the request
	ls.acceptedRequests++
	atomic.AddInt64(&ls.currentLoad, 1)
	decision.Context["current_load"] = atomic.LoadInt64(&ls.currentLoad)
	
	return decision
}

// CompleteRequest should be called when a request completes
func (ls *LoadShedder) CompleteRequest() {
	atomic.AddInt64(&ls.currentLoad, -1)
}

// shouldShedBasedOnPriority determines if a request should be shed based on priority
func (ls *LoadShedder) shouldShedBasedOnPriority(priority RequestPriority) bool {
	// Use a simple percentage-based shedding strategy
	// Higher priority requests have lower chance of being shed
	
	sheddingPercentage := ls.config.SheddingPercentage
	
	switch priority {
	case PriorityLow:
		// Shed more low-priority requests
		return ls.shouldShedWithPercentage(sheddingPercentage * 1.5)
	case PriorityNormal:
		// Shed normal percentage
		return ls.shouldShedWithPercentage(sheddingPercentage)
	case PriorityHigh:
		// Shed fewer high-priority requests
		return ls.shouldShedWithPercentage(sheddingPercentage * 0.5)
	case PriorityCritical:
		// Never shed critical requests
		return false
	default:
		return ls.shouldShedWithPercentage(sheddingPercentage)
	}
}

// shouldShedWithPercentage determines if a request should be shed based on percentage
func (ls *LoadShedder) shouldShedWithPercentage(percentage float64) bool {
	if percentage >= 100.0 {
		return true
	}
	if percentage <= 0.0 {
		return false
	}
	
	// Simple random shedding based on percentage
	// In production, you might want a more sophisticated algorithm
	return (ls.totalRequests % 100) < int64(percentage)
}

// shedRequest creates a decision to shed a request
func (ls *LoadShedder) shedRequest(priority RequestPriority, reason string, context map[string]interface{}) LoadSheddingDecision {
	ls.shedRequests++
	ls.shedByPriority[priority]++
	
	now := time.Now()
	ls.lastSheddingEvent = &now

	return LoadSheddingDecision{
		Allow:    false,
		Reason:   reason,
		Priority: priority,
		Context:  context,
	}
}

// startMonitoring starts the background monitoring of system resources
func (ls *LoadShedder) startMonitoring() {
	ls.wg.Add(1)
	go func() {
		defer ls.wg.Done()
		ticker := time.NewTicker(ls.config.CheckInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ls.ctx.Done():
				return
			case <-ticker.C:
				ls.checkSystemResources()
			}
		}
	}()
}

// checkSystemResources checks current system resources and updates shedding state
func (ls *LoadShedder) checkSystemResources() {
	if ls.metricsProvider == nil {
		return
	}

	metrics := ls.metricsProvider.GetCurrentMetrics()
	if metrics == nil {
		return
	}

	ls.mu.Lock()
	defer ls.mu.Unlock()

	previouslyActive := ls.sheddingActive
	ls.sheddingActive = false
	ls.sheddingReason = ""

	// Check each threshold
	if metrics.CPUPercent >= ls.config.CPUThreshold {
		ls.sheddingActive = true
		ls.sheddingReason = fmt.Sprintf("CPU usage %.2f%% exceeds threshold %.2f%%", 
			metrics.CPUPercent, ls.config.CPUThreshold)
	} else if metrics.MemoryPercent >= ls.config.MemoryThreshold {
		ls.sheddingActive = true
		ls.sheddingReason = fmt.Sprintf("Memory usage %.2f%% exceeds threshold %.2f%%", 
			metrics.MemoryPercent, ls.config.MemoryThreshold)
	} else if metrics.QueueSize >= ls.config.QueueSizeThreshold {
		ls.sheddingActive = true
		ls.sheddingReason = fmt.Sprintf("Queue size %d exceeds threshold %d", 
			metrics.QueueSize, ls.config.QueueSizeThreshold)
	} else if metrics.ErrorRate >= ls.config.ErrorRateThreshold {
		ls.sheddingActive = true
		ls.sheddingReason = fmt.Sprintf("Error rate %.2f%% exceeds threshold %.2f%%", 
			metrics.ErrorRate, ls.config.ErrorRateThreshold)
	}

	// Log state changes
	if ls.sheddingActive && !previouslyActive {
		ls.logger.Warn("load shedding activated", "reason", ls.sheddingReason)
	} else if !ls.sheddingActive && previouslyActive {
		ls.logger.Info("load shedding deactivated")
	}
}

// GetStats returns statistics about load shedding
func (ls *LoadShedder) GetStats() LoadSheddingStats {
	ls.mu.RLock()
	defer ls.mu.RUnlock()

	// Create copies of maps to avoid data races
	requestsByPriority := make(map[RequestPriority]int64)
	shedByPriority := make(map[RequestPriority]int64)
	
	for priority, count := range ls.requestsByPriority {
		requestsByPriority[priority] = count
	}
	for priority, count := range ls.shedByPriority {
		shedByPriority[priority] = count
	}

	return LoadSheddingStats{
		TotalRequests:      ls.totalRequests,
		AcceptedRequests:   ls.acceptedRequests,
		ShedRequests:       ls.shedRequests,
		CurrentLoad:        atomic.LoadInt64(&ls.currentLoad),
		MaxLoad:            ls.config.MaxConcurrentRequests,
		SheddingActive:     ls.sheddingActive,
		SheddingReason:     ls.sheddingReason,
		LastSheddingEvent:  ls.lastSheddingEvent,
		RequestsByPriority: requestsByPriority,
		ShedByPriority:     shedByPriority,
	}
}

// IsSheddingActive returns true if load shedding is currently active
func (ls *LoadShedder) IsSheddingActive() bool {
	ls.mu.RLock()
	defer ls.mu.RUnlock()
	return ls.sheddingActive
}

// GetCurrentLoad returns the current number of active requests
func (ls *LoadShedder) GetCurrentLoad() int64 {
	return atomic.LoadInt64(&ls.currentLoad)
}

// Reset resets all statistics and state
func (ls *LoadShedder) Reset() {
	ls.mu.Lock()
	defer ls.mu.Unlock()

	atomic.StoreInt64(&ls.currentLoad, 0)
	ls.totalRequests = 0
	ls.acceptedRequests = 0
	ls.shedRequests = 0
	ls.sheddingActive = false
	ls.sheddingReason = ""
	ls.lastSheddingEvent = nil
	
	// Reset priority counters
	for priority := range ls.requestsByPriority {
		ls.requestsByPriority[priority] = 0
	}
	for priority := range ls.shedByPriority {
		ls.shedByPriority[priority] = 0
	}

	ls.logger.Info("load shedder reset")
}

// Stop stops the load shedder and cleans up resources
func (ls *LoadShedder) Stop() {
	ls.cancel()
	ls.wg.Wait()
	ls.logger.Info("load shedder stopped")
}
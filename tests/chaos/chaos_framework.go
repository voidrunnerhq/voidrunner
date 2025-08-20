//go:build integration

package chaos

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"sync"
	"time"
)

// ChaosExperiment represents a chaos engineering experiment
type ChaosExperiment struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Duration    time.Duration          `json:"duration"`
	Config      map[string]interface{} `json:"config"`
	Setup       func(ctx context.Context) error
	Execute     func(ctx context.Context) error
	Cleanup     func(ctx context.Context) error
	Validate    func(ctx context.Context) error
}

// ChaosResult represents the result of a chaos experiment
type ChaosResult struct {
	ExperimentID string        `json:"experiment_id"`
	StartTime    time.Time     `json:"start_time"`
	EndTime      time.Time     `json:"end_time"`
	Duration     time.Duration `json:"duration"`
	Success      bool          `json:"success"`
	ErrorCount   int           `json:"error_count"`
	RecoveryTime time.Duration `json:"recovery_time"`
	Observations []Observation `json:"observations"`
	Metrics      ChaosMetrics  `json:"metrics"`
	Error        string        `json:"error,omitempty"`
}

// Observation represents an observation during chaos testing
type Observation struct {
	Timestamp time.Time              `json:"timestamp"`
	Type      string                 `json:"type"` // "error", "recovery", "degradation", "failure"
	Severity  string                 `json:"severity"`
	Message   string                 `json:"message"`
	Component string                 `json:"component"`
	Metadata  map[string]interface{} `json:"metadata"`
}

// ChaosMetrics holds metrics collected during chaos experiments
type ChaosMetrics struct {
	ErrorsBeforeFailure  int           `json:"errors_before_failure"`
	ErrorsDuringFailure  int           `json:"errors_during_failure"`
	ErrorsAfterRecovery  int           `json:"errors_after_recovery"`
	CircuitBreakerTrips  int           `json:"circuit_breaker_trips"`
	RetryAttempts        int           `json:"retry_attempts"`
	GracefulDegradations int           `json:"graceful_degradations"`
	SystemRecoveryTime   time.Duration `json:"system_recovery_time"`
	MaxResponseTime      time.Duration `json:"max_response_time"`
	AverageResponseTime  time.Duration `json:"average_response_time"`
	ResourceUsageSpike   float64       `json:"resource_usage_spike"`
}

// ChaosRunner orchestrates chaos experiments
type ChaosRunner struct {
	mu          sync.RWMutex
	logger      *slog.Logger
	experiments map[string]*ChaosExperiment
	results     []*ChaosResult
	observers   []Observer
	isRunning   bool
	stopChan    chan struct{}
}

// Observer defines the interface for chaos experiment observers
type Observer interface {
	OnExperimentStart(experiment *ChaosExperiment)
	OnExperimentEnd(result *ChaosResult)
	OnObservation(observation *Observation)
}

// NewChaosRunner creates a new chaos runner
func NewChaosRunner(logger *slog.Logger) *ChaosRunner {
	if logger == nil {
		logger = slog.Default()
	}

	return &ChaosRunner{
		logger:      logger.With("component", "chaos_runner"),
		experiments: make(map[string]*ChaosExperiment),
		results:     make([]*ChaosResult, 0),
		observers:   make([]Observer, 0),
		stopChan:    make(chan struct{}),
	}
}

// RegisterExperiment registers a chaos experiment
func (cr *ChaosRunner) RegisterExperiment(experiment *ChaosExperiment) {
	cr.mu.Lock()
	defer cr.mu.Unlock()

	cr.experiments[experiment.ID] = experiment
	cr.logger.Info("chaos experiment registered",
		"experiment_id", experiment.ID,
		"name", experiment.Name)
}

// AddObserver adds an observer to the chaos runner
func (cr *ChaosRunner) AddObserver(observer Observer) {
	cr.mu.Lock()
	defer cr.mu.Unlock()

	cr.observers = append(cr.observers, observer)
}

// RunExperiment runs a specific chaos experiment
func (cr *ChaosRunner) RunExperiment(ctx context.Context, experimentID string) (*ChaosResult, error) {
	cr.mu.RLock()
	experiment, exists := cr.experiments[experimentID]
	cr.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("experiment not found: %s", experimentID)
	}

	cr.logger.Info("starting chaos experiment",
		"experiment_id", experimentID,
		"name", experiment.Name,
		"duration", experiment.Duration)

	result := &ChaosResult{
		ExperimentID: experimentID,
		StartTime:    time.Now(),
		Observations: make([]Observation, 0),
		Metrics:      ChaosMetrics{},
	}

	// Notify observers
	for _, observer := range cr.observers {
		observer.OnExperimentStart(experiment)
	}

	// Setup phase
	if experiment.Setup != nil {
		cr.addObservation(result, "setup", "info", "Setting up experiment", "framework", nil)
		if err := experiment.Setup(ctx); err != nil {
			result.Success = false
			result.Error = fmt.Sprintf("setup failed: %v", err)
			result.EndTime = time.Now()
			result.Duration = result.EndTime.Sub(result.StartTime)
			return result, err
		}
	}

	// Execution phase with timeout
	execCtx, cancel := context.WithTimeout(ctx, experiment.Duration)
	defer cancel()

	executionDone := make(chan error, 1)
	go func() {
		cr.addObservation(result, "execution", "info", "Starting experiment execution", "framework", nil)
		executionDone <- experiment.Execute(execCtx)
	}()

	var execErr error
	select {
	case execErr = <-executionDone:
		// Experiment completed
	case <-execCtx.Done():
		// Experiment timed out
		cr.addObservation(result, "timeout", "warning", "Experiment timed out", "framework", nil)
		execErr = execCtx.Err()
	}

	// Cleanup phase
	if experiment.Cleanup != nil {
		cr.addObservation(result, "cleanup", "info", "Cleaning up experiment", "framework", nil)
		if cleanupErr := experiment.Cleanup(ctx); cleanupErr != nil {
			cr.logger.Error("cleanup failed", "error", cleanupErr)
			cr.addObservation(result, "cleanup", "error", fmt.Sprintf("Cleanup failed: %v", cleanupErr), "framework", nil)
		}
	}

	// Validation phase
	if experiment.Validate != nil {
		cr.addObservation(result, "validation", "info", "Validating experiment results", "framework", nil)
		if validationErr := experiment.Validate(ctx); validationErr != nil {
			result.Success = false
			if execErr == nil {
				result.Error = fmt.Sprintf("validation failed: %v", validationErr)
			}
			cr.addObservation(result, "validation", "error", fmt.Sprintf("Validation failed: %v", validationErr), "framework", nil)
		} else {
			result.Success = (execErr == nil)
		}
	} else {
		result.Success = (execErr == nil)
	}

	if execErr != nil && result.Error == "" {
		result.Error = execErr.Error()
	}

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)
	result.ErrorCount = cr.countObservationsByType(result.Observations, "error")

	// Store result
	cr.mu.Lock()
	cr.results = append(cr.results, result)
	cr.mu.Unlock()

	cr.logger.Info("chaos experiment completed",
		"experiment_id", experimentID,
		"success", result.Success,
		"duration", result.Duration,
		"error_count", result.ErrorCount)

	// Notify observers
	for _, observer := range cr.observers {
		observer.OnExperimentEnd(result)
	}

	return result, nil
}

// RunAllExperiments runs all registered experiments
func (cr *ChaosRunner) RunAllExperiments(ctx context.Context) ([]*ChaosResult, error) {
	cr.mu.RLock()
	experimentIDs := make([]string, 0, len(cr.experiments))
	for id := range cr.experiments {
		experimentIDs = append(experimentIDs, id)
	}
	cr.mu.RUnlock()

	results := make([]*ChaosResult, 0, len(experimentIDs))

	for _, experimentID := range experimentIDs {
		result, err := cr.RunExperiment(ctx, experimentID)
		if err != nil {
			cr.logger.Error("experiment failed", "experiment_id", experimentID, "error", err)
		}
		if result != nil {
			results = append(results, result)
		}
	}

	return results, nil
}

// addObservation adds an observation to the result
func (cr *ChaosRunner) addObservation(result *ChaosResult, observationType, severity, message, component string, metadata map[string]interface{}) {
	observation := Observation{
		Timestamp: time.Now(),
		Type:      observationType,
		Severity:  severity,
		Message:   message,
		Component: component,
		Metadata:  metadata,
	}

	result.Observations = append(result.Observations, observation)

	// Notify observers
	for _, observer := range cr.observers {
		observer.OnObservation(&observation)
	}
}

// countObservationsByType counts observations by type
func (cr *ChaosRunner) countObservationsByType(observations []Observation, observationType string) int {
	count := 0
	for _, obs := range observations {
		if obs.Type == observationType {
			count++
		}
	}
	return count
}

// GetResults returns all experiment results
func (cr *ChaosRunner) GetResults() []*ChaosResult {
	cr.mu.RLock()
	defer cr.mu.RUnlock()

	// Return a copy to prevent external modification
	results := make([]*ChaosResult, len(cr.results))
	copy(results, cr.results)
	return results
}

// GetResultsByExperiment returns results for a specific experiment
func (cr *ChaosRunner) GetResultsByExperiment(experimentID string) []*ChaosResult {
	cr.mu.RLock()
	defer cr.mu.RUnlock()

	var results []*ChaosResult
	for _, result := range cr.results {
		if result.ExperimentID == experimentID {
			results = append(results, result)
		}
	}
	return results
}

// ClearResults clears all stored results
func (cr *ChaosRunner) ClearResults() {
	cr.mu.Lock()
	defer cr.mu.Unlock()

	cr.results = make([]*ChaosResult, 0)
	cr.logger.Info("chaos experiment results cleared")
}

// FailureInjector provides various failure injection capabilities
type FailureInjector struct {
	logger *slog.Logger
	active map[string]context.CancelFunc
	mu     sync.RWMutex
}

// NewFailureInjector creates a new failure injector
func NewFailureInjector(logger *slog.Logger) *FailureInjector {
	if logger == nil {
		logger = slog.Default()
	}

	return &FailureInjector{
		logger: logger.With("component", "failure_injector"),
		active: make(map[string]context.CancelFunc),
	}
}

// InjectNetworkLatency injects network latency
func (fi *FailureInjector) InjectNetworkLatency(ctx context.Context, latency time.Duration, duration time.Duration) error {
	fi.logger.Info("injecting network latency",
		"latency", latency,
		"duration", duration)

	// Create a cancellable context for the injection
	injectionCtx, cancel := context.WithTimeout(ctx, duration)

	fi.mu.Lock()
	fi.active["network_latency"] = cancel
	fi.mu.Unlock()

	defer func() {
		fi.mu.Lock()
		delete(fi.active, "network_latency")
		fi.mu.Unlock()
	}()

	// Simulate network latency injection
	// In a real implementation, this would configure network interfaces or use tools like tc
	select {
	case <-injectionCtx.Done():
		fi.logger.Info("network latency injection completed")
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// InjectResourceExhaustion simulates resource exhaustion
func (fi *FailureInjector) InjectResourceExhaustion(ctx context.Context, resourceType string, percentage float64, duration time.Duration) error {
	fi.logger.Info("injecting resource exhaustion",
		"resource_type", resourceType,
		"percentage", percentage,
		"duration", duration)

	injectionCtx, cancel := context.WithTimeout(ctx, duration)

	fi.mu.Lock()
	fi.active[fmt.Sprintf("resource_%s", resourceType)] = cancel
	fi.mu.Unlock()

	defer func() {
		fi.mu.Lock()
		delete(fi.active, fmt.Sprintf("resource_%s", resourceType))
		fi.mu.Unlock()
	}()

	switch resourceType {
	case "cpu":
		return fi.injectCPUExhaustion(injectionCtx, percentage)
	case "memory":
		return fi.injectMemoryExhaustion(injectionCtx, percentage)
	case "disk":
		return fi.injectDiskExhaustion(injectionCtx, percentage)
	default:
		return fmt.Errorf("unsupported resource type: %s", resourceType)
	}
}

// injectCPUExhaustion simulates CPU exhaustion
func (fi *FailureInjector) injectCPUExhaustion(ctx context.Context, percentage float64) error {
	// Simulate CPU load by running busy loops
	numGoroutines := int(percentage / 10) // Scale based on percentage
	if numGoroutines < 1 {
		numGoroutines = 1
	}

	var wg sync.WaitGroup
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
					// Busy loop to consume CPU
					for j := 0; j < 1000000; j++ {
						_ = rand.Float64()
					}
					// Small sleep to prevent complete system lockup
					time.Sleep(1 * time.Microsecond)
				}
			}
		}()
	}

	<-ctx.Done()
	wg.Wait()
	return nil
}

// injectMemoryExhaustion simulates memory exhaustion
func (fi *FailureInjector) injectMemoryExhaustion(ctx context.Context, percentage float64) error {
	// Allocate memory to simulate exhaustion
	memoryBlocks := make([][]byte, 0)
	blockSize := 1024 * 1024 // 1MB blocks

	// Calculate number of blocks based on percentage
	maxBlocks := int(percentage * 10) // Scale factor

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Release allocated memory
			memoryBlocks = nil
			return nil
		case <-ticker.C:
			if len(memoryBlocks) < maxBlocks {
				block := make([]byte, blockSize)
				// Write some data to ensure allocation
				for i := range block {
					block[i] = byte(i % 256)
				}
				memoryBlocks = append(memoryBlocks, block)
			}
		}
	}
}

// injectDiskExhaustion simulates disk exhaustion
func (fi *FailureInjector) injectDiskExhaustion(ctx context.Context, percentage float64) error {
	// Simulate disk I/O load
	// In a real implementation, this would create temporary files and perform I/O operations
	fi.logger.Info("simulating disk exhaustion", "percentage", percentage)

	<-ctx.Done()
	return nil
}

// StopAllInjections stops all active failure injections
func (fi *FailureInjector) StopAllInjections() {
	fi.mu.Lock()
	defer fi.mu.Unlock()

	for name, cancel := range fi.active {
		cancel()
		fi.logger.Info("stopped failure injection", "injection", name)
	}
	fi.active = make(map[string]context.CancelFunc)
}

// GetActiveInjections returns names of all active injections
func (fi *FailureInjector) GetActiveInjections() []string {
	fi.mu.RLock()
	defer fi.mu.RUnlock()

	injections := make([]string, 0, len(fi.active))
	for name := range fi.active {
		injections = append(injections, name)
	}
	return injections
}

// LoggingObserver logs all chaos experiment events
type LoggingObserver struct {
	logger *slog.Logger
}

// NewLoggingObserver creates a new logging observer
func NewLoggingObserver(logger *slog.Logger) *LoggingObserver {
	if logger == nil {
		logger = slog.Default()
	}

	return &LoggingObserver{
		logger: logger.With("component", "chaos_observer"),
	}
}

// OnExperimentStart logs experiment start
func (lo *LoggingObserver) OnExperimentStart(experiment *ChaosExperiment) {
	lo.logger.Info("chaos experiment started",
		"experiment_id", experiment.ID,
		"name", experiment.Name,
		"duration", experiment.Duration)
}

// OnExperimentEnd logs experiment completion
func (lo *LoggingObserver) OnExperimentEnd(result *ChaosResult) {
	lo.logger.Info("chaos experiment completed",
		"experiment_id", result.ExperimentID,
		"success", result.Success,
		"duration", result.Duration,
		"error_count", result.ErrorCount,
		"observation_count", len(result.Observations))
}

// OnObservation logs observations
func (lo *LoggingObserver) OnObservation(observation *Observation) {
	level := slog.LevelInfo
	switch observation.Severity {
	case "error":
		level = slog.LevelError
	case "warning":
		level = slog.LevelWarn
	case "debug":
		level = slog.LevelDebug
	}

	lo.logger.Log(context.Background(), level, "chaos observation",
		"type", observation.Type,
		"component", observation.Component,
		"message", observation.Message,
		"timestamp", observation.Timestamp)
}

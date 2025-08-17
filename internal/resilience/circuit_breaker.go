package resilience

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// CircuitBreakerState represents the state of a circuit breaker
type CircuitBreakerState string

const (
	// StateClosed indicates the circuit breaker is closed (normal operation)
	StateClosed CircuitBreakerState = "closed"
	
	// StateOpen indicates the circuit breaker is open (failing fast)
	StateOpen CircuitBreakerState = "open"
	
	// StateHalfOpen indicates the circuit breaker is half-open (testing recovery)
	StateHalfOpen CircuitBreakerState = "half_open"
)

// CircuitBreakerConfig holds configuration for circuit breakers
type CircuitBreakerConfig struct {
	// Failure threshold to open the circuit
	FailureThreshold int `json:"failure_threshold"` // e.g., 5 failures
	
	// Success threshold to close the circuit from half-open
	SuccessThreshold int `json:"success_threshold"` // e.g., 3 successes
	
	// Timeout before transitioning from open to half-open
	OpenTimeout time.Duration `json:"open_timeout"` // e.g., 60 seconds
	
	// Rolling window for counting failures
	RollingWindow time.Duration `json:"rolling_window"` // e.g., 30 seconds
	
	// Maximum number of half-open requests
	MaxHalfOpenRequests int `json:"max_half_open_requests"` // e.g., 3
}

// DefaultCircuitBreakerConfig returns sensible defaults
func DefaultCircuitBreakerConfig() *CircuitBreakerConfig {
	return &CircuitBreakerConfig{
		FailureThreshold:    5,
		SuccessThreshold:    3,
		OpenTimeout:         60 * time.Second,
		RollingWindow:       30 * time.Second,
		MaxHalfOpenRequests: 3,
	}
}

// Validate checks if the configuration is valid
func (config *CircuitBreakerConfig) Validate() error {
	if config.FailureThreshold <= 0 {
		return fmt.Errorf("failure threshold must be positive")
	}
	if config.SuccessThreshold <= 0 {
		return fmt.Errorf("success threshold must be positive")
	}
	if config.OpenTimeout <= 0 {
		return fmt.Errorf("open timeout must be positive")
	}
	if config.RollingWindow <= 0 {
		return fmt.Errorf("rolling window must be positive")
	}
	if config.MaxHalfOpenRequests <= 0 {
		return fmt.Errorf("max half-open requests must be positive")
	}
	return nil
}

// ExecutionResult represents the result of a circuit breaker protected operation
type ExecutionResult struct {
	Success   bool          `json:"success"`
	Error     error         `json:"error,omitempty"`
	Duration  time.Duration `json:"duration"`
	Timestamp time.Time     `json:"timestamp"`
}

// CircuitBreakerStats holds statistics about circuit breaker performance
type CircuitBreakerStats struct {
	State              CircuitBreakerState `json:"state"`
	FailureCount       int                 `json:"failure_count"`
	SuccessCount       int                 `json:"success_count"`
	TotalRequests      int64               `json:"total_requests"`
	TotalFailures      int64               `json:"total_failures"`
	TotalSuccesses     int64               `json:"total_successes"`
	LastFailureTime    *time.Time          `json:"last_failure_time,omitempty"`
	LastSuccessTime    *time.Time          `json:"last_success_time,omitempty"`
	StateChangedAt     time.Time           `json:"state_changed_at"`
	OpenedAt           *time.Time          `json:"opened_at,omitempty"`
	HalfOpenRequests   int                 `json:"half_open_requests"`
}

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
	mu                 sync.RWMutex
	config             *CircuitBreakerConfig
	logger             *slog.Logger
	
	// State management
	state              CircuitBreakerState
	stateChangedAt     time.Time
	openedAt           *time.Time
	
	// Counters
	failureCount       int
	successCount       int
	totalRequests      int64
	totalFailures      int64
	totalSuccesses     int64
	halfOpenRequests   int
	
	// Timestamps for rolling window
	recentFailures     []time.Time
	recentSuccesses    []time.Time
	lastFailureTime    *time.Time
	lastSuccessTime    *time.Time
	
	// Name for logging and identification
	name string
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(name string, config *CircuitBreakerConfig, logger *slog.Logger) (*CircuitBreaker, error) {
	if config == nil {
		config = DefaultCircuitBreakerConfig()
	}
	
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid circuit breaker config: %w", err)
	}
	
	if logger == nil {
		logger = slog.Default()
	}

	return &CircuitBreaker{
		config:         config,
		logger:         logger.With("component", "circuit_breaker", "name", name),
		state:          StateClosed,
		stateChangedAt: time.Now(),
		name:           name,
	}, nil
}

// Execute runs the given function with circuit breaker protection
func (cb *CircuitBreaker) Execute(ctx context.Context, operation func(ctx context.Context) error) error {
	// Check if we can proceed with the request
	if !cb.canProceed() {
		cb.recordRequest(false)
		return fmt.Errorf("circuit breaker %s is open", cb.name)
	}

	// Record that we're processing a request
	cb.incrementHalfOpenRequests()
	defer cb.decrementHalfOpenRequests()

	start := time.Now()
	err := operation(ctx)
	duration := time.Since(start)

	// Record the result
	success := err == nil
	cb.recordResult(success, duration)

	return err
}

// canProceed determines if a request can proceed based on circuit breaker state
func (cb *CircuitBreaker) canProceed() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	switch cb.state {
	case StateClosed:
		return true
	case StateOpen:
		// Check if we should transition to half-open
		if time.Since(*cb.openedAt) >= cb.config.OpenTimeout {
			cb.mu.RUnlock()
			cb.mu.Lock()
			// Double-check after acquiring write lock
			if cb.state == StateOpen && time.Since(*cb.openedAt) >= cb.config.OpenTimeout {
				cb.transitionToHalfOpen()
			}
			cb.mu.Unlock()
			cb.mu.RLock()
			return cb.state == StateHalfOpen && cb.halfOpenRequests < cb.config.MaxHalfOpenRequests
		}
		return false
	case StateHalfOpen:
		return cb.halfOpenRequests < cb.config.MaxHalfOpenRequests
	default:
		return false
	}
}

// recordResult records the result of an operation and updates circuit breaker state
func (cb *CircuitBreaker) recordResult(success bool, duration time.Duration) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	now := time.Now()
	cb.totalRequests++

	if success {
		cb.totalSuccesses++
		cb.successCount++
		cb.lastSuccessTime = &now
		cb.recentSuccesses = append(cb.recentSuccesses, now)
		
		// Clean old successes outside rolling window
		cb.cleanOldResults(&cb.recentSuccesses, now)

		// Check if we should close the circuit from half-open
		if cb.state == StateHalfOpen && cb.successCount >= cb.config.SuccessThreshold {
			cb.transitionToClosed()
		}
	} else {
		cb.totalFailures++
		cb.failureCount++
		cb.lastFailureTime = &now
		cb.recentFailures = append(cb.recentFailures, now)
		
		// Clean old failures outside rolling window
		cb.cleanOldResults(&cb.recentFailures, now)

		// Check if we should open the circuit
		if (cb.state == StateClosed || cb.state == StateHalfOpen) && 
		   cb.failureCount >= cb.config.FailureThreshold {
			cb.transitionToOpen()
		}
	}

	cb.logger.Debug("operation result recorded",
		"success", success,
		"duration", duration,
		"state", cb.state,
		"failure_count", cb.failureCount,
		"success_count", cb.successCount)
}

// recordRequest records a request attempt (even if it doesn't execute)
func (cb *CircuitBreaker) recordRequest(executed bool) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	
	if executed {
		cb.totalRequests++
	}
}

// incrementHalfOpenRequests increments the half-open request counter
func (cb *CircuitBreaker) incrementHalfOpenRequests() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	
	if cb.state == StateHalfOpen {
		cb.halfOpenRequests++
	}
}

// decrementHalfOpenRequests decrements the half-open request counter
func (cb *CircuitBreaker) decrementHalfOpenRequests() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	
	if cb.state == StateHalfOpen && cb.halfOpenRequests > 0 {
		cb.halfOpenRequests--
	}
}

// transitionToOpen transitions the circuit breaker to open state
func (cb *CircuitBreaker) transitionToOpen() {
	now := time.Now()
	cb.state = StateOpen
	cb.stateChangedAt = now
	cb.openedAt = &now
	cb.failureCount = 0 // Reset for next cycle
	cb.successCount = 0
	cb.halfOpenRequests = 0

	cb.logger.Warn("circuit breaker opened due to failures",
		"failure_threshold", cb.config.FailureThreshold,
		"open_timeout", cb.config.OpenTimeout)
}

// transitionToHalfOpen transitions the circuit breaker to half-open state
func (cb *CircuitBreaker) transitionToHalfOpen() {
	cb.state = StateHalfOpen
	cb.stateChangedAt = time.Now()
	cb.failureCount = 0 // Reset for testing
	cb.successCount = 0
	cb.halfOpenRequests = 0

	cb.logger.Info("circuit breaker transitioned to half-open for testing")
}

// transitionToClosed transitions the circuit breaker to closed state
func (cb *CircuitBreaker) transitionToClosed() {
	cb.state = StateClosed
	cb.stateChangedAt = time.Now()
	cb.openedAt = nil
	cb.failureCount = 0
	cb.successCount = 0
	cb.halfOpenRequests = 0

	cb.logger.Info("circuit breaker closed after successful recovery")
}

// cleanOldResults removes results outside the rolling window
func (cb *CircuitBreaker) cleanOldResults(results *[]time.Time, now time.Time) {
	cutoff := now.Add(-cb.config.RollingWindow)
	
	// Find the first result within the window
	i := 0
	for i < len(*results) && (*results)[i].Before(cutoff) {
		i++
	}
	
	// Remove old results
	if i > 0 {
		*results = (*results)[i:]
	}
	
	// Update failure count based on remaining results
	if results == &cb.recentFailures {
		cb.failureCount = len(cb.recentFailures)
	} else if results == &cb.recentSuccesses {
		cb.successCount = len(cb.recentSuccesses)
	}
}

// GetState returns the current state of the circuit breaker
func (cb *CircuitBreaker) GetState() CircuitBreakerState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// GetStats returns statistics about the circuit breaker
func (cb *CircuitBreaker) GetStats() CircuitBreakerStats {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	return CircuitBreakerStats{
		State:              cb.state,
		FailureCount:       cb.failureCount,
		SuccessCount:       cb.successCount,
		TotalRequests:      cb.totalRequests,
		TotalFailures:      cb.totalFailures,
		TotalSuccesses:     cb.totalSuccesses,
		LastFailureTime:    cb.lastFailureTime,
		LastSuccessTime:    cb.lastSuccessTime,
		StateChangedAt:     cb.stateChangedAt,
		OpenedAt:           cb.openedAt,
		HalfOpenRequests:   cb.halfOpenRequests,
	}
}

// Reset resets the circuit breaker to its initial state
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.state = StateClosed
	cb.stateChangedAt = time.Now()
	cb.openedAt = nil
	cb.failureCount = 0
	cb.successCount = 0
	cb.totalRequests = 0
	cb.totalFailures = 0
	cb.totalSuccesses = 0
	cb.halfOpenRequests = 0
	cb.recentFailures = nil
	cb.recentSuccesses = nil
	cb.lastFailureTime = nil
	cb.lastSuccessTime = nil

	cb.logger.Info("circuit breaker reset to initial state")
}

// ForceOpen forces the circuit breaker to open state (for testing)
func (cb *CircuitBreaker) ForceOpen() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.transitionToOpen()
	cb.logger.Warn("circuit breaker force-opened")
}

// ForceClose forces the circuit breaker to closed state (for testing)
func (cb *CircuitBreaker) ForceClose() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.transitionToClosed()
	cb.logger.Info("circuit breaker force-closed")
}

// IsHealthy returns true if the circuit breaker is in a healthy state
func (cb *CircuitBreaker) IsHealthy() bool {
	return cb.GetState() != StateOpen
}

// GetName returns the name of the circuit breaker
func (cb *CircuitBreaker) GetName() string {
	return cb.name
}
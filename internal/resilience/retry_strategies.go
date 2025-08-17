package resilience

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"math/rand"
	"sync"
	"time"
)

// RetryStrategyType represents different retry strategy types
type RetryStrategyType string

const (
	// StrategyFixedDelay uses a fixed delay between retries
	StrategyFixedDelay RetryStrategyType = "fixed_delay"
	
	// StrategyLinearBackoff uses linear backoff (delay * attempt)
	StrategyLinearBackoff RetryStrategyType = "linear_backoff"
	
	// StrategyExponentialBackoff uses exponential backoff (base^attempt)
	StrategyExponentialBackoff RetryStrategyType = "exponential_backoff"
	
	// StrategyFibonacciBackoff uses Fibonacci sequence for backoff
	StrategyFibonacciBackoff RetryStrategyType = "fibonacci_backoff"
	
	// StrategyDecorrelatedJitter uses AWS's decorrelated jitter
	StrategyDecorrelatedJitter RetryStrategyType = "decorrelated_jitter"
)

// JitterType represents different jitter types
type JitterType string

const (
	// JitterNone disables jitter
	JitterNone JitterType = "none"
	
	// JitterFull applies full jitter (0 to calculated delay)
	JitterFull JitterType = "full"
	
	// JitterEqual applies equal jitter (50% to 100% of calculated delay)
	JitterEqual JitterType = "equal"
	
	// JitterDecorrelated uses decorrelated jitter
	JitterDecorrelated JitterType = "decorrelated"
)

// RetryBudgetConfig defines retry budget configuration
type RetryBudgetConfig struct {
	// Maximum number of retries allowed globally per time window
	MaxRetries int `json:"max_retries"`
	
	// Time window for retry budget
	TimeWindow time.Duration `json:"time_window"`
	
	// Percentage of budget that can be used for retries (0-100)
	BudgetPercentage float64 `json:"budget_percentage"`
	
	// Enable dynamic budget adjustment based on success rate
	DynamicAdjustment bool `json:"dynamic_adjustment"`
	
	// Minimum budget percentage (for dynamic adjustment)
	MinBudgetPercentage float64 `json:"min_budget_percentage"`
	
	// Maximum budget percentage (for dynamic adjustment)
	MaxBudgetPercentage float64 `json:"max_budget_percentage"`
}

// RetryStrategyConfig defines configuration for retry strategies
type RetryStrategyConfig struct {
	// Strategy type
	Strategy RetryStrategyType `json:"strategy"`
	
	// Jitter configuration
	Jitter JitterType `json:"jitter"`
	
	// Basic retry parameters
	MaxAttempts   int           `json:"max_attempts"`
	BaseDelay     time.Duration `json:"base_delay"`
	MaxDelay      time.Duration `json:"max_delay"`
	BackoffFactor float64       `json:"backoff_factor"`
	
	// Jitter parameters
	JitterRange float64 `json:"jitter_range"` // 0.0 to 1.0
	
	// Budget configuration
	Budget *RetryBudgetConfig `json:"budget,omitempty"`
	
	// Circuit breaker integration
	UseCircuitBreaker bool   `json:"use_circuit_breaker"`
	CircuitBreakerName string `json:"circuit_breaker_name,omitempty"`
	
	// Conditional retry configuration
	RetryConditions []RetryCondition `json:"retry_conditions"`
}

// RetryCondition defines when a retry should be attempted
type RetryCondition struct {
	// Error types that should be retried
	RetryableErrorTypes []string `json:"retryable_error_types"`
	
	// HTTP status codes that should be retried (if applicable)
	RetryableStatusCodes []int `json:"retryable_status_codes"`
	
	// Maximum response time for retry consideration
	MaxResponseTime time.Duration `json:"max_response_time"`
	
	// Custom retry predicate function name
	PredicateFunction string `json:"predicate_function,omitempty"`
}

// DefaultRetryStrategyConfig returns sensible defaults
func DefaultRetryStrategyConfig() *RetryStrategyConfig {
	return &RetryStrategyConfig{
		Strategy:       StrategyExponentialBackoff,
		Jitter:         JitterEqual,
		MaxAttempts:    5,
		BaseDelay:      1 * time.Second,
		MaxDelay:       60 * time.Second,
		BackoffFactor:  2.0,
		JitterRange:    0.1,
		Budget: &RetryBudgetConfig{
			MaxRetries:          1000,
			TimeWindow:          1 * time.Hour,
			BudgetPercentage:    10.0,
			DynamicAdjustment:   true,
			MinBudgetPercentage: 5.0,
			MaxBudgetPercentage: 25.0,
		},
		UseCircuitBreaker: false,
		RetryConditions: []RetryCondition{
			{
				RetryableErrorTypes:  []string{"timeout", "network", "rate_limit"},
				RetryableStatusCodes: []int{429, 502, 503, 504},
				MaxResponseTime:      30 * time.Second,
			},
		},
	}
}

// Validate checks if the configuration is valid
func (config *RetryStrategyConfig) Validate() error {
	if config.MaxAttempts <= 0 {
		return fmt.Errorf("max attempts must be positive")
	}
	if config.BaseDelay <= 0 {
		return fmt.Errorf("base delay must be positive")
	}
	if config.MaxDelay <= 0 {
		return fmt.Errorf("max delay must be positive")
	}
	if config.BackoffFactor <= 0 {
		return fmt.Errorf("backoff factor must be positive")
	}
	if config.JitterRange < 0 || config.JitterRange > 1 {
		return fmt.Errorf("jitter range must be between 0 and 1")
	}
	
	if config.Budget != nil {
		if err := config.Budget.Validate(); err != nil {
			return fmt.Errorf("invalid budget config: %w", err)
		}
	}
	
	return nil
}

// Validate checks if the budget configuration is valid
func (config *RetryBudgetConfig) Validate() error {
	if config.MaxRetries <= 0 {
		return fmt.Errorf("max retries must be positive")
	}
	if config.TimeWindow <= 0 {
		return fmt.Errorf("time window must be positive")
	}
	if config.BudgetPercentage < 0 || config.BudgetPercentage > 100 {
		return fmt.Errorf("budget percentage must be between 0 and 100")
	}
	if config.DynamicAdjustment {
		if config.MinBudgetPercentage < 0 || config.MinBudgetPercentage > 100 {
			return fmt.Errorf("min budget percentage must be between 0 and 100")
		}
		if config.MaxBudgetPercentage < 0 || config.MaxBudgetPercentage > 100 {
			return fmt.Errorf("max budget percentage must be between 0 and 100")
		}
		if config.MinBudgetPercentage > config.MaxBudgetPercentage {
			return fmt.Errorf("min budget percentage cannot be greater than max")
		}
	}
	return nil
}

// RetryAttempt represents a single retry attempt
type RetryAttempt struct {
	Attempt     int           `json:"attempt"`
	Delay       time.Duration `json:"delay"`
	StartTime   time.Time     `json:"start_time"`
	EndTime     time.Time     `json:"end_time"`
	Error       error         `json:"error,omitempty"`
	Success     bool          `json:"success"`
	Duration    time.Duration `json:"duration"`
	BudgetUsed  int           `json:"budget_used"`
}

// RetryStats holds statistics about retry attempts
type RetryStats struct {
	TotalAttempts     int64         `json:"total_attempts"`
	TotalRetries      int64         `json:"total_retries"`
	SuccessfulRetries int64         `json:"successful_retries"`
	FailedRetries     int64         `json:"failed_retries"`
	AverageDelay      time.Duration `json:"average_delay"`
	TotalDelay        time.Duration `json:"total_delay"`
	BudgetRemaining   int           `json:"budget_remaining"`
	BudgetUsed        int           `json:"budget_used"`
	SuccessRate       float64       `json:"success_rate"`
	LastRetryTime     *time.Time    `json:"last_retry_time,omitempty"`
}

// RetryBudget manages retry budget allocation
type RetryBudget struct {
	mu                    sync.RWMutex
	config                *RetryBudgetConfig
	logger                *slog.Logger
	
	// Budget tracking
	used                  int
	remaining             int
	windowStart           time.Time
	currentBudgetPercent  float64
	
	// Success rate tracking for dynamic adjustment
	totalOperations       int
	successfulOperations  int
	lastAdjustment        time.Time
}

// NewRetryBudget creates a new retry budget manager
func NewRetryBudget(config *RetryBudgetConfig, logger *slog.Logger) *RetryBudget {
	if config == nil {
		config = DefaultRetryStrategyConfig().Budget
	}
	
	if logger == nil {
		logger = slog.Default()
	}
	
	budget := &RetryBudget{
		config:               config,
		logger:               logger.With("component", "retry_budget"),
		windowStart:          time.Now(),
		currentBudgetPercent: config.BudgetPercentage,
		lastAdjustment:       time.Now(),
	}
	
	budget.resetBudget()
	return budget
}

// CanRetry checks if a retry is allowed within the budget
func (rb *RetryBudget) CanRetry(ctx context.Context) bool {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	
	// Check if we need to reset the time window
	if time.Since(rb.windowStart) >= rb.config.TimeWindow {
		rb.resetBudget()
	}
	
	// Adjust budget dynamically if enabled
	if rb.config.DynamicAdjustment {
		rb.adjustBudgetDynamically()
	}
	
	canRetry := rb.remaining > 0
	
	rb.logger.Debug("retry budget check",
		"can_retry", canRetry,
		"remaining", rb.remaining,
		"used", rb.used,
		"current_percent", rb.currentBudgetPercent)
	
	return canRetry
}

// ConsumeRetry consumes one retry from the budget
func (rb *RetryBudget) ConsumeRetry(ctx context.Context) bool {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	
	if rb.remaining <= 0 {
		return false
	}
	
	rb.used++
	rb.remaining--
	
	rb.logger.Debug("retry budget consumed",
		"used", rb.used,
		"remaining", rb.remaining)
	
	return true
}

// RecordOperation records the outcome of an operation for budget adjustment
func (rb *RetryBudget) RecordOperation(success bool) {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	
	rb.totalOperations++
	if success {
		rb.successfulOperations++
	}
}

// GetStats returns current budget statistics
func (rb *RetryBudget) GetStats() RetryStats {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	
	successRate := 0.0
	if rb.totalOperations > 0 {
		successRate = float64(rb.successfulOperations) / float64(rb.totalOperations)
	}
	
	return RetryStats{
		BudgetRemaining: rb.remaining,
		BudgetUsed:      rb.used,
		SuccessRate:     successRate,
	}
}

// resetBudget resets the budget for a new time window
func (rb *RetryBudget) resetBudget() {
	maxBudget := int(float64(rb.config.MaxRetries) * rb.currentBudgetPercent / 100.0)
	rb.used = 0
	rb.remaining = maxBudget
	rb.windowStart = time.Now()
	
	rb.logger.Info("retry budget reset",
		"max_budget", maxBudget,
		"budget_percent", rb.currentBudgetPercent,
		"window_start", rb.windowStart)
}

// adjustBudgetDynamically adjusts budget based on success rate
func (rb *RetryBudget) adjustBudgetDynamically() {
	// Only adjust every 5 minutes
	if time.Since(rb.lastAdjustment) < 5*time.Minute {
		return
	}
	
	if rb.totalOperations < 10 {
		return // Need enough samples
	}
	
	successRate := float64(rb.successfulOperations) / float64(rb.totalOperations)
	oldPercent := rb.currentBudgetPercent
	
	// Adjust budget based on success rate
	if successRate > 0.95 {
		// High success rate, can reduce retry budget
		rb.currentBudgetPercent = math.Max(rb.currentBudgetPercent*0.9, rb.config.MinBudgetPercentage)
	} else if successRate < 0.85 {
		// Lower success rate, increase retry budget
		rb.currentBudgetPercent = math.Min(rb.currentBudgetPercent*1.1, rb.config.MaxBudgetPercentage)
	}
	
	rb.lastAdjustment = time.Now()
	
	if oldPercent != rb.currentBudgetPercent {
		rb.logger.Info("retry budget adjusted dynamically",
			"old_percent", oldPercent,
			"new_percent", rb.currentBudgetPercent,
			"success_rate", successRate,
			"total_operations", rb.totalOperations)
	}
	
	// Reset counters for next period
	rb.totalOperations = 0
	rb.successfulOperations = 0
}

// RetryStrategy implements different retry strategies with jitter
type RetryStrategy struct {
	config      *RetryStrategyConfig
	budget      *RetryBudget
	logger      *slog.Logger
	
	// State for decorrelated jitter
	lastDelay   time.Duration
	
	// Random source
	rng         *rand.Rand
	
	// Statistics
	mu          sync.RWMutex
	stats       RetryStats
	attempts    []RetryAttempt
}

// NewRetryStrategy creates a new retry strategy
func NewRetryStrategy(config *RetryStrategyConfig, logger *slog.Logger) (*RetryStrategy, error) {
	if config == nil {
		config = DefaultRetryStrategyConfig()
	}
	
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid retry strategy config: %w", err)
	}
	
	if logger == nil {
		logger = slog.Default()
	}
	
	strategy := &RetryStrategy{
		config:    config,
		logger:    logger.With("component", "retry_strategy"),
		lastDelay: config.BaseDelay,
		rng:       rand.New(rand.NewSource(time.Now().UnixNano())),
		attempts:  make([]RetryAttempt, 0),
	}
	
	// Initialize budget if configured
	if config.Budget != nil {
		strategy.budget = NewRetryBudget(config.Budget, logger)
	}
	
	return strategy, nil
}

// CalculateDelay calculates the delay for the given attempt number
func (rs *RetryStrategy) CalculateDelay(attempt int) time.Duration {
	if attempt <= 0 {
		return 0
	}
	
	var baseDelay time.Duration
	
	// Calculate base delay using the selected strategy
	switch rs.config.Strategy {
	case StrategyFixedDelay:
		baseDelay = rs.config.BaseDelay
		
	case StrategyLinearBackoff:
		baseDelay = time.Duration(float64(rs.config.BaseDelay) * float64(attempt))
		
	case StrategyExponentialBackoff:
		baseDelay = time.Duration(float64(rs.config.BaseDelay) * math.Pow(rs.config.BackoffFactor, float64(attempt-1)))
		
	case StrategyFibonacciBackoff:
		fibNumber := rs.fibonacci(attempt)
		baseDelay = time.Duration(float64(rs.config.BaseDelay) * float64(fibNumber))
		
	case StrategyDecorrelatedJitter:
		// AWS decorrelated jitter: random between base and 3*lastDelay
		minDelay := rs.config.BaseDelay
		maxDelay := time.Duration(float64(rs.lastDelay) * 3)
		if maxDelay < minDelay {
			maxDelay = minDelay
		}
		baseDelay = rs.randomBetween(minDelay, maxDelay)
		rs.lastDelay = baseDelay
		
	default:
		baseDelay = rs.config.BaseDelay
	}
	
	// Apply maximum delay limit
	if baseDelay > rs.config.MaxDelay {
		baseDelay = rs.config.MaxDelay
	}
	
	// Apply jitter
	finalDelay := rs.applyJitter(baseDelay)
	
	rs.logger.Debug("calculated retry delay",
		"attempt", attempt,
		"strategy", rs.config.Strategy,
		"base_delay", baseDelay,
		"final_delay", finalDelay,
		"jitter", rs.config.Jitter)
	
	return finalDelay
}

// applyJitter applies the configured jitter to the delay
func (rs *RetryStrategy) applyJitter(delay time.Duration) time.Duration {
	switch rs.config.Jitter {
	case JitterNone:
		return delay
		
	case JitterFull:
		// Random between 0 and delay
		maxJitter := int64(delay)
		if maxJitter <= 0 {
			return delay
		}
		return time.Duration(rs.rng.Int63n(maxJitter))
		
	case JitterEqual:
		// Random between delay/2 and delay
		halfDelay := delay / 2
		jitterRange := delay - halfDelay
		if jitterRange <= 0 {
			return delay
		}
		return halfDelay + time.Duration(rs.rng.Int63n(int64(jitterRange)))
		
	case JitterDecorrelated:
		// Handled in strategy calculation
		return delay
		
	default:
		return delay
	}
}

// fibonacci calculates the nth Fibonacci number
func (rs *RetryStrategy) fibonacci(n int) int {
	if n <= 1 {
		return 1
	}
	if n == 2 {
		return 1
	}
	
	a, b := 1, 1
	for i := 3; i <= n; i++ {
		a, b = b, a+b
	}
	return b
}

// randomBetween returns a random duration between min and max
func (rs *RetryStrategy) randomBetween(min, max time.Duration) time.Duration {
	if min >= max {
		return min
	}
	
	diff := max - min
	return min + time.Duration(rs.rng.Int63n(int64(diff)))
}

// ShouldRetry determines if an operation should be retried based on error and conditions
func (rs *RetryStrategy) ShouldRetry(ctx context.Context, attempt int, err error) bool {
	// Check attempt limit
	if attempt >= rs.config.MaxAttempts {
		rs.logger.Debug("max attempts reached", "attempt", attempt, "max", rs.config.MaxAttempts)
		return false
	}
	
	// Check budget
	if rs.budget != nil && !rs.budget.CanRetry(ctx) {
		rs.logger.Debug("retry budget exhausted")
		return false
	}
	
	// Check retry conditions
	if !rs.isRetryableError(err) {
		rs.logger.Debug("error is not retryable", "error", err)
		return false
	}
	
	return true
}

// isRetryableError checks if an error meets retry conditions
func (rs *RetryStrategy) isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	
	// For now, implement basic retryable error detection
	// This could be extended to use the RetryConditions in the config
	errorStr := err.Error()
	
	for _, condition := range rs.config.RetryConditions {
		for _, errorType := range condition.RetryableErrorTypes {
			if contains(errorStr, errorType) {
				return true
			}
		}
	}
	
	return false
}

// contains checks if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	return len(s) >= len(substr) && 
		   (s == substr || len(substr) == 0 || 
		    s[:len(substr)] == substr || 
		    s[len(s)-len(substr):] == substr ||
		    findSubstring(s, substr))
}

// findSubstring performs simple substring search
func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// RecordAttempt records a retry attempt for statistics
func (rs *RetryStrategy) RecordAttempt(attempt RetryAttempt) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	
	rs.attempts = append(rs.attempts, attempt)
	rs.stats.TotalAttempts++
	
	if attempt.Attempt > 1 {
		rs.stats.TotalRetries++
		if attempt.Success {
			rs.stats.SuccessfulRetries++
		} else {
			rs.stats.FailedRetries++
		}
		
		rs.stats.TotalDelay += attempt.Delay
		if rs.stats.TotalRetries > 0 {
			rs.stats.AverageDelay = rs.stats.TotalDelay / time.Duration(rs.stats.TotalRetries)
		}
		
		rs.stats.LastRetryTime = &attempt.StartTime
	}
	
	// Update success rate
	if rs.stats.TotalAttempts > 0 {
		rs.stats.SuccessRate = float64(rs.stats.TotalAttempts-rs.stats.FailedRetries) / float64(rs.stats.TotalAttempts)
	}
	
	// Record in budget if available
	if rs.budget != nil {
		rs.budget.RecordOperation(attempt.Success)
		if attempt.Attempt > 1 {
			rs.budget.ConsumeRetry(context.Background())
		}
	}
}

// GetStats returns current retry statistics
func (rs *RetryStrategy) GetStats() RetryStats {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	
	stats := rs.stats
	
	// Add budget stats if available
	if rs.budget != nil {
		budgetStats := rs.budget.GetStats()
		stats.BudgetRemaining = budgetStats.BudgetRemaining
		stats.BudgetUsed = budgetStats.BudgetUsed
	}
	
	return stats
}

// Reset resets the retry strategy statistics
func (rs *RetryStrategy) Reset() {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	
	rs.stats = RetryStats{}
	rs.attempts = make([]RetryAttempt, 0)
	rs.lastDelay = rs.config.BaseDelay
	
	rs.logger.Info("retry strategy statistics reset")
}
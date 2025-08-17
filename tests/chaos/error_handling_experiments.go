//go:build integration

package chaos

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/voidrunnerhq/voidrunner/internal/executor"
	"github.com/voidrunnerhq/voidrunner/internal/monitoring"
	"github.com/voidrunnerhq/voidrunner/internal/reporting"
	"github.com/voidrunnerhq/voidrunner/internal/resilience"
)

// ErrorHandlingExperiments provides chaos experiments for testing error handling mechanisms
type ErrorHandlingExperiments struct {
	logger           *slog.Logger
	injector         *FailureInjector
	errorAggregator  *reporting.ErrorAggregator
	circuitBreakers  map[string]*resilience.CircuitBreaker
	retryExecutors   map[string]*resilience.RetryExecutor
	loadShedders     map[string]*resilience.LoadShedder
	degradationMgr   *resilience.GracefulDegradation
	resourceMonitor  *monitoring.ResourceMonitor
}

// NewErrorHandlingExperiments creates a new error handling experiments suite
func NewErrorHandlingExperiments(logger *slog.Logger) *ErrorHandlingExperiments {
	if logger == nil {
		logger = slog.Default()
	}

	return &ErrorHandlingExperiments{
		logger:          logger.With("component", "error_handling_experiments"),
		injector:        NewFailureInjector(logger),
		circuitBreakers: make(map[string]*resilience.CircuitBreaker),
		retryExecutors:  make(map[string]*resilience.RetryExecutor),
		loadShedders:    make(map[string]*resilience.LoadShedder),
	}
}

// SetupErrorReporting sets up error reporting components
func (ehe *ErrorHandlingExperiments) SetupErrorReporting() error {
	config := reporting.DefaultErrorAggregatorConfig()
	config.AutoReportInterval = 10 * time.Second // Fast reporting for testing
	
	ehe.errorAggregator = reporting.NewErrorAggregator(config, ehe.logger)
	return nil
}

// SetupResilience sets up resilience components
func (ehe *ErrorHandlingExperiments) SetupResilience() error {
	// Setup circuit breakers
	cbConfig := resilience.DefaultCircuitBreakerConfig()
	cbConfig.FailureThreshold = 3
	cbConfig.OpenTimeout = 10 * time.Second
	
	cb, err := resilience.NewCircuitBreaker("test-service", cbConfig, ehe.logger)
	if err != nil {
		return fmt.Errorf("failed to create circuit breaker: %w", err)
	}
	ehe.circuitBreakers["test-service"] = cb

	// Setup retry executors
	retryConfig := resilience.DefaultRetryStrategyConfig()
	retryConfig.MaxAttempts = 5
	retryConfig.BaseDelay = 100 * time.Millisecond
	
	retryExecutor, err := resilience.NewRetryExecutor(retryConfig, ehe.logger)
	if err != nil {
		return fmt.Errorf("failed to create retry executor: %w", err)
	}
	ehe.retryExecutors["test-service"] = retryExecutor

	// Setup load shedding
	lsConfig := resilience.DefaultLoadSheddingConfig()
	lsConfig.MaxConcurrentRequests = 10
	lsConfig.CPUThreshold = 80.0
	
	loadShedder, err := resilience.NewLoadShedder(lsConfig, nil, ehe.logger)
	if err != nil {
		return fmt.Errorf("failed to create load shedder: %w", err)
	}
	ehe.loadShedders["test-service"] = loadShedder

	// Setup graceful degradation
	degradationConfig := resilience.DefaultDegradationConfig()
	degradationConfig.RecoveryInterval = 5 * time.Second
	
	ehe.degradationMgr = resilience.NewGracefulDegradation(degradationConfig, nil, ehe.logger)

	return nil
}

// SetupMonitoring sets up monitoring components
func (ehe *ErrorHandlingExperiments) SetupMonitoring() error {
	thresholds := &monitoring.SystemThresholds{
		CPUWarning:    70.0,
		CPUCritical:   85.0,
		MemoryWarning: 75.0,
		MemoryCritical: 90.0,
		DiskWarning:   80.0,
		DiskCritical:  95.0,
		QueueSizeWarning: 100,
		QueueSizeCritical: 500,
		ErrorRateWarning: 5.0,
		ErrorRateCritical: 10.0,
	}

	alertConfig := &monitoring.AlertManagerConfig{
		CooldownPeriod: 30 * time.Second,
		MaxAlerts:      100,
	}

	resourceConfig := &monitoring.ResourceMonitorConfig{
		CheckInterval:    1 * time.Second,
		MetricsRetention: 5 * time.Minute,
		EnableCPU:        true,
		EnableMemory:     true,
		EnableDisk:       true,
		EnableDocker:     true,
	}

	var err error
	ehe.resourceMonitor, err = monitoring.NewResourceMonitor(resourceConfig, thresholds, alertConfig, nil, ehe.logger)
	if err != nil {
		return fmt.Errorf("failed to create resource monitor: %w", err)
	}

	return nil
}

// Cleanup cleans up all experiment resources
func (ehe *ErrorHandlingExperiments) Cleanup() {
	ehe.injector.StopAllInjections()
	
	if ehe.errorAggregator != nil {
		ehe.errorAggregator.Stop(context.Background())
	}
	
	if ehe.degradationMgr != nil {
		ehe.degradationMgr.Stop()
	}
	
	for _, ls := range ehe.loadShedders {
		ls.Stop()
	}
	
	if ehe.resourceMonitor != nil {
		ehe.resourceMonitor.Stop(context.Background())
	}
}

// CreateExperiments creates all error handling chaos experiments
func (ehe *ErrorHandlingExperiments) CreateExperiments() []*ChaosExperiment {
	experiments := []*ChaosExperiment{
		ehe.createCircuitBreakerExperiment(),
		ehe.createRetryLogicExperiment(),
		ehe.createLoadSheddingExperiment(),
		ehe.createGracefulDegradationExperiment(),
		ehe.createErrorReportingExperiment(),
		ehe.createResourceExhaustionExperiment(),
		ehe.createCascadingFailureExperiment(),
		ehe.createRecoveryValidationExperiment(),
	}

	return experiments
}

// createCircuitBreakerExperiment creates an experiment to test circuit breaker behavior
func (ehe *ErrorHandlingExperiments) createCircuitBreakerExperiment() *ChaosExperiment {
	return &ChaosExperiment{
		ID:          "circuit-breaker-test",
		Name:        "Circuit Breaker Behavior Test",
		Description: "Tests circuit breaker opening and closing under various failure conditions",
		Duration:    60 * time.Second,
		Config: map[string]interface{}{
			"failure_rate":     0.8,
			"test_duration":    "45s",
			"recovery_period":  "15s",
		},
		Setup: func(ctx context.Context) error {
			return ehe.SetupResilience()
		},
		Execute: func(ctx context.Context) error {
			cb := ehe.circuitBreakers["test-service"]
			if cb == nil {
				return fmt.Errorf("circuit breaker not found")
			}

			// Phase 1: Generate failures to trip circuit breaker
			ehe.logger.Info("phase 1: generating failures to trip circuit breaker")
			
			for i := 0; i < 10; i++ {
				err := cb.Execute(ctx, func(ctx context.Context) error {
					if i < 8 { // 80% failure rate
						return fmt.Errorf("simulated service failure %d", i)
					}
					return nil
				})
				
				if err != nil {
					ehe.logger.Debug("circuit breaker operation failed", "attempt", i, "error", err)
				}
				
				time.Sleep(100 * time.Millisecond)
			}

			// Verify circuit breaker is open
			if cb.GetState() != resilience.StateOpen {
				return fmt.Errorf("circuit breaker should be open, but is %s", cb.GetState())
			}

			// Phase 2: Wait for recovery
			ehe.logger.Info("phase 2: waiting for circuit breaker recovery")
			time.Sleep(15 * time.Second)

			// Phase 3: Test recovery with successful operations
			ehe.logger.Info("phase 3: testing recovery with successful operations")
			
			for i := 0; i < 5; i++ {
				err := cb.Execute(ctx, func(ctx context.Context) error {
					return nil // All successful
				})
				
				if err != nil {
					ehe.logger.Debug("recovery operation failed", "attempt", i, "error", err)
				}
				
				time.Sleep(100 * time.Millisecond)
			}

			// Verify circuit breaker is closed
			if cb.GetState() != resilience.StateClosed {
				return fmt.Errorf("circuit breaker should be closed after recovery, but is %s", cb.GetState())
			}

			return nil
		},
		Cleanup: func(ctx context.Context) error {
			if cb := ehe.circuitBreakers["test-service"]; cb != nil {
				cb.Reset()
			}
			return nil
		},
		Validate: func(ctx context.Context) error {
			cb := ehe.circuitBreakers["test-service"]
			stats := cb.GetStats()
			
			if stats.TotalRequests < 10 {
				return fmt.Errorf("expected at least 10 requests, got %d", stats.TotalRequests)
			}
			
			if stats.TotalFailures < 5 {
				return fmt.Errorf("expected at least 5 failures, got %d", stats.TotalFailures)
			}

			ehe.logger.Info("circuit breaker experiment validation successful",
				"total_requests", stats.TotalRequests,
				"total_failures", stats.TotalFailures,
				"state_changes", stats.StateChanges)
			
			return nil
		},
	}
}

// createRetryLogicExperiment creates an experiment to test retry logic
func (ehe *ErrorHandlingExperiments) createRetryLogicExperiment() *ChaosExperiment {
	return &ChaosExperiment{
		ID:          "retry-logic-test",
		Name:        "Retry Logic Behavior Test",
		Description: "Tests retry mechanisms with various failure patterns and jitter strategies",
		Duration:    45 * time.Second,
		Config: map[string]interface{}{
			"max_attempts":     5,
			"base_delay":       "100ms",
			"strategy":         "exponential_backoff",
		},
		Setup: func(ctx context.Context) error {
			return ehe.SetupResilience()
		},
		Execute: func(ctx context.Context) error {
			retryExecutor := ehe.retryExecutors["test-service"]
			if retryExecutor == nil {
				return fmt.Errorf("retry executor not found")
			}

			// Test 1: Eventual success after retries
			ehe.logger.Info("test 1: eventual success after retries")
			
			attemptCount := 0
			err := retryExecutor.Execute(ctx, "eventual-success", func(ctx context.Context, attempt int) error {
				attemptCount++
				if attempt < 3 {
					return fmt.Errorf("temporary failure %d", attempt)
				}
				return nil
			})

			if err != nil {
				return fmt.Errorf("eventual success test failed: %w", err)
			}

			if attemptCount != 3 {
				return fmt.Errorf("expected 3 attempts, got %d", attemptCount)
			}

			// Test 2: Max attempts exceeded
			ehe.logger.Info("test 2: max attempts exceeded")
			
			err = retryExecutor.Execute(ctx, "max-attempts", func(ctx context.Context, attempt int) error {
				return fmt.Errorf("persistent failure %d", attempt)
			})

			if err == nil {
				return fmt.Errorf("expected failure after max attempts")
			}

			// Test 3: Non-retryable error
			ehe.logger.Info("test 3: non-retryable error")
			
			attemptCount = 0
			err = retryExecutor.Execute(ctx, "non-retryable", func(ctx context.Context, attempt int) error {
				attemptCount++
				return fmt.Errorf("validation error") // Non-retryable
			})

			if err == nil {
				return fmt.Errorf("expected validation error")
			}

			if attemptCount != 1 {
				return fmt.Errorf("expected 1 attempt for non-retryable error, got %d", attemptCount)
			}

			return nil
		},
		Cleanup: func(ctx context.Context) error {
			if retryExecutor := ehe.retryExecutors["test-service"]; retryExecutor != nil {
				retryExecutor.Reset()
			}
			return nil
		},
		Validate: func(ctx context.Context) error {
			retryExecutor := ehe.retryExecutors["test-service"]
			stats := retryExecutor.GetStats()
			
			if stats.RetryStats.TotalAttempts < 6 { // At least 3 + 5 + 1 attempts
				return fmt.Errorf("expected at least 6 total attempts, got %d", stats.RetryStats.TotalAttempts)
			}
			
			if stats.RetryStats.TotalRetries < 2 { // At least 2 + 4 retries
				return fmt.Errorf("expected at least 2 total retries, got %d", stats.RetryStats.TotalRetries)
			}

			ehe.logger.Info("retry logic experiment validation successful",
				"total_attempts", stats.RetryStats.TotalAttempts,
				"total_retries", stats.RetryStats.TotalRetries,
				"successful_retries", stats.RetryStats.SuccessfulRetries)
			
			return nil
		},
	}
}

// createLoadSheddingExperiment creates an experiment to test load shedding
func (ehe *ErrorHandlingExperiments) createLoadSheddingExperiment() *ChaosExperiment {
	return &ChaosExperiment{
		ID:          "load-shedding-test",
		Name:        "Load Shedding Behavior Test",
		Description: "Tests load shedding under high concurrency and resource pressure",
		Duration:    30 * time.Second,
		Config: map[string]interface{}{
			"max_concurrent": 10,
			"request_count":  100,
		},
		Setup: func(ctx context.Context) error {
			return ehe.SetupResilience()
		},
		Execute: func(ctx context.Context) error {
			loadShedder := ehe.loadShedders["test-service"]
			if loadShedder == nil {
				return fmt.Errorf("load shedder not found")
			}

			// Test concurrent request handling
			ehe.logger.Info("testing concurrent request handling")
			
			var wg sync.WaitGroup
			acceptedCount := int64(0)
			shedCount := int64(0)
			
			// Launch many concurrent requests
			for i := 0; i < 50; i++ {
				wg.Add(1)
				go func(i int) {
					defer wg.Done()
					
					priority := resilience.PriorityNormal
					if i%10 == 0 {
						priority = resilience.PriorityCritical
					}
					
					decision := loadShedder.ShouldAcceptRequest(priority)
					if decision.Allow {
						acceptedCount++
						// Simulate request processing
						time.Sleep(100 * time.Millisecond)
						loadShedder.CompleteRequest()
					} else {
						shedCount++
					}
				}(i)
				
				time.Sleep(10 * time.Millisecond) // Stagger requests
			}
			
			wg.Wait()

			if acceptedCount == 0 {
				return fmt.Errorf("no requests were accepted")
			}

			if shedCount == 0 {
				return fmt.Errorf("no requests were shed (load shedding not activated)")
			}

			ehe.logger.Info("load shedding test completed",
				"accepted", acceptedCount,
				"shed", shedCount)

			return nil
		},
		Cleanup: func(ctx context.Context) error {
			if loadShedder := ehe.loadShedders["test-service"]; loadShedder != nil {
				loadShedder.Reset()
			}
			return nil
		},
		Validate: func(ctx context.Context) error {
			loadShedder := ehe.loadShedders["test-service"]
			stats := loadShedder.GetStats()
			
			if stats.TotalRequests != 50 {
				return fmt.Errorf("expected 50 total requests, got %d", stats.TotalRequests)
			}
			
			if stats.ShedRequests == 0 {
				return fmt.Errorf("expected some shed requests, got %d", stats.ShedRequests)
			}

			ehe.logger.Info("load shedding experiment validation successful",
				"total_requests", stats.TotalRequests,
				"accepted_requests", stats.AcceptedRequests,
				"shed_requests", stats.ShedRequests)
			
			return nil
		},
	}
}

// createGracefulDegradationExperiment creates an experiment to test graceful degradation
func (ehe *ErrorHandlingExperiments) createGracefulDegradationExperiment() *ChaosExperiment {
	return &ChaosExperiment{
		ID:          "graceful-degradation-test",
		Name:        "Graceful Degradation Test",
		Description: "Tests automatic degradation levels under resource pressure",
		Duration:    40 * time.Second,
		Setup: func(ctx context.Context) error {
			return ehe.SetupResilience()
		},
		Execute: func(ctx context.Context) error {
			if ehe.degradationMgr == nil {
				return fmt.Errorf("degradation manager not found")
			}

			// Start in normal mode
			if ehe.degradationMgr.GetCurrentLevel() != resilience.LevelNormal {
				return fmt.Errorf("expected normal level at start")
			}

			// Test manual degradation
			ehe.logger.Info("testing manual degradation")
			ehe.degradationMgr.SetLevel(resilience.LevelLimited, "chaos test")
			
			if ehe.degradationMgr.GetCurrentLevel() != resilience.LevelLimited {
				return fmt.Errorf("expected limited level after manual set")
			}

			// Test feature availability at different levels
			features := []resilience.FeatureName{
				resilience.FeatureLogging,
				resilience.FeatureMetrics,
				resilience.FeatureComplexTasks,
			}

			for _, feature := range features {
				enabled := ehe.degradationMgr.IsFeatureEnabled(feature)
				ehe.logger.Info("feature availability check",
					"feature", feature,
					"enabled", enabled,
					"level", ehe.degradationMgr.GetCurrentLevel())
			}

			// Test degradation to minimal
			ehe.degradationMgr.SetLevel(resilience.LevelMinimal, "chaos test - minimal")
			time.Sleep(2 * time.Second)

			// Test degradation to emergency
			ehe.degradationMgr.SetLevel(resilience.LevelEmergency, "chaos test - emergency")
			time.Sleep(2 * time.Second)

			// Test recovery
			ehe.logger.Info("testing recovery to normal")
			ehe.degradationMgr.SetLevel(resilience.LevelNormal, "chaos test - recovery")

			if ehe.degradationMgr.GetCurrentLevel() != resilience.LevelNormal {
				return fmt.Errorf("expected normal level after recovery")
			}

			return nil
		},
		Cleanup: func(ctx context.Context) error {
			if ehe.degradationMgr != nil {
				ehe.degradationMgr.Reset()
			}
			return nil
		},
		Validate: func(ctx context.Context) error {
			stats := ehe.degradationMgr.GetStats()
			
			if len(stats.LevelChangeHistory) < 4 {
				return fmt.Errorf("expected at least 4 level changes, got %d", len(stats.LevelChangeHistory))
			}
			
			if stats.CurrentLevel != resilience.LevelNormal {
				return fmt.Errorf("expected to end at normal level, got %s", stats.CurrentLevel)
			}

			ehe.logger.Info("graceful degradation experiment validation successful",
				"current_level", stats.CurrentLevel,
				"level_changes", len(stats.LevelChangeHistory),
				"enabled_features", len(stats.EnabledFeatures))
			
			return nil
		},
	}
}

// createErrorReportingExperiment creates an experiment to test error reporting
func (ehe *ErrorHandlingExperiments) createErrorReportingExperiment() *ChaosExperiment {
	return &ChaosExperiment{
		ID:          "error-reporting-test",
		Name:        "Error Reporting System Test",
		Description: "Tests error aggregation and reporting under various error patterns",
		Duration:    35 * time.Second,
		Setup: func(ctx context.Context) error {
			return ehe.SetupErrorReporting()
		},
		Execute: func(ctx context.Context) error {
			if ehe.errorAggregator == nil {
				return fmt.Errorf("error aggregator not found")
			}

			// Generate various types of errors
			errorTypes := []executor.ErrorType{
				executor.ErrorTypeTimeout,
				executor.ErrorTypeResource,
				executor.ErrorTypeValidation,
				executor.ErrorTypeSecurity,
				executor.ErrorTypeNetwork,
			}

			ehe.logger.Info("generating diverse error patterns")
			
			for i := 0; i < 50; i++ {
				errorType := errorTypes[i%len(errorTypes)]
				execError := &executor.ExecutionError{
					Type:        errorType,
					Code:        fmt.Sprintf("%s_%03d", errorType, i%10),
					Message:     fmt.Sprintf("Test error %d", i),
					Retryable:   i%3 == 0,
					TaskID:      fmt.Sprintf("task-%d", i%20),
					ExecutionID: fmt.Sprintf("exec-%d", i),
					Timestamp:   time.Now(),
					Context: map[string]interface{}{
						"test_index": i,
						"pattern":    "chaos_test",
					},
				}

				ehe.errorAggregator.RecordError(ctx, execError, execError.TaskID, fmt.Sprintf("user-%d", i%10))
				
				// Add some randomness to timing
				if i%5 == 0 {
					time.Sleep(50 * time.Millisecond)
				}
			}

			// Wait for aggregation
			time.Sleep(2 * time.Second)

			// Generate a report
			timeRange := reporting.TimeRange{
				Start: time.Now().Add(-1 * time.Hour),
				End:   time.Now(),
			}

			report, err := ehe.errorAggregator.GenerateReport(ctx, timeRange)
			if err != nil {
				return fmt.Errorf("failed to generate report: %w", err)
			}

			if report.TotalErrors != 50 {
				return fmt.Errorf("expected 50 errors in report, got %d", report.TotalErrors)
			}

			if len(report.ErrorsByType) != len(errorTypes) {
				return fmt.Errorf("expected %d error types, got %d", len(errorTypes), len(report.ErrorsByType))
			}

			ehe.logger.Info("error reporting test completed",
				"total_errors", report.TotalErrors,
				"unique_errors", report.UniqueErrors,
				"error_types", len(report.ErrorsByType),
				"recommendations", len(report.Recommendations))

			return nil
		},
		Cleanup: func(ctx context.Context) error {
			return nil
		},
		Validate: func(ctx context.Context) error {
			stats := ehe.errorAggregator.GetStats()
			
			totalErrorTypes := stats["total_error_types"].(int)
			if totalErrorTypes < 5 {
				return fmt.Errorf("expected at least 5 error types, got %d", totalErrorTypes)
			}
			
			recentErrorsCount := stats["recent_errors_count"].(int)
			if recentErrorsCount != 50 {
				return fmt.Errorf("expected 50 recent errors, got %d", recentErrorsCount)
			}

			ehe.logger.Info("error reporting experiment validation successful",
				"total_error_types", totalErrorTypes,
				"recent_errors_count", recentErrorsCount)
			
			return nil
		},
	}
}

// createResourceExhaustionExperiment creates an experiment to test behavior under resource exhaustion
func (ehe *ErrorHandlingExperiments) createResourceExhaustionExperiment() *ChaosExperiment {
	return &ChaosExperiment{
		ID:          "resource-exhaustion-test",
		Name:        "Resource Exhaustion Behavior Test",
		Description: "Tests system behavior under CPU, memory, and disk resource exhaustion",
		Duration:    60 * time.Second,
		Setup: func(ctx context.Context) error {
			if err := ehe.SetupResilience(); err != nil {
				return err
			}
			return ehe.SetupMonitoring()
		},
		Execute: func(ctx context.Context) error {
			// Test CPU exhaustion
			ehe.logger.Info("testing CPU exhaustion")
			cpuCtx, cpuCancel := context.WithTimeout(ctx, 15*time.Second)
			go ehe.injector.InjectResourceExhaustion(cpuCtx, "cpu", 70.0, 10*time.Second)
			time.Sleep(12 * time.Second)
			cpuCancel()

			// Test memory exhaustion
			ehe.logger.Info("testing memory exhaustion")
			memCtx, memCancel := context.WithTimeout(ctx, 15*time.Second)
			go ehe.injector.InjectResourceExhaustion(memCtx, "memory", 60.0, 10*time.Second)
			time.Sleep(12 * time.Second)
			memCancel()

			// Test system recovery
			ehe.logger.Info("testing system recovery")
			time.Sleep(10 * time.Second)

			return nil
		},
		Cleanup: func(ctx context.Context) error {
			ehe.injector.StopAllInjections()
			return nil
		},
		Validate: func(ctx context.Context) error {
			// Validate that monitoring detected the resource exhaustion
			if ehe.resourceMonitor != nil {
				stats := ehe.resourceMonitor.GetStats()
				ehe.logger.Info("resource monitoring stats", "stats", stats)
			}

			return nil
		},
	}
}

// createCascadingFailureExperiment creates an experiment to test cascading failure handling
func (ehe *ErrorHandlingExperiments) createCascadingFailureExperiment() *ChaosExperiment {
	return &ChaosExperiment{
		ID:          "cascading-failure-test",
		Name:        "Cascading Failure Prevention Test",
		Description: "Tests system resilience against cascading failures",
		Duration:    45 * time.Second,
		Setup: func(ctx context.Context) error {
			if err := ehe.SetupResilience(); err != nil {
				return err
			}
			return ehe.SetupErrorReporting()
		},
		Execute: func(ctx context.Context) error {
			// Simulate a cascade of failures across multiple components
			ehe.logger.Info("simulating cascading failures")

			// Start with network issues
			netCtx, netCancel := context.WithTimeout(ctx, 10*time.Second)
			go ehe.injector.InjectNetworkLatency(netCtx, 500*time.Millisecond, 8*time.Second)

			// Add resource pressure
			time.Sleep(2 * time.Second)
			resCtx, resCancel := context.WithTimeout(ctx, 15*time.Second)
			go ehe.injector.InjectResourceExhaustion(resCtx, "cpu", 80.0, 12*time.Second)

			// Generate errors that should trigger circuit breakers
			cb := ehe.circuitBreakers["test-service"]
			for i := 0; i < 20; i++ {
				if cb != nil {
					cb.Execute(ctx, func(ctx context.Context) error {
						if i < 15 {
							return fmt.Errorf("cascading failure %d", i)
						}
						return nil
					})
				}
				time.Sleep(200 * time.Millisecond)
			}

			netCancel()
			resCancel()

			// Allow system to recover
			time.Sleep(10 * time.Second)

			return nil
		},
		Cleanup: func(ctx context.Context) error {
			ehe.injector.StopAllInjections()
			return nil
		},
		Validate: func(ctx context.Context) error {
			// Validate that circuit breakers activated
			if cb := ehe.circuitBreakers["test-service"]; cb != nil {
				stats := cb.GetStats()
				if stats.TotalFailures < 10 {
					return fmt.Errorf("expected significant failures, got %d", stats.TotalFailures)
				}
			}

			return nil
		},
	}
}

// createRecoveryValidationExperiment creates an experiment to validate recovery mechanisms
func (ehe *ErrorHandlingExperiments) createRecoveryValidationExperiment() *ChaosExperiment {
	return &ChaosExperiment{
		ID:          "recovery-validation-test",
		Name:        "Recovery Mechanism Validation Test",
		Description: "Tests that all error handling mechanisms properly recover after failures",
		Duration:    50 * time.Second,
		Setup: func(ctx context.Context) error {
			if err := ehe.SetupResilience(); err != nil {
				return err
			}
			if err := ehe.SetupErrorReporting(); err != nil {
				return err
			}
			return ehe.SetupMonitoring()
		},
		Execute: func(ctx context.Context) error {
			// Phase 1: Stress all systems
			ehe.logger.Info("phase 1: stressing all systems")
			
			// Trigger circuit breaker
			cb := ehe.circuitBreakers["test-service"]
			for i := 0; i < 10; i++ {
				if cb != nil {
					cb.Execute(ctx, func(ctx context.Context) error {
						return fmt.Errorf("stress failure %d", i)
					})
				}
			}

			// Trigger load shedding
			ls := ehe.loadShedders["test-service"]
			for i := 0; i < 20; i++ {
				if ls != nil {
					ls.ShouldAcceptRequest(resilience.PriorityNormal)
				}
			}

			// Force degradation
			if ehe.degradationMgr != nil {
				ehe.degradationMgr.SetLevel(resilience.LevelEmergency, "recovery test")
			}

			// Phase 2: Wait for systems to stabilize
			ehe.logger.Info("phase 2: waiting for stabilization")
			time.Sleep(15 * time.Second)

			// Phase 3: Validate recovery
			ehe.logger.Info("phase 3: validating recovery")

			// Reset systems and test normal operation
			if cb != nil {
				cb.ForceClose()
			}
			if ls != nil {
				ls.Reset()
			}
			if ehe.degradationMgr != nil {
				ehe.degradationMgr.SetLevel(resilience.LevelNormal, "recovery complete")
			}

			// Test normal operations
			for i := 0; i < 5; i++ {
				if cb != nil {
					err := cb.Execute(ctx, func(ctx context.Context) error {
						return nil // All successful
					})
					if err != nil {
						return fmt.Errorf("recovery validation failed: %w", err)
					}
				}
			}

			return nil
		},
		Cleanup: func(ctx context.Context) error {
			// Reset all systems to clean state
			for _, cb := range ehe.circuitBreakers {
				cb.Reset()
			}
			for _, ls := range ehe.loadShedders {
				ls.Reset()
			}
			if ehe.degradationMgr != nil {
				ehe.degradationMgr.Reset()
			}
			return nil
		},
		Validate: func(ctx context.Context) error {
			// Validate that all systems are back to normal state
			if cb := ehe.circuitBreakers["test-service"]; cb != nil {
				if cb.GetState() != resilience.StateClosed {
					return fmt.Errorf("circuit breaker not recovered: %s", cb.GetState())
				}
			}

			if ehe.degradationMgr != nil {
				if ehe.degradationMgr.GetCurrentLevel() != resilience.LevelNormal {
					return fmt.Errorf("degradation manager not recovered: %s", ehe.degradationMgr.GetCurrentLevel())
				}
			}

			ehe.logger.Info("recovery validation experiment successful")
			return nil
		},
	}
}
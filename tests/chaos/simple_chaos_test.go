//go:build integration

package chaos

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/voidrunnerhq/voidrunner/internal/executor"
	"github.com/voidrunnerhq/voidrunner/internal/reporting"
	"github.com/voidrunnerhq/voidrunner/internal/resilience"
)

func TestSimpleChaosFramework(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	t.Run("BasicChaosRunner", func(t *testing.T) {
		runner := NewChaosRunner(logger)
		assert.NotNil(t, runner)

		// Test basic experiment registration and execution
		experiment := &ChaosExperiment{
			ID:          "simple-test",
			Name:        "Simple Test",
			Description: "A basic chaos test",
			Duration:    2 * time.Second,
			Setup: func(ctx context.Context) error {
				return nil
			},
			Execute: func(ctx context.Context) error {
				time.Sleep(500 * time.Millisecond)
				return nil
			},
			Cleanup: func(ctx context.Context) error {
				return nil
			},
			Validate: func(ctx context.Context) error {
				return nil
			},
		}

		runner.RegisterExperiment(experiment)

		ctx := context.Background()
		result, err := runner.RunExperiment(ctx, "simple-test")
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.True(t, result.Success)
		assert.Equal(t, "simple-test", result.ExperimentID)
		assert.True(t, result.Duration >= 500*time.Millisecond)

		t.Logf("Simple chaos test completed successfully in %v", result.Duration)
	})

	t.Run("FailureInjectorBasics", func(t *testing.T) {
		injector := NewFailureInjector(logger)
		assert.NotNil(t, injector)

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		// Test network latency injection
		err := injector.InjectNetworkLatency(ctx, 100*time.Millisecond, 1*time.Second)
		assert.NoError(t, err)

		// Test stopping all injections
		injector.StopAllInjections()
		assert.Empty(t, injector.GetActiveInjections())

		t.Log("Failure injector basic tests completed")
	})

	t.Run("LoggingObserverBasics", func(t *testing.T) {
		observer := NewLoggingObserver(logger)
		assert.NotNil(t, observer)

		experiment := &ChaosExperiment{
			ID:          "observer-test",
			Name:        "Observer Test",
			Description: "Test for logging observer",
			Duration:    1 * time.Second,
		}

		result := &ChaosResult{
			ExperimentID: "observer-test",
			StartTime:    time.Now(),
			EndTime:      time.Now().Add(1 * time.Second),
			Duration:     1 * time.Second,
			Success:      true,
		}

		observation := &Observation{
			Timestamp: time.Now(),
			Type:      "test",
			Severity:  "info",
			Message:   "Test observation",
			Component: "test-component",
		}

		// These should not panic
		observer.OnExperimentStart(experiment)
		observer.OnExperimentEnd(result)
		observer.OnObservation(observation)

		t.Log("Logging observer basic tests completed")
	})
}

func TestSimpleErrorHandlingChaos(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping error handling chaos tests in short mode")
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	t.Run("CircuitBreakerChaos", func(t *testing.T) {
		config := resilience.DefaultCircuitBreakerConfig()
		config.FailureThreshold = 3
		config.OpenTimeout = 5 * time.Second

		cb, err := resilience.NewCircuitBreaker("chaos-test", config, logger)
		require.NoError(t, err)
		defer cb.Reset()

		// Create a chaos experiment for circuit breaker
		experiment := &ChaosExperiment{
			ID:          "circuit-breaker-chaos",
			Name:        "Circuit Breaker Chaos Test",
			Description: "Tests circuit breaker under chaos conditions",
			Duration:    15 * time.Second,
			Execute: func(ctx context.Context) error {
				// Phase 1: Generate failures to trigger circuit breaker
				t.Log("Phase 1: Generating failures to trigger circuit breaker")
				for i := 0; i < 5; i++ {
					err := cb.Execute(ctx, func(ctx context.Context) error {
						return fmt.Errorf("chaos failure %d", i)
					})
					if err != nil {
						t.Logf("Expected failure %d: %v", i, err)
					}
					time.Sleep(100 * time.Millisecond)
				}

				// Check if circuit breaker is open
				if cb.GetState() == resilience.StateOpen {
					t.Log("Circuit breaker successfully opened")
				}

				// Phase 2: Wait for recovery
				t.Log("Phase 2: Waiting for circuit breaker recovery")
				time.Sleep(6 * time.Second)

				// Phase 3: Test recovery
				t.Log("Phase 3: Testing recovery with successful operations")
				for i := 0; i < 3; i++ {
					err := cb.Execute(ctx, func(ctx context.Context) error {
						return nil // Success
					})
					if err != nil {
						t.Logf("Recovery attempt %d failed: %v", i, err)
					} else {
						t.Logf("Recovery attempt %d succeeded", i)
					}
					time.Sleep(100 * time.Millisecond)
				}

				return nil
			},
			Validate: func(ctx context.Context) error {
				stats := cb.GetStats()
				if stats.TotalRequests < 5 {
					return fmt.Errorf("expected at least 5 requests, got %d", stats.TotalRequests)
				}
				t.Logf("Circuit breaker stats: %+v", stats)
				return nil
			},
		}

		runner := NewChaosRunner(logger)
		runner.RegisterExperiment(experiment)

		result, err := runner.RunExperiment(context.Background(), "circuit-breaker-chaos")
		require.NoError(t, err)
		assert.True(t, result.Success)

		t.Logf("Circuit breaker chaos test completed: %v", result.Duration)
	})

	t.Run("RetryLogicChaos", func(t *testing.T) {
		config := resilience.DefaultRetryStrategyConfig()
		config.MaxAttempts = 3
		config.BaseDelay = 100 * time.Millisecond

		retryExecutor, err := resilience.NewRetryExecutor(config, logger)
		require.NoError(t, err)
		defer retryExecutor.Reset()

		experiment := &ChaosExperiment{
			ID:          "retry-chaos",
			Name:        "Retry Logic Chaos Test",
			Description: "Tests retry logic under chaos conditions",
			Duration:    10 * time.Second,
			Execute: func(ctx context.Context) error {
				// Test eventual success
				t.Log("Testing eventual success scenario")
				attemptCount := 0
				err := retryExecutor.Execute(ctx, "eventual-success", func(ctx context.Context, attempt int) error {
					attemptCount++
					if attempt < 3 {
						return fmt.Errorf("temporary failure attempt %d", attempt)
					}
					return nil
				})

				if err != nil {
					return fmt.Errorf("eventual success test failed: %w", err)
				}

				t.Logf("Eventual success completed in %d attempts", attemptCount)

				// Test max attempts exceeded
				t.Log("Testing max attempts exceeded scenario")
				err = retryExecutor.Execute(ctx, "max-attempts", func(ctx context.Context, attempt int) error {
					return fmt.Errorf("persistent failure attempt %d", attempt)
				})

				if err == nil {
					return fmt.Errorf("expected failure after max attempts")
				}

				t.Logf("Max attempts test completed as expected: %v", err)
				return nil
			},
			Validate: func(ctx context.Context) error {
				stats := retryExecutor.GetStats()
				if stats.RetryStats.TotalAttempts < 5 {
					return fmt.Errorf("expected at least 5 total attempts")
				}
				t.Logf("Retry executor stats: total_attempts=%d, total_retries=%d",
					stats.RetryStats.TotalAttempts, stats.RetryStats.TotalRetries)
				return nil
			},
		}

		runner := NewChaosRunner(logger)
		runner.RegisterExperiment(experiment)

		result, err := runner.RunExperiment(context.Background(), "retry-chaos")
		require.NoError(t, err)
		assert.True(t, result.Success)

		t.Logf("Retry logic chaos test completed: %v", result.Duration)
	})

	t.Run("ErrorReportingChaos", func(t *testing.T) {
		config := reporting.DefaultErrorAggregatorConfig()
		aggregator := reporting.NewErrorAggregator(config, logger)
		defer aggregator.Stop(context.Background())

		experiment := &ChaosExperiment{
			ID:          "error-reporting-chaos",
			Name:        "Error Reporting Chaos Test",
			Description: "Tests error reporting under chaos conditions",
			Duration:    8 * time.Second,
			Execute: func(ctx context.Context) error {
				t.Log("Generating diverse error patterns")

				// Generate various error types
				errorTypes := []executor.ErrorType{
					executor.ErrorTypeTimeout,
					executor.ErrorTypeResource,
					executor.ErrorTypeValidation,
				}

				for i := 0; i < 30; i++ {
					errorType := errorTypes[i%len(errorTypes)]
					execError := &executor.ExecutionError{
						Type:        errorType,
						Code:        fmt.Sprintf("%s_%d", errorType, i%5),
						Message:     fmt.Sprintf("Chaos test error %d", i),
						Retryable:   i%2 == 0,
						TaskID:      fmt.Sprintf("chaos-task-%d", i%10),
						ExecutionID: fmt.Sprintf("chaos-exec-%d", i),
						Timestamp:   time.Now(),
						Context: map[string]interface{}{
							"chaos_test": true,
							"error_id":   i,
						},
					}

					aggregator.RecordError(ctx, execError, execError.TaskID, fmt.Sprintf("chaos-user-%d", i%5))

					if i%10 == 0 {
						time.Sleep(50 * time.Millisecond)
					}
				}

				// Wait for aggregation
				time.Sleep(1 * time.Second)

				// Generate a report
				timeRange := reporting.TimeRange{
					Start: time.Now().Add(-1 * time.Hour),
					End:   time.Now(),
				}

				report, err := aggregator.GenerateReport(ctx, timeRange)
				if err != nil {
					return fmt.Errorf("failed to generate report: %w", err)
				}

				t.Logf("Generated report: %d total errors, %d unique errors",
					report.TotalErrors, report.UniqueErrors)

				if report.TotalErrors != 30 {
					return fmt.Errorf("expected 30 errors, got %d", report.TotalErrors)
				}

				return nil
			},
			Validate: func(ctx context.Context) error {
				stats := aggregator.GetStats()
				recentErrorsCount := stats["recent_errors_count"].(int)
				if recentErrorsCount != 30 {
					return fmt.Errorf("expected 30 recent errors, got %d", recentErrorsCount)
				}
				t.Logf("Error aggregator stats: %+v", stats)
				return nil
			},
		}

		runner := NewChaosRunner(logger)
		runner.RegisterExperiment(experiment)

		result, err := runner.RunExperiment(context.Background(), "error-reporting-chaos")
		require.NoError(t, err)
		assert.True(t, result.Success)

		t.Logf("Error reporting chaos test completed: %v", result.Duration)
	})
}

func TestChaosExperimentSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping chaos experiment suite in short mode")
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	t.Run("RunMultipleExperiments", func(t *testing.T) {
		runner := NewChaosRunner(logger)
		observer := NewLoggingObserver(logger)
		runner.AddObserver(observer)

		// Create multiple simple experiments
		experiments := []*ChaosExperiment{
			{
				ID:          "experiment-1",
				Name:        "Fast Test",
				Description: "A fast chaos experiment",
				Duration:    3 * time.Second,
				Execute: func(ctx context.Context) error {
					time.Sleep(1 * time.Second)
					return nil
				},
				Validate: func(ctx context.Context) error {
					return nil
				},
			},
			{
				ID:          "experiment-2",
				Name:        "Medium Test",
				Description: "A medium chaos experiment",
				Duration:    5 * time.Second,
				Execute: func(ctx context.Context) error {
					time.Sleep(2 * time.Second)
					return nil
				},
				Validate: func(ctx context.Context) error {
					return nil
				},
			},
		}

		// Register experiments
		for _, exp := range experiments {
			runner.RegisterExperiment(exp)
		}

		// Run experiments
		successCount := 0
		for _, exp := range experiments {
			t.Logf("Running experiment: %s", exp.ID)
			result, err := runner.RunExperiment(context.Background(), exp.ID)

			if err != nil {
				t.Logf("Experiment %s failed: %v", exp.ID, err)
			} else {
				t.Logf("Experiment %s completed: success=%v, duration=%v",
					exp.ID, result.Success, result.Duration)
				if result.Success {
					successCount++
				}
			}
		}

		// Validate results
		assert.Equal(t, len(experiments), successCount, "All experiments should succeed")

		results := runner.GetResults()
		assert.Len(t, results, len(experiments))

		t.Logf("Chaos experiment suite completed: %d/%d experiments successful",
			successCount, len(experiments))
	})
}

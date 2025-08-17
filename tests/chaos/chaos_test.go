//go:build integration

package chaos

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChaosFramework(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	t.Run("ChaosRunner", func(t *testing.T) {
		runner := NewChaosRunner(logger)
		assert.NotNil(t, runner)

		// Test basic experiment registration and execution
		experiment := &ChaosExperiment{
			ID:          "test-experiment",
			Name:        "Test Experiment",
			Description: "A simple test experiment",
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
		result, err := runner.RunExperiment(ctx, "test-experiment")
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.True(t, result.Success)
		assert.Equal(t, "test-experiment", result.ExperimentID)
		assert.True(t, result.Duration >= 500*time.Millisecond)
	})

	t.Run("FailureInjector", func(t *testing.T) {
		injector := NewFailureInjector(logger)
		assert.NotNil(t, injector)

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		// Test network latency injection
		err := injector.InjectNetworkLatency(ctx, 100*time.Millisecond, 1*time.Second)
		assert.NoError(t, err)

		// Test resource exhaustion injection
		err = injector.InjectResourceExhaustion(ctx, "cpu", 50.0, 1*time.Second)
		assert.NoError(t, err)

		// Test memory exhaustion injection
		err = injector.InjectResourceExhaustion(ctx, "memory", 30.0, 1*time.Second)
		assert.NoError(t, err)

		// Test stopping all injections
		injector.StopAllInjections()
		assert.Empty(t, injector.GetActiveInjections())
	})

	t.Run("LoggingObserver", func(t *testing.T) {
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
	})
}

func TestErrorHandlingExperiments(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping chaos engineering tests in short mode")
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	t.Run("ErrorHandlingExperimentsSetup", func(t *testing.T) {
		experiments := NewErrorHandlingExperiments(logger)
		assert.NotNil(t, experiments)

		// Test setup methods
		err := experiments.SetupErrorReporting()
		assert.NoError(t, err)

		err = experiments.SetupResilience()
		assert.NoError(t, err)

		err = experiments.SetupMonitoring()
		assert.NoError(t, err)

		// Cleanup
		experiments.Cleanup()
	})

	t.Run("CreateExperiments", func(t *testing.T) {
		experiments := NewErrorHandlingExperiments(logger)
		defer experiments.Cleanup()

		chaosExperiments := experiments.CreateExperiments()
		assert.NotEmpty(t, chaosExperiments)
		assert.Len(t, chaosExperiments, 8) // We created 8 experiments

		// Verify all experiments have required fields
		for _, exp := range chaosExperiments {
			assert.NotEmpty(t, exp.ID)
			assert.NotEmpty(t, exp.Name)
			assert.NotEmpty(t, exp.Description)
			assert.True(t, exp.Duration > 0)
			assert.NotNil(t, exp.Execute)
		}
	})
}

func TestFullChaosEngineeringSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping full chaos engineering suite in short mode")
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	t.Run("RunAllErrorHandlingExperiments", func(t *testing.T) {
		// Create chaos runner
		runner := NewChaosRunner(logger)
		observer := NewLoggingObserver(logger)
		runner.AddObserver(observer)

		// Create error handling experiments
		experiments := NewErrorHandlingExperiments(logger)
		defer experiments.Cleanup()

		chaosExperiments := experiments.CreateExperiments()

		// Register all experiments
		for _, exp := range chaosExperiments {
			runner.RegisterExperiment(exp)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()

		t.Logf("Running %d chaos experiments...", len(chaosExperiments))

		// Run specific critical experiments (subset for faster testing)
		criticalExperiments := []string{
			"circuit-breaker-test",
			"retry-logic-test",
			"error-reporting-test",
		}

		successCount := 0
		totalExperiments := len(criticalExperiments)

		for _, experimentID := range criticalExperiments {
			t.Logf("Running experiment: %s", experimentID)
			
			result, err := runner.RunExperiment(ctx, experimentID)
			
			if err != nil {
				t.Logf("Experiment %s failed with error: %v", experimentID, err)
				continue
			}

			require.NotNil(t, result)
			t.Logf("Experiment %s completed: success=%v, duration=%v, errors=%d",
				experimentID, result.Success, result.Duration, result.ErrorCount)

			if result.Success {
				successCount++
			} else {
				t.Logf("Experiment %s failed: %s", experimentID, result.Error)
			}

			// Log observations
			for _, obs := range result.Observations {
				t.Logf("  [%s] %s: %s", obs.Severity, obs.Type, obs.Message)
			}
		}

		// Validate overall results
		successRate := float64(successCount) / float64(totalExperiments)
		t.Logf("Chaos engineering results: %d/%d experiments successful (%.1f%%)",
			successCount, totalExperiments, successRate*100)

		// We expect at least 80% success rate for critical experiments
		assert.True(t, successRate >= 0.8,
			"Expected at least 80%% success rate, got %.1f%%", successRate*100)

		// Verify we have some results
		results := runner.GetResults()
		assert.Len(t, results, totalExperiments)

		// Check that we have observations recorded
		totalObservations := 0
		for _, result := range results {
			totalObservations += len(result.Observations)
		}
		assert.True(t, totalObservations > 0, "Expected some observations to be recorded")

		t.Logf("Total observations recorded: %d", totalObservations)
	})

	t.Run("StressTestErrorHandling", func(t *testing.T) {
		// This test runs a subset of experiments under time pressure
		runner := NewChaosRunner(logger)
		experiments := NewErrorHandlingExperiments(logger)
		defer experiments.Cleanup()

		// Create a shortened version of the load shedding test
		stressExperiment := &ChaosExperiment{
			ID:          "stress-test",
			Name:        "Stress Test Error Handling",
			Description: "Rapid stress test of error handling mechanisms",
			Duration:    10 * time.Second,
			Setup: func(ctx context.Context) error {
				return experiments.SetupResilience()
			},
			Execute: func(ctx context.Context) error {
				// Rapid fire operations to stress test
				for i := 0; i < 100; i++ {
					// Test circuit breakers under stress
					if cb := experiments.circuitBreakers["test-service"]; cb != nil {
						cb.Execute(ctx, func(ctx context.Context) error {
							if i%10 < 7 { // 70% failure rate
								return assert.AnError
							}
							return nil
						})
					}

					// Test load shedding under stress
					if ls := experiments.loadShedders["test-service"]; ls != nil {
						decision := ls.ShouldAcceptRequest(resilience.PriorityNormal)
						if decision.Allow {
							ls.CompleteRequest()
						}
					}

					if i%10 == 0 {
						time.Sleep(10 * time.Millisecond)
					}
				}
				return nil
			},
			Cleanup: func(ctx context.Context) error {
				return nil
			},
			Validate: func(ctx context.Context) error {
				// Just verify systems are still responsive
				if cb := experiments.circuitBreakers["test-service"]; cb != nil {
					stats := cb.GetStats()
					assert.True(t, stats.TotalRequests > 50)
				}
				return nil
			},
		}

		runner.RegisterExperiment(stressExperiment)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		result, err := runner.RunExperiment(ctx, "stress-test")
		require.NoError(t, err)
		assert.NotNil(t, result)

		t.Logf("Stress test completed: success=%v, duration=%v, observations=%d",
			result.Success, result.Duration, len(result.Observations))

		// The stress test should complete successfully even if individual operations fail
		assert.True(t, result.Success)
	})
}

func TestChaosExperimentRecovery(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping recovery tests in short mode")
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	t.Run("RecoveryAfterFailure", func(t *testing.T) {
		runner := NewChaosRunner(logger)
		experiments := NewErrorHandlingExperiments(logger)
		defer experiments.Cleanup()

		// Create an experiment that fails but cleans up properly
		failingExperiment := &ChaosExperiment{
			ID:          "failing-experiment",
			Name:        "Intentionally Failing Experiment",
			Description: "Tests recovery after experiment failure",
			Duration:    5 * time.Second,
			Setup: func(ctx context.Context) error {
				return experiments.SetupResilience()
			},
			Execute: func(ctx context.Context) error {
				// This experiment intentionally fails
				return assert.AnError
			},
			Cleanup: func(ctx context.Context) error {
				// Cleanup should still run
				t.Log("Cleanup running after experiment failure")
				return nil
			},
			Validate: func(ctx context.Context) error {
				// Validation should not be called if execution fails
				t.Error("Validation should not be called after execution failure")
				return nil
			},
		}

		runner.RegisterExperiment(failingExperiment)

		ctx := context.Background()
		result, err := runner.RunExperiment(ctx, "failing-experiment")

		// The experiment should fail, but the runner should handle it gracefully
		assert.Error(t, err)
		assert.NotNil(t, result)
		assert.False(t, result.Success)
		assert.NotEmpty(t, result.Error)

		t.Logf("Failed experiment handled correctly: %s", result.Error)
	})

	t.Run("TimeoutHandling", func(t *testing.T) {
		runner := NewChaosRunner(logger)

		// Create an experiment that times out
		timeoutExperiment := &ChaosExperiment{
			ID:          "timeout-experiment",
			Name:        "Timeout Experiment",
			Description: "Tests timeout handling",
			Duration:    2 * time.Second,
			Setup: func(ctx context.Context) error {
				return nil
			},
			Execute: func(ctx context.Context) error {
				// This will timeout
				time.Sleep(5 * time.Second)
				return nil
			},
			Cleanup: func(ctx context.Context) error {
				t.Log("Cleanup running after timeout")
				return nil
			},
		}

		runner.RegisterExperiment(timeoutExperiment)

		ctx := context.Background()
		result, err := runner.RunExperiment(ctx, "timeout-experiment")

		// Should handle timeout gracefully
		assert.Error(t, err)
		assert.NotNil(t, result)
		assert.False(t, result.Success)
		assert.Contains(t, result.Error, "context deadline exceeded")

		t.Logf("Timeout handled correctly: %s", result.Error)
	})
}
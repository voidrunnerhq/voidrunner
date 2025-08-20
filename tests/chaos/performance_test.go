//go:build integration

package chaos

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/voidrunnerhq/voidrunner/internal/executor"
	"github.com/voidrunnerhq/voidrunner/internal/reporting"
	"github.com/voidrunnerhq/voidrunner/internal/resilience"
)

// PerformanceMetrics holds performance test results
type PerformanceMetrics struct {
	TotalOperations     int64         `json:"total_operations"`
	SuccessfulOps       int64         `json:"successful_operations"`
	FailedOps           int64         `json:"failed_operations"`
	AverageLatency      time.Duration `json:"average_latency"`
	P95Latency          time.Duration `json:"p95_latency"`
	P99Latency          time.Duration `json:"p99_latency"`
	MaxLatency          time.Duration `json:"max_latency"`
	Throughput          float64       `json:"throughput"` // ops/second
	MemoryUsage         int64         `json:"memory_usage_bytes"`
	GoroutineCount      int           `json:"goroutine_count"`
	ErrorRate           float64       `json:"error_rate"`
	CircuitBreakerTrips int64         `json:"circuit_breaker_trips"`
}

// LatencyTracker tracks operation latencies
type LatencyTracker struct {
	mu        sync.Mutex
	latencies []time.Duration
}

// AddLatency adds a latency measurement
func (lt *LatencyTracker) AddLatency(latency time.Duration) {
	lt.mu.Lock()
	defer lt.mu.Unlock()
	lt.latencies = append(lt.latencies, latency)
}

// GetPercentile calculates the specified percentile
func (lt *LatencyTracker) GetPercentile(percentile float64) time.Duration {
	lt.mu.Lock()
	defer lt.mu.Unlock()

	if len(lt.latencies) == 0 {
		return 0
	}

	// Simple percentile calculation (not optimized for large datasets)
	index := int(float64(len(lt.latencies)) * percentile / 100.0)
	if index >= len(lt.latencies) {
		index = len(lt.latencies) - 1
	}

	// Find the value at the percentile index (simplified)
	var sorted []time.Duration
	sorted = append(sorted, lt.latencies...)

	// Simple bubble sort for small datasets
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i] > sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	return sorted[index]
}

// GetAverage calculates the average latency
func (lt *LatencyTracker) GetAverage() time.Duration {
	lt.mu.Lock()
	defer lt.mu.Unlock()

	if len(lt.latencies) == 0 {
		return 0
	}

	total := time.Duration(0)
	for _, latency := range lt.latencies {
		total += latency
	}

	return total / time.Duration(len(lt.latencies))
}

// GetMax returns the maximum latency
func (lt *LatencyTracker) GetMax() time.Duration {
	lt.mu.Lock()
	defer lt.mu.Unlock()

	if len(lt.latencies) == 0 {
		return 0
	}

	max := lt.latencies[0]
	for _, latency := range lt.latencies {
		if latency > max {
			max = latency
		}
	}

	return max
}

func TestErrorHandlingPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping performance tests in short mode")
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))

	t.Run("CircuitBreakerPerformance", func(t *testing.T) {
		config := resilience.DefaultCircuitBreakerConfig()
		config.FailureThreshold = 10
		config.OpenTimeout = 5 * time.Second

		cb, err := resilience.NewCircuitBreaker("perf-test", config, logger)
		require.NoError(t, err)
		defer cb.Reset()

		const numOperations = 10000
		const numGoroutines = 100
		const duration = 30 * time.Second

		var (
			totalOps      int64
			successfulOps int64
			failedOps     int64
		)

		latencyTracker := &LatencyTracker{}
		startTime := time.Now()

		ctx, cancel := context.WithTimeout(context.Background(), duration)
		defer cancel()

		var wg sync.WaitGroup
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()

				operationCount := 0
				for {
					select {
					case <-ctx.Done():
						return
					default:
					}

					opStart := time.Now()
					err := cb.Execute(ctx, func(ctx context.Context) error {
						// Simulate work with occasional failures
						time.Sleep(1 * time.Millisecond)
						if operationCount%50 < 5 { // 10% failure rate
							return fmt.Errorf("simulated failure")
						}
						return nil
					})
					opDuration := time.Since(opStart)

					latencyTracker.AddLatency(opDuration)
					atomic.AddInt64(&totalOps, 1)

					if err != nil {
						atomic.AddInt64(&failedOps, 1)
					} else {
						atomic.AddInt64(&successfulOps, 1)
					}

					operationCount++

					// Small delay to prevent overwhelming the system
					time.Sleep(1 * time.Millisecond)
				}
			}(i)
		}

		wg.Wait()
		elapsed := time.Since(startTime)

		// Collect performance metrics
		var memStats runtime.MemStats
		runtime.ReadMemStats(&memStats)

		metrics := PerformanceMetrics{
			TotalOperations: totalOps,
			SuccessfulOps:   successfulOps,
			FailedOps:       failedOps,
			AverageLatency:  latencyTracker.GetAverage(),
			P95Latency:      latencyTracker.GetPercentile(95),
			P99Latency:      latencyTracker.GetPercentile(99),
			MaxLatency:      latencyTracker.GetMax(),
			Throughput:      float64(totalOps) / elapsed.Seconds(),
			MemoryUsage:     int64(memStats.Alloc),
			GoroutineCount:  runtime.NumGoroutine(),
			ErrorRate:       float64(failedOps) / float64(totalOps) * 100,
		}

		cbStats := cb.GetStats()
		metrics.CircuitBreakerTrips = int64(cbStats.StateChanges)

		t.Logf("Circuit Breaker Performance Results:")
		t.Logf("  Total Operations: %d", metrics.TotalOperations)
		t.Logf("  Success Rate: %.2f%%", float64(metrics.SuccessfulOps)/float64(metrics.TotalOperations)*100)
		t.Logf("  Throughput: %.0f ops/sec", metrics.Throughput)
		t.Logf("  Average Latency: %v", metrics.AverageLatency)
		t.Logf("  P95 Latency: %v", metrics.P95Latency)
		t.Logf("  P99 Latency: %v", metrics.P99Latency)
		t.Logf("  Max Latency: %v", metrics.MaxLatency)
		t.Logf("  Memory Usage: %.2f MB", float64(metrics.MemoryUsage)/1024/1024)
		t.Logf("  Circuit Breaker Trips: %d", metrics.CircuitBreakerTrips)

		// Performance assertions
		assert.True(t, metrics.TotalOperations > 1000, "Expected significant number of operations")
		assert.True(t, metrics.Throughput > 100, "Expected throughput > 100 ops/sec")
		assert.True(t, metrics.AverageLatency < 100*time.Millisecond, "Expected average latency < 100ms")
		assert.True(t, metrics.P99Latency < 1*time.Second, "Expected P99 latency < 1s")
	})

	t.Run("RetryExecutorPerformance", func(t *testing.T) {
		config := resilience.DefaultRetryStrategyConfig()
		config.MaxAttempts = 3
		config.BaseDelay = 10 * time.Millisecond
		config.MaxDelay = 100 * time.Millisecond

		retryExecutor, err := resilience.NewRetryExecutor(config, logger)
		require.NoError(t, err)
		defer retryExecutor.Reset()

		const numOperations = 1000
		const numGoroutines = 50
		const duration = 20 * time.Second

		var (
			totalOps      int64
			successfulOps int64
			failedOps     int64
		)

		latencyTracker := &LatencyTracker{}
		startTime := time.Now()

		ctx, cancel := context.WithTimeout(context.Background(), duration)
		defer cancel()

		var wg sync.WaitGroup
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()

				operationCount := 0
				for operationCount < numOperations/numGoroutines {
					select {
					case <-ctx.Done():
						return
					default:
					}

					opStart := time.Now()
					err := retryExecutor.Execute(ctx,
						fmt.Sprintf("op-%d-%d", workerID, operationCount),
						func(ctx context.Context, attempt int) error {
							// Simulate work
							time.Sleep(2 * time.Millisecond)

							// Fail on first attempt 30% of the time, succeed on retry
							if attempt == 1 && operationCount%10 < 3 {
								return fmt.Errorf("temporary failure")
							}
							return nil
						})
					opDuration := time.Since(opStart)

					latencyTracker.AddLatency(opDuration)
					atomic.AddInt64(&totalOps, 1)

					if err != nil {
						atomic.AddInt64(&failedOps, 1)
					} else {
						atomic.AddInt64(&successfulOps, 1)
					}

					operationCount++
					time.Sleep(5 * time.Millisecond)
				}
			}(i)
		}

		wg.Wait()
		elapsed := time.Since(startTime)

		var memStats runtime.MemStats
		runtime.ReadMemStats(&memStats)

		metrics := PerformanceMetrics{
			TotalOperations: totalOps,
			SuccessfulOps:   successfulOps,
			FailedOps:       failedOps,
			AverageLatency:  latencyTracker.GetAverage(),
			P95Latency:      latencyTracker.GetPercentile(95),
			P99Latency:      latencyTracker.GetPercentile(99),
			MaxLatency:      latencyTracker.GetMax(),
			Throughput:      float64(totalOps) / elapsed.Seconds(),
			MemoryUsage:     int64(memStats.Alloc),
			GoroutineCount:  runtime.NumGoroutine(),
			ErrorRate:       float64(failedOps) / float64(totalOps) * 100,
		}

		retryStats := retryExecutor.GetStats()

		t.Logf("Retry Executor Performance Results:")
		t.Logf("  Total Operations: %d", metrics.TotalOperations)
		t.Logf("  Success Rate: %.2f%%", float64(metrics.SuccessfulOps)/float64(metrics.TotalOperations)*100)
		t.Logf("  Throughput: %.0f ops/sec", metrics.Throughput)
		t.Logf("  Average Latency: %v", metrics.AverageLatency)
		t.Logf("  P95 Latency: %v", metrics.P95Latency)
		t.Logf("  P99 Latency: %v", metrics.P99Latency)
		t.Logf("  Total Retries: %d", retryStats.RetryStats.TotalRetries)
		t.Logf("  Successful Retries: %d", retryStats.RetryStats.SuccessfulRetries)

		// Performance assertions
		assert.True(t, metrics.TotalOperations > 500, "Expected significant number of operations")
		assert.True(t, metrics.Throughput > 10, "Expected throughput > 10 ops/sec")
		assert.True(t, retryStats.RetryStats.TotalRetries > 0, "Expected some retries to occur")
	})

	t.Run("ErrorReportingPerformance", func(t *testing.T) {
		config := reporting.DefaultErrorAggregatorConfig()
		config.AutoReportInterval = 5 * time.Second

		aggregator := reporting.NewErrorAggregator(config, logger)
		defer aggregator.Stop(context.Background())

		const numErrors = 10000
		const numGoroutines = 100

		var (
			totalErrors     int64
			processedErrors int64
		)

		startTime := time.Now()

		var wg sync.WaitGroup
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()

				for j := 0; j < numErrors/numGoroutines; j++ {
					execError := &executor.ExecutionError{
						Type:        executor.ErrorType(fmt.Sprintf("type_%d", j%5)),
						Code:        fmt.Sprintf("CODE_%d", j%20),
						Message:     fmt.Sprintf("Performance test error %d", j),
						Retryable:   j%2 == 0,
						TaskID:      fmt.Sprintf("task-%d", j%100),
						ExecutionID: fmt.Sprintf("exec-%d-%d", workerID, j),
						Timestamp:   time.Now(),
						Context: map[string]interface{}{
							"worker_id": workerID,
							"error_id":  j,
						},
					}

					aggregator.RecordError(context.Background(), execError, execError.TaskID, fmt.Sprintf("user-%d", j%50))
					atomic.AddInt64(&totalErrors, 1)
					atomic.AddInt64(&processedErrors, 1)

					// Small delay to prevent overwhelming
					if j%100 == 0 {
						time.Sleep(1 * time.Millisecond)
					}
				}
			}(i)
		}

		wg.Wait()
		elapsed := time.Since(startTime)

		// Give aggregator time to process
		time.Sleep(2 * time.Second)

		// Generate a report to test reporting performance
		reportStart := time.Now()
		timeRange := reporting.TimeRange{
			Start: startTime.Add(-1 * time.Hour),
			End:   time.Now(),
		}

		report, err := aggregator.GenerateReport(context.Background(), timeRange)
		reportDuration := time.Since(reportStart)

		require.NoError(t, err)
		assert.NotNil(t, report)

		var memStats runtime.MemStats
		runtime.ReadMemStats(&memStats)

		throughput := float64(totalErrors) / elapsed.Seconds()

		t.Logf("Error Reporting Performance Results:")
		t.Logf("  Total Errors Recorded: %d", totalErrors)
		t.Logf("  Recording Throughput: %.0f errors/sec", throughput)
		t.Logf("  Recording Duration: %v", elapsed)
		t.Logf("  Report Generation Time: %v", reportDuration)
		t.Logf("  Report Total Errors: %d", report.TotalErrors)
		t.Logf("  Report Unique Errors: %d", report.UniqueErrors)
		t.Logf("  Memory Usage: %.2f MB", float64(memStats.Alloc)/1024/1024)

		// Performance assertions
		assert.True(t, throughput > 1000, "Expected throughput > 1000 errors/sec")
		assert.True(t, reportDuration < 5*time.Second, "Expected report generation < 5s")
		assert.Equal(t, totalErrors, report.TotalErrors, "All errors should be in report")
		assert.True(t, report.UniqueErrors > 0, "Should have unique errors")
	})

	t.Run("LoadSheddingPerformance", func(t *testing.T) {
		config := resilience.DefaultLoadSheddingConfig()
		config.MaxConcurrentRequests = 50
		config.CPUThreshold = 80.0

		loadShedder, err := resilience.NewLoadShedder(config, nil, logger)
		require.NoError(t, err)
		defer loadShedder.Stop()

		const numRequests = 5000
		const numGoroutines = 200

		var (
			totalRequests    int64
			acceptedRequests int64
			shedRequests     int64
		)

		latencyTracker := &LatencyTracker{}
		startTime := time.Now()

		var wg sync.WaitGroup
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()

				for j := 0; j < numRequests/numGoroutines; j++ {
					requestStart := time.Now()

					priority := resilience.PriorityNormal
					if j%20 == 0 {
						priority = resilience.PriorityHigh
					} else if j%50 == 0 {
						priority = resilience.PriorityCritical
					}

					decision := loadShedder.ShouldAcceptRequest(priority)
					requestDuration := time.Since(requestStart)

					latencyTracker.AddLatency(requestDuration)
					atomic.AddInt64(&totalRequests, 1)

					if decision.Allow {
						atomic.AddInt64(&acceptedRequests, 1)
						// Simulate request processing
						time.Sleep(2 * time.Millisecond)
						loadShedder.CompleteRequest()
					} else {
						atomic.AddInt64(&shedRequests, 1)
					}

					// Small delay between requests
					time.Sleep(100 * time.Microsecond)
				}
			}(i)
		}

		wg.Wait()
		elapsed := time.Since(startTime)

		var memStats runtime.MemStats
		runtime.ReadMemStats(&memStats)

		metrics := PerformanceMetrics{
			TotalOperations: totalRequests,
			SuccessfulOps:   acceptedRequests,
			FailedOps:       shedRequests,
			AverageLatency:  latencyTracker.GetAverage(),
			P95Latency:      latencyTracker.GetPercentile(95),
			P99Latency:      latencyTracker.GetPercentile(99),
			MaxLatency:      latencyTracker.GetMax(),
			Throughput:      float64(totalRequests) / elapsed.Seconds(),
			MemoryUsage:     int64(memStats.Alloc),
			GoroutineCount:  runtime.NumGoroutine(),
		}

		lsStats := loadShedder.GetStats()

		t.Logf("Load Shedding Performance Results:")
		t.Logf("  Total Requests: %d", metrics.TotalOperations)
		t.Logf("  Accepted Requests: %d (%.2f%%)", acceptedRequests, float64(acceptedRequests)/float64(totalRequests)*100)
		t.Logf("  Shed Requests: %d (%.2f%%)", shedRequests, float64(shedRequests)/float64(totalRequests)*100)
		t.Logf("  Throughput: %.0f requests/sec", metrics.Throughput)
		t.Logf("  Average Decision Latency: %v", metrics.AverageLatency)
		t.Logf("  P99 Decision Latency: %v", metrics.P99Latency)
		t.Logf("  Memory Usage: %.2f MB", float64(metrics.MemoryUsage)/1024/1024)

		// Performance assertions
		assert.True(t, metrics.TotalOperations > 4000, "Expected significant number of requests")
		assert.True(t, metrics.Throughput > 1000, "Expected throughput > 1000 requests/sec")
		assert.True(t, metrics.AverageLatency < 10*time.Millisecond, "Expected fast decision making")
		assert.True(t, acceptedRequests > 0, "Some requests should be accepted")
		assert.Equal(t, totalRequests, lsStats.TotalRequests, "Stats should match")
	})
}

func TestConcurrentErrorHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping concurrent error handling tests in short mode")
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))

	t.Run("ConcurrentSystemStress", func(t *testing.T) {
		// Setup all error handling systems
		cbConfig := resilience.DefaultCircuitBreakerConfig()
		cb, err := resilience.NewCircuitBreaker("stress-test", cbConfig, logger)
		require.NoError(t, err)
		defer cb.Reset()

		retryConfig := resilience.DefaultRetryStrategyConfig()
		retryConfig.MaxAttempts = 3
		retryExecutor, err := resilience.NewRetryExecutor(retryConfig, logger)
		require.NoError(t, err)
		defer retryExecutor.Reset()

		lsConfig := resilience.DefaultLoadSheddingConfig()
		loadShedder, err := resilience.NewLoadShedder(lsConfig, nil, logger)
		require.NoError(t, err)
		defer loadShedder.Stop()

		reportingConfig := reporting.DefaultErrorAggregatorConfig()
		errorAggregator := reporting.NewErrorAggregator(reportingConfig, logger)
		defer errorAggregator.Stop(context.Background())

		const duration = 30 * time.Second
		const numWorkers = 50

		var (
			cbOperations        int64
			retryOperations     int64
			lsOperations        int64
			reportingOperations int64
		)

		ctx, cancel := context.WithTimeout(context.Background(), duration)
		defer cancel()

		startTime := time.Now()

		var wg sync.WaitGroup

		// Circuit breaker workers
		for i := 0; i < numWorkers/4; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for {
					select {
					case <-ctx.Done():
						return
					default:
					}

					cb.Execute(ctx, func(ctx context.Context) error {
						time.Sleep(1 * time.Millisecond)
						if atomic.LoadInt64(&cbOperations)%10 < 3 {
							return fmt.Errorf("cb failure")
						}
						return nil
					})
					atomic.AddInt64(&cbOperations, 1)
				}
			}()
		}

		// Retry executor workers
		for i := 0; i < numWorkers/4; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				opCount := 0
				for {
					select {
					case <-ctx.Done():
						return
					default:
					}

					retryExecutor.Execute(ctx, fmt.Sprintf("retry-%d-%d", workerID, opCount),
						func(ctx context.Context, attempt int) error {
							time.Sleep(1 * time.Millisecond)
							if attempt == 1 && opCount%5 == 0 {
								return fmt.Errorf("retry failure")
							}
							return nil
						})
					atomic.AddInt64(&retryOperations, 1)
					opCount++
					time.Sleep(2 * time.Millisecond)
				}
			}(i)
		}

		// Load shedding workers
		for i := 0; i < numWorkers/4; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for {
					select {
					case <-ctx.Done():
						return
					default:
					}

					decision := loadShedder.ShouldAcceptRequest(resilience.PriorityNormal)
					if decision.Allow {
						time.Sleep(1 * time.Millisecond)
						loadShedder.CompleteRequest()
					}
					atomic.AddInt64(&lsOperations, 1)
				}
			}()
		}

		// Error reporting workers
		for i := 0; i < numWorkers/4; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				errorCount := 0
				for {
					select {
					case <-ctx.Done():
						return
					default:
					}

					execError := &executor.ExecutionError{
						Type:        executor.ErrorType(fmt.Sprintf("stress_type_%d", errorCount%3)),
						Code:        fmt.Sprintf("STRESS_%d", errorCount%10),
						Message:     fmt.Sprintf("Stress test error %d", errorCount),
						Retryable:   errorCount%2 == 0,
						TaskID:      fmt.Sprintf("stress-task-%d", errorCount),
						ExecutionID: fmt.Sprintf("stress-exec-%d-%d", workerID, errorCount),
						Timestamp:   time.Now(),
					}

					errorAggregator.RecordError(ctx, execError, execError.TaskID, fmt.Sprintf("stress-user-%d", errorCount%10))
					atomic.AddInt64(&reportingOperations, 1)
					errorCount++
					time.Sleep(1 * time.Millisecond)
				}
			}(i)
		}

		wg.Wait()
		elapsed := time.Since(startTime)

		// Collect final metrics
		var memStats runtime.MemStats
		runtime.ReadMemStats(&memStats)

		totalOps := cbOperations + retryOperations + lsOperations + reportingOperations
		totalThroughput := float64(totalOps) / elapsed.Seconds()

		t.Logf("Concurrent System Stress Test Results:")
		t.Logf("  Duration: %v", elapsed)
		t.Logf("  Circuit Breaker Operations: %d", cbOperations)
		t.Logf("  Retry Operations: %d", retryOperations)
		t.Logf("  Load Shedding Operations: %d", lsOperations)
		t.Logf("  Error Reporting Operations: %d", reportingOperations)
		t.Logf("  Total Operations: %d", totalOps)
		t.Logf("  Overall Throughput: %.0f ops/sec", totalThroughput)
		t.Logf("  Memory Usage: %.2f MB", float64(memStats.Alloc)/1024/1024)
		t.Logf("  Goroutines: %d", runtime.NumGoroutine())

		// Performance assertions
		assert.True(t, totalOps > 10000, "Expected significant total operations")
		assert.True(t, totalThroughput > 500, "Expected overall throughput > 500 ops/sec")
		assert.True(t, cbOperations > 0, "Circuit breaker should have operations")
		assert.True(t, retryOperations > 0, "Retry executor should have operations")
		assert.True(t, lsOperations > 0, "Load shedder should have operations")
		assert.True(t, reportingOperations > 0, "Error reporting should have operations")

		// Memory usage should be reasonable (less than 100MB for this test)
		assert.True(t, memStats.Alloc < 100*1024*1024, "Memory usage should be reasonable")

		t.Logf("Concurrent stress test completed successfully")
	})
}

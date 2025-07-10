//go:build integration

package integration_test

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/voidrunnerhq/voidrunner/tests/testutil"
)

// TestOpenAPIValidator_ThreadSafety tests that the validator can be used concurrently
func TestOpenAPIValidator_ThreadSafety(t *testing.T) {
	const numGoroutines = 100
	const numValidationsPerGoroutine = 10

	var wg sync.WaitGroup
	errChan := make(chan error, numGoroutines*numValidationsPerGoroutine)

	// Run multiple goroutines that each create validators and validate responses
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			for j := 0; j < numValidationsPerGoroutine; j++ {
				// Create a new validator instance for each validation
				validator := testutil.NewOpenAPIValidator()

				// Create a mock response
				resp := &http.Response{
					StatusCode: 200,
					Header:     make(http.Header),
				}
				resp.Header.Set("Content-Type", "application/json")

				// Create a mock response body
				body := []byte(`{"status": "ok", "timestamp": "2023-01-01T00:00:00Z"}`)

				// This should not cause race conditions
				validator.ValidateResponse(t, "GET", "/health", resp, body)
			}
		}(i)
	}

	wg.Wait()
	close(errChan)

	// Check for any errors
	for err := range errChan {
		t.Errorf("Concurrent validation error: %v", err)
	}
}

// TestOpenAPIValidator_ConcurrentCreation tests that creating multiple validators concurrently is safe
func TestOpenAPIValidator_ConcurrentCreation(t *testing.T) {
	const numGoroutines = 50

	var wg sync.WaitGroup
	validators := make([]*testutil.OpenAPIValidator, numGoroutines)

	// Create validators concurrently
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			validators[index] = testutil.NewOpenAPIValidator()
		}(i)
	}

	wg.Wait()

	// Verify all validators were created successfully
	for i, validator := range validators {
		assert.NotNil(t, validator, "Validator %d should not be nil", i)
		assert.NotNil(t, validator.GetSpec(), "Validator %d spec should not be nil", i)
		assert.NotNil(t, validator.GetSpec().Paths, "Validator %d paths should not be nil", i)
	}
}

// TestOpenAPIValidator_BasicValidation tests basic validation functionality
func TestOpenAPIValidator_BasicValidation(t *testing.T) {
	validator := testutil.NewOpenAPIValidator()

	// Test health endpoint validation
	resp := httptest.NewRecorder()
	resp.Header().Set("Content-Type", "application/json")
	resp.WriteHeader(http.StatusOK)
	body := []byte(`{"status": "ok", "timestamp": "2023-01-01T00:00:00Z"}`)

	// This should not panic or cause errors
	validator.ValidateResponse(t, "GET", "/health", resp.Result(), body)
}

// TestHTTPResponseValidator_ConcurrentUsage tests concurrent usage of HTTPResponseValidator
func TestHTTPResponseValidator_ConcurrentUsage(t *testing.T) {
	const numGoroutines = 20

	var wg sync.WaitGroup

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			// Create a mock response
			resp := httptest.NewRecorder()
			resp.Header().Set("Content-Type", "application/json")
			resp.WriteHeader(http.StatusOK)
			_, _ = resp.WriteString(`{"test": "data"}`)

			// Create validator
			validator := testutil.NewHTTPResponseValidator(t, resp.Result())

			// Chain validation calls
			validator.ExpectStatus(200).
				ExpectContentType("application/json").
				ExpectValidJSON()
		}(i)
	}

	wg.Wait()
}

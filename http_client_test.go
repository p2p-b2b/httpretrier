package httpretrier

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestClientBuilder_WithMethods(t *testing.T) {
	builder := NewClientBuilder()

	// Test valid settings
	builder.WithMaxIdleConns(50).
		WithIdleConnTimeout(60 * time.Second).
		WithTLSHandshakeTimeout(5 * time.Second).
		WithExpectContinueTimeout(2 * time.Second).
		WithDisableKeepAlives(true).
		WithMaxIdleConnsPerHost(50).
		WithTimeout(10 * time.Second).
		WithMaxRetries(5).
		WithRetryBaseDelay(100 * time.Millisecond).
		WithRetryMaxDelay(5 * time.Second).
		WithRetryStrategy(FixedDelayStrategy)

	client := builder.client
	assert.Equal(t, 50, client.maxIdleConns)
	assert.Equal(t, 60*time.Second, client.idleConnTimeout)
	assert.Equal(t, 5*time.Second, client.tlsHandshakeTimeout)
	assert.Equal(t, 2*time.Second, client.expectContinueTimeout)
	assert.True(t, client.disableKeepAlives)
	assert.Equal(t, 50, client.maxIdleConnsPerHost)
	assert.Equal(t, 10*time.Second, client.timeout)
	assert.Equal(t, 5, client.maxRetries)
	assert.Equal(t, 100*time.Millisecond, client.retryBaseDelay) // Check the value *set* by WithRetryBaseDelay
	assert.Equal(t, 5*time.Second, client.retryMaxDelay)
	// Check the strategy *type* was set
	assert.Equal(t, FixedDelayStrategy, client.retryStrategyType) // Check the type *set* by WithRetryStrategy

	// Test invalid settings (should use defaults or adjusted values)
	builder = NewClientBuilder() // Reset builder
	builder.WithMaxIdleConns(0). // Invalid, use default
					WithIdleConnTimeout(0).                   // Invalid, use default
					WithTLSHandshakeTimeout(0).               // Invalid, use default
					WithExpectContinueTimeout(0).             // Invalid, use default
					WithMaxIdleConnsPerHost(0).               // Invalid, use default
					WithTimeout(0).                           // Invalid, use default
					WithMaxRetries(0).                        // Invalid, use default
					WithRetryBaseDelay(1 * time.Millisecond). // Invalid, use default
					WithRetryMaxDelay(50 * time.Millisecond). // Invalid, use default
					WithRetryStrategy("invalid")              // Invalid strategy type

	client = builder.client
	// Assert that the *invalid* values were set by the With... methods (before Build validation)
	assert.Equal(t, 0, client.maxIdleConns)
	assert.Equal(t, 0*time.Second, client.idleConnTimeout)
	assert.Equal(t, 0*time.Second, client.tlsHandshakeTimeout)
	assert.Equal(t, 0*time.Second, client.expectContinueTimeout)
	assert.Equal(t, 0, client.maxIdleConnsPerHost)
	assert.Equal(t, 0*time.Second, client.timeout)
	assert.Equal(t, 0, client.maxRetries)
	assert.Equal(t, 1*time.Millisecond, client.retryBaseDelay)
	assert.Equal(t, 50*time.Millisecond, client.retryMaxDelay)
	// Check that the invalid strategy type was set
	assert.Equal(t, Strategy("invalid"), client.retryStrategyType)
}

func TestClientBuilder_Build(t *testing.T) {
	baseDelay := 200 * time.Millisecond
	maxDelay := 2 * time.Second
	maxRetries := 4

	builder := NewClientBuilder().
		WithMaxIdleConns(55).
		WithIdleConnTimeout(65 * time.Second).
		WithTLSHandshakeTimeout(6 * time.Second).
		WithExpectContinueTimeout(3 * time.Second).
		WithDisableKeepAlives(true).
		WithMaxIdleConnsPerHost(55).
		WithTimeout(11 * time.Second).
		WithMaxRetries(maxRetries).
		WithRetryBaseDelay(baseDelay).
		WithRetryMaxDelay(maxDelay).
		WithRetryStrategy(JitterBackoffStrategy)

	httpClient := builder.Build()

	assert.NotNil(t, httpClient)
	assert.Equal(t, 11*time.Second, httpClient.Timeout)
	assert.NotNil(t, httpClient.Transport)

	// Check if transport is retryTransport
	rt, ok := httpClient.Transport.(*retryTransport)
	assert.True(t, ok, "Transport should be of type *retryTransport")
	assert.NotNil(t, rt)

	// Check retryTransport settings
	assert.Equal(t, maxRetries, rt.MaxRetries)
	assert.NotNil(t, rt.RetryStrategy)
	// Verify the strategy function produces a delay within the expected range for Jitter
	// (This is an indirect way to check if the correct strategy function was set)
	attempt := 1
	// IMPORTANT: Calculate expected delay using the *validated* baseDelay from the builder,
	// as the initial baseDelay (200ms) is invalid (< 300ms) and will be defaulted in Build.
	validatedBaseDelay := builder.client.retryBaseDelay // Get the delay after Build's validation
	if validatedBaseDelay < ValidMinBaseDelay || validatedBaseDelay > ValidMaxBaseDelay {
		validatedBaseDelay = DefaultBaseDelay // Manually apply the same defaulting logic as Build
	}
	validatedMaxDelay := builder.client.retryMaxDelay // Get the validated max delay
	if validatedMaxDelay < ValidMinMaxDelay || validatedMaxDelay > ValidMaxMaxDelay {
		validatedMaxDelay = DefaultMaxDelay
	}

	// Calculate the expected exponential backoff delay for this attempt using validated delays
	expectedExpDelay := ExponentialBackoff(validatedBaseDelay, validatedMaxDelay)(attempt)
	// Now get the actual delay which includes jitter
	actualDelay := rt.RetryStrategy(attempt)

	// Jitter delay should be >= the exponential delay for that attempt
	assert.GreaterOrEqual(t, actualDelay, expectedExpDelay, "Jitter delay for attempt %d should be >= exponential backoff delay (%v)", attempt, expectedExpDelay)
	// Max jitter delay is exponential delay + (exponential delay / 2)
	maxExpectedJitterDelay := expectedExpDelay + (expectedExpDelay / 2)
	assert.Less(t, actualDelay, maxExpectedJitterDelay, "Jitter delay for attempt %d (%v) should be < exponential backoff delay + half (%v)", attempt, actualDelay, maxExpectedJitterDelay)

	// Check underlying http.Transport settings
	stdTransport, ok := rt.Transport.(*http.Transport)
	assert.True(t, ok, "Inner transport should be of type *http.Transport")
	assert.NotNil(t, stdTransport)

	assert.Equal(t, 55, stdTransport.MaxIdleConns)
	assert.Equal(t, 65*time.Second, stdTransport.IdleConnTimeout)
	assert.Equal(t, 6*time.Second, stdTransport.TLSHandshakeTimeout)
	assert.Equal(t, 3*time.Second, stdTransport.ExpectContinueTimeout)
	assert.True(t, stdTransport.DisableKeepAlives)
	assert.Equal(t, 55, stdTransport.MaxIdleConnsPerHost)

	// Test building with default strategy (Exponential)
	builder = NewClientBuilder()
	httpClient = builder.Build()
	rt, _ = httpClient.Transport.(*retryTransport)
	delay := rt.RetryStrategy(1)          // Attempt 1
	expectedDelay := DefaultBaseDelay * 2 // Exponential backoff doubles for attempt 1
	assert.Equal(t, expectedDelay, delay, "Default strategy (Exponential) delay check failed")

	// Test building with FixedDelay strategy
	builder = NewClientBuilder().WithRetryBaseDelay(1 * time.Second).WithRetryStrategy(FixedDelayStrategy)
	httpClient = builder.Build()
	rt, _ = httpClient.Transport.(*retryTransport)
	delay = rt.RetryStrategy(1) // Attempt 1
	assert.Equal(t, 1*time.Second, delay, "FixedDelay strategy delay check failed")
	delay = rt.RetryStrategy(5) // Attempt 5
	assert.Equal(t, 1*time.Second, delay, "FixedDelay strategy delay check failed")
}

func TestStrategyString(t *testing.T) {
	assert.Equal(t, "fixed", FixedDelayStrategy.String())
	assert.Equal(t, "jitter", JitterBackoffStrategy.String())
	assert.Equal(t, "exponential", ExponentialBackoffStrategy.String())
	assert.Equal(t, "unknown", Strategy("unknown").String())
}

func TestClientError(t *testing.T) {
	err := &ClientError{Message: "test error"}
	assert.Equal(t, "test error", err.Error())
}

func TestClientBuilder_WithRetryStrategyAsString(t *testing.T) {
	tests := []struct {
		name          string
		inputStrategy string
		expectedType  Strategy
		expectWarning bool // Although we can't directly test logs here, good to note
	}{
		{
			name:          "Valid Fixed Strategy",
			inputStrategy: "fixed",
			expectedType:  FixedDelayStrategy,
			expectWarning: false,
		},
		{
			name:          "Valid Jitter Strategy",
			inputStrategy: "jitter",
			expectedType:  JitterBackoffStrategy,
			expectWarning: false,
		},
		{
			name:          "Valid Exponential Strategy",
			inputStrategy: "exponential",
			expectedType:  ExponentialBackoffStrategy,
			expectWarning: false,
		},
		{
			name:          "Invalid Strategy",
			inputStrategy: "invalid-strategy",
			expectedType:  ExponentialBackoffStrategy, // Should default
			expectWarning: true,
		},
		{
			name:          "Empty Strategy",
			inputStrategy: "",
			expectedType:  ExponentialBackoffStrategy, // Should default
			expectWarning: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := NewClientBuilder() // Start fresh for each test case
			builder.WithRetryStrategyAsString(tt.inputStrategy)

			// Assert that the correct strategy *type* was set on the internal client struct
			assert.Equal(t, tt.expectedType, builder.client.retryStrategyType)

			// Note: We expect a warning log for invalid strategies, but testing logs
			// usually requires more setup (e.g., capturing log output).
			// This test focuses on the functional outcome (correct strategy type set).
		})
	}
}

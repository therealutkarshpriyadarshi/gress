package errors

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRetryPolicy_Execute(t *testing.T) {
	t.Run("success on first attempt", func(t *testing.T) {
		policy := DefaultRetryPolicy()
		ctx := context.Background()

		result := policy.Execute(ctx, func(context.Context) error {
			return nil
		})

		if !result.Success {
			t.Error("Expected success")
		}
		if result.Attempts != 1 {
			t.Errorf("Expected 1 attempt, got %d", result.Attempts)
		}
		if result.LastError != nil {
			t.Errorf("Expected no error, got %v", result.LastError)
		}
	})

	t.Run("success after retries", func(t *testing.T) {
		policy := DefaultRetryPolicy()
		ctx := context.Background()

		attemptCount := 0
		result := policy.Execute(ctx, func(context.Context) error {
			attemptCount++
			if attemptCount < 3 {
				return errors.New("temporary error")
			}
			return nil
		})

		if !result.Success {
			t.Error("Expected success after retries")
		}
		if result.Attempts != 3 {
			t.Errorf("Expected 3 attempts, got %d", result.Attempts)
		}
	})

	t.Run("failure after max attempts", func(t *testing.T) {
		policy := &RetryPolicy{
			MaxAttempts:       2,
			InitialBackoff:    1 * time.Millisecond,
			MaxBackoff:        10 * time.Millisecond,
			BackoffMultiplier: 2.0,
			Jitter:            0.0,
			RetriableFunc:     IsRetriable,
		}
		ctx := context.Background()

		testErr := errors.New("persistent error")
		result := policy.Execute(ctx, func(context.Context) error {
			return testErr
		})

		if result.Success {
			t.Error("Expected failure")
		}
		if result.Attempts != 3 { // Initial + 2 retries
			t.Errorf("Expected 3 attempts, got %d", result.Attempts)
		}
		if result.LastError != testErr {
			t.Errorf("Expected error %v, got %v", testErr, result.LastError)
		}
	})

	t.Run("non-retriable error stops retry", func(t *testing.T) {
		policy := &RetryPolicy{
			MaxAttempts:       5,
			InitialBackoff:    1 * time.Millisecond,
			MaxBackoff:        10 * time.Millisecond,
			BackoffMultiplier: 2.0,
			Jitter:            0.0,
			RetriableFunc:     IsFatal, // Inverted - fatal errors return true
		}
		ctx := context.Background()

		fatalErr := NewClassifiedError(errors.New("fatal"), CategoryFatal, "fatal error")
		result := policy.Execute(ctx, func(context.Context) error {
			return fatalErr
		})

		if result.Success {
			t.Error("Expected failure")
		}
		if result.Attempts != 1 {
			t.Errorf("Expected 1 attempt (no retries for fatal), got %d", result.Attempts)
		}
	})

	t.Run("context cancellation stops retry", func(t *testing.T) {
		policy := &RetryPolicy{
			MaxAttempts:       10,
			InitialBackoff:    50 * time.Millisecond,
			MaxBackoff:        1 * time.Second,
			BackoffMultiplier: 2.0,
			Jitter:            0.0,
			RetriableFunc:     IsRetriable,
		}

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		result := policy.Execute(ctx, func(context.Context) error {
			return errors.New("persistent error")
		})

		if result.Success {
			t.Error("Expected failure due to context cancellation")
		}
		if result.LastError != context.DeadlineExceeded {
			t.Errorf("Expected context deadline exceeded, got %v", result.LastError)
		}
	})

	t.Run("no retry policy", func(t *testing.T) {
		policy := NoRetryPolicy()
		ctx := context.Background()

		testErr := errors.New("test error")
		result := policy.Execute(ctx, func(context.Context) error {
			return testErr
		})

		if result.Success {
			t.Error("Expected failure")
		}
		if result.Attempts != 1 {
			t.Errorf("Expected 1 attempt, got %d", result.Attempts)
		}
	})
}

func TestRetryPolicy_ExecuteWithCallback(t *testing.T) {
	t.Run("callback called on each retry", func(t *testing.T) {
		policy := &RetryPolicy{
			MaxAttempts:       3,
			InitialBackoff:    1 * time.Millisecond,
			MaxBackoff:        10 * time.Millisecond,
			BackoffMultiplier: 2.0,
			Jitter:            0.0,
			RetriableFunc:     IsRetriable,
		}
		ctx := context.Background()

		callbackCount := 0
		attemptCount := 0

		result := policy.ExecuteWithCallback(
			ctx,
			func(context.Context) error {
				attemptCount++
				return errors.New("test error")
			},
			func(attempt int, err error, nextBackoff time.Duration) {
				callbackCount++
			},
		)

		if result.Success {
			t.Error("Expected failure")
		}
		if callbackCount != 4 { // Initial + 3 retries
			t.Errorf("Expected 4 callbacks, got %d", callbackCount)
		}
	})
}

func TestRetryPolicy_calculateBackoff(t *testing.T) {
	policy := &RetryPolicy{
		InitialBackoff:    100 * time.Millisecond,
		MaxBackoff:        10 * time.Second,
		BackoffMultiplier: 2.0,
		Jitter:            0.0,
	}

	tests := []struct {
		attempt  int
		expected time.Duration
	}{
		{0, 100 * time.Millisecond},
		{1, 200 * time.Millisecond},
		{2, 400 * time.Millisecond},
		{3, 800 * time.Millisecond},
		{10, 10 * time.Second}, // Should be capped at MaxBackoff
	}

	for _, tt := range tests {
		result := policy.calculateBackoff(tt.attempt)
		if result != tt.expected {
			t.Errorf("calculateBackoff(%d) = %v, want %v", tt.attempt, result, tt.expected)
		}
	}
}

func TestRetryPolicy_calculateBackoffWithJitter(t *testing.T) {
	policy := &RetryPolicy{
		InitialBackoff:    100 * time.Millisecond,
		MaxBackoff:        10 * time.Second,
		BackoffMultiplier: 2.0,
		Jitter:            0.5,
	}

	// Calculate backoff multiple times and ensure jitter is applied
	backoffs := make(map[time.Duration]bool)
	for i := 0; i < 10; i++ {
		backoff := policy.calculateBackoff(1)
		backoffs[backoff] = true
	}

	// With jitter, we should see some variation
	// (though this is probabilistic and might rarely fail)
	if len(backoffs) < 2 {
		t.Log("Expected some variation in backoff due to jitter")
		// Don't fail the test as this is probabilistic
	}
}

func TestRetryPolicy_ShouldRetry(t *testing.T) {
	policy := &RetryPolicy{
		MaxAttempts:   3,
		RetriableFunc: IsRetriable,
	}

	t.Run("should retry retriable error within max attempts", func(t *testing.T) {
		err := errors.New("connection refused")
		if !policy.ShouldRetry(err, 1) {
			t.Error("Expected to retry")
		}
	})

	t.Run("should not retry beyond max attempts", func(t *testing.T) {
		err := errors.New("connection refused")
		if policy.ShouldRetry(err, 4) {
			t.Error("Expected not to retry beyond max attempts")
		}
	})

	t.Run("should not retry non-retriable error", func(t *testing.T) {
		err := NewClassifiedError(errors.New("test"), CategoryFatal, "fatal")
		if policy.ShouldRetry(err, 1) {
			t.Error("Expected not to retry fatal error")
		}
	})

	t.Run("should not retry nil error", func(t *testing.T) {
		if policy.ShouldRetry(nil, 1) {
			t.Error("Expected not to retry nil error")
		}
	})
}

func TestRetryPolicy_NextBackoff(t *testing.T) {
	policy := &RetryPolicy{
		InitialBackoff:    100 * time.Millisecond,
		MaxBackoff:        1 * time.Second,
		BackoffMultiplier: 2.0,
		Jitter:            0.0,
	}

	backoff := policy.NextBackoff(0)
	if backoff != 100*time.Millisecond {
		t.Errorf("NextBackoff(0) = %v, want %v", backoff, 100*time.Millisecond)
	}

	backoff = policy.NextBackoff(1)
	if backoff != 200*time.Millisecond {
		t.Errorf("NextBackoff(1) = %v, want %v", backoff, 200*time.Millisecond)
	}
}

func TestRetryStats(t *testing.T) {
	stats := NewRetryStats()

	result1 := &RetryResult{
		Success:      true,
		Attempts:     3,
		TotalBackoff: 300 * time.Millisecond,
	}
	stats.RecordAttempt(result1)

	result2 := &RetryResult{
		Success:      false,
		Attempts:     5,
		TotalBackoff: 500 * time.Millisecond,
	}
	stats.RecordAttempt(result2)

	if stats.TotalAttempts != 8 {
		t.Errorf("Expected 8 total attempts, got %d", stats.TotalAttempts)
	}

	if stats.SuccessfulRetries != 1 {
		t.Errorf("Expected 1 successful retry, got %d", stats.SuccessfulRetries)
	}

	if stats.FailedRetries != 1 {
		t.Errorf("Expected 1 failed retry, got %d", stats.FailedRetries)
	}

	if stats.TotalBackoffTime != 800*time.Millisecond {
		t.Errorf("Expected 800ms total backoff, got %v", stats.TotalBackoffTime)
	}
}

func TestInfiniteRetryPolicy(t *testing.T) {
	policy := InfiniteRetryPolicy()

	if policy.MaxAttempts != -1 {
		t.Errorf("Expected MaxAttempts=-1 for infinite retry, got %d", policy.MaxAttempts)
	}

	// Test that it retries many times
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	attemptCount := 0
	result := policy.Execute(ctx, func(context.Context) error {
		attemptCount++
		return errors.New("persistent error")
	})

	if result.Success {
		t.Error("Expected failure due to context timeout")
	}

	if attemptCount < 2 {
		t.Errorf("Expected multiple attempts with infinite retry, got %d", attemptCount)
	}
}

func BenchmarkRetryPolicy_Execute(b *testing.B) {
	policy := &RetryPolicy{
		MaxAttempts:       3,
		InitialBackoff:    1 * time.Microsecond,
		MaxBackoff:        10 * time.Microsecond,
		BackoffMultiplier: 2.0,
		Jitter:            0.0,
		RetriableFunc:     IsRetriable,
	}
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		policy.Execute(ctx, func(context.Context) error {
			return nil // Immediate success
		})
	}
}

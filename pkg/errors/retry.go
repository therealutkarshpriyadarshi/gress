package errors

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"time"
)

// RetryPolicy defines the retry behavior for failed operations
type RetryPolicy struct {
	// MaxAttempts is the maximum number of retry attempts (0 = no retries, -1 = infinite)
	MaxAttempts int
	// InitialBackoff is the initial backoff duration
	InitialBackoff time.Duration
	// MaxBackoff is the maximum backoff duration
	MaxBackoff time.Duration
	// BackoffMultiplier is the multiplier for exponential backoff (default: 2.0)
	BackoffMultiplier float64
	// Jitter adds randomness to backoff to prevent thundering herd (0.0-1.0)
	Jitter float64
	// RetriableFunc determines if an error is retriable (optional)
	RetriableFunc func(error) bool
}

// DefaultRetryPolicy returns a sensible default retry policy
func DefaultRetryPolicy() *RetryPolicy {
	return &RetryPolicy{
		MaxAttempts:       3,
		InitialBackoff:    100 * time.Millisecond,
		MaxBackoff:        30 * time.Second,
		BackoffMultiplier: 2.0,
		Jitter:            0.1,
		RetriableFunc:     IsRetriable,
	}
}

// NoRetryPolicy returns a policy that never retries
func NoRetryPolicy() *RetryPolicy {
	return &RetryPolicy{
		MaxAttempts: 0,
	}
}

// InfiniteRetryPolicy returns a policy that retries indefinitely with backoff
func InfiniteRetryPolicy() *RetryPolicy {
	return &RetryPolicy{
		MaxAttempts:       -1,
		InitialBackoff:    100 * time.Millisecond,
		MaxBackoff:        60 * time.Second,
		BackoffMultiplier: 2.0,
		Jitter:            0.2,
		RetriableFunc:     IsRetriable,
	}
}

// RetryableOperation is a function that can be retried
type RetryableOperation func(ctx context.Context) error

// RetryResult contains the result of a retry operation
type RetryResult struct {
	Success      bool
	Attempts     int
	LastError    error
	TotalBackoff time.Duration
}

// Execute executes an operation with retry logic
func (rp *RetryPolicy) Execute(ctx context.Context, operation RetryableOperation) *RetryResult {
	result := &RetryResult{
		Success:  false,
		Attempts: 0,
	}

	if rp.MaxAttempts == 0 {
		// No retries, execute once
		err := operation(ctx)
		result.Attempts = 1
		result.LastError = err
		result.Success = err == nil
		return result
	}

	for attempt := 0; rp.MaxAttempts < 0 || attempt <= rp.MaxAttempts; attempt++ {
		result.Attempts++

		// Execute the operation
		err := operation(ctx)
		if err == nil {
			result.Success = true
			return result
		}

		result.LastError = err

		// Check if error is retriable
		if rp.RetriableFunc != nil && !rp.RetriableFunc(err) {
			// Non-retriable error, stop retrying
			return result
		}

		// Check if we've exhausted attempts
		if rp.MaxAttempts >= 0 && attempt >= rp.MaxAttempts {
			return result
		}

		// Calculate backoff duration
		backoff := rp.calculateBackoff(attempt)
		result.TotalBackoff += backoff

		// Wait for backoff duration or context cancellation
		select {
		case <-ctx.Done():
			result.LastError = ctx.Err()
			return result
		case <-time.After(backoff):
			// Continue to next attempt
		}
	}

	return result
}

// ExecuteWithCallback executes an operation with retry logic and callback
type RetryCallback func(attempt int, err error, nextBackoff time.Duration)

// ExecuteWithCallback executes an operation with retry and calls callback on each failure
func (rp *RetryPolicy) ExecuteWithCallback(
	ctx context.Context,
	operation RetryableOperation,
	callback RetryCallback,
) *RetryResult {
	result := &RetryResult{
		Success:  false,
		Attempts: 0,
	}

	if rp.MaxAttempts == 0 {
		err := operation(ctx)
		result.Attempts = 1
		result.LastError = err
		result.Success = err == nil
		if !result.Success && callback != nil {
			callback(1, err, 0)
		}
		return result
	}

	for attempt := 0; rp.MaxAttempts < 0 || attempt <= rp.MaxAttempts; attempt++ {
		result.Attempts++

		err := operation(ctx)
		if err == nil {
			result.Success = true
			return result
		}

		result.LastError = err

		if rp.RetriableFunc != nil && !rp.RetriableFunc(err) {
			if callback != nil {
				callback(attempt+1, err, 0)
			}
			return result
		}

		if rp.MaxAttempts >= 0 && attempt >= rp.MaxAttempts {
			if callback != nil {
				callback(attempt+1, err, 0)
			}
			return result
		}

		backoff := rp.calculateBackoff(attempt)
		result.TotalBackoff += backoff

		if callback != nil {
			callback(attempt+1, err, backoff)
		}

		select {
		case <-ctx.Done():
			result.LastError = ctx.Err()
			return result
		case <-time.After(backoff):
		}
	}

	return result
}

// calculateBackoff calculates the backoff duration for a given attempt
func (rp *RetryPolicy) calculateBackoff(attempt int) time.Duration {
	// Exponential backoff: initialBackoff * (multiplier ^ attempt)
	backoff := float64(rp.InitialBackoff) * math.Pow(rp.BackoffMultiplier, float64(attempt))

	// Cap at max backoff
	if backoff > float64(rp.MaxBackoff) {
		backoff = float64(rp.MaxBackoff)
	}

	// Add jitter to prevent thundering herd
	if rp.Jitter > 0 {
		jitterAmount := backoff * rp.Jitter
		jitter := (rand.Float64()*2 - 1) * jitterAmount // Random value between -jitterAmount and +jitterAmount
		backoff += jitter
		if backoff < 0 {
			backoff = float64(rp.InitialBackoff)
		}
	}

	return time.Duration(backoff)
}

// ShouldRetry checks if an error should be retried according to this policy
func (rp *RetryPolicy) ShouldRetry(err error, attempt int) bool {
	if err == nil {
		return false
	}

	// Check max attempts
	if rp.MaxAttempts >= 0 && attempt > rp.MaxAttempts {
		return false
	}

	// Check if error is retriable
	if rp.RetriableFunc != nil {
		return rp.RetriableFunc(err)
	}

	return true
}

// NextBackoff returns the backoff duration for the next attempt
func (rp *RetryPolicy) NextBackoff(attempt int) time.Duration {
	return rp.calculateBackoff(attempt)
}

// RetryStats tracks retry statistics
type RetryStats struct {
	TotalAttempts    int
	SuccessfulRetries int
	FailedRetries    int
	TotalBackoffTime time.Duration
}

// NewRetryStats creates a new retry stats tracker
func NewRetryStats() *RetryStats {
	return &RetryStats{}
}

// RecordAttempt records a retry attempt
func (rs *RetryStats) RecordAttempt(result *RetryResult) {
	rs.TotalAttempts += result.Attempts
	if result.Success && result.Attempts > 1 {
		rs.SuccessfulRetries++
	} else if !result.Success {
		rs.FailedRetries++
	}
	rs.TotalBackoffTime += result.TotalBackoff
}

// String returns a string representation of retry stats
func (rs *RetryStats) String() string {
	return fmt.Sprintf(
		"RetryStats{TotalAttempts: %d, SuccessfulRetries: %d, FailedRetries: %d, TotalBackoff: %s}",
		rs.TotalAttempts,
		rs.SuccessfulRetries,
		rs.FailedRetries,
		rs.TotalBackoffTime,
	)
}

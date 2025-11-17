package errors

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCircuitBreaker_Execute(t *testing.T) {
	t.Run("closed state allows requests", func(t *testing.T) {
		cb := NewCircuitBreaker(DefaultCircuitBreakerConfig("test"))
		ctx := context.Background()

		err := cb.Execute(ctx, func() error {
			return nil
		})

		if err != nil {
			t.Errorf("Expected success, got error: %v", err)
		}

		if cb.State() != StateClosed {
			t.Errorf("Expected state=closed, got %v", cb.State())
		}
	})

	t.Run("failures open circuit", func(t *testing.T) {
		config := &CircuitBreakerConfig{
			Name:             "test",
			FailureThreshold: 3,
			Timeout:          1 * time.Second,
		}
		cb := NewCircuitBreaker(config)
		ctx := context.Background()

		testErr := errors.New("test error")

		// Fail 3 times to open circuit
		for i := 0; i < 3; i++ {
			cb.Execute(ctx, func() error {
				return testErr
			})
		}

		if cb.State() != StateOpen {
			t.Errorf("Expected state=open after %d failures, got %v", 3, cb.State())
		}

		// Next request should be rejected
		err := cb.Execute(ctx, func() error {
			return nil
		})

		var openErr *ErrCircuitOpen
		if !errors.As(err, &openErr) {
			t.Errorf("Expected ErrCircuitOpen, got %v", err)
		}
	})

	t.Run("open circuit transitions to half-open after timeout", func(t *testing.T) {
		config := &CircuitBreakerConfig{
			Name:             "test",
			FailureThreshold: 2,
			Timeout:          50 * time.Millisecond,
		}
		cb := NewCircuitBreaker(config)
		ctx := context.Background()

		testErr := errors.New("test error")

		// Open the circuit
		for i := 0; i < 2; i++ {
			cb.Execute(ctx, func() error {
				return testErr
			})
		}

		if cb.State() != StateOpen {
			t.Fatal("Circuit should be open")
		}

		// Wait for timeout
		time.Sleep(60 * time.Millisecond)

		// Next request should transition to half-open
		cb.Execute(ctx, func() error {
			return nil
		})

		stats := cb.Stats()
		if stats.State != StateHalfOpen && stats.State != StateClosed {
			t.Errorf("Expected state=half-open or closed, got %v", stats.State)
		}
	})

	t.Run("half-open closes on success", func(t *testing.T) {
		config := &CircuitBreakerConfig{
			Name:             "test",
			FailureThreshold: 2,
			SuccessThreshold: 2,
			Timeout:          50 * time.Millisecond,
		}
		cb := NewCircuitBreaker(config)
		ctx := context.Background()

		// Open the circuit
		for i := 0; i < 2; i++ {
			cb.Execute(ctx, func() error {
				return errors.New("error")
			})
		}

		// Wait for timeout
		time.Sleep(60 * time.Millisecond)

		// Succeed twice to close circuit
		cb.Execute(ctx, func() error {
			return nil
		})
		cb.Execute(ctx, func() error {
			return nil
		})

		if cb.State() != StateClosed {
			t.Errorf("Expected state=closed after successes, got %v", cb.State())
		}
	})

	t.Run("half-open reopens on failure", func(t *testing.T) {
		config := &CircuitBreakerConfig{
			Name:             "test",
			FailureThreshold: 2,
			SuccessThreshold: 2,
			Timeout:          50 * time.Millisecond,
		}
		cb := NewCircuitBreaker(config)
		ctx := context.Background()

		// Open the circuit
		for i := 0; i < 2; i++ {
			cb.Execute(ctx, func() error {
				return errors.New("error")
			})
		}

		// Wait for timeout
		time.Sleep(60 * time.Millisecond)

		// Fail in half-open state
		cb.Execute(ctx, func() error {
			return errors.New("error")
		})

		if cb.State() != StateOpen {
			t.Errorf("Expected state=open after failure in half-open, got %v", cb.State())
		}
	})

	t.Run("max concurrent requests in half-open", func(t *testing.T) {
		config := &CircuitBreakerConfig{
			Name:                  "test",
			FailureThreshold:      2,
			SuccessThreshold:      1,
			Timeout:               50 * time.Millisecond,
			MaxConcurrentRequests: 1,
		}
		cb := NewCircuitBreaker(config)
		ctx := context.Background()

		// Open the circuit
		for i := 0; i < 2; i++ {
			cb.Execute(ctx, func() error {
				return errors.New("error")
			})
		}

		// Wait for timeout
		time.Sleep(60 * time.Millisecond)

		// First request should transition to half-open
		done := make(chan bool)
		go func() {
			cb.Execute(ctx, func() error {
				time.Sleep(100 * time.Millisecond)
				return nil
			})
			done <- true
		}()

		time.Sleep(10 * time.Millisecond)

		// Second concurrent request should be rejected
		err := cb.Execute(ctx, func() error {
			return nil
		})

		var tooManyErr *ErrTooManyRequests
		if !errors.As(err, &tooManyErr) {
			t.Errorf("Expected ErrTooManyRequests, got %v", err)
		}

		<-done
	})
}

func TestCircuitBreaker_Reset(t *testing.T) {
	config := &CircuitBreakerConfig{
		Name:             "test",
		FailureThreshold: 2,
		Timeout:          1 * time.Second,
	}
	cb := NewCircuitBreaker(config)
	ctx := context.Background()

	// Open the circuit
	for i := 0; i < 2; i++ {
		cb.Execute(ctx, func() error {
			return errors.New("error")
		})
	}

	if cb.State() != StateOpen {
		t.Fatal("Circuit should be open")
	}

	// Reset circuit
	cb.Reset()

	if cb.State() != StateClosed {
		t.Errorf("Expected state=closed after reset, got %v", cb.State())
	}

	stats := cb.Stats()
	if stats.FailureCount != 0 {
		t.Errorf("Expected failure count=0 after reset, got %d", stats.FailureCount)
	}
}

func TestCircuitBreaker_IsAvailable(t *testing.T) {
	config := &CircuitBreakerConfig{
		Name:             "test",
		FailureThreshold: 2,
		Timeout:          100 * time.Millisecond,
	}
	cb := NewCircuitBreaker(config)
	ctx := context.Background()

	// Initially available
	if !cb.IsAvailable() {
		t.Error("Circuit should be available initially")
	}

	// Open the circuit
	for i := 0; i < 2; i++ {
		cb.Execute(ctx, func() error {
			return errors.New("error")
		})
	}

	// Not available when open
	if cb.IsAvailable() {
		t.Error("Circuit should not be available when open")
	}

	// Available after timeout
	time.Sleep(110 * time.Millisecond)
	if !cb.IsAvailable() {
		t.Error("Circuit should be available after timeout")
	}
}

func TestCircuitBreaker_Stats(t *testing.T) {
	cb := NewCircuitBreaker(DefaultCircuitBreakerConfig("test"))
	ctx := context.Background()

	// Execute some operations
	cb.Execute(ctx, func() error { return nil })
	cb.Execute(ctx, func() error { return errors.New("error") })
	cb.Execute(ctx, func() error { return nil })

	stats := cb.Stats()

	if stats.Name != "test" {
		t.Errorf("Expected name=test, got %s", stats.Name)
	}

	if stats.TotalRequests != 3 {
		t.Errorf("Expected 3 total requests, got %d", stats.TotalRequests)
	}

	if stats.TotalSuccesses != 2 {
		t.Errorf("Expected 2 successes, got %d", stats.TotalSuccesses)
	}

	if stats.TotalFailures != 1 {
		t.Errorf("Expected 1 failure, got %d", stats.TotalFailures)
	}
}

func TestCircuitBreaker_OnStateChange(t *testing.T) {
	stateChanges := []struct {
		from CircuitState
		to   CircuitState
	}{}

	config := &CircuitBreakerConfig{
		Name:             "test",
		FailureThreshold: 2,
		Timeout:          50 * time.Millisecond,
		OnStateChange: func(name string, from, to CircuitState) {
			stateChanges = append(stateChanges, struct {
				from CircuitState
				to   CircuitState
			}{from, to})
		},
	}

	cb := NewCircuitBreaker(config)
	ctx := context.Background()

	// Open the circuit
	for i := 0; i < 2; i++ {
		cb.Execute(ctx, func() error {
			return errors.New("error")
		})
	}

	// Wait a bit for the callback to execute
	time.Sleep(10 * time.Millisecond)

	if len(stateChanges) == 0 {
		t.Error("Expected state change callback to be called")
	}

	if stateChanges[0].from != StateClosed || stateChanges[0].to != StateOpen {
		t.Errorf("Expected transition from closed to open, got %v to %v",
			stateChanges[0].from, stateChanges[0].to)
	}
}

func TestCircuitBreakerRegistry(t *testing.T) {
	registry := NewCircuitBreakerRegistry()

	// Get or create
	cb1 := registry.GetOrCreate("test1", nil)
	if cb1 == nil {
		t.Fatal("Expected circuit breaker to be created")
	}

	// Get existing
	cb2 := registry.GetOrCreate("test1", nil)
	if cb1 != cb2 {
		t.Error("Expected to get the same circuit breaker instance")
	}

	// Get non-existent
	cb3, exists := registry.Get("test2")
	if exists {
		t.Error("Expected circuit breaker to not exist")
	}
	if cb3 != nil {
		t.Error("Expected nil circuit breaker")
	}

	// Get all
	all := registry.All()
	if len(all) != 1 {
		t.Errorf("Expected 1 circuit breaker, got %d", len(all))
	}

	// Reset all
	ctx := context.Background()
	cb1.Execute(ctx, func() error {
		return errors.New("error")
	})

	registry.Reset()

	stats := cb1.Stats()
	if stats.FailureCount != 0 {
		t.Error("Expected failure count to be reset")
	}
}

func TestCircuitState_String(t *testing.T) {
	tests := []struct {
		state    CircuitState
		expected string
	}{
		{StateClosed, "closed"},
		{StateOpen, "open"},
		{StateHalfOpen, "half-open"},
		{CircuitState(999), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.state.String()
			if result != tt.expected {
				t.Errorf("String() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func BenchmarkCircuitBreaker_Execute(b *testing.B) {
	cb := NewCircuitBreaker(DefaultCircuitBreakerConfig("bench"))
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cb.Execute(ctx, func() error {
			return nil
		})
	}
}

func BenchmarkCircuitBreaker_ExecuteWithFailures(b *testing.B) {
	cb := NewCircuitBreaker(DefaultCircuitBreakerConfig("bench"))
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if i%2 == 0 {
			cb.Execute(ctx, func() error {
				return nil
			})
		} else {
			cb.Execute(ctx, func() error {
				return errors.New("error")
			})
		}
	}
}

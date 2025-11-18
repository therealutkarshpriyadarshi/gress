package errors

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// CircuitState represents the state of a circuit breaker
type CircuitState int

const (
	// StateClosed - circuit is closed, requests flow normally
	StateClosed CircuitState = iota
	// StateOpen - circuit is open, requests fail immediately
	StateOpen
	// StateHalfOpen - circuit is testing if the service has recovered
	StateHalfOpen
)

func (s CircuitState) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// CircuitBreakerConfig configures a circuit breaker
type CircuitBreakerConfig struct {
	// Name of the circuit breaker for identification
	Name string
	// FailureThreshold is the number of failures before opening the circuit
	FailureThreshold uint32
	// SuccessThreshold is the number of successes in half-open before closing
	SuccessThreshold uint32
	// Timeout is how long to wait in open state before trying half-open
	Timeout time.Duration
	// MaxConcurrentRequests in half-open state
	MaxConcurrentRequests uint32
	// OnStateChange is called when circuit state changes (optional)
	OnStateChange func(name string, from, to CircuitState)
}

// DefaultCircuitBreakerConfig returns sensible defaults
func DefaultCircuitBreakerConfig(name string) *CircuitBreakerConfig {
	return &CircuitBreakerConfig{
		Name:                  name,
		FailureThreshold:      5,
		SuccessThreshold:      2,
		Timeout:               60 * time.Second,
		MaxConcurrentRequests: 1,
	}
}

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
	config *CircuitBreakerConfig

	mu                sync.RWMutex
	state             CircuitState
	failureCount      uint32
	successCount      uint32
	lastFailureTime   time.Time
	lastStateChange   time.Time
	halfOpenRequests  uint32
	totalRequests     uint64
	totalFailures     uint64
	totalSuccesses    uint64
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(config *CircuitBreakerConfig) *CircuitBreaker {
	return &CircuitBreaker{
		config:          config,
		state:           StateClosed,
		lastStateChange: time.Now(),
	}
}

// ErrCircuitOpen is returned when the circuit is open
type ErrCircuitOpen struct {
	CircuitName string
	OpenedAt    time.Time
}

func (e *ErrCircuitOpen) Error() string {
	return fmt.Sprintf("circuit breaker '%s' is open (opened at %s)", e.CircuitName, e.OpenedAt.Format(time.RFC3339))
}

// ErrTooManyRequests is returned when too many requests in half-open state
type ErrTooManyRequests struct {
	CircuitName string
}

func (e *ErrTooManyRequests) Error() string {
	return fmt.Sprintf("circuit breaker '%s' has too many concurrent requests in half-open state", e.CircuitName)
}

// Execute executes an operation through the circuit breaker
func (cb *CircuitBreaker) Execute(ctx context.Context, operation func() error) error {
	// Check if we can execute
	if err := cb.beforeRequest(); err != nil {
		return err
	}

	// Execute the operation
	err := operation()

	// Record the result
	cb.afterRequest(err)

	return err
}

// beforeRequest checks if a request can proceed
func (cb *CircuitBreaker) beforeRequest() error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.totalRequests++

	switch cb.state {
	case StateClosed:
		// Request can proceed
		return nil

	case StateOpen:
		// Check if timeout has elapsed
		if time.Since(cb.lastFailureTime) > cb.config.Timeout {
			// Transition to half-open
			cb.setState(StateHalfOpen)
			cb.halfOpenRequests = 1
			return nil
		}
		// Circuit is open, reject request
		return &ErrCircuitOpen{
			CircuitName: cb.config.Name,
			OpenedAt:    cb.lastFailureTime,
		}

	case StateHalfOpen:
		// Check if we can accept more requests
		if cb.halfOpenRequests >= cb.config.MaxConcurrentRequests {
			return &ErrTooManyRequests{
				CircuitName: cb.config.Name,
			}
		}
		cb.halfOpenRequests++
		return nil

	default:
		return fmt.Errorf("unknown circuit state: %d", cb.state)
	}
}

// afterRequest records the result of a request
func (cb *CircuitBreaker) afterRequest(err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err == nil {
		cb.onSuccess()
	} else {
		cb.onFailure()
	}
}

// onSuccess handles a successful request
func (cb *CircuitBreaker) onSuccess() {
	cb.totalSuccesses++

	switch cb.state {
	case StateClosed:
		// Reset failure count on success
		cb.failureCount = 0

	case StateHalfOpen:
		cb.successCount++
		if cb.successCount >= cb.config.SuccessThreshold {
			// Enough successes, close the circuit
			cb.setState(StateClosed)
			cb.failureCount = 0
			cb.successCount = 0
			cb.halfOpenRequests = 0
		} else {
			// Decrement half-open requests
			if cb.halfOpenRequests > 0 {
				cb.halfOpenRequests--
			}
		}
	}
}

// onFailure handles a failed request
func (cb *CircuitBreaker) onFailure() {
	cb.totalFailures++
	cb.lastFailureTime = time.Now()

	switch cb.state {
	case StateClosed:
		cb.failureCount++
		if cb.failureCount >= cb.config.FailureThreshold {
			// Too many failures, open the circuit
			cb.setState(StateOpen)
			cb.successCount = 0
		}

	case StateHalfOpen:
		// Any failure in half-open state reopens the circuit
		cb.setState(StateOpen)
		cb.successCount = 0
		cb.halfOpenRequests = 0
	}
}

// setState transitions the circuit to a new state
func (cb *CircuitBreaker) setState(newState CircuitState) {
	oldState := cb.state
	cb.state = newState
	cb.lastStateChange = time.Now()

	// Call state change callback if configured
	if cb.config.OnStateChange != nil && oldState != newState {
		// Call callback without holding lock to prevent deadlocks
		go cb.config.OnStateChange(cb.config.Name, oldState, newState)
	}
}

// State returns the current state of the circuit breaker
func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// Stats returns statistics about the circuit breaker
type CircuitBreakerStats struct {
	Name            string
	State           CircuitState
	FailureCount    uint32
	SuccessCount    uint32
	TotalRequests   uint64
	TotalFailures   uint64
	TotalSuccesses  uint64
	LastStateChange time.Time
	LastFailureTime time.Time
}

// Stats returns current statistics
func (cb *CircuitBreaker) Stats() CircuitBreakerStats {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	return CircuitBreakerStats{
		Name:            cb.config.Name,
		State:           cb.state,
		FailureCount:    cb.failureCount,
		SuccessCount:    cb.successCount,
		TotalRequests:   cb.totalRequests,
		TotalFailures:   cb.totalFailures,
		TotalSuccesses:  cb.totalSuccesses,
		LastStateChange: cb.lastStateChange,
		LastFailureTime: cb.lastFailureTime,
	}
}

// Reset resets the circuit breaker to closed state
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	oldState := cb.state
	cb.state = StateClosed
	cb.failureCount = 0
	cb.successCount = 0
	cb.halfOpenRequests = 0
	cb.lastStateChange = time.Now()

	if cb.config.OnStateChange != nil && oldState != StateClosed {
		go cb.config.OnStateChange(cb.config.Name, oldState, StateClosed)
	}
}

// IsAvailable checks if the circuit breaker will allow requests
func (cb *CircuitBreaker) IsAvailable() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	switch cb.state {
	case StateClosed:
		return true
	case StateHalfOpen:
		return cb.halfOpenRequests < cb.config.MaxConcurrentRequests
	case StateOpen:
		return time.Since(cb.lastFailureTime) > cb.config.Timeout
	default:
		return false
	}
}

// String returns a string representation of the circuit breaker
func (cb *CircuitBreaker) String() string {
	stats := cb.Stats()
	return fmt.Sprintf(
		"CircuitBreaker{Name: %s, State: %s, Failures: %d/%d, Successes: %d/%d, TotalRequests: %d}",
		stats.Name,
		stats.State,
		stats.FailureCount,
		cb.config.FailureThreshold,
		stats.SuccessCount,
		cb.config.SuccessThreshold,
		stats.TotalRequests,
	)
}

// CircuitBreakerRegistry manages multiple circuit breakers
type CircuitBreakerRegistry struct {
	mu       sync.RWMutex
	breakers map[string]*CircuitBreaker
}

// NewCircuitBreakerRegistry creates a new registry
func NewCircuitBreakerRegistry() *CircuitBreakerRegistry {
	return &CircuitBreakerRegistry{
		breakers: make(map[string]*CircuitBreaker),
	}
}

// GetOrCreate gets or creates a circuit breaker
func (r *CircuitBreakerRegistry) GetOrCreate(name string, config *CircuitBreakerConfig) *CircuitBreaker {
	r.mu.RLock()
	cb, exists := r.breakers[name]
	r.mu.RUnlock()

	if exists {
		return cb
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Double-check after acquiring write lock
	cb, exists = r.breakers[name]
	if exists {
		return cb
	}

	// Create new circuit breaker
	if config == nil {
		config = DefaultCircuitBreakerConfig(name)
	} else {
		config.Name = name
	}

	cb = NewCircuitBreaker(config)
	r.breakers[name] = cb
	return cb
}

// Get retrieves a circuit breaker by name
func (r *CircuitBreakerRegistry) Get(name string) (*CircuitBreaker, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	cb, exists := r.breakers[name]
	return cb, exists
}

// All returns all circuit breakers
func (r *CircuitBreakerRegistry) All() map[string]*CircuitBreaker {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string]*CircuitBreaker, len(r.breakers))
	for k, v := range r.breakers {
		result[k] = v
	}
	return result
}

// Reset resets all circuit breakers
func (r *CircuitBreakerRegistry) Reset() {
	r.mu.RLock()
	breakers := make([]*CircuitBreaker, 0, len(r.breakers))
	for _, cb := range r.breakers {
		breakers = append(breakers, cb)
	}
	r.mu.RUnlock()

	for _, cb := range breakers {
		cb.Reset()
	}
}

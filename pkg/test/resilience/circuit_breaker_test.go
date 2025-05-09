package resilience

import (
	"context"
	"errors"
	"testing"
	"time"
)

// CircuitBreakerState represents the state of a circuit breaker
type CircuitBreakerState int

// Circuit breaker states
const (
	StateClosed CircuitBreakerState = iota
	StateOpen
	StateHalfOpen
)

// Common errors
var (
	ErrCircuitOpen = errors.New("circuit breaker is open")
)

// CircuitBreakerConfig contains configuration for a circuit breaker
type CircuitBreakerConfig struct {
	ErrorThresholdPercentage int
	OpenStateSeconds         int
	HalfOpenRequestThreshold int
}

// DefaultCircuitBreakerConfig returns the default circuit breaker configuration
func DefaultCircuitBreakerConfig() CircuitBreakerConfig {
	return CircuitBreakerConfig{
		ErrorThresholdPercentage: 50,
		OpenStateSeconds:         30,
		HalfOpenRequestThreshold: 5,
	}
}

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
	name       string
	state      CircuitBreakerState
	config     CircuitBreakerConfig
	failures   int
	successes  int
	attemptsSinceLastStateChange int
	lastStateChangeTime          time.Time
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(name string, config CircuitBreakerConfig) *CircuitBreaker {
	return &CircuitBreaker{
		name:                name,
		state:               StateClosed,
		config:              config,
		lastStateChangeTime: time.Now(),
	}
}

// Execute executes a function with circuit breaker protection
func (c *CircuitBreaker) Execute(ctx context.Context, fn func(context.Context) error) error {
	// Check if circuit is open
	if c.state == StateOpen {
		// Check if enough time has passed to try again
		if time.Since(c.lastStateChangeTime) > time.Duration(c.config.OpenStateSeconds)*time.Second {
			// Transition to half-open state
			c.changeState(StateHalfOpen)
		} else {
			// Circuit is still open, fail fast
			return ErrCircuitOpen
		}
	}

	// Execute the function
	err := fn(ctx)

	// Update circuit breaker state based on result
	if err != nil {
		c.failures++
		c.attemptsSinceLastStateChange++

		// Check if threshold is reached
		if c.state == StateClosed {
			errorPercentage := (c.failures * 100) / c.attemptsSinceLastStateChange
			if errorPercentage >= c.config.ErrorThresholdPercentage {
				c.changeState(StateOpen)
			}
		} else if c.state == StateHalfOpen {
			// Any failure in half-open state opens the circuit again
			c.changeState(StateOpen)
		}
	} else {
		c.successes++
		c.attemptsSinceLastStateChange++

		// Check if we should close the circuit
		if c.state == StateHalfOpen && c.successes >= c.config.HalfOpenRequestThreshold {
			c.changeState(StateClosed)
		}
	}

	return err
}

// changeState changes the circuit breaker state
func (c *CircuitBreaker) changeState(newState CircuitBreakerState) {
	c.state = newState
	c.lastStateChangeTime = time.Now()
	c.failures = 0
	c.successes = 0
	c.attemptsSinceLastStateChange = 0
}

// State returns the current state of the circuit breaker
func (c *CircuitBreaker) State() CircuitBreakerState {
	return c.state
}

// TestCircuitBreaker tests the circuit breaker implementation
func TestCircuitBreaker(t *testing.T) {
	// Create a circuit breaker with quick transitions for testing
	cb := NewCircuitBreaker("test", CircuitBreakerConfig{
		ErrorThresholdPercentage: 50,
		OpenStateSeconds:         1,  // Quick transition for testing
		HalfOpenRequestThreshold: 2,
	})

	// Test initial state
	if cb.State() != StateClosed {
		t.Errorf("Initial state should be Closed, got %v", cb.State())
	}

	// Test successful execution
	err := cb.Execute(context.Background(), func(ctx context.Context) error {
		return nil
	})

	if err != nil {
		t.Errorf("Execution should succeed, got error: %v", err)
	}

	// Test circuit opening
	// We need 1 success + 2 failures to reach 50% error rate with 6 attempts
	for i := 0; i < 5; i++ {
		cb.Execute(context.Background(), func(ctx context.Context) error {
			return errors.New("test error")
		})
	}

	// Circuit should be open now
	if cb.State() != StateOpen {
		t.Errorf("State should be Open after failures, got %v", cb.State())
	}

	// Test fast fail when circuit is open
	err = cb.Execute(context.Background(), func(ctx context.Context) error {
		t.Error("This function should not be executed when circuit is open")
		return nil
	})

	if err != ErrCircuitOpen {
		t.Errorf("Expected ErrCircuitOpen, got %v", err)
	}

	// Wait for circuit to transition to half-open
	time.Sleep(1100 * time.Millisecond)

	// First execution in half-open state
	cb.Execute(context.Background(), func(ctx context.Context) error {
		return nil
	})

	// Second successful execution should close the circuit
	cb.Execute(context.Background(), func(ctx context.Context) error {
		return nil
	})

	// Circuit should be closed now
	if cb.State() != StateClosed {
		t.Errorf("State should be Closed after successes in half-open state, got %v", cb.State())
	}
}

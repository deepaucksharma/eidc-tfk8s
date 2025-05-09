package resilience

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// ErrCircuitOpen is returned when the circuit is open and requests are not allowed
	ErrCircuitOpen = errors.New("circuit breaker is open")
)

// CircuitBreakerState represents the current state of the circuit breaker
type CircuitBreakerState int

const (
	// StateClosed means the circuit is closed and requests are allowed
	StateClosed CircuitBreakerState = iota
	// StateOpen means the circuit is open and requests are not allowed
	StateOpen
	// StateHalfOpen means the circuit is allowing a limited number of requests to test recovery
	StateHalfOpen
)

// CircuitBreakerConfig contains configuration parameters for the circuit breaker
type CircuitBreakerConfig struct {
	// ErrorThresholdPercentage is the percentage of requests that can fail before the circuit opens
	ErrorThresholdPercentage int
	// MinimumRequestCount is the minimum number of requests needed before error rate is calculated
	MinimumRequestCount int
	// OpenStateSeconds is the duration the circuit remains open before moving to half-open
	OpenStateSeconds int
	// HalfOpenRequestThreshold is the number of successful requests needed to close the circuit
	HalfOpenRequestThreshold int
}

// DefaultCircuitBreakerConfig returns a default configuration
func DefaultCircuitBreakerConfig() CircuitBreakerConfig {
	return CircuitBreakerConfig{
		ErrorThresholdPercentage: 50,
		MinimumRequestCount:      20,
		OpenStateSeconds:         30,
		HalfOpenRequestThreshold: 5,
	}
}

// CircuitBreaker implements a circuit breaker pattern
type CircuitBreaker struct {
	name                      string
	state                     CircuitBreakerState
	config                    CircuitBreakerConfig
	mutex                     sync.RWMutex
	successCount              int
	requestCount              int
	failureCount              int
	lastStateChangeTime       time.Time
	halfOpenSuccessfulRequest int

	// Metrics
	stateGauge        prometheus.Gauge
	requestsTotal     prometheus.Counter
	failuresTotal     prometheus.Counter
	openStateTotal    prometheus.Counter
	stateChangesTotal *prometheus.CounterVec
}

// NewCircuitBreaker creates a new circuit breaker with the given configuration
func NewCircuitBreaker(name string, config CircuitBreakerConfig) *CircuitBreaker {
	cb := &CircuitBreaker{
		name:                name,
		state:               StateClosed,
		config:              config,
		lastStateChangeTime: time.Now(),

		// Initialize metrics
		stateGauge: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "fb_cb_state",
			Help: "Current state of the circuit breaker (0=closed, 1=open, 2=half-open)",
			ConstLabels: prometheus.Labels{
				"fb_name": name,
			},
		}),
		requestsTotal: promauto.NewCounter(prometheus.CounterOpts{
			Name: "fb_cb_requests_total",
			Help: "Total number of requests seen by the circuit breaker",
			ConstLabels: prometheus.Labels{
				"fb_name": name,
			},
		}),
		failuresTotal: promauto.NewCounter(prometheus.CounterOpts{
			Name: "fb_cb_failures_total",
			Help: "Total number of failures seen by the circuit breaker",
			ConstLabels: prometheus.Labels{
				"fb_name": name,
			},
		}),
		openStateTotal: promauto.NewCounter(prometheus.CounterOpts{
			Name: "fb_cb_open_seconds_total",
			Help: "Total number of seconds the circuit breaker has been open",
			ConstLabels: prometheus.Labels{
				"fb_name": name,
			},
		}),
		stateChangesTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "fb_cb_state_changes_total",
			Help: "Total number of circuit breaker state transitions",
			ConstLabels: prometheus.Labels{
				"fb_name": name,
			},
		}, []string{"from_state", "to_state"}),
	}

	// Start a goroutine to track open state time
	go cb.trackOpenState()

	return cb
}

// Execute executes the given function within the circuit breaker
func (cb *CircuitBreaker) Execute(ctx context.Context, fn func(ctx context.Context) error) error {
	if !cb.allowRequest() {
		return ErrCircuitOpen
	}

	err := fn(ctx)
	cb.recordResult(err == nil)
	return err
}

// allowRequest checks if a request should be allowed based on the current state
func (cb *CircuitBreaker) allowRequest() bool {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()

	switch cb.state {
	case StateClosed:
		return true
	case StateOpen:
		// Check if enough time has elapsed to transition to half-open
		if time.Since(cb.lastStateChangeTime) > time.Second*time.Duration(cb.config.OpenStateSeconds) {
			cb.mutex.RUnlock()
			cb.mutex.Lock()
			if cb.state == StateOpen {
				cb.transitionState(StateHalfOpen)
			}
			cb.mutex.Unlock()
			cb.mutex.RLock()
			return true
		}
		return false
	case StateHalfOpen:
		// Only allow a limited number of requests in half-open state
		return cb.halfOpenSuccessfulRequest < cb.config.HalfOpenRequestThreshold
	default:
		return true
	}
}

// recordResult records the result of a request and updates the circuit state
func (cb *CircuitBreaker) recordResult(success bool) {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	cb.requestsTotal.Inc()
	cb.requestCount++

	if !success {
		cb.failuresTotal.Inc()
		cb.failureCount++

		// If in half-open state, a single failure trips the circuit
		if cb.state == StateHalfOpen {
			cb.transitionState(StateOpen)
			return
		}
	} else if cb.state == StateHalfOpen {
		cb.halfOpenSuccessfulRequest++
		if cb.halfOpenSuccessfulRequest >= cb.config.HalfOpenRequestThreshold {
			cb.transitionState(StateClosed)
			return
		}
	}

	// Check error threshold for closed circuit
	if cb.state == StateClosed && cb.requestCount >= cb.config.MinimumRequestCount {
		errorRate := float64(cb.failureCount) / float64(cb.requestCount) * 100.0
		if errorRate >= float64(cb.config.ErrorThresholdPercentage) {
			cb.transitionState(StateOpen)
			return
		}
	}

	// Reset counts periodically
	if cb.state == StateClosed && cb.requestCount >= cb.config.MinimumRequestCount*2 {
		cb.resetCounts()
	}
}

// transitionState changes the state of the circuit breaker
func (cb *CircuitBreaker) transitionState(newState CircuitBreakerState) {
	oldState := cb.state
	cb.state = newState
	cb.lastStateChangeTime = time.Now()
	cb.stateGauge.Set(float64(newState))
	cb.stateChangesTotal.WithLabelValues(
		stateToString(oldState),
		stateToString(newState),
	).Inc()

	if newState == StateClosed {
		cb.resetCounts()
	} else if newState == StateHalfOpen {
		cb.halfOpenSuccessfulRequest = 0
	}
}

// resetCounts resets the success and failure counts
func (cb *CircuitBreaker) resetCounts() {
	cb.successCount = 0
	cb.failureCount = 0
	cb.requestCount = 0
}

// trackOpenState monitors and records the total time spent in open state
func (cb *CircuitBreaker) trackOpenState() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		cb.mutex.RLock()
		if cb.state == StateOpen {
			cb.openStateTotal.Inc()
		}
		cb.mutex.RUnlock()
	}
}

// GetState returns the current state of the circuit breaker
func (cb *CircuitBreaker) GetState() CircuitBreakerState {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()
	return cb.state
}

// stateToString converts a CircuitBreakerState to a string
func stateToString(state CircuitBreakerState) string {
	switch state {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half_open"
	default:
		return "unknown"
	}
}

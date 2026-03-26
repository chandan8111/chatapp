package resilience

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

// CircuitBreakerState represents the state of a circuit breaker
type CircuitBreakerState int

const (
	StateClosed CircuitBreakerState = iota
	StateOpen
	StateHalfOpen
)

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
	name           string
	maxFailures    int
	timeout        time.Duration
	resetTimeout   time.Duration
	state          CircuitBreakerState
	failures       int
	lastFailTime   time.Time
	mu             sync.RWMutex
	logger         *zap.Logger
	
	// Metrics
	stateGauge     prometheus.Gauge
	failureCounter prometheus.Counter
	successCounter prometheus.Counter
}

// CircuitBreakerConfig holds configuration for a circuit breaker
type CircuitBreakerConfig struct {
	Name         string
	MaxFailures  int
	Timeout      time.Duration
	ResetTimeout time.Duration
	Logger       *zap.Logger
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(config CircuitBreakerConfig) *CircuitBreaker {
	cb := &CircuitBreaker{
		name:         config.Name,
		maxFailures:  config.MaxFailures,
		timeout:      config.Timeout,
		resetTimeout: config.ResetTimeout,
		state:        StateClosed,
		logger:       config.Logger,
	}

	// Initialize metrics
	cb.stateGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name:        "circuit_breaker_state",
		Help:        "State of the circuit breaker (0=closed, 1=open, 2=half-open)",
		ConstLabels: prometheus.Labels{"circuit": config.Name},
	})

	cb.failureCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name:        "circuit_breaker_failures_total",
		Help:        "Total number of failures in circuit breaker",
		ConstLabels: prometheus.Labels{"circuit": config.Name},
	})

	cb.successCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name:        "circuit_breaker_successes_total",
		Help:        "Total number of successes in circuit breaker",
		ConstLabels: prometheus.Labels{"circuit": config.Name},
	})

	return cb
}

// Execute executes a function through the circuit breaker
func (cb *CircuitBreaker) Execute(ctx context.Context, fn func() error) error {
	if !cb.canExecute() {
		cb.logger.Warn("Circuit breaker is open, rejecting request",
			zap.String("circuit", cb.name),
			zap.String("state", cb.state.String()),
		)
		return errors.New("circuit breaker is open")
	}

	// Execute with timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, cb.timeout)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- fn()
	}()

	select {
	case err := <-done:
		if err != nil {
			cb.onFailure()
			return err
		}
		cb.onSuccess()
		return nil
	case <-timeoutCtx.Done():
		cb.onFailure()
		return errors.New("operation timed out")
	}
}

// canExecute determines if the operation can be executed
func (cb *CircuitBreaker) canExecute() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	switch cb.state {
	case StateClosed:
		return true
	case StateOpen:
		// Check if reset timeout has passed
		if time.Since(cb.lastFailTime) > cb.resetTimeout {
			cb.mu.RUnlock()
			cb.mu.Lock()
			cb.state = StateHalfOpen
			cb.stateGauge.Set(2)
			cb.logger.Info("Circuit breaker transitioning to half-open",
				zap.String("circuit", cb.name),
			)
			cb.mu.Unlock()
			cb.mu.RLock()
			return true
		}
		return false
	case StateHalfOpen:
		return true
	default:
		return false
	}
}

// onSuccess handles successful operation
func (cb *CircuitBreaker) onSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.successCounter.Inc()

	if cb.state == StateHalfOpen {
		cb.state = StateClosed
		cb.failures = 0
		cb.stateGauge.Set(0)
		cb.logger.Info("Circuit breaker closed after successful operation",
			zap.String("circuit", cb.name),
		)
	}
}

// onFailure handles failed operation
func (cb *CircuitBreaker) onFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures++
	cb.lastFailTime = time.Now()
	cb.failureCounter.Inc()

	if cb.failures >= cb.maxFailures {
		cb.state = StateOpen
		cb.stateGauge.Set(1)
		cb.logger.Warn("Circuit breaker opened due to failures",
			zap.String("circuit", cb.name),
			zap.Int("failures", cb.failures),
			zap.Int("max_failures", cb.maxFailures),
		)
	}
}

// GetState returns the current state of the circuit breaker
func (cb *CircuitBreaker) GetState() CircuitBreakerState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// Reset resets the circuit breaker to closed state
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.state = StateClosed
	cb.failures = 0
	cb.lastFailTime = time.Time{}
	cb.stateGauge.Set(0)
	cb.logger.Info("Circuit breaker reset to closed state",
		zap.String("circuit", cb.name),
	)
}

// GetMetrics returns Prometheus metrics for the circuit breaker
func (cb *CircuitBreaker) GetMetrics() []prometheus.Collector {
	return []prometheus.Collector{
		cb.stateGauge,
		cb.failureCounter,
		cb.successCounter,
	}
}

// String returns string representation of the state
func (s CircuitBreakerState) String() string {
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

// RetryConfig holds configuration for retry logic
type RetryConfig struct {
	MaxRetries      int
	InitialDelay    time.Duration
	MaxDelay        time.Duration
	BackoffFactor   float64
	RetryableErrors []error
}

// Retry executes a function with retry logic
func Retry(ctx context.Context, config RetryConfig, fn func() error, logger *zap.Logger) error {
	var lastErr error
	delay := config.InitialDelay

	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
			
			logger.Debug("Retrying operation",
				zap.Int("attempt", attempt),
				zap.Duration("delay", delay),
				zap.Error(lastErr),
			)
		}

		err := fn()
		if err == nil {
			return nil
		}

		lastErr = err

		// Check if error is retryable
		if !isRetryableError(err, config.RetryableErrors) {
			break
		}

		// Calculate next delay
		delay = time.Duration(float64(delay) * config.BackoffFactor)
		if delay > config.MaxDelay {
			delay = config.MaxDelay
		}
	}

	return lastErr
}

// isRetryableError checks if an error is retryable
func isRetryableError(err error, retryableErrors []error) bool {
	if len(retryableErrors) == 0 {
		// By default, retry all errors
		return true
	}

	for _, retryableErr := range retryableErrors {
		if errors.Is(err, retryableErr) {
			return true
		}
	}

	return false
}

// Bulkhead implements the bulkhead pattern to limit concurrent operations
type Bulkhead struct {
	semaphore chan struct{}
	name      string
	logger    *zap.Logger
	
	// Metrics
	activeGauge    prometheus.Gauge
	waitingGauge   prometheus.Gauge
	rejectedCounter prometheus.Counter
}

// BulkheadConfig holds configuration for a bulkhead
type BulkheadConfig struct {
	Name      string
	MaxConcurrent int
	Logger    *zap.Logger
}

// NewBulkhead creates a new bulkhead
func NewBulkhead(config BulkheadConfig) *Bulkhead {
	b := &Bulkhead{
		semaphore: make(chan struct{}, config.MaxConcurrent),
		name:      config.Name,
		logger:    config.Logger,
	}

	// Initialize metrics
	b.activeGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name:        "bulkhead_active_operations",
		Help:        "Number of active operations",
		ConstLabels: prometheus.Labels{"bulkhead": config.Name},
	})

	b.waitingGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name:        "bulkhead_waiting_operations",
		Help:        "Number of waiting operations",
		ConstLabels: prometheus.Labels{"bulkhead": config.Name},
	})

	b.rejectedCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name:        "bulkhead_rejected_operations_total",
		Help:        "Total number of rejected operations",
		ConstLabels: prometheus.Labels{"bulkhead": config.Name},
	})

	return b
}

// Execute executes a function through the bulkhead
func (b *Bulkhead) Execute(ctx context.Context, fn func() error) error {
	select {
	case b.semaphore <- struct{}{}:
		// Got semaphore
		b.activeGauge.Inc()
		defer func() {
			<-b.semaphore
			b.activeGauge.Dec()
		}()
		
		return fn()
	case <-ctx.Done():
		b.rejectedCounter.Inc()
		b.logger.Warn("Bulkhead rejected operation due to context cancellation",
			zap.String("bulkhead", b.name),
		)
		return ctx.Err()
	default:
		// Bulkhead is full
		b.rejectedCounter.Inc()
		b.logger.Warn("Bulkhead rejected operation due to capacity limit",
			zap.String("bulkhead", b.name),
		)
		return errors.New("bulkhead capacity exceeded")
	}
}

// GetMetrics returns Prometheus metrics for the bulkhead
func (b *Bulkhead) GetMetrics() []prometheus.Collector {
	return []prometheus.Collector{
		b.activeGauge,
		b.waitingGauge,
		b.rejectedCounter,
	}
}

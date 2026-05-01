package middleware

import (
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

type CircuitState int

const (
	StateClosed CircuitState = iota
	StateOpen
	StateHalfOpen
)

type CircuitBreaker struct {
	mu           sync.Mutex
	state        CircuitState
	failureCount int
	successCount int
	threshold    int
	resetTimeout time.Duration
	lastFailure  time.Time
	halfOpenMax  int
}

func NewCircuitBreaker(threshold int, resetTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		state:        StateClosed,
		threshold:    threshold,
		resetTimeout: resetTimeout,
		halfOpenMax:  3,
	}
}

func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	switch cb.state {
	case StateClosed:
		return true
	case StateOpen:
		if time.Since(cb.lastFailure) > cb.resetTimeout {
			cb.state = StateHalfOpen
			cb.successCount = 0
			log.Info("circuit breaker: open -> half-open")
			return true
		}
		return false
	case StateHalfOpen:
		return cb.successCount < cb.halfOpenMax
	}
	return false
}

func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	if cb.state == StateHalfOpen {
		cb.successCount++
		if cb.successCount >= cb.halfOpenMax {
			cb.state = StateClosed
			cb.failureCount = 0
			log.Info("circuit breaker: half-open -> closed")
		}
	}
	if cb.state == StateClosed {
		cb.failureCount = 0
	}
}

func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failureCount++
	cb.lastFailure = time.Now()
	if cb.state == StateHalfOpen {
		cb.state = StateOpen
		log.Warn("circuit breaker: half-open -> open")
	} else if cb.failureCount >= cb.threshold {
		cb.state = StateOpen
		log.Warnf("circuit breaker: closed -> open (failures=%d)", cb.failureCount)
	}
}

func (cb *CircuitBreaker) State() string {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	switch cb.state {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	}
	return "unknown"
}

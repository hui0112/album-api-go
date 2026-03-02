package main

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// ============================================================
// Circuit Breaker
// ============================================================

type CircuitState int

const (
	StateClosed   CircuitState = iota // Normal - requests pass through
	StateOpen                         // Tripped - requests fail fast
	StateHalfOpen                     // Testing - limited requests allowed
)

func (s CircuitState) String() string {
	switch s {
	case StateClosed:
		return "CLOSED"
	case StateOpen:
		return "OPEN"
	case StateHalfOpen:
		return "HALF_OPEN"
	default:
		return "UNKNOWN"
	}
}

type CircuitBreaker struct {
	mu sync.Mutex

	state          CircuitState
	failureCount   int
	successCount   int
	failThreshold  int           // failures before opening
	cooldownPeriod time.Duration // how long to stay open
	halfOpenMax    int           // successes needed to close again
	lastFailTime   time.Time

	// Metrics
	totalRequests atomic.Int64
	totalFailures atomic.Int64
	totalRejected atomic.Int64
}

func NewCircuitBreaker(failThreshold int, cooldown time.Duration, halfOpenMax int) *CircuitBreaker {
	return &CircuitBreaker{
		state:          StateClosed,
		failThreshold:  failThreshold,
		cooldownPeriod: cooldown,
		halfOpenMax:    halfOpenMax,
	}
}

// AllowRequest checks if a request should be allowed through
func (cb *CircuitBreaker) AllowRequest() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.totalRequests.Add(1)

	switch cb.state {
	case StateClosed:
		return true
	case StateOpen:
		// Check if cooldown has elapsed
		if time.Since(cb.lastFailTime) > cb.cooldownPeriod {
			cb.state = StateHalfOpen
			cb.successCount = 0
			fmt.Println("[CircuitBreaker] State: OPEN -> HALF_OPEN")
			return true
		}
		cb.totalRejected.Add(1)
		return false
	case StateHalfOpen:
		return true
	}
	return false
}

// RecordSuccess records a successful call
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateHalfOpen:
		cb.successCount++
		if cb.successCount >= cb.halfOpenMax {
			cb.state = StateClosed
			cb.failureCount = 0
			fmt.Println("[CircuitBreaker] State: HALF_OPEN -> CLOSED (recovered!)")
		}
	case StateClosed:
		cb.failureCount = 0 // reset on success
	}
}

// RecordFailure records a failed call
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.totalFailures.Add(1)
	cb.lastFailTime = time.Now()

	switch cb.state {
	case StateClosed:
		cb.failureCount++
		if cb.failureCount >= cb.failThreshold {
			cb.state = StateOpen
			fmt.Printf("[CircuitBreaker] State: CLOSED -> OPEN (failures: %d)\n", cb.failureCount)
		}
	case StateHalfOpen:
		cb.state = StateOpen
		fmt.Println("[CircuitBreaker] State: HALF_OPEN -> OPEN (probe failed)")
	}
}

// Status returns the current state as a map
func (cb *CircuitBreaker) Status() map[string]interface{} {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	return map[string]interface{}{
		"state":          cb.state.String(),
		"failure_count":  cb.failureCount,
		"success_count":  cb.successCount,
		"fail_threshold": cb.failThreshold,
		"cooldown_sec":   cb.cooldownPeriod.Seconds(),
		"total_requests": cb.totalRequests.Load(),
		"total_failures": cb.totalFailures.Load(),
		"total_rejected": cb.totalRejected.Load(),
	}
}

// ============================================================
// Bulkhead (Semaphore-based concurrency limiter)
// ============================================================

type Bulkhead struct {
	sem           chan struct{}
	maxConcurrent int

	totalAccepted atomic.Int64
	totalRejected atomic.Int64
}

func NewBulkhead(maxConcurrent int) *Bulkhead {
	return &Bulkhead{
		sem:           make(chan struct{}, maxConcurrent),
		maxConcurrent: maxConcurrent,
	}
}

// Acquire tries to acquire a slot. Returns false if bulkhead is full.
func (b *Bulkhead) Acquire() bool {
	select {
	case b.sem <- struct{}{}:
		b.totalAccepted.Add(1)
		return true
	default:
		b.totalRejected.Add(1)
		return false
	}
}

// Release gives back a slot
func (b *Bulkhead) Release() {
	<-b.sem
}

// Status returns current bulkhead metrics
func (b *Bulkhead) Status() map[string]interface{} {
	return map[string]interface{}{
		"max_concurrent":  b.maxConcurrent,
		"current_in_use":  len(b.sem),
		"available_slots": b.maxConcurrent - len(b.sem),
		"total_accepted":  b.totalAccepted.Load(),
		"total_rejected":  b.totalRejected.Load(),
	}
}

// ============================================================
// Fail Fast (Health check gate)
// ============================================================

type FailFast struct {
	healthy atomic.Bool

	totalChecks   atomic.Int64
	totalRejected atomic.Int64
}

func NewFailFast() *FailFast {
	ff := &FailFast{}
	ff.healthy.Store(true)
	return ff
}

// SetHealthy updates the downstream health status
func (ff *FailFast) SetHealthy(h bool) {
	ff.healthy.Store(h)
}

// IsHealthy checks if downstream is considered healthy
func (ff *FailFast) IsHealthy() bool {
	ff.totalChecks.Add(1)
	if !ff.healthy.Load() {
		ff.totalRejected.Add(1)
		return false
	}
	return true
}

// Status returns fail fast metrics
func (ff *FailFast) Status() map[string]interface{} {
	return map[string]interface{}{
		"downstream_healthy": ff.healthy.Load(),
		"total_checks":       ff.totalChecks.Load(),
		"total_rejected":     ff.totalRejected.Load(),
	}
}

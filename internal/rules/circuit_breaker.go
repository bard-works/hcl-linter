package rules

import (
	"context"
	"os"
	"sync"
	"time"
)

type CircuitBreaker struct {
	failures int
	lastFail time.Time
	mu       struct {
		sync.Mutex
	}
}

func NewCircuitBreaker() *CircuitBreaker {
	return &CircuitBreaker{}
}

func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	if cb.failures >= 3 {
		if time.Since(cb.lastFail) < time.Minute {
			return false
		}
		cb.failures = 0
	}
	return true
}

func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	cb.failures = 0
	cb.mu.Unlock()
}

func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	cb.failures++
	cb.lastFail = time.Now()
	cb.mu.Unlock()
}

func withTimeout[T any](op string, fn func() (T, error)) (T, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	done := make(chan struct{})
	var result T
	var err error

	go func() {
		result, err = fn()
		close(done)
	}()

	select {
	case <-done:
		return result, err
	case <-ctx.Done():
		var zero T
		return zero, &TimeoutError{Operation: op}
	}
}

type TimeoutError struct {
	Operation string
}

func (e *TimeoutError) Error() string {
	return "operation timed out: " + e.Operation
}

func safeStat(path string) (os.FileInfo, error) {
	return withTimeout("stat "+path, func() (os.FileInfo, error) {
		return os.Stat(path)
	})
}

func safeReadFile(path string) ([]byte, error) {
	return withTimeout("read "+path, func() ([]byte, error) {
		return os.ReadFile(path)
	})
}

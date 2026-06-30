package rules

import (
	"errors"
	"io/fs"
	"os"
	"sync"
	"time"
)

const (
	// cbFailureThreshold is the number of consecutive infrastructure failures
	// that opens the breaker.
	cbFailureThreshold = 3
	// cbCooldown is how long the breaker stays open before it allows a probe.
	cbCooldown = time.Minute
	// fsOpTimeout bounds how long a single filesystem operation may block before
	// it is treated as a failure.
	fsOpTimeout = 5 * time.Second
)

// ErrCircuitOpen is returned by Stat/ReadFile when the breaker is open. It is
// deliberately distinct from a "not found" error: os.IsNotExist(ErrCircuitOpen)
// is false, so callers never mistake a suspended check for a missing path.
var ErrCircuitOpen = errors.New("circuit breaker open: filesystem checks suspended after repeated failures")

// CircuitBreaker guards filesystem access for the dependency-resolving rules.
// On a slow or broken mount, repeated infrastructure failures open the breaker
// so subsequent operations fail fast instead of hammering the filesystem (and
// spawning a timeout goroutine per call). It closes again after a cooldown.
//
// A nil *CircuitBreaker is valid and behaves as an always-closed, no-op breaker
// that performs operations directly. This keeps hand-built test Contexts (which
// may leave Context.Breaker nil) working without special-casing.
type CircuitBreaker struct {
	mu       sync.Mutex
	failures int
	lastFail time.Time
}

func NewCircuitBreaker() *CircuitBreaker {
	return &CircuitBreaker{}
}

// Allow reports whether a new filesystem operation may proceed. When the
// breaker has been open longer than cbCooldown it half-resets and allows a
// probe.
func (cb *CircuitBreaker) Allow() bool {
	if cb == nil {
		return true
	}
	cb.mu.Lock()
	defer cb.mu.Unlock()
	if cb.failures >= cbFailureThreshold {
		if time.Since(cb.lastFail) < cbCooldown {
			return false
		}
		cb.failures = 0
	}
	return true
}

// RecordSuccess clears the failure count.
func (cb *CircuitBreaker) RecordSuccess() {
	if cb == nil {
		return
	}
	cb.mu.Lock()
	cb.failures = 0
	cb.mu.Unlock()
}

// RecordFailure increments the failure count and stamps the failure time.
func (cb *CircuitBreaker) RecordFailure() {
	if cb == nil {
		return
	}
	cb.mu.Lock()
	cb.failures++
	cb.lastFail = time.Now()
	cb.mu.Unlock()
}

// Stat is a circuit-broken, timeout-bounded os.Stat.
func (cb *CircuitBreaker) Stat(path string) (os.FileInfo, error) {
	if cb == nil {
		return os.Stat(path)
	}
	if !cb.Allow() {
		return nil, ErrCircuitOpen
	}
	info, err := withTimeout("stat "+path, func() (os.FileInfo, error) {
		return os.Stat(path)
	})
	cb.record(err)
	return info, err
}

// ReadFile is a circuit-broken, timeout-bounded os.ReadFile.
func (cb *CircuitBreaker) ReadFile(path string) ([]byte, error) {
	if cb == nil {
		return os.ReadFile(path)
	}
	if !cb.Allow() {
		return nil, ErrCircuitOpen
	}
	data, err := withTimeout("read "+path, func() ([]byte, error) {
		return os.ReadFile(path)
	})
	cb.record(err)
	return data, err
}

// record feeds an operation's result to the breaker, counting only
// infrastructure failures toward opening it.
func (cb *CircuitBreaker) record(err error) {
	if isInfraFailure(err) {
		cb.RecordFailure()
	} else {
		cb.RecordSuccess()
	}
}

// isInfraFailure decides whether an error reflects a broken filesystem (and so
// should count toward opening the breaker) rather than a normal, expected
// result. A missing path is the very thing the dependency rules look for, so it
// must NOT trip the breaker.
func isInfraFailure(err error) bool {
	if err == nil {
		return false
	}
	var te *TimeoutError
	if errors.As(err, &te) {
		return true
	}
	if errors.Is(err, fs.ErrNotExist) {
		return false
	}
	return true
}

// withTimeout runs fn, returning a *TimeoutError if it does not finish within
// fsOpTimeout. The result channel is buffered so the worker goroutine can always
// send and exit even after the timeout fired - there are no shared mutable vars
// (no data race) and no permanent goroutine leak. A genuinely blocked syscall
// (which cannot be cancelled) keeps its goroutine only until the syscall itself
// returns; the breaker opens after cbFailureThreshold such events so no new
// workers are spawned while the filesystem is unhealthy.
func withTimeout[T any](op string, fn func() (T, error)) (T, error) {
	type result struct {
		value T
		err   error
	}
	ch := make(chan result, 1)
	go func() {
		value, err := fn()
		ch <- result{value: value, err: err}
	}()

	select {
	case r := <-ch:
		return r.value, r.err
	case <-time.After(fsOpTimeout):
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

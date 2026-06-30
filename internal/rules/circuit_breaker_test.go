package rules

import (
	"errors"
	"io/fs"
	"os"
	"testing"
	"time"
)

func TestCircuitBreaker_Allow(t *testing.T) {
	cb := NewCircuitBreaker()

	if !cb.Allow() {
		t.Error("should allow when no failures")
	}
}

func TestCircuitBreaker_RecordSuccess(t *testing.T) {
	cb := NewCircuitBreaker()
	cb.RecordFailure()
	cb.RecordFailure()
	cb.RecordFailure()

	if cb.failures != 3 {
		t.Errorf("expected 3 failures, got %d", cb.failures)
	}

	cb.RecordSuccess()

	if cb.failures != 0 {
		t.Errorf("expected 0 failures after success, got %d", cb.failures)
	}
}

func TestCircuitBreaker_BlocksAfterThreeFailures(t *testing.T) {
	cb := NewCircuitBreaker()

	for i := 0; i < 3; i++ {
		cb.RecordFailure()
	}

	if cb.Allow() {
		t.Error("should not allow after 3 failures")
	}
}

func TestCircuitBreaker_ResetsAfterOneMinute(t *testing.T) {
	cb := NewCircuitBreaker()

	cb.failures = 3
	cb.lastFail = time.Now().Add(-2 * time.Minute)

	if !cb.Allow() {
		t.Error("should allow after cooldown period")
	}

	if cb.failures != 0 {
		t.Errorf("expected failures to reset to 0, got %d", cb.failures)
	}
}

func TestCircuitBreaker_Stat(t *testing.T) {
	cb := NewCircuitBreaker()
	tmpDir := t.TempDir()
	tmpFile := tmpDir + "/test.txt"
	if err := os.WriteFile(tmpFile, []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}

	info, err := cb.Stat(tmpFile)
	if err != nil {
		t.Errorf("Stat failed: %v", err)
	}
	if info.Name() != "test.txt" {
		t.Errorf("expected test.txt, got %s", info.Name())
	}

	_, err = cb.Stat(tmpDir + "/nonexistent")
	if !os.IsNotExist(err) {
		t.Errorf("expected IsNotExist error for nonexistent file, got %v", err)
	}
}

// A nil breaker behaves as a no-op breaker that performs the op directly. This
// is what hand-built test Contexts rely on.
func TestCircuitBreaker_NilReceiver(t *testing.T) {
	var cb *CircuitBreaker
	tmpDir := t.TempDir()
	tmpFile := tmpDir + "/test.txt"
	if err := os.WriteFile(tmpFile, []byte("hello world"), 0o644); err != nil {
		t.Fatal(err)
	}

	if !cb.Allow() {
		t.Error("nil breaker should always allow")
	}
	data, err := cb.ReadFile(tmpFile)
	if err != nil {
		t.Errorf("nil breaker ReadFile failed: %v", err)
	}
	if string(data) != "hello world" {
		t.Errorf("expected 'hello world', got %s", string(data))
	}
	// Record* on a nil breaker must not panic.
	cb.RecordFailure()
	cb.RecordSuccess()
}

func TestCircuitBreaker_ReadFile(t *testing.T) {
	cb := NewCircuitBreaker()
	tmpDir := t.TempDir()
	tmpFile := tmpDir + "/test.txt"
	if err := os.WriteFile(tmpFile, []byte("hello world"), 0o644); err != nil {
		t.Fatal(err)
	}

	data, err := cb.ReadFile(tmpFile)
	if err != nil {
		t.Errorf("ReadFile failed: %v", err)
	}
	if string(data) != "hello world" {
		t.Errorf("expected 'hello world', got %s", string(data))
	}
}

// A missing path is the normal thing the dependency rules look for; it must not
// count as an infrastructure failure or the breaker would trip on every run with
// a legitimately absent dependency directory.
func TestCircuitBreaker_NotExistDoesNotTrip(t *testing.T) {
	cb := NewCircuitBreaker()
	tmpDir := t.TempDir()

	for i := 0; i < cbFailureThreshold+2; i++ {
		if _, err := cb.Stat(tmpDir + "/nonexistent"); !os.IsNotExist(err) {
			t.Fatalf("expected IsNotExist, got %v", err)
		}
	}

	if !cb.Allow() {
		t.Error("breaker should stay closed after repeated not-found results")
	}
	if cb.failures != 0 {
		t.Errorf("expected 0 failures after not-found results, got %d", cb.failures)
	}
}

// An infrastructure failure (here, a timeout) must count toward opening the
// breaker; once open, operations fail fast with ErrCircuitOpen.
func TestCircuitBreaker_InfraFailureTripsAndShortCircuits(t *testing.T) {
	cb := NewCircuitBreaker()
	for i := 0; i < cbFailureThreshold; i++ {
		cb.record(&TimeoutError{Operation: "stat"})
	}

	if cb.Allow() {
		t.Fatal("breaker should be open after threshold infra failures")
	}
	if _, err := cb.Stat("/anything"); !errors.Is(err, ErrCircuitOpen) {
		t.Errorf("expected ErrCircuitOpen while open, got %v", err)
	}
	if _, err := cb.ReadFile("/anything"); !errors.Is(err, ErrCircuitOpen) {
		t.Errorf("expected ErrCircuitOpen while open, got %v", err)
	}
}

// ErrCircuitOpen must not be mistaken for a missing path, or dependency_paths
// would emit a false "does not exist" while the circuit is open.
func TestErrCircuitOpenIsNotNotExist(t *testing.T) {
	if os.IsNotExist(ErrCircuitOpen) {
		t.Error("ErrCircuitOpen must not satisfy os.IsNotExist")
	}
}

func TestIsInfraFailure(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"not-exist", fs.ErrNotExist, false},
		{"timeout", &TimeoutError{Operation: "x"}, true},
		{"permission", fs.ErrPermission, true},
	}
	for _, tc := range cases {
		if got := isInfraFailure(tc.err); got != tc.want {
			t.Errorf("%s: isInfraFailure=%v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestTimeoutError(t *testing.T) {
	err := &TimeoutError{Operation: "test op"}
	if err.Error() != "operation timed out: test op" {
		t.Errorf("unexpected error message: %s", err.Error())
	}
}

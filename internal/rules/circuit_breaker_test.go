package rules

import (
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

func TestSafeStat(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := tmpDir + "/test.txt"
	if err := os.WriteFile(tmpFile, []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}

	info, err := safeStat(tmpFile)
	if err != nil {
		t.Errorf("safeStat failed: %v", err)
	}
	if info.Name() != "test.txt" {
		t.Errorf("expected test.txt, got %s", info.Name())
	}

	_, err = safeStat(tmpDir + "/nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestSafeReadFile(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := tmpDir + "/test.txt"
	content := []byte("hello world")
	if err := os.WriteFile(tmpFile, content, 0o644); err != nil {
		t.Fatal(err)
	}

	data, err := safeReadFile(tmpFile)
	if err != nil {
		t.Errorf("safeReadFile failed: %v", err)
	}
	if string(data) != "hello world" {
		t.Errorf("expected 'hello world', got %s", string(data))
	}

	_, err = safeReadFile(tmpDir + "/nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestTimeoutError(t *testing.T) {
	err := &TimeoutError{Operation: "test op"}
	if err.Error() != "operation timed out: test op" {
		t.Errorf("unexpected error message: %s", err.Error())
	}
}
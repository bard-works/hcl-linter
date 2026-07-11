package engine

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestRunConcurrent_BoundsInFlightWorkers(t *testing.T) {
	paths := make([]string, 200)
	for i := range paths {
		paths[i] = "file"
	}

	var inFlight int32
	var maxSeen int32
	fn := func(_ string) int {
		n := atomic.AddInt32(&inFlight, 1)
		for {
			seen := atomic.LoadInt32(&maxSeen)
			if n <= seen || atomic.CompareAndSwapInt32(&maxSeen, seen, n) {
				break
			}
		}
		time.Sleep(time.Millisecond)
		atomic.AddInt32(&inFlight, -1)
		return 0
	}

	out := runConcurrent(context.Background(), paths, 2, fn)
	if len(out) != len(paths) {
		t.Fatalf("expected %d results, got %d", len(paths), len(out))
	}
	if atomic.LoadInt32(&maxSeen) > 2 {
		t.Errorf("expected at most 2 concurrent workers, saw %d", maxSeen)
	}
}

func TestRunConcurrent_CancelStopsNewDispatchButFinishesInFlight(t *testing.T) {
	paths := make([]string, 50)
	for i := range paths {
		paths[i] = "file"
	}

	ctx, cancel := context.WithCancel(context.Background())
	var started int32

	fn := func(_ string) int {
		n := atomic.AddInt32(&started, 1)
		if n == 2 {
			cancel()
		}
		time.Sleep(5 * time.Millisecond)
		return 1
	}

	out := runConcurrent(ctx, paths, 2, fn)

	if len(out) == 0 {
		t.Fatal("expected at least the in-flight results to be returned")
	}
	if len(out) == len(paths) {
		t.Error("expected cancellation to stop dispatch before all paths were processed")
	}
	for _, r := range out {
		if r != 1 {
			t.Errorf("expected every returned result to come from a completed call, got %d", r)
		}
	}
}

package hashlimit

import (
	"context"
	"errors"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestConcurrencyNeverExceedsTheCap(t *testing.T) {
	limiter := New(3, 5*time.Second)
	var running, peak atomic.Int32
	var wg sync.WaitGroup
	for range 40 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			release, err := limiter.Acquire(context.Background())
			if err != nil {
				t.Errorf("Acquire: %v", err)
				return
			}
			now := running.Add(1)
			for {
				seen := peak.Load()
				if now <= seen || peak.CompareAndSwap(seen, now) {
					break
				}
			}
			time.Sleep(2 * time.Millisecond)
			running.Add(-1)
			release()
		}()
	}
	wg.Wait()
	if got := peak.Load(); got > 3 || got < 1 {
		t.Errorf("peak concurrency = %d, want 1 to 3", got)
	}
}

func TestQueuedCallerTimesOut(t *testing.T) {
	limiter := New(1, 30*time.Millisecond)
	release, err := limiter.Acquire(context.Background())
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	defer release()
	start := time.Now()
	_, err = limiter.Acquire(context.Background())
	if !errors.Is(err, ErrBusy) {
		t.Fatalf("second Acquire error = %v, want ErrBusy", err)
	}
	if time.Since(start) < 25*time.Millisecond {
		t.Errorf("Acquire returned after %v, before the wait elapsed", time.Since(start))
	}
}

func TestCancelledContextStopsWaiting(t *testing.T) {
	limiter := New(1, time.Minute)
	release, err := limiter.Acquire(context.Background())
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	defer release()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := limiter.Acquire(ctx)
		done <- err
	}()
	time.Sleep(10 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, ErrBusy) || !errors.Is(err, context.Canceled) {
			t.Errorf("error = %v, want ErrBusy wrapping context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Acquire did not return after the context ended")
	}
}

func TestReleaseFreesASlotAndLeaksNoGoroutines(t *testing.T) {
	before := runtime.NumGoroutine()
	limiter := New(1, 20*time.Millisecond)
	release, err := limiter.Acquire(context.Background())
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := limiter.Acquire(context.Background()); !errors.Is(err, ErrBusy) {
				t.Errorf("error = %v, want ErrBusy", err)
			}
		}()
	}
	wg.Wait()
	release()
	again, err := limiter.Acquire(context.Background())
	if err != nil {
		t.Fatalf("Acquire after release: %v", err)
	}
	again()
	time.Sleep(50 * time.Millisecond)
	if after := runtime.NumGoroutine(); after > before+1 {
		t.Errorf("goroutines grew from %d to %d", before, after)
	}
}

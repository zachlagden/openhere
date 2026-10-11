package auth

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/zachlagden/openhere/internal/store"
	"github.com/zachlagden/openhere/internal/store/storetest"
)

func TestRunMaintenanceRunsExtraTasks(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		svc := newTestService(t, db, newFakeClock())
		var ran, failed atomic.Int32
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan struct{})
		go func() {
			defer close(done)
			svc.RunMaintenance(ctx, 5*time.Millisecond,
				MaintenanceTask{Name: "counting", Run: func(context.Context) error { ran.Add(1); return nil }},
				MaintenanceTask{Name: "failing", Run: func(context.Context) error { failed.Add(1); return errors.New("boom") }},
			)
		}()
		deadline := time.After(5 * time.Second)
		for ran.Load() < 2 || failed.Load() < 2 {
			select {
			case <-deadline:
				t.Fatalf("tasks ran %d and %d times", ran.Load(), failed.Load())
			case <-time.After(5 * time.Millisecond):
			}
		}
		cancel()
		<-done
	})
}

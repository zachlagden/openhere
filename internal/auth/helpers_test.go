package auth

import (
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/zachlagden/openhere/internal/store"
)

var testParams = Params{MemoryKiB: 8, Time: 1, Threads: 1, KeyLen: 16, SaltLen: 8}

const testPassword = "correct horse battery"

type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func newFakeClock() *fakeClock {
	return &fakeClock{now: time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)}
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

func newTestService(t *testing.T, db store.Store, clock *fakeClock) *Service {
	t.Helper()
	svc, err := New(db, Options{
		KeyPrefix: "sk_",
		Params:    testParams,
		Now:       clock.Now,
		Logger:    slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return svc
}

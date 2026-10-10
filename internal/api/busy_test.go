package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/zachlagden/selfhost/internal/hashlimit"
	"github.com/zachlagden/selfhost/internal/store"
	"github.com/zachlagden/selfhost/internal/store/storetest"
)

func saturateHashing(t *testing.T) {
	t.Helper()
	previous := hashlimit.Default()
	limiter := hashlimit.New(1, 20*time.Millisecond)
	hashlimit.SetDefault(limiter)
	release, err := limiter.Acquire(context.Background())
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	t.Cleanup(func() {
		release()
		hashlimit.SetDefault(previous)
	})
}

func TestLoginReportsServerBusyWhenHashingIsSaturated(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		h.completeSetup()
		saturateHashing(t)

		known := h.do("POST", "/api/v1/auth/login", loginBody())
		body := expectError(t, known, http.StatusServiceUnavailable, codeServerBusy)
		if body.RetryAfter == nil || *body.RetryAfter < 1 || known.Header().Get("Retry-After") == "" {
			t.Errorf("busy response lacks retry information: %+v, header %q", body, known.Header().Get("Retry-After"))
		}
		unknown := h.do("POST", "/api/v1/auth/login", loginRequest{Email: "nobody@example.com", Password: testPassword})
		expectError(t, unknown, http.StatusServiceUnavailable, codeServerBusy)
	})
}

func TestConcurrentAnonymousLoginsAreBoundedAndNeverFail(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		h.completeSetup()
		previous := hashlimit.Default()
		hashlimit.SetDefault(hashlimit.New(2, 5*time.Second))
		t.Cleanup(func() { hashlimit.SetDefault(previous) })

		var wg sync.WaitGroup
		results := make([]*httptest.ResponseRecorder, 30)
		for i := range results {
			wg.Add(1)
			go func() {
				defer wg.Done()
				results[i] = h.do("POST", "/api/v1/auth/login", loginRequest{Email: fmt.Sprintf("user%d@example.com", i), Password: testPassword})
			}()
		}
		wg.Wait()
		for i, rec := range results {
			if rec.Code != http.StatusUnauthorized {
				t.Errorf("login %d status = %d, want 401", i, rec.Code)
			}
		}
	})
}

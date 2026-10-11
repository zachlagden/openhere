package hosting

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/zachlagden/openhere/internal/settings"
	"github.com/zachlagden/openhere/internal/store"
	"github.com/zachlagden/openhere/internal/store/storetest"
)

type snapshotAfterWait struct {
	wait     time.Duration
	failures int
	enabled  bool
	reason   string
}

func TestRunRetriesEveryFifteenMinutesUntilACheckSucceeds(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.enableSubdomains()
		const interval = 6 * time.Hour
		attempts := 0
		client := doerFunc(func(req *http.Request) (*http.Response, error) {
			attempts++
			if attempts <= scheduledAttempts*3 {
				return reply(502, ""), nil
			}
			rec := httptest.NewRecorder()
			forwarded := req.Clone(req.Context())
			forwarded.Host = req.URL.Host
			e.router.ServeHTTP(rec, forwarded)
			return rec.Result(), nil
		})
		var waits []snapshotAfterWait
		sleep := func(ctx context.Context, d time.Duration) error {
			if d == attemptGap {
				return nil
			}
			cfg, err := e.settings.Get(ctx)
			if err != nil {
				t.Error(err)
			}
			waits = append(waits, snapshotAfterWait{d, cfg.WildcardFailures, cfg.SubdomainEnabled, cfg.SubdomainDisabledReason})
			if d == interval {
				return errors.New("stop")
			}
			return nil
		}
		checker := NewChecker(e.settings, e.probes, CheckerOptions{
			Public: httpsPublic, Resolver: &fakeResolver{addrs: []string{"192.0.2.10"}}, Client: client,
			Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Sleep: sleep,
		})

		checker.Run(t.Context(), interval)

		want := []snapshotAfterWait{
			{startupDelay, 0, true, ""},
			{RetryInterval, 1, true, ""},
			{RetryInterval, 2, true, ""},
			{RetryInterval, 3, false, settings.DisabledByWildcard},
			{interval, 0, false, ""},
		}
		if len(waits) != len(want) {
			t.Fatalf("waits = %+v", waits)
		}
		for i := range want {
			if waits[i] != want[i] {
				t.Errorf("wait %d = %+v, want %+v", i, waits[i], want[i])
			}
		}
		final, err := e.settings.Get(t.Context())
		if err != nil || final.WildcardStatus != settings.WildcardOK || final.SubdomainEnabled || !final.PathEnabled {
			t.Errorf("final settings = %+v, %v", final, err)
		}
	})
}

func TestRunKeepsTheNormalIntervalWhileSubdomainModeIsOff(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		const interval = 6 * time.Hour
		var waits []time.Duration
		sleep := func(_ context.Context, d time.Duration) error {
			if d == attemptGap {
				return nil
			}
			waits = append(waits, d)
			if len(waits) == 3 {
				return errors.New("stop")
			}
			return nil
		}
		client := doerFunc(func(*http.Request) (*http.Response, error) { return reply(502, ""), nil })
		checker := NewChecker(e.settings, e.probes, CheckerOptions{
			Public: httpsPublic, Resolver: &fakeResolver{addrs: []string{"192.0.2.10"}}, Client: client,
			Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Sleep: sleep,
		})
		checker.Run(t.Context(), interval)
		if len(waits) != 3 || waits[0] != startupDelay || waits[1] != interval || waits[2] != interval {
			t.Errorf("waits = %v", waits)
		}
	})
}

func TestManualChecksDoNotAdvanceTheFailureCounter(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.enableSubdomains()
		client := doerFunc(func(*http.Request) (*http.Response, error) { return reply(502, ""), nil })
		checker := newChecker(e, &fakeResolver{addrs: []string{"192.0.2.10"}}, client, httpsPublic)
		for range 5 {
			if _, err := checker.Check(t.Context()); err != nil {
				t.Fatal(err)
			}
		}
		got, err := e.settings.Get(t.Context())
		if err != nil || got.WildcardFailures != 0 || !got.SubdomainEnabled || got.WildcardStatus != settings.WildcardFailed {
			t.Errorf("after five manual failures = %+v, %v", got, err)
		}
	})
}

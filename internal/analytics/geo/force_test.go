package geo

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/zachlagden/openhere/internal/settings"
)

func (e *updaterEnv) force() error {
	e.t.Helper()
	return e.updater.ForceCheck(context.Background())
}

func TestForceCheckRetriesAFailureWithUnchangedCredentials(t *testing.T) {
	e := newUpdaterEnv(t, buildArchive(t, buildDatabase(t, "London")))
	e.mm.setStatus(401)
	e.check()
	if last := e.state.last(); last.Status != settings.GeoFailed || last.Detail != rejectedKeyDetail {
		t.Fatalf("recorded state = %+v", last)
	}
	e.mm.setStatus(0)
	failed := e.mm.requestCount()
	e.check()
	if e.mm.requestCount() != failed {
		t.Fatal("an unforced check retried inside the back-off")
	}
	if err := e.force(); err != nil {
		t.Fatalf("ForceCheck: %v", err)
	}
	if last := e.state.last(); last.Status != settings.GeoOK || !last.Updated {
		t.Errorf("recorded state = %+v", last)
	}
	e.assertNoSecretLogged()
}

func TestForceCheckAllowsOneAttemptPerMinute(t *testing.T) {
	e := newUpdaterEnv(t, nil)
	e.mm.setStatus(500)
	if err := e.force(); err != nil {
		t.Fatal(err)
	}
	requests := e.mm.requestCount()
	e.now = e.now.Add(20 * time.Second)
	var soon *TooSoonError
	if err := e.force(); !errors.As(err, &soon) {
		t.Fatalf("err = %v, want TooSoonError", err)
	}
	if soon.RetryAfter != 40*time.Second {
		t.Errorf("RetryAfter = %v, want 40s", soon.RetryAfter)
	}
	if e.mm.requestCount() != requests {
		t.Error("a refused check contacted MaxMind")
	}
	e.now = e.now.Add(40 * time.Second)
	if err := e.force(); err != nil {
		t.Fatalf("ForceCheck after the floor: %v", err)
	}
	if e.mm.requestCount() == requests {
		t.Error("the check after the floor did not contact MaxMind")
	}
}

func TestForceCheckKeepsTheWeeklyRefresh(t *testing.T) {
	e := newUpdaterEnv(t, buildArchive(t, buildDatabase(t, "London")))
	e.check()
	first := e.mm.requestCount()
	e.now = e.now.Add(2 * time.Minute)
	if err := e.force(); err != nil {
		t.Fatal(err)
	}
	if e.mm.requestCount() != first {
		t.Error("a forced check downloaded while the file was fresh")
	}
	e.now = e.now.Add(2 * time.Minute)
	e.state.cfg.Analytics.GeoUpdatedAt = e.now.Add(-8 * 24 * time.Hour).Format(time.RFC3339)
	if err := e.force(); err != nil {
		t.Fatal(err)
	}
	if e.mm.requestCount() == first {
		t.Error("a forced check did not refresh a stale file")
	}
}

func TestForceCheckDoesNotStartASecondDownload(t *testing.T) {
	e := newUpdaterEnv(t, buildArchive(t, buildDatabase(t, "London")))
	e.mm.block = make(chan struct{})
	e.mm.entered = make(chan struct{}, 8)
	running := make(chan error, 1)
	go func() { running <- e.updater.Check(context.Background()) }()
	<-e.mm.entered
	e.now = e.now.Add(time.Minute)
	forced := make(chan error, 1)
	go func() { forced <- e.updater.ForceCheck(context.Background()) }()
	time.Sleep(100 * time.Millisecond)
	if len(e.mm.entered) != 0 {
		t.Fatal("a second request reached MaxMind while a check was running")
	}
	select {
	case <-forced:
		t.Fatal("ForceCheck returned while a check was running")
	default:
	}
	close(e.mm.block)
	if err := <-running; err != nil {
		t.Fatal(err)
	}
	if err := <-forced; err != nil {
		t.Fatal(err)
	}
	if got := e.mm.requestCount(); got != 2 {
		t.Errorf("requests = %d, want 2 (archive and checksum, once)", got)
	}
}

func TestRestartRetriesAnOldFailureButNotAFreshOne(t *testing.T) {
	e := newUpdaterEnv(t, buildArchive(t, buildDatabase(t, "London")))
	e.state.cfg.Analytics.GeoStatus = settings.GeoFailed
	e.state.cfg.Analytics.GeoCheckedAt = e.now.Add(-2 * time.Minute).Format(time.RFC3339)
	e.check()
	if e.mm.requestCount() != 0 {
		t.Fatal("a failure from two minutes ago was retried on startup")
	}
	e.state.cfg.Analytics.GeoCheckedAt = e.now.Add(-6 * time.Minute).Format(time.RFC3339)
	e.check()
	if last := e.state.last(); last.Status != settings.GeoOK {
		t.Errorf("recorded state = %+v", last)
	}
}

func TestRunChecksAtOnceWhenNudged(t *testing.T) {
	e := newUpdaterEnv(t, buildArchive(t, buildDatabase(t, "London")))
	e.mm.setStatus(401)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		e.updater.Run(ctx, time.Hour)
		close(done)
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})
	waitForStatus(t, e, settings.GeoFailed)
	e.mm.setStatus(0)
	e.updater.Nudge()
	e.updater.Nudge()
	waitForStatus(t, e, settings.GeoOK)
}

func waitForStatus(t *testing.T, e *updaterEnv, status string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if e.state.last().Status == status {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("status never became %s; last = %+v", status, e.state.last())
}

func TestNudgeNeverBlocks(t *testing.T) {
	e := newUpdaterEnv(t, nil)
	for range 10 {
		e.updater.Nudge()
	}
}

func TestRecordingSurvivesACancelledRequest(t *testing.T) {
	e := newUpdaterEnv(t, nil)
	e.state.cfg.Analytics.MaxMindAccountID = ""
	e.state.cfg.Analytics.MaxMindLicenseKey = ""
	e.state.cfg.Analytics.GeoStatus = settings.GeoFailed
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := e.updater.ForceCheck(ctx); err != nil {
		t.Fatalf("ForceCheck: %v", err)
	}
	if last := e.state.last(); last.Status != settings.GeoUnconfigured {
		t.Errorf("recorded state = %+v", last)
	}
}

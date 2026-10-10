package domains

import (
	"context"
	"testing"
	"time"
)

const day = 24 * time.Hour

func (e *env) recheck() {
	e.t.Helper()
	if err := e.svc.Recheck(context.Background()); err != nil {
		e.t.Fatalf("Recheck: %v", err)
	}
}

func (e *env) dayPasses() {
	e.clock.Advance(day + time.Minute)
	e.recheck()
}

func TestReleaseStaleClaims(t *testing.T) {
	each(t, func(t *testing.T, e *env) {
		siteID := e.site("launch")
		old := e.claim(siteID, "old.example.org")
		e.clock.Advance(StaleClaimAge - time.Hour)
		fresh := e.claim(siteID, "fresh.example.org")
		kept := e.verified(siteID, "kept.example.org")
		e.clock.Advance(2 * time.Hour)
		released, err := e.svc.ReleaseStale(context.Background())
		if err != nil || released != 1 {
			t.Fatalf("ReleaseStale = %d, %v, want 1", released, err)
		}
		if _, err := e.svc.Get(context.Background(), siteID, old.ID); !isErr(err, ErrNotFound) {
			t.Errorf("stale claim still exists: %v", err)
		}
		e.get(siteID, fresh.ID)
		e.get(siteID, kept.ID)
		other := e.site("guides")
		e.claim(other, "old.example.org")
	})
}

func TestVerifiedDomainsAreNeverReleasedByAge(t *testing.T) {
	each(t, func(t *testing.T, e *env) {
		siteID := e.site("launch")
		d := e.verified(siteID, "www.example.org")
		e.clock.Advance(365 * day)
		if released, err := e.svc.ReleaseStale(context.Background()); err != nil || released != 0 {
			t.Fatalf("ReleaseStale = %d, %v", released, err)
		}
		e.get(siteID, d.ID)
	})
}

func TestRecheckWaitsADayBetweenChecks(t *testing.T) {
	each(t, func(t *testing.T, e *env) {
		siteID := e.site("launch")
		e.verified(siteID, "www.example.org")
		before := e.dns.queryCount()
		e.recheck()
		e.clock.Advance(23 * time.Hour)
		e.recheck()
		if e.dns.queryCount() != before {
			t.Errorf("rechecked before a day passed: %d new lookups", e.dns.queryCount()-before)
		}
		e.clock.Advance(2 * time.Hour)
		e.recheck()
		if e.dns.queryCount() != before+1 {
			t.Errorf("lookups after a day = %d, want %d", e.dns.queryCount(), before+1)
		}
	})
}

func TestThreeMissingChecksDropADomainBackToPending(t *testing.T) {
	each(t, func(t *testing.T, e *env) {
		siteID := e.site("launch")
		d := e.verified(siteID, "www.example.org")
		if _, err := e.svc.SetPrimary(context.Background(), siteID, d.ID, true); err != nil {
			t.Fatal(err)
		}
		e.dns.clear(d.ChallengeName())
		for miss := 1; miss < MissingChecksToDrop; miss++ {
			e.dayPasses()
			got := e.get(siteID, d.ID)
			if !got.Verified() || got.MissingChecks != miss || got.LastCheckResult != ResultNotFound {
				t.Fatalf("after %d misses: %+v", miss, got)
			}
			if !e.serves("www.example.org") {
				t.Fatalf("stopped serving after %d misses", miss)
			}
		}
		e.dayPasses()
		got := e.get(siteID, d.ID)
		if got.Verified() || got.IsPrimary || got.MissingChecks != 0 || got.Status() != StatusPending {
			t.Errorf("after %d misses: %+v", MissingChecksToDrop, got)
		}
		if e.serves("www.example.org") {
			t.Error("a dropped domain still serves")
		}
		if released, err := e.svc.ReleaseStale(context.Background()); err != nil || released != 0 {
			t.Errorf("a freshly dropped domain was released at once: %d, %v", released, err)
		}
		e.publishChallenge(got)
		again, err := e.svc.Verify(context.Background(), siteID, d.ID)
		if err != nil || !again.Verified() {
			t.Errorf("re-verifying a dropped domain = %+v, %v", again, err)
		}
	})
}

func TestTokenMismatchCountsAsMissing(t *testing.T) {
	each(t, func(t *testing.T, e *env) {
		siteID := e.site("launch")
		d := e.verified(siteID, "www.example.org")
		e.dns.set(d.ChallengeName(), []string{"someone-elses-token"}, nil)
		for range MissingChecksToDrop {
			e.dayPasses()
		}
		got := e.get(siteID, d.ID)
		if got.Verified() || got.LastCheckResult != ResultMismatch {
			t.Errorf("domain = %+v", got)
		}
	})
}

func TestTransientFailuresNeverCountAsMissing(t *testing.T) {
	each(t, func(t *testing.T, e *env) {
		siteID := e.site("launch")
		d := e.verified(siteID, "www.example.org")
		for _, failure := range []error{transient(), timeout()} {
			e.dns.set(d.ChallengeName(), nil, failure)
			for range MissingChecksToDrop + 2 {
				e.dayPasses()
			}
		}
		got := e.get(siteID, d.ID)
		if !got.Verified() || got.MissingChecks != 0 || got.LastCheckResult != ResultLookupFailed {
			t.Errorf("domain = %+v", got)
		}
		if !e.serves("www.example.org") {
			t.Error("a domain stopped serving because of resolver failures")
		}
	})
}

func TestTransientFailuresDoNotResetTheMissCount(t *testing.T) {
	each(t, func(t *testing.T, e *env) {
		siteID := e.site("launch")
		d := e.verified(siteID, "www.example.org")
		e.dns.clear(d.ChallengeName())
		e.dayPasses()
		e.dayPasses()
		e.dns.set(d.ChallengeName(), nil, timeout())
		e.dayPasses()
		if got := e.get(siteID, d.ID); got.MissingChecks != 2 || !got.Verified() {
			t.Fatalf("after a transient failure: %+v", got)
		}
		e.dns.clear(d.ChallengeName())
		e.dayPasses()
		if got := e.get(siteID, d.ID); got.Verified() {
			t.Errorf("third miss did not drop the domain: %+v", got)
		}
	})
}

func TestASuccessfulCheckResetsTheMissCount(t *testing.T) {
	each(t, func(t *testing.T, e *env) {
		siteID := e.site("launch")
		d := e.verified(siteID, "www.example.org")
		e.dns.clear(d.ChallengeName())
		e.dayPasses()
		e.dayPasses()
		e.publishChallenge(d)
		e.dayPasses()
		if got := e.get(siteID, d.ID); got.MissingChecks != 0 || got.LastCheckResult != ResultVerified {
			t.Fatalf("after recovery: %+v", got)
		}
		e.dns.clear(d.ChallengeName())
		e.dayPasses()
		e.dayPasses()
		if got := e.get(siteID, d.ID); !got.Verified() {
			t.Errorf("two misses after recovery dropped the domain: %+v", got)
		}
	})
}

func TestRecheckIgnoresPendingDomains(t *testing.T) {
	each(t, func(t *testing.T, e *env) {
		siteID := e.site("launch")
		e.claim(siteID, "www.example.org")
		e.dayPasses()
		if e.dns.queryCount() != 0 {
			t.Errorf("rechecked an unverified domain: %d lookups", e.dns.queryCount())
		}
	})
}

func TestMaintainReleasesAndRechecks(t *testing.T) {
	each(t, func(t *testing.T, e *env) {
		siteID := e.site("launch")
		stale := e.claim(siteID, "stale.example.org")
		live := e.verified(siteID, "live.example.org")
		e.clock.Advance(StaleClaimAge + day)
		e.publishChallenge(live)
		if err := e.svc.Maintain(context.Background()); err != nil {
			t.Fatal(err)
		}
		if _, err := e.svc.Get(context.Background(), siteID, stale.ID); !isErr(err, ErrNotFound) {
			t.Errorf("stale claim survived Maintain: %v", err)
		}
		if got := e.get(siteID, live.ID); !got.Verified() || got.LastCheckedAt == nil || !got.LastCheckedAt.After(*live.LastCheckedAt) {
			t.Errorf("verified domain was not rechecked: %+v", got)
		}
	})
}

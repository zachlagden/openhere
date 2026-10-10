package domains

import (
	"context"
	"strconv"
	"testing"
)

func TestLookupServesOnlyVerifiedDomains(t *testing.T) {
	each(t, func(t *testing.T, e *env) {
		siteID := e.site("launch")
		pending := e.claim(siteID, "pending.example.org")
		e.verified(siteID, "live.example.org")
		if e.serves(pending.Name) {
			t.Error("an unverified domain serves")
		}
		if !e.serves("live.example.org") {
			t.Error("a verified domain does not serve")
		}
		for _, host := range []string{"unknown.example.org", "", "localhost", "a", "x.y", "live.example.org.evil.example", "Live.Example.org"} {
			if e.serves(host) {
				t.Errorf("host %q serves", host)
			}
		}
	})
}

func TestLookupReturnsTheSiteTarget(t *testing.T) {
	each(t, func(t *testing.T, e *env) {
		siteID := e.site("launch")
		e.verified(siteID, "live.example.org")
		target, ok, err := e.svc.Lookup(context.Background(), "live.example.org")
		if err != nil || !ok || target != (Target{SiteID: siteID, Slug: "launch", Domain: "live.example.org"}) {
			t.Errorf("Lookup = %+v, %v, %v", target, ok, err)
		}
	})
}

func TestLookupCachesAndInvalidatesOnEveryChange(t *testing.T) {
	each(t, func(t *testing.T, e *env) {
		siteID := e.site("launch")
		d := e.claim(siteID, "www.example.org")
		if e.serves(d.Name) {
			t.Fatal("serves before verification")
		}
		e.publishChallenge(d)
		if _, err := e.svc.Verify(context.Background(), siteID, d.ID); err != nil {
			t.Fatal(err)
		}
		if !e.serves(d.Name) {
			t.Error("a negative cache entry hid a newly verified domain")
		}
		if err := e.svc.Delete(context.Background(), siteID, d.ID); err != nil {
			t.Fatal(err)
		}
		if e.serves(d.Name) {
			t.Error("a positive cache entry outlived Delete")
		}
	})
}

func TestLookupExpiresEntries(t *testing.T) {
	each(t, func(t *testing.T, e *env) {
		siteID := e.site("launch")
		e.verified(siteID, "www.example.org")
		if !e.serves("www.example.org") {
			t.Fatal("not serving")
		}
		if _, err := e.db.ExecContext(context.Background(), "DELETE FROM site_domains"); err != nil {
			t.Fatal(err)
		}
		if !e.serves("www.example.org") {
			t.Error("the cache was not used")
		}
		e.clock.Advance(foundTTL)
		if e.serves("www.example.org") {
			t.Error("an expired entry was served")
		}
	})
}

func TestLookupCacheStaysBounded(t *testing.T) {
	each(t, func(t *testing.T, e *env) {
		for i := range maxCachedKeys + 500 {
			e.serves("host-" + strconv.Itoa(i) + ".example.org")
		}
		e.svc.cache.mu.Lock()
		size := len(e.svc.cache.entries)
		e.svc.cache.mu.Unlock()
		if size > maxCachedKeys {
			t.Errorf("cache holds %d entries, over %d", size, maxCachedKeys)
		}
	})
}

func TestLookupSkipsTheDatabaseForImplausibleHosts(t *testing.T) {
	each(t, func(t *testing.T, e *env) {
		for _, host := range []string{"", "localhost", "a.b", repeatString("a", 300) + ".com"} {
			if _, ok, err := e.svc.Lookup(context.Background(), host); ok || err != nil {
				t.Errorf("Lookup(%q) = %v, %v", host, ok, err)
			}
		}
		e.svc.cache.mu.Lock()
		size := len(e.svc.cache.entries)
		e.svc.cache.mu.Unlock()
		if size != 0 {
			t.Errorf("implausible hosts were cached: %d", size)
		}
	})
}

func TestSummaries(t *testing.T) {
	each(t, func(t *testing.T, e *env) {
		first, second, empty := e.site("launch"), e.site("guides"), e.site("blank")
		a := e.verified(first, "a.example.org")
		e.claim(first, "b.example.org")
		e.verified(second, "c.example.org")
		if _, err := e.svc.SetPrimary(context.Background(), first, a.ID, true); err != nil {
			t.Fatal(err)
		}
		got, err := e.svc.Summaries(context.Background(), []string{first, second, empty})
		if err != nil {
			t.Fatal(err)
		}
		if got[first] != (Summary{Total: 2, Verified: 1, Primary: "a.example.org", Fallback: "a.example.org"}) {
			t.Errorf("first = %+v", got[first])
		}
		if got[second] != (Summary{Total: 1, Verified: 1, Fallback: "c.example.org"}) {
			t.Errorf("second = %+v", got[second])
		}
		if _, present := got[empty]; present {
			t.Errorf("empty site has a summary: %+v", got[empty])
		}
		if none, err := e.svc.Summaries(context.Background(), nil); err != nil || len(none) != 0 {
			t.Errorf("Summaries(nil) = %v, %v", none, err)
		}
	})
}

func TestServesSiteMatchesOnlyTheOwningSiteAndHonoursTheSwitch(t *testing.T) {
	each(t, func(t *testing.T, e *env) {
		owner, other := e.site("launch"), e.site("guides")
		e.verified(owner, "live.example.org")
		pending := e.claim(owner, "pending.example.org")
		check := func(siteID, host string, want bool) {
			t.Helper()
			got, err := e.svc.ServesSite(context.Background(), siteID, host)
			if err != nil || got != want {
				t.Errorf("ServesSite(%q) = %v, %v, want %v", host, got, err, want)
			}
		}
		check(owner, "live.example.org", true)
		check(other, "live.example.org", false)
		check(owner, pending.Name, false)
		check(owner, "unknown.example.org", false)
		e.set(`{"hosting.customDomainsEnabled": false}`)
		check(owner, "live.example.org", false)
	})
}

package domains

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/zachlagden/openhere/internal/settings"
	"github.com/zachlagden/openhere/internal/validate"
)

func TestClaimReturnsAPendingDomainWithAChallenge(t *testing.T) {
	each(t, func(t *testing.T, e *env) {
		siteID := e.site("launch")
		d := e.claim(siteID, "  WWW.Example.org. ")
		if d.Name != "www.example.org" || d.Verified() || d.Status() != StatusPending || d.IsPrimary {
			t.Errorf("domain = %+v", d)
		}
		if len(d.Token) < 32 {
			t.Errorf("token has %d characters, want at least 32 (128 bits of hex)", len(d.Token))
		}
		if d.ChallengeName() != "_openhere-challenge.www.example.org" {
			t.Errorf("challenge name = %s", d.ChallengeName())
		}
		other := e.claim(siteID, "www.example.net")
		if other.Token == d.Token {
			t.Error("two domains share a challenge token")
		}
	})
}

func TestClaimRejectsNamesTheInstanceOwns(t *testing.T) {
	each(t, func(t *testing.T, e *env) {
		siteID := e.site("launch")
		e.set(`{"hosting.baseDomain": "apps.example.org"}`)
		for _, name := range []string{
			managementHost, "api." + managementHost, "apps.example.org", "demo.apps.example.org", "deep.demo.apps.example.org",
			"localhost", "10.0.0.1", "*.example.org", "printer.local", "example.org:8080",
		} {
			_, err := e.svc.Claim(context.Background(), siteID, name)
			var invalid *validate.Error
			if !asValidation(err, &invalid) {
				t.Errorf("Claim(%q) error = %v, want a validation error", name, err)
			}
		}
		if _, err := e.svc.Claim(context.Background(), siteID, "notapps.example.org"); err != nil {
			t.Errorf("a sibling of the base domain was refused: %v", err)
		}
		if _, err := e.svc.Claim(context.Background(), siteID, "example.org"); err != nil {
			t.Errorf("the parent of the base domain was refused: %v", err)
		}
	})
}

func asValidation(err error, target **validate.Error) bool {
	if err == nil {
		return false
	}
	var v *validate.Error
	ok := errors.As(err, &v)
	*target = v
	return ok
}

func TestVerifiedDomainsAreExclusiveAcrossSites(t *testing.T) {
	each(t, func(t *testing.T, e *env) {
		first, second := e.site("launch"), e.site("guides")
		e.verified(first, "www.example.org")
		for _, name := range []string{"www.example.org", "WWW.EXAMPLE.ORG.", " www.example.org "} {
			if _, err := e.svc.Claim(context.Background(), second, name); !isErr(err, ErrTaken) {
				t.Errorf("Claim(%q) by another site = %v, want ErrTaken", name, err)
			}
			if _, err := e.svc.Claim(context.Background(), first, name); !isErr(err, ErrTaken) {
				t.Errorf("Claim(%q) again by the same site = %v, want ErrTaken", name, err)
			}
		}
	})
}

func TestASiteCannotClaimTheSameNameTwice(t *testing.T) {
	each(t, func(t *testing.T, e *env) {
		first := e.site("launch")
		e.claim(first, "www.example.org")
		if _, err := e.svc.Claim(context.Background(), first, "www.example.org"); !isErr(err, ErrTaken) {
			t.Errorf("second claim by the same site = %v, want ErrTaken", err)
		}
	})
}

func TestUnverifiedClaimsDoNotBlockOtherSitesFromClaiming(t *testing.T) {
	each(t, func(t *testing.T, e *env) {
		squatter, owner := e.site("squatter"), e.site("owner")
		e.claim(squatter, "www.example.org")
		if _, err := e.svc.Claim(context.Background(), owner, "www.example.org"); err != nil {
			t.Fatalf("the real owner was blocked by an unverified claim: %v", err)
		}
	})
}

func TestVerifyingDisplacesCompetingUnverifiedClaims(t *testing.T) {
	each(t, func(t *testing.T, e *env) {
		ctx := context.Background()
		squatter, owner := e.site("squatter"), e.site("owner")
		squatted := e.claim(squatter, "www.example.org")
		real := e.claim(owner, "www.example.org")
		e.publishChallenge(real)
		got, err := e.svc.Verify(ctx, owner, real.ID)
		if err != nil || !got.Verified() {
			t.Fatalf("Verify = %+v, %v", got, err)
		}
		if _, err := e.svc.Get(ctx, squatter, squatted.ID); !isErr(err, ErrNotFound) {
			t.Errorf("the displaced claim still exists: %v", err)
		}
		target, ok, err := e.svc.Lookup(ctx, "www.example.org")
		if err != nil || !ok || target.SiteID != owner {
			t.Errorf("Lookup = %+v, %v, %v; want the owner's site", target, ok, err)
		}
		if _, err := e.svc.Claim(ctx, squatter, "www.example.org"); !isErr(err, ErrTaken) {
			t.Errorf("claiming a verified name = %v, want ErrTaken", err)
		}
		if _, err := e.svc.Verify(ctx, squatter, squatted.ID); !isErr(err, ErrNotFound) {
			t.Errorf("verifying a displaced claim = %v, want ErrNotFound", err)
		}
	})
}

func TestAVerifiedDomainIsReleasedOnlyByDeletingIt(t *testing.T) {
	each(t, func(t *testing.T, e *env) {
		ctx := context.Background()
		first, second := e.site("launch"), e.site("guides")
		held := e.verified(first, "www.example.org")
		if _, err := e.svc.Claim(ctx, second, "www.example.org"); !isErr(err, ErrTaken) {
			t.Fatalf("claim of a verified name = %v", err)
		}
		if err := e.svc.Delete(ctx, first, held.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := e.svc.Claim(ctx, second, "www.example.org"); err != nil {
			t.Errorf("claim after the release = %v", err)
		}
	})
}

func TestConcurrentVerificationsOfOneDomainHaveOneWinner(t *testing.T) {
	each(t, func(t *testing.T, e *env) {
		const contenders = 12
		domains := make([]Domain, contenders)
		tokens := make([]string, contenders)
		for i := range domains {
			domains[i] = e.claim(e.site("site-"+string(rune('a'+i))), "contested.example.org")
			tokens[i] = domains[i].Token
		}
		e.dns.set(domains[0].ChallengeName(), tokens, nil)
		var winners atomic.Int32
		var wg sync.WaitGroup
		for _, d := range domains {
			wg.Add(1)
			go func() {
				defer wg.Done()
				got, err := e.svc.Verify(context.Background(), d.SiteID, d.ID)
				switch {
				case err == nil && got.Verified():
					winners.Add(1)
				case isErr(err, ErrTaken), isErr(err, ErrNotFound):
				default:
					t.Errorf("unexpected result: %+v, %v", got, err)
				}
			}()
		}
		wg.Wait()
		var verified, rows int
		if err := e.db.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM site_domains WHERE verified_at IS NOT NULL").Scan(&verified); err != nil {
			t.Fatal(err)
		}
		if err := e.db.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM site_domains WHERE domain = 'contested.example.org'").Scan(&rows); err != nil {
			t.Fatal(err)
		}
		if winners.Load() != 1 || verified != 1 || rows != 1 {
			t.Errorf("winners = %d, verified rows = %d, rows = %d; want 1, 1 and 1", winners.Load(), verified, rows)
		}
	})
}

func TestConcurrentClaimsRespectThePerSiteLimit(t *testing.T) {
	each(t, func(t *testing.T, e *env) {
		const attempts, limit = 16, 3
		siteID := e.site("launch")
		e.set(`{"hosting.maxDomainsPerSite": 3}`)
		var accepted, refused atomic.Int32
		var wg sync.WaitGroup
		for i := range attempts {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := e.svc.Claim(context.Background(), siteID, "host-"+strconv.Itoa(i)+".example.org")
				switch {
				case err == nil:
					accepted.Add(1)
				case isErr(err, ErrLimitReached):
					refused.Add(1)
				default:
					t.Errorf("unexpected error: %v", err)
				}
			}()
		}
		wg.Wait()
		list, err := e.svc.List(context.Background(), siteID)
		if err != nil || len(list) != limit || accepted.Load() != limit || refused.Load() != attempts-limit {
			t.Errorf("stored %d, accepted %d, refused %d (err %v), want %d, %d, %d",
				len(list), accepted.Load(), refused.Load(), err, limit, limit, attempts-limit)
		}
	})
}

func TestClaimEnforcesThePerSiteLimit(t *testing.T) {
	each(t, func(t *testing.T, e *env) {
		siteID, other := e.site("launch"), e.site("guides")
		e.set(`{"hosting.maxDomainsPerSite": 2}`)
		e.claim(siteID, "a.example.org")
		second := e.claim(siteID, "b.example.org")
		if _, err := e.svc.Claim(context.Background(), siteID, "c.example.org"); !isErr(err, ErrLimitReached) {
			t.Errorf("third claim = %v, want ErrLimitReached", err)
		}
		e.claim(other, "c.example.org")
		if err := e.svc.Delete(context.Background(), siteID, second.ID); err != nil {
			t.Fatal(err)
		}
		e.claim(siteID, "d.example.org")
		e.set(`{"hosting.maxDomainsPerSite": 1}`)
		if _, err := e.svc.Claim(context.Background(), siteID, "e.example.org"); !isErr(err, ErrLimitReached) {
			t.Errorf("claim over a lowered limit = %v, want ErrLimitReached", err)
		}
		list, err := e.svc.List(context.Background(), siteID)
		if err != nil || len(list) != 2 {
			t.Errorf("lowering the limit changed stored domains: %d, %v", len(list), err)
		}
	})
}

func TestDeletingASiteRemovesItsDomains(t *testing.T) {
	each(t, func(t *testing.T, e *env) {
		siteID := e.site("launch")
		e.verified(siteID, "www.example.org")
		if err := e.sites.DeleteSite(context.Background(), e.alex, "launch"); err != nil {
			t.Fatal(err)
		}
		e.svc.Invalidate()
		if e.serves("www.example.org") {
			t.Error("a deleted site's domain still serves")
		}
		var count int
		if err := e.db.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM site_domains").Scan(&count); err != nil || count != 0 {
			t.Errorf("rows left = %d, %v", count, err)
		}
		other := e.site("again")
		e.claim(other, "www.example.org")
	})
}

func TestBaseDomainCannotMoveOverACustomDomain(t *testing.T) {
	each(t, func(t *testing.T, e *env) {
		siteID := e.site("launch")
		e.claim(siteID, "www.example.org")
		e.claim(siteID, "example.net")
		for _, base := range []string{"example.org", "www.example.org", "example.net"} {
			e.expectBaseDomainRefused(base)
		}
		e.set(`{"hosting.baseDomain": "other.example.org"}`)
	})
}

func (e *env) expectBaseDomainRefused(base string) {
	e.t.Helper()
	patch := map[string]json.RawMessage{"hosting.baseDomain": json.RawMessage(`"` + base + `"`)}
	_, err := e.settings.Update(context.Background(), patch)
	if !isErr(err, settings.ErrBaseDomainInUse) {
		e.t.Errorf("base domain %s: error = %v, want ErrBaseDomainInUse", base, err)
	}
}

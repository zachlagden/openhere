package certs

import (
	"bytes"
	"context"
	"crypto/x509"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/zachlagden/openhere/internal/domains"
	"github.com/zachlagden/openhere/internal/ratelimit"
	"github.com/zachlagden/openhere/internal/settings"
)

type fakeSettings struct {
	cfg settings.Settings
	err error
}

func (f *fakeSettings) Get(context.Context) (settings.Settings, error) {
	return f.cfg, f.err
}

type fakeLookup struct {
	mu       sync.Mutex
	verified map[string]bool
	sites    map[string]string
	err      error
	calls    int
}

func (f *fakeLookup) Lookup(_ context.Context, host string) (domains.Target, bool, error) {
	f.mu.Lock()
	f.calls++
	f.mu.Unlock()
	if f.err != nil {
		return domains.Target{}, false, f.err
	}
	siteID := f.sites[host]
	if siteID == "" {
		siteID = "site-" + host
	}
	return domains.Target{Domain: host, SiteID: siteID}, f.verified[host], nil
}

type clock struct{ now time.Time }

func (c *clock) Now() time.Time { return c.now }

type policyEnv struct {
	policy   *Policy
	settings *fakeSettings
	lookup   *fakeLookup
	clock    *clock
	logs     *bytes.Buffer
}

func newPolicyEnv(t *testing.T, limits Limits) *policyEnv {
	t.Helper()
	logs := &bytes.Buffer{}
	clk := &clock{now: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	cfg := settings.Defaults()
	cfg.BaseDomain = "sites.example.com"
	cfg.SubdomainEnabled = true
	cfg.CustomDomainsEnabled = true
	env := &policyEnv{
		settings: &fakeSettings{cfg: cfg},
		lookup:   &fakeLookup{verified: map[string]bool{"www.example.org": true, "shop.example.org": true}, sites: map[string]string{}},
		clock:    clk,
		logs:     logs,
	}
	env.policy = NewPolicy(env.settings, env.lookup, PolicyOptions{
		ManagementHost: "Admin.Example.com", Limits: limits, Now: clk.Now,
		Logger: slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug})),
	})
	return env
}

func (e *policyEnv) allow(name string) error {
	return e.policy.Allow(context.Background(), name)
}

func TestVerifiedCustomDomainIsAllowed(t *testing.T) {
	env := newPolicyEnv(t, Limits{})
	for _, name := range []string{"www.example.org", "WWW.Example.org", "www.example.org."} {
		if err := env.allow(name); err != nil {
			t.Errorf("%q refused: %v", name, err)
		}
	}
}

func TestUnknownAndUnverifiedNamesAreRefused(t *testing.T) {
	env := newPolicyEnv(t, Limits{})
	for _, name := range []string{"unknown.example.org", "pending.example.org", "", "*.example.org", "example.org", "www.example.org.evil.test", "1.2.3.4"} {
		if err := env.allow(name); !errors.Is(err, ErrRefused) {
			t.Errorf("%q: error = %v, want ErrRefused", name, err)
		}
	}
}

func TestDisabledFeatureRefusesEverythingExceptTheManagementHost(t *testing.T) {
	env := newPolicyEnv(t, Limits{})
	env.settings.cfg.CustomDomainsEnabled = false
	if err := env.allow("www.example.org"); !errors.Is(err, ErrRefused) {
		t.Errorf("verified domain with the feature off: %v", err)
	}
	if err := env.allow("admin.example.com"); err != nil {
		t.Errorf("management host refused: %v", err)
	}
}

func TestManagementHostIsHandledWithoutLookups(t *testing.T) {
	env := newPolicyEnv(t, Limits{})
	env.settings.err = errors.New("settings are down")
	env.lookup.err = errors.New("database is down")
	for _, name := range []string{"admin.example.com", "ADMIN.example.com.", " admin.example.com "} {
		if err := env.allow(name); err != nil {
			t.Errorf("%q refused: %v", name, err)
		}
	}
	if env.lookup.calls != 0 {
		t.Errorf("lookups = %d, want 0", env.lookup.calls)
	}
}

func TestSiteSubdomainsAreNeverIssuedOnDemand(t *testing.T) {
	env := newPolicyEnv(t, Limits{})
	env.lookup.verified["launch.sites.example.com"] = true
	for _, name := range []string{"launch.sites.example.com", "deep.launch.sites.example.com", "sites.example.com", "check-abc123.sites.example.com"} {
		if err := env.allow(name); !errors.Is(err, ErrRefused) {
			t.Errorf("%q: error = %v, want ErrRefused", name, err)
		}
	}
	if env.lookup.calls != 0 {
		t.Errorf("subdomain names reached the domain lookup %d times", env.lookup.calls)
	}
}

func TestFailuresToDecideRefuseTheName(t *testing.T) {
	env := newPolicyEnv(t, Limits{})
	env.lookup.err = errors.New("database is down")
	if err := env.allow("www.example.org"); !errors.Is(err, ErrRefused) {
		t.Errorf("lookup failure: %v", err)
	}
	env.settings.err = errors.New("settings are down")
	if err := env.allow("www.example.org"); !errors.Is(err, ErrRefused) {
		t.Errorf("settings failure: %v", err)
	}
}

func TestRefusalsNeverExposeTheRequestedName(t *testing.T) {
	env := newPolicyEnv(t, Limits{})
	err := env.allow("attacker-controlled.example.net")
	if err == nil || strings.Contains(err.Error(), "attacker") {
		t.Errorf("error = %v", err)
	}
	env.clock.now = env.clock.now.Add(2 * refusalLogInterval)
	_ = env.allow("another-attacker.example.net")
	if env.logs.Len() == 0 {
		t.Fatal("no summary was logged")
	}
	if strings.Contains(env.logs.String(), "attacker") {
		t.Errorf("a requested name reached the log: %s", env.logs.String())
	}
}

func TestRefusalLoggingIsRateLimited(t *testing.T) {
	env := newPolicyEnv(t, Limits{})
	for range 500 {
		_ = env.allow("unknown.example.org")
	}
	if env.logs.Len() != 0 {
		t.Errorf("logged within the interval: %s", env.logs.String())
	}
	env.clock.now = env.clock.now.Add(refusalLogInterval)
	_ = env.allow("unknown.example.org")
	if lines := strings.Count(env.logs.String(), "\n"); lines != 1 || !strings.Contains(env.logs.String(), "count=501") {
		t.Errorf("log = %q", env.logs.String())
	}
}

func (e *policyEnv) reserve(name string) error {
	return e.policy.Reserve(context.Background(), name)
}

func TestDecisionsDoNotUseUpTheIssuanceQuota(t *testing.T) {
	limits := Limits{
		Total:   ratelimit.Rule{Max: 1, Window: time.Hour},
		PerName: ratelimit.Rule{Max: 1, Window: time.Hour},
		PerSite: ratelimit.Rule{Max: 1, Window: time.Hour},
	}
	env := newPolicyEnv(t, limits)
	for range 200 {
		if err := env.allow("www.example.org"); err != nil {
			t.Fatalf("decision refused: %v", err)
		}
	}
	if err := env.reserve("www.example.org"); err != nil {
		t.Errorf("the quota was spent by decisions: %v", err)
	}
}

func TestIssuanceIsRateLimitedPerNameAndInTotal(t *testing.T) {
	limits := Limits{
		Total:   ratelimit.Rule{Max: 3, Window: time.Hour},
		PerName: ratelimit.Rule{Max: 2, Window: time.Hour},
		PerSite: ratelimit.Rule{Max: 100, Window: time.Hour},
	}
	env := newPolicyEnv(t, limits)
	for range 2 {
		if err := env.reserve("www.example.org"); err != nil {
			t.Fatalf("within the per-name limit: %v", err)
		}
	}
	if err := env.reserve("www.example.org"); !errors.Is(err, errRateLimited) {
		t.Errorf("per-name limit not enforced: %v", err)
	}
	if err := env.reserve("shop.example.org"); err != nil {
		t.Fatalf("third issuance in total: %v", err)
	}
	if err := env.reserve("shop.example.org"); !errors.Is(err, errRateLimited) {
		t.Errorf("total limit not enforced: %v", err)
	}
	env.clock.now = env.clock.now.Add(time.Hour)
	if err := env.reserve("www.example.org"); err != nil {
		t.Errorf("limit did not reset: %v", err)
	}
}

func TestOneSiteCannotUseMoreThanItsShareOfTheQuota(t *testing.T) {
	limits := Limits{
		Total:   ratelimit.Rule{Max: 50, Window: time.Hour},
		PerName: ratelimit.Rule{Max: 5, Window: time.Hour},
		PerSite: ratelimit.Rule{Max: 3, Window: time.Hour},
	}
	env := newPolicyEnv(t, limits)
	names := []string{"a.example.org", "b.example.org", "c.example.org", "d.example.org"}
	for _, name := range names {
		env.lookup.verified[name] = true
		env.lookup.sites[name] = "greedy"
	}
	for _, name := range names[:3] {
		if err := env.reserve(name); err != nil {
			t.Fatalf("within the site share: %v", err)
		}
	}
	if err := env.reserve(names[3]); !errors.Is(err, errRateLimited) {
		t.Errorf("site share not enforced: %v", err)
	}
	if err := env.reserve("www.example.org"); err != nil {
		t.Errorf("another site was blocked by the greedy one: %v", err)
	}
}

func TestARefusedReservationSpendsNoQuota(t *testing.T) {
	limits := Limits{
		Total:   ratelimit.Rule{Max: 2, Window: time.Hour},
		PerName: ratelimit.Rule{Max: 1, Window: time.Hour},
		PerSite: ratelimit.Rule{Max: 100, Window: time.Hour},
	}
	env := newPolicyEnv(t, limits)
	if err := env.reserve("www.example.org"); err != nil {
		t.Fatal(err)
	}
	for range 20 {
		if err := env.reserve("www.example.org"); !errors.Is(err, errRateLimited) {
			t.Fatalf("reservation over the per-name limit = %v", err)
		}
		if err := env.reserve("unknown.example.org"); !errors.Is(err, errNotVerified) {
			t.Fatalf("reservation for an unverified name = %v", err)
		}
	}
	if err := env.reserve("shop.example.org"); err != nil {
		t.Errorf("refusals used up the total quota: %v", err)
	}
}

func TestTheIssuerRefusesBeforeReachingTheACMEClient(t *testing.T) {
	env := newPolicyEnv(t, Limits{})
	issuer := limitedIssuer{policy: env.policy}
	_, err := issuer.Issue(context.Background(), &x509.CertificateRequest{DNSNames: []string{"unknown.example.org"}})
	if !errors.Is(err, errNotVerified) {
		t.Errorf("Issue = %v, want errNotVerified before any ACME call", err)
	}
}

func TestConcurrentReservationsCannotPassTheLimit(t *testing.T) {
	limits := Limits{
		Total:   ratelimit.Rule{Max: 50, Window: time.Hour},
		PerName: ratelimit.Rule{Max: 5, Window: time.Hour},
		PerSite: ratelimit.Rule{Max: 50, Window: time.Hour},
	}
	env := newPolicyEnv(t, limits)
	var granted atomic.Int32
	var wg sync.WaitGroup
	for range 40 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if env.reserve("www.example.org") == nil {
				granted.Add(1)
			}
		}()
	}
	wg.Wait()
	if granted.Load() != 5 {
		t.Errorf("granted %d reservations under a limit of 5", granted.Load())
	}
}

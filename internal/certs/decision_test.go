package certs

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/zachlagden/selfhost/internal/domains"
	"github.com/zachlagden/selfhost/internal/ratelimit"
	"github.com/zachlagden/selfhost/internal/settings"
)

type fakeSettings struct {
	cfg settings.Settings
	err error
}

func (f *fakeSettings) Get(context.Context) (settings.Settings, error) {
	return f.cfg, f.err
}

type fakeLookup struct {
	verified map[string]bool
	err      error
	calls    int
}

func (f *fakeLookup) Lookup(_ context.Context, host string) (domains.Target, bool, error) {
	f.calls++
	if f.err != nil {
		return domains.Target{}, false, f.err
	}
	return domains.Target{Domain: host}, f.verified[host], nil
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
		lookup:   &fakeLookup{verified: map[string]bool{"www.example.org": true, "shop.example.org": true}},
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

func TestIssuanceIsRateLimitedPerNameAndInTotal(t *testing.T) {
	limits := Limits{
		Total:   ratelimit.Rule{Max: 3, Window: time.Hour},
		PerName: ratelimit.Rule{Max: 2, Window: time.Hour},
	}
	env := newPolicyEnv(t, limits)
	for range 2 {
		if err := env.allow("www.example.org"); err != nil {
			t.Fatalf("within the per-name limit: %v", err)
		}
	}
	if err := env.allow("www.example.org"); !errors.Is(err, ErrRefused) {
		t.Errorf("per-name limit not enforced: %v", err)
	}
	if err := env.allow("shop.example.org"); err != nil {
		t.Fatalf("third issuance in total: %v", err)
	}
	if err := env.allow("shop.example.org"); !errors.Is(err, ErrRefused) {
		t.Errorf("total limit not enforced: %v", err)
	}
	env.clock.now = env.clock.now.Add(time.Hour)
	if err := env.allow("www.example.org"); err != nil {
		t.Errorf("limit did not reset: %v", err)
	}
}

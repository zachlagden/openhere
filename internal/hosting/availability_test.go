package hosting

import (
	"slices"
	"strings"
	"testing"

	"github.com/zachlagden/selfhost/internal/config"
	"github.com/zachlagden/selfhost/internal/settings"
)

var (
	secureURL = config.PublicURL{Scheme: "https", Host: "sites.example.com"}
	plainURL  = config.PublicURL{Scheme: "http", Host: "sites.example.com"}
)

func hostingConfig(subdomain, path bool, mutate func(*settings.Settings)) settings.Settings {
	cfg := settings.Defaults()
	cfg.BaseDomain = "sites.example.com"
	cfg.SubdomainEnabled, cfg.PathEnabled = subdomain, path
	cfg.WildcardStatus = settings.WildcardOK
	if mutate != nil {
		mutate(&cfg)
	}
	return cfg
}

func codesOf(notices []Notice) []string {
	codes := make([]string, len(notices))
	for i, n := range notices {
		codes[i] = n.Code
	}
	return codes
}

func TestSiteHostingStates(t *testing.T) {
	live := SiteFacts{Slug: "docs", Live: true}
	cases := []struct {
		name      string
		public    config.PublicURL
		cfg       settings.Settings
		facts     SiteFacts
		reachable bool
		subdomain ModeStatus
		path      ModeStatus
		notices   []string
	}{
		{"both modes on", secureURL, hostingConfig(true, true, nil), live, true,
			ModeStatus{StateOn, ""}, ModeStatus{StateOn, ""}, nil},
		{"subdomain switched off by an admin", secureURL, hostingConfig(false, true, nil), live, true,
			ModeStatus{StateOff, ReasonSwitchedOff}, ModeStatus{StateOn, ""}, []string{"subdomain_off"}},
		{"subdomain switched off by failed checks", secureURL, hostingConfig(false, true, func(s *settings.Settings) {
			s.SubdomainDisabledReason = settings.DisabledByWildcard
			s.WildcardStatus = settings.WildcardFailed
		}), live, true, ModeStatus{StateOff, ReasonWildcardFailed}, ModeStatus{StateOn, ""}, []string{"subdomain_off"}},
		{"subdomain serving with a failed check", secureURL, hostingConfig(true, true, func(s *settings.Settings) {
			s.WildcardStatus = settings.WildcardFailed
		}), live, true, ModeStatus{StateOn, ReasonWildcardFailed}, ModeStatus{StateOn, ""}, []string{"subdomain_unreliable"}},
		{"subdomain on without a base domain", secureURL, hostingConfig(true, true, func(s *settings.Settings) { s.BaseDomain = "" }), live, true,
			ModeStatus{StateOff, ReasonNotApplicable}, ModeStatus{StateOn, ""}, []string{"subdomain_off"}},
		{"path switched off", secureURL, hostingConfig(true, false, nil), live, true,
			ModeStatus{StateOn, ""}, ModeStatus{StateOff, ReasonSwitchedOff}, []string{"path_off"}},
		{"no live version", secureURL, hostingConfig(true, true, nil), SiteFacts{Slug: "docs"}, false,
			ModeStatus{StateOn, ""}, ModeStatus{StateOn, ""}, []string{"no_live_version"}},
		{"gated site in path mode over http", plainURL, hostingConfig(true, true, nil), SiteFacts{Slug: "docs", Live: true, Gated: true}, true,
			ModeStatus{StateOn, ""}, ModeStatus{StateOff, ReasonHTTPSRequired}, []string{"path_https_required"}},
		{"gated site over http with path as the only mode", plainURL, hostingConfig(false, true, nil), SiteFacts{Slug: "docs", Live: true, Gated: true}, false,
			ModeStatus{StateOff, ReasonSwitchedOff}, ModeStatus{StateOff, ReasonHTTPSRequired}, []string{"subdomain_off", "path_https_required", "unreachable"}},
		{"public site over http", plainURL, hostingConfig(false, true, nil), live, true,
			ModeStatus{StateOff, ReasonSwitchedOff}, ModeStatus{StateOn, ""}, []string{"subdomain_off"}},
		{"gated site over https", secureURL, hostingConfig(false, true, nil), SiteFacts{Slug: "docs", Live: true, Gated: true}, true,
			ModeStatus{StateOff, ReasonSwitchedOff}, ModeStatus{StateOn, ""}, []string{"subdomain_off"}},
		{"nothing published and path as the only mode", secureURL, hostingConfig(false, true, nil), SiteFacts{Slug: "docs"}, false,
			ModeStatus{StateOff, ReasonSwitchedOff}, ModeStatus{StateOn, ""}, []string{"no_live_version", "subdomain_off"}},
	}
	for _, tc := range cases {
		got := SiteHosting(tc.public, tc.cfg, tc.facts)
		if got.Reachable != tc.reachable || got.Subdomain != tc.subdomain || got.Path != tc.path {
			t.Errorf("%s: got reachable=%v subdomain=%+v path=%+v", tc.name, got.Reachable, got.Subdomain, got.Path)
		}
		if !slices.Equal(codesOf(got.Notices), tc.notices) {
			t.Errorf("%s: notices = %v, want %v", tc.name, codesOf(got.Notices), tc.notices)
		}
	}
}

func TestSiteHostingURLMatchesSiteURL(t *testing.T) {
	cfg := hostingConfig(true, true, nil)
	got := SiteHosting(secureURL, cfg, SiteFacts{Slug: "docs", Live: true})
	if got.URL != "https://docs.sites.example.com/" || got.URL != SiteURL(secureURL, cfg, "docs") {
		t.Errorf("url = %q", got.URL)
	}
}

func TestNoticeMessagesAreWrittenForAnyUser(t *testing.T) {
	cfg := hostingConfig(false, false, func(s *settings.Settings) {
		s.SubdomainDisabledReason = settings.DisabledByWildcard
		s.WildcardStatus = settings.WildcardFailed
		s.WildcardDetail = "DNS did not resolve check-abc.sites.example.com"
		s.WildcardFailures = 3
	})
	site := SiteHosting(plainURL, cfg, SiteFacts{Slug: "docs", Live: true, Gated: true})
	instance := InstanceHosting(cfg)
	notices := append(append([]Notice{}, site.Notices...), instance.Notices...)
	if len(notices) == 0 {
		t.Fatal("no notices")
	}
	for _, n := range notices {
		lower := strings.ToLower(n.Message)
		for _, banned := range []string{"wildcard", "dns", "check-", "settings", "admin", "root", "example.com"} {
			if strings.Contains(lower, banned) {
				t.Errorf("notice %s leaks %q: %q", n.Code, banned, n.Message)
			}
		}
	}
}

func TestInstanceHosting(t *testing.T) {
	healthy := InstanceHosting(hostingConfig(true, true, nil))
	if healthy.Degraded || len(healthy.Notices) != 0 || healthy.Subdomain.State != StateOn || healthy.Path.State != StateOn {
		t.Errorf("healthy = %+v", healthy)
	}
	switchedOff := InstanceHosting(hostingConfig(false, true, nil))
	if switchedOff.Degraded || switchedOff.Subdomain != (ModeStatus{StateOff, ReasonSwitchedOff}) || !slices.Equal(codesOf(switchedOff.Notices), []string{"subdomain_off"}) {
		t.Errorf("switched off = %+v", switchedOff)
	}
	failing := InstanceHosting(hostingConfig(true, true, func(s *settings.Settings) { s.WildcardStatus = settings.WildcardFailed }))
	if !failing.Degraded || failing.Subdomain != (ModeStatus{StateOn, ReasonWildcardFailed}) || !slices.Equal(codesOf(failing.Notices), []string{"subdomain_unreliable"}) {
		t.Errorf("failing = %+v", failing)
	}
	disabled := InstanceHosting(hostingConfig(false, true, func(s *settings.Settings) { s.SubdomainDisabledReason = settings.DisabledByWildcard }))
	if !disabled.Degraded || disabled.Subdomain != (ModeStatus{StateOff, ReasonWildcardFailed}) {
		t.Errorf("auto-disabled = %+v", disabled)
	}
	pathOff := InstanceHosting(hostingConfig(true, false, nil))
	if pathOff.Path != (ModeStatus{StateOff, ReasonSwitchedOff}) || !slices.Equal(codesOf(pathOff.Notices), []string{"path_off"}) {
		t.Errorf("path off = %+v", pathOff)
	}
}

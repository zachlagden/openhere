package hosting

import (
	"testing"

	"github.com/zachlagden/selfhost/internal/config"
	"github.com/zachlagden/selfhost/internal/domains"
	"github.com/zachlagden/selfhost/internal/settings"
)

func TestNormaliseHost(t *testing.T) {
	cases := map[string]string{
		"sites.example.com":      "sites.example.com",
		"Sites.Example.COM":      "sites.example.com",
		"sites.example.com:8080": "sites.example.com",
		"sites.example.com.":     "sites.example.com",
		"SITES.example.com.:443": "sites.example.com",
		"[2001:db8::1]:8080":     "2001:db8::1",
		"[2001:db8::1]":          "2001:db8::1",
		"":                       "",
		"localhost":              "localhost",
		"demo.localhost:3000":    "demo.localhost",
		"sites.example.com..":    "sites.example.com.",
		"sites.example.com:":     "sites.example.com",
	}
	for raw, want := range cases {
		if got := NormaliseHost(raw); got != want {
			t.Errorf("NormaliseHost(%q) = %q, want %q", raw, got, want)
		}
	}
}

func TestSubdomainSlug(t *testing.T) {
	cases := []struct {
		host, base, want string
		ok               bool
	}{
		{"demo.sites.example.com", "sites.example.com", "demo", true},
		{"my-site-2.sites.example.com", "sites.example.com", "my-site-2", true},
		{"sites.example.com", "sites.example.com", "", false},
		{"a.b.sites.example.com", "sites.example.com", "", false},
		{"demo.example.com", "sites.example.com", "", false},
		{"demosites.example.com", "sites.example.com", "", false},
		{".sites.example.com", "sites.example.com", "", false},
		{"-bad.sites.example.com", "sites.example.com", "", false},
		{"under_score.sites.example.com", "sites.example.com", "", false},
		{"demo.sites.example.com", "", "", false},
		{"demo.localhost", "localhost", "demo", true},
	}
	for _, tc := range cases {
		got, ok := SubdomainSlug(tc.host, tc.base)
		if got != tc.want || ok != tc.ok {
			t.Errorf("SubdomainSlug(%q, %q) = %q, %v, want %q, %v", tc.host, tc.base, got, ok, tc.want, tc.ok)
		}
	}
}

func TestSiteURL(t *testing.T) {
	https := config.PublicURL{Scheme: "https", Host: "sites.example.com"}
	local := config.PublicURL{Scheme: "http", Host: "localhost", Port: "8080"}
	both := settings.Settings{BaseDomain: "sites.example.com", SubdomainEnabled: true, PathEnabled: true}
	pathOnly := settings.Settings{BaseDomain: "sites.example.com", PathEnabled: true}
	none := settings.Settings{BaseDomain: "sites.example.com"}
	noDomain := settings.Settings{SubdomainEnabled: true, PathEnabled: true}

	cases := []struct {
		name   string
		public config.PublicURL
		cfg    settings.Settings
		want   string
	}{
		{"subdomain wins", https, both, "https://demo.sites.example.com/"},
		{"path when subdomains are off", https, pathOnly, "https://sites.example.com/s/demo/"},
		{"nothing enabled", https, none, ""},
		{"subdomain without a base domain", https, noDomain, "https://sites.example.com/s/demo/"},
		{"port carries over to subdomains", local, settings.Settings{BaseDomain: "localhost", SubdomainEnabled: true}, "http://demo.localhost:8080/"},
		{"port carries over to paths", local, pathOnly, "http://localhost:8080/s/demo/"},
	}
	for _, tc := range cases {
		if got := SiteURL(tc.public, tc.cfg, "demo"); got != tc.want {
			t.Errorf("%s: SiteURL = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestPreferredURLOrder(t *testing.T) {
	public := config.PublicURL{Scheme: "https", Host: "sites.example.com", Port: "8443"}
	cfg := settings.Defaults()
	cfg.BaseDomain = "sites.example.com"
	cfg.SubdomainEnabled = true
	primary := domains.Summary{Total: 2, Verified: 2, Primary: "www.example.org", Fallback: "a.example.org"}
	if got := PreferredURL(public, cfg, "demo", primary); got != "https://www.example.org/" {
		t.Errorf("primary = %q", got)
	}
	if got := PreferredURL(public, cfg, "demo", domains.Summary{Total: 1, Verified: 1, Fallback: "a.example.org"}); got != "https://demo.sites.example.com:8443/" {
		t.Errorf("subdomain = %q", got)
	}
	cfg.SubdomainEnabled = false
	if got := PreferredURL(public, cfg, "demo", domains.Summary{}); got != "https://sites.example.com:8443/s/demo/" {
		t.Errorf("path = %q", got)
	}
	cfg.PathEnabled = false
	if got := PreferredURL(public, cfg, "demo", domains.Summary{Total: 1, Verified: 1, Fallback: "a.example.org"}); got != "https://a.example.org/" {
		t.Errorf("fallback = %q", got)
	}
	cfg.CustomDomainsEnabled = false
	if got := PreferredURL(public, cfg, "demo", primary); got != "" {
		t.Errorf("disabled = %q", got)
	}
}

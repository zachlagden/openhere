package proxy

import (
	"net/http/httptest"
	"net/netip"
	"testing"
)

func TestClientIP(t *testing.T) {
	trust := Trust{Prefixes: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8"), netip.MustParsePrefix("fd00::/8")}}
	cases := []struct {
		name   string
		remote string
		xff    []string
		want   string
	}{
		{"direct client", "203.0.113.5:1234", nil, "203.0.113.5"},
		{"untrusted peer cannot spoof", "203.0.113.5:1234", []string{"198.51.100.7"}, "203.0.113.5"},
		{"trusted proxy reports client", "10.0.0.2:80", []string{"198.51.100.7"}, "198.51.100.7"},
		{"client-supplied hop is skipped", "10.0.0.2:80", []string{"1.1.1.1, 198.51.100.7"}, "198.51.100.7"},
		{"chain of trusted proxies", "10.0.0.2:80", []string{"198.51.100.7, 10.0.0.9"}, "198.51.100.7"},
		{"split header lines", "10.0.0.2:80", []string{"1.1.1.1", "198.51.100.7"}, "198.51.100.7"},
		{"garbage falls back to peer", "10.0.0.2:80", []string{"not-an-ip"}, "10.0.0.2"},
		{"no header from trusted peer", "10.0.0.2:80", nil, "10.0.0.2"},
		{"mapped ipv4", "[::ffff:10.0.0.2]:80", []string{"198.51.100.7"}, "198.51.100.7"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(t.Context(), "GET", "/", nil)
			req.RemoteAddr = tc.remote
			for _, v := range tc.xff {
				req.Header.Add("X-Forwarded-For", v)
			}
			if got := trust.ClientIP(req).String(); got != tc.want {
				t.Errorf("ClientIP = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestIsHTTPS(t *testing.T) {
	trust := Trust{Prefixes: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}}
	cases := []struct {
		name   string
		remote string
		proto  string
		want   bool
	}{
		{"trusted https", "10.0.0.2:80", "https", true},
		{"trusted https mixed case list", "10.0.0.2:80", "HTTPS, http", true},
		{"trusted http", "10.0.0.2:80", "http", false},
		{"untrusted https", "203.0.113.5:80", "https", false},
		{"no header", "10.0.0.2:80", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(t.Context(), "GET", "/", nil)
			req.RemoteAddr = tc.remote
			if tc.proto != "" {
				req.Header.Set("X-Forwarded-Proto", tc.proto)
			}
			if got := trust.IsHTTPS(req); got != tc.want {
				t.Errorf("IsHTTPS = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestLimiterKeyAggregatesIPv6Subnets(t *testing.T) {
	a := LimiterKey(netip.MustParseAddr("2001:db8:1:2::1"))
	b := LimiterKey(netip.MustParseAddr("2001:db8:1:2:ffff::9"))
	c := LimiterKey(netip.MustParseAddr("2001:db8:1:3::1"))
	if a != b || a == c {
		t.Errorf("ipv6 keys = %s, %s, %s", a, b, c)
	}
	if got := LimiterKey(netip.MustParseAddr("203.0.113.5")); got != "203.0.113.5" {
		t.Errorf("ipv4 key = %s", got)
	}
	if got := LimiterKey(netip.Addr{}); got != "unknown" {
		t.Errorf("invalid key = %s", got)
	}
}

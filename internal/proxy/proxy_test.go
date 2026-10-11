package proxy

import (
	"crypto/tls"
	"net/http"
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

func TestIsHTTPSCountsTheRequestsOwnTLSState(t *testing.T) {
	cases := []struct {
		name   string
		trust  Trust
		remote string
		proto  string
	}{
		{"no proxies configured", Trust{}, "203.0.113.5:443", ""},
		{"untrusted peer sends http", Trust{}, "203.0.113.5:443", "http"},
		{"trusted peer sends http", Trust{Prefixes: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}}, "10.0.0.2:443", "http"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(t.Context(), "GET", "/", nil)
			req.RemoteAddr = tc.remote
			if tc.proto != "" {
				req.Header.Set("X-Forwarded-Proto", tc.proto)
			}
			if tc.trust.IsHTTPS(req) {
				t.Fatal("plain request counted as HTTPS")
			}
			req.TLS = &tls.ConnectionState{}
			if !tc.trust.IsHTTPS(req) {
				t.Error("a request with its own TLS state did not count as HTTPS")
			}
		})
	}
}

func TestClientIPHeader(t *testing.T) {
	trust := Trust{
		Prefixes:       []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")},
		ClientIPHeader: "Cf-Connecting-Ip",
	}
	const edge = "172.69.1.1"
	cases := []struct {
		name   string
		remote string
		header []string
		xff    []string
		want   string
	}{
		{"trusted peer and valid header", "10.0.0.2:80", []string{"198.51.100.7"}, []string{edge}, "198.51.100.7"},
		{"header wins over forwarded for", "10.0.0.2:80", []string{"198.51.100.7"}, []string{"198.51.100.99"}, "198.51.100.7"},
		{"valid ipv6", "10.0.0.2:80", []string{"2001:db8::1"}, []string{edge}, "2001:db8::1"},
		{"mapped ipv4 is unmapped", "10.0.0.2:80", []string{"::ffff:198.51.100.7"}, []string{edge}, "198.51.100.7"},
		{"forged second header line", "10.0.0.2:80", []string{"198.51.100.7", "203.0.113.50"}, []string{edge}, edge},
		{"forged list", "10.0.0.2:80", []string{"203.0.113.50, 198.51.100.7"}, []string{edge}, edge},
		{"port", "10.0.0.2:80", []string{"198.51.100.7:443"}, []string{edge}, edge},
		{"bracketed ipv6", "10.0.0.2:80", []string{"[2001:db8::1]"}, []string{edge}, edge},
		{"bracketed ipv6 with port", "10.0.0.2:80", []string{"[2001:db8::1]:443"}, []string{edge}, edge},
		{"ipv6 zone", "10.0.0.2:80", []string{"fe80::1%eth0"}, []string{edge}, edge},
		{"hostname", "10.0.0.2:80", []string{"client.example.com"}, []string{edge}, edge},
		{"junk", "10.0.0.2:80", []string{"unknown"}, []string{edge}, edge},
		{"empty value", "10.0.0.2:80", []string{""}, []string{edge}, edge},
		{"whitespace only", "10.0.0.2:80", []string{"   "}, []string{edge}, edge},
		{"padded address", "10.0.0.2:80", []string{" 198.51.100.7"}, []string{edge}, edge},
		{"malformed header and no forwarded for", "10.0.0.2:80", []string{"junk"}, nil, "10.0.0.2"},
		{"missing header falls back", "10.0.0.2:80", nil, []string{edge}, edge},
		{"missing header and no forwarded for", "10.0.0.2:80", nil, nil, "10.0.0.2"},
		{"untrusted peer with forged header", "203.0.113.5:1234", []string{"198.51.100.7"}, []string{"198.51.100.8"}, "203.0.113.5"},
		{"untrusted peer with forged header only", "203.0.113.5:1234", []string{"198.51.100.7"}, nil, "203.0.113.5"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(t.Context(), "GET", "/", nil)
			req.RemoteAddr = tc.remote
			for _, v := range tc.header {
				req.Header.Add("Cf-Connecting-Ip", v)
			}
			for _, v := range tc.xff {
				req.Header.Add("X-Forwarded-For", v)
			}
			if got := trust.ClientIP(req).String(); got != tc.want {
				t.Errorf("ClientIP = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestClientIPHeaderUnsetKeepsForwardedFor(t *testing.T) {
	trust := Trust{Prefixes: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}}
	req := httptest.NewRequestWithContext(t.Context(), "GET", "/", nil)
	req.RemoteAddr = "10.0.0.2:80"
	req.Header.Set("Cf-Connecting-Ip", "198.51.100.7")
	req.Header.Set("X-Forwarded-For", "198.51.100.8")
	if got := trust.ClientIP(req).String(); got != "198.51.100.8" {
		t.Errorf("ClientIP = %s, want 198.51.100.8", got)
	}
}

func TestClientIPHeaderKeepsIPv6Grouping(t *testing.T) {
	trust := Trust{
		Prefixes:       []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")},
		ClientIPHeader: "Cf-Connecting-Ip",
	}
	key := func(value string) string {
		req := httptest.NewRequestWithContext(t.Context(), "GET", "/", http.NoBody)
		req.RemoteAddr = "10.0.0.2:80"
		req.Header.Set("Cf-Connecting-Ip", value)
		return LimiterKey(trust.ClientIP(req))
	}
	a, b, c := key("2001:db8:1:2::1"), key("2001:db8:1:2:ffff::9"), key("2001:db8:1:3::1")
	if a != b || a == c || a != "2001:db8:1:2::/64" {
		t.Errorf("ipv6 keys = %s, %s, %s", a, b, c)
	}
}

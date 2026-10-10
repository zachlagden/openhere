package config

import (
	"encoding/base64"
	"log/slog"
	"strings"
	"testing"
)

func lookupFrom(values map[string]string) Lookup {
	return func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	}
}

func TestLoadDefaults(t *testing.T) {
	cfg, err := Load(lookupFrom(nil))
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.ListenAddr != ":8080" || cfg.DataDir != "./data" || cfg.DatabaseURL != "" || cfg.SentryDSN != "" {
		t.Errorf("unexpected defaults: %+v", cfg)
	}
	if cfg.LogLevel != slog.LevelInfo {
		t.Errorf("log level = %v, want info", cfg.LogLevel)
	}
}

func TestLoadOverrides(t *testing.T) {
	cfg, err := Load(lookupFrom(map[string]string{
		"LISTEN_ADDR":  "127.0.0.1:9000",
		"DATA_DIR":     "/srv/data",
		"DATABASE_URL": "postgres://db.example.com:5432/app",
		"LOG_LEVEL":    "DEBUG",
		"SENTRY_DSN":   "https://key@glitchtip.example.com/1",
	}))
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.ListenAddr != "127.0.0.1:9000" || cfg.DataDir != "/srv/data" {
		t.Errorf("unexpected values: %+v", cfg)
	}
	if cfg.LogLevel != slog.LevelDebug {
		t.Errorf("log level = %v, want debug", cfg.LogLevel)
	}
	if cfg.DatabaseURL == "" || cfg.SentryDSN == "" {
		t.Errorf("database URL and sentry DSN should be set: %+v", cfg)
	}
}

func TestLoadAuthDefaults(t *testing.T) {
	cfg, err := Load(lookupFrom(nil))
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.APIKeyPrefix != "sk_" {
		t.Errorf("API key prefix = %q, want sk_", cfg.APIKeyPrefix)
	}
	if len(cfg.TrustedProxies) != 0 {
		t.Errorf("trusted proxies = %v, want none", cfg.TrustedProxies)
	}
}

func TestLoadAuthOverrides(t *testing.T) {
	cfg, err := Load(lookupFrom(map[string]string{
		"API_KEY_PREFIX":  "acme_",
		"TRUSTED_PROXIES": "10.0.0.1, 192.168.0.0/16,,fd00::/8",
	}))
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.APIKeyPrefix != "acme_" {
		t.Errorf("API key prefix = %q, want acme_", cfg.APIKeyPrefix)
	}
	want := []string{"10.0.0.1/32", "192.168.0.0/16", "fd00::/8"}
	if len(cfg.TrustedProxies) != len(want) {
		t.Fatalf("trusted proxies = %v, want %v", cfg.TrustedProxies, want)
	}
	for i, prefix := range cfg.TrustedProxies {
		if prefix.String() != want[i] {
			t.Errorf("trusted proxy %d = %s, want %s", i, prefix, want[i])
		}
	}
}

func TestLoadBlankValuesUseDefaults(t *testing.T) {
	cfg, err := Load(lookupFrom(map[string]string{"LISTEN_ADDR": "  ", "LOG_LEVEL": ""}))
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.ListenAddr != ":8080" || cfg.LogLevel != slog.LevelInfo {
		t.Errorf("blank values should fall back to defaults: %+v", cfg)
	}
}

func TestLoadRejectsInvalidValues(t *testing.T) {
	cases := map[string]struct {
		env  map[string]string
		want string
	}{
		"listen address without port": {map[string]string{"LISTEN_ADDR": "localhost"}, "LISTEN_ADDR"},
		"unknown log level":           {map[string]string{"LOG_LEVEL": "loud"}, "LOG_LEVEL"},
		"non postgres database url":   {map[string]string{"DATABASE_URL": "mysql://db.example.com/app"}, "DATABASE_URL"},
		"database url without host":   {map[string]string{"DATABASE_URL": "postgres:///app"}, "DATABASE_URL"},
		"sentry dsn not a url":        {map[string]string{"SENTRY_DSN": "not-a-dsn"}, "SENTRY_DSN"},
		"key prefix with symbols":     {map[string]string{"API_KEY_PREFIX": "sk-"}, "API_KEY_PREFIX"},
		"key prefix too long":         {map[string]string{"API_KEY_PREFIX": "abcdefghijklmnopq"}, "API_KEY_PREFIX"},
		"trusted proxy not an ip":     {map[string]string{"TRUSTED_PROXIES": "10.0.0.1, proxy"}, "TRUSTED_PROXIES"},
		"argon2 concurrency zero":     {map[string]string{"ARGON2_CONCURRENCY": "0"}, "ARGON2_CONCURRENCY"},
		"argon2 concurrency too high": {map[string]string{"ARGON2_CONCURRENCY": "17"}, "ARGON2_CONCURRENCY"},
		"argon2 concurrency text":     {map[string]string{"ARGON2_CONCURRENCY": "many"}, "ARGON2_CONCURRENCY"},
		"trusted proxy bad cidr":      {map[string]string{"TRUSTED_PROXIES": "10.0.0.0/40"}, "TRUSTED_PROXIES"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := Load(lookupFrom(tc.env))
			if err == nil {
				t.Fatal("expected an error")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q should mention %s", err, tc.want)
			}
		})
	}
}

func TestLoadReportsEveryProblem(t *testing.T) {
	_, err := Load(lookupFrom(map[string]string{"LISTEN_ADDR": "nope", "LOG_LEVEL": "loud"}))
	if err == nil {
		t.Fatal("expected an error")
	}
	for _, key := range []string{"LISTEN_ADDR", "LOG_LEVEL"} {
		if !strings.Contains(err.Error(), key) {
			t.Errorf("error %q should mention %s", err, key)
		}
	}
}

func TestPublicURLDefaultsFollowTheListenPort(t *testing.T) {
	cases := map[string]string{
		"":               "http://localhost:8080",
		":9000":          "http://localhost:9000",
		"127.0.0.1:3000": "http://localhost:3000",
		":80":            "http://localhost",
	}
	for listen, want := range cases {
		env := map[string]string{}
		if listen != "" {
			env["LISTEN_ADDR"] = listen
		}
		cfg, err := Load(lookupFrom(env))
		if err != nil {
			t.Fatalf("listen %q: %v", listen, err)
		}
		if cfg.PublicURL.String() != want || cfg.PublicURL.Secure() {
			t.Errorf("listen %q: public URL = %s, want %s over http", listen, cfg.PublicURL, want)
		}
	}
}

func TestPublicURLParsing(t *testing.T) {
	cases := map[string]PublicURL{
		"https://sites.example.com":      {Scheme: "https", Host: "sites.example.com"},
		"HTTPS://Sites.Example.com/":     {Scheme: "https", Host: "sites.example.com"},
		"http://sites.example.com:8443":  {Scheme: "http", Host: "sites.example.com", Port: "8443"},
		"https://sites.example.com.":     {Scheme: "https", Host: "sites.example.com"},
		"http://[2001:db8::1]:8080":      {Scheme: "http", Host: "2001:db8::1", Port: "8080"},
		"  https://sites.example.com  ":  {Scheme: "https", Host: "sites.example.com"},
		"https://sites.example.com:443/": {Scheme: "https", Host: "sites.example.com", Port: "443"},
	}
	for raw, want := range cases {
		cfg, err := Load(lookupFrom(map[string]string{"PUBLIC_URL": raw}))
		if err != nil {
			t.Fatalf("%q: %v", raw, err)
		}
		if cfg.PublicURL != want {
			t.Errorf("%q: got %+v, want %+v", raw, cfg.PublicURL, want)
		}
	}
	cfg, err := Load(lookupFrom(map[string]string{"PUBLIC_URL": "https://sites.example.com"}))
	if err != nil || !cfg.PublicURL.Secure() || cfg.PublicURL.String() != "https://sites.example.com" {
		t.Errorf("https URL = %+v, %v", cfg.PublicURL, err)
	}
	ipv6 := PublicURL{Scheme: "http", Host: "2001:db8::1", Port: "8080"}
	if ipv6.String() != "http://[2001:db8::1]:8080" {
		t.Errorf("ipv6 string = %s", ipv6)
	}
}

func TestPublicURLRejectsBadValues(t *testing.T) {
	for _, raw := range []string{
		"sites.example.com", "ftp://sites.example.com", "https://", "https://sites.example.com/path",
		"https://sites.example.com?x=1", "https://sites.example.com#top", "https://alex:secret@sites.example.com",
		"https://sites.example.com:0", "https://sites.example.com:70000", "https://sites.example.com:abc", "https://sites.example.com/?",
	} {
		_, err := Load(lookupFrom(map[string]string{"PUBLIC_URL": raw}))
		if err == nil || !strings.Contains(err.Error(), "PUBLIC_URL") {
			t.Errorf("%q: error = %v, want a PUBLIC_URL problem", raw, err)
		}
	}
}

func TestLoadSecretKey(t *testing.T) {
	cfg, err := Load(lookupFrom(nil))
	if err != nil || cfg.SecretKey != nil {
		t.Fatalf("default secret key = %v, %v; want none", cfg.SecretKey, err)
	}
	valid := base64.StdEncoding.EncodeToString(make([]byte, 32))
	cfg, err = Load(lookupFrom(map[string]string{"SECRET_KEY": valid}))
	if err != nil || len(cfg.SecretKey) != 32 {
		t.Fatalf("valid secret key: %v, %v", cfg.SecretKey, err)
	}
	for _, bad := range []string{"short", base64.StdEncoding.EncodeToString(make([]byte, 16))} {
		_, err := Load(lookupFrom(map[string]string{"SECRET_KEY": bad}))
		if err == nil || strings.Contains(err.Error(), bad) {
			t.Errorf("SECRET_KEY %q: error = %v, want a rejection that does not echo the value", bad, err)
		}
	}
}

func TestAgentCredentialsPath(t *testing.T) {
	if got := AgentCredentialsPath(); got != "~/.selfhost/credentials" {
		t.Errorf("AgentCredentialsPath = %q", got)
	}
}

func tlsValues(overrides map[string]string) map[string]string {
	values := map[string]string{
		"TLS_ENABLED": "true",
		"ACME_EMAIL":  "admin@example.com",
		"PUBLIC_URL":  "https://sites.example.com",
		"DATA_DIR":    "/srv/data",
	}
	for key, value := range overrides {
		values[key] = value
	}
	return values
}

func TestTLSIsOffByDefault(t *testing.T) {
	cfg, err := Load(lookupFrom(nil))
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.TLS.Enabled {
		t.Errorf("TLS enabled by default: %+v", cfg.TLS)
	}
}

func TestTLSDefaultsWhenEnabled(t *testing.T) {
	cfg, err := Load(lookupFrom(tlsValues(nil)))
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	want := TLS{
		Enabled: true, HTTPAddr: ":80", HTTPSAddr: ":443", Email: "admin@example.com",
		CA: "https://acme-v02.api.letsencrypt.org/directory", Storage: "/srv/data/certs",
	}
	if cfg.TLS != want {
		t.Errorf("TLS = %+v, want %+v", cfg.TLS, want)
	}
	if cfg.TLS.HTTPPort() != "80" || cfg.TLS.HTTPSPort() != "443" {
		t.Errorf("ports = %s, %s", cfg.TLS.HTTPPort(), cfg.TLS.HTTPSPort())
	}
}

func TestTLSOverrides(t *testing.T) {
	cfg, err := Load(lookupFrom(tlsValues(map[string]string{
		"TLS_HTTP_ADDR":  "0.0.0.0:8080",
		"TLS_HTTPS_ADDR": "[::]:8443",
		"ACME_CA":        "https://acme-staging-v02.api.letsencrypt.org/directory",
		"ACME_CA_ROOT":   "/etc/ssl/ca.pem",
		"TLS_STORAGE":    "/var/lib/certs",
		"LISTEN_ADDR":    ":9999",
	})))
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.TLS.HTTPAddr != "0.0.0.0:8080" || cfg.TLS.HTTPSAddr != "[::]:8443" || cfg.TLS.CARoot != "/etc/ssl/ca.pem" ||
		cfg.TLS.Storage != "/var/lib/certs" || !strings.Contains(cfg.TLS.CA, "staging") {
		t.Errorf("unexpected TLS config: %+v", cfg.TLS)
	}
}

func TestTLSValidation(t *testing.T) {
	cases := []struct {
		name      string
		overrides map[string]string
		remove    string
		want      string
	}{
		{"missing email", nil, "ACME_EMAIL", "ACME_EMAIL is required"},
		{"bad email", map[string]string{"ACME_EMAIL": "Alex <admin@example.com>"}, "", "ACME_EMAIL must be a plain email"},
		{"http public url", map[string]string{"PUBLIC_URL": "http://sites.example.com"}, "", "PUBLIC_URL must be set to an https://"},
		{"missing public url", nil, "PUBLIC_URL", "PUBLIC_URL must be set to an https://"},
		{"ip public url", map[string]string{"PUBLIC_URL": "https://203.0.113.5"}, "", "host name, not an IP address"},
		{"bad http addr", map[string]string{"TLS_HTTP_ADDR": "eighty"}, "", "TLS_HTTP_ADDR"},
		{"bad https addr", map[string]string{"TLS_HTTPS_ADDR": "443"}, "", "TLS_HTTPS_ADDR"},
		{"same port", map[string]string{"TLS_HTTP_ADDR": ":8443", "TLS_HTTPS_ADDR": "127.0.0.1:8443"}, "", "different ports"},
		{"bad ca", map[string]string{"ACME_CA": "ftp://ca.example.com"}, "", "ACME_CA must be"},
		{"bad flag", map[string]string{"TLS_ENABLED": "maybe"}, "", "TLS_ENABLED"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			values := tlsValues(tc.overrides)
			if tc.remove != "" {
				delete(values, tc.remove)
			}
			_, err := Load(lookupFrom(values))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %v, want one containing %q", err, tc.want)
			}
		})
	}
}

func TestTLSSettingsAreIgnoredWhenDisabled(t *testing.T) {
	cfg, err := Load(lookupFrom(map[string]string{"TLS_HTTP_ADDR": "bad", "ACME_CA": "bad"}))
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.TLS.Enabled {
		t.Errorf("TLS should stay off: %+v", cfg.TLS)
	}
}

func TestArgon2Concurrency(t *testing.T) {
	cfg, err := Load(lookupFrom(nil))
	if err != nil || cfg.Argon2Concurrency != 2 {
		t.Fatalf("default = %d, %v; want 2", cfg.Argon2Concurrency, err)
	}
	cfg, err = Load(lookupFrom(map[string]string{"ARGON2_CONCURRENCY": "16"}))
	if err != nil || cfg.Argon2Concurrency != 16 {
		t.Fatalf("override = %d, %v; want 16", cfg.Argon2Concurrency, err)
	}
}

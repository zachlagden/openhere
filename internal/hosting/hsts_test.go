package hosting

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zachlagden/openhere/internal/store"
	"github.com/zachlagden/openhere/internal/store/storetest"
)

func (e *env) getOver(secure bool, host, target string) *httptest.ResponseRecorder {
	e.t.Helper()
	req := httptest.NewRequestWithContext(e.t.Context(), http.MethodGet, "http://"+host+target, nil)
	req.Host = host
	if secure {
		req.TLS = &tls.ConnectionState{}
	}
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	return rec
}

func hstsOf(rec *httptest.ResponseRecorder) string {
	return rec.Header().Get("Strict-Transport-Security")
}

func TestHSTSIsSentOverTLSOnEveryServedHost(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnvWith(t, db, envOptions{hsts: true})
		e.publishSite("demo", demoSite)
		e.verifiedDomain("demo", customHost)
		e.enableSubdomains()
		want := "max-age=31536000"
		if StrictTransportSecurity != want {
			t.Fatalf("constant = %q", StrictTransportSecurity)
		}
		for name, tc := range map[string]struct{ host, target string }{
			"management host":   {managementHost, "/api/v1/me"},
			"path mode":         {managementHost, "/s/demo/"},
			"subdomain":         {siteHost, "/"},
			"subdomain missing": {siteHost, "/missing"},
			"custom domain":     {customHost, "/"},
		} {
			rec := e.getOver(true, tc.host, tc.target)
			if got := hstsOf(rec); got != want {
				t.Errorf("%s: HSTS = %q, want %q", name, got, want)
			}
		}
	})
}

func TestHSTSNeverCoversMoreThanTheHostThatSentIt(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnvWith(t, db, envOptions{hsts: true})
		e.publishSite("demo", demoSite)
		e.enableSubdomains()
		for _, host := range []string{managementHost, siteHost} {
			value := hstsOf(e.getOver(true, host, "/"))
			for _, banned := range []string{"includeSubDomains", "preload"} {
				if strings.Contains(value, banned) {
					t.Errorf("%s: HSTS %q contains %s", host, value, banned)
				}
			}
		}
	})
}

func TestHSTSIsNotSentOverPlainHTTP(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnvWith(t, db, envOptions{hsts: true})
		e.publishSite("demo", demoSite)
		e.verifiedDomain("demo", customHost)
		e.enableSubdomains()
		for _, host := range []string{managementHost, siteHost, customHost} {
			if got := hstsOf(e.getOver(false, host, "/")); got != "" {
				t.Errorf("%s over HTTP: HSTS = %q", host, got)
			}
		}
	})
}

func TestHSTSIsNotSentWhenBuiltInTLSIsOff(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.publishSite("demo", demoSite)
		e.verifiedDomain("demo", customHost)
		e.enableSubdomains()
		for _, host := range []string{managementHost, siteHost, customHost} {
			if got := hstsOf(e.getOver(true, host, "/")); got != "" {
				t.Errorf("%s with TLS off: HSTS = %q", host, got)
			}
		}
	})
}

func TestHSTSIsNotSentForUnknownHosts(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnvWith(t, db, envOptions{hsts: true})
		e.enableSubdomains()
		for _, host := range []string{"unknown.example.net", "pending.example.org"} {
			if got := hstsOf(e.getOver(true, host, "/")); got != "" {
				t.Errorf("%s: HSTS = %q", host, got)
			}
		}
	})
}

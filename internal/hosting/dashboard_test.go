package hosting

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/zachlagden/selfhost/internal/config"
	"github.com/zachlagden/selfhost/internal/store"
	"github.com/zachlagden/selfhost/internal/store/storetest"
	"github.com/zachlagden/selfhost/internal/ui"
)

const dashboardShell = "<!doctype html><title>dashboard</title>"

func (e *env) routerWithDashboard() *Router {
	e.t.Helper()
	shell, err := ui.New(fstest.MapFS{
		"index.html":      {Data: []byte(dashboardShell)},
		"assets/app-1.js": {Data: []byte("export {}")},
	})
	if err != nil {
		e.t.Fatal(err)
	}
	management := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeText(w, http.StatusOK, "management", true)
	})
	return New(shell.Wrap(management), e.settings, e.db, e.files, Options{
		Public: config.PublicURL{Scheme: "https", Host: managementHost}, Probes: e.probes, Resolver: e.resolver,
		Access: e.access, Now: e.clock.Now, Domains: e.domains, Views: e.views,
	})
}

func serveVia(router *Router, host, target string) *httptest.ResponseRecorder {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "https://"+host+target, nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestOnlyTheManagementHostServesTheDashboardShell(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.publishSite("demo", demoSite)
		e.enableSubdomains()
		router := e.routerWithDashboard()

		for _, target := range []string{"/", "/login", "/invite/abc", "/index.html"} {
			rec := serveVia(router, managementHost, target)
			if rec.Code != http.StatusOK || rec.Body.String() != dashboardShell {
				t.Errorf("management %s = %d %q, want the shell", target, rec.Code, rec.Body.String())
			}
		}
		if rec := serveVia(router, managementHost, "/assets/app-1.js"); rec.Code != http.StatusOK || rec.Header().Get("Cache-Control") != "public, max-age=31536000, immutable" {
			t.Errorf("management asset = %d %q", rec.Code, rec.Header().Get("Cache-Control"))
		}
		if rec := serveVia(router, managementHost, "/api/v1/me"); rec.Body.String() != "management" {
			t.Errorf("API path = %q, want the management handler", rec.Body.String())
		}
		if rec := serveVia(router, managementHost, "/s/demo/"); rec.Body.String() != "<h1>demo</h1>" {
			t.Errorf("path mode = %q, want the site", rec.Body.String())
		}

		for _, host := range []string{siteHost, "nobody.sites.example.com", "demo.example.org", "192.0.2.10"} {
			for _, target := range []string{"/login", "/invite/abc", "/assets/app-1.js", "/missing"} {
				rec := serveVia(router, host, target)
				if strings.Contains(rec.Body.String(), "dashboard") || rec.Code != http.StatusNotFound {
					t.Errorf("%s%s = %d %q, want the plain 404", host, target, rec.Code, rec.Body.String())
				}
			}
		}
		if rec := serveVia(router, siteHost, "/"); rec.Body.String() != "<h1>demo</h1>" {
			t.Errorf("site root = %q, want the site", rec.Body.String())
		}
		if rec := serveVia(router, siteHost, "/index.html"); rec.Body.String() != "<h1>demo</h1>" {
			t.Errorf("site /index.html = %q, want the site", rec.Body.String())
		}
	})
}

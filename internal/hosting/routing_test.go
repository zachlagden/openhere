package hosting

import (
	"net/http"
	"strings"
	"testing"

	"github.com/zachlagden/selfhost/internal/store"
	"github.com/zachlagden/selfhost/internal/store/storetest"
)

var demoSite = siteSpec{files: map[string]string{"index.html": "<h1>demo</h1>"}}

func TestHostRoutingMatrix(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.publishSite("demo", demoSite)
		e.enableSubdomains()

		cases := []struct {
			name, host, target string
			status             int
			body               string
			managed            bool
		}{
			{"management root", managementHost, "/", 200, "management", true},
			{"management api path", managementHost, "/api/v1/me", 200, "management", true},
			{"management with port", managementHost + ":8443", "/x", 200, "management", true},
			{"management uppercase", "SITES.Example.COM", "/x", 200, "management", true},
			{"management trailing dot", managementHost + ".", "/x", 200, "management", true},
			{"site subdomain", siteHost, "/", 200, "<h1>demo</h1>", false},
			{"site subdomain with port", siteHost + ":8443", "/", 200, "<h1>demo</h1>", false},
			{"site subdomain uppercase", "DEMO.Sites.Example.com", "/", 200, "<h1>demo</h1>", false},
			{"site subdomain trailing dot", siteHost + ".", "/", 200, "<h1>demo</h1>", false},
			{"unknown slug", "nobody.sites.example.com", "/", 404, "Not found\n", false},
			{"two labels", "a.demo.sites.example.com", "/", 404, "Not found\n", false},
			{"other domain", "demo.example.org", "/", 404, "Not found\n", false},
			{"bare ip", "192.0.2.10", "/", 404, "Not found\n", false},
			{"empty host", "", "/", 404, "Not found\n", false},
			{"management route on a site host", siteHost, "/api/v1/me", 404, "Not found\n", false},
			{"site file on the management host", managementHost, "/index.html", 200, "management", true},
			{"path mode", managementHost, "/s/demo/", 200, "<h1>demo</h1>", false},
			{"path mode unknown slug", managementHost, "/s/nobody/", 404, "Not found\n", false},
			{"path mode with port and case", "Sites.Example.com:443", "/s/demo/", 200, "<h1>demo</h1>", false},
		}
		for _, tc := range cases {
			before := e.managed.Load()
			rec := e.get(tc.host, tc.target)
			expectBody(t, rec, tc.status, tc.body)
			if tc.managed && rec.Header().Get("X-Management-Path") != tc.target {
				t.Errorf("%s: management saw path %q, want %q", tc.name, rec.Header().Get("X-Management-Path"), tc.target)
			}
			if reached := e.managed.Load() != before; reached != tc.managed {
				t.Errorf("%s: management handler reached = %v, want %v", tc.name, reached, tc.managed)
			}
		}
	})
}

func TestDisabledModesServeNothing(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.publishSite("demo", demoSite)

		expectBody(t, e.site("/"), 404, "Not found\n")
		expectBody(t, e.get(managementHost, "/s/demo/"), 200, "<h1>demo</h1>")

		e.enableSubdomains()
		expectBody(t, e.site("/"), 200, "<h1>demo</h1>")

		e.set(`{"hosting.pathEnabled": false}`)
		expectBody(t, e.get(managementHost, "/s/demo/"), 404, "Not found\n")
		expectBody(t, e.site("/"), 200, "<h1>demo</h1>")

		e.set(`{"hosting.pathEnabled": true}`)
		e.set(`{"hosting.subdomainEnabled": false}`)
		expectBody(t, e.site("/"), 404, "Not found\n")
		expectBody(t, e.get(managementHost, "/s/demo/"), 200, "<h1>demo</h1>")
	})
}

func TestEveryNotFoundCauseLooksTheSame(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.publishSite("demo", demoSite)
		draft, err := e.publish.Begin(t.Context(), beginDraft(e, "draft"))
		if err != nil || draft.Site.Slug != "draft" {
			t.Fatalf("create draft site: %v", err)
		}
		e.enableSubdomains()

		subdomain := map[string]snapshot{
			"unknown slug":              capture(e.get("nobody.sites.example.com", "/")),
			"site without live":         capture(e.get("draft.sites.example.com", "/")),
			"unknown host":              capture(e.get("elsewhere.example.org", "/")),
			"two labels":                capture(e.get("a.b.sites.example.com", "/")),
			"missing file":              capture(e.get(siteHost, "/missing.png")),
			"invalid path":              capture(e.get(siteHost, "/a/%2e%2e/b")),
			"management on a site host": capture(e.get(siteHost, "/api/v1/me")),
		}
		assertIdentical(t, subdomain)

		e.set(`{"hosting.pathEnabled": true}`)
		paths := map[string]snapshot{
			"unknown slug":      capture(e.get(managementHost, "/s/nobody/")),
			"site without live": capture(e.get(managementHost, "/s/draft/")),
			"missing file":      capture(e.get(managementHost, "/s/demo/missing.png")),
			"invalid slug":      capture(e.get(managementHost, "/s/Not_A_Slug/")),
			"empty slug":        capture(e.get(managementHost, "/s/")),
			"bare s":            capture(e.get(managementHost, "/s")),
			"invalid path":      capture(e.get(managementHost, "/s/demo/a/%2e%2e/b")),
		}
		e.set(`{"hosting.pathEnabled": false}`)
		paths["path mode disabled"] = capture(e.get(managementHost, "/s/demo/"))
		assertIdentical(t, paths)

		e.set(`{"hosting.pathEnabled": true}`)
		e.set(`{"hosting.subdomainEnabled": false}`)
		disabled := capture(e.get(siteHost, "/"))
		if disabled != subdomain["unknown slug"] {
			t.Errorf("subdomain mode off differs from an unknown slug:\n%+v\n%+v", disabled, subdomain["unknown slug"])
		}
	})
}

func assertIdentical(t *testing.T, shots map[string]snapshot) {
	t.Helper()
	var reference snapshot
	var referenceName string
	for name, shot := range shots {
		if referenceName == "" {
			reference, referenceName = shot, name
		}
		if shot.status != http.StatusNotFound {
			t.Errorf("%s: status = %d, want 404", name, shot.status)
		}
		if shot != reference {
			t.Errorf("%s differs from %s:\n%+v\n%+v", name, referenceName, shot, reference)
		}
		if strings.Contains(shot.body, "demo") || strings.Contains(shot.body, "draft") {
			t.Errorf("%s: the body names a site: %q", name, shot.body)
		}
	}
}

func TestOnlyGetAndHeadAreAllowed(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.publishSite("demo", demoSite)
		e.enableSubdomains()

		for _, method := range []string{"POST", "PUT", "PATCH", "DELETE", "OPTIONS"} {
			for _, target := range [][2]string{{siteHost, "/"}, {"nobody.sites.example.com", "/"}, {managementHost, "/s/demo/"}, {managementHost, "/s/nobody/"}} {
				rec := e.request(method, target[0], target[1])
				if rec.Code != http.StatusMethodNotAllowed || rec.Header().Get("Allow") != "GET, HEAD" {
					t.Errorf("%s %s%s = %d, Allow %q", method, target[0], target[1], rec.Code, rec.Header().Get("Allow"))
				}
			}
		}
		head := e.request("HEAD", siteHost, "/")
		if head.Code != 200 || head.Body.Len() != 0 || head.Header().Get("Content-Length") != "13" {
			t.Errorf("HEAD = %d, %d body bytes, length %q", head.Code, head.Body.Len(), head.Header().Get("Content-Length"))
		}
	})
}

func TestManagementHandlerNeverSeesSiteHosts(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.publishSite("demo", demoSite)
		e.enableSubdomains()

		targets := []string{"/", "/api/v1/me", "/api/v1/setup", "/s/demo/", "/.well-known/selfhost-check/x"}
		for _, host := range []string{siteHost, "nobody.sites.example.com", "other.example.org", "localhost"} {
			for _, target := range targets {
				e.get(host, target, "Authorization", "Bearer sk_secret", "Cookie", "session=abc; __Host-session=abc")
			}
		}
		if reached := e.managed.Load(); reached != 0 {
			t.Errorf("the management handler ran %d times for site hosts", reached)
		}
	})
}

package hosting

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/zachlagden/selfhost/internal/config"
	"github.com/zachlagden/selfhost/internal/store"
	"github.com/zachlagden/selfhost/internal/store/storetest"
)

const customHost = "www.example.org"

func TestVerifiedCustomDomainServesTheSite(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.publishSite("demo", demoSite)
		e.verifiedDomain("demo", customHost)

		rec := e.get(customHost, "/")
		expectBody(t, rec, 200, "<h1>demo</h1>")
		if policy := rec.Header().Get("Content-Security-Policy"); policy != "" {
			t.Errorf("custom domain sent a policy: %q", policy)
		}
		if rec.Header().Get("Access-Control-Allow-Origin") != "" {
			t.Error("custom domain sent a CORS header")
		}
		expectBody(t, e.get("WWW.Example.org:8443", "/"), 200, "<h1>demo</h1>")
		expectBody(t, e.get("www.example.org.", "/"), 200, "<h1>demo</h1>")
		expectBody(t, e.get(customHost, "/missing"), 404, "Not found\n")
	})
}

func TestCustomDomainNeverReachesManagement(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.publishSite("demo", demoSite)
		e.verifiedDomain("demo", customHost)
		before := e.managed.Load()
		for _, target := range []string{"/api/v1/me", "/login", "/s/demo/", "/api/v1/access/handoff"} {
			expectBody(t, e.get(customHost, target), 404, "Not found\n")
		}
		if e.managed.Load() != before {
			t.Error("the management handler ran for a custom domain")
		}
	})
}

func TestCustomDomainRefusalsLookIdentical(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.publishSite("demo", demoSite)
		e.claimDomain("demo", "pending.example.org")
		gone := e.verifiedDomain("demo", "gone.example.org")
		e.verifiedDomain("demo", customHost)
		if err := e.domains.Delete(context.Background(), gone.SiteID, gone.ID); err != nil {
			t.Fatal(err)
		}
		reference := capture(e.get("unknown.example.org", "/"))
		if reference.status != 404 {
			t.Fatalf("unknown host = %d", reference.status)
		}
		for _, host := range []string{"pending.example.org", "gone.example.org", "www.example.org.evil.example", "nobody.sites.example.com"} {
			if got := capture(e.get(host, "/")); got != reference {
				t.Errorf("%s answered %+v, want %+v", host, got, reference)
			}
		}
		e.set(`{"hosting.customDomainsEnabled": false}`)
		if got := capture(e.get(customHost, "/")); got != reference {
			t.Errorf("disabled feature answered %+v, want %+v", got, reference)
		}
		e.set(`{"hosting.customDomainsEnabled": true}`)
		expectBody(t, e.get(customHost, "/"), 200, "<h1>demo</h1>")
	})
}

func TestCustomDomainsWorkWhilePathModeIsOff(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.publishSite("demo", demoSite)
		e.verifiedDomain("demo", customHost)
		e.enableSubdomains()
		e.set(`{"hosting.pathEnabled": false}`)
		expectBody(t, e.get(managementHost, "/s/demo/"), 404, "Not found\n")
		expectBody(t, e.get(customHost, "/"), 200, "<h1>demo</h1>")
	})
}

func TestEverySiteDomainServesItsOwnSiteOnly(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.publishSite("demo", demoSite)
		e.publishSite("guides", siteSpec{files: map[string]string{"index.html": "guides"}})
		e.verifiedDomain("demo", "a.example.org")
		e.verifiedDomain("demo", "b.example.org")
		e.verifiedDomain("guides", "c.example.org")
		for _, host := range []string{"a.example.org", "b.example.org"} {
			expectBody(t, e.get(host, "/"), 200, "<h1>demo</h1>")
		}
		expectBody(t, e.get("c.example.org", "/"), 200, "guides")
	})
}

func TestCustomDomainStopsServingWhenRemoved(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.publishSite("demo", demoSite)
		d := e.verifiedDomain("demo", customHost)
		expectBody(t, e.get(customHost, "/"), 200, "<h1>demo</h1>")
		if err := e.domains.Delete(context.Background(), d.SiteID, d.ID); err != nil {
			t.Fatal(err)
		}
		expectBody(t, e.get(customHost, "/"), 404, "Not found\n")
	})
}

func TestCustomDomainFollowsSiteChanges(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.publishSite("demo", demoSite)
		e.verifiedDomain("demo", customHost)
		expectBody(t, e.get(customHost, "/"), 200, "<h1>demo</h1>")
		e.publishSite("demo", siteSpec{files: map[string]string{"index.html": "second"}})
		expectBody(t, e.get(customHost, "/"), 200, "second")
		if err := e.sites.DeleteSite(context.Background(), e.alex, "demo"); err != nil {
			t.Fatal(err)
		}
		expectBody(t, e.get(customHost, "/"), 404, "Not found\n")
	})
}

func TestPasswordGateWorksOnACustomDomain(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.publishSite("demo", nextExport)
		e.protect("demo", passwordMode())
		e.verifiedDomain("demo", customHost)

		rec := e.get(customHost, "/about")
		if rec.Code != 401 || !hasFrameAncestorsNone(rec.Header()) || rec.Body.String() == "about" {
			t.Fatalf("locked custom domain = %d %q", rec.Code, rec.Body.String())
		}
		cookie := e.unlockedCookie(customHost, "")
		if cookie.Name != hostViewerCookie || cookie.Path != "/" || cookie.Domain != "" || !cookie.Secure || !cookie.HttpOnly {
			t.Errorf("cookie = %+v", cookie)
		}
		expectBody(t, e.withCookie(customHost, "/about", cookie), 200, "about")
		if rec := e.withCookie(customHost, "/about", cookie); rec.Header().Get("Cache-Control") == "" || !strings.Contains(rec.Header().Get("Cache-Control"), "private") {
			t.Errorf("gated content cache = %q", rec.Header().Get("Cache-Control"))
		}
	})
}

func TestCustomDomainCookieNameFollowsTheProxyHeaders(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnvWith(t, db, envOptions{public: config.PublicURL{Scheme: "http", Host: managementHost}, proxies: proxyTrust("192.0.2.0/24")})
		e.publishSite("demo", nextExport)
		e.protect("demo", passwordMode())
		e.verifiedDomain("demo", customHost)
		unlock := func(headers ...string) *http.Cookie {
			token := tokenIn(t, e.get(customHost, "/", headers...).Body.String())
			form := url.Values{"password": {sharedSecret}, "token": {token}, "return": {"/about"}}
			rec := e.postForm(customHost, UnlockPath, form, "", headers...)
			cookie := viewerCookieOf(rec)
			if rec.Code != 303 || cookie == nil {
				t.Fatalf("unlock = %d %+v", rec.Code, cookie)
			}
			return cookie
		}
		if plain := unlock(); plain.Name != viewerCookie || plain.Secure {
			t.Errorf("plain HTTP cookie = %+v", plain)
		}
		if secure := unlock("X-Forwarded-Proto", "https"); secure.Name != hostViewerCookie || !secure.Secure {
			t.Errorf("proxied HTTPS cookie = %+v", secure)
		}
	})
}

func TestACookieFromOneHostDoesNotUnlockAnother(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.publishSite("demo", nextExport)
		e.publishSite("other", nextExport)
		e.protect("demo", passwordMode())
		e.protect("other", passwordMode())
		e.verifiedDomain("demo", customHost)
		e.verifiedDomain("other", "other.example.org")
		cookie := e.unlockedCookie(customHost, "")
		if rec := e.withCookie("other.example.org", "/about", cookie); rec.Code != 401 {
			t.Errorf("a cookie for another site unlocked a different domain: %d", rec.Code)
		}
	})
}

func TestGatePageHandoffLinkNamesTheCustomDomain(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.publishSite("demo", nextExport)
		e.protect("demo", restrictedMode("sam@example.com"))
		e.verifiedDomain("demo", customHost)
		body := e.get(customHost, "/about").Body.String()
		if !strings.Contains(body, "host=www.example.org") || !strings.Contains(body, "site=demo") {
			t.Errorf("handoff link does not name the custom domain: %q", body)
		}
		e.enableSubdomains()
		if body := e.site("/about").Body.String(); strings.Contains(body, "host=") {
			t.Errorf("subdomain handoff link carries a host: %q", body)
		}
	})
}

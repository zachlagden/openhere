package hosting

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/zachlagden/openhere/internal/access"
	"github.com/zachlagden/openhere/internal/config"
	"github.com/zachlagden/openhere/internal/hashlimit"
	"github.com/zachlagden/openhere/internal/ratelimit"
	"github.com/zachlagden/openhere/internal/store"
	"github.com/zachlagden/openhere/internal/store/storetest"
)

func TestRightPasswordSetsACookieAndOpensTheSite(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.gatedDemo(passwordMode())
		rec := e.unlock(siteHost, "", sharedSecret)
		if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/about" {
			t.Fatalf("unlock = %d, Location %q", rec.Code, rec.Header().Get("Location"))
		}
		if rec.Header().Get("Cache-Control") != "private, no-store" {
			t.Errorf("Cache-Control = %q", rec.Header().Get("Cache-Control"))
		}
		cookie := viewerCookieOf(rec)
		if cookie == nil {
			t.Fatal("no viewer cookie")
		}
		page := e.withCookie(siteHost, "/about", cookie)
		expectBody(t, page, 200, "about")
		if page.Header().Get("Cache-Control") != "private, no-cache" {
			t.Errorf("html Cache-Control = %q", page.Header().Get("Cache-Control"))
		}
		asset := e.withCookie(siteHost, "/_next/static/chunk-1.js", cookie)
		expectBody(t, asset, 200, "console.log(1)")
		if asset.Header().Get("Cache-Control") != "private, no-cache" {
			t.Errorf("asset Cache-Control = %q", asset.Header().Get("Cache-Control"))
		}
		missing := e.withCookie(siteHost, "/nope", cookie)
		expectBody(t, missing, 404, "custom missing")
		if missing.Header().Get("Cache-Control") != "private, no-cache" {
			t.Errorf("404.html Cache-Control = %q", missing.Header().Get("Cache-Control"))
		}
		for _, r := range []*httpResult{{"page", page.Header()}, {"asset", asset.Header()}, {"404", missing.Header()}} {
			if !hasFrameAncestorsNone(r.header) || r.header.Get("X-Frame-Options") != "DENY" {
				t.Errorf("%s: gated content can be framed: %v", r.label, r.header)
			}
		}
		if e.withCookie(siteHost, "/about", cookie, "If-None-Match", `"`+hashOf("about")+`"`).Code != http.StatusNotModified {
			t.Error("conditional requests stopped working for an unlocked viewer")
		}
	})
}

type httpResult struct {
	label  string
	header http.Header
}

func TestWrongPasswordIsRefusedWithAGenericMessage(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.gatedDemo(passwordMode())
		for _, guess := range []string{"", "wrong", sharedSecret + " ", strings.ToUpper(sharedSecret)} {
			rec := e.unlock(siteHost, "", guess)
			if rec.Code != http.StatusUnauthorized || viewerCookieOf(rec) != nil {
				t.Errorf("guess %q: %d, cookie %v", guess, rec.Code, viewerCookieOf(rec))
			}
			if !strings.Contains(rec.Body.String(), "That password is not right.") {
				t.Errorf("guess %q: body lacks the message", guess)
			}
			if strings.Contains(rec.Body.String(), guess) && guess != "" {
				t.Errorf("guess %q was echoed back", guess)
			}
			assertNoContent(t, "wrong password", rec.Body.String())
		}
	})
}

func TestUnlockOnlyWorksInPasswordMode(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.gatedDemo(passwordMode())
		e.protect("demo", restrictedMode())
		form := url.Values{"password": {sharedSecret}, "token": {e.formToken(siteHost, "/")}}
		rec := e.postForm(siteHost, UnlockPath, form, "")
		if rec.Code != http.StatusUnauthorized || viewerCookieOf(rec) != nil {
			t.Errorf("restricted mode accepted a password: %d", rec.Code)
		}
	})
}

func TestSubdomainCookieAttributes(t *testing.T) {
	cases := []struct {
		name   string
		public config.PublicURL
		cookie string
		secure bool
	}{
		{"https", config.PublicURL{Scheme: "https", Host: managementHost}, "__Host-viewer", true},
		{"http", config.PublicURL{Scheme: "http", Host: managementHost}, "viewer", false},
	}
	for _, tc := range cases {
		storetest.Each(t, func(t *testing.T, db store.Store) {
			e := newEnvWith(t, db, envOptions{public: tc.public})
			e.gatedDemo(passwordMode())
			cookie := viewerCookieOf(e.unlock(siteHost, "", sharedSecret))
			if cookie == nil {
				t.Fatalf("%s: no cookie", tc.name)
			}
			if cookie.Name != tc.cookie || cookie.Path != "/" || cookie.Domain != "" || !cookie.HttpOnly ||
				cookie.Secure != tc.secure || cookie.SameSite != http.SameSiteLaxMode || cookie.MaxAge != 7*24*3600 {
				t.Errorf("%s: cookie = %+v", tc.name, cookie)
			}
		})
	}
}

func TestPathModeCookieAttributes(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.publishSite("demo", nextExport)
		e.protect("demo", passwordMode())
		rec := e.unlock(managementHost, "/s/demo", sharedSecret)
		if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/s/demo/about" {
			t.Fatalf("unlock = %d, Location %q", rec.Code, rec.Header().Get("Location"))
		}
		cookie := viewerCookieOf(rec)
		if cookie == nil {
			t.Fatal("no cookie")
		}
		if cookie.Name != "__Secure-viewer" || cookie.Path != "/s/demo/" || cookie.Domain != "" || !cookie.HttpOnly ||
			!cookie.Secure || cookie.SameSite != http.SameSiteNoneMode || cookie.MaxAge != 7*24*3600 {
			t.Errorf("cookie = %+v", cookie)
		}
		page := e.withCookie(managementHost, "/s/demo/about", cookie)
		expectBody(t, page, 200, "about")
		assertGatedPathModeHeaders(t, "unlocked page", page.Header())
		assertGatedPathModeHeaders(t, "gate page", e.get(managementHost, "/s/demo/").Header())
	})
}

func assertGatedPathModeHeaders(t *testing.T, label string, h http.Header) {
	t.Helper()
	policy := h.Get("Content-Security-Policy")
	if !strings.HasPrefix(policy, wantSandbox) || !strings.Contains(policy, "frame-ancestors 'none'") || strings.Contains(policy, "allow-same-origin") {
		t.Errorf("%s: policy = %q", label, policy)
	}
}

func TestPathModeCookieDoesNotOpenOtherSites(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.publishSite("demo", nextExport)
		e.publishSite("other", nextExport)
		e.protect("demo", passwordMode())
		e.protect("other", passwordMode())
		cookie := e.unlockedCookie(managementHost, "/s/demo")
		rec := e.get(managementHost, "/s/other/about", "Cookie", cookieHeader(cookie))
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("a cookie for demo opened other: %d", rec.Code)
		}
	})
}

func TestPathModeNeedsHTTPSForGatedSites(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnvWith(t, db, envOptions{public: config.PublicURL{Scheme: "http", Host: managementHost}})
		e.publishSite("demo", nextExport)
		e.protect("demo", passwordMode())
		for _, target := range []string{"/s/demo/", "/s/demo/about", "/s/demo/missing"} {
			rec := e.get(managementHost, target)
			if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "HTTPS is required") {
				t.Errorf("%s = %d %q", target, rec.Code, rec.Body.String())
			}
			assertNoContent(t, target, rec.Body.String())
			assertGatedPathModeHeaders(t, target, rec.Header())
			if rec.Header().Get("Set-Cookie") != "" {
				t.Errorf("%s set a cookie over HTTP", target)
			}
		}
		form := url.Values{"password": {sharedSecret}, "token": {"x"}}
		rec := e.postForm(managementHost, "/s/demo"+UnlockPath, form, "")
		if rec.Code != http.StatusForbidden || viewerCookieOf(rec) != nil {
			t.Errorf("unlock over HTTP = %d", rec.Code)
		}
		e.protect("demo", access.ConfigUpdate{Mode: strptr("public")})
		expectBody(t, e.get(managementHost, "/s/demo/about"), 200, "about")
	})
}

func TestPathModeOverHTTPSBehindAProxy(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		trusted := proxyTrust("192.0.2.0/24")
		e := newEnvWith(t, db, envOptions{public: config.PublicURL{Scheme: "http", Host: managementHost}, proxies: trusted})
		e.publishSite("demo", nextExport)
		e.protect("demo", passwordMode())
		rec := e.get(managementHost, "/s/demo/", "X-Forwarded-Proto", "https")
		if rec.Code != http.StatusUnauthorized || !strings.Contains(rec.Body.String(), `type="password"`) {
			t.Errorf("proxied HTTPS = %d", rec.Code)
		}
	})
}

func TestEveryKindOfChangeLogsViewersOut(t *testing.T) {
	changes := map[string]access.ConfigUpdate{
		"password":  {Password: access.PasswordChange{Set: true, Value: strptr("a different secret")}},
		"mode":      {Mode: strptr("restricted")},
		"emails":    {Emails: listptr("sam@example.com")},
		"domains":   {Domains: listptr("example.org")},
		"clear all": {Mode: strptr("public")},
	}
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.gatedDemo(passwordMode())
		for name, change := range changes {
			e.protect("demo", passwordMode())
			cookie := e.unlockedCookie(siteHost, "")
			expectBody(t, e.withCookie(siteHost, "/about", cookie), 200, "about")
			e.protect("demo", change)
			e.protect("demo", passwordMode())
			if rec := e.withCookie(siteHost, "/about", cookie); rec.Code != http.StatusUnauthorized {
				t.Errorf("%s: the old cookie still opens the site (%d)", name, rec.Code)
			}
		}
	})
}

func TestTamperedAndExpiredCookiesAreRefused(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.gatedDemo(passwordMode())
		cookie := e.unlockedCookie(siteHost, "")
		flipped := cookie.Value[:len(cookie.Value)-2] + "AA"
		if flipped == cookie.Value {
			flipped = cookie.Value[:len(cookie.Value)-2] + "BB"
		}
		for name, value := range map[string]string{"tampered": flipped, "empty": "", "junk": "junk", "truncated": cookie.Value[:20]} {
			rec := e.get(siteHost, "/about", "Cookie", cookie.Name+"="+value)
			if rec.Code != http.StatusUnauthorized {
				t.Errorf("%s cookie = %d", name, rec.Code)
			}
		}
		rec := e.get(siteHost, "/about", "Cookie", "viewer="+cookie.Value)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("a cookie under the wrong name opened the site (%d)", rec.Code)
		}
		e.clock.Advance(access.CookieLifetime - time.Second)
		if rec := e.withCookie(siteHost, "/about", cookie); rec.Code != http.StatusOK {
			t.Errorf("cookie refused a second before expiry (%d)", rec.Code)
		}
		e.clock.Advance(time.Second)
		if rec := e.withCookie(siteHost, "/about", cookie); rec.Code != http.StatusUnauthorized {
			t.Errorf("expired cookie accepted (%d)", rec.Code)
		}
	})
}

func TestCookieForOneSiteIsRefusedOnAnother(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.gatedDemo(passwordMode())
		e.publishSite("other", nextExport)
		e.protect("other", passwordMode())
		cookie := e.unlockedCookie(siteHost, "")
		if rec := e.withCookie(otherHost, "/about", cookie); rec.Code != http.StatusUnauthorized {
			t.Errorf("demo's cookie opened other (%d)", rec.Code)
		}
	})
}

func TestUnlockRejectsForgedForms(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.gatedDemo(passwordMode())
		e.publishSite("other", nextExport)
		e.protect("other", passwordMode())
		goodToken := e.formToken(siteHost, "/")
		otherToken := e.formToken(otherHost, "/")
		cases := map[string]struct {
			token   string
			headers []string
			remote  string
		}{
			"no token":            {"", nil, ""},
			"garbage token":       {"garbage", nil, ""},
			"another site token":  {otherToken, nil, ""},
			"another client":      {goodToken, nil, "198.51.100.9:4000"},
			"cross-site origin":   {goodToken, []string{"Origin", "https://evil.example"}, ""},
			"lookalike origin":    {goodToken, []string{"Origin", "https://demo.sites.example.com.evil.example"}, ""},
			"malformed origin":    {goodToken, []string{"Origin", "::::"}, ""},
			"origin without host": {goodToken, []string{"Origin", "https://"}, ""},
		}
		for name, tc := range cases {
			form := url.Values{"password": {sharedSecret}, "token": {tc.token}}
			rec := e.postForm(siteHost, UnlockPath, form, tc.remote, tc.headers...)
			if rec.Code != http.StatusForbidden || viewerCookieOf(rec) != nil {
				t.Errorf("%s: %d, cookie %v", name, rec.Code, viewerCookieOf(rec))
			}
		}
		form := url.Values{"password": {sharedSecret}, "token": {goodToken}}
		for _, origin := range []string{"https://demo.sites.example.com", "null", ""} {
			rec := e.postForm(siteHost, UnlockPath, form, "", "Origin", origin)
			if rec.Code != http.StatusSeeOther {
				t.Errorf("origin %q: %d", origin, rec.Code)
			}
		}
	})
}

func TestUnlockRequiresAFormBody(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.gatedDemo(passwordMode())
		rec := e.postForm(siteHost, UnlockPath, url.Values{"password": {sharedSecret}}, "", "Content-Type", "application/json")
		if rec.Code != http.StatusUnsupportedMediaType {
			t.Errorf("JSON body = %d", rec.Code)
		}
		big := url.Values{"password": {strings.Repeat("x", 10000)}}
		if rec := e.postForm(siteHost, UnlockPath, big, ""); rec.Code != http.StatusBadRequest {
			t.Errorf("oversized body = %d", rec.Code)
		}
	})
}

func TestReturnPathAfterUnlockIsValidated(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.gatedDemo(passwordMode())
		cases := map[string]string{
			"/docs/?a=1":     "/docs/?a=1",
			"//evil.example": "/",
			"https://evil":   "/",
			"/\\evil":        "/",
			"/%5Cevil":       "/",
			"/a/../b":        "/",
			"":               "/",
		}
		for given, want := range cases {
			form := url.Values{"password": {sharedSecret}, "token": {e.formToken(siteHost, "/")}, "return": {given}}
			rec := e.postForm(siteHost, UnlockPath, form, "")
			if rec.Header().Get("Location") != want {
				t.Errorf("return %q redirected to %q, want %q", given, rec.Header().Get("Location"), want)
			}
		}
	})
}

func TestUnlockAttemptsAreRateLimited(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		limits := DefaultGateLimits()
		limits.UnlockPerIP = ratelimit.Rule{Max: 3, Window: time.Minute}
		limits.UnlockPerSite = ratelimit.Rule{Max: 5, Window: time.Hour}
		e := newEnvWith(t, db, envOptions{limits: &limits})
		e.gatedDemo(passwordMode())
		attempt := func(remote string, headers ...string) int {
			form := url.Values{"password": {"wrong"}, "token": {e.tokenFor(siteHost, remote)}}
			return e.postForm(siteHost, UnlockPath, form, remote, headers...).Code
		}
		right := func(remote string, headers ...string) *httptest.ResponseRecorder {
			form := url.Values{"password": {sharedSecret}, "token": {e.tokenFor(siteHost, remote)}}
			return e.postForm(siteHost, UnlockPath, form, remote, headers...)
		}
		for i := 0; i < 3; i++ {
			if got := attempt("203.0.113.1:1000"); got != http.StatusUnauthorized {
				t.Fatalf("attempt %d = %d", i, got)
			}
		}
		limited := right("203.0.113.1:1000")
		if limited.Code != http.StatusTooManyRequests || viewerCookieOf(limited) != nil {
			t.Fatalf("fourth attempt = %d", limited.Code)
		}
		if limited.Header().Get("Retry-After") != "60" || !strings.Contains(limited.Body.String(), "Try again in 60 seconds") {
			t.Errorf("Retry-After %q, body %q", limited.Header().Get("Retry-After"), limited.Body.String())
		}
		asJSON := right("203.0.113.1:1000", "Accept", "application/json")
		if asJSON.Code != http.StatusTooManyRequests || !strings.Contains(asJSON.Body.String(), `"retry_after":60`) ||
			!strings.Contains(asJSON.Body.String(), `"code":"rate_limited"`) {
			t.Errorf("JSON limit response = %d %q", asJSON.Code, asJSON.Body.String())
		}
		for _, remote := range []string{"203.0.113.2:1", "203.0.113.3:1"} {
			if got := attempt(remote); got != http.StatusUnauthorized {
				t.Errorf("a different client from %s was limited (%d)", remote, got)
			}
		}
		if got := attempt("203.0.113.4:1"); got != http.StatusTooManyRequests {
			t.Errorf("the per-site limit did not hold (%d)", got)
		}
		e.clock.Advance(time.Minute)
		if got := right("203.0.113.1:1000").Code; got != http.StatusTooManyRequests {
			t.Errorf("the per-site limit lifted early (%d)", got)
		}
		e.clock.Advance(time.Hour)
		if got := right("203.0.113.1:1000").Code; got != http.StatusSeeOther {
			t.Errorf("the limit never lifted (%d)", got)
		}
	})
}

func TestUnlockReportsBusyWhenHashingIsSaturated(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.gatedDemo(passwordMode())
		previous := hashlimit.Default()
		limiter := hashlimit.New(1, 20*time.Millisecond)
		hashlimit.SetDefault(limiter)
		release, err := limiter.Acquire(context.Background())
		if err != nil {
			t.Fatalf("Acquire: %v", err)
		}
		t.Cleanup(func() {
			release()
			hashlimit.SetDefault(previous)
		})
		rec := e.unlock(siteHost, "", sharedSecret)
		if rec.Code != http.StatusServiceUnavailable || rec.Header().Get("Retry-After") == "" || viewerCookieOf(rec) != nil {
			t.Errorf("unlock = %d, Retry-After %q, cookie %v", rec.Code, rec.Header().Get("Retry-After"), viewerCookieOf(rec))
		}
	})
}

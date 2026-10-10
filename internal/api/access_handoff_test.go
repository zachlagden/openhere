package api

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/zachlagden/selfhost/internal/store"
	"github.com/zachlagden/selfhost/internal/store/storetest"
)

func handoffURL(site, ret string) string {
	query := url.Values{"site": {site}}
	if ret != "" {
		query.Set("return", ret)
	}
	return "/api/v1/access/handoff?" + query.Encode()
}

func TestHandoffWithoutASessionRedirectsToLogin(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		s := newAccessSetup(t, newHarness(t, db, harnessOptions{}))
		s.alex.setAccess(map[string]any{"mode": "restricted"})
		target := handoffURL("guarded", "/about")
		for _, slug := range []string{"guarded", "nothing-here"} {
			rec := s.h.do("GET", handoffURL(slug, "/about"), nil)
			if rec.Code != http.StatusSeeOther {
				t.Fatalf("%s: status = %d", slug, rec.Code)
			}
			loc := locationPath(t, rec)
			if loc.Path != "/login" || loc.Query().Get("next") != handoffURL(slug, "/about") {
				t.Errorf("%s: Location = %q", slug, rec.Header().Get("Location"))
			}
			if rec.Body.Len() != 0 {
				t.Errorf("%s: the redirect has a body: %q", slug, rec.Body.String())
			}
		}
		if rec := s.h.do("GET", target, nil, withBearer("sk_not-a-key")); rec.Code != http.StatusSeeOther || locationPath(t, rec).Path != "/login" {
			t.Errorf("an invalid key = %d", rec.Code)
		}
	})
}

func TestHandoffSignsInAMemberEndToEnd(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		s := newAccessSetup(t, newHarness(t, db, harnessOptions{public: httpsPublic}))
		s.alex.setAccess(map[string]any{"mode": "restricted"})
		anonymous := s.h.routedGet(testHost, "/s/guarded/about")
		if anonymous.Code != http.StatusUnauthorized {
			t.Fatalf("anonymous = %d", anonymous.Code)
		}
		if strings.Contains(anonymous.Body.String(), "secret home") {
			t.Fatal("the gate leaked the page")
		}

		session := s.h.sessionFor("sam@example.com")
		rec := s.h.do("GET", handoffURL("guarded", "/about"), nil, withCookie(session))
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("handoff = %d, %s", rec.Code, rec.Body.String())
		}
		loc := locationPath(t, rec)
		if loc.Path != "/s/guarded/.well-known/selfhost-auth" || loc.Query().Get("return") != "/about" || loc.Query().Get("token") == "" {
			t.Fatalf("Location = %q", rec.Header().Get("Location"))
		}
		if rec.Header().Get("Cache-Control") != "no-store" || rec.Header().Get("Referrer-Policy") != "no-referrer" {
			t.Errorf("headers = %v", rec.Header())
		}

		callback := s.h.routedGet(loc.Host, loc.RequestURI())
		if callback.Code != http.StatusSeeOther || callback.Header().Get("Location") != "/s/guarded/about" {
			t.Fatalf("callback = %d, Location %q", callback.Code, callback.Header().Get("Location"))
		}
		cookie := viewerCookieFrom(callback)
		if cookie == nil || cookie.Path != "/s/guarded/" {
			t.Fatalf("cookie = %+v", cookie)
		}
		page := s.h.routedGet(testHost, "/s/guarded/about", "Cookie", cookie.Name+"="+cookie.Value)
		if page.Code != http.StatusOK || page.Body.String() != "about" {
			t.Errorf("page = %d %q", page.Code, page.Body.String())
		}
		reuse := s.h.routedGet(loc.Host, loc.RequestURI())
		if reuse.Code != http.StatusUnauthorized || viewerCookieFrom(reuse) != nil {
			t.Errorf("a token worked twice (%d)", reuse.Code)
		}
	})
}

func TestHandoffTokenExpires(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		s := newAccessSetup(t, newHarness(t, db, harnessOptions{public: httpsPublic}))
		s.alex.setAccess(map[string]any{"mode": "restricted"})
		session := s.h.sessionFor("sam@example.com")
		rec := s.h.do("GET", handoffURL("guarded", ""), nil, withCookie(session))
		loc := locationPath(t, rec)
		if loc.Query().Get("return") != "/" {
			t.Errorf("default return = %q", loc.Query().Get("return"))
		}
		s.h.clock.Advance(61 * time.Second)
		if late := s.h.routedGet(loc.Host, loc.RequestURI()); late.Code != http.StatusUnauthorized {
			t.Errorf("an expired token worked (%d)", late.Code)
		}
	})
}

func TestHandoffWorksWithAnAPIKeyAndScopedGrants(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		s := newAccessSetup(t, newHarness(t, db, harnessOptions{public: httpsPublic}))
		s.alex.setAccess(map[string]any{"mode": "restricted"})
		scoped := s.h.bearerActor("rae@example.com")
		s.alex.addMember("acme", addMemberRequest{Email: "rae@example.com", Role: "read", ScopeType: "site", ScopeID: s.siteID})
		rec := scoped.call("GET", handoffURL("guarded", "/"), nil)
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("scoped reader = %d", rec.Code)
		}
		loc := locationPath(t, rec)
		callback := s.h.routedGet(loc.Host, loc.RequestURI())
		cookie := viewerCookieFrom(callback)
		if cookie == nil {
			t.Fatalf("callback = %d", callback.Code)
		}
		page := s.h.routedGet(testHost, "/s/guarded/", "Cookie", cookie.Name+"="+cookie.Value)
		if page.Code != http.StatusOK {
			t.Errorf("page = %d", page.Code)
		}
	})
}

func TestHandoffRefusesUsersWithoutReadAccess(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		s := newAccessSetup(t, newHarness(t, db, harnessOptions{}))
		s.alex.setAccess(map[string]any{"mode": "restricted"})
		stranger := s.h.sessionFor("lee@example.com")
		for _, slug := range []string{"guarded", "nothing-here"} {
			expectError(t, s.h.do("GET", handoffURL(slug, "/"), nil, withCookie(stranger)), http.StatusNotFound, codeNotFound)
		}
		var rows int
		if err := db.QueryRowContext(t.Context(), "SELECT COUNT(*) FROM viewer_handoffs").Scan(&rows); err != nil || rows != 0 {
			t.Errorf("tokens were issued: %d, err = %v", rows, err)
		}
		expectError(t, s.h.do("GET", "/api/v1/access/handoff", nil, withCookie(stranger)), http.StatusNotFound, codeNotFound)
	})
}

func TestHandoffForOtherModesGoesStraightToTheSite(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		s := newAccessSetup(t, newHarness(t, db, harnessOptions{public: httpsPublic}))
		session := s.h.sessionFor("sam@example.com")
		modes := []map[string]any{{"mode": "public"}, {"mode": "password", "password": siteSecret}}
		for _, update := range modes {
			s.alex.setAccess(update)
			rec := s.h.do("GET", handoffURL("guarded", "/docs/?a=1"), nil, withCookie(session))
			if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "https://example.com/s/guarded/docs/?a=1" {
				t.Errorf("%v: %d, Location %q", update["mode"], rec.Code, rec.Header().Get("Location"))
			}
		}
		var rows int
		if err := db.QueryRowContext(t.Context(), "SELECT COUNT(*) FROM viewer_handoffs").Scan(&rows); err != nil || rows != 0 {
			t.Errorf("tokens were issued for a site that does not need them: %d", rows)
		}
	})
}

func TestHandoffRefusesUnsafeReturnPaths(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		s := newAccessSetup(t, newHarness(t, db, harnessOptions{}))
		s.alex.setAccess(map[string]any{"mode": "restricted"})
		session := s.h.sessionFor("sam@example.com")
		evil := []string{
			"https://evil.example/", "//evil.example", "///evil.example", "/\\evil.example", "\\\\evil.example", "/%5cevil.example",
			"/%5Cevil.example", "/%2f/evil.example", "/%2F%2Fevil.example", "/%252f%252fevil.example", "javascript:alert(1)",
			"data:text/html,x", "evil.example", "/../x", "/a/../../x", "/a/%2e%2e/b", "/\t/evil.example", "/%09/evil.example",
			"/%0d%0aSet-Cookie:x=y", "/a\nb", "/#frag", "@evil.example", "http://example.com@evil.example/",
			"/" + strings.Repeat("a", 3000),
		}
		for _, ret := range evil {
			rec := s.h.do("GET", handoffURL("guarded", ret), nil, withCookie(session))
			if rec.Code != http.StatusBadRequest {
				t.Errorf("return %q: %d, Location %q", ret, rec.Code, rec.Header().Get("Location"))
			}
			if rec.Header().Get("Location") != "" {
				t.Errorf("return %q produced a redirect to %q", ret, rec.Header().Get("Location"))
			}
			signedOut := s.h.do("GET", handoffURL("guarded", ret), nil)
			if signedOut.Code != http.StatusBadRequest {
				t.Errorf("signed-out return %q: %d", ret, signedOut.Code)
			}
		}
		for _, ret := range []string{"/", "/docs/", "/a/b.html?x=1&y=//z", "/caf%C3%A9"} {
			rec := s.h.do("GET", handoffURL("guarded", ret), nil, withCookie(session))
			if rec.Code != http.StatusSeeOther || locationPath(t, rec).Query().Get("return") != ret {
				t.Errorf("return %q: %d, Location %q", ret, rec.Code, rec.Header().Get("Location"))
			}
		}
	})
}

func TestHandoffIsRateLimited(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		limits := Limits{LoginPerIP: Rule{Max: 1000, Window: time.Hour}, LoginPerPair: Rule{Max: 1000, Window: time.Hour}, LoginPerEmail: Rule{Max: 1000, Window: time.Hour},
			SetupPerIP: Rule{Max: 1000, Window: time.Hour}, SetupPerEmail: Rule{Max: 1000, Window: time.Hour},
			Handoff: Rule{Max: 3, Window: time.Minute}}
		s := newAccessSetup(t, newHarness(t, db, harnessOptions{limits: &limits}))
		s.alex.setAccess(map[string]any{"mode": "restricted"})
		session := s.h.sessionFor("sam@example.com")
		for i := 0; i < 3; i++ {
			if rec := s.h.do("GET", handoffURL("guarded", "/"), nil, withCookie(session)); rec.Code != http.StatusSeeOther {
				t.Fatalf("handoff %d = %d", i, rec.Code)
			}
		}
		body := expectError(t, s.h.do("GET", handoffURL("guarded", "/"), nil, withCookie(session)), http.StatusTooManyRequests, codeRateLimited)
		if body.RetryAfter == nil || *body.RetryAfter < 1 {
			t.Errorf("retry_after = %v", body.RetryAfter)
		}
	})
}

func TestHandoffRouteStaysOffSiteHosts(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		s := newAccessSetup(t, newHarness(t, db, harnessOptions{}))
		rec := s.h.routedGet("guarded.other.example", "/api/v1/access/handoff?site=guarded")
		if rec.Code != http.StatusNotFound || rec.Header().Get("Location") != "" {
			t.Errorf("a site host reached the API (%d)", rec.Code)
		}
	})
}

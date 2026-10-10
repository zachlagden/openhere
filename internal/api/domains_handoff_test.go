package api

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/zachlagden/selfhost/internal/store"
	"github.com/zachlagden/selfhost/internal/store/storetest"
)

func handoffWithHost(host string) string {
	query := url.Values{"site": {"guarded"}, "return": {"/about"}, "host": {host}}
	return "/api/v1/access/handoff?" + query.Encode()
}

func TestHandoffReturnsToACustomDomain(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		s := newAccessSetup(t, newHarness(t, db, harnessOptions{public: httpsPublic}))
		s.alex.setAccess(map[string]any{"mode": "restricted"})
		s.alex.verifiedDomain("guarded", customName)

		anonymous := s.h.routedGet(customName, "/about")
		if anonymous.Code != http.StatusUnauthorized {
			t.Fatalf("anonymous = %d", anonymous.Code)
		}
		if !strings.Contains(anonymous.Body.String(), "host=www.example.org") {
			t.Errorf("gate page does not carry the host parameter")
		}

		session := s.h.sessionFor("sam@example.com")
		rec := s.h.do("GET", handoffWithHost("WWW.Example.org"), nil, withCookie(session))
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("handoff = %d, %s", rec.Code, rec.Body.String())
		}
		loc := locationPath(t, rec)
		if loc.Scheme != "https" || loc.Host != customName || loc.Path != "/.well-known/selfhost-auth" || loc.Query().Get("return") != "/about" || loc.Query().Get("token") == "" {
			t.Fatalf("Location = %q", rec.Header().Get("Location"))
		}

		callback := s.h.routedGet(loc.Host, loc.RequestURI())
		if callback.Code != http.StatusSeeOther || callback.Header().Get("Location") != "/about" {
			t.Fatalf("callback = %d, Location %q", callback.Code, callback.Header().Get("Location"))
		}
		cookie := viewerCookieFrom(callback)
		if cookie == nil || cookie.Path != "/" || cookie.Domain != "" || cookie.Name != "__Host-viewer" {
			t.Fatalf("cookie = %+v", cookie)
		}
		page := s.h.routedGet(customName, "/about", "Cookie", cookie.Name+"="+cookie.Value)
		if page.Code != http.StatusOK || page.Body.String() != "about" {
			t.Errorf("page = %d %q", page.Code, page.Body.String())
		}
		if other := s.h.routedGet(testHost, "/s/guarded/about", "Cookie", cookie.Name+"="+cookie.Value); other.Code != http.StatusUnauthorized {
			t.Errorf("the custom domain cookie unlocked path mode (%d)", other.Code)
		}
	})
}

func TestHandoffRefusesEveryHostButTheSitesOwnVerifiedDomains(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		s := newAccessSetup(t, newHarness(t, db, harnessOptions{public: httpsPublic}))
		s.alex.setAccess(map[string]any{"mode": "restricted"})
		s.alex.publishLive("other", siteFiles{"index.html": "other"})
		s.alex.verifiedDomain("other", "other.example.org")
		pending := s.alex.addDomain("guarded", "pending.example.org")
		s.alex.verifiedDomain("guarded", customName)
		session := s.h.sessionFor("sam@example.com")

		refused := []string{
			"evil.example.net", "other.example.org", pending.Domain, testHost, "guarded." + testHost, "www.example.org.evil.example",
			"www.example.org@evil.example", "evil.example/www.example.org", "www.example.org:8443@evil.example", "//evil.example", "evil.example#www.example.org",
			"https://www.example.org", "www.example.org\r\nX: y", "WWW.EXAMPLE.ORG.EVIL.EXAMPLE",
		}
		for _, host := range refused {
			rec := s.h.do("GET", handoffWithHost(host), nil, withCookie(session))
			if rec.Code != http.StatusBadRequest || rec.Header().Get("Location") != "" {
				t.Errorf("host %q = %d, Location %q", host, rec.Code, rec.Header().Get("Location"))
			}
		}
		if rec := s.h.do("GET", handoffWithHost(customName), nil, withCookie(session)); rec.Code != http.StatusSeeOther {
			t.Errorf("own verified domain = %d", rec.Code)
		}

		s.alex.expect(http.StatusOK, "PATCH", "/api/v1/admin/settings", settingsPatch("hosting.customDomainsEnabled", false))
		if rec := s.h.do("GET", handoffWithHost(customName), nil, withCookie(session)); rec.Code != http.StatusBadRequest {
			t.Errorf("with the feature off = %d", rec.Code)
		}
	})
}

func TestHandoffHostStillNeedsASessionAndReadAccess(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		s := newAccessSetup(t, newHarness(t, db, harnessOptions{public: httpsPublic}))
		s.alex.setAccess(map[string]any{"mode": "restricted"})
		s.alex.verifiedDomain("guarded", customName)
		target := handoffWithHost(customName)
		if rec := s.h.do("GET", target, nil); rec.Code != http.StatusSeeOther || locationPath(t, rec).Path != "/login" {
			t.Errorf("anonymous = %d, Location %q", rec.Code, rec.Header().Get("Location"))
		}
		stranger := s.h.sessionFor("lee@example.com")
		if rec := s.h.do("GET", target, nil, withCookie(stranger)); rec.Code != http.StatusNotFound {
			t.Errorf("stranger = %d", rec.Code)
		}
	})
}

func TestHandoffWithoutAHostStaysOnTheUsualAddress(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		s := newAccessSetup(t, newHarness(t, db, harnessOptions{public: httpsPublic}))
		s.alex.setAccess(map[string]any{"mode": "restricted"})
		verified := s.alex.verifiedDomain("guarded", customName)
		s.alex.setPrimary(verified.ID, true)
		session := s.h.sessionFor("sam@example.com")
		rec := s.h.do("GET", handoffURL("guarded", "/about"), nil, withCookie(session))
		if loc := locationPath(t, rec); rec.Code != http.StatusSeeOther || loc.Host != testHost || loc.Path != "/s/guarded/.well-known/selfhost-auth" {
			t.Errorf("Location = %q", rec.Header().Get("Location"))
		}
	})
}

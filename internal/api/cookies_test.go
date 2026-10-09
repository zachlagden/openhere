package api

import (
	"net/http"
	"net/netip"
	"testing"
	"time"

	"github.com/zachlagden/selfhost/internal/config"
	"github.com/zachlagden/selfhost/internal/store"
	"github.com/zachlagden/selfhost/internal/store/storetest"
)

var httpsPublic = config.PublicURL{Scheme: "https", Host: testHost}

func TestSessionCookieOverHTTPSUsesTheHostPrefix(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{public: httpsPublic})
		setup := h.completeSetup()

		cookie := setup.cookie
		if cookie.Name != "__Host-session" || !cookie.Secure || cookie.Path != "/" || cookie.Domain != "" || !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode {
			t.Fatalf("cookie = %+v", cookie)
		}
		me := decodeBody[meBody](t, h.do("GET", "/api/v1/me", nil, withCookie(cookie)))
		if me.User.Email != "alex@example.com" {
			t.Errorf("me = %+v", me)
		}

		login := h.do("POST", "/api/v1/auth/login", loginBody())
		expectStatus(t, login, http.StatusOK)
		if c := sessionCookie(login); c == nil || c.Name != "__Host-session" || !c.Secure {
			t.Errorf("login cookie = %+v", c)
		}
	})
}

func TestPlainSessionCookieIsIgnoredOverHTTPS(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{public: httpsPublic})
		setup := h.completeSetup()

		tossed := &http.Cookie{Name: "session", Value: setup.cookie.Value, Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode}
		expectError(t, h.do("GET", "/api/v1/me", nil, withCookie(tossed)), http.StatusUnauthorized, codeUnauthenticated)
	})
}

func TestHostPrefixedCookieIsIgnoredOverPlainHTTP(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		setup := h.completeSetup()
		if setup.cookie.Name != "session" || setup.cookie.Secure {
			t.Fatalf("local cookie = %+v", setup.cookie)
		}
		expectStatus(t, h.do("GET", "/api/v1/me", nil, withCookie(setup.cookie)), http.StatusOK)

		prefixed := &http.Cookie{Name: "__Host-session", Value: setup.cookie.Value, Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode}
		expectError(t, h.do("GET", "/api/v1/me", nil, withCookie(prefixed)), http.StatusUnauthorized, codeUnauthenticated)
	})
}

func TestCookieFormFollowsTheTrustedProxy(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{proxies: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}})
		h.completeSetup()
		viaProxy := []requestOption{fromAddr("10.1.2.3:4000"), withHeader("X-Forwarded-Proto", "https")}

		secure := h.do("POST", "/api/v1/auth/login", loginBody(), viaProxy...)
		expectStatus(t, secure, http.StatusOK)
		cookie := sessionCookie(secure)
		if cookie == nil || cookie.Name != "__Host-session" || !cookie.Secure || cookie.Domain != "" || cookie.Path != "/" {
			t.Fatalf("cookie behind a trusted https proxy = %+v", cookie)
		}
		expectStatus(t, h.do("GET", "/api/v1/me", nil, append(viaProxy, withCookie(cookie))...), http.StatusOK)
		expectError(t, h.do("GET", "/api/v1/me", nil, withCookie(cookie), fromAddr("10.1.2.3:4000")), http.StatusUnauthorized, codeUnauthenticated)

		forged := h.do("POST", "/api/v1/auth/login", loginBody(), fromAddr("203.0.113.9:4000"), withHeader("X-Forwarded-Proto", "https"))
		if c := sessionCookie(forged); c == nil || c.Name != "session" || c.Secure {
			t.Errorf("cookie from an untrusted peer claiming https = %+v", c)
		}
		plain := h.do("POST", "/api/v1/auth/login", loginBody(), fromAddr("10.1.2.3:4000"))
		if c := sessionCookie(plain); c == nil || c.Name != "session" || c.Secure {
			t.Errorf("cookie behind a proxy speaking http = %+v", c)
		}
	})
}

func TestLogoutClearsTheCookieItSet(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{public: httpsPublic})
		setup := h.completeSetup()

		rec := h.do("POST", "/api/v1/auth/logout", nil, browser(), withCookie(setup.cookie))
		expectStatus(t, rec, http.StatusNoContent)
		cleared := sessionCookie(rec)
		if cleared == nil || cleared.Name != "__Host-session" || cleared.MaxAge >= 0 || !cleared.Secure || cleared.Path != "/" || cleared.Domain != "" {
			t.Errorf("cleared cookie = %+v", cleared)
		}
		expectError(t, h.do("GET", "/api/v1/me", nil, withCookie(setup.cookie)), http.StatusUnauthorized, codeUnauthenticated)
	})
}

func TestRenewedSessionKeepsTheHostPrefix(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{public: httpsPublic})
		setup := h.completeSetup()
		h.clock.Advance(24 * time.Hour)
		rec := h.do("GET", "/api/v1/me", nil, withCookie(setup.cookie))
		expectStatus(t, rec, http.StatusOK)
		renewed := sessionCookie(rec)
		if renewed == nil || renewed.Name != "__Host-session" || !renewed.Secure || renewed.Value != setup.cookie.Value {
			t.Errorf("renewed cookie = %+v", renewed)
		}
	})
}

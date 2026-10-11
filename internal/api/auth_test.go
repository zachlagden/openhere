package api

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strconv"
	"testing"
	"time"

	"github.com/zachlagden/openhere/internal/store"
	"github.com/zachlagden/openhere/internal/store/storetest"
)

type meBody struct {
	User       userResponse `json:"user"`
	AuthMethod string       `json:"authMethod"`
}

func loginBody() loginRequest {
	return loginRequest{Email: "alex@example.com", Password: testPassword}
}

func TestLogin(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		h.completeSetup()

		rec := h.do("POST", "/api/v1/auth/login", loginRequest{Email: " ALEX@example.com", Password: testPassword})
		expectStatus(t, rec, http.StatusOK)
		cookie := sessionCookie(rec)
		if cookie == nil {
			t.Fatal("login did not set a cookie")
		}
		if !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode || cookie.Path != "/" || cookie.Secure || cookie.MaxAge <= 0 {
			t.Errorf("cookie attributes = %+v", cookie)
		}
		me := decodeBody[meBody](t, h.do("GET", "/api/v1/me", nil, withCookie(cookie)))
		if me.User.Email != "alex@example.com" || me.AuthMethod != "session" {
			t.Errorf("me = %+v", me)
		}
	})
}

func TestLoginFailuresDoNotRevealAccounts(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		h.completeSetup()

		wrongPassword := h.do("POST", "/api/v1/auth/login", loginRequest{Email: "alex@example.com", Password: "not the password"})
		unknownEmail := h.do("POST", "/api/v1/auth/login", loginRequest{Email: "nobody@example.com", Password: testPassword})
		malformed := h.do("POST", "/api/v1/auth/login", loginRequest{Email: "nonsense", Password: testPassword})
		for _, rec := range []*httptest.ResponseRecorder{wrongPassword, unknownEmail, malformed} {
			expectError(t, rec, http.StatusUnauthorized, codeInvalidLogin)
			if sessionCookie(rec) != nil {
				t.Error("failed login set a cookie")
			}
		}
		if wrongPassword.Body.String() != unknownEmail.Body.String() || wrongPassword.Body.String() != malformed.Body.String() {
			t.Error("login failure bodies differ, which reveals whether an email exists")
		}
	})
}

func TestCookieSecureFlagFollowsTrustedProxy(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{proxies: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}})
		h.completeSetup()

		trusted := h.do("POST", "/api/v1/auth/login", loginBody(), fromAddr("10.1.2.3:4000"), withHeader("X-Forwarded-Proto", "https"))
		expectStatus(t, trusted, http.StatusOK)
		if c := sessionCookie(trusted); c == nil || !c.Secure {
			t.Errorf("cookie behind a trusted https proxy = %+v, want Secure", c)
		}
		untrusted := h.do("POST", "/api/v1/auth/login", loginBody(), fromAddr("203.0.113.9:4000"), withHeader("X-Forwarded-Proto", "https"))
		expectStatus(t, untrusted, http.StatusOK)
		if c := sessionCookie(untrusted); c == nil || c.Secure {
			t.Errorf("cookie from an untrusted peer = %+v, want not Secure", c)
		}
		plain := h.do("POST", "/api/v1/auth/login", loginBody(), fromAddr("10.1.2.3:4000"))
		if c := sessionCookie(plain); c == nil || c.Secure {
			t.Errorf("cookie behind a proxy speaking http = %+v, want not Secure", c)
		}
	})
}

func TestLoginRateLimitPerIP(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		limits := Limits{
			LoginPerIP:    Rule{Max: 3, Window: time.Minute},
			LoginPerPair:  Rule{Max: 1000, Window: time.Minute},
			LoginPerEmail: Rule{Max: 1000, Window: time.Minute},
			SetupPerIP:    Rule{Max: 1000, Window: time.Minute},
			SetupPerEmail: Rule{Max: 1000, Window: time.Minute},
		}
		h := newHarness(t, db, harnessOptions{limits: &limits, configured: true})
		bad := loginRequest{Email: "nobody@example.com", Password: "whatever it is"}
		for range 3 {
			expectError(t, h.do("POST", "/api/v1/auth/login", bad, fromAddr("203.0.113.1:1")), http.StatusUnauthorized, codeInvalidLogin)
		}
		rec := h.do("POST", "/api/v1/auth/login", bad, fromAddr("203.0.113.1:1"))
		body := expectError(t, rec, http.StatusTooManyRequests, codeRateLimited)
		if body.RetryAfter == nil || *body.RetryAfter < 1 || *body.RetryAfter > 60 {
			t.Errorf("retry_after = %v, want 1 to 60", body.RetryAfter)
		}
		if rec.Header().Get("Retry-After") == "" {
			t.Error("Retry-After header missing")
		}
		expectError(t, h.do("POST", "/api/v1/auth/login", bad, fromAddr("203.0.113.2:1")), http.StatusUnauthorized, codeInvalidLogin)

		h.clock.Advance(61 * time.Second)
		expectError(t, h.do("POST", "/api/v1/auth/login", bad, fromAddr("203.0.113.1:1")), http.StatusUnauthorized, codeInvalidLogin)
	})
}

func loginLimits(change func(*Limits)) *Limits {
	limits := Limits{
		LoginPerIP:    Rule{Max: 1000, Window: time.Minute},
		LoginPerPair:  Rule{Max: 1000, Window: time.Minute},
		LoginPerEmail: Rule{Max: 1000, Window: time.Minute},
		SetupPerIP:    Rule{Max: 1000, Window: time.Minute},
		SetupPerEmail: Rule{Max: 1000, Window: time.Minute},
	}
	change(&limits)
	return &limits
}

func TestLoginFailuresAreLimitedPerAddressAndEmailPair(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		limits := loginLimits(func(l *Limits) { l.LoginPerPair = Rule{Max: 2, Window: time.Minute} })
		h := newHarness(t, db, harnessOptions{limits: limits})
		h.completeSetup()
		bad := loginRequest{Email: "alex@example.com", Password: "not the password"}
		expectError(t, h.do("POST", "/api/v1/auth/login", bad, fromAddr("203.0.113.1:1")), http.StatusUnauthorized, codeInvalidLogin)
		expectError(t, h.do("POST", "/api/v1/auth/login", bad, fromAddr("203.0.113.1:1")), http.StatusUnauthorized, codeInvalidLogin)
		expectError(t, h.do("POST", "/api/v1/auth/login", bad, fromAddr("203.0.113.1:1")), http.StatusTooManyRequests, codeRateLimited)
		expectError(t, h.do("POST", "/api/v1/auth/login", loginBody(), fromAddr("203.0.113.1:1")), http.StatusTooManyRequests, codeRateLimited)

		expectStatus(t, h.do("POST", "/api/v1/auth/login", loginBody(), fromAddr("203.0.113.2:1")), http.StatusOK)

		other := loginRequest{Email: "sam@example.com", Password: "not the password"}
		expectError(t, h.do("POST", "/api/v1/auth/login", other, fromAddr("203.0.113.1:1")), http.StatusUnauthorized, codeInvalidLogin)

		h.clock.Advance(61 * time.Second)
		expectStatus(t, h.do("POST", "/api/v1/auth/login", loginBody(), fromAddr("203.0.113.1:1")), http.StatusOK)
	})
}

func TestAnAttackerCannotLockTheRealUserOut(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		limits := loginLimits(func(l *Limits) {
			l.LoginPerPair = Rule{Max: 3, Window: time.Minute}
			l.LoginPerEmail = Rule{Max: 6, Window: time.Minute}
		})
		h := newHarness(t, db, harnessOptions{limits: limits})
		h.completeSetup()
		bad := loginRequest{Email: "alex@example.com", Password: "not the password"}
		for i := range 30 {
			addr := fromAddr("198.51.100." + strconv.Itoa(i%250+1) + ":1")
			h.do("POST", "/api/v1/auth/login", bad, addr)
		}
		expectStatus(t, h.do("POST", "/api/v1/auth/login", loginBody(), fromAddr("203.0.113.50:1")), http.StatusOK)
	})
}

func TestEmailWideFailuresSlowOnlyAddressesThatAlreadyFailed(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		limits := loginLimits(func(l *Limits) {
			l.LoginPerPair = Rule{Max: 100, Window: time.Minute}
			l.LoginPerEmail = Rule{Max: 4, Window: time.Minute}
		})
		h := newHarness(t, db, harnessOptions{limits: limits})
		h.completeSetup()
		bad := loginRequest{Email: "alex@example.com", Password: "not the password"}
		for i := range 4 {
			addr := fromAddr("198.51.100." + strconv.Itoa(i+1) + ":1")
			expectError(t, h.do("POST", "/api/v1/auth/login", bad, addr), http.StatusUnauthorized, codeInvalidLogin)
		}
		expectError(t, h.do("POST", "/api/v1/auth/login", bad, fromAddr("198.51.100.1:1")), http.StatusTooManyRequests, codeRateLimited)
		expectError(t, h.do("POST", "/api/v1/auth/login", loginBody(), fromAddr("198.51.100.2:1")), http.StatusTooManyRequests, codeRateLimited)
		expectStatus(t, h.do("POST", "/api/v1/auth/login", loginBody(), fromAddr("203.0.113.77:1")), http.StatusOK)
	})
}

func TestUnknownEmailsAreLimitedLikeKnownOnes(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		limits := loginLimits(func(l *Limits) { l.LoginPerPair = Rule{Max: 2, Window: time.Minute} })
		h := newHarness(t, db, harnessOptions{limits: limits})
		h.completeSetup()
		bad := loginRequest{Email: "nobody@example.com", Password: "not the password"}
		expectError(t, h.do("POST", "/api/v1/auth/login", bad, fromAddr("203.0.113.1:1")), http.StatusUnauthorized, codeInvalidLogin)
		expectError(t, h.do("POST", "/api/v1/auth/login", bad, fromAddr("203.0.113.1:1")), http.StatusUnauthorized, codeInvalidLogin)
		expectError(t, h.do("POST", "/api/v1/auth/login", bad, fromAddr("203.0.113.1:1")), http.StatusTooManyRequests, codeRateLimited)
	})
}

func TestSuccessfulLoginResetsThePairCounter(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		limits := loginLimits(func(l *Limits) { l.LoginPerPair = Rule{Max: 2, Window: time.Minute} })
		h := newHarness(t, db, harnessOptions{limits: limits})
		h.completeSetup()
		bad := loginRequest{Email: "alex@example.com", Password: "not the password"}
		for range 5 {
			expectStatus(t, h.do("POST", "/api/v1/auth/login", loginBody()), http.StatusOK)
			expectError(t, h.do("POST", "/api/v1/auth/login", bad), http.StatusUnauthorized, codeInvalidLogin)
		}
	})
}

func TestSetupRateLimit(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		limits := Limits{
			LoginPerIP:    Rule{Max: 1000, Window: time.Minute},
			LoginPerPair:  Rule{Max: 1000, Window: time.Minute},
			LoginPerEmail: Rule{Max: 1000, Window: time.Minute},
			SetupPerIP:    Rule{Max: 2, Window: time.Minute},
			SetupPerEmail: Rule{Max: 1000, Window: time.Minute},
		}
		h := newHarness(t, db, harnessOptions{limits: &limits})
		guess := setupRequest{Email: "alex@example.com", Password: testPassword, SetupToken: "guess"}
		expectError(t, h.do("POST", "/api/v1/setup", guess), http.StatusForbidden, codeInvalidSetupToken)
		expectError(t, h.do("POST", "/api/v1/setup", guess), http.StatusForbidden, codeInvalidSetupToken)
		expectError(t, h.do("POST", "/api/v1/setup", guess), http.StatusTooManyRequests, codeRateLimited)
	})
}

func TestSessionExpiryAndSliding(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		cookie := h.completeSetup().cookie

		h.clock.Advance(20 * 24 * time.Hour)
		rec := h.do("GET", "/api/v1/me", nil, withCookie(cookie))
		expectStatus(t, rec, http.StatusOK)
		if refreshed := sessionCookie(rec); refreshed == nil || refreshed.Value != cookie.Value || refreshed.MaxAge <= 0 {
			t.Errorf("sliding session did not refresh its cookie: %+v", refreshed)
		}

		h.clock.Advance(25 * 24 * time.Hour)
		expectStatus(t, h.do("GET", "/api/v1/me", nil, withCookie(cookie)), http.StatusOK)

		h.clock.Advance(31 * 24 * time.Hour)
		expectError(t, h.do("GET", "/api/v1/me", nil, withCookie(cookie)), http.StatusUnauthorized, codeUnauthenticated)
	})
}

func TestLogout(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		cookie := h.completeSetup().cookie

		expectError(t, h.do("POST", "/api/v1/auth/logout", nil, withCookie(cookie)), http.StatusForbidden, codeCSRFRejected)
		expectStatus(t, h.do("GET", "/api/v1/me", nil, withCookie(cookie)), http.StatusOK)

		rec := h.do("POST", "/api/v1/auth/logout", nil, browser(), withCookie(cookie))
		expectStatus(t, rec, http.StatusNoContent)
		if cleared := sessionCookie(rec); cleared == nil || cleared.MaxAge >= 0 {
			t.Errorf("logout did not clear the cookie: %+v", cleared)
		}
		expectError(t, h.do("GET", "/api/v1/me", nil, withCookie(cookie)), http.StatusUnauthorized, codeUnauthenticated)
	})
}

func TestCSRFChecksForCookieRequests(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		cookie := h.completeSetup().cookie
		key := h.createKey(cookie, "laptop")

		expectError(t, h.do("POST", "/api/v1/me/keys", createKeyRequest{Label: "x"}, withCookie(cookie)), http.StatusForbidden, codeCSRFRejected)
		expectError(t, h.do("DELETE", "/api/v1/me/keys/"+key.APIKey.ID, nil, withCookie(cookie)), http.StatusForbidden, codeCSRFRejected)
		expectError(t, h.do("POST", "/api/v1/me/keys", createKeyRequest{Label: "x"}, withCookie(cookie), withHeader("X-Requested-With", "fetch")),
			http.StatusForbidden, codeCSRFRejected)
		expectError(t, h.do("POST", "/api/v1/me/keys", createKeyRequest{Label: "x"}, withCookie(cookie), withHeader("Origin", "http://"+testHost)),
			http.StatusForbidden, codeCSRFRejected)
		expectError(t, h.do("POST", "/api/v1/me/keys", createKeyRequest{Label: "x"}, browser(), withCookie(cookie), withHeader("Origin", "https://evil.example.net")),
			http.StatusForbidden, codeCSRFRejected)
		expectError(t, h.do("POST", "/api/v1/me/keys", createKeyRequest{Label: "x"}, browser(), withCookie(cookie), withHeader("Origin", "null")),
			http.StatusForbidden, codeCSRFRejected)
		expectStatus(t, h.do("GET", "/api/v1/me/keys", nil, withCookie(cookie)), http.StatusOK)
		expectStatus(t, h.do("POST", "/api/v1/me/keys", createKeyRequest{Label: "ok"}, browser(), withCookie(cookie)), http.StatusCreated)

		expectStatus(t, h.do("POST", "/api/v1/me/keys", createKeyRequest{Label: "bearer"}, withBearer(key.Key)), http.StatusCreated)
	})
}

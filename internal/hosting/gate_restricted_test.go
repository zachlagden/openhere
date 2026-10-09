package hosting

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/zachlagden/selfhost/internal/access"
	"github.com/zachlagden/selfhost/internal/ratelimit"
	"github.com/zachlagden/selfhost/internal/store"
	"github.com/zachlagden/selfhost/internal/store/storetest"
)

func answerWithout(rec *httptest.ResponseRecorder, email string) snapshot {
	snap := capture(rec)
	snap.body = strings.ReplaceAll(snap.body, email, "EMAIL")
	return snap
}

func TestAllowlistedEmailGetsACodeAndSignsIn(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.gatedDemo(restrictedMode("sam@example.com"))
		rec := e.requestCodeFor("sam@example.com")
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `name="code"`) {
			t.Fatalf("request = %d", rec.Code)
		}
		if len(e.mailer.Messages()) != 1 {
			t.Fatalf("mail = %+v", e.mailer.Messages())
		}
		code := lastCodeIn(t, e)
		wrong := e.verifyCodeFor("00000000")
		if wrong.Code != http.StatusUnauthorized || viewerCookieOf(wrong) != nil || !strings.Contains(wrong.Body.String(), "not right or has expired") {
			t.Errorf("wrong code = %d", wrong.Code)
		}
		ok := e.verifyCodeFor(code)
		if ok.Code != http.StatusSeeOther || ok.Header().Get("Location") != "/about" {
			t.Fatalf("verify = %d %q", ok.Code, ok.Header().Get("Location"))
		}
		cookie := viewerCookieOf(ok)
		if cookie == nil || cookie.Name != "__Host-viewer" {
			t.Fatalf("cookie = %+v", cookie)
		}
		expectBody(t, e.withCookie(siteHost, "/about", cookie), 200, "about")
		if again := e.verifyCodeFor(code); again.Code != http.StatusUnauthorized {
			t.Errorf("a code worked twice (%d)", again.Code)
		}
	})
}

func TestRequestingACodeRevealsNothingAboutTheAllowlist(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.gatedDemo(access.ConfigUpdate{Mode: strptr("restricted"), Emails: listptr("sam@example.com"), Domains: listptr("example.org")})
		listed := answerWithout(e.requestCodeFor("sam@example.com"), "sam@example.com")
		others := map[string]snapshot{
			"domain match": answerWithout(e.requestCodeFor("kim@example.org"), "kim@example.org"),
			"not on list":  answerWithout(e.requestCodeFor("eve@example.net"), "eve@example.net"),
			"not an email": answerWithout(e.requestCodeFor("nonsense"), "nonsense"),
		}
		if len(e.mailer.Messages()) != 2 {
			t.Fatalf("mail count = %d, want 2", len(e.mailer.Messages()))
		}
		for _, msg := range e.mailer.Messages() {
			if msg.To == "eve@example.net" {
				t.Error("mail went to an address off the allowlist")
			}
		}
		for name, other := range others {
			if other != listed {
				t.Errorf("%s answers differently:\n%+v\n%+v", name, other, listed)
			}
		}
	})
}

func TestEmptyAllowlistMeansMembersOnly(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.gatedDemo(restrictedMode())
		rec := e.requestCodeFor("sam@example.com")
		if rec.Code != http.StatusOK || len(e.mailer.Messages()) != 0 {
			t.Errorf("request = %d, mail = %d", rec.Code, len(e.mailer.Messages()))
		}
		rec = e.verifyCodeFor("12345678")
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("verify = %d", rec.Code)
		}
		token, err := e.access.IssueHandoff(context.Background(), e.siteIDOf("demo"), e.alex)
		if err != nil {
			t.Fatal(err)
		}
		cb := e.get(siteHost, CallbackPath+"?token="+token+"&return=/about")
		if cb.Code != http.StatusSeeOther || viewerCookieOf(cb) == nil {
			t.Errorf("a workspace admin could not sign in: %d", cb.Code)
		}
	})
}

func TestEmailSignInNeedsAConfiguredMailer(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.gatedDemo(restrictedMode("sam@example.com"))
		token := e.formToken(siteHost, "/")
		e.router.gate.access = access.New(db, access.Options{Params: fastParams, Now: e.clock.Now})
		body := e.site("/").Body.String()
		if strings.Contains(body, `name="email"`) || !strings.Contains(body, "Email sign-in is not available on this instance.") {
			t.Errorf("page = %s", body)
		}
		if !strings.Contains(body, "Sign in with your account") {
			t.Error("the account sign-in link disappeared")
		}
		form := url.Values{"email": {"sam@example.com"}, "token": {token}}
		rec := e.postForm(siteHost, CodePath, form, "")
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "not available") || len(e.mailer.Messages()) != 0 {
			t.Errorf("code request without a mailer = %d", rec.Code)
		}
	})
}

func TestCodeLimitsAndExpiryOverHTTP(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.gatedDemo(restrictedMode("sam@example.com"))
		e.requestCodeFor("sam@example.com")
		code := lastCodeIn(t, e)
		e.clock.Advance(access.CodeLifetime)
		if rec := e.verifyCodeFor(code); rec.Code != http.StatusUnauthorized {
			t.Errorf("an expired code worked (%d)", rec.Code)
		}
		e.requestCodeFor("sam@example.com")
		code = lastCodeIn(t, e)
		for i := 0; i < access.CodeAttempts; i++ {
			e.verifyCodeFor("00000000")
		}
		if rec := e.verifyCodeFor(code); rec.Code != http.StatusUnauthorized {
			t.Errorf("a code survived five wrong attempts (%d)", rec.Code)
		}
	})
}

func TestCodeRequestsAreRateLimitedTheSameForEveryAddress(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		limits := DefaultGateLimits()
		limits.CodeRequestPerKey = ratelimit.Rule{Max: 2, Window: 10 * time.Minute}
		limits.CodeRequestPerIP = ratelimit.Rule{Max: 7, Window: time.Hour}
		limits.CodeRequestSite = ratelimit.Rule{Max: 9, Window: time.Hour}
		e := newEnvWith(t, db, envOptions{limits: &limits})
		e.gatedDemo(restrictedMode("sam@example.com"))
		remote := "203.0.113.50:1000"
		ask := func(email, from string) *httptest.ResponseRecorder {
			form := url.Values{"email": {email}, "token": {e.tokenFor(siteHost, from)}}
			return e.postForm(siteHost, CodePath, form, from)
		}
		for _, email := range []string{"sam@example.com", "eve@example.net"} {
			for i := 0; i < 2; i++ {
				if rec := ask(email, remote); rec.Code != http.StatusOK {
					t.Fatalf("%s request %d = %d", email, i, rec.Code)
				}
			}
			rec := ask(email, remote)
			if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") == "" {
				t.Errorf("%s: third request = %d", email, rec.Code)
			}
		}
		if rec := ask("a@example.net", remote); rec.Code != http.StatusOK {
			t.Fatalf("a fresh address = %d", rec.Code)
		}
		if rec := ask("b@example.net", remote); rec.Code != http.StatusTooManyRequests {
			t.Errorf("the per-IP limit did not hold (%d)", rec.Code)
		}
		for i := 0; i < 2; i++ {
			from := "198.51.100." + string(rune('1'+i)) + ":1"
			if rec := ask("c"+string(rune('a'+i))+"@example.net", from); rec.Code != http.StatusOK {
				t.Fatalf("another client was limited early (%d)", rec.Code)
			}
		}
		if rec := ask("d@example.net", "198.51.100.9:1"); rec.Code != http.StatusTooManyRequests {
			t.Errorf("the per-site limit did not hold (%d)", rec.Code)
		}
		asJSON := e.postForm(siteHost, CodePath, url.Values{"email": {"x@example.net"}, "token": {"t"}}, remote, "Accept", "application/json")
		if !strings.Contains(asJSON.Body.String(), "retry_after") {
			t.Errorf("JSON limit body = %q", asJSON.Body.String())
		}
	})
}

func TestCodeVerificationIsRateLimited(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		limits := DefaultGateLimits()
		limits.CodeVerifyPerIP = ratelimit.Rule{Max: 4, Window: time.Hour}
		e := newEnvWith(t, db, envOptions{limits: &limits})
		e.gatedDemo(restrictedMode("sam@example.com"))
		var last int
		for i := 0; i < 5; i++ {
			last = e.verifyCodeFor("00000000").Code
		}
		if last != http.StatusTooManyRequests {
			t.Errorf("fifth verification = %d", last)
		}
	})
}

func TestHandoffCallbackSignsInAndIsSingleUse(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.gatedDemo(restrictedMode())
		siteID := e.siteIDOf("demo")
		token, err := e.access.IssueHandoff(t.Context(), siteID, e.alex)
		if err != nil {
			t.Fatal(err)
		}
		rec := e.get(siteHost, CallbackPath+"?token="+token+"&return=%2Fdocs%2F%3Fa%3D1")
		if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/docs/?a=1" {
			t.Fatalf("callback = %d, Location %q", rec.Code, rec.Header().Get("Location"))
		}
		if rec.Header().Get("Referrer-Policy") != "no-referrer" || rec.Header().Get("Cache-Control") != "private, no-store" {
			t.Errorf("callback headers = %v", rec.Header())
		}
		cookie := viewerCookieOf(rec)
		if cookie == nil || cookie.SameSite != http.SameSiteLaxMode {
			t.Fatalf("cookie = %+v", cookie)
		}
		expectBody(t, e.withCookie(siteHost, "/docs/", cookie), 200, "docs index")
		again := e.get(siteHost, CallbackPath+"?token="+token)
		if again.Code != http.StatusUnauthorized || viewerCookieOf(again) != nil || !strings.Contains(again.Body.String(), "expired") {
			t.Errorf("token reuse = %d", again.Code)
		}
	})
}

func TestHandoffCallbackRefusesBadTokens(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.gatedDemo(restrictedMode())
		e.publishSite("other", nextExport)
		e.protect("other", restrictedMode())
		other, err := e.access.IssueHandoff(t.Context(), e.siteIDOf("other"), e.alex)
		if err != nil {
			t.Fatal(err)
		}
		stranger := e.user("kim@example.com")
		unauthorised, err := e.access.IssueHandoff(t.Context(), e.siteIDOf("demo"), stranger)
		if err != nil {
			t.Fatal(err)
		}
		stale, err := e.access.IssueHandoff(t.Context(), e.siteIDOf("demo"), e.alex)
		if err != nil {
			t.Fatal(err)
		}
		e.clock.Advance(access.HandoffLifetime)
		for name, token := range map[string]string{
			"empty": "", "garbage": "garbage", "another site": other, "no access": unauthorised, "expired": stale,
		} {
			rec := e.get(siteHost, CallbackPath+"?token="+token)
			if rec.Code != http.StatusUnauthorized || viewerCookieOf(rec) != nil {
				t.Errorf("%s: %d", name, rec.Code)
			}
		}
	})
}

func TestHandoffReturnPathCannotLeaveTheSite(t *testing.T) {
	evil := []string{
		"https://evil.example/", "//evil.example", "/\\evil.example", "/%5cevil.example", "/%2f/evil.example",
		"/%2F%2Fevil.example", "\\\\evil.example", "javascript:alert(1)", "/../x", "/a/%2e%2e/b", "/\t/evil.example",
		"/%09/evil.example", "/a\r\nSet-Cookie: x=y", "http://demo.sites.example.com@evil.example/", "///evil.example",
		"/%252f%252fevil.example", "other", "",
	}
	for _, mode := range []string{"subdomain", "path"} {
		storetest.Each(t, func(t *testing.T, db store.Store) {
			e := newEnv(t, db)
			e.publishSite("demo", nextExport)
			e.enableSubdomains()
			e.protect("demo", restrictedMode())
			host, prefix := siteHost, ""
			if mode == "path" {
				host, prefix = managementHost, "/s/demo"
			}
			for _, ret := range evil {
				token, err := e.access.IssueHandoff(t.Context(), e.siteIDOf("demo"), e.alex)
				if err != nil {
					t.Fatal(err)
				}
				target := prefix + CallbackPath + "?token=" + token + "&return=" + url.QueryEscape(ret)
				rec := e.get(host, target)
				want := prefix + "/"
				if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != want {
					t.Errorf("%s return %q: %d, Location %q, want %q", mode, ret, rec.Code, rec.Header().Get("Location"), want)
				}
			}
		})
	}
}

func TestPathModeHandoffCallback(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.publishSite("demo", nextExport)
		e.protect("demo", restrictedMode())
		token, err := e.access.IssueHandoff(t.Context(), e.siteIDOf("demo"), e.alex)
		if err != nil {
			t.Fatal(err)
		}
		rec := e.get(managementHost, "/s/demo"+CallbackPath+"?token="+token+"&return=/about")
		if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/s/demo/about" {
			t.Fatalf("callback = %d, Location %q", rec.Code, rec.Header().Get("Location"))
		}
		cookie := viewerCookieOf(rec)
		if cookie == nil || cookie.Path != "/s/demo/" || cookie.SameSite != http.SameSiteNoneMode || !cookie.Secure {
			t.Fatalf("cookie = %+v", cookie)
		}
		expectBody(t, e.withCookie(managementHost, "/s/demo/about", cookie), 200, "about")
	})
}

func TestMemberCookieEndsWhenAccessIsRemoved(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		workspace := e.sharedWorkspace()
		e.publishSiteIn("demo", workspace, nextExport)
		e.enableSubdomains()
		e.protect("demo", restrictedMode())
		sam := e.user("sam@example.com")
		e.grantSite("sam@example.com", "demo")
		token, err := e.access.IssueHandoff(t.Context(), e.siteIDOf("demo"), sam)
		if err != nil {
			t.Fatal(err)
		}
		cookie := viewerCookieOf(e.get(siteHost, CallbackPath+"?token="+token))
		if cookie == nil {
			t.Fatal("a scoped reader could not sign in")
		}
		expectBody(t, e.withCookie(siteHost, "/about", cookie), 200, "about")
		if _, err := db.ExecContext(t.Context(), "DELETE FROM workspace_access WHERE user_id = $1", sam); err != nil {
			t.Fatal(err)
		}
		if rec := e.withCookie(siteHost, "/about", cookie); rec.Code != http.StatusUnauthorized {
			t.Errorf("a removed member still has access (%d)", rec.Code)
		}
	})
}

func TestFormsCannotBeFramedOrCrossPosted(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		e := newEnv(t, db)
		e.gatedDemo(restrictedMode("sam@example.com"))
		form := url.Values{"email": {"sam@example.com"}, "token": {e.formToken(siteHost, "/")}}
		rec := e.postForm(siteHost, CodePath, form, "", "Origin", "https://evil.example")
		if rec.Code != http.StatusForbidden || len(e.mailer.Messages()) != 0 {
			t.Errorf("cross-site code request = %d, mail = %d", rec.Code, len(e.mailer.Messages()))
		}
		if !hasFrameAncestorsNone(rec.Header()) {
			t.Error("the refusal can be framed")
		}
	})
}

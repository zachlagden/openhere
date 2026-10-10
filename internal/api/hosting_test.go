package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/zachlagden/selfhost/internal/settings"
	"github.com/zachlagden/selfhost/internal/store"
	"github.com/zachlagden/selfhost/internal/store/storetest"
)

type fakeChecker struct {
	h        *harness
	verified bool
	detail   string
	calls    int
}

func (f *fakeChecker) Check(ctx context.Context) (settings.Settings, error) {
	f.calls++
	return f.h.settings.RecordWildcard(ctx, f.verified, f.detail)
}

var tinySite = siteFiles{"index.html": "<h1>hello</h1>"}

func TestSiteURLFollowsTheHostingSettings(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{public: httpsPublic})
		root := h.rootActor()
		root.createWorkspace("Acme Ltd", "acme")

		pending := root.publishNewSite("acme-docs", tinySite)
		if pending.URL == nil || *pending.URL != "https://example.com/s/acme-docs/" {
			t.Fatalf("publish url = %v", pending.URL)
		}
		root.uploadFiles(pending, tinySite)
		finalized := decodeBody[resultResponse](t, root.finalize(pending.Slug, pending.VersionID))
		if finalized.URL == nil || *finalized.URL != "https://example.com/s/acme-docs/" {
			t.Errorf("finalize url = %v", finalized.URL)
		}
		site := decodeBody[siteEnvelope](t, root.expect(http.StatusOK, "GET", "/api/v1/sites/acme-docs", nil)).Site
		if site.URL == nil || *site.URL != "https://example.com/s/acme-docs/" {
			t.Errorf("site url = %v", site.URL)
		}
		listed := decodeBody[siteListResponse](t, root.expect(http.StatusOK, "GET", "/api/v1/sites", nil)).Sites
		if len(listed) != 1 || listed[0].URL == nil || *listed[0].URL != "https://example.com/s/acme-docs/" {
			t.Errorf("listed sites = %+v", listed)
		}

		if _, err := h.settings.RecordWildcard(context.Background(), true, ""); err != nil {
			t.Fatal(err)
		}
		root.setSettings(`{"settings":{"hosting.baseDomain":"sites.example.com"}}`)
		if _, err := h.settings.RecordWildcard(context.Background(), true, ""); err != nil {
			t.Fatal(err)
		}
		root.setSettings(`{"settings":{"hosting.subdomainEnabled":true}}`)
		site = decodeBody[siteEnvelope](t, root.expect(http.StatusOK, "GET", "/api/v1/sites/acme-docs", nil)).Site
		if site.URL == nil || *site.URL != "https://acme-docs.sites.example.com/" {
			t.Errorf("subdomain url = %v", site.URL)
		}
		again := decodeBody[resultResponse](t, root.expect(http.StatusOK, "POST", "/api/v1/publish/acme-docs/versions/"+pending.VersionID+"/restore", nil))
		if again.URL == nil || *again.URL != "https://acme-docs.sites.example.com/" {
			t.Errorf("restore url = %v", again.URL)
		}

		root.setSettings(`{"settings":{"hosting.subdomainEnabled":false}}`)
		site = decodeBody[siteEnvelope](t, root.expect(http.StatusOK, "GET", "/api/v1/sites/acme-docs", nil)).Site
		if site.URL == nil || *site.URL != "https://example.com/s/acme-docs/" {
			t.Errorf("url with only path mode on = %v", site.URL)
		}
	})
}

func TestSubdomainModeNeedsAVerifiedWildcard(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		root := h.rootActor()
		enable := `{"settings":{"hosting.subdomainEnabled":true}}`

		root.expectError(http.StatusConflict, codeWildcardNotVerified, "PATCH", "/api/v1/admin/settings", enable)
		if _, err := h.settings.RecordWildcard(context.Background(), false, "no record"); err != nil {
			t.Fatal(err)
		}
		root.expectError(http.StatusConflict, codeWildcardNotVerified, "PATCH", "/api/v1/admin/settings", enable)
		current := decodeBody[settingsEnvelope](t, root.expect(http.StatusOK, "GET", "/api/v1/admin/settings", nil)).Settings
		if current["hosting.subdomainEnabled"] != false || current["hosting.wildcardStatus"] != "failed" || current["hosting.wildcardDetail"] != "no record" || current["hosting.wildcardCheckedAt"] == nil {
			t.Errorf("settings after a failed check = %v", current)
		}

		if _, err := h.settings.RecordWildcard(context.Background(), true, ""); err != nil {
			t.Fatal(err)
		}
		on := decodeBody[settingsEnvelope](t, root.expect(http.StatusOK, "PATCH", "/api/v1/admin/settings", enable)).Settings
		if on["hosting.subdomainEnabled"] != true {
			t.Errorf("settings = %v", on)
		}
		if _, err := h.settings.RecordWildcard(context.Background(), false, "lost the record"); err != nil {
			t.Fatal(err)
		}
		off := decodeBody[settingsEnvelope](t, root.expect(http.StatusOK, "GET", "/api/v1/admin/settings", nil)).Settings
		if off["hosting.subdomainEnabled"] != false {
			t.Errorf("a failed check left subdomain mode on: %v", off)
		}
	})
}

func TestWildcardKeysAreReadOnly(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		root := h.rootActor()
		for _, body := range []string{
			`{"settings":{"hosting.wildcardStatus":"ok"}}`,
			`{"settings":{"hosting.wildcardCheckedAt":"2026-03-01T12:00:00Z"}}`,
			`{"settings":{"hosting.wildcardDetail":"fine"}}`,
		} {
			root.expectError(http.StatusBadRequest, codeValidationFailed, "PATCH", "/api/v1/admin/settings", body)
		}
		got := decodeBody[settingsEnvelope](t, root.expect(http.StatusOK, "GET", "/api/v1/admin/settings", nil)).Settings
		if got["hosting.wildcardStatus"] != "unchecked" {
			t.Errorf("settings = %v", got)
		}
	})
}

func TestWildcardCheckEndpoint(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		checker := &fakeChecker{verified: true}
		limits := DefaultLimits()
		limits.LoginPerIP = Rule{Max: 1000, Window: time.Hour}
		limits.WildcardCheck = Rule{Max: 2, Window: time.Hour}
		h := newHarness(t, db, harnessOptions{wildcard: checker, limits: &limits})
		checker.h = h
		root := h.rootActor()
		sam := h.bearerActor("sam@example.com")
		path := "/api/v1/admin/hosting/check"

		expectError(t, h.do("POST", path, nil), http.StatusUnauthorized, codeUnauthenticated)
		sam.expectError(http.StatusNotFound, codeNotFound, "POST", path, nil)
		if checker.calls != 0 {
			t.Fatalf("a non-root caller ran the check")
		}

		result := decodeBody[wildcardEnvelope](t, root.expect(http.StatusOK, "POST", path, nil))
		if result.Wildcard.Status != "ok" || result.Wildcard.CheckedAt == nil || result.Wildcard.Detail != "" {
			t.Errorf("result = %+v", result.Wildcard)
		}
		if result.Settings["hosting.wildcardStatus"] != "ok" {
			t.Errorf("settings = %v", result.Settings)
		}

		checker.verified, checker.detail = false, "no record"
		failed := decodeBody[wildcardEnvelope](t, root.expect(http.StatusOK, "POST", path, nil))
		if failed.Wildcard.Status != "failed" || failed.Wildcard.Detail != "no record" {
			t.Errorf("failed result = %+v", failed.Wildcard)
		}

		limited := root.call("POST", path, nil)
		expectError(t, limited, http.StatusTooManyRequests, codeRateLimited)
		if checker.calls != 2 {
			t.Errorf("calls = %d, want 2", checker.calls)
		}
	})
}

func TestWildcardCheckFollowsCSRFRulesForCookies(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		checker := &fakeChecker{verified: true}
		h := newHarness(t, db, harnessOptions{wildcard: checker})
		checker.h = h
		cookie := h.completeSetup().cookie
		expectError(t, h.do("POST", "/api/v1/admin/hosting/check", nil, withCookie(cookie)), http.StatusForbidden, codeCSRFRejected)
		expectStatus(t, h.do("POST", "/api/v1/admin/hosting/check", nil, withCookie(cookie), browser()), http.StatusOK)
	})
}

func TestWildcardCheckIsAbsentWithoutAChecker(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		root := h.rootActor()
		root.expectError(http.StatusNotFound, codeNotFound, "POST", "/api/v1/admin/hosting/check", nil)
	})
}

func (h *harness) serveRouted(method, host, target string, opts ...requestOption) *httptest.ResponseRecorder {
	h.t.Helper()
	req := httptest.NewRequestWithContext(h.t.Context(), method, "http://"+host+target, nil)
	req.Host = host
	for _, opt := range opts {
		opt(req)
	}
	rec := httptest.NewRecorder()
	h.routed.ServeHTTP(rec, req)
	return rec
}

func TestAuthenticationNeverRunsOnSiteHosts(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		root := h.rootActor()
		root.createWorkspace("Acme Ltd", "acme")
		root.publishLive("acme-docs", tinySite)
		if _, err := h.settings.RecordWildcard(context.Background(), true, ""); err != nil {
			t.Fatal(err)
		}
		root.setSettings(`{"settings":{"hosting.baseDomain":"sites.example.com"}}`)
		if _, err := h.settings.RecordWildcard(context.Background(), true, ""); err != nil {
			t.Fatal(err)
		}
		root.setSettings(`{"settings":{"hosting.subdomainEnabled":true}}`)

		login := h.do("POST", "/api/v1/auth/login", loginBody())
		expectStatus(t, login, http.StatusOK)
		session := sessionCookie(login)
		h.clock.Advance(time.Hour)
		before := lastUsedOf(t, h, session)

		bearer := withBearer(root.key)
		for _, target := range []string{"/api/v1/me", "/api/v1/me/keys", "/api/v1/admin/settings"} {
			rec := h.serveRouted("GET", "acme-docs.sites.example.com", target, bearer, withCookie(session))
			if rec.Code != http.StatusNotFound || rec.Body.String() != "Not found\n" || rec.Header().Get("Content-Type") == "application/json" {
				t.Errorf("GET %s on a site host = %d %q", target, rec.Code, rec.Body.String())
			}
			if len(rec.Result().Cookies()) != 0 || rec.Header().Get("WWW-Authenticate") != "" {
				t.Errorf("GET %s on a site host touched authentication: %v", target, rec.Header())
			}
		}
		post := h.serveRouted("POST", "acme-docs.sites.example.com", "/api/v1/auth/logout", bearer, withCookie(session), browser())
		if post.Code != http.StatusMethodNotAllowed {
			t.Errorf("POST on a site host = %d", post.Code)
		}
		site := h.serveRouted("GET", "acme-docs.sites.example.com", "/", bearer, withCookie(session))
		expectStatus(t, site, http.StatusOK)

		path := h.serveRouted("GET", testHost, "/s/acme-docs/", bearer, withCookie(session))
		expectStatus(t, path, http.StatusOK)
		if len(path.Result().Cookies()) != 0 {
			t.Errorf("path mode touched the session cookie: %v", path.Result().Cookies())
		}

		if used := lastUsedOf(t, h, session); used == nil || !used.Equal(*before) {
			t.Errorf("the key was used by a site host request: before %v, after %v", before, used)
		}
		expectStatus(t, h.serveRouted("GET", testHost, "/api/v1/me", bearer), http.StatusOK)
		if used := lastUsedOf(t, h, session); used == nil || !used.After(*before) {
			t.Error("the management host did not authenticate the key")
		}
	})
}

func lastUsedOf(t *testing.T, h *harness, session *http.Cookie) *time.Time {
	t.Helper()
	rec := h.do("GET", "/api/v1/me/keys", nil, withCookie(session))
	expectStatus(t, rec, http.StatusOK)
	keys := decodeBody[keyListResponse](t, rec).Keys
	for _, key := range keys {
		if key.Label == "tests" {
			return key.LastUsedAt
		}
	}
	t.Fatal("test key not found")
	return nil
}

func TestSettingsRefuseToSwitchEveryHostingModeOff(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		root := h.rootActor()
		path := "/api/v1/admin/settings"

		root.expectError(http.StatusConflict, codeNoHostingMode, "PATCH", path, `{"settings":{"hosting.pathEnabled":false}}`)
		got := decodeBody[settingsEnvelope](t, root.expect(http.StatusOK, "GET", path, nil)).Settings
		if got["hosting.pathEnabled"] != true {
			t.Errorf("a refused change was stored: %v", got)
		}
		if _, err := h.settings.RecordWildcard(context.Background(), true, ""); err != nil {
			t.Fatal(err)
		}
		swapped := decodeBody[updateSettingsEnvelope](t, root.expect(http.StatusOK, "PATCH", path,
			`{"settings":{"hosting.subdomainEnabled":true,"hosting.pathEnabled":false}}`))
		if swapped.Settings["hosting.subdomainEnabled"] != true || swapped.Settings["hosting.pathEnabled"] != false || len(swapped.Notices) != 0 {
			t.Errorf("a swap in one patch = %+v", swapped)
		}
		root.expectError(http.StatusConflict, codeNoHostingMode, "PATCH", path, `{"settings":{"hosting.subdomainEnabled":false}}`)
	})
}

func TestChangingTheBaseDomainReportsThatPathModeWasSwitchedOn(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		root := h.rootActor()
		root.setSettings(`{"settings":{"hosting.baseDomain":"sites.example.com"}}`)
		if _, err := h.settings.RecordWildcard(context.Background(), true, ""); err != nil {
			t.Fatal(err)
		}
		root.setSettings(`{"settings":{"hosting.subdomainEnabled":true,"hosting.pathEnabled":false}}`)

		moved := decodeBody[updateSettingsEnvelope](t, root.expect(http.StatusOK, "PATCH", "/api/v1/admin/settings",
			`{"settings":{"hosting.baseDomain":"sites.example.org"}}`))
		if moved.Settings["hosting.subdomainEnabled"] != false || moved.Settings["hosting.pathEnabled"] != true {
			t.Errorf("settings = %v", moved.Settings)
		}
		if len(moved.Notices) != 1 || moved.Notices[0].Code != "path_mode_enabled" || moved.Notices[0].Message == "" {
			t.Errorf("notices = %+v", moved.Notices)
		}
	})
}

package api

import (
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/zachlagden/selfhost/internal/store"
	"github.com/zachlagden/selfhost/internal/store/storetest"
)

func TestAccessDefaultsToPublic(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		s := newAccessSetup(t, newHarness(t, db, harnessOptions{}))
		got := s.alex.access()
		if got.Mode != "public" || got.HasPassword || len(got.EmailAllowlist) != 0 || len(got.DomainAllowlist) != 0 || !got.EmailCodeAvailable {
			t.Errorf("access = %+v", got)
		}
		rec := s.alex.expect(http.StatusOK, "GET", "/api/v1/publish/guarded/access", nil)
		for _, key := range []string{`"emailAllowlist":[]`, `"domainAllowlist":[]`} {
			if !strings.Contains(rec.Body.String(), key) {
				t.Errorf("response lacks %s: %s", key, rec.Body.String())
			}
		}
	})
}

func TestUpdatingAccessStoresAndNormalises(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		s := newAccessSetup(t, newHarness(t, db, harnessOptions{}))
		got := s.alex.setAccess(map[string]any{
			"mode": "password", "password": siteSecret,
			"emailAllowlist": []string{"Sam@Example.com", "alex@example.com", "sam@example.com"}, "domainAllowlist": []string{"@Example.org"},
		})
		if got.Mode != "password" || !got.HasPassword {
			t.Errorf("access = %+v", got)
		}
		if !slices.Equal(got.EmailAllowlist, []string{"alex@example.com", "sam@example.com"}) || !slices.Equal(got.DomainAllowlist, []string{"example.org"}) {
			t.Errorf("lists = %v %v", got.EmailAllowlist, got.DomainAllowlist)
		}
		partial := s.alex.setAccess(map[string]any{"emailAllowlist": []string{}})
		if len(partial.EmailAllowlist) != 0 || len(partial.DomainAllowlist) != 1 || partial.Mode != "password" || !partial.HasPassword {
			t.Errorf("a partial update disturbed other fields: %+v", partial)
		}
		cleared := s.alex.setAccess(map[string]any{"mode": "restricted", "password": nil})
		if cleared.Mode != "restricted" || cleared.HasPassword {
			t.Errorf("cleared = %+v", cleared)
		}
		again := s.alex.access()
		if again.Mode != "restricted" || again.HasPassword {
			t.Errorf("reread = %+v", again)
		}
	})
}

func TestAccessModeShowsInSiteResponses(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		s := newAccessSetup(t, newHarness(t, db, harnessOptions{}))
		site := decodeBody[siteEnvelope](t, s.alex.expect(http.StatusOK, "GET", "/api/v1/sites/guarded", nil)).Site
		if site.AccessMode != "public" {
			t.Errorf("accessMode = %q", site.AccessMode)
		}
		s.alex.setAccess(map[string]any{"mode": "restricted"})
		site = decodeBody[siteEnvelope](t, s.alex.expect(http.StatusOK, "GET", "/api/v1/sites/guarded", nil)).Site
		listed := decodeBody[siteListResponse](t, s.alex.expect(http.StatusOK, "GET", "/api/v1/sites", nil)).Sites
		if site.AccessMode != "restricted" || len(listed) != 1 || listed[0].AccessMode != "restricted" {
			t.Errorf("accessMode = %q, listed = %+v", site.AccessMode, listed)
		}
		updated := decodeBody[siteEnvelope](t, s.alex.expect(http.StatusOK, "PATCH", "/api/v1/sites/guarded", `{"displayName":"Guarded"}`)).Site
		if updated.AccessMode != "restricted" {
			t.Errorf("a site update lost the mode: %q", updated.AccessMode)
		}
	})
}

func TestNewSitesTakeTheInstanceDefaultMode(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		s := newAccessSetup(t, newHarness(t, db, harnessOptions{}))
		s.alex.setSettings(`{"settings":{"sites.defaultAccessMode":"restricted"}}`)
		created := s.alex.createSite("acme", createSiteRequest{Slug: "fresh-one"})
		if created.AccessMode != "restricted" {
			t.Errorf("created site mode = %q", created.AccessMode)
		}
		s.alex.publishLive("published-one", siteFiles{"index.html": "x"})
		published := decodeBody[siteEnvelope](t, s.alex.expect(http.StatusOK, "GET", "/api/v1/sites/published-one", nil)).Site
		if published.AccessMode != "restricted" {
			t.Errorf("published site mode = %q", published.AccessMode)
		}
		if old := decodeBody[siteEnvelope](t, s.alex.expect(http.StatusOK, "GET", "/api/v1/sites/guarded", nil)).Site; old.AccessMode != "public" {
			t.Errorf("an existing site changed to %q", old.AccessMode)
		}
		s.alex.setSettings(`{"settings":{"sites.defaultAccessMode":"public"}}`)
		if later := s.alex.createSite("acme", createSiteRequest{Slug: "fresh-two"}); later.AccessMode != "public" {
			t.Errorf("later site mode = %q", later.AccessMode)
		}
		s.alex.expectError(http.StatusBadRequest, codeValidationFailed, "PATCH", "/api/v1/admin/settings", `{"settings":{"sites.defaultAccessMode":"password"}}`)
	})
}

func TestAccessPermissions(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		s := newAccessSetup(t, newHarness(t, db, harnessOptions{}))
		path := "/api/v1/publish/guarded/access"
		body := map[string]any{"mode": "restricted"}

		s.lee.expectError(http.StatusNotFound, codeNotFound, "GET", path, nil)
		s.lee.expectError(http.StatusNotFound, codeNotFound, "PATCH", path, body)
		s.sam.expect(http.StatusOK, "GET", path, nil)
		s.sam.expectError(http.StatusForbidden, codeForbidden, "PATCH", path, body)
		s.kim.expect(http.StatusOK, "GET", path, nil)
		s.kim.expect(http.StatusOK, "PATCH", path, body)
		s.alex.expectError(http.StatusNotFound, codeNotFound, "GET", "/api/v1/publish/nothing-here/access", nil)
		s.alex.expectError(http.StatusNotFound, codeNotFound, "PATCH", "/api/v1/publish/nothing-here/access", body)
		expectError(t, s.h.do("GET", path, nil), http.StatusUnauthorized, codeUnauthenticated)
		expectError(t, s.h.do("PATCH", path, body), http.StatusUnauthorized, codeUnauthenticated)
		if got := s.alex.access(); got.Mode != "restricted" {
			t.Errorf("mode = %q", got.Mode)
		}
		if s.lee.call("PATCH", path, "not json").Code != http.StatusNotFound {
			t.Error("a user without access learned about the body format")
		}
	})
}

func TestAccessScopedGrants(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		s := newAccessSetup(t, newHarness(t, db, harnessOptions{}))
		s.alex.publishLive("elsewhere", siteFiles{"index.html": "x"})
		scopedRead := s.h.bearerActor("rae@example.com")
		scopedWrite := s.h.bearerActor("wes@example.com")
		s.alex.addMember("acme", addMemberRequest{Email: "rae@example.com", Role: "read", ScopeType: "site", ScopeID: s.siteID})
		s.alex.addMember("acme", addMemberRequest{Email: "wes@example.com", Role: "write", ScopeType: "site", ScopeID: s.siteID})

		scopedRead.expect(http.StatusOK, "GET", "/api/v1/publish/guarded/access", nil)
		scopedRead.expectError(http.StatusForbidden, codeForbidden, "PATCH", "/api/v1/publish/guarded/access", map[string]any{"mode": "restricted"})
		scopedRead.expectError(http.StatusNotFound, codeNotFound, "GET", "/api/v1/publish/elsewhere/access", nil)
		scopedWrite.expect(http.StatusOK, "PATCH", "/api/v1/publish/guarded/access", map[string]any{"mode": "restricted"})
		scopedWrite.expectError(http.StatusNotFound, codeNotFound, "PATCH", "/api/v1/publish/elsewhere/access", map[string]any{"mode": "restricted"})
	})
}

func TestAccessRejectsInvalidInput(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		s := newAccessSetup(t, newHarness(t, db, harnessOptions{}))
		path := "/api/v1/publish/guarded/access"
		many := make([]string, 501)
		for i := range many {
			many[i] = fmt.Sprintf("user%d@example.com", i)
		}
		bad := map[string]any{
			"unknown mode":         map[string]any{"mode": "friends"},
			"password mode empty":  map[string]any{"mode": "password"},
			"short password":       map[string]any{"password": "short"},
			"empty password":       map[string]any{"password": ""},
			"numeric password":     map[string]any{"password": 12345678},
			"bad email":            map[string]any{"emailAllowlist": []string{"nope"}},
			"bad domain":           map[string]any{"domainAllowlist": []string{"https://example.com"}},
			"too many emails":      map[string]any{"emailAllowlist": many},
			"unknown field":        map[string]any{"owner": "me"},
			"wrong type for list":  map[string]any{"emailAllowlist": "sam@example.com"},
			"wrong type for mode":  map[string]any{"mode": 1},
			"null entry in a list": map[string]any{"emailAllowlist": []any{nil}},
		}
		for name, body := range bad {
			rec := s.alex.call("PATCH", path, body)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("%s: status = %d, body %s", name, rec.Code, rec.Body.String())
			}
		}
		if got := s.alex.access(); got.Mode != "public" || got.HasPassword {
			t.Errorf("a rejected update changed the site: %+v", got)
		}
		exact := many[:500]
		s.alex.setAccess(map[string]any{"emailAllowlist": exact})
		s.alex.expectError(http.StatusUnsupportedMediaType, codeUnsupportedMedia, "PATCH", path, nil)
		huge := `{"emailAllowlist":["` + strings.Repeat("a", 600<<10) + `"]}`
		expectError(t, s.alex.call("PATCH", path, huge), http.StatusRequestEntityTooLarge, codeBodyTooLarge)
	})
}

func TestSessionAccessChangesNeedTheCSRFHeaders(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		s := newAccessSetup(t, newHarness(t, db, harnessOptions{}))
		cookie := s.h.sessionFor("kim@example.com")
		body := map[string]any{"mode": "restricted"}
		expectError(t, s.h.do("PATCH", "/api/v1/publish/guarded/access", body, withCookie(cookie)), http.StatusForbidden, codeCSRFRejected)
		expectStatus(t, s.h.do("PATCH", "/api/v1/publish/guarded/access", body, withCookie(cookie), browser()), http.StatusOK)
		expectStatus(t, s.h.do("GET", "/api/v1/publish/guarded/access", nil, withCookie(cookie)), http.StatusOK)
	})
}

func TestSitePasswordsNeverLeave(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		s := newAccessSetup(t, newHarness(t, db, harnessOptions{logToBuf: true}))
		responses := []string{
			s.alex.expect(http.StatusOK, "PATCH", "/api/v1/publish/guarded/access", map[string]any{"mode": "password", "password": evilSecret}).Body.String(),
			s.alex.expect(http.StatusOK, "GET", "/api/v1/publish/guarded/access", nil).Body.String(),
			s.alex.expect(http.StatusOK, "GET", "/api/v1/sites/guarded", nil).Body.String(),
			s.alex.expect(http.StatusOK, "GET", "/api/v1/sites", nil).Body.String(),
			s.alex.expect(http.StatusOK, "GET", "/api/v1/admin/settings", nil).Body.String(),
			s.alex.call("PATCH", "/api/v1/publish/guarded/access", map[string]any{"password": evilSecret, "mode": "friends"}).Body.String(),
			s.alex.call("PATCH", "/api/v1/publish/guarded/access", `{"password":"`+evilSecret+`",`).Body.String(),
		}
		for i, body := range responses {
			if strings.Contains(body, evilSecret) || strings.Contains(body, "argon2id") {
				t.Errorf("response %d exposes the password or its hash: %s", i, body)
			}
		}
		var stored string
		if err := db.QueryRowContext(t.Context(), "SELECT access_password_hash FROM sites WHERE id = $1", s.siteID).Scan(&stored); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(stored, evilSecret) {
			t.Error("the password is stored in clear")
		}
		if strings.Contains(s.h.logs.String(), evilSecret) {
			t.Errorf("the password reached the logs: %s", s.h.logs.String())
		}
	})
}

func TestSettingsNeverExposeTheViewerSecret(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		s := newAccessSetup(t, newHarness(t, db, harnessOptions{}))
		if _, err := s.h.access.IssueHandoff(t.Context(), s.siteID, s.alex.userID); err != nil {
			t.Fatal(err)
		}
		if _, _, err := s.h.access.IssueCookie(t.Context(), s.siteID, 0, "password", ""); err != nil {
			t.Fatal(err)
		}
		var stored string
		if err := db.QueryRowContext(t.Context(), "SELECT value FROM instance_settings WHERE key = 'internal.viewerSecret'").Scan(&stored); err != nil || stored == "" {
			t.Fatalf("the secret was not stored: %v", err)
		}
		body := s.alex.expect(http.StatusOK, "GET", "/api/v1/admin/settings", nil).Body.String()
		if strings.Contains(body, stored) || strings.Contains(body, "viewerSecret") {
			t.Errorf("settings expose the secret: %s", body)
		}
		for _, patch := range []string{
			`{"settings":{"internal.viewerSecret":"0000"}}`, `{"settings":{"viewerSecret":"0000"}}`,
		} {
			s.alex.expectError(http.StatusBadRequest, codeValidationFailed, "PATCH", "/api/v1/admin/settings", patch)
		}
	})
}

func TestEmailCodeAvailabilityFollowsTheMailer(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		s := newAccessSetup(t, newHarness(t, db, harnessOptions{}))
		if !s.alex.access().EmailCodeAvailable {
			t.Error("a configured mailer reported the codes as unavailable")
		}
	})
	storetest.Each(t, func(t *testing.T, db store.Store) {
		s := newAccessSetup(t, newHarness(t, db, harnessOptions{noMailer: true}))
		got := s.alex.setAccess(map[string]any{"mode": "restricted", "emailAllowlist": []string{"sam@example.com"}})
		if got.EmailCodeAvailable || len(got.EmailAllowlist) != 1 {
			t.Errorf("access = %+v", got)
		}
	})
}

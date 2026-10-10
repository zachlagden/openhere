package api

import (
	"net/http"
	"testing"

	"github.com/zachlagden/selfhost/internal/store"
	"github.com/zachlagden/selfhost/internal/store/storetest"
)

func TestSettingsAreRootOnly(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		root := h.rootActor()
		sam := h.bearerActor("sam@example.com")

		for _, method := range []string{"GET", "PATCH"} {
			expectError(t, h.do(method, "/api/v1/admin/settings", `{"settings":{"retention.keepVersions":5}}`), http.StatusUnauthorized, codeUnauthenticated)
			sam.expectError(http.StatusNotFound, codeNotFound, method, "/api/v1/admin/settings", `{"settings":{"retention.keepVersions":5}}`)
		}
		sam.expectError(http.StatusNotFound, codeNotFound, "PATCH", "/api/v1/admin/settings", `{"settings":{"nope":1}}`)
		got := decodeBody[settingsEnvelope](t, root.expect(http.StatusOK, "GET", "/api/v1/admin/settings", nil))
		if got.Settings["retention.keepVersions"] != float64(10) {
			t.Errorf("a rejected non-root PATCH changed settings: %v", got.Settings)
		}
	})
}

func TestSettingsReadAndUpdate(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		root := h.rootActor()

		defaults := decodeBody[settingsEnvelope](t, root.expect(http.StatusOK, "GET", "/api/v1/admin/settings", nil)).Settings
		want := map[string]any{
			"retention.keepVersions":          float64(10),
			"limits.maxFilesPerVersion":       float64(2500),
			"limits.maxVersionBytes":          float64(1 << 30),
			"limits.maxFileBytes":             float64(256 << 20),
			"hosting.baseDomain":              "",
			"hosting.subdomainEnabled":        false,
			"hosting.pathEnabled":             true,
			"hosting.customDomainsEnabled":    true,
			"hosting.maxDomainsPerSite":       float64(10),
			"hosting.wildcardFailures":        float64(0),
			"hosting.subdomainDisabledReason": nil,
			"hosting.wildcardStatus":          "unchecked",
			"hosting.wildcardCheckedAt":       nil,
			"hosting.wildcardDetail":          "",
			"sites.defaultAccessMode":         "public",
			"invites.whoCanInvite":            "root",
			"workspaces.whoCanCreate":         "any",
			"email.provider":                  "none",
			"email.fromAddress":               "",
			"email.fromName":                  "",
			"email.smtp.host":                 "",
			"email.smtp.port":                 float64(587),
			"email.smtp.security":             "starttls",
			"email.smtp.username":             "",
			"email.smtp.passwordSet":          false,
			"email.resend.apiKeySet":          false,
		}
		assertSettings(t, defaults, want)

		updated := decodeBody[settingsEnvelope](t, root.expect(http.StatusOK, "PATCH", "/api/v1/admin/settings",
			`{"settings":{"retention.keepVersions":4,"hosting.baseDomain":"Sites.Example.com"}}`)).Settings
		want["retention.keepVersions"] = float64(4)
		want["hosting.baseDomain"] = "sites.example.com"
		assertSettings(t, updated, want)

		reread := decodeBody[settingsEnvelope](t, root.expect(http.StatusOK, "GET", "/api/v1/admin/settings", nil)).Settings
		assertSettings(t, reread, want)
	})
}

func assertSettings(t *testing.T, got, want map[string]any) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("settings = %v, want %v", got, want)
		return
	}
	for key, value := range want {
		if got[key] != value {
			t.Errorf("%s = %v, want %v", key, got[key], value)
		}
	}
}

func TestSettingsRejectBadPatches(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		root := h.rootActor()
		bad := map[string]string{
			"unknown key":      `{"settings":{"retention.nothing":1}}`,
			"mixed with valid": `{"settings":{"retention.keepVersions":3,"nope":1}}`,
			"zero keep":        `{"settings":{"retention.keepVersions":0}}`,
			"wrong type":       `{"settings":{"hosting.pathEnabled":"yes"}}`,
			"bad domain":       `{"settings":{"hosting.baseDomain":"https://example.com"}}`,
			"empty":            `{"settings":{}}`,
			"missing":          `{}`,
		}
		for _, body := range bad {
			root.expectError(http.StatusBadRequest, codeValidationFailed, "PATCH", "/api/v1/admin/settings", body)
		}
		root.expectError(http.StatusBadRequest, codeInvalidJSON, "PATCH", "/api/v1/admin/settings", `{"keepVersions":3}`)
		root.expectError(http.StatusBadRequest, codeInvalidJSON, "PATCH", "/api/v1/admin/settings", `[]`)
		got := decodeBody[settingsEnvelope](t, root.expect(http.StatusOK, "GET", "/api/v1/admin/settings", nil)).Settings
		if got["retention.keepVersions"] != float64(10) {
			t.Errorf("a rejected patch changed settings: %v", got)
		}
	})
}

func TestSettingsPatchFollowsCSRFRules(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		cookie := h.completeSetup().cookie
		body := `{"settings":{"retention.keepVersions":2}}`
		expectError(t, h.do("PATCH", "/api/v1/admin/settings", body, withCookie(cookie)), http.StatusForbidden, codeCSRFRejected)
		expectStatus(t, h.do("PATCH", "/api/v1/admin/settings", body, withCookie(cookie), browser()), http.StatusOK)
		expectStatus(t, h.do("GET", "/api/v1/admin/settings", nil, withCookie(cookie)), http.StatusOK)
	})
}

func TestChangedLimitsApplyImmediately(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		root := h.rootActor()
		two := manifestOf(siteFiles{"a": "1", "b": "2"})
		root.expect(http.StatusCreated, "POST", "/api/v1/publish", publishRequest{Files: two})
		root.setSettings(`{"settings":{"limits.maxFilesPerVersion":1}}`)
		root.expectError(http.StatusRequestEntityTooLarge, codeTooManyFiles, "POST", "/api/v1/publish", publishRequest{Files: two})
		root.setSettings(`{"settings":{"limits.maxFilesPerVersion":2}}`)
		root.expect(http.StatusCreated, "POST", "/api/v1/publish", publishRequest{Files: two})
	})
}

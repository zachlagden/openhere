package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zachlagden/selfhost/internal/store"
	"github.com/zachlagden/selfhost/internal/store/storetest"
)

var allMethods = []string{"GET", "HEAD", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}

func TestOnlyThreeRoutesAreReachableWithoutAuth(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		h.completeSetup()

		public := map[string]bool{
			"GET /api/v1/setup/status":  true,
			"HEAD /api/v1/setup/status": true,
			"POST /api/v1/setup":        true,
			"POST /api/v1/auth/login":   true,
		}
		paths := []string{
			"/", "/health", "/healthz", "/ready", "/docs", "/openapi.yaml", "/openapi.json", "/llms.txt", "/robots.txt", "/favicon.ico",
			"/api", "/api/", "/api/v1", "/api/v1/", "/api/v1/health", "/api/v1/docs", "/api/v1/openapi.yaml",
			"/api/v1/setup", "/api/v1/setup/status", "/api/v1/setup/status/", "/api/v1/auth/login", "/api/v1/auth/logout",
			"/api/v1/me", "/api/v1/me/keys", "/api/v1/me/keys/abc", "/api/v1/nope",
			"/api/v1/workspaces", "/api/v1/workspaces/acme", "/api/v1/workspaces/acme/members", "/api/v1/workspaces/acme/members/abc",
			"/api/v1/workspaces/acme/folders", "/api/v1/workspaces/acme/folders/abc", "/api/v1/workspaces/acme/sites",
			"/api/v1/sites", "/api/v1/sites/launch", "/api/v1/sites/launch/files", "/api/v1/sites/launch/files/index.html", "/api/v1/sites/launch/files/a/b/c.js",
			"/api/v1/publish", "/api/v1/publish/launch", "/api/v1/publish/launch/versions", "/api/v1/publish/launch/versions/abc",
			"/api/v1/publish/launch/versions/abc/files/index.html", "/api/v1/publish/launch/versions/abc/files/a/b.js",
			"/api/v1/publish/launch/versions/abc/finalize", "/api/v1/publish/launch/versions/abc/restore", "/api/v1/admin/settings", "/api/v1/admin", "/api/v1/../me", "//", "/api//v1/me", "/./", "/s/site/",
		}
		for _, path := range paths {
			for _, method := range allMethods {
				if public[method+" "+path] {
					continue
				}
				rec := h.do(method, path, nil)
				if rec.Code != http.StatusUnauthorized && rec.Code != http.StatusNotFound {
					t.Errorf("%s %s = %d, want 401 or 404 without credentials", method, path, rec.Code)
				}
				if method != "HEAD" && rec.Body.Len() > 0 && rec.Header().Get("Content-Type") != "application/json" {
					t.Errorf("%s %s returned a non-JSON body", method, path)
				}
			}
		}
	})
}

func TestUnknownPathsReturnJSON404WhenSignedIn(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		cookie := h.completeSetup().cookie
		for _, path := range []string{"/", "/health", "/api/v1/nope"} {
			expectError(t, h.do("GET", path, nil, withCookie(cookie)), http.StatusNotFound, codeNotFound)
		}
		expectError(t, h.do("GET", "/api/v1/nope", nil), http.StatusUnauthorized, codeUnauthenticated)
	})
}

func TestSecurityHeadersOnEveryResponse(t *testing.T) {
	storetest.Each(t, func(t *testing.T, db store.Store) {
		h := newHarness(t, db, harnessOptions{})
		cookie := h.completeSetup().cookie
		responses := map[string]func() *httptest.ResponseRecorder{
			"ok":           func() *httptest.ResponseRecorder { return h.do("GET", "/api/v1/setup/status", nil) },
			"unauthorised": func() *httptest.ResponseRecorder { return h.do("GET", "/api/v1/me", nil) },
			"not found":    func() *httptest.ResponseRecorder { return h.do("GET", "/nope", nil, withCookie(cookie)) },
			"bad request":  func() *httptest.ResponseRecorder { return h.do("POST", "/api/v1/auth/login", "{") },
		}
		for name, call := range responses {
			rec := call()
			for header, want := range map[string]string{
				"X-Content-Type-Options": "nosniff",
				"Referrer-Policy":        "no-referrer",
				"X-Frame-Options":        "DENY",
				"Cache-Control":          "no-store",
			} {
				if got := rec.Header().Get(header); got != want {
					t.Errorf("%s: %s = %q, want %q", name, header, got, want)
				}
			}
			if rec.Header().Get("Content-Security-Policy") == "" {
				t.Errorf("%s: Content-Security-Policy missing", name)
			}
		}
	})
}
